# Configuration

mtlog supports JSON-based configuration following patterns similar to Serilog's appsettings.json approach.

## Basic Configuration

```json
{
  "Mtlog": {
    "MinimumLevel": "Information",
    "WriteTo": [
      {
        "Name": "Console",
        "Args": {
          "theme": "Default"
        }
      }
    ]
  }
}
```

## Loading Configuration

### From File

```go
import "github.com/willibrandon/mtlog/configuration"

logger, err := configuration.CreateLoggerFromFile("appsettings.json")
```

### From JSON Bytes

```go
jsonData := []byte(`{"Mtlog": {...}}`)
logger, err := configuration.CreateLoggerFromJSON(jsonData)
```

### Environment-Based Configuration

Supports environment-specific overrides:

```go
// Loads appsettings.json and appsettings.Development.json
logger, err := configuration.CreateLoggerFromEnvironment("Development")
```

## Configuration Options

### Minimum Level

Controls the minimum level of events to log:

```json
{
  "Mtlog": {
    "MinimumLevel": "Debug"
  }
}
```

Supported levels: `Verbose`, `Debug`, `Information`, `Warning`, `Error`, `Fatal`

### Sinks

Configure output destinations:

```json
{
  "Mtlog": {
    "WriteTo": [
      {
        "Name": "Console",
        "Args": {
          "theme": "Dev",
          "showProperties": true
        }
      },
      {
        "Name": "File",
        "Args": {
          "path": "logs/app.log"
        }
      }
    ]
  }
}
```

#### Available Sinks

**Console**
- `theme`: "Default", "Lite", "Dev", "NoColor"
- `showProperties`: true/false

**File**
- `path`: File path (required)

**RollingFile**
- `path`: File path (required)
- `fileSizeLimitBytes`: Maximum file size
- `rollingInterval`: "Hourly", "Daily", "Weekly", "Monthly"
- `retainedFileCount`: Number of files to keep
- `compress`: true/false
- `bufferSize`: Buffer size in bytes

**Seq**
- `serverUrl`: Seq server URL (required)
- `apiKey`: API key for authentication
- `batchSize`: Number of events per batch
- `period`: Flush interval (e.g., "5s")
- `compress`: true/false
- `maxRetries`: Retry count
- `retryBackoff`: Retry delay (e.g., "1s")

**Async** (wrapper for other sinks)
- `bufferSize`: Event buffer size
- `overflowStrategy`: "Block", "Drop", "DropOldest"
- `batchSize`: Batch size for flushing
- `flushInterval`: Flush interval (e.g., "100ms")
- `writeTo`: Nested sink configuration

**Elasticsearch**
- `url`: Elasticsearch URL (required)
- `index`: Index name prefix (default: "logs")
- `apiKey`: API key for authentication
- `username`: Username for basic auth
- `password`: Password for basic auth
- `batchSize`: Number of events per batch
- `batchTimeout`: Batch flush timeout (e.g., "5s")
- `dataStreams`: Use data streams instead of indices (true/false)
- `pipeline`: Ingest pipeline name

**Conditional** (filters events based on predicates)
- `name`: Optional name for debugging
- `when`: Predicate type ("level", "property", "propertyValue")
- `minimumLevel`: Minimum level when using "level" predicate
- `property`: Property name for "property" or "propertyValue" predicates
- `value`: Expected value for "propertyValue" predicate
- `writeTo`: Nested sink configuration for matching events

**Router** (routes events to multiple sinks)
- `mode`: "FirstMatch" (exclusive) or "AllMatch" (broadcast)
- `routes`: Array of route configurations
  - `name`: Route name
  - `when`: Predicate type ("level", "property", "propertyValue", "error", "audit", "metric")
  - `minimumLevel`: Minimum level for "level" predicate
  - `property`: Property name for property-based predicates
  - `value`: Expected value for "propertyValue" predicate
  - `writeTo`: Nested sink configuration
- `defaultSink`: Optional sink for non-matching events

### Enrichers

Add contextual properties to all events:

```json
{
  "Mtlog": {
    "Enrich": ["WithMachineName", "WithThreadId", "WithProcessId"],
    "EnrichWith": [
      {
        "Name": "WithEnvironmentName",
        "Args": {
          "environmentName": "Production"
        }
      }
    ]
  }
}
```

### Filters

Control which events are logged:

```json
{
  "Mtlog": {
    "Filter": [
      {
        "Name": "ByLevel",
        "Args": {
          "minimumLevel": "Warning"
        }
      }
    ]
  }
}
```

### Properties

Add global properties to all events:

```json
{
  "Mtlog": {
    "Properties": {
      "Application": "MyApp",
      "Version": "1.0.0",
      "Environment": "Production"
    }
  }
}
```

## Complete Example

```json
{
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
          "compress": true
        }
      },
      {
        "Name": "Async",
        "Args": {
          "bufferSize": 1000,
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
      "Application": "MyApp",
      "Team": "Platform",
      "DeploymentId": "deploy-2025-001"
    }
  }
}
```

## Environment-Based Configuration

Use separate files for different environments:

**appsettings.json** (base configuration):
```json
{
  "Mtlog": {
    "MinimumLevel": "Information",
    "WriteTo": [
      {
        "Name": "Console"
      }
    ],
    "Properties": {
      "Application": "MyApp"
    }
  }
}
```

**appsettings.Development.json** (development overrides):
```json
{
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
}
```

**appsettings.Production.json** (production overrides):
```json
{
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
}
```

## Extending Configuration

You can register custom sinks, enrichers, and filters:

```go
builder := configuration.NewLoggerBuilder()

// Register custom sink
builder.RegisterSink("MySink", func(args map[string]interface{}) (core.LogEventSink, error) {
    return NewMySink(args), nil
})

// Register custom enricher
builder.RegisterEnricher("WithCustomProperty", func(args map[string]interface{}) (core.LogEventEnricher, error) {
    return NewCustomEnricher(args), nil
})

// Build logger with custom components
logger, err := builder.Build(config)
```