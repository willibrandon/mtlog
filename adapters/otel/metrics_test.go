package otel_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	mtlogotel "github.com/willibrandon/mtlog/adapters/otel"
	"github.com/willibrandon/mtlog/core"
)


func TestMetricsExporter(t *testing.T) {
	// Create metrics exporter on a test port
	port := 9091
	exporter, err := mtlogotel.NewMetricsExporter(
		mtlogotel.WithMetricsPort(port),
	)
	if err != nil {
		t.Fatalf("Failed to create metrics exporter: %v", err)
	}
	defer exporter.Close()
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Record some events
	event1 := &core.LogEvent{
		Level:           core.InformationLevel,
		MessageTemplate: "Test info",
		Timestamp:       time.Now(),
		Properties: map[string]any{
			"trace.id": "abc123",
		},
	}
	exporter.RecordEvent(event1, 1.5)
	
	event2 := &core.LogEvent{
		Level:           core.ErrorLevel,
		MessageTemplate: "Test error",
		Timestamp:       time.Now(),
	}
	exporter.RecordEvent(event2, 2.3)
	
	// Record dropped event
	exporter.RecordDropped("queue_full")
	
	// Give metrics time to update
	time.Sleep(100 * time.Millisecond)
	
	// Fetch metrics
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	
	metrics := string(body)
	
	// Check for expected metrics (OTEL adds _total suffix to counters)
	expectedMetrics := []string{
		"mtlog_events_total_total",
		"mtlog_errors_total_total",
		"mtlog_dropped_total_total",
		"mtlog_latency_milliseconds_milliseconds",
		"mtlog_events_info_total_total",
		"mtlog_events_error_total_total",
	}
	
	for _, metric := range expectedMetrics {
		if !strings.Contains(metrics, metric) {
			t.Errorf("Expected metric %s not found in output", metric)
		}
	}
	
	// Check specific values
	if !strings.Contains(metrics, `mtlog_events_total_total{level="info"`) {
		t.Error("Expected info level event not found")
	}
	
	if !strings.Contains(metrics, `mtlog_events_total_total{level="error"`) {
		t.Error("Expected error level event not found")
	}
	
	if !strings.Contains(metrics, `reason="queue_full"`) {
		t.Error("Expected dropped event with reason not found")
	}
}

func TestMetricsSink(t *testing.T) {
	// Create metrics exporter
	exporter, err := mtlogotel.NewMetricsExporter(
		mtlogotel.WithMetricsPort(9092),
	)
	if err != nil {
		t.Fatalf("Failed to create metrics exporter: %v", err)
	}
	defer exporter.Close()
	
	// Create a test sink to wrap
	var emitted []*core.LogEvent
	innerSink := mtlogotel.NewTestSink(func(event *core.LogEvent) {
		emitted = append(emitted, event)
	})
	
	// Wrap with metrics sink
	metricsSink := mtlogotel.NewMetricsSink(innerSink, exporter)
	
	// Emit events
	levels := []core.LogEventLevel{
		core.VerboseLevel,
		core.DebugLevel,
		core.InformationLevel,
		core.WarningLevel,
		core.ErrorLevel,
		core.FatalLevel,
	}
	
	for _, level := range levels {
		event := &core.LogEvent{
			Level:           level,
			MessageTemplate: fmt.Sprintf("Test level %d", int(level)),
			Timestamp:       time.Now(),
		}
		metricsSink.Emit(event)
	}
	
	// Check that all events were forwarded
	if len(emitted) != len(levels) {
		t.Errorf("Expected %d events, got %d", len(levels), len(emitted))
	}
	
	// Give metrics time to update
	time.Sleep(100 * time.Millisecond)
	
	// Fetch and check metrics
	resp, err := http.Get("http://localhost:9092/metrics")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	metrics := string(body)
	
	// Check that each level was recorded
	for _, level := range levels {
		levelStr := getLevelString(level)
		if !strings.Contains(metrics, fmt.Sprintf(`level="%s"`, levelStr)) {
			t.Errorf("Expected %s level in metrics", levelStr)
		}
	}
	
	// Check error counter (should be 2: error and fatal)
	if !strings.Contains(metrics, "mtlog_errors_total_total") {
		t.Error("Expected error counter in metrics")
	}
}

func TestMetricsExporterWithCustomPath(t *testing.T) {
	// Create exporter with custom path
	port := 9093
	exporter, err := mtlogotel.NewMetricsExporter(
		mtlogotel.WithMetricsPort(port),
		mtlogotel.WithMetricsPath("/custom/metrics"),
	)
	if err != nil {
		t.Fatalf("Failed to create metrics exporter: %v", err)
	}
	defer exporter.Close()
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Record a test event to ensure metrics are available
	event := &core.LogEvent{
		Level:           core.InformationLevel,
		MessageTemplate: "Test event",
		Timestamp:       time.Now(),
	}
	exporter.RecordEvent(event, 1.0)
	
	// Give metrics time to update
	time.Sleep(100 * time.Millisecond)
	
	// Try default path (should 404)
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	resp.Body.Close()
	
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 on default path, got %d", resp.StatusCode)
	}
	
	// Try custom path (should work)
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/custom/metrics", port))
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 on custom path, got %d", resp.StatusCode)
	}
	
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "mtlog_events_total_total") {
		t.Error("Expected metrics on custom path")
	}
}

func getLevelString(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel:
		return "verbose"
	case core.DebugLevel:
		return "debug"
	case core.InformationLevel:
		return "info"
	case core.WarningLevel:
		return "warning"
	case core.ErrorLevel:
		return "error"
	case core.FatalLevel:
		return "fatal"
	default:
		return "unknown"
	}
}

func BenchmarkMetricsRecording(b *testing.B) {
	exporter, err := mtlogotel.NewMetricsExporter(
		mtlogotel.WithMetricsPort(9094),
	)
	if err != nil {
		b.Fatalf("Failed to create exporter: %v", err)
	}
	defer exporter.Close()
	
	event := &core.LogEvent{
		Level:           core.InformationLevel,
		MessageTemplate: "Benchmark",
		Timestamp:       time.Now(),
		Properties: map[string]any{
			"trace.id": "bench123",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exporter.RecordEvent(event, 1.0)
	}
}