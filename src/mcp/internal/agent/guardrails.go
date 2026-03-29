package agent

import "fmt"

// ---------------------------------------------------------------------------
// L1: FPF Guardrails — pure functions for tool precondition checks.
//
// Tools call these before executing. If precondition fails, tool returns
// a guidance error that the LLM reads and self-corrects.
// This replaces the v1 phase machine — FPF order enforced at tool level.
// ---------------------------------------------------------------------------

// CanExplore checks if quint_solution(explore) is allowed.
// Requires: problem framed (ProblemRef on cycle).
func CanExplore(cycle *Cycle) error {
	if cycle == nil || cycle.ProblemRef == "" {
		return &GuardrailError{
			Tool:     "quint_solution(explore)",
			Missing:  "problem frame",
			Guidance: "Frame the problem first: quint_problem(action=\"frame\", signal=..., acceptance=...)",
		}
	}
	return nil
}

// CanCompare checks if quint_solution(compare) is allowed.
// Requires: solution portfolio exists (PortfolioRef on cycle).
func CanCompare(cycle *Cycle) error {
	if cycle == nil || cycle.PortfolioRef == "" {
		return &GuardrailError{
			Tool:     "quint_solution(compare)",
			Missing:  "solution portfolio",
			Guidance: "Explore variants first: quint_solution(action=\"explore\", variants=[...])",
		}
	}
	return nil
}

// CanDecide checks if quint_decision(decide) is allowed.
// Requires: solution portfolio exists (explored variants).
func CanDecide(cycle *Cycle) error {
	if cycle == nil || cycle.PortfolioRef == "" {
		return &GuardrailError{
			Tool:     "quint_decision(decide)",
			Missing:  "explored variants",
			Guidance: "Explore at least 2 variants first: quint_solution(action=\"explore\", variants=[...]). FPF B.5.2 requires rival candidates.",
		}
	}
	return nil
}

// CanMeasure checks if quint_decision(measure) is allowed.
// Requires: decision exists (DecisionRef on cycle).
func CanMeasure(cycle *Cycle) error {
	if cycle == nil || cycle.DecisionRef == "" {
		return &GuardrailError{
			Tool:     "quint_decision(measure)",
			Missing:  "decision record",
			Guidance: "Record a decision first: quint_decision(action=\"decide\", selected_title=..., why_selected=...)",
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
