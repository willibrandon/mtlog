package sinks

import (
	"sync"

	"github.com/willibrandon/mtlog/core"
)

// MemorySink stores log events in memory for testing purposes.
type MemorySink struct {
	events []core.LogEvent
	mu     sync.RWMutex
}

// NewMemorySink creates a new memory sink.
func NewMemorySink() *MemorySink {
	return &MemorySink{
		events: make([]core.LogEvent, 0),
	}
}

// Emit stores the event in memory.
func (m *MemorySink) Emit(event *core.LogEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Make a copy to avoid data races
	eventCopy := *event
	if event.Properties != nil {
		eventCopy.Properties = make(map[string]any)
		for k, v := range event.Properties {
			eventCopy.Properties[k] = v
		}
	}

	m.events = append(m.events, eventCopy)
}

// Close does nothing for memory sink.
func (m *MemorySink) Close() error {
	return nil
}

// Events returns a copy of all stored events.
func (m *MemorySink) Events() []core.LogEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]core.LogEvent, len(m.events))
	copy(result, m.events)
	return result
}

// Clear removes all stored events.
func (m *MemorySink) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = m.events[:0]
}

// Count returns the number of stored events.
func (m *MemorySink) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.events)
}

// FindEvents returns events that match the given predicate.
func (m *MemorySink) FindEvents(predicate func(*core.LogEvent) bool) []core.LogEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []core.LogEvent
	for _, event := range m.events {
		if predicate(&event) {
			result = append(result, event)
		}
	}
	return result
}

// HasEvent returns true if any event matches the predicate.
func (m *MemorySink) HasEvent(predicate func(*core.LogEvent) bool) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, event := range m.events {
		if predicate(&event) {
			return true
		}
	}
	return false
}

// LastEvent returns the most recent event, or nil if no events.
func (m *MemorySink) LastEvent() *core.LogEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.events) == 0 {
		return nil
	}

	event := m.events[len(m.events)-1]
	return &event
}
