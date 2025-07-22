package sinks

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/parser"
)

// ConsoleSink writes log events to the console.
type ConsoleSink struct {
	output io.Writer
	mu     sync.Mutex
}

// NewConsoleSink creates a new console sink that writes to stdout.
func NewConsoleSink() *ConsoleSink {
	return &ConsoleSink{
		output: os.Stdout,
	}
}

// NewConsoleSinkWithWriter creates a new console sink with a custom writer.
func NewConsoleSinkWithWriter(w io.Writer) *ConsoleSink {
	return &ConsoleSink{
		output: w,
	}
}

// Emit writes the log event to the console.
func (cs *ConsoleSink) Emit(event *core.LogEvent) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	// Format the log event
	message := cs.formatEvent(event)
	
	// Write to output
	fmt.Fprintln(cs.output, message)
}

// Close releases any resources held by the sink.
func (cs *ConsoleSink) Close() error {
	// Nothing to close for console sink
	return nil
}

// formatEvent formats a log event for console output.
func (cs *ConsoleSink) formatEvent(event *core.LogEvent) string {
	// Parse template to render message
	tmpl, err := parser.Parse(event.MessageTemplate)
	if err != nil {
		// Fallback to raw template
		tmpl = &parser.MessageTemplate{
			Raw:    event.MessageTemplate,
			Tokens: []parser.MessageTemplateToken{&parser.TextToken{Text: event.MessageTemplate}},
		}
	}
	
	// Render the message
	message := tmpl.Render(event.Properties)
	
	// Format: [TIMESTAMP] [LEVEL] MESSAGE
	levelStr := formatLevel(event.Level)
	timestamp := event.Timestamp.Format("2006-01-02 15:04:05.000")
	
	return fmt.Sprintf("[%s] [%s] %s", timestamp, levelStr, message)
}

// EmitSimple writes a simple log message without allocations.
func (cs *ConsoleSink) EmitSimple(timestamp time.Time, level core.LogEventLevel, message string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	// Use zero-allocation formatter
	levelStr := formatLevel(level)
	_ = writeSimple(cs.output, timestamp, levelStr, message)
}

// formatLevel converts a log level to its string representation.
func formatLevel(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel:
		return "VRB"
	case core.DebugLevel:
		return "DBG"
	case core.InformationLevel:
		return "INF"
	case core.WarningLevel:
		return "WRN"
	case core.ErrorLevel:
		return "ERR"
	case core.FatalLevel:
		return "FTL"
	default:
		return "UNK"
	}
}