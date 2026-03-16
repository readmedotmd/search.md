package codesearch

import (
	"context"
	"strings"
	"testing"

	storemd "github.com/readmedotmd/store.md"
	"github.com/readmedotmd/store.md/backend/memory"

	"github.com/readmedotmd/search.md/document"
	"github.com/readmedotmd/search.md/index"
)

func newMemStore() *memory.StoreMemory { return memory.New() }

// testHelpers wraps memory.StoreMemory to satisfy index.IndexHelpers.
type testHelpers struct {
	store *memory.StoreMemory
}

func (h *testHelpers) Store() storemd.Store                      { return h.store }
func (h *testHelpers) IncrementDocFreq(field, term string) error { return nil }
func (h *testHelpers) DecrementDocFreq(field, term string) error { return nil }
func (h *testHelpers) GetDocCount() (uint64, error)              { return 1, nil }
func (h *testHelpers) GetInt64(key string) (int64, error) {
	v, err := h.store.Get(context.Background(), key)
	if err != nil {
		return 0, nil
	}
	var n int64
	for _, c := range v {
		n = n*10 + int64(c-'0')
	}
	return n, nil
}

func TestSymbolFieldIndexer_IndexAndDelete(t *testing.T) {
	ms := newMemStore()
	helpers := &testHelpers{store: ms}
	fi := &SymbolFieldIndexer{Extractor: NewRegexExtractor()}

	field := &document.Field{
		Name:     "code",
		Type:     document.FieldTypeSymbol,
		Value:    "package main\n\nfunc Hello() {}\ntype World struct{}\n",
		Store:    true,
		Index:    true,
		Language: "go",
	}

	entry, err := fi.IndexField(helpers, "doc1", field)
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil {
		t.Fatal("expected non-nil RevIdxEntry")
	}
	if entry.Type != "symbol" {
		t.Errorf("expected type 'symbol', got %q", entry.Type)
	}
	if entry.Field != "code" {
		t.Errorf("expected field 'code', got %q", entry.Field)
	}

	// Verify postings were written.
	val, err := ms.Get(context.Background(), "t/code.sym/hello/doc1")
	if err != nil {
		t.Error("expected posting for hello symbol")
	}
	if !strings.Contains(val, `"d":"doc1"`) {
		t.Errorf("unexpected posting value: %s", val)
	}

	// Verify kind posting.
	_, err = ms.Get(context.Background(), "t/code.kind/function/doc1")
	if err != nil {
		t.Error("expected posting for function kind")
	}
	_, err = ms.Get(context.Background(), "t/code.kind/struct/doc1")
	if err != nil {
		t.Error("expected posting for struct kind")
	}

	// Verify stored symbols.
	_, err = ms.Get(context.Background(), "sym/code/doc1")
	if err != nil {
		t.Error("expected stored symbols JSON")
	}

	// Verify field length.
	_, err = ms.Get(context.Background(), "f/code.sym/doc1")
	if err != nil {
		t.Error("expected field length entry")
	}

	// Now delete.
	err = fi.DeleteField(helpers, "doc1", *entry)
	if err != nil {
		t.Fatal(err)
	}

	// Verify postings removed.
	_, err = ms.Get(context.Background(), "t/code.sym/hello/doc1")
	if err == nil {
		t.Error("expected posting to be deleted")
	}
	_, err = ms.Get(context.Background(), "t/code.kind/function/doc1")
	if err == nil {
		t.Error("expected kind posting to be deleted")
	}
	_, err = ms.Get(context.Background(), "sym/code/doc1")
	if err == nil {
		t.Error("expected stored symbols to be deleted")
	}
	_, err = ms.Get(context.Background(), "f/code.sym/doc1")
	if err == nil {
		t.Error("expected field length to be deleted")
	}
}

func TestSymbolFieldIndexer_EmptySource(t *testing.T) {
	ms := newMemStore()
	helpers := &testHelpers{store: ms}
	fi := &SymbolFieldIndexer{Extractor: NewRegexExtractor()}

	field := &document.Field{
		Name:     "code",
		Type:     document.FieldTypeSymbol,
		Value:    "",
		Store:    true,
		Index:    true,
		Language: "go",
	}

	entry, err := fi.IndexField(helpers, "doc1", field)
	if err != nil {
		t.Fatal(err)
	}
	if entry != nil {
		t.Error("expected nil entry for empty source")
	}
}

func TestSymbolFieldIndexer_NoSymbolsFound(t *testing.T) {
	ms := newMemStore()
	helpers := &testHelpers{store: ms}
	fi := &SymbolFieldIndexer{Extractor: NewRegexExtractor()}

	field := &document.Field{
		Name:     "code",
		Type:     document.FieldTypeSymbol,
		Value:    "// just a comment\n// nothing to see here\n",
		Store:    true,
		Index:    true,
		Language: "go",
	}

	entry, err := fi.IndexField(helpers, "doc1", field)
	if err != nil {
		t.Fatal(err)
	}
	if entry != nil {
		t.Error("expected nil entry when no symbols found")
	}
}

func TestSymbolFieldIndexer_StoreDisabled(t *testing.T) {
	ms := newMemStore()
	helpers := &testHelpers{store: ms}
	fi := &SymbolFieldIndexer{Extractor: NewRegexExtractor()}

	field := &document.Field{
		Name:     "code",
		Type:     document.FieldTypeSymbol,
		Value:    "func Foo() {}",
		Store:    false, // explicitly disabled
		Index:    true,
		Language: "go",
	}

	entry, err := fi.IndexField(helpers, "doc1", field)
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	// Postings should exist.
	_, err = ms.Get(context.Background(), "t/code.sym/foo/doc1")
	if err != nil {
		t.Error("expected posting for foo symbol")
	}

	// Stored symbols should NOT exist.
	_, err = ms.Get(context.Background(), "sym/code/doc1")
	if err == nil {
		t.Error("expected no stored symbols when Store=false")
	}

	// _stored should not be in terms.
	for _, term := range entry.Terms {
		if term == "_stored" {
			t.Error("expected no _stored marker when Store=false")
		}
	}
}

func TestSymbolFieldIndexer_ScopeIndexed(t *testing.T) {
	ms := newMemStore()
	helpers := &testHelpers{store: ms}
	fi := &SymbolFieldIndexer{Extractor: NewRegexExtractor()}

	field := &document.Field{
		Name:     "code",
		Type:     document.FieldTypeSymbol,
		Value:    "func (s *Server) Handle() {}",
		Store:    true,
		Index:    true,
		Language: "go",
	}

	entry, err := fi.IndexField(helpers, "doc1", field)
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	// Verify scope posting.
	_, err = ms.Get(context.Background(), "t/code.scope/server/doc1")
	if err != nil {
		t.Error("expected scope posting for 'server'")
	}

	hasScopeTerm := false
	for _, term := range entry.Terms {
		if term == "scope:server" {
			hasScopeTerm = true
		}
	}
	if !hasScopeTerm {
		t.Error("expected 'scope:server' in rev index terms")
	}
}

func TestGetSymbols(t *testing.T) {
	ms := newMemStore()
	helpers := &testHelpers{store: ms}
	fi := &SymbolFieldIndexer{Extractor: NewRegexExtractor()}

	field := &document.Field{
		Name:     "src",
		Type:     document.FieldTypeSymbol,
		Value:    "func Greet() {}\ntype Person struct{}",
		Store:    true,
		Index:    true,
		Language: "go",
	}

	_, err := fi.IndexField(helpers, "doc1", field)
	if err != nil {
		t.Fatal(err)
	}

	symbols, err := GetSymbols(helpers, "src", "doc1")
	if err != nil {
		t.Fatal(err)
	}
	if len(symbols) == 0 {
		t.Fatal("expected symbols from GetSymbols")
	}

	found := make(map[string]bool)
	for _, sym := range symbols {
		found[sym.Name] = true
	}
	if !found["Greet"] {
		t.Error("expected Greet in stored symbols")
	}
	if !found["Person"] {
		t.Error("expected Person in stored symbols")
	}
}

func TestGetSymbols_NotFound(t *testing.T) {
	ms := newMemStore()
	helpers := &testHelpers{store: ms}

	_, err := GetSymbols(helpers, "code", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent document")
	}
}

func TestSymbolFieldIndexer_Type(t *testing.T) {
	fi := &SymbolFieldIndexer{Extractor: NewRegexExtractor()}
	if fi.Type() != "symbol" {
		t.Errorf("expected type 'symbol', got %q", fi.Type())
	}
	// Verify it satisfies the interface.
	var _ index.FieldIndexer = fi
}
