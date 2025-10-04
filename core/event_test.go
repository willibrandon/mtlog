package core

import (
	"testing"
)

// TestRenderMessage demonstrates that RenderMessage() fixes issue #64.
// It shows that RenderMessage() correctly handles all template features.
func TestRenderMessage(t *testing.T) {
	t.Run("capturing operator", func(t *testing.T) {
		event := &LogEvent{
			MessageTemplate: "Configuration: {@Config}",
			Properties: map[string]any{
				"Config": map[string]any{
					"debug": true,
					"port":  8080,
				},
			},
		}

		got := event.RenderMessage()
		expected := "Configuration: map[debug:true port:8080]"

		if got != expected {
			t.Errorf("RenderMessage() = %q, want %q", got, expected)
		}
	})

	t.Run("scalar hint", func(t *testing.T) {
		event := &LogEvent{
			MessageTemplate: "Values: {$Values}",
			Properties: map[string]any{
				"Values": []int{1, 2, 3},
			},
		}

		got := event.RenderMessage()
		expected := "Values: [1 2 3]"

		if got != expected {
			t.Errorf("RenderMessage() = %q, want %q", got, expected)
		}
	})

	t.Run("format specifier", func(t *testing.T) {
		event := &LogEvent{
			MessageTemplate: "Count: {Count:000}",
			Properties: map[string]any{
				"Count": 42,
			},
		}

		got := event.RenderMessage()
		expected := "Count: 042"

		if got != expected {
			t.Errorf("RenderMessage() = %q, want %q", got, expected)
		}
	})

	t.Run("simple properties", func(t *testing.T) {
		event := &LogEvent{
			MessageTemplate: "User {UserId} logged in from {City}",
			Properties: map[string]any{
				"UserId": 123,
				"City":   "Seattle",
			},
		}

		got := event.RenderMessage()
		expected := "User 123 logged in from Seattle"

		if got != expected {
			t.Errorf("RenderMessage() = %q, want %q", got, expected)
		}
	})
}