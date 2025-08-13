package parser

// ParseCached parses a template with caching to avoid repeated allocations.
// It uses the global LRU cache with bounded size to prevent memory exhaustion.
func ParseCached(template string) (*MessageTemplate, error) {
	cache := GetGlobalCache()
	
	// Check cache first
	if cached, ok := cache.Get(template); ok {
		return cached, nil
	}
	
	// Parse if not cached
	parsed, err := Parse(template)
	if err != nil {
		return nil, err
	}
	
	// Store in cache
	cache.Put(template, parsed)
	
	return parsed, nil
}

// ConfigureCache is a convenience function to configure the global cache
// This should be called at application startup before any cache usage
func ConfigureCache(opts ...CacheOption) {
	ConfigureGlobalCache(opts...)
}

// ExtractPropertyNamesFromTemplate extracts property names from an already parsed template.
func ExtractPropertyNamesFromTemplate(tmpl *MessageTemplate) []string {
	names := make([]string, 0, len(tmpl.Tokens)/2) // Pre-allocate with reasonable capacity
	seen := make(map[string]bool)
	
	for _, token := range tmpl.Tokens {
		if prop, ok := token.(*PropertyToken); ok {
			if !seen[prop.PropertyName] {
				names = append(names, prop.PropertyName)
				seen[prop.PropertyName] = true
			}
		}
	}
	
	return names
}

// ClearCache clears the template cache (useful for tests).
func ClearCache() {
	cache := GetGlobalCache()
	cache.Clear()
}

// GetCacheStats returns global cache statistics
func GetCacheStats() Stats {
	cache := GetGlobalCache()
	return cache.Stats()
}