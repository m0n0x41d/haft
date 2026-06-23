package artifact

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClassifyCommand(t *testing.T) {
	cases := []struct {
		command string
		class   string
		ok      bool
	}{
		{"go test ./internal/codebase/ -run TestX", "go test", true},
		{"go build ./...", "go build", true},
		{"go vet ./internal/artifact/", "go vet", true},
		{"go test ../other", "", false},
		{"go test /tmp/pkg", "", false},
		{"go test -coverprofile=/tmp/out ./...", "", false},
		{"go test ./internal/../outside", "", false},
		{"grep -rn proxy_for internal/", "grep", true},
		{"rg -c TODO internal/cli", "rg", true},
		{"grep -rn token /tmp/outside", "", false},
		{"grep -rn token ../outside", "", false},
		{"grep -rn token internal/../outside", "", false},
		{"rg --glob=../outside token internal/", "", false},
		{"go run main.go", "", false},
		{"rm -rf /", "", false},
		{"go test ./... ; rm -rf /", "", false},
		{"go test $(cat /etc/passwd)", "", false},
		{"go test ./... | tee out.log", "", false},
		{"bash -c 'go test'", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		class, ok := ClassifyCommand(c.command)
		if ok != c.ok || class != c.class {
			t.Errorf("ClassifyCommand(%q) = (%q, %v), want (%q, %v)", c.command, class, ok, c.class, c.ok)
		}
	}
}

func TestBuildMaintenancePlan_RungClassification(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	ripe := time.Now().Add(-48 * time.Hour).Format("2006-01-02")
	dec, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Maintenance plan fixture decision",
		WhySelected:   "For testing",
		ValidUntil:    time.Now().Add(60 * 24 * time.Hour).Format(time.RFC3339),
		Predictions: []PredictionInput{
			{
				Claim:       "tests stay green",
				Observable:  "go test on the artifact package",
				Threshold:   "exit 0",
				VerifyAfter: ripe,
				Command:     "go test ./internal/artifact/",
			},
			{
				Claim:       "operators report less noise",
				Observable:  "operator interviews",
				Threshold:   "majority positive",
				VerifyAfter: ripe,
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := BuildMaintenancePlan(ctx, store, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	var machine, judgment *MaintenanceTask
	for i := range plan.Tasks {
		task := plan.Tasks[i]
		if task.DecisionRef != dec.Meta.ID {
			continue
		}
		switch task.Rung {
		case RungMachine:
			machine = &plan.Tasks[i]
		case RungJudgment:
			judgment = &plan.Tasks[i]
		}
	}

	if machine == nil {
		t.Fatal("expected a rung-2 machine-checkable task for the command claim")
	}
	if machine.Command != "go test ./internal/artifact/" || machine.CommandClass != "go test" {
		t.Errorf("machine task command = %q class = %q", machine.Command, machine.CommandClass)
	}
	if machine.DecisionTitle == "" {
		t.Error("presentation floor violated: task carries no decision title")
	}
	if judgment == nil {
		t.Fatal("expected a rung-3 judgment task for the prose-only claim")
	}
	if plan.MachineCheckable < 1 || plan.JudgmentNeeded < 1 {
		t.Errorf("plan counters: machine=%d judgment=%d", plan.MachineCheckable, plan.JudgmentNeeded)
	}
}

func TestBuildMaintenancePlan_BlocksAutoBaselineWhenDecisionHealthExpired(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()

	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string {
	return "run"
}
`)

	dec := createTestDecision(t, store, "dec-maint-health-block", "Health blocks drift auto-baseline")
	dec.Meta.ValidUntil = time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	if err := store.Update(ctx, dec); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAffectedFiles(ctx, dec.Meta.ID, []AffectedFile{{Path: "app.go"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Baseline(ctx, store, projectRoot, BaselineInput{DecisionRef: dec.Meta.ID}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, projectRoot, "app.go", `package main

func Run() string {
	return "run"
}

func Extra() string {
	return "extra"
}
`)

	plan, err := BuildMaintenancePlan(ctx, store, projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	var driftTask *MaintenanceTask
	for i := range plan.Tasks {
		task := plan.Tasks[i]
		if task.DecisionRef == dec.Meta.ID && task.Source == "drift" {
			driftTask = &plan.Tasks[i]
			break
		}
	}
	if driftTask == nil {
		t.Fatal("expected drift task")
	}
	if driftTask.Category != string(SurfaceForReview) {
		t.Fatalf("category = %q, want %q", driftTask.Category, SurfaceForReview)
	}
	if driftTask.Rung != RungJudgment {
		t.Fatalf("rung = %d, want %d", driftTask.Rung, RungJudgment)
	}
	if !strings.Contains(driftTask.Reason, "auto-baseline is blocked") {
		t.Fatalf("reason does not explain health block: %q", driftTask.Reason)
	}
}

func TestBuildMaintenancePlan_CooldownSkipsClaimWithTodayEvidence(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	ripe := time.Now().Add(-48 * time.Hour).Format("2006-01-02")
	dec, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Cooldown fixture decision",
		WhySelected:   "For testing",
		ValidUntil:    time.Now().Add(60 * 24 * time.Hour).Format(time.RFC3339),
		Predictions: []PredictionInput{
			{
				Claim:       "build stays green",
				Observable:  "go build",
				Threshold:   "exit 0",
				VerifyAfter: ripe,
				Command:     "go build ./...",
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	claims := dec.UnmarshalDecisionFields().Claims
	if len(claims) != 1 {
		t.Fatalf("fixture decision claims = %#v, want one claim", claims)
	}
	claimRef := claims[0].ID

	if _, err := AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "go build ./... exit 0 (machine run)",
		Type:            "test",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ClaimRefs:       []string{claimRef},
		Provenance:      ProvenanceMachine,
	}); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildMaintenancePlan(ctx, store, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	for _, task := range plan.Tasks {
		if task.DecisionRef == dec.Meta.ID && task.ClaimID == claimRef {
			t.Error("claim with today's evidence must be cooldown-skipped, not re-planned")
		}
	}
	if plan.CooldownSkipped < 1 {
		t.Errorf("CooldownSkipped = %d, want >=1", plan.CooldownSkipped)
	}
}

func TestBuildMaintenanceJudgmentReview_GroupsJudgmentTasksAndPreservesAuthority(t *testing.T) {
	plan := &MaintenancePlan{
		GeneratedAt: "2026-06-19T00:00:00Z",
		Tasks: []MaintenanceTask{
			{
				DecisionRef:   "dec-machine",
				DecisionTitle: "Machine task",
				Source:        "stale",
				Category:      string(StaleCategoryPendingVerification),
				Rung:          RungMachine,
				Command:       "go test ./...",
			},
			{
				DecisionRef:   "dec-drift-confirm",
				DecisionTitle: "Material drift",
				Source:        "drift",
				Category:      string(StageForConfirm),
				Rung:          RungJudgment,
				Reason:        "a governed symbol body was modified/removed or a file was deleted",
			},
			{
				DecisionRef:   "dec-drift-review",
				DecisionTitle: "Unproven drift",
				Source:        "drift",
				Category:      string(SurfaceForReview),
				Rung:          RungJudgment,
				Reason:        "benignity could not be proven",
			},
			{
				DecisionRef:   "dec-claim",
				DecisionTitle: "Expired claim",
				Source:        "stale",
				Category:      string(StaleCategoryEvidenceExpired),
				Rung:          RungJudgment,
				ClaimID:       "claim-001",
				Observable:    "go test ./internal/artifact",
				Threshold:     "exit 0",
				Reason:        "claim evidence expired",
			},
		},
	}

	review := BuildMaintenanceJudgmentReview(plan)

	if review.TotalTasks != 4 || review.JudgmentTasks != 3 || review.OmittedNonJudgment != 1 {
		t.Fatalf("counts = total:%d judgment:%d omitted:%d", review.TotalTasks, review.JudgmentTasks, review.OmittedNonJudgment)
	}
	if review.SourcePlanGeneratedAt != plan.GeneratedAt {
		t.Fatalf("source plan time = %q, want %q", review.SourcePlanGeneratedAt, plan.GeneratedAt)
	}
	if review.AuthorityBoundary.Mutation != "not_mutation" || review.AuthorityBoundary.Approval != "not_approval" || review.AuthorityBoundary.Evidence != "not_evidence" {
		t.Fatalf("authority boundary = %#v", review.AuthorityBoundary)
	}
	for _, want := range []string{
		JudgmentRecommendationReviewMaterialDrift,
		JudgmentRecommendationReviewUnprovenDrift,
		JudgmentRecommendationRefreshEvidence,
	} {
		if review.Counts.ByRecommendation[want] != 1 {
			t.Fatalf("recommendation %q count = %d, want 1", want, review.Counts.ByRecommendation[want])
		}
	}

	var materialDrift *MaintenanceJudgmentTaskReview
	var claimRefresh *MaintenanceJudgmentTaskReview
	for i := range review.Groups {
		for j := range review.Groups[i].Tasks {
			task := &review.Groups[i].Tasks[j]
			if task.DecisionRef == "dec-drift-confirm" {
				materialDrift = task
			}
			if task.DecisionRef == "dec-claim" {
				claimRefresh = task
			}
		}
	}
	if materialDrift == nil || materialDrift.Confidence != JudgmentConfidenceHigh {
		t.Fatalf("material drift task = %#v", materialDrift)
	}
	if !maintenanceReviewTestContains(materialDrift.SuggestedCommands, `haft_decision(action="baseline"`) {
		t.Fatalf("material drift suggested commands missing baseline candidate: %#v", materialDrift.SuggestedCommands)
	}
	if claimRefresh == nil || !maintenanceReviewTestContains(claimRefresh.SuggestedCommands, `claim_refs=["claim-001"]`) {
		t.Fatalf("claim refresh commands = %#v", claimRefresh)
	}
	if !maintenanceReviewTestContains(claimRefresh.DrillDown, `haft_query(action="related", artifact_ref="dec-claim")`) {
		t.Fatalf("claim refresh drilldown = %#v", claimRefresh.DrillDown)
	}
}

func TestCompactMaintenanceJudgmentReviewBoundsTasksAndProposals(t *testing.T) {
	plan := &MaintenancePlan{
		GeneratedAt: "2026-06-19T12:00:00Z",
		Tasks: []MaintenanceTask{
			{
				DecisionRef:   "dec-a",
				DecisionTitle: "A",
				Source:        "drift",
				Category:      string(SurfaceForReview),
				Rung:          RungJudgment,
				Reason:        "needs review",
			},
			{
				DecisionRef:   "dec-b",
				DecisionTitle: "B",
				Source:        "drift",
				Category:      string(SurfaceForReview),
				Rung:          RungJudgment,
				Reason:        "needs review",
			},
			{
				DecisionRef:   "dec-c",
				DecisionTitle: "C",
				Source:        "stale",
				Category:      string(StaleCategoryEvidenceExpired),
				Rung:          RungJudgment,
				ClaimID:       "claim-001",
				Observable:    "go test ./...",
				Threshold:     "exit 0",
				Reason:        "expired",
			},
		},
	}
	review := BuildMaintenanceJudgmentReview(plan)
	review.Reconciliation = BuildMaintenanceReconciliationReview([]MaintenanceReconciliationReviewProposal{
		{ID: "proposal-a", Kind: "fallback_scope_repair_review", Reason: "a", SuggestedCommand: "haft decision reconcile --json"},
		{ID: "proposal-b", Kind: "fallback_scope_repair_review", Reason: "b", SuggestedCommand: "haft decision reconcile --json"},
	})

	compact := CompactMaintenanceJudgmentReview(review, 1)

	if compact == review {
		t.Fatal("compact projection should not alias the source review")
	}
	if compact.View != "compact" {
		t.Fatalf("view = %q, want compact", compact.View)
	}
	if compact.FullAuditCommand != "haft overseer judgment --json" {
		t.Fatalf("full audit command = %q", compact.FullAuditCommand)
	}
	if compact.JudgmentTasks != 3 || compact.OmittedJudgmentTasks != 2 {
		t.Fatalf("judgment counts = total:%d omitted:%d", compact.JudgmentTasks, compact.OmittedJudgmentTasks)
	}
	if maintenanceReviewTaskCount(compact) != 1 {
		t.Fatalf("compact task count = %d, want 1", maintenanceReviewTaskCount(compact))
	}
	if maintenanceReviewTaskCount(review) != 3 {
		t.Fatalf("source review was mutated: %#v", review.Groups)
	}
	if compact.Reconciliation == nil || compact.Reconciliation.ProposalCount != 2 {
		t.Fatalf("compact reconciliation = %#v", compact.Reconciliation)
	}
	if compact.Reconciliation.OmittedProposals != 1 || len(compact.Reconciliation.Proposals) != 1 {
		t.Fatalf("compact proposals = %#v", compact.Reconciliation)
	}
	if len(review.Reconciliation.Proposals) != 2 {
		t.Fatalf("source reconciliation was mutated: %#v", review.Reconciliation)
	}
}

func TestBuildMaintenanceReconciliationReviewNormalizesReadOnlyProposals(t *testing.T) {
	review := BuildMaintenanceReconciliationReview([]MaintenanceReconciliationReviewProposal{
		{
			ID:               " proposal-b ",
			Kind:             "fallback_scope_repair_review",
			Reason:           " fallback targets need enrichment ",
			DecisionRefs:     []string{"dec-b", "", "dec-a", "dec-a"},
			SuggestedCommand: " haft decision reconcile --json ",
		},
		{
			ID:                "proposal-a",
			Kind:              "high_fanout_reconciliation_review",
			Reason:            "fanout exceeds threshold",
			SuggestedCommand:  "haft decision reconcile --json",
			AuthorityBoundary: "read_only_reconciliation_proposal_not_binding_authority",
		},
		{
			Kind:   "",
			Reason: "missing kind",
		},
	})

	if review == nil {
		t.Fatal("expected reconciliation review")
	}
	if review.AuthorityBoundary != "read_only_reconciliation_proposal_not_binding_authority" {
		t.Fatalf("authority = %q", review.AuthorityBoundary)
	}
	if review.ProposalCount != 2 {
		t.Fatalf("proposal count = %d, want 2", review.ProposalCount)
	}
	if review.ByKind["fallback_scope_repair_review"] != 1 || review.ByKind["high_fanout_reconciliation_review"] != 1 {
		t.Fatalf("by kind = %#v", review.ByKind)
	}
	if len(review.SuggestedCommands) != 1 || review.SuggestedCommands[0] != "haft decision reconcile --json" {
		t.Fatalf("suggested commands = %#v", review.SuggestedCommands)
	}
	if review.Proposals[0].ID != "proposal-b" || review.Proposals[0].AuthorityBoundary != "read_only_reconciliation_proposal_not_binding_authority" {
		t.Fatalf("first proposal = %#v", review.Proposals[0])
	}
	if !maintenanceReviewTestContains(review.Proposals[0].DecisionRefs, "dec-a") || !maintenanceReviewTestContains(review.Proposals[0].DecisionRefs, "dec-b") {
		t.Fatalf("decision refs = %#v", review.Proposals[0].DecisionRefs)
	}
}

func maintenanceReviewTestContains(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func maintenanceReviewTaskCount(review *MaintenanceJudgmentReview) int {
	count := 0
	for _, group := range review.Groups {
		count += len(group.Tasks)
	}
	return count
}
