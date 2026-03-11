package index

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/readmedotmd/search.md/analysis"
	"github.com/readmedotmd/search.md/document"
	"github.com/readmedotmd/search.md/registry"
)

// TextFieldIndexer handles indexing for text, keyword, and code fields.
type TextFieldIndexer struct{}

func (fi *TextFieldIndexer) Type() string { return "text" }

func (fi *TextFieldIndexer) IndexField(helpers IndexHelpers, docID string, field *document.Field) (*RevIdxEntry, error) {
	text := field.TextValue()
	if text == "" {
		return nil, nil
	}

	analyzer, err := registry.AnalyzerByName(field.Analyzer)
	if err != nil {
		analyzer, _ = registry.AnalyzerByName("standard")
	}
	if analyzer == nil {
		return nil, fmt.Errorf("no analyzer available: %q and standard fallback both failed", field.Analyzer)
	}

	tokens := analyzer.Analyze([]byte(text))
	if len(tokens) == 0 {
		return nil, nil
	}

	store := helpers.Store()
	tfs := analysis.TokenFrequencies(tokens, field.IncludeTermVectors)
	fieldLen := len(tokens)
	norm := 1.0 / math.Sqrt(float64(fieldLen))

	indexedTerms := make([]string, 0, len(tfs))

	for term, tf := range tfs {
		indexedTerms = append(indexedTerms, term)

		posting := Posting{
			DocID:     docID,
			Frequency: tf.Frequency,
			Norm:      norm,
		}
		postingJSON, err := json.Marshal(posting)
		if err != nil {
			return nil, err
		}

		key := prefixTerm + field.Name + "/" + term + "/" + docID
		if err := store.Set(key, string(postingJSON)); err != nil {
			return nil, err
		}

		if err := helpers.IncrementDocFreq(field.Name, term); err != nil {
			return nil, err
		}

		if field.IncludeTermVectors && len(tf.Positions) > 0 {
			tv := TermVector{Positions: make([]analysis.TokenPosition, len(tf.Positions))}
			for i, p := range tf.Positions {
				tv.Positions[i] = *p
			}
			tvJSON, err := json.Marshal(tv)
			if err != nil {
				return nil, err
			}
			tvKey := prefixTermVec + field.Name + "/" + docID + "/" + term
			if err := store.Set(tvKey, string(tvJSON)); err != nil {
				return nil, err
			}
		}
	}

	// Store field length
	lenKey := prefixFieldLen + field.Name + "/" + docID
	if err := store.Set(lenKey, strconv.Itoa(fieldLen)); err != nil {
		return nil, err
	}

	// Update field length sum
	sumKey := prefixFieldLenSum + field.Name
	currentSum, _ := helpers.GetInt64(sumKey)
	if err := store.Set(sumKey, strconv.FormatInt(currentSum+int64(fieldLen), 10)); err != nil {
		return nil, err
	}

	return &RevIdxEntry{Field: field.Name, Type: "text", Terms: indexedTerms}, nil
}

func (fi *TextFieldIndexer) DeleteField(helpers IndexHelpers, docID string, entry RevIdxEntry) error {
	store := helpers.Store()
	for _, term := range entry.Terms {
		if err := deleteKey(store, prefixTerm+entry.Field+"/"+term+"/"+docID); err != nil {
			return err
		}
		if err := helpers.DecrementDocFreq(entry.Field, term); err != nil {
			return err
		}
		if err := deleteKey(store, prefixTermVec+entry.Field+"/"+docID+"/"+term); err != nil {
			return err
		}
	}
	// Delete field length and update sum
	lenKey := prefixFieldLen + entry.Field + "/" + docID
	if lenVal, err := store.Get(lenKey); err == nil {
		fieldLen, _ := strconv.Atoi(lenVal)
		sumKey := prefixFieldLenSum + entry.Field
		currentSum, _ := helpers.GetInt64(sumKey)
		newSum := currentSum - int64(fieldLen)
		if newSum < 0 {
			newSum = 0
		}
		if err := store.Set(sumKey, strconv.FormatInt(newSum, 10)); err != nil {
			return err
		}
		if err := deleteKey(store, lenKey); err != nil {
			return err
		}
	}
	return nil
}
