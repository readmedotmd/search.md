package plugin

import "math"

// BM25ScorerFactory creates BM25 scorers.
type BM25ScorerFactory struct {
	K1 float64
	B  float64
}

// DefaultBM25 returns a BM25ScorerFactory with standard parameters.
func DefaultBM25() *BM25ScorerFactory {
	return &BM25ScorerFactory{K1: 1.2, B: 0.75}
}

func (f *BM25ScorerFactory) Name() string { return "bm25" }

func (f *BM25ScorerFactory) NewScorer(docCount, docFreq uint64, avgFieldLength float64) Scorer {
	// Pre-compute IDF since it's constant per term.
	n := float64(docFreq)
	N := float64(docCount)
	idf := math.Log(1 + (N-n+0.5)/(n+0.5))

	avgdl := avgFieldLength
	if avgdl == 0 {
		avgdl = 1
	}
	// Pre-compute the base normalization factor: k1 * (1 - b)
	k1TimesOneMinusB := f.K1 * (1 - f.B)
	k1TimesBDivAvgdl := f.K1 * f.B / avgdl

	return &bm25Scorer{
		K1:                f.K1,
		idf:               idf,
		k1Plus1:           f.K1 + 1,
		k1TimesOneMinusB:  k1TimesOneMinusB,
		k1TimesBDivAvgdl:  k1TimesBDivAvgdl,
	}
}

// bm25Scorer implements Okapi BM25 scoring with pre-computed constants.
type bm25Scorer struct {
	K1                float64
	idf               float64 // pre-computed IDF
	k1Plus1           float64 // k1 + 1
	k1TimesOneMinusB  float64 // k1 * (1 - b)
	k1TimesBDivAvgdl  float64 // k1 * b / avgdl
}

// Score calculates the BM25 score for a document.
func (s *bm25Scorer) Score(termFreq int, fieldLength int, boost float64) float64 {
	tf := float64(termFreq)
	dl := float64(fieldLength)
	tfNorm := (tf * s.k1Plus1) / (tf + s.k1TimesOneMinusB + s.k1TimesBDivAvgdl*dl)
	return s.idf * tfNorm * boost
}
