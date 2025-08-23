package core

import "time"

// DeadlineStats provides statistics about deadline tracking.
type DeadlineStats struct {
	CacheSize            int           // Current number of contexts in the deadline cache
	CacheCapacity        int           // Maximum capacity of the deadline cache
	FirstWarningCount    int           // Number of contexts that have received first warnings
	FirstWarningCapacity int           // Maximum capacity of first warning set
	CacheTTL             time.Duration // Time-to-live for cache entries
}