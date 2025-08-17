package analyzer

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

// Reserved property names that might confuse users
var defaultReservedProperties = []string{
	"Timestamp",
	"Level",
	"Message",
	"MessageTemplate",
	"Exception",
	"SourceContext",
}

// checkWithArguments checks the With() method for common issues
func checkWithArguments(pass *analysis.Pass, call *ast.CallExpr, config *Config) {
	// Get method name
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "With" {
		return
	}

	// With() with no arguments is valid
	if len(call.Args) == 0 {
		return
	}

	// Track if we have odd arguments but continue checking other issues
	hasOddArgs := false
	
	// Check for odd number of arguments
	if !config.DisabledChecks["with-odd"] {
		if len(call.Args)%2 != 0 {
			hasOddArgs = true
			// Create suggested fixes
			var fixes []analysis.SuggestedFix

			// Fix 1: Add empty string as value
			fixes = append(fixes, analysis.SuggestedFix{
				Message: "Add empty string value for the last key",
				TextEdits: []analysis.TextEdit{{
					Pos:     call.Args[len(call.Args)-1].End(),
					End:     call.Args[len(call.Args)-1].End(),
					NewText: []byte(`, ""`),
				}},
			})

			// Fix 2: Remove the dangling key
			if len(call.Args) > 1 {
				// Find the comma before the last argument
				fixes = append(fixes, analysis.SuggestedFix{
					Message: "Remove the dangling key",
					TextEdits: []analysis.TextEdit{{
						// Remove from the comma before to the end of the argument
						Pos: call.Args[len(call.Args)-2].End(),
						End: call.Args[len(call.Args)-1].End(),
						NewText: []byte(""),
					}},
				})
			}

			diag := analysis.Diagnostic{
				Pos:            call.Pos(),
				Message:        fmt.Sprintf("[%s] With() requires an even number of arguments (key-value pairs), got %d", DiagIDWithOddArgs, len(call.Args)),
				SuggestedFixes: fixes,
			}
			pass.Report(diag)
		}
	}

	// Check each key-value pair
	seenKeys := make(map[string]int) // Track duplicate keys with their positions

	// Adjust loop to handle odd arguments
	maxPairs := len(call.Args) / 2
	if hasOddArgs {
		maxPairs = (len(call.Args) + 1) / 2
	}
	
	for i := 0; i < maxPairs*2 && i < len(call.Args); i += 2 {
		keyArg := call.Args[i]
		// valueArg := call.Args[i+1] if i+1 < len(call.Args) // We have the value but don't need it for key checks

		// Check if key is a string type
		if !config.DisabledChecks["with-nonstring"] {
			if !isStringType(pass, keyArg) {
				// Try to determine what the key is
				keyDesc := describeExpression(pass, keyArg)
				
				// Create suggested fix based on the type of expression
				var fixes []analysis.SuggestedFix
				
				// If it's a literal, convert it to string
				if lit, ok := keyArg.(*ast.BasicLit); ok {
					var newText string
					switch lit.Kind {
					case token.INT, token.FLOAT:
						// Convert number to string
						newText = fmt.Sprintf(`"%s"`, lit.Value)
						fixes = append(fixes, analysis.SuggestedFix{
							Message: fmt.Sprintf("Convert %s to string", lit.Value),
							TextEdits: []analysis.TextEdit{{
								Pos:     lit.Pos(),
								End:     lit.End(),
								NewText: []byte(newText),
							}},
						})
					}
				} else if ident, ok := keyArg.(*ast.Ident); ok {
					// If it's an identifier, suggest using it as the value with a string key
					fixes = append(fixes, analysis.SuggestedFix{
						Message: fmt.Sprintf("Use '%s' as value with string key", ident.Name),
						TextEdits: []analysis.TextEdit{{
							Pos:     ident.Pos(),
							End:     ident.End(),
							NewText: []byte(fmt.Sprintf(`"%s", %s`, toSnakeCase(ident.Name), ident.Name)),
						}},
					})
				}

				diag := analysis.Diagnostic{
					Pos:            keyArg.Pos(),
					Message:        fmt.Sprintf("[%s] With() key must be a string, got %s", DiagIDWithNonStringKey, keyDesc),
					SuggestedFixes: fixes,
				}
				pass.Report(diag)
				continue // Skip further checks for this key
			}
		}

		// Extract string literal value if possible
		keyValue := extractStringLiteral(pass, keyArg)
		if keyValue == "" && !isStringExpression(keyArg) {
			// Can't determine key value at compile time, skip remaining checks
			continue
		}

		// Check for empty key
		if !config.DisabledChecks["with-empty"] && keyValue == "" && isEmptyStringLiteral(keyArg) {
			var fixes []analysis.SuggestedFix
			
			// Fix: Remove the empty key-value pair
			if i+1 < len(call.Args) {
				// Determine what to remove
				startPos := keyArg.Pos()
				endPos := call.Args[i+1].End()
				
				// If not the first pair, include the comma before
				if i > 0 {
					// Try to include the comma and space before this pair
					startPos = call.Args[i-1].End()
				} else if i+2 < len(call.Args) {
					// First pair, include comma after if there are more arguments
					endPos = call.Args[i+2].Pos()
				}
				
				fixes = append(fixes, analysis.SuggestedFix{
					Message: "Remove empty key-value pair",
					TextEdits: []analysis.TextEdit{{
						Pos:     startPos,
						End:     endPos,
						NewText: []byte(""),
					}},
				})
			}

			diag := analysis.Diagnostic{
				Pos:            keyArg.Pos(),
				Message:        fmt.Sprintf("[%s] With() key is empty and will be ignored", DiagIDWithEmptyKey),
				SuggestedFixes: fixes,
			}
			pass.Report(diag)
			continue
		}

		// Track duplicate keys (only for literal strings)
		if keyValue != "" {
			if !config.DisabledChecks["duplicate"] {
				if prevPos, exists := seenKeys[keyValue]; exists {
					// Found duplicate
					reportDiagnosticWithID(pass, keyArg.Pos(), SeverityWarning, config, DiagIDDuplicateProperty,
						"duplicate key '%s' in With() call (previous at position %d)", keyValue, prevPos/2+1)
				} else {
					seenKeys[keyValue] = i
				}
			}

			// Check for reserved properties (off by default)
			if config.CheckReservedProperties {
				if isReservedProperty(keyValue, config) {
					diag := analysis.Diagnostic{
						Pos:     keyArg.Pos(),
						Message: fmt.Sprintf("[%s] %s: property '%s' shadows a built-in property", DiagIDWithReservedProp, SeveritySuggestion, keyValue),
						SuggestedFixes: []analysis.SuggestedFix{{
							Message: fmt.Sprintf("Rename to 'Custom%s' or 'User%s'", keyValue, keyValue),
							TextEdits: []analysis.TextEdit{{
								Pos:     keyArg.Pos(),
								End:     keyArg.End(),
								NewText: []byte(fmt.Sprintf(`"User%s"`, keyValue)),
							}},
						}},
					}
					pass.Report(diag)
				}
			}
		}
	}
}

// isStringType checks if an expression is of string type
func isStringType(pass *analysis.Pass, expr ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	
	// Check for string type
	if basic, ok := t.Underlying().(*types.Basic); ok {
		return basic.Kind() == types.String || basic.Kind() == types.UntypedString
	}
	
	return false
}

// isStringExpression checks if an expression is a string (for when we can't get the value)
func isStringExpression(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.Ident:
		// Could be a string variable
		return true
	case *ast.SelectorExpr:
		// Could be a string field
		return true
	case *ast.CallExpr:
		// Could be a function returning string
		return true
	default:
		return false
	}
}

// extractStringLiteral attempts to extract a string literal value
func extractStringLiteral(pass *analysis.Pass, expr ast.Expr) string {
	// Check if it's a basic literal
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		// Unquote the string
		if s, err := strconv.Unquote(lit.Value); err == nil {
			return s
		}
	}
	
	// Check if it's a constant
	if pass.TypesInfo.Types != nil {
		if tv, ok := pass.TypesInfo.Types[expr]; ok && tv.Value != nil {
			if tv.Value.Kind() == constant.String {
				return constant.StringVal(tv.Value)
			}
		}
	}
	
	return ""
}

// isEmptyStringLiteral checks if an expression is an empty string literal
func isEmptyStringLiteral(expr ast.Expr) bool {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return lit.Value == `""` || lit.Value == "``"
	}
	return false
}

// describeExpression returns a human-readable description of an expression's type
func describeExpression(pass *analysis.Pass, expr ast.Expr) string {
	// First check AST-based description for literals
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			return "numeric literal"
		case token.FLOAT:
			return "float literal"
		case token.STRING:
			return "string literal"
		default:
			return "literal"
		}
	case *ast.Ident:
		// Check if it's a variable with known type
		if t := pass.TypesInfo.TypeOf(expr); t != nil {
			// If it's not a string type, include the variable name
			if basic, ok := t.Underlying().(*types.Basic); ok {
				if basic.Kind() != types.String && basic.Kind() != types.UntypedString {
					return fmt.Sprintf("variable '%s'", e.Name)
				}
			}
		}
		return fmt.Sprintf("variable '%s'", e.Name)
	case *ast.SelectorExpr:
		return "field or method"
	case *ast.CallExpr:
		return "function call"
	default:
		// Fall back to type info if available
		if t := pass.TypesInfo.TypeOf(expr); t != nil {
			return t.String()
		}
		return "expression"
	}
}

// isReservedProperty checks if a property name is reserved
func isReservedProperty(name string, config *Config) bool {
	// Use configured reserved properties if available
	reservedList := config.ReservedProperties
	if len(reservedList) == 0 {
		reservedList = defaultReservedProperties
	}
	
	for _, reserved := range reservedList {
		if strings.EqualFold(name, reserved) {
			return true
		}
	}
	return false
}

// toSnakeCase converts a name to snake_case
// Handles:
// - Consecutive capitals (HTTPServer → http_server, IOError → io_error)
// - Numbers (userId123 → user_id_123)
// - Already snake_case input (already_snake_case → already_snake_case)
// - All caps (HTML → html, API → api)
func toSnakeCase(s string) string {
	// Early return: If already snake_case, just normalize to lowercase
	// This handles cases like "User_Id" → "user_id" or "user_id" → "user_id"
	if strings.Contains(s, "_") {
		return strings.ToLower(s)
	}
	
	var result strings.Builder
	runes := []rune(s)
	
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		
		// Determine where to insert underscores based on character transitions
		if i > 0 {
			prev := runes[i-1]
			
			// Check for uppercase letter transitions
			if unicode.IsUpper(r) {
				// Case 1: camelCase transition (e.g., "userId" → "user_id")
				// Insert underscore when moving from lowercase to uppercase
				if unicode.IsLower(prev) {
					result.WriteByte('_')
				} 
				// Case 2: Number to uppercase transition (e.g., "123User" → "123_user")
				// Insert underscore when moving from digit to uppercase letter
				else if unicode.IsDigit(prev) {
					result.WriteByte('_')
				} 
				// Case 3: Acronym boundary detection (e.g., "HTTPServer" → "http_server")
				// When we have consecutive uppercase letters followed by lowercase,
				// we're at the start of a new word after an acronym
				else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(prev) {
					result.WriteByte('_')
				}
			} 
			// Case 4: Letter to number transition (e.g., "user123" → "user_123")
			// Insert underscore when moving from non-digit to digit
			else if unicode.IsDigit(r) && !unicode.IsDigit(prev) {
				result.WriteByte('_')
			} 
			// Case 5: Number to lowercase transition (e.g., "123user" → "123_user")
			// Insert underscore when moving from digit to lowercase letter
			else if unicode.IsLower(r) && unicode.IsDigit(prev) {
				result.WriteByte('_')
			}
		}
		
		// Add the character (lowercase if upper)
		if unicode.IsUpper(r) {
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}