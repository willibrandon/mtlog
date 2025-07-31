package mtlog

import (
	"log"
	"strings"
	"sync"
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/sinks"
)

// Test types for type name extraction
type testUser struct {
	Name string
}

type testProduct struct {
	Price float64
}

func TestExtractTypeNameBasic(t *testing.T) {
	tests := []struct {
		name         string
		extractFunc  func() string
		expectedName string
	}{
		{
			name: "struct type",
			extractFunc: func() string {
				return getTypeNameSimple[testUser]()
			},
			expectedName: "testUser",
		},
		{
			name: "pointer type",
			extractFunc: func() string {
				return getTypeNameSimple[*testUser]()
			},
			expectedName: "testUser", // Should dereference
		},
		{
			name: "double pointer type",
			extractFunc: func() string {
				return getTypeNameSimple[**testUser]()
			},
			expectedName: "testUser", // Should dereference fully
		},
		{
			name: "built-in type",
			extractFunc: func() string {
				return getTypeNameSimple[string]()
			},
			expectedName: "string",
		},
		{
			name: "slice type",
			extractFunc: func() string {
				return getTypeNameSimple[[]int]()
			},
			expectedName: "[]int",
		},
		{
			name: "map type",
			extractFunc: func() string {
				return getTypeNameSimple[map[string]int]()
			},
			expectedName: "map[string]int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.extractFunc()
			if result != tt.expectedName {
				t.Errorf("expected %s, got %s", tt.expectedName, result)
			}
		})
	}
}

func TestExtractTypeNameWithPackage(t *testing.T) {
	// Test with package inclusion
	result := getTypeNameWithPackage[testUser]()
	
	// Should include "mtlog" as the immediate package
	expected := "mtlog.testUser"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestExtractTypeNameWithOptions(t *testing.T) {
	tests := []struct {
		name     string
		options  TypeNameOptions
		expected string
	}{
		{
			name: "default options",
			options: TypeNameOptions{
				IncludePackage: false,
			},
			expected: "testUser",
		},
		{
			name: "with package",
			options: TypeNameOptions{
				IncludePackage: true,
				PackageDepth:   1,
			},
			expected: "mtlog.testUser",
		},
		{
			name: "with prefix",
			options: TypeNameOptions{
				IncludePackage: false,
				Prefix:         "MyApp.",
			},
			expected: "MyApp.testUser",
		},
		{
			name: "with suffix",
			options: TypeNameOptions{
				IncludePackage: false,
				Suffix:         ".Service",
			},
			expected: "testUser.Service",
		},
		{
			name: "with prefix and suffix",
			options: TypeNameOptions{
				IncludePackage: false,
				Prefix:         "MyApp.",
				Suffix:         ".Service",
			},
			expected: "MyApp.testUser.Service",
		},
		{
			name: "package with prefix and suffix",
			options: TypeNameOptions{
				IncludePackage: true,
				PackageDepth:   1,
				Prefix:         "App.",
				Suffix:         ".Handler",
			},
			expected: "App.mtlog.testUser.Handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTypeName[testUser](tt.options)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestExtractTypeNamePackageDepth(t *testing.T) {
	// Test package depth limiting with actual package path github.com/willibrandon/mtlog
	
	tests := []struct {
		name        string
		depth       int
		expected    string
		description string
	}{
		{
			name:        "depth 0 (full path)",
			depth:       0,
			expected:    "github.com/willibrandon/mtlog.testUser",
			description: "should include full package path",
		},
		{
			name:        "depth 1 (immediate package)",
			depth:       1,
			expected:    "mtlog.testUser",
			description: "should include only immediate package",
		},
		{
			name:        "depth 2 (two levels)",
			depth:       2,
			expected:    "willibrandon/mtlog.testUser",
			description: "should include last two package levels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := TypeNameOptions{
				IncludePackage: true,
				PackageDepth:   tt.depth,
			}
			
			result := extractTypeName[testUser](options)
			
			if result != tt.expected {
				t.Errorf("expected %s, got %s (depth %d)", tt.expected, result, tt.depth)
			}
		})
	}
}

func TestExtractTypeNameBuiltinTypes(t *testing.T) {
	// Test built-in types that don't have package paths
	tests := []struct {
		name         string
		extractFunc  func() string
		expectedName string
	}{
		{
			name: "int",
			extractFunc: func() string {
				return extractTypeName[int](TypeNameOptions{IncludePackage: true})
			},
			expectedName: "int",
		},
		{
			name: "string",
			extractFunc: func() string {
				return extractTypeName[string](TypeNameOptions{IncludePackage: true})
			},
			expectedName: "string",
		},
		{
			name: "interface{}",
			extractFunc: func() string {
				return extractTypeName[interface{}](TypeNameOptions{IncludePackage: true})
			},
			expectedName: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.extractFunc()
			if result != tt.expectedName {
				t.Errorf("expected %s, got %s", tt.expectedName, result)
			}
		})
	}
}

func TestDefaultTypeNameOptions(t *testing.T) {
	// Clear cache to ensure test isolation
	ResetTypeNameCache()
	
	// Test that default options work as expected
	expected := TypeNameOptions{
		IncludePackage:    false,
		PackageDepth:      1,
		Prefix:            "",
		Suffix:            "",
		SimplifyAnonymous: false,
		WarnOnUnknown:     true,
	}
	
	if DefaultTypeNameOptions.IncludePackage != expected.IncludePackage {
		t.Errorf("expected IncludePackage=%v, got %v", expected.IncludePackage, DefaultTypeNameOptions.IncludePackage)
	}
	
	if DefaultTypeNameOptions.PackageDepth != expected.PackageDepth {
		t.Errorf("expected PackageDepth=%v, got %v", expected.PackageDepth, DefaultTypeNameOptions.PackageDepth)
	}
	
	if DefaultTypeNameOptions.Prefix != expected.Prefix {
		t.Errorf("expected Prefix=%v, got %v", expected.Prefix, DefaultTypeNameOptions.Prefix)
	}
	
	if DefaultTypeNameOptions.Suffix != expected.Suffix {
		t.Errorf("expected Suffix=%v, got %v", expected.Suffix, DefaultTypeNameOptions.Suffix)
	}
	
	if DefaultTypeNameOptions.SimplifyAnonymous != expected.SimplifyAnonymous {
		t.Errorf("expected SimplifyAnonymous=%v, got %v", expected.SimplifyAnonymous, DefaultTypeNameOptions.SimplifyAnonymous)
	}
	
	if DefaultTypeNameOptions.WarnOnUnknown != expected.WarnOnUnknown {
		t.Errorf("expected WarnOnUnknown=%v, got %v", expected.WarnOnUnknown, DefaultTypeNameOptions.WarnOnUnknown)
	}
}

func TestExtractTypeNameEdgeCases(t *testing.T) {
	// Test edge cases like anonymous structs, named interfaces, etc.
	
	// Anonymous struct
	t.Run("anonymous struct", func(t *testing.T) {
		result := extractTypeName[struct{ Name string }](DefaultTypeNameOptions)
		// Anonymous structs have empty names, should fall back to typ.String()
		expected := "struct { Name string }"
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})
	
	// Anonymous struct with complex fields
	t.Run("complex anonymous struct", func(t *testing.T) {
		result := extractTypeName[struct {
			ID    int
			Items []string
			Meta  map[string]interface{}
		}](DefaultTypeNameOptions)
		// Should contain the struct definition
		if !strings.Contains(result, "struct") {
			t.Errorf("expected result to contain 'struct', got %s", result)
		}
	})
	
	// Named interface
	t.Run("named interface", func(t *testing.T) {
		result := extractTypeName[Repository](DefaultTypeNameOptions)
		// Interfaces typically return "Unknown" because they have no concrete type name
		expected := "Unknown"
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})
	
	// Function type
	t.Run("function type", func(t *testing.T) {
		result := extractTypeName[func(string) error](DefaultTypeNameOptions)
		expected := "func(string) error"
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})
	
	// Channel type
	t.Run("channel type", func(t *testing.T) {
		result := extractTypeName[chan string](DefaultTypeNameOptions)
		expected := "chan string"
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	})
}

func TestSimplifyAnonymousOption(t *testing.T) {
	// Clear cache to ensure test isolation
	ResetTypeNameCache()
	
	// Test SimplifyAnonymous option
	tests := []struct {
		name         string
		options      TypeNameOptions
		extractFunc  func(TypeNameOptions) string
		expectedName string
	}{
		{
			name: "anonymous struct with SimplifyAnonymous=false",
			options: TypeNameOptions{
				SimplifyAnonymous: false,
			},
			extractFunc: func(opts TypeNameOptions) string {
				return extractTypeName[struct{ Name string }](opts)
			},
			expectedName: "struct { Name string }",
		},
		{
			name: "anonymous struct with SimplifyAnonymous=true",
			options: TypeNameOptions{
				SimplifyAnonymous: true,
			},
			extractFunc: func(opts TypeNameOptions) string {
				return extractTypeName[struct{ Name string }](opts)
			},
			expectedName: "AnonymousStruct",
		},
		{
			name: "complex anonymous struct with SimplifyAnonymous=true",
			options: TypeNameOptions{
				SimplifyAnonymous: true,
			},
			extractFunc: func(opts TypeNameOptions) string {
				return extractTypeName[struct {
					ID    int
					Items []string
					Meta  map[string]interface{}
				}](opts)
			},
			expectedName: "AnonymousStruct",
		},
		{
			name: "named struct unaffected by SimplifyAnonymous",
			options: TypeNameOptions{
				SimplifyAnonymous: true,
			},
			extractFunc: func(opts TypeNameOptions) string {
				return extractTypeName[testUser](opts)
			},
			expectedName: "testUser",
		},
		{
			name: "SimplifyAnonymous with prefix and suffix",
			options: TypeNameOptions{
				SimplifyAnonymous: true,
				Prefix:            "MyApp.",
				Suffix:            ".Type",
			},
			extractFunc: func(opts TypeNameOptions) string {
				return extractTypeName[struct{ Value int }](opts)
			},
			expectedName: "MyApp.AnonymousStruct.Type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.extractFunc(tt.options)
			if result != tt.expectedName {
				t.Errorf("expected %s, got %s", tt.expectedName, result)
			}
		})
	}
}

func TestWarnOnUnknownOption(t *testing.T) {
	// Clear cache to ensure test isolation
	ResetTypeNameCache()
	
	// Test WarnOnUnknown option
	tests := []struct {
		name        string
		options     TypeNameOptions
		expectWarn  bool
		description string
	}{
		{
			name: "warnings enabled by default",
			options: DefaultTypeNameOptions,
			expectWarn: true,
			description: "should warn with default options",
		},
		{
			name: "warnings disabled",
			options: TypeNameOptions{
				WarnOnUnknown: false,
			},
			expectWarn: false,
			description: "should not warn when disabled",
		},
		{
			name: "warnings explicitly enabled",
			options: TypeNameOptions{
				WarnOnUnknown: true,
			},
			expectWarn: true,
			description: "should warn when explicitly enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output to test warning behavior
			var logBuffer strings.Builder
			originalOutput := log.Writer()
			log.SetOutput(&logBuffer)
			defer log.SetOutput(originalOutput)
			
			// Test with interface type that results in "Unknown"
			result := extractTypeName[interface{}](tt.options)
			
			// Verify the result
			if result != "Unknown" {
				t.Errorf("expected 'Unknown' for interface{} type, got %s", result)
			}
			
			// Check if warning was logged
			logOutput := logBuffer.String()
			containsWarning := strings.Contains(logOutput, "[mtlog] Warning:")
			
			if tt.expectWarn && !containsWarning {
				t.Errorf("expected warning to be logged, but no warning found in output: %s", logOutput)
			}
			
			if !tt.expectWarn && containsWarning {
				t.Errorf("expected no warning, but warning was logged: %s", logOutput)
			}
		})
	}
}

func TestTypeNameCacheStats(t *testing.T) {
	// Clear cache to start fresh
	ResetTypeNameCache()
	
	// Check initial stats
	stats := GetTypeNameCacheStats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Evictions != 0 || stats.HitRatio != 0 || stats.Size != 0 {
		t.Errorf("expected clean stats, got hits=%d, misses=%d, evictions=%d, ratio=%.1f, size=%d", stats.Hits, stats.Misses, stats.Evictions, stats.HitRatio, stats.Size)
	}
	if stats.MaxSize <= 0 {
		t.Errorf("expected positive MaxSize, got %d", stats.MaxSize)
	}
	
	// First call should be a miss
	_ = extractTypeName[testUser](DefaultTypeNameOptions)
	stats = GetTypeNameCacheStats()
	if stats.Hits != 0 || stats.Misses != 1 {
		t.Errorf("expected 0 hits, 1 miss, got hits=%d, misses=%d", stats.Hits, stats.Misses)
	}
	if stats.HitRatio != 0 {
		t.Errorf("expected 0%% hit ratio, got %.1f%%", stats.HitRatio)
	}
	if stats.Size != 1 {
		t.Errorf("expected cache size=1, got %d", stats.Size)
	}
	
	// Second call should be a hit
	_ = extractTypeName[testUser](DefaultTypeNameOptions)
	stats = GetTypeNameCacheStats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Errorf("expected 1 hit, 1 miss, got hits=%d, misses=%d", stats.Hits, stats.Misses)
	}
	if stats.HitRatio != 50.0 {
		t.Errorf("expected 50%% hit ratio, got %.1f%%", stats.HitRatio)
	}
	if stats.Size != 1 {
		t.Errorf("expected cache size=1, got %d", stats.Size)
	}
	
	// Third call should be another hit
	_ = extractTypeName[testUser](DefaultTypeNameOptions)
	stats = GetTypeNameCacheStats()
	if stats.Hits != 2 || stats.Misses != 1 {
		t.Errorf("expected 2 hits, 1 miss, got hits=%d, misses=%d", stats.Hits, stats.Misses)
	}
	if stats.HitRatio < 66.0 || stats.HitRatio > 67.0 {
		t.Errorf("expected ~66.7%% hit ratio, got %.1f%%", stats.HitRatio)
	}
	if stats.Size != 1 {
		t.Errorf("expected cache size=1, got %d", stats.Size)
	}
	
	// Different type should be a miss
	_ = extractTypeName[testProduct](DefaultTypeNameOptions)
	stats = GetTypeNameCacheStats()
	if stats.Hits != 2 || stats.Misses != 2 {
		t.Errorf("expected 2 hits, 2 misses, got hits=%d, misses=%d", stats.Hits, stats.Misses)
	}
	if stats.HitRatio != 50.0 {
		t.Errorf("expected 50%% hit ratio, got %.1f%%", stats.HitRatio)
	}
	if stats.Size != 2 {
		t.Errorf("expected cache size=2, got %d", stats.Size)
	}
	
	// Reset should clear stats
	ResetTypeNameCache()
	stats = GetTypeNameCacheStats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Evictions != 0 || stats.HitRatio != 0 || stats.Size != 0 {
		t.Errorf("expected clean stats after reset, got hits=%d, misses=%d, evictions=%d, ratio=%.1f, size=%d", stats.Hits, stats.Misses, stats.Evictions, stats.HitRatio, stats.Size)
	}
}

// Generic types for testing
type GenericType[T any] struct {
	Value T
}

type GenericMap[K comparable, V any] map[K]V

func TestExtractTypeNameGenericTypes(t *testing.T) {
	// Clear cache to ensure test isolation
	ResetTypeNameCache()
	
	// Test generic types
	tests := []struct {
		name         string
		extractFunc  func() string
		expectedName string
	}{
		{
			name: "generic struct with string",
			extractFunc: func() string {
				return getTypeNameSimple[GenericType[string]]()
			},
			expectedName: "GenericType[string]",
		},
		{
			name: "generic struct with int",
			extractFunc: func() string {
				return getTypeNameSimple[GenericType[int]]()
			},
			expectedName: "GenericType[int]",
		},
		{
			name: "generic map",
			extractFunc: func() string {
				return getTypeNameSimple[GenericMap[string, int]]()
			},
			expectedName: "GenericMap[string,int]",
		},
		{
			name: "nested generic type",
			extractFunc: func() string {
				return getTypeNameSimple[GenericType[GenericType[int]]]()
			},
			expectedName: "GenericType[GenericType[int]]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.extractFunc()
			if result != tt.expectedName {
				t.Errorf("expected %s, got %s", tt.expectedName, result)
			}
		})
	}
}

func TestExtractTypeNameComplexGenericEdgeCases(t *testing.T) {
	// Clear cache to ensure test isolation
	ResetTypeNameCache()
	
	// Test complex generic edge cases that might occur in real applications
	tests := []struct {
		name         string
		extractFunc  func() string
		expectedName string
		description  string
	}{
		{
			name: "deeply nested generics",
			extractFunc: func() string {
				return getTypeNameSimple[GenericType[GenericType[GenericType[string]]]]()
			},
			expectedName: "GenericType[GenericType[GenericType[string]]]",
			description:  "should handle deeply nested generic types",
		},
		{
			name: "generic with multiple type parameters",
			extractFunc: func() string {
				return getTypeNameSimple[GenericMap[string, GenericType[int]]]()
			},
			expectedName: "GenericMap[string,GenericType[int]]",
			description:  "should handle multiple type parameters with nested generics",
		},
		{
			name: "generic with built-in types",
			extractFunc: func() string {
				return getTypeNameSimple[GenericMap[string, []map[int]interface{}]]()
			},
			expectedName: "GenericMap[string,[]map[int]interface {}]",
			description:  "should handle complex built-in type combinations",
		},
		{
			name: "pointer to generic type",
			extractFunc: func() string {
				return getTypeNameSimple[*GenericType[*GenericMap[string, int]]]()
			},
			expectedName: "GenericType[*GenericMap[string,int]]",
			description:  "should dereference outer pointer but preserve inner pointer types",
		},
		{
			name: "slice of generic pointers",
			extractFunc: func() string {
				return getTypeNameSimple[[]*GenericType[string]]()
			},
			expectedName: "[]*mtlog.GenericType[string]",
			description:  "should handle slice of generic pointer types",
		},
		{
			name: "map with generic keys and values",
			extractFunc: func() string {
				return getTypeNameSimple[map[GenericType[string]]GenericType[int]]()
			},
			expectedName: "map[mtlog.GenericType[string]]mtlog.GenericType[int]",
			description:  "should handle maps with generic key and value types",
		},
		{
			name: "channel of generic type",
			extractFunc: func() string {
				return getTypeNameSimple[chan GenericType[string]]()
			},
			expectedName: "chan mtlog.GenericType[string]",
			description:  "should handle channels of generic types",
		},
		{
			name: "function with generic parameters",
			extractFunc: func() string {
				return getTypeNameSimple[func(GenericType[string]) GenericType[int]]()
			},
			expectedName: "func(mtlog.GenericType[string]) mtlog.GenericType[int]",
			description:  "should handle function types with generic parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.extractFunc()
			if result != tt.expectedName {
				t.Errorf("%s: expected %s, got %s", tt.description, tt.expectedName, result)
			}
		})
	}
}

func TestExtractTypeNameConcurrency(t *testing.T) {
	// Clear cache to ensure test isolation
	ResetTypeNameCache()
	
	// Test concurrent usage to ensure thread safety
	const numGoroutines = 10
	const numIterations = 100
	
	t.Run("getTypeNameSimple", func(t *testing.T) {
		var wg sync.WaitGroup
		results := make(chan string, numGoroutines*numIterations)
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numIterations; j++ {
					result := getTypeNameSimple[testUser]()
					results <- result
				}
			}()
		}
		
		wg.Wait()
		close(results)
		
		// Verify all results are consistent
		expected := "testUser"
		count := 0
		for result := range results {
			count++
			if result != expected {
				t.Errorf("expected %s, got %s", expected, result)
			}
		}
		
		if count != numGoroutines*numIterations {
			t.Errorf("expected %d results, got %d", numGoroutines*numIterations, count)
		}
	})
	
	t.Run("extractTypeName with TypeNameOptions", func(t *testing.T) {
		// Test different TypeNameOptions configurations concurrently
		testCases := []struct {
			name     string
			options  TypeNameOptions
			expected string
		}{
			{
				name:     "default",
				options:  DefaultTypeNameOptions,
				expected: "testUser",
			},
			{
				name: "with package",
				options: TypeNameOptions{
					IncludePackage: true,
					PackageDepth:   1,
				},
				expected: "mtlog.testUser",
			},
			{
				name: "with prefix",
				options: TypeNameOptions{
					Prefix: "MyApp.",
				},
				expected: "MyApp.testUser",
			},
			{
				name: "with suffix",
				options: TypeNameOptions{
					Suffix: ".Handler",
				},
				expected: "testUser.Handler",
			},
			{
				name: "complex options",
				options: TypeNameOptions{
					IncludePackage: true,
					PackageDepth:   1,
					Prefix:         "App.",
					Suffix:         ".Service",
				},
				expected: "App.mtlog.testUser.Service",
			},
		}
		
		for _, tc := range testCases {
			tc := tc // capture loop variable
			t.Run(tc.name, func(t *testing.T) {
				var wg sync.WaitGroup
				results := make(chan string, numGoroutines*numIterations)
				
				for i := 0; i < numGoroutines; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						for j := 0; j < numIterations; j++ {
							result := extractTypeName[testUser](tc.options)
							results <- result
						}
					}()
				}
				
				wg.Wait()
				close(results)
				
				// Verify all results are consistent
				count := 0
				for result := range results {
					count++
					if result != tc.expected {
						t.Errorf("expected %s, got %s", tc.expected, result)
					}
				}
				
				if count != numGoroutines*numIterations {
					t.Errorf("expected %d results, got %d", numGoroutines*numIterations, count)
				}
			})
		}
	})
}

func TestTypeNameCacheLRUEviction(t *testing.T) {
	// Clear cache to start fresh
	ResetTypeNameCache()
	
	// Test LRU eviction with a small cache size
	// We'll simulate a small cache by setting a very small limit
	originalConfig := cacheConfig
	defer func() { cacheConfig = originalConfig }()
	
	cacheConfig = typeNameCacheConfig{
		maxSize: 2, // Very small cache for testing
		enabled: true,
	}
	
	// Add first type
	name1 := extractTypeName[testUser](DefaultTypeNameOptions)
	if name1 != "testUser" {
		t.Errorf("expected testUser, got %s", name1)
	}
	
	// Add second type
	name2 := extractTypeName[testProduct](DefaultTypeNameOptions)
	if name2 != "testProduct" {
		t.Errorf("expected testProduct, got %s", name2)
	}
	
	// Wait a bit for async eviction to complete
	time.Sleep(10 * time.Millisecond)
	
	// Add third type - this should trigger eviction of the least recently used
	name3 := extractTypeName[GenericType[string]](DefaultTypeNameOptions)
	if name3 != "GenericType[string]" {
		t.Errorf("expected GenericType[string], got %s", name3)
	}
	
	// Wait for eviction to complete
	time.Sleep(10 * time.Millisecond)
	
	// Check that we have evictions recorded
	stats := GetTypeNameCacheStats()
	if stats.Size > 2 {
		t.Errorf("expected cache size <= 2 after eviction, got %d", stats.Size)
	}
	if stats.MaxSize != 2 {
		t.Errorf("expected MaxSize=2, got %d", stats.MaxSize)
	}
}

func TestExtractTypeNameWithCacheKey(t *testing.T) {
	// Clear cache to start fresh
	ResetTypeNameCache()
	
	// Test custom cache keys for multi-tenant scenarios
	tenant1Prefix := "tenant:acme"
	tenant2Prefix := "tenant:globex"
	
	// Extract type names with different cache keys
	name1 := ExtractTypeNameWithCacheKey[testUser](DefaultTypeNameOptions, tenant1Prefix)
	name2 := ExtractTypeNameWithCacheKey[testUser](DefaultTypeNameOptions, tenant2Prefix)
	
	// Both should return the same type name
	if name1 != "testUser" || name2 != "testUser" {
		t.Errorf("expected both to be testUser, got %s and %s", name1, name2)
	}
	
	// Check that cache has separate entries for different tenants
	stats := GetTypeNameCacheStats()
	if stats.Size < 2 {
		t.Errorf("expected at least 2 cache entries for different tenants, got %d", stats.Size)
	}
	
	// Test cache hits with tenant-specific keys
	initialStats := GetTypeNameCacheStats()
	
	// This should be a cache hit for tenant1
	name1Again := ExtractTypeNameWithCacheKey[testUser](DefaultTypeNameOptions, tenant1Prefix)
	if name1Again != "testUser" {
		t.Errorf("expected testUser, got %s", name1Again)
	}
	
	finalStats := GetTypeNameCacheStats()
	if finalStats.Hits <= initialStats.Hits {
		t.Errorf("expected cache hit for tenant-specific key, hits did not increase")
	}
}

func TestForTypeWithCacheKey(t *testing.T) {
	// Clear cache to start fresh
	ResetTypeNameCache()
	
	// Create a memory sink to capture events
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Test ForType with custom cache key
	tenantPrefix := "tenant:acme"
	userLogger := ForTypeWithCacheKey[testUser](logger, tenantPrefix)
	userLogger.Information("User operation for tenant")
	
	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	
	event := events[0]
	if sourceContext, ok := event.Properties["SourceContext"]; !ok || sourceContext != "testUser" {
		t.Errorf("expected SourceContext=testUser, got %v", sourceContext)
	}
	
	// Verify that cache has an entry
	stats := GetTypeNameCacheStats()
	if stats.Size < 1 {
		t.Errorf("expected at least 1 cache entry, got %d", stats.Size)
	}
}
