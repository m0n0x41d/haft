package db

import "fmt"

var typedMemoryStorageMigration45 = Migration{
	Version:     45,
	Description: "Add empty transactional typed-memory graph storage",
	Apply:       applyTypedMemoryStorageMigration45,
}

var typedMemoryStorageTables45 = []string{
	"typed_memory_type_env_snapshots",
	"typed_memory_graph_heads",
	"typed_memory_graph_commits",
	"typed_memory_graph_events",
	"typed_memory_entities",
	"typed_memory_entity_contexts",
	"typed_memory_idempotency_history",
	"typed_memory_projection_jobs",
	"typed_memory_projection_debt_events",
}

func applyTypedMemoryStorageMigration45(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireTypedMemoryStorageSource45(tx); err != nil {
		return err
	}
	if err := requireAbsentTypedMemoryStorageFootprint45(tx, 0); err != nil {
		return err
	}
	statements := typedMemoryStorageStatements45()
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install transactional typed-memory graph storage: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify transactional typed-memory graph storage: %w", err)
	}
	return nil
}

func requireTypedMemoryStorageSource45(tx MigrationTransaction) error {
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 44",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect typed-memory storage source migration: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("typed-memory storage requires schema version 44")
	}
	return nil
}

func requireAbsentTypedMemoryStorageFootprint45(
	tx MigrationTransaction,
	index int,
) error {
	if index >= len(typedMemoryStorageTables45) {
		return nil
	}
	table := typedMemoryStorageTables45[index]
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect typed-memory storage table %s: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"typed-memory storage refused: unversioned table %s already exists; unknown partial schema requires manual review",
			table,
		)
	}
	return requireAbsentTypedMemoryStorageFootprint45(tx, index+1)
}

func typedMemoryStorageStatements45() []string {
	statements := []string{
		typedMemoryTypeEnvSnapshotsTable45(),
		typedMemoryGraphHeadsTable45(),
		typedMemoryGraphCommitsTable45(),
		typedMemoryGraphEventsTable45(),
		typedMemoryEntitiesTable45(),
		typedMemoryEntityContextsTable45(),
		typedMemoryIdempotencyTable45(),
		typedMemoryProjectionJobsTable45(),
		typedMemoryProjectionDebtTable45(),
		"CREATE INDEX idx_typed_memory_events_project_revision ON typed_memory_graph_events(project_id, graph_revision DESC)",
		"CREATE INDEX idx_typed_memory_entity_contexts_context ON typed_memory_entity_contexts(project_id, bounded_context_ref, entity_id)",
		"CREATE INDEX idx_typed_memory_projection_jobs_project_revision ON typed_memory_projection_jobs(project_id, graph_revision)",
		"CREATE INDEX idx_typed_memory_projection_debt_history ON typed_memory_projection_debt_events(project_id, debt_ref, recorded_at)",
		"CREATE UNIQUE INDEX idx_typed_memory_projection_debt_resolved_once ON typed_memory_projection_debt_events(project_id, supersedes_debt_event_ref) WHERE supersedes_debt_event_ref IS NOT NULL",
		typedMemoryTypeEnvNoReplaceTrigger45(),
		immutableTypedMemoryTrigger45("typed_memory_type_env_snapshots", "update"),
		immutableTypedMemoryTrigger45("typed_memory_type_env_snapshots", "delete"),
		typedMemoryGraphHeadNoReplaceTrigger45(),
		typedMemoryGraphHeadGenesisTrigger45(),
		typedMemoryGraphHeadCASTrigger45(),
		immutableTypedMemoryTrigger45("typed_memory_graph_heads", "delete"),
		typedMemoryGraphEventExactHeadTrigger45(),
		immutableTypedMemoryTrigger45("typed_memory_graph_events", "update"),
		immutableTypedMemoryTrigger45("typed_memory_graph_events", "delete"),
		typedMemoryEntityExactEventTrigger45(),
		immutableTypedMemoryTrigger45("typed_memory_entities", "update"),
		immutableTypedMemoryTrigger45("typed_memory_entities", "delete"),
		typedMemoryEntityContextExactEventTrigger45(),
		immutableTypedMemoryTrigger45("typed_memory_entity_contexts", "update"),
		immutableTypedMemoryTrigger45("typed_memory_entity_contexts", "delete"),
		typedMemoryIdempotencyExactEventTrigger45(),
		immutableTypedMemoryTrigger45("typed_memory_idempotency_history", "update"),
		immutableTypedMemoryTrigger45("typed_memory_idempotency_history", "delete"),
		typedMemoryProjectionJobExactEventTrigger45(),
		immutableTypedMemoryTrigger45("typed_memory_projection_jobs", "update"),
		immutableTypedMemoryTrigger45("typed_memory_projection_jobs", "delete"),
		typedMemoryGraphCommitNoReplaceTrigger45(),
		typedMemoryGraphCommitExactClosureTrigger45(),
		typedMemoryGraphCommitAdvanceHeadTrigger45(),
		immutableTypedMemoryTrigger45("typed_memory_graph_commits", "update"),
		immutableTypedMemoryTrigger45("typed_memory_graph_commits", "delete"),
		typedMemoryProjectionDebtExactJobTrigger45(),
		typedMemoryProjectionDebtResolutionTrigger45(),
		immutableTypedMemoryTrigger45("typed_memory_projection_debt_events", "update"),
		immutableTypedMemoryTrigger45("typed_memory_projection_debt_events", "delete"),
	}
	return statements
}

func typedMemoryTypeEnvSnapshotsTable45() string {
	return `CREATE TABLE typed_memory_type_env_snapshots (
		type_env_ref TEXT PRIMARY KEY CHECK(
			length(type_env_ref) = 79
			AND substr(type_env_ref, 1, 15) = 'typeenv:sha256:'
		),
		artifact_digest TEXT NOT NULL UNIQUE CHECK(
			length(artifact_digest) = 71
			AND substr(artifact_digest, 1, 7) = 'sha256:'
		),
		snapshot_format TEXT NOT NULL CHECK(snapshot_format != '' AND trim(snapshot_format) = snapshot_format),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		source_revision TEXT NOT NULL CHECK(source_revision != '' AND trim(source_revision) = source_revision),
		compiler_schema_version TEXT NOT NULL CHECK(compiler_schema_version != '' AND trim(compiler_schema_version) = compiler_schema_version),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		CHECK(type_env_ref = 'typeenv:' || artifact_digest)
	) WITHOUT ROWID`
}

func typedMemoryGraphHeadsTable45() string {
	return `CREATE TABLE typed_memory_graph_heads (
		project_id TEXT PRIMARY KEY REFERENCES project_ledger_binding(project_id),
		graph_revision INTEGER NOT NULL CHECK(graph_revision >= 0),
		active_type_env_ref TEXT NOT NULL REFERENCES typed_memory_type_env_snapshots(type_env_ref),
		last_event_ref TEXT NOT NULL DEFAULT '',
		last_commit_ref TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("updated_at") + `),
		CHECK(
			(graph_revision = 0 AND last_event_ref = '' AND last_commit_ref = '')
			OR (graph_revision > 0 AND last_event_ref != '' AND last_commit_ref != '')
		)
	) WITHOUT ROWID`
}

func typedMemoryGraphCommitsTable45() string {
	return `CREATE TABLE typed_memory_graph_commits (
		project_id TEXT NOT NULL REFERENCES typed_memory_graph_heads(project_id),
		commit_ref TEXT NOT NULL CHECK(commit_ref != '' AND trim(commit_ref) = commit_ref),
		event_ref TEXT NOT NULL,
		event_digest TEXT NOT NULL CHECK(length(event_digest) = 71 AND substr(event_digest, 1, 7) = 'sha256:'),
		expected_revision INTEGER NOT NULL CHECK(expected_revision >= 0),
		graph_revision INTEGER NOT NULL CHECK(graph_revision = expected_revision + 1),
		change_set_digest TEXT NOT NULL CHECK(length(change_set_digest) = 71 AND substr(change_set_digest, 1, 7) = 'sha256:'),
		idempotency_key TEXT NOT NULL CHECK(length(idempotency_key) BETWEEN 1 AND 512 AND trim(idempotency_key) = idempotency_key),
		projection_job_ref TEXT NOT NULL,
		entity_count INTEGER NOT NULL CHECK(entity_count >= 0),
		entity_context_count INTEGER NOT NULL CHECK(entity_context_count >= 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, commit_ref),
		UNIQUE(project_id, event_ref),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(project_id, idempotency_key)
			REFERENCES typed_memory_idempotency_history(project_id, idempotency_key),
		FOREIGN KEY(project_id, projection_job_ref)
			REFERENCES typed_memory_projection_jobs(project_id, projection_job_ref)
	) WITHOUT ROWID`
}

func typedMemoryGraphEventsTable45() string {
	return `CREATE TABLE typed_memory_graph_events (
		project_id TEXT NOT NULL REFERENCES typed_memory_graph_heads(project_id),
		event_ref TEXT NOT NULL CHECK(event_ref != '' AND trim(event_ref) = event_ref),
		commit_ref TEXT NOT NULL CHECK(commit_ref != '' AND trim(commit_ref) = commit_ref),
		event_digest TEXT NOT NULL UNIQUE CHECK(length(event_digest) = 71 AND substr(event_digest, 1, 7) = 'sha256:'),
		expected_revision INTEGER NOT NULL CHECK(expected_revision >= 0),
		graph_revision INTEGER NOT NULL CHECK(graph_revision = expected_revision + 1),
		basis_type_env_ref TEXT NOT NULL REFERENCES typed_memory_type_env_snapshots(type_env_ref),
		result_type_env_ref TEXT NOT NULL REFERENCES typed_memory_type_env_snapshots(type_env_ref),
		change_set_digest TEXT NOT NULL CHECK(length(change_set_digest) = 71 AND substr(change_set_digest, 1, 7) = 'sha256:'),
		canonical_change_set_bytes BLOB NOT NULL CHECK(length(canonical_change_set_bytes) > 0),
		change_count INTEGER NOT NULL CHECK(change_count > 0),
		event_kind TEXT NOT NULL CHECK(event_kind IN (
			'declare_entity', 'admit_alias', 'supersede_alias',
			'merge_entities', 'split_entity', 'instantiate_relation',
			'retract_assertion', 'mixed_change_set', 'activate_type_env'
		)),
		authority_class TEXT NOT NULL CHECK(authority_class IN (
			'non_binding_semantic_assertion', 'manual_type_env_activation'
		)),
		request_provenance_ref TEXT NOT NULL CHECK(request_provenance_ref != '' AND trim(request_provenance_ref) = request_provenance_ref),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, event_ref),
		UNIQUE(project_id, graph_revision),
		UNIQUE(project_id, commit_ref),
		FOREIGN KEY(project_id, commit_ref)
			REFERENCES typed_memory_graph_commits(project_id, commit_ref)
			DEFERRABLE INITIALLY DEFERRED,
		CHECK(
			(event_kind = 'activate_type_env' AND basis_type_env_ref != result_type_env_ref)
			OR (event_kind != 'activate_type_env' AND basis_type_env_ref = result_type_env_ref)
		)
	) WITHOUT ROWID`
}

func typedMemoryEntitiesTable45() string {
	return `CREATE TABLE typed_memory_entities (
		project_id TEXT NOT NULL REFERENCES typed_memory_graph_heads(project_id),
		entity_id TEXT NOT NULL CHECK(entity_id != '' AND trim(entity_id) = entity_id),
		first_declared_event_ref TEXT NOT NULL,
		first_declared_revision INTEGER NOT NULL CHECK(first_declared_revision > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, entity_id),
		FOREIGN KEY(project_id, first_declared_event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryEntityContextsTable45() string {
	return `CREATE TABLE typed_memory_entity_contexts (
		project_id TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		bounded_context_ref TEXT NOT NULL CHECK(bounded_context_ref != '' AND trim(bounded_context_ref) = bounded_context_ref),
		label TEXT NOT NULL CHECK(label != '' AND trim(label) = label),
		provenance_ref TEXT NOT NULL CHECK(provenance_ref != '' AND trim(provenance_ref) = provenance_ref),
		declared_event_ref TEXT NOT NULL,
		declared_revision INTEGER NOT NULL CHECK(declared_revision > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, entity_id, bounded_context_ref),
		FOREIGN KEY(project_id, entity_id)
			REFERENCES typed_memory_entities(project_id, entity_id),
		FOREIGN KEY(project_id, declared_event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryIdempotencyTable45() string {
	return `CREATE TABLE typed_memory_idempotency_history (
		project_id TEXT NOT NULL REFERENCES typed_memory_graph_heads(project_id),
		idempotency_key TEXT NOT NULL CHECK(
			length(idempotency_key) BETWEEN 1 AND 512
			AND trim(idempotency_key) = idempotency_key
		),
		change_set_digest TEXT NOT NULL CHECK(length(change_set_digest) = 71 AND substr(change_set_digest, 1, 7) = 'sha256:'),
		event_ref TEXT NOT NULL,
		graph_revision INTEGER NOT NULL CHECK(graph_revision > 0),
		result_digest TEXT NOT NULL CHECK(length(result_digest) = 71 AND substr(result_digest, 1, 7) = 'sha256:'),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, idempotency_key),
		UNIQUE(project_id, event_ref),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryProjectionJobsTable45() string {
	return `CREATE TABLE typed_memory_projection_jobs (
		project_id TEXT NOT NULL REFERENCES typed_memory_graph_heads(project_id),
		projection_job_ref TEXT NOT NULL CHECK(projection_job_ref != '' AND trim(projection_job_ref) = projection_job_ref),
		semantic_event_ref TEXT NOT NULL,
		graph_revision INTEGER NOT NULL CHECK(graph_revision > 0),
		target_kind TEXT NOT NULL CHECK(target_kind = 'project_carriers'),
		input_event_digest TEXT NOT NULL CHECK(length(input_event_digest) = 71 AND substr(input_event_digest, 1, 7) = 'sha256:'),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, projection_job_ref),
		UNIQUE(project_id, semantic_event_ref),
		FOREIGN KEY(project_id, semantic_event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryProjectionDebtTable45() string {
	return `CREATE TABLE typed_memory_projection_debt_events (
		project_id TEXT NOT NULL,
		debt_event_ref TEXT NOT NULL CHECK(debt_event_ref != '' AND trim(debt_event_ref) = debt_event_ref),
		debt_ref TEXT NOT NULL CHECK(debt_ref != '' AND trim(debt_ref) = debt_ref),
		projection_job_ref TEXT NOT NULL,
		semantic_event_ref TEXT NOT NULL,
		graph_revision INTEGER NOT NULL CHECK(graph_revision > 0),
		event_kind TEXT NOT NULL CHECK(event_kind IN ('opened', 'resolved')),
		reason_code TEXT NOT NULL CHECK(reason_code != '' AND trim(reason_code) = reason_code),
		detail TEXT NOT NULL CHECK(detail != '' AND trim(detail) = detail),
		expected_projection_digest TEXT NOT NULL CHECK(length(expected_projection_digest) = 71 AND substr(expected_projection_digest, 1, 7) = 'sha256:'),
		observed_projection_digest TEXT NOT NULL DEFAULT '' CHECK(
			observed_projection_digest = ''
			OR (length(observed_projection_digest) = 71 AND substr(observed_projection_digest, 1, 7) = 'sha256:')
		),
		supersedes_debt_event_ref TEXT,
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, debt_event_ref),
		FOREIGN KEY(project_id, projection_job_ref)
			REFERENCES typed_memory_projection_jobs(project_id, projection_job_ref),
		CHECK(
			(event_kind = 'opened' AND supersedes_debt_event_ref IS NULL AND observed_projection_digest = '')
			OR (event_kind = 'resolved' AND supersedes_debt_event_ref IS NOT NULL AND observed_projection_digest != '')
		)
	) WITHOUT ROWID`
}

func typedMemoryTypeEnvNoReplaceTrigger45() string {
	return `CREATE TRIGGER typed_memory_type_env_snapshots_no_replace
	BEFORE INSERT ON typed_memory_type_env_snapshots
	WHEN EXISTS (
		SELECT 1 FROM typed_memory_type_env_snapshots existing
		WHERE existing.type_env_ref = NEW.type_env_ref
			OR existing.artifact_digest = NEW.artifact_digest
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory TypeEnv snapshots are immutable');
	END`
}

func typedMemoryGraphHeadNoReplaceTrigger45() string {
	return `CREATE TRIGGER typed_memory_graph_heads_no_replace
	BEFORE INSERT ON typed_memory_graph_heads
	WHEN EXISTS (SELECT 1 FROM typed_memory_graph_heads existing WHERE existing.project_id = NEW.project_id) BEGIN
		SELECT RAISE(ABORT, 'typed-memory graph head already exists');
	END`
}

func typedMemoryGraphHeadGenesisTrigger45() string {
	return `CREATE TRIGGER typed_memory_graph_heads_genesis_only
	BEFORE INSERT ON typed_memory_graph_heads
	WHEN NEW.graph_revision != 0 OR NEW.last_event_ref != '' OR NEW.last_commit_ref != '' BEGIN
		SELECT RAISE(ABORT, 'typed-memory graph head must begin at revision zero without fabricated history');
	END`
}

func typedMemoryGraphHeadCASTrigger45() string {
	return `CREATE TRIGGER typed_memory_graph_heads_revision_cas
	BEFORE UPDATE ON typed_memory_graph_heads
	WHEN NEW.project_id != OLD.project_id
		OR NEW.graph_revision != OLD.graph_revision + 1
		OR NOT EXISTS (
			SELECT 1
			FROM typed_memory_graph_commits commit_record
			JOIN typed_memory_graph_events event
				ON event.project_id = commit_record.project_id
				AND event.event_ref = commit_record.event_ref
			WHERE commit_record.project_id = OLD.project_id
				AND commit_record.commit_ref = NEW.last_commit_ref
				AND commit_record.expected_revision = OLD.graph_revision
				AND commit_record.graph_revision = NEW.graph_revision
				AND event.event_ref = NEW.last_event_ref
				AND event.expected_revision = OLD.graph_revision
				AND event.graph_revision = NEW.graph_revision
				AND event.basis_type_env_ref = OLD.active_type_env_ref
				AND event.result_type_env_ref = NEW.active_type_env_ref
		)
	BEGIN
		SELECT RAISE(ABORT, 'typed-memory graph head update lacks its exact revision and TypeEnv event');
	END`
}

func typedMemoryGraphEventExactHeadTrigger45() string {
	return `CREATE TRIGGER typed_memory_graph_events_exact_head
	BEFORE INSERT ON typed_memory_graph_events
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_graph_heads head
		WHERE head.project_id = NEW.project_id
			AND head.graph_revision = NEW.expected_revision
			AND head.active_type_env_ref = NEW.basis_type_env_ref
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory event does not match current graph head and active TypeEnv');
	END`
}

func typedMemoryEntityExactEventTrigger45() string {
	return `CREATE TRIGGER typed_memory_entities_exact_event
	BEFORE INSERT ON typed_memory_entities
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_graph_events event
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.first_declared_event_ref
			AND event.graph_revision = NEW.first_declared_revision
			AND event.event_kind = 'declare_entity'
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_graph_commits commit_record
				WHERE commit_record.project_id = event.project_id
					AND commit_record.event_ref = event.event_ref
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory entity does not match its declaration event');
	END`
}

func typedMemoryEntityContextExactEventTrigger45() string {
	return `CREATE TRIGGER typed_memory_entity_contexts_exact_event
	BEFORE INSERT ON typed_memory_entity_contexts
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_entities entity
			ON entity.project_id = event.project_id
			AND entity.entity_id = NEW.entity_id
			AND entity.first_declared_event_ref = event.event_ref
			AND entity.first_declared_revision = event.graph_revision
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.declared_event_ref
			AND event.graph_revision = NEW.declared_revision
			AND event.event_kind = 'declare_entity'
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_graph_commits commit_record
				WHERE commit_record.project_id = event.project_id
					AND commit_record.event_ref = event.event_ref
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory entity context does not match its declaration event');
	END`
}

func typedMemoryIdempotencyExactEventTrigger45() string {
	return `CREATE TRIGGER typed_memory_idempotency_exact_event
	BEFORE INSERT ON typed_memory_idempotency_history
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_graph_events event
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND event.graph_revision = NEW.graph_revision
			AND event.change_set_digest = NEW.change_set_digest
			AND event.event_digest = NEW.result_digest
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory idempotency record does not match its semantic event');
	END`
}

func typedMemoryProjectionJobExactEventTrigger45() string {
	return `CREATE TRIGGER typed_memory_projection_jobs_exact_event
	BEFORE INSERT ON typed_memory_projection_jobs
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_graph_events event
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.semantic_event_ref
			AND event.graph_revision = NEW.graph_revision
			AND event.event_digest = NEW.input_event_digest
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory projection job does not match its semantic event');
	END`
}

func typedMemoryProjectionDebtExactJobTrigger45() string {
	return `CREATE TRIGGER typed_memory_projection_debt_exact_job
	BEFORE INSERT ON typed_memory_projection_debt_events
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_projection_jobs job
		WHERE job.project_id = NEW.project_id
			AND job.projection_job_ref = NEW.projection_job_ref
			AND job.semantic_event_ref = NEW.semantic_event_ref
			AND job.graph_revision = NEW.graph_revision
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory projection debt does not match its projection job');
	END`
}

func typedMemoryGraphCommitNoReplaceTrigger45() string {
	return `CREATE TRIGGER typed_memory_graph_commits_no_replace
	BEFORE INSERT ON typed_memory_graph_commits
	WHEN EXISTS (
		SELECT 1 FROM typed_memory_graph_commits existing
		WHERE existing.project_id = NEW.project_id
			AND (existing.commit_ref = NEW.commit_ref OR existing.event_ref = NEW.event_ref)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory graph commits are append-only');
	END`
}

func typedMemoryGraphCommitExactClosureTrigger45() string {
	return `CREATE TRIGGER typed_memory_graph_commits_exact_closure
	BEFORE INSERT ON typed_memory_graph_commits
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_idempotency_history idempotency
			ON idempotency.project_id = event.project_id
			AND idempotency.event_ref = event.event_ref
		JOIN typed_memory_projection_jobs projection_job
			ON projection_job.project_id = event.project_id
			AND projection_job.semantic_event_ref = event.event_ref
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND event.commit_ref = NEW.commit_ref
			AND event.event_digest = NEW.event_digest
			AND event.expected_revision = NEW.expected_revision
			AND event.graph_revision = NEW.graph_revision
			AND event.change_set_digest = NEW.change_set_digest
			AND event.event_kind = 'declare_entity'
			AND event.change_count = 1
			AND event.authority_class = 'non_binding_semantic_assertion'
			AND idempotency.idempotency_key = NEW.idempotency_key
			AND idempotency.change_set_digest = NEW.change_set_digest
			AND idempotency.graph_revision = NEW.graph_revision
			AND idempotency.result_digest = NEW.event_digest
			AND projection_job.projection_job_ref = NEW.projection_job_ref
			AND projection_job.graph_revision = NEW.graph_revision
			AND projection_job.input_event_digest = NEW.event_digest
			AND NEW.entity_count = 1
			AND NEW.entity_context_count = 1
			AND (SELECT COUNT(*) FROM typed_memory_entities entity
				WHERE entity.project_id = NEW.project_id
					AND entity.first_declared_event_ref = NEW.event_ref) = NEW.entity_count
			AND (SELECT COUNT(*) FROM typed_memory_entity_contexts context
				WHERE context.project_id = NEW.project_id
					AND context.declared_event_ref = NEW.event_ref) = NEW.entity_context_count
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory graph commit lacks its exact event, materialization, idempotency, or projection closure');
	END`
}

func typedMemoryGraphCommitAdvanceHeadTrigger45() string {
	return `CREATE TRIGGER typed_memory_graph_commits_advance_head
	AFTER INSERT ON typed_memory_graph_commits BEGIN
		UPDATE typed_memory_graph_heads
		SET graph_revision = NEW.graph_revision,
			active_type_env_ref = (
				SELECT event.result_type_env_ref
				FROM typed_memory_graph_events event
				WHERE event.project_id = NEW.project_id AND event.event_ref = NEW.event_ref
			),
			last_event_ref = NEW.event_ref,
			last_commit_ref = NEW.commit_ref,
			updated_at = NEW.recorded_at
		WHERE project_id = NEW.project_id
			AND graph_revision = NEW.expected_revision;
	END`
}

func typedMemoryProjectionDebtResolutionTrigger45() string {
	return `CREATE TRIGGER typed_memory_projection_debt_resolution_chain
	BEFORE INSERT ON typed_memory_projection_debt_events
	WHEN NEW.event_kind = 'resolved'
		AND NOT EXISTS (
			SELECT 1 FROM typed_memory_projection_debt_events opened
			WHERE opened.project_id = NEW.project_id
				AND opened.debt_event_ref = NEW.supersedes_debt_event_ref
				AND opened.debt_ref = NEW.debt_ref
				AND opened.projection_job_ref = NEW.projection_job_ref
				AND opened.semantic_event_ref = NEW.semantic_event_ref
				AND opened.graph_revision = NEW.graph_revision
				AND opened.event_kind = 'opened'
		) BEGIN
		SELECT RAISE(ABORT, 'typed-memory projection debt resolution does not close its exact opened event');
	END`
}

func immutableTypedMemoryTrigger45(table string, operation string) string {
	return `CREATE TRIGGER ` + table + `_no_` + operation + `
	BEFORE ` + operation + ` ON ` + table + ` BEGIN
		SELECT RAISE(ABORT, 'typed-memory history is append-only');
	END`
}
