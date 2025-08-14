#!/bin/bash

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting OpenTelemetry Collector and Jaeger...${NC}"

# Start the collector and Jaeger
docker-compose up -d

# Wait for services to be ready
echo -e "${YELLOW}Waiting for services to start...${NC}"
sleep 5

# Check if collector is healthy
if curl -s http://localhost:13133 > /dev/null; then
    echo -e "${GREEN}✓ OpenTelemetry Collector is healthy${NC}"
else
    echo -e "${RED}✗ OpenTelemetry Collector health check failed${NC}"
fi

# Check if Jaeger UI is accessible
if curl -s http://localhost:16686 > /dev/null; then
    echo -e "${GREEN}✓ Jaeger UI is accessible at http://localhost:16686${NC}"
else
    echo -e "${RED}✗ Jaeger UI is not accessible${NC}"
fi

echo ""
echo -e "${GREEN}Running mtlog OTEL example...${NC}"
echo ""

# Run the example
go run -tags=otel main.go

echo ""
echo -e "${YELLOW}Services are still running. You can:${NC}"
echo "  • View traces in Jaeger UI: http://localhost:16686"
echo "  • View collector metrics: http://localhost:8888/metrics"
echo "  • View collector zpages: http://localhost:55679/debug/tracez"
echo "  • Check logs in Docker: docker-compose logs otel-collector"
echo ""
echo -e "${YELLOW}To stop all services, run:${NC} docker-compose down"
echo ""
echo -e "${GREEN}To view the exported logs:${NC}"
echo "docker exec otel-collector cat /tmp/otel-logs.json | jq ."