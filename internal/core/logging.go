package core

import (
	"io"
	"os"

	"github.com/hashicorp/go-hclog"
)

// logger is the package-level structured logger. It defaults to WARN level
// and writes to stderr so that stdout remains clean for piped output.
// hclog is used instead of log/slog because go-plugin requires an hclog
// logger for structured log forwarding from plugin processes.
var logger = hclog.New(&hclog.LoggerOptions{
	Name:   "zhi",
	Level:  hclog.Warn,
	Output: os.Stderr,
})

// SetLogLevel configures the global logger with the given level.
// Call this early (e.g. from CLI flag handling) before any logging occurs.
func SetLogLevel(level hclog.Level) {
	logger = hclog.New(&hclog.LoggerOptions{
		Name:   "zhi",
		Level:  level,
		Output: os.Stderr,
	})
}

// SetLogOutput configures the logger to write to a custom writer.
// This is primarily useful in tests.
func SetLogOutput(w io.Writer, level hclog.Level) {
	logger = hclog.New(&hclog.LoggerOptions{
		Name:   "zhi",
		Level:  level,
		Output: w,
	})
}

// Logger returns the package-level logger for use in core functions.
func Logger() hclog.Logger {
	return logger
}
