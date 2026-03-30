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
	logDir     = ".haft"
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

// InitSession creates a per-session log file for the agent.
// Log path: ~/.haft/logs/sessions/{sessionID}.log
func InitSession(sessionID string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	sessionLogDir := filepath.Join(homeDir, logDir, logsSubDir, "sessions")
	if err := os.MkdirAll(sessionLogDir, 0755); err != nil {
		return err
	}

	logPath := filepath.Join(sessionLogDir, sessionID+".log")
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	zerolog.TimeFieldFormat = time.RFC3339
	log = zerolog.New(logFile).
		With().
		Timestamp().
		Str("session", sessionID).
		Logger()

	log.Info().Msg("Session logger initialized")
	return nil
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

	projectName := os.Getenv("HAFT_PROJECT_NAME")
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

// --- Agent logging helpers ---

// AgentSession logs session start/resume.
func AgentSession(op, sessionID, model string) {
	log.Info().
		Str("component", "agent").
		Str("op", op).
		Str("session_id", sessionID).
		Str("model", model).
		Msg("agent." + op)
}

// AgentPhase logs a lemniscate phase transition.
func AgentPhase(from, to, name string) {
	log.Info().
		Str("component", "agent").
		Str("from", from).
		Str("to", to).
		Str("phase_name", name).
		Msg("agent.phase_transition")
}

// AgentSignal logs a detected transition signal.
func AgentSignal(phase, signal, toolName string) {
	log.Info().
		Str("component", "agent").
		Str("phase", phase).
		Str("signal", signal).
		Str("tool", toolName).
		Msg("agent.signal")
}

// AgentStep logs a ReAct loop step.
func AgentStep(step int, phase string, toolCount int, hasText bool) {
	log.Debug().
		Str("component", "agent").
		Int("step", step).
		Str("phase", phase).
		Int("tool_calls", toolCount).
		Bool("has_text", hasText).
		Msg("agent.step")
}

// AgentToolGated logs when a tool is blocked by phase gating.
func AgentToolGated(phase, tool string) {
	log.Warn().
		Str("component", "agent").
		Str("phase", phase).
		Str("tool", tool).
		Msg("agent.tool_gated")
}

// AgentToolExec logs tool execution.
func AgentToolExec(tool, callID string, durationMs int64, isError bool) {
	e := log.Info().
		Str("component", "agent").
		Str("tool", tool).
		Str("call_id", callID).
		Int64("duration_ms", durationMs)
	if isError {
		e = e.Str("status", "error")
	} else {
		e = e.Str("status", "ok")
	}
	e.Msg("agent.tool_exec")
}

// AgentLLM logs an LLM call.
func AgentLLM(phase string, inputTokens, outputTokens int) {
	log.Info().
		Str("component", "agent").
		Str("phase", phase).
		Int("input_tokens", inputTokens).
		Int("output_tokens", outputTokens).
		Msg("agent.llm_call")
}

// AgentError logs an agent error.
func AgentError(phase string, err error) {
	log.Error().
		Str("component", "agent").
		Str("phase", phase).
		Err(err).
		Msg("agent.error")
}

// AgentMessage logs a conversation message (user, assistant, system, tool).
func AgentMessage(role, content string, toolCalls int, tokens int) {
	e := log.Info().
		Str("component", "agent").
		Str("role", role).
		Int("tool_calls", toolCalls).
		Int("tokens", tokens)
	// Truncate content for log readability (rune-safe, 1000 chars)
	if r := []rune(content); len(r) > 1000 {
		e = e.Str("content", string(r[:1000])+"...(truncated)")
	} else {
		e = e.Str("content", content)
	}
	e.Msg("agent.message")
}

// AgentPrompt logs the system prompt being used.
func AgentPrompt(phase string, promptLen int) {
	log.Debug().
		Str("component", "agent").
		Str("phase", phase).
		Int("prompt_len", promptLen).
		Msg("agent.prompt")
}

// AgentTUI logs TUI state transitions.
func AgentTUI(event string, details map[string]string) {
	e := log.Debug().Str("component", "tui").Str("event", event)
	for k, v := range details {
		e = e.Str(k, v)
	}
	e.Msg("tui." + event)
}
