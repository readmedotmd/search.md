package bench_comparison

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"

	searchmd "github.com/readmedotmd/search.md"
	"github.com/readmedotmd/search.md/mapping"
	"github.com/readmedotmd/search.md/search"
	"github.com/readmedotmd/search.md/search/query"
)

// ─────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────

func randomVector(dims int, rng *rand.Rand) []float64 {
	v := make([]float64, dims)
	for i := range v {
		v[i] = rng.Float64()*2 - 1
	}
	return v
}

func randomVector32(dims int, rng *rand.Rand) []float32 {
	v := make([]float32, dims)
	for i := range v {
		v[i] = float32(rng.Float64()*2 - 1)
	}
	return v
}

// normalizeF64 normalizes a float64 vector in place.
func normalizeF64(v []float64) {
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range v {
			v[i] /= norm
		}
	}
}

func setupVectorIndex(t testing.TB, dims, numDocs int) *searchmd.SearchIndex {
	t.Helper()
	store := newMemStore()
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(dims))
	m.DefaultMapping = dm

	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < numDocs; i++ {
		vec := randomVector(dims, rng)
		normalizeF64(vec)
		doc := map[string]interface{}{
			"title":     fmt.Sprintf("Document %d", i),
			"embedding": vec,
		}
		if err := idx.Index(fmt.Sprintf("doc%d", i), doc); err != nil {
			t.Fatalf("index doc%d: %v", i, err)
		}
	}
	return idx
}

// ─────────────────────────────────────────────────────────
// BENCHMARK: Vector Indexing
// ─────────────────────────────────────────────────────────

func BenchmarkVector_Index_128d_100docs(b *testing.B) {
	benchVectorIndex(b, 128, 100)
}

func BenchmarkVector_Index_128d_1000docs(b *testing.B) {
	benchVectorIndex(b, 128, 1000)
}

func BenchmarkVector_Index_768d_100docs(b *testing.B) {
	benchVectorIndex(b, 768, 100)
}

func BenchmarkVector_Index_768d_1000docs(b *testing.B) {
	benchVectorIndex(b, 768, 1000)
}

func benchVectorIndex(b *testing.B, dims, numDocs int) {
	rng := rand.New(rand.NewSource(42))
	docs := make([]map[string]interface{}, numDocs)
	for i := 0; i < numDocs; i++ {
		vec := randomVector(dims, rng)
		normalizeF64(vec)
		docs[i] = map[string]interface{}{
			"title":     fmt.Sprintf("Document %d", i),
			"embedding": vec,
		}
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		store := newMemStore()
		m := mapping.NewIndexMapping()
		dm := mapping.NewDocumentStaticMapping()
		dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
		dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(dims))
		m.DefaultMapping = dm

		idx, _ := searchmd.New(store, m)
		for i, doc := range docs {
			idx.Index(fmt.Sprintf("doc%d", i), doc)
		}
	}
}

// ─────────────────────────────────────────────────────────
// BENCHMARK: KNN Search at various scales
// ─────────────────────────────────────────────────────────

func BenchmarkVector_KNN_128d_100docs_k10(b *testing.B) {
	benchVectorKNN(b, 128, 100, 10)
}

func BenchmarkVector_KNN_128d_1000docs_k10(b *testing.B) {
	benchVectorKNN(b, 128, 1000, 10)
}

func BenchmarkVector_KNN_128d_5000docs_k10(b *testing.B) {
	benchVectorKNN(b, 128, 5000, 10)
}

func BenchmarkVector_KNN_768d_100docs_k10(b *testing.B) {
	benchVectorKNN(b, 768, 100, 10)
}

func BenchmarkVector_KNN_768d_1000docs_k10(b *testing.B) {
	benchVectorKNN(b, 768, 1000, 10)
}

func BenchmarkVector_KNN_768d_5000docs_k10(b *testing.B) {
	benchVectorKNN(b, 768, 5000, 10)
}

func BenchmarkVector_KNN_128d_1000docs_k50(b *testing.B) {
	benchVectorKNN(b, 128, 1000, 50)
}

func BenchmarkVector_KNN_128d_1000docs_k100(b *testing.B) {
	benchVectorKNN(b, 128, 1000, 100)
}

func benchVectorKNN(b *testing.B, dims, numDocs, k int) {
	idx := setupVectorIndex(b, dims, numDocs)
	rng := rand.New(rand.NewSource(99))
	qvec := randomVector32(dims, rng)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		q := query.NewKNNQuery(qvec, k).SetField("embedding")
		_, err := idx.Search(context.Background(), q)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// BENCHMARK: Hybrid search (text + vector)
// ─────────────────────────────────────────────────────────

func BenchmarkVector_Hybrid_128d_1000docs(b *testing.B) {
	store := newMemStore()
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("title", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("content", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(128))
	m.DefaultMapping = dm

	idx, _ := searchmd.New(store, m)

	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 1000; i++ {
		vec := randomVector(128, rng)
		normalizeF64(vec)
		doc := map[string]interface{}{
			"title":     fmt.Sprintf("Document %d: %s", i, corpus[i%len(corpus)][:30]),
			"content":   corpus[i%len(corpus)],
			"embedding": vec,
		}
		idx.Index(fmt.Sprintf("doc%d", i), doc)
	}

	qvec := randomVector32(128, rng)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		bq := query.NewBooleanQuery()
		bq.AddShould(query.NewMatchQuery("distributed systems").SetField("content"))
		bq.AddShould(query.NewKNNQuery(qvec, 10).SetField("embedding"))
		idx.Search(context.Background(), bq)
	}
}

// ─────────────────────────────────────────────────────────
// TEST: Vector search correctness
// ─────────────────────────────────────────────────────────

func TestVector_Correctness_KNN(t *testing.T) {
	store := newMemStore()
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(4))
	m.DefaultMapping = dm

	idx, _ := searchmd.New(store, m)

	// Index vectors with known relationships
	// cluster A: near [1,0,0,0]
	idx.Index("a1", map[string]interface{}{"embedding": []float64{1.0, 0.0, 0.0, 0.0}})
	idx.Index("a2", map[string]interface{}{"embedding": []float64{0.95, 0.05, 0.0, 0.0}})
	idx.Index("a3", map[string]interface{}{"embedding": []float64{0.9, 0.1, 0.0, 0.0}})

	// cluster B: near [0,1,0,0]
	idx.Index("b1", map[string]interface{}{"embedding": []float64{0.0, 1.0, 0.0, 0.0}})
	idx.Index("b2", map[string]interface{}{"embedding": []float64{0.05, 0.95, 0.0, 0.0}})
	idx.Index("b3", map[string]interface{}{"embedding": []float64{0.1, 0.9, 0.0, 0.0}})

	// cluster C: near [0,0,1,0]
	idx.Index("c1", map[string]interface{}{"embedding": []float64{0.0, 0.0, 1.0, 0.0}})
	idx.Index("c2", map[string]interface{}{"embedding": []float64{0.0, 0.0, 0.95, 0.05}})

	// orthogonal: [0,0,0,1]
	idx.Index("d1", map[string]interface{}{"embedding": []float64{0.0, 0.0, 0.0, 1.0}})

	t.Run("ClusterA_TopK3", func(t *testing.T) {
		q := query.NewKNNQuery([]float32{1, 0, 0, 0}, 3).SetField("embedding")
		results, err := idx.Search(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Query: [1,0,0,0] k=3")
		for _, h := range results.Hits {
			t.Logf("  %-4s score=%.4f", h.ID, h.Score)
		}

		if results.Total != 3 {
			t.Errorf("expected 3 hits, got %d", results.Total)
		}
		// Should return a1, a2, a3 in order
		expected := []string{"a1", "a2", "a3"}
		for i, exp := range expected {
			if i < len(results.Hits) && results.Hits[i].ID != exp {
				t.Errorf("position %d: expected %s, got %s", i, exp, results.Hits[i].ID)
			}
		}
	})

	t.Run("ClusterB_TopK3", func(t *testing.T) {
		q := query.NewKNNQuery([]float32{0, 1, 0, 0}, 3).SetField("embedding")
		results, err := idx.Search(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Query: [0,1,0,0] k=3")
		for _, h := range results.Hits {
			t.Logf("  %-4s score=%.4f", h.ID, h.Score)
		}

		if results.Total != 3 {
			t.Errorf("expected 3 hits, got %d", results.Total)
		}
		expected := []string{"b1", "b2", "b3"}
		for i, exp := range expected {
			if i < len(results.Hits) && results.Hits[i].ID != exp {
				t.Errorf("position %d: expected %s, got %s", i, exp, results.Hits[i].ID)
			}
		}
	})

	t.Run("OrthogonalExclusion", func(t *testing.T) {
		// Query [0,0,0,1] — only d1 should match (all others are orthogonal)
		q := query.NewKNNQuery([]float32{0, 0, 0, 1}, 10).SetField("embedding")
		results, err := idx.Search(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Query: [0,0,0,1] k=10")
		for _, h := range results.Hits {
			t.Logf("  %-4s score=%.4f", h.ID, h.Score)
		}

		// d1 exact match, c2 has slight component in 4th dim
		for _, h := range results.Hits {
			if h.ID == "a1" || h.ID == "b1" || h.ID == "c1" {
				t.Errorf("purely orthogonal doc %s should not appear", h.ID)
			}
		}
	})

	t.Run("ScoreMonotonicity", func(t *testing.T) {
		q := query.NewKNNQuery([]float32{1, 0, 0, 0}, 9).SetField("embedding")
		results, err := idx.Search(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Query: [1,0,0,0] k=9 — full ranking")
		for _, h := range results.Hits {
			t.Logf("  %-4s score=%.4f", h.ID, h.Score)
		}

		for i := 1; i < len(results.Hits); i++ {
			if results.Hits[i].Score > results.Hits[i-1].Score {
				t.Errorf("scores not monotonically decreasing: %s(%.4f) > %s(%.4f)",
					results.Hits[i].ID, results.Hits[i].Score,
					results.Hits[i-1].ID, results.Hits[i-1].Score)
			}
		}
	})
}

// ─────────────────────────────────────────────────────────
// TEST: Vector search recall quality at scale
// ─────────────────────────────────────────────────────────

func TestVector_RecallAtScale(t *testing.T) {
	dims := 128
	numDocs := 1000

	store := newMemStore()
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(dims))
	m.DefaultMapping = dm
	idx, _ := searchmd.New(store, m)

	rng := rand.New(rand.NewSource(42))
	vectors := make([][]float64, numDocs)
	for i := 0; i < numDocs; i++ {
		vec := randomVector(dims, rng)
		normalizeF64(vec)
		vectors[i] = vec
		idx.Index(fmt.Sprintf("doc%d", i), map[string]interface{}{"embedding": vec})
	}

	// Compute true top-K by brute force cosine similarity
	queryVec := randomVector32(dims, rng)
	queryVecF64 := make([]float64, dims)
	for i, v := range queryVec {
		queryVecF64[i] = float64(v)
	}

	type scored struct {
		id    string
		score float64
	}
	var bruteForce []scored
	for i, vec := range vectors {
		sim := cosineSimF64(queryVecF64, vec)
		if sim > 0 {
			bruteForce = append(bruteForce, scored{fmt.Sprintf("doc%d", i), sim})
		}
	}
	// Sort descending
	for i := 0; i < len(bruteForce); i++ {
		for j := i + 1; j < len(bruteForce); j++ {
			if bruteForce[j].score > bruteForce[i].score {
				bruteForce[i], bruteForce[j] = bruteForce[j], bruteForce[i]
			}
		}
	}

	for _, k := range []int{1, 5, 10, 50} {
		t.Run(fmt.Sprintf("k=%d", k), func(t *testing.T) {
			q := query.NewKNNQuery(queryVec, k).SetField("embedding")
			req := search.NewSearchRequestOptions(q, k, 0)
			results, err := idx.SearchWithRequest(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}

			// Build ground truth set
			truthK := k
			if truthK > len(bruteForce) {
				truthK = len(bruteForce)
			}
			truthSet := make(map[string]bool)
			for i := 0; i < truthK; i++ {
				truthSet[bruteForce[i].id] = true
			}

			// Compute recall@k
			found := 0
			for _, h := range results.Hits {
				if truthSet[h.ID] {
					found++
				}
			}
			recallAtK := float64(found) / float64(truthK)

			t.Logf("k=%d: returned=%d, recall@k=%.2f", k, len(results.Hits), recallAtK)

			if k <= 10 {
				t.Logf("  Top results:")
				limit := k
				if limit > len(results.Hits) {
					limit = len(results.Hits)
				}
				for i := 0; i < limit; i++ {
					h := results.Hits[i]
					marker := " "
					if truthSet[h.ID] {
						marker = "*"
					}
					t.Logf("    %s %-8s score=%.6f", marker, h.ID, h.Score)
				}
				t.Logf("  Ground truth:")
				for i := 0; i < limit && i < len(bruteForce); i++ {
					t.Logf("    %-8s score=%.6f", bruteForce[i].id, bruteForce[i].score)
				}
			}

			// Since search.md uses exact brute force, recall should be 1.0
			if recallAtK < 1.0 {
				t.Errorf("recall@%d = %.2f, expected 1.0 for exact search", k, recallAtK)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────
// TEST: Hybrid text + vector search
// ─────────────────────────────────────────────────────────

func TestVector_HybridSearch(t *testing.T) {
	store := newMemStore()
	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()
	dm.AddFieldMapping("content", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(4))
	m.DefaultMapping = dm
	idx, _ := searchmd.New(store, m)

	// Index docs with both text and vectors
	// doc1: text about databases, vector near [1,0,0,0]
	idx.Index("doc1", map[string]interface{}{
		"content":   "database query optimization techniques for PostgreSQL",
		"embedding": []float64{0.95, 0.05, 0.0, 0.0},
	})
	// doc2: text about databases, vector near [0,1,0,0]
	idx.Index("doc2", map[string]interface{}{
		"content":   "database replication and high availability strategies",
		"embedding": []float64{0.05, 0.95, 0.0, 0.0},
	})
	// doc3: text about search, vector near [1,0,0,0]
	idx.Index("doc3", map[string]interface{}{
		"content":   "full-text search engine architecture and inverted indexes",
		"embedding": []float64{0.9, 0.1, 0.0, 0.0},
	})
	// doc4: irrelevant text, irrelevant vector
	idx.Index("doc4", map[string]interface{}{
		"content":   "machine learning algorithms for image classification",
		"embedding": []float64{0.0, 0.0, 0.0, 1.0},
	})

	t.Run("TextOnly", func(t *testing.T) {
		q := query.NewMatchQuery("database query").SetField("content")
		results, _ := idx.Search(context.Background(), q)
		t.Logf("Text-only 'database query': %d hits", results.Total)
		for _, h := range results.Hits {
			t.Logf("  %-6s score=%.4f", h.ID, h.Score)
		}
	})

	t.Run("VectorOnly", func(t *testing.T) {
		q := query.NewKNNQuery([]float32{1, 0, 0, 0}, 4).SetField("embedding")
		results, _ := idx.Search(context.Background(), q)
		t.Logf("Vector-only [1,0,0,0] k=4: %d hits", results.Total)
		for _, h := range results.Hits {
			t.Logf("  %-6s score=%.4f", h.ID, h.Score)
		}
	})

	t.Run("HybridBooleanShould", func(t *testing.T) {
		bq := query.NewBooleanQuery()
		bq.AddShould(query.NewMatchQuery("database").SetField("content"))
		bq.AddShould(query.NewKNNQuery([]float32{1, 0, 0, 0}, 4).SetField("embedding"))

		results, err := idx.Search(context.Background(), bq)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Hybrid (text 'database' OR vector [1,0,0,0]): %d hits", results.Total)
		for _, h := range results.Hits {
			t.Logf("  %-6s score=%.4f", h.ID, h.Score)
		}

		// doc1 should rank highest: matches both text AND vector
		if len(results.Hits) > 0 && results.Hits[0].ID != "doc1" {
			t.Errorf("expected doc1 (matches both signals) as top result, got %s", results.Hits[0].ID)
		}
	})
}

// ─────────────────────────────────────────────────────────
// TEST: Vector dimension scaling
// ─────────────────────────────────────────────────────────

func TestVector_DimensionScaling(t *testing.T) {
	for _, dims := range []int{4, 32, 128, 384, 768} {
		t.Run(fmt.Sprintf("dims=%d", dims), func(t *testing.T) {
			idx := setupVectorIndex(t, dims, 100)
			rng := rand.New(rand.NewSource(99))
			qvec := randomVector32(dims, rng)

			q := query.NewKNNQuery(qvec, 5).SetField("embedding")
			results, err := idx.Search(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}

			t.Logf("dims=%d: %d hits returned", dims, results.Total)
			for _, h := range results.Hits {
				t.Logf("  %-8s score=%.6f", h.ID, h.Score)
			}

			// Verify scores are valid (0..1 range for normalized cosine)
			for _, h := range results.Hits {
				if h.Score < 0 || h.Score > 1.01 {
					t.Errorf("score out of range: %s = %.4f", h.ID, h.Score)
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────
// BENCHMARK: Memory usage for vector indexing
// ─────────────────────────────────────────────────────────

func BenchmarkVector_Memory_128d_1000docs(b *testing.B) {
	b.ReportAllocs()
	benchVectorIndex(b, 128, 1000)
}

func BenchmarkVector_Memory_768d_1000docs(b *testing.B) {
	b.ReportAllocs()
	benchVectorIndex(b, 768, 1000)
}

// ─────────────────────────────────────────────────────────
// BENCHMARK: SearchWithRequest (pagination)
// ─────────────────────────────────────────────────────────

func BenchmarkVector_KNN_WithPagination(b *testing.B) {
	idx := setupVectorIndex(b, 128, 1000)
	rng := rand.New(rand.NewSource(99))
	qvec := randomVector32(128, rng)

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		q := query.NewKNNQuery(qvec, 10).SetField("embedding")
		req := search.NewSearchRequestOptions(q, 5, 0)
		idx.SearchWithRequest(context.Background(), req)
	}
}

// ─────────────────────────────────────────────────────────
// cosine similarity in float64 for ground truth computation
// ─────────────────────────────────────────────────────────

func cosineSimF64(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
