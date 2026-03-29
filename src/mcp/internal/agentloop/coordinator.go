package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/m0n0x41d/quint-code/internal/agent"
	"github.com/m0n0x41d/quint-code/internal/artifact"
	"github.com/m0n0x41d/quint-code/internal/provider"
	"github.com/m0n0x41d/quint-code/internal/session"
	"github.com/m0n0x41d/quint-code/internal/tools"
	"github.com/m0n0x41d/quint-code/internal/tui"
	"github.com/m0n0x41d/quint-code/logger"
)

const (
	maxStepsPerTurn = 200 // hard cap per user turn
	loopWindowSize  = 8   // recent tool calls to check for loops
	loopMaxRepeats  = 3   // same tool+args repeated = loop
)

// Coordinator runs a single ReAct loop per user turn.
// v2: No phase machine. FPF enforced by tool guardrails.
// One unified prompt, all tools available. Cycle binding happens
// automatically when artifact tools return typed Meta.
type Coordinator struct {
	Provider      provider.LLMProvider
	Tools         *tools.Registry
	Sessions      session.SessionStore
	Messages      session.MessageStore
	Cycles        session.CycleStore
	ArtifactStore artifact.ArtifactStore
	Bus           *tui.Bus
	SystemPrompt  string
	AgentDef      agent.AgentDef
	Subagents     *SubagentTracker
	ProjectRoot   string // for drift detection
}

// Run executes one user turn: save message → react loop → done.
func (c *Coordinator) Run(ctx context.Context, sess *agent.Session, userText string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error().Str("component", "agent").Interface("panic", r).Msg("agent.panic")
			c.Bus.Send(tui.ErrorMsg{Err: fmt.Errorf("coordinator panic: %v", r)})
		}
		c.Bus.Send(tui.CoordinatorDoneMsg{})
	}()

	logger.AgentSession("user_turn", sess.ID, sess.Model)
	logger.AgentMessage("user", userText, 0, 0)

	// Wire cycle resolver so tools can check FPF guardrails
	if c.Cycles != nil {
		sessID := sess.ID
		c.Tools.SetCycleResolver(func(ctx context.Context) *agent.Cycle {
			cycle, _ := c.Cycles.GetActiveCycle(ctx, sessID)
			return cycle
		})

		// Restore active cycle state to TUI
		if cycle, err := c.Cycles.GetActiveCycle(ctx, sess.ID); err == nil && cycle != nil {
			c.sendCycleUpdate(cycle)
		}
	}

	// Save user message
	userMsg := &agent.Message{
		ID:        newMsgID(),
		SessionID: sess.ID,
		Role:      agent.RoleUser,
		Parts:     []agent.Part{agent.TextPart{Text: userText}},
		CreatedAt: time.Now().UTC(),
	}
	if err := c.Messages.Save(ctx, userMsg); err != nil {
		c.Bus.Send(tui.ErrorMsg{Err: fmt.Errorf("save user message: %w", err)})
		return
	}

	history, err := c.Messages.ListBySession(ctx, sess.ID)
	if err != nil {
		c.Bus.Send(tui.ErrorMsg{Err: fmt.Errorf("load history: %w", err)})
		return
	}
	history = sanitizeHistory(history)

	// Title generation on first turn
	isFirstTurn := sess.Title == "" && len(history) <= 1
	firstUserText := ""
	if isFirstTurn {
		firstUserText = userMsg.Text()

		// Drift detection on session start
		if c.ArtifactStore != nil {
			c.injectDriftWarnings(ctx, &history)
		}
	}

	// Build system prompt: unified agent prompt + project context
	systemPrompt := c.SystemPrompt
	if c.AgentDef.SystemPrompt != "" {
		systemPrompt += "\n\n" + c.AgentDef.SystemPrompt
	}

	systemMsg := agent.Message{
		Role:  agent.RoleSystem,
		Parts: []agent.Part{agent.TextPart{Text: systemPrompt}},
	}
	fullHistory := append([]agent.Message{systemMsg}, history...)

	// Run the react loop — all tools available, no phase restrictions
	c.reactLoop(ctx, sess, fullHistory)

	// Async title generation
	if isFirstTurn && firstUserText != "" {
		go c.generateTitle(sess, firstUserText)
	}
}

// ---------------------------------------------------------------------------
// React loop — the core of the agent
// ---------------------------------------------------------------------------

func (c *Coordinator) reactLoop(
	ctx context.Context,
	sess *agent.Session,
	fullHistory []agent.Message,
) {
	var (
		toolCallHistory []agent.ToolCallRecord
		tokenBudget     = agent.NewTokenBudget(agent.ModelContextWindow(sess.Model))
		compacted       bool
	)

	maxToolCalls := maxStepsPerTurn
	if c.AgentDef.MaxToolCalls > 0 {
		maxToolCalls = c.AgentDef.MaxToolCalls
	}

	allTools := c.Tools.List()

	for step := 0; step < maxStepsPerTurn; step++ {
		if ctx.Err() != nil {
			return
		}

		// Loop detection
		if agent.DetectLoop(toolCallHistory, loopWindowSize, loopMaxRepeats) {
			logger.Warn().Str("component", "agent").Msg("agent.loop_detected")
			c.Bus.Send(tui.ErrorMsg{Err: fmt.Errorf("loop detected: agent is repeating the same tool calls")})
			return
		}

		// Token budget
		if tokenBudget.Exhausted() {
			logger.Warn().Str("component", "agent").Int("used", tokenBudget.Used).Msg("agent.tokens_exhausted")
			c.Bus.Send(tui.ErrorMsg{Err: fmt.Errorf("context window exhausted (%d/%d tokens)", tokenBudget.Used, tokenBudget.Limit)})
			return
		}

		// Compaction
		if !compacted {
			if newHistory, didCompact := c.compactContext(ctx, sess, fullHistory, tokenBudget); didCompact {
				fullHistory = newHistory
				compacted = true
			}
		}

		logger.AgentStep(step, "react", len(toolCallHistory), false)

		// LLM call (5 minute timeout)
		llmCtx, llmCancel := context.WithTimeout(ctx, 5*time.Minute)
		llmStart := time.Now()
		assistantMsg, err := c.Provider.Stream(llmCtx, fullHistory, allTools, func(delta provider.StreamDelta) {
			if delta.Text != "" {
				c.Bus.Send(tui.StreamDeltaMsg{Text: delta.Text})
			}
			if delta.Thinking != "" {
				c.Bus.Send(tui.ThinkingDeltaMsg{Text: delta.Thinking})
			}
		})
		llmCancel()

		if err != nil {
			logger.AgentError("react", err)
			if assistantMsg != nil {
				for _, tc := range assistantMsg.ToolCalls() {
					c.saveToolResult(ctx, sess, tc.ToolCallID, tc.ToolName,
						fmt.Sprintf("Tool call interrupted: %s", err.Error()), true, &fullHistory)
				}
			}
			c.Bus.Send(tui.ErrorMsg{Err: err})
			return
		}

		assistantMsg.ID = newMsgID()
		assistantMsg.SessionID = sess.ID
		_ = c.Messages.Save(ctx, assistantMsg)

		toolCalls := assistantMsg.ToolCalls()
		tokenBudget = tokenBudget.Add(assistantMsg.Tokens)
		c.Bus.Send(tui.TokenUpdateMsg{Used: tokenBudget.Used, Limit: tokenBudget.Limit})

		logger.Debug().Str("component", "agent").
			Int("step", step).
			Int("tool_calls", len(toolCalls)).
			Bool("has_text", assistantMsg.Text() != "").
			Int64("llm_ms", time.Since(llmStart).Milliseconds()).
			Int("tokens", assistantMsg.Tokens).
			Msg("agent.llm_response")

		// No tool calls → done
		if len(toolCalls) == 0 {
			logger.AgentMessage("assistant", assistantMsg.Text(), 0, assistantMsg.Tokens)
			c.Bus.Send(tui.StreamDoneMsg{Message: *assistantMsg})
			return
		}

		c.Bus.Send(tui.StreamDoneMsg{Message: *assistantMsg})
		fullHistory = append(fullHistory, *assistantMsg)

		// Tool call budget
		if len(toolCallHistory)+len(toolCalls) > maxToolCalls {
			logger.Warn().Str("component", "agent").
				Int("budget", maxToolCalls).
				Msg("agent.tool_budget_exhausted")
			c.Bus.Send(tui.ErrorMsg{Err: fmt.Errorf(
				"tool call budget exhausted (%d calls)", maxToolCalls)})
			return
		}

		// Execute tool calls
		results := c.executeToolCalls(ctx, sess, toolCalls, &fullHistory)
		for _, r := range results {
			toolCallHistory = append(toolCallHistory, agent.ToolCallRecord{Name: r.toolName, Args: r.args})

			// Track file reads for read-before-edit enforcement
			if r.toolName == "read" && !r.isError {
				if path := extractJSONField(r.args, "file_path"); path != "" {
					c.Tools.MarkFileRead(path)
				}
			}

			// Decision-aware context injection: when editing governed files,
			// inject invariants so the LLM knows what constraints to preserve.
			if (r.toolName == "edit" || r.toolName == "write") && !r.isError && c.ArtifactStore != nil {
				if path := extractJSONField(r.args, "file_path"); path != "" {
					c.injectDecisionContext(ctx, sess, path, &fullHistory)
				}
			}

			// Cycle binding: typed Meta from artifact tools → update cycle refs
			if r.meta != nil && !r.isError {
				c.bindCycleArtifact(ctx, sess, r.meta)
			}
		}
	}

	logger.Warn().Str("component", "agent").Msg("agent.max_steps")
}

// ---------------------------------------------------------------------------
// Tool call execution
// ---------------------------------------------------------------------------

type toolCallResult struct {
	toolName string
	args     string
	output   string
	isError  bool
	meta     *agent.ArtifactMeta
}

// executeToolCalls runs tool calls — parallel when all are auto-approved.
func (c *Coordinator) executeToolCalls(
	ctx context.Context,
	sess *agent.Session,
	toolCalls []agent.ToolCallPart,
	fullHistory *[]agent.Message,
) []toolCallResult {
	// Check if all tool calls can run without permission
	allSafe := true
	for _, tc := range toolCalls {
		if agent.EvaluatePermission(tc.ToolName, tc.Arguments) != agent.PermissionAllowed {
			allSafe = false
			break
		}
	}

	if allSafe && len(toolCalls) > 1 {
		return c.executeToolCallsParallel(ctx, sess, toolCalls, fullHistory)
	}
	return c.executeToolCallsSequential(ctx, sess, toolCalls, fullHistory)
}

func (c *Coordinator) executeToolCallsParallel(
	ctx context.Context,
	sess *agent.Session,
	toolCalls []agent.ToolCallPart,
	fullHistory *[]agent.Message,
) []toolCallResult {
	results := make([]toolCallResult, len(toolCalls))

	// Send start events: non-agent tools first, then spawn_agent
	for _, tc := range toolCalls {
		if tc.ToolName != "spawn_agent" {
			c.Bus.Send(tui.ToolStartMsg{ToolCallID: tc.ToolCallID, ToolName: tc.ToolName, Args: tc.Arguments})
		}
	}
	for _, tc := range toolCalls {
		if tc.ToolName == "spawn_agent" {
			c.Bus.Send(tui.ToolStartMsg{ToolCallID: tc.ToolCallID, ToolName: tc.ToolName, Args: tc.Arguments})
		}
	}

	var wg sync.WaitGroup
	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, tc agent.ToolCallPart) {
			defer wg.Done()
			toolStart := time.Now()
			toolResult, execErr := c.Tools.Execute(ctx, tc.ToolName, tc.Arguments)
			var output string
			isError := false
			if execErr != nil {
				output = fmt.Sprintf("Tool error: %s", execErr.Error())
				isError = true
			} else {
				output = toolResult.DisplayText
			}
			logger.AgentToolExec(tc.ToolName, tc.ToolCallID, time.Since(toolStart).Milliseconds(), isError)

			output = truncateToolOutput(output)
			results[idx] = toolCallResult{toolName: tc.ToolName, args: tc.Arguments, output: output, isError: isError, meta: toolResult.Meta}
			c.Bus.Send(tui.ToolDoneMsg{ToolCallID: tc.ToolCallID, ToolName: tc.ToolName, Output: output, IsError: isError})
		}(i, tc)
	}
	wg.Wait()

	for i, tc := range toolCalls {
		c.saveToolResult(ctx, sess, tc.ToolCallID, tc.ToolName, results[i].output, results[i].isError, fullHistory)
	}

	return results
}

func (c *Coordinator) executeToolCallsSequential(
	ctx context.Context,
	sess *agent.Session,
	toolCalls []agent.ToolCallPart,
	fullHistory *[]agent.Message,
) []toolCallResult {
	var results []toolCallResult

	for _, tc := range toolCalls {
		if ctx.Err() != nil {
			break
		}

		c.Bus.Send(tui.ToolStartMsg{ToolCallID: tc.ToolCallID, ToolName: tc.ToolName, Args: tc.Arguments})

		var output string
		var isError bool
		var meta *agent.ArtifactMeta

		// Permission check
		level := agent.EvaluatePermission(tc.ToolName, tc.Arguments)
		if level == agent.PermissionNeedsApproval {
			replyCh := make(chan bool, 1)
			c.Bus.Send(tui.PermissionAskMsg{ToolName: tc.ToolName, Args: tc.Arguments, Reply: replyCh})
			select {
			case allowed := <-replyCh:
				if !allowed {
					output = "Permission denied by user."
					isError = true
				}
			case <-ctx.Done():
				break
			}
		}

		// Execute
		if output == "" {
			toolStart := time.Now()
			toolResult, execErr := c.Tools.Execute(ctx, tc.ToolName, tc.Arguments)
			if execErr != nil {
				output = fmt.Sprintf("Tool error: %s", execErr.Error())
				isError = true
			} else {
				output = toolResult.DisplayText
				meta = toolResult.Meta
			}
			logger.AgentToolExec(tc.ToolName, tc.ToolCallID, time.Since(toolStart).Milliseconds(), isError)
		}

		output = truncateToolOutput(output)
		c.Bus.Send(tui.ToolDoneMsg{ToolCallID: tc.ToolCallID, ToolName: tc.ToolName, Output: output, IsError: isError})
		c.saveToolResult(ctx, sess, tc.ToolCallID, tc.ToolName, output, isError, fullHistory)
		results = append(results, toolCallResult{toolName: tc.ToolName, args: tc.Arguments, output: output, isError: isError, meta: meta})
	}

	return results
}

// ---------------------------------------------------------------------------
// Cycle binding — typed Meta from artifact tools updates cycle refs
// ---------------------------------------------------------------------------

func (c *Coordinator) bindCycleArtifact(ctx context.Context, sess *agent.Session, meta *agent.ArtifactMeta) {
	if c.Cycles == nil || meta == nil {
		return
	}

	cycle, err := c.Cycles.GetActiveCycle(ctx, sess.ID)
	if err != nil || cycle == nil {
		cycle = &agent.Cycle{
			ID:        "cyc_" + uuid.NewString(),
			SessionID: sess.ID,
			Phase:     agent.PhaseFramer,
			Status:    agent.CycleActive,
			CLMin:     3,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := c.Cycles.CreateCycle(ctx, cycle); err != nil {
			logger.Error().Str("component", "agent").Err(err).Msg("agent.cycle_create_error")
			return
		}
		sess.ActiveCycleID = cycle.ID
		_ = c.Sessions.Update(ctx, sess)
		c.sendCycleUpdate(cycle)

		logger.Info().Str("component", "agent").
			Str("cycle_id", cycle.ID).
			Msg("agent.cycle_created")
	}

	updated := agent.BindArtifact(cycle, *meta)
	if updated == nil {
		return
	}

	if err := c.Cycles.UpdateCycle(ctx, updated); err != nil {
		logger.Error().Str("component", "agent").Err(err).Msg("agent.cycle_update_error")
		return
	}
	c.sendCycleUpdate(updated)

	logger.Info().Str("component", "agent").
		Str("cycle_id", updated.ID).
		Str("kind", meta.Kind).
		Str("ref", meta.ArtifactRef).
		Str("phase", string(updated.Phase)).
		Msg("agent.cycle_artifact_bound")
}

// injectDriftWarnings runs drift detection on first turn and injects
// warnings about decisions whose governed files have changed.
func (c *Coordinator) injectDriftWarnings(ctx context.Context, history *[]agent.Message) {
	reports, err := artifact.CheckDrift(ctx, c.ArtifactStore, c.ProjectRoot)
	if err != nil || len(reports) == 0 {
		return
	}

	var warnings []string
	for _, r := range reports {
		if len(r.Files) > 0 {
			warnings = append(warnings, fmt.Sprintf("- %s: %s (%d files changed since baseline)",
				r.DecisionID, r.DecisionTitle, len(r.Files)))
		}
	}

	if len(warnings) == 0 {
		return
	}

	msg := agent.Message{
		Role: agent.RoleSystem,
		Parts: []agent.Part{agent.TextPart{Text: fmt.Sprintf(
			"[Drift detection] The following decisions have files that changed since their baseline:\n%s\nConsider reviewing with /refresh or quint_refresh(drift).",
			strings.Join(warnings, "\n"))}},
	}
	*history = append(*history, msg)

	logger.Info().Str("component", "agent").
		Int("drifted", len(warnings)).
		Msg("agent.drift_warnings_injected")
}

// injectDecisionContext checks if an edited file is governed by a decision
// and injects the decision's invariants as a system message.
// This is the differentiating feature — no other agent does mid-turn context injection.
func (c *Coordinator) injectDecisionContext(ctx context.Context, sess *agent.Session, filePath string, history *[]agent.Message) {
	related, err := artifact.FetchRelatedArtifacts(ctx, c.ArtifactStore, filePath)
	if err != nil || len(related) == 0 {
		return
	}

	var parts []string
	for _, r := range related {
		if r.Meta.Kind != artifact.KindDecisionRecord {
			continue
		}
		parts = append(parts, fmt.Sprintf("[%s] %s", r.Meta.ID, r.Meta.Title))
	}
	if len(parts) == 0 {
		return
	}

	warning := fmt.Sprintf("[System] File %s is governed by: %s. Check that your edit preserves decision invariants.",
		filePath, strings.Join(parts, ", "))

	msg := agent.Message{
		Role:  agent.RoleSystem,
		Parts: []agent.Part{agent.TextPart{Text: warning}},
	}
	*history = append(*history, msg)

	logger.Debug().Str("component", "agent").
		Str("file", filePath).
		Int("decisions", len(parts)).
		Msg("agent.decision_context_injected")
}

func (c *Coordinator) sendCycleUpdate(cycle *agent.Cycle) {
	c.Bus.Send(tui.CycleUpdateMsg{
		CycleID:      cycle.ID,
		ProblemRef:   cycle.ProblemRef,
		PortfolioRef: cycle.PortfolioRef,
		DecisionRef:  cycle.DecisionRef,
		Phase:        cycle.Phase,
		Status:       cycle.Status,
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncateToolOutput(output string) string {
	const maxOutputBytes = 50_000
	const maxOutputLines = 2000
	if len(output) > maxOutputBytes {
		output = output[:maxOutputBytes] + "\n... (truncated to 50KB)"
	}
	if lines := strings.Count(output, "\n"); lines > maxOutputLines {
		cutLines := strings.SplitN(output, "\n", maxOutputLines+1)
		output = strings.Join(cutLines[:maxOutputLines], "\n") +
			fmt.Sprintf("\n... (%d more lines)", lines-maxOutputLines)
	}
	return output
}

func (c *Coordinator) saveToolResult(ctx context.Context, sess *agent.Session, callID, toolName, output string, isError bool, history *[]agent.Message) {
	msg := &agent.Message{
		ID: newMsgID(), SessionID: sess.ID, Role: agent.RoleTool,
		Parts: []agent.Part{agent.ToolResultPart{
			ToolCallID: callID, ToolName: toolName, Content: output, IsError: isError,
		}},
		CreatedAt: time.Now().UTC(),
	}
	_ = c.Messages.Save(ctx, msg)
	*history = append(*history, *msg)
}

func (c *Coordinator) generateTitle(sess *agent.Session, userText string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	prompt := agent.BuildTitlePrompt(userText)
	titleMsg, err := c.Provider.Stream(ctx,
		[]agent.Message{{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: prompt}}}},
		nil,
		func(delta provider.StreamDelta) {},
	)
	if err != nil || titleMsg == nil {
		return
	}

	title := strings.TrimSpace(titleMsg.Text())
	title = strings.Trim(title, "\"'")
	if len(title) > 50 {
		title = title[:50]
	}
	if title == "" {
		return
	}

	sess.Title = title
	_ = c.Sessions.Update(context.Background(), sess)
	c.Bus.Send(tui.SessionTitleMsg{Title: title})
}

// sanitizeHistory ensures every tool call has a matching tool result.
func sanitizeHistory(msgs []agent.Message) []agent.Message {
	resultIDs := make(map[string]bool)
	for _, msg := range msgs {
		if msg.Role == agent.RoleTool {
			for _, p := range msg.Parts {
				if tr, ok := p.(agent.ToolResultPart); ok {
					resultIDs[tr.ToolCallID] = true
				}
			}
		}
	}

	var patches []agent.Message
	for _, msg := range msgs {
		if msg.Role != agent.RoleAssistant {
			continue
		}
		for _, tc := range msg.ToolCalls() {
			if !resultIDs[tc.ToolCallID] {
				patches = append(patches, agent.Message{
					Role: agent.RoleTool,
					Parts: []agent.Part{agent.ToolResultPart{
						ToolCallID: tc.ToolCallID,
						ToolName:   tc.ToolName,
						Content:    "[session interrupted — no result]",
						IsError:    true,
					}},
				})
			}
		}
	}

	if len(patches) == 0 {
		return msgs
	}
	return append(msgs, patches...)
}

// extractJSONField extracts a string field from a JSON args string.
func extractJSONField(argsJSON, field string) string {
	var args map[string]any
	if json.Unmarshal([]byte(argsJSON), &args) != nil {
		return ""
	}
	v, _ := args[field].(string)
	return v
}

func newMsgID() string {
	return "msg_" + uuid.New().String()
}
