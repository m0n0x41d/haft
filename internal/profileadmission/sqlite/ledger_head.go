package sqlite

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const selectExactLedgerHeadSQL = `WITH target(project_root) AS (VALUES (?)),
all_revisions AS (
	SELECT revision.project_root, revision.ledger_revision
	FROM project_profile_revisions revision
	JOIN target ON target.project_root = revision.project_root
	UNION ALL
	SELECT revision.project_root, revision.ledger_revision
	FROM project_profile_revisions_v2 revision
	JOIN target ON target.project_root = revision.project_root
	UNION ALL
	SELECT revision.project_root, revision.ledger_revision
	FROM project_profile_revisions_v3 revision
	JOIN target ON target.project_root = revision.project_root
	UNION ALL
	SELECT revision.project_root, revision.ledger_revision
	FROM project_profile_revisions_v4 revision
	JOIN target ON target.project_root = revision.project_root
	UNION ALL
	SELECT revision.project_root, revision.ledger_revision
	FROM project_profile_revisions_v5 revision
	JOIN target ON target.project_root = revision.project_root
), all_admissions AS (
	SELECT admission.admission_id
	FROM project_profile_admissions admission
	JOIN target ON target.project_root = admission.project_root
	UNION ALL
	SELECT admission.admission_id
	FROM project_profile_admissions_v2 admission
	JOIN target ON target.project_root = admission.project_root
	UNION ALL
	SELECT admission.admission_id
	FROM project_profile_admissions_v3 admission
	JOIN target ON target.project_root = admission.project_root
	UNION ALL
	SELECT admission.admission_id
	FROM project_profile_admissions_v4 admission
	JOIN target ON target.project_root = admission.project_root
	UNION ALL
	SELECT admission.admission_id
	FROM project_profile_admissions_v5 admission
	JOIN target ON target.project_root = admission.project_root
), exact_uses AS (
	SELECT admission.admission_id
	FROM project_profile_revisions revision
	JOIN target ON target.project_root = revision.project_root
	JOIN project_profile_admissions admission
		ON admission.project_root = revision.project_root
		AND admission.ledger_revision = revision.ledger_revision
		AND admission.admission_id = revision.admission_id
		AND admission.admission_digest = revision.admission_digest
	JOIN authority_uses authority_use
		ON authority_use.committed_result_ref = admission.admission_id
		AND authority_use.committed_result_digest = admission.admission_digest
		AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
		AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
		AND authority_use.single_use_key = admission.single_use_key
		AND authority_use.admission_request_digest = admission.admission_request_digest
	UNION ALL
	SELECT admission.admission_id
	FROM project_profile_revisions_v2 revision
	JOIN target ON target.project_root = revision.project_root
	JOIN project_profile_admissions_v2 admission
		ON admission.project_root = revision.project_root
		AND admission.ledger_revision = revision.ledger_revision
		AND admission.admission_id = revision.admission_id
		AND admission.admission_digest = revision.admission_digest
	JOIN profile_declaration_authority_uses_v2 authority_use
		ON authority_use.committed_admission_ref = admission.admission_id
		AND authority_use.committed_admission_digest = admission.admission_digest
		AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
		AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
		AND authority_use.single_use_key = admission.single_use_key
		AND authority_use.admission_request_digest = admission.admission_request_digest
	UNION ALL
	SELECT admission.admission_id
	FROM project_profile_revisions_v3 revision
	JOIN target ON target.project_root = revision.project_root
	JOIN project_profile_admissions_v3 admission
		ON admission.project_root = revision.project_root
		AND admission.ledger_revision = revision.ledger_revision
		AND admission.admission_id = revision.admission_id
		AND admission.admission_digest = revision.admission_digest
	JOIN profile_declaration_authority_uses_v3 authority_use
		ON authority_use.committed_admission_ref = admission.admission_id
		AND authority_use.committed_admission_digest = admission.admission_digest
		AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
		AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
		AND authority_use.single_use_key = admission.single_use_key
		AND authority_use.admission_request_digest = admission.admission_request_digest
	UNION ALL
	SELECT admission.admission_id
	FROM project_profile_revisions_v4 revision
	JOIN target ON target.project_root = revision.project_root
	JOIN project_profile_admissions_v4 admission
		ON admission.project_root = revision.project_root
		AND admission.ledger_revision = revision.ledger_revision
		AND admission.admission_id = revision.admission_id
		AND admission.admission_digest = revision.admission_digest
	JOIN profile_declaration_authority_uses_v4 authority_use
		ON authority_use.committed_admission_ref = admission.admission_id
		AND authority_use.committed_admission_digest = admission.admission_digest
		AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
		AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
		AND authority_use.single_use_key = admission.single_use_key
		AND authority_use.admission_request_digest = admission.admission_request_digest
	UNION ALL
	SELECT admission.admission_id
	FROM project_profile_revisions_v5 revision
	JOIN target ON target.project_root = revision.project_root
	JOIN project_profile_admissions_v5 admission
		ON admission.project_root = revision.project_root
		AND admission.ledger_revision = revision.ledger_revision
		AND admission.admission_id = revision.admission_id
		AND admission.admission_digest = revision.admission_digest
	JOIN profile_declaration_authority_uses_v5 authority_use
		ON authority_use.committed_admission_ref = admission.admission_id
		AND authority_use.committed_admission_digest = admission.admission_digest
		AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
		AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
		AND authority_use.single_use_key = admission.single_use_key
		AND authority_use.admission_request_digest = admission.admission_request_digest
)
SELECT COUNT(*),
	COALESCE(MIN(ledger_revision), 0),
	COALESCE(MAX(ledger_revision), 0),
	(SELECT COUNT(*) FROM all_admissions),
	(SELECT COUNT(*) FROM exact_uses)
FROM all_revisions`

const selectExactHistoricalAdmissionSQL = `WITH target(project_root, ledger_revision) AS (VALUES (?, ?))
SELECT admission.admission_json,
	admission.admission_digest,
	admission.receipt_json,
	admission.receipt_digest,
	admission.profile_payload_json,
	admission.profile_payload_digest,
	admission.candidate_provenance_json,
	admission.candidate_provenance_digest,
	revision.ledger_revision
FROM project_profile_revisions revision
JOIN target ON target.project_root = revision.project_root
	AND target.ledger_revision = revision.ledger_revision
JOIN project_profile_admissions admission
	ON admission.project_root = revision.project_root
	AND admission.ledger_revision = revision.ledger_revision
	AND admission.expected_ledger_revision = revision.ledger_revision - 1
	AND admission.admission_id = revision.admission_id
	AND admission.admission_digest = revision.admission_digest
	AND admission.profile_payload_json = revision.profile_payload_json
	AND admission.profile_payload_digest = revision.profile_payload_digest
	AND admission.receipt_json = revision.receipt_json
	AND admission.receipt_digest = revision.receipt_digest
	AND admission.recorded_at = revision.recorded_at
JOIN authority_uses authority_use
	ON authority_use.committed_result_ref = admission.admission_id
	AND authority_use.committed_result_digest = admission.admission_digest
	AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
	AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
	AND authority_use.single_use_key = admission.single_use_key
	AND authority_use.action_kind = admission.action_kind
	AND authority_use.project_root = admission.project_root
	AND authority_use.project_binding_digest = admission.project_binding_digest
	AND authority_use.authority_record_ref = admission.authority_basis_ref
	AND authority_use.authority_record_digest = admission.authority_basis_digest
	AND authority_use.admission_request_digest = admission.admission_request_digest
	AND authority_use.consumed_at = admission.recorded_at
WHERE revision.configured_profile_kind = 'Declared'
	AND admission.action_kind = 'profile.declare.from_onboarding_candidate'
UNION ALL
SELECT admission.admission_json,
	admission.admission_digest,
	admission.receipt_json,
	admission.receipt_digest,
	admission.profile_payload_json,
	admission.profile_payload_digest,
	admission.candidate_provenance_json,
	admission.candidate_provenance_digest,
	revision.ledger_revision
FROM project_profile_revisions_v2 revision
JOIN target ON target.project_root = revision.project_root
	AND target.ledger_revision = revision.ledger_revision
JOIN project_profile_admissions_v2 admission
	ON admission.project_root = revision.project_root
	AND admission.ledger_revision = revision.ledger_revision
	AND admission.expected_ledger_revision = revision.ledger_revision - 1
	AND admission.admission_id = revision.admission_id
	AND admission.admission_digest = revision.admission_digest
	AND admission.profile_payload_json = revision.profile_payload_json
	AND admission.profile_payload_digest = revision.profile_payload_digest
	AND admission.receipt_json = revision.receipt_json
	AND admission.receipt_digest = revision.receipt_digest
	AND admission.recorded_at = revision.recorded_at
JOIN profile_declaration_authority_uses_v2 authority_use
	ON authority_use.committed_admission_ref = admission.admission_id
	AND authority_use.committed_admission_digest = admission.admission_digest
	AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
	AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
	AND authority_use.single_use_key = admission.single_use_key
	AND authority_use.action_kind = admission.action_kind
	AND authority_use.project_root = admission.project_root
	AND authority_use.project_binding_digest = admission.project_binding_digest
	AND authority_use.authority_basis_ref = admission.authority_basis_ref
	AND authority_use.authority_basis_digest = admission.authority_basis_digest
	AND authority_use.admission_request_digest = admission.admission_request_digest
	AND authority_use.consumed_at = admission.recorded_at
WHERE revision.configured_profile_kind = 'Declared'
	AND admission.action_kind = 'profile.declare.from_onboarding_candidate'
UNION ALL
SELECT admission.admission_json,
	admission.admission_digest,
	admission.receipt_json,
	admission.receipt_digest,
	admission.profile_payload_json,
	admission.profile_payload_digest,
	admission.candidate_provenance_json,
	admission.candidate_provenance_digest,
	revision.ledger_revision
FROM project_profile_revisions_v3 revision
JOIN target ON target.project_root = revision.project_root
	AND target.ledger_revision = revision.ledger_revision
JOIN project_profile_admissions_v3 admission
	ON admission.project_root = revision.project_root
	AND admission.ledger_revision = revision.ledger_revision
	AND admission.expected_ledger_revision = revision.ledger_revision - 1
	AND admission.admission_id = revision.admission_id
	AND admission.admission_digest = revision.admission_digest
	AND admission.profile_payload_json = revision.profile_payload_json
	AND admission.profile_payload_digest = revision.profile_payload_digest
	AND admission.receipt_json = revision.receipt_json
	AND admission.receipt_digest = revision.receipt_digest
	AND admission.recorded_at = revision.recorded_at
JOIN profile_declaration_authority_uses_v3 authority_use
	ON authority_use.committed_admission_ref = admission.admission_id
	AND authority_use.committed_admission_digest = admission.admission_digest
	AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
	AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
	AND authority_use.single_use_key = admission.single_use_key
	AND authority_use.action_kind = admission.action_kind
	AND authority_use.project_root = admission.project_root
	AND authority_use.project_binding_digest = admission.project_binding_digest
	AND authority_use.authority_basis_ref = admission.authority_basis_ref
	AND authority_use.authority_basis_digest = admission.authority_basis_digest
	AND authority_use.work_input_ref = admission.work_input_ref
	AND authority_use.work_input_digest = admission.work_input_digest
	AND authority_use.admission_request_digest = admission.admission_request_digest
	AND authority_use.consumed_at = admission.recorded_at
WHERE revision.configured_profile_kind = 'Declared'
	AND admission.action_kind = 'profile.declare.from_onboarding_candidate'
UNION ALL
SELECT admission.admission_json,
	admission.admission_digest,
	admission.receipt_json,
	admission.receipt_digest,
	admission.profile_payload_json,
	admission.profile_payload_digest,
	admission.candidate_provenance_json,
	admission.candidate_provenance_digest,
	revision.ledger_revision
FROM project_profile_revisions_v4 revision
JOIN target ON target.project_root = revision.project_root
	AND target.ledger_revision = revision.ledger_revision
JOIN project_profile_admissions_v4 admission
	ON admission.project_root = revision.project_root
	AND admission.ledger_revision = revision.ledger_revision
	AND admission.expected_ledger_revision = revision.ledger_revision - 1
	AND admission.admission_id = revision.admission_id
	AND admission.admission_digest = revision.admission_digest
	AND admission.profile_payload_json = revision.profile_payload_json
	AND admission.profile_payload_digest = revision.profile_payload_digest
	AND admission.receipt_json = revision.receipt_json
	AND admission.receipt_digest = revision.receipt_digest
	AND admission.recorded_at = revision.recorded_at
JOIN profile_declaration_authority_uses_v4 authority_use
	ON authority_use.committed_admission_ref = admission.admission_id
	AND authority_use.committed_admission_digest = admission.admission_digest
	AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
	AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
	AND authority_use.single_use_key = admission.single_use_key
	AND authority_use.action_kind = admission.action_kind
	AND authority_use.project_root = admission.project_root
	AND authority_use.project_binding_digest = admission.project_binding_digest
	AND authority_use.authority_basis_ref = admission.authority_basis_ref
	AND authority_use.authority_basis_digest = admission.authority_basis_digest
	AND authority_use.work_input_ref = admission.work_input_ref
	AND authority_use.work_input_digest = admission.work_input_digest
	AND authority_use.admission_request_digest = admission.admission_request_digest
	AND authority_use.consumed_at = admission.recorded_at
WHERE revision.configured_profile_kind = 'Declared'
	AND admission.action_kind = 'profile.apply_supported_singleton_default'
UNION ALL
SELECT admission.admission_json,
	admission.admission_digest,
	admission.receipt_json,
	admission.receipt_digest,
	admission.profile_payload_json,
	admission.profile_payload_digest,
	admission.candidate_provenance_json,
	admission.candidate_provenance_digest,
	revision.ledger_revision
FROM project_profile_revisions_v5 revision
JOIN target ON target.project_root = revision.project_root
	AND target.ledger_revision = revision.ledger_revision
JOIN project_profile_admissions_v5 admission
	ON admission.project_root = revision.project_root
	AND admission.ledger_revision = revision.ledger_revision
	AND admission.expected_ledger_revision = revision.ledger_revision - 1
	AND admission.admission_id = revision.admission_id
	AND admission.admission_digest = revision.admission_digest
	AND admission.profile_payload_json = revision.profile_payload_json
	AND admission.profile_payload_digest = revision.profile_payload_digest
	AND admission.receipt_json = revision.receipt_json
	AND admission.receipt_digest = revision.receipt_digest
	AND admission.recorded_at = revision.recorded_at
JOIN profile_declaration_authority_uses_v5 authority_use
	ON authority_use.committed_admission_ref = admission.admission_id
	AND authority_use.committed_admission_digest = admission.admission_digest
	AND authority_use.authority_resolution_ref = admission.authority_resolution_ref
	AND authority_use.authority_resolution_digest = admission.authority_resolution_digest
	AND authority_use.single_use_key = admission.single_use_key
	AND authority_use.action_kind = admission.action_kind
	AND authority_use.project_root = admission.project_root
	AND authority_use.project_binding_digest = admission.project_binding_digest
	AND authority_use.authority_basis_ref = admission.authority_basis_ref
	AND authority_use.authority_basis_digest = admission.authority_basis_digest
	AND authority_use.work_input_ref = admission.work_input_ref
	AND authority_use.work_input_digest = admission.work_input_digest
	AND authority_use.admission_request_digest = admission.admission_request_digest
	AND authority_use.consumed_at = admission.recorded_at
WHERE revision.configured_profile_kind = 'Declared'
	AND admission.action_kind = 'profile.declare.from_onboarding_candidate'`

const exactHistoryValidationLimit int64 = 4096

type ledgerHeadEvidence struct {
	revisionCount  int64
	minimum        int64
	maximum        int64
	admissionCount int64
	exactUseCount  int64
}

type historicalAdmissionRow struct {
	admissionJSON    string
	admissionDigest  string
	receiptJSON      string
	receiptDigest    string
	payloadJSON      string
	payloadDigest    string
	provenanceJSON   string
	provenanceDigest string
	ledgerRevision   int64
}

func loadExactLedgerHead(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectRoot projectprofile.ProjectRootV1,
) (projectprofile.LedgerRevision, error) {
	evidence := ledgerHeadEvidence{}
	root := projectRoot.String()
	arguments := []any{root}
	destinations := []any{
		&evidence.revisionCount,
		&evidence.minimum,
		&evidence.maximum,
		&evidence.admissionCount,
		&evidence.exactUseCount,
	}
	err := transaction.ScanOne(ctx, selectExactLedgerHeadSQL, arguments, destinations)
	if err != nil {
		return projectprofile.LedgerRevision{}, fmt.Errorf("read exact profile ledger head: %w", err)
	}
	empty, err := validateLedgerHeadEvidence(evidence)
	if err != nil {
		return projectprofile.LedgerRevision{}, err
	}
	if empty {
		return projectprofile.NewLedgerRevision(0), nil
	}
	err = validateExactHistory(
		ctx,
		transaction,
		projectRoot,
		evidence.maximum,
		1,
	)
	if err != nil {
		return projectprofile.LedgerRevision{}, err
	}
	head, err := parseLedgerRevision(evidence.maximum, false)
	if err != nil {
		return projectprofile.LedgerRevision{}, fmt.Errorf("parse exact profile ledger head: %w", err)
	}
	_, err = head.Next()
	if err != nil {
		return projectprofile.LedgerRevision{}, fmt.Errorf("profile ledger head cannot advance: %w", err)
	}
	return head, nil
}

func validateLedgerHeadEvidence(evidence ledgerHeadEvidence) (bool, error) {
	if exactEmptyLedger(evidence) {
		return true, nil
	}
	if !exactPopulatedLedger(evidence) {
		return false, fmt.Errorf("profile ledger history is not an exact contiguous admission-authority chain")
	}
	if evidence.maximum > exactHistoryValidationLimit {
		return false, fmt.Errorf("profile ledger history exceeds the bounded exact-validation limit")
	}
	return false, nil
}

func validateExactHistory(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectRoot projectprofile.ProjectRootV1,
	maximum int64,
	revision int64,
) error {
	if revision > maximum {
		return nil
	}
	row, err := loadHistoricalAdmission(ctx, transaction, projectRoot, revision)
	if err != nil {
		return err
	}
	err = validateHistoricalAdmission(row)
	if err != nil {
		return fmt.Errorf("validate profile ledger revision %d: %w", revision, err)
	}
	return validateExactHistory(ctx, transaction, projectRoot, maximum, revision+1)
}

func loadHistoricalAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	projectRoot projectprofile.ProjectRootV1,
	revision int64,
) (historicalAdmissionRow, error) {
	row := historicalAdmissionRow{}
	arguments := []any{projectRoot.String(), revision}
	destinations := []any{
		&row.admissionJSON,
		&row.admissionDigest,
		&row.receiptJSON,
		&row.receiptDigest,
		&row.payloadJSON,
		&row.payloadDigest,
		&row.provenanceJSON,
		&row.provenanceDigest,
		&row.ledgerRevision,
	}
	err := transaction.ScanOne(
		ctx,
		selectExactHistoricalAdmissionSQL,
		arguments,
		destinations,
	)
	if err != nil {
		return historicalAdmissionRow{}, fmt.Errorf("read exact profile ledger revision %d: %w", revision, err)
	}
	return row, nil
}

func validateHistoricalAdmission(row historicalAdmissionRow) error {
	admissionDigest, err := projectprofile.NewContentDigest(row.admissionDigest)
	if err != nil {
		return fmt.Errorf("parse historical admission digest: %w", err)
	}
	receiptDigest, err := projectprofile.NewContentDigest(row.receiptDigest)
	if err != nil {
		return fmt.Errorf("parse historical receipt digest: %w", err)
	}
	payloadDigest, err := projectprofile.NewContentDigest(row.payloadDigest)
	if err != nil {
		return fmt.Errorf("parse historical payload digest: %w", err)
	}
	provenanceDigest, err := projectprofile.NewContentDigest(row.provenanceDigest)
	if err != nil {
		return fmt.Errorf("parse historical provenance digest: %w", err)
	}
	revision, err := parseLedgerRevision(row.ledgerRevision, false)
	if err != nil {
		return err
	}
	builder := projectprofile.NewDurableProfileAdmissionTupleV1Builder(
		[]byte(row.admissionJSON),
		admissionDigest,
	)
	builder = builder.WithReceipt([]byte(row.receiptJSON), receiptDigest)
	builder = builder.WithPayload([]byte(row.payloadJSON), payloadDigest)
	builder = builder.AtLedgerRevision(revision)
	durable, err := builder.Build()
	if err != nil {
		return fmt.Errorf("build historical durable tuple: %w", err)
	}
	err = projectprofile.ValidateDurableProfileAdmissionRecordV1(
		durable,
		[]byte(row.provenanceJSON),
		provenanceDigest,
	)
	if err != nil {
		return err
	}
	return nil
}

func exactEmptyLedger(evidence ledgerHeadEvidence) bool {
	return evidence.revisionCount == 0 &&
		evidence.minimum == 0 &&
		evidence.maximum == 0 &&
		evidence.admissionCount == 0 &&
		evidence.exactUseCount == 0
}

func exactPopulatedLedger(evidence ledgerHeadEvidence) bool {
	positive := evidence.revisionCount > 0 && evidence.minimum == 1
	contiguous := evidence.maximum == evidence.revisionCount
	exactAdmissions := evidence.admissionCount == evidence.revisionCount
	exactAuthorityUses := evidence.exactUseCount == evidence.revisionCount
	return positive && contiguous && exactAdmissions && exactAuthorityUses
}
