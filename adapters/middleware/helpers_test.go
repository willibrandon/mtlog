package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func TestRequestLogger(t *testing.T) {
	t.Run("get request logger with context logger", func(t *testing.T) {
		memSink := sinks.NewMemorySink()
		logger := mtlog.New(mtlog.WithSink(memSink))
		
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), LoggerContextKey, logger)
		req = req.WithContext(ctx)
		
		rl := GetRequestLogger(req)
		
		if rl.Logger == nil {
			t.Errorf("RequestLogger should have a logger")
		}
		if rl.r != req {
			t.Errorf("RequestLogger should reference the request")
		}
	})
	
	t.Run("get request logger without context logger", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		
		rl := GetRequestLogger(req)
		
		// Should get a no-op logger
		if rl.Logger == nil {
			t.Errorf("RequestLogger should have a fallback logger")
		}
		if rl.r != req {
			t.Errorf("RequestLogger should reference the request")
		}
	})
	
	t.Run("fluent API methods", func(t *testing.T) {
		memSink := sinks.NewMemorySink()
		logger := mtlog.New(mtlog.WithSink(memSink))
		
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), LoggerContextKey, logger)
		req = req.WithContext(ctx)
		
		rl := GetRequestLogger(req)
		
		// Test fluent methods
		rl2 := rl.WithUser("user123")
		rl3 := rl2.WithSession("session456")
		rl4 := rl3.WithTenant("tenant789")
		rl5 := rl4.WithOperation("CreateUser")
		rl6 := rl5.WithResource("User", "user123")
		
		// Most fluent methods return the same instance (except WithResource which creates new)
		if rl != rl2 || rl2 != rl3 || rl3 != rl4 || rl4 != rl5 {
			t.Errorf("Most fluent methods should return the same instance for chaining")
		}
		
		// WithResource creates a new instance
		if rl5 == rl6 {
			t.Errorf("WithResource should create a new RequestLogger instance")
		}
		
		// But the underlying loggers should be different (since With() creates new loggers)
		if rl.Logger == rl6.Logger {
			t.Errorf("Underlying logger should be enriched/different")
		}
		
		// Request should be preserved
		if rl6.Request() != req {
			t.Errorf("Request should be preserved through fluent calls")
		}
		
		// Test logging with enriched logger
		rl6.Information("Test message")
		
		events := memSink.Events()
		if len(events) != 1 {
			t.Errorf("Expected 1 log event, got %d", len(events))
		}
		
		event := events[0]
		expectedProperties := map[string]bool{
			"UserId":       false,
			"SessionId":    false,
			"TenantId":     false,
			"Operation":    false,
			"ResourceType": false,
			"ResourceId":   false,
		}
		
		for propName := range event.Properties {
			if _, exists := expectedProperties[propName]; exists {
				expectedProperties[propName] = true
			}
		}
		
		for prop, found := range expectedProperties {
			if !found {
				t.Errorf("Expected property %s not found in log event", prop)
			}
		}
	})
	
	t.Run("with error", func(t *testing.T) {
		memSink := sinks.NewMemorySink()
		logger := mtlog.New(mtlog.WithSink(memSink))
		
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), LoggerContextKey, logger)
		req = req.WithContext(ctx)
		
		rl := GetRequestLogger(req)
		
		// Test with nil error
		rl2 := rl.WithError(nil)
		if rl2 != rl {
			t.Errorf("WithError(nil) should return the same instance")
		}
		
		// Test with actual error
		err := &testError{message: "test error"}
		rl3 := rl.WithError(err)
		
		rl3.Error("Error occurred")
		
		events := memSink.Events()
		if len(events) != 1 {
			t.Errorf("Expected 1 log event, got %d", len(events))
		}
		
		event := events[0]
		
		if errorValue, exists := event.Properties["Error"]; !exists {
			t.Errorf("Error property not found in log event")
		} else if errorValue != "test error" {
			t.Errorf("Expected Error property value 'test error', got %v", errorValue)
		}
		
		if errorTypeValue, exists := event.Properties["ErrorType"]; !exists {
			t.Errorf("ErrorType property not found in log event")
		} else if errorTypeValue != "test error" {
			t.Errorf("Expected ErrorType property value 'test error', got %v", errorTypeValue)
		}
	})
}

type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

func TestFieldExtractors(t *testing.T) {
	t.Run("UserIDFromHeader", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-User-ID", "user123")
		
		value := UserIDFromHeader.Extract(req)
		if value != "user123" {
			t.Errorf("Expected 'user123', got %v", value)
		}
		
		// Test without header
		req2 := httptest.NewRequest("GET", "/test", nil)
		value2 := UserIDFromHeader.Extract(req2)
		if value2 != nil {
			t.Errorf("Expected nil, got %v", value2)
		}
	})
	
	t.Run("UserIDFromAuthHeader", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer token123")
		
		value := UserIDFromAuthHeader.Extract(req)
		if value != "user-from-token" {
			t.Errorf("Expected 'user-from-token', got %v", value)
		}
		
		// Test without Bearer prefix
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.Header.Set("Authorization", "Basic dGVzdA==")
		
		value2 := UserIDFromAuthHeader.Extract(req2)
		if value2 != nil {
			t.Errorf("Expected nil for non-Bearer auth, got %v", value2)
		}
	})
	
	t.Run("SessionIDFromCookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: "session_id", Value: "session123"})
		
		value := SessionIDFromCookie.Extract(req)
		if value != "session123" {
			t.Errorf("Expected 'session123', got %v", value)
		}
		
		// Test without cookie
		req2 := httptest.NewRequest("GET", "/test", nil)
		value2 := SessionIDFromCookie.Extract(req2)
		if value2 != nil {
			t.Errorf("Expected nil, got %v", value2)
		}
	})
	
	t.Run("TraceIDFromContext", func(t *testing.T) {
		// Test context value
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), "trace-id", "trace123")
		req = req.WithContext(ctx)
		
		value := TraceIDFromContext.Extract(req)
		if value != "trace123" {
			t.Errorf("Expected 'trace123', got %v", value)
		}
		
		// Test header fallback
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.Header.Set("X-Trace-ID", "trace456")
		
		value2 := TraceIDFromContext.Extract(req2)
		if value2 != "trace456" {
			t.Errorf("Expected 'trace456', got %v", value2)
		}
		
		// Test B3 header fallback
		req3 := httptest.NewRequest("GET", "/test", nil)
		req3.Header.Set("X-B3-TraceId", "trace789")
		
		value3 := TraceIDFromContext.Extract(req3)
		if value3 != "trace789" {
			t.Errorf("Expected 'trace789', got %v", value3)
		}
	})
	
	t.Run("SpanIDFromContext", func(t *testing.T) {
		// Test context value
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), "span-id", "span123")
		req = req.WithContext(ctx)
		
		value := SpanIDFromContext.Extract(req)
		if value != "span123" {
			t.Errorf("Expected 'span123', got %v", value)
		}
		
		// Test header fallback
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.Header.Set("X-B3-SpanId", "span456")
		
		value2 := SpanIDFromContext.Extract(req2)
		if value2 != "span456" {
			t.Errorf("Expected 'span456', got %v", value2)
		}
	})
	
	t.Run("TenantIDFromHeader", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Tenant-ID", "tenant123")
		
		value := TenantIDFromHeader.Extract(req)
		if value != "tenant123" {
			t.Errorf("Expected 'tenant123', got %v", value)
		}
	})
	
	t.Run("TenantIDFromSubdomain", func(t *testing.T) {
		req := httptest.NewRequest("GET", "https://acme.example.com/test", nil)
		
		value := TenantIDFromSubdomain.Extract(req)
		if value != "acme" {
			t.Errorf("Expected 'acme', got %v", value)
		}
		
		// Test www subdomain (should be ignored)
		req2 := httptest.NewRequest("GET", "https://www.example.com/test", nil)
		value2 := TenantIDFromSubdomain.Extract(req2)
		if value2 != nil {
			t.Errorf("Expected nil for www subdomain, got %v", value2)
		}
		
		// Test api subdomain (should be ignored)
		req3 := httptest.NewRequest("GET", "https://api.example.com/test", nil)
		value3 := TenantIDFromSubdomain.Extract(req3)
		if value3 != nil {
			t.Errorf("Expected nil for api subdomain, got %v", value3)
		}
		
		// Test domain without subdomain - should return domain name as tenant
		req4 := httptest.NewRequest("GET", "/test", nil)
		req4.Host = "example.com"
		value4 := TenantIDFromSubdomain.Extract(req4)
		if value4 != "example" {
			t.Errorf("Expected 'example' for domain example.com, got %v", value4)
		}
		
		// Test single hostname (no dots) - should return nil
		req5 := httptest.NewRequest("GET", "/test", nil)
		req5.Host = "localhost"
		value5 := TenantIDFromSubdomain.Extract(req5)
		if value5 != nil {
			t.Errorf("Expected nil for localhost, got %v", value5)
		}
	})
	
	t.Run("APIVersionFromHeader", func(t *testing.T) {
		// Test X-API-Version header
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-API-Version", "v2")
		
		value := APIVersionFromHeader.Extract(req)
		if value != "v2" {
			t.Errorf("Expected 'v2', got %v", value)
		}
		
		// Test Accept header with version
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.Header.Set("Accept", "application/json; version=v3")
		
		value2 := APIVersionFromHeader.Extract(req2)
		if value2 != "v3" {
			t.Errorf("Expected 'v3', got %v", value2)
		}
		
		// Test Accept header with version and additional params
		req3 := httptest.NewRequest("GET", "/test", nil)
		req3.Header.Set("Accept", "application/json; version=v4; charset=utf-8")
		
		value3 := APIVersionFromHeader.Extract(req3)
		if value3 != "v4" {
			t.Errorf("Expected 'v4', got %v", value3)
		}
	})
	
	t.Run("APIVersionFromPath", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/users", nil)
		
		value := APIVersionFromPath.Extract(req)
		if value != "v1" {
			t.Errorf("Expected 'v1', got %v", value)
		}
		
		// Test v2
		req2 := httptest.NewRequest("GET", "/v2/posts/123", nil)
		value2 := APIVersionFromPath.Extract(req2)
		if value2 != "v2" {
			t.Errorf("Expected 'v2', got %v", value2)
		}
		
		// Test no version
		req3 := httptest.NewRequest("GET", "/users", nil)
		value3 := APIVersionFromPath.Extract(req3)
		if value3 != nil {
			t.Errorf("Expected nil for no version, got %v", value3)
		}
		
		// Test non-version path starting with v - should NOT be treated as a version
		req4 := httptest.NewRequest("GET", "/videos/123", nil)
		value4 := APIVersionFromPath.Extract(req4)
		if value4 != nil {
			t.Errorf("Expected nil for /videos path (not a valid version format), got %v", value4)
		}
		
		// Test path that doesn't start with /v
		req5 := httptest.NewRequest("GET", "/api/users", nil)
		value5 := APIVersionFromPath.Extract(req5)
		if value5 != nil {
			t.Errorf("Expected nil for non-/v path, got %v", value5)
		}
	})
	
	t.Run("ClientIDFromHeader", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Client-ID", "client123")
		
		value := ClientIDFromHeader.Extract(req)
		if value != "client123" {
			t.Errorf("Expected 'client123', got %v", value)
		}
	})
	
	t.Run("CorrelationIDFromHeader", func(t *testing.T) {
		// Test X-Correlation-ID
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Correlation-ID", "corr123")
		
		value := CorrelationIDFromHeader.Extract(req)
		if value != "corr123" {
			t.Errorf("Expected 'corr123', got %v", value)
		}
		
		// Test X-Request-ID fallback
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.Header.Set("X-Request-ID", "req456")
		
		value2 := CorrelationIDFromHeader.Extract(req2)
		if value2 != "req456" {
			t.Errorf("Expected 'req456', got %v", value2)
		}
		
		// Test X-Trace-ID fallback
		req3 := httptest.NewRequest("GET", "/test", nil)
		req3.Header.Set("X-Trace-ID", "trace789")
		
		value3 := CorrelationIDFromHeader.Extract(req3)
		if value3 != "trace789" {
			t.Errorf("Expected 'trace789', got %v", value3)
		}
	})
	
	t.Run("GeoLocationFromHeaders", func(t *testing.T) {
		// Test Cloudflare header
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("CF-IPCountry", "US")
		
		value := GeoLocationFromHeaders.Extract(req)
		if geo, ok := value.(map[string]string); ok {
			if geo["Country"] != "US" {
				t.Errorf("Expected Country=US, got %v", geo["Country"])
			}
		} else {
			t.Errorf("Expected map[string]string, got %T", value)
		}
		
		// Test CloudFront headers
		req2 := httptest.NewRequest("GET", "/test", nil)
		req2.Header.Set("CloudFront-Viewer-Country", "CA")
		req2.Header.Set("CloudFront-Viewer-Country-Region", "ON")
		
		value2 := GeoLocationFromHeaders.Extract(req2)
		if geo, ok := value2.(map[string]string); ok {
			if geo["Country"] != "CA" {
				t.Errorf("Expected Country=CA, got %v", geo["Country"])
			}
			if geo["Region"] != "ON" {
				t.Errorf("Expected Region=ON, got %v", geo["Region"])
			}
		} else {
			t.Errorf("Expected map[string]string, got %T", value2)
		}
		
		// Test Fastly header
		req3 := httptest.NewRequest("GET", "/test", nil)
		req3.Header.Set("X-Country-Code", "GB")
		
		value3 := GeoLocationFromHeaders.Extract(req3)
		if geo, ok := value3.(map[string]string); ok {
			if geo["Country"] != "GB" {
				t.Errorf("Expected Country=GB, got %v", geo["Country"])
			}
		} else {
			t.Errorf("Expected map[string]string, got %T", value3)
		}
		
		// Test no geo headers
		req4 := httptest.NewRequest("GET", "/test", nil)
		value4 := GeoLocationFromHeaders.Extract(req4)
		if value4 != nil {
			t.Errorf("Expected nil for no geo headers, got %v", value4)
		}
	})
	
	t.Run("DeviceTypeFromUserAgent", func(t *testing.T) {
		tests := []struct {
			userAgent    string
			expectedType string
		}{
			{"Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) Mobile", "mobile"},
			{"Mozilla/5.0 (iPad; CPU OS 14_0 like Mac OS X) Tablet", "tablet"},
			{"Googlebot/2.1 (+http://www.google.com/bot.html)", "bot"},
			{"curl/7.68.0", "cli"},
			{"wget/1.20.3", "cli"},
			{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", "desktop"},
			{"", ""},
		}
		
		for _, tt := range tests {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("User-Agent", tt.userAgent)
			
			value := DeviceTypeFromUserAgent.Extract(req)
			expected := tt.expectedType
			if expected == "" {
				if value != nil {
					t.Errorf("Expected nil for empty UA, got %v", value)
				}
			} else {
				if value != expected {
					t.Errorf("Expected %s for UA %s, got %v", expected, tt.userAgent, value)
				}
			}
		}
	})
}

func TestCombineExtractors(t *testing.T) {
	extractors := CombineExtractors(
		UserIDFromHeader,
		SessionIDFromCookie,
		TenantIDFromHeader,
	)
	
	if len(extractors) != 3 {
		t.Errorf("Expected 3 extractors, got %d", len(extractors))
	}
	
	if extractors[0].Name != "UserId" {
		t.Errorf("Expected first extractor name UserId, got %s", extractors[0].Name)
	}
	if extractors[1].Name != "SessionId" {
		t.Errorf("Expected second extractor name SessionId, got %s", extractors[1].Name)
	}
	if extractors[2].Name != "TenantId" {
		t.Errorf("Expected third extractor name TenantId, got %s", extractors[2].Name)
	}
}

func TestNoOpLogger(t *testing.T) {
	logger := &noOpLogger{}
	
	// All methods should not panic
	logger.Verbose("test")
	logger.Debug("test")
	logger.Information("test")
	logger.Warning("test")
	logger.Error("test")
	logger.Fatal("test")
	logger.Write(core.InformationLevel, "test")
	
	// Fluent methods should return the same instance
	if logger.ForContext("key", "value") != logger {
		t.Errorf("ForContext should return same instance")
	}
	if logger.WithContext(context.Background()) != logger {
		t.Errorf("WithContext should return same instance")
	}
	if logger.With("key", "value") != logger {
		t.Errorf("With should return same instance")
	}
	
	// IsEnabled should return false
	if logger.IsEnabled(core.InformationLevel) {
		t.Errorf("IsEnabled should return false")
	}
	
	// Short methods should not panic
	logger.Info("test")
	logger.Warn("test")
	logger.V("test")
	logger.D("test")
	logger.I("test")
	logger.W("test")
	logger.E("test")
	logger.F("test")
}

func TestContextLoggingHelpers(t *testing.T) {
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(
		mtlog.WithSink(memSink),
		mtlog.WithMinimumLevel(core.VerboseLevel),
	)
	
	t.Run("context logging functions", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), LoggerContextKey, logger)
		
		// Test all logging functions
		VerboseContext(ctx, "Verbose message")
		DebugContext(ctx, "Debug message")
		InfoContext(ctx, "Info message")
		InformationContext(ctx, "Information message")
		WarnContext(ctx, "Warn message")
		WarningContext(ctx, "Warning message")
		ErrorContext(ctx, "Error message")
		FatalContext(ctx, "Fatal message")
		
		events := memSink.Events()
		if len(events) != 8 {
			t.Errorf("Expected 8 log events, got %d", len(events))
		}
		
		expectedMessages := []string{
			"Verbose message",
			"Debug message", 
			"Info message",
			"Information message",
			"Warn message",
			"Warning message",
			"Error message",
			"Fatal message",
		}
		
		for i, event := range events {
			if event.MessageTemplate != expectedMessages[i] {
				t.Errorf("Event %d: expected message %q, got %q", i, expectedMessages[i], event.MessageTemplate)
			}
		}
	})
	
	t.Run("context logging with nil logger", func(t *testing.T) {
		ctx := context.Background() // No logger in context
		
		// Should not panic
		VerboseContext(ctx, "Should not log")
		DebugContext(ctx, "Should not log")
		InfoContext(ctx, "Should not log")
		InformationContext(ctx, "Should not log")
		WarnContext(ctx, "Should not log")
		WarningContext(ctx, "Should not log")
		ErrorContext(ctx, "Should not log")
		FatalContext(ctx, "Should not log")
		
		// No new events should be logged
		initialCount := len(memSink.Events())
		
		VerboseContext(ctx, "Should not log")
		
		if len(memSink.Events()) != initialCount {
			t.Errorf("No events should be logged with nil logger")
		}
	})
	
	t.Run("WithContext", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), LoggerContextKey, logger)
		
		newCtx := WithContext(ctx, "key", "value")
		
		// Should return a new context with enriched logger
		if newCtx == ctx {
			t.Errorf("WithContext should return a new context")
		}
		
		// Logger in new context should be enriched
		newLogger := FromContext(newCtx)
		if newLogger == logger {
			t.Errorf("Logger in new context should be different (enriched)")
		}
		
		// Test with nil logger
		ctxWithoutLogger := context.Background()
		newCtx2 := WithContext(ctxWithoutLogger, "key", "value")
		if newCtx2 != ctxWithoutLogger {
			t.Errorf("WithContext with nil logger should return same context")
		}
	})
	
	t.Run("WithFieldsContext", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), LoggerContextKey, logger)
		
		fields := map[string]any{
			"key1": "value1",
			"key2": "value2",
			"key3": 123,
		}
		
		newCtx := WithFieldsContext(ctx, fields)
		
		// Should return a new context with enriched logger
		if newCtx == ctx {
			t.Errorf("WithFieldsContext should return a new context")
		}
		
		// Test logging with enriched context
		InfoContext(newCtx, "Test message with fields")
		
		events := memSink.Events()
		lastEvent := events[len(events)-1]
		
		// Check that fields were added
		foundFields := make(map[string]bool)
		for propName, propValue := range lastEvent.Properties {
			if propName == "key1" && propValue == "value1" {
				foundFields["key1"] = true
			}
			if propName == "key2" && propValue == "value2" {
				foundFields["key2"] = true
			}
			if propName == "key3" && propValue == 123 {
				foundFields["key3"] = true
			}
		}
		
		for field := range fields {
			if !foundFields[field] {
				t.Errorf("Field %s not found in log event", field)
			}
		}
		
		// Test with nil logger
		ctxWithoutLogger := context.Background()
		newCtx2 := WithFieldsContext(ctxWithoutLogger, fields)
		if newCtx2 != ctxWithoutLogger {
			t.Errorf("WithFieldsContext with nil logger should return same context")
		}
	})
}

func TestHelperFromContext(t *testing.T) {
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(memSink))
	
	t.Run("context with logger", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), LoggerContextKey, logger)
		
		retrievedLogger := FromContext(ctx)
		if retrievedLogger != logger {
			t.Errorf("FromContext should return the logger from context")
		}
	})
	
	t.Run("context without logger", func(t *testing.T) {
		ctx := context.Background()
		
		retrievedLogger := FromContext(ctx)
		if retrievedLogger != nil {
			t.Errorf("FromContext should return nil when no logger in context")
		}
	})
	
	t.Run("context with wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), LoggerContextKey, "not a logger")
		
		retrievedLogger := FromContext(ctx)
		if retrievedLogger != nil {
			t.Errorf("FromContext should return nil when context value is not a logger")
		}
	})
}

func TestHelperRequestIDFromContext(t *testing.T) {
	t.Run("context with request ID", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), RequestIDContextKey, "request123")
		
		requestID := RequestIDFromContext(ctx)
		if requestID != "request123" {
			t.Errorf("RequestIDFromContext should return the request ID from context")
		}
	})
	
	t.Run("context without request ID", func(t *testing.T) {
		ctx := context.Background()
		
		requestID := RequestIDFromContext(ctx)
		if requestID != "" {
			t.Errorf("RequestIDFromContext should return empty string when no request ID in context")
		}
	})
	
	t.Run("context with wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), RequestIDContextKey, 123)
		
		requestID := RequestIDFromContext(ctx)
		if requestID != "" {
			t.Errorf("RequestIDFromContext should return empty string when context value is not a string")
		}
	})
}

func BenchmarkRequestLogger(b *testing.B) {
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(memSink))
	
	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), LoggerContextKey, logger)
	req = req.WithContext(ctx)
	
	b.ResetTimer()
	
	b.Run("GetRequestLogger", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			GetRequestLogger(req)
		}
	})
	
	b.Run("FluentAPI", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rl := GetRequestLogger(req)
			rl.WithUser("user123").
				WithSession("session456").
				WithTenant("tenant789").
				WithOperation("test").
				WithResource("User", "123")
		}
	})
}

func BenchmarkFieldExtractors(b *testing.B) {
	req := httptest.NewRequest("GET", "/v1/users", nil)
	req.Header.Set("X-User-ID", "user123")
	req.Header.Set("X-Tenant-ID", "tenant456")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "session789"})
	
	b.ResetTimer()
	
	b.Run("UserIDFromHeader", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			UserIDFromHeader.Extract(req)
		}
	})
	
	b.Run("TenantIDFromHeader", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			TenantIDFromHeader.Extract(req)
		}
	})
	
	b.Run("SessionIDFromCookie", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			SessionIDFromCookie.Extract(req)
		}
	})
	
	b.Run("APIVersionFromPath", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			APIVersionFromPath.Extract(req)
		}
	})
	
	b.Run("DeviceTypeFromUserAgent", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			DeviceTypeFromUserAgent.Extract(req)
		}
	})
}

func BenchmarkContextHelpers(b *testing.B) {
	memSink := sinks.NewMemorySink()
	logger := mtlog.New(mtlog.WithSink(memSink))
	ctx := context.WithValue(context.Background(), LoggerContextKey, logger)
	
	b.ResetTimer()
	
	b.Run("InfoContext", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			InfoContext(ctx, "Test message")
		}
	})
	
	b.Run("WithContext", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			WithContext(ctx, "key", "value")
		}
	})
	
	b.Run("WithFieldsContext", func(b *testing.B) {
		fields := map[string]any{
			"key1": "value1",
			"key2": "value2",
		}
		for i := 0; i < b.N; i++ {
			WithFieldsContext(ctx, fields)
		}
	})
}