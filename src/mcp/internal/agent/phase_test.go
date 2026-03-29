package agent

import "testing"

func TestDeriveNextPhase(t *testing.T) {
	// Standard depth: full 5-phase cycle
	standardTests := []struct {
		current Phase
		signal  TransitionSignal
		want    Phase
	}{
		{PhaseReady, SignalProblemFramed, PhaseExplorer},
		{PhaseFramer, SignalProblemFramed, PhaseExplorer},
		{PhaseExplorer, SignalVariantsExplored, PhaseDecider},
		{PhaseDecider, SignalDecisionMade, PhaseWorker},
		{PhaseWorker, SignalImplemented, PhaseMeasure},
		{PhaseMeasure, SignalMeasured, PhaseReady},
		{PhaseMeasure, SignalMeasureFailed, PhaseFramer},
		{PhaseMeasure, SignalTestsFailed, PhaseWorker},
	}

	for _, tt := range standardTests {
		got := DeriveNextPhase(tt.current, tt.signal, DepthStandard)
		if got != tt.want {
			t.Errorf("standard: DeriveNextPhase(%q, %q) = %q, want %q", tt.current, tt.signal, got, tt.want)
		}
	}

	// Tactical depth: frame → DECIDE → worker → measure (skip explorer)
	tacticalTests := []struct {
		current Phase
		signal  TransitionSignal
		want    Phase
	}{
		{PhaseReady, SignalProblemFramed, PhaseDecider},
		{PhaseFramer, SignalProblemFramed, PhaseDecider},
		{PhaseFramer, SignalLLMDone, PhaseReady}, // framer answered without framing → done
		{PhaseDecider, SignalDecisionMade, PhaseWorker},
		{PhaseWorker, SignalImplemented, PhaseMeasure},
		{PhaseMeasure, SignalMeasured, PhaseReady},
		{PhaseMeasure, SignalMeasureFailed, PhaseFramer},
		{PhaseMeasure, SignalTestsFailed, PhaseWorker},
	}

	for _, tt := range tacticalTests {
		got := DeriveNextPhase(tt.current, tt.signal, DepthTactical)
		if got != tt.want {
			t.Errorf("tactical: DeriveNextPhase(%q, %q) = %q, want %q", tt.current, tt.signal, got, tt.want)
		}
	}

	// Framer + LLMDone → PhaseReady (answered question or greeting, cycle done)
	if got := DeriveNextPhase(PhaseFramer, SignalLLMDone, DepthStandard); got != PhaseReady {
		t.Errorf("standard: framer+LLMDone should go to ready, got %q", got)
	}
	if got := DeriveNextPhase(PhaseFramer, SignalLLMDone, DepthTactical); got != PhaseReady {
		t.Errorf("tactical: framer+LLMDone should go to ready, got %q", got)
	}

	// Stay in current phase on wrong signals
	stayTests := []struct {
		current Phase
		signal  TransitionSignal
		want    Phase
	}{
		{PhaseFramer, SignalImplemented, PhaseFramer},
		{PhaseWorker, SignalProblemFramed, PhaseWorker},
		{PhaseExplorer, SignalImplemented, PhaseExplorer},
	}

	for _, tt := range stayTests {
		got := DeriveNextPhase(tt.current, tt.signal, DepthStandard)
		if got != tt.want {
			t.Errorf("stay: DeriveNextPhase(%q, %q) = %q, want %q", tt.current, tt.signal, got, tt.want)
		}
	}
}

func TestValidateProposal(t *testing.T) {
	// Signal-driven transitions are always trusted.
	// The signal confirms the tool was called — NavState is too coarse for per-cycle validation.
	tests := []struct {
		name     string
		proposed Phase
		status   NavStatus
		signal   TransitionSignal
	}{
		{"explorer from problem_framed", PhaseExplorer, NavFramed, SignalProblemFramed},
		{"explorer even with stale navstate", PhaseExplorer, NavDecided, SignalProblemFramed},
		{"decider from variants_explored", PhaseDecider, NavExploring, SignalVariantsExplored},
		{"decider from tactical (problem_framed)", PhaseDecider, NavFramed, SignalProblemFramed},
		{"worker from decision_made", PhaseWorker, NavDecided, SignalDecisionMade},
		{"worker from tests_failed", PhaseWorker, NavDecided, SignalTestsFailed},
		{"measure from implemented", PhaseMeasure, NavDecided, SignalImplemented},
		{"framer from measure_failed", PhaseFramer, NavDecided, SignalMeasureFailed},
		{"ready from measured", PhaseReady, NavDecided, SignalMeasured},
	}

	for _, tt := range tests {
		got := ValidateProposal(tt.proposed, tt.status, tt.signal)
		if !got {
			t.Errorf("%s: ValidateProposal(%q, %q, %q) = false, want true",
				tt.name, tt.proposed, tt.status, tt.signal)
		}
	}
}

func TestDeriveFromNavState(t *testing.T) {
	tests := []struct {
		name    string
		current Phase
		status  NavStatus
		depth   Depth
		want    Phase
	}{
		{"framer+FRAMED→explorer (standard)", PhaseFramer, NavFramed, DepthStandard, PhaseExplorer},
		{"framer+FRAMED→decider (tactical)", PhaseFramer, NavFramed, DepthTactical, PhaseDecider},
		{"framer+UNDERFRAMED→stay", PhaseFramer, NavUnderframed, DepthStandard, PhaseFramer},
		{"explorer+EXPLORING→decider", PhaseExplorer, NavExploring, DepthStandard, PhaseDecider},
		{"explorer+COMPARED→decider", PhaseExplorer, NavCompared, DepthStandard, PhaseDecider},
		{"decider+DECIDED→worker", PhaseDecider, NavDecided, DepthStandard, PhaseWorker},
		{"decider+EXPLORING→stay", PhaseDecider, NavExploring, DepthStandard, PhaseDecider},
		{"worker stays (no artifact)", PhaseWorker, NavDecided, DepthStandard, PhaseWorker},
		{"measure stays (signal-driven)", PhaseMeasure, NavDecided, DepthStandard, PhaseMeasure},
	}

	for _, tt := range tests {
		got := DeriveFromNavState(tt.current, tt.status, tt.depth)
		if got != tt.want {
			t.Errorf("%s: DeriveFromNavState(%q, %q, %q) = %q, want %q",
				tt.name, tt.current, tt.status, tt.depth, got, tt.want)
		}
	}
}

func TestFilterToolsForPhase(t *testing.T) {
	allTools := []ToolSchema{
		{Name: "bash"},
		{Name: "read"},
		{Name: "write"},
		{Name: "edit"},
		{Name: "glob"},
		{Name: "grep"},
		{Name: "quint_problem"},
		{Name: "quint_decision"},
		{Name: "quint_query"},
	}

	// Framer: only read/search + quint_problem
	framer := PhaseDef{
		Phase:        PhaseFramer,
		AllowedTools: []string{"read", "glob", "grep", "bash", "quint_problem", "quint_query"},
	}
	filtered := FilterToolsForPhase(allTools, framer)
	if len(filtered) != 6 {
		t.Errorf("framer should have 6 tools, got %d", len(filtered))
	}
	for _, tool := range filtered {
		if tool.Name == "write" || tool.Name == "edit" {
			t.Errorf("framer should not have %s", tool.Name)
		}
	}

	// Worker: bash/read/write/edit/glob/grep (no quint tools)
	worker := PhaseDef{
		Phase:        PhaseWorker,
		AllowedTools: []string{"bash", "read", "write", "edit", "glob", "grep"},
	}
	filtered = FilterToolsForPhase(allTools, worker)
	if len(filtered) != 6 {
		t.Errorf("worker should have 6 tools, got %d", len(filtered))
	}
	for _, tool := range filtered {
		if tool.Name == "quint_problem" || tool.Name == "quint_decision" {
			t.Errorf("worker should not have %s", tool.Name)
		}
	}
}

func TestIsToolAllowed(t *testing.T) {
	framer := PhaseDef{
		AllowedTools: []string{"read", "grep", "quint_problem"},
	}

	if !IsToolAllowed("read", framer) {
		t.Error("read should be allowed for framer")
	}
	if IsToolAllowed("write", framer) {
		t.Error("write should NOT be allowed for framer")
	}

	// Empty allowlist = everything allowed
	open := PhaseDef{AllowedTools: nil}
	if !IsToolAllowed("anything", open) {
		t.Error("empty allowlist should allow everything")
	}
}

func TestBuildTransitionInstruction(t *testing.T) {
	// Test all 5 phases have instructions
	phases := []Phase{PhaseFramer, PhaseExplorer, PhaseDecider, PhaseWorker, PhaseMeasure}
	for _, phase := range phases {
		msg := BuildTransitionInstruction(PhaseReady, phase, "")
		if msg == "" {
			t.Errorf("transition instruction for %q should not be empty", phase)
		}
	}

	// Test with summary
	msg := BuildTransitionInstruction(PhaseFramer, PhaseWorker, "Identified root cause in auth.go:42")
	if !contains(msg, "framer → worker") {
		t.Error("should contain phase names")
	}
	if !contains(msg, "auth.go:42") {
		t.Error("should contain summary")
	}
	if !contains(msg, "haft-worker") {
		t.Error("should contain role name")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
