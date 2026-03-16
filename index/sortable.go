package index

import (
	"encoding/binary"
	"fmt"
	"math"
)

// sortableFloat64Key encodes a float64 as a 16-char hex string that sorts
// lexicographically in the same order as the float64 values. This enables
// range queries using KV prefix/startAfter scans instead of full scans.
//
// The encoding flips all bits for negative numbers (so they sort before
// positives) and flips only the sign bit for positive numbers.
func sortableFloat64Key(f float64) string {
	bits := math.Float64bits(f)
	if f >= 0 {
		bits ^= 1 << 63 // flip sign bit
	} else {
		bits ^= math.MaxUint64 // flip all bits
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], bits)
	return fmt.Sprintf("%016x", buf)
}

// sortableFloat64FromKey decodes a sortable hex key back to float64.
func sortableFloat64FromKey(s string) (float64, error) {
	if len(s) != 16 {
		return 0, fmt.Errorf("invalid sortable key length: %d", len(s))
	}
	var buf [8]byte
	_, err := fmt.Sscanf(s, "%016x", &buf)
	if err != nil {
		return 0, err
	}
	bits := binary.BigEndian.Uint64(buf[:])
	if bits&(1<<63) != 0 {
		bits ^= 1 << 63 // positive: flip sign bit back
	} else {
		bits ^= math.MaxUint64 // negative: flip all bits back
	}
	return math.Float64frombits(bits), nil
}
