//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	otelmtlog "github.com/willibrandon/mtlog/adapters/otel"
	"github.com/willibrandon/mtlog/core"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestOTLPIntegration(t *testing.T) {
	collectorEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if collectorEndpoint == "" {
		collectorEndpoint = "localhost:4317"
	}

	// Check if collector is available
	if !isCollectorAvailable(collectorEndpoint) {
		t.Skip("OTEL Collector is not available, skipping integration test")
	}

	// Create a unique test ID
	testID := fmt.Sprintf("test-%d", time.Now().UnixNano())

	// Test both gRPC and HTTP transports
	t.Run("gRPC", func(t *testing.T) {
		testOTLPTransport(t, collectorEndpoint, otelmtlog.OTLPTransportGRPC, testID)
	})

	t.Run("HTTP", func(t *testing.T) {
		httpEndpoint := "localhost:4318"
		if envEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_HTTP_ENDPOINT"); envEndpoint != "" {
			httpEndpoint = envEndpoint
		}
		testOTLPTransport(t, httpEndpoint, otelmtlog.OTLPTransportHTTP, testID)
	})
}

func testOTLPTransport(t *testing.T, endpoint string, transport otelmtlog.OTLPTransport, testID string) {
	// Create OTLP sink
	sink, err := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint(endpoint),
		otelmtlog.WithOTLPTransport(transport),
		otelmtlog.WithOTLPBatching(10, 1*time.Second),
		otelmtlog.WithOTLPTimeout(30*time.Second),
		otelmtlog.WithOTLPCompression("gzip"),
	)
	if err != nil {
		t.Fatalf("Failed to create OTLP sink: %v", err)
	}
	defer sink.Close()

	// Create logger with OTLP sink
	// Convert transport to string
	transportStr := "gRPC"
	if transport == otelmtlog.OTLPTransportHTTP {
		transportStr = "HTTP"
	}
	
	log := mtlog.New(
		mtlog.WithSink(sink),
		mtlog.WithMinimumLevel(core.VerboseLevel),
		mtlog.WithProperty("TestRun", testID),
		mtlog.WithProperty("Transport", transportStr),
	)

	// Log various events
	log.Verbose("Verbose message {TestId}", testID)
	log.Debug("Debug message {TestId} with {Value}", testID, 42)
	log.Information("Information message {TestId}", testID)
	log.Warning("Warning message {TestId} with {Count}", testID, 5)
	log.Error("Error message {TestId} with error", testID)

	// Test with structured properties
	log.ForContext("User", map[string]any{
		"Id":    123,
		"Name":  "TestUser",
		"Email": "test@example.com",
	}).Information("User action {Action}", "login")

	// Flush to ensure events are sent
	if err := sink.Flush(); err != nil {
		t.Errorf("Failed to flush sink: %v", err)
	}

	// Give collector time to process
	time.Sleep(2 * time.Second)

	// Check metrics
	metrics := sink.GetMetrics()
	if metrics["exported"] == 0 {
		t.Error("No events were exported")
	}
	if metrics["dropped"] > 0 {
		t.Errorf("Events were dropped: %d", metrics["dropped"])
	}
	if metrics["errors"] > 0 {
		t.Errorf("Errors occurred: %d", metrics["errors"])
	}

	t.Logf("Successfully exported %d events via %v", metrics["exported"], transport)
}

func TestOTELEnricherIntegration(t *testing.T) {
	collectorEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if collectorEndpoint == "" {
		collectorEndpoint = "localhost:4317"
	}

	if !isCollectorAvailable(collectorEndpoint) {
		t.Skip("OTEL Collector is not available, skipping integration test")
	}

	// Setup tracer
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	tracer := otel.Tracer("integration-test")

	// Create context with span
	ctx, span := tracer.Start(context.Background(), "test-operation")
	defer span.End()

	spanCtx := span.SpanContext()
	testID := fmt.Sprintf("test-%d", time.Now().UnixNano())

	// Create OTLP sink
	sink, err := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint(collectorEndpoint),
		otelmtlog.WithOTLPBatching(10, 1*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create OTLP sink: %v", err)
	}
	defer sink.Close()

	// Test different enricher types
	t.Run("FastOTELEnricher", func(t *testing.T) {
		log := mtlog.New(
			otelmtlog.WithOTELEnricher(ctx),
			mtlog.WithSink(sink),
			mtlog.WithProperty("EnricherType", "Fast"),
			mtlog.WithProperty("TestRun", testID),
		)

		log.Information("Message with trace context")
		
		// Verify trace context was added
		// This would be visible in the collector output
	})

	t.Run("StaticOTELEnricher", func(t *testing.T) {
		enricher := otelmtlog.NewStaticOTELEnricher(ctx)
		log := mtlog.New(
			mtlog.WithEnricher(enricher),
			mtlog.WithSink(sink),
			mtlog.WithProperty("EnricherType", "Static"),
			mtlog.WithProperty("TestRun", testID),
		)

		log.Information("Message with static trace context")
	})

	t.Run("CachingOTELEnricher", func(t *testing.T) {
		enricher := otelmtlog.NewOTELEnricher(ctx)
		log := mtlog.New(
			mtlog.WithEnricher(enricher),
			mtlog.WithSink(sink),
			mtlog.WithProperty("EnricherType", "Caching"),
			mtlog.WithProperty("TestRun", testID),
		)

		// Log multiple times to test caching
		for i := 0; i < 5; i++ {
			log.Information("Message {Index} with cached trace context", i)
		}
	})

	// Flush and check
	if err := sink.Flush(); err != nil {
		t.Errorf("Failed to flush: %v", err)
	}

	time.Sleep(2 * time.Second)

	metrics := sink.GetMetrics()
	t.Logf("Enricher test exported %d events with trace.id=%s span.id=%s",
		metrics["exported"], spanCtx.TraceID().String(), spanCtx.SpanID().String())
}

func TestHealthCheck(t *testing.T) {
	collectorEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if collectorEndpoint == "" {
		collectorEndpoint = "localhost:4317"
	}

	if !isCollectorAvailable(collectorEndpoint) {
		t.Skip("OTEL Collector is not available, skipping integration test")
	}

	sink, err := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint(collectorEndpoint),
	)
	if err != nil {
		t.Fatalf("Failed to create OTLP sink: %v", err)
	}
	defer sink.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Perform health check
	if err := sink.HealthCheck(ctx); err != nil {
		t.Errorf("Health check failed: %v", err)
	}

	t.Log("Health check passed")
}

// Helper functions

func isCollectorAvailable(endpoint string) bool {
	// For HTTP, check the health endpoint
	if endpoint == "localhost:4318" {
		resp, err := http.Get("http://localhost:13133/health")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}

	// For gRPC, we'll try to create a sink and check if it succeeds
	sink, err := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint(endpoint),
		otelmtlog.WithOTLPTimeout(5*time.Second),
	)
	if err != nil {
		return false
	}
	defer sink.Close()

	// Try a health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	return sink.HealthCheck(ctx) == nil
}

