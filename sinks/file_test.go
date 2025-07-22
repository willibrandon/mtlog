package sinks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

func TestFileSink(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	
	sink, err := NewFileSink(logPath)
	if err != nil {
		t.Fatalf("Failed to create file sink: %v", err)
	}
	defer sink.Close()
	
	// Create test events
	events := []struct {
		level    core.LogEventLevel
		template string
		args     []interface{}
	}{
		{core.InformationLevel, "Application started", nil},
		{core.WarningLevel, "User {UserId} has {Count} failed attempts", []interface{}{123, 3}},
		{core.ErrorLevel, "Failed to process order {OrderId}", []interface{}{"ORD-789"}},
	}
	
	// Emit events
	for _, e := range events {
		props := make(map[string]interface{})
		if e.args != nil {
			// Simple property extraction for test
			if strings.Contains(e.template, "UserId") && len(e.args) > 0 {
				props["UserId"] = e.args[0]
			}
			if strings.Contains(e.template, "Count") && len(e.args) > 1 {
				props["Count"] = e.args[1]
			}
			if strings.Contains(e.template, "OrderId") && len(e.args) > 0 {
				props["OrderId"] = e.args[0]
			}
		}
		
		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           e.level,
			MessageTemplate: e.template,
			Properties:      props,
		}
		
		sink.Emit(event)
	}
	
	// Close sink to flush
	if err := sink.Close(); err != nil {
		t.Errorf("Failed to close sink: %v", err)
	}
	
	// Read and verify log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 log lines, got %d", len(lines))
	}
	
	// Check each line contains expected elements
	expectations := []struct {
		level   string
		message string
	}{
		{"[INF]", "Application started"},
		{"[WRN]", "User 123 has 3 failed attempts"},
		{"[ERR]", "Failed to process order ORD-789"},
	}
	
	for i, line := range lines {
		if !strings.Contains(line, expectations[i].level) {
			t.Errorf("Line %d: expected level %s, got: %s", i, expectations[i].level, line)
		}
		if !strings.Contains(line, expectations[i].message) {
			t.Errorf("Line %d: expected message '%s', got: %s", i, expectations[i].message, line)
		}
	}
}

func TestFileSinkWithAdditionalProperties(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	
	sink, err := NewFileSink(logPath)
	if err != nil {
		t.Fatalf("Failed to create file sink: %v", err)
	}
	defer sink.Close()
	
	// Event with extra properties
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.InformationLevel,
		MessageTemplate: "User {UserId} logged in",
		Properties: map[string]interface{}{
			"UserId":      123,
			"Environment": "Production",
			"Version":     "1.0.0",
		},
	}
	
	sink.Emit(event)
	sink.Close()
	
	// Read log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	
	line := strings.TrimSpace(string(content))
	
	// Should contain the rendered message
	if !strings.Contains(line, "User 123 logged in") {
		t.Error("Line should contain rendered message")
	}
	
	// Should contain additional properties
	if !strings.Contains(line, "Environment: Production") {
		t.Error("Line should contain Environment property")
	}
	
	if !strings.Contains(line, "Version: 1.0.0") {
		t.Error("Line should contain Version property")
	}
}

func TestFileSinkDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	// Create nested path that doesn't exist
	logPath := filepath.Join(tmpDir, "logs", "app", "test.log")
	
	sink, err := NewFileSink(logPath)
	if err != nil {
		t.Fatalf("Failed to create file sink: %v", err)
	}
	defer sink.Close()
	
	// Directory should have been created
	dir := filepath.Dir(logPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Log directory should have been created")
	}
}

func TestFileSinkConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "concurrent.log")
	
	sink, err := NewFileSink(logPath)
	if err != nil {
		t.Fatalf("Failed to create file sink: %v", err)
	}
	defer sink.Close()
	
	// Emit events concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				event := &core.LogEvent{
					Timestamp:       time.Now(),
					Level:           core.InformationLevel,
					MessageTemplate: "Goroutine {Id} message {Num}",
					Properties: map[string]interface{}{
						"Id":  id,
						"Num": j,
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
	
	// Read and count lines
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 100 {
		t.Errorf("Expected 100 log lines, got %d", len(lines))
	}
}