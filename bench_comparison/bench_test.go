package bench_comparison

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/blevesearch/bleve/v2"
	searchmd "github.com/readmedotmd/search.md"
	"github.com/readmedotmd/search.md/search/query"
	"github.com/readmedotmd/store.md/backend/memory"
)

// ─────────────────────────────────────────────────────────
// Test corpus — realistic text content
// ─────────────────────────────────────────────────────────

var corpus = []string{
	"The quick brown fox jumps over the lazy dog near the river bank",
	"A fast red car drives through the narrow mountain pass at dawn",
	"Cloud computing enables scalable infrastructure for modern applications",
	"Machine learning algorithms improve prediction accuracy over time",
	"Distributed systems require careful consideration of network partitions",
	"The database query optimizer selects the most efficient execution plan",
	"Containerized microservices communicate through message queues and APIs",
	"Full-text search engines index documents for fast retrieval of results",
	"The operating system kernel manages hardware resources and process scheduling",
	"Cryptographic hash functions provide data integrity verification mechanisms",
	"Natural language processing transforms unstructured text into structured data",
	"Graph databases excel at traversing complex relationships between entities",
	"Event-driven architectures decouple producers from consumers of messages",
	"Load balancers distribute incoming traffic across multiple backend servers",
	"Continuous integration pipelines automatically build test and deploy code",
	"Memory management in garbage collected languages reduces manual allocation",
	"Binary search trees provide logarithmic time complexity for lookups",
	"Version control systems track changes to source code over time",
	"API gateway patterns aggregate multiple microservice calls into one response",
	"Consensus algorithms ensure agreement in distributed fault tolerant systems",
}

func generateDoc(i int) map[string]interface{} {
	return map[string]interface{}{
		"title":   fmt.Sprintf("Document %d: %s", i, corpus[i%len(corpus)][:30]),
		"content": corpus[i%len(corpus)],
		"body":    fmt.Sprintf("%s Additional context for document number %d with more words to increase document length and test realistic scenarios.", corpus[(i+7)%len(corpus)], i),
		"rating":  float64(i%5) + 1.0,
		"active":  i%2 == 0,
	}
}

func newMemStore() *memory.StoreMemory {
	return memory.New()
}

// ─────────────────────────────────────────────────────────
// Index creation helpers
// ─────────────────────────────────────────────────────────

func newSearchMDIndex(b *testing.B, n int) *searchmd.SearchIndex {
	b.Helper()
	store := newMemStore()
	idx, err := searchmd.New(store, nil)
	if err != nil {
		b.Fatalf("search.md New failed: %v", err)
	}
	for i := 0; i < n; i++ {
		doc := generateDoc(i)
		if err := idx.Index(fmt.Sprintf("doc%d", i), doc); err != nil {
			b.Fatalf("search.md index doc%d: %v", i, err)
		}
	}
	return idx
}

func newBleveIndex(b *testing.B, n int) bleve.Index {
	b.Helper()
	dir, err := os.MkdirTemp("", "bleve-bench-*")
	if err != nil {
		b.Fatalf("tmpdir: %v", err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })

	mapping := bleve.NewIndexMapping()
	idx, err := bleve.New(dir, mapping)
	if err != nil {
		b.Fatalf("bleve New: %v", err)
	}
	b.Cleanup(func() { idx.Close() })

	for i := 0; i < n; i++ {
		doc := generateDoc(i)
		if err := idx.Index(fmt.Sprintf("doc%d", i), doc); err != nil {
			b.Fatalf("bleve index doc%d: %v", i, err)
		}
	}
	return idx
}

// ─────────────────────────────────────────────────────────
// INDEXING BENCHMARKS
// ─────────────────────────────────────────────────────────

func BenchmarkIndex_SearchMD_100(b *testing.B)   { benchIndexSearchMD(b, 100) }
func BenchmarkIndex_SearchMD_1000(b *testing.B)  { benchIndexSearchMD(b, 1000) }
func BenchmarkIndex_SearchMD_10000(b *testing.B) { benchIndexSearchMD(b, 10000) }

func BenchmarkIndex_Bleve_100(b *testing.B)   { benchIndexBleve(b, 100) }
func BenchmarkIndex_Bleve_1000(b *testing.B)  { benchIndexBleve(b, 1000) }
func BenchmarkIndex_Bleve_10000(b *testing.B) { benchIndexBleve(b, 10000) }

func benchIndexSearchMD(b *testing.B, n int) {
	for i := 0; i < b.N; i++ {
		store := newMemStore()
		idx, err := searchmd.New(store, nil)
		if err != nil {
			b.Fatal(err)
		}
		for j := 0; j < n; j++ {
			doc := generateDoc(j)
			if err := idx.Index(fmt.Sprintf("doc%d", j), doc); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func benchIndexBleve(b *testing.B, n int) {
	for i := 0; i < b.N; i++ {
		dir, err := os.MkdirTemp("", "bleve-idx-*")
		if err != nil {
			b.Fatal(err)
		}
		idx, err := bleve.New(dir, bleve.NewIndexMapping())
		if err != nil {
			os.RemoveAll(dir)
			b.Fatal(err)
		}
		for j := 0; j < n; j++ {
			doc := generateDoc(j)
			if err := idx.Index(fmt.Sprintf("doc%d", j), doc); err != nil {
				idx.Close()
				os.RemoveAll(dir)
				b.Fatal(err)
			}
		}
		idx.Close()
		os.RemoveAll(dir)
	}
}

// ─────────────────────────────────────────────────────────
// SINGLE DOCUMENT INDEXING (throughput)
// ─────────────────────────────────────────────────────────

func BenchmarkIndexSingle_SearchMD(b *testing.B) {
	store := newMemStore()
	idx, err := searchmd.New(store, nil)
	if err != nil {
		b.Fatal(err)
	}
	doc := generateDoc(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := idx.Index(fmt.Sprintf("doc%d", i), doc); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIndexSingle_Bleve(b *testing.B) {
	dir, err := os.MkdirTemp("", "bleve-single-*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)
	idx, err := bleve.New(dir, bleve.NewIndexMapping())
	if err != nil {
		b.Fatal(err)
	}
	defer idx.Close()
	doc := generateDoc(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := idx.Index(fmt.Sprintf("doc%d", i), doc); err != nil {
			b.Fatal(err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// BATCH INDEXING
// ─────────────────────────────────────────────────────────

func BenchmarkBatchIndex_SearchMD_1000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		store := newMemStore()
		idx, err := searchmd.New(store, nil)
		if err != nil {
			b.Fatal(err)
		}
		batch := idx.NewBatch()
		for j := 0; j < 1000; j++ {
			batch.Index(fmt.Sprintf("doc%d", j), generateDoc(j))
		}
		if err := batch.Execute(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBatchIndex_Bleve_1000(b *testing.B) {
	for i := 0; i < b.N; i++ {
		dir, err := os.MkdirTemp("", "bleve-batch-*")
		if err != nil {
			b.Fatal(err)
		}
		idx, err := bleve.New(dir, bleve.NewIndexMapping())
		if err != nil {
			os.RemoveAll(dir)
			b.Fatal(err)
		}
		batch := idx.NewBatch()
		for j := 0; j < 1000; j++ {
			if err := batch.Index(fmt.Sprintf("doc%d", j), generateDoc(j)); err != nil {
				idx.Close()
				os.RemoveAll(dir)
				b.Fatal(err)
			}
		}
		if err := idx.Batch(batch); err != nil {
			idx.Close()
			os.RemoveAll(dir)
			b.Fatal(err)
		}
		idx.Close()
		os.RemoveAll(dir)
	}
}

// ─────────────────────────────────────────────────────────
// TERM QUERY BENCHMARKS
// ─────────────────────────────────────────────────────────

func BenchmarkTermQuery_SearchMD_100(b *testing.B)   { benchTermSearchMD(b, 100) }
func BenchmarkTermQuery_SearchMD_1000(b *testing.B)  { benchTermSearchMD(b, 1000) }
func BenchmarkTermQuery_SearchMD_10000(b *testing.B) { benchTermSearchMD(b, 10000) }

func BenchmarkTermQuery_Bleve_100(b *testing.B)   { benchTermBleve(b, 100) }
func BenchmarkTermQuery_Bleve_1000(b *testing.B)  { benchTermBleve(b, 1000) }
func BenchmarkTermQuery_Bleve_10000(b *testing.B) { benchTermBleve(b, 10000) }

func benchTermSearchMD(b *testing.B, n int) {
	idx := newSearchMDIndex(b, n)
	ctx := context.Background()
	q := query.NewTermQuery("fox").SetField("content")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

func benchTermBleve(b *testing.B, n int) {
	idx := newBleveIndex(b, n)
	q := bleve.NewTermQuery("fox")
	q.SetField("content")
	req := bleve.NewSearchRequest(q)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(req); err != nil {
			b.Fatal(err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// MATCH QUERY BENCHMARKS (analyzed text)
// ─────────────────────────────────────────────────────────

func BenchmarkMatchQuery_SearchMD_1000(b *testing.B) {
	idx := newSearchMDIndex(b, 1000)
	ctx := context.Background()
	q := query.NewMatchQuery("scalable infrastructure").SetField("content")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMatchQuery_Bleve_1000(b *testing.B) {
	idx := newBleveIndex(b, 1000)
	q := bleve.NewMatchQuery("scalable infrastructure")
	q.SetField("content")
	req := bleve.NewSearchRequest(q)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(req); err != nil {
			b.Fatal(err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// PHRASE QUERY BENCHMARKS
// ─────────────────────────────────────────────────────────

func BenchmarkPhraseQuery_SearchMD_1000(b *testing.B) {
	idx := newSearchMDIndex(b, 1000)
	ctx := context.Background()
	q := query.NewMatchPhraseQuery("quick brown fox").SetField("content")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPhraseQuery_Bleve_1000(b *testing.B) {
	idx := newBleveIndex(b, 1000)
	q := bleve.NewMatchPhraseQuery("quick brown fox")
	q.SetField("content")
	req := bleve.NewSearchRequest(q)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(req); err != nil {
			b.Fatal(err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// PREFIX QUERY BENCHMARKS
// ─────────────────────────────────────────────────────────

func BenchmarkPrefixQuery_SearchMD_1000(b *testing.B) {
	idx := newSearchMDIndex(b, 1000)
	ctx := context.Background()
	q := query.NewPrefixQuery("comp").SetField("content")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrefixQuery_Bleve_1000(b *testing.B) {
	idx := newBleveIndex(b, 1000)
	q := bleve.NewPrefixQuery("comp")
	q.SetField("content")
	req := bleve.NewSearchRequest(q)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(req); err != nil {
			b.Fatal(err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// FUZZY QUERY BENCHMARKS
// ─────────────────────────────────────────────────────────

func BenchmarkFuzzyQuery_SearchMD_1000(b *testing.B) {
	idx := newSearchMDIndex(b, 1000)
	ctx := context.Background()
	q := query.NewFuzzyQuery("computng").SetField("content").SetFuzziness(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFuzzyQuery_Bleve_1000(b *testing.B) {
	idx := newBleveIndex(b, 1000)
	q := bleve.NewFuzzyQuery("computng")
	q.SetField("content")
	q.SetFuzziness(1)
	req := bleve.NewSearchRequest(q)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(req); err != nil {
			b.Fatal(err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// BOOLEAN/COMPOUND QUERY BENCHMARKS
// ─────────────────────────────────────────────────────────

func BenchmarkBooleanQuery_SearchMD_1000(b *testing.B) {
	idx := newSearchMDIndex(b, 1000)
	ctx := context.Background()
	bq := query.NewBooleanQuery()
	bq.AddMust(query.NewTermQuery("systems").SetField("content"))
	bq.AddMustNot(query.NewTermQuery("database").SetField("content"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, bq); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBooleanQuery_Bleve_1000(b *testing.B) {
	idx := newBleveIndex(b, 1000)
	must := bleve.NewTermQuery("systems")
	must.SetField("content")
	mustNot := bleve.NewTermQuery("database")
	mustNot.SetField("content")
	bq := bleve.NewBooleanQuery()
	bq.AddMust(must)
	bq.AddMustNot(mustNot)
	req := bleve.NewSearchRequest(bq)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(req); err != nil {
			b.Fatal(err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// LARGE INDEX SEARCH (10k docs)
// ─────────────────────────────────────────────────────────

func BenchmarkMatchLargeIndex_SearchMD_10000(b *testing.B) {
	idx := newSearchMDIndex(b, 10000)
	ctx := context.Background()
	q := query.NewMatchQuery("distributed systems").SetField("content")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMatchLargeIndex_Bleve_10000(b *testing.B) {
	idx := newBleveIndex(b, 10000)
	q := bleve.NewMatchQuery("distributed systems")
	q.SetField("content")
	req := bleve.NewSearchRequest(q)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(req); err != nil {
			b.Fatal(err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// WILDCARD QUERY BENCHMARKS
// ─────────────────────────────────────────────────────────

func BenchmarkWildcardQuery_SearchMD_1000(b *testing.B) {
	idx := newSearchMDIndex(b, 1000)
	ctx := context.Background()
	q := query.NewWildcardQuery("sys*").SetField("content")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWildcardQuery_Bleve_1000(b *testing.B) {
	idx := newBleveIndex(b, 1000)
	q := bleve.NewWildcardQuery("sys*")
	q.SetField("content")
	req := bleve.NewSearchRequest(q)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(req); err != nil {
			b.Fatal(err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// CONCURRENT SEARCH BENCHMARKS
// ─────────────────────────────────────────────────────────

func BenchmarkConcurrentSearch_SearchMD_1000(b *testing.B) {
	idx := newSearchMDIndex(b, 1000)
	ctx := context.Background()
	terms := []string{"fox", "systems", "search", "code", "data"}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			q := query.NewTermQuery(terms[i%len(terms)]).SetField("content")
			if _, err := idx.Search(ctx, q); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkConcurrentSearch_Bleve_1000(b *testing.B) {
	idx := newBleveIndex(b, 1000)
	terms := []string{"fox", "systems", "search", "code", "data"}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			q := bleve.NewTermQuery(terms[i%len(terms)])
			q.SetField("content")
			req := bleve.NewSearchRequest(q)
			if _, err := idx.Search(req); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// ─────────────────────────────────────────────────────────
// MEMORY FOOTPRINT TEST (not a benchmark, but useful)
// ─────────────────────────────────────────────────────────

func BenchmarkMemoryFootprint_SearchMD_1000(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		store := newMemStore()
		idx, err := searchmd.New(store, nil)
		if err != nil {
			b.Fatal(err)
		}
		for j := 0; j < 1000; j++ {
			if err := idx.Index(fmt.Sprintf("doc%d", j), generateDoc(j)); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkMemoryFootprint_Bleve_1000(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		dir, err := os.MkdirTemp("", "bleve-mem-*")
		if err != nil {
			b.Fatal(err)
		}
		idx, err := bleve.New(dir, bleve.NewIndexMapping())
		if err != nil {
			os.RemoveAll(dir)
			b.Fatal(err)
		}
		for j := 0; j < 1000; j++ {
			if err := idx.Index(fmt.Sprintf("doc%d", j), generateDoc(j)); err != nil {
				idx.Close()
				os.RemoveAll(dir)
				b.Fatal(err)
			}
		}
		idx.Close()
		os.RemoveAll(dir)
	}
}

// ─────────────────────────────────────────────────────────
// DELETION BENCHMARKS
// ─────────────────────────────────────────────────────────

func BenchmarkDelete_SearchMD(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		idx := newSearchMDIndex(b, 500)
		// shuffle delete order
		order := rand.Perm(500)
		b.StartTimer()
		for _, j := range order {
			if err := idx.Delete(fmt.Sprintf("doc%d", j)); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkDelete_Bleve(b *testing.B) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		idx := newBleveIndex(b, 500)
		order := rand.Perm(500)
		b.StartTimer()
		for _, j := range order {
			if err := idx.Delete(fmt.Sprintf("doc%d", j)); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// ─────────────────────────────────────────────────────────
// MULTI-FIELD MATCH
// ─────────────────────────────────────────────────────────

func BenchmarkMultiFieldMatch_SearchMD_1000(b *testing.B) {
	idx := newSearchMDIndex(b, 1000)
	ctx := context.Background()
	bq := query.NewBooleanQuery()
	bq.AddShould(query.NewMatchQuery("systems").SetField("content"))
	bq.AddShould(query.NewMatchQuery("Document").SetField("title"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(ctx, bq); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMultiFieldMatch_Bleve_1000(b *testing.B) {
	idx := newBleveIndex(b, 1000)
	q1 := bleve.NewMatchQuery("systems")
	q1.SetField("content")
	q2 := bleve.NewMatchQuery("Document")
	q2.SetField("title")
	dq := bleve.NewDisjunctionQuery(q1, q2)
	req := bleve.NewSearchRequest(dq)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Search(req); err != nil {
			b.Fatal(err)
		}
	}
}
