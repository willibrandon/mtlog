package filters

import (
	"strings"

	"github.com/willibrandon/mtlog/core"
)

// SourceContextLevelFilter filters log events based on source context and minimum level.
type SourceContextLevelFilter struct {
	defaultLevel core.LogEventLevel
	overrides    map[string]core.LogEventLevel
}

// NewSourceContextLevelFilter creates a new filter with source context level overrides.
func NewSourceContextLevelFilter(defaultLevel core.LogEventLevel, overrides map[string]core.LogEventLevel) *SourceContextLevelFilter {
	return &SourceContextLevelFilter{
		defaultLevel: defaultLevel,
		overrides:    overrides,
	}
}

// IsEnabled determines if a log event should be processed based on its source context and level.
func (f *SourceContextLevelFilter) IsEnabled(event *core.LogEvent) bool {
	// Get the source context from properties
	sourceContext, ok := event.Properties["SourceContext"].(string)
	if !ok {
		// No source context, use default level
		return event.Level >= f.defaultLevel
	}
	
	// Find the most specific matching override
	minLevel := f.findMinimumLevel(sourceContext)
	return event.Level >= minLevel
}

// findMinimumLevel finds the most specific minimum level for a source context.
func (f *SourceContextLevelFilter) findMinimumLevel(sourceContext string) core.LogEventLevel {
	// First, check for exact match
	if level, ok := f.overrides[sourceContext]; ok {
		return level
	}
	
	// Check for prefix matches, from most specific to least specific
	// For example, "Microsoft.Hosting.Lifetime" would match:
	// 1. "Microsoft.Hosting.Lifetime" (exact)
	// 2. "Microsoft.Hosting" (prefix)
	// 3. "Microsoft" (prefix)
	var longestMatch string
	var matchLevel core.LogEventLevel
	
	for prefix, level := range f.overrides {
		if strings.HasPrefix(sourceContext, prefix) {
			if len(prefix) > len(longestMatch) {
				longestMatch = prefix
				matchLevel = level
			}
		}
	}
	
	if longestMatch != "" {
		return matchLevel
	}
	
	// No match found, use default level
	return f.defaultLevel
}

// AddOverride adds or updates a source context level override.
func (f *SourceContextLevelFilter) AddOverride(sourceContext string, level core.LogEventLevel) {
	f.overrides[sourceContext] = level
}

// RemoveOverride removes a source context level override.
func (f *SourceContextLevelFilter) RemoveOverride(sourceContext string) {
	delete(f.overrides, sourceContext)
}

// SetDefaultLevel updates the default minimum level.
func (f *SourceContextLevelFilter) SetDefaultLevel(level core.LogEventLevel) {
	f.defaultLevel = level
}