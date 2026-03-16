package searchmd

import (
	"github.com/readmedotmd/store.md/backend/memory"
)

// newMemStore returns an in-memory Store from the store.md backend/memory package.
func newMemStore() *memory.StoreMemory {
	return memory.New()
}
