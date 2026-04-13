package agent

import "testing"

func TestBindArtifact_CompareBindsComparedPortfolioRef(t *testing.T) {
	cycle := &Cycle{
		Status:               CycleActive,
		ProblemRef:           "prob-1",
		PortfolioRef:         "port-1",
		SelectedPortfolioRef: "port-1",
		SelectedVariantRef:   "V1",
	}

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
	if updated.SelectedPortfolioRef != "" || updated.SelectedVariantRef != "" {
		t.Fatalf("selection = (%q, %q), want cleared", updated.SelectedPortfolioRef, updated.SelectedVariantRef)
	}
	if updated.Phase != PhaseExplorer {
		t.Fatalf("Phase = %s, want %s", updated.Phase, PhaseExplorer)
	}
}

func TestBindArtifact_FrameClearsDownstreamRefs(t *testing.T) {
	cycle := &Cycle{
		Status:               CycleActive,
		ProblemRef:           "prob-old",
		PortfolioRef:         "port-old",
		ComparedPortfolioRef: "port-old",
		SelectedPortfolioRef: "port-old",
		SelectedVariantRef:   "V2",
		DecisionRef:          "dec-old",
	}

	updated := BindArtifact(cycle, ArtifactMeta{
		Kind:        "problem",
		Operation:   "frame",
		ArtifactRef: "prob-new",
	})
	if updated == nil {
		t.Fatal("expected updated cycle")
	}
	if updated.ProblemRef != "prob-new" {
		t.Fatalf("ProblemRef = %q, want prob-new", updated.ProblemRef)
	}
	if updated.PortfolioRef != "" || updated.ComparedPortfolioRef != "" {
		t.Fatalf("portfolio refs = (%q, %q), want cleared", updated.PortfolioRef, updated.ComparedPortfolioRef)
	}
	if updated.SelectedPortfolioRef != "" || updated.SelectedVariantRef != "" {
		t.Fatalf("selection = (%q, %q), want cleared", updated.SelectedPortfolioRef, updated.SelectedVariantRef)
	}
	if updated.DecisionRef != "" {
		t.Fatalf("DecisionRef = %q, want empty", updated.DecisionRef)
	}
	if updated.Phase != PhaseExplorer {
		t.Fatalf("Phase = %s, want %s", updated.Phase, PhaseExplorer)
	}
}

func TestBindArtifact_ExploreClearsComparedPortfolioRef(t *testing.T) {
	cycle := &Cycle{
		Status:               CycleActive,
		ProblemRef:           "prob-1",
		PortfolioRef:         "port-old",
		ComparedPortfolioRef: "port-old",
		SelectedPortfolioRef: "port-old",
		SelectedVariantRef:   "V2",
	}

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
	if updated.SelectedPortfolioRef != "" || updated.SelectedVariantRef != "" {
		t.Fatalf("selection = (%q, %q), want cleared", updated.SelectedPortfolioRef, updated.SelectedVariantRef)
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
		SelectedPortfolioRef: "port-old",
		SelectedVariantRef:   "V1",
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
	if updated.SelectedPortfolioRef != "" || updated.SelectedVariantRef != "" {
		t.Fatalf("selection = (%q, %q), want cleared", updated.SelectedPortfolioRef, updated.SelectedVariantRef)
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
		SelectedPortfolioRef: "port-old",
		SelectedVariantRef:   "V2",
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
	if updated.SelectedPortfolioRef != "" || updated.SelectedVariantRef != "" {
		t.Fatalf("selection = (%q, %q), want cleared", updated.SelectedPortfolioRef, updated.SelectedVariantRef)
	}
	if updated.DecisionRef != "" {
		t.Fatalf("DecisionRef = %q, want empty", updated.DecisionRef)
	}
	if updated.Phase != PhaseExplorer {
		t.Fatalf("Phase = %s, want %s", updated.Phase, PhaseExplorer)
	}
}

func TestCanonicalizeCycleForPersistence_DerivesPhaseAndClearsStaleSelection(t *testing.T) {
	cycle := &Cycle{
		Status:               CycleActive,
		ProblemRef:           "prob-1",
		PortfolioRef:         "port-2",
		ComparedPortfolioRef: "port-1",
		SelectedPortfolioRef: "port-1",
		SelectedVariantRef:   "V2",
		Phase:                PhaseFramer,
	}

	canonical := CanonicalizeCycleForPersistence(cycle)
	if canonical == nil {
		t.Fatal("expected canonical cycle")
	}
	if canonical.Phase != PhaseExplorer {
		t.Fatalf("Phase = %s, want %s", canonical.Phase, PhaseExplorer)
	}
	if canonical.SelectedPortfolioRef != "" || canonical.SelectedVariantRef != "" {
		t.Fatalf("selection = (%q, %q), want cleared", canonical.SelectedPortfolioRef, canonical.SelectedVariantRef)
	}
}
