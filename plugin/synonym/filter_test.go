package synonym

import (
	"testing"

	"github.com/readmedotmd/search.md/analysis"
	pluginpkg "github.com/readmedotmd/search.md/plugin"
	"github.com/readmedotmd/search.md/registry"
)

func TestLookup(t *testing.T) {
	syns := Lookup("happy")
	if len(syns) == 0 {
		t.Fatal("expected synonyms for 'happy', got none")
	}

	found := false
	for _, s := range syns {
		if s == "glad" || s == "joyful" || s == "cheerful" || s == "content" || s == "pleased" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a common synonym for 'happy' (glad/joyful/cheerful/content/pleased), got: %v", syns[:min(5, len(syns))])
	}
}

func TestLookupCaseInsensitive(t *testing.T) {
	lower := Lookup("happy")
	upper := Lookup("HAPPY")
	mixed := Lookup("Happy")

	if len(lower) == 0 {
		t.Fatal("expected synonyms for 'happy'")
	}
	if len(upper) != len(lower) {
		t.Errorf("case mismatch: 'HAPPY' got %d syns, 'happy' got %d", len(upper), len(lower))
	}
	if len(mixed) != len(lower) {
		t.Errorf("case mismatch: 'Happy' got %d syns, 'happy' got %d", len(mixed), len(lower))
	}
}

func TestLookupUnknownWord(t *testing.T) {
	syns := Lookup("xyzzyplugh")
	if syns != nil {
		t.Errorf("expected nil for unknown word, got %v", syns)
	}
}

func TestSynonymFilter(t *testing.T) {
	f := NewSynonymFilter()
	tokens := []*analysis.Token{
		{Term: "happy", Position: 1, Start: 0, End: 5},
	}

	result := f.Filter(tokens)
	if len(result) < 2 {
		t.Fatalf("expected synonym expansion, got %d tokens", len(result))
	}

	if result[0].Term != "happy" {
		t.Errorf("first token should be original 'happy', got %q", result[0].Term)
	}

	for i, tok := range result {
		if tok.Position != 1 {
			t.Errorf("token %d (%q): expected position 1, got %d", i, tok.Term, tok.Position)
		}
	}
}

func TestSynonymFilterMaxSynonyms(t *testing.T) {
	f := NewSynonymFilterWithMax(3)
	tokens := []*analysis.Token{
		{Term: "happy", Position: 1, Start: 0, End: 5},
	}

	result := f.Filter(tokens)
	if len(result) > 4 {
		t.Errorf("expected at most 4 tokens (1 original + 3 synonyms), got %d", len(result))
	}
	if len(result) < 2 {
		t.Errorf("expected at least 2 tokens (1 original + 1 synonym), got %d", len(result))
	}
}

func TestSynonymFilterUnknownWord(t *testing.T) {
	f := NewSynonymFilter()
	tokens := []*analysis.Token{
		{Term: "xyzzyplugh", Position: 1, Start: 0, End: 10},
	}

	result := f.Filter(tokens)
	if len(result) != 1 {
		t.Errorf("expected 1 token for unknown word, got %d", len(result))
	}
}

func TestSynonymFilterMultipleTokens(t *testing.T) {
	f := NewSynonymFilterWithMax(2)
	tokens := []*analysis.Token{
		{Term: "happy", Position: 1, Start: 0, End: 5},
		{Term: "dog", Position: 2, Start: 6, End: 9},
	}

	result := f.Filter(tokens)

	for _, tok := range result {
		if tok.Term == "happy" && tok.Position != 1 {
			t.Errorf("'happy' should be at position 1, got %d", tok.Position)
		}
		if tok.Term == "dog" && tok.Position != 2 {
			t.Errorf("'dog' should be at position 2, got %d", tok.Position)
		}
	}
}

func TestRegister(t *testing.T) {
	registry.Reset()
	Register()

	f, err := registry.TokenFilterByName("synonym")
	if err != nil {
		t.Fatalf("synonym filter not registered: %v", err)
	}
	if f == nil {
		t.Fatal("synonym filter is nil")
	}

	a, err := registry.AnalyzerByName("synonym")
	if err != nil {
		t.Fatalf("synonym analyzer not registered: %v", err)
	}
	if a == nil {
		t.Fatal("synonym analyzer is nil")
	}
}

func TestWithSynonymsOption(t *testing.T) {
	registry.Reset()

	reg := pluginpkg.NewRegistry()
	opt := WithSynonyms()
	opt(reg)

	// Verify filter and analyzer are registered in the global registry.
	f, err := registry.TokenFilterByName("synonym")
	if err != nil {
		t.Fatalf("synonym filter not registered via WithSynonyms: %v", err)
	}
	if f == nil {
		t.Fatal("synonym filter is nil")
	}

	a, err := registry.AnalyzerByName("synonym")
	if err != nil {
		t.Fatalf("synonym analyzer not registered via WithSynonyms: %v", err)
	}
	if a == nil {
		t.Fatal("synonym analyzer is nil")
	}

	// Verify query plugin is registered.
	qp, err := reg.GetQueryPlugin("synonym")
	if err != nil {
		t.Fatalf("synonym query plugin not registered via WithSynonyms: %v", err)
	}
	if qp == nil {
		t.Fatal("synonym query plugin is nil")
	}
}

func TestSynonymQueryExtractTerms(t *testing.T) {
	registry.Reset()
	Register()

	q := NewSynonymQuery("happy").SetMaxSynonyms(3)
	terms := q.ExtractTerms()
	if len(terms) < 2 {
		t.Fatalf("expected extracted terms to include synonyms, got %d terms", len(terms))
	}

	// Should include the original match text.
	found := false
	for _, term := range terms {
		if term == "happy" {
			found = true
			break
		}
	}
	if !found {
		t.Error("extracted terms should include original 'happy'")
	}
}

func TestNewTextFieldMapping(t *testing.T) {
	fm := NewTextFieldMapping()
	if fm.Analyzer != "synonym" {
		t.Errorf("expected analyzer 'synonym', got %q", fm.Analyzer)
	}
	if !fm.Store {
		t.Error("expected Store=true")
	}
	if !fm.Index {
		t.Error("expected Index=true")
	}
	if !fm.IncludeTermVectors {
		t.Error("expected IncludeTermVectors=true")
	}
}

func TestThesaurusSize(t *testing.T) {
	loadThesaurus()
	if len(thesaurusMap) < 25000 {
		t.Errorf("expected at least 25000 entries, got %d", len(thesaurusMap))
	}
}
