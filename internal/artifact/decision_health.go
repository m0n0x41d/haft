package artifact

import "context"

// DecisionMaturity is the derived maturity axis for active decisions.
type DecisionMaturity string

const (
	DecisionMaturityUnassessed DecisionMaturity = "Unassessed"
	DecisionMaturityPending    DecisionMaturity = "Pending"
	DecisionMaturityShipped    DecisionMaturity = "Shipped"
)

// DecisionFreshness is the derived freshness axis for shipped decisions.
type DecisionFreshness string

const (
	DecisionFreshnessHealthy DecisionFreshness = "Healthy"
	DecisionFreshnessStale   DecisionFreshness = "Stale"
	DecisionFreshnessAtRisk  DecisionFreshness = "AT RISK"
)

// DecisionHealth is the derived, never-stored decision health view.
type DecisionHealth struct {
	Maturity  DecisionMaturity
	Freshness DecisionFreshness
}

func (health DecisionHealth) Label() string {
	if health.Maturity != DecisionMaturityShipped {
		return string(health.Maturity)
	}

	if health.Freshness == "" {
		return string(health.Maturity)
	}

	return string(health.Maturity) + " / " + string(health.Freshness)
}

// DeriveDecisionHealth computes the derived maturity + freshness view from
// active evidence only. The result is never persisted.
func DeriveDecisionHealth(ctx context.Context, store ArtifactStore, decisionID string) DecisionHealth {
	items, err := store.GetEvidenceItems(ctx, decisionID)
	if err != nil {
		return DecisionHealth{Maturity: DecisionMaturityUnassessed}
	}

	activeItems := activeEvidenceItems(items)
	if len(activeItems) == 0 {
		return DecisionHealth{Maturity: DecisionMaturityUnassessed}
	}

	if !hasAcceptedMeasurementEvidence(activeItems) {
		return DecisionHealth{Maturity: DecisionMaturityPending}
	}

	health := DecisionHealth{Maturity: DecisionMaturityShipped}
	reliability := ComputeWLNKSummary(ctx, store, decisionID).REff

	if reliability < 0.3 {
		health.Freshness = DecisionFreshnessAtRisk
		return health
	}

	if reliability < 0.5 {
		health.Freshness = DecisionFreshnessStale
		return health
	}

	health.Freshness = DecisionFreshnessHealthy
	return health
}

func activeEvidenceItems(items []EvidenceItem) []EvidenceItem {
	activeItems := make([]EvidenceItem, 0, len(items))

	for _, item := range items {
		if item.Verdict == "superseded" {
			continue
		}

		activeItems = append(activeItems, item)
	}

	return activeItems
}

func hasAcceptedMeasurementEvidence(items []EvidenceItem) bool {
	for _, item := range items {
		if item.Type != "measurement" {
			continue
		}

		if item.Verdict == "supports" || item.Verdict == "accepted" {
			return true
		}
	}

	return false
}

func hasMeasurement(ctx context.Context, store ArtifactStore, decisionID string) bool {
	items, err := store.GetEvidenceItems(ctx, decisionID)
	if err != nil {
		return false
	}

	for _, item := range activeEvidenceItems(items) {
		if item.Type == "measurement" {
			return true
		}
	}

	return false
}
