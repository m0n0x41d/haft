package agent

import "testing"

func TestCanonicalizeCycleForPersistence_TrimsRefsWithoutDerivingPhase(t *testing.T) {
	cycle := &Cycle{
		Status:               CycleActive,
		ProblemRef:           " prob-1 ",
		PortfolioRef:         " port-current ",
		ComparedPortfolioRef: " port-compared ",
		SelectedPortfolioRef: " port-compared ",
		SelectedVariantRef:   " V2 ",
		DecisionRef:          " dec-1 ",
		Phase:                PhaseFramer,
	}

	canonical := CanonicalizeCycleForPersistence(cycle)
	if canonical == nil {
		t.Fatal("expected canonical cycle")
	}
	if canonical.ProblemRef != "prob-1" || canonical.PortfolioRef != "port-current" {
		t.Fatalf("refs were not trimmed: problem=%q portfolio=%q", canonical.ProblemRef, canonical.PortfolioRef)
	}
	if canonical.ComparedPortfolioRef != "port-compared" || canonical.DecisionRef != "dec-1" {
		t.Fatalf("refs were not trimmed: compared=%q decision=%q", canonical.ComparedPortfolioRef, canonical.DecisionRef)
	}
	if canonical.SelectedPortfolioRef != "port-compared" || canonical.SelectedVariantRef != "V2" {
		t.Fatalf("consistent selection was not preserved: (%q, %q)", canonical.SelectedPortfolioRef, canonical.SelectedVariantRef)
	}
	if canonical.Phase != PhaseFramer {
		t.Fatalf("Phase = %s, want explicitly stored %s", canonical.Phase, PhaseFramer)
	}
	if cycle.ProblemRef != " prob-1 " {
		t.Fatalf("input cycle mutated: ProblemRef = %q", cycle.ProblemRef)
	}
}

func TestCanonicalizeCycleForPersistence_ClearsOnlyInconsistentSelection(t *testing.T) {
	cycle := &Cycle{
		Status:               CycleComplete,
		ProblemRef:           "prob-1",
		PortfolioRef:         "port-current",
		ComparedPortfolioRef: "port-compared",
		SelectedPortfolioRef: "port-stale",
		SelectedVariantRef:   "V2",
		DecisionRef:          "dec-1",
		Phase:                PhaseMeasure,
	}

	canonical := CanonicalizeCycleForPersistence(cycle)
	if canonical.SelectedPortfolioRef != "" || canonical.SelectedVariantRef != "" {
		t.Fatalf("selection = (%q, %q), want cleared", canonical.SelectedPortfolioRef, canonical.SelectedVariantRef)
	}
	if canonical.ProblemRef != "prob-1" || canonical.PortfolioRef != "port-current" {
		t.Fatalf("independent refs were cleared: problem=%q portfolio=%q", canonical.ProblemRef, canonical.PortfolioRef)
	}
	if canonical.ComparedPortfolioRef != "port-compared" || canonical.DecisionRef != "dec-1" {
		t.Fatalf("independent refs were cleared: compared=%q decision=%q", canonical.ComparedPortfolioRef, canonical.DecisionRef)
	}
	if canonical.Phase != PhaseMeasure {
		t.Fatalf("Phase = %s, want %s", canonical.Phase, PhaseMeasure)
	}
}
