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
	return &bm25Scorer{
		K1:             f.K1,
		B:              f.B,
		DocCount:       docCount,
		DocFreq:        docFreq,
		AvgFieldLength: avgFieldLength,
	}
}

// bm25Scorer implements Okapi BM25 scoring.
type bm25Scorer struct {
	// BM25 parameters
	K1 float64 // term frequency saturation parameter (default 1.2)
	B  float64 // field length normalization parameter (default 0.75)

	// Index statistics
	DocCount       uint64  // total number of documents
	DocFreq        uint64  // number of documents containing the term
	AvgFieldLength float64 // average field length
}

// Score calculates the BM25 score for a document.
func (s *bm25Scorer) Score(termFreq int, fieldLength int, boost float64) float64 {
	// IDF component: log(1 + (N - n + 0.5) / (n + 0.5))
	n := float64(s.DocFreq)
	N := float64(s.DocCount)
	idf := math.Log(1 + (N-n+0.5)/(n+0.5))

	// TF component: (tf * (k1 + 1)) / (tf + k1 * (1 - b + b * dl/avgdl))
	tf := float64(termFreq)
	dl := float64(fieldLength)
	avgdl := s.AvgFieldLength
	if avgdl == 0 {
		avgdl = 1
	}
	tfNorm := (tf * (s.K1 + 1)) / (tf + s.K1*(1-s.B+s.B*dl/avgdl))

	return idf * tfNorm * boost
}
