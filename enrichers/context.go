package enrichers

import (
	"context"
	
	"github.com/willibrandon/mtlog/core"
)

// contextKey is the type for context keys used by the logging system.
type contextKey string

const (
	// CorrelationIdKey is the context key for correlation IDs.
	CorrelationIdKey contextKey = "correlationId"
	
	// RequestIdKey is the context key for request IDs.
	RequestIdKey contextKey = "requestId"
	
	// UserIdKey is the context key for user IDs.
	UserIdKey contextKey = "userId"
	
	// SessionIdKey is the context key for session IDs.
	SessionIdKey contextKey = "sessionId"
)

// ContextEnricher enriches log events with values from context.Context.
type ContextEnricher struct {
	ctx          context.Context
	propertyKeys map[contextKey]string
}

// NewContextEnricher creates an enricher that extracts values from the given context.
func NewContextEnricher(ctx context.Context) *ContextEnricher {
	return &ContextEnricher{
		ctx: ctx,
		propertyKeys: map[contextKey]string{
			CorrelationIdKey: "CorrelationId",
			RequestIdKey:     "RequestId",
			UserIdKey:        "UserId",
			SessionIdKey:     "SessionId",
		},
	}
}

// NewContextEnricherWithKeys creates an enricher with custom context key mappings.
func NewContextEnricherWithKeys(ctx context.Context, propertyKeys map[contextKey]string) *ContextEnricher {
	return &ContextEnricher{
		ctx:          ctx,
		propertyKeys: propertyKeys,
	}
}

// Enrich adds context values to the log event.
func (c *ContextEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	if c.ctx == nil {
		return
	}
	
	for ctxKey, propName := range c.propertyKeys {
		if value := c.ctx.Value(ctxKey); value != nil {
			prop := propertyFactory.CreateProperty(propName, value)
			event.Properties[prop.Name] = prop.Value
		}
	}
}

// ContextValueEnricher enriches log events with a specific value from context.
type ContextValueEnricher struct {
	ctx          context.Context
	key          interface{}
	propertyName string
}

// NewContextValueEnricher creates an enricher for a specific context value.
func NewContextValueEnricher(ctx context.Context, key interface{}, propertyName string) *ContextValueEnricher {
	return &ContextValueEnricher{
		ctx:          ctx,
		key:          key,
		propertyName: propertyName,
	}
}

// Enrich adds the context value to the log event.
func (c *ContextValueEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	if c.ctx == nil {
		return
	}
	
	if value := c.ctx.Value(c.key); value != nil {
		prop := propertyFactory.CreateProperty(c.propertyName, value)
		event.Properties[prop.Name] = prop.Value
	}
}

// WithCorrelationId returns a context with a correlation ID.
func WithCorrelationId(ctx context.Context, correlationId string) context.Context {
	return context.WithValue(ctx, CorrelationIdKey, correlationId)
}

// WithRequestId returns a context with a request ID.
func WithRequestId(ctx context.Context, requestId string) context.Context {
	return context.WithValue(ctx, RequestIdKey, requestId)
}

// WithUserId returns a context with a user ID.
func WithUserId(ctx context.Context, userId interface{}) context.Context {
	return context.WithValue(ctx, UserIdKey, userId)
}

// WithSessionId returns a context with a session ID.
func WithSessionId(ctx context.Context, sessionId string) context.Context {
	return context.WithValue(ctx, SessionIdKey, sessionId)
}