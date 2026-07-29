package profileprojection

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const (
	selectCanonicalLedgerHeadSQL = `
SELECT storage_generation, admission_id, admission_digest,
       profile_payload_digest, ledger_revision
FROM current_project_profiles
WHERE project_root = ?`

	countCanonicalLedgerHeadSQL = `
SELECT COUNT(*)
FROM current_project_profiles
WHERE project_root = ?`

	selectExactAdmissionSourceSQL = `
WITH expected (
    project_root, ledger_revision, admission_id, admission_digest,
    profile_payload_digest, receipt_digest,
    authority_resolution_ref, authority_resolution_digest, recorded_at
) AS (VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)),
exact_sources AS (
    SELECT 'v1' AS storage_generation,
           admission.admission_id, admission.admission_digest,
           admission.receipt_digest, admission.authority_resolution_ref,
           admission.single_use_key, admission.project_root,
           admission.ledger_revision
    FROM expected
    JOIN project_profile_admissions admission
      ON admission.project_root = expected.project_root
     AND admission.ledger_revision = expected.ledger_revision
     AND admission.admission_id = expected.admission_id
     AND admission.admission_digest = expected.admission_digest
     AND admission.profile_payload_digest = expected.profile_payload_digest
     AND admission.receipt_digest = expected.receipt_digest
     AND admission.authority_resolution_ref = expected.authority_resolution_ref
     AND admission.authority_resolution_digest = expected.authority_resolution_digest
     AND admission.recorded_at = expected.recorded_at
    JOIN project_profile_revisions revision
      ON revision.project_root = admission.project_root
     AND revision.ledger_revision = admission.ledger_revision
     AND revision.admission_id = admission.admission_id
     AND revision.admission_digest = admission.admission_digest
     AND revision.profile_payload_digest = admission.profile_payload_digest
     AND revision.receipt_digest = admission.receipt_digest
     AND revision.recorded_at = admission.recorded_at
    UNION ALL
    SELECT 'v2' AS storage_generation,
           admission.admission_id, admission.admission_digest,
           admission.receipt_digest, admission.authority_resolution_ref,
           admission.single_use_key, admission.project_root,
           admission.ledger_revision
    FROM expected
    JOIN project_profile_admissions_v2 admission
      ON admission.project_root = expected.project_root
     AND admission.ledger_revision = expected.ledger_revision
     AND admission.admission_id = expected.admission_id
     AND admission.admission_digest = expected.admission_digest
     AND admission.profile_payload_digest = expected.profile_payload_digest
     AND admission.receipt_digest = expected.receipt_digest
     AND admission.authority_resolution_ref = expected.authority_resolution_ref
     AND admission.authority_resolution_digest = expected.authority_resolution_digest
     AND admission.recorded_at = expected.recorded_at
    JOIN project_profile_revisions_v2 revision
      ON revision.project_root = admission.project_root
     AND revision.ledger_revision = admission.ledger_revision
     AND revision.admission_id = admission.admission_id
     AND revision.admission_digest = admission.admission_digest
     AND revision.profile_payload_digest = admission.profile_payload_digest
     AND revision.receipt_digest = admission.receipt_digest
     AND revision.recorded_at = admission.recorded_at
    UNION ALL
    SELECT 'v3' AS storage_generation,
           admission.admission_id, admission.admission_digest,
           admission.receipt_digest, admission.authority_resolution_ref,
           admission.single_use_key, admission.project_root,
           admission.ledger_revision
    FROM expected
    JOIN project_profile_admissions_v3 admission
      ON admission.project_root = expected.project_root
     AND admission.ledger_revision = expected.ledger_revision
     AND admission.admission_id = expected.admission_id
     AND admission.admission_digest = expected.admission_digest
     AND admission.profile_payload_digest = expected.profile_payload_digest
     AND admission.receipt_digest = expected.receipt_digest
     AND admission.authority_resolution_ref = expected.authority_resolution_ref
     AND admission.authority_resolution_digest = expected.authority_resolution_digest
     AND admission.recorded_at = expected.recorded_at
    JOIN project_profile_revisions_v3 revision
      ON revision.project_root = admission.project_root
     AND revision.ledger_revision = admission.ledger_revision
     AND revision.admission_id = admission.admission_id
     AND revision.admission_digest = admission.admission_digest
     AND revision.profile_payload_digest = admission.profile_payload_digest
     AND revision.receipt_digest = admission.receipt_digest
     AND revision.recorded_at = admission.recorded_at
),
all_admissions AS (
    SELECT 'v1' AS storage_generation,
           admission_id, admission_digest, receipt_digest,
           authority_resolution_ref, single_use_key,
           project_root, ledger_revision
    FROM project_profile_admissions
    UNION ALL
    SELECT 'v2' AS storage_generation,
           admission_id, admission_digest, receipt_digest,
           authority_resolution_ref, single_use_key,
           project_root, ledger_revision
    FROM project_profile_admissions_v2
    UNION ALL
    SELECT 'v3' AS storage_generation,
           admission_id, admission_digest, receipt_digest,
           authority_resolution_ref, single_use_key,
           project_root, ledger_revision
    FROM project_profile_admissions_v3
)
SELECT COALESCE((
           SELECT storage_generation
           FROM exact_sources
           ORDER BY storage_generation
           LIMIT 1
       ), ''),
       (SELECT COUNT(*) FROM exact_sources),
       (SELECT COUNT(*)
        FROM exact_sources selected
        JOIN all_admissions other
          ON other.storage_generation != selected.storage_generation
         AND (
             other.admission_id = selected.admission_id
             OR other.admission_digest = selected.admission_digest
             OR other.receipt_digest = selected.receipt_digest
             OR other.authority_resolution_ref = selected.authority_resolution_ref
             OR other.single_use_key = selected.single_use_key
             OR (
                 other.project_root = selected.project_root
                 AND other.ledger_revision = selected.ledger_revision
             )
         ))`
)

type admissionStorageGeneration string

const (
	admissionStorageV1 admissionStorageGeneration = "v1"
	admissionStorageV2 admissionStorageGeneration = "v2"
	admissionStorageV3 admissionStorageGeneration = "v3"
)

type exactAdmissionSource struct {
	generation admissionStorageGeneration
}

func (source exactAdmissionSource) valid() bool {
	return source.generation == admissionStorageV1 ||
		source.generation == admissionStorageV2 ||
		source.generation == admissionStorageV3
}

type exactAdmissionSourceRow struct {
	generation                admissionStorageGeneration
	exactSourceCount          int
	crossGenerationCollisions int
}

func requireExactLedgerHead(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) (exactAdmissionSource, error) {
	count, err := countCanonicalLedgerHeads(transaction, ctx, admission.ProjectRoot())
	if err != nil {
		return exactAdmissionSource{}, err
	}
	if count != 1 {
		return exactAdmissionSource{}, fmt.Errorf(
			"%w: current profile view returned %d ledger heads",
			errLedgerHeadChanged,
			count,
		)
	}
	headGeneration, matches, err := canonicalLedgerHeadMatches(
		transaction,
		ctx,
		admission,
	)
	if err != nil {
		return exactAdmissionSource{}, err
	}
	if !matches {
		return exactAdmissionSource{}, fmt.Errorf(
			"%w: canonical profile token does not match locked revision",
			errLedgerHeadChanged,
		)
	}
	source, err := resolveExactAdmissionSource(transaction, ctx, admission)
	if err != nil {
		return exactAdmissionSource{}, err
	}
	if headGeneration != source.generation {
		return exactAdmissionSource{}, fmt.Errorf(
			"%w: current profile view generation %q differs from exact admission source %q",
			errLedgerHeadChanged,
			headGeneration,
			source.generation,
		)
	}
	return source, nil
}

func countCanonicalLedgerHeads(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
) (int, error) {
	var count int
	err := transaction.ScanOne(
		ctx,
		countCanonicalLedgerHeadSQL,
		[]any{projectRoot.String()},
		[]any{&count},
	)
	return count, err
}

func canonicalLedgerHeadMatches(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) (admissionStorageGeneration, bool, error) {
	var generation admissionStorageGeneration
	var admissionID string
	var admissionDigest string
	var payloadDigest string
	var ledgerRevision uint64
	err := transaction.ScanOne(
		ctx,
		selectCanonicalLedgerHeadSQL,
		[]any{admission.ProjectRoot().String()},
		[]any{
			&generation,
			&admissionID,
			&admissionDigest,
			&payloadDigest,
			&ledgerRevision,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, fmt.Errorf("%w: canonical profile head disappeared", errLedgerHeadChanged)
	}
	if err != nil {
		return "", false, err
	}
	matches := admissionID == admission.AdmissionRecordRef().String() &&
		admissionDigest == admission.AdmissionRecordDigest().String() &&
		payloadDigest == admission.PayloadDigest().String() &&
		ledgerRevision == admission.LedgerRevision().Value()
	return generation, matches, nil
}

func resolveExactAdmissionSource(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) (exactAdmissionSource, error) {
	row, err := loadExactAdmissionSourceRow(
		transaction,
		ctx,
		exactAdmissionSourceArguments(admission),
	)
	if err != nil {
		return exactAdmissionSource{}, err
	}
	return exactAdmissionSourceFromRow(row)
}

func loadExactAdmissionSourceRow(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	arguments []any,
) (exactAdmissionSourceRow, error) {
	row := exactAdmissionSourceRow{}
	err := transaction.ScanOne(
		ctx,
		selectExactAdmissionSourceSQL,
		arguments,
		[]any{
			&row.generation,
			&row.exactSourceCount,
			&row.crossGenerationCollisions,
		},
	)
	if err != nil {
		return exactAdmissionSourceRow{}, fmt.Errorf(
			"prove exact profile-admission source: %w",
			err,
		)
	}
	return row, nil
}

func exactAdmissionSourceFromRow(
	row exactAdmissionSourceRow,
) (exactAdmissionSource, error) {
	if row.exactSourceCount != 1 {
		return exactAdmissionSource{}, fmt.Errorf(
			"canonical profile admission resolves to %d exact storage sources; expected one",
			row.exactSourceCount,
		)
	}
	if row.crossGenerationCollisions != 0 {
		return exactAdmissionSource{}, fmt.Errorf(
			"canonical profile admission has %d cross-generation identity collision(s)",
			row.crossGenerationCollisions,
		)
	}
	source := exactAdmissionSource{generation: row.generation}
	if !source.valid() {
		return exactAdmissionSource{}, fmt.Errorf(
			"canonical profile admission has unknown storage generation %q",
			row.generation,
		)
	}
	return source, nil
}

func exactAdmissionSourceArguments(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) []any {
	return []any{
		admission.ProjectRoot().String(),
		admission.LedgerRevision().Value(),
		admission.AdmissionRecordRef().String(),
		admission.AdmissionRecordDigest().String(),
		admission.PayloadDigest().String(),
		admission.ReceiptDigest().String(),
		admission.AuthorityResolutionRef().String(),
		admission.AuthorityResolutionDigest().String(),
		admission.RecordedAt().UTC().Format(time.RFC3339Nano),
	}
}

func canonicalLedgerHeadExists(
	transaction *sqlitetransaction.Transaction,
	ctx context.Context,
	projectRoot projectprofile.ProjectRootV1,
) (bool, error) {
	count, err := countCanonicalLedgerHeads(transaction, ctx, projectRoot)
	if err != nil {
		return false, err
	}
	if count > 1 {
		return false, fmt.Errorf("current profile view returned %d ledger heads", count)
	}
	return count == 1, nil
}
