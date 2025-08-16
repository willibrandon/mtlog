//go:build go1.18
// +build go1.18

package otel

import (
	"context"
	"fmt"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/willibrandon/mtlog/core"
	olog "go.opentelemetry.io/otel/log"
)

// FuzzBridgeMessageTemplate fuzzes the bridge with various message templates
func FuzzBridgeMessageTemplate(f *testing.F) {
	// Add seed corpus
	f.Add("Simple message")
	f.Add("Message with {Property}")
	f.Add("Multiple {First} and {Second} properties")
	f.Add("Nested {User.Name} property")
	f.Add("Format specifier {Count:000}")
	f.Add("Special chars: 你好 👋 \n\t\r")
	f.Add("{")
	f.Add("}")
	f.Add("{}")
	f.Add("{{escaped}}")
	f.Add("{Property with spaces}")
	f.Add("Very long property name {ThisIsAVeryLongPropertyNameThatMightCauseIssuesInSomeImplementations}")
	f.Add("")

	// Create a mock logger for testing
	logger := &mockFuzzLogger{}
	bridge := NewBridge(logger)

	f.Fuzz(func(t *testing.T, template string) {
		// Skip invalid UTF-8
		if !utf8.ValidString(template) {
			t.Skip("Skipping invalid UTF-8")
		}

		// Create a record with the fuzzed template
		ctx := context.Background()
		var record olog.Record
		record.SetTimestamp(time.Now())
		record.SetSeverity(olog.SeverityInfo)
		record.SetBody(olog.StringValue(template))

		// Add some properties
		record.AddAttributes(
			olog.String("fuzz", "test"),
			olog.Int("count", 42),
			olog.Bool("active", true),
		)

		// This should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic with template %q: %v", template, r)
				}
			}()
			bridge.Emit(ctx, record)
		}()
	})
}

// FuzzHandlerProperties fuzzes the handler with various property types
func FuzzHandlerProperties(f *testing.F) {
	// Add seed corpus with different property combinations
	seedProps := []map[string]any{
		{"simple": "string"},
		{"number": 42},
		{"float": 3.14},
		{"bool": true},
		{"nil": nil},
		{"array": []string{"a", "b", "c"}},
		{"nested": map[string]any{"inner": "value"}},
		{"mixed": []any{1, "two", 3.0, true}},
		{"empty": ""},
		{"special": "!@#$%^&*()"},
		{"unicode": "你好世界 🌍"},
		{"escape": "quote\" and backslash\\"},
	}

	for _, props := range seedProps {
		f.Add(serializeProps(props))
	}

	// Create mock OTEL logger
	mockLogger := &mockOTELLogger{}
	handler := NewHandler(mockLogger)

	f.Fuzz(func(t *testing.T, propData string) {
		// Deserialize properties (simplified for fuzzing)
		props := deserializeProps(propData)

		event := &core.LogEvent{
			Timestamp:       time.Now(),
			Level:           core.InformationLevel,
			MessageTemplate: "Fuzz test {Property}",
			Properties:      props,
		}

		// This should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic with properties %v: %v", props, r)
				}
			}()
			handler.Emit(event)
		}()
	})
}

// FuzzEnricherContext fuzzes enrichers with various context values
func FuzzEnricherContext(f *testing.F) {
	// Seed with various context scenarios
	f.Add([]byte{}) // Empty context value
	f.Add([]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}) // Valid trace ID
	f.Add([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}) // Max values

	factory := &mockPropertyFactory{}

	f.Fuzz(func(t *testing.T, contextData []byte) {
		// Create various enricher types
		ctx := context.Background()
		
		// Add some fuzzed data to context (simulating trace context)
		if len(contextData) > 0 {
			ctx = context.WithValue(ctx, "fuzz", contextData)
		}

		enrichers := []core.LogEventEnricher{
			NewFastOTELEnricher(ctx),
			NewStaticOTELEnricher(ctx),
			NewOTELEnricher(ctx),
		}

		for _, enricher := range enrichers {
			event := &core.LogEvent{
				Properties: make(map[string]any),
			}

			// Should not panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic in enricher with context data %x: %v", contextData, r)
					}
				}()
				enricher.Enrich(event, factory)
			}()
		}
	})
}

// FuzzOTLPSinkConfiguration fuzzes OTLP sink configuration
func FuzzOTLPSinkConfiguration(f *testing.F) {
	// Seed with various configurations
	configs := []struct {
		endpoint     string
		batchSize    int
		batchTimeout int // milliseconds
		maxQueue     int
	}{
		{"localhost:4317", 10, 100, 1000},
		{"", 1, 1, 1},
		{"invalid endpoint!", 0, 0, 0},
		{"http://example.com:8080", 1000000, 60000, 1000000},
		{"256.256.256.256:99999", -1, -1, -1},
	}

	for _, cfg := range configs {
		f.Add(cfg.endpoint, cfg.batchSize, cfg.batchTimeout, cfg.maxQueue)
	}

	f.Fuzz(func(t *testing.T, endpoint string, batchSize, batchTimeout, maxQueue int) {
		// Skip invalid inputs
		if batchSize < 0 || batchTimeout < 0 || maxQueue < 0 {
			t.Skip("Skipping negative values")
		}
		if batchSize > 1000000 || batchTimeout > 3600000 || maxQueue > 10000000 {
			t.Skip("Skipping unreasonably large values")
		}

		// Attempt to create sink with fuzzed configuration
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Panic with config endpoint=%q batch=%d timeout=%d queue=%d: %v",
						endpoint, batchSize, batchTimeout, maxQueue, r)
				}
			}()

			opts := []OTLPOption{}
			if endpoint != "" {
				opts = append(opts, WithOTLPEndpoint(endpoint))
				// Use insecure for localhost testing
				if endpoint == "localhost:4317" || endpoint == "localhost:4318" {
					opts = append(opts, WithOTLPInsecure())
				}
			}
			if batchSize > 0 && batchTimeout > 0 {
				opts = append(opts, WithOTLPBatching(batchSize, time.Duration(batchTimeout)*time.Millisecond))
			}
			if maxQueue > 0 {
				opts = append(opts, WithOTLPMaxQueueSize(maxQueue))
			}

			sink, err := NewOTLPSink(opts...)
			if err != nil {
				// Error is ok, we're testing for panics
				return
			}
			defer sink.Close()

			// Try to emit an event
			event := &core.LogEvent{
				Timestamp:       time.Now(),
				Level:           core.InformationLevel,
				MessageTemplate: "Fuzz test",
				Properties:      map[string]any{},
			}
			sink.Emit(event)
		}()
	})
}

// Helper functions for fuzzing

func serializeProps(props map[string]any) string {
	// Simple serialization for fuzzing
	result := ""
	for k, v := range props {
		result += k + "=" + fmt.Sprintf("%v", v) + ";"
	}
	return result
}

func deserializeProps(data string) map[string]any {
	// Simple deserialization for fuzzing
	props := make(map[string]any)
	// Very basic parsing, just for fuzzing
	if data != "" {
		props["fuzzed"] = data
	}
	return props
}

// Mock implementations for fuzzing

type mockPropertyFactory struct{}

func (f *mockPropertyFactory) CreateProperty(name string, value any) *core.LogEventProperty {
	return &core.LogEventProperty{
		Name:  name,
		Value: value,
	}
}

type mockFuzzLogger struct {
	events []string
}

func (m *mockFuzzLogger) Log(timestamp time.Time, level core.LogEventLevel, template string, err error, props []core.LogEventProperty) {
	m.events = append(m.events, template)
}

func (m *mockFuzzLogger) IsEnabled(level core.LogEventLevel) bool {
	return true
}

func (m *mockFuzzLogger) Verbose(template string, args ...any) {}
func (m *mockFuzzLogger) Debug(template string, args ...any) {}
func (m *mockFuzzLogger) Information(template string, args ...any) {}
func (m *mockFuzzLogger) Warning(template string, args ...any) {}
func (m *mockFuzzLogger) Error(template string, args ...any) {}
func (m *mockFuzzLogger) Fatal(template string, args ...any) {}
func (m *mockFuzzLogger) Write(level core.LogEventLevel, template string, args ...any) {}
func (m *mockFuzzLogger) ForContext(propertyName string, value any) core.Logger { return m }
func (m *mockFuzzLogger) WithContext(ctx context.Context) core.Logger { return m }
func (m *mockFuzzLogger) With(args ...any) core.Logger { return m }
func (m *mockFuzzLogger) Info(template string, args ...any) {}
func (m *mockFuzzLogger) Warn(template string, args ...any) {}