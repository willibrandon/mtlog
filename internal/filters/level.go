package filters

import (
	"github.com/willibrandon/mtlog/core"
)

// LevelFilter filters log events based on their level.
type LevelFilter struct {
	minimumLevel core.LogEventLevel
}

// NewLevelFilter creates a filter that only allows events at or above the specified level.
func NewLevelFilter(minimumLevel core.LogEventLevel) *LevelFilter {
	return &LevelFilter{
		minimumLevel: minimumLevel,
	}
}

// IsEnabled returns true if the event level is at or above the minimum level.
func (f *LevelFilter) IsEnabled(event *core.LogEvent) bool {
	return event.Level >= f.minimumLevel
}

// MinimumLevelFilter is a convenience function that creates a level filter.
func MinimumLevelFilter(level core.LogEventLevel) core.LogEventFilter {
	return NewLevelFilter(level)
}