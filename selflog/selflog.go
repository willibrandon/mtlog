// Package selflog provides internal diagnostic logging for mtlog.
//
// When enabled, selflog captures internal errors and warnings that would
// otherwise be silently discarded. This is useful for debugging configuration
// issues or understanding why logs aren't appearing as expected.
//
// # Usage
//
// Enable selflog to write to stderr:
//
//	selflog.Enable(os.Stderr)
//	defer selflog.Disable()
//
// Enable with a custom handler:
//
//	selflog.EnableFunc(func(msg string) {
//	    syslog.Warning("mtlog: " + msg)
//	})
//
// For thread-safe file logging:
//
//	f, _ := os.Create("mtlog-debug.log")
//	selflog.Enable(selflog.Sync(f))
//
// # Format
//
// Messages are formatted as:
//
//	2025-01-29T15:30:45Z [component] message details
//
// # Environment Variable
//
// Set MTLOG_SELFLOG to automatically enable on startup:
//   - "stderr" - log to standard error
//   - "stdout" - log to standard output  
//   - "/path/to/file" - log to specified file
package selflog

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// outputWriter holds the current io.Writer (atomic pointer)
	outputWriter atomic.Pointer[io.Writer]
	// outputFunc holds the current function (atomic pointer)
	outputFunc atomic.Pointer[func(string)]
)

// Enable activates self-logging to the provided writer.
// The writer should be thread-safe or wrapped with Sync().
func Enable(w io.Writer) {
	if w == nil {
		return
	}
	outputFunc.Store(nil)
	outputWriter.Store(&w)
}

// EnableFunc activates self-logging using a callback function.
// The function will be called with formatted log messages.
func EnableFunc(fn func(string)) {
	if fn == nil {
		return
	}
	outputWriter.Store(nil)
	outputFunc.Store(&fn)
}

// Disable deactivates self-logging.
func Disable() {
	outputWriter.Store(nil)
	outputFunc.Store(nil)
}

// Printf logs an internal diagnostic message.
// This is called by mtlog internals and can be called by custom sinks.
// The format string should include the component in square brackets,
// e.g., "[console] write failed: %v"
func Printf(format string, args ...interface{}) {
	// Fast path - check if disabled first to avoid allocation
	w := outputWriter.Load()
	fn := outputFunc.Load()
	if w == nil && fn == nil {
		return // Disabled
	}

	// Format message once
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().UTC().Format(time.RFC3339)
	line := timestamp + " " + msg

	// Write to output
	if w != nil {
		fmt.Fprintln(*w, line)
	} else if fn != nil {
		(*fn)(line)
	}
}

// IsEnabled returns true if selflog is currently enabled.
// Use this to avoid formatting costs when disabled:
//
//	if selflog.IsEnabled() {
//	    selflog.Printf("[sink] processed %d events", count)
//	}
func IsEnabled() bool {
	return outputWriter.Load() != nil || outputFunc.Load() != nil
}

// syncWriter wraps an io.Writer to make it thread-safe
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// Sync wraps a writer to make it thread-safe.
// Use this when enabling file output or other non-synchronized writers.
func Sync(w io.Writer) io.Writer {
	return &syncWriter{w: w}
}

// init checks for MTLOG_SELFLOG environment variable
func init() {
	if dest := os.Getenv("MTLOG_SELFLOG"); dest != "" {
		switch dest {
		case "stderr":
			Enable(os.Stderr)
		case "stdout":
			Enable(os.Stdout)
		default:
			if f, err := os.OpenFile(dest, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
				Enable(Sync(f))
			}
		}
	}
}