package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/middleware"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	// Create mtlog logger with console output
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.DebugLevel),
		mtlog.WithProperty("service", "echo-example"),
	)

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true

	// Add recovery middleware
	e.Use(echomw.Recover())

	// Add mtlog middleware with custom options
	e.Use(middleware.Echo(logger, &middleware.Options{
		Logger:            logger,
		GenerateRequestID: true,
		RequestIDHeader:   "X-Request-ID",
		SkipPaths:         []string{"/health", "/metrics"},
		RequestFields: []string{
			"method",
			"path",
			"ip",
			"user_agent",
			"referer",
			"proto",
		},
		LatencyField: "duration_ms",
		LatencyUnit:  "ms",
	}))

	// Home endpoint
	e.GET("/", func(c echo.Context) error {
		// Get logger from context for nested logging
		if logger := middleware.LoggerFromEchoContext(c); logger != nil {
			logger.Debug("Processing home request")
		}

		return c.JSON(http.StatusOK, map[string]string{
			"message": "Welcome to mtlog Echo middleware example",
		})
	})

	// User endpoint
	e.GET("/api/users", func(c echo.Context) error {
		// Get logger and request ID from context
		logger := middleware.LoggerFromEchoContext(c)
		requestID := middleware.RequestIDFromEchoContext(c)

		if logger != nil {
			logger.Information("Fetching users", "RequestId", requestID)
		}

		users := []map[string]interface{}{
			{"id": 1, "name": "Alice", "role": "admin"},
			{"id": 2, "name": "Bob", "role": "user"},
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"users": users,
		})
	})

	// Get user by ID endpoint
	e.GET("/api/users/:id", func(c echo.Context) error {
		logger := middleware.LoggerFromEchoContext(c)
		userID := c.Param("id")

		if logger != nil {
			logger.Information("Fetching user by ID", "UserId", userID)
		}

		// Simulate not found
		if userID == "999" {
			return echo.NewHTTPError(http.StatusNotFound, "User not found")
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"id":   userID,
			"name": "Alice",
			"role": "admin",
		})
	})

	// Create user endpoint
	e.POST("/api/users", func(c echo.Context) error {
		logger := middleware.LoggerFromEchoContext(c)

		var user struct {
			Name  string `json:"name" validate:"required"`
			Email string `json:"email" validate:"required,email"`
		}

		if err := c.Bind(&user); err != nil {
			if logger != nil {
				logger.Warning("Invalid user data", "Error", err.Error())
			}
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		if logger != nil {
			logger.Information("Creating new user", "Name", user.Name, "Email", user.Email)
		}

		return c.JSON(http.StatusCreated, map[string]interface{}{
			"id":    3,
			"name":  user.Name,
			"email": user.Email,
		})
	})

	// Error endpoint (for testing error logging)
	e.GET("/error", func(c echo.Context) error {
		logger := middleware.LoggerFromEchoContext(c)
		if logger != nil {
			logger.Error("Simulated error occurred")
		}

		return echo.NewHTTPError(http.StatusInternalServerError, "Internal Server Error")
	})

	// Health check endpoint (skipped by middleware)
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Custom error handler
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		logger := middleware.LoggerFromEchoContext(c)
		
		code := http.StatusInternalServerError
		message := "Internal Server Error"
		
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			message = he.Message.(string)
		}

		if logger != nil && code >= 500 {
			logger.Error("HTTP error occurred", "Error", err.Error(), "StatusCode", code)
		}

		c.JSON(code, map[string]string{"error": message})
	}

	// Start server
	logger.Information("Starting Echo server on :8080")
	if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server failed to start", err)
	}
}