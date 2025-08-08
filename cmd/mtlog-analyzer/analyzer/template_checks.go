package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// templateMismatchContext groups parameters for creating template mismatch diagnostics
type templateMismatchContext struct {
	pass       *analysis.Pass
	call       *ast.CallExpr
	properties []string
	argCount   int
	cache      *templateCache
	config     *Config
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
		ctx := &templateMismatchContext{
			pass:       pass,
			call:       call,
			properties: properties,
			argCount:   argCount,
			cache:      cache,
			config:     config,
		}
		createTemplateMismatchDiagnostic(ctx)
		return
	}

	// Check for invalid format specifiers
	for i, prop := range properties {
		if err := validateFormatSpecifier(prop, config); err != nil {
			// Try to create a suggested fix for the invalid format
			parts := strings.SplitN(prop, ":", 2)
			if len(parts) == 2 {
				propName := parts[0]
				invalidFormat := parts[1]
				
				if suggestedFormat, ok := suggestValidFormatSpecifier(invalidFormat); ok {
					// Find the position of this property in the template
					templateStr := template
					propIndex := 0
					propStart := -1
					
					for pos := 0; pos < len(templateStr); pos++ {
						if templateStr[pos] == '{' && (pos == 0 || templateStr[pos-1] != '{') {
							if propIndex == i {
								propStart = pos + 1 // Skip the opening brace
								break
							}
							propIndex++
						}
					}
					
					if propStart >= 0 {
						// Calculate the actual position in the file
						litStart := lit.Pos() + 1 // Skip opening quote
						propPos := litStart + token.Pos(propStart)
						propEnd := propPos + token.Pos(len(prop))
						
						newProp := propName + ":" + suggestedFormat
						
						diagnostic := analysis.Diagnostic{
							Pos:     call.Pos(),
							End:     call.End(),
							Message: fmt.Sprintf("[%s] invalid format specifier in property '%s': %v", 
								DiagIDFormatSpecifier, prop, err),
							SuggestedFixes: []analysis.SuggestedFix{{
								Message: fmt.Sprintf("Change format from ':%s' to ':%s'", invalidFormat, suggestedFormat),
								TextEdits: []analysis.TextEdit{{
									Pos:     propPos,
									End:     propEnd,
									NewText: []byte(newProp),
								}},
							}},
						}
						
						pass.Report(diagnostic)
						continue
					}
				}
			}
			
			// No suggested fix available, report without fix
			reportDiagnosticWithID(pass, call.Pos(), SeverityError, config, DiagIDFormatSpecifier,
				"invalid format specifier in property '%s': %v", prop, err)
		}
	}
}

// createTemplateMismatchDiagnostic creates a diagnostic with suggested fixes for template/argument mismatch
func createTemplateMismatchDiagnostic(ctx *templateMismatchContext) {
	message := fmt.Sprintf("template has %d properties but %d arguments provided", 
		len(ctx.properties), ctx.argCount)
	
	// Apply downgrade if needed
	severity := SeverityError
	if ctx.config != nil && ctx.config.DowngradeErrors {
		severity = SeverityWarning
	}
	
	// Add severity prefix for non-error diagnostics
	if severity != SeverityError {
		message = severity + ": " + message
	}
	
	// Add diagnostic ID to message
	message = fmt.Sprintf("[%s] %s", DiagIDTemplateMismatch, message)
	
	diagnostic := analysis.Diagnostic{
		Pos:     ctx.call.Pos(),
		End:     ctx.call.End(),
		Message: message,
	}
	
	// Add suggested fix based on the mismatch
	if ctx.argCount < len(ctx.properties) {
		addMissingArgumentsFix(&diagnostic, ctx.pass, ctx.call, ctx.properties, ctx.argCount, ctx.cache)
	} else if ctx.argCount > len(ctx.properties) {
		addExtraArgumentsFix(&diagnostic, ctx.call, ctx.properties, ctx.argCount)
	}
	
	// Check if this diagnostic is suppressed
	if ctx.config != nil && ctx.config.SuppressedDiagnostics[DiagIDTemplateMismatch] {
		return // Diagnostic is suppressed
	}
	
	ctx.pass.Report(diagnostic)
}

// addMissingArgumentsFix adds a suggested fix for missing arguments
func addMissingArgumentsFix(diagnostic *analysis.Diagnostic, pass *analysis.Pass, call *ast.CallExpr, properties []string, argCount int, cache *templateCache) {
	missingCount := len(properties) - argCount
	placeholders := make([]string, missingCount)
	for i := 0; i < missingCount; i++ {
		placeholders[i] = "nil"
	}
	
	// Build TODO comment with all missing properties
	var missingProps []string
	for i := 0; i < missingCount; i++ {
		missingProps = append(missingProps, properties[argCount+i])
	}
	
	// Check if there's already a comment on this line after the call
	hasExistingComment := false
	callLine := pass.Fset.Position(call.End()).Line
	var lastCommentEnd token.Pos
	
	// Look through all comments in the file
	for _, f := range pass.Files {
		if pass.Fset.Position(f.Pos()).Filename == pass.Fset.Position(call.Pos()).Filename {
			for _, cg := range f.Comments {
				for _, c := range cg.List {
					commentPos := pass.Fset.Position(c.Pos())
					// Check if comment is on the same line AND after the call
					if commentPos.Line == callLine && c.Pos() > call.End() {
						hasExistingComment = true
						// Track the end of the last comment on this line
						if c.End() > lastCommentEnd {
							lastCommentEnd = c.End()
						}
					}
				}
			}
			break
		}
	}
	
	// Build the appropriate TODO comment
	var todoEdit analysis.TextEdit
	if hasExistingComment && lastCommentEnd > 0 {
		// There's an existing comment, put TODO on the next line with proper indentation
		todoEdit = createIndentedTodoEdit(pass, call, cache, lastCommentEnd, missingProps)
	} else {
		// No existing comment, put TODO on the same line
		todoComment := " // TODO: provide value for " + strings.Join(missingProps, ", ")
		todoEdit = analysis.TextEdit{
			Pos:     call.End(),
			End:     call.End(),
			NewText: []byte(todoComment),
		}
	}
	
	diagnostic.SuggestedFixes = []analysis.SuggestedFix{{
		Message: fmt.Sprintf("Add %d missing argument(s)", missingCount),
		TextEdits: []analysis.TextEdit{
			{
				Pos:     call.End() - 1, // Just before the closing paren
				End:     call.End() - 1,
				NewText: []byte(", " + strings.Join(placeholders, ", ")),
			},
			todoEdit,
		},
	}}
}

// createIndentedTodoEdit creates a TODO comment with proper indentation
func createIndentedTodoEdit(pass *analysis.Pass, call *ast.CallExpr, cache *templateCache, insertPos token.Pos, missingProps []string) analysis.TextEdit {
	file := pass.Fset.File(call.Pos())
	lineStart := file.LineStart(file.Line(call.Pos()))
	indentBytes := file.Offset(call.Pos()) - file.Offset(lineStart)
	
	var indent string
	
	// Check if we likely have mixed indentation
	if indentBytes > 0 && containsMixedIndent(indentBytes) {
		// Need to read actual source to preserve mixed indentation
		pos := pass.Fset.Position(call.Pos())
		if line, err := cache.getSourceLine(pos.Filename, pos.Line); err == nil {
			indent = "\n" + extractIndentation(line)
		} else {
			// Fallback to tabs if we can't read the file
			indent = "\n" + strings.Repeat("\t", indentBytes)
		}
	} else {
		// Standard tab indentation (most common case, no file read needed)
		indent = "\n" + strings.Repeat("\t", indentBytes)
	}
	
	todoComment := indent + "// TODO: provide value for " + strings.Join(missingProps, ", ")
	return analysis.TextEdit{
		Pos:     insertPos,
		End:     insertPos,
		NewText: []byte(todoComment),
	}
}

// addExtraArgumentsFix adds a suggested fix for extra arguments
func addExtraArgumentsFix(diagnostic *analysis.Diagnostic, call *ast.CallExpr, properties []string, argCount int) {
	// Too many arguments - suggest removing extras
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