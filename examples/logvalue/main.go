package main

import (
	"time"

	"github.com/willibrandon/mtlog"
)

// User demonstrates custom logging for sensitive data
type User struct {
	ID        int
	Username  string
	Email     string
	Password  string // Should never be logged
	APIKey    string // Should never be logged
	LastLogin time.Time
}

// LogValue provides a safe representation for logging
func (u User) LogValue() any {
	return map[string]any{
		"id":        u.ID,
		"username":  u.Username,
		"email":     maskEmail(u.Email),
		"lastLogin": u.LastLogin,
		// Password and APIKey are intentionally omitted
	}
}

func maskEmail(email string) string {
	if len(email) < 3 {
		return "***"
	}
	atIndex := -1
	for i, ch := range email {
		if ch == '@' {
			atIndex = i
			break
		}
	}
	if atIndex > 1 && atIndex < len(email)-1 {
		return email[:2] + "***" + email[atIndex:]
	}
	return email[:1] + "***"
}

// DatabaseConnection demonstrates custom logging for connection info
type DatabaseConnection struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string // Should be masked
}

func (db DatabaseConnection) LogValue() any {
	return struct {
		Host     string
		Port     int
		Database string
		Username string
		Status   string
	}{
		Host:     db.Host,
		Port:     db.Port,
		Database: db.Database,
		Username: db.Username,
		Status:   "connected",
		// Password is not included
	}
}

// APIRequest demonstrates selective logging of HTTP requests
type APIRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
}

func (r APIRequest) LogValue() any {
	// Filter sensitive headers
	safeHeaders := make(map[string]string)
	for k, v := range r.Headers {
		switch k {
		case "Authorization", "X-API-Key", "Cookie":
			safeHeaders[k] = "[REDACTED]"
		default:
			safeHeaders[k] = v
		}
	}

	// Truncate body if too large
	bodyPreview := string(r.Body)
	const maxBodyLength = 200
	if len(r.Body) > maxBodyLength {
		bodyPreview = string(r.Body[:maxBodyLength]) + "... (truncated)"
	}

	return map[string]any{
		"method":  r.Method,
		"url":     r.URL,
		"headers": safeHeaders,
		"body":    bodyPreview,
		"size":    len(r.Body),
	}
}

// MetricSample demonstrates efficient logging of metrics
type MetricSample struct {
	Name      string
	Value     float64
	Unit      string
	Timestamp time.Time
	Tags      map[string]string
}

func (m MetricSample) LogValue() any {
	// Format for efficient metric logging
	return struct {
		Metric string
		Value  float64
		Tags   map[string]string
	}{
		Metric: m.Name + " (" + m.Unit + ")",
		Value:  m.Value,
		Tags:   m.Tags,
	}
}

// ErrorContext represents an error with context
type ErrorContext struct {
	UserID    int
	Operation string
	Error     error
	Timestamp time.Time
}

// LoggableErrorContext wraps ErrorContext to implement LogValue
type LoggableErrorContext struct {
	ErrorContext
}

func (e LoggableErrorContext) LogValue() any {
	errorMsg := "none"
	if e.Error != nil {
		errorMsg = e.Error.Error()
	}

	return map[string]any{
		"userId":    e.UserID,
		"operation": e.Operation,
		"error":     errorMsg,
		"timestamp": e.Timestamp.Format(time.RFC3339),
	}
}

func main() {
	log := mtlog.New(
		mtlog.WithConsoleProperties(),
		mtlog.WithCapturing(),
	)

	// Example 1: User with sensitive data
	user := User{
		ID:        123,
		Username:  "alice",
		Email:     "alice@example.com",
		Password:  "super-secret-password",
		APIKey:    "sk_live_abcd1234",
		LastLogin: time.Now(),
	}

	log.Information("User logged in: {@User}", user)

	// Example 2: Database connection
	db := DatabaseConnection{
		Host:     "db.example.com",
		Port:     5432,
		Database: "production",
		Username: "dbuser",
		Password: "dbpass123", // Won't be logged
	}

	log.Information("Connected to database: {@Database}", db)

	// Example 3: API request with sensitive headers
	request := APIRequest{
		Method: "POST",
		URL:    "https://api.example.com/v1/users",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			"X-API-Key":     "api_key_123456",
			"X-Request-ID":  "req-789",
		},
		Body: []byte(`{
			"name": "New User",
			"email": "newuser@example.com",
			"preferences": {
				"notifications": true,
				"theme": "dark"
			}
		}`),
	}

	log.Information("API request: {@Request}", request)

	// Example 4: Metrics logging
	metric := MetricSample{
		Name:      "api.response.time",
		Value:     234.5,
		Unit:      "ms",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"endpoint": "/v1/users",
			"method":   "POST",
			"status":   "200",
		},
	}

	log.Information("Metric recorded: {@Metric}", metric)

	// Example 5: Array of custom types
	team := []User{
		{ID: 1, Username: "alice", Email: "alice@example.com", Password: "pass1"},
		{ID: 2, Username: "bob", Email: "bob@example.com", Password: "pass2"},
		{ID: 3, Username: "charlie", Email: "charlie@example.com", Password: "pass3"},
	}

	log.Information("Team members: {@Team}", team)

	// Example 6: Error scenarios with context
	errCtx := LoggableErrorContext{
		ErrorContext: ErrorContext{
			UserID:    user.ID,
			Operation: "CreateOrder",
			Error:     nil, // Simulate no error
			Timestamp: time.Now(),
		},
	}

	log.Information("Operation completed: {@Context}", errCtx)
}
