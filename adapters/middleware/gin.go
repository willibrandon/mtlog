package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/willibrandon/mtlog/core"
)

// Gin creates a Gin middleware for request logging
func Gin(logger core.Logger, opts ...*Options) gin.HandlerFunc {
	var options *Options
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	} else {
		options = DefaultOptions(logger)
	}
	
	// Ensure logger is set
	if options.Logger == nil {
		options.Logger = logger
	}
	
	// Ensure CustomLevelFunc is set
	if options.CustomLevelFunc == nil {
		options.CustomLevelFunc = defaultLevelFunc
	}

	return func(c *gin.Context) {
		// Check if we should skip this path
		for _, skip := range options.SkipPaths {
			if c.Request.URL.Path == skip {
				c.Next()
				return
			}
		}

		// Start timing
		start := time.Now()

		// Get or generate request ID
		requestID := c.GetHeader(options.RequestIDHeader)
		if requestID == "" && options.GenerateRequestID {
			requestID = uuid.New().String()
		}

		// Create logger with request ID
		logger := options.Logger
		if requestID != "" {
			logger = logger.With("RequestId", requestID)
			// Add request ID to response header
			c.Header(options.RequestIDHeader, requestID)
		}

		// Store logger in Gin context
		c.Set("logger", logger)
		c.Set("request_id", requestID)

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		latencyValue := getLatencyValue(latency, options.LatencyUnit)

		// Build positional arguments for the template
		statusCode := c.Writer.Status()
		args := []any{
			c.Request.Method,   // {Method}
			c.Request.URL.Path, // {Path}
			statusCode,         // {StatusCode}
			latencyValue,       // {LatencyField}
		}

		// Add additional properties using With()
		enrichedLogger := logger
		for _, field := range options.RequestFields {
			switch field {
			case "method":
				enrichedLogger = enrichedLogger.With("Method", c.Request.Method)
			case "path":
				enrichedLogger = enrichedLogger.With("Path", c.Request.URL.Path)
			case "ip":
				enrichedLogger = enrichedLogger.With("IP", c.ClientIP())
			case "user_agent":
				enrichedLogger = enrichedLogger.With("UserAgent", c.Request.UserAgent())
			case "referer":
				enrichedLogger = enrichedLogger.With("Referer", c.Request.Referer())
			case "proto":
				enrichedLogger = enrichedLogger.With("Proto", c.Request.Proto)
			case "host":
				enrichedLogger = enrichedLogger.With("Host", c.Request.Host)
			}
		}
		
		// Add standard properties
		enrichedLogger = enrichedLogger.With("StatusCode", statusCode).
			With("Size", c.Writer.Size()).
			With(options.LatencyField, latencyValue)

		// Add error if present
		if len(c.Errors) > 0 {
			enrichedLogger = enrichedLogger.With("Error", c.Errors.String())
		}

		// Determine log level
		level := options.CustomLevelFunc(statusCode)

		// Log the request with positional args for template
		template := "HTTP {Method} {Path} responded {StatusCode} in {" + options.LatencyField + "}" + options.LatencyUnit
		
		switch level {
		case core.ErrorLevel:
			enrichedLogger.Error(template, args...)
		case core.WarningLevel:
			enrichedLogger.Warning(template, args...)
		default:
			enrichedLogger.Information(template, args...)
		}
	}
}

// LoggerFromGinContext retrieves the logger from Gin context
func LoggerFromGinContext(c *gin.Context) core.Logger {
	if logger, exists := c.Get("logger"); exists {
		if l, ok := logger.(core.Logger); ok {
			return l
		}
	}
	return nil
}

// RequestIDFromGinContext retrieves the request ID from Gin context  
func RequestIDFromGinContext(c *gin.Context) string {
	if id, exists := c.Get("request_id"); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}