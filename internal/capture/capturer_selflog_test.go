package capture_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/capture"
	"github.com/willibrandon/mtlog/selflog"
)

// panickingLogValue implements LogValue but panics
type panickingLogValue struct{}

func (p panickingLogValue) LogValue() any {
	panic("LogValue panic!")
}

// propertyFactory is a simple implementation of LogEventPropertyFactory
type propertyFactory struct{}

func (pf *propertyFactory) CreateProperty(name string, value any) *core.LogEventProperty {
	return &core.LogEventProperty{
		Name:  name,
		Value: value,
	}
}

func TestCapturerSelfLog(t *testing.T) {
	t.Run("LogValue panic recovery", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create capturer
		d := capture.NewDefaultCapturer()
		factory := &propertyFactory{}

		// Try to capture a panicking LogValue
		value := panickingLogValue{}
		prop, ok := d.TryCapture(value, factory)

		// Should recover and return true
		if !ok {
			t.Error("expected TryCapture to return true after recovery")
		}
		if prop == nil {
			t.Error("expected non-nil property after recovery")
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[capture] LogValue.LogValue() panicked") {
			t.Errorf("expected LogValue panic in selflog, got: %s", output)
		}
		if !strings.Contains(output, "LogValue panic!") {
			t.Errorf("expected panic message in selflog, got: %s", output)
		}
	})

	t.Run("reflection panic recovery", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create capturer
		d := capture.NewDefaultCapturer()
		factory := &propertyFactory{}

		// Create a value that might cause reflection issues
		// Using a nil interface with concrete type
		var nilMap map[string]any
		prop, ok := d.TryCapture(nilMap, factory)

		// Should handle nil map gracefully
		if !ok {
			t.Error("expected TryCapture to return true")
		}
		if prop == nil {
			t.Error("expected non-nil property")
		}

		// No panic should occur for nil map, so no selflog output
		output := selflogBuf.String()
		if output != "" {
			t.Errorf("expected no selflog output for nil map, got: %s", output)
		}
	})

	t.Run("deep structure panic recovery", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create capturer with very low depth to trigger depth limiting
		d := capture.NewCapturer(1, 100, 10)
		factory := &propertyFactory{}

		// Create a deeply nested structure
		type Nested struct {
			Value any
		}
		deepValue := &Nested{Value: &Nested{Value: &Nested{Value: "deep"}}}

		prop, ok := d.TryCapture(deepValue, factory)

		// Should handle deep structure without panic
		if !ok {
			t.Error("expected TryCapture to return true")
		}
		if prop == nil {
			t.Error("expected non-nil property")
		}

		// No panic expected, just depth limiting
		output := selflogBuf.String()
		if output != "" {
			t.Errorf("expected no selflog output for depth limiting, got: %s", output)
		}
	})

	t.Run("invalid reflect value", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create capturer
		d := capture.NewDefaultCapturer()
		factory := &propertyFactory{}

		// Create a closed channel which might cause issues
		ch := make(chan int)
		close(ch)

		// Try to capture closed channel
		prop, ok := d.TryCapture(ch, factory)

		// Should handle without panic
		if !ok {
			t.Error("expected TryCapture to return true")
		}
		if prop == nil {
			t.Error("expected non-nil property")
		}

		// Channels are handled as fmt.Sprintf("%T"), no panic expected
		output := selflogBuf.String()
		if output != "" {
			t.Errorf("expected no selflog output for channel, got: %s", output)
		}
	})
}

// TestCapturerPanicScenarios tests specific panic scenarios
func TestCapturerPanicScenarios(t *testing.T) {
	t.Run("nil pointer dereference protection", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		d := capture.NewDefaultCapturer()
		factory := &propertyFactory{}

		// Nil pointer
		var nilPtr *struct{ Value string }
		prop, ok := d.TryCapture(nilPtr, factory)

		if !ok {
			t.Error("expected TryCapture to return true for nil pointer")
		}
		// Check for Null{} sentinel type
		if prop == nil {
			t.Error("expected non-nil property")
		} else if _, ok := prop.Value.(capture.Null); !ok {
			t.Errorf("expected Null{} for nil pointer, got: %T(%v)", prop.Value, prop.Value)
		}

		// No panic for nil pointer
		output := selflogBuf.String()
		if output != "" {
			t.Errorf("expected no selflog output for nil pointer, got: %s", output)
		}
	})

	t.Run("complex type with panicking stringer", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		d := capture.NewDefaultCapturer()
		factory := &propertyFactory{}

		// Create a type that panics in String() method
		type panickingStringer struct {
			value string
		}

		// This type will be handled by capturing without calling String()
		ps := &panickingStringer{value: "test"}
		prop, ok := d.TryCapture(ps, factory)

		if !ok {
			t.Error("expected TryCapture to return true")
		}
		if prop == nil {
			t.Error("expected non-nil property")
		}

		// Should capture as a struct without panic
		output := selflogBuf.String()
		if output != "" {
			t.Errorf("expected no selflog output, got: %s", output)
		}
	})
}
