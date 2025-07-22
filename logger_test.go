package mtlog

import (
	"strings"
	"sync"
	"testing"
	
	"github.com/willibrandon/mtlog/core"
)

// memorySink is a test sink that stores events in memory.
type memorySink struct {
	events []core.LogEvent
	mu     sync.Mutex
}

func (ms *memorySink) Emit(event *core.LogEvent) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.events = append(ms.events, *event)
}

func (ms *memorySink) Close() error {
	return nil
}

func (ms *memorySink) GetEvents() []core.LogEvent {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	events := make([]core.LogEvent, len(ms.events))
	copy(events, ms.events)
	return events
}

func TestLoggerLevels(t *testing.T) {
	sink := &memorySink{}
	logger := New()
	logger.AddSink(sink)
	
	// Set minimum level to Information
	logger.SetMinimumLevel(core.InformationLevel)
	
	// Log at different levels
	logger.Verbose("Verbose message")
	logger.Debug("Debug message")
	logger.Information("Info message")
	logger.Warning("Warning message")
	logger.Error("Error message")
	logger.Fatal("Fatal message")
	
	events := sink.GetEvents()
	
	// Should have 4 events (Information and above)
	if len(events) != 4 {
		t.Errorf("Expected 4 events, got %d", len(events))
	}
	
	// Check levels
	expectedLevels := []core.LogEventLevel{
		core.InformationLevel,
		core.WarningLevel,
		core.ErrorLevel,
		core.FatalLevel,
	}
	
	for i, event := range events {
		if event.Level != expectedLevels[i] {
			t.Errorf("Event %d: expected level %d, got %d", i, expectedLevels[i], event.Level)
		}
	}
}

func TestLoggerPropertyExtraction(t *testing.T) {
	sink := &memorySink{}
	logger := New()
	logger.AddSink(sink)
	
	// Log with properties
	logger.Information("User {UserId} logged in from {IpAddress}", 123, "192.168.1.1")
	
	events := sink.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	
	event := events[0]
	
	// Check properties
	if userId, ok := event.Properties["UserId"].(int); !ok || userId != 123 {
		t.Errorf("Expected UserId=123, got %v", event.Properties["UserId"])
	}
	
	if ip, ok := event.Properties["IpAddress"].(string); !ok || ip != "192.168.1.1" {
		t.Errorf("Expected IpAddress='192.168.1.1', got %v", event.Properties["IpAddress"])
	}
}

func TestLoggerForContext(t *testing.T) {
	sink := &memorySink{}
	logger := New()
	logger.AddSink(sink)
	
	// Create context logger
	ctxLogger := logger.ForContext("Environment", "Production")
	ctxLogger.Information("Test message")
	
	events := sink.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	
	event := events[0]
	
	// Check context property
	if env, ok := event.Properties["Environment"].(string); !ok || env != "Production" {
		t.Errorf("Expected Environment='Production', got %v", event.Properties["Environment"])
	}
}

func TestLoggerMultipleContexts(t *testing.T) {
	sink := &memorySink{}
	logger := New()
	logger.AddSink(sink)
	
	// Create nested context loggers
	ctx1 := logger.ForContext("Environment", "Production")
	ctx2 := ctx1.ForContext("Version", "1.0.0")
	ctx2.Information("Test message")
	
	events := sink.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	
	event := events[0]
	
	// Check both context properties
	if env, ok := event.Properties["Environment"].(string); !ok || env != "Production" {
		t.Errorf("Expected Environment='Production', got %v", event.Properties["Environment"])
	}
	
	if ver, ok := event.Properties["Version"].(string); !ok || ver != "1.0.0" {
		t.Errorf("Expected Version='1.0.0', got %v", event.Properties["Version"])
	}
}

func TestLoggerEnrichers(t *testing.T) {
	sink := &memorySink{}
	logger := New()
	logger.AddSink(sink)
	
	// Add a test enricher
	testEnricher := &testEnricher{property: "TestProp", value: "TestValue"}
	logger.AddEnricher(testEnricher)
	
	logger.Information("Test message")
	
	events := sink.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}
	
	event := events[0]
	
	// Check enriched property
	if val, ok := event.Properties["TestProp"].(string); !ok || val != "TestValue" {
		t.Errorf("Expected TestProp='TestValue', got %v", event.Properties["TestProp"])
	}
}

func TestLoggerFilters(t *testing.T) {
	sink := &memorySink{}
	logger := New()
	logger.AddSink(sink)
	
	// Add a filter that blocks messages containing "secret"
	filter := &containsFilter{substring: "secret"}
	logger.AddFilter(filter)
	
	logger.Information("Public message")
	logger.Information("This contains secret information")
	logger.Information("Another public message")
	
	events := sink.GetEvents()
	if len(events) != 2 {
		t.Errorf("Expected 2 events (filtered), got %d", len(events))
	}
	
	// Check that secret message was filtered
	for _, event := range events {
		if strings.Contains(event.MessageTemplate, "secret") {
			t.Error("Secret message should have been filtered")
		}
	}
}

func TestLoggerConcurrency(t *testing.T) {
	sink := &memorySink{}
	logger := New()
	logger.AddSink(sink)
	
	// Test concurrent logging
	var wg sync.WaitGroup
	numGoroutines := 10
	numLogs := 100
	
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numLogs; j++ {
				logger.Information("Goroutine {Id} log {Number}", id, j)
			}
		}(i)
	}
	
	wg.Wait()
	
	events := sink.GetEvents()
	expectedEvents := numGoroutines * numLogs
	if len(events) != expectedEvents {
		t.Errorf("Expected %d events, got %d", expectedEvents, len(events))
	}
}

// Test helpers

type testEnricher struct {
	property string
	value    interface{}
}

func (te *testEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	prop := propertyFactory.CreateProperty(te.property, te.value)
	event.Properties[prop.Name] = prop.Value
}

type containsFilter struct {
	substring string
}

func (cf *containsFilter) IsEnabled(event *core.LogEvent) bool {
	return !strings.Contains(event.MessageTemplate, cf.substring)
}