package enrichers

import (
	"os"
	"sync"
	
	"github.com/willibrandon/mtlog/core"
)

// ProcessEnricher adds process information to log events.
type ProcessEnricher struct {
	processID   int
	processName string
	once        sync.Once
}

// NewProcessEnricher creates a new process enricher.
func NewProcessEnricher() *ProcessEnricher {
	return &ProcessEnricher{}
}

// Enrich adds process information to the log event.
func (pe *ProcessEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	// Get process info once and cache it
	pe.once.Do(func() {
		pe.processID = os.Getpid()
		pe.processName = os.Args[0]
	})
	
	// Add process ID
	pidProp := propertyFactory.CreateProperty("ProcessId", pe.processID)
	event.Properties[pidProp.Name] = pidProp.Value
	
	// Add process name
	nameProp := propertyFactory.CreateProperty("ProcessName", pe.processName)
	event.Properties[nameProp.Name] = nameProp.Value
}