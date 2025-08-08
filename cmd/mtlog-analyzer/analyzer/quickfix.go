package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Common acronyms that should remain uppercase in constant names
var commonAcronyms = []string{"ID", "URL", "API", "HTTP", "HTTPS", "DNS", "IP", "CPU", "RAM", "OS", "DB"}

// Maximum attempts for making unique constant names.
// The value 100 is chosen as a reasonable upper bound to avoid excessive looping
// when generating unique constant names. In practice, collisions are rare, so 100
// attempts should be more than sufficient for expected use cases.
const uniqueConstNameMaxAttempts = 100

// Type checking helper functions

// isBasicType checks if a type is a basic Go type
func isBasicType(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Basic:
		return true
	default:
		return false
	}
}

// isTimeType checks if a type is time.Time
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

// isErrorType checks if the type is an error type
func isErrorType(t types.Type) bool {
	errorType := types.Universe.Lookup("error").Type()
	return types.Implements(t, errorType.Underlying().(*types.Interface))
}

// isErrorMethod checks if the method name is an error logging method
func isErrorMethod(name string) bool {
	return name == "Error" || name == "Err" || name == "E" ||
	       strings.HasPrefix(name, "Error") // Covers Errorw, Errorf, etc.
}

// String conversion functions

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

// unquoteString removes quotes from a string literal
func unquoteString(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') ||
			(s[0] == '`' && s[len(s)-1] == '`') ||
			(s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// Context key constant generation functions

// createContextKeyQuickFix creates a comprehensive quick fix for extracting string literals to constants
func createContextKeyQuickFix(pass *analysis.Pass, lit *ast.BasicLit, key string) *analysis.SuggestedFix {
	// Find the file containing this literal
	var targetFile *ast.File
	for _, file := range pass.Files {
		if pass.Fset.Position(file.Pos()).Filename == pass.Fset.Position(lit.Pos()).Filename {
			targetFile = file
			break
		}
	}
	
	if targetFile == nil {
		return nil
	}
	
	// Phase 1: Analysis - find all occurrences
	occurrences := findAllContextKeyOccurrences(targetFile, key)
	
	// Only create constant if there are 2+ occurrences
	if len(occurrences) < 2 {
		return nil
	}
	
	// Check for existing constant with same value
	if existingName := findExistingConstant(targetFile, key); existingName != "" {
		// Create fix to use existing constant
		return createReplacementFix(occurrences, existingName, 
			fmt.Sprintf("Use existing constant %s", existingName))
	}
	
	// Generate constant name
	constName := generateContextKeyName(key)
	
	// Check if name already exists and make unique if needed
	if pass.TypesInfo != nil && pass.Pkg != nil {
		constName = makeUniqueConstName(constName, pass.Pkg)
	}
	
	// Phase 2: Find where to insert the constant
	insertPos := findConstInsertPosition(targetFile)
	
	// Phase 3: Build text edits
	var edits []analysis.TextEdit
	
	// Format the constant declaration
	constDecl := formatConstDecl(constName, key)
	
	// Add the constant declaration edit
	edits = append(edits, analysis.TextEdit{
		Pos:     insertPos,
		End:     insertPos,
		NewText: []byte(constDecl),
	})
	
	// Replace all occurrences with the constant name
	for _, occurrence := range occurrences {
		edits = append(edits, analysis.TextEdit{
			Pos:     occurrence.Pos(),
			End:     occurrence.End(),
			NewText: []byte(constName),
		})
	}
	
	return &analysis.SuggestedFix{
		Message:   fmt.Sprintf("Extract %q to constant %s", key, constName),
		TextEdits: edits,
	}
}

// findAllContextKeyOccurrences finds all occurrences of a context key in mtlog calls
func findAllContextKeyOccurrences(file *ast.File, key string) []ast.Expr {
	var occurrences []ast.Expr
	
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		
		// Check for ForContext calls
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ForContext" {
			// ForContext can have multiple key-value pairs
			for i := 0; i < len(call.Args)-1; i += 2 {
				if lit, ok := call.Args[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if unquoteString(lit.Value) == key {
						occurrences = append(occurrences, lit)
					}
				}
			}
		}
		
		// Check for mtlog.PushProperty calls
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "PushProperty" {
			// Check if it's mtlog.PushProperty (package selector)
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "mtlog" {
				if len(call.Args) >= 2 {
					if lit, ok := call.Args[1].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if unquoteString(lit.Value) == key {
							occurrences = append(occurrences, lit)
						}
					}
				}
			}
		}
		
		return true
	})
	
	return occurrences
}

// findExistingConstant searches for an existing constant with the given value
func findExistingConstant(file *ast.File, value string) string {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		
		for _, spec := range genDecl.Specs {
			vspec, ok := spec.(*ast.ValueSpec)
			if !ok || len(vspec.Values) == 0 {
				continue
			}
			
			if lit, ok := vspec.Values[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if unquoteString(lit.Value) == value {
					return vspec.Names[0].Name
				}
			}
		}
	}
	
	return ""
}

// generateContextKeyName generates a constant name from a context key string
func generateContextKeyName(key string) string {
	if key == "" {
		return "emptyContextKey"
	}
	
	// Split into words, handling various separators
	parts := splitIntoWords(key)
	
	// Handle leading numbers
	if len(parts) > 0 && len(parts[0]) > 0 && parts[0][0] >= '0' && parts[0][0] <= '9' {
		parts[0] = "num" + parts[0]
	}
	
	// Apply casing rules
	for i, part := range parts {
		if isCommonAcronym(strings.ToUpper(part)) {
			parts[i] = strings.ToUpper(part)
		} else if i == 0 {
			// First word lowercase
			parts[i] = strings.ToLower(part[:1]) + strings.ToLower(part[1:])
		} else {
			// Others TitleCase
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	
	return strings.Join(parts, "") + "ContextKey"
}

// splitIntoWords splits a string into words, handling various separators
func splitIntoWords(s string) []string {
	// Use FieldsFunc to split on various separators
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == '/' || r == ':' || r == ' '
	})
}

// isCommonAcronym checks if a word is a common acronym that should be all caps
func isCommonAcronym(word string) bool {
	for _, acronym := range commonAcronyms {
		if word == acronym {
			return true
		}
	}
	return false
}

// makeUniqueConstName ensures the constant name is unique by adding a suffix if needed
func makeUniqueConstName(name string, pkg *types.Package) string {
	if pkg.Scope().Lookup(name) == nil {
		return name
	}
	
	// Add numeric suffix
	for i := 2; i < uniqueConstNameMaxAttempts; i++ {
		candidate := fmt.Sprintf("%s%d", name, i)
		if pkg.Scope().Lookup(candidate) == nil {
			return candidate
		}
	}
	
	return name + "_generated"
}

// findConstInsertPosition finds the best position to insert a constant declaration
func findConstInsertPosition(file *ast.File) token.Pos {
	// Look for existing const block with context keys
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
			// Check if this block has context-key-like constants
			if hasContextKeyConstants(genDecl) {
				// Add to this block - position just before the closing paren
				return genDecl.End() - 1
			}
		}
	}
	
	// Look for ANY existing const declaration (even if not context keys)
	// Insert after the last const declaration
	var lastConstDecl *ast.GenDecl
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
			lastConstDecl = genDecl
		}
	}
	if lastConstDecl != nil {
		return lastConstDecl.End()
	}
	
	// No const declarations found, insert after imports
	if len(file.Imports) > 0 {
		// Find the last import declaration
		for _, decl := range file.Decls {
			if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
				return genDecl.End()
			}
		}
	}
	
	// No imports, insert after package declaration
	return file.Name.End()
}

// hasContextKeyConstants checks if a const block contains context key constants
func hasContextKeyConstants(genDecl *ast.GenDecl) bool {
	for _, spec := range genDecl.Specs {
		if vspec, ok := spec.(*ast.ValueSpec); ok {
			for _, name := range vspec.Names {
				nameStr := name.Name
				// Check if it looks like a context key constant
				if strings.HasSuffix(nameStr, "ContextKey") ||
					strings.HasSuffix(nameStr, "Key") ||
					strings.HasSuffix(nameStr, "CtxKey") {
					return true
				}
			}
		}
	}
	return false
}

// formatConstDecl formats a constant declaration
func formatConstDecl(name, value string) string {
	// Always use block syntax for consistency
	return fmt.Sprintf("\n\nconst %s = %q", name, value)
}

// createReplacementFix creates a fix that just replaces occurrences with an existing constant
func createReplacementFix(occurrences []ast.Expr, constName, message string) *analysis.SuggestedFix {
	var edits []analysis.TextEdit
	
	for _, occurrence := range occurrences {
		edits = append(edits, analysis.TextEdit{
			Pos:     occurrence.Pos(),
			End:     occurrence.End(),
			NewText: []byte(constName),
		})
	}
	
	return &analysis.SuggestedFix{
		Message:   message,
		TextEdits: edits,
	}
}