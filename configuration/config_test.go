package configuration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/willibrandon/mtlog/core"
)

func TestLoadFromJSON(t *testing.T) {
	jsonData := `{
		"Mtlog": {
			"MinimumLevel": "Debug",
			"WriteTo": [
				{
					"Name": "Console",
					"Args": {
						"theme": "Dev"
					}
				},
				{
					"Name": "File",
					"Args": {
						"path": "logs/test.log"
					}
				}
			],
			"Enrich": ["WithMachineName", "WithThreadId"],
			"Properties": {
				"Application": "TestApp",
				"Version": "1.0.0"
			}
		}
	}`

	config, err := LoadFromJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("Failed to load JSON: %v", err)
	}

	// Check minimum level
	if config.Mtlog.MinimumLevel != "Debug" {
		t.Errorf("Expected minimum level Debug, got %s", config.Mtlog.MinimumLevel)
	}

	// Check sinks
	if len(config.Mtlog.WriteTo) != 2 {
		t.Errorf("Expected 2 sinks, got %d", len(config.Mtlog.WriteTo))
	}

	// Check first sink
	if config.Mtlog.WriteTo[0].Name != "Console" {
		t.Errorf("Expected first sink to be Console, got %s", config.Mtlog.WriteTo[0].Name)
	}

	// Check enrichers
	if len(config.Mtlog.Enrich) != 2 {
		t.Errorf("Expected 2 enrichers, got %d", len(config.Mtlog.Enrich))
	}

	// Check properties
	if config.Mtlog.Properties["Application"] != "TestApp" {
		t.Errorf("Expected Application property to be TestApp")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected core.LogEventLevel
		wantErr  bool
	}{
		{"verbose", core.VerboseLevel, false},
		{"VRB", core.VerboseLevel, false},
		{"Debug", core.DebugLevel, false},
		{"dbg", core.DebugLevel, false},
		{"Information", core.InformationLevel, false},
		{"info", core.InformationLevel, false},
		{"INF", core.InformationLevel, false},
		{"Warning", core.WarningLevel, false},
		{"warn", core.WarningLevel, false},
		{"WRN", core.WarningLevel, false},
		{"Error", core.ErrorLevel, false},
		{"err", core.ErrorLevel, false},
		{"Fatal", core.FatalLevel, false},
		{"FTL", core.FatalLevel, false},
		{"unknown", core.InformationLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLevel(%s) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if level != tt.expected {
				t.Errorf("ParseLevel(%s) = %v, want %v", tt.input, level, tt.expected)
			}
		})
	}
}

func TestGetHelpers(t *testing.T) {
	args := map[string]any{
		"stringVal":  "hello",
		"intVal":     42,
		"floatVal":   42.5,
		"boolVal":    true,
		"stringBool": "true",
		"stringInt":  "123",
	}

	// Test GetString
	if v := GetString(args, "stringVal", "default"); v != "hello" {
		t.Errorf("GetString failed, got %s", v)
	}
	if v := GetString(args, "missing", "default"); v != "default" {
		t.Errorf("GetString default failed, got %s", v)
	}

	// Test GetInt
	if v := GetInt(args, "intVal", 0); v != 42 {
		t.Errorf("GetInt failed, got %d", v)
	}
	if v := GetInt(args, "floatVal", 0); v != 42 {
		t.Errorf("GetInt from float failed, got %d", v)
	}
	if v := GetInt(args, "stringInt", 0); v != 123 {
		t.Errorf("GetInt from string failed, got %d", v)
	}

	// Test GetBool
	if v := GetBool(args, "boolVal", false); v != true {
		t.Errorf("GetBool failed")
	}
	if v := GetBool(args, "stringBool", false); v != true {
		t.Errorf("GetBool from string failed")
	}
}

func TestBuilderIntegration(t *testing.T) {
	jsonData := `{
		"Mtlog": {
			"MinimumLevel": "Debug",
			"WriteTo": [
				{
					"Name": "Console",
					"Args": {
						"theme": "NoColor",
						"showProperties": true
					}
				}
			],
			"Enrich": ["WithThreadId"],
			"Properties": {
				"TestRun": true
			}
		}
	}`

	logger, err := CreateLoggerFromJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Logger should be created successfully
	if logger == nil {
		t.Fatal("Logger is nil")
	}

	// Test logging
	logger.Debug("Test debug message")
	logger.Information("Test info message")
}

func TestComplexConfiguration(t *testing.T) {
	jsonData := `{
		"Mtlog": {
			"MinimumLevel": "Information",
			"WriteTo": [
				{
					"Name": "RollingFile",
					"Args": {
						"path": "logs/app.log",
						"fileSizeLimitBytes": 10485760,
						"rollingInterval": "Daily",
						"retainedFileCount": 7,
						"compress": true
					}
				},
				{
					"Name": "Async",
					"Args": {
						"bufferSize": 1000,
						"overflowStrategy": "Drop",
						"writeTo": {
							"Name": "Console",
							"Args": {
								"theme": "Dev"
							}
						}
					}
				}
			],
			"EnrichWith": [
				{
					"Name": "WithEnvironmentName",
					"Args": {
						"environmentName": "Production"
					}
				}
			],
			"Filter": [
				{
					"Name": "ByLevel",
					"Args": {
						"minimumLevel": "Warning"
					}
				}
			],
			"Properties": {
				"Application": "MyApp",
				"Environment": "Production"
			}
		}
	}`

	config, err := LoadFromJSON([]byte(jsonData))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check rolling file sink
	if len(config.Mtlog.WriteTo) != 2 {
		t.Errorf("Expected 2 sinks, got %d", len(config.Mtlog.WriteTo))
	}

	rollingFile := config.Mtlog.WriteTo[0]
	if rollingFile.Name != "RollingFile" {
		t.Errorf("Expected RollingFile sink, got %s", rollingFile.Name)
	}

	// Check file size limit
	sizeLimit := GetInt64(rollingFile.Args, "fileSizeLimitBytes", 0)
	if sizeLimit != 10485760 {
		t.Errorf("Expected file size limit 10485760, got %d", sizeLimit)
	}

	// Check async sink
	asyncSink := config.Mtlog.WriteTo[1]
	if asyncSink.Name != "Async" {
		t.Errorf("Expected Async sink, got %s", asyncSink.Name)
	}

	// Check nested writeTo
	writeTo, ok := asyncSink.Args["writeTo"].(map[string]any)
	if !ok {
		t.Error("Expected writeTo in async sink args")
	} else {
		sinkName, _ := writeTo["Name"].(string)
		if sinkName != "Console" {
			t.Errorf("Expected Console in writeTo, got %s", sinkName)
		}
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	configData := `{
		"Mtlog": {
			"MinimumLevel": "Information",
			"WriteTo": [
				{
					"Name": "File",
					"Args": {
						"path": "test.log"
					}
				}
			]
		}
	}`

	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load configuration
	config, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config file: %v", err)
	}

	if config.Mtlog.MinimumLevel != "Information" {
		t.Errorf("Expected minimum level Information, got %s", config.Mtlog.MinimumLevel)
	}
}

func TestEnvironmentConfiguration(t *testing.T) {
	// Create temporary directory with config files
	tempDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()

	// Base configuration
	baseConfig := `{
		"Mtlog": {
			"MinimumLevel": "Information",
			"WriteTo": [
				{
					"Name": "Console"
				}
			],
			"Properties": {
				"Application": "MyApp",
				"Version": "1.0.0"
			}
		}
	}`

	// Development configuration
	devConfig := `{
		"Mtlog": {
			"MinimumLevel": "Debug",
			"WriteTo": [
				{
					"Name": "Console",
					"Args": {
						"theme": "Dev",
						"showProperties": true
					}
				}
			],
			"Properties": {
				"Environment": "Development"
			}
		}
	}`

	// Write config files
	if err := os.WriteFile("appsettings.json", []byte(baseConfig), 0644); err != nil {
		t.Fatalf("Failed to write base config: %v", err)
	}
	if err := os.WriteFile("appsettings.Development.json", []byte(devConfig), 0644); err != nil {
		t.Fatalf("Failed to write dev config: %v", err)
	}

	// Create logger for development environment
	logger, err := CreateLoggerFromEnvironment("Development")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	if logger == nil {
		t.Fatal("Logger is nil")
	}

	// The logger should have debug level from dev config
	// and properties from both configs
}
