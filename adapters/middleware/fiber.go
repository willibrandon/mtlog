package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/willibrandon/mtlog/core"
)

// Fiber creates a Fiber middleware for request logging
func Fiber(logger core.Logger, opts ...*Options) fiber.Handler {
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

	return func(c *fiber.Ctx) error {
		// Check if we should skip this path
		for _, skip := range options.SkipPaths {
			if c.Path() == skip {
				return c.Next()
			}
		}

		// Start timing
		start := time.Now()

		// Get or generate request ID
		requestID := c.Get(options.RequestIDHeader)
		if requestID == "" && options.GenerateRequestID {
			requestID = uuid.New().String()
		}

		// Create logger with request ID
		logger := options.Logger
		if requestID != "" {
			logger = logger.With("RequestId", requestID)
			// Add request ID to response header
			c.Set(options.RequestIDHeader, requestID)
		}

		// Store logger in Fiber locals
		c.Locals("logger", logger)
		c.Locals("request_id", requestID)

		// Process request
		err := c.Next()

		// Calculate latency
		latency := time.Since(start)
		latencyValue := getLatencyValue(latency, options.LatencyUnit)

		// Get status code, handling error cases
		statusCode := c.Response().StatusCode()
		if err != nil && (statusCode == 0 || statusCode == 200) {
			statusCode = 500
		}

		// Build positional arguments for the template
		args := []any{
			c.Method(),      // {Method}
			c.Path(),        // {Path}
			statusCode,      // {StatusCode}
			latencyValue,    // {LatencyField}
		}

		// Add additional properties using With()
		enrichedLogger := logger
		for _, field := range options.RequestFields {
			switch field {
			case "method":
				enrichedLogger = enrichedLogger.With("Method", c.Method())
			case "path":
				enrichedLogger = enrichedLogger.With("Path", c.Path())
			case "ip":
				enrichedLogger = enrichedLogger.With("IP", c.IP())
			case "user_agent":
				enrichedLogger = enrichedLogger.With("UserAgent", string(c.Request().Header.UserAgent()))
			case "referer":
				enrichedLogger = enrichedLogger.With("Referer", string(c.Request().Header.Referer()))
			case "proto":
				enrichedLogger = enrichedLogger.With("Proto", c.Protocol())
			case "host":
				enrichedLogger = enrichedLogger.With("Host", c.Hostname())
			}
		}
		
		// Add standard properties
		// Note: Fiber's Response().Body() returns the full body, but we want the size
		responseSize := len(c.Response().Body())
		if responseSize == 0 {
			// If body is empty, try to get from Content-Length header
			if contentLength := c.Response().Header.ContentLength(); contentLength > 0 {
				responseSize = contentLength
			}
		}
		enrichedLogger = enrichedLogger.With("StatusCode", statusCode).
			With("Size", responseSize).
			With(options.LatencyField, latencyValue)

		// Add error if present
		if err != nil {
			enrichedLogger = enrichedLogger.With("Error", err.Error())
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

		return err
	}
}

// LoggerFromFiberContext retrieves the logger from Fiber context
func LoggerFromFiberContext(c *fiber.Ctx) core.Logger {
	if logger := c.Locals("logger"); logger != nil {
		if l, ok := logger.(core.Logger); ok {
			return l
		}
	}
	return nil
}

// RequestIDFromFiberContext retrieves the request ID from Fiber context
func RequestIDFromFiberContext(c *fiber.Ctx) string {
	if id := c.Locals("request_id"); id != nil {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}