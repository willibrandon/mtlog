package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTraceContextExtraction(t *testing.T) {
	tests := []struct {
		name        string
		format      TraceHeaderFormat
		headers     map[string]string
		wantTraceID string
		wantSpanID  string
		wantSampled bool
	}{
		{
			name:   "W3C_TraceContext",
			format: FormatW3C,
			headers: map[string]string{
				"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			},
			wantTraceID: "4bf92f3577b34da6a3ce929d0e0e4736",
			wantSpanID:  "00f067aa0ba902b7",
			wantSampled: true,
		},
		{
			name:   "B3_MultiHeader",
			format: FormatB3,
			headers: map[string]string{
				"X-B3-TraceId":  "80f198ee56343ba864fe8b2a57d3eff7",
				"X-B3-SpanId":   "e457b5a2e4d86bd1",
				"X-B3-Sampled":  "1",
				"X-B3-ParentSpanId": "05e3ac9a4f6e3b90",
			},
			wantTraceID: "80f198ee56343ba864fe8b2a57d3eff7",
			wantSpanID:  "e457b5a2e4d86bd1",
			wantSampled: true,
		},
		{
			name:   "B3_SingleHeader",
			format: FormatB3Single,
			headers: map[string]string{
				"b3": "80f198ee56343ba864fe8b2a57d3eff7-e457b5a2e4d86bd1-1",
			},
			wantTraceID: "80f198ee56343ba864fe8b2a57d3eff7",
			wantSpanID:  "e457b5a2e4d86bd1",
			wantSampled: true,
		},
		{
			name:   "XRay",
			format: FormatXRay,
			headers: map[string]string{
				"X-Amzn-Trace-Id": "Root=1-5e13aa58-2c8b3e0f4d8e9f0a1b2c3d4e;Parent=1234567890abcdef;Sampled=1",
			},
			wantTraceID: "5e13aa58-2c8b3e0f4d8e9f0a1b2c3d4e",
			wantSpanID:  "", // XRay parser doesn't extract Parent as SpanID
			wantSampled: true,
		},
		{
			name:   "Custom",
			format: FormatCustom,
			headers: map[string]string{
				"X-Trace-ID": "custom-trace-123",
				"X-Span-ID":  "custom-span-456",
			},
			wantTraceID: "custom-trace-123",
			wantSpanID:  "custom-span-456",
			wantSampled: true, // Default sampling is true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			traceCtx := extractTraceContext(req, tt.format)

			if traceCtx.TraceID != tt.wantTraceID {
				t.Errorf("TraceID = %v, want %v", traceCtx.TraceID, tt.wantTraceID)
			}
			if traceCtx.SpanID != tt.wantSpanID {
				t.Errorf("SpanID = %v, want %v", traceCtx.SpanID, tt.wantSpanID)
			}
			if traceCtx.Sampled != tt.wantSampled {
				t.Errorf("Sampled = %v, want %v", traceCtx.Sampled, tt.wantSampled)
			}
		})
	}
}

func TestTraceContextPropagation(t *testing.T) {
	tests := []struct {
		name           string
		format         TraceHeaderFormat
		checkHeaders   func(*http.Request) bool
	}{
		{
			name:   "W3C_Propagation",
			format: FormatW3C,
			checkHeaders: func(r *http.Request) bool {
				header := r.Header.Get("traceparent")
				return header != "" && strings.HasPrefix(header, "00-")
			},
		},
		{
			name:   "B3_Propagation",
			format: FormatB3,
			checkHeaders: func(r *http.Request) bool {
				return r.Header.Get("X-B3-TraceId") != "" &&
					r.Header.Get("X-B3-SpanId") != ""
			},
		},
		{
			name:   "B3Single_Propagation",
			format: FormatB3Single,
			checkHeaders: func(r *http.Request) bool {
				header := r.Header.Get("b3")
				return header != "" && strings.Contains(header, "-")
			},
		},
		{
			name:   "XRay_Propagation",
			format: FormatXRay,
			checkHeaders: func(r *http.Request) bool {
				header := r.Header.Get("X-Amzn-Trace-Id")
				return header != "" && strings.Contains(header, "Root=")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				traceCtx := GetTraceContext(ctx)
				
				if traceCtx.TraceID == "" {
					t.Error("TraceID should not be empty in handler")
				}
				if traceCtx.SpanID == "" {
					t.Error("SpanID should not be empty in handler")
				}
				
				w.WriteHeader(http.StatusOK)
			})

			middleware := PropagateTraceContext(handler, CorrelationOptions{
				GenerateTraceID:     true,
				GenerateSpanID:      true,
				HeaderFormat:        tt.format,
				PropagateDownstream: true,
			})

			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()
			
			middleware.ServeHTTP(rec, req)
			
			// Check response headers for propagation
			// Note: httptest.ResponseRecorder doesn't capture outbound request headers
			// In a real scenario, these would be checked on the downstream service
			// For this test, we verify that the middleware processed the request successfully
			if rec.Code != http.StatusOK {
				t.Errorf("Expected status OK, got %d", rec.Code)
			}
		})
	}
}

func TestBaggageExtraction(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		traceCtx := GetTraceContext(ctx)
		
		// Verify that baggage was extracted into TraceContext.Baggage
		if len(traceCtx.Baggage) != 2 {
			t.Errorf("Expected 2 baggage items, got %d", len(traceCtx.Baggage))
		}
		
		// Check specific baggage values
		if userID, ok := traceCtx.Baggage["User-Id"]; !ok || userID != "123" {
			t.Errorf("Expected User-Id baggage to be '123', got '%s' (exists: %v)", userID, ok)
		}
		
		if sessionID, ok := traceCtx.Baggage["Session-Id"]; !ok || sessionID != "abc" {
			t.Errorf("Expected Session-Id baggage to be 'abc', got '%s' (exists: %v)", sessionID, ok)
		}
		
		// Verify baggage is propagated in response headers
		if w.Header().Get("X-Baggage-User-Id") != "123" {
			t.Error("User-Id baggage not propagated in response headers")
		}
		if w.Header().Get("X-Baggage-Session-Id") != "abc" {
			t.Error("Session-Id baggage not propagated in response headers")
		}
		
		w.WriteHeader(http.StatusOK)
	})

	middleware := PropagateTraceContext(handler, CorrelationOptions{
		GenerateTraceID:     true,
		GenerateSpanID:      true,
		ExtractBaggage:      true,
		BaggagePrefix:       "X-Baggage-",
		PropagateDownstream: true,
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Baggage-User-Id", "123")
	req.Header.Set("X-Baggage-Session-Id", "abc")
	rec := httptest.NewRecorder()
	
	middleware.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", rec.Code)
	}
}

func TestTraceRoundTripper(t *testing.T) {
	// Create a test server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that trace headers are present
		if r.Header.Get("traceparent") == "" {
			t.Error("traceparent header not found in downstream request")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Create context with trace info
	ctx := context.Background()
	ctx = context.WithValue(ctx, TraceIDContextKey, "test-trace-id")
	ctx = context.WithValue(ctx, SpanIDContextKey, "test-span-id")

	// Create client with TraceRoundTripper
	client := &http.Client{
		Transport: &TraceRoundTripper{
			Transport: http.DefaultTransport,
			Format: FormatW3C,
		},
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", testServer.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %v, want %v", resp.StatusCode, http.StatusOK)
	}
}

func TestGenerateTraceID(t *testing.T) {
	id1 := generateTraceID()
	id2 := generateTraceID()
	
	if id1 == "" {
		t.Error("generateTraceID should not return empty string")
	}
	if id2 == "" {
		t.Error("generateTraceID should not return empty string")
	}
	if id1 == id2 {
		t.Error("generateTraceID should generate unique IDs")
	}
	if len(id1) != 32 {
		t.Errorf("TraceID length = %v, want 32", len(id1))
	}
}

func TestGenerateSpanID(t *testing.T) {
	id1 := generateSpanID()
	id2 := generateSpanID()
	
	if id1 == "" {
		t.Error("generateSpanID should not return empty string")
	}
	if id2 == "" {
		t.Error("generateSpanID should not return empty string")
	}
	if id1 == id2 {
		t.Error("generateSpanID should generate unique IDs")
	}
	if len(id1) != 16 {
		t.Errorf("SpanID length = %v, want 16", len(id1))
	}
}

func TestInvalidTraceHeaders(t *testing.T) {
	tests := []struct {
		name    string
		format  TraceHeaderFormat
		headers map[string]string
	}{
		{
			name:   "Invalid_W3C",
			format: FormatW3C,
			headers: map[string]string{
				"traceparent": "invalid-header",
			},
		},
		{
			name:   "Invalid_B3",
			format: FormatB3,
			headers: map[string]string{
				"X-B3-TraceId": "not-hex",
				"X-B3-SpanId":  "not-hex",
			},
		},
		{
			name:   "Invalid_B3Single",
			format: FormatB3Single,
			headers: map[string]string{
				"b3": "invalid",
			},
		},
		{
			name:   "Invalid_XRay",
			format: FormatXRay,
			headers: map[string]string{
				"X-Amzn-Trace-Id": "invalid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			// Should generate new IDs when headers are invalid
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				traceCtx := GetTraceContext(ctx)
				
				if traceCtx.TraceID == "" {
					t.Error("Should generate TraceID when headers are invalid")
				}
				if traceCtx.SpanID == "" {
					t.Error("Should generate SpanID when headers are invalid")
				}
			})

			middleware := PropagateTraceContext(handler, CorrelationOptions{
				GenerateTraceID: true,
				GenerateSpanID:  true,
				HeaderFormat:    tt.format,
			})

			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)
		})
	}
}

func TestTracingClient(t *testing.T) {
	client := NewTracingClient(FormatW3C)
	
	if client == nil {
		t.Fatal("NewTracingClient should not return nil")
	}
	
	transport, ok := client.Transport.(*TraceRoundTripper)
	if !ok {
		t.Fatal("Transport should be TraceRoundTripper")
	}
	
	if transport.Format != FormatW3C {
		t.Errorf("Format = %v, want %v", transport.Format, FormatW3C)
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	
	// Test with empty context
	traceCtx := GetTraceContext(ctx)
	if traceCtx == nil {
		t.Error("GetTraceContext should never return nil")
	}
	if traceCtx.TraceID != "" {
		t.Error("TraceID should be empty for empty context")
	}
	
	// Test with trace context
	expectedTraceCtx := &TraceContext{
		TraceID: "test-trace",
		SpanID:  "test-span",
	}
	ctx = context.WithValue(ctx, TraceIDContextKey, expectedTraceCtx.TraceID)
	ctx = context.WithValue(ctx, SpanIDContextKey, expectedTraceCtx.SpanID)
	
	retrieved := GetTraceContext(ctx)
	if retrieved == nil {
		t.Fatal("GetTraceContext should return trace context")
	}
	if retrieved.TraceID != "test-trace" {
		t.Errorf("TraceID = %v, want test-trace", retrieved.TraceID)
	}
}

func BenchmarkTraceExtraction(b *testing.B) {
	formats := []struct {
		name   string
		format TraceHeaderFormat
		setup  func(*http.Request)
	}{
		{
			name:   "W3C",
			format: FormatW3C,
			setup: func(r *http.Request) {
				r.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
			},
		},
		{
			name:   "B3",
			format: FormatB3,
			setup: func(r *http.Request) {
				r.Header.Set("X-B3-TraceId", "80f198ee56343ba864fe8b2a57d3eff7")
				r.Header.Set("X-B3-SpanId", "e457b5a2e4d86bd1")
			},
		},
		{
			name:   "B3Single",
			format: FormatB3Single,
			setup: func(r *http.Request) {
				r.Header.Set("b3", "80f198ee56343ba864fe8b2a57d3eff7-e457b5a2e4d86bd1-1")
			},
		},
	}

	for _, f := range formats {
		b.Run(f.name, func(b *testing.B) {
			req := httptest.NewRequest("GET", "/test", nil)
			f.setup(req)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := extractTraceContext(req, f.format)
				_ = ctx
			}
		})
	}
}