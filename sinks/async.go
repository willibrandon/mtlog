package sinks

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

// AsyncOptions configures the async sink wrapper.
type AsyncOptions struct {
	// BufferSize is the size of the channel buffer for log events.
	BufferSize int
	
	// OverflowStrategy defines what to do when the buffer is full.
	OverflowStrategy OverflowStrategy
	
	// FlushInterval is how often to flush events to the wrapped sink.
	// 0 means flush immediately for each event.
	FlushInterval time.Duration
	
	// BatchSize is the maximum number of events to batch together.
	// 0 means no batching.
	BatchSize int
	
	// OnError is called when an error occurs in the background worker.
	OnError func(error)
	
	// ShutdownTimeout is the maximum time to wait for pending events during shutdown.
	ShutdownTimeout time.Duration
}

// OverflowStrategy defines what to do when the async buffer is full.
type OverflowStrategy int

const (
	// OverflowBlock blocks the caller until space is available.
	OverflowBlock OverflowStrategy = iota
	
	// OverflowDrop drops the newest events when the buffer is full.
	OverflowDrop
	
	// OverflowDropOldest drops the oldest events to make room for new ones.
	OverflowDropOldest
)

// AsyncSink wraps another sink to provide asynchronous, non-blocking logging.
type AsyncSink struct {
	wrapped    core.LogEventSink
	options    AsyncOptions
	events     chan *core.LogEvent
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	
	// Metrics
	dropped    atomic.Uint64
	processed  atomic.Uint64
	errors     atomic.Uint64
}

// NewAsyncSink creates a new async sink wrapper.
func NewAsyncSink(wrapped core.LogEventSink, options AsyncOptions) *AsyncSink {
	// Apply defaults
	if options.BufferSize <= 0 {
		options.BufferSize = 1000
	}
	if options.ShutdownTimeout <= 0 {
		options.ShutdownTimeout = 30 * time.Second
	}
	if options.OnError == nil {
		options.OnError = func(err error) {
			fmt.Printf("AsyncSink error: %v\n", err)
		}
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	sink := &AsyncSink{
		wrapped: wrapped,
		options: options,
		events:  make(chan *core.LogEvent, options.BufferSize),
		ctx:     ctx,
		cancel:  cancel,
	}
	
	// Start background worker
	sink.wg.Add(1)
	go sink.worker()
	
	return sink
}

// Emit sends a log event to the async buffer.
func (as *AsyncSink) Emit(event *core.LogEvent) {
	select {
	case as.events <- event:
		// Event queued successfully
		
	default:
		// Buffer is full, apply overflow strategy
		switch as.options.OverflowStrategy {
		case OverflowBlock:
			// Block until we can send
			select {
			case as.events <- event:
				// Success
			case <-as.ctx.Done():
				// Shutting down, drop the event
				as.dropped.Add(1)
			}
			
		case OverflowDrop:
			// Drop this event
			as.dropped.Add(1)
			if selflog.IsEnabled() {
				dropped := as.dropped.Load()
				if dropped == 1 || dropped%1000 == 0 { // Log first drop and every 1000th
					selflog.Printf("[async] buffer full, dropped %d events total", dropped)
				}
			}
			
		case OverflowDropOldest:
			// Try to remove the oldest event
			select {
			case <-as.events:
				// Removed oldest, now try to add new one
				select {
				case as.events <- event:
					// Success
				default:
					// Still couldn't add, drop it
					as.dropped.Add(1)
				}
			default:
				// Couldn't remove oldest, drop new event
				as.dropped.Add(1)
			}
		}
	}
}

// Close shuts down the async sink and waits for pending events.
func (as *AsyncSink) Close() error {
	// Signal shutdown
	as.cancel()
	
	// Wait for worker to finish with timeout
	done := make(chan struct{})
	go func() {
		as.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Worker finished
	case <-time.After(as.options.ShutdownTimeout):
		// Timeout waiting for worker
		return fmt.Errorf("timeout waiting for async sink to shut down")
	}
	
	// Close wrapped sink
	if closer, ok := as.wrapped.(interface{ Close() error }); ok {
		return closer.Close()
	}
	
	return nil
}

// worker is the background goroutine that processes events.
func (as *AsyncSink) worker() {
	defer as.wg.Done()
	
	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			if selflog.IsEnabled() {
				selflog.Printf("[async] worker panic: %v", r)
			}
			if as.options.OnError != nil {
				as.options.OnError(fmt.Errorf("worker panic: %v", r))
			}
		}
	}()
	
	// Batch for collecting events
	var batch []*core.LogEvent
	if as.options.BatchSize > 0 {
		batch = make([]*core.LogEvent, 0, as.options.BatchSize)
	}
	
	// Timer for periodic flushing
	var flushTimer *time.Timer
	if as.options.FlushInterval > 0 {
		flushTimer = time.NewTimer(as.options.FlushInterval)
		defer flushTimer.Stop()
	} else {
		// Create a timer that never fires
		flushTimer = time.NewTimer(24 * time.Hour)
		flushTimer.Stop()
	}
	
	for {
		select {
		case event := <-as.events:
			if event == nil {
				continue
			}
			
			if as.options.BatchSize > 0 {
				// Add to batch
				batch = append(batch, event)
				
				// Flush if batch is full
				if len(batch) >= as.options.BatchSize {
					as.flushBatch(batch)
					batch = batch[:0]
				}
			} else {
				// No batching, emit immediately
				as.emitSingle(event)
			}
			
		case <-flushTimer.C:
			// Periodic flush
			if len(batch) > 0 {
				as.flushBatch(batch)
				batch = batch[:0]
			}
			
			// Reset timer
			if as.options.FlushInterval > 0 {
				flushTimer.Reset(as.options.FlushInterval)
			}
			
		case <-as.ctx.Done():
			// Shutting down, process remaining events
			
			// First, flush any batched events
			if len(batch) > 0 {
				as.flushBatch(batch)
			}
			
			// Then process any remaining events in the channel
			for {
				select {
				case event := <-as.events:
					if event != nil {
						as.emitSingle(event)
					}
				default:
					// No more events
					return
				}
			}
		}
	}
}

// emitSingle emits a single event to the wrapped sink.
func (as *AsyncSink) emitSingle(event *core.LogEvent) {
	defer func() {
		if r := recover(); r != nil {
			as.errors.Add(1)
			if selflog.IsEnabled() {
				selflog.Printf("[async] wrapped sink panic: %v", r)
			}
			if as.options.OnError != nil {
				as.options.OnError(fmt.Errorf("panic in wrapped sink: %v", r))
			}
		}
	}()
	
	as.wrapped.Emit(event)
	as.processed.Add(1)
}

// flushBatch emits a batch of events to the wrapped sink.
func (as *AsyncSink) flushBatch(batch []*core.LogEvent) {
	// Check if wrapped sink supports batch emit
	if batchSink, ok := as.wrapped.(interface {
		EmitBatch([]*core.LogEvent)
	}); ok {
		defer func() {
			if r := recover(); r != nil {
				as.errors.Add(uint64(len(batch)))
				if selflog.IsEnabled() {
					selflog.Printf("[async] wrapped sink batch panic: %v (batch_size=%d)", r, len(batch))
				}
				if as.options.OnError != nil {
					as.options.OnError(fmt.Errorf("panic in wrapped sink batch emit: %v", r))
				}
			}
		}()
		
		batchSink.EmitBatch(batch)
		as.processed.Add(uint64(len(batch)))
	} else {
		// Emit individually
		for _, event := range batch {
			as.emitSingle(event)
		}
	}
}

// GetMetrics returns metrics about the async sink operation.
func (as *AsyncSink) GetMetrics() AsyncMetrics {
	return AsyncMetrics{
		Processed:  as.processed.Load(),
		Dropped:    as.dropped.Load(),
		Errors:     as.errors.Load(),
		BufferSize: len(as.events),
		BufferCap:  cap(as.events),
	}
}

// AsyncMetrics contains operational metrics for the async sink.
type AsyncMetrics struct {
	Processed  uint64 // Number of events successfully processed
	Dropped    uint64 // Number of events dropped due to overflow
	Errors     uint64 // Number of errors encountered
	BufferSize int    // Current number of events in buffer
	BufferCap  int    // Buffer capacity
}

// WaitForEmpty blocks until the async buffer is empty or the context is cancelled.
// This is useful for testing or graceful shutdown scenarios.
func (as *AsyncSink) WaitForEmpty(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if len(as.events) == 0 {
				return nil
			}
		}
	}
}