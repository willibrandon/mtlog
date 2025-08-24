package sentry

import (
	"context"
	"time"

	"github.com/getsentry/sentry-go"
)

// transactionKey is the context key for Sentry transactions
type transactionKey struct{}

// spanKey is the context key for Sentry spans
type spanKey struct{}

// StartTransaction starts a new Sentry transaction and returns a context with it.
func StartTransaction(ctx context.Context, name string, operation string) context.Context {
	span := sentry.StartTransaction(ctx, name)
	span.Op = operation
	ctx = span.Context()
	ctx = context.WithValue(ctx, transactionKey{}, span)
	return ctx
}

// StartSpan starts a new span within the current transaction.
func StartSpan(ctx context.Context, operation string) (context.Context, func()) {
	span := sentry.StartSpan(ctx, operation)
	ctx = span.Context()
	ctx = context.WithValue(ctx, spanKey{}, span)
	
	return ctx, span.Finish
}

// GetTransaction retrieves the current transaction from context.
func GetTransaction(ctx context.Context) *sentry.Span {
	if span, ok := ctx.Value(transactionKey{}).(*sentry.Span); ok {
		return span
	}
	// Try to get from Sentry's internal context
	if span := sentry.SpanFromContext(ctx); span != nil && span.IsTransaction() {
		return span
	}
	return nil
}

// GetSpan retrieves the current span from context.
func GetSpan(ctx context.Context) *sentry.Span {
	if span, ok := ctx.Value(spanKey{}).(*sentry.Span); ok {
		return span
	}
	return GetTransaction(ctx)
}

// WithTransaction creates a transaction-aware logger context.
func WithTransaction(ctx context.Context, name string, operation string) context.Context {
	return StartTransaction(ctx, name, operation)
}

// RecordBreadcrumbInTransaction adds a breadcrumb to the current transaction.
func RecordBreadcrumbInTransaction(ctx context.Context, breadcrumb sentry.Breadcrumb) {
	if span := GetSpan(ctx); span != nil {
		span.SetContext("breadcrumb", sentry.Context{
			"message":  breadcrumb.Message,
			"category": breadcrumb.Category,
			"level":    string(breadcrumb.Level),
		})
	}
}

// SetSpanStatus sets the status of the current span.
func SetSpanStatus(ctx context.Context, status string) {
	if span := GetSpan(ctx); span != nil {
		// Set status as data since SpanStatus may not be directly settable
		span.SetData("status", status)
	}
}

// SetSpanTag sets a tag on the current span.
func SetSpanTag(ctx context.Context, key, value string) {
	if span := GetSpan(ctx); span != nil {
		span.SetTag(key, value)
	}
}

// SetSpanData sets data on the current span.
func SetSpanData(ctx context.Context, key string, value interface{}) {
	if span := GetSpan(ctx); span != nil {
		span.SetData(key, value)
	}
}

// MeasureSpan measures the duration of an operation and records it as a span.
func MeasureSpan(ctx context.Context, operation string, fn func() error) error {
	spanCtx, finish := StartSpan(ctx, operation)
	defer finish()

	err := fn()
	if err != nil {
		SetSpanStatus(spanCtx, "internal_error")
		SetSpanData(spanCtx, "error", err.Error())
	} else {
		SetSpanStatus(spanCtx, "ok")
	}

	return err
}

// TraceHTTPRequest creates a span for an HTTP request.
func TraceHTTPRequest(ctx context.Context, method, url string) (context.Context, func(statusCode int)) {
	operation := "http.client"
	spanCtx, finish := StartSpan(ctx, operation)

	SetSpanData(spanCtx, "http.method", method)
	SetSpanData(spanCtx, "http.url", url)

	return spanCtx, func(statusCode int) {
		SetSpanData(spanCtx, "http.status_code", statusCode)
		if statusCode >= 400 {
			SetSpanStatus(spanCtx, "failed_precondition")
		} else {
			SetSpanStatus(spanCtx, "ok")
		}
		finish()
	}
}

// TraceDatabaseQuery creates a span for a database query.
func TraceDatabaseQuery(ctx context.Context, query string, dbName string) (context.Context, func(error)) {
	operation := "db.query"
	spanCtx, finish := StartSpan(ctx, operation)

	SetSpanData(spanCtx, "db.name", dbName)
	SetSpanData(spanCtx, "db.statement", query)
	SetSpanData(spanCtx, "db.system", "sql")

	return spanCtx, func(err error) {
		if err != nil {
			SetSpanStatus(spanCtx, "internal_error")
			SetSpanData(spanCtx, "error", err.Error())
		} else {
			SetSpanStatus(spanCtx, "ok")
		}
		finish()
	}
}

// TraceCache creates a span for cache operations.
func TraceCache(ctx context.Context, operation, key string) (context.Context, func(hit bool)) {
	spanOp := "cache." + operation
	spanCtx, finish := StartSpan(ctx, spanOp)

	SetSpanData(spanCtx, "cache.key", key)
	SetSpanData(spanCtx, "cache.operation", operation)

	return spanCtx, func(hit bool) {
		SetSpanData(spanCtx, "cache.hit", hit)
		if hit {
			SetSpanTag(spanCtx, "cache.hit", "true")
		} else {
			SetSpanTag(spanCtx, "cache.hit", "false")
		}
		SetSpanStatus(spanCtx, "ok")
		finish()
	}
}

// enrichEventFromTransaction enriches a Sentry event with transaction data.
func enrichEventFromTransaction(ctx context.Context, event *sentry.Event) {
	if span := GetSpan(ctx); span != nil {
		event.Transaction = span.Name
		if traceID := span.TraceID; traceID != (sentry.TraceID{}) {
			event.Contexts["trace"] = sentry.Context{
				"trace_id":       traceID.String(),
				"span_id":        span.SpanID.String(),
				"parent_span_id": span.ParentSpanID.String(),
			}
		}

		// Add performance data
		if span.EndTime.After(span.StartTime) {
			duration := span.EndTime.Sub(span.StartTime)
			event.Extra["transaction.duration_ms"] = duration.Milliseconds()
		}
	}
}

// TransactionMiddleware wraps a handler with transaction tracking.
func TransactionMiddleware(name string) func(context.Context, func(context.Context) error) error {
	return func(ctx context.Context, next func(context.Context) error) error {
		txCtx := StartTransaction(ctx, name, "handler")
		defer func() {
			if tx := GetTransaction(txCtx); tx != nil {
				tx.Finish()
			}
		}()

		err := next(txCtx)
		if err != nil {
			SetSpanStatus(txCtx, "internal_error")
		} else {
			SetSpanStatus(txCtx, "ok")
		}

		return err
	}
}

// BatchSpan creates a span for batch operations with item count tracking.
func BatchSpan(ctx context.Context, operation string, itemCount int, fn func() error) error {
	spanCtx, finish := StartSpan(ctx, operation)
	defer finish()

	SetSpanData(spanCtx, "batch.size", itemCount)
	start := time.Now()

	err := fn()

	duration := time.Since(start)
	SetSpanData(spanCtx, "batch.duration_ms", duration.Milliseconds())
	if itemCount > 0 {
		SetSpanData(spanCtx, "batch.items_per_second", float64(itemCount)/duration.Seconds())
	}

	if err != nil {
		SetSpanStatus(spanCtx, "internal_error")
		SetSpanData(spanCtx, "error", err.Error())
	} else {
		SetSpanStatus(spanCtx, "ok")
	}

	return err
}