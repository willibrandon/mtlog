package mtlog

import (
	"sync"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
)

func TestNewLoggingLevelSwitch(t *testing.T) {
	// Test with different initial levels
	testCases := []struct {
		name         string
		initialLevel core.LogEventLevel
	}{
		{"Verbose", core.VerboseLevel},
		{"Debug", core.DebugLevel},
		{"Information", core.InformationLevel},
		{"Warning", core.WarningLevel},
		{"Error", core.ErrorLevel},
		{"Fatal", core.FatalLevel},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ls := NewLoggingLevelSwitch(tc.initialLevel)
			if ls.Level() != tc.initialLevel {
				t.Errorf("Expected initial level %v, got %v", tc.initialLevel, ls.Level())
			}
		})
	}
}

func TestLoggingLevelSwitch_SetLevelAndGetLevel(t *testing.T) {
	ls := NewLoggingLevelSwitch(core.InformationLevel)

	// Test setting various levels
	levels := []core.LogEventLevel{
		core.VerboseLevel,
		core.DebugLevel,
		core.InformationLevel,
		core.WarningLevel,
		core.ErrorLevel,
		core.FatalLevel,
	}

	for _, level := range levels {
		ls.SetLevel(level)
		if ls.Level() != level {
			t.Errorf("Expected level %v, got %v", level, ls.Level())
		}
	}
}

func TestLoggingLevelSwitch_IsEnabled(t *testing.T) {
	ls := NewLoggingLevelSwitch(core.InformationLevel)

	testCases := []struct {
		testLevel core.LogEventLevel
		expected  bool
	}{
		{core.VerboseLevel, false},
		{core.DebugLevel, false},
		{core.InformationLevel, true},
		{core.WarningLevel, true},
		{core.ErrorLevel, true},
		{core.FatalLevel, true},
	}

	for _, tc := range testCases {
		result := ls.IsEnabled(tc.testLevel)
		if result != tc.expected {
			t.Errorf("For minimum level Information, expected IsEnabled(%v) = %v, got %v",
				tc.testLevel, tc.expected, result)
		}
	}
}

func TestLoggingLevelSwitch_ConvenienceMethods(t *testing.T) {
	ls := NewLoggingLevelSwitch(core.InformationLevel)

	// Test fluent interface methods
	if ls.Verbose().Level() != core.VerboseLevel {
		t.Error("Verbose() method failed")
	}

	if ls.Debug().Level() != core.DebugLevel {
		t.Error("Debug() method failed")
	}

	if ls.Information().Level() != core.InformationLevel {
		t.Error("Information() method failed")
	}

	if ls.Warning().Level() != core.WarningLevel {
		t.Error("Warning() method failed")
	}

	if ls.Error().Level() != core.ErrorLevel {
		t.Error("Error() method failed")
	}

	if ls.Fatal().Level() != core.FatalLevel {
		t.Error("Fatal() method failed")
	}
}

func TestLoggingLevelSwitch_FluentInterface(t *testing.T) {
	ls := NewLoggingLevelSwitch(core.InformationLevel)

	// Test that methods return the same instance for chaining
	result := ls.Debug().Warning().Information()
	if result != ls {
		t.Error("Fluent interface should return the same instance")
	}

	if ls.Level() != core.InformationLevel {
		t.Errorf("Expected final level Information, got %v", ls.Level())
	}
}

func TestLoggingLevelSwitch_ThreadSafety(t *testing.T) {
	ls := NewLoggingLevelSwitch(core.InformationLevel)
	
	const numGoroutines = 100
	const numOperationsPerGoroutine = 100
	
	var wg sync.WaitGroup
	
	// Start multiple goroutines that concurrently read and write the level
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			levels := []core.LogEventLevel{
				core.VerboseLevel,
				core.DebugLevel,
				core.InformationLevel,
				core.WarningLevel,
				core.ErrorLevel,
				core.FatalLevel,
			}
			
			for j := 0; j < numOperationsPerGoroutine; j++ {
				// Set a level
				level := levels[j%len(levels)]
				ls.SetLevel(level)
				
				// Read the level
				currentLevel := ls.Level()
				
				// Test IsEnabled with various levels
				ls.IsEnabled(core.InformationLevel)
				ls.IsEnabled(currentLevel)
				
				// Brief pause to encourage race conditions if they exist
				if j%10 == 0 {
					time.Sleep(1 * time.Nanosecond)
				}
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// If we reach here without data races or panics, the test passed
}

func TestLoggingLevelSwitch_LevelProgression(t *testing.T) {
	ls := NewLoggingLevelSwitch(core.FatalLevel)
	
	// Test that each level correctly enables/disables appropriate levels
	testData := []struct {
		setLevel     core.LogEventLevel
		testLevel    core.LogEventLevel
		shouldEnable bool
	}{
		// When set to Verbose, all levels should be enabled
		{core.VerboseLevel, core.VerboseLevel, true},
		{core.VerboseLevel, core.DebugLevel, true},
		{core.VerboseLevel, core.InformationLevel, true},
		{core.VerboseLevel, core.WarningLevel, true},
		{core.VerboseLevel, core.ErrorLevel, true},
		{core.VerboseLevel, core.FatalLevel, true},
		
		// When set to Warning, only Warning and above should be enabled
		{core.WarningLevel, core.VerboseLevel, false},
		{core.WarningLevel, core.DebugLevel, false},
		{core.WarningLevel, core.InformationLevel, false},
		{core.WarningLevel, core.WarningLevel, true},
		{core.WarningLevel, core.ErrorLevel, true},
		{core.WarningLevel, core.FatalLevel, true},
		
		// When set to Fatal, only Fatal should be enabled
		{core.FatalLevel, core.VerboseLevel, false},
		{core.FatalLevel, core.DebugLevel, false},
		{core.FatalLevel, core.InformationLevel, false},
		{core.FatalLevel, core.WarningLevel, false},
		{core.FatalLevel, core.ErrorLevel, false},
		{core.FatalLevel, core.FatalLevel, true},
	}
	
	for _, td := range testData {
		ls.SetLevel(td.setLevel)
		result := ls.IsEnabled(td.testLevel)
		if result != td.shouldEnable {
			t.Errorf("With minimum level %v, expected IsEnabled(%v) = %v, got %v",
				td.setLevel, td.testLevel, td.shouldEnable, result)
		}
	}
}

func BenchmarkLoggingLevelSwitch_Level(b *testing.B) {
	ls := NewLoggingLevelSwitch(core.InformationLevel)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ls.Level()
	}
}

func BenchmarkLoggingLevelSwitch_SetLevel(b *testing.B) {
	ls := NewLoggingLevelSwitch(core.InformationLevel)
	levels := []core.LogEventLevel{
		core.VerboseLevel,
		core.DebugLevel,
		core.InformationLevel,
		core.WarningLevel,
		core.ErrorLevel,
		core.FatalLevel,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ls.SetLevel(levels[i%len(levels)])
	}
}

func BenchmarkLoggingLevelSwitch_IsEnabled(b *testing.B) {
	ls := NewLoggingLevelSwitch(core.InformationLevel)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ls.IsEnabled(core.InformationLevel)
	}
}