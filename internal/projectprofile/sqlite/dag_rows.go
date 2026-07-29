package sqlite

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const canonicalTimeLayout = time.RFC3339Nano

type closedIntervalProjection struct {
	From  string `json:"from"`
	Until string `json:"until"`
}

type stateTransitionProjectionV1 struct {
	PreStateRef       string `json:"pre_state_ref"`
	PostStateRef      string `json:"post_state_ref"`
	DeltaPredicateRef string `json:"delta_predicate_ref"`
}

type methodDescriptionProjectionV1 struct {
	Ref                  string `json:"ref"`
	DescribedMethodRef   string `json:"described_method_ref"`
	BoundedContextRef    string `json:"bounded_context_ref"`
	SourceRevision       string `json:"source_revision"`
	Edition              string `json:"edition"`
	RequiredRoleRef      string `json:"required_role_ref"`
	RequiredSystemKind   string `json:"required_system_kind"`
	StatePlaneRef        string `json:"state_plane_ref"`
	AffectedRefKind      string `json:"affected_ref_kind"`
	EffectWitnessRuleRef string `json:"effect_witness_rule_ref"`
}

type methodContractProjectionV1 struct {
	Ref                           string          `json:"ref"`
	Edition                       string          `json:"edition"`
	MethodDescriptionRef          string          `json:"method_description_ref"`
	MethodDescriptionDigest       string          `json:"method_description_digest"`
	BoundedContextRef             string          `json:"bounded_context_ref"`
	RoleAdmissionPolicyRef        string          `json:"role_admission_policy_ref"`
	SystemAdmissionPolicyRef      string          `json:"system_admission_policy_ref"`
	ParameterSpecSetDigest        string          `json:"parameter_spec_set_digest"`
	AcceptedResultKinds           json.RawMessage `json:"accepted_result_kinds"`
	RequiredOccurrenceSlots       json.RawMessage `json:"required_occurrence_slots"`
	OccurrenceCoverageRuleRefs    json.RawMessage `json:"occurrence_coverage_rule_refs"`
	EffectStateWitnessRuleRef     string          `json:"effect_state_witness_rule_ref"`
	AcceptanceStandardRef         string          `json:"acceptance_standard_ref"`
	AcceptanceStandardEdition     string          `json:"acceptance_standard_edition"`
	HolderEqualsExecutedWithinRef string          `json:"holder_equals_executed_within_rule_ref"`
}

type systemAdmissionProjectionV1 struct {
	Ref                          string                            `json:"ref"`
	SystemRef                    string                            `json:"system_ref"`
	AdmittedSystemKind           string                            `json:"admitted_system_kind"`
	BoundedContextRef            string                            `json:"bounded_context_ref"`
	GoverningPatternRef          string                            `json:"governing_pattern_ref"`
	IdentityBasis                executorIdentityBasisProjectionV1 `json:"identity_basis"`
	ActingEligibilityBasisRef    string                            `json:"acting_eligibility_basis_ref"`
	ActingEligibilityBasisDigest string                            `json:"acting_eligibility_basis_digest"`
	SessionRef                   string                            `json:"session_ref"`
	ValidityWindow               closedIntervalProjection          `json:"validity_window"`
	MethodDescriptionRef         string                            `json:"method_description_ref"`
	MethodDescriptionDigest      string                            `json:"method_description_digest"`
	MethodContractRef            string                            `json:"method_contract_ref"`
	MethodContractDigest         string                            `json:"method_contract_digest"`
	SystemAdmissionPolicyRef     string                            `json:"system_admission_policy_ref"`
}

type operatorDesignationProjectionV1 struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type executorIdentityBasisProjectionV1 struct {
	Kind               string                           `json:"kind"`
	SystemRef          string                           `json:"system_ref"`
	KernelOwned        *runtimeIdentityProjectionV1     `json:"kernel_owned"`
	OperatorDesignated *operatorDesignationProjectionV1 `json:"operator_designated"`
}

type roleAdmissionProjectionV1 struct {
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

type assignmentJustificationProjectionV1 struct {
	Ref                   string                   `json:"ref"`
	RuleRef               string                   `json:"rule_ref"`
	RuleStatement         string                   `json:"rule_statement"`
	BoundedContextRef     string                   `json:"bounded_context_ref"`
	SystemAdmissionRef    string                   `json:"system_admission_ref"`
	SystemAdmissionDigest string                   `json:"system_admission_digest"`
	RoleAdmissionRef      string                   `json:"role_admission_ref"`
	RoleAdmissionDigest   string                   `json:"role_admission_digest"`
	AssignmentWindow      closedIntervalProjection `json:"assignment_window"`
	MethodContractRef     string                   `json:"method_contract_ref"`
	MethodContractDigest  string                   `json:"method_contract_digest"`
}

type runtimeIdentityProjectionV1 struct {
	Identity string `json:"identity"`
	Version  string `json:"version"`
}

type assignmentProvenanceProjectionV1 struct {
	Ref                 string                      `json:"ref"`
	JustificationRef    string                      `json:"justification_ref"`
	JustificationDigest string                      `json:"justification_digest"`
	SessionRef          string                      `json:"session_ref"`
	Kernel              runtimeIdentityProjectionV1 `json:"kernel"`
	Runtime             runtimeIdentityProjectionV1 `json:"runtime"`
	RecordedAt          string                      `json:"recorded_at"`
}

type roleAssignmentProjectionV1 struct {
	RoleAssignmentRef          string                   `json:"role_assignment_ref"`
	HolderSystemRef            string                   `json:"holder_system_ref"`
	AdmittedRoleRef            string                   `json:"admitted_role_ref"`
	BoundedContextRef          string                   `json:"bounded_context_ref"`
	ValidityWindow             closedIntervalProjection `json:"validity_window"`
	SystemAdmissionRef         string                   `json:"system_admission_ref"`
	SystemAdmissionDigest      string                   `json:"system_admission_digest"`
	RoleAdmissionRef           string                   `json:"role_admission_ref"`
	RoleAdmissionDigest        string                   `json:"role_admission_digest"`
	AssignmentJustificationRef string                   `json:"assignment_justification_ref"`
	AssignmentJustificationDig string                   `json:"assignment_justification_digest"`
	AssignmentProvenanceRef    string                   `json:"assignment_provenance_ref"`
	AssignmentProvenanceDigest string                   `json:"assignment_provenance_digest"`
}

type observedBasisProjectionV1 struct {
	Ref               string                   `json:"ref"`
	ProjectRoot       string                   `json:"project_root"`
	ObservationWindow closedIntervalProjection `json:"observation_window"`
	DetectorVersion   string                   `json:"detector_version"`
	ClassifierVersion string                   `json:"classifier_version"`
}

type workOutcomeProjectionV1 struct {
	Kind                string `json:"kind"`
	PayloadDigest       string `json:"payload_digest"`
	ObservedBasisDigest string `json:"observed_basis_digest"`
	MissingBasisDigest  string `json:"missing_basis_digest"`
}

type workProjectionV1 struct {
	RecordRef                         string                      `json:"record_ref"`
	WorkRef                           string                      `json:"work_ref"`
	EnactsMethodRef                   string                      `json:"enacts_method_ref"`
	MethodDescriptionRef              string                      `json:"method_description_ref"`
	MethodDescriptionDigest           string                      `json:"method_description_digest"`
	MethodContractRef                 string                      `json:"method_contract_ref"`
	MethodContractDigest              string                      `json:"method_contract_digest"`
	ParameterBindings                 json.RawMessage             `json:"parameter_bindings"`
	PerformedBy                       string                      `json:"performed_by_role_assignment_ref"`
	ProfileAuthorRoleAssignmentRef    string                      `json:"profile_author_role_assignment_ref"`
	ProfileAuthorRoleAssignmentDigest string                      `json:"profile_author_role_assignment_digest"`
	ExecutedWithin                    string                      `json:"executed_within_system_ref"`
	BoundedContextRef                 string                      `json:"bounded_context_ref"`
	WorkInterval                      closedIntervalProjection    `json:"work_interval"`
	BasisObservationWindow            closedIntervalProjection    `json:"basis_observation_window"`
	ObservedProjectBasisRef           string                      `json:"observed_project_basis_ref"`
	ObservedProjectBasisDigest        string                      `json:"observed_project_basis_digest"`
	InputRefs                         json.RawMessage             `json:"input_refs"`
	OutputRefs                        json.RawMessage             `json:"output_refs"`
	ResourceRefs                      json.RawMessage             `json:"resource_refs"`
	AffectedRefKind                   string                      `json:"affected_ref_kind"`
	AffectedRefs                      json.RawMessage             `json:"affected_refs"`
	StatePlaneRef                     string                      `json:"state_plane_ref"`
	StateTransition                   stateTransitionProjectionV1 `json:"state_transition"`
	Outcome                           workOutcomeProjectionV1     `json:"outcome"`
}

type effectResultProjectionV1 struct {
	Kind                       string `json:"kind"`
	OutputRef                  string `json:"output_ref"`
	PayloadDigest              string `json:"payload_digest"`
	ObservedProjectBasisRef    string `json:"observed_project_basis_ref"`
	ObservedProjectBasisDigest string `json:"observed_project_basis_digest"`
	MissingBasisDigest         string `json:"missing_basis_digest"`
}

type effectProjectionV1 struct {
	Ref                        string                      `json:"ref"`
	WorkRecordRef              string                      `json:"work_record_ref"`
	WorkRef                    string                      `json:"work_ref"`
	WorkRecordDigest           string                      `json:"work_record_digest"`
	Result                     effectResultProjectionV1    `json:"result"`
	AffectedEntityRefs         json.RawMessage             `json:"affected_entity_of_concern_refs"`
	StatePlaneRef              string                      `json:"state_plane_ref"`
	StateWitness               stateTransitionProjectionV1 `json:"state_witness"`
	EvidenceProvenancePathRefs json.RawMessage             `json:"evidence_provenance_path_refs"`
}

type verdictProjectionV1 struct {
	Kind               string `json:"kind"`
	ReasonRef          string `json:"reason_ref"`
	MissingBasisDigest string `json:"missing_basis_digest"`
}

type assessmentProjectionV1 struct {
	Ref                        string              `json:"ref"`
	EffectRef                  string              `json:"effect_ref"`
	EffectDigest               string              `json:"effect_digest"`
	WorkRecordRef              string              `json:"work_record_ref"`
	WorkRef                    string              `json:"work_ref"`
	WorkRecordDigest           string              `json:"work_record_digest"`
	AcceptanceStandardRef      string              `json:"acceptance_standard_ref"`
	AcceptanceStandardEdition  string              `json:"acceptance_standard_edition"`
	ComparatorRef              string              `json:"comparator_ref"`
	ComparatorEdition          string              `json:"comparator_edition"`
	Verdict                    verdictProjectionV1 `json:"verdict"`
	EvidenceProvenancePathRefs json.RawMessage     `json:"evidence_provenance_path_refs"`
}

type methodDescriptionRow struct {
	methodDescriptionRef string
	describedMethodRef   string
	boundedContextRef    string
	sourceRevision       string
	edition              string
	requiredRoleRef      string
	requiredSystemKind   string
	statePlaneRef        string
	affectedRefKind      string
	effectWitnessRuleRef string
	canonicalJSON        string
	digest               string
	recordedAt           string
}

type methodContractRow struct {
	methodContractRef                 string
	edition                           string
	methodDescriptionRef              string
	methodDescriptionDigest           string
	boundedContextRef                 string
	roleAdmissionPolicyRef            string
	systemAdmissionPolicyRef          string
	parameterSpecSetDigest            string
	acceptedResultKindsJSON           string
	requiredOccurrenceSlotsJSON       string
	occurrenceCoverageRuleRefsJSON    string
	effectStateWitnessRuleRef         string
	acceptanceStandardRef             string
	acceptanceStandardEdition         string
	holderEqualsExecutedWithinRuleRef string
	canonicalJSON                     string
	digest                            string
	recordedAt                        string
}

type systemAdmissionRow struct {
	ref                            string
	systemRef                      string
	admittedSystemKind             string
	boundedContextRef              string
	governingPatternRef            string
	identityBasisKind              string
	identityBasisSystemRef         string
	identityBasisKernelIdentity    string
	identityBasisKernelVersion     string
	identityBasisDesignationRef    string
	identityBasisDesignationDigest string
	actingEligibilityBasisRef      string
	actingEligibilityBasisDigest   string
	sessionRef                     string
	validFrom                      string
	validUntil                     string
	methodDescriptionRef           string
	methodDescriptionDigest        string
	methodContractRef              string
	methodContractDigest           string
	admissionPolicyRef             string
	canonicalJSON                  string
	digest                         string
	recordedAt                     string
}

type roleAdmissionRow struct {
	ref                     string
	roleRef                 string
	boundedContextRef       string
	governingPatternRef     string
	methodDescriptionRef    string
	methodDescriptionDigest string
	methodContractRef       string
	methodContractDigest    string
	admissionPolicyRef      string
	canonicalJSON           string
	digest                  string
	recordedAt              string
}

type assignmentSupportRow struct {
	justificationRef              string
	ruleRef                       string
	ruleStatement                 string
	boundedContextRef             string
	systemAdmissionRef            string
	systemAdmissionDigest         string
	roleAdmissionRef              string
	roleAdmissionDigest           string
	assignmentFrom                string
	assignmentUntil               string
	methodContractRef             string
	methodContractDigest          string
	justificationJSON             string
	justificationDigest           string
	provenanceRef                 string
	provenanceJustificationRef    string
	provenanceJustificationDigest string
	sessionRef                    string
	kernelIdentity                string
	kernelVersion                 string
	runtimeIdentity               string
	runtimeVersion                string
	provenanceRecordedAt          string
	provenanceJSON                string
	provenanceDigest              string
	recordedAt                    string
}

type roleAssignmentRow struct {
	ref                   string
	holderSystemRef       string
	admittedRoleRef       string
	boundedContextRef     string
	validFrom             string
	validUntil            string
	systemAdmissionRef    string
	systemAdmissionDigest string
	roleAdmissionRef      string
	roleAdmissionDigest   string
	justificationRef      string
	justificationDigest   string
	provenanceRef         string
	provenanceDigest      string
	canonicalJSON         string
	digest                string
	recordedAt            string
}

type observedBasisRow struct {
	ref               string
	projectRoot       string
	observationFrom   string
	observationUntil  string
	detectorVersion   string
	classifierVersion string
	canonicalJSON     string
	digest            string
	recordedAt        string
}

type workRow struct {
	workRecordRef                     string
	workRef                           string
	projectRoot                       string
	enactsMethodRef                   string
	methodDescriptionRef              string
	methodDescriptionDigest           string
	methodContractRef                 string
	methodContractDigest              string
	parameterBindingsJSON             string
	performedByRoleAssignmentRef      string
	profileAuthorRoleAssignmentRef    string
	profileAuthorRoleAssignmentDigest string
	executedWithinRef                 string
	workFrom                          string
	workUntil                         string
	boundedContextRef                 string
	basisObservationFrom              string
	basisObservationUntil             string
	observedProjectBasisRef           string
	observedProjectBasisDigest        string
	inputsJSON                        string
	outputsJSON                       string
	resourcesJSON                     string
	affectedRefKind                   string
	affectedRefsJSON                  string
	statePlaneRef                     string
	preStateRef                       string
	postStateRef                      string
	deltaPredicateRef                 string
	outcomeKind                       string
	profilePayloadDigest              string
	observedBasisDigest               string
	missingBasisDigest                string
	canonicalJSON                     string
	digest                            string
	recordedAt                        string
}

type effectRow struct {
	ref                        string
	workRecordRef              string
	workRef                    string
	workRecordDigest           string
	resultKind                 string
	outputRef                  string
	profilePayloadDigest       string
	observedProjectBasisRef    string
	observedProjectBasisDigest string
	missingBasisDigest         string
	affectedEntityRefsJSON     string
	statePlaneRef              string
	preStateRef                string
	postStateRef               string
	deltaPredicateRef          string
	evidencePathRefsJSON       string
	canonicalJSON              string
	digest                     string
	recordedAt                 string
}

type assessmentRow struct {
	ref                       string
	effectRef                 string
	effectDigest              string
	workRecordRef             string
	workRef                   string
	workRecordDigest          string
	acceptanceStandardRef     string
	acceptanceStandardEdition string
	comparatorRef             string
	comparatorEdition         string
	verdictKind               string
	verdictReasonRef          string
	missingBasisDigest        string
	evidencePathRefsJSON      string
	canonicalJSON             string
	digest                    string
	recordedAt                string
}

type profileOnboardingRowsV1 struct {
	description methodDescriptionRow
	contract    methodContractRow
	system      systemAdmissionRow
	role        roleAdmissionRow
	support     assignmentSupportRow
	assignment  roleAssignmentRow
	basis       observedBasisRow
	work        workRow
	effect      effectRow
	assessment  assessmentRow
}

func prepareProfileOnboardingRowsV1(
	projectRoot projectprofile.ProjectRootV1,
	values ProfileOnboardingValueSetV1,
	recordedAt time.Time,
) (profileOnboardingRowsV1, error) {
	root, err := validateProjectRoot(projectRoot)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	err = validateProfileOnboardingValueSetV1(values)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	if values.observedBasis.ProjectRoot() != root {
		return profileOnboardingRowsV1{}, fmt.Errorf("ObservedProjectBasis project root does not match storage project root")
	}
	bindings := values.workRecord.ParameterBindings()
	err = validateMethodBindings(bindings, root)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	recordedAtText, err := canonicalTime("recorded_at", recordedAt)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	workInterval := values.workRecord.WorkInterval()
	basisWindow := values.workRecord.BasisObservationWindow()
	err = validateRecordingTime(recordedAt, workInterval, basisWindow)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	provenanceRecordedAt := values.provenance.RecordedAt()
	if recordedAt.Before(provenanceRecordedAt) {
		return profileOnboardingRowsV1{}, fmt.Errorf("recorded_at must not precede assignment provenance")
	}
	description, err := prepareMethodDescriptionRow(values.methodDescription, recordedAtText)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	contract, err := prepareMethodContractRow(values.methodContract, recordedAtText)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	system, err := prepareSystemAdmissionRow(values.systemAdmission, recordedAtText)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	role, err := prepareRoleAdmissionRow(values.roleAdmission, recordedAtText)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	support, err := prepareAssignmentSupportRow(values.justification, values.provenance, recordedAtText)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	assignment, err := prepareRoleAssignmentRow(values.roleAssignment, recordedAtText)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	basis, err := prepareObservedBasisRow(values.observedBasis, recordedAtText)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	work, err := prepareWorkRow(root, values.workRecord, recordedAtText)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	effect, err := prepareEffectRow(values.effect, recordedAtText)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	assessment, err := prepareAssessmentRow(values.assessment, recordedAtText)
	if err != nil {
		return profileOnboardingRowsV1{}, err
	}
	return profileOnboardingRowsV1{
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
	}, nil
}

func decodeProjectionV1(data []byte, target any, label string) error {
	err := json.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("project %s storage columns: %w", label, err)
	}
	return nil
}

func canonicalTime(name string, value time.Time) (string, error) {
	if value.IsZero() {
		return "", fmt.Errorf("%s is required", name)
	}
	utc := value.UTC()
	text := utc.Format(time.RFC3339Nano)
	_, err := parseCanonicalTime(name, text)
	if err != nil {
		return "", err
	}
	return text, nil
}

func parseCanonicalTime(name string, raw string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339Nano: %w", name, err)
	}
	utc := value.UTC()
	canonical := utc.Format(time.RFC3339Nano)
	if raw != canonical {
		return time.Time{}, fmt.Errorf("%s must use canonical UTC RFC3339Nano form", name)
	}
	return value, nil
}

func validateRecordingTime(
	recordedAt time.Time,
	workInterval projectprofile.WorkIntervalV1,
	basisWindow projectprofile.BasisObservationWindowV1,
) error {
	workUntil := workInterval.Until()
	if recordedAt.Before(workUntil) {
		return fmt.Errorf("recorded_at must not precede the Work interval end")
	}
	basisUntil := basisWindow.Until()
	if recordedAt.Before(basisUntil) {
		return fmt.Errorf("recorded_at must not precede the basis-observation window end")
	}
	return nil
}

func prepareMethodDescriptionRow(
	value projectprofile.ProfileOnboardingMethodDescriptionEdition,
	recordedAt string,
) (methodDescriptionRow, error) {
	canonical, digest, err := encodeProfileOnboardingMethodDescriptionEdition(value)
	if err != nil {
		return methodDescriptionRow{}, err
	}
	var projection methodDescriptionProjectionV1
	err = decodeProjectionV1(canonical, &projection, "MethodDescription")
	if err != nil {
		return methodDescriptionRow{}, err
	}
	canonicalText := string(canonical)
	digestText := digest.String()
	return methodDescriptionRow{
		methodDescriptionRef: projection.Ref,
		describedMethodRef:   projection.DescribedMethodRef,
		boundedContextRef:    projection.BoundedContextRef,
		sourceRevision:       projection.SourceRevision,
		edition:              projection.Edition,
		requiredRoleRef:      projection.RequiredRoleRef,
		requiredSystemKind:   projection.RequiredSystemKind,
		statePlaneRef:        projection.StatePlaneRef,
		affectedRefKind:      projection.AffectedRefKind,
		effectWitnessRuleRef: projection.EffectWitnessRuleRef,
		canonicalJSON:        canonicalText,
		digest:               digestText,
		recordedAt:           recordedAt,
	}, nil
}

func prepareMethodContractRow(
	value projectprofile.ProfileOnboardingMethodContractEdition,
	recordedAt string,
) (methodContractRow, error) {
	canonical, digest, err := encodeProfileOnboardingMethodContractEdition(value)
	if err != nil {
		return methodContractRow{}, err
	}
	var projection methodContractProjectionV1
	err = decodeProjectionV1(canonical, &projection, "MethodContract")
	if err != nil {
		return methodContractRow{}, err
	}
	acceptedResultKindsJSON := string(projection.AcceptedResultKinds)
	requiredOccurrenceSlotsJSON := string(projection.RequiredOccurrenceSlots)
	occurrenceCoverageRuleRefsJSON := string(projection.OccurrenceCoverageRuleRefs)
	canonicalText := string(canonical)
	digestText := digest.String()
	return methodContractRow{
		methodContractRef:                 projection.Ref,
		edition:                           projection.Edition,
		methodDescriptionRef:              projection.MethodDescriptionRef,
		methodDescriptionDigest:           projection.MethodDescriptionDigest,
		boundedContextRef:                 projection.BoundedContextRef,
		roleAdmissionPolicyRef:            projection.RoleAdmissionPolicyRef,
		systemAdmissionPolicyRef:          projection.SystemAdmissionPolicyRef,
		parameterSpecSetDigest:            projection.ParameterSpecSetDigest,
		acceptedResultKindsJSON:           acceptedResultKindsJSON,
		requiredOccurrenceSlotsJSON:       requiredOccurrenceSlotsJSON,
		occurrenceCoverageRuleRefsJSON:    occurrenceCoverageRuleRefsJSON,
		effectStateWitnessRuleRef:         projection.EffectStateWitnessRuleRef,
		acceptanceStandardRef:             projection.AcceptanceStandardRef,
		acceptanceStandardEdition:         projection.AcceptanceStandardEdition,
		holderEqualsExecutedWithinRuleRef: projection.HolderEqualsExecutedWithinRef,
		canonicalJSON:                     canonicalText,
		digest:                            digestText,
		recordedAt:                        recordedAt,
	}, nil
}

func encodeProfileOnboardingMethodDescriptionEdition(
	value projectprofile.ProfileOnboardingMethodDescriptionEdition,
) ([]byte, projectprofile.ContentDigest, error) {
	switch exact := value.(type) {
	case projectprofile.ProfileOnboardingMethodDescriptionV1:
		canonical, err := projectprofile.EncodeProfileOnboardingMethodDescriptionV1CanonicalJSON(exact)
		if err != nil {
			return nil, projectprofile.ContentDigest{}, fmt.Errorf("encode MethodDescription v1: %w", err)
		}
		digest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV1(exact)
		return canonical, digest, err
	case projectprofile.ProfileOnboardingMethodDescriptionV2:
		canonical, err := projectprofile.EncodeProfileOnboardingMethodDescriptionV2CanonicalJSON(exact)
		if err != nil {
			return nil, projectprofile.ContentDigest{}, fmt.Errorf("encode MethodDescription v2: %w", err)
		}
		digest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV2(exact)
		return canonical, digest, err
	default:
		return nil, projectprofile.ContentDigest{}, fmt.Errorf("unsupported profile-onboarding MethodDescription edition")
	}
}

func encodeProfileOnboardingMethodContractEdition(
	value projectprofile.ProfileOnboardingMethodContractEdition,
) ([]byte, projectprofile.ContentDigest, error) {
	switch exact := value.(type) {
	case projectprofile.ProfileOnboardingMethodContractV1:
		canonical, err := projectprofile.EncodeProfileOnboardingMethodContractV1CanonicalJSON(exact)
		if err != nil {
			return nil, projectprofile.ContentDigest{}, fmt.Errorf("encode MethodContract v1: %w", err)
		}
		digest, err := projectprofile.DigestProfileOnboardingMethodContractV1(exact)
		return canonical, digest, err
	case projectprofile.ProfileOnboardingMethodContractV2:
		canonical, err := projectprofile.EncodeProfileOnboardingMethodContractV2CanonicalJSON(exact)
		if err != nil {
			return nil, projectprofile.ContentDigest{}, fmt.Errorf("encode MethodContract v2: %w", err)
		}
		digest, err := projectprofile.DigestProfileOnboardingMethodContractV2(exact)
		return canonical, digest, err
	default:
		return nil, projectprofile.ContentDigest{}, fmt.Errorf("unsupported profile-onboarding MethodContract edition")
	}
}

func prepareSystemAdmissionRow(
	value projectprofile.ProfileOnboardingExecutorSystemAdmissionV1,
	recordedAt string,
) (systemAdmissionRow, error) {
	canonical, err := projectprofile.EncodeProfileOnboardingExecutorSystemAdmissionV1CanonicalJSON(value)
	if err != nil {
		return systemAdmissionRow{}, fmt.Errorf("encode executor-system admission: %w", err)
	}
	digest, err := projectprofile.DigestProfileOnboardingExecutorSystemAdmissionV1(value)
	if err != nil {
		return systemAdmissionRow{}, fmt.Errorf("digest executor-system admission: %w", err)
	}
	var projection systemAdmissionProjectionV1
	err = decodeProjectionV1(canonical, &projection, "executor-system admission")
	if err != nil {
		return systemAdmissionRow{}, err
	}
	kernelIdentity := ""
	kernelVersion := ""
	if projection.IdentityBasis.KernelOwned != nil {
		kernelIdentity = projection.IdentityBasis.KernelOwned.Identity
		kernelVersion = projection.IdentityBasis.KernelOwned.Version
	}
	designationRef := ""
	designationDigest := ""
	if projection.IdentityBasis.OperatorDesignated != nil {
		designationRef = projection.IdentityBasis.OperatorDesignated.Ref
		designationDigest = projection.IdentityBasis.OperatorDesignated.Digest
	}
	canonicalText := string(canonical)
	digestText := digest.String()
	return systemAdmissionRow{
		ref:                            projection.Ref,
		systemRef:                      projection.SystemRef,
		admittedSystemKind:             projection.AdmittedSystemKind,
		boundedContextRef:              projection.BoundedContextRef,
		governingPatternRef:            projection.GoverningPatternRef,
		identityBasisKind:              projection.IdentityBasis.Kind,
		identityBasisSystemRef:         projection.IdentityBasis.SystemRef,
		identityBasisKernelIdentity:    kernelIdentity,
		identityBasisKernelVersion:     kernelVersion,
		identityBasisDesignationRef:    designationRef,
		identityBasisDesignationDigest: designationDigest,
		actingEligibilityBasisRef:      projection.ActingEligibilityBasisRef,
		actingEligibilityBasisDigest:   projection.ActingEligibilityBasisDigest,
		sessionRef:                     projection.SessionRef,
		validFrom:                      projection.ValidityWindow.From,
		validUntil:                     projection.ValidityWindow.Until,
		methodDescriptionRef:           projection.MethodDescriptionRef,
		methodDescriptionDigest:        projection.MethodDescriptionDigest,
		methodContractRef:              projection.MethodContractRef,
		methodContractDigest:           projection.MethodContractDigest,
		admissionPolicyRef:             projection.SystemAdmissionPolicyRef,
		canonicalJSON:                  canonicalText,
		digest:                         digestText,
		recordedAt:                     recordedAt,
	}, nil
}

func prepareRoleAdmissionRow(
	value projectprofile.ProfileAuthorRoleAdmissionV1,
	recordedAt string,
) (roleAdmissionRow, error) {
	canonical, err := projectprofile.EncodeProfileAuthorRoleAdmissionV1CanonicalJSON(value)
	if err != nil {
		return roleAdmissionRow{}, fmt.Errorf("encode ProfileAuthor role admission: %w", err)
	}
	digest, err := projectprofile.DigestProfileAuthorRoleAdmissionV1(value)
	if err != nil {
		return roleAdmissionRow{}, fmt.Errorf("digest ProfileAuthor role admission: %w", err)
	}
	var projection roleAdmissionProjectionV1
	err = decodeProjectionV1(canonical, &projection, "ProfileAuthor role admission")
	if err != nil {
		return roleAdmissionRow{}, err
	}
	canonicalText := string(canonical)
	digestText := digest.String()
	return roleAdmissionRow{
		ref:                     projection.Ref,
		roleRef:                 projection.RoleRef,
		boundedContextRef:       projection.BoundedContextRef,
		governingPatternRef:     projection.GoverningPatternRef,
		methodDescriptionRef:    projection.MethodDescriptionRef,
		methodDescriptionDigest: projection.MethodDescriptionDigest,
		methodContractRef:       projection.MethodContractRef,
		methodContractDigest:    projection.MethodContractDigest,
		admissionPolicyRef:      projection.RoleAdmissionPolicyRef,
		canonicalJSON:           canonicalText,
		digest:                  digestText,
		recordedAt:              recordedAt,
	}, nil
}

func prepareAssignmentSupportRow(
	justification projectprofile.ProfileAuthorAssignmentJustificationV1,
	provenance projectprofile.ProfileAuthorAssignmentProvenanceV1,
	recordedAt string,
) (assignmentSupportRow, error) {
	justificationJSON, err := projectprofile.EncodeProfileAuthorAssignmentJustificationV1CanonicalJSON(justification)
	if err != nil {
		return assignmentSupportRow{}, fmt.Errorf("encode assignment justification: %w", err)
	}
	justificationDigest, err := projectprofile.DigestProfileAuthorAssignmentJustificationV1(justification)
	if err != nil {
		return assignmentSupportRow{}, fmt.Errorf("digest assignment justification: %w", err)
	}
	provenanceJSON, err := projectprofile.EncodeProfileAuthorAssignmentProvenanceV1CanonicalJSON(provenance)
	if err != nil {
		return assignmentSupportRow{}, fmt.Errorf("encode assignment provenance: %w", err)
	}
	provenanceDigest, err := projectprofile.DigestProfileAuthorAssignmentProvenanceV1(provenance)
	if err != nil {
		return assignmentSupportRow{}, fmt.Errorf("digest assignment provenance: %w", err)
	}
	var justificationProjection assignmentJustificationProjectionV1
	err = decodeProjectionV1(justificationJSON, &justificationProjection, "assignment justification")
	if err != nil {
		return assignmentSupportRow{}, err
	}
	var provenanceProjection assignmentProvenanceProjectionV1
	err = decodeProjectionV1(provenanceJSON, &provenanceProjection, "assignment provenance")
	if err != nil {
		return assignmentSupportRow{}, err
	}
	justificationText := string(justificationJSON)
	justificationDigestText := justificationDigest.String()
	provenanceText := string(provenanceJSON)
	provenanceDigestText := provenanceDigest.String()
	return assignmentSupportRow{
		justificationRef:              justificationProjection.Ref,
		ruleRef:                       justificationProjection.RuleRef,
		ruleStatement:                 justificationProjection.RuleStatement,
		boundedContextRef:             justificationProjection.BoundedContextRef,
		systemAdmissionRef:            justificationProjection.SystemAdmissionRef,
		systemAdmissionDigest:         justificationProjection.SystemAdmissionDigest,
		roleAdmissionRef:              justificationProjection.RoleAdmissionRef,
		roleAdmissionDigest:           justificationProjection.RoleAdmissionDigest,
		assignmentFrom:                justificationProjection.AssignmentWindow.From,
		assignmentUntil:               justificationProjection.AssignmentWindow.Until,
		methodContractRef:             justificationProjection.MethodContractRef,
		methodContractDigest:          justificationProjection.MethodContractDigest,
		justificationJSON:             justificationText,
		justificationDigest:           justificationDigestText,
		provenanceRef:                 provenanceProjection.Ref,
		provenanceJustificationRef:    provenanceProjection.JustificationRef,
		provenanceJustificationDigest: provenanceProjection.JustificationDigest,
		sessionRef:                    provenanceProjection.SessionRef,
		kernelIdentity:                provenanceProjection.Kernel.Identity,
		kernelVersion:                 provenanceProjection.Kernel.Version,
		runtimeIdentity:               provenanceProjection.Runtime.Identity,
		runtimeVersion:                provenanceProjection.Runtime.Version,
		provenanceRecordedAt:          provenanceProjection.RecordedAt,
		provenanceJSON:                provenanceText,
		provenanceDigest:              provenanceDigestText,
		recordedAt:                    recordedAt,
	}, nil
}

func prepareRoleAssignmentRow(
	value projectprofile.ProfileAuthorRoleAssignmentV1,
	recordedAt string,
) (roleAssignmentRow, error) {
	canonical, err := projectprofile.EncodeProfileAuthorRoleAssignmentV1CanonicalJSON(value)
	if err != nil {
		return roleAssignmentRow{}, fmt.Errorf("encode ProfileAuthorRoleAssignment: %w", err)
	}
	digest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(value)
	if err != nil {
		return roleAssignmentRow{}, fmt.Errorf("digest ProfileAuthorRoleAssignment: %w", err)
	}
	var projection roleAssignmentProjectionV1
	err = decodeProjectionV1(canonical, &projection, "ProfileAuthorRoleAssignment")
	if err != nil {
		return roleAssignmentRow{}, err
	}
	canonicalText := string(canonical)
	digestText := digest.String()
	return roleAssignmentRow{
		ref:                   projection.RoleAssignmentRef,
		holderSystemRef:       projection.HolderSystemRef,
		admittedRoleRef:       projection.AdmittedRoleRef,
		boundedContextRef:     projection.BoundedContextRef,
		validFrom:             projection.ValidityWindow.From,
		validUntil:            projection.ValidityWindow.Until,
		systemAdmissionRef:    projection.SystemAdmissionRef,
		systemAdmissionDigest: projection.SystemAdmissionDigest,
		roleAdmissionRef:      projection.RoleAdmissionRef,
		roleAdmissionDigest:   projection.RoleAdmissionDigest,
		justificationRef:      projection.AssignmentJustificationRef,
		justificationDigest:   projection.AssignmentJustificationDig,
		provenanceRef:         projection.AssignmentProvenanceRef,
		provenanceDigest:      projection.AssignmentProvenanceDigest,
		canonicalJSON:         canonicalText,
		digest:                digestText,
		recordedAt:            recordedAt,
	}, nil
}

func prepareObservedBasisRow(
	value projectprofile.ObservedProjectBasisV1,
	recordedAt string,
) (observedBasisRow, error) {
	canonical, err := projectprofile.EncodeObservedProjectBasisV1CanonicalJSON(value)
	if err != nil {
		return observedBasisRow{}, fmt.Errorf("encode ObservedProjectBasis: %w", err)
	}
	digest, err := projectprofile.DigestObservedProjectBasisV1(value)
	if err != nil {
		return observedBasisRow{}, fmt.Errorf("digest ObservedProjectBasis: %w", err)
	}
	var projection observedBasisProjectionV1
	err = decodeProjectionV1(canonical, &projection, "ObservedProjectBasis")
	if err != nil {
		return observedBasisRow{}, err
	}
	canonicalText := string(canonical)
	digestText := digest.String()
	return observedBasisRow{
		ref:               projection.Ref,
		projectRoot:       projection.ProjectRoot,
		observationFrom:   projection.ObservationWindow.From,
		observationUntil:  projection.ObservationWindow.Until,
		detectorVersion:   projection.DetectorVersion,
		classifierVersion: projection.ClassifierVersion,
		canonicalJSON:     canonicalText,
		digest:            digestText,
		recordedAt:        recordedAt,
	}, nil
}

func prepareWorkRow(
	projectRoot projectprofile.ProjectRootV1,
	value projectprofile.ProfileOnboardingWorkRecord,
	recordedAt string,
) (workRow, error) {
	canonical, err := projectprofile.EncodeProfileOnboardingWorkRecordCanonicalJSON(value)
	if err != nil {
		return workRow{}, fmt.Errorf("encode profile-onboarding Work: %w", err)
	}
	digest, err := projectprofile.DigestProfileOnboardingWorkRecord(value)
	if err != nil {
		return workRow{}, fmt.Errorf("digest profile-onboarding Work: %w", err)
	}
	var projection workProjectionV1
	err = decodeProjectionV1(canonical, &projection, "profile-onboarding Work")
	if err != nil {
		return workRow{}, err
	}
	outcomeKind, err := storageWorkOutcomeKindV1(projection.Outcome.Kind)
	if err != nil {
		return workRow{}, err
	}
	projectRootText := projectRoot.String()
	parameterBindingsJSON := string(projection.ParameterBindings)
	inputsJSON := string(projection.InputRefs)
	outputsJSON := string(projection.OutputRefs)
	resourcesJSON := string(projection.ResourceRefs)
	affectedRefsJSON := string(projection.AffectedRefs)
	canonicalText := string(canonical)
	digestText := digest.String()
	return workRow{
		workRecordRef:                     projection.RecordRef,
		workRef:                           projection.WorkRef,
		projectRoot:                       projectRootText,
		enactsMethodRef:                   projection.EnactsMethodRef,
		methodDescriptionRef:              projection.MethodDescriptionRef,
		methodDescriptionDigest:           projection.MethodDescriptionDigest,
		methodContractRef:                 projection.MethodContractRef,
		methodContractDigest:              projection.MethodContractDigest,
		parameterBindingsJSON:             parameterBindingsJSON,
		performedByRoleAssignmentRef:      projection.PerformedBy,
		profileAuthorRoleAssignmentRef:    projection.ProfileAuthorRoleAssignmentRef,
		profileAuthorRoleAssignmentDigest: projection.ProfileAuthorRoleAssignmentDigest,
		executedWithinRef:                 projection.ExecutedWithin,
		workFrom:                          projection.WorkInterval.From,
		workUntil:                         projection.WorkInterval.Until,
		boundedContextRef:                 projection.BoundedContextRef,
		basisObservationFrom:              projection.BasisObservationWindow.From,
		basisObservationUntil:             projection.BasisObservationWindow.Until,
		observedProjectBasisRef:           projection.ObservedProjectBasisRef,
		observedProjectBasisDigest:        projection.ObservedProjectBasisDigest,
		inputsJSON:                        inputsJSON,
		outputsJSON:                       outputsJSON,
		resourcesJSON:                     resourcesJSON,
		affectedRefKind:                   projection.AffectedRefKind,
		affectedRefsJSON:                  affectedRefsJSON,
		statePlaneRef:                     projection.StatePlaneRef,
		preStateRef:                       projection.StateTransition.PreStateRef,
		postStateRef:                      projection.StateTransition.PostStateRef,
		deltaPredicateRef:                 projection.StateTransition.DeltaPredicateRef,
		outcomeKind:                       outcomeKind,
		profilePayloadDigest:              projection.Outcome.PayloadDigest,
		observedBasisDigest:               projection.Outcome.ObservedBasisDigest,
		missingBasisDigest:                projection.Outcome.MissingBasisDigest,
		canonicalJSON:                     canonicalText,
		digest:                            digestText,
		recordedAt:                        recordedAt,
	}, nil
}

func storageWorkOutcomeKindV1(raw string) (string, error) {
	switch raw {
	case "candidate_payload_produced":
		return "CandidatePayloadProduced", nil
	case "classification_underdetermined":
		return "ClassificationUnderdetermined", nil
	default:
		return "", fmt.Errorf("unknown profile-onboarding Work outcome %q", raw)
	}
}

func prepareEffectRow(
	value projectprofile.ProfileOnboardingEffectV1,
	recordedAt string,
) (effectRow, error) {
	canonical, err := projectprofile.EncodeProfileOnboardingEffectV1CanonicalJSON(value)
	if err != nil {
		return effectRow{}, fmt.Errorf("encode profile-onboarding effect: %w", err)
	}
	digest, err := projectprofile.DigestProfileOnboardingEffectV1(value)
	if err != nil {
		return effectRow{}, fmt.Errorf("digest profile-onboarding effect: %w", err)
	}
	var projection effectProjectionV1
	err = decodeProjectionV1(canonical, &projection, "profile-onboarding effect")
	if err != nil {
		return effectRow{}, err
	}
	affectedEntityRefsJSON := string(projection.AffectedEntityRefs)
	evidencePathRefsJSON := string(projection.EvidenceProvenancePathRefs)
	canonicalText := string(canonical)
	digestText := digest.String()
	return effectRow{
		ref:                        projection.Ref,
		workRecordRef:              projection.WorkRecordRef,
		workRef:                    projection.WorkRef,
		workRecordDigest:           projection.WorkRecordDigest,
		resultKind:                 projection.Result.Kind,
		outputRef:                  projection.Result.OutputRef,
		profilePayloadDigest:       projection.Result.PayloadDigest,
		observedProjectBasisRef:    projection.Result.ObservedProjectBasisRef,
		observedProjectBasisDigest: projection.Result.ObservedProjectBasisDigest,
		missingBasisDigest:         projection.Result.MissingBasisDigest,
		affectedEntityRefsJSON:     affectedEntityRefsJSON,
		statePlaneRef:              projection.StatePlaneRef,
		preStateRef:                projection.StateWitness.PreStateRef,
		postStateRef:               projection.StateWitness.PostStateRef,
		deltaPredicateRef:          projection.StateWitness.DeltaPredicateRef,
		evidencePathRefsJSON:       evidencePathRefsJSON,
		canonicalJSON:              canonicalText,
		digest:                     digestText,
		recordedAt:                 recordedAt,
	}, nil
}

func prepareAssessmentRow(
	value projectprofile.ProfileOnboardingOutcomeAssessmentV1,
	recordedAt string,
) (assessmentRow, error) {
	canonical, err := projectprofile.EncodeProfileOnboardingOutcomeAssessmentV1CanonicalJSON(value)
	if err != nil {
		return assessmentRow{}, fmt.Errorf("encode profile-onboarding assessment: %w", err)
	}
	digest, err := projectprofile.DigestProfileOnboardingOutcomeAssessmentV1(value)
	if err != nil {
		return assessmentRow{}, fmt.Errorf("digest profile-onboarding assessment: %w", err)
	}
	var projection assessmentProjectionV1
	err = decodeProjectionV1(canonical, &projection, "profile-onboarding assessment")
	if err != nil {
		return assessmentRow{}, err
	}
	evidencePathRefsJSON := string(projection.EvidenceProvenancePathRefs)
	canonicalText := string(canonical)
	digestText := digest.String()
	return assessmentRow{
		ref:                       projection.Ref,
		effectRef:                 projection.EffectRef,
		effectDigest:              projection.EffectDigest,
		workRecordRef:             projection.WorkRecordRef,
		workRef:                   projection.WorkRef,
		workRecordDigest:          projection.WorkRecordDigest,
		acceptanceStandardRef:     projection.AcceptanceStandardRef,
		acceptanceStandardEdition: projection.AcceptanceStandardEdition,
		comparatorRef:             projection.ComparatorRef,
		comparatorEdition:         projection.ComparatorEdition,
		verdictKind:               projection.Verdict.Kind,
		verdictReasonRef:          projection.Verdict.ReasonRef,
		missingBasisDigest:        projection.Verdict.MissingBasisDigest,
		evidencePathRefsJSON:      evidencePathRefsJSON,
		canonicalJSON:             canonicalText,
		digest:                    digestText,
		recordedAt:                recordedAt,
	}, nil
}
