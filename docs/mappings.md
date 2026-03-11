# Field Mappings

Mappings control how document fields are indexed and stored.

## Field Types

| Mapping | Description |
|---------|-------------|
| `NewTextFieldMapping()` | Analyzed text with BM25 scoring |
| `NewKeywordFieldMapping()` | Exact-match, not analyzed |
| `NewNumericFieldMapping()` | Numeric values for range queries |
| `NewBooleanFieldMapping()` | Boolean values |
| `NewDateTimeFieldMapping()` | Date/time values for range queries |
| `NewVectorFieldMapping(dims)` | Float vectors for KNN search |
| `NewCodeFieldMapping()` | Source code with camelCase/snake_case splitting |
| `NewSymbolFieldMapping(lang)` | Code symbol extraction (functions, classes, types) |

## Custom Mapping

```go
m := searchmd.NewIndexMapping()
dm := searchmd.NewDocumentMapping()
dm.AddFieldMapping("title", searchmd.NewTextFieldMapping())
dm.AddFieldMapping("price", searchmd.NewNumericFieldMapping())
dm.AddFieldMapping("embedding", searchmd.NewVectorFieldMapping(128))
m.DefaultMapping = dm

idx, err := searchmd.New(store, m)
```

## Static vs Dynamic Mappings

```go
// Dynamic (default) - all fields are auto-indexed
dm := searchmd.NewDocumentMapping()

// Static - only explicitly mapped fields are indexed
dm := searchmd.NewDocumentStaticMapping()
dm.AddFieldMapping("title", searchmd.NewTextFieldMapping())
// "content" would NOT be indexed
```

## Type Mappings

Route documents to different mappings based on a type field:

```go
m := searchmd.NewIndexMapping()
m.TypeField = "type"

articleMapping := searchmd.NewDocumentStaticMapping()
articleMapping.AddFieldMapping("headline", searchmd.NewTextFieldMapping())
m.AddDocumentMapping("article", articleMapping)

productMapping := searchmd.NewDocumentStaticMapping()
productMapping.AddFieldMapping("name", searchmd.NewTextFieldMapping())
productMapping.AddFieldMapping("price", searchmd.NewNumericFieldMapping())
m.AddDocumentMapping("product", productMapping)
```
