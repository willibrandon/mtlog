package parser

import (
	"sync"
)

// templateCache is a thread-safe cache for parsed templates.
var templateCache = &struct {
	sync.RWMutex
	templates map[string]*MessageTemplate
}{
	templates: make(map[string]*MessageTemplate),
}

// ParseCached parses a template with caching to avoid repeated allocations.
func ParseCached(template string) (*MessageTemplate, error) {
	// Check cache first
	templateCache.RLock()
	if cached, ok := templateCache.templates[template]; ok {
		templateCache.RUnlock()
		return cached, nil
	}
	templateCache.RUnlock()
	
	// Parse if not cached
	parsed, err := Parse(template)
	if err != nil {
		return nil, err
	}
	
	// Store in cache
	templateCache.Lock()
	templateCache.templates[template] = parsed
	templateCache.Unlock()
	
	return parsed, nil
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
	templateCache.Lock()
	templateCache.templates = make(map[string]*MessageTemplate)
	templateCache.Unlock()
}