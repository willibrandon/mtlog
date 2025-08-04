package analyzer

// Config holds configuration options for the analyzer
type Config struct {
	// CommonContextKeys defines additional context keys that should be considered "common"
	// and trigger a suggestion to define as constants
	CommonContextKeys []string
	
	// DisabledChecks allows specific checks to be disabled
	DisabledChecks map[string]bool
	
	// StrictMode enables stricter checking (e.g., format specifier validation)
	StrictMode bool
	
	// IgnoreDynamicTemplates suppresses warnings for dynamic (non-literal) template strings
	IgnoreDynamicTemplates bool
	
	// StrictLoggerTypes disables lenient logger type checking
	StrictLoggerTypes bool
	
	// DowngradeErrors downgrades errors to warnings for CI migration
	DowngradeErrors bool
	
	// DisableAll is the global kill switch - disables all mtlog diagnostics
	DisableAll bool
	
	// SuppressedDiagnostics allows specific diagnostics to be suppressed by ID
	SuppressedDiagnostics map[string]bool
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		CommonContextKeys:      []string{"user_id", "request_id", "trace_id", "span_id"},
		DisabledChecks:         make(map[string]bool),
		StrictMode:             false,
		IgnoreDynamicTemplates: false,
		StrictLoggerTypes:      false,
		DowngradeErrors:        false,
		DisableAll:             false,
		SuppressedDiagnostics:  make(map[string]bool),
	}
}