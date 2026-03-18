// Package logger provides structured logging via zerolog.
// All components use Component("name") to get a sub-logger with
// the component field pre-set. Output is JSON to stderr by default.
//
// Usage:
//
//	logger.Init("info")
//	log := logger.Component("scheduler")
//	log.Info().Str("job", "cot-fetch").Msg("started")
package logger

import (
	"os"

	"github.com/rs/zerolog"
)

// Log is the root logger instance. Use Component() for sub-loggers.
var Log zerolog.Logger

// Init initializes the global logger with the given level string.
// Valid levels: "trace", "debug", "info", "warn", "error", "fatal", "panic".
// Defaults to "info" if the level string is invalid.
func Init(level string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	// Use console writer for development, JSON for production.
	// Since this is a Telegram bot (not a web service), we keep JSON
	// for structured log aggregation but include timestamps and caller.
	Log = zerolog.New(os.Stderr).With().
		Timestamp().
		Caller().
		Logger()
}

// Component returns a sub-logger tagged with a component name.
// Example: logger.Component("scheduler") -> {"component":"scheduler",...}
func Component(name string) zerolog.Logger {
	return Log.With().Str("component", name).Logger()
}
