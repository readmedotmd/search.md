package index

import (
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

	// docFreqs caches document frequencies per field+term.
	// Key: "field\x00term", Value: docFreq.
	docFreqs map[string]uint64

	// avgFieldLen caches average field length per field.
	avgFieldLen map[string]float64

	// docCount caches the total document count. -1 means not cached.
	docCount int64
}

const defaultMaxPostings = 1024

func newCache() *Cache {
	return &Cache{
		termDict:    make(map[string][]string),
		postings:    make(map[string][]Posting),
		maxPostings: defaultMaxPostings,
		fieldLens:   make(map[string]map[string]int),
		docFreqs:    make(map[string]uint64),
		avgFieldLen: make(map[string]float64),
		docCount:    -1,
	}
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
		// Evict oldest if over capacity.
		for len(c.postingsLRU) > c.maxPostings {
			evict := c.postingsLRU[0]
			c.postingsLRU = c.postingsLRU[1:]
			delete(c.postings, evict)
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

// InvalidateField clears all caches for a field (called on writes).
func (c *Cache) InvalidateField(field string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.termDict, field)
	delete(c.fieldLens, field)
	delete(c.avgFieldLen, field)
	c.docCount = -1
	// Remove all postings and docFreqs for this field.
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
}

// InvalidateAll clears all caches (called on batch operations).
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.termDict = make(map[string][]string)
	c.postings = make(map[string][]Posting)
	c.postingsLRU = c.postingsLRU[:0]
	c.fieldLens = make(map[string]map[string]int)
	c.docFreqs = make(map[string]uint64)
	c.avgFieldLen = make(map[string]float64)
	c.docCount = -1
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
	results, err := idx.store.List(storemd.ListArgs{Prefix: prefix})
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
