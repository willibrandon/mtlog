package mtlog

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// BenchmarkSampleEveryNth benchmarks the Sample method overhead
func BenchmarkSampleEveryNth(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))
	sampledLogger := logger.Sample(10) // Sample every 10th message

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sampledLogger.Info("Benchmark message {Number}", i)
	}
}

// BenchmarkSampleRate benchmarks the SampleRate method overhead
func BenchmarkSampleRate(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))
	sampledLogger := logger.SampleRate(0.1) // Sample 10% of messages

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sampledLogger.Info("Benchmark message {Number}", i)
	}
}

// BenchmarkSampleDuration benchmarks the SampleDuration method overhead
func BenchmarkSampleDuration(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))
	sampledLogger := logger.SampleDuration(time.Millisecond) // Sample at most once per millisecond

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sampledLogger.Info("Benchmark message {Number}", i)
	}
}

// BenchmarkSampleFirst benchmarks the SampleFirst method overhead
func BenchmarkSampleFirst(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))
	sampledLogger := logger.SampleFirst(100) // Log only first 100

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sampledLogger.Info("Benchmark message {Number}", i)
	}
}

// BenchmarkSampleGroup benchmarks the SampleGroup method overhead
func BenchmarkSampleGroup(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))
	sampledLogger := logger.SampleGroup("bench-group", 10)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sampledLogger.Info("Benchmark message {Number}", i)
	}
}

// BenchmarkSampleWhen benchmarks the SampleWhen method overhead
func BenchmarkSampleWhen(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))
	
	var condition atomic.Bool
	condition.Store(true)
	
	sampledLogger := logger.SampleWhen(func() bool {
		return condition.Load()
	}, 10)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sampledLogger.Info("Benchmark message {Number}", i)
	}
}

// BenchmarkSampleBackoff benchmarks the SampleBackoff method overhead
func BenchmarkSampleBackoff(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))
	sampledLogger := logger.SampleBackoff("bench-key", 2.0)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sampledLogger.Info("Benchmark message {Number}", i)
	}
}

// BenchmarkNoSampling provides a baseline without sampling
func BenchmarkNoSampling(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message {Number}", i)
	}
}

// BenchmarkSamplingOverhead measures just the sampling decision overhead
func BenchmarkSamplingOverhead(b *testing.B) {
	benchmarks := []struct {
		name string
		fn   func() bool
	}{
		{
			name: "Counter",
			fn: func() bool {
				var counter atomic.Uint64
				n := uint64(10)
				count := counter.Add(1)
				return count%n == 1
			},
		},
		{
			name: "Rate",
			fn: func() bool {
				var counter atomic.Uint32
				rate := float32(0.1)
				count := counter.Add(1)
				threshold := uint32(1.0 / rate)
				return count%threshold == 1
			},
		},
		{
			name: "Duration",
			fn: func() bool {
				var lastSample atomic.Int64
				duration := time.Millisecond
				now := time.Now().UnixNano()
				last := lastSample.Load()
				if now-last >= int64(duration) {
					return lastSample.CompareAndSwap(last, now)
				}
				return false
			},
		},
		{
			name: "First",
			fn: func() bool {
				var counter atomic.Uint64
				n := uint64(100)
				count := counter.Add(1)
				return count <= n
			},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = bm.fn()
			}
		})
	}
}

// BenchmarkConcurrentSampling benchmarks sampling under concurrent load
func BenchmarkConcurrentSampling(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))
	sampledLogger := logger.Sample(10)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sampledLogger.Info("Concurrent message {Number}", i)
			i++
		}
	})
}

// BenchmarkChainedSampling benchmarks chained sampling filters
func BenchmarkChainedSampling(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))
	
	// Chain multiple sampling methods
	sampledLogger := logger.Sample(2).Sample(3).Sample(5)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sampledLogger.Info("Benchmark message {Number}", i)
	}
}

// BenchmarkMemoryUsage measures memory usage of sampling state
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("BaseLogger", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sink := sinks.NewMemorySink()
			_ = New(WithSink(sink))
		}
	})

	b.Run("WithSampling", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sink := sinks.NewMemorySink()
			logger := New(WithSink(sink))
			_ = logger.Sample(10)
		}
	})

	b.Run("WithMultipleSampling", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sink := sinks.NewMemorySink()
			logger := New(WithSink(sink))
			_ = logger.Sample(10)
			_ = logger.SampleRate(0.5)
			_ = logger.SampleDuration(time.Second)
		}
	})
}

// BenchmarkGroupSamplingShared benchmarks shared group counter performance
func BenchmarkGroupSamplingShared(b *testing.B) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink), WithMinimumLevel(core.VerboseLevel))
	
	// Create multiple loggers sharing the same group
	loggers := make([]core.Logger, 10)
	for i := 0; i < 10; i++ {
		loggers[i] = logger.SampleGroup("shared-bench", 10)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Use different loggers but same group
			logger := loggers[i%len(loggers)]
			logger.Info("Shared group message {Number}", i)
			i++
		}
	})
}

// Comparison benchmarks to show overhead vs no filtering

// BenchmarkComparisonTable provides a comparison table
func BenchmarkComparisonTable(b *testing.B) {
	scenarios := []struct {
		name  string
		setup func() core.Logger
	}{
		{
			name: "Baseline_NoFilter",
			setup: func() core.Logger {
				return New(WithSink(sinks.NewMemorySink()))
			},
		},
		{
			name: "Sample_Every10",
			setup: func() core.Logger {
				return New(WithSink(sinks.NewMemorySink())).Sample(10)
			},
		},
		{
			name: "Sample_Every100",
			setup: func() core.Logger {
				return New(WithSink(sinks.NewMemorySink())).Sample(100)
			},
		},
		{
			name: "SampleRate_10Percent",
			setup: func() core.Logger {
				return New(WithSink(sinks.NewMemorySink())).SampleRate(0.1)
			},
		},
		{
			name: "SampleRate_1Percent",
			setup: func() core.Logger {
				return New(WithSink(sinks.NewMemorySink())).SampleRate(0.01)
			},
		},
		{
			name: "SampleDuration_1ms",
			setup: func() core.Logger {
				return New(WithSink(sinks.NewMemorySink())).SampleDuration(time.Millisecond)
			},
		},
		{
			name: "SampleFirst_1000",
			setup: func() core.Logger {
				return New(WithSink(sinks.NewMemorySink())).SampleFirst(1000)
			},
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			logger := scenario.setup()
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				logger.Info("Comparison message {Number}", i)
			}
		})
	}
}