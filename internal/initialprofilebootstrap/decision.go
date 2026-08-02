// Package initialprofilebootstrap owns the pure policy for deciding whether
// haft init may apply an initial project profile. It performs no IO and grants
// no authority outside the closed supported-singleton policy.
package initialprofilebootstrap

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/profiledetector"
)

type DecisionKind string

const (
	KeepExisting            DecisionKind = "keep_existing"
	ApplySupportedSingleton DecisionKind = "apply_supported_singleton"
	HumanReviewRequired     DecisionKind = "human_review_required"
)

type ReviewDisposition string

const (
	ReviewAbsent            ReviewDisposition = "absent"
	ReviewGeneratedUnedited ReviewDisposition = "generated_unedited"
	ReviewHumanOrForeign    ReviewDisposition = "human_or_foreign"
)

type ReviewReason string

const (
	ReviewReasonExistingProfile       ReviewReason = "existing_profile"
	ReviewReasonHumanOrForeignReview  ReviewReason = "human_or_foreign_review"
	ReviewReasonTruncatedObservation  ReviewReason = "truncated_observation"
	ReviewReasonUnsupportedConfidence ReviewReason = "unsupported_confidence"
	ReviewReasonScopeCardinality      ReviewReason = "scope_cardinality"
)

// InitialProfileBootstrapDecision is the closed pure result consumed by init
// planning. Only the apply variant carries a detector scope.
type InitialProfileBootstrapDecision struct {
	kind   DecisionKind
	reason ReviewReason
	scope  profiledetector.SuggestedScope
}

func Decide(
	currentProfileExists bool,
	review ReviewDisposition,
	suggestion profiledetector.Suggestion,
) (InitialProfileBootstrapDecision, error) {
	if !validReviewDisposition(review) {
		return InitialProfileBootstrapDecision{}, fmt.Errorf(
			"initial profile review disposition is invalid",
		)
	}
	if currentProfileExists {
		return InitialProfileBootstrapDecision{
			kind:   KeepExisting,
			reason: ReviewReasonExistingProfile,
		}, nil
	}
	if review == ReviewHumanOrForeign {
		return InitialProfileBootstrapDecision{
			kind:   HumanReviewRequired,
			reason: ReviewReasonHumanOrForeignReview,
		}, nil
	}
	if suggestion.Snapshot().Truncated() {
		return InitialProfileBootstrapDecision{
			kind:   HumanReviewRequired,
			reason: ReviewReasonTruncatedObservation,
		}, nil
	}
	if suggestion.ConfidencePosture() != profiledetector.SupportedConfidence {
		return InitialProfileBootstrapDecision{
			kind:   HumanReviewRequired,
			reason: ReviewReasonUnsupportedConfidence,
		}, nil
	}
	scopes := suggestion.SuggestedScopes()
	if len(scopes) != 1 {
		return InitialProfileBootstrapDecision{
			kind:   HumanReviewRequired,
			reason: ReviewReasonScopeCardinality,
		}, nil
	}
	return InitialProfileBootstrapDecision{
		kind:  ApplySupportedSingleton,
		scope: scopes[0],
	}, nil
}

func (decision InitialProfileBootstrapDecision) Kind() DecisionKind {
	return decision.kind
}

func (decision InitialProfileBootstrapDecision) ReviewReason() (
	ReviewReason,
	bool,
) {
	if decision.kind == ApplySupportedSingleton || decision.reason == "" {
		return "", false
	}
	return decision.reason, true
}

func (decision InitialProfileBootstrapDecision) SupportedSingleton() (
	profiledetector.SuggestedScope,
	bool,
) {
	if decision.kind != ApplySupportedSingleton {
		return profiledetector.SuggestedScope{}, false
	}
	return decision.scope, true
}

func validReviewDisposition(value ReviewDisposition) bool {
	return value == ReviewAbsent ||
		value == ReviewGeneratedUnedited ||
		value == ReviewHumanOrForeign
}
