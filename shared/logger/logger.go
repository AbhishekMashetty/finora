// Package logger provides a single structured JSON logger construction point
// shared by every Finora service, so log shape stays identical across the fleet
// and a future log aggregator (e.g. Loki) needs no per-service parsing rules.
package logger

import (
	"log/slog"
	"os"
)

// New returns a JSON slog.Logger with "service" bound to every record.
// Level defaults to info; pass "debug" via levelName for verbose local dev.
func New(serviceName, levelName string) *slog.Logger {
	var level slog.Level
	switch levelName {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler).With(slog.String("service", serviceName))
}
