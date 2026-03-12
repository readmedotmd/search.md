package fswatcher

import "time"

// EmbedFunc computes a vector embedding for the given text.
type EmbedFunc func(text string) ([]float32, error)

// FilterFunc returns true if the file at the given path should be indexed.
// The path is relative to the watched root directory.
type FilterFunc func(relPath string) bool

// Option configures a Watcher.
type Option func(*config)

type config struct {
	// Indexing modes
	fts  bool
	code bool
	lang string // language hint for code/symbol extraction

	vector   bool
	vectorFn EmbedFunc
	dims     int

	// Behavior
	pollInterval time.Duration
	filter       FilterFunc
	batchSize    int
}

func defaults() *config {
	return &config{
		pollInterval: 10 * time.Second,
		batchSize:    64,
	}
}

// WithFTS enables full-text search indexing of file content.
func WithFTS() Option {
	return func(c *config) { c.fts = true }
}

// WithCode enables code-aware indexing with syntax-aware tokenization and
// symbol extraction. The language parameter is a hint for symbol extraction
// (e.g., "go", "python", "javascript"). Pass "" to auto-detect from extension.
func WithCode(language string) Option {
	return func(c *config) {
		c.code = true
		c.lang = language
	}
}

// WithVector enables vector/semantic indexing. The dims parameter specifies
// the embedding dimensionality. The embed function is called for each file's
// content to produce the vector.
func WithVector(dims int, fn EmbedFunc) Option {
	return func(c *config) {
		c.vector = true
		c.vectorFn = fn
		c.dims = dims
	}
}

// WithPollInterval sets how often the watcher checks for file changes.
// Default is 10 seconds.
func WithPollInterval(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.pollInterval = d
		}
	}
}

// WithFilter sets a filter function. Only files for which the function
// returns true will be indexed. The path passed is relative to the root.
func WithFilter(fn FilterFunc) Option {
	return func(c *config) { c.filter = fn }
}

// WithBatchSize sets how many documents are indexed per batch during
// scanning. Default is 64.
func WithBatchSize(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.batchSize = n
		}
	}
}

// WithAll is a convenience option that enables FTS and code indexing.
// To also enable vector indexing, combine with WithVector.
func WithAll(language string) Option {
	return func(c *config) {
		c.fts = true
		c.code = true
		c.lang = language
	}
}
