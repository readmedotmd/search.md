package mapping

import (
	"encoding/json"
	"testing"

	"github.com/readmedotmd/search.md/document"
)

func FuzzMapDocument(f *testing.F) {
	f.Add([]byte(`{"title": "hello", "body": "world"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"count": 42, "active": true}`))
	f.Add([]byte(`{"nested": {"key": "value"}}`))
	f.Add([]byte(`{"_type": "article", "title": "test"}`))
	f.Add([]byte(`{"date": "2024-01-01T00:00:00Z"}`))
	f.Add([]byte(`{"a": 1.5, "b": "text", "c": true, "d": null}`))
	f.Add([]byte(`not valid json`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			// Not valid JSON; skip.
			return
		}

		im := NewIndexMapping()
		doc := document.NewDocument("fuzz-doc")

		// Must not panic on any valid JSON object.
		_ = im.MapDocument(doc, m)
	})
}
