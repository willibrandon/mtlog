package capture

import (
	"fmt"
	"reflect"
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

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
		maxDepth:           3,
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
func (d *DefaultCapturer) TryCapture(value interface{}, propertyFactory core.LogEventPropertyFactory) (prop *core.LogEventProperty, ok bool) {
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
		return propertyFactory.CreateProperty("", nil), true
	}
	
	// Check if the value implements LogValue interface
	if lv, ok := value.(core.LogValue); ok {
		// Protect against panics in LogValue implementation
		var logValue interface{}
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
func (d *DefaultCapturer) capture(value interface{}, depth int) (result interface{}) {
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
		return nil
	}
	
	// Check depth limit
	if depth >= d.maxDepth {
		return fmt.Sprintf("%T", value)
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
			return nil
		}
		return d.capture(v.Elem().Interface(), depth)
		
	case reflect.Interface:
		if v.IsNil() {
			return nil
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
		return fmt.Sprintf("%v", value)
	}
}

// captureSlice captures a slice or array.
func (d *DefaultCapturer) captureSlice(v reflect.Value, depth int) interface{} {
	length := v.Len()
	if length == 0 {
		return []interface{}{}
	}
	
	// Limit collection size
	if length > d.maxCollectionCount {
		length = d.maxCollectionCount
	}
	
	result := make([]interface{}, length)
	for i := 0; i < length; i++ {
		elem := v.Index(i).Interface()
		
		// Check if element implements LogValue
		if lv, ok := elem.(core.LogValue); ok {
			result[i] = d.capture(lv.LogValue(), depth+1)
		} else {
			result[i] = d.capture(elem, depth+1)
		}
	}
	
	// Add indicator if truncated
	if v.Len() > d.maxCollectionCount {
		result = append(result, fmt.Sprintf("... (%d more)", v.Len()-d.maxCollectionCount))
	}
	
	return result
}

// captureMap captures a map.
func (d *DefaultCapturer) captureMap(v reflect.Value, depth int) interface{} {
	if v.Len() == 0 {
		return map[string]interface{}{}
	}
	
	result := make(map[string]interface{})
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
func (d *DefaultCapturer) captureStruct(v reflect.Value, depth int) interface{} {
	t := v.Type()
	result := make(map[string]interface{})
	
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
		
		result[fieldName] = d.capture(fieldValue.Interface(), depth+1)
	}
	
	return result
}