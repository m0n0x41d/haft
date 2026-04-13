package agent

import (
	"fmt"
	"strings"
)

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
	if cycle == nil || cycle.Status != CycleActive || cycle.ProblemRef == "" {
		return &GuardrailError{
			Tool:     "haft_solution(explore)",
			Missing:  "problem frame bound to active cycle",
			Guidance: "Frame a new problem: haft_problem(action=\"frame\", signal=..., acceptance=...) OR adopt an existing one: haft_problem(action=\"adopt\", ref=\"prob-...\"). Note: characterize does not create a cycle — you need frame or adopt first.",
		}
	}
	if cycle.DecisionRef != "" {
		return &GuardrailError{
			Tool:     "haft_solution(explore)",
			Missing:  "an undecided active cycle",
			Guidance: "This cycle already has a recorded decision. Run baseline/measure on that decision, or frame/adopt a new problem before exploring another option set.",
		}
	}
	return nil
}

// CanCompare checks if haft_solution(compare) is allowed.
// Requires: solution portfolio exists (PortfolioRef on cycle).
func CanCompare(cycle *Cycle) error {
	if cycle == nil || cycle.Status != CycleActive || cycle.PortfolioRef == "" {
		return &GuardrailError{
			Tool:     "haft_solution(compare)",
			Missing:  "solution portfolio",
			Guidance: "Explore variants first: haft_solution(action=\"explore\", variants=[...])",
		}
	}
	if cycle.DecisionRef != "" {
		return &GuardrailError{
			Tool:     "haft_solution(compare)",
			Missing:  "an undecided active cycle",
			Guidance: "This cycle already has a recorded decision. Baseline/measure that decision, or frame/adopt a new problem before comparing again.",
		}
	}
	return nil
}

// CanDecide checks if haft_decision(decide) is allowed.
// Requires:
//   - Solution portfolio exists (explored variants) — FPF B.5.2
//   - Completed compare for the active portfolio
//   - Explicit user selection recorded for the active compared portfolio
//     (Transformer Mandate) — unless autonomous
//
// userSelectedAfterCompare should be true if the active cycle records an
// explicit human selection for the active compared portfolio. Pass true in
// autonomous mode.
func CanDecide(cycle *Cycle, userSelectedAfterCompare bool) error {
	if cycle == nil || cycle.Status != CycleActive || cycle.PortfolioRef == "" {
		return &GuardrailError{
			Tool:     "haft_decision(decide)",
			Missing:  "explored variants",
			Guidance: "Explore at least 2 variants first: haft_solution(action=\"explore\", variants=[...]). FPF B.5.2 requires rival candidates.",
		}
	}
	if cycle.DecisionRef != "" {
		return &GuardrailError{
			Tool:     "haft_decision(decide)",
			Missing:  "an undecided active cycle",
			Guidance: "This cycle already has a decision. Run baseline/measure for that decision, or frame/adopt a new problem before recording another one.",
		}
	}
	if cycle.ComparedPortfolioRef == "" || cycle.ComparedPortfolioRef != cycle.PortfolioRef {
		return &GuardrailError{
			Tool:     "haft_decision(decide)",
			Missing:  "completed comparison for the active portfolio",
			Guidance: "Compare the active variants first: haft_solution(action=\"compare\", portfolio_ref=...) and show the Pareto front before deciding.",
		}
	}
	if !userSelectedAfterCompare {
		return &GuardrailError{
			Tool:     "haft_decision(decide)",
			Missing:  "user selection",
			Guidance: "Present the compare summary to the user and wait for their choice. The Transformer Mandate (FPF A.12) applies at the compare -> decide boundary: you may frame, explore, and compare when delegated, but the human selects before haft_decision(action=\"decide\").",
		}
	}
	return nil
}

// HasDecisionSelection reports whether the cycle records an explicit human
// choice for the currently active compared portfolio.
func HasDecisionSelection(cycle *Cycle) bool {
	if cycle == nil {
		return false
	}
	if strings.TrimSpace(cycle.ComparedPortfolioRef) == "" {
		return false
	}
	if cycle.SelectedPortfolioRef != cycle.ComparedPortfolioRef {
		return false
	}
	return strings.TrimSpace(cycle.SelectedVariantRef) != ""
}

// CanBaseline checks if haft_decision(baseline) is allowed.
// Requires: an active cycle with a recorded decision.
func CanBaseline(cycle *Cycle) error {
	if cycle == nil || cycle.Status != CycleActive || cycle.DecisionRef == "" {
		return &GuardrailError{
			Tool:     "haft_decision(baseline)",
			Missing:  "decision record",
			Guidance: "Record a decision first: haft_decision(action=\"decide\", selected_title=..., why_selected=...)",
		}
	}
	return nil
}

// CanMeasure checks if haft_decision(measure) is allowed.
// Requires: decision exists (DecisionRef on cycle).
func CanMeasure(cycle *Cycle) error {
	if cycle == nil || cycle.Status != CycleActive || cycle.DecisionRef == "" {
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
func CheckREff(rEff float64, fEff ...int) error {
	var guidance []string

	if rEff < 0.3 {
		guidance = append(guidance,
			fmt.Sprintf("R_eff=%.2f is below 0.3 (AT RISK). Run tests, verify implementation, or attach evidence before closing the cycle.", rEff),
		)
	}

	if len(fEff) > 0 && fEff[0] == 0 {
		guidance = append(guidance,
			"F_eff=F0 (unsubstantiated). The closure path has no structured explicit evidence; record at least structured-informal evidence before treating the cycle as closed.",
		)
	}

	if len(guidance) == 0 {
		return nil
	}

	return &GuardrailError{
		Tool:     "cycle closure",
		Missing:  "sufficient substantiated evidence",
		Guidance: strings.Join(guidance, " "),
	}
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
