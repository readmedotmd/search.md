package index

import (
	"strconv"

	"github.com/readmedotmd/search.md/document"
)

// DateTimeFieldIndexer handles indexing for datetime fields.
type DateTimeFieldIndexer struct{}

func (fi *DateTimeFieldIndexer) Type() string { return "datetime" }

func (fi *DateTimeFieldIndexer) IndexField(helpers IndexHelpers, docID string, field *document.Field) (*RevIdxEntry, error) {
	t, ok := field.DateTimeValue()
	if !ok {
		return nil, nil
	}
	key := prefixDateTime + field.Name + "/" + docID
	if err := helpers.Store().Set(key, strconv.FormatInt(t.UnixNano(), 10)); err != nil {
		return nil, err
	}
	return &RevIdxEntry{Field: field.Name, Type: "datetime"}, nil
}

func (fi *DateTimeFieldIndexer) DeleteField(helpers IndexHelpers, docID string, entry RevIdxEntry) error {
	return deleteKey(helpers.Store(), prefixDateTime+entry.Field+"/"+docID)
}
