//go:build integration
// +build integration

package sinks

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

func TestDurableSink_SeqFailureAndRecovery(t *testing.T) {
	// Create temporary directory for buffer files
	tempDir, err := os.MkdirTemp("", "mtlog-durable-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Use an invalid Seq URL to force failure
	invalidSeqURL := "http://localhost:9999"
	
	// Create Seq sink that will fail
	seqSink, err := NewSeqSink(invalidSeqURL)
	if err != nil {
		t.Fatalf("Failed to create Seq sink: %v", err)
	}
	
	// Wrap with durable buffering
	durableSink, err := NewDurableSink(seqSink, DurableOptions{
		BufferPath:    tempDir,
		RetryInterval: 1 * time.Second,
		FlushInterval: 500 * time.Millisecond,
		BatchSize:     3,
		OnError: func(err error) {
			t.Logf("Expected durable sink error: %v", err)
		},
	})
	if err != nil {
		t.Fatalf("Failed to create durable sink: %v", err)
	}
	defer durableSink.Close()
	
	// Create test events that should be buffered
	testEvents := []*core.LogEvent{
		{
			Timestamp:       time.Now(),
			Level:          core.InformationLevel,
			MessageTemplate: "First buffered message",
			Properties:      map[string]interface{}{"Test": "DurableSeq"},
		},
		{
			Timestamp:       time.Now(),
			Level:          core.InformationLevel,
			MessageTemplate: "Second buffered message",
			Properties:      map[string]interface{}{"Test": "DurableSeq"},
		},
		{
			Timestamp:       time.Now(),
			Level:          core.InformationLevel,
			MessageTemplate: "Third buffered message",
			Properties:      map[string]interface{}{"Test": "DurableSeq"},
		},
		{
			Timestamp:       time.Now(),
			Level:          core.InformationLevel,
			MessageTemplate: "Fourth buffered message",
			Properties:      map[string]interface{}{"Test": "DurableSeq"},
		},
	}
	
	for _, event := range testEvents {
		durableSink.Emit(event)
	}
	
	// Wait for buffering
	time.Sleep(2 * time.Second)
	
	// Verify events were buffered
	metrics := durableSink.GetMetrics()
	if metrics["buffered"] == 0 {
		t.Error("Expected events to be buffered when Seq is unavailable")
	}
	
	if durableSink.IsHealthy() {
		t.Error("Expected sink to be marked as unhealthy")
	}
	
	// Check that buffer files were created
	bufferFiles, err := filepath.Glob(filepath.Join(tempDir, "buffer-*.jsonl"))
	if err != nil {
		t.Fatalf("Failed to check buffer files: %v", err)
	}
	
	if len(bufferFiles) == 0 {
		t.Error("Expected buffer files to be created")
	}
	
	t.Logf("Created %d buffer files", len(bufferFiles))
	t.Logf("Buffered %d events", metrics["buffered"])
}

func TestDurableSink_FileFailureAndRecovery(t *testing.T) {
	// Create temporary directories
	tempDir, err := os.MkdirTemp("", "mtlog-durable-file-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a working log file (file sink creation doesn't validate directories upfront)
	logFile := filepath.Join(tempDir, "test.log")
	fileSink, err := NewFileSink(logFile)
	if err != nil {
		t.Fatalf("Failed to create file sink: %v", err)
	}
	
	// Create durable wrapper
	durableSink, err := NewDurableSink(fileSink, DurableOptions{
		BufferPath:    filepath.Join(tempDir, "file-buffer"),
		RetryInterval: 1 * time.Second,
		FlushInterval: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create durable sink: %v", err)
	}
	defer durableSink.Close()
	
	// Create test events
	testEvents := []*core.LogEvent{
		{
			Timestamp:       time.Now(),
			Level:          core.InformationLevel,
			MessageTemplate: "File test message 1",
			Properties:      map[string]interface{}{"Test": "DurableFile"},
		},
		{
			Timestamp:       time.Now(),
			Level:          core.WarningLevel,
			MessageTemplate: "File test message 2",
			Properties:      map[string]interface{}{"Test": "DurableFile"},
		},
		{
			Timestamp:       time.Now(),
			Level:          core.ErrorLevel,
			MessageTemplate: "File test message 3",
			Properties:      map[string]interface{}{"Test": "DurableFile"},
		},
	}
	
	// Emit events
	for _, event := range testEvents {
		durableSink.Emit(event)
	}
	
	// Wait for processing
	time.Sleep(2 * time.Second)
	
	// Verify events were delivered to file
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Expected log file to be created")
	} else {
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		
		contentStr := string(content)
		expectedMessages := []string{
			"File test message 1",
			"File test message 2", 
			"File test message 3",
		}
		
		for _, msg := range expectedMessages {
			if !strings.Contains(contentStr, msg) {
				t.Errorf("Expected log file to contain '%s'", msg)
			}
		}
	}
	
	// Verify metrics
	metrics := durableSink.GetMetrics()
	if metrics["delivered"] == 0 {
		t.Error("Expected some events to be delivered")
	}
}

func TestDurableSink_BufferPersistence(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "mtlog-durable-persistence-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	bufferPath := filepath.Join(tempDir, "persistence-buffer")
	
	// Phase 1: Create durable sink with failing wrapped sink and buffer events
	{
		// Use invalid Seq URL to force failure
		seqSink, _ := NewSeqSink("http://localhost:9999")
		
		durableSink, err := NewDurableSink(seqSink, DurableOptions{
			BufferPath:    bufferPath,
			RetryInterval: 5 * time.Second, // Long interval to prevent retry
			FlushInterval: 200 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("Failed to create durable sink: %v", err)
		}
		
		// Buffer events
		persistentEvents := []*core.LogEvent{
			{
				Timestamp:       time.Now(),
				Level:          core.InformationLevel,
				MessageTemplate: "Persistent message 1",
				Properties:      map[string]interface{}{"Phase": "Buffering"},
			},
			{
				Timestamp:       time.Now(),
				Level:          core.InformationLevel,
				MessageTemplate: "Persistent message 2",
				Properties:      map[string]interface{}{"Phase": "Buffering"},
			},
			{
				Timestamp:       time.Now(),
				Level:          core.InformationLevel,
				MessageTemplate: "Persistent message 3",
				Properties:      map[string]interface{}{"Phase": "Buffering"},
			},
		}
		
		for _, event := range persistentEvents {
			durableSink.Emit(event)
		}
		
		// Wait for buffering
		time.Sleep(1 * time.Second)
		
		// Verify buffering
		metrics := durableSink.GetMetrics()
		if metrics["buffered"] == 0 {
			t.Error("Expected events to be buffered")
		}
		
		// Close the sink
		durableSink.Close()
	}
	
	// Verify buffer files were created and persisted
	bufferFiles, err := filepath.Glob(filepath.Join(bufferPath, "buffer-*.jsonl"))
	if err != nil {
		t.Fatalf("Failed to check buffer files: %v", err)
	}
	
	if len(bufferFiles) == 0 {
		t.Fatal("Expected buffer files to persist after sink closure")
	}
	
	// Verify buffer files have content
	totalSize := int64(0)
	for _, file := range bufferFiles {
		info, err := os.Stat(file)
		if err != nil {
			t.Errorf("Failed to stat buffer file %s: %v", file, err)
		} else {
			totalSize += info.Size()
		}
	}
	
	if totalSize == 0 {
		t.Error("Expected buffer files to have content")
	}
	
	t.Logf("Persisted %d buffer files with %d total bytes", len(bufferFiles), totalSize)
	
	// Phase 2: Create new durable sink with working wrapped sink to test recovery
	{
		// Create working file sink
		logFile := filepath.Join(tempDir, "recovered.log")
		fileSink, err := NewFileSink(logFile)
		if err != nil {
			t.Fatalf("Failed to create recovery file sink: %v", err)
		}
		
		durableSink, err := NewDurableSink(fileSink, DurableOptions{
			BufferPath:    bufferPath,
			RetryInterval: 100 * time.Millisecond,
			FlushInterval: 200 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("Failed to create recovery durable sink: %v", err)
		}
		defer durableSink.Close()
		
		// Wait for recovery
		time.Sleep(2 * time.Second)
		
		// Verify recovery metrics
		metrics := durableSink.GetMetrics()
		if metrics["delivered"] == 0 {
			t.Error("Expected events to be delivered after recovery")
		}
		
		// Verify log file contains recovered events
		if content, err := os.ReadFile(logFile); err != nil {
			t.Errorf("Failed to read recovery log file: %v", err)
		} else {
			contentStr := string(content)
			expectedMessages := []string{
				"Persistent message 1",
				"Persistent message 2",
				"Persistent message 3",
			}
			
			for _, msg := range expectedMessages {
				if !strings.Contains(contentStr, msg) {
					t.Errorf("Expected recovered log to contain '%s'", msg)
				}
			}
		}
	}
}

func TestDurableSink_ConsoleSink(t *testing.T) {
	// Test durable buffering with console sink (which should never fail)
	tempDir, err := os.MkdirTemp("", "mtlog-durable-console-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create console sink
	consoleSink := NewConsoleSink()
	
	// Wrap with durable buffering
	durableSink, err := NewDurableSink(consoleSink, DurableOptions{
		BufferPath:    filepath.Join(tempDir, "console-buffer"),
		RetryInterval: 1 * time.Second,
		FlushInterval: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Failed to create durable console sink: %v", err)
	}
	defer durableSink.Close()
	
	// Create test events
	testEvents := []*core.LogEvent{
		{
			Timestamp:       time.Now(),
			Level:          core.InformationLevel,
			MessageTemplate: "Console message 1",
			Properties:      map[string]interface{}{"Test": "DurableConsole"},
		},
		{
			Timestamp:       time.Now(),
			Level:          core.WarningLevel,
			MessageTemplate: "Console message 2",
			Properties:      map[string]interface{}{"Test": "DurableConsole"},
		},
		{
			Timestamp:       time.Now(),
			Level:          core.ErrorLevel,
			MessageTemplate: "Console message 3",
			Properties:      map[string]interface{}{"Test": "DurableConsole"},
		},
	}
	
	// Emit events
	for _, event := range testEvents {
		durableSink.Emit(event)
	}
	
	// Wait for processing
	time.Sleep(1 * time.Second)
	
	// Console sink should always be healthy and deliver immediately
	if !durableSink.IsHealthy() {
		t.Error("Expected console sink to be healthy")
	}
	
	metrics := durableSink.GetMetrics()
	if metrics["delivered"] == 0 {
		t.Error("Expected console events to be delivered")
	}
	
	// Should have minimal or no buffering
	if metrics["buffered"] > metrics["delivered"] {
		t.Error("Expected console sink to have minimal buffering")
	}
}

func TestDurableSink_HTTPTimeout(t *testing.T) {
	// Test with a slow HTTP endpoint to trigger timeouts
	tempDir, err := os.MkdirTemp("", "mtlog-durable-timeout-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a server that responds very slowly to simulate timeouts
	slowServer := &http.Server{
		Addr: "localhost:9998",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Second) // Longer than typical timeout
			w.WriteHeader(http.StatusOK)
		}),
	}
	
	go func() {
		slowServer.ListenAndServe()
	}()
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	defer slowServer.Close()
	
	// Create Seq sink pointing to slow server
	seqSink, err := NewSeqSink("http://localhost:9998")
	if err != nil {
		t.Fatalf("Failed to create Seq sink: %v", err)
	}
	
	// Wrap with durable buffering
	durableSink, err := NewDurableSink(seqSink, DurableOptions{
		BufferPath:    filepath.Join(tempDir, "timeout-buffer"),
		RetryInterval: 1 * time.Second,
		FlushInterval: 200 * time.Millisecond,
		OnError: func(err error) {
			t.Logf("Expected timeout error: %v", err)
		},
	})
	if err != nil {
		t.Fatalf("Failed to create durable sink: %v", err)
	}
	defer durableSink.Close()
	
	// Create test events that should timeout and be buffered
	testEvents := []*core.LogEvent{
		{
			Timestamp:       time.Now(),
			Level:          core.InformationLevel,
			MessageTemplate: "Timeout test message 1",
			Properties:      map[string]interface{}{"Test": "Timeout"},
		},
		{
			Timestamp:       time.Now(),
			Level:          core.WarningLevel,
			MessageTemplate: "Timeout test message 2",
			Properties:      map[string]interface{}{"Test": "Timeout"},
		},
	}
	
	for _, event := range testEvents {
		durableSink.Emit(event)
	}
	
	// Wait for timeout and buffering
	time.Sleep(8 * time.Second)
	
	// Verify events were buffered due to timeout
	metrics := durableSink.GetMetrics()
	if metrics["buffered"] == 0 {
		t.Error("Expected events to be buffered due to timeout")
	}
	
	if durableSink.IsHealthy() {
		t.Error("Expected sink to be marked unhealthy after timeout")
	}
}

func isHTTPServerAvailable(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}