package sinks

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

func TestRollingFileSink(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	// Create sink
	sink, err := NewRollingFileSink(RollingFileOptions{
		FilePath:        logPath,
		MaxFileSize:     1024, // 1KB for testing
		RetainFileCount: 3,
	})
	if err != nil {
		t.Fatalf("Failed to create rolling file sink: %v", err)
	}
	defer sink.Close()

	// Write a log event
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.InformationLevel,
		MessageTemplate: "Test message {Number}",
		Properties: map[string]any{
			"Number": 1,
		},
	}

	sink.Emit(event)
	sink.Close()

	// Check file exists and contains log
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "Test message 1") {
		t.Errorf("Log file doesn't contain expected message: %s", content)
	}
}

func TestRollingBySize(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	sink, err := NewRollingFileSink(RollingFileOptions{
		FilePath:    logPath,
		MaxFileSize: 200, // Small size for testing
	})
	if err != nil {
		t.Fatalf("Failed to create rolling file sink: %v", err)
	}
	defer sink.Close()

	// Write multiple events to trigger rolling
	for i := 0; i < 10; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "This is a test message number {Number} with some padding to increase size",
			Properties: map[string]any{
				"Number": i,
			},
		}
		sink.Emit(event)
	}

	sink.Close()

	// Check that rolling occurred
	files, err := filepath.Glob(filepath.Join(tempDir, "test-*.log"))
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if len(files) == 0 {
		t.Error("Expected rolled files but found none")
	}

	// Current file should exist and be small
	stat, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat current log file: %v", err)
	}

	// Allow some tolerance for the last partial write
	if stat.Size() > 300 {
		t.Errorf("Current log file too large: %d bytes (expected < 300)", stat.Size())
	}
}

func TestRollingWithCompression(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	sink, err := NewRollingFileSink(RollingFileOptions{
		FilePath:            logPath,
		MaxFileSize:         200,
		CompressRolledFiles: true,
	})
	if err != nil {
		t.Fatalf("Failed to create rolling file sink: %v", err)
	}
	defer sink.Close()

	// Write events to trigger rolling
	for i := 0; i < 10; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Test message {Number} with padding to increase file size significantly",
			Properties: map[string]any{
				"Number": i,
			},
		}
		sink.Emit(event)
	}

	sink.Close()

	// Check for compressed files
	gzFiles, err := filepath.Glob(filepath.Join(tempDir, "test-*.log.gz"))
	if err != nil {
		t.Fatalf("Failed to list gz files: %v", err)
	}

	if len(gzFiles) == 0 {
		t.Error("Expected compressed files but found none")
	}

	// Verify compressed file is valid
	if len(gzFiles) > 0 {
		f, err := os.Open(gzFiles[0])
		if err != nil {
			t.Fatalf("Failed to open compressed file: %v", err)
		}
		defer f.Close()

		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("Failed to create gzip reader: %v", err)
		}
		defer gz.Close()

		content, err := io.ReadAll(gz)
		if err != nil {
			t.Fatalf("Failed to read compressed content: %v", err)
		}

		if !strings.Contains(string(content), "Test message") {
			t.Error("Compressed file doesn't contain expected content")
		}
	}
}

func TestRetentionPolicy(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	sink, err := NewRollingFileSink(RollingFileOptions{
		FilePath:        logPath,
		MaxFileSize:     100, // Very small to force multiple rolls
		RetainFileCount: 2,   // Keep only 2 rolled files
	})
	if err != nil {
		t.Fatalf("Failed to create rolling file sink: %v", err)
	}
	defer sink.Close()

	// Write many events to create multiple rolled files
	for i := 0; i < 20; i++ {
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Message {Number} with padding",
			Properties: map[string]any{
				"Number": i,
			},
		}
		sink.Emit(event)
		time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	}

	sink.Close()

	// Check rolled file count
	files, err := filepath.Glob(filepath.Join(tempDir, "test-*.log"))
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	if len(files) > 2 {
		t.Errorf("Expected at most 2 rolled files, found %d", len(files))
	}
}

func TestTimeBasedRolling(t *testing.T) {
	// This test is tricky because we can't wait for actual time to pass
	// We'll test the roll time calculation instead

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	tests := []struct {
		name     string
		interval RollingInterval
		check    func(rollTime time.Time)
	}{
		{
			name:     "Hourly",
			interval: RollingIntervalHourly,
			check: func(rollTime time.Time) {
				if rollTime.Minute() != 0 || rollTime.Second() != 0 {
					t.Error("Hourly roll time should be at the start of an hour")
				}
			},
		},
		{
			name:     "Daily",
			interval: RollingIntervalDaily,
			check: func(rollTime time.Time) {
				// Check that it's at midnight (00:00:00)
				if rollTime.Hour() != 0 || rollTime.Minute() != 0 || rollTime.Second() != 0 {
					t.Errorf("Daily roll time should be at midnight (00:00:00), got: %v", rollTime.Format("15:04:05"))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink, err := NewRollingFileSink(RollingFileOptions{
				FilePath:        logPath,
				RollingInterval: tt.interval,
			})
			if err != nil {
				t.Fatalf("Failed to create sink: %v", err)
			}
			defer sink.Close()

			// Check roll time
			if sink.rollTime.Before(time.Now()) {
				t.Error("Roll time should be in the future")
			}

			tt.check(sink.rollTime)
		})
	}
}

func TestConcurrentWriting(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	sink, err := NewRollingFileSink(RollingFileOptions{
		FilePath:    logPath,
		MaxFileSize: 10 * 1024, // 10KB
	})
	if err != nil {
		t.Fatalf("Failed to create rolling file sink: %v", err)
	}
	defer sink.Close()

	// Write concurrently from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				event := &core.LogEvent{
					Timestamp:       time.Now(),
					Level:           core.InformationLevel,
					MessageTemplate: "Goroutine {ID} message {Number}",
					Properties: map[string]any{
						"ID":     id,
						"Number": j,
					},
				}
				sink.Emit(event)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	sink.Close()

	// Verify file exists and has content
	stat, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Failed to stat log file: %v", err)
	}

	if stat.Size() == 0 {
		t.Error("Log file is empty after concurrent writes")
	}
}

func TestInvalidOptions(t *testing.T) {
	tests := []struct {
		name    string
		options RollingFileOptions
		wantErr bool
	}{
		{
			name:    "Empty file path",
			options: RollingFileOptions{},
			wantErr: true,
		},
		{
			name: "Valid options",
			options: RollingFileOptions{
				FilePath: filepath.Join(t.TempDir(), "test.log"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink, err := NewRollingFileSink(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRollingFileSink() error = %v, wantErr %v", err, tt.wantErr)
			}
			if sink != nil {
				sink.Close()
			}
		})
	}
}

func TestBufferedWriter(t *testing.T) {
	// Test the buffered writer separately
	var buf strings.Builder
	bw := newBufferedWriter(&buf, 10)

	// Write less than buffer size
	n, err := bw.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Errorf("Write failed: n=%d, err=%v", n, err)
	}

	// Buffer should not be flushed yet
	if buf.Len() != 0 {
		t.Error("Buffer flushed too early")
	}

	// Write more to trigger flush
	n, err = bw.Write([]byte(" world"))
	if err != nil || n != 6 {
		t.Errorf("Write failed: n=%d, err=%v", n, err)
	}

	// Now buffer should be flushed
	if buf.Len() == 0 {
		t.Error("Buffer not flushed when full")
	}

	// Explicit flush
	if err := bw.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}

	if buf.String() != "hello world" {
		t.Errorf("Unexpected content: %q", buf.String())
	}
}

func BenchmarkRollingFileSink(b *testing.B) {
	tempDir := b.TempDir()
	logPath := filepath.Join(tempDir, "bench.log")

	sink, err := NewRollingFileSink(RollingFileOptions{
		FilePath:    logPath,
		MaxFileSize: 100 * 1024 * 1024, // 100MB (won't roll during benchmark)
		BufferSize:  64 * 1024,         // 64KB buffer
	})
	if err != nil {
		b.Fatalf("Failed to create sink: %v", err)
	}
	defer sink.Close()

	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.InformationLevel,
		MessageTemplate: "Benchmark message {Number} with {Text}",
		Properties: map[string]any{
			"Number": 42,
			"Text":   "some text value",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		sink.Emit(event)
	}
}
