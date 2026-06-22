package cli

import (
	"context"
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
