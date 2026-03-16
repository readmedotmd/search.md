package codesearch

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	storemd "github.com/readmedotmd/store.md"

	"github.com/readmedotmd/search.md/document"
	"github.com/readmedotmd/search.md/index"
)

// Sub-field suffixes appended to the base field name.
const (
	SubSym   = ".sym"   // symbol names
	SubKind  = ".kind"  // symbol kinds (function, class, etc.)
	SubScope = ".scope" // symbol scopes (parent class, module, etc.)
)

// KV store prefixes (matching index package conventions).
const (
	pfxTerm        = "t/"
	pfxFieldLen    = "f/"
	pfxFieldLenSum = "m/field_len_sum/"
	pfxDocFreq     = "n/"
	pfxSymData     = "sym/" // sym/{field}/{docID} -> symbols JSON
)

// SymbolFieldIndexer extracts symbols from code fields and indexes them
// as searchable sub-fields. It implements index.FieldIndexer for the "symbol" type.
//
// For a field named "code", the indexer creates these searchable sub-fields:
//   - code.sym:   symbol names (e.g., "getUserById", "User")
//   - code.kind:  symbol kinds (e.g., "function", "struct")
//   - code.scope: symbol scopes (e.g., parent class name)
//
// Query these using standard term queries:
//
//	query.NewTermQuery("getuser").SetField("code.sym")
//	query.NewTermQuery("function").SetField("code.kind")
type SymbolFieldIndexer struct {
	Extractor SymbolExtractor
}

func (fi *SymbolFieldIndexer) Type() string { return "symbol" }

func (fi *SymbolFieldIndexer) IndexField(helpers index.IndexHelpers, docID string, field *document.Field) (*index.RevIdxEntry, error) {
	source := field.TextValue()
	if source == "" {
		return nil, nil
	}

	symbols, err := fi.Extractor.Extract([]byte(source), field.Language)
	if err != nil {
		return nil, err
	}
	if len(symbols) == 0 {
		return nil, nil
	}

	store := helpers.Store()
	ctx := context.Background()

	// Aggregate frequencies per sub-field.
	nameFreqs := make(map[string]int)
	kindFreqs := make(map[string]int)
	scopeFreqs := make(map[string]int)

	for _, sym := range symbols {
		if name := strings.ToLower(sym.Name); name != "" {
			nameFreqs[name]++
		}
		if kind := string(sym.Kind); kind != "" {
			kindFreqs[kind]++
		}
		if scope := strings.ToLower(sym.Scope); scope != "" {
			scopeFreqs[scope]++
		}
	}

	symField := field.Name + SubSym
	kindField := field.Name + SubKind
	scopeField := field.Name + SubScope

	var allTerms []string
	totalTokens := 0

	// Index symbol names.
	for term, freq := range nameFreqs {
		totalTokens += freq
		if err := indexPosting(store, helpers, symField, term, docID, freq, len(nameFreqs)); err != nil {
			return nil, err
		}
		allTerms = append(allTerms, "sym:"+term)
	}

	// Index symbol kinds.
	for term, freq := range kindFreqs {
		if err := indexPosting(store, helpers, kindField, term, docID, freq, len(kindFreqs)); err != nil {
			return nil, err
		}
		allTerms = append(allTerms, "kind:"+term)
	}

	// Index scopes.
	for term, freq := range scopeFreqs {
		if err := indexPosting(store, helpers, scopeField, term, docID, freq, len(scopeFreqs)); err != nil {
			return nil, err
		}
		allTerms = append(allTerms, "scope:"+term)
	}

	// Store field length for BM25 scoring.
	if totalTokens > 0 {
		lenKey := pfxFieldLen + symField + "/" + docID
		if err := store.Set(ctx, lenKey, strconv.Itoa(totalTokens)); err != nil {
			return nil, err
		}
		sumKey := pfxFieldLenSum + symField
		currentSum, _ := helpers.GetInt64(sumKey)
		if err := store.Set(ctx, sumKey, strconv.FormatInt(currentSum+int64(totalTokens), 10)); err != nil {
			return nil, err
		}
	}

	// Store extracted symbols JSON for retrieval.
	if field.Store {
		symJSON, err := json.Marshal(symbols)
		if err != nil {
			return nil, err
		}
		if err := store.Set(ctx, pfxSymData+field.Name+"/"+docID, string(symJSON)); err != nil {
			return nil, err
		}
		allTerms = append(allTerms, "_stored")
	}

	return &index.RevIdxEntry{
		Field: field.Name,
		Type:  "symbol",
		Terms: allTerms,
	}, nil
}

func (fi *SymbolFieldIndexer) DeleteField(helpers index.IndexHelpers, docID string, entry index.RevIdxEntry) error {
	store := helpers.Store()
	ctx := context.Background()

	symField := entry.Field + SubSym
	kindField := entry.Field + SubKind
	scopeField := entry.Field + SubScope

	for _, term := range entry.Terms {
		if term == "_stored" {
			store.Delete(ctx, pfxSymData+entry.Field+"/"+docID)
			continue
		}

		parts := strings.SplitN(term, ":", 2)
		if len(parts) != 2 {
			continue
		}

		var field string
		switch parts[0] {
		case "sym":
			field = symField
		case "kind":
			field = kindField
		case "scope":
			field = scopeField
		default:
			continue
		}

		store.Delete(ctx, pfxTerm+field+"/"+parts[1]+"/"+docID)
		helpers.DecrementDocFreq(field, parts[1])
	}

	// Update field length sum and delete field length.
	lenKey := pfxFieldLen + symField + "/" + docID
	if lenVal, err := store.Get(ctx, lenKey); err == nil {
		fieldLen, _ := strconv.Atoi(lenVal)
		sumKey := pfxFieldLenSum + symField
		currentSum, _ := helpers.GetInt64(sumKey)
		newSum := currentSum - int64(fieldLen)
		if newSum < 0 {
			newSum = 0
		}
		store.Set(ctx, sumKey, strconv.FormatInt(newSum, 10))
		store.Delete(ctx, lenKey)
	}

	return nil
}

// indexPosting writes a single term posting to the store.
func indexPosting(store storemd.Store, helpers index.IndexHelpers, field, term, docID string, freq, fieldLen int) error {
	norm := 1.0 / math.Sqrt(float64(fieldLen))
	posting := struct {
		DocID     string  `json:"d"`
		Frequency int     `json:"f"`
		Norm      float64 `json:"n"`
	}{docID, freq, norm}

	data, err := json.Marshal(posting)
	if err != nil {
		return err
	}
	key := pfxTerm + field + "/" + term + "/" + docID
	if err := store.Set(context.Background(), key, string(data)); err != nil {
		return err
	}
	return helpers.IncrementDocFreq(field, term)
}

// GetSymbols retrieves the stored symbols JSON for a document field.
// Returns nil if no symbols were stored.
func GetSymbols(helpers index.IndexHelpers, field, docID string) ([]Symbol, error) {
	val, err := helpers.Store().Get(context.Background(), pfxSymData+field+"/"+docID)
	if err != nil {
		return nil, err
	}
	var symbols []Symbol
	if err := json.Unmarshal([]byte(val), &symbols); err != nil {
		return nil, err
	}
	return symbols, nil
}
