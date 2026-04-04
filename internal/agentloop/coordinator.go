package agentloop

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

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

	assuranceAdapterLevel       = "artifact_adapter"
	cycleAssuranceLevel         = "cycle_adapter"
	manualDependencyRelation    = "dependsOn"
	projectedDependencyRelation = "dependsOnProjected"
)

type assuranceEvidenceRecord struct {
	artifact.EvidenceItem
	AssuranceLevel string
	CreatedAt      time.Time
}

type dependencyProjection struct {
	Available bool
	Refs      []string
}

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
		c.Tools.SetCycleUpdater(func(ctx context.Context, cycle *agent.Cycle) error {
			return c.Cycles.UpdateCycle(ctx, cycle)
		})

		// Wire Transformer Mandate decision-boundary checker.
		// Compare remains callable without another user response; the pause is
		// enforced only at compare -> decide unless autonomous mode is active.
		c.Tools.SetDecisionBoundaryChecker(func(_ context.Context, cycle *agent.Cycle) (bool, error) {
			return decisionBoundarySatisfied(sess.Interaction, cycle), nil
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
	if err := c.captureDecisionSelection(ctx, sess.ID, userMsg.Text()); err != nil {
		c.Bus.SendError(fmt.Sprintf("persist decision selection: %s", err))
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

	systemPrompt += interactionModePrompt(sess.Interaction)

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

func interactionModePrompt(interaction agent.Interaction) string {
	if interaction != agent.InteractionAutonomous {
		return ""
	}

	return `

## [MODE: AUTONOMOUS — ACTIVE NOW]

The user explicitly toggled autonomous mode. This OVERRIDES the collaborative workflow rules above.

IN AUTONOMOUS MODE YOU MUST:
- ACT, don't describe. Never say "I'll do X" — just DO X.
- CHAIN all phases in one turn: frame → explore → compare → decide → implement → measure.
- EXECUTE tool calls immediately. Don't read 3 files then explain what you found — read, act, continue.
- SKIP all "STOP and present" checkpoints. The user delegated — they trust you.
- NEVER ask "shall I proceed?" or "would you like me to..." — the answer is always YES.
- When the user says "do it" or "давай" — that means START WORKING NOW, not "explain your plan."

The Transformer Mandate still applies to DECISIONS (user's choices/directions are authoritative).
But it does NOT apply to EXECUTION STEPS — you don't need approval for each tool call, each file read, each edit. Just do the work.`
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
		if level == agent.PermissionNeedsApproval && !sess.Yolo {
			reply, err := c.Bus.AskPermission(protocol.PermissionAsk{
				ToolName: tc.ToolName,
				Args:     tc.Arguments,
			})
			if err != nil || reply.Action == "deny" {
				output = "Permission denied by user."
				isError = true
			} else if reply.Action == "allow_session" || reply.Yolo {
				sess.Yolo = true
				_ = c.Sessions.Update(ctx, sess)
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
			c.recordSystemWarning(ctx, sess.ID, fmt.Sprintf("[FPF Warning] %s", rEffErr.Error()))
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

func decisionBoundarySatisfied(interaction agent.Interaction, cycle *agent.Cycle) bool {
	if interaction == agent.InteractionAutonomous {
		return true
	}
	return agent.HasDecisionSelection(cycle)
}

func (c *Coordinator) saveToolResult(ctx context.Context, sess *agent.Session, callID, toolName, output string, isError bool, history *[]agent.Message) {
	msg := &agent.Message{
		ID: newMsgID(), SessionID: sess.ID, Role: agent.RoleTool,
		Parts: []agent.Part{agent.ToolResultPart{
			ToolCallID: callID,
			ToolName:   toolName,
			Content:    output,
			IsError:    isError,
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

func (c *Coordinator) captureDecisionSelection(ctx context.Context, sessionID, userText string) error {
	if c.Cycles == nil || c.ArtifactStore == nil {
		return nil
	}

	if normalizeDecisionSelectionText(userText) == "" {
		return nil
	}

	cycle, err := c.Cycles.GetActiveCycle(ctx, sessionID)
	if err != nil || cycle == nil {
		return err
	}
	if cycle.ComparedPortfolioRef == "" || cycle.ComparedPortfolioRef != cycle.PortfolioRef {
		return nil
	}

	portfolio, err := c.ArtifactStore.Get(ctx, cycle.ComparedPortfolioRef)
	if err != nil {
		return nil
	}

	selectedVariantRef, ok := detectExplicitDecisionSelection(userText, selectionCandidatesForPortfolio(portfolio))
	if !ok {
		return nil
	}
	if cycle.SelectedPortfolioRef == cycle.ComparedPortfolioRef && cycle.SelectedVariantRef == selectedVariantRef {
		return nil
	}

	updated := *cycle
	updated.SelectedPortfolioRef = cycle.ComparedPortfolioRef
	updated.SelectedVariantRef = selectedVariantRef
	return c.Cycles.UpdateCycle(ctx, &updated)
}

type decisionSelectionCandidate struct {
	VariantRef string
	Aliases    []string
}

func selectionCandidatesForPortfolio(portfolio *artifact.Artifact) []decisionSelectionCandidate {
	fields := portfolio.UnmarshalPortfolioFields()
	candidates := make([]decisionSelectionCandidate, 0, len(fields.Variants))

	for _, variant := range fields.Variants {
		variantRef := strings.TrimSpace(variant.ID)
		if variantRef == "" {
			variantRef = strings.TrimSpace(variant.Title)
		}
		if variantRef == "" {
			continue
		}

		aliases := []string{
			variant.ID,
			variant.Title,
			"variant " + variant.ID,
			"option " + variant.ID,
			"variant " + variant.Title,
			"option " + variant.Title,
		}

		candidates = append(candidates, decisionSelectionCandidate{
			VariantRef: variantRef,
			Aliases:    normalizeDecisionSelectionAliases(aliases),
		})
	}

	return candidates
}

func detectExplicitDecisionSelection(message string, candidates []decisionSelectionCandidate) (string, bool) {
	normalizedMessage := normalizeDecisionSelectionText(message)
	if normalizedMessage == "" {
		return "", false
	}
	if strings.Contains(message, "?") {
		return "", false
	}

	matches := matchingDecisionSelectionRefs(normalizedMessage, candidates)
	if len(matches) != 1 {
		return "", false
	}

	selectedRef := matches[0]
	if isExactDecisionSelectionAlias(normalizedMessage, selectedRef, candidates) {
		return selectedRef, true
	}

	trimmedMessage := trimDecisionSelectionLeadIn(normalizedMessage)
	if hasNegativeDecisionSelectionPrefix(trimmedMessage) {
		return "", false
	}
	if !hasPositiveDecisionSelectionPrefix(trimmedMessage) {
		return "", false
	}

	return selectedRef, true
}

func matchingDecisionSelectionRefs(message string, candidates []decisionSelectionCandidate) []string {
	paddedMessage := " " + message + " "
	matches := make([]string, 0, len(candidates))

	for _, candidate := range candidates {
		for _, alias := range candidate.Aliases {
			if alias == "" {
				continue
			}
			if strings.Contains(paddedMessage, " "+alias+" ") {
				matches = append(matches, candidate.VariantRef)
				break
			}
		}
	}

	return matches
}

func isExactDecisionSelectionAlias(message, selectedRef string, candidates []decisionSelectionCandidate) bool {
	for _, candidate := range candidates {
		if candidate.VariantRef != selectedRef {
			continue
		}
		for _, alias := range candidate.Aliases {
			if message == alias {
				return true
			}
		}
	}

	return false
}

func normalizeDecisionSelectionAliases(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	aliases := make([]string, 0, len(values))

	for _, value := range values {
		normalized := normalizeDecisionSelectionText(value)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		aliases = append(aliases, normalized)
	}

	return aliases
}

func normalizeDecisionSelectionText(value string) string {
	lowered := strings.ToLower(strings.TrimSpace(value))
	cleaned := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r):
			return r
		case unicode.IsSpace(r):
			return ' '
		default:
			return ' '
		}
	}, lowered)

	return strings.Join(strings.Fields(cleaned), " ")
}

func trimDecisionSelectionLeadIn(value string) string {
	leadIns := []string{"ok ", "okay ", "so ", "then ", "actually ", "alright ", "all right "}
	trimmed := value

	for {
		updated := trimmed
		for _, leadIn := range leadIns {
			if strings.HasPrefix(updated, leadIn) {
				updated = strings.TrimSpace(strings.TrimPrefix(updated, leadIn))
				break
			}
		}
		if updated == trimmed {
			return trimmed
		}
		trimmed = updated
	}
}

func hasNegativeDecisionSelectionPrefix(value string) bool {
	prefixes := []string{
		"dont choose ",
		"do not choose ",
		"dont pick ",
		"do not pick ",
		"dont use ",
		"do not use ",
		"dont go with ",
		"do not go with ",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}

	return false
}

func hasPositiveDecisionSelectionPrefix(value string) bool {
	prefixes := []string{
		"choose ",
		"i choose ",
		"we choose ",
		"pick ",
		"i pick ",
		"we pick ",
		"select ",
		"i select ",
		"prefer ",
		"i prefer ",
		"go with ",
		"lets go with ",
		"let s go with ",
		"ship ",
		"use ",
		"proceed with ",
		"move forward with ",
		"lets do ",
		"let s do ",
		"my choice is ",
		"decision is ",
		"we should choose ",
		"we should pick ",
		"we should use ",
		"we should go with ",
		"i think we should choose ",
		"i think we should pick ",
		"i think we should use ",
		"i think we should go with ",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}

	return false
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

	activeEvidence, err := c.syncDecisionAssuranceGraph(ctx, dbStore.DB(), decision, chain, make(map[string]struct{}))
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
	chain *agent.EvidenceChain,
	visited map[string]struct{},
) ([]assuranceEvidenceRecord, error) {
	if decision == nil {
		return nil, fmt.Errorf("decision artifact is nil")
	}
	if _, ok := visited[decision.Meta.ID]; ok {
		return c.loadPersistedAssuranceEvidence(ctx, db, decision.Meta.ID)
	}
	visited[decision.Meta.ID] = struct{}{}

	projection, err := c.resolveDecisionDependencyRefs(ctx, db, decision.Meta.ID)
	if err != nil {
		return nil, err
	}

	dependencyRefs, err := c.activeDecisionDependencyRefs(ctx, db, decision.Meta.ID, projection)
	if err != nil {
		return nil, err
	}

	for _, dependencyRef := range dependencyRefs {
		dependencyDecision, err := c.ArtifactStore.Get(ctx, dependencyRef)
		if err != nil {
			return nil, fmt.Errorf("load dependency decision %s: %w", dependencyRef, err)
		}

		_, err = c.syncDecisionAssuranceGraph(ctx, db, dependencyDecision, nil, visited)
		if err != nil {
			return nil, err
		}
	}

	artifactEvidence, err := c.loadActiveArtifactEvidence(ctx, decision.Meta.ID)
	if err != nil {
		return nil, err
	}

	cycleEvidence, err := c.activeCycleAssuranceEvidence(ctx, db, decision, chain)
	if err != nil {
		return nil, err
	}

	activeEvidence := combineAssuranceEvidence(artifactEvidence, cycleEvidence)
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

	err = c.syncProjectedDecisionRelations(ctx, tx, decision.Meta.ID, projection, now)
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx,
		`DELETE FROM evidence WHERE holon_id = ? AND assurance_level IN (?, ?)`,
		decision.Meta.ID,
		assuranceAdapterLevel,
		cycleAssuranceLevel,
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

		createdAt := item.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO evidence (
				id, holon_id, type, content, verdict, assurance_level, formality_level,
				congruence_level, claim_scope, carrier_ref, valid_until, created_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID,
			decision.Meta.ID,
			item.Type,
			item.Content,
			item.Verdict,
			item.AssuranceLevel,
			item.FormalityLevel,
			item.CongruenceLevel,
			scopeJSON,
			nullIfEmpty(item.CarrierRef),
			validUntil,
			createdAt.Format(time.RFC3339),
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

func (c *Coordinator) loadActiveArtifactEvidence(
	ctx context.Context,
	decisionRef string,
) ([]artifact.EvidenceItem, error) {
	evidenceItems, err := c.ArtifactStore.GetEvidenceItems(ctx, decisionRef)
	if err != nil {
		return nil, fmt.Errorf("load decision evidence: %w", err)
	}

	return activeArtifactEvidence(evidenceItems), nil
}

func (c *Coordinator) loadPersistedAssuranceEvidence(
	ctx context.Context,
	db *sql.DB,
	decisionRef string,
) ([]assuranceEvidenceRecord, error) {
	artifactEvidence, err := c.loadActiveArtifactEvidence(ctx, decisionRef)
	if err != nil {
		return nil, err
	}

	cycleEvidence, err := c.loadPersistedCycleEvidence(ctx, db, decisionRef)
	if err != nil {
		return nil, err
	}

	return combineAssuranceEvidence(artifactEvidence, cycleEvidence), nil
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

func (c *Coordinator) syncProjectedDecisionRelations(
	ctx context.Context,
	tx *sql.Tx,
	decisionRef string,
	projection dependencyProjection,
	now time.Time,
) error {
	if !projection.Available {
		return nil
	}

	manualRefs, err := c.loadDecisionDependencyRefsTx(ctx, tx, decisionRef, manualDependencyRelation)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM relations
		WHERE source_id = ? AND relation_type = ?`,
		decisionRef,
		projectedDependencyRelation,
	)
	if err != nil {
		return fmt.Errorf("clear projected relations for %s: %w", decisionRef, err)
	}

	refsToInsert := subtractDependencyRefs(projection.Refs, manualRefs)

	for _, dependencyRef := range refsToInsert {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO relations (
				source_id, target_id, relation_type, congruence_level, created_at
			)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(source_id, target_id, relation_type) DO NOTHING`,
			decisionRef,
			dependencyRef,
			projectedDependencyRelation,
			3,
			now.Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("project relation %s -> %s: %w", decisionRef, dependencyRef, err)
		}
	}

	return nil
}

func (c *Coordinator) resolveDecisionDependencyRefs(
	ctx context.Context,
	db *sql.DB,
	decisionRef string,
) (dependencyProjection, error) {
	modules, err := codebase.NewScanner(db).GetModules(ctx)
	if isOptionalAssuranceQueryError(err) || len(modules) == 0 {
		return dependencyProjection{}, nil
	}
	if err != nil {
		return dependencyProjection{}, fmt.Errorf("load codebase modules: %w", err)
	}

	currentFiles, err := c.ArtifactStore.GetAffectedFiles(ctx, decisionRef)
	if err != nil {
		return dependencyProjection{}, fmt.Errorf("load affected files for %s: %w", decisionRef, err)
	}

	currentModules := resolveAffectedModules(modules, currentFiles)
	if len(currentModules) == 0 {
		return dependencyProjection{}, nil
	}

	decisionRefs, available, err := listActiveDecisionRefs(ctx, db)
	if err != nil {
		return dependencyProjection{}, err
	}
	if !available {
		return dependencyProjection{}, nil
	}

	dependencySet := make(map[string]struct{})

	for _, candidateRef := range decisionRefs {
		if candidateRef == decisionRef {
			continue
		}

		candidateFiles, err := c.ArtifactStore.GetAffectedFiles(ctx, candidateRef)
		if err != nil {
			return dependencyProjection{}, fmt.Errorf("load affected files for %s: %w", candidateRef, err)
		}

		candidateModules := resolveAffectedModules(modules, candidateFiles)
		if len(candidateModules) == 0 {
			continue
		}

		hasDependency, err := hasModuleDependency(ctx, db, currentModules, candidateModules)
		if isOptionalAssuranceQueryError(err) {
			return dependencyProjection{}, nil
		}
		if err != nil {
			return dependencyProjection{}, fmt.Errorf("check module dependency %s -> %s: %w", decisionRef, candidateRef, err)
		}
		if hasDependency {
			dependencySet[candidateRef] = struct{}{}
		}
	}

	dependencyRefs := make([]string, 0, len(dependencySet))
	for dependencyRef := range dependencySet {
		dependencyRefs = append(dependencyRefs, dependencyRef)
	}
	sort.Strings(dependencyRefs)

	return dependencyProjection{
		Available: true,
		Refs:      dependencyRefs,
	}, nil
}

func resolveAffectedModules(
	modules []codebase.Module,
	affectedFiles []artifact.AffectedFile,
) []string {
	moduleSet := make(map[string]struct{})

	for _, affectedFile := range affectedFiles {
		moduleID := resolveModuleForPath(modules, affectedFile.Path)
		if moduleID == "" {
			continue
		}
		moduleSet[moduleID] = struct{}{}
	}

	moduleIDs := make([]string, 0, len(moduleSet))
	for moduleID := range moduleSet {
		moduleIDs = append(moduleIDs, moduleID)
	}
	sort.Strings(moduleIDs)

	return moduleIDs
}

func resolveModuleForPath(modules []codebase.Module, filePath string) string {
	bestMatch := ""
	bestLen := -1

	for _, module := range modules {
		prefix := module.Path
		if prefix == "" {
			if bestLen < 0 {
				bestMatch = module.ID
				bestLen = 0
			}
			continue
		}

		hasPrefix := strings.HasPrefix(filePath, prefix+"/")
		hasSeparatorPrefix := strings.HasPrefix(filePath, prefix+string(filepath.Separator))
		if !hasPrefix && !hasSeparatorPrefix {
			continue
		}
		if len(prefix) <= bestLen {
			continue
		}

		bestMatch = module.ID
		bestLen = len(prefix)
	}

	return bestMatch
}

func hasModuleDependency(
	ctx context.Context,
	db *sql.DB,
	sourceModules []string,
	targetModules []string,
) (bool, error) {
	for _, sourceModule := range sourceModules {
		for _, targetModule := range targetModules {
			var exists int
			err := db.QueryRowContext(ctx, `
				SELECT 1
				FROM module_dependencies
				WHERE source_module = ? AND target_module = ?
				LIMIT 1`,
				sourceModule,
				targetModule,
			).Scan(&exists)
			if err == nil {
				return true, nil
			}
			if err == sql.ErrNoRows {
				continue
			}
			return false, err
		}
	}

	return false, nil
}

func listActiveDecisionRefs(ctx context.Context, db *sql.DB) ([]string, bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id
		FROM artifacts
		WHERE kind = ? AND status = ?`,
		string(artifact.KindDecisionRecord),
		string(artifact.StatusActive),
	)
	if isOptionalAssuranceQueryError(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("list active decision refs: %w", err)
	}
	defer rows.Close()

	refs := make([]string, 0)
	for rows.Next() {
		var ref string
		err = rows.Scan(&ref)
		if err != nil {
			return nil, false, fmt.Errorf("scan active decision ref: %w", err)
		}
		refs = append(refs, ref)
	}

	err = rows.Err()
	if err != nil {
		return nil, false, fmt.Errorf("iterate active decision refs: %w", err)
	}

	return refs, true, nil
}

func isOptionalAssuranceQueryError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "no such table")
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

func combineAssuranceEvidence(
	artifactItems []artifact.EvidenceItem,
	cycleItems []assuranceEvidenceRecord,
) []assuranceEvidenceRecord {
	records := make([]assuranceEvidenceRecord, 0, len(artifactItems)+len(cycleItems))

	for _, item := range artifactItems {
		records = append(records, assuranceEvidenceRecord{
			EvidenceItem: artifact.EvidenceItem{
				ID:              "artifact:" + item.ID,
				Type:            item.Type,
				Content:         item.Content,
				Verdict:         item.Verdict,
				CarrierRef:      item.CarrierRef,
				CongruenceLevel: item.CongruenceLevel,
				FormalityLevel:  item.FormalityLevel,
				ClaimScope:      append([]string(nil), item.ClaimScope...),
				ValidUntil:      item.ValidUntil,
			},
			AssuranceLevel: assuranceAdapterLevel,
		})
	}

	records = append(records, cycleItems...)
	return records
}

func (c *Coordinator) activeCycleAssuranceEvidence(
	ctx context.Context,
	db *sql.DB,
	decision *artifact.Artifact,
	chain *agent.EvidenceChain,
) ([]assuranceEvidenceRecord, error) {
	if chain != nil {
		return buildCycleAssuranceEvidence(decision, chain), nil
	}

	return c.loadPersistedCycleEvidence(ctx, db, decision.Meta.ID)
}

func (c *Coordinator) loadPersistedCycleEvidence(
	ctx context.Context,
	db *sql.DB,
	decisionRef string,
) ([]assuranceEvidenceRecord, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, type, content, verdict, COALESCE(carrier_ref, ''), congruence_level,
			formality_level, claim_scope, COALESCE(valid_until, ''), COALESCE(created_at, '')
		FROM evidence
		WHERE holon_id = ? AND assurance_level = ?
		ORDER BY id`,
		decisionRef,
		cycleAssuranceLevel,
	)
	if isOptionalAssuranceQueryError(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load persisted cycle evidence for %s: %w", decisionRef, err)
	}
	defer rows.Close()

	records := make([]assuranceEvidenceRecord, 0)

	for rows.Next() {
		var record assuranceEvidenceRecord
		var claimScopeJSON string
		var createdAtRaw string

		err = rows.Scan(
			&record.ID,
			&record.Type,
			&record.Content,
			&record.Verdict,
			&record.CarrierRef,
			&record.CongruenceLevel,
			&record.FormalityLevel,
			&claimScopeJSON,
			&record.ValidUntil,
			&createdAtRaw,
		)
		if err != nil {
			return nil, fmt.Errorf("scan persisted cycle evidence for %s: %w", decisionRef, err)
		}

		record.AssuranceLevel = cycleAssuranceLevel
		record.Type = normalizePersistedCycleEvidenceType(record.Type, record.Verdict)
		record.ClaimScope = decodeAssuranceClaimScope(claimScopeJSON)
		record.CreatedAt = parseAssuranceCreatedAt(createdAtRaw)
		records = append(records, record)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("iterate persisted cycle evidence for %s: %w", decisionRef, err)
	}

	return records, nil
}

func buildCycleAssuranceEvidence(
	decision *artifact.Artifact,
	chain *agent.EvidenceChain,
) []assuranceEvidenceRecord {
	if decision == nil || chain == nil {
		return nil
	}
	if strings.TrimSpace(chain.DecRef) != "" && chain.DecRef != decision.Meta.ID {
		return nil
	}

	explicitIndex := 0
	records := make([]assuranceEvidenceRecord, 0, len(chain.Items))

	for _, item := range chain.Items {
		if !item.Type.IsExplicitEvidence() {
			continue
		}

		explicitIndex++
		records = append(records, assuranceEvidenceRecord{
			EvidenceItem: artifact.EvidenceItem{
				ID:              cycleAssuranceEvidenceID(decision, chain, explicitIndex),
				Type:            cycleEvidenceType(item),
				Content:         cycleEvidenceContent(item),
				Verdict:         cycleEvidenceVerdict(item),
				CarrierRef:      cycleEvidenceCarrierRef(chain),
				CongruenceLevel: item.CL,
				FormalityLevel:  item.Formality,
				ClaimScope:      append([]string(nil), item.ClaimScope...),
				ValidUntil:      decision.Meta.ValidUntil,
			},
			AssuranceLevel: cycleAssuranceLevel,
			CreatedAt:      item.CapturedAt,
		})
	}

	return records
}

func cycleAssuranceEvidenceID(
	decision *artifact.Artifact,
	chain *agent.EvidenceChain,
	explicitIndex int,
) string {
	prefix := decision.Meta.ID
	if chain != nil && strings.TrimSpace(chain.CycleRef) != "" {
		prefix = chain.CycleRef
	}

	return fmt.Sprintf("cycle:%s:%03d", prefix, explicitIndex)
}

func cycleEvidenceType(item agent.EvidenceItem) string {
	switch item.Type {
	case agent.EvidenceMeasure:
		return string(agent.EvidenceMeasure)
	case agent.EvidencePartial:
		return string(agent.EvidencePartial)
	case agent.EvidenceAttached:
		return string(agent.EvidenceAttached)
	default:
		return string(item.Type)
	}
}

func cycleEvidenceContent(item agent.EvidenceItem) string {
	content := strings.TrimSpace(item.Detail)
	if content != "" {
		return content
	}

	return string(item.Type)
}

func cycleEvidenceVerdict(item agent.EvidenceItem) string {
	switch item.Type {
	case agent.EvidenceMeasure:
		return "accepted"
	case agent.EvidencePartial:
		return "partial"
	default:
		if item.BaseScore <= 0 {
			return "failed"
		}
		if item.BaseScore >= 1 {
			return "accepted"
		}
		return "partial"
	}
}

func cycleEvidenceCarrierRef(chain *agent.EvidenceChain) string {
	if chain == nil {
		return ""
	}

	carrierRef := strings.TrimSpace(chain.CycleRef)
	if carrierRef == "" {
		carrierRef = strings.TrimSpace(chain.DecRef)
	}
	if carrierRef == "" {
		return ""
	}

	return "cycle:" + carrierRef
}

func normalizePersistedCycleEvidenceType(evidenceType string, verdict string) string {
	switch strings.ToLower(strings.TrimSpace(evidenceType)) {
	case "measurement":
		if strings.EqualFold(strings.TrimSpace(verdict), "partial") {
			return string(agent.EvidencePartial)
		}
		return string(agent.EvidenceMeasure)
	default:
		return evidenceType
	}
}

func (c *Coordinator) activeDecisionDependencyRefs(
	ctx context.Context,
	db *sql.DB,
	decisionRef string,
	projection dependencyProjection,
) ([]string, error) {
	manualRefs, err := c.loadDecisionDependencyRefs(ctx, db, decisionRef, manualDependencyRelation)
	if err != nil {
		return nil, err
	}

	projectedRefs, err := c.loadDecisionDependencyRefs(ctx, db, decisionRef, projectedDependencyRelation)
	if err != nil {
		return nil, err
	}
	if !projection.Available {
		return mergeDependencyRefs(manualRefs, projectedRefs), nil
	}

	return mergeDependencyRefs(manualRefs, projection.Refs), nil
}

func (c *Coordinator) loadDecisionDependencyRefs(
	ctx context.Context,
	db *sql.DB,
	decisionRef string,
	relationType string,
) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT target_id
		FROM relations
		WHERE source_id = ? AND relation_type = ?
		ORDER BY target_id`,
		decisionRef,
		relationType,
	)
	if isOptionalAssuranceQueryError(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load %s dependency refs for %s: %w", relationType, decisionRef, err)
	}
	defer rows.Close()

	refs := make([]string, 0)

	for rows.Next() {
		var ref string
		err = rows.Scan(&ref)
		if err != nil {
			return nil, fmt.Errorf("scan %s dependency ref for %s: %w", relationType, decisionRef, err)
		}
		refs = append(refs, ref)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("iterate %s dependency refs for %s: %w", relationType, decisionRef, err)
	}

	return refs, nil
}

func (c *Coordinator) loadDecisionDependencyRefsTx(
	ctx context.Context,
	tx *sql.Tx,
	decisionRef string,
	relationType string,
) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT target_id
		FROM relations
		WHERE source_id = ? AND relation_type = ?
		ORDER BY target_id`,
		decisionRef,
		relationType,
	)
	if isOptionalAssuranceQueryError(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load %s dependency refs for %s: %w", relationType, decisionRef, err)
	}
	defer rows.Close()

	refs := make([]string, 0)

	for rows.Next() {
		var ref string
		err = rows.Scan(&ref)
		if err != nil {
			return nil, fmt.Errorf("scan %s dependency ref for %s: %w", relationType, decisionRef, err)
		}
		refs = append(refs, ref)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("iterate %s dependency refs for %s: %w", relationType, decisionRef, err)
	}

	return refs, nil
}

func mergeDependencyRefs(left []string, right []string) []string {
	refSet := make(map[string]struct{})
	merged := make([]string, 0, len(left)+len(right))

	for _, ref := range append(append([]string{}, left...), right...) {
		if strings.TrimSpace(ref) == "" {
			continue
		}
		if _, ok := refSet[ref]; ok {
			continue
		}
		refSet[ref] = struct{}{}
		merged = append(merged, ref)
	}

	sort.Strings(merged)
	return merged
}

func subtractDependencyRefs(left []string, right []string) []string {
	existing := make(map[string]struct{}, len(right))
	missing := make([]string, 0, len(left))

	for _, ref := range right {
		existing[ref] = struct{}{}
	}

	for _, ref := range left {
		if _, ok := existing[ref]; ok {
			continue
		}
		missing = append(missing, ref)
	}

	sort.Strings(missing)
	return missing
}

func claimScopeUnion(items []assuranceEvidenceRecord) []string {
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

func weakestPersistedEvidence(items []assuranceEvidenceRecord) string {
	minScore := 1.0
	weakest := ""
	now := time.Now().UTC()

	for _, item := range items {
		score := reff.ScoreTypedEvidence(item.Type, item.Verdict, item.CongruenceLevel, item.ValidUntil, now)
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

func decodeAssuranceClaimScope(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var scopes []string
	err := json.Unmarshal([]byte(raw), &scopes)
	if err != nil {
		return nil
	}

	return scopes
}

func parseAssuranceCreatedAt(raw string) time.Time {
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST",
	} {
		parsed, err := time.Parse(layout, strings.TrimSpace(raw))
		if err == nil {
			return parsed.UTC()
		}
	}

	return time.Time{}
}

func parseAssuranceValidUntil(primary string, fallback string) (any, error) {
	for _, candidate := range []string{primary, fallback} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}

		parsed, ok := reff.ParseValidUntil(candidate)
		if ok {
			return parsed.UTC(), nil
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
