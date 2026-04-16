package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

// dlog is the desktop debug logger. Writes to ~/.haft/desktop.log.
// Initialized in startup() via initDebugLog(), not init() — because
// wails build runs init() during compilation for binding generation.
var dlog = zerolog.Nop()

// initDebugLog must be called explicitly from startup(), not from init().
// Go's init() runs during wails build for binding generation, creating
// stale file handles that get inherited by the actual app process.
func initDebugLog() {
	if os.Getenv("HAFT_DEBUG") == "0" {
		dlog = zerolog.Nop()
	} else {
		dlog = newFileLogger()
	}
}

func newFileLogger() zerolog.Logger {
	home, err := os.UserHomeDir()
	if err != nil {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly}).
			With().Timestamp().Str("app", "haft-desktop").Logger()
	}

	logDir := filepath.Join(home, ".haft")
	_ = os.MkdirAll(logDir, 0o755)
	logPath := filepath.Join(logDir, "desktop.log")

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly}).
			With().Timestamp().Str("app", "haft-desktop").Logger()
	}

	// Write to both file and stderr for dev convenience
	multi := zerolog.MultiLevelWriter(
		file,
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.TimeOnly},
	)

	fmt.Fprintf(os.Stderr, "haft desktop: debug log → %s\n", logPath)

	return zerolog.New(multi).
		With().
		Timestamp().
		Str("app", "haft-desktop").
		Logger()
}
