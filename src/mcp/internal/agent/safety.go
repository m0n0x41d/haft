package agent

import "strings"

// ---------------------------------------------------------------------------
// Loop detection — pure functions, no side effects.
// ---------------------------------------------------------------------------

// ToolCallRecord tracks a tool call for loop detection.
type ToolCallRecord struct {
	Name string
	Args string
}

// DetectLoop checks if the last N tool calls contain repeated patterns.
// Returns true if any tool name+args combination appears more than maxRepeats times
// within the last windowSize calls.
// Pure function.
func DetectLoop(history []ToolCallRecord, windowSize, maxRepeats int) bool {
	if len(history) < windowSize {
		return false
	}

	window := history[len(history)-windowSize:]
	counts := make(map[string]int)
	for _, tc := range window {
		key := tc.Name + ":" + tc.Args
		counts[key]++
		if counts[key] >= maxRepeats {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Token tracking — pure functions for context window awareness.
// ---------------------------------------------------------------------------

// TokenBudget tracks cumulative token usage against a model's context limit.
type TokenBudget struct {
	Limit     int // model's context window size
	Used      int // cumulative tokens consumed
	Threshold int // warn/summarize when remaining < threshold
}

// NewTokenBudget creates a budget for a given model.
// Pure function.
func NewTokenBudget(contextWindow int) TokenBudget {
	threshold := contextWindow / 5 // 20% buffer
	if contextWindow > 200_000 {
		threshold = 20_000 // large models: fixed 20k buffer
	}
	return TokenBudget{
		Limit:     contextWindow,
		Threshold: threshold,
	}
}

// Add records token usage from the latest API response.
// tokens is TotalTokens (prompt + completion) — it already represents
// the current context size. We replace Used, not accumulate.
func (b TokenBudget) Add(tokens int) TokenBudget {
	b.Used = tokens
	return b
}

// Remaining returns how many tokens are left.
func (b TokenBudget) Remaining() int {
	r := b.Limit - b.Used
	if r < 0 {
		return 0
	}
	return r
}

// NeedsSummarization returns true if the remaining budget is below threshold.
func (b TokenBudget) NeedsSummarization() bool {
	return b.Remaining() <= b.Threshold
}

// Exhausted returns true if no tokens remain.
func (b TokenBudget) Exhausted() bool {
	return b.Used >= b.Limit
}

// ResetTokenBudget recalculates Used from the actual message history after compaction.
// This prevents stale cumulative counts from persisting after history truncation.
func ResetTokenBudget(old TokenBudget, history []Message) TokenBudget {
	used := 0
	for _, msg := range history {
		used += msg.Tokens
	}
	return TokenBudget{
		Limit:     old.Limit,
		Threshold: old.Threshold,
		Used:      used,
	}
}

// ModelContextWindow returns a reasonable context window estimate for a model.
// Pure function — no API calls.
func ModelContextWindow(model string) int {
	// Known models
	switch {
	case strings.Contains(model, "gpt-5.4"):
		return 1_050_000 // 1.05M context (announced March 2026)
	case strings.Contains(model, "gpt-5.3"):
		return 400_000
	case strings.Contains(model, "gpt-5.2"), strings.Contains(model, "gpt-5.1"):
		return 256_000
	case strings.Contains(model, "gpt-4o"), strings.Contains(model, "gpt-4.1"):
		return 128_000
	case strings.Contains(model, "o4"), strings.Contains(model, "o3"):
		return 200_000
	case strings.Contains(model, "claude"):
		return 200_000
	default:
		return 128_000 // conservative default
	}
}

// strContains removed — using strings.Contains instead
