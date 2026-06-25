package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSessionGraphAuditReportPassesCodexGraphPreflight(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "codex.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"timestamp":"2026-06-25T00:00:00Z","type":"session_meta","payload":{"base_instructions":{"text":"haft_query(action=\"status\") appears here but must not count"}}}`,
		`{"timestamp":"2026-06-25T00:00:01Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"status\"}"}}`,
		`{"timestamp":"2026-06-25T00:00:02Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"pull\"}"}}`,
		`{"timestamp":"2026-06-25T00:00:03Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"code_context\",\"file\":\"internal/cli/interface.go\"}"}}`,
		`{"timestamp":"2026-06-25T00:00:04Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"impact\",\"symbol\":\"BuiltinCatalog\"}"}}`,
		`{"timestamp":"2026-06-25T00:00:05Z","type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Begin Patch\n*** End Patch"}}`,
		`{"timestamp":"2026-06-25T00:00:06Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"close\",\"gate_results\":[{\"gate_id\":\"graph_preflight_recorded_before_governed_edit\",\"evidence_refs\":[\"haft_query(action=\\\"code_context\\\")\",\"haft_query(action=\\\"impact\\\")\"]}],\"verification\":{\"output_ref\":\"impact changed blast risk plan\"}}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Pass != 1 {
		t.Fatalf("pass = %d, want 1; report=%#v", report.Summary.Pass, report)
	}
	session := report.Sessions[0]
	if !session.StatusBeforeFirstEdit ||
		!session.MethodPullBeforeFirstEdit ||
		!session.GraphBeforeFirstEdit ||
		!session.RichGraphBeforeFirstEdit ||
		!session.GraphPlanInfluenceCloseoutHint {
		t.Fatalf("session missing preflight signals: %#v", session)
	}
	if session.Verdict != "pass" {
		t.Fatalf("verdict = %q, want pass", session.Verdict)
	}
}

func TestBuildSessionGraphAuditReportFailsEditBeforeGraph(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "codex-fail.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"status\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Begin Patch\n*** End Patch"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"code_context\",\"file\":\"internal/cli/interface.go\"}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Fail != 1 {
		t.Fatalf("fail = %d, want 1; report=%#v", report.Summary.Fail, report)
	}
	if len(report.Diagnostics) == 0 {
		t.Fatal("expected diagnostic for edit before graph preflight")
	}
	if report.Diagnostics[0].Code != "edit_before_graph_preflight" {
		t.Fatalf("first diagnostic = %#v", report.Diagnostics[0])
	}
}

func TestBuildSessionGraphAuditReportReadsClaudeToolUse(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "claude.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__haft__haft_query","input":{"action":"status"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__haft__haft_method","input":{"action":"pull"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__haft__haft_query","input":{"action":"node","symbol":"Pull"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"/tmp/out.txt","content":"x"}}]}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	session := report.Sessions[0]
	if session.Verdict != "needs_review" {
		t.Fatalf("verdict = %q, want needs_review", session.Verdict)
	}
	if !session.RichGraphBeforeFirstEdit {
		t.Fatalf("Claude rich graph action not detected: %#v", session)
	}
}

func TestSessionAuditTextShowsSummary(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "codex-fail.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Begin Patch\n*** End Patch"}}`,
	})

	report, err := buildSessionGraphAuditReport(root)
	if err != nil {
		t.Fatal(err)
	}
	report = limitSessionGraphAuditReport(report, 20)

	var output bytes.Buffer
	if err := writeSessionGraphAuditText(&output, report); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"Haft session graph-use audit", "edit_before_graph_preflight", "verdict=fail"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text missing %q:\n%s", want, text)
		}
	}
}

func writeSessionAuditFixture(
	t *testing.T,
	path string,
	lines []string,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
