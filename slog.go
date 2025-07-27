package mtlog

import (
	"log/slog"

	"github.com/willibrandon/mtlog/internal/handler"
)

// NewSlogLogger creates a new slog.Logger backed by mtlog
func NewSlogLogger(options ...Option) *slog.Logger {
	logger := New(options...)
	return slog.New(handler.NewSlogHandler(logger))
}

// AsSlogHandler returns the current logger as an slog.Handler
func (l *logger) AsSlogHandler() slog.Handler {
	return handler.NewSlogHandler(l)
}