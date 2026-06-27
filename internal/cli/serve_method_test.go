package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	methodpkg "github.com/m0n0x41d/haft/internal/method"
)

func TestHandleHaftMethodPullCreatesMethodRun(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := filepath.Join(t.TempDir(), ".haft")

	result, ref, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":             "pull",
		"task":               "Add Slack notification delivery",
		"declared_task_kind": "external_integration",
		"change_intent":      "add_feature",
		"intended_files": []any{
			"internal/slack/adapter.go",
			"internal/domain/notification.go",
		},
		"risk_signals": []any{
			map[string]any{"id": "external_io", "source": "test"},
			map[string]any{"id": "domain_boundary", "source": "test"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ref, "mpull-") {
		t.Fatalf("created ref = %q, want mpull-*", ref)
	}
	if !strings.Contains(result, "domain-port-before-adapter") {
		t.Fatalf("pull response missing domain method:\n%s", result)
	}
	if !strings.Contains(result, "source_kind=methodpack_card") {
		t.Fatalf("pull response missing method source posture:\n%s", result)
	}
	if !strings.Contains(result, "normativity=support_carrier_non_normative_fpf") {
		t.Fatalf("pull response lets MethodPack masquerade as normative FPF:\n%s", result)
	}
	if !strings.Contains(result, "haft_method(action=\"close\"") {
		t.Fatalf("pull response missing close instruction:\n%s", result)
	}
	for _, want := range []string{"Close template", `"gate_id"`, `"status": "satisfied"`, `"evidence_refs"`} {
		if !strings.Contains(result, want) {
			t.Fatalf("pull response missing close template field %q:\n%s", want, result)
		}
	}

	stored, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Meta.Kind != artifact.KindMethodRun {
		t.Fatalf("stored kind = %s, want MethodRun", stored.Meta.Kind)
	}
	run := decodeStoredMethodRun(t, ctx, store, ref)
	for _, card := range run.Methods {
		if card.SourcePosture.Normativity != methodpkg.MethodSourceNormativity {
			t.Fatalf("stored card %s normativity = %q", card.ID, card.SourcePosture.Normativity)
		}
	}

	path := filepath.Join(haftDir, "method-runs", ref+".md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("method run carrier missing at %s: %v", path, err)
	}
}

func TestHandleHaftMethodCatalogReturnsCurrentLifecycleReport(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := filepath.Join(t.TempDir(), ".haft")

	result, _, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":        "catalog",
		"method_status": "current",
	})
	if err != nil {
		t.Fatal(err)
	}

	var report methodpkg.CatalogReport
	if err := json.Unmarshal([]byte(result), &report); err != nil {
		t.Fatalf("decode catalog report: %v\n%s", err, result)
	}
	if report.Kind != methodpkg.CatalogReportKind {
		t.Fatalf("kind = %q", report.Kind)
	}
	if report.FilterStatus != methodpkg.LifecycleCurrent {
		t.Fatalf("filter = %q", report.FilterStatus)
	}
	if report.Summary.Returned == 0 {
		t.Fatalf("catalog returned no methods: %+v", report.Summary)
	}
	if !strings.Contains(report.AuthorityBoundary, "not_processpattern") {
		t.Fatalf("authority boundary = %q, want not_processpattern", report.AuthorityBoundary)
	}
	for _, entry := range report.Methods {
		if entry.Lifecycle.Status != methodpkg.LifecycleCurrent {
			t.Fatalf("current catalog included non-current entry: %+v", entry)
		}
		if entry.SourcePosture.Normativity != methodpkg.MethodSourceNormativity {
			t.Fatalf("entry %s normativity = %q", entry.ID, entry.SourcePosture.Normativity)
		}
	}
}

func TestHandleHaftMethodStatusAndShowRecoverOpenRun(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := filepath.Join(t.TempDir(), ".haft")

	_, ref, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":             "pull",
		"task":               "Fix failing parser test",
		"declared_task_kind": "bugfix",
		"change_intent":      "fix_bug",
		"risk_signals":       []any{map[string]any{"id": "failing_test"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	status, _, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action": "status",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, ref) {
		t.Fatalf("status response = %q, want open run %s", status, ref)
	}

	shown, _, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":  "show",
		"pull_id": ref,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shown, "Status: open") {
		t.Fatalf("show response = %q, want open status", shown)
	}
	for _, want := range []string{"Close template", `"gate_id"`, `"status": "satisfied"`, `"evidence_refs"`} {
		if !strings.Contains(shown, want) {
			t.Fatalf("show response missing close template field %q:\n%s", want, shown)
		}
	}
}

func TestHandleHaftMethodCloseRequiresEvidenceOrExplicitWaiver(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := filepath.Join(t.TempDir(), ".haft")

	_, ref, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":             "pull",
		"task":               "Fix failing parser test",
		"declared_task_kind": "bugfix",
		"change_intent":      "fix_bug",
		"risk_signals":       []any{map[string]any{"id": "failing_test"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	run := decodeStoredMethodRun(t, ctx, store, ref)
	gateResults := methodGateResultsWithoutEvidence(run)

	_, _, err = handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":        "close",
		"pull_id":       ref,
		"changed_files": []any{"internal/parser/parser.go"},
		"gate_results":  gateResults,
	})
	if err == nil {
		t.Fatal("close accepted hard gates without evidence")
	}
	for _, want := range []string{"expected gate_results[] shape", "gate_id", "status", "evidence_refs"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("close error missing %q: %v", want, err)
		}
	}

	waivers := methodWaiversForRun(run)
	result, _, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":        "close",
		"pull_id":       ref,
		"changed_files": []any{"internal/parser/parser.go"},
		"waivers":       waivers,
		"verification": map[string]any{
			"result": "waived in test fixture",
		},
	})
	if err != nil {
		t.Fatalf("close with explicit waivers failed: %v", err)
	}
	if !strings.Contains(result, "Method run closed") {
		t.Fatalf("close response = %q, want closed response", result)
	}

	closed := decodeStoredMethodRun(t, ctx, store, ref)
	if closed.Status != "closed" {
		t.Fatalf("run status = %q, want closed", closed.Status)
	}
}

func TestHandleHaftMethodCloseRequiresCarryThroughDisposition(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := filepath.Join(t.TempDir(), ".haft")

	pullResult, ref, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action": "pull",
		"task":   "Apply accepted external review finding",
		"carry_through": []any{map[string]any{
			"source_ref":      "review:external",
			"source_item_ref": "finding-1",
			"acceptance_ref":  "operator:accepted",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pullResult, `"carry_through"`) {
		t.Fatalf("pull response close template missing carry_through:\n%s", pullResult)
	}

	run := decodeStoredMethodRun(t, ctx, store, ref)
	gateResults := methodGateResultsWithEvidence(run, "test:evidence")
	_, _, err = handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":        "close",
		"pull_id":       ref,
		"gate_results":  gateResults,
		"verification":  map[string]any{"result": "pass"},
		"changed_files": []any{"internal/method/run.go"},
	})
	if err == nil {
		t.Fatal("close accepted missing carry-through disposition")
	}
	if !strings.Contains(err.Error(), "carry_through close disposition") {
		t.Fatalf("error = %v, want carry_through disposition failure", err)
	}

	result, _, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":        "close",
		"pull_id":       ref,
		"gate_results":  gateResults,
		"verification":  map[string]any{"result": "pass"},
		"changed_files": []any{"internal/method/run.go"},
		"carry_through": []any{map[string]any{
			"source_ref":      "review:external",
			"source_item_ref": "finding-1",
			"acceptance_ref":  "operator:accepted",
			"disposition":     "applied",
			"target_refs":     []any{"internal/method/run.go::ValidateClose"},
			"evidence_refs":   []any{"go test ./internal/method"},
		}},
	})
	if err != nil {
		t.Fatalf("close rejected applied carry-through disposition: %v", err)
	}
	if !strings.Contains(result, "Method run closed") {
		t.Fatalf("close result = %q", result)
	}

	closed := decodeStoredMethodRun(t, ctx, store, ref)
	if closed.Closeout == nil || len(closed.Closeout.CarryThrough) != 1 {
		t.Fatalf("closed run missing carry-through closeout: %#v", closed.Closeout)
	}
	if closed.Closeout.CarryThrough[0].Disposition != methodpkg.CarryDispositionApplied {
		t.Fatalf("closeout disposition = %#v", closed.Closeout.CarryThrough[0])
	}
}

func TestHandleHaftMethodCloseRequiresProblemGraphClosure(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := filepath.Join(t.TempDir(), ".haft")

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Completed work can stay backlog",
		Signal:     "Implementation shipped but the ProblemCard stayed unlinked.",
		Acceptance: "Method close detects the missing graph closure.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, ref, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":             "pull",
		"task":               "Implement closure hygiene",
		"declared_task_kind": "feature",
		"change_intent":      "add_feature",
		"artifact_refs": map[string]any{
			"problem_ref": problem.Meta.ID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	run := decodeStoredMethodRun(t, ctx, store, ref)
	gateResults := methodGateResultsWithEvidence(run, "test:evidence")
	if !methodRunHasGate(run, "problem_graph_closure_hygiene_recorded") {
		t.Fatalf("method run missing problem closure hygiene gate: %#v", run.Methods)
	}

	_, _, err = handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":        "close",
		"pull_id":       ref,
		"changed_files": []any{"internal/present/format.go"},
		"gate_results":  gateResults,
		"verification": map[string]any{
			"result": "pass",
		},
	})
	if err == nil {
		t.Fatal("close accepted linked active problem with no graph closure path")
	}
	for _, want := range []string{problem.Meta.ID, "lack graph closure path", "based_on", "supporting evidence"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("close error missing %q: %v", want, err)
		}
	}

	if _, err := artifact.AttachEvidence(ctx, store, artifact.EvidenceInput{
		ArtifactRef:     problem.Meta.ID,
		Type:            "test",
		Content:         "Regression evidence that implementation is linked to this problem.",
		Verdict:         "supports",
		CongruenceLevel: 3,
	}); err != nil {
		t.Fatal(err)
	}

	result, _, err := handleHaftMethod(ctx, store, haftDir, map[string]any{
		"action":        "close",
		"pull_id":       ref,
		"changed_files": []any{"internal/present/format.go"},
		"gate_results":  gateResults,
		"verification": map[string]any{
			"result": "pass",
		},
	})
	if err != nil {
		t.Fatalf("close rejected linked problem after supporting evidence: %v", err)
	}
	if !strings.Contains(result, "Method run closed") {
		t.Fatalf("close response = %q, want closed response", result)
	}
}

func decodeStoredMethodRun(t *testing.T, ctx context.Context, store *artifact.Store, ref string) methodpkg.MethodRun {
	t.Helper()

	stored, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	run, err := methodpkg.DecodeRun(stored)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func methodRunHasGate(run methodpkg.MethodRun, gateID string) bool {
	for _, card := range run.Methods {
		for _, gate := range card.HardGates {
			if gate.ID == gateID {
				return true
			}
		}
	}
	return false
}

func methodGateResultsWithEvidence(run methodpkg.MethodRun, evidenceRef string) []any {
	var results []any
	for _, card := range run.Methods {
		for _, gate := range card.HardGates {
			results = append(results, map[string]any{
				"gate_id":       gate.ID,
				"status":        "satisfied",
				"evidence_refs": []any{evidenceRef},
			})
		}
	}
	return results
}

func methodGateResultsWithoutEvidence(run methodpkg.MethodRun) []any {
	var results []any
	for _, card := range run.Methods {
		for _, gate := range card.HardGates {
			results = append(results, map[string]any{
				"gate_id": gate.ID,
				"status":  "satisfied",
			})
		}
	}
	return results
}

func methodWaiversForRun(run methodpkg.MethodRun) []any {
	var waivers []any
	for _, card := range run.Methods {
		for _, gate := range card.HardGates {
			waivers = append(waivers, map[string]any{
				"gate_id": gate.ID,
				"reason":  "Fixture covers close-by-id behavior; gate evidence is waived in this test.",
			})
		}
	}
	return waivers
}
