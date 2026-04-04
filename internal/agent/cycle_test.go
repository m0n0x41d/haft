package agent

import "testing"

func TestBindArtifact_CompareBindsComparedPortfolioRef(t *testing.T) {
	cycle := &Cycle{Status: CycleActive, ProblemRef: "prob-1", PortfolioRef: "port-1"}

	updated := BindArtifact(cycle, ArtifactMeta{
		Kind:                 "solution",
		Operation:            "compare",
		ComparedPortfolioRef: "port-2",
	})
	if updated == nil {
		t.Fatal("expected updated cycle")
	}
	if updated.ComparedPortfolioRef != "port-2" {
		t.Fatalf("ComparedPortfolioRef = %q, want port-2", updated.ComparedPortfolioRef)
	}
	if updated.Phase != PhaseExplorer {
		t.Fatalf("Phase = %s, want %s", updated.Phase, PhaseExplorer)
	}
}

func TestBindArtifact_ExploreClearsComparedPortfolioRef(t *testing.T) {
	cycle := &Cycle{Status: CycleActive, ProblemRef: "prob-1", PortfolioRef: "port-old", ComparedPortfolioRef: "port-old"}

	updated := BindArtifact(cycle, ArtifactMeta{Kind: "solution", Operation: "explore", ArtifactRef: "port-new"})
	if updated == nil {
		t.Fatal("expected updated cycle")
	}
	if updated.PortfolioRef != "port-new" {
		t.Fatalf("PortfolioRef = %q, want port-new", updated.PortfolioRef)
	}
	if updated.ComparedPortfolioRef != "" {
		t.Fatalf("ComparedPortfolioRef = %q, want empty", updated.ComparedPortfolioRef)
	}
	if updated.Phase != PhaseExplorer {
		t.Fatalf("Phase = %s, want %s", updated.Phase, PhaseExplorer)
	}
}

func TestDerivePhaseFromCycle_StaysExplorerUntilCompareCompletes(t *testing.T) {
	cycle := &Cycle{Status: CycleActive, ProblemRef: "prob-1", PortfolioRef: "port-1"}
	if got := DerivePhaseFromCycle(cycle); got != PhaseExplorer {
		t.Fatalf("Phase = %s, want %s", got, PhaseExplorer)
	}

	cycle.ComparedPortfolioRef = "port-1"
	if got := DerivePhaseFromCycle(cycle); got != PhaseDecider {
		t.Fatalf("Phase = %s, want %s", got, PhaseDecider)
	}
}

func TestBindArtifact_AdoptClearsComparedPortfolioWhenAdoptedPortfolioWasNotCompared(t *testing.T) {
	cycle := &Cycle{
		Status:               CycleActive,
		ProblemRef:           "prob-old",
		PortfolioRef:         "port-old",
		ComparedPortfolioRef: "port-old",
	}

	updated := BindArtifact(cycle, ArtifactMeta{
		Kind:              "problem",
		Operation:         "adopt",
		ArtifactRef:       "prob-new",
		AdoptPortfolioRef: "port-new",
	})
	if updated == nil {
		t.Fatal("expected updated cycle")
	}
	if updated.PortfolioRef != "port-new" {
		t.Fatalf("PortfolioRef = %q, want port-new", updated.PortfolioRef)
	}
	if updated.ComparedPortfolioRef != "" {
		t.Fatalf("ComparedPortfolioRef = %q, want empty", updated.ComparedPortfolioRef)
	}
	if updated.Phase != PhaseExplorer {
		t.Fatalf("Phase = %s, want %s", updated.Phase, PhaseExplorer)
	}
}

func TestBindArtifact_AdoptClearsStaleRefsWhenNewProblemHasNoRelatedArtifacts(t *testing.T) {
	cycle := &Cycle{
		Status:               CycleActive,
		ProblemRef:           "prob-old",
		PortfolioRef:         "port-old",
		ComparedPortfolioRef: "port-old",
		DecisionRef:          "dec-old",
	}

	updated := BindArtifact(cycle, ArtifactMeta{
		Kind:        "problem",
		Operation:   "adopt",
		ArtifactRef: "prob-new",
	})
	if updated == nil {
		t.Fatal("expected updated cycle")
	}
	if updated.ProblemRef != "prob-new" {
		t.Fatalf("ProblemRef = %q, want prob-new", updated.ProblemRef)
	}
	if updated.PortfolioRef != "" {
		t.Fatalf("PortfolioRef = %q, want empty", updated.PortfolioRef)
	}
	if updated.ComparedPortfolioRef != "" {
		t.Fatalf("ComparedPortfolioRef = %q, want empty", updated.ComparedPortfolioRef)
	}
	if updated.DecisionRef != "" {
		t.Fatalf("DecisionRef = %q, want empty", updated.DecisionRef)
	}
	if updated.Phase != PhaseExplorer {
		t.Fatalf("Phase = %s, want %s", updated.Phase, PhaseExplorer)
	}
}
