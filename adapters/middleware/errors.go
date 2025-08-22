package middleware

import (
	"fmt"
	"runtime/debug"
)

// ErrorType represents the type of error that occurred
type ErrorType string

const (
	// ErrorTypePanic indicates a panic occurred in the handler
	ErrorTypePanic ErrorType = "panic"
	
	// ErrorTypeBodyCapture indicates an error capturing request/response body
	ErrorTypeBodyCapture ErrorType = "body_capture"
	
	// ErrorTypeTimeout indicates a request timeout
	ErrorTypeTimeout ErrorType = "timeout"
	
	// ErrorTypeValidation indicates a validation error
	ErrorTypeValidation ErrorType = "validation"
	
	// ErrorTypeInternal indicates an internal server error
	ErrorTypeInternal ErrorType = "internal"
	
	// ErrorTypeExternal indicates an external service error
	ErrorTypeExternal ErrorType = "external"
)

// MiddlewareError represents a structured error in the middleware
type MiddlewareError struct {
	Type       ErrorType   `json:"type"`
	Message    string      `json:"message"`
	Cause      error       `json:"-"`
	StatusCode int         `json:"status_code,omitempty"`
	RequestID  string      `json:"request_id,omitempty"`
	Path       string      `json:"path,omitempty"`
	Method     string      `json:"method,omitempty"`
	StackTrace string      `json:"stack_trace,omitempty"`
	Details    interface{} `json:"details,omitempty"`
}

// Error implements the error interface
func (e *MiddlewareError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *MiddlewareError) Unwrap() error {
	return e.Cause
}

// WithStackTrace adds a stack trace to the error
func (e *MiddlewareError) WithStackTrace() *MiddlewareError {
	e.StackTrace = string(debug.Stack())
	return e
}

// WithRequestInfo adds request information to the error
func (e *MiddlewareError) WithRequestInfo(method, path, requestID string) *MiddlewareError {
	e.Method = method
	e.Path = path
	e.RequestID = requestID
	return e
}

// WithDetails adds additional details to the error
func (e *MiddlewareError) WithDetails(details interface{}) *MiddlewareError {
	e.Details = details
	return e
}

// NewPanicError creates a new panic error
func NewPanicError(panicValue interface{}, method, path, requestID string) *MiddlewareError {
	var err *MiddlewareError
	if EnablePooling {
		err = getError()
	} else {
		err = &MiddlewareError{}
	}
	err.Type = ErrorTypePanic
	err.Message = fmt.Sprintf("panic occurred: %v", panicValue)
	err.StatusCode = 500
	err.Method = method
	err.Path = path
	err.RequestID = requestID
	return err
}

// NewBodyCaptureError creates a new body capture error
func NewBodyCaptureError(err error, direction string) *MiddlewareError {
	var mErr *MiddlewareError
	if EnablePooling {
		mErr = getError()
	} else {
		mErr = &MiddlewareError{}
	}
	mErr.Type = ErrorTypeBodyCapture
	mErr.Message = fmt.Sprintf("failed to capture %s body", direction)
	mErr.Cause = err
	return mErr
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(path string, duration string) *MiddlewareError {
	var err *MiddlewareError
	if EnablePooling {
		err = getError()
	} else {
		err = &MiddlewareError{}
	}
	err.Type = ErrorTypeTimeout
	err.Message = fmt.Sprintf("request timeout after %s", duration)
	err.StatusCode = 408
	err.Path = path
	return err
}

// NewValidationError creates a new validation error
func NewValidationError(message string, details interface{}) *MiddlewareError {
	var err *MiddlewareError
	if EnablePooling {
		err = getError()
	} else {
		err = &MiddlewareError{}
	}
	err.Type = ErrorTypeValidation
	err.Message = message
	err.StatusCode = 400
	err.Details = details
	return err
}

// NewInternalError creates a new internal error
func NewInternalError(message string, cause error) *MiddlewareError {
	var err *MiddlewareError
	if EnablePooling {
		err = getError()
	} else {
		err = &MiddlewareError{}
	}
	err.Type = ErrorTypeInternal
	err.Message = message
	err.Cause = cause
	err.StatusCode = 500
	return err
}

// ErrorHandler handles middleware errors
type ErrorHandler func(err *MiddlewareError) (statusCode int, response interface{})

// DefaultErrorHandler provides a default error handling implementation
func DefaultErrorHandler(err *MiddlewareError) (statusCode int, response interface{}) {
	statusCode = err.StatusCode
	if statusCode == 0 {
		statusCode = 500
	}
	
	// In production, don't expose internal details
	response = map[string]interface{}{
		"error": err.Message,
		"type":  string(err.Type),
	}
	
	if err.RequestID != "" {
		response.(map[string]interface{})["request_id"] = err.RequestID
	}
	
	return statusCode, response
}

// DevelopmentErrorHandler provides detailed error information for development
func DevelopmentErrorHandler(err *MiddlewareError) (statusCode int, response interface{}) {
	statusCode = err.StatusCode
	if statusCode == 0 {
		statusCode = 500
	}
	
	response = map[string]interface{}{
		"error":      err.Message,
		"type":       string(err.Type),
		"request_id": err.RequestID,
		"path":       err.Path,
		"method":     err.Method,
	}
	
	if err.StackTrace != "" {
		response.(map[string]interface{})["stack_trace"] = err.StackTrace
	}
	
	if err.Details != nil {
		response.(map[string]interface{})["details"] = err.Details
	}
	
	if err.Cause != nil {
		response.(map[string]interface{})["cause"] = err.Cause.Error()
	}
	
	return statusCode, response
}