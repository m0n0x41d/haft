package present

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// MaintenancePlanResponse renders the kernel-compiled work order. Every line
// carries the decision title, what to do, and how it may be acted on — the
// presentation floor: no bare IDs without titles and next actions.
func MaintenancePlanResponse(plan *artifact.MaintenancePlan, navStrip string) string {
	if plan == nil || len(plan.Tasks) == 0 {
		return "Maintenance plan: nothing actionable — no ripe claims, no drift dispositions.\n" + navStrip
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Maintenance Plan (%d task(s): %d auto-baseline, %d machine-checkable, %d need judgment)\n\n",
		len(plan.Tasks), plan.AutoBaselineCandidates, plan.MachineCheckable, plan.JudgmentNeeded)

	sections := []struct {
		rung  int
		title string
		hint  string
	}{
		{artifact.RungDeterministic, "Rung 1 — auto-baseline candidates (deterministic, additive-only drift)", "actionable by the overseer execute-phase or `haft_decision(action=\"baseline\")`"},
		{artifact.RungMachine, "Rung 2 — machine-checkable claims (allowlisted command per claim)", "run the command, attach evidence via `haft_decision(action=\"evidence\", claim_refs=[...])`"},
		{artifact.RungJudgment, "Rung 3 — needs judgment (never auto-executed)", "verify via /h-verify or triage via waive/supersede/deprecate"},
	}
	for _, section := range sections {
		tasks := tasksForRung(plan.Tasks, section.rung)
		if len(tasks) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "### %s\n", section.title)
		for _, t := range tasks {
			sb.WriteString(formatMaintenanceTask(t))
		}
		fmt.Fprintf(&sb, "→ %s\n\n", section.hint)
	}

	if plan.CooldownSkipped > 0 {
		fmt.Fprintf(&sb, "_%d claim(s) skipped: evidence already attached today (flake cooldown)._\n", plan.CooldownSkipped)
	}
	return sb.String() + navStrip
}

func tasksForRung(tasks []artifact.MaintenanceTask, rung int) []artifact.MaintenanceTask {
	out := make([]artifact.MaintenanceTask, 0, len(tasks))
	for _, t := range tasks {
		if t.Rung == rung {
			out = append(out, t)
		}
	}
	return out
}

func formatMaintenanceTask(t artifact.MaintenanceTask) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "- **%s** `%s`", t.DecisionTitle, t.DecisionRef)
	if t.ClaimID != "" {
		fmt.Fprintf(&sb, " · %s", t.ClaimID)
	}
	fmt.Fprintf(&sb, " — %s", t.Reason)
	if t.Command != "" {
		fmt.Fprintf(&sb, "\n  run: `%s` · threshold: %s", t.Command, t.Threshold)
	} else if t.Observable != "" {
		fmt.Fprintf(&sb, "\n  observable: %s", truncate(t.Observable, 140))
	}
	sb.WriteString("\n")
	return sb.String()
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}
