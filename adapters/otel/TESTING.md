# OpenTelemetry Adapter Testing Documentation

## Testing Infrastructure

This document describes the comprehensive testing infrastructure for the mtlog OTEL adapter module.

### Test Categories

#### 1. Integration Tests (`integration/otlp_integration_test.go`)
- **Purpose**: Test real OTEL collector integration
- **Build Tag**: `//go:build integration`
- **Requirements**: Docker with OTEL collector and Jaeger running
- **Coverage**:
  - Both gRPC and HTTP transports
  - All enricher types (Fast, Static, Caching)
  - Health check functionality
  - Metrics validation

**Running Integration Tests:**
```bash
# Start Docker containers
cd integration
docker-compose up -d

# Run tests
go test -tags=integration ./integration/...

# Stop containers
docker-compose down
```

#### 2. Chaos Tests (`chaos_test.go`)
- **Purpose**: Test edge cases in batch processing
- **Coverage**:
  - Exact batch size scenarios
  - Queue overflow conditions
  - Timer race conditions
  - Concurrent batch modifications
  - Close-while-processing scenarios
  
**Test Scenarios:**
- `ExactBatchSize`: Tests when events exactly fill a batch
- `OneLessThanBatch`: Tests timer-based flushing
- `OneMoreThanBatch`: Tests batch overflow handling
- `MultipleBatches`: Tests sequential batch processing
- `TrickleWithTimeout`: Tests timeout-based flushing
- `RandomPattern`: Tests unpredictable event patterns
- `QueueOverflow`: Tests dropping behavior when queue is full

#### 3. Concurrency Tests (`concurrency_test.go`)
- **Purpose**: Catch race conditions under high load
- **Coverage**:
  - High concurrent enricher usage (100 goroutines × 1000 events)
  - Concurrent sink operations (writers, flushers, readers)
  - Race condition regression tests
  - Performance under concurrent load

**Race Detection:**
```bash
go test -race -run TestHighConcurrencyEnricher
go test -race -run TestConcurrentSinkOperations
go test -race -run TestRaceConditionRegression
```

#### 4. Fuzz Tests (`fuzz_test.go`)
- **Purpose**: Find edge cases through randomized testing
- **Build Tag**: `//go:build go1.18`
- **Coverage**:
  - Bridge message template parsing
  - Handler property conversion
  - Enricher context handling
  - OTLP sink configuration

**Running Fuzz Tests:**
```bash
# Run specific fuzz test
go test -fuzz=FuzzBridgeMessageTemplate -fuzztime=30s

# Run all fuzz tests
go test -fuzz=Fuzz -fuzztime=1m ./...
```

### Performance Benchmarks

The module includes comprehensive benchmarks:

```bash
# Run all benchmarks
go test -bench=. -benchmem ./...

# Run specific benchmark
go test -bench=BenchmarkFastOTELEnricher_NoSpan -benchtime=10s

# Run concurrent benchmarks
go test -bench=BenchmarkConcurrent -cpu=1,2,4,8
```

**Key Benchmark Results:**
- `FastOTELEnricher_NoSpan`: ~8.6ns/op, 0 allocs
- `StaticOTELEnricher_NoSpan`: ~3.3ns/op, 0 allocs
- `FastOTELEnricher_WithSpan`: ~204ns/op, 10 allocs
- `StaticOTELEnricher_WithSpan`: ~111ns/op, 6 allocs

### Race Conditions Fixed

Through testing, the following race conditions were identified and fixed:

1. **Timer Race Condition** (sink.go)
   - Issue: Timer could be set to nil while AfterFunc was executing
   - Fix: Added separate `timerMu` mutex for timer operations

2. **Logger Access Race** (sink.go)
   - Issue: `s.logger` was being reassigned during Flush while being read
   - Fix: Added `loggerMu` RWMutex to protect logger access
   - Changed Flush to use ForceFlush instead of recreating exporter

3. **Cached Flag Race** (enricher.go)
   - Issue: Bool flag had concurrent read/write without synchronization
   - Fix: Replaced `bool` with `atomic.Bool`

### Docker Compose Setup

The `integration/docker-compose.yml` provides:
- **OTEL Collector**: Receives and processes telemetry data
- **Jaeger**: Trace visualization and storage
- **Configuration**: Full OTLP pipeline with debug output

### Test Coverage

Run coverage analysis:
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### CI/CD Integration

The tests are designed to work in CI environments:
- Unit tests run without external dependencies
- Integration tests skip when services unavailable
- Chaos and concurrency tests use mock endpoints
- Fuzz tests can run with time limits

### Best Practices

1. **Always run with race detector** during development
2. **Use integration tests** for real service validation
3. **Run chaos tests** before releases to catch edge cases
4. **Fuzz new features** to find unexpected inputs
5. **Benchmark after changes** to ensure performance

### Known Limitations

1. **ForceFlush timeout**: Can hang if collector is unreachable
   - Mitigation: Use short timeouts in tests
   
2. **gRPC connection attempts**: May retry extensively
   - Mitigation: Use invalid endpoints for unit tests

3. **Resource cleanup**: Some tests may leak goroutines
   - Mitigation: Proper Close() calls and timeouts

### Future Improvements

- [ ] Add load testing with realistic traffic patterns
- [ ] Implement mutation testing for better coverage
- [ ] Add performance regression detection
- [ ] Create test fixtures for common scenarios
- [ ] Add distributed tracing validation tests