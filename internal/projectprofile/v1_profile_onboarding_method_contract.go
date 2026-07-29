package projectprofile

import (
	"bytes"
	"fmt"
)

const (
	profileOnboardingMethodDescriptionJSONSchemaV1 = "haft.project-profile.profile-onboarding-method-description/v1"
	profileOnboardingMethodContractJSONSchemaV1    = "haft.project-profile.profile-onboarding-method-contract/v1"
	profileOnboardingMethodDescriptionDigestV1     = "haft.project-profile.profile-onboarding-method-description/v1"
	profileOnboardingMethodContractDigestV1        = "haft.project-profile.profile-onboarding-method-contract/v1"
	profileOnboardingParameterSpecSetDigestV1      = "haft.project-profile.profile-onboarding-parameter-spec-set/v1"

	profileOnboardingFPFSourceRevisionV1Value = "44dd88188a07646ef23aca32627a3f670525853f"
	profileOnboardingMethodEditionV1Value     = "v1"
	profileOnboardingContractEditionV1Value   = "v1"
	profileOnboardingAcceptanceEditionV1Value = "v1"
	profileOnboardingMethodDescriptionKindV1  = "U.MethodDescription"

	profileOnboardingBoundedContextRefV1Value        = "haft:bounded-context:profile-onboarding/v1"
	profileOnboardingMethodContractRefV1Value        = "haft:method-contract:profile-onboarding/v1"
	profileOnboardingObservedBasisKindV1Value        = "ObservedProjectBasisV1"
	profileOnboardingCandidateResultKindV1Value      = "CandidatePayloadProduced"
	profileOnboardingUnderdeterminedKindV1Value      = "ClassificationUnderdetermined"
	profileOnboardingAffectedKindV1Value             = "ProfileClassificationEpistemeV1"
	profileOnboardingStatePlaneRefV1Value            = "haft:state-plane:project-profile-classification/v1"
	profileOnboardingSystemKindV1Value               = "U.System"
	profileOnboardingEffectWitnessRuleRefV1Value     = "haft:rule:profile-onboarding/pre-post-or-delta/v1"
	profileOnboardingRoleAdmissionPolicyRefV1Value   = "haft:policy:profile-onboarding/profile-author-role-admission/v1"
	profileOnboardingSystemAdmissionPolicyRefV1Value = "haft:policy:profile-onboarding/system-execution-admission/v1"
	profileOnboardingAcceptanceStandardRefV1Value    = "haft:acceptance-standard:profile-onboarding/v1"
	holderEqualsExecutedWithinRuleRefV1Value         = "haft:rule:profile-onboarding/holder-equals-executed-within/v1"
	roleAssignmentCoversWorkRuleRefV1Value           = "haft:rule:profile-onboarding/role-assignment-covers-work/v1"
	authorityCoversWorkRuleRefV1Value                = "haft:rule:profile-onboarding/authority-covers-work/v1"
	authorityCoversBasisRuleRefV1Value               = "haft:rule:profile-onboarding/authority-covers-basis-observation/v1"
	profileOnboardingWorkIntervalSlotV1Value         = "work_interval"
	profileOnboardingBasisWindowSlotV1Value          = "basis_observation_window"

	profileOnboardingSuccessCriterionRefV1Value    = "haft:criterion:profile-onboarding/typed-result/v1"
	profileOnboardingFailureStopPolicyRefV1Value   = "haft:policy:profile-onboarding/fail-closed/v1"
	profileOnboardingAcceptanceCriterionRefV1Value = "haft:criterion:profile-onboarding/contract-conformance/v1"

	profileOnboardingSuccessCriterionV1Statement = "produce exactly one CandidatePayloadProduced or ClassificationUnderdetermined value"
	profileOnboardingFailureStopV1Statement      = "emit no candidate when required input, parameter, role, system, affected-referent, state-plane, or state-witness basis is absent or invalid; represent insufficient observed basis only as ClassificationUnderdetermined"
	profileOnboardingAcceptanceV1Statement       = "accept only a declared result kind whose inputs, parameter bindings, role and system admission, affected referent, state plane, and pre/post-or-delta witness satisfy this exact contract"
	holderEqualsExecutedWithinV1Statement        = "RoleAssignment.holderSystemRef must equal Work.executedWithinSystemRef for ProfileOnboardingMethod use"
	profileOnboardingEffectWitnessV1Statement    = "require exactly one valid Work state witness: pre-state plus post-state, or pre-state plus delta-predicate; mixed and empty witnesses are inadmissible"
)

// The reference types in this file deliberately have no public parsers. The
// v1 method contract is one package-owned local contract, not an open registry
// of user-minted kinds, policies, or rules.
type ProfileOnboardingMethodContractRefV1 struct{ value string }
type ProfileOnboardingFPFSourceRevisionV1 struct{ value string }
type ProfileOnboardingInputKindV1 struct{ value string }
type ProfileOnboardingResultKindV1 struct{ value string }
type ProfileOnboardingAffectedKindV1 struct{ value string }
type ProfileOnboardingSystemKindV1 struct{ value string }
type ProfileOnboardingEffectWitnessRuleRefV1 struct{ value string }
type ProfileOnboardingAdmissionPolicyRefV1 struct{ value string }
type ProfileOnboardingAcceptanceStandardRefV1 struct{ value string }
type ProfileOnboardingCriterionRefV1 struct{ value string }
type ProfileOnboardingLocalRuleRefV1 struct{ value string }
type ProfileOnboardingOccurrenceSlotV1 struct{ value string }
type ProfileOnboardingOccurrenceCoverageRuleRefV1 struct{ value string }

func (ref ProfileOnboardingMethodContractRefV1) String() string { return ref.value }
func (ProfileOnboardingMethodContractRefV1) profileOnboardingMethodContractRefEdition() {
}
func (ref ProfileOnboardingFPFSourceRevisionV1) String() string     { return ref.value }
func (kind ProfileOnboardingInputKindV1) String() string            { return kind.value }
func (kind ProfileOnboardingResultKindV1) String() string           { return kind.value }
func (kind ProfileOnboardingAffectedKindV1) String() string         { return kind.value }
func (kind ProfileOnboardingSystemKindV1) String() string           { return kind.value }
func (ref ProfileOnboardingEffectWitnessRuleRefV1) String() string  { return ref.value }
func (ref ProfileOnboardingAdmissionPolicyRefV1) String() string    { return ref.value }
func (ref ProfileOnboardingAcceptanceStandardRefV1) String() string { return ref.value }
func (ref ProfileOnboardingCriterionRefV1) String() string          { return ref.value }
func (ref ProfileOnboardingLocalRuleRefV1) String() string          { return ref.value }
func (slot ProfileOnboardingOccurrenceSlotV1) String() string       { return slot.value }
func (ref ProfileOnboardingOccurrenceCoverageRuleRefV1) String() string {
	return ref.value
}

type ProfileOnboardingParameterValueKindV1 struct{ value string }
type ProfileOnboardingParameterBindingLocusV1 struct{ value string }
type ProfileOnboardingEffectWitnessRequirementV1 struct{ statement string }

func (kind ProfileOnboardingParameterValueKindV1) String() string { return kind.value }
func (locus ProfileOnboardingParameterBindingLocusV1) String() string {
	return locus.value
}
func (requirement ProfileOnboardingEffectWitnessRequirementV1) Statement() string {
	return requirement.statement
}

// ProfileOnboardingParameterDeclarationV1 declares a parameter of the method
// description. It carries no run value. All four values bind through the Work
// parameter map. Work and basis-observation windows are occurrence slots in
// the method contract, not MethodDescription parameters.
type ProfileOnboardingParameterDeclarationV1 struct {
	name         string
	valueKind    ProfileOnboardingParameterValueKindV1
	bindingLocus ProfileOnboardingParameterBindingLocusV1
	required     bool
}

func (declaration ProfileOnboardingParameterDeclarationV1) Name() string {
	return declaration.name
}

func (declaration ProfileOnboardingParameterDeclarationV1) ValueKind() ProfileOnboardingParameterValueKindV1 {
	return declaration.valueKind
}

func (declaration ProfileOnboardingParameterDeclarationV1) BindingLocus() ProfileOnboardingParameterBindingLocusV1 {
	return declaration.bindingLocus
}

func (declaration ProfileOnboardingParameterDeclarationV1) Required() bool {
	return declaration.required
}

// ProfileOnboardingCriterionV1 is descriptive contract content. It is neither
// an acceptance verdict nor evidence that Work occurred.
type ProfileOnboardingCriterionV1 struct {
	ref       ProfileOnboardingCriterionRefV1
	statement string
}

func (criterion ProfileOnboardingCriterionV1) Ref() ProfileOnboardingCriterionRefV1 {
	return criterion.ref
}

func (criterion ProfileOnboardingCriterionV1) Statement() string {
	return criterion.statement
}

// HolderEqualsExecutedWithinV1 is a Haft-local admission rule for this exact
// method use. It is not asserted as a new general FPF relation or kind.
type HolderEqualsExecutedWithinV1 interface {
	Ref() ProfileOnboardingLocalRuleRefV1
	Statement() string
	holderEqualsExecutedWithinV1()
}

type holderEqualsExecutedWithinV1 struct{}

func (holderEqualsExecutedWithinV1) holderEqualsExecutedWithinV1() {}

func (holderEqualsExecutedWithinV1) Ref() ProfileOnboardingLocalRuleRefV1 {
	return ProfileOnboardingLocalRuleRefV1{value: holderEqualsExecutedWithinRuleRefV1Value}
}

func (holderEqualsExecutedWithinV1) Statement() string {
	return holderEqualsExecutedWithinV1Statement
}

func ProfileOnboardingHolderEqualsExecutedWithinV1() HolderEqualsExecutedWithinV1 {
	return holderEqualsExecutedWithinV1{}
}

func ValidateHolderEqualsExecutedWithinV1(
	rule HolderEqualsExecutedWithinV1,
	assignment ProfileAuthorRoleAssignmentV1,
	executedWithin SystemRef,
) error {
	_, exact := rule.(holderEqualsExecutedWithinV1)
	if !exact {
		return fmt.Errorf("HolderEqualsExecutedWithinV1 rule must be the package-owned value")
	}
	canonical, err := canonicalProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return err
	}
	if !executedWithin.valid() {
		return fmt.Errorf("executed-within system ref is invalid")
	}
	if canonical.HolderSystemRef() != executedWithin {
		return fmt.Errorf("RoleAssignment holder system must equal Work executed-within system")
	}
	return nil
}

// ProfileOnboardingMethodDescriptionV1 is the fixed U.MethodDescription
// episteme for the local onboarding method. It describes a U.Method; it is not
// that Method, a JSON carrier, a Work occurrence, an authority decision, or an
// acceptance verdict.
type ProfileOnboardingMethodDescriptionV1 interface {
	FPFKindName() string
	Ref() MethodDescriptionRef
	DescribedMethodRef() MethodRef
	BoundedContextRef() BoundedContextRef
	FPFSourceRevision() ProfileOnboardingFPFSourceRevisionV1
	Edition() string
	ParameterDeclarations() []ProfileOnboardingParameterDeclarationV1
	AcceptedInputKinds() []ProfileOnboardingInputKindV1
	AcceptedResultKinds() []ProfileOnboardingResultKindV1
	AffectedRefKind() ProfileOnboardingAffectedKindV1
	StatePlaneRef() StatePlaneRef
	EffectWitnessRuleRef() ProfileOnboardingEffectWitnessRuleRefV1
	EffectWitnessRequirement() ProfileOnboardingEffectWitnessRequirementV1
	SuccessCriterion() ProfileOnboardingCriterionV1
	FailureStopPolicy() ProfileOnboardingCriterionV1
	AcceptanceCriterion() ProfileOnboardingCriterionV1
	RequiredRoleRef() RoleRef
	RequiredSystemKind() ProfileOnboardingSystemKindV1
	profileOnboardingMethodDescriptionEdition()
	profileOnboardingMethodDescriptionV1()
}

type profileOnboardingMethodDescriptionV1 struct {
	ref                      MethodDescriptionRef
	describedMethodRef       MethodRef
	boundedContextRef        BoundedContextRef
	sourceRevision           ProfileOnboardingFPFSourceRevisionV1
	edition                  string
	parameterDeclarations    []ProfileOnboardingParameterDeclarationV1
	acceptedInputKinds       []ProfileOnboardingInputKindV1
	acceptedResultKinds      []ProfileOnboardingResultKindV1
	affectedRefKind          ProfileOnboardingAffectedKindV1
	statePlaneRef            StatePlaneRef
	effectWitnessRuleRef     ProfileOnboardingEffectWitnessRuleRefV1
	effectWitnessRequirement ProfileOnboardingEffectWitnessRequirementV1
	successCriterion         ProfileOnboardingCriterionV1
	failureStopPolicy        ProfileOnboardingCriterionV1
	acceptanceCriterion      ProfileOnboardingCriterionV1
	requiredRoleRef          RoleRef
	requiredSystemKind       ProfileOnboardingSystemKindV1
}

func (profileOnboardingMethodDescriptionV1) profileOnboardingMethodDescriptionV1() {}
func (profileOnboardingMethodDescriptionV1) profileOnboardingMethodDescriptionEdition() {
}

func (profileOnboardingMethodDescriptionV1) FPFKindName() string {
	return profileOnboardingMethodDescriptionKindV1
}

func (description profileOnboardingMethodDescriptionV1) Ref() MethodDescriptionRef {
	return description.ref
}

func (description profileOnboardingMethodDescriptionV1) DescribedMethodRef() MethodRef {
	return description.describedMethodRef
}

func (description profileOnboardingMethodDescriptionV1) BoundedContextRef() BoundedContextRef {
	return description.boundedContextRef
}

func (description profileOnboardingMethodDescriptionV1) FPFSourceRevision() ProfileOnboardingFPFSourceRevisionV1 {
	return description.sourceRevision
}

func (description profileOnboardingMethodDescriptionV1) Edition() string {
	return description.edition
}

func (description profileOnboardingMethodDescriptionV1) ParameterDeclarations() []ProfileOnboardingParameterDeclarationV1 {
	return append([]ProfileOnboardingParameterDeclarationV1{}, description.parameterDeclarations...)
}

func (description profileOnboardingMethodDescriptionV1) AcceptedInputKinds() []ProfileOnboardingInputKindV1 {
	return append([]ProfileOnboardingInputKindV1{}, description.acceptedInputKinds...)
}

func (description profileOnboardingMethodDescriptionV1) AcceptedResultKinds() []ProfileOnboardingResultKindV1 {
	return append([]ProfileOnboardingResultKindV1{}, description.acceptedResultKinds...)
}

func (description profileOnboardingMethodDescriptionV1) AffectedRefKind() ProfileOnboardingAffectedKindV1 {
	return description.affectedRefKind
}

func (description profileOnboardingMethodDescriptionV1) StatePlaneRef() StatePlaneRef {
	return description.statePlaneRef
}

func (description profileOnboardingMethodDescriptionV1) EffectWitnessRuleRef() ProfileOnboardingEffectWitnessRuleRefV1 {
	return description.effectWitnessRuleRef
}

func (description profileOnboardingMethodDescriptionV1) EffectWitnessRequirement() ProfileOnboardingEffectWitnessRequirementV1 {
	return description.effectWitnessRequirement
}

func (description profileOnboardingMethodDescriptionV1) SuccessCriterion() ProfileOnboardingCriterionV1 {
	return description.successCriterion
}

func (description profileOnboardingMethodDescriptionV1) FailureStopPolicy() ProfileOnboardingCriterionV1 {
	return description.failureStopPolicy
}

func (description profileOnboardingMethodDescriptionV1) AcceptanceCriterion() ProfileOnboardingCriterionV1 {
	return description.acceptanceCriterion
}

func (description profileOnboardingMethodDescriptionV1) RequiredRoleRef() RoleRef {
	return description.requiredRoleRef
}

func (description profileOnboardingMethodDescriptionV1) RequiredSystemKind() ProfileOnboardingSystemKindV1 {
	return description.requiredSystemKind
}

func ProfileOnboardingBoundedContextRefV1() BoundedContextRef {
	return BoundedContextRef{v1Reference: v1Reference{value: profileOnboardingBoundedContextRefV1Value}}
}

func ProfileOnboardingMethodDescriptionV1Value() ProfileOnboardingMethodDescriptionV1 {
	return newProfileOnboardingMethodDescriptionV1()
}

// ProfileOnboardingMethodContractV1 is the exact local use contract. It binds
// the description episteme by digest but remains distinct from it. It contains
// no named holder, dates, concrete parameter values, Work result, permission,
// admission receipt, or ledger state.
type ProfileOnboardingMethodContractV1 interface {
	Ref() ProfileOnboardingMethodContractRefV1
	Edition() string
	MethodDescriptionRef() MethodDescriptionRef
	MethodDescriptionDigest() ContentDigest
	BoundedContextRef() BoundedContextRef
	RoleAdmissionPolicyRef() ProfileOnboardingAdmissionPolicyRefV1
	SystemAdmissionPolicyRef() ProfileOnboardingAdmissionPolicyRefV1
	ParameterSpecSetDigest() ContentDigest
	AcceptedResultKinds() []ProfileOnboardingResultKindV1
	RequiredOccurrenceSlots() []ProfileOnboardingOccurrenceSlotV1
	OccurrenceCoverageRuleRefs() []ProfileOnboardingOccurrenceCoverageRuleRefV1
	EffectStateWitnessRuleRef() ProfileOnboardingEffectWitnessRuleRefV1
	AcceptanceStandardRef() ProfileOnboardingAcceptanceStandardRefV1
	AcceptanceStandardEdition() string
	HolderEqualsExecutedWithinRule() HolderEqualsExecutedWithinV1
	MethodContractRefString() string
	profileOnboardingMethodContractEdition()
	profileOnboardingMethodContractV1()
}

type profileOnboardingMethodContractV1 struct {
	ref                            ProfileOnboardingMethodContractRefV1
	edition                        string
	methodDescriptionRef           MethodDescriptionRef
	methodDescriptionDigest        ContentDigest
	boundedContextRef              BoundedContextRef
	roleAdmissionPolicyRef         ProfileOnboardingAdmissionPolicyRefV1
	systemAdmissionPolicyRef       ProfileOnboardingAdmissionPolicyRefV1
	parameterSpecSetDigest         ContentDigest
	acceptedResultKinds            []ProfileOnboardingResultKindV1
	requiredOccurrenceSlots        []ProfileOnboardingOccurrenceSlotV1
	occurrenceCoverageRuleRefs     []ProfileOnboardingOccurrenceCoverageRuleRefV1
	effectStateWitnessRuleRef      ProfileOnboardingEffectWitnessRuleRefV1
	acceptanceStandardRef          ProfileOnboardingAcceptanceStandardRefV1
	acceptanceStandardEdition      string
	holderEqualsExecutedWithinRule holderEqualsExecutedWithinV1
}

func (profileOnboardingMethodContractV1) profileOnboardingMethodContractV1() {}
func (profileOnboardingMethodContractV1) profileOnboardingMethodContractEdition() {
}

func (contract profileOnboardingMethodContractV1) MethodContractRefString() string {
	return contract.ref.String()
}

func (contract profileOnboardingMethodContractV1) Ref() ProfileOnboardingMethodContractRefV1 {
	return contract.ref
}

func (contract profileOnboardingMethodContractV1) Edition() string {
	return contract.edition
}

func (contract profileOnboardingMethodContractV1) MethodDescriptionRef() MethodDescriptionRef {
	return contract.methodDescriptionRef
}

func (contract profileOnboardingMethodContractV1) MethodDescriptionDigest() ContentDigest {
	return contract.methodDescriptionDigest
}

func (contract profileOnboardingMethodContractV1) BoundedContextRef() BoundedContextRef {
	return contract.boundedContextRef
}

func (contract profileOnboardingMethodContractV1) RoleAdmissionPolicyRef() ProfileOnboardingAdmissionPolicyRefV1 {
	return contract.roleAdmissionPolicyRef
}

func (contract profileOnboardingMethodContractV1) SystemAdmissionPolicyRef() ProfileOnboardingAdmissionPolicyRefV1 {
	return contract.systemAdmissionPolicyRef
}

func (contract profileOnboardingMethodContractV1) ParameterSpecSetDigest() ContentDigest {
	return contract.parameterSpecSetDigest
}

func (contract profileOnboardingMethodContractV1) AcceptedResultKinds() []ProfileOnboardingResultKindV1 {
	return append([]ProfileOnboardingResultKindV1{}, contract.acceptedResultKinds...)
}

func (contract profileOnboardingMethodContractV1) RequiredOccurrenceSlots() []ProfileOnboardingOccurrenceSlotV1 {
	return append([]ProfileOnboardingOccurrenceSlotV1{}, contract.requiredOccurrenceSlots...)
}

func (contract profileOnboardingMethodContractV1) OccurrenceCoverageRuleRefs() []ProfileOnboardingOccurrenceCoverageRuleRefV1 {
	return append([]ProfileOnboardingOccurrenceCoverageRuleRefV1{}, contract.occurrenceCoverageRuleRefs...)
}

func (contract profileOnboardingMethodContractV1) EffectStateWitnessRuleRef() ProfileOnboardingEffectWitnessRuleRefV1 {
	return contract.effectStateWitnessRuleRef
}

func (contract profileOnboardingMethodContractV1) AcceptanceStandardRef() ProfileOnboardingAcceptanceStandardRefV1 {
	return contract.acceptanceStandardRef
}

func (contract profileOnboardingMethodContractV1) AcceptanceStandardEdition() string {
	return contract.acceptanceStandardEdition
}

func (contract profileOnboardingMethodContractV1) HolderEqualsExecutedWithinRule() HolderEqualsExecutedWithinV1 {
	return contract.holderEqualsExecutedWithinRule
}

func ProfileOnboardingMethodContractV1Value() (ProfileOnboardingMethodContractV1, error) {
	description := newProfileOnboardingMethodDescriptionV1()
	descriptionDigest, err := DigestProfileOnboardingMethodDescriptionV1(description)
	if err != nil {
		return nil, err
	}
	parameterDigest, err := digestProfileOnboardingParameterSpecSetV1(description.parameterDeclarations)
	if err != nil {
		return nil, err
	}
	return profileOnboardingMethodContractV1{
		ref:                      ProfileOnboardingMethodContractRefV1{value: profileOnboardingMethodContractRefV1Value},
		edition:                  profileOnboardingContractEditionV1Value,
		methodDescriptionRef:     description.ref,
		methodDescriptionDigest:  descriptionDigest,
		boundedContextRef:        description.boundedContextRef,
		roleAdmissionPolicyRef:   ProfileOnboardingAdmissionPolicyRefV1{value: profileOnboardingRoleAdmissionPolicyRefV1Value},
		systemAdmissionPolicyRef: ProfileOnboardingAdmissionPolicyRefV1{value: profileOnboardingSystemAdmissionPolicyRefV1Value},
		parameterSpecSetDigest:   parameterDigest,
		acceptedResultKinds:      append([]ProfileOnboardingResultKindV1{}, description.acceptedResultKinds...),
		requiredOccurrenceSlots: []ProfileOnboardingOccurrenceSlotV1{
			{value: profileOnboardingWorkIntervalSlotV1Value},
			{value: profileOnboardingBasisWindowSlotV1Value},
		},
		occurrenceCoverageRuleRefs: []ProfileOnboardingOccurrenceCoverageRuleRefV1{
			{value: roleAssignmentCoversWorkRuleRefV1Value},
			{value: authorityCoversWorkRuleRefV1Value},
			{value: authorityCoversBasisRuleRefV1Value},
		},
		effectStateWitnessRuleRef:      description.effectWitnessRuleRef,
		acceptanceStandardRef:          ProfileOnboardingAcceptanceStandardRefV1{value: profileOnboardingAcceptanceStandardRefV1Value},
		acceptanceStandardEdition:      profileOnboardingAcceptanceEditionV1Value,
		holderEqualsExecutedWithinRule: holderEqualsExecutedWithinV1{},
	}, nil
}

type profileOnboardingParameterDeclarationJSONV1 struct {
	Name         string `json:"name"`
	ValueKind    string `json:"value_kind"`
	BindingLocus string `json:"binding_locus"`
	Required     bool   `json:"required"`
}

type profileOnboardingCriterionJSONV1 struct {
	Ref       string `json:"ref"`
	Statement string `json:"statement"`
}

type profileOnboardingMethodDescriptionJSONV1 struct {
	Schema                   string                                        `json:"schema"`
	Kind                     string                                        `json:"kind"`
	Ref                      string                                        `json:"ref"`
	DescribedMethodRef       string                                        `json:"described_method_ref"`
	BoundedContextRef        string                                        `json:"bounded_context_ref"`
	SourceRevision           string                                        `json:"source_revision"`
	Edition                  string                                        `json:"edition"`
	ParameterDeclarations    []profileOnboardingParameterDeclarationJSONV1 `json:"parameter_declarations"`
	AcceptedInputKinds       []string                                      `json:"accepted_input_kinds"`
	AcceptedResultKinds      []string                                      `json:"accepted_result_kinds"`
	AffectedRefKind          string                                        `json:"affected_ref_kind"`
	StatePlaneRef            string                                        `json:"state_plane_ref"`
	EffectWitnessRuleRef     string                                        `json:"effect_witness_rule_ref"`
	EffectWitnessRequirement string                                        `json:"effect_witness_requirement"`
	SuccessCriterion         profileOnboardingCriterionJSONV1              `json:"success_criterion"`
	FailureStopPolicy        profileOnboardingCriterionJSONV1              `json:"failure_stop_policy"`
	AcceptanceCriterion      profileOnboardingCriterionJSONV1              `json:"acceptance_criterion"`
	RequiredRoleRef          string                                        `json:"required_role_ref"`
	RequiredSystemKind       string                                        `json:"required_system_kind"`
}

type profileOnboardingMethodContractJSONV1 struct {
	Schema                        string   `json:"schema"`
	Ref                           string   `json:"ref"`
	Edition                       string   `json:"edition"`
	MethodDescriptionRef          string   `json:"method_description_ref"`
	MethodDescriptionDigest       string   `json:"method_description_digest"`
	BoundedContextRef             string   `json:"bounded_context_ref"`
	RoleAdmissionPolicyRef        string   `json:"role_admission_policy_ref"`
	SystemAdmissionPolicyRef      string   `json:"system_admission_policy_ref"`
	ParameterSpecSetDigest        string   `json:"parameter_spec_set_digest"`
	AcceptedResultKinds           []string `json:"accepted_result_kinds"`
	RequiredOccurrenceSlots       []string `json:"required_occurrence_slots"`
	OccurrenceCoverageRuleRefs    []string `json:"occurrence_coverage_rule_refs"`
	EffectStateWitnessRuleRef     string   `json:"effect_state_witness_rule_ref"`
	AcceptanceStandardRef         string   `json:"acceptance_standard_ref"`
	AcceptanceStandardEdition     string   `json:"acceptance_standard_edition"`
	HolderEqualsExecutedWithinRef string   `json:"holder_equals_executed_within_rule_ref"`
}

// ProfileOnboardingMethodDescriptionJSONCarrierV1 makes the representation
// boundary explicit. The carrier is not the MethodDescription episteme.
type ProfileOnboardingMethodDescriptionJSONCarrierV1 interface {
	Schema() string
	MediaType() string
	CanonicalJSON() []byte
	ContentDigest() ContentDigest
	profileOnboardingMethodDescriptionJSONCarrierV1()
}

type profileOnboardingMethodDescriptionJSONCarrierV1 struct {
	canonicalJSON []byte
	digest        ContentDigest
}

func (profileOnboardingMethodDescriptionJSONCarrierV1) profileOnboardingMethodDescriptionJSONCarrierV1() {
}

func (profileOnboardingMethodDescriptionJSONCarrierV1) Schema() string {
	return profileOnboardingMethodDescriptionJSONSchemaV1
}

func (profileOnboardingMethodDescriptionJSONCarrierV1) MediaType() string {
	return "application/json"
}

func (carrier profileOnboardingMethodDescriptionJSONCarrierV1) CanonicalJSON() []byte {
	return append([]byte{}, carrier.canonicalJSON...)
}

func (carrier profileOnboardingMethodDescriptionJSONCarrierV1) ContentDigest() ContentDigest {
	return carrier.digest
}

type ProfileOnboardingMethodContractJSONCarrierV1 interface {
	Schema() string
	MediaType() string
	CanonicalJSON() []byte
	ContentDigest() ContentDigest
	profileOnboardingMethodContractJSONCarrierV1()
}

type profileOnboardingMethodContractJSONCarrierV1 struct {
	canonicalJSON []byte
	digest        ContentDigest
}

func (profileOnboardingMethodContractJSONCarrierV1) profileOnboardingMethodContractJSONCarrierV1() {
}

func (profileOnboardingMethodContractJSONCarrierV1) Schema() string {
	return profileOnboardingMethodContractJSONSchemaV1
}

func (profileOnboardingMethodContractJSONCarrierV1) MediaType() string {
	return "application/json"
}

func (carrier profileOnboardingMethodContractJSONCarrierV1) CanonicalJSON() []byte {
	return append([]byte{}, carrier.canonicalJSON...)
}

func (carrier profileOnboardingMethodContractJSONCarrierV1) ContentDigest() ContentDigest {
	return carrier.digest
}

func EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(
	value ProfileOnboardingMethodDescriptionV1,
) ([]byte, error) {
	exact, err := exactProfileOnboardingMethodDescriptionV1(value)
	if err != nil {
		return nil, err
	}
	dto := profileOnboardingMethodDescriptionToJSONV1(exact)
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileOnboardingMethodDescriptionV1CanonicalJSON(
	data []byte,
) (ProfileOnboardingMethodDescriptionV1, error) {
	var dto profileOnboardingMethodDescriptionJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return nil, err
	}
	canonical, err := marshalCanonicalJSONV1(dto)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("profile-onboarding MethodDescription JSON is not canonical")
	}
	expected := newProfileOnboardingMethodDescriptionV1()
	expectedDTO := profileOnboardingMethodDescriptionToJSONV1(expected)
	expectedJSON, err := marshalCanonicalJSONV1(expectedDTO)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonical, expectedJSON) {
		return nil, fmt.Errorf("profile-onboarding MethodDescription JSON does not equal the pinned v1 episteme")
	}
	return expected, nil
}

func DigestProfileOnboardingMethodDescriptionV1(
	value ProfileOnboardingMethodDescriptionV1,
) (ContentDigest, error) {
	canonical, err := EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	return digestProfileOnboardingCanonicalJSONV1(
		profileOnboardingMethodDescriptionDigestV1,
		canonical,
	), nil
}

func CarryProfileOnboardingMethodDescriptionV1(
	value ProfileOnboardingMethodDescriptionV1,
) (ProfileOnboardingMethodDescriptionJSONCarrierV1, error) {
	canonical, err := EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	digest := digestProfileOnboardingCanonicalJSONV1(
		profileOnboardingMethodDescriptionDigestV1,
		canonical,
	)
	return profileOnboardingMethodDescriptionJSONCarrierV1{
		canonicalJSON: append([]byte{}, canonical...),
		digest:        digest,
	}, nil
}

func EncodeProfileOnboardingMethodContractV1CanonicalJSON(
	value ProfileOnboardingMethodContractV1,
) ([]byte, error) {
	exact, err := exactProfileOnboardingMethodContractV1(value)
	if err != nil {
		return nil, err
	}
	dto := profileOnboardingMethodContractToJSONV1(exact)
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileOnboardingMethodContractV1CanonicalJSON(
	data []byte,
) (ProfileOnboardingMethodContractV1, error) {
	var dto profileOnboardingMethodContractJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return nil, err
	}
	canonical, err := marshalCanonicalJSONV1(dto)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("profile-onboarding MethodContract JSON is not canonical")
	}
	expected, err := newProfileOnboardingMethodContractV1()
	if err != nil {
		return nil, err
	}
	expectedDTO := profileOnboardingMethodContractToJSONV1(expected)
	expectedJSON, err := marshalCanonicalJSONV1(expectedDTO)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonical, expectedJSON) {
		return nil, fmt.Errorf("profile-onboarding MethodContract JSON does not equal the pinned v1 contract")
	}
	return expected, nil
}

func DigestProfileOnboardingMethodContractV1(
	value ProfileOnboardingMethodContractV1,
) (ContentDigest, error) {
	canonical, err := EncodeProfileOnboardingMethodContractV1CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	return digestProfileOnboardingCanonicalJSONV1(
		profileOnboardingMethodContractDigestV1,
		canonical,
	), nil
}

func CarryProfileOnboardingMethodContractV1(
	value ProfileOnboardingMethodContractV1,
) (ProfileOnboardingMethodContractJSONCarrierV1, error) {
	canonical, err := EncodeProfileOnboardingMethodContractV1CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	digest := digestProfileOnboardingCanonicalJSONV1(
		profileOnboardingMethodContractDigestV1,
		canonical,
	)
	return profileOnboardingMethodContractJSONCarrierV1{
		canonicalJSON: append([]byte{}, canonical...),
		digest:        digest,
	}, nil
}

func newProfileOnboardingMethodDescriptionV1() profileOnboardingMethodDescriptionV1 {
	parameters := profileOnboardingParameterDeclarationsV1()
	return profileOnboardingMethodDescriptionV1{
		ref:                   ProfileOnboardingMethodDescriptionRefV1(),
		describedMethodRef:    ProfileOnboardingMethodRefV1(),
		boundedContextRef:     ProfileOnboardingBoundedContextRefV1(),
		sourceRevision:        ProfileOnboardingFPFSourceRevisionV1{value: profileOnboardingFPFSourceRevisionV1Value},
		edition:               profileOnboardingMethodEditionV1Value,
		parameterDeclarations: parameters,
		acceptedInputKinds: []ProfileOnboardingInputKindV1{
			{value: profileOnboardingObservedBasisKindV1Value},
		},
		acceptedResultKinds: []ProfileOnboardingResultKindV1{
			{value: profileOnboardingCandidateResultKindV1Value},
			{value: profileOnboardingUnderdeterminedKindV1Value},
		},
		affectedRefKind:      ProfileOnboardingAffectedKindV1{value: profileOnboardingAffectedKindV1Value},
		statePlaneRef:        StatePlaneRef{v1Reference: v1Reference{value: profileOnboardingStatePlaneRefV1Value}},
		effectWitnessRuleRef: ProfileOnboardingEffectWitnessRuleRefV1{value: profileOnboardingEffectWitnessRuleRefV1Value},
		effectWitnessRequirement: ProfileOnboardingEffectWitnessRequirementV1{
			statement: profileOnboardingEffectWitnessV1Statement,
		},
		successCriterion: ProfileOnboardingCriterionV1{
			ref:       ProfileOnboardingCriterionRefV1{value: profileOnboardingSuccessCriterionRefV1Value},
			statement: profileOnboardingSuccessCriterionV1Statement,
		},
		failureStopPolicy: ProfileOnboardingCriterionV1{
			ref:       ProfileOnboardingCriterionRefV1{value: profileOnboardingFailureStopPolicyRefV1Value},
			statement: profileOnboardingFailureStopV1Statement,
		},
		acceptanceCriterion: ProfileOnboardingCriterionV1{
			ref:       ProfileOnboardingCriterionRefV1{value: profileOnboardingAcceptanceCriterionRefV1Value},
			statement: profileOnboardingAcceptanceV1Statement,
		},
		requiredRoleRef:    ProfileAuthorRoleRefV1(),
		requiredSystemKind: ProfileOnboardingSystemKindV1{value: profileOnboardingSystemKindV1Value},
	}
}

func newProfileOnboardingMethodContractV1() (profileOnboardingMethodContractV1, error) {
	value, err := ProfileOnboardingMethodContractV1Value()
	if err != nil {
		return profileOnboardingMethodContractV1{}, err
	}
	exact, ok := value.(profileOnboardingMethodContractV1)
	if !ok {
		return profileOnboardingMethodContractV1{}, fmt.Errorf("profile-onboarding MethodContract factory returned foreign value")
	}
	return exact, nil
}

func exactProfileOnboardingMethodDescriptionV1(
	value ProfileOnboardingMethodDescriptionV1,
) (profileOnboardingMethodDescriptionV1, error) {
	exact, ok := value.(profileOnboardingMethodDescriptionV1)
	if !ok {
		return profileOnboardingMethodDescriptionV1{}, fmt.Errorf("profile-onboarding MethodDescription must be the package-owned value")
	}
	actualDTO := profileOnboardingMethodDescriptionToJSONV1(exact)
	actual, err := marshalCanonicalJSONV1(actualDTO)
	if err != nil {
		return profileOnboardingMethodDescriptionV1{}, err
	}
	expected := newProfileOnboardingMethodDescriptionV1()
	expectedDTO := profileOnboardingMethodDescriptionToJSONV1(expected)
	expectedJSON, err := marshalCanonicalJSONV1(expectedDTO)
	if err != nil {
		return profileOnboardingMethodDescriptionV1{}, err
	}
	if !bytes.Equal(actual, expectedJSON) {
		return profileOnboardingMethodDescriptionV1{}, fmt.Errorf("profile-onboarding MethodDescription differs from the pinned v1 episteme")
	}
	return exact, nil
}

func exactProfileOnboardingMethodContractV1(
	value ProfileOnboardingMethodContractV1,
) (profileOnboardingMethodContractV1, error) {
	exact, ok := value.(profileOnboardingMethodContractV1)
	if !ok {
		return profileOnboardingMethodContractV1{}, fmt.Errorf("profile-onboarding MethodContract must be the package-owned value")
	}
	actualDTO := profileOnboardingMethodContractToJSONV1(exact)
	actual, err := marshalCanonicalJSONV1(actualDTO)
	if err != nil {
		return profileOnboardingMethodContractV1{}, err
	}
	expected, err := newProfileOnboardingMethodContractV1()
	if err != nil {
		return profileOnboardingMethodContractV1{}, err
	}
	expectedDTO := profileOnboardingMethodContractToJSONV1(expected)
	expectedJSON, err := marshalCanonicalJSONV1(expectedDTO)
	if err != nil {
		return profileOnboardingMethodContractV1{}, err
	}
	if !bytes.Equal(actual, expectedJSON) {
		return profileOnboardingMethodContractV1{}, fmt.Errorf("profile-onboarding MethodContract differs from the pinned v1 contract")
	}
	return exact, nil
}

func profileOnboardingMethodDescriptionToJSONV1(
	value profileOnboardingMethodDescriptionV1,
) profileOnboardingMethodDescriptionJSONV1 {
	return profileOnboardingMethodDescriptionJSONV1{
		Schema:                   profileOnboardingMethodDescriptionJSONSchemaV1,
		Kind:                     profileOnboardingMethodDescriptionKindV1,
		Ref:                      value.ref.String(),
		DescribedMethodRef:       value.describedMethodRef.String(),
		BoundedContextRef:        value.boundedContextRef.String(),
		SourceRevision:           value.sourceRevision.String(),
		Edition:                  value.edition,
		ParameterDeclarations:    profileOnboardingParameterDeclarationsToJSONV1(value.parameterDeclarations),
		AcceptedInputKinds:       profileOnboardingInputKindsToStringsV1(value.acceptedInputKinds),
		AcceptedResultKinds:      profileOnboardingResultKindsToStringsV1(value.acceptedResultKinds),
		AffectedRefKind:          value.affectedRefKind.String(),
		StatePlaneRef:            value.statePlaneRef.String(),
		EffectWitnessRuleRef:     value.effectWitnessRuleRef.String(),
		EffectWitnessRequirement: value.effectWitnessRequirement.Statement(),
		SuccessCriterion:         profileOnboardingCriterionToJSONV1(value.successCriterion),
		FailureStopPolicy:        profileOnboardingCriterionToJSONV1(value.failureStopPolicy),
		AcceptanceCriterion:      profileOnboardingCriterionToJSONV1(value.acceptanceCriterion),
		RequiredRoleRef:          value.requiredRoleRef.String(),
		RequiredSystemKind:       value.requiredSystemKind.String(),
	}
}

func profileOnboardingMethodContractToJSONV1(
	value profileOnboardingMethodContractV1,
) profileOnboardingMethodContractJSONV1 {
	return profileOnboardingMethodContractJSONV1{
		Schema:                        profileOnboardingMethodContractJSONSchemaV1,
		Ref:                           value.ref.String(),
		Edition:                       value.edition,
		MethodDescriptionRef:          value.methodDescriptionRef.String(),
		MethodDescriptionDigest:       value.methodDescriptionDigest.String(),
		BoundedContextRef:             value.boundedContextRef.String(),
		RoleAdmissionPolicyRef:        value.roleAdmissionPolicyRef.String(),
		SystemAdmissionPolicyRef:      value.systemAdmissionPolicyRef.String(),
		ParameterSpecSetDigest:        value.parameterSpecSetDigest.String(),
		AcceptedResultKinds:           profileOnboardingResultKindsToStringsV1(value.acceptedResultKinds),
		RequiredOccurrenceSlots:       profileOnboardingOccurrenceSlotsToStringsV1(value.requiredOccurrenceSlots),
		OccurrenceCoverageRuleRefs:    profileOnboardingOccurrenceRulesToStringsV1(value.occurrenceCoverageRuleRefs),
		EffectStateWitnessRuleRef:     value.effectStateWitnessRuleRef.String(),
		AcceptanceStandardRef:         value.acceptanceStandardRef.String(),
		AcceptanceStandardEdition:     value.acceptanceStandardEdition,
		HolderEqualsExecutedWithinRef: value.holderEqualsExecutedWithinRule.Ref().String(),
	}
}

func profileOnboardingParameterDeclarationsV1() []ProfileOnboardingParameterDeclarationV1 {
	return []ProfileOnboardingParameterDeclarationV1{
		profileOnboardingParameterDeclarationV1(
			"classifier_version",
			"ClassifierVersion",
			"work.parameter_bindings",
		),
		profileOnboardingParameterDeclarationV1(
			"policy_version",
			"PolicyVersion",
			"work.parameter_bindings",
		),
		profileOnboardingParameterDeclarationV1(
			"project_root",
			"ProjectRootV1",
			"work.parameter_bindings",
		),
		profileOnboardingParameterDeclarationV1(
			"session_ref",
			"SessionRef",
			"work.parameter_bindings",
		),
	}
}

func profileOnboardingParameterDeclarationV1(
	name string,
	valueKind string,
	bindingLocus string,
) ProfileOnboardingParameterDeclarationV1 {
	return ProfileOnboardingParameterDeclarationV1{
		name:         name,
		valueKind:    ProfileOnboardingParameterValueKindV1{value: valueKind},
		bindingLocus: ProfileOnboardingParameterBindingLocusV1{value: bindingLocus},
		required:     true,
	}
}

func profileOnboardingParameterDeclarationsToJSONV1(
	values []ProfileOnboardingParameterDeclarationV1,
) []profileOnboardingParameterDeclarationJSONV1 {
	return mapSliceV1Pure(values, func(value ProfileOnboardingParameterDeclarationV1) profileOnboardingParameterDeclarationJSONV1 {
		return profileOnboardingParameterDeclarationJSONV1{
			Name:         value.Name(),
			ValueKind:    value.ValueKind().String(),
			BindingLocus: value.BindingLocus().String(),
			Required:     value.Required(),
		}
	})
}

func profileOnboardingInputKindsToStringsV1(
	values []ProfileOnboardingInputKindV1,
) []string {
	return mapSliceV1Pure(values, func(value ProfileOnboardingInputKindV1) string {
		return value.String()
	})
}

func profileOnboardingResultKindsToStringsV1(
	values []ProfileOnboardingResultKindV1,
) []string {
	return mapSliceV1Pure(values, func(value ProfileOnboardingResultKindV1) string {
		return value.String()
	})
}

func profileOnboardingOccurrenceSlotsToStringsV1(
	values []ProfileOnboardingOccurrenceSlotV1,
) []string {
	return mapSliceV1Pure(values, func(value ProfileOnboardingOccurrenceSlotV1) string {
		return value.String()
	})
}

func profileOnboardingOccurrenceRulesToStringsV1(
	values []ProfileOnboardingOccurrenceCoverageRuleRefV1,
) []string {
	return mapSliceV1Pure(values, func(value ProfileOnboardingOccurrenceCoverageRuleRefV1) string {
		return value.String()
	})
}

func profileOnboardingCriterionToJSONV1(
	value ProfileOnboardingCriterionV1,
) profileOnboardingCriterionJSONV1 {
	return profileOnboardingCriterionJSONV1{
		Ref:       value.Ref().String(),
		Statement: value.Statement(),
	}
}

func digestProfileOnboardingParameterSpecSetV1(
	values []ProfileOnboardingParameterDeclarationV1,
) (ContentDigest, error) {
	dto := profileOnboardingParameterDeclarationsToJSONV1(values)
	canonical, err := marshalCanonicalJSONV1(dto)
	if err != nil {
		return ContentDigest{}, err
	}
	return digestProfileOnboardingCanonicalJSONV1(
		profileOnboardingParameterSpecSetDigestV1,
		canonical,
	), nil
}

func digestProfileOnboardingCanonicalJSONV1(
	domain string,
	canonical []byte,
) ContentDigest {
	writer := newCanonicalDigestWriter(domain)
	writer.add(string(canonical))
	return writer.digest()
}
