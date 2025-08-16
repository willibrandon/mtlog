package mtlog

import (
	"testing"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func BenchmarkWith(b *testing.B) {
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithMinimumLevel(core.InformationLevel),
	)

	b.Run("NoFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.With().Info("test message")
		}
	})

	b.Run("TwoFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.With("key1", "value1", "key2", 42).Info("test message")
		}
	})

	b.Run("FourFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.With(
				"key1", "value1",
				"key2", 42,
				"key3", true,
				"key4", 3.14,
			).Info("test message")
		}
	})

	b.Run("EightFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.With(
				"key1", "value1",
				"key2", 42,
				"key3", true,
				"key4", 3.14,
				"key5", "value5",
				"key6", 100,
				"key7", false,
				"key8", 2.71,
			).Info("test message")
		}
	})

	b.Run("TenFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.With(
				"key1", "value1",
				"key2", 42,
				"key3", true,
				"key4", 3.14,
				"key5", "value5",
				"key6", 100,
				"key7", false,
				"key8", 2.71,
				"key9", "value9",
				"key10", 999,
			).Info("test message")
		}
	})

	b.Run("ChainedWith", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.
				With("service", "api").
				With("version", "1.0").
				With("environment", "production").
				Info("test message")
		}
	})
}

func BenchmarkWithVsForContext(b *testing.B) {
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithMinimumLevel(core.InformationLevel),
	)

	b.Run("With", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.With("user_id", 123, "request_id", "abc").Info("test message")
		}
	})

	b.Run("ForContext", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.
				ForContext("user_id", 123).
				ForContext("request_id", "abc").
				Info("test message")
		}
	})
}

func BenchmarkWithCreation(b *testing.B) {
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithMinimumLevel(core.InformationLevel),
	)

	b.Run("CreateLoggerNoFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = logger.With()
		}
	})

	b.Run("CreateLoggerTwoFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = logger.With("key1", "value1", "key2", 42)
		}
	})

	b.Run("CreateLoggerEightFields", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = logger.With(
				"key1", "value1",
				"key2", 42,
				"key3", true,
				"key4", 3.14,
				"key5", "value5",
				"key6", 100,
				"key7", false,
				"key8", 2.71,
			)
		}
	})
}

func BenchmarkWithReuse(b *testing.B) {
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithMinimumLevel(core.InformationLevel),
	)

	// Create a logger with common fields once
	commonLogger := logger.With("service", "api", "version", "1.0")

	b.Run("ReuseCommonLogger", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			commonLogger.With("request_id", i).Info("test message")
		}
	})

	b.Run("RecreateEachTime", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			logger.With("service", "api", "version", "1.0", "request_id", i).Info("test message")
		}
	})
}