package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/sentry"
	"github.com/willibrandon/mtlog/core"
)

// MetricsMonitor tracks and displays Sentry metrics
type MetricsMonitor struct {
	sink       *sentry.SentrySink
	mu         sync.RWMutex
	history    []sentry.Metrics
	maxHistory int
}

func NewMetricsMonitor(sink *sentry.SentrySink, maxHistory int) *MetricsMonitor {
	return &MetricsMonitor{
		sink:       sink,
		history:    make([]sentry.Metrics, 0, maxHistory),
		maxHistory: maxHistory,
	}
}

func (m *MetricsMonitor) Collect() {
	metrics := m.sink.Metrics()
	
	m.mu.Lock()
	m.history = append(m.history, metrics)
	if len(m.history) > m.maxHistory {
		m.history = m.history[1:]
	}
	m.mu.Unlock()
}

func (m *MetricsMonitor) GetTrend() (eventRate float64, errorRate float64, retryRate float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if len(m.history) < 2 {
		return 0, 0, 0
	}
	
	first := m.history[0]
	last := m.history[len(m.history)-1]
	
	totalEvents := last.EventsSent - first.EventsSent
	if totalEvents > 0 {
		eventRate = float64(totalEvents) / float64(len(m.history))
		errorRate = float64(last.EventsFailed-first.EventsFailed) / float64(totalEvents) * 100
		retryRate = float64(last.RetryCount-first.RetryCount) / float64(totalEvents) * 100
	}
	
	return
}

func main() {
	// Create Sentry sink with comprehensive metrics
	dsn := "https://your-key@sentry.io/project-id"
	sentrySink, err := sentry.NewSentrySink(dsn,
		sentry.WithEnvironment("production"),
		sentry.WithRelease("v1.5.0"),
		
		// Configure for metrics monitoring
		sentry.WithMinLevel(core.DebugLevel),
		sentry.WithBreadcrumbLevel(core.VerboseLevel),
		sentry.WithMaxBreadcrumbs(200),
		
		// Batching configuration
		sentry.WithBatchSize(50),
		sentry.WithBatchTimeout(3*time.Second),
		
		// Retry configuration
		sentry.WithRetry(2, 500*time.Millisecond),
		sentry.WithRetryJitter(0.3),
		
		// Stack trace caching for performance
		sentry.WithStackTraceCacheSize(100),
		
		// Enable metrics
		sentry.WithMetrics(true),
	)
	if err != nil {
		log.Fatalf("Failed to create Sentry sink: %v", err)
	}
	defer sentrySink.Close()

	// Create logger
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithSink(sentrySink),
	)

	// Create metrics monitor
	monitor := NewMetricsMonitor(sentrySink, 20)

	// Set up periodic metrics display
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		
		for range ticker.C {
			m := sentrySink.Metrics()
		fmt.Printf("\n╔════════════════════════════════════════════╗\n")
		fmt.Printf("║          SENTRY METRICS DASHBOARD          ║\n")
		fmt.Printf("╠════════════════════════════════════════════╣\n")
		fmt.Printf("║ Events:                                    ║\n")
		fmt.Printf("║   Sent:        %-28d║\n", m.EventsSent)
		fmt.Printf("║   Failed:      %-28d║\n", m.EventsFailed)
		fmt.Printf("║   Dropped:     %-28d║\n", m.EventsDropped)
		fmt.Printf("║   Retried:     %-28d║\n", m.EventsRetried)
		fmt.Printf("╠════════════════════════════════════════════╣\n")
		fmt.Printf("║ Breadcrumbs:                               ║\n")
		fmt.Printf("║   Added:       %-28d║\n", m.BreadcrumbsAdded)
		fmt.Printf("║   Evicted:     %-28d║\n", m.BreadcrumbsEvicted)
		fmt.Printf("╠════════════════════════════════════════════╣\n")
		fmt.Printf("║ Batching:                                  ║\n")
		fmt.Printf("║   Batches:     %-28d║\n", m.BatchesSent)
		fmt.Printf("║   Avg Size:    %-28.2f║\n", m.AverageBatchSize)
		fmt.Printf("╠════════════════════════════════════════════╣\n")
		fmt.Printf("║ Performance:                               ║\n")
		fmt.Printf("║   Last Flush:  %-28v║\n", m.LastFlushDuration)
		fmt.Printf("║   Total Time:  %-28v║\n", m.TotalFlushTime)
		fmt.Printf("╠════════════════════════════════════════════╣\n")
		fmt.Printf("║ Network:                                   ║\n")
		fmt.Printf("║   Retries:     %-28d║\n", m.RetryCount)
		fmt.Printf("║   Errors:      %-28d║\n", m.NetworkErrors)
		fmt.Printf("╚════════════════════════════════════════════╝\n")
		}
	}()

	fmt.Println("Starting metrics monitoring example...")
	fmt.Println("This will generate various log events and display real-time metrics.")

	// Start background workload generator
	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	// Simulate steady state logging
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				// Generate various log levels
				switch rand.Intn(10) {
				case 0:
					logger.Verbose("System heartbeat {Timestamp}", time.Now().Unix())
				case 1, 2:
					logger.Debug("Debug message {Counter}", rand.Intn(1000))
				case 3, 4, 5:
					logger.Information("User action {Action} by {UserId}", 
						"click", fmt.Sprintf("user-%d", rand.Intn(100)))
				case 6, 7:
					logger.Warning("High memory usage: {Percentage}%", 75+rand.Intn(20))
				case 8:
					logger.Error("Request failed: {Error}", 
						errors.New("connection timeout"))
				case 9:
					if rand.Float32() < 0.3 {
						logger.Fatal("Critical error in {Component}", 
							"payment-processor")
					}
				}
			}
		}
	}()

	// Simulate burst traffic
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				// Generate burst of events
				fmt.Println("\n>>> Generating burst traffic...")
				for i := 0; i < 20; i++ {
					logger.Error("Burst error {Index}: {Message}", 
						i, "high load condition")
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
	}()

	// Simulate breadcrumb generation
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				// These create breadcrumbs (below error level)
				logger.Debug("Navigation: {Page}", 
					fmt.Sprintf("/page/%d", rand.Intn(20)))
			}
		}
	}()

	// Collect metrics periodically
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				monitor.Collect()
			}
		}
	}()

	// Run for 30 seconds
	fmt.Println("Monitoring will run for 30 seconds...")
	fmt.Println("Watch the metrics dashboard update in real-time.")
	
	time.Sleep(30 * time.Second)

	// Stop workload generators
	fmt.Println("\nStopping workload generators...")
	close(stopCh)
	wg.Wait()

	// Wait for final flush
	time.Sleep(5 * time.Second)

	// Display final statistics
	finalMetrics := sentrySink.Metrics()
	eventRate, errorRate, retryRate := monitor.GetTrend()
	
	fmt.Printf("\n╔════════════════════════════════════════════╗\n")
	fmt.Printf("║           FINAL STATISTICS                 ║\n")
	fmt.Printf("╠════════════════════════════════════════════╣\n")
	fmt.Printf("║ Total Events Sent:     %-20d║\n", finalMetrics.EventsSent)
	fmt.Printf("║ Total Events Failed:   %-20d║\n", finalMetrics.EventsFailed)
	fmt.Printf("║ Total Breadcrumbs:     %-20d║\n", finalMetrics.BreadcrumbsAdded)
	fmt.Printf("║ Total Batches:         %-20d║\n", finalMetrics.BatchesSent)
	fmt.Printf("╠════════════════════════════════════════════╣\n")
	fmt.Printf("║ Average Event Rate:    %-17.2f/s║\n", eventRate)
	fmt.Printf("║ Error Rate:            %-19.2f%%║\n", errorRate)
	fmt.Printf("║ Retry Rate:            %-19.2f%%║\n", retryRate)
	fmt.Printf("╠════════════════════════════════════════════╣\n")
	fmt.Printf("║ Efficiency Metrics:                        ║\n")
	if finalMetrics.BatchesSent > 0 {
		avgFlushTime := finalMetrics.TotalFlushTime / time.Duration(finalMetrics.BatchesSent)
		fmt.Printf("║ Avg Flush Time:        %-20v║\n", avgFlushTime)
	}
	fmt.Printf("║ Avg Batch Size:        %-20.2f║\n", finalMetrics.AverageBatchSize)
	if finalMetrics.EventsSent > 0 {
		successRate := float64(finalMetrics.EventsSent-finalMetrics.EventsFailed) / 
			float64(finalMetrics.EventsSent) * 100
		fmt.Printf("║ Success Rate:          %-19.2f%%║\n", successRate)
	}
	fmt.Printf("╚════════════════════════════════════════════╝\n")

	fmt.Println("\nMetrics monitoring example completed.")
	fmt.Println("This demonstrates comprehensive metrics collection and monitoring.")
}