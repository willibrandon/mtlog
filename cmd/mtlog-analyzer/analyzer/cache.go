package analyzer

import (
	"fmt"
	"os"
	"strings"
)

// templateCache avoids redundant parsing within a single pass
type templateCache struct {
	cache map[string]templateInfo
	fileCache map[string]*cachedFile  // Cache for source files
}

type templateInfo struct {
	properties []string
	err        error
}

// cachedFile holds cached source file content for indentation extraction
type cachedFile struct {
	lines []string  // Cached lines of the file
}

// getSourceLine retrieves a specific line from the source file, using cache if possible
func (tc *templateCache) getSourceLine(filename string, lineNum int) (string, error) {
	// Check if file is already cached
	if tc.fileCache == nil {
		tc.fileCache = make(map[string]*cachedFile)
	}
	
	cached, exists := tc.fileCache[filename]
	if !exists {
		// Read the file for the first time
		content, err := os.ReadFile(filename)
		if err != nil {
			return "", err
		}
		
		lines := strings.Split(string(content), "\n")
		cached = &cachedFile{lines: lines}
		tc.fileCache[filename] = cached
	}
	
	// Return the requested line (1-based line numbers)
	if lineNum <= 0 || lineNum > len(cached.lines) {
		return "", fmt.Errorf("line %d out of range", lineNum)
	}
	
	return cached.lines[lineNum-1], nil
}

// getTemplateInfo retrieves template info from cache or parses if needed
func (tc *templateCache) getTemplateInfo(template string, config *Config) templateInfo {
	// Include strict mode in cache key to avoid cross-mode pollution
	cacheKey := template
	if config.StrictMode {
		cacheKey = "strict:" + template
	}
	
	if info, ok := tc.cache[cacheKey]; ok {
		return info
	}
	
	properties, err := extractProperties(template)
	info := templateInfo{properties: properties, err: err}
	tc.cache[cacheKey] = info
	return info
}

// extractIndentation extracts the leading whitespace from a line
func extractIndentation(line string) string {
	var indent strings.Builder
	for _, r := range line {
		if r == '\t' || r == ' ' {
			indent.WriteRune(r)
		} else {
			break
		}
	}
	return indent.String()
}

// containsMixedIndent checks if the byte count suggests mixed tabs/spaces
// For example: 3 bytes could be tab+2spaces, 4 bytes could be 4 spaces (not tabs)
func containsMixedIndent(bytes int) bool {
	// Common mixed patterns in Go code (rare but possible):
	// 3 = tab + 2 spaces
	// 5 = tab + 4 spaces  
	// 6 = tab + 5 spaces
	// 7 = tab + 6 spaces
	// Basically any non-tab-aligned count could be mixed
	return bytes == 3 || bytes == 5 || bytes == 6 || bytes == 7
}