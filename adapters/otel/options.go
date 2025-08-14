package otel

import (
	"context"

	"github.com/willibrandon/mtlog"
)

// WithOTELEnricher adds OpenTelemetry trace context enrichment.
// The enricher extracts trace.id, span.id, and trace.flags from the context.
func WithOTELEnricher(ctx context.Context) mtlog.Option {
	enricher := NewFastOTELEnricher(ctx)
	return mtlog.WithEnricher(enricher)
}

// WithStaticOTELEnricher adds OpenTelemetry trace context enrichment with cached values.
// Use this for request-scoped loggers where the trace context doesn't change.
func WithStaticOTELEnricher(ctx context.Context) mtlog.Option {
	enricher := NewStaticOTELEnricher(ctx)
	return mtlog.WithEnricher(enricher)
}

// WithOTLPSink adds an OTLP sink for exporting logs to OpenTelemetry collectors.
func WithOTLPSink(endpoint string, opts ...OTLPOption) mtlog.Option {
	sinkOpts := append([]OTLPOption{
		WithOTLPEndpoint(endpoint),
		WithOTLPTransport(OTLPTransportGRPC),
	}, opts...)
	
	sink, err := NewOTLPSink(sinkOpts...)
	if err != nil {
		// Return a no-op sink that logs the error to selflog
		return mtlog.WithSink(&errorSink{err: err})
	}
	return mtlog.WithSink(sink)
}

// WithOTLPGRPC adds an OTLP sink using gRPC transport.
func WithOTLPGRPC(endpoint string, opts ...OTLPOption) mtlog.Option {
	return WithOTLPSink(endpoint, append([]OTLPOption{WithOTLPTransport(OTLPTransportGRPC)}, opts...)...)
}

// WithOTLPHTTP adds an OTLP sink using HTTP transport.
func WithOTLPHTTP(endpoint string, opts ...OTLPOption) mtlog.Option {
	return WithOTLPSink(endpoint, append([]OTLPOption{WithOTLPTransport(OTLPTransportHTTP)}, opts...)...)
}