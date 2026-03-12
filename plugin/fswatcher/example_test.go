package fswatcher_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	searchmd "github.com/readmedotmd/search.md"
	"github.com/readmedotmd/search.md/plugin/codesearch"
	"github.com/readmedotmd/search.md/plugin/fswatcher"
	"github.com/readmedotmd/search.md/search/query"
)

func Example() {
	// Create a temporary directory with some files.
	dir, _ := os.MkdirTemp("", "fswatcher-example")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("Hello, World! Welcome to search.md."), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("These are some important notes about the project."), 0o644)

	// Set up the index with FTS mode.
	store := newMemStore()
	opts := []fswatcher.Option{fswatcher.WithFTS()}
	m := fswatcher.IndexMapping(opts...)
	idx, _ := searchmd.New(store, m)

	// Create the watcher and do a one-shot scan.
	w, _ := fswatcher.New(idx, dir, opts...)
	_ = w.Scan(context.Background())

	// Search for documents.
	results, _ := idx.Search(context.Background(), query.NewMatchQuery("hello").SetField("content"))

	fmt.Printf("Found %d result(s)\n", results.Total)
	if results.Total > 0 {
		fmt.Printf("Top hit: %s\n", results.Hits[0].ID)
	}

	// Output:
	// Found 1 result(s)
	// Top hit: hello.txt
}

func Example_codeSearch() {
	// Create a temporary directory with a Go source file.
	dir, _ := os.MkdirTemp("", "fswatcher-code-example")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "server.go"), []byte(`package main

type HTTPServer struct {
	Port int
}

func NewHTTPServer(port int) *HTTPServer {
	return &HTTPServer{Port: port}
}

func (s *HTTPServer) Start() error {
	return nil
}
`), 0o644)

	// Set up the index with code mode.
	store := newMemStore()
	opts := []fswatcher.Option{fswatcher.WithCode("go")}
	m := fswatcher.IndexMapping(opts...)
	idx, _ := searchmd.New(store, m, codesearch.WithRegexSymbolIndexer())

	// Index the directory.
	w, _ := fswatcher.New(idx, dir, opts...)
	_ = w.Scan(context.Background())

	// Search for struct symbols.
	results, _ := idx.Search(context.Background(), query.NewTermQuery("struct").SetField("source.kind"))
	fmt.Printf("Structs found: %d\n", results.Total)

	// Search for a specific symbol name.
	results, _ = idx.Search(context.Background(), query.NewTermQuery("newhttpserver").SetField("source.sym"))
	fmt.Printf("NewHTTPServer found: %v\n", results.Total > 0)

	// Output:
	// Structs found: 1
	// NewHTTPServer found: true
}

func Example_continuousWatching() {
	dir, _ := os.MkdirTemp("", "fswatcher-watch-example")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("Initial content."), 0o644)

	store := newMemStore()
	opts := []fswatcher.Option{
		fswatcher.WithFTS(),
		fswatcher.WithPollInterval(50 * time.Millisecond),
	}
	m := fswatcher.IndexMapping(opts...)
	idx, _ := searchmd.New(store, m)

	w, _ := fswatcher.New(idx, dir, opts...)
	ctx := context.Background()
	_ = w.Start(ctx)

	count, _ := idx.DocCount()
	fmt.Printf("After start: %d doc(s)\n", count)

	// Add a file while watching.
	os.WriteFile(filepath.Join(dir, "added.txt"), []byte("Added later."), 0o644)
	time.Sleep(200 * time.Millisecond)

	count, _ = idx.DocCount()
	fmt.Printf("After adding file: %d doc(s)\n", count)

	w.Stop()

	// Output:
	// After start: 1 doc(s)
	// After adding file: 2 doc(s)
}

func Example_withFilter() {
	dir, _ := os.MkdirTemp("", "fswatcher-filter-example")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("Some notes"), 0o644)
	os.WriteFile(filepath.Join(dir, "image.png"), []byte("not really png"), 0o644)

	store := newMemStore()
	goFilter := fswatcher.WithFilter(func(rel string) bool {
		return filepath.Ext(rel) == ".go"
	})
	opts := []fswatcher.Option{fswatcher.WithFTS(), goFilter}
	m := fswatcher.IndexMapping(opts...)
	idx, _ := searchmd.New(store, m)

	w, _ := fswatcher.New(idx, dir, opts...)
	_ = w.Scan(context.Background())

	count, _ := idx.DocCount()
	fmt.Printf("Indexed %d file(s) (only .go)\n", count)

	// Output:
	// Indexed 1 file(s) (only .go)
}
