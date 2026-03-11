package query

import (
	"math"
	"testing"
)

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	a := []float32{1, 2, 3}
	result := cosineSimilarity(a, a)
	if math.Abs(result-1.0) > 1e-6 {
		t.Errorf("expected 1.0 for identical vectors, got %f", result)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	result := cosineSimilarity(a, b)
	if math.Abs(result) > 1e-6 {
		t.Errorf("expected 0.0 for orthogonal vectors, got %f", result)
	}
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	result := cosineSimilarity(a, b)
	if math.Abs(result-(-1.0)) > 1e-6 {
		t.Errorf("expected -1.0 for opposite vectors, got %f", result)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	if r := cosineSimilarity(a, b); r != 0 {
		t.Errorf("expected 0 when first vector is zero, got %f", r)
	}
	if r := cosineSimilarity(b, a); r != 0 {
		t.Errorf("expected 0 when second vector is zero, got %f", r)
	}
	if r := cosineSimilarity(a, a); r != 0 {
		t.Errorf("expected 0 when both vectors are zero, got %f", r)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	result := cosineSimilarity(a, b)
	if result != 0 {
		t.Errorf("expected 0 for different length vectors, got %f", result)
	}
}

func TestCosineSimilarity_Empty(t *testing.T) {
	result := cosineSimilarity([]float32{}, []float32{})
	if result != 0 {
		t.Errorf("expected 0 for empty vectors, got %f", result)
	}
}

func TestCosineSimilarity_KnownValue(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 1}
	// cos(45) = 1/sqrt(2) ~ 0.7071
	expected := 1.0 / math.Sqrt(2.0)
	result := cosineSimilarity(a, b)
	if math.Abs(result-expected) > 1e-6 {
		t.Errorf("expected %f, got %f", expected, result)
	}
}
