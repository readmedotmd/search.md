package bench_comparison

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/blevesearch/bleve/v2"
	bquery "github.com/blevesearch/bleve/v2/search/query"
	searchmd "github.com/readmedotmd/search.md"
	"github.com/readmedotmd/search.md/search"
	"github.com/readmedotmd/search.md/search/query"
)

// ─────────────────────────────────────────────────────────
// Richer corpus with varying relevance levels for quality testing
// ─────────────────────────────────────────────────────────

type testDoc struct {
	id      string
	title   string
	content string
}

var qualityCorpus = []testDoc{
	// Highly relevant to "database query optimization"
	{"db1", "Database Query Optimization Techniques", "The database query optimizer analyzes SQL statements and selects the most efficient execution plan using cost-based optimization strategies"},
	{"db2", "Advanced Query Planning in PostgreSQL", "PostgreSQL query planner uses statistics and cost models to optimize database queries for maximum throughput"},
	{"db3", "Index Selection for Database Query Performance", "Choosing the right database indexes dramatically improves query performance by reducing disk I/O operations"},

	// Moderately relevant (mentions database or query but not optimization)
	{"db4", "Introduction to NoSQL Databases", "NoSQL databases provide flexible schema design for modern applications that need horizontal scalability"},
	{"db5", "SQL Query Fundamentals", "Writing efficient SQL queries requires understanding joins, subqueries, and aggregate functions"},
	{"db6", "Database Replication Strategies", "Database replication ensures high availability through primary-replica architectures and conflict resolution"},

	// Tangentially relevant (mentions optimization but not database)
	{"opt1", "Code Optimization in Compilers", "Modern compilers apply optimization passes including dead code elimination loop unrolling and register allocation"},
	{"opt2", "Network Performance Optimization", "Network optimization involves reducing latency improving throughput and minimizing packet loss across distributed systems"},

	// Irrelevant documents
	{"irr1", "Introduction to Machine Learning", "Machine learning algorithms learn patterns from data to make predictions without explicit programming"},
	{"irr2", "Cloud Computing Fundamentals", "Cloud computing provides on-demand access to computing resources including servers storage and networking"},
	{"irr3", "The History of Programming Languages", "Programming languages have evolved from assembly to high-level languages enabling greater developer productivity"},
	{"irr4", "Container Orchestration with Kubernetes", "Kubernetes automates deployment scaling and management of containerized applications across clusters"},
	{"irr5", "Functional Programming Principles", "Functional programming emphasizes immutable data pure functions and declarative style over imperative approaches"},
	{"irr6", "Cryptography and Information Security", "Modern cryptography relies on mathematical problems that are computationally infeasible to solve without keys"},
	{"irr7", "User Interface Design Patterns", "Good user interface design follows principles of consistency feedback and progressive disclosure"},

	// Documents with overlapping terms for ranking challenge
	{"rank1", "Full-Text Search Engine Architecture", "Search engines use inverted indexes to map terms to documents enabling fast full-text search and retrieval"},
	{"rank2", "Building a Search Application", "Building search applications requires understanding of text analysis tokenization and relevance scoring"},
	{"rank3", "Information Retrieval Theory", "Information retrieval theory provides the mathematical foundation for search including TF-IDF and BM25 scoring models"},

	// Phrase matching test documents
	{"ph1", "The Quick Brown Fox Story", "The quick brown fox jumps over the lazy dog in the sunny meadow"},
	{"ph2", "Brown Fox Sightings", "A brown fox was spotted near the quick stream running through the forest"},
	{"ph3", "Quick Decisions in Business", "Making quick brown decisions requires careful analysis of market fox conditions and risk"},

	// Near-duplicate documents for ranking discrimination
	{"dup1", "Distributed Systems Design", "Distributed systems require consensus algorithms to maintain consistency across multiple nodes in a network"},
	{"dup2", "Designing Distributed Systems", "Designing distributed systems involves trade-offs between consistency availability and partition tolerance as described by the CAP theorem"},
	{"dup3", "Distributed Computing Challenges", "Distributed computing faces challenges including network partitions clock synchronization and Byzantine fault tolerance"},
}

// ─────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────

func setupSearchMD(t *testing.T) *searchmd.SearchIndex {
	t.Helper()
	store := newMemStore()
	idx, err := searchmd.New(store, nil)
	if err != nil {
		t.Fatalf("search.md New: %v", err)
	}
	for _, doc := range qualityCorpus {
		d := map[string]interface{}{
			"title":   doc.title,
			"content": doc.content,
		}
		if err := idx.Index(doc.id, d); err != nil {
			t.Fatalf("index %s: %v", doc.id, err)
		}
	}
	return idx
}

func setupBleve(t *testing.T) bleve.Index {
	t.Helper()
	dir, err := os.MkdirTemp("", "bleve-quality-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	idx, err := bleve.New(dir, bleve.NewIndexMapping())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { idx.Close() })

	for _, doc := range qualityCorpus {
		d := map[string]interface{}{
			"title":   doc.title,
			"content": doc.content,
		}
		if err := idx.Index(doc.id, d); err != nil {
			t.Fatalf("index %s: %v", doc.id, err)
		}
	}
	return idx
}

type hit struct {
	ID    string
	Score float64
}

func searchMDQuery(t *testing.T, idx *searchmd.SearchIndex, q query.Query, size int) []hit {
	t.Helper()
	req := search.NewSearchRequestOptions(q, size, 0)
	res, err := idx.SearchWithRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("search.md search: %v", err)
	}
	hits := make([]hit, len(res.Hits))
	for i, h := range res.Hits {
		hits[i] = hit{ID: h.ID, Score: h.Score}
	}
	return hits
}

func bleveQuery(t *testing.T, idx bleve.Index, req *bleve.SearchRequest) []hit {
	t.Helper()
	res, err := idx.Search(req)
	if err != nil {
		t.Fatalf("bleve search: %v", err)
	}
	hits := make([]hit, len(res.Hits))
	for i, h := range res.Hits {
		hits[i] = hit{ID: h.ID, Score: h.Score}
	}
	return hits
}

func hitIDs(hits []hit) []string {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	return ids
}

func containsID(hits []hit, id string) bool {
	for _, h := range hits {
		if h.ID == id {
			return true
		}
	}
	return false
}

func setOf(ids []string) map[string]bool {
	s := make(map[string]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}

// precision computes what fraction of returned results are relevant.
func precision(returned []hit, relevant map[string]bool) float64 {
	if len(returned) == 0 {
		return 0
	}
	count := 0
	for _, h := range returned {
		if relevant[h.ID] {
			count++
		}
	}
	return float64(count) / float64(len(returned))
}

// recall computes what fraction of relevant docs were returned.
func recall(returned []hit, relevant map[string]bool) float64 {
	if len(relevant) == 0 {
		return 0
	}
	count := 0
	for _, h := range returned {
		if relevant[h.ID] {
			count++
		}
	}
	return float64(count) / float64(len(relevant))
}

// dcg computes Discounted Cumulative Gain for ranking quality.
func dcg(ranked []string, relevance map[string]int) float64 {
	score := 0.0
	for i, id := range ranked {
		rel := float64(relevance[id])
		score += rel / math.Log2(float64(i+2)) // i+2 because log2(1) = 0
	}
	return score
}

// ndcg computes Normalized DCG.
func ndcg(ranked []string, relevance map[string]int) float64 {
	actual := dcg(ranked, relevance)
	// Build ideal ranking
	type kv struct {
		id  string
		rel int
	}
	var items []kv
	for id, rel := range relevance {
		if rel > 0 {
			items = append(items, kv{id, rel})
		}
	}
	// Sort by relevance desc
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].rel > items[i].rel {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	ideal := make([]string, len(items))
	for i, kv := range items {
		ideal[i] = kv.id
	}
	idealDCG := dcg(ideal, relevance)
	if idealDCG == 0 {
		return 0
	}
	return actual / idealDCG
}

// ─────────────────────────────────────────────────────────
// TEST 1: Recall — "database query optimization"
// Are the right documents found?
// ─────────────────────────────────────────────────────────

func TestQuality_Recall_DatabaseQuery(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	relevant := setOf([]string{"db1", "db2", "db3", "db5"}) // docs about database + query

	// search.md
	sq := query.NewMatchQuery("database query optimization").SetField("content")
	sHits := searchMDQuery(t, sIdx, sq, 10)

	// bleve
	bq := bleve.NewMatchQuery("database query optimization")
	bq.SetField("content")
	bReq := bleve.NewSearchRequestOptions(bq, 10, 0, false)
	bHits := bleveQuery(t, bIdx, bReq)

	sRecall := recall(sHits, relevant)
	bRecall := recall(bHits, relevant)
	sPrecision := precision(sHits, relevant)
	bPrecision := precision(bHits, relevant)

	t.Logf("Query: 'database query optimization' on content")
	t.Logf("")
	t.Logf("search.md: %d hits, recall=%.2f, precision=%.2f", len(sHits), sRecall, sPrecision)
	t.Logf("  IDs: %v", hitIDs(sHits))
	for _, h := range sHits {
		marker := " "
		if relevant[h.ID] {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
	t.Logf("")
	t.Logf("bleve:     %d hits, recall=%.2f, precision=%.2f", len(bHits), bRecall, bPrecision)
	t.Logf("  IDs: %v", hitIDs(bHits))
	for _, h := range bHits {
		marker := " "
		if relevant[h.ID] {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
}

// ─────────────────────────────────────────────────────────
// TEST 2: Ranking quality (NDCG) — "search engine retrieval"
// Are relevant docs ranked higher than less relevant ones?
// ─────────────────────────────────────────────────────────

func TestQuality_NDCG_SearchEngineRetrieval(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	// Graded relevance: 3=highly relevant, 2=relevant, 1=somewhat, 0=irrelevant
	relevance := map[string]int{
		"rank1": 3, // "Full-Text Search Engine Architecture" - exact match
		"rank2": 3, // "Building a Search Application" - very relevant
		"rank3": 2, // "Information Retrieval Theory" - relevant
		"db1":   1, // mentions query/optimization but not search engines
	}

	sq := query.NewMatchQuery("search engine retrieval").SetField("content")
	sHits := searchMDQuery(t, sIdx, sq, 10)

	bq := bleve.NewMatchQuery("search engine retrieval")
	bq.SetField("content")
	bReq := bleve.NewSearchRequestOptions(bq, 10, 0, false)
	bHits := bleveQuery(t, bIdx, bReq)

	sNDCG := ndcg(hitIDs(sHits), relevance)
	bNDCG := ndcg(hitIDs(bHits), relevance)

	t.Logf("Query: 'search engine retrieval' on content")
	t.Logf("")
	t.Logf("search.md: NDCG=%.4f, %d hits", sNDCG, len(sHits))
	for _, h := range sHits {
		rel := relevance[h.ID]
		t.Logf("  %-6s score=%.4f relevance=%d", h.ID, h.Score, rel)
	}
	t.Logf("")
	t.Logf("bleve:     NDCG=%.4f, %d hits", bNDCG, len(bHits))
	for _, h := range bHits {
		rel := relevance[h.ID]
		t.Logf("  %-6s score=%.4f relevance=%d", h.ID, h.Score, rel)
	}
}

// ─────────────────────────────────────────────────────────
// TEST 3: Phrase matching precision
// Does "quick brown fox" match the exact phrase, not scattered terms?
// ─────────────────────────────────────────────────────────

func TestQuality_PhraseMatch(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	// Only ph1 has the exact phrase "quick brown fox"
	exactPhraseDoc := "ph1"

	sq := query.NewMatchPhraseQuery("quick brown fox").SetField("content")
	sHits := searchMDQuery(t, sIdx, sq, 10)

	bq := bleve.NewMatchPhraseQuery("quick brown fox")
	bq.SetField("content")
	bReq := bleve.NewSearchRequestOptions(bq, 10, 0, false)
	bHits := bleveQuery(t, bIdx, bReq)

	t.Logf("Query: phrase 'quick brown fox' on content")
	t.Logf("")
	t.Logf("search.md: %d hits", len(sHits))
	for _, h := range sHits {
		marker := " "
		if h.ID == exactPhraseDoc {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
	sCorrect := len(sHits) == 1 && sHits[0].ID == exactPhraseDoc
	t.Logf("  Exact phrase only: %v", sCorrect)

	t.Logf("")
	t.Logf("bleve:     %d hits", len(bHits))
	for _, h := range bHits {
		marker := " "
		if h.ID == exactPhraseDoc {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
	bCorrect := len(bHits) == 1 && bHits[0].ID == exactPhraseDoc
	t.Logf("  Exact phrase only: %v", bCorrect)
}

// ─────────────────────────────────────────────────────────
// TEST 4: Fuzzy matching — typo tolerance
// Does "distribted" (missing 'u') find "distributed" docs?
// ─────────────────────────────────────────────────────────

func TestQuality_FuzzyTypoTolerance(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	expected := setOf([]string{"dup1", "dup2", "dup3"}) // "distributed" docs

	sq := query.NewFuzzyQuery("distribted").SetField("content").SetFuzziness(1)
	sHits := searchMDQuery(t, sIdx, sq, 10)

	bq := bleve.NewFuzzyQuery("distribted")
	bq.SetField("content")
	bq.SetFuzziness(1)
	bReq := bleve.NewSearchRequestOptions(bq, 10, 0, false)
	bHits := bleveQuery(t, bIdx, bReq)

	sRecall := recall(sHits, expected)
	bRecall := recall(bHits, expected)

	t.Logf("Query: fuzzy 'distribted' (fuzziness=1) on content")
	t.Logf("")
	t.Logf("search.md: %d hits, recall=%.2f", len(sHits), sRecall)
	for _, h := range sHits {
		marker := " "
		if expected[h.ID] {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
	t.Logf("")
	t.Logf("bleve:     %d hits, recall=%.2f", len(bHits), bRecall)
	for _, h := range bHits {
		marker := " "
		if expected[h.ID] {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
}

// ─────────────────────────────────────────────────────────
// TEST 5: Boolean query — must + must_not precision
// Must contain "distributed", must NOT contain "consensus"
// ─────────────────────────────────────────────────────────

func TestQuality_BooleanMustMustNot(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	// dup1 mentions consensus, dup2 and dup3 don't
	expected := setOf([]string{"dup2", "dup3"})
	excluded := "dup1"

	sbq := query.NewBooleanQuery()
	sbq.AddMust(query.NewMatchQuery("distributed").SetField("content"))
	sbq.AddMustNot(query.NewMatchQuery("consensus").SetField("content"))
	sHits := searchMDQuery(t, sIdx, sbq, 10)

	bmust := bleve.NewMatchQuery("distributed")
	bmust.SetField("content")
	bmustNot := bleve.NewMatchQuery("consensus")
	bmustNot.SetField("content")
	bbq := bleve.NewBooleanQuery()
	bbq.AddMust(bmust)
	bbq.AddMustNot(bmustNot)
	bReq := bleve.NewSearchRequestOptions(bbq, 10, 0, false)
	bHits := bleveQuery(t, bIdx, bReq)

	t.Logf("Query: must='distributed' must_not='consensus' on content")
	t.Logf("")
	t.Logf("search.md: %d hits", len(sHits))
	sExcluded := false
	for _, h := range sHits {
		marker := " "
		if expected[h.ID] {
			marker = "*"
		}
		if h.ID == excluded {
			marker = "X"
			sExcluded = true
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
	t.Logf("  Correctly excluded '%s': %v", excluded, !sExcluded)
	t.Logf("  Recall: %.2f", recall(sHits, expected))

	t.Logf("")
	t.Logf("bleve:     %d hits", len(bHits))
	bExcluded := false
	for _, h := range bHits {
		marker := " "
		if expected[h.ID] {
			marker = "*"
		}
		if h.ID == excluded {
			marker = "X"
			bExcluded = true
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
	t.Logf("  Correctly excluded '%s': %v", excluded, !bExcluded)
	t.Logf("  Recall: %.2f", recall(bHits, expected))
}

// ─────────────────────────────────────────────────────────
// TEST 6: Prefix query — "optim*" should find optimization docs
// ─────────────────────────────────────────────────────────

func TestQuality_PrefixQuery(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	// Docs that contain words starting with "optim"
	expected := setOf([]string{"db1", "db2", "opt1", "opt2"})

	sq := query.NewPrefixQuery("optim").SetField("content")
	sHits := searchMDQuery(t, sIdx, sq, 10)

	bq := bleve.NewPrefixQuery("optim")
	bq.SetField("content")
	bReq := bleve.NewSearchRequestOptions(bq, 10, 0, false)
	bHits := bleveQuery(t, bIdx, bReq)

	t.Logf("Query: prefix 'optim*' on content")
	t.Logf("")
	t.Logf("search.md: %d hits, recall=%.2f, precision=%.2f", len(sHits), recall(sHits, expected), precision(sHits, expected))
	for _, h := range sHits {
		marker := " "
		if expected[h.ID] {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
	t.Logf("")
	t.Logf("bleve:     %d hits, recall=%.2f, precision=%.2f", len(bHits), recall(bHits, expected), precision(bHits, expected))
	for _, h := range bHits {
		marker := " "
		if expected[h.ID] {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
}

// ─────────────────────────────────────────────────────────
// TEST 7: Score differentiation — do scores distinguish relevance?
// ─────────────────────────────────────────────────────────

func TestQuality_ScoreDifferentiation(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	// For "database query", db1 should score highest (has both + optimizer)
	sq := query.NewMatchQuery("database query").SetField("content")
	sHits := searchMDQuery(t, sIdx, sq, 10)

	bq := bleve.NewMatchQuery("database query")
	bq.SetField("content")
	bReq := bleve.NewSearchRequestOptions(bq, 10, 0, false)
	bHits := bleveQuery(t, bIdx, bReq)

	t.Logf("Query: 'database query' on content — score distribution")
	t.Logf("")

	reportScores := func(name string, hits []hit) {
		t.Logf("%s: %d hits", name, len(hits))
		if len(hits) == 0 {
			return
		}
		maxScore := hits[0].Score
		minScore := hits[len(hits)-1].Score
		spread := maxScore - minScore
		for _, h := range hits {
			bar := strings.Repeat("█", int(h.Score/maxScore*40))
			t.Logf("  %-6s score=%.4f %s", h.ID, h.Score, bar)
		}
		t.Logf("  Score spread: %.4f (max=%.4f, min=%.4f, ratio=%.2fx)",
			spread, maxScore, minScore, maxScore/math.Max(minScore, 0.0001))
		// Check monotonically decreasing
		mono := true
		for i := 1; i < len(hits); i++ {
			if hits[i].Score > hits[i-1].Score {
				mono = false
				break
			}
		}
		t.Logf("  Monotonically decreasing scores: %v", mono)
	}

	reportScores("search.md", sHits)
	t.Logf("")
	reportScores("bleve", bHits)
}

// ─────────────────────────────────────────────────────────
// TEST 8: Multi-term recall — how well do both handle queries
// with terms that appear in different documents?
// ─────────────────────────────────────────────────────────

func TestQuality_MultiTermCoverage(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	// "algorithms distributed systems" should find docs about:
	// - distributed systems (dup1, dup2, dup3)
	// - algorithms (irr1 = ML algorithms, db1 = cost-based)
	// - consensus algorithms (dup1)
	allRelevant := setOf([]string{"dup1", "dup2", "dup3", "irr1"})

	sq := query.NewMatchQuery("algorithms distributed systems").SetField("content")
	sHits := searchMDQuery(t, sIdx, sq, 15)

	bq := bleve.NewMatchQuery("algorithms distributed systems")
	bq.SetField("content")
	bReq := bleve.NewSearchRequestOptions(bq, 15, 0, false)
	bHits := bleveQuery(t, bIdx, bReq)

	t.Logf("Query: 'algorithms distributed systems' on content")
	t.Logf("")
	t.Logf("search.md: %d hits, recall=%.2f, precision=%.2f", len(sHits), recall(sHits, allRelevant), precision(sHits, allRelevant))
	for _, h := range sHits {
		marker := " "
		if allRelevant[h.ID] {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
	t.Logf("")
	t.Logf("bleve:     %d hits, recall=%.2f, precision=%.2f", len(bHits), recall(bHits, allRelevant), precision(bHits, allRelevant))
	for _, h := range bHits {
		marker := " "
		if allRelevant[h.ID] {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
}

// ─────────────────────────────────────────────────────────
// TEST 9: Wildcard precision
// ─────────────────────────────────────────────────────────

func TestQuality_Wildcard(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	// "dat*" should match: data, database, databases
	docsWithDat := setOf([]string{"db1", "db2", "db3", "db4", "db5", "db6", "irr1", "irr6", "rank3"})

	sq := query.NewWildcardQuery("dat*").SetField("content")
	sHits := searchMDQuery(t, sIdx, sq, 20)

	bq := bleve.NewWildcardQuery("dat*")
	bq.SetField("content")
	bReq := bleve.NewSearchRequestOptions(bq, 20, 0, false)
	bHits := bleveQuery(t, bIdx, bReq)

	t.Logf("Query: wildcard 'dat*' on content")
	t.Logf("")
	t.Logf("search.md: %d hits, recall=%.2f, precision=%.2f", len(sHits), recall(sHits, docsWithDat), precision(sHits, docsWithDat))
	for _, h := range sHits {
		marker := " "
		if docsWithDat[h.ID] {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
	t.Logf("")
	t.Logf("bleve:     %d hits, recall=%.2f, precision=%.2f", len(bHits), recall(bHits, docsWithDat), precision(bHits, docsWithDat))
	for _, h := range bHits {
		marker := " "
		if docsWithDat[h.ID] {
			marker = "*"
		}
		t.Logf("  %s %-6s score=%.4f", marker, h.ID, h.Score)
	}
}

// ─────────────────────────────────────────────────────────
// TEST 10: Total hit count accuracy
// ─────────────────────────────────────────────────────────

func TestQuality_TotalHitCount(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	// Count how many docs actually contain "the" (very common word)
	actualCount := 0
	for _, doc := range qualityCorpus {
		lower := strings.ToLower(doc.content)
		if strings.Contains(lower, "the") {
			actualCount++
		}
	}

	sq := query.NewTermQuery("the").SetField("content")
	sReq := search.NewSearchRequestOptions(sq, 100, 0)
	sRes, err := sIdx.SearchWithRequest(context.Background(), sReq)
	if err != nil {
		t.Fatal(err)
	}

	bq := bleve.NewTermQuery("the")
	bq.SetField("content")
	bReq := bleve.NewSearchRequestOptions(bq, 100, 0, false)
	bRes, err := bIdx.Search(bReq)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Query: term 'the' on content")
	t.Logf("Actual docs containing 'the': %d", actualCount)
	t.Logf("search.md: Total=%d, Hits=%d", sRes.Total, len(sRes.Hits))
	t.Logf("bleve:     Total=%d, Hits=%d", bRes.Total, len(bRes.Hits))

	// Now test with a rarer term
	sq2 := query.NewTermQuery("consensus").SetField("content")
	sReq2 := search.NewSearchRequestOptions(sq2, 100, 0)
	sRes2, err := sIdx.SearchWithRequest(context.Background(), sReq2)
	if err != nil {
		t.Fatal(err)
	}

	bq2 := bleve.NewTermQuery("consensus")
	bq2.SetField("content")
	bReq2 := bleve.NewSearchRequestOptions(bq2, 100, 0, false)
	bRes2, err := bIdx.Search(bReq2)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("")
	t.Logf("Query: term 'consensus' on content")
	t.Logf("search.md: Total=%d, Hits=%d", sRes2.Total, len(sRes2.Hits))
	for _, h := range sRes2.Hits {
		t.Logf("  %-6s score=%.4f", h.ID, h.Score)
	}
	t.Logf("bleve:     Total=%d, Hits=%d", bRes2.Total, len(bRes2.Hits))
	for _, h := range bRes2.Hits {
		t.Logf("  %-6s score=%.4f", h.ID, h.Score)
	}
}

// ─────────────────────────────────────────────────────────
// SUMMARY — run all quality metrics and produce a comparison
// ─────────────────────────────────────────────────────────

func TestQuality_Summary(t *testing.T) {
	sIdx := setupSearchMD(t)
	bIdx := setupBleve(t)

	type testCase struct {
		name       string
		smdQuery   query.Query
		bleveQuery *bleve.SearchRequest
		relevant   map[string]bool
		relevance  map[string]int // for NDCG (nil = skip NDCG)
	}

	mkBleveReq := func(q interface{ SetField(string) }, field string) *bleve.SearchRequest {
		q.SetField(field)
		return bleve.NewSearchRequestOptions(q.(bquery.Query), 10, 0, false)
	}

	cases := []testCase{
		{
			name:       "database optimization",
			smdQuery:   query.NewMatchQuery("database query optimization").SetField("content"),
			bleveQuery: mkBleveReq(bleve.NewMatchQuery("database query optimization"), "content"),
			relevant:   setOf([]string{"db1", "db2", "db3", "db5"}),
			relevance:  map[string]int{"db1": 3, "db2": 3, "db3": 2, "db5": 1, "db4": 1, "db6": 1},
		},
		{
			name:       "search engine",
			smdQuery:   query.NewMatchQuery("search engine retrieval").SetField("content"),
			bleveQuery: mkBleveReq(bleve.NewMatchQuery("search engine retrieval"), "content"),
			relevant:   setOf([]string{"rank1", "rank2", "rank3"}),
			relevance:  map[string]int{"rank1": 3, "rank2": 3, "rank3": 2},
		},
		{
			name:       "distributed systems",
			smdQuery:   query.NewMatchQuery("distributed systems").SetField("content"),
			bleveQuery: mkBleveReq(bleve.NewMatchQuery("distributed systems"), "content"),
			relevant:   setOf([]string{"dup1", "dup2", "dup3"}),
			relevance:  map[string]int{"dup1": 3, "dup2": 3, "dup3": 2, "opt2": 1},
		},
	}

	t.Logf("")
	t.Logf("╔══════════════════════════════════════════════════════════════════════════════╗")
	t.Logf("║                    RESULT QUALITY COMPARISON SUMMARY                        ║")
	t.Logf("╠══════════════════════════════════════════════════════════════════════════════╣")
	t.Logf("║ %-24s │ %-10s %-10s %-6s │ %-10s %-10s %-6s ║", "Query", "smd-Prec", "smd-Rec", "NDCG", "blv-Prec", "blv-Rec", "NDCG")
	t.Logf("╠══════════════════════════════════════════════════════════════════════════════╣")

	for _, tc := range cases {
		sHits := searchMDQuery(t, sIdx, tc.smdQuery, 10)
		bHits := bleveQuery(t, bIdx, tc.bleveQuery)

		sp := precision(sHits, tc.relevant)
		sr := recall(sHits, tc.relevant)
		bp := precision(bHits, tc.relevant)
		br := recall(bHits, tc.relevant)

		sn, bn := 0.0, 0.0
		sNDCGStr, bNDCGStr := "N/A", "N/A"
		if tc.relevance != nil {
			sn = ndcg(hitIDs(sHits), tc.relevance)
			bn = ndcg(hitIDs(bHits), tc.relevance)
			sNDCGStr = fmt.Sprintf("%.2f", sn)
			bNDCGStr = fmt.Sprintf("%.2f", bn)
		}

		t.Logf("║ %-24s │ %-10.2f %-10.2f %-6s │ %-10.2f %-10.2f %-6s ║",
			tc.name, sp, sr, sNDCGStr, bp, br, bNDCGStr)
	}
	t.Logf("╚══════════════════════════════════════════════════════════════════════════════╝")
}
