package main

import (
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	// Create logger with sampling
	logger := mtlog.New(mtlog.WithConsole())
	
	// Example 1: Basic Sampling Stats
	basicSamplingExample(logger)
	
	// Example 2: Detailed Metrics with String() formatting
	detailedMetricsExample()
	
	// Example 3: Prometheus Metrics Export
	prometheusExample()
	
	// Example 4: Monitoring with Periodic Collection
	monitoringExample(logger)
}

func basicSamplingExample(logger core.Logger) {
	fmt.Println("=== Basic Sampling Stats ===")
	
	// Create sampled logger
	sampledLogger := logger.Sample(10) // Every 10th message
	
	// Log messages
	for i := 0; i < 100; i++ {
		sampledLogger.Info("Processing item {Index}", i)
	}
	
	// Get and display stats
	sampled, skipped := sampledLogger.GetSamplingStats()
	fmt.Printf("Sampled: %d, Skipped: %d\n", sampled, skipped)
	fmt.Printf("Sampling rate: %.1f%%\n\n", float64(sampled)*100/float64(sampled+skipped))
}

func detailedMetricsExample() {
	fmt.Println("=== Detailed Metrics with Formatting ===")
	
	// Create sample metrics
	metrics := core.SamplingMetrics{
		TotalSampled:          1000,
		TotalSkipped:          9000,
		GroupCacheHits:        800,
		GroupCacheMisses:      200,
		GroupCacheSize:        50,
		GroupCacheEvictions:   10,
		BackoffCacheHits:      450,
		BackoffCacheMisses:    50,
		BackoffCacheSize:      30,
		BackoffCacheEvictions: 5,
		AdaptiveCacheHits:     300,
		AdaptiveCacheMisses:   100,
		AdaptiveCacheSize:     20,
	}
	
	// Display with different format verbs
	fmt.Println("String() output:")
	fmt.Println(metrics.String())
	
	fmt.Println("\nVerbose format:")
	fmt.Printf("%+v\n", metrics)
	
	fmt.Println("\nGo syntax:")
	fmt.Printf("%#v\n\n", metrics)
}

func prometheusExample() {
	fmt.Println("=== Prometheus Metrics Export ===")
	
	// Create sample metrics
	metrics := core.SamplingMetrics{
		TotalSampled:        5000,
		TotalSkipped:        45000,
		GroupCacheHits:      4000,
		GroupCacheMisses:    1000,
		GroupCacheSize:      100,
		BackoffCacheHits:    900,
		BackoffCacheMisses:  100,
		BackoffCacheSize:    50,
		AdaptiveCacheHits:   750,
		AdaptiveCacheMisses: 250,
		AdaptiveCacheSize:   40,
	}
	
	// Convert to Prometheus metrics
	promMetrics := metrics.PrometheusMetrics()
	
	// Display key metrics
	fmt.Printf("mtlog_sampling_rate: %.4f\n", promMetrics["mtlog_sampling_rate"])
	fmt.Printf("mtlog_sampling_total_sampled: %.0f\n", promMetrics["mtlog_sampling_total_sampled"])
	fmt.Printf("mtlog_sampling_total_skipped: %.0f\n", promMetrics["mtlog_sampling_total_skipped"])
	fmt.Printf("mtlog_sampling_group_cache_hit_rate: %.2f\n", promMetrics["mtlog_sampling_group_cache_hit_rate"])
	fmt.Printf("mtlog_sampling_backoff_cache_hit_rate: %.2f\n", promMetrics["mtlog_sampling_backoff_cache_hit_rate"])
	fmt.Printf("mtlog_sampling_adaptive_cache_hit_rate: %.2f\n\n", promMetrics["mtlog_sampling_adaptive_cache_hit_rate"])
	
	// Example HTTP endpoint for Prometheus scraping
	fmt.Println("Example HTTP endpoint (not started):")
	fmt.Printf("%s\n", `
http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
    for name, value := range metrics.PrometheusMetrics() {
        fmt.Fprintf(w, "%s %f\n", name, value)
    }
})`)
}

func monitoringExample(logger core.Logger) {
	fmt.Println("\n=== Monitoring with Periodic Collection ===")
	
	// Create sampled logger
	sampledLogger := logger.Sample(5)
	
	// Simulate logging with monitoring
	ticker := time.NewTicker(1 * time.Second)
	done := time.After(5 * time.Second)
	
	fmt.Println("Monitoring for 5 seconds...")
	
	messageCount := 0
loop:
	for {
		select {
		case <-ticker.C:
			// Log some messages
			for i := 0; i < 20; i++ {
				sampledLogger.Info("Periodic message {Count}", messageCount)
				messageCount++
			}
			
			// Collect and display metrics
			sampled, skipped := sampledLogger.GetSamplingStats()
			total := sampled + skipped
			if total > 0 {
				rate := float64(sampled) * 100 / float64(total)
				fmt.Printf("Metrics: Sampled=%d, Skipped=%d, Rate=%.1f%%\n", sampled, skipped, rate)
				
				// Example alert condition
				if rate < 15.0 {
					fmt.Printf("⚠️  Alert: Sampling rate (%.1f%%) below threshold (15%%)\n", rate)
				}
			}
			
		case <-done:
			ticker.Stop()
			break loop
		}
	}
	
	fmt.Println("\nMonitoring complete!")
}