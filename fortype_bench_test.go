package mtlog_test

import (
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/sinks"
)

// Benchmark types
type BenchUser struct {
	ID   int
	Name string
}

type BenchProduct struct {
	SKU   string
	Price float64
}

func BenchmarkForType(b *testing.B) {
	// Use memory sink for benchmarking
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	b.Run("ForType[User]", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[BenchUser](logger).Information("User operation")
		}
	})

	b.Run("ForSourceContext", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			logger.ForSourceContext("BenchUser").Information("User operation")
		}
	})

	b.Run("ForType[*User]", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[*BenchUser](logger).Information("User pointer operation")
		}
	})

	b.Run("ForType[Product]", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[BenchProduct](logger).Information("Product operation")
		}
	})
}

func BenchmarkForTypeChaining(b *testing.B) {
	// Use memory sink for benchmarking
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	b.Run("ForType+ForContext", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[BenchUser](logger).ForContext("Operation", "Create").Information("Chained operation")
		}
	})

	b.Run("ForSourceContext+ForContext", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			logger.ForSourceContext("BenchUser").ForContext("Operation", "Create").Information("Chained operation")
		}
	})

	b.Run("MultipleForContext", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[BenchUser](logger).
				ForContext("Operation", "Create").
				ForContext("RequestId", "req-123").
				Information("Multiple chain operation")
		}
	})
}

func BenchmarkTypeNameExtraction(b *testing.B) {
	b.Run("getTypeNameSimple[User]", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = mtlog.GetTypeNameSimple[BenchUser]()
		}
	})

	b.Run("getTypeNameSimple[*User]", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = mtlog.GetTypeNameSimple[*BenchUser]()
		}
	})

	b.Run("getTypeNameWithPackage[User]", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = mtlog.GetTypeNameWithPackage[BenchUser]()
		}
	})
}

func BenchmarkForTypeVsManual(b *testing.B) {
	// Compare ForType vs manual string SourceContext
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	b.Run("ForType_Automatic", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[BenchUser](logger).Information("Test message")
		}
	})

	b.Run("Manual_SourceContext", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			logger.ForSourceContext("BenchUser").Information("Test message")
		}
	})

	b.Run("Manual_ForContext", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			logger.ForContext("SourceContext", "BenchUser").Information("Test message")
		}
	})
}

// Complex types for benchmarking reflection overhead
type ComplexStruct struct {
	ID       int
	Name     string
	Tags     []string
	Metadata map[string]any
	Children []ComplexStruct
}

type BenchGeneric[T any] struct {
	Value T
	Count int
}

func BenchmarkComplexTypes(b *testing.B) {
	// Benchmark ForType with complex and generic types to measure reflection overhead
	sink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(sink))

	b.Run("ComplexStruct", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[ComplexStruct](logger).Information("Complex struct operation")
		}
	})

	b.Run("GenericType[string]", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[BenchGeneric[string]](logger).Information("Generic type operation")
		}
	})

	b.Run("GenericType[ComplexStruct]", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[BenchGeneric[ComplexStruct]](logger).Information("Nested generic operation")
		}
	})

	b.Run("NestedGeneric", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[BenchGeneric[BenchGeneric[int]]](logger).Information("Nested generic operation")
		}
	})

	b.Run("AnonymousStruct", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			mtlog.ForType[struct {
				ID   int
				Data []string
			}](logger).Information("Anonymous struct operation")
		}
	})
}

func BenchmarkExtractTypeNameWithOptions(b *testing.B) {
	// Benchmarks for ExtractTypeName and ForType show actual performance characteristics:
	// - Uncached calls: ~213 ns/op with 4 allocs (reflection + cache key creation)
	// - Cached calls: ~148 ns/op with 3 allocs (cache entry access + lastUsed update)
	// - Cache provides ~1.4x speedup with LRU tracking overhead
	// These benchmarks measure the performance impact of different TypeNameOptions configurations.

	// Benchmark ExtractTypeName with different TypeNameOptions configurations
	tests := []struct {
		name    string
		options mtlog.TypeNameOptions
	}{
		{
			name:    "DefaultOptions",
			options: mtlog.DefaultTypeNameOptions,
		},
		{
			name: "WithPackage",
			options: mtlog.TypeNameOptions{
				IncludePackage: true,
				PackageDepth:   1,
			},
		},
		{
			name: "WithPrefix",
			options: mtlog.TypeNameOptions{
				Prefix: "MyApp.",
			},
		},
		{
			name: "WithSuffix",
			options: mtlog.TypeNameOptions{
				Suffix: ".Service",
			},
		},
		{
			name: "ComplexOptions",
			options: mtlog.TypeNameOptions{
				IncludePackage: true,
				PackageDepth:   2,
				Prefix:         "API.",
				Suffix:         ".Handler",
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name+"_BenchUser", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = mtlog.ExtractTypeName[BenchUser](tt.options)
			}
		})

		b.Run(tt.name+"_BenchGeneric[string]", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = mtlog.ExtractTypeName[BenchGeneric[string]](tt.options)
			}
		})

		b.Run(tt.name+"_ComplexStruct", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				_ = mtlog.ExtractTypeName[ComplexStruct](tt.options)
			}
		})
	}
}

func BenchmarkCachedVsUncached(b *testing.B) {
	// Benchmark cached vs uncached type name extraction to measure cache effectiveness

	// Clear cache before benchmark (access through the global variable)
	mtlog.ResetTypeNameCache()

	b.Run("FirstCall_Uncached", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			// Clear cache each iteration to simulate uncached calls
			mtlog.ResetTypeNameCache()
			_ = mtlog.ExtractTypeName[BenchUser](mtlog.DefaultTypeNameOptions)
		}
	})

	b.Run("SubsequentCalls_Cached", func(b *testing.B) {
		// Prime the cache with one call
		_ = mtlog.ExtractTypeName[BenchUser](mtlog.DefaultTypeNameOptions)

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_ = mtlog.ExtractTypeName[BenchUser](mtlog.DefaultTypeNameOptions)
		}
	})

	b.Run("MixedTypes_CacheHitMiss", func(b *testing.B) {
		// Mix of cached and uncached calls to simulate real usage
		types := []func() string{
			func() string { return mtlog.ExtractTypeName[BenchUser](mtlog.DefaultTypeNameOptions) },
			func() string { return mtlog.ExtractTypeName[BenchProduct](mtlog.DefaultTypeNameOptions) },
			func() string { return mtlog.ExtractTypeName[ComplexStruct](mtlog.DefaultTypeNameOptions) },
			func() string { return mtlog.ExtractTypeName[BenchGeneric[string]](mtlog.DefaultTypeNameOptions) },
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; b.Loop(); i++ {
			_ = types[i%len(types)]()
		}
	})
}
