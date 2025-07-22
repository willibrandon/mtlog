# Go Container-Based Testing

## Container-Based Testing Strategy

### Test Infrastructure Setup

```yaml
# docker-compose.test.yml
version: '3.8'

services:
  seq:
    image: datalust/seq:latest
    environment:
      ACCEPT_EULA: Y
    ports:
      - "5341:5341"
      - "8080:80"
    volumes:
      - seq-data:/data
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:5341/api/events/signal"]
      interval: 5s
      timeout: 10s
      retries: 5

  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
      - "ES_JAVA_OPTS=-Xms512m -Xmx512m"
    ports:
      - "9200:9200"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9200/_cluster/health"]
      interval: 5s
      timeout: 10s
      retries: 5

  test-runner:
    build:
      context: .
      dockerfile: Dockerfile.test
    depends_on:
      seq:
        condition: service_healthy
      elasticsearch:
        condition: service_healthy
    environment:
      SEQ_URL: http://seq:5341
      ELASTICSEARCH_URL: http://elasticsearch:9200
    volumes:
      - .:/app
      - go-cache:/go/pkg/mod
    command: go test -v ./...

volumes:
  seq-data:
  go-cache:
```

### Real Integration Tests (No Mocks!)

```go
// seq_integration_test.go
package structlog_test

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "testing"
    "time"
)

func TestSeqIntegration(t *testing.T) {
    seqUrl := os.Getenv("SEQ_URL")
    if seqUrl == "" {
        t.Skip("SEQ_URL not set, skipping integration test")
    }
    
    // Create logger with Seq sink
    log := structlog.New().
        MinimumLevel().Debug().
        WriteTo().Seq(seqUrl).
        CreateLogger()
    
    // Log test events
    testId := fmt.Sprintf("test-%d", time.Now().Unix())
    log.Information("Test event {TestId}", testId)
    log.Warning("Test warning {TestId} {Count}", testId, 42)
    
    // Wait for batch to flush
    time.Sleep(3 * time.Second)
    
    // Query Seq API to verify events
    resp, err := http.Get(fmt.Sprintf("%s/api/events/signal?filter=TestId='%s'&count=10", seqUrl, testId))
    if err != nil {
        t.Fatalf("Failed to query Seq: %v", err)
    }
    defer resp.Body.Close()
    
    var result struct {
        Events []json.RawMessage `json:"events"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        t.Fatalf("Failed to decode response: %v", err)
    }
    
    // Verify we got our events
    if len(result.Events) != 2 {
        t.Errorf("Expected 2 events, got %d", len(result.Events))
    }
    
    // Verify event structure
    for _, event := range result.Events {
        var evt map[string]interface{}
        json.Unmarshal(event, &evt)
        
        // Check message template is preserved
        if mt, ok := evt["@mt"]; !ok {
            t.Error("Missing @mt field")
        } else if !strings.Contains(mt.(string), "{TestId}") {
            t.Error("Message template not preserved")
        }
        
        // Check properties
        if props, ok := evt["TestId"]; !ok || props != testId {
            t.Error("TestId property missing or incorrect")
        }
    }
}
```

### Table-Driven Tests (Go Style)

```go
func TestMessageTemplateParser(t *testing.T) {
    tests := []struct {
        name     string
        template string
        args     []interface{}
        expected map[string]interface{}
    }{
        {
            name:     "simple property",
            template: "User {UserId} logged in",
            args:     []interface{}{123},
            expected: map[string]interface{}{"UserId": 123},
        },
        {
            name:     "multiple properties",
            template: "User {UserId} performed {Action} on {Resource}",
            args:     []interface{}{123, "DELETE", "post-456"},
            expected: map[string]interface{}{
                "UserId":   123,
                "Action":   "DELETE", 
                "Resource": "post-456",
            },
        },
        {
            name:     "escaped braces",
            template: "Processing {{batch}} with {Count} items",
            args:     []interface{}{50},
            expected: map[string]interface{}{"Count": 50},
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tmpl, err := ParseMessageTemplate(tt.template)
            if err != nil {
                t.Fatalf("Failed to parse template: %v", err)
            }
            
            props := tmpl.ExtractProperties(tt.args)
            
            for key, expected := range tt.expected {
                if actual, ok := props[key]; !ok {
                    t.Errorf("Missing property %s", key)
                } else if actual != expected {
                    t.Errorf("Property %s: expected %v, got %v", key, expected, actual)
                }
            }
        })
    }
}
```

### Performance Testing

```go
func BenchmarkSimpleLog(b *testing.B) {
    // Use a null sink for pure performance testing
    log := structlog.New().
        WriteTo().Null().
        CreateLogger()
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        log.Information("User logged in")
    }
}

func BenchmarkStructuredLog(b *testing.B) {
    log := structlog.New().
        WriteTo().Null().
        CreateLogger()
    
    user := struct {
        ID   int
        Name string
    }{123, "test"}
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        log.Information("User {User} performed {Action}", user, "login")
    }
}
```

### Test Helpers

```go
// test_helpers.go
package structlog_test

type MemorySink struct {
    Events []LogEvent
    mu     sync.Mutex
}

func (m *MemorySink) Emit(event *LogEvent) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Events = append(m.Events, *event)
}

func (m *MemorySink) Clear() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Events = m.Events[:0]
}

// Helper to wait for async operations
func Eventually(t *testing.T, condition func() bool, timeout time.Duration) {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if condition() {
            return
        }
        time.Sleep(10 * time.Millisecond)
    }
    t.Fatal("Condition not met within timeout")
}
```

## Week-by-Week Plan

### Week 1: Core + Learning Go
- **Days 1-2**: Message template parser (learn string manipulation in Go)
- **Days 3-4**: Basic logger and sinks (learn interfaces and methods)
- **Days 5-7**: File operations and basic enrichers (learn Go's io package)

### Week 2: Pipeline + Seq
- **Days 1-2**: Pipeline architecture (learn composition patterns)
- **Days 3-4**: Seq sink with batching (learn goroutines and channels)
- **Days 5-7**: Container-based testing setup

### Week 3: Polish + Performance
- **Days 1-2**: Performance optimization (learn profiling)
- **Days 3-4**: Elasticsearch sink
- **Days 5-7**: Documentation and examples

## Quick Go Testing Commands

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run only integration tests
go test -tags=integration ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run with race detector
go test -race ./...

# Run specific test
go test -run TestSeqIntegration ./...

# Run tests in container
docker-compose -f docker-compose.test.yml up --abort-on-container-exit
```

## Testing Philosophy for This Project

1. **Real Dependencies** - Use real Seq, real Elasticsearch via containers
2. **Memory Sinks** - For unit tests, use in-memory sinks that capture events
3. **Table-Driven** - Go's table-driven tests are perfect for template parsing
4. **Integration First** - Test the actual integration points, not mocks
5. **Benchmarks** - Track allocations from day one

This approach means your tests actually prove the system works with real infrastructure, which is much more valuable than mock-based tests.