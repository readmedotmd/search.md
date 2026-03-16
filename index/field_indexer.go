package index

import (
	"context"
	"errors"

	storemd "github.com/readmedotmd/store.md"

	"github.com/readmedotmd/search.md/document"
)

// deleteKey deletes a key from the store, treating NotFoundError as success
// since the goal of deletion is to ensure the key no longer exists.
func deleteKey(store storemd.Store, key string) error {
	err := store.Delete(context.Background(), key)
	if err != nil && errors.Is(err, storemd.NotFoundError) {
		return nil
	}
	return err
}

// RevIdxEntry tracks what was indexed for a document so deletion can be targeted.
// Exported so field indexer plugins can create entries.
type RevIdxEntry struct {
	Field string   `json:"f"`
	Type  string   `json:"t"` // "text", "numeric", "bool", "datetime", "vector", or custom
	Terms []string `json:"r,omitempty"`
}

// IndexHelpers provides common index operations for field indexer plugins.
type IndexHelpers interface {
	Store() storemd.Store
	IncrementDocFreq(field, term string) error
	DecrementDocFreq(field, term string) error
	GetDocCount() (uint64, error)
	GetInt64(key string) (int64, error)
}

// FieldIndexer handles indexing and deletion for a specific field type.
// Implement this interface to add support for new field types (e.g., geo, graph).
type FieldIndexer interface {
	// Type returns the field type name this indexer handles.
	Type() string

	// IndexField indexes a single field for a document.
	// Returns a RevIdxEntry for targeted deletion, or nil if nothing was indexed.
	IndexField(helpers IndexHelpers, docID string, field *document.Field) (*RevIdxEntry, error)

	// DeleteField removes all indexed data for a document's field.
	DeleteField(helpers IndexHelpers, docID string, entry RevIdxEntry) error
}

// indexHelpersImpl wraps *Index to satisfy IndexHelpers without exposing the full Index.
type indexHelpersImpl struct {
	idx *Index
}

func (h *indexHelpersImpl) Store() storemd.Store               { return h.idx.store }
func (h *indexHelpersImpl) IncrementDocFreq(f, t string) error { return h.idx.incrementDocFreq(f, t) }
func (h *indexHelpersImpl) DecrementDocFreq(f, t string) error { return h.idx.decrementDocFreq(f, t) }
func (h *indexHelpersImpl) GetDocCount() (uint64, error)       { return h.idx.getDocCount() }
func (h *indexHelpersImpl) GetInt64(key string) (int64, error) { return h.idx.getInt64(key) }
