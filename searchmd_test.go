package searchmd

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/readmedotmd/search.md/mapping"
	"github.com/readmedotmd/search.md/search"
	"github.com/readmedotmd/search.md/search/query"
)

func newTestIndex(t *testing.T) *SearchIndex {
	t.Helper()
	store := newMemStore()
	idx, err := New(store, nil)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	return idx
}

func newTestIndexWithMapping(t *testing.T, m *mapping.IndexMapping) *SearchIndex {
	t.Helper()
	store := newMemStore()
	idx, err := New(store, m)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	return idx
}

func indexTestDocuments(t *testing.T, idx *SearchIndex) {
	t.Helper()

	docs := []struct {
		id   string
		data map[string]interface{}
	}{
		{"doc1", map[string]interface{}{
			"title":   "The quick brown fox",
			"content": "The quick brown fox jumps over the lazy dog",
			"author":  "Alice",
			"rating":  4.5,
		}},
		{"doc2", map[string]interface{}{
			"title":   "Go programming language",
			"content": "Go is an open source programming language that makes it easy to build simple, reliable software",
			"author":  "Bob",
			"rating":  4.8,
		}},
		{"doc3", map[string]interface{}{
			"title":   "Quick start guide",
			"content": "This quick start guide will help you get started with the search engine",
			"author":  "Alice",
			"rating":  3.5,
		}},
		{"doc4", map[string]interface{}{
			"title":   "Advanced search techniques",
			"content": "Learn about fuzzy search, wildcard search, and phrase search techniques",
			"author":  "Charlie",
			"rating":  4.2,
		}},
		{"doc5", map[string]interface{}{
			"title":   "The brown bear",
			"content": "A brown bear was seen in the forest near the river",
			"author":  "Bob",
			"rating":  3.0,
		}},
	}

	for _, d := range docs {
		if err := idx.Index(d.id, d.data); err != nil {
			t.Fatalf("failed to index doc %s: %v", d.id, err)
		}
	}
}

func TestIndexAndDocCount(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	count, err := idx.DocCount()
	if err != nil {
		t.Fatalf("DocCount error: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 docs, got %d", count)
	}
}

func TestGetDocument(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	fields, err := idx.Document("doc1")
	if err != nil {
		t.Fatalf("Document error: %v", err)
	}

	if fields["title"] != "The quick brown fox" {
		t.Errorf("unexpected title: %v", fields["title"])
	}
}

func TestDeleteDocument(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	if err := idx.Delete("doc1"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	count, _ := idx.DocCount()
	if count != 4 {
		t.Errorf("expected 4 docs after delete, got %d", count)
	}

	_, err := idx.Document("doc1")
	if err == nil {
		t.Error("expected error for deleted document")
	}
}

func TestTermQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewTermQuery("quick").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total == 0 {
		t.Error("expected at least 1 result for term 'quick'")
	}

	// Both doc1 ("The quick brown fox") and doc3 ("Quick start guide") should match
	if results.Total < 2 {
		t.Errorf("expected at least 2 results, got %d", results.Total)
	}
}

func TestMatchQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchQuery("quick brown").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total == 0 {
		t.Error("expected results for match 'quick brown'")
	}

	// doc1 should score highest (has both terms)
	if len(results.Hits) > 0 && results.Hits[0].ID != "doc1" {
		t.Errorf("expected doc1 as top result, got %s", results.Hits[0].ID)
	}
}

func TestMatchQueryAnd(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchQuery("quick brown").SetField("title").SetOperator("and")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// Only doc1 has both "quick" and "brown" in title
	if results.Total != 1 {
		t.Errorf("expected 1 result for AND query, got %d", results.Total)
	}
}

func TestMatchPhraseQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchPhraseQuery("quick brown fox").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total != 1 {
		t.Errorf("expected 1 result for phrase 'quick brown fox', got %d", results.Total)
	}
	if len(results.Hits) > 0 && results.Hits[0].ID != "doc1" {
		t.Errorf("expected doc1, got %s", results.Hits[0].ID)
	}
}

func TestPrefixQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewPrefixQuery("bro").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// "brown" matches "bro" prefix - doc1 and doc5
	if results.Total < 2 {
		t.Errorf("expected at least 2 results for prefix 'bro', got %d", results.Total)
	}
}

func TestFuzzyQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	// "quik" should match "quick" with fuzziness 1
	q := query.NewFuzzyQuery("quik").SetField("title").SetFuzziness(1)
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total == 0 {
		t.Error("expected results for fuzzy query 'quik'")
	}
}

func TestWildcardQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewWildcardQuery("bro*").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total < 2 {
		t.Errorf("expected at least 2 results for wildcard 'bro*', got %d", results.Total)
	}
}

func TestRegexpQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewRegexpQuery("^qu.*").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total == 0 {
		t.Error("expected results for regexp '^qu.*'")
	}
}

func TestBooleanQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	bq := query.NewBooleanQuery()
	bq.AddMust(query.NewMatchQuery("search").SetField("content"))
	bq.AddMustNot(query.NewMatchQuery("fuzzy").SetField("content"))

	results, err := idx.Search(context.Background(), bq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// Should include doc3 (has "search") but not doc4 (has both "search" and "fuzzy")
	for _, hit := range results.Hits {
		if hit.ID == "doc4" {
			t.Error("doc4 should be excluded by must_not")
		}
	}
}

func TestConjunctionQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	cq := query.NewConjunctionQuery(
		query.NewTermQuery("quick").SetField("content"),
		query.NewTermQuery("fox").SetField("content"),
	)

	results, err := idx.Search(context.Background(), cq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// Only doc1 has both "quick" and "fox" in content
	if results.Total != 1 {
		t.Errorf("expected 1 result for conjunction, got %d", results.Total)
	}
}

func TestDisjunctionQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	dq := query.NewDisjunctionQuery(
		query.NewTermQuery("fox").SetField("content"),
		query.NewTermQuery("bear").SetField("content"),
	)

	results, err := idx.Search(context.Background(), dq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// doc1 (fox) and doc5 (bear) should match
	if results.Total < 2 {
		t.Errorf("expected at least 2 results for disjunction, got %d", results.Total)
	}
}

func TestMatchAllQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchAllQuery()
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total != 5 {
		t.Errorf("expected 5 results for match all, got %d", results.Total)
	}
}

func TestMatchNoneQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchNoneQuery()
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total != 0 {
		t.Errorf("expected 0 results for match none, got %d", results.Total)
	}
}

func TestNumericRangeQuery(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	min := 4.0
	max := 5.0
	q := query.NewNumericRangeQuery(&min, &max).SetField("rating")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// doc1 (4.5), doc2 (4.8), doc4 (4.2) have ratings >= 4.0
	if results.Total < 3 {
		t.Errorf("expected at least 3 results for rating >= 4.0, got %d", results.Total)
	}
}

func TestDateRangeQuery(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentMapping()
	dm.AddFieldMapping("created", mapping.NewDateTimeFieldMapping())
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("doc1", map[string]interface{}{
		"title":   "Old doc",
		"created": "2024-01-15T00:00:00Z",
	})
	idx.Index("doc2", map[string]interface{}{
		"title":   "New doc",
		"created": "2025-06-15T00:00:00Z",
	})
	idx.Index("doc3", map[string]interface{}{
		"title":   "Newest doc",
		"created": "2026-01-15T00:00:00Z",
	})

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	q := query.NewDateRangeQuery(&start, &end).SetField("created")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// doc2 and doc3 should match
	if results.Total != 2 {
		t.Errorf("expected 2 results for date range, got %d", results.Total)
	}
}

func TestVectorSearch(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(3))
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("doc1", map[string]interface{}{
		"title":     "cat document",
		"embedding": []float64{1.0, 0.0, 0.0},
	})
	idx.Index("doc2", map[string]interface{}{
		"title":     "dog document",
		"embedding": []float64{0.0, 1.0, 0.0},
	})
	idx.Index("doc3", map[string]interface{}{
		"title":     "similar to cat",
		"embedding": []float64{0.9, 0.1, 0.0},
	})

	q := query.NewKNNQuery([]float32{1.0, 0.0, 0.0}, 2).SetField("embedding")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total < 2 {
		t.Errorf("expected at least 2 results for KNN, got %d", results.Total)
	}

	// doc1 should be most similar to query [1,0,0]
	if len(results.Hits) > 0 && results.Hits[0].ID != "doc1" {
		t.Errorf("expected doc1 as top result, got %s", results.Hits[0].ID)
	}

	// doc3 should be second most similar
	if len(results.Hits) > 1 && results.Hits[1].ID != "doc3" {
		t.Errorf("expected doc3 as second result, got %s", results.Hits[1].ID)
	}
}

func TestCodeSearch(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("code", mapping.NewCodeFieldMapping())
	dm.AddFieldMapping("filename", mapping.NewKeywordFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("file1", map[string]interface{}{
		"filename": "main.go",
		"code":     "func getUserName(ctx context.Context) string { return ctx.Value(userKey).(string) }",
	})
	idx.Index("file2", map[string]interface{}{
		"filename": "handler.go",
		"code":     "func handleRequest(w http.ResponseWriter, r *http.Request) { user := getUserName(r.Context()) }",
	})
	idx.Index("file3", map[string]interface{}{
		"filename": "utils.go",
		"code":     "func parseJSON(data []byte) (map[string]interface{}, error) { var result map[string]interface{} }",
	})

	// Search for camelCase function name
	q := query.NewMatchQuery("getUserName").SetField("code").SetAnalyzer("code")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total == 0 {
		t.Error("expected results for code search 'getUserName'")
	}

	// Search for a sub-token from camelCase splitting
	q2 := query.NewTermQuery("user").SetField("code")
	results2, err := idx.Search(context.Background(), q2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results2.Total == 0 {
		t.Error("expected results for sub-token 'user' from camelCase splitting")
	}

	// Search for context (dot notation: context.Context splits into "context")
	q3 := query.NewTermQuery("context").SetField("code")
	results3, err := idx.Search(context.Background(), q3)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results3.Total == 0 {
		t.Error("expected results for 'context' from dot notation splitting")
	}
}

func TestPagination(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchAllQuery()

	// Page 1
	req := &search.SearchRequest{
		Query: q,
		Size:  2,
		From:  0,
	}
	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total != 5 {
		t.Errorf("expected 5 total, got %d", results.Total)
	}
	if len(results.Hits) != 2 {
		t.Errorf("expected 2 hits on page 1, got %d", len(results.Hits))
	}

	// Page 2
	req.From = 2
	results2, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results2.Hits) != 2 {
		t.Errorf("expected 2 hits on page 2, got %d", len(results2.Hits))
	}

	// Page 3 (last page)
	req.From = 4
	results3, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results3.Hits) != 1 {
		t.Errorf("expected 1 hit on page 3, got %d", len(results3.Hits))
	}
}

func TestHighlighting(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchQuery("quick fox").SetField("content")
	req := &search.SearchRequest{
		Query: q,
		Size:  10,
		Highlight: &search.HighlightRequest{
			Fields: []string{"content"},
		},
	}

	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total == 0 {
		t.Fatal("expected results")
	}

	// Check that fragments are generated
	for _, hit := range results.Hits {
		if hit.ID == "doc1" {
			if len(hit.Fragments) == 0 {
				t.Error("expected highlight fragments for doc1")
			}
			if frags, ok := hit.Fragments["content"]; ok {
				found := false
				for _, f := range frags {
					if len(f) > 0 {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected non-empty fragments")
				}
			}
		}
	}
}

func TestFacets(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchAllQuery()
	req := &search.SearchRequest{
		Query: q,
		Size:  10,
		Facets: map[string]*search.FacetRequest{
			"rating_ranges": {
				Field: "rating",
				Size:  10,
				NumericRanges: []search.NumericRange{
					{Name: "low", Min: floatPtr(0), Max: floatPtr(3.5)},
					{Name: "medium", Min: floatPtr(3.5), Max: floatPtr(4.5)},
					{Name: "high", Min: floatPtr(4.5), Max: floatPtr(5.0)},
				},
			},
		},
	}

	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Facets == nil {
		t.Fatal("expected facet results")
	}

	ratingFacet, ok := results.Facets["rating_ranges"]
	if !ok {
		t.Fatal("expected 'rating_ranges' facet")
	}

	if len(ratingFacet.NumericRanges) != 3 {
		t.Errorf("expected 3 numeric ranges, got %d", len(ratingFacet.NumericRanges))
	}
}

func TestBatch(t *testing.T) {
	idx := newTestIndex(t)

	batch := idx.NewBatch()
	batch.Index("b1", map[string]interface{}{"title": "Batch doc 1"})
	batch.Index("b2", map[string]interface{}{"title": "Batch doc 2"})
	batch.Index("b3", map[string]interface{}{"title": "Batch doc 3"})

	if batch.Size() != 3 {
		t.Errorf("expected batch size 3, got %d", batch.Size())
	}

	if err := batch.Execute(); err != nil {
		t.Fatalf("Batch execute error: %v", err)
	}

	count, _ := idx.DocCount()
	if count != 3 {
		t.Errorf("expected 3 docs after batch, got %d", count)
	}

	// Batch delete
	batch2 := idx.NewBatch()
	batch2.Delete("b1")
	batch2.Delete("b2")
	if err := batch2.Execute(); err != nil {
		t.Fatalf("Batch delete error: %v", err)
	}

	count, _ = idx.DocCount()
	if count != 1 {
		t.Errorf("expected 1 doc after batch delete, got %d", count)
	}
}

func TestDocumentUpdate(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("doc1", map[string]interface{}{"title": "Original title"})
	idx.Index("doc1", map[string]interface{}{"title": "Updated title"})

	count, _ := idx.DocCount()
	if count != 1 {
		t.Errorf("expected 1 doc after update, got %d", count)
	}

	fields, err := idx.Document("doc1")
	if err != nil {
		t.Fatalf("Document error: %v", err)
	}
	if fields["title"] != "Updated title" {
		t.Errorf("expected 'Updated title', got '%v'", fields["title"])
	}

	// Search for old term should not find anything
	q := query.NewMatchQuery("original").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results.Total != 0 {
		t.Errorf("expected 0 results for old term, got %d", results.Total)
	}

	// Search for new term should find the doc
	q2 := query.NewMatchQuery("updated").SetField("title")
	results2, err := idx.Search(context.Background(), q2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results2.Total != 1 {
		t.Errorf("expected 1 result for new term, got %d", results2.Total)
	}
}

func TestBM25Scoring(t *testing.T) {
	idx := newTestIndex(t)

	// Index docs where one has the search term more frequently
	idx.Index("rare", map[string]interface{}{
		"content": "the cat sat on the mat",
	})
	idx.Index("frequent", map[string]interface{}{
		"content": "cat cat cat loves being a cat",
	})
	idx.Index("none", map[string]interface{}{
		"content": "the dog played in the park",
	})

	q := query.NewTermQuery("cat").SetField("content")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total != 2 {
		t.Errorf("expected 2 results, got %d", results.Total)
	}

	// Higher frequency doc should score higher
	if len(results.Hits) >= 2 {
		if results.Hits[0].ID != "frequent" {
			t.Errorf("expected 'frequent' as top result, got '%s'", results.Hits[0].ID)
		}
	}
}

func TestFieldSpecificSearch(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	// Search in title field
	q1 := query.NewMatchQuery("search").SetField("title")
	r1, _ := idx.Search(context.Background(), q1)

	// Search in content field
	q2 := query.NewMatchQuery("search").SetField("content")
	r2, _ := idx.Search(context.Background(), q2)

	// "search" appears in doc4 title and doc3/doc4 content
	if r1.Total == 0 {
		t.Error("expected results in title field")
	}
	if r2.Total == 0 {
		t.Error("expected results in content field")
	}
}

func TestStaticMapping(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	// "content" is NOT mapped
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("doc1", map[string]interface{}{
		"title":   "Hello World",
		"content": "This should not be indexed",
	})

	// Search in unmapped field should find nothing
	q := query.NewMatchQuery("indexed").SetField("content")
	results, _ := idx.Search(context.Background(), q)
	if results.Total != 0 {
		t.Errorf("expected 0 results in unmapped field, got %d", results.Total)
	}

	// Search in mapped field should work
	q2 := query.NewMatchQuery("hello").SetField("title")
	results2, _ := idx.Search(context.Background(), q2)
	if results2.Total != 1 {
		t.Errorf("expected 1 result in mapped field, got %d", results2.Total)
	}
}

func TestEmptyIndex(t *testing.T) {
	idx := newTestIndex(t)

	count, _ := idx.DocCount()
	if count != 0 {
		t.Errorf("expected 0 docs, got %d", count)
	}

	q := query.NewMatchQuery("anything").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results.Total != 0 {
		t.Errorf("expected 0 results, got %d", results.Total)
	}
}

func TestEmptyDocumentID(t *testing.T) {
	idx := newTestIndex(t)

	err := idx.Index("", map[string]interface{}{"title": "test"})
	if err == nil {
		t.Error("expected error for empty document ID")
	}
}

func TestSearchResultMetadata(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchQuery("quick").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Status.Total != 1 {
		t.Errorf("expected status total 1, got %d", results.Status.Total)
	}
	if results.Status.Successful != 1 {
		t.Errorf("expected status successful 1, got %d", results.Status.Successful)
	}
	if results.Took <= 0 {
		t.Error("expected positive Took duration")
	}
	if results.MaxScore <= 0 {
		t.Error("expected positive MaxScore")
	}
	// Scores should be descending
	for i := 1; i < len(results.Hits); i++ {
		if results.Hits[i].Score > results.Hits[i-1].Score {
			t.Error("hits should be sorted by score descending")
		}
	}
}

func TestSearchReturnsStoredFields(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewTermQuery("quick").SetField("title")
	results, _ := idx.Search(context.Background(), q)

	for _, hit := range results.Hits {
		if hit.Fields == nil {
			t.Errorf("expected stored fields for hit %s", hit.ID)
		}
		if _, ok := hit.Fields["title"]; !ok {
			t.Errorf("expected 'title' in stored fields for hit %s", hit.ID)
		}
	}
}

func TestSelectiveFieldLoading(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchAllQuery()
	req := &search.SearchRequest{
		Query:  q,
		Size:   10,
		Fields: []string{"title"},
	}
	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	for _, hit := range results.Hits {
		if _, ok := hit.Fields["title"]; !ok {
			t.Errorf("expected 'title' field for hit %s", hit.ID)
		}
		if _, ok := hit.Fields["content"]; ok {
			t.Errorf("did not expect 'content' field for hit %s (only requested 'title')", hit.ID)
		}
	}
}

func TestMultipleFieldSearch(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	// "brown" appears in title of doc1 and doc5
	q := query.NewTermQuery("brown").SetField("title")
	results, _ := idx.Search(context.Background(), q)
	if results.Total < 2 {
		t.Errorf("expected at least 2 results for 'brown' in title, got %d", results.Total)
	}

	// "brown" in content too
	q2 := query.NewTermQuery("brown").SetField("content")
	results2, _ := idx.Search(context.Background(), q2)
	if results2.Total < 2 {
		t.Errorf("expected at least 2 results for 'brown' in content, got %d", results2.Total)
	}
}

func TestBoostAffectsScoring(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	// Same query with different boosts
	q1 := query.NewTermQuery("quick").SetField("title").SetBoost(1.0)
	r1, _ := idx.Search(context.Background(), q1)

	q2 := query.NewTermQuery("quick").SetField("title").SetBoost(10.0)
	r2, _ := idx.Search(context.Background(), q2)

	if len(r1.Hits) > 0 && len(r2.Hits) > 0 {
		if r2.Hits[0].Score <= r1.Hits[0].Score {
			t.Error("boosted query should produce higher scores")
		}
	}
}

func TestDisjunctionMinMatch(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	// Require at least 2 of 3 terms to match
	dq := query.NewDisjunctionQuery(
		query.NewTermQuery("quick").SetField("content"),
		query.NewTermQuery("brown").SetField("content"),
		query.NewTermQuery("bear").SetField("content"),
	).SetMin(2)

	results, err := idx.Search(context.Background(), dq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// doc1 has "quick" and "brown" -> matches
	// doc5 has "brown" and "bear" -> matches
	// doc3 has "quick" only -> does NOT match
	for _, hit := range results.Hits {
		if hit.ID == "doc3" {
			t.Error("doc3 should not match with min=2 (only has 'quick')")
		}
	}
}

func TestBooleanQueryShouldBoost(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	// must: search in content, should: brown (should boost docs with "brown")
	bq := query.NewBooleanQuery()
	bq.AddMust(query.NewMatchQuery("search").SetField("content"))
	bq.AddShould(query.NewTermQuery("techniqu").SetField("content"))

	results, err := idx.Search(context.Background(), bq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// All must results should be present
	if results.Total == 0 {
		t.Error("expected results from boolean query")
	}
}

func TestBooleanOnlyMustNot(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	// Only must_not: should return all docs except excluded ones
	bq := query.NewBooleanQuery()
	bq.AddMustNot(query.NewTermQuery("quick").SetField("content"))

	results, err := idx.Search(context.Background(), bq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	for _, hit := range results.Hits {
		if hit.ID == "doc1" {
			t.Error("doc1 should be excluded by must_not")
		}
	}
	// Should have docs without "quick"
	if results.Total == 0 {
		t.Error("expected some results for must_not only query")
	}
}

func TestLargeDocumentSet(t *testing.T) {
	idx := newTestIndex(t)

	// Index 100 documents
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("doc%d", i)
		data := map[string]interface{}{
			"title":   fmt.Sprintf("Document number %d about testing", i),
			"content": fmt.Sprintf("This is document %d with various content about search engines and indexing", i),
			"num":     float64(i),
		}
		if err := idx.Index(id, data); err != nil {
			t.Fatalf("failed to index doc %d: %v", i, err)
		}
	}

	count, _ := idx.DocCount()
	if count != 100 {
		t.Errorf("expected 100 docs, got %d", count)
	}

	// Search should work
	q := query.NewMatchQuery("testing").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results.Total != 100 {
		t.Errorf("expected 100 results for 'testing', got %d", results.Total)
	}

	// Pagination through all results
	totalHits := 0
	for from := 0; from < 100; from += 10 {
		req := &search.SearchRequest{Query: q, Size: 10, From: from}
		r, _ := idx.SearchWithRequest(context.Background(), req)
		totalHits += len(r.Hits)
	}
	if totalHits != 100 {
		t.Errorf("expected 100 total paginated hits, got %d", totalHits)
	}
}

func TestSpecialCharactersInDocID(t *testing.T) {
	idx := newTestIndex(t)

	ids := []string{"doc:2", "doc.3", "doc-4", "doc_5", "doc with spaces"}
	for _, id := range ids {
		err := idx.Index(id, map[string]interface{}{"title": "test " + id})
		if err != nil {
			t.Fatalf("failed to index doc '%s': %v", id, err)
		}
	}

	count, _ := idx.DocCount()
	if count != uint64(len(ids)) {
		t.Errorf("expected %d docs, got %d", len(ids), count)
	}

	for _, id := range ids {
		fields, err := idx.Document(id)
		if err != nil {
			t.Errorf("failed to get doc '%s': %v", id, err)
		}
		if fields["title"] != "test "+id {
			t.Errorf("unexpected title for '%s': %v", id, fields["title"])
		}
	}
}

func TestUnicodeContent(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("zh", map[string]interface{}{"title": "你好世界"})
	idx.Index("emoji", map[string]interface{}{"title": "hello 🌍 world"})
	idx.Index("jp", map[string]interface{}{"title": "こんにちは世界"})
	idx.Index("mixed", map[string]interface{}{"title": "café résumé naïve"})

	count, _ := idx.DocCount()
	if count != 4 {
		t.Errorf("expected 4 docs, got %d", count)
	}

	// Search for unicode content
	q := query.NewTermQuery("café").SetField("title")
	results, _ := idx.Search(context.Background(), q)
	if results.Total != 1 {
		t.Errorf("expected 1 result for 'café', got %d", results.Total)
	}
}

func TestNumericRangeEdgeCases(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	// Only min, no max
	min := 4.0
	q := query.NewNumericRangeQuery(&min, nil).SetField("rating")
	results, _ := idx.Search(context.Background(), q)
	if results.Total < 3 {
		t.Errorf("expected at least 3 results for rating >= 4.0, got %d", results.Total)
	}

	// Only max, no min
	max := 3.5
	q2 := query.NewNumericRangeQuery(nil, &max).SetField("rating")
	results2, _ := idx.Search(context.Background(), q2)
	if results2.Total < 2 {
		t.Errorf("expected at least 2 results for rating <= 3.5, got %d", results2.Total)
	}
}

func TestVectorSearchOrdering(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(4))
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	// Index vectors with known similarities
	idx.Index("a", map[string]interface{}{"embedding": []float64{1, 0, 0, 0}})
	idx.Index("b", map[string]interface{}{"embedding": []float64{0.7, 0.7, 0, 0}})
	idx.Index("c", map[string]interface{}{"embedding": []float64{0, 0, 1, 0}})
	idx.Index("d", map[string]interface{}{"embedding": []float64{0, 0, 0, 1}})

	q := query.NewKNNQuery([]float32{1, 0, 0, 0}, 4).SetField("embedding")
	results, _ := idx.Search(context.Background(), q)

	// "c" and "d" are orthogonal (zero cosine similarity) so only "a" and "b" match
	if results.Total != 2 {
		t.Errorf("expected 2 results (orthogonal vectors excluded), got %d", results.Total)
	}

	// "a" should be first (exact match), "b" second (most similar)
	if results.Hits[0].ID != "a" {
		t.Errorf("expected 'a' as top result, got '%s'", results.Hits[0].ID)
	}
	if len(results.Hits) > 1 && results.Hits[1].ID != "b" {
		t.Errorf("expected 'b' as second result, got '%s'", results.Hits[1].ID)
	}
}

func TestTermFacets(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchAllQuery()
	req := &search.SearchRequest{
		Query: q,
		Size:  10,
		Facets: map[string]*search.FacetRequest{
			"authors": {
				Field: "author",
				Size:  5,
			},
		},
	}

	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	authorFacet, ok := results.Facets["authors"]
	if !ok {
		t.Fatal("expected 'authors' facet")
	}
	if authorFacet.Total == 0 {
		t.Error("expected non-zero facet total")
	}
}

func TestDateFacets(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentMapping()
	dm.AddFieldMapping("created", mapping.NewDateTimeFieldMapping())
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("d1", map[string]interface{}{"title": "old", "created": "2023-01-01T00:00:00Z"})
	idx.Index("d2", map[string]interface{}{"title": "mid", "created": "2024-06-01T00:00:00Z"})
	idx.Index("d3", map[string]interface{}{"title": "new", "created": "2025-11-01T00:00:00Z"})

	start2024 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end2024 := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
	start2025 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end2025 := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)

	q := query.NewMatchAllQuery()
	req := &search.SearchRequest{
		Query: q,
		Size:  10,
		Facets: map[string]*search.FacetRequest{
			"by_year": {
				Field: "created",
				Size:  5,
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

	yearFacet, ok := results.Facets["by_year"]
	if !ok {
		t.Fatal("expected 'by_year' facet")
	}
	if len(yearFacet.DateRanges) != 2 {
		t.Errorf("expected 2 date ranges, got %d", len(yearFacet.DateRanges))
	}
}

func TestHighlightingMultipleFields(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchQuery("quick").SetField("content")
	req := &search.SearchRequest{
		Query:     q,
		Size:      10,
		Highlight: &search.HighlightRequest{},
	}

	results, _ := idx.SearchWithRequest(context.Background(), req)

	// When no fields specified, all text fields should be highlighted
	for _, hit := range results.Hits {
		if hit.ID == "doc1" && len(hit.Fragments) == 0 {
			t.Error("expected fragments for doc1")
		}
	}
}

func TestCodeSearchMultiplePatterns(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("code", mapping.NewCodeFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("f1", map[string]interface{}{
		"code": "func processHTTPRequest(w http.ResponseWriter) error",
	})
	idx.Index("f2", map[string]interface{}{
		"code": "func get_user_profile(user_id int) *UserProfile",
	})
	idx.Index("f3", map[string]interface{}{
		"code": "func (s *Server) HandleConnection(conn net.Conn) error",
	})

	// Search for snake_case sub-token
	q1 := query.NewTermQuery("profile").SetField("code")
	r1, _ := idx.Search(context.Background(), q1)
	if r1.Total == 0 {
		t.Error("expected results for 'profile' from snake_case splitting")
	}

	// Search for PascalCase sub-token
	q2 := query.NewTermQuery("handle").SetField("code")
	r2, _ := idx.Search(context.Background(), q2)
	if r2.Total == 0 {
		t.Error("expected results for 'handle' from PascalCase splitting")
	}

	// Search for HTTP (acronym in camelCase)
	q3 := query.NewTermQuery("http").SetField("code")
	r3, _ := idx.Search(context.Background(), q3)
	if r3.Total == 0 {
		t.Error("expected results for 'http'")
	}
}

func TestBatchMixed(t *testing.T) {
	idx := newTestIndex(t)

	// Initial docs
	idx.Index("keep", map[string]interface{}{"title": "keep me"})
	idx.Index("remove", map[string]interface{}{"title": "remove me"})

	// Batch: add new, delete old, update existing
	batch := idx.NewBatch()
	batch.Index("new1", map[string]interface{}{"title": "brand new"})
	batch.Delete("remove")
	batch.Index("keep", map[string]interface{}{"title": "updated keep"})

	if err := batch.Execute(); err != nil {
		t.Fatalf("Batch execute error: %v", err)
	}

	count, _ := idx.DocCount()
	if count != 2 {
		t.Errorf("expected 2 docs, got %d", count)
	}

	// Verify updated doc
	fields, _ := idx.Document("keep")
	if fields["title"] != "updated keep" {
		t.Errorf("expected 'updated keep', got '%v'", fields["title"])
	}

	// Verify deleted doc is gone
	_, err := idx.Document("remove")
	if err == nil {
		t.Error("expected error for deleted document")
	}

	// Verify new doc exists
	fields2, err := idx.Document("new1")
	if err != nil {
		t.Fatalf("expected new1 to exist: %v", err)
	}
	if fields2["title"] != "brand new" {
		t.Errorf("unexpected title: %v", fields2["title"])
	}
}

func TestSearchNonExistentField(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchQuery("test").SetField("nonexistent")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results.Total != 0 {
		t.Errorf("expected 0 results for nonexistent field, got %d", results.Total)
	}
}

func TestPaginationBeyondResults(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchAllQuery()
	req := &search.SearchRequest{Query: q, Size: 10, From: 1000}
	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results.Hits) != 0 {
		t.Errorf("expected 0 hits for from=1000, got %d", len(results.Hits))
	}
	// Total should still reflect all matches
	if results.Total != 5 {
		t.Errorf("expected total 5, got %d", results.Total)
	}
}

func TestTypeMapping(t *testing.T) {
	m := mapping.NewIndexMapping()
	m.TypeField = "type"

	articleMapping := mapping.NewDocumentStaticMapping()
	articleMapping.AddFieldMapping("headline", mapping.NewTextFieldMapping())
	m.AddDocumentMapping("article", articleMapping)

	productMapping := mapping.NewDocumentStaticMapping()
	productMapping.AddFieldMapping("name", mapping.NewTextFieldMapping())
	productMapping.AddFieldMapping("price", mapping.NewNumericFieldMapping())
	m.AddDocumentMapping("product", productMapping)

	idx := newTestIndexWithMapping(t, m)

	idx.Index("a1", map[string]interface{}{"type": "article", "headline": "Breaking News"})
	idx.Index("p1", map[string]interface{}{"type": "product", "name": "Widget", "price": 9.99})

	// Search in article field
	q := query.NewMatchQuery("breaking").SetField("headline")
	results, _ := idx.Search(context.Background(), q)
	if results.Total != 1 {
		t.Errorf("expected 1 article result, got %d", results.Total)
	}

	// Search in product field
	q2 := query.NewMatchQuery("widget").SetField("name")
	results2, _ := idx.Search(context.Background(), q2)
	if results2.Total != 1 {
		t.Errorf("expected 1 product result, got %d", results2.Total)
	}
}

func TestStructIndexing(t *testing.T) {
	type Article struct {
		Title   string `json:"title"`
		Body    string `json:"body"`
		Views   int    `json:"views"`
		Premium bool   `json:"premium"`
	}

	idx := newTestIndex(t)

	err := idx.Index("art1", Article{
		Title:   "Go Concurrency Patterns",
		Body:    "Goroutines and channels are the building blocks",
		Views:   1500,
		Premium: true,
	})
	if err != nil {
		t.Fatalf("Index struct error: %v", err)
	}

	q := query.NewMatchQuery("concurrency").SetField("title")
	results, _ := idx.Search(context.Background(), q)
	if results.Total != 1 {
		t.Errorf("expected 1 result, got %d", results.Total)
	}
}

func TestDeleteNonExistent(t *testing.T) {
	idx := newTestIndex(t)

	// Deleting a non-existent doc should not error
	err := idx.Delete("nonexistent")
	if err != nil {
		t.Errorf("unexpected error deleting non-existent doc: %v", err)
	}

	count, _ := idx.DocCount()
	if count != 0 {
		t.Errorf("expected 0 docs, got %d", count)
	}
}

func TestReindexSameDocMultipleTimes(t *testing.T) {
	idx := newTestIndex(t)

	for i := 0; i < 10; i++ {
		err := idx.Index("doc1", map[string]interface{}{
			"title": fmt.Sprintf("version %d", i),
		})
		if err != nil {
			t.Fatalf("Index error on iteration %d: %v", i, err)
		}
	}

	count, _ := idx.DocCount()
	if count != 1 {
		t.Errorf("expected 1 doc after 10 re-indexes, got %d", count)
	}

	fields, _ := idx.Document("doc1")
	if fields["title"] != "version 9" {
		t.Errorf("expected 'version 9', got '%v'", fields["title"])
	}
}

func floatPtr(f float64) *float64 {
	return &f
}

// =============================================================================
// Memory Efficiency Tests
// =============================================================================

// TestBoundedHeapTopN verifies that SearchWithRequest returns the correct top-N
// results when there are many more matches than the requested page size.
func TestBoundedHeapTopN(t *testing.T) {
	idx := newTestIndex(t)

	// Index 50 docs with descending scores (doc0 scores highest for "alpha")
	for i := 0; i < 50; i++ {
		// Repeat term proportionally so BM25 scores vary
		content := ""
		for j := 0; j < 50-i; j++ {
			content += "alpha "
		}
		content += "filler text padding"
		idx.Index(fmt.Sprintf("doc%02d", i), map[string]interface{}{
			"content": content,
		})
	}

	q := query.NewTermQuery("alpha").SetField("content")

	// Request only top 5
	req := &search.SearchRequest{Query: q, Size: 5, From: 0}
	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total != 50 {
		t.Errorf("expected 50 total hits, got %d", results.Total)
	}
	if len(results.Hits) != 5 {
		t.Errorf("expected 5 hits, got %d", len(results.Hits))
	}

	// Scores must be descending
	for i := 1; i < len(results.Hits); i++ {
		if results.Hits[i].Score > results.Hits[i-1].Score {
			t.Error("hits should be in descending score order")
		}
	}

	// Second page
	req2 := &search.SearchRequest{Query: q, Size: 5, From: 5}
	results2, err := idx.SearchWithRequest(context.Background(), req2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results2.Hits) != 5 {
		t.Errorf("expected 5 hits on page 2, got %d", len(results2.Hits))
	}

	// Page 2's best score should be <= page 1's worst score
	if len(results.Hits) > 0 && len(results2.Hits) > 0 {
		page1Worst := results.Hits[len(results.Hits)-1].Score
		page2Best := results2.Hits[0].Score
		if page2Best > page1Worst {
			t.Errorf("page 2 best score (%f) should be <= page 1 worst (%f)", page2Best, page1Worst)
		}
	}
}

// TestReverseIndexTargetedDelete verifies that documents indexed with the reverse
// index are cleanly deleted without stale postings.
func TestReverseIndexTargetedDelete(t *testing.T) {
	idx := newTestIndex(t)

	// Index a doc with multiple field types
	idx.Index("target", map[string]interface{}{
		"title":   "alpha bravo charlie",
		"content": "this document should be cleanly deleted",
		"rating":  4.5,
	})

	// Verify it's searchable (use MatchQuery since standard analyzer stems)
	q := query.NewMatchQuery("alpha").SetField("title")
	r, _ := idx.Search(context.Background(), q)
	if r.Total != 1 {
		t.Fatalf("expected 1 result before delete, got %d", r.Total)
	}

	// Delete
	if err := idx.Delete("target"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	// Search should return 0
	r2, _ := idx.Search(context.Background(), q)
	if r2.Total != 0 {
		t.Errorf("expected 0 results after delete, got %d", r2.Total)
	}

	// Numeric range should also return 0
	min := 4.0
	max := 5.0
	nq := query.NewNumericRangeQuery(&min, &max).SetField("rating")
	r3, _ := idx.Search(context.Background(), nq)
	if r3.Total != 0 {
		t.Errorf("expected 0 numeric results after delete, got %d", r3.Total)
	}

	// Doc count should be 0
	count, _ := idx.DocCount()
	if count != 0 {
		t.Errorf("expected 0 docs, got %d", count)
	}
}

// TestDeleteAndReindexCycles verifies no stale data across multiple delete/reindex cycles.
func TestDeleteAndReindexCycles(t *testing.T) {
	// Use keyword field to avoid stemming issues
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("tag", mapping.NewKeywordFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	tags := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	for cycle, tag := range tags {
		err := idx.Index("cycled", map[string]interface{}{
			"tag": tag,
		})
		if err != nil {
			t.Fatalf("Index error on cycle %d: %v", cycle, err)
		}

		// Current tag should match
		q := query.NewTermQuery(tag).SetField("tag")
		r, _ := idx.Search(context.Background(), q)
		if r.Total != 1 {
			t.Errorf("cycle %d: expected 1 result for '%s', got %d", cycle, tag, r.Total)
		}

		// Previous tag should NOT match
		if cycle > 0 {
			old := query.NewTermQuery(tags[cycle-1]).SetField("tag")
			rOld, _ := idx.Search(context.Background(), old)
			if rOld.Total != 0 {
				t.Errorf("cycle %d: expected 0 results for old tag '%s', got %d", cycle, tags[cycle-1], rOld.Total)
			}
		}
	}

	count, _ := idx.DocCount()
	if count != 1 {
		t.Errorf("expected 1 doc after cycles, got %d", count)
	}
}

// TestMatchAllLargeDataset tests matchAll correctly counts >256 docs (beyond internal page boundary).
func TestMatchAllLargeDataset(t *testing.T) {
	idx := newTestIndex(t)

	numDocs := 300
	for i := 0; i < numDocs; i++ {
		idx.Index(fmt.Sprintf("d%04d", i), map[string]interface{}{
			"title": fmt.Sprintf("document %d", i),
		})
	}

	q := query.NewMatchAllQuery()
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if results.Total != uint64(numDocs) {
		t.Errorf("expected %d total, got %d", numDocs, results.Total)
	}

	// Default page size is 10
	if len(results.Hits) != 10 {
		t.Errorf("expected 10 hits (default page), got %d", len(results.Hits))
	}
}

// TestPaginationWithVaryingScores tests pagination correctness when scores differ.
func TestPaginationWithVaryingScores(t *testing.T) {
	idx := newTestIndex(t)

	// Index 50 docs with varying content so BM25 produces different scores
	for i := 0; i < 50; i++ {
		content := fmt.Sprintf("search %d", i)
		// Add extra occurrences to create score variation
		for j := 0; j < i%5; j++ {
			content += " search"
		}
		content += " filler padding words"
		idx.Index(fmt.Sprintf("p%02d", i), map[string]interface{}{
			"content": content,
		})
	}

	q := query.NewMatchQuery("search").SetField("content")

	// Paginate through all results
	collected := make(map[string]bool)
	pageSize := 10
	for from := 0; from < 50; from += pageSize {
		req := &search.SearchRequest{Query: q, Size: pageSize, From: from}
		r, err := idx.SearchWithRequest(context.Background(), req)
		if err != nil {
			t.Fatalf("Search error at from=%d: %v", from, err)
		}
		for _, hit := range r.Hits {
			collected[hit.ID] = true
		}
	}
	if len(collected) != 50 {
		t.Errorf("expected 50 unique docs across pages, got %d", len(collected))
	}
}

// TestKNNBoundedHeap verifies KNN returns correct top-K with more vectors than K.
func TestKNNBoundedHeap(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("vec", mapping.NewVectorFieldMapping(3))
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	// Index 20 vectors with known similarities to [1,0,0]
	for i := 0; i < 20; i++ {
		// Gradually rotate away from [1,0,0]
		angle := float64(i) * 0.05
		x := 1.0 - angle
		y := angle
		if x < 0 {
			x = 0
		}
		idx.Index(fmt.Sprintf("v%02d", i), map[string]interface{}{
			"vec": []float64{x, y, 0},
		})
	}

	// Request K=3
	q := query.NewKNNQuery([]float32{1, 0, 0}, 3).SetField("vec")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if len(results.Hits) > 3 {
		t.Errorf("expected at most 3 results for K=3, got %d", len(results.Hits))
	}

	// v00 should be the top result (most similar to [1,0,0])
	if len(results.Hits) > 0 && results.Hits[0].ID != "v00" {
		t.Errorf("expected v00 as top KNN result, got %s", results.Hits[0].ID)
	}

	// Scores should be descending
	for i := 1; i < len(results.Hits); i++ {
		if results.Hits[i].Score > results.Hits[i-1].Score {
			t.Error("KNN results should be in descending score order")
		}
	}
}

// TestBooleanMustNotOnlyUsesForEachDocID tests the must_not-only path
// which uses ForEachDocID streaming.
func TestBooleanMustNotOnlyUsesForEachDocID(t *testing.T) {
	idx := newTestIndex(t)

	// Index enough docs to exercise streaming
	for i := 0; i < 20; i++ {
		content := "generic content"
		if i%5 == 0 {
			content = "exclude this special content"
		}
		idx.Index(fmt.Sprintf("d%02d", i), map[string]interface{}{
			"content": content,
		})
	}

	// must_not only: exclude docs with "exclude"
	bq := query.NewBooleanQuery()
	bq.AddMustNot(query.NewMatchQuery("exclude").SetField("content"))

	results, err := idx.Search(context.Background(), bq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// 4 out of 20 have "exclude" (i=0,5,10,15)
	if results.Total != 16 {
		t.Errorf("expected 16 results (20-4 excluded), got %d", results.Total)
	}

	// None of the excluded docs should appear
	for _, hit := range results.Hits {
		for _, excluded := range []string{"d00", "d05", "d10", "d15"} {
			if hit.ID == excluded {
				t.Errorf("excluded doc %s appeared in results", hit.ID)
			}
		}
	}
}

// TestConjunctionIntersection verifies the conjunction searcher correctly intersects results.
func TestConjunctionIntersection(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("both", map[string]interface{}{"content": "alpha beta gamma"})
	idx.Index("only_alpha", map[string]interface{}{"content": "alpha delta"})
	idx.Index("only_beta", map[string]interface{}{"content": "beta epsilon"})
	idx.Index("neither", map[string]interface{}{"content": "zeta eta"})

	cq := query.NewConjunctionQuery(
		query.NewTermQuery("alpha").SetField("content"),
		query.NewTermQuery("beta").SetField("content"),
	)

	results, err := idx.Search(context.Background(), cq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total != 1 {
		t.Errorf("expected 1 result (both alpha and beta), got %d", results.Total)
	}
	if len(results.Hits) > 0 && results.Hits[0].ID != "both" {
		t.Errorf("expected 'both', got '%s'", results.Hits[0].ID)
	}
}

// TestMatchQueryBoostNonZero verifies that MatchQuery produces non-zero scores
// (regression test for the Boost=0.0 bug fix).
func TestMatchQueryBoostNonZero(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	// Multi-term match query (triggers DisjunctionQuery internally)
	q := query.NewMatchQuery("quick brown").SetField("title")
	results, err := idx.Search(context.Background(), q)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total == 0 {
		t.Fatal("expected results")
	}

	for _, hit := range results.Hits {
		if hit.Score <= 0 {
			t.Errorf("doc %s has non-positive score %f (boost bug?)", hit.ID, hit.Score)
		}
	}

	// AND operator also produces non-zero scores
	q2 := query.NewMatchQuery("quick brown").SetField("title").SetOperator("and")
	r2, _ := idx.Search(context.Background(), q2)
	for _, hit := range r2.Hits {
		if hit.Score <= 0 {
			t.Errorf("AND doc %s has non-positive score %f", hit.ID, hit.Score)
		}
	}
}

// TestSinglePassFaceting verifies facets are computed correctly alongside search results.
func TestSinglePassFaceting(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentMapping()
	dm.AddFieldMapping("category", mapping.NewKeywordFieldMapping())
	m.DefaultMapping = dm
	idx := newTestIndexWithMapping(t, m)

	// Index docs with varying categories
	categories := []string{"tech", "science", "tech", "art", "science", "tech"}
	for i, cat := range categories {
		idx.Index(fmt.Sprintf("f%d", i), map[string]interface{}{
			"title":    fmt.Sprintf("Document about %s number %d", cat, i),
			"category": cat,
		})
	}

	q := query.NewMatchAllQuery()
	req := &search.SearchRequest{
		Query: q,
		Size:  10,
		Facets: map[string]*search.FacetRequest{
			"categories": {Field: "category", Size: 10},
		},
	}

	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if results.Total != uint64(len(categories)) {
		t.Errorf("expected %d total, got %d", len(categories), results.Total)
	}

	catFacet, ok := results.Facets["categories"]
	if !ok {
		t.Fatal("expected 'categories' facet")
	}

	// Check facet counts
	termCounts := make(map[string]int)
	for _, tf := range catFacet.Terms {
		termCounts[tf.Term] = tf.Count
	}

	if termCounts["tech"] != 3 {
		t.Errorf("expected tech=3, got %d", termCounts["tech"])
	}
	if termCounts["science"] != 2 {
		t.Errorf("expected science=2, got %d", termCounts["science"])
	}
	if termCounts["art"] != 1 {
		t.Errorf("expected art=1, got %d", termCounts["art"])
	}
}

// TestBooleanMustWithMustNot verifies must + must_not combination.
func TestBooleanMustWithMustNot(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("a", map[string]interface{}{"content": "search engine fast"})
	idx.Index("b", map[string]interface{}{"content": "search engine slow"})
	idx.Index("c", map[string]interface{}{"content": "database engine fast"})

	bq := query.NewBooleanQuery()
	bq.AddMust(query.NewMatchQuery("engine").SetField("content"))
	bq.AddMustNot(query.NewMatchQuery("slow").SetField("content"))

	results, err := idx.Search(context.Background(), bq)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// "b" has "slow" so should be excluded
	if results.Total != 2 {
		t.Errorf("expected 2 results, got %d", results.Total)
	}
	for _, hit := range results.Hits {
		if hit.ID == "b" {
			t.Error("doc 'b' should be excluded by must_not")
		}
	}
}

// TestMaxScoreCorrectness verifies MaxScore is the highest score in the full result set.
func TestMaxScoreCorrectness(t *testing.T) {
	idx := newTestIndex(t)

	for i := 0; i < 30; i++ {
		content := "common"
		if i == 7 {
			content = "common common common rare unique special"
		}
		idx.Index(fmt.Sprintf("ms%02d", i), map[string]interface{}{
			"content": content,
		})
	}

	q := query.NewTermQuery("common").SetField("content")
	req := &search.SearchRequest{Query: q, Size: 3, From: 0}
	results, _ := idx.SearchWithRequest(context.Background(), req)

	// MaxScore should be >= score of the top hit
	if len(results.Hits) > 0 && results.MaxScore < results.Hits[0].Score {
		t.Errorf("MaxScore (%f) should be >= top hit score (%f)", results.MaxScore, results.Hits[0].Score)
	}

	// MaxScore should be positive
	if results.MaxScore <= 0 {
		t.Error("expected positive MaxScore")
	}
}

// TestDeleteWithVectorField verifies that deleting a document with vector fields
// cleans up vector data via the reverse index.
func TestDeleteWithVectorField(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(3))
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("vec1", map[string]interface{}{
		"title":     "vector doc",
		"embedding": []float64{1.0, 0.0, 0.0},
	})
	idx.Index("vec2", map[string]interface{}{
		"title":     "another doc",
		"embedding": []float64{0.0, 1.0, 0.0},
	})

	// Delete vec1
	idx.Delete("vec1")

	// KNN search should only return vec2
	q := query.NewKNNQuery([]float32{1.0, 0.0, 0.0}, 10).SetField("embedding")
	results, _ := idx.Search(context.Background(), q)

	for _, hit := range results.Hits {
		if hit.ID == "vec1" {
			t.Error("deleted vector doc 'vec1' should not appear in KNN results")
		}
	}
}

// TestDeleteWithBooleanField verifies cleanup of boolean field data.
func TestDeleteWithBooleanField(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentMapping()
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("active", mapping.NewBooleanFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("b1", map[string]interface{}{"title": "active doc", "active": true})
	idx.Index("b2", map[string]interface{}{"title": "inactive doc", "active": false})

	// Delete b1
	idx.Delete("b1")

	// Search for active=true should return 0
	q := query.NewTermQuery("T").SetField("active")
	r, _ := idx.Search(context.Background(), q)
	if r.Total != 0 {
		t.Errorf("expected 0 results for deleted boolean doc, got %d", r.Total)
	}

	// b2 should still be searchable
	q2 := query.NewTermQuery("F").SetField("active")
	r2, _ := idx.Search(context.Background(), q2)
	if r2.Total != 1 {
		t.Errorf("expected 1 result for remaining boolean doc, got %d", r2.Total)
	}
}

// TestDeleteWithDateTimeField verifies cleanup of datetime field data.
func TestDeleteWithDateTimeField(t *testing.T) {
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentMapping()
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("created", mapping.NewDateTimeFieldMapping())
	m.DefaultMapping = dm

	idx := newTestIndexWithMapping(t, m)

	idx.Index("dt1", map[string]interface{}{"title": "old", "created": "2024-01-01T00:00:00Z"})
	idx.Index("dt2", map[string]interface{}{"title": "new", "created": "2025-06-01T00:00:00Z"})

	// Delete dt1
	idx.Delete("dt1")

	// Date range search should only return dt2
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q := query.NewDateRangeQuery(&start, &end).SetField("created")
	r, _ := idx.Search(context.Background(), q)
	if r.Total != 1 {
		t.Errorf("expected 1 result after delete, got %d", r.Total)
	}
	if len(r.Hits) > 0 && r.Hits[0].ID != "dt2" {
		t.Errorf("expected dt2, got %s", r.Hits[0].ID)
	}
}

// TestFacetsWithFilteredQuery verifies facets work correctly with non-matchAll queries.
func TestFacetsWithFilteredQuery(t *testing.T) {
	idx := newTestIndex(t)

	idx.Index("f1", map[string]interface{}{"content": "search engine", "category": "tech"})
	idx.Index("f2", map[string]interface{}{"content": "search algorithm", "category": "science"})
	idx.Index("f3", map[string]interface{}{"content": "art gallery", "category": "art"})

	q := query.NewTermQuery("search").SetField("content")
	req := &search.SearchRequest{
		Query: q,
		Size:  10,
		Facets: map[string]*search.FacetRequest{
			"cats": {Field: "category", Size: 10},
		},
	}

	results, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	// Only f1 and f2 match "search"
	if results.Total != 2 {
		t.Errorf("expected 2 results, got %d", results.Total)
	}

	catFacet := results.Facets["cats"]
	if catFacet == nil {
		t.Fatal("expected facet results")
	}

	// Facets should only count the 2 matched documents
	if catFacet.Total != 2 {
		t.Errorf("expected facet total 2, got %d", catFacet.Total)
	}
}

func TestConcurrentIndexAndSearchWithDocCount(t *testing.T) {
	store := newMemStore()
	idx, err := New(store, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Index some initial docs
	for i := 0; i < 10; i++ {
		err := idx.Index(fmt.Sprintf("doc%d", i), map[string]interface{}{
			"title":   fmt.Sprintf("Document %d about testing", i),
			"content": "This is test content for concurrent access",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent writers
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				id := fmt.Sprintf("worker%d-doc%d", worker, i)
				err := idx.Index(id, map[string]interface{}{
					"title":   fmt.Sprintf("Worker %d document %d", worker, i),
					"content": "concurrent indexing test content",
				})
				if err != nil {
					errors <- fmt.Errorf("index error: %w", err)
					return
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
				q := query.NewMatchQuery("testing").SetField("title")
				_, err := idx.Search(ctx, q)
				if err != nil {
					errors <- fmt.Errorf("search error: %w", err)
					return
				}
			}
		}()
	}

	// Concurrent doc count reads
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_, err := idx.DocCount()
				if err != nil {
					errors <- fmt.Errorf("doc count error: %w", err)
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestConcurrentBatchAndSearch(t *testing.T) {
	store := newMemStore()
	idx, err := New(store, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent batch writers
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for b := 0; b < 5; b++ {
				batch := idx.NewBatch()
				for i := 0; i < 10; i++ {
					id := fmt.Sprintf("batch-w%d-b%d-d%d", worker, b, i)
					batch.Index(id, map[string]interface{}{
						"title": fmt.Sprintf("Batch document %d", i),
					})
				}
				if err := batch.Execute(); err != nil {
					errors <- fmt.Errorf("batch error: %w", err)
					return
				}
			}
		}(w)
	}

	// Concurrent readers
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				q := query.NewMatchAllQuery()
				_, err := idx.Search(ctx, q)
				if err != nil {
					errors <- fmt.Errorf("search error: %w", err)
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestConcurrentIndexAndDelete(t *testing.T) {
	store := newMemStore()
	idx, err := New(store, nil)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Writer that indexes then deletes
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				id := fmt.Sprintf("cd-w%d-d%d", worker, i)
				err := idx.Index(id, map[string]interface{}{
					"title": fmt.Sprintf("Document to delete %d", i),
				})
				if err != nil {
					errors <- fmt.Errorf("index error: %w", err)
					return
				}
				err = idx.Delete(id)
				if err != nil {
					errors <- fmt.Errorf("delete error: %w", err)
					return
				}
			}
		}(w)
	}

	// Concurrent document lookups
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				// Don't error on not found since docs are being deleted concurrently
				idx.Document(fmt.Sprintf("cd-w0-d%d", i%20))
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestDocIDValidation(t *testing.T) {
	store := newMemStore()
	idx, err := New(store, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Empty ID
	err = idx.Index("", map[string]interface{}{"title": "test"})
	if err == nil {
		t.Error("expected error for empty document ID")
	}

	// ID with slash
	err = idx.Index("doc/with/slash", map[string]interface{}{"title": "test"})
	if err == nil {
		t.Error("expected error for document ID containing '/'")
	}

	// Valid ID should work
	err = idx.Index("valid-doc-id", map[string]interface{}{"title": "test"})
	if err != nil {
		t.Errorf("unexpected error for valid document ID: %v", err)
	}

	// Delete with slash
	err = idx.Delete("doc/with/slash")
	if err == nil {
		t.Error("expected error for delete with '/' in ID")
	}

	// Document with slash
	_, err = idx.Document("doc/with/slash")
	if err == nil {
		t.Error("expected error for document lookup with '/' in ID")
	}
}

func TestSearchPaginationLimits(t *testing.T) {
	idx := newTestIndex(t)
	indexTestDocuments(t, idx)

	q := query.NewMatchAllQuery()

	// Test: requesting size > MaxSearchSize gets capped
	t.Run("size capped to MaxSearchSize", func(t *testing.T) {
		req := &search.SearchRequest{
			Query: q,
			Size:  MaxSearchSize + 100,
			From:  0,
		}
		results, err := idx.SearchWithRequest(context.Background(), req)
		if err != nil {
			t.Fatalf("Search error: %v", err)
		}
		// With only 5 docs indexed, we can't directly observe the cap,
		// but the search must succeed without error (no out-of-bounds).
		if results.Total != 5 {
			t.Errorf("expected 5 total hits, got %d", results.Total)
		}
	})

	// Test: requesting from > MaxSearchFrom gets capped
	t.Run("from capped to MaxSearchFrom", func(t *testing.T) {
		req := &search.SearchRequest{
			Query: q,
			Size:  10,
			From:  MaxSearchFrom + 500,
		}
		results, err := idx.SearchWithRequest(context.Background(), req)
		if err != nil {
			t.Fatalf("Search error: %v", err)
		}
		// With from beyond all docs, we expect zero hits returned.
		if len(results.Hits) != 0 {
			t.Errorf("expected 0 hits with large from offset, got %d", len(results.Hits))
		}
		// Total should still reflect the actual match count.
		if results.Total != 5 {
			t.Errorf("expected 5 total hits, got %d", results.Total)
		}
	})

	// Test: negative from gets capped to 0
	t.Run("negative from capped to zero", func(t *testing.T) {
		req := &search.SearchRequest{
			Query: q,
			Size:  10,
			From:  -5,
		}
		results, err := idx.SearchWithRequest(context.Background(), req)
		if err != nil {
			t.Fatalf("Search error: %v", err)
		}
		if results.Total != 5 {
			t.Errorf("expected 5 total hits, got %d", results.Total)
		}
		if len(results.Hits) != 5 {
			t.Errorf("expected 5 hits (from=0, size=10), got %d", len(results.Hits))
		}
	})
}

func TestValidateID_EdgeCases(t *testing.T) {
	idx := newTestIndex(t)

	// Test: ID with null byte should be rejected
	t.Run("control character null", func(t *testing.T) {
		err := idx.Index("doc\x00bad", map[string]interface{}{"title": "test"})
		if err == nil {
			t.Error("expected error for ID containing null byte")
		}
	})

	// Test: ID with newline should be rejected
	t.Run("control character newline", func(t *testing.T) {
		err := idx.Index("doc\nbad", map[string]interface{}{"title": "test"})
		if err == nil {
			t.Error("expected error for ID containing newline")
		}
	})

	// Test: ID with tab should be rejected
	t.Run("control character tab", func(t *testing.T) {
		err := idx.Index("doc\tbad", map[string]interface{}{"title": "test"})
		if err == nil {
			t.Error("expected error for ID containing tab")
		}
	})

	// Test: ID longer than 512 bytes should be rejected
	t.Run("ID exceeds max length", func(t *testing.T) {
		longID := strings.Repeat("a", 513)
		err := idx.Index(longID, map[string]interface{}{"title": "test"})
		if err == nil {
			t.Error("expected error for ID exceeding 512 bytes")
		}
	})

	// Test: ID at exactly 512 bytes should be accepted
	t.Run("ID at max length", func(t *testing.T) {
		maxID := strings.Repeat("b", 512)
		err := idx.Index(maxID, map[string]interface{}{"title": "test"})
		if err != nil {
			t.Errorf("unexpected error for 512-byte ID: %v", err)
		}
	})

	// Test: normal IDs should still work
	t.Run("normal ID", func(t *testing.T) {
		err := idx.Index("normal-doc-123", map[string]interface{}{"title": "test"})
		if err != nil {
			t.Errorf("unexpected error for normal ID: %v", err)
		}
	})
}
