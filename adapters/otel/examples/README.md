# OpenTelemetry Integration Example

This example demonstrates mtlog's OpenTelemetry integration with trace correlation and OTLP export.

## Quick Start

### Option 1: Run with Full Observability Stack (Recommended)

This starts OpenTelemetry Collector and Jaeger for full observability:

```bash
./run-with-collector.sh
```

Then you can:
- View traces in Jaeger UI: http://localhost:16686
- View collector metrics: http://localhost:8888/metrics
- View collector zpages: http://localhost:55679/debug/tracez

To stop all services:
```bash
docker-compose down
```

### Option 2: Simple Collector (Console Output Only)

Run just the collector with debug output to console:

```bash
# Terminal 1: Start collector
./run-collector-simple.sh

# Terminal 2: Run example
go run -tags=otel main.go
```

### Option 3: Run Example Without Collector

If you just want to see the example output (will show connection errors):

```bash
go run -tags=otel main.go
```

## What This Example Demonstrates

1. **OTEL Enrichment**: Automatically adds trace.id and span.id to logs
2. **OTLP Export**: Sends logs to OpenTelemetry Collector via gRPC/HTTP
3. **Distributed Tracing**: Shows how logs correlate with traces
4. **Performance**: Benchmarks showing <5ns overhead when no span present

## Architecture

```
┌─────────────┐      OTLP/gRPC     ┌──────────────┐      ┌─────────┐
│   mtlog     │───────:4317────────▶│     OTEL     │─────▶│ Jaeger  │
│ Application │                     │  Collector   │      └─────────┘
│             │───────:4318────────▶│              │      ┌─────────┐
└─────────────┘     OTLP/HTTP      │              │─────▶│  File   │
                                    └──────────────┘      └─────────┘
                                           │               ┌─────────┐
                                           └──────────────▶│ Console │
                                                           └─────────┘
```

## Viewing Exported Logs

When using docker-compose, logs are exported to a file in the collector container:

```bash
# View formatted logs
docker exec otel-collector cat /tmp/otel-logs.json | jq .

# Follow logs in real-time
docker-compose logs -f otel-collector
```

## Configuration

The example shows multiple ways to configure OTLP export:

1. **Environment Variables** (production recommended):
   ```bash
   export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
   logger := mtlog.New(mtlog.WithOTLP())
   ```

2. **Explicit gRPC**:
   ```go
   logger := mtlog.New(mtlog.WithOTLPGRPC("localhost:4317"))
   ```

3. **Explicit HTTP**:
   ```go
   logger := mtlog.New(mtlog.WithOTLPHTTP("http://localhost:4318/v1/logs"))
   ```

4. **Custom Configuration**:
   ```go
   config := mtlog.NewOTLPConfig().
       WithEndpoint("localhost:4317").
       WithGRPC().
       WithCompression("gzip").
       WithBatching(100, 5*time.Second)
   
   logger := mtlog.New(config.Build())
   ```

## Troubleshooting

### Connection Refused Errors
If you see "connection refused" errors, make sure the collector is running:
```bash
docker ps | grep otel-collector
```

### No Traces in Jaeger
Traces may take a few seconds to appear. Check:
1. Collector logs: `docker-compose logs otel-collector`
2. Jaeger UI: http://localhost:16686
3. Select "mtlog-otel-example" service in Jaeger

### Resource Schema Conflicts
The example uses `resource.NewWithAttributes("")` to avoid schema conflicts between different OTEL components.

## Performance

The example includes benchmarks showing:
- **StaticOTELEnricher**: ~3.3ns overhead (best for request-scoped loggers)
- **FastOTELEnricher**: ~8.6ns overhead (default, good balance)
- **Zero allocations** when no span is present