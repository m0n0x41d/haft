package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type durableProfileOnboardingSnapshotV1State struct {
	values ProfileOnboardingValueSetV1
}

// DurableProfileOnboardingSnapshotV1 is a package-owned same-snapshot view of
// the strictly reconstructed final-v1 values. Profile-ledger integrity belongs
// to the binding admission adapter, not this Work-DAG storage boundary.
type DurableProfileOnboardingSnapshotV1 struct {
	state *durableProfileOnboardingSnapshotV1State
}

func (value DurableProfileOnboardingSnapshotV1) Values() (
	ProfileOnboardingValueSetV1,
	bool,
) {
	if !value.valid() {
		return ProfileOnboardingValueSetV1{}, false
	}
	return value.state.values, true
}

func (value DurableProfileOnboardingSnapshotV1) valid() bool {
	if value.state == nil {
		return false
	}
	err := validateProfileOnboardingValueSetV1(value.state.values)
	return err == nil
}

// StoreAndReloadProfileOnboardingValueSetV1 persists all separately addressed
// values in dependency order and strictly rereads them. The concrete capability
// proves that BEGIN IMMEDIATE succeeded and owns commit or rollback outside this
// function. This function does not mutate authority, admission, or
// profile-revision tables.
func StoreAndReloadProfileOnboardingValueSetV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectRoot projectprofile.ProjectRootV1,
	values ProfileOnboardingValueSetV1,
	recordedAt time.Time,
) (DurableProfileOnboardingSnapshotV1, error) {
	if ctx == nil {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf("profile-onboarding storage context is required")
	}
	if err := transaction.RequireImmediate(); err != nil {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf("profile-onboarding storage transaction is invalid: %w", err)
	}
	rows, err := prepareProfileOnboardingRowsV1(projectRoot, values, recordedAt)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	err = persistProfileOnboardingRowsV1(ctx, transaction, rows)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	identity, err := profileOnboardingValueIdentityFromRowsV1(
		projectRoot,
		values,
		rows,
	)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	snapshot, err := resolveProfileOnboardingSnapshotV1(
		ctx,
		transaction,
		identity,
	)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	actual, ok := snapshot.Values()
	if !ok {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf("persisted profile-onboarding snapshot is unusable")
	}
	actualRows, err := prepareProfileOnboardingRowsV1(projectRoot, actual, recordedAt)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	if !sameProfileOnboardingRowSemantics(actualRows, rows) {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf("persisted profile-onboarding value set differs from requested values")
	}
	return snapshot, nil
}

// ResolveProfileOnboardingValueSetV1 restarts from only durable identities and
// expected digests. It walks the stored dependency refs, strictly decodes and
// redigests every value, and validates all direct relations through one opaque
// caller-owned SQLite transaction.
func ResolveProfileOnboardingValueSetV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	identity ProfileOnboardingValueIdentityV1,
) (DurableProfileOnboardingSnapshotV1, error) {
	if ctx == nil {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf("profile-onboarding resolve context is required")
	}
	if err := transaction.RequireActive(); err != nil {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf("profile-onboarding resolve transaction is invalid: %w", err)
	}
	validated, err := validateProfileOnboardingValueIdentityV1(identity)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	return resolveProfileOnboardingSnapshotV1(
		ctx,
		transaction,
		validated,
	)
}

func profileOnboardingValueIdentityFromRowsV1(
	projectRoot projectprofile.ProjectRootV1,
	values ProfileOnboardingValueSetV1,
	rows profileOnboardingRowsV1,
) (ProfileOnboardingValueIdentityV1, error) {
	workDigest, err := projectprofile.NewContentDigest(rows.work.digest)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, fmt.Errorf("parse prepared Work digest: %w", err)
	}
	assessmentDigest, err := projectprofile.NewContentDigest(rows.assessment.digest)
	if err != nil {
		return ProfileOnboardingValueIdentityV1{}, fmt.Errorf("parse prepared assessment digest: %w", err)
	}
	workRef := values.workRecord.RecordRef()
	assessmentRef := values.assessment.Ref()
	builder := NewProfileOnboardingValueIdentityV1Builder(projectRoot)
	builder = builder.WithWork(workRef, workDigest)
	builder = builder.WithAssessment(assessmentRef, assessmentDigest)
	return builder.Build()
}

func persistProfileOnboardingRowsV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	rows profileOnboardingRowsV1,
) error {
	operations := []profileOnboardingPersistOperationV1{
		func(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
			return persistMethodDescription(ctx, transaction, rows.description)
		},
		func(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
			return persistMethodContract(ctx, transaction, rows.contract)
		},
		func(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
			return persistSystemAdmission(ctx, transaction, rows.system)
		},
		func(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
			return persistRoleAdmission(ctx, transaction, rows.role)
		},
		func(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
			return persistAssignmentSupport(ctx, transaction, rows.support)
		},
		func(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
			return persistRoleAssignment(ctx, transaction, rows.assignment)
		},
		func(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
			return persistObservedBasis(ctx, transaction, rows.basis)
		},
		func(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
			return persistWork(ctx, transaction, rows.work)
		},
		func(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
			return persistEffect(ctx, transaction, rows.effect)
		},
		func(ctx context.Context, transaction *sqlitetransaction.Transaction) error {
			return persistAssessment(ctx, transaction, rows.assessment)
		},
	}
	return visitProfileOnboardingPersistOperationsV1(
		ctx,
		transaction,
		operations,
	)
}

type profileOnboardingPersistOperationV1 func(
	context.Context,
	*sqlitetransaction.Transaction,
) error

func visitProfileOnboardingPersistOperationsV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	operations []profileOnboardingPersistOperationV1,
) error {
	if len(operations) == 0 {
		return nil
	}
	operation := operations[0]
	err := operation(ctx, transaction)
	if err != nil {
		return err
	}
	remaining := operations[1:]
	return visitProfileOnboardingPersistOperationsV1(
		ctx,
		transaction,
		remaining,
	)
}

func validateWorkRecordRef(
	value projectprofile.ProfileOnboardingWorkRecordRef,
) (projectprofile.ProfileOnboardingWorkRecordRef, error) {
	raw := value.String()
	parsed, err := projectprofile.NewProfileOnboardingWorkRecordRef(raw)
	if err != nil {
		return projectprofile.ProfileOnboardingWorkRecordRef{}, fmt.Errorf("validate Work-record ref: %w", err)
	}
	return parsed, nil
}

func validateContentDigest(
	label string,
	value projectprofile.ContentDigest,
) (projectprofile.ContentDigest, error) {
	raw := value.String()
	parsed, err := projectprofile.NewContentDigest(raw)
	if err != nil {
		return projectprofile.ContentDigest{}, fmt.Errorf("validate %s digest: %w", label, err)
	}
	return parsed, nil
}

func resolveProfileOnboardingSnapshotV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	identity ProfileOnboardingValueIdentityV1,
) (DurableProfileOnboardingSnapshotV1, error) {
	projectRoot := identity.projectRoot
	workRecordRef := identity.workRef.String()
	expectedWorkDigest := identity.workDigest.String()
	assessmentRef := identity.assessmentRef.String()
	expectedAssessmentDigest := identity.assessmentDigest.String()
	rootKey := projectRoot.String()
	work, err := loadWork(ctx, transaction, workRecordRef, rootKey)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	if work.digest != expectedWorkDigest {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf("durable Work digest does not match expected digest")
	}
	assessment, err := loadAssessment(ctx, transaction, assessmentRef)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	if assessment.digest != expectedAssessmentDigest {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf("durable outcome-assessment digest does not match expected digest")
	}
	if assessment.workRecordRef != work.workRecordRef || assessment.workRecordDigest != work.digest {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf("durable assessment points to another Work record")
	}
	effect, err := loadEffect(ctx, transaction, assessment.effectRef)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	description, err := loadMethodDescription(ctx, transaction, work.methodDescriptionRef)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	contract, err := loadMethodContract(ctx, transaction, work.methodContractRef)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	assignment, err := loadRoleAssignment(ctx, transaction, work.profileAuthorRoleAssignmentRef)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	system, err := loadSystemAdmission(ctx, transaction, assignment.systemAdmissionRef)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	role, err := loadRoleAdmission(ctx, transaction, assignment.roleAdmissionRef)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	support, err := loadAssignmentSupport(ctx, transaction, assignment.justificationRef)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	basis, err := loadObservedBasis(ctx, transaction, work.observedProjectBasisRef)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	rows := profileOnboardingRowsV1{
		description: description,
		contract:    contract,
		system:      system,
		role:        role,
		support:     support,
		assignment:  assignment,
		basis:       basis,
		work:        work,
		effect:      effect,
		assessment:  assessment,
	}
	values, err := reconstructProfileOnboardingValueSetV1(rows)
	if err != nil {
		return DurableProfileOnboardingSnapshotV1{}, err
	}
	state := durableProfileOnboardingSnapshotV1State{
		values: values,
	}
	snapshot := DurableProfileOnboardingSnapshotV1{state: &state}
	if !snapshot.valid() {
		return DurableProfileOnboardingSnapshotV1{}, fmt.Errorf("resolved profile-onboarding snapshot is invalid")
	}
	return snapshot, nil
}

func reconstructProfileOnboardingValueSetV1(
	rows profileOnboardingRowsV1,
) (ProfileOnboardingValueSetV1, error) {
	description, err := reconstructMethodDescription(rows.description)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	contract, err := reconstructMethodContract(rows.contract)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	system, err := reconstructSystemAdmission(rows.system)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	role, err := reconstructRoleAdmission(rows.role)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	justification, provenance, err := reconstructAssignmentSupport(rows.support)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	assignment, err := reconstructRoleAssignment(rows.assignment)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	basis, err := reconstructObservedBasis(rows.basis)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	work, err := reconstructWork(rows.work)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	effect, err := reconstructEffect(rows.effect)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	assessment, err := reconstructAssessment(rows.assessment, effect)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	err = validateDurableRecordingTimes(rows, provenance, work)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	builder := NewProfileOnboardingValueSetV1Builder(work)
	builder, err = withValueSetMethodEdition(builder, description, contract)
	if err != nil {
		return ProfileOnboardingValueSetV1{}, err
	}
	builder = builder.WithSystemAdmission(system)
	builder = builder.WithRoleAdmission(role)
	builder = builder.WithAssignmentJustification(justification)
	builder = builder.WithAssignmentProvenance(provenance)
	builder = builder.WithRoleAssignment(assignment)
	builder = builder.WithObservedBasis(basis)
	builder = builder.WithEffect(effect)
	builder = builder.WithAssessment(assessment)
	return builder.Build()
}

func withValueSetMethodEdition(
	builder ProfileOnboardingValueSetV1Builder,
	description projectprofile.ProfileOnboardingMethodDescriptionEdition,
	contract projectprofile.ProfileOnboardingMethodContractEdition,
) (ProfileOnboardingValueSetV1Builder, error) {
	switch exactDescription := description.(type) {
	case projectprofile.ProfileOnboardingMethodDescriptionV1:
		exactContract, ok := contract.(projectprofile.ProfileOnboardingMethodContractV1)
		if !ok {
			return builder, fmt.Errorf("durable profile-onboarding method editions differ")
		}
		builder = builder.WithMethodDescription(exactDescription)
		builder = builder.WithMethodContract(exactContract)
		return builder, nil
	case projectprofile.ProfileOnboardingMethodDescriptionV2:
		exactContract, ok := contract.(projectprofile.ProfileOnboardingMethodContractV2)
		if !ok {
			return builder, fmt.Errorf("durable profile-onboarding method editions differ")
		}
		builder = builder.WithMethodDescriptionV2(exactDescription)
		builder = builder.WithMethodContractV2(exactContract)
		return builder, nil
	default:
		return builder, fmt.Errorf("durable profile-onboarding method edition is unsupported")
	}
}

func sameProfileOnboardingRowSemantics(
	left profileOnboardingRowsV1,
	right profileOnboardingRowsV1,
) bool {
	left.description.recordedAt = ""
	right.description.recordedAt = ""
	left.contract.recordedAt = ""
	right.contract.recordedAt = ""
	left.system.recordedAt = ""
	right.system.recordedAt = ""
	left.role.recordedAt = ""
	right.role.recordedAt = ""
	left.support.recordedAt = ""
	right.support.recordedAt = ""
	left.assignment.recordedAt = ""
	right.assignment.recordedAt = ""
	left.basis.recordedAt = ""
	right.basis.recordedAt = ""
	left.work.recordedAt = ""
	right.work.recordedAt = ""
	left.effect.recordedAt = ""
	right.effect.recordedAt = ""
	left.assessment.recordedAt = ""
	right.assessment.recordedAt = ""
	return left == right
}
