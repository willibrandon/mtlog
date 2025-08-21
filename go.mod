module github.com/willibrandon/mtlog

go 1.23.0

toolchain go1.24.5

// Exclude auto/sdk to avoid conflicting schema URL errors between different OTEL components.
// The auto/sdk package can introduce schema version conflicts when used alongside
// manual OTEL instrumentation, causing "conflicting Schema URL" errors at runtime.
exclude go.opentelemetry.io/auto/sdk v1.1.0

replace github.com/willibrandon/mtlog => .

require (
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
)
