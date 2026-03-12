package query

import (
	"math"
	"testing"
)

func makeVector(dims int) []float32 {
	v := make([]float32, dims)
	for i := range v {
		// Use a deterministic pattern that produces non-trivial values.
		v[i] = float32(math.Sin(float64(i)))
	}
	return v
}

func BenchmarkCosineSimilarity_128(b *testing.B) {
	a := makeVector(128)
	c := makeVector(128)
	// Shift c slightly so it is not identical to a.
	for i := range c {
		c[i] += 0.1
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cosineSimilarity(a, c)
	}
}

func BenchmarkCosineSimilarity_768(b *testing.B) {
	a := makeVector(768)
	c := makeVector(768)
	for i := range c {
		c[i] += 0.1
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cosineSimilarity(a, c)
	}
}
