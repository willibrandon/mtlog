package destructure

import (
	"fmt"
	"reflect"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// DefaultDestructurer is the default implementation of core.Destructurer.
type DefaultDestructurer struct {
	maxDepth           int
	maxStringLength    int
	maxCollectionCount int
	scalarTypes        map[reflect.Type]bool
}

// NewDefaultDestructurer creates a new destructurer with default settings.
func NewDefaultDestructurer() *DefaultDestructurer {
	d := &DefaultDestructurer{
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

// NewDestructurer creates a destructurer with custom limits.
func NewDestructurer(maxDepth, maxStringLength, maxCollectionCount int) *DefaultDestructurer {
	d := &DefaultDestructurer{
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
func (d *DefaultDestructurer) RegisterScalarType(t reflect.Type) {
	d.scalarTypes[t] = true
}

// TryDestructure attempts to destructure a value into a log-friendly representation.
func (d *DefaultDestructurer) TryDestructure(value interface{}, propertyFactory core.LogEventPropertyFactory) (*core.LogEventProperty, bool) {
	if value == nil {
		return propertyFactory.CreateProperty("", nil), true
	}
	
	// Check if the value implements LogValue interface
	if lv, ok := value.(core.LogValue); ok {
		// Recursively destructure the LogValue result
		logValue := lv.LogValue()
		destructured := d.destructure(logValue, 0)
		return propertyFactory.CreateProperty("", destructured), true
	}
	
	destructured := d.destructure(value, 0)
	return propertyFactory.CreateProperty("", destructured), true
}

// destructure recursively destructures a value.
func (d *DefaultDestructurer) destructure(value interface{}, depth int) interface{} {
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
		return d.destructure(v.Elem().Interface(), depth)
		
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return d.destructure(v.Elem().Interface(), depth)
		
	case reflect.Slice, reflect.Array:
		return d.destructureSlice(v, depth)
		
	case reflect.Map:
		return d.destructureMap(v, depth)
		
	case reflect.Struct:
		// Check if it's a known scalar type
		if d.scalarTypes[t] {
			return value
		}
		
		// Special handling for time.Time
		if _, ok := value.(time.Time); ok {
			return value.(time.Time).Format(time.RFC3339)
		}
		
		return d.destructureStruct(v, depth)
		
	case reflect.Func, reflect.Chan:
		return fmt.Sprintf("%T", value)
		
	default:
		return fmt.Sprintf("%v", value)
	}
}

// destructureSlice destructures a slice or array.
func (d *DefaultDestructurer) destructureSlice(v reflect.Value, depth int) interface{} {
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
			result[i] = d.destructure(lv.LogValue(), depth+1)
		} else {
			result[i] = d.destructure(elem, depth+1)
		}
	}
	
	// Add indicator if truncated
	if v.Len() > d.maxCollectionCount {
		result = append(result, fmt.Sprintf("... (%d more)", v.Len()-d.maxCollectionCount))
	}
	
	return result
}

// destructureMap destructures a map.
func (d *DefaultDestructurer) destructureMap(v reflect.Value, depth int) interface{} {
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
		result[keyStr] = d.destructure(v.MapIndex(key).Interface(), depth+1)
		count++
	}
	
	return result
}

// destructureStruct destructures a struct.
func (d *DefaultDestructurer) destructureStruct(v reflect.Value, depth int) interface{} {
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
		
		result[fieldName] = d.destructure(fieldValue.Interface(), depth+1)
	}
	
	return result
}