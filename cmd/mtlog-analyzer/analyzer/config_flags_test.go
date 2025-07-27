package analyzer_test

import (
	"flag"
	"testing"
	"github.com/willibrandon/mtlog/cmd/mtlog-analyzer/analyzer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

// createTestAnalyzer creates a new analyzer instance with a fresh flag set.
// This ensures flag isolation between tests, preventing test interference
// when running tests in parallel or in different orders.
func createTestAnalyzer() *analysis.Analyzer {
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
	a.Flags.Bool("downgrade-errors", false, "downgrade errors to warnings for CI environments")
	
	return a
}

func TestConfigurationFlags(t *testing.T) {
	testdata := analysistest.TestData()
	
	// Test with strict mode enabled
	t.Run("StrictMode", func(t *testing.T) {
		strictAnalyzer := createTestAnalyzer()
		strictAnalyzer.Flags.Set("strict", "true")
		analysistest.Run(t, testdata, strictAnalyzer, "strict")
	})
	
	// Test with disabled checks
	t.Run("DisabledChecks", func(t *testing.T) {
		disabledAnalyzer := createTestAnalyzer()
		disabledAnalyzer.Flags.Set("disable", "naming")
		analysistest.Run(t, testdata, disabledAnalyzer, "disabled")
	})
	
	// Test common-keys flag
	t.Run("CommonKeys", func(t *testing.T) {
		commonKeysAnalyzer := createTestAnalyzer()
		commonKeysAnalyzer.Flags.Set("common-keys", "custom_id,tenant_id,user.id,user-id,user:id,user/id,user_id.test-value,request-id:trace_id")
		analysistest.Run(t, testdata, commonKeysAnalyzer, "commonkeys")
	})
	
	// Test ignore-dynamic-templates flag
	t.Run("IgnoreDynamicTemplates", func(t *testing.T) {
		ignoreDynamicAnalyzer := createTestAnalyzer()
		ignoreDynamicAnalyzer.Flags.Set("ignore-dynamic-templates", "true")
		analysistest.Run(t, testdata, ignoreDynamicAnalyzer, "dynamicignore")
	})
	
	// Test strict-logger-types flag
	t.Run("StrictLoggerTypes", func(t *testing.T) {
		strictTypesAnalyzer := createTestAnalyzer()
		strictTypesAnalyzer.Flags.Set("strict-logger-types", "true")
		analysistest.Run(t, testdata, strictTypesAnalyzer, "stricttypes")
	})
	
	// Test invalid disable flag values
	t.Run("InvalidDisableValues", func(t *testing.T) {
		invalidDisableAnalyzer := createTestAnalyzer()
		// "invalid" is not a valid check name, should be ignored
		invalidDisableAnalyzer.Flags.Set("disable", "invalid,naming,notacheck")
		analysistest.Run(t, testdata, invalidDisableAnalyzer, "invalidflags")
	})
	
	// Test downgrade-errors flag
	t.Run("DowngradeErrors", func(t *testing.T) {
		downgradeAnalyzer := createTestAnalyzer()
		downgradeAnalyzer.Flags.Set("downgrade-errors", "true")
		downgradeAnalyzer.Flags.Set("strict", "true") // Enable strict mode to generate some errors
		analysistest.Run(t, testdata, downgradeAnalyzer, "downgrade")
	})
	
	// Test empty flag values
	t.Run("EmptyFlagValues", func(t *testing.T) {
		emptyFlagsAnalyzer := createTestAnalyzer()
		// Set empty values for various flags - should be handled gracefully
		emptyFlagsAnalyzer.Flags.Set("disable", "")
		emptyFlagsAnalyzer.Flags.Set("common-keys", "")
		analysistest.Run(t, testdata, emptyFlagsAnalyzer, "emptyflags")
	})
}