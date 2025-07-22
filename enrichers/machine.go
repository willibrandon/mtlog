package enrichers

import (
	"os"
	"sync"
	
	"github.com/willibrandon/mtlog/core"
)

// MachineNameEnricher adds the machine name to log events.
type MachineNameEnricher struct {
	propertyName string
	machineName  string
	once         sync.Once
}

// NewMachineNameEnricher creates a new machine name enricher.
func NewMachineNameEnricher() *MachineNameEnricher {
	return &MachineNameEnricher{
		propertyName: "MachineName",
	}
}

// NewMachineNameEnricherWithName creates a new machine name enricher with a custom property name.
func NewMachineNameEnricherWithName(propertyName string) *MachineNameEnricher {
	return &MachineNameEnricher{
		propertyName: propertyName,
	}
}

// Enrich adds the machine name to the log event.
func (me *MachineNameEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	// Get machine name once and cache it
	me.once.Do(func() {
		hostname, err := os.Hostname()
		if err != nil {
			me.machineName = "unknown"
		} else {
			me.machineName = hostname
		}
	})
	
	prop := propertyFactory.CreateProperty(me.propertyName, me.machineName)
	event.Properties[prop.Name] = prop.Value
}