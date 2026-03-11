package plugin

import (
	"fmt"
	"sync"

	"github.com/readmedotmd/search.md/index"
)

// Registry holds all registered plugins for a search index.
// Each SearchIndex gets its own Registry, populated with defaults
// and optionally extended with custom plugins.
type Registry struct {
	mu                 sync.RWMutex
	scorerFactory      ScorerFactory
	highlighterFactory HighlighterFactory
	facetFactory       FacetBuilderFactory
	queryPlugins       map[string]QueryPlugin
	fieldIndexers      []index.FieldIndexer // pending field indexers to register on the index
}

// NewRegistry creates a Registry with no defaults registered.
func NewRegistry() *Registry {
	return &Registry{
		queryPlugins: make(map[string]QueryPlugin),
	}
}

// RegisterFieldIndexer queues a field indexer for registration on the underlying index.
func (r *Registry) RegisterFieldIndexer(fi index.FieldIndexer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fieldIndexers = append(r.fieldIndexers, fi)
}

// FieldIndexers returns all registered field indexers.
func (r *Registry) FieldIndexers() []index.FieldIndexer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]index.FieldIndexer, len(r.fieldIndexers))
	copy(result, r.fieldIndexers)
	return result
}

// SetScorerFactory sets the scorer factory (e.g. BM25, TF-IDF).
func (r *Registry) SetScorerFactory(sf ScorerFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scorerFactory = sf
}

// ScorerFactory returns the registered scorer factory.
func (r *Registry) GetScorerFactory() ScorerFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.scorerFactory
}

// SetHighlighterFactory sets the highlighter factory.
func (r *Registry) SetHighlighterFactory(hf HighlighterFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.highlighterFactory = hf
}

// GetHighlighterFactory returns the registered highlighter factory.
func (r *Registry) GetHighlighterFactory() HighlighterFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.highlighterFactory
}

// SetFacetBuilderFactory sets the facet builder factory.
func (r *Registry) SetFacetBuilderFactory(ff FacetBuilderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.facetFactory = ff
}

// GetFacetBuilderFactory returns the registered facet builder factory.
func (r *Registry) GetFacetBuilderFactory() FacetBuilderFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.facetFactory
}

// RegisterQueryPlugin registers a query plugin by name.
func (r *Registry) RegisterQueryPlugin(qp QueryPlugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queryPlugins[qp.Name()] = qp
}

// GetQueryPlugin returns a query plugin by name.
func (r *Registry) GetQueryPlugin(name string) (QueryPlugin, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	qp, ok := r.queryPlugins[name]
	if !ok {
		return nil, fmt.Errorf("query plugin '%s' not found", name)
	}
	return qp, nil
}
