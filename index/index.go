package index

import (
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
	// dt/{field}/{docID} -> datetime value (unix nano)
	prefixDateTime = "dt/"
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
	}
	idx.helpers = &indexHelpersImpl{idx: idx}

	// Register built-in field indexers
	idx.RegisterFieldIndexer(&TextFieldIndexer{})
	idx.RegisterFieldIndexer(&NumericFieldIndexer{})
	idx.RegisterFieldIndexer(&BooleanFieldIndexer{})
	idx.RegisterFieldIndexer(&DateTimeFieldIndexer{})
	idx.RegisterFieldIndexer(&VectorFieldIndexer{})

	// Initialize index version if not already set, or check for mismatch.
	storedVersion, err := store.Get(keyIndexVersion)
	if err != nil {
		if err := store.Set(keyIndexVersion, currentIndexVersion); err != nil {
			return nil, fmt.Errorf("set index version: %w", err)
		}
	} else if storedVersion != currentIndexVersion {
		return nil, fmt.Errorf("index version mismatch: stored %q, expected %q", storedVersion, currentIndexVersion)
	}

	return idx, nil
}

// IndexVersion returns the stored index format version.
func (idx *Index) IndexVersion() string {
	val, err := idx.store.Get(keyIndexVersion)
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

// IndexDocument indexes a document's fields into the store.
func (idx *Index) IndexDocument(doc *document.Document) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// First, delete old data if document exists
	existed, err := idx.deleteDocInternal(doc.ID)
	if err != nil {
		return fmt.Errorf("delete existing document: %w", err)
	}

	// Store the document data
	storedData := doc.ToStoredData()
	docJSON, err := storedData.Marshal()
	if err != nil {
		return fmt.Errorf("marshal document: %w", err)
	}
	if err := idx.store.Set(prefixDoc+doc.ID, string(docJSON)); err != nil {
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
		if err := idx.store.Set(prefixRevIdx+doc.ID, string(riJSON)); err != nil {
			return fmt.Errorf("store reverse index: %w", err)
		}
	}

	// Only increment document count for new documents
	if !existed {
		count, _ := idx.getDocCount() // error intentionally ignored; 0 is acceptable fallback
		if err := idx.store.Set(keyDocCount, strconv.FormatUint(count+1, 10)); err != nil {
			return fmt.Errorf("update doc count: %w", err)
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
		count, _ := idx.getDocCount() // error intentionally ignored; 0 is acceptable fallback
		if count > 0 {
			if err := idx.store.Set(keyDocCount, strconv.FormatUint(count-1, 10)); err != nil {
				return fmt.Errorf("update doc count: %w", err)
			}
		}
	}
	return nil
}

func (idx *Index) deleteDocInternal(docID string) (bool, error) {
	// Check if document exists
	_, err := idx.store.Get(prefixDoc + docID)
	if err != nil {
		return false, nil
	}

	// Delete document data
	if err := deleteKey(idx.store, prefixDoc+docID); err != nil {
		return false, fmt.Errorf("delete document data: %w", err)
	}

	// Try targeted deletion using the reverse index
	riVal, riErr := idx.store.Get(prefixRevIdx + docID)
	if riErr == nil {
		var entries []RevIdxEntry
		if json.Unmarshal([]byte(riVal), &entries) == nil {
			if err := idx.deleteDocWithRevIdx(docID, entries); err != nil {
				return false, err
			}
			if err := deleteKey(idx.store, prefixRevIdx+docID); err != nil {
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
		results, err := idx.store.List(storemd.ListArgs{
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
				sumKey := prefixFieldLenSum + parts[0]
				currentSum, _ := idx.getInt64(sumKey) // error intentionally ignored; 0 is acceptable fallback
				newSum := currentSum - int64(fieldLen)
				if newSum < 0 {
					newSum = 0
				}
				if err := idx.store.Set(sumKey, strconv.FormatInt(newSum, 10)); err != nil {
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
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	val, err := idx.store.Get(prefixDoc + docID)
	if err != nil {
		return nil, fmt.Errorf("document not found: %s", docID)
	}
	return document.UnmarshalStoredData([]byte(val))
}

// DocCount returns the total number of indexed documents.
func (idx *Index) DocCount() (uint64, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.getDocCount()
}

func (idx *Index) getDocCount() (uint64, error) {
	val, err := idx.store.Get(keyDocCount)
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
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.termPostings(field, term)
}

func (idx *Index) termPostings(field, term string) ([]Posting, error) {
	prefix := prefixTerm + field + "/" + term + "/"
	results, err := idx.store.List(storemd.ListArgs{Prefix: prefix})
	if err != nil {
		return nil, err
	}

	postings := make([]Posting, 0, len(results))
	for _, kv := range results {
		var p Posting
		if err := json.Unmarshal([]byte(kv.Value), &p); err != nil {
			continue
		}
		postings = append(postings, p)
	}
	return postings, nil
}

// termListBatchSize is the number of KV entries fetched per batch when
// paginating through term listings.
const termListBatchSize = 1000

// TermsForField returns all unique terms in a field.
func (idx *Index) TermsForField(field string) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	prefix := prefixTerm + field + "/"
	termSet := make(map[string]struct{})
	startAfter := ""

	for {
		results, err := idx.store.List(storemd.ListArgs{
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
			// Key: t/{field}/{term}/{docID}
			rest := strings.TrimPrefix(kv.Key, prefix)
			slashIdx := strings.LastIndex(rest, "/")
			if slashIdx > 0 {
				term := rest[:slashIdx]
				termSet[term] = struct{}{}
			}
		}

		startAfter = results[len(results)-1].Key
		if len(results) < termListBatchSize {
			break
		}
	}

	terms := make([]string, 0, len(termSet))
	for term := range termSet {
		terms = append(terms, term)
	}
	sort.Strings(terms)
	return terms, nil
}

// TermsWithPrefix returns all terms in a field starting with the given prefix.
func (idx *Index) TermsWithPrefix(field, termPrefix string) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	prefix := prefixTerm + field + "/" + termPrefix
	termSet := make(map[string]struct{})
	fieldPrefix := prefixTerm + field + "/"
	startAfter := ""

	for {
		results, err := idx.store.List(storemd.ListArgs{
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
			rest := strings.TrimPrefix(kv.Key, fieldPrefix)
			slashIdx := strings.LastIndex(rest, "/")
			if slashIdx > 0 {
				term := rest[:slashIdx]
				termSet[term] = struct{}{}
			}
		}

		startAfter = results[len(results)-1].Key
		if len(results) < termListBatchSize {
			break
		}
	}

	terms := make([]string, 0, len(termSet))
	for term := range termSet {
		terms = append(terms, term)
	}
	sort.Strings(terms)
	return terms, nil
}

// DocumentFrequency returns the number of documents containing a term in a field.
func (idx *Index) DocumentFrequency(field, term string) (uint64, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.docFreq(field, term)
}

func (idx *Index) docFreq(field, term string) (uint64, error) {
	key := prefixDocFreq + field + "/" + term
	val, err := idx.store.Get(key)
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
	key := prefixDocFreq + field + "/" + term
	count, _ := idx.docFreq(field, term) // error intentionally ignored; 0 is acceptable fallback
	return idx.store.Set(key, strconv.FormatUint(count+1, 10))
}

func (idx *Index) decrementDocFreq(field, term string) error {
	key := prefixDocFreq + field + "/" + term
	count, _ := idx.docFreq(field, term) // error intentionally ignored; 0 is acceptable fallback
	if count <= 1 {
		return deleteKey(idx.store, key)
	}
	return idx.store.Set(key, strconv.FormatUint(count-1, 10))
}

// FieldLength returns the length (token count) of a field in a document.
func (idx *Index) FieldLength(field, docID string) (int, error) {
	key := prefixFieldLen + field + "/" + docID
	val, err := idx.store.Get(key)
	if err != nil {
		return 0, nil
	}
	return strconv.Atoi(val)
}

// AverageFieldLength returns the average field length across all documents.
func (idx *Index) AverageFieldLength(field string) (float64, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.avgFieldLength(field)
}

func (idx *Index) avgFieldLength(field string) (float64, error) {
	sumKey := prefixFieldLenSum + field
	sum, _ := idx.getInt64(sumKey) // error intentionally ignored; 0 is acceptable fallback
	count, _ := idx.getDocCount()  // error intentionally ignored; 0 is acceptable fallback
	if count == 0 {
		return 0, nil
	}
	return float64(sum) / float64(count), nil
}

func (idx *Index) getInt64(key string) (int64, error) {
	val, err := idx.store.Get(key)
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
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	key := prefixTermVec + field + "/" + docID + "/" + term
	val, err := idx.store.Get(key)
	if err != nil {
		return nil, err
	}
	var tv TermVector
	if err := json.Unmarshal([]byte(val), &tv); err != nil {
		return nil, err
	}
	return &tv, nil
}

// GetVector retrieves a stored vector for a field/doc.
func (idx *Index) GetVector(field, docID string) ([]float32, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	key := prefixVector + field + "/" + docID
	val, err := idx.store.Get(key)
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

	prefix := prefixVector + field + "/"
	results, err := idx.store.List(storemd.ListArgs{Prefix: prefix})
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

	prefix := prefixVector + field + "/"
	const batchSize = 256
	startAfter := ""

	for {
		results, err := idx.store.List(storemd.ListArgs{
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
		results, err := idx.store.List(storemd.ListArgs{
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

// NumericRange returns doc IDs with numeric values in [min, max] for a field.
func (idx *Index) NumericRange(field string, min, max *float64) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	prefix := prefixNumeric + field + "/"
	var docIDs []string
	startAfter := ""
	for {
		results, err := idx.store.List(storemd.ListArgs{
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
func (idx *Index) DateTimeRange(field string, start, end *time.Time) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	prefix := prefixDateTime + field + "/"
	var docIDs []string
	startAfter := ""
	for {
		results, err := idx.store.List(storemd.ListArgs{
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

	results, err := idx.store.List(storemd.ListArgs{Prefix: prefixDoc})
	if err != nil {
		return nil, err
	}

	docIDs := make([]string, 0, len(results))
	for _, kv := range results {
		docIDs = append(docIDs, strings.TrimPrefix(kv.Key, prefixDoc))
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
		storeStartAfter = prefixDoc + startAfter
	}

	results, err := idx.store.List(storemd.ListArgs{
		Prefix:     prefixDoc,
		StartAfter: storeStartAfter,
		Limit:      limit,
	})
	if err != nil {
		return nil, err
	}

	docIDs := make([]string, 0, len(results))
	for _, kv := range results {
		docIDs = append(docIDs, strings.TrimPrefix(kv.Key, prefixDoc))
	}
	return docIDs, nil
}

// GetNumericValue returns the numeric value for a field in a document.
func (idx *Index) GetNumericValue(field, docID string) (float64, bool) {
	key := prefixNumeric + field + "/" + docID
	val, err := idx.store.Get(key)
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
	key := prefixDateTime + field + "/" + docID
	val, err := idx.store.Get(key)
	if err != nil {
		return time.Time{}, false
	}
	nanos, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, nanos), true
}
