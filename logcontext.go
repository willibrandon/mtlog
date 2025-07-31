package mtlog

import (
	"context"
	"maps"
)

// logContextKey is a private type for context keys to avoid collisions.
type logContextKey struct{}

// logContextValue holds the properties pushed to the context.
type logContextValue struct {
	properties map[string]any
}

// PushProperty adds a property to the context that will be included in all log events
// created from loggers using this context. Properties are inherited - if a context
// already has properties, the new context will include both the existing and new properties.
//
// Properties set via PushProperty have the lowest precedence and are overridden by
// ForContext properties and event-specific properties.
//
// Thread Safety: This function is thread-safe. It creates a new context value with a copy
// of the properties map, ensuring immutability. Multiple goroutines can safely call
// PushProperty on the same context, and each will receive its own independent context.
//
// Example:
//
//	ctx := context.Background()
//	ctx = mtlog.PushProperty(ctx, "UserId", 123)
//	ctx = mtlog.PushProperty(ctx, "TenantId", "acme-corp")
//
//	// Both UserId and TenantId will be included in this log
//	logger.WithContext(ctx).Information("Processing user request")
//
//	// Properties can be overridden at more specific scopes
//	logger.WithContext(ctx).ForContext("UserId", 456).Information("Override test")
//	// Results in UserId=456 (ForContext overrides PushProperty)
func PushProperty(ctx context.Context, name string, value any) context.Context {
	// Handle nil context
	if ctx == nil {
		ctx = context.Background()
	}

	// Get existing properties if any
	var properties map[string]any

	if existing := ctx.Value(logContextKey{}); existing != nil {
		if lcv, ok := existing.(*logContextValue); ok {
			// Copy existing properties
			properties = make(map[string]any, len(lcv.properties)+1)
			maps.Copy(properties, lcv.properties)
		}
	}

	if properties == nil {
		properties = make(map[string]any, 1)
	}

	// Add new property
	properties[name] = value

	// Create new context value
	return context.WithValue(ctx, logContextKey{}, &logContextValue{
		properties: properties,
	})
}

// getLogContextProperties extracts properties from the context that were added via PushProperty.
func getLogContextProperties(ctx context.Context) map[string]any {
	if ctx == nil {
		return nil
	}

	if value := ctx.Value(logContextKey{}); value != nil {
		if lcv, ok := value.(*logContextValue); ok {
			// Return a copy to prevent mutation
			props := make(map[string]any, len(lcv.properties))
			maps.Copy(props, lcv.properties)
			return props
		}
	}

	return nil
}
