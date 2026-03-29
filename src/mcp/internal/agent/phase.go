package agent

import "strings"

// ---------------------------------------------------------------------------
// Phase transitions — pure functions, no side effects, fully testable.
//
// V3-symmetric transition gate:
// - Signals PROPOSE transitions (fast, no I/O)
// - NavState VALIDATES proposals at phase boundaries (one DB query)
// - NavState GENERATES proposals when signals are silent (fallback)
// ---------------------------------------------------------------------------

// TransitionSignal describes what happened that might trigger a phase transition.
type TransitionSignal string

const (
	SignalProblemFramed    TransitionSignal = "problem_framed"    // quint_problem(frame) was called
	SignalVariantsExplored TransitionSignal = "variants_explored" // quint_solution(explore) was called
	SignalDecisionMade     TransitionSignal = "decision_made"     // quint_decision(decide) was called
	SignalImplemented      TransitionSignal = "implemented"       // code was written (write/edit tool used successfully)
	SignalTestsPassed      TransitionSignal = "tests_passed"      // bash(test) returned exit 0
	SignalTestsFailed      TransitionSignal = "tests_failed"      // bash(test) returned non-zero
	SignalMeasured         TransitionSignal = "measured"          // quint_decision(measure) was called
	SignalMeasureFailed    TransitionSignal = "measure_failed"    // measurement verdict = failed
	SignalLLMDone          TransitionSignal = "llm_done"          // LLM finished without tool calls
)

// DeriveNextPhase computes the PROPOSED next phase from current phase, signal, and depth.
// This is the fast path — fires instantly with no I/O.
// The proposal MUST be validated by ValidateProposal before committing.
//
// Full lemniscate flow:
//
//	                  ┌──────────────────────────────────┐
//	                  │          LEMNISCATE ∞            │
//	                  │                                  │
//	┌─── LEFT CYCLE ──┤         RIGHT CYCLE ─────────────┤
//	│                 │                                  │
//	│  PhaseFramer ───┤  PhaseExplorer (standard/deep)   │
//	│    ↑ reframe    │    │ (explore)                    │
//	│    │            │    ▼                              │
//	│    │            │  PhaseDecider                     │
//	│    │            │    │ (decide)                     │
//	│    │            │    ▼                              │
//	│    │            │  PhaseWorker ◄──┐ tests fail      │
//	│    │            │    │ (implement)│ (signal-only)   │
//	│    │            │    ▼            │                 │
//	│    │            │  PhaseMeasure ──┘                 │
//	│    │            │    │ measure fail                 │
//	│    └────────────┘    │                              │
//	│                      ▼                              │
//	│                   PhaseReady (complete)               │
//	└─────────────────────────────────────────────────────┘
//
// Pure function.
func DeriveNextPhase(current Phase, signal TransitionSignal, depth Depth) Phase {
	switch current {
	case PhaseReady:
		switch signal {
		case SignalProblemFramed:
			if depth == DepthTactical {
				return PhaseDecider
			}
			return PhaseExplorer
		case SignalVariantsExplored:
			return PhaseDecider
		case SignalDecisionMade:
			return PhaseWorker
		}
		return PhaseReady

	case PhaseFramer:
		switch signal {
		case SignalProblemFramed:
			if depth == DepthTactical {
				return PhaseDecider
			}
			return PhaseExplorer
		case SignalLLMDone:
			// Framer finished without calling quint_problem(frame).
			// This means either: answered a question, or greeted the user.
			// In both cases the cycle is complete for this turn.
			// If there's a task that needs implementation, framer MUST frame it
			// (even a lightweight tactical frame) to trigger the cycle.
			return PhaseReady
		}
		return PhaseFramer

	case PhaseExplorer:
		switch signal {
		case SignalVariantsExplored, SignalLLMDone:
			return PhaseDecider
		}
		return PhaseExplorer

	case PhaseDecider:
		switch signal {
		case SignalDecisionMade, SignalLLMDone:
			return PhaseWorker
		}
		return PhaseDecider

	case PhaseWorker:
		switch signal {
		case SignalImplemented, SignalLLMDone:
			return PhaseMeasure
		}
		return PhaseWorker

	case PhaseMeasure:
		switch signal {
		case SignalMeasured, SignalTestsPassed:
			return PhaseReady // accepted → lemniscate closes
		case SignalMeasureFailed:
			return PhaseFramer // measurement failed → REFRAME (the loop!)
		case SignalTestsFailed:
			return PhaseWorker // tests failed → fix code (tight loop)
		case SignalLLMDone:
			return PhaseReady // validator finished → done
		}
		return PhaseMeasure
	}

	return PhaseReady
}

// ValidateProposal checks if a proposed phase transition is valid.
//
// Signal-driven proposals (fast path) are TRUSTED — the signal confirms
// the tool was called. NavState validation is only used for the fallback path
// (DeriveFromNavState) where no signal fired.
//
// Project-wide NavState is too coarse for per-cycle validation — old shipped
// decisions would permanently block new problems from entering decider.
//
// Pure function — no I/O, no store access.
func ValidateProposal(proposed Phase, status NavStatus, signal TransitionSignal) bool {
	// Signal-driven transitions: trust the signal.
	// The signal already confirms the tool was called successfully.
	if signal != "" && signal != SignalLLMDone {
		return true
	}
	// LLMDone and no-signal transitions: always valid (framer answered, worker done, etc.)
	return true
}

// DeriveFromNavState proposes a phase transition based on artifact state.
// This is the SLOW PATH fallback — used when signals are silent.
// Returns current phase if no transition is warranted.
//
// Pure function.
func DeriveFromNavState(current Phase, status NavStatus, depth Depth) Phase {
	switch current {
	case PhaseFramer:
		switch status {
		case NavFramed:
			if depth == DepthTactical {
				return PhaseDecider
			}
			return PhaseExplorer
		}
		return PhaseFramer

	case PhaseExplorer:
		switch status {
		case NavExploring, NavCompared:
			return PhaseDecider
		}
		return PhaseExplorer

	case PhaseDecider:
		if status == NavDecided {
			return PhaseWorker
		}
		return PhaseDecider

	case PhaseWorker:
		return PhaseWorker // NavState can't detect implementation (not an artifact)

	case PhaseMeasure:
		return PhaseMeasure // measure transitions are signal-driven
	}

	return current
}

// FilterToolsForPhase returns only the tools allowed in the given phase.
// Deterministic — dispatch-level gating, not prompt-level.
// Pure function.
func FilterToolsForPhase(allTools []ToolSchema, phase PhaseDef) []ToolSchema {
	if len(phase.AllowedTools) == 0 {
		return allTools // no restrictions
	}

	allowed := make(map[string]bool, len(phase.AllowedTools))
	for _, name := range phase.AllowedTools {
		allowed[name] = true
	}

	var filtered []ToolSchema
	for _, tool := range allTools {
		if allowed[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

// IsToolAllowed checks if a specific tool is permitted in the given phase.
// Pure function.
func IsToolAllowed(toolName string, phase PhaseDef) bool {
	if len(phase.AllowedTools) == 0 {
		return true
	}
	for _, name := range phase.AllowedTools {
		if name == toolName {
			return true
		}
	}
	return false
}

// BuildTransitionInstruction creates the system message injected on phase change.
// Pure function.
func BuildTransitionInstruction(from, to Phase, summary string) string {
	var b strings.Builder
	b.WriteString("[Phase transition: ")
	if from != PhaseReady {
		b.WriteString(string(from))
	} else {
		b.WriteString("start")
	}
	b.WriteString(" → ")
	b.WriteString(string(to))
	b.WriteString("]\n")

	if summary != "" {
		b.WriteString("Previous phase summary: ")
		b.WriteString(summary)
		b.WriteString("\n")
	}

	switch to {
	case PhaseFramer:
		b.WriteString("You are now haft-framer. Understand the problem before solving it. ")
		b.WriteString("Read code, search for context, identify the root cause. ")
		b.WriteString("Call quint_problem(action=frame) when you understand the problem.")
	case PhaseExplorer:
		b.WriteString("You are now haft-explorer. Generate genuinely distinct solution variants. ")
		b.WriteString("At least 2 approaches that differ in kind, not degree. Each needs a weakest_link. ")
		b.WriteString("Call quint_solution(action=explore) with your variants.")
	case PhaseDecider:
		b.WriteString("You are now haft-decider. Record your decision with rationale before implementing. ")
		b.WriteString("Compare variants on explicit dimensions. Present comparison to the user if present. ")
		b.WriteString("Call quint_decision(action=decide) with your selection and reasoning.")
	case PhaseWorker:
		b.WriteString("You are now haft-worker. Implement the solution. ")
		b.WriteString("Write code, run tests, make small reversible changes. ")
		b.WriteString("Your tools: bash, read, write, edit, glob, grep.")
	case PhaseMeasure:
		b.WriteString("You are now haft-measure. Validate the implementation. ")
		b.WriteString("Run tests, check acceptance criteria from the problem frame. ")
		b.WriteString("Report findings. Call quint_decision(action=measure) if a formal decision was recorded.")
	}

	return b.String()
}
