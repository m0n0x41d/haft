package agent

import (
	"fmt"
	"strings"
)

// CanExplore is retained as a compatibility seam. Explore is an independent
// capability, so no active-cycle predecessor is required.
func CanExplore(_ *Cycle) error {
	return nil
}

// CanCompare is retained as a compatibility seam. Compare accepts variants
// directly, so no active-cycle predecessor is required.
func CanCompare(_ *Cycle) error {
	return nil
}

// CanDecide retains only the human authority boundary. When a legacy cycle
// identifies a compared portfolio, the caller must prove that the human made
// the selection. Absence of that legacy context is not a phase/predecessor
// failure: manual decision validation belongs to the decision interface.
func CanDecide(cycle *Cycle, userSelectedAfterCompare bool) error {
	hasComparedPortfolioContext := cycle != nil && strings.TrimSpace(cycle.ComparedPortfolioRef) != ""
	if hasComparedPortfolioContext && !userSelectedAfterCompare {
		return &GuardrailError{
			Tool:     "haft_decision(decide)",
			Missing:  "user selection",
			Guidance: "The compared portfolio records alternatives, not authority. Obtain an explicit human selection before haft_decision(action=\"decide\").",
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

// CanBaseline is retained as a compatibility seam. The baseline command
// validates its explicit decision reference directly.
func CanBaseline(_ *Cycle) error {
	return nil
}

// CanMeasure is retained as a compatibility seam. The measure command
// validates its explicit decision reference directly.
func CanMeasure(_ *Cycle) error {
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

// GuardrailError is returned by tools when an authority or evidence boundary
// is not met.
type GuardrailError struct {
	Tool     string // which tool was blocked
	Missing  string // what precondition is missing
	Guidance string // what to do instead
}

func (e *GuardrailError) Error() string {
	return fmt.Sprintf("FPF guardrail: %s requires %s. %s", e.Tool, e.Missing, e.Guidance)
}
