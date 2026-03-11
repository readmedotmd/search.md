package plugin

import "math"

// TFIDFScorerFactory creates TF-IDF scorers.
type TFIDFScorerFactory struct{}

func (f *TFIDFScorerFactory) Name() string { return "tfidf" }

func (f *TFIDFScorerFactory) NewScorer(docCount, docFreq uint64, avgFieldLength float64) Scorer {
	return &tfidfScorer{
		docCount: docCount,
		docFreq:  docFreq,
	}
}

type tfidfScorer struct {
	docCount uint64
	docFreq  uint64
}

func (s *tfidfScorer) Score(termFreq int, fieldLength int, boost float64) float64 {
	tf := math.Log(1 + float64(termFreq))

	N := float64(s.docCount)
	n := float64(s.docFreq)
	idf := 0.0
	if n > 0 {
		idf = math.Log(N / n)
	}

	// Length normalization
	norm := 1.0
	if fieldLength > 0 {
		norm = 1.0 / math.Sqrt(float64(fieldLength))
	}

	return tf * idf * norm * boost
}
