package main

import (
	"fmt"
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
		mtlog.WithProperty("service", "advanced-example"),
	)

	// Create middleware with advanced options
	options := &middleware.Options{
		Logger:            logger,
		GenerateRequestID: true,
		RequestIDHeader:   "X-Request-ID",
		SkipPaths:         []string{"/health", "/metrics"},
		RequestFields: []string{
			"method",
			"path",
			"ip",
			"user_agent",
			"proto",
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
			{
				Name: "UserId",
				Extract: func(r *http.Request) any {
					// Extract from a hypothetical auth header
					if auth := r.Header.Get("Authorization"); auth != "" {
						// In real app, you'd parse JWT or session
						return "user-123"
					}
					return nil
				},
			},
			{
				Name: "ApiVersion",
				Extract: func(r *http.Request) any {
					return r.Header.Get("X-API-Version")
				},
			},
		},
		
		// Sampling: Log 50% of requests to /api/status, all others
		Sampler: middleware.NewPathSampler([]middleware.PathSamplingRule{
			{Pattern: "/api/status", Rate: 0.5},
			{Pattern: "*", Rate: 1.0},
		}),
		
		// Panic handler
		PanicHandler: func(w http.ResponseWriter, r *http.Request, err any) {
			logger.Error("Custom panic handler triggered", "Panic", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error": "An unexpected error occurred"}`)
		},
		
		// Timeout handler
		TimeoutHandler: func(r *http.Request, remaining time.Duration) {
			logger.Warning("Request approaching timeout", 
				"Path", r.URL.Path,
				"Remaining", remaining)
		},
		
		CustomLevelFunc: func(statusCode int) core.LogEventLevel {
			switch {
			case statusCode >= 500:
				return core.ErrorLevel
			case statusCode >= 400:
				return core.WarningLevel
			default:
				return core.InformationLevel
			}
		},
	}

	// Create middleware
	mw := middleware.Middleware(options)

	// Create HTTP handlers
	mux := http.NewServeMux()

	// Home endpoint - demonstrates context helpers
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Use context helpers for cleaner code
		middleware.InfoContext(r.Context(), "Processing home request")
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "Advanced middleware example"}`)
	})

	// Login endpoint - demonstrates body logging with sanitization
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Log with context helpers
		middleware.InfoContext(ctx, "Processing login attempt")
		
		// The password in request body will be sanitized in logs
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"token": "abc123", "user": {"id": 123, "name": "Alice"}}`)
	})

	// Status endpoint - demonstrates sampling (only 50% logged)
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "healthy"}`)
	})

	// Panic endpoint - demonstrates panic recovery
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("Intentional panic for testing")
	})

	// Slow endpoint - demonstrates slow request detection
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow processing
		time.Sleep(6 * time.Second)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"result": "finally done"}`)
	})

	// Protected endpoint - demonstrates custom field extraction
	mux.HandleFunc("/api/protected", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		
		// Add more context to the logger
		ctx = middleware.WithFieldsContext(ctx, map[string]any{
			"Operation": "GetUserProfile",
			"Resource":  "users",
		})
		
		middleware.InfoContext(ctx, "Accessing protected resource")
		
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"profile": {"id": 123, "role": "admin"}}`)
	})

	// Health check endpoint (skipped by middleware)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Apply middleware to all routes
	handler := mw(mux)

	// Add timeout middleware for demonstration
	handler = http.TimeoutHandler(handler, 10*time.Second, "Request timeout")

	// Start server
	logger.Information("Starting advanced HTTP server on :8080")
	logger.Information("Try these endpoints:")
	logger.Information("  curl http://localhost:8080/")
	logger.Information("  curl -X POST http://localhost:8080/api/login -d '{\"username\":\"alice\",\"password\":\"secret123\"}'")
	logger.Information("  curl http://localhost:8080/api/status (50% sampling)")
	logger.Information("  curl http://localhost:8080/panic (panic recovery)")
	logger.Information("  curl http://localhost:8080/slow (slow request warning)")
	logger.Information("  curl -H 'Authorization: Bearer token' http://localhost:8080/api/protected")
	
	if err := http.ListenAndServe(":8080", handler); err != nil {
		logger.Fatal("Server failed to start", err)
	}
}
