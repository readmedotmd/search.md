package index

import (
	"context"
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
	store := helpers.Store()
	ctx := context.Background()
	// Legacy key for point lookups (GetNumericValue).
	key := numericKey(field.Name, docID)
	if err := store.Set(ctx, key, strconv.FormatFloat(val, 'g', -1, 64)); err != nil {
		return nil, err
	}
	// Sorted key for efficient range scans.
	sortedKey := numericSortedKey(field.Name, val, docID)
	if err := store.Set(ctx, sortedKey, ""); err != nil {
		return nil, err
	}
	return &RevIdxEntry{Field: field.Name, Type: "numeric"}, nil
}

func (fi *NumericFieldIndexer) DeleteField(helpers IndexHelpers, docID string, entry RevIdxEntry) error {
	store := helpers.Store()
	ctx := context.Background()
	// Read the value to reconstruct the sorted key.
	if valStr, err := store.Get(ctx, numericKey(entry.Field, docID)); err == nil {
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			deleteKey(store, numericSortedKey(entry.Field, val, docID))
		}
	}
	return deleteKey(store, numericKey(entry.Field, docID))
}
