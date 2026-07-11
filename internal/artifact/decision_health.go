package artifact

import (
	"context"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

// DecisionMaturity is the derived maturity axis for active decisions.
type DecisionMaturity string

const (
	DecisionMaturityUnassessed DecisionMaturity = "Unassessed"
	DecisionMaturityPending    DecisionMaturity = "Pending"
	DecisionMaturityShipped    DecisionMaturity = "Shipped"
)

// DecisionEvidenceState explains why a decision is or is not assessable.
type DecisionEvidenceState string

const (
	DecisionEvidenceNoActiveEvidence DecisionEvidenceState = "no_active_evidence"
	DecisionEvidenceUnavailable      DecisionEvidenceState = "evidence_unavailable"
	DecisionEvidenceActive           DecisionEvidenceState = "active_evidence"
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
	Maturity      DecisionMaturity
	Freshness     DecisionFreshness
	EvidenceState DecisionEvidenceState
}

// DecisionVerificationSummary is the derived claim-verification view shown by
// status. verify_after is a planned evidence-check date, never a deadline or
// gate.
type DecisionVerificationSummary struct {
	ActiveClaims       int
	UnverifiedClaims   int
	NextScheduledCheck string
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
		return DecisionHealth{
			Maturity:      DecisionMaturityUnassessed,
			EvidenceState: DecisionEvidenceUnavailable,
		}
	}

	activeItems := activeEvidenceItems(items)
	if len(activeItems) == 0 {
		return DecisionHealth{
			Maturity:      DecisionMaturityUnassessed,
			EvidenceState: DecisionEvidenceNoActiveEvidence,
		}
	}

	if !hasAcceptedMeasurementEvidence(activeItems) {
		return DecisionHealth{
			Maturity:      DecisionMaturityPending,
			EvidenceState: DecisionEvidenceActive,
		}
	}

	health := DecisionHealth{
		Maturity:      DecisionMaturityShipped,
		EvidenceState: DecisionEvidenceActive,
	}
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

// DeriveDecisionVerificationSummary projects active claim verification state
// from the canonical DecisionRecord structured payload.
func DeriveDecisionVerificationSummary(decision *Artifact) DecisionVerificationSummary {
	summary := DecisionVerificationSummary{}
	if decision == nil {
		return summary
	}

	var next time.Time
	for _, claim := range decision.UnmarshalDecisionFields().Claims {
		lifecycle := EffectiveClaimLifecycleStatus(claim)
		if lifecycle != ClaimLifecycleActive && lifecycle != ClaimLifecycleRefreshDue {
			continue
		}

		summary.ActiveClaims++
		if normalizeClaimStatus(claim.Status) != ClaimStatusUnverified {
			continue
		}

		summary.UnverifiedClaims++
		verifyAt, ok := reff.ParseValidUntil(strings.TrimSpace(claim.VerifyAfter))
		if !ok || (!next.IsZero() && !verifyAt.Before(next)) {
			continue
		}
		next = verifyAt
	}

	if !next.IsZero() {
		summary.NextScheduledCheck = next.Format("2006-01-02")
	}
	return summary
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
