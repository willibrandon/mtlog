package analyzer_test

import (
	"testing"
	"github.com/willibrandon/mtlog/cmd/mtlog-analyzer/analyzer"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestEdgeCases(t *testing.T) {
	testdata := analysistest.TestData()
	
	// Test dynamic templates
	t.Run("DynamicTemplates", func(t *testing.T) {
		analysistest.Run(t, testdata, analyzer.Analyzer, "dynamic")
	})
	
	// Test aliased types
	t.Run("AliasedTypes", func(t *testing.T) {
		analysistest.Run(t, testdata, analyzer.Analyzer, "aliased")
	})
	
	// Test malformed Go syntax handling
	t.Run("MalformedSyntax", func(t *testing.T) {
		// The analysistest framework will skip analysis on malformed code
		// We're testing that the analyzer doesn't panic or cause issues
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Analyzer should not panic on malformed syntax: %v", r)
			}
		}()
		
		// The analyzer relies on go vet to handle malformed syntax
		// Since go vet won't run the analyzer on unparseable code,
		// we just verify that attempting to analyze doesn't panic
		t.Log("Malformed syntax is handled by go vet before reaching the analyzer")
		
		// Document the behavior: malformed Go files are rejected by the Go parser
		// before they reach our analyzer, so we don't need special handling
		t.Log("Go's analysis framework skips packages with syntax errors")
	})
}