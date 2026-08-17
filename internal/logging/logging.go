// Package logging implements lore's local logging approach, decided in
// https://github.com/cheetahbyte/lore/issues/13: log/slog, always to
// stderr (stdout is reserved for MCP protocol traffic during `lore
// serve`), with a text or JSON handler and level controlled by --verbose
// or LORE_LOG_LEVEL (flag wins).
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Options configures New. Verbose, if true, always wins over Level.
type Options struct {
	Verbose bool
	Level   string // "debug" | "info" | "warn" | "error"; ignored if Verbose
	JSON    bool
}

// New builds the process-wide logger per Options, per issue #13.
func New(opts Options) *slog.Logger {
	level := slog.LevelInfo
	switch {
	case opts.Verbose:
		level = slog.LevelDebug
	case opts.Level != "":
		level = parseLevel(opts.Level)
	case os.Getenv("LORE_LOG_LEVEL") != "":
		level = parseLevel(os.Getenv("LORE_LOG_LEVEL"))
	}

	handlerOpts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if opts.JSON {
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	}
	return slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
