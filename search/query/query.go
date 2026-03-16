package query

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/readmedotmd/search.md/plugin"
	"github.com/readmedotmd/search.md/registry"
)

// Query is the interface all query types implement.
type Query interface {
	// Searcher returns a Searcher that can find matching documents.
	Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error)
}

// Searcher iterates over matching documents.
type Searcher interface {
	// Next returns the next matching document and its score.
	Next() (*plugin.DocumentScore, error)
	// Count returns the total number of matches (may be an estimate).
	Count() int
}

// TermQuery searches for an exact term in a field.
type TermQuery struct {
	Term  string  `json:"term"`
	Field string  `json:"field"`
	Boost float64 `json:"boost,omitempty"`
}

func NewTermQuery(term string) *TermQuery {
	return &TermQuery{Term: term, Boost: 1.0}
}

func (q *TermQuery) SetField(field string) *TermQuery {
	q.Field = field
	return q
}

func (q *TermQuery) SetBoost(boost float64) *TermQuery {
	q.Boost = boost
	return q
}

func (q *TermQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	f := q.Field
	if f == "" {
		f = field
	}
	return newTermSearcher(reader, f, q.Term, q.Boost, sf)
}

func (q *TermQuery) ExtractTerms() []string {
	return []string{q.Term}
}

// MatchQuery analyzes the input text and searches for the resulting terms.
type MatchQuery struct {
	Match    string  `json:"match"`
	Field    string  `json:"field"`
	Analyzer string  `json:"analyzer,omitempty"`
	Boost    float64 `json:"boost,omitempty"`
	Operator string  `json:"operator,omitempty"` // "or" (default) or "and"
}

func NewMatchQuery(match string) *MatchQuery {
	return &MatchQuery{Match: match, Boost: 1.0, Operator: "or"}
}

func (q *MatchQuery) SetField(field string) *MatchQuery {
	q.Field = field
	return q
}

func (q *MatchQuery) SetAnalyzer(analyzer string) *MatchQuery {
	q.Analyzer = analyzer
	return q
}

func (q *MatchQuery) SetOperator(op string) *MatchQuery {
	q.Operator = op
	return q
}

func (q *MatchQuery) SetBoost(boost float64) *MatchQuery {
	q.Boost = boost
	return q
}

func (q *MatchQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	f := q.Field
	if f == "" {
		f = field
	}

	analyzerName := q.Analyzer
	if analyzerName == "" {
		analyzerName = "standard"
	}

	analyzer, err := registry.AnalyzerByName(analyzerName)
	if err != nil {
		return nil, err
	}

	tokens := analyzer.Analyze([]byte(q.Match))
	if len(tokens) == 0 {
		return &emptySearcher{}, nil
	}

	if len(tokens) == 1 {
		return newTermSearcher(reader, f, tokens[0].Term, q.Boost, sf)
	}

	// Multiple terms: create sub-queries
	queries := make([]Query, len(tokens))
	for i, t := range tokens {
		queries[i] = &TermQuery{Term: t.Term, Field: f, Boost: q.Boost}
	}

	if q.Operator == "and" {
		conj := &ConjunctionQuery{Conjuncts: queries, Boost: 1.0}
		return conj.Searcher(reader, f, sf)
	}

	disj := &DisjunctionQuery{Disjuncts: queries, Min: 1, Boost: 1.0}
	return disj.Searcher(reader, f, sf)
}

func (q *MatchQuery) ExtractTerms() []string {
	return []string{q.Match}
}

// MatchPhraseQuery searches for an exact phrase.
type MatchPhraseQuery struct {
	MatchPhrase string  `json:"match_phrase"`
	Field       string  `json:"field"`
	Analyzer    string  `json:"analyzer,omitempty"`
	Boost       float64 `json:"boost,omitempty"`
}

func NewMatchPhraseQuery(phrase string) *MatchPhraseQuery {
	return &MatchPhraseQuery{MatchPhrase: phrase, Boost: 1.0}
}

func (q *MatchPhraseQuery) SetField(field string) *MatchPhraseQuery {
	q.Field = field
	return q
}

func (q *MatchPhraseQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	f := q.Field
	if f == "" {
		f = field
	}

	analyzerName := q.Analyzer
	if analyzerName == "" {
		analyzerName = "standard"
	}

	analyzer, err := registry.AnalyzerByName(analyzerName)
	if err != nil {
		return nil, err
	}

	tokens := analyzer.Analyze([]byte(q.MatchPhrase))
	if len(tokens) == 0 {
		return &emptySearcher{}, nil
	}

	terms := make([]string, len(tokens))
	for i, t := range tokens {
		terms[i] = t.Term
	}

	return newPhraseSearcher(reader, f, terms, q.Boost, sf)
}

func (q *MatchPhraseQuery) ExtractTerms() []string {
	return []string{q.MatchPhrase}
}

// PrefixQuery searches for terms starting with a prefix.
type PrefixQuery struct {
	Prefix string  `json:"prefix"`
	Field  string  `json:"field"`
	Boost  float64 `json:"boost,omitempty"`
}

func NewPrefixQuery(prefix string) *PrefixQuery {
	return &PrefixQuery{Prefix: prefix, Boost: 1.0}
}

func (q *PrefixQuery) SetField(field string) *PrefixQuery {
	q.Field = field
	return q
}

func (q *PrefixQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	f := q.Field
	if f == "" {
		f = field
	}
	return newPrefixSearcher(reader, f, strings.ToLower(q.Prefix), q.Boost, sf)
}

func (q *PrefixQuery) ExtractTerms() []string {
	return []string{q.Prefix}
}

// FuzzyQuery searches for terms similar to the given term.
type FuzzyQuery struct {
	Term      string  `json:"term"`
	Field     string  `json:"field"`
	Fuzziness int     `json:"fuzziness"`
	Boost     float64 `json:"boost,omitempty"`
}

func NewFuzzyQuery(term string) *FuzzyQuery {
	return &FuzzyQuery{Term: term, Fuzziness: 1, Boost: 1.0}
}

func (q *FuzzyQuery) SetField(field string) *FuzzyQuery {
	q.Field = field
	return q
}

func (q *FuzzyQuery) SetFuzziness(f int) *FuzzyQuery {
	q.Fuzziness = f
	return q
}

func (q *FuzzyQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	f := q.Field
	if f == "" {
		f = field
	}
	// Analyze the term through the standard analyzer so that it is
	// stemmed/lowercased to match the indexed term forms. Edit distance
	// is then computed against the stemmed dictionary.
	term := strings.ToLower(q.Term)
	if analyzer, err := registry.AnalyzerByName("standard"); err == nil {
		tokens := analyzer.Analyze([]byte(q.Term))
		if len(tokens) > 0 {
			term = tokens[0].Term
		}
	}
	return newFuzzySearcher(reader, f, term, q.Fuzziness, q.Boost, sf)
}

func (q *FuzzyQuery) ExtractTerms() []string {
	return []string{q.Term}
}

// WildcardQuery searches using a wildcard pattern (* and ?).
type WildcardQuery struct {
	Wildcard string  `json:"wildcard"`
	Field    string  `json:"field"`
	Boost    float64 `json:"boost,omitempty"`
}

func NewWildcardQuery(wildcard string) *WildcardQuery {
	return &WildcardQuery{Wildcard: wildcard, Boost: 1.0}
}

func (q *WildcardQuery) SetField(field string) *WildcardQuery {
	q.Field = field
	return q
}

func (q *WildcardQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	f := q.Field
	if f == "" {
		f = field
	}
	// Convert wildcard to regex
	pattern := wildcardToRegex(strings.ToLower(q.Wildcard))
	return newRegexpSearcher(reader, f, pattern, q.Boost, sf)
}

// RegexpQuery searches using a regular expression.
type RegexpQuery struct {
	Regexp string  `json:"regexp"`
	Field  string  `json:"field"`
	Boost  float64 `json:"boost,omitempty"`
}

func NewRegexpQuery(re string) *RegexpQuery {
	return &RegexpQuery{Regexp: re, Boost: 1.0}
}

func (q *RegexpQuery) SetField(field string) *RegexpQuery {
	q.Field = field
	return q
}

func (q *RegexpQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	f := q.Field
	if f == "" {
		f = field
	}
	_, err := regexp.Compile(q.Regexp)
	if err != nil {
		return nil, fmt.Errorf("invalid regexp: %w", err)
	}
	return newRegexpSearcher(reader, f, q.Regexp, q.Boost, sf)
}

// NumericRangeQuery searches for documents with numeric values in a range.
type NumericRangeQuery struct {
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Field string   `json:"field"`
	Boost float64  `json:"boost,omitempty"`
}

func NewNumericRangeQuery(min, max *float64) *NumericRangeQuery {
	return &NumericRangeQuery{Min: min, Max: max, Boost: 1.0}
}

func (q *NumericRangeQuery) SetField(field string) *NumericRangeQuery {
	q.Field = field
	return q
}

func (q *NumericRangeQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	f := q.Field
	if f == "" {
		f = field
	}
	return newNumericRangeSearcher(reader, f, q.Min, q.Max, q.Boost)
}

// DateRangeQuery searches for documents with datetime values in a range.
type DateRangeQuery struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
	Field string     `json:"field"`
	Boost float64    `json:"boost,omitempty"`
}

func NewDateRangeQuery(start, end *time.Time) *DateRangeQuery {
	return &DateRangeQuery{Start: start, End: end, Boost: 1.0}
}

func (q *DateRangeQuery) SetField(field string) *DateRangeQuery {
	q.Field = field
	return q
}

func (q *DateRangeQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	f := q.Field
	if f == "" {
		f = field
	}
	return newDateRangeSearcher(reader, f, q.Start, q.End, q.Boost)
}

// BooleanQuery combines must, should, and must_not queries.
type BooleanQuery struct {
	Must    []Query `json:"must,omitempty"`
	Should  []Query `json:"should,omitempty"`
	MustNot []Query `json:"must_not,omitempty"`
	Boost   float64 `json:"boost,omitempty"`
}

func NewBooleanQuery() *BooleanQuery {
	return &BooleanQuery{Boost: 1.0}
}

func (q *BooleanQuery) AddMust(queries ...Query) {
	q.Must = append(q.Must, queries...)
}

func (q *BooleanQuery) AddShould(queries ...Query) {
	q.Should = append(q.Should, queries...)
}

func (q *BooleanQuery) AddMustNot(queries ...Query) {
	q.MustNot = append(q.MustNot, queries...)
}

func (q *BooleanQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	return newBooleanSearcher(reader, field, q, sf)
}

func (q *BooleanQuery) ExtractTerms() []string {
	var terms []string
	for _, mq := range q.Must {
		if ext, ok := mq.(plugin.QueryTermExtractor); ok {
			terms = append(terms, ext.ExtractTerms()...)
		}
	}
	for _, sq := range q.Should {
		if ext, ok := sq.(plugin.QueryTermExtractor); ok {
			terms = append(terms, ext.ExtractTerms()...)
		}
	}
	return terms
}

// ConjunctionQuery requires all sub-queries to match (AND).
type ConjunctionQuery struct {
	Conjuncts []Query `json:"conjuncts"`
	Boost     float64 `json:"boost,omitempty"`
}

func NewConjunctionQuery(queries ...Query) *ConjunctionQuery {
	return &ConjunctionQuery{Conjuncts: queries, Boost: 1.0}
}

func (q *ConjunctionQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	return newConjunctionSearcher(reader, field, q.Conjuncts, q.Boost, sf)
}

func (q *ConjunctionQuery) ExtractTerms() []string {
	var terms []string
	for _, cq := range q.Conjuncts {
		if ext, ok := cq.(plugin.QueryTermExtractor); ok {
			terms = append(terms, ext.ExtractTerms()...)
		}
	}
	return terms
}

// DisjunctionQuery requires at least Min sub-queries to match (OR).
type DisjunctionQuery struct {
	Disjuncts []Query `json:"disjuncts"`
	Min       int     `json:"min"`
	Boost     float64 `json:"boost,omitempty"`
}

func NewDisjunctionQuery(queries ...Query) *DisjunctionQuery {
	return &DisjunctionQuery{Disjuncts: queries, Min: 1, Boost: 1.0}
}

func (q *DisjunctionQuery) SetMin(min int) *DisjunctionQuery {
	q.Min = min
	return q
}

func (q *DisjunctionQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	return newDisjunctionSearcher(reader, field, q.Disjuncts, q.Min, q.Boost, sf)
}

func (q *DisjunctionQuery) ExtractTerms() []string {
	var terms []string
	for _, dq := range q.Disjuncts {
		if ext, ok := dq.(plugin.QueryTermExtractor); ok {
			terms = append(terms, ext.ExtractTerms()...)
		}
	}
	return terms
}

// MatchAllQuery matches all documents.
type MatchAllQuery struct {
	Boost float64 `json:"boost,omitempty"`
}

func NewMatchAllQuery() *MatchAllQuery {
	return &MatchAllQuery{Boost: 1.0}
}

func (q *MatchAllQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	return newMatchAllSearcher(reader, q.Boost)
}

// MatchNoneQuery matches no documents.
type MatchNoneQuery struct{}

func NewMatchNoneQuery() *MatchNoneQuery {
	return &MatchNoneQuery{}
}

func (q *MatchNoneQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	return &emptySearcher{}, nil
}

// KNNQuery performs k-nearest-neighbor vector search.
type KNNQuery struct {
	Vector []float32 `json:"vector"`
	Field  string    `json:"field"`
	K      int       `json:"k"`
	Boost  float64   `json:"boost,omitempty"`
}

func NewKNNQuery(vector []float32, k int) *KNNQuery {
	return &KNNQuery{Vector: vector, K: k, Boost: 1.0}
}

func (q *KNNQuery) SetField(field string) *KNNQuery {
	q.Field = field
	return q
}

func (q *KNNQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (Searcher, error) {
	f := q.Field
	if f == "" {
		f = field
	}
	return newKNNSearcher(reader, f, q.Vector, q.K, q.Boost)
}

// wildcardToRegex converts a wildcard pattern to a regex.
func wildcardToRegex(pattern string) string {
	var result strings.Builder
	result.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteString(".")
		case '.', '(', ')', '[', ']', '{', '}', '+', '^', '$', '|', '\\':
			result.WriteRune('\\')
			result.WriteRune(r)
		default:
			result.WriteRune(r)
		}
	}
	result.WriteString("$")
	return result.String()
}
