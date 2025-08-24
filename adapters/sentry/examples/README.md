# Sentry Integration Examples for mtlog

Send your Go application logs to Sentry with structured context, breadcrumbs, and error tracking.

## Quick Start

### Prerequisites
- Docker and Docker Compose installed
- Go 1.21 or later
- About 5GB of disk space for Sentry containers

### Step 1: Set Up Local Sentry

From the mtlog root directory:

```bash
# 1. Download and configure Sentry (one-time setup)
./adapters/sentry/scripts/setup-integration-test.sh

# 2. Navigate to docker directory
cd docker

# 3. Start and initialize Sentry
../adapters/sentry/scripts/initialize-sentry-pipeline.sh

# 4. Verify everything is working
../adapters/sentry/scripts/verify-sentry-pipeline.sh
```

The initialization script will display your DSN. Save it for the next step:
```
DSN: http://[your-key]@localhost:9000/1
```

### Step 2: Run the Examples

Set your DSN and run any example:

```bash
# Set the DSN from Step 1
export SENTRY_DSN="http://[your-key]@localhost:9000/1"

# Run an example
cd adapters/sentry/examples/basic
go run main.go
```

### Step 3: View Events in Sentry

1. Open http://localhost:9000 in your browser
2. Log in with:
   - Email: `admin@test.local`
   - Password: `admin`
3. Navigate to **Issues** to see your logged errors

## Examples Overview

### 📁 basic/
**Simple error logging with Sentry integration**

Shows the fundamentals of sending errors to Sentry:
- Creating a Sentry sink with DSN
- Logging errors that appear as issues in Sentry
- Basic configuration options

```go
// Create Sentry sink
sentrySink, _ := sentry.WithSentry(dsn,
    sentry.WithEnvironment("production"),
    sentry.WithRelease("v1.0.0"),
)

// Log an error - this will appear in Sentry
logger.Error("Payment failed: {Error}", paymentErr)
```

**Run it:** `cd basic && go run main.go`

### 📁 breadcrumbs/
**Shows how debug/info logs become breadcrumbs attached to errors**

Demonstrates Sentry's breadcrumb trail feature:
- Info and Debug logs are captured as breadcrumbs
- When an error occurs, breadcrumbs provide context
- Shows the user's journey leading to the error

```go
// These become breadcrumbs
logger.Debug("User session started")
logger.Information("Loading user preferences")
logger.Warning("Slow query detected")

// This error will have all the above as breadcrumbs
logger.Error("Transaction failed: {Error}", err)
```

**Run it:** `cd breadcrumbs && go run main.go`

### 📁 context/
**Demonstrates adding user context and custom tags**

Advanced context enrichment:
- Attach user information to errors
- Add custom tags for filtering in Sentry
- Set request context for API errors
- Custom device and location data

```go
// Add user context
ctx = sentry.WithUser(ctx, sentrygo.User{
    ID:    "user-123",
    Email: "user@example.com",
})

// Add custom tags
ctx = sentry.WithTags(ctx, map[string]string{
    "region": "us-west-2",
    "plan":   "premium",
})

// Errors will include this context
logger.WithContext(ctx).Error("Subscription failed")
```

**Run it:** `cd context && go run main.go`

## Configuration Guide

### Basic Configuration

```go
sentrySink, err := sentry.WithSentry(dsn)
```

### With Options

```go
sentrySink, err := sentry.WithSentry(dsn,
    // Environment (development, staging, production)
    sentry.WithEnvironment("production"),
    
    // Release version for tracking deployments
    sentry.WithRelease("v1.2.3"),
    
    // Server/host identification
    sentry.WithServerName("api-server-01"),
    
    // Only send Error and Fatal levels to Sentry
    sentry.WithMinLevel(core.ErrorLevel),
    
    // Capture Debug and above as breadcrumbs
    sentry.WithBreadcrumbLevel(core.DebugLevel),
    
    // Maximum breadcrumbs to attach (default: 100)
    sentry.WithMaxBreadcrumbs(100),
)
```

### Using with mtlog

```go
logger := mtlog.New(
    mtlog.WithConsole(),           // Local console output
    mtlog.WithSink(sentrySink),    // Send to Sentry
)
defer logger.Close()

// Use the logger normally
logger.Information("User {UserId} logged in", userId)
logger.Error("Payment failed: {Error}", err)
```

## How It Works

When you log an error with mtlog's Sentry integration:

1. **Error Level**: Messages at Error or Fatal level are sent to Sentry as events
2. **Breadcrumbs**: Lower level logs (Debug, Info, Warning) become breadcrumbs
3. **Context**: User info, tags, and custom data are automatically attached
4. **Grouping**: Errors are grouped by message template in Sentry
5. **Rich Data**: Stack traces, environment info, and metadata are preserved

The message template becomes the issue title in Sentry, making it easy to track error patterns:
- `"Database timeout for {Table}"` → Groups all database timeouts together
- `"Payment failed: {Error}"` → Groups payment failures by error type

## Testing with Production Sentry

To use with Sentry SaaS instead of local:

1. Get your DSN from your Sentry project settings
2. Set the environment variable:
   ```bash
   export SENTRY_DSN="https://[key]@sentry.io/[project]"
   ```
3. Run any example normally

## Troubleshooting

### Events Not Appearing in Sentry UI

1. **Check the DSN**: Ensure SENTRY_DSN is set correctly
2. **Wait a moment**: Events can take a few seconds to appear
3. **Check filters**: The Sentry UI may be filtering by date/environment
4. **Verify services**: Run `../scripts/verify-sentry-pipeline.sh`

### Common Issues

- **"Please set SENTRY_DSN"**: Export the DSN environment variable
- **Connection refused**: Sentry services aren't running. Run the initialization script
- **No events after restart**: Services need reinitialization after Docker restart

### Stopping Sentry

To stop the local Sentry instance:
```bash
cd docker
docker-compose -f docker-compose.test.yml down
```

To completely remove all data:
```bash
docker-compose -f docker-compose.test.yml down -v
```

## Further Reading

- [mtlog Documentation](../../../README.md)
- [Sentry Go SDK Documentation](https://docs.sentry.io/platforms/go/)
- [Sentry Event Context](https://docs.sentry.io/platforms/go/enriching-events/)