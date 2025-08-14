package otel

import (
	"sync"
	
	"github.com/willibrandon/mtlog/core"
)

// errorSink is a sink that reports initialization errors once via selflog
type errorSink struct {
	err      error
	once     sync.Once
	reported bool
}

func (s *errorSink) Emit(event *core.LogEvent) {
	s.once.Do(func() {
		sinkLog.Error("OTLP sink initialization failed: %v", s.err)
		s.reported = true
	})
}

func (s *errorSink) Close() error {
	return nil
}