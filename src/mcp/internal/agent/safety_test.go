package agent

import "testing"

func TestDetectLoop_NoLoop(t *testing.T) {
	history := []ToolCallRecord{
		{Name: "read", Args: "a.go"},
		{Name: "read", Args: "b.go"},
		{Name: "grep", Args: "pattern"},
		{Name: "read", Args: "c.go"},
	}
	if DetectLoop(history, 4, 3) {
		t.Error("should not detect loop with distinct calls")
	}
}

func TestDetectLoop_Detected(t *testing.T) {
	history := []ToolCallRecord{
		{Name: "read", Args: "a.go"},
		{Name: "read", Args: "a.go"},
		{Name: "read", Args: "a.go"},
		{Name: "read", Args: "a.go"},
	}
	if !DetectLoop(history, 4, 3) {
		t.Error("should detect loop — same call 4 times")
	}
}

func TestDetectLoop_ShortHistory(t *testing.T) {
	history := []ToolCallRecord{
		{Name: "read", Args: "a.go"},
	}
	if DetectLoop(history, 4, 3) {
		t.Error("should not detect loop with history shorter than window")
	}
}

func TestTokenBudget(t *testing.T) {
	b := NewTokenBudget(128_000)

	if b.NeedsSummarization() {
		t.Error("fresh budget should not need summarization")
	}

	b = b.Add(105_000)
	if !b.NeedsSummarization() {
		t.Errorf("budget with %d remaining should need summarization (threshold %d)", b.Remaining(), b.Threshold)
	}

	b = b.Add(50_000)
	if !b.Exhausted() {
		t.Error("budget should be exhausted")
	}
}

func TestModelContextWindow(t *testing.T) {
	if w := ModelContextWindow("gpt-5.4"); w != 400_000 {
		t.Errorf("gpt-5.4 should have 400k window, got %d", w)
	}
	if w := ModelContextWindow("gpt-4o"); w != 128_000 {
		t.Errorf("gpt-4o should have 128k window, got %d", w)
	}
	if w := ModelContextWindow("unknown-model"); w != 128_000 {
		t.Errorf("unknown model should default to 128k, got %d", w)
	}
}
