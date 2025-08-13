package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/willibrandon/mtlog"
	otelmtlog "github.com/willibrandon/mtlog/adapters/otel"
)

func main() {
	fmt.Println("OpenTelemetry TLS Configuration Examples")
	fmt.Println("========================================")

	ctx := context.Background()

	// Example 1: Insecure connection (for testing)
	example1Insecure(ctx)
	
	// Example 2: Custom TLS configuration
	example2CustomTLS(ctx)
	
	// Example 3: Skip certificate verification (insecure)
	example3SkipVerify(ctx)
	
	// Example 4: Client certificate authentication
	example4ClientCert(ctx)
	
	// Example 5: Custom CA certificate
	example5CustomCA(ctx)
	
	fmt.Println("\n✅ TLS examples completed!")
}

func example1Insecure(ctx context.Context) {
	fmt.Println("\n=== Example 1: Insecure Connection ===")
	
	// For development/testing - disables TLS
	sink, err := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint("localhost:4317"),
		otelmtlog.WithOTLPInsecure(), // Explicitly disable TLS
		otelmtlog.WithOTLPBatching(10, 2*time.Second),
	)
	if err != nil {
		fmt.Printf("Failed to create insecure sink: %v\n", err)
		return
	}
	defer sink.Close()
	
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(sink),
		mtlog.WithConsole(),
		mtlog.WithProperty("example", "insecure"),
	)
	
	logger.Information("Using insecure connection for development")
	logger.Warning("This should only be used in development environments")
	
	fmt.Println("Insecure connection example completed")
}

func example2CustomTLS(ctx context.Context) {
	fmt.Println("\n=== Example 2: Custom TLS Configuration ===")
	
	// Create custom TLS config with specific settings
	tlsConfig := &tls.Config{
		ServerName: "otel-collector.example.com",
		MinVersion: tls.VersionTLS12,
		// In production, you would not set InsecureSkipVerify
		InsecureSkipVerify: true, // Only for demo purposes
	}
	
	sink, err := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint("otel-collector.example.com:4317"),
		otelmtlog.WithOTLPTLSConfig(tlsConfig),
		otelmtlog.WithOTLPBatching(10, 2*time.Second),
	)
	if err != nil {
		fmt.Printf("Failed to create TLS sink: %v\n", err)
		return
	}
	defer sink.Close()
	
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(sink),
		mtlog.WithConsole(),
		mtlog.WithProperty("example", "custom_tls"),
	)
	
	logger.Information("Using custom TLS configuration")
	logger.Information("Server name: {ServerName}", tlsConfig.ServerName)
	logger.Information("Min TLS version: {MinVersion}", tlsConfig.MinVersion)
	
	fmt.Println("Custom TLS configuration example completed")
}

func example3SkipVerify(ctx context.Context) {
	fmt.Println("\n=== Example 3: Skip Certificate Verification ===")
	
	// Skip certificate verification - useful for self-signed certificates
	// WARNING: This is insecure and should only be used in development
	sink, err := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint("self-signed.example.com:4317"),
		otelmtlog.WithOTLPSkipVerify(), // Skip certificate verification
		otelmtlog.WithOTLPBatching(10, 2*time.Second),
	)
	if err != nil {
		fmt.Printf("Failed to create skip-verify sink: %v\n", err)
		return
	}
	defer sink.Close()
	
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(sink),
		mtlog.WithConsole(),
		mtlog.WithProperty("example", "skip_verify"),
	)
	
	logger.Warning("Skipping certificate verification - INSECURE")
	logger.Information("This should only be used with self-signed certificates in development")
	
	fmt.Println("Skip verification example completed")
}

func example4ClientCert(ctx context.Context) {
	fmt.Println("\n=== Example 4: Client Certificate Authentication ===")
	
	// For mutual TLS (mTLS) authentication
	// Note: This example will fail without actual certificate files
	sink, err := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint("mtls.example.com:4317"),
		otelmtlog.WithOTLPClientCert("client.crt", "client.key"), // Client certificate
		otelmtlog.WithOTLPBatching(10, 2*time.Second),
	)
	if err != nil {
		fmt.Printf("Expected failure - no client certificates: %v\n", err)
		fmt.Println("In production, provide actual certificate files:")
		fmt.Println("  - client.crt: Client certificate file")
		fmt.Println("  - client.key: Client private key file")
		return
	}
	defer sink.Close()
	
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(sink),
		mtlog.WithConsole(),
		mtlog.WithProperty("example", "client_cert"),
	)
	
	logger.Information("Using client certificate for mutual TLS")
	
	fmt.Println("Client certificate example completed")
}

func example5CustomCA(ctx context.Context) {
	fmt.Println("\n=== Example 5: Custom CA Certificate ===")
	
	// Use custom CA for server verification
	// Note: This example will fail without actual CA file
	sink, err := otelmtlog.NewOTLPSink(
		otelmtlog.WithOTLPEndpoint("private-ca.example.com:4317"),
		otelmtlog.WithOTLPCACert("custom-ca.crt"), // Custom CA certificate
		otelmtlog.WithOTLPBatching(10, 2*time.Second),
	)
	if err != nil {
		fmt.Printf("Expected failure - no CA certificate: %v\n", err)
		fmt.Println("In production, provide actual CA certificate file:")
		fmt.Println("  - custom-ca.crt: CA certificate to trust")
		return
	}
	defer sink.Close()
	
	logger := mtlog.New(
		otelmtlog.WithOTELEnricher(ctx),
		mtlog.WithSink(sink),
		mtlog.WithConsole(),
		mtlog.WithProperty("example", "custom_ca"),
	)
	
	logger.Information("Using custom CA certificate")
	
	fmt.Println("Custom CA example completed")
}

// Production example showing comprehensive TLS setup
func productionTLSExample() {
	fmt.Println("\n=== Production TLS Example ===")
	fmt.Println("// Production-ready TLS configuration:")
	fmt.Println(`
sink, err := otelmtlog.NewOTLPSink(
	otelmtlog.WithOTLPEndpoint("otel-collector.company.com:4317"),
	otelmtlog.WithOTLPClientCert("client.crt", "client.key"), // mTLS
	otelmtlog.WithOTLPCACert("company-ca.crt"),              // Custom CA
	otelmtlog.WithOTLPHeaders(map[string]string{
		"api-key": "your-api-key",
	}),
	otelmtlog.WithOTLPBatching(100, 5*time.Second),
	otelmtlog.WithOTLPRetry(time.Second, 30*time.Second),
)
	`)
	
	fmt.Println("Environment variables for TLS:")
	fmt.Println("  OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector.company.com:4317")
	fmt.Println("  OTEL_EXPORTER_OTLP_HEADERS=api-key=your-api-key")
}