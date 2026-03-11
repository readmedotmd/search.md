package document

import (
	"encoding/json"
	"time"
)

// FieldType represents the type of a document field.
type FieldType int

const (
	FieldTypeText FieldType = iota
	FieldTypeNumeric
	FieldTypeBoolean
	FieldTypeDateTime
	FieldTypeKeyword
	FieldTypeVector
	FieldTypeCode
	FieldTypeSymbol
)

// Field represents a single field in a document.
type Field struct {
	Name  string
	Type  FieldType
	Value interface{}

	// Analysis options
	Analyzer           string
	Store              bool // whether to store the original value
	Index              bool // whether to index for searching
	IncludeTermVectors bool // whether to include term position info (for highlighting/phrase queries)

	// Vector-specific
	Dims int // dimensionality for vector fields

	// Symbol/code-specific
	Language string // programming language for symbol extraction
}

// TextValue returns the field value as a string.
func (f *Field) TextValue() string {
	if s, ok := f.Value.(string); ok {
		return s
	}
	return ""
}

// NumericValue returns the field value as a float64.
func (f *Field) NumericValue() (float64, bool) {
	switch v := f.Value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return 0, false
	}
}

// BooleanValue returns the field value as a bool.
func (f *Field) BooleanValue() (bool, bool) {
	if b, ok := f.Value.(bool); ok {
		return b, true
	}
	return false, false
}

// DateTimeValue returns the field value as a time.Time.
func (f *Field) DateTimeValue() (time.Time, bool) {
	switch v := f.Value.(type) {
	case time.Time:
		return v, true
	case string:
		for _, layout := range []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02",
			"2006-01-02T15:04:05",
		} {
			if t, err := time.Parse(layout, v); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// VectorValue returns the field value as a float32 slice.
func (f *Field) VectorValue() ([]float32, bool) {
	switch v := f.Value.(type) {
	case []float32:
		return v, true
	case []float64:
		result := make([]float32, len(v))
		for i, val := range v {
			result[i] = float32(val)
		}
		return result, true
	case []interface{}:
		result := make([]float32, len(v))
		for i, val := range v {
			switch n := val.(type) {
			case float64:
				result[i] = float32(n)
			case float32:
				result[i] = n
			default:
				return nil, false
			}
		}
		return result, true
	}
	return nil, false
}

// Document represents a searchable document with an ID and fields.
type Document struct {
	ID     string
	Fields []*Field
}

// NewDocument creates a new document with the given ID.
func NewDocument(id string) *Document {
	return &Document{
		ID:     id,
		Fields: make([]*Field, 0),
	}
}

// AddField adds a field to the document.
func (d *Document) AddField(f *Field) {
	d.Fields = append(d.Fields, f)
}

// FieldNamed returns the first field with the given name.
func (d *Document) FieldNamed(name string) *Field {
	for _, f := range d.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// StoredData represents the stored data of a document for serialization.
type StoredData struct {
	ID     string                 `json:"id"`
	Fields map[string]interface{} `json:"fields"`
}

// ToStoredData converts the document to a storable format (only stored fields).
func (d *Document) ToStoredData() *StoredData {
	sd := &StoredData{
		ID:     d.ID,
		Fields: make(map[string]interface{}),
	}
	for _, f := range d.Fields {
		if f.Store {
			sd.Fields[f.Name] = f.Value
		}
	}
	return sd
}

// MarshalStoredData serializes stored data to JSON.
func (sd *StoredData) Marshal() ([]byte, error) {
	return json.Marshal(sd)
}

// UnmarshalStoredData deserializes stored data from JSON.
func UnmarshalStoredData(data []byte) (*StoredData, error) {
	sd := &StoredData{}
	err := json.Unmarshal(data, sd)
	return sd, err
}
