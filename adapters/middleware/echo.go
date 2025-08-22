package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/willibrandon/mtlog/core"
)

// Echo creates an Echo middleware for request logging
func Echo(logger core.Logger, opts ...*Options) echo.MiddlewareFunc {
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

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Check if we should skip this path
			for _, skip := range options.SkipPaths {
				if c.Request().URL.Path == skip {
					return next(c)
				}
			}

			// Start timing
			start := time.Now()

			// Get or generate request ID
			requestID := c.Request().Header.Get(options.RequestIDHeader)
			if requestID == "" && options.GenerateRequestID {
				requestID = uuid.New().String()
			}

			// Create logger with request ID
			logger := options.Logger
			if requestID != "" {
				logger = logger.With("RequestId", requestID)
				// Add request ID to response header
				c.Response().Header().Set(options.RequestIDHeader, requestID)
			}

			// Store logger in Echo context
			c.Set("logger", logger)
			c.Set("request_id", requestID)

			// Process request
			err := next(c)

			// Calculate latency
			latency := time.Since(start)
			latencyValue := getLatencyValue(latency, options.LatencyUnit)

			// Get status code, handling error cases
			statusCode := c.Response().Status
			
			// If there's an error and no status was explicitly set, derive it from the error
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					// Use the error's status code if response status is still 200 (default)
					if statusCode == 0 || statusCode == http.StatusOK {
						statusCode = he.Code
					}
				} else if statusCode == 0 || statusCode == http.StatusOK {
					// For non-HTTP errors, assume 500 if no explicit status was set
					statusCode = http.StatusInternalServerError
				}
			}

			// Build positional arguments for the template
			args := []any{
				c.Request().Method,   // {Method}
				c.Request().URL.Path, // {Path}
				statusCode,           // {StatusCode}
				latencyValue,         // {LatencyField}
			}

			// Add additional properties using With()
			enrichedLogger := logger
			for _, field := range options.RequestFields {
				switch field {
				case "method":
					enrichedLogger = enrichedLogger.With("Method", c.Request().Method)
				case "path":
					enrichedLogger = enrichedLogger.With("Path", c.Request().URL.Path)
				case "ip":
					enrichedLogger = enrichedLogger.With("IP", c.RealIP())
				case "user_agent":
					enrichedLogger = enrichedLogger.With("UserAgent", c.Request().UserAgent())
				case "referer":
					enrichedLogger = enrichedLogger.With("Referer", c.Request().Referer())
				case "proto":
					enrichedLogger = enrichedLogger.With("Proto", c.Request().Proto)
				case "host":
					enrichedLogger = enrichedLogger.With("Host", c.Request().Host)
				}
			}
			
			// Add standard properties
			enrichedLogger = enrichedLogger.With("StatusCode", statusCode).
				With("Size", c.Response().Size).
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
}

// LoggerFromEchoContext retrieves the logger from Echo context
func LoggerFromEchoContext(c echo.Context) core.Logger {
	if logger := c.Get("logger"); logger != nil {
		if l, ok := logger.(core.Logger); ok {
			return l
		}
	}
	return nil
}

// RequestIDFromEchoContext retrieves the request ID from Echo context
func RequestIDFromEchoContext(c echo.Context) string {
	if id := c.Get("request_id"); id != nil {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}