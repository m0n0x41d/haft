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

func maintenanceReviewTestContains(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}
