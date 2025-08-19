package analyzer_test

import (
	"flag"
	"os"
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

// TestWithMethodDiagnosticSuppression tests suppression of With() diagnostics
func TestWithMethodDiagnosticSuppression(t *testing.T) {
	testdata := analysistest.TestData()
	
	// Create analyzer with MTLOG009 suppressed
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
	
	// Set suppress flag for MTLOG009
	if err := a.Flags.Set("suppress", "MTLOG009"); err != nil {
		t.Fatal(err)
	}
	
	analysistest.Run(t, testdata, a, "suppress_with")
}

// TestMultipleWithDiagnosticSuppression tests suppression of multiple With() diagnostics
func TestMultipleWithDiagnosticSuppression(t *testing.T) {
	testdata := analysistest.TestData()
	
	// Create analyzer with MTLOG009 and MTLOG010 suppressed
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
	
	// Set suppress flag for MTLOG009 and MTLOG010
	if err := a.Flags.Set("suppress", "MTLOG009,MTLOG010"); err != nil {
		t.Fatal(err)
	}
	
	analysistest.Run(t, testdata, a, "suppress_multiple_with")
}

// TestWithSuppressionViaEnvironment tests suppression via MTLOG_SUPPRESS environment variable
func TestWithSuppressionViaEnvironment(t *testing.T) {
	// Save original env
	original := os.Getenv("MTLOG_SUPPRESS")
	defer os.Setenv("MTLOG_SUPPRESS", original)
	
	// Set suppression via env
	os.Setenv("MTLOG_SUPPRESS", "MTLOG009,MTLOG010")
	
	testdata := analysistest.TestData()
	
	// Run analyzer without explicit suppress flag - should use env var
	analysistest.Run(t, testdata, analyzer.Analyzer, "suppress_env_with")
}