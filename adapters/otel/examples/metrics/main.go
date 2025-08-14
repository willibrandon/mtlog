package main

import (
	"context"
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	otelmtlog "github.com/willibrandon/mtlog/adapters/otel"
	"github.com/willibrandon/mtlog/core"
)

// testConsoleSink is a simple sink for testing
type testConsoleSink struct{}

func (s *testConsoleSink) Emit(event *core.LogEvent) {
	fmt.Printf("[%d] %s\n", int(event.Level), event.MessageTemplate)
}

func (s *testConsoleSink) Close() error {
	return nil
}

func main() {
	fmt.Println("OpenTelemetry Metrics Examples")
	fmt.Println("==============================")
	fmt.Println("Prometheus metrics available at http://localhost:9090/metrics")

	ctx := context.Background()

	// Example 1: Basic metrics export
	example1BasicMetrics(ctx)
	
	// Example 2: Metrics with custom sink wrapper
	example2MetricsSink(ctx)
	
	// Example 3: OTLP sink with integrated metrics
	example3OTLPWithMetrics(ctx)
	
	fmt.Println("\n✅ Metrics examples completed!")
	fmt.Println("Check http://localhost:9090/metrics for Prometheus metrics")
	fmt.Println("Press Ctrl+C to exit")
	
	// Keep running to allow metric scraping
	select {}
}

func example1BasicMetrics(ctx context.Context) {
	fmt.Println("\n=== Example 1: Basic Metrics Export ===")
	
	// Create metrics exporter
	metricsExporter, err := otelmtlog.NewMetricsExporter(
		otelmtlog.WithMetricsPort(9090),
		otelmtlog.WithMetricsPath("/metrics"),
	)
	if err != nil {
		panic(err)
	}
	
	// Create logger with console output
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithConsole(),
	)
	
	// Generate events at different levels
	levels := []struct {
		level core.LogEventLevel
		count int
	}{
		{core.VerboseLevel, 5},
		{core.DebugLevel, 10},
		{core.InformationLevel, 20},
		{core.WarningLevel, 8},
		{core.ErrorLevel, 3},
		{core.FatalLevel, 1},
	}
	
	for _, l := range levels {
		for i := 0; i < l.count; i++ {
			event := &core.LogEvent{
				Level:           l.level,
				MessageTemplate: fmt.Sprintf("Level%d message {Index}", int(l.level)),
				Timestamp:       time.Now(),
				Properties: map[string]any{
					"Index": i,
					"trace.id": fmt.Sprintf("trace-%d", i),
				},
			}
			
			// Log the event normally
			start := time.Now()
			switch l.level {
			case core.VerboseLevel:
				logger.Verbose(event.MessageTemplate, i)
			case core.DebugLevel:
				logger.Debug(event.MessageTemplate, i)
			case core.InformationLevel:
				logger.Information(event.MessageTemplate, i)
			case core.WarningLevel:
				logger.Warning(event.MessageTemplate, i)
			case core.ErrorLevel:
				logger.Error(event.MessageTemplate, i)
			case core.FatalLevel:
				logger.Fatal(event.MessageTemplate, i)
			}
			latency := float64(time.Since(start).Microseconds()) / 1000.0
			
			metricsExporter.RecordEvent(event, latency)
			
			time.Sleep(10 * time.Millisecond) // Spread out events
		}
	}
	
	// Record some dropped events
	for i := 0; i < 5; i++ {
		metricsExporter.RecordDropped("queue_full")
	}
	
	fmt.Printf("Generated events: %d verbose, %d debug, %d info, %d warning, %d error, %d fatal\n",
		levels[0].count, levels[1].count, levels[2].count, 
		levels[3].count, levels[4].count, levels[5].count)
	fmt.Println("Recorded 5 dropped events")
}

func example2MetricsSink(ctx context.Context) {
	fmt.Println("\n=== Example 2: Metrics Sink Wrapper ===")
	
	// Create metrics exporter (reuse port 9090)
	metricsExporter, err := otelmtlog.NewMetricsExporter(
		otelmtlog.WithMetricsPort(9091), // Different port to avoid conflict
	)
	if err != nil {
		panic(err)
	}
	
	// Create a simple console sink for testing
	consoleSink := &testConsoleSink{}
	
	// Wrap with metrics recording
	metricsSink := otelmtlog.NewMetricsSink(consoleSink, metricsExporter)
	
	// Create logger using the metrics sink
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(metricsSink),
		mtlog.WithProperty("example", "metrics_sink"),
	)
	
	// Generate some activity
	for i := 0; i < 20; i++ {
		switch i % 4 {
		case 0:
			logger.Information("Processing request {RequestId}", i)
		case 1:
			logger.Debug("Cache hit for key {Key}", fmt.Sprintf("key-%d", i))
		case 2:
			logger.Warning("Slow query detected {Duration}ms", (i%5+1)*100)
		case 3:
			if i%8 == 3 {
				logger.Error("Database connection failed for attempt {Attempt}", i)
			} else {
				logger.Information("Request completed successfully")
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	fmt.Println("Generated 20 events through metrics sink")
	fmt.Println("Additional metrics available at http://localhost:9091/metrics")
}

func example3OTLPWithMetrics(ctx context.Context) {
	fmt.Println("\n=== Example 3: OTLP Sink with Integrated Metrics ===")
	
	// Create OTLP sink with integrated Prometheus metrics
	sink, err := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint("localhost:4317"),
		otelmtlog.WithOTLPBatching(10, 2*time.Second),
		otelmtlog.WithPrometheusMetrics(9092), // Adds metrics export on port 9092
	)
	if err != nil {
		// If OTLP fails, continue with console only
		fmt.Printf("OTLP sink creation failed: %v (continuing with console)\n", err)
	}
	
	// Create logger
	var opts []mtlog.Option
	opts = append(opts, otelmtlog.WithOTELEnricher(ctx))
	if sink != nil {
		opts = append(opts, mtlog.WithSink(sink))
	}
	opts = append(opts, mtlog.WithConsole())
	opts = append(opts, mtlog.WithProperty("example", "otlp_metrics"))
	
	logger := mtlog.New(opts...)
	
	// Simulate application workload with different patterns
	workloadPatterns := []struct {
		name     string
		duration time.Duration
		rate     time.Duration
		errorPct int
	}{
		{"startup", 3 * time.Second, 100 * time.Millisecond, 0},
		{"normal_load", 5 * time.Second, 200 * time.Millisecond, 5},
		{"high_load", 4 * time.Second, 50 * time.Millisecond, 15},
		{"error_spike", 2 * time.Second, 100 * time.Millisecond, 50},
	}
	
	for _, pattern := range workloadPatterns {
		fmt.Printf("Simulating %s pattern...\n", pattern.name)
		
		start := time.Now()
		counter := 0
		
		for time.Since(start) < pattern.duration {
			counter++
			
			// Determine if this should be an error
			isError := (counter*100/pattern.errorPct) > 0 && counter%pattern.errorPct == 0
			
			if isError && pattern.errorPct > 0 {
				logger.Error("Operation failed {Pattern} {Counter} {Error}", 
					pattern.name, counter, "timeout")
			} else {
				switch counter % 3 {
				case 0:
					logger.Information("Processing {Pattern} operation {Counter}", 
						pattern.name, counter)
				case 1:
					logger.Debug("Cache operation {Pattern} {Counter}", 
						pattern.name, counter)
				case 2:
					logger.Warning("Slow operation detected {Pattern} {Counter} {Duration}ms", 
						pattern.name, counter, (counter%10+1)*50)
				}
			}
			
			time.Sleep(pattern.rate)
		}
		
		fmt.Printf("Completed %s: %d events\n", pattern.name, counter)
	}
	
	fmt.Println("OTLP metrics available at http://localhost:9092/metrics")
	
	// Cleanup
	if sink != nil {
		sink.Close()
	}
}