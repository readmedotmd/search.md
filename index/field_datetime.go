package index

import (
	"context"
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
	store := helpers.Store()
	ctx := context.Background()
	nanos := t.UnixNano()
	// Legacy key for point lookups (GetDateTimeValue).
	key := dateTimeKey(field.Name, docID)
	if err := store.Set(ctx, key, strconv.FormatInt(nanos, 10)); err != nil {
		return nil, err
	}
	// Sorted key for efficient range scans.
	sortedKey := dateTimeSortedKey(field.Name, nanos, docID)
	if err := store.Set(ctx, sortedKey, ""); err != nil {
		return nil, err
	}
	return &RevIdxEntry{Field: field.Name, Type: "datetime"}, nil
}

func (fi *DateTimeFieldIndexer) DeleteField(helpers IndexHelpers, docID string, entry RevIdxEntry) error {
	store := helpers.Store()
	ctx := context.Background()
	// Read the value to reconstruct the sorted key.
	if valStr, err := store.Get(ctx, dateTimeKey(entry.Field, docID)); err == nil {
		if nanos, err := strconv.ParseInt(valStr, 10, 64); err == nil {
			deleteKey(store, dateTimeSortedKey(entry.Field, nanos, docID))
		}
	}
	return deleteKey(store, dateTimeKey(entry.Field, docID))
}
