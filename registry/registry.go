package registry

import (
	"fmt"
	"sync"

	"github.com/readmedotmd/search.md/analysis"
)

// Registry is a global registry for analyzers, tokenizers, and filters.
type Registry struct {
	mu          sync.RWMutex
	analyzers   map[string]*analysis.Analyzer
	tokenizers  map[string]analysis.Tokenizer
	filters     map[string]analysis.TokenFilter
	charFilters map[string]analysis.CharFilter
}

var global = &Registry{
	analyzers:   make(map[string]*analysis.Analyzer),
	tokenizers:  make(map[string]analysis.Tokenizer),
	filters:     make(map[string]analysis.TokenFilter),
	charFilters: make(map[string]analysis.CharFilter),
}

func init() {
	registerDefaults()
}

// Reset re-initializes the global registry to its default state.
// This is intended for use in tests to ensure isolation between test cases.
func Reset() {
	global.mu.Lock()
	global.analyzers = make(map[string]*analysis.Analyzer)
	global.tokenizers = make(map[string]analysis.Tokenizer)
	global.filters = make(map[string]analysis.TokenFilter)
	global.charFilters = make(map[string]analysis.CharFilter)
	global.mu.Unlock()
	registerDefaults()
}

func registerDefaults() {
	// Register default tokenizers
	RegisterTokenizer("unicode", &analysis.UnicodeTokenizer{})
	RegisterTokenizer("whitespace", &analysis.WhitespaceTokenizer{})
	RegisterTokenizer("keyword", &analysis.KeywordTokenizer{})
	RegisterTokenizer("code", &analysis.CodeTokenizer{})

	// Register default token filters
	RegisterTokenFilter("lowercase", &analysis.LowerCaseFilter{})
	RegisterTokenFilter("stop_en", analysis.NewStopWordsFilter())
	RegisterTokenFilter("porter_stem", &analysis.PorterStemFilter{})
	RegisterTokenFilter("trim", &analysis.TrimFilter{})
	RegisterTokenFilter("unique", &analysis.UniqueFilter{})

	// Register default char filters
	RegisterCharFilter("html", &analysis.HTMLCharFilter{})

	// Register default analyzers
	RegisterAnalyzer("standard", &analysis.Analyzer{
		Tokenizer: &analysis.UnicodeTokenizer{},
		TokenFilters: []analysis.TokenFilter{
			&analysis.LowerCaseFilter{},
			analysis.NewStopWordsFilter(),
			&analysis.PorterStemFilter{},
		},
	})

	RegisterAnalyzer("simple", &analysis.Analyzer{
		Tokenizer: &analysis.UnicodeTokenizer{},
		TokenFilters: []analysis.TokenFilter{
			&analysis.LowerCaseFilter{},
		},
	})

	RegisterAnalyzer("keyword", &analysis.Analyzer{
		Tokenizer: &analysis.KeywordTokenizer{},
	})

	RegisterAnalyzer("whitespace", &analysis.Analyzer{
		Tokenizer: &analysis.WhitespaceTokenizer{},
	})

	RegisterAnalyzer("code", &analysis.Analyzer{
		Tokenizer: &analysis.CodeTokenizer{},
		TokenFilters: []analysis.TokenFilter{
			&analysis.LowerCaseFilter{},
			&analysis.UniqueFilter{},
		},
	})
}

// RegisterAnalyzer registers an analyzer by name.
func RegisterAnalyzer(name string, a *analysis.Analyzer) {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.analyzers[name] = a
}

// AnalyzerByName returns a registered analyzer.
func AnalyzerByName(name string) (*analysis.Analyzer, error) {
	global.mu.RLock()
	defer global.mu.RUnlock()
	a, ok := global.analyzers[name]
	if !ok {
		return nil, fmt.Errorf("analyzer '%s' not found", name)
	}
	return a, nil
}

// RegisterTokenizer registers a tokenizer by name.
func RegisterTokenizer(name string, t analysis.Tokenizer) {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.tokenizers[name] = t
}

// TokenizerByName returns a registered tokenizer.
func TokenizerByName(name string) (analysis.Tokenizer, error) {
	global.mu.RLock()
	defer global.mu.RUnlock()
	t, ok := global.tokenizers[name]
	if !ok {
		return nil, fmt.Errorf("tokenizer '%s' not found", name)
	}
	return t, nil
}

// RegisterTokenFilter registers a token filter by name.
func RegisterTokenFilter(name string, f analysis.TokenFilter) {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.filters[name] = f
}

// TokenFilterByName returns a registered token filter.
func TokenFilterByName(name string) (analysis.TokenFilter, error) {
	global.mu.RLock()
	defer global.mu.RUnlock()
	f, ok := global.filters[name]
	if !ok {
		return nil, fmt.Errorf("token filter '%s' not found", name)
	}
	return f, nil
}

// RegisterCharFilter registers a char filter by name.
func RegisterCharFilter(name string, f analysis.CharFilter) {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.charFilters[name] = f
}

// CharFilterByName returns a registered char filter.
func CharFilterByName(name string) (analysis.CharFilter, error) {
	global.mu.RLock()
	defer global.mu.RUnlock()
	f, ok := global.charFilters[name]
	if !ok {
		return nil, fmt.Errorf("char filter '%s' not found", name)
	}
	return f, nil
}
