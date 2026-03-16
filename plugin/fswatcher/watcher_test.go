package fswatcher_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/readmedotmd/store.md/backend/memory"

	searchmd "github.com/readmedotmd/search.md"
	"github.com/readmedotmd/search.md/plugin/codesearch"
	"github.com/readmedotmd/search.md/plugin/fswatcher"
	"github.com/readmedotmd/search.md/search/query"
)

func newMemStore() *memory.StoreMemory {
	return memory.New()
}

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Create some test files.
	writeFile(t, dir, "hello.txt", "Hello, World! This is a test document.")
	writeFile(t, dir, "readme.md", "# README\n\nThis project is great.")
	writeFile(t, dir, "sub/nested.txt", "Nested file content here.")
	return dir
}

func writeFile(t *testing.T, base, rel, content string) {
	t.Helper()
	path := filepath.Join(base, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNew_Validation(t *testing.T) {
	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("no modes enabled", func(t *testing.T) {
		_, err := fswatcher.New(idx, t.TempDir())
		if err == nil {
			t.Error("expected error when no indexing mode is enabled")
		}
	})

	t.Run("not a directory", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "file.txt")
		os.WriteFile(f, []byte("x"), 0o644)
		_, err := fswatcher.New(idx, f, fswatcher.WithFTS())
		if err == nil {
			t.Error("expected error for non-directory path")
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		_, err := fswatcher.New(idx, "/nonexistent/path/xyz", fswatcher.WithFTS())
		if err == nil {
			t.Error("expected error for nonexistent path")
		}
	})

	t.Run("vector without func", func(t *testing.T) {
		_, err := fswatcher.New(idx, t.TempDir(), fswatcher.WithVector(128, nil))
		if err == nil {
			t.Error("expected error for vector mode without embed func")
		}
	})
}

func TestScan_FTS(t *testing.T) {
	dir := setupTestDir(t)
	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 documents, got %d", count)
	}

	// Check a document exists with expected ID.
	doc, err := idx.Document("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if doc["path"] != "hello.txt" {
		t.Errorf("expected path 'hello.txt', got %v", doc["path"])
	}
	if doc["content"] == nil {
		t.Error("expected content field to be set for FTS mode")
	}

	// Check nested file uses colon-separated ID.
	doc, err = idx.Document("sub:nested.txt")
	if err != nil {
		t.Fatal(err)
	}
	if doc["path"] != "sub/nested.txt" {
		t.Errorf("expected path 'sub/nested.txt', got %v", doc["path"])
	}
}

func TestScan_SkipsUnchanged(t *testing.T) {
	dir := setupTestDir(t)
	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	// Second scan should not error (unchanged files are skipped).
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 documents after second scan, got %d", count)
	}
}

func TestScan_DetectsChanges(t *testing.T) {
	dir := setupTestDir(t)
	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	// Modify a file.
	time.Sleep(10 * time.Millisecond) // ensure different mtime
	writeFile(t, dir, "hello.txt", "Updated content for testing changes.")
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	doc, err := idx.Document("hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	content, ok := doc["content"].(string)
	if !ok {
		t.Fatal("expected content to be a string")
	}
	if !strings.Contains(content, "Updated") {
		t.Errorf("expected updated content, got %q", content)
	}
}

func TestScan_DetectsDeletes(t *testing.T) {
	dir := setupTestDir(t)
	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	// Delete a file.
	os.Remove(filepath.Join(dir, "hello.txt"))
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 documents after deletion, got %d", count)
	}
}

func TestScan_NewFiles(t *testing.T) {
	dir := setupTestDir(t)
	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	// Add a new file.
	writeFile(t, dir, "new.txt", "Brand new file.")
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Errorf("expected 4 documents after adding file, got %d", count)
	}
}

func TestScan_WithFilter(t *testing.T) {
	dir := setupTestDir(t)
	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	// Only index .txt files.
	w, err := fswatcher.New(idx, dir,
		fswatcher.WithFTS(),
		fswatcher.WithFilter(func(rel string) bool {
			return strings.HasSuffix(rel, ".txt")
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 .txt documents, got %d", count)
	}
}

func TestScan_Code(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", `package main

func Hello() string {
	return "hello"
}

type Server struct {
	Port int
}
`)

	store := newMemStore()
	opts := []fswatcher.Option{fswatcher.WithCode("go")}
	m := fswatcher.IndexMapping(opts...)
	idx, err := searchmd.New(store, m, codesearch.WithRegexSymbolIndexer())
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, opts...)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 document, got %d", count)
	}

	doc, err := idx.Document("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if doc["code"] == nil {
		t.Error("expected code field to be set")
	}
}

func TestScan_Vector(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "doc.txt", "Some text for embedding.")

	store := newMemStore()
	embedCalled := false
	embedFn := func(text string) ([]float32, error) {
		embedCalled = true
		return []float32{0.1, 0.2, 0.3, 0.4}, nil
	}

	opts := []fswatcher.Option{fswatcher.WithVector(4, embedFn)}
	m := fswatcher.IndexMapping(opts...)
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, opts...)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !embedCalled {
		t.Error("expected embed function to be called")
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 document, got %d", count)
	}
}

func TestScan_AllModes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.go", `package main

func main() {
	fmt.Println("hello")
}
`)

	store := newMemStore()
	embedFn := func(text string) ([]float32, error) {
		return []float32{1.0, 2.0}, nil
	}

	opts := []fswatcher.Option{
		fswatcher.WithAll("go"),
		fswatcher.WithVector(2, embedFn),
	}
	m := fswatcher.IndexMapping(opts...)
	idx, err := searchmd.New(store, m, codesearch.WithRegexSymbolIndexer())
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, opts...)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	doc, err := idx.Document("app.go")
	if err != nil {
		t.Fatal(err)
	}

	// All fields should be present.
	if doc["content"] == nil {
		t.Error("expected content field (FTS)")
	}
	if doc["code"] == nil {
		t.Error("expected code field")
	}
	if doc["path"] == nil {
		t.Error("expected path field")
	}
}

func TestScan_SkipsBinaryFiles(t *testing.T) {
	dir := t.TempDir()
	// Write a binary file with null bytes.
	path := filepath.Join(dir, "binary.dat")
	os.WriteFile(path, []byte("hello\x00world"), 0o644)
	writeFile(t, dir, "text.txt", "normal text")

	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 document (binary skipped), got %d", count)
	}
}

func TestScan_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "visible.txt", "visible")
	writeFile(t, dir, ".hidden/secret.txt", "hidden")

	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 document (.hidden skipped), got %d", count)
	}
}

func TestStartStop(t *testing.T) {
	dir := setupTestDir(t)
	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir,
		fswatcher.WithFTS(),
		fswatcher.WithPollInterval(50*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Initial scan should have indexed files.
	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 documents after start, got %d", count)
	}

	// Add a file and wait for poll to pick it up.
	writeFile(t, dir, "added.txt", "Added during watch.")
	time.Sleep(200 * time.Millisecond)

	count, err = idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Errorf("expected 4 documents after poll detected new file, got %d", count)
	}

	w.Stop()
}

func TestDocIDRoundTrip(t *testing.T) {
	rel := "src/main/app.go"
	id := "src:main:app.go" // what docIDFromRel produces
	got := fswatcher.RelPathFromDocID(id)
	if got != rel {
		t.Errorf("RelPathFromDocID(%q) = %q, want %q", id, got, rel)
	}
}

func TestIndexMapping(t *testing.T) {
	// Just verify it doesn't panic and produces a valid mapping.
	m := fswatcher.IndexMapping(
		fswatcher.WithFTS(),
		fswatcher.WithCode("go"),
		fswatcher.WithVector(128, nil),
	)
	if m == nil {
		t.Fatal("expected non-nil mapping")
	}
}

func TestScan_MetadataFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src/app.go", "package main")

	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	doc, err := idx.Document("src:app.go")
	if err != nil {
		t.Fatal(err)
	}

	// Check all metadata fields are populated.
	if doc["path"] != "src/app.go" {
		t.Errorf("path = %v, want src/app.go", doc["path"])
	}
	if doc["filename"] != "app.go" {
		t.Errorf("filename = %v, want app.go", doc["filename"])
	}
	if doc["filename_text"] != "app.go" {
		t.Errorf("filename_text = %v, want app.go", doc["filename_text"])
	}
	if doc["ext"] != "go" {
		t.Errorf("ext = %v, want go", doc["ext"])
	}
	if doc["size"] == nil {
		t.Error("expected size field")
	}
	if doc["modified"] == nil {
		t.Error("expected modified field")
	}
}

func TestScan_FTSSearchQuery(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "alpha.txt", "The quick brown fox jumps over the lazy dog.")
	writeFile(t, dir, "beta.txt", "A fast red car drives through the city streets.")

	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Search for "fox" should find alpha.txt.
	results, err := idx.Search(ctx, query.NewMatchQuery("fox").SetField("content"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 1 {
		t.Errorf("expected 1 result for 'fox', got %d", results.Total)
	}
	if results.Total > 0 && results.Hits[0].ID != "alpha.txt" {
		t.Errorf("expected alpha.txt, got %s", results.Hits[0].ID)
	}

	// Search for "city" should find beta.txt.
	results, err = idx.Search(ctx, query.NewMatchQuery("city").SetField("content"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 1 {
		t.Errorf("expected 1 result for 'city', got %d", results.Total)
	}

	// Search for nonexistent term.
	results, err = idx.Search(ctx, query.NewMatchQuery("elephant").SetField("content"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 0 {
		t.Errorf("expected 0 results for 'elephant', got %d", results.Total)
	}

	// Search by keyword field (path).
	results, err = idx.Search(ctx, query.NewTermQuery("alpha.txt").SetField("path"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 1 {
		t.Errorf("expected 1 result for path 'alpha.txt', got %d", results.Total)
	}

	// Search by extension.
	results, err = idx.Search(ctx, query.NewTermQuery("txt").SetField("ext"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 2 {
		t.Errorf("expected 2 results for ext 'txt', got %d", results.Total)
	}
}

func TestScan_FilenameTextSearch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "server_utils.go", "package utils")
	writeFile(t, dir, "server_handler.go", "package handler")
	writeFile(t, dir, "client.go", "package client")

	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Partial match on analyzed filename_text should find both server files.
	results, err := idx.Search(ctx, query.NewMatchQuery("server").SetField("filename_text"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 2 {
		t.Errorf("expected 2 results for 'server' in filename_text, got %d", results.Total)
	}

	// Exact keyword match on filename should find only exact name.
	results, err = idx.Search(ctx, query.NewTermQuery("client.go").SetField("filename"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 1 {
		t.Errorf("expected 1 result for exact filename 'client.go', got %d", results.Total)
	}

	// Partial keyword match should NOT work on the keyword field.
	results, err = idx.Search(ctx, query.NewTermQuery("server").SetField("filename"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 0 {
		t.Errorf("expected 0 results for partial keyword 'server' on filename, got %d", results.Total)
	}
}

func TestScan_CodeSearchQuery(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "handler.go", `package api

type RequestHandler struct {
	Logger Logger
}

func NewRequestHandler(logger Logger) *RequestHandler {
	return &RequestHandler{Logger: logger}
}

func (h *RequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("request received")
}

type Logger interface {
	Info(msg string)
	Error(msg string)
}
`)

	store := newMemStore()
	opts := []fswatcher.Option{fswatcher.WithCode("go")}
	m := fswatcher.IndexMapping(opts...)
	idx, err := searchmd.New(store, m, codesearch.WithRegexSymbolIndexer())
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Search for struct kind.
	results, err := idx.Search(ctx, query.NewTermQuery("struct").SetField("source.kind"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected results for 'struct' kind")
	}

	// Search for interface kind.
	results, err = idx.Search(ctx, query.NewTermQuery("interface").SetField("source.kind"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected results for 'interface' kind")
	}

	// Search for specific symbol.
	results, err = idx.Search(ctx, query.NewTermQuery("newrequesthandler").SetField("source.sym"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 1 {
		t.Errorf("expected 1 result for 'newrequesthandler' symbol, got %d", results.Total)
	}

	// Code-aware search on code field (camelCase splitting).
	results, err = idx.Search(ctx, query.NewMatchQuery("request handler").SetField("code"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected results for code search 'request handler' (camelCase split)")
	}
}

func TestScan_VectorSearchQuery(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "doc1.txt", "Machine learning is fascinating.")
	writeFile(t, dir, "doc2.txt", "Cooking recipes for pasta.")

	store := newMemStore()

	// Simple mock embedder: returns different vectors for different content.
	embedFn := func(text string) ([]float32, error) {
		if strings.Contains(text, "Machine") {
			return []float32{1.0, 0.0, 0.0, 0.0}, nil
		}
		return []float32{0.0, 0.0, 0.0, 1.0}, nil
	}

	opts := []fswatcher.Option{fswatcher.WithVector(4, embedFn)}
	m := fswatcher.IndexMapping(opts...)
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	// KNN search with vector close to doc1.
	results, err := idx.Search(context.Background(),
		query.NewKNNQuery([]float32{0.9, 0.1, 0.0, 0.0}, 1).SetField("embedding"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Fatal("expected at least 1 KNN result")
	}
	if results.Hits[0].ID != "doc1.txt" {
		t.Errorf("expected doc1.txt as top KNN hit, got %s", results.Hits[0].ID)
	}
}

func TestScan_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	// Create enough files that cancellation has a chance to take effect.
	for i := 0; i < 50; i++ {
		writeFile(t, dir, fmt.Sprintf("file%03d.txt", i), fmt.Sprintf("Content of file %d", i))
	}

	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err = w.Scan(ctx)
	// Should either return context.Canceled or succeed (if scan completes before check).
	// Either way, it should not hang.
	if err != nil && err != context.Canceled {
		t.Fatalf("expected nil or context.Canceled, got %v", err)
	}
}

func TestScan_WithBatchSize(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 10; i++ {
		writeFile(t, dir, fmt.Sprintf("file%d.txt", i), fmt.Sprintf("Content %d", i))
	}

	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	// Use a batch size of 3 to force multiple batch flushes.
	w, err := fswatcher.New(idx, dir,
		fswatcher.WithFTS(),
		fswatcher.WithBatchSize(3),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 10 {
		t.Errorf("expected 10 documents with batch size 3, got %d", count)
	}
}

func TestScan_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	count, err := idx.DocCount()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("expected 0 documents for empty dir, got %d", count)
	}
}

func TestScan_DeeplyNested(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a/b/c/d/deep.txt", "Deeply nested file.")

	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Document ID should use colons for path separators.
	doc, err := idx.Document("a:b:c:d:deep.txt")
	if err != nil {
		t.Fatal(err)
	}
	if doc["path"] != "a/b/c/d/deep.txt" {
		t.Errorf("path = %v, want a/b/c/d/deep.txt", doc["path"])
	}
}

func TestScan_CodeAutoDetectLanguage(t *testing.T) {
	dir := t.TempDir()

	// Write files in different languages — all should get symbols extracted
	// with auto-detected language.
	writeFile(t, dir, "main.go", `package main
func AutoDetectedFunc() {}
`)
	writeFile(t, dir, "app.py", `def auto_detected_func():
    pass
`)

	store := newMemStore()
	opts := []fswatcher.Option{fswatcher.WithCode("")} // empty = auto-detect
	m := fswatcher.IndexMapping(opts...)
	idx, err := searchmd.New(store, m, codesearch.WithRegexSymbolIndexer())
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Go function should be found.
	results, err := idx.Search(ctx, query.NewTermQuery("autodetectedfunc").SetField("source.sym"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected to find Go function via auto-detected language")
	}

	// Python function should be found.
	results, err = idx.Search(ctx, query.NewTermQuery("auto_detected_func").SetField("source.sym"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected to find Python function via auto-detected language")
	}
}

func TestScan_ModifyWithCodeMode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "lib.go", `package lib
func OldFunction() {}
`)

	store := newMemStore()
	opts := []fswatcher.Option{fswatcher.WithCode("go")}
	m := fswatcher.IndexMapping(opts...)
	idx, err := searchmd.New(store, m, codesearch.WithRegexSymbolIndexer())
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, opts...)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	// Old function should be found.
	results, err := idx.Search(ctx, query.NewTermQuery("oldfunction").SetField("source.sym"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected to find OldFunction before modification")
	}

	// Modify the file.
	time.Sleep(10 * time.Millisecond)
	writeFile(t, dir, "lib.go", `package lib
func NewFunction() {}
`)
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	// Old function should be gone.
	results, err = idx.Search(ctx, query.NewTermQuery("oldfunction").SetField("source.sym"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total != 0 {
		t.Errorf("expected 0 results for old function after modification, got %d", results.Total)
	}

	// New function should be found.
	results, err = idx.Search(ctx, query.NewTermQuery("newfunction").SetField("source.sym"))
	if err != nil {
		t.Fatal(err)
	}
	if results.Total == 0 {
		t.Error("expected to find NewFunction after modification")
	}
}

func TestRelPathFromDocID(t *testing.T) {
	tests := []struct {
		docID string
		want  string
	}{
		{"file.txt", "file.txt"},
		{"src:main.go", "src/main.go"},
		{"a:b:c:d.txt", "a/b/c/d.txt"},
	}
	for _, tt := range tests {
		got := fswatcher.RelPathFromDocID(tt.docID)
		if got != tt.want {
			t.Errorf("RelPathFromDocID(%q) = %q, want %q", tt.docID, got, tt.want)
		}
	}
}

func TestScan_MultipleDeletesAndAdds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "File A")
	writeFile(t, dir, "b.txt", "File B")
	writeFile(t, dir, "c.txt", "File C")

	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir, fswatcher.WithFTS())
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	count, _ := idx.DocCount()
	if count != 3 {
		t.Fatalf("expected 3 documents, got %d", count)
	}

	// Delete two, add one.
	os.Remove(filepath.Join(dir, "a.txt"))
	os.Remove(filepath.Join(dir, "b.txt"))
	writeFile(t, dir, "d.txt", "File D")

	if err := w.Scan(ctx); err != nil {
		t.Fatal(err)
	}

	count, _ = idx.DocCount()
	if count != 2 {
		t.Errorf("expected 2 documents (c + d), got %d", count)
	}

	// Verify the right documents exist.
	if _, err := idx.Document("c.txt"); err != nil {
		t.Error("expected c.txt to exist")
	}
	if _, err := idx.Document("d.txt"); err != nil {
		t.Error("expected d.txt to exist")
	}
}

func TestStartStop_ContextCancellation(t *testing.T) {
	dir := setupTestDir(t)
	store := newMemStore()
	m := fswatcher.IndexMapping(fswatcher.WithFTS())
	idx, err := searchmd.New(store, m)
	if err != nil {
		t.Fatal(err)
	}

	w, err := fswatcher.New(idx, dir,
		fswatcher.WithFTS(),
		fswatcher.WithPollInterval(50*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Cancel context should stop the watcher.
	cancel()

	// Stop should return promptly without hanging.
	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() hung after context cancellation")
	}
}
