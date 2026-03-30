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

	// Add replaces Used (TotalTokens from API = current context size, not delta)
	b = b.Add(105_000)
	if !b.NeedsSummarization() {
		t.Errorf("budget with %d remaining should need summarization (threshold %d)", b.Remaining(), b.Threshold)
	}
	if b.Used != 105_000 {
		t.Errorf("Used should be 105000, got %d", b.Used)
	}

	// Simulate next API call with larger context
	b = b.Add(130_000)
	if !b.Exhausted() {
		t.Error("budget should be exhausted at 130k/128k")
	}

	// After compaction, context shrinks — API reports smaller TotalTokens
	b = b.Add(60_000)
	if b.Exhausted() {
		t.Error("budget should not be exhausted after compaction (60k/128k)")
	}
}

func TestRegistryContextWindow(t *testing.T) {
	// Registry-based context window lookup is tested in provider/registry_test.go.
	// Here we just verify DefaultContextWindow returns the fallback.
	if w := DefaultContextWindow("anything"); w != 128_000 {
		t.Errorf("DefaultContextWindow should return 128k, got %d", w)
	}
}
