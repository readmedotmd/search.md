package highlight_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/readmedotmd/search.md/analysis"
	"github.com/readmedotmd/search.md/search/highlight"

	// Import registry for side effects so analyzers are registered.
	_ "github.com/readmedotmd/search.md/registry"
)

// mockTermVectorReader implements highlight.TermVectorReader for testing.
type mockTermVectorReader struct {
	vectors map[string]*highlight.TermVector // key: "field|docID|term"
	err     error
}

func (m *mockTermVectorReader) GetTermVector(field, docID, term string) (*highlight.TermVector, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := field + "|" + docID + "|" + term
	tv, ok := m.vectors[key]
	if !ok {
		return nil, fmt.Errorf("no term vector for %s", key)
	}
	return tv, nil
}

func TestHighlight_EmptyText(t *testing.T) {
	h := highlight.NewHighlighter()
	reader := &mockTermVectorReader{}
	result := h.Highlight(reader, "doc1", "body", "", "standard", []string{"hello"})
	if result != nil {
		t.Errorf("expected nil for empty text, got %v", result)
	}
}

func TestHighlight_NoQueryTerms(t *testing.T) {
	h := highlight.NewHighlighter()
	reader := &mockTermVectorReader{}
	result := h.Highlight(reader, "doc1", "body", "some text here", "standard", nil)
	if result != nil {
		t.Errorf("expected nil for empty query terms, got %v", result)
	}

	result = h.Highlight(reader, "doc1", "body", "some text here", "standard", []string{})
	if result != nil {
		t.Errorf("expected nil for empty query terms slice, got %v", result)
	}
}

func TestHighlight_BasicHighlighting(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog"
	h := highlight.NewHighlighter()

	reader := &mockTermVectorReader{
		vectors: map[string]*highlight.TermVector{
			"body|doc1|fox": {
				Positions: []analysis.TokenPosition{
					{Position: 4, Start: 16, End: 19},
				},
			},
		},
	}

	result := h.Highlight(reader, "doc1", "body", text, "standard", []string{"fox"})
	if len(result) == 0 {
		t.Fatal("expected at least one fragment")
	}

	fragment := result[0]
	if !strings.Contains(fragment, h.PreTag+"fox"+h.PostTag) {
		t.Errorf("fragment should contain highlighted term, got: %s", fragment)
	}
}

func TestHighlight_FallbackToAnalyzer(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog"
	h := highlight.NewHighlighter()

	// Reader that always returns errors, forcing fallback to analyzer.
	reader := &mockTermVectorReader{
		err: fmt.Errorf("no term vectors available"),
	}

	result := h.Highlight(reader, "doc1", "body", text, "standard", []string{"fox"})
	if len(result) == 0 {
		t.Fatal("expected at least one fragment from analyzer fallback")
	}

	fragment := result[0]
	if !strings.Contains(fragment, h.PreTag+"fox"+h.PostTag) {
		t.Errorf("fragment should contain highlighted term via fallback, got: %s", fragment)
	}
}

func TestHighlight_HTMLEscaping(t *testing.T) {
	text := "Check <script>alert('xss')</script> and fox here"
	h := highlight.NewHighlighter()
	h.FragmentSize = 200

	reader := &mockTermVectorReader{
		vectors: map[string]*highlight.TermVector{
			"body|doc1|fox": {
				Positions: []analysis.TokenPosition{
					{Position: 1, Start: 39, End: 42},
				},
			},
		},
	}

	result := h.Highlight(reader, "doc1", "body", text, "standard", []string{"fox"})
	if len(result) == 0 {
		t.Fatal("expected at least one fragment")
	}

	fragment := result[0]
	if strings.Contains(fragment, "<script>") {
		t.Errorf("HTML should be escaped in output, got: %s", fragment)
	}
	if !strings.Contains(fragment, "&lt;script&gt;") {
		t.Errorf("expected escaped HTML entities, got: %s", fragment)
	}
}

func TestHighlight_FragmentBoundaries(t *testing.T) {
	// Term at the very beginning of text.
	text := "fox is here at the start"
	h := highlight.NewHighlighter()
	h.FragmentSize = 10

	reader := &mockTermVectorReader{
		vectors: map[string]*highlight.TermVector{
			"body|doc1|fox": {
				Positions: []analysis.TokenPosition{
					{Position: 1, Start: 0, End: 3},
				},
			},
		},
	}

	result := h.Highlight(reader, "doc1", "body", text, "standard", []string{"fox"})
	if len(result) == 0 {
		t.Fatal("expected at least one fragment")
	}

	// Term at the very end of text.
	text2 := "at the end is fox"
	reader2 := &mockTermVectorReader{
		vectors: map[string]*highlight.TermVector{
			"body|doc1|fox": {
				Positions: []analysis.TokenPosition{
					{Position: 5, Start: 14, End: 17},
				},
			},
		},
	}

	result2 := h.Highlight(reader2, "doc1", "body", text2, "standard", []string{"fox"})
	if len(result2) == 0 {
		t.Fatal("expected at least one fragment for term at end")
	}
}

func TestExtractQueryTerms(t *testing.T) {
	terms := highlight.ExtractQueryTerms([]string{"Running", "QUICKLY"}, "standard")
	if len(terms) == 0 {
		t.Fatal("expected extracted terms")
	}

	// Standard analyzer should lowercase (and possibly stem) the terms.
	for _, term := range terms {
		if term != strings.ToLower(term) {
			t.Errorf("expected lowercased term, got: %s", term)
		}
	}

	// The analyzer produces analyzed forms. Verify we get the right count
	// and that the results are non-empty lowercase strings.
	if len(terms) != 2 {
		t.Errorf("expected 2 extracted terms, got %d: %v", len(terms), terms)
	}

	// Verify the terms are analyzed (not the raw input).
	for _, term := range terms {
		if term == "Running" || term == "QUICKLY" {
			t.Errorf("expected analyzed (lowercased/stemmed) term, got raw input: %s", term)
		}
	}
}

func TestHighlight_ANSIHighlighter(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog"
	h := highlight.NewANSIHighlighter()

	reader := &mockTermVectorReader{
		vectors: map[string]*highlight.TermVector{
			"body|doc1|fox": {
				Positions: []analysis.TokenPosition{
					{Position: 4, Start: 16, End: 19},
				},
			},
		},
	}

	result := h.Highlight(reader, "doc1", "body", text, "standard", []string{"fox"})
	if len(result) == 0 {
		t.Fatal("expected at least one fragment")
	}

	fragment := result[0]
	if !strings.Contains(fragment, "\033[1;33m") {
		t.Errorf("expected ANSI bold/yellow code in fragment, got: %s", fragment)
	}
	if !strings.Contains(fragment, "\033[0m") {
		t.Errorf("expected ANSI reset code in fragment, got: %s", fragment)
	}
}

func TestHighlight_MultipleLocations(t *testing.T) {
	text := "The fox chased another fox through the forest where a fox hid"
	h := highlight.NewHighlighter()
	h.FragmentSize = 200 // Large enough to capture all in one fragment.

	reader := &mockTermVectorReader{
		vectors: map[string]*highlight.TermVector{
			"body|doc1|fox": {
				Positions: []analysis.TokenPosition{
					{Position: 2, Start: 4, End: 7},
					{Position: 5, Start: 24, End: 27},
					{Position: 10, Start: 54, End: 57},
				},
			},
		},
	}

	result := h.Highlight(reader, "doc1", "body", text, "standard", []string{"fox"})
	if len(result) == 0 {
		t.Fatal("expected at least one fragment")
	}

	// Count how many times the PreTag appears.
	fragment := result[0]
	count := strings.Count(fragment, h.PreTag)
	if count < 3 {
		t.Errorf("expected at least 3 highlighted occurrences, got %d in: %s", count, fragment)
	}
}
