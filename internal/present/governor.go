package present

import (
	"fmt"
	"strings"
)

// GovernorData is the compact, prompt-budgeted slice of project state used by
// host-side prompt governors (e.g. the @haft/pi before_agent_start hook).
// Counts are always present; each list is capped so the rendered block stays
// inside a system-prompt budget instead of clipping the full dashboard
// mid-table on the client side.
type GovernorData struct {
	OverseerLine               string
	ReconciliationLine         string
	PendingCount               int
	UnassessedCount            int
	StaleCount                 int
	DriftEventCount            int
	DriftImpactedDecisionCount int
	DriftMaxFanout             int
	TopAttention               []string
	ActiveProblems             []string
	OpenMethodRuns             []string
}

const governorListCap = 3

// StatusGovernor renders GovernorData as a compact markdown block: counts
// first, at most three lines per list, no navigation strip.
func StatusGovernor(d GovernorData) string {
	var sb strings.Builder
	sb.WriteString("## Haft Project State (governor)\n\n")

	if d.OverseerLine != "" {
		sb.WriteString(fmt.Sprintf("Overseer: %s\n", d.OverseerLine))
	}
	if d.ReconciliationLine != "" {
		sb.WriteString(fmt.Sprintf("Reconciliation: %s\n", d.ReconciliationLine))
	}
	sb.WriteString(fmt.Sprintf(
		"Decisions: %d pending, %d unassessed; %d refresh-due\n",
		d.PendingCount, d.UnassessedCount, d.StaleCount,
	))
	if d.DriftEventCount > 0 {
		sb.WriteString(fmt.Sprintf(
			"Drift: %d unique event(s), %d impacted decision(s), max fanout %d\n",
			d.DriftEventCount,
			d.DriftImpactedDecisionCount,
			d.DriftMaxFanout,
		))
	}

	writeGovernorList(&sb, "Attention", d.TopAttention)
	writeGovernorList(&sb, "Active problems", d.ActiveProblems)
	writeGovernorList(&sb, "Open method runs", d.OpenMethodRuns)

	if d.StaleCount > 0 || d.DriftEventCount > 0 {
		sb.WriteString("\nStale or drifted decisions above are evidence debt: verify with haft_query/haft_refresh before relying on them.\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

func writeGovernorList(sb *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}

	fmt.Fprintf(sb, "\n%s:\n", title)
	shown := items
	if len(shown) > governorListCap {
		shown = shown[:governorListCap]
	}
	for _, item := range shown {
		fmt.Fprintf(sb, "- %s\n", item)
	}
	if rest := len(items) - len(shown); rest > 0 {
		fmt.Fprintf(sb, "- ... and %d more\n", rest)
	}
}
