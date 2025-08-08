package analyzer

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

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

