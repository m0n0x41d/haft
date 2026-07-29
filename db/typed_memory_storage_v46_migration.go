package db

import (
	"fmt"
	"strings"
)

const (
	typedMemoryWriterGenerationCapability46 = "typed_memory_writer_generation"
	typedMemoryWriterGeneration46           = 46
	typedMemoryWriterMarkerBytes46          = "haft.typed-memory.storage.writer-generation=46"
	typedMemoryWriterMarkerDigest46         = "sha256:ef94ecbd2590981b016eb89ba3f1c4f9a101ebbbf779355e6c3f7e13a5cdd58a"
)

var typedMemoryStorageMigration46 = Migration{
	Version:     46,
	Description: "Add exact admission envelopes and complete typed-memory materialization closure",
	Apply:       applyTypedMemoryStorageMigration46,
}

var typedMemoryStorageTables46 = []string{
	"typed_memory_storage_capabilities",
	"typed_memory_event_writer_generations",
	"typed_memory_event_admission_bases",
	"typed_memory_context_slice_catalog",
	"typed_memory_context_slices",
	"typed_memory_entity_declarations",
	"typed_memory_value_blobs",
	"typed_memory_observable_input_blobs",
	"typed_memory_relation_instances",
	"typed_memory_relation_slots",
	"typed_memory_relation_fillers",
	"typed_memory_ordered_candidate_prefixes",
	"typed_memory_reference_resolution_uses",
	"typed_memory_memberof_evaluations",
	"typed_memory_memberof_observable_inputs",
	"typed_memory_relation_filler_memberof_uses",
	"typed_memory_alias_changes",
	"typed_memory_assertion_retractions",
	"typed_memory_commit_materialization_closures",
}

var typedMemoryStorageIndexes46 = []string{
	"idx_typed_memory_context_slices_digest_v46",
	"idx_typed_memory_entity_declarations_entity_v46",
	"idx_typed_memory_value_blobs_digest_v46",
	"idx_typed_memory_observable_blobs_digest_v46",
	"idx_typed_memory_relations_assertion_v46",
	"idx_typed_memory_fillers_entity_v46",
	"idx_typed_memory_memberof_entity_kind_v46",
	"idx_typed_memory_alias_lookup_v46",
	"idx_typed_memory_alias_replacement_lookup_v46",
	"idx_typed_memory_alias_supersession_lineage_v46",
	"idx_typed_memory_retractions_assertion_v46",
}

var typedMemoryStorageViews46 = []string{
	"typed_memory_event_materialization_footprints_v46",
}

var typedMemoryStorageSpecificTriggers46 = []string{
	"typed_memory_storage_capabilities_v46_exact_marker",
	"typed_memory_event_writer_generations_v46_exact_boundary",
	"typed_memory_event_admission_bases_v46_exact_event",
	"typed_memory_entity_declarations_v46_exact_materialization",
	"typed_memory_relation_instances_v46_exact_context_slice",
	"typed_memory_relation_slots_v46_exact_relation",
	"typed_memory_relation_fillers_v46_exact_slot",
	"typed_memory_reference_resolution_uses_v46_exact_filler",
	"typed_memory_memberof_evaluations_v46_exact_context_slice",
	"typed_memory_memberof_observable_inputs_v46_exact_evaluation_input",
	"typed_memory_relation_filler_memberof_uses_v46_exact_use",
	"typed_memory_alias_changes_v46_exact_supersession",
	"typed_memory_commit_materialization_closures_v46_basis_kind",
	"typed_memory_commit_materialization_closures_v46_exact_footprint",
}

type typedMemoryStorageObject46 struct {
	kind string
	name string
}

func applyTypedMemoryStorageMigration46(
	tx MigrationTransaction,
	_ []Migration,
) error {
	if err := requireTypedMemoryStorageSource46(tx); err != nil {
		return err
	}
	objects := typedMemoryStorageObjects46()
	if err := requireAbsentTypedMemoryStorageFootprint46(tx, objects, 0); err != nil {
		return err
	}
	statements := typedMemoryStorageStatements46()
	if err := executeStatements(tx, statements, 0); err != nil {
		return fmt.Errorf("install exact typed-memory admission storage: %w", err)
	}
	if err := verifyForeignKeys(tx); err != nil {
		return fmt.Errorf("verify exact typed-memory admission storage: %w", err)
	}
	return nil
}

func requireTypedMemoryStorageSource46(tx MigrationTransaction) error {
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM schema_version WHERE version = 45",
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect exact typed-memory storage source migration: %w", err)
	}
	if count != 1 {
		return fmt.Errorf("exact typed-memory admission storage requires schema version 45")
	}
	return requireExactTypedMemoryStorageFootprint45(
		tx,
		typedMemoryStorageExpectedObjects45(),
		0,
	)
}

type typedMemoryStorageExpectedObject45 struct {
	kind string
	name string
	sql  string
}

func requireExactTypedMemoryStorageFootprint45(
	tx MigrationTransaction,
	objects []typedMemoryStorageExpectedObject45,
	index int,
) error {
	if index >= len(objects) {
		return nil
	}
	object := objects[index]
	actualSQL := ""
	err := tx.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		object.kind,
		object.name,
	).Scan(&actualSQL)
	if err != nil {
		return fmt.Errorf(
			"exact typed-memory admission storage requires exact v45 %s %s: %w",
			object.kind,
			object.name,
			err,
		)
	}
	if normalizeSQLiteDDL46(actualSQL) != normalizeSQLiteDDL46(object.sql) {
		return fmt.Errorf(
			"exact typed-memory admission storage requires exact v45 %s %s; source footprint drifted",
			object.kind,
			object.name,
		)
	}
	return requireExactTypedMemoryStorageFootprint45(tx, objects, index+1)
}

func typedMemoryStorageExpectedObjects45() []typedMemoryStorageExpectedObject45 {
	statements := typedMemoryStorageStatements45()
	objects := make([]typedMemoryStorageExpectedObject45, 0, len(statements))
	for _, statement := range statements {
		fields := strings.Fields(statement)
		kind := ""
		nameIndex := 2
		if len(fields) >= 4 && fields[0] == "CREATE" && fields[1] == "UNIQUE" {
			kind = strings.ToLower(fields[2])
			nameIndex = 3
		} else if len(fields) >= 3 && fields[0] == "CREATE" {
			kind = strings.ToLower(fields[1])
		}
		if kind == "" || nameIndex >= len(fields) {
			continue
		}
		objects = append(objects, typedMemoryStorageExpectedObject45{
			kind: kind,
			name: strings.Trim(fields[nameIndex], "`\"[ ]"),
			sql:  statement,
		})
	}
	return objects
}

func normalizeSQLiteDDL46(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func requireAbsentTypedMemoryStorageFootprint46(
	tx MigrationTransaction,
	objects []typedMemoryStorageObject46,
	index int,
) error {
	if index >= len(objects) {
		return nil
	}
	object := objects[index]
	count := 0
	err := tx.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
		object.kind,
		object.name,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("inspect v46 typed-memory %s %s: %w", object.kind, object.name, err)
	}
	if count != 0 {
		return fmt.Errorf(
			"exact typed-memory admission storage refused: unversioned %s %s already exists; unknown partial v46 footprint requires manual review",
			object.kind,
			object.name,
		)
	}
	return requireAbsentTypedMemoryStorageFootprint46(tx, objects, index+1)
}

func typedMemoryStorageObjects46() []typedMemoryStorageObject46 {
	objects := make([]typedMemoryStorageObject46, 0)
	for _, table := range typedMemoryStorageTables46 {
		objects = append(objects, typedMemoryStorageObject46{kind: "table", name: table})
	}
	for _, view := range typedMemoryStorageViews46 {
		objects = append(objects, typedMemoryStorageObject46{kind: "view", name: view})
	}
	for _, index := range typedMemoryStorageIndexes46 {
		objects = append(objects, typedMemoryStorageObject46{kind: "index", name: index})
	}
	for _, trigger := range typedMemoryStorageSpecificTriggers46 {
		objects = append(objects, typedMemoryStorageObject46{kind: "trigger", name: trigger})
	}
	for _, table := range typedMemoryStorageTables46 {
		objects = append(objects, typedMemoryStorageObject46{
			kind: "trigger",
			name: table + "_v46_no_update",
		})
		objects = append(objects, typedMemoryStorageObject46{
			kind: "trigger",
			name: table + "_v46_no_delete",
		})
		if table == "typed_memory_storage_capabilities" {
			continue
		}
		objects = append(objects, typedMemoryStorageObject46{
			kind: "trigger",
			name: table + "_v46_open_event",
		})
	}
	return objects
}

func typedMemoryStorageStatements46() []string {
	statements := []string{
		typedMemoryStorageCapabilitiesTable46(),
		typedMemoryEventWriterGenerationsTable46(),
		typedMemoryEventAdmissionBasesTable46(),
		typedMemoryContextSliceCatalogTable46(),
		typedMemoryContextSlicesTable46(),
		typedMemoryEntityDeclarationsTable46(),
		typedMemoryValueBlobsTable46(),
		typedMemoryObservableInputBlobsTable46(),
		typedMemoryRelationInstancesTable46(),
		typedMemoryRelationSlotsTable46(),
		typedMemoryRelationFillersTable46(),
		typedMemoryOrderedCandidatePrefixesTable46(),
		typedMemoryReferenceResolutionUsesTable46(),
		typedMemoryMemberOfEvaluationsTable46(),
		typedMemoryMemberOfObservableInputsTable46(),
		typedMemoryRelationFillerMemberOfUsesTable46(),
		typedMemoryAliasChangesTable46(),
		typedMemoryAssertionRetractionsTable46(),
		typedMemoryCommitMaterializationClosuresTable46(),
		typedMemoryEventMaterializationFootprintsView46(),
		typedMemoryLegacyWriterGenerationBackfill46(),
		typedMemoryStorageCapabilityMarkerInsert46(),
	}
	statements = append(statements, typedMemoryStorageIndexStatements46()...)
	statements = append(statements, typedMemoryStorageTriggerStatements46()...)
	return statements
}

func typedMemoryStorageIndexStatements46() []string {
	return []string{
		"CREATE INDEX idx_typed_memory_context_slices_digest_v46 ON typed_memory_context_slices(project_id, context_slice_digest)",
		"CREATE INDEX idx_typed_memory_entity_declarations_entity_v46 ON typed_memory_entity_declarations(project_id, entity_id, bounded_context_ref)",
		"CREATE INDEX idx_typed_memory_value_blobs_digest_v46 ON typed_memory_value_blobs(project_id, value_digest)",
		"CREATE INDEX idx_typed_memory_observable_blobs_digest_v46 ON typed_memory_observable_input_blobs(project_id, observable_input_digest)",
		"CREATE INDEX idx_typed_memory_relations_assertion_v46 ON typed_memory_relation_instances(project_id, assertion_id)",
		"CREATE INDEX idx_typed_memory_fillers_entity_v46 ON typed_memory_relation_fillers(project_id, entity_id) WHERE filler_kind = 'by_reference'",
		"CREATE INDEX idx_typed_memory_memberof_entity_kind_v46 ON typed_memory_memberof_evaluations(project_id, entity_id, value_kind_ref)",
		"CREATE INDEX idx_typed_memory_alias_lookup_v46 ON typed_memory_alias_changes(project_id, bounded_context_ref, alias)",
		"CREATE INDEX idx_typed_memory_alias_replacement_lookup_v46 ON typed_memory_alias_changes(project_id, bounded_context_ref, replacement_alias) WHERE replacement_alias IS NOT NULL",
		"CREATE UNIQUE INDEX idx_typed_memory_alias_supersession_lineage_v46 ON typed_memory_alias_changes(project_id, supersedes_alias_change_ref) WHERE supersedes_alias_change_ref != ''",
		"CREATE INDEX idx_typed_memory_retractions_assertion_v46 ON typed_memory_assertion_retractions(project_id, assertion_id)",
	}
}

func typedMemoryStorageCapabilitiesTable46() string {
	return `CREATE TABLE typed_memory_storage_capabilities (
		capability_key TEXT PRIMARY KEY CHECK(capability_key = 'typed_memory_writer_generation'),
		writer_generation INTEGER NOT NULL CHECK(writer_generation = 46),
		capability_digest TEXT NOT NULL UNIQUE CHECK(
			capability_digest = 'sha256:ef94ecbd2590981b016eb89ba3f1c4f9a101ebbbf779355e6c3f7e13a5cdd58a'
		),
		canonical_bytes BLOB NOT NULL UNIQUE CHECK(
			CAST(canonical_bytes AS TEXT) = 'haft.typed-memory.storage.writer-generation=46'
		),
		installed_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("installed_at") + `)
	) WITHOUT ROWID`
}

func typedMemoryEventWriterGenerationsTable46() string {
	return `CREATE TABLE typed_memory_event_writer_generations (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		writer_generation INTEGER NOT NULL CHECK(writer_generation IN (45, 46)),
		provenance_kind TEXT NOT NULL CHECK(
			(writer_generation = 45 AND provenance_kind = 'migration_v45_backfill')
			OR (writer_generation = 46 AND provenance_kind = 'writer_v46')
		),
		PRIMARY KEY(project_id, event_ref),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryEventAdmissionBasesTable46() string {
	return `CREATE TABLE typed_memory_event_admission_bases (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		event_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("event_digest") + `),
		admission_basis_kind TEXT NOT NULL CHECK(
			admission_basis_kind IN ('snapshot_only', 'context_slice_membership')
		),
		type_env_ref TEXT NOT NULL REFERENCES typed_memory_type_env_snapshots(type_env_ref),
		basis_graph_revision INTEGER NOT NULL CHECK(basis_graph_revision >= 0),
		request_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("request_digest") + `),
		canonical_request_bytes BLOB NOT NULL CHECK(length(canonical_request_bytes) > 0),
		semantic_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("semantic_digest") + `),
		canonical_semantic_bytes BLOB NOT NULL CHECK(length(canonical_semantic_bytes) > 0),
		admission_envelope_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("admission_envelope_digest") + `),
		canonical_admission_envelope_bytes BLOB NOT NULL CHECK(length(canonical_admission_envelope_bytes) > 0),
		admission_basis_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("admission_basis_digest") + `),
		canonical_admission_basis_bytes BLOB NOT NULL CHECK(length(canonical_admission_basis_bytes) > 0),
		materialization_manifest_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("materialization_manifest_digest") + `),
		canonical_materialization_manifest_bytes BLOB NOT NULL CHECK(length(canonical_materialization_manifest_bytes) > 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, event_ref),
		UNIQUE(project_id, event_ref, admission_basis_kind, materialization_manifest_digest),
		UNIQUE(project_id, event_ref, request_digest),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		CHECK(request_digest != semantic_digest),
		CHECK(admission_envelope_digest != request_digest),
		CHECK(admission_envelope_digest != semantic_digest),
		CHECK(admission_basis_digest != admission_envelope_digest)
	) WITHOUT ROWID`
}

func typedMemoryContextSliceCatalogTable46() string {
	return `CREATE TABLE typed_memory_context_slice_catalog (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		context_slice_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("context_slice_ref") + `),
		context_slice_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("context_slice_digest") + `),
		bounded_context_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("bounded_context_ref") + `),
		canonical_context_slice_bytes BLOB NOT NULL CHECK(length(canonical_context_slice_bytes) > 0),
		PRIMARY KEY(project_id, context_slice_ref),
		UNIQUE(
			project_id, context_slice_ref, context_slice_digest,
			bounded_context_ref, canonical_context_slice_bytes
		),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		CHECK(context_slice_ref = 'context-slice:' || context_slice_digest)
	) WITHOUT ROWID`
}

func typedMemoryContextSlicesTable46() string {
	return `CREATE TABLE typed_memory_context_slices (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		context_slice_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("context_slice_ref") + `),
		context_slice_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("context_slice_digest") + `),
		bounded_context_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("bounded_context_ref") + `),
		canonical_context_slice_bytes BLOB NOT NULL CHECK(length(canonical_context_slice_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, context_slice_ref),
		UNIQUE(project_id, event_ref, context_slice_ref, context_slice_digest),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(
			project_id, context_slice_ref, context_slice_digest,
			bounded_context_ref, canonical_context_slice_bytes
		) REFERENCES typed_memory_context_slice_catalog(
			project_id, context_slice_ref, context_slice_digest,
			bounded_context_ref, canonical_context_slice_bytes
		),
		CHECK(context_slice_ref = 'context-slice:' || context_slice_digest)
	) WITHOUT ROWID`
}

func typedMemoryEntityDeclarationsTable46() string {
	return `CREATE TABLE typed_memory_entity_declarations (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		entity_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("entity_id") + `),
		batch_local_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("batch_local_ref") + `),
		bounded_context_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("bounded_context_ref") + `),
		label TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("label") + `),
		provenance_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("provenance_ref") + `),
		declaration_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("declaration_digest") + `),
		canonical_declaration_bytes BLOB NOT NULL CHECK(length(canonical_declaration_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, change_ordinal),
		UNIQUE(project_id, event_ref, entity_id, bounded_context_ref),
		UNIQUE(project_id, event_ref, batch_local_ref),
		UNIQUE(
			project_id, event_ref, change_ordinal,
			entity_id, declaration_digest, batch_local_ref
		),
		UNIQUE(
			project_id, event_ref, change_ordinal,
			entity_id, declaration_digest
		),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryValueBlobsTable46() string {
	return `CREATE TABLE typed_memory_value_blobs (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		value_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("value_ref") + `),
		value_kind_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("value_kind_ref") + `),
		value_shape_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("value_shape_ref") + `),
		codec_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("codec_ref") + `),
		value_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("value_digest") + `),
		canonical_value_bytes BLOB NOT NULL CHECK(length(canonical_value_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, value_ref),
		UNIQUE(project_id, event_ref, value_ref, value_digest),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryObservableInputBlobsTable46() string {
	return `CREATE TABLE typed_memory_observable_input_blobs (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		observable_input_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("observable_input_ref") + `),
		observable_input_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("observable_input_digest") + `),
		canonical_observable_input_bytes BLOB NOT NULL CHECK(length(canonical_observable_input_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, observable_input_ref),
		UNIQUE(project_id, event_ref, observable_input_ref, observable_input_digest),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryRelationInstancesTable46() string {
	return `CREATE TABLE typed_memory_relation_instances (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		assertion_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("assertion_id") + `),
		signature_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("signature_ref") + `),
		context_slice_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("context_slice_ref") + `),
		relation_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("relation_digest") + `),
		canonical_relation_bytes BLOB NOT NULL CHECK(length(canonical_relation_bytes) > 0),
		provenance_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("provenance_ref") + `),
		PRIMARY KEY(project_id, event_ref, change_ordinal),
		UNIQUE(project_id, event_ref, change_ordinal, assertion_id),
		UNIQUE(project_id, event_ref, assertion_id),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(project_id, event_ref, context_slice_ref)
			REFERENCES typed_memory_context_slices(project_id, event_ref, context_slice_ref)
	) WITHOUT ROWID`
}

func typedMemoryRelationSlotsTable46() string {
	return `CREATE TABLE typed_memory_relation_slots (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		assertion_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("assertion_id") + `),
		slot_ordinal INTEGER NOT NULL CHECK(slot_ordinal >= 0),
		slot_kind_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("slot_kind_ref") + `),
		slot_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("slot_digest") + `),
		canonical_slot_bytes BLOB NOT NULL CHECK(length(canonical_slot_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, change_ordinal, slot_ordinal),
		UNIQUE(project_id, event_ref, change_ordinal, assertion_id, slot_ordinal),
		UNIQUE(project_id, event_ref, change_ordinal, slot_kind_ref),
		FOREIGN KEY(project_id, event_ref, change_ordinal, assertion_id)
			REFERENCES typed_memory_relation_instances(project_id, event_ref, change_ordinal, assertion_id)
	) WITHOUT ROWID`
}

func typedMemoryRelationFillersTable46() string {
	return `CREATE TABLE typed_memory_relation_fillers (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		assertion_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("assertion_id") + `),
		slot_ordinal INTEGER NOT NULL CHECK(slot_ordinal >= 0),
		filler_ordinal INTEGER NOT NULL CHECK(filler_ordinal >= 0),
		filler_kind TEXT NOT NULL CHECK(filler_kind IN ('by_reference', 'by_value')),
		reference_kind_ref TEXT NOT NULL DEFAULT '',
		reference_id TEXT NOT NULL DEFAULT '',
		entity_id TEXT NOT NULL DEFAULT '',
		required_value_kind_ref TEXT NOT NULL DEFAULT '',
		value_ref TEXT NOT NULL DEFAULT '',
		filler_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("filler_digest") + `),
		canonical_filler_bytes BLOB NOT NULL CHECK(length(canonical_filler_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, change_ordinal, slot_ordinal, filler_ordinal),
		UNIQUE(
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest
		),
		FOREIGN KEY(project_id, event_ref, change_ordinal, assertion_id, slot_ordinal)
			REFERENCES typed_memory_relation_slots(project_id, event_ref, change_ordinal, assertion_id, slot_ordinal),
		CHECK(
			(filler_kind = 'by_reference'
				AND reference_kind_ref != ''
				AND trim(reference_kind_ref) = reference_kind_ref
				AND reference_id != ''
				AND trim(reference_id) = reference_id
				AND entity_id != ''
				AND trim(entity_id) = entity_id
				AND required_value_kind_ref != ''
				AND trim(required_value_kind_ref) = required_value_kind_ref
				AND value_ref = '')
			OR (filler_kind = 'by_value'
				AND reference_kind_ref = ''
				AND reference_id = ''
				AND entity_id = ''
				AND required_value_kind_ref = ''
				AND value_ref != ''
				AND trim(value_ref) = value_ref)
		)
	) WITHOUT ROWID`
}

func typedMemoryOrderedCandidatePrefixesTable46() string {
	return `CREATE TABLE typed_memory_ordered_candidate_prefixes (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		prefix_end_ordinal INTEGER NOT NULL CHECK(prefix_end_ordinal > 0),
		request_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("request_digest") + `),
		prefix_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("prefix_digest") + `),
		PRIMARY KEY(project_id, event_ref, prefix_end_ordinal),
		UNIQUE(project_id, event_ref, prefix_end_ordinal, prefix_digest),
		FOREIGN KEY(project_id, event_ref, request_digest)
			REFERENCES typed_memory_event_admission_bases(
				project_id, event_ref, request_digest
			)
	) WITHOUT ROWID`
}

func typedMemoryReferenceResolutionUsesTable46() string {
	return `CREATE TABLE typed_memory_reference_resolution_uses (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		assertion_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("assertion_id") + `),
		slot_ordinal INTEGER NOT NULL CHECK(slot_ordinal >= 0),
		filler_ordinal INTEGER NOT NULL CHECK(filler_ordinal >= 0),
		filler_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("filler_digest") + `),
		entity_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("entity_id") + `),
		resolution_kind TEXT NOT NULL CHECK(
			resolution_kind IN ('snapshot_reference', 'same_batch_declaration')
		),
		resolution_basis_ref TEXT,
		declaration_change_ordinal INTEGER,
		local_reference_kind_ref TEXT,
		batch_local_ref TEXT,
		declaration_digest TEXT,
		ordered_candidate_prefix_digest TEXT,
		resolution_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("resolution_digest") + `),
		canonical_resolution_bytes BLOB NOT NULL CHECK(length(canonical_resolution_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, change_ordinal, slot_ordinal, filler_ordinal),
		FOREIGN KEY(
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest
		) REFERENCES typed_memory_relation_fillers(
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest
		),
		FOREIGN KEY(
			project_id, event_ref, declaration_change_ordinal,
			entity_id, declaration_digest, batch_local_ref
		) REFERENCES typed_memory_entity_declarations(
			project_id, event_ref, change_ordinal,
			entity_id, declaration_digest, batch_local_ref
		),
		FOREIGN KEY(
			project_id, event_ref, change_ordinal,
			ordered_candidate_prefix_digest
		) REFERENCES typed_memory_ordered_candidate_prefixes(
			project_id, event_ref, prefix_end_ordinal, prefix_digest
		),
		CHECK(
			(resolution_kind = 'snapshot_reference'
				AND resolution_basis_ref IS NOT NULL
				AND resolution_basis_ref != ''
				AND trim(resolution_basis_ref) = resolution_basis_ref
				AND declaration_change_ordinal IS NULL
				AND local_reference_kind_ref IS NULL
				AND batch_local_ref IS NULL
				AND declaration_digest IS NULL
				AND ordered_candidate_prefix_digest IS NULL)
			OR (resolution_kind = 'same_batch_declaration'
				AND resolution_basis_ref IS NULL
				AND declaration_change_ordinal IS NOT NULL
				AND declaration_change_ordinal >= 0
				AND declaration_change_ordinal < change_ordinal
				AND local_reference_kind_ref IS NOT NULL
				AND local_reference_kind_ref != ''
				AND trim(local_reference_kind_ref) = local_reference_kind_ref
				AND batch_local_ref IS NOT NULL
				AND batch_local_ref != ''
				AND trim(batch_local_ref) = batch_local_ref
				AND declaration_digest IS NOT NULL
				AND ` + typedMemorySHA256Shape46("declaration_digest") + `
				AND ordered_candidate_prefix_digest IS NOT NULL
				AND ` + typedMemorySHA256Shape46("ordered_candidate_prefix_digest") + `)
		)
	) WITHOUT ROWID`
}

func typedMemoryMemberOfEvaluationsTable46() string {
	return `CREATE TABLE typed_memory_memberof_evaluations (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		evaluation_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("evaluation_ref") + `),
		judgement_kind TEXT NOT NULL CHECK(judgement_kind IN ('member', 'not_member')),
		entity_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("entity_id") + `),
		value_kind_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("value_kind_ref") + `),
		context_slice_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("context_slice_ref") + `),
		evaluator_rule_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("evaluator_rule_ref") + `),
		evaluation_provenance_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("evaluation_provenance_ref") + `),
		evaluation_view_kind TEXT NOT NULL CHECK(
			evaluation_view_kind IN ('persisted_snapshot', 'prospective_batch')
		),
		evaluation_view_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("evaluation_view_digest") + `),
		canonical_evaluation_view_bytes BLOB NOT NULL CHECK(length(canonical_evaluation_view_bytes) > 0),
		view_declaration_change_ordinal INTEGER,
		view_local_reference_kind_ref TEXT,
		view_batch_local_ref TEXT,
		view_declaration_digest TEXT,
		view_prefix_end_ordinal INTEGER,
		view_ordered_candidate_prefix_digest TEXT,
		observable_input_count INTEGER NOT NULL CHECK(observable_input_count > 0),
		observable_input_set_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("observable_input_set_digest") + `),
		query_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("query_digest") + `),
		canonical_query_bytes BLOB NOT NULL CHECK(length(canonical_query_bytes) > 0),
		basis_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("basis_digest") + `),
		canonical_basis_bytes BLOB NOT NULL CHECK(length(canonical_basis_bytes) > 0),
		judgement_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("judgement_digest") + `),
		canonical_judgement_bytes BLOB NOT NULL CHECK(length(canonical_judgement_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, evaluation_ref),
		UNIQUE(project_id, event_ref, evaluation_ref, judgement_kind),
		UNIQUE(project_id, event_ref, evaluation_ref, entity_id, value_kind_ref, query_digest, judgement_kind),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		FOREIGN KEY(project_id, event_ref, context_slice_ref)
			REFERENCES typed_memory_context_slices(project_id, event_ref, context_slice_ref),
		FOREIGN KEY(
			project_id, event_ref, view_declaration_change_ordinal,
			entity_id, view_declaration_digest
		) REFERENCES typed_memory_entity_declarations(
			project_id, event_ref, change_ordinal,
			entity_id, declaration_digest
		),
		FOREIGN KEY(
			project_id, event_ref, view_prefix_end_ordinal,
			view_ordered_candidate_prefix_digest
		) REFERENCES typed_memory_ordered_candidate_prefixes(
			project_id, event_ref, prefix_end_ordinal, prefix_digest
		),
		CHECK(
			(evaluation_view_kind = 'persisted_snapshot'
				AND view_declaration_change_ordinal IS NULL
				AND view_local_reference_kind_ref IS NULL
				AND view_batch_local_ref IS NULL
				AND view_declaration_digest IS NULL
				AND view_prefix_end_ordinal IS NULL
				AND view_ordered_candidate_prefix_digest IS NULL)
			OR (evaluation_view_kind = 'prospective_batch'
				AND view_declaration_change_ordinal IS NOT NULL
				AND view_declaration_change_ordinal >= 0
				AND view_local_reference_kind_ref IS NOT NULL
				AND ` + typedMemoryNonBlankShape46("view_local_reference_kind_ref") + `
				AND view_batch_local_ref IS NOT NULL
				AND ` + typedMemoryNonBlankShape46("view_batch_local_ref") + `
				AND view_declaration_digest IS NOT NULL
				AND ` + typedMemorySHA256Shape46("view_declaration_digest") + `
				AND view_prefix_end_ordinal IS NOT NULL
				AND view_prefix_end_ordinal > view_declaration_change_ordinal
				AND view_ordered_candidate_prefix_digest IS NOT NULL
				AND ` + typedMemorySHA256Shape46("view_ordered_candidate_prefix_digest") + `)
		)
	) WITHOUT ROWID`
}

func typedMemoryMemberOfObservableInputsTable46() string {
	return `CREATE TABLE typed_memory_memberof_observable_inputs (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		evaluation_ref TEXT NOT NULL,
		input_ordinal INTEGER NOT NULL CHECK(input_ordinal >= 0),
		observable_input_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("observable_input_ref") + `),
		observable_input_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("observable_input_digest") + `),
		PRIMARY KEY(project_id, event_ref, evaluation_ref, input_ordinal),
		UNIQUE(project_id, event_ref, evaluation_ref, observable_input_ref),
		FOREIGN KEY(project_id, event_ref, evaluation_ref)
			REFERENCES typed_memory_memberof_evaluations(project_id, event_ref, evaluation_ref),
		FOREIGN KEY(project_id, event_ref, observable_input_ref, observable_input_digest)
			REFERENCES typed_memory_observable_input_blobs(
				project_id, event_ref, observable_input_ref, observable_input_digest
			)
	) WITHOUT ROWID`
}

func typedMemoryRelationFillerMemberOfUsesTable46() string {
	return `CREATE TABLE typed_memory_relation_filler_memberof_uses (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		assertion_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("assertion_id") + `),
		slot_ordinal INTEGER NOT NULL CHECK(slot_ordinal >= 0),
		filler_ordinal INTEGER NOT NULL CHECK(filler_ordinal >= 0),
		filler_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("filler_digest") + `),
		use_kind TEXT NOT NULL CHECK(use_kind IN ('required_member', 'disjoint_not_member')),
		constraint_id TEXT NOT NULL DEFAULT '',
		queried_value_kind_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("queried_value_kind_ref") + `),
		query_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("query_digest") + `),
		evaluation_ref TEXT NOT NULL,
		expected_judgement_kind TEXT NOT NULL CHECK(expected_judgement_kind IN ('member', 'not_member')),
		use_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("use_digest") + `),
		canonical_use_bytes BLOB NOT NULL CHECK(length(canonical_use_bytes) > 0),
		PRIMARY KEY(
			project_id, event_ref, change_ordinal, slot_ordinal, filler_ordinal,
			use_kind, constraint_id, queried_value_kind_ref, query_digest
		),
		FOREIGN KEY(
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest
		) REFERENCES typed_memory_relation_fillers(
			project_id, event_ref, change_ordinal, assertion_id,
			slot_ordinal, filler_ordinal, filler_digest
		),
		FOREIGN KEY(project_id, event_ref, evaluation_ref, expected_judgement_kind)
			REFERENCES typed_memory_memberof_evaluations(
				project_id, event_ref, evaluation_ref, judgement_kind
			),
		CHECK(
			(use_kind = 'required_member'
				AND constraint_id = ''
				AND expected_judgement_kind = 'member')
			OR (use_kind = 'disjoint_not_member'
				AND constraint_id != ''
				AND expected_judgement_kind = 'not_member')
		)
	) WITHOUT ROWID`
}

func typedMemoryAliasChangesTable46() string {
	return `CREATE TABLE typed_memory_alias_changes (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		alias_change_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("alias_change_ref") + `),
		change_kind TEXT NOT NULL CHECK(change_kind IN ('admit_alias', 'supersede_alias')),
		bounded_context_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("bounded_context_ref") + `),
		alias TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("alias") + `),
		replacement_alias TEXT,
		entity_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("entity_id") + `),
		supersedes_alias_change_ref TEXT NOT NULL DEFAULT '',
		alias_change_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("alias_change_digest") + `),
		canonical_alias_change_bytes BLOB NOT NULL CHECK(length(canonical_alias_change_bytes) > 0),
		provenance_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("provenance_ref") + `),
		PRIMARY KEY(project_id, event_ref, change_ordinal),
		UNIQUE(project_id, alias_change_ref),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref),
		CHECK(
			(change_kind = 'admit_alias'
				AND replacement_alias IS NULL
				AND supersedes_alias_change_ref = '')
			OR (change_kind = 'supersede_alias'
				AND replacement_alias IS NOT NULL
				AND replacement_alias != ''
				AND trim(replacement_alias) = replacement_alias
				AND replacement_alias != alias
				AND supersedes_alias_change_ref != '')
		)
	) WITHOUT ROWID`
}

func typedMemoryAssertionRetractionsTable46() string {
	return `CREATE TABLE typed_memory_assertion_retractions (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		change_ordinal INTEGER NOT NULL CHECK(change_ordinal >= 0),
		retraction_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("retraction_ref") + `),
		assertion_id TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("assertion_id") + `),
		reason TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("reason") + `),
		provenance_ref TEXT NOT NULL CHECK(` + typedMemoryNonBlankShape46("provenance_ref") + `),
		retraction_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("retraction_digest") + `),
		canonical_retraction_bytes BLOB NOT NULL CHECK(length(canonical_retraction_bytes) > 0),
		PRIMARY KEY(project_id, event_ref, change_ordinal),
		UNIQUE(project_id, retraction_ref),
		FOREIGN KEY(project_id, event_ref)
			REFERENCES typed_memory_graph_events(project_id, event_ref)
	) WITHOUT ROWID`
}

func typedMemoryCommitMaterializationClosuresTable46() string {
	return `CREATE TABLE typed_memory_commit_materialization_closures (
		project_id TEXT NOT NULL,
		event_ref TEXT NOT NULL,
		commit_ref TEXT NOT NULL,
		event_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("event_digest") + `),
		admission_basis_kind TEXT NOT NULL CHECK(
			admission_basis_kind IN ('snapshot_only', 'context_slice_membership')
		),
		request_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("request_digest") + `),
		semantic_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("semantic_digest") + `),
		admission_envelope_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("admission_envelope_digest") + `),
		admission_basis_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("admission_basis_digest") + `),
		materialization_manifest_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("materialization_manifest_digest") + `),
		materialization_digest TEXT NOT NULL CHECK(` + typedMemorySHA256Shape46("materialization_digest") + `),
		canonical_materialization_bytes BLOB NOT NULL CHECK(length(canonical_materialization_bytes) > 0),
		entity_count INTEGER NOT NULL CHECK(entity_count >= 0),
		entity_context_count INTEGER NOT NULL CHECK(entity_context_count >= 0),
		entity_declaration_count INTEGER NOT NULL CHECK(entity_declaration_count >= 0),
		context_slice_catalog_count INTEGER NOT NULL CHECK(context_slice_catalog_count >= 0),
		context_slice_count INTEGER NOT NULL CHECK(context_slice_count >= 0),
		value_blob_count INTEGER NOT NULL CHECK(value_blob_count >= 0),
		observable_input_blob_count INTEGER NOT NULL CHECK(observable_input_blob_count >= 0),
		relation_count INTEGER NOT NULL CHECK(relation_count >= 0),
		relation_slot_count INTEGER NOT NULL CHECK(relation_slot_count >= 0),
		relation_filler_count INTEGER NOT NULL CHECK(relation_filler_count >= 0),
		ordered_candidate_prefix_count INTEGER NOT NULL CHECK(ordered_candidate_prefix_count >= 0),
		reference_resolution_use_count INTEGER NOT NULL CHECK(reference_resolution_use_count >= 0),
		memberof_evaluation_count INTEGER NOT NULL CHECK(memberof_evaluation_count >= 0),
		memberof_input_count INTEGER NOT NULL CHECK(memberof_input_count >= 0),
		memberof_use_count INTEGER NOT NULL CHECK(memberof_use_count >= 0),
		alias_change_count INTEGER NOT NULL CHECK(alias_change_count >= 0),
		retraction_count INTEGER NOT NULL CHECK(retraction_count >= 0),
		recorded_at TEXT NOT NULL CHECK(` + sqliteCanonicalUTCNanoShape("recorded_at") + `),
		PRIMARY KEY(project_id, event_ref),
		UNIQUE(project_id, commit_ref),
		FOREIGN KEY(project_id, event_ref, admission_basis_kind, materialization_manifest_digest)
			REFERENCES typed_memory_event_admission_bases(
				project_id, event_ref, admission_basis_kind, materialization_manifest_digest
			),
		FOREIGN KEY(project_id, commit_ref)
			REFERENCES typed_memory_graph_commits(project_id, commit_ref)
			DEFERRABLE INITIALLY DEFERRED
	) WITHOUT ROWID`
}

func typedMemoryEventMaterializationFootprintsView46() string {
	return `CREATE VIEW typed_memory_event_materialization_footprints_v46 AS
	SELECT
		event.project_id AS project_id,
		event.event_ref AS event_ref,
		(SELECT COUNT(*) FROM typed_memory_entities entity
			WHERE entity.project_id = event.project_id
				AND entity.first_declared_event_ref = event.event_ref) AS entity_count,
		(SELECT COUNT(*) FROM typed_memory_entity_contexts context
			WHERE context.project_id = event.project_id
				AND context.declared_event_ref = event.event_ref) AS entity_context_count,
		(SELECT COUNT(*) FROM typed_memory_entity_declarations declaration
			WHERE declaration.project_id = event.project_id
				AND declaration.event_ref = event.event_ref) AS entity_declaration_count,
		(SELECT COUNT(*) FROM typed_memory_context_slice_catalog catalog
			WHERE catalog.project_id = event.project_id
				AND catalog.event_ref = event.event_ref) AS context_slice_catalog_count,
		(SELECT COUNT(*) FROM typed_memory_context_slices slice
			WHERE slice.project_id = event.project_id
				AND slice.event_ref = event.event_ref) AS context_slice_count,
		(SELECT COUNT(*) FROM typed_memory_value_blobs value_blob
			WHERE value_blob.project_id = event.project_id
				AND value_blob.event_ref = event.event_ref) AS value_blob_count,
		(SELECT COUNT(*) FROM typed_memory_observable_input_blobs observable_blob
			WHERE observable_blob.project_id = event.project_id
				AND observable_blob.event_ref = event.event_ref) AS observable_input_blob_count,
		(SELECT COUNT(*) FROM typed_memory_relation_instances relation
			WHERE relation.project_id = event.project_id
				AND relation.event_ref = event.event_ref) AS relation_count,
		(SELECT COUNT(*) FROM typed_memory_relation_slots slot
			WHERE slot.project_id = event.project_id
				AND slot.event_ref = event.event_ref) AS relation_slot_count,
		(SELECT COUNT(*) FROM typed_memory_relation_fillers filler
			WHERE filler.project_id = event.project_id
				AND filler.event_ref = event.event_ref) AS relation_filler_count,
		(SELECT COUNT(*) FROM typed_memory_ordered_candidate_prefixes prefix
			WHERE prefix.project_id = event.project_id
				AND prefix.event_ref = event.event_ref) AS ordered_candidate_prefix_count,
		(SELECT COUNT(*) FROM typed_memory_reference_resolution_uses resolution_use
			WHERE resolution_use.project_id = event.project_id
				AND resolution_use.event_ref = event.event_ref) AS reference_resolution_use_count,
		(SELECT COUNT(*) FROM typed_memory_memberof_evaluations evaluation
			WHERE evaluation.project_id = event.project_id
				AND evaluation.event_ref = event.event_ref) AS memberof_evaluation_count,
		(SELECT COUNT(*) FROM typed_memory_memberof_observable_inputs evaluation_input
			WHERE evaluation_input.project_id = event.project_id
				AND evaluation_input.event_ref = event.event_ref) AS memberof_input_count,
		(SELECT COUNT(*) FROM typed_memory_relation_filler_memberof_uses member_use
			WHERE member_use.project_id = event.project_id
				AND member_use.event_ref = event.event_ref) AS memberof_use_count,
		(SELECT COUNT(*) FROM typed_memory_alias_changes alias_change
			WHERE alias_change.project_id = event.project_id
				AND alias_change.event_ref = event.event_ref) AS alias_change_count,
		(SELECT COUNT(*) FROM typed_memory_assertion_retractions retraction
			WHERE retraction.project_id = event.project_id
				AND retraction.event_ref = event.event_ref) AS retraction_count,
		(SELECT COUNT(*) FROM typed_memory_entity_declarations declaration
			WHERE declaration.project_id = event.project_id
				AND declaration.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_alias_changes alias_change
			WHERE alias_change.project_id = event.project_id
				AND alias_change.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_relation_instances relation
			WHERE relation.project_id = event.project_id
				AND relation.event_ref = event.event_ref)
		+ (SELECT COUNT(*) FROM typed_memory_assertion_retractions retraction
			WHERE retraction.project_id = event.project_id
				AND retraction.event_ref = event.event_ref) AS top_level_change_count
	FROM typed_memory_graph_events event`
}

func typedMemoryStorageCapabilityMarkerInsert46() string {
	return `INSERT INTO typed_memory_storage_capabilities (
		capability_key, writer_generation, capability_digest,
		canonical_bytes, installed_at
	) VALUES (
		'typed_memory_writer_generation',
		46,
		'sha256:ef94ecbd2590981b016eb89ba3f1c4f9a101ebbbf779355e6c3f7e13a5cdd58a',
		CAST('haft.typed-memory.storage.writer-generation=46' AS BLOB),
		strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
	)`
}

func typedMemoryLegacyWriterGenerationBackfill46() string {
	return `INSERT INTO typed_memory_event_writer_generations (
		project_id, event_ref, writer_generation, provenance_kind
	)
	SELECT project_id, event_ref, 45, 'migration_v45_backfill'
	FROM typed_memory_graph_events`
}

func typedMemorySHA256Shape46(column string) string {
	return "length(" + column + ") = 71 AND substr(" + column + ", 1, 7) = 'sha256:'"
}

func typedMemoryNonBlankShape46(column string) string {
	return column + " != '' AND trim(" + column + ") = " + column
}

func typedMemoryStorageTriggerStatements46() []string {
	statements := []string{
		typedMemoryStorageCapabilityExactMarkerTrigger46(),
		typedMemoryEventWriterGenerationExactBoundaryTrigger46(),
		typedMemoryEventAdmissionExactEventTrigger46(),
		typedMemoryEntityDeclarationExactMaterializationTrigger46(),
		typedMemoryRelationInstanceExactContextSliceTrigger46(),
		typedMemoryRelationSlotExactRelationTrigger46(),
		typedMemoryRelationFillerExactSlotTrigger46(),
		typedMemoryReferenceResolutionExactFillerTrigger46(),
		typedMemoryMemberOfEvaluationExactContextSliceTrigger46(),
		typedMemoryMemberOfInputExactEvaluationTrigger46(),
		typedMemoryRelationFillerMemberOfExactUseTrigger46(),
		typedMemoryAliasExactSupersessionTrigger46(),
		typedMemoryCommitClosureBasisKindTrigger46(),
		typedMemoryCommitClosureExactFootprintTrigger46(),
	}
	for _, table := range typedMemoryStorageTables46 {
		statements = append(statements, immutableTypedMemoryTrigger46(table, "update"))
		statements = append(statements, immutableTypedMemoryTrigger46(table, "delete"))
		if table == "typed_memory_storage_capabilities" {
			continue
		}
		statements = append(statements, typedMemoryOpenEventTrigger46(table))
	}
	statements = append(statements,
		"DROP TRIGGER typed_memory_entities_exact_event",
		typedMemoryEntityExactEventTrigger46(),
		"DROP TRIGGER typed_memory_entity_contexts_exact_event",
		typedMemoryEntityContextExactEventTrigger46(),
		"DROP TRIGGER typed_memory_graph_commits_exact_closure",
		typedMemoryGraphCommitExactClosureTrigger46(),
	)
	return statements
}

func typedMemoryEventWriterGenerationExactBoundaryTrigger46() string {
	return `CREATE TRIGGER typed_memory_event_writer_generations_v46_exact_boundary
	BEFORE INSERT ON typed_memory_event_writer_generations
	WHEN (NEW.writer_generation = 45 AND EXISTS (
		SELECT 1 FROM typed_memory_storage_capabilities
		WHERE capability_key = 'typed_memory_writer_generation'
	)) OR (NEW.writer_generation = 46 AND NOT EXISTS (
		SELECT 1 FROM typed_memory_storage_capabilities
		WHERE capability_key = 'typed_memory_writer_generation'
	))
	BEGIN
		SELECT RAISE(ABORT, 'typed-memory event writer generation crosses its sealed migration boundary');
	END`
}

func typedMemoryStorageCapabilityExactMarkerTrigger46() string {
	return `CREATE TRIGGER typed_memory_storage_capabilities_v46_exact_marker
	BEFORE INSERT ON typed_memory_storage_capabilities
	WHEN EXISTS (SELECT 1 FROM typed_memory_storage_capabilities)
		OR NEW.capability_key != 'typed_memory_writer_generation'
		OR NEW.writer_generation != 46
		OR NEW.capability_digest != 'sha256:ef94ecbd2590981b016eb89ba3f1c4f9a101ebbbf779355e6c3f7e13a5cdd58a'
		OR CAST(NEW.canonical_bytes AS TEXT) != 'haft.typed-memory.storage.writer-generation=46'
	BEGIN
		SELECT RAISE(ABORT, 'typed-memory writer-generation capability is immutable and exact');
	END`
}

func typedMemoryOpenEventTrigger46(table string) string {
	return `CREATE TRIGGER ` + table + `_v46_open_event
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
		SELECT RAISE(ABORT, 'typed-memory v46 materialization requires its exact open event');
	END`
}

func immutableTypedMemoryTrigger46(table string, operation string) string {
	return `CREATE TRIGGER ` + table + `_v46_no_` + operation + `
	BEFORE ` + operation + ` ON ` + table + ` BEGIN
		SELECT RAISE(ABORT, 'typed-memory v46 history is append-only');
	END`
}

func typedMemoryEventAdmissionExactEventTrigger46() string {
	return `CREATE TRIGGER typed_memory_event_admission_bases_v46_exact_event
	BEFORE INSERT ON typed_memory_event_admission_bases
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_graph_events event
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND event.event_digest = NEW.event_digest
			AND event.basis_type_env_ref = NEW.type_env_ref
			AND event.expected_revision = NEW.basis_graph_revision
			AND event.change_set_digest = NEW.semantic_digest
			AND event.canonical_change_set_bytes = NEW.canonical_semantic_bytes
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory admission basis does not bind its exact event and semantic bytes');
	END`
}

func typedMemoryEntityDeclarationExactMaterializationTrigger46() string {
	return `CREATE TRIGGER typed_memory_entity_declarations_v46_exact_materialization
	BEFORE INSERT ON typed_memory_entity_declarations
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_entity_contexts context
			ON context.project_id = event.project_id
			AND context.entity_id = NEW.entity_id
			AND context.bounded_context_ref = NEW.bounded_context_ref
			AND context.label = NEW.label
			AND context.provenance_ref = NEW.provenance_ref
			AND context.declared_event_ref = event.event_ref
			AND context.declared_revision = event.graph_revision
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND NEW.change_ordinal < event.change_count
			AND event.event_kind IN ('declare_entity', 'mixed_change_set')
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_graph_commits commit_record
				WHERE commit_record.project_id = event.project_id
					AND commit_record.event_ref = event.event_ref
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory entity declaration lacks its exact same-event context materialization');
	END`
}

func typedMemoryRelationInstanceExactContextSliceTrigger46() string {
	return `CREATE TRIGGER typed_memory_relation_instances_v46_exact_context_slice
	BEFORE INSERT ON typed_memory_relation_instances
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_context_slices slice
		WHERE slice.project_id = NEW.project_id
			AND slice.event_ref = NEW.event_ref
			AND slice.context_slice_ref = NEW.context_slice_ref
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory relation lacks its exact ContextSlice bytes');
	END`
}

func typedMemoryRelationSlotExactRelationTrigger46() string {
	return `CREATE TRIGGER typed_memory_relation_slots_v46_exact_relation
	BEFORE INSERT ON typed_memory_relation_slots
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_relation_instances relation
		WHERE relation.project_id = NEW.project_id
			AND relation.event_ref = NEW.event_ref
			AND relation.change_ordinal = NEW.change_ordinal
			AND relation.assertion_id = NEW.assertion_id
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory relation slot lacks its exact relation coordinate');
	END`
}

func typedMemoryRelationFillerExactSlotTrigger46() string {
	return `CREATE TRIGGER typed_memory_relation_fillers_v46_exact_slot
	BEFORE INSERT ON typed_memory_relation_fillers
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_relation_slots slot
		WHERE slot.project_id = NEW.project_id
			AND slot.event_ref = NEW.event_ref
			AND slot.change_ordinal = NEW.change_ordinal
			AND slot.assertion_id = NEW.assertion_id
			AND slot.slot_ordinal = NEW.slot_ordinal
	) OR (
		NEW.filler_kind = 'by_value'
		AND NOT EXISTS (
			SELECT 1 FROM typed_memory_value_blobs value_blob
			WHERE value_blob.project_id = NEW.project_id
				AND value_blob.event_ref = NEW.event_ref
				AND value_blob.value_ref = NEW.value_ref
		)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory relation filler lacks its exact slot or canonical value blob');
	END`
}

func typedMemoryReferenceResolutionExactFillerTrigger46() string {
	return `CREATE TRIGGER typed_memory_reference_resolution_uses_v46_exact_filler
	BEFORE INSERT ON typed_memory_reference_resolution_uses
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_relation_fillers filler
		WHERE filler.project_id = NEW.project_id
			AND filler.event_ref = NEW.event_ref
			AND filler.change_ordinal = NEW.change_ordinal
			AND filler.assertion_id = NEW.assertion_id
			AND filler.slot_ordinal = NEW.slot_ordinal
			AND filler.filler_ordinal = NEW.filler_ordinal
			AND filler.filler_digest = NEW.filler_digest
			AND filler.filler_kind = 'by_reference'
			AND filler.entity_id = NEW.entity_id
			AND (
				NEW.resolution_kind = 'snapshot_reference'
				OR (
					filler.reference_kind_ref = NEW.local_reference_kind_ref
					AND filler.reference_id = NEW.entity_id
				)
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory reference-resolution use does not bind its exact reference filler');
	END`
}

func typedMemoryMemberOfEvaluationExactContextSliceTrigger46() string {
	return `CREATE TRIGGER typed_memory_memberof_evaluations_v46_exact_context_slice
	BEFORE INSERT ON typed_memory_memberof_evaluations
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_context_slices slice
		WHERE slice.project_id = NEW.project_id
			AND slice.event_ref = NEW.event_ref
			AND slice.context_slice_ref = NEW.context_slice_ref
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory MemberOf evaluation lacks its exact ContextSlice bytes');
	END`
}

func typedMemoryMemberOfInputExactEvaluationTrigger46() string {
	return `CREATE TRIGGER typed_memory_memberof_observable_inputs_v46_exact_evaluation_input
	BEFORE INSERT ON typed_memory_memberof_observable_inputs
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_memberof_evaluations evaluation
		JOIN typed_memory_observable_input_blobs observable_blob
			ON observable_blob.project_id = evaluation.project_id
			AND observable_blob.event_ref = evaluation.event_ref
		WHERE evaluation.project_id = NEW.project_id
			AND evaluation.event_ref = NEW.event_ref
			AND evaluation.evaluation_ref = NEW.evaluation_ref
			AND observable_blob.observable_input_ref = NEW.observable_input_ref
			AND observable_blob.observable_input_digest = NEW.observable_input_digest
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory MemberOf input lacks its exact evaluation or observable bytes');
	END`
}

func typedMemoryRelationFillerMemberOfExactUseTrigger46() string {
	return `CREATE TRIGGER typed_memory_relation_filler_memberof_uses_v46_exact_use
	BEFORE INSERT ON typed_memory_relation_filler_memberof_uses
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_relation_fillers filler
		JOIN typed_memory_relation_instances relation
			ON relation.project_id = filler.project_id
			AND relation.event_ref = filler.event_ref
			AND relation.change_ordinal = filler.change_ordinal
			AND relation.assertion_id = filler.assertion_id
		JOIN typed_memory_reference_resolution_uses resolution_use
			ON resolution_use.project_id = filler.project_id
			AND resolution_use.event_ref = filler.event_ref
			AND resolution_use.change_ordinal = filler.change_ordinal
			AND resolution_use.assertion_id = filler.assertion_id
			AND resolution_use.slot_ordinal = filler.slot_ordinal
			AND resolution_use.filler_ordinal = filler.filler_ordinal
			AND resolution_use.filler_digest = filler.filler_digest
		JOIN typed_memory_memberof_evaluations evaluation
			ON evaluation.project_id = filler.project_id
			AND evaluation.event_ref = filler.event_ref
		WHERE filler.project_id = NEW.project_id
			AND filler.event_ref = NEW.event_ref
			AND filler.change_ordinal = NEW.change_ordinal
			AND filler.assertion_id = NEW.assertion_id
			AND filler.slot_ordinal = NEW.slot_ordinal
			AND filler.filler_ordinal = NEW.filler_ordinal
			AND filler.filler_digest = NEW.filler_digest
			AND filler.filler_kind = 'by_reference'
			AND evaluation.evaluation_ref = NEW.evaluation_ref
			AND evaluation.entity_id = filler.entity_id
			AND evaluation.value_kind_ref = NEW.queried_value_kind_ref
			AND evaluation.query_digest = NEW.query_digest
			AND evaluation.judgement_kind = NEW.expected_judgement_kind
			AND evaluation.context_slice_ref = relation.context_slice_ref
			AND (
				(resolution_use.resolution_kind = 'snapshot_reference'
					AND evaluation.evaluation_view_kind = 'persisted_snapshot')
				OR (resolution_use.resolution_kind = 'same_batch_declaration'
					AND evaluation.evaluation_view_kind = 'prospective_batch'
					AND evaluation.view_declaration_change_ordinal = resolution_use.declaration_change_ordinal
					AND evaluation.view_local_reference_kind_ref = resolution_use.local_reference_kind_ref
					AND evaluation.view_batch_local_ref = resolution_use.batch_local_ref
					AND evaluation.view_declaration_digest = resolution_use.declaration_digest
					AND evaluation.view_prefix_end_ordinal = filler.change_ordinal
					AND evaluation.view_ordered_candidate_prefix_digest = resolution_use.ordered_candidate_prefix_digest)
			)
			AND (
				(NEW.use_kind = 'required_member'
					AND NEW.queried_value_kind_ref = filler.required_value_kind_ref)
				OR (NEW.use_kind = 'disjoint_not_member'
					AND NEW.queried_value_kind_ref != filler.required_value_kind_ref)
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory MemberOf use does not bind its exact filler, query, view, and defined judgement');
	END`
}

func typedMemoryAliasExactSupersessionTrigger46() string {
	return `CREATE TRIGGER typed_memory_alias_changes_v46_exact_supersession
	BEFORE INSERT ON typed_memory_alias_changes
	WHEN NEW.change_kind = 'supersede_alias'
		AND NOT EXISTS (
			SELECT 1 FROM typed_memory_alias_changes previous
			WHERE previous.project_id = NEW.project_id
				AND previous.alias_change_ref = NEW.supersedes_alias_change_ref
				AND previous.bounded_context_ref = NEW.bounded_context_ref
				AND previous.entity_id = NEW.entity_id
				AND CASE previous.change_kind
					WHEN 'admit_alias' THEN previous.alias
					ELSE previous.replacement_alias
				END = NEW.alias
		) BEGIN
		SELECT RAISE(ABORT, 'typed-memory alias supersession does not bind the exact prior alias change');
	END`
}

func typedMemoryCommitClosureBasisKindTrigger46() string {
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
					AND NEW.memberof_use_count = 0)
				OR (admission.admission_basis_kind = 'context_slice_membership'
					AND NEW.context_slice_count >= 1
					AND NEW.reference_resolution_use_count >= 1
					AND NEW.observable_input_blob_count >= 1
					AND NEW.memberof_evaluation_count >= 1
					AND NEW.memberof_input_count >= 1
					AND NEW.memberof_use_count >= 1)
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory materialization closure does not match its sealed admission-basis kind');
	END`
}

func typedMemoryCommitClosureExactFootprintTrigger46() string {
	return `CREATE TRIGGER typed_memory_commit_materialization_closures_v46_exact_footprint
	BEFORE INSERT ON typed_memory_commit_materialization_closures
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_event_admission_bases admission
			ON admission.project_id = event.project_id
			AND admission.event_ref = event.event_ref
		JOIN typed_memory_event_materialization_footprints_v46 footprint
			ON footprint.project_id = event.project_id
			AND footprint.event_ref = event.event_ref
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND event.commit_ref = NEW.commit_ref
			AND event.event_digest = NEW.event_digest
			AND event.change_set_digest = NEW.semantic_digest
			AND admission.admission_basis_kind = NEW.admission_basis_kind
			AND admission.request_digest = NEW.request_digest
			AND admission.admission_envelope_digest = NEW.admission_envelope_digest
			AND admission.admission_basis_digest = NEW.admission_basis_digest
			AND event.change_count = footprint.top_level_change_count
			AND NEW.entity_count = footprint.entity_count
			AND NEW.entity_context_count = footprint.entity_context_count
			AND NEW.entity_declaration_count = footprint.entity_declaration_count
			AND NEW.context_slice_catalog_count = footprint.context_slice_catalog_count
			AND NEW.context_slice_count = footprint.context_slice_count
			AND NEW.value_blob_count = footprint.value_blob_count
			AND NEW.observable_input_blob_count = footprint.observable_input_blob_count
			AND NEW.relation_count = footprint.relation_count
			AND NEW.relation_slot_count = footprint.relation_slot_count
			AND NEW.relation_filler_count = footprint.relation_filler_count
			AND NEW.ordered_candidate_prefix_count = footprint.ordered_candidate_prefix_count
			AND NEW.reference_resolution_use_count = footprint.reference_resolution_use_count
			AND NEW.memberof_evaluation_count = footprint.memberof_evaluation_count
			AND NEW.memberof_input_count = footprint.memberof_input_count
			AND NEW.memberof_use_count = footprint.memberof_use_count
			AND NEW.alias_change_count = footprint.alias_change_count
			AND NEW.retraction_count = footprint.retraction_count
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_entities entity
				WHERE entity.project_id = NEW.project_id
					AND entity.first_declared_event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_entity_declarations declaration
						WHERE declaration.project_id = entity.project_id
							AND declaration.event_ref = entity.first_declared_event_ref
							AND declaration.entity_id = entity.entity_id
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_entity_contexts context
				WHERE context.project_id = NEW.project_id
					AND context.declared_event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_entity_declarations declaration
						WHERE declaration.project_id = context.project_id
							AND declaration.event_ref = context.declared_event_ref
							AND declaration.entity_id = context.entity_id
							AND declaration.bounded_context_ref = context.bounded_context_ref
							AND declaration.label = context.label
							AND declaration.provenance_ref = context.provenance_ref
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_entity_declarations declaration
				WHERE declaration.project_id = NEW.project_id
					AND declaration.event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_entity_contexts context
						WHERE context.project_id = declaration.project_id
							AND context.declared_event_ref = declaration.event_ref
							AND context.entity_id = declaration.entity_id
							AND context.bounded_context_ref = declaration.bounded_context_ref
							AND context.label = declaration.label
							AND context.provenance_ref = declaration.provenance_ref
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_context_slice_catalog catalog
				WHERE catalog.project_id = NEW.project_id
					AND catalog.event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_context_slices slice
						WHERE slice.project_id = catalog.project_id
							AND slice.event_ref = catalog.event_ref
							AND slice.context_slice_ref = catalog.context_slice_ref
							AND slice.context_slice_digest = catalog.context_slice_digest
							AND slice.bounded_context_ref = catalog.bounded_context_ref
							AND slice.canonical_context_slice_bytes = catalog.canonical_context_slice_bytes
					)
			)
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
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_relation_instances relation
				WHERE relation.project_id = NEW.project_id
					AND relation.event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_relation_slots slot
						WHERE slot.project_id = relation.project_id
							AND slot.event_ref = relation.event_ref
							AND slot.change_ordinal = relation.change_ordinal
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_relation_slots slot
				WHERE slot.project_id = NEW.project_id
					AND slot.event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_relation_fillers filler
						WHERE filler.project_id = slot.project_id
							AND filler.event_ref = slot.event_ref
							AND filler.change_ordinal = slot.change_ordinal
							AND filler.slot_ordinal = slot.slot_ordinal
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_relation_fillers filler
				WHERE filler.project_id = NEW.project_id
					AND filler.event_ref = NEW.event_ref
					AND filler.filler_kind = 'by_reference'
					AND (
						NOT EXISTS (
							SELECT 1 FROM typed_memory_reference_resolution_uses resolution_use
							WHERE resolution_use.project_id = filler.project_id
								AND resolution_use.event_ref = filler.event_ref
								AND resolution_use.change_ordinal = filler.change_ordinal
								AND resolution_use.slot_ordinal = filler.slot_ordinal
								AND resolution_use.filler_ordinal = filler.filler_ordinal
						)
						OR NOT EXISTS (
							SELECT 1 FROM typed_memory_relation_filler_memberof_uses member_use
							WHERE member_use.project_id = filler.project_id
								AND member_use.event_ref = filler.event_ref
								AND member_use.change_ordinal = filler.change_ordinal
								AND member_use.slot_ordinal = filler.slot_ordinal
								AND member_use.filler_ordinal = filler.filler_ordinal
								AND member_use.use_kind = 'required_member'
						)
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_memberof_evaluations evaluation
				WHERE evaluation.project_id = NEW.project_id
					AND evaluation.event_ref = NEW.event_ref
					AND (
						(SELECT COUNT(*) FROM typed_memory_memberof_observable_inputs evaluation_input
							WHERE evaluation_input.project_id = evaluation.project_id
								AND evaluation_input.event_ref = evaluation.event_ref
								AND evaluation_input.evaluation_ref = evaluation.evaluation_ref)
							!= evaluation.observable_input_count
						OR (SELECT MIN(evaluation_input.input_ordinal)
							FROM typed_memory_memberof_observable_inputs evaluation_input
							WHERE evaluation_input.project_id = evaluation.project_id
								AND evaluation_input.event_ref = evaluation.event_ref
								AND evaluation_input.evaluation_ref = evaluation.evaluation_ref) != 0
						OR (SELECT MAX(evaluation_input.input_ordinal)
							FROM typed_memory_memberof_observable_inputs evaluation_input
							WHERE evaluation_input.project_id = evaluation.project_id
								AND evaluation_input.event_ref = evaluation.event_ref
								AND evaluation_input.evaluation_ref = evaluation.evaluation_ref)
							!= evaluation.observable_input_count - 1
					)
			)
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_observable_input_blobs observable_blob
				WHERE observable_blob.project_id = NEW.project_id
					AND observable_blob.event_ref = NEW.event_ref
					AND NOT EXISTS (
						SELECT 1 FROM typed_memory_memberof_observable_inputs evaluation_input
						WHERE evaluation_input.project_id = observable_blob.project_id
							AND evaluation_input.event_ref = observable_blob.event_ref
							AND evaluation_input.observable_input_ref = observable_blob.observable_input_ref
							AND evaluation_input.observable_input_digest = observable_blob.observable_input_digest
					)
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory materialization closure does not match the exact complete event footprint');
	END`
}

func typedMemoryEntityExactEventTrigger46() string {
	return `CREATE TRIGGER typed_memory_entities_exact_event
	BEFORE INSERT ON typed_memory_entities
	WHEN NOT EXISTS (
		SELECT 1 FROM typed_memory_graph_events event
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.first_declared_event_ref
			AND event.graph_revision = NEW.first_declared_revision
			AND event.event_kind IN ('declare_entity', 'mixed_change_set')
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_graph_commits commit_record
				WHERE commit_record.project_id = event.project_id
					AND commit_record.event_ref = event.event_ref
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory entity does not match its declaration event');
	END`
}

func typedMemoryEntityContextExactEventTrigger46() string {
	return `CREATE TRIGGER typed_memory_entity_contexts_exact_event
	BEFORE INSERT ON typed_memory_entity_contexts
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events current_event
		JOIN typed_memory_entities entity
			ON entity.project_id = current_event.project_id
			AND entity.entity_id = NEW.entity_id
		WHERE current_event.project_id = NEW.project_id
			AND current_event.event_ref = NEW.declared_event_ref
			AND current_event.graph_revision = NEW.declared_revision
			AND current_event.event_kind IN ('declare_entity', 'mixed_change_set')
			AND NOT EXISTS (
				SELECT 1 FROM typed_memory_graph_commits commit_record
				WHERE commit_record.project_id = current_event.project_id
					AND commit_record.event_ref = current_event.event_ref
			)
			AND (
				(entity.first_declared_event_ref = current_event.event_ref
					AND entity.first_declared_revision = current_event.graph_revision)
				OR EXISTS (
					SELECT 1
					FROM typed_memory_graph_events declaration_event
					JOIN typed_memory_graph_commits declaration_commit
						ON declaration_commit.project_id = declaration_event.project_id
						AND declaration_commit.event_ref = declaration_event.event_ref
					WHERE declaration_event.project_id = entity.project_id
						AND declaration_event.event_ref = entity.first_declared_event_ref
						AND declaration_event.graph_revision = entity.first_declared_revision
						AND declaration_event.graph_revision <= current_event.expected_revision
						AND declaration_commit.graph_revision = declaration_event.graph_revision
				)
			)
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory entity context does not match its declaration event');
	END`
}

func typedMemoryGraphCommitExactClosureTrigger46() string {
	return `CREATE TRIGGER typed_memory_graph_commits_exact_closure
	BEFORE INSERT ON typed_memory_graph_commits
	WHEN NOT EXISTS (
		SELECT 1
		FROM typed_memory_graph_events event
		JOIN typed_memory_event_writer_generations generation
			ON generation.project_id = event.project_id
			AND generation.event_ref = event.event_ref
		JOIN typed_memory_idempotency_history idempotency
			ON idempotency.project_id = event.project_id
			AND idempotency.event_ref = event.event_ref
		JOIN typed_memory_projection_jobs projection_job
			ON projection_job.project_id = event.project_id
			AND projection_job.semantic_event_ref = event.event_ref
		JOIN typed_memory_event_admission_bases admission
			ON admission.project_id = event.project_id
			AND admission.event_ref = event.event_ref
		JOIN typed_memory_commit_materialization_closures closure
			ON closure.project_id = event.project_id
			AND closure.event_ref = event.event_ref
		JOIN typed_memory_event_materialization_footprints_v46 footprint
			ON footprint.project_id = event.project_id
			AND footprint.event_ref = event.event_ref
		WHERE event.project_id = NEW.project_id
			AND event.event_ref = NEW.event_ref
			AND event.commit_ref = NEW.commit_ref
			AND event.event_digest = NEW.event_digest
			AND event.expected_revision = NEW.expected_revision
			AND event.graph_revision = NEW.graph_revision
			AND event.change_set_digest = NEW.change_set_digest
			AND generation.writer_generation = 46
			AND generation.provenance_kind = 'writer_v46'
			AND event.authority_class IN (
				'non_binding_semantic_assertion', 'manual_type_env_activation'
			)
			AND idempotency.idempotency_key = NEW.idempotency_key
			AND idempotency.change_set_digest = NEW.change_set_digest
			AND idempotency.graph_revision = NEW.graph_revision
			AND idempotency.result_digest = NEW.event_digest
			AND projection_job.projection_job_ref = NEW.projection_job_ref
			AND projection_job.graph_revision = NEW.graph_revision
			AND projection_job.input_event_digest = NEW.event_digest
			AND admission.event_digest = NEW.event_digest
			AND admission.semantic_digest = NEW.change_set_digest
			AND closure.commit_ref = NEW.commit_ref
			AND closure.event_digest = NEW.event_digest
			AND closure.admission_basis_kind = admission.admission_basis_kind
			AND closure.request_digest = admission.request_digest
			AND closure.semantic_digest = admission.semantic_digest
			AND closure.admission_envelope_digest = admission.admission_envelope_digest
			AND closure.admission_basis_digest = admission.admission_basis_digest
			AND closure.materialization_manifest_digest = admission.materialization_manifest_digest
			AND event.change_count = footprint.top_level_change_count
			AND NEW.entity_count = footprint.entity_count
			AND NEW.entity_context_count = footprint.entity_context_count
			AND closure.entity_count = footprint.entity_count
			AND closure.entity_context_count = footprint.entity_context_count
			AND closure.entity_declaration_count = footprint.entity_declaration_count
			AND closure.context_slice_catalog_count = footprint.context_slice_catalog_count
			AND closure.context_slice_count = footprint.context_slice_count
			AND closure.value_blob_count = footprint.value_blob_count
			AND closure.observable_input_blob_count = footprint.observable_input_blob_count
			AND closure.relation_count = footprint.relation_count
			AND closure.relation_slot_count = footprint.relation_slot_count
			AND closure.relation_filler_count = footprint.relation_filler_count
			AND closure.ordered_candidate_prefix_count = footprint.ordered_candidate_prefix_count
			AND closure.reference_resolution_use_count = footprint.reference_resolution_use_count
			AND closure.memberof_evaluation_count = footprint.memberof_evaluation_count
			AND closure.memberof_input_count = footprint.memberof_input_count
			AND closure.memberof_use_count = footprint.memberof_use_count
			AND closure.alias_change_count = footprint.alias_change_count
			AND closure.retraction_count = footprint.retraction_count
	) BEGIN
		SELECT RAISE(ABORT, 'typed-memory graph commit requires its exact v46 admission and materialization closure');
	END`
}
