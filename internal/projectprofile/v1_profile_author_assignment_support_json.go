package projectprofile

import (
	"bytes"
	"fmt"
)

const (
	profileOnboardingExecutorSystemAdmissionJSONSchemaV1 = "haft.project-profile.profile-onboarding-executor-system-admission/v1"
	profileAuthorRoleAdmissionJSONSchemaV1               = "haft.project-profile.profile-author-role-admission/v1"
	profileAuthorAssignmentJustificationJSONSchemaV1     = "haft.project-profile.profile-author-assignment-justification/v1"
	profileAuthorAssignmentProvenanceJSONSchemaV1        = "haft.project-profile.profile-author-assignment-provenance/v1"
	profileOnboardingExecutorSystemAdmissionJSONSchemaV2 = "haft.project-profile.profile-onboarding-executor-system-admission/v2"
	profileAuthorRoleAdmissionJSONSchemaV2               = "haft.project-profile.profile-author-role-admission/v2"
	profileAuthorAssignmentJustificationJSONSchemaV2     = "haft.project-profile.profile-author-assignment-justification/v2"

	profileOnboardingExecutorSystemAdmissionDigestDomainV1 = "haft.project-profile.profile-onboarding-executor-system-admission/v1"
	profileAuthorRoleAdmissionDigestDomainV1               = "haft.project-profile.profile-author-role-admission/v1"
	profileAuthorAssignmentJustificationDigestDomainV1     = "haft.project-profile.profile-author-assignment-justification/v1"
	profileAuthorAssignmentProvenanceDigestDomainV1        = "haft.project-profile.profile-author-assignment-provenance/v1"
	profileOnboardingExecutorSystemAdmissionDigestDomainV2 = "haft.project-profile.profile-onboarding-executor-system-admission/v2"
	profileAuthorRoleAdmissionDigestDomainV2               = "haft.project-profile.profile-author-role-admission/v2"
	profileAuthorAssignmentJustificationDigestDomainV2     = "haft.project-profile.profile-author-assignment-justification/v2"
)

type profileOnboardingExecutorSystemAdmissionJSONV1 struct {
	Schema                       string                                       `json:"schema"`
	Ref                          string                                       `json:"ref"`
	SystemRef                    string                                       `json:"system_ref"`
	AdmittedSystemKind           string                                       `json:"admitted_system_kind"`
	BoundedContextRef            string                                       `json:"bounded_context_ref"`
	GoverningPatternRef          string                                       `json:"governing_pattern_ref"`
	IdentityBasis                profileOnboardingExecutorIdentityBasisJSONV1 `json:"identity_basis"`
	ActingEligibilityBasisRef    string                                       `json:"acting_eligibility_basis_ref"`
	ActingEligibilityBasisDigest string                                       `json:"acting_eligibility_basis_digest"`
	SessionRef                   string                                       `json:"session_ref"`
	ValidityWindow               closedIntervalJSONV1                         `json:"validity_window"`
	MethodDescriptionRef         string                                       `json:"method_description_ref"`
	MethodDescriptionDigest      string                                       `json:"method_description_digest"`
	MethodContractRef            string                                       `json:"method_contract_ref"`
	MethodContractDigest         string                                       `json:"method_contract_digest"`
	SystemAdmissionPolicyRef     string                                       `json:"system_admission_policy_ref"`
}

type profileOnboardingOperatorDesignationJSONV1 struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type profileOnboardingExecutorIdentityBasisJSONV1 struct {
	Kind               string                                      `json:"kind"`
	SystemRef          string                                      `json:"system_ref"`
	KernelOwned        *profileOnboardingRuntimeIdentityJSONV1     `json:"kernel_owned,omitempty"`
	OperatorDesignated *profileOnboardingOperatorDesignationJSONV1 `json:"operator_designated,omitempty"`
}

type profileAuthorRoleAdmissionJSONV1 struct {
	Schema                  string `json:"schema"`
	Ref                     string `json:"ref"`
	RoleRef                 string `json:"role_ref"`
	BoundedContextRef       string `json:"bounded_context_ref"`
	GoverningPatternRef     string `json:"governing_pattern_ref"`
	MethodDescriptionRef    string `json:"method_description_ref"`
	MethodDescriptionDigest string `json:"method_description_digest"`
	MethodContractRef       string `json:"method_contract_ref"`
	MethodContractDigest    string `json:"method_contract_digest"`
	RoleAdmissionPolicyRef  string `json:"role_admission_policy_ref"`
}

type profileAuthorAssignmentJustificationJSONV1 struct {
	Schema                string               `json:"schema"`
	Ref                   string               `json:"ref"`
	RuleRef               string               `json:"rule_ref"`
	RuleStatement         string               `json:"rule_statement"`
	BoundedContextRef     string               `json:"bounded_context_ref"`
	SystemAdmissionRef    string               `json:"system_admission_ref"`
	SystemAdmissionDigest string               `json:"system_admission_digest"`
	RoleAdmissionRef      string               `json:"role_admission_ref"`
	RoleAdmissionDigest   string               `json:"role_admission_digest"`
	AssignmentWindow      closedIntervalJSONV1 `json:"assignment_window"`
	MethodContractRef     string               `json:"method_contract_ref"`
	MethodContractDigest  string               `json:"method_contract_digest"`
}

type profileOnboardingRuntimeIdentityJSONV1 struct {
	Identity string `json:"identity"`
	Version  string `json:"version"`
}

type profileAuthorAssignmentProvenanceJSONV1 struct {
	Schema              string                                 `json:"schema"`
	Ref                 string                                 `json:"ref"`
	JustificationRef    string                                 `json:"justification_ref"`
	JustificationDigest string                                 `json:"justification_digest"`
	SessionRef          string                                 `json:"session_ref"`
	Kernel              profileOnboardingRuntimeIdentityJSONV1 `json:"kernel"`
	Runtime             profileOnboardingRuntimeIdentityJSONV1 `json:"runtime"`
	RecordedAt          string                                 `json:"recorded_at"`
}

func EncodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(
	value ProfileOnboardingExecutorSystemAdmissionV1,
) ([]byte, error) {
	canonical, err := canonicalProfileOnboardingExecutorSystemAdmissionV1(value)
	if err != nil {
		return nil, err
	}
	dto, err := profileOnboardingExecutorSystemAdmissionToJSONV1(canonical)
	if err != nil {
		return nil, err
	}
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(
	data []byte,
) (ProfileOnboardingExecutorSystemAdmissionV1, error) {
	var dto profileOnboardingExecutorSystemAdmissionJSONV1
	if err := decodeJSONV1(data, &dto); err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	if dto.Schema != profileOnboardingExecutorSystemAdmissionJSONSchemaV1 {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, fmt.Errorf("unsupported executor-system admission schema %q", dto.Schema)
	}
	return DecodeProfileOnboardingExecutorSystemAdmissionCanonicalJSON(data)
}

func DecodeProfileOnboardingExecutorSystemAdmissionCanonicalJSON(
	data []byte,
) (ProfileOnboardingExecutorSystemAdmissionV1, error) {
	var dto profileOnboardingExecutorSystemAdmissionJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	value, err := profileOnboardingExecutorSystemAdmissionFromJSONV1(dto)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	canonical, err := EncodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(value)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, fmt.Errorf("executor-system admission JSON is not canonical")
	}
	return value, nil
}

func DigestProfileOnboardingExecutorSystemAdmissionV1(
	value ProfileOnboardingExecutorSystemAdmissionV1,
) (ContentDigest, error) {
	data, err := EncodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	domain, err := profileOnboardingExecutorSystemAdmissionDigestDomain(value.methodContractRef)
	if err != nil {
		return ContentDigest{}, err
	}
	writer := newCanonicalDigestWriter(domain)
	content := string(data)
	writer.add(content)
	return writer.digest(), nil
}

func EncodeProfileAuthorRoleAdmissionV1CanonicalJSON(
	value ProfileAuthorRoleAdmissionV1,
) ([]byte, error) {
	canonical, err := canonicalProfileAuthorRoleAdmissionV1(value)
	if err != nil {
		return nil, err
	}
	dto := profileAuthorRoleAdmissionToJSONV1(canonical)
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileAuthorRoleAdmissionV1CanonicalJSON(
	data []byte,
) (ProfileAuthorRoleAdmissionV1, error) {
	var dto profileAuthorRoleAdmissionJSONV1
	if err := decodeJSONV1(data, &dto); err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	if dto.Schema != profileAuthorRoleAdmissionJSONSchemaV1 {
		return ProfileAuthorRoleAdmissionV1{}, fmt.Errorf("unsupported ProfileAuthor role-admission schema %q", dto.Schema)
	}
	return DecodeProfileAuthorRoleAdmissionCanonicalJSON(data)
}

func DecodeProfileAuthorRoleAdmissionCanonicalJSON(
	data []byte,
) (ProfileAuthorRoleAdmissionV1, error) {
	var dto profileAuthorRoleAdmissionJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	value, err := profileAuthorRoleAdmissionFromJSONV1(dto)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	canonical, err := EncodeProfileAuthorRoleAdmissionV1CanonicalJSON(value)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ProfileAuthorRoleAdmissionV1{}, fmt.Errorf("ProfileAuthor role-admission JSON is not canonical")
	}
	return value, nil
}

func DigestProfileAuthorRoleAdmissionV1(
	value ProfileAuthorRoleAdmissionV1,
) (ContentDigest, error) {
	data, err := EncodeProfileAuthorRoleAdmissionV1CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	domain, err := profileAuthorRoleAdmissionDigestDomain(value.methodContractRef)
	if err != nil {
		return ContentDigest{}, err
	}
	writer := newCanonicalDigestWriter(domain)
	content := string(data)
	writer.add(content)
	return writer.digest(), nil
}

func EncodeProfileAuthorAssignmentJustificationV1CanonicalJSON(
	value ProfileAuthorAssignmentJustificationV1,
) ([]byte, error) {
	canonical, err := canonicalProfileAuthorAssignmentJustificationV1(value)
	if err != nil {
		return nil, err
	}
	dto := profileAuthorAssignmentJustificationToJSONV1(canonical)
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileAuthorAssignmentJustificationV1CanonicalJSON(
	data []byte,
) (ProfileAuthorAssignmentJustificationV1, error) {
	var dto profileAuthorAssignmentJustificationJSONV1
	if err := decodeJSONV1(data, &dto); err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	if dto.Schema != profileAuthorAssignmentJustificationJSONSchemaV1 {
		return ProfileAuthorAssignmentJustificationV1{}, fmt.Errorf("unsupported ProfileAuthor assignment-justification schema %q", dto.Schema)
	}
	return DecodeProfileAuthorAssignmentJustificationCanonicalJSON(data)
}

func DecodeProfileAuthorAssignmentJustificationCanonicalJSON(
	data []byte,
) (ProfileAuthorAssignmentJustificationV1, error) {
	var dto profileAuthorAssignmentJustificationJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	value, err := profileAuthorAssignmentJustificationFromJSONV1(dto)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	canonical, err := EncodeProfileAuthorAssignmentJustificationV1CanonicalJSON(value)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ProfileAuthorAssignmentJustificationV1{}, fmt.Errorf("ProfileAuthor assignment-justification JSON is not canonical")
	}
	return value, nil
}

func DigestProfileAuthorAssignmentJustificationV1(
	value ProfileAuthorAssignmentJustificationV1,
) (ContentDigest, error) {
	data, err := EncodeProfileAuthorAssignmentJustificationV1CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	domain, err := profileAuthorAssignmentJustificationDigestDomain(value.methodContractRef)
	if err != nil {
		return ContentDigest{}, err
	}
	writer := newCanonicalDigestWriter(domain)
	content := string(data)
	writer.add(content)
	return writer.digest(), nil
}

func EncodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(
	value ProfileAuthorAssignmentProvenanceV1,
) ([]byte, error) {
	canonical, err := canonicalProfileAuthorAssignmentProvenanceV1(value)
	if err != nil {
		return nil, err
	}
	dto := profileAuthorAssignmentProvenanceToJSONV1(canonical)
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(
	data []byte,
) (ProfileAuthorAssignmentProvenanceV1, error) {
	var dto profileAuthorAssignmentProvenanceJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	value, err := profileAuthorAssignmentProvenanceFromJSONV1(dto)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	canonical, err := EncodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(value)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ProfileAuthorAssignmentProvenanceV1{}, fmt.Errorf("ProfileAuthor assignment-provenance JSON is not canonical")
	}
	return value, nil
}

func DigestProfileAuthorAssignmentProvenanceV1(
	value ProfileAuthorAssignmentProvenanceV1,
) (ContentDigest, error) {
	data, err := EncodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(value)
	if err != nil {
		return ContentDigest{}, err
	}
	writer := newCanonicalDigestWriter(profileAuthorAssignmentProvenanceDigestDomainV1)
	content := string(data)
	writer.add(content)
	return writer.digest(), nil
}

func profileOnboardingExecutorSystemAdmissionToJSONV1(
	value ProfileOnboardingExecutorSystemAdmissionV1,
) (profileOnboardingExecutorSystemAdmissionJSONV1, error) {
	identityBasis, err := profileOnboardingExecutorIdentityBasisToJSONV1(value.identityBasis)
	if err != nil {
		return profileOnboardingExecutorSystemAdmissionJSONV1{}, err
	}
	ref := value.ref.String()
	systemRef := value.systemRef.String()
	systemKind := value.admittedSystemKind.String()
	contextRef := value.boundedContextRef.String()
	patternRef := value.governingPatternRef.String()
	actingRef := value.actingEligibilityBasisRef.String()
	actingDigest := value.actingEligibilityBasisDigest.String()
	sessionRef := value.sessionRef.String()
	validityWindow := closedIntervalToJSONV1(value.validityWindow.closedIntervalV1)
	descriptionRef := value.methodDescriptionRef.String()
	descriptionDigest := value.methodDescriptionDigest.String()
	contractRef := value.methodContractRef.String()
	contractDigest := value.methodContractDigest.String()
	policyRef := value.systemAdmissionPolicyRef.String()
	schema, err := profileOnboardingExecutorSystemAdmissionSchema(value.methodContractRef)
	if err != nil {
		return profileOnboardingExecutorSystemAdmissionJSONV1{}, err
	}
	return profileOnboardingExecutorSystemAdmissionJSONV1{
		Schema:                       schema,
		Ref:                          ref,
		SystemRef:                    systemRef,
		AdmittedSystemKind:           systemKind,
		BoundedContextRef:            contextRef,
		GoverningPatternRef:          patternRef,
		IdentityBasis:                identityBasis,
		ActingEligibilityBasisRef:    actingRef,
		ActingEligibilityBasisDigest: actingDigest,
		SessionRef:                   sessionRef,
		ValidityWindow:               validityWindow,
		MethodDescriptionRef:         descriptionRef,
		MethodDescriptionDigest:      descriptionDigest,
		MethodContractRef:            contractRef,
		MethodContractDigest:         contractDigest,
		SystemAdmissionPolicyRef:     policyRef,
	}, nil
}

func profileOnboardingExecutorIdentityBasisToJSONV1(
	basis ProfileOnboardingExecutorIdentityBasisV1,
) (profileOnboardingExecutorIdentityBasisJSONV1, error) {
	canonical, err := canonicalProfileOnboardingExecutorIdentityBasisV1(basis)
	if err != nil {
		return profileOnboardingExecutorIdentityBasisJSONV1{}, err
	}
	kernel, kernelOwned := canonical.KernelIdentity()
	systemRef := canonical.SystemRef()
	systemRefValue := systemRef.String()
	if kernelOwned {
		kernelIdentity := kernel.Identity()
		kernelVersion := kernel.Version()
		kernelJSON := profileOnboardingRuntimeIdentityJSONV1{
			Identity: kernelIdentity,
			Version:  kernelVersion,
		}
		return profileOnboardingExecutorIdentityBasisJSONV1{
			Kind:        profileOnboardingKernelExecutorIdentityKindV1,
			SystemRef:   systemRefValue,
			KernelOwned: &kernelJSON,
		}, nil
	}
	designationRef, designationDigest, operatorDesignated := canonical.OperatorDesignation()
	if !operatorDesignated {
		return profileOnboardingExecutorIdentityBasisJSONV1{}, fmt.Errorf("unsupported executor identity-basis variant")
	}
	designationRefValue := designationRef.String()
	designationDigestValue := designationDigest.String()
	designationJSON := profileOnboardingOperatorDesignationJSONV1{
		Ref:    designationRefValue,
		Digest: designationDigestValue,
	}
	return profileOnboardingExecutorIdentityBasisJSONV1{
		Kind:               profileOnboardingOperatorExecutorIdentityKindV1,
		SystemRef:          systemRefValue,
		OperatorDesignated: &designationJSON,
	}, nil
}

func profileOnboardingExecutorSystemAdmissionFromJSONV1(
	dto profileOnboardingExecutorSystemAdmissionJSONV1,
) (ProfileOnboardingExecutorSystemAdmissionV1, error) {
	ref, err := NewSystemAdmissionRef(dto.Ref)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	systemRef, err := NewSystemRef(dto.SystemRef)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	contextRef, err := NewBoundedContextRef(dto.BoundedContextRef)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	patternRef, err := NewSourceUnitRef(dto.GoverningPatternRef)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	identityBasis, err := profileOnboardingExecutorIdentityBasisFromJSONV1(dto.IdentityBasis)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	actingRef, err := NewProfileOnboardingSystemActingEligibilityBasisRefV1(dto.ActingEligibilityBasisRef)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	actingDigest, err := NewContentDigest(dto.ActingEligibilityBasisDigest)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	sessionRef, err := NewSessionRef(dto.SessionRef)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	windowValue, err := closedIntervalFromJSONV1(
		"profile-onboarding executor-admission window",
		dto.ValidityWindow,
	)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	descriptionRef, err := NewMethodDescriptionRef(dto.MethodDescriptionRef)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	descriptionDigest, err := NewContentDigest(dto.MethodDescriptionDigest)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	contractDigest, err := NewContentDigest(dto.MethodContractDigest)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	contractRef, err := profileOnboardingMethodContractRefFromString(dto.MethodContractRef)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	expectedSchema, err := profileOnboardingExecutorSystemAdmissionSchema(contractRef)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	if dto.Schema != expectedSchema {
		return ProfileOnboardingExecutorSystemAdmissionV1{}, fmt.Errorf("executor-system admission schema does not match its method edition")
	}
	value := ProfileOnboardingExecutorSystemAdmissionV1{
		ref:                          ref,
		systemRef:                    systemRef,
		admittedSystemKind:           ProfileOnboardingSystemKindV1{value: dto.AdmittedSystemKind},
		boundedContextRef:            contextRef,
		governingPatternRef:          patternRef,
		identityBasis:                identityBasis,
		actingEligibilityBasisRef:    actingRef,
		actingEligibilityBasisDigest: actingDigest,
		sessionRef:                   sessionRef,
		validityWindow: ProfileOnboardingExecutorAdmissionWindowV1{
			closedIntervalV1: windowValue,
		},
		methodDescriptionRef:     descriptionRef,
		methodDescriptionDigest:  descriptionDigest,
		methodContractRef:        contractRef,
		methodContractDigest:     contractDigest,
		systemAdmissionPolicyRef: ProfileOnboardingAdmissionPolicyRefV1{value: dto.SystemAdmissionPolicyRef},
	}
	return canonicalProfileOnboardingExecutorSystemAdmissionV1(value)
}

func profileOnboardingExecutorIdentityBasisFromJSONV1(
	dto profileOnboardingExecutorIdentityBasisJSONV1,
) (ProfileOnboardingExecutorIdentityBasisV1, error) {
	systemRef, err := NewSystemRef(dto.SystemRef)
	if err != nil {
		return ProfileOnboardingExecutorIdentityBasisV1{}, err
	}
	if dto.Kind == profileOnboardingKernelExecutorIdentityKindV1 {
		if dto.KernelOwned == nil || dto.OperatorDesignated != nil {
			return ProfileOnboardingExecutorIdentityBasisV1{}, fmt.Errorf(
				"kernel-owned executor identity basis must contain only kernel_owned content",
			)
		}
		kernel, kernelErr := NewProfileOnboardingKernelIdentityV1(
			dto.KernelOwned.Identity,
			dto.KernelOwned.Version,
		)
		if kernelErr != nil {
			return ProfileOnboardingExecutorIdentityBasisV1{}, kernelErr
		}
		return NewProfileOnboardingKernelExecutorIdentityBasisV1(systemRef, kernel)
	}
	if dto.Kind != profileOnboardingOperatorExecutorIdentityKindV1 {
		return ProfileOnboardingExecutorIdentityBasisV1{}, fmt.Errorf(
			"unsupported executor identity-basis kind %q",
			dto.Kind,
		)
	}
	if dto.OperatorDesignated == nil || dto.KernelOwned != nil {
		return ProfileOnboardingExecutorIdentityBasisV1{}, fmt.Errorf(
			"operator-designated executor identity basis must contain only operator_designated content",
		)
	}
	designationRef, err := NewProfileOnboardingSystemIdentityBasisRefV1(
		dto.OperatorDesignated.Ref,
	)
	if err != nil {
		return ProfileOnboardingExecutorIdentityBasisV1{}, err
	}
	designationDigest, err := NewContentDigest(dto.OperatorDesignated.Digest)
	if err != nil {
		return ProfileOnboardingExecutorIdentityBasisV1{}, err
	}
	return NewProfileOnboardingOperatorDesignatedExecutorIdentityBasisV1(
		systemRef,
		designationRef,
		designationDigest,
	)
}

func profileAuthorRoleAdmissionToJSONV1(
	value ProfileAuthorRoleAdmissionV1,
) profileAuthorRoleAdmissionJSONV1 {
	ref := value.ref.String()
	roleRef := value.roleRef.String()
	contextRef := value.boundedContextRef.String()
	patternRef := value.governingPatternRef.String()
	descriptionRef := value.methodDescriptionRef.String()
	descriptionDigest := value.methodDescriptionDigest.String()
	contractRef := value.methodContractRef.String()
	contractDigest := value.methodContractDigest.String()
	policyRef := value.roleAdmissionPolicyRef.String()
	schema, _ := profileAuthorRoleAdmissionSchema(value.methodContractRef)
	return profileAuthorRoleAdmissionJSONV1{
		Schema:                  schema,
		Ref:                     ref,
		RoleRef:                 roleRef,
		BoundedContextRef:       contextRef,
		GoverningPatternRef:     patternRef,
		MethodDescriptionRef:    descriptionRef,
		MethodDescriptionDigest: descriptionDigest,
		MethodContractRef:       contractRef,
		MethodContractDigest:    contractDigest,
		RoleAdmissionPolicyRef:  policyRef,
	}
}

func profileAuthorRoleAdmissionFromJSONV1(
	dto profileAuthorRoleAdmissionJSONV1,
) (ProfileAuthorRoleAdmissionV1, error) {
	ref, err := NewRoleAdmissionRef(dto.Ref)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	roleRef, err := NewRoleRef(dto.RoleRef)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	contextRef, err := NewBoundedContextRef(dto.BoundedContextRef)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	patternRef, err := NewSourceUnitRef(dto.GoverningPatternRef)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	descriptionRef, err := NewMethodDescriptionRef(dto.MethodDescriptionRef)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	descriptionDigest, err := NewContentDigest(dto.MethodDescriptionDigest)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	contractDigest, err := NewContentDigest(dto.MethodContractDigest)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	contractRef, err := profileOnboardingMethodContractRefFromString(dto.MethodContractRef)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	expectedSchema, err := profileAuthorRoleAdmissionSchema(contractRef)
	if err != nil {
		return ProfileAuthorRoleAdmissionV1{}, err
	}
	if dto.Schema != expectedSchema {
		return ProfileAuthorRoleAdmissionV1{}, fmt.Errorf("ProfileAuthor role-admission schema does not match its method edition")
	}
	value := ProfileAuthorRoleAdmissionV1{
		ref:                     ref,
		roleRef:                 roleRef,
		boundedContextRef:       contextRef,
		governingPatternRef:     patternRef,
		methodDescriptionRef:    descriptionRef,
		methodDescriptionDigest: descriptionDigest,
		methodContractRef:       contractRef,
		methodContractDigest:    contractDigest,
		roleAdmissionPolicyRef:  ProfileOnboardingAdmissionPolicyRefV1{value: dto.RoleAdmissionPolicyRef},
	}
	return canonicalProfileAuthorRoleAdmissionV1(value)
}

func profileAuthorAssignmentJustificationToJSONV1(
	value ProfileAuthorAssignmentJustificationV1,
) profileAuthorAssignmentJustificationJSONV1 {
	ref := value.ref.String()
	ruleRef := value.rule.Ref()
	ruleRefValue := ruleRef.String()
	ruleStatement := value.rule.Statement()
	contextRef := value.boundedContextRef.String()
	systemRef := value.systemAdmissionRef.String()
	systemDigest := value.systemAdmissionDigest.String()
	roleRef := value.roleAdmissionRef.String()
	roleDigest := value.roleAdmissionDigest.String()
	assignmentWindow := closedIntervalToJSONV1(value.assignmentWindow.closedIntervalV1)
	contractRef := value.methodContractRef.String()
	contractDigest := value.methodContractDigest.String()
	schema, _ := profileAuthorAssignmentJustificationSchema(value.methodContractRef)
	return profileAuthorAssignmentJustificationJSONV1{
		Schema:                schema,
		Ref:                   ref,
		RuleRef:               ruleRefValue,
		RuleStatement:         ruleStatement,
		BoundedContextRef:     contextRef,
		SystemAdmissionRef:    systemRef,
		SystemAdmissionDigest: systemDigest,
		RoleAdmissionRef:      roleRef,
		RoleAdmissionDigest:   roleDigest,
		AssignmentWindow:      assignmentWindow,
		MethodContractRef:     contractRef,
		MethodContractDigest:  contractDigest,
	}
}

func profileAuthorAssignmentJustificationFromJSONV1(
	dto profileAuthorAssignmentJustificationJSONV1,
) (ProfileAuthorAssignmentJustificationV1, error) {
	ref, err := NewRoleAssignmentJustificationRef(dto.Ref)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	contextRef, err := NewBoundedContextRef(dto.BoundedContextRef)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	systemRef, err := NewSystemAdmissionRef(dto.SystemAdmissionRef)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	systemDigest, err := NewContentDigest(dto.SystemAdmissionDigest)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	roleRef, err := NewRoleAdmissionRef(dto.RoleAdmissionRef)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	roleDigest, err := NewContentDigest(dto.RoleAdmissionDigest)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	windowValue, err := closedIntervalFromJSONV1("ProfileAuthor assignment window", dto.AssignmentWindow)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	contractDigest, err := NewContentDigest(dto.MethodContractDigest)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	contractRef, err := profileOnboardingMethodContractRefFromString(dto.MethodContractRef)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	expectedSchema, err := profileAuthorAssignmentJustificationSchema(contractRef)
	if err != nil {
		return ProfileAuthorAssignmentJustificationV1{}, err
	}
	if dto.Schema != expectedSchema {
		return ProfileAuthorAssignmentJustificationV1{}, fmt.Errorf("ProfileAuthor assignment-justification schema does not match its method edition")
	}
	value := ProfileAuthorAssignmentJustificationV1{
		ref: ref,
		rule: ProfileAuthorAssignmentAdmissionRuleV1{
			ref:       ProfileOnboardingLocalRuleRefV1{value: dto.RuleRef},
			statement: dto.RuleStatement,
		},
		boundedContextRef:     contextRef,
		systemAdmissionRef:    systemRef,
		systemAdmissionDigest: systemDigest,
		roleAdmissionRef:      roleRef,
		roleAdmissionDigest:   roleDigest,
		assignmentWindow:      RoleAssignmentWindowV1{closedIntervalV1: windowValue},
		methodContractRef:     contractRef,
		methodContractDigest:  contractDigest,
	}
	return canonicalProfileAuthorAssignmentJustificationV1(value)
}

func profileOnboardingExecutorSystemAdmissionSchema(
	ref ProfileOnboardingMethodContractRef,
) (string, error) {
	return profileOnboardingMethodEditionValue(
		ref,
		profileOnboardingExecutorSystemAdmissionJSONSchemaV1,
		profileOnboardingExecutorSystemAdmissionJSONSchemaV2,
	)
}

func profileAuthorRoleAdmissionSchema(
	ref ProfileOnboardingMethodContractRef,
) (string, error) {
	return profileOnboardingMethodEditionValue(
		ref,
		profileAuthorRoleAdmissionJSONSchemaV1,
		profileAuthorRoleAdmissionJSONSchemaV2,
	)
}

func profileAuthorAssignmentJustificationSchema(
	ref ProfileOnboardingMethodContractRef,
) (string, error) {
	return profileOnboardingMethodEditionValue(
		ref,
		profileAuthorAssignmentJustificationJSONSchemaV1,
		profileAuthorAssignmentJustificationJSONSchemaV2,
	)
}

func profileOnboardingExecutorSystemAdmissionDigestDomain(
	ref ProfileOnboardingMethodContractRef,
) (string, error) {
	return profileOnboardingMethodEditionValue(
		ref,
		profileOnboardingExecutorSystemAdmissionDigestDomainV1,
		profileOnboardingExecutorSystemAdmissionDigestDomainV2,
	)
}

func profileAuthorRoleAdmissionDigestDomain(
	ref ProfileOnboardingMethodContractRef,
) (string, error) {
	return profileOnboardingMethodEditionValue(
		ref,
		profileAuthorRoleAdmissionDigestDomainV1,
		profileAuthorRoleAdmissionDigestDomainV2,
	)
}

func profileAuthorAssignmentJustificationDigestDomain(
	ref ProfileOnboardingMethodContractRef,
) (string, error) {
	return profileOnboardingMethodEditionValue(
		ref,
		profileAuthorAssignmentJustificationDigestDomainV1,
		profileAuthorAssignmentJustificationDigestDomainV2,
	)
}

func profileOnboardingMethodEditionValue(
	ref ProfileOnboardingMethodContractRef,
	v1 string,
	v2 string,
) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("profile-onboarding MethodContract ref is required")
	}
	switch ref.String() {
	case profileOnboardingMethodContractRefV1Value:
		return v1, nil
	case profileOnboardingMethodContractRefV2Value:
		return v2, nil
	default:
		return "", fmt.Errorf("unknown profile-onboarding MethodContract edition")
	}
}

func profileAuthorAssignmentProvenanceToJSONV1(
	value ProfileAuthorAssignmentProvenanceV1,
) profileAuthorAssignmentProvenanceJSONV1 {
	ref := value.ref.String()
	justificationRef := value.justificationRef.String()
	justificationDigest := value.justificationDigest.String()
	sessionRef := value.sessionRef.String()
	kernelIdentity := value.kernel.Identity()
	kernelVersion := value.kernel.Version()
	kernelJSON := profileOnboardingRuntimeIdentityJSONV1{
		Identity: kernelIdentity,
		Version:  kernelVersion,
	}
	runtimeIdentity := value.runtime.Identity()
	runtimeVersion := value.runtime.Version()
	runtimeJSON := profileOnboardingRuntimeIdentityJSONV1{
		Identity: runtimeIdentity,
		Version:  runtimeVersion,
	}
	recordedAt := canonicalTime(value.recordedAt)
	return profileAuthorAssignmentProvenanceJSONV1{
		Schema:              profileAuthorAssignmentProvenanceJSONSchemaV1,
		Ref:                 ref,
		JustificationRef:    justificationRef,
		JustificationDigest: justificationDigest,
		SessionRef:          sessionRef,
		Kernel:              kernelJSON,
		Runtime:             runtimeJSON,
		RecordedAt:          recordedAt,
	}
}

func profileAuthorAssignmentProvenanceFromJSONV1(
	dto profileAuthorAssignmentProvenanceJSONV1,
) (ProfileAuthorAssignmentProvenanceV1, error) {
	if dto.Schema != profileAuthorAssignmentProvenanceJSONSchemaV1 {
		return ProfileAuthorAssignmentProvenanceV1{}, fmt.Errorf("unsupported ProfileAuthor assignment-provenance schema %q", dto.Schema)
	}
	ref, err := NewRoleAssignmentProvenanceRef(dto.Ref)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	justificationRef, err := NewRoleAssignmentJustificationRef(dto.JustificationRef)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	justificationDigest, err := NewContentDigest(dto.JustificationDigest)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	sessionRef, err := NewSessionRef(dto.SessionRef)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	kernel, err := NewProfileOnboardingKernelIdentityV1(dto.Kernel.Identity, dto.Kernel.Version)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	runtime, err := NewProfileOnboardingRuntimeIdentityV1(dto.Runtime.Identity, dto.Runtime.Version)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	recordedAt, err := parseCanonicalTimeV1("assignment provenance recorded_at", dto.RecordedAt)
	if err != nil {
		return ProfileAuthorAssignmentProvenanceV1{}, err
	}
	value := ProfileAuthorAssignmentProvenanceV1{
		ref:                 ref,
		justificationRef:    justificationRef,
		justificationDigest: justificationDigest,
		sessionRef:          sessionRef,
		kernel:              kernel,
		runtime:             runtime,
		recordedAt:          recordedAt,
	}
	return canonicalProfileAuthorAssignmentProvenanceV1(value)
}

// ProfileAuthorAssignmentSupportCarrierV1 stores four separate canonical
// representations and four separate digests. It has no aggregate ref, schema,
// or digest and therefore does not turn the support chain into one ontology
// object.
type ProfileAuthorAssignmentSupportCarrierV1 struct {
	systemAdmission       ProfileOnboardingExecutorSystemAdmissionV1
	systemAdmissionJSON   []byte
	systemAdmissionDigest ContentDigest
	roleAdmission         ProfileAuthorRoleAdmissionV1
	roleAdmissionJSON     []byte
	roleAdmissionDigest   ContentDigest
	justification         ProfileAuthorAssignmentJustificationV1
	justificationJSON     []byte
	justificationDigest   ContentDigest
	provenance            ProfileAuthorAssignmentProvenanceV1
	provenanceJSON        []byte
	provenanceDigest      ContentDigest
}

func CarryProfileAuthorAssignmentSupportV1(
	systemAdmission ProfileOnboardingExecutorSystemAdmissionV1,
	roleAdmission ProfileAuthorRoleAdmissionV1,
	justification ProfileAuthorAssignmentJustificationV1,
	provenance ProfileAuthorAssignmentProvenanceV1,
) (ProfileAuthorAssignmentSupportCarrierV1, error) {
	err := validateProfileAuthorAssignmentSupportChainV1(
		systemAdmission,
		roleAdmission,
		justification,
		provenance,
	)
	if err != nil {
		return ProfileAuthorAssignmentSupportCarrierV1{}, err
	}
	systemJSON, err := EncodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(systemAdmission)
	if err != nil {
		return ProfileAuthorAssignmentSupportCarrierV1{}, err
	}
	systemDigest, err := DigestProfileOnboardingExecutorSystemAdmissionV1(systemAdmission)
	if err != nil {
		return ProfileAuthorAssignmentSupportCarrierV1{}, err
	}
	roleJSON, err := EncodeProfileAuthorRoleAdmissionV1CanonicalJSON(roleAdmission)
	if err != nil {
		return ProfileAuthorAssignmentSupportCarrierV1{}, err
	}
	roleDigest, err := DigestProfileAuthorRoleAdmissionV1(roleAdmission)
	if err != nil {
		return ProfileAuthorAssignmentSupportCarrierV1{}, err
	}
	justificationJSON, err := EncodeProfileAuthorAssignmentJustificationV1CanonicalJSON(justification)
	if err != nil {
		return ProfileAuthorAssignmentSupportCarrierV1{}, err
	}
	justificationDigest, err := DigestProfileAuthorAssignmentJustificationV1(justification)
	if err != nil {
		return ProfileAuthorAssignmentSupportCarrierV1{}, err
	}
	provenanceJSON, err := EncodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(provenance)
	if err != nil {
		return ProfileAuthorAssignmentSupportCarrierV1{}, err
	}
	provenanceDigest, err := DigestProfileAuthorAssignmentProvenanceV1(provenance)
	if err != nil {
		return ProfileAuthorAssignmentSupportCarrierV1{}, err
	}
	storedSystemJSON := bytes.Clone(systemJSON)
	storedRoleJSON := bytes.Clone(roleJSON)
	storedJustificationJSON := bytes.Clone(justificationJSON)
	storedProvenanceJSON := bytes.Clone(provenanceJSON)
	return ProfileAuthorAssignmentSupportCarrierV1{
		systemAdmission:       systemAdmission,
		systemAdmissionJSON:   storedSystemJSON,
		systemAdmissionDigest: systemDigest,
		roleAdmission:         roleAdmission,
		roleAdmissionJSON:     storedRoleJSON,
		roleAdmissionDigest:   roleDigest,
		justification:         justification,
		justificationJSON:     storedJustificationJSON,
		justificationDigest:   justificationDigest,
		provenance:            provenance,
		provenanceJSON:        storedProvenanceJSON,
		provenanceDigest:      provenanceDigest,
	}, nil
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) exactValues() (
	ProfileOnboardingExecutorSystemAdmissionV1,
	ProfileAuthorRoleAdmissionV1,
	ProfileAuthorAssignmentJustificationV1,
	ProfileAuthorAssignmentProvenanceV1,
	error,
) {
	err := validateProfileAuthorAssignmentSupportChainV1(
		carrier.systemAdmission,
		carrier.roleAdmission,
		carrier.justification,
		carrier.provenance,
	)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{},
			ProfileAuthorRoleAdmissionV1{},
			ProfileAuthorAssignmentJustificationV1{},
			ProfileAuthorAssignmentProvenanceV1{},
			err
	}
	systemDigest, err := DigestProfileOnboardingExecutorSystemAdmissionV1(carrier.systemAdmission)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{},
			ProfileAuthorRoleAdmissionV1{},
			ProfileAuthorAssignmentJustificationV1{},
			ProfileAuthorAssignmentProvenanceV1{},
			err
	}
	roleDigest, err := DigestProfileAuthorRoleAdmissionV1(carrier.roleAdmission)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{},
			ProfileAuthorRoleAdmissionV1{},
			ProfileAuthorAssignmentJustificationV1{},
			ProfileAuthorAssignmentProvenanceV1{},
			err
	}
	justificationDigest, err := DigestProfileAuthorAssignmentJustificationV1(carrier.justification)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{},
			ProfileAuthorRoleAdmissionV1{},
			ProfileAuthorAssignmentJustificationV1{},
			ProfileAuthorAssignmentProvenanceV1{},
			err
	}
	provenanceDigest, err := DigestProfileAuthorAssignmentProvenanceV1(carrier.provenance)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{},
			ProfileAuthorRoleAdmissionV1{},
			ProfileAuthorAssignmentJustificationV1{},
			ProfileAuthorAssignmentProvenanceV1{},
			err
	}
	systemJSON, err := EncodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(carrier.systemAdmission)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{},
			ProfileAuthorRoleAdmissionV1{},
			ProfileAuthorAssignmentJustificationV1{},
			ProfileAuthorAssignmentProvenanceV1{},
			err
	}
	roleJSON, err := EncodeProfileAuthorRoleAdmissionV1CanonicalJSON(carrier.roleAdmission)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{},
			ProfileAuthorRoleAdmissionV1{},
			ProfileAuthorAssignmentJustificationV1{},
			ProfileAuthorAssignmentProvenanceV1{},
			err
	}
	justificationJSON, err := EncodeProfileAuthorAssignmentJustificationV1CanonicalJSON(carrier.justification)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{},
			ProfileAuthorRoleAdmissionV1{},
			ProfileAuthorAssignmentJustificationV1{},
			ProfileAuthorAssignmentProvenanceV1{},
			err
	}
	provenanceJSON, err := EncodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(carrier.provenance)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{},
			ProfileAuthorRoleAdmissionV1{},
			ProfileAuthorAssignmentJustificationV1{},
			ProfileAuthorAssignmentProvenanceV1{},
			err
	}
	systemJSONMatches := bytes.Equal(systemJSON, carrier.systemAdmissionJSON)
	roleJSONMatches := bytes.Equal(roleJSON, carrier.roleAdmissionJSON)
	justificationJSONMatches := bytes.Equal(justificationJSON, carrier.justificationJSON)
	provenanceJSONMatches := bytes.Equal(provenanceJSON, carrier.provenanceJSON)
	checks := []profileAuthorAssignmentSupportCheckV1{
		{valid: systemDigest == carrier.systemAdmissionDigest, reason: "carrier system-admission digest does not match its exact object"},
		{valid: roleDigest == carrier.roleAdmissionDigest, reason: "carrier role-admission digest does not match its exact object"},
		{valid: justificationDigest == carrier.justificationDigest, reason: "carrier justification digest does not match its exact object"},
		{valid: provenanceDigest == carrier.provenanceDigest, reason: "carrier provenance digest does not match its exact object"},
		{valid: systemJSONMatches, reason: "carrier system-admission JSON does not match its exact object"},
		{valid: roleJSONMatches, reason: "carrier role-admission JSON does not match its exact object"},
		{valid: justificationJSONMatches, reason: "carrier justification JSON does not match its exact object"},
		{valid: provenanceJSONMatches, reason: "carrier provenance JSON does not match its exact object"},
	}
	err = visitSliceV1(checks, validateProfileAuthorAssignmentSupportCheckV1)
	if err != nil {
		return ProfileOnboardingExecutorSystemAdmissionV1{},
			ProfileAuthorRoleAdmissionV1{},
			ProfileAuthorAssignmentJustificationV1{},
			ProfileAuthorAssignmentProvenanceV1{},
			err
	}
	return carrier.systemAdmission,
		carrier.roleAdmission,
		carrier.justification,
		carrier.provenance,
		nil
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) SystemAdmission() ProfileOnboardingExecutorSystemAdmissionV1 {
	return carrier.systemAdmission
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) RoleAdmission() ProfileAuthorRoleAdmissionV1 {
	return carrier.roleAdmission
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) Justification() ProfileAuthorAssignmentJustificationV1 {
	return carrier.justification
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) Provenance() ProfileAuthorAssignmentProvenanceV1 {
	return carrier.provenance
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) SystemAdmissionCanonicalJSON() []byte {
	return bytes.Clone(carrier.systemAdmissionJSON)
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) SystemAdmissionDigest() ContentDigest {
	return carrier.systemAdmissionDigest
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) RoleAdmissionCanonicalJSON() []byte {
	return bytes.Clone(carrier.roleAdmissionJSON)
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) RoleAdmissionDigest() ContentDigest {
	return carrier.roleAdmissionDigest
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) JustificationCanonicalJSON() []byte {
	return bytes.Clone(carrier.justificationJSON)
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) JustificationDigest() ContentDigest {
	return carrier.justificationDigest
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) ProvenanceCanonicalJSON() []byte {
	return bytes.Clone(carrier.provenanceJSON)
}

func (carrier ProfileAuthorAssignmentSupportCarrierV1) ProvenanceDigest() ContentDigest {
	return carrier.provenanceDigest
}
