package index

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
