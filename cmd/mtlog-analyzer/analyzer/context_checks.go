package analyzer

import (
	"fmt"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// checkContextUsage checks for ForContext and PushProperty usage patterns
func checkContextUsage(pass *analysis.Pass, call *ast.CallExpr, config *Config) {
	// Check for both ForContext (method) and PushProperty (function)
	isContextCall := false
	keyArgIndex := 0

	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		// Method call like logger.ForContext
		switch sel.Sel.Name {
		case "ForContext":
			isContextCall = true
			keyArgIndex = 0
		case "PushProperty":
			// Check if it's a package function call (mtlog.PushProperty) as opposed to a method call
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "mtlog" {
				isContextCall = true
				keyArgIndex = 1 // PushProperty(logger, key, value)
			}
		}
	}

	if !isContextCall {
		return
	}

	// Check if we have enough arguments
	if len(call.Args) <= keyArgIndex {
		return
	}

	// ForContext can have multiple key-value pairs
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ForContext" {
		// Check all key arguments (every other arg starting from 0)
		for i := 0; i < len(call.Args)-1; i += 2 {
			if lit, ok := call.Args[i].(*ast.BasicLit); ok && lit.Value != "" {
				key := strings.Trim(lit.Value, `"`)
				checkCommonContextKey(pass, lit, key, config)
			}
		}
	} else {
		// PushProperty has single key-value pair
		if lit, ok := call.Args[keyArgIndex].(*ast.BasicLit); ok && lit.Value != "" {
			key := strings.Trim(lit.Value, `"`)
			checkCommonContextKey(pass, lit, key, config)
		}
	}
}

// checkCommonContextKey checks if a context key is common and suggests using a constant
func checkCommonContextKey(pass *analysis.Pass, lit *ast.BasicLit, key string, config *Config) {
	// Suggest using constants for common keys (from config)
	for _, common := range config.CommonContextKeys {
		if strings.EqualFold(key, common) {
			// Check if diagnostic is suppressed
			if !config.SuppressedDiagnostics[DiagIDContextKey] {
				diagnostic := analysis.Diagnostic{
					Pos:     lit.Pos(),
					End:     lit.End(),
					Message: fmt.Sprintf("[%s] suggestion: consider defining a constant for commonly used context key '%s'", DiagIDContextKey, key),
				}

				// Try to create comprehensive quick fix (only if 2+ occurrences)
				fix := createContextKeyQuickFix(pass, lit, key)
				if fix != nil {
					diagnostic.SuggestedFixes = []analysis.SuggestedFix{*fix}
				} else {
					// Fallback to simple replacement suggestion (for single occurrences)
					constName := generateContextKeyName(key)
					diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
						Message: fmt.Sprintf("Replace with constant %s", constName),
						TextEdits: []analysis.TextEdit{{
							Pos:     lit.Pos(),
							End:     lit.End(),
							NewText: []byte(constName),
						}},
					}}
				}

				pass.Report(diagnostic)
			}
			break
		}
	}
}
