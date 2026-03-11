# Query Types

search.md ships with 14 query types covering text search, structured data, vector search, and compound logic.

All query constructors are in the `github.com/readmedotmd/search.md/search/query` package.

## Text Queries

### Term Query

Exact term match against the inverted index. The term is **not** analyzed — use this when you know the exact indexed token.

```go
query.NewTermQuery("hello").SetField("title")
```

### Match Query

Analyzed text search. The input is tokenized and matched against indexed terms. Default operator is OR.

```go
query.NewMatchQuery("quick brown fox").SetField("content")

// Require all terms
query.NewMatchQuery("quick brown").SetField("content").SetOperator("and")
```

### Match Phrase Query

Matches an exact phrase in order, using term vector positions.

```go
query.NewMatchPhraseQuery("quick brown fox").SetField("content")
```

### Prefix Query

Matches all terms starting with the given prefix. Expansion is capped at 1,000 matching terms.

```go
query.NewPrefixQuery("hel").SetField("title")
```

### Fuzzy Query

Edit distance matching using Levenshtein distance. Fuzziness is capped at 2. Expansion is capped at 1,000 matching terms.

```go
query.NewFuzzyQuery("helo").SetField("title").SetFuzziness(1)
```

### Wildcard Query

Pattern matching with `*` (any characters) and `?` (single character).

```go
query.NewWildcardQuery("he*o").SetField("title")
```

### Regexp Query

Full regular expression matching against indexed terms. Expansion is capped at 1,000 matching terms.

```go
query.NewRegexpQuery("^hel+o$").SetField("title")
```

## Structured Queries

### Numeric Range Query

Matches documents where a numeric field falls within a range.

```go
min, max := 10.0, 100.0
query.NewNumericRangeQuery(&min, &max).SetField("price")

// Open-ended ranges
query.NewNumericRangeQuery(&min, nil).SetField("price") // >= 10
query.NewNumericRangeQuery(nil, &max).SetField("price") // <= 100
```

### Date Range Query

Matches documents where a datetime field falls within a range.

```go
start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
end := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
query.NewDateRangeQuery(&start, &end).SetField("created")
```

## Compound Queries

### Boolean Query

Combines must, should, and must_not clauses.

```go
bq := query.NewBooleanQuery()
bq.AddMust(query.NewMatchQuery("search").SetField("content"))
bq.AddShould(query.NewTermQuery("fast").SetField("content"))
bq.AddMustNot(query.NewTermQuery("slow").SetField("content"))
```

### Conjunction Query (AND)

All sub-queries must match.

```go
query.NewConjunctionQuery(q1, q2, q3)
```

### Disjunction Query (OR)

At least one sub-query must match (configurable minimum).

```go
query.NewDisjunctionQuery(q1, q2).SetMin(2) // at least 2 must match
```

## Vector Search

### KNN Query

k-nearest-neighbor vector search using cosine similarity.

```go
query.NewKNNQuery([]float32{0.1, 0.2, ...}, 10).SetField("embedding")
```

## Utility Queries

### Match All / Match None

```go
query.NewMatchAllQuery()
query.NewMatchNoneQuery()
```

## Query Boost

All queries support a boost factor to influence scoring:

```go
query.NewMatchQuery("important").SetField("title").SetBoost(2.0)
```
