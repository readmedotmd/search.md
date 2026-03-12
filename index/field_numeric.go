package index

import (
	"strconv"

	"github.com/readmedotmd/search.md/document"
)

// NumericFieldIndexer handles indexing for numeric fields.
type NumericFieldIndexer struct{}

func (fi *NumericFieldIndexer) Type() string { return "numeric" }

func (fi *NumericFieldIndexer) IndexField(helpers IndexHelpers, docID string, field *document.Field) (*RevIdxEntry, error) {
	val, ok := field.NumericValue()
	if !ok {
		return nil, nil
	}
	key := numericKey(field.Name, docID)
	if err := helpers.Store().Set(key, strconv.FormatFloat(val, 'g', -1, 64)); err != nil {
		return nil, err
	}
	return &RevIdxEntry{Field: field.Name, Type: "numeric"}, nil
}

func (fi *NumericFieldIndexer) DeleteField(helpers IndexHelpers, docID string, entry RevIdxEntry) error {
	return deleteKey(helpers.Store(), numericKey(entry.Field, docID))
}
