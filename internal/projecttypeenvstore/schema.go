package projecttypeenvstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const CurrentSchemaVersion = 2

const createSchemaVersionTable = `CREATE TABLE IF NOT EXISTS project_typeenv_artifact_store_schema (
	singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
	version INTEGER NOT NULL CHECK (version > 0)
)`

var schemaMigrations = []schemaMigration{
	{
		version: 1,
		statements: []string{
			`CREATE TABLE IF NOT EXISTS project_typeenv_artifacts (
				artifact_kind TEXT NOT NULL CHECK (artifact_kind IN (
					'base_type_env',
					'project_type_env_extension',
					'runtime_evaluation_basis',
					'project_type_env_composite'
				)),
				artifact_ref TEXT NOT NULL,
				artifact_digest TEXT NOT NULL,
				canonical_schema_version TEXT NOT NULL,
				producer_schema_version TEXT NOT NULL,
				canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0),
				PRIMARY KEY (artifact_kind, artifact_ref),
				UNIQUE (artifact_kind, artifact_digest)
			)`,
			`CREATE INDEX IF NOT EXISTS project_typeenv_artifacts_ref
				ON project_typeenv_artifacts(artifact_ref, artifact_kind)`,
			`CREATE TABLE IF NOT EXISTS project_typeenv_runtime_mechanisms (
				artifact_ref TEXT NOT NULL,
				edition TEXT NOT NULL,
				artifact_digest TEXT NOT NULL,
				canonical_schema_version TEXT NOT NULL,
				canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0),
				PRIMARY KEY (artifact_ref, edition)
			)`,
			`CREATE INDEX IF NOT EXISTS project_typeenv_runtime_mechanisms_digest
				ON project_typeenv_runtime_mechanisms(artifact_digest)`,
		},
	},
	{
		version: 2,
		statements: []string{
			`CREATE TABLE IF NOT EXISTS project_typeenv_registration_policies (
				registration_ref TEXT PRIMARY KEY,
				artifact_digest TEXT NOT NULL UNIQUE,
				canonical_schema_version TEXT NOT NULL,
				canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0)
			)`,
			`CREATE INDEX IF NOT EXISTS project_typeenv_registration_policies_digest
				ON project_typeenv_registration_policies(artifact_digest)`,
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
		return fmt.Errorf("begin project TypeEnv artifact schema transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	if _, err := transaction.ExecContext(ctx, createSchemaVersionTable); err != nil {
		return fmt.Errorf("create project TypeEnv artifact schema version table: %w", err)
	}
	current, err := readSchemaVersion(ctx, transaction)
	if err != nil {
		return err
	}
	if current > CurrentSchemaVersion {
		return fmt.Errorf(
			"project TypeEnv artifact schema version %d is newer than supported version %d",
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
				"project TypeEnv artifact schema migration gap from %d to %d",
				current,
				migration.version,
			)
		}
		for _, statement := range migration.statements {
			if _, err := transaction.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf(
					"apply project TypeEnv artifact schema migration %d: %w",
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
			"project TypeEnv artifact schema version is %d; want %d",
			current,
			CurrentSchemaVersion,
		)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit project TypeEnv artifact schema transaction: %w", err)
	}
	return nil
}

func readSchemaVersion(ctx context.Context, transaction *sql.Tx) (int, error) {
	var version int
	err := transaction.QueryRowContext(
		ctx,
		`SELECT version FROM project_typeenv_artifact_store_schema WHERE singleton = 1`,
	).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read project TypeEnv artifact schema version: %w", err)
	}
	if version <= 0 {
		return 0, fmt.Errorf("project TypeEnv artifact schema version %d is invalid", version)
	}
	return version, nil
}

func writeSchemaVersion(ctx context.Context, transaction *sql.Tx, version int) error {
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_artifact_store_schema (singleton, version)
		 VALUES (1, ?)
		 ON CONFLICT(singleton) DO UPDATE SET version = excluded.version`,
		version,
	)
	if err != nil {
		return fmt.Errorf("write project TypeEnv artifact schema version %d: %w", version, err)
	}
	return nil
}
