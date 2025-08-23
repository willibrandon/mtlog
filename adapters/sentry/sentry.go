// Package sentry provides a Sentry integration sink for mtlog.
// It automatically tracks errors, captures stack traces, and provides
// breadcrumb support for production error monitoring.
package sentry

import (
	"fmt"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

// SentrySink sends log events to Sentry for error tracking and monitoring.
// It supports batching, breadcrumb collection, and custom fingerprinting.
type SentrySink struct {
	client *sentry.Client
	hub    *sentry.Hub

	// Configuration
	minLevel         core.LogEventLevel
	breadcrumbLevel  core.LogEventLevel
	sampleRate       float64
	environment      string
	release          string
	serverName       string
	maxBreadcrumbs   int

	// Breadcrumbs
	breadcrumbs *BreadcrumbBuffer

	// Fingerprinting
	fingerprinter Fingerprinter

	// Batching
	batchSize    int
	batchTimeout time.Duration
	batch        []*sentry.Event
	batchMu      sync.Mutex
	timer        *time.Timer
	stopCh       chan struct{}
	flushCh      chan struct{}
	wg           sync.WaitGroup

	// BeforeSend hook
	beforeSend sentry.EventProcessor
}

// Fingerprinter is a function that generates fingerprints for error grouping.
type Fingerprinter func(*core.LogEvent) []string

// WithSentry creates a new Sentry sink with the given DSN and options.
// This is a convenience function that returns a core.LogEventSink.
func WithSentry(dsn string, opts ...Option) (core.LogEventSink, error) {
	return NewSentrySink(dsn, opts...)
}

// NewSentrySink creates a new Sentry sink with the given DSN and options.
func NewSentrySink(dsn string, opts ...Option) (*SentrySink, error) {
	s := &SentrySink{
		minLevel:        core.ErrorLevel,
		breadcrumbLevel: core.DebugLevel,
		sampleRate:      1.0,
		maxBreadcrumbs:  100,
		batchSize:       100,
		batchTimeout:    5 * time.Second,
		stopCh:          make(chan struct{}),
		flushCh:         make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Create Sentry client
	clientOpts := sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      s.environment,
		Release:          s.release,
		ServerName:       s.serverName,
		SampleRate:       s.sampleRate,
		AttachStacktrace: true,
		BeforeSend:       s.beforeSend,
	}

	client, err := sentry.NewClient(clientOpts)
	if err != nil {
		if selflog.IsEnabled() {
			selflog.Printf("[sentry] failed to create client: %v", err)
		}
		return nil, fmt.Errorf("failed to create Sentry client: %w", err)
	}

	s.client = client
	s.hub = sentry.NewHub(client, sentry.NewScope())
	s.breadcrumbs = NewBreadcrumbBuffer(s.maxBreadcrumbs)
	s.batch = make([]*sentry.Event, 0, s.batchSize)

	// Start background worker
	s.wg.Add(1)
	go s.worker()

	return s, nil
}

// Emit sends a log event to Sentry.
func (s *SentrySink) Emit(event *core.LogEvent) {
	if event == nil {
		return
	}

	// Check if this should be a breadcrumb
	if event.Level < s.minLevel && event.Level >= s.breadcrumbLevel {
		s.addBreadcrumb(event)
		return
	}

	// Only send events at or above minimum level
	if event.Level < s.minLevel {
		return
	}

	// Convert to Sentry event
	sentryEvent := s.convertToSentryEvent(event)
	if sentryEvent == nil {
		return
	}

	// Add to batch
	s.batchMu.Lock()
	s.batch = append(s.batch, sentryEvent)
	shouldFlush := len(s.batch) >= s.batchSize
	s.batchMu.Unlock()

	if shouldFlush {
		select {
		case s.flushCh <- struct{}{}:
		default:
		}
	}
}

// Close flushes any pending events and closes the sink.
func (s *SentrySink) Close() error {
	close(s.stopCh)
	s.wg.Wait()

	// Final flush
	s.flush()

	// Flush Sentry client
	if s.client != nil {
		if !s.client.Flush(2 * time.Second) {
			if selflog.IsEnabled() {
				selflog.Printf("[sentry] timeout during final flush")
			}
		}
	}

	return nil
}

// worker handles batching and periodic flushing.
func (s *SentrySink) worker() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.flush()
		case <-s.flushCh:
			s.flush()
			ticker.Reset(s.batchTimeout)
		}
	}
}

// flush sends all batched events to Sentry.
func (s *SentrySink) flush() {
	s.batchMu.Lock()
	if len(s.batch) == 0 {
		s.batchMu.Unlock()
		return
	}
	events := s.batch
	s.batch = make([]*sentry.Event, 0, s.batchSize)
	s.batchMu.Unlock()

	for _, event := range events {
		// Attach current breadcrumbs
		event.Breadcrumbs = s.breadcrumbs.GetAll()

		// Send to Sentry
		eventID := s.hub.CaptureEvent(event)
		if eventID == nil && selflog.IsEnabled() {
			selflog.Printf("[sentry] failed to capture event: %s", event.Message)
		}
	}
}

// addBreadcrumb adds a log event as a breadcrumb.
func (s *SentrySink) addBreadcrumb(event *core.LogEvent) {
	breadcrumb := sentry.Breadcrumb{
		Type:      "default",
		Category:  levelToCategory(event.Level),
		Message:   event.MessageTemplate,
		Level:     levelToSentryLevel(event.Level),
		Timestamp: event.Timestamp,
	}

	// Add properties as data
	if len(event.Properties) > 0 {
		breadcrumb.Data = make(map[string]interface{})
		for k, v := range event.Properties {
			breadcrumb.Data[k] = v
		}
	}

	s.breadcrumbs.Add(breadcrumb)
}

// convertToSentryEvent converts a log event to a Sentry event.
func (s *SentrySink) convertToSentryEvent(event *core.LogEvent) *sentry.Event {
	sentryEvent := &sentry.Event{
		Message:   event.MessageTemplate,
		Level:     levelToSentryLevel(event.Level),
		Timestamp: event.Timestamp,
		Extra:     make(map[string]interface{}),
		Tags:      make(map[string]string),
	}

	// Add message template as tag for grouping
	sentryEvent.Tags["message.template"] = event.MessageTemplate

	// Add properties as extra data
	for k, v := range event.Properties {
		// Check for special properties
		switch k {
		case "error", "err", "Error":
			if err, ok := v.(error); ok {
				sentryEvent.Exception = s.extractException(err)
			}
		case "user", "User":
			if user, ok := v.(sentry.User); ok {
				sentryEvent.User = user
			}
		default:
			sentryEvent.Extra[k] = v
		}
	}

	// Apply custom fingerprinting
	if s.fingerprinter != nil {
		sentryEvent.Fingerprint = s.fingerprinter(event)
	}

	return sentryEvent
}

// extractException extracts exception information from an error.
func (s *SentrySink) extractException(err error) []sentry.Exception {
	return []sentry.Exception{
		{
			Type:       fmt.Sprintf("%T", err),
			Value:      err.Error(),
			Stacktrace: sentry.ExtractStacktrace(err),
		},
	}
}

// levelToSentryLevel converts mtlog level to Sentry level.
func levelToSentryLevel(level core.LogEventLevel) sentry.Level {
	switch level {
	case core.VerboseLevel, core.DebugLevel:
		return sentry.LevelDebug
	case core.InformationLevel:
		return sentry.LevelInfo
	case core.WarningLevel:
		return sentry.LevelWarning
	case core.ErrorLevel:
		return sentry.LevelError
	case core.FatalLevel:
		return sentry.LevelFatal
	default:
		return sentry.LevelInfo
	}
}

// levelToCategory returns a breadcrumb category for a log level.
func levelToCategory(level core.LogEventLevel) string {
	switch level {
	case core.VerboseLevel, core.DebugLevel:
		return "debug"
	case core.InformationLevel:
		return "info"
	case core.WarningLevel:
		return "warning"
	case core.ErrorLevel:
		return "error"
	case core.FatalLevel:
		return "fatal"
	default:
		return "log"
	}
}