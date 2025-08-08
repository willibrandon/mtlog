package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

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
			checkAtPrefix(pass, call, arg, argType, propName, config)
		} else if strings.HasPrefix(propName, "$") {
			checkDollarPrefix(pass, call, arg, argType, propName, config)
		} else {
			checkNoPrefix(pass, call, arg, argType, propName, config)
		}
	}
}

// checkAtPrefix checks if @ prefix is appropriate for the type
func checkAtPrefix(pass *analysis.Pass, call *ast.CallExpr, arg ast.Expr, argType types.Type, propName string, config *Config) {
	// Good - using @ for complex type, but check if it's actually basic
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
}

// checkDollarPrefix checks if $ prefix is appropriate for the type
func checkDollarPrefix(pass *analysis.Pass, call *ast.CallExpr, arg ast.Expr, argType types.Type, propName string, config *Config) {
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
}

// checkNoPrefix checks if a prefix should be added for the type
func checkNoPrefix(pass *analysis.Pass, call *ast.CallExpr, arg ast.Expr, argType types.Type, propName string, config *Config) {
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