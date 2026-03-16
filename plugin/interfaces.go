// Package plugin defines the core interfaces for the search.md plugin system.
//
// Every major component of the search engine is defined as an interface here,
// allowing third-party plugins to extend or replace default behavior.
package plugin

import (
	"time"

	"github.com/readmedotmd/search.md/index"
	"github.com/readmedotmd/search.md/search"
)

// IndexReader provides read-only access to the index.
// Query and Searcher implementations depend on this interface
// instead of the concrete *index.Index, enabling decoupling and testing.
type IndexReader interface {
	// Document retrieval
	DocCount() (uint64, error)
	GetDocument(docID string) (map[string]interface{}, error)

	// Term index
	TermPostings(field, term string) ([]index.Posting, error)
	TermsForField(field string) ([]string, error)
	TermsWithPrefix(field, termPrefix string) ([]string, error)
	DocumentFrequency(field, term string) (uint64, error)
	FieldLength(field, docID string) (int, error)
	AverageFieldLength(field string) (float64, error)

	// Term vectors
	GetTermVector(field, docID, term string) (*index.TermVector, error)

	// Vectors
	GetVector(field, docID string) ([]float32, error)
	ForEachVector(field string, fn func(docID string, vec []float32) bool) error

	// Doc ID iteration
	ForEachDocID(fn func(docID string) bool) error
	ListDocIDs(startAfter string, limit int) ([]string, error)

	// Typed field access
	NumericRange(field string, min, max *float64) ([]string, error)
	DateTimeRange(field string, start, end *time.Time) ([]string, error)
	GetNumericValue(field, docID string) (float64, bool)
	GetDateTimeValue(field, docID string) (time.Time, bool)

	// Optimized search methods
	FuzzyTerms(field, term string, maxDist int) ([]string, error)
	HNSWSearch(field string, query []float32, k int) ([]string, []float64, bool)
}

// Scorer computes relevance scores for matched documents.
type Scorer interface {
	// Score computes a relevance score given term statistics.
	Score(termFreq int, fieldLength int, boost float64) float64
}

// ScorerFactory creates a Scorer given index-level statistics for a term.
type ScorerFactory interface {
	Name() string
	NewScorer(docCount, docFreq uint64, avgFieldLength float64) Scorer
}

// DocumentScore represents a scored document from a searcher.
type DocumentScore struct {
	ID    string
	Score float64
}

// Searcher iterates over matching documents.
type Searcher interface {
	Next() (*DocumentScore, error)
	Count() int
}

// QueryPlugin allows registering new query types that can be constructed by name.
type QueryPlugin interface {
	Name() string
	ParseQuery(params map[string]interface{}) (interface{}, error)
}

// Highlighter produces highlighted fragments showing where matches occurred.
type Highlighter interface {
	Highlight(reader IndexReader, docID, field, text, analyzerName string, queryTerms []string) []string
}

// HighlighterFactory creates a Highlighter.
type HighlighterFactory interface {
	Name() string
	NewHighlighter() Highlighter
}

// FacetBuilder computes facet results for matched documents.
type FacetBuilder interface {
	Build(docIDs []string, facets map[string]*search.FacetRequest) map[string]*search.FacetResult
}

// FacetBuilderFactory creates a FacetBuilder.
type FacetBuilderFactory interface {
	Name() string
	NewFacetBuilder(reader IndexReader) FacetBuilder
}

// QueryTermExtractor extracts search terms from a query for highlighting.
// Queries that contain searchable terms should implement this interface.
type QueryTermExtractor interface {
	ExtractTerms() []string
}
