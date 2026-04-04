package agent

import (
	"strings"
	"testing"
)

func TestCheckREff_WarnsOnUnsubstantiatedClosure(t *testing.T) {
	err := CheckREff(0.82, 0)
	if err == nil {
		t.Fatal("expected F0 closure warning")
	}
	if !strings.Contains(err.Error(), "F_eff=F0") {
		t.Fatalf("warning = %q, want F_eff=F0 guidance", err.Error())
	}
}

func TestCanDecide_RequiresCompareForActivePortfolio(t *testing.T) {
	cycle := &Cycle{PortfolioRef: "port-1"}

	err := CanDecide(cycle, true)
	if err == nil {
		t.Fatal("expected compare guardrail")
	}
	if !strings.Contains(err.Error(), "completed comparison for the active portfolio") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestCanDecide_RejectsStaleComparedPortfolio(t *testing.T) {
	cycle := &Cycle{PortfolioRef: "port-2", ComparedPortfolioRef: "port-1"}

	err := CanDecide(cycle, true)
	if err == nil {
		t.Fatal("expected stale compare guardrail")
	}
	if !strings.Contains(err.Error(), "completed comparison for the active portfolio") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestCanCompare_AllowsActivePortfolioBeforeUserSelection(t *testing.T) {
	cycle := &Cycle{PortfolioRef: "port-1"}

	if err := CanCompare(cycle); err != nil {
		t.Fatalf("CanCompare: %v", err)
	}
}

func TestCanDecide_RequiresUserSelectionAfterCompare(t *testing.T) {
	cycle := &Cycle{PortfolioRef: "port-1", ComparedPortfolioRef: "port-1"}

	err := CanDecide(cycle, false)
	if err == nil {
		t.Fatal("expected decision boundary guardrail")
	}
	if !strings.Contains(err.Error(), "compare -> decide boundary") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestCanDecide_AllowsComparedActivePortfolio(t *testing.T) {
	cycle := &Cycle{PortfolioRef: "port-1", ComparedPortfolioRef: "port-1"}

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
