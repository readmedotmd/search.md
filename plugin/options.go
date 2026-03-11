package plugin

import "github.com/readmedotmd/search.md/index"

// Option configures a Registry.
type Option func(*Registry)

// WithScorer sets the scorer factory.
func WithScorer(sf ScorerFactory) Option {
	return func(r *Registry) {
		r.SetScorerFactory(sf)
	}
}

// WithHighlighter sets the highlighter factory.
func WithHighlighter(hf HighlighterFactory) Option {
	return func(r *Registry) {
		r.SetHighlighterFactory(hf)
	}
}

// WithFacetBuilder sets the facet builder factory.
func WithFacetBuilder(ff FacetBuilderFactory) Option {
	return func(r *Registry) {
		r.SetFacetBuilderFactory(ff)
	}
}

// WithQueryPlugin registers a query plugin.
func WithQueryPlugin(qp QueryPlugin) Option {
	return func(r *Registry) {
		r.RegisterQueryPlugin(qp)
	}
}

// WithFieldIndexer registers a custom field indexer (e.g., geo, graph).
// The indexer will be registered on the underlying index at creation time.
func WithFieldIndexer(fi index.FieldIndexer) Option {
	return func(r *Registry) {
		r.RegisterFieldIndexer(fi)
	}
}

// Apply applies options to a registry.
func (r *Registry) Apply(opts ...Option) {
	for _, opt := range opts {
		opt(r)
	}
}
