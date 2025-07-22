package core

// LogEventLevel specifies the severity of a log event.
type LogEventLevel int

const (
	// VerboseLevel is the most detailed logging level.
	VerboseLevel LogEventLevel = iota
	
	// DebugLevel is for debugging information.
	DebugLevel
	
	// InformationLevel is for informational messages.
	InformationLevel
	
	// WarningLevel is for warnings.
	WarningLevel
	
	// ErrorLevel is for errors.
	ErrorLevel
	
	// FatalLevel is for fatal errors.
	FatalLevel
)