package main

import (
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func main() {
	// Create a logger with Serilog-style configuration similar to:
	// {
	//   "MinimumLevel": {
	//     "Default": "Information",
	//     "Override": {
	//       "Microsoft": "Warning",
	//       "Microsoft.Hosting.Lifetime": "Information",
	//       "Microsoft.EntityFrameworkCore": "Warning"
	//     }
	//   },
	//   "WriteTo": [
	//     {
	//       "Name": "Console",
	//       "outputTemplate": "[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message:lj}${NewLine}${Exception}"
	//     },
	//     {
	//       "Name": "File",
	//       "path": "logs/monitor-.log",
	//       "outputTemplate": "[${Timestamp:yyyy-MM-dd HH:mm:ss.fff zzz} ${Level:u3}] {SourceContext}: ${Message:lj}${NewLine}${Exception}"
	//     }
	//   ],
	//   "Enrich": [ "FromLogContext", "WithMachineName", "WithThreadId" ]
	// }

	// Create console sink with template and theme
	consoleSink, err := sinks.NewConsoleSinkWithTemplateAndTheme(
		"[${Timestamp:HH:mm:ss} ${Level:u3}] {SourceContext}: ${Message:lj}",
		sinks.LiterateTheme(), // Serilog Literate theme with ANSI 256 colors
	)
	if err != nil {
		panic(err)
	}
	
	logger := mtlog.New(
		// Serilog-style output templates
		mtlog.WithSink(consoleSink),
		mtlog.WithFileTemplate("logs/monitor-.log", "[${Timestamp:yyyy-MM-dd HH:mm:ss.fff zzz} ${Level:u3}] {SourceContext}: ${Message:lj}"),

		// Enrichers equivalent to Serilog's "Enrich"
		mtlog.WithMachineName(),
		mtlog.WithThreadId(),
		mtlog.WithAutoSourceContext(), // Automatically detect source context from caller

		// Minimum level overrides (equivalent to Serilog's "MinimumLevel.Override")
		mtlog.WithMinimumLevelOverrides(core.InformationLevel, map[string]core.LogEventLevel{
			"Microsoft":                     core.WarningLevel,
			"Microsoft.Hosting.Lifetime":    core.InformationLevel,
			"Microsoft.EntityFrameworkCore": core.WarningLevel,
		}),
	)
	defer logger.Close()

	// Log some example messages - source context will be auto-detected
	logger.Information("Application starting up")
	logger.Information("Database connection established")
	logger.Warning("Configuration value 'MaxRetries' not found, using default: 3")
	logger.Error("Failed to connect to external service")

	// Demonstrate logging with properties
	logger.Information("Processing batch {BatchId} with {ItemCount} items", "batch-2024-001", 150)
	logger.Information("Request completed in {Duration:F2}ms with status {StatusCode}", 125.67, 200)

	// Demonstrate structured data
	logger.Information("Performance metrics: {RequestsPerSecond:F1} req/s, {MemoryUsage:F1}MB, {CpuUsage:P0}", 125.3, 45.2, 0.65)
}
