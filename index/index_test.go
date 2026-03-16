package index_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/readmedotmd/store.md/backend/memory"

	"github.com/readmedotmd/search.md/document"
	"github.com/readmedotmd/search.md/index"
)

func newMemStore() *memory.StoreMemory {
	return memory.New()
}

// recordingLogger captures log messages for verification.
type recordingLogger struct {
	mu       sync.Mutex
	messages []string
}

func (l *recordingLogger) Warn(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, fmt.Sprintf(msg, args...))
}

func (l *recordingLogger) Messages() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]string, len(l.messages))
	copy(cp, l.messages)
	return cp
}

// makeTextDoc creates a simple document with a single indexed text field.
func makeTextDoc(id, fieldName, text string) *document.Document {
	doc := document.NewDocument(id)
	doc.AddField(&document.Field{
		Name:  fieldName,
		Type:  document.FieldTypeText,
		Value: text,
		Store: true,
		Index: true,
	})
	return doc
}

func TestNew_VersionMismatch(t *testing.T) {
	store := newMemStore()

	// Create an index to initialise the store.
	_, err := index.New(store)
	if err != nil {
		t.Fatalf("first New: %v", err)
	}

	// Manually overwrite the version to something incompatible.
	if err := store.Set(context.Background(), "m/index_version", "99"); err != nil {
		t.Fatalf("set version: %v", err)
	}

	// A second New should detect the mismatch.
	_, err = index.New(store)
	if err == nil {
		t.Fatal("expected error for version mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "version mismatch") {
		t.Fatalf("expected error containing 'version mismatch', got: %v", err)
	}
}

func TestNew_VersionInitialization(t *testing.T) {
	store := newMemStore()

	idx, err := index.New(store)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ver := idx.IndexVersion()
	if ver != "1" {
		t.Fatalf("expected version %q, got %q", "1", ver)
	}
}

func TestSetLogger(t *testing.T) {
	t.Run("NopLogger", func(t *testing.T) {
		store := newMemStore()
		idx, err := index.New(store)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		// Should not panic.
		idx.SetLogger(index.NopLogger{})
	})

	t.Run("DefaultLogger", func(t *testing.T) {
		store := newMemStore()
		idx, err := index.New(store)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		// Should not panic.
		idx.SetLogger(index.DefaultLogger{})
	})

	t.Run("RecordingLogger_FullScanFallback", func(t *testing.T) {
		store := newMemStore()
		idx, err := index.New(store)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		rec := &recordingLogger{}
		idx.SetLogger(rec)

		// Index a document so it exists in the store.
		doc := makeTextDoc("doc1", "body", "hello world")
		if err := idx.IndexDocument(doc); err != nil {
			t.Fatalf("IndexDocument: %v", err)
		}

		// Remove the reverse index entry so deletion falls back to full scan.
		if err := store.Delete(context.Background(), "ri/doc1"); err != nil {
			t.Fatalf("delete reverse index: %v", err)
		}

		// Deleting should trigger the full-scan path, which logs a warning.
		if err := idx.DeleteDocument("doc1"); err != nil {
			t.Fatalf("DeleteDocument: %v", err)
		}

		msgs := rec.Messages()
		found := false
		for _, m := range msgs {
			if strings.Contains(m, "full-scan") || strings.Contains(m, "falling back") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected a warning about full-scan deletion, got messages: %v", msgs)
		}
	})
}

func TestGetDocCount_CorruptValue(t *testing.T) {
	store := newMemStore()
	idx, err := index.New(store)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Manually corrupt the doc count.
	if err := store.Set(context.Background(), "m/doc_count", "not_a_number"); err != nil {
		t.Fatalf("set doc count: %v", err)
	}

	_, err = idx.DocCount()
	if err == nil {
		t.Fatal("expected error for corrupt doc count, got nil")
	}
}

func TestTermPostings_CorruptJSON(t *testing.T) {
	store := newMemStore()
	idx, err := index.New(store)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Store corrupt JSON under a term posting key.
	// Key format: t/{field}/{term}/{docID}
	if err := store.Set(context.Background(), "t/body/hello/doc1", "NOT VALID JSON{{{"); err != nil {
		t.Fatalf("set corrupt posting: %v", err)
	}

	// TermPostings should skip the corrupt entry rather than crashing.
	postings, err := idx.TermPostings("body", "hello")
	if err != nil {
		t.Fatalf("TermPostings returned error: %v", err)
	}
	if len(postings) != 0 {
		t.Fatalf("expected 0 postings (corrupt entry skipped), got %d", len(postings))
	}
}

func TestIndexDocument_And_Delete(t *testing.T) {
	store := newMemStore()
	idx, err := index.New(store)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Run("IndexAndVerifyCount", func(t *testing.T) {
		doc := makeTextDoc("doc1", "body", "the quick brown fox")
		if err := idx.IndexDocument(doc); err != nil {
			t.Fatalf("IndexDocument: %v", err)
		}

		count, err := idx.DocCount()
		if err != nil {
			t.Fatalf("DocCount: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected doc count 1, got %d", count)
		}
	})

	t.Run("GetDocumentAfterIndex", func(t *testing.T) {
		sd, err := idx.GetDocument("doc1")
		if err != nil {
			t.Fatalf("GetDocument: %v", err)
		}
		if sd.ID != "doc1" {
			t.Fatalf("expected doc ID %q, got %q", "doc1", sd.ID)
		}
	})

	t.Run("DeleteAndVerifyCount", func(t *testing.T) {
		if err := idx.DeleteDocument("doc1"); err != nil {
			t.Fatalf("DeleteDocument: %v", err)
		}

		count, err := idx.DocCount()
		if err != nil {
			t.Fatalf("DocCount: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected doc count 0 after delete, got %d", count)
		}
	})

	t.Run("GetDocumentAfterDelete", func(t *testing.T) {
		_, err := idx.GetDocument("doc1")
		if err == nil {
			t.Fatal("expected error getting deleted document, got nil")
		}
	})
}

func TestFieldLength(t *testing.T) {
	store := newMemStore()
	idx, err := index.New(store)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	doc := makeTextDoc("doc1", "body", "one two three four five")
	if err := idx.IndexDocument(doc); err != nil {
		t.Fatalf("IndexDocument: %v", err)
	}

	length, err := idx.FieldLength("body", "doc1")
	if err != nil {
		t.Fatalf("FieldLength: %v", err)
	}
	if length <= 0 {
		t.Fatalf("expected positive field length, got %d", length)
	}
}
