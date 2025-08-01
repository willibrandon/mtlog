package sinks

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/formatters/output"
	"github.com/willibrandon/mtlog/internal/parser"
	"github.com/willibrandon/mtlog/selflog"
)

// FileSink writes log events to a file.
type FileSink struct {
	path           string
	file           *os.File
	mu             sync.Mutex
	bufferSize     int
	buffer         []byte
	isOpen         bool
	template       string
	parsedTemplate *output.Template
}

// NewFileSink creates a new file sink.
func NewFileSink(path string) (*FileSink, error) {
	return NewFileSinkWithOptions(path, 4096) // 4KB default buffer
}

// NewFileSinkWithOptions creates a new file sink with custom options.
func NewFileSinkWithOptions(path string, bufferSize int) (*FileSink, error) {
	fs := &FileSink{
		path:       path,
		bufferSize: bufferSize,
		buffer:     make([]byte, 0, bufferSize),
	}

	if err := fs.open(); err != nil {
		return nil, err
	}

	return fs, nil
}

// NewFileSinkWithTemplate creates a new file sink with a custom output template.
func NewFileSinkWithTemplate(path string, template string) (*FileSink, error) {
	parsedTemplate, err := output.Parse(template)
	if err != nil {
		return nil, fmt.Errorf("invalid output template: %w", err)
	}
	fs := &FileSink{
		path:           path,
		bufferSize:     4096,
		buffer:         make([]byte, 0, 4096),
		template:       template,
		parsedTemplate: parsedTemplate,
	}

	if err := fs.open(); err != nil {
		return nil, err
	}

	return fs, nil
}

// Emit writes the log event to the file.
func (fs *FileSink) Emit(event *core.LogEvent) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if !fs.isOpen {
		return
	}

	var message string
	if fs.parsedTemplate != nil {
		// Use template-based formatting
		message = fs.parsedTemplate.Render(event)
	} else {
		// Use default formatting
		message = fs.formatEvent(event)
	}

	// Write to file
	if _, err := fs.file.WriteString(message + "\n"); err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[file] write failed: %v (path=%s)", err, fs.path)
		}
	}
}

// Close flushes and closes the file.
func (fs *FileSink) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if !fs.isOpen {
		return nil
	}

	fs.isOpen = false

	// Sync to ensure all data is written
	if err := fs.file.Sync(); err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[file] sync failed: %v (path=%s)", err, fs.path)
		}
		return fmt.Errorf("failed to sync log file: %w", err)
	}

	// Close the file
	if err := fs.file.Close(); err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[file] close failed: %v (path=%s)", err, fs.path)
		}
		return fmt.Errorf("failed to close log file: %w", err)
	}

	return nil
}

// open creates or opens the log file.
func (fs *FileSink) open() error {
	// Ensure directory exists
	dir := filepath.Dir(fs.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[file] failed to create directory: %v (path=%s)", err, dir)
		}
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open file with append mode
	file, err := os.OpenFile(fs.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[file] failed to open file: %v (path=%s)", err, fs.path)
		}
		return fmt.Errorf("failed to open log file: %w", err)
	}

	fs.file = file
	fs.isOpen = true

	return nil
}

// formatEvent formats a log event for file output.
func (fs *FileSink) formatEvent(event *core.LogEvent) string {
	// Parse template to render message
	tmpl, err := parser.Parse(event.MessageTemplate)
	if err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[file] template parse error: %v (template=%q)", err, event.MessageTemplate)
		}
		// Fallback to raw template
		tmpl = &parser.MessageTemplate{
			Raw:    event.MessageTemplate,
			Tokens: []parser.MessageTemplateToken{&parser.TextToken{Text: event.MessageTemplate}},
		}
	}

	// Render the message
	message := tmpl.Render(event.Properties)

	// Format: TIMESTAMP [LEVEL] MESSAGE {PROPERTIES}
	levelStr := formatLevel(event.Level)
	timestamp := event.Timestamp.Format(time.RFC3339)

	// Add properties if any (excluding those already in the message)
	var propsStr string
	if len(event.Properties) > 0 {
		// Extract property names from template
		propNames := parser.ExtractPropertyNames(event.MessageTemplate)
		propNamesSet := make(map[string]bool)
		for _, name := range propNames {
			propNamesSet[name] = true
		}

		// Add additional properties
		additionalProps := make(map[string]any)
		for k, v := range event.Properties {
			if !propNamesSet[k] {
				additionalProps[k] = v
			}
		}

		if len(additionalProps) > 0 {
			propsStr = " " + formatProperties(additionalProps)
		}
	}

	return fmt.Sprintf("%s [%s] %s%s", timestamp, levelStr, message, propsStr)
}

// EmitSimple writes a simple log message without allocations.
func (fs *FileSink) EmitSimple(timestamp time.Time, level core.LogEventLevel, message string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if !fs.isOpen {
		return
	}

	// Format the simple message
	levelStr := formatLevel(level)
	timestampStr := timestamp.Format(time.RFC3339)

	// Write to file
	if _, err := fmt.Fprintf(fs.file, "%s [%s] %s\n", timestampStr, levelStr, message); err != nil {
		// Log to stderr if file write fails
		fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
	}
}

// formatProperties formats additional properties as JSON-like string.
func formatProperties(props map[string]any) string {
	if len(props) == 0 {
		return ""
	}

	result := "{"
	first := true
	for k, v := range props {
		if !first {
			result += ", "
		}
		result += fmt.Sprintf("%s: %v", k, v)
		first = false
	}
	result += "}"

	return result
}
