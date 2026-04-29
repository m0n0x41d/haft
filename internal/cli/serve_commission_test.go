package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestHandleHaftCommission_ShowReturnsOneCommission(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": workCommissionFixture("wc-show-001", "queued", "2099-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "show",
		"commission_id": "wc-show-001",
		"older_than":    "1h",
	})
	if err != nil {
		t.Fatal(err)
	}

	shown := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &shown); err != nil {
		t.Fatal(err)
	}

	if shown["commission"]["id"] != "wc-show-001" {
		t.Fatalf("shown commission id = %#v", shown["commission"]["id"])
	}

	operator, ok := shown["commission"]["operator"].(map[string]any)
	if !ok {
		t.Fatalf("operator missing in %#v", shown["commission"])
	}
	if operator["attention"] != true {
		t.Fatalf("operator attention = %#v, want true", operator["attention"])
	}
	if !containsAnyString(operator["suggested_actions"], "requeue") {
		t.Fatalf("suggested_actions = %#v, want requeue", operator["suggested_actions"])
	}
}

func TestHandleHaftCommission_RequeueClearsLeaseAndRecordsEvent(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": workCommissionFixture("wc-requeue-001", "queued", "2099-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": "wc-requeue-001",
		"runner_id":     "open-sleigh:test",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "requeue",
		"commission_id": "wc-requeue-001",
		"runner_id":     "haft-cli:test",
		"reason":        "stale_operator_recovery",
	})
	if err != nil {
		t.Fatal(err)
	}

	requeued := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &requeued); err != nil {
		t.Fatal(err)
	}

	commission := requeued["commission"]
	if commission["state"] != "queued" {
		t.Fatalf("state = %#v, want queued", commission["state"])
	}
	if _, ok := commission["lease"]; ok {
		t.Fatalf("lease = %#v, want removed", commission["lease"])
	}
	if commission["fetched_at"] == "2026-04-22T10:00:00Z" {
		t.Fatalf("fetched_at = %#v, want refreshed queue timestamp", commission["fetched_at"])
	}

	events, ok := commission["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("events = %#v, want one recovery event", commission["events"])
	}

	event, ok := events[0].(map[string]any)
	if !ok {
		t.Fatalf("event = %#v, want object", events[0])
	}
	if event["event"] != "commission_requeued" {
		t.Fatalf("event = %#v, want commission_requeued", event["event"])
	}
	if event["reason"] != "stale_operator_recovery" {
		t.Fatalf("reason = %#v", event["reason"])
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
		t.Fatalf("listed commissions = %#v, want requeued commission runnable", listed["commissions"])
	}
}

func TestHandleHaftCommission_RequeueRejectsTerminalCommissions(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	for _, state := range []string{"completed", "completed_with_projection_debt", "cancelled", "expired"} {
		commissionID := "wc-requeue-" + state
		_, err := handleHaftCommission(ctx, store, map[string]any{
			"action":     "create",
			"commission": workCommissionFixture(commissionID, state, "2099-01-01T00:00:00Z"),
		})
		if err != nil {
			t.Fatal(err)
		}

		_, err = handleHaftCommission(ctx, store, map[string]any{
			"action":        "requeue",
			"commission_id": commissionID,
			"reason":        "operator_recovered",
		})
		if err == nil {
			t.Fatalf("expected %s requeue to fail", state)
		}
		if !strings.Contains(err.Error(), "commission_not_requeueable") {
			t.Fatalf("err = %v, want commission_not_requeueable", err)
		}
	}
}

func TestHandleHaftCommission_RequeueRequiresReasonForRecoverableState(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": workCommissionFixture("wc-requeue-no-reason", "blocked_policy", "2099-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "requeue",
		"commission_id": "wc-requeue-no-reason",
	})
	if err == nil {
		t.Fatal("expected requeue without reason to fail")
	}
	if !strings.Contains(err.Error(), "reason is required") {
		t.Fatalf("err = %v, want reason is required", err)
	}
}

func TestHandleHaftCommission_RequeueRejectsExpiredOpenCommission(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": workCommissionFixture("wc-requeue-expired", "queued", "2000-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "requeue",
		"commission_id": "wc-requeue-expired",
		"reason":        "operator_recovered",
	})
	if err == nil {
		t.Fatal("expected expired requeue to fail")
	}
	if !strings.Contains(err.Error(), "valid_until expired") {
		t.Fatalf("err = %v, want valid_until expired", err)
	}
}

func TestHandleHaftCommission_CancelRejectsTerminalCommissions(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	for _, state := range []string{"completed", "completed_with_projection_debt", "cancelled", "expired"} {
		commissionID := "wc-cancel-" + state
		_, err := handleHaftCommission(ctx, store, map[string]any{
			"action":     "create",
			"commission": workCommissionFixture(commissionID, state, "2099-01-01T00:00:00Z"),
		})
		if err != nil {
			t.Fatal(err)
		}

		_, err = handleHaftCommission(ctx, store, map[string]any{
			"action":        "cancel",
			"commission_id": commissionID,
			"reason":        "operator_cancelled",
		})
		if err == nil {
			t.Fatalf("expected %s cancel to fail", state)
		}
		if !strings.Contains(err.Error(), "commission_not_cancellable") {
			t.Fatalf("err = %v, want commission_not_cancellable", err)
		}
	}
}

func TestHandleHaftCommission_ListStaleAndCancel(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	for _, commission := range []map[string]any{
		workCommissionFixture("wc-stale-open", "queued", "2099-01-01T00:00:00Z"),
		workCommissionFixture("wc-terminal", "completed", "2099-01-01T00:00:00Z"),
	} {
		_, err := handleHaftCommission(ctx, store, map[string]any{
			"action":     "create",
			"commission": commission,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	staleResult, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "list",
		"selector":   "stale",
		"older_than": "1h",
	})
	if err != nil {
		t.Fatal(err)
	}

	stale := map[string][]map[string]any{}
	if err := json.Unmarshal([]byte(staleResult), &stale); err != nil {
		t.Fatal(err)
	}
	if len(stale["commissions"]) != 1 {
		t.Fatalf("stale commissions = %#v, want one open stale commission", stale["commissions"])
	}
	if stale["commissions"][0]["id"] != "wc-stale-open" {
		t.Fatalf("stale commission id = %#v", stale["commissions"][0]["id"])
	}

	operator, ok := stale["commissions"][0]["operator"].(map[string]any)
	if !ok || operator["attention"] != true {
		t.Fatalf("operator = %#v, want attention", stale["commissions"][0]["operator"])
	}
	if !containsAnyString(operator["suggested_actions"], "requeue") {
		t.Fatalf("suggested_actions = %#v, want requeue", operator["suggested_actions"])
	}

	cancelResult, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "cancel",
		"commission_id": "wc-stale-open",
		"runner_id":     "haft-cli:test",
		"reason":        "dogfood cleanup",
	})
	if err != nil {
		t.Fatal(err)
	}

	cancelled := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(cancelResult), &cancelled); err != nil {
		t.Fatal(err)
	}

	commission := cancelled["commission"]
	if commission["state"] != "cancelled" {
		t.Fatalf("state = %#v, want cancelled", commission["state"])
	}
	events, ok := commission["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("events = %#v, want one cancel event", commission["events"])
	}
	event, ok := events[0].(map[string]any)
	if !ok || event["event"] != "commission_cancelled" {
		t.Fatalf("event = %#v, want commission_cancelled", events[0])
	}
	if event["reason"] != "dogfood cleanup" {
		t.Fatalf("reason = %#v, want dogfood cleanup", event["reason"])
	}

	openResult, err := handleHaftCommission(ctx, store, map[string]any{
		"action":   "list",
		"selector": "open",
	})
	if err != nil {
		t.Fatal(err)
	}

	open := map[string][]map[string]any{}
	if err := json.Unmarshal([]byte(openResult), &open); err != nil {
		t.Fatal(err)
	}
	if len(open["commissions"]) != 0 {
		t.Fatalf("open commissions = %#v, want none after cancellation", open["commissions"])
	}
}

func TestHandleHaftCommission_ListStaleIncludesBlockedAndRunningTooLong(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	fresh := workCommissionFixture("wc-fresh-open", "queued", "2099-01-01T00:00:00Z")
	fresh["fetched_at"] = now.Format(time.RFC3339)

	blocked := workCommissionFixture("wc-blocked-attention", "blocked_conflict", "2099-01-01T00:00:00Z")
	blocked["fetched_at"] = now.Format(time.RFC3339)

	running := workCommissionFixture("wc-running-attention", "running", "2099-01-01T00:00:00Z")
	running["fetched_at"] = now.Format(time.RFC3339)
	running["lease"] = map[string]any{
		"claimed_at": now.Add(-3 * time.Hour).Format(time.RFC3339),
	}

	completed := workCommissionFixture("wc-completed-attention", "completed", "2099-01-01T00:00:00Z")

	for _, commission := range []map[string]any{fresh, blocked, running, completed} {
		_, err := handleHaftCommission(ctx, store, map[string]any{
			"action":     "create",
			"commission": commission,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "list",
		"selector":   "stale",
		"older_than": "24h",
	})
	if err != nil {
		t.Fatal(err)
	}

	stale := map[string][]map[string]any{}
	if err := json.Unmarshal([]byte(result), &stale); err != nil {
		t.Fatal(err)
	}

	commissions := commissionsByID(stale["commissions"])
	if len(commissions) != 2 {
		t.Fatalf("stale commissions = %#v, want blocked and running only", stale["commissions"])
	}

	blockedOperator, ok := commissions["wc-blocked-attention"]["operator"].(map[string]any)
	if !ok {
		t.Fatalf("blocked operator = %#v, want object", commissions["wc-blocked-attention"]["operator"])
	}
	if blockedOperator["attention_reason"] != "requires operator decision: blocked_conflict" {
		t.Fatalf("blocked attention_reason = %#v", blockedOperator["attention_reason"])
	}
	if !containsAnyString(blockedOperator["suggested_actions"], "requeue") {
		t.Fatalf("blocked suggested_actions = %#v, want requeue", blockedOperator["suggested_actions"])
	}

	runningOperator, ok := commissions["wc-running-attention"]["operator"].(map[string]any)
	if !ok {
		t.Fatalf("running operator = %#v, want object", commissions["wc-running-attention"]["operator"])
	}
	if runningOperator["attention_reason"] != "active lease older than 2h0m0s" {
		t.Fatalf("running attention_reason = %#v", runningOperator["attention_reason"])
	}
	if !containsAnyString(runningOperator["suggested_actions"], "requeue") {
		t.Fatalf("running suggested_actions = %#v, want requeue", runningOperator["suggested_actions"])
	}
}

func TestHandleHaftCommission_CompleteOrBlockMarksCommissionBlocked(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision := createCommissionDecisionFixture(t, ctx, store, haftDir, "Blocked lifecycle", "internal/cli/serve_commission.go")

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "create_from_decision",
		"decision_ref":  decision.Meta.ID,
		"repo_ref":      "local:haft",
		"base_sha":      "base-r1",
		"target_branch": "dev",
		"valid_until":   "2099-01-01T00:00:00Z",
		"spec_readiness_override": map[string]any{
			"kind":              "tactical",
			"out_of_spec":       true,
			"project_readiness": "needs_onboard",
			"reason":            "unit test fixture without project spec carriers",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	created := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &created); err != nil {
		t.Fatal(err)
	}
	commissionID := created["commission"]["id"].(string)

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": commissionID,
		"runner_id":     "open-sleigh:test",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "start_after_preflight",
		"commission_id": commissionID,
		"runner_id":     "open-sleigh:test",
		"event":         "preflight_passed",
		"verdict":       "pass",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "complete_or_block",
		"commission_id": commissionID,
		"runner_id":     "open-sleigh:test",
		"event":         "phase_blocked",
		"verdict":       "blocked",
		"reason":        "semantic gate failed",
	})
	if err != nil {
		t.Fatal(err)
	}

	blocked := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &blocked); err != nil {
		t.Fatal(err)
	}

	commission := blocked["commission"]
	if commission["state"] != "blocked_policy" {
		t.Fatalf("state = %#v, want blocked_policy", commission["state"])
	}

	events, ok := commission["events"].([]any)
	if !ok || len(events) != 2 {
		t.Fatalf("events = %#v, want two lifecycle events", commission["events"])
	}

	lastEvent, ok := events[len(events)-1].(map[string]any)
	if !ok {
		t.Fatalf("last event = %#v, want object", events[len(events)-1])
	}
	if lastEvent["event"] != "phase_blocked" {
		t.Fatalf("event = %#v, want phase_blocked", lastEvent["event"])
	}
	if lastEvent["verdict"] != "blocked" {
		t.Fatalf("verdict = %#v, want blocked", lastEvent["verdict"])
	}
}

func TestHandleHaftCommission_CompleteOrBlockRecordsProjectionDebtForExternalRequired(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	commission := workCommissionFixture("wc-external-required", "running", "2099-01-01T00:00:00Z")
	commission["projection_policy"] = "external_required"

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": commission,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "complete_or_block",
		"commission_id": "wc-external-required",
		"runner_id":     "open-sleigh:test",
		"event":         "workflow_terminal",
		"verdict":       "pass",
		"payload": map[string]any{
			"external_publication": map[string]any{
				"state":        "failed",
				"carrier":      "linear",
				"target":       "LIN-123",
				"last_error":   "permission_denied",
				"retry_policy": "operator_retry_after_credentials",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	completed := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &completed); err != nil {
		t.Fatal(err)
	}

	got := completed["commission"]
	if got["state"] != "completed_with_projection_debt" {
		t.Fatalf("state = %#v, want completed_with_projection_debt", got["state"])
	}

	debt, ok := got["projection_debt"].(map[string]any)
	if !ok {
		t.Fatalf("projection_debt = %#v, want object", got["projection_debt"])
	}
	if debt["carrier"] != "linear" || debt["target"] != "LIN-123" {
		t.Fatalf("projection_debt = %#v, want carrier and target", debt)
	}
	if debt["last_error"] != "permission_denied" {
		t.Fatalf("projection_debt last_error = %#v", debt["last_error"])
	}

	localExecution, ok := got["local_execution"].(map[string]any)
	if !ok {
		t.Fatalf("local_execution = %#v, want object", got["local_execution"])
	}
	if localExecution["verdict"] != "pass" {
		t.Fatalf("local_execution verdict = %#v, want pass", localExecution["verdict"])
	}
	if localExecution["runtime_run_id"] == "" {
		t.Fatalf("local_execution runtime_run_id = %#v, want local runtime evidence ref", localExecution["runtime_run_id"])
	}
}

func TestHandleHaftCommission_CompleteOrBlockKeepsLocalOnlyCompletionUnaffected(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": workCommissionFixture("wc-local-only-pass", "running", "2099-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "complete_or_block",
		"commission_id": "wc-local-only-pass",
		"runner_id":     "open-sleigh:test",
		"event":         "workflow_terminal",
		"verdict":       "pass",
	})
	if err != nil {
		t.Fatal(err)
	}

	completed := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &completed); err != nil {
		t.Fatal(err)
	}

	got := completed["commission"]
	if got["state"] != "completed" {
		t.Fatalf("state = %#v, want completed", got["state"])
	}
	if _, ok := got["projection_debt"]; ok {
		t.Fatalf("projection_debt = %#v, want absent", got["projection_debt"])
	}
}

func TestHandleHaftCommission_RecordRunEventPersistsRuntimeRunRefDuringPreflight(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision := createCommissionDecisionFixture(t, ctx, store, haftDir, "RuntimeRun lifecycle", "internal/cli/serve_commission.go")

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "create_from_decision",
		"decision_ref":  decision.Meta.ID,
		"repo_ref":      "local:haft",
		"base_sha":      "base-r1",
		"target_branch": "dev",
		"valid_until":   "2099-01-01T00:00:00Z",
		"spec_readiness_override": map[string]any{
			"kind":              "tactical",
			"out_of_spec":       true,
			"project_readiness": "needs_onboard",
			"reason":            "unit test fixture without project spec carriers",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	created := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &created); err != nil {
		t.Fatal(err)
	}
	commissionID := created["commission"]["id"].(string)

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": commissionID,
		"runner_id":     "open-sleigh:test",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "record_run_event",
		"commission_id": commissionID,
		"runner_id":     "open-sleigh:test",
		"event":         "phase_outcome",
		"verdict":       "pass",
		"payload": map[string]any{
			"phase": "preflight",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	recorded := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &recorded); err != nil {
		t.Fatal(err)
	}

	commission := recorded["commission"]
	if commission["state"] != "preflighting" {
		t.Fatalf("state = %#v, want preflighting", commission["state"])
	}

	events, ok := commission["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("events = %#v, want one runtime event", commission["events"])
	}

	event, ok := events[0].(map[string]any)
	if !ok {
		t.Fatalf("event = %#v, want object", events[0])
	}
	if event["runtime_run_id"] != commissionID+"#runtime-run-001" {
		t.Fatalf("runtime_run_id = %#v, want deterministic attempt ref", event["runtime_run_id"])
	}
	if event["runner_id"] != "open-sleigh:test" {
		t.Fatalf("runner_id = %#v, want open-sleigh:test", event["runner_id"])
	}
}

func TestHandleHaftCommission_CommissionLifecycleRejectsOutOfOrderEvents(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": workCommissionFixture("wc-lifecycle-order", "queued", "2099-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, action := range []string{"record_preflight", "start_after_preflight", "complete_or_block"} {
		_, err = handleHaftCommission(ctx, store, map[string]any{
			"action":        action,
			"commission_id": "wc-lifecycle-order",
			"runner_id":     "open-sleigh:test",
			"event":         "phase_outcome",
			"verdict":       "pass",
		})
		if err == nil {
			t.Fatalf("expected %s from queued commission to fail", action)
		}
		if !strings.Contains(err.Error(), "commission_lifecycle_forbidden") {
			t.Fatalf("err = %v, want commission_lifecycle_forbidden", err)
		}
	}

	stored, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "show",
		"commission_id": "wc-lifecycle-order",
	})
	if err != nil {
		t.Fatal(err)
	}

	shown := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(stored), &shown); err != nil {
		t.Fatal(err)
	}
	if shown["commission"]["state"] != "queued" {
		t.Fatalf("state = %#v, want queued", shown["commission"]["state"])
	}
	if events, ok := shown["commission"]["events"].([]any); ok && len(events) > 0 {
		t.Fatalf("events = %#v, want no rejected lifecycle events", events)
	}
}

func TestHandleHaftCommission_CreateFromDecisionBuildsRunnableCommission(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	projectRoot := writeCommissionSpecCarriers(t, "TS.commission.001")

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
		SectionRefs:         []string{"TS.commission.001"},
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
		"project_root":    projectRoot,
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
	if commission["delivery_policy"] != defaultDeliveryPolicy {
		t.Fatalf("delivery_policy = %#v, want %s", commission["delivery_policy"], defaultDeliveryPolicy)
	}
	if !hexLike(commission["decision_revision_hash"]) {
		t.Fatalf("decision_revision_hash = %#v, want sha256 hex", commission["decision_revision_hash"])
	}
	if !hexLike(commission["scope_hash"]) {
		t.Fatalf("scope_hash = %#v, want sha256 hex", commission["scope_hash"])
	}
	if !containsAnyString(commission["spec_section_refs"], "TS.commission.001") {
		t.Fatalf("spec_section_refs = %#v, want decision section ref", commission["spec_section_refs"])
	}

	revisionHashes, ok := commission["spec_revision_hashes"].(map[string]any)
	if !ok {
		t.Fatalf("spec_revision_hashes = %#v, want object", commission["spec_revision_hashes"])
	}
	if !hexLike(revisionHashes["TS.commission.001"]) {
		t.Fatalf("spec revision hash = %#v, want sha256 hex", revisionHashes["TS.commission.001"])
	}

	specSnapshot, ok := commission["spec_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("spec_snapshot = %#v, want object", commission["spec_snapshot"])
	}
	if specSnapshot["snapshot_source"] != "project_specification_set" {
		t.Fatalf("snapshot_source = %#v, want project_specification_set", specSnapshot["snapshot_source"])
	}
	if specSnapshot["snapshot_state"] != "resolved" {
		t.Fatalf("snapshot_state = %#v, want resolved", specSnapshot["snapshot_state"])
	}
	if !containsAnyString(specSnapshot["section_refs"], "TS.commission.001") {
		t.Fatalf("snapshot section_refs = %#v, want decision section ref", specSnapshot["section_refs"])
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
	if !ok || len(requirements) != 2 {
		t.Fatalf("evidence_requirements = %#v, want decision and spec requirements", commission["evidence_requirements"])
	}

	requirement, ok := requirements[0].(map[string]any)
	if !ok || requirement["command"] != "go test ./internal/cli" {
		t.Fatalf("evidence requirement = %#v, want command from decision", requirements[0])
	}
	specRequirement, ok := requirements[1].(map[string]any)
	if !ok || specRequirement["spec_section_ref"] != "TS.commission.001" {
		t.Fatalf("spec evidence requirement = %#v, want section-scoped requirement", requirements[1])
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

func TestHandleHaftCommission_CreateFromDecisionRequiresSpecRefsOrTacticalOverride(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Non-spec tactical work",
		Signal:     "A local fix has no governing spec section yet.",
		Acceptance: "Commission creation records that the work is explicitly out-of-spec tactical.",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   "Allow explicit tactical commission",
		WhySelected:     "The harness needs a visible exception instead of silently inventing spec authority.",
		SelectionPolicy: "Prefer rejecting missing refs unless the user records a tactical reason.",
		CounterArgument: "Blocking all no-spec work could slow small local fixes.",
		WeakestLink:     "The override reason must remain visible on the commission snapshot.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Silent non-spec commission",
			Reason:  "It hides the missing authority edge.",
		}},
		Rollback:      &artifact.RollbackSpec{Triggers: []string{"Tactical override is missing from the commission."}},
		AffectedFiles: []string{"internal/cli/serve_commission.go"},
		ValidUntil:    "2099-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	baseArgs := map[string]any{
		"action":        "create_from_decision",
		"decision_ref":  decision.Meta.ID,
		"repo_ref":      "local:haft",
		"base_sha":      "base-r1",
		"target_branch": "dev",
		"valid_until":   "2099-01-01T00:00:00Z",
	}

	_, err = handleHaftCommission(ctx, store, baseArgs)
	if err == nil || !strings.Contains(err.Error(), "spec_section_refs is required") {
		t.Fatalf("error = %v, want missing spec refs rejection", err)
	}

	tacticalArgs := copyStringAnyMap(baseArgs)
	tacticalArgs["spec_readiness_override"] = map[string]any{
		"kind":              "tactical",
		"out_of_spec":       true,
		"project_readiness": "needs_onboard",
		"reason":            "urgent local repair before spec onboarding",
	}

	result, err := handleHaftCommission(ctx, store, tacticalArgs)
	if err != nil {
		t.Fatal(err)
	}

	created := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &created); err != nil {
		t.Fatal(err)
	}

	override, ok := created["commission"]["spec_readiness_override"].(map[string]any)
	if !ok {
		t.Fatalf("spec_readiness_override = %#v, want object", created["commission"]["spec_readiness_override"])
	}
	if override["out_of_spec"] != true {
		t.Fatalf("override out_of_spec = %#v, want true", override["out_of_spec"])
	}
	if override["reason"] != "urgent local repair before spec onboarding" {
		t.Fatalf("override reason = %#v", override["reason"])
	}
}

func TestHandleHaftCommission_StartAfterPreflightBlocksFreshnessDrift(t *testing.T) {
	tests := []struct {
		name      string
		wantCode  string
		startArgs map[string]any
		drift     func(*testing.T, context.Context, *artifact.Store, commissionFreshnessFixture)
	}{
		{
			name:     "decision revision hash",
			wantCode: "decision_revision_hash_changed",
			drift: func(t *testing.T, ctx context.Context, store *artifact.Store, fixture commissionFreshnessFixture) {
				t.Helper()

				fixture.Decision.Body += "\n\nRuntime changed this decision after commission queue."
				if err := store.Update(ctx, fixture.Decision); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:     "problem revision hash",
			wantCode: "problem_revision_hash_changed",
			drift: func(t *testing.T, ctx context.Context, store *artifact.Store, fixture commissionFreshnessFixture) {
				t.Helper()

				fixture.Problem.Body += "\n\nRuntime changed this problem after commission queue."
				if err := store.Update(ctx, fixture.Problem); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:      "base SHA",
			wantCode:  "admitted_base_sha_changed",
			startArgs: map[string]any{"base_sha": "base-r2"},
			drift: func(t *testing.T, _ context.Context, _ *artifact.Store, _ commissionFreshnessFixture) {
				t.Helper()
			},
		},
		{
			name:     "scope hash",
			wantCode: "scope_hash_changed",
			drift: func(t *testing.T, ctx context.Context, store *artifact.Store, fixture commissionFreshnessFixture) {
				t.Helper()

				updateStoredCommissionForTest(t, ctx, store, fixture.CommissionID, func(commission map[string]any) {
					scope, ok := mapArg(commission, "scope")
					if !ok {
						t.Fatal("scope missing")
					}
					scope["allowed_paths"] = append(scope["allowed_paths"].([]any), "internal/cli/serve.go")
				})
			},
		},
		{
			name:     "spec revision hash",
			wantCode: "spec_revision_hash_changed",
			drift: func(t *testing.T, _ context.Context, _ *artifact.Store, fixture commissionFreshnessFixture) {
				t.Helper()

				writeSpecCheckCLIFile(
					t,
					filepath.Join(fixture.ProjectRoot, ".haft", "specs", "target-system.md"),
					commissionCLISpecSection(fixture.SectionRef, "Changed commission authority"),
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := setupCLIArtifactStore(t)
			ctx := context.Background()
			fixture := createClaimedFreshnessCommission(t, ctx, store)

			test.drift(t, ctx, store, fixture)

			args := map[string]any{
				"action":        "start_after_preflight",
				"commission_id": fixture.CommissionID,
				"runner_id":     "open-sleigh:test",
				"event":         "preflight_passed",
				"verdict":       "pass",
				"project_root":  fixture.ProjectRoot,
			}
			for key, value := range test.startArgs {
				args[key] = value
			}

			_, err := handleHaftCommission(ctx, store, args)
			if err == nil {
				t.Fatal("expected stale commission start to fail")
			}
			if !strings.Contains(err.Error(), "commission_freshness_blocked") {
				t.Fatalf("err = %v, want commission_freshness_blocked", err)
			}
			if !strings.Contains(err.Error(), test.wantCode) {
				t.Fatalf("err = %v, want %s", err, test.wantCode)
			}

			commission, err := loadWorkCommissionPayload(ctx, store, fixture.CommissionID)
			if err != nil {
				t.Fatal(err)
			}
			if commission["state"] != "blocked_stale" {
				t.Fatalf("state = %#v, want blocked_stale", commission["state"])
			}
			if !commissionHasFreshnessIssue(commission, test.wantCode) {
				t.Fatalf("events = %#v, want freshness issue %s", commission["events"], test.wantCode)
			}
		})
	}
}

func TestHandleHaftCommission_StartAfterPreflightRecordsDeferredPlanGap(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision := createCommissionDecisionFixture(t, ctx, store, haftDir, "Plan gap", "internal/cli/serve_commission.go")
	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":                       "create_from_decision",
		"decision_ref":                 decision.Meta.ID,
		"repo_ref":                     "local:haft",
		"base_sha":                     "base-r1",
		"target_branch":                "dev",
		"implementation_plan_ref":      "plan-gap-001",
		"implementation_plan_revision": "plan-r1",
		"valid_until":                  "2099-01-01T00:00:00Z",
		"spec_readiness_override": map[string]any{
			"kind":              "tactical",
			"out_of_spec":       true,
			"project_readiness": "needs_onboard",
			"reason":            "unit test fixture without project spec carriers",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	created := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &created); err != nil {
		t.Fatal(err)
	}
	commissionID := created["commission"]["id"].(string)

	runCommissionStartAfterPreflight(t, ctx, store, commissionID, nil)

	commission, err := loadWorkCommissionPayload(ctx, store, commissionID)
	if err != nil {
		t.Fatal(err)
	}
	if commission["state"] != "running" {
		t.Fatalf("state = %#v, want running", commission["state"])
	}
	if !commissionHasFreshnessGap(commission, "implementation_plan_gate_deferred") {
		t.Fatalf("events = %#v, want deferred implementation plan gap", commission["events"])
	}
}

func TestHandleHaftCommission_CreateBatchFromDecisionsBuildsRunnableCommissions(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	first := createCommissionDecisionFixture(t, ctx, store, haftDir, "Batch first", "internal/cli/commission.go")
	second := createCommissionDecisionFixture(t, ctx, store, haftDir, "Batch second", "internal/cli/serve_commission.go")

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":          "create_batch_from_decisions",
		"decision_refs":   []any{first.Meta.ID, second.Meta.ID},
		"repo_ref":        "local:haft",
		"base_sha":        "base-r1",
		"target_branch":   "dev",
		"allowed_actions": []any{"edit_files", "run_tests"},
		"valid_until":     "2099-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	created := map[string][]map[string]any{}
	if err := json.Unmarshal([]byte(result), &created); err != nil {
		t.Fatal(err)
	}

	if len(created["commissions"]) != 2 {
		t.Fatalf("created commissions = %#v, want two", created["commissions"])
	}

	seen := map[string]bool{}
	for _, commission := range created["commissions"] {
		seen[commission["decision_ref"].(string)] = true
		if commission["state"] != "queued" {
			t.Fatalf("commission state = %#v, want queued", commission["state"])
		}
		if !hexLike(commission["scope_hash"]) {
			t.Fatalf("scope_hash = %#v, want sha256 hex", commission["scope_hash"])
		}
	}

	if !seen[first.Meta.ID] || !seen[second.Meta.ID] {
		t.Fatalf("created decision refs = %#v, want both decisions", seen)
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
	if len(listed["commissions"]) != 2 {
		t.Fatalf("listed commissions = %#v, want two runnable commissions", listed["commissions"])
	}
}

func TestHandleHaftCommission_CreateFromPlanBuildsRunnableCommissions(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	first := createCommissionDecisionFixture(t, ctx, store, haftDir, "Plan first", "internal/cli/commission.go")
	second := createCommissionDecisionFixture(t, ctx, store, haftDir, "Plan second", "internal/cli/serve_commission.go")

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action": "create_from_plan",
		"plan": map[string]any{
			"id":                "plan-cli-001",
			"revision":          "p1",
			"repo_ref":          "local:haft",
			"base_sha":          "base-r1",
			"target_branch":     "dev",
			"valid_until":       "2099-01-01T00:00:00Z",
			"projection_policy": "local_only",
			"delivery_policy":   "workspace_patch_auto_on_pass",
			"defaults": map[string]any{
				"allowed_actions":       []any{"edit_files", "run_tests"},
				"evidence_requirements": []any{"go test ./internal/cli"},
			},
			"decisions": []any{
				map[string]any{
					"ref": first.Meta.ID,
					"tags": []any{
						"cli",
					},
				},
				map[string]any{
					"ref": second.Meta.ID,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	created := map[string]any{}
	if err := json.Unmarshal([]byte(result), &created); err != nil {
		t.Fatal(err)
	}

	commissions, ok := created["commissions"].([]any)
	if !ok || len(commissions) != 2 {
		t.Fatalf("commissions = %#v, want two", created["commissions"])
	}

	for _, raw := range commissions {
		commission, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("commission = %#v, want object", raw)
		}
		if commission["implementation_plan_ref"] != "plan-cli-001" {
			t.Fatalf("implementation_plan_ref = %#v, want plan-cli-001", commission["implementation_plan_ref"])
		}
		if commission["implementation_plan_revision"] != "p1" {
			t.Fatalf("implementation_plan_revision = %#v, want p1", commission["implementation_plan_revision"])
		}
		if commission["delivery_policy"] != "workspace_patch_auto_on_pass" {
			t.Fatalf("delivery_policy = %#v, want workspace_patch_auto_on_pass", commission["delivery_policy"])
		}
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
	if len(listed["commissions"]) != 2 {
		t.Fatalf("listed commissions = %#v, want two runnable commissions", listed["commissions"])
	}
}

func TestHandleHaftCommission_CreateFromPlanSchedulesDependencies(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	first := createCommissionDecisionFixture(t, ctx, store, haftDir, "Plan dependency first", "internal/cli/commission.go")
	second := createCommissionDecisionFixture(t, ctx, store, haftDir, "Plan dependency second", "internal/cli/serve_commission.go")

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action": "create_from_plan",
		"plan": map[string]any{
			"id":            "plan-cli-deps",
			"revision":      "p1",
			"repo_ref":      "local:haft",
			"base_sha":      "base-r1",
			"target_branch": "dev",
			"valid_until":   "2099-01-01T00:00:00Z",
			"decisions": []any{
				map[string]any{"ref": first.Meta.ID},
				map[string]any{
					"ref":        second.Meta.ID,
					"depends_on": []any{first.Meta.ID},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	created := map[string]any{}
	if err := json.Unmarshal([]byte(result), &created); err != nil {
		t.Fatal(err)
	}

	commissions := created["commissions"].([]any)
	firstID := commissionIDForDecision(t, commissions, first.Meta.ID)
	secondID := commissionIDForDecision(t, commissions, second.Meta.ID)
	secondCommission := commissionForDecision(t, commissions, second.Meta.ID)

	if !containsAnyString(secondCommission["depends_on"], firstID) {
		t.Fatalf("depends_on = %#v, want first commission id %s", secondCommission["depends_on"], firstID)
	}

	listed := map[string][]map[string]any{}
	listResult, err := handleHaftCommission(ctx, store, map[string]any{
		"action":   "list_runnable",
		"plan_ref": "plan-cli-deps",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(listResult), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed["commissions"]) != 1 {
		t.Fatalf("listed commissions = %#v, want only root dependency runnable", listed["commissions"])
	}
	if listed["commissions"][0]["id"] != firstID {
		t.Fatalf("listed commission id = %#v, want %s", listed["commissions"][0]["id"], firstID)
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": secondID,
		"runner_id":     "open-sleigh:test",
	})
	if err == nil || !strings.Contains(err.Error(), "commission_not_runnable") {
		t.Fatalf("claim error = %v, want dependency-blocked commission_not_runnable", err)
	}

	runCommissionThroughPreflight(t, ctx, store, firstID)

	listed = map[string][]map[string]any{}
	listResult, err = handleHaftCommission(ctx, store, map[string]any{
		"action":   "list_runnable",
		"plan_ref": "plan-cli-deps",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(listResult), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed["commissions"]) != 1 {
		t.Fatalf("listed commissions = %#v, want dependent commission runnable", listed["commissions"])
	}
	if listed["commissions"][0]["id"] != secondID {
		t.Fatalf("listed commission id = %#v, want %s", listed["commissions"][0]["id"], secondID)
	}

	claimed := map[string]map[string]any{}
	claimResult, err := handleHaftCommission(ctx, store, map[string]any{
		"action":    "claim_for_preflight",
		"runner_id": "open-sleigh:test",
		"plan_ref":  "plan-cli-deps",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(claimResult), &claimed); err != nil {
		t.Fatal(err)
	}
	if claimed["commission"]["id"] != secondID {
		t.Fatalf("claimed commission id = %#v, want %s", claimed["commission"]["id"], secondID)
	}
}

func TestHandleHaftCommission_CreateFromPlanRejectsUnknownDependency(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	first := createCommissionDecisionFixture(t, ctx, store, haftDir, "Plan unknown dependency", "internal/cli/commission.go")

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action": "create_from_plan",
		"plan": map[string]any{
			"id":            "plan-cli-unknown-dep",
			"revision":      "p1",
			"repo_ref":      "local:haft",
			"base_sha":      "base-r1",
			"target_branch": "dev",
			"valid_until":   "2099-01-01T00:00:00Z",
			"decisions": []any{
				map[string]any{
					"ref":        first.Meta.ID,
					"depends_on": []any{"dec-missing"},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "depends on unknown decision") {
		t.Fatalf("error = %v, want unknown dependency rejection", err)
	}
}

func TestHandleHaftCommission_CreateFromPlanRejectsDependencyCycle(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	first := createCommissionDecisionFixture(t, ctx, store, haftDir, "Plan cycle first", "internal/cli/commission.go")
	second := createCommissionDecisionFixture(t, ctx, store, haftDir, "Plan cycle second", "internal/cli/serve_commission.go")

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action": "create_from_plan",
		"plan": map[string]any{
			"id":            "plan-cli-cycle",
			"revision":      "p1",
			"repo_ref":      "local:haft",
			"base_sha":      "base-r1",
			"target_branch": "dev",
			"valid_until":   "2099-01-01T00:00:00Z",
			"decisions": []any{
				map[string]any{
					"ref":        first.Meta.ID,
					"depends_on": []any{second.Meta.ID},
				},
				map[string]any{
					"ref":        second.Meta.ID,
					"depends_on": []any{first.Meta.ID},
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("error = %v, want dependency cycle rejection", err)
	}
}

func TestHandleHaftCommission_RunnableFilterMatchesPlanRevision(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	for _, revision := range []string{"p1", "p2"} {
		commission := workCommissionFixture("wc-plan-"+revision, "queued", "2099-01-01T00:00:00Z")
		commission["decision_ref"] = "dec-plan-" + revision
		commission["implementation_plan_ref"] = "plan-revisioned"
		commission["implementation_plan_revision"] = revision

		_, err := handleHaftCommission(ctx, store, map[string]any{
			"action":     "create",
			"commission": commission,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	listed := map[string][]map[string]any{}
	listResult, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "list_runnable",
		"plan_ref":      "plan-revisioned",
		"plan_revision": "p2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(listResult), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed["commissions"]) != 1 {
		t.Fatalf("listed commissions = %#v, want one p2 commission", listed["commissions"])
	}
	if listed["commissions"][0]["id"] != "wc-plan-p2" {
		t.Fatalf("listed commission = %#v, want wc-plan-p2", listed["commissions"][0]["id"])
	}

	claimed := map[string]map[string]any{}
	claimResult, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"runner_id":     "open-sleigh:test",
		"plan_ref":      "plan-revisioned",
		"plan_revision": "p2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(claimResult), &claimed); err != nil {
		t.Fatal(err)
	}
	if claimed["commission"]["id"] != "wc-plan-p2" {
		t.Fatalf("claimed commission = %#v, want wc-plan-p2", claimed["commission"]["id"])
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
		SectionRefs:     []string{"TS.commission.scope"},
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

func TestHandleHaftCommission_AutonomyEnvelopeRejectsOutOfEnvelopeAction(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision := createCommissionDecisionFixture(t, ctx, store, haftDir, "Envelope action", "internal/cli/serve_commission.go")

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":                     "create_from_decision",
		"decision_ref":               decision.Meta.ID,
		"repo_ref":                   "local:haft",
		"base_sha":                   "base-r1",
		"target_branch":              "dev",
		"allowed_actions":            []any{"edit_files", "tag_release"},
		"autonomy_envelope_snapshot": autonomyEnvelopeSnapshotFixture("2099-01-01T00:00:00Z"),
		"valid_until":                "2099-01-01T00:00:00Z",
	})
	if err == nil {
		t.Fatal("expected out-of-envelope action to fail")
	}
	if !strings.Contains(err.Error(), "commission_autonomy_envelope_blocked") {
		t.Fatalf("err = %v, want commission_autonomy_envelope_blocked", err)
	}
	if !strings.Contains(err.Error(), "one_way_door_action_forbidden") {
		t.Fatalf("err = %v, want one_way_door_action_forbidden", err)
	}
}

func TestHandleHaftCommission_AutonomyEnvelopeCannotSkipRequiredGates(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	commission := workCommissionFixture("wc-env-skip", "queued", "2099-01-01T00:00:00Z")
	envelope := autonomyEnvelopeSnapshotFixture("2099-01-01T00:00:00Z")
	envelope["skip_gates"] = []any{"freshness", "scope", "evidence"}
	commission["autonomy_envelope_snapshot"] = envelope

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": commission,
	})
	if err == nil {
		t.Fatal("expected gate-skipping envelope to fail")
	}
	if !strings.Contains(err.Error(), "autonomy_envelope_gate_skip_forbidden") {
		t.Fatalf("err = %v, want autonomy_envelope_gate_skip_forbidden", err)
	}
}

func TestHandleHaftCommission_StartAfterPreflightBlocksExpiredOrRevokedEnvelope(t *testing.T) {
	tests := []struct {
		name   string
		code   string
		mutate func(map[string]any)
	}{
		{
			name: "expired",
			code: "autonomy_envelope_expired",
			mutate: func(envelope map[string]any) {
				envelope["valid_until"] = "2000-01-01T00:00:00Z"
			},
		},
		{
			name: "revoked",
			code: "autonomy_envelope_revoked",
			mutate: func(envelope map[string]any) {
				envelope["state"] = "revoked"
				envelope["revoked_at"] = "2026-04-22T10:00:00Z"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := setupCLIArtifactStore(t)
			ctx := context.Background()
			haftDir := t.TempDir()
			decision := createCommissionDecisionFixture(t, ctx, store, haftDir, "Envelope "+test.name, "internal/cli/serve_commission.go")

			result, err := handleHaftCommission(ctx, store, map[string]any{
				"action":                     "create_from_decision",
				"decision_ref":               decision.Meta.ID,
				"repo_ref":                   "local:haft",
				"base_sha":                   "base-r1",
				"target_branch":              "dev",
				"autonomy_envelope_snapshot": autonomyEnvelopeSnapshotFixture("2099-01-01T00:00:00Z"),
				"valid_until":                "2099-01-01T00:00:00Z",
			})
			if err != nil {
				t.Fatal(err)
			}

			created := map[string]map[string]any{}
			if err := json.Unmarshal([]byte(result), &created); err != nil {
				t.Fatal(err)
			}
			commissionID := created["commission"]["id"].(string)

			_, err = handleHaftCommission(ctx, store, map[string]any{
				"action":        "claim_for_preflight",
				"commission_id": commissionID,
				"runner_id":     "open-sleigh:test",
			})
			if err != nil {
				t.Fatal(err)
			}

			updateStoredCommissionForTest(t, ctx, store, commissionID, func(commission map[string]any) {
				envelope, ok := mapArg(commission, "autonomy_envelope_snapshot")
				if !ok {
					t.Fatal("autonomy_envelope_snapshot missing")
				}
				test.mutate(envelope)
				delete(envelope, "hash")
			})

			_, err = handleHaftCommission(ctx, store, map[string]any{
				"action":        "start_after_preflight",
				"commission_id": commissionID,
				"runner_id":     "open-sleigh:test",
				"event":         "preflight_passed",
				"verdict":       "pass",
			})
			if err == nil {
				t.Fatal("expected expired or revoked envelope to block start")
			}
			if !strings.Contains(err.Error(), "commission_autonomy_envelope_blocked") {
				t.Fatalf("err = %v, want commission_autonomy_envelope_blocked", err)
			}
			if !strings.Contains(err.Error(), test.code) {
				t.Fatalf("err = %v, want %s", err, test.code)
			}

			commission, err := loadWorkCommissionPayload(ctx, store, commissionID)
			if err != nil {
				t.Fatal(err)
			}
			if commission["state"] != "blocked_policy" {
				t.Fatalf("state = %#v, want blocked_policy", commission["state"])
			}
		})
	}
}

func TestHandleHaftCommission_NoEnvelopeRemainsManualRunnable(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": workCommissionFixture("wc-no-envelope", "queued", "2099-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatal(err)
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
		t.Fatalf("listed commissions = %#v, want one manual runnable commission", listed["commissions"])
	}
	if _, ok := listed["commissions"][0]["autonomy_envelope_snapshot"]; ok {
		t.Fatalf("autonomy_envelope_snapshot = %#v, want absent", listed["commissions"][0]["autonomy_envelope_snapshot"])
	}
}

func TestHandleQuintCommission_AutoApply(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()

	commission := workCommissionFixture("wc-auto-apply", "running", "2099-01-01T00:00:00Z")
	commission["delivery_policy"] = "workspace_patch_auto_on_pass"
	commission["autonomy_envelope_snapshot"] = autonomyEnvelopeSnapshotFixture("2099-01-01T00:00:00Z")

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": commission,
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "complete_or_block",
		"commission_id": "wc-auto-apply",
		"runner_id":     "open-sleigh:test",
		"event":         "workflow_terminal",
		"verdict":       "pass",
	})
	if err != nil {
		t.Fatal(err)
	}

	completed := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &completed); err != nil {
		t.Fatal(err)
	}

	got := completed["commission"]
	if got["state"] != "completed" {
		t.Fatalf("state = %#v, want completed", got["state"])
	}

	decision, ok := got["delivery_decision"].(map[string]any)
	if !ok {
		t.Fatalf("delivery_decision = %#v, want object", got["delivery_decision"])
	}
	if decision["action"] != "auto_apply" {
		t.Fatalf("delivery action = %#v, want auto_apply", decision["action"])
	}
	if decision["auto_apply"] != true {
		t.Fatalf("delivery auto_apply = %#v, want true", decision["auto_apply"])
	}
	if decision["autonomy_envelope_decision"] != "allowed" {
		t.Fatalf("envelope decision = %#v, want allowed", decision["autonomy_envelope_decision"])
	}

	autoApply, ok := got["auto_apply"].(map[string]any)
	if !ok || autoApply["allowed"] != true {
		t.Fatalf("auto_apply = %#v, want allowed", got["auto_apply"])
	}
}

func TestStaleLeaseCap(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	stale := workCommissionFixture("wc-stale-lease", "queued", "2099-01-01T00:00:00Z")
	stale["lease"] = map[string]any{
		"runner_id":  "open-sleigh:old",
		"state":      "claimed_for_preflight",
		"claimed_at": now.Add(-25 * time.Hour).Format(time.RFC3339),
	}

	fresh := workCommissionFixture("wc-fresh-lease", "queued", "2099-01-01T00:00:00Z")
	fresh["decision_ref"] = "dec-fresh-lease"

	for _, commission := range []map[string]any{stale, fresh} {
		_, err := handleHaftCommission(ctx, store, map[string]any{
			"action":     "create",
			"commission": commission,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "list_runnable",
		"lease_age_cap": "24h",
	})
	if err != nil {
		t.Fatal(err)
	}

	listed := map[string][]map[string]any{}
	if err := json.Unmarshal([]byte(result), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed["commissions"]) != 1 {
		t.Fatalf("runnable commissions = %#v, want fresh commission only", listed["commissions"])
	}
	if listed["commissions"][0]["id"] != "wc-fresh-lease" {
		t.Fatalf("runnable commission = %#v, want wc-fresh-lease", listed["commissions"][0]["id"])
	}
	if len(listed["skipped"]) != 1 {
		t.Fatalf("skipped = %#v, want one stale lease", listed["skipped"])
	}
	if listed["skipped"][0]["reason"] != "lease_too_old" {
		t.Fatalf("skip reason = %#v, want lease_too_old", listed["skipped"][0]["reason"])
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": "wc-stale-lease",
		"runner_id":     "open-sleigh:test",
		"lease_age_cap": "24h",
	})
	if err == nil || !strings.Contains(err.Error(), "lease_too_old") {
		t.Fatalf("claim error = %v, want lease_too_old", err)
	}

	showResult, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "show",
		"commission_id": "wc-stale-lease",
		"lease_age_cap": "24h",
	})
	if err != nil {
		t.Fatal(err)
	}

	shown := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(showResult), &shown); err != nil {
		t.Fatal(err)
	}

	operator, ok := shown["commission"]["operator"].(map[string]any)
	if !ok {
		t.Fatalf("operator = %#v, want object", shown["commission"]["operator"])
	}
	if operator["attention_code"] != "lease_too_old" {
		t.Fatalf("attention_code = %#v, want lease_too_old", operator["attention_code"])
	}
}

func TestHarnessRun_Drain(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	runnable := workCommissionFixture("wc-drain-runnable", "queued", "2099-01-01T00:00:00Z")
	stale := workCommissionFixture("wc-drain-stale", "queued", "2099-01-01T00:00:00Z")
	stale["decision_ref"] = "dec-drain-stale"
	stale["lease"] = map[string]any{
		"runner_id":  "open-sleigh:old",
		"state":      "claimed_for_preflight",
		"claimed_at": now.Add(-25 * time.Hour).Format(time.RFC3339),
	}

	for _, commission := range []map[string]any{runnable, stale} {
		_, err := handleHaftCommission(ctx, store, map[string]any{
			"action":     "create",
			"commission": commission,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "drain_status",
		"lease_age_cap": "24h",
	})
	if err != nil {
		t.Fatal(err)
	}

	status := map[string]any{}
	if err := json.Unmarshal([]byte(result), &status); err != nil {
		t.Fatal(err)
	}

	drain, ok := status["drain"].(map[string]any)
	if !ok {
		t.Fatalf("drain = %#v, want object", status["drain"])
	}
	if drain["empty"] != false {
		t.Fatalf("drain empty = %#v, want false", drain["empty"])
	}
	if drain["runnable_count"] != float64(1) {
		t.Fatalf("runnable_count = %#v, want 1", drain["runnable_count"])
	}
	if drain["skipped_count"] != float64(1) {
		t.Fatalf("skipped_count = %#v, want 1", drain["skipped_count"])
	}
}

func writeCommissionSpecCarriers(t *testing.T, sectionRefs ...string) string {
	t.Helper()

	root := t.TempDir()
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sections := make([]string, 0, len(sectionRefs))
	for _, ref := range sectionRefs {
		sections = append(sections, commissionCLISpecSection(ref, "Commission authority"))
	}

	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), strings.Join(sections, "\n"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "enabling-system.md"), coverageCLIDraftSpecSection("ES.commission.001", "Commission fixture"))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), validCLITermMapCarrier())

	return root
}

func commissionCLISpecSection(id string, title string) string {
	return "## " + id + " " + title + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"kind: environment-change\n" +
		"title: " + title + "\n" +
		"statement_type: definition\n" +
		"claim_layer: object\n" +
		"owner: human\n" +
		"status: active\n" +
		"evidence_required:\n" +
		"  - kind: review\n" +
		"    description: Confirm the commission preserves this section.\n" +
		"```\n"
}

type commissionFreshnessFixture struct {
	CommissionID string
	Decision     *artifact.Artifact
	Problem      *artifact.Artifact
	ProjectRoot  string
	SectionRef   string
}

func createClaimedFreshnessCommission(
	t *testing.T,
	ctx context.Context,
	store *artifact.Store,
) commissionFreshnessFixture {
	t.Helper()

	haftDir := t.TempDir()
	sectionRef := "TS.freshness.001"
	projectRoot := writeCommissionSpecCarriers(t, sectionRef)

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      "Freshness gate problem",
		Signal:     "A queued commission can drift before execution starts.",
		Acceptance: "Drift blocks before Execute.",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   "Gate commission freshness",
		WhySelected:     "The runtime needs Haft to enforce the snapshot before Execute.",
		SelectionPolicy: "Block hard deterministic mismatches at start_after_preflight.",
		CounterArgument: "Open-Sleigh already has structural gates.",
		WeakestLink:     "The Go lifecycle transition must not admit stale snapshots.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "Advisory warning",
			Reason:  "It would still let Execute start with stale authority.",
		}},
		Rollback:      &artifact.RollbackSpec{Triggers: []string{"Freshness mismatch enters running."}},
		EvidenceReqs:  []string{"go test ./internal/cli"},
		AffectedFiles: []string{"internal/cli/serve_commission.go"},
		SectionRefs:   []string{sectionRef},
		ValidUntil:    "2099-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "create_from_decision",
		"decision_ref":  decision.Meta.ID,
		"repo_ref":      "local:haft",
		"base_sha":      "base-r1",
		"target_branch": "dev",
		"project_root":  projectRoot,
		"valid_until":   "2099-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	created := map[string]map[string]any{}
	if err := json.Unmarshal([]byte(result), &created); err != nil {
		t.Fatal(err)
	}
	commissionID, ok := created["commission"]["id"].(string)
	if !ok || commissionID == "" {
		t.Fatalf("commission id = %#v, want string", created["commission"]["id"])
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": commissionID,
		"runner_id":     "open-sleigh:test",
	})
	if err != nil {
		t.Fatal(err)
	}

	return commissionFreshnessFixture{
		CommissionID: commissionID,
		Decision:     decision,
		Problem:      problem,
		ProjectRoot:  projectRoot,
		SectionRef:   sectionRef,
	}
}

func updateStoredCommissionForTest(
	t *testing.T,
	ctx context.Context,
	store *artifact.Store,
	commissionID string,
	mutate func(map[string]any),
) {
	t.Helper()

	tx, err := store.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	commission, err := loadWorkCommissionPayloadForUpdate(ctx, tx, commissionID)
	if err != nil {
		t.Fatal(err)
	}
	mutate(commission)
	if err := updateWorkCommissionPayload(ctx, tx, commission); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func commissionHasFreshnessIssue(commission map[string]any, code string) bool {
	events, ok := commission["events"].([]any)
	if !ok || len(events) == 0 {
		return false
	}

	event, ok := events[len(events)-1].(map[string]any)
	if !ok || event["event"] != "freshness_blocked" {
		return false
	}
	payload, ok := event["payload"].(map[string]any)
	if !ok {
		return false
	}
	return freshnessIssueListContains(payload["freshness_mismatches"], code)
}

func commissionHasFreshnessGap(commission map[string]any, code string) bool {
	events, ok := commission["events"].([]any)
	if !ok || len(events) == 0 {
		return false
	}

	event, ok := events[len(events)-1].(map[string]any)
	if !ok {
		return false
	}
	payload, ok := event["payload"].(map[string]any)
	if !ok {
		return false
	}
	return freshnessIssueListContains(payload["freshness_gaps"], code)
}

func freshnessIssueListContains(value any, code string) bool {
	issues, ok := value.([]any)
	if !ok {
		return false
	}

	for _, raw := range issues {
		issue, ok := raw.(map[string]any)
		if ok && issue["code"] == code {
			return true
		}
	}
	return false
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

func commissionIDForDecision(t *testing.T, commissions []any, decisionRef string) string {
	t.Helper()

	commission := commissionForDecision(t, commissions, decisionRef)
	id, ok := commission["id"].(string)
	if !ok || id == "" {
		t.Fatalf("commission id for %s = %#v, want string", decisionRef, commission["id"])
	}
	return id
}

func commissionForDecision(t *testing.T, commissions []any, decisionRef string) map[string]any {
	t.Helper()

	for _, raw := range commissions {
		commission, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("commission = %#v, want object", raw)
		}
		if commission["decision_ref"] == decisionRef {
			return commission
		}
	}

	t.Fatalf("missing commission for decision %s in %#v", decisionRef, commissions)
	return nil
}

func runCommissionThroughPreflight(
	t *testing.T,
	ctx context.Context,
	store *artifact.Store,
	commissionID string,
) {
	t.Helper()

	runCommissionStartAfterPreflight(t, ctx, store, commissionID, nil)

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "complete_or_block",
		"commission_id": commissionID,
		"runner_id":     "open-sleigh:test",
		"event":         "workflow_terminal",
		"verdict":       "completed",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func runCommissionStartAfterPreflight(
	t *testing.T,
	ctx context.Context,
	store *artifact.Store,
	commissionID string,
	extraArgs map[string]any,
) {
	t.Helper()

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": commissionID,
		"runner_id":     "open-sleigh:test",
	})
	if err != nil {
		t.Fatal(err)
	}

	args := map[string]any{
		"action":        "start_after_preflight",
		"commission_id": commissionID,
		"runner_id":     "open-sleigh:test",
		"event":         "preflight_passed",
		"verdict":       "pass",
	}
	for key, value := range extraArgs {
		args[key] = value
	}

	_, err = handleHaftCommission(ctx, store, args)
	if err != nil {
		t.Fatal(err)
	}
}

func createCommissionDecisionFixture(
	t *testing.T,
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	title string,
	affectedFile string,
) *artifact.Artifact {
	t.Helper()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:      title + " problem",
		Signal:     "Harness needs batch commission intake for " + affectedFile + ".",
		Acceptance: "A runnable WorkCommission exists for " + affectedFile + ".",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := artifact.Decide(ctx, store, haftDir, artifact.DecideInput{
		ProblemRef:      problem.Meta.ID,
		SelectedTitle:   title,
		WhySelected:     "The batch harness needs one bounded WorkCommission per DecisionRecord.",
		SelectionPolicy: "Prefer queueable commissions with independent locksets.",
		CounterArgument: "A single large commission would be simpler.",
		WeakestLink:     "Overlapping scopes must stay controlled by locksets.",
		WhyNotOthers: []artifact.RejectionReason{{
			Variant: "One large commission",
			Reason:  "It hides per-decision authorization boundaries.",
		}},
		Rollback:      &artifact.RollbackSpec{Triggers: []string{"Batch commission creation regresses."}},
		EvidenceReqs:  []string{"go test ./internal/cli"},
		AffectedFiles: []string{affectedFile},
		SectionRefs:   []string{"TS.commission.fixture"},
		ValidUntil:    "2099-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}

	return decision
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

func commissionsByID(commissions []map[string]any) map[string]map[string]any {
	byID := make(map[string]map[string]any, len(commissions))

	for _, commission := range commissions {
		byID[stringField(commission, "id")] = commission
	}

	return byID
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

func autonomyEnvelopeSnapshotFixture(validUntil string) map[string]any {
	return map[string]any{
		"ref":                            "ae-test-001",
		"revision":                       "ae-r1",
		"state":                          "active",
		"allowed_repos":                  []any{"local:haft", "github:m0n0x41d/haft"},
		"allowed_paths":                  []any{"internal/cli/**"},
		"forbidden_paths":                []any{"internal/cli/migrations/**"},
		"allowed_actions":                []any{"edit_files", "run_tests"},
		"allowed_modules":                []any{"internal/cli"},
		"forbidden_actions":              []any{"delete_data"},
		"forbidden_one_way_door_actions": []any{"tag_release", "merge_pr"},
		"max_concurrency":                float64(2),
		"commission_budget":              float64(8),
		"on_failure":                     "block_node",
		"on_stale":                       "block_node",
		"valid_until":                    validUntil,
		"required_gates":                 []any{"freshness", "scope", "evidence"},
	}
}

func workCommissionFixture(id, state, validUntil string) map[string]any {
	return map[string]any{
		"id":                           id,
		"decision_ref":                 "dec-20260422-001",
		"decision_revision_hash":       "decision-r1",
		"problem_card_ref":             "pc-20260422-001",
		"implementation_plan_ref":      "plan-20260422-001",
		"implementation_plan_revision": "plan-r1",
		"spec_readiness_override": map[string]any{
			"kind":              "tactical",
			"out_of_spec":       true,
			"project_readiness": "needs_onboard",
			"reason":            "unit test fixture without spec-linked authority",
			"recorded_at":       "2026-04-22T10:00:00Z",
		},
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
