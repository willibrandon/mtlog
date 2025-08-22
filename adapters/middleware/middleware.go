// Package middleware provides HTTP middleware for automatic request/response logging
// with mtlog. It supports standard net/http and popular web frameworks including
// Gin, Echo, Fiber, and Chi.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/selflog"
)

// contextKey is a type for context keys used by this package
type contextKey string

const (
	// LoggerContextKey is used to store the logger in request context
	LoggerContextKey contextKey = "mtlog.logger"
	// RequestIDContextKey is used to store the request ID in context
	RequestIDContextKey contextKey = "mtlog.request_id"
)

// FieldExtractor extracts custom fields from requests
type FieldExtractor struct {
	Name    string
	Extract func(r *http.Request) any
}

// Options configures the middleware behavior
type Options struct {
	// Logger is the mtlog logger to use
	Logger core.Logger
	
	// RequestFields specifies which request fields to log
	RequestFields []string
	
	// LatencyField specifies the field name for request duration
	LatencyField string
	
	// LatencyUnit specifies the unit for latency (ms, us, ns)
	LatencyUnit string
	
	// RequestIDHeader specifies the header to check for existing request ID
	RequestIDHeader string
	
	// GenerateRequestID enables automatic request ID generation
	GenerateRequestID bool
	
	// SkipPaths specifies paths to skip logging (e.g., health checks)
	SkipPaths []string
	
	// LogRequestBody enables request body logging
	LogRequestBody bool
	
	// LogResponseBody enables response body logging  
	LogResponseBody bool
	
	// MaxBodySize limits body logging size (in bytes)
	MaxBodySize int
	
	// CustomLevelFunc allows custom log level based on status code
	CustomLevelFunc func(statusCode int) core.LogEventLevel
	
	// BodySanitizer sanitizes request/response bodies before logging
	BodySanitizer BodySanitizer
	
	// CustomFields are additional fields to extract from requests
	CustomFields []FieldExtractor
	
	// Sampler determines which requests to log
	Sampler Sampler
	
	// PanicHandler handles panics in the request handler
	PanicHandler func(w http.ResponseWriter, r *http.Request, err any)
	
	// TimeoutHandler is called when a request is close to timing out
	TimeoutHandler func(r *http.Request, remaining time.Duration)
	
	// MetricsRecorder records HTTP metrics
	MetricsRecorder MetricsRecorder
	
	// BeforeRequest is called before processing the request
	BeforeRequest func(r *http.Request, logger core.Logger)
	
	// AfterRequest is called after processing the request
	AfterRequest func(r *http.Request, statusCode int, duration time.Duration, logger core.Logger)
	
	// ErrorHandler handles structured errors
	ErrorHandler ErrorHandler
}

// DefaultOptions returns middleware options with sensible defaults
func DefaultOptions(logger core.Logger) *Options {
	return &Options{
		Logger:            logger,
		RequestFields:     []string{"method", "path", "ip"},
		LatencyField:      "duration_ms",
		LatencyUnit:       "ms",
		RequestIDHeader:   "X-Request-ID",
		GenerateRequestID: true,
		SkipPaths:         []string{"/health", "/healthz", "/ping"},
		LogRequestBody:    false,
		LogResponseBody:   false,
		MaxBodySize:       4096,
		CustomLevelFunc:   defaultLevelFunc,
	}
}

// Validate checks that the options are valid
func (opts *Options) Validate() error {
	if opts.Logger == nil {
		return fmt.Errorf("logger is required")
	}
	
	if opts.MaxBodySize < 0 {
		return fmt.Errorf("MaxBodySize must be non-negative, got %d", opts.MaxBodySize)
	}
	
	if opts.LatencyUnit != "" {
		switch opts.LatencyUnit {
		case "ms", "us", "ns", "s":
			// Valid units
		default:
			return fmt.Errorf("invalid LatencyUnit %q, must be one of: ms, us, ns, s", opts.LatencyUnit)
		}
	}
	
	if opts.LatencyField == "" {
		return fmt.Errorf("LatencyField cannot be empty")
	}
	
	// Validate samplers if present
	if opts.Sampler != nil {
		// Try a test sample to ensure it doesn't panic
		req, _ := http.NewRequest("GET", "/test", nil)
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Sampler panicked, which is invalid
					panic(fmt.Errorf("sampler panicked during validation: %v", r))
				}
			}()
			_ = opts.Sampler.ShouldSample(req)
		}()
	}
	
	// Validate custom field extractors
	for i, extractor := range opts.CustomFields {
		if extractor.Name == "" {
			return fmt.Errorf("CustomFields[%d].Name cannot be empty", i)
		}
		if extractor.Extract == nil {
			return fmt.Errorf("CustomFields[%d].Extract cannot be nil", i)
		}
	}
	
	return nil
}

// defaultLevelFunc returns the log level based on status code
func defaultLevelFunc(statusCode int) core.LogEventLevel {
	switch {
	case statusCode >= 500:
		return core.ErrorLevel
	case statusCode >= 400:
		return core.WarningLevel
	default:
		return core.InformationLevel
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
	size       int
}

// newResponseWriter creates a new response writer wrapper
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

// Write captures the response size
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// Status returns the captured status code
func (rw *responseWriter) Status() int {
	return rw.statusCode
}

// Size returns the response size in bytes
func (rw *responseWriter) Size() int {
	return rw.size
}

// Middleware creates a standard net/http middleware with the given options
func Middleware(opts *Options) func(http.Handler) http.Handler {
	if opts == nil {
		panic("middleware: options cannot be nil")
	}
	
	// Validate options
	if err := opts.Validate(); err != nil {
		panic(fmt.Sprintf("middleware: invalid options: %v", err))
	}
	
	// Set defaults for optional fields
	if opts.CustomLevelFunc == nil {
		opts.CustomLevelFunc = defaultLevelFunc
	}
	if opts.Sampler == nil {
		opts.Sampler = &AlwaysSampler{}
	}
	if opts.MaxBodySize == 0 {
		opts.MaxBodySize = 4096
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if we should skip this path
			for _, skip := range opts.SkipPaths {
				if r.URL.Path == skip {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check sampling
			if opts.Sampler != nil && !opts.Sampler.ShouldSample(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Start timing
			start := time.Now()
			
			// Call BeforeRequest interceptor if configured
			if opts.BeforeRequest != nil {
				opts.BeforeRequest(r, opts.Logger)
			}
			
			// Check for approaching timeout/deadline
			if opts.TimeoutHandler != nil {
				if deadline, ok := r.Context().Deadline(); ok {
					remaining := time.Until(deadline)
					if remaining < 5*time.Second && remaining > 0 {
						opts.TimeoutHandler(r, remaining)
					}
				}
			}

			// Get or generate request ID
			requestID := r.Header.Get(opts.RequestIDHeader)
			if requestID == "" && opts.GenerateRequestID {
				requestID = uuid.New().String()
			}

			// Create logger with request ID
			logger := opts.Logger
			if requestID != "" {
				logger = logger.With("RequestId", requestID)
				// Add request ID to response header
				w.Header().Set(opts.RequestIDHeader, requestID)
			}

			// Capture request body if configured
			var requestBody []byte
			var requestBodyErr error
			if opts.LogRequestBody {
				requestBody, requestBodyErr = captureRequestBody(r, opts.MaxBodySize, opts.BodySanitizer)
				if requestBodyErr != nil && selflog.IsEnabled() {
					selflog.Printf("[middleware] failed to capture request body: %v (path=%s)", requestBodyErr, r.URL.Path)
				}
			}

			// Inject logger and request ID into context
			ctx := r.Context()
			ctx = context.WithValue(ctx, LoggerContextKey, logger)
			ctx = context.WithValue(ctx, RequestIDContextKey, requestID)
			r = r.WithContext(ctx)

			// Wrap response writer to capture status code and optionally body
			var rw http.ResponseWriter
			var respRecorder *limitedResponseRecorder
			if opts.LogResponseBody {
				if EnablePooling {
					respRecorder = getLimitedResponseRecorder(w, opts.MaxBodySize)
				} else {
					respRecorder = newLimitedResponseRecorder(w, opts.MaxBodySize)
				}
				rw = respRecorder
			} else {
				if EnablePooling {
					rw = getResponseWriter(w)
				} else {
					rw = newResponseWriter(w)
				}
			}

			// Panic recovery
			var panicErr any
			func() {
				defer func() {
					if err := recover(); err != nil {
						panicErr = err
						
						// Record panic metric
						if opts.MetricsRecorder != nil {
							opts.MetricsRecorder.RecordPanic(r.Method, r.URL.Path)
						}
						
						// Log the panic via selflog for debugging
						if selflog.IsEnabled() {
							selflog.Printf("[middleware] panic in handler: %v (path=%s)", err, r.URL.Path)
						}
						
						// Log the panic via the logger for application visibility
						logger.Error("Panic in handler: {Error}", err)
						
						// Call custom panic handler if configured
						if opts.PanicHandler != nil {
							opts.PanicHandler(w, r, err)
						} else {
							// Default panic response
							if statusWriter, ok := rw.(*responseWriter); ok && !statusWriter.written {
								http.Error(w, "Internal Server Error", http.StatusInternalServerError)
							}
						}
					}
				}()
				
				// Process request
				next.ServeHTTP(rw, r)
			}()

			// Calculate latency
			latency := time.Since(start)
			latencyValue := getLatencyValue(latency, opts.LatencyUnit)

			// Get status code
			statusCode := http.StatusOK
			if statusWriter, ok := rw.(*responseWriter); ok {
				statusCode = statusWriter.Status()
			} else if respRecorder != nil {
				statusCode = respRecorder.Status()
			}

			// Override status if there was a panic
			if panicErr != nil {
				statusCode = http.StatusInternalServerError
			}

			// Build positional arguments for the template
			args := []any{
				r.Method,
				r.URL.Path,
				statusCode,
				latencyValue,
			}

			// Add additional properties using With()
			enrichedLogger := logger
			for _, field := range opts.RequestFields {
				switch field {
				case "method":
					enrichedLogger = enrichedLogger.With("Method", r.Method)
				case "path":
					enrichedLogger = enrichedLogger.With("Path", r.URL.Path)
				case "ip":
					enrichedLogger = enrichedLogger.With("IP", getClientIP(r))
				case "user_agent":
					enrichedLogger = enrichedLogger.With("UserAgent", r.UserAgent())
				case "referer":
					enrichedLogger = enrichedLogger.With("Referer", r.Referer())
				case "proto":
					enrichedLogger = enrichedLogger.With("Proto", r.Proto)
				case "host":
					enrichedLogger = enrichedLogger.With("Host", r.Host)
				}
			}

			// Add custom field extractors
			for _, extractor := range opts.CustomFields {
				if value := extractor.Extract(r); value != nil {
					enrichedLogger = enrichedLogger.With(extractor.Name, value)
				}
			}
			
			// Add standard properties
			responseSize := 0
			if statusWriter, ok := rw.(*responseWriter); ok {
				responseSize = statusWriter.Size()
			} else if respRecorder != nil {
				responseSize = len(respRecorder.Body())
			}
			
			enrichedLogger = enrichedLogger.With("StatusCode", statusCode).
				With("Size", responseSize).
				With(opts.LatencyField, latencyValue)

			// Add request body if captured
			if opts.LogRequestBody && requestBodyErr == nil && len(requestBody) > 0 {
				enrichedLogger = enrichedLogger.With("RequestBody", string(requestBody))
				if opts.MetricsRecorder != nil {
					opts.MetricsRecorder.RecordBodySize("request", len(requestBody))
				}
			}

			// Add response body if captured
			if opts.LogResponseBody && respRecorder != nil {
				body := respRecorder.Body()
				if opts.BodySanitizer != nil {
					contentType := w.Header().Get("Content-Type")
					body = opts.BodySanitizer(contentType, body)
				}
				if len(body) > 0 {
					enrichedLogger = enrichedLogger.With("ResponseBody", string(body))
					if opts.MetricsRecorder != nil {
						opts.MetricsRecorder.RecordBodySize("response", len(body))
					}
				}
			}

			// Add panic info if there was one
			if panicErr != nil {
				enrichedLogger = enrichedLogger.With("Panic", fmt.Sprintf("%v", panicErr))
			}

			// Determine log level
			level := opts.CustomLevelFunc(statusCode)
			
			// Check if request was slow (log slow requests at warning level)
			if latency > 5*time.Second && level < core.WarningLevel {
				level = core.WarningLevel
				enrichedLogger = enrichedLogger.With("SlowRequest", true)
			}

			// Log the request with positional args for template
			template := "HTTP {Method} {Path} responded {StatusCode} in {" + opts.LatencyField + "}" + opts.LatencyUnit
			
			switch level {
			case core.ErrorLevel:
				enrichedLogger.Error(template, args...)
			case core.WarningLevel:
				enrichedLogger.Warning(template, args...)
			default:
				enrichedLogger.Information(template, args...)
			}
			
			// Record metrics
			if opts.MetricsRecorder != nil {
				opts.MetricsRecorder.RecordRequest(r.Method, r.URL.Path, statusCode, latency)
			}
			
			// Call AfterRequest interceptor if configured
			if opts.AfterRequest != nil {
				opts.AfterRequest(r, statusCode, latency, enrichedLogger)
			}
			
			// Return response writers to pool
			if EnablePooling {
				if respRecorder != nil {
					putLimitedResponseRecorder(respRecorder)
				} else if rw, ok := rw.(*responseWriter); ok {
					putResponseWriter(rw)
				}
			}
		})
	}
}

// FromContext retrieves the logger from the request context
func FromContext(ctx context.Context) core.Logger {
	if logger, ok := ctx.Value(LoggerContextKey).(core.Logger); ok {
		return logger
	}
	return nil
}

// RequestIDFromContext retrieves the request ID from the context
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDContextKey).(string); ok {
		return id
	}
	return ""
}

// getLatencyValue converts latency to the specified unit
func getLatencyValue(latency time.Duration, unit string) float64 {
	switch unit {
	case "us":
		return float64(latency.Microseconds())
	case "ns":
		return float64(latency.Nanoseconds())
	case "s":
		return latency.Seconds()
	default: // ms
		return float64(latency.Milliseconds())
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		if idx := len(xff) - 1; idx >= 0 {
			for i := idx; i >= 0; i-- {
				if xff[i] == ',' || xff[i] == ' ' {
					return xff[i+1:]
				}
			}
			return xff
		}
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}