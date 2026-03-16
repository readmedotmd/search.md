package plugin

import (
	"time"

	"github.com/readmedotmd/search.md/index"
)

// IndexAdapter wraps a concrete *index.Index to implement IndexReader.
type IndexAdapter struct {
	Idx *index.Index
}

var _ IndexReader = (*IndexAdapter)(nil)

func (a *IndexAdapter) DocCount() (uint64, error) {
	return a.Idx.DocCount()
}

func (a *IndexAdapter) GetDocument(docID string) (map[string]interface{}, error) {
	sd, err := a.Idx.GetDocument(docID)
	if err != nil {
		return nil, err
	}
	return sd.Fields, nil
}

func (a *IndexAdapter) TermPostings(field, term string) ([]index.Posting, error) {
	return a.Idx.TermPostings(field, term)
}

func (a *IndexAdapter) TermsForField(field string) ([]string, error) {
	return a.Idx.TermsForField(field)
}

func (a *IndexAdapter) TermsWithPrefix(field, termPrefix string) ([]string, error) {
	return a.Idx.TermsWithPrefix(field, termPrefix)
}

func (a *IndexAdapter) DocumentFrequency(field, term string) (uint64, error) {
	return a.Idx.DocumentFrequency(field, term)
}

func (a *IndexAdapter) FieldLength(field, docID string) (int, error) {
	return a.Idx.FieldLength(field, docID)
}

func (a *IndexAdapter) AverageFieldLength(field string) (float64, error) {
	return a.Idx.AverageFieldLength(field)
}

func (a *IndexAdapter) GetTermVector(field, docID, term string) (*index.TermVector, error) {
	tv, err := a.Idx.GetTermVector(field, docID, term)
	if err != nil {
		return nil, err
	}
	if tv == nil {
		return nil, nil
	}
	return tv, nil
}

func (a *IndexAdapter) GetVector(field, docID string) ([]float32, error) {
	return a.Idx.GetVector(field, docID)
}

func (a *IndexAdapter) ForEachVector(field string, fn func(docID string, vec []float32) bool) error {
	return a.Idx.ForEachVector(field, fn)
}

func (a *IndexAdapter) ForEachDocID(fn func(docID string) bool) error {
	return a.Idx.ForEachDocID(fn)
}

func (a *IndexAdapter) ListDocIDs(startAfter string, limit int) ([]string, error) {
	return a.Idx.ListDocIDs(startAfter, limit)
}

func (a *IndexAdapter) NumericRange(field string, min, max *float64) ([]string, error) {
	return a.Idx.NumericRange(field, min, max)
}

func (a *IndexAdapter) DateTimeRange(field string, start, end *time.Time) ([]string, error) {
	return a.Idx.DateTimeRange(field, start, end)
}

func (a *IndexAdapter) GetNumericValue(field, docID string) (float64, bool) {
	return a.Idx.GetNumericValue(field, docID)
}

func (a *IndexAdapter) GetDateTimeValue(field, docID string) (time.Time, bool) {
	return a.Idx.GetDateTimeValue(field, docID)
}

func (a *IndexAdapter) FuzzyTerms(field, term string, maxDist int) ([]string, error) {
	return a.Idx.FuzzyTerms(field, term, maxDist)
}

func (a *IndexAdapter) HNSWSearch(field string, query []float32, k int) ([]string, []float64, bool) {
	return a.Idx.HNSWSearch(field, query, k)
}
