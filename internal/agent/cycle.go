package agent

import "strings"

// CanonicalizeCycleForPersistence normalizes legacy wire references without
// inferring a phase or deleting independently meaningful artifacts. Selection
// is the sole coupled tuple: it is retained only when it names a variant of
// the compared portfolio recorded by the legacy cycle.
func CanonicalizeCycleForPersistence(cycle *Cycle) *Cycle {
	if cycle == nil {
		return nil
	}

	updated := *cycle
	updated.ProblemRef = strings.TrimSpace(updated.ProblemRef)
	updated.PortfolioRef = strings.TrimSpace(updated.PortfolioRef)
	updated.ComparedPortfolioRef = strings.TrimSpace(updated.ComparedPortfolioRef)
	updated.SelectedPortfolioRef = strings.TrimSpace(updated.SelectedPortfolioRef)
	updated.SelectedVariantRef = strings.TrimSpace(updated.SelectedVariantRef)
	updated.DecisionRef = strings.TrimSpace(updated.DecisionRef)

	hasSelection := updated.SelectedPortfolioRef != "" || updated.SelectedVariantRef != ""
	selectionIsConsistent := updated.ComparedPortfolioRef != "" &&
		updated.SelectedPortfolioRef == updated.ComparedPortfolioRef &&
		updated.SelectedVariantRef != ""
	if hasSelection && !selectionIsConsistent {
		updated.SelectedPortfolioRef = ""
		updated.SelectedVariantRef = ""
	}

	return &updated
}
