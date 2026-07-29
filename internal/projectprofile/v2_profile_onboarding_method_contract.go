package projectprofile

import (
	"bytes"
	"fmt"
)

const (
	profileOnboardingMethodDescriptionJSONSchemaV2 = "haft.project-profile.profile-onboarding-method-description/v2"
	profileOnboardingMethodContractJSONSchemaV2    = "haft.project-profile.profile-onboarding-method-contract/v2"
	profileOnboardingMethodDescriptionDigestV2     = "haft.project-profile.profile-onboarding-method-description/v2"
	profileOnboardingMethodContractDigestV2        = "haft.project-profile.profile-onboarding-method-contract/v2"

	profileOnboardingMethodRefV2Value            = "haft:method:profile-onboarding/v2"
	profileOnboardingMethodDescriptionRefV2Value = "haft:method-description:profile-onboarding/v2"
	profileOnboardingMethodContractRefV2Value    = "haft:method-contract:profile-onboarding/v2"
	profileOnboardingMethodEditionV2Value        = "v2"
	profileOnboardingContractEditionV2Value      = "v2"
	profileOnboardingWorkInputKindV1Value        = "ProfileOnboardingWorkInputV1"
)

// ProfileOnboardingMethodContractRef is the closed v1|v2 reference union.
// It cannot represent an unknown future edition.
type ProfileOnboardingMethodContractRef interface {
	String() string
	profileOnboardingMethodContractRefEdition()
}

type ProfileOnboardingMethodContractRefV2 struct{ value string }

func (ref ProfileOnboardingMethodContractRefV2) String() string { return ref.value }
func (ProfileOnboardingMethodContractRefV2) profileOnboardingMethodContractRefEdition() {
}

// ProfileOnboardingMethodDescriptionEdition is the sealed v1|v2 semantic
// union. A source carrier is deliberately not a member of this union.
type ProfileOnboardingMethodDescriptionEdition interface {
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
}

// ProfileOnboardingMethodContractEdition is the sealed v1|v2 semantic union.
// Ref remains edition-specific; MethodContractRefString is the common exact
// projection used by storage without weakening either concrete ref type.
type ProfileOnboardingMethodContractEdition interface {
	Edition() string
	MethodContractRefString() string
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
	profileOnboardingMethodContractEdition()
}

type ProfileOnboardingMethodDescriptionV2 interface {
	ProfileOnboardingMethodDescriptionEdition
	profileOnboardingMethodDescriptionV2()
}

type profileOnboardingMethodDescriptionV2 struct {
	base profileOnboardingMethodDescriptionV1
}

func (profileOnboardingMethodDescriptionV2) profileOnboardingMethodDescriptionEdition() {}
func (profileOnboardingMethodDescriptionV2) profileOnboardingMethodDescriptionV2()      {}

func (description profileOnboardingMethodDescriptionV2) FPFKindName() string {
	return description.base.FPFKindName()
}

func (description profileOnboardingMethodDescriptionV2) Ref() MethodDescriptionRef {
	return description.base.ref
}

func (description profileOnboardingMethodDescriptionV2) DescribedMethodRef() MethodRef {
	return description.base.describedMethodRef
}

func (description profileOnboardingMethodDescriptionV2) BoundedContextRef() BoundedContextRef {
	return description.base.boundedContextRef
}

func (description profileOnboardingMethodDescriptionV2) FPFSourceRevision() ProfileOnboardingFPFSourceRevisionV1 {
	return description.base.sourceRevision
}

func (description profileOnboardingMethodDescriptionV2) Edition() string {
	return description.base.edition
}

func (description profileOnboardingMethodDescriptionV2) ParameterDeclarations() []ProfileOnboardingParameterDeclarationV1 {
	return description.base.ParameterDeclarations()
}

func (description profileOnboardingMethodDescriptionV2) AcceptedInputKinds() []ProfileOnboardingInputKindV1 {
	return description.base.AcceptedInputKinds()
}

func (description profileOnboardingMethodDescriptionV2) AcceptedResultKinds() []ProfileOnboardingResultKindV1 {
	return description.base.AcceptedResultKinds()
}

func (description profileOnboardingMethodDescriptionV2) AffectedRefKind() ProfileOnboardingAffectedKindV1 {
	return description.base.affectedRefKind
}

func (description profileOnboardingMethodDescriptionV2) StatePlaneRef() StatePlaneRef {
	return description.base.statePlaneRef
}

func (description profileOnboardingMethodDescriptionV2) EffectWitnessRuleRef() ProfileOnboardingEffectWitnessRuleRefV1 {
	return description.base.effectWitnessRuleRef
}

func (description profileOnboardingMethodDescriptionV2) EffectWitnessRequirement() ProfileOnboardingEffectWitnessRequirementV1 {
	return description.base.effectWitnessRequirement
}

func (description profileOnboardingMethodDescriptionV2) SuccessCriterion() ProfileOnboardingCriterionV1 {
	return description.base.successCriterion
}

func (description profileOnboardingMethodDescriptionV2) FailureStopPolicy() ProfileOnboardingCriterionV1 {
	return description.base.failureStopPolicy
}

func (description profileOnboardingMethodDescriptionV2) AcceptanceCriterion() ProfileOnboardingCriterionV1 {
	return description.base.acceptanceCriterion
}

func (description profileOnboardingMethodDescriptionV2) RequiredRoleRef() RoleRef {
	return description.base.requiredRoleRef
}

func (description profileOnboardingMethodDescriptionV2) RequiredSystemKind() ProfileOnboardingSystemKindV1 {
	return description.base.requiredSystemKind
}

func ProfileOnboardingMethodRefV2() MethodRef {
	return MethodRef{v1Reference: v1Reference{value: profileOnboardingMethodRefV2Value}}
}

func ProfileOnboardingMethodDescriptionRefV2() MethodDescriptionRef {
	return MethodDescriptionRef{
		v1Reference: v1Reference{value: profileOnboardingMethodDescriptionRefV2Value},
	}
}

func ProfileOnboardingMethodDescriptionV2Value() ProfileOnboardingMethodDescriptionV2 {
	return newProfileOnboardingMethodDescriptionV2()
}

type ProfileOnboardingMethodContractV2 interface {
	ProfileOnboardingMethodContractEdition
	Ref() ProfileOnboardingMethodContractRefV2
	profileOnboardingMethodContractV2()
}

type profileOnboardingMethodContractV2 struct {
	base profileOnboardingMethodContractV1
	ref  ProfileOnboardingMethodContractRefV2
}

func (profileOnboardingMethodContractV2) profileOnboardingMethodContractEdition() {}
func (profileOnboardingMethodContractV2) profileOnboardingMethodContractV2()      {}

func (contract profileOnboardingMethodContractV2) Ref() ProfileOnboardingMethodContractRefV2 {
	return contract.ref
}

func (contract profileOnboardingMethodContractV2) Edition() string {
	return contract.base.edition
}

func (contract profileOnboardingMethodContractV2) MethodContractRefString() string {
	return contract.ref.String()
}

func (contract profileOnboardingMethodContractV2) MethodDescriptionRef() MethodDescriptionRef {
	return contract.base.methodDescriptionRef
}

func (contract profileOnboardingMethodContractV2) MethodDescriptionDigest() ContentDigest {
	return contract.base.methodDescriptionDigest
}

func (contract profileOnboardingMethodContractV2) BoundedContextRef() BoundedContextRef {
	return contract.base.boundedContextRef
}

func (contract profileOnboardingMethodContractV2) RoleAdmissionPolicyRef() ProfileOnboardingAdmissionPolicyRefV1 {
	return contract.base.roleAdmissionPolicyRef
}

func (contract profileOnboardingMethodContractV2) SystemAdmissionPolicyRef() ProfileOnboardingAdmissionPolicyRefV1 {
	return contract.base.systemAdmissionPolicyRef
}

func (contract profileOnboardingMethodContractV2) ParameterSpecSetDigest() ContentDigest {
	return contract.base.parameterSpecSetDigest
}

func (contract profileOnboardingMethodContractV2) AcceptedResultKinds() []ProfileOnboardingResultKindV1 {
	return contract.base.AcceptedResultKinds()
}

func (contract profileOnboardingMethodContractV2) RequiredOccurrenceSlots() []ProfileOnboardingOccurrenceSlotV1 {
	return contract.base.RequiredOccurrenceSlots()
}

func (contract profileOnboardingMethodContractV2) OccurrenceCoverageRuleRefs() []ProfileOnboardingOccurrenceCoverageRuleRefV1 {
	return contract.base.OccurrenceCoverageRuleRefs()
}

func (contract profileOnboardingMethodContractV2) EffectStateWitnessRuleRef() ProfileOnboardingEffectWitnessRuleRefV1 {
	return contract.base.effectStateWitnessRuleRef
}

func (contract profileOnboardingMethodContractV2) AcceptanceStandardRef() ProfileOnboardingAcceptanceStandardRefV1 {
	return contract.base.acceptanceStandardRef
}

func (contract profileOnboardingMethodContractV2) AcceptanceStandardEdition() string {
	return contract.base.acceptanceStandardEdition
}

func (contract profileOnboardingMethodContractV2) HolderEqualsExecutedWithinRule() HolderEqualsExecutedWithinV1 {
	return contract.base.holderEqualsExecutedWithinRule
}

func ProfileOnboardingMethodContractV2Value() (ProfileOnboardingMethodContractV2, error) {
	description := newProfileOnboardingMethodDescriptionV2()
	descriptionDigest, err := DigestProfileOnboardingMethodDescriptionV2(description)
	if err != nil {
		return nil, err
	}
	base, err := newProfileOnboardingMethodContractV1()
	if err != nil {
		return nil, err
	}
	base.edition = profileOnboardingContractEditionV2Value
	base.methodDescriptionRef = description.Ref()
	base.methodDescriptionDigest = descriptionDigest
	base.boundedContextRef = description.BoundedContextRef()
	return profileOnboardingMethodContractV2{
		base: base,
		ref:  ProfileOnboardingMethodContractRefV2{value: profileOnboardingMethodContractRefV2Value},
	}, nil
}

type ProfileOnboardingMethodDescriptionJSONCarrierV2 interface {
	Schema() string
	MediaType() string
	CanonicalJSON() []byte
	ContentDigest() ContentDigest
	profileOnboardingMethodDescriptionJSONCarrierV2()
}

type profileOnboardingMethodDescriptionJSONCarrierV2 struct {
	canonicalJSON []byte
	digest        ContentDigest
}

func (profileOnboardingMethodDescriptionJSONCarrierV2) profileOnboardingMethodDescriptionJSONCarrierV2() {
}
func (profileOnboardingMethodDescriptionJSONCarrierV2) Schema() string {
	return profileOnboardingMethodDescriptionJSONSchemaV2
}
func (profileOnboardingMethodDescriptionJSONCarrierV2) MediaType() string { return "application/json" }
func (carrier profileOnboardingMethodDescriptionJSONCarrierV2) CanonicalJSON() []byte {
	return append([]byte{}, carrier.canonicalJSON...)
}
func (carrier profileOnboardingMethodDescriptionJSONCarrierV2) ContentDigest() ContentDigest {
	return carrier.digest
}

type ProfileOnboardingMethodContractJSONCarrierV2 interface {
	Schema() string
	MediaType() string
	CanonicalJSON() []byte
	ContentDigest() ContentDigest
	profileOnboardingMethodContractJSONCarrierV2()
}

type profileOnboardingMethodContractJSONCarrierV2 struct {
	canonicalJSON []byte
	digest        ContentDigest
}

func (profileOnboardingMethodContractJSONCarrierV2) profileOnboardingMethodContractJSONCarrierV2() {
}
func (profileOnboardingMethodContractJSONCarrierV2) Schema() string {
	return profileOnboardingMethodContractJSONSchemaV2
}
func (profileOnboardingMethodContractJSONCarrierV2) MediaType() string { return "application/json" }
func (carrier profileOnboardingMethodContractJSONCarrierV2) CanonicalJSON() []byte {
	return append([]byte{}, carrier.canonicalJSON...)
}
func (carrier profileOnboardingMethodContractJSONCarrierV2) ContentDigest() ContentDigest {
	return carrier.digest
}

func EncodeProfileOnboardingMethodDescriptionV2CanonicalJSON(
	value ProfileOnboardingMethodDescriptionV2,
) ([]byte, error) {
	exact, err := exactProfileOnboardingMethodDescriptionV2(value)
	if err != nil {
		return nil, err
	}
	dto := profileOnboardingMethodDescriptionToJSONV2(exact)
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileOnboardingMethodDescriptionV2CanonicalJSON(
	data []byte,
) (ProfileOnboardingMethodDescriptionV2, error) {
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
		return nil, fmt.Errorf("profile-onboarding MethodDescription v2 JSON is not canonical")
	}
	expected := newProfileOnboardingMethodDescriptionV2()
	expectedJSON, err := EncodeProfileOnboardingMethodDescriptionV2CanonicalJSON(expected)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonical, expectedJSON) {
		return nil, fmt.Errorf("profile-onboarding MethodDescription JSON does not equal the pinned v2 episteme")
	}
	return expected, nil
}

func DigestProfileOnboardingMethodDescriptionV2(
	value ProfileOnboardingMethodDescriptionV2,
) (ContentDigest, error) {
	canonical, err := EncodeProfileOnboardingMethodDescriptionV2CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	return digestProfileOnboardingCanonicalJSONV1(
		profileOnboardingMethodDescriptionDigestV2,
		canonical,
	), nil
}

func CarryProfileOnboardingMethodDescriptionV2(
	value ProfileOnboardingMethodDescriptionV2,
) (ProfileOnboardingMethodDescriptionJSONCarrierV2, error) {
	canonical, err := EncodeProfileOnboardingMethodDescriptionV2CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	digest := digestProfileOnboardingCanonicalJSONV1(
		profileOnboardingMethodDescriptionDigestV2,
		canonical,
	)
	return profileOnboardingMethodDescriptionJSONCarrierV2{
		canonicalJSON: append([]byte{}, canonical...),
		digest:        digest,
	}, nil
}

func EncodeProfileOnboardingMethodContractV2CanonicalJSON(
	value ProfileOnboardingMethodContractV2,
) ([]byte, error) {
	exact, err := exactProfileOnboardingMethodContractV2(value)
	if err != nil {
		return nil, err
	}
	dto := profileOnboardingMethodContractToJSONV2(exact)
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileOnboardingMethodContractV2CanonicalJSON(
	data []byte,
) (ProfileOnboardingMethodContractV2, error) {
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
		return nil, fmt.Errorf("profile-onboarding MethodContract v2 JSON is not canonical")
	}
	expected, err := ProfileOnboardingMethodContractV2Value()
	if err != nil {
		return nil, err
	}
	expectedJSON, err := EncodeProfileOnboardingMethodContractV2CanonicalJSON(expected)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonical, expectedJSON) {
		return nil, fmt.Errorf("profile-onboarding MethodContract JSON does not equal the pinned v2 contract")
	}
	return expected, nil
}

func DigestProfileOnboardingMethodContractV2(
	value ProfileOnboardingMethodContractV2,
) (ContentDigest, error) {
	canonical, err := EncodeProfileOnboardingMethodContractV2CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	return digestProfileOnboardingCanonicalJSONV1(
		profileOnboardingMethodContractDigestV2,
		canonical,
	), nil
}

func CarryProfileOnboardingMethodContractV2(
	value ProfileOnboardingMethodContractV2,
) (ProfileOnboardingMethodContractJSONCarrierV2, error) {
	canonical, err := EncodeProfileOnboardingMethodContractV2CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	digest := digestProfileOnboardingCanonicalJSONV1(
		profileOnboardingMethodContractDigestV2,
		canonical,
	)
	return profileOnboardingMethodContractJSONCarrierV2{
		canonicalJSON: append([]byte{}, canonical...),
		digest:        digest,
	}, nil
}

func newProfileOnboardingMethodDescriptionV2() profileOnboardingMethodDescriptionV2 {
	base := newProfileOnboardingMethodDescriptionV1()
	base.ref = ProfileOnboardingMethodDescriptionRefV2()
	base.describedMethodRef = ProfileOnboardingMethodRefV2()
	base.edition = profileOnboardingMethodEditionV2Value
	base.acceptedInputKinds = []ProfileOnboardingInputKindV1{
		{value: profileOnboardingObservedBasisKindV1Value},
		{value: profileOnboardingWorkInputKindV1Value},
	}
	return profileOnboardingMethodDescriptionV2{base: base}
}

func exactProfileOnboardingMethodDescriptionV2(
	value ProfileOnboardingMethodDescriptionV2,
) (profileOnboardingMethodDescriptionV2, error) {
	exact, ok := value.(profileOnboardingMethodDescriptionV2)
	if !ok {
		return profileOnboardingMethodDescriptionV2{}, fmt.Errorf("profile-onboarding MethodDescription v2 must be the package-owned value")
	}
	actual, err := EncodeProfileOnboardingMethodDescriptionV2CanonicalJSONUnchecked(exact)
	if err != nil {
		return profileOnboardingMethodDescriptionV2{}, err
	}
	expected := newProfileOnboardingMethodDescriptionV2()
	expectedJSON, err := EncodeProfileOnboardingMethodDescriptionV2CanonicalJSONUnchecked(expected)
	if err != nil {
		return profileOnboardingMethodDescriptionV2{}, err
	}
	if !bytes.Equal(actual, expectedJSON) {
		return profileOnboardingMethodDescriptionV2{}, fmt.Errorf("profile-onboarding MethodDescription differs from the pinned v2 episteme")
	}
	return exact, nil
}

func EncodeProfileOnboardingMethodDescriptionV2CanonicalJSONUnchecked(
	value profileOnboardingMethodDescriptionV2,
) ([]byte, error) {
	dto := profileOnboardingMethodDescriptionToJSONV2(value)
	return marshalCanonicalJSONV1(dto)
}

func exactProfileOnboardingMethodContractV2(
	value ProfileOnboardingMethodContractV2,
) (profileOnboardingMethodContractV2, error) {
	exact, ok := value.(profileOnboardingMethodContractV2)
	if !ok {
		return profileOnboardingMethodContractV2{}, fmt.Errorf("profile-onboarding MethodContract v2 must be the package-owned value")
	}
	actual, err := EncodeProfileOnboardingMethodContractV2CanonicalJSONUnchecked(exact)
	if err != nil {
		return profileOnboardingMethodContractV2{}, err
	}
	expectedValue, err := ProfileOnboardingMethodContractV2Value()
	if err != nil {
		return profileOnboardingMethodContractV2{}, err
	}
	expected := expectedValue.(profileOnboardingMethodContractV2)
	expectedJSON, err := EncodeProfileOnboardingMethodContractV2CanonicalJSONUnchecked(expected)
	if err != nil {
		return profileOnboardingMethodContractV2{}, err
	}
	if !bytes.Equal(actual, expectedJSON) {
		return profileOnboardingMethodContractV2{}, fmt.Errorf("profile-onboarding MethodContract differs from the pinned v2 contract")
	}
	return exact, nil
}

func EncodeProfileOnboardingMethodContractV2CanonicalJSONUnchecked(
	value profileOnboardingMethodContractV2,
) ([]byte, error) {
	dto := profileOnboardingMethodContractToJSONV2(value)
	return marshalCanonicalJSONV1(dto)
}

func profileOnboardingMethodDescriptionToJSONV2(
	value profileOnboardingMethodDescriptionV2,
) profileOnboardingMethodDescriptionJSONV1 {
	dto := profileOnboardingMethodDescriptionToJSONV1(value.base)
	dto.Schema = profileOnboardingMethodDescriptionJSONSchemaV2
	return dto
}

func profileOnboardingMethodContractToJSONV2(
	value profileOnboardingMethodContractV2,
) profileOnboardingMethodContractJSONV1 {
	dto := profileOnboardingMethodContractToJSONV1(value.base)
	dto.Schema = profileOnboardingMethodContractJSONSchemaV2
	dto.Ref = value.ref.String()
	return dto
}

func profileOnboardingMethodContractRefFromString(
	raw string,
) (ProfileOnboardingMethodContractRef, error) {
	switch raw {
	case profileOnboardingMethodContractRefV1Value:
		return ProfileOnboardingMethodContractRefV1{value: raw}, nil
	case profileOnboardingMethodContractRefV2Value:
		return ProfileOnboardingMethodContractRefV2{value: raw}, nil
	default:
		return nil, fmt.Errorf("unknown ProfileOnboardingMethodContract ref %q", raw)
	}
}
