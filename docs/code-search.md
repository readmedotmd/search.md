# Code Search

The `plugin/codesearch` package provides structured code symbol extraction and indexing. It goes beyond the built-in `CodeFieldMapping` (which does token-level text search) by extracting **named symbols** — functions, classes, types, methods, constants, etc. — and indexing them as queryable sub-fields.

## Extraction Strategies

Three strategies are available, all implementing the `SymbolExtractor` interface:

| Extractor | How it works | Dependencies | Best for |
|-----------|-------------|--------------|----------|
| `RegexExtractor` | Line-oriented regex patterns | None | Quick setup, broad language coverage |
| `TreeSitterExtractor` | AST walking via tree-sitter | Bring your own parser | Precise, scope-aware extraction |
| `TagExtractor` | ctags-compatible extraction | None | Interop with editors/existing ctags files |

## Quick Start

```go
import (
    "github.com/readmedotmd/search.md/mapping"
    "github.com/readmedotmd/search.md/plugin/codesearch"
)

// 1. Configure a symbol field in your mapping.
im := searchmd.NewIndexMapping()
dm := searchmd.NewDocumentStaticMapping()
dm.AddFieldMapping("source", mapping.NewSymbolFieldMapping("go"))
dm.AddFieldMapping("path", searchmd.NewKeywordFieldMapping())
im.DefaultMapping = dm

// 2. Create the index with a symbol extractor plugin.
idx, _ := searchmd.New(store, im, codesearch.WithRegexSymbolIndexer())

// 3. Index source code.
idx.Index("main.go", map[string]interface{}{
    "path": "cmd/server/main.go",
    "source": `package main

func StartServer(addr string) error {
    return http.ListenAndServe(addr, nil)
}

type Config struct {
    Addr    string
    Debug   bool
}
`,
})

// 4. Query by symbol name or kind.
results, _ := idx.Search(ctx, query.NewTermQuery("startserver").SetField("source.sym"))
results, _ = idx.Search(ctx, query.NewTermQuery("struct").SetField("source.kind"))
```

## Sub-fields

When a symbol field named `source` is indexed, the `SymbolFieldIndexer` creates three searchable sub-fields:

| Sub-field | Contains | Example query |
|-----------|----------|---------------|
| `source.sym` | Lowercased symbol names | `query.NewTermQuery("getuser").SetField("source.sym")` |
| `source.kind` | Symbol kinds | `query.NewTermQuery("function").SetField("source.kind")` |
| `source.scope` | Parent scope names | `query.NewTermQuery("server").SetField("source.scope")` |

Symbol kind values: `function`, `method`, `class`, `struct`, `interface`, `type`, `variable`, `constant`, `field`, `import`, `package`, `module`, `property`, `enum`, `enum_value`, `trait`.

## RegexExtractor

Zero-dependency symbol extraction using language-specific regex patterns. Supports 10 languages out of the box:

Go, Python, JavaScript, TypeScript, Java, Rust, C, C++, Ruby, PHP

```go
// Use the convenience option (most common):
idx, _ := searchmd.New(store, im, codesearch.WithRegexSymbolIndexer())

// Or configure manually:
extractor := codesearch.NewRegexExtractor()
idx, _ := searchmd.New(store, im, codesearch.WithSymbolIndexer(extractor))
```

**Language aliases** are resolved automatically: `golang` -> `go`, `py`/`python3` -> `python`, `js`/`jsx` -> `javascript`, `ts`/`tsx` -> `typescript`, `rs` -> `rust`, `rb` -> `ruby`, `cpp`/`cc`/`cxx` -> `c++`.

**Custom language patterns** can be added at runtime:

```go
extractor := codesearch.NewRegexExtractor()
err := extractor.AddLanguage("haskell", []codesearch.PatternDef{
    {Pattern: `^(\w+)\s+::`, Kind: codesearch.SymbolFunction, NameGroup: 1, ScopeGroup: -1},
    {Pattern: `^data\s+(\w+)`, Kind: codesearch.SymbolType,     NameGroup: 1, ScopeGroup: -1},
    {Pattern: `^class\s+(\w+)`, Kind: codesearch.SymbolClass,    NameGroup: 1, ScopeGroup: -1},
})
if err != nil {
    log.Fatal(err)
}
```

## TreeSitterExtractor

AST-based extraction for maximum precision. You bring your own tree-sitter Go binding (no CGo dependency in search.md itself).

```go
import (
    sitter "github.com/smacker/go-tree-sitter"
    "github.com/smacker/go-tree-sitter/golang"
    "github.com/readmedotmd/search.md/plugin/codesearch"
)

// 1. Implement the ASTParser interface.
type goParser struct{ parser *sitter.Parser }

func (p *goParser) Parse(source []byte) (*codesearch.ASTNode, error) {
    p.parser.SetLanguage(golang.GetLanguage())
    tree, err := p.parser.ParseCtx(context.Background(), nil, source)
    if err != nil {
        return nil, err
    }
    return convertNode(tree.RootNode(), source), nil
}

func convertNode(n *sitter.Node, source []byte) *codesearch.ASTNode {
    node := &codesearch.ASTNode{
        Type:      n.Type(),
        Content:   n.Content(source),
        StartByte: n.StartByte(),
        EndByte:   n.EndByte(),
        StartRow:  n.StartPoint().Row,
        StartCol:  n.StartPoint().Column,
    }
    for i := 0; i < int(n.ChildCount()); i++ {
        child := n.Child(i)
        childNode := convertNode(child, source)
        childNode.FieldName = n.FieldNameForChild(i)
        node.Children = append(node.Children, childNode)
    }
    return node
}

// 2. Register the parser with the extractor.
ext := codesearch.NewTreeSitterExtractor()
ext.AddLanguage("go", &goParser{parser: sitter.NewParser()}, nil)

// 3. Use it.
idx, _ := searchmd.New(store, im, codesearch.WithTreeSitterIndexer(ext))
```

Built-in extraction rules for Go, Python, JavaScript, TypeScript, Java, Rust, C, C++, and Ruby. Custom rules via the third argument to `AddLanguage`:

```go
ext.AddLanguage("swift", swiftParser, []codesearch.ExtractionRule{
    {NodeType: "function_declaration", Kind: codesearch.SymbolFunction, NameField: "name"},
    {NodeType: "class_declaration",    Kind: codesearch.SymbolClass,    NameField: "name"},
    {NodeType: "protocol_declaration", Kind: codesearch.SymbolInterface, NameField: "name",
        ScopeParentTypes: []string{"class_declaration"}, ScopeNameField: "name"},
})
```

## TagExtractor

ctags-compatible extraction for interoperability with editors and IDEs.

```go
idx, _ := searchmd.New(store, im, codesearch.WithTagIndexer())

// Extract tags in ctags format for external use:
ext := codesearch.NewTagExtractor()
tags, _ := ext.ExtractTags(sourceCode, "go", "main.go")
output := codesearch.FormatTags(tags)

// Import an existing ctags file:
ctagsContent, _ := os.ReadFile("tags")
symbols, _ := codesearch.ParseTags(string(ctagsContent))
```

## CompositeExtractor

Combine multiple extractors for best coverage. Results are deduplicated by name+kind:

```go
regex := codesearch.NewRegexExtractor()

ts := codesearch.NewTreeSitterExtractor()
ts.AddLanguage("go", myGoParser, nil)

composite := codesearch.NewCompositeExtractor(regex, ts)
idx, _ := searchmd.New(store, im, codesearch.WithSymbolIndexer(composite))
```

## Retrieving Stored Symbols

When a symbol field has `Store: true` (the default), the full extracted symbol data is persisted:

```go
symbols, err := codesearch.GetSymbols(indexHelpers, "source", "main.go")
for _, sym := range symbols {
    fmt.Printf("%s %s (line %d)\n", sym.Kind, sym.Name, sym.Line)
}
// Output:
//   package main (line 1)
//   function StartServer (line 3)
//   struct Config (line 7)
```
