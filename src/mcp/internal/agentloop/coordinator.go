package agentloop

import (
	"context"
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
	maxStepsPerPhase   = 50
	loopWindowSize     = 8
	loopMaxRepeats     = 3
	pendingSignalGrace = 3
)

// Coordinator runs one phase per user message.
// Phase state persists on the Session. Each Run() call:
// 1. Reads sess.CurrentPhase
// 2. Runs the ReAct loop for that phase with phase-specific prompt + tools
// 3. On transition signal: validates with NavState, saves next phase, returns to TUI
// 4. Next user message picks up from the saved phase
//
// V3-symmetric transition gate:
// - Signals propose transitions (fast, pure L2)
// - NavState validates proposals at phase boundaries (one DB query)
// - NavState generates proposals when signals are silent (fallback)
//
// In autonomous mode (session.Interaction == autonomous): auto-chains phases.
type Coordinator struct {
	Provider       provider.LLMProvider
	Tools          *tools.Registry
	Sessions       session.SessionStore
	Messages       session.MessageStore
	Cycles         session.CycleStore
	ArtifactStore  artifact.ArtifactStore // for NavState computation at phase boundaries
	Bus            *tui.Bus
	SystemPrompt   string
	AgentDef       agent.AgentDef
	SessionContext string // context name for scoping NavState to this session
	Subagents      *SubagentTracker
}

// Run executes one user turn.
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

	// Restore active cycle state to TUI (supports resume)
	if c.Cycles != nil {
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

	// Sanitize history: ensure every tool call has a matching tool result.
	// Interrupted sessions may have orphaned tool calls → OpenAI 400 error.
	history = sanitizeHistory(history)

	// Generate title on first turn (session has no title yet)
	isFirstTurn := sess.Title == "" && len(history) <= 1
	firstUserText := ""
	if isFirstTurn {
		firstUserText = userMsg.Text()
	}

	if c.AgentDef.Lemniscate {
		c.runPhase(ctx, sess, history)
	} else {
		c.runPlainReAct(ctx, sess, history)
	}

	// Async title generation after first turn
	if isFirstTurn && firstUserText != "" {
		go c.generateTitle(sess, firstUserText)
	}
}

func (c *Coordinator) generateTitle(sess *agent.Session, userText string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	prompt := agent.BuildTitlePrompt(userText)
	titleMsg, err := c.Provider.Stream(ctx,
		[]agent.Message{{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: prompt}}}},
		nil,                                 // no tools
		func(delta provider.StreamDelta) {}, // ignore streaming
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
	logger.Debug().Str("component", "agent").Str("title", title).Msg("agent.title_generated")
}

// ---------------------------------------------------------------------------
// Plain ReAct (no lemniscate)
// ---------------------------------------------------------------------------

func (c *Coordinator) runPlainReAct(ctx context.Context, sess *agent.Session, history []agent.Message) {
	systemMsg := agent.Message{
		Role:  agent.RoleSystem,
		Parts: []agent.Part{agent.TextPart{Text: c.SystemPrompt}},
	}
	fullHistory := append([]agent.Message{systemMsg}, history...)
	c.reactLoop(ctx, sess, fullHistory, c.Tools.List(), agent.PhaseReady, nil)
}

// ---------------------------------------------------------------------------
// Lemniscate: cycle-derived phase transitions
// ---------------------------------------------------------------------------

// runPhase executes the current phase.
// Phase transitions are derived from cycle state (via bindCycleArtifact),
// not from signal parsing. The cycle IS the state.
//
// Fallback: when no cycle exists (no artifact tools called), signal-based
// transitions still work for non-cycle phases (worker write/edit signals).
func (c *Coordinator) runPhase(ctx context.Context, sess *agent.Session, history []agent.Message) {
	if ctx.Err() != nil {
		return
	}

	currentPhase := sess.CurrentPhase

	// Auto-enter framer on first message
	if currentPhase == agent.PhaseReady {
		currentPhase = agent.PhaseFramer
		sess.CurrentPhase = currentPhase
		_ = c.Sessions.Update(ctx, sess)
		c.Bus.Send(tui.PhaseChangeMsg{From: agent.PhaseReady, To: currentPhase, Name: "haft-framer"})
		logger.Info().Str("component", "agent").Msg("agent.auto_enter_framer")
	}

	allTools := c.Tools.List()

	// Build phase-specific prompt and tools
	var systemPrompt string
	var phaseTools []agent.ToolSchema
	phaseDef := c.AgentDef.PhaseByID(currentPhase)
	if phaseDef != nil {
		systemPrompt = c.SystemPrompt + "\n\n" + phaseDef.SystemPrompt
		phaseTools = agent.FilterToolsForPhase(allTools, *phaseDef)
	}
	if systemPrompt == "" {
		systemPrompt = c.SystemPrompt
		phaseTools = allTools
	}

	systemMsg := agent.Message{
		Role:  agent.RoleSystem,
		Parts: []agent.Part{agent.TextPart{Text: systemPrompt}},
	}
	fullHistory := append([]agent.Message{systemMsg}, history...)

	logger.Debug().Str("component", "agent").
		Str("phase", string(currentPhase)).
		Int("tools", len(phaseTools)).
		Msg("agent.run_phase")

	// Run ReAct loop — bindCycleArtifact updates cycle state during execution
	signal := c.reactLoop(ctx, sess, fullHistory, phaseTools, currentPhase, phaseDef)

	// --- Cycle-derived transition ---
	// Primary: read phase from active cycle (updated by bindCycleArtifact)
	// Fallback: signal-based for non-cycle operations (worker write/edit, LLMDone)
	nextPhase := c.deriveNextPhase(ctx, sess, currentPhase, signal)

	logger.Debug().Str("component", "agent").
		Str("current", string(currentPhase)).
		Str("signal", string(signal)).
		Str("next", string(nextPhase)).
		Msg("agent.phase_transition")

	// No transition — stay or done
	if nextPhase == currentPhase || nextPhase == agent.PhaseReady {
		if nextPhase == agent.PhaseReady && currentPhase != agent.PhaseReady {
			sess.CurrentPhase = agent.PhaseReady
			_ = c.Sessions.Update(ctx, sess)
			c.Bus.Send(tui.PhaseChangeMsg{From: currentPhase, To: agent.PhaseReady, Name: ""})
			logger.Info().Str("component", "agent").Msg("agent.lemniscate_complete")
		}
		return
	}

	// Commit transition
	c.commitTransition(ctx, sess, currentPhase, nextPhase)

	// Autonomous mode: auto-chain immediately
	if sess.Interaction == agent.InteractionAutonomous {
		history, _ = c.Messages.ListBySession(ctx, sess.ID)
		c.runPhase(ctx, sess, history)
		return
	}

	// Symbiotic mode: pause for user approval.
	// y/Enter = proceed (auto-chain). n = stop, let user discuss.
	if c.sendPhasePause(ctx, sess, currentPhase, nextPhase) {
		// User approved — chain into next phase
		history, _ = c.Messages.ListBySession(ctx, sess.ID)
		c.runPhase(ctx, sess, history)
	}
	// User said "not yet" — return to TUI input. Next user message
	// will trigger Run which picks up from sess.CurrentPhase.
}

// deriveNextPhase determines the next phase.
// Primary path: cycle state (structural, from typed tool results).
// Fallback: signal-based (for worker write/edit, LLMDone, no-cycle sessions).
func (c *Coordinator) deriveNextPhase(
	ctx context.Context,
	sess *agent.Session,
	currentPhase agent.Phase,
	signal agent.TransitionSignal,
) agent.Phase {
	// Primary: check active cycle phase (set by bindCycleArtifact)
	// Only transition FORWARD — never go backward in the cycle.
	if c.Cycles != nil {
		if cycle, err := c.Cycles.GetActiveCycle(ctx, sess.ID); err == nil && cycle != nil {
			cyclePhase := agent.DerivePhaseFromCycle(cycle)
			if cyclePhase != currentPhase && agent.PhaseAfter(cyclePhase, currentPhase) {
				return cyclePhase
			}
		}
	}

	// Fallback: signal-based transitions for non-artifact events
	if signal == agent.SignalLLMDone {
		// Framer: stay in pre-abductive seam (FPF B.4.1)
		if currentPhase == agent.PhaseFramer {
			return agent.PhaseFramer
		}
		// Worker LLMDone → measure
		if currentPhase == agent.PhaseWorker {
			return agent.PhaseMeasure
		}
		// Measure LLMDone → complete cycle and finish
		if currentPhase == agent.PhaseMeasure {
			c.closeCycle(ctx, sess, signal)
			return agent.PhaseReady
		}
	}

	// Measure signals: complete or abandon cycle
	if currentPhase == agent.PhaseMeasure {
		if signal == agent.SignalMeasured || signal == agent.SignalTestsPassed {
			c.closeCycle(ctx, sess, signal)
			return agent.PhaseReady
		}
		if signal == agent.SignalMeasureFailed || signal == agent.SignalTestsFailed {
			c.reframeCycle(ctx, sess)
			return agent.PhaseFramer
		}
	}

	// Worker: write/edit signals → measure
	if signal == agent.SignalImplemented && currentPhase == agent.PhaseWorker {
		return agent.PhaseMeasure
	}

	return currentPhase
}

// commitTransition saves the transition and notifies TUI.
func (c *Coordinator) commitTransition(ctx context.Context, sess *agent.Session, from, to agent.Phase) {
	nextDef := c.AgentDef.PhaseByID(to)
	name := string(to)
	if nextDef != nil {
		name = nextDef.Name
	}

	logger.AgentPhase(string(from), string(to), name)

	// Save transition instruction
	transition := agent.BuildTransitionInstruction(from, to, "")
	transitionMsg := &agent.Message{
		ID: newMsgID(), SessionID: sess.ID, Role: agent.RoleSystem,
		Parts:     []agent.Part{agent.TextPart{Text: transition}},
		CreatedAt: time.Now().UTC(),
	}
	_ = c.Messages.Save(ctx, transitionMsg)

	// Update and persist session phase
	sess.CurrentPhase = to
	_ = c.Sessions.Update(ctx, sess)
	c.Bus.Send(tui.PhaseChangeMsg{From: from, To: to, Name: name})

	// Sync cycle phase (for transitions not driven by artifact binding,
	// e.g. worker→measure via write/edit signal)
	if c.Cycles != nil {
		if cycle, err := c.Cycles.GetActiveCycle(ctx, sess.ID); err == nil && cycle != nil {
			if cycle.Phase != to {
				cycle.Phase = to
				_ = c.Cycles.UpdateCycle(ctx, cycle)
				c.sendCycleUpdate(cycle)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ReAct loop core
// ---------------------------------------------------------------------------

func (c *Coordinator) reactLoop(
	ctx context.Context,
	sess *agent.Session,
	fullHistory []agent.Message,
	toolSchemas []agent.ToolSchema,
	currentPhase agent.Phase,
	phaseDef *agent.PhaseDef,
) agent.TransitionSignal {
	var (
		pendingSignal   agent.TransitionSignal
		pendingCount    int
		toolCallHistory []agent.ToolCallRecord
		toolCallCount   int
		tokenBudget     = agent.NewTokenBudget(agent.ModelContextWindow(sess.Model))
	)

	// Per-phase tool call budget (PhaseReady gets a default of 50)
	maxToolCalls := 50
	if phaseDef != nil && phaseDef.MaxToolCalls > 0 {
		maxToolCalls = phaseDef.MaxToolCalls
	}

	for step := 0; step < maxStepsPerPhase; step++ {
		if ctx.Err() != nil {
			return agent.SignalLLMDone
		}

		// Safety: loop detection
		if agent.DetectLoop(toolCallHistory, loopWindowSize, loopMaxRepeats) {
			logger.Warn().Str("component", "agent").Str("phase", string(currentPhase)).Msg("agent.loop_detected")
			c.Bus.Send(tui.ErrorMsg{Err: fmt.Errorf("loop detected: agent is repeating the same tool calls")})
			return agent.SignalLLMDone
		}

		// Context compaction: retry on every step while budget is pressured.
		// After compaction, reset token budget from the new (shorter) history.
		if newHistory, didCompact := c.compactContext(ctx, sess, fullHistory, tokenBudget); didCompact {
			fullHistory = newHistory
			tokenBudget = agent.ResetTokenBudget(tokenBudget, fullHistory)
			logger.Debug().Str("component", "agent").
				Int("used_after", tokenBudget.Used).
				Int("limit", tokenBudget.Limit).
				Msg("agent.compacted_budget_reset")
		}

		// Safety: token budget — checked AFTER compaction attempt
		if tokenBudget.Exhausted() {
			logger.Warn().Str("component", "agent").Int("used", tokenBudget.Used).Msg("agent.tokens_exhausted")
			c.Bus.Send(tui.ErrorMsg{Err: fmt.Errorf("context window exhausted (%d/%d tokens)", tokenBudget.Used, tokenBudget.Limit)})
			return agent.SignalLLMDone
		}

		logger.AgentStep(step, string(currentPhase), len(toolCallHistory), false)

		// LLM call (5 minute timeout per call — prevents runaway generation)
		llmCtx, llmCancel := context.WithTimeout(ctx, 5*time.Minute)
		llmStart := time.Now()
		assistantMsg, err := c.Provider.Stream(llmCtx, fullHistory, toolSchemas, func(delta provider.StreamDelta) {
			if delta.Text != "" {
				c.Bus.Send(tui.StreamDeltaMsg{Text: delta.Text})
			}
			if delta.Thinking != "" {
				c.Bus.Send(tui.ThinkingDeltaMsg{Text: delta.Thinking})
			}
		})
		llmCancel()
		if err != nil {
			logger.AgentError(string(currentPhase), err)
			// Error recovery: if the LLM partially produced tool calls,
			// save empty results so the conversation stays valid.
			if assistantMsg != nil {
				for _, tc := range assistantMsg.ToolCalls() {
					c.saveToolResult(ctx, sess, tc.ToolCallID, tc.ToolName,
						fmt.Sprintf("Tool call interrupted: %s", err.Error()), true, &fullHistory)
				}
			}
			c.Bus.Send(tui.ErrorMsg{Err: err})
			return agent.SignalLLMDone
		}

		assistantMsg.ID = newMsgID()
		assistantMsg.SessionID = sess.ID
		_ = c.Messages.Save(ctx, assistantMsg)

		toolCalls := assistantMsg.ToolCalls()
		tokenBudget = tokenBudget.Add(assistantMsg.Tokens)
		c.Bus.Send(tui.TokenUpdateMsg{Used: tokenBudget.Used, Limit: tokenBudget.Limit})

		logger.Debug().Str("component", "agent").
			Str("phase", string(currentPhase)).
			Int("step", step).
			Int("tool_calls", len(toolCalls)).
			Bool("has_text", assistantMsg.Text() != "").
			Int64("llm_ms", time.Since(llmStart).Milliseconds()).
			Int("tokens", assistantMsg.Tokens).
			Msg("agent.llm_response")

		// No tool calls — phase done
		if len(toolCalls) == 0 {
			logger.AgentMessage("assistant", assistantMsg.Text(), 0, assistantMsg.Tokens)
			c.Bus.Send(tui.StreamDoneMsg{Message: *assistantMsg})
			if pendingSignal != "" {
				return pendingSignal
			}
			return agent.SignalLLMDone
		}

		c.Bus.Send(tui.StreamDoneMsg{Message: *assistantMsg})
		fullHistory = append(fullHistory, *assistantMsg)

		// Execute tool calls — parallel when all are auto-approved, sequential otherwise
		toolCallCount += len(toolCalls)
		if maxToolCalls > 0 && toolCallCount > maxToolCalls {
			logger.Warn().Str("component", "agent").
				Str("phase", string(currentPhase)).
				Int("budget", maxToolCalls).
				Msg("agent.phase_budget_exhausted")
			c.Bus.Send(tui.ErrorMsg{Err: fmt.Errorf(
				"phase %s budget exhausted (%d tool calls)", currentPhase, maxToolCalls)})
			return agent.SignalLLMDone
		}

		var lastSignal agent.TransitionSignal
		results := c.executeToolCalls(ctx, sess, toolCalls, currentPhase, phaseDef, &fullHistory)
		cycleTransition := false
		for _, r := range results {
			toolCallHistory = append(toolCallHistory, agent.ToolCallRecord{Name: r.toolName, Args: r.args})

			// Cycle binding: typed Meta from artifact tools → update cycle refs
			if r.meta != nil && !r.isError {
				c.bindCycleArtifact(ctx, sess, r.meta)

				// Check if cycle binding changed the phase — if so, break immediately.
				// Don't let the LLM continue in the old phase with wrong tools.
				if c.Cycles != nil {
					if cyc, err := c.Cycles.GetActiveCycle(ctx, sess.ID); err == nil && cyc != nil {
						if agent.DerivePhaseFromCycle(cyc) != currentPhase {
							cycleTransition = true
						}
					}
				}
			}

			if sig := detectSignal(currentPhase, r.toolName, r.args, r.output, r.isError); sig != "" {
				logger.AgentSignal(string(currentPhase), string(sig), r.toolName)
				lastSignal = sig
			}
		}

		// Cycle-driven phase change: exit reactLoop immediately.
		// deriveNextPhase will read the new phase from the cycle.
		if cycleTransition {
			return lastSignal
		}

		// Store first signal, let LLM produce summary
		if lastSignal != "" && currentPhase != agent.PhaseReady {
			if pendingSignal == "" {
				pendingSignal = lastSignal
			}
		}

		if pendingSignal != "" {
			pendingCount++
			if pendingCount > pendingSignalGrace {
				return pendingSignal
			}
		}
	}

	logger.Warn().Str("component", "agent").Msg("agent.max_steps")
	return agent.SignalLLMDone
}

// ---------------------------------------------------------------------------
// Tool call execution — parallel when safe, sequential otherwise
// ---------------------------------------------------------------------------

type toolCallResult struct {
	toolName string
	args     string
	output   string
	isError  bool
	meta     *agent.ArtifactMeta // non-nil for artifact-producing tools
}

// executeToolCalls runs tool calls, parallelizing when all are auto-approved.
func (c *Coordinator) executeToolCalls(
	ctx context.Context,
	sess *agent.Session,
	toolCalls []agent.ToolCallPart,
	currentPhase agent.Phase,
	phaseDef *agent.PhaseDef,
	fullHistory *[]agent.Message,
) []toolCallResult {
	// Check if all tool calls can run without permission
	allSafe := true
	for _, tc := range toolCalls {
		if phaseDef != nil && !agent.IsToolAllowed(tc.ToolName, *phaseDef) {
			allSafe = false
			break
		}
		if agent.EvaluatePermission(tc.ToolName, tc.Arguments) != agent.PermissionAllowed {
			allSafe = false
			break
		}
	}

	if allSafe && len(toolCalls) > 1 {
		return c.executeToolCallsParallel(ctx, sess, toolCalls, fullHistory)
	}
	return c.executeToolCallsSequential(ctx, sess, toolCalls, currentPhase, phaseDef, fullHistory)
}

// executeToolCallsParallel runs all tool calls concurrently.
func (c *Coordinator) executeToolCallsParallel(
	ctx context.Context,
	sess *agent.Session,
	toolCalls []agent.ToolCallPart,
	fullHistory *[]agent.Message,
) []toolCallResult {
	results := make([]toolCallResult, len(toolCalls))

	// Send start events: non-agent tools first, then spawn_agent.
	// Agents are long-running — rendering them last keeps them visible
	// at the bottom of the TUI while the user waits.
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

	// Execute in parallel
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

			// Send done event from goroutine (bus is thread-safe)
			c.Bus.Send(tui.ToolDoneMsg{ToolCallID: tc.ToolCallID, ToolName: tc.ToolName, Output: output, IsError: isError})
		}(i, tc)
	}
	wg.Wait()

	// Save results to history sequentially (maintains message order)
	for i, tc := range toolCalls {
		c.saveToolResult(ctx, sess, tc.ToolCallID, tc.ToolName, results[i].output, results[i].isError, fullHistory)
	}

	return results
}

// executeToolCallsSequential runs tool calls one at a time (for permission-required tools).
func (c *Coordinator) executeToolCallsSequential(
	ctx context.Context,
	sess *agent.Session,
	toolCalls []agent.ToolCallPart,
	currentPhase agent.Phase,
	phaseDef *agent.PhaseDef,
	fullHistory *[]agent.Message,
) []toolCallResult {
	var results []toolCallResult

	for _, tc := range toolCalls {
		if ctx.Err() != nil {
			break
		}

		// Phase tool gating
		if phaseDef != nil && !agent.IsToolAllowed(tc.ToolName, *phaseDef) {
			logger.AgentToolGated(string(currentPhase), tc.ToolName)
			output := fmt.Sprintf("Tool '%s' is not available in the %s phase.", tc.ToolName, phaseDef.Name)
			c.Bus.Send(tui.ToolStartMsg{ToolCallID: tc.ToolCallID, ToolName: tc.ToolName, Args: tc.Arguments})
			c.Bus.Send(tui.ToolDoneMsg{ToolCallID: tc.ToolCallID, ToolName: tc.ToolName, Output: output, IsError: true})
			c.saveToolResult(ctx, sess, tc.ToolCallID, tc.ToolName, output, true, fullHistory)
			results = append(results, toolCallResult{toolName: tc.ToolName, args: tc.Arguments, output: output, isError: true})
			continue
		}

		c.Bus.Send(tui.ToolStartMsg{ToolCallID: tc.ToolCallID, ToolName: tc.ToolName, Args: tc.Arguments})

		// Permission
		var output string
		var isError bool
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
		var meta *agent.ArtifactMeta
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

// truncateToolOutput caps tool output to prevent context blowup.
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

// ---------------------------------------------------------------------------
// Cycle binding — typed Meta from artifact tools updates cycle refs
// ---------------------------------------------------------------------------

// bindCycleArtifact creates or updates the active cycle when an artifact tool
// returns structured Meta. This is the typed replacement for string-based
// signal detection for cycle state changes.
func (c *Coordinator) bindCycleArtifact(ctx context.Context, sess *agent.Session, meta *agent.ArtifactMeta) {
	if c.Cycles == nil || meta == nil {
		return
	}

	// Get or create active cycle
	cycle, err := c.Cycles.GetActiveCycle(ctx, sess.ID)
	if err != nil || cycle == nil {
		// First artifact in this session — create a new cycle
		cycle = &agent.Cycle{
			ID:        "cyc_" + uuid.NewString(),
			SessionID: sess.ID,
			Phase:     agent.PhaseFramer,
			Depth:     sess.Depth,
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
			Str("depth", string(cycle.Depth)).
			Msg("agent.cycle_created")
	}

	// Bind artifact ref to cycle (L1 pure function)
	updated := agent.BindArtifact(cycle, *meta)
	if updated == nil {
		return // meta didn't bind (notes, queries, etc.)
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

// sendPhasePause sends a pause summary to the TUI in symbiotic mode.
// Returns true if user approved (y/Enter), false if user wants to discuss (n).
func (c *Coordinator) sendPhasePause(ctx context.Context, sess *agent.Session, from, to agent.Phase) bool {
	summary := buildPhaseSummary(from, to, sess)

	replyCh := make(chan bool, 1)
	c.Bus.Send(tui.PhasePauseMsg{
		Phase:   to,
		Summary: summary,
		Reply:   replyCh,
	})

	// Block until user responds
	var proceed bool
	select {
	case proceed = <-replyCh:
	case <-ctx.Done():
		return false
	}

	logger.Info().Str("component", "agent").
		Str("from", string(from)).
		Str("to", string(to)).
		Bool("proceed", proceed).
		Msg("agent.phase_pause_acknowledged")
	return proceed
}

func buildPhaseSummary(from, to agent.Phase, _ *agent.Session) string {
	switch to {
	case agent.PhaseExplorer:
		return "Problem framed. Next: characterize dimensions + explore variants."
	case agent.PhaseDecider:
		return "Variants explored. Next: compare on dimensions + decide."
	case agent.PhaseWorker:
		return "Decision recorded. Next: implement the chosen approach."
	case agent.PhaseMeasure:
		return "Implementation done. Next: verify against predictions."
	default:
		return fmt.Sprintf("%s → %s", from, to)
	}
}

// closeCycle marks the active cycle as complete after successful measurement.
func (c *Coordinator) closeCycle(ctx context.Context, sess *agent.Session, signal agent.TransitionSignal) {
	if c.Cycles == nil {
		return
	}
	cycle, err := c.Cycles.GetActiveCycle(ctx, sess.ID)
	if err != nil || cycle == nil {
		return
	}

	// Determine weakest link from signal type
	wlnk := "measurement passed"
	if signal == agent.SignalTestsPassed {
		wlnk = "tests passed (no structured prediction check)"
	}

	completed := agent.CompleteCycle(cycle, wlnk, 0.0) // R_eff computed later when predictions exist
	_ = c.Cycles.UpdateCycle(ctx, completed)

	sess.ActiveCycleID = ""
	_ = c.Sessions.Update(ctx, sess)
	c.sendCycleUpdate(completed)

	logger.Info().Str("component", "agent").
		Str("cycle_id", completed.ID).
		Str("weakest_link", wlnk).
		Msg("agent.cycle_completed")
}

// reframeCycle abandons the current cycle and creates a new one with lineage.
// Called when measurement fails — the lemniscate loops back to framer.
func (c *Coordinator) reframeCycle(ctx context.Context, sess *agent.Session) {
	if c.Cycles == nil {
		return
	}
	cycle, err := c.Cycles.GetActiveCycle(ctx, sess.ID)
	if err != nil || cycle == nil {
		return
	}

	abandoned := agent.AbandonCycle(cycle)
	_ = c.Cycles.UpdateCycle(ctx, abandoned)

	// Create new cycle linked to the failed one
	newCycle := agent.NewCycleFromLineage("cyc_"+uuid.NewString(), sess.ID, abandoned)
	_ = c.Cycles.CreateCycle(ctx, newCycle)

	sess.ActiveCycleID = newCycle.ID
	_ = c.Sessions.Update(ctx, sess)
	c.sendCycleUpdate(newCycle)

	logger.Info().Str("component", "agent").
		Str("failed_cycle", abandoned.ID).
		Str("new_cycle", newCycle.ID).
		Msg("agent.cycle_reframed")
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
// Signal detection
// ---------------------------------------------------------------------------

func detectSignal(phase agent.Phase, toolName, args, output string, isError bool) agent.TransitionSignal {
	switch phase {
	case agent.PhaseReady, agent.PhaseFramer:
		if toolName == "quint_problem" && strings.Contains(args, `"frame"`) {
			return agent.SignalProblemFramed
		}

	case agent.PhaseExplorer:
		if toolName == "quint_solution" && strings.Contains(args, `"explore"`) {
			return agent.SignalVariantsExplored
		}

	case agent.PhaseDecider:
		if toolName == "quint_decision" && strings.Contains(args, `"decide"`) {
			return agent.SignalDecisionMade
		}

	case agent.PhaseWorker:
		// Gap 1 fix: check !isError — failed writes don't count as implementation
		if (toolName == "write" || toolName == "edit") && !isError {
			return agent.SignalImplemented
		}

	case agent.PhaseMeasure:
		if toolName == "quint_decision" && strings.Contains(args, `"measure"`) {
			if isError || strings.Contains(strings.ToLower(output), "failed") {
				return agent.SignalMeasureFailed
			}
			return agent.SignalMeasured
		}
		if toolName == "bash" {
			lowerArgs := strings.ToLower(args)
			isTest := strings.Contains(lowerArgs, "test") ||
				strings.Contains(lowerArgs, "pytest") ||
				strings.Contains(lowerArgs, "jest") ||
				strings.Contains(lowerArgs, "cargo test")
			if isTest {
				if isError {
					return agent.SignalTestsFailed
				}
				return agent.SignalTestsPassed
			}
		}
	}
	return ""
}

// sanitizeHistory ensures every tool call has a matching tool result.
// Adds placeholder results for orphaned calls (interrupted sessions).
func sanitizeHistory(msgs []agent.Message) []agent.Message {
	// Collect all tool call IDs and tool result IDs
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

	// Find orphaned tool calls and append placeholder results
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

func newMsgID() string {
	return "msg_" + uuid.New().String()
}
