package sinks

import (
	"github.com/willibrandon/mtlog/core"
)

// TypedSink is a generic sink that can handle specific event types
type TypedSink[T any] interface {
	EmitTyped(event *core.LogEvent, typedData T) error
	Close() error
}

// TypedBatchingSink batches typed events before emission
type TypedBatchingSink[T any] struct {
	innerSink  TypedSink[T]
	batchSize  int
	batch      []TypedEvent[T]
}

// TypedEvent combines a log event with typed data
type TypedEvent[T any] struct {
	Event *core.LogEvent
	Data  T
}

// NewTypedBatchingSink creates a new typed batching sink
func NewTypedBatchingSink[T any](innerSink TypedSink[T], batchSize int) *TypedBatchingSink[T] {
	return &TypedBatchingSink[T]{
		innerSink: innerSink,
		batchSize: batchSize,
		batch:     make([]TypedEvent[T], 0, batchSize),
	}
}

// EmitTyped adds a typed event to the batch
func (s *TypedBatchingSink[T]) EmitTyped(event *core.LogEvent, data T) error {
	s.batch = append(s.batch, TypedEvent[T]{Event: event, Data: data})
	
	if len(s.batch) >= s.batchSize {
		return s.Flush()
	}
	return nil
}

// Flush sends all batched events
func (s *TypedBatchingSink[T]) Flush() error {
	for _, te := range s.batch {
		if err := s.innerSink.EmitTyped(te.Event, te.Data); err != nil {
			return err
		}
	}
	s.batch = s.batch[:0]
	return nil
}

// Close flushes and closes the sink
func (s *TypedBatchingSink[T]) Close() error {
	if err := s.Flush(); err != nil {
		return err
	}
	return s.innerSink.Close()
}

// FilteredSink applies type-safe filtering
type FilteredSink[T any] struct {
	innerSink TypedSink[T]
	predicate func(event *core.LogEvent, data T) bool
}

// NewFilteredSink creates a new filtered sink
func NewFilteredSink[T any](innerSink TypedSink[T], predicate func(*core.LogEvent, T) bool) *FilteredSink[T] {
	return &FilteredSink[T]{
		innerSink: innerSink,
		predicate: predicate,
	}
}

// EmitTyped emits only if predicate returns true
func (s *FilteredSink[T]) EmitTyped(event *core.LogEvent, data T) error {
	if s.predicate(event, data) {
		return s.innerSink.EmitTyped(event, data)
	}
	return nil
}

// Close closes the inner sink
func (s *FilteredSink[T]) Close() error {
	return s.innerSink.Close()
}