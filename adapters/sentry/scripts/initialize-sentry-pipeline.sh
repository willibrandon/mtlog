#!/bin/bash
set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/../../.." && pwd )"
DOCKER_DIR="$PROJECT_ROOT/docker"

echo "=== Initializing Sentry Pipeline ==="

# Change to docker directory
cd "$DOCKER_DIR"

# Function to wait for service
wait_for_service() {
    local service=$1
    local max_attempts=30
    local attempt=0
    
    echo "Waiting for $service to be ready..."
    while [ $attempt -lt $max_attempts ]; do
        if docker ps | grep -q "$service"; then
            if docker exec "$service" echo "OK" >/dev/null 2>&1; then
                echo "✓ $service is ready"
                return 0
            fi
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
    
    echo "✗ Timeout waiting for $service"
    return 1
}

# Start core infrastructure services first
echo "Starting core infrastructure services..."
docker-compose -f docker-compose.test.yml up -d postgres redis memcached clickhouse kafka zookeeper

# Wait for Kafka to be ready
wait_for_service "docker-kafka-1"

# Create all required Kafka topics
echo "Creating required Kafka topics..."
docker exec docker-kafka-1 kafka-topics --create --topic ingest-events --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1 --if-not-exists || true
docker exec docker-kafka-1 kafka-topics --create --topic ingest-transactions --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1 --if-not-exists || true
docker exec docker-kafka-1 kafka-topics --create --topic events --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1 --if-not-exists || true
docker exec docker-kafka-1 kafka-topics --create --topic transactions --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1 --if-not-exists || true
docker exec docker-kafka-1 kafka-topics --create --topic ingest-attachments --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1 --if-not-exists || true
docker exec docker-kafka-1 kafka-topics --create --topic ingest-metrics --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1 --if-not-exists || true
docker exec docker-kafka-1 kafka-topics --create --topic outcomes --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1 --if-not-exists || true
docker exec docker-kafka-1 kafka-topics --create --topic ingest-sessions --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1 --if-not-exists || true
docker exec docker-kafka-1 kafka-topics --create --topic snuba-commit-log --bootstrap-server localhost:9092 --partitions 1 --replication-factor 1 --if-not-exists || true

echo "✓ Kafka topics created"

# Start Snuba services
echo "Starting Snuba services..."
docker-compose -f docker-compose.test.yml up -d snuba-api snuba-consumer snuba-outcomes-consumer snuba-sessions-consumer snuba-transactions-consumer

# Wait for Snuba API to be ready
wait_for_service "docker-snuba-api-1"

# Run Snuba migrations
echo "Running Snuba migrations..."
docker exec docker-snuba-api-1 snuba migrations migrate --force
echo "✓ Snuba migrations complete"

# Start Sentry web and worker services
echo "Starting Sentry services..."
docker-compose -f docker-compose.test.yml up -d web worker cron

# Wait for Sentry web to be healthy
echo "Waiting for Sentry to be ready..."
for i in {1..60}; do
    if curl -f http://localhost:9000/_health/ >/dev/null 2>&1; then
        echo "✓ Sentry web is ready"
        break
    fi
    sleep 2
done

# Start consumer services
echo "Starting consumer services..."
docker-compose -f docker-compose.test.yml up -d events-consumer transactions-consumer

# Wait for consumers to stabilize
sleep 10

# Create and configure test project
echo "Configuring test project..."
docker exec docker-web-1 sentry exec -c "
from sentry.models import Organization, Project, ProjectKey, ProjectOption

# Create org and project
org, _ = Organization.objects.get_or_create(slug='sentry', defaults={'name': 'Sentry'})
project, _ = Project.objects.get_or_create(
    organization=org,
    slug='internal',  # Use internal as it's the default
    defaults={'name': 'Internal', 'platform': None}
)

# Ensure project accepts all platforms
project.platform = None
project.save()

# Remove any platform restrictions
ProjectOption.objects.filter(project=project, key__contains='platform').delete()

# Get the existing key
keys = ProjectKey.objects.filter(project=project)
if keys.exists():
    key = keys.first()
else:
    key = ProjectKey.objects.create(
        project=project,
        label='Default'
    )

print(f'✓ Project configured to accept all platforms')
print(f'  Project ID: {project.id}')
print(f'  Project slug: {project.slug}')
print(f'  DSN: {key.dsn_public}')
"

# Start remaining services (nginx, relay, etc.)
echo "Starting remaining services..."
docker-compose -f docker-compose.test.yml up -d

# Final wait for everything to stabilize
echo "Waiting for all services to stabilize..."
sleep 15

# Verify pipeline
echo ""
echo "=== Verifying Pipeline Status ==="

# Check Kafka topics
echo "Kafka topics:"
docker exec docker-kafka-1 kafka-topics --list --bootstrap-server localhost:9092 | head -10

# Check consumer groups
echo ""
echo "Consumer groups:"
docker exec docker-kafka-1 kafka-consumer-groups --bootstrap-server localhost:9092 --list

# Check ClickHouse tables
echo ""
echo "ClickHouse tables (first 5):"
docker exec docker-clickhouse-1 clickhouse-client --query "SELECT name FROM system.tables WHERE database = 'default' LIMIT 5;"

echo ""
echo "=== Sentry Pipeline Initialization Complete ==="
echo "Sentry UI: http://localhost:9000"
echo "Use the DSN shown above for sending events"