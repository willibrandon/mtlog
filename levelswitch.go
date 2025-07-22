package mtlog

import (
	"sync/atomic"
	
	"github.com/willibrandon/mtlog/core"
)

// LoggingLevelSwitch provides thread-safe, runtime control of the minimum log level.
// It enables dynamic adjustment of logging levels without restarting the application.
type LoggingLevelSwitch struct {
	// level is stored as int32 to enable atomic operations
	// We use int32 because core.LogEventLevel is an int32 underneath
	level int32
}

// NewLoggingLevelSwitch creates a new logging level switch with the specified initial level.
func NewLoggingLevelSwitch(initialLevel core.LogEventLevel) *LoggingLevelSwitch {
	ls := &LoggingLevelSwitch{}
	ls.SetLevel(initialLevel)
	return ls
}

// Level returns the current minimum log level.
func (ls *LoggingLevelSwitch) Level() core.LogEventLevel {
	return core.LogEventLevel(atomic.LoadInt32(&ls.level))
}

// SetLevel updates the minimum log level.
// This operation is thread-safe and takes effect immediately.
func (ls *LoggingLevelSwitch) SetLevel(level core.LogEventLevel) {
	atomic.StoreInt32(&ls.level, int32(level))
}

// IsEnabled returns true if the specified level would be processed
// with the current minimum level setting.
func (ls *LoggingLevelSwitch) IsEnabled(level core.LogEventLevel) bool {
	return level >= ls.Level()
}

// Verbose sets the minimum level to Verbose.
func (ls *LoggingLevelSwitch) Verbose() *LoggingLevelSwitch {
	ls.SetLevel(core.VerboseLevel)
	return ls
}

// Debug sets the minimum level to Debug.
func (ls *LoggingLevelSwitch) Debug() *LoggingLevelSwitch {
	ls.SetLevel(core.DebugLevel)
	return ls
}

// Information sets the minimum level to Information.
func (ls *LoggingLevelSwitch) Information() *LoggingLevelSwitch {
	ls.SetLevel(core.InformationLevel)
	return ls
}

// Warning sets the minimum level to Warning.
func (ls *LoggingLevelSwitch) Warning() *LoggingLevelSwitch {
	ls.SetLevel(core.WarningLevel)
	return ls
}

// Error sets the minimum level to Error.
func (ls *LoggingLevelSwitch) Error() *LoggingLevelSwitch {
	ls.SetLevel(core.ErrorLevel)
	return ls
}

// Fatal sets the minimum level to Fatal.
func (ls *LoggingLevelSwitch) Fatal() *LoggingLevelSwitch {
	ls.SetLevel(core.FatalLevel)
	return ls
}