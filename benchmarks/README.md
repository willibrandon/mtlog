# mtlog Benchmarks

This is a separate Go module containing performance benchmarks that compare mtlog with other popular Go logging libraries (zap, zerolog).

## Why a Separate Module?

The benchmarks are in a separate module to avoid forcing mtlog users to download unnecessary dependencies (zap, zerolog) that are only used for performance comparisons.

## Running Benchmarks

```bash
cd benchmarks
go test -bench=. -benchmem
```

## Benchmark Categories

- **Simple String**: Basic string logging without allocations
- **With Properties**: Logging with structured fields
- **Template Parsing**: Message template with multiple parameters
- **Complex Object**: Logging with object destructuring
- **Filtered Out**: Performance when log level filters out the message
- **Console Output**: Performance with formatted console output

## Results

See the main repository README for current benchmark results.