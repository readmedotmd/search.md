package document

import (
	"encoding/json"
	"testing"
	"time"
)

// --- TextValue ---

func TestTextValue_String(t *testing.T) {
	f := &Field{Value: "hello"}
	if f.TextValue() != "hello" {
		t.Errorf("expected 'hello', got %q", f.TextValue())
	}
}

func TestTextValue_NonString(t *testing.T) {
	f := &Field{Value: 42}
	if f.TextValue() != "" {
		t.Errorf("expected empty string for non-string value, got %q", f.TextValue())
	}
}

func TestTextValue_Nil(t *testing.T) {
	f := &Field{Value: nil}
	if f.TextValue() != "" {
		t.Errorf("expected empty string for nil, got %q", f.TextValue())
	}
}

// --- NumericValue ---

func TestNumericValue_Float64(t *testing.T) {
	f := &Field{Value: float64(3.14)}
	v, ok := f.NumericValue()
	if !ok || v != 3.14 {
		t.Errorf("expected (3.14, true), got (%f, %v)", v, ok)
	}
}

func TestNumericValue_Float32(t *testing.T) {
	f := &Field{Value: float32(2.5)}
	v, ok := f.NumericValue()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if v != float64(float32(2.5)) {
		t.Errorf("expected %f, got %f", float64(float32(2.5)), v)
	}
}

func TestNumericValue_Int(t *testing.T) {
	f := &Field{Value: int(42)}
	v, ok := f.NumericValue()
	if !ok || v != 42.0 {
		t.Errorf("expected (42, true), got (%f, %v)", v, ok)
	}
}

func TestNumericValue_Int64(t *testing.T) {
	f := &Field{Value: int64(100)}
	v, ok := f.NumericValue()
	if !ok || v != 100.0 {
		t.Errorf("expected (100, true), got (%f, %v)", v, ok)
	}
}

func TestNumericValue_Int32(t *testing.T) {
	f := &Field{Value: int32(50)}
	v, ok := f.NumericValue()
	if !ok || v != 50.0 {
		t.Errorf("expected (50, true), got (%f, %v)", v, ok)
	}
}

func TestNumericValue_Unsupported(t *testing.T) {
	f := &Field{Value: "not a number"}
	v, ok := f.NumericValue()
	if ok || v != 0 {
		t.Errorf("expected (0, false), got (%f, %v)", v, ok)
	}
}

// --- BooleanValue ---

func TestBooleanValue_True(t *testing.T) {
	f := &Field{Value: true}
	v, ok := f.BooleanValue()
	if !ok || !v {
		t.Errorf("expected (true, true), got (%v, %v)", v, ok)
	}
}

func TestBooleanValue_False(t *testing.T) {
	f := &Field{Value: false}
	v, ok := f.BooleanValue()
	if !ok || v {
		t.Errorf("expected (false, true), got (%v, %v)", v, ok)
	}
}

func TestBooleanValue_NonBool(t *testing.T) {
	f := &Field{Value: "true"}
	v, ok := f.BooleanValue()
	if ok {
		t.Errorf("expected ok=false for string, got (%v, %v)", v, ok)
	}
}

// --- DateTimeValue ---

func TestDateTimeValue_TimeType(t *testing.T) {
	now := time.Now()
	f := &Field{Value: now}
	v, ok := f.DateTimeValue()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !v.Equal(now) {
		t.Errorf("expected %v, got %v", now, v)
	}
}

func TestDateTimeValue_RFC3339(t *testing.T) {
	f := &Field{Value: "2024-01-15T10:30:00Z"}
	v, ok := f.DateTimeValue()
	if !ok {
		t.Fatal("expected ok=true for RFC3339")
	}
	if v.Year() != 2024 || v.Month() != 1 || v.Day() != 15 {
		t.Errorf("unexpected date: %v", v)
	}
}

func TestDateTimeValue_RFC3339Nano(t *testing.T) {
	f := &Field{Value: "2024-01-15T10:30:00.123456789Z"}
	v, ok := f.DateTimeValue()
	if !ok {
		t.Fatal("expected ok=true for RFC3339Nano")
	}
	if v.Nanosecond() != 123456789 {
		t.Errorf("expected nanoseconds 123456789, got %d", v.Nanosecond())
	}
}

func TestDateTimeValue_DateOnly(t *testing.T) {
	f := &Field{Value: "2024-01-15"}
	v, ok := f.DateTimeValue()
	if !ok {
		t.Fatal("expected ok=true for date-only format")
	}
	if v.Year() != 2024 || v.Month() != 1 || v.Day() != 15 {
		t.Errorf("unexpected date: %v", v)
	}
}

func TestDateTimeValue_DateTimeNoTZ(t *testing.T) {
	f := &Field{Value: "2024-01-15T10:30:00"}
	v, ok := f.DateTimeValue()
	if !ok {
		t.Fatal("expected ok=true for datetime without TZ")
	}
	if v.Hour() != 10 || v.Minute() != 30 {
		t.Errorf("unexpected time: %v", v)
	}
}

func TestDateTimeValue_InvalidString(t *testing.T) {
	f := &Field{Value: "not a date"}
	_, ok := f.DateTimeValue()
	if ok {
		t.Error("expected ok=false for invalid date string")
	}
}

func TestDateTimeValue_NonStringNonTime(t *testing.T) {
	f := &Field{Value: 42}
	_, ok := f.DateTimeValue()
	if ok {
		t.Error("expected ok=false for integer value")
	}
}

// --- VectorValue ---

func TestVectorValue_Float32Slice(t *testing.T) {
	expected := []float32{1.0, 2.0, 3.0}
	f := &Field{Value: expected}
	v, ok := f.VectorValue()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(v) != 3 || v[0] != 1.0 || v[1] != 2.0 || v[2] != 3.0 {
		t.Errorf("unexpected vector: %v", v)
	}
}

func TestVectorValue_Float64Slice(t *testing.T) {
	f := &Field{Value: []float64{1.0, 2.0, 3.0}}
	v, ok := f.VectorValue()
	if !ok {
		t.Fatal("expected ok=true for []float64")
	}
	if len(v) != 3 || v[0] != 1.0 || v[1] != 2.0 || v[2] != 3.0 {
		t.Errorf("unexpected vector: %v", v)
	}
}

func TestVectorValue_InterfaceSlice_Float64(t *testing.T) {
	f := &Field{Value: []interface{}{float64(1.0), float64(2.0)}}
	v, ok := f.VectorValue()
	if !ok {
		t.Fatal("expected ok=true for []interface{} with float64")
	}
	if len(v) != 2 || v[0] != 1.0 || v[1] != 2.0 {
		t.Errorf("unexpected vector: %v", v)
	}
}

func TestVectorValue_InterfaceSlice_Float32(t *testing.T) {
	f := &Field{Value: []interface{}{float32(1.0), float32(2.0)}}
	v, ok := f.VectorValue()
	if !ok {
		t.Fatal("expected ok=true for []interface{} with float32")
	}
	if len(v) != 2 {
		t.Errorf("expected 2 elements, got %d", len(v))
	}
}

func TestVectorValue_InterfaceSlice_Invalid(t *testing.T) {
	f := &Field{Value: []interface{}{"not", "numbers"}}
	_, ok := f.VectorValue()
	if ok {
		t.Error("expected ok=false for []interface{} with strings")
	}
}

func TestVectorValue_Unsupported(t *testing.T) {
	f := &Field{Value: "not a vector"}
	_, ok := f.VectorValue()
	if ok {
		t.Error("expected ok=false for string value")
	}
}

// --- NewDocument ---

func TestNewDocument(t *testing.T) {
	doc := NewDocument("doc-1")
	if doc.ID != "doc-1" {
		t.Errorf("expected ID 'doc-1', got %q", doc.ID)
	}
	if len(doc.Fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(doc.Fields))
	}
}

// --- AddField ---

func TestAddField(t *testing.T) {
	doc := NewDocument("doc-1")
	f1 := &Field{Name: "title", Value: "hello"}
	f2 := &Field{Name: "body", Value: "world"}
	doc.AddField(f1)
	doc.AddField(f2)
	if len(doc.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(doc.Fields))
	}
}

// --- FieldNamed ---

func TestFieldNamed_Found(t *testing.T) {
	doc := NewDocument("doc-1")
	doc.AddField(&Field{Name: "title", Value: "hello"})
	doc.AddField(&Field{Name: "body", Value: "world"})

	f := doc.FieldNamed("body")
	if f == nil {
		t.Fatal("expected to find 'body' field")
	}
	if f.TextValue() != "world" {
		t.Errorf("expected 'world', got %q", f.TextValue())
	}
}

func TestFieldNamed_NotFound(t *testing.T) {
	doc := NewDocument("doc-1")
	doc.AddField(&Field{Name: "title", Value: "hello"})

	f := doc.FieldNamed("missing")
	if f != nil {
		t.Error("expected nil for missing field")
	}
}

func TestFieldNamed_ReturnsFirst(t *testing.T) {
	doc := NewDocument("doc-1")
	doc.AddField(&Field{Name: "tag", Value: "first"})
	doc.AddField(&Field{Name: "tag", Value: "second"})

	f := doc.FieldNamed("tag")
	if f.TextValue() != "first" {
		t.Errorf("expected first occurrence, got %q", f.TextValue())
	}
}

// --- ToStoredData ---

func TestToStoredData_OnlyStoredFields(t *testing.T) {
	doc := NewDocument("doc-1")
	doc.AddField(&Field{Name: "title", Value: "hello", Store: true})
	doc.AddField(&Field{Name: "internal", Value: "secret", Store: false})
	doc.AddField(&Field{Name: "count", Value: float64(42), Store: true})

	sd := doc.ToStoredData()
	if sd.ID != "doc-1" {
		t.Errorf("expected ID 'doc-1', got %q", sd.ID)
	}
	if len(sd.Fields) != 2 {
		t.Errorf("expected 2 stored fields, got %d", len(sd.Fields))
	}
	if sd.Fields["title"] != "hello" {
		t.Errorf("expected title='hello', got %v", sd.Fields["title"])
	}
	if _, ok := sd.Fields["internal"]; ok {
		t.Error("non-stored field 'internal' should not be in stored data")
	}
	if sd.Fields["count"] != float64(42) {
		t.Errorf("expected count=42, got %v", sd.Fields["count"])
	}
}

func TestToStoredData_NoStoredFields(t *testing.T) {
	doc := NewDocument("doc-1")
	doc.AddField(&Field{Name: "a", Value: "x", Store: false})

	sd := doc.ToStoredData()
	if len(sd.Fields) != 0 {
		t.Errorf("expected 0 stored fields, got %d", len(sd.Fields))
	}
}

func TestToStoredData_EmptyDocument(t *testing.T) {
	doc := NewDocument("empty")
	sd := doc.ToStoredData()
	if sd.ID != "empty" {
		t.Errorf("expected ID 'empty', got %q", sd.ID)
	}
	if len(sd.Fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(sd.Fields))
	}
}

// --- StoredData Marshal/Unmarshal round trip ---

func TestStoredData_MarshalUnmarshal(t *testing.T) {
	original := &StoredData{
		ID: "doc-1",
		Fields: map[string]interface{}{
			"title": "hello world",
			"count": float64(42),
			"flag":  true,
		},
	}

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	restored, err := UnmarshalStoredData(data)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if restored.ID != original.ID {
		t.Errorf("expected ID %q, got %q", original.ID, restored.ID)
	}
	if len(restored.Fields) != len(original.Fields) {
		t.Fatalf("expected %d fields, got %d", len(original.Fields), len(restored.Fields))
	}
	if restored.Fields["title"] != "hello world" {
		t.Errorf("expected title='hello world', got %v", restored.Fields["title"])
	}
	// JSON numbers unmarshal as float64
	if restored.Fields["count"] != float64(42) {
		t.Errorf("expected count=42, got %v", restored.Fields["count"])
	}
	if restored.Fields["flag"] != true {
		t.Errorf("expected flag=true, got %v", restored.Fields["flag"])
	}
}

func TestStoredData_MarshalIsValidJSON(t *testing.T) {
	sd := &StoredData{
		ID:     "test",
		Fields: map[string]interface{}{"key": "value"},
	}
	data, err := sd.Marshal()
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if !json.Valid(data) {
		t.Error("marshaled data is not valid JSON")
	}
}

func TestUnmarshalStoredData_InvalidJSON(t *testing.T) {
	_, err := UnmarshalStoredData([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestStoredData_EmptyFields(t *testing.T) {
	original := &StoredData{
		ID:     "empty",
		Fields: map[string]interface{}{},
	}
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	restored, err := UnmarshalStoredData(data)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(restored.Fields) != 0 {
		t.Errorf("expected 0 fields, got %d", len(restored.Fields))
	}
}
