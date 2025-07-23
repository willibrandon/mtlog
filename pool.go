package mtlog

import (
	"sync"
	
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

