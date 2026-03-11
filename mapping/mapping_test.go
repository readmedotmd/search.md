package mapping

import (
	"reflect"
	"testing"
	"time"

	"github.com/readmedotmd/search.md/document"
)

// --- NewIndexMapping defaults ---

func TestNewIndexMapping_Defaults(t *testing.T) {
	im := NewIndexMapping()
	if im.DefaultAnalyzer != "standard" {
		t.Errorf("expected default analyzer 'standard', got %q", im.DefaultAnalyzer)
	}
	if im.TypeField != "_type" {
		t.Errorf("expected type field '_type', got %q", im.TypeField)
	}
	if im.DefaultMapping == nil {
		t.Fatal("expected non-nil default mapping")
	}
	if !im.DefaultMapping.Enabled {
		t.Error("expected default mapping to be enabled")
	}
	if !im.DefaultMapping.Dynamic {
		t.Error("expected default mapping to be dynamic")
	}
	if im.TypeMapping == nil {
		t.Error("expected non-nil type mapping map")
	}
}

// --- Validate ---

func TestValidate_Valid(t *testing.T) {
	im := NewIndexMapping()
	if err := im.Validate(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidate_NilDefaultMapping(t *testing.T) {
	im := NewIndexMapping()
	im.DefaultMapping = nil
	if err := im.Validate(); err == nil {
		t.Error("expected error for nil default mapping")
	}
}

// --- MapDocument with map input ---

func TestMapDocument_MapInput(t *testing.T) {
	im := NewIndexMapping()
	doc := document.NewDocument("1")
	data := map[string]interface{}{
		"title": "hello world",
		"count": float64(42),
	}
	if err := im.MapDocument(doc, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(doc.Fields))
	}
	titleField := doc.FieldNamed("title")
	if titleField == nil {
		t.Fatal("expected title field")
	}
	if titleField.Type != document.FieldTypeText {
		t.Errorf("expected text type, got %d", titleField.Type)
	}
	countField := doc.FieldNamed("count")
	if countField == nil {
		t.Fatal("expected count field")
	}
	if countField.Type != document.FieldTypeNumeric {
		t.Errorf("expected numeric type, got %d", countField.Type)
	}
}

// --- MapDocument with struct input ---

func TestMapDocument_StructInput(t *testing.T) {
	type Article struct {
		Title string `json:"title"`
		Views int    `json:"views"`
	}

	im := NewIndexMapping()
	doc := document.NewDocument("2")
	data := Article{Title: "test", Views: 100}
	if err := im.MapDocument(doc, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.FieldNamed("title") == nil {
		t.Error("expected title field from struct")
	}
	if doc.FieldNamed("views") == nil {
		t.Error("expected views field from struct")
	}
}

// --- MapDocument with JSON bytes input ---

func TestMapDocument_JSONBytesInput(t *testing.T) {
	im := NewIndexMapping()
	doc := document.NewDocument("3")
	data := []byte(`{"name":"alice","active":true}`)
	if err := im.MapDocument(doc, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nameField := doc.FieldNamed("name")
	if nameField == nil {
		t.Fatal("expected name field")
	}
	activeField := doc.FieldNamed("active")
	if activeField == nil {
		t.Fatal("expected active field")
	}
	if activeField.Type != document.FieldTypeBoolean {
		t.Errorf("expected boolean type, got %d", activeField.Type)
	}
}

// --- Dynamic mapping auto-detect ---

func TestDynamicMapping_String(t *testing.T) {
	im := NewIndexMapping()
	doc := document.NewDocument("d1")
	data := map[string]interface{}{"text": "hello"}
	im.MapDocument(doc, data)

	f := doc.FieldNamed("text")
	if f == nil {
		t.Fatal("expected text field")
	}
	if f.Type != document.FieldTypeText {
		t.Errorf("expected text type, got %d", f.Type)
	}
	if !f.IncludeTermVectors {
		t.Error("dynamic text fields should include term vectors")
	}
}

func TestDynamicMapping_Number(t *testing.T) {
	im := NewIndexMapping()
	for _, val := range []interface{}{float64(1.5), float32(1.5), int(3), int64(3), int32(3)} {
		doc := document.NewDocument("d")
		data := map[string]interface{}{"num": val}
		im.MapDocument(doc, data)

		f := doc.FieldNamed("num")
		if f == nil {
			t.Fatalf("expected num field for type %T", val)
		}
		if f.Type != document.FieldTypeNumeric {
			t.Errorf("expected numeric type for %T, got %d", val, f.Type)
		}
	}
}

func TestDynamicMapping_Bool(t *testing.T) {
	im := NewIndexMapping()
	doc := document.NewDocument("d")
	data := map[string]interface{}{"flag": true}
	im.MapDocument(doc, data)

	f := doc.FieldNamed("flag")
	if f == nil {
		t.Fatal("expected flag field")
	}
	if f.Type != document.FieldTypeBoolean {
		t.Errorf("expected boolean type, got %d", f.Type)
	}
}

func TestDynamicMapping_DateTime(t *testing.T) {
	im := NewIndexMapping()
	doc := document.NewDocument("d")
	data := map[string]interface{}{"created": "2024-01-15T10:30:00Z"}
	im.MapDocument(doc, data)

	f := doc.FieldNamed("created")
	if f == nil {
		t.Fatal("expected created field")
	}
	if f.Type != document.FieldTypeDateTime {
		t.Errorf("expected datetime type, got %d", f.Type)
	}
}

func TestDynamicMapping_Vector(t *testing.T) {
	im := NewIndexMapping()

	doc := document.NewDocument("d1")
	data := map[string]interface{}{"vec": []float32{1.0, 2.0}}
	im.MapDocument(doc, data)
	f := doc.FieldNamed("vec")
	if f == nil || f.Type != document.FieldTypeVector {
		t.Errorf("expected vector type for []float32")
	}

	doc2 := document.NewDocument("d2")
	data2 := map[string]interface{}{"vec": []float64{1.0, 2.0}}
	im.MapDocument(doc2, data2)
	f2 := doc2.FieldNamed("vec")
	if f2 == nil || f2.Type != document.FieldTypeVector {
		t.Errorf("expected vector type for []float64")
	}
}

// --- Static mapping ---

func TestStaticMapping_OnlyMappedFields(t *testing.T) {
	im := NewIndexMapping()
	dm := NewDocumentStaticMapping()
	dm.AddFieldMapping("title", NewTextFieldMapping())
	im.DefaultMapping = dm

	doc := document.NewDocument("s1")
	data := map[string]interface{}{
		"title":       "hello",
		"description": "world",
		"count":       float64(5),
	}
	im.MapDocument(doc, data)

	if doc.FieldNamed("title") == nil {
		t.Error("expected title field in static mapping")
	}
	if doc.FieldNamed("description") != nil {
		t.Error("description should not be indexed in static mapping")
	}
	if doc.FieldNamed("count") != nil {
		t.Error("count should not be indexed in static mapping")
	}
}

// --- Sub-document mapping ---

func TestSubDocumentMapping(t *testing.T) {
	im := NewIndexMapping()
	dm := NewDocumentMapping()
	subDm := NewDocumentMapping()
	subDm.AddFieldMapping("street", NewTextFieldMapping())
	dm.AddSubDocumentMapping("address", subDm)
	im.DefaultMapping = dm

	doc := document.NewDocument("n1")
	data := map[string]interface{}{
		"name": "Bob",
		"address": map[string]interface{}{
			"street": "123 Main St",
			"zip":    "90210",
		},
	}
	im.MapDocument(doc, data)

	streetField := doc.FieldNamed("address.street")
	if streetField == nil {
		t.Fatal("expected address.street field")
	}
	if streetField.TextValue() != "123 Main St" {
		t.Errorf("expected '123 Main St', got %q", streetField.TextValue())
	}

	// "zip" should be dynamically mapped since subDm is dynamic
	zipField := doc.FieldNamed("address.zip")
	if zipField == nil {
		t.Fatal("expected address.zip to be dynamically mapped")
	}
}

// --- Field mapping constructors ---

func TestNewTextFieldMapping(t *testing.T) {
	fm := NewTextFieldMapping()
	if fm.Type != document.FieldTypeText {
		t.Errorf("expected text type")
	}
	if fm.Analyzer != "standard" {
		t.Errorf("expected standard analyzer, got %q", fm.Analyzer)
	}
	if !fm.Store || !fm.Index || !fm.IncludeTermVectors {
		t.Error("expected store, index, and term vectors to be true")
	}
}

func TestNewKeywordFieldMapping(t *testing.T) {
	fm := NewKeywordFieldMapping()
	if fm.Type != document.FieldTypeKeyword {
		t.Errorf("expected keyword type")
	}
	if fm.Analyzer != "keyword" {
		t.Errorf("expected keyword analyzer, got %q", fm.Analyzer)
	}
	if !fm.Store || !fm.Index {
		t.Error("expected store and index to be true")
	}
	if fm.IncludeTermVectors {
		t.Error("keyword should not include term vectors by default")
	}
}

func TestNewNumericFieldMapping(t *testing.T) {
	fm := NewNumericFieldMapping()
	if fm.Type != document.FieldTypeNumeric {
		t.Errorf("expected numeric type")
	}
	if !fm.Store || !fm.Index {
		t.Error("expected store and index to be true")
	}
}

func TestNewBooleanFieldMapping(t *testing.T) {
	fm := NewBooleanFieldMapping()
	if fm.Type != document.FieldTypeBoolean {
		t.Errorf("expected boolean type")
	}
	if !fm.Store || !fm.Index {
		t.Error("expected store and index to be true")
	}
}

func TestNewDateTimeFieldMapping(t *testing.T) {
	fm := NewDateTimeFieldMapping()
	if fm.Type != document.FieldTypeDateTime {
		t.Errorf("expected datetime type")
	}
	if !fm.Store || !fm.Index {
		t.Error("expected store and index to be true")
	}
}

func TestNewVectorFieldMapping(t *testing.T) {
	fm := NewVectorFieldMapping(128)
	if fm.Type != document.FieldTypeVector {
		t.Errorf("expected vector type")
	}
	if fm.Dims != 128 {
		t.Errorf("expected dims=128, got %d", fm.Dims)
	}
	if !fm.Store || !fm.Index {
		t.Error("expected store and index to be true")
	}
}

func TestNewCodeFieldMapping(t *testing.T) {
	fm := NewCodeFieldMapping()
	if fm.Type != document.FieldTypeCode {
		t.Errorf("expected code type")
	}
	if fm.Analyzer != "code" {
		t.Errorf("expected code analyzer, got %q", fm.Analyzer)
	}
	if !fm.Store || !fm.Index || !fm.IncludeTermVectors {
		t.Error("expected store, index, and term vectors to be true")
	}
}

// --- toMap ---

func TestToMap_MapInput(t *testing.T) {
	input := map[string]interface{}{"a": 1}
	result, err := toMap(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["a"] != 1 {
		t.Errorf("expected a=1, got %v", result["a"])
	}
}

func TestToMap_JSONBytes(t *testing.T) {
	input := []byte(`{"key":"value"}`)
	result, err := toMap(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
}

func TestToMap_JSONString(t *testing.T) {
	input := `{"key":"value"}`
	result, err := toMap(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
}

func TestToMap_Struct(t *testing.T) {
	type S struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	result, err := toMap(S{Name: "Alice", Age: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", result["name"])
	}
	if result["age"] != 30 {
		t.Errorf("expected age=30, got %v", result["age"])
	}
}

func TestToMap_StructPointer(t *testing.T) {
	type S struct {
		Val string `json:"val"`
	}
	result, err := toMap(&S{Val: "ptr"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["val"] != "ptr" {
		t.Errorf("expected val=ptr, got %v", result["val"])
	}
}

func TestToMap_InvalidJSON(t *testing.T) {
	_, err := toMap([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON bytes")
	}
	_, err = toMap("not json")
	if err == nil {
		t.Error("expected error for invalid JSON string")
	}
}

// --- structToMap ---

func TestStructToMap_JSONTags(t *testing.T) {
	type S struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Ignored   string `json:"-"`
		NoTag     string
	}
	s := S{FirstName: "John", LastName: "Doe", Ignored: "skip", NoTag: "keep"}
	result, err := structToMap(reflectVal(s))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["first_name"] != "John" {
		t.Errorf("expected first_name=John, got %v", result["first_name"])
	}
	if result["last_name"] != "Doe" {
		t.Errorf("expected last_name=Doe, got %v", result["last_name"])
	}
	if _, ok := result["Ignored"]; ok {
		t.Error("field with json:\"-\" should be excluded")
	}
	if _, ok := result["-"]; ok {
		t.Error("field with json:\"-\" should be excluded")
	}
	if result["NoTag"] != "keep" {
		t.Errorf("field without tag should use field name, got %v", result["NoTag"])
	}
}

func TestStructToMap_UnexportedFields(t *testing.T) {
	type S struct {
		Public  string
		private string //nolint:unused
	}
	s := S{Public: "visible"}
	result, err := structToMap(reflectVal(s))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["Public"] != "visible" {
		t.Errorf("expected Public=visible, got %v", result["Public"])
	}
	if _, ok := result["private"]; ok {
		t.Error("unexported fields should be excluded")
	}
}

// --- Type mapping selection ---

func TestMapDocument_TypeMapping(t *testing.T) {
	im := NewIndexMapping()

	articleMapping := NewDocumentStaticMapping()
	articleMapping.AddFieldMapping("title", NewTextFieldMapping())
	im.AddDocumentMapping("article", articleMapping)

	doc := document.NewDocument("t1")
	data := map[string]interface{}{
		"_type":  "article",
		"title":  "Hello",
		"author": "Bob",
	}
	im.MapDocument(doc, data)

	if doc.FieldNamed("title") == nil {
		t.Error("expected title field via type mapping")
	}
	// author should not be mapped since article mapping is static
	if doc.FieldNamed("author") != nil {
		t.Error("author should not be indexed in static article mapping")
	}
}

// --- Disabled mapping ---

func TestMapDocument_DisabledMapping(t *testing.T) {
	im := NewIndexMapping()
	im.DefaultMapping.Enabled = false

	doc := document.NewDocument("dis1")
	data := map[string]interface{}{"title": "ignored"}
	im.MapDocument(doc, data)

	if len(doc.Fields) != 0 {
		t.Errorf("expected 0 fields for disabled mapping, got %d", len(doc.Fields))
	}
}

// --- MapDocument with explicit field mapping uses correct analyzer ---

func TestMapDocument_ExplicitFieldUsesAnalyzer(t *testing.T) {
	im := NewIndexMapping()
	dm := NewDocumentMapping()
	fm := NewKeywordFieldMapping()
	dm.AddFieldMapping("tag", fm)
	im.DefaultMapping = dm

	doc := document.NewDocument("a1")
	data := map[string]interface{}{"tag": "golang"}
	im.MapDocument(doc, data)

	f := doc.FieldNamed("tag")
	if f == nil {
		t.Fatal("expected tag field")
	}
	if f.Analyzer != "keyword" {
		t.Errorf("expected keyword analyzer, got %q", f.Analyzer)
	}
}

// --- MapDocument datetime RFC3339Nano ---

func TestDynamicMapping_DateTimeNano(t *testing.T) {
	im := NewIndexMapping()
	doc := document.NewDocument("d")
	ts := time.Now().Format(time.RFC3339Nano)
	data := map[string]interface{}{"ts": ts}
	im.MapDocument(doc, data)

	f := doc.FieldNamed("ts")
	if f == nil {
		t.Fatal("expected ts field")
	}
	if f.Type != document.FieldTypeDateTime {
		t.Errorf("expected datetime type for RFC3339Nano, got %d", f.Type)
	}
}

// helper
func reflectVal(v interface{}) reflect.Value {
	return reflect.ValueOf(v)
}
