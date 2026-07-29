package db

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

const (
	projectTypeEnvHeadSelectionSchemaVersion47 = 47

	candidateFootprintAbsent47 candidateFootprintPosture47 = iota
	candidateFootprintExactV247
)

var projectTypeEnvHeadSelectionMigration47 = Migration{
	Version:            projectTypeEnvHeadSelectionSchemaVersion47,
	Description:        "Add atomic ProjectTypeEnvHead selection and TypeEnv activation closure",
	Apply:              applyProjectTypeEnvHeadSelectionMigration47,
	ApplyBoundary:      ForeignKeyTableRebuildBoundary,
	ForeignKeyVerifier: verifyForeignKeys,
}

type candidateFootprintPosture47 uint8

type sqliteSchemaObject47 struct {
	kind string
	name string
	sql  string
}

type projectTypeEnvCandidatePostures47 struct {
	artifact candidateFootprintPosture47
	stage    candidateFootprintPosture47
	head     candidateFootprintPosture47
}

func applyProjectTypeEnvHeadSelectionMigration47(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireExactProjectTypeEnvHeadSelectionSource47(tx); err != nil {
		return err
	}
	postures, err := inspectProjectTypeEnvCandidatePostures47(tx)
	if err != nil {
		return err
	}
	if err := requireAbsentProjectTypeEnvEffectFootprint47(tx); err != nil {
		return err
	}
	statements, err := projectTypeEnvHeadSelectionStatements47(postures)
	if err != nil {
		return err
	}
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install ProjectTypeEnv head-selection storage: %w", err)
	}
	if err := verifyProjectTypeEnvHeadSelectionFootprint47(tx); err != nil {
		return err
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify ProjectTypeEnv head-selection storage: %w", err)
	}
	return nil
}

func requireExactProjectTypeEnvHeadSelectionSource47(
	tx MigrationTransaction,
) error {
	var maximumVersion int
	err := tx.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(
		&maximumVersion,
	)
	if err != nil {
		return fmt.Errorf("inspect ProjectTypeEnv migration source version: %w", err)
	}
	if maximumVersion != 46 {
		return fmt.Errorf(
			"ProjectTypeEnv head-selection storage requires exact schema version 46; found %d",
			maximumVersion,
		)
	}
	var sourceVersionCount int
	err = tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 46",
	).Scan(&sourceVersionCount)
	if err != nil {
		return fmt.Errorf("inspect ProjectTypeEnv migration source marker: %w", err)
	}
	if sourceVersionCount != 1 {
		return fmt.Errorf(
			"ProjectTypeEnv head-selection storage requires schema version 46",
		)
	}
	expected, err := exactTypedMemorySourceObjects47()
	if err != nil {
		return err
	}
	if err := requireExactSQLiteObjects47(
		tx,
		expected,
		"typed-memory v46 source footprint",
	); err != nil {
		return err
	}
	if err := requireNoUnknownTypedMemoryObjects47(tx, expected); err != nil {
		return err
	}
	if err := requireExactTypedMemoryCapability47(tx); err != nil {
		return err
	}
	if err := requireColumnAbsent47(
		tx,
		"typed_memory_commit_materialization_closures",
		"type_env_activation_count",
	); err != nil {
		return err
	}
	return verifyForeignKeys(tx)
}

func exactTypedMemorySourceObjects47() (
	map[string]sqliteSchemaObject47,
	error,
) {
	objects := make(map[string]sqliteSchemaObject47)
	statementGroups := [][]string{
		typedMemoryStorageStatements45(),
		typedMemoryStorageStatements46(),
	}
	for _, statements := range statementGroups {
		for _, statement := range statements {
			if err := applyExpectedSchemaStatement47(objects, statement); err != nil {
				return nil, err
			}
		}
	}
	return objects, nil
}

func applyExpectedSchemaStatement47(
	objects map[string]sqliteSchemaObject47,
	statement string,
) error {
	trimmed := strings.TrimSpace(statement)
	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return nil
	}
	if fields[0] == "DROP" {
		if len(fields) != 3 {
			return fmt.Errorf("unsupported typed-memory source DROP statement")
		}
		kind := strings.ToLower(fields[1])
		name := trimSQLiteObjectName47(fields[2])
		delete(objects, sqliteSchemaObjectKey47(kind, name))
		return nil
	}
	if fields[0] != "CREATE" {
		return nil
	}
	kindIndex := 1
	nameIndex := 2
	if fields[1] == "UNIQUE" {
		kindIndex = 2
		nameIndex = 3
	}
	if len(fields) <= nameIndex {
		return fmt.Errorf("unsupported typed-memory source CREATE statement")
	}
	kind := strings.ToLower(fields[kindIndex])
	if kind != "table" && kind != "index" &&
		kind != "view" && kind != "trigger" {
		return nil
	}
	name := trimSQLiteObjectName47(fields[nameIndex])
	object := sqliteSchemaObject47{
		kind: kind,
		name: name,
		sql:  normalizeSQLiteDDL47(trimmed),
	}
	objects[sqliteSchemaObjectKey47(kind, name)] = object
	return nil
}

func trimSQLiteObjectName47(raw string) string {
	return strings.Trim(strings.TrimSuffix(raw, ";"), "`\"[ ]")
}

func sqliteSchemaObjectKey47(kind string, name string) string {
	return kind + "/" + name
}

func normalizeSQLiteDDL47(raw string) string {
	normalized := strings.Join(strings.Fields(raw), " ")
	normalized = strings.ReplaceAll(normalized, " IF NOT EXISTS ", " ")
	return strings.TrimSuffix(normalized, ";")
}

func requireExactSQLiteObjects47(
	tx MigrationTransaction,
	expected map[string]sqliteSchemaObject47,
	label string,
) error {
	keys := make([]string, 0, len(expected))
	for key := range expected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		object := expected[key]
		var actualSQL string
		err := tx.QueryRow(
			"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
			object.kind,
			object.name,
		).Scan(&actualSQL)
		if err != nil {
			return fmt.Errorf(
				"%s requires exact %s %s: %w",
				label,
				object.kind,
				object.name,
				err,
			)
		}
		if normalizeSQLiteDDL47(actualSQL) != object.sql {
			return fmt.Errorf(
				"%s drifted at %s %s",
				label,
				object.kind,
				object.name,
			)
		}
	}
	return nil
}

func requireNoUnknownTypedMemoryObjects47(
	tx MigrationTransaction,
	expected map[string]sqliteSchemaObject47,
) error {
	rows, err := tx.Query(
		`SELECT type, name
		FROM sqlite_master
		WHERE (name LIKE 'typed_memory_%'
				OR name LIKE 'idx_typed_memory_%')
			AND type IN ('table', 'index', 'view', 'trigger')
		ORDER BY type, name`,
	)
	if err != nil {
		return fmt.Errorf("inspect typed-memory v46 object names: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var name string
		if err := rows.Scan(&kind, &name); err != nil {
			return fmt.Errorf("scan typed-memory v46 object name: %w", err)
		}
		if _, ok := expected[sqliteSchemaObjectKey47(kind, name)]; !ok {
			return fmt.Errorf(
				"typed-memory v46 source footprint contains unknown %s %s",
				kind,
				name,
			)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate typed-memory v46 object names: %w", err)
	}
	return nil
}

func requireExactTypedMemoryCapability47(tx MigrationTransaction) error {
	var generation int
	var digest string
	var canonical []byte
	err := tx.QueryRow(
		`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_storage_capabilities
		WHERE capability_key = ?`,
		typedMemoryWriterGenerationCapability46,
	).Scan(&generation, &digest, &canonical)
	if err != nil {
		return fmt.Errorf("read exact v46 typed-memory capability: %w", err)
	}
	if generation != typedMemoryWriterGeneration46 ||
		digest != typedMemoryWriterMarkerDigest46 ||
		string(canonical) != typedMemoryWriterMarkerBytes46 {
		return fmt.Errorf("typed-memory v46 capability marker drifted")
	}
	var invalidGenerationCount int
	err = tx.QueryRow(
		`SELECT COUNT(*)
		FROM typed_memory_event_writer_generations
		WHERE writer_generation NOT IN (45, 46)
			OR provenance_kind NOT IN ('migration_v45_backfill', 'writer_v46')`,
	).Scan(&invalidGenerationCount)
	if err != nil {
		return fmt.Errorf("inspect v46 writer generations: %w", err)
	}
	if invalidGenerationCount != 0 {
		return fmt.Errorf("typed-memory v46 writer-generation history drifted")
	}
	return nil
}

func requireColumnAbsent47(
	tx MigrationTransaction,
	table string,
	column string,
) error {
	rows, err := tx.Query("PRAGMA table_xinfo(" + quoteSQLiteIdentifier(table) + ")")
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		var hidden int
		if err := rows.Scan(
			&cid,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
			&hidden,
		); err != nil {
			return fmt.Errorf("scan %s columns: %w", table, err)
		}
		if name == column {
			return fmt.Errorf(
				"unknown partial v47 footprint: %s.%s already exists",
				table,
				column,
			)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s columns: %w", table, err)
	}
	return nil
}

func inspectProjectTypeEnvCandidatePostures47(
	tx MigrationTransaction,
) (projectTypeEnvCandidatePostures47, error) {
	artifact, err := inspectCandidateFootprint47(
		tx,
		projectTypeEnvArtifactCandidateObjects47(),
		"project TypeEnv artifact candidate store",
		"project_typeenv_artifact_store_schema",
		2,
	)
	if err != nil {
		return projectTypeEnvCandidatePostures47{}, err
	}
	stage, err := inspectCandidateFootprint47(
		tx,
		projectTypeEnvStageCandidateObjects47(),
		"project TypeEnv Stage candidate store",
		"project_typeenv_stage_store_schema",
		2,
	)
	if err != nil {
		return projectTypeEnvCandidatePostures47{}, err
	}
	head, err := inspectCandidateFootprint47(
		tx,
		projectTypeEnvHeadStoreObjects47(),
		"project TypeEnv head store",
		"project_typeenv_head_store_schema",
		1,
	)
	if err != nil {
		return projectTypeEnvCandidatePostures47{}, err
	}
	if head == candidateFootprintExactV247 {
		if err := requireEmptyPreReleaseHeadStore47(tx); err != nil {
			return projectTypeEnvCandidatePostures47{}, err
		}
	}
	known := make(map[string]sqliteSchemaObject47)
	mergeSchemaObjects47(known, projectTypeEnvArtifactCandidateObjects47())
	mergeSchemaObjects47(known, projectTypeEnvStageCandidateObjects47())
	mergeSchemaObjects47(known, projectTypeEnvHeadStoreObjects47())
	if err := requireNoUnknownProjectTypeEnvObjects47(tx, known); err != nil {
		return projectTypeEnvCandidatePostures47{}, err
	}
	return projectTypeEnvCandidatePostures47{
		artifact: artifact,
		stage:    stage,
		head:     head,
	}, nil
}

func inspectCandidateFootprint47(
	tx MigrationTransaction,
	expected map[string]sqliteSchemaObject47,
	label string,
	versionTable string,
	expectedVersion int,
) (candidateFootprintPosture47, error) {
	present := 0
	for _, object := range expected {
		var count int
		err := tx.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
			object.kind,
			object.name,
		).Scan(&count)
		if err != nil {
			return candidateFootprintAbsent47, fmt.Errorf(
				"inspect %s %s %s: %w",
				label,
				object.kind,
				object.name,
				err,
			)
		}
		present += count
	}
	if present == 0 {
		return candidateFootprintAbsent47, nil
	}
	if present != len(expected) {
		return candidateFootprintAbsent47, fmt.Errorf(
			"%s has unknown partial footprint: found %d of %d objects",
			label,
			present,
			len(expected),
		)
	}
	if err := requireExactSQLiteObjects47(tx, expected, label); err != nil {
		return candidateFootprintAbsent47, err
	}
	var version int
	err := tx.QueryRow(
		"SELECT version FROM " + quoteSQLiteIdentifier(versionTable) + " WHERE singleton = 1",
	).Scan(&version)
	if err != nil {
		return candidateFootprintAbsent47, fmt.Errorf(
			"read %s schema version: %w",
			label,
			err,
		)
	}
	if version != expectedVersion {
		return candidateFootprintAbsent47, fmt.Errorf(
			"%s schema version is %d; exact live version %d is required",
			label,
			version,
			expectedVersion,
		)
	}
	var versionRows int
	err = tx.QueryRow(
		"SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(versionTable),
	).Scan(&versionRows)
	if err != nil {
		return candidateFootprintAbsent47, fmt.Errorf(
			"count %s schema versions: %w",
			label,
			err,
		)
	}
	if versionRows != 1 {
		return candidateFootprintAbsent47, fmt.Errorf(
			"%s schema-version footprint is not exact",
			label,
		)
	}
	return candidateFootprintExactV247, nil
}

func mergeSchemaObjects47(
	target map[string]sqliteSchemaObject47,
	source map[string]sqliteSchemaObject47,
) {
	for key, object := range source {
		target[key] = object
	}
}

func requireEmptyPreReleaseHeadStore47(tx MigrationTransaction) error {
	for _, table := range []string{
		"project_typeenv_head_states",
		"project_typeenv_heads",
	} {
		var count int
		err := tx.QueryRow(
			"SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table),
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("inspect pre-release %s: %w", table, err)
		}
		if count != 0 {
			return fmt.Errorf(
				"non-empty pre-release ProjectTypeEnv head store requires explicit cache rebuild; migration 47 will not fabricate effect history",
			)
		}
	}
	return nil
}

func requireNoUnknownProjectTypeEnvObjects47(
	tx MigrationTransaction,
	known map[string]sqliteSchemaObject47,
) error {
	rows, err := tx.Query(
		`SELECT type, name
		FROM sqlite_master
		WHERE (name LIKE 'project_typeenv_%'
				OR name LIKE 'idx_project_typeenv_%')
			AND type IN ('table', 'index', 'view', 'trigger')
		ORDER BY type, name`,
	)
	if err != nil {
		return fmt.Errorf("inspect ProjectTypeEnv pre-v47 object names: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var name string
		if err := rows.Scan(&kind, &name); err != nil {
			return fmt.Errorf("scan ProjectTypeEnv pre-v47 object name: %w", err)
		}
		if _, ok := known[sqliteSchemaObjectKey47(kind, name)]; !ok {
			return fmt.Errorf(
				"unknown partial v47 footprint contains %s %s",
				kind,
				name,
			)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate ProjectTypeEnv pre-v47 object names: %w", err)
	}
	return nil
}

func requireAbsentProjectTypeEnvEffectFootprint47(
	tx MigrationTransaction,
) error {
	for _, object := range projectTypeEnvEffectObjects47() {
		var count int
		err := tx.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
			object.kind,
			object.name,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf(
				"inspect v47 %s %s: %w",
				object.kind,
				object.name,
				err,
			)
		}
		if count != 0 {
			return fmt.Errorf(
				"unknown partial v47 footprint already contains %s %s",
				object.kind,
				object.name,
			)
		}
	}
	return nil
}

func projectTypeEnvHeadSelectionStatements47(
	postures projectTypeEnvCandidatePostures47,
) ([]string, error) {
	statements := make([]string, 0, 128)
	if postures.artifact == candidateFootprintAbsent47 {
		statements = append(statements, projectTypeEnvArtifactCandidateInstall47()...)
	}
	if postures.stage == candidateFootprintAbsent47 {
		statements = append(statements, projectTypeEnvStageCandidateInstall47()...)
	}
	if postures.head == candidateFootprintAbsent47 {
		statements = append(statements, projectTypeEnvHeadStoreInstall47()...)
	}
	statements = append(statements, projectTypeEnvCandidateAnnexTriggers47()...)
	coordinateStatements, err := typedMemoryTypeEnvCoordinateStatements47()
	if err != nil {
		return nil, err
	}
	statements = append(statements, coordinateStatements...)
	statements = append(statements, projectTypeEnvEffectTableStatements47()...)
	statements = append(statements, projectTypeEnvEffectIndexStatements47()...)
	statements = append(
		statements,
		"DROP TRIGGER typed_memory_commit_materialization_closures_v46_exact_footprint",
		"DROP VIEW typed_memory_event_materialization_footprints_v46",
		`ALTER TABLE typed_memory_commit_materialization_closures
			ADD COLUMN type_env_activation_count INTEGER NOT NULL DEFAULT 0
			CHECK(type_env_activation_count >= 0)`,
	)
	footprintView, err := typedMemoryEventMaterializationFootprintsView47()
	if err != nil {
		return nil, err
	}
	statements = append(statements, footprintView)
	closureTrigger, err := typedMemoryCommitClosureExactFootprintTrigger47()
	if err != nil {
		return nil, err
	}
	statements = append(statements, closureTrigger)
	statements = append(statements, projectTypeEnvEffectTriggerStatements47()...)
	return statements, nil
}

func projectTypeEnvArtifactCandidateInstall47() []string {
	return []string{
		`CREATE TABLE project_typeenv_artifact_store_schema (
			singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
			version INTEGER NOT NULL CHECK (version > 0)
		)`,
		`CREATE TABLE project_typeenv_artifacts (
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
		`CREATE INDEX project_typeenv_artifacts_ref
			ON project_typeenv_artifacts(artifact_ref, artifact_kind)`,
		`CREATE TABLE project_typeenv_runtime_mechanisms (
			artifact_ref TEXT NOT NULL,
			edition TEXT NOT NULL,
			artifact_digest TEXT NOT NULL,
			canonical_schema_version TEXT NOT NULL,
			canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0),
			PRIMARY KEY (artifact_ref, edition)
		)`,
		`CREATE INDEX project_typeenv_runtime_mechanisms_digest
			ON project_typeenv_runtime_mechanisms(artifact_digest)`,
		`CREATE TABLE project_typeenv_registration_policies (
			registration_ref TEXT PRIMARY KEY,
			artifact_digest TEXT NOT NULL UNIQUE,
			canonical_schema_version TEXT NOT NULL,
			canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0)
		)`,
		`CREATE INDEX project_typeenv_registration_policies_digest
			ON project_typeenv_registration_policies(artifact_digest)`,
		`INSERT INTO project_typeenv_artifact_store_schema (singleton, version)
			VALUES (1, 2)`,
	}
}

func projectTypeEnvStageCandidateInstall47() []string {
	return []string{
		`CREATE TABLE project_typeenv_stage_store_schema (
			singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
			version INTEGER NOT NULL CHECK (version > 0)
		)`,
		`CREATE TABLE project_typeenv_composite_verifications (
			verification_ref TEXT PRIMARY KEY,
			verification_digest TEXT NOT NULL UNIQUE,
			lowerer_schema_version TEXT NOT NULL,
			canonical_schema_version TEXT NOT NULL,
			canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0)
		)`,
		`CREATE TABLE project_typeenv_stages (
			stage_ref TEXT PRIMARY KEY,
			stage_digest TEXT NOT NULL UNIQUE,
			project_id TEXT NOT NULL,
			composite_verification_ref TEXT NOT NULL,
			canonical_schema_version TEXT NOT NULL,
			canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0),
			FOREIGN KEY (composite_verification_ref)
				REFERENCES project_typeenv_composite_verifications(verification_ref)
		)`,
		`CREATE INDEX project_typeenv_stages_project
			ON project_typeenv_stages(project_id, stage_ref)`,
		`CREATE TABLE project_typeenv_executable_snapshots (
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
		`CREATE INDEX project_typeenv_stages_executable_snapshot
			ON project_typeenv_stages(executable_type_env_ref, stage_ref)`,
		`INSERT INTO project_typeenv_stage_store_schema (singleton, version)
			VALUES (1, 2)`,
	}
}

func typedMemoryTypeEnvCoordinateStatements47() ([]string, error) {
	graphHeads, err := typedMemoryGraphHeadsTable47(
		"typed_memory_graph_heads",
	)
	if err != nil {
		return nil, err
	}
	graphEvents, err := typedMemoryGraphEventsTable47(
		"typed_memory_graph_events",
	)
	if err != nil {
		return nil, err
	}
	admissionBases, err := typedMemoryEventAdmissionBasesTable47(
		"typed_memory_event_admission_bases",
	)
	if err != nil {
		return nil, err
	}
	dependencyDrops, dependencyCreates, err := typedMemoryCoordinateRebuildDependencies47()
	if err != nil {
		return nil, err
	}
	statements := []string{
		typedMemoryTypeEnvCoordinatesTable47(),
		`INSERT INTO typed_memory_type_env_coordinates (
			type_env_ref,
			representation_kind,
			generic_snapshot_ref,
			project_executable_ref
		)
		SELECT
			type_env_ref,
			'generic_snapshot',
			type_env_ref,
			NULL
		FROM typed_memory_type_env_snapshots`,
		`INSERT INTO typed_memory_type_env_coordinates (
			type_env_ref,
			representation_kind,
			generic_snapshot_ref,
			project_executable_ref
		)
		SELECT
			type_env_ref,
			'project_executable',
			NULL,
			type_env_ref
		FROM project_typeenv_executable_snapshots`,
		typedMemoryTypeEnvCoordinateNoReplaceTrigger47(),
		immutableTypedMemoryTrigger45(
			"typed_memory_type_env_coordinates",
			"update",
		),
		immutableTypedMemoryTrigger45(
			"typed_memory_type_env_coordinates",
			"delete",
		),
		typedMemoryGenericSnapshotCoordinateTrigger47(),
		typedMemoryProjectExecutableCoordinateTrigger47(),
	}
	statements = append(statements, dependencyDrops...)
	statements = append(statements,
		`CREATE TEMP TABLE typed_memory_graph_heads_v47_backup AS
		SELECT * FROM typed_memory_graph_heads`,
		`CREATE TEMP TABLE typed_memory_graph_events_v47_backup AS
		SELECT * FROM typed_memory_graph_events`,
		`CREATE TEMP TABLE typed_memory_event_admission_bases_v47_backup AS
		SELECT * FROM typed_memory_event_admission_bases`,
		"DROP TABLE typed_memory_event_admission_bases",
		"DROP TABLE typed_memory_graph_events",
		"DROP TABLE typed_memory_graph_heads",
		graphHeads,
		graphEvents,
		admissionBases,
		`INSERT INTO typed_memory_graph_heads (
			project_id,
			graph_revision,
			active_type_env_ref,
			last_event_ref,
			last_commit_ref,
			updated_at
		)
		SELECT
			project_id,
			graph_revision,
			active_type_env_ref,
			last_event_ref,
			last_commit_ref,
			updated_at
		FROM typed_memory_graph_heads_v47_backup`,
		`INSERT INTO typed_memory_graph_events (
			project_id,
			event_ref,
			commit_ref,
			event_digest,
			expected_revision,
			graph_revision,
			basis_type_env_ref,
			result_type_env_ref,
			change_set_digest,
			canonical_change_set_bytes,
			change_count,
			event_kind,
			authority_class,
			request_provenance_ref,
			recorded_at
		)
		SELECT
			project_id,
			event_ref,
			commit_ref,
			event_digest,
			expected_revision,
			graph_revision,
			basis_type_env_ref,
			result_type_env_ref,
			change_set_digest,
			canonical_change_set_bytes,
			change_count,
			event_kind,
			authority_class,
			request_provenance_ref,
			recorded_at
		FROM typed_memory_graph_events_v47_backup`,
		`INSERT INTO typed_memory_event_admission_bases (
			project_id,
			event_ref,
			event_digest,
			admission_basis_kind,
			type_env_ref,
			basis_graph_revision,
			request_digest,
			canonical_request_bytes,
			semantic_digest,
			canonical_semantic_bytes,
			admission_envelope_digest,
			canonical_admission_envelope_bytes,
			admission_basis_digest,
			canonical_admission_basis_bytes,
			materialization_manifest_digest,
			canonical_materialization_manifest_bytes,
			recorded_at
		)
		SELECT
			project_id,
			event_ref,
			event_digest,
			admission_basis_kind,
			type_env_ref,
			basis_graph_revision,
			request_digest,
			canonical_request_bytes,
			semantic_digest,
			canonical_semantic_bytes,
			admission_envelope_digest,
			canonical_admission_envelope_bytes,
			admission_basis_digest,
			canonical_admission_basis_bytes,
			materialization_manifest_digest,
			canonical_materialization_manifest_bytes,
			recorded_at
		FROM typed_memory_event_admission_bases_v47_backup`,
		"DROP TABLE typed_memory_graph_heads_v47_backup",
		"DROP TABLE typed_memory_graph_events_v47_backup",
		"DROP TABLE typed_memory_event_admission_bases_v47_backup",
	)
	statements = append(statements, dependencyCreates...)
	return statements, nil
}

func typedMemoryTypeEnvCoordinateObjects47() (
	map[string]sqliteSchemaObject47,
	error,
) {
	statements, err := typedMemoryTypeEnvCoordinateStatements47()
	if err != nil {
		return nil, err
	}
	return schemaObjectsFromStatements47(statements), nil
}

func typedMemoryTypeEnvCoordinatesTable47() string {
	return `CREATE TABLE typed_memory_type_env_coordinates (
		type_env_ref TEXT PRIMARY KEY CHECK(
			length(type_env_ref) = 79
			AND substr(type_env_ref, 1, 15) = 'typeenv:sha256:'
		),
		representation_kind TEXT NOT NULL CHECK(
			representation_kind IN ('generic_snapshot', 'project_executable')
		),
		generic_snapshot_ref TEXT UNIQUE,
		project_executable_ref TEXT UNIQUE,
		UNIQUE(type_env_ref, representation_kind),
		FOREIGN KEY(generic_snapshot_ref)
			REFERENCES typed_memory_type_env_snapshots(type_env_ref),
		FOREIGN KEY(project_executable_ref)
			REFERENCES project_typeenv_executable_snapshots(type_env_ref),
		CHECK(
			(representation_kind = 'generic_snapshot'
				AND generic_snapshot_ref = type_env_ref
				AND project_executable_ref IS NULL)
			OR
			(representation_kind = 'project_executable'
				AND generic_snapshot_ref IS NULL
				AND project_executable_ref = type_env_ref)
		)
	) WITHOUT ROWID`
}

func typedMemoryTypeEnvCoordinateNoReplaceTrigger47() string {
	return `CREATE TRIGGER typed_memory_type_env_coordinates_v47_no_insert
	BEFORE INSERT ON typed_memory_type_env_coordinates
	WHEN EXISTS (
		SELECT 1
		FROM typed_memory_type_env_coordinates existing
		WHERE existing.type_env_ref = NEW.type_env_ref
			OR (
				NEW.generic_snapshot_ref IS NOT NULL
				AND existing.generic_snapshot_ref = NEW.generic_snapshot_ref
			)
			OR (
				NEW.project_executable_ref IS NOT NULL
				AND existing.project_executable_ref = NEW.project_executable_ref
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory TypeEnv coordinate ownership is immutable');
	END`
}

func typedMemoryGenericSnapshotCoordinateTrigger47() string {
	return `CREATE TRIGGER typed_memory_type_env_snapshots_v47_register_coordinate
	AFTER INSERT ON typed_memory_type_env_snapshots
	BEGIN
		INSERT INTO typed_memory_type_env_coordinates (
			type_env_ref,
			representation_kind,
			generic_snapshot_ref,
			project_executable_ref
		) VALUES (
			NEW.type_env_ref,
			'generic_snapshot',
			NEW.type_env_ref,
			NULL
		);
	END`
}

func typedMemoryProjectExecutableCoordinateTrigger47() string {
	return `CREATE TRIGGER project_typeenv_executable_snapshots_v47_register_coordinate
	AFTER INSERT ON project_typeenv_executable_snapshots
	BEGIN
		INSERT INTO typed_memory_type_env_coordinates (
			type_env_ref,
			representation_kind,
			generic_snapshot_ref,
			project_executable_ref
		) VALUES (
			NEW.type_env_ref,
			'project_executable',
			NULL,
			NEW.type_env_ref
		);
	END`
}

func typedMemoryGraphHeadsTable47(table string) (string, error) {
	definition, err := replaceExactSQL47(
		typedMemoryGraphHeadsTable45(),
		"CREATE TABLE typed_memory_graph_heads",
		"CREATE TABLE "+table,
		1,
		"typed-memory graph-head rebuild table",
	)
	if err != nil {
		return "", err
	}
	return replaceExactSQL47(
		definition,
		"REFERENCES typed_memory_type_env_snapshots(type_env_ref)",
		"REFERENCES typed_memory_type_env_coordinates(type_env_ref)",
		1,
		"typed-memory graph-head TypeEnv owner",
	)
}

func typedMemoryGraphEventsTable47(table string) (string, error) {
	definition, err := replaceExactSQL47(
		typedMemoryGraphEventsTable45(),
		"CREATE TABLE typed_memory_graph_events",
		"CREATE TABLE "+table,
		1,
		"typed-memory graph-event rebuild table",
	)
	if err != nil {
		return "", err
	}
	return replaceExactSQL47(
		definition,
		"REFERENCES typed_memory_type_env_snapshots(type_env_ref)",
		"REFERENCES typed_memory_type_env_coordinates(type_env_ref)",
		2,
		"typed-memory graph-event TypeEnv owners",
	)
}

func typedMemoryEventAdmissionBasesTable47(table string) (string, error) {
	definition, err := replaceExactSQL47(
		typedMemoryEventAdmissionBasesTable46(),
		"CREATE TABLE typed_memory_event_admission_bases",
		"CREATE TABLE "+table,
		1,
		"typed-memory admission-basis rebuild table",
	)
	if err != nil {
		return "", err
	}
	return replaceExactSQL47(
		definition,
		"REFERENCES typed_memory_type_env_snapshots(type_env_ref)",
		"REFERENCES typed_memory_type_env_coordinates(type_env_ref)",
		1,
		"typed-memory admission-basis TypeEnv owner",
	)
}

func replaceExactSQL47(
	source string,
	old string,
	replacement string,
	expected int,
	label string,
) (string, error) {
	if actual := strings.Count(source, old); actual != expected {
		return "", fmt.Errorf(
			"%s source occurrence count = %d; want %d",
			label,
			actual,
			expected,
		)
	}
	return strings.ReplaceAll(source, old, replacement), nil
}

func typedMemoryCoordinateRebuildDependencies47() (
	[]string,
	[]string,
	error,
) {
	source, err := exactTypedMemorySourceObjects47()
	if err != nil {
		return nil, nil, err
	}
	objects := make([]sqliteSchemaObject47, 0)
	for _, object := range source {
		if object.kind != "index" &&
			object.kind != "view" &&
			object.kind != "trigger" {
			continue
		}
		if !typedMemoryCoordinateDependency47(object.sql) {
			continue
		}
		objects = append(objects, object)
	}
	sort.Slice(objects, func(left int, right int) bool {
		leftRank := typedMemoryCoordinateDependencyRank47(objects[left].kind)
		rightRank := typedMemoryCoordinateDependencyRank47(objects[right].kind)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return objects[left].name < objects[right].name
	})
	drops := make([]string, 0, len(objects))
	for index := len(objects) - 1; index >= 0; index-- {
		object := objects[index]
		drops = append(
			drops,
			"DROP "+strings.ToUpper(object.kind)+" "+
				quoteSQLiteIdentifier(object.name),
		)
	}
	creates := make([]string, 0, len(objects))
	for _, object := range objects {
		creates = append(creates, object.sql)
	}
	return drops, creates, nil
}

func typedMemoryCoordinateDependency47(sql string) bool {
	for _, table := range []string{
		"typed_memory_graph_heads",
		"typed_memory_graph_events",
		"typed_memory_event_admission_bases",
	} {
		if strings.Contains(sql, table) {
			return true
		}
	}
	return false
}

func typedMemoryCoordinateDependencyRank47(kind string) int {
	switch kind {
	case "index":
		return 1
	case "view":
		return 2
	case "trigger":
		return 3
	default:
		return 4
	}
}

func projectTypeEnvHeadStoreInstall47() []string {
	return []string{
		`CREATE TABLE project_typeenv_head_store_schema (
			singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
			version INTEGER NOT NULL CHECK (version > 0)
		)`,
		projectTypeEnvHeadStatesTable47(),
		projectTypeEnvCurrentHeadsTable47(),
		`CREATE INDEX project_typeenv_head_states_by_head
			ON project_typeenv_head_states(
				project_id,
				head_ref,
				head_revision
			)`,
		projectTypeEnvHeadsNoReplaceTrigger47(),
		projectTypeEnvHeadsGenesisOnlyTrigger47(),
		projectTypeEnvHeadsRevisionCASTrigger47(),
		projectTypeEnvHeadsStateOnInsertTrigger47(),
		projectTypeEnvHeadsStateOnUpdateTrigger47(),
		projectTypeEnvHeadStoreImmutableTrigger47(
			"project_typeenv_head_states",
			"update",
		),
		projectTypeEnvHeadStoreImmutableTrigger47(
			"project_typeenv_head_states",
			"delete",
		),
		projectTypeEnvHeadStoreImmutableTrigger47(
			"project_typeenv_heads",
			"delete",
		),
		`INSERT INTO project_typeenv_head_store_schema (singleton, version)
			VALUES (1, 1)`,
	}
}

func projectTypeEnvArtifactCandidateObjects47() map[string]sqliteSchemaObject47 {
	return schemaObjectsFromStatements47(projectTypeEnvArtifactCandidateInstall47())
}

func projectTypeEnvStageCandidateObjects47() map[string]sqliteSchemaObject47 {
	objects := schemaObjectsFromStatements47(projectTypeEnvStageCandidateInstall47())
	key := sqliteSchemaObjectKey47("table", "project_typeenv_stages")
	object := objects[key]
	object.sql = normalizeSQLiteDDL47(
		`CREATE TABLE project_typeenv_stages (
			stage_ref TEXT PRIMARY KEY,
			stage_digest TEXT NOT NULL UNIQUE,
			project_id TEXT NOT NULL,
			composite_verification_ref TEXT NOT NULL,
			canonical_schema_version TEXT NOT NULL,
			canonical_bytes BLOB NOT NULL CHECK (length(canonical_bytes) > 0),
			executable_type_env_ref TEXT
				REFERENCES project_typeenv_executable_snapshots(type_env_ref),
			FOREIGN KEY (composite_verification_ref)
				REFERENCES project_typeenv_composite_verifications(verification_ref)
		)`,
	)
	objects[key] = object
	return objects
}

func projectTypeEnvHeadStoreObjects47() map[string]sqliteSchemaObject47 {
	return schemaObjectsFromStatements47(projectTypeEnvHeadStoreInstall47())
}

func schemaObjectsFromStatements47(
	statements []string,
) map[string]sqliteSchemaObject47 {
	objects := make(map[string]sqliteSchemaObject47)
	for _, statement := range statements {
		_ = applyExpectedSchemaStatement47(objects, statement)
	}
	return objects
}

func projectTypeEnvHeadStatesTable47() string {
	return `CREATE TABLE project_typeenv_head_states (
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
	) WITHOUT ROWID`
}

func projectTypeEnvCurrentHeadsTable47() string {
	return `CREATE TABLE project_typeenv_heads (
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
	) WITHOUT ROWID`
}

func projectTypeEnvHeadsNoReplaceTrigger47() string {
	return `CREATE TRIGGER project_typeenv_heads_no_replace
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
		END`
}

func projectTypeEnvHeadsGenesisOnlyTrigger47() string {
	return `CREATE TRIGGER project_typeenv_heads_genesis_only
		BEFORE INSERT ON project_typeenv_heads
		WHEN NEW.head_revision != 1
		BEGIN
			SELECT RAISE(
				ABORT,
				'project TypeEnv head must begin at HeadRevision 1'
			);
		END`
}

func projectTypeEnvHeadsRevisionCASTrigger47() string {
	return `CREATE TRIGGER project_typeenv_heads_revision_cas
		BEFORE UPDATE ON project_typeenv_heads
		WHEN NEW.project_id != OLD.project_id
			OR NEW.head_ref != OLD.head_ref
			OR NEW.head_revision != OLD.head_revision + 1
		BEGIN
			SELECT RAISE(
				ABORT,
				'project TypeEnv head update is not an exact successor'
			);
		END`
}

func projectTypeEnvHeadsStateOnInsertTrigger47() string {
	return `CREATE TRIGGER project_typeenv_heads_state_on_insert
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
		END`
}

func projectTypeEnvHeadsStateOnUpdateTrigger47() string {
	return `CREATE TRIGGER project_typeenv_heads_state_on_update
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
		END`
}

func projectTypeEnvHeadStoreImmutableTrigger47(
	table string,
	operation string,
) string {
	return `CREATE TRIGGER ` + table + `_no_` + operation + `
		BEFORE ` + operation + ` ON ` + table + `
		BEGIN
			SELECT RAISE(ABORT, 'project TypeEnv head states are immutable');
		END`
}

var projectTypeEnvEffectTables47 = []string{
	"project_typeenv_head_selection_config_authority_bases",
	"project_typeenv_head_selection_mode_policies",
	"project_typeenv_no_prior_head_proofs",
	"project_typeenv_head_selection_requests",
	"project_typeenv_head_selection_authorization_contents",
	"project_typeenv_head_selection_trusted_cli_sources",
	"project_typeenv_head_selection_speech_act_records",
	"project_typeenv_head_selection_permissions_v3",
	"project_typeenv_head_selection_authority_resolution_bases",
	"project_typeenv_head_selection_authority_resolutions",
	"project_typeenv_head_selection_explicit_policy_acceptance_resolutions",
	"project_typeenv_head_selection_strict_permission_resolutions",
	"project_typeenv_head_selection_authority_uses",
	"project_typeenv_head_cas_work_records",
	"typed_memory_type_env_activations",
	"project_typeenv_head_history",
	"project_typeenv_head_effect_obligations",
	"project_typeenv_head_selection_receipts",
	"project_typeenv_head_selection_closures",
}

var projectTypeEnvEffectIndexes47 = []string{
	"idx_project_typeenv_requests_project_key_v47",
	"idx_project_typeenv_authority_uses_replay_v47",
	"idx_project_typeenv_activations_revision_v47",
	"idx_project_typeenv_head_history_graph_revision_v47",
}

var projectTypeEnvEffectSpecificTriggers47 = []string{
	"project_typeenv_head_selection_requests_v47_exact_predecessor",
	"project_typeenv_head_selection_mode_policies_v47_exact_config",
	"project_typeenv_head_selection_authorization_contents_v47_exact_request",
	"project_typeenv_head_selection_trusted_cli_sources_v47_exact_source",
	"project_typeenv_head_selection_speech_act_records_v47_exact_source",
	"project_typeenv_head_selection_permissions_v3_v47_exact_source",
	"project_typeenv_head_selection_authority_resolution_bases_v47_exact_source",
	"project_typeenv_head_selection_authority_resolutions_v47_exact_basis",
	"project_typeenv_head_selection_explicit_policy_acceptance_resolutions_v47_exact_base",
	"project_typeenv_head_selection_strict_permission_resolutions_v47_exact_base",
	"project_typeenv_head_selection_authority_uses_v47_exact_source",
	"project_typeenv_head_cas_work_records_v47_exact_use",
	"typed_memory_type_env_activations_v47_exact_effect",
	"project_typeenv_head_history_v47_exact_effect",
	"project_typeenv_head_states_v47_exact_composite_owner",
	"project_typeenv_heads_v47_exact_composite_owner_insert",
	"project_typeenv_heads_v47_exact_composite_owner_update",
	"project_typeenv_heads_v47_obligation_on_insert",
	"project_typeenv_heads_v47_obligation_on_update",
	"project_typeenv_head_selection_receipts_v47_exact_effect",
	"project_typeenv_head_selection_closures_v47_exact_effect",
	"typed_memory_graph_commits_v47_activation_effect",
}

func projectTypeEnvEffectObjects47() []sqliteSchemaObject47 {
	objects := make([]sqliteSchemaObject47, 0)
	for _, table := range projectTypeEnvEffectTables47 {
		objects = append(objects, sqliteSchemaObject47{
			kind: "table",
			name: table,
		})
	}
	for _, index := range projectTypeEnvEffectIndexes47 {
		objects = append(objects, sqliteSchemaObject47{
			kind: "index",
			name: index,
		})
	}
	for _, trigger := range projectTypeEnvEffectSpecificTriggers47 {
		objects = append(objects, sqliteSchemaObject47{
			kind: "trigger",
			name: trigger,
		})
	}
	for _, table := range projectTypeEnvEffectTables47 {
		for _, operation := range []string{"insert", "update", "delete"} {
			objects = append(objects, sqliteSchemaObject47{
				kind: "trigger",
				name: table + "_v47_no_" + operation,
			})
		}
	}
	for _, trigger := range projectTypeEnvCandidateAnnexTriggerNames47() {
		objects = append(objects, sqliteSchemaObject47{
			kind: "trigger",
			name: trigger,
		})
	}
	return objects
}

func projectTypeEnvEffectTableStatements47() []string {
	return []string{
		projectTypeEnvConfigAuthorityBasesTable47(),
		projectTypeEnvModePoliciesTable47(),
		projectTypeEnvNoPriorHeadProofsTable47(),
		projectTypeEnvHeadSelectionRequestsTable47(),
		projectTypeEnvAuthorizationContentsTable47(),
		projectTypeEnvTrustedCLISourcesTable47(),
		projectTypeEnvSpeechActRecordsTable47(),
		projectTypeEnvPermissionsV3Table47(),
		projectTypeEnvAuthorityResolutionBasesTable47(),
		projectTypeEnvAuthorityResolutionsTable47(),
		projectTypeEnvExplicitPolicyAcceptanceResolutionsTable47(),
		projectTypeEnvStrictPermissionResolutionsTable47(),
		projectTypeEnvAuthorityUsesTable47(),
		projectTypeEnvCASWorkRecordsTable47(),
		typedMemoryTypeEnvActivationsTable47(),
		projectTypeEnvHeadHistoryTable47(),
		projectTypeEnvHeadEffectObligationsTable47(),
		projectTypeEnvSelectionReceiptsTable47(),
		projectTypeEnvSelectionClosuresTable47(),
	}
}

func projectTypeEnvEffectIndexStatements47() []string {
	return []string{
		`CREATE UNIQUE INDEX idx_project_typeenv_requests_project_key_v47
			ON project_typeenv_head_selection_requests(
				project_id,
				original_idempotency_key
			)`,
		`CREATE UNIQUE INDEX idx_project_typeenv_authority_uses_replay_v47
			ON project_typeenv_head_selection_authority_uses(
				project_id,
				original_idempotency_key
			)`,
		`CREATE UNIQUE INDEX idx_project_typeenv_activations_revision_v47
			ON typed_memory_type_env_activations(
				project_id,
				committed_graph_revision
			)`,
		`CREATE UNIQUE INDEX idx_project_typeenv_head_history_graph_revision_v47
			ON project_typeenv_head_history(
				project_id,
				graph_revision
			)`,
	}
}

func projectTypeEnvConfigAuthorityBasesTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_config_authority_bases (
		config_authority_basis_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("config_authority_basis_ref") + `),
		config_authority_basis_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("config_authority_basis_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		authority_mode TEXT NOT NULL CHECK(
			authority_mode IN ('explicit_h_decide', 'strict_cli_speech_act')
		),
		config_carrier_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("config_carrier_ref") + `),
		config_carrier_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("config_carrier_digest") + `),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(config_authority_basis_ref, config_authority_basis_digest),
		UNIQUE(project_id, authority_mode, config_carrier_ref, config_carrier_digest)
	) WITHOUT ROWID`
}

func projectTypeEnvModePoliciesTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_mode_policies (
		mode_policy_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("mode_policy_ref") + `),
		mode_policy_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("mode_policy_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		authority_mode TEXT NOT NULL CHECK(
			authority_mode IN ('explicit_h_decide', 'strict_cli_speech_act')
		),
		config_authority_basis_ref TEXT NOT NULL,
		config_authority_basis_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("config_authority_basis_digest") + `),
		resolver_policy_ref TEXT,
		resolver_policy_edition TEXT,
		resolver_policy_digest TEXT,
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(mode_policy_ref, mode_policy_digest),
		FOREIGN KEY(config_authority_basis_ref, config_authority_basis_digest)
			REFERENCES project_typeenv_head_selection_config_authority_bases(
				config_authority_basis_ref,
				config_authority_basis_digest
			),
		CHECK(
			(authority_mode = 'explicit_h_decide'
				AND resolver_policy_ref IS NULL
				AND resolver_policy_edition IS NULL
				AND resolver_policy_digest IS NULL)
			OR
			(authority_mode = 'strict_cli_speech_act'
				AND resolver_policy_ref IS NOT NULL
				AND ` + typedMemoryNonBlankShape46("resolver_policy_ref") + `
				AND resolver_policy_edition IS NOT NULL
				AND ` + typedMemoryNonBlankShape46("resolver_policy_edition") + `
				AND resolver_policy_digest IS NOT NULL
				AND ` + typedMemorySHA256Shape46("resolver_policy_digest") + `)
		)
	) WITHOUT ROWID`
}

func projectTypeEnvNoPriorHeadProofsTable47() string {
	return `CREATE TABLE project_typeenv_no_prior_head_proofs (
		proof_ref TEXT PRIMARY KEY CHECK(` + typedMemoryNonBlankShape46("proof_ref") + `),
		proof_digest TEXT NOT NULL UNIQUE CHECK(` + typedMemorySHA256Shape46("proof_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		head_ref TEXT NOT NULL CHECK(
			head_ref = 'project-typeenv-head:' || project_id
		),
		graph_snapshot_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("graph_snapshot_ref") + `),
		graph_snapshot_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("graph_snapshot_digest") + `),
		expected_graph_revision INTEGER NOT NULL CHECK(expected_graph_revision >= 0),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(proof_ref, proof_digest),
		UNIQUE(
			project_id,
			head_ref,
			graph_snapshot_ref,
			graph_snapshot_digest,
			expected_graph_revision
		)
	) WITHOUT ROWID`
}

func projectTypeEnvHeadSelectionRequestsTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_requests (
		request_ref TEXT PRIMARY KEY CHECK(` + typedMemoryNonBlankShape46("request_ref") + `),
		request_digest TEXT NOT NULL UNIQUE CHECK(` + typedMemorySHA256Shape46("request_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		head_ref TEXT NOT NULL CHECK(
			head_ref = 'project-typeenv-head:' || project_id
		),
		predecessor_kind TEXT NOT NULL CHECK(
			predecessor_kind IN ('genesis', 'transition')
		),
		no_prior_head_proof_ref TEXT,
		no_prior_head_proof_digest TEXT,
		prior_head_ref TEXT,
		prior_head_revision INTEGER,
		prior_selected_composite_ref TEXT
			REFERENCES project_typeenv_executable_snapshots(type_env_ref),
		base_type_env_ref TEXT NOT NULL
			REFERENCES typed_memory_type_env_snapshots(type_env_ref),
		ordered_extension_refs_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("ordered_extension_refs_digest") + `),
		canonical_ordered_extension_refs BLOB NOT NULL
			CHECK(length(canonical_ordered_extension_refs) > 0),
		runtime_evaluation_basis_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("runtime_evaluation_basis_ref") + `),
		selected_composite_ref TEXT NOT NULL
			REFERENCES project_typeenv_executable_snapshots(type_env_ref),
		stage_ref TEXT NOT NULL
			REFERENCES project_typeenv_stages(stage_ref),
		stage_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("stage_digest") + `),
		expected_graph_revision INTEGER NOT NULL CHECK(expected_graph_revision >= 0),
		original_idempotency_key TEXT NOT NULL CHECK(
			length(original_idempotency_key) BETWEEN 1 AND 512
			AND trim(original_idempotency_key) = original_idempotency_key
		),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(request_ref, request_digest),
		FOREIGN KEY(no_prior_head_proof_ref, no_prior_head_proof_digest)
			REFERENCES project_typeenv_no_prior_head_proofs(
				proof_ref,
				proof_digest
			),
		FOREIGN KEY(project_id, prior_head_revision)
			REFERENCES project_typeenv_head_states(
				project_id,
				head_revision
			),
		CHECK(
			(predecessor_kind = 'genesis'
				AND no_prior_head_proof_ref IS NOT NULL
				AND no_prior_head_proof_digest IS NOT NULL
				AND prior_head_ref IS NULL
				AND prior_head_revision IS NULL
				AND prior_selected_composite_ref IS NULL)
			OR
			(predecessor_kind = 'transition'
				AND no_prior_head_proof_ref IS NULL
				AND no_prior_head_proof_digest IS NULL
				AND prior_head_ref IS NOT NULL
				AND prior_head_revision IS NOT NULL
				AND prior_head_revision > 0
				AND prior_selected_composite_ref IS NOT NULL)
		)
	) WITHOUT ROWID`
}

func projectTypeEnvAuthorizationContentsTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_authorization_contents (
		content_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("content_ref") + `),
		content_ref_kind TEXT NOT NULL CHECK(
			content_ref_kind IN ('claim_id', 'episteme')
		),
		content_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("content_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("request_digest") + `),
		judgement_context_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("judgement_context_ref") + `),
		action_kind TEXT NOT NULL CHECK(action_kind IN ('genesis', 'transition')),
		valid_from TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("valid_from") + `),
		valid_until TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("valid_until") + `),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(content_ref, content_digest),
		UNIQUE(content_digest),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			),
		CHECK(valid_from < valid_until)
	) WITHOUT ROWID`
}

func projectTypeEnvTrustedCLISourcesTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_trusted_cli_sources (
		trusted_cli_source_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("trusted_cli_source_ref") + `),
		trusted_cli_source_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("trusted_cli_source_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		mode_policy_ref TEXT NOT NULL,
		mode_policy_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("mode_policy_digest") + `),
		config_authority_basis_ref TEXT NOT NULL,
		config_authority_basis_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("config_authority_basis_digest") + `),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("content_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("request_digest") + `),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(trusted_cli_source_ref, trusted_cli_source_digest),
		FOREIGN KEY(mode_policy_ref, mode_policy_digest)
			REFERENCES project_typeenv_head_selection_mode_policies(
				mode_policy_ref,
				mode_policy_digest
			),
		FOREIGN KEY(config_authority_basis_ref, config_authority_basis_digest)
			REFERENCES project_typeenv_head_selection_config_authority_bases(
				config_authority_basis_ref,
				config_authority_basis_digest
			),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(
				content_ref,
				content_digest
			),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			)
	) WITHOUT ROWID`
}

func projectTypeEnvSpeechActRecordsTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_speech_act_records (
		speech_act_record_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("speech_act_record_ref") + `),
		speech_act_record_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("speech_act_record_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		speech_act_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("speech_act_ref") + `),
		human_work_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("human_work_ref") + `),
		source_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("source_digest") + `),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("content_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("request_digest") + `),
		permission_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("permission_ref") + `),
		permission_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("permission_digest") + `),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(speech_act_record_ref, speech_act_record_digest),
		UNIQUE(speech_act_ref),
		UNIQUE(human_work_ref),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(
				content_ref,
				content_digest
			),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			),
		FOREIGN KEY(permission_ref, permission_digest)
			REFERENCES project_typeenv_head_selection_permissions_v3(
				permission_ref,
				permission_digest
			)
			DEFERRABLE INITIALLY DEFERRED
	) WITHOUT ROWID`
}

func projectTypeEnvPermissionsV3Table47() string {
	return `CREATE TABLE project_typeenv_head_selection_permissions_v3 (
		permission_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("permission_ref") + `),
		permission_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("permission_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		subject_role_assignment_ref TEXT NOT NULL CHECK(
			subject_role_assignment_ref =
				'role-assignment:haft-software-system:project-governance-substrate:' ||
				subject_role_assignment_digest
		),
		subject_role_assignment_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("subject_role_assignment_digest") + `),
		subject_schema TEXT NOT NULL CHECK(
			subject_schema =
				'haft.project-typeenv.head-selection-permission-subject-role-assignment/v1'
		),
		subject_holder_system_ref TEXT NOT NULL CHECK(
			subject_holder_system_ref = 'system:haft-software-system'
		),
		subject_holder_kind TEXT NOT NULL CHECK(subject_holder_kind = 'U.System'),
		subject_role_ref TEXT NOT NULL CHECK(
			subject_role_ref = 'role:project-governance-substrate'
		),
		subject_context_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("subject_context_ref") + `),
		subject_assignment_from TEXT NOT NULL CHECK(` +
		sqliteCanonicalUTCNanoShape("subject_assignment_from") + `),
		subject_assignment_until TEXT NOT NULL CHECK(` +
		sqliteCanonicalUTCNanoShape("subject_assignment_until") + `),
		subject_assignment_policy_ref TEXT NOT NULL CHECK(
			subject_assignment_policy_ref =
				'project-typeenv-head-selection-execution-role-policy:' ||
				subject_assignment_policy_digest
		),
		subject_assignment_policy_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("subject_assignment_policy_digest") + `),
		subject_assignment_policy_edition_ref TEXT NOT NULL CHECK(
			subject_assignment_policy_edition_ref =
				'policy-edition:project-typeenv-head-selection-execution-role/v1'
		),
		subject_assignment_policy_selection TEXT NOT NULL CHECK(
			subject_assignment_policy_selection = 'current_for_new_write_at_seal'
		),
		subject_system_admission_ref TEXT NOT NULL CHECK(
			subject_system_admission_ref =
				'system-admission:project-typeenv-head-selection:' ||
				subject_system_admission_digest
		),
		subject_system_admission_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("subject_system_admission_digest") + `),
		subject_role_admission_ref TEXT NOT NULL CHECK(
			subject_role_admission_ref =
				'role-admission:project-typeenv-head-selection:' ||
				subject_role_admission_digest
		),
		subject_role_admission_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("subject_role_admission_digest") + `),
		subject_assignment_justification_ref TEXT NOT NULL CHECK(
			subject_assignment_justification_ref =
				'role-assignment-justification:project-typeenv-head-selection:' ||
				subject_assignment_justification_digest
		),
		subject_assignment_justification_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("subject_assignment_justification_digest") + `),
		subject_assignment_provenance_ref TEXT NOT NULL CHECK(
			subject_assignment_provenance_ref =
				'role-assignment-provenance:project-typeenv-head-selection:' ||
				subject_assignment_provenance_digest
		),
		subject_assignment_provenance_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("subject_assignment_provenance_digest") + `),
		subject_authorization_description_kind TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("subject_authorization_description_kind") + `),
		subject_authorization_description_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("subject_authorization_description_ref") + `),
		subject_authorization_content_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("subject_authorization_content_digest") + `),
		subject_canonical_bytes BLOB NOT NULL
			CHECK(length(subject_canonical_bytes) > 0),
		modality TEXT NOT NULL CHECK(modality = 'MAY'),
		claim_scope_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("claim_scope_ref") + `),
		claim_scope_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("claim_scope_digest") + `),
		context_policy_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("context_policy_ref") + `),
		context_policy_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("context_policy_digest") + `),
		referents_canonical_bytes BLOB NOT NULL
			CHECK(length(referents_canonical_bytes) > 0),
		effective_from TEXT NOT NULL CHECK(` +
		sqliteCanonicalUTCNanoShape("effective_from") + `),
		validity_until TEXT NOT NULL CHECK(` +
		sqliteCanonicalUTCNanoShape("validity_until") + `),
		speech_act_record_ref TEXT NOT NULL,
		speech_act_record_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("speech_act_record_digest") + `),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("content_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("request_digest") + `),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		UNIQUE(permission_ref, permission_digest),
		UNIQUE(speech_act_record_ref, speech_act_record_digest),
		FOREIGN KEY(speech_act_record_ref, speech_act_record_digest)
			REFERENCES project_typeenv_head_selection_speech_act_records(
				speech_act_record_ref,
				speech_act_record_digest
			)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(
				content_ref,
				content_digest
			),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			),
		CHECK(
			subject_assignment_from <= effective_from
			AND effective_from < validity_until
			AND validity_until <= subject_assignment_until
			AND julianday(subject_assignment_from) >=
				julianday('2026-07-15T19:44:27.046111Z')
			AND julianday(subject_assignment_until) <=
				julianday('2026-09-18T23:59:59Z')
		)
	) WITHOUT ROWID`
}

func projectTypeEnvAuthorityResolutionBasesTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_authority_resolution_bases (
		basis_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("basis_ref") + `),
		basis_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("basis_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		resolver_policy_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("resolver_policy_ref") + `),
		resolver_policy_edition TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("resolver_policy_edition") + `),
		resolver_policy_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("resolver_policy_digest") + `),
		speech_act_record_ref TEXT NOT NULL,
		speech_act_record_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("speech_act_record_digest") + `),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("content_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("request_digest") + `),
		stage_ref TEXT NOT NULL REFERENCES project_typeenv_stages(stage_ref),
		stage_digest TEXT NOT NULL REFERENCES project_typeenv_stages(stage_digest),
		evaluated_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("evaluated_at") + `),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		UNIQUE(basis_ref, basis_digest),
		FOREIGN KEY(speech_act_record_ref, speech_act_record_digest)
			REFERENCES project_typeenv_head_selection_speech_act_records(
				speech_act_record_ref,
				speech_act_record_digest
			),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(
				content_ref,
				content_digest
			),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			)
	) WITHOUT ROWID`
}

func projectTypeEnvAuthorityResolutionsTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_authority_resolutions (
		authority_resolution_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("authority_resolution_ref") + `),
		authority_resolution_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("authority_resolution_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		authority_resolution_kind TEXT NOT NULL CHECK(
			authority_resolution_kind IN (
				'explicit_policy_acceptance',
				'strict_permission'
			)
		),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("content_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("request_digest") + `),
		trusted_cli_source_ref TEXT,
		trusted_cli_source_digest TEXT,
		strict_basis_ref TEXT,
		strict_basis_digest TEXT,
		explicit_resolution_ref TEXT,
		explicit_resolution_digest TEXT,
		strict_resolution_ref TEXT,
		strict_resolution_digest TEXT,
		evaluated_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("evaluated_at") + `),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(authority_resolution_ref, authority_resolution_digest),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(
				content_ref,
				content_digest
			),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			),
		FOREIGN KEY(trusted_cli_source_ref, trusted_cli_source_digest)
			REFERENCES project_typeenv_head_selection_trusted_cli_sources(
				trusted_cli_source_ref,
				trusted_cli_source_digest
			),
		FOREIGN KEY(strict_basis_ref, strict_basis_digest)
			REFERENCES project_typeenv_head_selection_authority_resolution_bases(
				basis_ref,
				basis_digest
			),
		FOREIGN KEY(explicit_resolution_ref, explicit_resolution_digest)
			REFERENCES project_typeenv_head_selection_explicit_policy_acceptance_resolutions(
				authority_resolution_ref,
				authority_resolution_digest
			)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(strict_resolution_ref, strict_resolution_digest)
			REFERENCES project_typeenv_head_selection_strict_permission_resolutions(
				authority_resolution_ref,
				authority_resolution_digest
			)
			DEFERRABLE INITIALLY DEFERRED,
		CHECK(
			(authority_resolution_kind = 'explicit_policy_acceptance'
				AND trusted_cli_source_ref IS NOT NULL
				AND trusted_cli_source_digest IS NOT NULL
				AND strict_basis_ref IS NULL
				AND strict_basis_digest IS NULL
				AND explicit_resolution_ref = authority_resolution_ref
				AND explicit_resolution_digest = authority_resolution_digest
				AND strict_resolution_ref IS NULL
				AND strict_resolution_digest IS NULL)
			OR
			(authority_resolution_kind = 'strict_permission'
				AND trusted_cli_source_ref IS NULL
				AND trusted_cli_source_digest IS NULL
				AND strict_basis_ref IS NOT NULL
				AND strict_basis_digest IS NOT NULL
				AND explicit_resolution_ref IS NULL
				AND explicit_resolution_digest IS NULL
				AND strict_resolution_ref = authority_resolution_ref
				AND strict_resolution_digest = authority_resolution_digest)
		)
	) WITHOUT ROWID`
}

func projectTypeEnvExplicitPolicyAcceptanceResolutionsTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_explicit_policy_acceptance_resolutions (
		authority_resolution_ref TEXT PRIMARY KEY,
		authority_resolution_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("authority_resolution_digest") + `),
		trusted_cli_source_ref TEXT NOT NULL,
		trusted_cli_source_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("trusted_cli_source_digest") + `),
		UNIQUE(authority_resolution_ref, authority_resolution_digest),
		FOREIGN KEY(authority_resolution_ref, authority_resolution_digest)
			REFERENCES project_typeenv_head_selection_authority_resolutions(
				authority_resolution_ref,
				authority_resolution_digest
			)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(trusted_cli_source_ref, trusted_cli_source_digest)
			REFERENCES project_typeenv_head_selection_trusted_cli_sources(
				trusted_cli_source_ref,
				trusted_cli_source_digest
			)
	) WITHOUT ROWID`
}

func projectTypeEnvStrictPermissionResolutionsTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_strict_permission_resolutions (
		authority_resolution_ref TEXT PRIMARY KEY,
		authority_resolution_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("authority_resolution_digest") + `),
		basis_ref TEXT NOT NULL,
		basis_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("basis_digest") + `),
		speech_act_record_ref TEXT NOT NULL,
		speech_act_record_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("speech_act_record_digest") + `),
		permission_ref TEXT NOT NULL,
		permission_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("permission_digest") + `),
		UNIQUE(authority_resolution_ref, authority_resolution_digest),
		FOREIGN KEY(authority_resolution_ref, authority_resolution_digest)
			REFERENCES project_typeenv_head_selection_authority_resolutions(
				authority_resolution_ref,
				authority_resolution_digest
			)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(basis_ref, basis_digest)
			REFERENCES project_typeenv_head_selection_authority_resolution_bases(
				basis_ref,
				basis_digest
			),
		FOREIGN KEY(speech_act_record_ref, speech_act_record_digest)
			REFERENCES project_typeenv_head_selection_speech_act_records(
				speech_act_record_ref,
				speech_act_record_digest
			),
		FOREIGN KEY(permission_ref, permission_digest)
			REFERENCES project_typeenv_head_selection_permissions_v3(
				permission_ref,
				permission_digest
			)
	) WITHOUT ROWID`
}

func projectTypeEnvAuthorityUsesTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_authority_uses (
		authority_use_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("authority_use_ref") + `),
		authority_use_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("authority_use_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		original_idempotency_key TEXT NOT NULL CHECK(
			length(original_idempotency_key) BETWEEN 1 AND 512
			AND trim(original_idempotency_key) = original_idempotency_key
		),
		authority_resolution_kind TEXT NOT NULL CHECK(
			authority_resolution_kind IN (
				'explicit_policy_acceptance',
				'strict_permission'
			)
		),
		authority_resolution_ref TEXT NOT NULL,
		authority_resolution_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("authority_resolution_digest") + `),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("content_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("request_digest") + `),
		work_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("work_ref") + `),
		receipt_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("receipt_ref") + `),
		predecessor_kind TEXT NOT NULL CHECK(
			predecessor_kind IN ('genesis', 'transition')
		),
		predecessor_head_ref TEXT,
		predecessor_head_revision INTEGER,
		predecessor_selected_composite_ref TEXT
			REFERENCES project_typeenv_executable_snapshots(type_env_ref),
		base_type_env_ref TEXT NOT NULL
			REFERENCES typed_memory_type_env_snapshots(type_env_ref),
		ordered_extension_refs_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("ordered_extension_refs_digest") + `),
		canonical_ordered_extension_refs BLOB NOT NULL
			CHECK(length(canonical_ordered_extension_refs) > 0),
		runtime_evaluation_basis_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("runtime_evaluation_basis_ref") + `),
		selected_composite_ref TEXT NOT NULL
			REFERENCES project_typeenv_executable_snapshots(type_env_ref),
		stage_ref TEXT NOT NULL REFERENCES project_typeenv_stages(stage_ref),
		stage_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("stage_digest") + `),
		expected_graph_revision INTEGER NOT NULL CHECK(expected_graph_revision >= 0),
		committed_head_revision INTEGER NOT NULL CHECK(committed_head_revision > 0),
		committed_graph_revision INTEGER NOT NULL CHECK(
			committed_graph_revision = expected_graph_revision + 1
		),
		verifier_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("verifier_ref") + `),
		verifier_edition INTEGER NOT NULL CHECK(verifier_edition > 0),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(authority_use_ref, authority_use_digest),
		FOREIGN KEY(authority_resolution_ref, authority_resolution_digest)
			REFERENCES project_typeenv_head_selection_authority_resolutions(
				authority_resolution_ref,
				authority_resolution_digest
			),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(
				content_ref,
				content_digest
			),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			),
		FOREIGN KEY(work_ref)
			REFERENCES project_typeenv_head_cas_work_records(work_ref)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(receipt_ref)
			REFERENCES project_typeenv_head_selection_receipts(receipt_ref)
			DEFERRABLE INITIALLY DEFERRED,
		CHECK(
			(predecessor_kind = 'genesis'
				AND predecessor_head_ref IS NULL
				AND predecessor_head_revision IS NULL
				AND predecessor_selected_composite_ref IS NULL
				AND committed_head_revision = 1)
			OR
			(predecessor_kind = 'transition'
				AND predecessor_head_ref IS NOT NULL
				AND predecessor_head_revision IS NOT NULL
				AND predecessor_head_revision > 0
				AND predecessor_selected_composite_ref IS NOT NULL
				AND committed_head_revision = predecessor_head_revision + 1)
		)
	) WITHOUT ROWID`
}

func projectTypeEnvCASWorkRecordsTable47() string {
	return `CREATE TABLE project_typeenv_head_cas_work_records (
		cas_work_record_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("cas_work_record_ref") + `),
		cas_work_record_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("cas_work_record_digest") + `),
		work_ref TEXT NOT NULL UNIQUE CHECK(` + typedMemoryNonBlankShape46("work_ref") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		authority_use_ref TEXT NOT NULL,
		authority_use_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("authority_use_digest") + `),
		receipt_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("receipt_ref") + `),
		activation_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("activation_ref") + `),
		method_description_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("method_description_ref") + `),
		executor_system_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("executor_system_ref") + `),
		executor_role_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("executor_role_ref") + `),
		bounded_context_ref TEXT NOT NULL CHECK(` +
		typedMemoryNonBlankShape46("bounded_context_ref") + `),
		work_started_at TEXT NOT NULL CHECK(` +
		sqliteCanonicalUTCNanoShape("work_started_at") + `),
		effect_sealed_at TEXT NOT NULL CHECK(` +
		sqliteCanonicalUTCNanoShape("effect_sealed_at") + `),
		committed_head_revision INTEGER NOT NULL CHECK(committed_head_revision > 0),
		committed_graph_revision INTEGER NOT NULL CHECK(committed_graph_revision > 0),
		selected_composite_ref TEXT NOT NULL
			REFERENCES project_typeenv_executable_snapshots(type_env_ref),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(cas_work_record_ref, cas_work_record_digest),
		FOREIGN KEY(authority_use_ref, authority_use_digest)
			REFERENCES project_typeenv_head_selection_authority_uses(
				authority_use_ref,
				authority_use_digest
			)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(receipt_ref)
			REFERENCES project_typeenv_head_selection_receipts(receipt_ref)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(activation_ref)
			REFERENCES typed_memory_type_env_activations(activation_ref)
			DEFERRABLE INITIALLY DEFERRED,
		CHECK(work_started_at <= effect_sealed_at)
	) WITHOUT ROWID`
}

func typedMemoryTypeEnvActivationsTable47() string {
	return `CREATE TABLE typed_memory_type_env_activations (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal = 0),
		activation_ref TEXT NOT NULL UNIQUE CHECK(` +
		typedMemoryNonBlankShape46("activation_ref") + `),
		activation_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("activation_digest") + `),
		canonical_activation_bytes BLOB NOT NULL
			CHECK(length(canonical_activation_bytes) > 0),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("request_digest") + `),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("content_digest") + `),
		authority_use_ref TEXT NOT NULL,
		authority_use_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("authority_use_digest") + `),
		work_ref TEXT NOT NULL,
		basis_type_env_ref TEXT NOT NULL
			REFERENCES typed_memory_type_env_coordinates(type_env_ref),
		result_type_env_ref TEXT NOT NULL
			REFERENCES project_typeenv_executable_snapshots(type_env_ref),
		stage_ref TEXT NOT NULL REFERENCES project_typeenv_stages(stage_ref),
		stage_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("stage_digest") + `),
		head_ref TEXT NOT NULL,
		expected_graph_revision INTEGER NOT NULL CHECK(expected_graph_revision >= 0),
		committed_graph_revision INTEGER NOT NULL CHECK(
			committed_graph_revision = expected_graph_revision + 1
		),
		committed_head_revision INTEGER NOT NULL CHECK(committed_head_revision > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, event_ref, change_ordinal),
		UNIQUE(project_id, event_ref),
		UNIQUE(project_id, activation_ref),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(
				content_ref,
				content_digest
			),
		FOREIGN KEY(authority_use_ref, authority_use_digest)
			REFERENCES project_typeenv_head_selection_authority_uses(
				authority_use_ref,
				authority_use_digest
			)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(work_ref)
			REFERENCES project_typeenv_head_cas_work_records(work_ref)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(project_id, committed_head_revision)
			REFERENCES project_typeenv_head_states(
				project_id,
				head_revision
			)
			DEFERRABLE INITIALLY DEFERRED,
		CHECK(basis_type_env_ref != result_type_env_ref),
		CHECK(head_ref = 'project-typeenv-head:' || project_id)
	) WITHOUT ROWID`
}

func projectTypeEnvHeadHistoryTable47() string {
	return `CREATE TABLE project_typeenv_head_history (
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		head_ref TEXT NOT NULL CHECK(
			head_ref = 'project-typeenv-head:' || project_id
		),
		head_revision INTEGER NOT NULL CHECK(head_revision > 0),
		selected_composite_ref TEXT NOT NULL
			REFERENCES project_typeenv_executable_snapshots(type_env_ref),
		graph_revision INTEGER NOT NULL CHECK(graph_revision > 0),
		graph_event_ref TEXT NOT NULL,
		graph_commit_ref TEXT NOT NULL,
		activation_ref TEXT NOT NULL,
		activation_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("activation_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("request_digest") + `),
		authority_use_ref TEXT NOT NULL,
		authority_use_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("authority_use_digest") + `),
		work_ref TEXT NOT NULL,
		receipt_ref TEXT NOT NULL,
		head_state_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("head_state_digest") + `),
		canonical_head_state_bytes BLOB NOT NULL
			CHECK(length(canonical_head_state_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, head_revision),
		UNIQUE(project_id, graph_revision),
		UNIQUE(project_id, graph_event_ref),
		UNIQUE(project_id, graph_commit_ref),
		UNIQUE(project_id, activation_ref),
		UNIQUE(
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			head_state_digest,
			canonical_head_state_bytes
		),
		FOREIGN KEY(
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			head_state_digest,
			canonical_head_state_bytes
		) REFERENCES project_typeenv_head_states(
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			state_digest,
			canonical_bytes
		),
		FOREIGN KEY(project_id, graph_event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(project_id, graph_commit_ref)
			REFERENCES typed_memory_graph_commits(project_id, commit_ref)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(project_id, activation_ref)
			REFERENCES typed_memory_type_env_activations(project_id, activation_ref),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			),
		FOREIGN KEY(authority_use_ref, authority_use_digest)
			REFERENCES project_typeenv_head_selection_authority_uses(
				authority_use_ref,
				authority_use_digest
			)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(work_ref)
			REFERENCES project_typeenv_head_cas_work_records(work_ref)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(receipt_ref)
			REFERENCES project_typeenv_head_selection_receipts(receipt_ref)
			DEFERRABLE INITIALLY DEFERRED
	) WITHOUT ROWID`
}

func projectTypeEnvHeadEffectObligationsTable47() string {
	return `CREATE TABLE project_typeenv_head_effect_obligations (
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		head_ref TEXT NOT NULL CHECK(
			head_ref = 'project-typeenv-head:' || project_id
		),
		head_revision INTEGER NOT NULL CHECK(head_revision > 0),
		selected_composite_ref TEXT NOT NULL
			REFERENCES project_typeenv_executable_snapshots(type_env_ref),
		head_state_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("head_state_digest") + `),
		canonical_head_state_bytes BLOB NOT NULL
			CHECK(length(canonical_head_state_bytes) > 0),
		PRIMARY KEY(project_id, head_revision),
		FOREIGN KEY(
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			head_state_digest,
			canonical_head_state_bytes
		) REFERENCES project_typeenv_head_states(
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			state_digest,
			canonical_bytes
		)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			head_state_digest,
			canonical_head_state_bytes
		) REFERENCES project_typeenv_head_history(
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			head_state_digest,
			canonical_head_state_bytes
		)
			DEFERRABLE INITIALLY DEFERRED
	) WITHOUT ROWID`
}

func projectTypeEnvSelectionReceiptsTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_receipts (
		receipt_ref TEXT PRIMARY KEY CHECK(` + typedMemoryNonBlankShape46("receipt_ref") + `),
		receipt_digest TEXT NOT NULL UNIQUE CHECK(` + typedMemorySHA256Shape46("receipt_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		authority_use_ref TEXT NOT NULL UNIQUE,
		authority_use_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("authority_use_digest") + `),
		cas_work_record_ref TEXT NOT NULL UNIQUE,
		cas_work_record_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("cas_work_record_digest") + `),
		work_ref TEXT NOT NULL UNIQUE,
		activation_ref TEXT NOT NULL UNIQUE,
		activation_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("activation_digest") + `),
		authority_resolution_ref TEXT NOT NULL,
		authority_resolution_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("authority_resolution_digest") + `),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("content_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("request_digest") + `),
		head_ref TEXT NOT NULL,
		head_revision INTEGER NOT NULL CHECK(head_revision > 0),
		selected_composite_ref TEXT NOT NULL
			REFERENCES project_typeenv_executable_snapshots(type_env_ref),
		graph_revision INTEGER NOT NULL CHECK(graph_revision > 0),
		graph_event_ref TEXT NOT NULL,
		graph_commit_ref TEXT NOT NULL,
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(receipt_ref, receipt_digest),
		FOREIGN KEY(authority_use_ref, authority_use_digest)
			REFERENCES project_typeenv_head_selection_authority_uses(
				authority_use_ref,
				authority_use_digest
			)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(cas_work_record_ref, cas_work_record_digest)
			REFERENCES project_typeenv_head_cas_work_records(
				cas_work_record_ref,
				cas_work_record_digest
			)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(work_ref)
			REFERENCES project_typeenv_head_cas_work_records(work_ref)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(project_id, activation_ref)
			REFERENCES typed_memory_type_env_activations(project_id, activation_ref),
		FOREIGN KEY(authority_resolution_ref, authority_resolution_digest)
			REFERENCES project_typeenv_head_selection_authority_resolutions(
				authority_resolution_ref,
				authority_resolution_digest
			),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(
				content_ref,
				content_digest
			),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			),
		FOREIGN KEY(project_id, head_revision)
			REFERENCES project_typeenv_head_history(project_id, head_revision)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(project_id, graph_event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(project_id, graph_commit_ref)
			REFERENCES typed_memory_graph_commits(project_id, commit_ref)
			DEFERRABLE INITIALLY DEFERRED
	) WITHOUT ROWID`
}

func projectTypeEnvSelectionClosuresTable47() string {
	return `CREATE TABLE project_typeenv_head_selection_closures (
		closure_ref TEXT PRIMARY KEY CHECK(` + typedMemoryNonBlankShape46("closure_ref") + `),
		closure_digest TEXT NOT NULL UNIQUE CHECK(` + typedMemorySHA256Shape46("closure_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		authority_use_ref TEXT NOT NULL UNIQUE,
		authority_use_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("authority_use_digest") + `),
		cas_work_record_ref TEXT NOT NULL UNIQUE,
		cas_work_record_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("cas_work_record_digest") + `),
		receipt_ref TEXT NOT NULL UNIQUE,
		receipt_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("receipt_digest") + `),
		activation_ref TEXT NOT NULL UNIQUE,
		activation_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("activation_digest") + `),
		authority_resolution_ref TEXT NOT NULL,
		authority_resolution_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("authority_resolution_digest") + `),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("content_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("request_digest") + `),
		head_ref TEXT NOT NULL,
		head_revision INTEGER NOT NULL CHECK(head_revision > 0),
		head_state_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("head_state_digest") + `),
		graph_revision INTEGER NOT NULL CHECK(graph_revision > 0),
		graph_event_ref TEXT NOT NULL,
		graph_event_digest TEXT NOT NULL CHECK(` +
		typedMemorySHA256Shape46("graph_event_digest") + `),
		graph_commit_ref TEXT NOT NULL,
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(closure_ref, closure_digest),
		FOREIGN KEY(authority_use_ref, authority_use_digest)
			REFERENCES project_typeenv_head_selection_authority_uses(
				authority_use_ref,
				authority_use_digest
			),
		FOREIGN KEY(cas_work_record_ref, cas_work_record_digest)
			REFERENCES project_typeenv_head_cas_work_records(
				cas_work_record_ref,
				cas_work_record_digest
			),
		FOREIGN KEY(receipt_ref, receipt_digest)
			REFERENCES project_typeenv_head_selection_receipts(
				receipt_ref,
				receipt_digest
			),
		FOREIGN KEY(project_id, activation_ref)
			REFERENCES typed_memory_type_env_activations(project_id, activation_ref),
		FOREIGN KEY(authority_resolution_ref, authority_resolution_digest)
			REFERENCES project_typeenv_head_selection_authority_resolutions(
				authority_resolution_ref,
				authority_resolution_digest
			),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(
				content_ref,
				content_digest
			),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(
				request_ref,
				request_digest
			),
		FOREIGN KEY(project_id, head_revision)
			REFERENCES project_typeenv_head_history(project_id, head_revision),
		FOREIGN KEY(project_id, graph_event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(project_id, graph_commit_ref)
			REFERENCES typed_memory_graph_commits(project_id, commit_ref)
			DEFERRABLE INITIALLY DEFERRED
	) WITHOUT ROWID`
}

type immutableTablePolicy47 struct {
	table              string
	duplicatePredicate string
}

func projectTypeEnvCandidatePolicies47() []immutableTablePolicy47 {
	return []immutableTablePolicy47{
		{
			table:              "project_typeenv_artifact_store_schema",
			duplicatePredicate: "existing.singleton = NEW.singleton",
		},
		{
			table: "project_typeenv_artifacts",
			duplicatePredicate: `(existing.artifact_kind = NEW.artifact_kind
				AND (existing.artifact_ref = NEW.artifact_ref
					OR existing.artifact_digest = NEW.artifact_digest))`,
		},
		{
			table: "project_typeenv_runtime_mechanisms",
			duplicatePredicate: `(existing.artifact_ref = NEW.artifact_ref
				AND existing.edition = NEW.edition)`,
		},
		{
			table:              "project_typeenv_registration_policies",
			duplicatePredicate: `(existing.registration_ref = NEW.registration_ref OR existing.artifact_digest = NEW.artifact_digest)`,
		},
		{
			table:              "project_typeenv_stage_store_schema",
			duplicatePredicate: "existing.singleton = NEW.singleton",
		},
		{
			table:              "project_typeenv_composite_verifications",
			duplicatePredicate: `(existing.verification_ref = NEW.verification_ref OR existing.verification_digest = NEW.verification_digest)`,
		},
		{
			table:              "project_typeenv_stages",
			duplicatePredicate: `(existing.stage_ref = NEW.stage_ref OR existing.stage_digest = NEW.stage_digest)`,
		},
		{
			table:              "project_typeenv_executable_snapshots",
			duplicatePredicate: `(existing.type_env_ref = NEW.type_env_ref OR existing.snapshot_digest = NEW.snapshot_digest)`,
		},
	}
}

func projectTypeEnvCandidateAnnexTriggerNames47() []string {
	names := make([]string, 0)
	for _, policy := range projectTypeEnvCandidatePolicies47() {
		for _, operation := range []string{"insert", "update", "delete"} {
			names = append(
				names,
				policy.table+"_v47_no_"+operation,
			)
		}
	}
	return names
}

func projectTypeEnvCandidateAnnexTriggers47() []string {
	statements := make([]string, 0)
	for _, policy := range projectTypeEnvCandidatePolicies47() {
		statements = append(
			statements,
			projectTypeEnvNoReplaceTrigger47(
				policy.table,
				policy.duplicatePredicate,
				"project TypeEnv candidate store is immutable",
			),
			projectTypeEnvImmutableTrigger47(
				policy.table,
				"update",
				"project TypeEnv candidate store is immutable",
			),
			projectTypeEnvImmutableTrigger47(
				policy.table,
				"delete",
				"project TypeEnv candidate store is immutable",
			),
		)
	}
	return statements
}

func projectTypeEnvEffectPolicies47() []immutableTablePolicy47 {
	return []immutableTablePolicy47{
		{
			table: "project_typeenv_head_selection_config_authority_bases",
			duplicatePredicate: `(existing.config_authority_basis_ref = NEW.config_authority_basis_ref
				OR existing.config_authority_basis_digest = NEW.config_authority_basis_digest)`,
		},
		{
			table: "project_typeenv_head_selection_mode_policies",
			duplicatePredicate: `(existing.mode_policy_ref = NEW.mode_policy_ref
				OR existing.mode_policy_digest = NEW.mode_policy_digest)`,
		},
		{
			table: "project_typeenv_no_prior_head_proofs",
			duplicatePredicate: `(existing.proof_ref = NEW.proof_ref
				OR existing.proof_digest = NEW.proof_digest)`,
		},
		{
			table: "project_typeenv_head_selection_requests",
			duplicatePredicate: `(existing.request_ref = NEW.request_ref
				OR existing.request_digest = NEW.request_digest
				OR (existing.project_id = NEW.project_id
					AND existing.original_idempotency_key = NEW.original_idempotency_key))`,
		},
		{
			table: "project_typeenv_head_selection_authorization_contents",
			duplicatePredicate: `(existing.content_digest = NEW.content_digest
				OR (existing.content_ref = NEW.content_ref
					AND existing.content_digest = NEW.content_digest))`,
		},
		{
			table: "project_typeenv_head_selection_trusted_cli_sources",
			duplicatePredicate: `(existing.trusted_cli_source_ref = NEW.trusted_cli_source_ref
				OR existing.trusted_cli_source_digest = NEW.trusted_cli_source_digest)`,
		},
		{
			table: "project_typeenv_head_selection_speech_act_records",
			duplicatePredicate: `(existing.speech_act_record_ref = NEW.speech_act_record_ref
				OR existing.speech_act_record_digest = NEW.speech_act_record_digest
				OR existing.speech_act_ref = NEW.speech_act_ref
				OR existing.human_work_ref = NEW.human_work_ref)`,
		},
		{
			table: "project_typeenv_head_selection_permissions_v3",
			duplicatePredicate: `(existing.permission_ref = NEW.permission_ref
				OR existing.permission_digest = NEW.permission_digest
				OR (existing.speech_act_record_ref = NEW.speech_act_record_ref
					AND existing.speech_act_record_digest = NEW.speech_act_record_digest))`,
		},
		{
			table: "project_typeenv_head_selection_authority_resolution_bases",
			duplicatePredicate: `(existing.basis_ref = NEW.basis_ref
				OR existing.basis_digest = NEW.basis_digest)`,
		},
		{
			table: "project_typeenv_head_selection_authority_resolutions",
			duplicatePredicate: `(existing.authority_resolution_ref = NEW.authority_resolution_ref
				OR existing.authority_resolution_digest = NEW.authority_resolution_digest)`,
		},
		{
			table: "project_typeenv_head_selection_explicit_policy_acceptance_resolutions",
			duplicatePredicate: `(existing.authority_resolution_ref = NEW.authority_resolution_ref
				OR existing.authority_resolution_digest = NEW.authority_resolution_digest)`,
		},
		{
			table: "project_typeenv_head_selection_strict_permission_resolutions",
			duplicatePredicate: `(existing.authority_resolution_ref = NEW.authority_resolution_ref
				OR existing.authority_resolution_digest = NEW.authority_resolution_digest)`,
		},
		{
			table: "project_typeenv_head_selection_authority_uses",
			duplicatePredicate: `(existing.authority_use_ref = NEW.authority_use_ref
				OR existing.authority_use_digest = NEW.authority_use_digest
				OR (existing.project_id = NEW.project_id
					AND existing.original_idempotency_key = NEW.original_idempotency_key))`,
		},
		{
			table: "project_typeenv_head_cas_work_records",
			duplicatePredicate: `(existing.cas_work_record_ref = NEW.cas_work_record_ref
				OR existing.cas_work_record_digest = NEW.cas_work_record_digest
				OR existing.work_ref = NEW.work_ref)`,
		},
		{
			table: "typed_memory_type_env_activations",
			duplicatePredicate: `((existing.project_id = NEW.project_id
					AND existing.event_ref = NEW.event_ref)
				OR existing.activation_ref = NEW.activation_ref
				OR existing.activation_digest = NEW.activation_digest
				OR (existing.project_id = NEW.project_id
					AND existing.committed_graph_revision = NEW.committed_graph_revision))`,
		},
		{
			table: "project_typeenv_head_history",
			duplicatePredicate: `((existing.project_id = NEW.project_id
					AND existing.head_revision = NEW.head_revision)
				OR (existing.project_id = NEW.project_id
					AND existing.graph_revision = NEW.graph_revision)
				OR (existing.project_id = NEW.project_id
					AND existing.graph_event_ref = NEW.graph_event_ref)
				OR (existing.project_id = NEW.project_id
					AND existing.graph_commit_ref = NEW.graph_commit_ref)
				OR (existing.project_id = NEW.project_id
					AND existing.activation_ref = NEW.activation_ref))`,
		},
		{
			table: "project_typeenv_head_effect_obligations",
			duplicatePredicate: `(existing.project_id = NEW.project_id
				AND existing.head_revision = NEW.head_revision)`,
		},
		{
			table: "project_typeenv_head_selection_receipts",
			duplicatePredicate: `(existing.receipt_ref = NEW.receipt_ref
				OR existing.receipt_digest = NEW.receipt_digest
				OR existing.authority_use_ref = NEW.authority_use_ref
				OR existing.cas_work_record_ref = NEW.cas_work_record_ref
				OR existing.work_ref = NEW.work_ref
				OR existing.activation_ref = NEW.activation_ref)`,
		},
		{
			table: "project_typeenv_head_selection_closures",
			duplicatePredicate: `(existing.closure_ref = NEW.closure_ref
				OR existing.closure_digest = NEW.closure_digest
				OR existing.authority_use_ref = NEW.authority_use_ref
				OR existing.cas_work_record_ref = NEW.cas_work_record_ref
				OR existing.receipt_ref = NEW.receipt_ref
				OR existing.activation_ref = NEW.activation_ref)`,
		},
	}
}

func projectTypeEnvEffectImmutabilityTriggers47() []string {
	statements := make([]string, 0)
	for _, policy := range projectTypeEnvEffectPolicies47() {
		statements = append(
			statements,
			projectTypeEnvNoReplaceTrigger47(
				policy.table,
				policy.duplicatePredicate,
				"ProjectTypeEnv head-selection history is immutable",
			),
			projectTypeEnvImmutableTrigger47(
				policy.table,
				"update",
				"ProjectTypeEnv head-selection history is immutable",
			),
			projectTypeEnvImmutableTrigger47(
				policy.table,
				"delete",
				"ProjectTypeEnv head-selection history is immutable",
			),
		)
	}
	return statements
}

func projectTypeEnvNoReplaceTrigger47(
	table string,
	duplicatePredicate string,
	message string,
) string {
	return `CREATE TRIGGER ` + table + `_v47_no_insert
		BEFORE INSERT ON ` + table + `
		WHEN EXISTS (
			SELECT 1 FROM ` + table + ` existing
			WHERE ` + duplicatePredicate + `
		)
		BEGIN
			SELECT RAISE(ABORT, '` + message + `');
		END`
}

func projectTypeEnvImmutableTrigger47(
	table string,
	operation string,
	message string,
) string {
	return `CREATE TRIGGER ` + table + `_v47_no_` + operation + `
		BEFORE ` + operation + ` ON ` + table + `
		BEGIN
			SELECT RAISE(ABORT, '` + message + `');
		END`
}

func typedMemoryEventMaterializationFootprintsView47() (string, error) {
	source := typedMemoryEventMaterializationFootprintsView46()
	retractionFragment := `(SELECT COUNT(*) FROM typed_memory_assertion_retractions retraction
			WHERE retraction.project_id = event.project_id
				AND retraction.event_ref = event.event_ref) AS retraction_count,`
	if strings.Count(source, retractionFragment) != 1 {
		return "", fmt.Errorf(
			"v47 footprint view cannot locate exact v46 retraction-count seam",
		)
	}
	activationFragment := retractionFragment + `
		(SELECT COUNT(*) FROM typed_memory_type_env_activations activation
			WHERE activation.project_id = event.project_id
				AND activation.event_ref = event.event_ref) AS type_env_activation_count,`
	source = strings.Replace(source, retractionFragment, activationFragment, 1)
	topLevelTail := `+ (SELECT COUNT(*) FROM typed_memory_assertion_retractions retraction
			WHERE retraction.project_id = event.project_id
				AND retraction.event_ref = event.event_ref) AS top_level_change_count`
	if strings.Count(source, topLevelTail) != 1 {
		return "", fmt.Errorf(
			"v47 footprint view cannot locate exact v46 top-level-count seam",
		)
	}
	topLevelActivationTail := `+ (SELECT COUNT(*) FROM typed_memory_assertion_retractions retraction
			WHERE retraction.project_id = event.project_id
				AND retraction.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_type_env_activations activation
			WHERE activation.project_id = event.project_id
				AND activation.event_ref = event.event_ref) AS top_level_change_count`
	source = strings.Replace(source, topLevelTail, topLevelActivationTail, 1)
	return source, nil
}

func typedMemoryCommitClosureExactFootprintTrigger47() (string, error) {
	source := typedMemoryCommitClosureExactFootprintTrigger46()
	needle := "AND NEW.retraction_count = footprint.retraction_count"
	if strings.Count(source, needle) != 1 {
		return "", fmt.Errorf(
			"v47 closure trigger cannot locate exact v46 retraction-count seam",
		)
	}
	replacement := needle + `
			AND NEW.type_env_activation_count = footprint.type_env_activation_count`
	return strings.Replace(source, needle, replacement, 1), nil
}

func projectTypeEnvEffectTriggerStatements47() []string {
	statements := []string{
		projectTypeEnvRequestExactPredecessorTrigger47(),
		projectTypeEnvModePolicyExactConfigTrigger47(),
		projectTypeEnvAuthorizationContentExactRequestTrigger47(),
		projectTypeEnvTrustedCLISourceExactSourceTrigger47(),
		projectTypeEnvSpeechActExactSourceTrigger47(),
		projectTypeEnvPermissionV3ExactSourceTrigger47(),
		projectTypeEnvAuthorityResolutionBasisExactSourceTrigger47(),
		projectTypeEnvAuthorityResolutionExactBasisTrigger47(),
		projectTypeEnvExplicitPolicyAcceptanceResolutionExactBaseTrigger47(),
		projectTypeEnvStrictPermissionResolutionExactBaseTrigger47(),
		projectTypeEnvAuthorityUseExactSourceTrigger47(),
		projectTypeEnvCASWorkExactUseTrigger47(),
		typedMemoryTypeEnvActivationExactEffectTrigger47(),
		projectTypeEnvHeadHistoryExactEffectTrigger47(),
		projectTypeEnvHeadStateExactCompositeOwnerTrigger47(),
		projectTypeEnvHeadExactCompositeOwnerInsertTrigger47(),
		projectTypeEnvHeadExactCompositeOwnerUpdateTrigger47(),
		projectTypeEnvHeadEffectObligationOnInsertTrigger47(),
		projectTypeEnvHeadEffectObligationOnUpdateTrigger47(),
		projectTypeEnvSelectionReceiptExactEffectTrigger47(),
		projectTypeEnvSelectionClosureExactEffectTrigger47(),
		typedMemoryGraphCommitActivationEffectTrigger47(),
	}
	return append(statements, projectTypeEnvEffectImmutabilityTriggers47()...)
}

func projectTypeEnvHeadStateExactCompositeOwnerTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_states_v47_exact_composite_owner",
		"project_typeenv_head_states",
		`SELECT 1
		FROM project_typeenv_executable_snapshots snapshot
		WHERE snapshot.type_env_ref = NEW.selected_composite_ref`,
		"ProjectTypeEnv head state lacks its exact executable composite owner",
	)
}

func projectTypeEnvHeadExactCompositeOwnerInsertTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_heads_v47_exact_composite_owner_insert",
		"project_typeenv_heads",
		`SELECT 1
		FROM project_typeenv_executable_snapshots snapshot
		WHERE snapshot.type_env_ref = NEW.selected_composite_ref`,
		"ProjectTypeEnv current head lacks its exact executable composite owner",
	)
}

func projectTypeEnvHeadExactCompositeOwnerUpdateTrigger47() string {
	return `CREATE TRIGGER project_typeenv_heads_v47_exact_composite_owner_update
	BEFORE UPDATE ON project_typeenv_heads
	WHEN NOT EXISTS (
		SELECT 1
		FROM project_typeenv_executable_snapshots snapshot
		WHERE snapshot.type_env_ref = NEW.selected_composite_ref
	) BEGIN
		SELECT RAISE(
			ABORT,
			'ProjectTypeEnv current head lacks its exact executable composite owner'
		);
	END`
}

func projectTypeEnvExactInsertTrigger47(
	name string,
	table string,
	exactSourceQuery string,
	message string,
) string {
	return `CREATE TRIGGER ` + name + `
		BEFORE INSERT ON ` + table + `
		WHEN NOT EXISTS (
			` + exactSourceQuery + `
		)
		BEGIN
			SELECT RAISE(ABORT, '` + message + `');
		END`
}

func projectTypeEnvRequestExactPredecessorTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_requests_v47_exact_predecessor",
		"project_typeenv_head_selection_requests",
		`SELECT 1
		FROM project_typeenv_stages stage
		JOIN typed_memory_type_env_coordinates base_coordinate
			ON base_coordinate.type_env_ref = NEW.base_type_env_ref
			AND base_coordinate.representation_kind = 'generic_snapshot'
			AND base_coordinate.generic_snapshot_ref = NEW.base_type_env_ref
		JOIN project_typeenv_artifacts base_artifact
			ON base_artifact.artifact_kind = 'base_type_env'
			AND base_artifact.artifact_ref = NEW.base_type_env_ref
		JOIN project_typeenv_executable_snapshots selected_snapshot
			ON selected_snapshot.type_env_ref = NEW.selected_composite_ref
		WHERE stage.stage_ref = NEW.stage_ref
			AND stage.stage_digest = NEW.stage_digest
			AND stage.project_id = NEW.project_id
			AND stage.executable_type_env_ref = NEW.selected_composite_ref
			AND (
				(NEW.predecessor_kind = 'genesis'
					AND EXISTS (
						SELECT 1
						FROM project_typeenv_no_prior_head_proofs proof
						WHERE proof.proof_ref = NEW.no_prior_head_proof_ref
							AND proof.proof_digest = NEW.no_prior_head_proof_digest
							AND proof.project_id = NEW.project_id
							AND proof.head_ref = NEW.head_ref
							AND proof.expected_graph_revision = NEW.expected_graph_revision
					))
				OR
				(NEW.predecessor_kind = 'transition'
					AND EXISTS (
						SELECT 1
						FROM project_typeenv_head_states prior
						WHERE prior.project_id = NEW.project_id
							AND prior.head_ref = NEW.prior_head_ref
							AND prior.head_revision = NEW.prior_head_revision
							AND prior.selected_composite_ref =
								NEW.prior_selected_composite_ref
					))
			)`,
		"ProjectTypeEnv head-selection request lacks its exact predecessor and Stage",
	)
}

func projectTypeEnvModePolicyExactConfigTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_mode_policies_v47_exact_config",
		"project_typeenv_head_selection_mode_policies",
		`SELECT 1
		FROM project_typeenv_head_selection_config_authority_bases config
		WHERE config.config_authority_basis_ref =
				NEW.config_authority_basis_ref
			AND config.config_authority_basis_digest =
				NEW.config_authority_basis_digest
			AND config.project_id = NEW.project_id
			AND config.authority_mode = NEW.authority_mode`,
		"ProjectTypeEnv head-selection mode policy lacks its exact config basis",
	)
}

func projectTypeEnvAuthorizationContentExactRequestTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_authorization_contents_v47_exact_request",
		"project_typeenv_head_selection_authorization_contents",
		`SELECT 1
		FROM project_typeenv_head_selection_requests request
		WHERE request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
			AND request.project_id = NEW.project_id
			AND request.predecessor_kind = NEW.action_kind`,
		"ProjectTypeEnv head-selection authorization content lacks its exact request",
	)
}

func projectTypeEnvTrustedCLISourceExactSourceTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_trusted_cli_sources_v47_exact_source",
		"project_typeenv_head_selection_trusted_cli_sources",
		`SELECT 1
		FROM project_typeenv_head_selection_mode_policies mode_policy
		JOIN project_typeenv_head_selection_config_authority_bases config
			ON config.config_authority_basis_ref =
				mode_policy.config_authority_basis_ref
			AND config.config_authority_basis_digest =
				mode_policy.config_authority_basis_digest
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
		WHERE mode_policy.mode_policy_ref = NEW.mode_policy_ref
			AND mode_policy.mode_policy_digest = NEW.mode_policy_digest
			AND mode_policy.authority_mode = 'explicit_h_decide'
			AND mode_policy.project_id = NEW.project_id
			AND config.config_authority_basis_ref =
				NEW.config_authority_basis_ref
			AND config.config_authority_basis_digest =
				NEW.config_authority_basis_digest
			AND config.project_id = NEW.project_id
			AND config.authority_mode = 'explicit_h_decide'
			AND content.project_id = NEW.project_id
			AND content.request_ref = request.request_ref
			AND content.request_digest = request.request_digest
			AND request.project_id = NEW.project_id`,
		"trusted dedicated CLI source lacks its exact policy, content, and request",
	)
}

func projectTypeEnvSpeechActExactSourceTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_speech_act_records_v47_exact_source",
		"project_typeenv_head_selection_speech_act_records",
		`SELECT 1
		FROM project_typeenv_head_selection_authorization_contents content
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = content.request_ref
			AND request.request_digest = content.request_digest
		WHERE content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
			AND content.project_id = NEW.project_id
			AND request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
			AND request.project_id = NEW.project_id`,
		"strict SpeechAct record lacks its exact content and request",
	)
}

func projectTypeEnvPermissionV3ExactSourceTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_permissions_v3_v47_exact_source",
		"project_typeenv_head_selection_permissions_v3",
		`SELECT 1
		FROM project_typeenv_head_selection_speech_act_records speech
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
		JOIN project_typeenv_head_selection_mode_policies mode_policy
			ON mode_policy.project_id = NEW.project_id
			AND mode_policy.authority_mode = 'strict_cli_speech_act'
			AND mode_policy.resolver_policy_ref = NEW.context_policy_ref
			AND mode_policy.resolver_policy_digest = NEW.context_policy_digest
		WHERE speech.speech_act_record_ref = NEW.speech_act_record_ref
			AND speech.speech_act_record_digest = NEW.speech_act_record_digest
			AND speech.project_id = NEW.project_id
			AND speech.permission_ref = NEW.permission_ref
			AND speech.permission_digest = NEW.permission_digest
			AND speech.content_ref = NEW.content_ref
			AND speech.content_digest = NEW.content_digest
			AND speech.request_ref = NEW.request_ref
			AND speech.request_digest = NEW.request_digest
			AND content.project_id = NEW.project_id
			AND content.request_ref = request.request_ref
			AND content.request_digest = request.request_digest
			AND request.project_id = NEW.project_id
			AND NEW.subject_context_ref = content.judgement_context_ref
			AND NEW.subject_assignment_from = content.valid_from
			AND NEW.subject_assignment_until = content.valid_until
			AND NEW.subject_authorization_description_kind =
				content.content_ref_kind
			AND NEW.subject_authorization_description_ref = content.content_ref
			AND NEW.subject_authorization_content_digest =
				content.content_digest
			AND NEW.effective_from >= content.valid_from
			AND NEW.validity_until <= content.valid_until`,
		"strict Permission v3 lacks its exact system subject, source, scope, content, and request",
	)
}

func projectTypeEnvAuthorityResolutionBasisExactSourceTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_authority_resolution_bases_v47_exact_source",
		"project_typeenv_head_selection_authority_resolution_bases",
		`SELECT 1
		FROM project_typeenv_head_selection_speech_act_records speech
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
		JOIN project_typeenv_stages stage
			ON stage.stage_ref = NEW.stage_ref
			AND stage.stage_digest = NEW.stage_digest
		JOIN project_typeenv_head_selection_mode_policies mode_policy
			ON mode_policy.project_id = NEW.project_id
			AND mode_policy.authority_mode = 'strict_cli_speech_act'
			AND mode_policy.resolver_policy_ref = NEW.resolver_policy_ref
			AND mode_policy.resolver_policy_edition = NEW.resolver_policy_edition
			AND mode_policy.resolver_policy_digest = NEW.resolver_policy_digest
		WHERE speech.speech_act_record_ref = NEW.speech_act_record_ref
			AND speech.speech_act_record_digest = NEW.speech_act_record_digest
			AND speech.project_id = NEW.project_id
			AND speech.content_ref = NEW.content_ref
			AND speech.content_digest = NEW.content_digest
			AND speech.request_ref = NEW.request_ref
			AND speech.request_digest = NEW.request_digest
			AND content.project_id = NEW.project_id
			AND content.request_ref = request.request_ref
			AND content.request_digest = request.request_digest
			AND request.project_id = NEW.project_id
			AND request.stage_ref = stage.stage_ref
			AND request.stage_digest = stage.stage_digest
			AND stage.project_id = NEW.project_id
			AND NEW.evaluated_at >= content.valid_from
			AND NEW.evaluated_at < content.valid_until`,
		"strict authority-resolution basis lacks its exact policy, source, content, request, and Stage",
	)
}

func projectTypeEnvAuthorityResolutionExactBasisTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_authority_resolutions_v47_exact_basis",
		"project_typeenv_head_selection_authority_resolutions",
		`SELECT 1
		FROM project_typeenv_head_selection_authorization_contents content
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
		WHERE content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
			AND content.project_id = NEW.project_id
			AND content.request_ref = request.request_ref
			AND content.request_digest = request.request_digest
			AND request.project_id = NEW.project_id
			AND NEW.evaluated_at >= content.valid_from
			AND NEW.evaluated_at < content.valid_until
			AND (
				(NEW.authority_resolution_kind = 'explicit_policy_acceptance'
					AND EXISTS (
						SELECT 1
						FROM project_typeenv_head_selection_trusted_cli_sources source
						WHERE source.trusted_cli_source_ref =
								NEW.trusted_cli_source_ref
							AND source.trusted_cli_source_digest =
								NEW.trusted_cli_source_digest
							AND source.project_id = NEW.project_id
							AND source.content_ref = NEW.content_ref
							AND source.content_digest = NEW.content_digest
							AND source.request_ref = NEW.request_ref
							AND source.request_digest = NEW.request_digest
							AND NEW.evaluated_at >= source.recorded_at
					))
				OR
				(NEW.authority_resolution_kind = 'strict_permission'
					AND EXISTS (
						SELECT 1
						FROM project_typeenv_head_selection_authority_resolution_bases basis
						WHERE basis.basis_ref = NEW.strict_basis_ref
							AND basis.basis_digest = NEW.strict_basis_digest
							AND basis.project_id = NEW.project_id
							AND basis.content_ref = NEW.content_ref
							AND basis.content_digest = NEW.content_digest
							AND basis.request_ref = NEW.request_ref
							AND basis.request_digest = NEW.request_digest
							AND basis.evaluated_at = NEW.evaluated_at
					))
			)`,
		"ProjectTypeEnv head-selection authority resolution lacks its exact typed branch",
	)
}

func projectTypeEnvExplicitPolicyAcceptanceResolutionExactBaseTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_explicit_policy_acceptance_resolutions_v47_exact_base",
		"project_typeenv_head_selection_explicit_policy_acceptance_resolutions",
		`SELECT 1
		FROM project_typeenv_head_selection_authority_resolutions resolution
		JOIN project_typeenv_head_selection_trusted_cli_sources source
			ON source.trusted_cli_source_ref = NEW.trusted_cli_source_ref
			AND source.trusted_cli_source_digest = NEW.trusted_cli_source_digest
		WHERE resolution.authority_resolution_ref = NEW.authority_resolution_ref
			AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
			AND resolution.authority_resolution_kind = 'explicit_policy_acceptance'
			AND resolution.trusted_cli_source_ref = NEW.trusted_cli_source_ref
			AND resolution.trusted_cli_source_digest = NEW.trusted_cli_source_digest
			AND resolution.explicit_resolution_ref = NEW.authority_resolution_ref
			AND resolution.explicit_resolution_digest = NEW.authority_resolution_digest
			AND source.project_id = resolution.project_id
			AND source.content_ref = resolution.content_ref
			AND source.content_digest = resolution.content_digest
			AND source.request_ref = resolution.request_ref
			AND source.request_digest = resolution.request_digest`,
		"explicit policy-acceptance resolution lacks its exact base and trusted CLI source",
	)
}

func projectTypeEnvStrictPermissionResolutionExactBaseTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_strict_permission_resolutions_v47_exact_base",
		"project_typeenv_head_selection_strict_permission_resolutions",
		`SELECT 1
		FROM project_typeenv_head_selection_authority_resolutions resolution
		JOIN project_typeenv_head_selection_authority_resolution_bases basis
			ON basis.basis_ref = NEW.basis_ref
			AND basis.basis_digest = NEW.basis_digest
		JOIN project_typeenv_head_selection_speech_act_records speech
			ON speech.speech_act_record_ref = NEW.speech_act_record_ref
			AND speech.speech_act_record_digest = NEW.speech_act_record_digest
		JOIN project_typeenv_head_selection_permissions_v3 permission
			ON permission.permission_ref = NEW.permission_ref
			AND permission.permission_digest = NEW.permission_digest
		WHERE resolution.authority_resolution_ref = NEW.authority_resolution_ref
			AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
			AND resolution.authority_resolution_kind = 'strict_permission'
			AND resolution.strict_basis_ref = NEW.basis_ref
			AND resolution.strict_basis_digest = NEW.basis_digest
			AND resolution.strict_resolution_ref = NEW.authority_resolution_ref
			AND resolution.strict_resolution_digest = NEW.authority_resolution_digest
			AND basis.project_id = resolution.project_id
			AND basis.speech_act_record_ref = speech.speech_act_record_ref
			AND basis.speech_act_record_digest = speech.speech_act_record_digest
			AND basis.content_ref = resolution.content_ref
			AND basis.content_digest = resolution.content_digest
			AND basis.request_ref = resolution.request_ref
			AND basis.request_digest = resolution.request_digest
			AND permission.project_id = resolution.project_id
			AND permission.speech_act_record_ref = speech.speech_act_record_ref
			AND permission.speech_act_record_digest = speech.speech_act_record_digest
			AND permission.content_ref = resolution.content_ref
			AND permission.content_digest = resolution.content_digest
			AND permission.request_ref = resolution.request_ref
			AND permission.request_digest = resolution.request_digest`,
		"strict Permission resolution lacks its exact base, basis, source, and Permission",
	)
}

func projectTypeEnvAuthorityUseExactSourceTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_authority_uses_v47_exact_source",
		"project_typeenv_head_selection_authority_uses",
		`SELECT 1
		FROM project_typeenv_head_selection_authority_resolutions resolution
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
		JOIN project_typeenv_stages stage
			ON stage.stage_ref = NEW.stage_ref
			AND stage.stage_digest = NEW.stage_digest
		WHERE resolution.authority_resolution_ref =
				NEW.authority_resolution_ref
			AND resolution.authority_resolution_digest =
				NEW.authority_resolution_digest
			AND resolution.project_id = NEW.project_id
			AND resolution.authority_resolution_kind =
				NEW.authority_resolution_kind
			AND resolution.content_ref = NEW.content_ref
			AND resolution.content_digest = NEW.content_digest
			AND resolution.request_ref = NEW.request_ref
			AND resolution.request_digest = NEW.request_digest
			AND content.project_id = NEW.project_id
			AND content.request_ref = NEW.request_ref
			AND content.request_digest = NEW.request_digest
			AND request.project_id = NEW.project_id
			AND request.original_idempotency_key =
				NEW.original_idempotency_key
			AND request.predecessor_kind = NEW.predecessor_kind
			AND request.base_type_env_ref = NEW.base_type_env_ref
			AND request.ordered_extension_refs_digest =
				NEW.ordered_extension_refs_digest
			AND request.canonical_ordered_extension_refs =
				NEW.canonical_ordered_extension_refs
			AND request.runtime_evaluation_basis_ref =
				NEW.runtime_evaluation_basis_ref
			AND request.selected_composite_ref =
				NEW.selected_composite_ref
			AND request.stage_ref = NEW.stage_ref
			AND request.stage_digest = NEW.stage_digest
			AND request.expected_graph_revision =
				NEW.expected_graph_revision
			AND stage.project_id = NEW.project_id
			AND stage.executable_type_env_ref =
				NEW.selected_composite_ref
			AND (
				(NEW.authority_resolution_kind = 'explicit_policy_acceptance'
					AND EXISTS (
						SELECT 1
						FROM project_typeenv_head_selection_explicit_policy_acceptance_resolutions child
						WHERE child.authority_resolution_ref =
								NEW.authority_resolution_ref
							AND child.authority_resolution_digest =
								NEW.authority_resolution_digest
					))
				OR
				(NEW.authority_resolution_kind = 'strict_permission'
					AND EXISTS (
						SELECT 1
						FROM project_typeenv_head_selection_strict_permission_resolutions child
						WHERE child.authority_resolution_ref =
								NEW.authority_resolution_ref
							AND child.authority_resolution_digest =
								NEW.authority_resolution_digest
					))
			)
			AND (
				(NEW.predecessor_kind = 'genesis'
					AND request.prior_head_ref IS NULL
					AND request.prior_head_revision IS NULL
					AND request.prior_selected_composite_ref IS NULL
					AND NEW.predecessor_head_ref IS NULL
					AND NEW.predecessor_head_revision IS NULL
					AND NEW.predecessor_selected_composite_ref IS NULL)
				OR
				(NEW.predecessor_kind = 'transition'
					AND request.prior_head_ref =
						NEW.predecessor_head_ref
					AND request.prior_head_revision =
						NEW.predecessor_head_revision
					AND request.prior_selected_composite_ref =
						NEW.predecessor_selected_composite_ref)
			)`,
		"ProjectTypeEnv head-selection authority use lacks its exact resolution and request",
	)
}

func projectTypeEnvCASWorkExactUseTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_cas_work_records_v47_exact_use",
		"project_typeenv_head_cas_work_records",
		`SELECT 1
		FROM project_typeenv_head_selection_authority_uses authority_use
		WHERE authority_use.authority_use_ref = NEW.authority_use_ref
			AND authority_use.authority_use_digest =
				NEW.authority_use_digest
			AND authority_use.project_id = NEW.project_id
			AND authority_use.work_ref = NEW.work_ref
			AND authority_use.receipt_ref = NEW.receipt_ref
			AND authority_use.committed_head_revision =
				NEW.committed_head_revision
			AND authority_use.committed_graph_revision =
				NEW.committed_graph_revision
			AND authority_use.selected_composite_ref =
				NEW.selected_composite_ref`,
		"ProjectTypeEnv head CAS Work record lacks its exact authority use",
	)
}

func typedMemoryTypeEnvActivationExactEffectTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"typed_memory_type_env_activations_v47_exact_effect",
		"typed_memory_type_env_activations",
		`SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_graph_heads graph_head
			ON graph_head.project_id = event.project_id
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
		JOIN project_typeenv_head_selection_authority_uses authority_use
			ON authority_use.authority_use_ref = NEW.authority_use_ref
			AND authority_use.authority_use_digest =
				NEW.authority_use_digest
		JOIN project_typeenv_head_cas_work_records work_record
			ON work_record.work_ref = NEW.work_ref
		JOIN project_typeenv_heads head
			ON head.project_id = NEW.project_id
		JOIN typed_memory_type_env_coordinates basis_coordinate
			ON basis_coordinate.type_env_ref = NEW.basis_type_env_ref
		JOIN typed_memory_type_env_coordinates result_coordinate
			ON result_coordinate.type_env_ref = NEW.result_type_env_ref
			AND result_coordinate.representation_kind = 'project_executable'
			AND result_coordinate.project_executable_ref =
				NEW.result_type_env_ref
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND event.event_kind = 'activate_type_env'
			AND event.authority_class = 'manual_type_env_activation'
			AND event.request_provenance_ref = NEW.request_ref
			AND event.expected_revision = NEW.expected_graph_revision
			AND event.graph_revision = NEW.committed_graph_revision
			AND event.basis_type_env_ref = NEW.basis_type_env_ref
			AND event.result_type_env_ref = NEW.result_type_env_ref
			AND event.change_set_digest = NEW.activation_digest
			AND event.canonical_change_set_bytes =
				NEW.canonical_activation_bytes
			AND event.change_count = 1
			AND graph_head.graph_revision = NEW.expected_graph_revision
			AND graph_head.active_type_env_ref = NEW.basis_type_env_ref
			AND request.project_id = NEW.project_id
			AND request.selected_composite_ref = NEW.result_type_env_ref
			AND request.stage_ref = NEW.stage_ref
			AND request.stage_digest = NEW.stage_digest
			AND request.expected_graph_revision =
				NEW.expected_graph_revision
			AND (
				(request.predecessor_kind = 'genesis'
					AND request.base_type_env_ref =
						NEW.basis_type_env_ref
					AND basis_coordinate.representation_kind =
						'generic_snapshot'
					AND basis_coordinate.generic_snapshot_ref =
						NEW.basis_type_env_ref)
				OR
				(request.predecessor_kind = 'transition'
					AND request.prior_selected_composite_ref =
						NEW.basis_type_env_ref
					AND basis_coordinate.representation_kind =
						'project_executable'
					AND basis_coordinate.project_executable_ref =
						NEW.basis_type_env_ref)
			)
			AND content.project_id = NEW.project_id
			AND content.request_ref = NEW.request_ref
			AND content.request_digest = NEW.request_digest
			AND authority_use.project_id = NEW.project_id
			AND authority_use.request_ref = NEW.request_ref
			AND authority_use.request_digest = NEW.request_digest
			AND authority_use.content_ref = NEW.content_ref
			AND authority_use.content_digest = NEW.content_digest
			AND authority_use.work_ref = NEW.work_ref
			AND authority_use.stage_ref = NEW.stage_ref
			AND authority_use.stage_digest = NEW.stage_digest
			AND authority_use.selected_composite_ref =
				NEW.result_type_env_ref
			AND authority_use.expected_graph_revision =
				NEW.expected_graph_revision
			AND authority_use.committed_graph_revision =
				NEW.committed_graph_revision
			AND authority_use.committed_head_revision =
				NEW.committed_head_revision
			AND work_record.project_id = NEW.project_id
			AND work_record.authority_use_ref =
				NEW.authority_use_ref
			AND work_record.authority_use_digest =
				NEW.authority_use_digest
			AND work_record.activation_ref = NEW.activation_ref
			AND work_record.selected_composite_ref =
				NEW.result_type_env_ref
			AND work_record.committed_graph_revision =
				NEW.committed_graph_revision
			AND work_record.committed_head_revision =
				NEW.committed_head_revision
			AND head.head_ref = NEW.head_ref
			AND head.head_revision = NEW.committed_head_revision
			AND head.selected_composite_ref =
				NEW.result_type_env_ref
			AND NOT EXISTS (
				SELECT 1
				FROM typed_memory_graph_commits commit_record
				WHERE commit_record.project_id = NEW.project_id
					AND commit_record.event_ref = NEW.event_ref
			)`,
		"typed-memory TypeEnv activation lacks its exact open authority effect",
	)
}

func projectTypeEnvHeadHistoryExactEffectTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_history_v47_exact_effect",
		"project_typeenv_head_history",
		`SELECT 1
		FROM typed_memory_type_env_activations activation
		JOIN typed_memory_graph_events event
			ON event.project_id = activation.project_id
			AND event.event_ref = activation.event_ref
		JOIN project_typeenv_heads head
			ON head.project_id = activation.project_id
		JOIN project_typeenv_head_states head_state
			ON head_state.project_id = head.project_id
			AND head_state.head_revision = head.head_revision
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = activation.request_ref
			AND request.request_digest = activation.request_digest
		JOIN project_typeenv_head_selection_authority_uses authority_use
			ON authority_use.authority_use_ref =
				activation.authority_use_ref
			AND authority_use.authority_use_digest =
				activation.authority_use_digest
		JOIN project_typeenv_head_cas_work_records work_record
			ON work_record.work_ref = activation.work_ref
		WHERE activation.project_id = NEW.project_id
			AND activation.activation_ref = NEW.activation_ref
			AND activation.activation_digest = NEW.activation_digest
			AND activation.committed_head_revision =
				NEW.head_revision
			AND activation.committed_graph_revision =
				NEW.graph_revision
			AND activation.result_type_env_ref =
				NEW.selected_composite_ref
			AND event.event_ref = NEW.graph_event_ref
			AND event.commit_ref = NEW.graph_commit_ref
			AND event.graph_revision = NEW.graph_revision
			AND head.head_ref = NEW.head_ref
			AND head.head_revision = NEW.head_revision
			AND head.selected_composite_ref =
				NEW.selected_composite_ref
			AND head.state_digest = NEW.head_state_digest
			AND head.canonical_bytes = NEW.canonical_head_state_bytes
			AND head_state.head_ref = NEW.head_ref
			AND head_state.selected_composite_ref =
				NEW.selected_composite_ref
			AND head_state.state_digest = NEW.head_state_digest
			AND head_state.canonical_bytes =
				NEW.canonical_head_state_bytes
			AND request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
			AND authority_use.authority_use_ref =
				NEW.authority_use_ref
			AND authority_use.authority_use_digest =
				NEW.authority_use_digest
			AND authority_use.work_ref = NEW.work_ref
			AND authority_use.receipt_ref = NEW.receipt_ref
			AND work_record.work_ref = NEW.work_ref
			AND work_record.receipt_ref = NEW.receipt_ref`,
		"ProjectTypeEnv head history lacks its exact activation and head state",
	)
}

func projectTypeEnvHeadEffectObligationOnInsertTrigger47() string {
	return `CREATE TRIGGER project_typeenv_heads_v47_obligation_on_insert
		AFTER INSERT ON project_typeenv_heads
		BEGIN
			INSERT INTO project_typeenv_head_effect_obligations (
				project_id,
				head_ref,
				head_revision,
				selected_composite_ref,
				head_state_digest,
				canonical_head_state_bytes
			) VALUES (
				NEW.project_id,
				NEW.head_ref,
				NEW.head_revision,
				NEW.selected_composite_ref,
				NEW.state_digest,
				NEW.canonical_bytes
			);
		END`
}

func projectTypeEnvHeadEffectObligationOnUpdateTrigger47() string {
	return `CREATE TRIGGER project_typeenv_heads_v47_obligation_on_update
		AFTER UPDATE ON project_typeenv_heads
		BEGIN
			INSERT INTO project_typeenv_head_effect_obligations (
				project_id,
				head_ref,
				head_revision,
				selected_composite_ref,
				head_state_digest,
				canonical_head_state_bytes
			) VALUES (
				NEW.project_id,
				NEW.head_ref,
				NEW.head_revision,
				NEW.selected_composite_ref,
				NEW.state_digest,
				NEW.canonical_bytes
			);
		END`
}

func projectTypeEnvSelectionReceiptExactEffectTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_receipts_v47_exact_effect",
		"project_typeenv_head_selection_receipts",
		`SELECT 1
		FROM project_typeenv_head_selection_authority_uses authority_use
		JOIN project_typeenv_head_cas_work_records work_record
			ON work_record.authority_use_ref =
				authority_use.authority_use_ref
			AND work_record.authority_use_digest =
				authority_use.authority_use_digest
		JOIN typed_memory_type_env_activations activation
			ON activation.project_id = authority_use.project_id
			AND activation.authority_use_ref =
				authority_use.authority_use_ref
			AND activation.authority_use_digest =
				authority_use.authority_use_digest
		JOIN project_typeenv_head_selection_authority_resolutions resolution
			ON resolution.authority_resolution_ref =
				authority_use.authority_resolution_ref
			AND resolution.authority_resolution_digest =
				authority_use.authority_resolution_digest
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = authority_use.content_ref
			AND content.content_digest = authority_use.content_digest
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = authority_use.request_ref
			AND request.request_digest = authority_use.request_digest
		JOIN project_typeenv_head_history history
			ON history.project_id = authority_use.project_id
			AND history.head_revision =
				authority_use.committed_head_revision
		JOIN typed_memory_graph_events event
			ON event.project_id = activation.project_id
			AND event.event_ref = activation.event_ref
		WHERE authority_use.authority_use_ref =
				NEW.authority_use_ref
			AND authority_use.authority_use_digest =
				NEW.authority_use_digest
			AND authority_use.project_id = NEW.project_id
			AND authority_use.work_ref = NEW.work_ref
			AND authority_use.receipt_ref = NEW.receipt_ref
			AND work_record.cas_work_record_ref =
				NEW.cas_work_record_ref
			AND work_record.cas_work_record_digest =
				NEW.cas_work_record_digest
			AND work_record.work_ref = NEW.work_ref
			AND work_record.receipt_ref = NEW.receipt_ref
			AND activation.activation_ref = NEW.activation_ref
			AND activation.activation_digest = NEW.activation_digest
			AND resolution.authority_resolution_ref =
				NEW.authority_resolution_ref
			AND resolution.authority_resolution_digest =
				NEW.authority_resolution_digest
			AND content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
			AND request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
			AND history.head_ref = NEW.head_ref
			AND history.head_revision = NEW.head_revision
			AND history.selected_composite_ref =
				NEW.selected_composite_ref
			AND history.graph_revision = NEW.graph_revision
			AND history.graph_event_ref = NEW.graph_event_ref
			AND history.graph_commit_ref = NEW.graph_commit_ref
			AND history.receipt_ref = NEW.receipt_ref
			AND event.event_ref = NEW.graph_event_ref
			AND event.commit_ref = NEW.graph_commit_ref
			AND event.graph_revision = NEW.graph_revision`,
		"ProjectTypeEnv head-selection receipt lacks its exact effect closure members",
	)
}

func projectTypeEnvSelectionClosureExactEffectTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_closures_v47_exact_effect",
		"project_typeenv_head_selection_closures",
		`SELECT 1
		FROM project_typeenv_head_selection_receipts receipt
		JOIN project_typeenv_head_selection_authority_uses authority_use
			ON authority_use.authority_use_ref =
				receipt.authority_use_ref
			AND authority_use.authority_use_digest =
				receipt.authority_use_digest
		JOIN project_typeenv_head_cas_work_records work_record
			ON work_record.cas_work_record_ref =
				receipt.cas_work_record_ref
			AND work_record.cas_work_record_digest =
				receipt.cas_work_record_digest
		JOIN typed_memory_type_env_activations activation
			ON activation.project_id = receipt.project_id
			AND activation.activation_ref = receipt.activation_ref
		JOIN project_typeenv_head_selection_authority_resolutions resolution
			ON resolution.authority_resolution_ref =
				receipt.authority_resolution_ref
			AND resolution.authority_resolution_digest =
				receipt.authority_resolution_digest
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = receipt.content_ref
			AND content.content_digest = receipt.content_digest
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = receipt.request_ref
			AND request.request_digest = receipt.request_digest
		JOIN project_typeenv_head_history history
			ON history.project_id = receipt.project_id
			AND history.head_revision = receipt.head_revision
		JOIN project_typeenv_heads head
			ON head.project_id = receipt.project_id
		JOIN typed_memory_graph_events event
			ON event.project_id = receipt.project_id
			AND event.event_ref = receipt.graph_event_ref
		WHERE receipt.receipt_ref = NEW.receipt_ref
			AND receipt.receipt_digest = NEW.receipt_digest
			AND receipt.project_id = NEW.project_id
			AND receipt.authority_use_ref =
				NEW.authority_use_ref
			AND receipt.authority_use_digest =
				NEW.authority_use_digest
			AND receipt.cas_work_record_ref =
				NEW.cas_work_record_ref
			AND receipt.cas_work_record_digest =
				NEW.cas_work_record_digest
			AND receipt.activation_ref = NEW.activation_ref
			AND receipt.activation_digest = NEW.activation_digest
			AND receipt.authority_resolution_ref =
				NEW.authority_resolution_ref
			AND receipt.authority_resolution_digest =
				NEW.authority_resolution_digest
			AND receipt.content_ref = NEW.content_ref
			AND receipt.content_digest = NEW.content_digest
			AND receipt.request_ref = NEW.request_ref
			AND receipt.request_digest = NEW.request_digest
			AND receipt.head_ref = NEW.head_ref
			AND receipt.head_revision = NEW.head_revision
			AND receipt.graph_revision = NEW.graph_revision
			AND receipt.graph_event_ref = NEW.graph_event_ref
			AND receipt.graph_commit_ref = NEW.graph_commit_ref
			AND authority_use.authority_use_ref =
				NEW.authority_use_ref
			AND authority_use.authority_use_digest =
				NEW.authority_use_digest
			AND work_record.cas_work_record_ref =
				NEW.cas_work_record_ref
			AND work_record.cas_work_record_digest =
				NEW.cas_work_record_digest
			AND activation.activation_ref = NEW.activation_ref
			AND activation.activation_digest = NEW.activation_digest
			AND resolution.authority_resolution_ref =
				NEW.authority_resolution_ref
			AND resolution.authority_resolution_digest =
				NEW.authority_resolution_digest
			AND content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
			AND request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
			AND history.head_ref = NEW.head_ref
			AND history.head_revision = NEW.head_revision
			AND history.head_state_digest =
				NEW.head_state_digest
			AND history.graph_revision = NEW.graph_revision
			AND history.graph_event_ref = NEW.graph_event_ref
			AND history.graph_commit_ref = NEW.graph_commit_ref
			AND head.head_ref = NEW.head_ref
			AND head.head_revision = NEW.head_revision
			AND head.state_digest = NEW.head_state_digest
			AND event.event_digest = NEW.graph_event_digest
			AND event.commit_ref = NEW.graph_commit_ref
			AND event.graph_revision = NEW.graph_revision`,
		"ProjectTypeEnv head-selection closure does not authenticate its exact members",
	)
}

func typedMemoryGraphCommitActivationEffectTrigger47() string {
	return projectTypeEnvExactInsertTrigger47(
		"typed_memory_graph_commits_v47_activation_effect",
		"typed_memory_graph_commits",
		`SELECT 1
		FROM typed_memory_graph_events event
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND event.commit_ref = NEW.commit_ref
			AND event.event_digest = NEW.event_digest
			AND event.expected_revision = NEW.expected_revision
			AND event.graph_revision = NEW.graph_revision
			AND event.change_set_digest = NEW.change_set_digest
			AND (
				(event.event_kind != 'activate_type_env'
					AND event.authority_class =
						'non_binding_semantic_assertion'
					AND NOT EXISTS (
						SELECT 1
						FROM typed_memory_type_env_activations activation
						WHERE activation.project_id = NEW.project_id
							AND activation.event_ref = NEW.event_ref
					))
				OR
				(event.event_kind = 'activate_type_env'
					AND event.authority_class =
						'manual_type_env_activation'
					AND event.change_count = 1
					AND EXISTS (
						SELECT 1
						FROM typed_memory_type_env_activations activation
						JOIN project_typeenv_head_selection_authority_uses authority_use
							ON authority_use.authority_use_ref =
								activation.authority_use_ref
							AND authority_use.authority_use_digest =
								activation.authority_use_digest
						JOIN project_typeenv_head_cas_work_records work_record
							ON work_record.work_ref =
								activation.work_ref
						JOIN project_typeenv_head_history history
							ON history.project_id =
								activation.project_id
							AND history.head_revision =
								activation.committed_head_revision
						JOIN project_typeenv_head_selection_receipts receipt
							ON receipt.receipt_ref =
								history.receipt_ref
						JOIN project_typeenv_head_selection_closures closure
							ON closure.receipt_ref =
								receipt.receipt_ref
							AND closure.receipt_digest =
								receipt.receipt_digest
						JOIN project_typeenv_heads project_head
							ON project_head.project_id =
								activation.project_id
						JOIN typed_memory_graph_heads graph_head
							ON graph_head.project_id =
								activation.project_id
						JOIN typed_memory_commit_materialization_closures materialization
							ON materialization.project_id =
								activation.project_id
							AND materialization.event_ref =
								activation.event_ref
						WHERE activation.project_id =
								NEW.project_id
							AND activation.event_ref = NEW.event_ref
							AND activation.change_ordinal = 0
							AND activation.activation_digest =
								NEW.change_set_digest
							AND activation.expected_graph_revision =
								NEW.expected_revision
							AND activation.committed_graph_revision =
								NEW.graph_revision
							AND activation.basis_type_env_ref =
								event.basis_type_env_ref
							AND activation.result_type_env_ref =
								event.result_type_env_ref
							AND activation.request_ref =
								event.request_provenance_ref
							AND authority_use.project_id =
								NEW.project_id
							AND authority_use.work_ref =
								activation.work_ref
							AND authority_use.committed_graph_revision =
								NEW.graph_revision
							AND authority_use.committed_head_revision =
								activation.committed_head_revision
							AND work_record.project_id =
								NEW.project_id
							AND work_record.activation_ref =
								activation.activation_ref
							AND work_record.committed_graph_revision =
								NEW.graph_revision
							AND work_record.committed_head_revision =
								activation.committed_head_revision
							AND history.graph_revision =
								NEW.graph_revision
							AND history.graph_event_ref =
								NEW.event_ref
							AND history.graph_commit_ref =
								NEW.commit_ref
							AND history.activation_ref =
								activation.activation_ref
							AND history.authority_use_ref =
								authority_use.authority_use_ref
							AND history.work_ref =
								work_record.work_ref
							AND receipt.project_id =
								NEW.project_id
							AND receipt.authority_use_ref =
								authority_use.authority_use_ref
							AND receipt.cas_work_record_ref =
								work_record.cas_work_record_ref
							AND receipt.activation_ref =
								activation.activation_ref
							AND receipt.graph_revision =
								NEW.graph_revision
							AND receipt.graph_event_ref =
								NEW.event_ref
							AND receipt.graph_commit_ref =
								NEW.commit_ref
							AND closure.project_id =
								NEW.project_id
							AND closure.authority_use_ref =
								authority_use.authority_use_ref
							AND closure.cas_work_record_ref =
								work_record.cas_work_record_ref
							AND closure.activation_ref =
								activation.activation_ref
							AND closure.graph_revision =
								NEW.graph_revision
							AND closure.graph_event_ref =
								NEW.event_ref
							AND closure.graph_commit_ref =
								NEW.commit_ref
							AND project_head.head_ref =
								activation.head_ref
							AND project_head.head_revision =
								activation.committed_head_revision
							AND project_head.selected_composite_ref =
								activation.result_type_env_ref
							AND graph_head.graph_revision =
								NEW.expected_revision
							AND graph_head.active_type_env_ref =
								activation.basis_type_env_ref
							AND materialization.commit_ref =
								NEW.commit_ref
							AND materialization.event_digest =
								NEW.event_digest
							AND materialization.admission_basis_kind =
								'snapshot_only'
							AND materialization.request_digest =
								activation.request_digest
							AND materialization.semantic_digest =
								activation.activation_digest
							AND materialization.entity_count = 0
							AND materialization.entity_context_count = 0
							AND materialization.entity_declaration_count = 0
							AND materialization.context_slice_catalog_count = 0
							AND materialization.context_slice_count = 0
							AND materialization.value_blob_count = 0
							AND materialization.observable_input_blob_count = 0
							AND materialization.relation_count = 0
							AND materialization.relation_slot_count = 0
							AND materialization.relation_filler_count = 0
							AND materialization.ordered_candidate_prefix_count = 0
							AND materialization.reference_resolution_use_count = 0
							AND materialization.memberof_evaluation_count = 0
							AND materialization.memberof_input_count = 0
							AND materialization.memberof_use_count = 0
							AND materialization.alias_change_count = 0
							AND materialization.retraction_count = 0
							AND materialization.type_env_activation_count = 1
					))
			)`,
		"typed-memory graph commit lacks its exact TypeEnv activation effect",
	)
}

func verifyProjectTypeEnvHeadSelectionFootprint47(
	tx MigrationTransaction,
) error {
	coordinateObjects, err := typedMemoryTypeEnvCoordinateObjects47()
	if err != nil {
		return err
	}
	expectedProjectObjects := make(map[string]sqliteSchemaObject47)
	mergeSchemaObjects47(
		expectedProjectObjects,
		projectTypeEnvArtifactCandidateObjects47(),
	)
	mergeSchemaObjects47(
		expectedProjectObjects,
		projectTypeEnvStageCandidateObjects47(),
	)
	mergeSchemaObjects47(
		expectedProjectObjects,
		projectTypeEnvHeadStoreObjects47(),
	)
	effectObjects := schemaObjectsFromStatements47(
		append(
			append(
				append(
					[]string{},
					projectTypeEnvEffectTableStatements47()...,
				),
				projectTypeEnvEffectIndexStatements47()...,
			),
			projectTypeEnvEffectTriggerStatements47()...,
		),
	)
	mergeSchemaObjects47(expectedProjectObjects, effectObjects)
	candidateTriggerObjects := schemaObjectsFromStatements47(
		projectTypeEnvCandidateAnnexTriggers47(),
	)
	mergeSchemaObjects47(expectedProjectObjects, candidateTriggerObjects)
	for key, object := range coordinateObjects {
		if strings.HasPrefix(object.name, "project_typeenv_") ||
			strings.HasPrefix(object.name, "idx_project_typeenv_") {
			expectedProjectObjects[key] = object
		}
	}
	if err := requireExactSQLiteObjects47(
		tx,
		expectedProjectObjects,
		"ProjectTypeEnv v47 footprint",
	); err != nil {
		return err
	}
	if err := requireNoUnknownProjectTypeEnvObjects47(
		tx,
		expectedProjectObjects,
	); err != nil {
		return err
	}
	if err := requireExactCandidateSchemaVersion47(
		tx,
		"project_typeenv_artifact_store_schema",
		2,
	); err != nil {
		return err
	}
	if err := requireExactCandidateSchemaVersion47(
		tx,
		"project_typeenv_stage_store_schema",
		2,
	); err != nil {
		return err
	}
	if err := requireExactCandidateSchemaVersion47(
		tx,
		"project_typeenv_head_store_schema",
		1,
	); err != nil {
		return err
	}
	if err := requireExactTypeEnvActivationColumn47(tx); err != nil {
		return err
	}
	view, err := typedMemoryEventMaterializationFootprintsView47()
	if err != nil {
		return err
	}
	closureTrigger, err := typedMemoryCommitClosureExactFootprintTrigger47()
	if err != nil {
		return err
	}
	expectedTypedMemoryObjects := make(map[string]sqliteSchemaObject47)
	for key, object := range coordinateObjects {
		if strings.HasPrefix(object.name, "typed_memory_") ||
			strings.HasPrefix(object.name, "idx_typed_memory_") {
			expectedTypedMemoryObjects[key] = object
		}
	}
	v47TypedMemoryObjects := schemaObjectsFromStatements47(
		append(
			[]string{view, closureTrigger},
			projectTypeEnvEffectTriggerStatements47()...,
		),
	)
	activationObjects := schemaObjectsFromStatements47([]string{
		typedMemoryTypeEnvActivationsTable47(),
		`CREATE UNIQUE INDEX idx_project_typeenv_activations_revision_v47
			ON typed_memory_type_env_activations(
				project_id,
				committed_graph_revision
			)`,
	})
	mergeSchemaObjects47(expectedTypedMemoryObjects, v47TypedMemoryObjects)
	mergeSchemaObjects47(expectedTypedMemoryObjects, activationObjects)
	for key, object := range expectedTypedMemoryObjects {
		if !strings.HasPrefix(object.name, "typed_memory_") &&
			!strings.HasPrefix(object.name, "idx_typed_memory_") {
			delete(expectedTypedMemoryObjects, key)
		}
	}
	if err := requireExactSQLiteObjects47(
		tx,
		expectedTypedMemoryObjects,
		"typed-memory v47 extension footprint",
	); err != nil {
		return err
	}
	if err := requireExactTypeEnvForeignKeyTargets47(tx); err != nil {
		return err
	}
	if err := requireNoUnexpectedP8GBackfill47(tx); err != nil {
		return err
	}
	return nil
}

func requireExactCandidateSchemaVersion47(
	tx MigrationTransaction,
	table string,
	expectedVersion int,
) error {
	var rowCount int
	var matchingCount int
	err := tx.QueryRow(
		"SELECT COUNT(*), COALESCE(SUM(CASE WHEN singleton = 1 AND version = ? THEN 1 ELSE 0 END), 0) FROM "+
			quoteSQLiteIdentifier(table),
		expectedVersion,
	).Scan(&rowCount, &matchingCount)
	if err != nil {
		return fmt.Errorf("verify %s schema version: %w", table, err)
	}
	if rowCount != 1 || matchingCount != 1 {
		return fmt.Errorf(
			"%s requires one exact schema-version row at version %d",
			table,
			expectedVersion,
		)
	}
	return nil
}

func requireExactTypeEnvActivationColumn47(
	tx MigrationTransaction,
) error {
	rows, err := tx.Query(
		"PRAGMA table_xinfo(typed_memory_commit_materialization_closures)",
	)
	if err != nil {
		return fmt.Errorf("inspect v47 activation-count column: %w", err)
	}
	defer rows.Close()
	found := 0
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		var hidden int
		if err := rows.Scan(
			&cid,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
			&hidden,
		); err != nil {
			return fmt.Errorf("scan v47 activation-count column: %w", err)
		}
		if name != "type_env_activation_count" {
			continue
		}
		found++
		if strings.ToUpper(columnType) != "INTEGER" ||
			notNull != 1 ||
			!defaultValue.Valid ||
			strings.Trim(defaultValue.String, "()'\" ") != "0" ||
			primaryKey != 0 ||
			hidden != 0 {
			return fmt.Errorf(
				"typed-memory activation-count column has unknown shape",
			)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate v47 activation-count columns: %w", err)
	}
	if found != 1 {
		return fmt.Errorf(
			"typed-memory activation-count column requires one exact occurrence",
		)
	}
	return nil
}

type typeEnvForeignKeyTarget47 struct {
	table        string
	column       string
	parentTable  string
	parentColumn string
}

func requireExactTypeEnvForeignKeyTargets47(
	tx MigrationTransaction,
) error {
	targets := []typeEnvForeignKeyTarget47{
		{
			table:        "typed_memory_type_env_coordinates",
			column:       "generic_snapshot_ref",
			parentTable:  "typed_memory_type_env_snapshots",
			parentColumn: "type_env_ref",
		},
		{
			table:        "typed_memory_type_env_coordinates",
			column:       "project_executable_ref",
			parentTable:  "project_typeenv_executable_snapshots",
			parentColumn: "type_env_ref",
		},
		{
			table:        "typed_memory_graph_heads",
			column:       "active_type_env_ref",
			parentTable:  "typed_memory_type_env_coordinates",
			parentColumn: "type_env_ref",
		},
		{
			table:        "typed_memory_graph_events",
			column:       "basis_type_env_ref",
			parentTable:  "typed_memory_type_env_coordinates",
			parentColumn: "type_env_ref",
		},
		{
			table:        "typed_memory_graph_events",
			column:       "result_type_env_ref",
			parentTable:  "typed_memory_type_env_coordinates",
			parentColumn: "type_env_ref",
		},
		{
			table:        "typed_memory_event_admission_bases",
			column:       "type_env_ref",
			parentTable:  "typed_memory_type_env_coordinates",
			parentColumn: "type_env_ref",
		},
		{
			table:        "project_typeenv_head_selection_requests",
			column:       "base_type_env_ref",
			parentTable:  "typed_memory_type_env_snapshots",
			parentColumn: "type_env_ref",
		},
		{
			table:        "project_typeenv_head_selection_requests",
			column:       "selected_composite_ref",
			parentTable:  "project_typeenv_executable_snapshots",
			parentColumn: "type_env_ref",
		},
		{
			table:        "project_typeenv_head_selection_authority_uses",
			column:       "base_type_env_ref",
			parentTable:  "typed_memory_type_env_snapshots",
			parentColumn: "type_env_ref",
		},
		{
			table:        "project_typeenv_head_selection_authority_uses",
			column:       "selected_composite_ref",
			parentTable:  "project_typeenv_executable_snapshots",
			parentColumn: "type_env_ref",
		},
		{
			table:        "typed_memory_type_env_activations",
			column:       "basis_type_env_ref",
			parentTable:  "typed_memory_type_env_coordinates",
			parentColumn: "type_env_ref",
		},
		{
			table:        "typed_memory_type_env_activations",
			column:       "result_type_env_ref",
			parentTable:  "project_typeenv_executable_snapshots",
			parentColumn: "type_env_ref",
		},
		{
			table:        "project_typeenv_head_selection_receipts",
			column:       "selected_composite_ref",
			parentTable:  "project_typeenv_executable_snapshots",
			parentColumn: "type_env_ref",
		},
	}
	for _, target := range targets {
		if err := requireExactForeignKeyTarget47(tx, target); err != nil {
			return err
		}
	}
	return nil
}

func requireExactForeignKeyTarget47(
	tx MigrationTransaction,
	target typeEnvForeignKeyTarget47,
) error {
	rows, err := tx.Query(
		"PRAGMA foreign_key_list(" + quoteSQLiteIdentifier(target.table) + ")",
	)
	if err != nil {
		return fmt.Errorf(
			"inspect %s.%s foreign key: %w",
			target.table,
			target.column,
			err,
		)
	}
	defer rows.Close()
	matches := 0
	for rows.Next() {
		var id int
		var sequence int
		var parentTable string
		var column string
		var parentColumn string
		var onUpdate string
		var onDelete string
		var match string
		if err := rows.Scan(
			&id,
			&sequence,
			&parentTable,
			&column,
			&parentColumn,
			&onUpdate,
			&onDelete,
			&match,
		); err != nil {
			return fmt.Errorf(
				"scan %s.%s foreign key: %w",
				target.table,
				target.column,
				err,
			)
		}
		if column == target.column &&
			parentTable == target.parentTable &&
			parentColumn == target.parentColumn {
			matches++
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf(
			"iterate %s.%s foreign keys: %w",
			target.table,
			target.column,
			err,
		)
	}
	if matches != 1 {
		return fmt.Errorf(
			"%s.%s requires one exact foreign key to %s.%s; found %d",
			target.table,
			target.column,
			target.parentTable,
			target.parentColumn,
			matches,
		)
	}
	return nil
}

func requireNoUnexpectedP8GBackfill47(
	tx MigrationTransaction,
) error {
	for _, table := range projectTypeEnvEffectTables47 {
		var rowCount int
		err := tx.QueryRow(
			"SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table),
		).Scan(&rowCount)
		if err != nil {
			return fmt.Errorf("inspect v47 backfill at %s: %w", table, err)
		}
		if rowCount != 0 {
			return fmt.Errorf(
				"migration 47 must not backfill %s; found %d row(s)",
				table,
				rowCount,
			)
		}
	}
	var nonZeroLegacyClosures int
	err := tx.QueryRow(
		`SELECT COUNT(*)
		FROM typed_memory_commit_materialization_closures
		WHERE type_env_activation_count != 0`,
	).Scan(&nonZeroLegacyClosures)
	if err != nil {
		return fmt.Errorf("inspect v47 legacy activation counts: %w", err)
	}
	if nonZeroLegacyClosures != 0 {
		return fmt.Errorf(
			"migration 47 fabricated TypeEnv activation counts for legacy closures",
		)
	}
	return nil
}
