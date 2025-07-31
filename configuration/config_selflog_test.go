package configuration_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/willibrandon/mtlog/configuration"
	"github.com/willibrandon/mtlog/selflog"
)

func TestConfigurationSelfLog(t *testing.T) {
	t.Run("unknown log level warning", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Parse unknown level
		level, err := configuration.ParseLevel("SuperVerbose")

		// Should return error but also default level
		if err == nil {
			t.Error("expected error for unknown level")
		}
		if level != 2 { // InformationLevel
			t.Errorf("expected Information level (2), got %d", level)
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[configuration] unknown log level 'SuperVerbose'") {
			t.Errorf("expected unknown log level warning in selflog, got: %s", output)
		}
	})

	t.Run("type mismatch warnings", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Test GetString with wrong type
		args := map[string]any{
			"path": 123, // number instead of string
		}

		result := configuration.GetString(args, "path", "/default/path")
		if result != "/default/path" {
			t.Errorf("expected default value, got %s", result)
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[configuration] expected string for 'path', got int") {
			t.Errorf("expected type mismatch warning in selflog, got: %s", output)
		}
	})

	t.Run("int parse failure warning", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Test GetInt with unparseable string
		args := map[string]any{
			"port": "not-a-number",
		}

		result := configuration.GetInt(args, "port", 8080)
		if result != 8080 {
			t.Errorf("expected default value 8080, got %d", result)
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[configuration] failed to parse 'not-a-number' as int for 'port'") {
			t.Errorf("expected parse failure warning in selflog, got: %s", output)
		}
	})

	t.Run("unknown sink type warning", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create configuration with unknown sink
		config := &configuration.Configuration{
			Mtlog: configuration.LoggerConfiguration{
				WriteTo: []configuration.SinkConfiguration{
					{
						Name: "UnknownSink",
						Args: map[string]any{},
					},
				},
			},
		}

		// Try to build logger
		builder := configuration.NewLoggerBuilder()
		_, err := builder.Build(config)

		// Should fail
		if err == nil {
			t.Error("expected error for unknown sink")
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[configuration] unknown sink type 'UnknownSink'") {
			t.Errorf("expected unknown sink warning in selflog, got: %s", output)
		}
		if !strings.Contains(output, "available sinks:") {
			t.Errorf("expected available sinks list in selflog, got: %s", output)
		}
	})

	t.Run("unknown enricher warning", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create configuration with unknown enricher
		config := &configuration.Configuration{
			Mtlog: configuration.LoggerConfiguration{
				Enrich: []string{"UnknownEnricher"},
			},
		}

		// Try to build logger
		builder := configuration.NewLoggerBuilder()
		_, err := builder.Build(config)

		// Should fail
		if err == nil {
			t.Error("expected error for unknown enricher")
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[configuration] unknown enricher 'UnknownEnricher'") {
			t.Errorf("expected unknown enricher warning in selflog, got: %s", output)
		}
	})

	t.Run("unknown filter warning", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create configuration with unknown filter
		config := &configuration.Configuration{
			Mtlog: configuration.LoggerConfiguration{
				Filter: []configuration.FilterConfiguration{
					{
						Name: "UnknownFilter",
						Args: map[string]any{},
					},
				},
			},
		}

		// Try to build logger
		builder := configuration.NewLoggerBuilder()
		_, err := builder.Build(config)

		// Should fail
		if err == nil {
			t.Error("expected error for unknown filter")
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[configuration] unknown filter 'UnknownFilter'") {
			t.Errorf("expected unknown filter warning in selflog, got: %s", output)
		}
	})

	t.Run("unknown console theme warning", func(t *testing.T) {
		// Setup selflog capture
		var selflogBuf bytes.Buffer
		selflog.Enable(selflog.Sync(&selflogBuf))
		defer selflog.Disable()

		// Create configuration with unknown theme
		config := &configuration.Configuration{
			Mtlog: configuration.LoggerConfiguration{
				WriteTo: []configuration.SinkConfiguration{
					{
						Name: "Console",
						Args: map[string]any{
							"theme": "SuperColorful",
						},
					},
				},
			},
		}

		// Try to build logger
		builder := configuration.NewLoggerBuilder()
		logger, err := builder.Build(config)

		// Should succeed with default theme
		if err != nil {
			t.Errorf("expected success with default theme, got error: %v", err)
		}
		if logger == nil {
			t.Error("expected logger to be created")
		}

		// Check selflog output
		output := selflogBuf.String()
		t.Logf("selflog output: %q", output)
		if !strings.Contains(output, "[configuration] unknown console theme 'SuperColorful'") {
			t.Errorf("expected unknown theme warning in selflog, got: %s", output)
		}
	})
}
