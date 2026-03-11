// Package codesearch provides plugins for extracting and indexing code symbols.
//
// It includes three extraction strategies:
//   - RegexExtractor: lightweight regex-based symbol extraction (zero dependencies)
//   - TreeSitterExtractor: AST-based extraction via tree-sitter (bring your own parser)
//   - TagExtractor: ctags-compatible tag extraction and parsing
//
// All extractors implement the SymbolExtractor interface and can be used with
// the SymbolFieldIndexer to create searchable sub-fields for symbol names,
// kinds, and scopes.
//
// Usage:
//
//	extractor := codesearch.NewRegexExtractor()
//	idx, _ := searchmd.New(store, mapping,
//	    codesearch.WithSymbolIndexer(extractor),
//	)
package codesearch

// SymbolKind represents the type of a code symbol.
type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolClass     SymbolKind = "class"
	SymbolStruct    SymbolKind = "struct"
	SymbolInterface SymbolKind = "interface"
	SymbolType      SymbolKind = "type"
	SymbolVariable  SymbolKind = "variable"
	SymbolConstant  SymbolKind = "constant"
	SymbolField     SymbolKind = "field"
	SymbolImport    SymbolKind = "import"
	SymbolPackage   SymbolKind = "package"
	SymbolModule    SymbolKind = "module"
	SymbolProperty  SymbolKind = "property"
	SymbolEnum      SymbolKind = "enum"
	SymbolEnumValue SymbolKind = "enum_value"
	SymbolTrait     SymbolKind = "trait"
)

// Symbol represents an extracted code symbol.
type Symbol struct {
	Name      string     `json:"name"`
	Kind      SymbolKind `json:"kind"`
	Language  string     `json:"language,omitempty"`
	Scope     string     `json:"scope,omitempty"`
	Signature string     `json:"signature,omitempty"`
	Line      int        `json:"line,omitempty"`
	StartByte int        `json:"start_byte,omitempty"`
	EndByte   int        `json:"end_byte,omitempty"`
}

// SymbolExtractor extracts code symbols from source code.
// Implementations include RegexExtractor, TreeSitterExtractor, and TagExtractor.
type SymbolExtractor interface {
	// Extract parses source code and returns extracted symbols.
	// Language is a hint (e.g., "go", "python", "javascript"); implementations
	// may ignore it or use it to select language-specific patterns.
	Extract(source []byte, language string) ([]Symbol, error)

	// SupportedLanguages returns the languages this extractor supports.
	SupportedLanguages() []string
}
