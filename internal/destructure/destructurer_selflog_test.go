package destructure_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/destructure"
	"github.com/willibrandon/mtlog/selflog"
)

// panickingLogValue implements LogValue but panics
type panickingLogValue struct{}

func (p panickingLogValue) LogValue() interface{} {
	panic("LogValue panic!")
}


// propertyFactory is a simple implementation of LogEventPropertyFactory
type propertyFactory struct{}

func (pf *propertyFactory) CreateProperty(name string, value interface{}) *core.LogEventProperty {
	return &core.LogEventProperty{
		Name:  name,
		Value: value,
	}
}

func TestDestructurerSelfLog(t *testing.T) {
	t.Run("LogValue panic recovery", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create destructurer
		d := destructure.NewDefaultDestructurer()
		factory := &propertyFactory{}

		// Try to destructure a panicking LogValue
		value := panickingLogValue{}
		prop, ok := d.TryDestructure(value, factory)

		// Should recover and return true
		if !ok {
			t.Error("expected TryDestructure to return true after recovery")
		}
		if prop == nil {
			t.Error("expected non-nil property after recovery")
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[destructure] LogValue.LogValue() panicked") {
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

		// Create destructurer
		d := destructure.NewDefaultDestructurer()
		factory := &propertyFactory{}

		// Create a value that might cause reflection issues
		// Using a nil interface with concrete type
		var nilMap map[string]interface{}
		prop, ok := d.TryDestructure(nilMap, factory)

		// Should handle nil map gracefully
		if !ok {
			t.Error("expected TryDestructure to return true")
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

		// Create destructurer with very low depth to trigger depth limiting
		d := destructure.NewDestructurer(1, 100, 10)
		factory := &propertyFactory{}

		// Create a deeply nested structure
		type Nested struct {
			Value interface{}
		}
		deepValue := &Nested{Value: &Nested{Value: &Nested{Value: "deep"}}}

		prop, ok := d.TryDestructure(deepValue, factory)

		// Should handle deep structure without panic
		if !ok {
			t.Error("expected TryDestructure to return true")
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

		// Create destructurer
		d := destructure.NewDefaultDestructurer()
		factory := &propertyFactory{}

		// Create a closed channel which might cause issues
		ch := make(chan int)
		close(ch)

		// Try to destructure closed channel
		prop, ok := d.TryDestructure(ch, factory)

		// Should handle without panic
		if !ok {
			t.Error("expected TryDestructure to return true")
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

// TestDestructurerPanicScenarios tests specific panic scenarios
func TestDestructurerPanicScenarios(t *testing.T) {
	t.Run("nil pointer dereference protection", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		d := destructure.NewDefaultDestructurer()
		factory := &propertyFactory{}

		// Nil pointer
		var nilPtr *struct{ Value string }
		prop, ok := d.TryDestructure(nilPtr, factory)

		if !ok {
			t.Error("expected TryDestructure to return true for nil pointer")
		}
		if prop == nil || prop.Value != nil {
			t.Errorf("expected nil value for nil pointer, got: %v", prop)
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

		d := destructure.NewDefaultDestructurer()
		factory := &propertyFactory{}

		// Create a type that panics in String() method
		type panickingStringer struct {
			value string
		}
		
		// This type will be handled by destructuring without calling String()
		ps := &panickingStringer{value: "test"}
		prop, ok := d.TryDestructure(ps, factory)

		if !ok {
			t.Error("expected TryDestructure to return true")
		}
		if prop == nil {
			t.Error("expected non-nil property")
		}

		// Should destructure as a struct without panic
		output := selflogBuf.String()
		if output != "" {
			t.Errorf("expected no selflog output, got: %s", output)
		}
	})
}