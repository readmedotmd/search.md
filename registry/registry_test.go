package registry

import (
	"testing"

	"github.com/readmedotmd/search.md/analysis"
)

// ---------------------------------------------------------------------------
// Default analyzers
// ---------------------------------------------------------------------------

func TestDefaultAnalyzers(t *testing.T) {
	names := []string{"standard", "simple", "keyword", "whitespace", "code"}
	for _, name := range names {
		a, err := AnalyzerByName(name)
		if err != nil {
			t.Errorf("AnalyzerByName(%q) error: %v", name, err)
			continue
		}
		if a == nil {
			t.Errorf("AnalyzerByName(%q) returned nil", name)
			continue
		}
		if a.Tokenizer == nil {
			t.Errorf("analyzer %q has nil Tokenizer", name)
		}
	}
}

func TestStandardAnalyzer_Analyzes(t *testing.T) {
	a, err := AnalyzerByName("standard")
	if err != nil {
		t.Fatal(err)
	}
	tokens := a.Analyze([]byte("The Quick Brown Fox"))
	// "the" is a stop word, should be removed; remaining should be lowercased and stemmed
	for _, tok := range tokens {
		if tok.Term == "the" {
			t.Error("standard analyzer should remove stop word 'the'")
		}
		if tok.Term == "The" || tok.Term == "Quick" || tok.Term == "Brown" || tok.Term == "Fox" {
			t.Errorf("standard analyzer should lowercase: got %q", tok.Term)
		}
	}
	if len(tokens) == 0 {
		t.Error("expected tokens from standard analyzer")
	}
}

func TestSimpleAnalyzer_Analyzes(t *testing.T) {
	a, err := AnalyzerByName("simple")
	if err != nil {
		t.Fatal(err)
	}
	tokens := a.Analyze([]byte("Hello World"))
	if len(tokens) != 2 {
		t.Errorf("simple analyzer token count = %d, want 2", len(tokens))
	}
	if len(tokens) >= 2 {
		if tokens[0].Term != "hello" || tokens[1].Term != "world" {
			t.Errorf("simple analyzer tokens = [%q, %q], want [hello, world]", tokens[0].Term, tokens[1].Term)
		}
	}
}

func TestKeywordAnalyzer_Analyzes(t *testing.T) {
	a, err := AnalyzerByName("keyword")
	if err != nil {
		t.Fatal(err)
	}
	tokens := a.Analyze([]byte("Hello World"))
	if len(tokens) != 1 {
		t.Errorf("keyword analyzer token count = %d, want 1", len(tokens))
	}
	if len(tokens) == 1 && tokens[0].Term != "Hello World" {
		t.Errorf("keyword analyzer token = %q, want %q", tokens[0].Term, "Hello World")
	}
}

// ---------------------------------------------------------------------------
// Default tokenizers
// ---------------------------------------------------------------------------

func TestDefaultTokenizers(t *testing.T) {
	names := []string{"unicode", "whitespace", "keyword", "code"}
	for _, name := range names {
		tok, err := TokenizerByName(name)
		if err != nil {
			t.Errorf("TokenizerByName(%q) error: %v", name, err)
			continue
		}
		if tok == nil {
			t.Errorf("TokenizerByName(%q) returned nil", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Default filters
// ---------------------------------------------------------------------------

func TestDefaultTokenFilters(t *testing.T) {
	names := []string{"lowercase", "stop_en", "porter_stem", "trim", "unique"}
	for _, name := range names {
		f, err := TokenFilterByName(name)
		if err != nil {
			t.Errorf("TokenFilterByName(%q) error: %v", name, err)
			continue
		}
		if f == nil {
			t.Errorf("TokenFilterByName(%q) returned nil", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Default char filters
// ---------------------------------------------------------------------------

func TestDefaultCharFilters(t *testing.T) {
	names := []string{"html"}
	for _, name := range names {
		cf, err := CharFilterByName(name)
		if err != nil {
			t.Errorf("CharFilterByName(%q) error: %v", name, err)
			continue
		}
		if cf == nil {
			t.Errorf("CharFilterByName(%q) returned nil", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Register and retrieve custom components
// ---------------------------------------------------------------------------

type dummyTokenizer struct{}

func (d *dummyTokenizer) Tokenize(input []byte) []*analysis.Token {
	return []*analysis.Token{{Term: string(input), Position: 1}}
}

type dummyFilter struct{}

func (d *dummyFilter) Filter(tokens []*analysis.Token) []*analysis.Token {
	return tokens
}

type dummyCharFilter struct{}

func (d *dummyCharFilter) Filter(input []byte) []byte {
	return input
}

func TestRegisterCustomAnalyzer(t *testing.T) {
	custom := &analysis.Analyzer{
		Tokenizer: &dummyTokenizer{},
	}
	RegisterAnalyzer("test_custom_analyzer", custom)

	got, err := AnalyzerByName("test_custom_analyzer")
	if err != nil {
		t.Fatalf("AnalyzerByName error: %v", err)
	}
	if got != custom {
		t.Error("retrieved analyzer is not the same instance")
	}
}

func TestRegisterCustomTokenizer(t *testing.T) {
	tok := &dummyTokenizer{}
	RegisterTokenizer("test_custom_tokenizer", tok)

	got, err := TokenizerByName("test_custom_tokenizer")
	if err != nil {
		t.Fatalf("TokenizerByName error: %v", err)
	}
	if got != tok {
		t.Error("retrieved tokenizer is not the same instance")
	}
}

func TestRegisterCustomTokenFilter(t *testing.T) {
	f := &dummyFilter{}
	RegisterTokenFilter("test_custom_filter", f)

	got, err := TokenFilterByName("test_custom_filter")
	if err != nil {
		t.Fatalf("TokenFilterByName error: %v", err)
	}
	if got != f {
		t.Error("retrieved filter is not the same instance")
	}
}

func TestRegisterCustomCharFilter(t *testing.T) {
	cf := &dummyCharFilter{}
	RegisterCharFilter("test_custom_charfilter", cf)

	got, err := CharFilterByName("test_custom_charfilter")
	if err != nil {
		t.Fatalf("CharFilterByName error: %v", err)
	}
	if got != cf {
		t.Error("retrieved char filter is not the same instance")
	}
}

// ---------------------------------------------------------------------------
// Errors for unknown names
// ---------------------------------------------------------------------------

func TestAnalyzerByName_NotFound(t *testing.T) {
	_, err := AnalyzerByName("nonexistent_analyzer_xyz")
	if err == nil {
		t.Error("expected error for unknown analyzer")
	}
}

func TestTokenizerByName_NotFound(t *testing.T) {
	_, err := TokenizerByName("nonexistent_tokenizer_xyz")
	if err == nil {
		t.Error("expected error for unknown tokenizer")
	}
}

func TestTokenFilterByName_NotFound(t *testing.T) {
	_, err := TokenFilterByName("nonexistent_filter_xyz")
	if err == nil {
		t.Error("expected error for unknown token filter")
	}
}

func TestCharFilterByName_NotFound(t *testing.T) {
	_, err := CharFilterByName("nonexistent_charfilter_xyz")
	if err == nil {
		t.Error("expected error for unknown char filter")
	}
}

// ---------------------------------------------------------------------------
// Overwrite existing registration
// ---------------------------------------------------------------------------

func TestRegisterAnalyzer_Overwrite(t *testing.T) {
	a1 := &analysis.Analyzer{Tokenizer: &dummyTokenizer{}}
	a2 := &analysis.Analyzer{Tokenizer: &dummyTokenizer{}}
	RegisterAnalyzer("test_overwrite", a1)
	RegisterAnalyzer("test_overwrite", a2)

	got, err := AnalyzerByName("test_overwrite")
	if err != nil {
		t.Fatal(err)
	}
	if got != a2 {
		t.Error("expected second registration to overwrite first")
	}
}
