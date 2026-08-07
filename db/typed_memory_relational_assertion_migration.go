package db

import (
	"bytes"
	"fmt"
	"strings"
)

const typedMemoryRelationalAssertionSchemaVersion53 = 53

const (
	typedMemoryRelationalAssertionsTable53            = "typed_memory_relational_assertions_v3"
	typedMemoryRelationalAssertionSlotsTable53        = "typed_memory_relational_assertion_slots_v3"
	typedMemoryRelationalAssertionFillersTable53      = "typed_memory_relational_assertion_fillers_v3"
	typedMemoryRelationalAssertionResolutionsTable53  = "typed_memory_relational_assertion_reference_resolution_uses_v3"
	typedMemoryRelationalAssertionMemberOfUsesTable53 = "typed_memory_relational_assertion_memberof_uses_v3"
	typedMemoryRelationalAssertionDisjointUsesTable53 = "typed_memory_relational_assertion_disjointness_uses_v3"
	typedMemoryWriterCapabilitiesTable53              = "typed_memory_writer_capabilities_v53"
)

const (
	typedMemoryWriterGenerationCapability53 = "typed_memory_assert_relation_writer_generation"
	typedMemoryWriterGeneration53           = 53
	typedMemoryWriterMarkerBytes53          = "haft.typed-memory.storage.assert-relation-writer-generation=53"
	typedMemoryWriterMarkerDigest53         = "sha256:a2445bb17e50f89d3c943fd37c7b50203ce38a7d3d303b1cabab45ee57d12a0d"
)

var typedMemoryRelationalAssertionMigration53 = Migration{
	Version:            typedMemoryRelationalAssertionSchemaVersion53,
	Description:        "Add the explicit-modality relational-assertion storage lane",
	Apply:              applyTypedMemoryRelationalAssertionMigration53,
	ApplyBoundary:      ForeignKeyTableRebuildBoundary,
	ForeignKeyVerifier: verifyForeignKeys,
}

type typedMemoryRelationalAssertionObject53 struct {
	kind string
	name string
	sql  string
}

func applyTypedMemoryRelationalAssertionMigration53(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireTypedMemoryRelationalAssertionSource53(tx); err != nil {
		return err
	}
	if err := requireAbsentTypedMemoryRelationalAssertionFootprint53(tx); err != nil {
		return err
	}
	preservedDDL, err := loadTypedMemoryOwnedDDL53(tx)
	if err != nil {
		return err
	}
	statements, err := typedMemoryRelationalAssertionStatements53(preservedDDL)
	if err != nil {
		return err
	}
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install relational-assertion storage generation 53: %w", err)
	}
	if err := verifyTypedMemoryRelationalAssertionFootprint53(tx); err != nil {
		return err
	}
	if err := verifyHistoricalTypedMemoryFootprints49(tx); err != nil {
		return fmt.Errorf("verify historical typed-memory footprints after v53: %w", err)
	}
	return verifyForeignKeys(tx)
}

func loadTypedMemoryOwnedDDL53(
	tx MigrationTransaction,
) (map[string]string, error) {
	coordinates := []struct {
		kind string
		name string
	}{
		{kind: "index", name: "idx_typed_memory_events_project_revision"},
		{kind: "trigger", name: "typed_memory_graph_events_exact_head"},
		{kind: "trigger", name: "typed_memory_graph_events_no_update"},
		{kind: "trigger", name: "typed_memory_graph_events_no_delete"},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v46_open_event"},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v46_no_update"},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v46_no_delete"},
	}
	result := make(map[string]string, len(coordinates))
	for _, coordinate := range coordinates {
		var sqlText string
		err := tx.QueryRow(
			"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
			coordinate.kind,
			coordinate.name,
		).Scan(&sqlText)
		if err != nil {
			return nil, fmt.Errorf(
				"load byte-preserved v53 %s %s: %w",
				coordinate.kind,
				coordinate.name,
				err,
			)
		}
		result[coordinate.name] = sqlText
	}
	return result, nil
}

func requireTypedMemoryRelationalAssertionSource53(
	tx MigrationTransaction,
) error {
	var maximumVersion int
	err := tx.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&maximumVersion)
	if err != nil {
		return fmt.Errorf("inspect relational-assertion source schema: %w", err)
	}
	if maximumVersion != typedMemoryIdentityReconciliationSchemaVersion52 {
		return fmt.Errorf(
			"relational-assertion storage requires exact schema version 52, found %d",
			maximumVersion,
		)
	}
	if err := requireTypedMemoryWriterCapability46(tx); err != nil {
		return err
	}
	objects, err := typedMemoryRelationalAssertionSourceObjects53()
	if err != nil {
		return err
	}
	for _, object := range objects {
		if err := requireExactTypedMemoryObject53(tx, object, "v52 source"); err != nil {
			return err
		}
	}
	return verifyForeignKeys(tx)
}

func typedMemoryRelationalAssertionSourceObjects53() (
	[]typedMemoryRelationalAssertionObject53,
	error,
) {
	graphEvents, err := typedMemoryGraphEventsTable47("typed_memory_graph_events")
	if err != nil {
		return nil, fmt.Errorf("derive exact v52 graph-event table: %w", err)
	}
	footprint, err := typedMemoryEventMaterializationFootprintsView49()
	if err != nil {
		return nil, fmt.Errorf("derive exact v52 materialization view: %w", err)
	}
	closure, err := typedMemoryCommitClosureExactFootprintTrigger47()
	if err != nil {
		return nil, fmt.Errorf("derive exact v52 materialization closure: %w", err)
	}
	commit, err := typedMemoryGraphCommitExactClosureTrigger52()
	if err != nil {
		return nil, fmt.Errorf("derive exact v52 graph-commit closure: %w", err)
	}
	return []typedMemoryRelationalAssertionObject53{
		{kind: "table", name: "typed_memory_graph_events", sql: graphEvents},
		{kind: "table", name: "typed_memory_event_writer_generations", sql: typedMemoryEventWriterGenerationsTable46()},
		{kind: "table", name: "typed_memory_relation_instances", sql: typedMemoryRelationInstancesTable46()},
		{kind: "index", name: "idx_typed_memory_events_project_revision", sql: "CREATE INDEX idx_typed_memory_events_project_revision ON typed_memory_graph_events(project_id, graph_revision DESC)"},
		{kind: "trigger", name: "typed_memory_graph_events_exact_head", sql: typedMemoryGraphEventExactHeadTrigger45()},
		{kind: "trigger", name: "typed_memory_graph_events_no_update", sql: immutableTypedMemoryTrigger45("typed_memory_graph_events", "update")},
		{kind: "trigger", name: "typed_memory_graph_events_no_delete", sql: immutableTypedMemoryTrigger45("typed_memory_graph_events", "delete")},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v46_exact_boundary", sql: typedMemoryEventWriterGenerationExactBoundaryTrigger46()},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v46_open_event", sql: typedMemoryOpenEventTrigger46("typed_memory_event_writer_generations")},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v46_no_update", sql: immutableTypedMemoryTrigger46("typed_memory_event_writer_generations", "update")},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v46_no_delete", sql: immutableTypedMemoryTrigger46("typed_memory_event_writer_generations", "delete")},
		{kind: "view", name: "typed_memory_event_materialization_footprints_v46", sql: footprint},
		{kind: "trigger", name: "typed_memory_commit_materialization_closures_v46_exact_footprint", sql: closure},
		{kind: "trigger", name: "typed_memory_graph_commits_exact_closure", sql: commit},
	}, nil
}

func requireAbsentTypedMemoryRelationalAssertionFootprint53(
	tx MigrationTransaction,
) error {
	objects, err := typedMemoryRelationalAssertionTargetObjects53()
	if err != nil {
		return err
	}
	for _, object := range objects {
		if typedMemoryRelationalAssertionReplacesExistingObject53(object.name) {
			continue
		}
		var count int
		err := tx.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
			object.kind,
			object.name,
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("inspect v53 %s %s: %w", object.kind, object.name, err)
		}
		if count != 0 {
			return fmt.Errorf(
				"relational-assertion storage refused unknown partial v53 footprint: %s %s already exists",
				object.kind,
				object.name,
			)
		}
	}
	return nil
}

func typedMemoryRelationalAssertionReplacesExistingObject53(name string) bool {
	switch name {
	case "typed_memory_graph_events",
		"typed_memory_event_writer_generations",
		"idx_typed_memory_events_project_revision",
		"typed_memory_graph_events_exact_head",
		"typed_memory_graph_events_no_update",
		"typed_memory_graph_events_no_delete",
		"typed_memory_event_writer_generations_v46_open_event",
		"typed_memory_event_writer_generations_v46_no_update",
		"typed_memory_event_writer_generations_v46_no_delete",
		"typed_memory_event_materialization_footprints_v46",
		"typed_memory_commit_materialization_closures_v46_exact_footprint",
		"typed_memory_graph_commits_exact_closure":
		return true
	default:
		return false
	}
}

func requireExactTypedMemoryObject53(
	tx MigrationTransaction,
	object typedMemoryRelationalAssertionObject53,
	label string,
) error {
	var actual string
	err := tx.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		object.kind,
		object.name,
	).Scan(&actual)
	if err != nil {
		return fmt.Errorf("load %s %s %s: %w", label, object.kind, object.name, err)
	}
	if normalizeSQLiteDDL46(actual) != normalizeSQLiteDDL46(object.sql) {
		return fmt.Errorf("%s %s %s differs from its exact schema", label, object.kind, object.name)
	}
	return nil
}

func typedMemoryRelationalAssertionStatements53(
	preservedDDL map[string]string,
) ([]string, error) {
	graphEvents, err := typedMemoryGraphEventsTable53()
	if err != nil {
		return nil, err
	}
	footprint, err := typedMemoryEventMaterializationFootprintsView53()
	if err != nil {
		return nil, err
	}
	closure, err := typedMemoryCommitClosureExactFootprintTrigger53()
	if err != nil {
		return nil, err
	}
	commit, err := typedMemoryGraphCommitExactClosureTrigger53()
	if err != nil {
		return nil, err
	}
	statements := []string{
		`CREATE TEMP TABLE typed_memory_graph_events_v53_backup AS
		SELECT * FROM typed_memory_graph_events`,
		`CREATE TEMP TABLE typed_memory_event_writer_generations_v53_backup AS
		SELECT * FROM typed_memory_event_writer_generations`,
		"DROP TRIGGER typed_memory_graph_commits_exact_closure",
		"DROP TRIGGER typed_memory_commit_materialization_closures_v46_exact_footprint",
		"DROP VIEW typed_memory_event_materialization_footprints_v46",
		"DROP TRIGGER typed_memory_event_writer_generations_v46_exact_boundary",
		"DROP TRIGGER typed_memory_event_writer_generations_v46_open_event",
		"DROP TRIGGER typed_memory_event_writer_generations_v46_no_update",
		"DROP TRIGGER typed_memory_event_writer_generations_v46_no_delete",
		"DROP TRIGGER typed_memory_graph_events_exact_head",
		"DROP TRIGGER typed_memory_graph_events_no_update",
		"DROP TRIGGER typed_memory_graph_events_no_delete",
		"DROP INDEX idx_typed_memory_events_project_revision",
		"DROP TABLE typed_memory_event_writer_generations",
		"DROP TABLE typed_memory_graph_events",
		graphEvents,
		typedMemoryEventWriterGenerationsTable53(),
		`INSERT INTO typed_memory_graph_events (
			project_id, event_ref, commit_ref, event_digest,
			expected_revision, graph_revision,
			basis_type_env_ref, result_type_env_ref,
			change_set_digest, canonical_change_set_bytes,
			change_count, event_kind, authority_class,
			request_provenance_ref, recorded_at
		)
		SELECT
			project_id, event_ref, commit_ref, event_digest,
			expected_revision, graph_revision,
			basis_type_env_ref, result_type_env_ref,
			change_set_digest, canonical_change_set_bytes,
			change_count, event_kind, authority_class,
			request_provenance_ref, recorded_at
		FROM typed_memory_graph_events_v53_backup`,
		`INSERT INTO typed_memory_event_writer_generations (
			project_id, event_ref, writer_generation, provenance_kind
		)
		SELECT project_id, event_ref, writer_generation, provenance_kind
		FROM typed_memory_event_writer_generations_v53_backup`,
		"DROP TABLE typed_memory_graph_events_v53_backup",
		"DROP TABLE typed_memory_event_writer_generations_v53_backup",
		preservedDDL["idx_typed_memory_events_project_revision"],
		preservedDDL["typed_memory_graph_events_exact_head"],
		preservedDDL["typed_memory_graph_events_no_update"],
		preservedDDL["typed_memory_graph_events_no_delete"],
		typedMemoryEventWriterGenerationExactBoundaryTrigger53(),
		preservedDDL["typed_memory_event_writer_generations_v46_open_event"],
		preservedDDL["typed_memory_event_writer_generations_v46_no_update"],
		preservedDDL["typed_memory_event_writer_generations_v46_no_delete"],
		typedMemoryWriterCapabilitiesTableStatement53(),
		typedMemoryWriterCapabilityMarkerInsert53(),
		typedMemoryWriterCapabilityExactMarkerTrigger53(),
		immutableTypedMemoryTrigger53(typedMemoryWriterCapabilitiesTable53, "update"),
		immutableTypedMemoryTrigger53(typedMemoryWriterCapabilitiesTable53, "delete"),
	}
	statements = append(statements, typedMemoryRelationalAssertionTableStatements53()...)
	statements = append(statements, typedMemoryRelationalAssertionIndexStatements53()...)
	statements = append(statements, typedMemoryRelationalAssertionTriggerStatements53()...)
	statements = append(
		statements,
		typedMemoryLegacyRelationInsertFreezeTrigger53(),
		footprint,
		closure,
		commit,
	)
	return statements, nil
}

func typedMemoryGraphEventsTable53() (string, error) {
	source, err := typedMemoryGraphEventsTable47("typed_memory_graph_events")
	if err != nil {
		return "", fmt.Errorf("derive v53 graph-event source: %w", err)
	}
	needle := "'merge_entities', 'split_entity', 'instantiate_relation',\n\t\t\t'retract_assertion', 'mixed_change_set', 'activate_type_env'"
	replacement := "'merge_entities', 'split_entity', 'instantiate_relation',\n\t\t\t'assert_relation', 'retract_assertion', 'mixed_change_set', 'activate_type_env'"
	return replaceExactSQL47(
		source,
		needle,
		replacement,
		1,
		"v53 graph-event kind union",
	)
}

func typedMemoryEventWriterGenerationsTable53() string {
	return `CREATE TABLE typed_memory_event_writer_generations (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		writer_generation INTEGER NOT NULL CHECK(writer_generation IN (45, 46, 53)),
		provenance_kind TEXT NOT NULL CHECK(
			(writer_generation = 45 AND provenance_kind = 'migration_v45_backfill')
			OR (writer_generation = 46 AND provenance_kind = 'writer_v46')
			OR (writer_generation = 53 AND provenance_kind = 'writer_v53')
		),
		PRIMARY KEY(project_id, event_ref),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryWriterCapabilitiesTableStatement53() string {
	return `CREATE TABLE typed_memory_writer_capabilities_v53 (
		capability_key TEXT PRIMARY KEY CHECK(
			capability_key = 'typed_memory_assert_relation_writer_generation'
		),
		writer_generation INTEGER NOT NULL CHECK(writer_generation = 53),
		capability_digest TEXT NOT NULL UNIQUE CHECK(
			capability_digest = 'sha256:a2445bb17e50f89d3c943fd37c7b50203ce38a7d3d303b1cabab45ee57d12a0d'
		),
		canonical_bytes BLOB NOT NULL UNIQUE CHECK(
			CAST(canonical_bytes AS TEXT) =
				'haft.typed-memory.storage.assert-relation-writer-generation=53'
		),
		installed_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("installed_at") + `)
	) WITHOUT ROWID`
}

func typedMemoryWriterCapabilityMarkerInsert53() string {
	return `INSERT INTO typed_memory_writer_capabilities_v53 (
		capability_key, writer_generation, capability_digest,
		canonical_bytes, installed_at
	) VALUES (
		'typed_memory_assert_relation_writer_generation',
		53,
		'sha256:a2445bb17e50f89d3c943fd37c7b50203ce38a7d3d303b1cabab45ee57d12a0d',
		CAST('haft.typed-memory.storage.assert-relation-writer-generation=53' AS BLOB),
		strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	)`
}

func typedMemoryWriterCapabilityExactMarkerTrigger53() string {
	return `CREATE TRIGGER typed_memory_writer_capabilities_v53_exact_marker
	BEFORE INSERT ON typed_memory_writer_capabilities_v53
	WHEN EXISTS (SELECT 1 FROM typed_memory_writer_capabilities_v53)
		OR NEW.capability_key != 'typed_memory_assert_relation_writer_generation'
		OR NEW.writer_generation != 53
		OR NEW.capability_digest != 'sha256:a2445bb17e50f89d3c943fd37c7b50203ce38a7d3d303b1cabab45ee57d12a0d'
		OR CAST(NEW.canonical_bytes AS TEXT) !=
			'haft.typed-memory.storage.assert-relation-writer-generation=53'
	BEGIN
		SELECT RAISE(ABORT, 'typed-memory v53 writer capability is immutable and exact');
	END`
}

func typedMemoryEventWriterGenerationExactBoundaryTrigger53() string {
	return `CREATE TRIGGER typed_memory_event_writer_generations_v53_exact_boundary
	BEFORE INSERT ON typed_memory_event_writer_generations
	WHEN NEW.writer_generation = 45
		OR (NEW.writer_generation = 53 AND NOT EXISTS (
			SELECT 1 FROM typed_memory_writer_capabilities_v53 capability
			WHERE capability.capability_key =
				'typed_memory_assert_relation_writer_generation'
				AND capability.writer_generation = 53
		))
		OR (NEW.writer_generation = 46 AND EXISTS (
			SELECT 1 FROM typed_memory_graph_events event
			WHERE event.project_id = NEW.project_id
				AND event.event_ref = NEW.event_ref
				AND event.event_kind = 'assert_relation'
		))
	BEGIN
		SELECT RAISE(ABORT, 'typed-memory event writer generation crosses its sealed v53 boundary');
	END`
}

func typedMemoryRelationalAssertionTableStatements53() []string {
	return []string{
		typedMemoryRelationalAssertionsTableStatement53(),
		typedMemoryRelationalAssertionSlotsTableStatement53(),
		typedMemoryRelationalAssertionFillersTableStatement53(),
		typedMemoryRelationalAssertionResolutionsTableStatement53(),
		typedMemoryRelationalAssertionMemberOfUsesTableStatement53(),
		typedMemoryRelationalAssertionDisjointUsesTableStatement53(),
	}
}

func typedMemoryRelationalAssertionsTableStatement53() string {
	return `CREATE TABLE typed_memory_relational_assertions_v3 (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		assertion_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("assertion_id") + `),
		signature_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("signature_ref") + `),
		context_slice_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("context_slice_ref") + `),
		modality TEXT NOT NULL CHECK(modality IN (
			'affirms_obtaining', 'denies_obtaining', 'obtaining_unknown'
		)),
		assertion_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("assertion_digest") + `),
		canonical_assertion_bytes BLOB NOT NULL CHECK(length(canonical_assertion_bytes) > 0),
		provenance_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("provenance_ref") + `),
		PRIMARY KEY(project_id, event_ref, change_ordinal),
		UNIQUE(project_id, event_ref, change_ordinal, assertion_id),
		UNIQUE(project_id, assertion_id),
		UNIQUE(project_id, assertion_digest),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(project_id, event_ref, context_slice_ref)
			REFERENCES typed_memory_context_slices(project_id, event_ref, context_slice_ref)
	) WITHOUT ROWID`
}

func typedMemoryRelationalAssertionSlotsTableStatement53() string {
	source := typedMemoryRelationSlotsTable46()
	replacer := strings.NewReplacer(
		"typed_memory_relation_slots", typedMemoryRelationalAssertionSlotsTable53,
		"typed_memory_relation_instances", typedMemoryRelationalAssertionsTable53,
	)
	return replacer.Replace(source)
}

func typedMemoryRelationalAssertionFillersTableStatement53() string {
	source := typedMemoryRelationFillersTable46()
	replacer := strings.NewReplacer(
		"typed_memory_relation_fillers", typedMemoryRelationalAssertionFillersTable53,
		"typed_memory_relation_slots", typedMemoryRelationalAssertionSlotsTable53,
	)
	return replacer.Replace(source)
}

func typedMemoryRelationalAssertionResolutionsTableStatement53() string {
	source := typedMemoryReferenceResolutionUsesTable46()
	replacer := strings.NewReplacer(
		"typed_memory_reference_resolution_uses", typedMemoryRelationalAssertionResolutionsTable53,
		"typed_memory_relation_fillers", typedMemoryRelationalAssertionFillersTable53,
	)
	return replacer.Replace(source)
}

func typedMemoryRelationalAssertionMemberOfUsesTableStatement53() string {
	source := typedMemoryRelationFillerMemberOfUsesTable46()
	replacer := strings.NewReplacer(
		"typed_memory_relation_filler_memberof_uses", typedMemoryRelationalAssertionMemberOfUsesTable53,
		"typed_memory_relation_fillers", typedMemoryRelationalAssertionFillersTable53,
	)
	return replacer.Replace(source)
}

func typedMemoryRelationalAssertionDisjointUsesTableStatement53() string {
	source := typedMemoryDisjointEntailmentUsesTableStatement49()
	replacer := strings.NewReplacer(
		"typed_memory_relation_filler_disjoint_entailment_uses", typedMemoryRelationalAssertionDisjointUsesTable53,
		"typed_memory_relation_fillers", typedMemoryRelationalAssertionFillersTable53,
	)
	return replacer.Replace(source)
}

func typedMemoryRelationalAssertionIndexStatements53() []string {
	return []string{
		"CREATE INDEX idx_typed_memory_relational_assertions_signature_v53 ON typed_memory_relational_assertions_v3(project_id, signature_ref, event_ref)",
		"CREATE INDEX idx_typed_memory_relational_assertions_modality_v53 ON typed_memory_relational_assertions_v3(project_id, modality, event_ref)",
		"CREATE INDEX idx_typed_memory_relational_assertion_fillers_entity_v53 ON typed_memory_relational_assertion_fillers_v3(project_id, entity_id) WHERE filler_kind = 'by_reference'",
	}
}

func typedMemoryRelationalAssertionTriggerStatements53() []string {
	tables := []string{
		typedMemoryRelationalAssertionsTable53,
		typedMemoryRelationalAssertionSlotsTable53,
		typedMemoryRelationalAssertionFillersTable53,
		typedMemoryRelationalAssertionResolutionsTable53,
		typedMemoryRelationalAssertionMemberOfUsesTable53,
		typedMemoryRelationalAssertionDisjointUsesTable53,
	}
	statements := []string{
		typedMemoryRelationalAssertionExactEventTrigger53(),
		typedMemoryRelationalAssertionCrossLaneGuard53(),
		typedMemoryRelationalAssertionSlotExactParentTrigger53(),
		typedMemoryRelationalAssertionFillerExactSlotTrigger53(),
		typedMemoryRelationalAssertionResolutionExactFillerTrigger53(),
		typedMemoryRelationalAssertionMemberOfExactUseTrigger53(),
		typedMemoryRelationalAssertionDisjointExactUseTrigger53(),
	}
	for _, table := range tables {
		statements = append(statements, typedMemoryOpenEventTrigger53(table))
		statements = append(statements, immutableTypedMemoryTrigger53(table, "update"))
		statements = append(statements, immutableTypedMemoryTrigger53(table, "delete"))
	}
	return statements
}

func typedMemoryRelationalAssertionExactEventTrigger53() string {
	return `CREATE TRIGGER typed_memory_relational_assertions_v3_v53_exact_event
	BEFORE INSERT ON typed_memory_relational_assertions_v3
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_event_writer_generations generation
			ON generation.project_id = event.project_id
			AND generation.event_ref = event.event_ref
		JOIN typed_memory_context_slices slice
			ON slice.project_id = event.project_id
			AND slice.event_ref = event.event_ref
			AND slice.context_slice_ref = NEW.context_slice_ref
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND NEW.change_ordinal < event.change_count
			AND event.event_kind IN ('assert_relation', 'mixed_change_set')
			AND generation.writer_generation = 53
			AND generation.provenance_kind = 'writer_v53'
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_graph_commits commit_record
				WHERE commit_record.project_id = event.project_id
					AND commit_record.event_ref = event.event_ref
			)
	) BEGIN
		SELECT RAISE(ABORT, 'relational assertion lacks its exact open writer-53 event and ContextSlice');
	END`
}

func typedMemoryRelationalAssertionCrossLaneGuard53() string {
	return `CREATE TRIGGER typed_memory_relational_assertions_v3_v53_no_legacy_duplicate
	BEFORE INSERT ON typed_memory_relational_assertions_v3
	WHEN EXISTS (
		SELECT 1 FROM typed_memory_relation_instances legacy
		WHERE legacy.project_id = NEW.project_id
			AND legacy.assertion_id = NEW.assertion_id
	) BEGIN
		SELECT RAISE(ABORT, 'assertion identity already exists in the legacy relation lane');
	END`
}

func typedMemoryLegacyRelationInsertFreezeTrigger53() string {
	return `CREATE TRIGGER typed_memory_relation_instances_v53_legacy_insert_frozen
	BEFORE INSERT ON typed_memory_relation_instances
	BEGIN
		SELECT RAISE(ABORT, 'legacy relation-instance storage is replay-only after schema v53');
	END`
}

func typedMemoryRelationalAssertionSlotExactParentTrigger53() string {
	return deriveTypedMemoryRelationalAssertionTrigger53(
		typedMemoryRelationSlotExactRelationTrigger46(),
		"typed_memory_relation_slots_v46_exact_relation",
		"typed_memory_relational_assertion_slots_v3_v53_exact_parent",
	)
}

func typedMemoryRelationalAssertionFillerExactSlotTrigger53() string {
	return deriveTypedMemoryRelationalAssertionTrigger53(
		typedMemoryRelationFillerExactSlotTrigger46(),
		"typed_memory_relation_fillers_v46_exact_slot",
		"typed_memory_relational_assertion_fillers_v3_v53_exact_slot",
	)
}

func typedMemoryRelationalAssertionResolutionExactFillerTrigger53() string {
	return deriveTypedMemoryRelationalAssertionTrigger53(
		typedMemoryReferenceResolutionExactFillerTrigger46(),
		"typed_memory_reference_resolution_uses_v46_exact_filler",
		"typed_memory_relational_assertion_reference_uses_v3_v53_exact_filler",
	)
}

func typedMemoryRelationalAssertionMemberOfExactUseTrigger53() string {
	return deriveTypedMemoryRelationalAssertionTrigger53(
		typedMemoryRelationFillerMemberOfExactUseTrigger46(),
		"typed_memory_relation_filler_memberof_uses_v46_exact_use",
		"typed_memory_relational_assertion_memberof_uses_v3_v53_exact_use",
	)
}

func typedMemoryRelationalAssertionDisjointExactUseTrigger53() string {
	return deriveTypedMemoryRelationalAssertionTrigger53(
		typedMemoryDisjointEntailmentExactUseTrigger49(),
		"typed_memory_relation_filler_disjoint_entailment_uses_v49_exact_use",
		"typed_memory_relational_assertion_disjointness_uses_v3_v53_exact_use",
	)
}

func deriveTypedMemoryRelationalAssertionTrigger53(
	source string,
	oldName string,
	newName string,
) string {
	replacer := strings.NewReplacer(
		oldName, newName,
		"typed_memory_relation_filler_disjoint_entailment_uses", typedMemoryRelationalAssertionDisjointUsesTable53,
		"typed_memory_relation_filler_memberof_uses", typedMemoryRelationalAssertionMemberOfUsesTable53,
		"typed_memory_reference_resolution_uses", typedMemoryRelationalAssertionResolutionsTable53,
		"typed_memory_relation_fillers", typedMemoryRelationalAssertionFillersTable53,
		"typed_memory_relation_slots", typedMemoryRelationalAssertionSlotsTable53,
		"typed_memory_relation_instances", typedMemoryRelationalAssertionsTable53,
	)
	return replacer.Replace(source)
}

func typedMemoryOpenEventTrigger53(table string) string {
	return `CREATE TRIGGER ` + table + `_v53_open_event
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
		SELECT RAISE(ABORT, 'typed-memory v53 materialization requires its exact open event');
	END`
}

func immutableTypedMemoryTrigger53(table string, operation string) string {
	return `CREATE TRIGGER ` + table + `_v53_no_` + operation + `
	BEFORE ` + operation + ` ON ` + table + ` BEGIN
		SELECT RAISE(ABORT, 'typed-memory v53 history is append-only');
	END`
}

func typedMemoryEventMaterializationFootprintsView53() (string, error) {
	source, err := typedMemoryEventMaterializationFootprintsView49()
	if err != nil {
		return "", fmt.Errorf("derive v53 materialization footprint source: %w", err)
	}
	replacements := []struct {
		old string
		new string
	}{
		{
			old: `(SELECT COUNT(*) FROM typed_memory_relation_instances relation
			WHERE relation.project_id = event.project_id
				AND relation.event_ref = event.event_ref) AS relation_count,`,
			new: `(SELECT COUNT(*) FROM typed_memory_relation_instances relation
			WHERE relation.project_id = event.project_id
				AND relation.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relational_assertions_v3 assertion
			WHERE assertion.project_id = event.project_id
				AND assertion.event_ref = event.event_ref) AS relation_count,`,
		},
		{
			old: `(SELECT COUNT(*) FROM typed_memory_relation_slots slot
			WHERE slot.project_id = event.project_id
				AND slot.event_ref = event.event_ref) AS relation_slot_count,`,
			new: `(SELECT COUNT(*) FROM typed_memory_relation_slots slot
			WHERE slot.project_id = event.project_id
				AND slot.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relational_assertion_slots_v3 slot
			WHERE slot.project_id = event.project_id
				AND slot.event_ref = event.event_ref) AS relation_slot_count,`,
		},
		{
			old: `(SELECT COUNT(*) FROM typed_memory_relation_fillers filler
			WHERE filler.project_id = event.project_id
				AND filler.event_ref = event.event_ref) AS relation_filler_count,`,
			new: `(SELECT COUNT(*) FROM typed_memory_relation_fillers filler
			WHERE filler.project_id = event.project_id
				AND filler.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relational_assertion_fillers_v3 filler
			WHERE filler.project_id = event.project_id
				AND filler.event_ref = event.event_ref) AS relation_filler_count,`,
		},
		{
			old: `(SELECT COUNT(*) FROM typed_memory_reference_resolution_uses resolution_use
			WHERE resolution_use.project_id = event.project_id
				AND resolution_use.event_ref = event.event_ref) AS reference_resolution_use_count,`,
			new: `(SELECT COUNT(*) FROM typed_memory_reference_resolution_uses resolution_use
			WHERE resolution_use.project_id = event.project_id
				AND resolution_use.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relational_assertion_reference_resolution_uses_v3 resolution_use
			WHERE resolution_use.project_id = event.project_id
				AND resolution_use.event_ref = event.event_ref) AS reference_resolution_use_count,`,
		},
		{
			old: `(SELECT COUNT(*) FROM typed_memory_relation_filler_memberof_uses member_use
			WHERE member_use.project_id = event.project_id
				AND member_use.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relation_filler_disjoint_entailment_uses entailment_use
			WHERE entailment_use.project_id = event.project_id
				AND entailment_use.event_ref = event.event_ref) AS memberof_use_count,`,
			new: `(SELECT COUNT(*) FROM typed_memory_relation_filler_memberof_uses member_use
			WHERE member_use.project_id = event.project_id
				AND member_use.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relation_filler_disjoint_entailment_uses entailment_use
			WHERE entailment_use.project_id = event.project_id
				AND entailment_use.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relational_assertion_memberof_uses_v3 member_use
			WHERE member_use.project_id = event.project_id
				AND member_use.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relational_assertion_disjointness_uses_v3 disjoint_use
			WHERE disjoint_use.project_id = event.project_id
				AND disjoint_use.event_ref = event.event_ref) AS memberof_use_count,`,
		},
		{
			old: `+ (SELECT COUNT(*) FROM typed_memory_relation_instances relation
			WHERE relation.project_id = event.project_id
				AND relation.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_assertion_retractions retraction`,
			new: `+ (SELECT COUNT(*) FROM typed_memory_relation_instances relation
			WHERE relation.project_id = event.project_id
				AND relation.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relational_assertions_v3 assertion
			WHERE assertion.project_id = event.project_id
				AND assertion.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_assertion_retractions retraction`,
		},
	}
	for _, replacement := range replacements {
		if strings.Count(source, replacement.old) != 1 {
			return "", fmt.Errorf("v53 footprint cannot locate exact source seam")
		}
		source = strings.Replace(source, replacement.old, replacement.new, 1)
	}
	return source, nil
}

func typedMemoryCommitClosureExactFootprintTrigger53() (string, error) {
	source, err := typedMemoryCommitClosureExactFootprintTrigger47()
	if err != nil {
		return "", fmt.Errorf("derive v53 closure source: %w", err)
	}
	legacyPrefixCompleteness := `
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_ordered_candidate_prefixes prefix
				WHERE prefix.project_id = NEW.project_id
					AND prefix.event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_reference_resolution_uses resolution_use
						WHERE resolution_use.project_id = prefix.project_id
							AND resolution_use.event_ref = prefix.event_ref
							AND resolution_use.change_ordinal = prefix.prefix_end_ordinal
							AND resolution_use.ordered_candidate_prefix_digest = prefix.prefix_digest
					)
			)`
	v53PrefixCompleteness := `
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_ordered_candidate_prefixes prefix
				WHERE prefix.project_id = NEW.project_id
					AND prefix.event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_reference_resolution_uses resolution_use
						WHERE resolution_use.project_id = prefix.project_id
							AND resolution_use.event_ref = prefix.event_ref
							AND resolution_use.change_ordinal = prefix.prefix_end_ordinal
							AND resolution_use.ordered_candidate_prefix_digest = prefix.prefix_digest
					)
					AND NOT EXISTS (
						SELECT 1
						FROM typed_memory_relational_assertion_reference_resolution_uses_v3 resolution_use
						JOIN typed_memory_relational_assertion_fillers_v3 filler
							ON filler.project_id = resolution_use.project_id
							AND filler.event_ref = resolution_use.event_ref
							AND filler.change_ordinal = resolution_use.change_ordinal
							AND filler.assertion_id = resolution_use.assertion_id
							AND filler.slot_ordinal = resolution_use.slot_ordinal
							AND filler.filler_ordinal = resolution_use.filler_ordinal
							AND filler.filler_digest = resolution_use.filler_digest
						WHERE resolution_use.project_id = prefix.project_id
							AND resolution_use.event_ref = prefix.event_ref
							AND resolution_use.change_ordinal = prefix.prefix_end_ordinal
							AND resolution_use.ordered_candidate_prefix_digest = prefix.prefix_digest
							AND resolution_use.resolution_kind = 'same_batch_declaration'
							AND filler.filler_kind = 'by_reference'
					)
			)`
	if strings.Count(source, legacyPrefixCompleteness) != 1 {
		return "", fmt.Errorf("v53 closure cannot locate exact prefix-completeness seam")
	}
	source = strings.Replace(source, legacyPrefixCompleteness, v53PrefixCompleteness, 1)
	marker := `
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_memberof_evaluations evaluation`
	index := strings.Index(source, marker)
	if index < 0 {
		return "", fmt.Errorf("v53 closure cannot locate exact completeness seam")
	}
	v3Completeness := `
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_relational_assertions_v3 assertion
				WHERE assertion.project_id = NEW.project_id
					AND assertion.event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_relational_assertion_slots_v3 slot
						WHERE slot.project_id = assertion.project_id
							AND slot.event_ref = assertion.event_ref
							AND slot.change_ordinal = assertion.change_ordinal
							AND slot.assertion_id = assertion.assertion_id
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_relational_assertion_slots_v3 slot
				WHERE slot.project_id = NEW.project_id
					AND slot.event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_relational_assertion_fillers_v3 filler
						WHERE filler.project_id = slot.project_id
							AND filler.event_ref = slot.event_ref
							AND filler.change_ordinal = slot.change_ordinal
							AND filler.assertion_id = slot.assertion_id
							AND filler.slot_ordinal = slot.slot_ordinal
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_relational_assertion_fillers_v3 filler
				WHERE filler.project_id = NEW.project_id
					AND filler.event_ref = NEW.event_ref
					AND filler.filler_kind = 'by_reference'
					AND (
						NOT EXISTS (
							SELECT 1 FROM typed_memory_relational_assertion_reference_resolution_uses_v3 resolution_use
							WHERE resolution_use.project_id = filler.project_id
								AND resolution_use.event_ref = filler.event_ref
								AND resolution_use.change_ordinal = filler.change_ordinal
								AND resolution_use.assertion_id = filler.assertion_id
								AND resolution_use.slot_ordinal = filler.slot_ordinal
								AND resolution_use.filler_ordinal = filler.filler_ordinal
						)
						OR NOT EXISTS (
							SELECT 1 FROM typed_memory_relational_assertion_memberof_uses_v3 member_use
							WHERE member_use.project_id = filler.project_id
								AND member_use.event_ref = filler.event_ref
								AND member_use.change_ordinal = filler.change_ordinal
								AND member_use.assertion_id = filler.assertion_id
								AND member_use.slot_ordinal = filler.slot_ordinal
								AND member_use.filler_ordinal = filler.filler_ordinal
								AND member_use.use_kind = 'required_member'
						)
					)
			)`
	return source[:index] + v3Completeness + source[index:], nil
}

func typedMemoryGraphCommitExactClosureTrigger53() (string, error) {
	source, err := typedMemoryGraphCommitExactClosureTrigger52()
	if err != nil {
		return "", fmt.Errorf("derive v53 graph-commit source: %w", err)
	}
	legacy := typedMemoryGraphCommitExactClosureTrigger46()
	prefix := `CREATE TRIGGER typed_memory_graph_commits_exact_closure
	BEFORE INSERT ON typed_memory_graph_commits
	WHEN NOT EXISTS (`
	suffix := "\n\t) BEGIN\n\t\tSELECT RAISE(ABORT"
	if !strings.HasPrefix(legacy, prefix) {
		return "", fmt.Errorf("v53 graph-commit source prefix changed")
	}
	legacyEnd := strings.LastIndex(legacy, suffix)
	if legacyEnd < 0 {
		return "", fmt.Errorf("v53 graph-commit source suffix changed")
	}
	writer53 := legacy[len(prefix):legacyEnd]
	writer53 = strings.Replace(writer53, "generation.writer_generation = 46", "generation.writer_generation = 53", 1)
	writer53 = strings.Replace(writer53, "generation.provenance_kind = 'writer_v46'", "generation.provenance_kind = 'writer_v53'", 1)
	sourceEnd := strings.LastIndex(source, suffix)
	if sourceEnd < 0 {
		return "", fmt.Errorf("v53 graph-commit v52 seam changed")
	}
	branch := "\n\t) AND NOT EXISTS (" + writer53
	return source[:sourceEnd] + branch + source[sourceEnd:], nil
}

func typedMemoryRelationalAssertionTargetObjects53() (
	[]typedMemoryRelationalAssertionObject53,
	error,
) {
	graphEvents, err := typedMemoryGraphEventsTable53()
	if err != nil {
		return nil, err
	}
	footprint, err := typedMemoryEventMaterializationFootprintsView53()
	if err != nil {
		return nil, err
	}
	closure, err := typedMemoryCommitClosureExactFootprintTrigger53()
	if err != nil {
		return nil, err
	}
	commit, err := typedMemoryGraphCommitExactClosureTrigger53()
	if err != nil {
		return nil, err
	}
	objects := []typedMemoryRelationalAssertionObject53{
		{kind: "table", name: "typed_memory_graph_events", sql: graphEvents},
		{kind: "table", name: "typed_memory_event_writer_generations", sql: typedMemoryEventWriterGenerationsTable53()},
		{kind: "index", name: "idx_typed_memory_events_project_revision", sql: "CREATE INDEX idx_typed_memory_events_project_revision ON typed_memory_graph_events(project_id, graph_revision DESC)"},
		{kind: "trigger", name: "typed_memory_graph_events_exact_head", sql: typedMemoryGraphEventExactHeadTrigger45()},
		{kind: "trigger", name: "typed_memory_graph_events_no_update", sql: immutableTypedMemoryTrigger45("typed_memory_graph_events", "update")},
		{kind: "trigger", name: "typed_memory_graph_events_no_delete", sql: immutableTypedMemoryTrigger45("typed_memory_graph_events", "delete")},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v46_open_event", sql: typedMemoryOpenEventTrigger46("typed_memory_event_writer_generations")},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v46_no_update", sql: immutableTypedMemoryTrigger46("typed_memory_event_writer_generations", "update")},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v46_no_delete", sql: immutableTypedMemoryTrigger46("typed_memory_event_writer_generations", "delete")},
		{kind: "table", name: typedMemoryWriterCapabilitiesTable53, sql: typedMemoryWriterCapabilitiesTableStatement53()},
		{kind: "trigger", name: "typed_memory_event_writer_generations_v53_exact_boundary", sql: typedMemoryEventWriterGenerationExactBoundaryTrigger53()},
		{kind: "trigger", name: "typed_memory_writer_capabilities_v53_exact_marker", sql: typedMemoryWriterCapabilityExactMarkerTrigger53()},
		{kind: "trigger", name: typedMemoryWriterCapabilitiesTable53 + "_v53_no_update", sql: immutableTypedMemoryTrigger53(typedMemoryWriterCapabilitiesTable53, "update")},
		{kind: "trigger", name: typedMemoryWriterCapabilitiesTable53 + "_v53_no_delete", sql: immutableTypedMemoryTrigger53(typedMemoryWriterCapabilitiesTable53, "delete")},
		{kind: "trigger", name: "typed_memory_relation_instances_v53_legacy_insert_frozen", sql: typedMemoryLegacyRelationInsertFreezeTrigger53()},
		{kind: "view", name: "typed_memory_event_materialization_footprints_v46", sql: footprint},
		{kind: "trigger", name: "typed_memory_commit_materialization_closures_v46_exact_footprint", sql: closure},
		{kind: "trigger", name: "typed_memory_graph_commits_exact_closure", sql: commit},
	}
	tables := typedMemoryRelationalAssertionTableStatements53()
	tableNames := []string{
		typedMemoryRelationalAssertionsTable53,
		typedMemoryRelationalAssertionSlotsTable53,
		typedMemoryRelationalAssertionFillersTable53,
		typedMemoryRelationalAssertionResolutionsTable53,
		typedMemoryRelationalAssertionMemberOfUsesTable53,
		typedMemoryRelationalAssertionDisjointUsesTable53,
	}
	for index, table := range tables {
		objects = append(objects, typedMemoryRelationalAssertionObject53{
			kind: "table",
			name: tableNames[index],
			sql:  table,
		})
	}
	for _, index := range typedMemoryRelationalAssertionIndexStatements53() {
		fields := strings.Fields(index)
		objects = append(objects, typedMemoryRelationalAssertionObject53{
			kind: "index",
			name: fields[2],
			sql:  index,
		})
	}
	for _, trigger := range typedMemoryRelationalAssertionTriggerStatements53() {
		fields := strings.Fields(trigger)
		objects = append(objects, typedMemoryRelationalAssertionObject53{
			kind: "trigger",
			name: fields[2],
			sql:  trigger,
		})
	}
	return objects, nil
}

func verifyTypedMemoryRelationalAssertionFootprint53(
	tx MigrationTransaction,
) error {
	objects, err := typedMemoryRelationalAssertionTargetObjects53()
	if err != nil {
		return err
	}
	for _, object := range objects {
		if err := requireExactTypedMemoryObject53(tx, object, "v53 target"); err != nil {
			return err
		}
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
		return fmt.Errorf("load v53 writer capability: %w", err)
	}
	matches := generation == typedMemoryWriterGeneration53
	matches = matches && digest == typedMemoryWriterMarkerDigest53
	matches = matches && bytes.Equal(canonical, []byte(typedMemoryWriterMarkerBytes53))
	if !matches {
		return fmt.Errorf("v53 writer capability marker differs from its sealed bytes")
	}
	return verifyTypedMemoryRelationalAssertionTablesEmpty53(tx)
}

func verifyTypedMemoryRelationalAssertionTablesEmpty53(
	tx MigrationTransaction,
) error {
	for _, table := range []string{
		typedMemoryRelationalAssertionsTable53,
		typedMemoryRelationalAssertionSlotsTable53,
		typedMemoryRelationalAssertionFillersTable53,
		typedMemoryRelationalAssertionResolutionsTable53,
		typedMemoryRelationalAssertionMemberOfUsesTable53,
		typedMemoryRelationalAssertionDisjointUsesTable53,
	} {
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			return fmt.Errorf("count migrated v53 table %s: %w", table, err)
		}
		if count != 0 {
			return fmt.Errorf("v53 migration unexpectedly materialized %d rows in %s", count, table)
		}
	}
	return nil
}
