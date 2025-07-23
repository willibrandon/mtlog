package mtlog

import (
	"github.com/go-logr/logr"
	"github.com/willibrandon/mtlog/handler"
)

// NewLogrLogger creates a new logr.Logger backed by mtlog
func NewLogrLogger(options ...Option) logr.Logger {
	logger := New(options...)
	return logr.New(handler.NewLogrSink(logger))
}

// AsLogrSink returns the current logger as a logr.LogSink
func (l *logger) AsLogrSink() logr.LogSink {
	return handler.NewLogrSink(l)
}