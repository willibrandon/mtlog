package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	
	"golang.org/x/tools/go/analysis"
)

func TestErrorVariableScopeDetection(t *testing.T) {
	// Test the exact scenarios from requirements
	src := `package main

import (
    "errors"
    "net"
    "github.com/willibrandon/mtlog"
    "github.com/willibrandon/mtlog/core"
)

func main() {
    log := mtlog.New(
        mtlog.WithConsole(),
        mtlog.WithMinimumLevel(core.InformationLevel),
    )

    // Has err in scope from earlier
    conn, err := net.Dial("tcp", "localhost:8080")
    if err != nil {
        log.Error("Connection failed") // Line 21: Should find 'err'
    }

    // Different block, no err
    if conn != nil {
        log.Error("Connection issue") // Line 26: Should add 'nil' with TODO
    }

    // Ignore error with blank identifier
    _, _ = doSomething()
    log.Error("Ignored error case") // Line 31: Should add 'nil' with TODO
}

func doSomething() (interface{}, error) {
    return nil, errors.New("test error")
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	// Create a mock pass for testing
	pass := &analysis.Pass{
		Fset: fset,
		Files: []*ast.File{file},
	}

	// Test each log.Error call
	testCases := []struct {
		line int
		expected string
		description string
	}{
		{19, "err", "Should find 'err' in same if block"},
		{24, "nil", "Should use 'nil' in different block"},
		{29, "nil", "Should use 'nil' when error ignored"},
	}

	foundCalls := 0
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if isErrorCall(call) {
				pos := fset.Position(call.Pos())
				foundCalls++
				
				for _, tc := range testCases {
					if pos.Line == tc.line {
						result := findErrorVariableInScope(pass, call)
						if result == "" {
							result = "nil"
						}
						
						if result != tc.expected {
							t.Errorf("Line %d (%s): expected '%s', got '%s'", 
								tc.line, tc.description, tc.expected, result)
						}
						break
					}
				}
			}
		}
		return true
	})
	
	if foundCalls != len(testCases) {
		t.Errorf("Expected %d Error calls, found %d", len(testCases), foundCalls)
	}
}

func isErrorCall(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Name == "Error"
	}
	return false
}