package capture

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

// Use the existing mockPropertyFactory from capturer_test.go

func TestCapturedStructFieldOrdering(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	type Person struct {
		ID       int
		Username string
		Email    string
		Age      int
	}
	
	person := Person{ID: 123, Username: "alice", Email: "alice@example.com", Age: 30}
	prop, _ := d.TryCapture(person, factory)
	
	// Should return *CapturedStruct, not map
	cs, ok := prop.Value.(*CapturedStruct)
	if !ok {
		t.Fatalf("Expected *CapturedStruct, got %T", prop.Value)
	}
	
	// Check field order matches struct definition
	expectedOrder := []string{"ID", "Username", "Email", "Age"}
	if len(cs.Fields) != len(expectedOrder) {
		t.Fatalf("Expected %d fields, got %d", len(expectedOrder), len(cs.Fields))
	}
	
	for i, field := range cs.Fields {
		if field.Name != expectedOrder[i] {
			t.Errorf("Field %d: expected %s, got %s", i, expectedOrder[i], field.Name)
		}
	}
	
	// Check values
	expectedValues := []any{123, "alice", "alice@example.com", 30}
	for i, field := range cs.Fields {
		if !reflect.DeepEqual(field.Value, expectedValues[i]) {
			t.Errorf("Field %s: expected %v, got %v", field.Name, expectedValues[i], field.Value)
		}
	}
}

func TestByteSliceToString(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	tests := []struct {
		name     string
		input    []byte
		expected any
	}{
		{"JSON", []byte(`{"status": "ok"}`), `{"status": "ok"}`},
		{"Text", []byte("hello world"), "hello world"},
		{"Binary with nulls", []byte{0x48, 0x65, 0x00, 0x6C}, []byte{0x48, 0x65, 0x00, 0x6C}},
		{"Empty", []byte{}, []byte{}},
		{"Long text", bytes.Repeat([]byte("a"), 2000), strings.Repeat("a", 1000) + "..."},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop, _ := d.TryCapture(tt.input, factory)
			if !reflect.DeepEqual(prop.Value, tt.expected) {
				t.Errorf("Expected %v (%T), got %v (%T)", 
					tt.expected, tt.expected, prop.Value, prop.Value)
			}
		})
	}
}

func TestByteSliceInStruct(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	type Response struct {
		StatusCode int
		Body       []byte
		Error      error
	}
	
	resp := Response{
		StatusCode: 200,
		Body:       []byte(`{"status": "ok"}`),
		Error:      nil,
	}
	
	prop, _ := d.TryCapture(resp, factory)
	cs, ok := prop.Value.(*CapturedStruct)
	if !ok {
		t.Fatalf("Expected *CapturedStruct, got %T", prop.Value)
	}
	
	// Find Body field
	var bodyValue any
	for _, field := range cs.Fields {
		if field.Name == "Body" {
			bodyValue = field.Value
			break
		}
	}
	
	// Body should be converted to string
	if bodyStr, ok := bodyValue.(string); !ok || bodyStr != `{"status": "ok"}` {
		t.Errorf("Expected Body to be string `{\"status\": \"ok\"}`, got %v (%T)", bodyValue, bodyValue)
	}
}

func TestNilRepresentation(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	// Test various nil scenarios
	tests := []struct {
		name  string
		setup func() any
	}{
		{"nil value", func() any { return nil }},
		{"nil pointer", func() any { var p *int; return p }},
		{"nil interface", func() any { var i interface{}; return i }},
		{"nil slice", func() any { var s []int; return s }},
		{"nil map", func() any { var m map[string]int; return m }},
		{"nil error", func() any { var e error; return e }},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop, _ := d.TryCapture(tt.setup(), factory)
			if _, ok := prop.Value.(Null); !ok {
				t.Errorf("Expected Null{}, got %v (%T)", prop.Value, prop.Value)
			}
		})
	}
}

func TestNilInStruct(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	type Container struct {
		Value  string
		Error  error
		Next   *Container
	}
	
	c := Container{
		Value: "test",
		Error: nil,
		Next:  nil,
	}
	
	prop, _ := d.TryCapture(c, factory)
	cs, ok := prop.Value.(*CapturedStruct)
	if !ok {
		t.Fatalf("Expected *CapturedStruct, got %T", prop.Value)
	}
	
	// Check that nil fields are represented as Null{}
	for _, field := range cs.Fields {
		if field.Name == "Error" || field.Name == "Next" {
			if _, ok := field.Value.(Null); !ok {
				t.Errorf("Field %s: expected Null{}, got %v (%T)", 
					field.Name, field.Value, field.Value)
			}
		}
	}
}

func TestAnonymousStructInSlice(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	items := []any{
		"string",
		123,
		struct{ Name string }{"test"},
		map[string]int{"a": 1},
	}
	
	prop, _ := d.TryCapture(items, factory)
	result, ok := prop.Value.([]any)
	if !ok {
		t.Fatalf("Expected []any, got %T", prop.Value)
	}
	
	if len(result) != 4 {
		t.Fatalf("Expected 4 items, got %d", len(result))
	}
	
	// Check anonymous struct is captured as CapturedStruct
	cs, ok := result[2].(*CapturedStruct)
	if !ok {
		t.Fatalf("Expected *CapturedStruct for anonymous struct, got %T", result[2])
	}
	
	if len(cs.Fields) != 1 || cs.Fields[0].Name != "Name" || cs.Fields[0].Value != "test" {
		t.Errorf("Anonymous struct not captured correctly: %+v", cs)
	}
}

func TestCapturedStructString(t *testing.T) {
	cs := &CapturedStruct{
		TypeName: "Person",
		Fields: []CapturedField{
			{Name: "ID", Value: 123},
			{Name: "Name", Value: "alice"},
			{Name: "Active", Value: true},
			{Name: "Error", Value: Null{}},
		},
	}
	
	expected := "{ID:123 Name:alice Active:true Error:nil}"
	result := cs.String()
	
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
	
	// Test nested CapturedStruct
	nested := &CapturedStruct{
		TypeName: "Address",
		Fields: []CapturedField{
			{Name: "Street", Value: "123 Main St"},
			{Name: "City", Value: "Anytown"},
		},
	}
	
	cs2 := &CapturedStruct{
		TypeName: "Person",
		Fields: []CapturedField{
			{Name: "Name", Value: "bob"},
			{Name: "Address", Value: nested},
		},
	}
	
	expected2 := "{Name:bob Address:{Street:123 Main St City:Anytown}}"
	result2 := cs2.String()
	
	if result2 != expected2 {
		t.Errorf("Expected %s, got %s", expected2, result2)
	}
	
	// Test nil CapturedStruct
	var nilCS *CapturedStruct
	if nilCS.String() != "nil" {
		t.Errorf("Expected nil CapturedStruct to return \"nil\", got %s", nilCS.String())
	}
}

func TestCachedCapturerByteSlice(t *testing.T) {
	d := NewCachedCapturer()
	factory := &mockPropertyFactory{}
	
	tests := []struct {
		name     string
		input    []byte
		expected any
	}{
		{"JSON", []byte(`{"test": true}`), `{"test": true}`},
		{"Text", []byte("hello cached"), "hello cached"},
		{"Binary", []byte{0xFF, 0xFE, 0x00}, []byte{0xFF, 0xFE, 0x00}},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop, _ := d.TryCapture(tt.input, factory)
			if !reflect.DeepEqual(prop.Value, tt.expected) {
				t.Errorf("Expected %v (%T), got %v (%T)", 
					tt.expected, tt.expected, prop.Value, prop.Value)
			}
		})
	}
}

func TestCachedCapturerNil(t *testing.T) {
	d := NewCachedCapturer()
	factory := &mockPropertyFactory{}
	
	// Test nil value
	prop, _ := d.TryCapture(nil, factory)
	if _, ok := prop.Value.(Null); !ok {
		t.Errorf("Expected Null{}, got %v (%T)", prop.Value, prop.Value)
	}
	
	// Test nil pointer
	var p *int
	prop2, _ := d.TryCapture(p, factory)
	if _, ok := prop2.Value.(Null); !ok {
		t.Errorf("Expected Null{} for nil pointer, got %v (%T)", prop2.Value, prop2.Value)
	}
}

func TestConvertBytesToStringIfPrintable(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		maxLength int
		expected  interface{}
	}{
		{"Valid UTF-8", []byte("hello world"), 100, "hello world"},
		{"JSON", []byte(`{"key": "value"}`), 100, `{"key": "value"}`},
		{"With null byte", []byte("hello\x00world"), 100, []byte("hello\x00world")},
		{"Invalid UTF-8", []byte{0xFF, 0xFE}, 100, []byte{0xFF, 0xFE}},
		{"Long string", []byte("abcdefghij"), 5, "abcde..."},
		{"Empty", []byte{}, 100, []byte{}},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertBytesToStringIfPrintable(tt.input, tt.maxLength)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v (%T), got %v (%T)",
					tt.expected, tt.expected, result, result)
			}
		})
	}
}

func TestStructWithTags(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	type Tagged struct {
		PublicField  string
		privateField string        // Should be skipped
		Tagged       string `log:"custom_name"`
		Ignored      string `log:"-"`  // Should be skipped
	}
	
	tagged := Tagged{
		PublicField:  "public",
		privateField: "private",
		Tagged:       "tagged value",
		Ignored:      "ignored value",
	}
	
	prop, _ := d.TryCapture(tagged, factory)
	cs, ok := prop.Value.(*CapturedStruct)
	if !ok {
		t.Fatalf("Expected *CapturedStruct, got %T", prop.Value)
	}
	
	// Should only have 2 fields: PublicField and custom_name
	if len(cs.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d: %+v", len(cs.Fields), cs.Fields)
	}
	
	// Check field names
	expectedFields := map[string]string{
		"PublicField": "public",
		"custom_name": "tagged value",
	}
	
	for _, field := range cs.Fields {
		expected, ok := expectedFields[field.Name]
		if !ok {
			t.Errorf("Unexpected field: %s", field.Name)
		} else if field.Value != expected {
			t.Errorf("Field %s: expected %v, got %v", field.Name, expected, field.Value)
		}
	}
}

// Benchmark tests
func BenchmarkCaptureStruct(b *testing.B) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	type Person struct {
		ID       int
		Username string
		Email    string
		Age      int
		Active   bool
	}
	
	person := Person{
		ID:       123,
		Username: "alice",
		Email:    "alice@example.com",
		Age:      30,
		Active:   true,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.TryCapture(person, factory)
	}
}

func BenchmarkCaptureByteSlice(b *testing.B) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	data := []byte(`{"status": "ok", "message": "success", "data": {"id": 123}}`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.TryCapture(data, factory)
	}
}