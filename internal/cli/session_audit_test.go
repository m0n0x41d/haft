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
		`{"timestamp":"2026-06-25T00:00:035Z","type":"response_item","payload":{"type":"function_call_output","output":"## Code context index — internal/cli/interface.go"}}`,
		`{"timestamp":"2026-06-25T00:00:04Z","type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"impact\",\"symbol\":\"BuiltinCatalog\"}"}}`,
		`{"timestamp":"2026-06-25T00:00:045Z","type":"response_item","payload":{"type":"function_call_output","output":"Index epoch: 3\n\n## Impact of BuiltinCatalog\n\nResolution: 4 resolved • 0 ambiguous • 1 unresolved"}}`,
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
	if !session.GraphResultBeforeFirstEdit || !session.GraphTruthBeforeFirstEdit {
		t.Fatalf("v2 graph result/truth signals missing: %#v", session)
	}
	if session.CodeGraphOrientation != sessionAuditUseUsed ||
		session.TypedMemoryOrientation != sessionAuditUseNotApplicable {
		t.Fatalf("orientation outcomes = %#v", session)
	}
}

func TestSessionAuditSeparatesTypedMemoryHydrationFromGraphPreflight(
	t *testing.T,
) {
	path := filepath.Join(t.TempDir(), "context-heavy.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Review the prior multi-session Haft work and continue the implementation."}]}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"status\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"pull\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"memory\",\"mode\":\"resolve\",\"query\":\"Haft typed memory\",\"max_candidates\":5}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"{\"result_kind\":\"entity_candidates\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"memory\",\"mode\":\"neighborhood\",\"entity_ref\":{\"ref_kind_id\":\"U.EntityRef\",\"reference_id\":\"entity:haft-v9-typed-memory\"}}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"{\"result_kind\":\"exact_neighborhood\",\"result\":{\"interpretation_contract\":{\"hydrate_before_reliance\":true}}}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"impact\",\"symbol\":\"summarizeSessionGraphAudit\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"Index epoch: 9\n\n## Impact of summarizeSessionGraphAudit\n\nResolution: 3 resolved"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Update File: internal/cli/session_audit.go\n@@"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"close\",\"gate_results\":[{\"evidence_refs\":[\"impact\"]}],\"verification\":{\"output_ref\":\"graph result changed plan\"}}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	session := report.Sessions[0]
	if !session.ContextHeavyMemoryUseDetected ||
		!session.TypedMemoryResolveBeforeFirstEdit ||
		!session.TypedMemoryHydrationBeforeFirstEdit {
		t.Fatalf("typed-memory orientation signals missing: %#v", session)
	}
	if session.TypedMemoryBasisUnavailableBeforeFirstEdit {
		t.Fatalf("successful hydration marked unavailable: %#v", session)
	}
	if !session.GraphBeforeFirstEdit || session.Verdict != "pass" {
		t.Fatalf("memory and graph signals collapsed: %#v", session)
	}
	if report.Summary.TypedMemoryHydrationBeforeFirstEdit != 1 {
		t.Fatalf("typed-memory summary = %#v", report.Summary)
	}
	if session.CodeGraphOrientation != sessionAuditUseUsed ||
		session.TypedMemoryOrientation != sessionAuditUseUsed {
		t.Fatalf("orientation outcomes = %#v", session)
	}
}

func TestSessionAuditTreatsUnavailableTypedMemoryBasisAsNonBlocking(
	t *testing.T,
) {
	path := filepath.Join(t.TempDir(), "memory-unavailable.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Continue the implementation using context from the previous session."}]}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"status\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"pull\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"memory\",\"mode\":\"resolve\",\"query\":\"prior context\",\"max_candidates\":5}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"project_basis_unavailable: no current project TypeEnv head; continue unrelated ordinary Work"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"code_context\",\"file\":\"internal/cli/session_audit.go\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"## Code context index — internal/cli/session_audit.go"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Update File: internal/cli/session_audit.go\n@@"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"close\",\"gate_results\":[{\"evidence_refs\":[\"code_context\"]}],\"verification\":{\"output_ref\":\"graph scope changed plan\"}}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	session := report.Sessions[0]
	if !session.TypedMemoryBasisUnavailableBeforeFirstEdit {
		t.Fatalf("unavailable basis was not distinguished: %#v", session)
	}
	if session.TypedMemoryHydrationBeforeFirstEdit {
		t.Fatalf("unavailable basis became fake hydration: %#v", session)
	}
	if session.Verdict != "pass" {
		t.Fatalf("unavailable memory basis blocked graph-backed work: %#v", session)
	}
	if !sessionAuditDiagnosticsContain(
		report.Diagnostics,
		"typed_memory_basis_unavailable_non_blocking",
	) {
		t.Fatalf("missing non-blocking unavailable-basis diagnostic: %#v", report)
	}
	if session.TypedMemoryOrientation != sessionAuditUseUnavailable ||
		session.CodeGraphOrientation != sessionAuditUseUsed {
		t.Fatalf("unavailable memory orientation = %#v", session)
	}
}

func TestSessionAuditFlagsContextHeavyWorkWithoutTypedMemoryOrientation(
	t *testing.T,
) {
	path := filepath.Join(t.TempDir(), "missing-memory.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Audit the previous Codex session and continue its implementation."}]}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"status\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"pull\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"impact\",\"symbol\":\"Serve\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"Index epoch: 2\n\n## Impact of Serve\n\nResolution: 2 resolved"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Update File: internal/cli/serve.go\n@@"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"close\",\"gate_results\":[{\"evidence_refs\":[\"impact\"]}],\"verification\":{\"output_ref\":\"impact changed plan\"}}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.Sessions[0].Verdict != "needs_review" {
		t.Fatalf("missing typed-memory orientation passed: %#v", report)
	}
	if !sessionAuditDiagnosticsContain(
		report.Diagnostics,
		"context_heavy_without_typed_memory_orientation",
	) {
		t.Fatalf("missing typed-memory diagnostic: %#v", report)
	}
	if report.Sessions[0].TypedMemoryOrientation !=
		sessionAuditUseIncorrectlySkipped {
		t.Fatalf("typed memory outcome = %#v", report.Sessions[0])
	}
}

func TestSessionAuditFlagsUnauthorizedTypedMemoryPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unauthorized-admit.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Inspect the prior session and report what happened."}]}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_memory","arguments":"{\"action\":\"admit\",\"contract_version\":\"haft.memory.v2\",\"authority_class\":\"non_binding_semantic_assertion\",\"change_set\":{\"changes\":[]}}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	session := report.Sessions[0]
	if !session.TypedMemoryAdmissionAttempted ||
		!session.UnauthorizedTypedMemoryAdmission {
		t.Fatalf("unauthorized admission was not classified: %#v", session)
	}
	if session.Verdict != "fail" ||
		!sessionAuditDiagnosticsContain(
			report.Diagnostics,
			"unauthorized_typed_memory_persistence",
		) {
		t.Fatalf("unauthorized admission did not fail audit: %#v", report)
	}
}

func TestSessionAuditFlagsUnauthorizedTypedMemoryPersistenceNestedInExec(
	t *testing.T,
) {
	path := filepath.Join(t.TempDir(), "unauthorized-admit-exec.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Inspect the prior session and report what happened."}]}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"exec","arguments":"await tools.mcp__haft__haft_memory({action:\"admit\", request_provenance_ref:\"agent-generated\"})"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	session := report.Sessions[0]
	if !session.TypedMemoryAdmissionAttempted ||
		!session.UnauthorizedTypedMemoryAdmission {
		t.Fatalf("nested unauthorized admission was not classified: %#v", session)
	}
	if session.Verdict != "fail" {
		t.Fatalf("nested unauthorized admission verdict = %q, want fail", session.Verdict)
	}
}

func TestSessionAuditAcceptsExplicitNamedMemoryReceivingUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authorized-admit.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Запиши это в проектную память Haft для следующей сессии."}]}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_memory","arguments":"{\"action\":\"admit\",\"contract_version\":\"haft.memory.v2\",\"authority_class\":\"non_binding_semantic_assertion\",\"request_provenance_ref\":\"operator:current-message\",\"change_set\":{\"changes\":[]}}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	session := report.Sessions[0]
	if !session.TypedMemoryAdmissionAttempted ||
		session.UnauthorizedTypedMemoryAdmission {
		t.Fatalf("explicit receiving use was lost: %#v", session)
	}
	if session.Verdict != "no_edit" {
		t.Fatalf("authorized memory admission changed graph verdict: %#v", session)
	}
}

func TestSessionAuditRecognizesGraphPreflightNestedInExec(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested-exec.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"exec","arguments":"const result = await tools.mcp__haft__haft_query({action:\"impact\", symbol:\"Serve\"}); text(result);"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"Index epoch: 4\n\n## Impact of Serve\n\nResolution: 2 resolved"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Update File: internal/cli/serve.go\n@@"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Sessions[0].GraphBeforeFirstEdit ||
		!slicesContains(
			report.Sessions[0].GraphActionsBeforeFirstEdit,
			"impact",
		) {
		t.Fatalf("nested graph call was invisible: %#v", report.Sessions[0])
	}
}

func TestBuildSessionGraphAuditReportV2TracksTypeScriptBatchAnchorAndTruth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "typescript-v2.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"status\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"pull\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"code_context\",\"files\":[\"src/session.ts\",\"src/View.vue\"],\"lane\":\"decisions\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"# Code context batch 1/2 — src/session.ts\n\n## Code context lane — decisions"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"impact\",\"anchor_id\":\"sym-v2-session\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"Index epoch: 7\n\n## Impact of selectPersona\n\nResolution: 5 resolved • 1 ambiguous • 2 unresolved"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Update File: src/session.ts\n@@\n-export const value = 1\n+export const value = 2"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"close\",\"gate_results\":[{\"gate_id\":\"graph_preflight_recorded\",\"evidence_refs\":[\"code_context batch\",\"impact anchor\"]}],\"verification\":{\"output_ref\":\"graph blast radius changed the plan\"}}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 4 {
		t.Fatalf("schema version = %d", report.SchemaVersion)
	}
	session := report.Sessions[0]
	if session.Verdict != "pass" || !session.TypeScriptEditDetected || !session.TypeScriptGraphBeforeFirstEdit {
		t.Fatalf("TypeScript v2 audit = %#v", session)
	}
	if !session.BatchGraphBeforeFirstEdit || !session.AnchorGraphBeforeFirstEdit || !session.GraphResultBeforeFirstEdit || !session.GraphTruthBeforeFirstEdit {
		t.Fatalf("batch/anchor/truth telemetry missing: %#v", session)
	}
	if !session.IndexEpochObservedBeforeFirstEdit || !session.ResolutionObservedBeforeFirstEdit {
		t.Fatalf("epoch/resolution telemetry missing: %#v", session)
	}
	if report.Summary.TypeScriptSessionsWithEdits != 1 || report.Summary.GraphResultBeforeFirstEdit != 1 || report.Summary.GraphTruthBeforeFirstEdit != 1 {
		t.Fatalf("v2 summary = %#v", report.Summary)
	}
}

func TestBuildSessionGraphAuditReportV2RejectsTypeScriptEditWithoutTargetedGraph(t *testing.T) {
	path := filepath.Join(t.TempDir(), "typescript-missing-graph.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"code_context\",\"file\":\"internal/cli/serve.go\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"## Code context index — internal/cli/serve.go"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Update File: src/session.ts\n@@"}}`,
	})
	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.Sessions[0].Verdict != "fail" || !sessionAuditDiagnosticsContain(report.Diagnostics, "typescript_edit_without_typescript_graph_preflight") {
		t.Fatalf("missing TypeScript graph must fail: %#v", report)
	}
}

func TestBuildSessionGraphAuditReportV2FlagsAttemptWithoutResultAndDegradedResult(t *testing.T) {
	root := t.TempDir()
	attemptPath := filepath.Join(root, "attempt.jsonl")
	writeSessionAuditFixture(t, attemptPath, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"impact\",\"symbol\":\"Serve\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Update File: internal/cli/serve.go\n@@"}}`,
	})
	report, err := buildSessionGraphAuditReport(attemptPath)
	if err != nil {
		t.Fatal(err)
	}
	if report.Sessions[0].Verdict != "needs_review" || !sessionAuditDiagnosticsContain(report.Diagnostics, "graph_preflight_result_not_observed") {
		t.Fatalf("attempt without result = %#v", report)
	}

	degradedPath := filepath.Join(root, "degraded.jsonl")
	writeSessionAuditFixture(t, degradedPath, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"impact\",\"file\":\"src/session.ts\",\"symbol\":\"run\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"Index epoch: 4 • degraded: parse src/session.ts failed\n\n## Impact of run\n\nResolution: 2 resolved • 0 ambiguous • 3 unresolved"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Update File: src/session.ts\n@@"}}`,
	})
	report, err = buildSessionGraphAuditReport(degradedPath)
	if err != nil {
		t.Fatal(err)
	}
	if report.Sessions[0].Verdict != "needs_review" || !sessionAuditDiagnosticsContain(report.Diagnostics, "degraded_graph_used_before_edit") {
		t.Fatalf("degraded graph result = %#v", report)
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

func TestBuildSessionGraphAuditReportClassifiesSubstantiveTextWithoutEditAsNoEdit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "architecture-text.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"## Architecture\n\n- Selected structure: a query core, a typed index, and a read-only transport shell.\n- Boundary: this architecture does not authorize implementation work.\n- Verification plan: exercise lookup and abstention through the public interfaces.\n\nThis is substantive architecture reasoning, not a status update."}]}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Fail != 0 {
		t.Fatalf("fail = %d, want 0; report=%#v", report.Summary.Fail, report)
	}
	if report.Summary.NoEdit != 1 {
		t.Fatalf("no_edit = %d, want 1", report.Summary.NoEdit)
	}
	if report.Sessions[0].FirstSubstantiveOrdinal == 0 {
		t.Fatalf("expected substantive text ordinal: %#v", report.Sessions[0])
	}
	if report.Sessions[0].Verdict != "no_edit" {
		t.Fatalf("verdict = %q, want no_edit", report.Sessions[0].Verdict)
	}
}

func TestBuildSessionGraphAuditReportDoesNotClassifyMechanicalTextAsSubstantive(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mechanical-text.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"README.md\ncmd/haft/main.go\ninternal/cli/session_audit.go"}]}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.NoEdit != 1 {
		t.Fatalf("no_edit = %d, want 1; report=%#v", report.Summary.NoEdit, report)
	}
	if report.Sessions[0].FirstSubstantiveOrdinal != 0 {
		t.Fatalf("first substantive ordinal = %d, want 0", report.Sessions[0].FirstSubstantiveOrdinal)
	}
}

func TestSessionAuditClassifiesMechanicalEditAsGraphNotApplicable(
	t *testing.T,
) {
	path := filepath.Join(t.TempDir(), "mechanical-edit.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"pull\",\"declared_task_kind\":\"mechanical_edit\",\"change_intent\":\"format generated marker\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Update File: README.md\n@@"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"close\",\"verification\":{\"output_ref\":\"format check passed\"}}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	session := report.Sessions[0]
	if session.CodeGraphOrientation != sessionAuditUseNotApplicable ||
		session.TypedMemoryOrientation != sessionAuditUseNotApplicable ||
		session.Verdict != "pass" {
		t.Fatalf("mechanical orientation = %#v", session)
	}
	if sessionAuditDiagnosticsContain(
		report.Diagnostics,
		"edit_before_graph_preflight",
	) {
		t.Fatalf("mechanical edit was treated as missing graph: %#v", report)
	}
}

func TestSessionAuditDoesNotRequireStatusForBoundedGraphBackedEdit(
	t *testing.T,
) {
	path := filepath.Join(t.TempDir(), "bounded-edit.jsonl")
	writeSessionAuditFixture(t, path, []string{
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"pull\",\"declared_task_kind\":\"bugfix\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_query","arguments":"{\"action\":\"code_context\",\"file\":\"internal/cli/session_audit.go\"}"}}`,
		`{"type":"response_item","payload":{"type":"function_call_output","output":"## Code context index — internal/cli/session_audit.go"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"functions","name":"apply_patch","arguments":"*** Update File: internal/cli/session_audit.go\n@@"}}`,
		`{"type":"response_item","payload":{"type":"function_call","namespace":"mcp__haft","name":"haft_method","arguments":"{\"action\":\"close\",\"gate_results\":[{\"evidence_refs\":[\"code_context\"]}],\"verification\":{\"output_ref\":\"graph evidence bounded the plan and blast radius\"}}"}}`,
	})

	report, err := buildSessionGraphAuditReport(path)
	if err != nil {
		t.Fatal(err)
	}
	session := report.Sessions[0]
	if session.StatusBeforeFirstEdit ||
		session.CodeGraphOrientation != sessionAuditUseUsed ||
		session.Verdict != "pass" {
		t.Fatalf("bounded graph-backed edit = %#v", session)
	}
}

func TestSessionAuditHasNoRoutingExpectationFlags(t *testing.T) {
	for _, name := range []string{"expect-pattern", "expect-strategy"} {
		if flag := sessionAuditCmd.Flags().Lookup(name); flag != nil {
			t.Fatalf("legacy routing expectation flag %q is still public", name)
		}
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
