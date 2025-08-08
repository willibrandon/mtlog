package analyzer_test

import (
	"flag"
	"testing"

	"github.com/willibrandon/mtlog/cmd/mtlog-analyzer/analyzer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

// TestSuppression tests the suppression functionality
func TestSuppression(t *testing.T) {
	testdata := analysistest.TestData()
	
	// Run the analyzer with no suppression
	analysistest.Run(t, testdata, analyzer.Analyzer, "suppression_baseline")
}

// TestIndividualSuppression tests suppression of individual diagnostics
func TestIndividualSuppression(t *testing.T) {
	testdata := analysistest.TestData()
	
	// Create analyzer with MTLOG001 suppressed
	a := &analysis.Analyzer{
		Name:     analyzer.Analyzer.Name,
		Doc:      analyzer.Analyzer.Doc,
		Requires: analyzer.Analyzer.Requires,
		Run:      analyzer.Analyzer.Run,
		Flags:    flag.FlagSet{},
	}
	
	// Copy flag definitions
	analyzer.Analyzer.Flags.VisitAll(func(f *flag.Flag) {
		if f.Value.String() == "false" {
			a.Flags.Bool(f.Name, false, f.Usage)
		} else {
			a.Flags.String(f.Name, f.DefValue, f.Usage)
		}
	})
	
	// Set suppress flag for MTLOG001
	if err := a.Flags.Set("suppress", "MTLOG001"); err != nil {
		t.Fatal(err)
	}
	
	analysistest.Run(t, testdata, a, "suppress_mtlog001")
}

// TestMultipleSuppression tests suppression of multiple diagnostics
func TestMultipleSuppression(t *testing.T) {
	testdata := analysistest.TestData()
	
	// Create analyzer with MTLOG001 and MTLOG004 suppressed
	a := &analysis.Analyzer{
		Name:     analyzer.Analyzer.Name,
		Doc:      analyzer.Analyzer.Doc,
		Requires: analyzer.Analyzer.Requires,
		Run:      analyzer.Analyzer.Run,
		Flags:    flag.FlagSet{},
	}
	
	// Copy flag definitions
	analyzer.Analyzer.Flags.VisitAll(func(f *flag.Flag) {
		if f.Value.String() == "false" {
			a.Flags.Bool(f.Name, false, f.Usage)
		} else {
			a.Flags.String(f.Name, f.DefValue, f.Usage)
		}
	})
	
	// Set suppress flag for multiple IDs
	if err := a.Flags.Set("suppress", "MTLOG001,MTLOG004"); err != nil {
		t.Fatal(err)
	}
	
	analysistest.Run(t, testdata, a, "suppress_multiple")
}