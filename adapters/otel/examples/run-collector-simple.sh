#!/bin/bash

# Simple script to run just the OTEL collector with minimal config

echo "Starting OpenTelemetry Collector (simple mode)..."

# Create a simple config that just prints to console
cat > /tmp/otel-simple-config.yaml <<EOF
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
    metrics:
      receivers: [otlp]
      exporters: [debug]
    logs:
      receivers: [otlp]
      exporters: [debug]
EOF

# Run collector in foreground
docker run --rm \
  -p 4317:4317 \
  -p 4318:4318 \
  -v /tmp/otel-simple-config.yaml:/etc/otel-config.yaml \
  otel/opentelemetry-collector:latest \
  --config=/etc/otel-config.yaml