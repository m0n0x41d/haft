package db

import (
	"database/sql"
	"fmt"
	"strings"
)

const (
	projectTypeEnvHeadSelectionSchemaVersion48 = 48

	projectTypeEnvRequestSchemaV1 = "haft.project-typeenv.head-selection-request.v1"
	projectTypeEnvRequestSchemaV2 = "haft.project-typeenv.head-selection-request.v2"

	projectTypeEnvProofObservationLegacy47 = "legacy_request_owned_v47"
	projectTypeEnvProofObservationEffectV1 = "effect_owned_head_absence_v1"
)

var projectTypeEnvHeadSelectionMigration48 = Migration{
	Version:            projectTypeEnvHeadSelectionSchemaVersion48,
	Description:        "Move Genesis head-absence proof ownership into the committed ProjectTypeEnv effect",
	Apply:              applyProjectTypeEnvHeadSelectionMigration48,
	ApplyBoundary:      ForeignKeyTableRebuildBoundary,
	ForeignKeyVerifier: verifyForeignKeys,
}

var projectTypeEnvProofEffectTables48 = []string{
	"project_typeenv_head_cas_work_records",
	"typed_memory_type_env_activations",
	"project_typeenv_head_history",
	"project_typeenv_head_selection_receipts",
	"project_typeenv_head_selection_closures",
}

var projectTypeEnvEffectProofTriggers48 = []string{
	"project_typeenv_head_cas_work_records_v48_exact_genesis_proof",
	"typed_memory_type_env_activations_v48_exact_genesis_proof",
	"project_typeenv_head_history_v48_exact_genesis_proof",
	"project_typeenv_head_selection_receipts_v48_exact_genesis_proof",
	"project_typeenv_head_selection_closures_v48_exact_genesis_proof",
}

type projectTypeEnvDependentTrigger48 struct {
	name string
	sql  string
}

func applyProjectTypeEnvHeadSelectionMigration48(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireProjectTypeEnvHeadSelectionSource48(tx); err != nil {
		return err
	}
	if err := requireAbsentProjectTypeEnvHeadSelectionFootprint48(tx); err != nil {
		return err
	}
	dependentTriggers, err := loadProjectTypeEnvDependentTriggers48(tx)
	if err != nil {
		return err
	}
	if err := executeStatements(
		tx,
		projectTypeEnvDependentTriggerDrops48(dependentTriggers),
		0,
	); err != nil {
		return fmt.Errorf(
			"detach ProjectTypeEnv v47 dependent triggers: %w",
			err,
		)
	}
	if err := executeStatements(
		tx,
		projectTypeEnvHeadSelectionStatements48(),
		0,
	); err != nil {
		return fmt.Errorf("install ProjectTypeEnv Genesis-proof effect ownership: %w", err)
	}
	if err := executeStatements(
		tx,
		projectTypeEnvDependentTriggerRestores48(dependentTriggers),
		0,
	); err != nil {
		return fmt.Errorf(
			"restore ProjectTypeEnv v47 dependent triggers: %w",
			err,
		)
	}
	if err := verifyProjectTypeEnvHeadSelectionFootprint48(tx); err != nil {
		return err
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify ProjectTypeEnv Genesis-proof effect ownership: %w", err)
	}
	return nil
}

func loadProjectTypeEnvDependentTriggers48(
	tx MigrationTransaction,
) ([]projectTypeEnvDependentTrigger48, error) {
	rows, err := tx.Query(
		`SELECT name, sql
		FROM sqlite_master
		WHERE type = 'trigger'
			AND tbl_name NOT IN (
				'project_typeenv_no_prior_head_proofs',
				'project_typeenv_head_selection_requests'
			)
			AND (
				instr(sql, 'project_typeenv_no_prior_head_proofs') > 0
				OR instr(sql, 'project_typeenv_head_selection_requests') > 0
			)
		ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"inspect ProjectTypeEnv v47 dependent triggers: %w",
			err,
		)
	}
	defer rows.Close()
	triggers := make([]projectTypeEnvDependentTrigger48, 0)
	strictTriggerFound := false
	for rows.Next() {
		var trigger projectTypeEnvDependentTrigger48
		if err := rows.Scan(&trigger.name, &trigger.sql); err != nil {
			return nil, fmt.Errorf(
				"scan ProjectTypeEnv v47 dependent trigger: %w",
				err,
			)
		}
		if trigger.name ==
			"project_typeenv_head_selection_permissions_v3_v47_exact_source" {
			strictTriggerFound = true
			continue
		}
		triggers = append(triggers, trigger)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate ProjectTypeEnv v47 dependent triggers: %w",
			err,
		)
	}
	if !strictTriggerFound {
		return nil, fmt.Errorf(
			"ProjectTypeEnv v47 strict Permission trigger was not dependency-bound",
		)
	}
	return triggers, nil
}

func projectTypeEnvDependentTriggerDrops48(
	triggers []projectTypeEnvDependentTrigger48,
) []string {
	statements := make([]string, 0, len(triggers)+1)
	statements = append(
		statements,
		`DROP TRIGGER "project_typeenv_head_selection_permissions_v3_v47_exact_source"`,
	)
	for _, trigger := range triggers {
		statements = append(
			statements,
			`DROP TRIGGER "`+strings.ReplaceAll(trigger.name, `"`, `""`)+`"`,
		)
	}
	return statements
}

func projectTypeEnvDependentTriggerRestores48(
	triggers []projectTypeEnvDependentTrigger48,
) []string {
	statements := make([]string, 0, len(triggers))
	for _, trigger := range triggers {
		statements = append(statements, trigger.sql)
	}
	return statements
}

func requireProjectTypeEnvHeadSelectionSource48(
	tx MigrationTransaction,
) error {
	var version47 int
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = ?",
		projectTypeEnvHeadSelectionSchemaVersion47,
	).Scan(&version47)
	if err != nil {
		return fmt.Errorf("inspect ProjectTypeEnv v48 source version: %w", err)
	}
	if version47 != 1 {
		return fmt.Errorf(
			"ProjectTypeEnv v48 requires exactly one applied v47 source migration",
		)
	}
	requiredTables := append(
		[]string{
			"project_typeenv_no_prior_head_proofs",
			"project_typeenv_head_selection_requests",
			"project_typeenv_head_selection_permissions_v3",
		},
		projectTypeEnvProofEffectTables48...,
	)
	for _, table := range requiredTables {
		exists, err := sqliteObjectExists48(tx, "table", table)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("ProjectTypeEnv v48 source table %s is missing", table)
		}
	}
	strictTrigger, err := sqliteObjectExists48(
		tx,
		"trigger",
		"project_typeenv_head_selection_permissions_v3_v47_exact_source",
	)
	if err != nil {
		return err
	}
	if !strictTrigger {
		return fmt.Errorf(
			"ProjectTypeEnv v48 source strict Permission trigger is missing",
		)
	}
	return nil
}

func requireAbsentProjectTypeEnvHeadSelectionFootprint48(
	tx MigrationTransaction,
) error {
	columns := []struct {
		table  string
		column string
	}{
		{"project_typeenv_no_prior_head_proofs", "observation_schema"},
		{"project_typeenv_no_prior_head_proofs", "observed_at"},
		{"project_typeenv_head_selection_requests", "request_schema"},
	}
	for _, table := range projectTypeEnvProofEffectTables48 {
		columns = append(
			columns,
			struct {
				table  string
				column string
			}{table, "no_prior_head_proof_ref"},
			struct {
				table  string
				column string
			}{table, "no_prior_head_proof_digest"},
		)
	}
	for _, coordinate := range columns {
		exists, err := sqliteColumnExists48(
			tx,
			coordinate.table,
			coordinate.column,
		)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf(
				"unknown partial v48 footprint already contains %s.%s",
				coordinate.table,
				coordinate.column,
			)
		}
	}
	for _, trigger := range append(
		[]string{
			"project_typeenv_no_prior_head_proofs_v48_exact_observation",
			"project_typeenv_head_selection_requests_v48_current_schema",
			"project_typeenv_head_selection_requests_v48_exact_predecessor",
			"project_typeenv_head_selection_permissions_v3_v48_exact_source",
		},
		projectTypeEnvEffectProofTriggers48...,
	) {
		exists, err := sqliteObjectExists48(tx, "trigger", trigger)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf(
				"unknown partial v48 footprint already contains trigger %s",
				trigger,
			)
		}
	}
	return nil
}

func projectTypeEnvHeadSelectionStatements48() []string {
	statements := []string{
		projectTypeEnvNoPriorHeadProofsTable48(),
		projectTypeEnvHeadSelectionRequestsTable48(),
		`INSERT INTO project_typeenv_no_prior_head_proofs_v48 (
			proof_ref,
			proof_digest,
			project_id,
			head_ref,
			graph_snapshot_ref,
			graph_snapshot_digest,
			expected_graph_revision,
			observation_schema,
			observed_at,
			canonical_bytes,
			recorded_at
		)
		SELECT
			proof_ref,
			proof_digest,
			project_id,
			head_ref,
			graph_snapshot_ref,
			graph_snapshot_digest,
			expected_graph_revision,
			'` + projectTypeEnvProofObservationLegacy47 + `',
			NULL,
			canonical_bytes,
			recorded_at
		FROM project_typeenv_no_prior_head_proofs`,
		`INSERT INTO project_typeenv_head_selection_requests_v48 (
			request_ref,
			request_digest,
			request_schema,
			project_id,
			head_ref,
			predecessor_kind,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			prior_head_ref,
			prior_head_revision,
			prior_selected_composite_ref,
			base_type_env_ref,
			ordered_extension_refs_digest,
			canonical_ordered_extension_refs,
			runtime_evaluation_basis_ref,
			selected_composite_ref,
			stage_ref,
			stage_digest,
			expected_graph_revision,
			original_idempotency_key,
			canonical_bytes,
			recorded_at
		)
		SELECT
			request_ref,
			request_digest,
			'` + projectTypeEnvRequestSchemaV1 + `',
			project_id,
			head_ref,
			predecessor_kind,
			no_prior_head_proof_ref,
			no_prior_head_proof_digest,
			prior_head_ref,
			prior_head_revision,
			prior_selected_composite_ref,
			base_type_env_ref,
			ordered_extension_refs_digest,
			canonical_ordered_extension_refs,
			runtime_evaluation_basis_ref,
			selected_composite_ref,
			stage_ref,
			stage_digest,
			expected_graph_revision,
			original_idempotency_key,
			canonical_bytes,
			recorded_at
		FROM project_typeenv_head_selection_requests`,
		"DROP TABLE project_typeenv_head_selection_requests",
		"DROP TABLE project_typeenv_no_prior_head_proofs",
		"ALTER TABLE project_typeenv_no_prior_head_proofs_v48 RENAME TO project_typeenv_no_prior_head_proofs",
		"ALTER TABLE project_typeenv_head_selection_requests_v48 RENAME TO project_typeenv_head_selection_requests",
	}
	for _, table := range projectTypeEnvProofEffectTables48 {
		statements = append(
			statements,
			`ALTER TABLE `+table+`
				ADD COLUMN no_prior_head_proof_ref TEXT
				REFERENCES project_typeenv_no_prior_head_proofs(proof_ref)`,
			`ALTER TABLE `+table+`
				ADD COLUMN no_prior_head_proof_digest TEXT
				REFERENCES project_typeenv_no_prior_head_proofs(proof_digest)`,
		)
	}
	statements = append(
		statements,
		`CREATE UNIQUE INDEX idx_project_typeenv_requests_project_key_v48
			ON project_typeenv_head_selection_requests(
				project_id,
				original_idempotency_key
			)`,
		`CREATE UNIQUE INDEX idx_project_typeenv_no_prior_head_proofs_legacy_coordinate_v48
			ON project_typeenv_no_prior_head_proofs(
				project_id,
				head_ref,
				graph_snapshot_ref,
				graph_snapshot_digest,
				expected_graph_revision
			)
			WHERE observation_schema = '`+projectTypeEnvProofObservationLegacy47+`'`,
		`CREATE UNIQUE INDEX idx_project_typeenv_no_prior_head_proofs_effect_observation_v48
			ON project_typeenv_no_prior_head_proofs(
				project_id,
				head_ref,
				graph_snapshot_ref,
				graph_snapshot_digest,
				expected_graph_revision,
				observed_at
			)
			WHERE observation_schema = '`+projectTypeEnvProofObservationEffectV1+`'`,
		projectTypeEnvNoReplaceTrigger48(
			"project_typeenv_no_prior_head_proofs",
			`existing.proof_ref = NEW.proof_ref
				OR existing.proof_digest = NEW.proof_digest`,
		),
		projectTypeEnvImmutableTrigger48(
			"project_typeenv_no_prior_head_proofs",
			"update",
		),
		projectTypeEnvImmutableTrigger48(
			"project_typeenv_no_prior_head_proofs",
			"delete",
		),
		projectTypeEnvNoReplaceTrigger48(
			"project_typeenv_head_selection_requests",
			`existing.request_ref = NEW.request_ref
				OR existing.request_digest = NEW.request_digest
				OR (existing.project_id = NEW.project_id
					AND existing.original_idempotency_key =
						NEW.original_idempotency_key)`,
		),
		projectTypeEnvImmutableTrigger48(
			"project_typeenv_head_selection_requests",
			"update",
		),
		projectTypeEnvImmutableTrigger48(
			"project_typeenv_head_selection_requests",
			"delete",
		),
		projectTypeEnvNoPriorHeadProofExactObservationTrigger48(),
		projectTypeEnvRequestExactPredecessorTrigger48(),
		projectTypeEnvRequestCurrentSchemaTrigger48(),
		projectTypeEnvPermissionV3ExactSourceTrigger48(),
		projectTypeEnvCASWorkExactGenesisProofTrigger48(),
		typedMemoryTypeEnvActivationExactGenesisProofTrigger48(),
		projectTypeEnvHeadHistoryExactGenesisProofTrigger48(),
		projectTypeEnvSelectionReceiptExactGenesisProofTrigger48(),
		projectTypeEnvSelectionClosureExactGenesisProofTrigger48(),
	)
	return statements
}

func projectTypeEnvNoPriorHeadProofsTable48() string {
	return `CREATE TABLE project_typeenv_no_prior_head_proofs_v48 (
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
		observation_schema TEXT NOT NULL CHECK(
			observation_schema IN (
				'` + projectTypeEnvProofObservationLegacy47 + `',
				'` + projectTypeEnvProofObservationEffectV1 + `'
			)
		),
		observed_at TEXT CHECK(
			observed_at IS NULL OR ` + sqliteCanonicalUTCNanoShape("observed_at") + `
		),
		canonical_bytes BLOB NOT NULL CHECK(length(canonical_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		UNIQUE(proof_ref, proof_digest),
		CHECK(
			(observation_schema = '` + projectTypeEnvProofObservationLegacy47 + `'
				AND observed_at IS NULL)
			OR
			(observation_schema = '` + projectTypeEnvProofObservationEffectV1 + `'
				AND observed_at IS NOT NULL
				AND ` + sqliteUTCNanoLessOrEqual("observed_at", "recorded_at") + `)
		)
	) WITHOUT ROWID`
}

func projectTypeEnvHeadSelectionRequestsTable48() string {
	return `CREATE TABLE project_typeenv_head_selection_requests_v48 (
		request_ref TEXT PRIMARY KEY CHECK(` + typedMemoryNonBlankShape46("request_ref") + `),
		request_digest TEXT NOT NULL UNIQUE CHECK(` + typedMemorySHA256Shape46("request_digest") + `),
		request_schema TEXT NOT NULL DEFAULT '` + projectTypeEnvRequestSchemaV2 + `' CHECK(
			request_schema IN (
				'` + projectTypeEnvRequestSchemaV1 + `',
				'` + projectTypeEnvRequestSchemaV2 + `'
			)
		),
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
			(request_schema = '` + projectTypeEnvRequestSchemaV1 + `'
				AND predecessor_kind = 'genesis'
				AND no_prior_head_proof_ref IS NOT NULL
				AND no_prior_head_proof_digest IS NOT NULL
				AND prior_head_ref IS NULL
				AND prior_head_revision IS NULL
				AND prior_selected_composite_ref IS NULL)
			OR
			(request_schema = '` + projectTypeEnvRequestSchemaV2 + `'
				AND predecessor_kind = 'genesis'
				AND no_prior_head_proof_ref IS NULL
				AND no_prior_head_proof_digest IS NULL
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

func projectTypeEnvNoReplaceTrigger48(
	table string,
	duplicatePredicate string,
) string {
	return `CREATE TRIGGER ` + table + `_v48_no_insert
		BEFORE INSERT ON ` + table + `
		WHEN EXISTS (
			SELECT 1 FROM ` + table + ` existing
			WHERE ` + duplicatePredicate + `
		)
		BEGIN
			SELECT RAISE(
				ABORT,
				'ProjectTypeEnv head-selection history is immutable'
			);
		END`
}

func projectTypeEnvImmutableTrigger48(
	table string,
	operation string,
) string {
	return `CREATE TRIGGER ` + table + `_v48_no_` + operation + `
		BEFORE ` + strings.ToUpper(operation) + ` ON ` + table + `
		BEGIN
			SELECT RAISE(
				ABORT,
				'ProjectTypeEnv head-selection history is immutable'
			);
		END`
}

func projectTypeEnvNoPriorHeadProofExactObservationTrigger48() string {
	return `CREATE TRIGGER project_typeenv_no_prior_head_proofs_v48_exact_observation
		BEFORE INSERT ON project_typeenv_no_prior_head_proofs
		WHEN NEW.observation_schema != '` + projectTypeEnvProofObservationEffectV1 + `'
			OR NEW.observed_at IS NULL
			OR NOT (` + sqliteUTCNanoLessOrEqual(
		"NEW.observed_at",
		"NEW.recorded_at",
	) + `)
		BEGIN
			SELECT RAISE(
				ABORT,
				'new no-prior-head proof lacks an exact effect-owned observation'
			);
		END`
}

func projectTypeEnvRequestCurrentSchemaTrigger48() string {
	return `CREATE TRIGGER project_typeenv_head_selection_requests_v48_current_schema
		BEFORE INSERT ON project_typeenv_head_selection_requests
		WHEN NEW.request_schema != '` + projectTypeEnvRequestSchemaV2 + `'
		BEGIN
			SELECT RAISE(
				ABORT,
				'legacy ProjectTypeEnv request v1 is read-only; new writes require v2'
			);
		END`
}

func projectTypeEnvRequestExactPredecessorTrigger48() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_requests_v48_exact_predecessor",
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
				(NEW.request_schema = '`+projectTypeEnvRequestSchemaV2+`'
					AND NEW.predecessor_kind = 'genesis'
					AND NEW.no_prior_head_proof_ref IS NULL
					AND NEW.no_prior_head_proof_digest IS NULL)
				OR
				(NEW.request_schema = '`+projectTypeEnvRequestSchemaV2+`'
					AND NEW.predecessor_kind = 'transition'
					AND EXISTS (
						SELECT 1
						FROM project_typeenv_head_states prior
						WHERE prior.project_id = NEW.project_id
							AND prior.head_ref = NEW.prior_head_ref
							AND prior.head_revision =
								NEW.prior_head_revision
							AND prior.selected_composite_ref =
								NEW.prior_selected_composite_ref
					))
			)`,
		"ProjectTypeEnv head-selection request lacks its exact predecessor and Stage",
	)
}

func projectTypeEnvPermissionV3ExactSourceTrigger48() string {
	return projectTypeEnvExactInsertTrigger47(
		"project_typeenv_head_selection_permissions_v3_v48_exact_source",
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
		WHERE speech.speech_act_record_ref = NEW.speech_act_record_ref
			AND speech.speech_act_record_digest =
				NEW.speech_act_record_digest
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

func projectTypeEnvCASWorkExactGenesisProofTrigger48() string {
	return projectTypeEnvExactInsertTrigger47(
		projectTypeEnvEffectProofTriggers48[0],
		"project_typeenv_head_cas_work_records",
		`SELECT 1
		FROM project_typeenv_head_selection_authority_uses authority_use
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = authority_use.request_ref
			AND request.request_digest = authority_use.request_digest
		LEFT JOIN project_typeenv_no_prior_head_proofs proof
			ON proof.proof_ref = NEW.no_prior_head_proof_ref
			AND proof.proof_digest = NEW.no_prior_head_proof_digest
		WHERE authority_use.authority_use_ref = NEW.authority_use_ref
			AND authority_use.authority_use_digest =
				NEW.authority_use_digest
			AND authority_use.project_id = NEW.project_id
			AND authority_use.work_ref = NEW.work_ref
			AND (
				(request.predecessor_kind = 'genesis'
					AND proof.observation_schema =
						'`+projectTypeEnvProofObservationEffectV1+`'
					AND proof.project_id = NEW.project_id
					AND proof.head_ref =
						'project-typeenv-head:' || NEW.project_id
					AND proof.expected_graph_revision =
						authority_use.expected_graph_revision
					AND proof.observed_at IS NOT NULL
					AND `+sqliteUTCNanoLessOrEqual(
			"NEW.work_started_at",
			"proof.observed_at",
		)+`
					AND `+sqliteUTCNanoLessOrEqual(
			"proof.observed_at",
			"NEW.effect_sealed_at",
		)+`)
				OR
				(request.predecessor_kind = 'transition'
					AND NEW.no_prior_head_proof_ref IS NULL
					AND NEW.no_prior_head_proof_digest IS NULL)
			)`,
		"ProjectTypeEnv head CAS Work record lacks its exact effect-owned Genesis proof",
	)
}

func typedMemoryTypeEnvActivationExactGenesisProofTrigger48() string {
	return projectTypeEnvExactInsertTrigger47(
		projectTypeEnvEffectProofTriggers48[1],
		"typed_memory_type_env_activations",
		`SELECT 1
		FROM project_typeenv_head_selection_requests request
		JOIN project_typeenv_head_cas_work_records work_record
			ON work_record.project_id = NEW.project_id
			AND work_record.work_ref = NEW.work_ref
		LEFT JOIN project_typeenv_no_prior_head_proofs proof
			ON proof.proof_ref = NEW.no_prior_head_proof_ref
			AND proof.proof_digest = NEW.no_prior_head_proof_digest
		WHERE request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
			AND request.project_id = NEW.project_id
			AND (
				(request.predecessor_kind = 'genesis'
					AND NEW.no_prior_head_proof_ref =
						work_record.no_prior_head_proof_ref
					AND NEW.no_prior_head_proof_digest =
						work_record.no_prior_head_proof_digest
					AND proof.observation_schema =
						'`+projectTypeEnvProofObservationEffectV1+`'
					AND proof.project_id = NEW.project_id
					AND proof.expected_graph_revision =
						NEW.expected_graph_revision)
				OR
				(request.predecessor_kind = 'transition'
					AND NEW.no_prior_head_proof_ref IS NULL
					AND NEW.no_prior_head_proof_digest IS NULL
					AND work_record.no_prior_head_proof_ref IS NULL
					AND work_record.no_prior_head_proof_digest IS NULL)
			)`,
		"typed-memory TypeEnv activation lacks its exact effect-owned Genesis proof",
	)
}

func projectTypeEnvHeadHistoryExactGenesisProofTrigger48() string {
	return projectTypeEnvExactInsertTrigger47(
		projectTypeEnvEffectProofTriggers48[2],
		"project_typeenv_head_history",
		`SELECT 1
		FROM typed_memory_type_env_activations activation
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = activation.request_ref
			AND request.request_digest = activation.request_digest
		LEFT JOIN project_typeenv_no_prior_head_proofs proof
			ON proof.proof_ref = NEW.no_prior_head_proof_ref
			AND proof.proof_digest = NEW.no_prior_head_proof_digest
		WHERE activation.project_id = NEW.project_id
			AND activation.activation_ref = NEW.activation_ref
			AND (
				(request.predecessor_kind = 'genesis'
					AND NEW.no_prior_head_proof_ref =
						activation.no_prior_head_proof_ref
					AND NEW.no_prior_head_proof_digest =
						activation.no_prior_head_proof_digest
					AND proof.observation_schema =
						'`+projectTypeEnvProofObservationEffectV1+`'
					AND proof.project_id = NEW.project_id
					AND proof.expected_graph_revision =
						request.expected_graph_revision)
				OR
				(request.predecessor_kind = 'transition'
					AND NEW.no_prior_head_proof_ref IS NULL
					AND NEW.no_prior_head_proof_digest IS NULL
					AND activation.no_prior_head_proof_ref IS NULL
					AND activation.no_prior_head_proof_digest IS NULL)
			)`,
		"ProjectTypeEnv head history lacks its exact effect-owned Genesis proof",
	)
}

func projectTypeEnvSelectionReceiptExactGenesisProofTrigger48() string {
	return projectTypeEnvExactInsertTrigger47(
		projectTypeEnvEffectProofTriggers48[3],
		"project_typeenv_head_selection_receipts",
		`SELECT 1
		FROM project_typeenv_head_cas_work_records work_record
		JOIN typed_memory_type_env_activations activation
			ON activation.project_id = NEW.project_id
			AND activation.activation_ref = NEW.activation_ref
		JOIN project_typeenv_head_history history
			ON history.project_id = NEW.project_id
			AND history.head_revision = NEW.head_revision
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = NEW.request_ref
			AND request.request_digest = NEW.request_digest
		LEFT JOIN project_typeenv_no_prior_head_proofs proof
			ON proof.proof_ref = NEW.no_prior_head_proof_ref
			AND proof.proof_digest = NEW.no_prior_head_proof_digest
		WHERE work_record.cas_work_record_ref = NEW.cas_work_record_ref
			AND work_record.cas_work_record_digest =
				NEW.cas_work_record_digest
			AND work_record.project_id = NEW.project_id
			AND (
				(request.predecessor_kind = 'genesis'
					AND NEW.no_prior_head_proof_ref =
						work_record.no_prior_head_proof_ref
					AND NEW.no_prior_head_proof_digest =
						work_record.no_prior_head_proof_digest
					AND NEW.no_prior_head_proof_ref =
						activation.no_prior_head_proof_ref
					AND NEW.no_prior_head_proof_digest =
						activation.no_prior_head_proof_digest
					AND NEW.no_prior_head_proof_ref =
						history.no_prior_head_proof_ref
					AND NEW.no_prior_head_proof_digest =
						history.no_prior_head_proof_digest
					AND proof.observation_schema =
						'`+projectTypeEnvProofObservationEffectV1+`'
					AND proof.project_id = NEW.project_id)
				OR
				(request.predecessor_kind = 'transition'
					AND NEW.no_prior_head_proof_ref IS NULL
					AND NEW.no_prior_head_proof_digest IS NULL
					AND work_record.no_prior_head_proof_ref IS NULL
					AND work_record.no_prior_head_proof_digest IS NULL
					AND activation.no_prior_head_proof_ref IS NULL
					AND activation.no_prior_head_proof_digest IS NULL
					AND history.no_prior_head_proof_ref IS NULL
					AND history.no_prior_head_proof_digest IS NULL)
			)`,
		"ProjectTypeEnv head-selection receipt lacks its exact effect-owned Genesis proof",
	)
}

func projectTypeEnvSelectionClosureExactGenesisProofTrigger48() string {
	return projectTypeEnvExactInsertTrigger47(
		projectTypeEnvEffectProofTriggers48[4],
		"project_typeenv_head_selection_closures",
		`SELECT 1
		FROM project_typeenv_head_selection_receipts receipt
		JOIN project_typeenv_head_cas_work_records work_record
			ON work_record.cas_work_record_ref =
				receipt.cas_work_record_ref
			AND work_record.cas_work_record_digest =
				receipt.cas_work_record_digest
		JOIN typed_memory_type_env_activations activation
			ON activation.project_id = receipt.project_id
			AND activation.activation_ref = receipt.activation_ref
		JOIN project_typeenv_head_history history
			ON history.project_id = receipt.project_id
			AND history.head_revision = receipt.head_revision
		JOIN project_typeenv_head_selection_requests request
			ON request.request_ref = receipt.request_ref
			AND request.request_digest = receipt.request_digest
		LEFT JOIN project_typeenv_no_prior_head_proofs proof
			ON proof.proof_ref = NEW.no_prior_head_proof_ref
			AND proof.proof_digest = NEW.no_prior_head_proof_digest
		WHERE receipt.receipt_ref = NEW.receipt_ref
			AND receipt.receipt_digest = NEW.receipt_digest
			AND receipt.project_id = NEW.project_id
			AND (
				(request.predecessor_kind = 'genesis'
					AND NEW.no_prior_head_proof_ref =
						receipt.no_prior_head_proof_ref
					AND NEW.no_prior_head_proof_digest =
						receipt.no_prior_head_proof_digest
					AND NEW.no_prior_head_proof_ref =
						work_record.no_prior_head_proof_ref
					AND NEW.no_prior_head_proof_digest =
						work_record.no_prior_head_proof_digest
					AND NEW.no_prior_head_proof_ref =
						activation.no_prior_head_proof_ref
					AND NEW.no_prior_head_proof_digest =
						activation.no_prior_head_proof_digest
					AND NEW.no_prior_head_proof_ref =
						history.no_prior_head_proof_ref
					AND NEW.no_prior_head_proof_digest =
						history.no_prior_head_proof_digest
					AND proof.observation_schema =
						'`+projectTypeEnvProofObservationEffectV1+`'
					AND proof.project_id = NEW.project_id)
				OR
				(request.predecessor_kind = 'transition'
					AND NEW.no_prior_head_proof_ref IS NULL
					AND NEW.no_prior_head_proof_digest IS NULL
					AND receipt.no_prior_head_proof_ref IS NULL
					AND receipt.no_prior_head_proof_digest IS NULL
					AND work_record.no_prior_head_proof_ref IS NULL
					AND work_record.no_prior_head_proof_digest IS NULL
					AND activation.no_prior_head_proof_ref IS NULL
					AND activation.no_prior_head_proof_digest IS NULL
					AND history.no_prior_head_proof_ref IS NULL
					AND history.no_prior_head_proof_digest IS NULL)
			)`,
		"ProjectTypeEnv head-selection closure lacks its exact effect-owned Genesis proof",
	)
}

func verifyProjectTypeEnvHeadSelectionFootprint48(
	tx MigrationTransaction,
) error {
	columns := []struct {
		table  string
		column string
	}{
		{"project_typeenv_no_prior_head_proofs", "observation_schema"},
		{"project_typeenv_no_prior_head_proofs", "observed_at"},
		{"project_typeenv_head_selection_requests", "request_schema"},
	}
	for _, table := range projectTypeEnvProofEffectTables48 {
		columns = append(
			columns,
			struct {
				table  string
				column string
			}{table, "no_prior_head_proof_ref"},
			struct {
				table  string
				column string
			}{table, "no_prior_head_proof_digest"},
		)
	}
	for _, coordinate := range columns {
		exists, err := sqliteColumnExists48(
			tx,
			coordinate.table,
			coordinate.column,
		)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf(
				"ProjectTypeEnv v48 column %s.%s is missing",
				coordinate.table,
				coordinate.column,
			)
		}
	}
	for _, trigger := range append(
		[]string{
			"project_typeenv_no_prior_head_proofs_v48_exact_observation",
			"project_typeenv_head_selection_requests_v48_current_schema",
			"project_typeenv_head_selection_requests_v48_exact_predecessor",
			"project_typeenv_head_selection_permissions_v3_v48_exact_source",
		},
		projectTypeEnvEffectProofTriggers48...,
	) {
		exists, err := sqliteObjectExists48(tx, "trigger", trigger)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("ProjectTypeEnv v48 trigger %s is missing", trigger)
		}
	}
	oldStrictTrigger, err := sqliteObjectExists48(
		tx,
		"trigger",
		"project_typeenv_head_selection_permissions_v3_v47_exact_source",
	)
	if err != nil {
		return err
	}
	if oldStrictTrigger {
		return fmt.Errorf("ProjectTypeEnv v47 strict Permission trigger survived v48")
	}
	return nil
}

func sqliteObjectExists48(
	tx MigrationTransaction,
	objectType string,
	name string,
) (bool, error) {
	var count int
	err := tx.QueryRow(
		`SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = ? AND name = ?`,
		objectType,
		name,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf(
			"inspect SQLite %s %s: %w",
			objectType,
			name,
			err,
		)
	}
	return count == 1, nil
}

func sqliteColumnExists48(
	tx MigrationTransaction,
	table string,
	column string,
) (bool, error) {
	rows, err := tx.Query("PRAGMA table_xinfo(" + table + ")")
	if err != nil {
		return false, fmt.Errorf("inspect SQLite table %s: %w", table, err)
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
			return false, fmt.Errorf(
				"scan SQLite table %s column: %w",
				table,
				err,
			)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf(
			"iterate SQLite table %s columns: %w",
			table,
			err,
		)
	}
	return false, nil
}
