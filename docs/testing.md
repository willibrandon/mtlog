# Go Container-Based Testing

## Container-Based Testing Strategy

### Running Integration Tests

Integration tests run against real services using Docker containers.

#### Using Docker Compose

```bash
# Start all test services
docker-compose -f docker/docker-compose.test.yml up -d

# Run integration tests
go test -tags=integration ./...

# Stop services
docker-compose -f docker/docker-compose.test.yml down
```

#### Manual Container Management

```bash
# Run integration tests with Seq
docker run -d --name seq-test -e ACCEPT_EULA=Y -e SEQ_FIRSTRUN_NOAUTHENTICATION=true -p 8080:80 -p 5342:5341 datalust/seq
go test -tags=integration ./...
docker stop seq-test && docker rm seq-test

# Run integration tests with Elasticsearch
docker run -d --name es-test -e "discovery.type=single-node" -e "xpack.security.enabled=false" -p 9200:9200 docker.elastic.co/elasticsearch/elasticsearch:8.11.1
# Wait for Elasticsearch to be ready
sleep 30
go test -tags=integration ./...
docker stop es-test && docker rm es-test

# Run integration tests with Splunk
docker run -d --name splunk-test -p 8000:8000 -p 8088:8088 -e SPLUNK_START_ARGS="--accept-license" -e SPLUNK_PASSWORD="changeme" -e SPLUNK_HEC_TOKEN="00000000-0000-0000-0000-000000000000" splunk/splunk:latest
# Wait for Splunk to be ready
sleep 60
go test -tags=integration ./...
docker stop splunk-test && docker rm splunk-test
```

### Integration Tests

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

# Run tests with docker-compose
docker-compose -f docker/docker-compose.test.yml up -d
go test -tags=integration ./...
docker-compose -f docker/docker-compose.test.yml down
```

## Testing Philosophy for This Project

1. **Real Dependencies** - Use real Seq, real Elasticsearch via containers
2. **Memory Sinks** - For unit tests, use in-memory sinks that capture events
3. **Table-Driven** - Go's table-driven tests are perfect for template parsing
4. **Integration First** - Test the actual integration points, not mocks
5. **Benchmarks** - Track allocations from day one

This approach means your tests actually prove the system works with real infrastructure, which is much more valuable than mock-based tests.