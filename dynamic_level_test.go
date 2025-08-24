package mtlog

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func TestDynamicLevelControl_BasicFunctionality(t *testing.T) {
	// Create a memory sink to capture output
	memorySink := sinks.NewMemorySink()
	
	// Create level switch starting at Information level
	levelSwitch := NewLoggingLevelSwitch(core.InformationLevel)
	
	// Create logger with the level switch
	logger := New(
		WithSink(memorySink),
		WithLevelSwitch(levelSwitch),
	)

	// Test initial level (Information)
	logger.Debug("Debug message")      // Should be filtered
	logger.Information("Info message") // Should pass
	logger.Warning("Warning message")  // Should pass

	events := memorySink.Events()
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	if events[0].MessageTemplate != "Info message" {
		t.Errorf("Expected 'Info message', got '%s'", events[0].MessageTemplate)
	}
	
	if events[1].MessageTemplate != "Warning message" {
		t.Errorf("Expected 'Warning message', got '%s'", events[1].MessageTemplate)
	}

	// Change level to Debug
	levelSwitch.SetLevel(core.DebugLevel)
	memorySink.Clear()

	logger.Debug("Debug message 2")      // Should now pass
	logger.Information("Info message 2") // Should pass

	events = memorySink.Events()
	if len(events) != 2 {
		t.Fatalf("Expected 2 events after level change, got %d", len(events))
	}

	// Change level to Error
	levelSwitch.SetLevel(core.ErrorLevel)
	memorySink.Clear()

	logger.Debug("Debug message 3")   // Should be filtered
	logger.Information("Info message 3") // Should be filtered
	logger.Warning("Warning message 3")  // Should be filtered
	logger.Error("Error message 3")      // Should pass

	events = memorySink.Events()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event after level change to Error, got %d", len(events))
	}

	if events[0].MessageTemplate != "Error message 3" {
		t.Errorf("Expected 'Error message 3', got '%s'", events[0].MessageTemplate)
	}
}

func TestDynamicLevelControl_vs_StaticLevel(t *testing.T) {
	memorySink := sinks.NewMemorySink()

	// Test logger with static level
	staticLogger := New(
		WithSink(memorySink),
		WithMinimumLevel(core.WarningLevel),
	)

	// Test logger with dynamic level
	levelSwitch := NewLoggingLevelSwitch(core.WarningLevel)
	dynamicLogger := New(
		WithSink(memorySink),
		WithLevelSwitch(levelSwitch),
	)

	// Both should behave the same initially
	staticLogger.Information("Static info")  // Filtered
	staticLogger.Warning("Static warning")   // Pass

	dynamicLogger.Information("Dynamic info")  // Filtered
	dynamicLogger.Warning("Dynamic warning")   // Pass

	events := memorySink.Events()
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// Verify that level switch takes precedence over static level
	staticAndDynamicLogger := New(
		WithSink(memorySink),
		WithMinimumLevel(core.ErrorLevel), // Static level is Error
		WithLevelSwitch(levelSwitch),      // Dynamic level is Warning
	)

	memorySink.Clear()
	staticAndDynamicLogger.Warning("Should pass") // Warning should pass because dynamic level is Warning

	events = memorySink.Events()
	if len(events) != 1 {
		t.Errorf("Expected dynamic level to override static level")
	}
}

func TestDynamicLevelControl_ConvenienceMethods(t *testing.T) {
	// Test WithControlledLevel convenience method
	option, levelSwitch := WithControlledLevel(core.InformationLevel)

	memorySink := sinks.NewMemorySink()
	logger := New(
		WithSink(memorySink),
		option,
	)

	// Test that the level switch works
	logger.Debug("Debug")         // Filtered
	logger.Information("Info")    // Pass

	events := memorySink.Events()
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	// Test changing level via the returned switch
	levelSwitch.Debug()
	memorySink.Clear()

	logger.Debug("Debug 2")       // Should now pass
	logger.Information("Info 2")  // Should pass

	events = memorySink.Events()
	if len(events) != 2 {
		t.Errorf("Expected 2 events after level change, got %d", len(events))
	}
}

func TestDynamicLevelControl_LoggerMethods(t *testing.T) {
	levelSwitch := NewLoggingLevelSwitch(core.WarningLevel)
	logger := New(
		WithSink(sinks.NewMemorySink()),
		WithLevelSwitch(levelSwitch),
	)

	// Test GetMinimumLevel
	if logger.GetMinimumLevel() != core.WarningLevel {
		t.Errorf("Expected Warning level, got %v", logger.GetMinimumLevel())
	}

	// Test GetLevelSwitch
	if logger.GetLevelSwitch() != levelSwitch {
		t.Error("GetLevelSwitch should return the configured level switch")
	}

	// Test IsEnabled
	if logger.IsEnabled(core.DebugLevel) {
		t.Error("Debug should not be enabled when minimum level is Warning")
	}

	if !logger.IsEnabled(core.ErrorLevel) {
		t.Error("Error should be enabled when minimum level is Warning")
	}

	// Change level and test again
	levelSwitch.Debug()

	if logger.GetMinimumLevel() != core.DebugLevel {
		t.Errorf("Expected Debug level after change, got %v", logger.GetMinimumLevel())
	}

	if !logger.IsEnabled(core.DebugLevel) {
		t.Error("Debug should be enabled when minimum level is Debug")
	}
}

func TestDynamicLevelControl_ForContext(t *testing.T) {
	levelSwitch := NewLoggingLevelSwitch(core.InformationLevel)
	memorySink := sinks.NewMemorySink()
	
	logger := New(
		WithSink(memorySink),
		WithLevelSwitch(levelSwitch),
	)

	// Create context logger
	contextLogger := logger.ForContext("Component", "Test")

	// Test that context logger inherits level switch
	contextLogger.Debug("Debug")         // Filtered
	contextLogger.Information("Info")    // Pass

	events := memorySink.Events()
	if len(events) != 1 {
		t.Errorf("Expected 1 event from context logger, got %d", len(events))
	}

	// Test that level change affects context logger
	levelSwitch.Debug()
	memorySink.Clear()

	contextLogger.Debug("Debug 2")       // Should now pass
	events = memorySink.Events()
	if len(events) != 1 {
		t.Errorf("Expected context logger to respect level changes, got %d events", len(events))
	}

	// Verify the context logger has the same level switch
	if contextLogger.(interface{ GetLevelSwitch() *LoggingLevelSwitch }).GetLevelSwitch() != levelSwitch {
		t.Error("Context logger should inherit the same level switch")
	}
}

func TestDynamicLevelControl_Concurrency(t *testing.T) {
	levelSwitch := NewLoggingLevelSwitch(core.InformationLevel)
	memorySink := sinks.NewMemorySink()
	
	logger := New(
		WithSink(memorySink),
		WithLevelSwitch(levelSwitch),
	)

	const numGoroutines = 50
	const numOperationsPerGoroutine = 100
	var wg sync.WaitGroup

	// Start goroutines that concurrently change levels and log
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			levels := []core.LogEventLevel{
				core.VerboseLevel,
				core.DebugLevel,
				core.InformationLevel,
				core.WarningLevel,
				core.ErrorLevel,
			}
			
			for j := 0; j < numOperationsPerGoroutine; j++ {
				// Randomly change level
				if j%10 == 0 {
					level := levels[j%len(levels)]
					levelSwitch.SetLevel(level)
				}
				
				// Log at various levels
				logger.Debug("Debug message from goroutine %d", id)
				logger.Information("Info message from goroutine %d", id)
				logger.Warning("Warning message from goroutine %d", id)
				
				// Brief pause
				if j%20 == 0 {
					time.Sleep(1 * time.Millisecond)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// If we reach here without panics or data races, the test passed
	// The exact number of events is not deterministic due to concurrent level changes
	events := memorySink.Events()
	t.Logf("Concurrent logging produced %d events", len(events))
}

func TestDynamicLevelControl_Performance(t *testing.T) {
	levelSwitch := NewLoggingLevelSwitch(core.WarningLevel)
	memorySink := sinks.NewMemorySink()
	
	logger := New(
		WithSink(memorySink),
		WithLevelSwitch(levelSwitch),
	)

	// Benchmark logging below minimum level (should be very fast)
	start := time.Now()
	const iterations = 100000

	for i := 0; i < iterations; i++ {
		logger.Debug("Debug message that should be filtered")
	}

	duration := time.Since(start)
	nsPerOp := duration.Nanoseconds() / iterations

	// Should be very fast (under 150ns per operation for filtered messages)
	// Allow variance for different architectures, OS (Windows tends to be slower), and Go versions
	expectedThreshold := int64(150)
	if runtime.GOOS == "windows" {
		expectedThreshold = 200 // Windows CI runners are typically slower
	}
	if nsPerOp > expectedThreshold {
		t.Errorf("Dynamic level filtering too slow: %d ns/op (expected < %d ns/op)", nsPerOp, expectedThreshold)
	}

	t.Logf("Dynamic level filtering performance: %d ns/op", nsPerOp)

	// Verify no events were actually logged
	events := memorySink.Events()
	if len(events) > 0 {
		t.Errorf("Expected no events to be logged below minimum level, got %d", len(events))
	}
}

func TestDynamicLevelControl_FluentInterface(t *testing.T) {
	memorySink := sinks.NewMemorySink()
	levelSwitch := NewLoggingLevelSwitch(core.ErrorLevel)
	
	logger := New(
		WithSink(memorySink),
		WithLevelSwitch(levelSwitch),
	)

	// Test fluent interface methods work correctly
	levelSwitch.Debug().Information().Warning().Error()

	if levelSwitch.Level() != core.ErrorLevel {
		t.Errorf("Expected Error level, got %v", levelSwitch.Level())
	}

	logger.Error("Error message")
	events := memorySink.Events()
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	// Chain level changes
	levelSwitch.Verbose().Debug()
	memorySink.Clear()

	logger.Debug("Debug message")
	events = memorySink.Events()
	if len(events) != 1 {
		t.Errorf("Expected 1 event after chained level change, got %d", len(events))
	}
}

func TestDynamicLevelControl_StaticAndDynamic(t *testing.T) {
	memorySink := sinks.NewMemorySink()

	// Test logger with both static level and level switch
	// Level switch should take precedence
	levelSwitch := NewLoggingLevelSwitch(core.DebugLevel)
	logger := New(
		WithSink(memorySink),
		WithMinimumLevel(core.ErrorLevel), // Static level: Error
		WithLevelSwitch(levelSwitch),      // Dynamic level: Debug
	)

	// Debug should pass because dynamic level is Debug (not Error)
	logger.Debug("Debug message")
	events := memorySink.Events()
	if len(events) != 1 {
		t.Errorf("Expected dynamic level to override static level, got %d events", len(events))
	}

	// Test GetMinimumLevel returns dynamic level
	if logger.GetMinimumLevel() != core.DebugLevel {
		t.Errorf("Expected GetMinimumLevel to return dynamic level (Debug), got %v", logger.GetMinimumLevel())
	}

	// Test logger without level switch uses static level
	staticLogger := New(
		WithSink(memorySink),
		WithMinimumLevel(core.ErrorLevel),
	)

	if staticLogger.GetLevelSwitch() != nil {
		t.Error("Expected GetLevelSwitch to return nil for logger without level switch")
	}

	if staticLogger.GetMinimumLevel() != core.ErrorLevel {
		t.Errorf("Expected static logger to return static minimum level, got %v", staticLogger.GetMinimumLevel())
	}
}

func TestDynamicLevelControl_Integration(t *testing.T) {
	// Test a realistic scenario with multiple loggers sharing a level switch
	levelSwitch := NewLoggingLevelSwitch(core.InformationLevel)
	
	// Create multiple sinks
	memorySink := sinks.NewMemorySink()
	consoleSink := sinks.NewMemorySink() // Using memory sink to capture output
	
	// Create loggers for different components that share the same level switch
	userLogger := New(
		WithSink(memorySink),
		WithSink(consoleSink),
		WithLevelSwitch(levelSwitch),
		WithProperty("Component", "User"),
	)
	
	dbLogger := New(
		WithSink(memorySink),
		WithLevelSwitch(levelSwitch),
		WithProperty("Component", "Database"),
	)
	
	httpLogger := New(
		WithSink(consoleSink),
		WithLevelSwitch(levelSwitch),
		WithProperty("Component", "HTTP"),
	)

	// Log at Information level
	userLogger.Information("User logged in")
	dbLogger.Information("Database query executed")
	httpLogger.Information("HTTP request processed")
	
	// All should be logged
	memoryEvents := memorySink.Events()
	consoleEvents := consoleSink.Events()
	
	if len(memoryEvents) != 2 { // userLogger and dbLogger
		t.Errorf("Expected 2 events in memory sink, got %d", len(memoryEvents))
	}
	
	if len(consoleEvents) != 2 { // userLogger and httpLogger
		t.Errorf("Expected 2 events in console sink, got %d", len(consoleEvents))
	}

	// Change level to Warning - should filter out subsequent Information messages
	levelSwitch.Warning()
	memorySink.Clear()
	consoleSink.Clear()

	userLogger.Information("User action") // Filtered
	dbLogger.Warning("Database slow query") // Pass
	httpLogger.Error("HTTP error") // Pass

	memoryEvents = memorySink.Events()
	consoleEvents = consoleSink.Events()
	
	if len(memoryEvents) != 1 { // Only dbLogger warning
		t.Errorf("Expected 1 event in memory sink after level change, got %d", len(memoryEvents))
	}
	
	if len(consoleEvents) != 1 { // Only httpLogger error
		t.Errorf("Expected 1 event in console sink after level change, got %d", len(consoleEvents))
	}

	// Verify component properties are preserved
	if memoryEvents[0].Properties["Component"] != "Database" {
		t.Errorf("Expected Database component, got %v", memoryEvents[0].Properties["Component"])
	}
	
	if consoleEvents[0].Properties["Component"] != "HTTP" {
		t.Errorf("Expected HTTP component, got %v", consoleEvents[0].Properties["Component"])
	}
}