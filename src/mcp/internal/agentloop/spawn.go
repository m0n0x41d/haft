package agentloop

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/m0n0x41d/quint-code/internal/agent"
	"github.com/m0n0x41d/quint-code/internal/provider"
	"github.com/m0n0x41d/quint-code/internal/tui"
	"github.com/m0n0x41d/quint-code/logger"
)

// SubagentHandle tracks a running subagent goroutine.
type SubagentHandle struct {
	ID        string
	Name      string
	SessionID string
	Cancel    context.CancelFunc
	Result    <-chan SubagentResult
	done      bool
}

// SubagentResult is the output of a completed subagent.
type SubagentResult struct {
	ID      string
	Summary string
	Error   error
	Tokens  int
}

// SubagentTracker manages active subagent handles. Thread-safe.
type SubagentTracker struct {
	mu      sync.Mutex
	handles map[string]*SubagentHandle
	count   int
}

func NewSubagentTracker() *SubagentTracker {
	return &SubagentTracker{handles: make(map[string]*SubagentHandle)}
}

func (t *SubagentTracker) Add(h *SubagentHandle) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handles[h.ID] = h
	t.count++
}

func (t *SubagentTracker) Get(id string) (*SubagentHandle, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	h, ok := t.handles[id]
	return h, ok
}

func (t *SubagentTracker) MarkDone(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if h, ok := t.handles[id]; ok {
		h.done = true
		t.count--
	}
}

func (t *SubagentTracker) ActiveCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.count
}

func (t *SubagentTracker) CancelAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, h := range t.handles {
		if !h.done {
			h.Cancel()
		}
	}
}

// SpawnSubagent creates a child session and runs a subagent in a goroutine.
// Returns the handle immediately — the subagent runs asynchronously.
func (c *Coordinator) SpawnSubagent(
	ctx context.Context,
	parentSess *agent.Session,
	def agent.SubagentDef,
	task string,
) (*SubagentHandle, error) {
	// Depth validation (pure L2)
	if err := agent.ValidateSpawnDepth(parentSess.ParentID); err != nil {
		return nil, err
	}

	// Concurrency limit
	if c.Subagents.ActiveCount() >= agent.MaxConcurrentSubagents {
		return nil, fmt.Errorf("max concurrent subagents reached (%d)", agent.MaxConcurrentSubagents)
	}

	subagentID := "sub_" + uuid.NewString()[:8]

	// Model: use override if specified, else parent's model
	model := parentSess.Model
	if def.Model != "" {
		model = def.Model
	}

	// Create child session
	childSess := &agent.Session{
		ID:        uuid.NewString(),
		ParentID:  parentSess.ID,
		Title:     def.Name + ": " + truncateTask(task, 40),
		Model:     model,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := c.Sessions.Create(ctx, childSess); err != nil {
		return nil, fmt.Errorf("create child session: %w", err)
	}

	// Build filtered tool registry for subagent
	childTools := c.buildSubagentTools(def)

	// Build system prompt
	systemPrompt := def.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a subagent. Complete the task and return a concise summary."
	}

	// Max steps
	maxSteps := def.MaxSteps
	if maxSteps <= 0 {
		maxSteps = agent.DefaultSubagentMaxSteps
	}

	// Child context derived from parent — Ctrl+C propagates
	childCtx, childCancel := context.WithCancel(ctx)

	resultCh := make(chan SubagentResult, 1)

	handle := &SubagentHandle{
		ID:        subagentID,
		Name:      def.Name,
		SessionID: childSess.ID,
		Cancel:    childCancel,
		Result:    resultCh,
	}
	c.Subagents.Add(handle)

	logger.Info().Str("component", "agent").
		Str("subagent_id", subagentID).
		Str("type", def.Name).
		Str("session_id", childSess.ID).
		Str("parent_id", parentSess.ID).
		Str("model", model).
		Msg("agent.subagent_spawned")

	// Notify TUI
	c.Bus.Send(tui.SubagentStartMsg{
		SubagentID: subagentID,
		Name:       def.Name,
		Task:       task,
	})

	// Launch goroutine
	go c.runSubagent(childCtx, childSess, subagentID, systemPrompt, task, childTools, maxSteps, resultCh)

	return handle, nil
}

// runSubagent executes the subagent's reactLoop in a goroutine.
func (c *Coordinator) runSubagent(
	ctx context.Context,
	sess *agent.Session,
	subagentID string,
	systemPrompt string,
	task string,
	toolSchemas []agent.ToolSchema,
	maxSteps int,
	resultCh chan<- SubagentResult,
) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error().Str("component", "agent").
				Str("subagent_id", subagentID).
				Interface("panic", r).
				Msg("agent.subagent_panic")
			resultCh <- SubagentResult{ID: subagentID, Error: fmt.Errorf("subagent panic: %v", r)}
		}
		c.Subagents.MarkDone(subagentID)
		c.Bus.Send(tui.SubagentDoneMsg{SubagentID: subagentID})
	}()

	// Build message history: system + user task
	systemMsg := agent.Message{
		Role:  agent.RoleSystem,
		Parts: []agent.Part{agent.TextPart{Text: systemPrompt}},
	}
	userMsg := &agent.Message{
		ID:        newMsgID(),
		SessionID: sess.ID,
		Role:      agent.RoleUser,
		Parts:     []agent.Part{agent.TextPart{Text: task}},
		CreatedAt: time.Now().UTC(),
	}
	_ = c.Messages.Save(ctx, userMsg)

	history := []agent.Message{systemMsg, *userMsg}

	// Run a simplified reactLoop — no phase transitions, no signals
	result := c.subagentLoop(ctx, sess, subagentID, history, toolSchemas, maxSteps)
	resultCh <- result
}

// subagentLoop is a stripped-down reactLoop for subagents.
// No phase transitions, no signal detection. Just tool execution.
func (c *Coordinator) subagentLoop(
	ctx context.Context,
	sess *agent.Session,
	subagentID string,
	history []agent.Message,
	toolSchemas []agent.ToolSchema,
	maxSteps int,
) SubagentResult {
	var totalTokens int

	for step := 0; step < maxSteps; step++ {
		if ctx.Err() != nil {
			return SubagentResult{ID: subagentID, Error: ctx.Err(), Tokens: totalTokens}
		}

		// LLM call — stream deltas are NOT sent to bus (subagent text is internal)
		assistantMsg, err := c.Provider.Stream(ctx, history, toolSchemas, func(delta provider.StreamDelta) {
			// Subagent streaming text is not shown — only tool calls are visible
		})
		if err != nil {
			return SubagentResult{ID: subagentID, Error: err, Tokens: totalTokens}
		}

		assistantMsg.ID = newMsgID()
		assistantMsg.SessionID = sess.ID
		_ = c.Messages.Save(ctx, assistantMsg)
		totalTokens += assistantMsg.Tokens

		toolCalls := assistantMsg.ToolCalls()

		// No tool calls — done. Return the assistant's text as summary.
		if len(toolCalls) == 0 {
			return SubagentResult{
				ID:      subagentID,
				Summary: assistantMsg.Text(),
				Tokens:  totalTokens,
			}
		}

		history = append(history, *assistantMsg)

		// Execute tools — send tagged events to bus for nested TUI rendering
		for _, tc := range toolCalls {
			if ctx.Err() != nil {
				return SubagentResult{ID: subagentID, Error: ctx.Err(), Tokens: totalTokens}
			}

			// Send tagged ToolStartMsg (TUI renders nested under spawn)
			c.Bus.Send(tui.ToolStartMsg{
				ToolCallID: tc.ToolCallID,
				ToolName:   tc.ToolName,
				Args:       tc.Arguments,
				SubagentID: subagentID,
			})

			// Execute (subagent tools are pre-approved — no permission prompts)
			output, isError := c.executeSubagentTool(ctx, tc)

			// Truncate
			const maxBytes = 50_000
			if len(output) > maxBytes {
				output = output[:maxBytes] + "\n... (truncated)"
			}

			c.Bus.Send(tui.ToolDoneMsg{
				ToolCallID: tc.ToolCallID,
				ToolName:   tc.ToolName,
				Output:     output,
				IsError:    isError,
				SubagentID: subagentID,
			})

			// Save tool result
			msg := &agent.Message{
				ID: newMsgID(), SessionID: sess.ID, Role: agent.RoleTool,
				Parts: []agent.Part{agent.ToolResultPart{
					ToolCallID: tc.ToolCallID, ToolName: tc.ToolName,
					Content: output, IsError: isError,
				}},
				CreatedAt: time.Now().UTC(),
			}
			_ = c.Messages.Save(ctx, msg)
			history = append(history, *msg)
		}
	}

	// Max steps reached
	return SubagentResult{
		ID:      subagentID,
		Summary: "(subagent reached step limit)",
		Tokens:  totalTokens,
	}
}

func (c *Coordinator) executeSubagentTool(ctx context.Context, tc agent.ToolCallPart) (string, bool) {
	result, err := c.Tools.Execute(ctx, tc.ToolName, tc.Arguments)
	if err != nil {
		return fmt.Sprintf("Tool error: %s", err.Error()), true
	}
	return result.DisplayText, false
}

// WaitSubagents blocks until all specified subagents complete or timeout.
// Returns a map of subagentID → summary.
func (c *Coordinator) WaitSubagents(ctx context.Context, ids []string, timeoutMs int) (map[string]SubagentResult, error) {
	if timeoutMs <= 0 {
		timeoutMs = 120_000 // 2 minutes default
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	deadline := time.After(timeout)

	results := make(map[string]SubagentResult, len(ids))

	for _, id := range ids {
		handle, ok := c.Subagents.Get(id)
		if !ok {
			results[id] = SubagentResult{ID: id, Error: fmt.Errorf("unknown subagent: %s", id)}
			continue
		}

		select {
		case result := <-handle.Result:
			results[id] = result
			logger.Info().Str("component", "agent").
				Str("subagent_id", id).
				Int("tokens", result.Tokens).
				Bool("error", result.Error != nil).
				Msg("agent.subagent_completed")

			// Notify TUI with summary
			summary := result.Summary
			isError := false
			if result.Error != nil {
				summary = result.Error.Error()
				isError = true
			}
			c.Bus.Send(tui.SubagentDoneMsg{
				SubagentID: id,
				Summary:    summary,
				IsError:    isError,
			})

		case <-deadline:
			handle.Cancel()
			results[id] = SubagentResult{ID: id, Error: fmt.Errorf("timeout after %dms", timeoutMs)}

		case <-ctx.Done():
			return results, ctx.Err()
		}
	}

	return results, nil
}

// buildSubagentTools creates a filtered tool schema list for the subagent.
func (c *Coordinator) buildSubagentTools(def agent.SubagentDef) []agent.ToolSchema {
	if len(def.AllowedTools) == 0 && !def.ReadOnly {
		return c.Tools.List()
	}

	allTools := c.Tools.List()
	allowed := make(map[string]bool, len(def.AllowedTools))
	for _, name := range def.AllowedTools {
		allowed[name] = true
	}

	var filtered []agent.ToolSchema
	for _, tool := range allTools {
		if len(def.AllowedTools) > 0 && !allowed[tool.Name] {
			continue
		}
		if def.ReadOnly && (tool.Name == "write" || tool.Name == "edit") {
			continue
		}
		// Never give subagents spawn/wait tools (prevents recursion at tool level)
		if tool.Name == "spawn_agent" || tool.Name == "wait_agent" {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

func truncateTask(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// FormatWaitResults formats wait results into a string for the LLM.
func FormatWaitResults(results map[string]SubagentResult) string {
	var b strings.Builder
	for id, r := range results {
		b.WriteString(fmt.Sprintf("## Agent %s\n", id))
		if r.Error != nil {
			b.WriteString(fmt.Sprintf("Error: %s\n", r.Error))
		} else {
			b.WriteString(r.Summary)
		}
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}
