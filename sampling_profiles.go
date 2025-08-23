package mtlog

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/filters"
	"github.com/willibrandon/mtlog/selflog"
)

// SamplingProfile represents a predefined sampling configuration for common scenarios with versioning support
type SamplingProfile struct {
	name        string
	description string
	version     string                           // Version string for backward compatibility (e.g., "1.0", "2.1")
	deprecated  bool                             // Mark profile as deprecated
	replacement string                           // Suggested replacement profile name if deprecated
	factory     func() core.LogEventFilter
}

// MigrationConsent represents user consent for profile version migration
type MigrationConsent int

const (
	// MigrationDeny - Do not migrate, use requested version or fail
	MigrationDeny MigrationConsent = iota
	// MigrationPrompt - Ask for consent (default, logs warning and uses current)
	MigrationPrompt
	// MigrationAuto - Automatically migrate to latest version
	MigrationAuto
)

// MigrationPolicy controls how profile version migration is handled
type MigrationPolicy struct {
	Consent            MigrationConsent // How to handle missing versions
	PreferStable       bool             // Prefer stable versions over latest
	MaxVersionDistance int              // Maximum version distance for auto-migration (0 = no limit)
}

// profileRegistry manages sampling profiles with thread-safe access, immutability, and versioning support
type profileRegistry struct {
	mu               sync.RWMutex
	profiles         map[string]SamplingProfile              // Current profiles by name
	versionedProfiles map[string]map[string]SamplingProfile // Profile name -> version -> profile
	frozen           bool                                     // Prevents further modifications after being frozen
	migrationPolicy  MigrationPolicy                         // Policy for handling version migrations
}

// globalProfileRegistry is the global registry for sampling profiles
var globalProfileRegistry = &profileRegistry{
	profiles:         make(map[string]SamplingProfile),
	versionedProfiles: make(map[string]map[string]SamplingProfile),
	frozen:           false,
	migrationPolicy: MigrationPolicy{
		Consent:            MigrationPrompt, // Default to prompting user
		PreferStable:       true,           // Prefer stable versions
		MaxVersionDistance: 2,              // Allow migration within 2 major versions
	},
}

// initializePredefinedProfiles sets up the default sampling profiles
func init() {
	initializePredefinedProfiles()
}

// initializePredefinedProfiles populates the registry with common sampling configurations
func initializePredefinedProfiles() {
	predefined := map[string]SamplingProfile{
	"HighTrafficAPI": {
		name:        "High Traffic API",
		description: "Optimized for high-traffic API endpoints with 1% sampling rate",
		version:     "1.0",
		deprecated:  false,
		factory: func() core.LogEventFilter {
			return filters.NewRateSamplingFilter(0.01) // 1% sampling
		},
	},
	"BackgroundWorker": {
		name:        "Background Worker",
		description: "Suitable for background workers with every 10th message sampling",
		version:     "1.0",
		deprecated:  false,
		factory: func() core.LogEventFilter {
			return filters.NewCounterSamplingFilter(10) // Every 10th message
		},
	},
	"DebugVerbose": {
		name:        "Debug Verbose",
		description: "Development mode with first 100 messages then 10% sampling",
		version:     "1.0",
		deprecated:  false,
		factory: func() core.LogEventFilter {
			// Create a composite filter: first 100 messages OR 10% sampling
			return &compositeSamplingFilter{
				filters: []core.LogEventFilter{
					filters.NewFirstNSamplingFilter(100),
					filters.NewRateSamplingFilter(0.1),
				},
				mode: compositeOR,
			}
		},
	},
	"ProductionErrors": {
		name:        "Production Errors",
		description: "Error logging with exponential backoff for production environments",
		version:     "1.0",
		deprecated:  false,
		factory: func() core.LogEventFilter {
			return filters.NewBackoffSamplingFilter("prod-errors", 2.0, globalBackoffState)
		},
	},
	"HealthChecks": {
		name:        "Health Checks",
		description: "Health check endpoints with time-based sampling (once per minute)",
		version:     "1.0",
		deprecated:  false,
		factory: func() core.LogEventFilter {
			return filters.NewDurationSamplingFilter(1 * time.Minute)
		},
	},
	"CriticalAlerts": {
		name:        "Critical Alerts",
		description: "Critical alerts with first 50 occurrences then exponential backoff",
		version:     "1.0",
		deprecated:  false,
		factory: func() core.LogEventFilter {
			// Create a composite filter: first 50 messages OR exponential backoff
			return &compositeSamplingFilter{
				filters: []core.LogEventFilter{
					filters.NewFirstNSamplingFilter(50),
					filters.NewBackoffSamplingFilter("critical-alerts", 3.0, globalBackoffState),
				},
				mode: compositeOR,
			}
		},
	},
	"DevelopmentDebug": {
		name:        "Development Debug",
		description: "Development environment with minimal sampling (every 2nd message)",
		version:     "1.0",
		deprecated:  false,
		factory: func() core.LogEventFilter {
			return filters.NewCounterSamplingFilter(2) // Every 2nd message
		},
	},
	"PerformanceMetrics": {
		name:        "Performance Metrics",
		description: "Performance metrics with every 100th sample",
		version:     "1.0",
		deprecated:  false,
		factory: func() core.LogEventFilter {
			return filters.NewCounterSamplingFilter(100) // Every 100th message
		},
	},
	}

	// Register all predefined profiles with versioning support
	for name, profile := range predefined {
		globalProfileRegistry.profiles[name] = profile
		
		// Store versioned profile
		if globalProfileRegistry.versionedProfiles[name] == nil {
			globalProfileRegistry.versionedProfiles[name] = make(map[string]SamplingProfile)
		}
		globalProfileRegistry.versionedProfiles[name][profile.version] = profile
	}
}

// getProfile safely retrieves a profile from the registry
func (r *profileRegistry) getProfile(name string) (SamplingProfile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	profile, exists := r.profiles[name]
	return profile, exists
}

// addProfile safely adds a new profile to the registry if not frozen
func (r *profileRegistry) addProfile(name string, profile SamplingProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.frozen {
		return fmt.Errorf("profile registry is frozen, cannot add profile '%s'", name)
	}
	
	// Set default version if not specified
	if profile.version == "" {
		profile.version = "1.0"
	}
	
	r.profiles[name] = profile
	
	// Store versioned profile
	if r.versionedProfiles[name] == nil {
		r.versionedProfiles[name] = make(map[string]SamplingProfile)
	}
	r.versionedProfiles[name][profile.version] = profile
	
	return nil
}

// getProfileWithVersion safely retrieves a specific version of a profile from the registry
func (r *profileRegistry) getProfileWithVersion(name, version string) (SamplingProfile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if versions, exists := r.versionedProfiles[name]; exists {
		if profile, found := versions[version]; found {
			return profile, true
		}
	}
	
	return SamplingProfile{}, false
}

// getProfileVersions returns all available versions for a profile
func (r *profileRegistry) getProfileVersions(name string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var versions []string
	if profileVersions, exists := r.versionedProfiles[name]; exists {
		for version := range profileVersions {
			versions = append(versions, version)
		}
	}
	
	return versions
}

// freeze makes the registry immutable
func (r *profileRegistry) freeze() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frozen = true
}

// isFrozen checks if the registry is frozen
func (r *profileRegistry) isFrozen() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.frozen
}

// getProfileNames returns all available profile names
func (r *profileRegistry) getProfileNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.profiles))
	for name := range r.profiles {
		names = append(names, name)
	}
	return names
}

// parseVersion parses a semantic version string into major.minor.patch components
func parseVersion(version string) (major, minor, patch int, err error) {
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return 0, 0, 0, fmt.Errorf("invalid version format: %s", version)
	}
	
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version: %s", parts[0])
	}
	
	if len(parts) > 1 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid minor version: %s", parts[1])
		}
	}
	
	if len(parts) > 2 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid patch version: %s", parts[2])
		}
	}
	
	return major, minor, patch, nil
}

// compareVersions returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	maj1, min1, pat1, err1 := parseVersion(v1)
	maj2, min2, pat2, err2 := parseVersion(v2)
	
	if err1 != nil || err2 != nil {
		// Fallback to string comparison if parsing fails
		if v1 < v2 {
			return -1
		} else if v1 > v2 {
			return 1
		}
		return 0
	}
	
	if maj1 != maj2 {
		if maj1 < maj2 {
			return -1
		}
		return 1
	}
	
	if min1 != min2 {
		if min1 < min2 {
			return -1
		}
		return 1
	}
	
	if pat1 != pat2 {
		if pat1 < pat2 {
			return -1
		}
		return 1
	}
	
	return 0
}

// getVersionDistance calculates the distance between two versions (major version difference)
func getVersionDistance(fromVersion, toVersion string) int {
	maj1, _, _, err1 := parseVersion(fromVersion)
	maj2, _, _, err2 := parseVersion(toVersion)
	
	if err1 != nil || err2 != nil {
		return 0 // Can't calculate distance, assume compatible
	}
	
	distance := maj2 - maj1
	if distance < 0 {
		distance = -distance
	}
	return distance
}

// findBestVersionForMigration finds the best version to migrate to based on policy
func (r *profileRegistry) findBestVersionForMigration(profileName string, requestedVersion string) (string, SamplingProfile, bool) {
	versions, exists := r.versionedProfiles[profileName]
	if !exists {
		return "", SamplingProfile{}, false
	}
	
	// Get all available versions sorted
	availableVersions := make([]string, 0, len(versions))
	for version := range versions {
		availableVersions = append(availableVersions, version)
	}
	
	// Sort versions in descending order (newest first)
	sort.Slice(availableVersions, func(i, j int) bool {
		return compareVersions(availableVersions[i], availableVersions[j]) > 0
	})
	
	// Find best candidate based on policy
	for _, version := range availableVersions {
		profile := versions[version]
		
		// Skip deprecated versions unless specifically requested
		if profile.deprecated && r.migrationPolicy.PreferStable {
			continue
		}
		
		// Check version distance constraint
		if r.migrationPolicy.MaxVersionDistance >= 0 { // Changed from > 0 to >= 0 so 0 is also enforced
			distance := getVersionDistance(requestedVersion, version)
			if distance > r.migrationPolicy.MaxVersionDistance {
				continue
			}
		}
		
		// This version meets our criteria
		return version, profile, true
	}
	
	// Fallback: return the latest version regardless of policy
	if len(availableVersions) > 0 {
		latestVersion := availableVersions[0]
		return latestVersion, versions[latestVersion], true
	}
	
	return "", SamplingProfile{}, false
}

// getProfileWithMigration retrieves a profile with migration support
func (r *profileRegistry) getProfileWithMigration(name, requestedVersion string) (SamplingProfile, string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// First try to get the exact version requested
	if versions, exists := r.versionedProfiles[name]; exists {
		if profile, found := versions[requestedVersion]; found {
			return profile, requestedVersion, true
		}
	}
	
	// Exact version not found, check migration policy
	switch r.migrationPolicy.Consent {
	case MigrationDeny:
		// Don't migrate, return failure
		return SamplingProfile{}, "", false
		
	case MigrationPrompt:
		// Log warning and attempt migration
		bestVersion, bestProfile, found := r.findBestVersionForMigration(name, requestedVersion)
		if found {
			if selflog.IsEnabled() {
				selflog.Printf("Profile '%s' version '%s' not found, using version '%s' instead. Consider updating your configuration.", 
					name, requestedVersion, bestVersion)
			}
			return bestProfile, bestVersion, true
		}
		return SamplingProfile{}, "", false
		
	case MigrationAuto:
		// Automatically migrate to best version
		bestVersion, bestProfile, found := r.findBestVersionForMigration(name, requestedVersion)
		if found {
			if selflog.IsEnabled() {
				selflog.Printf("Auto-migrated profile '%s' from version '%s' to version '%s'", 
					name, requestedVersion, bestVersion)
			}
			return bestProfile, bestVersion, true
		}
		return SamplingProfile{}, "", false
		
	default:
		return SamplingProfile{}, "", false
	}
}

// SetMigrationPolicy updates the global migration policy
func SetMigrationPolicy(policy MigrationPolicy) error {
	if globalProfileRegistry.isFrozen() {
		return fmt.Errorf("cannot set migration policy: registry is frozen")
	}
	
	globalProfileRegistry.mu.Lock()
	defer globalProfileRegistry.mu.Unlock()
	globalProfileRegistry.migrationPolicy = policy
	return nil
}

// GetMigrationPolicy returns the current migration policy
func GetMigrationPolicy() MigrationPolicy {
	globalProfileRegistry.mu.RLock()
	defer globalProfileRegistry.mu.RUnlock()
	return globalProfileRegistry.migrationPolicy
}

// SampleProfile applies a predefined sampling profile to the logger
func (l *logger) SampleProfile(profileName string) core.Logger {
	profile, exists := globalProfileRegistry.getProfile(profileName)
	if !exists {
		// If profile doesn't exist, return logger unchanged
		// Could also default to a conservative profile
		return l
	}
	
	filter := profile.factory()
	
	// Create a new logger configuration with the filter
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
		fields:       l.fields, // Copy the fields for properties
		properties:   l.properties, // Copy the properties map
		samplingFilter: l.samplingFilter, // Copy existing sampling filter if any
	}
}

// SampleProfileWithVersion applies a specific version of a predefined sampling profile to the logger
func (l *logger) SampleProfileWithVersion(profileName, version string) core.Logger {
	profile, actualVersion, exists := globalProfileRegistry.getProfileWithMigration(profileName, version)
	if !exists {
		// If profile doesn't exist at all, return logger unchanged
		return l
	}
	
	// Check if profile is deprecated and log warning
	if profile.deprecated && selflog.IsEnabled() {
		if profile.replacement != "" {
			selflog.Printf("Using deprecated sampling profile '%s' version '%s'. Consider migrating to '%s'", 
				profileName, actualVersion, profile.replacement)
		} else {
			selflog.Printf("Using deprecated sampling profile '%s' version '%s'", profileName, actualVersion)
		}
	}
	
	filter := profile.factory()
	
	// Create a new logger configuration with the filter
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
		fields:       l.fields, // Copy the fields for properties
		properties:   l.properties, // Copy the properties map
		samplingFilter: l.samplingFilter, // Copy existing sampling filter if any
	}
}

// GetAvailableProfiles returns a list of available sampling profiles
func GetAvailableProfiles() []string {
	return globalProfileRegistry.getProfileNames()
}

// GetProfileDescription returns the description for a given profile
func GetProfileDescription(profileName string) (string, bool) {
	profile, exists := globalProfileRegistry.getProfile(profileName)
	if !exists {
		return "", false
	}
	return profile.description, true
}

// GetProfileWithVersion returns a specific version of a profile
func GetProfileWithVersion(profileName, version string) (SamplingProfile, bool) {
	return globalProfileRegistry.getProfileWithVersion(profileName, version)
}

// GetProfileWithMigration retrieves a profile with automatic migration support
// Returns the profile, the actual version used, and whether it was found
func GetProfileWithMigration(profileName, requestedVersion string) (SamplingProfile, string, bool) {
	return globalProfileRegistry.getProfileWithMigration(profileName, requestedVersion)
}

// GetProfileVersions returns all available versions for a profile
func GetProfileVersions(profileName string) []string {
	return globalProfileRegistry.getProfileVersions(profileName)
}

// GetProfileVersion returns the version of the currently active profile
func GetProfileVersion(profileName string) (string, bool) {
	profile, exists := globalProfileRegistry.getProfile(profileName)
	if !exists {
		return "", false
	}
	return profile.version, true
}

// IsProfileDeprecated checks if a profile is marked as deprecated
func IsProfileDeprecated(profileName string) (bool, string) {
	profile, exists := globalProfileRegistry.getProfile(profileName)
	if !exists {
		return false, ""
	}
	return profile.deprecated, profile.replacement
}

// AddCustomProfile allows users to add their own sampling profiles (before freezing)
func AddCustomProfile(name, description string, factory func() core.LogEventFilter) error {
	return globalProfileRegistry.addProfile(name, SamplingProfile{
		name:        description,
		description: description,
		version:     "1.0", // Default version for backward compatibility
		deprecated:  false,
		factory:     factory,
	})
}

// AddCustomProfileWithVersion allows users to add versioned sampling profiles (before freezing)
func AddCustomProfileWithVersion(name, description, version string, deprecated bool, replacement string, factory func() core.LogEventFilter) error {
	return globalProfileRegistry.addProfile(name, SamplingProfile{
		name:        description,
		description: description,
		version:     version,
		deprecated:  deprecated,
		replacement: replacement,
		factory:     factory,
	})
}

// FreezeProfiles makes the profile registry immutable, preventing further modifications.
// This should be called after all custom profiles have been registered, typically during
// application initialization to ensure thread-safety and prevent accidental modifications.
func FreezeProfiles() {
	globalProfileRegistry.freeze()
}

// IsProfileRegistryFrozen returns true if the profile registry has been frozen.
func IsProfileRegistryFrozen() bool {
	return globalProfileRegistry.isFrozen()
}

// resetProfileRegistryForTesting resets the profile registry to unfrozen state (for testing only).
// This function should only be used in tests to ensure clean state between test runs.
func resetProfileRegistryForTesting() {
	globalProfileRegistry.mu.Lock()
	defer globalProfileRegistry.mu.Unlock()
	globalProfileRegistry.frozen = false
}

// RegisterCustomProfiles allows bulk registration of custom profiles before freezing.
// Returns an error if any profile fails to register or if the registry is already frozen.
func RegisterCustomProfiles(customProfiles map[string]SamplingProfile) error {
	if globalProfileRegistry.isFrozen() {
		return fmt.Errorf("cannot register custom profiles: registry is frozen")
	}
	
	for name, profile := range customProfiles {
		if err := globalProfileRegistry.addProfile(name, profile); err != nil {
			return fmt.Errorf("failed to register profile '%s': %w", name, err)
		}
	}
	
	return nil
}