package projectprofile

import (
	"bytes"
	"fmt"
)

const profileAuthorRoleAssignmentJSONSchemaV1 = "haft.project-profile.profile-author-role-assignment/v1"

type profileAuthorRoleAssignmentJSONV1 struct {
	Schema                     string               `json:"schema"`
	RoleAssignmentRef          string               `json:"role_assignment_ref"`
	HolderSystemRef            string               `json:"holder_system_ref"`
	AdmittedRoleRef            string               `json:"admitted_role_ref"`
	BoundedContextRef          string               `json:"bounded_context_ref"`
	ValidityWindow             closedIntervalJSONV1 `json:"validity_window"`
	SystemAdmissionRef         string               `json:"system_admission_ref"`
	SystemAdmissionDigest      string               `json:"system_admission_digest"`
	RoleAdmissionRef           string               `json:"role_admission_ref"`
	RoleAdmissionDigest        string               `json:"role_admission_digest"`
	AssignmentJustificationRef string               `json:"assignment_justification_ref"`
	JustificationDigest        string               `json:"assignment_justification_digest"`
	AssignmentProvenanceRef    string               `json:"assignment_provenance_ref"`
	ProvenanceDigest           string               `json:"assignment_provenance_digest"`
}

func EncodeProfileAuthorRoleAssignmentV1CanonicalJSON(
	assignment ProfileAuthorRoleAssignmentV1,
) ([]byte, error) {
	canonical, err := canonicalProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return nil, err
	}
	dto := profileAuthorRoleAssignmentToJSONV1(canonical)
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileAuthorRoleAssignmentV1CanonicalJSON(
	data []byte,
) (ProfileAuthorRoleAssignmentV1, error) {
	var dto profileAuthorRoleAssignmentJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	assignment, err := profileAuthorRoleAssignmentFromJSONV1(dto)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	canonical, err := EncodeProfileAuthorRoleAssignmentV1CanonicalJSON(assignment)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ProfileAuthorRoleAssignmentV1{}, fmt.Errorf("ProfileAuthorRoleAssignmentV1 JSON is not canonical")
	}
	return assignment, nil
}

func profileAuthorRoleAssignmentToJSONV1(
	assignment ProfileAuthorRoleAssignmentV1,
) profileAuthorRoleAssignmentJSONV1 {
	return profileAuthorRoleAssignmentJSONV1{
		Schema:                     profileAuthorRoleAssignmentJSONSchemaV1,
		RoleAssignmentRef:          assignment.roleAssignmentRef.String(),
		HolderSystemRef:            assignment.holderSystemRef.String(),
		AdmittedRoleRef:            assignment.admittedRoleRef.String(),
		BoundedContextRef:          assignment.boundedContextRef.String(),
		ValidityWindow:             closedIntervalToJSONV1(assignment.validityWindow.closedIntervalV1),
		SystemAdmissionRef:         assignment.systemAdmissionRef.String(),
		SystemAdmissionDigest:      assignment.systemAdmissionDigest.String(),
		RoleAdmissionRef:           assignment.roleAdmissionRef.String(),
		RoleAdmissionDigest:        assignment.roleAdmissionDigest.String(),
		AssignmentJustificationRef: assignment.justificationRef.String(),
		JustificationDigest:        assignment.justificationDigest.String(),
		AssignmentProvenanceRef:    assignment.provenanceRef.String(),
		ProvenanceDigest:           assignment.provenanceDigest.String(),
	}
}

func profileAuthorRoleAssignmentFromJSONV1(
	dto profileAuthorRoleAssignmentJSONV1,
) (ProfileAuthorRoleAssignmentV1, error) {
	if dto.Schema != profileAuthorRoleAssignmentJSONSchemaV1 {
		return ProfileAuthorRoleAssignmentV1{}, fmt.Errorf("unsupported ProfileAuthorRoleAssignmentV1 JSON schema %q", dto.Schema)
	}
	assignmentRef, err := NewRoleAssignmentRef(dto.RoleAssignmentRef)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	holderRef, err := NewSystemRef(dto.HolderSystemRef)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	roleRef, err := NewRoleRef(dto.AdmittedRoleRef)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	contextRef, err := NewBoundedContextRef(dto.BoundedContextRef)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	windowValue, err := closedIntervalFromJSONV1("RoleAssignment validity window", dto.ValidityWindow)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	systemAdmissionRef, err := NewSystemAdmissionRef(dto.SystemAdmissionRef)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	systemAdmissionDigest, err := NewContentDigest(dto.SystemAdmissionDigest)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	roleAdmissionRef, err := NewRoleAdmissionRef(dto.RoleAdmissionRef)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	roleAdmissionDigest, err := NewContentDigest(dto.RoleAdmissionDigest)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	justificationRef, err := NewRoleAssignmentJustificationRef(dto.AssignmentJustificationRef)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	justificationDigest, err := NewContentDigest(dto.JustificationDigest)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	provenanceRef, err := NewRoleAssignmentProvenanceRef(dto.AssignmentProvenanceRef)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	provenanceDigest, err := NewContentDigest(dto.ProvenanceDigest)
	if err != nil {
		return ProfileAuthorRoleAssignmentV1{}, err
	}
	window := RoleAssignmentWindowV1{closedIntervalV1: windowValue}
	builder := NewProfileAuthorRoleAssignmentV1Builder(assignmentRef)
	builder = builder.HeldBy(holderRef)
	builder = builder.Assigning(roleRef)
	builder = builder.InContext(contextRef)
	builder = builder.ValidDuring(window)
	builder = builder.WithSystemAdmission(systemAdmissionRef, systemAdmissionDigest)
	builder = builder.WithRoleAdmission(roleAdmissionRef, roleAdmissionDigest)
	builder = builder.JustifiedBy(justificationRef, justificationDigest)
	builder = builder.WithProvenance(provenanceRef, provenanceDigest)
	return builder.Build()
}
