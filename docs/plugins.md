# Plugin Architecture

search.md is fully modular. Scoring, highlighting, faceting, field indexing, and query types can all be replaced or extended via plugins.

## Configuring Plugins

Pass `plugin.Option` values to `searchmd.New()`:

```go
import "github.com/readmedotmd/search.md/plugin"

idx, _ := searchmd.New(store, nil,
    plugin.WithScorer(&plugin.TFIDFScorerFactory{}),           // use TF-IDF instead of BM25
    plugin.WithHighlighter(plugin.DefaultANSIHighlighter()),   // ANSI terminal highlighting
    plugin.WithFieldIndexer(myGeoIndexer),                     // add a custom field type
)
```

Or register at runtime:

```go
idx.Plugins().SetScorerFactory(plugin.DefaultBM25())
idx.Plugins().RegisterQueryPlugin(myQueryPlugin)
idx.RegisterFieldIndexer(myFieldIndexer)
```

## Built-in Plugins

| Component | Default | Alternatives |
|-----------|---------|-------------|
| Scorer | BM25 (`K1=1.2, B=0.75`) | TF-IDF, or implement `plugin.ScorerFactory` |
| Highlighter | HTML (`<mark>`) | ANSI terminal, or implement `plugin.HighlighterFactory` |
| Facet Builder | Default | Implement `plugin.FacetBuilderFactory` |
| Field Indexers | text, keyword, numeric, boolean, datetime, vector, code, symbol | Implement `index.FieldIndexer` |

## Custom Scorer

```go
type MyScorerFactory struct{}

func (f *MyScorerFactory) Name() string { return "custom" }
func (f *MyScorerFactory) NewScorer(docCount, docFreq uint64, avgFieldLen float64) plugin.Scorer {
    return &myScorer{docCount: docCount, docFreq: docFreq, avgFL: avgFieldLen}
}

type myScorer struct { docCount, docFreq uint64; avgFL float64 }

func (s *myScorer) Score(termFreq, fieldLength int, boost float64) float64 {
    // your scoring logic
}
```

## Custom Field Indexer

Add support for new field types (e.g., geo, graph) without modifying core code:

```go
type GeoFieldIndexer struct{}

func (g *GeoFieldIndexer) Type() string { return "geo" }

func (g *GeoFieldIndexer) IndexField(helpers index.IndexHelpers, docID string, field *document.Field) (*index.RevIdxEntry, error) {
    // index the field data using helpers.Store()
    return &index.RevIdxEntry{Field: field.Name, Type: "geo"}, nil
}

func (g *GeoFieldIndexer) DeleteField(helpers index.IndexHelpers, docID string, entry index.RevIdxEntry) error {
    // clean up stored data
    return nil
}

// Register via option or at runtime
idx, _ := searchmd.New(store, nil, plugin.WithFieldIndexer(&GeoFieldIndexer{}))
```

## Plugin Interfaces

| Interface | Package | Purpose |
|-----------|---------|---------|
| `ScorerFactory` / `Scorer` | `plugin` | Document relevance scoring |
| `HighlighterFactory` / `Highlighter` | `plugin` | Search result highlighting |
| `FacetBuilderFactory` / `FacetBuilder` | `plugin` | Faceted search aggregation |
| `QueryPlugin` | `plugin` | Custom query types |
| `FieldPlugin` | `plugin` | Custom field type metadata |
| `FieldIndexer` | `index` | Field indexing and deletion logic |
| `IndexReader` | `plugin` | Read-only index access for searchers |
