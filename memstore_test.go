package searchmd

import (
	"sort"
	"strings"
	"sync"

	storemd "github.com/readmedotmd/store.md"
)

// memStore is a simple in-memory Store implementation for testing.
type memStore struct {
	mu   sync.RWMutex
	data map[string]string
}

func newMemStore() *memStore {
	return &memStore{data: make(map[string]string)}
}

func (m *memStore) Get(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	if !ok {
		return "", storemd.NotFoundError
	}
	return val, nil
}

func (m *memStore) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *memStore) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[key]; !ok {
		return storemd.NotFoundError
	}
	delete(m.data, key)
	return nil
}

func (m *memStore) List(args storemd.ListArgs) ([]storemd.KeyValuePair, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []string
	for k := range m.data {
		if args.Prefix != "" && !strings.HasPrefix(k, args.Prefix) {
			continue
		}
		if args.StartAfter != "" && k <= args.StartAfter {
			continue
		}
		keys = append(keys, k)
	}

	sort.Strings(keys)

	limit := 0
	if args.Limit > 0 {
		limit = args.Limit
	}

	var result []storemd.KeyValuePair
	for _, k := range keys {
		if limit > 0 && len(result) >= limit {
			break
		}
		result = append(result, storemd.KeyValuePair{Key: k, Value: m.data[k]})
	}

	if result == nil {
		result = []storemd.KeyValuePair{}
	}
	return result, nil
}
