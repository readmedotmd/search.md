package index

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"

	storemd "github.com/readmedotmd/store.md"
)

// Cache provides in-memory caches for term dictionaries, posting lists,
// field lengths, document frequencies, and aggregate statistics.
// It is embedded in Index and invalidated on writes.
type Cache struct {
	mu sync.RWMutex

	// termDict caches sorted term lists per field.
	// Key: field name, Value: sorted []string of terms.
	termDict map[string][]string

	// postings caches posting lists per field+term.
	// Key: "field\x00term", Value: []Posting.
	postings    map[string][]Posting
	postingsLRU []string // LRU order (most recent at end)
	maxPostings int      // max entries in postings cache

	// fieldLens caches field lengths per field per document.
	// Key: field name, Value: map[docID]fieldLen.
	fieldLens map[string]map[string]int
	// fieldLensLoaded tracks which fields have had all lengths bulk-loaded.
	fieldLensLoaded map[string]bool

	// docFreqs caches document frequencies per field+term.
	// Key: "field\x00term", Value: docFreq.
	docFreqs map[string]uint64

	// avgFieldLen caches average field length per field.
	avgFieldLen map[string]float64

	// docCount caches the total document count. -1 means not cached.
	docCount int64

	// termVecs caches term vectors per field+docID+term.
	termVecs map[string]*TermVector

	// docs caches unmarshaled stored documents. Only populated in in-memory mode.
	docs       map[string]*cachedDoc
	docsLoaded bool // true when all documents have been cached
}

type cachedDoc struct {
	fields map[string]interface{}
}

const defaultMaxPostings = 1024

func newCache() *Cache {
	return &Cache{
		termDict:        make(map[string][]string),
		postings:        make(map[string][]Posting),
		maxPostings:     defaultMaxPostings,
		fieldLens:       make(map[string]map[string]int),
		fieldLensLoaded: make(map[string]bool),
		docFreqs:        make(map[string]uint64),
		avgFieldLen:     make(map[string]float64),
		docCount:        -1,
		termVecs:        make(map[string]*TermVector),
		docs:            make(map[string]*cachedDoc),
	}
}

// SetUnlimitedPostings removes the LRU eviction limit on the postings cache.
func (c *Cache) SetUnlimitedPostings() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxPostings = 0 // 0 means unlimited
}

// SetMaxPostings sets the maximum number of entries in the postings cache.
func (c *Cache) SetMaxPostings(max int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxPostings = max
}

func postingsCacheKey(field, term string) string {
	return field + "\x00" + term
}

// GetTermDict returns a cached sorted term list for a field, or nil if not cached.
func (c *Cache) GetTermDict(field string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.termDict[field]
}

// SetTermDict caches a sorted term list for a field.
func (c *Cache) SetTermDict(field string, terms []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.termDict[field] = terms
}

// GetPostings returns cached postings for a field+term, or nil if not cached.
func (c *Cache) GetPostings(field, term string) []Posting {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := postingsCacheKey(field, term)
	return c.postings[key]
}

// SetPostings caches postings for a field+term with LRU eviction.
func (c *Cache) SetPostings(field, term string, postings []Posting) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := postingsCacheKey(field, term)
	if _, exists := c.postings[key]; !exists {
		c.postingsLRU = append(c.postingsLRU, key)
		// Evict oldest if over capacity (skip when maxPostings==0 i.e. unlimited).
		if c.maxPostings > 0 {
			for len(c.postingsLRU) > c.maxPostings {
				evict := c.postingsLRU[0]
				c.postingsLRU = c.postingsLRU[1:]
				delete(c.postings, evict)
			}
		}
	}
	c.postings[key] = postings
}

// GetFieldLen returns a cached field length, or (0, false) if not cached.
func (c *Cache) GetFieldLen(field, docID string) (int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if m, ok := c.fieldLens[field]; ok {
		v, found := m[docID]
		return v, found
	}
	return 0, false
}

// SetFieldLen caches a field length for a document.
func (c *Cache) SetFieldLen(field, docID string, length int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.fieldLens[field]
	if !ok {
		m = make(map[string]int)
		c.fieldLens[field] = m
	}
	m[docID] = length
}

// BatchSetFieldLen sets multiple field lengths under a single lock acquisition.
func (c *Cache) BatchSetFieldLen(field string, lengths map[string]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.fieldLens[field]
	if !ok {
		m = make(map[string]int, len(lengths))
		c.fieldLens[field] = m
	}
	for docID, length := range lengths {
		m[docID] = length
	}
}

// IsFieldLensLoaded returns whether all field lengths for a field have been bulk-loaded.
func (c *Cache) IsFieldLensLoaded(field string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fieldLensLoaded[field]
}

// SetFieldLensAll bulk-sets all field lengths for a field and marks it as fully loaded.
func (c *Cache) SetFieldLensAll(field string, lengths map[string]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fieldLens[field] = lengths
	c.fieldLensLoaded[field] = true
}

// GetTermVec returns a cached term vector, or nil if not cached.
func (c *Cache) GetTermVec(field, docID, term string) (*TermVector, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := field + "\x00" + docID + "\x00" + term
	v, ok := c.termVecs[key]
	return v, ok
}

// SetTermVec caches a term vector.
func (c *Cache) SetTermVec(field, docID, term string, tv *TermVector) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := field + "\x00" + docID + "\x00" + term
	c.termVecs[key] = tv
}

// GetDocFreq returns a cached document frequency, or (0, false) if not cached.
func (c *Cache) GetDocFreq(field, term string) (uint64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := postingsCacheKey(field, term)
	v, ok := c.docFreqs[key]
	return v, ok
}

// SetDocFreq caches a document frequency.
func (c *Cache) SetDocFreq(field, term string, freq uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := postingsCacheKey(field, term)
	c.docFreqs[key] = freq
}

// GetAvgFieldLen returns a cached average field length, or (0, false) if not cached.
func (c *Cache) GetAvgFieldLen(field string) (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.avgFieldLen[field]
	return v, ok
}

// SetAvgFieldLen caches an average field length.
func (c *Cache) SetAvgFieldLen(field string, avg float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.avgFieldLen[field] = avg
}

// GetDocCount returns the cached document count, or (-1, false) if not cached.
func (c *Cache) GetDocCount() (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.docCount >= 0 {
		return c.docCount, true
	}
	return -1, false
}

// SetDocCount caches the document count.
func (c *Cache) SetDocCount(count int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.docCount = count
}

// TermSearcherState holds all cached state needed by a term searcher,
// fetched in a single lock acquisition to reduce contention.
type TermSearcherState struct {
	Postings    []Posting
	DocFreq     uint64
	AvgFieldLen float64
	DocCount    int64
	PostingsOK  bool
	DocFreqOK   bool
	AvgFieldOK  bool
	DocCountOK  bool
}

// GetTermSearcherState fetches postings, docFreq, avgFieldLen, and docCount
// for a field+term under a single read lock to minimize lock contention.
func (c *Cache) GetTermSearcherState(field, term string) TermSearcherState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := postingsCacheKey(field, term)
	var st TermSearcherState
	st.Postings = c.postings[key]
	st.PostingsOK = st.Postings != nil
	st.DocFreq, st.DocFreqOK = c.docFreqs[key]
	st.AvgFieldLen, st.AvgFieldOK = c.avgFieldLen[field]
	if c.docCount >= 0 {
		st.DocCount = c.docCount
		st.DocCountOK = true
	}
	return st
}

// GetFieldLenBatch retrieves field lengths for multiple docIDs under a single lock.
// Returns a map and a bool indicating if all requested docIDs were found in cache.
// When all field lengths are bulk-loaded, returns the inner map directly (no copy).
func (c *Cache) GetFieldLenBatch(field string, docIDs []string) (map[string]int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.fieldLens[field]
	if !ok {
		return nil, false
	}
	// If bulk-loaded, return the inner map directly — no allocation needed.
	if c.fieldLensLoaded[field] {
		return m, true
	}
	result := make(map[string]int, len(docIDs))
	allFound := true
	for _, docID := range docIDs {
		if v, found := m[docID]; found {
			result[docID] = v
		} else {
			allFound = false
		}
	}
	return result, allFound
}

// GetCachedDoc returns a cached document's fields, or nil if not cached.
func (c *Cache) GetCachedDoc(docID string) (map[string]interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.docs[docID]
	if !ok {
		return nil, false
	}
	return d.fields, true
}

// SetCachedDoc caches a document's fields.
func (c *Cache) SetCachedDoc(docID string, fields map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.docs[docID] = &cachedDoc{fields: fields}
}

// InvalidateDoc removes a cached document.
func (c *Cache) InvalidateDoc(docID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.docs, docID)
	c.docsLoaded = false
}

// InvalidateField clears all caches for a field (called on writes).
func (c *Cache) InvalidateField(field string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.termDict, field)
	delete(c.fieldLens, field)
	delete(c.fieldLensLoaded, field)
	delete(c.avgFieldLen, field)
	c.docCount = -1
	// Remove all postings, docFreqs, and termVecs for this field.
	prefix := field + "\x00"
	newLRU := c.postingsLRU[:0]
	for _, key := range c.postingsLRU {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.postings, key)
			delete(c.docFreqs, key)
		} else {
			newLRU = append(newLRU, key)
		}
	}
	c.postingsLRU = newLRU
	for key := range c.termVecs {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.termVecs, key)
		}
	}
}

// InvalidateFieldPrefix clears all caches for fields that start with prefix.
// This handles plugin fields that create sub-fields (e.g., "symbols.sym", "symbols.kind").
func (c *Cache) InvalidateFieldPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.docCount = -1
	for field := range c.termDict {
		if len(field) >= len(prefix) && field[:len(prefix)] == prefix {
			delete(c.termDict, field)
		}
	}
	for field := range c.fieldLens {
		if len(field) >= len(prefix) && field[:len(prefix)] == prefix {
			delete(c.fieldLens, field)
			delete(c.fieldLensLoaded, field)
		}
	}
	for field := range c.avgFieldLen {
		if len(field) >= len(prefix) && field[:len(prefix)] == prefix {
			delete(c.avgFieldLen, field)
		}
	}
	newLRU := c.postingsLRU[:0]
	for _, key := range c.postingsLRU {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.postings, key)
			delete(c.docFreqs, key)
		} else {
			newLRU = append(newLRU, key)
		}
	}
	c.postingsLRU = newLRU
	for key := range c.termVecs {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.termVecs, key)
		}
	}
}

// InvalidateAll clears all caches (called on batch operations).
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.termDict = make(map[string][]string)
	c.postings = make(map[string][]Posting)
	c.postingsLRU = c.postingsLRU[:0]
	c.fieldLens = make(map[string]map[string]int)
	c.fieldLensLoaded = make(map[string]bool)
	c.docFreqs = make(map[string]uint64)
	c.avgFieldLen = make(map[string]float64)
	c.docCount = -1
	c.termVecs = make(map[string]*TermVector)
	c.docs = make(map[string]*cachedDoc)
	c.docsLoaded = false
}

// --- Integration with Index ---

// cachedTermsForField returns terms from cache or loads from store and caches.
func (idx *Index) cachedTermsForField(field string) ([]string, error) {
	if cached := idx.cache.GetTermDict(field); cached != nil {
		return cached, nil
	}

	// Load from store.
	prefix := termFieldOnlyPrefix(field)
	termSet := make(map[string]struct{})
	startAfter := ""

	for {
		results, err := idx.store.List(context.Background(), storemd.ListArgs{
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
			rest := kv.Key[len(prefix):]
			slashIdx := lastIndexByte(rest, '/')
			if slashIdx > 0 {
				termSet[rest[:slashIdx]] = struct{}{}
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

	idx.cache.SetTermDict(field, terms)
	return terms, nil
}

// cachedTermPostings returns postings from cache or loads from store and caches.
func (idx *Index) cachedTermPostings(field, term string) ([]Posting, error) {
	if cached := idx.cache.GetPostings(field, term); cached != nil {
		return cached, nil
	}

	prefix := termFieldPrefix(field, term)
	results, err := idx.store.List(context.Background(), storemd.ListArgs{Prefix: prefix})
	if err != nil {
		return nil, err
	}

	postings := make([]Posting, 0, len(results))
	for _, kv := range results {
		p, ok := fastParsePosting(kv.Value)
		if !ok {
			// Fallback to JSON for non-standard formats.
			if err := json.Unmarshal([]byte(kv.Value), &p); err != nil {
				continue
			}
		}
		postings = append(postings, p)
	}

	idx.cache.SetPostings(field, term, postings)
	return postings, nil
}

// fastParsePosting parses the compact JSON posting format {"d":"...","f":N,"n":F}
// without using encoding/json reflection. Returns (Posting, true) on success.
func fastParsePosting(s string) (Posting, bool) {
	// Find "d":" to extract DocID.
	dIdx := strings.Index(s, `"d":"`)
	if dIdx < 0 {
		return Posting{}, false
	}
	rest := s[dIdx+5:]
	endQuote := strings.IndexByte(rest, '"')
	if endQuote < 0 {
		return Posting{}, false
	}
	docID := rest[:endQuote]

	// Find "f": to extract frequency.
	fIdx := strings.Index(s, `"f":`)
	if fIdx < 0 {
		return Posting{}, false
	}
	rest = s[fIdx+4:]
	// Frequency ends at , or }
	endF := strings.IndexAny(rest, ",}")
	if endF < 0 {
		return Posting{}, false
	}
	freq, err := strconv.Atoi(strings.TrimSpace(rest[:endF]))
	if err != nil {
		return Posting{}, false
	}

	// Find "n": to extract norm.
	nIdx := strings.Index(s, `"n":`)
	if nIdx < 0 {
		return Posting{}, false
	}
	rest = s[nIdx+4:]
	endN := strings.IndexAny(rest, ",}")
	if endN < 0 {
		return Posting{}, false
	}
	norm, err := strconv.ParseFloat(strings.TrimSpace(rest[:endN]), 64)
	if err != nil {
		return Posting{}, false
	}

	return Posting{DocID: docID, Frequency: freq, Norm: norm}, true
}

// lastIndexByte returns the index of the last occurrence of c in s, or -1.
func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
