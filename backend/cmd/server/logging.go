package main

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// setupLogging installs the process-wide slog handler.
//
// Go's standard log package writes through the default slog handler, so every
// existing log.Printf in the backend becomes a structured record at INFO
// without being rewritten. That is what gives the log collector a severity to
// label this container's lines with.
//
// LOG_FORMAT=text swaps the JSON encoder for the human-readable one during
// local development; anything else, unset included, is JSON. LOG_LEVEL takes
// any level slog itself parses (debug, info, warn, error) and falls back to
// info.
func setupLogging() {
	handler, badLevel := newLogHandler(os.Stderr, os.Getenv("LOG_FORMAT"), os.Getenv("LOG_LEVEL"))
	slog.SetDefault(slog.New(handler))
	if badLevel != "" {
		slog.Warn("ignoring unparseable LOG_LEVEL", "value", badLevel)
	}
}

// newLogHandler builds the handler setupLogging installs and reports the
// LOG_LEVEL value it could not parse, if any, so the caller can complain about
// it once a logger exists.
func newLogHandler(w io.Writer, format, level string) (slog.Handler, string) {
	options := &slog.HandlerOptions{Level: slog.LevelInfo}
	badLevel := ""
	if trimmed := strings.TrimSpace(level); trimmed != "" {
		var parsed slog.Level
		if err := parsed.UnmarshalText([]byte(trimmed)); err == nil {
			options.Level = parsed
		} else {
			badLevel = trimmed
		}
	}
	if strings.EqualFold(strings.TrimSpace(format), "text") {
		return slog.NewTextHandler(w, options), badLevel
	}
	return slog.NewJSONHandler(w, options), badLevel
}
