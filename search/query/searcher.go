package query

import (
	"container/heap"
	"regexp"
	"sort"
	"time"

	"github.com/readmedotmd/search.md/plugin"
)

// scoreMinHeap is a min-heap for keeping top-K scored documents.
type scoreMinHeap []plugin.DocumentScore

func (h scoreMinHeap) Len() int            { return len(h) }
func (h scoreMinHeap) Less(i, j int) bool  { return h[i].Score < h[j].Score }
func (h scoreMinHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *scoreMinHeap) Push(x interface{}) { *h = append(*h, x.(plugin.DocumentScore)) }
func (h *scoreMinHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// termSearcher searches for a single term.
type termSearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

func newTermSearcher(reader plugin.IndexReader, field, term string, boost float64, sf plugin.ScorerFactory) (*termSearcher, error) {
	postings, err := reader.TermPostings(field, term)
	if err != nil {
		return nil, err
	}

	docCount, _ := reader.DocCount()
	docFreq, _ := reader.DocumentFrequency(field, term)
	avgFieldLen, _ := reader.AverageFieldLength(field)

	s := sf.NewScorer(docCount, docFreq, avgFieldLen)

	results := make([]*plugin.DocumentScore, 0, len(postings))
	for _, p := range postings {
		fieldLen, _ := reader.FieldLength(field, p.DocID)
		if fieldLen == 0 {
			fieldLen = 1
		}
		score := s.Score(p.Frequency, fieldLen, boost)
		results = append(results, &plugin.DocumentScore{
			ID:    p.DocID,
			Score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return &termSearcher{results: results}, nil
}

func (s *termSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *termSearcher) Count() int {
	return len(s.results)
}

// phraseSearcher searches for an exact phrase using term vectors.
type phraseSearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

func newPhraseSearcher(reader plugin.IndexReader, field string, terms []string, boost float64, sf plugin.ScorerFactory) (*phraseSearcher, error) {
	if len(terms) == 0 {
		return &phraseSearcher{}, nil
	}

	firstPostings, err := reader.TermPostings(field, terms[0])
	if err != nil || len(firstPostings) == 0 {
		return &phraseSearcher{}, nil
	}

	docCount, _ := reader.DocCount()
	docFreq, _ := reader.DocumentFrequency(field, terms[0])
	avgFieldLen, _ := reader.AverageFieldLength(field)
	s := sf.NewScorer(docCount, docFreq, avgFieldLen)

	var results []*plugin.DocumentScore

	for _, fp := range firstPostings {
		docID := fp.DocID

		if containsPhrase(reader, field, docID, terms) {
			fieldLen, _ := reader.FieldLength(field, docID)

			score := s.Score(fp.Frequency, fieldLen, boost)

			results = append(results, &plugin.DocumentScore{
				ID:    docID,
				Score: score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return &phraseSearcher{results: results}, nil
}

func containsPhrase(reader plugin.IndexReader, field, docID string, terms []string) bool {
	if len(terms) <= 1 {
		return true
	}

	positions := make([][]int, len(terms))
	for i, term := range terms {
		tv, err := reader.GetTermVector(field, docID, term)
		if err != nil || tv == nil {
			return false
		}
		for _, p := range tv.Positions {
			positions[i] = append(positions[i], p.Position)
		}
		sort.Ints(positions[i])
	}

	for _, startPos := range positions[0] {
		found := true
		for i := 1; i < len(terms); i++ {
			expectedPos := startPos + i
			idx := sort.SearchInts(positions[i], expectedPos)
			if idx >= len(positions[i]) || positions[i][idx] != expectedPos {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}

	return false
}

func (s *phraseSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *phraseSearcher) Count() int {
	return len(s.results)
}

// prefixSearcher searches for terms starting with a prefix.
type prefixSearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

func newPrefixSearcher(reader plugin.IndexReader, field, prefix string, boost float64, sf plugin.ScorerFactory) (*prefixSearcher, error) {
	terms, err := reader.TermsWithPrefix(field, prefix)
	if err != nil || len(terms) == 0 {
		return &prefixSearcher{}, nil
	}
	if len(terms) > maxMatchingTerms {
		terms = terms[:maxMatchingTerms]
	}

	docCount, _ := reader.DocCount()
	avgFieldLen, _ := reader.AverageFieldLength(field)

	docScores := make(map[string]float64)
	for _, term := range terms {
		postings, _ := reader.TermPostings(field, term)
		docFreq, _ := reader.DocumentFrequency(field, term)
		s := sf.NewScorer(docCount, docFreq, avgFieldLen)

		for _, p := range postings {
			fieldLen, _ := reader.FieldLength(field, p.DocID)
			if fieldLen == 0 {
				fieldLen = 1
			}
			score := s.Score(p.Frequency, fieldLen, boost)
			docScores[p.DocID] += score
		}
	}

	results := make([]*plugin.DocumentScore, 0, len(docScores))
	for docID, score := range docScores {
		results = append(results, &plugin.DocumentScore{ID: docID, Score: score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return &prefixSearcher{results: results}, nil
}

func (s *prefixSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *prefixSearcher) Count() int {
	return len(s.results)
}

// fuzzySearcher searches for terms within edit distance.
type fuzzySearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

const maxFuzziness = 2

// maxMatchingTerms is the maximum number of terms that prefix, fuzzy, and regexp
// searchers will expand to. This prevents unbounded work from broad patterns.
const maxMatchingTerms = 1000

func newFuzzySearcher(reader plugin.IndexReader, field, term string, fuzziness int, boost float64, sf plugin.ScorerFactory) (*fuzzySearcher, error) {
	if fuzziness > maxFuzziness {
		fuzziness = maxFuzziness
	}

	// Use BK-tree based fuzzy search (O(T^(k/log T)) instead of O(T)).
	matchingTerms, err := reader.FuzzyTerms(field, term, fuzziness)
	if err != nil {
		return &fuzzySearcher{}, nil
	}
	if len(matchingTerms) > maxMatchingTerms {
		matchingTerms = matchingTerms[:maxMatchingTerms]
	}

	if len(matchingTerms) == 0 {
		return &fuzzySearcher{}, nil
	}

	docCount, _ := reader.DocCount()
	avgFieldLen, _ := reader.AverageFieldLength(field)

	docScores := make(map[string]float64)
	for _, mt := range matchingTerms {
		postings, _ := reader.TermPostings(field, mt)
		docFreq, _ := reader.DocumentFrequency(field, mt)
		s := sf.NewScorer(docCount, docFreq, avgFieldLen)

		for _, p := range postings {
			fieldLen, _ := reader.FieldLength(field, p.DocID)
			if fieldLen == 0 {
				fieldLen = 1
			}
			score := s.Score(p.Frequency, fieldLen, boost)
			docScores[p.DocID] += score
		}
	}

	results := make([]*plugin.DocumentScore, 0, len(docScores))
	for docID, score := range docScores {
		results = append(results, &plugin.DocumentScore{ID: docID, Score: score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return &fuzzySearcher{results: results}, nil
}

func (s *fuzzySearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *fuzzySearcher) Count() int {
	return len(s.results)
}

// levenshteinDistance computes the edit distance between two strings.
func levenshteinDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}

// regexpSearcher searches for terms matching a regular expression.
type regexpSearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

func newRegexpSearcher(reader plugin.IndexReader, field, pattern string, boost float64, sf plugin.ScorerFactory) (*regexpSearcher, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	allTerms, err := reader.TermsForField(field)
	if err != nil {
		return &regexpSearcher{}, nil
	}

	var matchingTerms []string
	for _, t := range allTerms {
		if re.MatchString(t) {
			matchingTerms = append(matchingTerms, t)
		}
	}
	if len(matchingTerms) > maxMatchingTerms {
		matchingTerms = matchingTerms[:maxMatchingTerms]
	}

	if len(matchingTerms) == 0 {
		return &regexpSearcher{}, nil
	}

	docCount, _ := reader.DocCount()
	avgFieldLen, _ := reader.AverageFieldLength(field)

	docScores := make(map[string]float64)
	for _, mt := range matchingTerms {
		postings, _ := reader.TermPostings(field, mt)
		docFreq, _ := reader.DocumentFrequency(field, mt)
		s := sf.NewScorer(docCount, docFreq, avgFieldLen)

		for _, p := range postings {
			fieldLen, _ := reader.FieldLength(field, p.DocID)
			if fieldLen == 0 {
				fieldLen = 1
			}
			score := s.Score(p.Frequency, fieldLen, boost)
			docScores[p.DocID] += score
		}
	}

	results := make([]*plugin.DocumentScore, 0, len(docScores))
	for docID, score := range docScores {
		results = append(results, &plugin.DocumentScore{ID: docID, Score: score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return &regexpSearcher{results: results}, nil
}

func (s *regexpSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *regexpSearcher) Count() int {
	return len(s.results)
}

// numericRangeSearcher searches for documents with numeric values in range.
type numericRangeSearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

func newNumericRangeSearcher(reader plugin.IndexReader, field string, min, max *float64, boost float64) (*numericRangeSearcher, error) {
	docIDs, err := reader.NumericRange(field, min, max)
	if err != nil {
		return nil, err
	}

	results := make([]*plugin.DocumentScore, len(docIDs))
	for i, docID := range docIDs {
		results[i] = &plugin.DocumentScore{ID: docID, Score: boost}
	}

	return &numericRangeSearcher{results: results}, nil
}

func (s *numericRangeSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *numericRangeSearcher) Count() int {
	return len(s.results)
}

// dateRangeSearcher searches for documents with datetime values in range.
type dateRangeSearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

func newDateRangeSearcher(reader plugin.IndexReader, field string, start, end *time.Time, boost float64) (*dateRangeSearcher, error) {
	docIDs, err := reader.DateTimeRange(field, start, end)
	if err != nil {
		return nil, err
	}

	results := make([]*plugin.DocumentScore, len(docIDs))
	for i, docID := range docIDs {
		results[i] = &plugin.DocumentScore{ID: docID, Score: boost}
	}

	return &dateRangeSearcher{results: results}, nil
}

func (s *dateRangeSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *dateRangeSearcher) Count() int {
	return len(s.results)
}

// conjunctionSearcher requires all sub-queries to match.
type conjunctionSearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

func newConjunctionSearcher(reader plugin.IndexReader, field string, queries []Query, boost float64, sf plugin.ScorerFactory) (*conjunctionSearcher, error) {
	if len(queries) == 0 {
		return &conjunctionSearcher{}, nil
	}

	firstSearcher, err := queries[0].Searcher(reader, field, sf)
	if err != nil {
		return nil, err
	}

	candidates := make(map[string]float64)
	for {
		ds, err := firstSearcher.Next()
		if err != nil || ds == nil {
			break
		}
		candidates[ds.ID] = ds.Score
	}

	for i := 1; i < len(queries) && len(candidates) > 0; i++ {
		s, err := queries[i].Searcher(reader, field, sf)
		if err != nil {
			return nil, err
		}
		otherScores := make(map[string]float64)
		for {
			ds, err := s.Next()
			if err != nil || ds == nil {
				break
			}
			if _, ok := candidates[ds.ID]; ok {
				otherScores[ds.ID] = ds.Score
			}
		}
		for docID := range candidates {
			if otherScore, ok := otherScores[docID]; ok {
				candidates[docID] += otherScore
			} else {
				delete(candidates, docID)
			}
		}
	}

	results := make([]*plugin.DocumentScore, 0, len(candidates))
	for docID, score := range candidates {
		results = append(results, &plugin.DocumentScore{ID: docID, Score: score * boost})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return &conjunctionSearcher{results: results}, nil
}

func (s *conjunctionSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *conjunctionSearcher) Count() int {
	return len(s.results)
}

// disjunctionSearcher requires at least Min sub-queries to match.
type disjunctionSearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

func newDisjunctionSearcher(reader plugin.IndexReader, field string, queries []Query, minMatch int, boost float64, sf plugin.ScorerFactory) (*disjunctionSearcher, error) {
	if len(queries) == 0 {
		return &disjunctionSearcher{}, nil
	}

	docScores := make(map[string]float64)
	docCounts := make(map[string]int)

	for _, q := range queries {
		s, err := q.Searcher(reader, field, sf)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]bool)
		for {
			ds, err := s.Next()
			if err != nil || ds == nil {
				break
			}
			if !seen[ds.ID] {
				seen[ds.ID] = true
				docCounts[ds.ID]++
			}
			docScores[ds.ID] += ds.Score
		}
	}

	results := make([]*plugin.DocumentScore, 0)
	for docID, count := range docCounts {
		if count >= minMatch {
			results = append(results, &plugin.DocumentScore{
				ID:    docID,
				Score: docScores[docID] * boost,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return &disjunctionSearcher{results: results}, nil
}

func (s *disjunctionSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *disjunctionSearcher) Count() int {
	return len(s.results)
}

// booleanSearcher implements boolean query logic.
type booleanSearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

func newBooleanSearcher(reader plugin.IndexReader, field string, q *BooleanQuery, sf plugin.ScorerFactory) (*booleanSearcher, error) {
	mustNotDocs := make(map[string]bool)
	for _, mnq := range q.MustNot {
		s, err := mnq.Searcher(reader, field, sf)
		if err != nil {
			return nil, err
		}
		for {
			ds, err := s.Next()
			if err != nil || ds == nil {
				break
			}
			mustNotDocs[ds.ID] = true
		}
	}

	finalDocs := make(map[string]float64)

	if len(q.Must) > 0 {
		conj, err := newConjunctionSearcher(reader, field, q.Must, 1.0, sf)
		if err != nil {
			return nil, err
		}
		for {
			ds, err := conj.Next()
			if err != nil || ds == nil {
				break
			}
			if !mustNotDocs[ds.ID] {
				finalDocs[ds.ID] = ds.Score
			}
		}

		if len(q.Should) > 0 {
			shouldDocs := make(map[string]float64)
			disj, err := newDisjunctionSearcher(reader, field, q.Should, 1, 1.0, sf)
			if err != nil {
				return nil, err
			}
			for {
				ds, err := disj.Next()
				if err != nil || ds == nil {
					break
				}
				shouldDocs[ds.ID] = ds.Score
			}
			for docID := range finalDocs {
				if shouldScore, ok := shouldDocs[docID]; ok {
					finalDocs[docID] += shouldScore
				}
			}
		}
	} else if len(q.Should) > 0 {
		disj, err := newDisjunctionSearcher(reader, field, q.Should, 1, 1.0, sf)
		if err != nil {
			return nil, err
		}
		for {
			ds, err := disj.Next()
			if err != nil || ds == nil {
				break
			}
			if !mustNotDocs[ds.ID] {
				finalDocs[ds.ID] = ds.Score
			}
		}
	} else if len(q.MustNot) > 0 {
		if err := reader.ForEachDocID(func(docID string) bool {
			if !mustNotDocs[docID] {
				finalDocs[docID] = 1.0
			}
			return true
		}); err != nil {
			return nil, err
		}
	}

	results := make([]*plugin.DocumentScore, 0, len(finalDocs))
	for docID, score := range finalDocs {
		results = append(results, &plugin.DocumentScore{ID: docID, Score: score * q.Boost})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return &booleanSearcher{results: results}, nil
}

func (s *booleanSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *booleanSearcher) Count() int {
	return len(s.results)
}

// matchAllSearcher matches all documents using paginated iteration.
type matchAllSearcher struct {
	reader  plugin.IndexReader
	boost   float64
	buf     []string
	pos     int
	count   int
	counted bool
	done    bool
}

const matchAllPageSize = 256

func newMatchAllSearcher(reader plugin.IndexReader, boost float64) (*matchAllSearcher, error) {
	s := &matchAllSearcher{reader: reader, boost: boost}
	s.loadNextPage()
	return s, nil
}

func (s *matchAllSearcher) loadNextPage() {
	if s.done {
		return
	}
	startAfter := ""
	if len(s.buf) > 0 {
		startAfter = s.buf[len(s.buf)-1]
	}
	s.buf = s.buf[:0]
	s.pos = 0

	results, err := s.reader.ListDocIDs(startAfter, matchAllPageSize)
	if err != nil || len(results) == 0 {
		s.done = true
		return
	}
	s.buf = results
	if len(results) < matchAllPageSize {
		s.done = true
	}
}

func (s *matchAllSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.buf) {
		if s.done {
			return nil, nil
		}
		s.loadNextPage()
		if s.pos >= len(s.buf) {
			return nil, nil
		}
	}
	result := &plugin.DocumentScore{ID: s.buf[s.pos], Score: s.boost}
	s.pos++
	return result, nil
}

func (s *matchAllSearcher) Count() int {
	if !s.counted {
		count, _ := s.reader.DocCount()
		s.count = int(count)
		s.counted = true
	}
	return s.count
}

// emptySearcher returns no results.
type emptySearcher struct{}

func (s *emptySearcher) Next() (*plugin.DocumentScore, error) {
	return nil, nil
}

func (s *emptySearcher) Count() int {
	return 0
}

// knnSearcher performs k-nearest-neighbor vector search.
type knnSearcher struct {
	results []*plugin.DocumentScore
	pos     int
}

func newKNNSearcher(reader plugin.IndexReader, field string, queryVec []float32, k int, boost float64) (*knnSearcher, error) {
	if k <= 0 {
		k = 10
	}

	// Try HNSW approximate search first (O(log N) vs O(N)).
	if ids, sims, ok := reader.HNSWSearch(field, queryVec, k); ok && len(ids) > 0 {
		results := make([]*plugin.DocumentScore, len(ids))
		maxSim := sims[0]
		for i, id := range ids {
			score := sims[i]
			if maxSim > 0 {
				score = (score / maxSim) * boost
			}
			results[i] = &plugin.DocumentScore{ID: id, Score: score}
		}
		return &knnSearcher{results: results}, nil
	}

	// Fallback: brute-force scan.
	h := &scoreMinHeap{}
	heap.Init(h)

	err := reader.ForEachVector(field, func(docID string, vec []float32) bool {
		similarity := cosineSimilarity(queryVec, vec)
		if similarity <= 0 {
			return true
		}
		if h.Len() < k {
			heap.Push(h, plugin.DocumentScore{ID: docID, Score: similarity})
		} else if similarity > (*h)[0].Score {
			(*h)[0] = plugin.DocumentScore{ID: docID, Score: similarity}
			heap.Fix(h, 0)
		}
		return true
	})
	if err != nil || h.Len() == 0 {
		return &knnSearcher{}, nil
	}

	n := h.Len()
	results := make([]*plugin.DocumentScore, n)
	for i := n - 1; i >= 0; i-- {
		ds := heap.Pop(h).(plugin.DocumentScore)
		results[i] = &ds
	}

	maxScore := results[0].Score
	for _, r := range results {
		if maxScore > 0 {
			r.Score = (r.Score / maxScore) * boost
		}
	}

	return &knnSearcher{results: results}, nil
}

func (s *knnSearcher) Next() (*plugin.DocumentScore, error) {
	if s.pos >= len(s.results) {
		return nil, nil
	}
	result := s.results[s.pos]
	s.pos++
	return result, nil
}

func (s *knnSearcher) Count() int {
	return len(s.results)
}
