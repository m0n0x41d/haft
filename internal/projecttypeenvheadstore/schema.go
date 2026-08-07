package projecttypeenvheadstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const CurrentSchemaVersion = 1

const createSchemaVersionTable = `CREATE TABLE IF NOT EXISTS project_typeenv_head_store_schema (
	singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
	version INTEGER NOT NULL CHECK (version > 0)
)`

var schemaMigrations = []schemaMigration{
	{
		version: 1,
		statements: []string{
			`CREATE TABLE IF NOT EXISTS project_typeenv_head_states (
				project_id TEXT NOT NULL
					CHECK(project_id != '' AND trim(project_id) = project_id),
				head_ref TEXT NOT NULL
					CHECK(head_ref = 'project-typeenv-head:' || project_id),
				head_revision INTEGER NOT NULL CHECK(head_revision > 0),
				selected_composite_ref TEXT NOT NULL CHECK(
					length(selected_composite_ref) = 79
					AND substr(selected_composite_ref, 1, 15) = 'typeenv:sha256:'
				),
				state_digest TEXT NOT NULL CHECK(
					length(state_digest) = 71
					AND substr(state_digest, 1, 7) = 'sha256:'
				),
				canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
				PRIMARY KEY(project_id, head_revision),
				UNIQUE(state_digest),
				UNIQUE(
					project_id,
					head_ref,
					head_revision,
					selected_composite_ref,
					state_digest,
					canonical_bytes
				)
			) WITHOUT ROWID`,
			`CREATE TABLE IF NOT EXISTS project_typeenv_heads (
				project_id TEXT PRIMARY KEY
					CHECK(project_id != '' AND trim(project_id) = project_id),
				head_ref TEXT NOT NULL UNIQUE
					CHECK(head_ref = 'project-typeenv-head:' || project_id),
				head_revision INTEGER NOT NULL CHECK(head_revision > 0),
				selected_composite_ref TEXT NOT NULL CHECK(
					length(selected_composite_ref) = 79
					AND substr(selected_composite_ref, 1, 15) = 'typeenv:sha256:'
				),
				state_digest TEXT NOT NULL UNIQUE CHECK(
					length(state_digest) = 71
					AND substr(state_digest, 1, 7) = 'sha256:'
				),
				canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
				FOREIGN KEY (
					project_id,
					head_ref,
					head_revision,
					selected_composite_ref,
					state_digest,
					canonical_bytes
				) REFERENCES project_typeenv_head_states (
					project_id,
					head_ref,
					head_revision,
					selected_composite_ref,
					state_digest,
					canonical_bytes
				) DEFERRABLE INITIALLY DEFERRED
			) WITHOUT ROWID`,
			`CREATE INDEX IF NOT EXISTS project_typeenv_head_states_by_head
				ON project_typeenv_head_states(
					project_id,
					head_ref,
					head_revision
				)`,
			`CREATE TRIGGER IF NOT EXISTS project_typeenv_heads_no_replace
				BEFORE INSERT ON project_typeenv_heads
				WHEN EXISTS (
					SELECT 1
					FROM project_typeenv_heads existing
					WHERE existing.project_id = NEW.project_id
						OR existing.head_ref = NEW.head_ref
				)
				BEGIN
					SELECT RAISE(
						ABORT,
						'project TypeEnv current head cannot be replaced'
					);
				END`,
			`CREATE TRIGGER IF NOT EXISTS project_typeenv_heads_genesis_only
				BEFORE INSERT ON project_typeenv_heads
				WHEN NEW.head_revision != 1
				BEGIN
					SELECT RAISE(
						ABORT,
						'project TypeEnv head must begin at HeadRevision 1'
					);
				END`,
			`CREATE TRIGGER IF NOT EXISTS project_typeenv_heads_revision_cas
				BEFORE UPDATE ON project_typeenv_heads
				WHEN NEW.project_id != OLD.project_id
					OR NEW.head_ref != OLD.head_ref
					OR NEW.head_revision != OLD.head_revision + 1
				BEGIN
					SELECT RAISE(
						ABORT,
						'project TypeEnv head update is not an exact successor'
					);
				END`,
			`CREATE TRIGGER IF NOT EXISTS project_typeenv_heads_state_on_insert
				AFTER INSERT ON project_typeenv_heads
				BEGIN
					INSERT INTO project_typeenv_head_states (
						project_id,
						head_ref,
						head_revision,
						selected_composite_ref,
						state_digest,
						canonical_bytes
					) VALUES (
						NEW.project_id,
						NEW.head_ref,
						NEW.head_revision,
						NEW.selected_composite_ref,
						NEW.state_digest,
						NEW.canonical_bytes
					);
				END`,
			`CREATE TRIGGER IF NOT EXISTS project_typeenv_heads_state_on_update
				AFTER UPDATE ON project_typeenv_heads
				BEGIN
					INSERT INTO project_typeenv_head_states (
						project_id,
						head_ref,
						head_revision,
						selected_composite_ref,
						state_digest,
						canonical_bytes
					) VALUES (
						NEW.project_id,
						NEW.head_ref,
						NEW.head_revision,
						NEW.selected_composite_ref,
						NEW.state_digest,
						NEW.canonical_bytes
					);
				END`,
			immutableHeadStoreTrigger(
				"project_typeenv_head_states",
				"update",
			),
			immutableHeadStoreTrigger(
				"project_typeenv_head_states",
				"delete",
			),
			immutableHeadStoreTrigger(
				"project_typeenv_heads",
				"delete",
			),
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
		return fmt.Errorf("begin project TypeEnv head schema transaction: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()
	_, err = transaction.ExecContext(ctx, createSchemaVersionTable)
	if err != nil {
		return fmt.Errorf("create project TypeEnv head schema version table: %w", err)
	}
	current, err := readSchemaVersion(ctx, transaction)
	if err != nil {
		return err
	}
	if current == 0 {
		if err := verifyNoUnversionedHeadFootprint(ctx, transaction); err != nil {
			return err
		}
	}
	if current > CurrentSchemaVersion {
		return fmt.Errorf(
			"project TypeEnv head schema version %d is newer than supported version %d",
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
				"project TypeEnv head schema migration gap from %d to %d",
				current,
				migration.version,
			)
		}
		for _, statement := range migration.statements {
			_, err = transaction.ExecContext(ctx, statement)
			if err != nil {
				return fmt.Errorf(
					"apply project TypeEnv head schema migration %d: %w",
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
			"project TypeEnv head schema version is %d; want %d",
			current,
			CurrentSchemaVersion,
		)
	}
	if err := verifySchemaFootprint(ctx, transaction); err != nil {
		return err
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit project TypeEnv head schema transaction: %w", err)
	}
	return nil
}

type schemaObject struct {
	objectType string
	name       string
}

var headSchemaObjects = []schemaObject{
	{objectType: "table", name: "project_typeenv_head_states"},
	{objectType: "table", name: "project_typeenv_heads"},
	{objectType: "index", name: "project_typeenv_head_states_by_head"},
	{objectType: "trigger", name: "project_typeenv_heads_no_replace"},
	{objectType: "trigger", name: "project_typeenv_heads_genesis_only"},
	{objectType: "trigger", name: "project_typeenv_heads_revision_cas"},
	{objectType: "trigger", name: "project_typeenv_heads_state_on_insert"},
	{objectType: "trigger", name: "project_typeenv_heads_state_on_update"},
	{objectType: "trigger", name: "project_typeenv_head_states_no_update"},
	{objectType: "trigger", name: "project_typeenv_head_states_no_delete"},
	{objectType: "trigger", name: "project_typeenv_heads_no_delete"},
}

func verifyNoUnversionedHeadFootprint(
	ctx context.Context,
	transaction *sql.Tx,
) error {
	for _, object := range headSchemaObjects {
		count, err := schemaObjectCount(ctx, transaction, object)
		if err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf(
				"unversioned project TypeEnv head schema contains %s %q",
				object.objectType,
				object.name,
			)
		}
	}
	return nil
}

func verifySchemaFootprint(
	ctx context.Context,
	transaction *sql.Tx,
) error {
	for _, object := range headSchemaObjects {
		count, err := schemaObjectCount(ctx, transaction, object)
		if err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf(
				"project TypeEnv head schema requires exactly one %s %q; found %d",
				object.objectType,
				object.name,
				count,
			)
		}
	}
	return nil
}

func schemaObjectCount(
	ctx context.Context,
	transaction *sql.Tx,
	object schemaObject,
) (int, error) {
	var count int
	err := transaction.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = ? AND name = ?`,
		object.objectType,
		object.name,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf(
			"inspect project TypeEnv head schema %s %q: %w",
			object.objectType,
			object.name,
			err,
		)
	}
	return count, nil
}

func readSchemaVersion(ctx context.Context, transaction *sql.Tx) (int, error) {
	var version int
	err := transaction.QueryRowContext(
		ctx,
		`SELECT version
		FROM project_typeenv_head_store_schema
		WHERE singleton = 1`,
	).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read project TypeEnv head schema version: %w", err)
	}
	if version <= 0 {
		return 0, fmt.Errorf(
			"project TypeEnv head schema version %d is invalid",
			version,
		)
	}
	return version, nil
}

func writeSchemaVersion(
	ctx context.Context,
	transaction *sql.Tx,
	version int,
) error {
	_, err := transaction.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_head_store_schema (
			singleton,
			version
		) VALUES (1, ?)
		ON CONFLICT(singleton) DO UPDATE SET version = excluded.version`,
		version,
	)
	if err != nil {
		return fmt.Errorf(
			"write project TypeEnv head schema version %d: %w",
			version,
			err,
		)
	}
	return nil
}

func immutableHeadStoreTrigger(table string, operation string) string {
	return `CREATE TRIGGER IF NOT EXISTS ` + table + `_no_` + operation + `
		BEFORE ` + operation + ` ON ` + table + `
		BEGIN
			SELECT RAISE(ABORT, 'project TypeEnv head states are immutable');
		END`
}
