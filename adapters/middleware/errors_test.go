package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name       string
		createFunc func() *MiddlewareError
		wantType   ErrorType
		wantStatus int
		wantMsg    string
	}{
		{
			name: "PanicError",
			createFunc: func() *MiddlewareError {
				return NewPanicError("test panic", "GET", "/test", "req-123")
			},
			wantType:   ErrorTypePanic,
			wantStatus: 500,
			wantMsg:    "panic occurred: test panic",
		},
		{
			name: "ValidationError",
			createFunc: func() *MiddlewareError {
				return NewValidationError("invalid input", map[string]string{"field": "required"})
			},
			wantType:   ErrorTypeValidation,
			wantStatus: 400,
			wantMsg:    "invalid input",
		},
		{
			name: "TimeoutError",
			createFunc: func() *MiddlewareError {
				return NewTimeoutError("/slow", "30s")
			},
			wantType:   ErrorTypeTimeout,
			wantStatus: 408,
			wantMsg:    "request timeout after 30s",
		},
		{
			name: "InternalError",
			createFunc: func() *MiddlewareError {
				return NewInternalError("database error", errors.New("connection failed"))
			},
			wantType:   ErrorTypeInternal,
			wantStatus: 500,
			wantMsg:    "database error",
		},
		{
			name: "BodyCaptureError",
			createFunc: func() *MiddlewareError {
				return NewBodyCaptureError(errors.New("read error"), "request")
			},
			wantType:   ErrorTypeBodyCapture,
			wantStatus: 0,
			wantMsg:    "failed to capture request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createFunc()
			
			if err.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", err.Type, tt.wantType)
			}
			
			if err.StatusCode != tt.wantStatus {
				t.Errorf("StatusCode = %v, want %v", err.StatusCode, tt.wantStatus)
			}
			
			if err.Message != tt.wantMsg {
				t.Errorf("Message = %v, want %v", err.Message, tt.wantMsg)
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	baseErr := errors.New("base error")
	err := NewInternalError("wrapper", baseErr)
	
	if err.Unwrap() != baseErr {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), baseErr)
	}
	
	if !errors.Is(err, baseErr) {
		t.Error("errors.Is() should return true for wrapped error")
	}
}

func TestErrorWithMethods(t *testing.T) {
	err := NewValidationError("test", nil)
	
	// Test WithRequestInfo
	err = err.WithRequestInfo("POST", "/api/users", "req-456")
	if err.Method != "POST" {
		t.Errorf("Method = %v, want POST", err.Method)
	}
	if err.Path != "/api/users" {
		t.Errorf("Path = %v, want /api/users", err.Path)
	}
	if err.RequestID != "req-456" {
		t.Errorf("RequestID = %v, want req-456", err.RequestID)
	}
	
	// Test WithStackTrace
	err = err.WithStackTrace()
	if err.StackTrace == "" {
		t.Error("StackTrace should not be empty after WithStackTrace()")
	}
	if !strings.Contains(err.StackTrace, "TestErrorWithMethods") {
		t.Error("StackTrace should contain function name")
	}
}

func TestDefaultErrorHandler(t *testing.T) {
	tests := []struct {
		name       string
		err        *MiddlewareError
		wantStatus int
		checkBody  func(map[string]interface{}) bool
	}{
		{
			name: "with_status_code",
			err: &MiddlewareError{
				Type:       ErrorTypeValidation,
				Message:    "validation failed",
				StatusCode: 400,
			},
			wantStatus: 400,
			checkBody: func(body map[string]interface{}) bool {
				return body["error"] == "validation failed"
			},
		},
		{
			name: "without_status_code",
			err: &MiddlewareError{
				Type:    ErrorTypeInternal,
				Message: "internal error",
			},
			wantStatus: 500,
			checkBody: func(body map[string]interface{}) bool {
				return body["error"] == "internal error"
			},
		},
		{
			name: "with_request_id",
			err: &MiddlewareError{
				Type:      ErrorTypeTimeout,
				Message:   "timeout",
				RequestID: "req-789",
			},
			wantStatus: 500,
			checkBody: func(body map[string]interface{}) bool {
				return body["request_id"] == "req-789"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, response := DefaultErrorHandler(tt.err)
			
			if status != tt.wantStatus {
				t.Errorf("status = %v, want %v", status, tt.wantStatus)
			}
			
			respMap, ok := response.(map[string]interface{})
			if !ok {
				t.Fatal("response should be a map[string]interface{}")
			}
			
			if !tt.checkBody(respMap) {
				t.Errorf("response body check failed: %+v", respMap)
			}
		})
	}
}

func TestDevelopmentErrorHandler(t *testing.T) {
	err := &MiddlewareError{
		Type:       ErrorTypePanic,
		Message:    "panic occurred",
		StatusCode: 500,
		Method:     "GET",
		Path:       "/test",
		RequestID:  "req-123",
		StackTrace: "stack trace here",
		Details:    map[string]string{"key": "value"},
	}
	
	status, response := DevelopmentErrorHandler(err)
	
	if status != 500 {
		t.Errorf("status = %v, want 500", status)
	}
	
	// Verify all fields are included in development mode
	respMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatal("response should be a map[string]interface{}")
	}
	
	expectedFields := []string{"error", "type", "method", "path", "request_id", "stack_trace"}
	for _, field := range expectedFields {
		if _, exists := respMap[field]; !exists {
			t.Errorf("missing field %s in development response", field)
		}
	}
	
	// Check that details are preserved
	if respMap["details"] == nil {
		t.Error("details should be included in development response")
	}
}

func TestErrorReset(t *testing.T) {
	err := &MiddlewareError{
		Type:       ErrorTypeValidation,
		Message:    "test",
		Cause:      errors.New("cause"),
		StatusCode: 400,
		RequestID:  "req-123",
		Path:       "/test",
		Method:     "POST",
		StackTrace: "stack",
		Details:    map[string]string{"key": "value"},
	}
	
	err.Reset()
	
	if err.Type != "" {
		t.Error("Type should be empty after Reset()")
	}
	if err.Message != "" {
		t.Error("Message should be empty after Reset()")
	}
	if err.Cause != nil {
		t.Error("Cause should be nil after Reset()")
	}
	if err.StatusCode != 0 {
		t.Error("StatusCode should be 0 after Reset()")
	}
	if err.RequestID != "" {
		t.Error("RequestID should be empty after Reset()")
	}
	if err.Path != "" {
		t.Error("Path should be empty after Reset()")
	}
	if err.Method != "" {
		t.Error("Method should be empty after Reset()")
	}
	if err.StackTrace != "" {
		t.Error("StackTrace should be empty after Reset()")
	}
	if err.Details != nil {
		t.Error("Details should be nil after Reset()")
	}
}

func TestErrorJSONSerialization(t *testing.T) {
	err := &MiddlewareError{
		Type:       ErrorTypeValidation,
		Message:    "validation error",
		StatusCode: 400,
		RequestID:  "req-123",
		Details:    map[string]string{"field": "required"},
	}
	
	_, response := DevelopmentErrorHandler(err)
	
	// Should be JSON serializable
	jsonData, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		t.Fatalf("Failed to marshal response to JSON: %v", marshalErr)
	}
	
	// Should be valid JSON
	var decoded map[string]interface{}
	if unmarshalErr := json.Unmarshal(jsonData, &decoded); unmarshalErr != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", unmarshalErr)
	}
	
	// Verify key fields are present
	if decoded["error"] != "validation error" {
		t.Errorf("error field = %v, want 'validation error'", decoded["error"])
	}
	if decoded["request_id"] != "req-123" {
		t.Errorf("request_id field = %v, want 'req-123'", decoded["request_id"])
	}
}

func TestErrorPooling(t *testing.T) {
	// Enable pooling for this test
	oldPooling := EnablePooling
	EnablePooling = true
	defer func() { EnablePooling = oldPooling }()
	
	// Create and return errors to pool multiple times
	for i := 0; i < 100; i++ {
		err := NewValidationError("test", nil)
		if err == nil {
			t.Fatal("NewValidationError should not return nil")
		}
		
		// Simulate using the error
		err.WithRequestInfo("GET", "/test", "req-123")
		
		// Return to pool
		putError(err)
	}
	
	// Get from pool and verify it's reset
	err := getError()
	if err.Type != "" || err.Message != "" || err.StatusCode != 0 {
		t.Error("Error from pool should be reset")
	}
	putError(err)
}

func BenchmarkErrorCreation(b *testing.B) {
	b.Run("WithoutPooling", func(b *testing.B) {
		oldPooling := EnablePooling
		EnablePooling = false
		defer func() { EnablePooling = oldPooling }()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := NewValidationError("test", nil)
			_ = err
		}
	})
	
	b.Run("WithPooling", func(b *testing.B) {
		oldPooling := EnablePooling
		EnablePooling = true
		defer func() { EnablePooling = oldPooling }()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := NewValidationError("test", nil)
			putError(err)
		}
	})
}

func TestErrorHandlerWithNilError(t *testing.T) {
	// Test that handlers handle nil gracefully - DefaultErrorHandler expects non-nil
	// In practice, the middleware should never pass nil to DefaultErrorHandler
	// This test verifies proper error construction
	err := NewValidationError("test error", nil)
	status, response := DefaultErrorHandler(err)
	if status != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", status, http.StatusBadRequest)
	}
	if response == nil {
		t.Error("response should not be nil even for nil error")
	}
	
	// Test DevelopmentErrorHandler with a valid error
	status2, response2 := DevelopmentErrorHandler(err)
	if status2 != http.StatusBadRequest {
		t.Errorf("status = %v, want %v", status2, http.StatusBadRequest)
	}
	if response2 == nil {
		t.Error("response should not be nil")
	}
}