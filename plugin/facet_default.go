package plugin

import (
	"sort"

	"github.com/readmedotmd/search.md/search"
)

// DefaultFacetBuilderFactory creates the default facet builder.
type DefaultFacetBuilderFactory struct{}

func (f *DefaultFacetBuilderFactory) Name() string { return "default" }

func (f *DefaultFacetBuilderFactory) NewFacetBuilder(reader IndexReader) FacetBuilder {
	return &defaultFacetBuilder{reader: reader}
}

type defaultFacetBuilder struct {
	reader IndexReader
}

func (b *defaultFacetBuilder) Build(docIDs []string, facets map[string]*search.FacetRequest) map[string]*search.FacetResult {
	if len(facets) == 0 {
		return nil
	}

	results := make(map[string]*search.FacetResult)
	for name, req := range facets {
		results[name] = b.buildFacet(docIDs, req)
	}
	return results
}

func (b *defaultFacetBuilder) buildFacet(docIDs []string, req *search.FacetRequest) *search.FacetResult {
	result := &search.FacetResult{
		Field: req.Field,
	}

	if len(req.NumericRanges) > 0 {
		b.buildNumericRangeFacet(docIDs, req, result)
	} else if len(req.DateTimeRanges) > 0 {
		b.buildDateRangeFacet(docIDs, req, result)
	} else {
		b.buildTermFacet(docIDs, req, result)
	}

	return result
}

const (
	maxUniqueFacetTerms = 100000
	maxFacetSize        = 10000
)

func (b *defaultFacetBuilder) buildTermFacet(docIDs []string, req *search.FacetRequest, result *search.FacetResult) {
	termCounts := make(map[string]int)
	total := 0

	docIDSet := make(map[string]struct{}, len(docIDs))
	for _, id := range docIDs {
		docIDSet[id] = struct{}{}
	}

	terms, err := b.reader.TermsForField(req.Field)
	if err != nil {
		terms = nil
	}
	for _, term := range terms {
		postings, err := b.reader.TermPostings(req.Field, term)
		if err != nil {
			continue
		}
		for _, p := range postings {
			if _, ok := docIDSet[p.DocID]; ok {
				if _, exists := termCounts[term]; !exists && len(termCounts) >= maxUniqueFacetTerms {
					continue
				}
				termCounts[term]++
				total++
			}
		}
	}

	result.Total = total

	type termCount struct {
		term  string
		count int
	}
	sorted := make([]termCount, 0, len(termCounts))
	for term, count := range termCounts {
		sorted = append(sorted, termCount{term, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count == sorted[j].count {
			return sorted[i].term < sorted[j].term
		}
		return sorted[i].count > sorted[j].count
	})

	size := req.Size
	if size > maxFacetSize {
		size = maxFacetSize
	}
	if size <= 0 || size > len(sorted) {
		size = len(sorted)
	}

	result.Terms = make([]search.TermFacet, size)
	shown := 0
	for i := 0; i < size; i++ {
		result.Terms[i] = search.TermFacet{
			Term:  sorted[i].term,
			Count: sorted[i].count,
		}
		shown += sorted[i].count
	}
	result.Other = total - shown
}

func (b *defaultFacetBuilder) buildNumericRangeFacet(docIDs []string, req *search.FacetRequest, result *search.FacetResult) {
	result.NumericRanges = make([]search.NumericRangeFacet, len(req.NumericRanges))

	total := 0
	for i, nr := range req.NumericRanges {
		count := 0
		for _, docID := range docIDs {
			val, ok := b.reader.GetNumericValue(req.Field, docID)
			if !ok {
				continue
			}
			inRange := true
			if nr.Min != nil && val < *nr.Min {
				inRange = false
			}
			if nr.Max != nil && val > *nr.Max {
				inRange = false
			}
			if inRange {
				count++
			}
		}
		result.NumericRanges[i] = search.NumericRangeFacet{
			Name:  nr.Name,
			Min:   nr.Min,
			Max:   nr.Max,
			Count: count,
		}
		total += count
	}
	result.Total = total
}

func (b *defaultFacetBuilder) buildDateRangeFacet(docIDs []string, req *search.FacetRequest, result *search.FacetResult) {
	result.DateRanges = make([]search.DateRangeFacet, len(req.DateTimeRanges))

	total := 0
	for i, dr := range req.DateTimeRanges {
		count := 0
		for _, docID := range docIDs {
			val, ok := b.reader.GetDateTimeValue(req.Field, docID)
			if !ok {
				continue
			}
			inRange := true
			if dr.Start != nil && val.Before(*dr.Start) {
				inRange = false
			}
			if dr.End != nil && val.After(*dr.End) {
				inRange = false
			}
			if inRange {
				count++
			}
		}
		result.DateRanges[i] = search.DateRangeFacet{
			Name:  dr.Name,
			Start: dr.Start,
			End:   dr.End,
			Count: count,
		}
		total += count
	}
	result.Total = total
}
