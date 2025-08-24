package sentry

import (
	"context"

	"github.com/getsentry/sentry-go"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const (
	// userContextKey is the context key for Sentry user information.
	userContextKey contextKey = "sentry.user"

	// tagsContextKey is the context key for Sentry tags.
	tagsContextKey contextKey = "sentry.tags"

	// contextContextKey is the context key for Sentry contexts.
	contextContextKey contextKey = "sentry.context"
)

// WithUser adds Sentry user information to the context.
func WithUser(ctx context.Context, user sentry.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext extracts Sentry user information from the context.
func UserFromContext(ctx context.Context) (sentry.User, bool) {
	user, ok := ctx.Value(userContextKey).(sentry.User)
	return user, ok
}

// WithTags adds Sentry tags to the context.
func WithTags(ctx context.Context, tags map[string]string) context.Context {
	existing := TagsFromContext(ctx)
	merged := make(map[string]string, len(existing)+len(tags))
	
	// Copy existing tags
	for k, v := range existing {
		merged[k] = v
	}
	
	// Add new tags
	for k, v := range tags {
		merged[k] = v
	}
	
	return context.WithValue(ctx, tagsContextKey, merged)
}

// TagsFromContext extracts Sentry tags from the context.
func TagsFromContext(ctx context.Context) map[string]string {
	tags, ok := ctx.Value(tagsContextKey).(map[string]string)
	if !ok {
		return make(map[string]string)
	}
	return tags
}

// WithContext adds Sentry context data to the context.
func WithContext(ctx context.Context, key string, data interface{}) context.Context {
	contexts := ContextsFromContext(ctx)
	contexts[key] = data
	return context.WithValue(ctx, contextContextKey, contexts)
}

// ContextsFromContext extracts Sentry contexts from the context.
func ContextsFromContext(ctx context.Context) map[string]interface{} {
	contexts, ok := ctx.Value(contextContextKey).(map[string]interface{})
	if !ok {
		return make(map[string]interface{})
	}
	return contexts
}

// enrichEventFromContext enriches a Sentry event with context data.
func enrichEventFromContext(ctx context.Context, event *sentry.Event) {
	// Add user if present
	if user, ok := UserFromContext(ctx); ok {
		event.User = user
	}

	// Add tags if present
	if tags := TagsFromContext(ctx); len(tags) > 0 {
		if event.Tags == nil {
			event.Tags = make(map[string]string)
		}
		for k, v := range tags {
			event.Tags[k] = v
		}
	}

	// Add contexts if present
	if contexts := ContextsFromContext(ctx); len(contexts) > 0 {
		if event.Contexts == nil {
			event.Contexts = make(map[string]sentry.Context)
		}
		for k, v := range contexts {
			// Convert to sentry.Context if needed
			if sentryCtx, ok := v.(sentry.Context); ok {
				event.Contexts[k] = sentryCtx
			} else if mapCtx, ok := v.(map[string]interface{}); ok {
				event.Contexts[k] = sentry.Context(mapCtx)
			}
		}
	}

	// Add transaction/span information
	enrichEventFromTransaction(ctx, event)
}