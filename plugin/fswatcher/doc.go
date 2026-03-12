// Package fswatcher watches a directory and automatically indexes its files
// into a search.md [searchmd.SearchIndex].
//
// It supports three indexing modes that can be enabled independently or combined:
//
//   - FTS (full-text search): indexes file content as analyzed text for BM25
//     ranking via the "content" field.
//   - Code: indexes file content with code-aware tokenization (camelCase and
//     snake_case splitting) via the "code" field, and extracts symbols
//     (functions, classes, types) into searchable "source.sym", "source.kind",
//     and "source.scope" sub-fields.
//   - Vector: indexes file content as vector embeddings for semantic KNN search
//     via the "embedding" field, using a caller-supplied [EmbedFunc].
//
// # Document schema
//
// Every indexed file produces a document with these metadata fields:
//
//   - path          (keyword)  — relative path from the watched root (e.g., "src/main.go")
//   - filename      (keyword)  — base file name for exact matching (e.g., "main.go")
//   - filename_text (text)     — base file name analyzed for partial/fuzzy search
//   - ext           (keyword)  — file extension without dot (e.g., "go")
//   - size          (numeric)  — file size in bytes
//   - modified      (datetime) — last modification time in RFC 3339
//
// Additional fields are added based on the enabled modes (see above).
//
// Document IDs are derived from relative paths with "/" replaced by ":" since
// document IDs cannot contain slashes. Use [RelPathFromDocID] to convert back.
//
// # Setup
//
// Use [IndexMapping] to generate a [mapping.IndexMapping] that matches the
// enabled modes, then pass the same options to [New]:
//
//	opts := []fswatcher.Option{
//	    fswatcher.WithFTS(),
//	    fswatcher.WithCode("go"),
//	}
//
//	m := fswatcher.IndexMapping(opts...)
//	idx, _ := searchmd.New(store, m, codesearch.WithRegexSymbolIndexer())
//	w, _ := fswatcher.New(idx, "/path/to/project", opts...)
//
// # Watching
//
// Call [Watcher.Start] to perform an initial full scan and begin polling for
// changes on a configurable interval (default 10s). The watcher detects new
// files, modified files (via mtime + size), and deleted files. Binary files
// and hidden directories (names starting with ".") are skipped automatically.
//
//	err := w.Start(ctx)
//	defer w.Stop()
//
// For one-shot indexing without continuous watching, call [Watcher.Scan]
// directly.
//
// # Filtering
//
// Use [WithFilter] to control which files are indexed:
//
//	fswatcher.WithFilter(func(rel string) bool {
//	    return strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, ".py")
//	})
//
// # Language detection
//
// When code mode is enabled with an empty language string, the language is
// auto-detected from file extensions. Supported extensions: .go, .py, .js,
// .jsx, .mjs, .ts, .tsx, .java, .rs, .c, .h, .cpp, .cc, .cxx, .hpp, .hxx,
// .rb, .php.
package fswatcher
