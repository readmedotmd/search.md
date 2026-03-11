package search

import (
	"time"
)

// DocumentMatch represents a document that matched a search query.
type DocumentMatch struct {
	ID        string                 `json:"id"`
	Score     float64                `json:"score"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Fragments map[string][]string    `json:"fragments,omitempty"`
}

// SearchRequest describes a search operation.
type SearchRequest struct {
	Query     interface{}              `json:"query"` // a Query interface
	Size      int                      `json:"size"`
	From      int                      `json:"from"`
	Fields    []string                 `json:"fields,omitempty"`
	Highlight *HighlightRequest        `json:"highlight,omitempty"`
	Facets    map[string]*FacetRequest `json:"facets,omitempty"`
}

// NewSearchRequest creates a SearchRequest with defaults.
func NewSearchRequest(query interface{}) *SearchRequest {
	return &SearchRequest{
		Query: query,
		Size:  10,
		From:  0,
	}
}

// NewSearchRequestOptions creates a SearchRequest with custom size and offset.
func NewSearchRequestOptions(query interface{}, size, from int) *SearchRequest {
	return &SearchRequest{
		Query: query,
		Size:  size,
		From:  from,
	}
}

// SearchResult contains the results of a search.
type SearchResult struct {
	Status   SearchStatus            `json:"status"`
	Hits     []*DocumentMatch        `json:"hits"`
	Total    uint64                  `json:"total_hits"`
	MaxScore float64                 `json:"max_score"`
	Took     time.Duration           `json:"took"`
	Facets   map[string]*FacetResult `json:"facets,omitempty"`
	Request  *SearchRequest          `json:"request,omitempty"`
}

// SearchStatus reports the status of a search.
type SearchStatus struct {
	Total      int `json:"total"`
	Failed     int `json:"failed"`
	Successful int `json:"successful"`
}

// HighlightRequest describes how to highlight results.
type HighlightRequest struct {
	Style  string   `json:"style,omitempty"`
	Fields []string `json:"fields,omitempty"`
}

// FacetRequest describes a facet to compute.
type FacetRequest struct {
	Size           int             `json:"size"`
	Field          string          `json:"field"`
	NumericRanges  []NumericRange  `json:"numeric_ranges,omitempty"`
	DateTimeRanges []DateTimeRange `json:"date_ranges,omitempty"`
}

// NumericRange defines a named numeric range for faceting.
type NumericRange struct {
	Name string   `json:"name"`
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
}

// DateTimeRange defines a named date range for faceting.
type DateTimeRange struct {
	Name  string     `json:"name"`
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

// FacetResult contains the results of a facet computation.
type FacetResult struct {
	Field         string              `json:"field"`
	Total         int                 `json:"total"`
	Missing       int                 `json:"missing"`
	Other         int                 `json:"other"`
	Terms         []TermFacet         `json:"terms,omitempty"`
	NumericRanges []NumericRangeFacet `json:"numeric_ranges,omitempty"`
	DateRanges    []DateRangeFacet    `json:"date_ranges,omitempty"`
}

// TermFacet is a single term facet result.
type TermFacet struct {
	Term  string `json:"term"`
	Count int    `json:"count"`
}

// NumericRangeFacet is a numeric range facet result.
type NumericRangeFacet struct {
	Name  string   `json:"name"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
	Count int      `json:"count"`
}

// DateRangeFacet is a date range facet result.
type DateRangeFacet struct {
	Name  string     `json:"name"`
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
	Count int        `json:"count"`
}
