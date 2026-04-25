package cli

import (
	"context"
	"encoding/json"
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

	_, err := handleHaftCommission(ctx, store, map[string]any{
		"action":     "create",
		"commission": workCommissionFixture("wc-blocked-001", "queued", "2099-01-01T00:00:00Z"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "claim_for_preflight",
		"commission_id": "wc-blocked-001",
		"runner_id":     "open-sleigh:test",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "start_after_preflight",
		"commission_id": "wc-blocked-001",
		"runner_id":     "open-sleigh:test",
		"event":         "preflight_passed",
		"verdict":       "pass",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleHaftCommission(ctx, store, map[string]any{
		"action":        "complete_or_block",
		"commission_id": "wc-blocked-001",
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
	if commission["delivery_policy"] != defaultDeliveryPolicy {
		t.Fatalf("delivery_policy = %#v, want %s", commission["delivery_policy"], defaultDeliveryPolicy)
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

	_, err = handleHaftCommission(ctx, store, map[string]any{
		"action":        "complete_or_block",
		"commission_id": firstID,
		"runner_id":     "open-sleigh:test",
		"verdict":       "completed",
	})
	if err != nil {
		t.Fatal(err)
	}

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
