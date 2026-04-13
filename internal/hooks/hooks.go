// Package hooks provides pre/post tool hook execution.
// Hooks are shell commands triggered by tool events. They run synchronously
// and can block or modify tool execution.
//
// Configuration: .haft/hooks.yaml or ~/.haft/hooks.yaml
package hooks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Trigger identifies when a hook fires.
type Trigger string

const (
	TriggerPreTool     Trigger = "pre_tool"     // before tool execution
	TriggerPostTool    Trigger = "post_tool"    // after tool execution
	TriggerPreSubmit   Trigger = "pre_submit"   // before user message is sent to LLM
	TriggerPostSession Trigger = "post_session" // when session ends
)

// HookDef is a single hook definition from config.
type HookDef struct {
	Name    string  `yaml:"name"`
	Trigger Trigger `yaml:"trigger"`
	Command string  `yaml:"command"` // shell command
	// Filters (optional): only fire for matching tools/patterns
	ToolMatch string `yaml:"tool_match,omitempty"` // glob pattern for tool name (e.g., "bash", "edit*")
	Timeout   int    `yaml:"timeout_ms,omitempty"` // max execution time in ms (default: 5000)
}

// HookResult is the output of a hook execution.
type HookResult struct {
	Name     string
	Stdout   string
	Stderr   string
	ExitCode int
	Blocked  bool   // if true, the hook wants to block the action
	Message  string // reason for blocking (from stdout)
	Duration time.Duration
}

// Config holds all hook definitions.
type Config struct {
	Hooks []HookDef `yaml:"hooks"`
}

// Executor loads and runs hooks.
type Executor struct {
	hooks       []HookDef
	projectRoot string
}

// NewExecutor creates a hook executor from config files.
// Loads from .haft/hooks.yaml (project) and ~/.haft/hooks.yaml (global).
// Project hooks take precedence.
func NewExecutor(projectRoot string) *Executor {
	e := &Executor{projectRoot: projectRoot}
	e.loadConfig()
	return e
}

func (e *Executor) loadConfig() {
	// Project-level hooks
	projectPath := filepath.Join(e.projectRoot, ".haft", "hooks.yaml")
	e.loadFile(projectPath)

	// Global hooks
	home, err := os.UserHomeDir()
	if err == nil {
		globalPath := filepath.Join(home, ".haft", "hooks.yaml")
		e.loadFile(globalPath)
	}
}

func (e *Executor) loadFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return
	}
	e.hooks = append(e.hooks, cfg.Hooks...)
}

// HasHooks returns true if any hooks are configured.
func (e *Executor) HasHooks() bool {
	return len(e.hooks) > 0
}

// Run executes all hooks matching the given trigger and tool name.
// Returns results for each hook that ran.
// If any hook returns exit code 1, it's treated as a block request.
func (e *Executor) Run(ctx context.Context, trigger Trigger, env HookEnv) []HookResult {
	matching := e.match(trigger, env.ToolName)
	if len(matching) == 0 {
		return nil
	}

	var results []HookResult
	for _, hook := range matching {
		result := e.execute(ctx, hook, env)
		results = append(results, result)
		// Stop on block
		if result.Blocked {
			break
		}
	}
	return results
}

// HookEnv carries context passed to hook commands as environment variables.
type HookEnv struct {
	ToolName   string
	ToolArgs   string
	ToolOutput string // only for post_tool
	SessionID  string
	UserText   string // only for pre_submit
}

func (e *Executor) match(trigger Trigger, toolName string) []HookDef {
	var matched []HookDef
	for _, h := range e.hooks {
		if h.Trigger != trigger {
			continue
		}
		if h.ToolMatch != "" && toolName != "" {
			ok, _ := filepath.Match(h.ToolMatch, toolName)
			if !ok {
				continue
			}
		}
		matched = append(matched, h)
	}
	return matched
}

func (e *Executor) execute(ctx context.Context, hook HookDef, env HookEnv) HookResult {
	timeout := time.Duration(hook.Timeout) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "sh", "-c", hook.Command)
	cmd.Dir = e.projectRoot

	// Pass context as env vars
	cmd.Env = append(os.Environ(),
		"HAFT_HOOK_NAME="+hook.Name,
		"HAFT_HOOK_TRIGGER="+string(hook.Trigger),
		"HAFT_TOOL_NAME="+env.ToolName,
		"HAFT_TOOL_ARGS="+env.ToolArgs,
		"HAFT_TOOL_OUTPUT="+env.ToolOutput,
		"HAFT_SESSION_ID="+env.SessionID,
		"HAFT_USER_TEXT="+env.UserText,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	result := HookResult{
		Name:     hook.Name,
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: exitCode,
		Duration: duration,
	}

	// Exit code 1 = block request (stdout is the reason)
	if exitCode == 1 {
		result.Blocked = true
		result.Message = result.Stdout
		if result.Message == "" {
			result.Message = fmt.Sprintf("Hook '%s' blocked the action", hook.Name)
		}
	}

	return result
}
