package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// ResolveProfileOnboardingValueSetByRefsV1 starts from two exact durable
// addresses when the expected digests are themselves stored inside another
// canonical authority record. It first reads the Work and assessment rows to
// recover their canonical digests, then delegates to the same strict DAG
// reconstruction used by admission. A ref is only a lookup key, never proof.
func ResolveProfileOnboardingValueSetByRefsV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectRoot projectprofile.ProjectRootV1,
	workRef projectprofile.ProfileOnboardingWorkRecordRef,
	assessmentRef projectprofile.ProfileOnboardingOutcomeAssessmentRefV1,
) (DurableProfileOnboardingSnapshotV1, error) {
	if ctx == nil {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf(
			"profile-onboarding ref resolve context is required",
		)
	}
	if err := transaction.RequireActive(); err != nil {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf(
			"profile-onboarding ref resolve transaction is invalid: %w",
			err,
		)
	}
	root, err := validateProjectRoot(projectRoot)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	canonicalWorkRef, err := validateWorkRecordRef(workRef)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	canonicalAssessmentRef, err := projectprofile.NewProfileOnboardingOutcomeAssessmentRefV1(
		assessmentRef.String(),
	)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf(
			"validate outcome-assessment ref: %w",
			err,
		)
	}
	workRow, err := loadWork(
		ctx,
		transaction,
		canonicalWorkRef.String(),
		root.String(),
	)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	assessmentRow, err := loadAssessment(
		ctx,
		transaction,
		canonicalAssessmentRef.String(),
	)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	workDigest, err := projectprofile.NewContentDigest(workRow.digest)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf(
			"parse durable Work digest: %w",
			err,
		)
	}
	assessmentDigest, err := projectprofile.NewContentDigest(assessmentRow.digest)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf(
			"parse durable outcome-assessment digest: %w",
			err,
		)
	}
	builder := NewProfileOnboardingValueIdentityV1Builder(root)
	builder = builder.WithWork(canonicalWorkRef, workDigest)
	builder = builder.WithAssessment(canonicalAssessmentRef, assessmentDigest)
	identity, err := builder.Build()
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	return ResolveProfileOnboardingValueSetV1(ctx, transaction, identity)
}
