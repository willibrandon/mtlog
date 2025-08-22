package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestErrorPool(t *testing.T) {
	t.Run("get and put error", func(t *testing.T) {
		// Enable pooling for this test
		originalPooling := EnablePooling
		EnablePooling = true
		defer func() { EnablePooling = originalPooling }()
		
		err := getError()
		if err == nil {
			t.Errorf("getError() should return a non-nil error")
		}
		
		// Set some values
		err.Type = "TestError"
		err.Message = "Test message"
		err.StatusCode = 500
		err.RequestID = "req123"
		err.Path = "/test"
		err.Method = "GET"
		err.StackTrace = "stack trace"
		err.Details = map[string]any{"key": "value"}
		
		// Put it back
		putError(err)
		
		// Get another one - should be reset
		err2 := getError()
		if err2.Type != "" {
			t.Errorf("Pooled error should be reset, Type = %q", err2.Type)
		}
		if err2.Message != "" {
			t.Errorf("Pooled error should be reset, Message = %q", err2.Message)
		}
		if err2.StatusCode != 0 {
			t.Errorf("Pooled error should be reset, StatusCode = %d", err2.StatusCode)
		}
		
		putError(err2)
	})
	
	t.Run("put nil error", func(t *testing.T) {
		// Should not panic
		putError(nil)
	})
	
	t.Run("error reset", func(t *testing.T) {
		err := &MiddlewareError{
			Type:       "TestError",
			Message:    "Test message",
			StatusCode: 500,
			RequestID:  "req123",
			Path:       "/test",
			Method:     "GET",
			StackTrace: "stack trace",
			Details:    map[string]any{"key": "value"},
		}
		
		err.Reset()
		
		if err.Type != "" || err.Message != "" || err.StatusCode != 0 ||
			err.RequestID != "" || err.Path != "" || err.Method != "" ||
			err.StackTrace != "" || err.Details != nil {
			t.Errorf("Reset should clear all fields")
		}
	})
}

func TestResponseWriterPool(t *testing.T) {
	t.Run("get and put response writer", func(t *testing.T) {
		originalPooling := EnablePooling
		EnablePooling = true
		defer func() { EnablePooling = originalPooling }()
		
		w := httptest.NewRecorder()
		rw := getResponseWriter(w)
		
		if rw.ResponseWriter != w {
			t.Errorf("ResponseWriter should be wrapped")
		}
		if rw.statusCode != http.StatusOK {
			t.Errorf("Initial status code should be 200, got %d", rw.statusCode)
		}
		if rw.written {
			t.Errorf("Initial written should be false")
		}
		if rw.size != 0 {
			t.Errorf("Initial size should be 0, got %d", rw.size)
		}
		
		// Modify the response writer
		rw.WriteHeader(http.StatusCreated)
		rw.Write([]byte("test"))
		
		// Put it back
		putResponseWriter(rw)
		
		// Get another one - should be reset
		w2 := httptest.NewRecorder()
		rw2 := getResponseWriter(w2)
		
		if rw2.statusCode != http.StatusOK {
			t.Errorf("Pooled response writer should be reset, status = %d", rw2.statusCode)
		}
		if rw2.written {
			t.Errorf("Pooled response writer should be reset, written = %v", rw2.written)
		}
		if rw2.size != 0 {
			t.Errorf("Pooled response writer should be reset, size = %d", rw2.size)
		}
		
		putResponseWriter(rw2)
	})
	
	t.Run("put nil response writer", func(t *testing.T) {
		// Should not panic
		putResponseWriter(nil)
	})
	
	t.Run("response writer reset", func(t *testing.T) {
		w := httptest.NewRecorder()
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusCreated,
			written:        true,
			size:           100,
		}
		
		rw.Reset()
		
		if rw.ResponseWriter != nil {
			t.Errorf("ResponseWriter should be nil after reset")
		}
		if rw.statusCode != http.StatusOK {
			t.Errorf("Status code should be reset to 200, got %d", rw.statusCode)
		}
		if rw.written {
			t.Errorf("Written should be reset to false")
		}
		if rw.size != 0 {
			t.Errorf("Size should be reset to 0, got %d", rw.size)
		}
	})
}

func TestRequestMetricPool(t *testing.T) {
	t.Run("get and put request metric", func(t *testing.T) {
		originalPooling := EnablePooling
		EnablePooling = true
		defer func() { EnablePooling = originalPooling }()
		
		metric := getRequestMetric()
		if metric == nil {
			t.Errorf("getRequestMetric() should return a non-nil metric")
		}
		
		// Set some values
		metric.Method = "GET"
		metric.Path = "/test"
		metric.StatusCode = 200
		metric.Duration = time.Second
		metric.Timestamp = time.Now()
		
		// Put it back
		putRequestMetric(metric)
		
		// Get another one - should be reset
		metric2 := getRequestMetric()
		if metric2.Method != "" {
			t.Errorf("Pooled metric should be reset, Method = %q", metric2.Method)
		}
		if metric2.Path != "" {
			t.Errorf("Pooled metric should be reset, Path = %q", metric2.Path)
		}
		if metric2.StatusCode != 0 {
			t.Errorf("Pooled metric should be reset, StatusCode = %d", metric2.StatusCode)
		}
		if metric2.Duration != 0 {
			t.Errorf("Pooled metric should be reset, Duration = %v", metric2.Duration)
		}
		if !metric2.Timestamp.IsZero() {
			t.Errorf("Pooled metric should be reset, Timestamp = %v", metric2.Timestamp)
		}
		
		putRequestMetric(metric2)
	})
	
	t.Run("put nil metric", func(t *testing.T) {
		// Should not panic
		putRequestMetric(nil)
	})
	
	t.Run("metric reset", func(t *testing.T) {
		metric := &RequestMetric{
			Method:     "GET",
			Path:       "/test",
			StatusCode: 200,
			Duration:   time.Second,
			Timestamp:  time.Now(),
		}
		
		metric.Reset()
		
		if metric.Method != "" || metric.Path != "" || metric.StatusCode != 0 ||
			metric.Duration != 0 || !metric.Timestamp.IsZero() {
			t.Errorf("Reset should clear all fields")
		}
	})
}

func TestBufferPool(t *testing.T) {
	t.Run("get and put buffer", func(t *testing.T) {
		originalPooling := EnablePooling
		EnablePooling = true
		defer func() { EnablePooling = originalPooling }()
		
		buf := getBuffer()
		if buf == nil {
			t.Errorf("getBuffer() should return a non-nil buffer")
		}
		
		if buf.Len() != 0 {
			t.Errorf("Buffer should be reset, len = %d", buf.Len())
		}
		
		// Use the buffer
		buf.WriteString("test data")
		if buf.Len() != 9 {
			t.Errorf("Buffer should contain data, len = %d", buf.Len())
		}
		
		// Put it back
		putBuffer(buf)
		
		// Get another one - should be reset
		buf2 := getBuffer()
		if buf2.Len() != 0 {
			t.Errorf("Pooled buffer should be reset, len = %d", buf2.Len())
		}
		
		putBuffer(buf2)
	})
	
	t.Run("put nil buffer", func(t *testing.T) {
		// Should not panic
		putBuffer(nil)
	})
	
	t.Run("buffer reuse", func(t *testing.T) {
		originalPooling := EnablePooling
		EnablePooling = true
		defer func() { EnablePooling = originalPooling }()
		
		// Get and return a buffer
		buf1 := getBuffer()
		putBuffer(buf1)
		
		// Get another buffer - might be the same one
		buf2 := getBuffer()
		putBuffer(buf2)
		
		// Test passes if no panic occurs
	})
}

func TestLimitedResponseRecorderPool(t *testing.T) {
	t.Run("get and put limited response recorder", func(t *testing.T) {
		originalPooling := EnablePooling
		EnablePooling = true
		defer func() { EnablePooling = originalPooling }()
		
		w := httptest.NewRecorder()
		recorder := getLimitedResponseRecorder(w, 1024)
		
		if recorder.ResponseWriter != w {
			t.Errorf("ResponseWriter should be wrapped")
		}
		if recorder.maxSize != 1024 {
			t.Errorf("maxSize should be 1024, got %d", recorder.maxSize)
		}
		if recorder.statusCode != http.StatusOK {
			t.Errorf("Initial status code should be 200, got %d", recorder.statusCode)
		}
		if recorder.written {
			t.Errorf("Initial written should be false")
		}
		if recorder.body == nil {
			t.Errorf("Body buffer should not be nil")
		}
		
		// Use the recorder
		recorder.WriteHeader(http.StatusCreated)
		recorder.Write([]byte("test data"))
		
		// Put it back
		putLimitedResponseRecorder(recorder)
		
		// Get another one - should be reset
		w2 := httptest.NewRecorder()
		recorder2 := getLimitedResponseRecorder(w2, 2048)
		
		if recorder2.statusCode != http.StatusOK {
			t.Errorf("Pooled recorder should be reset, status = %d", recorder2.statusCode)
		}
		if recorder2.written {
			t.Errorf("Pooled recorder should be reset, written = %v", recorder2.written)
		}
		if recorder2.maxSize != 2048 {
			t.Errorf("maxSize should be set to new value 2048, got %d", recorder2.maxSize)
		}
		if recorder2.body == nil {
			t.Errorf("Body buffer should not be nil")
		}
		if recorder2.body.Len() != 0 {
			t.Errorf("Body buffer should be empty, len = %d", recorder2.body.Len())
		}
		
		putLimitedResponseRecorder(recorder2)
	})
	
	t.Run("put nil recorder", func(t *testing.T) {
		// Should not panic
		putLimitedResponseRecorder(nil)
	})
	
	t.Run("recorder reset", func(t *testing.T) {
		w := httptest.NewRecorder()
		buf := &bytes.Buffer{}
		buf.WriteString("test")
		
		recorder := &limitedResponseRecorder{
			ResponseWriter: w,
			body:          buf,
			statusCode:    http.StatusCreated,
			written:       true,
			maxSize:       1024,
		}
		
		recorder.Reset()
		
		if recorder.ResponseWriter != nil {
			t.Errorf("ResponseWriter should be nil after reset")
		}
		if recorder.body != nil {
			t.Errorf("Body should be nil after reset")
		}
		if recorder.statusCode != http.StatusOK {
			t.Errorf("Status code should be reset to 200, got %d", recorder.statusCode)
		}
		if recorder.written {
			t.Errorf("Written should be reset to false")
		}
		if recorder.maxSize != 0 {
			t.Errorf("MaxSize should be reset to 0, got %d", recorder.maxSize)
		}
	})
}

func TestEnablePooling(t *testing.T) {
	originalPooling := EnablePooling
	defer func() { EnablePooling = originalPooling }()
	
	t.Run("pooling enabled", func(t *testing.T) {
		EnablePooling = true
		
		// Test that pooled objects are used
		buf := getBuffer()
		buf.WriteString("test")
		putBuffer(buf)
		
		buf2 := getBuffer()
		// Buffer should be reset even if it's the same instance
		if buf2.Len() != 0 {
			t.Errorf("Buffer should be reset when pooling is enabled")
		}
		putBuffer(buf2)
	})
	
	t.Run("pooling disabled", func(t *testing.T) {
		EnablePooling = false
		
		// Should still work but may create new instances each time
		buf := getBuffer()
		putBuffer(buf)
		
		buf2 := getBuffer()
		putBuffer(buf2)
		
		// Test passes if no panic occurs
	})
}

func TestPoolStats(t *testing.T) {
	// Reset stats before test
	ResetPoolStats()
	
	// Get initial stats
	initialStats := GetPoolStats()
	if initialStats.ErrorPoolHits != 0 || initialStats.ErrorPoolMisses != 0 {
		t.Errorf("Initial stats should be zero")
	}
	
	// Test reset functionality
	ResetPoolStats()
	statsAfterReset := GetPoolStats()
	if statsAfterReset.ErrorPoolHits != 0 {
		t.Errorf("Stats should be reset to zero")
	}
}

func TestPoolConcurrency(t *testing.T) {
	originalPooling := EnablePooling
	EnablePooling = true
	defer func() { EnablePooling = originalPooling }()
	
	const numGoroutines = 50
	const numOperations = 100
	
	t.Run("error pool concurrency", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					err := getError()
					err.Type = "TestError"
					err.Message = "Test message"
					putError(err)
				}
			}()
		}
		
		wg.Wait()
		// Test passes if no race conditions occur
	})
	
	t.Run("buffer pool concurrency", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					buf := getBuffer()
					buf.WriteString("test data")
					putBuffer(buf)
				}
			}()
		}
		
		wg.Wait()
		// Test passes if no race conditions occur
	})
	
	t.Run("response writer pool concurrency", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					w := httptest.NewRecorder()
					rw := getResponseWriter(w)
					rw.WriteHeader(http.StatusOK)
					rw.Write([]byte("test"))
					putResponseWriter(rw)
				}
			}()
		}
		
		wg.Wait()
		// Test passes if no race conditions occur
	})
	
	t.Run("request metric pool concurrency", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					metric := getRequestMetric()
					metric.Method = "GET"
					metric.Path = "/test"
					metric.StatusCode = 200
					putRequestMetric(metric)
				}
			}()
		}
		
		wg.Wait()
		// Test passes if no race conditions occur
	})
	
	t.Run("limited response recorder pool concurrency", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					w := httptest.NewRecorder()
					recorder := getLimitedResponseRecorder(w, 1024)
					recorder.Write([]byte("test"))
					putLimitedResponseRecorder(recorder)
				}
			}()
		}
		
		wg.Wait()
		// Test passes if no race conditions occur
	})
}

func TestPoolMemoryUsage(t *testing.T) {
	originalPooling := EnablePooling
	EnablePooling = true
	defer func() { EnablePooling = originalPooling }()
	
	// Test that pools don't leak memory by creating many objects
	// and ensuring they can be properly recycled
	
	t.Run("buffer pool memory usage", func(t *testing.T) {
		const numBuffers = 1000
		buffers := make([]*bytes.Buffer, numBuffers)
		
		// Create many buffers
		for i := 0; i < numBuffers; i++ {
			buf := getBuffer()
			buf.WriteString("test data that takes some memory")
			buffers[i] = buf
		}
		
		// Return them all
		for _, buf := range buffers {
			putBuffer(buf)
		}
		
		// Get new buffers - should reuse the pooled ones
		for i := 0; i < numBuffers; i++ {
			buf := getBuffer()
			if buf.Len() != 0 {
				t.Errorf("Recycled buffer should be empty, len = %d", buf.Len())
			}
			putBuffer(buf)
		}
	})
	
	t.Run("error pool memory usage", func(t *testing.T) {
		const numErrors = 1000
		errors := make([]*MiddlewareError, numErrors)
		
		// Create many errors
		for i := 0; i < numErrors; i++ {
			err := getError()
			err.Type = "TestError"
			err.Message = "Test error message with some content"
			err.Details = map[string]any{"key": "value"}
			errors[i] = err
		}
		
		// Return them all
		for _, err := range errors {
			putError(err)
		}
		
		// Get new errors - should reuse the pooled ones
		for i := 0; i < numErrors; i++ {
			err := getError()
			if err.Type != "" || err.Message != "" || err.Details != nil {
				t.Errorf("Recycled error should be reset")
			}
			putError(err)
		}
	})
}

func BenchmarkPools(b *testing.B) {
	originalPooling := EnablePooling
	defer func() { EnablePooling = originalPooling }()
	
	b.Run("ErrorPool_Enabled", func(b *testing.B) {
		EnablePooling = true
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				err := getError()
				err.Type = "TestError"
				putError(err)
			}
		})
	})
	
	b.Run("ErrorPool_Disabled", func(b *testing.B) {
		EnablePooling = false
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				err := &MiddlewareError{}
				err.Type = "TestError"
				err.Reset()
			}
		})
	})
	
	b.Run("BufferPool_Enabled", func(b *testing.B) {
		EnablePooling = true
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := getBuffer()
				buf.WriteString("test")
				putBuffer(buf)
			}
		})
	})
	
	b.Run("BufferPool_Disabled", func(b *testing.B) {
		EnablePooling = false
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := &bytes.Buffer{}
				buf.WriteString("test")
				buf.Reset()
			}
		})
	})
	
	b.Run("ResponseWriterPool_Enabled", func(b *testing.B) {
		EnablePooling = true
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				w := httptest.NewRecorder()
				rw := getResponseWriter(w)
				rw.WriteHeader(http.StatusOK)
				putResponseWriter(rw)
			}
		})
	})
	
	b.Run("ResponseWriterPool_Disabled", func(b *testing.B) {
		EnablePooling = false
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				w := httptest.NewRecorder()
				rw := &responseWriter{
					ResponseWriter: w,
					statusCode:     http.StatusOK,
				}
				rw.WriteHeader(http.StatusOK)
				rw.Reset()
			}
		})
	})
	
	b.Run("RequestMetricPool_Enabled", func(b *testing.B) {
		EnablePooling = true
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metric := getRequestMetric()
				metric.Method = "GET"
				metric.StatusCode = 200
				putRequestMetric(metric)
			}
		})
	})
	
	b.Run("RequestMetricPool_Disabled", func(b *testing.B) {
		EnablePooling = false
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metric := &RequestMetric{}
				metric.Method = "GET"
				metric.StatusCode = 200
				metric.Reset()
			}
		})
	})
	
	b.Run("LimitedResponseRecorderPool_Enabled", func(b *testing.B) {
		EnablePooling = true
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				w := httptest.NewRecorder()
				recorder := getLimitedResponseRecorder(w, 1024)
				recorder.Write([]byte("test"))
				putLimitedResponseRecorder(recorder)
			}
		})
	})
	
	b.Run("LimitedResponseRecorderPool_Disabled", func(b *testing.B) {
		EnablePooling = false
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				w := httptest.NewRecorder()
				recorder := newLimitedResponseRecorder(w, 1024)
				recorder.Write([]byte("test"))
				// No pooling - just let GC handle it
			}
		})
	})
}

func BenchmarkPoolAllocation(b *testing.B) {
	originalPooling := EnablePooling
	defer func() { EnablePooling = originalPooling }()
	
	b.Run("WithPooling", func(b *testing.B) {
		EnablePooling = true
		b.ReportAllocs()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			// Simulate typical middleware usage
			err := getError()
			buf := getBuffer()
			w := httptest.NewRecorder()
			rw := getResponseWriter(w)
			metric := getRequestMetric()
			recorder := getLimitedResponseRecorder(w, 1024)
			
			// Use the objects
			err.Type = "TestError"
			buf.WriteString("test")
			rw.WriteHeader(http.StatusOK)
			metric.Method = "GET"
			recorder.Write([]byte("test"))
			
			// Return to pools
			putError(err)
			putBuffer(buf)
			putResponseWriter(rw)
			putRequestMetric(metric)
			putLimitedResponseRecorder(recorder)
		}
	})
	
	b.Run("WithoutPooling", func(b *testing.B) {
		EnablePooling = false
		b.ReportAllocs()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			// Simulate typical middleware usage without pooling
			err := &MiddlewareError{}
			buf := &bytes.Buffer{}
			w := httptest.NewRecorder()
			rw := newResponseWriter(w)
			metric := &RequestMetric{}
			recorder := newLimitedResponseRecorder(w, 1024)
			
			// Use the objects
			err.Type = "TestError"
			buf.WriteString("test")
			rw.WriteHeader(http.StatusOK)
			metric.Method = "GET"
			recorder.Write([]byte("test"))
			
			// No cleanup needed - GC will handle
		}
	})
}