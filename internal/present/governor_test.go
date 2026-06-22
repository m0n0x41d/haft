package present

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestStatusGovernorCountsAlwaysPresent(t *testing.T) {
	out := StatusGovernor(GovernorData{
		ReconciliationLine: ReconciliationCueSummary(artifact.ReconciliationCueReport{
			Summary: artifact.ReconciliationCueSummary{
				HighFanoutEvents: 1,
				MaxFanout:        7,
			},
			Cues: []artifact.ReconciliationCue{{
				Kind: artifact.ReconciliationCueHighFanout,
			}},
			Commands: []string{`haft_query(action="drift_events")`},
		}),
		PendingCount:               4,
		UnassessedCount:            3,
		StaleCount:                 11,
		DriftEventCount:            28,
		DriftImpactedDecisionCount: 31,
		DriftMaxFanout:             7,
	})

	if !strings.Contains(out, "Decisions: 4 pending, 3 unassessed; 11 refresh-due") {
		t.Fatalf("counts line missing:\n%s", out)
	}
	if !strings.Contains(out, "Drift: 28 unique event(s), 31 impacted decision(s), max fanout 7") {
		t.Fatalf("drift event line missing:\n%s", out)
	}
	if !strings.Contains(out, `Reconciliation: 1 high-fanout drift event(s), max fanout 7; drill down with haft_query(action="drift_events")`) {
		t.Fatalf("reconciliation line missing:\n%s", out)
	}
	if !strings.Contains(out, "evidence debt") {
		t.Fatalf("evidence-debt reminder missing despite stale/drift counts:\n%s", out)
	}
}

func TestStatusGovernorOmitsEmptySections(t *testing.T) {
	out := StatusGovernor(GovernorData{})

	for _, absent := range []string{"Overseer:", "Reconciliation:", "Attention:", "Active problems:", "Open method runs:", "evidence debt"} {
		if strings.Contains(out, absent) {
			t.Fatalf("expected %q to be omitted for empty data:\n%s", absent, out)
		}
	}
}

func TestStatusGovernorCapsLists(t *testing.T) {
	attention := []string{"a", "b", "c", "d", "e"}
	out := StatusGovernor(GovernorData{StaleCount: 5, TopAttention: attention})

	if !strings.Contains(out, "- ... and 2 more") {
		t.Fatalf("expected capped list with remainder:\n%s", out)
	}
	if strings.Contains(out, "- d") {
		t.Fatalf("expected items beyond cap to be hidden:\n%s", out)
	}
}

func TestStatusGovernorStaysInsidePromptBudget(t *testing.T) {
	long := strings.Repeat("x", 200)
	data := GovernorData{
		OverseerLine:               "41 signal(s), 7 high, 35 suppressed",
		PendingCount:               99,
		UnassessedCount:            99,
		StaleCount:                 99,
		DriftEventCount:            99,
		DriftImpactedDecisionCount: 99,
		DriftMaxFanout:             99,
		TopAttention:               []string{long, long, long, long, long, long},
		ActiveProblems:             []string{long, long, long, long},
		OpenMethodRuns:             []string{long, long, long, long},
	}

	if got := len(StatusGovernor(data)); got > 2600 {
		t.Fatalf("governor block exceeds prompt budget: %d chars", got)
	}
}
