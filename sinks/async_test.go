package sinks

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

// mockSink is a test sink that tracks emitted events
type mockSink struct {
	mu     sync.Mutex
	events []*core.LogEvent
	delay  time.Duration
	failAt int
	calls  atomic.Int32
}

func (m *mockSink) Emit(event *core.LogEvent) {
	m.calls.Add(1)

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failAt > 0 && len(m.events) >= m.failAt {
		panic("mock sink failure")
	}

	m.events = append(m.events, event)
}

func (m *mockSink) EmitBatch(events []*core.LogEvent) {
	for _, event := range events {
		m.Emit(event)
	}
}

func (m *mockSink) GetEvents() []*core.LogEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]*core.LogEvent, len(m.events))
	copy(result, m.events)
	return result
}

func (m *mockSink) Close() error {
	return nil
}

func TestAsyncSinkBasic(t *testing.T) {
	mock := &mockSink{}
	async := NewAsyncSink(mock, AsyncOptions{
		BufferSize: 10,
	})
	defer async.Close()

	// Emit some events
	for i := 0; i < 5; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Test message {Number}",
			Properties: map[string]any{
				"Number": i,
			},
		}
		async.Emit(event)
	}

	// Wait for processing
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := async.WaitForEmpty(ctx); err != nil {
		t.Fatalf("Failed to wait for empty: %v", err)
	}

	// Check events were processed
	events := mock.GetEvents()
	if len(events) != 5 {
		t.Errorf("Expected 5 events, got %d", len(events))
	}
}

func TestAsyncSinkOverflowDrop(t *testing.T) {
	mock := &mockSink{
		delay: 50 * time.Millisecond, // Slow sink
	}

	errorCount := 0
	async := NewAsyncSink(mock, AsyncOptions{
		BufferSize:       2,
		OverflowStrategy: OverflowDrop,
		OnError: func(err error) {
			errorCount++
		},
	})
	defer async.Close()

	// Emit more events than buffer can hold
	for i := 0; i < 10; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Event {Number}",
			Properties: map[string]any{
				"Number": i,
			},
		}
		async.Emit(event)
	}

	// Close and wait
	if err := async.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Check metrics
	metrics := async.GetMetrics()
	if metrics.Dropped == 0 {
		t.Error("Expected some events to be dropped")
	}

	t.Logf("Processed: %d, Dropped: %d", metrics.Processed, metrics.Dropped)
}

func TestAsyncSinkOverflowBlock(t *testing.T) {
	mock := &mockSink{
		delay: 20 * time.Millisecond, // Slow processing
	}
	async := NewAsyncSink(mock, AsyncOptions{
		BufferSize:       2,
		OverflowStrategy: OverflowBlock,
	})
	defer async.Close()

	// Emit events - should block when buffer is full
	start := time.Now()
	for i := 0; i < 5; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Event {Number}",
			Properties: map[string]any{
				"Number": i,
			},
		}
		async.Emit(event)
	}

	// Should have taken some time due to blocking
	// 5 events with 20ms processing = 100ms, but parallel processing
	elapsed := time.Since(start)
	if elapsed < 40*time.Millisecond {
		t.Errorf("Expected blocking behavior, took only %v", elapsed)
	}
}

func TestAsyncSinkBatching(t *testing.T) {
	mock := &mockSink{}
	async := NewAsyncSink(mock, AsyncOptions{
		BufferSize:    100,
		BatchSize:     5,
		FlushInterval: 100 * time.Millisecond,
	})
	defer async.Close()

	// Emit exactly one batch worth of events
	for i := 0; i < 5; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Event {Number}",
			Properties: map[string]any{
				"Number": i,
			},
		}
		async.Emit(event)
	}

	// Wait for batch to be processed
	time.Sleep(50 * time.Millisecond)

	// Should have all events
	events := mock.GetEvents()
	if len(events) != 5 {
		t.Errorf("Expected 5 events after batch, got %d", len(events))
	}

	// Emit partial batch
	for i := 5; i < 7; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Event {Number}",
			Properties: map[string]any{
				"Number": i,
			},
		}
		async.Emit(event)
	}

	// Wait for flush interval
	time.Sleep(150 * time.Millisecond)

	// Should have flushed partial batch
	events = mock.GetEvents()
	if len(events) != 7 {
		t.Errorf("Expected 7 events after flush interval, got %d", len(events))
	}
}

func TestAsyncSinkErrorHandling(t *testing.T) {
	mock := &mockSink{
		failAt: 3, // Fail on third event
	}

	errorCount := 0
	var lastError error
	async := NewAsyncSink(mock, AsyncOptions{
		BufferSize: 10,
		OnError: func(err error) {
			errorCount++
			lastError = err
		},
	})
	defer async.Close()

	// Emit events
	for i := 0; i < 5; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Event {Number}",
			Properties: map[string]any{
				"Number": i,
			},
		}
		async.Emit(event)
	}

	// Close and wait
	async.Close()

	// Should have seen errors
	if errorCount == 0 {
		t.Error("Expected error callback to be called")
	}

	if lastError == nil || errorCount == 0 {
		t.Error("Expected error to be captured")
	}

	// Check metrics
	metrics := async.GetMetrics()
	if metrics.Errors == 0 {
		t.Error("Expected errors in metrics")
	}
}

func TestAsyncSinkShutdown(t *testing.T) {
	mock := &mockSink{
		delay: 10 * time.Millisecond, // Slow processing
	}

	async := NewAsyncSink(mock, AsyncOptions{
		BufferSize:      100,
		ShutdownTimeout: 1 * time.Second,
	})

	// Emit many events
	for i := 0; i < 20; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Event {Number}",
			Properties: map[string]any{
				"Number": i,
			},
		}
		async.Emit(event)
	}

	// Close should wait for all events
	start := time.Now()
	if err := async.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
	elapsed := time.Since(start)

	// Should have processed all events
	events := mock.GetEvents()
	if len(events) != 20 {
		t.Errorf("Expected 20 events, got %d", len(events))
	}

	// Should have taken at least 200ms (20 events * 10ms)
	if elapsed < 200*time.Millisecond {
		t.Error("Close returned too quickly")
	}
}

func TestAsyncSinkConcurrency(t *testing.T) {
	mock := &mockSink{}
	async := NewAsyncSink(mock, AsyncOptions{
		BufferSize: 1000,
	})
	defer async.Close()

	// Emit from multiple goroutines
	var wg sync.WaitGroup
	eventsPerGoroutine := 100
	goroutines := 10

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				event := &core.LogEvent{
					Timestamp:       time.Now(),
					Level:           core.InformationLevel,
					MessageTemplate: "Event from {Goroutine} number {Number}",
					Properties: map[string]any{
						"Goroutine": id,
						"Number":    i,
					},
				}
				async.Emit(event)
			}
		}(g)
	}

	wg.Wait()

	// Close and check
	if err := async.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	events := mock.GetEvents()
	expectedTotal := goroutines * eventsPerGoroutine
	if len(events) != expectedTotal {
		t.Errorf("Expected %d events, got %d", expectedTotal, len(events))
	}
}

func BenchmarkAsyncSink(b *testing.B) {
	benchmarks := []struct {
		name    string
		options AsyncOptions
	}{
		{
			name: "NoBatch",
			options: AsyncOptions{
				BufferSize: 1000,
			},
		},
		{
			name: "Batch10",
			options: AsyncOptions{
				BufferSize:    1000,
				BatchSize:     10,
				FlushInterval: 10 * time.Millisecond,
			},
		},
		{
			name: "Batch100",
			options: AsyncOptions{
				BufferSize:    1000,
				BatchSize:     100,
				FlushInterval: 50 * time.Millisecond,
			},
		},
	}

	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.InformationLevel,
		MessageTemplate: "Benchmark event {Number}",
		Properties: map[string]any{
			"Number": 42,
		},
	}

	for _, bench := range benchmarks {
		b.Run(bench.name, func(b *testing.B) {
			mock := &mockSink{}
			async := NewAsyncSink(mock, bench.options)
			defer async.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				async.Emit(event)
			}
		})
	}
}

// TestAsyncSinkWithRealSink tests async wrapper with a real file sink
func TestAsyncSinkWithRealSink(t *testing.T) {
	// Skip if short
	if testing.Short() {
		t.Skip("Skipping real sink test in short mode")
	}

	tempDir := t.TempDir()
	logPath := tempDir + "/async-test.log"

	// Create a file sink
	fileSink, err := NewFileSink(logPath)
	if err != nil {
		t.Fatalf("Failed to create file sink: %v", err)
	}

	// Wrap with async
	async := NewAsyncSink(fileSink, AsyncOptions{
		BufferSize:    100,
		BatchSize:     10,
		FlushInterval: 50 * time.Millisecond,
	})
	defer async.Close()

	// Log some events
	for i := 0; i < 25; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Async test message {Number}",
			Properties: map[string]any{
				"Number": i,
			},
		}
		async.Emit(event)
	}

	// Close and verify
	if err := async.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Read the file and count lines
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 25 {
		t.Errorf("Expected 25 log lines, got %d", len(lines))
	}
}
