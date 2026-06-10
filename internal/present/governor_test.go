package present

import (
	"strings"
	"testing"
)

func TestStatusGovernorCountsAlwaysPresent(t *testing.T) {
	out := StatusGovernor(GovernorData{PendingCount: 4, UnassessedCount: 3, StaleCount: 11, DriftCount: 28})

	if !strings.Contains(out, "Decisions: 4 pending, 3 unassessed; 11 refresh-due, 28 drifted") {
		t.Fatalf("counts line missing:\n%s", out)
	}
	if !strings.Contains(out, "evidence debt") {
		t.Fatalf("evidence-debt reminder missing despite stale/drift counts:\n%s", out)
	}
}

func TestStatusGovernorOmitsEmptySections(t *testing.T) {
	out := StatusGovernor(GovernorData{})

	for _, absent := range []string{"Overseer:", "Attention:", "Active problems:", "Open method runs:", "evidence debt"} {
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
		OverseerLine:    "41 signal(s), 7 high, 35 suppressed",
		PendingCount:    99,
		UnassessedCount: 99,
		StaleCount:      99,
		DriftCount:      99,
		TopAttention:    []string{long, long, long, long, long, long},
		ActiveProblems:  []string{long, long, long, long},
		OpenMethodRuns:  []string{long, long, long, long},
	}

	if got := len(StatusGovernor(data)); got > 2600 {
		t.Fatalf("governor block exceeds prompt budget: %d chars", got)
	}
}
