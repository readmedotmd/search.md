package fswatcher

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	searchmd "github.com/readmedotmd/search.md"
	"github.com/readmedotmd/search.md/mapping"
)

// fileState tracks the last-known modification time for a file.
type fileState struct {
	modTime time.Time
	size    int64
}

// Watcher watches a directory and keeps a search index in sync with it.
type Watcher struct {
	idx  *searchmd.SearchIndex
	root string
	cfg  *config

	mu    sync.Mutex
	known map[string]fileState // relPath -> state
	stop  context.CancelFunc
	done  chan struct{}
}

// New creates a Watcher that indexes files from root into idx.
// At least one indexing mode (WithFTS, WithCode, or WithVector) must be enabled.
func New(idx *searchmd.SearchIndex, root string, opts ...Option) (*Watcher, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("fswatcher: resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("fswatcher: stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("fswatcher: %s is not a directory", abs)
	}

	cfg := defaults()
	for _, o := range opts {
		o(cfg)
	}
	if !cfg.fts && !cfg.code && !cfg.vector {
		return nil, fmt.Errorf("fswatcher: at least one indexing mode must be enabled (WithFTS, WithCode, or WithVector)")
	}
	if cfg.vector && cfg.vectorFn == nil {
		return nil, fmt.Errorf("fswatcher: WithVector requires a non-nil embed function")
	}

	return &Watcher{
		idx:   idx,
		root:  abs,
		cfg:   cfg,
		known: make(map[string]fileState),
	}, nil
}

// Start performs an initial full scan and then polls for changes until the
// context is cancelled or Stop is called.
func (w *Watcher) Start(ctx context.Context) error {
	if err := w.Scan(ctx); err != nil {
		return fmt.Errorf("fswatcher: initial scan: %w", err)
	}

	ctx, w.stop = context.WithCancel(ctx)
	w.done = make(chan struct{})

	go w.pollLoop(ctx)
	return nil
}

// Stop stops the watcher. It blocks until the poll loop exits.
func (w *Watcher) Stop() {
	if w.stop != nil {
		w.stop()
		<-w.done
	}
}

// Scan walks the directory tree and indexes all matching files. Files that
// haven't changed since the last scan are skipped. Deleted files are removed
// from the index.
func (w *Watcher) Scan(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	seen := make(map[string]bool)
	batch := w.idx.NewBatch()
	var batchErr error

	// Buffer pending known-map updates; apply only after successful batch execution.
	type pendingUpdate struct {
		rel   string
		state fileState
	}
	var pendingUpdates []pendingUpdate

	// commitPending applies buffered known-map updates and resets the buffer.
	commitPending := func() {
		for _, pu := range pendingUpdates {
			w.known[pu.rel] = pu.state
		}
		pendingUpdates = pendingUpdates[:0]
	}

	err := filepath.WalkDir(w.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			// Skip hidden directories.
			if strings.HasPrefix(d.Name(), ".") && path != w.root {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(w.root, path)
		if err != nil {
			return nil
		}

		// Apply filter with panic recovery.
		if w.cfg.filter != nil {
			skip := func() (skip bool) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("fswatcher: panic in FilterFunc for %s: %v", rel, r)
						skip = true
					}
				}()
				return !w.cfg.filter(rel)
			}()
			if skip {
				return nil
			}
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Skip files that exceed the maximum file size.
		if w.cfg.maxFileSize > 0 && info.Size() > w.cfg.maxFileSize {
			return nil
		}

		seen[rel] = true

		// Skip unchanged files.
		if prev, ok := w.known[rel]; ok {
			if prev.modTime.Equal(info.ModTime()) && prev.size == info.Size() {
				return nil
			}
		}

		doc, err := w.buildDocument(path, rel, info)
		if err != nil {
			return nil // skip files we can't read
		}

		docID := docIDFromRel(rel)
		batch.Index(docID, doc)
		pendingUpdates = append(pendingUpdates, pendingUpdate{
			rel:   rel,
			state: fileState{modTime: info.ModTime(), size: info.Size()},
		})

		if batch.Size() >= w.cfg.batchSize {
			if err := batch.Execute(); err != nil {
				batchErr = err
				return err
			}
			commitPending()
			batch = w.idx.NewBatch()
		}
		return nil
	})
	if err != nil && err != context.Canceled {
		return err
	}
	if batchErr != nil {
		return batchErr
	}

	// Flush remaining.
	if batch.Size() > 0 {
		if err := batch.Execute(); err != nil {
			return err
		}
		commitPending()
	}

	// Remove deleted files.
	for rel := range w.known {
		if !seen[rel] {
			_ = w.idx.Delete(docIDFromRel(rel))
			delete(w.known, rel)
		}
	}

	return nil
}

func (w *Watcher) pollLoop(ctx context.Context) {
	defer close(w.done)
	ticker := time.NewTicker(w.cfg.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = w.Scan(ctx)
		}
	}
}

// buildDocument creates the document map for a file based on enabled modes.
func (w *Watcher) buildDocument(absPath, relPath string, info os.FileInfo) (map[string]interface{}, error) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	// Skip binary files (simple heuristic: check for null bytes in first 512 bytes).
	check := content
	if len(check) > 512 {
		check = check[:512]
	}
	for _, b := range check {
		if b == 0 {
			return nil, fmt.Errorf("binary file")
		}
	}

	text := string(content)

	basename := filepath.Base(relPath)
	doc := map[string]interface{}{
		"path":          relPath,
		"filename":      basename,
		"filename_text": basename,
		"ext":           strings.TrimPrefix(filepath.Ext(relPath), "."),
		"size":          float64(info.Size()),
		"modified":      info.ModTime().UTC().Format(time.RFC3339),
	}

	if w.cfg.fts {
		doc["content"] = text
	}

	if w.cfg.code {
		doc["code"] = text
		lang := w.cfg.lang
		if lang == "" {
			lang = langFromExt(filepath.Ext(relPath))
		}
		if lang != "" {
			doc["source"] = text
			doc["_lang"] = lang
		}
	}

	if w.cfg.vector && w.cfg.vectorFn != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("fswatcher: panic in EmbedFunc for %s: %v", relPath, r)
				}
			}()
			vec, err := w.cfg.vectorFn(text)
			if err == nil && len(vec) > 0 {
				doc["embedding"] = vec
			}
		}()
	}

	return doc, nil
}

// IndexMapping returns a mapping configured for the enabled indexing modes.
// Use this to create the SearchIndex that will be passed to New.
func IndexMapping(opts ...Option) *mapping.IndexMapping {
	cfg := defaults()
	for _, o := range opts {
		o(cfg)
	}

	m := mapping.NewIndexMapping()
	dm := mapping.NewDocumentStaticMapping()

	// Always-present metadata fields.
	dm.AddFieldMapping("path", mapping.NewKeywordFieldMapping())
	dm.AddFieldMapping("filename", mapping.NewKeywordFieldMapping())
	dm.AddFieldMapping("filename_text", mapping.NewTextFieldMapping())
	dm.AddFieldMapping("ext", mapping.NewKeywordFieldMapping())
	dm.AddFieldMapping("size", mapping.NewNumericFieldMapping())
	dm.AddFieldMapping("modified", mapping.NewDateTimeFieldMapping())

	if cfg.fts {
		dm.AddFieldMapping("content", mapping.NewTextFieldMapping())
	}

	if cfg.code {
		dm.AddFieldMapping("code", mapping.NewCodeFieldMapping())
		lang := cfg.lang
		if lang == "" {
			lang = "auto"
		}
		dm.AddFieldMapping("source", mapping.NewSymbolFieldMapping(lang))
	}

	if cfg.vector && cfg.dims > 0 {
		dm.AddFieldMapping("embedding", mapping.NewVectorFieldMapping(cfg.dims))
	}

	m.DefaultMapping = dm
	return m
}

// docIDFromRel converts a relative path to a valid document ID by replacing
// path separators with colons (document IDs cannot contain '/').
func docIDFromRel(rel string) string {
	return strings.ReplaceAll(filepath.ToSlash(rel), "/", ":")
}

// RelPathFromDocID converts a document ID back to a relative path.
func RelPathFromDocID(docID string) string {
	return strings.ReplaceAll(docID, ":", "/")
}

// langFromExt maps file extensions to language names for symbol extraction.
func langFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx":
		return "cpp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	default:
		return ""
	}
}
