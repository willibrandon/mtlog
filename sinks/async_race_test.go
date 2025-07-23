// +build race

package sinks_test

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// TestAsyncSinkRaceCondition tests for race conditions in async sink
func TestAsyncSinkRaceCondition(t *testing.T) {
	// Create a slow sink that simulates processing time
	slowSink := &slowTestSink{
		processTime: 10 * time.Microsecond,
		events:      make([]core.LogEvent, 0),
		mu:          &sync.Mutex{},
	}

	// Wrap in async sink with small buffer to force contention
	asyncSink := sinks.NewAsyncSink(slowSink, sinks.AsyncOptions{
		BufferSize: 10,
	})

	// Create logger
	logger := mtlog.New(
		mtlog.WithSink(asyncSink),
		mtlog.WithMinimumLevel(core.VerboseLevel),
	)

	// Number of goroutines and events per goroutine
	numGoroutines := 100
	eventsPerGoroutine := 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Start time for coordinated start
	start := time.Now().Add(10 * time.Millisecond)

	// Launch goroutines that all log concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()

			// Wait for coordinated start
			time.Sleep(time.Until(start))

			// Log many events rapidly
			for j := 0; j < eventsPerGoroutine; j++ {
				logger.Information("Event from routine {RoutineID}, event {EventID}", routineID, j)
				
				// Occasionally use different log levels
				if j%10 == 0 {
					logger.Debug("Debug event {RoutineID}-{EventID}", routineID, j)
				}
				if j%20 == 0 {
					logger.Warning("Warning event {RoutineID}-{EventID}", routineID, j)
				}
			}
		}(i)
	}

	// Wait for all goroutines to finish logging
	wg.Wait()

	// Close the async sink and wait for processing
	err := asyncSink.Close()
	if err != nil {
		t.Fatalf("Failed to close async sink: %v", err)
	}

	// Verify all events were processed
	processedEvents := slowSink.EventCount()
	// Each goroutine logs 100 regular + 10 debug + 5 warning = 115 events
	expectedEvents := numGoroutines * 115
	
	if processedEvents != expectedEvents {
		t.Errorf("Expected %d events, but got %d", expectedEvents, processedEvents)
	}

	// Check for any nil events or corrupted data
	events := slowSink.GetEvents()
	for i, event := range events {
		if event.Properties == nil {
			t.Errorf("Event %d has nil properties", i)
		}
		if event.MessageTemplate == "" {
			t.Errorf("Event %d has empty message template", i)
		}
	}
}

// TestAsyncSinkCloseRace tests racing Close() calls
func TestAsyncSinkCloseRace(t *testing.T) {
	memSink := sinks.NewMemorySink()
	asyncSink := sinks.NewAsyncSink(memSink, sinks.AsyncOptions{
		BufferSize: 100,
	})

	logger := mtlog.New(mtlog.WithSink(asyncSink))

	// Log some events
	for i := 0; i < 100; i++ {
		logger.Information("Event {ID}", i)
	}

	// Race multiple Close() calls
	var wg sync.WaitGroup
	numClosers := 10
	wg.Add(numClosers)

	for i := 0; i < numClosers; i++ {
		go func() {
			defer wg.Done()
			_ = asyncSink.Close() // Ignore error, testing for race
		}()
	}

	wg.Wait()
}

// TestAsyncSinkConcurrentEmitAndClose tests racing Emit and Close
func TestAsyncSinkConcurrentEmitAndClose(t *testing.T) {
	for i := 0; i < 10; i++ {
		memSink := sinks.NewMemorySink()
		asyncSink := sinks.NewAsyncSink(memSink, sinks.AsyncOptions{
			BufferSize: 50,
		})

		logger := mtlog.New(mtlog.WithSink(asyncSink))

		var wg sync.WaitGroup
		wg.Add(2)

		// Goroutine 1: Keep logging
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				logger.Information("Event {ID}", j)
				runtime.Gosched() // Yield to increase chance of race
			}
		}()

		// Goroutine 2: Close after a short delay
		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond)
			_ = asyncSink.Close()
		}()

		wg.Wait()
	}
}

// TestDurableSinkRaceCondition tests for race conditions in durable sink
func TestDurableSinkRaceCondition(t *testing.T) {
	// Create temp directory for durable buffer
	tempDir := t.TempDir()

	// Create memory sink as the target
	memSink := sinks.NewMemorySink()

	// Create durable sink
	durableSink, err := sinks.NewDurableSink(memSink, sinks.DurableOptions{
		BufferPath:        tempDir,
		MaxBufferSize:     10 * 1024 * 1024, // 10MB - larger buffer for race test
		RetryInterval:     5 * time.Millisecond,
		MaxBufferFiles:    5, // Allow multiple buffer files
		ChannelBufferSize: 5000, // Large enough for test load
		BatchSize:         50,   // Process in batches
	})
	if err != nil {
		t.Fatalf("Failed to create durable sink: %v", err)
	}

	logger := mtlog.New(mtlog.WithSink(durableSink))

	// Launch concurrent writers
	var wg sync.WaitGroup
	numWriters := 10  // Reduced to avoid overwhelming the buffer
	eventsPerWriter := 50

	wg.Add(numWriters)
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerWriter; j++ {
				logger.Information("Durable event from writer {WriterID}, event {EventID}", id, j)
				// Small delay to allow buffer to process
				if j%10 == 0 {
					time.Sleep(time.Microsecond)
				}
			}
		}(i)
	}

	wg.Wait()

	// Wait a bit for events to be queued
	time.Sleep(50 * time.Millisecond)

	// Close and verify
	err = durableSink.Close()
	if err != nil {
		t.Fatalf("Failed to close durable sink: %v", err)
	}
	
	// Verify all events were eventually delivered
	allEvents := memSink.Events()
	
	// Filter out health check events from durable sink
	var events []core.LogEvent
	for _, event := range allEvents {
		if event.MessageTemplate != "health check" {
			events = append(events, event)
		}
	}
	
	expectedEvents := numWriters * eventsPerWriter
	if len(events) != expectedEvents {
		t.Errorf("Expected %d events, got %d", expectedEvents, len(events))
		
		// Debug: Check what events we actually got
		writerEvents := make(map[int]int)
		for _, event := range events {
			if writerID, ok := event.Properties["WriterID"].(int); ok {
				writerEvents[writerID]++
			} else {
				t.Logf("Event missing WriterID: template=%q, props=%v, timestamp=%v", 
					event.MessageTemplate, event.Properties, event.Timestamp)
			}
		}
		
		// Check event distribution
		for i := 0; i < numWriters; i++ {
			if count, ok := writerEvents[i]; ok {
				if count != eventsPerWriter {
					t.Errorf("Writer %d: expected %d events, got %d", i, eventsPerWriter, count)
				}
			} else {
				t.Errorf("Writer %d: no events found", i)
			}
		}
	}
	
	// Verify no data corruption
	for i, event := range events {
		if event.Properties == nil {
			t.Errorf("Event %d has nil properties", i)
		}
		if event.MessageTemplate == "" {
			t.Errorf("Event %d has empty message template", i)
		}
	}
}

// slowTestSink simulates a slow sink for testing
type slowTestSink struct {
	processTime time.Duration
	events      []core.LogEvent
	mu          *sync.Mutex
	count       int64
}

func (s *slowTestSink) Emit(event *core.LogEvent) {
	// Simulate processing time
	time.Sleep(s.processTime)
	
	// Store event thread-safely
	s.mu.Lock()
	s.events = append(s.events, *event)
	s.mu.Unlock()
	
	atomic.AddInt64(&s.count, 1)
}

func (s *slowTestSink) Close() error {
	return nil
}

func (s *slowTestSink) EventCount() int {
	return int(atomic.LoadInt64(&s.count))
}

func (s *slowTestSink) GetEvents() []core.LogEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	result := make([]core.LogEvent, len(s.events))
	copy(result, s.events)
	return result
}