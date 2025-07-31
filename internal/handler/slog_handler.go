package handler

import (
	"context"
	"log/slog"
	"runtime"
	"strings"

	"github.com/willibrandon/mtlog/core"
)

// SlogHandler implements slog.Handler backed by mtlog's logger
type SlogHandler struct {
	logger core.Logger
	attrs  []slog.Attr
	groups []string
}

// NewSlogHandler creates a new slog.Handler that writes to the provided mtlog logger
func NewSlogHandler(logger core.Logger) *SlogHandler {
	return &SlogHandler{
		logger: logger,
	}
}

// Enabled reports whether the handler handles records at the given level
func (h *SlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	mtlogLevel := slogLevelToMtlog(level)
	return h.logger.IsEnabled(mtlogLevel)
}

// Handle handles the Record
func (h *SlogHandler) Handle(ctx context.Context, record slog.Record) error {
	// Convert slog level to mtlog level
	level := slogLevelToMtlog(record.Level)

	// Start building the logger with context
	logger := h.logger

	// Add pre-existing attributes from WithAttrs
	for _, attr := range h.attrs {
		logger = logger.ForContext(h.formatKey(attr.Key), attr.Value.Any())
	}

	// Add attributes from the record
	record.Attrs(func(attr slog.Attr) bool {
		logger = logger.ForContext(h.formatKey(attr.Key), attr.Value.Any())
		return true
	})

	// Add source information if available
	if record.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{record.PC})
		f, _ := fs.Next()
		if f.File != "" {
			logger = logger.ForContext("source", map[string]any{
				"file":     f.File,
				"line":     f.Line,
				"function": f.Function,
			})
		}
	}

	// Log the message at the appropriate level
	switch level {
	case core.VerboseLevel:
		logger.Verbose(record.Message)
	case core.DebugLevel:
		logger.Debug(record.Message)
	case core.InformationLevel:
		logger.Information(record.Message)
	case core.WarningLevel:
		logger.Warning(record.Message)
	case core.ErrorLevel:
		logger.Error(record.Message)
	case core.FatalLevel:
		logger.Fatal(record.Message)
	default:
		logger.Information(record.Message)
	}

	return nil
}

// WithAttrs returns a new Handler whose attributes consist of
// both the receiver's attributes and the arguments
func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &SlogHandler{
		logger: h.logger,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

// WithGroup returns a new Handler with the given group appended to
// the receiver's existing groups
func (h *SlogHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &SlogHandler{
		logger: h.logger,
		attrs:  h.attrs,
		groups: newGroups,
	}
}

// formatKey formats a key with the current group prefix
func (h *SlogHandler) formatKey(key string) string {
	if len(h.groups) == 0 {
		return key
	}
	return strings.Join(h.groups, ".") + "." + key
}

// slogLevelToMtlog converts slog levels to mtlog levels
func slogLevelToMtlog(level slog.Level) core.LogEventLevel {
	switch {
	case level <= slog.LevelDebug-4:
		return core.VerboseLevel
	case level <= slog.LevelDebug:
		return core.DebugLevel
	case level <= slog.LevelInfo:
		return core.InformationLevel
	case level <= slog.LevelWarn:
		return core.WarningLevel
	case level <= slog.LevelError:
		return core.ErrorLevel
	default:
		return core.FatalLevel
	}
}

// MtlogLevelToSlog converts mtlog levels to slog levels
func MtlogLevelToSlog(level core.LogEventLevel) slog.Level {
	switch level {
	case core.VerboseLevel:
		return slog.LevelDebug - 4
	case core.DebugLevel:
		return slog.LevelDebug
	case core.InformationLevel:
		return slog.LevelInfo
	case core.WarningLevel:
		return slog.LevelWarn
	case core.ErrorLevel:
		return slog.LevelError
	case core.FatalLevel:
		return slog.LevelError + 4
	default:
		return slog.LevelInfo
	}
}
