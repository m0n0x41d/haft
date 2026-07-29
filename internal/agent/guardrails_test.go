package agent

import (
	"strings"
	"testing"
)

func TestCompatibilityGuardrailsDoNotImposePredecessors(t *testing.T) {
	checks := map[string]func(*Cycle) error{
		"explore":  CanExplore,
		"compare":  CanCompare,
		"baseline": CanBaseline,
		"measure":  CanMeasure,
	}

	for name, check := range checks {
		t.Run(name+" without cycle", func(t *testing.T) {
			if err := check(nil); err != nil {
				t.Fatalf("compatibility guardrail imposed a predecessor: %v", err)
			}
		})
		t.Run(name+" with legacy terminal state", func(t *testing.T) {
			cycle := &Cycle{Status: CycleComplete, DecisionRef: "dec-legacy"}
			if err := check(cycle); err != nil {
				t.Fatalf("compatibility guardrail imposed a phase constraint: %v", err)
			}
		})
	}
}

func TestCanDecide_AllowsNoComparedPortfolioContext(t *testing.T) {
	cycle := &Cycle{
		Status:      CycleComplete,
		DecisionRef: "dec-legacy",
	}

	if err := CanDecide(cycle, false); err != nil {
		t.Fatalf("CanDecide imposed a predecessor without compared-portfolio context: %v", err)
	}
}

func TestCanDecide_RequiresHumanSelectionForComparedPortfolioContext(t *testing.T) {
	cycle := &Cycle{ComparedPortfolioRef: "port-1"}

	err := CanDecide(cycle, false)
	if err == nil {
		t.Fatal("expected human-selection authority guardrail")
	}
	if !strings.Contains(err.Error(), "user selection") {
		t.Fatalf("error = %q, want human-selection guidance", err.Error())
	}
}

func TestCanDecide_AllowsHumanSelectionForComparedPortfolioContext(t *testing.T) {
	cycle := &Cycle{
		PortfolioRef:         "port-current",
		ComparedPortfolioRef: "port-legacy",
		DecisionRef:          "dec-legacy",
	}

	if err := CanDecide(cycle, true); err != nil {
		t.Fatalf("CanDecide: %v", err)
	}
}

func TestHasDecisionSelection_RequiresMatchingComparedPortfolio(t *testing.T) {
	cycle := &Cycle{
		ComparedPortfolioRef: "port-1",
		SelectedPortfolioRef: "port-old",
		SelectedVariantRef:   "V2",
	}

	if HasDecisionSelection(cycle) {
		t.Fatal("expected stale selection to be rejected")
	}
}

func TestHasDecisionSelection_AllowsActiveSelection(t *testing.T) {
	cycle := &Cycle{
		ComparedPortfolioRef: "port-1",
		SelectedPortfolioRef: "port-1",
		SelectedVariantRef:   "V2",
	}

	if !HasDecisionSelection(cycle) {
		t.Fatal("expected active selection to satisfy the boundary")
	}
}

func TestCheckREff_WarnsOnUnsubstantiatedClosure(t *testing.T) {
	err := CheckREff(0.82, 0)
	if err == nil {
		t.Fatal("expected F0 closure warning")
	}
	if !strings.Contains(err.Error(), "F_eff=F0") {
		t.Fatalf("warning = %q, want F_eff=F0 guidance", err.Error())
	}
}
