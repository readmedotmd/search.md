# Architecture

## Storage

search.md stores all index data in a key-value store via the [store.md](https://github.com/readmedotmd/store.md) interface. Key prefixes organize data:

| Prefix | Contents |
|--------|----------|
| `d/` | Stored document JSON |
| `t/` | Term postings (inverted index) |
| `f/` | Field lengths (token counts) |
| `n/` | Document frequencies per term |
| `v/` | Vector data |
| `tv/` | Term vector positions (for phrase queries) |
| `num/` | Numeric field values |
| `dt/` | DateTime field values |
| `bool/` | Boolean field values |
| `ri/` | Reverse index for targeted deletion |
| `sym/` | Extracted symbol data (codesearch plugin) |
| `m/` | Metadata (doc count, field length sums, index version) |

## Memory Efficiency

search.md is designed to work with large datasets without loading everything into memory:

- **Bounded min-heap for top-N**: Search results use a min-heap of size `from + size` instead of collecting all matches. Only the top-scoring documents are kept in memory.

- **Reverse index for O(1) deletion**: Each document stores a reverse index entry (`ri/{docID}`) listing all its fields and indexed terms. Deletion constructs exact key paths instead of scanning the entire index.

- **Streaming vector iteration**: KNN search uses `ForEachVector` to stream vectors in batches of 256 from the store, maintaining a bounded heap of size K. Only K vectors are held in memory at any time.

- **Paginated MatchAll**: The match-all searcher loads document IDs in pages of 256 via cursor-based pagination, rather than loading all doc IDs at once.

- **Single-pass faceting**: Facet computation collects document IDs during the same search pass that scores results, avoiding a second query execution.

- **Conjunction intersection**: The conjunction (AND) searcher materializes the first sub-query as a candidate set, then intersects with subsequent queries. Only candidate-matching scores are tracked.

- **Facet memory cap**: Facet computation collects at most 100,000 document IDs (`MaxFacetDocIDs`). Facets computed from the subset still provide representative distribution information without unbounded memory growth.

## Index Versioning

The index stores a format version at key `m/index_version` (currently `"1"`). This is written on first initialization and can be read via `Index.IndexVersion()`. Future versions will use this to handle migrations or reject incompatible data.

## Scoring

Text search defaults to **Okapi BM25** scoring (k1=1.2, b=0.75), replaceable via `plugin.WithScorer()`. Scores are influenced by:

- Term frequency in the document
- Inverse document frequency (rarer terms score higher)
- Field length normalization (shorter fields with the term score higher)
- Boost factors on queries

Vector search uses **cosine similarity**, normalized to [0, 1].
