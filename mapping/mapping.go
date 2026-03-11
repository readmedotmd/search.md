package mapping

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/readmedotmd/search.md/document"
)

// FieldMapping describes how a single field should be indexed.
type FieldMapping struct {
	Name               string             `json:"name,omitempty"`
	Type               document.FieldType `json:"type"`
	Analyzer           string             `json:"analyzer,omitempty"`
	Store              bool               `json:"store"`
	Index              bool               `json:"index"`
	IncludeTermVectors bool               `json:"include_term_vectors"`
	Dims               int                `json:"dims,omitempty"`     // for vector fields
	Language           string             `json:"language,omitempty"` // for symbol fields
}

// NewTextFieldMapping returns a default text field mapping.
func NewTextFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:               document.FieldTypeText,
		Analyzer:           "standard",
		Store:              true,
		Index:              true,
		IncludeTermVectors: true,
	}
}

// NewKeywordFieldMapping returns a field mapping that indexes text without analysis.
func NewKeywordFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:     document.FieldTypeKeyword,
		Analyzer: "keyword",
		Store:    true,
		Index:    true,
	}
}

// NewNumericFieldMapping returns a default numeric field mapping.
func NewNumericFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:  document.FieldTypeNumeric,
		Store: true,
		Index: true,
	}
}

// NewBooleanFieldMapping returns a default boolean field mapping.
func NewBooleanFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:  document.FieldTypeBoolean,
		Store: true,
		Index: true,
	}
}

// NewDateTimeFieldMapping returns a default datetime field mapping.
func NewDateTimeFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:  document.FieldTypeDateTime,
		Store: true,
		Index: true,
	}
}

// NewVectorFieldMapping returns a vector field mapping with the given dimensions.
func NewVectorFieldMapping(dims int) *FieldMapping {
	return &FieldMapping{
		Type:  document.FieldTypeVector,
		Store: true,
		Index: true,
		Dims:  dims,
	}
}

// NewCodeFieldMapping returns a code field mapping.
func NewCodeFieldMapping() *FieldMapping {
	return &FieldMapping{
		Type:               document.FieldTypeCode,
		Analyzer:           "code",
		Store:              true,
		Index:              true,
		IncludeTermVectors: true,
	}
}

// NewSymbolFieldMapping returns a symbol field mapping for code symbol extraction.
// Language specifies the programming language for symbol extraction (e.g., "go", "python", "javascript").
func NewSymbolFieldMapping(language string) *FieldMapping {
	return &FieldMapping{
		Type:     document.FieldTypeSymbol,
		Store:    true,
		Index:    true,
		Language: language,
	}
}

// DocumentMapping describes how a document type should be indexed.
type DocumentMapping struct {
	Enabled     bool                        `json:"enabled"`
	Dynamic     bool                        `json:"dynamic"`
	Fields      map[string]*FieldMapping    `json:"fields,omitempty"`
	SubMappings map[string]*DocumentMapping `json:"sub_mappings,omitempty"`
}

// NewDocumentMapping returns a new document mapping with default settings.
func NewDocumentMapping() *DocumentMapping {
	return &DocumentMapping{
		Enabled:     true,
		Dynamic:     true,
		Fields:      make(map[string]*FieldMapping),
		SubMappings: make(map[string]*DocumentMapping),
	}
}

// NewDocumentStaticMapping returns a document mapping that won't auto-index unmapped fields.
func NewDocumentStaticMapping() *DocumentMapping {
	return &DocumentMapping{
		Enabled:     true,
		Dynamic:     false,
		Fields:      make(map[string]*FieldMapping),
		SubMappings: make(map[string]*DocumentMapping),
	}
}

// AddFieldMapping adds a field mapping.
func (dm *DocumentMapping) AddFieldMapping(name string, fm *FieldMapping) {
	fm.Name = name
	dm.Fields[name] = fm
}

// AddSubDocumentMapping adds a sub-document mapping for nested objects.
func (dm *DocumentMapping) AddSubDocumentMapping(name string, sdm *DocumentMapping) {
	dm.SubMappings[name] = sdm
}

// IndexMapping is the top-level mapping configuration for an index.
type IndexMapping struct {
	TypeMapping     map[string]*DocumentMapping `json:"type_mapping,omitempty"`
	DefaultMapping  *DocumentMapping            `json:"default_mapping"`
	DefaultAnalyzer string                      `json:"default_analyzer"`
	TypeField       string                      `json:"type_field"`
}

// NewIndexMapping creates a new IndexMapping with default settings.
func NewIndexMapping() *IndexMapping {
	return &IndexMapping{
		TypeMapping:     make(map[string]*DocumentMapping),
		DefaultMapping:  NewDocumentMapping(),
		DefaultAnalyzer: "standard",
		TypeField:       "_type",
	}
}

// AddDocumentMapping adds a document mapping for a named type.
func (im *IndexMapping) AddDocumentMapping(name string, dm *DocumentMapping) {
	im.TypeMapping[name] = dm
}

// Validate checks the index mapping for errors.
func (im *IndexMapping) Validate() error {
	if im.DefaultMapping == nil {
		return fmt.Errorf("default mapping is required")
	}
	return nil
}

// maxDepth is the maximum nesting depth for recursive document mapping.
const maxDepth = 32

// MapDocument converts raw data into a Document with fields according to the mapping.
func (im *IndexMapping) MapDocument(doc *document.Document, data interface{}) error {
	// Determine which document mapping to use
	docMapping := im.DefaultMapping

	// Convert data to a map
	dataMap, err := toMap(data)
	if err != nil {
		return fmt.Errorf("cannot map document: %w", err)
	}

	// Check if there's a type field that selects a specific mapping
	if im.TypeField != "" {
		if typeVal, ok := dataMap[im.TypeField]; ok {
			if typeName, ok := typeVal.(string); ok {
				if tm, ok := im.TypeMapping[typeName]; ok {
					docMapping = tm
				}
			}
		}
	}

	return im.mapFields(doc, docMapping, dataMap, "", 0)
}

func (im *IndexMapping) mapFields(doc *document.Document, dm *DocumentMapping, data map[string]interface{}, prefix string, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("mapping exceeds maximum nesting depth of %d", maxDepth)
	}
	if !dm.Enabled {
		return nil
	}

	for key, value := range data {
		fieldName := key
		if prefix != "" {
			fieldName = prefix + "." + key
		}

		// Check for explicit field mapping
		if fm, ok := dm.Fields[key]; ok {
			field := im.createField(fieldName, fm, value)
			if field != nil {
				doc.AddField(field)
			}
			continue
		}

		// Check for sub-document mapping
		if subMap, ok := value.(map[string]interface{}); ok {
			if sdm, ok := dm.SubMappings[key]; ok {
				if err := im.mapFields(doc, sdm, subMap, fieldName, depth+1); err != nil {
					return err
				}
				continue
			}
		}

		// Dynamic mapping: auto-detect field type
		if dm.Dynamic {
			field := im.dynamicMap(fieldName, value)
			if field != nil {
				doc.AddField(field)
			}
		}
	}

	return nil
}

func (im *IndexMapping) createField(name string, fm *FieldMapping, value interface{}) *document.Field {
	analyzer := fm.Analyzer
	if analyzer == "" {
		analyzer = im.DefaultAnalyzer
	}

	return &document.Field{
		Name:               name,
		Type:               fm.Type,
		Value:              value,
		Analyzer:           analyzer,
		Store:              fm.Store,
		Index:              fm.Index,
		IncludeTermVectors: fm.IncludeTermVectors,
		Dims:               fm.Dims,
		Language:           fm.Language,
	}
}

func (im *IndexMapping) dynamicMap(name string, value interface{}) *document.Field {
	switch v := value.(type) {
	case string:
		// Try to parse as datetime
		for _, layout := range []string{time.RFC3339, time.RFC3339Nano} {
			if _, err := time.Parse(layout, v); err == nil {
				return &document.Field{
					Name:     name,
					Type:     document.FieldTypeDateTime,
					Value:    v,
					Analyzer: im.DefaultAnalyzer,
					Store:    true,
					Index:    true,
				}
			}
		}
		return &document.Field{
			Name:               name,
			Type:               document.FieldTypeText,
			Value:              v,
			Analyzer:           im.DefaultAnalyzer,
			Store:              true,
			Index:              true,
			IncludeTermVectors: true,
		}
	case float64, float32, int, int64, int32:
		return &document.Field{
			Name:  name,
			Type:  document.FieldTypeNumeric,
			Value: value,
			Store: true,
			Index: true,
		}
	case bool:
		return &document.Field{
			Name:  name,
			Type:  document.FieldTypeBoolean,
			Value: v,
			Store: true,
			Index: true,
		}
	case []float32, []float64:
		return &document.Field{
			Name:  name,
			Type:  document.FieldTypeVector,
			Value: value,
			Store: true,
			Index: true,
		}
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return &document.Field{
				Name:  name,
				Type:  document.FieldTypeNumeric,
				Value: f,
				Store: true,
				Index: true,
			}
		}
	}
	return nil
}

// toMap converts various Go types to a map[string]interface{}.
func toMap(data interface{}) (map[string]interface{}, error) {
	switch v := data.(type) {
	case map[string]interface{}:
		return v, nil
	case []byte:
		var m map[string]interface{}
		if err := json.Unmarshal(v, &m); err != nil {
			return nil, err
		}
		return m, nil
	case string:
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, err
		}
		return m, nil
	default:
		// Use reflection for structs
		rv := reflect.ValueOf(data)
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Struct {
			return structToMap(rv)
		}
		// Try JSON round-trip as last resort
		b, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("cannot convert %T to map: %w", data, err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		return m, nil
	}
}

func structToMap(rv reflect.Value) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if !field.IsExported() {
			continue
		}
		name := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			parts := strings.SplitN(tag, ",", 2)
			if parts[0] == "-" {
				continue
			}
			if parts[0] != "" {
				name = parts[0]
			}
		}
		result[name] = rv.Field(i).Interface()
	}
	return result, nil
}
