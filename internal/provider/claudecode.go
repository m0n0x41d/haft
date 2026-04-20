// Package provider — Claude Code CLI integration.
//
// This provider wraps the `claude` CLI (Claude Code) as a subprocess so users
// with a Claude Pro/Max subscription can drive haft's interactive agent
// without a separate ANTHROPIC_API_KEY. Auth is delegated entirely to the
// CLI (OAuth, keychain, API key — whichever Claude Code is configured with).
//
// Scope:
//   - Flattens haft's structured message history into a single prompt.
//   - Streams assistant text via `claude -p --output-format stream-json`.
//   - Wires haft's own MCP server (`haft serve`) into the CLI via
//     `--mcp-config` so the model can call `haft_note`, `haft_problem`,
//     `haft_decision`, `haft_query`, etc. Tool execution happens entirely
//     inside the CLI subprocess — haft's outer agent loop receives the final
//     assistant text after all tool round-trips have completed.
//
// Caveats:
//   - The CLI's built-in tools (Read/Write/Bash/etc.) are allowed by default
//     under `bypassPermissions`. Haft's own per-tool hooks and permission
//     model do not run for this provider. If that matters, use the
//     `anthropic` or `openai` providers whose tools go through haft's loop.
//   - Set `HAFT_CLAUDECODE_NO_MCP=1` to disable the MCP bridge and run the
//     CLI in text-only mode (no tool-use at all).
//
// Future work:
//   - Session reuse via --resume to amortize per-turn spawn cost.
//   - Propagate CLI token-accounting into `Message.Tokens`.
package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/project"
)

const claudeCodeBinary = "claude"

// ClaudeCodeProvider invokes the `claude` CLI per turn.
//
// Paths that don't change during the process lifetime (haft binary, detected
// project root, filtered child env) are resolved once at construction and
// cached so every turn doesn't re-walk the filesystem or re-copy the env.
type ClaudeCodeProvider struct {
	modelID     string   // haft-facing model id (reported to the agent loop)
	cliPath     string   // resolved `claude` binary
	haftExe     string   // resolved current `haft` binary (for --mcp-config)
	projectRoot string   // detected .haft/ root; "" when not in a haft project
	childEnv    []string // cached env with ANTHROPIC_API_KEY stripped
}

var _ LLMProvider = (*ClaudeCodeProvider)(nil)

// NewClaudeCode returns a provider that shells out to `claude`.
//
// modelID is the haft-facing model identifier. A suffix after "claude-code:"
// is forwarded to the CLI as --model (e.g. "claude-code:sonnet" → --model sonnet).
// The bare "claude-code" uses whatever model the CLI picks by default.
func NewClaudeCode(modelID string) (*ClaudeCodeProvider, error) {
	path, err := exec.LookPath(claudeCodeBinary)
	if err != nil {
		return nil, fmt.Errorf(
			"claude CLI not found in PATH: install Claude Code (https://docs.claude.com/en/docs/claude-code) " +
				"and sign in, or pick a different model",
		)
	}

	// os.Executable / project detection may fail (test binaries, no cwd, no
	// .haft/). Failing here would make the provider unusable in text-only
	// contexts, so we downgrade to "MCP bridge off" by leaving fields empty.
	haftExe, _ := os.Executable()
	projectRoot, _ := project.FindRootFromCwd()

	return &ClaudeCodeProvider{
		modelID:     modelID,
		cliPath:     path,
		haftExe:     haftExe,
		projectRoot: projectRoot,
		// Snapshot the env at construction. ANTHROPIC_API_KEY is the only
		// var we actively mask; users rarely toggle it mid-session, and the
		// subprocess we spawn is short-lived per turn, so a static snapshot
		// is fine. Any env var *added* after construction will be missed —
		// callers needing fresh env should rebuild the provider.
		childEnv: envWithout(os.Environ(), "ANTHROPIC_API_KEY"),
	}, nil
}

// ModelID returns the haft-facing model identifier.
func (p *ClaudeCodeProvider) ModelID() string { return p.modelID }

// cliSubModel returns the optional --model override forwarded to the CLI.
// Derived from modelID, not stored — one source of truth.
func (p *ClaudeCodeProvider) cliSubModel() string {
	rest, ok := strings.CutPrefix(p.modelID, "claude-code:")
	if !ok {
		return ""
	}
	return rest
}

// Stream sends the conversation to the CLI and emits text deltas.
//
// Tool schemas are currently ignored (see package docs). If the caller
// passes tools, the provider still succeeds but the model has no way
// to invoke them — it will respond with text only.
func (p *ClaudeCodeProvider) Stream(
	ctx context.Context,
	messages []agent.Message,
	_ []agent.ToolSchema,
	handler func(StreamDelta),
) (*agent.Message, error) {
	system, prompt := flattenConversation(messages)

	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",                // required by CLI when using stream-json
		"--no-session-persistence", // ephemeral turn
		"--input-format", "text",
	}

	// Wire haft's MCP server into the CLI so the model can call haft_*
	// artifact tools. Opt out with HAFT_CLAUDECODE_NO_MCP=1 for text-only.
	mcpBridged := false
	if p.canBridgeMCP() {
		cfgPath, err := writeHaftMCPConfig(p.haftExe, p.projectRoot)
		if err == nil && cfgPath != "" {
			defer func() { _ = os.Remove(cfgPath) }()
			args = append(args,
				"--mcp-config", cfgPath,
				"--permission-mode", "bypassPermissions",
				// Use equals form so a project root starting with "-"
				// can't be interpreted as a CLI flag.
				"--add-dir="+p.projectRoot,
			)
			mcpBridged = true
		}
	}
	if !mcpBridged {
		// Text-only fallback: disable built-ins so the model can't
		// write files when haft's own surface isn't bridged in.
		args = append(args, "--allowed-tools", "")
	}

	if sub := p.cliSubModel(); sub != "" {
		args = append(args, "--model", sub)
	}
	if system != "" {
		// Replace, don't append. Claude Code's default system prompt is ~30K
		// tokens and would drown haft's FPF protocol instructions. Haft
		// owns the prompt for this provider; if it wants CLI tool usage
		// described, it can include that itself.
		args = append(args, "--system-prompt", system)
	}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.Stdin = strings.NewReader(prompt)
	// Child env has ANTHROPIC_API_KEY stripped so Claude Code falls back to
	// its OAuth credentials. Max/Pro subscribers who happen to have an API
	// key exported would otherwise be silently billed per-token instead of
	// drawing from their subscription. See
	// https://github.com/anthropics/claude-code/issues/43333 (fixed Apr 2026).
	cmd.Env = p.childEnv

	// Cap stderr at 64KB so a chatty --verbose session can't blow up the
	// parent's memory. We only need the tail to surface in error messages.
	stderrBuf := &cappedBuffer{limit: 64 * 1024}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("claudecode: stdout pipe: %w", err)
	}
	cmd.Stderr = stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("claudecode: start %s: %w", p.cliPath, err)
	}

	text, finishReason, parseErr := parseClaudeStream(stdout, handler)
	waitErr := cmd.Wait()

	if parseErr != nil {
		return nil, fmt.Errorf("claudecode: parse stream: %w", parseErr)
	}
	if waitErr != nil {
		// stderrBuf already caps at 64KB with a tail-preserving ring.
		if stderrTxt := strings.TrimSpace(stderrBuf.String()); stderrTxt != "" {
			return nil, fmt.Errorf("claudecode: cli exited: %w: %s", waitErr, stderrTxt)
		}
		return nil, fmt.Errorf("claudecode: cli exited: %w", waitErr)
	}

	if finishReason == "" {
		finishReason = "stop"
	}
	handler(StreamDelta{Done: true, FinishReason: finishReason})

	msg := &agent.Message{
		Role:      agent.RoleAssistant,
		Model:     p.modelID,
		CreatedAt: time.Now().UTC(),
	}
	if text != "" {
		msg.Parts = append(msg.Parts, agent.TextPart{Text: text})
	}
	return msg, nil
}

// flattenConversation compresses haft's structured messages into the single
// system + user prompt pair the CLI expects.
//
//   - All RoleSystem messages are joined as the system prompt.
//   - User / assistant / tool turns are rendered as labeled blocks so the
//     model sees the full transcript, including prior tool calls that this
//     provider can't reproduce natively.
func flattenConversation(messages []agent.Message) (string, string) {
	var sys strings.Builder
	var body strings.Builder

	writeBlock := func(label, content string) {
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}
		if body.Len() > 0 {
			body.WriteString("\n\n")
		}
		body.WriteString(label)
		body.WriteString(": ")
		body.WriteString(content)
	}

	for _, m := range messages {
		switch m.Role {
		case agent.RoleSystem:
			if s := strings.TrimSpace(m.Text()); s != "" {
				if sys.Len() > 0 {
					sys.WriteString("\n\n")
				}
				sys.WriteString(s)
			}
		case agent.RoleUser:
			writeBlock("User", renderParts(m.Parts))
		case agent.RoleAssistant:
			writeBlock("Assistant", renderParts(m.Parts))
		case agent.RoleTool:
			writeBlock("Tool", renderParts(m.Parts))
		}
	}
	return sys.String(), body.String()
}

func renderParts(parts []agent.Part) string {
	var b strings.Builder
	for _, p := range parts {
		switch v := p.(type) {
		case agent.TextPart:
			b.WriteString(v.Text)
		case agent.ToolCallPart:
			fmt.Fprintf(&b, "\n[tool_call name=%s id=%s]\n%s\n[/tool_call]\n",
				v.ToolName, v.ToolCallID, v.Arguments)
		case agent.ToolResultPart:
			prefix := "tool_result"
			if v.IsError {
				prefix = "tool_result_error"
			}
			fmt.Fprintf(&b, "\n[%s id=%s]\n%s\n[/%s]\n",
				prefix, v.ToolCallID, v.Content, prefix)
		}
	}
	return b.String()
}

// canBridgeMCP reports whether the environment and provider state support
// routing haft's MCP server into the CLI. Off when opted out explicitly,
// when no haft binary was resolvable at construction, or when the provider
// wasn't constructed inside a haft project.
func (p *ClaudeCodeProvider) canBridgeMCP() bool {
	if os.Getenv("HAFT_CLAUDECODE_NO_MCP") != "" {
		return false
	}
	return p.haftExe != "" && p.projectRoot != ""
}

// writeHaftMCPConfig generates a tmpfile containing an --mcp-config payload
// that exposes haft's own MCP server (via `haft serve`) to the `claude` CLI
// subprocess. The caller is responsible for removing the returned path.
func writeHaftMCPConfig(haftExe, projectRoot string) (string, error) {
	type mcpEntry struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	type mcpConfig struct {
		McpServers map[string]mcpEntry `json:"mcpServers"`
	}
	cfg := mcpConfig{McpServers: map[string]mcpEntry{
		"haft": {
			Command: haftExe,
			Args:    []string{"serve"},
			Env:     map[string]string{"QUINT_PROJECT_ROOT": projectRoot},
		},
	}}

	data, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}

	f, err := os.CreateTemp("", "haft-mcp-*.json")
	if err != nil {
		return "", err
	}
	// Defense-in-depth: CreateTemp uses 0600 on Unix with a normal umask,
	// but a permissive umask (0000) would leave the file world-readable.
	// The tmpfile embeds the haft binary path and project root, so lock
	// it down explicitly.
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// cappedBuffer is an io.Writer that retains the *last* `limit` bytes written.
// Used for subprocess stderr so a chatty child can't pressure the parent's
// memory. Keeping the tail (not the head) matters because real failures
// almost always print near the end of a run — startup chatter is the
// discardable prefix, the error message is the useful suffix.
type cappedBuffer struct {
	limit     int
	buf       []byte
	truncated bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if n >= c.limit {
		c.buf = append(c.buf[:0], p[n-c.limit:]...)
		c.truncated = true
		return n, nil
	}
	if len(c.buf)+n > c.limit {
		drop := len(c.buf) + n - c.limit
		c.buf = c.buf[drop:]
		c.truncated = true
	}
	c.buf = append(c.buf, p...)
	return n, nil
}

func (c *cappedBuffer) String() string {
	if c.truncated {
		return "…(truncated)" + string(c.buf)
	}
	return string(c.buf)
}

// envWithout returns env with any entries matching key=... removed. The
// match is case-sensitive and anchored at "=" so keys that merely share
// a prefix are left untouched.
func envWithout(env []string, key string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// streamEvent captures the fields we care about from stream-json NDJSON.
// See: https://docs.claude.com/en/docs/claude-code/sdk
type streamEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`
	Message *struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content"`
	} `json:"message,omitempty"`
	IsError bool `json:"is_error,omitempty"`
}

// parseClaudeStream reads NDJSON events from the CLI's stdout and forwards
// text deltas to handler. Returns the concatenated text, finish reason, and
// any scanner error. Unknown event types are ignored (forward-compat).
func parseClaudeStream(r io.Reader, handler func(StreamDelta)) (string, string, error) {
	var buf strings.Builder
	// 16KB covers the typical single-turn text response (few-KB messages plus
	// thinking). Avoids ~4 rounds of grow-and-copy doubling for common cases;
	// grows naturally for longer outputs.
	buf.Grow(16 * 1024)
	var finishReason string

	scanner := bufio.NewScanner(r)
	// Allow large events — a single assistant block can exceed the 64KB default.
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev streamEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// Malformed event — skip rather than abort. The CLI occasionally
			// interleaves debug lines when --verbose is on.
			continue
		}

		switch ev.Type {
		case "assistant":
			if ev.Message == nil {
				continue
			}
			for _, block := range ev.Message.Content {
				if block.Type == "text" && block.Text != "" {
					buf.WriteString(block.Text)
					handler(StreamDelta{Text: block.Text})
				}
			}
		case "result":
			// subtype is "success" or "error_*"; map to haft's finish reasons.
			if ev.IsError || strings.HasPrefix(ev.Subtype, "error") {
				finishReason = "error"
			} else {
				finishReason = "stop"
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return buf.String(), finishReason, err
	}
	return buf.String(), finishReason, nil
}
