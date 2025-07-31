package sinks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

func TestDurableSink_CreationAndBasicConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mtlog-durable-basic-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with file sink
	logFile := filepath.Join(tempDir, "test.log")
	fileSink, err := NewFileSink(logFile)
	if err != nil {
		t.Fatalf("Failed to create file sink: %v", err)
	}

	// Create durable sink with default options
	durableSink, err := NewDurableSink(fileSink, DurableOptions{
		BufferPath: filepath.Join(tempDir, "buffer"),
	})
	if err != nil {
		t.Fatalf("Failed to create durable sink: %v", err)
	}
	defer durableSink.Close()

	// Test basic properties
	if !durableSink.IsHealthy() {
		t.Error("Expected newly created sink to be healthy")
	}

	metrics := durableSink.GetMetrics()
	expectedMetrics := []string{"delivered", "buffered", "dropped", "retries"}
	for _, metric := range expectedMetrics {
		if _, exists := metrics[metric]; !exists {
			t.Errorf("Expected metric '%s' to exist", metric)
		}
	}

	// Test that buffer directory was created
	if _, err := os.Stat(filepath.Join(tempDir, "buffer")); os.IsNotExist(err) {
		t.Error("Expected buffer directory to be created")
	}
}

func TestDurableSink_FileLogging(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mtlog-durable-file-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create file sink
	logFile := filepath.Join(tempDir, "app.log")
	fileSink, err := NewFileSink(logFile)
	if err != nil {
		t.Fatalf("Failed to create file sink: %v", err)
	}

	// Create durable wrapper
	durableSink, err := NewDurableSink(fileSink, DurableOptions{
		BufferPath:    filepath.Join(tempDir, "buffer"),
		FlushInterval: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create durable sink: %v", err)
	}
	defer durableSink.Close()

	// Create test events directly
	testEvents := []*core.LogEvent{
		{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Application started successfully",
			Properties:      map[string]any{"Component": "DurableTest"},
		},
		{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Processing user request",
			Properties:      map[string]any{"Component": "DurableTest"},
		},
		{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Operation completed",
			Properties:      map[string]any{"Component": "DurableTest"},
		},
	}

	// Emit events directly to sink
	for _, event := range testEvents {
		durableSink.Emit(event)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify file was created and contains messages
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	contentStr := string(content)
	expectedMessages := []string{
		"Application started successfully",
		"Processing user request",
		"Operation completed",
	}
	for _, msg := range expectedMessages {
		if !strings.Contains(contentStr, msg) {
			t.Errorf("Expected log file to contain '%s'", msg)
		}
	}

	// Verify metrics
	metrics := durableSink.GetMetrics()
	if metrics["delivered"] == 0 {
		t.Error("Expected some events to be delivered")
	}

	if !durableSink.IsHealthy() {
		t.Error("Expected file sink to remain healthy")
	}
}

func TestDurableSink_BufferFileCreation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mtlog-durable-buffer-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a working file sink (file sink creation doesn't validate path until first write)
	workingFile := filepath.Join(tempDir, "working.log")
	fileSink, err := NewFileSink(workingFile)
	if err != nil {
		t.Fatalf("Failed to create working file sink: %v", err)
	}

	bufferPath := filepath.Join(tempDir, "test-buffer")

	// Create durable sink with small buffer size to force rotation
	durableSink, err := NewDurableSink(fileSink, DurableOptions{
		BufferPath:     bufferPath,
		MaxBufferSize:  100, // Very small to trigger rotation
		MaxBufferFiles: 3,
		FlushInterval:  50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create durable sink: %v", err)
	}
	defer durableSink.Close()

	// Create many events to ensure buffer file testing
	for i := range 20 {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: fmt.Sprintf("Buffer test message %d with some additional data to make it larger", i),
			Properties:      map[string]any{"Test": "BufferRotation", "Index": i},
		}
		durableSink.Emit(event)
	}

	// Wait for processing
	time.Sleep(1 * time.Second)

	// Check that buffer directory was created (even if not used due to healthy sink)
	if _, err := os.Stat(bufferPath); os.IsNotExist(err) {
		t.Error("Expected buffer directory to be created")
	}

	// Verify working file contains the messages
	if content, err := os.ReadFile(workingFile); err != nil {
		t.Errorf("Failed to read working log file: %v", err)
	} else {
		contentStr := string(content)
		if !strings.Contains(contentStr, "Buffer test message") {
			t.Error("Expected working log file to contain test messages")
		}
	}
}

func TestDurableSink_ConfigurationValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mtlog-durable-config-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fileSink, err := NewFileSink(filepath.Join(tempDir, "test.log"))
	if err != nil {
		t.Fatalf("Failed to create file sink: %v", err)
	}

	// Test with minimal config
	durableSink1, err := NewDurableSink(fileSink, DurableOptions{
		BufferPath: filepath.Join(tempDir, "buffer1"),
	})
	if err != nil {
		t.Errorf("Expected minimal config to work: %v", err)
	} else {
		durableSink1.Close()
	}

	// Test with comprehensive config
	durableSink2, err := NewDurableSink(fileSink, DurableOptions{
		BufferPath:      filepath.Join(tempDir, "buffer2"),
		MaxBufferSize:   1024 * 1024,
		MaxBufferFiles:  5,
		RetryInterval:   30 * time.Second,
		BatchSize:       50,
		FlushInterval:   1 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		OnError: func(err error) {
			t.Logf("Config test error: %v", err)
		},
	})
	if err != nil {
		t.Errorf("Expected comprehensive config to work: %v", err)
	} else {
		durableSink2.Close()
	}
}

func TestDurableSink_CleanupProcess(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mtlog-durable-cleanup-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	bufferPath := filepath.Join(tempDir, "cleanup-buffer")

	// Create a failing sink first to generate buffer files
	invalidSink, _ := NewSeqSink("http://localhost:9999") // Invalid URL to force buffering
	durableSink, err := NewDurableSink(invalidSink, DurableOptions{
		BufferPath:     bufferPath,
		MaxBufferSize:  100, // Small size to force rotation
		MaxBufferFiles: 3,   // Keep only 3 files
		FlushInterval:  50 * time.Millisecond,
		RetryInterval:  100 * time.Millisecond,
		OnError: func(err error) {
			// Suppress error output for this test
		},
	})
	if err != nil {
		t.Fatalf("Failed to create durable sink: %v", err)
	}

	// Create many events to generate multiple buffer files
	for i := range 50 {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: fmt.Sprintf("Cleanup test message %d with additional content to reach buffer size limit and force rotation", i),
			Properties:      map[string]any{"Test": "Cleanup", "Index": i},
		}
		durableSink.Emit(event)
	}

	// Wait for buffer rotation
	time.Sleep(500 * time.Millisecond)

	// Check initial buffer files created
	initialFiles, err := filepath.Glob(filepath.Join(bufferPath, "buffer-*.jsonl"))
	if err != nil {
		t.Fatalf("Failed to get initial buffer files: %v", err)
	}

	if len(initialFiles) == 0 {
		t.Fatal("Expected buffer files to be created")
	}

	t.Logf("Initial buffer files: %d", len(initialFiles))

	// Create some fake processed files to test processed file cleanup
	processedFile1 := filepath.Join(bufferPath, "buffer-999990.jsonl.processed")
	processedFile2 := filepath.Join(bufferPath, "buffer-999991.jsonl.processed")
	if err := os.WriteFile(processedFile1, []byte("processed"), 0644); err != nil {
		t.Fatalf("Failed to create test processed file: %v", err)
	}
	if err := os.WriteFile(processedFile2, []byte("processed"), 0644); err != nil {
		t.Fatalf("Failed to create test processed file: %v", err)
	}

	// Close the sink to stop generating new files
	durableSink.Close()

	// Now create a working sink to trigger cleanup during recovery
	logFile := filepath.Join(tempDir, "cleanup.log")
	workingSink, err := NewFileSink(logFile)
	if err != nil {
		t.Fatalf("Failed to create working sink: %v", err)
	}

	durableSink2, err := NewDurableSink(workingSink, DurableOptions{
		BufferPath:     bufferPath,
		MaxBufferSize:  1024 * 1024,
		MaxBufferFiles: 3, // This should trigger cleanup
		RetryInterval:  100 * time.Millisecond,
		FlushInterval:  100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create second durable sink: %v", err)
	}
	defer durableSink2.Close()

	// Wait for cleanup and recovery
	time.Sleep(1 * time.Second)

	// Check final buffer files after cleanup
	finalFiles, err := filepath.Glob(filepath.Join(bufferPath, "buffer-*.jsonl"))
	if err != nil {
		t.Fatalf("Failed to get final buffer files: %v", err)
	}

	// Should have at most MaxBufferFiles + 1 (the new current buffer file)
	if len(finalFiles) > 4 {
		t.Errorf("Expected at most 4 buffer files after cleanup, got %d", len(finalFiles))
	}

	t.Logf("Final buffer files: %d", len(finalFiles))

	// Check that processed files were cleaned up
	processedFiles, err := filepath.Glob(filepath.Join(bufferPath, "*.processed"))
	if err != nil {
		t.Fatalf("Failed to check processed files: %v", err)
	}

	if len(processedFiles) > 0 {
		t.Errorf("Expected processed files to be cleaned up, but found %d", len(processedFiles))
	}

	// Verify that events were recovered and written to the working log
	if content, err := os.ReadFile(logFile); err != nil {
		t.Errorf("Failed to read recovery log: %v", err)
	} else {
		contentStr := string(content)
		if !strings.Contains(contentStr, "Cleanup test message") {
			t.Error("Expected recovered events in working log file")
		}
		t.Logf("Successfully recovered events to working log file")
	}
}

func TestDurableSink_MultipleInstances(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mtlog-durable-multi-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create multiple durable sinks with different buffer paths
	var durableSinks []*DurableSink

	for i := range 3 {
		// Create separate file sink for each instance
		logFile := filepath.Join(tempDir, fmt.Sprintf("app%d.log", i))
		fileSink, err := NewFileSink(logFile)
		if err != nil {
			t.Fatalf("Failed to create file sink %d: %v", i, err)
		}

		// Create durable wrapper with unique buffer path
		durableSink, err := NewDurableSink(fileSink, DurableOptions{
			BufferPath:    filepath.Join(tempDir, fmt.Sprintf("buffer%d", i)),
			FlushInterval: 100 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("Failed to create durable sink %d: %v", i, err)
		}

		durableSinks = append(durableSinks, durableSink)
	}

	// Log to all instances
	for i, durableSink := range durableSinks {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: fmt.Sprintf("Message from instance %d", i),
			Properties:      map[string]any{"Instance": i},
		}
		durableSink.Emit(event)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify each instance worked independently
	for i, durableSink := range durableSinks {
		metrics := durableSink.GetMetrics()
		if metrics["delivered"] == 0 {
			t.Errorf("Expected instance %d to deliver events", i)
		}

		if !durableSink.IsHealthy() {
			t.Errorf("Expected instance %d to be healthy", i)
		}

		durableSink.Close()
	}

	// Verify separate log files were created
	for i := range 3 {
		logFile := filepath.Join(tempDir, fmt.Sprintf("app%d.log", i))
		if content, err := os.ReadFile(logFile); err != nil {
			t.Errorf("Failed to read log file %d: %v", i, err)
		} else {
			contentStr := string(content)
			expectedText := fmt.Sprintf("Message from instance %d", i)
			if !strings.Contains(contentStr, expectedText) {
				t.Errorf("Expected log file %d to contain '%s'", i, expectedText)
			}
		}
	}
}
