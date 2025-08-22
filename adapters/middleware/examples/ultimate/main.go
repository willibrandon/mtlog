package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/middleware"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	// Create mtlog logger with console output
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.DebugLevel),
		mtlog.WithProperty("service", "ultimate-middleware-demo"),
		mtlog.WithProperty("version", "1.0.0"),
	)

	// 1. HEALTH CHECK HANDLER - Dedicated health check with metrics
	healthHandler := middleware.NewHealthCheckHandler(logger).
		WithVersion("1.0.0").
		WithEnvironment("development").
		WithMetrics(true).
		AddCheck("database", func() middleware.Check {
			// Simulate database check
			return middleware.Check{
				Status:  "healthy",
				Message: "Database connection OK",
			}
		}).
		AddCheck("cache", func() middleware.Check {
			// Simulate cache check
			return middleware.Check{
				Status:  "healthy",
				Message: "Redis connection OK",
			}
		})

	// 2. DYNAMIC SAMPLING - Runtime adjustable sampling rules
	dynamicSampler := middleware.NewDynamicPathSampler([]middleware.PathSamplingRule{
		{Pattern: "/health", Rate: 0.0},        // Never log health checks
		{Pattern: "/metrics", Rate: 0.0},       // Never log metrics
		{Pattern: "/api/debug/*", Rate: 1.0},   // Always log debug endpoints
		{Pattern: "/api/status", Rate: 0.1},    // Log 10% of status checks
		{Pattern: "*", Rate: 0.5},              // Log 50% of everything else
	})

	// Set up change handler for sampling rule updates
	dynamicSampler.SetOnChange(func(oldRules, newRules []middleware.PathSamplingRule) {
		logger.Information("Sampling rules updated",
			"OldRuleCount", len(oldRules),
			"NewRuleCount", len(newRules))
	})

	// 3. BATCH METRICS RECORDER - High-throughput metrics batching
	batchMetrics := middleware.NewBatchMetricsRecorder(
		func(metrics []middleware.RequestMetric) {
			// In production, send to your metrics backend
			logger.Debug("Flushing batch of metrics",
				"Count", len(metrics),
				"FirstTimestamp", metrics[0].Timestamp,
			)
		},
		5*time.Second, // Flush every 5 seconds
		100,           // or when batch reaches 100 metrics
	)
	defer batchMetrics.Close()

	// 4. STRUCTURED ERROR HANDLER
	errorHandler := func(err *middleware.MiddlewareError) (int, interface{}) {
		// Log the structured error
		logger.Error("Middleware error occurred",
			"Type", string(err.Type),
			"Message", err.Message,
			"RequestID", err.RequestID,
			"Path", err.Path,
		)

		// Return appropriate response
		return middleware.DevelopmentErrorHandler(err)
	}

	// 5. REQUEST/RESPONSE INTERCEPTORS
	beforeRequest := func(r *http.Request, log core.Logger) {
		log.Debug("Before request interceptor",
			"Headers", len(r.Header),
			"ContentLength", r.ContentLength,
		)
	}

	afterRequest := func(r *http.Request, statusCode int, duration time.Duration, log core.Logger) {
		// Custom business logic after request
		if duration > 1*time.Second {
			log.Warning("Slow request detected",
				"Duration", duration,
				"Threshold", "1s",
			)
		}

		// Update health check counters
		healthHandler.IncrementRequestCount()
		if statusCode >= 500 {
			healthHandler.IncrementErrorCount()
		}
	}

	// 6. MIDDLEWARE OPTIONS with all features
	middlewareOptions := &middleware.Options{
		Logger:            logger,
		GenerateRequestID: true,
		RequestIDHeader:   "X-Request-ID",
		SkipPaths:         []string{"/health", "/metrics", "/favicon.ico"},
		RequestFields: []string{
			"method", "path", "ip", "user_agent", "proto",
		},
		LatencyField: "duration_ms",
		LatencyUnit:  "ms",

		// Body logging with sanitization
		LogRequestBody:  true,
		LogResponseBody: true,
		MaxBodySize:     2048,
		BodySanitizer:   middleware.DefaultBodySanitizer,

		// Custom field extractors
		CustomFields: []middleware.FieldExtractor{
			middleware.UserIDFromHeader,
			middleware.APIVersionFromHeader,
			middleware.TenantIDFromSubdomain,
			middleware.DeviceTypeFromUserAgent,
		},

		// Dynamic sampling
		Sampler: dynamicSampler,

		// Metrics recording
		MetricsRecorder: batchMetrics,

		// Interceptors
		BeforeRequest: beforeRequest,
		AfterRequest:  afterRequest,

		// Error handling
		ErrorHandler: errorHandler,

		// Panic handler with structured error
		PanicHandler: func(w http.ResponseWriter, r *http.Request, err any) {
			panicErr := middleware.NewPanicError(err, r.Method, r.URL.Path, 
				middleware.RequestIDFromContext(r.Context())).
				WithStackTrace()
			
			statusCode, response := errorHandler(panicErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(response)
		},

		// Timeout handler
		TimeoutHandler: func(r *http.Request, remaining time.Duration) {
			logger.Warning("Request approaching timeout",
				"Path", r.URL.Path,
				"Remaining", remaining,
			)
		},
	}

	// Create main middleware
	loggingMiddleware := middleware.Middleware(middlewareOptions)

	// 7. CORRELATION/TRACING MIDDLEWARE
	correlationMiddleware := func(next http.Handler) http.Handler {
		return middleware.PropagateTraceContext(next, middleware.CorrelationOptions{
			GenerateTraceID:     true,
			GenerateSpanID:      true,
			PropagateDownstream: true,
			HeaderFormat:        middleware.FormatW3C, // W3C Trace Context
			Logger:              logger,
			ExtractBaggage:      true,
			BaggagePrefix:       "X-Baggage-",
		})
	}

	// Create router
	mux := http.NewServeMux()

	// Health endpoints
	mux.Handle("/health", healthHandler)
	mux.HandleFunc("/liveness", middleware.LivenessHandler())
	mux.HandleFunc("/readiness", middleware.ReadinessHandler(
		middleware.DatabaseHealthChecker("postgres", func() error { return nil }),
		middleware.HTTPHealthChecker("upstream", "http://example.com", 5*time.Second),
	))

	// Main application endpoints
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		traceCtx := middleware.GetTraceContext(ctx)
		
		middleware.InfoContext(ctx, "Home endpoint accessed",
			"TraceID", traceCtx.TraceID,
			"SpanID", traceCtx.SpanID,
		)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Ultimate middleware demo",
			"features": []string{
				"Structured errors",
				"Request/response interceptors",
				"Batch metrics",
				"Dynamic sampling",
				"Health checks",
				"Distributed tracing",
			},
			"trace_id": traceCtx.TraceID,
		})
	})

	// API endpoint with authentication simulation
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Simulate authentication check
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			err := middleware.NewValidationError(
				"Authorization required",
				map[string]string{"header": "Authorization"},
			).WithRequestInfo(r.Method, r.URL.Path, middleware.RequestIDFromContext(ctx))
			
			statusCode, response := errorHandler(err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(response)
			return
		}

		// Use request logger helper
		reqLogger := middleware.GetRequestLogger(r).
			WithUser("user-123").
			WithOperation("GetData").
			WithResource("Data", "data-456")

		reqLogger.Information("Fetching data")

		// Simulate data processing
		time.Sleep(100 * time.Millisecond)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": 1, "value": "Alpha"},
				{"id": 2, "value": "Beta"},
			},
			"timestamp": time.Now(),
		})
	})

	// Admin endpoint to update sampling rules
	mux.HandleFunc("/admin/sampling", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		switch r.Method {
		case http.MethodGet:
			// Return current sampling rules
			rules := dynamicSampler.GetRules()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rules)

		case http.MethodPost:
			// Update sampling rules
			var newRules []middleware.PathSamplingRule
			if err := json.NewDecoder(r.Body).Decode(&newRules); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			dynamicSampler.UpdateRules(newRules)
			middleware.InfoContext(ctx, "Sampling rules updated",
				"NewRuleCount", len(newRules),
			)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": "Sampling rules updated",
				"rules":   newRules,
			})

		case http.MethodPatch:
			// Update specific rule rate
			var update struct {
				Pattern string  `json:"pattern"`
				Rate    float64 `json:"rate"`
			}
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if dynamicSampler.UpdateRuleRate(update.Pattern, update.Rate) {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"message": "Rule updated",
					"pattern": update.Pattern,
					"rate":    update.Rate,
				})
			} else {
				http.Error(w, "Rule not found", http.StatusNotFound)
			}

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Metrics endpoint
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		batchSize, lastFlush := batchMetrics.Stats()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"batch_size":  batchSize,
			"last_flush":  lastFlush,
			"uptime":      time.Since(time.Now()).String(),
		})
	})

	// Panic test endpoint
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("Intentional panic for testing structured error handling")
	})

	// Slow endpoint for timeout testing
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		select {
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Finally done"))
		case <-ctx.Done():
			middleware.ErrorContext(ctx, "Request cancelled")
			return
		}
	})

	// Downstream service call example
	mux.HandleFunc("/api/downstream", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Create tracing client for downstream calls
		client := middleware.NewTracingClient(middleware.FormatW3C)
		
		// Make downstream request with trace context
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com/api", nil)
		
		// The TraceRoundTripper automatically adds trace headers
		resp, err := client.Do(req)
		if err != nil {
			middleware.ErrorContext(ctx, "Downstream call failed: {Error}", err)
			http.Error(w, "Downstream service unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		middleware.InfoContext(ctx, "Downstream call completed",
			"Status", resp.StatusCode,
		)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Downstream call completed",
			"status":  resp.StatusCode,
		})
	})

	// Chain all middleware
	// Apply middleware directly to mux for proper panic recovery
	handler := correlationMiddleware(loggingMiddleware(mux))

	// Start server
	logger.Information("Starting Ultimate Middleware Demo on :8080")
	logger.Information("Features demonstrated:")
	logger.Information("  • Structured error types with stack traces")
	logger.Information("  • Request/response interceptors for custom logic")
	logger.Information("  • Batch metrics recording for high throughput")
	logger.Information("  • Dynamic sampling with runtime updates")
	logger.Information("  • Dedicated health check handlers")
	logger.Information("  • Distributed tracing with W3C Trace Context")
	logger.Information("")
	logger.Information("Try these endpoints:")
	logger.Information("  GET  /                    - Home (with tracing)")
	logger.Information("  GET  /health              - Comprehensive health check")
	logger.Information("  GET  /api/data            - Protected endpoint (needs Authorization header)")
	logger.Information("  GET  /admin/sampling      - View current sampling rules")
	logger.Information("  POST /admin/sampling      - Update sampling rules")
	logger.Information("  GET  /panic               - Test panic recovery")
	logger.Information("  GET  /slow                - Test timeout handling")
	logger.Information("  GET  /api/downstream      - Test distributed tracing")
	
	if err := http.ListenAndServe(":8080", handler); err != nil {
		logger.Fatal("Server failed to start: {Error}", err)
	}
}