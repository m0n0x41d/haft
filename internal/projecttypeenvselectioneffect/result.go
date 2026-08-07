package projecttypeenvselectioneffect

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ProjectTypeEnvHeadSelectionDeliveryPosture is closed. It describes only
// delivery knowledge after the pre-commit effect record was sealed.
type ProjectTypeEnvHeadSelectionDeliveryPosture interface {
	projectTypeEnvHeadSelectionDeliveryPostureVariant()
}

// SuccessfulProjectTypeEnvHeadSelectionDeliveryPosture excludes the unknown
// outcome from FreshlyCommitted at compile time.
type SuccessfulProjectTypeEnvHeadSelectionDeliveryPosture interface {
	ProjectTypeEnvHeadSelectionDeliveryPosture
	successfulProjectTypeEnvHeadSelectionDeliveryPosture()
}

type CommittedAndObserved struct{}

func (CommittedAndObserved) projectTypeEnvHeadSelectionDeliveryPostureVariant() {}
func (CommittedAndObserved) successfulProjectTypeEnvHeadSelectionDeliveryPosture() {
}

type CommitRecoveredByExactClosureReread struct{}

func (CommitRecoveredByExactClosureReread) projectTypeEnvHeadSelectionDeliveryPostureVariant() {
}
func (CommitRecoveredByExactClosureReread) successfulProjectTypeEnvHeadSelectionDeliveryPosture() {
}

type CommitOutcomeUnknownPosture struct{}

func (CommitOutcomeUnknownPosture) projectTypeEnvHeadSelectionDeliveryPostureVariant() {
}

// ProjectTypeEnvHeadSelectionResult is a closed public outcome. Only the two
// success variants contain a closure; NotSelected cannot carry a receipt.
type ProjectTypeEnvHeadSelectionResult interface {
	projectTypeEnvHeadSelectionResultVariant()
}

type FreshlyCommitted struct {
	closure  ProjectTypeEnvHeadSelectionClosureV1
	delivery SuccessfulProjectTypeEnvHeadSelectionDeliveryPosture
}

func NewFreshlyCommitted(
	closure ProjectTypeEnvHeadSelectionClosureV1,
	delivery SuccessfulProjectTypeEnvHeadSelectionDeliveryPosture,
) (FreshlyCommitted, error) {
	if err := closure.Verify(); err != nil {
		return FreshlyCommitted{}, err
	}
	if delivery == nil {
		return FreshlyCommitted{}, fmt.Errorf("successful delivery posture is required")
	}
	switch delivery.(type) {
	case CommittedAndObserved, CommitRecoveredByExactClosureReread:
	default:
		return FreshlyCommitted{}, fmt.Errorf("successful delivery posture is unsupported")
	}
	return FreshlyCommitted{closure: closure, delivery: delivery}, nil
}

func (FreshlyCommitted) projectTypeEnvHeadSelectionResultVariant() {}

func (result FreshlyCommitted) Closure() ProjectTypeEnvHeadSelectionClosureV1 {
	return result.closure
}

func (result FreshlyCommitted) Delivery() SuccessfulProjectTypeEnvHeadSelectionDeliveryPosture {
	return result.delivery
}

type ReplayedExisting struct {
	closure ProjectTypeEnvHeadSelectionClosureV1
}

func NewReplayedExisting(
	closure ProjectTypeEnvHeadSelectionClosureV1,
) (ReplayedExisting, error) {
	if err := closure.Verify(); err != nil {
		return ReplayedExisting{}, err
	}
	return ReplayedExisting{closure: closure}, nil
}

func (ReplayedExisting) projectTypeEnvHeadSelectionResultVariant() {}

func (result ReplayedExisting) Closure() ProjectTypeEnvHeadSelectionClosureV1 {
	return result.closure
}

type ReplayConflict struct {
	key                    projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	existingRequestDigest  typedmemory.SHA256Digest
	presentedRequestDigest typedmemory.SHA256Digest
	existingContentDigest  authority.Digest
	presentedContentDigest authority.Digest
}

type ReplayConflictInput struct {
	Key                    projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	ExistingRequestDigest  typedmemory.SHA256Digest
	PresentedRequestDigest typedmemory.SHA256Digest
	ExistingContentDigest  authority.Digest
	PresentedContentDigest authority.Digest
}

func NewReplayConflict(input ReplayConflictInput) (ReplayConflict, error) {
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		input.Key.String(),
	)
	if err != nil || key != input.Key {
		return ReplayConflict{}, fmt.Errorf("replay-conflict key is required")
	}
	existingRequest, err := typedmemory.NewSHA256Digest(
		input.ExistingRequestDigest.String(),
	)
	if err != nil || existingRequest != input.ExistingRequestDigest {
		return ReplayConflict{}, fmt.Errorf("existing request digest is required")
	}
	presentedRequest, err := typedmemory.NewSHA256Digest(
		input.PresentedRequestDigest.String(),
	)
	if err != nil || presentedRequest != input.PresentedRequestDigest {
		return ReplayConflict{}, fmt.Errorf("presented request digest is required")
	}
	existingContent, err := authority.NewDigest(
		input.ExistingContentDigest.String(),
	)
	if err != nil || existingContent != input.ExistingContentDigest {
		return ReplayConflict{}, fmt.Errorf("existing content digest is required")
	}
	presentedContent, err := authority.NewDigest(
		input.PresentedContentDigest.String(),
	)
	if err != nil || presentedContent != input.PresentedContentDigest {
		return ReplayConflict{}, fmt.Errorf("presented content digest is required")
	}
	if existingRequest == presentedRequest &&
		existingContent == presentedContent {
		return ReplayConflict{},
			fmt.Errorf("replay conflict requires a changed request or content digest")
	}
	return ReplayConflict{
		key:                    key,
		existingRequestDigest:  existingRequest,
		presentedRequestDigest: presentedRequest,
		existingContentDigest:  existingContent,
		presentedContentDigest: presentedContent,
	}, nil
}

func (ReplayConflict) projectTypeEnvHeadSelectionResultVariant() {}

func (result ReplayConflict) Key() projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey {
	return result.key
}

func (result ReplayConflict) ExistingRequestDigest() typedmemory.SHA256Digest {
	return result.existingRequestDigest
}

func (result ReplayConflict) PresentedRequestDigest() typedmemory.SHA256Digest {
	return result.presentedRequestDigest
}

func (result ReplayConflict) ExistingContentDigest() authority.Digest {
	return result.existingContentDigest
}

func (result ReplayConflict) PresentedContentDigest() authority.Digest {
	return result.presentedContentDigest
}

type ProjectTypeEnvHeadSelectionNotSelectedReason struct {
	value string
}

const (
	notSelectedStaleGraph             = "stale_graph"
	notSelectedPriorHeadExists        = "prior_head_exists"
	notSelectedPriorHeadAbsent        = "prior_head_absent"
	notSelectedStalePriorHead         = "stale_prior_head"
	notSelectedCorruptHeadSlot        = "corrupt_head_slot"
	notSelectedStageDrift             = "stage_drift"
	notSelectedTargetIntegrity        = "target_b_e_x_c_integrity_failure"
	notSelectedTargetSnapshotMissing  = "target_snapshot_missing"
	notSelectedTargetSnapshotConflict = "target_snapshot_conflict"
	notSelectedProfileDrift           = "profile_drift"
	notSelectedProfileIncompatible    = "profile_incompatible"
	notSelectedProfileUnderdetermined = "profile_underdetermined"
	notSelectedAssertionRevalidation  = "assertion_revalidation_failure"
	notSelectedCurrentAuthority       = "current_authority_rejection"
	notSelectedReviewExpired          = "review_expired"
	notSelectedCancellation           = "cancellation"
	notSelectedStorageFailure         = "storage_failure"
)

func NotSelectedStaleGraph() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedStaleGraph}
}

func NotSelectedPriorHeadExists() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedPriorHeadExists}
}

func NotSelectedPriorHeadAbsent() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedPriorHeadAbsent}
}

func NotSelectedStalePriorHead() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedStalePriorHead}
}

func NotSelectedCorruptHeadSlot() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedCorruptHeadSlot}
}

func NotSelectedStageDrift() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedStageDrift}
}

func NotSelectedTargetIntegrityFailure() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedTargetIntegrity}
}

func NotSelectedTargetSnapshotMissing() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedTargetSnapshotMissing}
}

func NotSelectedTargetSnapshotConflict() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedTargetSnapshotConflict}
}

func NotSelectedProfileDrift() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedProfileDrift}
}

func NotSelectedProfileIncompatible() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedProfileIncompatible}
}

func NotSelectedProfileUnderdetermined() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedProfileUnderdetermined}
}

func NotSelectedAssertionRevalidationFailure() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedAssertionRevalidation}
}

func NotSelectedCurrentAuthorityRejection() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedCurrentAuthority}
}

func NotSelectedReviewExpired() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedReviewExpired}
}

func NotSelectedCancellation() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedCancellation}
}

func NotSelectedStorageFailure() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return ProjectTypeEnvHeadSelectionNotSelectedReason{value: notSelectedStorageFailure}
}

func (reason ProjectTypeEnvHeadSelectionNotSelectedReason) String() string {
	return reason.value
}

func validNotSelectedReason(
	reason ProjectTypeEnvHeadSelectionNotSelectedReason,
) bool {
	switch reason.value {
	case notSelectedStaleGraph,
		notSelectedPriorHeadExists,
		notSelectedPriorHeadAbsent,
		notSelectedStalePriorHead,
		notSelectedCorruptHeadSlot,
		notSelectedStageDrift,
		notSelectedTargetIntegrity,
		notSelectedTargetSnapshotMissing,
		notSelectedTargetSnapshotConflict,
		notSelectedProfileDrift,
		notSelectedProfileIncompatible,
		notSelectedProfileUnderdetermined,
		notSelectedAssertionRevalidation,
		notSelectedCurrentAuthority,
		notSelectedReviewExpired,
		notSelectedCancellation,
		notSelectedStorageFailure:
		return true
	default:
		return false
	}
}

type NotSelected struct {
	reason ProjectTypeEnvHeadSelectionNotSelectedReason
}

func NewNotSelected(
	reason ProjectTypeEnvHeadSelectionNotSelectedReason,
) (NotSelected, error) {
	if !validNotSelectedReason(reason) {
		return NotSelected{}, fmt.Errorf("not-selected reason is unsupported")
	}
	return NotSelected{reason: reason}, nil
}

func (NotSelected) projectTypeEnvHeadSelectionResultVariant() {}

func (result NotSelected) Reason() ProjectTypeEnvHeadSelectionNotSelectedReason {
	return result.reason
}

type CommitOutcomeUnknown struct {
	retryKey      projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	requestDigest typedmemory.SHA256Digest
	contentDigest authority.Digest
	delivery      CommitOutcomeUnknownPosture
}

type CommitOutcomeUnknownInput struct {
	RetryKey      projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	RequestDigest typedmemory.SHA256Digest
	ContentDigest authority.Digest
}

func NewCommitOutcomeUnknown(
	input CommitOutcomeUnknownInput,
) (CommitOutcomeUnknown, error) {
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		input.RetryKey.String(),
	)
	if err != nil || key != input.RetryKey {
		return CommitOutcomeUnknown{}, fmt.Errorf("commit-unknown retry key is required")
	}
	requestDigest, err := typedmemory.NewSHA256Digest(input.RequestDigest.String())
	if err != nil || requestDigest != input.RequestDigest {
		return CommitOutcomeUnknown{}, fmt.Errorf("commit-unknown request digest is required")
	}
	contentDigest, err := authority.NewDigest(input.ContentDigest.String())
	if err != nil || contentDigest != input.ContentDigest {
		return CommitOutcomeUnknown{}, fmt.Errorf("commit-unknown content digest is required")
	}
	return CommitOutcomeUnknown{
		retryKey:      key,
		requestDigest: requestDigest,
		contentDigest: contentDigest,
		delivery:      CommitOutcomeUnknownPosture{},
	}, nil
}

func (CommitOutcomeUnknown) projectTypeEnvHeadSelectionResultVariant() {}

func (result CommitOutcomeUnknown) RetryKey() projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey {
	return result.retryKey
}

func (result CommitOutcomeUnknown) RequestDigest() typedmemory.SHA256Digest {
	return result.requestDigest
}

func (result CommitOutcomeUnknown) ContentDigest() authority.Digest {
	return result.contentDigest
}

func (result CommitOutcomeUnknown) Delivery() CommitOutcomeUnknownPosture {
	return result.delivery
}
