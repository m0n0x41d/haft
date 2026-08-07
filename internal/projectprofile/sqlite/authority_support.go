package sqlite

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// ProfileOnboardingAuthoritySupportV1 is the pre-Work support closure required
// by a profile-declaration authorization. It deliberately excludes an
// ObservedProjectBasis, Work occurrence, effect, assessment, payload, and
// candidate: those values may exist only after the authority resolution has
// been durably recorded.
type ProfileOnboardingAuthoritySupportV1 struct {
	methodDescription projectprofile.ProfileOnboardingMethodDescriptionEdition
	methodContract    projectprofile.ProfileOnboardingMethodContractEdition
	systemAdmission   projectprofile.ProfileOnboardingExecutorSystemAdmissionV1
	roleAdmission     projectprofile.ProfileAuthorRoleAdmissionV1
	justification     projectprofile.ProfileAuthorAssignmentJustificationV1
	provenance        projectprofile.ProfileAuthorAssignmentProvenanceV1
	roleAssignment    projectprofile.ProfileAuthorRoleAssignmentV1
}

type ProfileOnboardingAuthoritySupportV1Builder struct {
	value ProfileOnboardingAuthoritySupportV1
}

func NewProfileOnboardingAuthoritySupportV1Builder(
	assignment projectprofile.ProfileAuthorRoleAssignmentV1,
) ProfileOnboardingAuthoritySupportV1Builder {
	return ProfileOnboardingAuthoritySupportV1Builder{
		value: ProfileOnboardingAuthoritySupportV1{roleAssignment: assignment},
	}
}

func (builder ProfileOnboardingAuthoritySupportV1Builder) WithMethodDescription(
	description projectprofile.ProfileOnboardingMethodDescriptionV1,
) ProfileOnboardingAuthoritySupportV1Builder {
	builder.value.methodDescription = description
	return builder
}

func (builder ProfileOnboardingAuthoritySupportV1Builder) WithMethodDescriptionV2(
	description projectprofile.ProfileOnboardingMethodDescriptionV2,
) ProfileOnboardingAuthoritySupportV1Builder {
	builder.value.methodDescription = description
	return builder
}

func (builder ProfileOnboardingAuthoritySupportV1Builder) WithMethodContract(
	contract projectprofile.ProfileOnboardingMethodContractV1,
) ProfileOnboardingAuthoritySupportV1Builder {
	builder.value.methodContract = contract
	return builder
}

func (builder ProfileOnboardingAuthoritySupportV1Builder) WithMethodContractV2(
	contract projectprofile.ProfileOnboardingMethodContractV2,
) ProfileOnboardingAuthoritySupportV1Builder {
	builder.value.methodContract = contract
	return builder
}

func (builder ProfileOnboardingAuthoritySupportV1Builder) WithSystemAdmission(
	admission projectprofile.ProfileOnboardingExecutorSystemAdmissionV1,
) ProfileOnboardingAuthoritySupportV1Builder {
	builder.value.systemAdmission = admission
	return builder
}

func (builder ProfileOnboardingAuthoritySupportV1Builder) WithRoleAdmission(
	admission projectprofile.ProfileAuthorRoleAdmissionV1,
) ProfileOnboardingAuthoritySupportV1Builder {
	builder.value.roleAdmission = admission
	return builder
}

func (builder ProfileOnboardingAuthoritySupportV1Builder) WithAssignmentJustification(
	justification projectprofile.ProfileAuthorAssignmentJustificationV1,
) ProfileOnboardingAuthoritySupportV1Builder {
	builder.value.justification = justification
	return builder
}

func (builder ProfileOnboardingAuthoritySupportV1Builder) WithAssignmentProvenance(
	provenance projectprofile.ProfileAuthorAssignmentProvenanceV1,
) ProfileOnboardingAuthoritySupportV1Builder {
	builder.value.provenance = provenance
	return builder
}

func (builder ProfileOnboardingAuthoritySupportV1Builder) Build() (
	ProfileOnboardingAuthoritySupportV1,
	error,
) {
	if err := validateProfileOnboardingAuthoritySupportV1(builder.value); err != nil {
		return ProfileOnboardingAuthoritySupportV1{}, err
	}
	return builder.value, nil
}

func (value ProfileOnboardingAuthoritySupportV1) MethodDescription() projectprofile.ProfileOnboardingMethodDescriptionV1 {
	description, _ := value.methodDescription.(projectprofile.ProfileOnboardingMethodDescriptionV1)
	return description
}

func (value ProfileOnboardingAuthoritySupportV1) MethodContract() projectprofile.ProfileOnboardingMethodContractV1 {
	contract, _ := value.methodContract.(projectprofile.ProfileOnboardingMethodContractV1)
	return contract
}

func (value ProfileOnboardingAuthoritySupportV1) MethodDescriptionEdition() projectprofile.ProfileOnboardingMethodDescriptionEdition {
	return value.methodDescription
}

func (value ProfileOnboardingAuthoritySupportV1) MethodContractEdition() projectprofile.ProfileOnboardingMethodContractEdition {
	return value.methodContract
}

func (value ProfileOnboardingAuthoritySupportV1) MethodDescriptionV2() (
	projectprofile.ProfileOnboardingMethodDescriptionV2,
	bool,
) {
	description, ok := value.methodDescription.(projectprofile.ProfileOnboardingMethodDescriptionV2)
	return description, ok
}

func (value ProfileOnboardingAuthoritySupportV1) MethodContractV2() (
	projectprofile.ProfileOnboardingMethodContractV2,
	bool,
) {
	contract, ok := value.methodContract.(projectprofile.ProfileOnboardingMethodContractV2)
	return contract, ok
}

func (value ProfileOnboardingAuthoritySupportV1) SystemAdmission() projectprofile.ProfileOnboardingExecutorSystemAdmissionV1 {
	return value.systemAdmission
}

func (value ProfileOnboardingAuthoritySupportV1) RoleAdmission() projectprofile.ProfileAuthorRoleAdmissionV1 {
	return value.roleAdmission
}

func (value ProfileOnboardingAuthoritySupportV1) AssignmentJustification() projectprofile.ProfileAuthorAssignmentJustificationV1 {
	return value.justification
}

func (value ProfileOnboardingAuthoritySupportV1) AssignmentProvenance() projectprofile.ProfileAuthorAssignmentProvenanceV1 {
	return value.provenance
}

func (value ProfileOnboardingAuthoritySupportV1) RoleAssignment() projectprofile.ProfileAuthorRoleAssignmentV1 {
	return value.roleAssignment
}

func validateProfileOnboardingAuthoritySupportV1(
	value ProfileOnboardingAuthoritySupportV1,
) error {
	carrier, err := projectprofile.CarryProfileAuthorAssignmentSupportV1(
		value.systemAdmission,
		value.roleAdmission,
		value.justification,
		value.provenance,
	)
	if err != nil {
		return fmt.Errorf("carry ProfileAuthor RoleAssignment support: %w", err)
	}
	if err := projectprofile.ValidateProfileAuthorRoleAssignmentV1Support(
		value.roleAssignment,
		carrier,
	); err != nil {
		return fmt.Errorf("validate ProfileAuthor RoleAssignment support: %w", err)
	}
	descriptionRef, descriptionDigest, contractRef, contractDigest, err := profileOnboardingEditionPins(
		value.methodDescription,
		value.methodContract,
	)
	if err != nil {
		return err
	}
	assignment := value.roleAssignment
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: value.systemAdmission.MethodDescriptionRef() == descriptionRef, name: "system-admission MethodDescription ref"},
		{matches: value.systemAdmission.MethodDescriptionDigest() == descriptionDigest, name: "system-admission MethodDescription digest"},
		{matches: value.systemAdmission.MethodContractRef() != nil && value.systemAdmission.MethodContractRef().String() == contractRef, name: "system-admission MethodContract ref"},
		{matches: value.systemAdmission.MethodContractDigest() == contractDigest, name: "system-admission MethodContract digest"},
		{matches: assignment.BoundedContextRef() == value.methodDescription.BoundedContextRef(), name: "RoleAssignment bounded context"},
	}
	invalid := slices.IndexFunc(checks, func(check struct {
		matches bool
		name    string
	}) bool {
		return !check.matches
	})
	if invalid >= 0 {
		return fmt.Errorf("profile-onboarding authority support has mismatched %s", checks[invalid].name)
	}
	return nil
}

func profileOnboardingEditionPins(
	description projectprofile.ProfileOnboardingMethodDescriptionEdition,
	contract projectprofile.ProfileOnboardingMethodContractEdition,
) (
	projectprofile.MethodDescriptionRef,
	projectprofile.ContentDigest,
	string,
	projectprofile.ContentDigest,
	error,
) {
	switch exactDescription := description.(type) {
	case projectprofile.ProfileOnboardingMethodDescriptionV1:
		exactContract, ok := contract.(projectprofile.ProfileOnboardingMethodContractV1)
		if !ok {
			return projectprofile.MethodDescriptionRef{}, projectprofile.ContentDigest{}, "", projectprofile.ContentDigest{}, fmt.Errorf("profile-onboarding method editions differ")
		}
		descriptionDigest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV1(exactDescription)
		if err != nil {
			return projectprofile.MethodDescriptionRef{}, projectprofile.ContentDigest{}, "", projectprofile.ContentDigest{}, err
		}
		contractDigest, err := projectprofile.DigestProfileOnboardingMethodContractV1(exactContract)
		if err != nil {
			return projectprofile.MethodDescriptionRef{}, projectprofile.ContentDigest{}, "", projectprofile.ContentDigest{}, err
		}
		return exactDescription.Ref(), descriptionDigest, exactContract.Ref().String(), contractDigest, nil
	case projectprofile.ProfileOnboardingMethodDescriptionV2:
		exactContract, ok := contract.(projectprofile.ProfileOnboardingMethodContractV2)
		if !ok {
			return projectprofile.MethodDescriptionRef{}, projectprofile.ContentDigest{}, "", projectprofile.ContentDigest{}, fmt.Errorf("profile-onboarding method editions differ")
		}
		descriptionDigest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV2(exactDescription)
		if err != nil {
			return projectprofile.MethodDescriptionRef{}, projectprofile.ContentDigest{}, "", projectprofile.ContentDigest{}, err
		}
		contractDigest, err := projectprofile.DigestProfileOnboardingMethodContractV2(exactContract)
		if err != nil {
			return projectprofile.MethodDescriptionRef{}, projectprofile.ContentDigest{}, "", projectprofile.ContentDigest{}, err
		}
		return exactDescription.Ref(), descriptionDigest, exactContract.Ref().String(), contractDigest, nil
	default:
		return projectprofile.MethodDescriptionRef{}, projectprofile.ContentDigest{}, "", projectprofile.ContentDigest{}, fmt.Errorf("profile-onboarding method edition is absent or unsupported")
	}
}

type profileOnboardingAuthoritySupportRowsV1 struct {
	description methodDescriptionRow
	contract    methodContractRow
	system      systemAdmissionRow
	role        roleAdmissionRow
	support     assignmentSupportRow
	assignment  roleAssignmentRow
}

type durableProfileOnboardingAuthoritySupportV1State struct {
	value ProfileOnboardingAuthoritySupportV1
}

// DurableProfileOnboardingAuthoritySupportV1 is an exact same-transaction
// reread of the pre-Work support closure.
type DurableProfileOnboardingAuthoritySupportV1 struct {
	state *durableProfileOnboardingAuthoritySupportV1State
}

func (snapshot DurableProfileOnboardingAuthoritySupportV1) Value() (
	ProfileOnboardingAuthoritySupportV1,
	bool,
) {
	if snapshot.state == nil {
		return ProfileOnboardingAuthoritySupportV1{}, false
	}
	if err := validateProfileOnboardingAuthoritySupportV1(snapshot.state.value); err != nil {
		return ProfileOnboardingAuthoritySupportV1{}, false
	}
	return snapshot.state.value, true
}

// StoreAndReloadProfileOnboardingAuthoritySupportV1 stores only the support
// that must pre-exist the authorizing SpeechAct. It cannot store or synthesize
// performed Work or a profile candidate.
func StoreAndReloadProfileOnboardingAuthoritySupportV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	value ProfileOnboardingAuthoritySupportV1,
	recordedAt time.Time,
) (DurableProfileOnboardingAuthoritySupportV1, error) {
	if ctx == nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("profile-onboarding authority-support context is required")
	}
	if err := transaction.RequireImmediate(); err != nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("profile-onboarding authority-support transaction is invalid: %w", err)
	}
	rows, err := prepareProfileOnboardingAuthoritySupportRowsV1(value, recordedAt)
	if err != nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, err
	}
	if err := persistProfileOnboardingAuthoritySupportRowsV1(ctx, transaction, rows); err != nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, err
	}
	actualRows, err := loadProfileOnboardingAuthoritySupportRowsV1(
		ctx,
		transaction,
		rows.assignment.ref,
	)
	if err != nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, err
	}
	if !sameProfileOnboardingAuthoritySupportSemanticsV1(rows, actualRows) {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("persisted profile-onboarding authority support differs from requested values")
	}
	actual, err := reconstructProfileOnboardingAuthoritySupportV1(actualRows)
	if err != nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, err
	}
	state := durableProfileOnboardingAuthoritySupportV1State{value: actual}
	snapshot := DurableProfileOnboardingAuthoritySupportV1{state: &state}
	if _, ok := snapshot.Value(); !ok {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("persisted profile-onboarding authority support is unusable")
	}
	return snapshot, nil
}

// ResolveProfileOnboardingAuthoritySupportByAssignmentV1 strictly rebuilds
// the immutable pre-Work support closure from its canonical RoleAssignment.
// It performs no insert and is valid in either a read or immediate transaction.
func ResolveProfileOnboardingAuthoritySupportByAssignmentV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	assignmentRef projectprofile.RoleAssignmentRef,
) (DurableProfileOnboardingAuthoritySupportV1, error) {
	if ctx == nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("profile-onboarding authority-support context is required")
	}
	if transaction == nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("profile-onboarding authority-support transaction is required")
	}
	if err := transaction.RequireActive(); err != nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("profile-onboarding authority-support transaction is invalid: %w", err)
	}
	if assignmentRef.String() == "" {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("profile-onboarding RoleAssignment ref is required")
	}
	rows, err := loadProfileOnboardingAuthoritySupportRowsV1(
		ctx,
		transaction,
		assignmentRef.String(),
	)
	if err != nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, err
	}
	value, err := reconstructProfileOnboardingAuthoritySupportV1(rows)
	if err != nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, err
	}
	state := durableProfileOnboardingAuthoritySupportV1State{value: value}
	snapshot := DurableProfileOnboardingAuthoritySupportV1{state: &state}
	if _, ok := snapshot.Value(); !ok {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("resolved profile-onboarding authority support is unusable")
	}
	return snapshot, nil
}

// ResolveProfileOnboardingAuthoritySupportByAssignmentV2 uses the same
// version-neutral rows but refuses to return historical v1 method pins.
func ResolveProfileOnboardingAuthoritySupportByAssignmentV2(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	assignmentRef projectprofile.RoleAssignmentRef,
) (DurableProfileOnboardingAuthoritySupportV1, error) {
	durable, err := ResolveProfileOnboardingAuthoritySupportByAssignmentV1(
		ctx,
		transaction,
		assignmentRef,
	)
	if err != nil {
		return DurableProfileOnboardingAuthoritySupportV1{}, err
	}
	value, ok := durable.Value()
	if !ok {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("resolved profile-onboarding authority support is unusable")
	}
	if _, ok := value.MethodDescriptionV2(); !ok {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("resolved profile-onboarding authority support is not method edition v2")
	}
	if _, ok := value.MethodContractV2(); !ok {
		return DurableProfileOnboardingAuthoritySupportV1{}, fmt.Errorf("resolved profile-onboarding authority support is not method-contract edition v2")
	}
	return durable, nil
}

func prepareProfileOnboardingAuthoritySupportRowsV1(
	value ProfileOnboardingAuthoritySupportV1,
	recordedAt time.Time,
) (profileOnboardingAuthoritySupportRowsV1, error) {
	if err := validateProfileOnboardingAuthoritySupportV1(value); err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	recordedAtText, err := canonicalTime("recorded_at", recordedAt)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	if recordedAt.Before(value.provenance.RecordedAt()) {
		return profileOnboardingAuthoritySupportRowsV1{}, fmt.Errorf("recorded_at must not precede assignment provenance")
	}
	description, err := prepareMethodDescriptionRow(value.methodDescription, recordedAtText)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	contract, err := prepareMethodContractRow(value.methodContract, recordedAtText)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	system, err := prepareSystemAdmissionRow(value.systemAdmission, recordedAtText)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	role, err := prepareRoleAdmissionRow(value.roleAdmission, recordedAtText)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	support, err := prepareAssignmentSupportRow(value.justification, value.provenance, recordedAtText)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	assignment, err := prepareRoleAssignmentRow(value.roleAssignment, recordedAtText)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	return profileOnboardingAuthoritySupportRowsV1{
		description: description,
		contract:    contract,
		system:      system,
		role:        role,
		support:     support,
		assignment:  assignment,
	}, nil
}

func persistProfileOnboardingAuthoritySupportRowsV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	rows profileOnboardingAuthoritySupportRowsV1,
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
	}
	return visitProfileOnboardingPersistOperationsV1(ctx, transaction, operations)
}

func loadProfileOnboardingAuthoritySupportRowsV1(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	assignmentRef string,
) (profileOnboardingAuthoritySupportRowsV1, error) {
	assignment, err := loadRoleAssignment(ctx, transaction, assignmentRef)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	system, err := loadSystemAdmission(ctx, transaction, assignment.systemAdmissionRef)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	role, err := loadRoleAdmission(ctx, transaction, assignment.roleAdmissionRef)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	support, err := loadAssignmentSupport(ctx, transaction, assignment.justificationRef)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	description, err := loadMethodDescription(ctx, transaction, system.methodDescriptionRef)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	contract, err := loadMethodContract(ctx, transaction, system.methodContractRef)
	if err != nil {
		return profileOnboardingAuthoritySupportRowsV1{}, err
	}
	return profileOnboardingAuthoritySupportRowsV1{
		description: description,
		contract:    contract,
		system:      system,
		role:        role,
		support:     support,
		assignment:  assignment,
	}, nil
}

func reconstructProfileOnboardingAuthoritySupportV1(
	rows profileOnboardingAuthoritySupportRowsV1,
) (ProfileOnboardingAuthoritySupportV1, error) {
	description, err := reconstructMethodDescription(rows.description)
	if err != nil {
		return ProfileOnboardingAuthoritySupportV1{}, err
	}
	contract, err := reconstructMethodContract(rows.contract)
	if err != nil {
		return ProfileOnboardingAuthoritySupportV1{}, err
	}
	system, err := reconstructSystemAdmission(rows.system)
	if err != nil {
		return ProfileOnboardingAuthoritySupportV1{}, err
	}
	role, err := reconstructRoleAdmission(rows.role)
	if err != nil {
		return ProfileOnboardingAuthoritySupportV1{}, err
	}
	justification, provenance, err := reconstructAssignmentSupport(rows.support)
	if err != nil {
		return ProfileOnboardingAuthoritySupportV1{}, err
	}
	assignment, err := reconstructRoleAssignment(rows.assignment)
	if err != nil {
		return ProfileOnboardingAuthoritySupportV1{}, err
	}
	builder := NewProfileOnboardingAuthoritySupportV1Builder(assignment)
	builder, err = withAuthoritySupportMethodEdition(builder, description, contract)
	if err != nil {
		return ProfileOnboardingAuthoritySupportV1{}, err
	}
	builder = builder.WithSystemAdmission(system)
	builder = builder.WithRoleAdmission(role)
	builder = builder.WithAssignmentJustification(justification)
	builder = builder.WithAssignmentProvenance(provenance)
	return builder.Build()
}

func withAuthoritySupportMethodEdition(
	builder ProfileOnboardingAuthoritySupportV1Builder,
	description projectprofile.ProfileOnboardingMethodDescriptionEdition,
	contract projectprofile.ProfileOnboardingMethodContractEdition,
) (ProfileOnboardingAuthoritySupportV1Builder, error) {
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

func sameProfileOnboardingAuthoritySupportSemanticsV1(
	left profileOnboardingAuthoritySupportRowsV1,
	right profileOnboardingAuthoritySupportRowsV1,
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
	return left == right
}
