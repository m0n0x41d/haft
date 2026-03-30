package agent

import "fmt"

// ---------------------------------------------------------------------------
// L1: FPF Guardrails — pure functions for tool precondition checks.
//
// Tools call these before executing. If precondition fails, tool returns
// a guidance error that the LLM reads and self-corrects.
// This replaces the v1 phase machine — FPF order enforced at tool level.
// ---------------------------------------------------------------------------

// CanExplore checks if haft_solution(explore) is allowed.
// Requires: problem framed (ProblemRef on cycle).
func CanExplore(cycle *Cycle) error {
	if cycle == nil || cycle.ProblemRef == "" {
		return &GuardrailError{
			Tool:     "haft_solution(explore)",
			Missing:  "problem frame",
			Guidance: "Frame the problem first: haft_problem(action=\"frame\", signal=..., acceptance=...)",
		}
	}
	return nil
}

// CanCompare checks if haft_solution(compare) is allowed.
// Requires: solution portfolio exists (PortfolioRef on cycle).
func CanCompare(cycle *Cycle) error {
	if cycle == nil || cycle.PortfolioRef == "" {
		return &GuardrailError{
			Tool:     "haft_solution(compare)",
			Missing:  "solution portfolio",
			Guidance: "Explore variants first: haft_solution(action=\"explore\", variants=[...])",
		}
	}
	return nil
}

// CanDecide checks if haft_decision(decide) is allowed.
// Requires:
//   - Solution portfolio exists (explored variants) — FPF B.5.2
//   - User has responded after explore (Transformer Mandate) — unless autonomous
//
// userRespondedAfterExplore should be true if a user message exists in history
// after the last explore tool call. Pass true in autonomous mode.
func CanDecide(cycle *Cycle, userRespondedAfterExplore bool) error {
	if cycle == nil || cycle.PortfolioRef == "" {
		return &GuardrailError{
			Tool:     "haft_decision(decide)",
			Missing:  "explored variants",
			Guidance: "Explore at least 2 variants first: haft_solution(action=\"explore\", variants=[...]). FPF B.5.2 requires rival candidates.",
		}
	}
	if !userRespondedAfterExplore {
		return &GuardrailError{
			Tool:     "haft_decision(decide)",
			Missing:  "user selection",
			Guidance: "Present the variants to the user and wait for their choice. The Transformer Mandate (FPF A.12) requires the human to select — the system proposes, the human decides.",
		}
	}
	return nil
}

// CanMeasure checks if haft_decision(measure) is allowed.
// Requires: decision exists (DecisionRef on cycle).
func CanMeasure(cycle *Cycle) error {
	if cycle == nil || cycle.DecisionRef == "" {
		return &GuardrailError{
			Tool:     "haft_decision(measure)",
			Missing:  "decision record",
			Guidance: "Record a decision first: haft_decision(action=\"decide\", selected_title=..., why_selected=...)",
		}
	}
	return nil
}

// CheckREff validates that R_eff meets minimum threshold for cycle closure.
// Returns nil if sufficient, error with guidance if not.
func CheckREff(rEff float64) error {
	if rEff < 0.3 {
		return &GuardrailError{
			Tool:     "cycle closure",
			Missing:  "sufficient evidence",
			Guidance: fmt.Sprintf("R_eff=%.2f is below 0.3 (AT RISK). Run tests, verify implementation, or attach evidence before closing the cycle.", rEff),
		}
	}
	return nil
}

// GuardrailError is returned by tools when FPF preconditions are not met.
// The LLM reads this error and self-corrects by calling the right tool.
type GuardrailError struct {
	Tool     string // which tool was blocked
	Missing  string // what precondition is missing
	Guidance string // what to do instead
}

func (e *GuardrailError) Error() string {
	return fmt.Sprintf("FPF guardrail: %s requires %s. %s", e.Tool, e.Missing, e.Guidance)
}
