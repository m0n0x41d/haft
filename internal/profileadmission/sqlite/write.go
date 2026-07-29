package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const insertAdmissionSQL = `INSERT INTO project_profile_admissions_v2 (
	admission_id, action_kind, project_root, project_binding_digest,
	profile_payload_json, candidate_provenance_json, candidate_provenance_digest,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest,
	profile_payload_digest, observed_project_basis_ref, observed_project_basis_digest,
	work_record_ref, work_record_digest,
	outcome_assessment_ref, outcome_assessment_digest,
	authority_basis_ref, authority_basis_digest,
	authority_resolution_ref, authority_resolution_digest,
	receipt_json, receipt_digest,
	expected_ledger_revision, ledger_revision,
	single_use_key, admission_request_digest,
	admission_json, admission_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertAuthorityUseSQL = `INSERT INTO profile_declaration_authority_uses_v2 (
	use_ref, use_digest, project_root, action_kind, project_binding_digest,
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	permission_ref, permission_digest,
	authorization_content_ref, authorization_content_digest,
	single_use_key, admission_request_digest,
	committed_admission_ref, committed_admission_digest,
	canonical_json, consumed_at, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertRevisionSQL = `INSERT INTO project_profile_revisions_v2 (
	project_root, ledger_revision, configured_profile_kind,
	profile_payload_json, profile_payload_digest,
	receipt_json, receipt_digest,
	admission_id, admission_digest, recorded_at
) VALUES (?, ?, 'Declared', ?, ?, ?, ?, ?, ?, ?)`

const insertAdmissionV3SQL = `INSERT INTO project_profile_admissions_v3 (
	admission_id, action_kind, project_root, authority_mode, resolution_kind,
	project_binding_digest, work_input_ref, work_input_digest,
	profile_payload_json, candidate_provenance_json, candidate_provenance_digest,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest,
	profile_payload_digest, observed_project_basis_ref, observed_project_basis_digest,
	work_record_ref, work_record_digest,
	outcome_assessment_ref, outcome_assessment_digest,
	authority_basis_ref, authority_basis_digest,
	authority_resolution_ref, authority_resolution_digest,
	receipt_json, receipt_digest,
	expected_ledger_revision, ledger_revision,
	single_use_key, admission_request_digest,
	admission_json, admission_digest, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertAuthorityUseV3SQL = `INSERT INTO profile_declaration_authority_uses_v3 (
	use_ref, use_digest, project_root, action_kind, authority_mode, resolution_kind,
	project_binding_digest,
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	work_input_ref, work_input_digest,
	single_use_key, admission_request_digest,
	committed_admission_ref, committed_admission_digest,
	canonical_json, consumed_at, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertRevisionV3SQL = `INSERT INTO project_profile_revisions_v3 (
	project_root, ledger_revision, configured_profile_kind,
	profile_payload_json, profile_payload_digest,
	receipt_json, receipt_digest,
	admission_id, admission_digest, recorded_at
) VALUES (?, ?, 'Declared', ?, ?, ?, ?, ?, ?, ?)`

type writeMaterial struct {
	prepared          projectprofile.PreparedProfileAdmissionV1
	tentative         projectprofile.TentativeProfileAdmissionTransactionMaterialV1
	authority         authorityMaterial
	authorityUse      profileauthority.AuthorityUseRecord
	v3AuthorityUse    v3AuthorityUseRecord
	admissionRef      projectprofile.ProfileDeclarationAdmissionRecordRef
	useRef            string
	recordedAt        time.Time
	committedRevision projectprofile.LedgerRevision
}

func newWriteMaterialV3(
	prepared projectprofile.PreparedProfileAdmissionV1,
	authorityValue authorityMaterial,
) (writeMaterial, error) {
	expectedRevision := prepared.ExpectedLedgerRevision()
	committedRevision, err := expectedRevision.Next()
	if err != nil {
		return writeMaterial{}, fmt.Errorf("advance profile ledger revision: %w", err)
	}
	requestDigest := prepared.AdmissionRequestDigest()
	digestSuffix := strings.TrimPrefix(requestDigest.String(), "sha256:")
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(
		"profile-admission." + digestSuffix,
	)
	if err != nil {
		return writeMaterial{}, fmt.Errorf("derive admission-record ref: %w", err)
	}
	recordedAt := authorityValue.judgementTime.UTC().Round(0)
	tentative, err := projectprofile.PrepareTentativeProfileAdmissionTransactionMaterialV1(
		prepared,
		committedRevision,
		recordedAt,
		admissionRef,
	)
	if err != nil {
		return writeMaterial{}, fmt.Errorf("prepare tentative admission material: %w", err)
	}
	use, err := newV3AuthorityUseRecord(
		"profile-authority-use:"+digestSuffix,
		authorityValue,
		requestDigest.String(),
		admissionRef.String(),
		tentative.TentativeAdmissionRecordDigest().String(),
		recordedAt,
	)
	if err != nil {
		return writeMaterial{}, err
	}
	return writeMaterial{
		prepared:          prepared,
		tentative:         tentative,
		authority:         authorityValue,
		v3AuthorityUse:    use,
		admissionRef:      admissionRef,
		useRef:            use.ref,
		recordedAt:        recordedAt,
		committedRevision: committedRevision,
	}, nil
}

func newWriteMaterial(
	prepared projectprofile.PreparedProfileAdmissionV1,
	authorityValue authorityMaterial,
) (writeMaterial, error) {
	expectedRevision := prepared.ExpectedLedgerRevision()
	committedRevision, err := expectedRevision.Next()
	if err != nil {
		return writeMaterial{}, fmt.Errorf("advance profile ledger revision: %w", err)
	}
	requestDigestValue := prepared.AdmissionRequestDigest()
	requestDigestRaw := requestDigestValue.String()
	digestSuffix := strings.TrimPrefix(requestDigestRaw, "sha256:")
	admissionRefText := "profile-admission." + digestSuffix
	admissionRef, err := projectprofile.NewProfileDeclarationAdmissionRecordRef(admissionRefText)
	if err != nil {
		return writeMaterial{}, fmt.Errorf("derive admission-record ref: %w", err)
	}
	recordedAt := authorityValue.judgementTime
	recordedAt = recordedAt.UTC()
	tentative, err := projectprofile.PrepareTentativeProfileAdmissionTransactionMaterialV1(
		prepared,
		committedRevision,
		recordedAt,
		admissionRef,
	)
	if err != nil {
		return writeMaterial{}, fmt.Errorf("prepare tentative admission material: %w", err)
	}
	useRef, err := profileauthority.NewProfileDeclarationAuthorityUseRef(
		"profile-authority-use:" + digestSuffix,
	)
	if err != nil {
		return writeMaterial{}, fmt.Errorf("derive authority-use ref: %w", err)
	}
	requestDigest, err := authority.NewDigest(requestDigestRaw)
	if err != nil {
		return writeMaterial{}, fmt.Errorf("convert admission-request digest: %w", err)
	}
	committedRef, err := profileauthority.NewCommittedProfileAdmissionRef(admissionRef.String())
	if err != nil {
		return writeMaterial{}, fmt.Errorf("convert committed admission ref: %w", err)
	}
	tentativeDigest := tentative.TentativeAdmissionRecordDigest()
	committedDigest, err := authority.NewDigest(tentativeDigest.String())
	if err != nil {
		return writeMaterial{}, fmt.Errorf("convert committed admission digest: %w", err)
	}
	useResult := profileauthority.EvaluateNewAuthorityUse(
		useRef,
		authorityValue.admittedUse,
		requestDigest,
		committedRef,
		committedDigest,
		recordedAt,
	)
	if useResult.Kind() != profileauthority.AuthorityUseNew {
		return writeMaterial{}, fmt.Errorf("profile authority use gate rejected canonical admission material")
	}
	newUse, ok := useResult.New()
	if !ok {
		return writeMaterial{}, fmt.Errorf("profile authority use gate returned no canonical use")
	}
	authorityUse, ok := newUse.Record()
	if !ok {
		return writeMaterial{}, fmt.Errorf("profile authority use record is unavailable")
	}
	return writeMaterial{
		prepared:          prepared,
		tentative:         tentative,
		authority:         authorityValue,
		authorityUse:      authorityUse,
		admissionRef:      admissionRef,
		useRef:            useRef.String(),
		recordedAt:        recordedAt,
		committedRevision: committedRevision,
	}, nil
}

type admissionWriteRow struct {
	admissionID               string
	actionKind                string
	projectRoot               string
	projectBindingDigest      string
	payloadJSON               string
	provenanceJSON            string
	provenanceDigest          string
	assignmentRef             string
	assignmentDigest          string
	payloadDigest             string
	basisRef                  string
	basisDigest               string
	workRef                   string
	workDigest                string
	assessmentRef             string
	assessmentDigest          string
	authorityBasisRef         string
	authorityBasisDigest      string
	authorityResolutionRef    string
	authorityResolutionDigest string
	receiptJSON               string
	receiptDigest             string
	expectedRevision          uint64
	committedRevision         uint64
	singleUseKey              string
	admissionRequestDigest    string
	admissionJSON             string
	admissionDigest           string
	recordedAt                string
}

type authorityUseWriteRow struct {
	useID                     string
	useDigest                 string
	projectRoot               string
	actionKind                string
	projectBindingDigest      string
	authorityResolutionRef    string
	authorityResolutionDigest string
	authorityBasisRef         string
	authorityBasisDigest      string
	permissionRef             string
	permissionDigest          string
	authorizationContentRef   string
	authorizationContentHash  string
	singleUseKey              string
	admissionRequestDigest    string
	committedResultRef        string
	committedResultDigest     string
	canonicalJSON             string
	consumedAt                string
	recordedAt                string
}

type revisionWriteRow struct {
	projectRoot     string
	ledgerRevision  uint64
	payloadJSON     string
	payloadDigest   string
	receiptJSON     string
	receiptDigest   string
	admissionID     string
	admissionDigest string
	recordedAt      string
}

func (row admissionWriteRow) args() []any {
	return []any{
		row.admissionID, row.actionKind, row.projectRoot, row.projectBindingDigest,
		row.payloadJSON, row.provenanceJSON, row.provenanceDigest,
		row.assignmentRef, row.assignmentDigest, row.payloadDigest,
		row.basisRef, row.basisDigest, row.workRef, row.workDigest,
		row.assessmentRef, row.assessmentDigest,
		row.authorityBasisRef, row.authorityBasisDigest,
		row.authorityResolutionRef, row.authorityResolutionDigest,
		row.receiptJSON, row.receiptDigest,
		row.expectedRevision, row.committedRevision,
		row.singleUseKey, row.admissionRequestDigest,
		row.admissionJSON, row.admissionDigest, row.recordedAt,
	}
}

func (row authorityUseWriteRow) args() []any {
	return []any{
		row.useID, row.useDigest, row.projectRoot, row.actionKind,
		row.projectBindingDigest, row.authorityResolutionRef,
		row.authorityResolutionDigest, row.authorityBasisRef,
		row.authorityBasisDigest, row.permissionRef, row.permissionDigest,
		row.authorizationContentRef, row.authorizationContentHash,
		row.singleUseKey, row.admissionRequestDigest,
		row.committedResultRef, row.committedResultDigest,
		row.canonicalJSON, row.consumedAt, row.recordedAt,
	}
}

func (row revisionWriteRow) args() []any {
	return []any{
		row.projectRoot, row.ledgerRevision, row.payloadJSON, row.payloadDigest,
		row.receiptJSON, row.receiptDigest, row.admissionID,
		row.admissionDigest, row.recordedAt,
	}
}

func persistAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	material writeMaterial,
) error {
	row := buildAdmissionWriteRow(material)
	args := row.args()
	_, err := transaction.Execute(ctx, insertAdmissionSQL, args)
	if err != nil {
		return fmt.Errorf("insert profile admission: %w", err)
	}
	return nil
}

func persistAuthorityUse(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	material writeMaterial,
) error {
	row := buildAuthorityUseWriteRow(material)
	args := row.args()
	_, err := transaction.Execute(ctx, insertAuthorityUseSQL, args)
	if err != nil {
		return fmt.Errorf("insert authority use: %w", err)
	}
	return nil
}

func persistRevision(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	material writeMaterial,
) error {
	row := buildRevisionWriteRow(material)
	args := row.args()
	_, err := transaction.Execute(ctx, insertRevisionSQL, args)
	if err != nil {
		return fmt.Errorf("insert project profile revision: %w", err)
	}
	return nil
}

func persistAdmissionV3(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	material writeMaterial,
) error {
	row := buildAdmissionWriteRow(material)
	authorityValue := material.authority
	args := []any{
		row.admissionID, row.actionKind, row.projectRoot,
		authorityValue.authorityMode, authorityValue.resolutionKind,
		row.projectBindingDigest,
		authorityValue.workInputRef, authorityValue.workInputDigest,
		row.payloadJSON, row.provenanceJSON, row.provenanceDigest,
		row.assignmentRef, row.assignmentDigest, row.payloadDigest,
		row.basisRef, row.basisDigest, row.workRef, row.workDigest,
		row.assessmentRef, row.assessmentDigest,
		row.authorityBasisRef, row.authorityBasisDigest,
		row.authorityResolutionRef, row.authorityResolutionDigest,
		row.receiptJSON, row.receiptDigest,
		row.expectedRevision, row.committedRevision,
		row.singleUseKey, row.admissionRequestDigest,
		row.admissionJSON, row.admissionDigest, row.recordedAt,
	}
	_, err := transaction.Execute(ctx, insertAdmissionV3SQL, args)
	if err != nil {
		return fmt.Errorf("insert v3 profile admission: %w", err)
	}
	return nil
}

func persistAuthorityUseV3(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	material writeMaterial,
) error {
	use := material.v3AuthorityUse
	args := []any{
		use.ref, use.digest, use.projectRoot, use.actionKind,
		use.mode, use.resolutionKind, use.projectBindingDigest,
		use.authorityResolutionRef, use.authorityResolutionDigest,
		use.authorityBasisRef, use.authorityBasisDigest,
		use.workInputRef, use.workInputDigest, use.singleUseKey,
		use.admissionRequestDigest, use.committedAdmissionRef,
		use.committedAdmissionDigest, use.canonicalJSON,
		use.consumedAt, use.consumedAt,
	}
	_, err := transaction.Execute(ctx, insertAuthorityUseV3SQL, args)
	if err != nil {
		return fmt.Errorf("insert v3 authority use: %w", err)
	}
	return nil
}

func persistRevisionV3(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	material writeMaterial,
) error {
	row := buildRevisionWriteRow(material)
	_, err := transaction.Execute(ctx, insertRevisionV3SQL, row.args())
	if err != nil {
		return fmt.Errorf("insert v3 project profile revision: %w", err)
	}
	return nil
}

func buildAdmissionWriteRow(material writeMaterial) admissionWriteRow {
	prepared := material.prepared
	tentative := material.tentative
	plan := prepared.CommitPlan()
	inputs := plan.Inputs()
	candidate := inputs.Candidate()
	provenance := candidate.Provenance()
	assignment := prepared.ProfileAuthorRoleAssignment()
	basis := prepared.ObservedProjectBasis()
	work := prepared.WorkRecord()
	assessment := prepared.OutcomeAssessment()
	actionKind := material.authority.actionKind
	singleUseKey := material.authority.singleUseKey
	authorityBasisDigest := material.authority.authorityBasisDigest
	payloadJSON := prepared.ProfilePayloadCanonicalJSON()
	provenanceJSON := prepared.CandidateProvenanceCanonicalJSON()
	receiptJSON := tentative.TentativeReceiptCanonicalJSON()
	admissionJSON := tentative.TentativeAdmissionRecordCanonicalJSON()
	projectRoot := prepared.ProjectRoot()
	provenanceDigest := prepared.CandidateProvenanceDigest()
	assignmentRef := assignment.RoleAssignmentRef()
	assignmentDigest := prepared.ProfileAuthorRoleAssignmentDigest()
	payloadDigest := prepared.ProfilePayloadDigest()
	basisRef := basis.Ref()
	basisDigest := prepared.ObservedProjectBasisDigest()
	workRef := work.RecordRef()
	workDigest := prepared.WorkRecordDigest()
	assessmentRef := assessment.Ref()
	assessmentDigest := prepared.OutcomeAssessmentDigest()
	authorityBasisRef := provenance.AuthorityBasisRef()
	receiptDigest := tentative.TentativeReceiptDigest()
	expectedRevision := prepared.ExpectedLedgerRevision()
	requestDigest := prepared.AdmissionRequestDigest()
	admissionDigest := tentative.TentativeAdmissionRecordDigest()
	return admissionWriteRow{
		admissionID:               material.admissionRef.String(),
		actionKind:                actionKind.String(),
		projectRoot:               projectRoot.String(),
		projectBindingDigest:      material.authority.projectBindingHash.String(),
		payloadJSON:               string(payloadJSON),
		provenanceJSON:            string(provenanceJSON),
		provenanceDigest:          provenanceDigest.String(),
		assignmentRef:             assignmentRef.String(),
		assignmentDigest:          assignmentDigest.String(),
		payloadDigest:             payloadDigest.String(),
		basisRef:                  basisRef.String(),
		basisDigest:               basisDigest.String(),
		workRef:                   workRef.String(),
		workDigest:                workDigest.String(),
		assessmentRef:             assessmentRef.String(),
		assessmentDigest:          assessmentDigest.String(),
		authorityBasisRef:         authorityBasisRef.String(),
		authorityBasisDigest:      authorityBasisDigest.String(),
		authorityResolutionRef:    material.authority.resolutionRef.String(),
		authorityResolutionDigest: material.authority.resolutionDigest.String(),
		receiptJSON:               string(receiptJSON),
		receiptDigest:             receiptDigest.String(),
		expectedRevision:          expectedRevision.Value(),
		committedRevision:         material.committedRevision.Value(),
		singleUseKey:              singleUseKey.String(),
		admissionRequestDigest:    requestDigest.String(),
		admissionJSON:             string(admissionJSON),
		admissionDigest:           admissionDigest.String(),
		recordedAt:                formatTime(material.recordedAt),
	}
}

func buildAuthorityUseWriteRow(material writeMaterial) authorityUseWriteRow {
	use := material.authorityUse
	useRef, _ := use.Ref()
	useDigest, _ := use.Digest()
	projectRoot, actionKind, projectBindingDigest, _ := use.ProjectBinding()
	resolutionRef, resolutionDigest, _ := use.Resolution()
	basisRef, basisDigest, _ := use.Basis()
	permissionRef, permissionDigest, _ := use.Permission()
	contentRef, contentDigest, _ := use.AuthorizationContent()
	singleUseKey, _ := use.SingleUseKey()
	requestDigest, _ := use.AdmissionRequestDigest()
	committedRef, committedDigest, _ := use.CommittedAdmission()
	consumedAt, _ := use.ConsumedAt()
	canonical, _ := use.CanonicalBytes()
	return authorityUseWriteRow{
		useID:                     useRef.String(),
		useDigest:                 useDigest.String(),
		projectRoot:               projectRoot.String(),
		actionKind:                actionKind.String(),
		projectBindingDigest:      projectBindingDigest.String(),
		authorityResolutionRef:    resolutionRef.String(),
		authorityResolutionDigest: resolutionDigest.String(),
		authorityBasisRef:         basisRef.String(),
		authorityBasisDigest:      basisDigest.String(),
		permissionRef:             permissionRef.String(),
		permissionDigest:          permissionDigest.String(),
		authorizationContentRef:   contentRef.String(),
		authorizationContentHash:  contentDigest.String(),
		singleUseKey:              singleUseKey.String(),
		admissionRequestDigest:    requestDigest.String(),
		committedResultRef:        committedRef.String(),
		committedResultDigest:     committedDigest.String(),
		canonicalJSON:             string(canonical),
		consumedAt:                formatTime(consumedAt),
		recordedAt:                formatTime(consumedAt),
	}
}

func buildRevisionWriteRow(material writeMaterial) revisionWriteRow {
	prepared := material.prepared
	tentative := material.tentative
	payloadJSON := prepared.ProfilePayloadCanonicalJSON()
	receiptJSON := tentative.TentativeReceiptCanonicalJSON()
	admissionDigest := tentative.TentativeAdmissionRecordDigest()
	projectRoot := prepared.ProjectRoot()
	payloadDigest := prepared.ProfilePayloadDigest()
	receiptDigest := tentative.TentativeReceiptDigest()
	return revisionWriteRow{
		projectRoot:     projectRoot.String(),
		ledgerRevision:  material.committedRevision.Value(),
		payloadJSON:     string(payloadJSON),
		payloadDigest:   payloadDigest.String(),
		receiptJSON:     string(receiptJSON),
		receiptDigest:   receiptDigest.String(),
		admissionID:     material.admissionRef.String(),
		admissionDigest: admissionDigest.String(),
		recordedAt:      formatTime(material.recordedAt),
	}
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
