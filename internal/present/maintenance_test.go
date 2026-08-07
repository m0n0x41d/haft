package present

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestCompactMaintenancePlanResponseSamplesJudgmentTail(t *testing.T) {
	plan := maintenancePlanTestPlan()

	text := CompactMaintenancePlanResponse(plan, "")

	for _, want := range []string{
		"Compact view",
		"Machine check",
		"Judgment one",
		"Judgment two",
		"Judgment three",
		"... 2 more judgment task(s)",
		`haft_refresh(action="review")`,
		`haft_refresh(action="plan", verbose=true)`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("compact plan missing %q:\n%s", want, text)
		}
	}
	for _, absent := range []string{
		"Judgment four",
		"Judgment five",
	} {
		if strings.Contains(text, absent) {
			t.Fatalf("compact plan should omit %q:\n%s", absent, text)
		}
	}
}

func TestMaintenancePlanResponseKeepsFullJudgmentTail(t *testing.T) {
	plan := maintenancePlanTestPlan()

	text := MaintenancePlanResponse(plan, "")

	for _, want := range []string{
		"Machine check",
		"Judgment one",
		"Judgment two",
		"Judgment three",
		"Judgment four",
		"Judgment five",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("full plan missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Compact view") {
		t.Fatalf("full plan should not be labeled compact:\n%s", text)
	}
	if strings.Contains(text, "more judgment task") {
		t.Fatalf("full plan should not omit judgment tasks:\n%s", text)
	}
}

func maintenancePlanTestPlan() *artifact.MaintenancePlan {
	return &artifact.MaintenancePlan{
		AutoBaselineCandidates: 1,
		MachineCheckable:       1,
		JudgmentNeeded:         5,
		Tasks: []artifact.MaintenanceTask{
			{
				DecisionRef:   "dec-auto",
				DecisionTitle: "Auto baseline",
				Rung:          artifact.RungDeterministic,
				Reason:        "every drift is additive",
			},
			{
				DecisionRef:   "dec-machine",
				DecisionTitle: "Machine check",
				Rung:          artifact.RungMachine,
				Reason:        "claim ready for verification",
				Command:       "go test ./...",
				Threshold:     "exit 0",
			},
			maintenancePlanJudgmentTask("dec-j1", "Judgment one"),
			maintenancePlanJudgmentTask("dec-j2", "Judgment two"),
			maintenancePlanJudgmentTask("dec-j3", "Judgment three"),
			maintenancePlanJudgmentTask("dec-j4", "Judgment four"),
			maintenancePlanJudgmentTask("dec-j5", "Judgment five"),
		},
	}
}

func maintenancePlanJudgmentTask(ref string, title string) artifact.MaintenanceTask {
	return artifact.MaintenanceTask{
		DecisionRef:   ref,
		DecisionTitle: title,
		Rung:          artifact.RungJudgment,
		Reason:        "needs operator judgment",
		Observable:    "inspect exact diff",
	}
}
