// Package searchmd provides a full-text search engine backed by store.md.
//
// It supports normal text search (BM25 scoring), vector/semantic search (KNN),
// code search (syntax-aware tokenization), and multiple query types including
// term, match, phrase, fuzzy, wildcard, regexp, boolean, numeric range, and
// date range queries.
//
// The entire system is modular via the plugin package. Scoring, highlighting,
// faceting, and query types can all be replaced or extended.
//
// Basic usage:
//
//	store, _ := bbolt.New("my.db")
//	idx, _ := searchmd.New(store, nil)
//
//	idx.Index("doc1", map[string]interface{}{
//	    "title":   "Hello World",
//	    "content": "This is a test document",
//	})
//
//	results, _ := idx.Search(ctx, query.NewMatchQuery("hello").SetField("title"))
package searchmd

import (
	"container/heap"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	storemd "github.com/readmedotmd/store.md"
	"github.com/readmedotmd/store.md/sync/cache"

	"github.com/readmedotmd/search.md/document"
	"github.com/readmedotmd/search.md/index"
	"github.com/readmedotmd/search.md/mapping"
	"github.com/readmedotmd/search.md/plugin"
	"github.com/readmedotmd/search.md/search"
	"github.com/readmedotmd/search.md/search/highlight"
	"github.com/readmedotmd/search.md/search/query"

	// Import registry for side effects (registers default analyzers).
	_ "github.com/readmedotmd/search.md/registry"
)

// scoreHeap is a min-heap of DocumentMatch by score, used to keep only the
// top-N highest scoring documents in bounded memory.
type scoreHeap []*search.DocumentMatch

func (h scoreHeap) Len() int            { return len(h) }
func (h scoreHeap) Less(i, j int) bool  { return h[i].Score < h[j].Score } // min-heap
func (h scoreHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *scoreHeap) Push(x interface{}) { *h = append(*h, x.(*search.DocumentMatch)) }
func (h *scoreHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

// SearchIndex is the main search index.
type SearchIndex struct {
	store   storemd.Store
	idx     *index.Index
	reader  plugin.IndexReader
	mapping *mapping.IndexMapping
	plugins *plugin.Registry
	mu      sync.RWMutex
}

// New creates a new SearchIndex backed by the given store.
// If indexMapping is nil, a default mapping is used.
// Optional plugin.Option values configure scoring, highlighting, etc.
func New(store storemd.Store, indexMapping *mapping.IndexMapping, opts ...plugin.Option) (*SearchIndex, error) {
	if indexMapping == nil {
		indexMapping = mapping.NewIndexMapping()
	}
	if err := indexMapping.Validate(); err != nil {
		return nil, fmt.Errorf("invalid mapping: %w", err)
	}

	// Wrap store with in-memory cache to reduce repeated reads.
	cached := cache.New(store)

	idx, err := index.New(cached)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	reg := plugin.NewRegistry()

	// Set defaults
	reg.SetScorerFactory(plugin.DefaultBM25())
	reg.SetHighlighterFactory(plugin.DefaultHTMLHighlighter())
	reg.SetFacetBuilderFactory(&plugin.DefaultFacetBuilderFactory{})

	// Apply user options (can override defaults)
	reg.Apply(opts...)

	// Register any custom field indexers on the underlying index
	for _, fi := range reg.FieldIndexers() {
		idx.RegisterFieldIndexer(fi)
	}

	return &SearchIndex{
		store:   cached,
		idx:     idx,
		reader:  &plugin.IndexAdapter{Idx: idx},
		mapping: indexMapping,
		plugins: reg,
	}, nil
}

// Plugins returns the plugin registry, allowing runtime registration.
func (si *SearchIndex) Plugins() *plugin.Registry {
	return si.plugins
}

// RegisterFieldIndexer registers a custom field indexer on the underlying index.
// Use this to add support for new field types at runtime.
func (si *SearchIndex) RegisterFieldIndexer(fi index.FieldIndexer) {
	si.idx.RegisterFieldIndexer(fi)
}

// SetLogger sets the logger used by the underlying index.
// Pass index.NopLogger{} to suppress all log output.
func (si *SearchIndex) SetLogger(l index.Logger) {
	si.idx.SetLogger(l)
}

// SetInMemoryMode enables or disables in-memory mode on the underlying index.
// When enabled, the postings cache has no LRU eviction limit and WarmCache()
// can be used to eagerly preload all index data for maximum query performance.
func (si *SearchIndex) SetInMemoryMode(enabled bool) {
	si.idx.SetInMemoryMode(enabled)
}

// WarmCache eagerly preloads all term dictionaries, posting lists,
// document frequencies, field lengths, and aggregate statistics into memory.
// Most effective after calling SetInMemoryMode(true).
func (si *SearchIndex) WarmCache() error {
	return si.idx.WarmCache()
}

// Close releases resources held by the index. After calling Close,
// further operations on the index will panic. The underlying store
// is NOT closed; the caller is responsible for closing it.
func (si *SearchIndex) Close() error {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.idx = nil
	si.reader = nil
	si.store = nil
	return nil
}

// validateID checks that a document ID is valid.
func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("document ID cannot be empty")
	}
	if len(id) > 512 {
		return fmt.Errorf("document ID exceeds maximum length of 512 bytes")
	}
	if strings.Contains(id, "/") {
		return fmt.Errorf("document ID cannot contain '/'")
	}
	for i := 0; i < len(id); i++ {
		if id[i] < 0x20 {
			return fmt.Errorf("document ID contains invalid control character at byte %d", i)
		}
	}
	return nil
}

// indexInternal adds or updates a document without acquiring the mutex.
// Callers must hold si.mu (write lock).
func (si *SearchIndex) indexInternal(id string, data interface{}) error {
	if err := validateID(id); err != nil {
		return err
	}

	doc := document.NewDocument(id)
	if err := si.mapping.MapDocument(doc, data); err != nil {
		return fmt.Errorf("map document: %w", err)
	}

	return si.idx.IndexDocument(doc)
}

// deleteInternal removes a document without acquiring the mutex.
// Callers must hold si.mu (write lock).
func (si *SearchIndex) deleteInternal(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	return si.idx.DeleteDocument(id)
}

// Index adds or updates a document in the index.
func (si *SearchIndex) Index(id string, data interface{}) error {
	si.mu.Lock()
	defer si.mu.Unlock()
	return si.indexInternal(id, data)
}

// Delete removes a document from the index.
func (si *SearchIndex) Delete(id string) error {
	si.mu.Lock()
	defer si.mu.Unlock()
	return si.deleteInternal(id)
}

// Document retrieves a stored document by ID.
func (si *SearchIndex) Document(id string) (map[string]interface{}, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	si.mu.RLock()
	defer si.mu.RUnlock()
	sd, err := si.idx.GetDocument(id)
	if err != nil {
		return nil, err
	}
	return sd.Fields, nil
}

// DocCount returns the total number of indexed documents.
func (si *SearchIndex) DocCount() (uint64, error) {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.idx.DocCount()
}

// Batch allows multiple index/delete operations to be performed together.
type Batch struct {
	si  *SearchIndex
	ops []batchOp
}

type batchOp struct {
	isDelete bool
	id       string
	data     interface{}
}

// NewBatch creates a new batch for the given index.
func (si *SearchIndex) NewBatch() *Batch {
	return &Batch{si: si}
}

// Index adds an index operation to the batch.
func (b *Batch) Index(id string, data interface{}) {
	b.ops = append(b.ops, batchOp{id: id, data: data})
}

// Delete adds a delete operation to the batch.
func (b *Batch) Delete(id string) {
	b.ops = append(b.ops, batchOp{isDelete: true, id: id})
}

// Execute runs all operations in the batch atomically with respect to readers.
func (b *Batch) Execute() error {
	b.si.mu.Lock()
	defer b.si.mu.Unlock()
	// Enable batch mode to defer per-document cache invalidation.
	b.si.idx.BeginBatch()
	defer b.si.idx.EndBatch()
	for _, op := range b.ops {
		if op.isDelete {
			if err := b.si.deleteInternal(op.id); err != nil {
				return err
			}
		} else {
			if err := b.si.indexInternal(op.id, op.data); err != nil {
				return err
			}
		}
	}
	return nil
}

// Size returns the number of operations in the batch.
func (b *Batch) Size() int {
	return len(b.ops)
}

// Pagination limits to prevent excessive memory allocation.
const (
	// MaxSearchSize is the maximum number of results that can be requested.
	MaxSearchSize = 10000
	// MaxSearchFrom is the maximum offset for pagination.
	MaxSearchFrom = 100000
	// MaxFacetDocIDs is the maximum number of document IDs collected for facet computation.
	// Facets computed from a subset still provide useful distribution information.
	MaxFacetDocIDs = 100000
)

// Search executes a search query and returns matching documents.
func (si *SearchIndex) Search(ctx context.Context, q query.Query) (*search.SearchResult, error) {
	return si.SearchWithRequest(ctx, &search.SearchRequest{
		Query: q,
		Size:  10,
		From:  0,
	})
}

// SearchWithRequest executes a search with a full SearchRequest.
func (si *SearchIndex) SearchWithRequest(ctx context.Context, req *search.SearchRequest) (*search.SearchResult, error) {
	startTime := time.Now()

	si.mu.RLock()
	defer si.mu.RUnlock()

	q, ok := req.Query.(query.Query)
	if !ok {
		return nil, fmt.Errorf("invalid query type: %T", req.Query)
	}

	defaultField := "_all"
	sf := si.plugins.GetScorerFactory()

	searcher, err := q.Searcher(si.reader, defaultField, sf)
	if err != nil {
		return nil, fmt.Errorf("create searcher: %w", err)
	}

	size := req.Size
	if size <= 0 {
		size = 10
	}
	if size > MaxSearchSize {
		size = MaxSearchSize
	}
	from := req.From
	if from < 0 {
		from = 0
	}
	if from > MaxSearchFrom {
		from = MaxSearchFrom
	}
	heapCap := from + size
	// Guard against overflow: from and size are both bounded, but be safe.
	if heapCap < 0 || heapCap > MaxSearchFrom+MaxSearchSize {
		heapCap = MaxSearchFrom + MaxSearchSize
	}

	var totalHits uint64
	maxScore := 0.0
	h := &scoreHeap{}
	heap.Init(h)

	hasFacets := len(req.Facets) > 0
	var facetDocIDs []string

	// Check context cancellation periodically, not every iteration.
	// The select with channel read is expensive in tight loops.
	ctxDone := ctx.Done()
	for {
		if totalHits&63 == 0 && ctxDone != nil {
			select {
			case <-ctxDone:
				return nil, ctx.Err()
			default:
			}
		}

		ds, err := searcher.Next()
		if err != nil {
			return nil, err
		}
		if ds == nil {
			break
		}

		totalHits++
		if ds.Score > maxScore {
			maxScore = ds.Score
		}

		if hasFacets && len(facetDocIDs) < MaxFacetDocIDs {
			facetDocIDs = append(facetDocIDs, ds.ID)
		}

		if h.Len() < heapCap {
			heap.Push(h, &search.DocumentMatch{ID: ds.ID, Score: ds.Score})
		} else if ds.Score > (*h)[0].Score {
			(*h)[0] = &search.DocumentMatch{ID: ds.ID, Score: ds.Score}
			heap.Fix(h, 0)
		}
	}

	heapLen := h.Len()
	sorted := make([]*search.DocumentMatch, heapLen)
	for i := heapLen - 1; i >= 0; i-- {
		sorted[i] = heap.Pop(h).(*search.DocumentMatch)
	}

	var allDocs []*search.DocumentMatch
	if from >= len(sorted) {
		allDocs = nil
	} else {
		end := from + size
		if end > len(sorted) {
			end = len(sorted)
		}
		allDocs = sorted[from:end]
	}

	// Load stored fields and compute highlights
	for _, dm := range allDocs {
		// Load stored fields: nil means load all fields, non-empty means
		// load only the listed fields, empty non-nil slice means load none.
		if req.Fields != nil && len(req.Fields) == 0 {
			// Explicitly requested no fields; skip loading.
		} else {
			sd, err := si.idx.GetDocument(dm.ID)
			if err == nil {
				if len(req.Fields) > 0 {
					dm.Fields = make(map[string]interface{})
					for _, f := range req.Fields {
						if v, ok := sd.Fields[f]; ok {
							dm.Fields[f] = v
						}
					}
				} else {
					dm.Fields = sd.Fields
				}
			}
		}

		// Highlighting
		if req.Highlight != nil && dm.Fields != nil {
			dm.Fragments = make(map[string][]string)

			queryTerms := extractQueryTerms(q)

			fields := req.Highlight.Fields
			if len(fields) == 0 {
				for fieldName := range dm.Fields {
					fields = append(fields, fieldName)
				}
			}

			// Use plugin highlighter (defaults are always set)
			hf := si.plugins.GetHighlighterFactory()
			hl := hf.NewHighlighter()
			for _, fieldName := range fields {
				if textVal, ok := dm.Fields[fieldName].(string); ok {
					analyzerName := si.getFieldAnalyzer(fieldName)
					analyzedTerms := highlight.ExtractQueryTerms(queryTerms, analyzerName)
					fragments := hl.Highlight(si.reader, dm.ID, fieldName, textVal, analyzerName, analyzedTerms)
					if len(fragments) > 0 {
						dm.Fragments[fieldName] = fragments
					}
				}
			}
		}
	}

	// Build facets (defaults are always set)
	var facetResults map[string]*search.FacetResult
	if hasFacets {
		ff := si.plugins.GetFacetBuilderFactory()
		fb := ff.NewFacetBuilder(si.reader)
		facetResults = fb.Build(facetDocIDs, req.Facets)
	}

	return &search.SearchResult{
		Status: search.SearchStatus{
			Total:      1,
			Successful: 1,
		},
		Hits:     allDocs,
		Total:    totalHits,
		MaxScore: maxScore,
		Took:     time.Since(startTime),
		Facets:   facetResults,
		Request:  req,
	}, nil
}

func (si *SearchIndex) getFieldAnalyzer(fieldName string) string {
	if fm, ok := si.mapping.DefaultMapping.Fields[fieldName]; ok {
		if fm.Analyzer != "" {
			return fm.Analyzer
		}
	}
	for _, dm := range si.mapping.TypeMapping {
		if fm, ok := dm.Fields[fieldName]; ok {
			if fm.Analyzer != "" {
				return fm.Analyzer
			}
		}
	}
	return si.mapping.DefaultAnalyzer
}

// extractQueryTerms extracts search terms from a query for highlighting.
func extractQueryTerms(q query.Query) []string {
	if ext, ok := q.(plugin.QueryTermExtractor); ok {
		return ext.ExtractTerms()
	}
	return nil
}

// Convenience functions for creating mappings and queries.

// NewIndexMapping creates a new IndexMapping.
func NewIndexMapping() *mapping.IndexMapping {
	return mapping.NewIndexMapping()
}

// NewDocumentMapping creates a new DocumentMapping.
func NewDocumentMapping() *mapping.DocumentMapping {
	return mapping.NewDocumentMapping()
}

// NewDocumentStaticMapping creates a new static DocumentMapping.
func NewDocumentStaticMapping() *mapping.DocumentMapping {
	return mapping.NewDocumentStaticMapping()
}

// NewTextFieldMapping creates a text field mapping.
func NewTextFieldMapping() *mapping.FieldMapping {
	return mapping.NewTextFieldMapping()
}

// NewKeywordFieldMapping creates a keyword field mapping.
func NewKeywordFieldMapping() *mapping.FieldMapping {
	return mapping.NewKeywordFieldMapping()
}

// NewNumericFieldMapping creates a numeric field mapping.
func NewNumericFieldMapping() *mapping.FieldMapping {
	return mapping.NewNumericFieldMapping()
}

// NewBooleanFieldMapping creates a boolean field mapping.
func NewBooleanFieldMapping() *mapping.FieldMapping {
	return mapping.NewBooleanFieldMapping()
}

// NewDateTimeFieldMapping creates a datetime field mapping.
func NewDateTimeFieldMapping() *mapping.FieldMapping {
	return mapping.NewDateTimeFieldMapping()
}

// NewVectorFieldMapping creates a vector field mapping.
func NewVectorFieldMapping(dims int) *mapping.FieldMapping {
	return mapping.NewVectorFieldMapping(dims)
}

// NewCodeFieldMapping creates a code field mapping.
func NewCodeFieldMapping() *mapping.FieldMapping {
	return mapping.NewCodeFieldMapping()
}

// NewSymbolFieldMapping creates a symbol field mapping for code symbol extraction.
func NewSymbolFieldMapping(language string) *mapping.FieldMapping {
	return mapping.NewSymbolFieldMapping(language)
}
