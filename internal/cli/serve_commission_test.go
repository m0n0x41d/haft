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

func TestHandleHaftCommission_CreateFromDecisionBuildsRunnableCommission(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Harness intake",
		Signal:     "Open-Sleigh needs runnable work without hand-written commission JSON.",
		Acceptance: "A DecisionRecord can become a bounded WorkCommission.",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:          problem.Meta.ID,
		SelectedTitle:       "Create commissions from decisions",
		WhySelected:         "The harness needs Haft-owned authorization objects before it can run.",
		SelectionPolicy:     "Prefer the shortest path that keeps DecisionRecord and WorkCommission distinct.",
		CounterArgument:     "A direct prompt runner would be simpler for a one-off local task.",
		WeakestLink:         "Scope must remain explicit enough that the runner cannot widen authority.",
		WhyNotOthers:        []artifact.RejectionReason{{Variant: "Hand-written JSON", Reason: "Too error-prone for repeated harness runs."}},
		Rollback:            &artifact.RollbackSpec{Triggers: []string{"Commission creation produces invalid scope."}},
		EvidenceReqs:        []string{"go test ./internal/cli"},
		AffectedFiles:       []string{"internal/cli/commission.go", "internal/cli/serve_commission.go"},
		ValidUntil:          "2099-01-01T00:00:00Z",
		FirstModuleCoverage: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":          "create_from_decision",
		"decision_ref":    decision.Meta.ID,
		"repo_ref":        "local:haft",
		"base_sha":        "base-r1",
		"target_branch":   "dev",
		"allowed_actions": []any{"edit_files", "run_tests"},
		"valid_until":     "2099-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	created := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &created); err != nil {
		t.Fatal(err)
	}

	commission := created["commission"]
	if commission["decision_ref"] != decision.Meta.ID {
		t.Fatalf("decision_ref = %#v, want %s", commission["decision_ref"], decision.Meta.ID)
	}
	if commission["problem_card_ref"] != problem.Meta.ID {
		t.Fatalf("problem_card_ref = %#v, want %s", commission["problem_card_ref"], problem.Meta.ID)
	}
	if commission["state"] != "queued" {
		t.Fatalf("state = %#v, want queued", commission["state"])
	}
	if !hexLike(commission["decision_revision_hash"]) {
		t.Fatalf("decision_revision_hash = %#v, want sha256 hex", commission["decision_revision_hash"])
	}
	if !hexLike(commission["scope_hash"]) {
		t.Fatalf("scope_hash = %#v, want sha256 hex", commission["scope_hash"])
	}

	scope, ok := commission["scope"].(map[string]any)
	if !ok {
		t.Fatalf("scope = %#v, want object", commission["scope"])
	}
	if scope["hash"] != commission["scope_hash"] {
		t.Fatalf("scope.hash = %#v, want scope_hash %#v", scope["hash"], commission["scope_hash"])
	}
	if !containsAnyString(scope["allowed_paths"], "internal/cli/commission.go") {
		t.Fatalf("allowed_paths = %#v, want decision affected files", scope["allowed_paths"])
	}
	if !containsAnyString(scope["lockset"], "internal/cli/serve_commission.go") {
		t.Fatalf("lockset = %#v, want decision affected files", scope["lockset"])
	}

	requirements, ok := commission["evidence_requirements"].([]any)
	if !ok || len(requirements) != 1 {
		t.Fatalf("evidence_requirements = %#v, want one requirement", commission["evidence_requirements"])
	}

	requirement, ok := requirements[0].(map[string]any)
	if !ok || requirement["command"] != "go test ./internal/cli" {
		t.Fatalf("evidence requirement = %#v, want command from decision", requirements[0])
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
		t.Fatalf("listed commissions = %#v, want created commission runnable", listed["commissions"])
	}
}

func TestHandleHaftCommission_CreateFromDecisionRequiresScope(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Harness intake",
		Signal:     "A decision has no affected files.",
		Acceptance: "Commission creation refuses fuzzy scope.",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   "Keep scope explicit",
		WhySelected:     "A commission without allowed paths would turn a decision into broad authority.",
		SelectionPolicy: "Prefer hard authority boundaries over convenience defaults.",
		CounterArgument: "The caller could provide a prompt that names the intended files.",
		WeakestLink:     "The fallback would still be untyped prose.",
		WhyNotOthers:    []artifact.RejectionReason{{Variant: "Use the whole repo", Reason: "Too much authority for a single commission."}},
		Rollback:        &artifact.RollbackSpec{Triggers: []string{"Scope is not declared."}},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "create_from_decision",
		"decision_ref":  decision.Meta.ID,
		"repo_ref":      "local:haft",
		"base_sha":      "base-r1",
		"target_branch": "dev",
		"valid_until":   "2099-01-01T00:00:00Z",
	})
	if err == nil || !strings.Contains(err.Error(), "allowed_paths is required") {
		t.Fatalf("error = %v, want allowed_paths requirement", err)
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

func hexLike(value any) bool {
	text, ok := value.(string)
	if !ok || len(text) != 64 {
		return false
	}

	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			continue
		case r >= 'a' && r <= 'f':
			continue
		default:
			return false
		}
	}
	return true
}

func containsAnyString(value any, target string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}

	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
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
