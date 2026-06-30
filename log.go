package main

import (
	"io"
	"log/slog"
	"os"

	"github.com/matt-riley/skill-evaluator/internal/agit"
)

// Default to a no-op logger so tests and helpers don't panic when they log
// before main has called initLogger.
var logger = slog.New(slog.NewTextHandler(io.Discard, nil))

func initLogger(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	agit.Logger = logger
}
