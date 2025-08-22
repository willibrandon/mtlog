package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/middleware"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	// Create mtlog logger with console output
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.DebugLevel),
		mtlog.WithProperty("service", "gin-example"),
	)

	// Create Gin router
	router := gin.New()
	router.Use(gin.Recovery())

	// Add mtlog middleware with custom options
	router.Use(middleware.Gin(logger, &middleware.Options{
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
	}))

	// Home endpoint
	router.GET("/", func(c *gin.Context) {
		// Get logger from context for nested logging
		if logger := middleware.LoggerFromGinContext(c); logger != nil {
			logger.Debug("Processing home request")
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to mtlog Gin middleware example",
		})
	})

	// User endpoint
	router.GET("/api/users", func(c *gin.Context) {
		// Get logger and request ID from context
		logger := middleware.LoggerFromGinContext(c)
		requestID := middleware.RequestIDFromGinContext(c)

		if logger != nil {
			logger.Information("Fetching users", "RequestId", requestID)
		}

		users := []gin.H{
			{"id": 1, "name": "Alice", "role": "admin"},
			{"id": 2, "name": "Bob", "role": "user"},
		}

		c.JSON(http.StatusOK, gin.H{
			"users": users,
		})
	})

	// Create user endpoint
	router.POST("/api/users", func(c *gin.Context) {
		logger := middleware.LoggerFromGinContext(c)
		
		var user struct {
			Name  string `json:"name" binding:"required"`
			Email string `json:"email" binding:"required,email"`
		}

		if err := c.ShouldBindJSON(&user); err != nil {
			if logger != nil {
				logger.Warning("Invalid user data", "Error", err.Error())
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if logger != nil {
			logger.Information("Creating new user", "Name", user.Name, "Email", user.Email)
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":    3,
			"name":  user.Name,
			"email": user.Email,
		})
	})

	// Error endpoint (for testing error logging)
	router.GET("/error", func(c *gin.Context) {
		logger := middleware.LoggerFromGinContext(c)
		if logger != nil {
			logger.Error("Simulated error occurred")
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal Server Error",
		})
	})

	// Not found endpoint
	router.GET("/not-found", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Resource not found",
		})
	})

	// Health check endpoint (skipped by middleware)
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Start server
	logger.Information("Starting Gin server on :8080")
	if err := router.Run(":8080"); err != nil {
		logger.Fatal("Server failed to start", err)
	}
}