package sqlite

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func reconstructMethodDescription(
	row methodDescriptionRow,
) (projectprofile.ProfileOnboardingMethodDescriptionEdition, error) {
	canonicalJSON := []byte(row.canonicalJSON)
	value, err := decodeProfileOnboardingMethodDescriptionEdition(row.edition, canonicalJSON)
	if err != nil {
		return nil, fmt.Errorf("strictly decode durable MethodDescription: %w", err)
	}
	recordedAt, err := parseCanonicalTime("MethodDescription recorded_at", row.recordedAt)
	if err != nil {
		return nil, err
	}
	recordedAtText := recordedAt.Format(canonicalTimeLayout)
	projected, err := prepareMethodDescriptionRow(value, recordedAtText)
	if err != nil {
		return nil, err
	}
	if row != projected {
		return nil, fmt.Errorf("durable MethodDescription columns do not match its canonical record")
	}
	return value, nil
}

func reconstructMethodContract(
	row methodContractRow,
) (projectprofile.ProfileOnboardingMethodContractEdition, error) {
	canonicalJSON := []byte(row.canonicalJSON)
	value, err := decodeProfileOnboardingMethodContractEdition(row.edition, canonicalJSON)
	if err != nil {
		return nil, fmt.Errorf("strictly decode durable MethodContract: %w", err)
	}
	recordedAt, err := parseCanonicalTime("MethodContract recorded_at", row.recordedAt)
	if err != nil {
		return nil, err
	}
	recordedAtText := recordedAt.Format(canonicalTimeLayout)
	projected, err := prepareMethodContractRow(value, recordedAtText)
	if err != nil {
		return nil, err
	}
	if row != projected {
		return nil, fmt.Errorf("durable MethodContract columns do not match its canonical record")
	}
	return value, nil
}

func decodeProfileOnboardingMethodDescriptionEdition(
	edition string,
	canonicalJSON []byte,
) (projectprofile.ProfileOnboardingMethodDescriptionEdition, error) {
	switch edition {
	case "v1":
		return projectprofile.DecodeProfileOnboardingMethodDescriptionV1CanonicalJSON(canonicalJSON)
	case "v2":
		return projectprofile.DecodeProfileOnboardingMethodDescriptionV2CanonicalJSON(canonicalJSON)
	default:
		return nil, fmt.Errorf("unsupported durable MethodDescription edition %q", edition)
	}
}

func decodeProfileOnboardingMethodContractEdition(
	edition string,
	canonicalJSON []byte,
) (projectprofile.ProfileOnboardingMethodContractEdition, error) {
	switch edition {
	case "v1":
		return projectprofile.DecodeProfileOnboardingMethodContractV1CanonicalJSON(canonicalJSON)
	case "v2":
		return projectprofile.DecodeProfileOnboardingMethodContractV2CanonicalJSON(canonicalJSON)
	default:
		return nil, fmt.Errorf("unsupported durable MethodContract edition %q", edition)
	}
}

func reconstructSystemAdmission(
	row systemAdmissionRow,
) (projectprofile.ProfileOnboardingExecutorSystemAdmissionV1, error) {
	canonicalJSON := []byte(row.canonicalJSON)
	value, err := projectprofile.DecodeProfileOnboardingExecutorSystemAdmissionCanonicalJSON(canonicalJSON)
	if err != nil {
		return projectprofile.ProfileOnboardingExecutorSystemAdmissionV1{}, fmt.Errorf("strictly decode durable executor-system admission: %w", err)
	}
	recordedAt, err := parseCanonicalTime("executor-system admission recorded_at", row.recordedAt)
	if err != nil {
		return projectprofile.ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	recordedAtText := recordedAt.Format(canonicalTimeLayout)
	projected, err := prepareSystemAdmissionRow(value, recordedAtText)
	if err != nil {
		return projectprofile.ProfileOnboardingExecutorSystemAdmissionV1{}, err
	}
	if row != projected {
		return projectprofile.ProfileOnboardingExecutorSystemAdmissionV1{}, fmt.Errorf("durable executor-system admission columns do not match its canonical record")
	}
	return value, nil
}

func reconstructRoleAdmission(
	row roleAdmissionRow,
) (projectprofile.ProfileAuthorRoleAdmissionV1, error) {
	canonicalJSON := []byte(row.canonicalJSON)
	value, err := projectprofile.DecodeProfileAuthorRoleAdmissionCanonicalJSON(canonicalJSON)
	if err != nil {
		return projectprofile.ProfileAuthorRoleAdmissionV1{}, fmt.Errorf("strictly decode durable ProfileAuthor role admission: %w", err)
	}
	recordedAt, err := parseCanonicalTime("ProfileAuthor role admission recorded_at", row.recordedAt)
	if err != nil {
		return projectprofile.ProfileAuthorRoleAdmissionV1{}, err
	}
	recordedAtText := recordedAt.Format(canonicalTimeLayout)
	projected, err := prepareRoleAdmissionRow(value, recordedAtText)
	if err != nil {
		return projectprofile.ProfileAuthorRoleAdmissionV1{}, err
	}
	if row != projected {
		return projectprofile.ProfileAuthorRoleAdmissionV1{}, fmt.Errorf("durable ProfileAuthor role-admission columns do not match its canonical record")
	}
	return value, nil
}

func reconstructAssignmentSupport(
	row assignmentSupportRow,
) (
	projectprofile.ProfileAuthorAssignmentJustificationV1,
	projectprofile.ProfileAuthorAssignmentProvenanceV1,
	error,
) {
	justificationJSON := []byte(row.justificationJSON)
	justification, err := projectprofile.DecodeProfileAuthorAssignmentJustificationCanonicalJSON(justificationJSON)
	if err != nil {
		return projectprofile.ProfileAuthorAssignmentJustificationV1{}, projectprofile.ProfileAuthorAssignmentProvenanceV1{}, fmt.Errorf("strictly decode durable assignment justification: %w", err)
	}
	provenanceJSON := []byte(row.provenanceJSON)
	provenance, err := projectprofile.DecodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(provenanceJSON)
	if err != nil {
		return projectprofile.ProfileAuthorAssignmentJustificationV1{}, projectprofile.ProfileAuthorAssignmentProvenanceV1{}, fmt.Errorf("strictly decode durable assignment provenance: %w", err)
	}
	recordedAt, err := parseCanonicalTime("assignment support recorded_at", row.recordedAt)
	if err != nil {
		return projectprofile.ProfileAuthorAssignmentJustificationV1{}, projectprofile.ProfileAuthorAssignmentProvenanceV1{}, err
	}
	recordedAtText := recordedAt.Format(canonicalTimeLayout)
	projected, err := prepareAssignmentSupportRow(justification, provenance, recordedAtText)
	if err != nil {
		return projectprofile.ProfileAuthorAssignmentJustificationV1{}, projectprofile.ProfileAuthorAssignmentProvenanceV1{}, err
	}
	if row != projected {
		return projectprofile.ProfileAuthorAssignmentJustificationV1{}, projectprofile.ProfileAuthorAssignmentProvenanceV1{}, fmt.Errorf("durable assignment-support columns do not match canonical records")
	}
	return justification, provenance, nil
}

func reconstructRoleAssignment(
	row roleAssignmentRow,
) (projectprofile.ProfileAuthorRoleAssignmentV1, error) {
	canonicalJSON := []byte(row.canonicalJSON)
	value, err := projectprofile.DecodeProfileAuthorRoleAssignmentV1CanonicalJSON(canonicalJSON)
	if err != nil {
		return projectprofile.ProfileAuthorRoleAssignmentV1{}, fmt.Errorf("strictly decode durable ProfileAuthorRoleAssignment: %w", err)
	}
	recordedAt, err := parseCanonicalTime("RoleAssignment recorded_at", row.recordedAt)
	if err != nil {
		return projectprofile.ProfileAuthorRoleAssignmentV1{}, err
	}
	recordedAtText := recordedAt.Format(canonicalTimeLayout)
	projected, err := prepareRoleAssignmentRow(value, recordedAtText)
	if err != nil {
		return projectprofile.ProfileAuthorRoleAssignmentV1{}, err
	}
	if row != projected {
		return projectprofile.ProfileAuthorRoleAssignmentV1{}, fmt.Errorf("durable ProfileAuthorRoleAssignment columns do not match its canonical record")
	}
	return value, nil
}

func reconstructObservedBasis(
	row observedBasisRow,
) (projectprofile.ObservedProjectBasisV1, error) {
	canonicalJSON := []byte(row.canonicalJSON)
	value, err := projectprofile.DecodeObservedProjectBasisV1CanonicalJSON(canonicalJSON)
	if err != nil {
		return nil, fmt.Errorf("strictly decode durable ObservedProjectBasis: %w", err)
	}
	recordedAt, err := parseCanonicalTime("ObservedProjectBasis recorded_at", row.recordedAt)
	if err != nil {
		return nil, err
	}
	recordedAtText := recordedAt.Format(canonicalTimeLayout)
	projected, err := prepareObservedBasisRow(value, recordedAtText)
	if err != nil {
		return nil, err
	}
	if row != projected {
		return nil, fmt.Errorf("durable ObservedProjectBasis columns do not match its canonical record")
	}
	return value, nil
}

func reconstructWork(
	row workRow,
) (projectprofile.ProfileOnboardingWorkRecord, error) {
	canonicalJSON := []byte(row.canonicalJSON)
	value, err := projectprofile.DecodeProfileOnboardingWorkRecordCanonicalJSON(canonicalJSON)
	if err != nil {
		return projectprofile.ProfileOnboardingWorkRecord{}, fmt.Errorf("strictly decode durable profile-onboarding Work: %w", err)
	}
	recordedAt, err := parseCanonicalTime("Work recorded_at", row.recordedAt)
	if err != nil {
		return projectprofile.ProfileOnboardingWorkRecord{}, err
	}
	root, err := projectprofile.NewProjectRootV1(row.projectRoot)
	if err != nil {
		return projectprofile.ProfileOnboardingWorkRecord{}, fmt.Errorf("decode durable Work project root: %w", err)
	}
	recordedAtText := recordedAt.Format(canonicalTimeLayout)
	projected, err := prepareWorkRow(root, value, recordedAtText)
	if err != nil {
		return projectprofile.ProfileOnboardingWorkRecord{}, err
	}
	if row != projected {
		return projectprofile.ProfileOnboardingWorkRecord{}, fmt.Errorf("durable Work columns do not match its canonical record")
	}
	return value, nil
}

func reconstructEffect(
	row effectRow,
) (projectprofile.ProfileOnboardingEffectV1, error) {
	canonicalJSON := []byte(row.canonicalJSON)
	value, err := projectprofile.DecodeProfileOnboardingEffectV1CanonicalJSON(canonicalJSON)
	if err != nil {
		return nil, fmt.Errorf("strictly decode durable profile-onboarding effect: %w", err)
	}
	recordedAt, err := parseCanonicalTime("effect recorded_at", row.recordedAt)
	if err != nil {
		return nil, err
	}
	recordedAtText := recordedAt.Format(canonicalTimeLayout)
	projected, err := prepareEffectRow(value, recordedAtText)
	if err != nil {
		return nil, err
	}
	if row != projected {
		return nil, fmt.Errorf("durable effect columns do not match its canonical record")
	}
	return value, nil
}

func reconstructAssessment(
	row assessmentRow,
	effect projectprofile.ProfileOnboardingEffectV1,
) (projectprofile.ProfileOnboardingOutcomeAssessmentV1, error) {
	canonicalJSON := []byte(row.canonicalJSON)
	value, err := projectprofile.DecodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(
		canonicalJSON,
		effect,
	)
	if err != nil {
		return nil, fmt.Errorf("strictly decode durable profile-onboarding assessment: %w", err)
	}
	recordedAt, err := parseCanonicalTime("assessment recorded_at", row.recordedAt)
	if err != nil {
		return nil, err
	}
	recordedAtText := recordedAt.Format(canonicalTimeLayout)
	projected, err := prepareAssessmentRow(value, recordedAtText)
	if err != nil {
		return nil, err
	}
	if row != projected {
		return nil, fmt.Errorf("durable assessment columns do not match its canonical record")
	}
	return value, nil
}

func validateDurableRecordingTimes(
	rows profileOnboardingRowsV1,
	provenance projectprofile.ProfileAuthorAssignmentProvenanceV1,
	work projectprofile.ProfileOnboardingWorkRecord,
) error {
	workRecordedAt, err := parseCanonicalTime(
		"Work recorded_at",
		rows.work.recordedAt,
	)
	if err != nil {
		return err
	}
	workInterval := work.WorkInterval()
	basisWindow := work.BasisObservationWindow()
	err = validateRecordingTime(workRecordedAt, workInterval, basisWindow)
	if err != nil {
		return fmt.Errorf("validate durable Work recorded_at: %w", err)
	}
	supportRecordedAt, err := parseCanonicalTime(
		"assignment support recorded_at",
		rows.support.recordedAt,
	)
	if err != nil {
		return err
	}
	provenanceRecordedAt := provenance.RecordedAt()
	if supportRecordedAt.Before(provenanceRecordedAt) {
		return fmt.Errorf("durable assignment-support recorded_at must not precede assignment provenance")
	}
	return nil
}
