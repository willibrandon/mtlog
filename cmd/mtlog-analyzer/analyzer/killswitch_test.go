package analyzer_test

import (
	"flag"
	"testing"

	"github.com/willibrandon/mtlog/cmd/mtlog-analyzer/analyzer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

// TestDisableAll tests the global kill switch
func TestDisableAll(t *testing.T) {
	testdata := analysistest.TestData()

	// Create a new analyzer with disable-all flag
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

	// Set disable-all flag
	a.Flags.Set("disable-all", "true")

	// Should produce no diagnostics
	analysistest.Run(t, testdata, a, "killswitch")
}
