package provider

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/agent"
)

func TestFlattenConversationSplitsSystemAndBody(t *testing.T) {
	messages := []agent.Message{
		{Role: agent.RoleSystem, Parts: []agent.Part{agent.TextPart{Text: "be terse"}}},
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "hello"}}},
		{Role: agent.RoleAssistant, Parts: []agent.Part{agent.TextPart{Text: "hi"}}},
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "more"}}},
	}

	sys, body := flattenConversation(messages)

	if sys != "be terse" {
		t.Fatalf("system prompt = %q, want %q", sys, "be terse")
	}
	wantBody := "User: hello\n\nAssistant: hi\n\nUser: more"
	if body != wantBody {
		t.Fatalf("body mismatch:\n got: %q\nwant: %q", body, wantBody)
	}
}

func TestFlattenConversationMergesMultipleSystemPrompts(t *testing.T) {
	messages := []agent.Message{
		{Role: agent.RoleSystem, Parts: []agent.Part{agent.TextPart{Text: "rule A"}}},
		{Role: agent.RoleSystem, Parts: []agent.Part{agent.TextPart{Text: "rule B"}}},
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "go"}}},
	}
	sys, body := flattenConversation(messages)
	if sys != "rule A\n\nrule B" {
		t.Fatalf("merged system = %q", sys)
	}
	if body != "User: go" {
		t.Fatalf("body = %q", body)
	}
}

func TestFlattenConversationSkipsEmptyTurns(t *testing.T) {
	messages := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "   "}}},
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "hello"}}},
	}
	_, body := flattenConversation(messages)
	if body != "User: hello" {
		t.Fatalf("empty-turn skip failed, body = %q", body)
	}
}

func TestRenderPartsIncludesToolCallsAndResults(t *testing.T) {
	parts := []agent.Part{
		agent.TextPart{Text: "checking"},
		agent.ToolCallPart{ToolCallID: "c1", ToolName: "haft_note", Arguments: `{"x":1}`},
		agent.ToolResultPart{ToolCallID: "c1", Content: "ok"},
		agent.ToolResultPart{ToolCallID: "c2", Content: "boom", IsError: true},
	}
	s := renderParts(parts)
	for _, want := range []string{
		"checking",
		"[tool_call name=haft_note id=c1]",
		`{"x":1}`,
		"[tool_result id=c1]",
		"[tool_result_error id=c2]",
		"boom",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("renderParts output missing %q\nfull:\n%s", want, s)
		}
	}
}

func TestParseClaudeStreamExtractsTextDeltas(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello "}]}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"world"}]}}`,
		`{"type":"result","subtype":"success","is_error":false}`,
	}, "\n")

	var deltas []string
	text, reason, err := parseClaudeStream(strings.NewReader(stream), func(d StreamDelta) {
		if d.Text != "" {
			deltas = append(deltas, d.Text)
		}
	})
	if err != nil {
		t.Fatalf("parseClaudeStream: %v", err)
	}
	if text != "Hello world" {
		t.Fatalf("concatenated text = %q, want %q", text, "Hello world")
	}
	if reason != "stop" {
		t.Fatalf("finish reason = %q, want stop", reason)
	}
	if got := strings.Join(deltas, "|"); got != "Hello |world" {
		t.Fatalf("deltas = %q", got)
	}
}

func TestParseClaudeStreamHandlesErrorResult(t *testing.T) {
	stream := `{"type":"result","subtype":"error_during_execution","is_error":true}`
	_, reason, err := parseClaudeStream(strings.NewReader(stream), func(StreamDelta) {})
	if err != nil {
		t.Fatalf("parseClaudeStream: %v", err)
	}
	if reason != "error" {
		t.Fatalf("finish reason = %q, want error", reason)
	}
}

func TestParseClaudeStreamSkipsMalformedLines(t *testing.T) {
	stream := strings.Join([]string{
		`not json at all`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}`,
		``,
		`{"type":"result","subtype":"success"}`,
	}, "\n")
	text, reason, err := parseClaudeStream(strings.NewReader(stream), func(StreamDelta) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "ok" {
		t.Fatalf("text = %q", text)
	}
	if reason != "stop" {
		t.Fatalf("reason = %q", reason)
	}
}

func TestGuessProviderFromPrefixClaudeCodeBeatsAnthropic(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"claude-code", "claudecode"},
		{"claude-code:sonnet", "claudecode"},
		{"claude-opus-4-20250514", "anthropic"},
		{"claude-sonnet-4-20250514", "anthropic"},
		{"gpt-5.4", "openai"},
		{"gemini-2.5-pro", "google"},
	}
	for _, tc := range tests {
		if got := guessProviderFromPrefix(tc.model); got != tc.want {
			t.Errorf("guessProviderFromPrefix(%q) = %q, want %q", tc.model, got, tc.want)
		}
	}
}
