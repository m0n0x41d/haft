package db

import (
	"fmt"
	"strings"
)

const (
	typeEnvCompatibleAuthorityGeneration57 = "compatible_successor_policy"
	typeEnvCompatibleResolutionKind57      = "automatic_compatible_successor"
	typeEnvCompatiblePolicyEdition57       = "haft.project-typeenv.compatible-successor-policy/v1"
)

var projectTypeEnvCompatibleSuccessorMigration57 = Migration{
	Version:            57,
	Description:        "Add automatic compatible ProjectTypeEnv successor authority",
	Apply:              applyProjectTypeEnvCompatibleSuccessorMigration57,
	ApplyBoundary:      ForeignKeyTableRebuildBoundary,
	ForeignKeyVerifier: verifyForeignKeys,
}

func applyProjectTypeEnvCompatibleSuccessorMigration57(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireSchemaVersion57(tx, 56); err != nil {
		return err
	}
	for _, table := range []string{
		"project_typeenv_head_selection_compatible_resolutions_v1",
		"project_typeenv_head_selection_compatible_uses_v1",
		"project_typeenv_head_selection_authority_resolutions_v56_snapshot",
		"project_typeenv_head_selection_authority_uses_v56_snapshot",
	} {
		count := 0
		if err := tx.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&count); err != nil {
			return fmt.Errorf("inspect compatible TypeEnv footprint %s: %w", table, err)
		}
		if count != 0 {
			return fmt.Errorf(
				"compatible TypeEnv migration refused: unversioned table %s already exists",
				table,
			)
		}
	}
	graphEvents, err := typedMemoryGraphEventsTable57()
	if err != nil {
		return err
	}
	graphCommit, err := typedMemoryGraphCommitExactClosureTrigger57()
	if err != nil {
		return err
	}
	activationEffect, err := typedMemoryTypeEnvActivationExactEffectTrigger57()
	if err != nil {
		return err
	}
	commitEffect, err := typedMemoryGraphCommitActivationEffectTrigger57()
	if err != nil {
		return err
	}
	preservedGraphDDL := make(map[string]string, 4)
	for _, object := range []struct {
		kind string
		name string
	}{
		{kind: "index", name: "idx_typed_memory_events_project_revision"},
		{kind: "trigger", name: "typed_memory_graph_events_exact_head"},
		{kind: "trigger", name: "typed_memory_graph_events_no_update"},
		{kind: "trigger", name: "typed_memory_graph_events_no_delete"},
	} {
		var ddl string
		if err := tx.QueryRow(
			"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
			object.kind,
			object.name,
		).Scan(&ddl); err != nil {
			return fmt.Errorf("load v57 preserved %s %s: %w", object.kind, object.name, err)
		}
		if strings.TrimSpace(ddl) == "" {
			return fmt.Errorf("v57 preserved %s %s has empty SQL", object.kind, object.name)
		}
		preservedGraphDDL[object.name] = ddl
	}
	statements := []string{
		`CREATE TABLE project_typeenv_head_selection_authority_resolutions_v56_snapshot
		 AS SELECT * FROM project_typeenv_head_selection_authority_resolutions`,
		`CREATE TABLE project_typeenv_head_selection_authority_uses_v56_snapshot
		 AS SELECT * FROM project_typeenv_head_selection_authority_uses`,
		`DROP TABLE project_typeenv_head_selection_authority_uses`,
		`DROP TABLE project_typeenv_head_selection_authority_resolutions`,
		compatibleTypeEnvResolutionTableSQL57(),
		compatibleTypeEnvAuthorityResolutionCatalogSQL57(),
		`INSERT INTO project_typeenv_head_selection_authority_resolutions (
			authority_resolution_ref, authority_resolution_digest,
			authority_generation, project_id, authority_resolution_kind,
			content_ref, content_digest, request_ref, request_digest,
			trusted_cli_source_ref, trusted_cli_source_digest,
			strict_basis_ref, strict_basis_digest,
			explicit_resolution_ref, explicit_resolution_digest,
			strict_resolution_ref, strict_resolution_digest,
			host_resolution_ref, host_resolution_digest,
			compatible_resolution_ref, compatible_resolution_digest,
			evaluated_at, canonical_bytes, recorded_at
		) SELECT authority_resolution_ref, authority_resolution_digest,
			authority_generation, project_id, authority_resolution_kind,
			content_ref, content_digest, request_ref, request_digest,
			trusted_cli_source_ref, trusted_cli_source_digest,
			strict_basis_ref, strict_basis_digest,
			explicit_resolution_ref, explicit_resolution_digest,
			strict_resolution_ref, strict_resolution_digest,
			host_resolution_ref, host_resolution_digest,
			NULL, NULL, evaluated_at, canonical_bytes, recorded_at
		 FROM project_typeenv_head_selection_authority_resolutions_v56_snapshot`,
		compatibleTypeEnvAuthorityUseCatalogSQL57(),
		`INSERT INTO project_typeenv_head_selection_authority_uses (
			authority_use_ref, authority_use_digest, authority_generation,
			project_id, original_idempotency_key, authority_resolution_kind,
			authority_resolution_ref, authority_resolution_digest,
			content_ref, content_digest, request_ref, request_digest,
			work_ref, receipt_ref, predecessor_kind, predecessor_head_ref,
			predecessor_head_revision, predecessor_selected_composite_ref,
			base_type_env_ref, ordered_extension_refs_digest,
			canonical_ordered_extension_refs, runtime_evaluation_basis_ref,
			selected_composite_ref, stage_ref, stage_digest,
			expected_graph_revision, committed_head_revision,
			committed_graph_revision, verifier_ref, verifier_edition,
			canonical_bytes, recorded_at
		) SELECT authority_use_ref, authority_use_digest, authority_generation,
			project_id, original_idempotency_key, authority_resolution_kind,
			authority_resolution_ref, authority_resolution_digest,
			content_ref, content_digest, request_ref, request_digest,
			work_ref, receipt_ref, predecessor_kind, predecessor_head_ref,
			predecessor_head_revision, predecessor_selected_composite_ref,
			base_type_env_ref, ordered_extension_refs_digest,
			canonical_ordered_extension_refs, runtime_evaluation_basis_ref,
			selected_composite_ref, stage_ref, stage_digest,
			expected_graph_revision, committed_head_revision,
			committed_graph_revision, verifier_ref, verifier_edition,
			canonical_bytes, recorded_at
		 FROM project_typeenv_head_selection_authority_uses_v56_snapshot`,
		compatibleTypeEnvUseTableSQL57(),
		`CREATE TEMP TABLE typed_memory_graph_events_v57_backup AS
		 SELECT * FROM typed_memory_graph_events`,
		"DROP TRIGGER typed_memory_graph_commits_exact_closure",
		"DROP TRIGGER typed_memory_type_env_activations_v47_exact_effect",
		"DROP TRIGGER typed_memory_graph_commits_v47_activation_effect",
		"DROP TABLE typed_memory_graph_events",
		graphEvents,
		`INSERT INTO typed_memory_graph_events (
			project_id, event_ref, commit_ref, event_digest,
			expected_revision, graph_revision,
			basis_type_env_ref, result_type_env_ref,
			change_set_digest, canonical_change_set_bytes,
			change_count, event_kind, authority_class,
			request_provenance_ref, recorded_at
		) SELECT
			project_id, event_ref, commit_ref, event_digest,
			expected_revision, graph_revision,
			basis_type_env_ref, result_type_env_ref,
			change_set_digest, canonical_change_set_bytes,
			change_count, event_kind, authority_class,
			request_provenance_ref, recorded_at
		 FROM typed_memory_graph_events_v57_backup`,
		"DROP TABLE typed_memory_graph_events_v57_backup",
		preservedGraphDDL["idx_typed_memory_events_project_revision"],
		preservedGraphDDL["typed_memory_graph_events_exact_head"],
		preservedGraphDDL["typed_memory_graph_events_no_update"],
		preservedGraphDDL["typed_memory_graph_events_no_delete"],
		`DROP TABLE project_typeenv_head_selection_authority_resolutions_v56_snapshot`,
		`DROP TABLE project_typeenv_head_selection_authority_uses_v56_snapshot`,
		hostRoutedTypeEnvCatalogResolutionExactHostTrigger56(),
		hostRoutedTypeEnvCatalogUseExactHostTrigger56(),
		compatibleTypeEnvResolutionExactSourcesTrigger57(),
		compatibleTypeEnvCatalogResolutionExactSourceTrigger57(),
		compatibleTypeEnvCatalogUseExactSourceTrigger57(),
		compatibleTypeEnvUseExactCatalogTrigger57(),
		graphCommit,
		activationEffect,
		commitEffect,
		currentGenerationOnlyTrigger57(
			"project_typeenv_head_selection_authority_resolutions",
		),
		currentGenerationOnlyTrigger57(
			"project_typeenv_head_selection_authority_uses",
		),
		"CREATE INDEX idx_project_typeenv_compatible_resolution_project ON project_typeenv_head_selection_compatible_resolutions_v1(project_id, evaluated_at)",
		"CREATE INDEX idx_project_typeenv_compatible_use_project ON project_typeenv_head_selection_compatible_uses_v1(project_id, consumed_at)",
	}
	for _, table := range []string{
		"project_typeenv_head_selection_compatible_resolutions_v1",
		"project_typeenv_head_selection_compatible_uses_v1",
		"project_typeenv_head_selection_authority_resolutions",
		"project_typeenv_head_selection_authority_uses",
	} {
		statements = append(
			statements,
			"CREATE TRIGGER "+table+"_no_update BEFORE UPDATE ON "+table+" BEGIN SELECT RAISE(ABORT, '"+table+" is append-only'); END",
			"CREATE TRIGGER "+table+"_no_delete BEFORE DELETE ON "+table+" BEGIN SELECT RAISE(ABORT, '"+table+" is append-only'); END",
		)
	}
	for _, table := range []string{
		"project_typeenv_head_selection_compatible_resolutions_v1",
		"project_typeenv_head_selection_compatible_uses_v1",
	} {
		statements = append(statements, compatibleTypeEnvRootGuard57(table))
	}
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install automatic compatible TypeEnv authority: %w", err)
	}
	return verifyForeignKeys(tx)
}

func requireSchemaVersion57(tx MigrationTransaction, version int) error {
	count := 0
	if err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = ?",
		version,
	).Scan(&count); err != nil {
		return fmt.Errorf("inspect compatible TypeEnv source migration: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("automatic compatible TypeEnv authority requires schema version %d", version)
	}
	return nil
}

func typedMemoryGraphEventsTable57() (string, error) {
	source, err := typedMemoryGraphEventsTable53()
	if err != nil {
		return "", err
	}
	return replaceExactSQL47(
		source,
		"'non_binding_semantic_assertion', 'manual_type_env_activation'",
		"'non_binding_semantic_assertion', 'manual_type_env_activation', 'host_routed_operator_request', 'compatible_successor_policy'",
		1,
		"v57 graph-event authority-class union",
	)
}

func typedMemoryGraphCommitExactClosureTrigger57() (string, error) {
	source, err := typedMemoryGraphCommitExactClosureTrigger54()
	if err != nil {
		return "", err
	}
	return replaceExactSQL47(
		source,
		"'non_binding_semantic_assertion', 'manual_type_env_activation'",
		"'non_binding_semantic_assertion', 'manual_type_env_activation', 'host_routed_operator_request', 'compatible_successor_policy'",
		3,
		"v57 graph-commit authority-class union",
	)
}

func typedMemoryTypeEnvActivationExactEffectTrigger57() (string, error) {
	source := typedMemoryTypeEnvActivationExactEffectTrigger47()
	return replaceExactSQL47(
		source,
		"AND event.authority_class = 'manual_type_env_activation'",
		`AND (
				(event.authority_class = 'host_routed_operator_request'
					AND authority_use.authority_generation = 'host_routed_operator_request')
				OR (event.authority_class = 'compatible_successor_policy'
					AND authority_use.authority_generation = 'compatible_successor_policy')
			)`,
		1,
		"v57 TypeEnv activation authority provenance",
	)
}

func typedMemoryGraphCommitActivationEffectTrigger57() (string, error) {
	source := typedMemoryGraphCommitActivationEffectTrigger47()
	return replaceExactSQL47(
		source,
		`AND event.authority_class =
						'manual_type_env_activation'`,
		`AND event.authority_class IN (
						'host_routed_operator_request',
						'compatible_successor_policy'
					)`,
		1,
		"v57 graph-commit activation authority provenance",
	)
}

func compatibleTypeEnvResolutionTableSQL57() string {
	return `CREATE TABLE project_typeenv_head_selection_compatible_resolutions_v1 (
		resolution_ref TEXT PRIMARY KEY CHECK(resolution_ref != ''),
		resolution_digest TEXT NOT NULL UNIQUE CHECK(` + typedMemorySHA256Shape46("resolution_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		project_binding_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("project_binding_digest") + `),
		selection_request_ref TEXT NOT NULL UNIQUE,
		selection_request_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("selection_request_digest") + `),
		content_ref TEXT NOT NULL UNIQUE,
		content_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("content_digest") + `),
		stage_ref TEXT NOT NULL UNIQUE,
		stage_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("stage_digest") + `),
		policy_edition TEXT NOT NULL CHECK(policy_edition = '` + typeEnvCompatiblePolicyEdition57 + `'),
		policy_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("policy_digest") + `),
		resolution_kind TEXT NOT NULL CHECK(resolution_kind = '` + typeEnvCompatibleResolutionKind57 + `'),
		predicate_result TEXT NOT NULL CHECK(predicate_result = 'satisfied'),
		canonical_bytes BLOB NOT NULL UNIQUE CHECK(length(canonical_bytes) > 0),
		evaluated_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("evaluated_at") + `),
		UNIQUE(resolution_ref, resolution_digest),
		FOREIGN KEY(selection_request_ref, selection_request_digest)
			REFERENCES project_typeenv_head_selection_requests(request_ref, request_digest),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(content_ref, content_digest),
		FOREIGN KEY(stage_ref)
			REFERENCES project_typeenv_stages(stage_ref),
		FOREIGN KEY(stage_digest)
			REFERENCES project_typeenv_stages(stage_digest)
	) WITHOUT ROWID`
}

func compatibleTypeEnvUseTableSQL57() string {
	return `CREATE TABLE project_typeenv_head_selection_compatible_uses_v1 (
		use_ref TEXT PRIMARY KEY CHECK(use_ref != ''),
		use_digest TEXT NOT NULL UNIQUE CHECK(` + typedMemorySHA256Shape46("use_digest") + `),
		resolution_ref TEXT NOT NULL UNIQUE,
		resolution_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("resolution_digest") + `),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		project_root TEXT NOT NULL CHECK(project_root != ''),
		selected_composite_ref TEXT NOT NULL CHECK(selected_composite_ref != ''),
		head_revision INTEGER NOT NULL CHECK(head_revision > 0),
		canonical_bytes BLOB NOT NULL UNIQUE CHECK(length(canonical_bytes) > 0),
		consumed_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("consumed_at") + `),
		UNIQUE(use_ref, use_digest),
		FOREIGN KEY(use_ref, use_digest)
			REFERENCES project_typeenv_head_selection_authority_uses(authority_use_ref, authority_use_digest),
		FOREIGN KEY(resolution_ref, resolution_digest)
			REFERENCES project_typeenv_head_selection_compatible_resolutions_v1(resolution_ref, resolution_digest)
	) WITHOUT ROWID`
}

func compatibleTypeEnvAuthorityResolutionCatalogSQL57() string {
	return `CREATE TABLE project_typeenv_head_selection_authority_resolutions (
		authority_resolution_ref TEXT PRIMARY KEY CHECK(` +
		typedMemoryNonBlankShape46("authority_resolution_ref") + `),
		authority_resolution_digest TEXT NOT NULL UNIQUE CHECK(` +
		typedMemorySHA256Shape46("authority_resolution_digest") + `),
		authority_generation TEXT NOT NULL CHECK(authority_generation IN (
			'legacy_unreproducible', '` + hostRoutedAuthorityMode56 + `',
			'` + typeEnvCompatibleAuthorityGeneration57 + `'
		)),
		project_id TEXT NOT NULL REFERENCES project_ledger_binding(project_id),
		authority_resolution_kind TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("authority_resolution_kind") + `),
		content_ref TEXT NOT NULL,
		content_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("content_digest") + `),
		request_ref TEXT NOT NULL,
		request_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("request_digest") + `),
		trusted_cli_source_ref TEXT,
		trusted_cli_source_digest TEXT,
		strict_basis_ref TEXT,
		strict_basis_digest TEXT,
		explicit_resolution_ref TEXT,
		explicit_resolution_digest TEXT,
		strict_resolution_ref TEXT,
		strict_resolution_digest TEXT,
		host_resolution_ref TEXT,
		host_resolution_digest TEXT,
		compatible_resolution_ref TEXT,
		compatible_resolution_digest TEXT,
		evaluated_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("evaluated_at") + `),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(authority_resolution_ref, authority_resolution_digest),
		FOREIGN KEY(content_ref, content_digest)
			REFERENCES project_typeenv_head_selection_authorization_contents(content_ref, content_digest),
		FOREIGN KEY(request_ref, request_digest)
			REFERENCES project_typeenv_head_selection_requests(request_ref, request_digest),
		FOREIGN KEY(trusted_cli_source_ref, trusted_cli_source_digest)
			REFERENCES project_typeenv_head_selection_trusted_cli_sources(trusted_cli_source_ref, trusted_cli_source_digest),
		FOREIGN KEY(strict_basis_ref, strict_basis_digest)
			REFERENCES project_typeenv_head_selection_authority_resolution_bases(basis_ref, basis_digest),
		FOREIGN KEY(explicit_resolution_ref, explicit_resolution_digest)
			REFERENCES project_typeenv_head_selection_explicit_policy_acceptance_resolutions(authority_resolution_ref, authority_resolution_digest)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(strict_resolution_ref, strict_resolution_digest)
			REFERENCES project_typeenv_head_selection_strict_permission_resolutions(authority_resolution_ref, authority_resolution_digest)
			DEFERRABLE INITIALLY DEFERRED,
		FOREIGN KEY(host_resolution_ref, host_resolution_digest)
			REFERENCES project_typeenv_head_selection_host_resolutions_v1(resolution_ref, resolution_digest),
		FOREIGN KEY(compatible_resolution_ref, compatible_resolution_digest)
			REFERENCES project_typeenv_head_selection_compatible_resolutions_v1(resolution_ref, resolution_digest),
		CHECK(
			(authority_generation = 'legacy_unreproducible'
				AND host_resolution_ref IS NULL
				AND host_resolution_digest IS NULL
				AND compatible_resolution_ref IS NULL
				AND compatible_resolution_digest IS NULL)
			OR
			(authority_generation = '` + hostRoutedAuthorityMode56 + `'
				AND authority_resolution_kind = '` + hostRoutedResolution56 + `'
				AND trusted_cli_source_ref IS NULL
				AND trusted_cli_source_digest IS NULL
				AND strict_basis_ref IS NULL
				AND strict_basis_digest IS NULL
				AND explicit_resolution_ref IS NULL
				AND explicit_resolution_digest IS NULL
				AND strict_resolution_ref IS NULL
				AND strict_resolution_digest IS NULL
				AND host_resolution_ref = authority_resolution_ref
				AND host_resolution_digest = authority_resolution_digest
				AND compatible_resolution_ref IS NULL
				AND compatible_resolution_digest IS NULL)
			OR
			(authority_generation = '` + typeEnvCompatibleAuthorityGeneration57 + `'
				AND authority_resolution_kind = '` + typeEnvCompatibleResolutionKind57 + `'
				AND trusted_cli_source_ref IS NULL
				AND trusted_cli_source_digest IS NULL
				AND strict_basis_ref IS NULL
				AND strict_basis_digest IS NULL
				AND explicit_resolution_ref IS NULL
				AND explicit_resolution_digest IS NULL
				AND strict_resolution_ref IS NULL
				AND strict_resolution_digest IS NULL
				AND host_resolution_ref IS NULL
				AND host_resolution_digest IS NULL
				AND compatible_resolution_ref = authority_resolution_ref
				AND compatible_resolution_digest = authority_resolution_digest)
		)
	) WITHOUT ROWID`
}

func compatibleTypeEnvAuthorityUseCatalogSQL57() string {
	value := hostRoutedTypeEnvAuthorityUseCatalogSQL56()
	value = strings.Replace(
		value,
		"authority_generation IN ('legacy_unreproducible', '"+hostRoutedAuthorityMode56+"')",
		"authority_generation IN ('legacy_unreproducible', '"+hostRoutedAuthorityMode56+"', '"+typeEnvCompatibleAuthorityGeneration57+"')",
		1,
	)
	value = strings.Replace(
		value,
		"CHECK((authority_generation = 'legacy_unreproducible') OR (authority_generation = '"+hostRoutedAuthorityMode56+"' AND authority_resolution_kind = '"+hostRoutedResolution56+"'))",
		"CHECK((authority_generation = 'legacy_unreproducible') OR "+
			"(authority_generation = '"+hostRoutedAuthorityMode56+"' AND authority_resolution_kind = '"+hostRoutedResolution56+"') OR "+
			"(authority_generation = '"+typeEnvCompatibleAuthorityGeneration57+"' AND authority_resolution_kind = '"+typeEnvCompatibleResolutionKind57+"'))",
		1,
	)
	return value
}

func compatibleTypeEnvResolutionExactSourcesTrigger57() string {
	return `CREATE TRIGGER project_typeenv_head_selection_compatible_resolutions_v1_exact_sources
	 BEFORE INSERT ON project_typeenv_head_selection_compatible_resolutions_v1
	 WHEN NOT EXISTS (
		SELECT 1 FROM project_ledger_binding binding
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = NEW.selection_request_ref
			AND request.request_digest = NEW.selection_request_digest
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
		JOIN project_typeenv_stages stage
			ON stage.stage_ref = NEW.stage_ref
			AND stage.stage_digest = NEW.stage_digest
		WHERE binding.project_id = NEW.project_id
		AND binding.project_root = NEW.project_root
		AND request.project_id = NEW.project_id
		AND request.predecessor_kind = 'transition'
		AND request.stage_ref = NEW.stage_ref
		AND request.stage_digest = NEW.stage_digest
		AND content.project_id = NEW.project_id
		AND content.request_ref = NEW.selection_request_ref
		AND content.request_digest = NEW.selection_request_digest
		AND stage.project_id = NEW.project_id
		AND NEW.evaluated_at >= content.valid_from
		AND NEW.evaluated_at < content.valid_until
	 ) BEGIN SELECT RAISE(ABORT, 'compatible TypeEnv resolution lacks exact project, request, content, and Stage sources'); END`
}

func compatibleTypeEnvCatalogResolutionExactSourceTrigger57() string {
	return `CREATE TRIGGER project_typeenv_head_selection_authority_resolutions_v57_exact_compatible
	 BEFORE INSERT ON project_typeenv_head_selection_authority_resolutions
	 WHEN NEW.authority_generation = '` + typeEnvCompatibleAuthorityGeneration57 + `'
	 AND NOT EXISTS (
		SELECT 1 FROM project_typeenv_head_selection_compatible_resolutions_v1 resolution
		WHERE resolution.resolution_ref = NEW.authority_resolution_ref
		AND resolution.resolution_digest = NEW.authority_resolution_digest
		AND resolution.project_id = NEW.project_id
		AND resolution.selection_request_ref = NEW.request_ref
		AND resolution.selection_request_digest = NEW.request_digest
		AND resolution.content_ref = NEW.content_ref
		AND resolution.content_digest = NEW.content_digest
		AND resolution.evaluated_at = NEW.evaluated_at
		AND resolution.canonical_bytes = NEW.canonical_bytes
	 ) BEGIN SELECT RAISE(ABORT, 'current TypeEnv authority resolution lacks its exact compatible-successor resolution'); END`
}

func compatibleTypeEnvCatalogUseExactSourceTrigger57() string {
	return `CREATE TRIGGER project_typeenv_head_selection_authority_uses_v57_exact_compatible
	 BEFORE INSERT ON project_typeenv_head_selection_authority_uses
	 WHEN NEW.authority_generation = '` + typeEnvCompatibleAuthorityGeneration57 + `'
	 AND NOT EXISTS (
		SELECT 1 FROM project_typeenv_head_selection_authority_resolutions resolution
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
		JOIN project_typeenv_head_selection_authorization_contents content
			ON content.content_ref = NEW.content_ref
			AND content.content_digest = NEW.content_digest
		JOIN project_typeenv_stages stage
			ON stage.stage_ref = NEW.stage_ref
			AND stage.stage_digest = NEW.stage_digest
		WHERE resolution.authority_resolution_ref = NEW.authority_resolution_ref
		AND resolution.authority_resolution_digest = NEW.authority_resolution_digest
		AND resolution.authority_generation = NEW.authority_generation
		AND resolution.authority_resolution_kind = NEW.authority_resolution_kind
		AND resolution.project_id = NEW.project_id
		AND resolution.content_ref = NEW.content_ref
		AND resolution.content_digest = NEW.content_digest
		AND resolution.request_ref = NEW.request_ref
		AND resolution.request_digest = NEW.request_digest
		AND request.project_id = NEW.project_id
		AND request.original_idempotency_key = NEW.original_idempotency_key
		AND request.predecessor_kind = NEW.predecessor_kind
		AND request.stage_ref = NEW.stage_ref
		AND request.stage_digest = NEW.stage_digest
		AND request.selected_composite_ref = NEW.selected_composite_ref
		AND content.project_id = NEW.project_id
		AND stage.project_id = NEW.project_id
	 ) BEGIN SELECT RAISE(ABORT, 'compatible TypeEnv authority use lacks its exact resolution and selection request'); END`
}

func compatibleTypeEnvUseExactCatalogTrigger57() string {
	return `CREATE TRIGGER project_typeenv_head_selection_compatible_uses_v1_exact_catalog
	 BEFORE INSERT ON project_typeenv_head_selection_compatible_uses_v1
	 WHEN NOT EXISTS (
		SELECT 1 FROM project_typeenv_head_selection_authority_uses authority_use
		JOIN project_typeenv_head_selection_compatible_resolutions_v1 resolution
			ON resolution.resolution_ref = NEW.resolution_ref
			AND resolution.resolution_digest = NEW.resolution_digest
		WHERE authority_use.authority_use_ref = NEW.use_ref
		AND authority_use.authority_use_digest = NEW.use_digest
		AND authority_use.authority_generation = '` + typeEnvCompatibleAuthorityGeneration57 + `'
		AND authority_use.authority_resolution_kind = '` + typeEnvCompatibleResolutionKind57 + `'
		AND authority_use.project_id = NEW.project_id
		AND authority_use.authority_resolution_ref = NEW.resolution_ref
		AND authority_use.authority_resolution_digest = NEW.resolution_digest
		AND authority_use.selected_composite_ref = NEW.selected_composite_ref
		AND authority_use.committed_head_revision = NEW.head_revision
		AND authority_use.canonical_bytes = NEW.canonical_bytes
		AND resolution.project_id = NEW.project_id
		AND resolution.project_root = NEW.project_root
	 ) BEGIN SELECT RAISE(ABORT, 'compatible TypeEnv authority use lacks its exact committed authority-use record'); END`
}

func currentGenerationOnlyTrigger57(table string) string {
	return `CREATE TRIGGER ` + table + `_v57_current_generation_only
	 BEFORE INSERT ON ` + table + `
	 WHEN NEW.authority_generation NOT IN ('` + hostRoutedAuthorityMode56 + `', '` + typeEnvCompatibleAuthorityGeneration57 + `')
	 BEGIN SELECT RAISE(ABORT, 'legacy TypeEnv authority is historical and cannot be reproduced'); END`
}

func compatibleTypeEnvRootGuard57(table string) string {
	return `CREATE TRIGGER ` + table + `_project_ledger_root
	 BEFORE INSERT ON ` + table + `
	 WHEN NOT EXISTS (
		SELECT 1 FROM project_ledger_binding binding
		WHERE binding.project_id = NEW.project_id
		AND binding.project_root = NEW.project_root
	 ) BEGIN SELECT RAISE(ABORT, 'compatible TypeEnv authority does not match the bound project ledger root'); END`
}
