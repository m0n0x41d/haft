package profileprojection

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	_ "modernc.org/sqlite"
)

func TestLegacyV1OpenDebtResolvesThroughV3TaggedEvent(t *testing.T) {
	database := openDebtStorageTestDatabase(t)
	defer database.Close()
	opened := debtStorageTestRecord(t, debtEventStorageV1, admissionStorageV1)
	insertLegacyDebtStorageTestEvent(t, database, opened)

	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("begin debt reconciliation: %v", err)
	}
	scope, err := newDebtEventScope(exactAdmissionSource{generation: admissionStorageV1})
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("create v1 debt scope: %v", err)
	}
	exact := debtStorageTestExactArguments(opened)
	if err := validateExactDebtChain(transaction, ctx, scope, exact); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("validate historical open debt: %v", err)
	}
	service := Service{now: func() time.Time {
		return time.Date(2026, time.July, 15, 8, 9, 11, 123, time.UTC)
	}}
	observed := debtStorageTestDigest(t, "2")
	err = service.resolveExactDebt(
		transaction,
		ctx,
		exactAdmissionSource{generation: admissionStorageV1},
		opened,
		observed,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("resolve historical open debt: %v", err)
	}
	if err := validateExactDebtChain(transaction, ctx, scope, exact); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("validate cross-storage resolution chain: %v", err)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit cross-storage resolution: %v", finish.Err())
	}

	assertDebtStorageTestCounts(t, database, 1, 0, 1)
	assertDebtStorageTestResolution(t, database, opened, observed)
}

func TestHistoricalV2OpenDebtResolvesThroughV3TaggedEvent(t *testing.T) {
	database := openDebtStorageTestDatabase(t)
	defer database.Close()
	opened := debtStorageTestRecord(t, debtEventStorageV2, admissionStorageV2)
	insertV2DebtStorageTestEvent(t, database, opened)

	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("begin v2 debt reconciliation: %v", err)
	}
	source := exactAdmissionSource{generation: admissionStorageV2}
	scope, err := newDebtEventScope(source)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("create v2 debt scope: %v", err)
	}
	exact := debtStorageTestExactArguments(opened)
	if err := validateExactDebtChain(transaction, ctx, scope, exact); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("validate historical v2 open debt: %v", err)
	}
	service := Service{now: func() time.Time {
		return time.Date(2026, time.July, 15, 8, 9, 11, 123, time.UTC)
	}}
	observed := debtStorageTestDigest(t, "2")
	err = service.resolveExactDebt(
		transaction,
		ctx,
		source,
		opened,
		observed,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("resolve historical v2 open debt: %v", err)
	}
	if err := validateExactDebtChain(transaction, ctx, scope, exact); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("validate v2-to-v3 resolution chain: %v", err)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit v2-to-v3 resolution: %v", finish.Err())
	}

	assertDebtStorageTestCounts(t, database, 0, 1, 1)
	assertDebtStorageTestResolutionForGeneration(
		t,
		database,
		opened,
		observed,
		"v2",
	)
}

func TestNewDebtEventsForAllAdmissionGenerationsWriteOnlyV3(t *testing.T) {
	database := openDebtStorageTestDatabase(t)
	defer database.Close()
	v1 := debtStorageTestRecord(t, debtEventStorageV3, admissionStorageV1)
	v2 := debtStorageTestRecord(t, debtEventStorageV3, admissionStorageV2)
	v3 := debtStorageTestRecord(t, debtEventStorageV3, admissionStorageV3)
	v2.eventID = "projection-event.opened-v2"
	v2.debtID = "projection-debt.v2"
	v3.eventID = "projection-event.opened-v3"
	v3.debtID = "projection-debt.v3"

	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("begin new debt event writes: %v", err)
	}
	if err := insertOpenedDebtEvent(transaction, ctx, v1); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("write v1-profile debt to v3 event sum: %v", err)
	}
	if err := insertOpenedDebtEvent(transaction, ctx, v2); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("write v2-profile debt to v3 event sum: %v", err)
	}
	if err := insertOpenedDebtEvent(transaction, ctx, v3); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("write v3-profile debt to v3 event sum: %v", err)
	}
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit tagged debt events: %v", finish.Err())
	}

	assertDebtStorageTestCounts(t, database, 0, 0, 3)
	var v1Count int
	var v2Count int
	var v3Count int
	var invalidLineageCount int
	err = database.QueryRow(
		`SELECT
			SUM(CASE WHEN profile_revision_generation = 'v1' THEN 1 ELSE 0 END),
			SUM(CASE WHEN profile_revision_generation = 'v2' THEN 1 ELSE 0 END),
			SUM(CASE WHEN profile_revision_generation = 'v3' THEN 1 ELSE 0 END),
			SUM(CASE WHEN supersedes_event_generation IS NOT NULL
			          OR supersedes_event_id IS NOT NULL THEN 1 ELSE 0 END)
		 FROM project_profile_projection_debt_v3`,
	).Scan(&v1Count, &v2Count, &v3Count, &invalidLineageCount)
	if err != nil {
		t.Fatalf("read tagged opened debt events: %v", err)
	}
	if v1Count != 1 || v2Count != 1 || v3Count != 1 || invalidLineageCount != 0 {
		t.Fatalf(
			"tagged opened events = v1:%d v2:%d v3:%d invalid-lineage:%d",
			v1Count,
			v2Count,
			v3Count,
			invalidLineageCount,
		)
	}
}

func TestV2AdmissionDebtScopeRejectsLegacyEventOwnership(t *testing.T) {
	database := openDebtStorageTestDatabase(t)
	defer database.Close()
	opened := debtStorageTestRecord(t, debtEventStorageV1, admissionStorageV2)
	insertLegacyDebtStorageTestEvent(t, database, opened)

	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("begin debt ownership validation: %v", err)
	}
	scope, err := newDebtEventScope(exactAdmissionSource{generation: admissionStorageV2})
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("create v2 debt scope: %v", err)
	}
	err = validateExactDebtChain(
		transaction,
		ctx,
		scope,
		debtStorageTestExactArguments(opened),
	)
	_ = transaction.Rollback(ctx)
	if err == nil || !strings.Contains(err.Error(), "outside its exact admission generation") {
		t.Fatalf("v2 admission accepted legacy debt ownership: %v", err)
	}
}

func TestV3AdmissionDebtScopeRejectsV2EventOwnership(t *testing.T) {
	database := openDebtStorageTestDatabase(t)
	defer database.Close()
	opened := debtStorageTestRecord(t, debtEventStorageV2, admissionStorageV2)
	insertV2DebtStorageTestEvent(t, database, opened)

	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("begin v3 debt ownership validation: %v", err)
	}
	scope, err := newDebtEventScope(exactAdmissionSource{generation: admissionStorageV3})
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("create v3 debt scope: %v", err)
	}
	err = validateExactDebtChain(
		transaction,
		ctx,
		scope,
		debtStorageTestExactArguments(opened),
	)
	_ = transaction.Rollback(ctx)
	if err == nil || !strings.Contains(err.Error(), "outside its exact admission generation") {
		t.Fatalf("v3 admission accepted v2 debt ownership: %v", err)
	}
}

func openDebtStorageTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open(
		"sqlite",
		"file:"+filepath.Join(t.TempDir(), "debt-storage.db")+"?_pragma=foreign_keys(1)",
	)
	if err != nil {
		t.Fatalf("open debt storage test database: %v", err)
	}
	statements := []string{
		`CREATE TABLE project_profile_projection_debt (
			event_id TEXT PRIMARY KEY,
			debt_id TEXT NOT NULL,
			admission_id TEXT NOT NULL,
			admission_digest TEXT NOT NULL,
			project_root TEXT NOT NULL,
			ledger_revision INTEGER NOT NULL,
			profile_payload_digest TEXT NOT NULL,
			projection_path TEXT NOT NULL,
			event_kind TEXT NOT NULL,
			reason_code TEXT NOT NULL,
			detail TEXT NOT NULL,
			expected_projection_digest TEXT NOT NULL,
			observed_projection_digest TEXT NOT NULL,
			supersedes_event_id TEXT,
			recorded_at TEXT NOT NULL
		)`,
		`CREATE TABLE project_profile_projection_debt_v2 (
			event_id TEXT PRIMARY KEY,
			debt_id TEXT NOT NULL,
			profile_revision_generation TEXT NOT NULL,
			admission_id TEXT NOT NULL,
			admission_digest TEXT NOT NULL,
			project_root TEXT NOT NULL,
			ledger_revision INTEGER NOT NULL,
			profile_payload_digest TEXT NOT NULL,
			projection_path TEXT NOT NULL,
			event_kind TEXT NOT NULL,
			reason_code TEXT NOT NULL,
			detail TEXT NOT NULL,
			expected_projection_digest TEXT NOT NULL,
			observed_projection_digest TEXT NOT NULL,
			supersedes_event_generation TEXT,
			supersedes_event_id TEXT,
			recorded_at TEXT NOT NULL
		)`,
		`CREATE TABLE project_profile_projection_debt_v3 (
			event_id TEXT PRIMARY KEY,
			debt_id TEXT NOT NULL,
			profile_revision_generation TEXT NOT NULL,
			admission_id TEXT NOT NULL,
			admission_digest TEXT NOT NULL,
			project_root TEXT NOT NULL,
			ledger_revision INTEGER NOT NULL,
			profile_payload_digest TEXT NOT NULL,
			projection_path TEXT NOT NULL,
			event_kind TEXT NOT NULL,
			reason_code TEXT NOT NULL,
			detail TEXT NOT NULL,
			expected_projection_digest TEXT NOT NULL,
			observed_projection_digest TEXT NOT NULL,
			supersedes_event_generation TEXT,
			supersedes_event_id TEXT,
			recorded_at TEXT NOT NULL
		)`,
	}
	executeDebtStorageTestStatements(t, database, statements, 0)
	return database
}

func executeDebtStorageTestStatements(
	t *testing.T,
	database *sql.DB,
	statements []string,
	index int,
) {
	t.Helper()
	if index >= len(statements) {
		return
	}
	_, err := database.Exec(statements[index])
	if err != nil {
		t.Fatalf("create debt storage test schema: %v", err)
	}
	executeDebtStorageTestStatements(t, database, statements, index+1)
}

func debtStorageTestRecord(
	t *testing.T,
	storage debtEventStorageGeneration,
	profileGeneration admissionStorageGeneration,
) debtRecord {
	t.Helper()
	return debtRecord{
		storageGeneration:         storage,
		profileRevisionGeneration: profileGeneration,
		eventID:                   "projection-event.opened",
		debtID:                    "projection-debt.1",
		admissionID:               "profile-admission.1",
		admissionDigest:           debtStorageTestDigest(t, "a").String(),
		projectRoot:               filepath.Join(t.TempDir(), "project"),
		ledgerRevision:            1,
		profilePayloadDigest:      debtStorageTestDigest(t, "b").String(),
		projectionPath:            filepath.Join(t.TempDir(), "project", projectionRelativePath),
		reasonCode:                "projection_missing",
		detail:                    "projection is absent",
		expectedProjectionDigest:  debtStorageTestDigest(t, "c").String(),
		observedProjectionDigest:  "",
		recordedAt:                "2026-07-15T08:09:10.000000001Z",
	}
}

func debtStorageTestDigest(t *testing.T, digit string) projectprofile.ContentDigest {
	t.Helper()
	digest, err := projectprofile.NewContentDigest("sha256:" + strings.Repeat(digit, 64))
	if err != nil {
		t.Fatalf("create test digest: %v", err)
	}
	return digest
}

func debtStorageTestExactArguments(record debtRecord) []any {
	return []any{
		record.projectRoot,
		record.ledgerRevision,
		record.admissionID,
		record.admissionDigest,
		record.profilePayloadDigest,
		record.projectionPath,
		record.expectedProjectionDigest,
	}
}

func insertLegacyDebtStorageTestEvent(
	t *testing.T,
	database *sql.DB,
	record debtRecord,
) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO project_profile_projection_debt (
			event_id, debt_id, admission_id, admission_digest,
			project_root, ledger_revision, profile_payload_digest,
			projection_path, event_kind, reason_code, detail,
			expected_projection_digest, observed_projection_digest,
			supersedes_event_id, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'opened', ?, ?, ?, ?, NULL, ?)`,
		record.eventID,
		record.debtID,
		record.admissionID,
		record.admissionDigest,
		record.projectRoot,
		record.ledgerRevision,
		record.profilePayloadDigest,
		record.projectionPath,
		record.reasonCode,
		record.detail,
		record.expectedProjectionDigest,
		record.observedProjectionDigest,
		record.recordedAt,
	)
	if err != nil {
		t.Fatalf("insert historical projection debt: %v", err)
	}
}

func insertV2DebtStorageTestEvent(
	t *testing.T,
	database *sql.DB,
	record debtRecord,
) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO project_profile_projection_debt_v2 (
			event_id, debt_id, profile_revision_generation,
			admission_id, admission_digest,
			project_root, ledger_revision, profile_payload_digest,
			projection_path, event_kind, reason_code, detail,
			expected_projection_digest, observed_projection_digest,
			supersedes_event_generation, supersedes_event_id, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'opened', ?, ?, ?, ?, NULL, NULL, ?)`,
		record.eventID,
		record.debtID,
		string(record.profileRevisionGeneration),
		record.admissionID,
		record.admissionDigest,
		record.projectRoot,
		record.ledgerRevision,
		record.profilePayloadDigest,
		record.projectionPath,
		record.reasonCode,
		record.detail,
		record.expectedProjectionDigest,
		record.observedProjectionDigest,
		record.recordedAt,
	)
	if err != nil {
		t.Fatalf("insert historical v2 projection debt: %v", err)
	}
}

func assertDebtStorageTestCounts(
	t *testing.T,
	database *sql.DB,
	wantLegacy int,
	wantV2 int,
	wantV3 int,
) {
	t.Helper()
	var legacy int
	var v2 int
	var v3 int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM project_profile_projection_debt",
	).Scan(&legacy); err != nil {
		t.Fatalf("count legacy debt events: %v", err)
	}
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM project_profile_projection_debt_v2",
	).Scan(&v2); err != nil {
		t.Fatalf("count v2 debt events: %v", err)
	}
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM project_profile_projection_debt_v3",
	).Scan(&v3); err != nil {
		t.Fatalf("count v3 debt events: %v", err)
	}
	if legacy != wantLegacy || v2 != wantV2 || v3 != wantV3 {
		t.Fatalf(
			"debt storage counts = legacy:%d v2:%d v3:%d, want legacy:%d v2:%d v3:%d",
			legacy,
			v2,
			v3,
			wantLegacy,
			wantV2,
			wantV3,
		)
	}
}

func assertDebtStorageTestResolution(
	t *testing.T,
	database *sql.DB,
	opened debtRecord,
	observed projectprofile.ContentDigest,
) {
	assertDebtStorageTestResolutionForGeneration(
		t,
		database,
		opened,
		observed,
		"v1",
	)
}

func assertDebtStorageTestResolutionForGeneration(
	t *testing.T,
	database *sql.DB,
	opened debtRecord,
	observed projectprofile.ContentDigest,
	wantGeneration string,
) {
	t.Helper()
	var profileGeneration string
	var eventKind string
	var supersedesGeneration string
	var supersedesEventID string
	var observedDigest string
	err := database.QueryRow(
		`SELECT profile_revision_generation, event_kind,
		        supersedes_event_generation, supersedes_event_id,
		        observed_projection_digest
		 FROM project_profile_projection_debt_v3`,
	).Scan(
		&profileGeneration,
		&eventKind,
		&supersedesGeneration,
		&supersedesEventID,
		&observedDigest,
	)
	if err != nil {
		t.Fatalf("read v3 resolution event: %v", err)
	}
	if profileGeneration != wantGeneration ||
		eventKind != "resolved" ||
		supersedesGeneration != wantGeneration ||
		supersedesEventID != opened.eventID ||
		observedDigest != observed.String() {
		t.Fatalf(
			"v3 resolution lineage = generation:%q kind:%q supersedes:%q/%q observed:%q",
			profileGeneration,
			eventKind,
			supersedesGeneration,
			supersedesEventID,
			observedDigest,
		)
	}
}
