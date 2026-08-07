package db

import (
	"fmt"
	"strings"
)

const typedMemoryIdentityReconciliationSchemaVersion52 = 52

const (
	typedMemoryIdentityReconciliationsTable52 = "typed_memory_identity_reconciliations"
	typedMemoryIdentityParticipantsTable52    = "typed_memory_identity_reconciliation_participants"
	typedMemoryIdentityRedirectsTable52       = "typed_memory_identity_redirects"
	typedMemoryIdentityClosuresTable52        = "typed_memory_identity_reconciliation_closures"
)

var typedMemoryIdentityReconciliationMigration52 = Migration{
	Version:     typedMemoryIdentityReconciliationSchemaVersion52,
	Description: "Add immutable reviewed identity-reconciliation graph events",
	Apply:       applyTypedMemoryIdentityReconciliationMigration52,
}

type typedMemoryIdentityObject52 struct {
	kind string
	name string
	sql  string
}

func applyTypedMemoryIdentityReconciliationMigration52(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireTypedMemoryIdentitySource52(tx); err != nil {
		return err
	}
	if err := requireAbsentTypedMemoryIdentityFootprint52(tx); err != nil {
		return err
	}
	objects, err := typedMemoryIdentityObjects52()
	if err != nil {
		return err
	}
	statements := make([]string, 0, len(objects)+1)
	for _, object := range objects {
		if object.kind == "index" {
			statements = append(statements, object.sql)
			continue
		}
		statements = append(statements, object.sql)
	}
	statements = append(
		[]string{"DROP TRIGGER typed_memory_graph_commits_exact_closure"},
		statements...,
	)
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install reviewed identity-reconciliation ledger: %w", err)
	}
	if err := verifyTypedMemoryIdentityFootprint52(tx); err != nil {
		return err
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify identity-reconciliation foreign keys: %w", err)
	}
	return nil
}

func requireTypedMemoryIdentitySource52(tx MigrationTransaction) error {
	var maximumVersion int
	err := tx.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&maximumVersion)
	if err != nil {
		return fmt.Errorf("inspect identity-reconciliation source schema: %w", err)
	}
	if maximumVersion != 51 {
		return fmt.Errorf(
			"identity-reconciliation ledger requires exact schema version 51, found %d",
			maximumVersion,
		)
	}
	if err := requireTypedMemoryWriterCapability46(tx); err != nil {
		return err
	}
	return requireExactTypedMemoryObject52(
		tx,
		typedMemoryIdentityObject52{
			kind: "trigger",
			name: "typed_memory_graph_commits_exact_closure",
			sql:  typedMemoryGraphCommitExactClosureTrigger46(),
		},
	)
}

func requireAbsentTypedMemoryIdentityFootprint52(tx MigrationTransaction) error {
	for _, name := range []string{
		typedMemoryIdentityReconciliationsTable52,
		typedMemoryIdentityParticipantsTable52,
		typedMemoryIdentityRedirectsTable52,
		typedMemoryIdentityClosuresTable52,
	} {
		var count int
		err := tx.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE name = ?",
			name,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("inspect identity-reconciliation footprint %s: %w", name, err)
		}
		if count != 0 {
			return fmt.Errorf("identity-reconciliation footprint %s already exists", name)
		}
	}
	return nil
}

func typedMemoryIdentityObjects52() ([]typedMemoryIdentityObject52, error) {
	commitTrigger, err := typedMemoryGraphCommitExactClosureTrigger52()
	if err != nil {
		return nil, err
	}
	objects := []typedMemoryIdentityObject52{
		{kind: "table", name: typedMemoryIdentityReconciliationsTable52, sql: typedMemoryIdentityReconciliationsSQL52()},
		{kind: "table", name: typedMemoryIdentityParticipantsTable52, sql: typedMemoryIdentityParticipantsSQL52()},
		{kind: "table", name: typedMemoryIdentityRedirectsTable52, sql: typedMemoryIdentityRedirectsSQL52()},
		{kind: "table", name: typedMemoryIdentityClosuresTable52, sql: typedMemoryIdentityClosuresSQL52()},
		{kind: "index", name: "idx_typed_memory_identity_redirect_source_v52", sql: "CREATE INDEX idx_typed_memory_identity_redirect_source_v52 ON typed_memory_identity_redirects(project_id, bounded_context_ref, source_entity_id, event_ref)"},
		{kind: "index", name: "idx_typed_memory_identity_redirect_target_v52", sql: "CREATE INDEX idx_typed_memory_identity_redirect_target_v52 ON typed_memory_identity_redirects(project_id, bounded_context_ref, target_entity_id, event_ref)"},
		{kind: "trigger", name: "typed_memory_identity_reconciliations_v52_exact_event", sql: typedMemoryIdentityExactEventTrigger52()},
		{kind: "trigger", name: "typed_memory_identity_participants_v52_exact_reconciliation", sql: typedMemoryIdentityParticipantTrigger52()},
		{kind: "trigger", name: "typed_memory_identity_redirects_v52_exact_participant", sql: typedMemoryIdentityRedirectTrigger52()},
		{kind: "trigger", name: "typed_memory_identity_closures_v52_exact_reconciliation", sql: typedMemoryIdentityClosureTrigger52()},
	}
	for _, table := range []string{
		typedMemoryIdentityReconciliationsTable52,
		typedMemoryIdentityParticipantsTable52,
		typedMemoryIdentityRedirectsTable52,
		typedMemoryIdentityClosuresTable52,
	} {
		objects = append(
			objects,
			typedMemoryIdentityObject52{kind: "trigger", name: table + "_v52_no_update", sql: immutableTypedMemoryIdentityTrigger52(table, "update")},
			typedMemoryIdentityObject52{kind: "trigger", name: table + "_v52_no_delete", sql: immutableTypedMemoryIdentityTrigger52(table, "delete")},
		)
	}
	objects = append(objects, typedMemoryIdentityObject52{
		kind: "trigger",
		name: "typed_memory_graph_commits_exact_closure",
		sql:  commitTrigger,
	})
	return objects, nil
}

func typedMemoryIdentityReconciliationsSQL52() string {
	return `CREATE TABLE typed_memory_identity_reconciliations (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		commit_ref TEXT NOT NULL,
		reconciliation_ref TEXT NOT NULL CHECK(reconciliation_ref != '' AND trim(reconciliation_ref) = reconciliation_ref),
		operation TEXT NOT NULL CHECK(operation IN ('merge_entities', 'split_entity')),
		bounded_context_ref TEXT NOT NULL CHECK(bounded_context_ref != '' AND trim(bounded_context_ref) = bounded_context_ref),
		primary_entity_id TEXT NOT NULL CHECK(primary_entity_id != '' AND trim(primary_entity_id) = primary_entity_id),
		reconciliation_basis_ref TEXT NOT NULL CHECK(reconciliation_basis_ref != '' AND trim(reconciliation_basis_ref) = reconciliation_basis_ref),
		basis_type_env_ref TEXT NOT NULL REFERENCES typed_memory_type_env_snapshots(type_env_ref),
		basis_graph_revision INTEGER NOT NULL CHECK(basis_graph_revision >= 0),
		review_payload_digest TEXT NOT NULL CHECK(length(review_payload_digest) = 71 AND substr(review_payload_digest, 1, 7) = 'sha256:'),
		review_provenance_ref TEXT NOT NULL CHECK(review_provenance_ref != '' AND trim(review_provenance_ref) = review_provenance_ref),
		basis_digest TEXT NOT NULL CHECK(length(basis_digest) = 71 AND substr(basis_digest, 1, 7) = 'sha256:'),
		canonical_basis_bytes BLOB NOT NULL CHECK(length(canonical_basis_bytes) > 0),
		change_digest TEXT NOT NULL CHECK(length(change_digest) = 71 AND substr(change_digest, 1, 7) = 'sha256:'),
		canonical_change_bytes BLOB NOT NULL CHECK(length(canonical_change_bytes) > 0),
		admission_digest TEXT NOT NULL CHECK(length(admission_digest) = 71 AND substr(admission_digest, 1, 7) = 'sha256:'),
		canonical_admission_bytes BLOB NOT NULL CHECK(length(canonical_admission_bytes) > 0),
		reconciliation_digest TEXT NOT NULL CHECK(length(reconciliation_digest) = 71 AND substr(reconciliation_digest, 1, 7) = 'sha256:'),
		canonical_reconciliation_bytes BLOB NOT NULL CHECK(length(canonical_reconciliation_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, event_ref),
		UNIQUE(project_id, reconciliation_ref),
		UNIQUE(project_id, reconciliation_digest),
		FOREIGN KEY(project_id, event_ref) REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(project_id, primary_entity_id) REFERENCES typed_memory_entities(project_id, entity_id)
	) WITHOUT ROWID`
}

func typedMemoryIdentityParticipantsSQL52() string {
	return `CREATE TABLE typed_memory_identity_reconciliation_participants (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		participant_ordinal INTEGER NOT NULL CHECK(participant_ordinal >= 0),
		participant_role TEXT NOT NULL CHECK(participant_role IN ('merged_entity', 'split_target')),
		entity_id TEXT NOT NULL CHECK(entity_id != '' AND trim(entity_id) = entity_id),
		PRIMARY KEY(project_id, event_ref, participant_ordinal),
		UNIQUE(project_id, event_ref, entity_id),
		FOREIGN KEY(project_id, event_ref) REFERENCES typed_memory_identity_reconciliations(project_id, event_ref),
		FOREIGN KEY(project_id, entity_id) REFERENCES typed_memory_entities(project_id, entity_id)
	) WITHOUT ROWID`
}

func typedMemoryIdentityRedirectsSQL52() string {
	return `CREATE TABLE typed_memory_identity_redirects (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		redirect_ordinal INTEGER NOT NULL CHECK(redirect_ordinal >= 0),
		resolution_kind TEXT NOT NULL CHECK(resolution_kind IN ('merge_redirect', 'split_candidate')),
		bounded_context_ref TEXT NOT NULL CHECK(bounded_context_ref != '' AND trim(bounded_context_ref) = bounded_context_ref),
		source_entity_id TEXT NOT NULL CHECK(source_entity_id != '' AND trim(source_entity_id) = source_entity_id),
		target_entity_id TEXT NOT NULL CHECK(target_entity_id != '' AND trim(target_entity_id) = target_entity_id),
		reconciliation_basis_ref TEXT NOT NULL CHECK(reconciliation_basis_ref != '' AND trim(reconciliation_basis_ref) = reconciliation_basis_ref),
		PRIMARY KEY(project_id, event_ref, redirect_ordinal),
		UNIQUE(project_id, event_ref, source_entity_id, target_entity_id),
		FOREIGN KEY(project_id, event_ref) REFERENCES typed_memory_identity_reconciliations(project_id, event_ref),
		FOREIGN KEY(project_id, source_entity_id) REFERENCES typed_memory_entities(project_id, entity_id),
		FOREIGN KEY(project_id, target_entity_id) REFERENCES typed_memory_entities(project_id, entity_id)
	) WITHOUT ROWID`
}

func typedMemoryIdentityClosuresSQL52() string {
	return `CREATE TABLE typed_memory_identity_reconciliation_closures (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		commit_ref TEXT NOT NULL,
		event_digest TEXT NOT NULL CHECK(length(event_digest) = 71 AND substr(event_digest, 1, 7) = 'sha256:'),
		change_digest TEXT NOT NULL CHECK(length(change_digest) = 71 AND substr(change_digest, 1, 7) = 'sha256:'),
		reconciliation_digest TEXT NOT NULL CHECK(length(reconciliation_digest) = 71 AND substr(reconciliation_digest, 1, 7) = 'sha256:'),
		materialization_digest TEXT NOT NULL CHECK(length(materialization_digest) = 71 AND substr(materialization_digest, 1, 7) = 'sha256:'),
		canonical_materialization_bytes BLOB NOT NULL CHECK(length(canonical_materialization_bytes) > 0),
		participant_count INTEGER NOT NULL CHECK(participant_count > 0),
		redirect_count INTEGER NOT NULL CHECK(redirect_count = participant_count),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, event_ref),
		UNIQUE(project_id, commit_ref),
		FOREIGN KEY(project_id, event_ref) REFERENCES typed_memory_identity_reconciliations(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryIdentityExactEventTrigger52() string {
	return `CREATE TRIGGER typed_memory_identity_reconciliations_v52_exact_event
	BEFORE INSERT ON typed_memory_identity_reconciliations
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_graph_events event
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND event.commit_ref = NEW.commit_ref
			AND event.expected_revision = NEW.basis_graph_revision
			AND event.basis_type_env_ref = NEW.basis_type_env_ref
			AND event.result_type_env_ref = NEW.basis_type_env_ref
			AND event.change_set_digest = NEW.change_digest
			AND event.event_kind = NEW.operation
			AND event.authority_class = 'non_binding_semantic_assertion'
			AND event.request_provenance_ref = NEW.review_provenance_ref
			AND event.change_count = 1
	) BEGIN
		SELECT RAISE(ABORT, 'identity reconciliation does not bind its exact reviewed graph event');
	END`
}

func typedMemoryIdentityParticipantTrigger52() string {
	return `CREATE TRIGGER typed_memory_identity_participants_v52_exact_reconciliation
	BEFORE INSERT ON typed_memory_identity_reconciliation_participants
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_identity_reconciliations reconciliation
		WHERE reconciliation.project_id = NEW.project_id
			AND reconciliation.event_ref = NEW.event_ref
			AND (
				(reconciliation.operation = 'merge_entities' AND NEW.participant_role = 'merged_entity')
				OR (reconciliation.operation = 'split_entity' AND NEW.participant_role = 'split_target')
			)
			AND reconciliation.primary_entity_id != NEW.entity_id
	) BEGIN
		SELECT RAISE(ABORT, 'identity reconciliation participant does not match the exact operation');
	END`
}

func typedMemoryIdentityRedirectTrigger52() string {
	return `CREATE TRIGGER typed_memory_identity_redirects_v52_exact_participant
	BEFORE INSERT ON typed_memory_identity_redirects
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_identity_reconciliations reconciliation
		JOIN typed_memory_identity_reconciliation_participants participant
			ON participant.project_id = reconciliation.project_id
			AND participant.event_ref = reconciliation.event_ref
			AND participant.participant_ordinal = NEW.redirect_ordinal
		WHERE reconciliation.project_id = NEW.project_id
			AND reconciliation.event_ref = NEW.event_ref
			AND reconciliation.bounded_context_ref = NEW.bounded_context_ref
			AND reconciliation.reconciliation_basis_ref = NEW.reconciliation_basis_ref
			AND (
				(reconciliation.operation = 'merge_entities'
					AND NEW.resolution_kind = 'merge_redirect'
					AND NEW.source_entity_id = participant.entity_id
					AND NEW.target_entity_id = reconciliation.primary_entity_id)
				OR (reconciliation.operation = 'split_entity'
					AND NEW.resolution_kind = 'split_candidate'
					AND NEW.source_entity_id = reconciliation.primary_entity_id
					AND NEW.target_entity_id = participant.entity_id)
			)
	) BEGIN
		SELECT RAISE(ABORT, 'identity redirect does not preserve the exact reviewed participant mapping');
	END`
}

func typedMemoryIdentityClosureTrigger52() string {
	return `CREATE TRIGGER typed_memory_identity_closures_v52_exact_reconciliation
	BEFORE INSERT ON typed_memory_identity_reconciliation_closures
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_identity_reconciliations reconciliation
		JOIN typed_memory_graph_events event
			ON event.project_id = reconciliation.project_id
			AND event.event_ref = reconciliation.event_ref
		WHERE reconciliation.project_id = NEW.project_id
			AND reconciliation.event_ref = NEW.event_ref
			AND reconciliation.commit_ref = NEW.commit_ref
			AND reconciliation.change_digest = NEW.change_digest
			AND reconciliation.reconciliation_digest = NEW.reconciliation_digest
			AND event.event_digest = NEW.event_digest
			AND NEW.participant_count = (
				SELECT COUNT(*) FROM typed_memory_identity_reconciliation_participants participant
				WHERE participant.project_id = NEW.project_id AND participant.event_ref = NEW.event_ref
			)
			AND NEW.redirect_count = (
				SELECT COUNT(*) FROM typed_memory_identity_redirects redirect
				WHERE redirect.project_id = NEW.project_id AND redirect.event_ref = NEW.event_ref
			)
			AND (
				(reconciliation.operation = 'merge_entities' AND NEW.participant_count >= 1)
				OR (reconciliation.operation = 'split_entity' AND NEW.participant_count >= 2)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_event_writer_generations generation
				WHERE generation.project_id = NEW.project_id AND generation.event_ref = NEW.event_ref
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_event_admission_bases admission
				WHERE admission.project_id = NEW.project_id AND admission.event_ref = NEW.event_ref
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_commit_materialization_closures closure
				WHERE closure.project_id = NEW.project_id AND closure.event_ref = NEW.event_ref
			)
	) BEGIN
		SELECT RAISE(ABORT, 'identity reconciliation closure does not bind its exact immutable event');
	END`
}

func typedMemoryGraphCommitExactClosureTrigger52() (string, error) {
	legacy := typedMemoryGraphCommitExactClosureTrigger46()
	marker := "\n\t) BEGIN\n\t\tSELECT RAISE(ABORT"
	index := strings.LastIndex(legacy, marker)
	if index < 0 {
		return "", fmt.Errorf("derive v52 graph-commit trigger: v46 trigger grammar changed")
	}
	identityBranch := `
	) AND NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_idempotency_history idempotency
			ON idempotency.project_id = event.project_id
			AND idempotency.event_ref = event.event_ref
		JOIN typed_memory_projection_jobs projection_job
			ON projection_job.project_id = event.project_id
			AND projection_job.semantic_event_ref = event.event_ref
		JOIN typed_memory_identity_reconciliations reconciliation
			ON reconciliation.project_id = event.project_id
			AND reconciliation.event_ref = event.event_ref
		JOIN typed_memory_identity_reconciliation_closures closure
			ON closure.project_id = event.project_id
			AND closure.event_ref = event.event_ref
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND event.commit_ref = NEW.commit_ref
			AND event.event_digest = NEW.event_digest
			AND event.expected_revision = NEW.expected_revision
			AND event.graph_revision = NEW.graph_revision
			AND event.change_set_digest = NEW.change_set_digest
			AND event.event_kind IN ('merge_entities', 'split_entity')
			AND event.authority_class = 'non_binding_semantic_assertion'
			AND event.change_count = 1
			AND reconciliation.operation = event.event_kind
			AND reconciliation.commit_ref = NEW.commit_ref
			AND reconciliation.change_digest = NEW.change_set_digest
			AND idempotency.idempotency_key = NEW.idempotency_key
			AND idempotency.change_set_digest = NEW.change_set_digest
			AND idempotency.graph_revision = NEW.graph_revision
			AND idempotency.result_digest = NEW.event_digest
			AND projection_job.projection_job_ref = NEW.projection_job_ref
			AND projection_job.graph_revision = NEW.graph_revision
			AND projection_job.input_event_digest = NEW.event_digest
			AND closure.commit_ref = NEW.commit_ref
			AND closure.event_digest = NEW.event_digest
			AND closure.change_digest = NEW.change_set_digest
			AND NEW.entity_count = 0
			AND NEW.entity_context_count = 0`
	return legacy[:index] + identityBranch + legacy[index:], nil
}

func immutableTypedMemoryIdentityTrigger52(table string, action string) string {
	return fmt.Sprintf(
		`CREATE TRIGGER %s_v52_no_%s BEFORE %s ON %s BEGIN
			SELECT RAISE(ABORT, '%s is append-only');
		END`,
		table,
		action,
		strings.ToUpper(action),
		table,
		table,
	)
}

func requireExactTypedMemoryObject52(
	tx MigrationTransaction,
	object typedMemoryIdentityObject52,
) error {
	var sqlText string
	err := tx.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		object.kind,
		object.name,
	).Scan(&sqlText)
	if err != nil {
		return fmt.Errorf("load exact identity-reconciliation %s %s: %w", object.kind, object.name, err)
	}
	if normalizeSQLiteDDL46(sqlText) != normalizeSQLiteDDL46(object.sql) {
		return fmt.Errorf("identity-reconciliation %s %s differs from its exact schema", object.kind, object.name)
	}
	return nil
}

func verifyTypedMemoryIdentityFootprint52(tx MigrationTransaction) error {
	objects, err := typedMemoryIdentityObjects52()
	if err != nil {
		return err
	}
	for _, object := range objects {
		if err := requireExactTypedMemoryObject52(tx, object); err != nil {
			return err
		}
	}
	return nil
}
