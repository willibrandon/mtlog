// Package analyzer implements a static analyzer for mtlog usage.
//
// The analyzer detects common mistakes when using mtlog, including:
//   - Template/argument count mismatches
//   - Invalid format specifiers
//   - Duplicate property names
//   - Poor property naming conventions
//   - Missing capturing hints for complex types
//   - Error logging without error values
package analyzer

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/ast/astutil"
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

// Environment variable for diagnostic suppression
const EnvMtlogSuppress = "MTLOG_SUPPRESS"

func init() {
	// Register flags directly on the analyzer
	Analyzer.Flags.Bool("strict", false, "enable strict format specifier validation")
	Analyzer.Flags.String("common-keys", "", "comma-separated list of common context keys to add to defaults (user_id,request_id,trace_id,span_id)")
	Analyzer.Flags.String("disable", "", "comma-separated list of checks to disable")
	Analyzer.Flags.Bool("ignore-dynamic-templates", false, "suppress warnings for dynamic (non-literal) template strings")
	Analyzer.Flags.Bool("strict-logger-types", false, "disable lenient logger type checking (require exact mtlog types)")
	Analyzer.Flags.Bool("downgrade-errors", false, "downgrade errors to warnings (useful for CI environments during migration)")
	Analyzer.Flags.Bool("disable-all", false, "disable all mtlog diagnostics (global kill switch)")
	Analyzer.Flags.String("suppress", "", "comma-separated list of diagnostic IDs to suppress (e.g., MTLOG001,MTLOG004)")
}

// Analyzer is the mtlog analyzer that checks for common logging mistakes.
// It can be used with go vet or as a standalone tool.
var Analyzer = &analysis.Analyzer{
	Name:     "mtlog",
	Doc:      "check for common mtlog mistakes",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}


// Template cache to avoid redundant parsing within a single pass
type templateCache struct {
	cache map[string]templateInfo
}

type templateInfo struct {
	properties []string
	err        error
}

// Severity levels for diagnostics
const (
	SeverityError      = "error"
	SeverityWarning    = "warning"
	SeveritySuggestion = "suggestion"
)

// getBoolFlag is a helper function to lookup and extract boolean flag values
func getBoolFlag(pass *analysis.Pass, flagName string) (bool, bool) {
	if f := pass.Analyzer.Flags.Lookup(flagName); f != nil {
		if getter, ok := f.Value.(flag.Getter); ok {
			if b, ok := getter.Get().(bool); ok {
				return b, true
			}
		}
	}
	return false, false
}

// getStringFlag is a helper function to lookup and extract string flag values
func getStringFlag(pass *analysis.Pass, flagName string) (string, bool) {
	if f := pass.Analyzer.Flags.Lookup(flagName); f != nil {
		if getter, ok := f.Value.(flag.Getter); ok {
			if s, ok := getter.Get().(string); ok {
				return s, true
			}
		}
	}
	return "", false
}

// run executes the analyzer on the given pass.
func run(pass *analysis.Pass) (interface{}, error) {
	// Build configuration from flags
	defaultConfig := DefaultConfig()
	config := &defaultConfig
	
	// Check for global kill switch first
	if disableAll, found := getBoolFlag(pass, "disable-all"); found && disableAll {
		// Global kill switch enabled - skip all analysis
		return nil, nil
	}
	
	// Get flag values
	if strict, found := getBoolFlag(pass, "strict"); found {
		config.StrictMode = strict
	}
	
	if commonKeys, found := getStringFlag(pass, "common-keys"); found && commonKeys != "" {
		// Append to existing defaults instead of replacing
		config.CommonContextKeys = append(config.CommonContextKeys, strings.Split(commonKeys, ",")...)
	}
	
	if disable, found := getStringFlag(pass, "disable"); found && disable != "" {
		for _, check := range strings.Split(disable, ",") {
			// Normalize to lowercase for case-insensitive matching
			config.DisabledChecks[strings.ToLower(strings.TrimSpace(check))] = true
		}
	}
	
	if ignoreDynamic, found := getBoolFlag(pass, "ignore-dynamic-templates"); found {
		config.IgnoreDynamicTemplates = ignoreDynamic
	}
	
	if strictTypes, found := getBoolFlag(pass, "strict-logger-types"); found {
		config.StrictLoggerTypes = strictTypes
	}
	
	if downgrade, found := getBoolFlag(pass, "downgrade-errors"); found {
		config.DowngradeErrors = downgrade
	}
	
	// Check environment variable for suppression (for VS Code integration)
	if envSuppress := os.Getenv(EnvMtlogSuppress); envSuppress != "" {
		// Parse comma-separated diagnostic IDs from environment
		for _, id := range strings.Split(envSuppress, ",") {
			trimmedID := strings.TrimSpace(strings.ToUpper(id))
			config.SuppressedDiagnostics[trimmedID] = true
		}
	}
	
	// Also check flags for suppression
	if suppress, found := getStringFlag(pass, "suppress"); found && suppress != "" {
		// Parse comma-separated diagnostic IDs
		for _, id := range strings.Split(suppress, ",") {
			config.SuppressedDiagnostics[strings.TrimSpace(strings.ToUpper(id))] = true
		}
	}
	
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	cache := &templateCache{cache: make(map[string]templateInfo)}

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		call := n.(*ast.CallExpr)
		
		// Check if this is an mtlog-related call (logging or context methods)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		
		// Quick check: is this potentially an mtlog logger call?
		if !isPotentialMtlogCall(pass, sel, config) {
			return
		}
		
		// Process all method calls on what could be a logger
		// Individual checks will decide if they apply
		runWithAllChecks(pass, call, cache, config)
	})

	return nil, nil
}

// isPotentialMtlogCall checks if the selector expression could be an mtlog logger call
func isPotentialMtlogCall(pass *analysis.Pass, sel *ast.SelectorExpr, config *Config) bool {
	// Check if the method name is one we care about
	methodName := sel.Sel.Name
	if !isRelevantMethod(methodName) {
		return false
	}
	
	// Try to check the receiver type
	recvType := pass.TypesInfo.TypeOf(sel.X)
	if recvType == nil {
		// If we can't determine the type, be conservative and check it
		return true
	}
	
	// Check if it's an mtlog logger type
	result := isMtlogLoggerType(recvType, config)
	return result
}

// isRelevantMethod checks if the method name is one we analyze
func isRelevantMethod(name string) bool {
	relevantMethods := map[string]bool{
		// Logging methods
		"Verbose": true, "V": true,
		"Debug": true, "D": true,
		"Information": true, "Info": true, "I": true,
		"Warning": true, "Warn": true, "W": true,
		"Error": true, "Err": true, "E": true,
		"Fatal": true, "F": true,
		// Context methods
		"ForContext": true,
	}
	return relevantMethods[name]
}

// isMtlogLoggerType checks if the type is an mtlog logger
func isMtlogLoggerType(t types.Type, config *Config) bool {
	// Handle pointer types
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	
	// Handle type aliases (e.g., type MyLogger = Logger)
	if alias, ok := t.(*types.Alias); ok {
		// Get the actual type the alias refers to
		t = alias.Rhs()
	}
	
	// Check if it's a named type
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	
	// Check the package path for real mtlog loggers
	pkg := named.Obj().Pkg()
	if pkg != nil {
		pkgPath := pkg.Path()
		// Accept mtlog package paths
		if strings.Contains(pkgPath, "github.com/willibrandon/mtlog") ||
		   strings.HasSuffix(pkgPath, "/mtlog") {
			return true
		}
	}
	
	// In strict mode, only accept exact mtlog types
	if config.StrictLoggerTypes {
		return false
	}
	
	// Check the type name - must be "Logger" or end with "Logger"
	typeName := named.Obj().Name()
	if typeName == "Logger" || strings.HasSuffix(typeName, "Logger") {
		// For test files and examples, check if it has the expected methods
		// This is more lenient but still validates it's a logger-like type
		if hasLoggerMethods(t) {
			return true
		}
	}
	
	return false
}

// hasLoggerMethods checks if a type has the expected mtlog Logger methods
func hasLoggerMethods(t types.Type) bool {
	// Check for at least one key logging method
	// We're being lenient here - if it has Error or any other logging method, it's probably a logger
	loggingMethods := []string{"Error", "Warning", "Information", "Debug", "Fatal", "Verbose", "E", "W", "I", "D", "F", "V"}
	
	for _, methodName := range loggingMethods {
		method, _, _ := types.LookupFieldOrMethod(t, true, nil, methodName)
		if method == nil {
			continue // Try next method
		}
		
		// Verify it's a method (not a field)
		fn, ok := method.(*types.Func)
		if !ok {
			continue
		}
		
		// Basic signature check: should accept (string, ...interface{})
		sig, ok := fn.Type().(*types.Signature)
		if !ok || sig.Params().Len() < 1 {
			continue
		}
		
		// Found at least one valid logging method
		return true
	}
	
	return false
}

// isLogCall checks if the given call expression is a call to an mtlog logging method.
// It recognizes both full method names (Information, Warning) and short aliases (Info, Warn).
func isLogCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check if the method is one of the logging methods
	method := sel.Sel.Name
	loggingMethods := []string{
		"Verbose", "V",
		"Debug", "D",
		"Information", "Info", "I",
		"Warning", "Warn", "W",
		"Error", "Err", "E",
		"Fatal", "F",
	}

	for _, m := range loggingMethods {
		if method == m {
			return true
		}
	}
	return false
}

// getMethodName extracts the method name from a call expression.
func getMethodName(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return sel.Sel.Name
}

// checkTemplateArguments validates that the number of properties in the template
// matches the number of arguments provided to the logging method.
func checkTemplateArguments(pass *analysis.Pass, call *ast.CallExpr, cache *templateCache, config *Config) {
	if len(call.Args) < 1 {
		return
	}

	// Get the template string
	templateArg := call.Args[0]
	lit, ok := templateArg.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		// Non-literal template
		if !config.IgnoreDynamicTemplates {
			reportDiagnosticWithID(pass, templateArg.Pos(), SeverityWarning, config, DiagIDDynamicTemplate,
				"dynamic template strings are not analyzed")
		}
		return
	}

	// If template check is disabled, skip the rest
	if config.DisabledChecks["template"] {
		return
	}

	// Parse the template string (remove quotes)
	template := lit.Value
	if len(template) >= 2 {
		template = template[1 : len(template)-1]
	}

	// Extract property names from template (with caching)
	info := cache.getTemplateInfo(template, config)
	if info.err != nil {
		reportDiagnosticWithID(pass, lit.Pos(), SeverityError, config, DiagIDTemplateMismatch,
			"invalid template: %v", info.err)
		return
	}
	
	properties := info.properties
	
	// Count arguments (excluding the template)
	argCount := len(call.Args) - 1
	
	// For Error/E methods, check if last argument is an error
	methodName := getMethodName(call)
	if isErrorMethod(methodName) && argCount == len(properties) + 1 {
		// Verify the last argument is an error type
		lastArg := call.Args[len(call.Args)-1]
		if argType := pass.TypesInfo.TypeOf(lastArg); argType != nil {
			if !isErrorType(argType) {
				reportDiagnosticWithID(pass, lastArg.Pos(), SeverityError, config, DiagIDErrorLogging,
					"last argument to %s method should be an error, got %s", 
					methodName, argType)
			}
		}
		return
	}
	
	// Check if argument count matches property count
	if len(properties) != argCount {
		// Create diagnostic with suggested fix
		message := fmt.Sprintf("template has %d properties but %d arguments provided", 
			len(properties), argCount)
		
		// Apply downgrade if needed
		severity := SeverityError
		if config != nil && config.DowngradeErrors {
			severity = SeverityWarning
		}
		
		// Add severity prefix for non-error diagnostics
		if severity != SeverityError {
			message = severity + ": " + message
		}
		
		// Add diagnostic ID to message
		message = fmt.Sprintf("[%s] %s", DiagIDTemplateMismatch, message)
		
		diagnostic := analysis.Diagnostic{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: message,
		}
		
		// Add suggested fix based on the mismatch
		if argCount < len(properties) {
			// Too few arguments - suggest adding placeholders
			missingCount := len(properties) - argCount
			placeholders := make([]string, missingCount)
			for i := 0; i < missingCount; i++ {
				placeholders[i] = "nil /* TODO: provide value for " + properties[argCount+i] + " */"
			}
			
			diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
				Message: fmt.Sprintf("Add %d missing argument(s)", missingCount),
				TextEdits: []analysis.TextEdit{{
					Pos:     call.End() - 1, // Just before the closing paren
					End:     call.End() - 1,
					NewText: []byte(", " + strings.Join(placeholders, ", ")),
				}},
			}}
		} else if argCount > len(properties) {
			// Too many arguments - suggest removing extras
			// Find the position after the last valid argument
			if len(properties) > 0 && len(call.Args) > len(properties)+1 {
				endArg := call.Args[len(call.Args)-1]
				
				diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
					Message: fmt.Sprintf("Remove %d extra argument(s)", argCount-len(properties)),
					TextEdits: []analysis.TextEdit{{
						Pos:     call.Args[len(properties)].End(), // After last valid arg
						End:     endArg.End(),
						NewText: []byte(""),
					}},
				}}
			}
		}
		
		// Check if this diagnostic is suppressed
		if config != nil && config.SuppressedDiagnostics[DiagIDTemplateMismatch] {
			return // Diagnostic is suppressed
		}
		
		pass.Report(diagnostic)
		return
	}

	// Check for invalid format specifiers
	for _, prop := range properties {
		if err := validateFormatSpecifier(prop, config); err != nil {
			reportDiagnosticWithID(pass, call.Pos(), SeverityError, config, DiagIDFormatSpecifier,
				"invalid format specifier in property '%s': %v", prop, err)
		}
	}
}

// isErrorMethod checks if the method name is an error logging method
func isErrorMethod(name string) bool {
	return name == "Error" || name == "Err" || name == "E" ||
	       strings.HasPrefix(name, "Error") // Covers Errorw, Errorf, etc.
}

// getTemplateInfo extracts template info with caching
func (tc *templateCache) getTemplateInfo(template string, config *Config) templateInfo {
	// Include strict mode in cache key to avoid cross-mode pollution
	cacheKey := template
	if config.StrictMode {
		cacheKey = "strict:" + template
	}
	
	if info, ok := tc.cache[cacheKey]; ok {
		return info
	}
	
	properties, err := extractProperties(template)
	info := templateInfo{properties: properties, err: err}
	tc.cache[cacheKey] = info
	return info
}

// extractProperties parses a message template and extracts all property names.
// It handles escaped braces ({{) and returns property names including any format specifiers.
func extractProperties(template string) ([]string, error) {
	var properties []string
	inProperty := false
	propertyStart := 0
	braceDepth := 0
	
	for i := 0; i < len(template); i++ {
		if template[i] == '{' {
			// Check for escaped brace
			if i+1 < len(template) && template[i+1] == '{' {
				i++ // Skip escaped brace
				continue
			}
			if !inProperty {
				inProperty = true
				propertyStart = i + 1
				braceDepth = 1
			} else {
				braceDepth++
			}
		} else if template[i] == '}' && inProperty {
			braceDepth--
			if braceDepth == 0 {
				// Extract property name
				prop := template[propertyStart:i]
				if prop != "" {
					properties = append(properties, prop)
				}
				inProperty = false
			}
		}
	}
	
	// Check for unclosed property
	if inProperty {
		return nil, fmt.Errorf("unclosed property brace at position %d", propertyStart-1)
	}
	
	return properties, nil
}

// Pre-compiled list of valid format specifiers
var validFormats = map[string]bool{
	// Number formats
	"000": true, "0000": true, "00000": true,
	// Float formats
	"F": true, "F0": true, "F1": true, "F2": true, "F3": true, "F4": true,
	// Percentage
	"P": true, "P0": true, "P1": true, "P2": true,
	// Exponential
	"E": true, "E0": true, "E1": true, "E2": true,
	// General
	"G": true, "G0": true, "G1": true, "G2": true,
	// Hex
	"X": true, "X2": true, "X4": true, "X8": true,
	"x": true, "x2": true, "x4": true, "x8": true,
}

// validateFormatSpecifier checks if a property's format specifier is valid.
// Returns nil if the format is valid or no format specifier is present.
func validateFormatSpecifier(property string, config *Config) error {
	// Split property and format specifier
	parts := strings.SplitN(property, ":", 2)
	if len(parts) < 2 {
		return nil // No format specifier
	}
	
	format := parts[1]
	
	// Check if it's a known format
	if validFormats[format] {
		return nil
	}
	
	// Check if it's a time format (contains time format characters)
	timeChars := []string{"HH", "mm", "ss", "yyyy", "MM", "dd"}
	for _, tc := range timeChars {
		if strings.Contains(format, tc) {
			return nil
		}
	}
	
	// Check if it's an alignment specifier
	if len(format) > 0 && (format[0] == '-' || (format[0] >= '0' && format[0] <= '9')) {
		return nil
	}
	
	// In strict mode, unknown formats are errors
	if config.StrictMode {
		return fmt.Errorf("unknown format specifier: %s", format)
	}
	
	// In lenient mode, unknown formats are allowed
	// This behavior is documented in the README
	return nil
}

// reportDiagnostic reports a diagnostic with severity prefix
func reportDiagnostic(pass *analysis.Pass, pos token.Pos, severity string, format string, args ...interface{}) {
	reportDiagnosticWithConfig(pass, pos, severity, nil, format, args...)
}

// reportDiagnosticWithConfig reports a diagnostic with severity prefix and config
func reportDiagnosticWithConfig(pass *analysis.Pass, pos token.Pos, severity string, config *Config, format string, args ...interface{}) {
	reportDiagnosticWithID(pass, pos, severity, config, "", format, args...)
}

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

// isErrorType checks if the type is an error type
func isErrorType(t types.Type) bool {
	errorType := types.Universe.Lookup("error").Type()
	return types.Implements(t, errorType.Underlying().(*types.Interface))
}

// runWithAllChecks runs all applicable checks on a method call
func runWithAllChecks(pass *analysis.Pass, call *ast.CallExpr, cache *templateCache, config *Config) {
	// Always check context usage (applies to ForContext)
	if !config.DisabledChecks["context"] {
		checkContextUsage(pass, call, config)
	}
	
	// Check if this is a logging method
	if !isLogCall(call) {
		return
	}
	
	// Logging-specific checks
	if len(call.Args) < 1 {
		return
	}

	// Always check template arguments (handles dynamic templates)
	checkTemplateArguments(pass, call, cache, config)

	// Get the template string for additional checks
	templateArg := call.Args[0]
	lit, ok := templateArg.(*ast.BasicLit)
	if !ok {
		// Non-literal template, skip remaining checks
		return
	}

	template := strings.Trim(lit.Value, `"`)
	if !config.DisabledChecks["duplicate"] {
		checkDuplicatePropertiesWithConfig(pass, call, template, config)
	}
	if !config.DisabledChecks["naming"] {
		checkPropertyNamingWithConfig(pass, call, template, config)
	}
	if !config.DisabledChecks["capturing"] {
		checkCapturingUsageWithConfig(pass, call, template, config)
	}
	if !config.DisabledChecks["error"] {
		checkErrorLoggingWithConfig(pass, call, config)
	}
}


// checkDuplicatePropertiesWithConfig checks for duplicate property names in templates
func checkDuplicatePropertiesWithConfig(pass *analysis.Pass, call *ast.CallExpr, template string, config *Config) {
	properties, _ := extractProperties(template)
	seen := make(map[string]bool)
	
	for _, prop := range properties {
		// Remove format specifier for comparison
		propName := strings.SplitN(prop, ":", 2)[0]
		// Remove capturing hints
		propName = strings.TrimPrefix(propName, "@")
		propName = strings.TrimPrefix(propName, "$")
		
		if seen[propName] {
			reportDiagnosticWithID(pass, call.Pos(), SeverityError, config, DiagIDDuplicateProperty,
				"duplicate property '%s' in template", propName)
		}
		seen[propName] = true
	}
}


// checkPropertyNamingWithConfig checks for common naming issues
func checkPropertyNamingWithConfig(pass *analysis.Pass, call *ast.CallExpr, template string, config *Config) {
	properties, _ := extractProperties(template)
	
	// Track which properties we've already suggested fixes for
	suggested := make(map[string]bool)
	
	for _, prop := range properties {
		// Remove format specifier
		propName := strings.SplitN(prop, ":", 2)[0]
		// Remove capturing hints
		originalName := propName
		propName = strings.TrimPrefix(propName, "@")
		propName = strings.TrimPrefix(propName, "$")
		
		// Check for empty property
		if propName == "" {
			reportDiagnosticWithID(pass, call.Pos(), SeverityError, config, DiagIDPropertyNaming,
				"empty property name in template")
			continue
		}
		
		// Check for spaces in property names
		if strings.Contains(propName, " ") {
			reportDiagnosticWithID(pass, call.Pos(), SeverityError, config, DiagIDPropertyNaming,
				"property name '%s' contains spaces", propName)
			continue // Skip other checks for invalid names
		}
		
		// Check for starting with number
		if len(propName) > 0 && propName[0] >= '0' && propName[0] <= '9' {
			reportDiagnosticWithID(pass, call.Pos(), SeverityError, config, DiagIDPropertyNaming,
				"property name '%s' starts with a number", propName)
			continue // Skip other checks for invalid names
		}
		
		// Suggest PascalCase for property names
		if len(propName) > 0 && propName[0] >= 'a' && propName[0] <= 'z' {
			// Skip OTEL-style dotted properties (they follow different conventions)
			if strings.Contains(propName, ".") {
				continue
			}
			
			// Skip if we've already suggested a fix for this property
			if suggested[propName] {
				continue
			}
			suggested[propName] = true
			
			// Create suggested fix to convert to PascalCase
			pascalCase := toPascalCase(propName)
			
			// Get the literal string to create proper replacement
			if lit, ok := call.Args[0].(*ast.BasicLit); ok {
				oldTemplate := lit.Value
				// Replace the property in the template, preserving quotes
				newTemplate := strings.Replace(oldTemplate, "{"+originalName, "{"+strings.Replace(originalName, propName, pascalCase, 1), -1)
				
				// Check if diagnostic is suppressed
				if !config.SuppressedDiagnostics[DiagIDPropertyNaming] {
					diag := analysis.Diagnostic{
						Pos:     call.Pos(),
						Message: fmt.Sprintf("[%s] %s: consider using PascalCase for property '%s'", DiagIDPropertyNaming, SeveritySuggestion, propName),
						SuggestedFixes: []analysis.SuggestedFix{{
							Message: fmt.Sprintf("Change '%s' to '%s'", propName, pascalCase),
							TextEdits: []analysis.TextEdit{{
								Pos:     lit.Pos(),
								End:     lit.End(),
								NewText: []byte(newTemplate),
							}},
						}},
					}
					pass.Report(diag)
				}
			} else {
				reportDiagnosticWithID(pass, call.Pos(), SeveritySuggestion, config, DiagIDPropertyNaming,
					"consider using PascalCase for property '%s'", propName)
			}
		}
	}
}


// checkCapturingUsageWithConfig checks for proper use of @ and $ prefixes
func checkCapturingUsageWithConfig(pass *analysis.Pass, call *ast.CallExpr, template string, config *Config) {
	if len(call.Args) < 2 {
		return
	}
	
	properties, _ := extractProperties(template)
	
	for i, prop := range properties {
		argIndex := i + 1 // Skip template argument
		if argIndex >= len(call.Args) {
			break
		}
		
		arg := call.Args[argIndex]
		argType := pass.TypesInfo.TypeOf(arg)
		
		if argType == nil {
			continue
		}
		
		propName := strings.SplitN(prop, ":", 2)[0]
		
		// Check @ prefix for complex types
		if strings.HasPrefix(propName, "@") {
			// Good - using @ for complex type
			if isBasicType(argType) {
				// Create suggested fix to remove @ prefix
				if lit, ok := call.Args[0].(*ast.BasicLit); ok {
					oldTemplate := lit.Value
					newProp := strings.TrimPrefix(propName, "@")
					// Replace in the template, preserving quotes
					newTemplate := strings.Replace(oldTemplate, "{"+propName, "{"+newProp, -1)
					
					// Check if diagnostic is suppressed
					if !config.SuppressedDiagnostics[DiagIDCapturingHints] {
						diag := analysis.Diagnostic{
							Pos:     arg.Pos(),
							Message: fmt.Sprintf("[%s] %s: using @ prefix for basic type %s, consider removing prefix", DiagIDCapturingHints, SeverityWarning, argType),
							SuggestedFixes: []analysis.SuggestedFix{{
								Message: fmt.Sprintf("Remove @ prefix from '%s'", propName),
								TextEdits: []analysis.TextEdit{{
									Pos:     lit.Pos(),
									End:     lit.End(),
									NewText: []byte(newTemplate),
								}},
							}},
						}
						pass.Report(diag)
					}
				} else {
					reportDiagnosticWithID(pass, arg.Pos(), SeverityWarning, config, DiagIDCapturingHints,
						"using @ prefix for basic type %s, consider removing prefix", argType)
				}
			}
		} else if strings.HasPrefix(propName, "$") {
			// $ is for scalar rendering - make sure it's appropriate
			if !isBasicType(argType) && !isStringer(argType) {
				// Create suggested fix to change $ to @
				if lit, ok := call.Args[0].(*ast.BasicLit); ok {
					oldTemplate := lit.Value
					newProp := "@" + strings.TrimPrefix(propName, "$")
					// Replace in the template, preserving quotes
					newTemplate := strings.Replace(oldTemplate, "{"+propName, "{"+newProp, -1)
					
					// Check if diagnostic is suppressed
					if !config.SuppressedDiagnostics[DiagIDCapturingHints] {
						diag := analysis.Diagnostic{
							Pos:     arg.Pos(),
							Message: fmt.Sprintf("[%s] %s: using $ prefix for complex type %s, consider using @ for capturing", DiagIDCapturingHints, SeverityWarning, argType),
							SuggestedFixes: []analysis.SuggestedFix{{
								Message: fmt.Sprintf("Change '$' to '@' prefix for '%s'", propName),
								TextEdits: []analysis.TextEdit{{
									Pos:     lit.Pos(),
									End:     lit.End(),
									NewText: []byte(newTemplate),
								}},
							}},
						}
						pass.Report(diag)
					}
				} else {
					reportDiagnosticWithID(pass, arg.Pos(), SeverityWarning, config, DiagIDCapturingHints,
						"using $ prefix for complex type %s, consider using @ for capturing", argType)
				}
			}
		} else {
			// No prefix - suggest @ for complex types
			if !isBasicType(argType) && !isTimeType(argType) && !isStringer(argType) && !isErrorType(argType) {
				// Create suggested fix to add @ prefix
				if lit, ok := call.Args[0].(*ast.BasicLit); ok {
					oldTemplate := lit.Value
					newProp := "@" + propName
					// Replace in the template, preserving quotes
					newTemplate := strings.Replace(oldTemplate, "{"+propName, "{"+newProp, -1)
					
					// Check if diagnostic is suppressed
					if !config.SuppressedDiagnostics[DiagIDCapturingHints] {
						diag := analysis.Diagnostic{
							Pos:     arg.Pos(),
							Message: fmt.Sprintf("[%s] %s: consider using @ prefix for complex type %s to enable capturing", DiagIDCapturingHints, SeveritySuggestion, argType),
							SuggestedFixes: []analysis.SuggestedFix{{
								Message: fmt.Sprintf("Add @ prefix to '%s' for capturing", propName),
								TextEdits: []analysis.TextEdit{{
									Pos:     lit.Pos(),
									End:     lit.End(),
									NewText: []byte(newTemplate),
								}},
							}},
						}
						pass.Report(diag)
					}
				} else {
					reportDiagnosticWithID(pass, arg.Pos(), SeveritySuggestion, config, DiagIDCapturingHints,
						"consider using @ prefix for complex type %s to enable capturing", argType)
				}
			}
		}
	}
}

// checkContextUsage checks for ForContext usage patterns
func checkContextUsage(pass *analysis.Pass, call *ast.CallExpr, config *Config) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ForContext" {
		return
	}
	
	// Check if ForContext is called with string literals
	if len(call.Args) >= 2 {
		// Check if first argument (key) is a string literal
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Value != "" {
			key := strings.Trim(lit.Value, `"`)
			// Suggest using constants for common keys (from config)
			for _, common := range config.CommonContextKeys {
				if strings.EqualFold(key, common) {
					// Generate a constant name from the key
					constName := "ctx" + toPascalCase(key)
					
					// Check if diagnostic is suppressed
					if !config.SuppressedDiagnostics[DiagIDContextKey] {
						diagnostic := analysis.Diagnostic{
							Pos:     lit.Pos(),
							End:     lit.End(),
							Message: fmt.Sprintf("[%s] suggestion: consider defining a constant for commonly used context key '%s'", DiagIDContextKey, key),
						}
						
						// Add suggested fix to replace with constant
						diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
							Message: fmt.Sprintf("Replace with constant %s", constName),
							TextEdits: []analysis.TextEdit{{
								Pos:     lit.Pos(),
								End:     lit.End(),
								NewText: []byte(constName),
							}},
						}}
						
						pass.Report(diagnostic)
					}
					break
				}
			}
		}
	}
}


// checkErrorLoggingWithConfig checks for proper error logging patterns
func checkErrorLoggingWithConfig(pass *analysis.Pass, call *ast.CallExpr, config *Config) {
	methodName := getMethodName(call)
	if !isErrorMethod(methodName) {
		return
	}
	
	// Check if an error is being logged
	hasError := false
	for i := 1; i < len(call.Args); i++ {
		argType := pass.TypesInfo.TypeOf(call.Args[i])
		if argType != nil && isErrorType(argType) {
			hasError = true
			break
		}
	}
	
	if !hasError {
		// Check if diagnostic is suppressed
		if config.SuppressedDiagnostics[DiagIDErrorLogging] {
			return
		}
		
		// Create diagnostic with suggested fix
		diagnostic := analysis.Diagnostic{
			Pos:     call.Pos(),
			End:     call.End(),
			Message: fmt.Sprintf("[%s] %s: Error level log without error value, consider including the error or using Warning level", DiagIDErrorLogging, SeveritySuggestion),
		}
		
		// Find error variable in scope or use nil
		errorVar := findErrorVariableInScope(pass, call)
		errorParam := errorVar
		if errorParam == "" {
			errorParam = "nil"
		}
		
		// Add suggested fix to add error parameter
		diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
			Message: "Add error parameter",
			TextEdits: []analysis.TextEdit{{
				Pos:     call.End() - 1, // Just before the closing paren
				End:     call.End() - 1,
				NewText: []byte(", " + errorParam),
			}},
		}}
		
		// If using nil, add TODO comment
		if errorParam == "nil" {
			// Find the end of the current line
			pos := pass.Fset.Position(call.Pos())
			
			// Find the actual end of the line by scanning forward
			src, err := os.ReadFile(pos.Filename)
			if err == nil {
				lines := strings.Split(string(src), "\n")
				if pos.Line <= len(lines) {
					currentLine := lines[pos.Line-1]
					
					// Check if there's already a comment on this line
					hasComment := strings.Contains(currentLine, "//")
					
					if hasComment {
						// Put TODO on next line with proper indentation
						indent := ""
						for _, r := range currentLine {
							if r == ' ' || r == '\t' {
								indent += string(r)
							} else {
								break
							}
						}
						
						// Find position at end of current line
						lineStart := call.Pos() - token.Pos(pos.Column-1)
						lineEnd := lineStart + token.Pos(len(currentLine))
						
						diagnostic.SuggestedFixes[0].TextEdits = append(diagnostic.SuggestedFixes[0].TextEdits, analysis.TextEdit{
							Pos:     lineEnd,
							End:     lineEnd,
							NewText: []byte("\n" + indent + "// TODO: replace nil with actual error"),
						})
					} else {
						// Put TODO at end of current line
						lineStart := call.Pos() - token.Pos(pos.Column-1)
						lineEnd := lineStart + token.Pos(len(currentLine))
						
						diagnostic.SuggestedFixes[0].TextEdits = append(diagnostic.SuggestedFixes[0].TextEdits, analysis.TextEdit{
							Pos:     lineEnd,
							End:     lineEnd,
							NewText: []byte(" // TODO: replace nil with actual error"),
						})
					}
				}
			}
		}
		
		pass.Report(diagnostic)
	}
}

// findErrorVariableInScope finds an error variable in scope similar to GoLand plugin logic
func findErrorVariableInScope(pass *analysis.Pass, call *ast.CallExpr) string {
	if pass.Fset == nil {
		return ""
	}
	// Find the containing function (either FuncDecl or FuncLit)
	var funcType *ast.FuncType
	var funcBody *ast.BlockStmt
	
	// Walk up the AST to find the closest function
	path, _ := astutil.PathEnclosingInterval(pass.Files[0], call.Pos(), call.End())
	for _, node := range path {
		switch n := node.(type) {
		case *ast.FuncDecl:
			funcType = n.Type
			funcBody = n.Body
			break
		case *ast.FuncLit:
			funcType = n.Type
			funcBody = n.Body
			break
		}
		if funcType != nil {
			break
		}
	}
	
	if funcType == nil || funcBody == nil {
		return ""
	}
	
	// Check function parameters first
	if funcType.Params != nil && pass.TypesInfo != nil {
		for _, field := range funcType.Params.List {
			// Get the type of this parameter
			typ := pass.TypesInfo.TypeOf(field.Type)
			if typ != nil && isErrorType(typ) {
				// Get the parameter name(s)
				for _, name := range field.Names {
					// Accept any error-typed parameter, common names: err, e, error, myErr, etc.
					return name.Name
				}
			}
		}
	}
	
	// Check named return values
	if funcType.Results != nil && pass.TypesInfo != nil {
		for _, field := range funcType.Results.List {
			// Get the type of this return value
			typ := pass.TypesInfo.TypeOf(field.Type)
			if typ != nil && isErrorType(typ) && len(field.Names) > 0 {
				for _, name := range field.Names {
					// Accept any error-typed return value
					return name.Name
				}
			}
		}
	}
	
	// Simple approach: look backwards from the call line for error variables
	// but only within the same logical scope
	callPos := pass.Fset.Position(call.Pos())
	
	// Find all error variable declarations in the function
	type errorVar struct {
		name string
		line int
		inIfBlock bool
		ifBlockStart int
		ifBlockEnd int
	}
	
	var errorVars []errorVar
	
	
	ast.Inspect(funcBody, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE { // :=
				nodePos := pass.Fset.Position(node.Pos())
				if nodePos.Line < callPos.Line {
					// Check if this assignment defines error variables
					for _, lhs := range node.Lhs {
						if ident, ok := lhs.(*ast.Ident); ok {
							if isLikelyErrorVariable(ident.Name) {
								// Check if this is inside an if block
								var inIf bool
								var ifStart, ifEnd int
								
								// Find if this assignment is inside an if statement
								ast.Inspect(funcBody, func(n2 ast.Node) bool {
									if ifStmt, ok := n2.(*ast.IfStmt); ok {
										if node.Pos() >= ifStmt.Pos() && node.End() <= ifStmt.End() {
											inIf = true
											ifStart = pass.Fset.Position(ifStmt.Pos()).Line
											ifEnd = pass.Fset.Position(ifStmt.End()).Line
											return false
										}
									}
									return true
								})
								
								errorVars = append(errorVars, errorVar{
									name: ident.Name,
									line: nodePos.Line,
									inIfBlock: inIf,
									ifBlockStart: ifStart,
									ifBlockEnd: ifEnd,
								})
								
							}
						}
					}
				}
			}
		}
		return true
	})
	
	// Find which if block (if any) contains our call
	var callInIf bool
	
	ast.Inspect(funcBody, func(n ast.Node) bool {
		if ifStmt, ok := n.(*ast.IfStmt); ok {
			ifStart := pass.Fset.Position(ifStmt.Pos()).Line
			ifEnd := pass.Fset.Position(ifStmt.End()).Line
			
			if callPos.Line >= ifStart && callPos.Line <= ifEnd {
				callInIf = true
				return false
			}
		}
		return true
	})
	
	
	// Find the most recent error variable that's accessible from the call
	var bestVar string
	var bestLine int = -1
	
	for _, ev := range errorVars {
		// Skip variables declared after our call
		if ev.line >= callPos.Line {
			continue
		}
		
		// Check scope compatibility:
		// 1. If call is in an if block that checks the error (like "if err != nil"), use that error var
		// 2. If call is in a different if block, don't use error vars from other blocks
		// 3. If call is not in any if block, be very restrictive - don't use error vars
		if callInIf {
			// Call is in an if block - check if this if block uses the error variable
			// For now, assume if the error var was defined recently and we're in an if block,
			// it's probably being checked by that if block
			isRecentVar := (callPos.Line - ev.line) <= 5 // Within 5 lines
			if !isRecentVar {
				continue // Skip - error var is too old/distant
			}
		} else {
			// Call is not in an if block - be very restrictive
			// Only allow error vars that are defined in the same "section" without any if blocks in between
			// For safety, don't use any error vars when not in an if block
			continue // Skip all error vars when not in if block
		}
		
		// Use the most recent (closest) error variable
		if ev.line > bestLine {
			bestVar = ev.name
			bestLine = ev.line
		}
	}
	
	return bestVar
}

// findContainingIfBlock finds the if statement that contains the call
func findContainingIfBlock(call *ast.CallExpr) *ast.IfStmt {
	// This is a simplified version - would need proper AST traversal
	return nil
}

// getErrorVariableFromIfCondition extracts error variable from if condition
func getErrorVariableFromIfCondition(ifStmt *ast.IfStmt) string {
	if ifStmt.Cond == nil {
		return ""
	}
	
	// Look for patterns like "err != nil"
	if binExpr, ok := ifStmt.Cond.(*ast.BinaryExpr); ok {
		if binExpr.Op == token.NEQ || binExpr.Op == token.EQL {
			if ident, ok := binExpr.X.(*ast.Ident); ok {
				if isLikelyErrorVariable(ident.Name) {
					return ident.Name
				}
			}
		}
	}
	
	return ""
}

// findContainingFunction finds the function containing the call
func findContainingFunction(call *ast.CallExpr) *ast.FuncDecl {
	// This needs proper AST traversal - simplified for now
	return nil
}

// findClosestErrorVariable finds the closest error variable to the call
func findClosestErrorVariable(function *ast.FuncDecl, call *ast.CallExpr) string {
	// This needs proper implementation with scope analysis
	return ""
}

// hasRecentErrorIgnorance checks if errors are being ignored nearby
func hasRecentErrorIgnorance(function *ast.FuncDecl, call *ast.CallExpr) bool {
	// Check for patterns like "_, _ = someFunc()"
	return false
}

// isVariableInScope checks if a variable name is in scope
func isVariableInScope(function *ast.FuncDecl, call *ast.CallExpr, varName string) bool {
	// This needs proper scope analysis
	return false
}

// isLikelyErrorVariable checks if a variable name looks like an error
func isLikelyErrorVariable(name string) bool {
	errorNames := []string{"err", "error", "e", "errs", "errors"}
	nameLower := strings.ToLower(name)
	
	for _, errorName := range errorNames {
		if nameLower == errorName {
			return true
		}
	}
	
	return strings.HasSuffix(name, "Err") || 
		   strings.HasSuffix(name, "Error") ||
		   strings.HasPrefix(nameLower, "err") ||
		   strings.HasPrefix(nameLower, "error")
}

func isBasicType(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Basic:
		return true
	default:
		return false
	}
}

func isTimeType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	
	return named.Obj().Pkg() != nil && 
		named.Obj().Pkg().Path() == "time" && 
		named.Obj().Name() == "Time"
}

// isStringer checks if a type implements the Stringer interface with proper signature
func isStringer(t types.Type) bool {
	// Look for String method
	method, _, _ := types.LookupFieldOrMethod(t, true, nil, "String")
	if method == nil {
		return false
	}
	
	// Verify it's a method with the correct signature: func() string
	fn, ok := method.(*types.Func)
	if !ok {
		return false
	}
	
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}
	
	// Check no parameters
	if sig.Params().Len() != 0 {
		return false
	}
	
	// Check returns single string
	if sig.Results().Len() != 1 {
		return false
	}
	
	resultType := sig.Results().At(0).Type()
	basic, ok := resultType.(*types.Basic)
	return ok && basic.Kind() == types.String
}

// toPascalCase converts a string to PascalCase
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}
	
	// Split by underscores, hyphens, dots, spaces, or other common separators
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == ' ' || r == ':' || r == '/' || r == '\\'
	})
	
	result := ""
	for _, part := range parts {
		if part != "" {
			// Capitalize first letter, lowercase rest
			result += strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	
	return result
}