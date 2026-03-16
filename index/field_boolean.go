package index

import (
	"context"
	"encoding/json"

	"github.com/readmedotmd/search.md/document"
)

// BooleanFieldIndexer handles indexing for boolean fields.
type BooleanFieldIndexer struct{}

func (fi *BooleanFieldIndexer) Type() string { return "bool" }

func (fi *BooleanFieldIndexer) IndexField(helpers IndexHelpers, docID string, field *document.Field) (*RevIdxEntry, error) {
	val, ok := field.BooleanValue()
	if !ok {
		return nil, nil
	}
	store := helpers.Store()
	ctx := context.Background()
	termVal := "F"
	if val {
		termVal = "T"
	}
	posting := Posting{DocID: docID, Frequency: 1, Norm: 1.0}
	postingJSON, _ := json.Marshal(posting)
	key := termKey(field.Name, termVal, docID)
	if err := store.Set(ctx, key, string(postingJSON)); err != nil {
		return nil, err
	}
	if err := helpers.IncrementDocFreq(field.Name, termVal); err != nil {
		return nil, err
	}
	bk := boolKey(field.Name, docID)
	if err := store.Set(ctx, bk, termVal); err != nil {
		return nil, err
	}
	return &RevIdxEntry{Field: field.Name, Type: "bool", Terms: []string{termVal}}, nil
}

func (fi *BooleanFieldIndexer) DeleteField(helpers IndexHelpers, docID string, entry RevIdxEntry) error {
	store := helpers.Store()
	for _, term := range entry.Terms {
		if err := deleteKey(store, termKey(entry.Field, term, docID)); err != nil {
			return err
		}
		if err := helpers.DecrementDocFreq(entry.Field, term); err != nil {
			return err
		}
	}
	if err := deleteKey(store, boolKey(entry.Field, docID)); err != nil {
		return err
	}
	return nil
}
