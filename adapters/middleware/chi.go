package middleware

import (
	"net/http"

	"github.com/willibrandon/mtlog/core"
)

// Chi creates a Chi middleware for request logging
// Chi uses standard net/http middleware, so we can reuse our core implementation
func Chi(logger core.Logger, opts ...*Options) func(http.Handler) http.Handler {
	var options *Options
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	} else {
		options = DefaultOptions(logger)
	}
	
	// Ensure logger is set
	if options.Logger == nil {
		options.Logger = logger
	}
	
	// Ensure CustomLevelFunc is set
	if options.CustomLevelFunc == nil {
		options.CustomLevelFunc = defaultLevelFunc
	}

	// Chi uses standard net/http middleware
	return Middleware(options)
}