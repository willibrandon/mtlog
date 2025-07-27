package enrichers

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/willibrandon/mtlog/core"
)

// Default cache size, can be overridden by MTLOG_SOURCE_CTX_CACHE env var
var maxSourceContextCacheSize = getSourceContextCacheSize()

func getSourceContextCacheSize() int {
	if size := os.Getenv("MTLOG_SOURCE_CTX_CACHE"); size != "" {
		if n, err := strconv.Atoi(size); err == nil && n > 0 {
			return n
		}
	}
	return 10000 // Default size
}

// lruEntry represents an entry in the LRU cache
type lruEntry struct {
	key   uintptr
	value string
	prev  *lruEntry
	next  *lruEntry
}

// sourceContextCache caches source contexts by program counter with LRU eviction
var sourceContextCache = struct {
	sync.RWMutex
	m       map[uintptr]*lruEntry
	head    *lruEntry // Most recently used
	tail    *lruEntry // Least recently used
	size    int
	maxSize int
}{
	m:       make(map[uintptr]*lruEntry),
	maxSize: maxSourceContextCacheSize,
}

// SourceContextEnricher adds the source context (logger name/type) to log events.
type SourceContextEnricher struct {
	sourceContext string
}

// NewSourceContextEnricher creates an enricher that adds the specified source context.
func NewSourceContextEnricher(sourceContext string) *SourceContextEnricher {
	return &SourceContextEnricher{
		sourceContext: sourceContext,
	}
}

// NewAutoSourceContextEnricher creates an enricher that automatically detects the source context.
func NewAutoSourceContextEnricher() *SourceContextEnricher {
	// Return a special instance that will detect context dynamically
	return &SourceContextEnricher{
		sourceContext: "", // Empty means auto-detect
	}
}

// Enrich adds the source context to the log event.
func (e *SourceContextEnricher) Enrich(event *core.LogEvent, propertyFactory core.LogEventPropertyFactory) {
	// Don't overwrite existing SourceContext
	if _, exists := event.Properties["SourceContext"]; exists {
		return
	}
	
	if e.sourceContext != "" {
		// Use the pre-configured source context
		event.AddProperty("SourceContext", e.sourceContext)
	} else {
		// Auto-detect source context from runtime
		sourceContext := e.detectSourceContext()
		event.AddProperty("SourceContext", sourceContext)
	}
}

// normalizeSourceContext extracts a meaningful source context from a file path.
func (e *SourceContextEnricher) normalizeSourceContext(file string) string {
	// Normalize path separators
	file = strings.ReplaceAll(file, "\\", "/")
	
	// Extract just the filename without path
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		file = file[idx+1:]
	}
	
	// Remove .go extension
	file = strings.TrimSuffix(file, ".go")
	
	// If empty, return a default
	if file == "" {
		return "unknown"
	}
	
	return file
}

// detectSourceContext detects the source context from the call stack with caching.
func (e *SourceContextEnricher) detectSourceContext() string {
	// Get the caller's PC for cache lookup
	var callerPC uintptr
	pcs := make([]uintptr, 1)
	n := runtime.Callers(4, pcs) // Skip runtime.Callers, detectSourceContext, Enrich, and the logger method
	if n > 0 {
		callerPC = pcs[0]
		
		// Check cache first
		sourceContextCache.RLock()
		if entry, ok := sourceContextCache.m[callerPC]; ok {
			ctx := entry.value
			sourceContextCache.RUnlock()
			// Move to front (most recently used)
			sourceContextCache.Lock()
			moveToFront(entry)
			sourceContextCache.Unlock()
			return ctx
		}
		sourceContextCache.RUnlock()
	}
	
	// Not in cache, need to detect
	sourceContext := "unknown"
	
	// Walk up the call stack
	pcs = make([]uintptr, 20)
	n = runtime.Callers(3, pcs) // Skip runtime.Callers, this function, and immediate caller
	
	if n > 0 {
		frames := runtime.CallersFrames(pcs[:n])
		for {
			frame, more := frames.Next()
			
			// Skip mtlog internal packages
			if strings.HasPrefix(frame.Function, "github.com/willibrandon/mtlog.") ||
			   strings.HasPrefix(frame.Function, "github.com/willibrandon/mtlog/") {
				if !more {
					break
				}
				continue
			}
			
			// Skip runtime internals
			if strings.HasPrefix(frame.Function, "runtime.") {
				if !more {
					break
				}
				continue
			}
			
			// Found user code
			sourceContext = e.normalizeSourceContext(frame.File)
			break
		}
	}
	
	// Cache the result if we have a valid caller PC
	if callerPC != 0 {
		sourceContextCache.Lock()
		defer sourceContextCache.Unlock()
		
		// Check if already exists (double-check after acquiring write lock)
		if entry, ok := sourceContextCache.m[callerPC]; ok {
			moveToFront(entry)
			return sourceContext
		}
		
		// Check if we need to evict
		if sourceContextCache.size >= sourceContextCache.maxSize {
			// Evict least recently used (tail)
			if sourceContextCache.tail != nil {
				delete(sourceContextCache.m, sourceContextCache.tail.key)
				removeEntry(sourceContextCache.tail)
				sourceContextCache.size--
			}
		}
		
		// Add new entry at front
		entry := &lruEntry{
			key:   callerPC,
			value: sourceContext,
		}
		sourceContextCache.m[callerPC] = entry
		addToFront(entry)
		sourceContextCache.size++
	}
	
	return sourceContext
}

// ForSourceContext creates a new enricher with the specified source context.
// This is useful for creating sub-loggers with specific contexts.
func ForSourceContext(sourceContext string) *SourceContextEnricher {
	return NewSourceContextEnricher(sourceContext)
}

// LRU cache helper functions

// moveToFront moves an entry to the front of the LRU list
func moveToFront(entry *lruEntry) {
	if sourceContextCache.head == entry {
		return // Already at front
	}
	removeEntry(entry)
	addToFront(entry)
}

// removeEntry removes an entry from the LRU list
func removeEntry(entry *lruEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		sourceContextCache.head = entry.next
	}
	
	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		sourceContextCache.tail = entry.prev
	}
	
	entry.prev = nil
	entry.next = nil
}

// addToFront adds an entry to the front of the LRU list
func addToFront(entry *lruEntry) {
	entry.next = sourceContextCache.head
	entry.prev = nil
	
	if sourceContextCache.head != nil {
		sourceContextCache.head.prev = entry
	}
	sourceContextCache.head = entry
	
	if sourceContextCache.tail == nil {
		sourceContextCache.tail = entry
	}
}