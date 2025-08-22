package middleware

import (
	"context"
	"net/http"
	"strings"
	
	"github.com/willibrandon/mtlog/core"
)

// RequestLogger provides a fluent API for request-scoped logging
type RequestLogger struct {
	core.Logger
	r *http.Request
}

// GetRequestLogger retrieves or creates a logger for the request
func GetRequestLogger(r *http.Request) *RequestLogger {
	logger := FromContext(r.Context())
	if logger == nil {
		// Fallback to a no-op logger if none in context
		logger = &noOpLogger{}
	}
	return &RequestLogger{
		Logger: logger,
		r:      r,
	}
}

// WithUser adds user ID to the logger
func (rl *RequestLogger) WithUser(userID string) *RequestLogger {
	rl.Logger = rl.Logger.With("UserId", userID)
	return rl
}

// WithSession adds session ID to the logger
func (rl *RequestLogger) WithSession(sessionID string) *RequestLogger {
	rl.Logger = rl.Logger.With("SessionId", sessionID)
	return rl
}

// WithTenant adds tenant ID to the logger (for multi-tenant apps)
func (rl *RequestLogger) WithTenant(tenantID string) *RequestLogger {
	rl.Logger = rl.Logger.With("TenantId", tenantID)
	return rl
}

// WithOperation adds operation name to the logger
func (rl *RequestLogger) WithOperation(operation string) *RequestLogger {
	rl.Logger = rl.Logger.With("Operation", operation)
	return rl
}

// WithResource adds resource type/name to the logger
func (rl *RequestLogger) WithResource(resourceType, resourceID string) *RequestLogger {
	return &RequestLogger{
		Logger: rl.Logger.With("ResourceType", resourceType).With("ResourceId", resourceID),
		r:      rl.r,
	}
}

// WithError adds error details to the logger
func (rl *RequestLogger) WithError(err error) *RequestLogger {
	if err != nil {
		rl.Logger = rl.Logger.With("Error", err.Error()).With("ErrorType", strings.Split(err.Error(), ":")[0])
	}
	return rl
}

// Request returns the underlying HTTP request
func (rl *RequestLogger) Request() *http.Request {
	return rl.r
}

// Common Field Extractors - pre-defined extractors for common use cases

var (
	// UserIDFromHeader extracts user ID from X-User-ID header
	UserIDFromHeader = FieldExtractor{
		Name: "UserId",
		Extract: func(r *http.Request) any {
			if userID := r.Header.Get("X-User-ID"); userID != "" {
				return userID
			}
			return nil
		},
	}
	
	// UserIDFromAuthHeader extracts user ID from Authorization header (Bearer token)
	// This is a simple example - in production you'd parse the JWT
	UserIDFromAuthHeader = FieldExtractor{
		Name: "UserId",
		Extract: func(r *http.Request) any {
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				// In real app, you'd decode the JWT and extract user ID
				// This is just a placeholder
				return "user-from-token"
			}
			return nil
		},
	}
	
	// SessionIDFromCookie extracts session ID from session cookie
	SessionIDFromCookie = FieldExtractor{
		Name: "SessionId",
		Extract: func(r *http.Request) any {
			if cookie, err := r.Cookie("session_id"); err == nil {
				return cookie.Value
			}
			return nil
		},
	}
	
	// TraceIDFromContext extracts trace ID from context (for distributed tracing)
	TraceIDFromContext = FieldExtractor{
		Name: "TraceId",
		Extract: func(r *http.Request) any {
			if traceID := r.Context().Value("trace-id"); traceID != nil {
				return traceID
			}
			// Also check common headers
			if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
				return traceID
			}
			if traceID := r.Header.Get("X-B3-TraceId"); traceID != "" {
				return traceID
			}
			return nil
		},
	}
	
	// SpanIDFromContext extracts span ID from context
	SpanIDFromContext = FieldExtractor{
		Name: "SpanId",
		Extract: func(r *http.Request) any {
			if spanID := r.Context().Value("span-id"); spanID != nil {
				return spanID
			}
			if spanID := r.Header.Get("X-B3-SpanId"); spanID != "" {
				return spanID
			}
			return nil
		},
	}
	
	// TenantIDFromHeader extracts tenant ID for multi-tenant applications
	TenantIDFromHeader = FieldExtractor{
		Name: "TenantId",
		Extract: func(r *http.Request) any {
			if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
				return tenantID
			}
			return nil
		},
	}
	
	// TenantIDFromSubdomain extracts tenant ID from subdomain
	TenantIDFromSubdomain = FieldExtractor{
		Name: "TenantId",
		Extract: func(r *http.Request) any {
			host := r.Host
			if idx := strings.Index(host, "."); idx > 0 {
				subdomain := host[:idx]
				// Skip common subdomains that aren't tenants
				if subdomain != "www" && subdomain != "api" {
					return subdomain
				}
			}
			return nil
		},
	}
	
	// APIVersionFromHeader extracts API version from header
	APIVersionFromHeader = FieldExtractor{
		Name: "ApiVersion",
		Extract: func(r *http.Request) any {
			if version := r.Header.Get("X-API-Version"); version != "" {
				return version
			}
			// Also check Accept header for version
			if accept := r.Header.Get("Accept"); strings.Contains(accept, "version=") {
				parts := strings.Split(accept, "version=")
				if len(parts) > 1 {
					return strings.Split(parts[1], ";")[0]
				}
			}
			return nil
		},
	}
	
	// APIVersionFromPath extracts API version from URL path (e.g., /v1/users, /v2.1/items)
	APIVersionFromPath = FieldExtractor{
		Name: "ApiVersion",
		Extract: func(r *http.Request) any {
			path := r.URL.Path
			if strings.HasPrefix(path, "/v") {
				parts := strings.Split(path[1:], "/")
				if len(parts) > 0 && strings.HasPrefix(parts[0], "v") {
					version := parts[0]
					// Validate that it's a proper version format: v followed by numbers and optional dots
					// Examples: v1, v2, v1.0, v2.1, v1.0.0
					if len(version) > 1 {
						versionNum := version[1:] // Remove 'v' prefix
						isValid := true
						hasDigit := false
						
						for i, ch := range versionNum {
							if ch >= '0' && ch <= '9' {
								hasDigit = true
							} else if ch == '.' {
								// Dots are allowed but not at the start or end
								if i == 0 || i == len(versionNum)-1 {
									isValid = false
									break
								}
								// No consecutive dots
								if i > 0 && versionNum[i-1] == '.' {
									isValid = false
									break
								}
							} else {
								// Invalid character
								isValid = false
								break
							}
						}
						
						if isValid && hasDigit {
							return version
						}
					}
				}
			}
			return nil
		},
	}
	
	// ClientIDFromHeader extracts OAuth client ID
	ClientIDFromHeader = FieldExtractor{
		Name: "ClientId",
		Extract: func(r *http.Request) any {
			if clientID := r.Header.Get("X-Client-ID"); clientID != "" {
				return clientID
			}
			return nil
		},
	}
	
	// CorrelationIDFromHeader extracts correlation ID for request tracking
	CorrelationIDFromHeader = FieldExtractor{
		Name: "CorrelationId",
		Extract: func(r *http.Request) any {
			// Check multiple common headers
			for _, header := range []string{"X-Correlation-ID", "X-Request-ID", "X-Trace-ID"} {
				if id := r.Header.Get(header); id != "" {
					return id
				}
			}
			return nil
		},
	}
	
	// GeoLocationFromHeaders extracts geo information from CDN headers
	GeoLocationFromHeaders = FieldExtractor{
		Name: "GeoLocation",
		Extract: func(r *http.Request) any {
			location := make(map[string]string)
			
			// Cloudflare headers
			if country := r.Header.Get("CF-IPCountry"); country != "" {
				location["Country"] = country
			}
			
			// AWS CloudFront headers
			if country := r.Header.Get("CloudFront-Viewer-Country"); country != "" {
				location["Country"] = country
			}
			if region := r.Header.Get("CloudFront-Viewer-Country-Region"); region != "" {
				location["Region"] = region
			}
			
			// Fastly headers
			if country := r.Header.Get("X-Country-Code"); country != "" {
				location["Country"] = country
			}
			
			if len(location) > 0 {
				return location
			}
			return nil
		},
	}
	
	// DeviceTypeFromUserAgent extracts device type from User-Agent
	DeviceTypeFromUserAgent = FieldExtractor{
		Name: "DeviceType",
		Extract: func(r *http.Request) any {
			ua := strings.ToLower(r.UserAgent())
			switch {
			case strings.Contains(ua, "mobile"):
				return "mobile"
			case strings.Contains(ua, "tablet"):
				return "tablet"
			case strings.Contains(ua, "bot"):
				return "bot"
			case strings.Contains(ua, "curl") || strings.Contains(ua, "wget"):
				return "cli"
			default:
				if ua != "" {
					return "desktop"
				}
				return nil
			}
		},
	}
)

// CombineExtractors combines multiple extractors
func CombineExtractors(extractors ...FieldExtractor) []FieldExtractor {
	return extractors
}

// noOpLogger is a logger that does nothing (for fallback)
type noOpLogger struct{}

func (n *noOpLogger) Verbose(template string, args ...any)     {}
func (n *noOpLogger) Debug(template string, args ...any)       {}
func (n *noOpLogger) Information(template string, args ...any) {}
func (n *noOpLogger) Warning(template string, args ...any)     {}
func (n *noOpLogger) Error(template string, args ...any)       {}
func (n *noOpLogger) Fatal(template string, args ...any)       {}
func (n *noOpLogger) Write(level core.LogEventLevel, template string, args ...any) {}
func (n *noOpLogger) ForContext(propertyName string, value any) core.Logger { return n }
func (n *noOpLogger) WithContext(ctx context.Context) core.Logger { return n }
func (n *noOpLogger) With(args ...any) core.Logger { return n }
func (n *noOpLogger) IsEnabled(level core.LogEventLevel) bool { return false }
func (n *noOpLogger) Info(template string, args ...any) {}
func (n *noOpLogger) Warn(template string, args ...any) {}
func (n *noOpLogger) V(template string, args ...any) {}
func (n *noOpLogger) D(template string, args ...any) {}
func (n *noOpLogger) I(template string, args ...any) {}
func (n *noOpLogger) W(template string, args ...any) {}
func (n *noOpLogger) E(template string, args ...any) {}
func (n *noOpLogger) F(template string, args ...any) {}