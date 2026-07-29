package db

import (
	"bytes"
	"fmt"
	"strings"
)

const typedMemoryKindClassificationSchemaVersion54 = 54

const (
	typedMemoryKindClassificationWriterCapabilitiesTable54  = "typed_memory_writer_capabilities_v54"
	typedMemoryKindClassificationSourceBlobsTable54         = "typed_memory_kind_classification_source_blobs_v54"
	typedMemoryKindClassificationEvaluationsTable54         = "typed_memory_kind_classification_evaluations_v54"
	typedMemoryKindClassificationFeaturesTable54            = "typed_memory_kind_classification_features_v54"
	typedMemoryRelationalAssertionClassificationUsesTable54 = "typed_memory_relational_assertion_classification_uses_v54"
)

const (
	typedMemoryKindClassificationCapabilityKey54 = "typed_memory_kind_classification_writer_generation"
	typedMemoryKindClassificationWriter54        = 54
	typedMemoryKindClassificationMarkerBytes54   = "haft.typed-memory.storage.kind-classification-writer-generation=54"
	typedMemoryKindClassificationMarkerDigest54  = "sha256:1395bb80205b84b5b6a57e1e4a9b71ec559f93b0c50e518847a408a68ffcbe37"
)

var typedMemoryKindClassificationMigration54 = Migration{
	Version:            typedMemoryKindClassificationSchemaVersion54,
	Description:        "Add current KindClassification storage without reinterpreting historical MemberOf rows",
	Apply:              applyTypedMemoryKindClassificationMigration54,
	ApplyBoundary:      ForeignKeyTableRebuildBoundary,
	ForeignKeyVerifier: verifyForeignKeys,
}

func applyTypedMemoryKindClassificationMigration54(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireTypedMemoryKindClassificationSource54(tx); err != nil {
		return err
	}
	if err := requireAbsentTypedMemoryKindClassificationFootprint54(tx); err != nil {
		return err
	}
	statements, err := typedMemoryKindClassificationStatements54()
	if err != nil {
		return err
	}
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install kind-classification storage generation 54: %w", err)
	}
	if err := verifyTypedMemoryKindClassificationFootprint54(tx); err != nil {
		return err
	}
	if err := verifyHistoricalTypedMemoryFootprints49(tx); err != nil {
		return fmt.Errorf("verify historical typed-memory footprints after v54: %w", err)
	}
	return verifyForeignKeys(tx)
}

func requireTypedMemoryKindClassificationSource54(
	tx MigrationTransaction,
) error {
	var maximumVersion int
	err := tx.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&maximumVersion)
	if err != nil {
		return fmt.Errorf("inspect kind-classification source schema: %w", err)
	}
	if maximumVersion != typedMemoryRelationalAssertionSchemaVersion53 {
		return fmt.Errorf(
			"kind-classification storage requires exact schema version 53, found %d",
			maximumVersion,
		)
	}
	var generation int
	var digest string
	var canonical []byte
	err = tx.QueryRow(
		`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_writer_capabilities_v53
		WHERE capability_key = ?`,
		typedMemoryWriterGenerationCapability53,
	).Scan(&generation, &digest, &canonical)
	if err != nil {
		return fmt.Errorf("load exact v53 writer capability: %w", err)
	}
	matches := generation == typedMemoryWriterGeneration53
	matches = matches && digest == typedMemoryWriterMarkerDigest53
	matches = matches && bytes.Equal(canonical, []byte(typedMemoryWriterMarkerBytes53))
	if !matches {
		return fmt.Errorf("v53 writer capability differs from its sealed bytes")
	}
	return verifyForeignKeys(tx)
}

func requireAbsentTypedMemoryKindClassificationFootprint54(
	tx MigrationTransaction,
) error {
	names := []string{
		typedMemoryKindClassificationWriterCapabilitiesTable54,
		typedMemoryKindClassificationSourceBlobsTable54,
		typedMemoryKindClassificationEvaluationsTable54,
		typedMemoryKindClassificationFeaturesTable54,
		typedMemoryRelationalAssertionClassificationUsesTable54,
	}
	for _, name := range names {
		var count int
		err := tx.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE name = ?",
			name,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("inspect v54 object %s: %w", name, err)
		}
		if count != 0 {
			return fmt.Errorf(
				"kind-classification storage refused unknown partial v54 footprint: %s already exists",
				name,
			)
		}
	}
	return nil
}

func typedMemoryKindClassificationStatements54() ([]string, error) {
	admissionTable, err := typedMemoryEventAdmissionBasesTable54()
	if err != nil {
		return nil, err
	}
	closureTable, err := typedMemoryCommitMaterializationClosuresTable54()
	if err != nil {
		return nil, err
	}
	footprintView, err := typedMemoryEventMaterializationFootprintsView54()
	if err != nil {
		return nil, err
	}
	exactClosure, err := typedMemoryCommitClosureExactFootprintTrigger54()
	if err != nil {
		return nil, err
	}
	graphCommit, err := typedMemoryGraphCommitExactClosureTrigger54()
	if err != nil {
		return nil, err
	}
	statements := []string{
		`CREATE TEMP TABLE typed_memory_event_writer_generations_v54_backup AS
		SELECT * FROM typed_memory_event_writer_generations`,
		`CREATE TEMP TABLE typed_memory_event_admission_bases_v54_backup AS
		SELECT * FROM typed_memory_event_admission_bases`,
		`CREATE TEMP TABLE typed_memory_commit_materialization_closures_v54_backup AS
		SELECT * FROM typed_memory_commit_materialization_closures`,
		"DROP TRIGGER typed_memory_graph_commits_exact_closure",
		"DROP TRIGGER typed_memory_commit_materialization_closures_v46_exact_footprint",
		"DROP TRIGGER typed_memory_commit_materialization_closures_v46_basis_kind",
		"DROP VIEW typed_memory_event_materialization_footprints_v46",
		"DROP TRIGGER typed_memory_relational_assertions_v3_v53_exact_event",
		"DROP TRIGGER typed_memory_event_writer_generations_v53_exact_boundary",
		"DROP TABLE typed_memory_commit_materialization_closures",
		"DROP TABLE typed_memory_event_admission_bases",
		"DROP TABLE typed_memory_event_writer_generations",
		typedMemoryEventWriterGenerationsTable54(),
		admissionTable,
		closureTable,
		`INSERT INTO typed_memory_event_writer_generations (
			project_id, event_ref, writer_generation, provenance_kind
		) SELECT project_id, event_ref, writer_generation, provenance_kind
		FROM typed_memory_event_writer_generations_v54_backup`,
		`INSERT INTO typed_memory_event_admission_bases (
			project_id, event_ref, event_digest, admission_basis_kind,
			type_env_ref, basis_graph_revision,
			request_digest, canonical_request_bytes,
			semantic_digest, canonical_semantic_bytes,
			admission_envelope_digest, canonical_admission_envelope_bytes,
			admission_basis_digest, canonical_admission_basis_bytes,
			materialization_manifest_digest,
			canonical_materialization_manifest_bytes, recorded_at
		) SELECT
			project_id, event_ref, event_digest, admission_basis_kind,
			type_env_ref, basis_graph_revision,
			request_digest, canonical_request_bytes,
			semantic_digest, canonical_semantic_bytes,
			admission_envelope_digest, canonical_admission_envelope_bytes,
			admission_basis_digest, canonical_admission_basis_bytes,
			materialization_manifest_digest,
			canonical_materialization_manifest_bytes, recorded_at
		FROM typed_memory_event_admission_bases_v54_backup`,
		`INSERT INTO typed_memory_commit_materialization_closures (
			project_id, event_ref, commit_ref, event_digest,
			admission_basis_kind, request_digest, semantic_digest,
			admission_envelope_digest, admission_basis_digest,
			materialization_manifest_digest,
			materialization_digest, canonical_materialization_bytes,
			entity_count, entity_context_count, entity_declaration_count,
			context_slice_catalog_count, context_slice_count,
			value_blob_count, observable_input_blob_count, relation_count,
			relation_slot_count, relation_filler_count,
			ordered_candidate_prefix_count,
			reference_resolution_use_count, memberof_evaluation_count,
			memberof_input_count, memberof_use_count,
			kind_classification_source_blob_count,
			kind_classification_evaluation_count,
			kind_classification_feature_count,
			kind_classification_use_count,
			alias_change_count, retraction_count,
			type_env_activation_count, recorded_at
		) SELECT
			project_id, event_ref, commit_ref, event_digest,
			admission_basis_kind, request_digest, semantic_digest,
			admission_envelope_digest, admission_basis_digest,
			materialization_manifest_digest,
			materialization_digest, canonical_materialization_bytes,
			entity_count, entity_context_count, entity_declaration_count,
			context_slice_catalog_count, context_slice_count,
			value_blob_count, observable_input_blob_count, relation_count,
			relation_slot_count, relation_filler_count,
			ordered_candidate_prefix_count,
			reference_resolution_use_count, memberof_evaluation_count,
			memberof_input_count, memberof_use_count,
			0, 0, 0, 0,
			alias_change_count, retraction_count,
			type_env_activation_count, recorded_at
		FROM typed_memory_commit_materialization_closures_v54_backup`,
		"DROP TABLE typed_memory_event_writer_generations_v54_backup",
		"DROP TABLE typed_memory_event_admission_bases_v54_backup",
		"DROP TABLE typed_memory_commit_materialization_closures_v54_backup",
		typedMemoryKindClassificationWriterCapabilitiesTableStatement54(),
		typedMemoryKindClassificationSourceBlobsTableStatement54(),
		typedMemoryKindClassificationEvaluationsTableStatement54(),
		typedMemoryKindClassificationFeaturesTableStatement54(),
		typedMemoryRelationalAssertionClassificationUsesTableStatement54(),
		typedMemoryKindClassificationWriterCapabilityInsert54(),
		footprintView,
		typedMemoryEventWriterGenerationExactBoundaryTrigger54(),
		typedMemoryEventAdmissionExactEventTrigger46(),
		typedMemoryCommitClosureBasisKindTrigger54(),
		exactClosure,
		graphCommit,
		typedMemoryRelationalAssertionExactEventTrigger54(),
	}
	statements = append(statements, typedMemoryKindClassificationIndexStatements54()...)
	statements = append(statements, typedMemoryKindClassificationTriggerStatements54()...)
	statements = append(
		statements,
		immutableTypedMemoryTrigger46("typed_memory_event_writer_generations", "update"),
		immutableTypedMemoryTrigger46("typed_memory_event_writer_generations", "delete"),
		typedMemoryOpenEventTrigger46("typed_memory_event_writer_generations"),
		immutableTypedMemoryTrigger46("typed_memory_event_admission_bases", "update"),
		immutableTypedMemoryTrigger46("typed_memory_event_admission_bases", "delete"),
		typedMemoryOpenEventTrigger46("typed_memory_event_admission_bases"),
		immutableTypedMemoryTrigger46("typed_memory_commit_materialization_closures", "update"),
		immutableTypedMemoryTrigger46("typed_memory_commit_materialization_closures", "delete"),
		typedMemoryOpenEventTrigger46("typed_memory_commit_materialization_closures"),
	)
	return statements, nil
}

func typedMemoryEventWriterGenerationsTable54() string {
	return `CREATE TABLE typed_memory_event_writer_generations (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		writer_generation INTEGER NOT NULL CHECK(writer_generation IN (45, 46, 53, 54)),
		provenance_kind TEXT NOT NULL CHECK(
			(writer_generation = 45 AND provenance_kind = 'migration_v45_backfill')
			OR (writer_generation = 46 AND provenance_kind = 'writer_v46')
			OR (writer_generation = 53 AND provenance_kind = 'writer_v53')
			OR (writer_generation = 54 AND provenance_kind = 'writer_v54')
		),
		PRIMARY KEY(project_id, event_ref),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryEventAdmissionBasesTable54() (string, error) {
	source, err := typedMemoryEventAdmissionBasesTable47(
		"typed_memory_event_admission_bases",
	)
	if err != nil {
		return "", err
	}
	needle := "admission_basis_kind IN ('snapshot_only', 'context_slice_membership')"
	replacement := "admission_basis_kind IN ('snapshot_only', 'context_slice_membership', 'context_slice_classification')"
	if strings.Count(source, needle) != 1 {
		return "", fmt.Errorf("v54 admission-basis table source seam changed")
	}
	return strings.Replace(source, needle, replacement, 1), nil
}

func typedMemoryCommitMaterializationClosuresTable54() (string, error) {
	source := typedMemoryCommitMaterializationClosuresTable46()
	basisNeedle := "admission_basis_kind IN ('snapshot_only', 'context_slice_membership')"
	basisReplacement := "admission_basis_kind IN ('snapshot_only', 'context_slice_membership', 'context_slice_classification')"
	if strings.Count(source, basisNeedle) != 1 {
		return "", fmt.Errorf("v54 materialization-closure basis seam changed")
	}
	source = strings.Replace(source, basisNeedle, basisReplacement, 1)
	countNeedle := `		memberof_use_count INTEGER NOT NULL CHECK(memberof_use_count >= 0),
		alias_change_count INTEGER NOT NULL CHECK(alias_change_count >= 0),`
	countReplacement := `		memberof_use_count INTEGER NOT NULL CHECK(memberof_use_count >= 0),
		kind_classification_source_blob_count INTEGER NOT NULL DEFAULT 0 CHECK(kind_classification_source_blob_count >= 0),
		kind_classification_evaluation_count INTEGER NOT NULL DEFAULT 0 CHECK(kind_classification_evaluation_count >= 0),
		kind_classification_feature_count INTEGER NOT NULL DEFAULT 0 CHECK(kind_classification_feature_count >= 0),
		kind_classification_use_count INTEGER NOT NULL DEFAULT 0 CHECK(kind_classification_use_count >= 0),
		alias_change_count INTEGER NOT NULL CHECK(alias_change_count >= 0),`
	if strings.Count(source, countNeedle) != 1 {
		return "", fmt.Errorf("v54 materialization-closure count seam changed")
	}
	source = strings.Replace(source, countNeedle, countReplacement, 1)
	recordedAtNeedle := `		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),`
	recordedAtReplacement := `		type_env_activation_count INTEGER NOT NULL DEFAULT 0 CHECK(type_env_activation_count >= 0),
` + recordedAtNeedle
	if strings.Count(source, recordedAtNeedle) != 1 {
		return "", fmt.Errorf("v54 materialization-closure activation-count seam changed")
	}
	return strings.Replace(source, recordedAtNeedle, recordedAtReplacement, 1), nil
}

func typedMemoryKindClassificationWriterCapabilitiesTableStatement54() string {
	return `CREATE TABLE typed_memory_writer_capabilities_v54 (
		capability_key TEXT PRIMARY KEY CHECK(
			capability_key = 'typed_memory_kind_classification_writer_generation'
		),
		writer_generation INTEGER NOT NULL CHECK(writer_generation = 54),
		capability_digest TEXT NOT NULL UNIQUE CHECK(
			capability_digest = 'sha256:1395bb80205b84b5b6a57e1e4a9b71ec559f93b0c50e518847a408a68ffcbe37'
		),
		canonical_bytes BLOB NOT NULL UNIQUE CHECK(
			CAST(canonical_bytes AS TEXT) =
				'haft.typed-memory.storage.kind-classification-writer-generation=54'
		),
		installed_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("installed_at") + `)
	) WITHOUT ROWID`
}

func typedMemoryKindClassificationWriterCapabilityInsert54() string {
	return `INSERT INTO typed_memory_writer_capabilities_v54 (
		capability_key, writer_generation, capability_digest,
		canonical_bytes, installed_at
	) VALUES (
		'typed_memory_kind_classification_writer_generation',
		54,
		'sha256:1395bb80205b84b5b6a57e1e4a9b71ec559f93b0c50e518847a408a68ffcbe37',
		CAST('haft.typed-memory.storage.kind-classification-writer-generation=54' AS BLOB),
		strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	)`
}

func typedMemoryKindClassificationSourceBlobsTableStatement54() string {
	return `CREATE TABLE typed_memory_kind_classification_source_blobs_v54 (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		source_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("source_ref") + `),
		source_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("source_digest") + `),
		canonical_source_bytes BLOB NOT NULL CHECK(length(canonical_source_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, source_ref),
		UNIQUE(project_id, event_ref, source_ref, source_digest),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryKindClassificationEvaluationsTableStatement54() string {
	return `CREATE TABLE typed_memory_kind_classification_evaluations_v54 (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		evaluation_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("evaluation_ref") + `),
		judgement_kind TEXT NOT NULL CHECK(judgement_kind IN ('true', 'false')),
		entity_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("entity_id") + `),
		candidate_value_kind_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("candidate_value_kind_ref") + `),
		local_value_kind_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("local_value_kind_ref") + `),
		signature_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("signature_ref") + `),
		context_slice_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("context_slice_ref") + `),
		criterion_rule_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("criterion_rule_ref") + `),
		feature_set_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("feature_set_digest") + `),
		request_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("request_digest") + `),
		canonical_request_bytes BLOB NOT NULL CHECK(length(canonical_request_bytes) > 0),
		basis_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("basis_digest") + `),
		canonical_basis_bytes BLOB NOT NULL CHECK(length(canonical_basis_bytes) > 0),
		judgement_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("judgement_digest") + `),
		canonical_judgement_bytes BLOB NOT NULL CHECK(length(canonical_judgement_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, evaluation_ref),
		UNIQUE(project_id, event_ref, judgement_digest),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(project_id, event_ref, context_slice_ref)
			REFERENCES typed_memory_context_slices(project_id, event_ref, context_slice_ref)
	) WITHOUT ROWID`
}

func typedMemoryKindClassificationFeaturesTableStatement54() string {
	return `CREATE TABLE typed_memory_kind_classification_features_v54 (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		evaluation_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("evaluation_ref") + `),
		feature_ordinal INTEGER NOT NULL CHECK(feature_ordinal >= 0),
		source_kind TEXT NOT NULL CHECK(source_kind IN ('internal_visibility', 'external_blob')),
		source_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("source_ref") + `),
		source_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("source_digest") + `),
		feature_key TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("feature_key") + `),
		governor_rule_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("governor_rule_ref") + `),
		feature_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("feature_digest") + `),
		canonical_feature_bytes BLOB NOT NULL CHECK(length(canonical_feature_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, evaluation_ref, feature_ordinal),
		UNIQUE(project_id, event_ref, evaluation_ref, feature_key),
		FOREIGN KEY(project_id, event_ref, evaluation_ref)
			REFERENCES typed_memory_kind_classification_evaluations_v54(
				project_id, event_ref, evaluation_ref
			)
	) WITHOUT ROWID`
}

func typedMemoryRelationalAssertionClassificationUsesTableStatement54() string {
	return `CREATE TABLE typed_memory_relational_assertion_classification_uses_v54 (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		assertion_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("assertion_id") + `),
		slot_ordinal INTEGER NOT NULL CHECK(slot_ordinal >= 0),
		filler_ordinal INTEGER NOT NULL CHECK(filler_ordinal >= 0),
		filler_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("filler_digest") + `),
		use_kind TEXT NOT NULL CHECK(use_kind IN ('required_true', 'disjoint_false')),
		constraint_id TEXT NOT NULL DEFAULT '',
		queried_value_kind_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("queried_value_kind_ref") + `),
		request_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("request_digest") + `),
		evaluation_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("evaluation_ref") + `),
		expected_judgement_kind TEXT NOT NULL CHECK(expected_judgement_kind IN ('true', 'false')),
		use_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("use_digest") + `),
		canonical_use_bytes BLOB NOT NULL CHECK(length(canonical_use_bytes) > 0),
		PRIMARY KEY(
			project_id, event_ref, change_ordinal, slot_ordinal,
			filler_ordinal, use_kind, constraint_id, queried_value_kind_ref
		),
		FOREIGN KEY(
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest
		) REFERENCES typed_memory_relational_assertion_fillers_v3(
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest
		),
		FOREIGN KEY(project_id, event_ref, evaluation_ref)
			REFERENCES typed_memory_kind_classification_evaluations_v54(
				project_id, event_ref, evaluation_ref
			),
		CHECK(
			(use_kind = 'required_true'
				AND constraint_id = ''
				AND expected_judgement_kind = 'true')
			OR (use_kind = 'disjoint_false'
				AND constraint_id != ''
				AND trim(constraint_id) = constraint_id
				AND expected_judgement_kind = 'false')
		)
	) WITHOUT ROWID`
}

func typedMemoryEventMaterializationFootprintsView54() (string, error) {
	source, err := typedMemoryEventMaterializationFootprintsView53()
	if err != nil {
		return "", err
	}
	needle := ` AS memberof_use_count,
		(SELECT COUNT(*) FROM typed_memory_alias_changes alias_change`
	replacement := ` AS memberof_use_count,
		(SELECT COUNT(*) FROM typed_memory_kind_classification_source_blobs_v54 source_blob
			WHERE source_blob.project_id = event.project_id
				AND source_blob.event_ref = event.event_ref) AS kind_classification_source_blob_count,
		(SELECT COUNT(*) FROM typed_memory_kind_classification_evaluations_v54 evaluation
			WHERE evaluation.project_id = event.project_id
				AND evaluation.event_ref = event.event_ref) AS kind_classification_evaluation_count,
		(SELECT COUNT(*) FROM typed_memory_kind_classification_features_v54 feature
			WHERE feature.project_id = event.project_id
				AND feature.event_ref = event.event_ref) AS kind_classification_feature_count,
		(SELECT COUNT(*) FROM typed_memory_relational_assertion_classification_uses_v54 classification_use
			WHERE classification_use.project_id = event.project_id
				AND classification_use.event_ref = event.event_ref) AS kind_classification_use_count,
		(SELECT COUNT(*) FROM typed_memory_alias_changes alias_change`
	if strings.Count(source, needle) != 1 {
		return "", fmt.Errorf("v54 materialization footprint source seam changed")
	}
	return strings.Replace(source, needle, replacement, 1), nil
}

func typedMemoryCommitClosureBasisKindTrigger54() string {
	return `CREATE TRIGGER typed_memory_commit_materialization_closures_v46_basis_kind
	BEFORE INSERT ON typed_memory_commit_materialization_closures
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_event_admission_bases admission
		WHERE admission.project_id = NEW.project_id
			AND admission.event_ref = NEW.event_ref
			AND admission.admission_basis_kind = NEW.admission_basis_kind
			AND admission.event_digest = NEW.event_digest
			AND admission.request_digest = NEW.request_digest
			AND admission.semantic_digest = NEW.semantic_digest
			AND admission.admission_envelope_digest = NEW.admission_envelope_digest
			AND admission.admission_basis_digest = NEW.admission_basis_digest
			AND admission.materialization_manifest_digest = NEW.materialization_manifest_digest
			AND (
				(admission.admission_basis_kind = 'snapshot_only'
					AND NEW.ordered_candidate_prefix_count = 0
					AND NEW.reference_resolution_use_count = 0
					AND NEW.observable_input_blob_count = 0
					AND NEW.memberof_evaluation_count = 0
					AND NEW.memberof_input_count = 0
					AND NEW.memberof_use_count = 0
					AND NEW.kind_classification_source_blob_count = 0
					AND NEW.kind_classification_evaluation_count = 0
					AND NEW.kind_classification_feature_count = 0
					AND NEW.kind_classification_use_count = 0)
				OR (admission.admission_basis_kind = 'context_slice_membership'
					AND NEW.context_slice_count >= 1
					AND NEW.reference_resolution_use_count >= 1
					AND NEW.observable_input_blob_count >= 1
					AND NEW.memberof_evaluation_count >= 1
					AND NEW.memberof_input_count >= 1
					AND NEW.memberof_use_count >= 1
					AND NEW.kind_classification_source_blob_count = 0
					AND NEW.kind_classification_evaluation_count = 0
					AND NEW.kind_classification_feature_count = 0
					AND NEW.kind_classification_use_count = 0)
				OR (admission.admission_basis_kind = 'context_slice_classification'
					AND NEW.context_slice_count >= 1
					AND NEW.reference_resolution_use_count >= 1
					AND NEW.observable_input_blob_count = 0
					AND NEW.memberof_evaluation_count = 0
					AND NEW.memberof_input_count = 0
					AND NEW.memberof_use_count = 0
					AND NEW.kind_classification_evaluation_count >= 1
					AND NEW.kind_classification_feature_count >= 1
					AND NEW.kind_classification_use_count >= 1)
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory materialization closure does not match its sealed admission-basis kind');
	END`
}

func typedMemoryCommitClosureExactFootprintTrigger54() (string, error) {
	source, err := typedMemoryCommitClosureExactFootprintTrigger53()
	if err != nil {
		return "", err
	}
	countNeedle := `			AND NEW.memberof_use_count = footprint.memberof_use_count
			AND NEW.alias_change_count = footprint.alias_change_count`
	countReplacement := `			AND NEW.memberof_use_count = footprint.memberof_use_count
			AND NEW.kind_classification_source_blob_count = footprint.kind_classification_source_blob_count
			AND NEW.kind_classification_evaluation_count = footprint.kind_classification_evaluation_count
			AND NEW.kind_classification_feature_count = footprint.kind_classification_feature_count
			AND NEW.kind_classification_use_count = footprint.kind_classification_use_count
			AND NEW.alias_change_count = footprint.alias_change_count`
	if strings.Count(source, countNeedle) != 1 {
		return "", fmt.Errorf("v54 exact-footprint count seam changed")
	}
	source = strings.Replace(source, countNeedle, countReplacement, 1)
	memberNeedle := `						OR NOT EXISTS (
							SELECT 1 FROM typed_memory_relational_assertion_memberof_uses_v3 member_use
							WHERE member_use.project_id = filler.project_id
								AND member_use.event_ref = filler.event_ref
								AND member_use.change_ordinal = filler.change_ordinal
								AND member_use.assertion_id = filler.assertion_id
								AND member_use.slot_ordinal = filler.slot_ordinal
								AND member_use.filler_ordinal = filler.filler_ordinal
								AND member_use.use_kind = 'required_member'
						)
					)`
	memberReplacement := `						OR (
							NOT EXISTS (
								SELECT 1 FROM typed_memory_relational_assertion_memberof_uses_v3 member_use
								WHERE member_use.project_id = filler.project_id
									AND member_use.event_ref = filler.event_ref
									AND member_use.change_ordinal = filler.change_ordinal
									AND member_use.assertion_id = filler.assertion_id
									AND member_use.slot_ordinal = filler.slot_ordinal
									AND member_use.filler_ordinal = filler.filler_ordinal
									AND member_use.use_kind = 'required_member'
							)
							AND NOT EXISTS (
								SELECT 1 FROM typed_memory_relational_assertion_classification_uses_v54 classification_use
								WHERE classification_use.project_id = filler.project_id
									AND classification_use.event_ref = filler.event_ref
									AND classification_use.change_ordinal = filler.change_ordinal
									AND classification_use.assertion_id = filler.assertion_id
									AND classification_use.slot_ordinal = filler.slot_ordinal
									AND classification_use.filler_ordinal = filler.filler_ordinal
									AND classification_use.use_kind = 'required_true'
							)
						)
					)`
	if strings.Count(source, memberNeedle) != 1 {
		return "", fmt.Errorf("v54 v3 filler-completeness seam changed")
	}
	source = strings.Replace(source, memberNeedle, memberReplacement, 1)
	marker := `
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_memberof_evaluations evaluation`
	index := strings.Index(source, marker)
	if index < 0 {
		return "", fmt.Errorf("v54 classification-completeness seam changed")
	}
	classificationCompleteness := `
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_kind_classification_source_blobs_v54 source_blob
				WHERE source_blob.project_id = NEW.project_id
					AND source_blob.event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_kind_classification_features_v54 feature
						WHERE feature.project_id = source_blob.project_id
							AND feature.event_ref = source_blob.event_ref
							AND feature.source_kind = 'external_blob'
							AND feature.source_ref = source_blob.source_ref
							AND feature.source_digest = source_blob.source_digest
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_kind_classification_evaluations_v54 evaluation
				WHERE evaluation.project_id = NEW.project_id
					AND evaluation.event_ref = NEW.event_ref
					AND (
						NOT EXISTS (
							SELECT 1 FROM typed_memory_kind_classification_features_v54 feature
							WHERE feature.project_id = evaluation.project_id
								AND feature.event_ref = evaluation.event_ref
								AND feature.evaluation_ref = evaluation.evaluation_ref
						)
						OR NOT EXISTS (
							SELECT 1 FROM typed_memory_relational_assertion_classification_uses_v54 classification_use
							WHERE classification_use.project_id = evaluation.project_id
								AND classification_use.event_ref = evaluation.event_ref
								AND classification_use.evaluation_ref = evaluation.evaluation_ref
						)
					)
			)`
	return source[:index] + classificationCompleteness + source[index:], nil
}

func typedMemoryGraphCommitExactClosureTrigger54() (string, error) {
	source, err := typedMemoryGraphCommitExactClosureTrigger53()
	if err != nil {
		return "", err
	}
	countNeedle := `			AND closure.memberof_use_count = footprint.memberof_use_count
			AND closure.alias_change_count = footprint.alias_change_count`
	countReplacement := `			AND closure.memberof_use_count = footprint.memberof_use_count
			AND closure.kind_classification_source_blob_count = footprint.kind_classification_source_blob_count
			AND closure.kind_classification_evaluation_count = footprint.kind_classification_evaluation_count
			AND closure.kind_classification_feature_count = footprint.kind_classification_feature_count
			AND closure.kind_classification_use_count = footprint.kind_classification_use_count
			AND closure.alias_change_count = footprint.alias_change_count`
	count := strings.Count(source, countNeedle)
	if count == 0 {
		return "", fmt.Errorf("v54 graph-commit count seam changed")
	}
	source = strings.ReplaceAll(source, countNeedle, countReplacement)
	legacy := typedMemoryGraphCommitExactClosureTrigger46()
	prefix := `CREATE TRIGGER typed_memory_graph_commits_exact_closure
	BEFORE INSERT ON typed_memory_graph_commits
	WHEN NOT EXISTS (`
	suffix := "\n\t) BEGIN\n\t\tSELECT RAISE(ABORT"
	if !strings.HasPrefix(legacy, prefix) {
		return "", fmt.Errorf("v54 graph-commit branch prefix changed")
	}
	legacyEnd := strings.LastIndex(legacy, suffix)
	if legacyEnd < 0 {
		return "", fmt.Errorf("v54 graph-commit branch suffix changed")
	}
	writer54 := legacy[len(prefix):legacyEnd]
	writer54 = strings.Replace(
		writer54,
		"generation.writer_generation = 46",
		"generation.writer_generation = 54",
		1,
	)
	writer54 = strings.Replace(
		writer54,
		"generation.provenance_kind = 'writer_v46'",
		"generation.provenance_kind = 'writer_v54'",
		1,
	)
	writer54 = strings.Replace(
		writer54,
		countNeedle,
		countReplacement,
		1,
	)
	basisNeedle := `			AND admission.event_digest = NEW.event_digest
			AND admission.semantic_digest = NEW.change_set_digest`
	basisReplacement := `			AND admission.event_digest = NEW.event_digest
			AND admission.admission_basis_kind = 'context_slice_classification'
			AND admission.semantic_digest = NEW.change_set_digest`
	if strings.Count(writer54, basisNeedle) != 1 {
		return "", fmt.Errorf("v54 graph-commit admission seam changed")
	}
	writer54 = strings.Replace(writer54, basisNeedle, basisReplacement, 1)
	sourceEnd := strings.LastIndex(source, suffix)
	if sourceEnd < 0 {
		return "", fmt.Errorf("v54 graph-commit target suffix changed")
	}
	branch := "\n\t) AND NOT EXISTS (" + writer54
	return source[:sourceEnd] + branch + source[sourceEnd:], nil
}

func typedMemoryEventWriterGenerationExactBoundaryTrigger54() string {
	return `CREATE TRIGGER typed_memory_event_writer_generations_v54_exact_boundary
	BEFORE INSERT ON typed_memory_event_writer_generations
	WHEN NEW.writer_generation = 45
		OR (NEW.writer_generation = 46 AND (
			NOT EXISTS (
				SELECT 1 FROM typed_memory_storage_capabilities capability
				WHERE capability.capability_key = 'typed_memory_writer_generation'
					AND capability.writer_generation = 46
			)
			OR EXISTS (
				SELECT 1 FROM typed_memory_graph_events event
				WHERE event.project_id = NEW.project_id
					AND event.event_ref = NEW.event_ref
					AND event.event_kind = 'assert_relation'
			)
		))
		OR (NEW.writer_generation = 53 AND NOT EXISTS (
			SELECT 1 FROM typed_memory_writer_capabilities_v53 capability
			WHERE capability.capability_key = 'typed_memory_assert_relation_writer_generation'
				AND capability.writer_generation = 53
		))
		OR (NEW.writer_generation = 54 AND (
			NOT EXISTS (
				SELECT 1 FROM typed_memory_writer_capabilities_v54 capability
				WHERE capability.capability_key = 'typed_memory_kind_classification_writer_generation'
					AND capability.writer_generation = 54
			)
			OR NOT EXISTS (
				SELECT 1
				FROM typed_memory_event_admission_bases admission
				WHERE admission.project_id = NEW.project_id
					AND admission.event_ref = NEW.event_ref
					AND admission.admission_basis_kind = 'context_slice_classification'
			)
		))
	BEGIN
		SELECT RAISE(ABORT, 'typed-memory event writer generation crosses its sealed v54 boundary');
	END`
}

func typedMemoryRelationalAssertionExactEventTrigger54() string {
	return `CREATE TRIGGER typed_memory_relational_assertions_v3_v54_exact_event
	BEFORE INSERT ON typed_memory_relational_assertions_v3
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_event_writer_generations generation
			ON generation.project_id = event.project_id
			AND generation.event_ref = event.event_ref
		JOIN typed_memory_event_admission_bases admission
			ON admission.project_id = event.project_id
			AND admission.event_ref = event.event_ref
		JOIN typed_memory_context_slices slice
			ON slice.project_id = event.project_id
			AND slice.event_ref = event.event_ref
			AND slice.context_slice_ref = NEW.context_slice_ref
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND NEW.change_ordinal < event.change_count
			AND event.event_kind IN ('assert_relation', 'mixed_change_set')
			AND (
				(generation.writer_generation = 53
					AND generation.provenance_kind = 'writer_v53'
					AND admission.admission_basis_kind != 'context_slice_classification')
				OR (generation.writer_generation = 54
					AND generation.provenance_kind = 'writer_v54'
					AND admission.admission_basis_kind = 'context_slice_classification')
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_graph_commits commit_record
				WHERE commit_record.project_id = event.project_id
					AND commit_record.event_ref = event.event_ref
			)
	) BEGIN
		SELECT RAISE(ABORT, 'relational assertion lacks its exact open writer event and ContextSlice');
	END`
}

func typedMemoryKindClassificationIndexStatements54() []string {
	return []string{
		"CREATE INDEX idx_typed_memory_kind_classification_source_digest_v54 ON typed_memory_kind_classification_source_blobs_v54(project_id, source_digest)",
		"CREATE INDEX idx_typed_memory_kind_classification_entity_kind_v54 ON typed_memory_kind_classification_evaluations_v54(project_id, entity_id, local_value_kind_ref)",
		"CREATE INDEX idx_typed_memory_kind_classification_feature_source_v54 ON typed_memory_kind_classification_features_v54(project_id, source_ref, source_digest)",
		"CREATE INDEX idx_typed_memory_relational_assertion_classification_use_v54 ON typed_memory_relational_assertion_classification_uses_v54(project_id, assertion_id, slot_ordinal, filler_ordinal)",
	}
}

func typedMemoryKindClassificationTriggerStatements54() []string {
	tables := []string{
		typedMemoryKindClassificationSourceBlobsTable54,
		typedMemoryKindClassificationEvaluationsTable54,
		typedMemoryKindClassificationFeaturesTable54,
		typedMemoryRelationalAssertionClassificationUsesTable54,
	}
	statements := []string{
		typedMemoryKindClassificationWriterCapabilityExactMarkerTrigger54(),
		typedMemoryKindClassificationSourceExactEventTrigger54(),
		typedMemoryKindClassificationEvaluationExactEventTrigger54(),
		typedMemoryKindClassificationFeatureExactSourceTrigger54(),
		typedMemoryKindClassificationUseExactFillerTrigger54(),
		immutableTypedMemoryTrigger54(
			typedMemoryKindClassificationWriterCapabilitiesTable54,
			"update",
		),
		immutableTypedMemoryTrigger54(
			typedMemoryKindClassificationWriterCapabilitiesTable54,
			"delete",
		),
	}
	for _, table := range tables {
		statements = append(statements, typedMemoryOpenEventTrigger54(table))
		statements = append(statements, immutableTypedMemoryTrigger54(table, "update"))
		statements = append(statements, immutableTypedMemoryTrigger54(table, "delete"))
	}
	return statements
}

func typedMemoryKindClassificationWriterCapabilityExactMarkerTrigger54() string {
	return `CREATE TRIGGER typed_memory_writer_capabilities_v54_exact_marker
	BEFORE INSERT ON typed_memory_writer_capabilities_v54
	WHEN EXISTS (SELECT 1 FROM typed_memory_writer_capabilities_v54)
		OR NEW.capability_key != 'typed_memory_kind_classification_writer_generation'
		OR NEW.writer_generation != 54
		OR NEW.capability_digest != 'sha256:1395bb80205b84b5b6a57e1e4a9b71ec559f93b0c50e518847a408a68ffcbe37'
		OR CAST(NEW.canonical_bytes AS TEXT) !=
			'haft.typed-memory.storage.kind-classification-writer-generation=54'
	BEGIN
		SELECT RAISE(ABORT, 'typed-memory v54 writer capability is immutable and exact');
	END`
}

func typedMemoryKindClassificationSourceExactEventTrigger54() string {
	return `CREATE TRIGGER typed_memory_kind_classification_source_blobs_v54_exact_event
	BEFORE INSERT ON typed_memory_kind_classification_source_blobs_v54
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_event_admission_bases admission
			ON admission.project_id = event.project_id
			AND admission.event_ref = event.event_ref
		JOIN typed_memory_event_writer_generations generation
			ON generation.project_id = event.project_id
			AND generation.event_ref = event.event_ref
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND admission.admission_basis_kind = 'context_slice_classification'
			AND generation.writer_generation = 54
			AND generation.provenance_kind = 'writer_v54'
	) BEGIN
		SELECT RAISE(ABORT, 'kind-classification source lacks its exact writer-54 admission event');
	END`
}

func typedMemoryKindClassificationEvaluationExactEventTrigger54() string {
	return `CREATE TRIGGER typed_memory_kind_classification_evaluations_v54_exact_event
	BEFORE INSERT ON typed_memory_kind_classification_evaluations_v54
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_event_admission_bases admission
			ON admission.project_id = event.project_id
			AND admission.event_ref = event.event_ref
		JOIN typed_memory_event_writer_generations generation
			ON generation.project_id = event.project_id
			AND generation.event_ref = event.event_ref
		JOIN typed_memory_context_slices slice
			ON slice.project_id = event.project_id
			AND slice.event_ref = event.event_ref
			AND slice.context_slice_ref = NEW.context_slice_ref
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND admission.admission_basis_kind = 'context_slice_classification'
			AND generation.writer_generation = 54
			AND generation.provenance_kind = 'writer_v54'
	) BEGIN
		SELECT RAISE(ABORT, 'kind-classification evaluation lacks its exact writer-54 admission event');
	END`
}

func typedMemoryKindClassificationFeatureExactSourceTrigger54() string {
	return `CREATE TRIGGER typed_memory_kind_classification_features_v54_exact_source
	BEFORE INSERT ON typed_memory_kind_classification_features_v54
	WHEN (NEW.source_kind = 'internal_visibility'
			AND NEW.source_ref NOT LIKE 'kind-classification-visibility:%')
		OR (NEW.source_kind = 'external_blob'
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_kind_classification_source_blobs_v54 source_blob
				WHERE source_blob.project_id = NEW.project_id
					AND source_blob.event_ref = NEW.event_ref
					AND source_blob.source_ref = NEW.source_ref
					AND source_blob.source_digest = NEW.source_digest
			))
	BEGIN
		SELECT RAISE(ABORT, 'kind-classification feature lacks its exact governed source');
	END`
}

func typedMemoryKindClassificationUseExactFillerTrigger54() string {
	return `CREATE TRIGGER typed_memory_relational_assertion_classification_uses_v54_exact_filler
	BEFORE INSERT ON typed_memory_relational_assertion_classification_uses_v54
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_relational_assertion_fillers_v3 filler
		JOIN typed_memory_relational_assertion_reference_resolution_uses_v3 resolution_use
			ON resolution_use.project_id = filler.project_id
			AND resolution_use.event_ref = filler.event_ref
			AND resolution_use.change_ordinal = filler.change_ordinal
			AND resolution_use.assertion_id = filler.assertion_id
			AND resolution_use.slot_ordinal = filler.slot_ordinal
			AND resolution_use.filler_ordinal = filler.filler_ordinal
			AND resolution_use.filler_digest = filler.filler_digest
		JOIN typed_memory_kind_classification_evaluations_v54 evaluation
			ON evaluation.project_id = filler.project_id
			AND evaluation.event_ref = filler.event_ref
			AND evaluation.evaluation_ref = NEW.evaluation_ref
		WHERE filler.project_id = NEW.project_id
			AND filler.event_ref = NEW.event_ref
			AND filler.change_ordinal = NEW.change_ordinal
			AND filler.assertion_id = NEW.assertion_id
			AND filler.slot_ordinal = NEW.slot_ordinal
			AND filler.filler_ordinal = NEW.filler_ordinal
			AND filler.filler_digest = NEW.filler_digest
			AND filler.filler_kind = 'by_reference'
			AND evaluation.entity_id = filler.entity_id
			AND evaluation.local_value_kind_ref = NEW.queried_value_kind_ref
			AND (
				(NEW.use_kind = 'required_true'
					AND NEW.queried_value_kind_ref = filler.required_value_kind_ref)
				OR NEW.use_kind = 'disjoint_false'
			)
			AND evaluation.request_digest = NEW.request_digest
			AND evaluation.judgement_kind = NEW.expected_judgement_kind
	) BEGIN
		SELECT RAISE(ABORT, 'kind-classification use does not bind its exact resolved relation filler and evaluation');
	END`
}

func typedMemoryOpenEventTrigger54(table string) string {
	return `CREATE TRIGGER ` + table + `_v54_open_event
	BEFORE INSERT ON ` + table + `
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_graph_events event
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_graph_commits commit_record
				WHERE commit_record.project_id = event.project_id
					AND commit_record.event_ref = event.event_ref
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory v54 materialization requires its exact open event');
	END`
}

func immutableTypedMemoryTrigger54(table string, operation string) string {
	return `CREATE TRIGGER ` + table + `_v54_no_` + operation + `
	BEFORE ` + operation + ` ON ` + table + ` BEGIN
		SELECT RAISE(ABORT, 'typed-memory v54 history is append-only');
	END`
}

func verifyTypedMemoryKindClassificationFootprint54(
	tx MigrationTransaction,
) error {
	var generation int
	var digest string
	var canonical []byte
	err := tx.QueryRow(
		`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_writer_capabilities_v54
		WHERE capability_key = ?`,
		typedMemoryKindClassificationCapabilityKey54,
	).Scan(&generation, &digest, &canonical)
	if err != nil {
		return fmt.Errorf("load v54 writer capability: %w", err)
	}
	matches := generation == typedMemoryKindClassificationWriter54
	matches = matches && digest == typedMemoryKindClassificationMarkerDigest54
	matches = matches && bytes.Equal(canonical, []byte(typedMemoryKindClassificationMarkerBytes54))
	if !matches {
		return fmt.Errorf("v54 writer capability marker differs from its sealed bytes")
	}
	for _, table := range []string{
		typedMemoryKindClassificationSourceBlobsTable54,
		typedMemoryKindClassificationEvaluationsTable54,
		typedMemoryKindClassificationFeaturesTable54,
		typedMemoryRelationalAssertionClassificationUsesTable54,
	} {
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			return fmt.Errorf("count migrated v54 table %s: %w", table, err)
		}
		if count != 0 {
			return fmt.Errorf(
				"v54 migration unexpectedly materialized %d rows in %s",
				count,
				table,
			)
		}
	}
	checks := []struct {
		kind string
		name string
	}{
		{kind: "table", name: "typed_memory_event_writer_generations"},
		{kind: "table", name: "typed_memory_event_admission_bases"},
		{kind: "table", name: "typed_memory_commit_materialization_closures"},
		{kind: "view", name: "typed_memory_event_materialization_footprints_v46"},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v54_exact_boundary"},
		{kind: "trigger", name: "typed_memory_relational_assertions_v3_v54_exact_event"},
		{kind: "trigger", name: "typed_memory_graph_commits_exact_closure"},
	}
	for _, check := range checks {
		var sqlText string
		err := tx.QueryRow(
			"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
			check.kind,
			check.name,
		).Scan(&sqlText)
		if err != nil || strings.TrimSpace(sqlText) == "" {
			return fmt.Errorf(
				"load v54 %s %s: %w",
				check.kind,
				check.name,
				err,
			)
		}
	}
	return nil
}
