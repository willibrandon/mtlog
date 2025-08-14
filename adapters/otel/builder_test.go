package otel_test

import (
	"context"
	"os"
	"testing"
	"time"

	mtlogotel "github.com/willibrandon/mtlog/adapters/otel"
	"github.com/willibrandon/mtlog/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestBuilder(t *testing.T) {
	// Setup tracer for testing
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	tracer := otel.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()

	t.Run("BasicBuilder", func(t *testing.T) {
		builder := mtlogotel.NewBuilder(ctx)
		logger, err := builder.
			WithConsole().
			WithMinimumLevel(core.DebugLevel).
			Build()

		if err != nil {
			t.Fatalf("Failed to build logger: %v", err)
		}

		if logger == nil {
			t.Fatal("Expected logger, got nil")
		}

		// Test that it can log
		logger.Debug("Test message")
		logger.Information("Test with {Property}", "value")
	})

	t.Run("BuilderWithEndpoint", func(t *testing.T) {
		builder := mtlogotel.NewBuilder(ctx)
		logger, err := builder.
			WithEndpoint("localhost:4317").
			WithGRPC().
			WithBatching(50, 2*time.Second).
			WithConsole().
			Build()

		if err != nil {
			t.Fatalf("Failed to build logger: %v", err)
		}

		if logger == nil {
			t.Fatal("Expected logger, got nil")
		}
	})

	t.Run("BuilderWithStaticEnricher", func(t *testing.T) {
		builder := mtlogotel.NewBuilder(ctx)
		logger, err := builder.
			WithStaticEnricher().
			WithConsole().
			Build()

		if err != nil {
			t.Fatalf("Failed to build logger: %v", err)
		}

		logger.Information("Test with static enricher")
	})

	t.Run("BuilderWithEnvironment", func(t *testing.T) {
		// Set environment variable
		os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "test-endpoint:4317")
		defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

		builder := mtlogotel.NewBuilder(ctx)
		logger, err := builder.
			WithEndpointFromEnv().
			WithConsole().
			Build()

		if err != nil {
			t.Fatalf("Failed to build logger: %v", err)
		}

		if logger == nil {
			t.Fatal("Expected logger, got nil")
		}
	})

	t.Run("MustBuild", func(t *testing.T) {
		builder := mtlogotel.NewBuilder(ctx)
		
		// Should not panic with valid config
		logger := builder.
			WithConsole().
			MustBuild()

		if logger == nil {
			t.Fatal("Expected logger, got nil")
		}
	})
}

func TestConvenienceFunctions(t *testing.T) {
	// Setup tracer
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	tracer := otel.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()

	t.Run("NewOTELLogger", func(t *testing.T) {
		// Clear any existing env var
		os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

		logger := mtlogotel.NewOTELLogger(ctx)
		if logger == nil {
			t.Fatal("Expected logger, got nil")
		}

		logger.Information("Test OTEL logger")
	})

	t.Run("NewOTELLoggerWithEnv", func(t *testing.T) {
		os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "test:4317")
		defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

		logger := mtlogotel.NewOTELLogger(ctx)
		if logger == nil {
			t.Fatal("Expected logger, got nil")
		}

		logger.Information("Test with environment endpoint")
	})

	t.Run("NewRequestLogger", func(t *testing.T) {
		requestID := "req-123"
		logger := mtlogotel.NewRequestLogger(ctx, requestID)
		
		if logger == nil {
			t.Fatal("Expected logger, got nil")
		}

		logger.Information("Request processing")
	})

	t.Run("NewRequestLoggerWithOTLP", func(t *testing.T) {
		os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
		defer os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

		logger := mtlogotel.NewRequestLogger(ctx, "req-456")
		if logger == nil {
			t.Fatal("Expected logger, got nil")
		}

		logger.Information("Request with OTLP")
	})
}

func TestBuilderChaining(t *testing.T) {
	ctx := context.Background()

	// Test that all methods return the builder for chaining
	builder := mtlogotel.NewBuilder(ctx)
	
	// This should compile and work
	result := builder.
		WithEndpoint("localhost:4317").
		WithGRPC().
		WithHTTP(). // Switch transport
		WithFastEnricher().
		WithStaticEnricher(). // Switch enricher
		WithCachingEnricher(). // Switch again
		WithBatching(100, 5*time.Second).
		WithConsole().
		WithMinimumLevel(core.VerboseLevel).
		WithHeaders(map[string]string{"key": "value"})

	if result != builder {
		t.Error("Expected builder methods to return self for chaining")
	}

	// Should be able to build after all that
	logger, err := result.Build()
	if err != nil {
		t.Fatalf("Failed to build after chaining: %v", err)
	}

	if logger == nil {
		t.Fatal("Expected logger after chaining")
	}
}

func BenchmarkBuilderCreation(b *testing.B) {
	ctx := context.Background()

	b.Run("NewBuilder", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			builder := mtlogotel.NewBuilder(ctx)
			_ = builder
		}
	})

	b.Run("BuildMinimal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger, _ := mtlogotel.NewBuilder(ctx).Build()
			_ = logger
		}
	})

	b.Run("BuildWithOptions", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger, _ := mtlogotel.NewBuilder(ctx).
				WithConsole().
				WithStaticEnricher().
				WithMinimumLevel(core.DebugLevel).
				Build()
			_ = logger
		}
	})

	b.Run("NewOTELLogger", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger := mtlogotel.NewOTELLogger(ctx)
			_ = logger
		}
	})

	b.Run("NewRequestLogger", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			logger := mtlogotel.NewRequestLogger(ctx, "req-id")
			_ = logger
		}
	})
}