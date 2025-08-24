#!/bin/bash
set -e

echo "=== Verifying Sentry Pipeline ==="

ERRORS=0

# Check Kafka topics exist
echo -n "Checking Kafka topics... "
TOPICS=$(docker exec docker-kafka-1 kafka-topics --list --bootstrap-server localhost:9092 2>/dev/null)
REQUIRED_TOPICS=("ingest-events" "ingest-transactions" "events" "transactions" "outcomes")

for topic in "${REQUIRED_TOPICS[@]}"; do
    if ! echo "$TOPICS" | grep -q "^$topic$"; then
        echo ""
        echo "  ✗ Missing Kafka topic: $topic"
        ERRORS=$((ERRORS + 1))
    fi
done

if [ $ERRORS -eq 0 ]; then
    echo "✓"
fi

# Check essential containers are running
echo -n "Checking essential services... "
REQUIRED_SERVICES=(
    "docker-kafka-1"
    "docker-clickhouse-1"
    "docker-web-1"
    "docker-events-consumer-1"
    "docker-snuba-api-1"
    "docker-snuba-consumer-1"
)

for service in "${REQUIRED_SERVICES[@]}"; do
    if ! docker ps --format "{{.Names}}" | grep -q "^$service$"; then
        echo ""
        echo "  ✗ Service not running: $service"
        ERRORS=$((ERRORS + 1))
    fi
done

if [ $ERRORS -eq 0 ]; then
    echo "✓"
fi

# Check Sentry web health
echo -n "Checking Sentry web health... "
if curl -f http://localhost:9000/_health/ >/dev/null 2>&1; then
    echo "✓"
else
    echo "✗"
    ERRORS=$((ERRORS + 1))
fi

# Check ClickHouse has required tables
echo -n "Checking ClickHouse tables... "
TABLES=$(docker exec docker-clickhouse-1 clickhouse-client --query "SELECT count(*) FROM system.tables WHERE database = 'default';" 2>/dev/null)
if [ "$TABLES" -gt 20 ]; then
    echo "✓ ($TABLES tables)"
else
    echo "✗ (only $TABLES tables found)"
    ERRORS=$((ERRORS + 1))
fi

# Check consumer groups are active
echo -n "Checking consumer groups... "
CONSUMER_GROUPS=$(docker exec docker-kafka-1 kafka-consumer-groups --bootstrap-server localhost:9092 --list 2>/dev/null | wc -l)
if [ "$CONSUMER_GROUPS" -gt 0 ]; then
    echo "✓ ($CONSUMER_GROUPS groups)"
else
    echo "✗ (no consumer groups found)"
    ERRORS=$((ERRORS + 1))
fi

# Check if events can be processed (send a test event)
echo -n "Testing event processing... "
TEST_RESPONSE=$(curl -sf -X POST "http://localhost:9000/api/1/envelope/" \
    -H "Content-Type: application/json" \
    -H "X-Sentry-Auth: Sentry sentry_version=7, sentry_key=test" \
    -d '{"event_id":"00000000000000000000000000000000","timestamp":"2024-01-01T00:00:00Z","platform":"other"}' 2>/dev/null || echo "FAILED")

if [ "$TEST_RESPONSE" != "FAILED" ]; then
    echo "✓"
else
    echo "✗ (event submission failed)"
    ERRORS=$((ERRORS + 1))
fi

echo ""
if [ $ERRORS -eq 0 ]; then
    echo "=== ✓ All pipeline components verified ==="
    exit 0
else
    echo "=== ✗ Pipeline verification failed with $ERRORS errors ==="
    echo ""
    echo "Troubleshooting:"
    echo "1. Check docker logs: docker-compose -f docker/docker-compose.test.yml logs"
    echo "2. Restart services: docker-compose -f docker/docker-compose.test.yml restart"
    echo "3. Check Kafka topics: docker exec docker-kafka-1 kafka-topics --list --bootstrap-server localhost:9092"
    exit 1
fi