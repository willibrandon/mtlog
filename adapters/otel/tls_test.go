package otel_test

import (
	"crypto/tls"
	"strings"
	"testing"

	mtlogotel "github.com/willibrandon/mtlog/adapters/otel"
)

func TestTLSConfiguration(t *testing.T) {
	t.Run("Insecure connection", func(t *testing.T) {
		sink, err := mtlogotel.NewOTLPSink(
			mtlogotel.WithOTLPEndpoint("localhost:4317"),
			mtlogotel.WithOTLPInsecure(),
		)
		if err != nil {
			t.Fatalf("Failed to create insecure sink: %v", err)
		}
		defer sink.Close()
		
		// Test that sink was created successfully
		if sink == nil {
			t.Fatal("Expected sink to be created")
		}
	})
	
	t.Run("Custom TLS config", func(t *testing.T) {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:        "test-server",
		}
		
		sink, err := mtlogotel.NewOTLPSink(
			mtlogotel.WithOTLPEndpoint("localhost:4317"),
			mtlogotel.WithOTLPTLSConfig(tlsConfig),
		)
		if err != nil {
			t.Fatalf("Failed to create sink with TLS config: %v", err)
		}
		defer sink.Close()
		
		if sink == nil {
			t.Fatal("Expected sink to be created")
		}
	})
	
	t.Run("Skip verify option", func(t *testing.T) {
		sink, err := mtlogotel.NewOTLPSink(
			mtlogotel.WithOTLPEndpoint("localhost:4317"),
			mtlogotel.WithOTLPSkipVerify(),
		)
		if err != nil {
			t.Fatalf("Failed to create sink with skip verify: %v", err)
		}
		defer sink.Close()
		
		if sink == nil {
			t.Fatal("Expected sink to be created")
		}
	})
	
	t.Run("HTTP transport with TLS", func(t *testing.T) {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		
		sink, err := mtlogotel.NewOTLPSink(
			mtlogotel.WithOTLPEndpoint("localhost:4318"),
			mtlogotel.WithOTLPTransport(mtlogotel.OTLPTransportHTTP),
			mtlogotel.WithOTLPTLSConfig(tlsConfig),
		)
		if err != nil {
			t.Fatalf("Failed to create HTTP sink with TLS: %v", err)
		}
		defer sink.Close()
		
		if sink == nil {
			t.Fatal("Expected HTTP sink to be created")
		}
	})
	
	t.Run("Combined TLS options", func(t *testing.T) {
		// Test that multiple TLS options work together
		sink, err := mtlogotel.NewOTLPSink(
			mtlogotel.WithOTLPEndpoint("secure.example.com:4317"),
			mtlogotel.WithOTLPTLSConfig(&tls.Config{ServerName: "secure.example.com"}),
			mtlogotel.WithOTLPSkipVerify(), // This should override the above
		)
		if err != nil {
			t.Fatalf("Failed to create sink with combined TLS options: %v", err)
		}
		defer sink.Close()
		
		if sink == nil {
			t.Fatal("Expected sink to be created")
		}
	})
}

func TestTLSConfigurationErrors(t *testing.T) {
	t.Run("Invalid client certificate", func(t *testing.T) {
		_, err := mtlogotel.NewOTLPSink(
			mtlogotel.WithOTLPEndpoint("localhost:4317"),
			mtlogotel.WithOTLPClientCert("/nonexistent/cert.pem", "/nonexistent/key.pem"),
		)
		if err == nil {
			t.Fatal("Expected error for invalid client certificate")
		}
		
		if !strings.Contains(err.Error(), "failed to load client certificate") {
			t.Errorf("Expected certificate error, got: %v", err)
		}
	})
	
	t.Run("Invalid CA certificate", func(t *testing.T) {
		_, err := mtlogotel.NewOTLPSink(
			mtlogotel.WithOTLPEndpoint("localhost:4317"),
			mtlogotel.WithOTLPCACert("/nonexistent/ca.pem"),
		)
		if err == nil {
			t.Fatal("Expected error for invalid CA certificate")
		}
		
		if !strings.Contains(err.Error(), "failed to read CA certificate") {
			t.Errorf("Expected CA certificate error, got: %v", err)
		}
	})
}