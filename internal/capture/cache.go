package capture

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/willibrandon/mtlog/core"
)

// typeDescriptor caches reflection information about a type.
type typeDescriptor struct {
	Type   reflect.Type
	Kind   reflect.Kind
	Fields []fieldDescriptor
}

// fieldDescriptor caches information about a struct field.
type fieldDescriptor struct {
	Index      int
	Name       string
	Type       reflect.Type
	Tag        string
	IsExported bool
}

// typeCache is a thread-safe cache for type descriptors.
type typeCache struct {
	mu    sync.RWMutex
	cache map[reflect.Type]*typeDescriptor
}

// newTypeCache creates a new type cache.
func newTypeCache() *typeCache {
	return &typeCache{
		cache: make(map[reflect.Type]*typeDescriptor),
	}
}

// get retrieves a type descriptor from the cache.
func (tc *typeCache) get(t reflect.Type) (*typeDescriptor, bool) {
	tc.mu.RLock()
	desc, ok := tc.cache[t]
	tc.mu.RUnlock()
	return desc, ok
}

// getOrCreate retrieves a type descriptor or creates it if not cached.
func (tc *typeCache) getOrCreate(t reflect.Type) *typeDescriptor {
	// Try to get from cache first
	if desc, ok := tc.get(t); ok {
		return desc
	}

	// Create new descriptor
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Double-check after acquiring write lock
	if desc, ok := tc.cache[t]; ok {
		return desc
	}

	desc := tc.createDescriptor(t)
	tc.cache[t] = desc
	return desc
}

// createDescriptor creates a type descriptor for a type.
func (tc *typeCache) createDescriptor(t reflect.Type) *typeDescriptor {
	desc := &typeDescriptor{
		Type: t,
		Kind: t.Kind(),
	}

	// Cache struct fields if applicable
	if t.Kind() == reflect.Struct {
		desc.Fields = make([]fieldDescriptor, 0, t.NumField())

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)

			// Skip unexported fields
			if field.PkgPath != "" {
				continue
			}

			// Get log tag
			tag := field.Tag.Get("log")
			if tag == "-" {
				continue // Skip fields marked with log:"-"
			}

			fieldDesc := fieldDescriptor{
				Index:      i,
				Name:       field.Name,
				Type:       field.Type,
				Tag:        tag,
				IsExported: field.PkgPath == "",
			}

			// Use tag name if provided
			if tag != "" && tag != "-" {
				fieldDesc.Name = tag
			}

			desc.Fields = append(desc.Fields, fieldDesc)
		}
	}

	return desc
}

// Global type cache instance
var globalTypeCache = newTypeCache()

// CachedCapturer is a capturer that caches type information.
type CachedCapturer struct {
	*DefaultCapturer
	typeCache *typeCache
}

// NewCachedCapturer creates a new cached capturer.
func NewCachedCapturer() *CachedCapturer {
	return &CachedCapturer{
		DefaultCapturer: NewDefaultCapturer(),
		typeCache:       globalTypeCache,
	}
}

// TryCapture attempts to capture a value using cached type information.
func (d *CachedCapturer) TryCapture(value any, propertyFactory core.LogEventPropertyFactory) (*core.LogEventProperty, bool) {
	if value == nil {
		return propertyFactory.CreateProperty("", Null{}), true
	}

	// Check if the value implements LogValue interface
	if lv, ok := value.(core.LogValue); ok {
		// Recursively capture the LogValue result
		logValue := lv.LogValue()
		captured := d.capture(logValue, 0)
		return propertyFactory.CreateProperty("", captured), true
	}

	captured := d.capture(value, 0)
	return propertyFactory.CreateProperty("", captured), true
}

// captureStructCached captures a struct using cached type information.
func (d *CachedCapturer) captureStructCached(v reflect.Value, depth int) any {
	t := v.Type()
	desc := d.typeCache.getOrCreate(t)

	fields := make([]CapturedField, 0, len(desc.Fields))

	for _, field := range desc.Fields {
		fieldValue := v.Field(field.Index)
		fields = append(fields, CapturedField{
			Name:  field.Name,
			Value: d.capture(fieldValue.Interface(), depth+1),
		})
	}

	// Return CapturedStruct to preserve type information and field order
	return &CapturedStruct{
		TypeName: t.String(),
		Fields:   fields,
	}
}

// captureSliceCached captures a slice checking for LogValue on elements.
func (d *CachedCapturer) captureSliceCached(v reflect.Value, depth int) any {
	// Check if slice is nil
	if v.Kind() == reflect.Slice && v.IsNil() {
		return Null{}
	}
	
	// Special handling for byte slices - render as string if valid UTF-8
	if v.Type().Elem().Kind() == reflect.Uint8 {
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

// Override the capture method to use caching
func (d *CachedCapturer) capture(value any, depth int) any {
	if value == nil {
		return Null{}  // Return Null{} sentinel type
	}

	// Check depth limit
	if depth >= d.maxDepth {
		return formatType(value)
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
		return d.capture(v.Elem().Interface(), depth)

	case reflect.Interface:
		if v.IsNil() {
			return Null{}  // Return Null{} sentinel type
		}
		return d.capture(v.Elem().Interface(), depth)

	case reflect.Slice, reflect.Array:
		return d.captureSliceCached(v, depth)

	case reflect.Map:
		return d.captureMap(v, depth)

	case reflect.Struct:
		// Check if it's a known scalar type
		if d.scalarTypes[t] {
			return value
		}

		// Special handling for time.Time
		if isTimeType(t) {
			return formatTime(value)
		}

		// Use cached capturing for structs
		return d.captureStructCached(v, depth)

	case reflect.Func, reflect.Chan:
		return formatType(value)

	default:
		return formatValue(value)
	}
}

// Helper functions
func formatType(v any) string {
	return reflect.TypeOf(v).String()
}

func formatValue(v any) string {
	return fmt.Sprintf("%v", v)
}

func isTimeType(t reflect.Type) bool {
	return t.PkgPath() == "time" && t.Name() == "Time"
}

func formatTime(v any) string {
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	return formatValue(v)
}
