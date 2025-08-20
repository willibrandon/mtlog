package analyzer

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// runWithAllChecks runs all applicable checks on a method call
func runWithAllChecks(pass *analysis.Pass, call *ast.CallExpr, cache *templateCache, config *Config) {
	// Always check context usage (applies to ForContext)
	if !config.DisabledChecks["context"] {
		checkContextUsage(pass, call, config)
	}
	
	// Check With() method arguments
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if ok && sel.Sel.Name == "With" {
		checkWithArguments(pass, call, config)
		return // With() doesn't have templates, so skip template checks
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
	if !ok || lit.Kind != token.STRING {
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

