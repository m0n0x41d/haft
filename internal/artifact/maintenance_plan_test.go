package artifact

import (
	"context"
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

	ripe := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
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

	ripe := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
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

	if _, err := AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef:     dec.Meta.ID,
		Content:         "go build ./... exit 0 (machine run)",
		Type:            "test",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ClaimRefs:       []string{"claim-001"},
		Provenance:      ProvenanceMachine,
	}); err != nil {
		t.Fatal(err)
	}

	plan, err := BuildMaintenancePlan(ctx, store, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	for _, task := range plan.Tasks {
		if task.DecisionRef == dec.Meta.ID && task.ClaimID == "claim-001" {
			t.Error("claim with today's evidence must be cooldown-skipped, not re-planned")
		}
	}
	if plan.CooldownSkipped < 1 {
		t.Errorf("CooldownSkipped = %d, want >=1", plan.CooldownSkipped)
	}
}
