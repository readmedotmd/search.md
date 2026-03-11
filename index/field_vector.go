package index

import (
	"encoding/json"

	"github.com/readmedotmd/search.md/document"
)

// VectorFieldIndexer handles indexing for vector fields.
type VectorFieldIndexer struct{}

func (fi *VectorFieldIndexer) Type() string { return "vector" }

func (fi *VectorFieldIndexer) IndexField(helpers IndexHelpers, docID string, field *document.Field) (*RevIdxEntry, error) {
	vec, ok := field.VectorValue()
	if !ok {
		return nil, nil
	}
	vecJSON, err := json.Marshal(vec)
	if err != nil {
		return nil, err
	}
	key := prefixVector + field.Name + "/" + docID
	if err := helpers.Store().Set(key, string(vecJSON)); err != nil {
		return nil, err
	}
	return &RevIdxEntry{Field: field.Name, Type: "vector"}, nil
}

func (fi *VectorFieldIndexer) DeleteField(helpers IndexHelpers, docID string, entry RevIdxEntry) error {
	return deleteKey(helpers.Store(), prefixVector+entry.Field+"/"+docID)
}
