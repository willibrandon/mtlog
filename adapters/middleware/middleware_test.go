package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func TestMiddleware(t *testing.T) {
	// Create a memory sink to capture logs
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(memSink))

	tests := []struct {
		name           string
		options        *Options
		method         string
		path           string
		expectedStatus int
		expectedLevel  core.LogEventLevel
		shouldLog      bool
	}{
		{
			name:           "successful request",
			options:        DefaultOptions(logger),
			method:         "GET",
			path:           "/api/users",
			expectedStatus: http.StatusOK,
			expectedLevel:  core.InformationLevel,
			shouldLog:      true,
		},
		{
			name:           "not found request",
			options:        DefaultOptions(logger),
			method:         "GET",
			path:           "/not-found",
			expectedStatus: http.StatusNotFound,
			expectedLevel:  core.WarningLevel,
			shouldLog:      true,
		},
		{
			name:           "server error",
			options:        DefaultOptions(logger),
			method:         "POST",
			path:           "/error",
			expectedStatus: http.StatusInternalServerError,
			expectedLevel:  core.ErrorLevel,
			shouldLog:      true,
		},
		{
			name:           "skip health check",
			options:        DefaultOptions(logger),
			method:         "GET",
			path:           "/health",
			expectedStatus: http.StatusOK,
			expectedLevel:  core.InformationLevel,
			shouldLog:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear memory sink
			memSink.Clear()

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check that logger is in context
				logger := FromContext(r.Context())
				if logger == nil && tt.shouldLog {
					t.Error("Expected logger in context, got nil")
				}

				// Check request ID in context
				requestID := RequestIDFromContext(r.Context())
				if requestID == "" && tt.shouldLog && tt.options.GenerateRequestID {
					t.Error("Expected request ID in context, got empty string")
				}

				// Set appropriate status
				w.WriteHeader(tt.expectedStatus)
				w.Write([]byte("test response"))
			})

			// Apply middleware
			mw := Middleware(tt.options)
			wrappedHandler := mw(handler)

			// Create test request
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			// Execute request
			wrappedHandler.ServeHTTP(rec, req)

			// Check response
			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			// Check logs
			events := memSink.Events()
			if tt.shouldLog {
				if len(events) == 0 {
					t.Error("Expected log event, got none")
				} else {
					event := events[0]
					if event.Level != tt.expectedLevel {
						t.Errorf("Expected log level %v, got %v", tt.expectedLevel, event.Level)
					}

					// Check required properties
					if _, ok := event.Properties["Method"]; !ok {
						t.Error("Expected Method property in log")
					}
					if _, ok := event.Properties["Path"]; !ok {
						t.Error("Expected Path property in log")
					}
					if _, ok := event.Properties["StatusCode"]; !ok {
						t.Error("Expected StatusCode property in log")
					}
					if _, ok := event.Properties["duration_ms"]; !ok {
						t.Error("Expected duration_ms property in log")
					}
				}
			} else {
				if len(events) > 0 {
					t.Errorf("Expected no logs for path %s, got %d", tt.path, len(events))
				}
			}
		})
	}
}

func TestResponseWriter(t *testing.T) {
	t.Run("captures status code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		rw.WriteHeader(http.StatusCreated)
		
		if rw.Status() != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, rw.Status())
		}
	})

	t.Run("captures response size", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		data := []byte("Hello, World!")
		n, err := rw.Write(data)
		
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if n != len(data) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
		}
		if rw.Size() != len(data) {
			t.Errorf("Expected size %d, got %d", len(data), rw.Size())
		}
	})

	t.Run("defaults to 200 OK", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		rw.Write([]byte("test"))
		
		if rw.Status() != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rw.Status())
		}
	})

	t.Run("only writes header once", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := newResponseWriter(rec)

		rw.WriteHeader(http.StatusCreated)
		rw.WriteHeader(http.StatusBadRequest) // Should be ignored
		
		if rw.Status() != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, rw.Status())
		}
	})
}

func TestGetLatencyValue(t *testing.T) {
	tests := []struct {
		name     string
		unit     string
		expected float64
	}{
		{"milliseconds", "ms", 100},
		{"microseconds", "us", 100000},
		{"nanoseconds", "ns", 100000000},
		{"seconds", "s", 0.1},
		{"default to ms", "", 100},
	}

	latency := 100 * time.Millisecond
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := getLatencyValue(latency, tt.unit)
			if value != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, value)
			}
		})
	}
}

func TestFromContext(t *testing.T) {
	logger := mtlog.New()
	
	t.Run("retrieves logger from context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), LoggerContextKey, logger)
		retrieved := FromContext(ctx)
		
		if retrieved == nil {
			t.Error("Expected logger, got nil")
		}
	})

	t.Run("returns nil for missing logger", func(t *testing.T) {
		ctx := context.Background()
		retrieved := FromContext(ctx)
		
		if retrieved != nil {
			t.Error("Expected nil, got logger")
		}
	})
}

func TestRequestIDFromContext(t *testing.T) {
	requestID := "test-request-id"
	
	t.Run("retrieves request ID from context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), RequestIDContextKey, requestID)
		retrieved := RequestIDFromContext(ctx)
		
		if retrieved != requestID {
			t.Errorf("Expected %s, got %s", requestID, retrieved)
		}
	})

	t.Run("returns empty string for missing request ID", func(t *testing.T) {
		ctx := context.Background()
		retrieved := RequestIDFromContext(ctx)
		
		if retrieved != "" {
			t.Errorf("Expected empty string, got %s", retrieved)
		}
	})
}

func TestCustomLevelFunc(t *testing.T) {
	tests := []struct {
		statusCode    int
		expectedLevel core.LogEventLevel
	}{
		{200, core.InformationLevel},
		{201, core.InformationLevel},
		{301, core.InformationLevel},
		{400, core.WarningLevel},
		{404, core.WarningLevel},
		{500, core.ErrorLevel},
		{503, core.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			level := defaultLevelFunc(tt.statusCode)
			if level != tt.expectedLevel {
				t.Errorf("For status %d, expected level %v, got %v", 
					tt.statusCode, tt.expectedLevel, level)
			}
		})
	}
}