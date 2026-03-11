package highlight

import (
	"html"
	"sort"
	"strings"

	"github.com/readmedotmd/search.md/analysis"
	"github.com/readmedotmd/search.md/registry"
)

// TermVectorReader is the minimal interface needed by the highlighter to
// retrieve term-vector data from the index.  Both TermVectorReader and
// index.Index satisfy this interface.
type TermVectorReader interface {
	GetTermVector(field, docID, term string) (*TermVector, error)
}

// TermVector stores position information for highlighting and phrase queries.
type TermVector struct {
	Positions []analysis.TokenPosition
}

// Highlighter produces highlighted fragments of text showing where matches occurred.
type Highlighter struct {
	FragmentSize int
	PreTag       string
	PostTag      string
	Separator    string
}

// NewHighlighter creates a new highlighter with default settings.
func NewHighlighter() *Highlighter {
	return &Highlighter{
		FragmentSize: 100,
		PreTag:       "<mark>",
		PostTag:      "</mark>",
		Separator:    " ... ",
	}
}

// NewANSIHighlighter creates a highlighter using ANSI terminal codes.
func NewANSIHighlighter() *Highlighter {
	return &Highlighter{
		FragmentSize: 100,
		PreTag:       "\033[1;33m",
		PostTag:      "\033[0m",
		Separator:    " ... ",
	}
}

// Highlight generates highlighted fragments for a document field.
func (h *Highlighter) Highlight(reader TermVectorReader, docID, field, text, analyzerName string, queryTerms []string) []string {
	if text == "" || len(queryTerms) == 0 {
		return nil
	}

	// Get term locations
	locations := h.findLocations(reader, docID, field, text, analyzerName, queryTerms)
	if len(locations) == 0 {
		return nil
	}

	return h.buildFragments(text, locations)
}

type termLocation struct {
	term  string
	start int
	end   int
}

func (h *Highlighter) findLocations(reader TermVectorReader, docID, field, text, analyzerName string, queryTerms []string) []termLocation {
	var locations []termLocation

	// First try term vectors from the index
	for _, term := range queryTerms {
		tv, err := reader.GetTermVector(field, docID, term)
		if err == nil && tv != nil {
			for _, pos := range tv.Positions {
				locations = append(locations, termLocation{
					term:  term,
					start: pos.Start,
					end:   pos.End,
				})
			}
		}
	}

	// Fallback: re-analyze and find terms
	if len(locations) == 0 {
		analyzer, err := registry.AnalyzerByName(analyzerName)
		if err != nil {
			analyzer, _ = registry.AnalyzerByName("standard")
		}
		if analyzer == nil {
			return nil
		}

		tokens := analyzer.Analyze([]byte(text))
		queryTermSet := make(map[string]bool)
		for _, qt := range queryTerms {
			queryTermSet[strings.ToLower(qt)] = true
		}

		for _, token := range tokens {
			if queryTermSet[token.Term] {
				locations = append(locations, termLocation{
					term:  token.Term,
					start: token.Start,
					end:   token.End,
				})
			}
		}
	}

	// Sort by position
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].start < locations[j].start
	})

	return locations
}

func (h *Highlighter) buildFragments(text string, locations []termLocation) []string {
	if len(locations) == 0 {
		return nil
	}

	var fragments []string
	used := make([]bool, len(locations))

	for i, loc := range locations {
		if used[i] {
			continue
		}

		// Define fragment window
		fragStart := loc.start - h.FragmentSize/2
		if fragStart < 0 {
			fragStart = 0
		}
		fragEnd := loc.start + h.FragmentSize/2
		if fragEnd > len(text) {
			fragEnd = len(text)
		}

		// Collect all locations within this fragment
		var fragLocs []termLocation
		for j := i; j < len(locations); j++ {
			if locations[j].start >= fragStart && locations[j].end <= fragEnd {
				fragLocs = append(fragLocs, locations[j])
				used[j] = true
			}
		}

		// Build highlighted fragment
		fragment := h.highlightFragment(text, fragStart, fragEnd, fragLocs)
		fragments = append(fragments, fragment)
	}

	return fragments
}

func (h *Highlighter) highlightFragment(text string, start, end int, locations []termLocation) string {
	var result strings.Builder

	if start > 0 {
		result.WriteString("...")
	}

	cursor := start
	for _, loc := range locations {
		if loc.start > cursor {
			result.WriteString(html.EscapeString(text[cursor:loc.start]))
		}
		result.WriteString(h.PreTag)
		result.WriteString(html.EscapeString(text[loc.start:loc.end]))
		result.WriteString(h.PostTag)
		cursor = loc.end
	}
	if cursor < end {
		result.WriteString(html.EscapeString(text[cursor:end]))
	}

	if end < len(text) {
		result.WriteString("...")
	}

	return result.String()
}

// ExtractQueryTerms extracts the query terms that need highlighting.
func ExtractQueryTerms(queryTerms []string, analyzerName string) []string {
	if analyzerName == "" {
		analyzerName = "standard"
	}

	analyzer, err := registry.AnalyzerByName(analyzerName)
	if err != nil {
		return queryTerms
	}

	var terms []string
	for _, qt := range queryTerms {
		tokens := analyzer.Analyze([]byte(qt))
		for _, t := range tokens {
			terms = append(terms, t.Term)
		}
	}
	return terms
}
