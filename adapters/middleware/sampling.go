package middleware

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Sampler determines whether a request should be logged.
// Implementations MUST be thread-safe as ShouldSample may be
// called concurrently from multiple goroutines.
type Sampler interface {
	// ShouldSample returns true if the request should be logged
	ShouldSample(r *http.Request) bool
}

// AlwaysSampler logs every request
type AlwaysSampler struct{}

func (s *AlwaysSampler) ShouldSample(r *http.Request) bool {
	return true
}

// NeverSampler never logs requests (useful for testing)
type NeverSampler struct{}

func (s *NeverSampler) ShouldSample(r *http.Request) bool {
	return false
}

// RateSampler logs a percentage of requests
type RateSampler struct {
	rate float64
	rng  *rand.Rand
	mu   sync.Mutex
}

// NewRateSampler creates a sampler that logs a percentage of requests
// rate should be between 0.0 and 1.0 (e.g., 0.1 for 10%)
// The random number generator uses a cryptographically secure seed for unpredictability.
func NewRateSampler(rate float64) *RateSampler {
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	
	// Use crypto/rand for unpredictable seeding
	var seed int64
	if err := binary.Read(crypto_rand.Reader, binary.BigEndian, &seed); err != nil {
		// Fallback to time-based seed if crypto/rand fails
		seed = time.Now().UnixNano()
	}
	
	return &RateSampler{
		rate: rate,
		rng:  rand.New(rand.NewSource(seed)),
	}
}

func (s *RateSampler) ShouldSample(r *http.Request) bool {
	if s.rate >= 1.0 {
		return true
	}
	if s.rate <= 0.0 {
		return false
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64() < s.rate
}

// CounterSampler logs every Nth request
type CounterSampler struct {
	n       uint64
	counter uint64
}

// NewCounterSampler creates a sampler that logs every nth request
func NewCounterSampler(n uint64) *CounterSampler {
	if n == 0 {
		n = 1
	}
	return &CounterSampler{n: n}
}

func (s *CounterSampler) ShouldSample(r *http.Request) bool {
	count := atomic.AddUint64(&s.counter, 1)
	return count%s.n == 0
}

// AdaptiveSampler adjusts sampling rate based on request volume
type AdaptiveSampler struct {
	targetPerSecond float64
	window          time.Duration
	
	mu          sync.RWMutex
	requests    uint64
	windowStart time.Time
	currentRate float64
	rng         *rand.Rand // Thread-safe random source
}

// NewAdaptiveSampler creates a sampler that aims for a target logging rate
func NewAdaptiveSampler(targetPerSecond float64) *AdaptiveSampler {
	// Use crypto/rand for unpredictable seeding
	var seed int64
	if err := binary.Read(crypto_rand.Reader, binary.BigEndian, &seed); err != nil {
		// Fallback to time-based seed if crypto/rand fails
		seed = time.Now().UnixNano()
	}
	
	return &AdaptiveSampler{
		targetPerSecond: targetPerSecond,
		window:          time.Second,
		windowStart:     time.Now(),
		currentRate:     1.0,
		rng:             rand.New(rand.NewSource(seed)),
	}
}

func (s *AdaptiveSampler) ShouldSample(r *http.Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(s.windowStart)
	
	// Reset window if needed
	if elapsed >= s.window {
		// Calculate actual rate from last window
		actualRate := float64(s.requests) / elapsed.Seconds()
		
		// Adjust sampling rate
		if actualRate > 0 {
			s.currentRate = s.targetPerSecond / actualRate
			if s.currentRate > 1.0 {
				s.currentRate = 1.0
			} else if s.currentRate < 0.001 {
				s.currentRate = 0.001 // Minimum 0.1% sampling
			}
		}
		
		// Reset for new window
		s.requests = 0
		s.windowStart = now
	}
	
	s.requests++
	
	// Use current rate to decide sampling
	if s.currentRate >= 1.0 {
		return true
	}
	
	return s.rng.Float64() < s.currentRate
}

// PathSampler samples based on path patterns
type PathSampler struct {
	rules         []PathSamplingRule
	caseSensitive bool
	allowEscapes  bool
	defaultSample bool // What to do when no rules match
	rng           *rand.Rand
	mu            sync.Mutex // Protects rng
}

// PathSamplingRule defines sampling behavior for specific paths
type PathSamplingRule struct {
	Pattern       string  // Glob pattern for path matching
	Rate          float64 // Sampling rate for matching paths
	MatchSegments bool    // If true, * won't match across / boundaries
}

// NewPathSampler creates a sampler with path-specific rules (defaults to allowing non-matching paths)
func NewPathSampler(rules []PathSamplingRule) *PathSampler {
	// Use crypto/rand for unpredictable seeding
	var seed int64
	if err := binary.Read(crypto_rand.Reader, binary.BigEndian, &seed); err != nil {
		// Fallback to time-based seed if crypto/rand fails
		seed = time.Now().UnixNano()
	}
	
	return &PathSampler{
		rules:         rules,
		caseSensitive: true,
		allowEscapes:  false,
		defaultSample: true, // Default to logging (most common use case)
		rng:           rand.New(rand.NewSource(seed)),
	}
}

// NewExplicitPathSampler creates a sampler that only logs paths matching explicit rules
func NewExplicitPathSampler(rules []PathSamplingRule) *PathSampler {
	// Use crypto/rand for unpredictable seeding
	var seed int64
	if err := binary.Read(crypto_rand.Reader, binary.BigEndian, &seed); err != nil {
		// Fallback to time-based seed if crypto/rand fails
		seed = time.Now().UnixNano()
	}
	
	return &PathSampler{
		rules:         rules,
		caseSensitive: true,
		allowEscapes:  false,
		defaultSample: false, // Only log paths that match rules
		rng:           rand.New(rand.NewSource(seed)),
	}
}

func (s *PathSampler) ShouldSample(r *http.Request) bool {
	path := r.URL.Path
	if !s.caseSensitive {
		path = strings.ToLower(path)
	}
	
	for _, rule := range s.rules {
		pattern := rule.Pattern
		if !s.caseSensitive {
			pattern = strings.ToLower(pattern)
		}
		
		matched := false
		if rule.MatchSegments {
			matched = matchPathWithSegments(pattern, path)
		} else if s.allowEscapes {
			matched = matchPathEscaped(pattern, path)
		} else {
			matched = matchPath(pattern, path)
		}
		
		if matched {
			if rule.Rate >= 1.0 {
				return true
			}
			if rule.Rate <= 0.0 {
				return false
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			return s.rng.Float64() < rule.Rate
		}
	}
	
	// Use configured default behavior when no rules match
	return s.defaultSample
}

// matchPath performs complete glob-style matching
func matchPath(pattern, path string) bool {
	return matchPathInternal(pattern, path, 0, 0, -1, -1)
}

func matchPathInternal(pattern, path string, pIdx, sIdx, lastStarIdx, lastMatchIdx int) bool {
	// Match characters until we hit a star or end
	for sIdx < len(path) {
		if pIdx < len(pattern) {
			switch pattern[pIdx] {
			case '?':
				// '?' matches any single character
				pIdx++
				sIdx++
			case '*':
				// Remember star position for backtracking
				lastStarIdx = pIdx
				lastMatchIdx = sIdx
				pIdx++
				// Try to match with 0 characters first
			default:
				// Regular character must match exactly
				if pattern[pIdx] == path[sIdx] {
					pIdx++
					sIdx++
				} else if lastStarIdx != -1 {
					// Backtrack to last star
					pIdx = lastStarIdx + 1
					lastMatchIdx++
					sIdx = lastMatchIdx
				} else {
					return false
				}
			}
		} else if lastStarIdx != -1 {
			// Pattern exhausted but we have a star to backtrack to
			pIdx = lastStarIdx + 1
			lastMatchIdx++
			sIdx = lastMatchIdx
		} else {
			// Pattern exhausted and no star to help
			return false
		}
	}
	
	// Path exhausted, check remaining pattern
	// Only '*' characters are allowed at the end
	for pIdx < len(pattern) && pattern[pIdx] == '*' {
		pIdx++
	}
	
	return pIdx == len(pattern)
}

// matchPathEscaped handles escaped special characters
func matchPathEscaped(pattern, path string) bool {
	// Pre-process pattern to handle escapes and track if we have any unescaped wildcards
	var processedPattern strings.Builder
	hasWildcards := false
	escaped := false
	
	for i := 0; i < len(pattern); i++ {
		if escaped {
			// This character is escaped, write it literally
			processedPattern.WriteByte(pattern[i])
			escaped = false
		} else if pattern[i] == '\\' && i+1 < len(pattern) {
			next := pattern[i+1]
			if next == '*' || next == '?' || next == '\\' {
				// Escape sequence - skip the backslash, next char will be literal
				escaped = true
			} else {
				// Not an escape sequence, keep the backslash
				processedPattern.WriteByte(pattern[i])
			}
		} else {
			// Regular character
			if pattern[i] == '*' || pattern[i] == '?' {
				hasWildcards = true
			}
			processedPattern.WriteByte(pattern[i])
		}
	}
	
	processedPatternStr := processedPattern.String()
	
	// If we have wildcards, use glob matching; otherwise use literal comparison
	if hasWildcards {
		return matchPath(processedPatternStr, path)
	} else {
		return processedPatternStr == path
	}
}

// matchPathWithSegments ensures * doesn't cross path boundaries
func matchPathWithSegments(pattern, path string) bool {
	// Handle ** for recursive matching
	if strings.Contains(pattern, "**") {
		return matchDoubleStarPath(pattern, path)
	}
	
	// For single *, ensure it doesn't match /
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")
	
	if len(patternParts) != len(pathParts) {
		return false
	}
	
	for i := 0; i < len(patternParts); i++ {
		if !matchPath(patternParts[i], pathParts[i]) {
			return false
		}
	}
	
	return true
}

// matchDoubleStarPath handles ** for recursive directory matching
func matchDoubleStarPath(pattern, path string) bool {
	// Split on ** to handle recursive matching
	parts := strings.Split(pattern, "**")
	if len(parts) == 1 {
		// No ** in pattern
		return matchPathWithSegments(pattern, path)
	}
	
	// Check prefix
	prefix := strings.TrimSuffix(parts[0], "/")
	if prefix != "" && !strings.HasPrefix(path, prefix) {
		return false
	}
	
	// Check suffix (if any)
	if len(parts) == 2 && parts[1] != "" {
		suffix := strings.TrimPrefix(parts[1], "/")
		remaining := strings.TrimPrefix(path, prefix)
		remaining = strings.TrimPrefix(remaining, "/")
		
		// Try to match suffix at any position
		segments := strings.Split(remaining, "/")
		for i := 0; i <= len(segments); i++ {
			testPath := strings.Join(segments[i:], "/")
			if matchPathWithSegments(suffix, testPath) {
				return true
			}
		}
		return false
	}
	
	return true
}

// PathSamplerBuilder for fluent configuration
type PathSamplerBuilder struct {
	rules         []PathSamplingRule
	caseSensitive bool
	allowEscapes  bool
	defaultSample bool
}

// NewPathSamplerBuilder creates a new builder for PathSampler
func NewPathSamplerBuilder() *PathSamplerBuilder {
	return &PathSamplerBuilder{
		caseSensitive: true,
		allowEscapes:  false,
		defaultSample: true, // Default to allowing non-matching paths
	}
}

// CaseInsensitive enables case-insensitive pattern matching
func (b *PathSamplerBuilder) CaseInsensitive() *PathSamplerBuilder {
	b.caseSensitive = false
	return b
}

// WithEscapes enables escaped special characters in patterns
func (b *PathSamplerBuilder) WithEscapes() *PathSamplerBuilder {
	b.allowEscapes = true
	return b
}

// DefaultAllow configures the sampler to log all requests that don't match any rules (default)
func (b *PathSamplerBuilder) DefaultAllow() *PathSamplerBuilder {
	b.defaultSample = true
	return b
}

// DefaultDeny configures the sampler to only log requests that match explicit rules
func (b *PathSamplerBuilder) DefaultDeny() *PathSamplerBuilder {
	b.defaultSample = false
	return b
}

// Always logs all requests matching the pattern
func (b *PathSamplerBuilder) Always(pattern string) *PathSamplerBuilder {
	b.rules = append(b.rules, PathSamplingRule{Pattern: pattern, Rate: 1.0})
	return b
}

// Never logs requests matching the pattern
func (b *PathSamplerBuilder) Never(pattern string) *PathSamplerBuilder {
	b.rules = append(b.rules, PathSamplingRule{Pattern: pattern, Rate: 0.0})
	return b
}

// Sometimes logs a percentage of requests matching the pattern
func (b *PathSamplerBuilder) Sometimes(pattern string, rate float64) *PathSamplerBuilder {
	b.rules = append(b.rules, PathSamplingRule{Pattern: pattern, Rate: rate})
	return b
}

// WithSegments ensures * doesn't match across path segments
func (b *PathSamplerBuilder) WithSegments(pattern string, rate float64) *PathSamplerBuilder {
	b.rules = append(b.rules, PathSamplingRule{
		Pattern:       pattern,
		Rate:          rate,
		MatchSegments: true,
	})
	return b
}

// Build creates the configured PathSampler
func (b *PathSamplerBuilder) Build() *PathSampler {
	// Use crypto/rand for unpredictable seeding
	var seed int64
	if err := binary.Read(crypto_rand.Reader, binary.BigEndian, &seed); err != nil {
		// Fallback to time-based seed if crypto/rand fails
		seed = time.Now().UnixNano()
	}
	
	return &PathSampler{
		rules:         b.rules,
		caseSensitive: b.caseSensitive,
		allowEscapes:  b.allowEscapes,
		defaultSample: b.defaultSample,
		rng:           rand.New(rand.NewSource(seed)),
	}
}

// DynamicPathSampler allows runtime changes to sampling rules
type DynamicPathSampler struct {
	mu            sync.RWMutex
	rules         []PathSamplingRule
	caseSensitive bool
	allowEscapes  bool
	defaultSample bool
	onChange      func(oldRules, newRules []PathSamplingRule)
	rng           *rand.Rand // Random source
	rngMu         sync.Mutex // Protects rng
}

// NewDynamicPathSampler creates a new dynamic path sampler
func NewDynamicPathSampler(rules []PathSamplingRule) *DynamicPathSampler {
	// Use crypto/rand for unpredictable seeding
	var seed int64
	if err := binary.Read(crypto_rand.Reader, binary.BigEndian, &seed); err != nil {
		// Fallback to time-based seed if crypto/rand fails
		seed = time.Now().UnixNano()
	}
	
	return &DynamicPathSampler{
		rules:         rules,
		caseSensitive: true,
		allowEscapes:  false,
		defaultSample: true, // Default to allowing non-matching paths
		rng:           rand.New(rand.NewSource(seed)),
	}
}

// ShouldSample checks if a request should be sampled
func (s *DynamicPathSampler) ShouldSample(r *http.Request) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	path := r.URL.Path
	if !s.caseSensitive {
		path = strings.ToLower(path)
	}
	
	for _, rule := range s.rules {
		pattern := rule.Pattern
		if !s.caseSensitive {
			pattern = strings.ToLower(pattern)
		}
		
		matched := false
		if rule.MatchSegments {
			matched = matchPathWithSegments(pattern, path)
		} else if s.allowEscapes {
			matched = matchPathEscaped(pattern, path)
		} else {
			matched = matchPath(pattern, path)
		}
		
		if matched {
			if rule.Rate >= 1.0 {
				return true
			}
			if rule.Rate <= 0.0 {
				return false
			}
			// Use local RNG for thread-safe random numbers
			s.rngMu.Lock()
			result := s.rng.Float64() < rule.Rate
			s.rngMu.Unlock()
			return result
		}
	}
	
	return s.defaultSample
}

// UpdateRules updates the sampling rules at runtime
func (s *DynamicPathSampler) UpdateRules(rules []PathSamplingRule) {
	s.mu.Lock()
	oldRules := s.rules
	s.rules = rules
	onChange := s.onChange
	s.mu.Unlock()
	
	// Call change handler outside of lock
	if onChange != nil {
		onChange(oldRules, rules)
	}
}

// GetRules returns a copy of the current rules
func (s *DynamicPathSampler) GetRules() []PathSamplingRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	rules := make([]PathSamplingRule, len(s.rules))
	copy(rules, s.rules)
	return rules
}

// SetCaseSensitive updates case sensitivity
func (s *DynamicPathSampler) SetCaseSensitive(sensitive bool) {
	s.mu.Lock()
	s.caseSensitive = sensitive
	s.mu.Unlock()
}

// SetAllowEscapes updates escape handling
func (s *DynamicPathSampler) SetAllowEscapes(allow bool) {
	s.mu.Lock()
	s.allowEscapes = allow
	s.mu.Unlock()
}

// SetOnChange sets a callback for rule changes
func (s *DynamicPathSampler) SetOnChange(fn func(oldRules, newRules []PathSamplingRule)) {
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

// AddRule adds a new rule at runtime
func (s *DynamicPathSampler) AddRule(rule PathSamplingRule) {
	s.mu.Lock()
	oldRules := make([]PathSamplingRule, len(s.rules))
	copy(oldRules, s.rules)
	s.rules = append(s.rules, rule)
	newRules := make([]PathSamplingRule, len(s.rules))
	copy(newRules, s.rules)
	onChange := s.onChange
	s.mu.Unlock()
	
	if onChange != nil {
		onChange(oldRules, newRules)
	}
}

// RemoveRule removes a rule by pattern
func (s *DynamicPathSampler) RemoveRule(pattern string) bool {
	s.mu.Lock()
	oldRules := make([]PathSamplingRule, len(s.rules))
	copy(oldRules, s.rules)
	
	found := false
	newRules := make([]PathSamplingRule, 0, len(s.rules))
	for _, rule := range s.rules {
		if rule.Pattern != pattern {
			newRules = append(newRules, rule)
		} else {
			found = true
		}
	}
	
	if found {
		s.rules = newRules
		onChange := s.onChange
		s.mu.Unlock()
		
		if onChange != nil {
			onChange(oldRules, newRules)
		}
		return true
	}
	
	s.mu.Unlock()
	return false
}

// UpdateRuleRate updates the sampling rate for a specific pattern
func (s *DynamicPathSampler) UpdateRuleRate(pattern string, newRate float64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for i := range s.rules {
		if s.rules[i].Pattern == pattern {
			s.rules[i].Rate = newRate
			return true
		}
	}
	return false
}

// CompositeSampler combines multiple samplers with AND/OR logic
type CompositeSampler struct {
	samplers []Sampler
	mode     CompositeMode
}

type CompositeMode int

const (
	CompositeAND CompositeMode = iota // All samplers must agree
	CompositeOR                        // Any sampler can approve
)

// NewCompositeSampler creates a sampler that combines multiple samplers
func NewCompositeSampler(mode CompositeMode, samplers ...Sampler) *CompositeSampler {
	return &CompositeSampler{
		samplers: samplers,
		mode:     mode,
	}
}

func (s *CompositeSampler) ShouldSample(r *http.Request) bool {
	if len(s.samplers) == 0 {
		return true
	}
	
	switch s.mode {
	case CompositeAND:
		for _, sampler := range s.samplers {
			if !sampler.ShouldSample(r) {
				return false
			}
		}
		return true
	case CompositeOR:
		for _, sampler := range s.samplers {
			if sampler.ShouldSample(r) {
				return true
			}
		}
		return false
	default:
		return true
	}
}