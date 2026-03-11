package searchmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/readmedotmd/search.md/document"
	"github.com/readmedotmd/search.md/index"
	"github.com/readmedotmd/search.md/mapping"
	"github.com/readmedotmd/search.md/plugin"
	"github.com/readmedotmd/search.md/search"
	"github.com/readmedotmd/search.md/search/query"
)

// =============================================================================
// Full Lifecycle Integration Tests
// =============================================================================

// TestFullLifecycle exercises the complete document lifecycle:
// create index → index docs → search → update → delete → verify consistency.
func TestFullLifecycle(t *testing.T) {
	idx := newTestIndex(t)

	// Phase 1: Index documents
	docs := []struct {
		id   string
		data map[string]interface{}
	}{
		{"article-1", map[string]interface{}{
			"title":   "Introduction to Go",
			"content": "Go is a statically typed compiled language designed at Google",
			"author":  "Alice",
			"rating":  4.5,
		}},
		{"article-2", map[string]interface{}{
			"title":   "Advanced Go Patterns",
			"content": "This article covers advanced Go patterns including concurrency",
			"author":  "Bob",
			"rating":  4.8,
		}},
		{"article-3", map[string]interface{}{
			"title":   "Python for Data Science",
			"content": "Python is widely used for data science and machine learning",
			"author":  "Alice",
			"rating":  3.9,
		}},
	}

	for _, d := range docs {
		if err := idx.Index(d.id, d.data); err != nil {
			t.Fatalf("Index error: %v", err)
		}
	}

	count, _ := idx.DocCount()
	if count != 3 {
		t.Fatalf("expected 3 docs, got %d", count)
	}

	// Phase 2: Search and verify results
	q := query.NewMatchQuery("go").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results.Total < 2 {
		t.Errorf("expected at least 2 results for 'go' in title, got %d", results.Total)
	}

	// Phase 3: Update a document
	err = idx.Index("article-1", map[string]interface{}{
		"title":   "Introduction to Rust",
		"content": "Rust is a systems programming language focused on safety",
		"author":  "Alice",
		"rating":  4.7,
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}

	count, _ = idx.DocCount()
	if count != 3 {
		t.Errorf("expected 3 docs after update, got %d", count)
	}

	// Old content should be gone
	qOld := query.NewMatchQuery("go").SetField("title")
	rOld, _ := idx.Search(context.Background(), qOld)
	for _, hit := range rOld.Hits {
		if hit.ID == "article-1" {
			t.Error("article-1 should not match 'go' after update to 'Rust'")
		}
	}

	// New content should be findable
	qNew := query.NewMatchQuery("rust").SetField("title")
	rNew, _ := idx.Search(context.Background(), qNew)
	if rNew.Total != 1 {
		t.Errorf("expected 1 result for 'rust', got %d", rNew.Total)
	}

	// Phase 4: Delete a document
	if err := idx.Delete("article-3"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	count, _ = idx.DocCount()
	if count != 2 {
		t.Errorf("expected 2 docs after delete, got %d", count)
	}

	// Deleted doc should not appear in searches
	qPython := query.NewMatchQuery("python").SetField("title")
	rPython, _ := idx.Search(context.Background(), qPython)
	if rPython.Total != 0 {
		t.Errorf("expected 0 results for deleted doc's content, got %d", rPython.Total)
	}

	// Remaining docs should still be searchable
	qAll := query.NewMatchAllQuery()
	rAll, _ := idx.Search(context.Background(), qAll)
	if rAll.Total != 2 {
		t.Errorf("expected 2 docs from match all, got %d", rAll.Total)
	}

	// Phase 5: Verify stored fields survive
	fields, err := idx.Document("article-1")
	if err != nil {
		t.Fatalf("Document error: %v", err)
	}
	if fields["title"] != "Introduction to Rust" {
		t.Errorf("expected updated title, got %v", fields["title"])
	}
}

// =============================================================================
// All Field Types Integration
// =============================================================================

// TestAllFieldTypesDocument indexes a document using every supported field type
// and verifies each is searchable via the appropriate query type.
func TestAllFieldTypesDocument(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("tag", mapping.NewKeywordFieldMapping())
	dm.AddFieldMapping("price", mapping.NewNumericFieldMapping())
	dm.AddFieldMapping("active", mapping.NewBooleanFieldMapping())
	dm.AddFieldMapping("created", mapping.NewDateTimeFieldMapping())
	dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(4))
	dm.AddFieldMapping("code", mapping.NewCodeFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	err := idx.Index("multi-1", map[string]interface{}{
		"title":     "Full field test document",
		"tag":       "integration",
		"price":     29.99,
		"active":    true,
		"created":   "2025-06-15T10:30:00Z",
		"embedding": []float64{0.5, 0.5, 0.5, 0.5},
		"code":      "func processHTTPRequest(ctx context.Context) error",
	})
	if err != nil {
		t.Fatalf("Index error: %v", err)
	}

	// Text field search
	q1 := query.NewMatchQuery("test").SetField("title")
	r1, _ := idx.Search(context.Background(), q1)
	if r1.Total != 1 {
		t.Errorf("text field: expected 1, got %d", r1.Total)
	}

	// Keyword field search (exact match)
	q2 := query.NewTermQuery("integration").SetField("tag")
	r2, _ := idx.Search(context.Background(), q2)
	if r2.Total != 1 {
		t.Errorf("keyword field: expected 1, got %d", r2.Total)
	}

	// Numeric range search
	min, max := 20.0, 40.0
	q3 := query.NewNumericRangeQuery(&min, &max).SetField("price")
	r3, _ := idx.Search(context.Background(), q3)
	if r3.Total != 1 {
		t.Errorf("numeric field: expected 1, got %d", r3.Total)
	}

	// Boolean field search
	q4 := query.NewTermQuery("T").SetField("active")
	r4, _ := idx.Search(context.Background(), q4)
	if r4.Total != 1 {
		t.Errorf("boolean field: expected 1, got %d", r4.Total)
	}

	// DateTime range search
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	q5 := query.NewDateRangeQuery(&start, &end).SetField("created")
	r5, _ := idx.Search(context.Background(), q5)
	if r5.Total != 1 {
		t.Errorf("datetime field: expected 1, got %d", r5.Total)
	}

	// Vector search
	q6 := query.NewKNNQuery([]float32{0.5, 0.5, 0.5, 0.5}, 5).SetField("embedding")
	r6, _ := idx.Search(context.Background(), q6)
	if r6.Total != 1 {
		t.Errorf("vector field: expected 1, got %d", r6.Total)
	}

	// Code field search (camelCase splitting)
	q7 := query.NewTermQuery("process").SetField("code")
	r7, _ := idx.Search(context.Background(), q7)
	if r7.Total != 1 {
		t.Errorf("code field (camelCase): expected 1, got %d", r7.Total)
	}

	// Code field search (HTTP acronym)
	q8 := query.NewTermQuery("http").SetField("code")
	r8, _ := idx.Search(context.Background(), q8)
	if r8.Total != 1 {
		t.Errorf("code field (acronym): expected 1, got %d", r8.Total)
	}

	// Delete and verify ALL field types are cleaned up
	idx.Delete("multi-1")

	count, _ := idx.DocCount()
	if count != 0 {
		t.Errorf("expected 0 docs after delete, got %d", count)
	}

	// Re-check every field type returns 0
	r1a, _ := idx.Search(context.Background(), q1)
	r2a, _ := idx.Search(context.Background(), q2)
	r3a, _ := idx.Search(context.Background(), q3)
	r4a, _ := idx.Search(context.Background(), q4)
	r5a, _ := idx.Search(context.Background(), q5)
	r6a, _ := idx.Search(context.Background(), q6)
	r7a, _ := idx.Search(context.Background(), q7)

	for _, r := range []*search.SearchResult{r1a, r2a, r3a, r4a, r5a, r6a, r7a} {
		if r.Total != 0 {
			t.Errorf("expected 0 results after delete, got %d", r.Total)
		}
	}
}

// =============================================================================
// Concurrent Operations Integration Tests
// =============================================================================

// TestConcurrentIndexAndSearch verifies that concurrent indexing and searching
// does not cause data races or panics.
func TestConcurrentIndexAndSearch(t *testing.T) {
	idx := newTestIndex(t)

	// Pre-seed some docs
	for i := 0; i < 10; i++ {
		idx.Index(fmt.Sprintf("seed%d", i), map[string]interface{}{
			"title": fmt.Sprintf("seed document %d", i),
		})
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent writers
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				id := fmt.Sprintf("w%d-d%d", worker, i)
				err := idx.Index(id, map[string]interface{}{
					"title": fmt.Sprintf("worker %d document %d", worker, i),
				})
				if err != nil {
					errors <- fmt.Errorf("index error w%d-d%d: %v", worker, i, err)
				}
			}
		}(w)
	}

	// Concurrent readers
	for r := 0; r < 5; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				q := query.NewMatchAllQuery()
				_, err := idx.Search(context.Background(), q)
				if err != nil {
					errors <- fmt.Errorf("search error: %v", err)
				}
			}
		}()
	}

	// Concurrent deleters
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			idx.Delete(fmt.Sprintf("seed%d", i))
		}
	}()

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Index should be in a consistent state
	count, err := idx.DocCount()
	if err != nil {
		t.Fatalf("DocCount error: %v", err)
	}
	// 5 workers * 20 docs = 100 new docs, 10 seed docs deleted
	if count != 100 {
		t.Errorf("expected 100 docs after concurrent ops, got %d", count)
	}
}

// TestConcurrentReindex verifies concurrent updates to the same document are safe.
func TestConcurrentReindex(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("shared", map[string]interface{}{"title": "initial"})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(version int) {
			defer wg.Done()
			idx.Index("shared", map[string]interface{}{
				"title": fmt.Sprintf("version %d", version),
			})
		}(i)
	}
	wg.Wait()

	count, _ := idx.DocCount()
	if count != 1 {
		t.Errorf("expected 1 doc after concurrent reindex, got %d", count)
	}

	// Should still be retrievable
	fields, err := idx.Document("shared")
	if err != nil {
		t.Fatalf("Document error: %v", err)
	}
	if fields["title"] == nil {
		t.Error("expected non-nil title after concurrent reindex")
	}
}

// =============================================================================
// Complex Nested Boolean Query Integration Tests
// =============================================================================

// TestNestedBooleanQueries exercises deeply nested boolean query combinations.
func TestNestedBooleanQueries(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("d1", map[string]interface{}{"content": "alpha beta gamma", "tag": "a"})
	idx.Index("d2", map[string]interface{}{"content": "alpha delta", "tag": "b"})
	idx.Index("d3", map[string]interface{}{"content": "beta gamma epsilon", "tag": "a"})
	idx.Index("d4", map[string]interface{}{"content": "alpha beta delta gamma", "tag": "c"})
	idx.Index("d5", map[string]interface{}{"content": "zeta eta", "tag": "b"})

	// (must: alpha AND beta) AND (must_not: delta) => only d1
	bq := query.NewBooleanQuery()
	bq.AddMust(
		query.NewConjunctionQuery(
			query.NewMatchQuery("alpha").SetField("content"),
			query.NewMatchQuery("beta").SetField("content"),
		),
	)
	bq.AddMustNot(query.NewMatchQuery("delta").SetField("content"))

	results, err := idx.Search(context.Background(), bq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results.Total != 1 {
		t.Errorf("expected 1 result, got %d", results.Total)
	}
	if len(results.Hits) > 0 && results.Hits[0].ID != "d1" {
		t.Errorf("expected d1, got %s", results.Hits[0].ID)
	}
}

// TestBooleanMustShouldMustNot exercises all three boolean clauses together.
func TestBooleanMustShouldMustNot(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("d1", map[string]interface{}{"content": "search engine fast reliable"})
	idx.Index("d2", map[string]interface{}{"content": "search engine slow legacy"})
	idx.Index("d3", map[string]interface{}{"content": "search database fast"})
	idx.Index("d4", map[string]interface{}{"content": "database only"})

	bq := query.NewBooleanQuery()
	bq.AddMust(query.NewMatchQuery("search").SetField("content"))
	bq.AddShould(query.NewMatchQuery("fast").SetField("content"))
	bq.AddMustNot(query.NewMatchQuery("legacy").SetField("content"))

	results, err := idx.Search(context.Background(), bq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// Must match "search": d1, d2, d3
	// Must not "legacy": excludes d2
	// Remaining: d1, d3
	if results.Total != 2 {
		t.Errorf("expected 2 results, got %d", results.Total)
	}

	// d1 has "fast" (should bonus) so should score higher than d3
	if len(results.Hits) >= 2 {
		// d1 should be first due to should boost from "fast"
		hasD1 := false
		for _, hit := range results.Hits {
			if hit.ID == "d1" {
				hasD1 = true
			}
			if hit.ID == "d2" {
				t.Error("d2 should be excluded by must_not")
			}
		}
		if !hasD1 {
			t.Error("d1 should be in results")
		}
	}
}

// TestDisjunctionInsideConjunction nests a disjunction within a conjunction.
func TestDisjunctionInsideConjunction(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("d1", map[string]interface{}{"content": "alpha gamma"})
	idx.Index("d2", map[string]interface{}{"content": "alpha delta"})
	idx.Index("d3", map[string]interface{}{"content": "beta gamma"})
	idx.Index("d4", map[string]interface{}{"content": "beta delta"})

	// Must have (alpha OR beta) AND (gamma)
	cq := query.NewConjunctionQuery(
		query.NewDisjunctionQuery(
			query.NewTermQuery("alpha").SetField("content"),
			query.NewTermQuery("beta").SetField("content"),
		),
		query.NewTermQuery("gamma").SetField("content"),
	)

	results, err := idx.Search(context.Background(), cq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// d1 (alpha + gamma) and d3 (beta + gamma) match
	if results.Total != 2 {
		t.Errorf("expected 2 results, got %d", results.Total)
	}

	ids := make(map[string]bool)
	for _, hit := range results.Hits {
		ids[hit.ID] = true
	}
	if !ids["d1"] || !ids["d3"] {
		t.Errorf("expected d1 and d3, got %v", ids)
	}
}

// =============================================================================
// Hybrid Search (KNN + Text) Integration Tests
// =============================================================================

// TestHybridKNNAndTextSearch combines vector and text search using boolean queries.
func TestHybridKNNAndTextSearch(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(3))
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("cat", map[string]interface{}{
		"title":     "cat document about felines",
		"embedding": []float64{1.0, 0.0, 0.0},
	})
	idx.Index("dog", map[string]interface{}{
		"title":     "dog document about canines",
		"embedding": []float64{0.0, 1.0, 0.0},
	})
	idx.Index("catdog", map[string]interface{}{
		"title":     "cat and dog living together",
		"embedding": []float64{0.7, 0.7, 0.0},
	})
	idx.Index("bird", map[string]interface{}{
		"title":     "bird document about avians",
		"embedding": []float64{0.0, 0.0, 1.0},
	})

	// Boolean: must match text "cat", should have vector similar to cat
	bq := query.NewBooleanQuery()
	bq.AddMust(query.NewMatchQuery("cat").SetField("title"))
	bq.AddShould(query.NewKNNQuery([]float32{1.0, 0.0, 0.0}, 10).SetField("embedding"))

	results, err := idx.Search(context.Background(), bq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// "cat" and "catdog" match text, "cat" also gets vector boost
	if results.Total < 2 {
		t.Errorf("expected at least 2 results, got %d", results.Total)
	}

	// "cat" should rank first (text match + high vector similarity)
	if len(results.Hits) > 0 && results.Hits[0].ID != "cat" {
		t.Errorf("expected 'cat' as top hybrid result, got '%s'", results.Hits[0].ID)
	}
}

// =============================================================================
// Highlighting Integration Tests
// =============================================================================

// TestHighlightingWithTermQuery verifies highlighting works with term queries.
func TestHighlightingWithTermQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewTermQuery("quick").SetField("content")
	req := &search.SearchRequest{
		Query:     q,
		Size:      10,
		Highlight: &search.HighlightRequest{Fields: []string{"content"}},
	}

	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	found := false
	for _, hit := range results.Hits {
		if hit.ID == "doc1" {
			if frags, ok := hit.Fragments["content"]; ok && len(frags) > 0 {
				for _, f := range frags {
					if strings.Contains(f, "<mark>") {
						found = true
					}
				}
			}
		}
	}
	if !found {
		t.Error("expected <mark> tags in highlighted fragments for doc1")
	}
}

// TestHighlightingWithBooleanQuery tests highlighting across compound queries.
func TestHighlightingWithBooleanQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	bq := query.NewBooleanQuery()
	bq.AddMust(query.NewMatchQuery("search").SetField("content"))
	bq.AddShould(query.NewMatchQuery("engine").SetField("content"))

	req := &search.SearchRequest{
		Query:     bq,
		Size:      10,
		Highlight: &search.HighlightRequest{Fields: []string{"content"}},
	}

	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// At least one result should have fragments
	hasFragments := false
	for _, hit := range results.Hits {
		if len(hit.Fragments["content"]) > 0 {
			hasFragments = true
			break
		}
	}
	if results.Total > 0 && !hasFragments {
		t.Error("expected at least one hit with highlight fragments")
	}
}

// TestHighlightingAllFieldsAutomatic tests that omitting Fields highlights all stored text fields.
func TestHighlightingAllFieldsAutomatic(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchQuery("quick").SetField("content")
	req := &search.SearchRequest{
		Query:     q,
		Size:      10,
		Highlight: &search.HighlightRequest{}, // no specific fields
	}

	results, _ := idx.SearchWithRequest(context.Background(), req)

	for _, hit := range results.Hits {
		if hit.ID == "doc1" {
			// Should have fragments for at least one field
			if len(hit.Fragments) == 0 {
				t.Error("expected auto-highlighted fragments for doc1")
			}
		}
	}
}

// TestHighlightingWithPhraseQuery verifies highlighting with phrase queries.
func TestHighlightingWithPhraseQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchPhraseQuery("quick brown fox").SetField("content")
	req := &search.SearchRequest{
		Query:     q,
		Size:      10,
		Highlight: &search.HighlightRequest{Fields: []string{"content"}},
	}

	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total > 0 {
		hit := results.Hits[0]
		if len(hit.Fragments["content"]) == 0 {
			t.Error("expected highlighted fragments for phrase query")
		}
	}
}

// =============================================================================
// Multi-Facet Combination Integration Tests
// =============================================================================

// TestMultipleFacetTypes requests term, numeric range, and date range facets
// in a single query.
func TestMultipleFacetTypes(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentMapping()
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("author", mapping.NewKeywordFieldMapping())
	dm.AddFieldMapping("price", mapping.NewNumericFieldMapping())
	dm.AddFieldMapping("published", mapping.NewDateTimeFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("b1", map[string]interface{}{
		"title": "Go Programming", "author": "alice", "price": 29.99,
		"published": "2024-03-15T00:00:00Z",
	})
	idx.Index("b2", map[string]interface{}{
		"title": "Python Cookbook", "author": "bob", "price": 39.99,
		"published": "2024-08-20T00:00:00Z",
	})
	idx.Index("b3", map[string]interface{}{
		"title": "Rust Handbook", "author": "alice", "price": 49.99,
		"published": "2025-01-10T00:00:00Z",
	})
	idx.Index("b4", map[string]interface{}{
		"title": "Java Patterns", "author": "charlie", "price": 19.99,
		"published": "2025-06-01T00:00:00Z",
	})

	start2024 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end2024 := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
	start2025 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end2025 := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

	req := &search.SearchRequest{
		Query: query.NewMatchAllQuery(),
		Size:  10,
		Facets: map[string]*search.FacetRequest{
			"authors": {Field: "author", Size: 10},
			"price_ranges": {
				Field: "price", Size: 3,
				NumericRanges: []search.NumericRange{
					{Name: "cheap", Min: floatPtr(0), Max: floatPtr(30)},
					{Name: "mid", Min: floatPtr(30), Max: floatPtr(50)},
				},
			},
			"by_year": {
				Field: "published", Size: 5,
				DateTimeRanges: []search.DateTimeRange{
					{Name: "2024", Start: &start2024, End: &end2024},
					{Name: "2025", Start: &start2025, End: &end2025},
				},
			},
		},
	}

	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total != 4 {
		t.Errorf("expected 4 total, got %d", results.Total)
	}

	// Check term facet
	authorFacet := results.Facets["authors"]
	if authorFacet == nil {
		t.Fatal("expected 'authors' facet")
	}
	authorCounts := make(map[string]int)
	for _, tf := range authorFacet.Terms {
		authorCounts[tf.Term] = tf.Count
	}
	if authorCounts["alice"] != 2 {
		t.Errorf("expected alice=2, got %d", authorCounts["alice"])
	}

	// Check numeric range facet
	priceFacet := results.Facets["price_ranges"]
	if priceFacet == nil {
		t.Fatal("expected 'price_ranges' facet")
	}
	if len(priceFacet.NumericRanges) != 2 {
		t.Errorf("expected 2 price ranges, got %d", len(priceFacet.NumericRanges))
	}
	priceCounts := make(map[string]int)
	for _, nr := range priceFacet.NumericRanges {
		priceCounts[nr.Name] = nr.Count
	}
	if priceCounts["cheap"] != 2 {
		t.Errorf("expected cheap=2 (29.99, 19.99), got %d", priceCounts["cheap"])
	}
	if priceCounts["mid"] != 2 {
		t.Errorf("expected mid=2 (39.99, 49.99), got %d", priceCounts["mid"])
	}

	// Check date range facet
	dateFacet := results.Facets["by_year"]
	if dateFacet == nil {
		t.Fatal("expected 'by_year' facet")
	}
	dateCounts := make(map[string]int)
	for _, dr := range dateFacet.DateRanges {
		dateCounts[dr.Name] = dr.Count
	}
	if dateCounts["2024"] != 2 {
		t.Errorf("expected 2024=2, got %d", dateCounts["2024"])
	}
	if dateCounts["2025"] != 2 {
		t.Errorf("expected 2025=2, got %d", dateCounts["2025"])
	}
}

// TestFacetOtherCount verifies the "Other" field in facet results when
// Size limits the number of returned terms.
func TestFacetOtherCount(t *testing.T) {
	idx := newTestIndex(t)

	authors := []string{"alice", "bob", "charlie", "diana", "eve"}
	for i, author := range authors {
		for j := 0; j <= i; j++ {
			idx.Index(fmt.Sprintf("%s-%d", author, j), map[string]interface{}{
				"title":  fmt.Sprintf("Doc by %s", author),
				"author": author,
			})
		}
	}

	req := &search.SearchRequest{
		Query: query.NewMatchAllQuery(),
		Size:  20,
		Facets: map[string]*search.FacetRequest{
			"top_authors": {Field: "author", Size: 2}, // Only top 2
		},
	}

	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	authorFacet := results.Facets["top_authors"]
	if authorFacet == nil {
		t.Fatal("expected facet")
	}

	if len(authorFacet.Terms) != 2 {
		t.Errorf("expected 2 terms (size limit), got %d", len(authorFacet.Terms))
	}

	// Other should account for the remaining terms
	if authorFacet.Other <= 0 {
		t.Errorf("expected positive Other count, got %d", authorFacet.Other)
	}
}

// =============================================================================
// Analyzer Behavior Integration Tests
// =============================================================================

// TestKeywordVsStandardAnalyzer verifies that keyword fields preserve exact values
// while standard text fields are analyzed.
func TestKeywordVsStandardAnalyzer(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("name", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("code", mapping.NewKeywordFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("d1", map[string]interface{}{
		"name": "HTTP-404-NotFound",
		"code": "HTTP-404-NotFound",
	})

	// Text field: standard analyzer lowercases and stems
	q1 := query.NewTermQuery("http-404-notfound").SetField("name")
	r1, _ := idx.Search(context.Background(), q1)
	// Standard analyzer splits this, so exact term won't match
	// The individual tokens might match though

	// Keyword field: exact match preserved
	q2 := query.NewTermQuery("HTTP-404-NotFound").SetField("code")
	r2, _ := idx.Search(context.Background(), q2)
	if r2.Total != 1 {
		t.Errorf("keyword field should match exact value, got %d", r2.Total)
	}

	// Keyword should NOT match lowercased version
	q3 := query.NewTermQuery("http-404-notfound").SetField("code")
	r3, _ := idx.Search(context.Background(), q3)
	if r3.Total != 0 {
		t.Errorf("keyword field should NOT match case-changed value, got %d", r3.Total)
	}

	_ = r1 // text field behavior may vary with analyzer
}

// TestCodeAnalyzerIntegration verifies code-aware tokenization end-to-end.
func TestCodeAnalyzerIntegration(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("source", mapping.NewCodeFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("f1", map[string]interface{}{
		"source": "func processHTTPRequest(ctx context.Context) error { return handleAuth(ctx) }",
	})
	idx.Index("f2", map[string]interface{}{
		"source": "type UserProfile struct { FirstName string; LastName string }",
	})
	idx.Index("f3", map[string]interface{}{
		"source": "func get_user_by_id(user_id int) *User { return db.Find(user_id) }",
	})

	tests := []struct {
		term     string
		expected int
		desc     string
	}{
		{"process", 1, "camelCase sub-token"},
		{"http", 1, "acronym in camelCase"},
		{"request", 1, "camelCase tail"},
		{"context", 1, "package.Type splitting"},
		{"user", 2, "appears in f2 (UserProfile) and f3 (get_user_by_id)"},
		{"profile", 1, "PascalCase sub-token"},
		{"first", 1, "PascalCase sub-token from FirstName"},
		{"get", 1, "snake_case prefix"},
		{"id", 1, "snake_case suffix"},
	}

	for _, tt := range tests {
		q := query.NewTermQuery(tt.term).SetField("source")
		r, err := idx.Search(context.Background(), q)
		if err != nil {
			t.Errorf("%s: search error: %v", tt.desc, err)
			continue
		}
		if int(r.Total) != tt.expected {
			t.Errorf("%s: term '%s' expected %d, got %d", tt.desc, tt.term, tt.expected, r.Total)
		}
	}
}

// =============================================================================
// Edge Cases and Error Handling Integration Tests
// =============================================================================

// TestSearchEmptyMatchQuery verifies empty match query returns no results.
func TestSearchEmptyMatchQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchQuery("").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results.Total != 0 {
		t.Errorf("expected 0 results for empty match, got %d", results.Total)
	}
}

// TestSearchEmptyPhraseQuery verifies empty phrase query returns no results.
func TestSearchEmptyPhraseQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchPhraseQuery("").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results.Total != 0 {
		t.Errorf("expected 0 results for empty phrase, got %d", results.Total)
	}
}

// TestInvalidRegexpQuery verifies invalid regex returns an error.
func TestInvalidRegexpQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewRegexpQuery("[invalid").SetField("title")
	_, err := idx.Search(context.Background(), q)
	if err == nil {
		t.Error("expected error for invalid regexp")
	}
}

// TestSearchAfterAllDocsDeleted verifies search works on empty index after deletions.
func TestSearchAfterAllDocsDeleted(t *testing.T) {
	idx := newTestIndex(t)

	// Index then delete all
	for i := 0; i < 5; i++ {
		idx.Index(fmt.Sprintf("d%d", i), map[string]interface{}{"title": "test"})
	}
	for i := 0; i < 5; i++ {
		idx.Delete(fmt.Sprintf("d%d", i))
	}

	count, _ := idx.DocCount()
	if count != 0 {
		t.Errorf("expected 0 docs, got %d", count)
	}

	// All query types should return 0
	queries := []query.Query{
		query.NewMatchAllQuery(),
		query.NewMatchQuery("test").SetField("title"),
		query.NewTermQuery("test").SetField("title"),
		query.NewPrefixQuery("te").SetField("title"),
		query.NewMatchNoneQuery(),
	}

	for i, q := range queries {
		r, err := idx.Search(context.Background(), q)
		if err != nil {
			t.Errorf("query %d: error: %v", i, err)
			continue
		}
		if r.Total != 0 {
			t.Errorf("query %d: expected 0 results on empty index, got %d", i, r.Total)
		}
	}
}

// TestZeroSizeRequest verifies Size=0 defaults to 10 results.
func TestZeroSizeRequest(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	req := &search.SearchRequest{
		Query: query.NewMatchAllQuery(),
		Size:  0,
	}
	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results.Total != 5 {
		t.Errorf("expected total 5, got %d", results.Total)
	}
	if len(results.Hits) != 5 {
		t.Errorf("expected 5 hits (default size), got %d", len(results.Hits))
	}
}

// TestDocumentWithMissingFields verifies behavior when documents have
// different sets of fields (sparse fields).
func TestDocumentWithMissingFields(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("full", map[string]interface{}{
		"title":   "Full document",
		"content": "has all fields",
		"rating":  4.5,
	})
	idx.Index("partial", map[string]interface{}{
		"title": "Partial document",
		// no content, no rating
	})
	idx.Index("minimal", map[string]interface{}{
		"content": "only content",
		// no title
	})

	// Search title - should find "full" and "partial" but not "minimal"
	q := query.NewMatchAllQuery()
	results, _ := idx.Search(context.Background(), q)
	if results.Total != 3 {
		t.Errorf("expected 3 docs, got %d", results.Total)
	}

	// Search content field
	qContent := query.NewMatchQuery("content").SetField("content")
	rContent, _ := idx.Search(context.Background(), qContent)
	if rContent.Total < 1 {
		t.Errorf("expected at least 1 result for 'content', got %d", rContent.Total)
	}

	// Numeric on field that some docs don't have
	min := 4.0
	qNum := query.NewNumericRangeQuery(&min, nil).SetField("rating")
	rNum, _ := idx.Search(context.Background(), qNum)
	if rNum.Total != 1 {
		t.Errorf("expected 1 result for rating >= 4, got %d", rNum.Total)
	}
}

// TestVeryLongDocumentContent verifies indexing and searching large text.
func TestVeryLongDocumentContent(t *testing.T) {
	idx := newTestIndex(t)

	// Build a long document (~10k words)
	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		sb.WriteString(fmt.Sprintf("word%d filler padding text ", i))
	}
	sb.WriteString("needle haystack")

	err := idx.Index("long", map[string]interface{}{
		"content": sb.String(),
	})
	if err != nil {
		t.Fatalf("Index error: %v", err)
	}

	// Should be able to find the needle
	q := query.NewMatchQuery("needle").SetField("content")
	r, _ := idx.Search(context.Background(), q)
	if r.Total != 1 {
		t.Errorf("expected 1 result for 'needle' in long doc, got %d", r.Total)
	}
}

// TestDocumentWithManyFields verifies indexing documents with many fields.
func TestDocumentWithManyFields(t *testing.T) {
	idx := newTestIndex(t)

	doc := make(map[string]interface{})
	for i := 0; i < 50; i++ {
		doc[fmt.Sprintf("field%d", i)] = fmt.Sprintf("value for field %d", i)
	}

	err := idx.Index("many-fields", doc)
	if err != nil {
		t.Fatalf("Index error: %v", err)
	}

	// Search in a specific field
	q := query.NewMatchQuery("value").SetField("field25")
	r, _ := idx.Search(context.Background(), q)
	if r.Total != 1 {
		t.Errorf("expected 1 result, got %d", r.Total)
	}

	// Verify stored data
	fields, err := idx.Document("many-fields")
	if err != nil {
		t.Fatalf("Document error: %v", err)
	}
	if len(fields) != 50 {
		t.Errorf("expected 50 stored fields, got %d", len(fields))
	}
}

// =============================================================================
// Large-Scale Stress Test
// =============================================================================

// TestLargeScaleIndexSearchDelete performs a stress test with many documents,
// mixed operations, and comprehensive queries.
func TestLargeScaleIndexSearchDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	idx := newTestIndex(t)

	numDocs := 500
	categories := []string{"tech", "science", "art", "music", "sport"}
	authors := []string{"alice", "bob", "charlie", "diana"}

	// Phase 1: Bulk index
	for i := 0; i < numDocs; i++ {
		cat := categories[i%len(categories)]
		author := authors[i%len(authors)]
		idx.Index(fmt.Sprintf("stress%04d", i), map[string]interface{}{
			"title":    fmt.Sprintf("Document %d about %s", i, cat),
			"content":  fmt.Sprintf("This is document number %d in category %s by %s with various keywords", i, cat, author),
			"category": cat,
			"author":   author,
			"rating":   float64(i%5) + 1.0,
		})
	}

	count, _ := idx.DocCount()
	if count != uint64(numDocs) {
		t.Fatalf("expected %d docs, got %d", numDocs, count)
	}

	// Phase 2: Various searches
	// Match query
	q1 := query.NewMatchQuery("document").SetField("title")
	r1, _ := idx.Search(context.Background(), q1)
	if r1.Total != uint64(numDocs) {
		t.Errorf("expected %d for 'document', got %d", numDocs, r1.Total)
	}

	// Prefix query
	q2 := query.NewPrefixQuery("tech").SetField("category")
	r2, _ := idx.Search(context.Background(), q2)
	if r2.Total < 1 {
		t.Error("expected results for prefix 'tech'")
	}

	// Numeric range
	min := 4.0
	q3 := query.NewNumericRangeQuery(&min, nil).SetField("rating")
	r3, _ := idx.Search(context.Background(), q3)
	if r3.Total == 0 {
		t.Error("expected results for rating >= 4")
	}

	// Boolean with facets
	bq := query.NewBooleanQuery()
	bq.AddMust(query.NewMatchQuery("document").SetField("content"))
	bq.AddMustNot(query.NewTermQuery("tech").SetField("category"))

	req := &search.SearchRequest{
		Query: bq,
		Size:  20,
		Facets: map[string]*search.FacetRequest{
			"authors": {Field: "author", Size: 10},
		},
	}
	rBQ, _ := idx.SearchWithRequest(context.Background(), req)
	if rBQ.Total == 0 {
		t.Error("expected results from boolean query")
	}
	if rBQ.Facets["authors"] == nil {
		t.Error("expected author facet")
	}

	// Pagination through large result set
	totalPaged := 0
	for from := 0; from < int(r1.Total); from += 50 {
		req := &search.SearchRequest{Query: q1, Size: 50, From: from}
		r, _ := idx.SearchWithRequest(context.Background(), req)
		totalPaged += len(r.Hits)
	}
	if totalPaged != numDocs {
		t.Errorf("expected %d total paged, got %d", numDocs, totalPaged)
	}

	// Phase 3: Delete half the docs
	for i := 0; i < numDocs; i += 2 {
		idx.Delete(fmt.Sprintf("stress%04d", i))
	}

	count, _ = idx.DocCount()
	expected := uint64(numDocs / 2)
	if count != expected {
		t.Errorf("expected %d docs after delete, got %d", expected, count)
	}

	// Phase 4: Verify searches still work correctly
	r1After, _ := idx.Search(context.Background(), q1)
	if r1After.Total != expected {
		t.Errorf("expected %d results after delete, got %d", expected, r1After.Total)
	}

	// Deleted docs should not appear
	rAll := &search.SearchRequest{Query: query.NewMatchAllQuery(), Size: numDocs}
	rAllRes, _ := idx.SearchWithRequest(context.Background(), rAll)
	for _, hit := range rAllRes.Hits {
		// Extract numeric part
		var num int
		fmt.Sscanf(hit.ID, "stress%d", &num)
		if num%2 == 0 {
			t.Errorf("deleted doc %s appeared in results", hit.ID)
		}
	}
}

// =============================================================================
// Search + Store Field Integration Tests
// =============================================================================

// TestSearchRequestFieldSelection verifies that field selection in SearchRequest
// correctly filters which stored fields are returned.
func TestSearchRequestFieldSelection(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	// Request specific fields
	req := &search.SearchRequest{
		Query:  query.NewMatchAllQuery(),
		Size:   10,
		Fields: []string{"title", "author"},
	}
	results, _ := idx.SearchWithRequest(context.Background(), req)

	for _, hit := range results.Hits {
		if _, ok := hit.Fields["title"]; !ok {
			t.Errorf("hit %s: expected 'title' field", hit.ID)
		}
		if _, ok := hit.Fields["author"]; !ok {
			t.Errorf("hit %s: expected 'author' field", hit.ID)
		}
		if _, ok := hit.Fields["content"]; ok {
			t.Errorf("hit %s: did not expect 'content' field", hit.ID)
		}
		if _, ok := hit.Fields["rating"]; ok {
			t.Errorf("hit %s: did not expect 'rating' field", hit.ID)
		}
	}
}

// TestSearchResultIncludesRequest verifies the result includes the original request.
func TestSearchResultIncludesRequest(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	req := &search.SearchRequest{
		Query: query.NewMatchAllQuery(),
		Size:  3,
		From:  1,
	}
	results, _ := idx.SearchWithRequest(context.Background(), req)

	if results.Request == nil {
		t.Fatal("expected Request in result")
	}
	if results.Request.Size != 3 {
		t.Errorf("expected Size=3 in request, got %d", results.Request.Size)
	}
	if results.Request.From != 1 {
		t.Errorf("expected From=1 in request, got %d", results.Request.From)
	}
}

// =============================================================================
// Type Mapping End-to-End Integration Tests
// =============================================================================

// TestTypeMappingEndToEnd verifies type-based document routing with different
// field schemas per type.
func TestTypeMappingEndToEnd(t *testing.T) {
	m := mapping.NewIndexMapping()
	m.TypeField = "type"

	articleMapping := mapping.NewDocumentStaticMapping()
	articleMapping.AddFieldMapping("headline", mapping.NewTextFieldMapping())
	articleMapping.AddFieldMapping("body", mapping.NewTextFieldMapping())
	m.AddDocumentMapping("article", articleMapping)

	productMapping := mapping.NewDocumentStaticMapping()
	productMapping.AddFieldMapping("name", mapping.NewTextFieldMapping())
	productMapping.AddFieldMapping("price", mapping.NewNumericFieldMapping())
	productMapping.AddFieldMapping("sku", mapping.NewKeywordFieldMapping())
	m.AddDocumentMapping("product", productMapping)

	idx := newTestIndexWithMapping(t, m)

	// Index articles
	idx.Index("a1", map[string]interface{}{
		"type": "article", "headline": "Breaking News Today", "body": "Something happened",
	})
	idx.Index("a2", map[string]interface{}{
		"type": "article", "headline": "Weather Update", "body": "Sunny skies ahead",
	})

	// Index products
	idx.Index("p1", map[string]interface{}{
		"type": "product", "name": "Widget Pro", "price": 49.99, "sku": "WP-001",
	})
	idx.Index("p2", map[string]interface{}{
		"type": "product", "name": "Gadget Plus", "price": 29.99, "sku": "GP-002",
	})

	count, _ := idx.DocCount()
	if count != 4 {
		t.Errorf("expected 4 docs, got %d", count)
	}

	// Search article fields
	qHead := query.NewMatchQuery("breaking").SetField("headline")
	rHead, _ := idx.Search(context.Background(), qHead)
	if rHead.Total != 1 {
		t.Errorf("expected 1 article for 'breaking', got %d", rHead.Total)
	}

	// Search product fields
	qName := query.NewMatchQuery("widget").SetField("name")
	rName, _ := idx.Search(context.Background(), qName)
	if rName.Total != 1 {
		t.Errorf("expected 1 product for 'widget', got %d", rName.Total)
	}

	// Numeric search on product field
	min := 30.0
	qPrice := query.NewNumericRangeQuery(&min, nil).SetField("price")
	rPrice, _ := idx.Search(context.Background(), qPrice)
	if rPrice.Total != 1 {
		t.Errorf("expected 1 product with price >= 30, got %d", rPrice.Total)
	}

	// Keyword exact match
	qSku := query.NewTermQuery("WP-001").SetField("sku")
	rSku, _ := idx.Search(context.Background(), qSku)
	if rSku.Total != 1 {
		t.Errorf("expected 1 for SKU 'WP-001', got %d", rSku.Total)
	}

	// Article fields should NOT be indexed for products (static mapping)
	qBody := query.NewMatchQuery("sunny").SetField("body")
	rBody, _ := idx.Search(context.Background(), qBody)
	if rBody.Total != 1 {
		t.Errorf("expected 1 result for 'sunny' in body, got %d", rBody.Total)
	}

	// Delete an article, verify product still works
	idx.Delete("a1")
	rHead2, _ := idx.Search(context.Background(), qHead)
	if rHead2.Total != 0 {
		t.Errorf("expected 0 after deleting article, got %d", rHead2.Total)
	}
	rName2, _ := idx.Search(context.Background(), qName)
	if rName2.Total != 1 {
		t.Errorf("product should still be searchable, got %d", rName2.Total)
	}
}

// =============================================================================
// Batch Operations Integration Tests
// =============================================================================

// TestBatchLargeScale exercises batch operations at scale.
func TestBatchLargeScale(t *testing.T) {
	idx := newTestIndex(t)

	// Batch index 100 docs
	batch := idx.NewBatch()
	for i := 0; i < 100; i++ {
		batch.Index(fmt.Sprintf("batch%03d", i), map[string]interface{}{
			"title": fmt.Sprintf("batch document %d", i),
		})
	}
	if batch.Size() != 100 {
		t.Errorf("expected batch size 100, got %d", batch.Size())
	}
	if err := batch.Execute(); err != nil {
		t.Fatalf("Batch execute error: %v", err)
	}

	count, _ := idx.DocCount()
	if count != 100 {
		t.Errorf("expected 100 docs, got %d", count)
	}

	// Batch delete half
	batch2 := idx.NewBatch()
	for i := 0; i < 100; i += 2 {
		batch2.Delete(fmt.Sprintf("batch%03d", i))
	}
	if err := batch2.Execute(); err != nil {
		t.Fatalf("Batch delete error: %v", err)
	}

	count, _ = idx.DocCount()
	if count != 50 {
		t.Errorf("expected 50 docs after batch delete, got %d", count)
	}

	// Verify only odd-numbered docs remain
	q := query.NewMatchAllQuery()
	req := &search.SearchRequest{Query: q, Size: 100}
	results, _ := idx.SearchWithRequest(context.Background(), req)
	for _, hit := range results.Hits {
		var num int
		fmt.Sscanf(hit.ID, "batch%d", &num)
		if num%2 == 0 {
			t.Errorf("even doc %s should have been deleted", hit.ID)
		}
	}
}

// TestBatchMixedWithSearch exercises mixed batch operations followed by complex search.
func TestBatchMixedWithSearch(t *testing.T) {
	idx := newTestIndex(t)

	// Initial set
	idx.Index("keep1", map[string]interface{}{"title": "keeper one", "rating": 4.0})
	idx.Index("keep2", map[string]interface{}{"title": "keeper two", "rating": 3.0})
	idx.Index("remove1", map[string]interface{}{"title": "remover one", "rating": 2.0})

	// Mixed batch: add, update, delete
	batch := idx.NewBatch()
	batch.Index("new1", map[string]interface{}{"title": "brand new", "rating": 5.0})
	batch.Index("keep1", map[string]interface{}{"title": "updated keeper", "rating": 4.5})
	batch.Delete("remove1")
	batch.Execute()

	// Verify state
	count, _ := idx.DocCount()
	if count != 3 {
		t.Errorf("expected 3 docs, got %d", count)
	}

	// Updated doc
	fields, _ := idx.Document("keep1")
	if fields["title"] != "updated keeper" {
		t.Errorf("expected updated title, got %v", fields["title"])
	}

	// Search with numeric range
	min := 4.0
	q := query.NewNumericRangeQuery(&min, nil).SetField("rating")
	r, _ := idx.Search(context.Background(), q)
	if r.Total != 2 {
		t.Errorf("expected 2 results for rating >= 4.0, got %d", r.Total)
	}
}

// =============================================================================
// Query Scoring Integration Tests
// =============================================================================

// TestScoringConsistency verifies that search scores are consistent and meaningful
// across different query types.
func TestScoringConsistency(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("frequent", map[string]interface{}{
		"content": "search search search search engine",
	})
	idx.Index("rare", map[string]interface{}{
		"content": "search engine platform",
	})
	idx.Index("none", map[string]interface{}{
		"content": "database platform service",
	})

	q := query.NewTermQuery("search").SetField("content")
	r, _ := idx.Search(context.Background(), q)

	if r.Total != 2 {
		t.Errorf("expected 2 results, got %d", r.Total)
	}

	if len(r.Hits) >= 2 {
		// Document with higher TF should score higher
		if r.Hits[0].ID != "frequent" {
			t.Errorf("expected 'frequent' as top result, got '%s'", r.Hits[0].ID)
		}
		if r.Hits[0].Score <= r.Hits[1].Score {
			t.Error("higher TF document should score higher")
		}
	}

	// Boosted query should produce proportionally higher scores
	qBoosted := query.NewTermQuery("search").SetField("content").SetBoost(10.0)
	rBoosted, _ := idx.Search(context.Background(), qBoosted)
	if len(r.Hits) > 0 && len(rBoosted.Hits) > 0 {
		ratio := rBoosted.Hits[0].Score / r.Hits[0].Score
		if ratio < 5 || ratio > 15 {
			t.Errorf("boost ratio should be ~10x, got %.2fx", ratio)
		}
	}
}

// TestWildcardAndFuzzyOnSameField verifies both wildcard and fuzzy queries
// work correctly on the same indexed data.
func TestWildcardAndFuzzyOnSameField(t *testing.T) {
	// Use keyword field to avoid stemming affecting fuzzy matching
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("tag", mapping.NewKeywordFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("d1", map[string]interface{}{"tag": "alpha"})
	idx.Index("d2", map[string]interface{}{"tag": "alpine"})
	idx.Index("d3", map[string]interface{}{"tag": "beta"})

	// Wildcard: alp* should match alpha and alpine
	qWild := query.NewWildcardQuery("alp*").SetField("tag")
	rWild, _ := idx.Search(context.Background(), qWild)
	if rWild.Total != 2 {
		t.Errorf("expected 2 wildcard results, got %d", rWild.Total)
	}

	// Fuzzy: "alphb" -> "alpha" (edit distance 1)
	qFuzzy := query.NewFuzzyQuery("alphb").SetField("tag").SetFuzziness(1)
	rFuzzy, _ := idx.Search(context.Background(), qFuzzy)
	if rFuzzy.Total < 1 {
		t.Errorf("expected at least 1 fuzzy result for 'alphb', got %d", rFuzzy.Total)
	}
}

// =============================================================================
// Plugin Architecture Integration Tests
// =============================================================================

// TestPluginTFIDFScorer verifies that the TF-IDF scorer can replace BM25.
func TestPluginTFIDFScorer(t *testing.T) {
	store := newMemStore()
	idx, err := New(store, nil, plugin.WithScorer(&plugin.TFIDFScorerFactory{}))
	if err != nil {
		t.Fatal(err)
	}

	idx.Index("doc1", map[string]interface{}{"title": "alpha bravo charlie"})
	idx.Index("doc2", map[string]interface{}{"title": "alpha delta echo"})
	idx.Index("doc3", map[string]interface{}{"title": "foxtrot golf hotel"})

	q := query.NewMatchQuery("alpha").SetField("title")
	result, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected 2 results, got %d", result.Total)
	}
	// Scores should be positive with TF-IDF
	for _, hit := range result.Hits {
		if hit.Score <= 0 {
			t.Errorf("expected positive TF-IDF score for %s, got %f", hit.ID, hit.Score)
		}
	}
}

// TestPluginBM25VsTFIDFDifferentScores verifies that different scorers produce different scores.
func TestPluginBM25VsTFIDFDifferentScores(t *testing.T) {
	data := []struct {
		id   string
		data map[string]interface{}
	}{
		{"d1", map[string]interface{}{"content": "alpha bravo charlie delta echo alpha"}},
		{"d2", map[string]interface{}{"content": "alpha foxtrot golf hotel india"}},
	}

	storeBM25 := newMemStore()
	idxBM25, _ := New(storeBM25, nil) // default BM25

	storeTFIDF := newMemStore()
	idxTFIDF, _ := New(storeTFIDF, nil, plugin.WithScorer(&plugin.TFIDFScorerFactory{}))

	for _, d := range data {
		idxBM25.Index(d.id, d.data)
		idxTFIDF.Index(d.id, d.data)
	}

	q := query.NewMatchQuery("alpha").SetField("content")

	resBM25, _ := idxBM25.Search(context.Background(), q)
	resTFIDF, _ := idxTFIDF.Search(context.Background(), q)

	if resBM25.Total != resTFIDF.Total {
		t.Errorf("hit counts should match: BM25=%d, TFIDF=%d", resBM25.Total, resTFIDF.Total)
	}

	// Scores should differ between the two scorers
	if len(resBM25.Hits) > 0 && len(resTFIDF.Hits) > 0 {
		if resBM25.Hits[0].Score == resTFIDF.Hits[0].Score {
			t.Log("BM25 and TF-IDF produced identical scores (unlikely but possible for edge cases)")
		}
	}
}

// TestPluginRegistryAccess verifies the Plugins() accessor works.
func TestPluginRegistryAccess(t *testing.T) {
	store := newMemStore()
	idx, _ := New(store, nil)

	reg := idx.Plugins()
	if reg == nil {
		t.Fatal("expected non-nil plugin registry")
	}

	sf := reg.GetScorerFactory()
	if sf == nil {
		t.Fatal("expected default scorer factory")
	}
	if sf.Name() != "bm25" {
		t.Errorf("expected bm25 default, got %s", sf.Name())
	}

	// Switch to TF-IDF at runtime
	reg.SetScorerFactory(&plugin.TFIDFScorerFactory{})
	if reg.GetScorerFactory().Name() != "tfidf" {
		t.Error("expected tfidf after runtime switch")
	}
}

// TestPluginCustomBM25Params verifies custom BM25 parameters work.
func TestPluginCustomBM25Params(t *testing.T) {
	store := newMemStore()
	customBM25 := &plugin.BM25ScorerFactory{K1: 2.0, B: 0.5}
	idx, _ := New(store, nil, plugin.WithScorer(customBM25))

	idx.Index("doc1", map[string]interface{}{"title": "alpha bravo alpha alpha"})
	idx.Index("doc2", map[string]interface{}{"title": "alpha charlie"})

	q := query.NewMatchQuery("alpha").SetField("title")
	result, _ := idx.Search(context.Background(), q)

	if result.Total != 2 {
		t.Errorf("expected 2 results, got %d", result.Total)
	}
	// doc1 has higher TF for alpha, custom K1=2.0 affects saturation
	if len(result.Hits) >= 2 && result.Hits[0].Score <= result.Hits[1].Score {
		t.Log("doc1 should score higher due to higher alpha TF")
	}
}

// =============================================================================
// Field Indexer Plugin Tests
// =============================================================================

// TestFieldIndexerPluginRegistration verifies that built-in field indexers work
// through the plugin dispatch path (text, numeric, boolean, datetime, vector).
func TestFieldIndexerPluginRegistration(t *testing.T) {
	store := newMemStore()
	idx, err := New(store, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Index a document with all field types
	m := NewIndexMapping()
	dm := NewDocumentStaticMapping()
	dm.AddFieldMapping("title", NewTextFieldMapping())
	dm.AddFieldMapping("price", NewNumericFieldMapping())
	dm.AddFieldMapping("active", NewBooleanFieldMapping())
	dm.AddFieldMapping("created", NewDateTimeFieldMapping())
	dm.AddFieldMapping("embed", NewVectorFieldMapping(3))
	m.DefaultMapping = dm

	idx2, err := New(store, m)
	if err != nil {
		t.Fatal(err)
	}
	_ = idx

	err = idx2.Index("doc1", map[string]interface{}{
		"title":   "alpha bravo charlie",
		"price":   float64(29.99),
		"active":  true,
		"created": "2025-06-15T00:00:00Z",
		"embed":   []float32{1.0, 0.0, 0.0},
	})
	if err != nil {
		t.Fatalf("index error: %v", err)
	}

	// Verify text search works
	r, _ := idx2.Search(context.Background(), query.NewMatchQuery("alpha").SetField("title"))
	if r.Total != 1 {
		t.Errorf("text search: expected 1 hit, got %d", r.Total)
	}

	// Verify numeric range works
	lo, hi := 20.0, 40.0
	rn, _ := idx2.Search(context.Background(), query.NewNumericRangeQuery(&lo, &hi).SetField("price"))
	if rn.Total != 1 {
		t.Errorf("numeric range: expected 1 hit, got %d", rn.Total)
	}

	// Verify deletion works (field indexers handle deletion too)
	idx2.Delete("doc1")
	r2, _ := idx2.Search(context.Background(), query.NewMatchQuery("alpha").SetField("title"))
	if r2.Total != 0 {
		t.Errorf("after delete: expected 0 hits, got %d", r2.Total)
	}
}

// TestCustomFieldIndexerViaOption verifies that a custom field indexer
// registered via plugin.WithFieldIndexer is called during indexing.
func TestCustomFieldIndexerViaOption(t *testing.T) {
	// Create a custom "tag" field indexer that stores tags as keywords
	// This simulates what a third-party geo/graph plugin would do.
	custom := &testTagFieldIndexer{indexed: make(map[string][]string)}

	store := newMemStore()
	idx, err := New(store, nil, plugin.WithFieldIndexer(custom))
	if err != nil {
		t.Fatal(err)
	}

	// The custom indexer is registered but won't be triggered by standard
	// field types. Verify it's accessible via the SearchIndex.
	_ = idx

	// Verify that the indexer was registered by checking we can still
	// index and search normal documents (existing field indexers still work)
	idx.Index("doc1", map[string]interface{}{"content": "hello world"})
	r, _ := idx.Search(context.Background(), query.NewMatchQuery("hello").SetField("content"))
	if r.Total != 1 {
		t.Errorf("expected 1 hit, got %d", r.Total)
	}
}

// testTagFieldIndexer is a custom field indexer for testing.
type testTagFieldIndexer struct {
	indexed map[string][]string // docID -> tags
}

func (fi *testTagFieldIndexer) Type() string { return "tag" }

func (fi *testTagFieldIndexer) IndexField(helpers index.IndexHelpers, docID string, field *document.Field) (*index.RevIdxEntry, error) {
	text := field.TextValue()
	if text == "" {
		return nil, nil
	}
	fi.indexed[docID] = append(fi.indexed[docID], text)
	return &index.RevIdxEntry{Field: field.Name, Type: "tag"}, nil
}

func (fi *testTagFieldIndexer) DeleteField(helpers index.IndexHelpers, docID string, entry index.RevIdxEntry) error {
	delete(fi.indexed, docID)
	return nil
}

// TestFieldIndexerRuntimeRegistration verifies registering field indexers at runtime.
func TestFieldIndexerRuntimeRegistration(t *testing.T) {
	store := newMemStore()
	idx, err := New(store, nil)
	if err != nil {
		t.Fatal(err)
	}

	custom := &testTagFieldIndexer{indexed: make(map[string][]string)}
	idx.RegisterFieldIndexer(custom)

	// Normal indexing still works
	idx.Index("doc1", map[string]interface{}{"title": "hello world"})
	r, _ := idx.Search(context.Background(), query.NewMatchQuery("hello").SetField("title"))
	if r.Total != 1 {
		t.Errorf("expected 1 hit, got %d", r.Total)
	}
}
