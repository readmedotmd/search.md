package synonym

import (
	"github.com/readmedotmd/search.md/analysis"
)

// SynonymFilter is a token filter that expands each token with its synonyms
// from the Moby Thesaurus. Synonym tokens share the same position as the
// original token, enabling phrase-aware synonym matching.
//
// The filter can optionally limit the number of synonyms per token to control
// index size.
type SynonymFilter struct {
	// MaxSynonyms limits the number of synonyms injected per token.
	// Zero means unlimited (all synonyms from the thesaurus).
	MaxSynonyms int
}

// NewSynonymFilter returns a SynonymFilter with no synonym limit.
func NewSynonymFilter() *SynonymFilter {
	return &SynonymFilter{}
}

// NewSynonymFilterWithMax returns a SynonymFilter that injects at most max
// synonyms per token.
func NewSynonymFilterWithMax(max int) *SynonymFilter {
	return &SynonymFilter{MaxSynonyms: max}
}

// Filter expands each token with its synonyms. Synonyms are inserted at the
// same position as the original token so that phrase queries still work
// correctly.
func (f *SynonymFilter) Filter(tokens []*analysis.Token) []*analysis.Token {
	loadThesaurus()

	var rv []*analysis.Token
	for _, t := range tokens {
		// Keep the original token.
		rv = append(rv, t)

		syns := thesaurusMap[t.Term]
		if len(syns) == 0 {
			continue
		}

		limit := len(syns)
		if f.MaxSynonyms > 0 && f.MaxSynonyms < limit {
			limit = f.MaxSynonyms
		}

		for i := 0; i < limit; i++ {
			rv = append(rv, &analysis.Token{
				Term:     syns[i],
				Position: t.Position, // same position as original
				Start:    t.Start,
				End:      t.End,
				Type:     t.Type,
			})
		}
	}
	return rv
}
