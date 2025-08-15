package capture_test

import (
	"encoding/json"
	"testing"

	"github.com/willibrandon/mtlog/internal/capture"
)

func TestNullJSONMarshaling(t *testing.T) {
	// Test that Null{} marshals to JSON null
	null := capture.Null{}
	
	jsonBytes, err := json.Marshal(null)
	if err != nil {
		t.Fatalf("Failed to marshal Null{}: %v", err)
	}
	
	if string(jsonBytes) != "null" {
		t.Errorf("Expected Null{} to marshal to 'null', got: %s", string(jsonBytes))
	}
	
	// Test String() method
	if null.String() != "nil" {
		t.Errorf("Expected Null{}.String() to return 'nil', got: %s", null.String())
	}
}

func TestCapturedStructJSONMarshaling(t *testing.T) {
	// Test that CapturedStruct marshals to proper JSON object
	cs := &capture.CapturedStruct{
		TypeName: "User",
		Fields: []capture.CapturedField{
			{Name: "ID", Value: 123},
			{Name: "Username", Value: "alice"},
			{Name: "Active", Value: true},
			{Name: "Score", Value: 98.5},
			{Name: "Tags", Value: []any{"admin", "user"}},
			{Name: "Extra", Value: capture.Null{}}, // nil value
		},
	}
	
	jsonBytes, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("Failed to marshal CapturedStruct: %v", err)
	}
	
	// Parse back to verify structure
	var result map[string]any
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	
	// Check fields
	if result["ID"] != float64(123) { // JSON numbers are float64
		t.Errorf("Expected ID=123, got: %v", result["ID"])
	}
	if result["Username"] != "alice" {
		t.Errorf("Expected Username='alice', got: %v", result["Username"])
	}
	if result["Active"] != true {
		t.Errorf("Expected Active=true, got: %v", result["Active"])
	}
	if result["Score"] != 98.5 {
		t.Errorf("Expected Score=98.5, got: %v", result["Score"])
	}
	if result["Extra"] != nil { // null in JSON becomes nil in Go
		t.Errorf("Expected Extra=nil (JSON null), got: %v", result["Extra"])
	}
	
	// Test String() method for struct notation
	strRepr := cs.String()
	expected := "{ID:123 Username:alice Active:true Score:98.5 Tags:[admin user] Extra:nil}"
	if strRepr != expected {
		t.Errorf("Expected String() = %q, got: %q", expected, strRepr)
	}
}

func TestNestedCapturedStructJSON(t *testing.T) {
	// Test nested CapturedStruct
	inner := &capture.CapturedStruct{
		TypeName: "Address",
		Fields: []capture.CapturedField{
			{Name: "Street", Value: "123 Main St"},
			{Name: "City", Value: "Springfield"},
		},
	}
	
	outer := &capture.CapturedStruct{
		TypeName: "Person",
		Fields: []capture.CapturedField{
			{Name: "Name", Value: "John"},
			{Name: "Address", Value: inner},
		},
	}
	
	jsonBytes, err := json.Marshal(outer)
	if err != nil {
		t.Fatalf("Failed to marshal nested struct: %v", err)
	}
	
	// Parse back to verify structure
	var result map[string]any
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
	
	// Check nested structure
	if result["Name"] != "John" {
		t.Errorf("Expected Name='John', got: %v", result["Name"])
	}
	
	address, ok := result["Address"].(map[string]any)
	if !ok {
		t.Fatalf("Expected Address to be a map, got: %T", result["Address"])
	}
	
	if address["Street"] != "123 Main St" {
		t.Errorf("Expected Street='123 Main St', got: %v", address["Street"])
	}
	if address["City"] != "Springfield" {
		t.Errorf("Expected City='Springfield', got: %v", address["City"])
	}
}