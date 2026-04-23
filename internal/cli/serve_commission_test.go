package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleHaftCommission_CreateListAndClaim(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": workCommissionFixture("wc-cli-001", "queued", "2099-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatal(err)
	}

	listed := map[string][]map[string]any{}
	listResult, err := handleHaftCommission(ctx, store, map[string]any{
		"action": "list_runnable",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(listResult), &listed); err != nil {
		t.Fatal(err)
	}

	if len(listed["commissions"]) != 1 {
		t.Fatalf("listed commissions = %#v, want one runnable commission", listed["commissions"])
	}
	if listed["commissions"][0]["id"] != "wc-cli-001" {
		t.Fatalf("listed commission id = %#v", listed["commissions"][0]["id"])
	}

	claimed := map[string]map[string]any{}
	claimResult, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": "wc-cli-001",
		"runner_id":     "open-sleigh:test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(claimResult), &claimed); err != nil {
		t.Fatal(err)
	}

	if claimed["commission"]["state"] != "preflighting" {
		t.Fatalf("claimed state = %#v", claimed["commission"]["state"])
	}

	stored, err := store.Get(ctx, "wc-cli-001")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Meta.Kind != artifact.KindWorkCommission {
		t.Fatalf("stored kind = %s, want WorkCommission", stored.Meta.Kind)
	}

	payload := map[string]any{}
	if err := json.Unmarshal([]byte(stored.StructuredData), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["state"] != "preflighting" {
		t.Fatalf("stored state = %#v", payload["state"])
	}
}

func TestHandleHaftCommission_ClaimRejectsActiveLocksetConflict(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	for _, commission := range []map[string]any{
		workCommissionFixtureWithLockset("wc-lock-a", "queued", "2099-01-01T00:00:00Z", []any{"internal/cli/**"}),
		workCommissionFixtureWithLockset("wc-lock-b", "queued", "2099-01-01T00:00:00Z", []any{"internal/cli/serve.go"}),
		workCommissionFixtureWithLockset("wc-lock-c", "queued", "2099-01-01T00:00:00Z", []any{"open-sleigh/**"}),
	} {
		_, err := handleHaftCommission(ctx, store, map[string]any{
			"action":     "create",
			"commission": commission,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": "wc-lock-a",
		"runner_id":     "open-sleigh:test",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": "wc-lock-b",
		"runner_id":     "open-sleigh:test",
	})
	if err == nil || !strings.Contains(err.Error(), "commission_lock_conflict") {
		t.Fatalf("claim error = %v, want commission_lock_conflict", err)
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": "wc-lock-c",
		"runner_id":     "open-sleigh:test",
	})
	if err != nil {
		t.Fatalf("non-overlapping claim error = %v", err)
	}
}

func TestHandleHaftCommission_ListRunnableFiltersExpiredAndTerminal(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	for _, commission := range []map[string]any{
		workCommissionFixture("wc-ready", "ready", "2099-01-01T00:00:00Z"),
		workCommissionFixture("wc-expired", "queued", "2000-01-01T00:00:00Z"),
		workCommissionFixture("wc-completed", "completed", "2099-01-01T00:00:00Z"),
	} {
		_, err := handleHaftCommission(ctx, store, map[string]any{
			"action":     "create",
			"commission": commission,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action": "list_runnable",
	})
	if err != nil {
		t.Fatal(err)
	}

	listed := map[string][]map[string]any{}
	if err := json.Unmarshal([]byte(result), &listed); err != nil {
		t.Fatal(err)
	}

	if len(listed["commissions"]) != 1 {
		t.Fatalf("listed commissions = %#v, want only ready commission", listed["commissions"])
	}
	if listed["commissions"][0]["id"] != "wc-ready" {
		t.Fatalf("listed commission id = %#v", listed["commissions"][0]["id"])
	}
}

func workCommissionFixtureWithLockset(
	id string,
	state string,
	validUntil string,
	lockset []any,
) map[string]any {
	commission := workCommissionFixture(id, state, validUntil)
	scope, _ := mapArg(commission, "scope")

	scope["allowed_paths"] = lockset
	scope["affected_files"] = lockset
	scope["lockset"] = lockset

	return commission
}

func workCommissionFixture(id, state, validUntil string) map[string]any {
	return map[string]any{
		"id":                           id,
		"decision_ref":                 "dec-20260422-001",
		"decision_revision_hash":       "decision-r1",
		"problem_card_ref":             "pc-20260422-001",
		"implementation_plan_ref":      "plan-20260422-001",
		"implementation_plan_revision": "plan-r1",
		"evidence_requirements": []any{
			map[string]any{
				"kind":    "go_test",
				"command": "go test ./...",
			},
		},
		"projection_policy": "local_only",
		"state":             state,
		"valid_until":       validUntil,
		"fetched_at":        "2026-04-22T10:00:00Z",
		"scope": map[string]any{
			"repo_ref":      "github:m0n0x41d/haft",
			"base_sha":      "base-r1",
			"target_branch": "feature/commission-source",
			"allowed_paths": []any{
				"internal/cli/serve_commission.go",
			},
			"forbidden_paths": []any{},
			"allowed_actions": []any{
				"edit_files",
				"run_tests",
			},
			"affected_files": []any{
				"internal/cli/serve_commission.go",
			},
			"allowed_modules": []any{
				"internal/cli",
			},
			"lockset": []any{
				"internal/cli/serve_commission.go",
			},
		},
	}
}
