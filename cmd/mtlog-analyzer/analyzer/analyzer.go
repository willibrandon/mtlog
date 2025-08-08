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
