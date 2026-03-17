package index

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	storemd "github.com/readmedotmd/store.md"

	"github.com/readmedotmd/search.md/analysis"
	"github.com/readmedotmd/search.md/document"
)

// Key prefixes for organizing data in the KV store.
const (
	// d/{docID} -> stored document JSON
	prefixDoc = "d/"
	// t/{field}/{term}/{docID} -> term frequency JSON (posting)
	prefixTerm = "t/"
	// f/{field}/{docID} -> field length (number of tokens)
	prefixFieldLen = "f/"
	// m/doc_count -> total document count
	keyDocCount = "m/doc_count"
	// m/field_len_sum/{field} -> sum of field lengths for avg calculation
	prefixFieldLenSum = "m/field_len_sum/"
	// n/{field}/{term} -> document frequency (number of docs containing term)
	prefixDocFreq = "n/"
	// v/{field}/{docID} -> vector data JSON
	prefixVector = "v/"
	// tv/{field}/{docID}/{term} -> term vector positions JSON
	prefixTermVec = "tv/"
	// num/{field}/{docID} -> numeric value
	prefixNumeric = "num/"
	// ns/{field}/{sortableValue}/{docID} -> "" (sorted numeric index for range scans)
	prefixNumericSorted = "ns/"
	// dt/{field}/{docID} -> datetime value (unix nano)
	prefixDateTime = "dt/"
	// ds/{field}/{sortableNanos}/{docID} -> "" (sorted datetime index for range scans)
	prefixDateTimeSorted = "ds/"
	// bool/{field}/{docID} -> boolean value
	prefixBool = "bool/"
	// ri/{docID} -> JSON array of {field, type, terms} for targeted deletion
	prefixRevIdx = "ri/"
	// m/index_version -> index format version
	keyIndexVersion = "m/index_version"
	// currentIndexVersion is the current index format version.
	// Increment this when the key format changes.
	currentIndexVersion = "1"
)

// Posting represents a single entry in the inverted index.
type Posting struct {
	DocID     string  `json:"d"`
	Frequency int     `json:"f"`
	Norm      float64 `json:"n"` // 1/sqrt(fieldLength)
}

// TermVector stores position information for highlighting and phrase queries.
type TermVector struct {
	Positions []analysis.TokenPosition `json:"p"`
}

// Logger is the interface used by Index for optional log output.
type Logger interface {
	Warn(msg string, args ...any)
}

// DefaultLogger wraps the standard log package.
type DefaultLogger struct{}

// Warn logs a warning message using log.Printf.
func (DefaultLogger) Warn(msg string, args ...any) {
	log.Printf("WARN: "+msg, args...)
}

// NopLogger discards all log output.
type NopLogger struct{}

// Warn is a no-op.
func (NopLogger) Warn(string, ...any) {}

// Index is the core search index backed by a store.md Store.
type Index struct {
	store         storemd.Store
	mu            sync.RWMutex
	fieldIndexers map[string]FieldIndexer // type name -> indexer
	helpers       IndexHelpers
	logger        Logger
	cache         *Cache               // term dictionary + posting list cache
	bkTrees       map[string]*bkTree   // field -> BK-tree for fuzzy search
	hnswIndices   map[string]*HNSW     // field -> HNSW index for KNN search
	batchMode     bool                 // when true, defer cache invalidation
	inMemoryMode  bool                 // when true, cache is unlimited and eagerly warmed
}

// SetLogger sets the logger used by the index. Pass NopLogger{} to suppress output.
func (idx *Index) SetLogger(l Logger) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.logger = l
}

// New creates a new Index backed by the given store.
func New(store storemd.Store) (*Index, error) {
	idx := &Index{
		store:         store,
		fieldIndexers: make(map[string]FieldIndexer),
		logger:        DefaultLogger{},
		cache:         newCache(),
		bkTrees:       make(map[string]*bkTree),
		hnswIndices:   make(map[string]*HNSW),
	}
	idx.helpers = &indexHelpersImpl{idx: idx}

	// Register built-in field indexers
	idx.RegisterFieldIndexer(&TextFieldIndexer{})
	idx.RegisterFieldIndexer(&NumericFieldIndexer{})
	idx.RegisterFieldIndexer(&BooleanFieldIndexer{})
	idx.RegisterFieldIndexer(&DateTimeFieldIndexer{})
	idx.RegisterFieldIndexer(&VectorFieldIndexer{})

	// Initialize index version if not already set, or check for mismatch.
	storedVersion, err := store.Get(context.Background(), keyIndexVersion)
	if err != nil {
		if err := store.Set(context.Background(), keyIndexVersion, currentIndexVersion); err != nil {
			return nil, fmt.Errorf("set index version: %w", err)
		}
	} else if storedVersion != currentIndexVersion {
		return nil, fmt.Errorf("index version mismatch: stored %q, expected %q", storedVersion, currentIndexVersion)
	}

	return idx, nil
}

// IndexVersion returns the stored index format version.
func (idx *Index) IndexVersion() string {
	val, err := idx.store.Get(context.Background(),keyIndexVersion)
	if err != nil {
		return ""
	}
	return val
}

// RegisterFieldIndexer registers a field indexer for a given type.
// This allows third-party plugins to add new field types.
func (idx *Index) RegisterFieldIndexer(fi FieldIndexer) {
	idx.fieldIndexers[fi.Type()] = fi
}

// fieldTypeToIndexerType maps document.FieldType to indexer type name.
func fieldTypeToIndexerType(ft document.FieldType) string {
	switch ft {
	case document.FieldTypeText, document.FieldTypeKeyword, document.FieldTypeCode:
		return "text"
	case document.FieldTypeNumeric:
		return "numeric"
	case document.FieldTypeBoolean:
		return "bool"
	case document.FieldTypeDateTime:
		return "datetime"
	case document.FieldTypeVector:
		return "vector"
	case document.FieldTypeSymbol:
		return "symbol"
	default:
		return ""
	}
}

// BeginBatch enables batch mode, deferring cache invalidation until EndBatch.
// Caller must hold idx.mu.Lock().
func (idx *Index) BeginBatch() {
	idx.batchMode = true
}

// EndBatch disables batch mode and invalidates all caches.
// Caller must hold idx.mu.Lock().
func (idx *Index) EndBatch() {
	idx.batchMode = false
	idx.cache.InvalidateAll()
	idx.bkTrees = make(map[string]*bkTree)
}

// SetInMemoryMode enables or disables in-memory mode.
// When enabled, the postings cache has no LRU eviction limit,
// and WarmCache can be used to eagerly preload all index data.
func (idx *Index) SetInMemoryMode(enabled bool) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.inMemoryMode = enabled
	if enabled {
		idx.cache.SetUnlimitedPostings()
	} else {
		idx.cache.SetMaxPostings(defaultMaxPostings)
	}
}

// InMemoryMode returns whether in-memory mode is enabled.
func (idx *Index) InMemoryMode() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.inMemoryMode
}

// WarmCache eagerly preloads all term dictionaries, posting lists,
// document frequencies, field lengths, and aggregate statistics into
// the in-memory cache. This is most effective when in-memory mode is on.
func (idx *Index) WarmCache() error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// 0. Preload all documents into cache.
	if err := idx.forEachWithPrefix(prefixDoc, func(key, value string) error {
		docID := key[len(prefixDoc):]
		sd, err := document.UnmarshalStoredData([]byte(value))
		if err != nil {
			return nil // skip malformed docs
		}
		idx.cache.SetCachedDoc(docID, sd.Fields)
		return nil
	}); err != nil {
		return fmt.Errorf("warm cache: preload documents: %w", err)
	}

	// 1. Load doc count.
	docCount, err := idx.getDocCount()
	if err != nil {
		return err
	}
	idx.cache.SetDocCount(int64(docCount))

	// 2. Single-pass: load ALL term postings, building term dicts and posting lists.
	fields := make(map[string]struct{})
	termDicts := make(map[string]map[string]struct{})        // field -> set of terms
	postingsMap := make(map[string]map[string][]Posting)     // field -> term -> postings
	if err := idx.forEachWithPrefix(prefixTerm, func(key, value string) error {
		// Key format: t/{field}/{term}/{docID}
		rest := key[len(prefixTerm):]
		slash1 := strings.IndexByte(rest, '/')
		if slash1 < 0 {
			return nil
		}
		field := rest[:slash1]
		rest2 := rest[slash1+1:]
		slash2 := strings.IndexByte(rest2, '/')
		if slash2 < 0 {
			return nil
		}
		term := rest2[:slash2]

		fields[field] = struct{}{}
		if termDicts[field] == nil {
			termDicts[field] = make(map[string]struct{})
		}
		termDicts[field][term] = struct{}{}

		p, ok := fastParsePosting(value)
		if !ok {
			var jp Posting
			if json.Unmarshal([]byte(value), &jp) == nil {
				p = jp
			} else {
				return nil
			}
		}
		if postingsMap[field] == nil {
			postingsMap[field] = make(map[string][]Posting)
		}
		postingsMap[field][term] = append(postingsMap[field][term], p)
		return nil
	}); err != nil {
		return fmt.Errorf("warm cache: scan postings: %w", err)
	}

	// Cache term dicts and postings.
	for field, termSet := range termDicts {
		terms := make([]string, 0, len(termSet))
		for t := range termSet {
			terms = append(terms, t)
		}
		sort.Strings(terms)
		idx.cache.SetTermDict(field, terms)

		for _, term := range terms {
			if ps, ok := postingsMap[field][term]; ok {
				idx.cache.SetPostings(field, term, ps)
			}
		}
	}

	// 3. Single-pass: load all doc frequencies.
	if err := idx.forEachWithPrefix(prefixDocFreq, func(key, value string) error {
		rest := key[len(prefixDocFreq):]
		slashIdx := strings.IndexByte(rest, '/')
		if slashIdx < 0 {
			return nil
		}
		field := rest[:slashIdx]
		term := rest[slashIdx+1:]
		df, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil
		}
		idx.cache.SetDocFreq(field, term, df)
		_ = field // already discovered
		return nil
	}); err != nil {
		return fmt.Errorf("warm cache: scan doc freqs: %w", err)
	}

	// 4. For each field: avg field len + bulk field lengths.
	for field := range fields {
		avg, err := idx.avgFieldLength(field)
		if err == nil {
			idx.cache.SetAvgFieldLen(field, avg)
		}

		if !idx.cache.IsFieldLensLoaded(field) {
			lengths := make(map[string]int)
			prefix := fieldLenKey(field, "")
			if err := idx.forEachWithPrefix(prefix, func(key, value string) error {
				docID := key[len(prefix):]
				n, err := strconv.Atoi(value)
				if err != nil {
					return nil
				}
				lengths[docID] = n
				return nil
			}); err != nil {
				return fmt.Errorf("warm cache: field lengths %q: %w", field, err)
			}
			idx.cache.SetFieldLensAll(field, lengths)
		}
	}

	return nil
}

// GetTermSearcherState returns the coalesced cache state for a term searcher.
func (idx *Index) GetTermSearcherState(field, term string) TermSearcherState {
	return idx.cache.GetTermSearcherState(field, term)
}

// BatchGetFieldLengths returns field lengths for multiple docIDs from cache.
// Returns (map, true) if all were found in cache, or falls back to individual lookups.
func (idx *Index) BatchGetFieldLengths(field string, docIDs []string) (map[string]int, bool) {
	if m, ok := idx.cache.GetFieldLenBatch(field, docIDs); ok {
		return m, true
	}
	// Fall back: preload into cache then batch-read.
	idx.BatchFieldLengths(field, docIDs)
	return idx.cache.GetFieldLenBatch(field, docIDs)
}

// IndexDocument indexes a document's fields into the store.
func (idx *Index) IndexDocument(doc *document.Document) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// First, delete old data if document exists
	existed, err := idx.deleteDocInternal(doc.ID)
	if err != nil {
		return fmt.Errorf("delete existing document: %w", err)
	}

	// Invalidate cached document.
	idx.cache.InvalidateDoc(doc.ID)

	// Store the document data
	storedData := doc.ToStoredData()
	docJSON, err := storedData.Marshal()
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}
	if err := idx.store.Set(context.Background(),docKey(doc.ID), string(docJSON)); err != nil {
		return fmt.Errorf("store document: %w", err)
	}

	// Process each field via registered field indexer plugins.
	var revEntries []RevIdxEntry
	for _, field := range doc.Fields {
		if !field.Index {
			continue
		}

		typeName := fieldTypeToIndexerType(field.Type)
		if typeName == "" {
			continue
		}

		fi, ok := idx.fieldIndexers[typeName]
		if !ok {
			continue
		}

		entry, err := fi.IndexField(idx.helpers, doc.ID, field)
		if err != nil {
			return err
		}
		if entry != nil {
			revEntries = append(revEntries, *entry)
		}
	}

	// Store reverse index
	if len(revEntries) > 0 {
		riJSON, err := json.Marshal(revEntries)
		if err != nil {
			return fmt.Errorf("marshal reverse index: %w", err)
		}
		if err := idx.store.Set(context.Background(),revIdxKey(doc.ID), string(riJSON)); err != nil {
			return fmt.Errorf("store reverse index: %w", err)
		}
	}

	// Only increment document count for new documents
	if !existed {
		count, err := idx.getDocCount()
		if err != nil {
			return fmt.Errorf("get doc count: %w", err)
		}
		if err := idx.store.Set(context.Background(),keyDocCount, strconv.FormatUint(count+1, 10)); err != nil {
			return fmt.Errorf("update doc count: %w", err)
		}
	}

	// Invalidate caches and update in-memory indices for indexed fields.
	for _, entry := range revEntries {
		if entry.Type == "vector" {
			// Update HNSW index for vector fields (always, even in batch mode).
			for _, field := range doc.Fields {
				if field.Name == entry.Field {
					if vec, ok := field.VectorValue(); ok {
						hnsw, exists := idx.hnswIndices[entry.Field]
						if !exists {
							hnsw = NewHNSW(len(vec))
							idx.hnswIndices[entry.Field] = hnsw
						}
						hnsw.Insert(doc.ID, vec)
					}
					break
				}
			}
		}
		// Skip cache invalidation in batch mode — EndBatch will do it all at once.
		if !idx.batchMode && len(entry.Terms) > 0 {
			idx.cache.InvalidateFieldPrefix(entry.Field)
			for k := range idx.bkTrees {
				if len(k) >= len(entry.Field) && k[:len(entry.Field)] == entry.Field {
					delete(idx.bkTrees, k)
				}
			}
		}
	}

	return nil
}

// DeleteDocument removes a document from the index.
func (idx *Index) DeleteDocument(docID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	existed, err := idx.deleteDocInternal(docID)
	if err != nil {
		return err
	}
	if existed {
		count, err := idx.getDocCount()
		if err != nil {
			return fmt.Errorf("get doc count: %w", err)
		}
		if count > 0 {
			if err := idx.store.Set(context.Background(),keyDocCount, strconv.FormatUint(count-1, 10)); err != nil {
				return fmt.Errorf("update doc count: %w", err)
			}
		}
	}
	return nil
}

func (idx *Index) deleteDocInternal(docID string) (bool, error) {
	// Check if document exists
	_, err := idx.store.Get(context.Background(),docKey(docID))
	if err != nil {
		return false, nil
	}

	// Delete document data
	if err := deleteKey(idx.store, docKey(docID)); err != nil {
		return false, fmt.Errorf("delete document data: %w", err)
	}

	// Try targeted deletion using the reverse index
	riVal, riErr := idx.store.Get(context.Background(),revIdxKey(docID))
	if riErr == nil {
		var entries []RevIdxEntry
		if json.Unmarshal([]byte(riVal), &entries) == nil {
			if err := idx.deleteDocWithRevIdx(docID, entries); err != nil {
				return false, err
			}
			// Invalidate caches and in-memory indices.
			for _, entry := range entries {
				if len(entry.Terms) > 0 {
					idx.cache.InvalidateFieldPrefix(entry.Field)
					for k := range idx.bkTrees {
						if len(k) >= len(entry.Field) && k[:len(entry.Field)] == entry.Field {
							delete(idx.bkTrees, k)
						}
					}
				}
				if entry.Type == "vector" {
					if hnsw, ok := idx.hnswIndices[entry.Field]; ok {
						hnsw.Delete(docID)
					}
				}
			}
			if err := deleteKey(idx.store, revIdxKey(docID)); err != nil {
				return false, fmt.Errorf("delete reverse index: %w", err)
			}
			return true, nil
		}
	}

	// Fallback: legacy full-scan deletion for documents indexed before
	// the reverse index was added.
	if err := idx.deleteDocFullScan(docID); err != nil {
		return false, err
	}
	// Full-scan fallback: invalidate all caches since we don't know which fields changed.
	idx.cache.InvalidateAll()
	idx.bkTrees = make(map[string]*bkTree)
	for _, hnsw := range idx.hnswIndices {
		hnsw.Delete(docID)
	}
	return true, nil
}

// deleteDocWithRevIdx performs targeted deletion using field indexer plugins.
func (idx *Index) deleteDocWithRevIdx(docID string, entries []RevIdxEntry) error {
	for _, entry := range entries {
		if fi, ok := idx.fieldIndexers[entry.Type]; ok {
			if err := fi.DeleteField(idx.helpers, docID, entry); err != nil {
				return err
			}
		}
	}
	return nil
}

// forEachWithPrefix iterates over all keys with the given prefix in paginated
// batches of termListBatchSize, calling fn for each KV pair. It returns the
// first error from fn or from the underlying store.List call.
func (idx *Index) forEachWithPrefix(prefix string, fn func(key, value string) error) error {
	startAfter := ""
	for {
		results, err := idx.store.List(context.Background(),storemd.ListArgs{
			Prefix:     prefix,
			StartAfter: startAfter,
			Limit:      termListBatchSize,
		})
		if err != nil {
			return err
		}
		if len(results) == 0 {
			break
		}
		for _, kv := range results {
			if err := fn(kv.Key, kv.Value); err != nil {
				return err
			}
		}
		startAfter = results[len(results)-1].Key
		if len(results) < termListBatchSize {
			break
		}
	}
	return nil
}

// deleteDocFullScan is the legacy fallback for documents without a reverse index.
func (idx *Index) deleteDocFullScan(docID string) error {
	idx.logger.Warn("search.md: falling back to full-scan deletion for document %q (no reverse index found). This is slow for large indexes. Re-index this document to create a reverse index entry.", docID)
	suffix := "/" + docID

	// Delete all term postings for this doc
	if err := idx.forEachWithPrefix(prefixTerm, func(key, value string) error {
		if strings.HasSuffix(key, suffix) {
			parts := strings.SplitN(strings.TrimPrefix(key, prefixTerm), "/", 3)
			if len(parts) == 3 {
				idx.decrementDocFreq(parts[0], parts[1])
			}
			if err := deleteKey(idx.store, key); err != nil {
				return fmt.Errorf("delete term posting: %w", err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("list term postings: %w", err)
	}

	// Delete field lengths
	if err := idx.forEachWithPrefix(prefixFieldLen, func(key, value string) error {
		if strings.HasSuffix(key, suffix) {
			parts := strings.SplitN(strings.TrimPrefix(key, prefixFieldLen), "/", 2)
			if len(parts) == 2 {
				fieldLen, _ := strconv.Atoi(value)
				sumKey := fieldLenSumKey(parts[0])
				currentSum, _ := idx.getInt64(sumKey)
				newSum := currentSum - int64(fieldLen)
				if newSum < 0 {
					newSum = 0
				}
				if err := idx.store.Set(context.Background(),sumKey, strconv.FormatInt(newSum, 10)); err != nil {
					return fmt.Errorf("update field length sum: %w", err)
				}
			}
			if err := deleteKey(idx.store, key); err != nil {
				return fmt.Errorf("delete field length: %w", err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("list field lengths: %w", err)
	}

	// Delete term vectors (key format: tv/{field}/{docID}/{term})
	if err := idx.forEachWithPrefix(prefixTermVec, func(key, value string) error {
		parts := strings.TrimPrefix(key, prefixTermVec)
		slashIdx := strings.Index(parts, "/")
		if slashIdx >= 0 {
			rest := parts[slashIdx+1:]
			nextSlash := strings.Index(rest, "/")
			if nextSlash >= 0 {
				tvDocID := rest[:nextSlash]
				if tvDocID == docID {
					if err := deleteKey(idx.store, key); err != nil {
						return fmt.Errorf("delete term vector: %w", err)
					}
				}
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("list term vectors: %w", err)
	}

	// Delete vectors
	if err := idx.forEachWithPrefix(prefixVector, func(key, value string) error {
		if strings.HasSuffix(key, suffix) {
			if err := deleteKey(idx.store, key); err != nil {
				return fmt.Errorf("delete vector: %w", err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("list vectors: %w", err)
	}

	// Delete numeric values
	if err := idx.forEachWithPrefix(prefixNumeric, func(key, value string) error {
		if strings.HasSuffix(key, suffix) {
			if err := deleteKey(idx.store, key); err != nil {
				return fmt.Errorf("delete numeric value: %w", err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("list numeric values: %w", err)
	}
	// Delete sorted numeric index
	if err := idx.forEachWithPrefix(prefixNumericSorted, func(key, value string) error {
		if strings.HasSuffix(key, suffix) {
			if err := deleteKey(idx.store, key); err != nil {
				return fmt.Errorf("delete sorted numeric: %w", err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("list sorted numeric: %w", err)
	}

	// Delete datetime values
	if err := idx.forEachWithPrefix(prefixDateTime, func(key, value string) error {
		if strings.HasSuffix(key, suffix) {
			if err := deleteKey(idx.store, key); err != nil {
				return fmt.Errorf("delete datetime value: %w", err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("list datetime values: %w", err)
	}
	// Delete sorted datetime index
	if err := idx.forEachWithPrefix(prefixDateTimeSorted, func(key, value string) error {
		if strings.HasSuffix(key, suffix) {
			if err := deleteKey(idx.store, key); err != nil {
				return fmt.Errorf("delete sorted datetime: %w", err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("list sorted datetime: %w", err)
	}

	// Delete boolean values
	if err := idx.forEachWithPrefix(prefixBool, func(key, value string) error {
		if strings.HasSuffix(key, suffix) {
			if err := deleteKey(idx.store, key); err != nil {
				return fmt.Errorf("delete boolean value: %w", err)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("list boolean values: %w", err)
	}

	return nil
}

// GetDocument retrieves a stored document by ID.
func (idx *Index) GetDocument(docID string) (*document.StoredData, error) {
	// Fast path: check doc cache (in-memory mode).
	if fields, ok := idx.cache.GetCachedDoc(docID); ok {
		return &document.StoredData{Fields: fields}, nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	val, err := idx.store.Get(context.Background(),docKey(docID))
	if err != nil {
		return nil, fmt.Errorf("document not found: %s", docID)
	}
	sd, err := document.UnmarshalStoredData([]byte(val))
	if err != nil {
		return nil, err
	}

	// Cache if in-memory mode.
	if idx.inMemoryMode {
		idx.cache.SetCachedDoc(docID, sd.Fields)
	}

	return sd, nil
}

// DocCount returns the total number of indexed documents.
func (idx *Index) DocCount() (uint64, error) {
	// Fast path: check cache without index lock.
	if v, ok := idx.cache.GetDocCount(); ok {
		return uint64(v), nil
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	v, err := idx.getDocCount()
	if err != nil {
		return 0, err
	}
	idx.cache.SetDocCount(int64(v))
	return v, nil
}

func (idx *Index) getDocCount() (uint64, error) {
	val, err := idx.store.Get(context.Background(),keyDocCount)
	if err != nil {
		return 0, nil
	}
	count, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse doc count %q: %w", val, err)
	}
	return count, nil
}

// TermPostings retrieves all postings for a term in a field.
func (idx *Index) TermPostings(field, term string) ([]Posting, error) {
	// Fast path: check cache without index lock.
	if cached := idx.cache.GetPostings(field, term); cached != nil {
		return cached, nil
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.cachedTermPostings(field, term)
}

func (idx *Index) termPostings(field, term string) ([]Posting, error) {
	return idx.cachedTermPostings(field, term)
}

// termListBatchSize is the number of KV entries fetched per batch when
// paginating through term listings.
const termListBatchSize = 1000

// TermsForField returns all unique terms in a field.
// Uses the in-memory term dictionary cache to avoid repeated KV scans.
func (idx *Index) TermsForField(field string) ([]string, error) {
	// Fast path: check cache without index lock.
	if cached := idx.cache.GetTermDict(field); cached != nil {
		return cached, nil
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.cachedTermsForField(field)
}

// TermsWithPrefix returns all terms in a field starting with the given prefix.
// Uses the cached term dictionary with binary search for O(log N + results).
func (idx *Index) TermsWithPrefix(field, termPrefix string) ([]string, error) {
	// Fast path: check cache without index lock.
	allTerms := idx.cache.GetTermDict(field)
	if allTerms == nil {
		idx.mu.RLock()
		var err error
		allTerms, err = idx.cachedTermsForField(field)
		idx.mu.RUnlock()
		if err != nil {
			return nil, err
		}
	}

	// Binary search to find the start of matching terms.
	start := sort.SearchStrings(allTerms, termPrefix)
	var result []string
	for i := start; i < len(allTerms); i++ {
		if len(allTerms[i]) < len(termPrefix) || allTerms[i][:len(termPrefix)] != termPrefix {
			break
		}
		result = append(result, allTerms[i])
	}
	return result, nil
}


// DocumentFrequency returns the number of documents containing a term in a field.
func (idx *Index) DocumentFrequency(field, term string) (uint64, error) {
	// Fast path: check cache without index lock.
	if v, ok := idx.cache.GetDocFreq(field, term); ok {
		return v, nil
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	v, err := idx.docFreq(field, term)
	if err != nil {
		return 0, err
	}
	idx.cache.SetDocFreq(field, term, v)
	return v, nil
}

func (idx *Index) docFreq(field, term string) (uint64, error) {
	key := docFreqKey(field, term)
	val, err := idx.store.Get(context.Background(),key)
	if err != nil {
		return 0, nil
	}
	count, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse doc freq %q: %w", val, err)
	}
	return count, nil
}

func (idx *Index) incrementDocFreq(field, term string) error {
	key := docFreqKey(field, term)
	count, _ := idx.docFreq(field, term) // error intentionally ignored; 0 is acceptable fallback
	return idx.store.Set(context.Background(),key, strconv.FormatUint(count+1, 10))
}

func (idx *Index) decrementDocFreq(field, term string) error {
	key := docFreqKey(field, term)
	count, _ := idx.docFreq(field, term) // error intentionally ignored; 0 is acceptable fallback
	if count <= 1 {
		return deleteKey(idx.store, key)
	}
	return idx.store.Set(context.Background(),key, strconv.FormatUint(count-1, 10))
}

// FieldLength returns the length (token count) of a field in a document.
func (idx *Index) FieldLength(field, docID string) (int, error) {
	// Fast path: check cache without index lock.
	if v, ok := idx.cache.GetFieldLen(field, docID); ok {
		return v, nil
	}
	key := fieldLenKey(field, docID)
	val, err := idx.store.Get(context.Background(), key)
	if err != nil {
		idx.cache.SetFieldLen(field, docID, 0)
		return 0, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, err
	}
	idx.cache.SetFieldLen(field, docID, n)
	return n, nil
}

// BatchFieldLengths pre-fetches field lengths for a batch of docIDs into the cache.
// Subsequent FieldLength calls for these documents will be cache hits.
func (idx *Index) BatchFieldLengths(field string, docIDs []string) {
	// First pass: find which docIDs need fetching (under read lock).
	var missing []string
	for _, docID := range docIDs {
		if _, ok := idx.cache.GetFieldLen(field, docID); !ok {
			missing = append(missing, docID)
		}
	}
	if len(missing) == 0 {
		return
	}

	// Second pass: fetch from store.
	lengths := make(map[string]int, len(missing))
	for _, docID := range missing {
		key := fieldLenKey(field, docID)
		val, err := idx.store.Get(context.Background(), key)
		if err != nil {
			lengths[docID] = 0
			continue
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			lengths[docID] = 0
			continue
		}
		lengths[docID] = n
	}

	// Third pass: batch write to cache (single lock acquisition).
	idx.cache.BatchSetFieldLen(field, lengths)
}

// AverageFieldLength returns the average field length across all documents.
func (idx *Index) AverageFieldLength(field string) (float64, error) {
	// Fast path: check cache without index lock.
	if v, ok := idx.cache.GetAvgFieldLen(field); ok {
		return v, nil
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	v, err := idx.avgFieldLength(field)
	if err != nil {
		return 0, err
	}
	idx.cache.SetAvgFieldLen(field, v)
	return v, nil
}

func (idx *Index) avgFieldLength(field string) (float64, error) {
	sumKey := fieldLenSumKey(field)
	sum, err := idx.getInt64(sumKey)
	if err != nil {
		return 0, fmt.Errorf("get field length sum for %q: %w", field, err)
	}
	count, err := idx.getDocCount()
	if err != nil {
		return 0, fmt.Errorf("get doc count: %w", err)
	}
	if count == 0 {
		return 0, nil
	}
	return float64(sum) / float64(count), nil
}

func (idx *Index) getInt64(key string) (int64, error) {
	val, err := idx.store.Get(context.Background(),key)
	if err != nil {
		return 0, nil
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse int64 %q for key %q: %w", val, key, err)
	}
	return n, nil
}

// GetTermVector retrieves term vectors for a specific field/doc/term.
func (idx *Index) GetTermVector(field, docID, term string) (*TermVector, error) {
	// Fast path: check cache without index lock.
	if tv, ok := idx.cache.GetTermVec(field, docID, term); ok {
		return tv, nil
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	key := termVecKey(field, docID, term)
	val, err := idx.store.Get(context.Background(), key)
	if err != nil {
		return nil, err
	}
	var tv TermVector
	if err := json.Unmarshal([]byte(val), &tv); err != nil {
		return nil, err
	}
	idx.cache.SetTermVec(field, docID, term, &tv)
	return &tv, nil
}

// GetVector retrieves a stored vector for a field/doc.
func (idx *Index) GetVector(field, docID string) ([]float32, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	key := vectorKey(field, docID)
	val, err := idx.store.Get(context.Background(),key)
	if err != nil {
		return nil, err
	}
	var vec []float32
	if err := json.Unmarshal([]byte(val), &vec); err != nil {
		return nil, err
	}
	return vec, nil
}

// AllVectors retrieves all vectors for a field, returning docID -> vector.
func (idx *Index) AllVectors(field string) (map[string][]float32, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	prefix := vectorFieldPrefix(field)
	results, err := idx.store.List(context.Background(),storemd.ListArgs{Prefix: prefix})
	if err != nil {
		return nil, err
	}

	vectors := make(map[string][]float32)
	for _, kv := range results {
		docID := strings.TrimPrefix(kv.Key, prefix)
		var vec []float32
		if err := json.Unmarshal([]byte(kv.Value), &vec); err != nil {
			continue
		}
		vectors[docID] = vec
	}
	return vectors, nil
}

// ForEachVector streams vectors for a field in batches, calling fn for each one.
// If fn returns false, iteration stops. This avoids loading all vectors into memory.
func (idx *Index) ForEachVector(field string, fn func(docID string, vec []float32) bool) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	prefix := vectorFieldPrefix(field)
	const batchSize = 256
	startAfter := ""

	for {
		results, err := idx.store.List(context.Background(),storemd.ListArgs{
			Prefix:     prefix,
			StartAfter: startAfter,
			Limit:      batchSize,
		})
		if err != nil {
			return err
		}
		if len(results) == 0 {
			break
		}

		for _, kv := range results {
			docID := strings.TrimPrefix(kv.Key, prefix)
			var vec []float32
			if err := json.Unmarshal([]byte(kv.Value), &vec); err != nil {
				continue
			}
			if !fn(docID, vec) {
				return nil
			}
		}

		startAfter = results[len(results)-1].Key
		if len(results) < batchSize {
			break
		}
	}
	return nil
}

// ForEachDocID streams document IDs in batches, calling fn for each one.
// If fn returns false, iteration stops.
func (idx *Index) ForEachDocID(fn func(docID string) bool) error {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	const batchSize = 256
	startAfter := ""

	for {
		results, err := idx.store.List(context.Background(),storemd.ListArgs{
			Prefix:     prefixDoc,
			StartAfter: startAfter,
			Limit:      batchSize,
		})
		if err != nil {
			return err
		}
		if len(results) == 0 {
			break
		}

		for _, kv := range results {
			docID := strings.TrimPrefix(kv.Key, prefixDoc)
			if !fn(docID) {
				return nil
			}
		}

		startAfter = results[len(results)-1].Key
		if len(results) < batchSize {
			break
		}
	}
	return nil
}

// FuzzyTerms returns terms within the given edit distance using a BK-tree.
// The BK-tree is built lazily on first access per field and cached.
func (idx *Index) FuzzyTerms(field, term string, maxDist int) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	tree, ok := idx.bkTrees[field]
	if !ok {
		// Build BK-tree from the cached term dictionary.
		terms, err := idx.cachedTermsForField(field)
		if err != nil {
			return nil, err
		}
		tree = buildBKTree(terms)
		idx.bkTrees[field] = tree
	}
	return tree.search(term, maxDist), nil
}

// HNSWSearch performs approximate nearest neighbor search using the HNSW index.
// Returns docIDs and cosine similarities. Falls back to brute-force if no
// HNSW index exists for the field.
func (idx *Index) HNSWSearch(field string, query []float32, k int) ([]string, []float64, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	hnsw, ok := idx.hnswIndices[field]
	if !ok || hnsw.Size() == 0 {
		return nil, nil, false
	}
	ef := k * 10
	if ef < 100 {
		ef = 100
	}
	ids, sims := hnsw.Search(query, k, ef)
	return ids, sims, true
}

// NumericRange returns doc IDs with numeric values in [min, max] for a field.
// It uses the sorted numeric index (ns/ prefix) for efficient range scans with
// early termination, falling back to a full scan of the legacy (num/) prefix
// if the sorted index is empty.
func (idx *Index) NumericRange(field string, min, max *float64) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Try sorted index first.
	docIDs, err := idx.numericRangeSorted(field, min, max)
	if err != nil {
		return nil, err
	}
	if len(docIDs) > 0 {
		return docIDs, nil
	}
	// Fallback: legacy full scan.
	return idx.numericRangeLegacy(field, min, max)
}

func (idx *Index) numericRangeSorted(field string, min, max *float64) ([]string, error) {
	prefix := numericSortedFieldPrefix(field)
	// StartAfter lets us skip below min.
	startAfter := ""
	if min != nil {
		startAfter = numericSortedKeyBefore(field, *min)
	}
	maxKey := ""
	if max != nil {
		maxKey = numericSortedKeyAfter(field, *max)
	}

	var docIDs []string
	for {
		results, err := idx.store.List(context.Background(),storemd.ListArgs{
			Prefix:     prefix,
			StartAfter: startAfter,
			Limit:      termListBatchSize,
		})
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			break
		}
		for _, kv := range results {
			if maxKey != "" && kv.Key >= maxKey {
				return docIDs, nil // early termination
			}
			docID := numericSortedDocID(kv.Key)
			if docID != "" {
				docIDs = append(docIDs, docID)
			}
		}
		startAfter = results[len(results)-1].Key
		if len(results) < termListBatchSize {
			break
		}
	}
	return docIDs, nil
}

func (idx *Index) numericRangeLegacy(field string, min, max *float64) ([]string, error) {
	prefix := numericFieldPrefix(field)
	var docIDs []string
	startAfter := ""
	for {
		results, err := idx.store.List(context.Background(),storemd.ListArgs{
			Prefix:     prefix,
			StartAfter: startAfter,
			Limit:      termListBatchSize,
		})
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			break
		}
		for _, kv := range results {
			val, err := strconv.ParseFloat(kv.Value, 64)
			if err != nil {
				continue
			}
			if min != nil && val < *min {
				continue
			}
			if max != nil && val > *max {
				continue
			}
			docID := strings.TrimPrefix(kv.Key, prefix)
			docIDs = append(docIDs, docID)
		}
		startAfter = results[len(results)-1].Key
		if len(results) < termListBatchSize {
			break
		}
	}
	return docIDs, nil
}

// DateTimeRange returns doc IDs with datetime values in [start, end] for a field.
// It uses the sorted datetime index (ds/ prefix) for efficient range scans,
// falling back to the legacy (dt/) prefix if the sorted index is empty.
func (idx *Index) DateTimeRange(field string, start, end *time.Time) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Try sorted index first.
	docIDs, err := idx.dateTimeRangeSorted(field, start, end)
	if err != nil {
		return nil, err
	}
	if len(docIDs) > 0 {
		return docIDs, nil
	}
	// Fallback: legacy full scan.
	return idx.dateTimeRangeLegacy(field, start, end)
}

func (idx *Index) dateTimeRangeSorted(field string, start, end *time.Time) ([]string, error) {
	prefix := dateTimeSortedFieldPrefix(field)
	startAfter := ""
	if start != nil {
		startAfter = dateTimeSortedKeyBefore(field, *start)
	}
	maxKey := ""
	if end != nil {
		maxKey = dateTimeSortedKeyAfter(field, *end)
	}

	var docIDs []string
	for {
		results, err := idx.store.List(context.Background(),storemd.ListArgs{
			Prefix:     prefix,
			StartAfter: startAfter,
			Limit:      termListBatchSize,
		})
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			break
		}
		for _, kv := range results {
			if maxKey != "" && kv.Key >= maxKey {
				return docIDs, nil
			}
			docID := dateTimeSortedDocID(kv.Key)
			if docID != "" {
				docIDs = append(docIDs, docID)
			}
		}
		startAfter = results[len(results)-1].Key
		if len(results) < termListBatchSize {
			break
		}
	}
	return docIDs, nil
}

func (idx *Index) dateTimeRangeLegacy(field string, start, end *time.Time) ([]string, error) {
	prefix := dateTimeFieldPrefix(field)
	var docIDs []string
	startAfter := ""
	for {
		results, err := idx.store.List(context.Background(),storemd.ListArgs{
			Prefix:     prefix,
			StartAfter: startAfter,
			Limit:      termListBatchSize,
		})
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			break
		}
		for _, kv := range results {
			nanos, err := strconv.ParseInt(kv.Value, 10, 64)
			if err != nil {
				continue
			}
			t := time.Unix(0, nanos)
			if start != nil && t.Before(*start) {
				continue
			}
			if end != nil && t.After(*end) {
				continue
			}
			docID := strings.TrimPrefix(kv.Key, prefix)
			docIDs = append(docIDs, docID)
		}
		startAfter = results[len(results)-1].Key
		if len(results) < termListBatchSize {
			break
		}
	}
	return docIDs, nil
}

// AllDocIDs returns all document IDs in the index.
func (idx *Index) AllDocIDs() ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	results, err := idx.store.List(context.Background(),storemd.ListArgs{Prefix: prefixDoc})
	if err != nil {
		return nil, err
	}

	docIDs := make([]string, 0, len(results))
	for _, kv := range results {
		id := strings.TrimPrefix(kv.Key, prefixDoc)
		docIDs = append(docIDs, id)
	}
	return docIDs, nil
}

// ListDocIDs returns a page of document IDs for paginated iteration.
// startAfter is a doc ID cursor (e.g., "lastDocID"); pass "" for the first page.
func (idx *Index) ListDocIDs(startAfter string, limit int) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	storeStartAfter := ""
	if startAfter != "" {
		storeStartAfter = docKey(startAfter)
	}

	results, err := idx.store.List(context.Background(),storemd.ListArgs{
		Prefix:     prefixDoc,
		StartAfter: storeStartAfter,
		Limit:      limit,
	})
	if err != nil {
		return nil, err
	}

	docIDs := make([]string, 0, len(results))
	for _, kv := range results {
		id := strings.TrimPrefix(kv.Key, prefixDoc)
		docIDs = append(docIDs, id)
	}
	return docIDs, nil
}

// GetNumericValue returns the numeric value for a field in a document.
func (idx *Index) GetNumericValue(field, docID string) (float64, bool) {
	key := numericKey(field, docID)
	val, err := idx.store.Get(context.Background(),key)
	if err != nil {
		return 0, false
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// GetDateTimeValue returns the datetime value for a field in a document.
func (idx *Index) GetDateTimeValue(field, docID string) (time.Time, bool) {
	key := dateTimeKey(field, docID)
	val, err := idx.store.Get(context.Background(),key)
	if err != nil {
		return time.Time{}, false
	}
	nanos, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, nanos), true
}
