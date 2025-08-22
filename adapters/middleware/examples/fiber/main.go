package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/middleware"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	// Create mtlog logger with console output
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.DebugLevel),
		mtlog.WithProperty("service", "fiber-example"),
	)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Add recovery middleware
	app.Use(recover.New())

	// Add mtlog middleware with custom options
	app.Use(middleware.Fiber(logger, &middleware.Options{
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
			"host",
		},
		LatencyField: "duration_ms",
		LatencyUnit:  "ms",
	}))

	// Home endpoint
	app.Get("/", func(c *fiber.Ctx) error {
		// Get logger from context for nested logging
		if logger := middleware.LoggerFromFiberContext(c); logger != nil {
			logger.Debug("Processing home request")
		}

		return c.JSON(fiber.Map{
			"message": "Welcome to mtlog Fiber middleware example",
		})
	})

	// User endpoint
	app.Get("/api/users", func(c *fiber.Ctx) error {
		// Get logger and request ID from context
		logger := middleware.LoggerFromFiberContext(c)
		requestID := middleware.RequestIDFromFiberContext(c)

		if logger != nil {
			logger.Information("Fetching users", "RequestId", requestID)
		}

		users := []fiber.Map{
			{"id": 1, "name": "Alice", "role": "admin"},
			{"id": 2, "name": "Bob", "role": "user"},
		}

		return c.JSON(fiber.Map{
			"users": users,
		})
	})

	// Get user by ID endpoint
	app.Get("/api/users/:id", func(c *fiber.Ctx) error {
		logger := middleware.LoggerFromFiberContext(c)
		userID := c.Params("id")

		if logger != nil {
			logger.Information("Fetching user by ID", "UserId", userID)
		}

		// Simulate not found
		if userID == "999" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		return c.JSON(fiber.Map{
			"id":   userID,
			"name": "Alice",
			"role": "admin",
		})
	})

	// Create user endpoint
	app.Post("/api/users", func(c *fiber.Ctx) error {
		logger := middleware.LoggerFromFiberContext(c)

		var user struct {
			Name  string `json:"name" validate:"required"`
			Email string `json:"email" validate:"required,email"`
		}

		if err := c.BodyParser(&user); err != nil {
			if logger != nil {
				logger.Warning("Invalid user data", "Error", err.Error())
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if logger != nil {
			logger.Information("Creating new user", "Name", user.Name, "Email", user.Email)
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"id":    3,
			"name":  user.Name,
			"email": user.Email,
		})
	})

	// Update user endpoint
	app.Put("/api/users/:id", func(c *fiber.Ctx) error {
		logger := middleware.LoggerFromFiberContext(c)
		userID := c.Params("id")

		var updates struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		if err := c.BodyParser(&updates); err != nil {
			if logger != nil {
				logger.Warning("Invalid update data", "Error", err.Error())
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if logger != nil {
			logger.Information("Updating user", "UserId", userID, "Updates", updates)
		}

		return c.JSON(fiber.Map{
			"id":    userID,
			"name":  updates.Name,
			"email": updates.Email,
		})
	})

	// Delete user endpoint
	app.Delete("/api/users/:id", func(c *fiber.Ctx) error {
		logger := middleware.LoggerFromFiberContext(c)
		userID := c.Params("id")

		if logger != nil {
			logger.Information("Deleting user", "UserId", userID)
		}

		return c.SendStatus(fiber.StatusNoContent)
	})

	// Error endpoint (for testing error logging)
	app.Get("/error", func(c *fiber.Ctx) error {
		logger := middleware.LoggerFromFiberContext(c)
		if logger != nil {
			logger.Error("Simulated error occurred")
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal Server Error",
		})
	})

	// Health check endpoint (skipped by middleware)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	// Custom error handler
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Route not found",
		})
	})

	// Start server
	logger.Information("Starting Fiber server on :8080")
	if err := app.Listen(":8080"); err != nil {
		logger.Fatal("Server failed to start", err)
	}
}