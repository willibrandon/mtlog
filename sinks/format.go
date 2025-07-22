package sinks

import (
	"io"
	"time"
)

// formatBuffer is a reusable buffer for formatting without allocations.
type formatBuffer struct {
	buf [256]byte
}

// formatSimple formats a simple log message into the buffer without allocations.
func (fb *formatBuffer) formatSimple(timestamp time.Time, levelStr, message string) []byte {
	b := fb.buf[:0]
	
	// Format: [YYYY-MM-DD HH:MM:SS.mmm] [LVL] message
	b = append(b, '[')
	b = timestamp.AppendFormat(b, "2006-01-02 15:04:05.000")
	b = append(b, ']', ' ', '[')
	b = append(b, levelStr...)
	b = append(b, ']', ' ')
	b = append(b, message...)
	b = append(b, '\n')
	
	return b
}

// writeSimple writes bytes to a writer without allocations.
func writeSimple(w io.Writer, timestamp time.Time, levelStr, message string) error {
	var fb formatBuffer
	formatted := fb.formatSimple(timestamp, levelStr, message)
	_, err := w.Write(formatted)
	return err
}