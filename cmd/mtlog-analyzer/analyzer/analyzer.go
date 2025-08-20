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
	"go/ast"
	"os"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

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
	Analyzer.Flags.String("reserved-props", "", "comma-separated list of reserved property names for With() method (overrides defaults)")
	Analyzer.Flags.Bool("check-reserved", false, "enable checking for reserved property names in With() calls")
}

// Analyzer is the mtlog-analyzer that checks for common logging mistakes.
// It can be used with go vet or as a standalone tool.
var Analyzer = &analysis.Analyzer{
	Name:     "mtlog",
	Doc:      "check for common mtlog mistakes",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// run executes the analyzer on the given pass.
func run(pass *analysis.Pass) (any, error) {
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

	// Parse reserved properties configuration
	if reservedProps, found := getStringFlag(pass, "reserved-props"); found && reservedProps != "" {
		config.ReservedProperties = strings.Split(reservedProps, ",")
		for i := range config.ReservedProperties {
			config.ReservedProperties[i] = strings.TrimSpace(config.ReservedProperties[i])
		}
	}

	if checkReserved, found := getBoolFlag(pass, "check-reserved"); found {
		config.CheckReservedProperties = checkReserved
	}

	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	cache := &templateCache{cache: make(map[string]templateInfo)}
	
	// Create cross-call analyzer for tracking logger variables
	crossCallAnalyzer := NewCrossCallAnalyzer(pass, config)

	// First pass: track variable assignments
	nodeFilter := []ast.Node{
		(*ast.AssignStmt)(nil),
		(*ast.CallExpr)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Track logger variable assignments for cross-call analysis
			crossCallAnalyzer.AnalyzeAssignment(node)
			
		case *ast.CallExpr:
			// Check if this is an mtlog-related call (logging or context methods)
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return
			}

			// Quick check: is this potentially an mtlog logger call?
			if !isPotentialMtlogCall(pass, sel, config) {
				return
			}

			// Check for cross-call duplicates (collects them for later reporting)
			crossCallAnalyzer.CheckMethodCall(node)

			// Process all method calls on what could be a logger
			// Individual checks will decide if they apply
			runWithAllChecks(pass, node, cache, config)
		}
	})

	// Report all collected cross-call duplicates
	// IMPORTANT: We collect duplicates during the AST walk and report them here at the end
	// rather than reporting them immediately when found. This is necessary because the
	// analysistest framework has issues matching diagnostics that are reported from
	// different analyzer contexts. When diagnostics are reported directly from the
	// CrossCallAnalyzer during the AST walk, the test framework cannot properly match
	// them with want comments, causing tests to fail with "unexpected diagnostic" errors.
	// By collecting them first and reporting them all here from the main analyzer context,
	// we ensure the test framework can properly match diagnostics with their want comments.
	for _, dup := range crossCallAnalyzer.GetDuplicates() {
		var message string
		if dup.MethodName == "With" {
			message = "With() overrides property '%s' set in previous call"
		} else {
			message = "ForContext() overrides property '%s' set in previous call"
		}
		
		reportDiagnosticWithID(pass, dup.Pos, SeverityWarning, config, DiagIDWithCrossCall,
			message, dup.Key)
	}

	return nil, nil
}
