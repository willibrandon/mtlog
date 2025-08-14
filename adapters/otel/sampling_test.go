package otel_test

import (
	"fmt"
	"testing"
	"time"

	mtlogotel "github.com/willibrandon/mtlog/adapters/otel"
	"github.com/willibrandon/mtlog/core"
)

func TestRateSampler(t *testing.T) {
	tests := []struct {
		name         string
		rate         float64
		numEvents    int
		expectedMin  int
		expectedMax  int
	}{
		{"0% sampling", 0.0, 1000, 0, 0},
		{"100% sampling", 1.0, 1000, 1000, 1000},
		{"50% sampling", 0.5, 1000, 400, 600}, // Allow 40-60% range
		{"10% sampling", 0.1, 1000, 50, 150},   // Allow 5-15% range
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := mtlogotel.NewRateSampler(tt.rate)
			sampled := 0
			
			for i := 0; i < tt.numEvents; i++ {
				event := &core.LogEvent{
					Level:           core.InformationLevel,
					MessageTemplate: "Test message",
				}
				if sampler.ShouldSample(event) {
					sampled++
				}
			}
			
			if sampled < tt.expectedMin || sampled > tt.expectedMax {
				t.Errorf("Expected %d-%d sampled events, got %d", 
					tt.expectedMin, tt.expectedMax, sampled)
			}
		})
	}
}

func TestCounterSampler(t *testing.T) {
	tests := []struct {
		name      string
		n         uint64
		numEvents int
		expected  int
	}{
		{"Every event", 1, 100, 100},
		{"Every 2nd event", 2, 100, 50},
		{"Every 10th event", 10, 100, 10},
		{"Every 3rd event", 3, 99, 33},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := mtlogotel.NewCounterSampler(tt.n)
			sampled := 0
			
			for i := 0; i < tt.numEvents; i++ {
				event := &core.LogEvent{
					Level:           core.InformationLevel,
					MessageTemplate: "Test message",
				}
				if sampler.ShouldSample(event) {
					sampled++
				}
			}
			
			if sampled != tt.expected {
				t.Errorf("Expected %d sampled events, got %d", tt.expected, sampled)
			}
		})
	}
}

func TestLevelSampler(t *testing.T) {
	sampler := mtlogotel.NewLevelSampler(core.WarningLevel)
	
	tests := []struct {
		level    core.LogEventLevel
		expected bool
	}{
		{core.VerboseLevel, false},
		{core.DebugLevel, false},
		{core.InformationLevel, false},
		{core.WarningLevel, true},
		{core.ErrorLevel, true},
		{core.FatalLevel, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Level%d", int(tt.level)), func(t *testing.T) {
			event := &core.LogEvent{
				Level:           tt.level,
				MessageTemplate: "Test message",
			}
			
			result := sampler.ShouldSample(event)
			if result != tt.expected {
				t.Errorf("Expected %v for level %v, got %v", 
					tt.expected, tt.level, result)
			}
		})
	}
}

func TestAdaptiveSampler(t *testing.T) {
	// Test that adaptive sampler adjusts rate
	targetRate := uint64(100) // 100 events per second
	sampler := mtlogotel.NewAdaptiveSampler(targetRate)
	
	// Simulate high volume (should reduce sampling)
	highVolumeSampled := 0
	for i := 0; i < 1000; i++ {
		event := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "High volume message",
		}
		if sampler.ShouldSample(event) {
			highVolumeSampled++
		}
	}
	
	// Wait for window to reset
	time.Sleep(1100 * time.Millisecond)
	
	// Simulate low volume (should increase sampling)
	lowVolumeSampled := 0
	for i := 0; i < 50; i++ {
		event := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "Low volume message",
		}
		if sampler.ShouldSample(event) {
			lowVolumeSampled++
		}
	}
	
	t.Logf("High volume sampled: %d/1000", highVolumeSampled)
	t.Logf("Low volume sampled: %d/50", lowVolumeSampled)
	
	// Low volume should have higher sampling rate
	highVolumeRate := float64(highVolumeSampled) / 1000.0
	lowVolumeRate := float64(lowVolumeSampled) / 50.0
	
	if lowVolumeRate <= highVolumeRate {
		t.Errorf("Expected low volume rate (%f) > high volume rate (%f)", 
			lowVolumeRate, highVolumeRate)
	}
}

func TestCompositeSampler(t *testing.T) {
	t.Run("AllMode", func(t *testing.T) {
		// All samplers must agree
		sampler := mtlogotel.NewCompositeSampler(
			mtlogotel.AllMode,
			mtlogotel.NewRateSampler(1.0), // Always true
			mtlogotel.NewLevelSampler(core.WarningLevel),
		)
		
		// Should be false for Info level (level sampler rejects)
		infoEvent := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "Info message",
		}
		if sampler.ShouldSample(infoEvent) {
			t.Error("Expected Info event to be rejected")
		}
		
		// Should be true for Warning level (both accept)
		warnEvent := &core.LogEvent{
			Level:           core.WarningLevel,
			MessageTemplate: "Warning message",
		}
		if !sampler.ShouldSample(warnEvent) {
			t.Error("Expected Warning event to be accepted")
		}
	})
	
	t.Run("AnyMode", func(t *testing.T) {
		// Any sampler can accept
		sampler := mtlogotel.NewCompositeSampler(
			mtlogotel.AnyMode,
			mtlogotel.NewRateSampler(0.0), // Always false
			mtlogotel.NewLevelSampler(core.WarningLevel),
		)
		
		// Should be false for Info level (both reject)
		infoEvent := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "Info message",
		}
		if sampler.ShouldSample(infoEvent) {
			t.Error("Expected Info event to be rejected")
		}
		
		// Should be true for Warning level (level sampler accepts)
		warnEvent := &core.LogEvent{
			Level:           core.WarningLevel,
			MessageTemplate: "Warning message",
		}
		if !sampler.ShouldSample(warnEvent) {
			t.Error("Expected Warning event to be accepted")
		}
	})
}

func TestSamplingSink(t *testing.T) {
	// Create a memory sink to wrap
	var emitted []*core.LogEvent
	memorySink := mtlogotel.NewTestSink(func(event *core.LogEvent) {
		emitted = append(emitted, event)
	})
	
	// Wrap with sampling (50% rate)
	samplingSink := mtlogotel.NewSamplingSink(
		memorySink,
		mtlogotel.NewRateSampler(0.5),
	)
	
	// Emit 1000 events
	for i := 0; i < 1000; i++ {
		event := &core.LogEvent{
			Level:           core.InformationLevel,
			MessageTemplate: "Test message",
			Timestamp:       time.Now(),
		}
		samplingSink.Emit(event)
	}
	
	// Check stats
	sampled, dropped := samplingSink.GetStats()
	total := sampled + dropped
	
	if total != 1000 {
		t.Errorf("Expected 1000 total events, got %d", total)
	}
	
	// Should be roughly 50% sampled (allow 40-60% range)
	if sampled < 400 || sampled > 600 {
		t.Errorf("Expected 400-600 sampled events, got %d", sampled)
	}
	
	if len(emitted) != int(sampled) {
		t.Errorf("Expected %d emitted events, got %d", sampled, len(emitted))
	}
}


func BenchmarkSamplers(b *testing.B) {
	event := &core.LogEvent{
		Level:           core.InformationLevel,
		MessageTemplate: "Benchmark message",
		Timestamp:       time.Now(),
	}
	
	b.Run("RateSampler", func(b *testing.B) {
		sampler := mtlogotel.NewRateSampler(0.5)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = sampler.ShouldSample(event)
		}
	})
	
	b.Run("CounterSampler", func(b *testing.B) {
		sampler := mtlogotel.NewCounterSampler(10)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = sampler.ShouldSample(event)
		}
	})
	
	b.Run("LevelSampler", func(b *testing.B) {
		sampler := mtlogotel.NewLevelSampler(core.WarningLevel)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = sampler.ShouldSample(event)
		}
	})
	
	b.Run("AdaptiveSampler", func(b *testing.B) {
		sampler := mtlogotel.NewAdaptiveSampler(1000)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = sampler.ShouldSample(event)
		}
	})
}