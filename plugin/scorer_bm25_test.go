package plugin

import (
	"math"
	"testing"
)

// newTestBM25Scorer creates a bm25Scorer with default K1/B for testing.
func newTestBM25Scorer(docCount, docFreq uint64, avgFieldLength float64) *bm25Scorer {
	f := &BM25ScorerFactory{K1: 1.2, B: 0.75}
	return f.NewScorer(docCount, docFreq, avgFieldLength).(*bm25Scorer)
}

// --- BM25Scorer.Score tests ---

func TestBM25Scorer_Score_Basic(t *testing.T) {
	// Standard case: 100 docs, term in 10, avg length 50
	s := newTestBM25Scorer(100, 10, 50.0)
	score := s.Score(3, 50, 1.0)

	// IDF = log(1 + (100 - 10 + 0.5) / (10 + 0.5)) = log(1 + 90.5/10.5) = log(1 + 8.619...) = log(9.619...)
	expectedIDF := math.Log(1 + (100.0-10.0+0.5)/(10.0+0.5))
	// TF component: dl == avgdl so normalization factor = 1
	// tfNorm = (3 * 2.2) / (3 + 1.2 * (1 - 0.75 + 0.75*1)) = 6.6 / (3 + 1.2) = 6.6 / 4.2
	expectedTF := (3.0 * 2.2) / (3.0 + 1.2*(1.0-0.75+0.75*50.0/50.0))
	expected := expectedIDF * expectedTF * 1.0

	if math.Abs(score-expected) > 1e-10 {
		t.Errorf("expected %f, got %f", expected, score)
	}
}

func TestBM25Scorer_Score_WithBoost(t *testing.T) {
	s := newTestBM25Scorer(100, 10, 50.0)
	score1 := s.Score(3, 50, 1.0)
	score2 := s.Score(3, 50, 2.5)

	if math.Abs(score2-score1*2.5) > 1e-10 {
		t.Errorf("boost should scale linearly: score1=%f, score2=%f", score1, score2)
	}
}

func TestBM25Scorer_Score_ZeroDocCount(t *testing.T) {
	s := newTestBM25Scorer(0, 0, 50.0)
	score := s.Score(3, 50, 1.0)
	// IDF = log(1 + (0 - 0 + 0.5)/(0 + 0.5)) = log(1+1) = log(2)
	if math.IsNaN(score) || math.IsInf(score, 0) {
		t.Errorf("should not produce NaN/Inf with zero doc count, got %f", score)
	}
}

func TestBM25Scorer_Score_ZeroAvgLength(t *testing.T) {
	s := newTestBM25Scorer(100, 10, 0)
	score := s.Score(3, 50, 1.0)
	// avgdl should be treated as 1 when 0
	if math.IsNaN(score) || math.IsInf(score, 0) {
		t.Errorf("should not produce NaN/Inf with zero avg length, got %f", score)
	}
	// Verify it uses avgdl=1 fallback
	s2 := newTestBM25Scorer(100, 10, 1.0)
	score2 := s2.Score(3, 50, 1.0)
	if math.Abs(score-score2) > 1e-10 {
		t.Errorf("zero avgdl should behave like avgdl=1: got %f vs %f", score, score2)
	}
}

func TestBM25Scorer_Score_ZeroTermFreq(t *testing.T) {
	s := newTestBM25Scorer(100, 10, 50.0)
	score := s.Score(0, 50, 1.0)
	if score != 0 {
		t.Errorf("expected 0 for zero term freq, got %f", score)
	}
}

func TestBM25Scorer_Score_LongerDocScoresLower(t *testing.T) {
	s := newTestBM25Scorer(100, 10, 50.0)
	scoreShort := s.Score(3, 20, 1.0)
	scoreLong := s.Score(3, 200, 1.0)
	if scoreLong >= scoreShort {
		t.Errorf("longer doc should score lower: short=%f, long=%f", scoreShort, scoreLong)
	}
}

func TestBM25Scorer_Score_HigherTermFreqScoresHigher(t *testing.T) {
	s := newTestBM25Scorer(100, 10, 50.0)
	scoreLow := s.Score(1, 50, 1.0)
	scoreHigh := s.Score(10, 50, 1.0)
	if scoreHigh <= scoreLow {
		t.Errorf("higher term freq should score higher: low=%f, high=%f", scoreLow, scoreHigh)
	}
}

func TestBM25Scorer_Score_RarerTermScoresHigher(t *testing.T) {
	sRare := newTestBM25Scorer(100, 2, 50.0)
	sCommon := newTestBM25Scorer(100, 80, 50.0)
	scoreRare := sRare.Score(3, 50, 1.0)
	scoreCommon := sCommon.Score(3, 50, 1.0)
	if scoreCommon >= scoreRare {
		t.Errorf("rarer term should score higher: rare=%f, common=%f", scoreRare, scoreCommon)
	}
}

func TestNewBM25Scorer_Defaults(t *testing.T) {
	s := newTestBM25Scorer(100, 10, 50.0)
	if s.K1 != 1.2 {
		t.Errorf("expected K1=1.2, got %f", s.K1)
	}
	// Verify pre-computed IDF
	expectedIDF := math.Log(1 + (100.0-10.0+0.5)/(10.0+0.5))
	if math.Abs(s.idf-expectedIDF) > 1e-10 {
		t.Errorf("expected idf=%f, got %f", expectedIDF, s.idf)
	}
	if s.k1Plus1 != 2.2 {
		t.Errorf("expected k1Plus1=2.2, got %f", s.k1Plus1)
	}
}
