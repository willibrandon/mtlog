package mtlog

import (
	"sync/atomic"
	"time"

	"github.com/willibrandon/mtlog/core"
)

// AdaptiveSamplingFilter adjusts sampling rates based on system load or event frequency.
// Features hysteresis, dampening, and configurable aggressiveness for stable production use.
type AdaptiveSamplingFilter struct {
	targetEventsPerSecond   uint64
	currentRate             atomic.Uint64 // Stored as uint64, represents rate * 1000000
	eventCount              atomic.Uint64
	lastAdjustment          atomic.Int64  // Unix timestamp
	adjustmentInterval      time.Duration
	minRate                 float64
	maxRate                 float64
	hysteresisThreshold     float64       // Threshold for change before adjusting (prevents oscillation)
	aggressiveness         float64       // How quickly to adjust (0.1 = conservative, 0.5 = moderate, 0.9 = aggressive)
	dampeningFactor        float64       // Additional dampening for extreme load variations (0.1 = heavy dampening, 0.9 = light dampening)
	previousEventsPerSecond atomic.Uint64 // Previous period's events per second for smoothing
	adjustmentHistory      [3]float64    // History of recent adjustments for dampening calculations
	historyIndex           int           // Current index in adjustment history (not atomic, only used by single adjuster)
}

// NewAdaptiveSamplingFilter creates a filter that adjusts sampling based on target events per second.
func NewAdaptiveSamplingFilter(targetEventsPerSecond uint64) *AdaptiveSamplingFilter {
	filter := &AdaptiveSamplingFilter{
		targetEventsPerSecond:  targetEventsPerSecond,
		adjustmentInterval:     1 * time.Second, // Adjust every second
		minRate:               0.001, // 0.1% minimum
		maxRate:               1.0,   // 100% maximum
		hysteresisThreshold:    0.15, // 15% threshold for changes to prevent oscillation
		aggressiveness:         0.3,  // Moderate adjustment speed
		dampeningFactor:        0.7,  // Moderate dampening for stability
	}
	
	// Start with 50% sampling rate
	filter.currentRate.Store(uint64(0.5 * 1000000))
	filter.lastAdjustment.Store(time.Now().Unix())
	filter.previousEventsPerSecond.Store(uint64(float64(targetEventsPerSecond) * 0.5))
	
	return filter
}

// NewAdaptiveSamplingFilterWithOptions creates a filter with custom options.
func NewAdaptiveSamplingFilterWithOptions(targetEventsPerSecond uint64, minRate, maxRate float64, adjustmentInterval time.Duration) *AdaptiveSamplingFilter {
	if minRate < 0 {
		minRate = 0.001
	}
	if maxRate > 1.0 {
		maxRate = 1.0
	}
	if minRate >= maxRate {
		minRate = 0.001
		maxRate = 1.0
	}
	if adjustmentInterval <= 0 {
		adjustmentInterval = 1 * time.Second
	}
	
	filter := &AdaptiveSamplingFilter{
		targetEventsPerSecond:  targetEventsPerSecond,
		adjustmentInterval:     adjustmentInterval,
		minRate:               minRate,
		maxRate:               maxRate,
		hysteresisThreshold:    0.15, // 15% threshold for changes to prevent oscillation
		aggressiveness:         0.3,  // Moderate adjustment speed
		dampeningFactor:        0.7,  // Moderate dampening for stability
	}
	
	// Start with middle rate
	initialRate := (minRate + maxRate) / 2
	filter.currentRate.Store(uint64(initialRate * 1000000))
	filter.lastAdjustment.Store(time.Now().Unix())
	filter.previousEventsPerSecond.Store(uint64(float64(targetEventsPerSecond) * initialRate))
	
	return filter
}

// NewAdaptiveSamplingFilterWithHysteresis creates a filter with hysteresis and aggressiveness control for stability.
func NewAdaptiveSamplingFilterWithHysteresis(targetEventsPerSecond uint64, minRate, maxRate float64, adjustmentInterval time.Duration, hysteresisThreshold, aggressiveness float64) *AdaptiveSamplingFilter {
	if minRate < 0 {
		minRate = 0.001
	}
	if maxRate > 1.0 {
		maxRate = 1.0
	}
	if minRate >= maxRate {
		minRate = 0.001
		maxRate = 1.0
	}
	if adjustmentInterval <= 0 {
		adjustmentInterval = 1 * time.Second
	}
	if hysteresisThreshold < 0 {
		hysteresisThreshold = 0.05 // 5% minimum
	}
	if hysteresisThreshold > 0.5 {
		hysteresisThreshold = 0.5 // 50% maximum
	}
	if aggressiveness <= 0 {
		aggressiveness = 0.1 // Conservative minimum
	}
	if aggressiveness > 1.0 {
		aggressiveness = 1.0 // Maximum
	}
	
	filter := &AdaptiveSamplingFilter{
		targetEventsPerSecond:  targetEventsPerSecond,
		adjustmentInterval:     adjustmentInterval,
		minRate:               minRate,
		maxRate:               maxRate,
		hysteresisThreshold:    hysteresisThreshold,
		aggressiveness:         aggressiveness,
		dampeningFactor:        0.8, // Default moderate dampening
	}
	
	// Start with middle rate
	initialRate := (minRate + maxRate) / 2
	filter.currentRate.Store(uint64(initialRate * 1000000))
	filter.lastAdjustment.Store(time.Now().Unix())
	filter.previousEventsPerSecond.Store(uint64(float64(targetEventsPerSecond) * initialRate))
	
	return filter
}

// NewAdaptiveSamplingFilterWithDampening creates a filter with complete control including dampening factor.
func NewAdaptiveSamplingFilterWithDampening(targetEventsPerSecond uint64, minRate, maxRate float64, adjustmentInterval time.Duration, hysteresisThreshold, aggressiveness, dampeningFactor float64) *AdaptiveSamplingFilter {
	if minRate < 0 {
		minRate = 0.001
	}
	if maxRate > 1.0 {
		maxRate = 1.0
	}
	if minRate >= maxRate {
		minRate = 0.001
		maxRate = 1.0
	}
	if adjustmentInterval <= 0 {
		adjustmentInterval = 1 * time.Second
	}
	if hysteresisThreshold < 0 {
		hysteresisThreshold = 0.05 // 5% minimum
	}
	if hysteresisThreshold > 0.5 {
		hysteresisThreshold = 0.5 // 50% maximum
	}
	if aggressiveness <= 0 {
		aggressiveness = 0.1 // Conservative minimum
	}
	if aggressiveness > 1.0 {
		aggressiveness = 1.0 // Maximum
	}
	if dampeningFactor <= 0 {
		dampeningFactor = 0.1 // Heavy dampening minimum
	}
	if dampeningFactor > 1.0 {
		dampeningFactor = 1.0 // No dampening maximum
	}
	
	filter := &AdaptiveSamplingFilter{
		targetEventsPerSecond:  targetEventsPerSecond,
		adjustmentInterval:     adjustmentInterval,
		minRate:               minRate,
		maxRate:               maxRate,
		hysteresisThreshold:    hysteresisThreshold,
		aggressiveness:         aggressiveness,
		dampeningFactor:        dampeningFactor,
	}
	
	// Start with middle rate
	initialRate := (minRate + maxRate) / 2
	filter.currentRate.Store(uint64(initialRate * 1000000))
	filter.lastAdjustment.Store(time.Now().Unix())
	filter.previousEventsPerSecond.Store(uint64(float64(targetEventsPerSecond) * initialRate))
	
	return filter
}

// DampeningPreset represents a predefined dampening configuration
type DampeningPreset int

const (
	// DampeningConservative - Heavy dampening for stable, predictable environments
	DampeningConservative DampeningPreset = iota
	// DampeningModerate - Balanced dampening for general use (default)
	DampeningModerate
	// DampeningAggressive - Light dampening for dynamic environments that need quick response
	DampeningAggressive
	// DampeningUltraStable - Maximum dampening for critical systems where stability is paramount
	DampeningUltraStable
	// DampeningResponsive - Minimal dampening for development or testing environments
	DampeningResponsive
)

// DampeningConfig holds the configuration for a dampening preset
type DampeningConfig struct {
	Name                string
	Description         string
	HysteresisThreshold float64 // Threshold before making adjustments
	Aggressiveness      float64 // How quickly to adjust rates
	DampeningFactor     float64 // Additional dampening for extreme variations
	AdjustmentInterval  time.Duration // How often to check for adjustments
}

// GetDampeningConfig returns the configuration for a given preset
func GetDampeningConfig(preset DampeningPreset) DampeningConfig {
	switch preset {
	case DampeningConservative:
		return DampeningConfig{
			Name:                "Conservative",
			Description:         "Heavy dampening for stable, predictable production environments",
			HysteresisThreshold: 0.25,              // 25% threshold - requires significant change
			Aggressiveness:      0.15,              // Very slow adjustments
			DampeningFactor:     0.5,               // Heavy dampening
			AdjustmentInterval:  3 * time.Second,   // Check every 3 seconds
		}
	case DampeningModerate:
		return DampeningConfig{
			Name:                "Moderate",
			Description:         "Balanced dampening suitable for most production environments",
			HysteresisThreshold: 0.15,              // 15% threshold - moderate sensitivity
			Aggressiveness:      0.3,               // Moderate adjustment speed
			DampeningFactor:     0.7,               // Moderate dampening
			AdjustmentInterval:  1 * time.Second,   // Check every second
		}
	case DampeningAggressive:
		return DampeningConfig{
			Name:                "Aggressive",
			Description:         "Light dampening for dynamic environments requiring quick response",
			HysteresisThreshold: 0.08,              // 8% threshold - high sensitivity
			Aggressiveness:      0.6,               // Fast adjustments
			DampeningFactor:     0.85,              // Light dampening
			AdjustmentInterval:  500 * time.Millisecond, // Check every 500ms
		}
	case DampeningUltraStable:
		return DampeningConfig{
			Name:                "Ultra Stable",
			Description:         "Maximum dampening for critical systems where stability is paramount",
			HysteresisThreshold: 0.4,               // 40% threshold - very high stability
			Aggressiveness:      0.05,              // Extremely slow adjustments
			DampeningFactor:     0.3,               // Maximum dampening
			AdjustmentInterval:  5 * time.Second,   // Check every 5 seconds
		}
	case DampeningResponsive:
		return DampeningConfig{
			Name:                "Responsive",
			Description:         "Minimal dampening for development or testing environments",
			HysteresisThreshold: 0.05,              // 5% threshold - very sensitive
			Aggressiveness:      0.8,               // Very fast adjustments
			DampeningFactor:     0.95,              // Minimal dampening
			AdjustmentInterval:  200 * time.Millisecond, // Check every 200ms
		}
	default:
		// Default to moderate
		return GetDampeningConfig(DampeningModerate)
	}
}

// NewAdaptiveSamplingFilterWithPreset creates a filter using a predefined dampening preset
func NewAdaptiveSamplingFilterWithPreset(targetEventsPerSecond uint64, preset DampeningPreset, minRate, maxRate float64) *AdaptiveSamplingFilter {
	config := GetDampeningConfig(preset)
	
	return NewAdaptiveSamplingFilterWithDampening(
		targetEventsPerSecond,
		minRate,
		maxRate,
		config.AdjustmentInterval,
		config.HysteresisThreshold,
		config.Aggressiveness,
		config.DampeningFactor,
	)
}

// NewAdaptiveSamplingFilterPresetDefaults creates a filter using a preset with default rate limits
func NewAdaptiveSamplingFilterPresetDefaults(targetEventsPerSecond uint64, preset DampeningPreset) *AdaptiveSamplingFilter {
	return NewAdaptiveSamplingFilterWithPreset(targetEventsPerSecond, preset, 0.001, 1.0)
}

// GetAvailableDampeningPresets returns descriptions of all available presets
func GetAvailableDampeningPresets() []DampeningConfig {
	return []DampeningConfig{
		GetDampeningConfig(DampeningConservative),
		GetDampeningConfig(DampeningModerate),
		GetDampeningConfig(DampeningAggressive),
		GetDampeningConfig(DampeningUltraStable),
		GetDampeningConfig(DampeningResponsive),
	}
}

// IsEnabled implements core.LogEventFilter.
func (f *AdaptiveSamplingFilter) IsEnabled(event *core.LogEvent) bool {
	now := time.Now()
	
	// Check if we need to adjust the rate
	lastAdjust := f.lastAdjustment.Load()
	if now.Unix()-lastAdjust >= int64(f.adjustmentInterval.Seconds()) {
		f.adjustSamplingRate(now)
	}
	
	// Increment event counter
	f.eventCount.Add(1)
	
	// Apply current sampling rate using pseudo-random decision
	currentRateRaw := f.currentRate.Load()
	currentRate := float64(currentRateRaw) / 1000000.0
	
	// Use event timestamp hash for deterministic but pseudo-random sampling
	hash := f.hashEvent(event)
	threshold := uint32(float64(^uint32(0)) * currentRate)
	
	return hash <= threshold
}

// adjustSamplingRate adjusts the sampling rate based on recent event frequency.
// Uses hysteresis to prevent oscillation and exponential smoothing for stability.
func (f *AdaptiveSamplingFilter) adjustSamplingRate(now time.Time) {
	// Try to update the last adjustment time atomically
	lastAdjust := f.lastAdjustment.Load()
	if !f.lastAdjustment.CompareAndSwap(lastAdjust, now.Unix()) {
		// Another goroutine is already adjusting
		return
	}
	
	// Calculate events per second since last adjustment
	elapsed := now.Unix() - lastAdjust
	if elapsed <= 0 {
		return
	}
	
	eventCount := f.eventCount.Swap(0) // Reset counter and get current count
	currentEventsPerSecond := float64(eventCount) / float64(elapsed)
	
	// Get previous events per second for smoothing
	previousEventsPerSecond := float64(f.previousEventsPerSecond.Load())
	
	// Apply exponential smoothing to the events per second measurement
	// This prevents sudden spikes/drops from causing overreactions
	smoothedEventsPerSecond := previousEventsPerSecond*0.7 + currentEventsPerSecond*0.3
	f.previousEventsPerSecond.Store(uint64(smoothedEventsPerSecond))
	
	// Calculate current rate
	currentRateRaw := f.currentRate.Load()
	currentRate := float64(currentRateRaw) / 1000000.0
	
	// Calculate the deviation from target
	target := float64(f.targetEventsPerSecond)
	deviation := (smoothedEventsPerSecond - target) / target
	
	// Apply hysteresis - only adjust if deviation exceeds threshold
	if deviation < 0 {
		deviation = -deviation // Make positive for comparison
	}
	if deviation < f.hysteresisThreshold {
		// Within hysteresis band, don't adjust
		return
	}
	
	// Calculate adjustment factor based on how far we are from target
	var adjustmentFactor float64
	if smoothedEventsPerSecond > 0 {
		adjustmentFactor = target / smoothedEventsPerSecond
	} else {
		adjustmentFactor = 2.0 // If no events, increase rate significantly
	}
	
	// Apply aggressiveness factor to control adjustment speed
	// aggressiveness: 0.1 = very conservative, 0.5 = moderate, 0.9 = aggressive
	proposedRate := currentRate * (1 + f.aggressiveness*(adjustmentFactor-1))
	
	// Calculate the proposed change magnitude
	changeAmount := proposedRate - currentRate
	
	// Apply dampening factor to reduce oscillations under extreme load variations
	// Store this adjustment in history for dampening calculations
	f.adjustmentHistory[f.historyIndex] = changeAmount
	f.historyIndex = (f.historyIndex + 1) % len(f.adjustmentHistory)
	
	// Calculate dampening based on recent adjustment history
	var oscillationDetected bool
	
	// Check for oscillation patterns in adjustment history
	if f.adjustmentHistory[0] != 0 && f.adjustmentHistory[1] != 0 && f.adjustmentHistory[2] != 0 {
		// Check if recent changes are alternating in direction (oscillation pattern)
		sign1 := f.adjustmentHistory[0] > 0
		sign2 := f.adjustmentHistory[1] > 0
		sign3 := f.adjustmentHistory[2] > 0
		
		// Oscillation detected if signs alternate
		oscillationDetected = (sign1 != sign2) && (sign2 != sign3)
	}
	
	// Apply dampening factor with increased dampening if oscillation is detected
	var effectiveDampeningFactor float64
	if oscillationDetected {
		// Increase dampening when oscillation is detected
		effectiveDampeningFactor = f.dampeningFactor * 0.5 // Reduce effective dampening factor by half
	} else {
		effectiveDampeningFactor = f.dampeningFactor
	}
	
	// Apply dampening to the change amount
	dampenedChange := changeAmount * effectiveDampeningFactor
	newRate := currentRate + dampenedChange
	
	// Additional smoothing: limit how much the rate can change in one adjustment
	maxRateChange := currentRate * 0.5 // Maximum 50% change per adjustment
	if newRate > currentRate+maxRateChange {
		newRate = currentRate + maxRateChange
	} else if newRate < currentRate-maxRateChange {
		newRate = currentRate - maxRateChange
	}
	
	// Clamp to bounds
	if newRate < f.minRate {
		newRate = f.minRate
	} else if newRate > f.maxRate {
		newRate = f.maxRate
	}
	
	f.currentRate.Store(uint64(newRate * 1000000))
}

// GetCurrentRate returns the current sampling rate (0.0 to 1.0).
func (f *AdaptiveSamplingFilter) GetCurrentRate() float64 {
	return float64(f.currentRate.Load()) / 1000000.0
}

// GetStats returns statistics about the adaptive sampling.
func (f *AdaptiveSamplingFilter) GetStats() AdaptiveSamplingStats {
	return AdaptiveSamplingStats{
		CurrentRate:             f.GetCurrentRate(),
		TargetEventsPerSecond:   f.targetEventsPerSecond,
		RecentEventCount:        f.eventCount.Load(),
		SmoothedEventsPerSecond: float64(f.previousEventsPerSecond.Load()),
		LastAdjustment:          time.Unix(f.lastAdjustment.Load(), 0),
		HysteresisThreshold:     f.hysteresisThreshold,
		Aggressiveness:          f.aggressiveness,
	}
}

// AdaptiveSamplingStats provides statistics about adaptive sampling behavior.
type AdaptiveSamplingStats struct {
	CurrentRate             float64
	TargetEventsPerSecond   uint64
	RecentEventCount        uint64
	SmoothedEventsPerSecond float64
	LastAdjustment          time.Time
	HysteresisThreshold     float64
	Aggressiveness          float64
}

// Reset resets the adaptive sampling state.
func (f *AdaptiveSamplingFilter) Reset() {
	initialRate := (f.minRate + f.maxRate) / 2
	f.currentRate.Store(uint64(initialRate * 1000000))
	f.eventCount.Store(0)
	f.lastAdjustment.Store(time.Now().Unix())
	f.previousEventsPerSecond.Store(uint64(float64(f.targetEventsPerSecond) * initialRate))
}

// hashEvent creates a hash from the event for pseudo-random sampling.
func (f *AdaptiveSamplingFilter) hashEvent(event *core.LogEvent) uint32 {
	// Simple hash based on timestamp and message template
	hash := uint32(2166136261) // FNV-1a offset basis
	
	// Hash timestamp
	tsBytes := uint64(event.Timestamp.UnixNano())
	// If timestamp is zero (uninitialized), use current time
	if tsBytes == 0 {
		tsBytes = uint64(time.Now().UnixNano())
	}
	
	for i := 0; i < 8; i++ {
		hash ^= uint32(tsBytes & 0xFF)
		hash *= 16777619 // FNV-1a prime
		tsBytes >>= 8
	}
	
	// Hash message template if available
	if event.MessageTemplate != "" {
		for _, b := range []byte(event.MessageTemplate) {
			hash ^= uint32(b)
			hash *= 16777619
		}
	}
	
	// Add event counter to ensure different hashes for rapid successive events
	// This helps on systems with low timestamp resolution
	counter := f.eventCount.Load()
	hash ^= uint32(counter)
	hash *= 16777619
	
	return hash
}

// SampleAdaptive creates a logger with adaptive sampling based on target events per second.
func (l *logger) SampleAdaptive(targetEventsPerSecond uint64) core.Logger {
	filter := NewAdaptiveSamplingFilter(targetEventsPerSecond)
	
	// Create new pipeline with the adaptive filter
	newFilters := make([]core.LogEventFilter, len(l.pipeline.filters)+1)
	copy(newFilters, l.pipeline.filters)
	newFilters[len(l.pipeline.filters)] = filter
	
	newPipeline := &pipeline{
		enrichers: l.pipeline.enrichers,
		filters:   newFilters,
		capturer:  l.pipeline.capturer,
		sinks:     l.pipeline.sinks,
	}
	
	return &logger{
		minimumLevel: l.minimumLevel,
		levelSwitch:  l.levelSwitch,
		pipeline:     newPipeline,
		fields:       l.fields,
		properties:   l.properties,
		samplingFilter: l.samplingFilter,
	}
}

// SampleAdaptiveWithOptions creates a logger with adaptive sampling and custom options.
func (l *logger) SampleAdaptiveWithOptions(targetEventsPerSecond uint64, minRate, maxRate float64, adjustmentInterval time.Duration) core.Logger {
	filter := NewAdaptiveSamplingFilterWithOptions(targetEventsPerSecond, minRate, maxRate, adjustmentInterval)
	
	// Create new pipeline with the adaptive filter
	newFilters := make([]core.LogEventFilter, len(l.pipeline.filters)+1)
	copy(newFilters, l.pipeline.filters)
	newFilters[len(l.pipeline.filters)] = filter
	
	newPipeline := &pipeline{
		enrichers: l.pipeline.enrichers,
		filters:   newFilters,
		capturer:  l.pipeline.capturer,
		sinks:     l.pipeline.sinks,
	}
	
	return &logger{
		minimumLevel: l.minimumLevel,
		levelSwitch:  l.levelSwitch,
		pipeline:     newPipeline,
		fields:       l.fields,
		properties:   l.properties,
		samplingFilter: l.samplingFilter,
	}
}

// SampleAdaptiveWithHysteresis creates a logger with adaptive sampling that includes hysteresis and aggressiveness control for stability.
func (l *logger) SampleAdaptiveWithHysteresis(targetEventsPerSecond uint64, minRate, maxRate float64, adjustmentInterval time.Duration, hysteresisThreshold, aggressiveness float64) core.Logger {
	filter := NewAdaptiveSamplingFilterWithHysteresis(targetEventsPerSecond, minRate, maxRate, adjustmentInterval, hysteresisThreshold, aggressiveness)
	
	// Create new pipeline with the adaptive filter
	newFilters := make([]core.LogEventFilter, len(l.pipeline.filters)+1)
	copy(newFilters, l.pipeline.filters)
	newFilters[len(l.pipeline.filters)] = filter
	
	newPipeline := &pipeline{
		enrichers: l.pipeline.enrichers,
		filters:   newFilters,
		capturer:  l.pipeline.capturer,
		sinks:     l.pipeline.sinks,
	}
	
	return &logger{
		minimumLevel: l.minimumLevel,
		levelSwitch:  l.levelSwitch,
		pipeline:     newPipeline,
		fields:       l.fields,
		properties:   l.properties,
		samplingFilter: l.samplingFilter,
	}
}

// SampleAdaptiveWithDampening creates a logger with adaptive sampling that includes dampening for extreme load variations.
func (l *logger) SampleAdaptiveWithDampening(targetEventsPerSecond uint64, minRate, maxRate float64, adjustmentInterval time.Duration, hysteresisThreshold, aggressiveness, dampeningFactor float64) core.Logger {
	filter := NewAdaptiveSamplingFilterWithDampening(targetEventsPerSecond, minRate, maxRate, adjustmentInterval, hysteresisThreshold, aggressiveness, dampeningFactor)
	
	// Create new pipeline with the adaptive filter
	newFilters := make([]core.LogEventFilter, len(l.pipeline.filters)+1)
	copy(newFilters, l.pipeline.filters)
	newFilters[len(l.pipeline.filters)] = filter
	
	newPipeline := &pipeline{
		enrichers: l.pipeline.enrichers,
		filters:   newFilters,
		capturer:  l.pipeline.capturer,
		sinks:     l.pipeline.sinks,
	}
	
	return &logger{
		minimumLevel: l.minimumLevel,
		levelSwitch:  l.levelSwitch,
		pipeline:     newPipeline,
		fields:       l.fields,
		properties:   l.properties,
		samplingFilter: l.samplingFilter,
	}
}

// SampleAdaptiveWithPreset creates a logger with adaptive sampling using a predefined dampening preset
func (l *logger) SampleAdaptiveWithPreset(targetEventsPerSecond uint64, preset DampeningPreset) core.Logger {
	filter := NewAdaptiveSamplingFilterPresetDefaults(targetEventsPerSecond, preset)
	
	// Create new pipeline with the adaptive filter
	newFilters := make([]core.LogEventFilter, len(l.pipeline.filters)+1)
	copy(newFilters, l.pipeline.filters)
	newFilters[len(l.pipeline.filters)] = filter
	
	newPipeline := &pipeline{
		enrichers: l.pipeline.enrichers,
		filters:   newFilters,
		capturer:  l.pipeline.capturer,
		sinks:     l.pipeline.sinks,
	}
	
	return &logger{
		minimumLevel: l.minimumLevel,
		levelSwitch:  l.levelSwitch,
		pipeline:     newPipeline,
		fields:       l.fields,
		properties:   l.properties,
		samplingFilter: l.samplingFilter,
	}
}

// SampleAdaptiveWithPresetCustom creates a logger with adaptive sampling using a preset and custom rate limits
func (l *logger) SampleAdaptiveWithPresetCustom(targetEventsPerSecond uint64, preset DampeningPreset, minRate, maxRate float64) core.Logger {
	filter := NewAdaptiveSamplingFilterWithPreset(targetEventsPerSecond, preset, minRate, maxRate)
	
	// Create new pipeline with the adaptive filter
	newFilters := make([]core.LogEventFilter, len(l.pipeline.filters)+1)
	copy(newFilters, l.pipeline.filters)
	newFilters[len(l.pipeline.filters)] = filter
	
	newPipeline := &pipeline{
		enrichers: l.pipeline.enrichers,
		filters:   newFilters,
		capturer:  l.pipeline.capturer,
		sinks:     l.pipeline.sinks,
	}
	
	return &logger{
		minimumLevel: l.minimumLevel,
		levelSwitch:  l.levelSwitch,
		pipeline:     newPipeline,
		fields:       l.fields,
		properties:   l.properties,
		samplingFilter: l.samplingFilter,
	}
}
