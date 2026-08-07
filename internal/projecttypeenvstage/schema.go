package projecttypeenvstage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const CurrentSchemaVersion = 2

const createSchemaVersionTable = `CREATE TABLE IF NOT EXISTS project_typeenv_stage_store_schema (
	singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
	version INTEGER NOT NULL CHECK (version > 0)
)`

var schemaMigrations = []schemaMigration{
	{
		version: 1,
		statements: []string{
			`CREATE TABLE IF NOT EXISTS project_typeenv_composite_verifications (
				verification_ref TEXT PRIMARY KEY,
				verification_digest TEXT NOT NULL UNIQUE,
				lowerer_schema_version TEXT NOT NULL,
				canonical_schema_version TEXT NOT NULL,
				canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0)
			)`,
			`CREATE TABLE IF NOT EXISTS project_typeenv_stages (
				stage_ref TEXT PRIMARY KEY,
				stage_digest TEXT NOT NULL UNIQUE,
				project_id TEXT NOT NULL,
				composite_verification_ref TEXT NOT NULL,
				canonical_schema_version TEXT NOT NULL,
				canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0),
				FOREIGN KEY (composite_verification_ref)
					REFERENCES project_typeenv_composite_verifications(verification_ref)
			)`,
			`CREATE INDEX IF NOT EXISTS project_typeenv_stages_project
				ON project_typeenv_stages(project_id, stage_ref)`,
		},
	},
	{
		version: 2,
		statements: []string{
			`CREATE TABLE IF NOT EXISTS project_typeenv_executable_snapshots (
				type_env_ref TEXT PRIMARY KEY,
				snapshot_digest TEXT NOT NULL UNIQUE,
				lowered_environment_digest TEXT NOT NULL,
				source_revision TEXT NOT NULL,
				compiler_schema_version TEXT NOT NULL,
				lowerer_schema_version TEXT NOT NULL,
				verification_ref TEXT NOT NULL,
				canonical_schema_version TEXT NOT NULL,
				canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0),
				FOREIGN KEY (verification_ref)
					REFERENCES project_typeenv_composite_verifications(verification_ref)
			)`,
			`ALTER TABLE project_typeenv_stages
				ADD COLUMN executable_type_env_ref TEXT
				REFERENCES project_typeenv_executable_snapshots(type_env_ref)`,
			`CREATE INDEX IF NOT EXISTS project_typeenv_stages_executable_snapshot
				ON project_typeenv_stages(executable_type_env_ref, stage_ref)`,
		},
	},
}

type schemaMigration struct {
	version    int
	statements []string
}

func ensureSchema(ctx context.Context, database *sql.DB) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if database == nil {
		return ErrStoreRequired
	}
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin project TypeEnv Stage schema transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	if _, err := transaction.ExecContext(ctx, createSchemaVersionTable); err != nil {
		return fmt.Errorf("create project TypeEnv Stage schema version table: %w", err)
	}
	current, err := readSchemaVersion(ctx, transaction)
	if err != nil {
		return err
	}
	if current > CurrentSchemaVersion {
		return fmt.Errorf(
			"project TypeEnv Stage schema version %d is newer than supported version %d",
			current,
			CurrentSchemaVersion,
		)
	}
	for _, migration := range schemaMigrations {
		if migration.version <= current {
			continue
		}
		if migration.version != current+1 {
			return fmt.Errorf(
				"project TypeEnv Stage schema migration gap from %d to %d",
				current,
				migration.version,
			)
		}
		for _, statement := range migration.statements {
			if _, err := transaction.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf(
					"apply project TypeEnv Stage schema migration %d: %w",
					migration.version,
					err,
				)
			}
		}
		if err := writeSchemaVersion(ctx, transaction, migration.version); err != nil {
			return err
		}
		current = migration.version
	}
	if current != CurrentSchemaVersion {
		return fmt.Errorf(
			"project TypeEnv Stage schema version is %d; want %d",
			current,
			CurrentSchemaVersion,
		)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit project TypeEnv Stage schema transaction: %w", err)
	}
	return nil
}

func readSchemaVersion(ctx context.Context, transaction *sql.Tx) (int, error) {
	var version int
	err := transaction.QueryRowContext(
		ctx,
		`SELECT version FROM project_typeenv_stage_store_schema WHERE singleton = 1`,
	).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read project TypeEnv Stage schema version: %w", err)
	}
	if version <= 0 {
		return 0, fmt.Errorf("project TypeEnv Stage schema version %d is invalid", version)
	}
	return version, nil
}

func writeSchemaVersion(ctx context.Context, transaction *sql.Tx, version int) error {
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_stage_store_schema (singleton, version)
		 VALUES (1, ?)
		 ON CONFLICT(singleton) DO UPDATE SET version = excluded.version`,
		version,
	)
	if err != nil {
		return fmt.Errorf("write project TypeEnv Stage schema version %d: %w", version, err)
	}
	return nil
}
