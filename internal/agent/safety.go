package agent

// ---------------------------------------------------------------------------
// Loop detection — pure functions, no side effects.
//
// Three-level escalation:
//   Green  (< warnThreshold repeats): normal operation
//   Yellow (≥ warnThreshold repeats): inject warning, don't stop
//   Red    (≥ hardThreshold repeats): hard stop
//
// Guardrail errors (tool rejected, not executed) are excluded from counting.
// Only successful same-tool same-args calls trigger detection.
// ---------------------------------------------------------------------------

// ToolCallRecord tracks a tool call for loop detection.
type ToolCallRecord struct {
	Name    string
	Args    string
	IsError bool // true = tool returned error (guardrail, not found, etc.)
}

// LoopLevel indicates the severity of detected repetition.
type LoopLevel int

const (
	LoopNone    LoopLevel = iota // no repetition
	LoopWarning                  // yellow: agent should summarize and ask user
	LoopHard                     // red: hard stop
)

// DetectLoopLevel checks repetition in tool call history.
// Only counts SUCCESSFUL calls (IsError=false) — guardrail errors are learning, not loops.
// Returns LoopWarning at warnThreshold, LoopHard at hardThreshold.
// Pure function.
func DetectLoopLevel(history []ToolCallRecord, windowSize, warnThreshold, hardThreshold int) LoopLevel {
	if len(history) < windowSize {
		return LoopNone
	}

	window := history[len(history)-windowSize:]
	counts := make(map[string]int)
	for _, tc := range window {
		if tc.IsError {
			continue // guardrail rejections don't count
		}
		key := tc.Name + ":" + tc.Args
		counts[key]++
	}

	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}

	switch {
	case maxCount >= hardThreshold:
		return LoopHard
	case maxCount >= warnThreshold:
		return LoopWarning
	default:
		return LoopNone
	}
}

// DetectLoop is the legacy API — returns true only on hard stop.
// Kept for backward compatibility with tests.
func DetectLoop(history []ToolCallRecord, windowSize, maxRepeats int) bool {
	return DetectLoopLevel(history, windowSize, maxRepeats, maxRepeats) == LoopHard
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
func NewTokenBudget(contextWindow int) TokenBudget {
	threshold := contextWindow / 5
	if contextWindow > 200_000 {
		threshold = 20_000
	}
	return TokenBudget{Limit: contextWindow, Threshold: threshold}
}

func (b TokenBudget) Add(tokens int) TokenBudget {
	b.Used = tokens
	return b
}

func (b TokenBudget) Remaining() int {
	r := b.Limit - b.Used
	if r < 0 {
		return 0
	}
	return r
}

func (b TokenBudget) NeedsSummarization() bool {
	return b.Remaining() <= b.Threshold
}

func (b TokenBudget) Exhausted() bool {
	return b.Used >= b.Limit
}

func ResetTokenBudget(old TokenBudget, history []Message) TokenBudget {
	used := 0
	for _, msg := range history {
		used += msg.Tokens
	}
	return TokenBudget{Limit: old.Limit, Threshold: old.Threshold, Used: used}
}

type ContextWindowFunc func(model string) int

func DefaultContextWindow(model string) int {
	return 128_000
}
