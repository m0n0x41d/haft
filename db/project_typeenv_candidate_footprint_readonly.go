package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// RequireCurrentProjectTypeEnvCandidateFootprintReadOnly verifies the exact
// current kernel edition and the migration-owned candidate-store DDL needed to
// persist one project-TypeEnv B/E/X/C closure and Stage. It never repairs,
// creates, or migrates storage.
func RequireCurrentProjectTypeEnvCandidateFootprintReadOnly(
	ctx context.Context,
	database *sql.DB,
) error {
	if ctx == nil {
		return fmt.Errorf(
			"verify current project-TypeEnv candidate footprint: context is required",
		)
	}
	if database == nil {
		return fmt.Errorf(
			"verify current project-TypeEnv candidate footprint: database is required",
		)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf(
			"verify current project-TypeEnv candidate footprint: %w",
			err,
		)
	}
	if err := RequireCurrentSchemaReadOnly(ctx, database); err != nil {
		return err
	}

	expected, err := currentProjectTypeEnvPreparationObjects47()
	if err != nil {
		return err
	}
	reader := &contextualReadOnlyMigrationTransaction{
		ctx:      ctx,
		database: database,
	}
	if err := requireExactSQLiteObjects47(
		reader,
		expected,
		"current project-TypeEnv candidate footprint",
	); err != nil {
		return err
	}
	if err := requireExactCandidateSchemaVersion47(
		reader,
		"project_typeenv_artifact_store_schema",
		2,
	); err != nil {
		return err
	}
	if err := requireExactCandidateSchemaVersion47(
		reader,
		"project_typeenv_stage_store_schema",
		2,
	); err != nil {
		return err
	}
	return requireNoUnknownPreparationTriggers47(
		reader,
		expected,
		currentProjectTypeEnvPreparationTouchedTables47(),
	)
}

func currentProjectTypeEnvPreparationObjects47() (
	map[string]sqliteSchemaObject47,
	error,
) {
	expected := make(map[string]sqliteSchemaObject47)
	mergeSchemaObjects47(
		expected,
		projectTypeEnvArtifactCandidateObjects47(),
	)
	mergeSchemaObjects47(
		expected,
		projectTypeEnvStageCandidateObjects47(),
	)
	mergeSchemaObjects47(
		expected,
		schemaObjectsFromStatements47(
			projectTypeEnvCandidateAnnexTriggers47(),
		),
	)

	source, err := exactTypedMemorySourceObjects47()
	if err != nil {
		return nil, err
	}
	if err := mergeRequiredSQLiteObjects47(
		expected,
		source,
		[]string{
			"table/typed_memory_type_env_snapshots",
			"trigger/typed_memory_type_env_snapshots_no_replace",
			"trigger/typed_memory_type_env_snapshots_no_update",
			"trigger/typed_memory_type_env_snapshots_no_delete",
		},
		"typed-memory v46 TypeEnv snapshot source",
	); err != nil {
		return nil, err
	}

	coordinates, err := typedMemoryTypeEnvCoordinateObjects47()
	if err != nil {
		return nil, err
	}
	if err := mergeRequiredSQLiteObjects47(
		expected,
		coordinates,
		[]string{
			"table/typed_memory_type_env_coordinates",
			"table/typed_memory_graph_heads",
			"trigger/typed_memory_type_env_coordinates_v47_no_insert",
			"trigger/typed_memory_type_env_coordinates_no_update",
			"trigger/typed_memory_type_env_coordinates_no_delete",
			"trigger/typed_memory_type_env_snapshots_v47_register_coordinate",
			"trigger/project_typeenv_executable_snapshots_v47_register_coordinate",
			"trigger/typed_memory_graph_heads_no_replace",
			"trigger/typed_memory_graph_heads_genesis_only",
			"trigger/typed_memory_graph_heads_revision_cas",
			"trigger/typed_memory_graph_heads_no_delete",
		},
		"typed-memory v47 TypeEnv coordinate source",
	); err != nil {
		return nil, err
	}
	return expected, nil
}

func mergeRequiredSQLiteObjects47(
	target map[string]sqliteSchemaObject47,
	source map[string]sqliteSchemaObject47,
	keys []string,
	label string,
) error {
	for _, key := range keys {
		object, ok := source[key]
		if !ok {
			return fmt.Errorf(
				"%s lacks required schema object %s",
				label,
				key,
			)
		}
		target[key] = object
	}
	return nil
}

func currentProjectTypeEnvPreparationTouchedTables47() []string {
	return []string{
		"typed_memory_type_env_snapshots",
		"typed_memory_type_env_coordinates",
		"typed_memory_graph_heads",
		"project_typeenv_artifacts",
		"project_typeenv_runtime_mechanisms",
		"project_typeenv_registration_policies",
		"project_typeenv_composite_verifications",
		"project_typeenv_executable_snapshots",
		"project_typeenv_stages",
	}
}

func requireNoUnknownPreparationTriggers47(
	tx MigrationTransaction,
	expected map[string]sqliteSchemaObject47,
	tables []string,
) error {
	placeholders := make([]string, len(tables))
	args := make([]any, len(tables))
	for index, table := range tables {
		placeholders[index] = "?"
		args[index] = table
	}
	rows, err := tx.Query(
		`SELECT name, tbl_name
		 FROM sqlite_schema
		 WHERE type = 'trigger'
		   AND tbl_name IN (`+strings.Join(placeholders, ", ")+`)
		 ORDER BY tbl_name, name`,
		args...,
	)
	if err != nil {
		return fmt.Errorf(
			"inspect current project-TypeEnv preparation trigger set: %w",
			err,
		)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var table string
		if err := rows.Scan(&name, &table); err != nil {
			return fmt.Errorf(
				"decode current project-TypeEnv preparation trigger: %w",
				err,
			)
		}
		key := sqliteSchemaObjectKey47("trigger", name)
		if _, ok := expected[key]; !ok {
			return fmt.Errorf(
				"current project-TypeEnv candidate footprint contains unknown trigger %s on %s",
				name,
				table,
			)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf(
			"inspect current project-TypeEnv preparation triggers: %w",
			err,
		)
	}
	return nil
}

// contextualReadOnlyMigrationTransaction adapts the migration-owned exact-DDL
// verifier to a caller context while making writes inexpressible at this
// boundary.
type contextualReadOnlyMigrationTransaction struct {
	ctx      context.Context
	database *sql.DB
}

func (transaction *contextualReadOnlyMigrationTransaction) Exec(
	string,
	...any,
) (sql.Result, error) {
	return nil, fmt.Errorf(
		"current project-TypeEnv candidate footprint is read-only",
	)
}

func (transaction *contextualReadOnlyMigrationTransaction) Query(
	query string,
	args ...any,
) (*sql.Rows, error) {
	return transaction.database.QueryContext(
		transaction.ctx,
		query,
		args...,
	)
}

func (transaction *contextualReadOnlyMigrationTransaction) QueryRow(
	query string,
	args ...any,
) *sql.Row {
	return transaction.database.QueryRowContext(
		transaction.ctx,
		query,
		args...,
	)
}
