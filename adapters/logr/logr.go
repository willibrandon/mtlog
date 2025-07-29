// Package logr provides an adapter that allows mtlog to be used as a backend for logr,
// the structured logging interface used throughout the Kubernetes ecosystem.
//
// This adapter enables Kubernetes controllers and operators to leverage mtlog's
// powerful features including message templates, pipeline architecture, and
// rich sink ecosystem while maintaining compatibility with logr's API.
//
// # Basic Usage
//
// Create a logr.Logger backed by mtlog:
//
//	import (
//	    "github.com/willibrandon/mtlog"
//	    mtlogr "github.com/willibrandon/mtlog/adapters/logr"
//	)
//
//	logger := mtlogr.NewLogger(
//	    mtlog.WithConsole(),
//	    mtlog.WithMinimumLevel(core.DebugLevel),
//	)
//
//	// Use standard logr API
//	logger.Info("reconciling", "namespace", "default", "name", "my-app")
//	logger.Error(err, "failed to update resource")
//
// # V-Level Mapping
//
// logr V-levels are mapped to mtlog levels as follows:
//   - V(0) → Information
//   - V(1) → Debug
//   - V(2+) → Verbose
//
// # Advanced Usage
//
// For more control, create a custom sink:
//
//	mtlogLogger := mtlog.New(
//	    mtlog.WithSeq("http://localhost:5341"),
//	    mtlog.WithProperty("service", "my-controller"),
//	)
//	logrLogger := logr.New(mtlogr.NewLogrSink(mtlogLogger))
package logr

import (
	"github.com/go-logr/logr"
	"github.com/willibrandon/mtlog"
)

// NewLogger creates a new logr.Logger backed by mtlog.
//
// This is the simplest way to create a logr logger that writes to mtlog.
// All standard mtlog options can be passed to configure the underlying logger,
// including sinks, enrichers, filters, and minimum level.
//
// Example:
//
//	logger := mtlogr.NewLogger(
//	    mtlog.WithConsole(),
//	    mtlog.WithSeq("http://localhost:5341"),
//	    mtlog.WithProperty("app", "my-service"),
//	    mtlog.WithMinimumLevel(core.DebugLevel),
//	)
func NewLogger(options ...mtlog.Option) logr.Logger {
	logger := mtlog.New(options...)
	return logr.New(NewLogrSink(logger))
}