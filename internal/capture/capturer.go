package capture

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

// Null is a sentinel type for nil values that renders as "nil" in strings but null in JSON
type Null struct{}

// String returns "nil" for Go-idiomatic string representation
func (Null) String() string {
	return "nil"
}

// MarshalJSON returns JSON null
func (Null) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}

// convertBytesToStringIfPrintable converts byte slices to strings if they contain printable UTF-8 text.
func convertBytesToStringIfPrintable(bytes []byte, maxLength int) interface{} {
	if utf8.Valid(bytes) && len(bytes) > 0 {
		// Check for null bytes (binary indicator)
		for _, b := range bytes {
			if b == 0 {
				return bytes // Binary data
			}
		}
		s := string(bytes)
		if len(s) > maxLength {
			return s[:maxLength] + "..."
		}
		return s
	}
	return bytes
}

// CapturedField represents a single field in a captured struct.
type CapturedField struct {
	Name  string
	Value any
}

// CapturedStruct represents a captured struct that preserves type information and field order.
type CapturedStruct struct {
	TypeName string
	Fields   []CapturedField // Preserves field order
}

// String implements fmt.Stringer to properly format the struct
func (cs *CapturedStruct) String() string {
	if cs == nil {
		return "nil"  // Go convention: nil without brackets
	}
	
	var parts []string
	for _, field := range cs.Fields {
		// Format each field as Key:Value
		var valueStr string
		switch v := field.Value.(type) {
		case *CapturedStruct:
			valueStr = v.String()
		case string:
			valueStr = v  // Don't use fmt.Sprintf which might re-encode
		default:
			valueStr = fmt.Sprintf("%v", v)
		}
		parts = append(parts, fmt.Sprintf("%s:%s", field.Name, valueStr))
	}
	
	// Return in struct notation: {Field1:Value1 Field2:Value2}
	return "{" + strings.Join(parts, " ") + "}"
}

// MarshalJSON implements json.Marshaler to output the struct as a JSON object
func (cs *CapturedStruct) MarshalJSON() ([]byte, error) {
	if cs == nil {
		return []byte("null"), nil
	}
	
	// Convert to a map for proper JSON object output
	m := make(map[string]any, len(cs.Fields))
	for _, field := range cs.Fields {
		m[field.Name] = field.Value
	}
	return json.Marshal(m)
}

// DefaultCapturer is the default implementation of core.Capturer.
type DefaultCapturer struct {
	maxDepth           int
	maxStringLength    int
	maxCollectionCount int
	scalarTypes        map[reflect.Type]bool
}

// NewDefaultCapturer creates a new capturer with default settings.
func NewDefaultCapturer() *DefaultCapturer {
	d := &DefaultCapturer{
		maxDepth:           5,  // Increased from 3 to handle deeper nesting
		maxStringLength:    1000,
		maxCollectionCount: 100,
		scalarTypes:        make(map[reflect.Type]bool),
	}

	// Register common scalar types
	// Note: time.Time is NOT registered as scalar so it gets formatted
	d.RegisterScalarType(reflect.TypeOf(time.Duration(0)))

	return d
}

// NewCapturer creates a capturer with custom limits.
func NewCapturer(maxDepth, maxStringLength, maxCollectionCount int) *DefaultCapturer {
	d := &DefaultCapturer{
		maxDepth:           maxDepth,
		maxStringLength:    maxStringLength,
		maxCollectionCount: maxCollectionCount,
		scalarTypes:        make(map[reflect.Type]bool),
	}

	// Register common scalar types
	// Note: time.Time is NOT registered as scalar so it gets formatted
	d.RegisterScalarType(reflect.TypeOf(time.Duration(0)))

	return d
}

// RegisterScalarType registers a type that should be treated as a scalar.
func (d *DefaultCapturer) RegisterScalarType(t reflect.Type) {
	d.scalarTypes[t] = true
}

// TryCapture attempts to capture a value into a log-friendly representation.
func (d *DefaultCapturer) TryCapture(value any, propertyFactory core.LogEventPropertyFactory) (prop *core.LogEventProperty, ok bool) {
	// Recover from panics during capturing
	defer func() {
		if r := recover(); r != nil {
			if selflog.IsEnabled() {
				selflog.Printf("[capture] panic during capturing: %v (type=%T)", r, value)
			}
			// Return the value as-is with type information
			prop = propertyFactory.CreateProperty("", fmt.Sprintf("%T(%v)", value, value))
			ok = true
		}
	}()

	if value == nil {
		return propertyFactory.CreateProperty("", Null{}), true
	}

	// Check if the value implements LogValue interface
	if lv, ok := value.(core.LogValue); ok {
		// Protect against panics in LogValue implementation
		var logValue any
		func() {
			defer func() {
				if r := recover(); r != nil {
					if selflog.IsEnabled() {
						selflog.Printf("[capture] LogValue.LogValue() panicked: %v (type=%T)", r, value)
					}
					// Use the original value if LogValue panics
					logValue = value
				}
			}()
			logValue = lv.LogValue()
		}()

		captured := d.capture(logValue, 0)
		return propertyFactory.CreateProperty("", captured), true
	}

	captured := d.capture(value, 0)
	return propertyFactory.CreateProperty("", captured), true
}

// capture recursively captures a value.
func (d *DefaultCapturer) capture(value any, depth int) (result any) {
	// Recover from panics during reflection operations
	defer func() {
		if r := recover(); r != nil {
			if selflog.IsEnabled() {
				selflog.Printf("[capture] panic during reflection: %v (type=%T, depth=%d)", r, value, depth)
			}
			// Return safe representation
			result = fmt.Sprintf("%T(%v)", value, value)
		}
	}()

	if value == nil {
		return Null{}  // Return Null{} sentinel type
	}

	// Check depth limit
	if depth >= d.maxDepth {
		// For simple types, return the value even at max depth
		switch value.(type) {
		case bool, int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64,
			float32, float64, complex64, complex128,
			string:
			return value
		case map[string]any:
			// For maps, show it's truncated
			return "<max depth reached>"
		default:
			// For complex types, show truncation indicator
			return "<max depth reached>"
		}
	}

	v := reflect.ValueOf(value)
	t := v.Type()

	// Handle basic types
	switch v.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return value

	case reflect.String:
		s := v.String()
		if len(s) > d.maxStringLength {
			return s[:d.maxStringLength] + "..."
		}
		return s

	case reflect.Ptr:
		if v.IsNil() {
			return Null{}  // Return Null{} sentinel type
		}
		// Dereference the pointer and capture the underlying value
		elem := v.Elem()
		// If it's a struct, capture it as a struct
		if elem.Kind() == reflect.Struct {
			return d.captureStruct(elem, depth)
		}
		return d.capture(elem.Interface(), depth)

	case reflect.Interface:
		if v.IsNil() {
			return Null{}  // Return Null{} sentinel type
		}
		return d.capture(v.Elem().Interface(), depth)

	case reflect.Slice, reflect.Array:
		return d.captureSlice(v, depth)

	case reflect.Map:
		return d.captureMap(v, depth)

	case reflect.Struct:
		// Check if it's a known scalar type
		if d.scalarTypes[t] {
			return value
		}

		// Special handling for time.Time
		if _, ok := value.(time.Time); ok {
			return value.(time.Time).Format(time.RFC3339)
		}

		return d.captureStruct(v, depth)

	case reflect.Func, reflect.Chan:
		return fmt.Sprintf("%T", value)

	default:
		// Simplified default case
		if v.Kind() == reflect.Struct {
			return d.captureStruct(v, depth)
		}
		return fmt.Sprintf("%v", value)
	}
}

// captureSlice captures a slice or array.
func (d *DefaultCapturer) captureSlice(v reflect.Value, depth int) any {
	// Check if slice is nil
	if v.Kind() == reflect.Slice && v.IsNil() {
		return Null{}
	}
	
	// Special handling for byte slices - render as string if valid UTF-8
	if v.Type().Elem().Kind() == reflect.Uint8 {
		// Can't use v.Bytes() if it's not addressable, get the bytes differently
		length := v.Len()
		bytes := make([]byte, length)
		for i := 0; i < length; i++ {
			bytes[i] = byte(v.Index(i).Uint())
		}
		
		
		// Use shared function to convert bytes to string if printable
		result := convertBytesToStringIfPrintable(bytes, d.maxStringLength)
		if str, ok := result.(string); ok {
			return str
		}
		// For binary data, show first few bytes and length
		if len(bytes) > 20 {
			return fmt.Sprintf("[%d bytes: %v...]", len(bytes), bytes[:20])
		}
		return bytes
	}

	length := v.Len()
	if length == 0 {
		return []any{}
	}

	// Limit collection size
	if length > d.maxCollectionCount {
		length = d.maxCollectionCount
	}

	result := make([]any, length)
	for i := 0; i < length; i++ {
		elemValue := v.Index(i)
		
		// If it's an interface, check if it contains a pointer to a struct
		if elemValue.Kind() == reflect.Interface && !elemValue.IsNil() {
			actualElem := elemValue.Elem()
			// If it's a pointer to a struct, dereference it
			if actualElem.Kind() == reflect.Ptr && !actualElem.IsNil() && actualElem.Elem().Kind() == reflect.Struct {
				// Capture the dereferenced struct
				result[i] = d.capture(actualElem.Elem().Interface(), depth+1)
				continue
			}
		}
		
		elem := elemValue.Interface()

		// Check if element implements LogValue
		if lv, ok := elem.(core.LogValue); ok {
			result[i] = d.capture(lv.LogValue(), depth+1)
		} else {
			// Check if it's a struct BEFORE capturing
			ev := reflect.ValueOf(elem)
			if ev.Kind() == reflect.Struct {
				// Capture it directly as a struct to avoid the default case
				result[i] = d.captureStruct(ev, depth+1)
			} else {
				result[i] = d.capture(elem, depth+1)
			}
		}
	}

	// Add indicator if truncated
	if v.Len() > d.maxCollectionCount {
		result = append(result, fmt.Sprintf("... (%d more)", v.Len()-d.maxCollectionCount))
	}

	return result
}

// isPrintableUTF8 checks if bytes are valid and printable UTF-8 text
func isPrintableUTF8(b []byte) bool {
	if !utf8.Valid(b) {
		return false
	}
	// Check if it contains mostly printable characters
	nonPrintable := 0
	for _, r := range string(b) {
		// Count non-printable characters (control chars except whitespace)
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			nonPrintable++
		}
		// Also reject if we see null bytes or other binary indicators
		if r == 0 {
			return false
		}
	}
	// Consider it text if we have very few non-printable chars
	return len(b) > 0 && nonPrintable < len(b)/10  // Allow up to 10% non-printable
}

// captureMap captures a map.
func (d *DefaultCapturer) captureMap(v reflect.Value, depth int) any {
	// Check if map is nil
	if v.IsNil() {
		return Null{}
	}
	
	if v.Len() == 0 {
		return map[string]any{}
	}

	result := make(map[string]any)
	count := 0

	for _, key := range v.MapKeys() {
		if count >= d.maxCollectionCount {
			result["..."] = fmt.Sprintf("(%d more)", v.Len()-count)
			break
		}

		keyStr := fmt.Sprintf("%v", key.Interface())
		result[keyStr] = d.capture(v.MapIndex(key).Interface(), depth+1)
		count++
	}

	return result
}

// captureStruct captures a struct.
func (d *DefaultCapturer) captureStruct(v reflect.Value, depth int) any {
	t := v.Type()
	var fields []CapturedField

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Skip fields with log:"-" tag
		tag := field.Tag.Get("log")
		if tag == "-" {
			continue
		}

		fieldValue := v.Field(i)
		fieldName := field.Name

		// Use tag name if provided
		if tag != "" && tag != "-" {
			fieldName = tag
		}

		capturedValue := d.capture(fieldValue.Interface(), depth+1)
		
		
		// Special handling for byte slices in struct fields
		if bytes, ok := capturedValue.([]byte); ok {
			result := convertBytesToStringIfPrintable(bytes, d.maxStringLength)
			if str, ok := result.(string); ok {
				capturedValue = str
			}
		}
		
		fields = append(fields, CapturedField{
			Name:  fieldName,
			Value: capturedValue,
		})
	}

	// Return CapturedStruct to preserve type information and field order
	return &CapturedStruct{
		TypeName: t.String(),
		Fields:   fields,
	}
}
