package main

import (
	"fmt"
	"net/http"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/middleware"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	// Create mtlog logger with console output
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.DebugLevel),
		mtlog.WithProperty("service", "http-example"),
	)

	// Create middleware options
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

	// Home endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Get logger from context for nested logging
		if logger := middleware.FromContext(r.Context()); logger != nil {
			logger.Debug("Processing home request")
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"message": "Welcome to mtlog HTTP middleware example"}`)
	})

	// User endpoint
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		// Get logger from context
		logger := middleware.FromContext(r.Context())
		requestID := middleware.RequestIDFromContext(r.Context())
		
		if logger != nil {
			logger.Information("Fetching users", "RequestId", requestID)
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]}`)
	})

	// Error endpoint (for testing error logging)
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		logger := middleware.FromContext(r.Context())
		if logger != nil {
			logger.Error("Simulated error occurred")
		}
		
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	})

	// Health check endpoint (skipped by middleware)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Apply middleware to all routes
	handler := mw(mux)

	// Start server
	logger.Information("Starting HTTP server on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		logger.Fatal("Server failed to start", err)
	}
}