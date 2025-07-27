package enrichers

import (
	"runtime"
	"strconv"
	
	"github.com/willibrandon/mtlog/core"
)

// ThreadIdEnricher adds the current goroutine ID to log events.
// Note: Go doesn't expose goroutine IDs officially, so this uses a workaround.
type ThreadIdEnricher struct{}

// NewThreadIdEnricher creates a new thread ID enricher.
func NewThreadIdEnricher() *ThreadIdEnricher {
	return &ThreadIdEnricher{}
}

// Enrich adds the goroutine ID to the log event.
func (t *ThreadIdEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	// Get goroutine ID from runtime
	buf := make([]byte, 64)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])
	
	// Extract goroutine ID from stack trace
	// Stack trace starts with "goroutine <id> ["
	var goroutineId string
	if len(stack) > 10 && stack[:9] == "goroutine" {
		for i := 10; i < len(stack); i++ {
			if stack[i] == ' ' {
				goroutineId = stack[10:i]
				break
			}
		}
	}
	
	if goroutineId != "" {
		if id, err := strconv.Atoi(goroutineId); err == nil {
			prop := propertyFactory.CreateProperty("ThreadId", id)
			event.Properties[prop.Name] = prop.Value
		}
	}
}

// CallersEnricher adds caller information to log events.
type CallersEnricher struct {
	skip int
}

// NewCallersEnricher creates an enricher that adds caller information.
// skip determines how many stack frames to skip (default should be 6-7 for most logger implementations).
func NewCallersEnricher(skip int) *CallersEnricher {
	return &CallersEnricher{skip: skip}
}

// Enrich adds caller information to the log event.
func (c *CallersEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	_, file, line, ok := runtime.Caller(c.skip)
	if ok {
		prop := propertyFactory.CreateProperty("Caller", file+":"+strconv.Itoa(line))
		event.Properties[prop.Name] = prop.Value
		
		// Also add separate properties for file and line
		fileProp := propertyFactory.CreateProperty("SourceFile", file)
		event.Properties[fileProp.Name] = fileProp.Value
		
		lineProp := propertyFactory.CreateProperty("SourceLine", line)
		event.Properties[lineProp.Name] = lineProp.Value
	}
}