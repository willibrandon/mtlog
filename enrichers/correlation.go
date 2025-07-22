package enrichers

import (
	"runtime"
	"sync"
	
	"github.com/willibrandon/mtlog/core"
)

// CorrelationIdEnricher adds a correlation ID to all log events.
// This is useful for tracing requests across multiple services.
type CorrelationIdEnricher struct {
	correlationId string
}

// NewCorrelationIdEnricher creates an enricher with a specific correlation ID.
func NewCorrelationIdEnricher(correlationId string) *CorrelationIdEnricher {
	return &CorrelationIdEnricher{
		correlationId: correlationId,
	}
}

// Enrich adds the correlation ID to the log event.
func (c *CorrelationIdEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	if c.correlationId != "" {
		prop := propertyFactory.CreateProperty("CorrelationId", c.correlationId)
		event.Properties[prop.Name] = prop.Value
	}
}

// ThreadLocalCorrelationIdEnricher uses goroutine-local storage for correlation IDs.
// Note: This is a simplified implementation. In production, you might want to use
// a proper goroutine-local storage library or pass correlation IDs explicitly.
type ThreadLocalCorrelationIdEnricher struct {
	correlationIds sync.Map // goroutine ID -> correlation ID
}

// NewThreadLocalCorrelationIdEnricher creates a new thread-local correlation ID enricher.
func NewThreadLocalCorrelationIdEnricher() *ThreadLocalCorrelationIdEnricher {
	return &ThreadLocalCorrelationIdEnricher{}
}

// SetCorrelationId sets the correlation ID for the current goroutine.
func (t *ThreadLocalCorrelationIdEnricher) SetCorrelationId(correlationId string) {
	gid := getGoroutineId()
	if gid != "" {
		t.correlationIds.Store(gid, correlationId)
	}
}

// ClearCorrelationId clears the correlation ID for the current goroutine.
func (t *ThreadLocalCorrelationIdEnricher) ClearCorrelationId() {
	gid := getGoroutineId()
	if gid != "" {
		t.correlationIds.Delete(gid)
	}
}

// Enrich adds the correlation ID for the current goroutine to the log event.
func (t *ThreadLocalCorrelationIdEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	gid := getGoroutineId()
	if gid != "" {
		if value, ok := t.correlationIds.Load(gid); ok {
			if correlationId, ok := value.(string); ok && correlationId != "" {
				prop := propertyFactory.CreateProperty("CorrelationId", correlationId)
				event.Properties[prop.Name] = prop.Value
			}
		}
	}
}

// getGoroutineId extracts the current goroutine ID from the runtime.
// This is a workaround since Go doesn't expose goroutine IDs officially.
func getGoroutineId() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	stack := string(buf[:n])
	
	// Extract goroutine ID from stack trace
	// Stack trace starts with "goroutine <id> ["
	if len(stack) > 10 && stack[:9] == "goroutine" {
		for i := 10; i < len(stack); i++ {
			if stack[i] == ' ' {
				return stack[10:i]
			}
		}
	}
	
	return ""
}