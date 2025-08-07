package analyzer

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestSuggestedFixes(t *testing.T) {
	testdata := analysistest.TestData()
	
	tests := []struct {
		name string
		dir  string
	}{
		{
			name: "MTLOG001 - Template argument mismatch fixes",
			dir:  "suggestedfix/mtlog001",
		},
		{
			name: "MTLOG006 - Missing error parameter fixes",
			dir:  "suggestedfix/mtlog006",
		},
		{
			name: "MTLOG007 - Context key constants",
			dir:  "suggestedfix/mtlog007",
		},
		// TODO: Add test data for other diagnostics with suggested fixes:
		// - MTLOG004 (PascalCase properties)
		// - MTLOG005 (Capturing hints)
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run with suggested fix validation
			analysistest.RunWithSuggestedFixes(t, testdata, Analyzer, tt.dir)
		})
	}
}