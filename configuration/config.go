package configuration

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	
	"github.com/willibrandon/mtlog/core"
)

// LoggerConfiguration represents the JSON configuration for mtlog.
type LoggerConfiguration struct {
	MinimumLevel   string                   `json:"MinimumLevel,omitempty"`
	WriteTo        []SinkConfiguration      `json:"WriteTo,omitempty"`
	Enrich         []string                 `json:"Enrich,omitempty"`
	EnrichWith     []EnricherConfiguration  `json:"EnrichWith,omitempty"`
	Properties     map[string]interface{}   `json:"Properties,omitempty"`
	Filter         []FilterConfiguration    `json:"Filter,omitempty"`
}

// SinkConfiguration represents a sink configuration.
type SinkConfiguration struct {
	Name string                 `json:"Name"`
	Args map[string]interface{} `json:"Args,omitempty"`
}

// EnricherConfiguration represents an enricher with arguments.
type EnricherConfiguration struct {
	Name string                 `json:"Name"`
	Args map[string]interface{} `json:"Args,omitempty"`
}

// FilterConfiguration represents a filter configuration.
type FilterConfiguration struct {
	Name string                 `json:"Name"`
	Args map[string]interface{} `json:"Args,omitempty"`
}

// Configuration is the root configuration object.
type Configuration struct {
	Mtlog LoggerConfiguration `json:"Mtlog"`
}

// LoadFromFile loads configuration from a JSON file.
func LoadFromFile(filename string) (*Configuration, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	return LoadFromJSON(data)
}

// LoadFromJSON loads configuration from JSON data.
func LoadFromJSON(data []byte) (*Configuration, error) {
	var config Configuration
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	// Apply defaults
	if config.Mtlog.MinimumLevel == "" {
		config.Mtlog.MinimumLevel = "Information"
	}
	
	return &config, nil
}

// ParseLevel parses a log level string.
func ParseLevel(levelStr string) (core.LogEventLevel, error) {
	switch strings.ToLower(levelStr) {
	case "verbose", "vrb":
		return core.VerboseLevel, nil
	case "debug", "dbg":
		return core.DebugLevel, nil
	case "information", "info", "inf":
		return core.InformationLevel, nil
	case "warning", "warn", "wrn":
		return core.WarningLevel, nil
	case "error", "err":
		return core.ErrorLevel, nil
	case "fatal", "ftl":
		return core.FatalLevel, nil
	default:
		return core.InformationLevel, fmt.Errorf("unknown log level: %s", levelStr)
	}
}

// GetString gets a string value from configuration args.
func GetString(args map[string]interface{}, key string, defaultValue string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

// GetInt gets an int value from configuration args.
func GetInt(args map[string]interface{}, key string, defaultValue int) int {
	if v, ok := args[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		case string:
			// Try to parse string as int
			var i int
			if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
				return i
			}
		}
	}
	return defaultValue
}

// GetInt64 gets an int64 value from configuration args.
func GetInt64(args map[string]interface{}, key string, defaultValue int64) int64 {
	if v, ok := args[key]; ok {
		switch val := v.(type) {
		case float64:
			return int64(val)
		case int64:
			return val
		case int:
			return int64(val)
		case string:
			// Try to parse string as int64
			var i int64
			if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
				return i
			}
		}
	}
	return defaultValue
}

// GetBool gets a bool value from configuration args.
func GetBool(args map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := args[key]; ok {
		switch val := v.(type) {
		case bool:
			return val
		case string:
			return strings.ToLower(val) == "true"
		}
	}
	return defaultValue
}

// GetDuration gets a duration value from configuration args.
// Supports formats like "100ms", "5s", "1m", etc.
func GetDuration(args map[string]interface{}, key string, defaultValue string) string {
	return GetString(args, key, defaultValue)
}