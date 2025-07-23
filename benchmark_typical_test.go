package mtlog_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
)

// discardSink for benchmarking
type discardSink struct{}

func (d *discardSink) Emit(event *core.LogEvent) {}
func (d *discardSink) Close() error              { return nil }

// Typical application structs
type User struct {
	ID        int
	Username  string
	Email     string
	CreatedAt time.Time
}

type Request struct {
	Method    string
	Path      string
	UserID    int
	RequestID string
	StartTime time.Time
}

// BenchmarkTypicalWebRequest simulates typical web request logging
func BenchmarkTypicalWebRequest(b *testing.B) {
	req := Request{
		Method:    "POST",
		Path:      "/api/users/123/orders",
		UserID:    123,
		RequestID: "req-456-789",
		StartTime: time.Now(),
	}
	duration := 234 * time.Millisecond
	statusCode := 201
	
	b.Run("mtlog", func(b *testing.B) {
		logger := mtlog.New(
			mtlog.WithSink(&discardSink{}),
			mtlog.WithMinimumLevel(core.InformationLevel),
		)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Information("HTTP {Method} {Path} completed with {StatusCode} in {Duration}ms",
				req.Method, req.Path, statusCode, duration.Milliseconds())
		}
	})
	
	b.Run("mtlog-structured", func(b *testing.B) {
		logger := mtlog.New(
			mtlog.WithSink(&discardSink{}),
			mtlog.WithMinimumLevel(core.InformationLevel),
		)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.ForContext("method", req.Method).
				ForContext("path", req.Path).
				ForContext("user_id", req.UserID).
				ForContext("request_id", req.RequestID).
				ForContext("status_code", statusCode).
				ForContext("duration_ms", duration.Milliseconds()).
				Information("HTTP request completed")
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		logger := newZapLogger(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info("HTTP request completed",
				zap.String("method", req.Method),
				zap.String("path", req.Path),
				zap.Int("status_code", statusCode),
				zap.Int64("duration_ms", duration.Milliseconds()),
			)
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info().
				Str("method", req.Method).
				Str("path", req.Path).
				Int("status_code", statusCode).
				Int64("duration_ms", duration.Milliseconds()).
				Msg("HTTP request completed")
		}
	})
}

// BenchmarkTypicalError simulates typical error logging with stack trace
func BenchmarkTypicalError(b *testing.B) {
	err := errors.New("connection timeout")
	user := User{
		ID:       789,
		Username: "testuser",
		Email:    "test@example.com",
	}
	operation := "database.query"
	
	b.Run("mtlog", func(b *testing.B) {
		logger := mtlog.New(
			mtlog.WithSink(&discardSink{}),
			mtlog.WithMinimumLevel(core.InformationLevel),
		)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Error("Operation {Operation} failed for user {UserId}: {Error}",
				operation, user.ID, err)
		}
	})
	
	b.Run("mtlog-structured", func(b *testing.B) {
		logger := mtlog.New(
			mtlog.WithSink(&discardSink{}),
			mtlog.WithMinimumLevel(core.InformationLevel),
		)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.ForContext("operation", operation).
				ForContext("user", user).
				ForContext("error", err).
				Error("Operation failed")
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		logger := newZapLogger(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Error("Operation failed",
				zap.String("operation", operation),
				zap.Int("user_id", user.ID),
				zap.Error(err),
			)
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Error().
				Str("operation", operation).
				Int("user_id", user.ID).
				Err(err).
				Msg("Operation failed")
		}
	})
}

// BenchmarkTypicalDebugWithContext simulates debug logging with context
func BenchmarkTypicalDebugWithContext(b *testing.B) {
	ctx := context.WithValue(context.Background(), "request_id", "req-123")
	ctx = context.WithValue(ctx, "user_id", 456)
	
	query := "SELECT * FROM orders WHERE user_id = ?"
	params := []interface{}{456}
	rowCount := 25
	
	b.Run("mtlog", func(b *testing.B) {
		logger := mtlog.New(
			mtlog.WithSink(&discardSink{}),
			mtlog.WithMinimumLevel(core.DebugLevel),
		)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.WithContext(ctx).
				Debug("Executed query {Query} with {ParamCount} parameters, returned {RowCount} rows",
					query, len(params), rowCount)
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		logger := newZapLogger(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Debug("Executed query",
				zap.String("query", query),
				zap.Int("param_count", len(params)),
				zap.Int("row_count", rowCount),
			)
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(io.Discard).With().Logger()
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Debug().
				Str("query", query).
				Int("param_count", len(params)).
				Int("row_count", rowCount).
				Msg("Executed query")
		}
	})
}

// BenchmarkTypicalBatchOperation simulates logging in a loop (batch processing)
func BenchmarkTypicalBatchOperation(b *testing.B) {
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}
	
	b.Run("mtlog", func(b *testing.B) {
		logger := mtlog.New(
			mtlog.WithSink(&discardSink{}),
			mtlog.WithMinimumLevel(core.InformationLevel),
		)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			processed := 0
			for _, item := range items {
				if item%10 == 0 {
					logger.Debug("Processing item {ItemID}", item)
				}
				processed++
			}
			logger.Information("Batch processing completed: {ProcessedCount} items", processed)
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		logger := newZapLogger(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			processed := 0
			for _, item := range items {
				if item%10 == 0 {
					logger.Debug("Processing item", zap.Int("item_id", item))
				}
				processed++
			}
			logger.Info("Batch processing completed", zap.Int("processed_count", processed))
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			processed := 0
			for _, item := range items {
				if item%10 == 0 {
					logger.Debug().Int("item_id", item).Msg("Processing item")
				}
				processed++
			}
			logger.Info().Int("processed_count", processed).Msg("Batch processing completed")
		}
	})
}

// BenchmarkTypicalMetrics simulates metric/monitoring style logging
func BenchmarkTypicalMetrics(b *testing.B) {
	metrics := map[string]interface{}{
		"cpu_usage":      75.5,
		"memory_usage":   82.3,
		"goroutines":     1523,
		"gc_pause_ns":    125000,
		"requests_sec":   1250.5,
		"errors_total":   23,
		"latency_p99_ms": 45,
	}
	
	b.Run("mtlog", func(b *testing.B) {
		logger := mtlog.New(
			mtlog.WithSink(&discardSink{}),
			mtlog.WithMinimumLevel(core.InformationLevel),
		)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Information("System metrics: CPU={CPU:F1}% Memory={Memory:F1}% Goroutines={Goroutines} GC={GCPause}μs RPS={RPS:F1} Errors={Errors} P99={P99}ms",
				metrics["cpu_usage"], metrics["memory_usage"], metrics["goroutines"],
				metrics["gc_pause_ns"].(int)/1000, metrics["requests_sec"],
				metrics["errors_total"], metrics["latency_p99_ms"])
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		logger := newZapLogger(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info("System metrics",
				zap.Float64("cpu_usage", metrics["cpu_usage"].(float64)),
				zap.Float64("memory_usage", metrics["memory_usage"].(float64)),
				zap.Int("goroutines", metrics["goroutines"].(int)),
				zap.Int("gc_pause_us", metrics["gc_pause_ns"].(int)/1000),
				zap.Float64("requests_sec", metrics["requests_sec"].(float64)),
				zap.Int("errors_total", metrics["errors_total"].(int)),
				zap.Int("latency_p99_ms", metrics["latency_p99_ms"].(int)),
			)
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info().
				Float64("cpu_usage", metrics["cpu_usage"].(float64)).
				Float64("memory_usage", metrics["memory_usage"].(float64)).
				Int("goroutines", metrics["goroutines"].(int)).
				Int("gc_pause_us", metrics["gc_pause_ns"].(int)/1000).
				Float64("requests_sec", metrics["requests_sec"].(float64)).
				Int("errors_total", metrics["errors_total"].(int)).
				Int("latency_p99_ms", metrics["latency_p99_ms"].(int)).
				Msg("System metrics")
		}
	})
}

// Helper function to create zap logger
func newZapLogger(w io.Writer) *zap.Logger {
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		TimeKey:        "time",
		NameKey:        "logger",
		CallerKey:      "caller",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(w),
		zapcore.InfoLevel,
	)
	
	return zap.New(core)
}