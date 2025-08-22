package middleware

import (
	"math"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestAlwaysSampler(t *testing.T) {
	sampler := &AlwaysSampler{}
	
	req := httptest.NewRequest("GET", "/test", nil)
	
	for i := 0; i < 100; i++ {
		if !sampler.ShouldSample(req) {
			t.Errorf("AlwaysSampler should always return true")
		}
	}
}

func TestNeverSampler(t *testing.T) {
	sampler := &NeverSampler{}
	
	req := httptest.NewRequest("GET", "/test", nil)
	
	for i := 0; i < 100; i++ {
		if sampler.ShouldSample(req) {
			t.Errorf("NeverSampler should always return false")
		}
	}
}

func TestRateSampler(t *testing.T) {
	t.Run("rate boundaries", func(t *testing.T) {
		tests := []struct {
			rate     float64
			expected bool
		}{
			{0.0, false},
			{-0.1, false},
			{1.0, true},
			{1.1, true},
		}
		
		for _, tt := range tests {
			sampler := NewRateSampler(tt.rate)
			req := httptest.NewRequest("GET", "/test", nil)
			
			result := sampler.ShouldSample(req)
			if result != tt.expected {
				t.Errorf("RateSampler(rate=%f).ShouldSample() = %v, want %v", tt.rate, result, tt.expected)
			}
		}
	})
	
	t.Run("rate approximation", func(t *testing.T) {
		rate := 0.3
		sampler := NewRateSampler(rate)
		req := httptest.NewRequest("GET", "/test", nil)
		
		const trials = 10000
		hits := 0
		
		for i := 0; i < trials; i++ {
			if sampler.ShouldSample(req) {
				hits++
			}
		}
		
		actualRate := float64(hits) / float64(trials)
		tolerance := 0.05 // 5% tolerance
		
		if math.Abs(actualRate-rate) > tolerance {
			t.Errorf("RateSampler(rate=%f): actual rate %f is outside tolerance %f", rate, actualRate, tolerance)
		}
	})
	
	t.Run("thread safety", func(t *testing.T) {
		sampler := NewRateSampler(0.5)
		req := httptest.NewRequest("GET", "/test", nil)
		
		const numGoroutines = 100
		const numSamples = 100
		
		var wg sync.WaitGroup
		var totalHits int64
		var mutex sync.Mutex
		
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				hits := 0
				for j := 0; j < numSamples; j++ {
					if sampler.ShouldSample(req) {
						hits++
					}
				}
				mutex.Lock()
				totalHits += int64(hits)
				mutex.Unlock()
			}()
		}
		
		wg.Wait()
		
		totalSamples := numGoroutines * numSamples
		actualRate := float64(totalHits) / float64(totalSamples)
		
		// More lenient tolerance for concurrent test
		tolerance := 0.1
		if math.Abs(actualRate-0.5) > tolerance {
			t.Errorf("Concurrent RateSampler: actual rate %f is outside tolerance %f", actualRate, tolerance)
		}
	})
}

func TestCounterSampler(t *testing.T) {
	t.Run("every nth request", func(t *testing.T) {
		sampler := NewCounterSampler(3)
		req := httptest.NewRequest("GET", "/test", nil)
		
		expected := []bool{false, false, true, false, false, true, false, false, true}
		
		for i, want := range expected {
			got := sampler.ShouldSample(req)
			if got != want {
				t.Errorf("CounterSampler request %d: got %v, want %v", i+1, got, want)
			}
		}
	})
	
	t.Run("zero n defaults to 1", func(t *testing.T) {
		sampler := NewCounterSampler(0)
		req := httptest.NewRequest("GET", "/test", nil)
		
		// Every request should be sampled
		for i := 0; i < 5; i++ {
			if !sampler.ShouldSample(req) {
				t.Errorf("CounterSampler(0) request %d should be sampled", i+1)
			}
		}
	})
	
	t.Run("thread safety", func(t *testing.T) {
		sampler := NewCounterSampler(10)
		req := httptest.NewRequest("GET", "/test", nil)
		
		const numGoroutines = 100
		const numRequests = 100
		
		var wg sync.WaitGroup
		var totalSampled int64
		var mutex sync.Mutex
		
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				sampled := 0
				for j := 0; j < numRequests; j++ {
					if sampler.ShouldSample(req) {
						sampled++
					}
				}
				mutex.Lock()
				totalSampled += int64(sampled)
				mutex.Unlock()
			}()
		}
		
		wg.Wait()
		
		totalRequests := numGoroutines * numRequests
		expectedSampled := totalRequests / 10
		
		// Allow some variance due to concurrency
		tolerance := expectedSampled / 10
		if math.Abs(float64(totalSampled-int64(expectedSampled))) > float64(tolerance) {
			t.Errorf("CounterSampler concurrent: got %d sampled, expected ~%d", totalSampled, expectedSampled)
		}
	})
}

func TestAdaptiveSampler(t *testing.T) {
	t.Run("initial sampling", func(t *testing.T) {
		sampler := NewAdaptiveSampler(100) // 100 requests per second
		req := httptest.NewRequest("GET", "/test", nil)
		
		// Initial rate should be 1.0 (sample everything)
		for i := 0; i < 10; i++ {
			if !sampler.ShouldSample(req) {
				t.Errorf("AdaptiveSampler should sample everything initially")
			}
		}
	})
	
	t.Run("rate adaptation", func(t *testing.T) {
		sampler := NewAdaptiveSampler(10) // 10 requests per second
		req := httptest.NewRequest("GET", "/test", nil)
		
		// Simulate high load (more than 10 req/sec)
		const requestsPerBatch = 50
		const batches = 3
		
		for batch := 0; batch < batches; batch++ {
			for i := 0; i < requestsPerBatch; i++ {
				sampler.ShouldSample(req)
			}
			// Wait for window to reset
			time.Sleep(1100 * time.Millisecond)
		}
		
		// After adaptation, sampling rate should be lower
		sampledCount := 0
		const testRequests = 100
		for i := 0; i < testRequests; i++ {
			if sampler.ShouldSample(req) {
				sampledCount++
			}
		}
		
		// Should sample much less than 100% due to adaptation
		if sampledCount > testRequests/2 {
			t.Errorf("AdaptiveSampler should have reduced sampling rate, got %d/%d", sampledCount, testRequests)
		}
	})
	
	t.Run("minimum rate enforcement", func(t *testing.T) {
		sampler := NewAdaptiveSampler(0.1) // Very low target
		req := httptest.NewRequest("GET", "/test", nil)
		
		// Simulate very high load
		for i := 0; i < 1000; i++ {
			sampler.ShouldSample(req)
		}
		
		// Wait for adaptation
		time.Sleep(1100 * time.Millisecond)
		
		// Should still sample some requests (minimum 0.1%)
		sampledCount := 0
		const testRequests = 10000
		for i := 0; i < testRequests; i++ {
			if sampler.ShouldSample(req) {
				sampledCount++
			}
		}
		
		if sampledCount == 0 {
			t.Errorf("AdaptiveSampler should maintain minimum sampling rate")
		}
	})
}

func TestPathSampler(t *testing.T) {
	t.Run("exact path matching", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/api/users", Rate: 1.0},
			{Pattern: "/health", Rate: 0.0},
		}
		sampler := NewPathSampler(rules)
		
		tests := []struct {
			path     string
			expected bool
		}{
			{"/api/users", true},
			{"/health", false},
			{"/other", true}, // Default to true
		}
		
		for _, tt := range tests {
			req := httptest.NewRequest("GET", tt.path, nil)
			result := sampler.ShouldSample(req)
			if result != tt.expected {
				t.Errorf("PathSampler path %s: got %v, want %v", tt.path, result, tt.expected)
			}
		}
	})
	
	t.Run("wildcard matching", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/api/*", Rate: 1.0},
			{Pattern: "/admin/*", Rate: 0.0},
		}
		sampler := NewPathSampler(rules)
		
		tests := []struct {
			path     string
			expected bool
		}{
			{"/api/users", true},
			{"/api/posts/123", true},
			{"/admin/dashboard", false},
			{"/admin/users/delete", false},
			{"/public/assets", true}, // Default
		}
		
		for _, tt := range tests {
			req := httptest.NewRequest("GET", tt.path, nil)
			result := sampler.ShouldSample(req)
			if result != tt.expected {
				t.Errorf("PathSampler path %s: got %v, want %v", tt.path, result, tt.expected)
			}
		}
	})
	
	t.Run("question mark matching", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/user?", Rate: 1.0},
		}
		sampler := NewExplicitPathSampler(rules)
		
		tests := []struct {
			path     string
			expected bool
		}{
			{"/users", true},
			{"/userx", true},
			{"/user", false}, // ? requires exactly one character
			{"/userss", false},
		}
		
		for _, tt := range tests {
			req := httptest.NewRequest("GET", tt.path, nil)
			result := sampler.ShouldSample(req)
			if result != tt.expected {
				t.Errorf("PathSampler path %s: got %v, want %v", tt.path, result, tt.expected)
			}
		}
	})
	
	t.Run("segment matching", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/api/*/users", Rate: 1.0, MatchSegments: true},
		}
		sampler := NewPathSampler(rules)
		
		tests := []struct {
			path     string
			expected bool
		}{
			{"/api/v1/users", true},
			{"/api/v2/users", true},
			{"/api/v1/admin/users", true}, // Default (no match)
		}
		
		for _, tt := range tests {
			req := httptest.NewRequest("GET", tt.path, nil)
			result := sampler.ShouldSample(req)
			if result != tt.expected {
				t.Errorf("PathSampler segment path %s: got %v, want %v", tt.path, result, tt.expected)
			}
		}
	})
	
	t.Run("double star matching", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/api/**", Rate: 0.0, MatchSegments: true},
		}
		sampler := NewPathSampler(rules)
		
		tests := []struct {
			path     string
			expected bool
		}{
			{"/api/v1/users", false},
			{"/api/v1/admin/users", false},
			{"/api/", false},
			{"/public/assets", true}, // Default
		}
		
		for _, tt := range tests {
			req := httptest.NewRequest("GET", tt.path, nil)
			result := sampler.ShouldSample(req)
			if result != tt.expected {
				t.Errorf("PathSampler double star path %s: got %v, want %v", tt.path, result, tt.expected)
			}
		}
	})
	
	t.Run("case sensitivity", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/API/*", Rate: 0.0},
		}
		
		// Case sensitive sampler
		sampler := &PathSampler{
			rules:         rules,
			caseSensitive: true,
			defaultSample: true, // Default to logging non-matching paths
		}
		
		req1 := httptest.NewRequest("GET", "/API/users", nil)
		req2 := httptest.NewRequest("GET", "/api/users", nil)
		
		if sampler.ShouldSample(req1) != false {
			t.Errorf("Case sensitive: /API/users should not be sampled")
		}
		if sampler.ShouldSample(req2) != true {
			t.Errorf("Case sensitive: /api/users should be sampled (default)")
		}
		
		// Case insensitive sampler
		sampler.caseSensitive = false
		
		if sampler.ShouldSample(req1) != false {
			t.Errorf("Case insensitive: /API/users should not be sampled")
		}
		if sampler.ShouldSample(req2) != false {
			t.Errorf("Case insensitive: /api/users should not be sampled")
		}
	})
	
	t.Run("escape handling", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/api/\\*", Rate: 0.0},
		}
		
		sampler := &PathSampler{
			rules:         rules,
			allowEscapes:  true,
			defaultSample: true, // Default to logging non-matching paths
		}
		
		req1 := httptest.NewRequest("GET", "/api/*", nil)
		req2 := httptest.NewRequest("GET", "/api/users", nil)
		
		if sampler.ShouldSample(req1) != false {
			t.Errorf("Escaped pattern: /api/* should not be sampled")
		}
		if sampler.ShouldSample(req2) != true {
			t.Errorf("Escaped pattern: /api/users should be sampled (default)")
		}
	})
	
	t.Run("rate sampling", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/api/*", Rate: 0.5},
		}
		sampler := NewPathSampler(rules)
		
		const trials = 10000
		hits := 0
		
		for i := 0; i < trials; i++ {
			req := httptest.NewRequest("GET", "/api/users", nil)
			if sampler.ShouldSample(req) {
				hits++
			}
		}
		
		actualRate := float64(hits) / float64(trials)
		expectedRate := 0.5
		tolerance := 0.05
		
		if math.Abs(actualRate-expectedRate) > tolerance {
			t.Errorf("PathSampler rate: got %f, want %f ± %f", actualRate, expectedRate, tolerance)
		}
	})
}

func TestPathSamplerBuilder(t *testing.T) {
	t.Run("builder pattern", func(t *testing.T) {
		sampler := NewPathSamplerBuilder().
			CaseInsensitive().
			WithEscapes().
			Always("/health").
			Never("/admin/*").
			Sometimes("/api/*", 0.5).
			WithSegments("/users/*/profile", 0.8).
			Build()
		
		tests := []struct {
			path        string
			shouldMatch bool
			rate        float64
		}{
			{"/health", true, 1.0},
			{"/HEALTH", true, 1.0}, // Case insensitive
			{"/admin/dashboard", false, 0.0},
			{"/api/users", true, 0.5},
			{"/users/123/profile", true, 0.8},
		}
		
		for _, tt := range tests {
			req := httptest.NewRequest("GET", tt.path, nil)
			
			if tt.rate == 1.0 {
				if !sampler.ShouldSample(req) {
					t.Errorf("Path %s should always be sampled", tt.path)
				}
			} else if tt.rate == 0.0 {
				if sampler.ShouldSample(req) {
					t.Errorf("Path %s should never be sampled", tt.path)
				}
			}
			// For probabilistic rates, we just check it doesn't panic
		}
	})
}

func TestDynamicPathSampler(t *testing.T) {
	t.Run("basic functionality", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/api/*", Rate: 1.0},
		}
		sampler := NewDynamicPathSampler(rules)
		
		req := httptest.NewRequest("GET", "/api/users", nil)
		if !sampler.ShouldSample(req) {
			t.Errorf("Initial rule should sample /api/users")
		}
	})
	
	t.Run("rule updates", func(t *testing.T) {
		sampler := NewDynamicPathSampler(nil)
		
		// Initially no rules, should default to sampling
		req := httptest.NewRequest("GET", "/api/users", nil)
		if !sampler.ShouldSample(req) {
			t.Errorf("Should sample by default when no rules")
		}
		
		// Update rules
		newRules := []PathSamplingRule{
			{Pattern: "/api/*", Rate: 0.0},
		}
		sampler.UpdateRules(newRules)
		
		if sampler.ShouldSample(req) {
			t.Errorf("Should not sample after rule update")
		}
	})
	
	t.Run("get rules", func(t *testing.T) {
		originalRules := []PathSamplingRule{
			{Pattern: "/api/*", Rate: 1.0},
			{Pattern: "/admin/*", Rate: 0.0},
		}
		sampler := NewDynamicPathSampler(originalRules)
		
		retrievedRules := sampler.GetRules()
		
		if len(retrievedRules) != len(originalRules) {
			t.Errorf("GetRules() returned %d rules, want %d", len(retrievedRules), len(originalRules))
		}
		
		// Should be a copy, not the same slice
		if &retrievedRules[0] == &originalRules[0] {
			t.Errorf("GetRules() should return a copy, not the original slice")
		}
	})
	
	t.Run("add rule", func(t *testing.T) {
		sampler := NewDynamicPathSampler(nil)
		
		newRule := PathSamplingRule{Pattern: "/api/*", Rate: 0.0}
		sampler.AddRule(newRule)
		
		rules := sampler.GetRules()
		if len(rules) != 1 {
			t.Errorf("Expected 1 rule after AddRule, got %d", len(rules))
		}
		
		if rules[0].Pattern != "/api/*" {
			t.Errorf("Added rule pattern mismatch")
		}
	})
	
	t.Run("remove rule", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/api/*", Rate: 1.0},
			{Pattern: "/admin/*", Rate: 0.0},
		}
		sampler := NewDynamicPathSampler(rules)
		
		// Remove existing rule
		if !sampler.RemoveRule("/api/*") {
			t.Errorf("RemoveRule should return true for existing rule")
		}
		
		remainingRules := sampler.GetRules()
		if len(remainingRules) != 1 {
			t.Errorf("Expected 1 rule after removal, got %d", len(remainingRules))
		}
		
		// Remove non-existing rule
		if sampler.RemoveRule("/nonexistent") {
			t.Errorf("RemoveRule should return false for non-existing rule")
		}
	})
	
	t.Run("update rule rate", func(t *testing.T) {
		rules := []PathSamplingRule{
			{Pattern: "/api/*", Rate: 1.0},
		}
		sampler := NewDynamicPathSampler(rules)
		
		// Update existing rule
		if !sampler.UpdateRuleRate("/api/*", 0.5) {
			t.Errorf("UpdateRuleRate should return true for existing rule")
		}
		
		updatedRules := sampler.GetRules()
		if updatedRules[0].Rate != 0.5 {
			t.Errorf("Rule rate should be updated to 0.5, got %f", updatedRules[0].Rate)
		}
		
		// Update non-existing rule
		if sampler.UpdateRuleRate("/nonexistent", 0.8) {
			t.Errorf("UpdateRuleRate should return false for non-existing rule")
		}
	})
	
	t.Run("configuration changes", func(t *testing.T) {
		sampler := NewDynamicPathSampler(nil)
		
		// Test case sensitivity
		sampler.SetCaseSensitive(false)
		// Test escape handling
		sampler.SetAllowEscapes(true)
		
		// These should not panic and changes should be applied
		// We can't easily test the effects without more complex setup
	})
	
	t.Run("change callback", func(t *testing.T) {
		sampler := NewDynamicPathSampler(nil)
		
		var callbackCalled bool
		var oldRulesCount, newRulesCount int
		
		sampler.SetOnChange(func(oldRules, newRules []PathSamplingRule) {
			callbackCalled = true
			oldRulesCount = len(oldRules)
			newRulesCount = len(newRules)
		})
		
		// Add a rule to trigger callback
		sampler.AddRule(PathSamplingRule{Pattern: "/test", Rate: 1.0})
		
		if !callbackCalled {
			t.Errorf("OnChange callback should have been called")
		}
		if oldRulesCount != 0 || newRulesCount != 1 {
			t.Errorf("Callback received wrong rule counts: old=%d, new=%d", oldRulesCount, newRulesCount)
		}
	})
	
	t.Run("concurrent access", func(t *testing.T) {
		sampler := NewDynamicPathSampler([]PathSamplingRule{
			{Pattern: "/api/*", Rate: 0.5},
		})
		
		const numGoroutines = 50
		var wg sync.WaitGroup
		wg.Add(numGoroutines)
		
		// Concurrent reads and writes
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				
				req := httptest.NewRequest("GET", "/api/test", nil)
				
				for j := 0; j < 100; j++ {
					// Mix of reads and writes
					if j%10 == 0 {
						sampler.UpdateRuleRate("/api/*", 0.3)
					} else if j%20 == 0 {
						sampler.GetRules()
					} else {
						sampler.ShouldSample(req)
					}
				}
			}(i)
		}
		
		wg.Wait()
		// Test passes if no race conditions occur
	})
}

func TestCompositeSampler(t *testing.T) {
	t.Run("AND mode", func(t *testing.T) {
		sampler1 := &AlwaysSampler{}
		sampler2 := &AlwaysSampler{}
		sampler3 := &NeverSampler{}
		
		// All samplers agree (true)
		composite1 := NewCompositeSampler(CompositeAND, sampler1, sampler2)
		req := httptest.NewRequest("GET", "/test", nil)
		if !composite1.ShouldSample(req) {
			t.Errorf("CompositeAND with all true should return true")
		}
		
		// One sampler disagrees
		composite2 := NewCompositeSampler(CompositeAND, sampler1, sampler2, sampler3)
		if composite2.ShouldSample(req) {
			t.Errorf("CompositeAND with one false should return false")
		}
	})
	
	t.Run("OR mode", func(t *testing.T) {
		sampler1 := &NeverSampler{}
		sampler2 := &NeverSampler{}
		sampler3 := &AlwaysSampler{}
		
		// All samplers disagree (false)
		composite1 := NewCompositeSampler(CompositeOR, sampler1, sampler2)
		req := httptest.NewRequest("GET", "/test", nil)
		if composite1.ShouldSample(req) {
			t.Errorf("CompositeOR with all false should return false")
		}
		
		// One sampler agrees
		composite2 := NewCompositeSampler(CompositeOR, sampler1, sampler2, sampler3)
		if !composite2.ShouldSample(req) {
			t.Errorf("CompositeOR with one true should return true")
		}
	})
	
	t.Run("empty sampler list", func(t *testing.T) {
		composite := NewCompositeSampler(CompositeAND)
		req := httptest.NewRequest("GET", "/test", nil)
		
		// Should default to true
		if !composite.ShouldSample(req) {
			t.Errorf("Composite sampler with no samplers should return true")
		}
	})
	
	t.Run("complex composition", func(t *testing.T) {
		rateSampler := NewRateSampler(0.5)
		pathSampler := NewPathSampler([]PathSamplingRule{
			{Pattern: "/api/*", Rate: 1.0},
			{Pattern: "/health", Rate: 0.0},
		})
		
		// AND: both must agree
		composite := NewCompositeSampler(CompositeAND, rateSampler, pathSampler)
		
		// Test health endpoint (path sampler says no)
		req1 := httptest.NewRequest("GET", "/health", nil)
		if composite.ShouldSample(req1) {
			t.Errorf("Health endpoint should not be sampled (path sampler overrides)")
		}
		
		// Test API endpoint (depends on rate sampler)
		req2 := httptest.NewRequest("GET", "/api/users", nil)
		
		// Run multiple times to check rate sampling
		samples := 0
		trials := 1000
		for i := 0; i < trials; i++ {
			if composite.ShouldSample(req2) {
				samples++
			}
		}
		
		// Should sample roughly 50% of API requests
		rate := float64(samples) / float64(trials)
		if rate < 0.3 || rate > 0.7 {
			t.Errorf("API endpoint sampling rate %f should be around 0.5", rate)
		}
	})
}

func TestGlobPatternMatching(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		// Basic patterns
		{"*", "/anything", true},
		{"*", "", true},
		{"test", "test", true},
		{"test", "testing", false},
		
		// Wildcard patterns
		{"/api/*", "/api/users", true},
		{"/api/*", "/api/", true},
		{"/api/*", "/api", false},
		{"/api/*", "/api/users/123", true},
		
		// Question mark patterns
		{"/user?", "/users", true},
		{"/user?", "/user1", true},
		{"/user?", "/user", false},
		{"/user?", "/userss", false},
		
		// Multiple wildcards
		{"*/test/*", "prefix/test/suffix", true},
		{"*/test/*", "a/test/b/c", true},
		{"*/test/*", "test/suffix", false},
		{"*/test/*", "prefix/test", false},
		
		// Complex patterns
		{"/api/v?/users/*", "/api/v1/users/123", true},
		{"/api/v?/users/*", "/api/v2/users/profile", true},
		{"/api/v?/users/*", "/api/v10/users/123", false},
		
		// Edge cases
		{"", "", true},
		{"", "anything", false},
		{"*", "", true},
		{"**", "anything/with/slashes", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.pattern+"->"+tt.path, func(t *testing.T) {
			result := matchPath(tt.pattern, tt.path)
			if result != tt.match {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tt.pattern, tt.path, result, tt.match)
			}
		})
	}
}

func TestGlobPatternMatchingWithSegments(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		// Segment-aware matching (* doesn't cross /)
		{"/api/*/users", "/api/v1/users", true},
		{"/api/*/users", "/api/v1/v2/users", false},
		
		// Double star patterns
		{"/api/**/users", "/api/v1/users", true},
		{"/api/**/users", "/api/v1/v2/users", true},
		{"/api/**/users", "/api/v1/v2/v3/users", true},
		
		// Complex segment patterns
		{"/*/*/*", "/a/b/c", true},
		{"/*/*/*", "/a/b/c/d", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.pattern+"->"+tt.path, func(t *testing.T) {
			result := matchPathWithSegments(tt.pattern, tt.path)
			if result != tt.match {
				t.Errorf("matchPathWithSegments(%q, %q) = %v, want %v", tt.pattern, tt.path, result, tt.match)
			}
		})
	}
}

func TestGlobPatternMatchingWithEscapes(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		// Escaped special characters
		{"\\*", "*", true},
		{"\\*", "anything", false},
		{"\\?", "?", true},
		{"\\?", "a", false},
		
		// Mixed escaped and unescaped
		{"/api/\\*/users", "/api/*/users", true},
		{"/api/\\*/users", "/api/v1/users", false},
		
		// Escaped backslash
		{"\\\\", "\\", true},
		{"\\\\*", "\\anything", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.pattern+"->"+tt.path, func(t *testing.T) {
			result := matchPathEscaped(tt.pattern, tt.path)
			if result != tt.match {
				t.Errorf("matchPathEscaped(%q, %q) = %v, want %v", tt.pattern, tt.path, result, tt.match)
			}
		})
	}
}

func BenchmarkSamplers(b *testing.B) {
	req := httptest.NewRequest("GET", "/api/users", nil)
	
	b.Run("AlwaysSampler", func(b *testing.B) {
		sampler := &AlwaysSampler{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sampler.ShouldSample(req)
		}
	})
	
	b.Run("NeverSampler", func(b *testing.B) {
		sampler := &NeverSampler{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sampler.ShouldSample(req)
		}
	})
	
	b.Run("RateSampler", func(b *testing.B) {
		sampler := NewRateSampler(0.5)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sampler.ShouldSample(req)
		}
	})
	
	b.Run("CounterSampler", func(b *testing.B) {
		sampler := NewCounterSampler(10)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sampler.ShouldSample(req)
		}
	})
	
	b.Run("AdaptiveSampler", func(b *testing.B) {
		sampler := NewAdaptiveSampler(1000)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sampler.ShouldSample(req)
		}
	})
	
	b.Run("PathSampler", func(b *testing.B) {
		rules := []PathSamplingRule{
			{Pattern: "/api/*", Rate: 1.0},
			{Pattern: "/admin/*", Rate: 0.0},
			{Pattern: "/health", Rate: 0.0},
		}
		sampler := NewPathSampler(rules)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sampler.ShouldSample(req)
		}
	})
	
	b.Run("CompositeSampler", func(b *testing.B) {
		rateSampler := NewRateSampler(0.5)
		pathSampler := NewPathSampler([]PathSamplingRule{
			{Pattern: "/api/*", Rate: 1.0},
		})
		composite := NewCompositeSampler(CompositeAND, rateSampler, pathSampler)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			composite.ShouldSample(req)
		}
	})
}

func BenchmarkGlobMatching(b *testing.B) {
	pattern := "/api/*/users/*"
	path := "/api/v1/users/123"
	
	b.Run("matchPath", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			matchPath(pattern, path)
		}
	})
	
	b.Run("matchPathWithSegments", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			matchPathWithSegments(pattern, path)
		}
	})
	
	b.Run("matchPathEscaped", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			matchPathEscaped(pattern, path)
		}
	})
}

func TestPathSamplerDefaultBehavior(t *testing.T) {
	t.Run("default allow", func(t *testing.T) {
		sampler := NewPathSampler([]PathSamplingRule{
			{Pattern: "/api/*", Rate: 1.0},
		})
		
		// Matching path
		req := httptest.NewRequest("GET", "/api/users", nil)
		if !sampler.ShouldSample(req) {
			t.Error("PathSampler should sample matching path")
		}
		
		// Non-matching path - should default to true
		req = httptest.NewRequest("GET", "/health", nil)
		if !sampler.ShouldSample(req) {
			t.Error("PathSampler should sample non-matching path (default allow)")
		}
	})
	
	t.Run("explicit only", func(t *testing.T) {
		sampler := NewExplicitPathSampler([]PathSamplingRule{
			{Pattern: "/api/*", Rate: 1.0},
		})
		
		// Matching path
		req := httptest.NewRequest("GET", "/api/users", nil)
		if !sampler.ShouldSample(req) {
			t.Error("ExplicitPathSampler should sample matching path")
		}
		
		// Non-matching path - should default to false
		req = httptest.NewRequest("GET", "/health", nil)
		if sampler.ShouldSample(req) {
			t.Error("ExplicitPathSampler should not sample non-matching path (explicit only)")
		}
	})
	
	t.Run("builder default allow", func(t *testing.T) {
		sampler := NewPathSamplerBuilder().
			Always("/api/*").
			DefaultAllow().
			Build()
		
		req := httptest.NewRequest("GET", "/health", nil)
		if !sampler.ShouldSample(req) {
			t.Error("Builder with DefaultAllow should sample non-matching paths")
		}
	})
	
	t.Run("builder default deny", func(t *testing.T) {
		sampler := NewPathSamplerBuilder().
			Always("/api/*").
			DefaultDeny().
			Build()
		
		req := httptest.NewRequest("GET", "/health", nil)
		if sampler.ShouldSample(req) {
			t.Error("Builder with DefaultDeny should not sample non-matching paths")
		}
	})
}