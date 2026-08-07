package projectprofile

import (
	"bytes"
	"fmt"
	"strconv"
	"time"
)

const (
	profileDeclarationAdmissionRequestJSONSchemaV1 = "haft.project-profile.declaration-admission-request/v1"
	profileDeclarationAdmissionRequestDigestV1     = "haft.project-profile.declaration-admission-request/v1"
)

// PreparedProfileAdmissionV1 is package-owned non-binding request material.
// It contains no Declared profile, receipt, admission record, committed
// revision, commit time, or admission-record identity. It is not authority and
// it does not prove that a transaction started or committed.
//
// The unexported marker and concrete implementation keep raw JSON, caller-made
// digests, and foreign implementations out of this boundary. Validation still
// rejects the Go interface-embedding trick explicitly by requiring the exact
// package-owned concrete value.
type PreparedProfileAdmissionV1 interface {
	CommitPlan() ProfileDeclarationCommitPlan
	ProjectRoot() ProjectRootV1
	WorkRecord() ProfileOnboardingWorkRecord
	MethodDescription() ProfileOnboardingMethodDescriptionV1
	MethodContract() ProfileOnboardingMethodContractV1
	MethodDescriptionEdition() ProfileOnboardingMethodDescriptionEdition
	MethodContractEdition() ProfileOnboardingMethodContractEdition
	MethodDescriptionV2() (ProfileOnboardingMethodDescriptionV2, bool)
	MethodContractV2() (ProfileOnboardingMethodContractV2, bool)
	ProfileAuthorRoleAssignment() ProfileAuthorRoleAssignmentV1
	ExecutorSystemAdmission() ProfileOnboardingExecutorSystemAdmissionV1
	ProfileAuthorRoleAdmission() ProfileAuthorRoleAdmissionV1
	AssignmentJustification() ProfileAuthorAssignmentJustificationV1
	AssignmentProvenance() ProfileAuthorAssignmentProvenanceV1
	ObservedProjectBasis() ObservedProjectBasisV1
	OnboardingEffect() ProfileOnboardingEffectV1
	OutcomeAssessment() ProfileOnboardingOutcomeAssessmentV1
	ExpectedLedgerRevision() LedgerRevision
	AdmissionRequestCanonicalJSON() []byte
	AdmissionRequestDigest() ContentDigest
	ProfilePayloadCanonicalJSON() []byte
	ProfilePayloadDigest() ContentDigest
	CandidateProvenanceCanonicalJSON() []byte
	CandidateProvenanceDigest() ContentDigest
	WorkRecordCanonicalJSON() []byte
	WorkRecordDigest() ContentDigest
	MethodDescriptionCanonicalJSON() []byte
	MethodDescriptionDigest() ContentDigest
	MethodContractCanonicalJSON() []byte
	MethodContractDigest() ContentDigest
	ProfileAuthorRoleAssignmentCanonicalJSON() []byte
	ProfileAuthorRoleAssignmentDigest() ContentDigest
	ExecutorSystemAdmissionCanonicalJSON() []byte
	ExecutorSystemAdmissionDigest() ContentDigest
	ProfileAuthorRoleAdmissionCanonicalJSON() []byte
	ProfileAuthorRoleAdmissionDigest() ContentDigest
	AssignmentJustificationCanonicalJSON() []byte
	AssignmentJustificationDigest() ContentDigest
	AssignmentProvenanceCanonicalJSON() []byte
	AssignmentProvenanceDigest() ContentDigest
	ObservedProjectBasisCanonicalJSON() []byte
	ObservedProjectBasisDigest() ContentDigest
	OnboardingEffectCanonicalJSON() []byte
	OnboardingEffectDigest() ContentDigest
	OutcomeAssessmentCanonicalJSON() []byte
	OutcomeAssessmentDigest() ContentDigest
	preparedProfileAdmissionV1Variant()
}

type profileDeclarationAdmissionRequestJSONV1 struct {
	Schema                          string                                         `json:"schema"`
	ProjectRoot                     string                                         `json:"project_root"`
	Candidate                       profileDeclarationCandidateJSONV1              `json:"candidate"`
	WorkRecord                      profileOnboardingWorkRecordJSONV1              `json:"work_record"`
	MethodDescription               profileOnboardingMethodDescriptionJSONV1       `json:"method_description"`
	MethodContract                  profileOnboardingMethodContractJSONV1          `json:"method_contract"`
	ProfileAuthorRoleAssignment     profileAuthorRoleAssignmentJSONV1              `json:"profile_author_role_assignment"`
	ExecutorSystemAdmission         profileOnboardingExecutorSystemAdmissionJSONV1 `json:"executor_system_admission"`
	ProfileAuthorRoleAdmission      profileAuthorRoleAdmissionJSONV1               `json:"profile_author_role_admission"`
	AssignmentJustification         profileAuthorAssignmentJustificationJSONV1     `json:"assignment_justification"`
	AssignmentProvenance            profileAuthorAssignmentProvenanceJSONV1        `json:"assignment_provenance"`
	ObservedProjectBasis            observedProjectBasisJSONV1                     `json:"observed_project_basis"`
	OnboardingEffect                profileOnboardingEffectJSONV1                  `json:"onboarding_effect"`
	OutcomeAssessment               profileOnboardingOutcomeAssessmentJSONV1       `json:"outcome_assessment"`
	AuthorityResolutionRecordRef    string                                         `json:"authority_resolution_record_ref"`
	AuthorityResolutionRecordDigest string                                         `json:"authority_resolution_record_digest"`
	SingleUseKey                    string                                         `json:"single_use_key"`
}

type preparedProfileAdmissionV1 struct {
	plan                                     ProfileDeclarationCommitPlan
	projectRoot                              ProjectRootV1
	workRecord                               ProfileOnboardingWorkRecord
	methodDescription                        ProfileOnboardingMethodDescriptionEdition
	methodContract                           ProfileOnboardingMethodContractEdition
	profileAuthorRoleAssignment              ProfileAuthorRoleAssignmentV1
	profileAuthorAssignmentSupport           ProfileAuthorAssignmentSupportCarrierV1
	observedProjectBasis                     observedProjectBasisV1
	onboardingEffect                         profileOnboardingEffectV1
	outcomeAssessment                        profileOnboardingOutcomeAssessmentV1
	admissionRequestCanonicalJSON            []byte
	admissionRequestDigest                   ContentDigest
	profilePayloadCanonicalJSON              []byte
	profilePayloadDigest                     ContentDigest
	candidateProvenanceJSON                  []byte
	candidateProvenanceDigest                ContentDigest
	workRecordCanonicalJSON                  []byte
	workRecordDigest                         ContentDigest
	methodDescriptionCanonicalJSON           []byte
	methodDescriptionDigest                  ContentDigest
	methodContractCanonicalJSON              []byte
	methodContractDigest                     ContentDigest
	profileAuthorRoleAssignmentCanonicalJSON []byte
	profileAuthorRoleAssignmentDigest        ContentDigest
	observedProjectBasisCanonicalJSON        []byte
	observedProjectBasisDigest               ContentDigest
	onboardingEffectCanonicalJSON            []byte
	onboardingEffectDigest                   ContentDigest
	outcomeAssessmentCanonicalJSON           []byte
	outcomeAssessmentDigest                  ContentDigest
}

func (preparedProfileAdmissionV1) preparedProfileAdmissionV1Variant() {}

type profileAdmissionPreparationInputV1 struct {
	plan                           ProfileDeclarationCommitPlan
	workRecord                     ProfileOnboardingWorkRecord
	methodDescription              ProfileOnboardingMethodDescriptionEdition
	methodContract                 ProfileOnboardingMethodContractEdition
	profileAuthorRoleAssignment    ProfileAuthorRoleAssignmentV1
	profileAuthorAssignmentSupport ProfileAuthorAssignmentSupportCarrierV1
	observedProjectBasis           ObservedProjectBasisV1
	onboardingEffect               ProfileOnboardingEffectV1
	outcomeAssessment              ProfileOnboardingOutcomeAssessmentV1
	projectRoot                    ProjectRootV1
}

// ProfileAdmissionPreparationV1Builder is the only public constructor for
// pre-commit final-v1 request material. Each method supplies one coherent
// reliance fragment; Build validates the complete exact-support closure.
// Callers cannot provide serialized material, digests, or defaulted values.
type ProfileAdmissionPreparationV1Builder struct {
	input profileAdmissionPreparationInputV1
}

// NewProfileAdmissionPreparationV1Builder starts an exact preparation request
// from its commit intent and project boundary. All three support fragments are
// still required before Build can succeed.
func NewProfileAdmissionPreparationV1Builder(
	plan ProfileDeclarationCommitPlan,
	projectRoot ProjectRootV1,
) ProfileAdmissionPreparationV1Builder {
	input := profileAdmissionPreparationInputV1{}
	input.plan = plan
	input.projectRoot = projectRoot
	return ProfileAdmissionPreparationV1Builder{input: input}
}

// WithWork supplies performed Work and the exact method carriers it enacts.
func (builder ProfileAdmissionPreparationV1Builder) WithWork(
	record ProfileOnboardingWorkRecord,
	description ProfileOnboardingMethodDescriptionV1,
	contract ProfileOnboardingMethodContractV1,
) ProfileAdmissionPreparationV1Builder {
	builder.input.workRecord = record
	builder.input.methodDescription = description
	builder.input.methodContract = contract
	return builder
}

func (builder ProfileAdmissionPreparationV1Builder) WithWorkV2(
	record ProfileOnboardingWorkRecord,
	description ProfileOnboardingMethodDescriptionV2,
	contract ProfileOnboardingMethodContractV2,
) ProfileAdmissionPreparationV1Builder {
	builder.input.workRecord = record
	builder.input.methodDescription = description
	builder.input.methodContract = contract
	return builder
}

// WithProfileAuthor supplies the exact assignment and its support closure.
func (builder ProfileAdmissionPreparationV1Builder) WithProfileAuthor(
	assignment ProfileAuthorRoleAssignmentV1,
	support ProfileAuthorAssignmentSupportCarrierV1,
) ProfileAdmissionPreparationV1Builder {
	builder.input.profileAuthorRoleAssignment = assignment
	builder.input.profileAuthorAssignmentSupport = support
	return builder
}

// WithObservedOutcome supplies the observed basis, effect, and assessment.
func (builder ProfileAdmissionPreparationV1Builder) WithObservedOutcome(
	basis ObservedProjectBasisV1,
	effect ProfileOnboardingEffectV1,
	assessment ProfileOnboardingOutcomeAssessmentV1,
) ProfileAdmissionPreparationV1Builder {
	builder.input.observedProjectBasis = basis
	builder.input.onboardingEffect = effect
	builder.input.outcomeAssessment = assessment
	return builder
}

func withPreparedProfileAdmissionMethodEdition(
	builder ProfileAdmissionPreparationV1Builder,
	record ProfileOnboardingWorkRecord,
	description ProfileOnboardingMethodDescriptionEdition,
	contract ProfileOnboardingMethodContractEdition,
) (ProfileAdmissionPreparationV1Builder, error) {
	switch exactDescription := description.(type) {
	case ProfileOnboardingMethodDescriptionV1:
		exactContract, ok := contract.(ProfileOnboardingMethodContractV1)
		if !ok {
			return builder, fmt.Errorf("prepared profile-onboarding method editions differ")
		}
		return builder.WithWork(record, exactDescription, exactContract), nil
	case ProfileOnboardingMethodDescriptionV2:
		exactContract, ok := contract.(ProfileOnboardingMethodContractV2)
		if !ok {
			return builder, fmt.Errorf("prepared profile-onboarding method editions differ")
		}
		return builder.WithWorkV2(record, exactDescription, exactContract), nil
	default:
		return builder, fmt.Errorf("prepared profile-onboarding method edition is unsupported")
	}
}

func validateProfileAdmissionPreparationMethodEdition(
	candidate ProfileDeclarationCandidateV1,
	input profileAdmissionPreparationInputV1,
) error {
	switch description := input.methodDescription.(type) {
	case ProfileOnboardingMethodDescriptionV1:
		contract, ok := input.methodContract.(ProfileOnboardingMethodContractV1)
		if !ok {
			return fmt.Errorf("prepared profile-onboarding method editions differ")
		}
		return ValidateProfileDeclarationCandidateV1AgainstSupports(
			candidate,
			input.workRecord,
			description,
			contract,
			input.profileAuthorRoleAssignment,
			input.profileAuthorAssignmentSupport,
			input.observedProjectBasis,
			input.onboardingEffect,
			input.outcomeAssessment,
		)
	case ProfileOnboardingMethodDescriptionV2:
		contract, ok := input.methodContract.(ProfileOnboardingMethodContractV2)
		if !ok {
			return fmt.Errorf("prepared profile-onboarding method editions differ")
		}
		workInputRef, ok := input.workRecord.ProfileOnboardingWorkInputRefV2()
		if !ok {
			return fmt.Errorf("prepared profile-onboarding Work v2 lacks its exact WorkInput ref")
		}
		return ValidateProfileDeclarationCandidateV1AgainstSupportsV2(
			candidate,
			input.workRecord,
			description,
			contract,
			input.profileAuthorRoleAssignment,
			input.profileAuthorAssignmentSupport,
			input.observedProjectBasis,
			input.onboardingEffect,
			input.outcomeAssessment,
			workInputRef,
		)
	default:
		return fmt.Errorf("prepared profile-onboarding method edition is unsupported")
	}
}

func exactPreparedProfileOnboardingMethodEdition(
	description ProfileOnboardingMethodDescriptionEdition,
	contract ProfileOnboardingMethodContractEdition,
) (
	ProfileOnboardingMethodDescriptionEdition,
	ProfileOnboardingMethodContractEdition,
	error,
) {
	switch exactDescription := description.(type) {
	case ProfileOnboardingMethodDescriptionV1:
		validatedDescription, err := exactProfileOnboardingMethodDescriptionV1(exactDescription)
		if err != nil {
			return nil, nil, err
		}
		exactContract, ok := contract.(ProfileOnboardingMethodContractV1)
		if !ok {
			return nil, nil, fmt.Errorf("prepared profile-onboarding method editions differ")
		}
		validatedContract, err := exactProfileOnboardingMethodContractV1(exactContract)
		return validatedDescription, validatedContract, err
	case ProfileOnboardingMethodDescriptionV2:
		validatedDescription, err := exactProfileOnboardingMethodDescriptionV2(exactDescription)
		if err != nil {
			return nil, nil, err
		}
		exactContract, ok := contract.(ProfileOnboardingMethodContractV2)
		if !ok {
			return nil, nil, fmt.Errorf("prepared profile-onboarding method editions differ")
		}
		validatedContract, err := exactProfileOnboardingMethodContractV2(exactContract)
		return validatedDescription, validatedContract, err
	default:
		return nil, nil, fmt.Errorf("prepared profile-onboarding method edition is unsupported")
	}
}

func encodePreparedMethodDescriptionEdition(
	value ProfileOnboardingMethodDescriptionEdition,
) ([]byte, ContentDigest, error) {
	switch exact := value.(type) {
	case ProfileOnboardingMethodDescriptionV1:
		data, err := EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(exact)
		if err != nil {
			return nil, ContentDigest{}, err
		}
		digest, err := DigestProfileOnboardingMethodDescriptionV1(exact)
		return data, digest, err
	case ProfileOnboardingMethodDescriptionV2:
		data, err := EncodeProfileOnboardingMethodDescriptionV2CanonicalJSON(exact)
		if err != nil {
			return nil, ContentDigest{}, err
		}
		digest, err := DigestProfileOnboardingMethodDescriptionV2(exact)
		return data, digest, err
	default:
		return nil, ContentDigest{}, fmt.Errorf("prepared profile-onboarding MethodDescription edition is unsupported")
	}
}

func encodePreparedMethodContractEdition(
	value ProfileOnboardingMethodContractEdition,
) ([]byte, ContentDigest, error) {
	switch exact := value.(type) {
	case ProfileOnboardingMethodContractV1:
		data, err := EncodeProfileOnboardingMethodContractV1CanonicalJSON(exact)
		if err != nil {
			return nil, ContentDigest{}, err
		}
		digest, err := DigestProfileOnboardingMethodContractV1(exact)
		return data, digest, err
	case ProfileOnboardingMethodContractV2:
		data, err := EncodeProfileOnboardingMethodContractV2CanonicalJSON(exact)
		if err != nil {
			return nil, ContentDigest{}, err
		}
		digest, err := DigestProfileOnboardingMethodContractV2(exact)
		return data, digest, err
	default:
		return nil, ContentDigest{}, fmt.Errorf("prepared profile-onboarding MethodContract edition is unsupported")
	}
}

// Build validates every required support and derives opaque canonical material.
func (builder ProfileAdmissionPreparationV1Builder) Build() (PreparedProfileAdmissionV1, error) {
	prepared, err := buildPreparedProfileAdmissionV1(builder.input)
	return prepared, err
}

// ValidatePreparedProfileAdmissionV1 recomputes every canonical byte and
// digest from the typed non-binding source values. It intentionally
// rejects nil, zero, and foreign interface implementations.
func ValidatePreparedProfileAdmissionV1(prepared PreparedProfileAdmissionV1) error {
	value, ok := prepared.(preparedProfileAdmissionV1)
	if !ok {
		return fmt.Errorf("unknown or externally supplied PreparedProfileAdmissionV1")
	}
	builder := NewProfileAdmissionPreparationV1Builder(value.plan, value.projectRoot)
	builder, err := withPreparedProfileAdmissionMethodEdition(
		builder,
		value.workRecord,
		value.methodDescription,
		value.methodContract,
	)
	if err != nil {
		return err
	}
	builder = builder.WithProfileAuthor(value.profileAuthorRoleAssignment, value.profileAuthorAssignmentSupport)
	builder = builder.WithObservedOutcome(value.observedProjectBasis, value.onboardingEffect, value.outcomeAssessment)
	expected, err := buildPreparedProfileAdmissionV1(builder.input)
	if err != nil {
		return err
	}
	return comparePreparedProfileAdmissionV1(value, expected)
}

func buildPreparedProfileAdmissionV1(
	input profileAdmissionPreparationInputV1,
) (preparedProfileAdmissionV1, error) {
	err := validateProfileDeclarationCommitPlan(input.plan)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	candidate := input.plan.inputs.candidate
	err = validateProfileAdmissionPreparationMethodEdition(candidate, input)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	if !input.projectRoot.valid() {
		return preparedProfileAdmissionV1{}, fmt.Errorf("prepared admission project root is invalid")
	}
	if candidate.provenance.projectRoot != input.projectRoot {
		return preparedProfileAdmissionV1{}, fmt.Errorf("prepared admission project root does not match candidate provenance")
	}
	exactInput, err := exactProfileAdmissionPreparationInputV1From(input)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	prepared, err := derivePreparedProfileAdmissionV1(exactInput)
	return prepared, err
}

func derivePreparedProfileAdmissionV1(
	exactInput exactProfileAdmissionPreparationInputV1,
) (preparedProfileAdmissionV1, error) {
	requestJSON, err := encodeProfileDeclarationAdmissionRequestV1CanonicalJSON(exactInput)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	requestDigest := digestProfileDeclarationAdmissionRequestV1(requestJSON)
	requestMaterial := newPreparedCanonicalMaterialV1(requestJSON, requestDigest)
	payload := exactInput.plan.inputs.candidate.payload
	payloadJSON, err := EncodeProfileDeclarationPayloadCanonicalJSON(payload)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	payloadDigest, err := DigestProfileDeclarationPayload(payload)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	payloadMaterial := newPreparedCanonicalMaterialV1(payloadJSON, payloadDigest)
	provenance := exactInput.plan.inputs.candidate.provenance
	provenanceJSON, err := encodeCandidateProvenanceV1CanonicalJSON(provenance)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	provenanceDigest, err := DigestCandidateProvenanceV1(provenance)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	provenanceMaterial := newPreparedCanonicalMaterialV1(provenanceJSON, provenanceDigest)
	workJSON, err := EncodeProfileOnboardingWorkRecordCanonicalJSON(exactInput.workRecord)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	workDigest, err := DigestProfileOnboardingWorkRecord(exactInput.workRecord)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	workMaterial := newPreparedCanonicalMaterialV1(workJSON, workDigest)
	descriptionJSON, descriptionDigest, err := encodePreparedMethodDescriptionEdition(exactInput.methodDescription)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	descriptionMaterial := newPreparedCanonicalMaterialV1(descriptionJSON, descriptionDigest)
	contractJSON, contractDigest, err := encodePreparedMethodContractEdition(exactInput.methodContract)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	contractMaterial := newPreparedCanonicalMaterialV1(contractJSON, contractDigest)
	assignmentJSON, err := EncodeProfileAuthorRoleAssignmentV1CanonicalJSON(exactInput.profileAuthorRoleAssignment)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	assignmentDigest, err := DigestProfileAuthorRoleAssignmentV1(exactInput.profileAuthorRoleAssignment)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	assignmentMaterial := newPreparedCanonicalMaterialV1(assignmentJSON, assignmentDigest)
	basisJSON, err := EncodeObservedProjectBasisV1CanonicalJSON(exactInput.observedProjectBasis)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	basisDigest, err := DigestObservedProjectBasisV1(exactInput.observedProjectBasis)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	basisMaterial := newPreparedCanonicalMaterialV1(basisJSON, basisDigest)
	effectJSON, err := EncodeProfileOnboardingEffectV1CanonicalJSON(exactInput.onboardingEffect)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	effectDigest, err := DigestProfileOnboardingEffectV1(exactInput.onboardingEffect)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	effectMaterial := newPreparedCanonicalMaterialV1(effectJSON, effectDigest)
	assessmentJSON, err := EncodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(exactInput.outcomeAssessment)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	assessmentDigest, err := DigestProfileOnboardingOutcomeAssessmentV1(exactInput.outcomeAssessment)
	if err != nil {
		return preparedProfileAdmissionV1{}, err
	}
	assessmentMaterial := newPreparedCanonicalMaterialV1(assessmentJSON, assessmentDigest)
	assembler := newPreparedProfileAdmissionV1Assembler(exactInput)
	assembler = assembler.withAdmissionRequest(requestMaterial)
	assembler = assembler.withProfilePayload(payloadMaterial)
	assembler = assembler.withCandidateProvenance(provenanceMaterial)
	assembler = assembler.withWorkRecord(workMaterial)
	assembler = assembler.withMethodDescription(descriptionMaterial)
	assembler = assembler.withMethodContract(contractMaterial)
	assembler = assembler.withProfileAuthorRoleAssignment(assignmentMaterial)
	assembler = assembler.withObservedProjectBasis(basisMaterial)
	assembler = assembler.withOnboardingEffect(effectMaterial)
	assembler = assembler.withOutcomeAssessment(assessmentMaterial)
	prepared := assembler.build()
	return prepared, nil
}

type exactProfileAdmissionPreparationInputV1 struct {
	plan                           ProfileDeclarationCommitPlan
	projectRoot                    ProjectRootV1
	workRecord                     ProfileOnboardingWorkRecord
	methodDescription              ProfileOnboardingMethodDescriptionEdition
	methodContract                 ProfileOnboardingMethodContractEdition
	profileAuthorRoleAssignment    ProfileAuthorRoleAssignmentV1
	profileAuthorAssignmentSupport ProfileAuthorAssignmentSupportCarrierV1
	observedProjectBasis           observedProjectBasisV1
	onboardingEffect               profileOnboardingEffectV1
	outcomeAssessment              profileOnboardingOutcomeAssessmentV1
}

func exactProfileAdmissionPreparationInputV1From(
	input profileAdmissionPreparationInputV1,
) (exactProfileAdmissionPreparationInputV1, error) {
	exact := exactProfileAdmissionPreparationInputV1{}
	exact.plan = input.plan
	exact.projectRoot = input.projectRoot
	exact.workRecord = input.workRecord
	description, contract, err := exactPreparedProfileOnboardingMethodEdition(
		input.methodDescription,
		input.methodContract,
	)
	if err != nil {
		return exactProfileAdmissionPreparationInputV1{}, err
	}
	exact.methodDescription = description
	exact.methodContract = contract
	assignment, err := canonicalProfileAuthorRoleAssignmentV1(input.profileAuthorRoleAssignment)
	if err != nil {
		return exactProfileAdmissionPreparationInputV1{}, err
	}
	exact.profileAuthorRoleAssignment = assignment
	systemAdmission, roleAdmission, justification, provenance, err := input.profileAuthorAssignmentSupport.exactValues()
	if err != nil {
		return exactProfileAdmissionPreparationInputV1{}, err
	}
	assignmentSupport, err := CarryProfileAuthorAssignmentSupportV1(
		systemAdmission,
		roleAdmission,
		justification,
		provenance,
	)
	if err != nil {
		return exactProfileAdmissionPreparationInputV1{}, err
	}
	exact.profileAuthorAssignmentSupport = assignmentSupport
	basis, err := exactObservedProjectBasisV1(input.observedProjectBasis)
	if err != nil {
		return exactProfileAdmissionPreparationInputV1{}, err
	}
	exact.observedProjectBasis = basis
	effect, err := exactProfileOnboardingEffectV1(input.onboardingEffect)
	if err != nil {
		return exactProfileAdmissionPreparationInputV1{}, err
	}
	exact.onboardingEffect = effect
	assessment, err := exactProfileOnboardingOutcomeAssessmentV1(input.outcomeAssessment)
	if err != nil {
		return exactProfileAdmissionPreparationInputV1{}, err
	}
	exact.outcomeAssessment = assessment
	return exact, nil
}

type preparedCanonicalMaterialV1 struct {
	canonicalJSON []byte
	digest        ContentDigest
}

func newPreparedCanonicalMaterialV1(
	canonicalJSON []byte,
	digest ContentDigest,
) preparedCanonicalMaterialV1 {
	return preparedCanonicalMaterialV1{
		canonicalJSON: canonicalJSON,
		digest:        digest,
	}
}

type preparedProfileAdmissionV1Assembler struct {
	value preparedProfileAdmissionV1
}

func newPreparedProfileAdmissionV1Assembler(
	input exactProfileAdmissionPreparationInputV1,
) preparedProfileAdmissionV1Assembler {
	value := preparedProfileAdmissionV1{}
	value.plan = input.plan
	value.projectRoot = input.projectRoot
	value.workRecord = input.workRecord
	value.methodDescription = input.methodDescription
	value.methodContract = input.methodContract
	value.profileAuthorRoleAssignment = input.profileAuthorRoleAssignment
	value.profileAuthorAssignmentSupport = input.profileAuthorAssignmentSupport
	value.observedProjectBasis = input.observedProjectBasis
	value.onboardingEffect = input.onboardingEffect
	value.outcomeAssessment = input.outcomeAssessment
	return preparedProfileAdmissionV1Assembler{value: value}
}

func (assembler preparedProfileAdmissionV1Assembler) withAdmissionRequest(
	material preparedCanonicalMaterialV1,
) preparedProfileAdmissionV1Assembler {
	assembler.value.admissionRequestCanonicalJSON = material.canonicalJSON
	assembler.value.admissionRequestDigest = material.digest
	return assembler
}

func (assembler preparedProfileAdmissionV1Assembler) withProfilePayload(
	material preparedCanonicalMaterialV1,
) preparedProfileAdmissionV1Assembler {
	assembler.value.profilePayloadCanonicalJSON = material.canonicalJSON
	assembler.value.profilePayloadDigest = material.digest
	return assembler
}

func (assembler preparedProfileAdmissionV1Assembler) withCandidateProvenance(
	material preparedCanonicalMaterialV1,
) preparedProfileAdmissionV1Assembler {
	assembler.value.candidateProvenanceJSON = material.canonicalJSON
	assembler.value.candidateProvenanceDigest = material.digest
	return assembler
}

func (assembler preparedProfileAdmissionV1Assembler) withWorkRecord(
	material preparedCanonicalMaterialV1,
) preparedProfileAdmissionV1Assembler {
	assembler.value.workRecordCanonicalJSON = material.canonicalJSON
	assembler.value.workRecordDigest = material.digest
	return assembler
}

func (assembler preparedProfileAdmissionV1Assembler) withMethodDescription(
	material preparedCanonicalMaterialV1,
) preparedProfileAdmissionV1Assembler {
	assembler.value.methodDescriptionCanonicalJSON = material.canonicalJSON
	assembler.value.methodDescriptionDigest = material.digest
	return assembler
}

func (assembler preparedProfileAdmissionV1Assembler) withMethodContract(
	material preparedCanonicalMaterialV1,
) preparedProfileAdmissionV1Assembler {
	assembler.value.methodContractCanonicalJSON = material.canonicalJSON
	assembler.value.methodContractDigest = material.digest
	return assembler
}

func (assembler preparedProfileAdmissionV1Assembler) withProfileAuthorRoleAssignment(
	material preparedCanonicalMaterialV1,
) preparedProfileAdmissionV1Assembler {
	assembler.value.profileAuthorRoleAssignmentCanonicalJSON = material.canonicalJSON
	assembler.value.profileAuthorRoleAssignmentDigest = material.digest
	return assembler
}

func (assembler preparedProfileAdmissionV1Assembler) withObservedProjectBasis(
	material preparedCanonicalMaterialV1,
) preparedProfileAdmissionV1Assembler {
	assembler.value.observedProjectBasisCanonicalJSON = material.canonicalJSON
	assembler.value.observedProjectBasisDigest = material.digest
	return assembler
}

func (assembler preparedProfileAdmissionV1Assembler) withOnboardingEffect(
	material preparedCanonicalMaterialV1,
) preparedProfileAdmissionV1Assembler {
	assembler.value.onboardingEffectCanonicalJSON = material.canonicalJSON
	assembler.value.onboardingEffectDigest = material.digest
	return assembler
}

func (assembler preparedProfileAdmissionV1Assembler) withOutcomeAssessment(
	material preparedCanonicalMaterialV1,
) preparedProfileAdmissionV1Assembler {
	assembler.value.outcomeAssessmentCanonicalJSON = material.canonicalJSON
	assembler.value.outcomeAssessmentDigest = material.digest
	return assembler
}

func (assembler preparedProfileAdmissionV1Assembler) build() preparedProfileAdmissionV1 {
	return assembler.value
}

func validateAdmissionRecordingTimeV1(
	recordedAt time.Time,
	workRecord ProfileOnboardingWorkRecord,
) error {
	if recordedAt.IsZero() {
		return fmt.Errorf("profile admission recording time is required")
	}
	if recordedAt.Before(workRecord.workInterval.until) {
		return fmt.Errorf("profile admission cannot precede completed onboarding Work")
	}
	if recordedAt.Before(workRecord.basisObservationWindow.until) {
		return fmt.Errorf("profile admission cannot precede the complete basis-observation window")
	}
	return nil
}

// TentativeProfileAdmissionTransactionMaterialV1 is transaction-write
// material, not a committed profile or receipt. The authority transaction may
// persist these exact bytes and digests, but no consumer may treat this value
// as Declared. A committed semantic result must be reconstructed from a
// verified durable reread after COMMIT.
type TentativeProfileAdmissionTransactionMaterialV1 interface {
	Prepared() PreparedProfileAdmissionV1
	TentativeReceiptCanonicalJSON() []byte
	TentativeReceiptDigest() ContentDigest
	TentativeAdmissionRecordCanonicalJSON() []byte
	TentativeAdmissionRecordDigest() ContentDigest
	tentativeProfileAdmissionTransactionMaterialV1Variant()
}

type tentativeProfileAdmissionTransactionMaterialV1 struct {
	prepared                     preparedProfileAdmissionV1
	admissionRecordRef           ProfileDeclarationAdmissionRecordRef
	committedLedgerRevision      LedgerRevision
	recordedAt                   time.Time
	receiptCanonicalJSON         []byte
	receiptDigest                ContentDigest
	admissionRecordCanonicalJSON []byte
	admissionRecordDigest        ContentDigest
}

func (tentativeProfileAdmissionTransactionMaterialV1) tentativeProfileAdmissionTransactionMaterialV1Variant() {
}

// PrepareTentativeProfileAdmissionTransactionMaterialV1 performs the pure
// write-material calculation for transaction-owned values. The result remains
// explicitly tentative: this package cannot observe or prove a SQLite COMMIT.
// Only the transaction-owning authority adapter may use it as write material.
func PrepareTentativeProfileAdmissionTransactionMaterialV1(
	prepared PreparedProfileAdmissionV1,
	committedLedgerRevision LedgerRevision,
	recordedAt time.Time,
	admissionRecordRef ProfileDeclarationAdmissionRecordRef,
) (TentativeProfileAdmissionTransactionMaterialV1, error) {
	value, ok := prepared.(preparedProfileAdmissionV1)
	if !ok {
		return nil, fmt.Errorf("unknown or externally supplied PreparedProfileAdmissionV1")
	}
	err := ValidatePreparedProfileAdmissionV1(value)
	if err != nil {
		return nil, err
	}
	return buildTentativeProfileAdmissionTransactionMaterialV1(
		value,
		committedLedgerRevision,
		recordedAt,
		admissionRecordRef,
	)
}

func ValidateTentativeProfileAdmissionTransactionMaterialV1(
	material TentativeProfileAdmissionTransactionMaterialV1,
) error {
	value, ok := material.(tentativeProfileAdmissionTransactionMaterialV1)
	if !ok {
		return fmt.Errorf("unknown or externally supplied TentativeProfileAdmissionTransactionMaterialV1")
	}
	err := ValidatePreparedProfileAdmissionV1(value.prepared)
	if err != nil {
		return err
	}
	expected, err := buildTentativeProfileAdmissionTransactionMaterialV1(
		value.prepared,
		value.committedLedgerRevision,
		value.recordedAt,
		value.admissionRecordRef,
	)
	if err != nil {
		return err
	}
	return compareTentativeProfileAdmissionTransactionMaterialV1(value, expected)
}

func buildTentativeProfileAdmissionTransactionMaterialV1(
	prepared preparedProfileAdmissionV1,
	committedLedgerRevision LedgerRevision,
	recordedAt time.Time,
	admissionRecordRef ProfileDeclarationAdmissionRecordRef,
) (tentativeProfileAdmissionTransactionMaterialV1, error) {
	err := validateAdmissionRecordingTimeV1(recordedAt, prepared.workRecord)
	if err != nil {
		return tentativeProfileAdmissionTransactionMaterialV1{}, err
	}
	if !admissionRecordRef.valid() {
		return tentativeProfileAdmissionTransactionMaterialV1{}, fmt.Errorf("tentative admission-record ref is invalid")
	}
	expectedCommittedRevision, err := prepared.plan.inputs.expectedLedgerRevision.Next()
	if err != nil {
		return tentativeProfileAdmissionTransactionMaterialV1{}, err
	}
	if committedLedgerRevision != expectedCommittedRevision {
		return tentativeProfileAdmissionTransactionMaterialV1{}, fmt.Errorf("tentative committed revision is not the expected next revision")
	}
	canonicalRecordedAt := recordedAt.UTC()
	receiptDTO := tentativeProfileDeclarationReceiptJSONV1(
		prepared,
		committedLedgerRevision,
		canonicalRecordedAt,
	)
	receiptJSON, err := marshalCanonicalJSONV1(receiptDTO)
	if err != nil {
		return tentativeProfileAdmissionTransactionMaterialV1{}, err
	}
	receiptDigest := digestTentativeProfileDeclarationReceiptV1(
		prepared,
		committedLedgerRevision,
		canonicalRecordedAt,
	)
	admissionDTO, err := tentativeProfileDeclarationAdmissionRecordJSONV1(
		prepared,
		admissionRecordRef,
		committedLedgerRevision,
		canonicalRecordedAt,
		receiptDTO,
	)
	if err != nil {
		return tentativeProfileAdmissionTransactionMaterialV1{}, err
	}
	admissionJSON, err := marshalCanonicalJSONV1(admissionDTO)
	if err != nil {
		return tentativeProfileAdmissionTransactionMaterialV1{}, err
	}
	admissionDigest := digestTentativeProfileDeclarationAdmissionRecordV1(
		prepared,
		admissionRecordRef,
		committedLedgerRevision,
		canonicalRecordedAt,
		receiptDigest,
	)
	return tentativeProfileAdmissionTransactionMaterialV1{
		prepared:                     prepared,
		admissionRecordRef:           admissionRecordRef,
		committedLedgerRevision:      committedLedgerRevision,
		recordedAt:                   canonicalRecordedAt,
		receiptCanonicalJSON:         receiptJSON,
		receiptDigest:                receiptDigest,
		admissionRecordCanonicalJSON: admissionJSON,
		admissionRecordDigest:        admissionDigest,
	}, nil
}

func tentativeProfileDeclarationReceiptJSONV1(
	prepared preparedProfileAdmissionV1,
	committedLedgerRevision LedgerRevision,
	recordedAt time.Time,
) profileDeclarationReceiptJSONV1 {
	plan := prepared.plan
	provenance := plan.inputs.candidate.provenance
	authorityResolutionRecordRef := plan.authorityResolutionRecordRef.String()
	authorityResolutionRecordDigest := plan.authorityResolutionRecordDigest.String()
	authorityBasisRef := provenance.authorityBasisRef.String()
	workRecordRef := provenance.workRecordRef.String()
	candidateProvenanceDigest := provenance.candidateProvenanceHash.String()
	payloadDigest := provenance.payloadDigest.String()
	observedBasisDigest := provenance.observedBasisDigest.String()
	ledgerRevision := committedLedgerRevision.Value()
	canonicalRecordedAt := canonicalTime(recordedAt)
	return profileDeclarationReceiptJSONV1{
		Schema:                          profileDeclarationReceiptJSONSchemaV1,
		AuthorityResolutionRecordRef:    authorityResolutionRecordRef,
		AuthorityResolutionRecordDigest: authorityResolutionRecordDigest,
		AuthorityBasisRef:               authorityBasisRef,
		WorkRecordRef:                   workRecordRef,
		CandidateProvenanceDigest:       candidateProvenanceDigest,
		PayloadDigest:                   payloadDigest,
		ObservedBasisDigest:             observedBasisDigest,
		LedgerRevision:                  ledgerRevision,
		RecordedAt:                      canonicalRecordedAt,
	}
}

func tentativeProfileDeclarationAdmissionRecordJSONV1(
	prepared preparedProfileAdmissionV1,
	admissionRecordRef ProfileDeclarationAdmissionRecordRef,
	committedLedgerRevision LedgerRevision,
	recordedAt time.Time,
	receipt profileDeclarationReceiptJSONV1,
) (profileDeclarationAdmissionRecordJSONV1, error) {
	payload, err := profileDeclarationPayloadToJSONV1(prepared.plan.inputs.candidate.payload)
	if err != nil {
		return profileDeclarationAdmissionRecordJSONV1{}, err
	}
	plan := prepared.plan
	provenance := plan.inputs.candidate.provenance
	admissionRecordRefValue := admissionRecordRef.String()
	candidateProvenance := candidateProvenanceToJSONV1(provenance)
	classificationWorkRecordRef := provenance.workRecordRef.String()
	authorityBasisRef := provenance.authorityBasisRef.String()
	authorityResolutionRecordRef := plan.authorityResolutionRecordRef.String()
	authorityResolutionRecordDigest := plan.authorityResolutionRecordDigest.String()
	expectedLedgerRevision := plan.inputs.expectedLedgerRevision.Value()
	committedLedgerRevisionValue := committedLedgerRevision.Value()
	singleUseKey := plan.singleUseKey.String()
	committedAt := canonicalTime(recordedAt)
	return profileDeclarationAdmissionRecordJSONV1{
		Schema:                          profileDeclarationAdmissionRecordJSONSchemaV1,
		AdmissionRecordRef:              admissionRecordRefValue,
		Payload:                         payload,
		CandidateProvenance:             candidateProvenance,
		ClassificationWorkRecordRef:     classificationWorkRecordRef,
		AuthorityBasisRef:               authorityBasisRef,
		AuthorityResolutionRecordRef:    authorityResolutionRecordRef,
		AuthorityResolutionRecordDigest: authorityResolutionRecordDigest,
		Receipt:                         receipt,
		ExpectedLedgerRevision:          expectedLedgerRevision,
		CommittedLedgerRevision:         committedLedgerRevisionValue,
		SingleUseKey:                    singleUseKey,
		CommittedAt:                     committedAt,
	}, nil
}

func digestTentativeProfileDeclarationReceiptV1(
	prepared preparedProfileAdmissionV1,
	committedLedgerRevision LedgerRevision,
	recordedAt time.Time,
) ContentDigest {
	plan := prepared.plan
	provenance := plan.inputs.candidate.provenance
	authorityResolutionRecordRef := plan.authorityResolutionRecordRef.String()
	authorityResolutionRecordDigest := plan.authorityResolutionRecordDigest.String()
	authorityBasisRef := provenance.authorityBasisRef.String()
	workRecordRef := provenance.workRecordRef.String()
	candidateProvenanceDigest := provenance.candidateProvenanceHash.String()
	payloadDigest := provenance.payloadDigest.String()
	observedBasisDigest := provenance.observedBasisDigest.String()
	ledgerRevision := committedLedgerRevision.Value()
	ledgerRevisionText := strconv.FormatUint(ledgerRevision, 10)
	canonicalRecordedAt := canonicalTime(recordedAt)
	writer := newCanonicalDigestWriter(profileReceiptDigestDomainV1)
	writer.add(authorityResolutionRecordRef)
	writer.add(authorityResolutionRecordDigest)
	writer.add(authorityBasisRef)
	writer.add(workRecordRef)
	writer.add(candidateProvenanceDigest)
	writer.add(payloadDigest)
	writer.add(observedBasisDigest)
	writer.add(ledgerRevisionText)
	writer.add(canonicalRecordedAt)
	return writer.digest()
}

func digestTentativeProfileDeclarationAdmissionRecordV1(
	prepared preparedProfileAdmissionV1,
	admissionRecordRef ProfileDeclarationAdmissionRecordRef,
	committedLedgerRevision LedgerRevision,
	recordedAt time.Time,
	receiptDigest ContentDigest,
) ContentDigest {
	plan := prepared.plan
	provenance := plan.inputs.candidate.provenance
	admissionRecordRefValue := admissionRecordRef.String()
	payloadDigest := provenance.payloadDigest.String()
	candidateProvenanceDigest := provenance.candidateProvenanceHash.String()
	workRecordRef := provenance.workRecordRef.String()
	authorityBasisRef := provenance.authorityBasisRef.String()
	authorityResolutionRecordRef := plan.authorityResolutionRecordRef.String()
	authorityResolutionRecordDigest := plan.authorityResolutionRecordDigest.String()
	receiptDigestValue := receiptDigest.String()
	expectedLedgerRevision := plan.inputs.expectedLedgerRevision.Value()
	expectedLedgerRevisionText := strconv.FormatUint(expectedLedgerRevision, 10)
	committedLedgerRevisionValue := committedLedgerRevision.Value()
	committedLedgerRevisionText := strconv.FormatUint(committedLedgerRevisionValue, 10)
	singleUseKey := plan.singleUseKey.String()
	canonicalRecordedAt := canonicalTime(recordedAt)
	writer := newCanonicalDigestWriter(profileAdmissionRecordDigestDomainV1)
	writer.add(admissionRecordRefValue)
	writer.add(payloadDigest)
	writer.add(candidateProvenanceDigest)
	writer.add(workRecordRef)
	writer.add(authorityBasisRef)
	writer.add(authorityResolutionRecordRef)
	writer.add(authorityResolutionRecordDigest)
	writer.add(receiptDigestValue)
	writer.add(expectedLedgerRevisionText)
	writer.add(committedLedgerRevisionText)
	writer.add(singleUseKey)
	writer.add(canonicalRecordedAt)
	return writer.digest()
}

func compareTentativeProfileAdmissionTransactionMaterialV1(
	actual tentativeProfileAdmissionTransactionMaterialV1,
	expected tentativeProfileAdmissionTransactionMaterialV1,
) error {
	receiptJSONMatches := bytes.Equal(actual.receiptCanonicalJSON, expected.receiptCanonicalJSON)
	receiptDigestMatches := actual.receiptDigest == expected.receiptDigest
	admissionRecordJSONMatches := bytes.Equal(actual.admissionRecordCanonicalJSON, expected.admissionRecordCanonicalJSON)
	admissionRecordDigestMatches := actual.admissionRecordDigest == expected.admissionRecordDigest
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: receiptJSONMatches, name: "receipt JSON"},
		{matches: receiptDigestMatches, name: "receipt digest"},
		{matches: admissionRecordJSONMatches, name: "admission-record JSON"},
		{matches: admissionRecordDigestMatches, name: "admission-record digest"},
	}
	return visitSliceV1(checks, func(_ int, check struct {
		matches bool
		name    string
	}) error {
		if !check.matches {
			return fmt.Errorf("tentative profile-admission %s is not canonical", check.name)
		}
		return nil
	})
}

func encodeProfileDeclarationAdmissionRequestV1CanonicalJSON(
	input exactProfileAdmissionPreparationInputV1,
) ([]byte, error) {
	err := validateProfileDeclarationCommitPlan(input.plan)
	if err != nil {
		return nil, err
	}
	candidateDTO, err := profileDeclarationCandidateToJSONV1(input.plan.inputs.candidate)
	if err != nil {
		return nil, err
	}
	workDTO, err := profileOnboardingWorkRecordToJSONV1(input.workRecord)
	if err != nil {
		return nil, err
	}
	systemAdmission, roleAdmission, justification, provenanceSupport, err := input.profileAuthorAssignmentSupport.exactValues()
	if err != nil {
		return nil, err
	}
	systemAdmissionDTO, err := profileOnboardingExecutorSystemAdmissionToJSONV1(systemAdmission)
	if err != nil {
		return nil, err
	}
	effectDTO, err := profileOnboardingEffectToJSONV1(input.onboardingEffect)
	if err != nil {
		return nil, err
	}
	assessmentDTO, err := profileOnboardingOutcomeAssessmentToJSONV1(input.outcomeAssessment)
	if err != nil {
		return nil, err
	}
	projectRootValue := input.projectRoot.String()
	methodDescriptionDTO, methodContractDTO, err := profileOnboardingPreparedMethodToJSON(
		input.methodDescription,
		input.methodContract,
	)
	if err != nil {
		return nil, err
	}
	roleAssignmentDTO := profileAuthorRoleAssignmentToJSONV1(input.profileAuthorRoleAssignment)
	roleAdmissionDTO := profileAuthorRoleAdmissionToJSONV1(roleAdmission)
	justificationDTO := profileAuthorAssignmentJustificationToJSONV1(justification)
	provenanceDTO := profileAuthorAssignmentProvenanceToJSONV1(provenanceSupport)
	observedProjectBasisDTO := observedProjectBasisToJSONV1(input.observedProjectBasis)
	authorityResolutionRecordRef := input.plan.authorityResolutionRecordRef.String()
	authorityResolutionRecordDigest := input.plan.authorityResolutionRecordDigest.String()
	singleUseKey := input.plan.singleUseKey.String()
	dto := profileDeclarationAdmissionRequestJSONV1{
		Schema:                          profileDeclarationAdmissionRequestJSONSchemaV1,
		ProjectRoot:                     projectRootValue,
		Candidate:                       candidateDTO,
		WorkRecord:                      workDTO,
		MethodDescription:               methodDescriptionDTO,
		MethodContract:                  methodContractDTO,
		ProfileAuthorRoleAssignment:     roleAssignmentDTO,
		ExecutorSystemAdmission:         systemAdmissionDTO,
		ProfileAuthorRoleAdmission:      roleAdmissionDTO,
		AssignmentJustification:         justificationDTO,
		AssignmentProvenance:            provenanceDTO,
		ObservedProjectBasis:            observedProjectBasisDTO,
		OnboardingEffect:                effectDTO,
		OutcomeAssessment:               assessmentDTO,
		AuthorityResolutionRecordRef:    authorityResolutionRecordRef,
		AuthorityResolutionRecordDigest: authorityResolutionRecordDigest,
		SingleUseKey:                    singleUseKey,
	}
	return marshalCanonicalJSONV1(dto)
}

func profileOnboardingPreparedMethodToJSON(
	description ProfileOnboardingMethodDescriptionEdition,
	contract ProfileOnboardingMethodContractEdition,
) (
	profileOnboardingMethodDescriptionJSONV1,
	profileOnboardingMethodContractJSONV1,
	error,
) {
	switch exactDescription := description.(type) {
	case profileOnboardingMethodDescriptionV1:
		exactContract, ok := contract.(profileOnboardingMethodContractV1)
		if !ok {
			return profileOnboardingMethodDescriptionJSONV1{}, profileOnboardingMethodContractJSONV1{}, fmt.Errorf("prepared profile-onboarding method editions differ")
		}
		return profileOnboardingMethodDescriptionToJSONV1(exactDescription), profileOnboardingMethodContractToJSONV1(exactContract), nil
	case profileOnboardingMethodDescriptionV2:
		exactContract, ok := contract.(profileOnboardingMethodContractV2)
		if !ok {
			return profileOnboardingMethodDescriptionJSONV1{}, profileOnboardingMethodContractJSONV1{}, fmt.Errorf("prepared profile-onboarding method editions differ")
		}
		return profileOnboardingMethodDescriptionToJSONV2(exactDescription), profileOnboardingMethodContractToJSONV2(exactContract), nil
	default:
		return profileOnboardingMethodDescriptionJSONV1{}, profileOnboardingMethodContractJSONV1{}, fmt.Errorf("prepared profile-onboarding method edition is unsupported")
	}
}

func encodeCandidateProvenanceV1CanonicalJSON(
	provenance CandidateProvenanceV1,
) ([]byte, error) {
	err := validateCandidateProvenanceV1(provenance)
	if err != nil {
		return nil, err
	}
	dto := candidateProvenanceToJSONV1(provenance)
	return marshalCanonicalJSONV1(dto)
}

func digestProfileDeclarationAdmissionRequestV1(canonicalJSON []byte) ContentDigest {
	canonicalJSONText := string(canonicalJSON)
	writer := newCanonicalDigestWriter(profileDeclarationAdmissionRequestDigestV1)
	writer.add(canonicalJSONText)
	return writer.digest()
}

func comparePreparedProfileAdmissionV1(
	actual preparedProfileAdmissionV1,
	expected preparedProfileAdmissionV1,
) error {
	planDigestMatches := actual.plan.digest == expected.plan.digest
	admissionRequestJSONMatches := bytes.Equal(actual.admissionRequestCanonicalJSON, expected.admissionRequestCanonicalJSON)
	admissionRequestDigestMatches := actual.admissionRequestDigest == expected.admissionRequestDigest
	profilePayloadJSONMatches := bytes.Equal(actual.profilePayloadCanonicalJSON, expected.profilePayloadCanonicalJSON)
	profilePayloadDigestMatches := actual.profilePayloadDigest == expected.profilePayloadDigest
	candidateProvenanceJSONMatches := bytes.Equal(actual.candidateProvenanceJSON, expected.candidateProvenanceJSON)
	candidateProvenanceDigestMatches := actual.candidateProvenanceDigest == expected.candidateProvenanceDigest
	workRecordJSONMatches := bytes.Equal(actual.workRecordCanonicalJSON, expected.workRecordCanonicalJSON)
	workRecordDigestMatches := actual.workRecordDigest == expected.workRecordDigest
	methodDescriptionJSONMatches := bytes.Equal(actual.methodDescriptionCanonicalJSON, expected.methodDescriptionCanonicalJSON)
	methodDescriptionDigestMatches := actual.methodDescriptionDigest == expected.methodDescriptionDigest
	methodContractJSONMatches := bytes.Equal(actual.methodContractCanonicalJSON, expected.methodContractCanonicalJSON)
	methodContractDigestMatches := actual.methodContractDigest == expected.methodContractDigest
	roleAssignmentJSONMatches := bytes.Equal(actual.profileAuthorRoleAssignmentCanonicalJSON, expected.profileAuthorRoleAssignmentCanonicalJSON)
	roleAssignmentDigestMatches := actual.profileAuthorRoleAssignmentDigest == expected.profileAuthorRoleAssignmentDigest
	actualSystemAdmissionJSON := actual.profileAuthorAssignmentSupport.SystemAdmissionCanonicalJSON()
	expectedSystemAdmissionJSON := expected.profileAuthorAssignmentSupport.SystemAdmissionCanonicalJSON()
	systemAdmissionJSONMatches := bytes.Equal(actualSystemAdmissionJSON, expectedSystemAdmissionJSON)
	actualSystemAdmissionDigest := actual.profileAuthorAssignmentSupport.SystemAdmissionDigest()
	expectedSystemAdmissionDigest := expected.profileAuthorAssignmentSupport.SystemAdmissionDigest()
	systemAdmissionDigestMatches := actualSystemAdmissionDigest == expectedSystemAdmissionDigest
	actualRoleAdmissionJSON := actual.profileAuthorAssignmentSupport.RoleAdmissionCanonicalJSON()
	expectedRoleAdmissionJSON := expected.profileAuthorAssignmentSupport.RoleAdmissionCanonicalJSON()
	roleAdmissionJSONMatches := bytes.Equal(actualRoleAdmissionJSON, expectedRoleAdmissionJSON)
	actualRoleAdmissionDigest := actual.profileAuthorAssignmentSupport.RoleAdmissionDigest()
	expectedRoleAdmissionDigest := expected.profileAuthorAssignmentSupport.RoleAdmissionDigest()
	roleAdmissionDigestMatches := actualRoleAdmissionDigest == expectedRoleAdmissionDigest
	actualJustificationJSON := actual.profileAuthorAssignmentSupport.JustificationCanonicalJSON()
	expectedJustificationJSON := expected.profileAuthorAssignmentSupport.JustificationCanonicalJSON()
	justificationJSONMatches := bytes.Equal(actualJustificationJSON, expectedJustificationJSON)
	actualJustificationDigest := actual.profileAuthorAssignmentSupport.JustificationDigest()
	expectedJustificationDigest := expected.profileAuthorAssignmentSupport.JustificationDigest()
	justificationDigestMatches := actualJustificationDigest == expectedJustificationDigest
	actualProvenanceJSON := actual.profileAuthorAssignmentSupport.ProvenanceCanonicalJSON()
	expectedProvenanceJSON := expected.profileAuthorAssignmentSupport.ProvenanceCanonicalJSON()
	provenanceJSONMatches := bytes.Equal(actualProvenanceJSON, expectedProvenanceJSON)
	actualProvenanceDigest := actual.profileAuthorAssignmentSupport.ProvenanceDigest()
	expectedProvenanceDigest := expected.profileAuthorAssignmentSupport.ProvenanceDigest()
	provenanceDigestMatches := actualProvenanceDigest == expectedProvenanceDigest
	observedProjectBasisJSONMatches := bytes.Equal(actual.observedProjectBasisCanonicalJSON, expected.observedProjectBasisCanonicalJSON)
	observedProjectBasisDigestMatches := actual.observedProjectBasisDigest == expected.observedProjectBasisDigest
	onboardingEffectJSONMatches := bytes.Equal(actual.onboardingEffectCanonicalJSON, expected.onboardingEffectCanonicalJSON)
	onboardingEffectDigestMatches := actual.onboardingEffectDigest == expected.onboardingEffectDigest
	outcomeAssessmentJSONMatches := bytes.Equal(actual.outcomeAssessmentCanonicalJSON, expected.outcomeAssessmentCanonicalJSON)
	outcomeAssessmentDigestMatches := actual.outcomeAssessmentDigest == expected.outcomeAssessmentDigest
	type canonicalCheckV1 struct {
		matches bool
		name    string
	}
	identityChecks := []canonicalCheckV1{
		{matches: planDigestMatches, name: "commit-plan digest"},
		{matches: admissionRequestJSONMatches, name: "admission-request JSON"},
		{matches: admissionRequestDigestMatches, name: "admission-request digest"},
	}
	candidateChecks := []canonicalCheckV1{
		{matches: profilePayloadJSONMatches, name: "profile-payload JSON"},
		{matches: profilePayloadDigestMatches, name: "profile-payload digest"},
		{matches: candidateProvenanceJSONMatches, name: "candidate-provenance JSON"},
		{matches: candidateProvenanceDigestMatches, name: "candidate-provenance digest"},
	}
	workMethodChecks := []canonicalCheckV1{
		{matches: workRecordJSONMatches, name: "Work-record JSON"},
		{matches: workRecordDigestMatches, name: "Work-record digest"},
		{matches: methodDescriptionJSONMatches, name: "MethodDescription JSON"},
		{matches: methodDescriptionDigestMatches, name: "MethodDescription digest"},
		{matches: methodContractJSONMatches, name: "MethodContract JSON"},
		{matches: methodContractDigestMatches, name: "MethodContract digest"},
	}
	authorChecks := []canonicalCheckV1{
		{matches: roleAssignmentJSONMatches, name: "ProfileAuthorRoleAssignment JSON"},
		{matches: roleAssignmentDigestMatches, name: "ProfileAuthorRoleAssignment digest"},
		{matches: systemAdmissionJSONMatches, name: "executor-system admission JSON"},
		{matches: systemAdmissionDigestMatches, name: "executor-system admission digest"},
		{matches: roleAdmissionJSONMatches, name: "ProfileAuthor role-admission JSON"},
		{matches: roleAdmissionDigestMatches, name: "ProfileAuthor role-admission digest"},
		{matches: justificationJSONMatches, name: "assignment-justification JSON"},
		{matches: justificationDigestMatches, name: "assignment-justification digest"},
		{matches: provenanceJSONMatches, name: "assignment-provenance JSON"},
		{matches: provenanceDigestMatches, name: "assignment-provenance digest"},
	}
	outcomeChecks := []canonicalCheckV1{
		{matches: observedProjectBasisJSONMatches, name: "ObservedProjectBasis JSON"},
		{matches: observedProjectBasisDigestMatches, name: "ObservedProjectBasis digest"},
		{matches: onboardingEffectJSONMatches, name: "ProfileOnboardingEffect JSON"},
		{matches: onboardingEffectDigestMatches, name: "ProfileOnboardingEffect digest"},
		{matches: outcomeAssessmentJSONMatches, name: "outcome-assessment JSON"},
		{matches: outcomeAssessmentDigestMatches, name: "outcome-assessment digest"},
	}
	checks := make([]canonicalCheckV1, 0, 29)
	checks = append(checks, identityChecks...)
	checks = append(checks, candidateChecks...)
	checks = append(checks, workMethodChecks...)
	checks = append(checks, authorChecks...)
	checks = append(checks, outcomeChecks...)
	return visitSliceV1(checks, func(_ int, check canonicalCheckV1) error {
		if !check.matches {
			return fmt.Errorf("prepared profile admission %s is not canonical", check.name)
		}
		return nil
	})
}

func clonePreparedBytesV1(value []byte) []byte {
	return append([]byte{}, value...)
}

func (prepared preparedProfileAdmissionV1) CommitPlan() ProfileDeclarationCommitPlan {
	return prepared.plan
}

func (prepared preparedProfileAdmissionV1) ProjectRoot() ProjectRootV1 {
	return prepared.projectRoot
}

func (prepared preparedProfileAdmissionV1) WorkRecord() ProfileOnboardingWorkRecord {
	return prepared.workRecord
}

func (prepared preparedProfileAdmissionV1) MethodDescription() ProfileOnboardingMethodDescriptionV1 {
	description, _ := prepared.methodDescription.(ProfileOnboardingMethodDescriptionV1)
	return description
}

func (prepared preparedProfileAdmissionV1) MethodContract() ProfileOnboardingMethodContractV1 {
	contract, _ := prepared.methodContract.(ProfileOnboardingMethodContractV1)
	return contract
}

func (prepared preparedProfileAdmissionV1) MethodDescriptionEdition() ProfileOnboardingMethodDescriptionEdition {
	return prepared.methodDescription
}

func (prepared preparedProfileAdmissionV1) MethodContractEdition() ProfileOnboardingMethodContractEdition {
	return prepared.methodContract
}

func (prepared preparedProfileAdmissionV1) MethodDescriptionV2() (
	ProfileOnboardingMethodDescriptionV2,
	bool,
) {
	description, ok := prepared.methodDescription.(ProfileOnboardingMethodDescriptionV2)
	return description, ok
}

func (prepared preparedProfileAdmissionV1) MethodContractV2() (
	ProfileOnboardingMethodContractV2,
	bool,
) {
	contract, ok := prepared.methodContract.(ProfileOnboardingMethodContractV2)
	return contract, ok
}

func (prepared preparedProfileAdmissionV1) ProfileAuthorRoleAssignment() ProfileAuthorRoleAssignmentV1 {
	return prepared.profileAuthorRoleAssignment
}

func (prepared preparedProfileAdmissionV1) ExecutorSystemAdmission() ProfileOnboardingExecutorSystemAdmissionV1 {
	return prepared.profileAuthorAssignmentSupport.SystemAdmission()
}

func (prepared preparedProfileAdmissionV1) ProfileAuthorRoleAdmission() ProfileAuthorRoleAdmissionV1 {
	return prepared.profileAuthorAssignmentSupport.RoleAdmission()
}

func (prepared preparedProfileAdmissionV1) AssignmentJustification() ProfileAuthorAssignmentJustificationV1 {
	return prepared.profileAuthorAssignmentSupport.Justification()
}

func (prepared preparedProfileAdmissionV1) AssignmentProvenance() ProfileAuthorAssignmentProvenanceV1 {
	return prepared.profileAuthorAssignmentSupport.Provenance()
}

func (prepared preparedProfileAdmissionV1) ObservedProjectBasis() ObservedProjectBasisV1 {
	return prepared.observedProjectBasis
}

func (prepared preparedProfileAdmissionV1) OnboardingEffect() ProfileOnboardingEffectV1 {
	return prepared.onboardingEffect
}

func (prepared preparedProfileAdmissionV1) OutcomeAssessment() ProfileOnboardingOutcomeAssessmentV1 {
	return prepared.outcomeAssessment
}

func (prepared preparedProfileAdmissionV1) ExpectedLedgerRevision() LedgerRevision {
	return prepared.plan.inputs.expectedLedgerRevision
}

func (prepared preparedProfileAdmissionV1) AdmissionRequestCanonicalJSON() []byte {
	return clonePreparedBytesV1(prepared.admissionRequestCanonicalJSON)
}

func (prepared preparedProfileAdmissionV1) AdmissionRequestDigest() ContentDigest {
	return prepared.admissionRequestDigest
}

func (prepared preparedProfileAdmissionV1) ProfilePayloadCanonicalJSON() []byte {
	return clonePreparedBytesV1(prepared.profilePayloadCanonicalJSON)
}

func (prepared preparedProfileAdmissionV1) ProfilePayloadDigest() ContentDigest {
	return prepared.profilePayloadDigest
}

func (prepared preparedProfileAdmissionV1) CandidateProvenanceCanonicalJSON() []byte {
	return clonePreparedBytesV1(prepared.candidateProvenanceJSON)
}

func (prepared preparedProfileAdmissionV1) CandidateProvenanceDigest() ContentDigest {
	return prepared.candidateProvenanceDigest
}

func (prepared preparedProfileAdmissionV1) WorkRecordCanonicalJSON() []byte {
	return clonePreparedBytesV1(prepared.workRecordCanonicalJSON)
}

func (prepared preparedProfileAdmissionV1) WorkRecordDigest() ContentDigest {
	return prepared.workRecordDigest
}

func (prepared preparedProfileAdmissionV1) MethodDescriptionCanonicalJSON() []byte {
	return clonePreparedBytesV1(prepared.methodDescriptionCanonicalJSON)
}

func (prepared preparedProfileAdmissionV1) MethodDescriptionDigest() ContentDigest {
	return prepared.methodDescriptionDigest
}

func (prepared preparedProfileAdmissionV1) MethodContractCanonicalJSON() []byte {
	return clonePreparedBytesV1(prepared.methodContractCanonicalJSON)
}

func (prepared preparedProfileAdmissionV1) MethodContractDigest() ContentDigest {
	return prepared.methodContractDigest
}

func (prepared preparedProfileAdmissionV1) ProfileAuthorRoleAssignmentCanonicalJSON() []byte {
	return clonePreparedBytesV1(prepared.profileAuthorRoleAssignmentCanonicalJSON)
}

func (prepared preparedProfileAdmissionV1) ProfileAuthorRoleAssignmentDigest() ContentDigest {
	return prepared.profileAuthorRoleAssignmentDigest
}

func (prepared preparedProfileAdmissionV1) ExecutorSystemAdmissionCanonicalJSON() []byte {
	return prepared.profileAuthorAssignmentSupport.SystemAdmissionCanonicalJSON()
}

func (prepared preparedProfileAdmissionV1) ExecutorSystemAdmissionDigest() ContentDigest {
	return prepared.profileAuthorAssignmentSupport.SystemAdmissionDigest()
}

func (prepared preparedProfileAdmissionV1) ProfileAuthorRoleAdmissionCanonicalJSON() []byte {
	return prepared.profileAuthorAssignmentSupport.RoleAdmissionCanonicalJSON()
}

func (prepared preparedProfileAdmissionV1) ProfileAuthorRoleAdmissionDigest() ContentDigest {
	return prepared.profileAuthorAssignmentSupport.RoleAdmissionDigest()
}

func (prepared preparedProfileAdmissionV1) AssignmentJustificationCanonicalJSON() []byte {
	return prepared.profileAuthorAssignmentSupport.JustificationCanonicalJSON()
}

func (prepared preparedProfileAdmissionV1) AssignmentJustificationDigest() ContentDigest {
	return prepared.profileAuthorAssignmentSupport.JustificationDigest()
}

func (prepared preparedProfileAdmissionV1) AssignmentProvenanceCanonicalJSON() []byte {
	return prepared.profileAuthorAssignmentSupport.ProvenanceCanonicalJSON()
}

func (prepared preparedProfileAdmissionV1) AssignmentProvenanceDigest() ContentDigest {
	return prepared.profileAuthorAssignmentSupport.ProvenanceDigest()
}

func (prepared preparedProfileAdmissionV1) ObservedProjectBasisCanonicalJSON() []byte {
	return clonePreparedBytesV1(prepared.observedProjectBasisCanonicalJSON)
}

func (prepared preparedProfileAdmissionV1) ObservedProjectBasisDigest() ContentDigest {
	return prepared.observedProjectBasisDigest
}

func (prepared preparedProfileAdmissionV1) OnboardingEffectCanonicalJSON() []byte {
	return clonePreparedBytesV1(prepared.onboardingEffectCanonicalJSON)
}

func (prepared preparedProfileAdmissionV1) OnboardingEffectDigest() ContentDigest {
	return prepared.onboardingEffectDigest
}

func (prepared preparedProfileAdmissionV1) OutcomeAssessmentCanonicalJSON() []byte {
	return clonePreparedBytesV1(prepared.outcomeAssessmentCanonicalJSON)
}

func (prepared preparedProfileAdmissionV1) OutcomeAssessmentDigest() ContentDigest {
	return prepared.outcomeAssessmentDigest
}

func (material tentativeProfileAdmissionTransactionMaterialV1) Prepared() PreparedProfileAdmissionV1 {
	return material.prepared
}

func (material tentativeProfileAdmissionTransactionMaterialV1) TentativeReceiptCanonicalJSON() []byte {
	return clonePreparedBytesV1(material.receiptCanonicalJSON)
}

func (material tentativeProfileAdmissionTransactionMaterialV1) TentativeReceiptDigest() ContentDigest {
	return material.receiptDigest
}

func (material tentativeProfileAdmissionTransactionMaterialV1) TentativeAdmissionRecordCanonicalJSON() []byte {
	return clonePreparedBytesV1(material.admissionRecordCanonicalJSON)
}

func (material tentativeProfileAdmissionTransactionMaterialV1) TentativeAdmissionRecordDigest() ContentDigest {
	return material.admissionRecordDigest
}
