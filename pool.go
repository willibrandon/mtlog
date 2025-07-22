package mtlog

import (
	"sync"
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// pools holds all object pools used by the logger.
var pools = struct {
	properties *sync.Pool
	events     *sync.Pool
}{
	properties: &sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{}, 8) // Pre-size for typical use
		},
	},
	events: &sync.Pool{
		New: func() interface{} {
			return &core.LogEvent{
				Properties: make(map[string]interface{}, 8),
			}
		},
	},
}

// getPropertyMap gets a property map from the pool.
func getPropertyMap() map[string]interface{} {
	m := pools.properties.Get().(map[string]interface{})
	// Clear the map for reuse
	for k := range m {
		delete(m, k)
	}
	return m
}

// putPropertyMap returns a property map to the pool.
func putPropertyMap(m map[string]interface{}) {
	// Don't pool maps that have grown too large
	if len(m) > 32 {
		return
	}
	pools.properties.Put(m)
}

// getLogEvent gets a log event from the pool.
func getLogEvent() *core.LogEvent {
	return pools.events.Get().(*core.LogEvent)
}

// putLogEvent returns a log event to the pool.
func putLogEvent(e *core.LogEvent) {
	// Clear the event for reuse
	e.Timestamp = time.Time{}
	e.Level = 0
	e.MessageTemplate = ""
	e.Exception = nil
	
	// Clear properties
	for k := range e.Properties {
		delete(e.Properties, k)
	}
	
	pools.events.Put(e)
}