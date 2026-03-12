package searchmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/readmedotmd/search.md/search/query"
)

// newBenchIndex creates a SearchIndex backed by a memStore and populates it
// with n documents containing text, numeric, and boolean fields.
func newBenchIndex(b *testing.B, n int) *SearchIndex {
	b.Helper()
	store := newMemStore()
	idx, err := New(store, nil)
	if err != nil {
		b.Fatalf("failed to create index: %v", err)
	}
	for i := 0; i < n; i++ {
		doc := map[string]interface{}{
			"title":   fmt.Sprintf("document number %d about search engines", i),
			"content": fmt.Sprintf("the quick brown fox jumps over the lazy dog in document %d with extra words for variety", i),
			"rating":  float64(i%5) + 0.5,
			"active":  i%2 == 0,
		}
		if err := idx.Index(fmt.Sprintf("doc%d", i), doc); err != nil {
			b.Fatalf("failed to index doc%d: %v", i, err)
		}
	}
	return idx
}

func BenchmarkIndex(b *testing.B) {
	store := newMemStore()
	idx, err := New(store, nil)
	if err != nil {
		b.Fatalf("failed to create index: %v", err)
	}
	doc := map[string]interface{}{
		"title":   "benchmark document title for indexing",
		"content": "the quick brown fox jumps over the lazy dog and this is a longer text for realistic indexing benchmarks",
		"rating":  4.5,
		"active":  true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("bench%d", i)
		if err := idx.Index(id, doc); err != nil {
			b.Fatalf("index error: %v", err)
		}
	}
}

func BenchmarkSearch_Term(b *testing.B) {
	idx := newBenchIndex(b, 100)
	ctx := context.Background()
	q := query.NewTermQuery("fox").SetField("content")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatalf("search error: %v", err)
		}
	}
}

func BenchmarkSearch_Phrase(b *testing.B) {
	idx := newBenchIndex(b, 100)
	ctx := context.Background()
	q := query.NewMatchPhraseQuery("quick brown fox").SetField("content")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatalf("search error: %v", err)
		}
	}
}

func BenchmarkSearch_Boolean(b *testing.B) {
	idx := newBenchIndex(b, 100)
	ctx := context.Background()
	bq := query.NewBooleanQuery()
	bq.AddMust(query.NewTermQuery("fox").SetField("content"))
	bq.AddMustNot(query.NewTermQuery("document").SetField("title"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, bq); err != nil {
			b.Fatalf("search error: %v", err)
		}
	}
}

func BenchmarkSearch_Prefix(b *testing.B) {
	idx := newBenchIndex(b, 100)
	ctx := context.Background()
	q := query.NewPrefixQuery("doc").SetField("content")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatalf("search error: %v", err)
		}
	}
}

func BenchmarkSearch_Fuzzy(b *testing.B) {
	idx := newBenchIndex(b, 100)
	ctx := context.Background()
	q := query.NewFuzzyQuery("foz").SetField("content").SetFuzziness(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatalf("search error: %v", err)
		}
	}
}
