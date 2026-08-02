package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const selectDurableAdmissionSQL = `WITH all_durable AS (
	SELECT 'v1' AS storage_generation,
		a.admission_id, a.action_kind, a.project_root, a.project_binding_digest,
		a.profile_payload_json, a.profile_payload_digest,
		a.candidate_provenance_json, a.candidate_provenance_digest,
		a.profile_author_role_assignment_ref, a.profile_author_role_assignment_digest,
		a.observed_project_basis_ref, a.observed_project_basis_digest,
		a.work_record_ref, a.work_record_digest,
		a.outcome_assessment_ref, a.outcome_assessment_digest,
		a.authority_basis_ref, a.authority_basis_digest,
		a.authority_resolution_ref, a.authority_resolution_digest,
		a.receipt_json, a.receipt_digest,
		a.expected_ledger_revision, a.ledger_revision,
		a.single_use_key, a.admission_request_digest,
		a.admission_json, a.admission_digest, a.recorded_at,
		u.use_id, '' AS use_digest, u.authority_resolution_ref AS use_resolution_ref,
		u.authority_resolution_digest AS use_resolution_digest,
		u.single_use_key AS use_single_use_key, u.action_kind AS use_action_kind,
		u.project_root AS use_project_root,
		u.project_binding_digest AS use_project_binding_digest,
		u.envelope_digest, u.authority_record_ref, u.authority_record_digest,
		u.admission_request_digest AS use_request_digest,
		u.verifier_identity, u.verifier_version,
		u.committed_result_ref, u.committed_result_digest, u.consumed_at,
		r.project_root AS revision_project_root,
		r.ledger_revision AS revision_ledger_revision,
		r.configured_profile_kind, r.profile_payload_json AS revision_payload_json,
		r.profile_payload_digest AS revision_payload_digest,
		r.receipt_json AS revision_receipt_json,
		r.receipt_digest AS revision_receipt_digest,
		r.admission_id AS revision_admission_id,
		r.admission_digest AS revision_admission_digest,
		r.recorded_at AS revision_recorded_at
	FROM project_profile_admissions a
	JOIN authority_uses u ON u.committed_result_ref = a.admission_id
	JOIN project_profile_revisions r
		ON r.admission_id = a.admission_id
		AND r.project_root = a.project_root
		AND r.ledger_revision = a.ledger_revision
	UNION ALL
	SELECT 'v2' AS storage_generation,
		a.admission_id, a.action_kind, a.project_root, a.project_binding_digest,
		a.profile_payload_json, a.profile_payload_digest,
		a.candidate_provenance_json, a.candidate_provenance_digest,
		a.profile_author_role_assignment_ref, a.profile_author_role_assignment_digest,
		a.observed_project_basis_ref, a.observed_project_basis_digest,
		a.work_record_ref, a.work_record_digest,
		a.outcome_assessment_ref, a.outcome_assessment_digest,
		a.authority_basis_ref, a.authority_basis_digest,
		a.authority_resolution_ref, a.authority_resolution_digest,
		a.receipt_json, a.receipt_digest,
		a.expected_ledger_revision, a.ledger_revision,
		a.single_use_key, a.admission_request_digest,
		a.admission_json, a.admission_digest, a.recorded_at,
		u.use_ref AS use_id, u.use_digest,
		u.authority_resolution_ref AS use_resolution_ref,
		u.authority_resolution_digest AS use_resolution_digest,
		u.single_use_key AS use_single_use_key, u.action_kind AS use_action_kind,
		u.project_root AS use_project_root,
		u.project_binding_digest AS use_project_binding_digest,
		resolution.action_envelope_digest AS envelope_digest,
		u.authority_basis_ref AS authority_record_ref,
		u.authority_basis_digest AS authority_record_digest,
		u.admission_request_digest AS use_request_digest,
		resolution.verifier_identity, resolution.verifier_version,
		u.committed_admission_ref AS committed_result_ref,
		u.committed_admission_digest AS committed_result_digest,
		u.consumed_at,
		r.project_root AS revision_project_root,
		r.ledger_revision AS revision_ledger_revision,
		r.configured_profile_kind, r.profile_payload_json AS revision_payload_json,
		r.profile_payload_digest AS revision_payload_digest,
		r.receipt_json AS revision_receipt_json,
		r.receipt_digest AS revision_receipt_digest,
		r.admission_id AS revision_admission_id,
		r.admission_digest AS revision_admission_digest,
		r.recorded_at AS revision_recorded_at
	FROM project_profile_admissions_v2 a
	JOIN profile_declaration_authority_uses_v2 u
		ON u.committed_admission_ref = a.admission_id
	JOIN profile_declaration_authority_resolutions_v2 resolution
		ON resolution.authority_resolution_ref = u.authority_resolution_ref
	JOIN project_profile_revisions_v2 r
		ON r.admission_id = a.admission_id
		AND r.project_root = a.project_root
		AND r.ledger_revision = a.ledger_revision
	UNION ALL
	SELECT 'v3' AS storage_generation,
		a.admission_id, a.action_kind, a.project_root, a.project_binding_digest,
		a.profile_payload_json, a.profile_payload_digest,
		a.candidate_provenance_json, a.candidate_provenance_digest,
		a.profile_author_role_assignment_ref, a.profile_author_role_assignment_digest,
		a.observed_project_basis_ref, a.observed_project_basis_digest,
		a.work_record_ref, a.work_record_digest,
		a.outcome_assessment_ref, a.outcome_assessment_digest,
		a.authority_basis_ref, a.authority_basis_digest,
		a.authority_resolution_ref, a.authority_resolution_digest,
		a.receipt_json, a.receipt_digest,
		a.expected_ledger_revision, a.ledger_revision,
		a.single_use_key, a.admission_request_digest,
		a.admission_json, a.admission_digest, a.recorded_at,
		u.use_ref AS use_id, u.use_digest,
		u.authority_resolution_ref AS use_resolution_ref,
		u.authority_resolution_digest AS use_resolution_digest,
		u.single_use_key AS use_single_use_key, u.action_kind AS use_action_kind,
		u.project_root AS use_project_root,
		u.project_binding_digest AS use_project_binding_digest,
		'' AS envelope_digest,
		u.authority_basis_ref AS authority_record_ref,
		u.authority_basis_digest AS authority_record_digest,
		u.admission_request_digest AS use_request_digest,
		resolution.verifier_identity, resolution.verifier_version,
		u.committed_admission_ref AS committed_result_ref,
		u.committed_admission_digest AS committed_result_digest,
		u.consumed_at,
		r.project_root AS revision_project_root,
		r.ledger_revision AS revision_ledger_revision,
		r.configured_profile_kind, r.profile_payload_json AS revision_payload_json,
		r.profile_payload_digest AS revision_payload_digest,
		r.receipt_json AS revision_receipt_json,
		r.receipt_digest AS revision_receipt_digest,
		r.admission_id AS revision_admission_id,
		r.admission_digest AS revision_admission_digest,
		r.recorded_at AS revision_recorded_at
	FROM project_profile_admissions_v3 a
	JOIN profile_declaration_authority_uses_v3 u
		ON u.committed_admission_ref = a.admission_id
	JOIN profile_declaration_authority_resolutions_v3 resolution
		ON resolution.authority_resolution_ref = u.authority_resolution_ref
	JOIN project_profile_revisions_v3 r
		ON r.admission_id = a.admission_id
		AND r.project_root = a.project_root
		AND r.ledger_revision = a.ledger_revision
	UNION ALL
	SELECT 'v4' AS storage_generation,
		a.admission_id, a.action_kind, a.project_root, a.project_binding_digest,
		a.profile_payload_json, a.profile_payload_digest,
		a.candidate_provenance_json, a.candidate_provenance_digest,
		a.profile_author_role_assignment_ref, a.profile_author_role_assignment_digest,
		a.observed_project_basis_ref, a.observed_project_basis_digest,
		a.work_record_ref, a.work_record_digest,
		a.outcome_assessment_ref, a.outcome_assessment_digest,
		a.authority_basis_ref, a.authority_basis_digest,
		a.authority_resolution_ref, a.authority_resolution_digest,
		a.receipt_json, a.receipt_digest,
		a.expected_ledger_revision, a.ledger_revision,
		a.single_use_key, a.admission_request_digest,
		a.admission_json, a.admission_digest, a.recorded_at,
		u.use_ref AS use_id, u.use_digest,
		u.authority_resolution_ref AS use_resolution_ref,
		u.authority_resolution_digest AS use_resolution_digest,
		u.single_use_key AS use_single_use_key, u.action_kind AS use_action_kind,
		u.project_root AS use_project_root,
		u.project_binding_digest AS use_project_binding_digest,
		'' AS envelope_digest,
		u.authority_basis_ref AS authority_record_ref,
		u.authority_basis_digest AS authority_record_digest,
		u.admission_request_digest AS use_request_digest,
		resolution.verifier_identity, resolution.verifier_version,
		u.committed_admission_ref AS committed_result_ref,
		u.committed_admission_digest AS committed_result_digest,
		u.consumed_at,
		r.project_root AS revision_project_root,
		r.ledger_revision AS revision_ledger_revision,
		r.configured_profile_kind, r.profile_payload_json AS revision_payload_json,
		r.profile_payload_digest AS revision_payload_digest,
		r.receipt_json AS revision_receipt_json,
		r.receipt_digest AS revision_receipt_digest,
		r.admission_id AS revision_admission_id,
		r.admission_digest AS revision_admission_digest,
		r.recorded_at AS revision_recorded_at
	FROM project_profile_admissions_v4 a
	JOIN profile_declaration_authority_uses_v4 u
		ON u.committed_admission_ref = a.admission_id
	JOIN profile_initial_bootstrap_authority_resolutions_v1 resolution
		ON resolution.authority_resolution_ref = u.authority_resolution_ref
	JOIN project_profile_revisions_v4 r
		ON r.admission_id = a.admission_id
		AND r.project_root = a.project_root
		AND r.ledger_revision = a.ledger_revision
	UNION ALL
	SELECT 'v5' AS storage_generation,
		a.admission_id, a.action_kind, a.project_root, a.project_binding_digest,
		a.profile_payload_json, a.profile_payload_digest,
		a.candidate_provenance_json, a.candidate_provenance_digest,
		a.profile_author_role_assignment_ref, a.profile_author_role_assignment_digest,
		a.observed_project_basis_ref, a.observed_project_basis_digest,
		a.work_record_ref, a.work_record_digest,
		a.outcome_assessment_ref, a.outcome_assessment_digest,
		a.authority_basis_ref, a.authority_basis_digest,
		a.authority_resolution_ref, a.authority_resolution_digest,
		a.receipt_json, a.receipt_digest,
		a.expected_ledger_revision, a.ledger_revision,
		a.single_use_key, a.admission_request_digest,
		a.admission_json, a.admission_digest, a.recorded_at,
		u.use_ref AS use_id, u.use_digest,
		u.authority_resolution_ref AS use_resolution_ref,
		u.authority_resolution_digest AS use_resolution_digest,
		u.single_use_key AS use_single_use_key, u.action_kind AS use_action_kind,
		u.project_root AS use_project_root,
		u.project_binding_digest AS use_project_binding_digest,
		'' AS envelope_digest,
		u.authority_basis_ref AS authority_record_ref,
		u.authority_basis_digest AS authority_record_digest,
		u.admission_request_digest AS use_request_digest,
		resolution.verifier_identity, resolution.verifier_version,
		u.committed_admission_ref AS committed_result_ref,
		u.committed_admission_digest AS committed_result_digest,
		u.consumed_at,
		r.project_root AS revision_project_root,
		r.ledger_revision AS revision_ledger_revision,
		r.configured_profile_kind, r.profile_payload_json AS revision_payload_json,
		r.profile_payload_digest AS revision_payload_digest,
		r.receipt_json AS revision_receipt_json,
		r.receipt_digest AS revision_receipt_digest,
		r.admission_id AS revision_admission_id,
		r.admission_digest AS revision_admission_digest,
		r.recorded_at AS revision_recorded_at
	FROM project_profile_admissions_v5 a
	JOIN profile_declaration_authority_uses_v5 u
		ON u.committed_admission_ref = a.admission_id
	JOIN profile_declaration_authority_resolutions_v5 resolution
		ON resolution.authority_resolution_ref = u.authority_resolution_ref
	JOIN project_profile_revisions_v5 r
		ON r.admission_id = a.admission_id
		AND r.project_root = a.project_root
		AND r.ledger_revision = a.ledger_revision
)
SELECT * FROM all_durable WHERE admission_id = ?`

type durableAdmissionRow struct {
	storageGeneration          string
	admissionID                string
	actionKind                 string
	projectRoot                string
	projectBindingDigest       string
	payloadJSON                string
	payloadDigest              string
	candidateProvenanceJSON    string
	candidateProvenanceDigest  string
	roleAssignmentRef          string
	roleAssignmentDigest       string
	observedBasisRef           string
	observedBasisDigest        string
	workRecordRef              string
	workRecordDigest           string
	outcomeAssessmentRef       string
	outcomeAssessmentDigest    string
	authorityBasisRef          string
	authorityBasisDigest       string
	authorityResolutionRef     string
	authorityResolutionDigest  string
	receiptJSON                string
	receiptDigest              string
	expectedLedgerRevision     int64
	ledgerRevision             int64
	singleUseKey               string
	admissionRequestDigest     string
	admissionJSON              string
	admissionDigest            string
	recordedAt                 string
	useID                      string
	useDigest                  string
	useAuthorityResolutionRef  string
	useAuthorityResolutionHash string
	useSingleUseKey            string
	useActionKind              string
	useProjectRoot             string
	useProjectBindingDigest    string
	useEnvelopeDigest          string
	useAuthorityRecordRef      string
	useAuthorityRecordDigest   string
	useAdmissionRequestDigest  string
	useVerifierIdentity        string
	useVerifierVersion         string
	useCommittedResultRef      string
	useCommittedResultDigest   string
	consumedAt                 string
	revisionProjectRoot        string
	revisionLedgerRevision     int64
	revisionConfiguredKind     string
	revisionPayloadJSON        string
	revisionPayloadDigest      string
	revisionReceiptJSON        string
	revisionReceiptDigest      string
	revisionAdmissionID        string
	revisionAdmissionDigest    string
	revisionRecordedAt         string
}

func (row *durableAdmissionRow) scanTargets() []any {
	return []any{
		&row.storageGeneration,
		&row.admissionID,
		&row.actionKind,
		&row.projectRoot,
		&row.projectBindingDigest,
		&row.payloadJSON,
		&row.payloadDigest,
		&row.candidateProvenanceJSON,
		&row.candidateProvenanceDigest,
		&row.roleAssignmentRef,
		&row.roleAssignmentDigest,
		&row.observedBasisRef,
		&row.observedBasisDigest,
		&row.workRecordRef,
		&row.workRecordDigest,
		&row.outcomeAssessmentRef,
		&row.outcomeAssessmentDigest,
		&row.authorityBasisRef,
		&row.authorityBasisDigest,
		&row.authorityResolutionRef,
		&row.authorityResolutionDigest,
		&row.receiptJSON,
		&row.receiptDigest,
		&row.expectedLedgerRevision,
		&row.ledgerRevision,
		&row.singleUseKey,
		&row.admissionRequestDigest,
		&row.admissionJSON,
		&row.admissionDigest,
		&row.recordedAt,
		&row.useID,
		&row.useDigest,
		&row.useAuthorityResolutionRef,
		&row.useAuthorityResolutionHash,
		&row.useSingleUseKey,
		&row.useActionKind,
		&row.useProjectRoot,
		&row.useProjectBindingDigest,
		&row.useEnvelopeDigest,
		&row.useAuthorityRecordRef,
		&row.useAuthorityRecordDigest,
		&row.useAdmissionRequestDigest,
		&row.useVerifierIdentity,
		&row.useVerifierVersion,
		&row.useCommittedResultRef,
		&row.useCommittedResultDigest,
		&row.consumedAt,
		&row.revisionProjectRoot,
		&row.revisionLedgerRevision,
		&row.revisionConfiguredKind,
		&row.revisionPayloadJSON,
		&row.revisionPayloadDigest,
		&row.revisionReceiptJSON,
		&row.revisionReceiptDigest,
		&row.revisionAdmissionID,
		&row.revisionAdmissionDigest,
		&row.revisionRecordedAt,
	}
}

func loadDurableAdmissionRow(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admissionRef string,
) (durableAdmissionRow, error) {
	row := durableAdmissionRow{}
	arguments := []any{admissionRef}
	destinations := row.scanTargets()
	err := transaction.ScanOne(ctx, selectDurableAdmissionSQL, arguments, destinations)
	if errors.Is(err, sql.ErrNoRows) {
		return durableAdmissionRow{}, sql.ErrNoRows
	}
	if err != nil {
		return durableAdmissionRow{}, fmt.Errorf("load durable profile admission: %w", err)
	}
	return row, nil
}

func firstMismatch(
	checks []struct {
		matches bool
		name    string
	},
	object string,
) error {
	return mismatchAt(checks, object, 0)
}

func mismatchAt(
	checks []struct {
		matches bool
		name    string
	},
	object string,
	index int,
) error {
	if index >= len(checks) {
		return nil
	}
	check := checks[index]
	if !check.matches {
		return fmt.Errorf("%s has a mismatched %s", object, check.name)
	}
	return mismatchAt(checks, object, index+1)
}

func parseLedgerRevision(
	value int64,
	allowZero bool,
) (projectprofile.LedgerRevision, error) {
	if value < 0 {
		return projectprofile.LedgerRevision{}, fmt.Errorf("durable ledger revision is negative")
	}
	if value == 0 && !allowZero {
		return projectprofile.LedgerRevision{}, fmt.Errorf("durable committed ledger revision is zero")
	}
	revision := projectprofile.NewLedgerRevision(uint64(value))
	_, err := revision.Next()
	if err != nil {
		return projectprofile.LedgerRevision{}, fmt.Errorf("durable ledger revision is not incrementable: %w", err)
	}
	return revision, nil
}

func parseCanonicalTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse durable admission time: %w", err)
	}
	canonical := formatTime(parsed)
	if canonical != value {
		return time.Time{}, fmt.Errorf("durable admission time is not canonical UTC")
	}
	return parsed.UTC(), nil
}
