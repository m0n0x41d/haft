package profileprojection

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	_ "modernc.org/sqlite"
)

func TestExactAdmissionSourceSelectsV2AndRejectsCrossGenerationCollision(
	t *testing.T,
) {
	database := openAdmissionGenerationTestDatabase(t)
	defer database.Close()
	expected := admissionGenerationTestArguments()
	insertAdmissionGenerationExactV2(t, database, expected)

	row := readAdmissionGenerationTestRow(t, database, expected)
	source, err := exactAdmissionSourceFromRow(row)
	if err != nil {
		t.Fatalf("select exact v2 admission source: %v", err)
	}
	if source.generation != admissionStorageV2 {
		t.Fatalf("exact source generation = %q, want v2", source.generation)
	}

	insertAdmissionGenerationLegacyCollision(t, database, expected)
	row = readAdmissionGenerationTestRow(t, database, expected)
	_, err = exactAdmissionSourceFromRow(row)
	if err == nil || !strings.Contains(err.Error(), "cross-generation identity collision") {
		t.Fatalf("cross-generation admission identity was accepted: %v", err)
	}
}

func TestExactAdmissionSourceSelectsV3(t *testing.T) {
	database := openAdmissionGenerationTestDatabase(t)
	defer database.Close()
	expected := admissionGenerationTestArguments()
	insertAdmissionGenerationExactV3(t, database, expected)

	row := readAdmissionGenerationTestRow(t, database, expected)
	source, err := exactAdmissionSourceFromRow(row)
	if err != nil {
		t.Fatalf("select exact v3 admission source: %v", err)
	}
	if source.generation != admissionStorageV3 {
		t.Fatalf("exact source generation = %q, want v3", source.generation)
	}
}

func TestExactAdmissionSourceSelectsLegacyV1(t *testing.T) {
	database := openAdmissionGenerationTestDatabase(t)
	defer database.Close()
	expected := admissionGenerationTestArguments()
	insertAdmissionGenerationExactV1(t, database, expected)

	row := readAdmissionGenerationTestRow(t, database, expected)
	source, err := exactAdmissionSourceFromRow(row)
	if err != nil {
		t.Fatalf("select exact legacy v1 admission source: %v", err)
	}
	if source.generation != admissionStorageV1 {
		t.Fatalf("exact source generation = %q, want v1", source.generation)
	}
}

func openAdmissionGenerationTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open(
		"sqlite",
		"file:"+filepath.Join(t.TempDir(), "admission-generation.db"),
	)
	if err != nil {
		t.Fatalf("open admission generation test database: %v", err)
	}
	statements := []string{
		admissionGenerationTestAdmissionTable("project_profile_admissions"),
		admissionGenerationTestAdmissionTable("project_profile_admissions_v2"),
		admissionGenerationTestAdmissionTable("project_profile_admissions_v3"),
		admissionGenerationTestRevisionTable("project_profile_revisions"),
		admissionGenerationTestRevisionTable("project_profile_revisions_v2"),
		admissionGenerationTestRevisionTable("project_profile_revisions_v3"),
	}
	executeAdmissionGenerationTestStatements(t, database, statements, 0)
	return database
}

func insertAdmissionGenerationExactV3(
	t *testing.T,
	database *sql.DB,
	expected []any,
) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO project_profile_admissions_v3 (
			project_root, ledger_revision, admission_id, admission_digest,
			profile_payload_digest, receipt_digest,
			authority_resolution_ref, authority_resolution_digest,
			recorded_at, single_use_key
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		append(expected, "single-use:exact-v3")...,
	)
	if err != nil {
		t.Fatalf("insert exact v3 admission: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO project_profile_revisions_v3 (
			project_root, ledger_revision, admission_id, admission_digest,
			profile_payload_digest, receipt_digest, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		expected[0],
		expected[1],
		expected[2],
		expected[3],
		expected[4],
		expected[5],
		expected[8],
	)
	if err != nil {
		t.Fatalf("insert exact v3 revision: %v", err)
	}
}

func insertAdmissionGenerationExactV1(
	t *testing.T,
	database *sql.DB,
	expected []any,
) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO project_profile_admissions (
			project_root, ledger_revision, admission_id, admission_digest,
			profile_payload_digest, receipt_digest,
			authority_resolution_ref, authority_resolution_digest,
			recorded_at, single_use_key
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		append(expected, "single-use:exact-v1")...,
	)
	if err != nil {
		t.Fatalf("insert exact legacy v1 admission: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO project_profile_revisions (
			project_root, ledger_revision, admission_id, admission_digest,
			profile_payload_digest, receipt_digest, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		expected[0],
		expected[1],
		expected[2],
		expected[3],
		expected[4],
		expected[5],
		expected[8],
	)
	if err != nil {
		t.Fatalf("insert exact legacy v1 revision: %v", err)
	}
}

func admissionGenerationTestAdmissionTable(name string) string {
	return `CREATE TABLE ` + name + ` (
		admission_id TEXT PRIMARY KEY,
		admission_digest TEXT NOT NULL,
		receipt_digest TEXT NOT NULL,
		authority_resolution_ref TEXT NOT NULL,
		authority_resolution_digest TEXT NOT NULL,
		single_use_key TEXT NOT NULL,
		project_root TEXT NOT NULL,
		ledger_revision INTEGER NOT NULL,
		profile_payload_digest TEXT NOT NULL,
		recorded_at TEXT NOT NULL
	)`
}

func admissionGenerationTestRevisionTable(name string) string {
	return `CREATE TABLE ` + name + ` (
		project_root TEXT NOT NULL,
		ledger_revision INTEGER NOT NULL,
		admission_id TEXT NOT NULL,
		admission_digest TEXT NOT NULL,
		profile_payload_digest TEXT NOT NULL,
		receipt_digest TEXT NOT NULL,
		recorded_at TEXT NOT NULL
	)`
}

func executeAdmissionGenerationTestStatements(
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
		t.Fatalf("create admission generation test schema: %v", err)
	}
	executeAdmissionGenerationTestStatements(t, database, statements, index+1)
}

func admissionGenerationTestArguments() []any {
	return []any{
		"/tmp/profile-generation-project",
		uint64(3),
		"profile-admission.exact-v2",
		"sha256:" + strings.Repeat("a", 64),
		"sha256:" + strings.Repeat("b", 64),
		"sha256:" + strings.Repeat("c", 64),
		"profile-authority-resolution:exact-v2",
		"sha256:" + strings.Repeat("d", 64),
		"2026-07-15T08:09:10.123456789Z",
	}
}

func insertAdmissionGenerationExactV2(
	t *testing.T,
	database *sql.DB,
	expected []any,
) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO project_profile_admissions_v2 (
			project_root, ledger_revision, admission_id, admission_digest,
			profile_payload_digest, receipt_digest,
			authority_resolution_ref, authority_resolution_digest,
			recorded_at, single_use_key
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		append(expected, "single-use:exact-v2")...,
	)
	if err != nil {
		t.Fatalf("insert exact v2 admission: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO project_profile_revisions_v2 (
			project_root, ledger_revision, admission_id, admission_digest,
			profile_payload_digest, receipt_digest, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		expected[0],
		expected[1],
		expected[2],
		expected[3],
		expected[4],
		expected[5],
		expected[8],
	)
	if err != nil {
		t.Fatalf("insert exact v2 revision: %v", err)
	}
}

func insertAdmissionGenerationLegacyCollision(
	t *testing.T,
	database *sql.DB,
	expected []any,
) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO project_profile_admissions (
			project_root, ledger_revision, admission_id, admission_digest,
			profile_payload_digest, receipt_digest,
			authority_resolution_ref, authority_resolution_digest,
			recorded_at, single_use_key
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"/tmp/another-project",
		uint64(1),
		"profile-admission.legacy-collision",
		"sha256:"+strings.Repeat("e", 64),
		"sha256:"+strings.Repeat("f", 64),
		expected[5],
		"authority-resolution:legacy-collision",
		"sha256:"+strings.Repeat("1", 64),
		"2026-07-15T08:09:09Z",
		"single-use:legacy-collision",
	)
	if err != nil {
		t.Fatalf("insert legacy identity collision: %v", err)
	}
}

func readAdmissionGenerationTestRow(
	t *testing.T,
	database *sql.DB,
	expected []any,
) exactAdmissionSourceRow {
	t.Helper()
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, database)
	if err != nil {
		t.Fatalf("begin exact admission source read: %v", err)
	}
	row, err := loadExactAdmissionSourceRow(transaction, ctx, expected)
	finish := transaction.Rollback(ctx)
	if err != nil {
		t.Fatalf("load exact admission source row: %v", err)
	}
	if !finish.Succeeded() {
		t.Fatalf("finish exact admission source read: %v", finish.Err())
	}
	return row
}
