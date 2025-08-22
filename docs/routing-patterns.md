# Common Routing Patterns

This guide shows typical production routing patterns for structured logging with mtlog.

## Pattern 1: Environment-Based Routing

Route logs differently based on environment (development vs production).

```go
// Development: Everything to console with colors
// Production: Errors to alerts, everything to file
func SetupLogging(environment string) core.Logger {
    if environment == "development" {
        return mtlog.New(
            mtlog.WithConsoleTheme("dev"),
            mtlog.WithMinimumLevel(core.DebugLevel),
        )
    }
    
    // Production setup
    alertSink, _ := sinks.NewFileSink("alerts.log")
    allSink, _ := sinks.NewFileSink("app.log")
    
    router := sinks.NewRouterSink(sinks.AllMatch,
        sinks.Route{
            Name: "critical-alerts",
            Predicate: sinks.NewPredicateBuilder().
                Level(core.ErrorLevel).
                And().Property("Alert").
                Build(),
            Sink: alertSink,
        },
        sinks.Route{
            Name:      "all-events",
            Predicate: func(*core.LogEvent) bool { return true },
            Sink:      allSink,
        },
    )
    
    return mtlog.New(
        mtlog.WithSink(router),
        mtlog.WithMinimumLevel(core.InformationLevel),
    )
}
```

## Pattern 2: Compliance and Audit Logging

Separate audit events for compliance while maintaining operational logs.

```go
func SetupComplianceLogging() core.Logger {
    // Audit sink with durable buffering
    auditSink, _ := sinks.NewFileSink("audit/audit.log")
    durableAudit := sinks.NewDurableSink(auditSink, sinks.DurableOptions{
        BufferPath: "audit/buffer",
        MaxBufferSize: 100 * 1024 * 1024, // 100MB
    })
    
    // Regular operations sink
    opsSink, _ := sinks.NewRollingFileSink(sinks.RollingFileOptions{
        FilePath: "logs/app.log",
        RollingInterval: sinks.RollingIntervalDaily,
    })
    
    // Router with AllMatch for compliance
    router := sinks.NewRouterSink(sinks.AllMatch,
        sinks.Route{
            Name:      "audit",
            Predicate: sinks.PropertyPredicate("Audit"),
            Sink:      durableAudit,
        },
        sinks.Route{
            Name:      "operations",
            Predicate: func(*core.LogEvent) bool { return true },
            Sink:      opsSink,
        },
    )
    
    return mtlog.New(
        mtlog.WithSink(router),
        mtlog.WithProperty("Application", "ComplianceApp"),
    )
}

// Usage
log.With("Audit", true).Information("User {UserId} accessed {Resource}", userId, resource)
```

## Pattern 3: Performance Monitoring

Route slow operations and metrics to specialized sinks.

```go
func SetupPerformanceLogging() core.Logger {
    // Metrics sink (could be Elasticsearch, Splunk, etc.)
    metricsSink, _ := sinks.NewElasticsearchSink(
        "http://localhost:9200",
        sinks.WithElasticsearchIndex("metrics"),
    )
    
    // Slow query sink
    slowQuerySink, _ := sinks.NewFileSink("performance/slow-queries.log")
    
    // Console for development
    consoleSink := sinks.NewConsoleSink()
    
    router := sinks.NewRouterSink(sinks.AllMatch,
        // Route metrics to Elasticsearch
        sinks.Route{
            Name:      "metrics",
            Predicate: sinks.PropertyPredicate("Metric"),
            Sink:      metricsSink,
        },
        // Route slow operations to dedicated file
        sinks.Route{
            Name: "slow-ops",
            Predicate: func(e *core.LogEvent) bool {
                if elapsed, ok := e.Properties["Elapsed"].(time.Duration); ok {
                    return elapsed > 100*time.Millisecond
                }
                return false
            },
            Sink: slowQuerySink,
        },
        // Everything to console for monitoring
        sinks.Route{
            Name:      "console",
            Predicate: func(*core.LogEvent) bool { return true },
            Sink:      consoleSink,
        },
    )
    
    return mtlog.New(mtlog.WithSink(router))
}

// Usage
start := time.Now()
// ... perform operation ...
elapsed := time.Since(start)
log.With("Elapsed", elapsed, "Metric", true).
    Information("Query executed in {Elapsed}", elapsed)
```

## Pattern 4: Multi-Tenant Routing

Route logs based on tenant for isolation and billing.

```go
func SetupMultiTenantLogging() core.Logger {
    // Create tenant-specific sinks
    tenantSinks := map[string]core.LogEventSink{
        "tenant-a": createTenantSink("tenant-a"),
        "tenant-b": createTenantSink("tenant-b"),
        "tenant-c": createTenantSink("tenant-c"),
    }
    
    // Default sink for unknown tenants
    defaultSink, _ := sinks.NewFileSink("logs/unknown-tenant.log")
    
    // Create routes for each tenant
    var routes []sinks.Route
    for tenantId, sink := range tenantSinks {
        tid := tenantId // Capture for closure
        routes = append(routes, sinks.Route{
            Name: fmt.Sprintf("tenant-%s", tid),
            Predicate: sinks.PropertyValuePredicate("TenantId", tid),
            Sink: sink,
        })
    }
    
    router := sinks.NewRouterSinkWithDefault(
        sinks.FirstMatch,
        defaultSink,
        routes...,
    )
    
    return mtlog.New(mtlog.WithSink(router))
}

func createTenantSink(tenantId string) core.LogEventSink {
    sink, _ := sinks.NewFileSink(fmt.Sprintf("logs/tenants/%s.log", tenantId))
    // Could also route to tenant-specific Seq instance, S3 bucket, etc.
    return sink
}

// Usage
tenantLogger := log.With("TenantId", request.TenantId)
tenantLogger.Information("Processing request for tenant")
```

## Pattern 5: Error Alerting with Escalation

Route errors with intelligent escalation based on severity and frequency.

```go
func SetupErrorAlerting() core.Logger {
    // Immediate alerts for critical errors
    pagingSink := &PagerDutySink{} // Custom sink for PagerDuty
    
    // Slack for warnings
    slackSink := &SlackSink{} // Custom sink for Slack
    
    // File sink for all errors
    errorFile, _ := sinks.NewFileSink("errors/all-errors.log")
    
    router := sinks.NewRouterSink(sinks.AllMatch,
        // Critical errors -> PagerDuty
        sinks.Route{
            Name: "pager",
            Predicate: sinks.NewPredicateBuilder().
                Level(core.ErrorLevel).
                And().Property("Critical").
                Build(),
            Sink: pagingSink,
        },
        // Errors with Alert flag -> Slack
        sinks.Route{
            Name: "slack",
            Predicate: sinks.NewPredicateBuilder().
                Level(core.ErrorLevel).
                And().Property("Alert").
                Build(),
            Sink: slackSink,
        },
        // All errors -> File
        sinks.Route{
            Name:      "error-file",
            Predicate: sinks.LevelPredicate(core.ErrorLevel),
            Sink:      errorFile,
        },
    )
    
    return mtlog.New(mtlog.WithSink(router))
}

// Usage
log.With("Critical", true, "Alert", true).
    Error("Database connection lost: {Error}", err)
```

## Pattern 6: Development vs Production with Statistics

Use route statistics for monitoring and debugging.

```go
func SetupLoggingWithStats() (*mtlog.Logger, *sinks.RouterSink) {
    router := sinks.NewRouterSink(sinks.AllMatch,
        sinks.Route{
            Name:      "errors",
            Predicate: sinks.LevelPredicate(core.ErrorLevel),
            Sink:      createErrorSink(),
        },
        sinks.Route{
            Name:      "audit",
            Predicate: sinks.PropertyPredicate("Audit"),
            Sink:      createAuditSink(),
        },
        sinks.Route{
            Name:      "metrics",
            Predicate: sinks.PropertyPredicate("Metric"),
            Sink:      createMetricsSink(),
        },
    )
    
    // Periodically log statistics
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        
        for range ticker.C {
            stats := router.GetStats()
            fmt.Printf("Routing Stats - Total: %d, Errors: %d, Audit: %d, Metrics: %d, Dropped: %d\n",
                stats.TotalEvents,
                stats.RouteHits["errors"],
                stats.RouteHits["audit"],
                stats.RouteHits["metrics"],
                stats.DroppedEvents,
            )
        }
    }()
    
    logger := mtlog.New(mtlog.WithSink(router))
    return logger, router
}

// Test routing configuration
func TestRoutingConfig(router *sinks.RouterSink) {
    testEvents := []struct {
        name  string
        event *core.LogEvent
    }{
        {
            name: "Error Event",
            event: &core.LogEvent{
                Level: core.ErrorLevel,
                Properties: map[string]any{},
            },
        },
        {
            name: "Audit Event",
            event: &core.LogEvent{
                Level: core.InformationLevel,
                Properties: map[string]any{"Audit": true},
            },
        },
    }
    
    for _, test := range testEvents {
        matches := router.TestEvent(test.event)
        fmt.Printf("%s would route to: %v\n", test.name, matches)
    }
}
```

## Pattern 7: Priority-Based Routing

Use explicit priority values to control route evaluation order for optimal performance.

```go
func SetupPriorityRouting() core.Logger {
    router := sinks.NewRouterSink(sinks.FirstMatch,
        // Priority 10: Critical alerts (check first)
        sinks.NewRoute("critical").
            WithPriority(10).
            When(sinks.NewPredicateBuilder().
                Level(core.FatalLevel).
                Or().Property("Critical").
                Build()).
            To(pagerDutySink),
        
        // Priority 20: Errors
        sinks.NewRoute("errors").
            WithPriority(20).
            When(sinks.LevelPredicate(core.ErrorLevel)).
            To(errorSink),
        
        // Priority 100: Info and above (default)
        sinks.NewRoute("general").
            WithPriority(100).
            When(sinks.LevelPredicate(core.InformationLevel)).
            To(generalSink),
        
        // Priority 200: Debug (check last)
        sinks.NewRoute("debug").
            WithPriority(200).
            When(sinks.LevelPredicate(core.DebugLevel)).
            To(debugSink),
    )
    
    return mtlog.New(mtlog.WithSink(router))
}
```

## Pattern 8: Route Groups for Organization

Group related routes together for better management and dynamic configuration.

```go
func SetupGroupedRouting() (*mtlog.Logger, *sinks.RouterSink) {
    router := sinks.NewRouterSink(sinks.AllMatch)
    
    // Security-related routes
    securityGroup := sinks.NewRouteGroup("security",
        sinks.Route{
            Name:     "auth-failures",
            Priority: 10,
            Predicate: sinks.PropertyValuePredicate("EventType", "AuthFailure"),
            Sink:     securitySink,
        },
        sinks.Route{
            Name:     "access-violations",
            Priority: 10,
            Predicate: sinks.PropertyPredicate("AccessViolation"),
            Sink:     securitySink,
        },
    )
    
    // Performance monitoring routes
    performanceGroup := sinks.NewRouteGroup("performance",
        sinks.Route{
            Name:     "slow-queries",
            Priority: 50,
            Predicate: func(e *core.LogEvent) bool {
                if duration, ok := e.Properties["Duration"].(time.Duration); ok {
                    return duration > 1*time.Second
                }
                return false
            },
            Sink: performanceSink,
        },
        sinks.Route{
            Name:     "metrics",
            Priority: 100,
            Predicate: sinks.PropertyPredicate("Metric"),
            Sink:     metricsSink,
        },
    )
    
    // Add groups to router
    router.AddRouteGroup(securityGroup)
    router.AddRouteGroup(performanceGroup)
    
    // Dynamically enable/disable groups based on environment
    if os.Getenv("DISABLE_PERFORMANCE_MONITORING") == "true" {
        router.RemoveRouteGroup("performance")
    }
    
    return mtlog.New(mtlog.WithSink(router)), router
}

// Runtime group management
func ToggleSecurityLogging(router *sinks.RouterSink, enable bool) {
    if enable {
        router.AddRouteGroup(getSecurityGroup())
    } else {
        router.RemoveRouteGroup("security")
    }
}
```

## Pattern 9: Conditional Sampling

Sample verbose logs in production while keeping all errors.

```go
func SetupSampledLogging() core.Logger {
    // Always log errors
    errorSink, _ := sinks.NewFileSink("errors.log")
    
    // Sample 10% of debug logs
    debugSink, _ := sinks.NewFileSink("debug.log")
    sampledDebugSink := sinks.NewConditionalSink(
        func(e *core.LogEvent) bool {
            // Simple sampling: hash of message
            return e.Level <= core.DebugLevel && 
                   hash(e.MessageTemplate)%10 == 0
        },
        debugSink,
    )
    
    // All info and above
    infoSink, _ := sinks.NewFileSink("app.log")
    
    router := sinks.NewRouterSink(sinks.FirstMatch,
        sinks.Route{
            Name:      "errors",
            Predicate: sinks.LevelPredicate(core.ErrorLevel),
            Sink:      errorSink,
        },
        sinks.Route{
            Name:      "sampled-debug",
            Predicate: func(e *core.LogEvent) bool { return e.Level <= core.DebugLevel },
            Sink:      sampledDebugSink,
        },
        sinks.Route{
            Name:      "info",
            Predicate: func(*core.LogEvent) bool { return true },
            Sink:      infoSink,
        },
    )
    
    return mtlog.New(mtlog.WithSink(router))
}
```

## Pattern 10: Prometheus Metrics Integration

Export router statistics to Prometheus for monitoring and alerting.

```go
func SetupMetricsExport() {
    // Create routers
    mainRouter := sinks.NewRouterSink(sinks.FirstMatch, routes...)
    auditRouter := sinks.NewRouterSink(sinks.AllMatch, auditRoutes...)
    
    // Create metrics exporter
    exporter := sinks.NewRouterMetricsExporter()
    exporter.RegisterRouter("main", mainRouter)
    exporter.RegisterRouter("audit", auditRouter)
    
    // Start metrics server
    go func() {
        if err := sinks.StartMetricsServer(":9090", exporter); err != nil {
            log.Printf("Failed to start metrics server: %v", err)
        }
    }()
}

// Prometheus queries for monitoring:
// - Rate of events per route: rate(mtlog_router_route_hits_total[5m])
// - Dropped events alert: mtlog_router_dropped_events_total > 0
// - Route distribution: mtlog_router_route_hits_total / mtlog_router_events_total

// Import the Grafana dashboard from dashboards/grafana-mtlog-dashboard.json
```

## Pattern 11: Health Checks and Circuit Breakers

Implement resilient logging with health checks and circuit breakers for unreliable sinks.

```go
func SetupResilientLogging() core.Logger {
    // Wrap unreliable sinks with circuit breakers
    dbSink := createDatabaseSink()
    protectedDbSink := sinks.NewCircuitBreakerSinkWithOptions(dbSink, sinks.CircuitBreakerOptions{
        Name:             "database",
        FailureThreshold: 5,        // Open after 5 failures
        SuccessThreshold: 3,        // Close after 3 successes in half-open
        ResetTimeout:     30 * time.Second,
        FallbackSink:     sinks.NewFileSink("fallback.log"), // Fallback when open
        OnStateChange: func(from, to sinks.CircuitState) {
            log.Printf("Circuit breaker state changed: %s -> %s", from, to)
        },
    })
    
    // Create router with health-checkable sinks
    router := sinks.NewRouterSink(sinks.FirstMatch,
        sinks.Route{
            Name:      "database",
            Priority:  10,
            Predicate: sinks.PropertyPredicate("Audit"),
            Sink:      protectedDbSink,
        },
        sinks.Route{
            Name:      "file",
            Priority:  100,
            Predicate: func(*core.LogEvent) bool { return true },
            Sink:      sinks.NewFileSink("app.log"),
        },
    )
    
    // Start periodic health checks
    ctx := context.Background()
    router.PeriodicHealthCheck(ctx, 1*time.Minute, func(results map[string]sinks.HealthStatus) {
        for route, status := range results {
            if !status.Healthy {
                log.Printf("Route %s unhealthy: %v", route, status.Error)
                // Could trigger alerts or take corrective action
            }
        }
    })
    
    return mtlog.New(mtlog.WithSink(router))
}

// Implement HealthCheckable for custom sinks
type CustomSink struct {
    client *SomeClient
}

func (cs *CustomSink) HealthCheck(ctx context.Context) error {
    // Test connectivity
    if err := cs.client.Ping(ctx); err != nil {
        return fmt.Errorf("ping failed: %w", err)
    }
    return nil
}

// Manual health check endpoint for monitoring
func HealthCheckHandler(router *sinks.RouterSink) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
        defer cancel()
        
        results := router.CheckHealth(ctx)
        allHealthy := true
        
        for _, status := range results {
            if !status.Healthy {
                allHealthy = false
                break
            }
        }
        
        if allHealthy {
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(results)
        } else {
            w.WriteHeader(http.StatusServiceUnavailable)
            json.NewEncoder(w).Encode(results)
        }
    }
}
```

## Best Practices

### 1. Use FirstMatch for Performance
When routes are mutually exclusive, use `FirstMatch` mode for better performance:
```go
// Good: FirstMatch when routes don't overlap
router := sinks.NewRouterSink(sinks.FirstMatch,
    sinks.ErrorRoute("errors", errorSink),
    sinks.Route{
        Name:      "warnings",
        Predicate: sinks.LevelPredicate(core.WarningLevel),
        Sink:      warningSink,
    },
)
```

### 2. Use AllMatch for Compliance
When events need multiple destinations (audit, metrics, alerts), use `AllMatch`:
```go
// Good: AllMatch for compliance and monitoring
router := sinks.NewRouterSink(sinks.AllMatch,
    sinks.AuditRoute("audit", auditSink),
    sinks.MetricRoute("metrics", metricsSink),
    sinks.Route{
        Name:      "all",
        Predicate: func(*core.LogEvent) bool { return true },
        Sink:      archiveSink,
    },
)
```

### 3. Always Have a Default or Catch-All
Prevent losing events by having a default sink or catch-all route:
```go
// Good: Default sink for unmatched events
router := sinks.NewRouterSinkWithDefault(
    sinks.FirstMatch,
    defaultSink, // Catches everything that doesn't match
    routes...,
)

// Or: Catch-all route
routes = append(routes, sinks.Route{
    Name:      "catch-all",
    Predicate: func(*core.LogEvent) bool { return true },
    Sink:      defaultSink,
})
```

### 4. Use PredicateBuilder for Complex Logic
For readable complex predicates, use the fluent builder:
```go
// Good: Clear, readable predicate
predicate := sinks.NewPredicateBuilder().
    Level(core.ErrorLevel).
    And().Property("Critical").
    And().Not().Property("Retry").
    Build()

// Instead of: Hard to read nested functions
predicate := sinks.AndPredicate(
    sinks.LevelPredicate(core.ErrorLevel),
    sinks.PropertyPredicate("Critical"),
    sinks.NotPredicate(sinks.PropertyPredicate("Retry")),
)
```

### 5. Monitor Route Statistics
In production, monitor routing statistics to ensure proper configuration:
```go
// Expose metrics endpoint
http.HandleFunc("/metrics/logging", func(w http.ResponseWriter, r *http.Request) {
    stats := router.GetStats()
    json.NewEncoder(w).Encode(stats)
})
```

### 6. Test Your Routing Configuration
Use `TestEvent` to verify routing logic:
```go
// In tests
func TestLoggingRoutes(t *testing.T) {
    router := setupProductionRouter()
    
    criticalError := &core.LogEvent{
        Level: core.ErrorLevel,
        Properties: map[string]any{"Critical": true},
    }
    
    matches := router.TestEvent(criticalError)
    assert.Contains(t, matches, "pagerduty")
    assert.Contains(t, matches, "error-archive")
}
```

## Performance Considerations

### Predicate Order Matters
In `FirstMatch` mode, put most likely matches first:
```go
// Good: Common cases first
router := sinks.NewRouterSink(sinks.FirstMatch,
    sinks.Route{
        Name:      "info",  // 80% of events
        Predicate: sinks.LevelPredicate(core.InformationLevel),
        Sink:      infoSink,
    },
    sinks.Route{
        Name:      "errors",  // 15% of events
        Predicate: sinks.LevelPredicate(core.ErrorLevel),
        Sink:      errorSink,
    },
    sinks.Route{
        Name:      "debug",  // 5% of events
        Predicate: sinks.LevelPredicate(core.DebugLevel),
        Sink:      debugSink,
    },
)
```

### Cache Predicate Results
For expensive predicates, consider caching:
```go
// Expensive predicate with caching
var userCache = &sync.Map{}

func isVIPUser(e *core.LogEvent) bool {
    userId, ok := e.Properties["UserId"].(string)
    if !ok {
        return false
    }
    
    // Check cache
    if cached, ok := userCache.Load(userId); ok {
        return cached.(bool)
    }
    
    // Expensive check
    isVIP := checkUserVIPStatus(userId)
    userCache.Store(userId, isVIP)
    return isVIP
}
```

### Use Conditional Sinks for Simple Filtering
For single-condition filtering, `ConditionalSink` is more efficient than `RouterSink`:
```go
// Good: Simple condition with ConditionalSink
errorOnlySink := sinks.NewConditionalSink(
    sinks.LevelPredicate(core.ErrorLevel),
    targetSink,
)

// Overkill: Router for single condition
router := sinks.NewRouterSink(sinks.FirstMatch,
    sinks.ErrorRoute("errors", targetSink),
)
```