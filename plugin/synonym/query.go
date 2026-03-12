package synonym

import (
	"strings"

	"github.com/readmedotmd/search.md/plugin"
	"github.com/readmedotmd/search.md/registry"
	"github.com/readmedotmd/search.md/search/query"
)

// SynonymQuery expands a search term with its synonyms at query time.
// It analyzes the input text, looks up synonyms for each token, and builds
// a disjunction (OR) query across the original terms and their synonyms.
//
// This works with any index — no special analyzer is needed at index time.
// Synonym matches are boosted lower than exact matches so original terms
// rank higher.
//
// Usage:
//
//	q := synonym.NewSynonymQuery("happy").SetField("content")
//	results, _ := idx.Search(ctx, q)
type SynonymQuery struct {
	Match        string  `json:"match"`
	Field        string  `json:"field"`
	Analyzer     string  `json:"analyzer,omitempty"`
	Boost        float64 `json:"boost,omitempty"`
	SynonymBoost float64 `json:"synonym_boost,omitempty"` // boost for synonym terms (default 0.5)
	MaxSynonyms  int     `json:"max_synonyms,omitempty"`  // per-token limit (default 10)
}

// NewSynonymQuery creates a query that expands search terms with synonyms.
func NewSynonymQuery(match string) *SynonymQuery {
	return &SynonymQuery{
		Match:        match,
		Boost:        1.0,
		SynonymBoost: 0.5,
		MaxSynonyms:  10,
	}
}

// SetField sets the field to search.
func (q *SynonymQuery) SetField(field string) *SynonymQuery {
	q.Field = field
	return q
}

// SetAnalyzer sets the analyzer used to tokenize the input (default "standard").
func (q *SynonymQuery) SetAnalyzer(analyzer string) *SynonymQuery {
	q.Analyzer = analyzer
	return q
}

// SetBoost sets the boost for the original terms.
func (q *SynonymQuery) SetBoost(boost float64) *SynonymQuery {
	q.Boost = boost
	return q
}

// SetSynonymBoost sets the boost applied to synonym terms (default 0.5).
// Lower values cause synonym matches to rank below exact matches.
func (q *SynonymQuery) SetSynonymBoost(boost float64) *SynonymQuery {
	q.SynonymBoost = boost
	return q
}

// SetMaxSynonyms sets the maximum number of synonyms per token (default 10).
func (q *SynonymQuery) SetMaxSynonyms(max int) *SynonymQuery {
	q.MaxSynonyms = max
	return q
}

// Searcher implements query.Query. It analyzes the input, expands each token
// with synonyms, and creates a disjunction of term queries.
func (q *SynonymQuery) Searcher(reader plugin.IndexReader, field string, sf plugin.ScorerFactory) (query.Searcher, error) {
	loadThesaurus()

	f := q.Field
	if f == "" {
		f = field
	}

	analyzerName := q.Analyzer
	if analyzerName == "" {
		analyzerName = "standard"
	}

	analyzer, err := registry.AnalyzerByName(analyzerName)
	if err != nil {
		return nil, err
	}

	tokens := analyzer.Analyze([]byte(q.Match))
	if len(tokens) == 0 {
		return query.NewMatchNoneQuery().Searcher(reader, f, sf)
	}

	synBoost := q.SynonymBoost
	if synBoost <= 0 {
		synBoost = 0.5
	}
	maxSyns := q.MaxSynonyms
	if maxSyns <= 0 {
		maxSyns = 10
	}

	var queries []query.Query

	for _, tok := range tokens {
		// Add the original term with full boost.
		queries = append(queries, query.NewTermQuery(tok.Term).SetField(f).SetBoost(q.Boost))

		// Look up synonyms for the original (unstemmed) input word.
		// We match against the lowercased original term since the thesaurus
		// stores headwords in lowercase.
		syns := thesaurusMap[strings.ToLower(tok.Term)]

		limit := len(syns)
		if maxSyns < limit {
			limit = maxSyns
		}

		for i := 0; i < limit; i++ {
			// Re-analyze each synonym through the same analyzer so it
			// matches the indexed form (e.g., stemmed).
			synTokens := analyzer.Analyze([]byte(syns[i]))
			for _, st := range synTokens {
				queries = append(queries, query.NewTermQuery(st.Term).SetField(f).SetBoost(synBoost))
			}
		}
	}

	if len(queries) == 1 {
		return queries[0].Searcher(reader, f, sf)
	}

	disj := query.NewDisjunctionQuery(queries...)
	return disj.Searcher(reader, f, sf)
}

// ExtractTerms implements plugin.QueryTermExtractor for highlighting support.
func (q *SynonymQuery) ExtractTerms() []string {
	loadThesaurus()

	terms := []string{q.Match}

	analyzerName := q.Analyzer
	if analyzerName == "" {
		analyzerName = "standard"
	}
	analyzer, err := registry.AnalyzerByName(analyzerName)
	if err != nil {
		return terms
	}

	tokens := analyzer.Analyze([]byte(q.Match))
	for _, tok := range tokens {
		terms = append(terms, tok.Term)
		syns := thesaurusMap[strings.ToLower(tok.Term)]
		limit := len(syns)
		if q.MaxSynonyms > 0 && q.MaxSynonyms < limit {
			limit = q.MaxSynonyms
		}
		for i := 0; i < limit; i++ {
			terms = append(terms, syns[i])
		}
	}

	return terms
}

// synonymQueryPlugin registers SynonymQuery as a named query type in the
// plugin registry so it can be constructed by name.
type synonymQueryPlugin struct{}

func (p *synonymQueryPlugin) Name() string {
	return "synonym"
}

func (p *synonymQueryPlugin) ParseQuery(params map[string]interface{}) (interface{}, error) {
	match, _ := params["match"].(string)
	q := NewSynonymQuery(match)
	if field, ok := params["field"].(string); ok {
		q.SetField(field)
	}
	if analyzer, ok := params["analyzer"].(string); ok {
		q.SetAnalyzer(analyzer)
	}
	if boost, ok := params["boost"].(float64); ok {
		q.SetBoost(boost)
	}
	if synBoost, ok := params["synonym_boost"].(float64); ok {
		q.SetSynonymBoost(synBoost)
	}
	if maxSyns, ok := params["max_synonyms"].(float64); ok {
		q.SetMaxSynonyms(int(maxSyns))
	}
	return q, nil
}
