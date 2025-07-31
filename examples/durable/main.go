package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	// Create a temporary directory for buffer files
	tempDir := filepath.Join(os.TempDir(), "mtlog-durable-example")
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	fmt.Printf("Durable buffer directory: %s\n", tempDir)

	// Example 1: Basic durable buffering with Seq
	fmt.Println("\n=== Example 1: Durable Seq Sink ===")

	// Create a Seq sink that might fail (we'll use a mock if it fails)
	var seqSinkInterface core.LogEventSink
	seqSink, err := sinks.NewSeqSink("http://localhost:5341")
	if err != nil {
		fmt.Printf("Note: Seq sink failed to connect (expected): %v\n", err)
		// Use a mock sink instead for demonstration
		seqSinkInterface = &mockSink{name: "Seq"}
	} else {
		seqSinkInterface = seqSink
	}

	// Wrap with durable buffering
	durableSeq, err := sinks.NewDurableSink(seqSinkInterface, sinks.DurableOptions{
		BufferPath:    filepath.Join(tempDir, "seq-buffer"),
		MaxBufferSize: 1024 * 1024, // 1MB
		RetryInterval: 5 * time.Second,
		FlushInterval: 1 * time.Second,
		BatchSize:     10,
		OnError: func(err error) {
			fmt.Printf("Durable sink error: %v\n", err)
		},
	})
	if err != nil {
		log.Fatalf("Failed to create durable sink: %v", err)
	}

	logger := mtlog.New(
		mtlog.WithSink(durableSeq),
		mtlog.WithProperty("Service", "DurableExample"),
	)

	// Log some events
	logger.Information("Application started")
	logger.Warning("This message will be buffered if Seq is unavailable")
	logger.Error("Critical error occurred")

	// Show metrics
	showMetrics("Durable Seq", durableSeq)

	durableSeq.Close()

	// Example 2: Using convenience methods
	fmt.Println("\n=== Example 2: Convenience Methods ===")

	logger2 := mtlog.New(
		mtlog.WithDurableSeq("http://localhost:5341", filepath.Join(tempDir, "convenient-buffer")),
		mtlog.WithProperty("Example", "Convenience"),
	)

	logger2.Information("Using convenient durable Seq configuration")
	logger2.Debug("Debug message with convenience method")

	// Example 3: Configuration-based setup
	fmt.Println("\n=== Example 3: Advanced Configuration ===")

	// Create a failing sink for demonstration
	failingSink := &failingSink{}

	durableAdvanced, err := sinks.NewDurableSink(failingSink, sinks.DurableOptions{
		BufferPath:      filepath.Join(tempDir, "advanced-buffer"),
		MaxBufferSize:   512 * 1024, // 512KB
		MaxBufferFiles:  5,
		RetryInterval:   2 * time.Second,
		FlushInterval:   500 * time.Millisecond,
		BatchSize:       5,
		ShutdownTimeout: 10 * time.Second,
		OnError: func(err error) {
			fmt.Printf("Advanced sink error: %v\n", err)
		},
	})
	if err != nil {
		log.Fatalf("Failed to create advanced durable sink: %v", err)
	}

	logger3 := mtlog.New(
		mtlog.WithSink(durableAdvanced),
		mtlog.WithProperty("Mode", "Advanced"),
	)

	// Generate events that will be buffered
	for i := range 10 {
		logger3.Information("Buffered message {Index}", i)
		time.Sleep(100 * time.Millisecond)
	}

	showMetrics("Advanced Durable", durableAdvanced)

	// Simulate sink recovery
	fmt.Println("\nSimulating sink recovery...")
	failingSink.Recover()

	// Wait for retry
	time.Sleep(3 * time.Second)

	showMetrics("Advanced Durable (after recovery)", durableAdvanced)

	durableAdvanced.Close()

	// Example 4: Multiple sinks with different reliability
	fmt.Println("\n=== Example 4: Mixed Reliability ===")

	// Reliable console sink
	consoleSink := sinks.NewConsoleSink()

	// Unreliable remote sink with durable buffering
	remoteSink := &intermittentSink{failureRate: 0.3}
	durableRemote, err := sinks.NewDurableSink(remoteSink, sinks.DurableOptions{
		BufferPath:    filepath.Join(tempDir, "remote-buffer"),
		RetryInterval: 1 * time.Second,
		FlushInterval: 500 * time.Millisecond,
	})
	if err != nil {
		log.Fatalf("Failed to create durable remote sink: %v", err)
	}

	logger4 := mtlog.New(
		mtlog.WithSink(consoleSink),   // Always works
		mtlog.WithSink(durableRemote), // Sometimes fails, but buffered
		mtlog.WithProperty("Reliability", "Mixed"),
	)

	// Generate mixed traffic
	for i := range 5 {
		logger4.Information("Mixed reliability message {Index}", i)
		time.Sleep(200 * time.Millisecond)
	}

	showMetrics("Mixed Remote", durableRemote)

	durableRemote.Close()

	fmt.Println("\n=== Durable Buffer Example Complete ===")
	fmt.Printf("Buffer files created in: %s\n", tempDir)
}

func showMetrics(name string, sink *sinks.DurableSink) {
	metrics := sink.GetMetrics()
	fmt.Printf("%s Metrics:\n", name)
	fmt.Printf("  Delivered: %d\n", metrics["delivered"])
	fmt.Printf("  Buffered:  %d\n", metrics["buffered"])
	fmt.Printf("  Dropped:   %d\n", metrics["dropped"])
	fmt.Printf("  Retries:   %d\n", metrics["retries"])
	fmt.Printf("  Healthy:   %t\n", sink.IsHealthy())
	fmt.Println()
}

// Mock sink for demonstration
type mockSink struct {
	name string
}

func (ms *mockSink) Emit(event *core.LogEvent) {
	fmt.Printf("[%s] %v: %s\n", ms.name, event.Level, event.MessageTemplate)
}

func (ms *mockSink) Close() error {
	return nil
}

// Failing sink for demonstration
type failingSink struct {
	recovered bool
}

func (fs *failingSink) Emit(event *core.LogEvent) {
	if !fs.recovered {
		panic("sink is failing")
	}
	fmt.Printf("[RECOVERED] %v: %s\n", event.Level, event.MessageTemplate)
}

func (fs *failingSink) Close() error {
	return nil
}

func (fs *failingSink) Recover() {
	fs.recovered = true
	fmt.Println("Sink has recovered!")
}

// Intermittent sink for demonstration
type intermittentSink struct {
	failureRate float64
	counter     int
}

func (is *intermittentSink) Emit(event *core.LogEvent) {
	is.counter++
	// Fail based on counter pattern
	if float64(is.counter%10) < is.failureRate*10 {
		panic("intermittent failure")
	}
	fmt.Printf("[REMOTE] %v: %s\n", event.Level, event.MessageTemplate)
}

func (is *intermittentSink) Close() error {
	return nil
}
