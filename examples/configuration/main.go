package main

import (
	"fmt"
	"os"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/configuration"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	fmt.Println("=== JSON Configuration Examples ===")

	// Example 1: Basic configuration
	fmt.Println("1. Basic Configuration")
	demoBasicConfig()

	// Example 2: Complex configuration
	fmt.Println("\n2. Complex Configuration")
	demoComplexConfig()

	// Example 3: Environment-based configuration
	fmt.Println("\n3. Environment-based Configuration")
	demoEnvironmentConfig()

	// Example 4: Programmatic configuration with JSON
	fmt.Println("\n4. Inline JSON Configuration")
	demoInlineConfig()
}

func demoBasicConfig() {
	// Create a basic configuration file
	configJSON := `{
		"Mtlog": {
			"MinimumLevel": "Information",
			"WriteTo": [
				{
					"Name": "Console",
					"Args": {
						"theme": "Default"
					}
				},
				{
					"Name": "File",
					"Args": {
						"path": "logs/app.log"
					}
				}
			],
			"Enrich": ["WithMachineName", "WithThreadId"],
			"Properties": {
				"Application": "ConfigExample",
				"Version": "1.0.0"
			}
		}
	}`

	// Save to file
	os.WriteFile("basic-config.json", []byte(configJSON), 0644)
	defer os.Remove("basic-config.json")

	// Create logger from file
	logger, err := configuration.CreateLoggerFromFile("basic-config.json")
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}

	// Use the logger
	logger.Information("Application started with basic configuration")
	logger.Information("Processing request {RequestId}", "req-001")
	logger.Warning("Cache miss for key {Key}", "user:123")
}

func demoComplexConfig() {
	// Create a complex configuration with multiple sinks and features
	configJSON := `{
		"Mtlog": {
			"MinimumLevel": "Debug",
			"WriteTo": [
				{
					"Name": "Console",
					"Args": {
						"theme": "Dev",
						"showProperties": true
					}
				},
				{
					"Name": "RollingFile",
					"Args": {
						"path": "logs/app.log",
						"rollingInterval": "Daily",
						"fileSizeLimitBytes": 10485760,
						"retainedFileCount": 7,
						"compress": true,
						"bufferSize": 65536
					}
				},
				{
					"Name": "Async",
					"Args": {
						"bufferSize": 1000,
						"batchSize": 100,
						"flushInterval": "100ms",
						"overflowStrategy": "Drop",
						"writeTo": {
							"Name": "Seq",
							"Args": {
								"serverUrl": "http://localhost:5341",
								"batchSize": 50,
								"period": "5s"
							}
						}
					}
				}
			],
			"Enrich": ["WithMachineName", "WithProcessId", "WithThreadId"],
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
						"minimumLevel": "Information"
					}
				}
			],
			"Properties": {
				"Application": "ComplexApp",
				"Team": "Platform",
				"DeploymentId": "deploy-2025-001"
			}
		}
	}`

	// Create logger from JSON
	logger, err := configuration.CreateLoggerFromJSON([]byte(configJSON))
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}

	// Demonstrate various log levels
	logger.Debug("This debug message will be filtered out")
	logger.Information("Complex configuration loaded successfully")
	logger.Information("Processing batch {BatchId} with {ItemCount} items", "batch-001", 150)

	// Log with structured data
	order := struct {
		ID     string
		Total  float64
		Status string
	}{
		ID:     "ORD-2025-001",
		Total:  299.99,
		Status: "Processing",
	}
	logger.Information("Order received: {@Order}", order)

	// Simulate some warnings and errors
	logger.Warning("High memory usage: {UsagePercent:P}", 0.85)
	logger.Error("Failed to connect to service {Service}: {Error}", "PaymentGateway", "Connection timeout")
}

func demoEnvironmentConfig() {
	// Create base configuration
	baseConfig := `{
		"Mtlog": {
			"MinimumLevel": "Information",
			"WriteTo": [
				{
					"Name": "Console"
				}
			],
			"Enrich": ["WithMachineName"],
			"Properties": {
				"Application": "MyApp",
				"Version": "2.0.0"
			}
		}
	}`

	// Create development override
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
				"Environment": "Development",
				"DebugMode": true
			}
		}
	}`

	// Create production override
	prodConfig := `{
		"Mtlog": {
			"MinimumLevel": "Warning",
			"WriteTo": [
				{
					"Name": "RollingFile",
					"Args": {
						"path": "logs/prod.log",
						"rollingInterval": "Daily",
						"retainedFileCount": 30
					}
				},
				{
					"Name": "Seq",
					"Args": {
						"serverUrl": "https://seq.production.example.com",
						"apiKey": "your-api-key-here"
					}
				}
			],
			"Properties": {
				"Environment": "Production",
				"DataCenter": "US-East"
			}
		}
	}`

	// Save configuration files
	os.WriteFile("appsettings.json", []byte(baseConfig), 0644)
	os.WriteFile("appsettings.Development.json", []byte(devConfig), 0644)
	os.WriteFile("appsettings.Production.json", []byte(prodConfig), 0644)

	defer os.Remove("appsettings.json")
	defer os.Remove("appsettings.Development.json")
	defer os.Remove("appsettings.Production.json")

	// Get environment from env var or default to Development
	environment := os.Getenv("ASPNETCORE_ENVIRONMENT")
	if environment == "" {
		environment = "Development"
	}

	fmt.Printf("Loading configuration for environment: %s\n", environment)

	// Create logger for current environment
	logger, err := configuration.CreateLoggerFromEnvironment(environment)
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}

	// Log based on environment
	logger.Debug("Debug message - only visible in Development")
	logger.Information("Application started in {Environment} mode", environment)
	logger.Warning("This warning appears in all environments")
	logger.Error("This error is logged everywhere")
}

func demoInlineConfig() {
	// Create a logger configuration inline for quick setup
	logger, err := configuration.CreateLoggerFromJSON([]byte(`{
		"Mtlog": {
			"MinimumLevel": "Verbose",
			"WriteTo": [
				{
					"Name": "Console",
					"Args": {
						"theme": "Lite"
					}
				}
			],
			"Properties": {
				"Component": "InlineDemo"
			}
		}
	}`))

	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return
	}

	// Demonstrate all log levels
	logger.Verbose("Verbose: Detailed trace information")
	logger.Debug("Debug: Internal system events")
	logger.Information("Information: Normal application flow")
	logger.Warning("Warning: Abnormal or unexpected events")
	logger.Error("Error: Errors and exceptions")
	logger.Fatal("Fatal: Critical errors causing shutdown")

	// Performance comparison
	iterations := 1000

	// Time configuration-based logger
	start := time.Now()
	for i := range iterations {
		logger.Information("Message {Number}", i)
	}
	configTime := time.Since(start)

	// Time programmatic logger for comparison
	programmaticLogger := mtlog.New(
		mtlog.WithSink(sinks.NewConsoleSinkWithTheme(sinks.LiteTheme())),
		mtlog.WithProperty("Component", "Programmatic"),
	)

	start = time.Now()
	for i := range iterations {
		programmaticLogger.Information("Message {Number}", i)
	}
	programmaticTime := time.Since(start)

	fmt.Printf("\nPerformance comparison (%d iterations):\n", iterations)
	fmt.Printf("  Configuration-based: %v\n", configTime)
	fmt.Printf("  Programmatic: %v\n", programmaticTime)
	fmt.Printf("  Difference: %v\n", configTime-programmaticTime)
}
