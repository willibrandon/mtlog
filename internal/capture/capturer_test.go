package capture

import (
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// mockPropertyFactory for testing
type mockPropertyFactory struct{}

func (m *mockPropertyFactory) CreateProperty(name string, value interface{}) *core.LogEventProperty {
	return &core.LogEventProperty{
		Name:  name,
		Value: value,
	}
}

func TestCaptureBasicTypes(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{"nil", nil, nil},
		{"bool", true, true},
		{"int", 42, 42},
		{"float", 3.14, 3.14},
		{"string", "hello", "hello"},
		{"long string", string(make([]byte, 2000)), string(make([]byte, 1000)) + "..."},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prop, ok := d.TryCapture(tt.input, factory)
			if !ok {
				t.Fatal("TryCapture failed")
			}
			
			if prop.Value != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, prop.Value)
			}
		})
	}
}

func TestCaptureSlice(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	// Test normal slice
	slice := []int{1, 2, 3, 4, 5}
	prop, _ := d.TryCapture(slice, factory)
	
	result, ok := prop.Value.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", prop.Value)
	}
	
	if len(result) != 5 {
		t.Errorf("Expected length 5, got %d", len(result))
	}
	
	// Test large slice (should be truncated)
	largeSlice := make([]int, 200)
	prop2, _ := d.TryCapture(largeSlice, factory)
	
	result2, ok := prop2.Value.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", prop2.Value)
	}
	
	// Should have 100 items plus truncation indicator
	if len(result2) != 101 {
		t.Errorf("Expected length 101 (100 + indicator), got %d", len(result2))
	}
	
	// Check truncation indicator
	if indicator, ok := result2[100].(string); !ok || indicator != "... (100 more)" {
		t.Errorf("Expected truncation indicator, got %v", result2[100])
	}
}

func TestCaptureMap(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	// Test normal map
	m := map[string]int{
		"a": 1,
		"b": 2,
		"c": 3,
	}
	
	prop, _ := d.TryCapture(m, factory)
	result, ok := prop.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", prop.Value)
	}
	
	if len(result) != 3 {
		t.Errorf("Expected length 3, got %d", len(result))
	}
	
	if result["a"] != 1 || result["b"] != 2 || result["c"] != 3 {
		t.Error("Map values don't match")
	}
}

func TestCaptureStruct(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	type Address struct {
		Street string
		City   string
	}
	
	type Person struct {
		Name       string
		Age        int
		Email      string    `log:"email"`
		Password   string    `log:"-"`
		unexported string
		Address    Address
		Tags       []string
	}
	
	person := Person{
		Name:       "Alice",
		Age:        30,
		Email:      "alice@example.com",
		Password:   "secret123",
		unexported: "hidden",
		Address: Address{
			Street: "123 Main St",
			City:   "Anytown",
		},
		Tags: []string{"developer", "team-lead"},
	}
	
	prop, _ := d.TryCapture(person, factory)
	result, ok := prop.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", prop.Value)
	}
	
	// Check fields
	if result["Name"] != "Alice" {
		t.Errorf("Expected Name=Alice, got %v", result["Name"])
	}
	
	if result["Age"] != 30 {
		t.Errorf("Expected Age=30, got %v", result["Age"])
	}
	
	// Check tag rename
	if result["email"] != "alice@example.com" {
		t.Errorf("Expected email field, got %v", result["email"])
	}
	
	// Password should be excluded
	if _, exists := result["Password"]; exists {
		t.Error("Password field should be excluded")
	}
	
	// Unexported field should be excluded
	if _, exists := result["unexported"]; exists {
		t.Error("Unexported field should be excluded")
	}
	
	// Check nested struct
	address, ok := result["Address"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected Address to be map[string]interface{}, got %T", result["Address"])
	}
	
	if address["Street"] != "123 Main St" {
		t.Errorf("Expected Street='123 Main St', got %v", address["Street"])
	}
	
	// Check slice field
	tags, ok := result["Tags"].([]interface{})
	if !ok {
		t.Fatalf("Expected Tags to be []interface{}, got %T", result["Tags"])
	}
	
	if len(tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(tags))
	}
}

func TestCaptureDepthLimit(t *testing.T) {
	d := NewCapturer(3, 1000, 100) // Max depth of 3 (to reach the nested structs)
	factory := &mockPropertyFactory{}
	
	type Nested struct {
		Level int
		Next  *Nested
	}
	
	// Create nested structure with depth > 2
	nested := &Nested{
		Level: 1,
		Next: &Nested{
			Level: 2,
			Next: &Nested{
				Level: 3,
				Next: &Nested{
					Level: 4,
				},
			},
		},
	}
	
	prop, _ := d.TryCapture(nested, factory)
	result := prop.Value.(map[string]interface{})
	
	// Check first level
	if result["Level"] != 1 {
		t.Errorf("Expected Level=1, got %v", result["Level"])
	}
	
	// Check second level
	level2, ok := result["Next"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected Next to be map[string]interface{}, got %T", result["Next"])
	}
	level2Val, ok := level2["Level"].(int)
	if !ok || level2Val != 2 {
		t.Errorf("Expected Level=2 (int), got %T: %v", level2["Level"], level2["Level"])
	}
	
	// Third level should still be a map
	level3, ok := level2["Next"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected level 3 to be map[string]interface{}, got %T", level2["Next"])
	}
	
	// Fourth level should be type string due to depth limit
	level4 := level3["Next"]
	if _, ok := level4.(string); !ok {
		t.Errorf("Expected depth limit to return type string, got %T: %v", level4, level4)
	}
}

func TestCaptureTime(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	now := time.Date(2025, 1, 15, 10, 30, 45, 0, time.UTC)
	prop, _ := d.TryCapture(now, factory)
	
	// Time should be formatted as RFC3339
	expected := "2025-01-15T10:30:45Z"
	if prop.Value != expected {
		t.Errorf("Expected %s, got %v", expected, prop.Value)
	}
	
	// Duration should remain as-is (registered as scalar)
	duration := 5 * time.Minute
	prop2, _ := d.TryCapture(duration, factory)
	
	if prop2.Value != duration {
		t.Errorf("Expected duration to remain as-is, got %v", prop2.Value)
	}
}

func TestCapturePointers(t *testing.T) {
	d := NewDefaultCapturer()
	factory := &mockPropertyFactory{}
	
	// Test nil pointer
	var nilPtr *int
	prop, _ := d.TryCapture(nilPtr, factory)
	if prop.Value != nil {
		t.Errorf("Expected nil, got %v", prop.Value)
	}
	
	// Test non-nil pointer
	val := 42
	ptr := &val
	prop2, _ := d.TryCapture(ptr, factory)
	if prop2.Value != 42 {
		t.Errorf("Expected 42, got %v", prop2.Value)
	}
}