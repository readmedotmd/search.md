package query

import (
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	storemd "github.com/readmedotmd/store.md"

	"github.com/readmedotmd/search.md/document"
	"github.com/readmedotmd/search.md/index"
	"github.com/readmedotmd/search.md/plugin"
)

// memStore is an in-memory implementation of storemd.Store for testing.
type memStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func newMemStore() *memStore {
	return &memStore{data: make(map[string]string)}
}

func (m *memStore) Get(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return "", storemd.NotFoundError
	}
	return v, nil
}

func (m *memStore) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *memStore) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *memStore) List(args storemd.ListArgs) ([]storemd.KeyValuePair, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []string
	for k := range m.data {
		if args.Prefix != "" && !strings.HasPrefix(k, args.Prefix) {
			continue
		}
		if args.StartAfter != "" && k <= args.StartAfter {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var result []storemd.KeyValuePair
	for _, k := range keys {
		result = append(result, storemd.KeyValuePair{Key: k, Value: m.data[k]})
		if args.Limit > 0 && len(result) >= args.Limit {
			break
		}
	}
	return result, nil
}

// defaultSF returns the default BM25 scorer factory for tests.
func defaultSF() plugin.ScorerFactory {
	return plugin.DefaultBM25()
}

// readerFor wraps an *index.Index into a plugin.IndexReader.
func readerFor(idx *index.Index) plugin.IndexReader {
	return &plugin.IndexAdapter{Idx: idx}
}

// ---------------------------------------------------------------------------
// wildcardToRegex tests
// ---------------------------------------------------------------------------

func TestWildcardToRegex_Star(t *testing.T) {
	got := wildcardToRegex("foo*")
	if got != "^foo.*$" {
		t.Errorf("wildcardToRegex(\"foo*\") = %q, want %q", got, "^foo.*$")
	}
}

func TestWildcardToRegex_Question(t *testing.T) {
	got := wildcardToRegex("fo?")
	if got != "^fo.$" {
		t.Errorf("wildcardToRegex(\"fo?\") = %q, want %q", got, "^fo.$")
	}
}

func TestWildcardToRegex_Mixed(t *testing.T) {
	got := wildcardToRegex("f*o?")
	if got != "^f.*o.$" {
		t.Errorf("wildcardToRegex(\"f*o?\") = %q, want %q", got, "^f.*o.$")
	}
}

func TestWildcardToRegex_SpecialChars(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"a.b", `^a\.b$`},
		{"a(b)", `^a\(b\)$`},
		{"a[b]", `^a\[b\]$`},
		{"a{b}", `^a\{b\}$`},
		{"a+b", `^a\+b$`},
		{"a^b", `^a\^b$`},
		{"a$b", `^a\$b$`},
		{"a|b", `^a\|b$`},
		{`a\b`, `^a\\b$`},
	}
	for _, tt := range tests {
		got := wildcardToRegex(tt.input)
		if got != tt.want {
			t.Errorf("wildcardToRegex(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWildcardToRegex_NoWildcards(t *testing.T) {
	got := wildcardToRegex("hello")
	if got != "^hello$" {
		t.Errorf("wildcardToRegex(\"hello\") = %q, want %q", got, "^hello$")
	}
}

func TestWildcardToRegex_Empty(t *testing.T) {
	got := wildcardToRegex("")
	if got != "^$" {
		t.Errorf("wildcardToRegex(\"\") = %q, want %q", got, "^$")
	}
}

func TestWildcardToRegex_AllStar(t *testing.T) {
	got := wildcardToRegex("*")
	if got != "^.*$" {
		t.Errorf("wildcardToRegex(\"*\") = %q, want %q", got, "^.*$")
	}
}

// ---------------------------------------------------------------------------
// levenshteinDistance tests
// ---------------------------------------------------------------------------

func TestLevenshteinDistance_Identical(t *testing.T) {
	if d := levenshteinDistance("hello", "hello"); d != 0 {
		t.Errorf("identical strings: got %d, want 0", d)
	}
}

func TestLevenshteinDistance_SingleEdit(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"cat", "bat", 1},  // substitution
		{"cat", "cats", 1}, // insertion
		{"cats", "cat", 1}, // deletion
	}
	for _, c := range cases {
		if d := levenshteinDistance(c.a, c.b); d != c.want {
			t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", c.a, c.b, d, c.want)
		}
	}
}

func TestLevenshteinDistance_MultipleEdits(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"kitten", "sitting", 3},
		{"sunday", "saturday", 3},
	}
	for _, c := range cases {
		if d := levenshteinDistance(c.a, c.b); d != c.want {
			t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", c.a, c.b, d, c.want)
		}
	}
}

func TestLevenshteinDistance_EmptyStrings(t *testing.T) {
	if d := levenshteinDistance("", ""); d != 0 {
		t.Errorf("both empty: got %d, want 0", d)
	}
	if d := levenshteinDistance("abc", ""); d != 3 {
		t.Errorf("second empty: got %d, want 3", d)
	}
	if d := levenshteinDistance("", "abc"); d != 3 {
		t.Errorf("first empty: got %d, want 3", d)
	}
}

func TestLevenshteinDistance_DifferentLengths(t *testing.T) {
	if d := levenshteinDistance("a", "abcdef"); d != 5 {
		t.Errorf("got %d, want 5", d)
	}
	if d := levenshteinDistance("abcdef", "a"); d != 5 {
		t.Errorf("got %d, want 5", d)
	}
}

// ---------------------------------------------------------------------------
// Query constructor and setter tests
// ---------------------------------------------------------------------------

func TestNewTermQuery(t *testing.T) {
	q := NewTermQuery("hello")
	if q.Term != "hello" {
		t.Errorf("Term = %q, want %q", q.Term, "hello")
	}
	if q.Boost != 1.0 {
		t.Errorf("Boost = %f, want 1.0", q.Boost)
	}
	if q.Field != "" {
		t.Errorf("Field = %q, want empty", q.Field)
	}
	q.SetField("title").SetBoost(2.5)
	if q.Field != "title" {
		t.Errorf("after SetField: Field = %q, want %q", q.Field, "title")
	}
	if q.Boost != 2.5 {
		t.Errorf("after SetBoost: Boost = %f, want 2.5", q.Boost)
	}
}

func TestNewMatchQuery(t *testing.T) {
	q := NewMatchQuery("hello world")
	if q.Match != "hello world" {
		t.Errorf("Match = %q", q.Match)
	}
	if q.Boost != 1.0 {
		t.Errorf("Boost = %f", q.Boost)
	}
	if q.Operator != "or" {
		t.Errorf("Operator = %q, want %q", q.Operator, "or")
	}
	q.SetField("body").SetAnalyzer("simple").SetOperator("and").SetBoost(3.0)
	if q.Field != "body" || q.Analyzer != "simple" || q.Operator != "and" || q.Boost != 3.0 {
		t.Errorf("setters failed: %+v", q)
	}
}

func TestNewMatchPhraseQuery(t *testing.T) {
	q := NewMatchPhraseQuery("quick fox")
	if q.MatchPhrase != "quick fox" {
		t.Errorf("MatchPhrase = %q", q.MatchPhrase)
	}
	if q.Boost != 1.0 {
		t.Errorf("Boost = %f", q.Boost)
	}
	q.SetField("content")
	if q.Field != "content" {
		t.Errorf("Field = %q", q.Field)
	}
}

func TestNewPrefixQuery(t *testing.T) {
	q := NewPrefixQuery("hel")
	if q.Prefix != "hel" {
		t.Errorf("Prefix = %q", q.Prefix)
	}
	if q.Boost != 1.0 {
		t.Errorf("Boost = %f", q.Boost)
	}
	q.SetField("title")
	if q.Field != "title" {
		t.Errorf("Field = %q", q.Field)
	}
}

func TestNewFuzzyQuery(t *testing.T) {
	q := NewFuzzyQuery("helo")
	if q.Term != "helo" {
		t.Errorf("Term = %q", q.Term)
	}
	if q.Fuzziness != 1 {
		t.Errorf("Fuzziness = %d, want 1", q.Fuzziness)
	}
	if q.Boost != 1.0 {
		t.Errorf("Boost = %f", q.Boost)
	}
	q.SetField("body").SetFuzziness(2)
	if q.Fuzziness != 2 || q.Field != "body" {
		t.Errorf("setters failed: %+v", q)
	}
}

func TestNewWildcardQuery(t *testing.T) {
	q := NewWildcardQuery("hel*")
	if q.Wildcard != "hel*" {
		t.Errorf("Wildcard = %q", q.Wildcard)
	}
	q.SetField("title")
	if q.Field != "title" {
		t.Errorf("Field = %q", q.Field)
	}
}

func TestNewRegexpQuery(t *testing.T) {
	q := NewRegexpQuery("hel[lo]+")
	if q.Regexp != "hel[lo]+" {
		t.Errorf("Regexp = %q", q.Regexp)
	}
	q.SetField("title")
	if q.Field != "title" {
		t.Errorf("Field = %q", q.Field)
	}
}

func TestNewNumericRangeQuery(t *testing.T) {
	lo, hi := 1.0, 10.0
	q := NewNumericRangeQuery(&lo, &hi)
	if *q.Min != 1.0 || *q.Max != 10.0 {
		t.Errorf("Min/Max unexpected: %+v", q)
	}
	q.SetField("price")
	if q.Field != "price" {
		t.Errorf("Field = %q", q.Field)
	}
}

func TestNewDateRangeQuery(t *testing.T) {
	s := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	e := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	q := NewDateRangeQuery(&s, &e)
	if !q.Start.Equal(s) || !q.End.Equal(e) {
		t.Errorf("Start/End unexpected: %+v", q)
	}
	q.SetField("created_at")
	if q.Field != "created_at" {
		t.Errorf("Field = %q", q.Field)
	}
}

func TestNewBooleanQuery(t *testing.T) {
	q := NewBooleanQuery()
	if q.Boost != 1.0 {
		t.Errorf("Boost = %f", q.Boost)
	}
	q.AddMust(NewTermQuery("a"))
	q.AddShould(NewTermQuery("b"))
	q.AddMustNot(NewTermQuery("c"))
	if len(q.Must) != 1 || len(q.Should) != 1 || len(q.MustNot) != 1 {
		t.Errorf("Must/Should/MustNot lengths: %d/%d/%d", len(q.Must), len(q.Should), len(q.MustNot))
	}
}

func TestNewConjunctionQuery(t *testing.T) {
	a := NewTermQuery("a")
	b := NewTermQuery("b")
	q := NewConjunctionQuery(a, b)
	if len(q.Conjuncts) != 2 {
		t.Errorf("Conjuncts length = %d, want 2", len(q.Conjuncts))
	}
	if q.Boost != 1.0 {
		t.Errorf("Boost = %f", q.Boost)
	}
}

func TestNewDisjunctionQuery(t *testing.T) {
	a := NewTermQuery("a")
	b := NewTermQuery("b")
	q := NewDisjunctionQuery(a, b)
	if len(q.Disjuncts) != 2 {
		t.Errorf("Disjuncts length = %d, want 2", len(q.Disjuncts))
	}
	if q.Min != 1 {
		t.Errorf("Min = %d, want 1", q.Min)
	}
	q.SetMin(2)
	if q.Min != 2 {
		t.Errorf("after SetMin: Min = %d, want 2", q.Min)
	}
}

func TestNewMatchAllQuery(t *testing.T) {
	q := NewMatchAllQuery()
	if q.Boost != 1.0 {
		t.Errorf("Boost = %f", q.Boost)
	}
}

func TestNewMatchNoneQuery(t *testing.T) {
	q := NewMatchNoneQuery()
	if q == nil {
		t.Fatal("nil MatchNoneQuery")
	}
}

func TestNewKNNQuery(t *testing.T) {
	vec := []float32{1.0, 2.0, 3.0}
	q := NewKNNQuery(vec, 5)
	if q.K != 5 {
		t.Errorf("K = %d, want 5", q.K)
	}
	if len(q.Vector) != 3 {
		t.Errorf("Vector length = %d, want 3", len(q.Vector))
	}
	if q.Boost != 1.0 {
		t.Errorf("Boost = %f", q.Boost)
	}
	q.SetField("embedding")
	if q.Field != "embedding" {
		t.Errorf("Field = %q", q.Field)
	}
}

// ---------------------------------------------------------------------------
// Helper to build a small test index with documents
// ---------------------------------------------------------------------------

func buildTestIndex(t *testing.T) (*index.Index, plugin.IndexReader) {
	t.Helper()
	store := newMemStore()
	idx, err := index.New(store)
	if err != nil {
		t.Fatal(err)
	}

	docs := []*document.Document{
		{
			ID: "doc1",
			Fields: []*document.Field{
				{Name: "title", Type: document.FieldTypeText, Value: "The quick brown fox", Index: true, Store: true, IncludeTermVectors: true},
				{Name: "body", Type: document.FieldTypeText, Value: "A quick brown fox jumps over the lazy dog", Index: true, Store: true, IncludeTermVectors: true},
			},
		},
		{
			ID: "doc2",
			Fields: []*document.Field{
				{Name: "title", Type: document.FieldTypeText, Value: "Lazy dog sleeps", Index: true, Store: true, IncludeTermVectors: true},
				{Name: "body", Type: document.FieldTypeText, Value: "The lazy dog sleeps all day long", Index: true, Store: true, IncludeTermVectors: true},
			},
		},
		{
			ID: "doc3",
			Fields: []*document.Field{
				{Name: "title", Type: document.FieldTypeText, Value: "Fox and hound", Index: true, Store: true, IncludeTermVectors: true},
				{Name: "body", Type: document.FieldTypeText, Value: "The fox and the hound are friends", Index: true, Store: true, IncludeTermVectors: true},
			},
		},
	}

	for _, doc := range docs {
		if err := idx.IndexDocument(doc); err != nil {
			t.Fatalf("indexing %s: %v", doc.ID, err)
		}
	}
	return idx, readerFor(idx)
}

// ---------------------------------------------------------------------------
// Searcher integration tests
// ---------------------------------------------------------------------------

func TestTermQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewTermQuery("fox").SetField("title")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	count := 0
	for {
		ds, err := s.Next()
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if ds == nil {
			break
		}
		count++
		if ds.Score <= 0 {
			t.Errorf("expected positive score, got %f", ds.Score)
		}
	}
	if count == 0 {
		t.Error("expected at least one result for term 'fox'")
	}
}

func TestTermQuery_Searcher_DefaultField(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewTermQuery("fox")
	s, err := q.Searcher(reader, "title", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() == 0 {
		t.Error("expected results when field passed as arg")
	}
}

func TestMatchQuery_Searcher_SingleTerm(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewMatchQuery("fox").SetField("body")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() == 0 {
		t.Error("expected at least one result")
	}
}

func TestMatchQuery_Searcher_MultipleTerms_Or(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewMatchQuery("fox dog").SetField("body")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() == 0 {
		t.Error("expected results for 'fox dog' with OR")
	}
}

func TestMatchQuery_Searcher_MultipleTerms_And(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewMatchQuery("fox dog").SetField("body").SetOperator("and")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	count := 0
	for {
		ds, _ := s.Next()
		if ds == nil {
			break
		}
		count++
	}
	if count == 0 {
		t.Error("expected at least one result for AND query")
	}
}

func TestMatchQuery_Searcher_EmptyTokens(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewMatchQuery("the").SetField("body")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	_ = s
}

func TestMatchQuery_Searcher_BadAnalyzer(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewMatchQuery("fox").SetField("body").SetAnalyzer("nonexistent_analyzer")
	_, err := q.Searcher(reader, "", sf)
	if err == nil {
		t.Error("expected error for unknown analyzer")
	}
}

func TestMatchPhraseQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewMatchPhraseQuery("quick brown fox").SetField("body")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	_ = s
}

func TestPrefixQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewPrefixQuery("fox").SetField("title")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() == 0 {
		t.Error("expected at least one result for prefix 'fox'")
	}
}

func TestFuzzyQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewFuzzyQuery("foz").SetField("title").SetFuzziness(1)
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() == 0 {
		t.Error("expected at least one fuzzy match for 'foz'")
	}
}

func TestWildcardQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewWildcardQuery("fo*").SetField("title")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() == 0 {
		t.Error("expected at least one result for wildcard 'fo*'")
	}
}

func TestRegexpQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewRegexpQuery("fo.").SetField("title")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() == 0 {
		t.Error("expected at least one result for regexp 'fo.'")
	}
}

func TestRegexpQuery_Searcher_InvalidRegexp(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewRegexpQuery("[invalid").SetField("title")
	_, err := q.Searcher(reader, "", sf)
	if err == nil {
		t.Error("expected error for invalid regexp")
	}
}

func TestMatchAllQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewMatchAllQuery()
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() != 3 {
		t.Errorf("MatchAll count = %d, want 3", s.Count())
	}
}

func TestMatchNoneQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewMatchNoneQuery()
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() != 0 {
		t.Errorf("MatchNone count = %d, want 0", s.Count())
	}
	ds, err := s.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if ds != nil {
		t.Error("expected nil from MatchNone Next()")
	}
}

func TestConjunctionQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewConjunctionQuery(
		NewTermQuery("fox").SetField("body"),
		NewTermQuery("dog").SetField("body"),
	)
	s, err := q.Searcher(reader, "body", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	count := 0
	for {
		ds, _ := s.Next()
		if ds == nil {
			break
		}
		count++
	}
	if count == 0 {
		t.Error("expected at least one conjunction result")
	}
}

func TestDisjunctionQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewDisjunctionQuery(
		NewTermQuery("fox").SetField("body"),
		NewTermQuery("sleep").SetField("body"),
	)
	s, err := q.Searcher(reader, "body", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() == 0 {
		t.Error("expected at least one disjunction result")
	}
}

func TestDisjunctionQuery_Searcher_MinMatch(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewDisjunctionQuery(
		NewTermQuery("fox").SetField("body"),
		NewTermQuery("hound").SetField("body"),
		NewTermQuery("dog").SetField("body"),
	).SetMin(2)
	s, err := q.Searcher(reader, "body", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	_ = s
}

func TestBooleanQuery_Searcher(t *testing.T) {
	_, reader := buildTestIndex(t)
	sf := defaultSF()
	q := NewBooleanQuery()
	q.AddMust(NewTermQuery("fox").SetField("body"))
	q.AddMustNot(NewTermQuery("dog").SetField("body"))
	s, err := q.Searcher(reader, "body", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	for {
		ds, _ := s.Next()
		if ds == nil {
			break
		}
		if ds.ID == "doc1" {
			t.Error("doc1 should be excluded by must_not dog")
		}
	}
}

func TestNumericRangeQuery_Searcher(t *testing.T) {
	store := newMemStore()
	idx, err := index.New(store)
	if err != nil {
		t.Fatal(err)
	}
	reader := readerFor(idx)
	sf := defaultSF()

	doc := &document.Document{
		ID: "n1",
		Fields: []*document.Field{
			{Name: "price", Type: document.FieldTypeNumeric, Value: 25.0, Index: true, Store: true},
		},
	}
	if err := idx.IndexDocument(doc); err != nil {
		t.Fatal(err)
	}
	doc2 := &document.Document{
		ID: "n2",
		Fields: []*document.Field{
			{Name: "price", Type: document.FieldTypeNumeric, Value: 50.0, Index: true, Store: true},
		},
	}
	if err := idx.IndexDocument(doc2); err != nil {
		t.Fatal(err)
	}

	lo, hi := 20.0, 30.0
	q := NewNumericRangeQuery(&lo, &hi).SetField("price")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() != 1 {
		t.Errorf("NumericRange count = %d, want 1", s.Count())
	}
}

func TestDateRangeQuery_Searcher(t *testing.T) {
	store := newMemStore()
	idx, err := index.New(store)
	if err != nil {
		t.Fatal(err)
	}
	reader := readerFor(idx)
	sf := defaultSF()

	t1 := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)

	doc := &document.Document{
		ID: "d1",
		Fields: []*document.Field{
			{Name: "created", Type: document.FieldTypeDateTime, Value: t1, Index: true, Store: true},
		},
	}
	if err := idx.IndexDocument(doc); err != nil {
		t.Fatal(err)
	}
	doc2 := &document.Document{
		ID: "d2",
		Fields: []*document.Field{
			{Name: "created", Type: document.FieldTypeDateTime, Value: t2, Index: true, Store: true},
		},
	}
	if err := idx.IndexDocument(doc2); err != nil {
		t.Fatal(err)
	}

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	q := NewDateRangeQuery(&start, &end).SetField("created")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() != 1 {
		t.Errorf("DateRange count = %d, want 1", s.Count())
	}
}

func TestKNNQuery_Searcher(t *testing.T) {
	store := newMemStore()
	idx, err := index.New(store)
	if err != nil {
		t.Fatal(err)
	}
	reader := readerFor(idx)
	sf := defaultSF()

	docs := []*document.Document{
		{
			ID: "v1",
			Fields: []*document.Field{
				{Name: "embed", Type: document.FieldTypeVector, Value: []float32{1.0, 0.0, 0.0}, Index: true, Dims: 3},
			},
		},
		{
			ID: "v2",
			Fields: []*document.Field{
				{Name: "embed", Type: document.FieldTypeVector, Value: []float32{0.0, 1.0, 0.0}, Index: true, Dims: 3},
			},
		},
		{
			ID: "v3",
			Fields: []*document.Field{
				{Name: "embed", Type: document.FieldTypeVector, Value: []float32{0.9, 0.1, 0.0}, Index: true, Dims: 3},
			},
		},
	}
	for _, d := range docs {
		if err := idx.IndexDocument(d); err != nil {
			t.Fatal(err)
		}
	}

	queryVec := []float32{1.0, 0.0, 0.0}
	q := NewKNNQuery(queryVec, 2).SetField("embed")
	s, err := q.Searcher(reader, "", sf)
	if err != nil {
		t.Fatalf("Searcher error: %v", err)
	}
	if s.Count() != 2 {
		t.Errorf("KNN count = %d, want 2", s.Count())
	}
	ds, _ := s.Next()
	if ds == nil {
		t.Fatal("expected first result")
	}
	if ds.ID != "v1" {
		t.Errorf("first result ID = %q, want %q", ds.ID, "v1")
	}
}

func TestEmptySearcher(t *testing.T) {
	s := &emptySearcher{}
	if s.Count() != 0 {
		t.Errorf("Count = %d, want 0", s.Count())
	}
	ds, err := s.Next()
	if err != nil || ds != nil {
		t.Error("expected nil, nil from empty searcher")
	}
}
