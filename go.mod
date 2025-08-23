module github.com/willibrandon/mtlog

go 1.21

// Exclude auto/sdk to avoid conflicting schema URL errors between different OTEL components.
// The auto/sdk package can introduce schema version conflicts when used alongside
// manual OTEL instrumentation, causing "conflicting Schema URL" errors at runtime.
exclude go.opentelemetry.io/auto/sdk v1.1.0

require (
	github.com/getsentry/sentry-go v0.35.1 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)
