package analyzer_test

import (
	"flag"
	"testing"

	"github.com/willibrandon/mtlog/cmd/mtlog-analyzer/analyzer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestWithReservedPropertiesEnabled(t *testing.T) {
	testdata := analysistest.TestData()
	
	// Create a new analyzer instance with its own flag set
	a := &analysis.Analyzer{
		Name:     analyzer.Analyzer.Name,
		Doc:      analyzer.Analyzer.Doc,
		Requires: analyzer.Analyzer.Requires,
		Run:      analyzer.Analyzer.Run,
		Flags:    *flag.NewFlagSet("test", flag.ContinueOnError),
	}
	
	// Register all flags
	a.Flags.Bool("strict", false, "enable strict format specifier validation")
	a.Flags.String("common-keys", "", "comma-separated list of common context keys")
	a.Flags.String("disable", "", "comma-separated list of checks to disable")
	a.Flags.Bool("ignore-dynamic-templates", false, "suppress warnings for dynamic templates")
	a.Flags.Bool("strict-logger-types", false, "disable lenient logger type checking")
	a.Flags.Bool("downgrade-errors", false, "downgrade errors to warnings")
	a.Flags.Bool("disable-all", false, "disable all mtlog diagnostics")
	a.Flags.String("suppress", "", "comma-separated list of diagnostic IDs to suppress")
	a.Flags.String("reserved-props", "", "comma-separated list of reserved property names")
	a.Flags.Bool("check-reserved", false, "enable checking for reserved property names")
	
	// Set the flag value
	if err := a.Flags.Set("check-reserved", "true"); err != nil {
		t.Fatalf("Failed to set check-reserved flag: %v", err)
	}
	
	analysistest.Run(t, testdata, a, "with_reserved")
}