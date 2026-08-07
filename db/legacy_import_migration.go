package db

import "fmt"

const legacyImportSchemaVersion50 = 50

var legacyImportMigration50 = Migration{
	Version:     legacyImportSchemaVersion50,
	Description: "Add dormant exact legacy-import history and single-write-new guard",
	Apply:       applyLegacyImportMigration50,
}

func applyLegacyImportMigration50(
	tx MigrationTransaction,
	_ []Migration,
) error {
	var maximumVersion int
	err := tx.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&maximumVersion)
	if err != nil {
		return fmt.Errorf("inspect legacy-import source schema: %w", err)
	}
	if maximumVersion != 49 {
		return fmt.Errorf(
			"legacy-import schema requires exact schema version 49, found %d",
			maximumVersion,
		)
	}
	if err := requireLegacyImportSourceTables50(tx); err != nil {
		return err
	}
	if err := requireAbsentLegacyImportFootprint50(tx); err != nil {
		return err
	}
	if err := executeStatements(tx, legacyImportStatements50(), 0); err != nil {
		return fmt.Errorf("install dormant exact legacy-import schema: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify dormant legacy-import foreign keys: %w", err)
	}
	return nil
}

func requireLegacyImportSourceTables50(tx MigrationTransaction) error {
	required := []string{
		"project_ledger_binding",
		"project_typeenv_head_history",
		"project_typeenv_head_selection_receipts",
		"project_typeenv_head_selection_closures",
		"typed_memory_graph_events",
		"typed_memory_graph_commits",
		"typed_memory_entity_contexts",
		"typed_memory_idempotency_history",
		"holons",
		"relations",
		"artifacts",
		"artifact_links",
	}
	for _, table := range required {
		var count int
		err := tx.QueryRow(
			`SELECT COUNT(*)
			 FROM sqlite_master
			 WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf(
				"inspect legacy-import source table %s: %w",
				table,
				err,
			)
		}
		if count != 1 {
			return fmt.Errorf(
				"legacy-import schema requires source table %s",
				table,
			)
		}
	}
	return nil
}

func requireAbsentLegacyImportFootprint50(tx MigrationTransaction) error {
	for _, name := range legacyImportObjectNames50() {
		var count int
		err := tx.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE name = ?",
			name,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf(
				"inspect legacy-import target object %s: %w",
				name,
				err,
			)
		}
		if count != 0 {
			return fmt.Errorf(
				"legacy-import target object %s already exists",
				name,
			)
		}
	}
	return nil
}

func legacyImportObjectNames50() []string {
	return []string{
		"legacy_import_runs",
		"legacy_import_carriers",
		"legacy_import_run_carriers",
		"legacy_import_dispositions",
		"legacy_semantic_imports",
		"legacy_identity_bridges",
		"legacy_semantic_write_policy",
		"legacy_import_runs_no_update",
		"legacy_import_runs_no_delete",
		"legacy_import_carriers_no_update",
		"legacy_import_carriers_no_delete",
		"legacy_import_run_carriers_no_update",
		"legacy_import_run_carriers_no_delete",
		"legacy_import_dispositions_no_update",
		"legacy_import_dispositions_no_delete",
		"legacy_semantic_imports_no_update",
		"legacy_semantic_imports_no_delete",
		"legacy_identity_bridges_no_update",
		"legacy_identity_bridges_no_delete",
		"legacy_semantic_write_policy_insert_guard",
		"legacy_semantic_write_policy_transition_guard",
		"legacy_semantic_write_policy_no_delete",
		"legacy_single_write_holons_insert",
		"legacy_single_write_holons_update",
		"legacy_single_write_holons_delete",
		"legacy_single_write_relations_insert",
		"legacy_single_write_relations_update",
		"legacy_single_write_relations_delete",
		"legacy_single_write_artifacts_insert",
		"legacy_single_write_artifacts_update",
		"legacy_single_write_artifacts_delete",
		"legacy_single_write_artifact_links_insert",
		"legacy_single_write_artifact_links_update",
		"legacy_single_write_artifact_links_delete",
	}
}

func legacyImportStatements50() []string {
	statements := []string{
		`CREATE TABLE legacy_import_runs (
			project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
			idempotency_key TEXT NOT NULL CHECK(
				idempotency_key != '' AND trim(idempotency_key) = idempotency_key
			),
			import_plan_digest TEXT NOT NULL CHECK(
				length(import_plan_digest) = 71
				AND substr(import_plan_digest, 1, 7) = 'sha256:'
			),
			dry_run_report_digest TEXT NOT NULL CHECK(
				length(dry_run_report_digest) = 71
				AND substr(dry_run_report_digest, 1, 7) = 'sha256:'
			),
			source_snapshot_digest TEXT NOT NULL CHECK(
				length(source_snapshot_digest) = 71
				AND substr(source_snapshot_digest, 1, 7) = 'sha256:'
			),
			selected_head_ref TEXT NOT NULL,
			selected_head_revision INTEGER NOT NULL CHECK(selected_head_revision > 0),
			selected_type_env_ref TEXT NOT NULL,
			selected_graph_revision INTEGER NOT NULL CHECK(selected_graph_revision > 0),
			selection_receipt_ref TEXT NOT NULL,
			selection_closure_ref TEXT NOT NULL,
			import_receipt_ref TEXT NOT NULL UNIQUE,
			import_receipt_digest TEXT NOT NULL UNIQUE CHECK(
				length(import_receipt_digest) = 71
				AND substr(import_receipt_digest, 1, 7) = 'sha256:'
			),
			import_plan_canonical BLOB NOT NULL CHECK(length(import_plan_canonical) > 0),
			dry_run_report_canonical BLOB NOT NULL CHECK(length(dry_run_report_canonical) > 0),
			import_receipt_canonical BLOB NOT NULL CHECK(length(import_receipt_canonical) > 0),
			opaque_carrier_count INTEGER NOT NULL CHECK(opaque_carrier_count > 0),
			subject_disposition_count INTEGER NOT NULL CHECK(subject_disposition_count > 0),
			PRIMARY KEY(project_id, idempotency_key),
			UNIQUE(project_id, import_receipt_ref),
			FOREIGN KEY(project_id, selected_head_revision)
				REFERENCES project_typeenv_head_history(project_id, head_revision),
			FOREIGN KEY(selection_receipt_ref)
				REFERENCES project_typeenv_head_selection_receipts(receipt_ref),
			FOREIGN KEY(selection_closure_ref)
				REFERENCES project_typeenv_head_selection_closures(closure_ref),
			FOREIGN KEY(project_id, selected_graph_revision)
				REFERENCES typed_memory_graph_events(project_id, graph_revision)
		) WITHOUT ROWID`,
		`CREATE TABLE legacy_import_carriers (
			project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
			carrier_ref TEXT NOT NULL,
			carrier_edition TEXT NOT NULL,
			carrier_digest TEXT NOT NULL CHECK(
				length(carrier_digest) = 71
				AND substr(carrier_digest, 1, 7) = 'sha256:'
			),
			source_coordinate TEXT NOT NULL,
			carrier_format TEXT NOT NULL,
			exact_bytes BLOB NOT NULL,
			legacy_identity_ref TEXT,
			PRIMARY KEY(project_id, carrier_ref, carrier_edition),
			UNIQUE(project_id, source_coordinate),
			UNIQUE(project_id, carrier_ref, carrier_edition, carrier_digest)
		) WITHOUT ROWID`,
		`CREATE TABLE legacy_import_run_carriers (
			project_id TEXT NOT NULL,
			import_receipt_ref TEXT NOT NULL,
			carrier_ref TEXT NOT NULL,
			carrier_edition TEXT NOT NULL,
			carrier_digest TEXT NOT NULL,
			PRIMARY KEY(
				project_id,
				import_receipt_ref,
				carrier_ref,
				carrier_edition
			),
			FOREIGN KEY(project_id, import_receipt_ref)
				REFERENCES legacy_import_runs(project_id, import_receipt_ref),
			FOREIGN KEY(
				project_id,
				carrier_ref,
				carrier_edition,
				carrier_digest
			) REFERENCES legacy_import_carriers(
				project_id,
				carrier_ref,
				carrier_edition,
				carrier_digest
			)
		) WITHOUT ROWID`,
		`CREATE TABLE legacy_import_dispositions (
			project_id TEXT NOT NULL,
			import_receipt_ref TEXT NOT NULL,
			subject_ref TEXT NOT NULL,
			classification_kind TEXT NOT NULL CHECK(
				classification_kind IN (
					'carrier_only',
					'legacy_unbound',
					'unresolved'
				)
			),
			unresolved_reason TEXT NOT NULL DEFAULT '',
			canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
			PRIMARY KEY(project_id, import_receipt_ref, subject_ref),
			FOREIGN KEY(project_id, import_receipt_ref)
				REFERENCES legacy_import_runs(project_id, import_receipt_ref)
		) WITHOUT ROWID`,
		`CREATE TABLE legacy_semantic_imports (
			project_id TEXT NOT NULL,
			semantic_import_ref TEXT NOT NULL UNIQUE,
			import_receipt_ref TEXT NOT NULL,
			candidate_digest TEXT NOT NULL CHECK(
				length(candidate_digest) = 71
				AND substr(candidate_digest, 1, 7) = 'sha256:'
			),
			semantic_change_digest TEXT NOT NULL CHECK(
				length(semantic_change_digest) = 71
				AND substr(semantic_change_digest, 1, 7) = 'sha256:'
			),
			typed_idempotency_key TEXT NOT NULL,
			request_provenance_ref TEXT NOT NULL,
			graph_event_ref TEXT NOT NULL,
			graph_commit_ref TEXT NOT NULL,
			result_graph_revision INTEGER NOT NULL CHECK(result_graph_revision > 0),
			result_digest TEXT NOT NULL CHECK(
				length(result_digest) = 71
				AND substr(result_digest, 1, 7) = 'sha256:'
			),
			bridge_count INTEGER NOT NULL CHECK(bridge_count > 0),
			canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
			PRIMARY KEY(project_id, semantic_import_ref),
			UNIQUE(project_id, import_receipt_ref),
			UNIQUE(project_id, typed_idempotency_key),
			FOREIGN KEY(project_id, import_receipt_ref)
				REFERENCES legacy_import_runs(project_id, import_receipt_ref),
			FOREIGN KEY(project_id, graph_event_ref)
				REFERENCES typed_memory_graph_events(project_id, event_ref),
			FOREIGN KEY(project_id, graph_commit_ref)
				REFERENCES typed_memory_graph_commits(project_id, commit_ref),
			FOREIGN KEY(project_id, typed_idempotency_key)
				REFERENCES typed_memory_idempotency_history(
					project_id,
					idempotency_key
				)
		) WITHOUT ROWID`,
		`CREATE TABLE legacy_identity_bridges (
			project_id TEXT NOT NULL,
			semantic_import_ref TEXT NOT NULL,
			legacy_identity_ref TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			bounded_context_ref TEXT NOT NULL,
			mapping_carrier_ref TEXT NOT NULL,
			mapping_carrier_edition TEXT NOT NULL,
			mapping_carrier_digest TEXT NOT NULL CHECK(
				length(mapping_carrier_digest) = 71
				AND substr(mapping_carrier_digest, 1, 7) = 'sha256:'
			),
			bridge_digest TEXT NOT NULL CHECK(
				length(bridge_digest) = 71
				AND substr(bridge_digest, 1, 7) = 'sha256:'
			),
			canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
			PRIMARY KEY(
				project_id,
				semantic_import_ref,
				legacy_identity_ref,
				entity_id,
				bounded_context_ref,
				mapping_carrier_ref,
				mapping_carrier_edition
			),
			UNIQUE(project_id, bridge_digest),
			FOREIGN KEY(project_id, semantic_import_ref)
				REFERENCES legacy_semantic_imports(
					project_id,
					semantic_import_ref
				),
			FOREIGN KEY(
				project_id,
				entity_id,
				bounded_context_ref
			) REFERENCES typed_memory_entity_contexts(
				project_id,
				entity_id,
				bounded_context_ref
			),
			FOREIGN KEY(
				project_id,
				mapping_carrier_ref,
				mapping_carrier_edition,
				mapping_carrier_digest
			) REFERENCES legacy_import_carriers(
				project_id,
				carrier_ref,
				carrier_edition,
				carrier_digest
				)
		) WITHOUT ROWID`,
		`CREATE TABLE legacy_semantic_write_policy (
			singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
			mode TEXT NOT NULL CHECK(
				mode IN ('legacy_compatible', 'typed_single_write')
			),
			activation_semantic_import_ref TEXT NOT NULL DEFAULT '',
			activation_import_receipt_ref TEXT NOT NULL DEFAULT '',
			activation_head_ref TEXT NOT NULL DEFAULT '',
			activation_head_revision INTEGER NOT NULL DEFAULT 0,
			activation_type_env_ref TEXT NOT NULL DEFAULT '',
			activation_graph_revision INTEGER NOT NULL DEFAULT 0,
			activation_graph_commit_ref TEXT NOT NULL DEFAULT '',
			CHECK(
				(mode = 'legacy_compatible'
					AND activation_semantic_import_ref = ''
					AND activation_import_receipt_ref = ''
					AND activation_head_ref = ''
					AND activation_head_revision = 0
					AND activation_type_env_ref = ''
					AND activation_graph_revision = 0
					AND activation_graph_commit_ref = '')
				OR
				(mode = 'typed_single_write'
					AND activation_semantic_import_ref != ''
					AND activation_import_receipt_ref != ''
					AND activation_head_ref != ''
					AND activation_head_revision > 0
					AND activation_type_env_ref != ''
					AND activation_graph_revision > 0
					AND activation_graph_commit_ref != '')
			)
		) WITHOUT ROWID`,
		`INSERT INTO legacy_semantic_write_policy(singleton, mode)
		 VALUES (1, 'legacy_compatible')`,
	}
	for _, table := range []string{
		"legacy_import_runs",
		"legacy_import_carriers",
		"legacy_import_run_carriers",
		"legacy_import_dispositions",
		"legacy_semantic_imports",
		"legacy_identity_bridges",
	} {
		statements = append(
			statements,
			"CREATE TRIGGER "+table+"_no_update BEFORE UPDATE ON "+table+
				" BEGIN SELECT RAISE(ABORT, '"+table+" is append-only'); END",
			"CREATE TRIGGER "+table+"_no_delete BEFORE DELETE ON "+table+
				" BEGIN SELECT RAISE(ABORT, '"+table+" is append-only'); END",
		)
	}
	statements = append(
		statements,
		`CREATE TRIGGER legacy_semantic_write_policy_insert_guard
		BEFORE INSERT ON legacy_semantic_write_policy
		BEGIN
			SELECT RAISE(ABORT, 'legacy semantic-write policy singleton already exists');
		END`,
		`CREATE TRIGGER legacy_semantic_write_policy_transition_guard
		BEFORE UPDATE ON legacy_semantic_write_policy
		WHEN OLD.mode != 'legacy_compatible'
			OR NEW.mode != 'typed_single_write'
			OR NOT EXISTS (
				SELECT 1
				FROM legacy_semantic_imports semantic_import
				JOIN legacy_import_runs import_run
					ON import_run.project_id = semantic_import.project_id
					AND import_run.import_receipt_ref =
						semantic_import.import_receipt_ref
				JOIN project_typeenv_heads current_head
					ON current_head.project_id = semantic_import.project_id
				WHERE semantic_import.semantic_import_ref =
						NEW.activation_semantic_import_ref
					AND semantic_import.import_receipt_ref =
						NEW.activation_import_receipt_ref
					AND semantic_import.graph_commit_ref =
						NEW.activation_graph_commit_ref
					AND semantic_import.result_graph_revision =
						NEW.activation_graph_revision
					AND import_run.selected_head_ref =
						NEW.activation_head_ref
					AND import_run.selected_head_revision =
						NEW.activation_head_revision
					AND import_run.selected_type_env_ref =
						NEW.activation_type_env_ref
					AND current_head.head_ref =
						NEW.activation_head_ref
					AND current_head.head_revision =
						NEW.activation_head_revision
					AND current_head.selected_composite_ref =
						NEW.activation_type_env_ref
			)
		BEGIN
			SELECT RAISE(
				ABORT,
				'typed single-write activation lacks exact import and current-head basis'
			);
		END`,
		`CREATE TRIGGER legacy_semantic_write_policy_no_delete
		BEFORE DELETE ON legacy_semantic_write_policy
		BEGIN
			SELECT RAISE(ABORT, 'legacy semantic-write policy is permanent');
		END`,
	)
	for _, table := range []string{
		"holons",
		"relations",
		"artifacts",
		"artifact_links",
	} {
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE"} {
			name := "legacy_single_write_" + table + "_" +
				map[string]string{
					"INSERT": "insert",
					"UPDATE": "update",
					"DELETE": "delete",
				}[operation]
			statements = append(
				statements,
				"CREATE TRIGGER "+name+" BEFORE "+operation+" ON "+table+
					" WHEN (SELECT mode FROM legacy_semantic_write_policy"+
					" WHERE singleton = 1) = 'typed_single_write'"+
					" BEGIN SELECT RAISE(ABORT, 'legacy semantic writes are disabled; use typed admission'); END",
			)
		}
	}
	return statements
}
