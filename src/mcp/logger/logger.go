package logger

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	log     zerolog.Logger
	logFile *os.File
	once    sync.Once
)

const (
	logDir     = ".quint-code"
	logsSubDir = "logs"
	maxLogSize = 10 * 1024 * 1024 // 10MB
)

func Init(projectRoot string) error {
	var initErr error
	once.Do(func() {
		initErr = initLogger(projectRoot)
	})
	return initErr
}

func initLogger(projectRoot string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logDirPath := filepath.Join(homeDir, logDir, logsSubDir)
	if err := os.MkdirAll(logDirPath, 0755); err != nil {
		return err
	}

	projectName := os.Getenv("QUINT_PROJECT_NAME")
	if projectName == "" {
		projectName = filepath.Base(projectRoot)
	}
	if projectName == "" || projectName == "." || projectName == "/" {
		projectName = "unknown"
	}

	logPath := filepath.Join(logDirPath, projectName+".log")

	if info, err := os.Stat(logPath); err == nil && info.Size() > maxLogSize {
		rotated := logPath + "." + time.Now().Format("2006-01-02-150405")
		_ = os.Rename(logPath, rotated)
	}

	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	zerolog.TimeFieldFormat = time.RFC3339

	log = zerolog.New(logFile).
		With().
		Timestamp().
		Str("project", projectName).
		Logger()

	log.Info().Msg("Logger initialized")

	return nil
}

func Debug() *zerolog.Event {
	return log.Debug()
}

func Info() *zerolog.Event {
	return log.Info()
}

func Warn() *zerolog.Event {
	return log.Warn()
}

func Error() *zerolog.Event {
	return log.Error()
}

func Fatal() *zerolog.Event {
	return log.Fatal()
}

func With() zerolog.Context {
	return log.With()
}

func Output(w io.Writer) zerolog.Logger {
	return log.Output(w)
}

func Close() {
	if logFile != nil {
		_ = logFile.Close()
	}
}

func SetLevel(level zerolog.Level) {
	log = log.Level(level)
}

// --- Structured logging helpers ---

// ToolCall logs the entry of an MCP tool call.
func ToolCall(tool, action string, params map[string]string) {
	e := log.Info().Str("component", "mcp").Str("tool", tool).Str("action", action)
	for k, v := range params {
		e = e.Str(k, v)
	}
	e.Msg("tool.call")
}

// ToolResult logs the exit of an MCP tool call with duration.
func ToolResult(tool, action string, durationMs int64, err error) {
	e := log.Info().Str("component", "mcp").Str("tool", tool).Str("action", action).Int64("duration_ms", durationMs)
	if err != nil {
		e = e.Err(err).Str("status", "error")
	} else {
		e = e.Str("status", "ok")
	}
	e.Msg("tool.result")
}

// ArtifactOp logs an artifact lifecycle operation.
func ArtifactOp(op, artifactID, kind string) {
	log.Info().
		Str("component", "artifact").
		Str("op", op).
		Str("artifact_id", artifactID).
		Str("kind", kind).
		Msg("artifact." + op)
}

// CodebaseOp logs a codebase scanning operation.
func CodebaseOp(op string, count int, durationMs int64) {
	log.Info().
		Str("component", "codebase").
		Str("op", op).
		Int("count", count).
		Int64("duration_ms", durationMs).
		Msg("codebase." + op)
}
