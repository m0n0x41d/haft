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

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{})
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

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{})
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

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{})
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

func TestBuildSessionGraphAuditReportFlagsPatternUseBypass(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "rename-bypass.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"status\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_problem","arguments":"{\"action\":\"frame\",\"title\":\"Choose whether Haft should be renamed\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_solution","arguments":"{\"action\":\"explore\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_solution","arguments":"{\"action\":\"compare\"}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.PatternUseBypasses != 1 {
		t.Fatalf("pattern use bypasses = %d, want 1; report=%#v", report.Summary.PatternUseBypasses, report)
	}
	if report.Summary.SessionsWithSubstantiveMoves != 1 {
		t.Fatalf("substantive sessions = %d, want 1", report.Summary.SessionsWithSubstantiveMoves)
	}
	session := report.Sessions[0]
	if !session.PatternUseBypass {
		t.Fatalf("session should be pattern_use bypass: %#v", session)
	}
	if len(session.SubstantiveActionsBeforePatternUse) == 0 {
		t.Fatalf("substantive actions missing: %#v", session)
	}
	if !sessionAuditDiagnosticsContain(report.Diagnostics, "pattern_use_bypass") {
		t.Fatalf("diagnostics missing pattern_use_bypass: %#v", report.Diagnostics)
	}
}

func TestBuildSessionGraphAuditReportAcceptsPatternUseBeforeSubstantiveMove(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "rename-gateway.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"status\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"pattern_use\",\"mode\":\"compact\",\"query\":\"Choose a better name for haft\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_problem","arguments":"{\"action\":\"frame\",\"title\":\"Choose whether Haft should be renamed\"}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.PatternUseBypasses != 0 {
		t.Fatalf("pattern use bypasses = %d, want 0; report=%#v", report.Summary.PatternUseBypasses, report)
	}
	if report.Summary.PatternUseBeforeSubstantive != 1 {
		t.Fatalf("pattern use before substantive = %d, want 1", report.Summary.PatternUseBeforeSubstantive)
	}
	if report.Sessions[0].PatternUseBypass {
		t.Fatalf("session should not be bypass: %#v", report.Sessions[0])
	}
}

func TestBuildSessionGraphAuditReportFlagsProgressiveDisclosureBypass(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "retrieval-compact-bypass.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"pattern_use\",\"mode\":\"compact\",\"query\":\"Use boundary norm square\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"{\"recommended_pattern_use\":{\"pattern_ref\":\"A.6.B\"},\"support_level\":\"retrieved_uncompiled\",\"route_match_strategy\":\"retrieved_uncompiled\"}"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"## Boundary Application\n\n- Candidate pattern: A.6.B Boundary Norm Square.\n- Apply it directly to the current source, claim, use, and authority relation.\n- Evidence relation: the retrieved snippet is treated as enough to shape the work.\n- Verification plan: record the allowed use and blocked stronger use after applying the card.\n\nThis is substantive boundary reasoning based on compact retrieval only."}]}}`,
	})

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.ProgressiveDisclosureBypasses != 1 {
		t.Fatalf("progressive disclosure bypasses = %d, want 1; report=%#v", report.Summary.ProgressiveDisclosureBypasses, report)
	}
	if !report.Sessions[0].ProgressiveDisclosureBypass {
		t.Fatalf("session should flag progressive disclosure bypass: %#v", report.Sessions[0])
	}
	if !sessionAuditDiagnosticsContain(report.Diagnostics, "progressive_disclosure_bypass") {
		t.Fatalf("diagnostics missing progressive_disclosure_bypass: %#v", report.Diagnostics)
	}
}

func TestBuildSessionGraphAuditReportAcceptsFullPatternUseBeforeRetrievedApplication(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "retrieval-full-ok.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"pattern_use\",\"mode\":\"compact\",\"query\":\"Use boundary norm square\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"{\"recommended_pattern_use\":{\"pattern_ref\":\"A.6.B\"},\"support_level\":\"retrieved_uncompiled\",\"route_match_strategy\":\"retrieved_uncompiled\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"pattern_use\",\"mode\":\"full\",\"query\":\"Use boundary norm square\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"{\"recommended_pattern_use\":{\"pattern_ref\":\"A.6.B\"},\"support_level\":\"retrieved_uncompiled\",\"route_match_strategy\":\"retrieved_uncompiled\",\"candidate_pattern_use_set\":[{\"pattern_ref\":\"A.6.B\",\"source_card\":{\"source_ref\":\"fixture.md\",\"body\":\"Full card body\"}}]}"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"## Boundary Application\n\n- Candidate pattern: A.6.B Boundary Norm Square.\n- Apply it after reading the full source_card body and checking applicability.\n- Evidence relation: the card is still retrieved_uncompiled and cannot become authority.\n- Verification plan: record allowed use and blocked stronger use separately.\n\nThis is substantive boundary reasoning after full PatternUse disclosure."}]}}`,
	})

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.ProgressiveDisclosureBypasses != 0 {
		t.Fatalf("progressive disclosure bypasses = %d, want 0; report=%#v", report.Summary.ProgressiveDisclosureBypasses, report)
	}
	session := report.Sessions[0]
	if !session.FullBeforePatternApplication {
		t.Fatalf("expected full before application: %#v", session)
	}
	if session.ProgressiveDisclosureBypass {
		t.Fatalf("session should not flag bypass: %#v", session)
	}
}

func TestBuildSessionGraphAuditReportFlagsNonEditTextReasoningBypass(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "architecture-text-bypass.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"## Architecture\n\n- Selected structure: a router core, a route-card index, and a read-only recommendation shell.\n- Boundary: this architecture does not authorize implementation work.\n- Verification plan: run a transcript audit and fixture audit before claiming behavior lift.\n\nThis is substantive architecture reasoning, not a status update."}]}}`,
	})

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Fail != 1 {
		t.Fatalf("fail = %d, want 1; report=%#v", report.Summary.Fail, report)
	}
	if report.Summary.NoEdit != 0 {
		t.Fatalf("no_edit = %d, want 0", report.Summary.NoEdit)
	}
	if report.Summary.PatternUseBypasses != 1 {
		t.Fatalf("pattern use bypasses = %d, want 1", report.Summary.PatternUseBypasses)
	}
	if report.Sessions[0].FirstSubstantiveOrdinal == 0 {
		t.Fatalf("expected substantive text ordinal: %#v", report.Sessions[0])
	}
}

func TestBuildSessionGraphAuditReportDoesNotFlagMechanicalText(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mechanical-text.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"README.md\ncmd/haft/main.go\ninternal/cli/session_audit.go"}]}}`,
	})

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{PatternRef: "none", RouteStrategy: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.NoEdit != 1 {
		t.Fatalf("no_edit = %d, want 1; report=%#v", report.Summary.NoEdit, report)
	}
	if report.Summary.PatternUseBypasses != 0 {
		t.Fatalf("pattern use bypasses = %d, want 0", report.Summary.PatternUseBypasses)
	}
	if report.Summary.ScenarioPass != 1 {
		t.Fatalf("scenario pass = %d, want 1", report.Summary.ScenarioPass)
	}
}

func TestBuildSessionGraphAuditReportDoesNotFlagMechanicalFileListing(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mechanical-file-listing.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Файлы в проекте:\n\n` +
			"```text" +
			`\n.codex/config.toml\n.haft/evidence/.gitkeep\n.haft/methods/swe-core/behavior-first-testing.yaml\n.haft/methods/swe-core/verification-before-completion.yaml\n.haft/project.yaml\n.haft/specs/target-system.md\nREADME.md\n` +
			"```" +
			`"}]}}`,
	})

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{PatternRef: "none", RouteStrategy: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.NoEdit != 1 {
		t.Fatalf("no_edit = %d, want 1; report=%#v", report.Summary.NoEdit, report)
	}
	if report.Summary.PatternUseBypasses != 0 {
		t.Fatalf("pattern use bypasses = %d, want 0", report.Summary.PatternUseBypasses)
	}
	if report.Summary.ScenarioPass != 1 {
		t.Fatalf("scenario pass = %d, want 1", report.Summary.ScenarioPass)
	}
}

func TestBuildSessionGraphAuditReportChecksExpectedPatternUseResult(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pattern-use-expected.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"item.completed","item":{"type":"mcp_tool_call","tool":"haft_query","arguments":{"action":"pattern_use","mode":"compact","query":"именуй нормально"},"result":{"content":[{"type":"text","text":"{\"schema_version\":1,\"record_kind\":\"pattern_use_gateway\",\"recommended_pattern_use\":{\"pattern_ref\":\"F.18\",\"title\":\"nameCard\"},\"support_level\":\"implemented_substrate\",\"route_match_strategy\":\"semantic_compiled_route\"}"}]}}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"## Naming\n\n- EntityOfConcern: chat notes to work-card service.\n- Candidate name: NoteForge.\n- Boundary: this name is not a public commitment until collision checks pass.\n\nThis is substantive naming reasoning after the PatternUse gateway."}}`,
	})

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{
		PatternRef:    "F.18",
		RouteStrategy: "semantic_compiled_route",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.PatternUseBeforeSubstantive != 1 {
		t.Fatalf("pattern use before substantive = %d, want 1", report.Summary.PatternUseBeforeSubstantive)
	}
	if report.Summary.ScenarioPass != 1 {
		t.Fatalf("scenario pass = %d, want 1; report=%#v", report.Summary.ScenarioPass, report)
	}
	session := report.Sessions[0]
	if !slicesContains(session.ObservedPatternRefs, "F.18") {
		t.Fatalf("observed patterns = %#v", session.ObservedPatternRefs)
	}
	if !slicesContains(session.ObservedRouteMatchStrategies, "semantic_compiled_route") {
		t.Fatalf("observed strategies = %#v", session.ObservedRouteMatchStrategies)
	}
}

func TestBuildSessionGraphAuditReportTreatsNameCardLabelsAsSubstantive(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pattern-use-namecard-labels.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"pattern_use\",\"mode\":\"compact\",\"query\":\"Name the notes-to-card service\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"{\"recommended_pattern_use\":{\"pattern_ref\":\"F.18\"},\"support_level\":\"implemented_substrate\",\"route_match_strategy\":\"semantic_compiled_route\"}"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Я бы назвал его **Tracewright**.\n\nEntityOfConcern: не карточка и не заметка, а маленький сервис-переработчик: из сырого чатового материала он делает рабочие карточки, которые можно проверить по источникам.\n\nПочему это имя: Trace держит главный смысл: карточка не просто сформулирована, у нее есть след к исходным цитатам и основание для проверки.\n\nБлижайший отвергнутый вариант: Cardifier. Он слишком механический и называет операцию по выходному формату, а не ценность сервиса.\n\nUsage sentence: Tracewright прогнал чат и выпустил три reviewable work cards с source quotes и confidence notes."}]}}`,
	})

	report, err := buildSessionGraphAuditReport(path, sessionAuditExpectation{
		PatternRef:    "F.18",
		RouteStrategy: "semantic_compiled_route",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.PatternUseBeforeSubstantive != 1 {
		t.Fatalf("pattern use before substantive = %d, want 1; report=%#v", report.Summary.PatternUseBeforeSubstantive, report)
	}
	if report.Summary.ScenarioPass != 1 {
		t.Fatalf("scenario pass = %d, want 1; report=%#v", report.Summary.ScenarioPass, report)
	}
	if report.Sessions[0].Verdict != "pass" {
		t.Fatalf("verdict = %q; session=%#v", report.Sessions[0].Verdict, report.Sessions[0])
	}
}

func TestSessionAuditTextShowsSummary(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "codex-fail.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Begin Patch\n*** End Patch"}}`,
	})

	report, err := buildSessionGraphAuditReport(root, sessionAuditExpectation{})
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

func sessionAuditDiagnosticsContain(
	diagnostics []sessionGraphAuditDiagnostic,
	code string,
) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func slicesContains(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
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
