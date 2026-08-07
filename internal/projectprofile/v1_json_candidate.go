package projectprofile

import (
	"bytes"
	"fmt"
)

const (
	profileDeclarationCandidateJSONSchemaV1       = "haft.project-profile.declaration-candidate/v1"
	profileDeclarationAdmissionInputsJSONSchemaV1 = "haft.project-profile.admission-inputs/v1"
	profileDeclarationCommitPlanJSONSchemaV1      = "haft.project-profile.commit-plan/v1"
)

type candidateProvenanceJSONV1 struct {
	AuthorityBasisRef                 string `json:"authority_basis_ref"`
	WorkRecordRef                     string `json:"work_record_ref"`
	WorkRecordDigest                  string `json:"work_record_digest"`
	ProfileAuthorRoleAssignmentRef    string `json:"profile_author_role_assignment_ref"`
	ProfileAuthorRoleAssignmentDigest string `json:"profile_author_role_assignment_digest"`
	ObservedProjectBasisRef           string `json:"observed_project_basis_ref"`
	ObservedProjectBasisDigest        string `json:"observed_project_basis_digest"`
	OutcomeAssessmentRef              string `json:"outcome_assessment_ref"`
	OutcomeAssessmentDigest           string `json:"outcome_assessment_digest"`
	ProjectRoot                       string `json:"project_root"`
	ClassifierVersion                 string `json:"classifier_version"`
	PolicyVersion                     string `json:"policy_version"`
	SessionRef                        string `json:"session_ref"`
	PayloadDigest                     string `json:"payload_digest"`
	ProvenanceDigest                  string `json:"provenance_digest"`
}

type profileDeclarationCandidateJSONV1 struct {
	Schema     string                          `json:"schema"`
	Payload    profileDeclarationPayloadJSONV1 `json:"payload"`
	Provenance candidateProvenanceJSONV1       `json:"provenance"`
}

type profileDeclarationAdmissionInputsJSONV1 struct {
	Schema                 string                            `json:"schema"`
	Candidate              profileDeclarationCandidateJSONV1 `json:"candidate"`
	ExpectedLedgerRevision uint64                            `json:"expected_ledger_revision"`
	InputsDigest           string                            `json:"inputs_digest"`
}

type profileDeclarationCommitPlanJSONV1 struct {
	Schema                          string                                  `json:"schema"`
	Inputs                          profileDeclarationAdmissionInputsJSONV1 `json:"inputs"`
	AuthorityResolutionRecordRef    string                                  `json:"authority_resolution_record_ref"`
	AuthorityResolutionRecordDigest string                                  `json:"authority_resolution_record_digest"`
	SingleUseKey                    string                                  `json:"single_use_key"`
	PlanDigest                      string                                  `json:"plan_digest"`
}

func EncodeProfileDeclarationCandidateV1CanonicalJSON(
	candidate ProfileDeclarationCandidateV1,
) ([]byte, error) {
	dto, err := profileDeclarationCandidateToJSONV1(candidate)
	if err != nil {
		return nil, err
	}
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileDeclarationCandidateV1CanonicalJSON(
	data []byte,
) (ProfileDeclarationCandidateV1, error) {
	var dto profileDeclarationCandidateJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return ProfileDeclarationCandidateV1{}, err
	}
	candidate, err := profileDeclarationCandidateFromJSONV1(dto)
	if err != nil {
		return ProfileDeclarationCandidateV1{}, err
	}
	canonical, err := EncodeProfileDeclarationCandidateV1CanonicalJSON(candidate)
	if err != nil {
		return ProfileDeclarationCandidateV1{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ProfileDeclarationCandidateV1{}, fmt.Errorf("profile declaration candidate JSON is not canonical")
	}
	return candidate, nil
}

func EncodeProfileDeclarationAdmissionInputsCanonicalJSON(
	inputs ProfileDeclarationAdmissionInputs,
) ([]byte, error) {
	dto, err := profileDeclarationAdmissionInputsToJSONV1(inputs)
	if err != nil {
		return nil, err
	}
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileDeclarationAdmissionInputsCanonicalJSON(
	data []byte,
) (ProfileDeclarationAdmissionInputs, error) {
	var dto profileDeclarationAdmissionInputsJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return ProfileDeclarationAdmissionInputs{}, err
	}
	inputs, err := profileDeclarationAdmissionInputsFromJSONV1(dto)
	if err != nil {
		return ProfileDeclarationAdmissionInputs{}, err
	}
	canonical, err := EncodeProfileDeclarationAdmissionInputsCanonicalJSON(inputs)
	if err != nil {
		return ProfileDeclarationAdmissionInputs{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ProfileDeclarationAdmissionInputs{}, fmt.Errorf("profile declaration admission-inputs JSON is not canonical")
	}
	return inputs, nil
}

func EncodeProfileDeclarationCommitPlanCanonicalJSON(
	plan ProfileDeclarationCommitPlan,
) ([]byte, error) {
	dto, err := profileDeclarationCommitPlanToJSONV1(plan)
	if err != nil {
		return nil, err
	}
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileDeclarationCommitPlanCanonicalJSON(
	data []byte,
) (ProfileDeclarationCommitPlan, error) {
	var dto profileDeclarationCommitPlanJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return ProfileDeclarationCommitPlan{}, err
	}
	plan, err := profileDeclarationCommitPlanFromJSONV1(dto)
	if err != nil {
		return ProfileDeclarationCommitPlan{}, err
	}
	canonical, err := EncodeProfileDeclarationCommitPlanCanonicalJSON(plan)
	if err != nil {
		return ProfileDeclarationCommitPlan{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ProfileDeclarationCommitPlan{}, fmt.Errorf("profile declaration commit-plan JSON is not canonical")
	}
	return plan, nil
}

func profileDeclarationCandidateToJSONV1(
	candidate ProfileDeclarationCandidateV1,
) (profileDeclarationCandidateJSONV1, error) {
	err := validateProfileDeclarationCandidateV1(candidate)
	if err != nil {
		return profileDeclarationCandidateJSONV1{}, err
	}
	payload, err := profileDeclarationPayloadToJSONV1(candidate.payload)
	if err != nil {
		return profileDeclarationCandidateJSONV1{}, err
	}
	provenance := candidateProvenanceToJSONV1(candidate.provenance)
	return profileDeclarationCandidateJSONV1{
		Schema:     profileDeclarationCandidateJSONSchemaV1,
		Payload:    payload,
		Provenance: provenance,
	}, nil
}

func profileDeclarationCandidateFromJSONV1(
	dto profileDeclarationCandidateJSONV1,
) (ProfileDeclarationCandidateV1, error) {
	if dto.Schema != profileDeclarationCandidateJSONSchemaV1 {
		return ProfileDeclarationCandidateV1{}, fmt.Errorf("unsupported candidate JSON schema %q", dto.Schema)
	}
	payload, err := profileDeclarationPayloadFromJSONV1(dto.Payload)
	if err != nil {
		return ProfileDeclarationCandidateV1{}, err
	}
	provenance, err := candidateProvenanceFromJSONV1(dto.Provenance)
	if err != nil {
		return ProfileDeclarationCandidateV1{}, err
	}
	return NewProfileDeclarationCandidateV1(payload, provenance)
}

func candidateProvenanceToJSONV1(
	value CandidateProvenanceV1,
) candidateProvenanceJSONV1 {
	authorityBasisRef := value.authorityBasisRef.String()
	workRecordRef := value.workRecordRef.String()
	workRecordDigest := value.workRecordDigest.String()
	assignmentRef := value.profileAuthorRoleAssignmentRef.String()
	assignmentDigest := value.profileAuthorRoleAssignmentDigest.String()
	basisRef := value.observedProjectBasisRef.String()
	basisDigest := value.observedBasisDigest.String()
	assessmentRef := value.outcomeAssessmentRef.String()
	assessmentDigest := value.outcomeAssessmentDigest.String()
	projectRoot := value.projectRoot.String()
	classifierVersion := value.classifierVersion.String()
	policyVersion := value.policyVersion.String()
	sessionRef := value.sessionRef.String()
	payloadDigest := value.payloadDigest.String()
	provenanceDigest := value.candidateProvenanceHash.String()
	return candidateProvenanceJSONV1{
		AuthorityBasisRef:                 authorityBasisRef,
		WorkRecordRef:                     workRecordRef,
		WorkRecordDigest:                  workRecordDigest,
		ProfileAuthorRoleAssignmentRef:    assignmentRef,
		ProfileAuthorRoleAssignmentDigest: assignmentDigest,
		ObservedProjectBasisRef:           basisRef,
		ObservedProjectBasisDigest:        basisDigest,
		OutcomeAssessmentRef:              assessmentRef,
		OutcomeAssessmentDigest:           assessmentDigest,
		ProjectRoot:                       projectRoot,
		ClassifierVersion:                 classifierVersion,
		PolicyVersion:                     policyVersion,
		SessionRef:                        sessionRef,
		PayloadDigest:                     payloadDigest,
		ProvenanceDigest:                  provenanceDigest,
	}
}

func candidateProvenanceFromJSONV1(
	dto candidateProvenanceJSONV1,
) (CandidateProvenanceV1, error) {
	authorityBasisRef, err := NewProfileDeclarationAuthorityBasisRef(dto.AuthorityBasisRef)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	workRecordRef, err := NewProfileOnboardingWorkRecordRef(dto.WorkRecordRef)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	workRecordDigest, err := NewContentDigest(dto.WorkRecordDigest)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	assignmentRef, err := NewRoleAssignmentRef(dto.ProfileAuthorRoleAssignmentRef)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	assignmentDigest, err := NewContentDigest(dto.ProfileAuthorRoleAssignmentDigest)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	observedProjectBasisRef, err := NewObservedProjectBasisRefV1(dto.ObservedProjectBasisRef)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	observedProjectBasisDigest, err := NewContentDigest(dto.ObservedProjectBasisDigest)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	outcomeAssessmentRef, err := NewProfileOnboardingOutcomeAssessmentRefV1(dto.OutcomeAssessmentRef)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	outcomeAssessmentDigest, err := NewContentDigest(dto.OutcomeAssessmentDigest)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	projectRoot, err := NewProjectRootV1(dto.ProjectRoot)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	classifierVersion, err := NewClassifierVersion(dto.ClassifierVersion)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	policyVersion, err := NewPolicyVersion(dto.PolicyVersion)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	sessionRef, err := NewSessionRef(dto.SessionRef)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	payloadDigest, err := NewContentDigest(dto.PayloadDigest)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	providedDigest, err := NewContentDigest(dto.ProvenanceDigest)
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	builder := NewCandidateProvenanceV1Builder(authorityBasisRef, workRecordRef, workRecordDigest)
	builder = builder.ForProfileAuthorRoleAssignment(assignmentRef, assignmentDigest)
	builder = builder.ForObservedProjectBasis(observedProjectBasisRef, observedProjectBasisDigest)
	builder = builder.ForOutcomeAssessment(outcomeAssessmentRef, outcomeAssessmentDigest)
	builder = builder.ForProject(projectRoot)
	builder = builder.ClassifiedBy(classifierVersion, policyVersion)
	builder = builder.InSession(sessionRef)
	builder = builder.ForPayload(payloadDigest)
	value, err := builder.Build()
	if err != nil {
		return CandidateProvenanceV1{}, err
	}
	if value.candidateProvenanceHash != providedDigest {
		return CandidateProvenanceV1{}, fmt.Errorf("candidate provenance JSON digest does not match fields")
	}
	return value, nil
}

func profileDeclarationAdmissionInputsToJSONV1(
	inputs ProfileDeclarationAdmissionInputs,
) (profileDeclarationAdmissionInputsJSONV1, error) {
	err := validateProfileDeclarationAdmissionInputs(inputs)
	if err != nil {
		return profileDeclarationAdmissionInputsJSONV1{}, err
	}
	candidate, err := profileDeclarationCandidateToJSONV1(inputs.candidate)
	if err != nil {
		return profileDeclarationAdmissionInputsJSONV1{}, err
	}
	return profileDeclarationAdmissionInputsJSONV1{
		Schema:                 profileDeclarationAdmissionInputsJSONSchemaV1,
		Candidate:              candidate,
		ExpectedLedgerRevision: inputs.expectedLedgerRevision.Value(),
		InputsDigest:           inputs.digest.String(),
	}, nil
}

func profileDeclarationAdmissionInputsFromJSONV1(
	dto profileDeclarationAdmissionInputsJSONV1,
) (ProfileDeclarationAdmissionInputs, error) {
	if dto.Schema != profileDeclarationAdmissionInputsJSONSchemaV1 {
		return ProfileDeclarationAdmissionInputs{}, fmt.Errorf("unsupported admission-inputs JSON schema %q", dto.Schema)
	}
	candidate, err := profileDeclarationCandidateFromJSONV1(dto.Candidate)
	if err != nil {
		return ProfileDeclarationAdmissionInputs{}, err
	}
	providedDigest, err := NewContentDigest(dto.InputsDigest)
	if err != nil {
		return ProfileDeclarationAdmissionInputs{}, err
	}
	expectedRevision := NewLedgerRevision(dto.ExpectedLedgerRevision)
	inputs, err := NewProfileDeclarationAdmissionInputs(candidate, expectedRevision)
	if err != nil {
		return ProfileDeclarationAdmissionInputs{}, err
	}
	if inputs.digest != providedDigest {
		return ProfileDeclarationAdmissionInputs{}, fmt.Errorf("admission-inputs JSON digest does not match fields")
	}
	return inputs, nil
}

func profileDeclarationCommitPlanToJSONV1(
	plan ProfileDeclarationCommitPlan,
) (profileDeclarationCommitPlanJSONV1, error) {
	err := validateProfileDeclarationCommitPlan(plan)
	if err != nil {
		return profileDeclarationCommitPlanJSONV1{}, err
	}
	inputs, err := profileDeclarationAdmissionInputsToJSONV1(plan.inputs)
	if err != nil {
		return profileDeclarationCommitPlanJSONV1{}, err
	}
	return profileDeclarationCommitPlanJSONV1{
		Schema:                          profileDeclarationCommitPlanJSONSchemaV1,
		Inputs:                          inputs,
		AuthorityResolutionRecordRef:    plan.authorityResolutionRecordRef.String(),
		AuthorityResolutionRecordDigest: plan.authorityResolutionRecordDigest.String(),
		SingleUseKey:                    plan.singleUseKey.String(),
		PlanDigest:                      plan.digest.String(),
	}, nil
}

func profileDeclarationCommitPlanFromJSONV1(
	dto profileDeclarationCommitPlanJSONV1,
) (ProfileDeclarationCommitPlan, error) {
	if dto.Schema != profileDeclarationCommitPlanJSONSchemaV1 {
		return ProfileDeclarationCommitPlan{}, fmt.Errorf("unsupported commit-plan JSON schema %q", dto.Schema)
	}
	inputs, err := profileDeclarationAdmissionInputsFromJSONV1(dto.Inputs)
	if err != nil {
		return ProfileDeclarationCommitPlan{}, err
	}
	resolutionRef, err := NewAuthorityResolutionRecordRef(dto.AuthorityResolutionRecordRef)
	if err != nil {
		return ProfileDeclarationCommitPlan{}, err
	}
	resolutionDigest, err := NewContentDigest(dto.AuthorityResolutionRecordDigest)
	if err != nil {
		return ProfileDeclarationCommitPlan{}, err
	}
	singleUseKey, err := NewSingleUseKey(dto.SingleUseKey)
	if err != nil {
		return ProfileDeclarationCommitPlan{}, err
	}
	providedDigest, err := NewContentDigest(dto.PlanDigest)
	if err != nil {
		return ProfileDeclarationCommitPlan{}, err
	}
	plan, err := NewProfileDeclarationCommitPlan(
		inputs,
		resolutionRef,
		resolutionDigest,
		singleUseKey,
	)
	if err != nil {
		return ProfileDeclarationCommitPlan{}, err
	}
	if plan.digest != providedDigest {
		return ProfileDeclarationCommitPlan{}, fmt.Errorf("commit-plan JSON digest does not match fields")
	}
	return plan, nil
}
