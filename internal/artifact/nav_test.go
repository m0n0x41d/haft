package artifact

import (
	"context"
	"strings"
	"testing"
)

// --- State tests: one per DerivedStatus ---

func TestComputeNavState_Underframed(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedUnderframed {
		t.Errorf("status = %q, want UNDERFRAMED", state.DerivedStatus)
	}
	if !strings.Contains(state.NextAction, "/h-frame") {
		t.Errorf("NextAction = %q, want /h-frame", state.NextAction)
	}
}

func TestComputeNavState_FramedTactical(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedFramed {
		t.Errorf("status = %q, want FRAMED", state.DerivedStatus)
	}
	// All modes should offer explore (all phases mandatory)
	if !strings.Contains(state.NextAction, "/h-explore") && !strings.Contains(state.NextAction, "/h-char") {
		t.Errorf("NextAction should contain /h-explore or /h-char, got %q", state.NextAction)
	}
}

func TestComputeNavState_FramedStandard_NoChar(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	_, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedFramed {
		t.Errorf("status = %q, want FRAMED", state.DerivedStatus)
	}
	if state.Mode != ModeStandard {
		t.Errorf("mode = %q, want standard", state.Mode)
	}
	if !strings.Contains(state.NextAction, "/h-char") {
		t.Errorf("NextAction should contain /h-char for standard without characterization, got %q", state.NextAction)
	}
	if !strings.Contains(state.NextAction, "/h-explore") {
		t.Errorf("NextAction should contain /h-explore as alternative, got %q", state.NextAction)
	}
}

func TestComputeNavState_FramedStandard_WithChar(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add characterization
	_, _, err = CharacterizeProblem(ctx, store, haftDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{
			{Name: "latency", ScaleType: "ratio", Unit: "ms", Polarity: "lower_better"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedFramed {
		t.Errorf("status = %q, want FRAMED", state.DerivedStatus)
	}
	if !strings.Contains(state.NextAction, "/h-explore") {
		t.Errorf("NextAction should contain /h-explore, got %q", state.NextAction)
	}
	if strings.Contains(state.NextAction, "/h-char") {
		t.Errorf("NextAction should NOT contain /h-char after characterization, got %q", state.NextAction)
	}
}

func TestComputeNavState_ExploringTactical(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("Option A", "complexity", "Optimize for implementation simplicity"),
			testVariant("Option B", "performance", "Optimize for throughput headroom"),
		},
		NoSteppingStoneRationale: "Both options are direct implementation candidates.",
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedExploring {
		t.Errorf("status = %q, want EXPLORING", state.DerivedStatus)
	}
	// After exploring, should offer compare or decide
	if !strings.Contains(state.NextAction, "/h-compare") && !strings.Contains(state.NextAction, "/h-decide") {
		t.Errorf("NextAction should contain /h-compare or /h-decide, got %q", state.NextAction)
	}
}

func TestComputeNavState_ExploringStandard(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("Option A", "complexity", "Optimize for implementation simplicity"),
			testVariant("Option B", "performance", "Optimize for throughput headroom"),
		},
		NoSteppingStoneRationale: "Both options are direct implementation candidates.",
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedExploring {
		t.Errorf("status = %q, want EXPLORING", state.DerivedStatus)
	}
	if !strings.Contains(state.NextAction, "/h-compare") {
		t.Errorf("NextAction should contain /h-compare for standard EXPLORING, got %q", state.NextAction)
	}
	if strings.Contains(state.NextAction, "/h-decide") {
		t.Errorf("NextAction should NOT contain /h-decide for standard EXPLORING, got %q", state.NextAction)
	}
}

func TestComputeNavState_Compared(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	sol, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("Option A", "complexity", "Optimize for implementation simplicity"),
			testVariant("Option B", "performance", "Optimize for throughput headroom"),
		},
		NoSteppingStoneRationale: "Both options are direct implementation candidates.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: sol.Meta.ID,
		Results: ComparisonResult{
			Dimensions:      []string{"speed", "cost"},
			Scores:          map[string]map[string]string{"Option A": {"speed": "fast", "cost": "high"}, "Option B": {"speed": "slow", "cost": "low"}},
			NonDominatedSet: []string{"Option A", "Option B"},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Option A", Summary: "Higher speed, but higher cost."},
				{Variant: "Option B", Summary: "Lower cost, but lower speed."},
			},
			PolicyApplied: "optimize speed",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedCompared {
		t.Errorf("status = %q, want COMPARED", state.DerivedStatus)
	}
	if !strings.Contains(state.NextAction, "/h-decide") {
		t.Errorf("NextAction should contain /h-decide, got %q", state.NextAction)
	}
	if !strings.Contains(state.NextAction, "human's chosen variant") {
		t.Errorf("NextAction should make the human decision boundary explicit, got %q", state.NextAction)
	}
}

func TestComputeNavState_Decided(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}

	sol, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("Option A", "complexity", "Optimize for implementation simplicity"),
			testVariant("Option B", "performance", "Optimize for throughput headroom"),
		},
		NoSteppingStoneRationale: "Both options are direct implementation candidates.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Decide(ctx, store, haftDir, completeDecision(DecideInput{
		PortfolioRef:  sol.Meta.ID,
		SelectedTitle: "Option A",
		WhySelected:   "faster",
		Context:       "test-ctx",
	}))
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedDecided {
		t.Errorf("status = %q, want DECIDED", state.DerivedStatus)
	}
	if state.NextAction != "" {
		t.Errorf("DECIDED should have no NextAction, got %q", state.NextAction)
	}
}

// FormatNavStrip tests moved to internal/present/format_test.go

// --- Contract tests: invariants that must hold across ALL states ---

// buildNavStates creates NavStates for every reachable DerivedStatus.
// Returns a map[DerivedStatus]NavState for contract assertions.
func buildNavStates(t *testing.T) map[DerivedStatus]NavState {
	t.Helper()
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	states := make(map[DerivedStatus]NavState)

	// UNDERFRAMED
	states[DerivedUnderframed] = ComputeNavState(ctx, store, "c-under")

	// FRAMED tactical
	_, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Tactical", Signal: "sig", Context: "c-tac", Mode: "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}
	states[DerivedFramed] = ComputeNavState(ctx, store, "c-tac")

	// FRAMED standard (separate context)
	_, _, err = FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Standard", Signal: "sig", Context: "c-std", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	// EXPLORING
	prob, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Exploring", Signal: "sig", Context: "c-expl", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("A", "w", "Optimize for minimal moving parts"),
			testVariant("B", "w", "Optimize for future scaling margin"),
		},
		NoSteppingStoneRationale: "Both options are evaluated as production-ready endpoints.",
	})
	if err != nil {
		t.Fatal(err)
	}
	states[DerivedExploring] = ComputeNavState(ctx, store, "c-expl")

	// COMPARED
	prob2, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Compared", Signal: "sig", Context: "c-comp", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	sol, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob2.Meta.ID,
		Variants: []Variant{
			testVariant("X", "w", "Keep the integration surface small"),
			testVariant("Y", "w", "Bias toward operational elasticity"),
		},
		NoSteppingStoneRationale: "The compared options are both end-state candidates.",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: sol.Meta.ID,
		Results: ComparisonResult{
			Dimensions:      []string{"speed"},
			NonDominatedSet: []string{"X"},
			Scores:          map[string]map[string]string{"X": {"speed": "fast"}, "Y": {"speed": "slow"}},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "Y",
					DominatedBy: []string{"X"},
					Summary:     "Worse on the only compared dimension.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "X", Summary: "Best value on the compared dimension."},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	states[DerivedCompared] = ComputeNavState(ctx, store, "c-comp")

	// DECIDED
	prob3, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Decided", Signal: "sig", Context: "c-dec", Mode: "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}
	sol2, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob3.Meta.ID,
		Variants: []Variant{
			testVariant("P", "w", "Prioritize delivery speed"),
			testVariant("Q", "w", "Prioritize runtime efficiency"),
		},
		NoSteppingStoneRationale: "Both tactical choices are direct endpoints.",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = Decide(ctx, store, haftDir, completeDecision(DecideInput{
		PortfolioRef: sol2.Meta.ID, SelectedTitle: "P", WhySelected: "reason", Context: "c-dec",
	}))
	if err != nil {
		t.Fatal(err)
	}
	states[DerivedDecided] = ComputeNavState(ctx, store, "c-dec")

	return states
}

// Contract: NextAction never contains raw tool-call syntax (haft_*, or
// the legacy quint_* prefix). All user-facing actions must use slash
// commands (/h-*).
func TestContract_NoToolCallSyntax(t *testing.T) {
	for status, state := range buildNavStates(t) {
		if state.NextAction == "" {
			continue
		}
		for _, prefix := range []string{"haft_", "quint_"} {
			if strings.Contains(state.NextAction, prefix) {
				t.Errorf("[%s] NextAction uses tool-call syntax %q: %q", status, prefix, state.NextAction)
			}
		}
		if !strings.Contains(state.NextAction, "/h-") {
			t.Errorf("[%s] NextAction should use slash commands (/h-*): %q", status, state.NextAction)
		}
	}
}

// Contract: NextAction is set iff state warrants available actions.
func TestContract_NextActionConsistency(t *testing.T) {
	for status, state := range buildNavStates(t) {
		hasAction := state.NextAction != ""
		if hasAction && !strings.Contains(state.NextAction, "/h-") {
			t.Errorf("[%s] NextAction should use slash commands (/h-*): %q", status, state.NextAction)
		}
	}
}

// Contract: DECIDED is terminal — no available actions.
func TestContract_DecidedIsTerminal(t *testing.T) {
	states := buildNavStates(t)
	decided := states[DerivedDecided]

	if decided.NextAction != "" {
		t.Errorf("DECIDED should be terminal, got NextAction = %q", decided.NextAction)
	}
}
