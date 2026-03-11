<p align="center">
  <img src="https://faas-lon1-917a94a7.doserverless.co/api/v1/web/fn-38e8948f-7a79-4b9d-af0e-f6399470d7be/generate/svg?type=root" alt="search.md" />
</p>

# search.md

Full-text search for Go. One dependency. Zero config.

Index documents, search with BM25 scoring, vector search, facets, highlighting — all backed by any [store.md](https://github.com/readmedotmd/store.md) key-value store.

## Get Started

```bash
go get github.com/readmedotmd/search.md
```

```go
store, _ := storemd.New("my.db")
idx, _ := searchmd.New(store, nil)

idx.Index("doc1", map[string]interface{}{
    "title":   "Hello World",
    "content": "This is a test document about search engines",
})
idx.Index("doc2", map[string]interface{}{
    "title":   "Go Programming",
    "content": "Go makes it easy to build reliable software",
})

results, _ := idx.Search(ctx, query.NewMatchQuery("search").SetField("content"))
for _, hit := range results.Hits {
    fmt.Printf("%s (score: %.4f)\n", hit.ID, hit.Score)
}
```

That's it. No schema, no server, no configuration.

## Features

**Search** — BM25 text search, KNN vector search, code-aware search with camelCase/snake_case splitting

**14 query types** — term, match, phrase, prefix, fuzzy, wildcard, regexp, boolean, conjunction, disjunction, numeric range, date range, KNN, match-all

**Rich results** — highlighting, faceted aggregation (term, numeric range, date range), pagination

**Flexible mappings** — text, keyword, numeric, boolean, datetime, vector, code, symbol fields. Static or dynamic. Multi-type indices.

**Code symbol search** — extract and query functions, classes, types across 10+ languages via regex, tree-sitter, or ctags

**Pluggable** — swap scoring (BM25/TF-IDF/custom), highlighting (HTML/ANSI/custom), faceting, field indexers, and query types

**Efficient** — bounded heaps for top-N, reverse index for O(1) deletion, streaming vector iteration, single-pass faceting

## Quick Examples

```go
// Fuzzy search
idx.Search(ctx, query.NewFuzzyQuery("helo").SetField("title").SetFuzziness(1))

// Boolean query
bq := query.NewBooleanQuery()
bq.AddMust(query.NewMatchQuery("search").SetField("content"))
bq.AddMustNot(query.NewTermQuery("slow").SetField("content"))
idx.Search(ctx, bq)

// Vector search
idx.Search(ctx, query.NewKNNQuery(embedding, 10).SetField("vector"))

// Search with highlighting and facets
req := &search.SearchRequest{
    Query:     query.NewMatchQuery("search").SetField("content"),
    Size:      20,
    Highlight: &search.HighlightRequest{},
    Facets:    map[string]*search.FacetRequest{
        "authors": {Field: "author", Size: 10},
    },
}
results, _ := idx.SearchWithRequest(ctx, req)
```

## Docs

| | |
|---|---|
| **[Queries](./docs/queries.md)** | All 14 query types with examples |
| **[Search Options](./docs/search-options.md)** | Pagination, highlighting, facets, context/cancellation |
| **[Mappings](./docs/mappings.md)** | Field types, static vs dynamic, type mappings |
| **[Plugins](./docs/plugins.md)** | Custom scorers, highlighters, field indexers |
| **[Code Search](./docs/code-search.md)** | Symbol extraction via regex, tree-sitter, ctags |
| **[Architecture](./docs/architecture.md)** | Storage layout, memory efficiency, scoring |

## License

MIT — see [LICENSE](./LICENSE).
