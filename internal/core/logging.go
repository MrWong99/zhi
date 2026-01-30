package core

import (
	"io"
	"log/slog"
	"os"
)

// logger is the package-level structured logger. It defaults to WARN level
// and writes to stderr so that stdout remains clean for piped output.
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelWarn,
}))

// SetLogLevel configures the global logger with the given level.
// Call this early (e.g. from CLI flag handling) before any logging occurs.
func SetLogLevel(level slog.Level) {
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
}

// SetLogOutput configures the logger to write to a custom writer.
// This is primarily useful in tests.
func SetLogOutput(w io.Writer, level slog.Level) {
	logger = slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
	}))
}

// Logger returns the package-level logger for use in core functions.
func Logger() *slog.Logger {
	return logger
}
