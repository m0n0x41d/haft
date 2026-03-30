package agent

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

// ContextWindowFunc is a lookup function for model context windows.
// Injected by the coordinator from the registry. Decouples agent from provider.
type ContextWindowFunc func(model string) int

// DefaultContextWindow is a static fallback when no registry is available.
// Used in tests and when registry hasn't been loaded yet.
func DefaultContextWindow(model string) int {
	return 128_000
}
