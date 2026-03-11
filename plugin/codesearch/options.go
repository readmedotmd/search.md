package codesearch

import "github.com/readmedotmd/search.md/plugin"

// WithSymbolIndexer returns a plugin.Option that registers a SymbolFieldIndexer
// with the given extractor. This enables indexing of FieldTypeSymbol fields.
//
// Usage:
//
//	extractor := codesearch.NewRegexExtractor()
//	idx, _ := searchmd.New(store, mapping,
//	    codesearch.WithSymbolIndexer(extractor),
//	)
func WithSymbolIndexer(extractor SymbolExtractor) plugin.Option {
	return plugin.WithFieldIndexer(&SymbolFieldIndexer{
		Extractor: extractor,
	})
}

// WithRegexSymbolIndexer is a convenience function that registers a
// SymbolFieldIndexer backed by the built-in RegexExtractor.
func WithRegexSymbolIndexer() plugin.Option {
	return WithSymbolIndexer(NewRegexExtractor())
}

// WithTreeSitterIndexer registers a SymbolFieldIndexer backed by a
// TreeSitterExtractor. The extractor must have parsers registered
// for the languages you want to index.
func WithTreeSitterIndexer(extractor *TreeSitterExtractor) plugin.Option {
	return WithSymbolIndexer(extractor)
}

// WithTagIndexer registers a SymbolFieldIndexer backed by a TagExtractor.
func WithTagIndexer() plugin.Option {
	return WithSymbolIndexer(NewTagExtractor())
}
