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
	if !strings.Contains(state.NextAction, "/q-frame") {
		t.Errorf("NextAction = %q, want /q-frame", state.NextAction)
	}
}

func TestComputeNavState_FramedTactical(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	_, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedFramed {
		t.Errorf("status = %q, want FRAMED", state.DerivedStatus)
	}
	if state.Mode != ModeTactical {
		t.Errorf("mode = %q, want tactical", state.Mode)
	}
	if !strings.Contains(state.NextAction, "/q-explore") {
		t.Errorf("NextAction should contain /q-explore, got %q", state.NextAction)
	}
	if !strings.Contains(state.NextAction, "/q-decide") {
		t.Errorf("NextAction should contain /q-decide for tactical, got %q", state.NextAction)
	}
	if strings.Contains(state.NextAction, "/q-char") {
		t.Errorf("NextAction should NOT contain /q-char for tactical, got %q", state.NextAction)
	}
}

func TestComputeNavState_FramedStandard_NoChar(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	_, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
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
	if !strings.Contains(state.NextAction, "/q-char") {
		t.Errorf("NextAction should contain /q-char for standard without characterization, got %q", state.NextAction)
	}
	if !strings.Contains(state.NextAction, "/q-explore") {
		t.Errorf("NextAction should contain /q-explore as alternative, got %q", state.NextAction)
	}
}

func TestComputeNavState_FramedStandard_WithChar(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Add characterization
	_, _, err = CharacterizeProblem(ctx, store, quintDir, CharacterizeInput{
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
	if !strings.Contains(state.NextAction, "/q-explore") {
		t.Errorf("NextAction should contain /q-explore, got %q", state.NextAction)
	}
	if strings.Contains(state.NextAction, "/q-char") {
		t.Errorf("NextAction should NOT contain /q-char after characterization, got %q", state.NextAction)
	}
}

func TestComputeNavState_ExploringTactical(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{Title: "Option A", WeakestLink: "complexity"},
			{Title: "Option B", WeakestLink: "performance"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedExploring {
		t.Errorf("status = %q, want EXPLORING", state.DerivedStatus)
	}
	if !strings.Contains(state.NextAction, "/q-decide") {
		t.Errorf("NextAction should contain /q-decide for tactical EXPLORING, got %q", state.NextAction)
	}
	if !strings.Contains(state.NextAction, "/q-compare") {
		t.Errorf("NextAction should contain /q-compare as upgrade option, got %q", state.NextAction)
	}
}

func TestComputeNavState_ExploringStandard(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{Title: "Option A", WeakestLink: "complexity"},
			{Title: "Option B", WeakestLink: "performance"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedExploring {
		t.Errorf("status = %q, want EXPLORING", state.DerivedStatus)
	}
	if !strings.Contains(state.NextAction, "/q-compare") {
		t.Errorf("NextAction should contain /q-compare for standard EXPLORING, got %q", state.NextAction)
	}
	if strings.Contains(state.NextAction, "/q-decide") {
		t.Errorf("NextAction should NOT contain /q-decide for standard EXPLORING, got %q", state.NextAction)
	}
}

func TestComputeNavState_Compared(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	sol, _, err := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{Title: "Option A", WeakestLink: "complexity"},
			{Title: "Option B", WeakestLink: "performance"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = CompareSolutions(ctx, store, quintDir, CompareInput{
		PortfolioRef: sol.Meta.ID,
		Results: ComparisonResult{
			Dimensions:      []string{"speed", "cost"},
			Scores:          map[string]map[string]string{"Option A": {"speed": "fast", "cost": "high"}, "Option B": {"speed": "slow", "cost": "low"}},
			NonDominatedSet: []string{"Option A", "Option B"},
			PolicyApplied:   "optimize speed",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	state := ComputeNavState(ctx, store, "test-ctx")

	if state.DerivedStatus != DerivedCompared {
		t.Errorf("status = %q, want COMPARED", state.DerivedStatus)
	}
	if !strings.Contains(state.NextAction, "/q-decide") {
		t.Errorf("NextAction should contain /q-decide, got %q", state.NextAction)
	}
}

func TestComputeNavState_Decided(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Test problem", Signal: "something broke", Context: "test-ctx", Mode: "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}

	sol, _, err := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{Title: "Option A", WeakestLink: "complexity"},
			{Title: "Option B", WeakestLink: "performance"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = Decide(ctx, store, quintDir, DecideInput{
		PortfolioRef:  sol.Meta.ID,
		SelectedTitle: "Option A",
		WhySelected:   "faster",
		Context:       "test-ctx",
	})
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

// --- FormatNavStrip tests ---

func TestFormatNavStrip_AvailableGuardLine(t *testing.T) {
	state := NavState{
		DerivedStatus: DerivedFramed,
		Mode:          ModeTactical,
		NextAction:    `/q-explore (generate variants) | /q-decide (decide directly)`,
	}

	output := FormatNavStrip(state)

	if !strings.Contains(output, "Available:") {
		t.Errorf("should contain 'Available:', got:\n%s", output)
	}
	if strings.Contains(output, "Next:") {
		t.Errorf("should NOT contain 'Next:', got:\n%s", output)
	}
	if !strings.Contains(output, "do not auto-execute") {
		t.Errorf("should contain guard line, got:\n%s", output)
	}
}

func TestFormatNavStrip_NoGuardWhenDecided(t *testing.T) {
	state := NavState{
		DerivedStatus: DerivedDecided,
		DecisionInfo:  "Use Redis",
	}

	output := FormatNavStrip(state)

	if strings.Contains(output, "Available:") {
		t.Errorf("DECIDED state should NOT show Available, got:\n%s", output)
	}
	if strings.Contains(output, "do not auto-execute") {
		t.Errorf("DECIDED state should NOT show guard line, got:\n%s", output)
	}
}

func TestFormatNavStrip_AllFieldsRendered(t *testing.T) {
	state := NavState{
		Context:       "payments",
		Mode:          ModeStandard,
		DerivedStatus: DerivedExploring,
		PortfolioInfo: "API redesign",
		StaleCount:    2,
		NextAction:    "/q-compare (compare variants)",
	}

	output := FormatNavStrip(state)

	for _, want := range []string{"Context: payments", "Mode: standard", "Status: EXPLORING", "Portfolio: API redesign", "Stale: 2", "Available:", "/q-compare"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}

// --- Contract tests: invariants that must hold across ALL states ---

// buildNavStates creates NavStates for every reachable DerivedStatus.
// Returns a map[DerivedStatus]NavState for contract assertions.
func buildNavStates(t *testing.T) map[DerivedStatus]NavState {
	t.Helper()
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	states := make(map[DerivedStatus]NavState)

	// UNDERFRAMED
	states[DerivedUnderframed] = ComputeNavState(ctx, store, "c-under")

	// FRAMED tactical
	_, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Tactical", Signal: "sig", Context: "c-tac", Mode: "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}
	states[DerivedFramed] = ComputeNavState(ctx, store, "c-tac")

	// FRAMED standard (separate context)
	_, _, err = FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Standard", Signal: "sig", Context: "c-std", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}

	// EXPLORING
	prob, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Exploring", Signal: "sig", Context: "c-expl", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{Title: "A", WeakestLink: "w"}, {Title: "B", WeakestLink: "w"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	states[DerivedExploring] = ComputeNavState(ctx, store, "c-expl")

	// COMPARED
	prob2, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Compared", Signal: "sig", Context: "c-comp", Mode: "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	sol, _, err := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob2.Meta.ID,
		Variants: []Variant{
			{Title: "X", WeakestLink: "w"}, {Title: "Y", WeakestLink: "w"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = CompareSolutions(ctx, store, quintDir, CompareInput{
		PortfolioRef: sol.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"d1"}, NonDominatedSet: []string{"X"},
			Scores: map[string]map[string]string{"X": {"d1": "good"}, "Y": {"d1": "ok"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	states[DerivedCompared] = ComputeNavState(ctx, store, "c-comp")

	// DECIDED
	prob3, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Decided", Signal: "sig", Context: "c-dec", Mode: "tactical",
	})
	if err != nil {
		t.Fatal(err)
	}
	sol2, _, err := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob3.Meta.ID,
		Variants: []Variant{
			{Title: "P", WeakestLink: "w"}, {Title: "Q", WeakestLink: "w"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = Decide(ctx, store, quintDir, DecideInput{
		PortfolioRef: sol2.Meta.ID, SelectedTitle: "P", WhySelected: "reason", Context: "c-dec",
	})
	if err != nil {
		t.Fatal(err)
	}
	states[DerivedDecided] = ComputeNavState(ctx, store, "c-dec")

	return states
}

// Contract: NextAction never contains tool call syntax (quint_*).
// All actions must use slash commands (/q-*).
func TestContract_NoToolCallSyntax(t *testing.T) {
	for status, state := range buildNavStates(t) {
		if state.NextAction == "" {
			continue
		}
		if strings.Contains(state.NextAction, "quint_") {
			t.Errorf("[%s] NextAction uses tool call syntax: %q", status, state.NextAction)
		}
		if !strings.Contains(state.NextAction, "/q-") {
			t.Errorf("[%s] NextAction should use slash commands (/q-*): %q", status, state.NextAction)
		}
	}
}

// Contract: guard line present iff NextAction present.
func TestContract_GuardLineIffNextAction(t *testing.T) {
	for status, state := range buildNavStates(t) {
		output := FormatNavStrip(state)
		hasAvailable := strings.Contains(output, "Available:")
		hasGuard := strings.Contains(output, "do not auto-execute")
		hasAction := state.NextAction != ""

		if hasAction && !hasAvailable {
			t.Errorf("[%s] has NextAction but no Available line", status)
		}
		if hasAction && !hasGuard {
			t.Errorf("[%s] has NextAction but no guard line", status)
		}
		if !hasAction && hasAvailable {
			t.Errorf("[%s] no NextAction but Available line shown", status)
		}
		if !hasAction && hasGuard {
			t.Errorf("[%s] no NextAction but guard line shown", status)
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
	output := FormatNavStrip(decided)
	if strings.Contains(output, "Available:") {
		t.Errorf("DECIDED should show no Available actions:\n%s", output)
	}
}

// Contract: "Next:" must never appear in output (replaced by "Available:").
func TestContract_NoLegacyNextLabel(t *testing.T) {
	for status, state := range buildNavStates(t) {
		output := FormatNavStrip(state)
		if strings.Contains(output, "Next:") {
			t.Errorf("[%s] output contains legacy 'Next:' label:\n%s", status, output)
		}
	}
}
