package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// BodySanitizer defines a function that sanitizes request/response bodies
type BodySanitizer func(contentType string, body []byte) []byte

// SensitiveFieldNames are common field names that contain sensitive data
var SensitiveFieldNames = []string{
	"password", "passwd", "pwd", "secret", "token", "apikey", "api_key",
	"access_token", "refresh_token", "private_key", "credit_card",
	"card_number", "cvv", "ssn", "social_security", "tax_id",
}

// DefaultBodySanitizer redacts sensitive fields in JSON payloads
func DefaultBodySanitizer(contentType string, body []byte) []byte {
	if !strings.Contains(contentType, "application/json") {
		return body
	}

	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		// Not valid JSON, return as-is
		return body
	}

	sanitizeMap(data)

	sanitized, err := json.Marshal(data)
	if err != nil {
		return body
	}

	return sanitized
}

// sanitizeMap recursively sanitizes sensitive fields in a map
func sanitizeMap(data map[string]any) {
	for key, value := range data {
		if isSensitiveField(key) {
			data[key] = "[REDACTED]"
			continue
		}

		// Recursively sanitize nested objects
		switch v := value.(type) {
		case map[string]any:
			sanitizeMap(v)
		case []any:
			for i, item := range v {
				if m, ok := item.(map[string]any); ok {
					sanitizeMap(m)
					v[i] = m
				}
			}
		}
	}
}

// isSensitiveField checks if a field name is sensitive
func isSensitiveField(field string) bool {
	lowerField := strings.ToLower(field)
	for _, sensitive := range SensitiveFieldNames {
		if strings.Contains(lowerField, sensitive) {
			return true
		}
	}
	return false
}

// RegexBodySanitizer creates a sanitizer that uses regex patterns
func RegexBodySanitizer(patterns ...*regexp.Regexp) BodySanitizer {
	return func(contentType string, body []byte) []byte {
		result := body
		for _, pattern := range patterns {
			result = pattern.ReplaceAll(result, []byte("[REDACTED]"))
		}
		return result
	}
}

// NoOpBodySanitizer returns the body unchanged
func NoOpBodySanitizer(contentType string, body []byte) []byte {
	return body
}

// captureRequestBody captures and optionally sanitizes the request body
func captureRequestBody(r *http.Request, maxSize int, sanitizer BodySanitizer) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	// Read the body with limit
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(maxSize+1)))
	if err != nil {
		return nil, err
	}

	// Check if body was truncated
	truncated := len(body) > maxSize
	if truncated {
		body = append(body[:maxSize], []byte("...[truncated]")...)
	}

	// Restore the body for the handler (with original content)
	if truncated {
		// Need to read the rest if truncated
		rest, _ := io.ReadAll(r.Body)
		fullBody := append(body[:maxSize], rest...)
		r.Body = io.NopCloser(bytes.NewReader(fullBody))
	} else {
		r.Body = io.NopCloser(bytes.NewReader(body))
	}

	// Sanitize if configured
	if sanitizer != nil {
		contentType := r.Header.Get("Content-Type")
		body = sanitizer(contentType, body)
	}

	return body, nil
}

// responseRecorder wraps ResponseWriter to capture response body
type responseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	written    bool
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	var buf *bytes.Buffer
	if EnablePooling {
		buf = getBuffer()
	} else {
		buf = &bytes.Buffer{}
	}
	return &responseRecorder{
		ResponseWriter: w,
		body:          buf,
		statusCode:    http.StatusOK,
	}
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.written {
		r.statusCode = code
		r.ResponseWriter.WriteHeader(code)
		r.written = true
	}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.WriteHeader(http.StatusOK)
	}
	r.body.Write(b) // Capture the response
	return r.ResponseWriter.Write(b)
}

// limitedResponseRecorder captures response body up to a size limit
type limitedResponseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	written    bool
	maxSize    int
	truncated  bool
}

func newLimitedResponseRecorder(w http.ResponseWriter, maxSize int) *limitedResponseRecorder {
	return &limitedResponseRecorder{
		ResponseWriter: w,
		body:          &bytes.Buffer{},
		statusCode:    http.StatusOK,
		maxSize:       maxSize,
	}
}

func (r *limitedResponseRecorder) WriteHeader(code int) {
	if !r.written {
		r.statusCode = code
		r.ResponseWriter.WriteHeader(code)
		r.written = true
	}
}

func (r *limitedResponseRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.WriteHeader(http.StatusOK)
	}
	
	// Capture up to maxSize
	if !r.truncated && r.body.Len() < r.maxSize {
		remaining := r.maxSize - r.body.Len()
		if len(b) > remaining {
			r.body.Write(b[:remaining])
			r.body.WriteString("...[truncated]")
			r.truncated = true
		} else {
			r.body.Write(b)
		}
	}
	
	return r.ResponseWriter.Write(b)
}

func (r *limitedResponseRecorder) Status() int {
	return r.statusCode
}

func (r *limitedResponseRecorder) Body() []byte {
	return r.body.Bytes()
}

func (r *responseRecorder) Status() int {
	return r.statusCode
}

func (r *responseRecorder) Body() []byte {
	return r.body.Bytes()
}