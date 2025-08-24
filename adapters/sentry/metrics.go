package sentry

import (
	"sync/atomic"
	"time"
)

// Metrics provides runtime statistics for the Sentry sink.
type Metrics struct {
	// Event statistics
	EventsSent     int64
	EventsDropped  int64
	EventsFailed   int64
	EventsRetried  int64

	// Breadcrumb statistics
	BreadcrumbsAdded   int64
	BreadcrumbsEvicted int64

	// Batch statistics
	BatchesSent      int64
	AverageBatchSize float64

	// Performance metrics
	LastFlushDuration time.Duration
	TotalFlushTime    time.Duration

	// Network statistics
	RetryCount    int64
	NetworkErrors int64
}

// metricsCollector handles atomic metric updates
type metricsCollector struct {
	eventsSent         atomic.Int64
	eventsDropped      atomic.Int64
	eventsFailed       atomic.Int64
	eventsRetried      atomic.Int64
	breadcrumbsAdded   atomic.Int64
	breadcrumbsEvicted atomic.Int64
	batchesSent        atomic.Int64
	totalBatchSize     atomic.Int64
	lastFlushDuration  atomic.Int64
	totalFlushTime     atomic.Int64
	retryCount         atomic.Int64
	networkErrors      atomic.Int64
}

func newMetricsCollector() *metricsCollector {
	return &metricsCollector{}
}

func (m *metricsCollector) snapshot() Metrics {
	batchesSent := m.batchesSent.Load()
	totalBatchSize := m.totalBatchSize.Load()

	avgBatchSize := float64(0)
	if batchesSent > 0 {
		avgBatchSize = float64(totalBatchSize) / float64(batchesSent)
	}

	return Metrics{
		EventsSent:         m.eventsSent.Load(),
		EventsDropped:      m.eventsDropped.Load(),
		EventsFailed:       m.eventsFailed.Load(),
		EventsRetried:      m.eventsRetried.Load(),
		BreadcrumbsAdded:   m.breadcrumbsAdded.Load(),
		BreadcrumbsEvicted: m.breadcrumbsEvicted.Load(),
		BatchesSent:        batchesSent,
		AverageBatchSize:   avgBatchSize,
		LastFlushDuration:  time.Duration(m.lastFlushDuration.Load()),
		TotalFlushTime:     time.Duration(m.totalFlushTime.Load()),
		RetryCount:         m.retryCount.Load(),
		NetworkErrors:      m.networkErrors.Load(),
	}
}