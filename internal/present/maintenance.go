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

// MaintenanceJudgmentReviewResponse renders the rung-3 maintenance packet as a
// read-only review aid. It deliberately names the authority boundary before the
// tasks so the packet cannot be mistaken for evidence, approval, or execution.
func MaintenanceJudgmentReviewResponse(review *artifact.MaintenanceJudgmentReview, navStrip string) string {
	if review == nil || review.JudgmentTasks == 0 {
		return "Maintenance judgment review: nothing needs judgment in the current plan.\n" + navStrip
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Maintenance Judgment Review (%d need judgment; %d non-judgment omitted)\n\n",
		review.JudgmentTasks, review.OmittedNonJudgment)
	fmt.Fprintf(&sb, "Authority: %s, %s, %s; agent role: %s; apply surface: %s.\n\n",
		review.AuthorityBoundary.Mutation,
		review.AuthorityBoundary.Approval,
		review.AuthorityBoundary.Evidence,
		review.AuthorityBoundary.AgentRole,
		review.AuthorityBoundary.ApplySurface)

	for _, group := range review.Groups {
		fmt.Fprintf(&sb, "### %s (%s confidence, %d task(s))\n", group.Recommendation, group.Confidence, group.TaskCount)
		fmt.Fprintf(&sb, "source: %s · category: %s\n", group.Source, group.Category)
		fmt.Fprintf(&sb, "evidence need: %s\n", group.EvidenceNeed)
		fmt.Fprintf(&sb, "suggested action: %s\n", group.SuggestedAction)
		for i, task := range group.Tasks {
			if i >= 3 {
				fmt.Fprintf(&sb, "- ... %d more task(s) in this group; use `haft overseer judgment --json` for the full packet.\n", len(group.Tasks)-i)
				break
			}
			sb.WriteString(formatMaintenanceJudgmentTask(task))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Next: gather evidence or exact diffs first; mutate only after explicit operator approval via the suggested commands.\n")
	return sb.String() + navStrip
}

// MaintenanceDrainResponse renders the explicit h-verify/overseer drain report.
// It names what was machine-safe enough to execute and what remains blocked on
// operator judgment.
func MaintenanceDrainResponse(report MaintenanceDrainRenderable, navStrip string) string {
	if report == nil {
		return "Maintenance drain report unavailable.\n" + navStrip
	}

	fields := report.GetMaintenanceDrainFields()
	var sb strings.Builder
	mode := "executed"
	if fields.DryRun {
		mode = "dry-run"
	}
	fmt.Fprintf(&sb, "## Maintenance Drain (%s)\n\n", mode)
	if fields.MaintenanceRunID != "" {
		fmt.Fprintf(&sb, "Maintenance run: `%s`\n", fields.MaintenanceRunID)
	}
	fmt.Fprintf(&sb, "Authority: %s; approval: %s; evidence: %s; gate: %s.\n\n",
		fields.Mutation,
		fields.Approval,
		fields.Evidence,
		fields.GateDecision)
	fmt.Fprintf(&sb, "Actions: %d total, %d applied, %d evidence-attached, %d proposed, %d failed.\n",
		fields.ExecutedActions,
		fields.AppliedActions,
		fields.EvidenceActions,
		fields.ProposedActions,
		fields.FailedActions)
	fmt.Fprintf(&sb, "Needs operator: %d task(s) after drain.\n\n", fields.NeedsOperatorTasks)

	for _, line := range fields.ExecutedLines {
		sb.WriteString("- ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if len(fields.ExecutedLines) > 0 {
		sb.WriteString("\n")
	}
	for _, line := range fields.NeedsOperatorLines {
		sb.WriteString("- ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	if len(fields.NeedsOperatorLines) > 0 {
		sb.WriteString("\n")
	}
	for _, action := range fields.NextActions {
		fmt.Fprintf(&sb, "Next: %s\n", action)
	}
	return sb.String() + navStrip
}

type MaintenanceDrainRenderable interface {
	GetMaintenanceDrainFields() MaintenanceDrainFields
}

// MaintenanceDrainFields is a renderer adapter so the CLI can keep the JSON
// report type local while presentation stays side-effect free.
type MaintenanceDrainFields struct {
	DryRun             bool
	MaintenanceRunID   string
	Mutation           string
	Approval           string
	Evidence           string
	GateDecision       string
	ExecutedActions    int
	AppliedActions     int
	EvidenceActions    int
	ProposedActions    int
	FailedActions      int
	NeedsOperatorTasks int
	ExecutedLines      []string
	NeedsOperatorLines []string
	NextActions        []string
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

func formatMaintenanceJudgmentTask(t artifact.MaintenanceJudgmentTaskReview) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "- **%s** `%s`", t.DecisionTitle, t.DecisionRef)
	if t.ClaimID != "" {
		fmt.Fprintf(&sb, " · %s", t.ClaimID)
	}
	fmt.Fprintf(&sb, " — %s", truncate(t.Reason, 160))
	if t.Observable != "" || t.Threshold != "" {
		fmt.Fprintf(&sb, "\n  claim: %s", truncate(t.EvidenceNeed, 160))
	}
	if len(t.SuggestedCommands) > 0 {
		fmt.Fprintf(&sb, "\n  first command candidate: `%s`", t.SuggestedCommands[0])
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
