package synonym

import (
	"github.com/readmedotmd/search.md/analysis"
	"github.com/readmedotmd/search.md/document"
	"github.com/readmedotmd/search.md/mapping"
	"github.com/readmedotmd/search.md/plugin"
	"github.com/readmedotmd/search.md/registry"
)

// WithSynonyms returns a plugin.Option that enables synonym support.
// It registers the "synonym" token filter and analyzer in the global registry,
// and registers a "synonym" query plugin so SynonymQuery can be used by name.
//
// Usage:
//
//	idx, _ := searchmd.New(store, mapping,
//	    synonym.WithSynonyms(),
//	)
//
//	// Query-time expansion (works with any analyzer at index time):
//	results, _ := idx.Search(ctx, synonym.NewSynonymQuery("happy").SetField("content"))
//
//	// Or use the "synonym" analyzer on a field for index-time expansion:
//	fm := synonym.NewTextFieldMapping()
//	docMapping.AddFieldMapping("content", fm)
func WithSynonyms() plugin.Option {
	return WithSynonymsMax(0)
}

// WithSynonymsMax returns a plugin.Option that enables synonym support with
// a per-token synonym limit for the index-time token filter.
func WithSynonymsMax(maxSynonyms int) plugin.Option {
	return func(r *plugin.Registry) {
		// Register the token filter and analyzer in the global registry
		// for index-time synonym expansion.
		f := &SynonymFilter{MaxSynonyms: maxSynonyms}
		registry.RegisterTokenFilter("synonym", f)
		registry.RegisterAnalyzer("synonym", &analysis.Analyzer{
			Tokenizer: &analysis.UnicodeTokenizer{},
			TokenFilters: []analysis.TokenFilter{
				&analysis.LowerCaseFilter{},
				f,
				analysis.NewStopWordsFilter(),
				&analysis.PorterStemFilter{},
			},
		})

		// Register the synonym query plugin for query-time expansion.
		r.RegisterQueryPlugin(&synonymQueryPlugin{})
	}
}

// Register registers the synonym filter and a "synonym" analyzer in the
// global registry. The analyzer uses unicode tokenization, lowercasing,
// synonym expansion, stop-word removal, and Porter stemming.
//
// Prefer WithSynonyms() when using searchmd.New() — this function is for
// standalone use of the token filter without the full plugin system.
func Register() {
	RegisterWithMax(0)
}

// RegisterWithMax registers the synonym filter with a per-token synonym limit.
func RegisterWithMax(maxSynonyms int) {
	f := &SynonymFilter{MaxSynonyms: maxSynonyms}
	registry.RegisterTokenFilter("synonym", f)

	registry.RegisterAnalyzer("synonym", &analysis.Analyzer{
		Tokenizer: &analysis.UnicodeTokenizer{},
		TokenFilters: []analysis.TokenFilter{
			&analysis.LowerCaseFilter{},
			f,
			analysis.NewStopWordsFilter(),
			&analysis.PorterStemFilter{},
		},
	})
}

// NewTextFieldMapping returns a text field mapping that uses the "synonym"
// analyzer for index-time synonym expansion. Documents indexed with this
// mapping will have synonyms added to the inverted index automatically.
func NewTextFieldMapping() *mapping.FieldMapping {
	return &mapping.FieldMapping{
		Type:               document.FieldTypeText,
		Analyzer:           "synonym",
		Store:              true,
		Index:              true,
		IncludeTermVectors: true,
	}
}
