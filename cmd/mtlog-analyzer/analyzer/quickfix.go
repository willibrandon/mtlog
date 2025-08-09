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

// suggestValidFormatSpecifier suggests a valid format specifier based on an invalid one
func suggestValidFormatSpecifier(invalidFormat string) (string, bool) {
	// Convert to lowercase for comparison
	lower := strings.ToLower(invalidFormat)
	
	// Common mistakes and their corrections
	switch {
	// Integer/decimal formats
	case lower == "d" || lower == "decimal" || lower == "int":
		return "000", true
	case strings.HasPrefix(lower, "d") && len(lower) > 1:
		// e.g., "d3" -> "000"
		if digits := lower[1:]; isAllDigits(digits) {
			return strings.Repeat("0", parseInt(digits, 3)), true
		}
		return "000", true
		
	// Float formats
	case lower == "f" || lower == "float":
		return "F2", true
	case strings.HasPrefix(lower, "f") && len(lower) > 1:
		// e.g., "f3" -> "F3"
		if digits := lower[1:]; isAllDigits(digits) {
			return "F" + digits, true
		}
		return "F2", true
		
	// Percentage formats
	case lower == "p" || lower == "percent" || lower == "percentage":
		return "P", true
	case strings.HasPrefix(lower, "p") && len(lower) > 1:
		// e.g., "p1" -> "P1"
		if digits := lower[1:]; isAllDigits(digits) {
			return "P" + digits, true
		}
		return "P", true
		
	// Exponential formats
	case lower == "e" || lower == "exp" || lower == "exponential":
		return "E", true
	case strings.HasPrefix(lower, "e") && len(lower) > 1:
		// e.g., "e2" -> "E2"
		if digits := lower[1:]; isAllDigits(digits) {
			return "E" + digits, true
		}
		return "E", true
		
	// General formats
	case lower == "g" || lower == "general":
		return "G", true
	case strings.HasPrefix(lower, "g") && len(lower) > 1:
		// e.g., "g2" -> "G2"
		if digits := lower[1:]; isAllDigits(digits) {
			return "G" + digits, true
		}
		return "G", true
		
	// Hexadecimal formats
	case lower == "h" || lower == "hex" || lower == "hexadecimal":
		return "X", true
	case len(invalidFormat) == 1 && (invalidFormat == "x" || invalidFormat == "X"):
		// Single 'x' or 'X' should become 'X' (uppercase is standard)
		return "X", true
	case strings.HasPrefix(lower, "h") && len(lower) > 1:
		// e.g., "h8" -> "X8"
		if digits := lower[1:]; isAllDigits(digits) {
			return "X" + digits, true
		}
		return "X", true
	case strings.HasPrefix(lower, "x") && len(lower) > 1:
		// e.g., "x8" -> "X8" (keep original case)
		if digits := lower[1:]; isAllDigits(digits) {
			if invalidFormat[0] == 'X' {
				return "X" + digits, true
			}
			return "x" + digits, true
		}
		return "X", true
		
	// Common .NET format strings that people might try
	case lower == "c" || lower == "currency":
		return "F2", true // Suggest float format for currency
	case lower == "n" || lower == "number":
		return "F0", true // Number format -> float with no decimals
	case lower == "r" || lower == "roundtrip":
		return "G", true // Roundtrip -> general format
		
	// Padding/alignment (just a number)
	case isAllDigits(lower):
		count := parseInt(lower, 3)
		return strings.Repeat("0", count), true
		
	default:
		// No suggestion available
		return "", false
	}
}

// isAllDigits checks if a string contains only digits
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// parseInt parses an integer with a default value
func parseInt(s string, def int) int {
	var val int
	if n, err := fmt.Sscanf(s, "%d", &val); n == 1 && err == nil {
		return val
	}
	return def
}

// createLogValueStub generates a LogValue() method stub for a complex type
func createLogValueStub(pass *analysis.Pass, argType types.Type) *analysis.SuggestedFix {
	// Get the underlying named type
	named, ok := argType.(*types.Named)
	if !ok {
		// Check if it's a pointer to a named type
		if ptr, ok := argType.(*types.Pointer); ok {
			named, ok = ptr.Elem().(*types.Named)
			if !ok {
				return nil
			}
		} else {
			return nil
		}
	}
	
	// Don't generate for types from other packages
	if named.Obj().Pkg() == nil {
		return nil
	}
	// Check if the type is in the current package being analyzed
	if named.Obj().Pkg() != pass.Pkg {
		// For test purposes, also check if it's in the same file
		typePos := named.Obj().Pos()
		inCurrentFile := false
		for _, file := range pass.Files {
			if file.Pos() <= typePos && typePos < file.End() {
				inCurrentFile = true
				break
			}
		}
		if !inCurrentFile {
			return nil
		}
	}
	
	// Check if LogValue method already exists
	if hasLogValueMethod(named) {
		return nil
	}
	
	// Get the struct type to inspect fields
	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		// Not a struct, can't generate meaningful stub
		return nil
	}
	
	// Find the type declaration in the AST
	typePos := named.Obj().Pos()
	var targetFile *ast.File
	for _, file := range pass.Files {
		if file.Pos() <= typePos && typePos < file.End() {
			targetFile = file
			break
		}
	}
	
	if targetFile == nil {
		return nil
	}
	
	// Find the end of the type declaration to insert the method after
	var typeDecl *ast.TypeSpec
	ast.Inspect(targetFile, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == named.Obj().Name() {
			typeDecl = ts
			return false
		}
		return true
	})
	
	if typeDecl == nil {
		return nil
	}
	
	// Generate the LogValue method stub
	methodStub := generateLogValueMethodStub(named, structType)
	
	// Find the insertion point (after the type declaration)
	insertPos := findMethodInsertPosition(targetFile, typeDecl)
	
	return &analysis.SuggestedFix{
		Message: fmt.Sprintf("Generate LogValue() method for %s", named.Obj().Name()),
		TextEdits: []analysis.TextEdit{{
			Pos:     insertPos,
			End:     insertPos,
			NewText: []byte(methodStub),
		}},
	}
}

// hasLogValueMethod checks if a type already has a LogValue method
func hasLogValueMethod(named *types.Named) bool {
	for i := 0; i < named.NumMethods(); i++ {
		method := named.Method(i)
		if method.Name() == "LogValue" {
			return true
		}
	}
	return false
}

// generateLogValueMethodStub generates the LogValue method code
func generateLogValueMethodStub(named *types.Named, structType *types.Struct) string {
	typeName := named.Obj().Name()
	receiverName := strings.ToLower(typeName[:1])
	
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n\n// LogValue provides a custom log representation for %s\n", typeName))
	sb.WriteString(fmt.Sprintf("func (%s %s) LogValue() any {\n", receiverName, typeName))
	sb.WriteString("\treturn map[string]any{\n")
	
	// List of common sensitive field names to warn about
	sensitiveNames := map[string]bool{
		"password": true, "pass": true, "passwd": true, "pwd": true,
		"secret": true, "apikey": true, "api_key": true, "apiKey": true,
		"token": true, "accesstoken": true, "access_token": true, "accessToken": true,
		"refreshtoken": true, "refresh_token": true, "refreshToken": true,
		"privatekey": true, "private_key": true, "privateKey": true,
		"key": true, "authtoken": true, "auth_token": true, "authToken": true,
		"credential": true, "credentials": true, "cred": true, "creds": true,
		"ssn": true, "socialsecurity": true, "social_security": true,
		"creditcard": true, "credit_card": true, "creditCard": true,
		"cardnumber": true, "card_number": true, "cardNumber": true,
		"cvv": true, "cvc": true, "securitycode": true, "security_code": true,
	}
	
	// Generate field entries
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		if !field.Exported() {
			continue
		}
		
		fieldName := field.Name()
		fieldNameLower := strings.ToLower(fieldName)
		
		// Check if this might be a sensitive field
		isSensitive := false
		for sensitive := range sensitiveNames {
			if strings.Contains(fieldNameLower, sensitive) {
				isSensitive = true
				break
			}
		}
		
		if isSensitive {
			sb.WriteString(fmt.Sprintf("\t\t// \"%s\": %s.%s, // TODO: Review - potentially sensitive field\n", 
				fieldName, receiverName, fieldName))
		} else {
			sb.WriteString(fmt.Sprintf("\t\t\"%s\": %s.%s,\n", fieldName, receiverName, fieldName))
		}
	}
	
	sb.WriteString("\t}\n")
	sb.WriteString("}")
	
	return sb.String()
}

// findMethodInsertPosition finds where to insert a method for a type
func findMethodInsertPosition(file *ast.File, typeDecl *ast.TypeSpec) token.Pos {
	// First, try to find other methods for this type and insert after them
	typeName := typeDecl.Name.Name
	var lastMethodEnd token.Pos
	
	// Look for existing methods
	for _, decl := range file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok && funcDecl.Recv != nil {
			if len(funcDecl.Recv.List) > 0 {
				recvType := funcDecl.Recv.List[0].Type
				
				// Check if this is a method for our type
				if ident, ok := recvType.(*ast.Ident); ok && ident.Name == typeName {
					if funcDecl.End() > lastMethodEnd {
						lastMethodEnd = funcDecl.End()
					}
				} else if star, ok := recvType.(*ast.StarExpr); ok {
					if ident, ok := star.X.(*ast.Ident); ok && ident.Name == typeName {
						if funcDecl.End() > lastMethodEnd {
							lastMethodEnd = funcDecl.End()
						}
					}
				}
			}
		}
	}
	
	if lastMethodEnd != 0 {
		return lastMethodEnd
	}
	
	// No existing methods, insert after the type declaration
	// Find the GenDecl containing the TypeSpec
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range genDecl.Specs {
				if spec == typeDecl {
					return genDecl.End()
				}
			}
		}
	}
	
	// Fallback: insert at the end of the file
	return file.End()
}