// Package launch provides functions for launching zhi plugin processes.
//
// This package extracts the plugin launching logic from the internal engine
// so that meta-plugins (composition plugins that orchestrate child plugins)
// can reuse the same launch and binary audit mechanisms.
package launch

import (
	"log/slog"
)

// AuditMode controls how binary integrity verification failures are handled.
type AuditMode int

const (
	// AuditHardFail returns an error if the binary integrity check fails.
	// This is the default for the public API.
	AuditHardFail AuditMode = iota
	// AuditWarnOnly logs a warning but continues launching the plugin.
	AuditWarnOnly
)

// options holds configuration for launching a plugin process.
type options struct {
	logger      *slog.Logger
	isolatedEnv map[string]string
	auditMode   AuditMode
}

// Option configures plugin launching.
type Option func(*options)

// WithLogger sets a structured logger for the child process. The logger
// is used to report binary audit results and child process lifecycle events.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) {
		o.logger = l
	}
}

// WithIsolatedEnv restricts the child process environment to only the
// variables needed for the go-plugin handshake (ZHI_PLUGIN, PATH) plus
// the user-specified extras. This prevents leaking parent secrets or
// credentials to children. Without this option, children inherit the
// parent's full environment.
func WithIsolatedEnv(env map[string]string) Option {
	return func(o *options) {
		o.isolatedEnv = env
	}
}

// WithAuditMode controls how binary integrity check failures are handled.
// The default is [AuditHardFail] which returns an error. Use
// [AuditWarnOnly] to log a warning and continue (matching the engine's
// historical behavior).
func WithAuditMode(mode AuditMode) Option {
	return func(o *options) {
		o.auditMode = mode
	}
}

func applyOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (o *options) log() *slog.Logger {
	if o.logger != nil {
		return o.logger
	}
	return slog.Default()
}
