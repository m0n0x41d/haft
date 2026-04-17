package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ANSI escape sequences for terminal formatting.
const (
	aBold    = "\033[1m"
	aDim     = "\033[2m"
	aReset   = "\033[0m"
	aRed     = "\033[31m"
	aGreen   = "\033[32m"
	aYellow  = "\033[33m"
	aBlue    = "\033[34m"
	aMagenta = "\033[35m"
	aCyan    = "\033[36m"
)

// plainRenderer consumes RunEvents from a channel and prints formatted plain
// text to stdout using ANSI formatting. This is the --no-tui / piped-output
// renderer that produces output semantically identical to the original haft run.
type plainRenderer struct{}

// Run loops over events from the channel and prints each one.
// Blocks until the channel is closed.
func (r *plainRenderer) Run(events <-chan RunEvent) {
	isFirst := true
	for e := range events {
		switch {
		case e.PhaseBegan != nil:
			if isFirst {
				fmt.Println(aCyan + strings.Repeat("━", 52) + aReset)
				fmt.Printf("  %s%s%s\n", aBold, e.PhaseBegan.Name, aReset)
				fmt.Println(aCyan + strings.Repeat("━", 52) + aReset)
				isFirst = false
			} else {
				fmt.Printf("\n  %s⟳ %s%s\n", aCyan, e.PhaseBegan.Name, aReset)
				fmt.Printf("  %s──────────────────────────%s\n", aDim, aReset)
			}
		case e.MetaInfo != nil:
			fmt.Printf("  %s%-14s%s %s\n", aDim, e.MetaInfo.Label, aReset, e.MetaInfo.Value)
		case e.TaskStatusChanged != nil:
			t := e.TaskStatusChanged
			switch t.Status {
			case TaskPending, TaskRunning:
				// silent — phase event already announced
			case TaskPassed:
				fmt.Printf("  %s✓%s Task %s done (%ds)\n", aGreen, aReset, t.TaskID, int(t.Elapsed.Seconds()))
			case TaskFailed:
				msg := fmt.Sprintf("Task %s failed", t.TaskID)
				if t.Detail != "" {
					msg += ": " + t.Detail
				}
				fmt.Printf("  %s✗%s %s\n", aRed, aReset, msg)
			case TaskSkipped:
				fmt.Printf("  %s–%s Task %s skipped\n", aDim, aReset, t.TaskID)
			}
		case e.AgentChunk != nil:
			// In plain-text mode, agent output goes directly to stdout via spawnAgent.
			// AgentChunk events are consumed by the TUI model when active.
		case e.BuildResult != nil:
			if e.BuildResult.OK {
				fmt.Printf("  %s✓%s %s passed\n", aGreen, aReset, e.BuildResult.Command)
			} else {
				fmt.Printf("  %s✗%s %s failed\n", aRed, aReset, e.BuildResult.Command)
			}
		case e.InvariantResult != nil:
			inv := e.InvariantResult
			icon := aGreen + "✓" + aReset
			if !inv.Pass {
				icon = aRed + "✗" + aReset
			}
			fmt.Printf("  %s %s[%s]%s %s\n", icon, aDim, inv.Source, aReset, inv.Text)
			if !inv.Pass && inv.Reason != "" {
				fmt.Printf("       %s%s%s\n", aRed, inv.Reason, aReset)
			}
		case e.PlanLoaded != nil:
			pl := e.PlanLoaded
			fmt.Printf("  %s%d tasks planned%s\n", aDim, pl.TaskCount, aReset)
			for _, t := range pl.Tasks {
				fmt.Printf("  %s%-4s%s %s\n", aCyan, t.ID, aReset, t.Title)
			}
			fmt.Println()
		case e.Summary != nil:
			switch e.Summary.Level {
			case StatusOK:
				fmt.Printf("  %s✓%s %s\n", aGreen, aReset, e.Summary.Message)
			case StatusWarn:
				fmt.Printf("  %s⚠%s %s\n", aYellow, aReset, e.Summary.Message)
			case StatusFail:
				fmt.Printf("  %s✗%s %s\n", aRed, aReset, e.Summary.Message)
			}
		case e.PipelineDone != nil:
			fmt.Println()
			fmt.Println(aCyan + strings.Repeat("━", 52) + aReset)
			fmt.Printf("  %sDuration: %ds%s\n", aDim, int(e.PipelineDone.Elapsed.Seconds()), aReset)
		}
	}
}

// spawnAgent launches an agent subprocess. When no writer is provided (or nil),
// stdout pipes to os.Stdout (plain-text mode). When a writer is provided, stdout
// pipes to it and claude receives --output-format stream-json for JSONL capture.
func spawnAgent(agent, prompt, projectRoot string, capture ...io.Writer) error {
	var w io.Writer
	if len(capture) > 0 {
		w = capture[0]
	}

	var cmd *exec.Cmd
	switch agent {
	case "codex":
		cmd = exec.Command("codex", "exec", "--full-auto", "-c", "mcp_servers={}", "-")
	case "claude":
		args := []string{"-p", prompt, "--allowedTools", "Edit,Write,Bash,Read,Glob,Grep"}
		if w != nil {
			args = append(args, "--output-format", "stream-json")
		}
		cmd = exec.Command("claude", args...)
	default:
		return fmt.Errorf("unknown agent: %s", agent)
	}

	cmd.Dir = projectRoot
	if w != nil {
		cmd.Stdout = w
	} else {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr

	if agent == "codex" {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		_, _ = stdin.Write([]byte(prompt))
		_ = stdin.Close()
		return cmd.Wait()
	}

	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// JSONL stream parsing — claude stream-json → AgentChunk events
// ---------------------------------------------------------------------------

// claudeEnvelope is the top-level JSON object in claude's stream-json output.
type claudeEnvelope struct {
	Type    string             `json:"type"`
	Message claudeMessageField `json:"message"`
	Result  json.RawMessage    `json:"result"`
	IsError bool               `json:"is_error"`
	Error   *claudeErrorField  `json:"error"`
}

type claudeMessageField struct {
	Content json.RawMessage `json:"content"`
}

type claudeErrorField struct {
	Message string `json:"message"`
}

type claudeContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Thinking string          `json:"thinking"`
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
}

// parseAgentJSONL reads claude JSONL output line-by-line from r and emits typed
// AgentChunk events via send. For codex output (non-JSON lines), emits ChunkRaw.
// Blocks until r is closed or returns an error. Always emits AgentDone on exit.
func parseAgentJSONL(r io.Reader, send *EventSender) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var env claudeEnvelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			send.Agent(ChunkRaw, line)
			continue
		}

		switch env.Type {
		case "system", "rate_limit_event":
			// noise — skip
		case "assistant", "user", "message":
			emitContentBlocks(env.Message.Content, send)
		case "result":
			emitResultBlocks(env, send)
		case "error":
			msg := line
			if env.Error != nil && env.Error.Message != "" {
				msg = env.Error.Message
			}
			send.Agent(ChunkRaw, msg)
		default:
			send.Agent(ChunkRaw, line)
		}
	}

	send.AgentDone()
}

// emitContentBlocks parses a claude message.content field (array of content blocks
// or a plain string) and emits the appropriate AgentChunk events.
func emitContentBlocks(raw json.RawMessage, send *EventSender) {
	if len(raw) == 0 {
		return
	}

	// Try as plain string first.
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if text != "" {
			send.Agent(ChunkText, text)
		}
		return
	}

	// Parse as array of content blocks.
	var blocks []claudeContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		send.Agent(ChunkRaw, string(raw))
		return
	}

	for _, block := range blocks {
		switch block.Type {
		case "thinking", "redacted_thinking":
			t := block.Thinking
			if t == "" {
				t = block.Text
			}
			if t == "" && block.Type == "redacted_thinking" {
				t = "[redacted]"
			}
			if t != "" {
				send.Agent(ChunkThinking, t)
			}
		case "tool_use":
			send.AgentTool(block.Name, truncateToolArgs(block.Input))
		case "text":
			if block.Text != "" {
				send.Agent(ChunkText, block.Text)
			}
		default:
			if block.Text != "" {
				send.Agent(ChunkRaw, block.Text)
			}
		}
	}
}

// emitResultBlocks handles the "result" envelope type — the final agent response.
func emitResultBlocks(env claudeEnvelope, send *EventSender) {
	if len(env.Result) == 0 {
		return
	}

	// Result can be a plain string or structured content.
	var text string
	if err := json.Unmarshal(env.Result, &text); err == nil {
		if text != "" {
			send.Agent(ChunkText, text)
		}
		return
	}

	// Otherwise treat as raw.
	s := strings.TrimSpace(string(env.Result))
	if s != "" {
		send.Agent(ChunkRaw, s)
	}
}

// truncateToolArgs returns a compact string representation of tool input JSON,
// truncated to 120 characters for display.
func truncateToolArgs(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	s := string(raw)
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}

func newShellCmd(command, dir string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func uniqueStrings(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
