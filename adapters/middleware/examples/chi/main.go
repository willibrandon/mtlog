package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/adapters/middleware"
	"github.com/willibrandon/mtlog/core"
)

func main() {
	// Create mtlog logger with console output
	logger := mtlog.New(
		mtlog.WithConsole(),
		mtlog.WithMinimumLevel(core.DebugLevel),
		mtlog.WithProperty("service", "chi-example"),
	)

	// Create Chi router
	r := chi.NewRouter()

	// Add Chi's recovery middleware
	r.Use(chimw.Recoverer)

	// Add mtlog middleware with custom options
	r.Use(middleware.Chi(logger, &middleware.Options{
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
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		// Get logger from context for nested logging
		if logger := middleware.FromContext(r.Context()); logger != nil {
			logger.Debug("Processing home request")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Welcome to mtlog Chi middleware example",
		})
	})

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Users routes
		r.Route("/users", func(r chi.Router) {
			// List users
			r.Get("/", func(w http.ResponseWriter, req *http.Request) {
				// Get logger and request ID from context
				logger := middleware.FromContext(req.Context())
				requestID := middleware.RequestIDFromContext(req.Context())

				if logger != nil {
					logger.Information("Fetching users", "RequestId", requestID)
				}

				users := []map[string]interface{}{
					{"id": 1, "name": "Alice", "role": "admin"},
					{"id": 2, "name": "Bob", "role": "user"},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"users": users,
				})
			})

			// Get user by ID
			r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
				logger := middleware.FromContext(req.Context())
				userID := chi.URLParam(req, "id")

				if logger != nil {
					logger.Information("Fetching user by ID", "UserId", userID)
				}

				// Simulate not found
				if userID == "999" {
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "User not found",
					})
					return
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id":   userID,
					"name": "Alice",
					"role": "admin",
				})
			})

			// Create user
			r.Post("/", func(w http.ResponseWriter, req *http.Request) {
				logger := middleware.FromContext(req.Context())

				var user struct {
					Name  string `json:"name"`
					Email string `json:"email"`
				}

				if err := json.NewDecoder(req.Body).Decode(&user); err != nil {
					if logger != nil {
						logger.Warning("Invalid user data", "Error", err.Error())
					}
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]string{
						"error": err.Error(),
					})
					return
				}

				if user.Name == "" || user.Email == "" {
					if logger != nil {
						logger.Warning("Missing required fields")
					}
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]string{
						"error": "Name and email are required",
					})
					return
				}

				if logger != nil {
					logger.Information("Creating new user", "Name", user.Name, "Email", user.Email)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id":    3,
					"name":  user.Name,
					"email": user.Email,
				})
			})

			// Update user
			r.Put("/{id}", func(w http.ResponseWriter, req *http.Request) {
				logger := middleware.FromContext(req.Context())
				userID := chi.URLParam(req, "id")

				var updates struct {
					Name  string `json:"name"`
					Email string `json:"email"`
				}

				if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
					if logger != nil {
						logger.Warning("Invalid update data", "Error", err.Error())
					}
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]string{
						"error": err.Error(),
					})
					return
				}

				if logger != nil {
					logger.Information("Updating user", "UserId", userID, "Name", updates.Name, "Email", updates.Email)
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id":    userID,
					"name":  updates.Name,
					"email": updates.Email,
				})
			})

			// Delete user
			r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
				logger := middleware.FromContext(req.Context())
				userID := chi.URLParam(req, "id")

				if logger != nil {
					logger.Information("Deleting user", "UserId", userID)
				}

				w.WriteHeader(http.StatusNoContent)
			})
		})

		// Posts routes (demonstrating nested routing)
		r.Route("/posts", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, req *http.Request) {
				logger := middleware.FromContext(req.Context())
				if logger != nil {
					logger.Information("Fetching posts")
				}

				posts := []map[string]interface{}{
					{"id": 1, "title": "First Post", "author": "Alice"},
					{"id": 2, "title": "Second Post", "author": "Bob"},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"posts": posts,
				})
			})
		})
	})

	// Error endpoint (for testing error logging)
	r.Get("/error", func(w http.ResponseWriter, req *http.Request) {
		logger := middleware.FromContext(req.Context())
		if logger != nil {
			logger.Error("Simulated error occurred")
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Internal Server Error",
		})
	})

	// Health check endpoint (skipped by middleware)
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Not found handler
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Route not found",
		})
	})

	// Start server
	logger.Information("Starting Chi server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		logger.Fatal("Server failed to start", err)
	}
}