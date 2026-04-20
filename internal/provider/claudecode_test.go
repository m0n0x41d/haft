package provider

import (
	"encoding/json"
	"os"
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
		`{"type":"system","subtype":"init","session_id":"sess-123"}`,
		`{"type":"assistant","session_id":"sess-123","message":{"role":"assistant","content":[{"type":"text","text":"Hello "}]}}`,
		`{"type":"assistant","session_id":"sess-123","message":{"role":"assistant","content":[{"type":"text","text":"world"}]}}`,
		`{"type":"result","subtype":"success","is_error":false,"session_id":"sess-123"}`,
	}, "\n")

	var deltas []string
	res, err := parseClaudeStream(strings.NewReader(stream), func(d StreamDelta) {
		if d.Text != "" {
			deltas = append(deltas, d.Text)
		}
	})
	if err != nil {
		t.Fatalf("parseClaudeStream: %v", err)
	}
	if res.text != "Hello world" {
		t.Fatalf("concatenated text = %q, want %q", res.text, "Hello world")
	}
	if res.finishReason != "stop" {
		t.Fatalf("finish reason = %q, want stop", res.finishReason)
	}
	if res.sessionID != "sess-123" {
		t.Fatalf("session id = %q, want sess-123", res.sessionID)
	}
	if got := strings.Join(deltas, "|"); got != "Hello |world" {
		t.Fatalf("deltas = %q", got)
	}
}

func TestParseClaudeStreamHandlesErrorResult(t *testing.T) {
	stream := `{"type":"result","subtype":"error_during_execution","is_error":true}`
	res, err := parseClaudeStream(strings.NewReader(stream), func(StreamDelta) {})
	if err != nil {
		t.Fatalf("parseClaudeStream: %v", err)
	}
	if res.finishReason != "error" {
		t.Fatalf("finish reason = %q, want error", res.finishReason)
	}
}

func TestParseClaudeStreamSkipsMalformedLines(t *testing.T) {
	stream := strings.Join([]string{
		`not json at all`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}`,
		``,
		`{"type":"result","subtype":"success"}`,
	}, "\n")
	res, err := parseClaudeStream(strings.NewReader(stream), func(StreamDelta) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.text != "ok" {
		t.Fatalf("text = %q", res.text)
	}
	if res.finishReason != "stop" {
		t.Fatalf("reason = %q", res.finishReason)
	}
}

func TestTakeResumeDecisionFreshOnFirstTurn(t *testing.T) {
	p := &ClaudeCodeProvider{}
	messages := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "hello"}}},
	}
	plan := p.takeResumeDecision(messages)
	if plan.resume {
		t.Fatalf("expected fresh turn on first call")
	}
	if plan.prompt == "" {
		t.Fatalf("expected flattened prompt, got empty")
	}
}

func TestTakeResumeDecisionContinues(t *testing.T) {
	p := &ClaudeCodeProvider{sessionID: "sess-abc", msgsSent: 2}
	messages := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "hi"}}},
		{Role: agent.RoleAssistant, Parts: []agent.Part{agent.TextPart{Text: "yo"}}},
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "follow-up"}}},
	}
	plan := p.takeResumeDecision(messages)
	if !plan.resume {
		t.Fatalf("expected resume plan")
	}
	if plan.sessionID != "sess-abc" {
		t.Fatalf("sessionID = %q", plan.sessionID)
	}
	if plan.prompt != "follow-up" {
		t.Fatalf("prompt = %q, want %q", plan.prompt, "follow-up")
	}
}

func TestTakeResumeDecisionResetsOnGap(t *testing.T) {
	p := &ClaudeCodeProvider{sessionID: "sess-abc", msgsSent: 2}
	// Conversation jumped by 2 (not 1) since last turn — history was edited
	// or branched. Fall back to fresh, clearing state.
	messages := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "a"}}},
		{Role: agent.RoleAssistant, Parts: []agent.Part{agent.TextPart{Text: "b"}}},
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "c"}}},
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "d"}}},
	}
	plan := p.takeResumeDecision(messages)
	if plan.resume {
		t.Fatalf("expected reset when conversation grew by >1")
	}
	if p.sessionID != "" || p.msgsSent != 0 {
		t.Fatalf("state not reset: sessionID=%q msgsSent=%d", p.sessionID, p.msgsSent)
	}
}

func TestTakeResumeDecisionResetsOnNonUserTail(t *testing.T) {
	p := &ClaudeCodeProvider{sessionID: "sess-abc", msgsSent: 1}
	messages := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "hi"}}},
		{Role: agent.RoleTool, Parts: []agent.Part{agent.ToolResultPart{Content: "x"}}},
	}
	plan := p.takeResumeDecision(messages)
	if plan.resume {
		t.Fatalf("should not resume when tail is a tool-result message")
	}
}

func TestTakeResumeDecisionHonorsEnvOptOut(t *testing.T) {
	t.Setenv("HAFT_CLAUDECODE_NO_RESUME", "1")
	p := &ClaudeCodeProvider{sessionID: "sess-abc", msgsSent: 2}
	messages := []agent.Message{
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "a"}}},
		{Role: agent.RoleAssistant, Parts: []agent.Part{agent.TextPart{Text: "b"}}},
		{Role: agent.RoleUser, Parts: []agent.Part{agent.TextPart{Text: "c"}}},
	}
	plan := p.takeResumeDecision(messages)
	if plan.resume {
		t.Fatalf("HAFT_CLAUDECODE_NO_RESUME should force fresh turns")
	}
}

func TestInvalidateAndRecordSession(t *testing.T) {
	p := &ClaudeCodeProvider{}
	p.recordTurnResult("sess-1", 3)
	if p.sessionID != "sess-1" || p.msgsSent != 4 {
		t.Fatalf("record: got (%q, %d)", p.sessionID, p.msgsSent)
	}
	// Empty new id preserves existing (CLI sometimes omits on retries).
	p.recordTurnResult("", 5)
	if p.sessionID != "sess-1" || p.msgsSent != 6 {
		t.Fatalf("preserve: got (%q, %d)", p.sessionID, p.msgsSent)
	}
	p.invalidateSession()
	if p.sessionID != "" || p.msgsSent != 0 {
		t.Fatalf("invalidate: got (%q, %d)", p.sessionID, p.msgsSent)
	}
}

func TestWriteHaftMCPConfigShape(t *testing.T) {
	const haftExe = "/fake/haft"
	const projectRoot = "/fake/project"

	path, err := writeHaftMCPConfig(haftExe, projectRoot)
	if err != nil {
		t.Fatalf("writeHaftMCPConfig: %v", err)
	}
	if path == "" {
		t.Fatalf("expected a tmpfile path")
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("tmpfile perm = %o, want 0600", mode)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var parsed struct {
		McpServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v: %s", err, data)
	}
	entry, ok := parsed.McpServers["haft"]
	if !ok {
		t.Fatalf("missing haft entry: %s", data)
	}
	if entry.Command != haftExe {
		t.Errorf("command = %q, want %q", entry.Command, haftExe)
	}
	if len(entry.Args) != 1 || entry.Args[0] != "serve" {
		t.Errorf("unexpected args: %v", entry.Args)
	}
	if entry.Env["QUINT_PROJECT_ROOT"] != projectRoot {
		t.Errorf("QUINT_PROJECT_ROOT = %q, want %q", entry.Env["QUINT_PROJECT_ROOT"], projectRoot)
	}
}

func TestCappedBufferKeepsTail(t *testing.T) {
	c := &cappedBuffer{limit: 10}
	c.Write([]byte("hello "))
	c.Write([]byte("world — how are you today?"))
	got := c.String()
	// Last 10 bytes of "hello world — how are you today?" is "you today?"
	// Note: "—" is a 3-byte UTF-8 rune; we slice bytes, not runes. Any
	// tail that ends with " today?" is sufficient to prove the ring works.
	if !strings.HasSuffix(got, " today?") {
		t.Fatalf("want suffix ' today?'; got %q", got)
	}
	if !strings.HasPrefix(got, "…(truncated)") {
		t.Fatalf("want truncated prefix; got %q", got)
	}
}

func TestCappedBufferNoTruncationUnderLimit(t *testing.T) {
	c := &cappedBuffer{limit: 64}
	c.Write([]byte("short"))
	if got := c.String(); got != "short" {
		t.Fatalf("got %q, want %q", got, "short")
	}
}

func TestCliSubModel(t *testing.T) {
	for _, tc := range []struct {
		modelID string
		want    string
	}{
		{"claude-code", ""},
		{"claude-code:sonnet", "sonnet"},
		{"claude-code:opus", "opus"},
		{"claude-code:claude-opus-4-5", "claude-opus-4-5"},
	} {
		p := &ClaudeCodeProvider{modelID: tc.modelID}
		if got := p.cliSubModel(); got != tc.want {
			t.Errorf("cliSubModel(%q) = %q, want %q", tc.modelID, got, tc.want)
		}
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
