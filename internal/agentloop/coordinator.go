package agentloop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/m0n0x41d/haft/assurance"
	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/hooks"
	"github.com/m0n0x41d/haft/internal/protocol"
	"github.com/m0n0x41d/haft/internal/provider"
	"github.com/m0n0x41d/haft/internal/reff"
	"github.com/m0n0x41d/haft/internal/session"
	"github.com/m0n0x41d/haft/internal/tools"
	"github.com/m0n0x41d/haft/logger"
)

const (
	maxStepsPerTurn = 200 // hard cap per user turn
	loopWindowSize  = 12  // recent tool calls to check for loops
	loopWarnRepeats = 3   // yellow: inject warning, don't stop
	loopHardRepeats = 5   // red: hard stop

	assuranceAdapterLevel = "artifact_adapter"
)

// Coordinator runs a single ReAct loop per user turn.
// v2: No phase machine. FPF enforced by tool guardrails.
// One unified prompt, all tools available. Cycle binding happens
// automatically when artifact tools return typed Meta.
type Coordinator struct {
	Provider       provider.LLMProvider
	Tools          *tools.Registry
	Sessions       session.SessionStore
	Messages       session.MessageStore
	Cycles         session.CycleStore
	ArtifactStore  artifact.ArtifactStore
	Bus            *protocol.Bus
	SystemPrompt   string
	AgentDef       agent.AgentDef
	Subagents      *SubagentTracker
	ProjectRoot    string               // for drift detection
	repoMapCache   string               // cached repo map text, invalidated after edit/write
	repoMapDirty   bool                 // true = needs rebuild before next LLM call
	evidence       *agent.EvidenceChain // auto-tracked evidence for active decision
	OverseerAlerts chan []string        // pending alerts from background overseer (thread-safe)
	planMode       bool                 // true = write tools blocked (plan mode active)
	Hooks          *hooks.Executor      // optional: pre/post tool hooks
}

// SetPlanMode toggles plan mode. Implements tools.PlanModeController.
func (c *Coordinator) SetPlanMode(enabled bool) { c.planMode = enabled }

// IsPlanMode returns whether plan mode is active. Implements tools.PlanModeController.
func (c *Coordinator) IsPlanMode() bool { return c.planMode }

// Run executes one user turn: save message → react loop → done.
func (c *Coordinator) Run(ctx context.Context, sess *agent.Session, userParts []agent.Part) {
	if len(userParts) == 0 {
		userParts = []agent.Part{agent.TextPart{Text: ""}}
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Error().Str("component", "agent").Interface("panic", r).Msg("agent.panic")
			c.Bus.SendError(fmt.Sprintf("coordinator panic: %v", r))
		}
		c.Bus.SendCoordDone()
	}()

	logger.AgentSession("user_turn", sess.ID, sess.Model)
	logger.AgentMessage("user", textFromParts(userParts), 0, 0)

	// Wire cycle resolver so tools can check FPF guardrails
	if c.Cycles != nil {
		sessID := sess.ID
		c.Tools.SetCycleResolver(func(ctx context.Context) *agent.Cycle {
			cycle, _ := c.Cycles.GetActiveCycle(ctx, sessID)
			return cycle
		})

		// Wire Transformer Mandate consent checker:
		// Returns true if a user message exists after the last explore tool call,
		// OR if interaction mode is autonomous (user explicitly delegated).
		c.Tools.SetConsentChecker(func(ctx context.Context) bool {
			if sess.Interaction == agent.InteractionAutonomous {
				return true // user delegated — skip consent check
			}
			// Check message history: was there a user message after the last explore?
			msgs, err := c.Messages.ListBySession(ctx, sessID)
			if err != nil {
				return true // on error, don't block
			}
			lastExploreIdx := -1
			lastUserIdx := -1
			for i, msg := range msgs {
				for _, part := range msg.Parts {
					if tp, ok := part.(agent.ToolResultPart); ok {
						if tp.ToolName == "haft_solution" && !tp.IsError {
							lastExploreIdx = i
						}
					}
				}
				if msg.Role == agent.RoleUser {
					lastUserIdx = i
				}
			}
			// No explore yet → no consent needed (pre-explore phase)
			if lastExploreIdx == -1 {
				return true
			}
			// User message after explore → consent given
			return lastUserIdx > lastExploreIdx
		})

		// Restore active cycle state to TUI (or clear if none)
		if cycle, err := c.Cycles.GetActiveCycle(ctx, sess.ID); err == nil && cycle != nil {
			c.sendCycleUpdate(cycle)
		} else {
			// No active cycle — clear any stale display from previous session
			c.Bus.SendCycleUpdate(protocol.CycleUpdate{})
		}
	}

	// Save user message
	userMsg := &agent.Message{
		ID:        newMsgID(),
		SessionID: sess.ID,
		Role:      agent.RoleUser,
		Parts:     userParts,
		CreatedAt: time.Now().UTC(),
	}
	if err := c.Messages.Save(ctx, userMsg); err != nil {
		c.Bus.SendError(fmt.Sprintf("save user message: %s", err))
		return
	}

	history, err := c.Messages.ListBySession(ctx, sess.ID)
	if err != nil {
		c.Bus.SendError(fmt.Sprintf("load history: %s", err))
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

	// Build system prompt: unified prompt + repo map (dynamic, invalidated after edits)
	systemPrompt := c.SystemPrompt

	// Inject interaction mode — the LLM must know if autonomous mode is active
	if sess.Interaction == agent.InteractionAutonomous {
		systemPrompt += `

## [MODE: AUTONOMOUS — ACTIVE NOW]

The user explicitly toggled autonomous mode. This OVERRIDES the collaborative workflow rules above.

IN AUTONOMOUS MODE YOU MUST:
- ACT, don't describe. Never say "I'll do X" — just DO X.
- CHAIN all phases in one turn: frame → explore → decide → implement → measure.
- EXECUTE tool calls immediately. Don't read 3 files then explain what you found — read, act, continue.
- SKIP all "STOP and present" checkpoints. The user delegated — they trust you.
- NEVER ask "shall I proceed?" or "would you like me to..." — the answer is always YES.
- When the user says "do it" or "давай" — that means START WORKING NOW, not "explain your plan."

The Transformer Mandate still applies to DECISIONS (user's choices/directions are authoritative).
But it does NOT apply to EXECUTION STEPS — you don't need approval for each tool call, each file read, each edit. Just do the work.`
	}

	// Inject repo map (lazy rebuild after edits)
	if repoMap := c.getRepoMap(); repoMap != "" {
		systemPrompt += "\n\n" + repoMap
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
		tokenBudget     = agent.NewTokenBudget(provider.DefaultRegistry().ContextWindow(sess.Model))
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

		// Loop detection — three-level escalation
		loopLevel := agent.DetectLoopLevel(toolCallHistory, loopWindowSize, loopWarnRepeats, loopHardRepeats)
		switch loopLevel {
		case agent.LoopHard:
			logger.Warn().Str("component", "agent").Msg("agent.loop_hard_stop")
			c.Bus.SendError("loop detected: agent is repeating the same tool calls")
			return
		case agent.LoopWarning:
			// Yellow: inject warning, don't stop. Agent should summarize and ask user.
			warnMsg := agent.Message{
				Role:  agent.RoleSystem,
				Parts: []agent.Part{agent.TextPart{Text: "[Loop Warning] You are repeating similar tool calls. Stop iterating. Summarize what you've tried, what's blocking you, and present current options to the user. If you need to retry, explain why this attempt will differ."}},
			}
			fullHistory = append(fullHistory, warnMsg)
			logger.Warn().Str("component", "agent").Msg("agent.loop_warning_injected")
		}

		// Inject overseer alerts as system message (if any pending)
		if c.OverseerAlerts != nil {
			select {
			case alerts := <-c.OverseerAlerts:
				if len(alerts) > 0 {
					alertMsg := agent.Message{
						Role:  agent.RoleSystem,
						Parts: []agent.Part{agent.TextPart{Text: "[Overseer] " + strings.Join(alerts, "; ") + ". Mention to user if relevant."}},
					}
					fullHistory = append(fullHistory, alertMsg)
				}
			default:
				// no alerts pending, continue
			}
		}

		// Token budget
		if tokenBudget.Exhausted() {
			logger.Warn().Str("component", "agent").Int("used", tokenBudget.Used).Msg("agent.tokens_exhausted")
			c.Bus.SendError(fmt.Sprintf("context window exhausted (%d/%d tokens)", tokenBudget.Used, tokenBudget.Limit))
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
		liveAssistant := &agent.Message{
			ID:        newMsgID(),
			SessionID: sess.ID,
			Role:      agent.RoleAssistant,
			Model:     c.Provider.ModelID(),
			CreatedAt: time.Now().UTC(),
		}
		_ = c.Messages.Save(ctx, liveAssistant)
		llmCtx, llmCancel := context.WithTimeout(ctx, 5*time.Minute)
		llmStart := time.Now()
		assistantMsg, err := c.Provider.Stream(llmCtx, fullHistory, allTools, func(delta provider.StreamDelta) {
			updated := false
			if delta.Text != "" {
				liveAssistant.AppendText(delta.Text)
				updated = true
			}
			if delta.Thinking != "" {
				liveAssistant.AppendThinking(delta.Thinking)
				updated = true
			}
			if updated {
				_ = c.Messages.UpdateMessage(ctx, liveAssistant)
				c.Bus.SendMsgUpdate(msgToUpdate(liveAssistant, true))
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
			c.Bus.SendError(err.Error())
			return
		}

		assistantMsg.ID = liveAssistant.ID
		assistantMsg.SessionID = sess.ID
		assistantMsg.CreatedAt = liveAssistant.CreatedAt
		// Merge: responseToMessage has authoritative text + tool calls from final response.
		// liveAssistant has streaming-accumulated thinking (reasoning summaries).
		// Append thinking parts that only exist in the streaming accumulator.
		for _, p := range liveAssistant.Parts {
			if tp, ok := p.(agent.TextPart); ok && strings.HasPrefix(tp.Text, "[thinking]") {
				assistantMsg.Parts = append(assistantMsg.Parts, tp)
			}
		}
		_ = c.Messages.UpdateMessage(ctx, assistantMsg)
		c.Bus.SendMsgUpdate(msgToUpdate(assistantMsg, false))

		toolCalls := assistantMsg.ToolCalls()
		tokenBudget = tokenBudget.Add(assistantMsg.Tokens)
		c.Bus.SendTokenUpdate(protocol.TokenUpdate{Used: tokenBudget.Used, Limit: tokenBudget.Limit})

		// Log LLM response details
		toolNames := make([]string, len(toolCalls))
		for i, tc := range toolCalls {
			toolNames[i] = tc.ToolName
		}
		logger.Info().Str("component", "agent").
			Int("step", step).
			Int("tool_calls", len(toolCalls)).
			Bool("has_text", assistantMsg.Text() != "").
			Int("tokens", assistantMsg.Tokens).
			Strs("tools", toolNames).
			Msg("agent.llm_response")

		// Detect empty response — LLM returned nothing useful
		hasText := assistantMsg.Text() != ""
		hasThinking := liveAssistant != nil && strings.Contains(liveAssistant.Text(), "[thinking]")
		if !hasText && len(toolCalls) == 0 && !hasThinking {
			if tokenBudget.NeedsSummarization() {
				c.Bus.SendError(fmt.Sprintf("context window nearly full (%d/%d tokens). Use /compact to free space", tokenBudget.Used, tokenBudget.Limit))
			} else {
				c.Bus.SendError("model returned empty response — try rephrasing or use /compact")
			}
			return
		}

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
			return
		}

		fullHistory = append(fullHistory, *assistantMsg)

		// Tool call budget
		if len(toolCallHistory)+len(toolCalls) > maxToolCalls {
			logger.Warn().Str("component", "agent").
				Int("budget", maxToolCalls).
				Msg("agent.tool_budget_exhausted")
			c.Bus.SendError(fmt.Sprintf(
				"tool call budget exhausted (%d calls)", maxToolCalls))
			return
		}

		// Execute tool calls
		results := c.executeToolCalls(ctx, sess, toolCalls, &fullHistory)
		for _, r := range results {
			toolCallHistory = append(toolCallHistory, agent.ToolCallRecord{Name: r.toolName, Args: r.args, IsError: r.isError})

			// Track file reads for read-before-edit enforcement
			if r.toolName == "read" && !r.isError {
				if path := extractJSONField(r.args, "path"); path != "" {
					c.Tools.MarkFileRead(path)
				}
			}

			// Invalidate repo map after file changes
			if (r.toolName == "edit" || r.toolName == "write" || r.toolName == "multiedit") && !r.isError {
				c.invalidateRepoMap()
			}

			// Decision-aware context injection: when editing governed files,
			// inject invariants so the LLM knows what constraints to preserve.
			if (r.toolName == "edit" || r.toolName == "write") && !r.isError && c.ArtifactStore != nil {
				if path := extractJSONField(r.args, "file_path"); path != "" {
					c.injectDecisionContext(ctx, sess, path, &fullHistory)
				}
			}

			// Evidence auto-tracking: observe agent actions after decide
			if c.evidence != nil {
				if ev := agent.DetectObservationFromTool(r.toolName, r.args, r.output, r.isError); ev != nil {
					c.evidence.Items = append(c.evidence.Items, *ev)
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
	// Plan mode enforcement: block write tools before they reach the executor
	if c.planMode {
		var allowed []agent.ToolCallPart
		var blocked []toolCallResult
		for _, tc := range toolCalls {
			if tools.PlanModeGuard(c, tc.ToolName) {
				c.Bus.SendToolStart(protocol.ToolStart{CallID: tc.ToolCallID, Name: tc.ToolName, Args: tc.Arguments})
				msg := tools.PlanModeBlockMessage(tc.ToolName)
				c.Bus.SendToolDone(protocol.ToolDone{CallID: tc.ToolCallID, Name: tc.ToolName, Output: msg, IsError: true})
				blocked = append(blocked, toolCallResult{toolName: tc.ToolName, args: tc.Arguments, output: msg, isError: true})
			} else {
				allowed = append(allowed, tc)
			}
		}
		if len(allowed) == 0 {
			return blocked
		}
		results := c.executeToolCallsInner(ctx, sess, allowed, fullHistory)
		return append(blocked, results...)
	}

	return c.executeToolCallsInner(ctx, sess, toolCalls, fullHistory)
}

func (c *Coordinator) executeToolCallsInner(
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
			c.Bus.SendToolStart(protocol.ToolStart{CallID: tc.ToolCallID, Name: tc.ToolName, Args: tc.Arguments})
		}
	}
	for _, tc := range toolCalls {
		if tc.ToolName == "spawn_agent" {
			c.Bus.SendToolStart(protocol.ToolStart{CallID: tc.ToolCallID, Name: tc.ToolName, Args: tc.Arguments})
		}
	}

	var wg sync.WaitGroup
	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, tc agent.ToolCallPart) {
			defer wg.Done()
			toolStart := time.Now()
			toolCtx := tools.WithActiveToolCallID(ctx, tc.ToolCallID)
			toolResult, execErr := c.Tools.Execute(toolCtx, tc.ToolName, tc.Arguments)
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
			c.Bus.SendToolDone(protocol.ToolDone{CallID: tc.ToolCallID, Name: tc.ToolName, Output: output, IsError: isError})
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

		c.Bus.SendToolStart(protocol.ToolStart{CallID: tc.ToolCallID, Name: tc.ToolName, Args: tc.Arguments})

		var output string
		var isError bool
		var meta *agent.ArtifactMeta

		// Permission check
		level := agent.EvaluatePermission(tc.ToolName, tc.Arguments)
		if level == agent.PermissionNeedsApproval {
			reply, err := c.Bus.AskPermission(protocol.PermissionAsk{
				ToolName: tc.ToolName,
				Args:     tc.Arguments,
			})
			if err != nil || reply.Action == "deny" {
				output = "Permission denied by user."
				isError = true
			}
		}

		// Pre-tool hook
		if output == "" && c.Hooks != nil && c.Hooks.HasHooks() {
			hookResults := c.Hooks.Run(ctx, hooks.TriggerPreTool, hooks.HookEnv{
				ToolName: tc.ToolName, ToolArgs: tc.Arguments, SessionID: sess.ID,
			})
			for _, hr := range hookResults {
				if hr.Blocked {
					output = fmt.Sprintf("Blocked by hook '%s': %s", hr.Name, hr.Message)
					isError = true
					break
				}
			}
		}

		// Execute
		if output == "" {
			toolStart := time.Now()
			toolCtx := tools.WithActiveToolCallID(ctx, tc.ToolCallID)
			toolResult, execErr := c.Tools.Execute(toolCtx, tc.ToolName, tc.Arguments)
			if execErr != nil {
				output = fmt.Sprintf("Tool error: %s", execErr.Error())
				isError = true
			} else {
				output = toolResult.DisplayText
				meta = toolResult.Meta
			}
			logger.AgentToolExec(tc.ToolName, tc.ToolCallID, time.Since(toolStart).Milliseconds(), isError)

			// Post-tool hook
			if c.Hooks != nil && c.Hooks.HasHooks() {
				c.Hooks.Run(ctx, hooks.TriggerPostTool, hooks.HookEnv{
					ToolName: tc.ToolName, ToolArgs: tc.Arguments, ToolOutput: output, SessionID: sess.ID,
				})
			}
		}

		output = truncateToolOutput(output)
		c.Bus.SendToolDone(protocol.ToolDone{CallID: tc.ToolCallID, Name: tc.ToolName, Output: output, IsError: isError})
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
		if meta.Operation == "measure" && meta.MeasureVerdict != "" {
			updated = cycle
		} else {
			return
		}
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

	// Start evidence tracking when decision is made
	if meta.Operation == "decide" {
		c.evidence = &agent.EvidenceChain{
			DecRef:   meta.ArtifactRef,
			CycleRef: updated.ID,
		}
		logger.Debug().Str("component", "agent").
			Str("decision", meta.ArtifactRef).
			Msg("agent.evidence_tracking_started")
	}

	// Cycle closure: measure verdict closes or abandons the cycle
	if meta.Operation == "measure" && meta.MeasureVerdict != "" {
		// Add measure verdict as evidence
		if c.evidence != nil {
			var evType agent.EvidenceType
			if meta.MeasureVerdict == "accepted" {
				evType = agent.EvidenceMeasure
			} else {
				evType = agent.EvidencePartial
			}
			c.evidence.Items = append(c.evidence.Items, agent.NewEvidenceItem(evType, "verdict: "+meta.MeasureVerdict, 3))
		}

		// If no evidence at all (agent skipped tests/verification), add "no_verification"
		if c.evidence != nil && len(c.evidence.Items) <= 1 {
			c.evidence.Items = append(c.evidence.Items, agent.NewEvidenceItem(agent.ObservationNoVerify, "no tests or lint run before measure", 3))
		}

		assurance, wlnk, syncErr := c.computeClosedCycleAssurance(ctx, updated.DecisionRef, c.evidence)
		if syncErr != nil {
			logger.Warn().Str("component", "agent").
				Str("decision_ref", updated.DecisionRef).
				Err(syncErr).
				Msg("agent.assurance_sync_failed")
			c.recordSystemWarning(ctx, sess.ID,
				fmt.Sprintf("[Assurance Warning] durable assurance sync failed; using cycle-local evidence only: %s", syncErr.Error()),
			)
		}

		rEff := assurance.R

		// R_eff threshold check (FPF: low evidence = low trust)
		if rEffErr := agent.CheckREff(rEff, assurance.F); rEffErr != nil && meta.MeasureVerdict == "accepted" {
			logger.Warn().Str("component", "agent").
				Float64("r_eff", rEff).
				Msg("agent.low_reff_warning")
			// Inject warning but don't block — agent can still close with low R_eff
			warningMsg := agent.Message{
				Role:  agent.RoleSystem,
				Parts: []agent.Part{agent.TextPart{Text: fmt.Sprintf("[FPF Warning] %s", rEffErr.Error())}},
			}
			_ = c.Messages.Save(ctx, &warningMsg)
		}

		switch meta.MeasureVerdict {
		case "accepted", "partial":
			completed := agent.CompleteCycle(updated, wlnk, assurance)
			_ = c.Cycles.UpdateCycle(ctx, completed)
			sess.ActiveCycleID = ""
			_ = c.Sessions.Update(ctx, sess)
			c.sendCycleUpdate(completed)
			evCount := 0
			if c.evidence != nil {
				evCount = len(c.evidence.Items)
			}

			// Outer cycle trigger: suggest reframe after completion (FPF lemniscate feedback)
			outerMsg := agent.Message{
				Role: agent.RoleSystem,
				Parts: []agent.Part{agent.TextPart{Text: fmt.Sprintf(
					"[Cycle complete] Problem solved (R_eff: %.2f, %d evidence items). "+
						"The lemniscate closes here. Situation may have changed — "+
						"if the user has more tasks, frame a new problem. "+
						"If the solution revealed new issues, suggest reframing.",
					rEff, evCount)}},
			}
			_ = c.Messages.Save(ctx, &agent.Message{
				ID: newMsgID(), SessionID: sess.ID, Role: agent.RoleSystem,
				Parts: outerMsg.Parts, CreatedAt: time.Now().UTC(),
			})

			c.evidence = nil
			logger.Info().Str("component", "agent").
				Str("cycle_id", completed.ID).
				Str("verdict", meta.MeasureVerdict).
				Float64("r_eff", rEff).
				Int("evidence_items", evCount).
				Msg("agent.cycle_completed")

		case "failed":
			abandoned := agent.AbandonCycle(updated)
			_ = c.Cycles.UpdateCycle(ctx, abandoned)
			// Create new cycle from lineage for reframing
			newCycle := agent.NewCycleFromLineage("cyc_"+uuid.NewString(), sess.ID, abandoned)
			_ = c.Cycles.CreateCycle(ctx, newCycle)
			sess.ActiveCycleID = newCycle.ID
			_ = c.Sessions.Update(ctx, sess)
			c.sendCycleUpdate(newCycle)
			c.evidence = nil
			logger.Info().Str("component", "agent").
				Str("failed_cycle", abandoned.ID).
				Str("new_cycle", newCycle.ID).
				Float64("r_eff", rEff).
				Msg("agent.cycle_reframed")
		}
	}
}

// getRepoMap returns the cached repo map, rebuilding if dirty or first call.
func (c *Coordinator) getRepoMap() string {
	if c.ProjectRoot == "" {
		return ""
	}
	if c.repoMapCache == "" || c.repoMapDirty {
		rm, err := codebase.BuildRepoMap(c.ProjectRoot, 500)
		if err != nil || rm == nil || rm.TotalFiles == 0 {
			return ""
		}
		c.repoMapCache = codebase.RenderRepoMap(rm, 2000)
		c.repoMapDirty = false
	}
	return c.repoMapCache
}

// invalidateRepoMap marks the repo map as needing rebuild.
// Called after edit/write tool modifies files.
func (c *Coordinator) invalidateRepoMap() {
	c.repoMapDirty = true
}

// injectDriftWarnings runs drift detection on first turn and injects
// warnings about decisions whose governed files have changed.
func (c *Coordinator) injectDriftWarnings(ctx context.Context, history *[]agent.Message) {
	reports, err := artifact.CheckDrift(ctx, c.ArtifactStore, c.ProjectRoot)
	if err != nil || len(reports) == 0 {
		return
	}

	drifted := 0
	for _, r := range reports {
		if len(r.Files) > 0 {
			drifted++
		}
	}
	if drifted == 0 {
		return
	}

	// Keep drift injection minimal — just a count, not full list.
	// Full details available via /refresh command.
	msg := agent.Message{
		Role: agent.RoleSystem,
		Parts: []agent.Part{agent.TextPart{Text: fmt.Sprintf(
			"[Drift] %d decision(s) have files changed since baseline. Use /refresh for details.",
			drifted)}},
	}
	*history = append(*history, msg)

	logger.Info().Str("component", "agent").
		Int("drifted", drifted).
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

	warning := fmt.Sprintf("[Overseer] ⚠ File %s is governed by: %s. "+
		"Be careful — verify that your edit preserves decision invariants. "+
		"If you're unsure, read the decision first with haft_query(search).",
		filePath, strings.Join(parts, ", "))

	logger.Info().Str("component", "overseer").
		Str("file", filePath).
		Int("decisions", len(parts)).
		Msg("overseer.invariant_warning")

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
	// Look up problem title for human-readable display
	var problemTitle string
	if cycle.ProblemRef != "" && c.ArtifactStore != nil {
		if a, err := c.ArtifactStore.Get(context.Background(), cycle.ProblemRef); err == nil {
			problemTitle = a.Meta.Title
		}
	}

	c.Bus.SendCycleUpdate(protocol.CycleUpdate{
		CycleID:      cycle.ID,
		ProblemRef:   cycle.ProblemRef,
		ProblemTitle: problemTitle,
		PortfolioRef: cycle.PortfolioRef,
		DecisionRef:  cycle.DecisionRef,
		Phase:        string(cycle.Phase),
		Status:       string(cycle.Status),
		REff:         cycle.REff,
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

func textFromParts(parts []agent.Part) string {
	var b strings.Builder
	for _, part := range parts {
		switch p := part.(type) {
		case agent.TextPart:
			b.WriteString(p.Text)
		case agent.ImagePart:
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString("[image attachment: ")
			b.WriteString(p.Filename)
			b.WriteString("]")
		}
	}
	return b.String()
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
	c.Bus.SendSessionTitle(protocol.SessionTitle{Title: title})
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

func (c *Coordinator) recordSystemWarning(ctx context.Context, sessionID, text string) {
	if c.Messages == nil || sessionID == "" || strings.TrimSpace(text) == "" {
		return
	}

	_ = c.Messages.Save(ctx, &agent.Message{
		ID:        newMsgID(),
		SessionID: sessionID,
		Role:      agent.RoleSystem,
		Parts:     []agent.Part{agent.TextPart{Text: text}},
		CreatedAt: time.Now().UTC(),
	})
}

func (c *Coordinator) computeClosedCycleAssurance(
	ctx context.Context,
	decisionRef string,
	chain *agent.EvidenceChain,
) (agent.AssuranceTuple, string, error) {
	assuranceTuple := agent.ComputeAssurance(chain)
	weakestLink := weakestChainLink(chain)

	if c.ArtifactStore == nil || decisionRef == "" {
		return assuranceTuple, weakestLink, nil
	}

	dbStore, ok := c.ArtifactStore.(interface{ DB() *sql.DB })
	if !ok {
		return assuranceTuple, weakestLink, fmt.Errorf("artifact store does not expose SQL access")
	}

	decision, err := c.ArtifactStore.Get(ctx, decisionRef)
	if err != nil {
		return assuranceTuple, weakestLink, fmt.Errorf("load decision %s: %w", decisionRef, err)
	}

	activeEvidence, err := c.syncDecisionAssuranceGraph(ctx, dbStore.DB(), decision)
	if err != nil {
		return assuranceTuple, weakestLink, err
	}

	report, err := assurance.New(dbStore.DB()).CalculateReliability(ctx, decisionRef)
	if err != nil {
		return assuranceTuple, weakestLink, fmt.Errorf("calculate dependency-aware reliability: %w", err)
	}

	assuranceTuple.F = report.FormalityScore
	assuranceTuple.G = claimScopeUnion(activeEvidence)
	assuranceTuple.R = report.FinalScore

	if report.WeakestLink != "" {
		weakestLink = "dependency " + report.WeakestLink
	}
	if weakestLink == "" {
		weakestLink = weakestPersistedEvidence(activeEvidence)
	}

	return assuranceTuple, weakestLink, nil
}

func (c *Coordinator) syncDecisionAssuranceGraph(
	ctx context.Context,
	db *sql.DB,
	decision *artifact.Artifact,
) ([]artifact.EvidenceItem, error) {
	if decision == nil {
		return nil, fmt.Errorf("decision artifact is nil")
	}

	evidenceItems, err := c.ArtifactStore.GetEvidenceItems(ctx, decision.Meta.ID)
	if err != nil {
		return nil, fmt.Errorf("load decision evidence: %w", err)
	}

	activeEvidence := activeArtifactEvidence(evidenceItems)
	now := time.Now().UTC()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin assurance sync: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = c.ensureDecisionHolon(ctx, tx, decision, now)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx,
		`DELETE FROM evidence WHERE holon_id = ? AND assurance_level = ?`,
		decision.Meta.ID,
		assuranceAdapterLevel,
	)
	if err != nil {
		return nil, fmt.Errorf("clear adapter evidence: %w", err)
	}

	for _, item := range activeEvidence {
		scopeJSON := "[]"
		if len(item.ClaimScope) > 0 {
			data, err := json.Marshal(item.ClaimScope)
			if err != nil {
				return nil, fmt.Errorf("marshal claim_scope for %s: %w", item.ID, err)
			}
			scopeJSON = string(data)
		}

		validUntil, err := parseAssuranceValidUntil(item.ValidUntil, decision.Meta.ValidUntil)
		if err != nil {
			return nil, fmt.Errorf("parse valid_until for %s: %w", item.ID, err)
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO evidence (
				id, holon_id, type, content, verdict, assurance_level, formality_level,
				claim_scope, carrier_ref, valid_until, created_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"artifact:"+item.ID,
			decision.Meta.ID,
			item.Type,
			item.Content,
			item.Verdict,
			assuranceAdapterLevel,
			item.FormalityLevel,
			scopeJSON,
			nullIfEmpty(item.CarrierRef),
			validUntil,
			now.Format(time.RFC3339),
		)
		if err != nil {
			return nil, fmt.Errorf("persist assurance evidence %s: %w", item.ID, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("commit assurance sync: %w", err)
	}

	return activeEvidence, nil
}

func (c *Coordinator) ensureDecisionHolon(
	ctx context.Context,
	tx *sql.Tx,
	decision *artifact.Artifact,
	now time.Time,
) error {
	affectedFiles, err := c.ArtifactStore.GetAffectedFiles(ctx, decision.Meta.ID)
	if err != nil {
		return fmt.Errorf("load affected files for %s: %w", decision.Meta.ID, err)
	}

	scopeParts := make([]string, 0, len(affectedFiles))
	for _, file := range affectedFiles {
		scopeParts = append(scopeParts, file.Path)
	}
	sort.Strings(scopeParts)

	contextID := strings.TrimSpace(decision.Meta.Context)
	if contextID == "" {
		contextID = "default"
	}

	createdAt := decision.Meta.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO holons (
			id, type, kind, layer, title, content, context_id, scope, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			kind = excluded.kind,
			layer = excluded.layer,
			title = excluded.title,
			content = excluded.content,
			context_id = excluded.context_id,
			scope = excluded.scope,
			updated_at = excluded.updated_at`,
		decision.Meta.ID,
		"DRR",
		string(artifact.KindDecisionRecord),
		"DRR",
		decision.Meta.Title,
		decision.Body,
		contextID,
		strings.Join(scopeParts, "\n"),
		createdAt.Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("upsert decision holon %s: %w", decision.Meta.ID, err)
	}

	return nil
}

func activeArtifactEvidence(items []artifact.EvidenceItem) []artifact.EvidenceItem {
	active := make([]artifact.EvidenceItem, 0, len(items))

	for _, item := range items {
		if strings.EqualFold(item.Verdict, "superseded") {
			continue
		}
		active = append(active, item)
	}

	return active
}

func claimScopeUnion(items []artifact.EvidenceItem) []string {
	seen := make(map[string]struct{})
	union := make([]string, 0)

	for _, item := range items {
		for _, scope := range item.ClaimScope {
			scope = strings.TrimSpace(scope)
			if scope == "" {
				continue
			}
			if _, ok := seen[scope]; ok {
				continue
			}
			seen[scope] = struct{}{}
			union = append(union, scope)
		}
	}

	sort.Strings(union)
	if len(union) == 0 {
		return nil
	}

	return union
}

func weakestChainLink(chain *agent.EvidenceChain) string {
	if chain == nil {
		return ""
	}

	minScore := 1.0
	weakest := ""

	for _, item := range chain.Items {
		if !item.Type.IsExplicitEvidence() {
			continue
		}

		score := item.BaseScore - clPenalty(item.CL)
		if score >= minScore {
			continue
		}

		minScore = score
		weakest = string(item.Type)
	}

	if weakest == "" {
		return ""
	}

	return weakest + fmt.Sprintf(" (score: %.1f)", minScore)
}

func weakestPersistedEvidence(items []artifact.EvidenceItem) string {
	minScore := 1.0
	weakest := ""
	now := time.Now().UTC()

	for _, item := range items {
		score := reff.ScoreEvidence(item.Verdict, item.CongruenceLevel, item.ValidUntil, now)
		if score >= minScore {
			continue
		}

		minScore = score
		label := strings.TrimSpace(item.Type)
		if label == "" {
			label = "evidence"
		}
		weakest = label
	}

	if weakest == "" {
		return ""
	}

	return weakest + fmt.Sprintf(" (score: %.1f)", minScore)
}

func parseAssuranceValidUntil(primary string, fallback string) (any, error) {
	for _, candidate := range []string{primary, fallback} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}

		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			parsed, err := time.Parse(layout, candidate)
			if err == nil {
				return parsed.UTC(), nil
			}
		}

		return nil, fmt.Errorf("unsupported timestamp %q", candidate)
	}

	return nil, nil
}

func nullIfEmpty(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return value
}

func clPenalty(cl int) float64 {
	switch cl {
	case 3:
		return 0.0
	case 2:
		return 0.1
	case 1:
		return 0.4
	default:
		return 0.9
	}
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

// msgToUpdate converts an agent.Message to a protocol.MsgUpdate for the TUI.
func msgToUpdate(msg *agent.Message, streaming bool) protocol.MsgUpdate {
	var thinking string
	var toolCalls []protocol.ToolCall
	for _, p := range msg.Parts {
		switch tp := p.(type) {
		case agent.TextPart:
			if strings.HasPrefix(tp.Text, "[thinking]") {
				if thinking != "" {
					thinking += "\n"
				}
				thinking += strings.TrimPrefix(tp.Text, "[thinking]")
			}
		case agent.ToolCallPart:
			toolCalls = append(toolCalls, protocol.ToolCall{
				CallID:  tp.ToolCallID,
				Name:    tp.ToolName,
				Args:    tp.Arguments,
				Running: streaming,
			})
		}
	}
	return protocol.MsgUpdate{
		ID:        msg.ID,
		Text:      msg.Text(),
		Thinking:  thinking,
		Tools:     toolCalls,
		Streaming: streaming,
	}
}
