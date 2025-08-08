package analyzer

import (
	"fmt"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Diagnostic IDs for suppression
const (
	DiagIDTemplateMismatch  = "MTLOG001" // Template/argument count mismatch
	DiagIDFormatSpecifier   = "MTLOG002" // Invalid format specifier
	DiagIDDuplicateProperty = "MTLOG003" // Duplicate property names
	DiagIDPropertyNaming    = "MTLOG004" // Property naming (PascalCase)
	DiagIDCapturingHints    = "MTLOG005" // Missing capturing hints
	DiagIDErrorLogging      = "MTLOG006" // Error logging without error value
	DiagIDContextKey        = "MTLOG007" // Context key constant suggestion
	DiagIDDynamicTemplate   = "MTLOG008" // Dynamic template warning
)

// Severity levels for diagnostics
const (
	SeverityError      = "error"
	SeverityWarning    = "warning"
	SeveritySuggestion = "suggestion"
)

// reportDiagnosticWithID reports a diagnostic with ID, severity prefix and config
func reportDiagnosticWithID(pass *analysis.Pass, pos token.Pos, severity string, config *Config, diagID string, format string, args ...interface{}) {
	// Check if this diagnostic is suppressed
	if config != nil && diagID != "" && config.SuppressedDiagnostics[diagID] {
		return // Diagnostic is suppressed
	}
	
	message := fmt.Sprintf(format, args...)
	
	// Downgrade errors to warnings if requested
	if severity == SeverityError && config != nil && config.DowngradeErrors {
		severity = SeverityWarning
	}
	
	// Add severity prefix for non-error diagnostics
	if severity != SeverityError {
		message = severity + ": " + message
	}
	
	// Add diagnostic ID to message
	if diagID != "" {
		message = fmt.Sprintf("[%s] %s", diagID, message)
	}
	
	// Create diagnostic with suggested fixes if applicable
	diag := analysis.Diagnostic{
		Pos:     pos,
		Message: message,
	}
	
	pass.Report(diag)
}