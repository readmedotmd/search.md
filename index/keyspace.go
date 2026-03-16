package index

import (
	"strings"
	"time"
)

// keyspace.go centralizes KV key construction for the search index.
// All key format conventions are defined here so that index.go and
// field indexer files reference helpers instead of inlining prefixes.

// --- Document keys: d/{id} ---

// docKey returns the key for a stored document.
func docKey(id string) string {
	return prefixDoc + id
}

// --- Term posting keys: t/{field}/{term}/{id} ---

// termKey returns the key for a term posting entry.
func termKey(field, term, id string) string {
	return prefixTerm + field + "/" + term + "/" + id
}

// termFieldPrefix returns the prefix for all postings of a term in a field.
func termFieldPrefix(field, term string) string {
	return prefixTerm + field + "/" + term + "/"
}

// termFieldOnlyPrefix returns the prefix for all terms in a field.
func termFieldOnlyPrefix(field string) string {
	return prefixTerm + field + "/"
}

// --- Field length keys: f/{field}/{id} ---

// fieldLenKey returns the key for the token count of a field in a document.
func fieldLenKey(field, id string) string {
	return prefixFieldLen + field + "/" + id
}

// --- Field length sum keys: m/field_len_sum/{field} ---

// fieldLenSumKey returns the metadata key for the sum of field lengths.
func fieldLenSumKey(field string) string {
	return prefixFieldLenSum + field
}

// --- Document frequency keys: n/{field}/{term} ---

// docFreqKey returns the key for the document frequency of a term in a field.
func docFreqKey(field, term string) string {
	return prefixDocFreq + field + "/" + term
}

// --- Term vector keys: tv/{field}/{id}/{term} ---

// termVecKey returns the key for term vector positions.
func termVecKey(field, id, term string) string {
	return prefixTermVec + field + "/" + id + "/" + term
}

// --- Vector keys: v/{field}/{id} ---

// vectorKey returns the key for a vector embedding.
func vectorKey(field, id string) string {
	return prefixVector + field + "/" + id
}

// vectorFieldPrefix returns the prefix for all vectors in a field.
func vectorFieldPrefix(field string) string {
	return prefixVector + field + "/"
}

// --- Numeric keys: num/{field}/{id} ---

// numericKey returns the key for a numeric field value.
func numericKey(field, id string) string {
	return prefixNumeric + field + "/" + id
}

// numericFieldPrefix returns the prefix for all numeric values in a field.
func numericFieldPrefix(field string) string {
	return prefixNumeric + field + "/"
}

// --- DateTime keys: dt/{field}/{id} ---

// dateTimeKey returns the key for a datetime field value.
func dateTimeKey(field, id string) string {
	return prefixDateTime + field + "/" + id
}

// dateTimeFieldPrefix returns the prefix for all datetime values in a field.
func dateTimeFieldPrefix(field string) string {
	return prefixDateTime + field + "/"
}

// --- Sorted numeric keys: ns/{field}/{sortableValue}/{id} ---

// numericSortedKey returns the sorted-index key for a numeric value.
func numericSortedKey(field string, val float64, id string) string {
	return prefixNumericSorted + field + "/" + sortableFloat64Key(val) + "/" + id
}

// numericSortedFieldPrefix returns the prefix for all sorted numeric entries in a field.
func numericSortedFieldPrefix(field string) string {
	return prefixNumericSorted + field + "/"
}

// numericSortedKeyBefore returns a key just before the given value for StartAfter.
func numericSortedKeyBefore(field string, val float64) string {
	return prefixNumericSorted + field + "/" + sortableFloat64Key(val)
}

// numericSortedKeyAfter returns a key just after the given value for max bound check.
func numericSortedKeyAfter(field string, val float64) string {
	return prefixNumericSorted + field + "/" + sortableFloat64Key(val) + "/\xff"
}

// numericSortedDocID extracts the docID from a sorted numeric key.
func numericSortedDocID(key string) string {
	// key: ns/{field}/{16hexchars}/{docID}
	i := strings.LastIndex(key, "/")
	if i < 0 {
		return ""
	}
	return key[i+1:]
}

// --- Sorted datetime keys: ds/{field}/{sortableNanos}/{id} ---

// dateTimeSortedKey returns the sorted-index key for a datetime value.
func dateTimeSortedKey(field string, nanos int64, id string) string {
	return prefixDateTimeSorted + field + "/" + sortableFloat64Key(float64(nanos)) + "/" + id
}

// dateTimeSortedFieldPrefix returns the prefix for all sorted datetime entries in a field.
func dateTimeSortedFieldPrefix(field string) string {
	return prefixDateTimeSorted + field + "/"
}

// dateTimeSortedKeyBefore returns a key just before the given time for StartAfter.
func dateTimeSortedKeyBefore(field string, t time.Time) string {
	return prefixDateTimeSorted + field + "/" + sortableFloat64Key(float64(t.UnixNano()))
}

// dateTimeSortedKeyAfter returns a key just after the given time for max bound check.
func dateTimeSortedKeyAfter(field string, t time.Time) string {
	return prefixDateTimeSorted + field + "/" + sortableFloat64Key(float64(t.UnixNano())) + "/\xff"
}

// dateTimeSortedDocID extracts the docID from a sorted datetime key.
func dateTimeSortedDocID(key string) string {
	i := strings.LastIndex(key, "/")
	if i < 0 {
		return ""
	}
	return key[i+1:]
}

// --- Boolean keys: bool/{field}/{id} ---

// boolKey returns the key for a boolean field value.
func boolKey(field, id string) string {
	return prefixBool + field + "/" + id
}

// --- Reverse index keys: ri/{id} ---

// revIdxKey returns the key for a document's reverse index entry.
func revIdxKey(id string) string {
	return prefixRevIdx + id
}
