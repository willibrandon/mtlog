package destructure

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

// CachedDestructurer is a destructurer that caches type information.
type CachedDestructurer struct {
	*DefaultDestructurer
	typeCache *typeCache
}

// NewCachedDestructurer creates a new cached destructurer.
func NewCachedDestructurer() *CachedDestructurer {
	return &CachedDestructurer{
		DefaultDestructurer: NewDefaultDestructurer(),
		typeCache:          globalTypeCache,
	}
}

// TryDestructure attempts to destructure a value using cached type information.
func (d *CachedDestructurer) TryDestructure(value interface{}, propertyFactory core.LogEventPropertyFactory) (*core.LogEventProperty, bool) {
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

// destructureStructCached destructures a struct using cached type information.
func (d *CachedDestructurer) destructureStructCached(v reflect.Value, depth int) interface{} {
	t := v.Type()
	desc := d.typeCache.getOrCreate(t)
	
	result := make(map[string]interface{}, len(desc.Fields))
	
	for _, field := range desc.Fields {
		fieldValue := v.Field(field.Index)
		result[field.Name] = d.destructure(fieldValue.Interface(), depth+1)
	}
	
	return result
}

// destructureSliceCached destructures a slice checking for LogValue on elements.
func (d *CachedDestructurer) destructureSliceCached(v reflect.Value, depth int) interface{} {
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

// Override the destructure method to use caching
func (d *CachedDestructurer) destructure(value interface{}, depth int) interface{} {
	if value == nil {
		return nil
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
			return nil
		}
		return d.destructure(v.Elem().Interface(), depth)
		
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return d.destructure(v.Elem().Interface(), depth)
		
	case reflect.Slice, reflect.Array:
		return d.destructureSliceCached(v, depth)
		
	case reflect.Map:
		return d.destructureMap(v, depth)
		
	case reflect.Struct:
		// Check if it's a known scalar type
		if d.scalarTypes[t] {
			return value
		}
		
		// Special handling for time.Time
		if isTimeType(t) {
			return formatTime(value)
		}
		
		// Use cached destructuring for structs
		return d.destructureStructCached(v, depth)
		
	case reflect.Func, reflect.Chan:
		return formatType(value)
		
	default:
		return formatValue(value)
	}
}

// Helper functions
func formatType(v interface{}) string {
	return reflect.TypeOf(v).String()
}

func formatValue(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

func isTimeType(t reflect.Type) bool {
	return t.PkgPath() == "time" && t.Name() == "Time"
}

func formatTime(v interface{}) string {
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	return formatValue(v)
}