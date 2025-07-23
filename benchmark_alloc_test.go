package mtlog

import (
	"fmt"
	"testing"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/parser"
)

// Let's break down where allocations happen
func TestAllocationBreakdown(t *testing.T) {
	// Test 1: Parse template
	t.Run("ParseTemplate", func(t *testing.T) {
		allocs := testing.AllocsPerRun(100, func() {
			_, err := parser.Parse("This is a simple log message")
			if err != nil {
				panic(fmt.Sprintf("Failed to parse template: %v", err))
			}
		})
		t.Logf("Parse template allocations: %.1f", allocs)
	})
	
	// Test 2: Create LogEvent
	t.Run("CreateLogEvent", func(t *testing.T) {
		allocs := testing.AllocsPerRun(100, func() {
			_ = &core.LogEvent{
				Properties: make(map[string]interface{}),
			}
		})
		t.Logf("Create LogEvent allocations: %.1f", allocs)
	})
	
	// Test 3: Extract properties (empty)
	t.Run("ExtractProperties", func(t *testing.T) {
		logger := New()
		tmpl, _ := parser.Parse("This is a simple log message")
		
		allocs := testing.AllocsPerRun(100, func() {
			logger.extractProperties(tmpl, nil)
		})
		t.Logf("Extract properties allocations: %.1f", allocs)
	})
	
	// Test 4: Full logging flow
	t.Run("FullLoggingFlow", func(t *testing.T) {
		logger := New(WithSink(&discardSink{}))
		
		allocs := testing.AllocsPerRun(100, func() {
			logger.Information("This is a simple log message")
		})
		t.Logf("Full logging flow allocations: %.1f", allocs)
	})
	
	// Test 5: Just the map creation
	t.Run("MapCreation", func(t *testing.T) {
		allocs := testing.AllocsPerRun(100, func() {
			_ = make(map[string]interface{})
		})
		t.Logf("Map creation allocations: %.1f", allocs)
	})
}