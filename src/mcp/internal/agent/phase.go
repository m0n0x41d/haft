package agent

// ---------------------------------------------------------------------------
// v2: Phase types and signals retained for backward compatibility.
// Phase machine logic (DeriveNextPhase, ValidateProposal, etc.) removed.
// FPF order enforced by tool guardrails, not phase transitions.
// ---------------------------------------------------------------------------

// TransitionSignal describes what happened during tool execution.
// Used for logging and cycle binding, not for phase transitions.
type TransitionSignal string

const (
	SignalProblemFramed    TransitionSignal = "problem_framed"
	SignalVariantsExplored TransitionSignal = "variants_explored"
	SignalDecisionMade     TransitionSignal = "decision_made"
	SignalImplemented      TransitionSignal = "implemented"
	SignalTestsPassed      TransitionSignal = "tests_passed"
	SignalTestsFailed      TransitionSignal = "tests_failed"
	SignalMeasured         TransitionSignal = "measured"
	SignalMeasureFailed    TransitionSignal = "measure_failed"
	SignalLLMDone          TransitionSignal = "llm_done"
)
