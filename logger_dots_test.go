package mtlog

import (
	"testing"
	"github.com/willibrandon/mtlog/sinks"
	"github.com/willibrandon/mtlog/internal/parser"
)

func TestLoggerWithDottedPropertyNames(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Test basic OTEL-style properties
	logger.Information("HTTP request {http.method} to {http.url} took {http.duration.ms}ms", "GET", "/api/users", 123.45)
	
	if len(sink.Events()) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(sink.Events()))
	}
	
	event := sink.Events()[0]
	
	// Check properties
	if method, ok := event.Properties["http.method"].(string); !ok || method != "GET" {
		t.Errorf("Expected http.method='GET', got %v", event.Properties["http.method"])
	}
	
	if url, ok := event.Properties["http.url"].(string); !ok || url != "/api/users" {
		t.Errorf("Expected http.url='/api/users', got %v", event.Properties["http.url"])
	}
	
	if duration, ok := event.Properties["http.duration.ms"].(float64); !ok || duration != 123.45 {
		t.Errorf("Expected http.duration.ms=123.45, got %v", event.Properties["http.duration.ms"])
	}
	
	// Test rendered message by parsing and rendering
	mt, err := parser.Parse(event.MessageTemplate)
	if err != nil {
		t.Fatalf("Failed to parse message template: %v", err)
	}
	renderedMsg := mt.Render(event.Properties)
	expectedMsg := "HTTP request GET to /api/users took 123.45ms"
	if renderedMsg != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, renderedMsg)
	}
}

func TestLoggerWithComplexDottedProperties(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Test various OTEL semantic conventions
	logger.Information("Service {service.name} version {service.version} in {service.namespace}", "api-gateway", "2.1.0", "production")
	logger.Information("Database {db.system} operation {db.operation} on {db.name}", "postgresql", "SELECT", "users")
	logger.Information("Span {trace.span_id} with parent {trace.parent_id}", "abc123", "parent456")
	
	if len(sink.Events()) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(sink.Events()))
	}
	
	// Verify service properties
	serviceEvent := sink.Events()[0]
	if name, ok := serviceEvent.Properties["service.name"].(string); !ok || name != "api-gateway" {
		t.Errorf("Expected service.name='api-gateway', got %v", serviceEvent.Properties["service.name"])
	}
	
	// Verify database properties
	dbEvent := sink.Events()[1]
	if system, ok := dbEvent.Properties["db.system"].(string); !ok || system != "postgresql" {
		t.Errorf("Expected db.system='postgresql', got %v", dbEvent.Properties["db.system"])
	}
	
	// Verify trace properties (with underscore in property name)
	traceEvent := sink.Events()[2]
	if spanId, ok := traceEvent.Properties["trace.span_id"].(string); !ok || spanId != "abc123" {
		t.Errorf("Expected trace.span_id='abc123', got %v", traceEvent.Properties["trace.span_id"])
	}
}

func TestLoggerWithDottedPropertiesEdgeCases(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Test edge cases
	logger.Information("Property {ends.with.dot.}", "value1")
	logger.Information("Property {has..consecutive..dots}", "value2")
	logger.Information("Property {many.levels.of.nesting.here}", "value3")
	
	if len(sink.Events()) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(sink.Events()))
	}
	
	// Check edge case properties
	if val, ok := sink.Events()[0].Properties["ends.with.dot."].(string); !ok || val != "value1" {
		t.Errorf("Expected property with trailing dot, got %v", sink.Events()[0].Properties)
	}
	
	if val, ok := sink.Events()[1].Properties["has..consecutive..dots"].(string); !ok || val != "value2" {
		t.Errorf("Expected property with consecutive dots, got %v", sink.Events()[1].Properties)
	}
	
	if val, ok := sink.Events()[2].Properties["many.levels.of.nesting.here"].(string); !ok || val != "value3" {
		t.Errorf("Expected property with many dots, got %v", sink.Events()[2].Properties)
	}
}

func TestLoggerWithDottedPropertiesFormatting(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Test with format specifiers
	logger.Information("Duration: {http.duration.ms:F2}ms, Status: {http.status.code:000}", 123.456, 200)
	
	event := sink.Events()[0]
	
	// Check the formatted message by parsing and rendering
	mt, err := parser.Parse(event.MessageTemplate)
	if err != nil {
		t.Fatalf("Failed to parse message template: %v", err)
	}
	renderedMsg := mt.Render(event.Properties)
	expectedMsg := "Duration: 123.46ms, Status: 200"
	if renderedMsg != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, renderedMsg)
	}
	
	// Properties should still have original values
	if duration, ok := event.Properties["http.duration.ms"].(float64); !ok || duration != 123.456 {
		t.Errorf("Expected http.duration.ms=123.456, got %v", event.Properties["http.duration.ms"])
	}
}

func TestLoggerWithMixedPropertyStyles(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))

	// Mix traditional PascalCase with OTEL-style dotted properties
	logger.Information("User {UserId} made {http.method} request to {http.url} from {ClientIP}", 
		123, "POST", "/api/orders", "192.168.1.100")
	
	event := sink.Events()[0]
	
	// Check both styles work together
	if userId, ok := event.Properties["UserId"].(int); !ok || userId != 123 {
		t.Errorf("Expected UserId=123, got %v", event.Properties["UserId"])
	}
	
	if method, ok := event.Properties["http.method"].(string); !ok || method != "POST" {
		t.Errorf("Expected http.method='POST', got %v", event.Properties["http.method"])
	}
	
	if ip, ok := event.Properties["ClientIP"].(string); !ok || ip != "192.168.1.100" {
		t.Errorf("Expected ClientIP='192.168.1.100', got %v", event.Properties["ClientIP"])
	}
}

func TestContextWithDottedPropertyNames(t *testing.T) {
	sink := sinks.NewMemorySink()
	logger := New(WithSink(sink))
	
	// Test ForContext with dotted property names
	contextLogger := logger.ForContext("trace.id", "xyz789").
		ForContext("http.request_id", "req-123")
	
	contextLogger.Information("Processing request")
	
	event := sink.Events()[0]
	
	// Check context properties with dots
	if traceId, ok := event.Properties["trace.id"].(string); !ok || traceId != "xyz789" {
		t.Errorf("Expected trace.id='xyz789', got %v", event.Properties["trace.id"])
	}
	
	if reqId, ok := event.Properties["http.request_id"].(string); !ok || reqId != "req-123" {
		t.Errorf("Expected http.request_id='req-123', got %v", event.Properties["http.request_id"])
	}
}