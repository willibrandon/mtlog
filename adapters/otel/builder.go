package otel

import (
	"context"
	"os"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

// Builder provides a convenient way to create mtlog loggers with OTEL support
type Builder struct {
	ctx            context.Context
	endpoint       string
	transport      OTLPTransport
	enricherType   string
	batchSize      int
	batchTimeout   time.Duration
	minimumLevel   core.LogEventLevel
	consoleEnabled bool
	headers        map[string]string
	options        []mtlog.Option
}

// NewBuilder creates a new OTEL logger builder
func NewBuilder(ctx context.Context) *Builder {
	return &Builder{
		ctx:          ctx,
		enricherType: "fast",
		batchSize:    100,
		batchTimeout: 5 * time.Second,
		minimumLevel: core.InformationLevel,
		transport:    OTLPTransportGRPC,
	}
}

// WithEndpoint sets the OTLP endpoint
func (b *Builder) WithEndpoint(endpoint string) *Builder {
	b.endpoint = endpoint
	return b
}

// WithEndpointFromEnv reads endpoint from environment variable
func (b *Builder) WithEndpointFromEnv() *Builder {
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		b.endpoint = endpoint
	}
	return b
}

// WithGRPC uses gRPC transport (default)
func (b *Builder) WithGRPC() *Builder {
	b.transport = OTLPTransportGRPC
	return b
}

// WithHTTP uses HTTP transport
func (b *Builder) WithHTTP() *Builder {
	b.transport = OTLPTransportHTTP
	return b
}

// WithFastEnricher uses the fast enricher (default)
func (b *Builder) WithFastEnricher() *Builder {
	b.enricherType = "fast"
	return b
}

// WithStaticEnricher uses the static enricher (best for request-scoped loggers)
func (b *Builder) WithStaticEnricher() *Builder {
	b.enricherType = "static"
	return b
}

// WithCachingEnricher uses the caching enricher
func (b *Builder) WithCachingEnricher() *Builder {
	b.enricherType = "caching"
	return b
}

// WithBatching configures batching parameters
func (b *Builder) WithBatching(size int, timeout time.Duration) *Builder {
	b.batchSize = size
	b.batchTimeout = timeout
	return b
}

// WithConsole enables console output
func (b *Builder) WithConsole() *Builder {
	b.consoleEnabled = true
	return b
}

// WithMinimumLevel sets the minimum log level
func (b *Builder) WithMinimumLevel(level core.LogEventLevel) *Builder {
	b.minimumLevel = level
	return b
}

// WithHeaders adds custom headers for OTLP requests
func (b *Builder) WithHeaders(headers map[string]string) *Builder {
	b.headers = headers
	return b
}

// WithOption adds a custom mtlog option
func (b *Builder) WithOption(opt mtlog.Option) *Builder {
	b.options = append(b.options, opt)
	return b
}

// Build creates the configured logger
func (b *Builder) Build() (core.Logger, error) {
	opts := make([]mtlog.Option, 0)

	// Add enricher based on type
	switch b.enricherType {
	case "static":
		opts = append(opts, mtlog.WithEnricher(NewStaticOTELEnricher(b.ctx)))
	case "caching":
		opts = append(opts, mtlog.WithEnricher(NewOTELEnricher(b.ctx)))
	default:
		opts = append(opts, WithOTELEnricher(b.ctx))
	}

	// Add OTLP sink if endpoint is configured
	if b.endpoint != "" {
		sink, err := NewOTLPSink(
			WithOTLPEndpoint(b.endpoint),
			WithOTLPTransport(b.transport),
			WithOTLPBatching(b.batchSize, b.batchTimeout),
			WithOTLPHeaders(b.headers),
		)
		if err != nil {
			return nil, err
		}
		opts = append(opts, mtlog.WithSink(sink))
	}

	// Add console if enabled
	if b.consoleEnabled {
		opts = append(opts, mtlog.WithConsole())
	}

	// Add minimum level
	opts = append(opts, mtlog.WithMinimumLevel(b.minimumLevel))

	// Add custom options
	opts = append(opts, b.options...)

	return mtlog.New(opts...), nil
}

// MustBuild creates the configured logger or panics on error
func (b *Builder) MustBuild() core.Logger {
	logger, err := b.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

// NewOTELLogger creates a logger with sensible OTEL defaults
// This is a convenience function for the most common use case
func NewOTELLogger(ctx context.Context, opts ...mtlog.Option) core.Logger {
	// Get endpoint from environment or use default
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4317"
	}

	// Build default options
	defaultOpts := []mtlog.Option{
		WithOTELEnricher(ctx),
	}

	// Only add OTLP sink if we can create it
	if sink, err := NewOTLPSink(
		WithOTLPEndpoint(endpoint),
		WithOTLPBatching(100, 5*time.Second),
	); err == nil {
		defaultOpts = append(defaultOpts, mtlog.WithSink(sink))
	} else {
		// Log to selflog if available
		sinkLog.Warn("Failed to create OTLP sink: %v", err)
	}

	// Add console by default
	defaultOpts = append(defaultOpts, mtlog.WithConsole())

	// Apply user options last to allow overrides
	return mtlog.New(append(defaultOpts, opts...)...)
}

// NewRequestLogger creates a per-request logger with trace context
// This is optimized for HTTP request handling
func NewRequestLogger(ctx context.Context, requestID string, opts ...mtlog.Option) core.Logger {
	// Use static enricher for request-scoped loggers
	defaultOpts := []mtlog.Option{
		mtlog.WithEnricher(NewStaticOTELEnricher(ctx)),
		mtlog.WithProperty("request_id", requestID),
		mtlog.WithConsole(),
	}

	// Check for OTLP endpoint
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		if sink, err := NewOTLPSink(
			WithOTLPEndpoint(endpoint),
			WithOTLPBatching(50, 2*time.Second),
		); err == nil {
			defaultOpts = append(defaultOpts, mtlog.WithSink(sink))
		}
	}

	return mtlog.New(append(defaultOpts, opts...)...)
}