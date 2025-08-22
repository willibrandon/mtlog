package middleware

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

// errorPool is a pool for MiddlewareError structs to reduce allocations
var errorPool = sync.Pool{
	New: func() any {
		return &MiddlewareError{}
	},
}

// getError retrieves a MiddlewareError from the pool
func getError() *MiddlewareError {
	return errorPool.Get().(*MiddlewareError)
}

// putError returns a MiddlewareError to the pool after resetting it
func putError(err *MiddlewareError) {
	if err == nil {
		return
	}
	err.Reset()
	errorPool.Put(err)
}

// Reset clears all fields of the MiddlewareError for reuse
func (e *MiddlewareError) Reset() {
	e.Type = ""
	e.Message = ""
	e.Cause = nil
	e.StatusCode = 0
	e.RequestID = ""
	e.Path = ""
	e.Method = ""
	e.StackTrace = ""
	e.Details = nil
}

// responseWriterPool is a pool for responseWriter wrappers
var responseWriterPool = sync.Pool{
	New: func() any {
		return &responseWriter{
			statusCode: http.StatusOK,
		}
	},
}

// getResponseWriter retrieves a responseWriter from the pool
func getResponseWriter(w http.ResponseWriter) *responseWriter {
	rw := responseWriterPool.Get().(*responseWriter)
	rw.ResponseWriter = w
	rw.statusCode = http.StatusOK
	rw.written = false
	rw.size = 0
	return rw
}

// putResponseWriter returns a responseWriter to the pool after resetting it
func putResponseWriter(rw *responseWriter) {
	if rw == nil {
		return
	}
	rw.Reset()
	responseWriterPool.Put(rw)
}

// Reset clears all fields of the responseWriter for reuse
func (rw *responseWriter) Reset() {
	rw.ResponseWriter = nil
	rw.statusCode = http.StatusOK
	rw.written = false
	rw.size = 0
}

// requestMetricPool is a pool for RequestMetric structs
var requestMetricPool = sync.Pool{
	New: func() any {
		return &RequestMetric{}
	},
}

// getRequestMetric retrieves a RequestMetric from the pool
func getRequestMetric() *RequestMetric {
	return requestMetricPool.Get().(*RequestMetric)
}

// putRequestMetric returns a RequestMetric to the pool after resetting it
func putRequestMetric(m *RequestMetric) {
	if m == nil {
		return
	}
	m.Reset()
	requestMetricPool.Put(m)
}

// Reset clears all fields of the RequestMetric for reuse
func (m *RequestMetric) Reset() {
	m.Method = ""
	m.Path = ""
	m.StatusCode = 0
	m.Duration = 0
	m.Timestamp = time.Time{}
}

// bufferPool is a pool for bytes.Buffer used in body capture
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// getBuffer retrieves a buffer from the pool
func getBuffer() *bytes.Buffer {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// putBuffer returns a buffer to the pool after resetting it
func putBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	buf.Reset()
	bufferPool.Put(buf)
}

// limitedResponseRecorderPool is a pool for limitedResponseRecorder structs
var limitedResponseRecorderPool = sync.Pool{
	New: func() any {
		return &limitedResponseRecorder{}
	},
}

// getLimitedResponseRecorder retrieves a limitedResponseRecorder from the pool
func getLimitedResponseRecorder(w http.ResponseWriter, maxSize int) *limitedResponseRecorder {
	recorder := limitedResponseRecorderPool.Get().(*limitedResponseRecorder)
	recorder.ResponseWriter = w
	recorder.statusCode = http.StatusOK
	recorder.body = getBuffer()
	recorder.maxSize = maxSize
	recorder.written = false
	return recorder
}

// putLimitedResponseRecorder returns a limitedResponseRecorder to the pool
func putLimitedResponseRecorder(recorder *limitedResponseRecorder) {
	if recorder == nil {
		return
	}
	if recorder.body != nil {
		putBuffer(recorder.body)
	}
	recorder.Reset()
	limitedResponseRecorderPool.Put(recorder)
}

// Reset clears all fields of the limitedResponseRecorder for reuse
func (r *limitedResponseRecorder) Reset() {
	r.ResponseWriter = nil
	r.statusCode = http.StatusOK
	r.body = nil
	r.maxSize = 0
	r.written = false
}

// EnablePooling controls whether object pooling is enabled
var EnablePooling = true

// PoolStats provides statistics about pool usage
type PoolStats struct {
	ErrorPoolHits    uint64
	ErrorPoolMisses  uint64
	WriterPoolHits   uint64
	WriterPoolMisses uint64
	MetricPoolHits   uint64
	MetricPoolMisses uint64
	BufferPoolHits   uint64
	BufferPoolMisses uint64
}

// poolStats tracks pool usage statistics
var poolStats PoolStats
var poolStatsMu sync.RWMutex

// GetPoolStats returns current pool usage statistics
func GetPoolStats() PoolStats {
	poolStatsMu.RLock()
	defer poolStatsMu.RUnlock()
	return poolStats
}

// ResetPoolStats resets all pool statistics
func ResetPoolStats() {
	poolStatsMu.Lock()
	defer poolStatsMu.Unlock()
	poolStats = PoolStats{}
}