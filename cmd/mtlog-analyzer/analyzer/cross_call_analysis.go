package analyzer

import (
	"go/ast"
	"go/token"
	"sort"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

// LoggerProperties tracks the accumulated properties for a logger variable
type LoggerProperties struct {
	// Properties maps property names to their source locations
	Properties map[string]token.Pos
	// BaseLogger is the variable this logger was derived from (if any)
	BaseLogger string
}

// CrossCallDuplicate represents a cross-call duplicate property
type CrossCallDuplicate struct {
	Key        string
	Pos        token.Pos
	PrevPos    token.Pos
	MethodName string
}

// CrossCallAnalyzer tracks logger variables and their accumulated properties
// for detecting cross-call duplicate properties
type CrossCallAnalyzer struct {
	pass *analysis.Pass
	// loggerProps maps variable names to their accumulated properties
	loggerProps map[string]*LoggerProperties
	config      *Config
	// duplicates collects all found duplicates to be reported later
	duplicates []CrossCallDuplicate
}

// NewCrossCallAnalyzer creates a new cross-call analyzer
func NewCrossCallAnalyzer(pass *analysis.Pass, config *Config) *CrossCallAnalyzer {
	return &CrossCallAnalyzer{
		pass:        pass,
		loggerProps: make(map[string]*LoggerProperties),
		config:      config,
		duplicates:  []CrossCallDuplicate{},
	}
}

// GetDuplicates returns all collected cross-call duplicates
func (c *CrossCallAnalyzer) GetDuplicates() []CrossCallDuplicate {
	return c.duplicates
}

// AnalyzeAssignment tracks logger variable assignments
func (c *CrossCallAnalyzer) AnalyzeAssignment(assign *ast.AssignStmt) {
	// Skip if cross-call checking is disabled
	if c.config.DisabledChecks["with-cross-call"] {
		return
	}

	// Look for assignments like: logger2 := logger1.With(...)
	for i, lhs := range assign.Lhs {
		if i >= len(assign.Rhs) {
			continue
		}
		
		// Get the variable being assigned to
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		
		// Check if RHS is a method call on a logger
		call, ok := assign.Rhs[i].(*ast.CallExpr)
		if !ok {
			continue
		}
		
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		
		// Check if it's a With() or ForContext() call
		if sel.Sel.Name != "With" && sel.Sel.Name != "ForContext" {
			continue
		}
		
		// Get the base logger variable
		baseIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			// Could be a chained call, handle later
			continue
		}
		
		// Track the new logger's properties
		c.trackLoggerAssignment(ident.Name, baseIdent.Name, call, sel.Sel.Name)
	}
}

// trackLoggerAssignment tracks properties for a logger assignment
func (c *CrossCallAnalyzer) trackLoggerAssignment(newLogger, baseLogger string, call *ast.CallExpr, methodName string) {
	// Get base logger's properties
	var baseProps map[string]token.Pos
	if base, exists := c.loggerProps[baseLogger]; exists {
		// Copy base properties
		baseProps = make(map[string]token.Pos)
		for k, v := range base.Properties {
			baseProps[k] = v
		}
	} else {
		baseProps = make(map[string]token.Pos)
	}
	
	// Extract new properties from the call
	newProps := c.extractPropertiesFromCall(call, methodName)
	
	// Merge properties (new ones override old ones)
	for k, v := range newProps {
		baseProps[k] = v
	}
	
	// Store the accumulated properties
	c.loggerProps[newLogger] = &LoggerProperties{
		Properties: baseProps,
		BaseLogger: baseLogger,
	}
}

// extractPropertiesFromCall extracts property names from With() or ForContext() calls
func (c *CrossCallAnalyzer) extractPropertiesFromCall(call *ast.CallExpr, methodName string) map[string]token.Pos {
	props := make(map[string]token.Pos)
	
	if methodName == "With" {
		// With() has key-value pairs
		for i := 0; i < len(call.Args)-1; i += 2 {
			keyArg := call.Args[i]
			
			// Try to extract string literal
			if lit, ok := keyArg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if key, err := strconv.Unquote(lit.Value); err == nil && key != "" {
					props[key] = keyArg.Pos()
				}
			}
		}
	} else if methodName == "ForContext" && len(call.Args) > 0 {
		// ForContext() has a single key as first argument
		keyArg := call.Args[0]
		if lit, ok := keyArg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if key, err := strconv.Unquote(lit.Value); err == nil && key != "" {
				props[key] = keyArg.Pos()
			}
		}
	}
	
	return props
}

// CheckMethodCall checks a With() or ForContext() call for cross-call duplicates
func (c *CrossCallAnalyzer) CheckMethodCall(call *ast.CallExpr) {
	// Skip if cross-call checking is disabled
	if c.config.DisabledChecks["with-cross-call"] {
		return
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	
	methodName := sel.Sel.Name
	if methodName != "With" && methodName != "ForContext" {
		return
	}
	
	// Try to get the receiver variable
	var receiverName string
	var accumulatedProps map[string]token.Pos
	
	switch receiver := sel.X.(type) {
	case *ast.Ident:
		// Direct variable: logger.With(...)
		receiverName = receiver.Name
		if props, exists := c.loggerProps[receiverName]; exists {
			accumulatedProps = props.Properties
		}
	case *ast.CallExpr:
		// Chained call: logger.With(...).With(...)
		// We need to analyze the chain
		accumulatedProps = c.analyzeCallChain(receiver)
	}
	
	// Extract properties from current call
	currentProps := c.extractPropertiesFromCall(call, methodName)
	
	// Collect duplicates and sort by position for deterministic output
	type duplicate struct {
		key     string
		pos     token.Pos
		prevPos token.Pos
	}
	var duplicates []duplicate
	
	// Check for duplicates with accumulated properties
	for key, pos := range currentProps {
		if prevPos, exists := accumulatedProps[key]; exists {
			// Found a cross-call duplicate
			duplicates = append(duplicates, duplicate{key, pos, prevPos})
		}
	}
	
	// Sort by position for deterministic output
	sort.Slice(duplicates, func(i, j int) bool {
		return duplicates[i].pos < duplicates[j].pos
	})
	
	// Report diagnostics in order
	for _, dup := range duplicates {
		c.reportCrossCallDuplicate(dup.key, dup.pos, dup.prevPos, methodName)
	}
	
	// Note: We don't need to call checkChainedDuplicates here because
	// we've already checked for duplicates against accumulatedProps,
	// which includes properties from the entire chain
}

// analyzeCallChain analyzes a chain of method calls to accumulate properties
func (c *CrossCallAnalyzer) analyzeCallChain(call *ast.CallExpr) map[string]token.Pos {
	props := make(map[string]token.Pos)
	
	// Recursively analyze the chain
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		methodName := sel.Sel.Name
		
		// Get properties from earlier in the chain
		switch receiver := sel.X.(type) {
		case *ast.CallExpr:
			// Continue up the chain
			props = c.analyzeCallChain(receiver)
		case *ast.Ident:
			// Base of the chain
			if loggerProps, exists := c.loggerProps[receiver.Name]; exists {
				// Copy accumulated properties
				for k, v := range loggerProps.Properties {
					props[k] = v
				}
			}
		}
		
		// Add properties from this call
		if methodName == "With" || methodName == "ForContext" {
			currentProps := c.extractPropertiesFromCall(call, methodName)
			for k, v := range currentProps {
				props[k] = v
			}
		}
	}
	
	return props
}


// reportCrossCallDuplicate collects a cross-call duplicate property
func (c *CrossCallAnalyzer) reportCrossCallDuplicate(key string, currentPos, prevPos token.Pos, methodName string) {
	// Collect the duplicate to be reported later
	c.duplicates = append(c.duplicates, CrossCallDuplicate{
		Key:        key,
		Pos:        currentPos,
		PrevPos:    prevPos,
		MethodName: methodName,
	})
}