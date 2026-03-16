package plugin

import (
	"strings"
	"testing"
	"time"

	"github.com/readmedotmd/search.md/analysis"
	"github.com/readmedotmd/search.md/index"
)

func TestBM25ScorerFactory(t *testing.T) {
	sf := DefaultBM25()
	if sf.Name() != "bm25" {
		t.Errorf("Name = %q, want bm25", sf.Name())
	}

	s := sf.NewScorer(100, 10, 50.0)
	score := s.Score(3, 40, 1.0)
	if score <= 0 {
		t.Errorf("BM25 score = %f, want > 0", score)
	}

	// Higher boost should produce higher score
	boosted := s.Score(3, 40, 2.0)
	if boosted <= score {
		t.Errorf("boosted score %f should be > %f", boosted, score)
	}
}

func TestTFIDFScorerFactory(t *testing.T) {
	sf := &TFIDFScorerFactory{}
	if sf.Name() != "tfidf" {
		t.Errorf("Name = %q, want tfidf", sf.Name())
	}

	s := sf.NewScorer(100, 10, 50.0)
	score := s.Score(3, 40, 1.0)
	if score <= 0 {
		t.Errorf("TF-IDF score = %f, want > 0", score)
	}

	// Higher boost should produce higher score
	boosted := s.Score(3, 40, 2.0)
	if boosted <= score {
		t.Errorf("boosted score %f should be > %f", boosted, score)
	}
}

func TestRegistry_ScorerFactory(t *testing.T) {
	r := NewRegistry()
	if r.GetScorerFactory() != nil {
		t.Error("expected nil scorer factory initially")
	}

	r.SetScorerFactory(DefaultBM25())
	sf := r.GetScorerFactory()
	if sf == nil || sf.Name() != "bm25" {
		t.Errorf("expected bm25 scorer factory, got %v", sf)
	}

	// Override with TF-IDF
	r.SetScorerFactory(&TFIDFScorerFactory{})
	sf = r.GetScorerFactory()
	if sf == nil || sf.Name() != "tfidf" {
		t.Errorf("expected tfidf scorer factory, got %v", sf)
	}
}

func TestRegistry_Options(t *testing.T) {
	r := NewRegistry()
	r.Apply(
		WithScorer(DefaultBM25()),
	)
	sf := r.GetScorerFactory()
	if sf == nil || sf.Name() != "bm25" {
		t.Errorf("WithScorer option failed, got %v", sf)
	}
}

type mockQueryPlugin struct{}

func (m *mockQueryPlugin) Name() string { return "mock" }
func (m *mockQueryPlugin) ParseQuery(params map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func TestRegistry_QueryPlugin(t *testing.T) {
	r := NewRegistry()
	r.RegisterQueryPlugin(&mockQueryPlugin{})

	qp, err := r.GetQueryPlugin("mock")
	if err != nil {
		t.Fatalf("GetQueryPlugin error: %v", err)
	}
	if qp.Name() != "mock" {
		t.Errorf("Name = %q, want mock", qp.Name())
	}

	_, err = r.GetQueryPlugin("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent query plugin")
	}
}

// ===========================================================================
// Mock IndexReader for highlighter tests
// ===========================================================================

type mockIndexReader struct {
	termVectors map[string]*index.TermVector // key: "field/docID/term"
}

func (m *mockIndexReader) DocCount() (uint64, error) { return 0, nil }
func (m *mockIndexReader) GetDocument(docID string) (map[string]interface{}, error) {
	return nil, nil
}
func (m *mockIndexReader) TermPostings(field, term string) ([]index.Posting, error) { return nil, nil }
func (m *mockIndexReader) TermsForField(field string) ([]string, error)             { return nil, nil }
func (m *mockIndexReader) TermsWithPrefix(field, termPrefix string) ([]string, error) {
	return nil, nil
}
func (m *mockIndexReader) DocumentFrequency(field, term string) (uint64, error) { return 0, nil }
func (m *mockIndexReader) FieldLength(field, docID string) (int, error)         { return 0, nil }
func (m *mockIndexReader) AverageFieldLength(field string) (float64, error)     { return 0, nil }
func (m *mockIndexReader) GetTermVector(field, docID, term string) (*index.TermVector, error) {
	if m.termVectors != nil {
		key := field + "/" + docID + "/" + term
		if tv, ok := m.termVectors[key]; ok {
			return tv, nil
		}
	}
	return nil, nil
}
func (m *mockIndexReader) GetVector(field, docID string) ([]float32, error) { return nil, nil }
func (m *mockIndexReader) ForEachVector(field string, fn func(docID string, vec []float32) bool) error {
	return nil
}
func (m *mockIndexReader) ForEachDocID(fn func(docID string) bool) error { return nil }
func (m *mockIndexReader) ListDocIDs(startAfter string, limit int) ([]string, error) {
	return nil, nil
}
func (m *mockIndexReader) NumericRange(field string, min, max *float64) ([]string, error) {
	return nil, nil
}
func (m *mockIndexReader) DateTimeRange(field string, start, end *time.Time) ([]string, error) {
	return nil, nil
}
func (m *mockIndexReader) GetNumericValue(field, docID string) (float64, bool) { return 0, false }
func (m *mockIndexReader) GetDateTimeValue(field, docID string) (time.Time, bool) {
	return time.Time{}, false
}
func (m *mockIndexReader) FuzzyTerms(field, term string, maxDist int) ([]string, error) {
	return nil, nil
}
func (m *mockIndexReader) HNSWSearch(field string, query []float32, k int) ([]string, []float64, bool) {
	return nil, nil, false
}

// ===========================================================================
// HTML Highlighter Tests
// ===========================================================================

func TestHTMLHighlighter_Name(t *testing.T) {
	f := DefaultHTMLHighlighter()
	if f.Name() != "html" {
		t.Errorf("Name = %q, want html", f.Name())
	}
}

func TestANSIHighlighter_Name(t *testing.T) {
	f := DefaultANSIHighlighter()
	if f.Name() != "ansi" {
		t.Errorf("Name = %q, want ansi", f.Name())
	}
}

func TestHTMLHighlighter_EscapesXSS(t *testing.T) {
	reader := &mockIndexReader{
		termVectors: map[string]*index.TermVector{
			"content/doc1/alert": {
				Positions: []analysis.TokenPosition{
					{Position: 1, Start: 21, End: 26},
				},
			},
		},
	}

	f := &HTMLHighlighterFactory{FragmentSize: 200, PreTag: "<mark>", PostTag: "</mark>"}
	hl := f.NewHighlighter()

	// Text containing HTML/XSS payload
	text := `<script>alert("xss")</script> alert normal text`
	fragments := hl.Highlight(reader, "doc1", "content", text, "standard", []string{"alert"})

	for _, frag := range fragments {
		// The fragment should NOT contain raw <script> tags
		if strings.Contains(frag, "<script>") {
			t.Errorf("fragment contains unescaped <script> tag: %s", frag)
		}
		// Should contain escaped version
		if !strings.Contains(frag, "&lt;script&gt;") {
			t.Errorf("fragment should contain escaped script tag, got: %s", frag)
		}
		// The <mark> tags themselves should NOT be escaped (they're our highlight markers)
		if !strings.Contains(frag, "<mark>") {
			t.Errorf("fragment should contain <mark> highlight tag, got: %s", frag)
		}
	}
}

func TestHTMLHighlighter_EmptyInput(t *testing.T) {
	reader := &mockIndexReader{}
	f := DefaultHTMLHighlighter()
	hl := f.NewHighlighter()

	// Empty text
	frags := hl.Highlight(reader, "doc1", "content", "", "standard", []string{"hello"})
	if len(frags) != 0 {
		t.Errorf("expected no fragments for empty text, got %d", len(frags))
	}

	// Empty query terms
	frags = hl.Highlight(reader, "doc1", "content", "hello world", "standard", nil)
	if len(frags) != 0 {
		t.Errorf("expected no fragments for empty query terms, got %d", len(frags))
	}
}

func TestHTMLHighlighter_EscapesAmpersands(t *testing.T) {
	reader := &mockIndexReader{
		termVectors: map[string]*index.TermVector{
			"content/doc1/terms": {
				Positions: []analysis.TokenPosition{
					{Position: 1, Start: 15, End: 20},
				},
			},
		},
	}

	f := &HTMLHighlighterFactory{FragmentSize: 200, PreTag: "<mark>", PostTag: "</mark>"}
	hl := f.NewHighlighter()

	text := "foo & bar baz terms rest"
	fragments := hl.Highlight(reader, "doc1", "content", text, "standard", []string{"terms"})

	for _, frag := range fragments {
		if strings.Contains(frag, "foo & bar") {
			t.Errorf("fragment contains unescaped '&': %s", frag)
		}
		if !strings.Contains(frag, "foo &amp; bar") {
			t.Errorf("fragment should contain escaped '&amp;', got: %s", frag)
		}
	}
}
