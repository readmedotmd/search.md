package plugin

import (
	"github.com/readmedotmd/search.md/search/highlight"
)

// HTMLHighlighterFactory creates HTML highlighters.
type HTMLHighlighterFactory struct {
	FragmentSize int
	PreTag       string
	PostTag      string
}

// DefaultHTMLHighlighter returns the default HTML highlighter factory.
func DefaultHTMLHighlighter() *HTMLHighlighterFactory {
	return &HTMLHighlighterFactory{
		FragmentSize: 100,
		PreTag:       "<mark>",
		PostTag:      "</mark>",
	}
}

func (f *HTMLHighlighterFactory) Name() string { return "html" }

func (f *HTMLHighlighterFactory) NewHighlighter() Highlighter {
	return &htmlHighlighter{
		inner: &highlight.Highlighter{
			FragmentSize: f.FragmentSize,
			PreTag:       f.PreTag,
			PostTag:      f.PostTag,
			Separator:    " ... ",
		},
	}
}

// ANSIHighlighterFactory creates ANSI terminal highlighters.
type ANSIHighlighterFactory struct {
	FragmentSize int
}

// DefaultANSIHighlighter returns the default ANSI highlighter factory.
func DefaultANSIHighlighter() *ANSIHighlighterFactory {
	return &ANSIHighlighterFactory{FragmentSize: 100}
}

func (f *ANSIHighlighterFactory) Name() string { return "ansi" }

func (f *ANSIHighlighterFactory) NewHighlighter() Highlighter {
	return &htmlHighlighter{
		inner: &highlight.Highlighter{
			FragmentSize: f.FragmentSize,
			PreTag:       "\033[1;33m",
			PostTag:      "\033[0m",
			Separator:    " ... ",
		},
	}
}

type htmlHighlighter struct {
	inner *highlight.Highlighter
}

func (h *htmlHighlighter) Highlight(reader IndexReader, docID, field, text, analyzerName string, queryTerms []string) []string {
	return h.inner.Highlight(&termVectorAdapter{reader}, docID, field, text, analyzerName, queryTerms)
}

// termVectorAdapter wraps a plugin.IndexReader so it satisfies
// highlight.TermVectorReader, converting index.TermVector to highlight.TermVector.
type termVectorAdapter struct {
	r IndexReader
}

func (a *termVectorAdapter) GetTermVector(field, docID, term string) (*highlight.TermVector, error) {
	tv, err := a.r.GetTermVector(field, docID, term)
	if err != nil || tv == nil {
		return nil, err
	}
	return &highlight.TermVector{Positions: tv.Positions}, nil
}
