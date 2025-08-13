package otel

import (
	"sync"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

// errorSink is a sink that reports initialization errors once via selflog
type errorSink struct {
	err      error
	once     sync.Once
	reported bool
}

func (s *errorSink) Emit(event *core.LogEvent) {
	s.once.Do(func() {
		if selflog.IsEnabled() {
			selflog.Printf("[otel] OTLP sink initialization failed: %v", s.err)
		}
		s.reported = true
	})
}

func (s *errorSink) Close() error {
	return nil
}