package projectprofile

import (
	"fmt"
	"time"
)

const (
	profileOnboardingSystemAdmissionPatternRefV1Value = "A.1"
	profileAuthorRoleAdmissionPatternRefV1Value       = "A.2.1"
	profileAuthorAssignmentAdmissionRuleRefV1Value    = "haft:rule:profile-onboarding/profile-author-assignment-admission/v1"
	profileAuthorAssignmentAdmissionRuleV1Statement   = "admit ProfileAuthorRole assignment only when exact executor-system and ProfileAuthor-role admissions bind the pinned ProfileOnboarding MethodDescription and MethodContract in one bounded context, one onboarding session, and a system-admission window that contains the declared assignment window"
	profileOnboardingKernelExecutorIdentityKindV1     = "kernel_owned"
	profileOnboardingOperatorExecutorIdentityKindV1   = "operator_designated"
)

// ProfileOnboardingSystemIdentityBasisRefV1 identifies the exact carrier of
// an operator designation. A ref alone is not an executor-identity basis: the
// closed operator-designated variant below also binds its digest and system.
type ProfileOnboardingSystemIdentityBasisRefV1 struct{ v1Reference }
type ProfileOnboardingSystemActingEligibilityBasisRefV1 struct{ v1Reference }

func NewProfileOnboardingSystemIdentityBasisRefV1(
	raw string,
) (ProfileOnboardingSystemIdentityBasisRefV1, error) {
	ref, err := newV1Reference("profile-onboarding system identity-basis ref", raw)
	return ProfileOnboardingSystemIdentityBasisRefV1{v1Reference: ref}, err
}

func NewProfileOnboardingSystemActingEligibilityBasisRefV1(
	raw string,
) (ProfileOnboardingSystemActingEligibilityBasisRefV1, error) {
	ref, err := newV1Reference("profile-onboarding system acting-eligibility-basis ref", raw)
	return ProfileOnboardingSystemActingEligibilityBasisRefV1{v1Reference: ref}, err
}

// ProfileOnboardingExecutorAdmissionWindowV1 is the interval in which the
// exact executor admission may support a RoleAssignment. It is intentionally
// distinct from the assignment window it must contain.
type ProfileOnboardingExecutorAdmissionWindowV1 struct{ closedIntervalV1 }

func NewProfileOnboardingExecutorAdmissionWindowV1(
	from time.Time,
	until time.Time,
) (ProfileOnboardingExecutorAdmissionWindowV1, error) {
	interval, err := newClosedIntervalV1("profile-onboarding executor-admission window", from, until)
	return ProfileOnboardingExecutorAdmissionWindowV1{closedIntervalV1: interval}, err
}

func (window ProfileOnboardingExecutorAdmissionWindowV1) From() time.Time {
	return window.from
}

func (window ProfileOnboardingExecutorAdmissionWindowV1) Until() time.Time {
	return window.until
}

func (window ProfileOnboardingExecutorAdmissionWindowV1) valid() bool {
	return window.closedIntervalV1.valid()
}

func (window ProfileOnboardingExecutorAdmissionWindowV1) coversRoleAssignment(
	assignment RoleAssignmentWindowV1,
) bool {
	return window.contains(assignment.closedIntervalV1)
}

func (window ProfileOnboardingExecutorAdmissionWindowV1) CoversWork(
	work WorkIntervalV1,
) bool {
	return window.contains(work.closedIntervalV1)
}

// ProfileOnboardingExecutorIdentityBasisKindV1 is the closed discriminant for
// the two identity bases admitted by the final-v1 method-local contract.
type ProfileOnboardingExecutorIdentityBasisKindV1 struct{ value string }

func (kind ProfileOnboardingExecutorIdentityBasisKindV1) String() string {
	return kind.value
}

type profileOnboardingKernelExecutorIdentityBasisV1 struct {
	systemRef SystemRef
	kernel    ProfileOnboardingKernelIdentityV1
}

type profileOnboardingOperatorExecutorIdentityBasisV1 struct {
	systemRef         SystemRef
	designationRef    ProfileOnboardingSystemIdentityBasisRefV1
	designationDigest ContentDigest
}

// ProfileOnboardingExecutorIdentityBasisV1 is a sealed sum. Its zero value and
// mixed variants are invalid. Generic caller refs cannot inhabit the
// kernel-owned branch, and a designation ref cannot inhabit the
// operator-designated branch without an exact digest and system binding.
type ProfileOnboardingExecutorIdentityBasisV1 struct {
	kernel   *profileOnboardingKernelExecutorIdentityBasisV1
	operator *profileOnboardingOperatorExecutorIdentityBasisV1
}

// Constructors below parse the exact identity basis selected by the caller;
// they do not attest that a build is the running build or that an operator
// issued a designation. The effect boundary must recover that fact from the
// kernel-owned runtime or the exact designated carrier before constructing an
// admission. Keeping that check outside this value avoids importing authority
// presentation semantics into the project-memory algebra.

func NewProfileOnboardingKernelExecutorIdentityBasisV1(
	systemRef SystemRef,
	kernel ProfileOnboardingKernelIdentityV1,
) (ProfileOnboardingExecutorIdentityBasisV1, error) {
	value := ProfileOnboardingExecutorIdentityBasisV1{
		kernel: &profileOnboardingKernelExecutorIdentityBasisV1{
			systemRef: systemRef,
			kernel:    kernel,
		},
	}
	return canonicalProfileOnboardingExecutorIdentityBasisV1(value)
}

func NewProfileOnboardingOperatorDesignatedExecutorIdentityBasisV1(
	systemRef SystemRef,
	designationRef ProfileOnboardingSystemIdentityBasisRefV1,
	designationDigest ContentDigest,
) (ProfileOnboardingExecutorIdentityBasisV1, error) {
	value := ProfileOnboardingExecutorIdentityBasisV1{
		operator: &profileOnboardingOperatorExecutorIdentityBasisV1{
			systemRef:         systemRef,
			designationRef:    designationRef,
			designationDigest: designationDigest,
		},
	}
	return canonicalProfileOnboardingExecutorIdentityBasisV1(value)
}

func (basis ProfileOnboardingExecutorIdentityBasisV1) Kind() ProfileOnboardingExecutorIdentityBasisKindV1 {
	kernelValid := validKernelExecutorIdentityBasisV1(basis.kernel)
	operatorValid := validOperatorExecutorIdentityBasisV1(basis.operator)
	kernelOnly := kernelValid && basis.operator == nil
	operatorOnly := operatorValid && basis.kernel == nil
	if kernelOnly {
		return ProfileOnboardingExecutorIdentityBasisKindV1{value: profileOnboardingKernelExecutorIdentityKindV1}
	}
	if operatorOnly {
		return ProfileOnboardingExecutorIdentityBasisKindV1{value: profileOnboardingOperatorExecutorIdentityKindV1}
	}
	return ProfileOnboardingExecutorIdentityBasisKindV1{}
}

func validKernelExecutorIdentityBasisV1(
	value *profileOnboardingKernelExecutorIdentityBasisV1,
) bool {
	if value == nil {
		return false
	}
	systemValid := value.systemRef.valid()
	kernelValid := value.kernel.valid()
	return systemValid && kernelValid
}

func validOperatorExecutorIdentityBasisV1(
	value *profileOnboardingOperatorExecutorIdentityBasisV1,
) bool {
	if value == nil {
		return false
	}
	systemValid := value.systemRef.valid()
	refValid := value.designationRef.valid()
	digestValid := value.designationDigest.valid()
	return systemValid && refValid && digestValid
}

func (basis ProfileOnboardingExecutorIdentityBasisV1) SystemRef() SystemRef {
	if basis.kernel != nil && basis.operator == nil {
		return basis.kernel.systemRef
	}
	if basis.operator != nil && basis.kernel == nil {
		return basis.operator.systemRef
	}
	return SystemRef{}
}

func (basis ProfileOnboardingExecutorIdentityBasisV1) KernelIdentity() (
	ProfileOnboardingKernelIdentityV1,
	bool,
) {
	kind := basis.Kind()
	kindValue := kind.String()
	if kindValue != profileOnboardingKernelExecutorIdentityKindV1 {
		return ProfileOnboardingKernelIdentityV1{}, false
	}
	return basis.kernel.kernel, true
}

func (basis ProfileOnboardingExecutorIdentityBasisV1) OperatorDesignation() (
	ProfileOnboardingSystemIdentityBasisRefV1,
	ContentDigest,
	bool,
) {
	kind := basis.Kind()
	kindValue := kind.String()
	if kindValue != profileOnboardingOperatorExecutorIdentityKindV1 {
		return ProfileOnboardingSystemIdentityBasisRefV1{}, ContentDigest{}, false
	}
	return basis.operator.designationRef, basis.operator.designationDigest, true
}

func canonicalProfileOnboardingExecutorIdentityBasisV1(
	basis ProfileOnboardingExecutorIdentityBasisV1,
) (ProfileOnboardingExecutorIdentityBasisV1, error) {
	systemRef := basis.SystemRef()
	systemValid := systemRef.valid()
	if !systemValid {
		return ProfileOnboardingExecutorIdentityBasisV1{}, fmt.Errorf(
			"executor identity basis is underdetermined: require one exact kernel-owned identity plus build version or one operator-designated ref plus digest",
		)
	}
	kind := basis.Kind()
	kindValue := kind.String()
	if kindValue == "" {
		return ProfileOnboardingExecutorIdentityBasisV1{}, fmt.Errorf(
			"executor identity basis must contain exactly one closed variant",
		)
	}
	return basis, nil
}

type profileOnboardingMethodPinsV1 struct {
	edition               string
	descriptionRef        MethodDescriptionRef
	descriptionDigest     ContentDigest
	contractRef           ProfileOnboardingMethodContractRef
	contractDigest        ContentDigest
	boundedContextRef     BoundedContextRef
	requiredRoleRef       RoleRef
	requiredSystemKind    ProfileOnboardingSystemKindV1
	roleAdmissionPolicy   ProfileOnboardingAdmissionPolicyRefV1
	systemAdmissionPolicy ProfileOnboardingAdmissionPolicyRefV1
}

func loadProfileOnboardingMethodPinsV1() (profileOnboardingMethodPinsV1, error) {
	description := ProfileOnboardingMethodDescriptionV1Value()
	descriptionDigest, err := DigestProfileOnboardingMethodDescriptionV1(description)
	if err != nil {
		return profileOnboardingMethodPinsV1{}, err
	}
	contract, err := ProfileOnboardingMethodContractV1Value()
	if err != nil {
		return profileOnboardingMethodPinsV1{}, err
	}
	contractDigest, err := DigestProfileOnboardingMethodContractV1(contract)
	if err != nil {
		return profileOnboardingMethodPinsV1{}, err
	}
	descriptionRef := description.Ref()
	contractRef := contract.Ref()
	boundedContextRef := description.BoundedContextRef()
	requiredRoleRef := description.RequiredRoleRef()
	requiredSystemKind := description.RequiredSystemKind()
	roleAdmissionPolicy := contract.RoleAdmissionPolicyRef()
	systemAdmissionPolicy := contract.SystemAdmissionPolicyRef()
	return profileOnboardingMethodPinsV1{
		edition:               profileOnboardingMethodEditionV1Value,
		descriptionRef:        descriptionRef,
		descriptionDigest:     descriptionDigest,
		contractRef:           contractRef,
		contractDigest:        contractDigest,
		boundedContextRef:     boundedContextRef,
		requiredRoleRef:       requiredRoleRef,
		requiredSystemKind:    requiredSystemKind,
		roleAdmissionPolicy:   roleAdmissionPolicy,
		systemAdmissionPolicy: systemAdmissionPolicy,
	}, nil
}

func loadProfileOnboardingMethodPinsV2() (profileOnboardingMethodPinsV1, error) {
	description := ProfileOnboardingMethodDescriptionV2Value()
	descriptionDigest, err := DigestProfileOnboardingMethodDescriptionV2(description)
	if err != nil {
		return profileOnboardingMethodPinsV1{}, err
	}
	contract, err := ProfileOnboardingMethodContractV2Value()
	if err != nil {
		return profileOnboardingMethodPinsV1{}, err
	}
	contractDigest, err := DigestProfileOnboardingMethodContractV2(contract)
	if err != nil {
		return profileOnboardingMethodPinsV1{}, err
	}
	return profileOnboardingMethodPinsV1{
		edition:               profileOnboardingMethodEditionV2Value,
		descriptionRef:        description.Ref(),
		descriptionDigest:     descriptionDigest,
		contractRef:           contract.Ref(),
		contractDigest:        contractDigest,
		boundedContextRef:     description.BoundedContextRef(),
		requiredRoleRef:       description.RequiredRoleRef(),
		requiredSystemKind:    description.RequiredSystemKind(),
		roleAdmissionPolicy:   contract.RoleAdmissionPolicyRef(),
		systemAdmissionPolicy: contract.SystemAdmissionPolicyRef(),
	}, nil
}

func loadProfileOnboardingMethodPinsForRefs(
	descriptionRef MethodDescriptionRef,
	contractRef ProfileOnboardingMethodContractRef,
) (profileOnboardingMethodPinsV1, error) {
	if contractRef == nil {
		return profileOnboardingMethodPinsV1{}, fmt.Errorf("profile-onboarding MethodContract ref is required")
	}
	v1, err := loadProfileOnboardingMethodPinsV1()
	if err != nil {
		return profileOnboardingMethodPinsV1{}, err
	}
	if descriptionRef == v1.descriptionRef && contractRef.String() == v1.contractRef.String() {
		return v1, nil
	}
	v2, err := loadProfileOnboardingMethodPinsV2()
	if err != nil {
		return profileOnboardingMethodPinsV1{}, err
	}
	if descriptionRef == v2.descriptionRef && contractRef.String() == v2.contractRef.String() {
		return v2, nil
	}
	return profileOnboardingMethodPinsV1{}, fmt.Errorf("method pins do not form an exact supported profile-onboarding v1 or v2 edition")
}

func loadProfileOnboardingMethodPinsForContractRef(
	contractRef ProfileOnboardingMethodContractRef,
) (profileOnboardingMethodPinsV1, error) {
	if contractRef == nil {
		return profileOnboardingMethodPinsV1{}, fmt.Errorf("profile-onboarding MethodContract ref is required")
	}
	v1, err := loadProfileOnboardingMethodPinsV1()
	if err != nil {
		return profileOnboardingMethodPinsV1{}, err
	}
	if contractRef.String() == v1.contractRef.String() {
		return v1, nil
	}
	v2, err := loadProfileOnboardingMethodPinsV2()
	if err != nil {
		return profileOnboardingMethodPinsV1{}, err
	}
	if contractRef.String() == v2.contractRef.String() {
		return v2, nil
	}
	return profileOnboardingMethodPinsV1{}, fmt.Errorf("unknown profile-onboarding MethodContract edition")
}

// ProfileOnboardingExecutorSystemAdmissionV1 is a method-local A.1 admission
// of one concrete holder as U.System for ProfileOnboarding execution. It does
// not turn an identity record, method description, contract, or policy into an
// acting system. Identity and acting eligibility stay separately bound.
type ProfileOnboardingExecutorSystemAdmissionV1 struct {
	ref                          SystemAdmissionRef
	systemRef                    SystemRef
	admittedSystemKind           ProfileOnboardingSystemKindV1
	boundedContextRef            BoundedContextRef
	governingPatternRef          SourceUnitRef
	identityBasis                ProfileOnboardingExecutorIdentityBasisV1
	actingEligibilityBasisRef    ProfileOnboardingSystemActingEligibilityBasisRefV1
	actingEligibilityBasisDigest ContentDigest
	sessionRef                   SessionRef
	validityWindow               ProfileOnboardingExecutorAdmissionWindowV1
	methodDescriptionRef         MethodDescriptionRef
	methodDescriptionDigest      ContentDigest
	methodContractRef            ProfileOnboardingMethodContractRef
	methodContractDigest         ContentDigest
	systemAdmissionPolicyRef     ProfileOnboardingAdmissionPolicyRefV1
}

type ProfileOnboardingExecutorSystemAdmissionV1Builder struct {
	ref                          SystemAdmissionRef
	systemRef                    SystemRef
	identityBasis                ProfileOnboardingExecutorIdentityBasisV1
	actingEligibilityBasisRef    ProfileOnboardingSystemActingEligibilityBasisRefV1
	actingEligibilityBasisDigest ContentDigest
	sessionRef                   SessionRef
	validityWindow               ProfileOnboardingExecutorAdmissionWindowV1
	methodEdition                string
}

func NewProfileOnboardingExecutorSystemAdmissionV1Builder(
	ref SystemAdmissionRef,
	systemRef SystemRef,
) ProfileOnboardingExecutorSystemAdmissionV1Builder {
	return ProfileOnboardingExecutorSystemAdmissionV1Builder{
		ref:       ref,
		systemRef: systemRef,
	}
}

func (builder ProfileOnboardingExecutorSystemAdmissionV1Builder) IdentifiedBy(
	basis ProfileOnboardingExecutorIdentityBasisV1,
) ProfileOnboardingExecutorSystemAdmissionV1Builder {
	builder.identityBasis = basis
	return builder
}

func (builder ProfileOnboardingExecutorSystemAdmissionV1Builder) AdmittedToActBy(
	ref ProfileOnboardingSystemActingEligibilityBasisRefV1,
	digest ContentDigest,
) ProfileOnboardingExecutorSystemAdmissionV1Builder {
	builder.actingEligibilityBasisRef = ref
	builder.actingEligibilityBasisDigest = digest
	return builder
}

func (builder ProfileOnboardingExecutorSystemAdmissionV1Builder) InSession(
	ref SessionRef,
) ProfileOnboardingExecutorSystemAdmissionV1Builder {
	builder.sessionRef = ref
	return builder
}

func (builder ProfileOnboardingExecutorSystemAdmissionV1Builder) ValidDuring(
	window ProfileOnboardingExecutorAdmissionWindowV1,
) ProfileOnboardingExecutorSystemAdmissionV1Builder {
	builder.validityWindow = window
	return builder
}

func (builder ProfileOnboardingExecutorSystemAdmissionV1Builder) UsingMethodEditionV2() ProfileOnboardingExecutorSystemAdmissionV1Builder {
	builder.methodEdition = profileOnboardingMethodEditionV2Value
	return builder
}

func (builder ProfileOnboardingExecutorSystemAdmissionV1Builder) Build() (
	ProfileOnboardingExecutorSystemAdmissionV1,
	error,
) {
	pins, err := loadProfileOnboardingMethodPinsV1()
	if builder.methodEdition == profileOnboardingMethodEditionV2Value {
		pins, err = loadProfileOnboardingMethodPinsV2()
	}
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	value := ProfileOnboardingExecutorSystemAdmissionV1{
		ref:                          builder.ref,
		systemRef:                    builder.systemRef,
		admittedSystemKind:           pins.requiredSystemKind,
		boundedContextRef:            pins.boundedContextRef,
		governingPatternRef:          SourceUnitRef{value: profileOnboardingSystemAdmissionPatternRefV1Value},
		identityBasis:                builder.identityBasis,
		actingEligibilityBasisRef:    builder.actingEligibilityBasisRef,
		actingEligibilityBasisDigest: builder.actingEligibilityBasisDigest,
		sessionRef:                   builder.sessionRef,
		validityWindow:               builder.validityWindow,
		methodDescriptionRef:         pins.descriptionRef,
		methodDescriptionDigest:      pins.descriptionDigest,
		methodContractRef:            pins.contractRef,
		methodContractDigest:         pins.contractDigest,
		systemAdmissionPolicyRef:     pins.systemAdmissionPolicy,
	}
	return canonicalProfileOnboardingExecutorSystemAdmissionV1(value)
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) Ref() SystemAdmissionRef {
	return value.ref
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) SystemRef() SystemRef {
	return value.systemRef
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) AdmittedSystemKind() ProfileOnboardingSystemKindV1 {
	return value.admittedSystemKind
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) BoundedContextRef() BoundedContextRef {
	return value.boundedContextRef
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) GoverningPatternRef() SourceUnitRef {
	return value.governingPatternRef
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) IdentityBasis() ProfileOnboardingExecutorIdentityBasisV1 {
	return value.identityBasis
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) ActingEligibilityBasisRef() ProfileOnboardingSystemActingEligibilityBasisRefV1 {
	return value.actingEligibilityBasisRef
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) ActingEligibilityBasisDigest() ContentDigest {
	return value.actingEligibilityBasisDigest
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) SessionRef() SessionRef {
	return value.sessionRef
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) ValidityWindow() ProfileOnboardingExecutorAdmissionWindowV1 {
	return value.validityWindow
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) MethodDescriptionRef() MethodDescriptionRef {
	return value.methodDescriptionRef
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) MethodDescriptionDigest() ContentDigest {
	return value.methodDescriptionDigest
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) MethodContractRef() ProfileOnboardingMethodContractRef {
	return value.methodContractRef
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) MethodContractDigest() ContentDigest {
	return value.methodContractDigest
}

func (value ProfileOnboardingExecutorSystemAdmissionV1) AdmissionPolicyRef() ProfileOnboardingAdmissionPolicyRefV1 {
	return value.systemAdmissionPolicyRef
}

// ProfileAuthorRoleAdmissionV1 admits the fixed ProfileAuthor U.Role for the
// exact pinned ProfileOnboarding MethodDescription and MethodContract. It is a
// role-value admission, not a holder assignment or a performed occurrence.
type ProfileAuthorRoleAdmissionV1 struct {
	ref                     RoleAdmissionRef
	roleRef                 RoleRef
	boundedContextRef       BoundedContextRef
	governingPatternRef     SourceUnitRef
	methodDescriptionRef    MethodDescriptionRef
	methodDescriptionDigest ContentDigest
	methodContractRef       ProfileOnboardingMethodContractRef
	methodContractDigest    ContentDigest
	roleAdmissionPolicyRef  ProfileOnboardingAdmissionPolicyRefV1
}

func NewProfileAuthorRoleAdmissionV1(
	ref RoleAdmissionRef,
) (ProfileAuthorRoleAdmissionV1, error) {
	pins, err := loadProfileOnboardingMethodPinsV1()
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	value := ProfileAuthorRoleAdmissionV1{
		ref:                     ref,
		roleRef:                 pins.requiredRoleRef,
		boundedContextRef:       pins.boundedContextRef,
		governingPatternRef:     SourceUnitRef{value: profileAuthorRoleAdmissionPatternRefV1Value},
		methodDescriptionRef:    pins.descriptionRef,
		methodDescriptionDigest: pins.descriptionDigest,
		methodContractRef:       pins.contractRef,
		methodContractDigest:    pins.contractDigest,
		roleAdmissionPolicyRef:  pins.roleAdmissionPolicy,
	}
	return canonicalProfileAuthorRoleAdmissionV1(value)
}

func NewProfileAuthorRoleAdmissionV2(
	ref RoleAdmissionRef,
) (ProfileAuthorRoleAdmissionV1, error) {
	pins, err := loadProfileOnboardingMethodPinsV2()
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	value := ProfileAuthorRoleAdmissionV1{
		ref:                     ref,
		roleRef:                 pins.requiredRoleRef,
		boundedContextRef:       pins.boundedContextRef,
		governingPatternRef:     SourceUnitRef{value: profileAuthorRoleAdmissionPatternRefV1Value},
		methodDescriptionRef:    pins.descriptionRef,
		methodDescriptionDigest: pins.descriptionDigest,
		methodContractRef:       pins.contractRef,
		methodContractDigest:    pins.contractDigest,
		roleAdmissionPolicyRef:  pins.roleAdmissionPolicy,
	}
	return canonicalProfileAuthorRoleAdmissionV1(value)
}

func (value ProfileAuthorRoleAdmissionV1) Ref() RoleAdmissionRef {
	return value.ref
}

func (value ProfileAuthorRoleAdmissionV1) RoleRef() RoleRef {
	return value.roleRef
}

func (value ProfileAuthorRoleAdmissionV1) BoundedContextRef() BoundedContextRef {
	return value.boundedContextRef
}

func (value ProfileAuthorRoleAdmissionV1) GoverningPatternRef() SourceUnitRef {
	return value.governingPatternRef
}

func (value ProfileAuthorRoleAdmissionV1) MethodDescriptionRef() MethodDescriptionRef {
	return value.methodDescriptionRef
}

func (value ProfileAuthorRoleAdmissionV1) MethodDescriptionDigest() ContentDigest {
	return value.methodDescriptionDigest
}

func (value ProfileAuthorRoleAdmissionV1) MethodContractRef() ProfileOnboardingMethodContractRef {
	return value.methodContractRef
}

func (value ProfileAuthorRoleAdmissionV1) MethodContractDigest() ContentDigest {
	return value.methodContractDigest
}

func (value ProfileAuthorRoleAdmissionV1) AdmissionPolicyRef() ProfileOnboardingAdmissionPolicyRefV1 {
	return value.roleAdmissionPolicyRef
}

type ProfileAuthorAssignmentAdmissionRuleV1 struct {
	ref       ProfileOnboardingLocalRuleRefV1
	statement string
}

func (rule ProfileAuthorAssignmentAdmissionRuleV1) Ref() ProfileOnboardingLocalRuleRefV1 {
	return rule.ref
}

func (rule ProfileAuthorAssignmentAdmissionRuleV1) Statement() string {
	return rule.statement
}

func profileAuthorAssignmentAdmissionRuleV1() ProfileAuthorAssignmentAdmissionRuleV1 {
	return ProfileAuthorAssignmentAdmissionRuleV1{
		ref:       ProfileOnboardingLocalRuleRefV1{value: profileAuthorAssignmentAdmissionRuleRefV1Value},
		statement: profileAuthorAssignmentAdmissionRuleV1Statement,
	}
}

// ProfileAuthorAssignmentJustificationV1 applies the fixed local admission
// rule to exact system and role admissions for one assignment window. It does
// not cite a RoleAssignment, so the assignment can cite this justification
// without a provenance cycle.
type ProfileAuthorAssignmentJustificationV1 struct {
	ref                   RoleAssignmentJustificationRef
	rule                  ProfileAuthorAssignmentAdmissionRuleV1
	boundedContextRef     BoundedContextRef
	systemAdmissionRef    SystemAdmissionRef
	systemAdmissionDigest ContentDigest
	roleAdmissionRef      RoleAdmissionRef
	roleAdmissionDigest   ContentDigest
	assignmentWindow      RoleAssignmentWindowV1
	methodContractRef     ProfileOnboardingMethodContractRef
	methodContractDigest  ContentDigest
}

type ProfileAuthorAssignmentJustificationV1Builder struct {
	ref              RoleAssignmentJustificationRef
	systemAdmission  ProfileOnboardingExecutorSystemAdmissionV1
	roleAdmission    ProfileAuthorRoleAdmissionV1
	assignmentWindow RoleAssignmentWindowV1
}

func NewProfileAuthorAssignmentJustificationV1Builder(
	ref RoleAssignmentJustificationRef,
) ProfileAuthorAssignmentJustificationV1Builder {
	return ProfileAuthorAssignmentJustificationV1Builder{ref: ref}
}

func (builder ProfileAuthorAssignmentJustificationV1Builder) ApplyingAdmissions(
	systemAdmission ProfileOnboardingExecutorSystemAdmissionV1,
	roleAdmission ProfileAuthorRoleAdmissionV1,
) ProfileAuthorAssignmentJustificationV1Builder {
	builder.systemAdmission = systemAdmission
	builder.roleAdmission = roleAdmission
	return builder
}

func (builder ProfileAuthorAssignmentJustificationV1Builder) ValidDuring(
	window RoleAssignmentWindowV1,
) ProfileAuthorAssignmentJustificationV1Builder {
	builder.assignmentWindow = window
	return builder
}

func (builder ProfileAuthorAssignmentJustificationV1Builder) Build() (
	ProfileAuthorAssignmentJustificationV1,
	error,
) {
	systemAdmission, err := canonicalProfileOnboardingExecutorSystemAdmissionV1(builder.systemAdmission)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	roleAdmission, err := canonicalProfileAuthorRoleAdmissionV1(builder.roleAdmission)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	if !systemAdmission.validityWindow.coversRoleAssignment(builder.assignmentWindow) {
		return ProfileAuthorAssignmentJustificationV1{}, fmt.Errorf(
			"executor-system admission window must contain the ProfileAuthor assignment window",
		)
	}
	if systemAdmission.boundedContextRef != roleAdmission.boundedContextRef {
		return ProfileAuthorAssignmentJustificationV1{}, fmt.Errorf(
			"executor-system and ProfileAuthor role admissions must share one bounded context",
		)
	}
	systemDigest, err := DigestProfileOnboardingExecutorSystemAdmissionV1(systemAdmission)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	roleDigest, err := DigestProfileAuthorRoleAdmissionV1(roleAdmission)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	rule := profileAuthorAssignmentAdmissionRuleV1()
	value := ProfileAuthorAssignmentJustificationV1{
		ref:                   builder.ref,
		rule:                  rule,
		boundedContextRef:     systemAdmission.boundedContextRef,
		systemAdmissionRef:    systemAdmission.ref,
		systemAdmissionDigest: systemDigest,
		roleAdmissionRef:      roleAdmission.ref,
		roleAdmissionDigest:   roleDigest,
		assignmentWindow:      builder.assignmentWindow,
		methodContractRef:     systemAdmission.methodContractRef,
		methodContractDigest:  systemAdmission.methodContractDigest,
	}
	return canonicalProfileAuthorAssignmentJustificationV1(value)
}

func (value ProfileAuthorAssignmentJustificationV1) Ref() RoleAssignmentJustificationRef {
	return value.ref
}

func (value ProfileAuthorAssignmentJustificationV1) Rule() ProfileAuthorAssignmentAdmissionRuleV1 {
	return value.rule
}

func (value ProfileAuthorAssignmentJustificationV1) BoundedContextRef() BoundedContextRef {
	return value.boundedContextRef
}

func (value ProfileAuthorAssignmentJustificationV1) SystemAdmissionRef() SystemAdmissionRef {
	return value.systemAdmissionRef
}

func (value ProfileAuthorAssignmentJustificationV1) SystemAdmissionDigest() ContentDigest {
	return value.systemAdmissionDigest
}

func (value ProfileAuthorAssignmentJustificationV1) RoleAdmissionRef() RoleAdmissionRef {
	return value.roleAdmissionRef
}

func (value ProfileAuthorAssignmentJustificationV1) RoleAdmissionDigest() ContentDigest {
	return value.roleAdmissionDigest
}

func (value ProfileAuthorAssignmentJustificationV1) AssignmentWindow() RoleAssignmentWindowV1 {
	return value.assignmentWindow
}

func (value ProfileAuthorAssignmentJustificationV1) MethodContractRef() ProfileOnboardingMethodContractRef {
	return value.methodContractRef
}

func (value ProfileAuthorAssignmentJustificationV1) MethodContractDigest() ContentDigest {
	return value.methodContractDigest
}

type ProfileOnboardingKernelIdentityV1 struct {
	identity string
	version  string
}

func NewProfileOnboardingKernelIdentityV1(
	identity string,
	version string,
) (ProfileOnboardingKernelIdentityV1, error) {
	parsedIdentity, err := requireText("profile-onboarding kernel identity", identity)
	if err != nil {
		return ProfileOnboardingKernelIdentityV1{}, err
	}
	parsedVersion, err := requireText("profile-onboarding kernel version", version)
	if err != nil {
		return ProfileOnboardingKernelIdentityV1{}, err
	}
	return ProfileOnboardingKernelIdentityV1{
		identity: parsedIdentity,
		version:  parsedVersion,
	}, nil
}

func (value ProfileOnboardingKernelIdentityV1) Identity() string {
	return value.identity
}

func (value ProfileOnboardingKernelIdentityV1) Version() string {
	return value.version
}

func (value ProfileOnboardingKernelIdentityV1) valid() bool {
	_, identityErr := requireText("profile-onboarding kernel identity", value.identity)
	_, versionErr := requireText("profile-onboarding kernel version", value.version)
	return identityErr == nil && versionErr == nil
}

type ProfileOnboardingRuntimeIdentityV1 struct {
	identity string
	version  string
}

func NewProfileOnboardingRuntimeIdentityV1(
	identity string,
	version string,
) (ProfileOnboardingRuntimeIdentityV1, error) {
	parsedIdentity, err := requireText("profile-onboarding runtime identity", identity)
	if err != nil {
		return ProfileOnboardingRuntimeIdentityV1{}, err
	}
	parsedVersion, err := requireText("profile-onboarding runtime version", version)
	if err != nil {
		return ProfileOnboardingRuntimeIdentityV1{}, err
	}
	return ProfileOnboardingRuntimeIdentityV1{
		identity: parsedIdentity,
		version:  parsedVersion,
	}, nil
}

func (value ProfileOnboardingRuntimeIdentityV1) Identity() string {
	return value.identity
}

func (value ProfileOnboardingRuntimeIdentityV1) Version() string {
	return value.version
}

func (value ProfileOnboardingRuntimeIdentityV1) valid() bool {
	_, identityErr := requireText("profile-onboarding runtime identity", value.identity)
	_, versionErr := requireText("profile-onboarding runtime version", value.version)
	return identityErr == nil && versionErr == nil
}

// ProfileAuthorAssignmentProvenanceV1 records method-local origin metadata for
// the justification carrier. It is not an A.10 evidence graph, does not prove
// that the named runtime performed Work, and does not establish currentness.
// It intentionally has no RoleAssignment field: the origin record supports
// construction of the assignment rather than depending on that assignment.
type ProfileAuthorAssignmentProvenanceV1 struct {
	ref                 RoleAssignmentProvenanceRef
	justificationRef    RoleAssignmentJustificationRef
	justificationDigest ContentDigest
	sessionRef          SessionRef
	kernel              ProfileOnboardingKernelIdentityV1
	runtime             ProfileOnboardingRuntimeIdentityV1
	recordedAt          time.Time
}

type ProfileAuthorAssignmentProvenanceV1Builder struct {
	ref           RoleAssignmentProvenanceRef
	justification ProfileAuthorAssignmentJustificationV1
	sessionRef    SessionRef
	kernel        ProfileOnboardingKernelIdentityV1
	runtime       ProfileOnboardingRuntimeIdentityV1
	recordedAt    time.Time
}

func NewProfileAuthorAssignmentProvenanceV1Builder(
	ref RoleAssignmentProvenanceRef,
	justification ProfileAuthorAssignmentJustificationV1,
) ProfileAuthorAssignmentProvenanceV1Builder {
	return ProfileAuthorAssignmentProvenanceV1Builder{
		ref:           ref,
		justification: justification,
	}
}

func (builder ProfileAuthorAssignmentProvenanceV1Builder) InSession(
	ref SessionRef,
) ProfileAuthorAssignmentProvenanceV1Builder {
	builder.sessionRef = ref
	return builder
}

func (builder ProfileAuthorAssignmentProvenanceV1Builder) ProducedBy(
	kernel ProfileOnboardingKernelIdentityV1,
	runtime ProfileOnboardingRuntimeIdentityV1,
) ProfileAuthorAssignmentProvenanceV1Builder {
	builder.kernel = kernel
	builder.runtime = runtime
	return builder
}

func (builder ProfileAuthorAssignmentProvenanceV1Builder) RecordedAt(
	value time.Time,
) ProfileAuthorAssignmentProvenanceV1Builder {
	builder.recordedAt = value.UTC()
	return builder
}

func (builder ProfileAuthorAssignmentProvenanceV1Builder) Build() (
	ProfileAuthorAssignmentProvenanceV1,
	error,
) {
	justification, err := canonicalProfileAuthorAssignmentJustificationV1(builder.justification)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	justificationDigest, err := DigestProfileAuthorAssignmentJustificationV1(justification)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	value := ProfileAuthorAssignmentProvenanceV1{
		ref:                 builder.ref,
		justificationRef:    justification.ref,
		justificationDigest: justificationDigest,
		sessionRef:          builder.sessionRef,
		kernel:              builder.kernel,
		runtime:             builder.runtime,
		recordedAt:          builder.recordedAt,
	}
	return canonicalProfileAuthorAssignmentProvenanceV1(value)
}

func (value ProfileAuthorAssignmentProvenanceV1) Ref() RoleAssignmentProvenanceRef {
	return value.ref
}

func (value ProfileAuthorAssignmentProvenanceV1) JustificationRef() RoleAssignmentJustificationRef {
	return value.justificationRef
}

func (value ProfileAuthorAssignmentProvenanceV1) JustificationDigest() ContentDigest {
	return value.justificationDigest
}

func (value ProfileAuthorAssignmentProvenanceV1) SessionRef() SessionRef {
	return value.sessionRef
}

func (value ProfileAuthorAssignmentProvenanceV1) Kernel() ProfileOnboardingKernelIdentityV1 {
	return value.kernel
}

func (value ProfileAuthorAssignmentProvenanceV1) Runtime() ProfileOnboardingRuntimeIdentityV1 {
	return value.runtime
}

func (value ProfileAuthorAssignmentProvenanceV1) RecordedAt() time.Time {
	return value.recordedAt
}

type profileAuthorAssignmentSupportCheckV1 struct {
	valid  bool
	reason string
}

func validateProfileAuthorAssignmentSupportCheckV1(
	_ int,
	check profileAuthorAssignmentSupportCheckV1,
) error {
	if !check.valid {
		return fmt.Errorf("profile-author assignment support is invalid: %s", check.reason)
	}
	return nil
}

func canonicalProfileOnboardingExecutorSystemAdmissionV1(
	value ProfileOnboardingExecutorSystemAdmissionV1,
) (ProfileOnboardingExecutorSystemAdmissionV1, error) {
	pins, err := loadProfileOnboardingMethodPinsForRefs(
		value.methodDescriptionRef,
		value.methodContractRef,
	)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	expectedPattern := SourceUnitRef{value: profileOnboardingSystemAdmissionPatternRefV1Value}
	identityBasis, identityBasisErr := canonicalProfileOnboardingExecutorIdentityBasisV1(value.identityBasis)
	refValid := value.ref.valid()
	systemRefValid := value.systemRef.valid()
	identityBasisValid := identityBasisErr == nil
	identitySystemRef := identityBasis.SystemRef()
	identitySystemMatches := identitySystemRef == value.systemRef
	actingRefValid := value.actingEligibilityBasisRef.valid()
	actingDigestValid := value.actingEligibilityBasisDigest.valid()
	sessionValid := value.sessionRef.valid()
	windowValid := value.validityWindow.valid()
	checks := []profileAuthorAssignmentSupportCheckV1{
		{valid: refValid, reason: "system-admission ref is required"},
		{valid: systemRefValid, reason: "executor-system ref is required"},
		{valid: value.admittedSystemKind == pins.requiredSystemKind, reason: "executor admission must admit the pinned U.System kind"},
		{valid: value.boundedContextRef == pins.boundedContextRef, reason: "executor admission must use the pinned bounded context"},
		{valid: value.governingPatternRef == expectedPattern, reason: "executor admission must be governed by A.1"},
		{valid: identityBasisValid, reason: "system identity basis is underdetermined"},
		{valid: identitySystemMatches, reason: "system identity basis must bind the admitted executor system"},
		{valid: actingRefValid, reason: "system acting-eligibility-basis ref is required"},
		{valid: actingDigestValid, reason: "system acting-eligibility-basis digest is required"},
		{valid: sessionValid, reason: "system admission must bind one onboarding session"},
		{valid: windowValid, reason: "system admission must bind a validity window"},
		{valid: value.methodDescriptionRef == pins.descriptionRef, reason: "system admission must bind the pinned MethodDescription ref"},
		{valid: value.methodDescriptionDigest == pins.descriptionDigest, reason: "system admission must bind the pinned MethodDescription digest"},
		{valid: sameProfileOnboardingMethodContractRef(value.methodContractRef, pins.contractRef), reason: "system admission must bind the pinned MethodContract ref"},
		{valid: value.methodContractDigest == pins.contractDigest, reason: "system admission must bind the pinned MethodContract digest"},
		{valid: value.systemAdmissionPolicyRef == pins.systemAdmissionPolicy, reason: "system admission must bind the pinned admission policy"},
	}
	err = visitSliceV1(checks, validateProfileAuthorAssignmentSupportCheckV1)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	return value, nil
}

func canonicalProfileAuthorRoleAdmissionV1(
	value ProfileAuthorRoleAdmissionV1,
) (ProfileAuthorRoleAdmissionV1, error) {
	pins, err := loadProfileOnboardingMethodPinsForRefs(
		value.methodDescriptionRef,
		value.methodContractRef,
	)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	expectedPattern := SourceUnitRef{value: profileAuthorRoleAdmissionPatternRefV1Value}
	refValid := value.ref.valid()
	checks := []profileAuthorAssignmentSupportCheckV1{
		{valid: refValid, reason: "role-admission ref is required"},
		{valid: value.roleRef == pins.requiredRoleRef, reason: "role admission must admit the pinned ProfileAuthor role"},
		{valid: value.boundedContextRef == pins.boundedContextRef, reason: "role admission must use the pinned bounded context"},
		{valid: value.governingPatternRef == expectedPattern, reason: "role admission for the assignment must be governed by A.2.1"},
		{valid: value.methodDescriptionRef == pins.descriptionRef, reason: "role admission must bind the pinned MethodDescription ref"},
		{valid: value.methodDescriptionDigest == pins.descriptionDigest, reason: "role admission must bind the pinned MethodDescription digest"},
		{valid: sameProfileOnboardingMethodContractRef(value.methodContractRef, pins.contractRef), reason: "role admission must bind the pinned MethodContract ref"},
		{valid: value.methodContractDigest == pins.contractDigest, reason: "role admission must bind the pinned MethodContract digest"},
		{valid: value.roleAdmissionPolicyRef == pins.roleAdmissionPolicy, reason: "role admission must bind the pinned admission policy"},
	}
	err = visitSliceV1(checks, validateProfileAuthorAssignmentSupportCheckV1)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	return value, nil
}

func canonicalProfileAuthorAssignmentJustificationV1(
	value ProfileAuthorAssignmentJustificationV1,
) (ProfileAuthorAssignmentJustificationV1, error) {
	pins, err := loadProfileOnboardingMethodPinsForContractRef(value.methodContractRef)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	expectedRule := profileAuthorAssignmentAdmissionRuleV1()
	refValid := value.ref.valid()
	systemRefValid := value.systemAdmissionRef.valid()
	systemDigestValid := value.systemAdmissionDigest.valid()
	roleRefValid := value.roleAdmissionRef.valid()
	roleDigestValid := value.roleAdmissionDigest.valid()
	windowValid := value.assignmentWindow.valid()
	checks := []profileAuthorAssignmentSupportCheckV1{
		{valid: refValid, reason: "assignment-justification ref is required"},
		{valid: value.rule == expectedRule, reason: "assignment justification must apply the pinned admission rule"},
		{valid: value.boundedContextRef == pins.boundedContextRef, reason: "assignment justification must use the pinned bounded context"},
		{valid: systemRefValid, reason: "assignment justification needs a system-admission ref"},
		{valid: systemDigestValid, reason: "assignment justification needs a system-admission digest"},
		{valid: roleRefValid, reason: "assignment justification needs a role-admission ref"},
		{valid: roleDigestValid, reason: "assignment justification needs a role-admission digest"},
		{valid: windowValid, reason: "assignment justification needs a valid window"},
		{valid: sameProfileOnboardingMethodContractRef(value.methodContractRef, pins.contractRef), reason: "assignment justification must bind the pinned MethodContract ref"},
		{valid: value.methodContractDigest == pins.contractDigest, reason: "assignment justification must bind the pinned MethodContract digest"},
	}
	err = visitSliceV1(checks, validateProfileAuthorAssignmentSupportCheckV1)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	return value, nil
}

func canonicalProfileAuthorAssignmentProvenanceV1(
	value ProfileAuthorAssignmentProvenanceV1,
) (ProfileAuthorAssignmentProvenanceV1, error) {
	refValid := value.ref.valid()
	justificationRefValid := value.justificationRef.valid()
	justificationDigestValid := value.justificationDigest.valid()
	sessionValid := value.sessionRef.valid()
	kernelValid := value.kernel.valid()
	runtimeValid := value.runtime.valid()
	recordedAtPresent := !value.recordedAt.IsZero()
	recordedAtLocation := value.recordedAt.Location()
	recordedAtUTC := recordedAtLocation == time.UTC
	checks := []profileAuthorAssignmentSupportCheckV1{
		{valid: refValid, reason: "assignment-provenance ref is required"},
		{valid: justificationRefValid, reason: "assignment provenance needs a justification ref"},
		{valid: justificationDigestValid, reason: "assignment provenance needs a justification digest"},
		{valid: sessionValid, reason: "assignment provenance needs an onboarding session ref"},
		{valid: kernelValid, reason: "assignment provenance needs kernel identity and version"},
		{valid: runtimeValid, reason: "assignment provenance needs runtime identity and version"},
		{valid: recordedAtPresent, reason: "assignment provenance needs recorded-at time"},
		{valid: recordedAtUTC, reason: "assignment provenance recorded-at time must be UTC"},
	}
	err := visitSliceV1(checks, validateProfileAuthorAssignmentSupportCheckV1)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	return value, nil
}

func validateProfileAuthorAssignmentSupportChainV1(
	systemAdmission ProfileOnboardingExecutorSystemAdmissionV1,
	roleAdmission ProfileAuthorRoleAdmissionV1,
	justification ProfileAuthorAssignmentJustificationV1,
	provenance ProfileAuthorAssignmentProvenanceV1,
) error {
	systemValue, err := canonicalProfileOnboardingExecutorSystemAdmissionV1(systemAdmission)
	if err != nil {
		return err
	}
	roleValue, err := canonicalProfileAuthorRoleAdmissionV1(roleAdmission)
	if err != nil {
		return err
	}
	justificationValue, err := canonicalProfileAuthorAssignmentJustificationV1(justification)
	if err != nil {
		return err
	}
	provenanceValue, err := canonicalProfileAuthorAssignmentProvenanceV1(provenance)
	if err != nil {
		return err
	}
	systemDigest, err := DigestProfileOnboardingExecutorSystemAdmissionV1(systemValue)
	if err != nil {
		return err
	}
	roleDigest, err := DigestProfileAuthorRoleAdmissionV1(roleValue)
	if err != nil {
		return err
	}
	justificationDigest, err := DigestProfileAuthorAssignmentJustificationV1(justificationValue)
	if err != nil {
		return err
	}
	windowCovered := systemValue.validityWindow.coversRoleAssignment(
		justificationValue.assignmentWindow,
	)
	recordedInsideSystemAdmission := systemValue.validityWindow.containsMoment(
		provenanceValue.recordedAt,
	)
	checks := []profileAuthorAssignmentSupportCheckV1{
		{valid: systemValue.boundedContextRef == roleValue.boundedContextRef, reason: "system and role admissions must share a bounded context"},
		{valid: systemValue.methodDescriptionRef == roleValue.methodDescriptionRef, reason: "system and role admissions must share a MethodDescription ref"},
		{valid: systemValue.methodDescriptionDigest == roleValue.methodDescriptionDigest, reason: "system and role admissions must share a MethodDescription digest"},
		{valid: sameProfileOnboardingMethodContractRef(systemValue.methodContractRef, roleValue.methodContractRef), reason: "system and role admissions must share a MethodContract ref"},
		{valid: systemValue.methodContractDigest == roleValue.methodContractDigest, reason: "system and role admissions must share a MethodContract digest"},
		{valid: justificationValue.systemAdmissionRef == systemValue.ref, reason: "justification system-admission ref does not match"},
		{valid: justificationValue.systemAdmissionDigest == systemDigest, reason: "justification system-admission digest does not match"},
		{valid: justificationValue.roleAdmissionRef == roleValue.ref, reason: "justification role-admission ref does not match"},
		{valid: justificationValue.roleAdmissionDigest == roleDigest, reason: "justification role-admission digest does not match"},
		{valid: windowCovered, reason: "system-admission window must contain the justified assignment window"},
		{valid: provenanceValue.justificationRef == justificationValue.ref, reason: "provenance justification ref does not match"},
		{valid: provenanceValue.justificationDigest == justificationDigest, reason: "provenance justification digest does not match"},
		{valid: provenanceValue.sessionRef == systemValue.sessionRef, reason: "system admission and provenance must bind the same onboarding session"},
		{valid: recordedInsideSystemAdmission, reason: "assignment origin metadata must be recorded inside the executor-system admission window"},
	}
	err = visitSliceV1(checks, validateProfileAuthorAssignmentSupportCheckV1)
	if err != nil {
		return err
	}
	kernelIdentity, kernelOwned := systemValue.identityBasis.KernelIdentity()
	if kernelOwned && kernelIdentity != provenanceValue.kernel {
		return fmt.Errorf(
			"profile-author assignment support is invalid: kernel-owned executor identity must match provenance kernel identity and build version",
		)
	}
	return nil
}

func sameProfileOnboardingMethodContractRef(
	left ProfileOnboardingMethodContractRef,
	right ProfileOnboardingMethodContractRef,
) bool {
	if left == nil || right == nil {
		return false
	}
	return left.String() == right.String()
}

func validateProfileAuthorRoleAssignmentV1SupportValues(
	assignment ProfileAuthorRoleAssignmentV1,
	systemAdmission ProfileOnboardingExecutorSystemAdmissionV1,
	roleAdmission ProfileAuthorRoleAdmissionV1,
	justification ProfileAuthorAssignmentJustificationV1,
	provenance ProfileAuthorAssignmentProvenanceV1,
) error {
	relation, err := canonicalProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return err
	}
	err = validateProfileAuthorAssignmentSupportChainV1(
		systemAdmission,
		roleAdmission,
		justification,
		provenance,
	)
	if err != nil {
		return err
	}
	systemDigest, err := DigestProfileOnboardingExecutorSystemAdmissionV1(systemAdmission)
	if err != nil {
		return err
	}
	roleDigest, err := DigestProfileAuthorRoleAdmissionV1(roleAdmission)
	if err != nil {
		return err
	}
	justificationDigest, err := DigestProfileAuthorAssignmentJustificationV1(justification)
	if err != nil {
		return err
	}
	provenanceDigest, err := DigestProfileAuthorAssignmentProvenanceV1(provenance)
	if err != nil {
		return err
	}
	windowsMatch := sameRoleAssignmentWindowV1(
		relation.validityWindow,
		justification.assignmentWindow,
	)
	windowContained := systemAdmission.validityWindow.coversRoleAssignment(
		relation.validityWindow,
	)
	checks := []profileAuthorAssignmentSupportCheckV1{
		{valid: relation.systemAdmissionRef == systemAdmission.ref, reason: "RoleAssignment system-admission ref does not match"},
		{valid: relation.systemAdmissionDigest == systemDigest, reason: "RoleAssignment system-admission digest does not match"},
		{valid: relation.roleAdmissionRef == roleAdmission.ref, reason: "RoleAssignment role-admission ref does not match"},
		{valid: relation.roleAdmissionDigest == roleDigest, reason: "RoleAssignment role-admission digest does not match"},
		{valid: relation.justificationRef == justification.ref, reason: "RoleAssignment justification ref does not match"},
		{valid: relation.justificationDigest == justificationDigest, reason: "RoleAssignment justification digest does not match"},
		{valid: relation.provenanceRef == provenance.ref, reason: "RoleAssignment provenance ref does not match"},
		{valid: relation.provenanceDigest == provenanceDigest, reason: "RoleAssignment provenance digest does not match"},
		{valid: relation.holderSystemRef == systemAdmission.systemRef, reason: "RoleAssignment holder does not match admitted executor system"},
		{valid: relation.admittedRoleRef == roleAdmission.roleRef, reason: "RoleAssignment role does not match admitted ProfileAuthor role"},
		{valid: relation.boundedContextRef == systemAdmission.boundedContextRef, reason: "RoleAssignment context does not match system admission"},
		{valid: relation.boundedContextRef == roleAdmission.boundedContextRef, reason: "RoleAssignment context does not match role admission"},
		{valid: windowsMatch, reason: "RoleAssignment window does not match justification"},
		{valid: windowContained, reason: "system-admission window does not contain the RoleAssignment window"},
	}
	return visitSliceV1(checks, validateProfileAuthorAssignmentSupportCheckV1)
}

// ValidateProfileAuthorRoleAssignmentV1Support validates one exact assignment
// against the four separately addressed support objects carried together for
// orchestration convenience. The carrier has no aggregate ontology identity.
func ValidateProfileAuthorRoleAssignmentV1Support(
	assignment ProfileAuthorRoleAssignmentV1,
	carrier ProfileAuthorAssignmentSupportCarrierV1,
) error {
	systemAdmission, roleAdmission, justification, provenance, err := carrier.exactValues()
	if err != nil {
		return err
	}
	return validateProfileAuthorRoleAssignmentV1SupportValues(
		assignment,
		systemAdmission,
		roleAdmission,
		justification,
		provenance,
	)
}

func sameRoleAssignmentWindowV1(
	left RoleAssignmentWindowV1,
	right RoleAssignmentWindowV1,
) bool {
	leftFrom := left.From()
	rightFrom := right.From()
	startsEqual := leftFrom.Equal(rightFrom)
	leftUntil := left.Until()
	rightUntil := right.Until()
	endsEqual := leftUntil.Equal(rightUntil)
	return startsEqual && endsEqual
}
