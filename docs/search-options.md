# Search Options

## Basic Search

```go
results, err := idx.Search(ctx, query.NewMatchQuery("hello").SetField("title"))
```

## Search Requests

For full control, use `SearchWithRequest`:

```go
req := &search.SearchRequest{
    Query:  query.NewMatchQuery("search").SetField("content"),
    Size:   20,          // results per page (max 10,000)
    From:   0,           // offset for pagination (max 100,000)
    Fields: []string{"title", "author"}, // fields to return
}

results, err := idx.SearchWithRequest(ctx, req)
```

Convenience constructors:

```go
req := search.NewSearchRequest(myQuery)                // defaults: size=10, from=0
req := search.NewSearchRequestOptions(myQuery, 20, 0)  // size=20, from=0
```

## Context & Cancellation

All search methods accept a `context.Context` for timeout and cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

results, err := idx.Search(ctx, myQuery)
```

## Highlighting

Add highlighted fragments to results:

```go
req := &search.SearchRequest{
    Query: myQuery,
    Highlight: &search.HighlightRequest{
        Style:  "html",                  // optional style hint
        Fields: []string{"content"},     // omit for all fields
    },
}

results, _ := idx.SearchWithRequest(ctx, req)
for _, hit := range results.Hits {
    for field, frags := range hit.Fragments {
        fmt.Printf("  %s: %v\n", field, frags)
    }
}
```

## Faceted Search

Aggregate results into buckets:

```go
req := &search.SearchRequest{
    Query: myQuery,
    Facets: map[string]*search.FacetRequest{
        // Term facets
        "authors": {Field: "author", Size: 10},

        // Numeric range facets
        "price_ranges": {
            Field: "price",
            Size:  3,
            NumericRanges: []search.NumericRange{
                {Name: "cheap", Min: floatPtr(0), Max: floatPtr(10)},
                {Name: "mid",   Min: floatPtr(10), Max: floatPtr(50)},
                {Name: "expensive", Min: floatPtr(50), Max: floatPtr(1000)},
            },
        },

        // Date range facets
        "date_ranges": {
            Field: "created",
            Size:  2,
            DateTimeRanges: []search.DateTimeRange{
                {Name: "this_year", Start: timePtr(2025, 1, 1), End: timePtr(2026, 1, 1)},
                {Name: "last_year", Start: timePtr(2024, 1, 1), End: timePtr(2025, 1, 1)},
            },
        },
    },
}

results, _ := idx.SearchWithRequest(ctx, req)
for name, facet := range results.Facets {
    fmt.Printf("Facet %s (total=%d):\n", name, facet.Total)
    for _, tf := range facet.Terms {
        fmt.Printf("  %s: %d\n", tf.Term, tf.Count)
    }
}
```

## Working with Results

```go
results, err := idx.Search(ctx, myQuery)

fmt.Println("Total:", results.Total)
fmt.Println("MaxScore:", results.MaxScore)
fmt.Println("Took:", results.Took)

for _, hit := range results.Hits {
    fmt.Printf("  %s score=%.4f fields=%v\n", hit.ID, hit.Score, hit.Fields)
}
```

## Managing Documents

```go
// Retrieve a stored document by ID
fields, err := idx.Document("doc1")

// Delete a document
err := idx.Delete("doc1")

// Get total document count
count, err := idx.DocCount()
```

### Document ID Rules

Document IDs are validated on index, delete, and retrieve operations:

- IDs **cannot be empty**
- IDs **cannot contain `/`** (used as a key separator internally)

Invalid IDs return an error immediately.

## Safety Limits

search.md enforces several limits to prevent excessive memory allocation:

| Limit | Value | Description |
|-------|-------|-------------|
| `MaxSearchSize` | 10,000 | Maximum results per page |
| `MaxSearchFrom` | 100,000 | Maximum pagination offset |
| `MaxFacetDocIDs` | 100,000 | Maximum doc IDs collected for facet computation |

Values exceeding these limits are silently clamped.

## Concurrency

All `SearchIndex` methods are safe for concurrent use. Reads (`Search`, `Document`, `DocCount`) use a shared read lock; writes (`Index`, `Delete`, `Batch.Execute`) use an exclusive write lock. Batch operations hold the write lock for the entire batch to ensure atomicity.
