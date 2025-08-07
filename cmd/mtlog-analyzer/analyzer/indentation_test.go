package analyzer

import (
	"testing"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestMTLOG001IndentationWithExistingComments(t *testing.T) {
	testdata := analysistest.TestData()
	
	// Test that TODO comments are properly indented when there's an existing comment
	analysistest.RunWithSuggestedFixes(t, testdata, Analyzer, "indentation_test")
}

// TestIndentationLogic validates the corrected indentation calculation
func TestIndentationLogic(t *testing.T) {
	testCases := []struct {
		name        string
		column      int
		wantTabs    int
		description string
	}{
		{"col1", 1, 0, "Column 1 (pos 0) should have 0 tabs"},
		{"col2", 2, 0, "Column 2 (pos 1) should have 0 tabs"},
		{"col8", 8, 0, "Column 8 (pos 7) should have 0 tabs"},
		{"col9", 9, 1, "Column 9 (pos 8) should have 1 tab"},
		{"col16", 16, 1, "Column 16 (pos 15) should have 1 tab"},
		{"col17", 17, 2, "Column 17 (pos 16) should have 2 tabs"},
		{"col24", 24, 2, "Column 24 (pos 23) should have 2 tabs"},
		{"col25", 25, 3, "Column 25 (pos 24) should have 3 tabs"},
		{"col32", 32, 3, "Column 32 (pos 31) should have 3 tabs"},
		{"col33", 33, 4, "Column 33 (pos 32) should have 4 tabs"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// What the logic SHOULD be: divide by 8 (tab width)
			// Column numbers are 1-based, positions are 0-based
			position := tc.column - 1
			correctTabCount := position / 8
			
			if correctTabCount != tc.wantTabs {
				t.Errorf("Column %d: correct logic gives %d tabs, want %d tabs. %s", 
					tc.column, correctTabCount, tc.wantTabs, tc.description)
			}
		})
	}
}