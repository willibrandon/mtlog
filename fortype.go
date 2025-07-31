package mtlog

import (
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/willibrandon/mtlog/core"
)

// TypeNameOptions controls how type names are extracted and formatted for SourceContext.
//
// Use with extractTypeName to create custom loggers with more control over type name formatting
// than the default ForType function. See examples/fortype/main.go demonstrateTypeNameOptions
// for practical usage examples.
//
// Example:
//
//	opts := TypeNameOptions{IncludePackage: true, Prefix: "MyApp."}
//	name := extractTypeName[User](opts) // Result: "MyApp.mypackage.User"
//	logger := baseLogger.ForContext("SourceContext", name)
type TypeNameOptions struct {
	// IncludePackage determines whether to include the package path in the type name.
	// Default: false (only type name)
	IncludePackage bool

	// PackageDepth limits how many package path segments to include when IncludePackage is true.
	// 0 means include full package path. Default: 1 (only immediate package)
	PackageDepth int

	// Prefix is prepended to the type name for consistent naming.
	// Example: "MyApp." would result in "MyApp.User" for User type
	Prefix string

	// Suffix is appended to the type name.
	// Example: ".Service" would result in "User.Service" for User type
	Suffix string

	// SimplifyAnonymous controls how anonymous struct types are displayed.
	// When true, anonymous structs return "AnonymousStruct" instead of their full definition.
	// Default: false (shows full struct definition)
	SimplifyAnonymous bool

	// WarnOnUnknown controls whether to log warnings for "Unknown" type names.
	// When true, warnings are logged to help with debugging interface types and unresolvable types.
	// When false, warnings are suppressed to reduce log noise in production.
	// Default: true (warnings enabled)
	WarnOnUnknown bool
}

// DefaultTypeNameOptions provides sensible defaults for type name extraction.
var DefaultTypeNameOptions = TypeNameOptions{
	IncludePackage:    false,
	PackageDepth:      1,
	Prefix:            "",
	Suffix:            "",
	SimplifyAnonymous: false,
	WarnOnUnknown:     true,
}

// typeNameCacheEntry represents a cache entry with LRU tracking
type typeNameCacheEntry struct {
	value    string
	lastUsed time.Time
}

// typeNameCacheConfig holds cache configuration
type typeNameCacheConfig struct {
	maxSize int64
	enabled bool
}

// getTypeNameCacheConfig returns the cache configuration from environment variables
func getTypeNameCacheConfig() typeNameCacheConfig {
	config := typeNameCacheConfig{
		maxSize: 10000, // Default: 10,000 entries
		enabled: true,
	}

	if sizeStr := os.Getenv("MTLOG_TYPE_NAME_CACHE_SIZE"); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil && size >= 0 {
			config.maxSize = size
			if size == 0 {
				config.enabled = false
			}
		}
	}

	return config
}

// typeNameCache provides a global cache for type name strings to improve performance.
// This cache is thread-safe and stores computed type names to avoid redundant reflection calls.
// It implements LRU eviction when the cache size exceeds the configured maximum.
var typeNameCache sync.Map

// typeNameCacheStats tracks cache performance metrics including evictions
var typeNameCacheStats struct {
	sync.RWMutex
	hits      int64
	misses    int64
	evictions int64
}

// typeNameCacheConfig holds the global cache configuration
var cacheConfig = getTypeNameCacheConfig()

// debugLogger provides structured logging for ForType warnings and debug information.
// This is initialized lazily to avoid circular dependencies.
var debugLogger interface {
	Debug(messageTemplate string, args ...any)
}

// logDebug logs debug messages using structured logging if available, otherwise falls back to log.Printf.
func logDebug(messageTemplate string, args ...any) {
	if debugLogger != nil {
		debugLogger.Debug(messageTemplate, args...)
	} else {
		// Fallback to standard log for cases where mtlog logger isn't available
		log.Printf("[mtlog] "+messageTemplate, args...)
	}
}

// ExtractTypeName extracts a string representation of the type T using the provided options.
// This is the exported version of extractTypeName for creating custom loggers.
func ExtractTypeName[T any](options TypeNameOptions) string {
	return extractTypeName[T](options)
}

// ExtractTypeNameWithCacheKey extracts a string representation of the type T using the provided options
// and a custom cache key prefix for multi-tenant scenarios. The cacheKeyPrefix allows for
// separate cache namespaces, useful in multi-tenant applications where different tenants
// might have different type naming requirements.
//
// Example usage:
//   tenantPrefix := fmt.Sprintf("tenant:%s", tenantID)
//   name := ExtractTypeNameWithCacheKey[User](options, tenantPrefix)
//   logger := baseLogger.ForContext("SourceContext", name)
func ExtractTypeNameWithCacheKey[T any](options TypeNameOptions, cacheKeyPrefix string) string {
	return extractTypeNameWithCacheKey[T](options, cacheKeyPrefix)
}

// extractTypeNameWithCacheKey extracts a string representation of the type T using the provided options
// and a custom cache key prefix for multi-tenant scenarios.
func extractTypeNameWithCacheKey[T any](options TypeNameOptions, cacheKeyPrefix string) string {
	var zero T
	typ := reflect.TypeOf(zero)

	// Create a cache key that includes the type, options, and custom prefix
	cacheKey := struct {
		Type      reflect.Type
		Options   TypeNameOptions
		KeyPrefix string
	}{typ, options, cacheKeyPrefix}

	return extractTypeNameWithKey[T](typ, options, cacheKey)
}

// extractTypeName extracts a string representation of the type T using the provided options.
func extractTypeName[T any](options TypeNameOptions) string {
	var zero T
	typ := reflect.TypeOf(zero)

	// Create a cache key that includes both the type and options
	cacheKey := struct {
		Type    reflect.Type
		Options TypeNameOptions
	}{typ, options}

	return extractTypeNameWithKey[T](typ, options, cacheKey)
}

// extractTypeNameWithKey is the core implementation that handles both regular and custom cache keys
func extractTypeNameWithKey[T any](typ reflect.Type, options TypeNameOptions, cacheKey any) string {

	// Check cache first
	if cached, ok := typeNameCache.Load(cacheKey); ok {
		if entry, ok := cached.(typeNameCacheEntry); ok {
			// Cache hit - update last used time for LRU
			entry.lastUsed = time.Now()
			typeNameCache.Store(cacheKey, entry)

			typeNameCacheStats.Lock()
			typeNameCacheStats.hits++
			typeNameCacheStats.Unlock()
			return entry.value
		}
	}

	// Cache miss
	typeNameCacheStats.Lock()
	typeNameCacheStats.misses++
	typeNameCacheStats.Unlock()

	// Handle pointer types by getting the element type
	for typ != nil && typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ == nil {
		result := "Unknown"
		if cacheConfig.enabled {
			entry := typeNameCacheEntry{
				value:    result,
				lastUsed: time.Now(),
			}
			typeNameCache.Store(cacheKey, entry)
			evictLRUEntriesIfNeeded()
		}
		// Log warning for Unknown type names to help with debugging (if enabled)
		if options.WarnOnUnknown {
			logDebug("Warning: Unable to determine type name, returning 'Unknown'. This may indicate an interface{{}} or unresolvable type.")
		}
		return result
	}

	var name string

	if options.IncludePackage && typ.PkgPath() != "" {
		pkgPath := typ.PkgPath()

		// Apply package depth limiting
		if options.PackageDepth > 0 {
			parts := strings.Split(pkgPath, "/")
			if len(parts) > options.PackageDepth {
				parts = parts[len(parts)-options.PackageDepth:]
			}
			pkgPath = strings.Join(parts, "/")
		}

		typeName := typ.Name()

		// For generic types, clean up package paths in type parameters when IncludePackage=false in nested generics
		if !options.IncludePackage && strings.Contains(typeName, "[") {
			// More robust cleanup: parse generic type parameters using reflection
			typeName = cleanGenericTypeName(typ, options)
		}

		name = pkgPath + "." + typeName
	} else {
		typeName := typ.Name()

		// For generic types without package inclusion, clean up package paths in type parameters
		if strings.Contains(typeName, "[") {
			// More robust cleanup: parse generic type parameters using reflection
			typeName = cleanGenericTypeName(typ, options)
		}

		name = typeName
	}

	// Handle empty names (for built-in types, interfaces, etc.)
	if name == "" || name == "." {
		if options.SimplifyAnonymous && (typ.Kind() == reflect.Struct || typ.Kind() == reflect.Interface) {
			name = "AnonymousStruct"
		} else {
			name = typ.String()
			// Log warning for interface types which result in "Unknown" (if enabled)
			if options.WarnOnUnknown && (name == "" || typ.Kind() == reflect.Interface) {
				logDebug("Warning: Type {Type} resolved to empty or interface type, using type string representation: {TypeString}", typ, name)
			}
		}
	}

	// Apply prefix and suffix
	name = options.Prefix + name + options.Suffix

	// Store in cache with LRU tracking
	if cacheConfig.enabled {
		entry := typeNameCacheEntry{
			value:    name,
			lastUsed: time.Now(),
		}
		typeNameCache.Store(cacheKey, entry)

		// Trigger eviction if cache is getting too large
		// We do this synchronously but with a simple size check to avoid deadlocks
		evictLRUEntriesIfNeeded()
	}

	return name
}

// getTypeNameSimple is a convenience function that extracts just the type name without package.
func getTypeNameSimple[T any]() string {
	return extractTypeName[T](DefaultTypeNameOptions)
}

// getTypeNameWithPackage extracts the type name including the immediate package name.
func getTypeNameWithPackage[T any]() string {
	opts := DefaultTypeNameOptions
	opts.IncludePackage = true
	opts.PackageDepth = 1
	return extractTypeName[T](opts)
}

// GetTypeNameSimple is the exported version of getTypeNameSimple for testing and benchmarking.
func GetTypeNameSimple[T any]() string {
	return getTypeNameSimple[T]()
}

// GetTypeNameWithPackage is the exported version of getTypeNameWithPackage for testing and benchmarking.
func GetTypeNameWithPackage[T any]() string {
	return getTypeNameWithPackage[T]()
}

// TypeNameCacheStats provides performance statistics for the type name cache.
type TypeNameCacheStats struct {
	Hits      int64   // Number of cache hits
	Misses    int64   // Number of cache misses
	Evictions int64   // Number of cache evictions due to LRU policy
	HitRatio  float64 // Hit ratio as a percentage (0-100)
	Size      int64   // Number of entries currently in the cache
	MaxSize   int64   // Maximum cache size before eviction
}

// GetTypeNameCacheStats returns current cache performance statistics.
func GetTypeNameCacheStats() TypeNameCacheStats {
	typeNameCacheStats.RLock()
	defer typeNameCacheStats.RUnlock()

	hits := typeNameCacheStats.hits
	misses := typeNameCacheStats.misses
	evictions := typeNameCacheStats.evictions
	total := hits + misses

	var hitRatio float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total) * 100
	}

	// Count cache entries
	var size int64
	typeNameCache.Range(func(key, value any) bool {
		if _, ok := value.(typeNameCacheEntry); ok {
			size++
		}
		return true
	})

	return TypeNameCacheStats{
		Hits:      hits,
		Misses:    misses,
		Evictions: evictions,
		HitRatio:  hitRatio,
		Size:      size,
		MaxSize:   cacheConfig.maxSize,
	}
}

// evictLRUEntriesIfNeeded checks cache size and evicts entries if needed
func evictLRUEntriesIfNeeded() {
	if !cacheConfig.enabled || cacheConfig.maxSize <= 0 {
		return
	}

	// Quick size check to avoid expensive operations if eviction isn't needed
	var currentSize int64
	typeNameCache.Range(func(key, value any) bool {
		currentSize++
		// Stop counting if we're clearly over the limit
		if currentSize > cacheConfig.maxSize*2 {
			return false
		}
		return true
	})

	// Only trigger eviction if we're over the limit
	if currentSize <= cacheConfig.maxSize {
		return
	}

	evictLRUEntries()
}

// evictLRUEntries removes the least recently used entries when cache exceeds maxSize
func evictLRUEntries() {
	// Collect all entries with their access times
	type entryInfo struct {
		key      any
		lastUsed time.Time
	}

	var entries []entryInfo
	typeNameCache.Range(func(key, value any) bool {
		if entry, ok := value.(typeNameCacheEntry); ok {
			entries = append(entries, entryInfo{
				key:      key,
				lastUsed: entry.lastUsed,
			})
		}
		return true
	})

	// Check if eviction is needed
	if int64(len(entries)) <= cacheConfig.maxSize {
		return
	}

	// Sort by lastUsed time (oldest first) - using a simple bubble sort for simplicity
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].lastUsed.After(entries[j].lastUsed) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Remove oldest entries to get under the limit
	entriesToRemove := int64(len(entries)) - cacheConfig.maxSize
	evicted := int64(0)
	for i := int64(0); i < entriesToRemove && i < int64(len(entries)); i++ {
		typeNameCache.Delete(entries[i].key)
		evicted++
	}

	// Update eviction counter
	if evicted > 0 {
		typeNameCacheStats.Lock()
		typeNameCacheStats.evictions += evicted
		typeNameCacheStats.Unlock()
	}
}

// ResetTypeNameCache clears the type name cache and statistics. Useful for testing and benchmarking.
func ResetTypeNameCache() {
	typeNameCache = sync.Map{}
	typeNameCacheStats.Lock()
	typeNameCacheStats.hits = 0
	typeNameCacheStats.misses = 0
	typeNameCacheStats.evictions = 0
	typeNameCacheStats.Unlock()
}

// cleanGenericTypeName provides more robust generic type name cleaning using reflection
// to avoid issues with packages that have similar prefixes or complex type parameters.
func cleanGenericTypeName(typ reflect.Type, options TypeNameOptions) string {
	// Get the raw type name
	typeName := typ.Name()

	// If it's not a generic type, return as-is
	if !strings.Contains(typeName, "[") {
		return typeName
	}

	// Find the opening bracket
	openBracket := strings.Index(typeName, "[")
	if openBracket == -1 {
		return typeName
	}

	// Extract base type name and generic parameters
	baseName := typeName[:openBracket]
	paramsPart := typeName[openBracket:]

	// If we want to include package, don't clean anything
	if options.IncludePackage {
		return typeName
	}

	// For complex cleaning, we need to be more careful about nested generics
	// Parse the parameters recursively, removing only the current package path
	if typ.PkgPath() != "" {
		// Only clean our own package path, not arbitrary package paths
		packagePrefix := typ.PkgPath() + "."
		paramsPart = strings.ReplaceAll(paramsPart, packagePrefix, "")

		// Also handle nested package paths that might be from the same package
		// but be careful not to remove legitimate periods in type names
		if strings.Count(packagePrefix, ".") > 1 {
			// For complex package paths like "github.com/owner/repo.Type"
			// we only want to remove the exact full path, not partial matches
			parts := strings.Split(typ.PkgPath(), "/")
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1] + "."
				paramsPart = strings.ReplaceAll(paramsPart, lastPart, "")
			}
		}
	}

	return baseName + paramsPart
}

// ForType creates a logger with SourceContext automatically set from the type name.
// The type name is extracted using reflection and provides a convenient way to categorize
// logs by the types they relate to.
//
// By default, only the type name is used (without package). For example:
//
//	type User struct { Name string }
//	ForType[User](logger).Information("User created") // SourceContext: "User"
//	ForType[*User](logger).Information("User updated") // SourceContext: "User" (pointer dereferenced)
//
// This is equivalent to:
//
//	logger.ForContext("SourceContext", "User").Information("User created")
//
// But more convenient and less error-prone for type-specific logging.
//
// Performance: Uses reflection for type name extraction, which incurs a small performance
// overhead (~7%) compared to manual ForSourceContext. For high-performance scenarios,
// consider using ForSourceContext with string literals.
//
// Edge Cases:
//   - Anonymous structs: Return their full definition (e.g., "struct { Name string }")
//   - Interfaces: Return "Unknown" as they have no concrete type name
//   - Generic types: Include type parameters (e.g., "GenericType[string]")
//   - Built-in types: Return their standard names (e.g., "string", "int", "[]string")
//   - Function/channel types: Return their signature (e.g., "func(string) error", "chan string")
//
// For more control over type name formatting, use extractTypeName with custom TypeNameOptions.
// See createCustomLogger in examples/fortype/main.go for a pattern to create custom loggers
// with TypeNameOptions for specific naming requirements.
func ForType[T any](logger core.Logger) core.Logger {
	typeName := getTypeNameSimple[T]()
	return logger.ForContext("SourceContext", typeName)
}

// ForTypeWithCacheKey creates a logger with SourceContext automatically set from the type name
// using a custom cache key prefix. This is useful in multi-tenant scenarios where different
// tenants might require separate cache namespaces.
//
// Example usage:
//   tenantPrefix := fmt.Sprintf("tenant:%s", tenantID)
//   userLogger := ForTypeWithCacheKey[User](logger, tenantPrefix)
//   userLogger.Information("User operation") // SourceContext: "User" (cached per tenant)
func ForTypeWithCacheKey[T any](logger core.Logger, cacheKeyPrefix string) core.Logger {
	typeName := ExtractTypeNameWithCacheKey[T](DefaultTypeNameOptions, cacheKeyPrefix)
	return logger.ForContext("SourceContext", typeName)
}
