package otel

import "github.com/willibrandon/mtlog/core"

// TestSink is a simple sink for testing
type TestSink struct {
	emit func(*core.LogEvent)
}

func (s *TestSink) Emit(event *core.LogEvent) {
	if s.emit != nil {
		s.emit(event)
	}
}

func (s *TestSink) Close() error {
	return nil
}

func NewTestSink(emitFunc func(*core.LogEvent)) *TestSink {
	return &TestSink{emit: emitFunc}
}