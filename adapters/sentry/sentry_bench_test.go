package sentry

import (
	"errors"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/willibrandon/mtlog/core"
)

func BenchmarkMessageInterpolation(b *testing.B) {
	sink := &SentrySink{}
	event := &core.LogEvent{
		MessageTemplate: "User {UserId} performed {Action} on {Resource}",
		Properties: map[string]interface{}{
			"UserId":   "user-123",
			"Action":   "DELETE",
			"Resource": "post-789",
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sink.renderMessage(event)
	}
}

func BenchmarkEventConversion(b *testing.B) {
	sink := &SentrySink{}
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.ErrorLevel,
		MessageTemplate: "Error: {Error}",
		Properties: map[string]interface{}{
			"Error": errors.New("test"),
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sink.convertToSentryEvent(event)
	}
}

func BenchmarkComplexMessageInterpolation(b *testing.B) {
	sink := &SentrySink{}
	event := &core.LogEvent{
		MessageTemplate: "Order {OrderId} for customer {CustomerId} containing {ItemCount} items worth {Total:F2} failed at {Timestamp} with error: {Error}",
		Properties: map[string]interface{}{
			"OrderId":    "ORD-12345",
			"CustomerId": "CUST-67890",
			"ItemCount":  42,
			"Total":      1234.56,
			"Timestamp":  time.Now(),
			"Error":      errors.New("payment declined"),
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sink.renderMessage(event)
	}
}

func BenchmarkBreadcrumbAddition(b *testing.B) {
	sink := &SentrySink{
		breadcrumbs:     NewBreadcrumbBuffer(100),
		breadcrumbLevel: core.DebugLevel,
	}
	event := &core.LogEvent{
		Level:           core.DebugLevel,
		MessageTemplate: "Debug: {Message}",
		Timestamp:       time.Now(),
		Properties: map[string]interface{}{
			"Message": "test breadcrumb",
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink.addBreadcrumb(event)
	}
}

func BenchmarkBatchProcessing(b *testing.B) {
	sink := &SentrySink{
		batch:     make([]*sentry.Event, 0, 100),
		batchSize: 100,
	}
	event := &core.LogEvent{
		Timestamp:       time.Now(),
		Level:           core.ErrorLevel,
		MessageTemplate: "Error occurred",
		Properties:      map[string]interface{}{},
	}
	sentryEvent := sink.convertToSentryEvent(event)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sink.batchMu.Lock()
		sink.batch = append(sink.batch, sentryEvent)
		if len(sink.batch) >= sink.batchSize {
			sink.batch = make([]*sentry.Event, 0, sink.batchSize)
		}
		sink.batchMu.Unlock()
	}
}