package agentloop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/internal/agent"
	"github.com/m0n0x41d/quint-code/internal/artifact"
	"github.com/m0n0x41d/quint-code/internal/provider"
	"github.com/m0n0x41d/quint-code/logger"
)

// compactContext runs the 3-stage compaction pipeline.
// Returns the compacted history. Called from reactLoop when context is running low.
//
// Stage 1 (≥70%): Observation masking — prune old tool outputs (free, no LLM)
// Stage 2 (≥85%): Artifact-anchored LLM compaction — spawn compact subagent
// Stage 3 (fallback): Emergency truncation — drop oldest messages
func (c *Coordinator) compactContext(
	ctx context.Context,
	sess *agent.Session,
	fullHistory []agent.Message,
	tokenBudget agent.TokenBudget,
) ([]agent.Message, bool) {
	stage := agent.CompactionStage(tokenBudget.Used, tokenBudget.Limit)
	if stage == 0 {
		return fullHistory, false
	}

	logger.Info().Str("component", "agent").
		Int("stage", stage).
		Int("used", tokenBudget.Used).
		Int("limit", tokenBudget.Limit).
		Msg("agent.compaction_triggered")

	// Stage 1: observation masking (always run first)
	result := agent.PruneOldToolOutputs(fullHistory, agent.ProtectLastN)

	if stage < 2 {
		return result, true
	}

	// Stage 2: artifact-anchored LLM compaction
	anchor := c.buildFullAnchor(ctx, sess, result)
	if anchor != "" {
		result = agent.BuildCompactedHistory(result, anchor, agent.ProtectLastN)
		logger.Info().Str("component", "agent").
			Int("messages_after", len(result)).
			Msg("agent.compaction_stage2_complete")
		return result, true
	}

	// Stage 2 failed (no artifacts, no LLM response) — fall through to Stage 3
	logger.Warn().Str("component", "agent").Msg("agent.compaction_stage2_failed_fallback")

	// Stage 3: emergency truncation — keep system + last N only
	return emergencyTruncate(result, agent.ProtectLastN), true
}

// buildFullAnchor constructs the complete compaction anchor:
// artifact sections (lossless, from .quint/) + LLM summary (for conversation gaps).
func (c *Coordinator) buildFullAnchor(
	ctx context.Context,
	sess *agent.Session,
	history []agent.Message,
) string {
	// 1. Build artifact anchor sections (no LLM needed)
	artifactAnchor := c.buildArtifactSections(ctx)

	// 2. Check for existing anchor from previous compaction
	existingAnchor := extractExistingAnchor(history)

	// 3. Extract conversation text for LLM summarization
	conversationText := agent.ExtractConversationText(history, agent.ProtectLastN)
	if conversationText == "" && artifactAnchor == "" {
		return existingAnchor // nothing to summarize
	}

	// 4. Build prompt and call LLM for conversation summary
	prompt := agent.BuildCompactPrompt(artifactAnchor, existingAnchor, conversationText)
	llmSummary := c.callCompactLLM(ctx, sess, prompt)

	// 5. Combine: artifact anchor + LLM summary
	var b strings.Builder
	if artifactAnchor != "" {
		b.WriteString(artifactAnchor)
	}
	if llmSummary != "" {
		b.WriteString(llmSummary)
	} else if existingAnchor != "" {
		// LLM failed — preserve existing anchor at minimum
		b.WriteString(existingAnchor)
	}

	return b.String()
}

// buildArtifactSections reads .quint/ artifacts and formats them for the anchor.
func (c *Coordinator) buildArtifactSections(ctx context.Context) string {
	if c.ArtifactStore == nil {
		return ""
	}

	navState := artifact.ComputeNavState(ctx, c.ArtifactStore, "")

	var problemSummary, decisionSummary, portfolioSummary string

	if navState.ProblemTitle != "" {
		problemSummary = fmt.Sprintf("[%s] %s", navState.ProblemStatus, navState.ProblemTitle)
	}
	if navState.DecisionInfo != "" {
		decisionSummary = navState.DecisionInfo
	}
	if navState.PortfolioInfo != "" {
		portfolioSummary = navState.PortfolioInfo
	}

	return agent.BuildArtifactAnchor(problemSummary, decisionSummary, portfolioSummary)
}

// callCompactLLM makes a single LLM call for conversation summarization.
// Uses the parent's provider directly — the compact subagent infrastructure
// is available but for inline compaction a direct call is simpler and blocking.
func (c *Coordinator) callCompactLLM(ctx context.Context, sess *agent.Session, prompt string) string {
	messages := []agent.Message{
		{Role: agent.RoleSystem, Parts: []agent.Part{agent.TextPart{Text: agent.CompactSubagent().SystemPrompt}}},
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: prompt}}},
	}

	resp, err := c.Provider.Stream(ctx, messages, nil, func(delta provider.StreamDelta) {
		// No streaming to TUI — compaction is internal
	})
	if err != nil {
		logger.Error().Str("component", "agent").Err(err).Msg("agent.compact_llm_error")
		return ""
	}
	if resp == nil {
		return ""
	}

	return strings.TrimSpace(resp.Text())
}

// extractExistingAnchor finds a previous compaction summary in the history.
func extractExistingAnchor(history []agent.Message) string {
	for _, msg := range history {
		if msg.Role != agent.RoleSystem {
			continue
		}
		text := msg.Text()
		if strings.HasPrefix(text, "[Context compaction summary]") {
			// Strip the prefix to get the raw anchor
			return strings.TrimPrefix(text, "[Context compaction summary]\n\n")
		}
	}
	return ""
}

// emergencyTruncate keeps only system prompt + last N messages.
func emergencyTruncate(history []agent.Message, keepLastN int) []agent.Message {
	var result []agent.Message

	// Keep system messages from the beginning
	for _, msg := range history {
		if msg.Role == agent.RoleSystem {
			result = append(result, msg)
		} else {
			break
		}
	}

	// Keep last N
	start := len(history) - keepLastN
	if start < 0 {
		start = 0
	}
	for _, msg := range history[start:] {
		if msg.Role != agent.RoleSystem { // avoid duplicating system messages
			result = append(result, msg)
		}
	}

	return result
}

// ---------------------------------------------------------------------------
// Force compact (user-triggered via /compact)
// ---------------------------------------------------------------------------

// ForceCompact runs compaction regardless of token usage and persists the result.
// Returns (messagesBefore, messagesAfter, error).
func (c *Coordinator) ForceCompact(ctx context.Context, sess *agent.Session) (int, int, error) {
	history, err := c.Messages.ListBySession(ctx, sess.ID)
	if err != nil {
		return 0, 0, fmt.Errorf("load history: %w", err)
	}

	before := len(history)
	if before <= agent.ProtectLastN+1 {
		return before, before, nil // too few messages to compact
	}

	// Build full history with system prompt (matches reactLoop structure)
	systemMsg := agent.Message{
		Role:  agent.RoleSystem,
		Parts: []agent.Part{agent.TextPart{Text: c.SystemPrompt}},
	}
	fullHistory := append([]agent.Message{systemMsg}, history...)

	// Run stage 2 compaction (LLM summary + artifacts)
	anchor := c.buildFullAnchor(ctx, sess, fullHistory)
	if anchor == "" {
		// No artifacts and LLM failed — do stage 1 (prune tool outputs) at minimum
		pruned := agent.PruneOldToolOutputs(fullHistory, agent.ProtectLastN)
		anchor = agent.ExtractConversationText(pruned, agent.ProtectLastN)
		if anchor == "" {
			return before, before, nil
		}
	}

	// Persist: delete old messages from DB, keep last N
	deleted, err := c.Messages.DeleteOlderThan(ctx, sess.ID, agent.ProtectLastN)
	if err != nil {
		return before, before, fmt.Errorf("delete old messages: %w", err)
	}

	// Insert anchor as a system message (placed before the kept messages by timestamp)
	anchorMsg := &agent.Message{
		ID:        newMsgID(),
		SessionID: sess.ID,
		Role:      agent.RoleSystem,
		Parts:     []agent.Part{agent.TextPart{Text: fmt.Sprintf("[Context compaction summary]\n\n%s", anchor)}},
		CreatedAt: time.Now().UTC().Add(-time.Second), // slightly before kept messages
	}
	if err := c.Messages.Save(ctx, anchorMsg); err != nil {
		return before, before, fmt.Errorf("save anchor: %w", err)
	}

	after := before - deleted + 1 // +1 for the anchor
	logger.Info().Str("component", "agent").
		Int("before", before).
		Int("after", after).
		Int("deleted", deleted).
		Msg("agent.force_compact_done")

	return before, after, nil
}
