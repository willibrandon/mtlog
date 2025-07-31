package configuration

import (
	"fmt"
	"maps"
	"os"

	"github.com/willibrandon/mtlog/core"
)

// CreateLoggerFromFile creates a logger from a JSON configuration file.
// This is the main entry point for configuration-based logger creation.
func CreateLoggerFromFile(filename string) (core.Logger, error) {
	config, err := LoadFromFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	builder := NewLoggerBuilder()
	return builder.Build(config)
}

// CreateLoggerFromJSON creates a logger from JSON configuration data.
func CreateLoggerFromJSON(jsonData []byte) (core.Logger, error) {
	config, err := LoadFromJSON(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	builder := NewLoggerBuilder()
	return builder.Build(config)
}

// CreateLoggerFromEnvironment creates a logger from environment-specific config files.
// It looks for appsettings.json and appsettings.{environment}.json files.
func CreateLoggerFromEnvironment(environment string) (core.Logger, error) {
	// Load base configuration
	baseConfig, err := LoadFromFile("appsettings.json")
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load base configuration: %w", err)
	}

	// If no base config, create empty one
	if baseConfig == nil {
		baseConfig = &Configuration{
			Mtlog: LoggerConfiguration{
				MinimumLevel: "Information",
			},
		}
	}

	// Load environment-specific configuration if provided
	if environment != "" {
		envFile := fmt.Sprintf("appsettings.%s.json", environment)
		envConfig, err := LoadFromFile(envFile)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load environment configuration: %w", err)
		}

		if envConfig != nil {
			// Merge configurations (environment overrides base)
			mergeConfiguration(baseConfig, envConfig)
		}
	}

	builder := NewLoggerBuilder()
	return builder.Build(baseConfig)
}

// mergeConfiguration merges source configuration into target.
// Source values override target values.
func mergeConfiguration(target, source *Configuration) {
	if source.Mtlog.MinimumLevel != "" {
		target.Mtlog.MinimumLevel = source.Mtlog.MinimumLevel
	}

	// Merge sinks (replace entirely if specified)
	if len(source.Mtlog.WriteTo) > 0 {
		target.Mtlog.WriteTo = source.Mtlog.WriteTo
	}

	// Merge enrichers (append)
	target.Mtlog.Enrich = append(target.Mtlog.Enrich, source.Mtlog.Enrich...)
	target.Mtlog.EnrichWith = append(target.Mtlog.EnrichWith, source.Mtlog.EnrichWith...)

	// Merge filters (append)
	target.Mtlog.Filter = append(target.Mtlog.Filter, source.Mtlog.Filter...)

	// Merge properties
	if target.Mtlog.Properties == nil {
		target.Mtlog.Properties = make(map[string]any)
	}
	maps.Copy(target.Mtlog.Properties, source.Mtlog.Properties)
}
