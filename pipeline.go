package mtlog

import (
	"time"
	
	"github.com/willibrandon/mtlog/core"
)

// pipeline represents the immutable logging pipeline.
// Once created, the pipeline cannot be modified.
type pipeline struct {
	enrichers    []core.LogEventEnricher
	filters      []core.LogEventFilter
	destructurer core.Destructurer
	sinks        []core.LogEventSink
}

// newPipeline creates a new pipeline with the given stages.
func newPipeline(enrichers []core.LogEventEnricher, filters []core.LogEventFilter, destructurer core.Destructurer, sinks []core.LogEventSink) *pipeline {
	return &pipeline{
		enrichers:    enrichers,
		filters:      filters,
		destructurer: destructurer,
		sinks:        sinks,
	}
}

// process runs a log event through all pipeline stages.
func (p *pipeline) process(event *core.LogEvent, factory core.LogEventPropertyFactory) {
	// Stage 1: Enrichment - add contextual properties
	for _, enricher := range p.enrichers {
		enricher.Enrich(event, factory)
	}
	
	// Stage 2: Filtering - determine if event should proceed
	for _, filter := range p.filters {
		if !filter.IsEnabled(event) {
			return // Event filtered out
		}
	}
	
	// Stage 3: Destructuring - handled during property extraction for @ hints
	// The destructurer is made available to the logger but not applied here
	
	// Stage 4: Output - send to sinks
	for _, sink := range p.sinks {
		sink.Emit(event)
	}
}

// processSimple handles the fast path for simple string messages.
func (p *pipeline) processSimple(timestamp time.Time, level core.LogEventLevel, message string) {
	// Fast path bypasses enrichment, filtering, and destructuring
	for _, sink := range p.sinks {
		if simpleSink, ok := sink.(core.SimpleSink); ok {
			simpleSink.EmitSimple(timestamp, level, message)
		} else {
			// Fallback for sinks that don't implement SimpleSink
			event := &core.LogEvent{
				Timestamp:       timestamp,
				Level:           level,
				MessageTemplate: message,
				Properties:      make(map[string]interface{}),
			}
			sink.Emit(event)
		}
	}
}