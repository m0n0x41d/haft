// Package provider — Claude Code CLI integration.
//
// This provider wraps the `claude` CLI (Claude Code) as a subprocess so users
// with a Claude Pro/Max subscription can drive haft's interactive agent
// without a separate ANTHROPIC_API_KEY. Auth is delegated entirely to the
// CLI (OAuth, keychain, API key — whichever Claude Code is configured with).
//
// MVP scope (this file):
//   - Flattens haft's structured message history into a single prompt.
//   - Streams assistant text via `claude -p --output-format stream-json`.
//   - Does NOT translate haft's tool schemas into CLI tools. The agent loop
//     will get text responses only; haft's artifact tools (haft_note, etc.)
//     are not invokable by the model through this provider yet.
//
// Future work (explicitly out of scope for the first cut):
//   - Expose haft's tools via --mcp-config so the CLI can call them.
//   - Parse tool_use / tool_result events from stream-json.
//   - Session reuse via --resume for multi-turn efficiency.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/agent"
)

const claudeCodeBinary = "claude"

// ClaudeCodeProvider invokes the `claude` CLI per turn.
type ClaudeCodeProvider struct {
	modelID string // haft model id (reported to agent loop)
	subModel string // optional --model override passed to the CLI
	cliPath  string
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

	sub := ""
	if rest, ok := strings.CutPrefix(modelID, "claude-code:"); ok {
		sub = rest
	}

	return &ClaudeCodeProvider{
		modelID:  modelID,
		subModel: sub,
		cliPath:  path,
	}, nil
}

// ModelID returns the haft-facing model identifier.
func (p *ClaudeCodeProvider) ModelID() string { return p.modelID }

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
		"--allowed-tools", "",      // disable built-in tools; agent surface is haft's, not CLI's
		"--no-session-persistence", // ephemeral turn
		"--input-format", "text",
	}
	if p.subModel != "" {
		args = append(args, "--model", p.subModel)
	}
	if system != "" {
		args = append(args, "--append-system-prompt", system)
	}

	cmd := exec.CommandContext(ctx, p.cliPath, args...)
	cmd.Stdin = strings.NewReader(prompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("claudecode: stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("claudecode: start %s: %w", p.cliPath, err)
	}

	text, finishReason, parseErr := parseClaudeStream(stdout, handler)
	waitErr := cmd.Wait()

	if parseErr != nil {
		return nil, fmt.Errorf("claudecode: parse stream: %w", parseErr)
	}
	if waitErr != nil {
		stderrTxt := strings.TrimSpace(stderrBuf.String())
		if stderrTxt != "" {
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
