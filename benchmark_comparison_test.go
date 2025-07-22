package mtlog

import (
	"io"
	"testing"
	"time"
	
	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

// Benchmark simple string logging (no allocations target)
func BenchmarkSimpleString(b *testing.B) {
	b.Run("mtlog", func(b *testing.B) {
		logger := New(WithSink(&discardSink{}))
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Information("This is a simple log message")
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		logger := newZapLogger(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info("This is a simple log message")
		}
	})
	
	b.Run("zap-sugar", func(b *testing.B) {
		logger := newZapLogger(io.Discard).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info("This is a simple log message")
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info().Msg("This is a simple log message")
		}
	})
}

// Benchmark logging with 2 properties
func BenchmarkTwoProperties(b *testing.B) {
	b.Run("mtlog", func(b *testing.B) {
		logger := New(WithSink(&discardSink{}))
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Information("User {UserId} performed action {Action}", 123, "login")
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		logger := newZapLogger(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info("User performed action",
				zap.Int("UserId", 123),
				zap.String("Action", "login"))
		}
	})
	
	b.Run("zap-sugar", func(b *testing.B) {
		logger := newZapLogger(io.Discard).Sugar()
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Infow("User performed action",
				"UserId", 123,
				"Action", "login")
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info().
				Int("UserId", 123).
				Str("Action", "login").
				Msg("User performed action")
		}
	})
}

// Benchmark logging with 10 properties
func BenchmarkTenProperties(b *testing.B) {
	b.Run("mtlog", func(b *testing.B) {
		logger := New(WithSink(&discardSink{}))
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Information("Request {Method} {Path} {Status} {Duration} {Size} {UserAgent} {IP} {Referrer} {RequestId} {UserId}",
				"GET", "/api/users", 200, 123*time.Millisecond, 1024, "Mozilla/5.0", "192.168.1.1", "https://example.com", "req-123", 456)
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		logger := newZapLogger(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info("Request",
				zap.String("Method", "GET"),
				zap.String("Path", "/api/users"),
				zap.Int("Status", 200),
				zap.Duration("Duration", 123*time.Millisecond),
				zap.Int("Size", 1024),
				zap.String("UserAgent", "Mozilla/5.0"),
				zap.String("IP", "192.168.1.1"),
				zap.String("Referrer", "https://example.com"),
				zap.String("RequestId", "req-123"),
				zap.Int("UserId", 456))
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info().
				Str("Method", "GET").
				Str("Path", "/api/users").
				Int("Status", 200).
				Dur("Duration", 123*time.Millisecond).
				Int("Size", 1024).
				Str("UserAgent", "Mozilla/5.0").
				Str("IP", "192.168.1.1").
				Str("Referrer", "https://example.com").
				Str("RequestId", "req-123").
				Int("UserId", 456).
				Msg("Request")
		}
	})
}

// Benchmark logging with context (enriched logger)
func BenchmarkWithContext(b *testing.B) {
	b.Run("mtlog", func(b *testing.B) {
		logger := New(WithSink(&discardSink{}))
		contextLogger := logger.ForContext("Environment", "Production").
			ForContext("Service", "API").
			ForContext("Version", "1.0.0")
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			contextLogger.Information("Processing request")
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		logger := newZapLogger(io.Discard)
		contextLogger := logger.With(
			zap.String("Environment", "Production"),
			zap.String("Service", "API"),
			zap.String("Version", "1.0.0"))
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			contextLogger.Info("Processing request")
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(io.Discard)
		contextLogger := logger.With().
			Str("Environment", "Production").
			Str("Service", "API").
			Str("Version", "1.0.0").
			Logger()
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			contextLogger.Info().Msg("Processing request")
		}
	})
}

// Benchmark logging with complex object (struct)
func BenchmarkComplexObject(b *testing.B) {
	type User struct {
		ID    int
		Name  string
		Email string
		Role  string
	}
	
	user := User{
		ID:    123,
		Name:  "Alice",
		Email: "alice@example.com",
		Role:  "Admin",
	}
	
	b.Run("mtlog", func(b *testing.B) {
		logger := New(
			WithSink(&discardSink{}),
			WithDestructuring(),
		)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Information("Processing user {@User}", user)
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		logger := newZapLogger(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info("Processing user",
				zap.Any("User", user))
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(io.Discard)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info().
				Interface("User", user).
				Msg("Processing user")
		}
	})
}

// Benchmark logging below minimum level (should be very fast)
func BenchmarkFilteredOut(b *testing.B) {
	b.Run("mtlog", func(b *testing.B) {
		logger := New(
			WithSink(&discardSink{}),
			WithMinimumLevel(core.InformationLevel),
		)
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Debug("This should be filtered out")
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		cfg := zap.NewProductionEncoderConfig()
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(cfg),
			zapcore.AddSync(io.Discard),
			zapcore.InfoLevel,
		)
		logger := zap.New(core)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Debug("This should be filtered out")
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		logger := zerolog.New(io.Discard)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Debug().Msg("This should be filtered out")
		}
	})
}

// Benchmark with console output formatting
func BenchmarkConsoleOutput(b *testing.B) {
	b.Run("mtlog", func(b *testing.B) {
		logger := New(WithSink(sinks.NewConsoleSinkWithWriter(io.Discard)))
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Information("User {UserId} logged in", 123)
		}
	})
	
	b.Run("zap", func(b *testing.B) {
		cfg := zap.NewDevelopmentEncoderConfig()
		core := zapcore.NewCore(
			zapcore.NewConsoleEncoder(cfg),
			zapcore.AddSync(io.Discard),
			zapcore.InfoLevel,
		)
		logger := zap.New(core)
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info("User logged in", zap.Int("UserId", 123))
		}
	})
	
	b.Run("zerolog", func(b *testing.B) {
		logger := zerolog.New(zerolog.ConsoleWriter{Out: io.Discard})
		
		b.ResetTimer()
		b.ReportAllocs()
		
		for i := 0; i < b.N; i++ {
			logger.Info().Int("UserId", 123).Msg("User logged in")
		}
	})
}

// Helper to create a zap logger
func newZapLogger(w io.Writer) *zap.Logger {
	cfg := zap.NewProductionEncoderConfig()
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg),
		zapcore.AddSync(w),
		zapcore.InfoLevel,
	)
	return zap.New(core)
}