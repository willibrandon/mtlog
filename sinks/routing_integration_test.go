// +build integration

package sinks_test

import (
	"os"
	"testing"
	"time"
	
	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func TestRoutingSinksIntegration(t *testing.T) {
	t.Run("ConditionalSink with real file sink", func(t *testing.T) {
		// Create temp file for alerts
		alertFile := "test_alerts.log"
		defer os.Remove(alertFile)
		
		alertSink, err := sinks.NewFileSink(alertFile)
		if err != nil {
			t.Fatalf("Failed to create file sink: %v", err)
		}
		defer alertSink.Close()
		
		// Create conditional sink
		conditionalSink := sinks.NewConditionalSink(
			func(e *core.LogEvent) bool {
				return e.Level >= core.ErrorLevel && e.Properties["Alert"] != nil
			},
			alertSink,
		)
		
		// Create logger
		log := mtlog.New(
			mtlog.WithSink(conditionalSink),
			mtlog.WithSink(sinks.NewMemorySink()), // Also use memory sink to verify all events
		)
		defer log.Close()
		
		// Log various events
		log.Information("Normal operation")
		log.Warning("Low disk space")
		log.Error("Connection failed") // No Alert property
		log.With("Alert", true).Error("Database down") // Should go to alert file
		log.With("Alert", true).Fatal("System critical") // Should go to alert file
		
		// Give time for file writes
		time.Sleep(100 * time.Millisecond)
		
		// Check alert file
		content, err := os.ReadFile(alertFile)
		if err != nil {
			t.Fatalf("Failed to read alert file: %v", err)
		}
		
		contentStr := string(content)
		if !contains(contentStr, "Database down") {
			t.Error("Alert file should contain 'Database down'")
		}
		if !contains(contentStr, "System critical") {
			t.Error("Alert file should contain 'System critical'")
		}
		if contains(contentStr, "Normal operation") {
			t.Error("Alert file should not contain 'Normal operation'")
		}
		if contains(contentStr, "Connection failed") {
			t.Error("Alert file should not contain 'Connection failed' (no Alert property)")
		}
	})
	
	t.Run("RouterSink FirstMatch with multiple file sinks", func(t *testing.T) {
		// Create temp files
		errorFile := "test_errors.log"
		warningFile := "test_warnings.log"
		infoFile := "test_info.log"
		defer os.Remove(errorFile)
		defer os.Remove(warningFile)
		defer os.Remove(infoFile)
		
		errorSink, _ := sinks.NewFileSink(errorFile)
		warningSink, _ := sinks.NewFileSink(warningFile)
		infoSink, _ := sinks.NewFileSink(infoFile)
		defer errorSink.Close()
		defer warningSink.Close()
		defer infoSink.Close()
		
		// Create router with FirstMatch
		router := sinks.NewRouterSink(sinks.FirstMatch,
			sinks.Route{
				Name:      "errors",
				Predicate: sinks.LevelPredicate(core.ErrorLevel),
				Sink:      errorSink,
			},
			sinks.Route{
				Name:      "warnings",
				Predicate: sinks.LevelPredicate(core.WarningLevel),
				Sink:      warningSink,
			},
			sinks.Route{
				Name:      "info",
				Predicate: sinks.LevelPredicate(core.InformationLevel),
				Sink:      infoSink,
			},
		)
		defer router.Close()
		
		// Create logger
		log := mtlog.New(mtlog.WithSink(router))
		defer log.Close()
		
		// Log events
		log.Information("App started")
		log.Warning("Cache miss")
		log.Error("Database timeout")
		log.Fatal("Out of memory")
		
		// Give time for file writes
		time.Sleep(100 * time.Millisecond)
		
		// Check files
		errorContent, _ := os.ReadFile(errorFile)
		warningContent, _ := os.ReadFile(warningFile)
		infoContent, _ := os.ReadFile(infoFile)
		
		// Errors should only be in error file
		if !contains(string(errorContent), "Database timeout") {
			t.Error("Error file should contain 'Database timeout'")
		}
		if !contains(string(errorContent), "Out of memory") {
			t.Error("Error file should contain 'Out of memory'")
		}
		
		// Warnings should only be in warning file
		if !contains(string(warningContent), "Cache miss") {
			t.Error("Warning file should contain 'Cache miss'")
		}
		
		// Info should only be in info file
		if !contains(string(infoContent), "App started") {
			t.Error("Info file should contain 'App started'")
		}
		
		// Verify FirstMatch behavior - errors shouldn't be in warning/info files
		if contains(string(warningContent), "Database timeout") {
			t.Error("Warning file should not contain error events (FirstMatch)")
		}
		if contains(string(infoContent), "Database timeout") {
			t.Error("Info file should not contain error events (FirstMatch)")
		}
	})
	
	t.Run("RouterSink AllMatch with console and file", func(t *testing.T) {
		// Create files for different purposes
		auditFile := "test_audit.log"
		metricsFile := "test_metrics.log"
		allFile := "test_all.log"
		defer os.Remove(auditFile)
		defer os.Remove(metricsFile)
		defer os.Remove(allFile)
		
		auditSink, _ := sinks.NewFileSink(auditFile)
		metricsSink, _ := sinks.NewFileSink(metricsFile)
		allSink, _ := sinks.NewFileSink(allFile)
		defer auditSink.Close()
		defer metricsSink.Close()
		defer allSink.Close()
		
		// Create router with AllMatch
		// Add a route that always matches for the "all" sink
		router := sinks.NewRouterSink(
			sinks.AllMatch,
			sinks.Route{
				Name:      "all",
				Predicate: func(e *core.LogEvent) bool { return true }, // Always matches
				Sink:      allSink,
			},
			sinks.AuditRoute("audit", auditSink),
			sinks.MetricRoute("metrics", metricsSink),
		)
		defer router.Close()
		
		// Create logger
		log := mtlog.New(mtlog.WithSink(router))
		defer log.Close()
		
		// Log various events
		log.Information("Regular event")
		log.With("Audit", true).Information("User {UserId} logged in", 123)
		log.With("Metric", true).Information("Response time {Duration}ms", 45)
		log.With("Audit", true, "Metric", true).Warning("Slow authentication for {UserId}", 456)
		
		// Give time for file writes
		time.Sleep(100 * time.Millisecond)
		
		// Check files
		auditContent, _ := os.ReadFile(auditFile)
		metricsContent, _ := os.ReadFile(metricsFile)
		allContent, _ := os.ReadFile(allFile)
		
		// Audit file should have audit events
		if !contains(string(auditContent), "User 123 logged in") {
			t.Error("Audit file should contain login event")
		}
		if !contains(string(auditContent), "Slow authentication") {
			t.Error("Audit file should contain slow auth event")
		}
		
		// Metrics file should have metric events
		if !contains(string(metricsContent), "Response time 45ms") {
			t.Error("Metrics file should contain response time")
		}
		if !contains(string(metricsContent), "Slow authentication") {
			t.Error("Metrics file should contain slow auth event")
		}
		
		// All file should have everything
		if !contains(string(allContent), "Regular event") {
			t.Error("All file should contain regular event")
		}
		if !contains(string(allContent), "User 123 logged in") {
			t.Error("All file should contain login event")
		}
		if !contains(string(allContent), "Response time 45ms") {
			t.Error("All file should contain response time")
		}
		if !contains(string(allContent), "Slow authentication") {
			t.Error("All file should contain slow auth event")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || (len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr))))
}