package projecttypeenvstage

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestStageStoreSchemaV1ToV2PreservesHistoricalRows(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	ctx := context.Background()
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin v1 schema transaction: %v", err)
	}
	if _, err := transaction.ExecContext(ctx, createSchemaVersionTable); err != nil {
		t.Fatalf("create schema-version table: %v", err)
	}
	for _, statement := range schemaMigrations[0].statements {
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			t.Fatalf("apply v1 statement: %v", err)
		}
	}
	if err := writeSchemaVersion(ctx, transaction, 1); err != nil {
		t.Fatalf("write v1 schema version: %v", err)
	}
	if _, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_composite_verifications (
			verification_ref,
			verification_digest,
			lowerer_schema_version,
			canonical_schema_version,
			canonical_bytes
		) VALUES ('verification:v1', 'sha256:v1', 'lowerer/v1', 'record/v1', X'01')`,
	); err != nil {
		t.Fatalf("insert historical verification row: %v", err)
	}
	if _, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_stages (
			stage_ref,
			stage_digest,
			project_id,
			composite_verification_ref,
			canonical_schema_version,
			canonical_bytes
		) VALUES ('stage:v1', 'sha256:stage-v1', 'project:v1', 'verification:v1', 'stage/v1', X'01')`,
	); err != nil {
		t.Fatalf("insert historical Stage row: %v", err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit v1 schema fixture: %v", err)
	}

	if err := ensureSchema(ctx, database); err != nil {
		t.Fatalf("migrate Stage schema v1 to v2: %v", err)
	}

	var version int
	if err := database.QueryRowContext(
		ctx,
		`SELECT version FROM project_typeenv_stage_store_schema WHERE singleton = 1`,
	).Scan(&version); err != nil {
		t.Fatalf("read migrated schema version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("schema version = %d; want %d", version, CurrentSchemaVersion)
	}
	var executableRef sql.NullString
	if err := database.QueryRowContext(
		ctx,
		`SELECT executable_type_env_ref FROM project_typeenv_stages WHERE stage_ref = 'stage:v1'`,
	).Scan(&executableRef); err != nil {
		t.Fatalf("read historical Stage after migration: %v", err)
	}
	if executableRef.Valid {
		t.Fatalf("historical v1 Stage acquired executable snapshot %q", executableRef.String)
	}
	var snapshotTableCount int
	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master
		 WHERE type = 'table' AND name = 'project_typeenv_executable_snapshots'`,
	).Scan(&snapshotTableCount); err != nil {
		t.Fatalf("inspect executable snapshot table: %v", err)
	}
	if snapshotTableCount != 1 {
		t.Fatalf("executable snapshot table count = %d; want 1", snapshotTableCount)
	}
}
