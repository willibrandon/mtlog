package analyzer

import (
	"fmt"
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
			name: "MTLOG005 - LogValue() stub generation",
			dir:  "suggestedfix/mtlog005",
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
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run with suggested fix validation
			analysistest.RunWithSuggestedFixes(t, testdata, Analyzer, tt.dir)
		})
	}
}

// TestMTLOG002FormatSpecifierFixes tests format specifier fixes with strict mode
func TestMTLOG002FormatSpecifierFixes(t *testing.T) {
	testdata := analysistest.TestData()
	
	// Save original flag state
	originalStrict := false
	if flag := Analyzer.Flags.Lookup("strict"); flag != nil {
		if val, ok := flag.Value.(interface{ String() string }); ok {
			originalStrict = val.String() == "true"
		}
	}
	
	// Set strict mode for this test
	Analyzer.Flags.Set("strict", "true")
	defer func() {
		// Restore original state
		Analyzer.Flags.Set("strict", fmt.Sprintf("%v", originalStrict))
	}()
	
	// Run with suggested fix validation
	analysistest.RunWithSuggestedFixes(t, testdata, Analyzer, "suggestedfix/mtlog002")
}