package db

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"
)

const typedMemoryRecordedAt46 = "2026-07-16T14:00:00Z"

type typedMemoryAdmissionFixture46 struct {
	event                    typedMemoryDeclarationFixture45
	requestDigest            string
	semanticDigest           string
	envelopeDigest           string
	basisDigest              string
	manifestDigest           string
	materializationDigest    string
	canonicalRequest         []byte
	canonicalSemantic        []byte
	canonicalEnvelope        []byte
	canonicalBasis           []byte
	canonicalManifest        []byte
	canonicalMaterialization []byte
}

type typedMemoryClosureCounts46 struct {
	entity                 int
	entityContext          int
	entityDeclaration      int
	contextSliceCatalog    int
	contextSlice           int
	valueBlob              int
	observableInputBlob    int
	relation               int
	relationSlot           int
	relationFiller         int
	orderedCandidatePrefix int
	referenceResolutionUse int
	memberOfEvaluation     int
	memberOfInput          int
	memberOfUse            int
	aliasChange            int
	retraction             int
}

func TestTypedMemoryStorageMigration46InstallsExactSchemaAndPreservesV45Objects(t *testing.T) {
	t.Parallel()

	database := openDatabaseBeforeTypedMemoryStorageMigration46(t)
	defer database.Close()
	before := typedMemorySchemaObjects46(t, database)

	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryStorageMigration46},
	); err != nil {
		t.Fatalf("migrate v45 database through v46: %v", err)
	}

	assertMigrationVersionPresent(t, database, 45)
	assertMigrationVersionPresent(t, database, 46)
	for _, table := range typedMemoryStorageTables46 {
		assertSQLiteObjectExists(t, database, "table", table)
	}
	for _, view := range typedMemoryStorageViews46 {
		assertSQLiteObjectExists(t, database, "view", view)
	}
	for _, index := range typedMemoryStorageIndexes46 {
		assertSQLiteObjectExists(t, database, "index", index)
	}
	for _, trigger := range typedMemoryStorageSpecificTriggers46 {
		assertSQLiteObjectExists(t, database, "trigger", trigger)
	}

	after := typedMemorySchemaObjects46(t, database)
	superseded := map[string]bool{
		"trigger/typed_memory_entities_exact_event":        true,
		"trigger/typed_memory_entity_contexts_exact_event": true,
		"trigger/typed_memory_graph_commits_exact_closure": true,
	}
	for key, sqlText := range before {
		if superseded[key] {
			if after[key] == "" || after[key] == sqlText {
				t.Fatalf("v46 did not explicitly supersede %s", key)
			}
			continue
		}
		if after[key] != sqlText {
			t.Fatalf("v46 changed preserved v45 object %s", key)
		}
	}

	var generation int
	var digest string
	var canonical []byte
	err := database.QueryRow(`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_storage_capabilities WHERE capability_key = ?`,
		typedMemoryWriterGenerationCapability46,
	).Scan(&generation, &digest, &canonical)
	if err != nil {
		t.Fatalf("read v46 writer-generation capability: %v", err)
	}
	if generation != typedMemoryWriterGeneration46 ||
		digest != typedMemoryWriterMarkerDigest46 ||
		string(canonical) != typedMemoryWriterMarkerBytes46 {
		t.Fatalf("writer-generation capability = %d %q %q", generation, digest, canonical)
	}
	_, updateErr := database.Exec(`UPDATE typed_memory_storage_capabilities
		SET writer_generation = 45 WHERE capability_key = ?`,
		typedMemoryWriterGenerationCapability46,
	)
	if updateErr == nil || !strings.Contains(updateErr.Error(), "append-only") {
		t.Fatalf("mutable writer-generation marker error = %v", updateErr)
	}
	_, deleteErr := database.Exec(`DELETE FROM typed_memory_storage_capabilities
		WHERE capability_key = ?`,
		typedMemoryWriterGenerationCapability46,
	)
	if deleteErr == nil || !strings.Contains(deleteErr.Error(), "append-only") {
		t.Fatalf("deletable writer-generation marker error = %v", deleteErr)
	}
	columns := typedMemoryStorageColumns46(t, database)
	assertTypedMemoryStorageColumnPresent46(t, columns, "typed_memory_value_blobs", "value_shape_ref")
	assertTypedMemoryStorageColumnPresent46(t, columns, "typed_memory_value_blobs", "codec_ref")
	assertTypedMemoryStorageColumnPresent46(t, columns, "typed_memory_entity_declarations", "batch_local_ref")
	assertTypedMemoryStorageColumnPresent46(t, columns, "typed_memory_ordered_candidate_prefixes", "prefix_digest")
	assertTypedMemoryStorageColumnPresent46(t, columns, "typed_memory_relation_fillers", "reference_kind_ref")
	assertTypedMemoryStorageColumnPresent46(t, columns, "typed_memory_relation_fillers", "reference_id")
	assertTypedMemoryStorageColumnAbsent46(t, columns, "typed_memory_relation_fillers", "subject_ref")
	assertTypedMemoryStorageColumnPresent46(t, columns, "typed_memory_reference_resolution_uses", "declaration_change_ordinal")
	assertTypedMemoryStorageColumnPresent46(t, columns, "typed_memory_reference_resolution_uses", "ordered_candidate_prefix_digest")
	assertTypedMemoryStorageColumnPresent46(t, columns, "typed_memory_memberof_evaluations", "observable_input_set_digest")
	assertTypedMemoryStorageColumnPresent46(t, columns, "typed_memory_alias_changes", "replacement_alias")
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestTypedMemoryStorageMigration46PreservesV45HistoryWithoutFabricatedBasis(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, false)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryDeclarationFixture45("historical-v45", "1", "2", typeEnvRef, 0)
	commitTypedMemoryDeclaration45(t, database, fixture)

	var eventBytesBefore []byte
	var eventDigestBefore string
	err := database.QueryRow(`SELECT canonical_change_set_bytes, event_digest
		FROM typed_memory_graph_events WHERE project_id = ? AND event_ref = ?`,
		fixture.projectID,
		fixture.eventRef,
	).Scan(&eventBytesBefore, &eventDigestBefore)
	if err != nil {
		t.Fatalf("read v45 event before v46: %v", err)
	}

	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryStorageMigration46},
	); err != nil {
		t.Fatalf("migrate historical v45 graph through v46: %v", err)
	}

	var eventBytesAfter []byte
	var eventDigestAfter string
	err = database.QueryRow(`SELECT canonical_change_set_bytes, event_digest
		FROM typed_memory_graph_events WHERE project_id = ? AND event_ref = ?`,
		fixture.projectID,
		fixture.eventRef,
	).Scan(&eventBytesAfter, &eventDigestAfter)
	if err != nil {
		t.Fatalf("read preserved v45 event after v46: %v", err)
	}
	if !bytes.Equal(eventBytesAfter, eventBytesBefore) || eventDigestAfter != eventDigestBefore {
		t.Fatal("v46 changed historical v45 event bytes or digest")
	}
	assertTypedMemoryEventCompanionCount46(
		t,
		database,
		"typed_memory_event_admission_bases",
		fixture.eventRef,
		0,
	)
	assertTypedMemoryEventCompanionCount46(
		t,
		database,
		"typed_memory_commit_materialization_closures",
		fixture.eventRef,
		0,
	)
	var generation int
	var provenance string
	err = database.QueryRow(`SELECT writer_generation, provenance_kind
		FROM typed_memory_event_writer_generations
		WHERE project_id = ? AND event_ref = ?`,
		fixture.projectID,
		fixture.eventRef,
	).Scan(&generation, &provenance)
	if err != nil {
		t.Fatalf("read preserved v45 writer generation: %v", err)
	}
	if generation != 45 || provenance != "migration_v45_backfill" {
		t.Fatalf(
			"preserved v45 writer generation = %d %q; want migration backfill",
			generation,
			provenance,
		)
	}
	assertTypedMemoryHeadRevision45(t, database, 1)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestTypedMemoryStorageMigration46RejectsPostCapabilityV45Generation(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryDeclarationFixture45(
		"fabricated-post-capability-v45",
		"5",
		"6",
		typeEnvRef,
		0,
	)
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin fabricated v45 generation transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture, 1)
	_, insertErr := transaction.Exec(`INSERT INTO typed_memory_event_writer_generations (
		project_id, event_ref, writer_generation, provenance_kind
	) VALUES (?, ?, 45, 'migration_v45_backfill')`,
		fixture.projectID,
		fixture.eventRef,
	)
	if insertErr == nil || !strings.Contains(insertErr.Error(), "sealed migration boundary") {
		_ = transaction.Rollback()
		t.Fatalf("fabricated post-capability v45 generation error = %v", insertErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back fabricated v45 generation transaction: %v", err)
	}
}

func TestTypedMemoryStorageMigration46RequiresCompleteV45Source(t *testing.T) {
	t.Parallel()

	database := openDatabaseBeforeTypedMemoryStorageMigration45(t)
	defer database.Close()

	err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryStorageMigration46},
	)
	if err == nil || !strings.Contains(err.Error(), "requires schema version 45") {
		t.Fatalf("missing v45 source error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 46)
	for _, table := range typedMemoryStorageTables46 {
		assertSQLiteObjectAbsent(t, database, "table", table)
	}
}

func TestTypedMemoryStorageMigration46RejectsDriftedV45SourceFootprintAtomically(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		statements []string
	}{
		{
			name: "critical table columns",
			statements: []string{
				"ALTER TABLE typed_memory_graph_events ADD COLUMN drifted_source_column TEXT",
			},
		},
		{
			name: "critical index definition",
			statements: []string{
				"DROP INDEX idx_typed_memory_events_project_revision",
				"CREATE INDEX idx_typed_memory_events_project_revision ON typed_memory_graph_events(graph_revision, project_id)",
			},
		},
		{
			name: "critical trigger definition",
			statements: []string{
				"DROP TRIGGER typed_memory_graph_events_exact_head",
				`CREATE TRIGGER typed_memory_graph_events_exact_head
				BEFORE INSERT ON typed_memory_graph_events
				WHEN 0 BEGIN SELECT 1; END`,
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database := openDatabaseBeforeTypedMemoryStorageMigration46(t)
			defer database.Close()
			for _, statement := range testCase.statements {
				if _, err := database.Exec(statement); err != nil {
					t.Fatalf("seed drifted v45 source footprint: %v", err)
				}
			}

			err := Migrate(
				database,
				"schema_version",
				[]Migration{typedMemoryStorageMigration46},
			)
			if err == nil || !strings.Contains(err.Error(), "source footprint drifted") {
				t.Fatalf("drifted v45 source error = %v", err)
			}
			assertMigrationVersionAbsent(t, database, 46)
			for _, table := range typedMemoryStorageTables46 {
				assertSQLiteObjectAbsent(t, database, "table", table)
			}
		})
	}
}

func TestTypedMemoryStorageMigration46RejectsUnknownPartialFootprintAtomically(t *testing.T) {
	t.Parallel()

	database := openDatabaseBeforeTypedMemoryStorageMigration46(t)
	defer database.Close()
	partialTable := "typed_memory_context_slices"

	_, err := database.Exec(
		"CREATE TABLE " + quoteSQLiteIdentifier(partialTable) + " (unknown TEXT)",
	)
	if err != nil {
		t.Fatalf("seed unknown partial v46 footprint: %v", err)
	}
	before := typedMemorySchemaObjects46(t, database)
	err = Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryStorageMigration46},
	)
	if err == nil || !strings.Contains(err.Error(), "unknown partial v46 footprint") {
		t.Fatalf("partial v46 footprint error = %v", err)
	}

	assertMigrationVersionAbsent(t, database, 46)
	assertSQLiteObjectExists(t, database, "table", partialTable)
	after := typedMemorySchemaObjects46(t, database)
	if len(after) != len(before) {
		t.Fatalf("failed v46 migration changed typed-memory object count: before=%d after=%d", len(before), len(after))
	}
	for key, sqlText := range before {
		if after[key] != sqlText {
			t.Fatalf("failed v46 migration changed object %s", key)
		}
	}
}

func TestTypedMemoryStorageMigration46MakesV45WriterFailClosedWithoutRows(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	assertTypedMemoryWriterGeneration46(t, database)
	fixture := newTypedMemoryDeclarationFixture45("legacy-writer", "3", "4", typeEnvRef, 0)

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin old-writer transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture, 1)
	mustInsertTypedMemoryDeclarationMaterialization45(t, transaction, fixture)
	commitErr := insertTypedMemoryGraphCommit45(transaction, fixture)
	if commitErr == nil || !strings.Contains(commitErr.Error(), "requires its exact v46 admission") {
		_ = transaction.Rollback()
		t.Fatalf("old writer commit error = %v", commitErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back old-writer transaction: %v", err)
	}

	for _, table := range []string{
		"typed_memory_graph_events",
		"typed_memory_graph_commits",
		"typed_memory_entities",
		"typed_memory_entity_contexts",
		"typed_memory_idempotency_history",
		"typed_memory_projection_jobs",
		"typed_memory_projection_debt_events",
	} {
		assertTypedMemoryTableRowCount45(t, database, table, 0)
	}
	for _, table := range typedMemoryStorageTables46 {
		expected := 0
		if table == "typed_memory_storage_capabilities" {
			expected = 1
		}
		assertTypedMemoryTableRowCount45(t, database, table, expected)
	}
	assertTypedMemoryGenesisHead46(t, database, typeEnvRef)
}

func TestTypedMemoryStorageMigration46PreservesEntityContextEventCorrelation(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	declared := newTypedMemoryAdmissionFixture46(typeEnvRef, "declared-v46", 0)
	commitTypedMemorySnapshotDeclaration46(t, database, declared)

	other := newTypedMemoryDeclarationFixture45("other-v46", "5", "6", typeEnvRef, 1)
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin v46 mismatched-context transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, other, 1)
	_, insertErr := transaction.Exec(`INSERT INTO typed_memory_entity_contexts (
		project_id, entity_id, bounded_context_ref, label, provenance_ref,
		declared_event_ref, declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		declared.event.projectID,
		declared.event.entityID,
		"bounded-context:wrong-v46-event",
		"Wrong v46 event",
		"provenance:wrong-v46-event",
		other.eventRef,
		declared.event.graphRevision,
		typedMemoryRecordedAt46,
	)
	if insertErr == nil || !strings.Contains(insertErr.Error(), "does not match its declaration event") {
		_ = transaction.Rollback()
		t.Fatalf("v46 mismatched entity-context event error = %v", insertErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back v46 mismatched-context transaction: %v", err)
	}

	assertTypedMemoryTableRowCount45(t, database, "typed_memory_entities", 1)
	assertTypedMemoryTableRowCount45(t, database, "typed_memory_entity_contexts", 1)
	assertTypedMemoryHeadRevision45(t, database, 1)
}

func TestTypedMemoryStorageMigration46PreservesClosedEventMaterializationBoundary(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "closed-v46", 0)
	commitTypedMemorySnapshotDeclaration46(t, database, fixture)

	_, entityErr := database.Exec(`INSERT INTO typed_memory_entities (
		project_id, entity_id, first_declared_event_ref,
		first_declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		"entity:late-v46",
		fixture.event.eventRef,
		fixture.event.graphRevision,
		typedMemoryRecordedAt46,
	)
	if entityErr == nil || !strings.Contains(entityErr.Error(), "does not match its declaration event") {
		t.Fatalf("v46 closed-event entity error = %v", entityErr)
	}
	_, contextErr := database.Exec(`INSERT INTO typed_memory_entity_contexts (
		project_id, entity_id, bounded_context_ref, label, provenance_ref,
		declared_event_ref, declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.entityID,
		"bounded-context:late-v46",
		"Late v46 context",
		"provenance:late-v46-context",
		fixture.event.eventRef,
		fixture.event.graphRevision,
		typedMemoryRecordedAt46,
	)
	if contextErr == nil || !strings.Contains(contextErr.Error(), "does not match its declaration event") {
		t.Fatalf("v46 closed-event entity-context error = %v", contextErr)
	}

	assertTypedMemoryTableRowCount45(t, database, "typed_memory_entities", 1)
	assertTypedMemoryTableRowCount45(t, database, "typed_memory_entity_contexts", 1)
	assertTypedMemoryHeadRevision45(t, database, 1)
}

func TestTypedMemoryStorageMigration46CommitsSnapshotOnlyDeclarationWithExactClosure(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "snapshot-closed", 0)

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin v46 snapshot-only transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture.event, 1)
	mustInsertTypedMemoryDeclarationMaterialization45(t, transaction, fixture.event)
	mustInsertTypedMemoryEntityDeclaration46(
		t,
		transaction,
		fixture.event,
		0,
		"local:"+fixture.event.eventRef,
		"Entity "+fixture.event.entityID,
		"provenance:"+fixture.event.entityID,
	)
	mustInsertTypedMemoryAdmission46(t, transaction, fixture, "snapshot_only")
	counts := typedMemoryClosureCounts46{entity: 1, entityContext: 1, entityDeclaration: 1}
	mustInsertTypedMemoryClosure46(t, transaction, fixture, "snapshot_only", counts)
	if err := insertTypedMemoryGraphCommit45(transaction, fixture.event); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert v46 snapshot-only graph commit: %v", err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit v46 snapshot-only declaration: %v", err)
	}

	assertTypedMemoryHeadRevision45(t, database, 1)
	assertTypedMemoryEventCompanionCount46(
		t,
		database,
		"typed_memory_event_admission_bases",
		fixture.event.eventRef,
		1,
	)
	assertTypedMemoryEventCompanionCount46(
		t,
		database,
		"typed_memory_commit_materialization_closures",
		fixture.event.eventRef,
		1,
	)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestTypedMemoryStorageMigration46CommitsExistingEntityInNewBoundedContext(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	base := newTypedMemoryAdmissionFixture46(typeEnvRef, "cross-context-base", 0)
	commitTypedMemorySnapshotDeclaration46(t, database, base)
	next := newTypedMemoryAdmissionFixture46(typeEnvRef, "cross-context-next", 1)
	next.event.entityID = base.event.entityID
	next.event.boundedContext = "bounded-context:cross-context-next"
	next.event.eventDigest = typedMemoryDigest45("7")
	next.event.changeSetDigest = typedMemoryDigest45("8")
	next.semanticDigest = next.event.changeSetDigest

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin cross-context declaration transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, next.event, 1)
	mustInsertTypedMemoryContextOnlyMaterialization46(t, transaction, next.event)
	mustInsertTypedMemoryEntityDeclaration46(
		t,
		transaction,
		next.event,
		0,
		"local:"+next.event.eventRef,
		"Entity "+next.event.entityID,
		"provenance:"+next.event.entityID+":"+next.event.boundedContext,
	)
	mustInsertTypedMemoryAdmission46(t, transaction, next, "snapshot_only")
	counts := typedMemoryClosureCounts46{entityContext: 1, entityDeclaration: 1}
	mustInsertTypedMemoryClosure46(t, transaction, next, "snapshot_only", counts)
	if err := insertTypedMemoryGraphCommitCounts46(transaction, next.event, 0, 1); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert cross-context graph commit: %v", err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit cross-context declaration: %v", err)
	}

	assertTypedMemoryHeadRevision45(t, database, 2)
	assertTypedMemoryTableRowCount45(t, database, "typed_memory_entities", 1)
	assertTypedMemoryTableRowCount45(t, database, "typed_memory_entity_contexts", 2)
	var entityCount int
	var entityContextCount int
	var topLevelChangeCount int
	err = database.QueryRow(`SELECT entity_count, entity_context_count, top_level_change_count
		FROM typed_memory_event_materialization_footprints_v46
		WHERE project_id = ? AND event_ref = ?`,
		next.event.projectID,
		next.event.eventRef,
	).Scan(&entityCount, &entityContextCount, &topLevelChangeCount)
	if err != nil {
		t.Fatalf("read cross-context materialization footprint: %v", err)
	}
	if entityCount != 0 || entityContextCount != 1 || topLevelChangeCount != 1 {
		t.Fatalf(
			"cross-context footprint = entities:%d contexts:%d top-level:%d",
			entityCount,
			entityContextCount,
			topLevelChangeCount,
		)
	}
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestTypedMemoryStorageMigration46RejectsUnpairedEntityAndForeignContextDeclaration(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	base := newTypedMemoryAdmissionFixture46(typeEnvRef, "pairing-base", 0)
	commitTypedMemorySnapshotDeclaration46(t, database, base)
	current := newTypedMemoryAdmissionFixture46(typeEnvRef, "pairing-current", 1)
	current.event.entityID = base.event.entityID
	current.event.boundedContext = "bounded-context:pairing-current"
	current.event.eventDigest = typedMemoryDigest45("7")
	current.event.changeSetDigest = typedMemoryDigest45("8")
	current.semanticDigest = current.event.changeSetDigest

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin mismatched entity/context pairing transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, current.event, 1)
	_, err = transaction.Exec(`INSERT INTO typed_memory_entities (
		project_id, entity_id, first_declared_event_ref,
		first_declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?)`,
		current.event.projectID,
		"entity:unpaired-new",
		current.event.eventRef,
		current.event.graphRevision,
		typedMemoryRecordedAt46,
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert unpaired new entity: %v", err)
	}
	mustInsertTypedMemoryContextOnlyMaterialization46(t, transaction, current.event)
	mustInsertTypedMemoryEntityDeclaration46(
		t,
		transaction,
		current.event,
		0,
		"local:"+current.event.eventRef,
		"Entity "+current.event.entityID,
		"provenance:"+current.event.entityID+":"+current.event.boundedContext,
	)
	mustInsertTypedMemoryAdmission46(t, transaction, current, "snapshot_only")
	counts := typedMemoryClosureCounts46{
		entity:            1,
		entityContext:     1,
		entityDeclaration: 1,
	}
	closureErr := insertTypedMemoryClosure46(transaction, current, "snapshot_only", counts)
	if closureErr == nil || !strings.Contains(closureErr.Error(), "exact complete event footprint") {
		_ = transaction.Rollback()
		t.Fatalf("unpaired entity/context closure error = %v", closureErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back mismatched entity/context pairing: %v", err)
	}
	assertTypedMemoryHeadRevision45(t, database, 1)
	assertTypedMemoryTableRowCount45(t, database, "typed_memory_entities", 1)
}

func TestTypedMemoryStorageMigration46RejectsContextBoundToCommittedForeignEvent(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	base := newTypedMemoryAdmissionFixture46(typeEnvRef, "foreign-context-base", 0)
	commitTypedMemorySnapshotDeclaration46(t, database, base)

	_, err := database.Exec(`INSERT INTO typed_memory_entity_contexts (
		project_id, entity_id, bounded_context_ref, label, provenance_ref,
		declared_event_ref, declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		base.event.projectID,
		base.event.entityID,
		"bounded-context:foreign-event",
		"Foreign event context",
		"provenance:foreign-event",
		base.event.eventRef,
		base.event.graphRevision,
		typedMemoryRecordedAt46,
	)
	if err == nil || !strings.Contains(err.Error(), "does not match its declaration event") {
		t.Fatalf("committed foreign-event context error = %v", err)
	}
	assertTypedMemoryTableRowCount45(t, database, "typed_memory_entity_contexts", 1)
}

func TestTypedMemoryStorageMigration46RejectsEntityFromUncommittedForeignEvent(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	base := newTypedMemoryAdmissionFixture46(typeEnvRef, "uncommitted-source", 0)
	commitTypedMemorySnapshotDeclaration46(t, database, base)
	next := newTypedMemoryAdmissionFixture46(typeEnvRef, "uncommitted-current", 1)
	next.event.entityID = base.event.entityID
	next.event.boundedContext = "bounded-context:uncommitted-current"
	next.event.eventDigest = typedMemoryDigest45("9")
	next.event.changeSetDigest = typedMemoryDigest45("0")
	next.semanticDigest = next.event.changeSetDigest

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin uncommitted-source transaction: %v", err)
	}
	// The deferred event-to-commit foreign key makes an uncommitted foreign
	// declaration unreachable through the normal API across transactions. This
	// transaction-local corruption fixture removes only the source commit long
	// enough to prove the v46 context trigger itself remains fail-closed.
	if _, err := transaction.Exec("DROP TRIGGER typed_memory_graph_commits_no_delete"); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("temporarily expose uncommitted-source fixture: %v", err)
	}
	if _, err := transaction.Exec(`DELETE FROM typed_memory_graph_commits
		WHERE project_id = ? AND event_ref = ?`,
		base.event.projectID,
		base.event.eventRef,
	); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("remove source commit in negative fixture: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, next.event, 1)
	_, contextErr := transaction.Exec(`INSERT INTO typed_memory_entity_contexts (
		project_id, entity_id, bounded_context_ref, label, provenance_ref,
		declared_event_ref, declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		next.event.projectID,
		next.event.entityID,
		next.event.boundedContext,
		"Uncommitted source context",
		"provenance:uncommitted-source",
		next.event.eventRef,
		next.event.graphRevision,
		typedMemoryRecordedAt46,
	)
	if contextErr == nil || !strings.Contains(contextErr.Error(), "does not match its declaration event") {
		_ = transaction.Rollback()
		t.Fatalf("uncommitted foreign-source context error = %v", contextErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back uncommitted-source fixture: %v", err)
	}
	assertTypedMemoryHeadRevision45(t, database, 1)
	assertTypedMemoryTableRowCount45(t, database, "typed_memory_entity_contexts", 1)
}

func TestTypedMemoryStorageMigration46RejectsClosureWhenTopLevelChangeCountIsPartial(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "partial-change", 0)

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin partial-change transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture.event, 2)
	mustInsertTypedMemoryDeclarationMaterialization45(t, transaction, fixture.event)
	mustInsertTypedMemoryEntityDeclaration46(
		t,
		transaction,
		fixture.event,
		0,
		"local:"+fixture.event.eventRef,
		"Entity "+fixture.event.entityID,
		"provenance:"+fixture.event.entityID,
	)
	mustInsertTypedMemoryAdmission46(t, transaction, fixture, "snapshot_only")
	counts := typedMemoryClosureCounts46{entity: 1, entityContext: 1, entityDeclaration: 1}
	closureErr := insertTypedMemoryClosure46(transaction, fixture, "snapshot_only", counts)
	if closureErr == nil || !strings.Contains(closureErr.Error(), "exact complete event footprint") {
		_ = transaction.Rollback()
		t.Fatalf("partial top-level closure error = %v", closureErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back partial-change transaction: %v", err)
	}
	assertTypedMemoryHeadRevision45(t, database, 0)
}

func TestTypedMemoryStorageMigration46RejectsUndefinedMemberOfRows(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "undefined-memberof", 0)

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin undefined-MemberOf transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture.event, 1)
	contextSliceRef := mustInsertTypedMemoryContextSlice46(
		t,
		transaction,
		fixture,
		"5",
		"bounded-context:fixture",
		[]byte("context-slice-bytes"),
	)
	_, memberErr := transaction.Exec(`INSERT INTO typed_memory_memberof_evaluations (
		project_id, event_ref, evaluation_ref, judgement_kind,
		entity_id, value_kind_ref, context_slice_ref,
		evaluator_rule_ref, evaluation_provenance_ref,
		evaluation_view_kind, evaluation_view_digest, canonical_evaluation_view_bytes,
		observable_input_count, observable_input_set_digest,
		query_digest, canonical_query_bytes,
		basis_digest, canonical_basis_bytes,
		judgement_digest, canonical_judgement_bytes
	) VALUES (?, ?, ?, 'undefined', ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		"evaluation:undefined",
		"entity:fixture",
		"value-kind:fixture",
		contextSliceRef,
		"rule:fixture",
		"provenance:fixture",
		"persisted_snapshot",
		typedMemoryDigest45("6"),
		[]byte("view"),
		typedMemoryDigest45("0"),
		typedMemoryDigest45("7"),
		[]byte("query"),
		typedMemoryDigest45("8"),
		[]byte("basis"),
		typedMemoryDigest45("9"),
		[]byte("judgement"),
	)
	if memberErr == nil || !strings.Contains(memberErr.Error(), "CHECK constraint failed") {
		_ = transaction.Rollback()
		t.Fatalf("undefined MemberOf insert error = %v", memberErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back undefined-MemberOf transaction: %v", err)
	}
}

func TestTypedMemoryStorageMigration46SealsContentAddressedContextSliceIdentity(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "context-slice-identity", 0)

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin ContextSlice identity transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture.event, 1)
	contextSliceRef := mustInsertTypedMemoryContextSlice46(
		t,
		transaction,
		fixture,
		"2",
		"bounded-context:fixture",
		[]byte("context-slice:stable"),
	)
	_, addressErr := transaction.Exec(`INSERT INTO typed_memory_context_slice_catalog (
		project_id, event_ref, context_slice_ref, context_slice_digest,
		bounded_context_ref, canonical_context_slice_bytes
	) VALUES (?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		"context-slice:not-the-digest",
		typedMemoryDigest45("3"),
		"bounded-context:fixture",
		[]byte("context-slice:wrong-address"),
	)
	if addressErr == nil || !strings.Contains(addressErr.Error(), "CHECK constraint failed") {
		_ = transaction.Rollback()
		t.Fatalf("non-content-addressed ContextSlice error = %v", addressErr)
	}
	_, conflictErr := transaction.Exec(`INSERT INTO typed_memory_context_slice_catalog (
		project_id, event_ref, context_slice_ref, context_slice_digest,
		bounded_context_ref, canonical_context_slice_bytes
	) VALUES (?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		contextSliceRef,
		typedMemoryDigest45("2"),
		"bounded-context:fixture",
		[]byte("context-slice:different-bytes"),
	)
	if conflictErr == nil || !strings.Contains(conflictErr.Error(), "UNIQUE constraint failed") {
		_ = transaction.Rollback()
		t.Fatalf("same ContextSlice ref with different bytes error = %v", conflictErr)
	}
	otherDigest := typedMemoryDigest45("4")
	otherRef := "context-slice:" + otherDigest
	_, err = transaction.Exec(`INSERT INTO typed_memory_context_slice_catalog (
		project_id, event_ref, context_slice_ref, context_slice_digest,
		bounded_context_ref, canonical_context_slice_bytes
	) VALUES (?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		otherRef,
		otherDigest,
		"bounded-context:fixture",
		[]byte("context-slice:catalog-bytes"),
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert second ContextSlice catalog identity: %v", err)
	}
	_, useErr := transaction.Exec(`INSERT INTO typed_memory_context_slices (
		project_id, event_ref, context_slice_ref, context_slice_digest,
		bounded_context_ref, canonical_context_slice_bytes
	) VALUES (?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		otherRef,
		otherDigest,
		"bounded-context:fixture",
		[]byte("context-slice:forged-use-bytes"),
	)
	if useErr == nil || !strings.Contains(useErr.Error(), "FOREIGN KEY constraint failed") {
		_ = transaction.Rollback()
		t.Fatalf("ContextSlice use with non-catalog bytes error = %v", useErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back ContextSlice identity transaction: %v", err)
	}
}

func TestTypedMemoryStorageMigration46SealsReferenceResolutionVariants(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                     string
		resolutionKind           string
		resolutionBasisRef       any
		declarationChangeOrdinal any
		batchLocalRefOverride    any
		omitDeclaration          bool
		wantAccepted             bool
		wantErrorFragment        string
	}{
		{
			name:                     "snapshot reference",
			resolutionKind:           "snapshot_reference",
			resolutionBasisRef:       "snapshot-basis:fixture",
			declarationChangeOrdinal: nil,
			wantAccepted:             true,
		},
		{
			name:                     "same batch declaration",
			resolutionKind:           "same_batch_declaration",
			resolutionBasisRef:       nil,
			declarationChangeOrdinal: 0,
			wantAccepted:             true,
		},
		{
			name:                     "snapshot cannot carry declaration coordinate",
			resolutionKind:           "snapshot_reference",
			resolutionBasisRef:       "snapshot-basis:fixture",
			declarationChangeOrdinal: 0,
			wantAccepted:             false,
			wantErrorFragment:        "CHECK constraint failed",
		},
		{
			name:                     "same batch cannot fabricate snapshot basis",
			resolutionKind:           "same_batch_declaration",
			resolutionBasisRef:       "fabricated-snapshot-basis",
			declarationChangeOrdinal: 0,
			wantAccepted:             false,
			wantErrorFragment:        "CHECK constraint failed",
		},
		{
			name:                     "same batch requires declaration coordinate",
			resolutionKind:           "same_batch_declaration",
			resolutionBasisRef:       nil,
			declarationChangeOrdinal: nil,
			wantAccepted:             false,
			wantErrorFragment:        "CHECK constraint failed",
		},
		{
			name:                     "same batch declaration must precede relation",
			resolutionKind:           "same_batch_declaration",
			resolutionBasisRef:       nil,
			declarationChangeOrdinal: 1,
			wantAccepted:             false,
			wantErrorFragment:        "CHECK constraint failed",
		},
		{
			name:                     "same batch local ref must name exact declaration",
			resolutionKind:           "same_batch_declaration",
			resolutionBasisRef:       nil,
			declarationChangeOrdinal: 0,
			batchLocalRefOverride:    "local:wrong-declaration",
			wantAccepted:             false,
			wantErrorFragment:        "FOREIGN KEY constraint failed",
		},
		{
			name:                     "same batch requires exact declaration row",
			resolutionKind:           "same_batch_declaration",
			resolutionBasisRef:       nil,
			declarationChangeOrdinal: 0,
			omitDeclaration:          true,
			wantAccepted:             false,
			wantErrorFragment:        "FOREIGN KEY constraint failed",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
			defer database.Close()
			insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
			fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "resolution-variant", 0)

			transaction, err := database.Begin()
			if err != nil {
				t.Fatalf("begin resolution-variant transaction: %v", err)
			}
			mustInsertTypedMemoryEvent45(t, transaction, fixture.event, 2)
			mustInsertTypedMemoryDeclarationMaterialization45(t, transaction, fixture.event)
			if !testCase.omitDeclaration {
				mustInsertTypedMemoryEntityDeclaration46(
					t,
					transaction,
					fixture.event,
					0,
					"local:fixture",
					"Entity "+fixture.event.entityID,
					"provenance:"+fixture.event.entityID,
				)
			}
			mustInsertTypedMemoryAdmission46(t, transaction, fixture, "context_slice_membership")
			_, err = transaction.Exec(`INSERT INTO typed_memory_ordered_candidate_prefixes (
				project_id, event_ref, prefix_end_ordinal, request_digest, prefix_digest
			) VALUES (?, ?, 1, ?, ?)`,
				fixture.event.projectID,
				fixture.event.eventRef,
				fixture.requestDigest,
				typedMemoryDigest45("b"),
			)
			if err != nil {
				_ = transaction.Rollback()
				t.Fatalf("insert exact candidate prefix identity: %v", err)
			}
			mustInsertTypedMemoryReferenceFiller46(t, transaction, fixture, 1)
			insertErr := insertTypedMemoryReferenceResolution46(
				transaction,
				fixture,
				1,
				testCase.resolutionKind,
				testCase.resolutionBasisRef,
				testCase.declarationChangeOrdinal,
				testCase.batchLocalRefOverride,
			)
			if testCase.wantAccepted && insertErr != nil {
				_ = transaction.Rollback()
				t.Fatalf("insert accepted resolution variant: %v", insertErr)
			}
			if !testCase.wantAccepted &&
				(insertErr == nil || !strings.Contains(insertErr.Error(), testCase.wantErrorFragment)) {
				_ = transaction.Rollback()
				t.Fatalf("rejected resolution variant error = %v", insertErr)
			}
			if err := transaction.Rollback(); err != nil {
				t.Fatalf("roll back resolution-variant transaction: %v", err)
			}
		})
	}
}

func TestTypedMemoryStorageMigration46RejectsResolutionEvaluationViewMismatch(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		resolutionKind string
		viewKind       string
	}{
		{name: "snapshot resolution with prospective view", resolutionKind: "snapshot_reference", viewKind: "prospective_batch"},
		{name: "same-batch resolution with persisted view", resolutionKind: "same_batch_declaration", viewKind: "persisted_snapshot"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
			defer database.Close()
			insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
			fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "view-mismatch", 0)
			transaction, err := database.Begin()
			if err != nil {
				t.Fatalf("begin view-mismatch transaction: %v", err)
			}
			mustInsertTypedMemoryEvent45(t, transaction, fixture.event, 2)
			mustInsertTypedMemoryDeclarationMaterialization45(t, transaction, fixture.event)
			mustInsertTypedMemoryEntityDeclaration46(
				t, transaction, fixture.event, 0, "local:fixture",
				"Entity "+fixture.event.entityID,
				"provenance:"+fixture.event.entityID,
			)
			mustInsertTypedMemoryAdmission46(t, transaction, fixture, "context_slice_membership")
			_, err = transaction.Exec(`INSERT INTO typed_memory_ordered_candidate_prefixes (
				project_id, event_ref, prefix_end_ordinal, request_digest, prefix_digest
			) VALUES (?, ?, 1, ?, ?)`,
				fixture.event.projectID, fixture.event.eventRef,
				fixture.requestDigest, typedMemoryDigest45("b"),
			)
			if err != nil {
				_ = transaction.Rollback()
				t.Fatalf("insert view-mismatch prefix: %v", err)
			}
			mustInsertTypedMemoryReferenceFiller46(t, transaction, fixture, 1)
			basisRef := any(nil)
			declarationOrdinal := any(0)
			if testCase.resolutionKind == "snapshot_reference" {
				basisRef = "snapshot-basis:fixture"
				declarationOrdinal = nil
			}
			if err := insertTypedMemoryReferenceResolution46(
				transaction, fixture, 1, testCase.resolutionKind,
				basisRef, declarationOrdinal, nil,
			); err != nil {
				_ = transaction.Rollback()
				t.Fatalf("insert view-mismatch resolution: %v", err)
			}
			evaluationRef := mustInsertTypedMemoryMemberOfEvaluation46(
				t, transaction, fixture, testCase.viewKind, 1,
			)
			useErr := insertTypedMemoryMemberOfUse46(transaction, fixture, 1, evaluationRef)
			if useErr == nil || !strings.Contains(useErr.Error(), "exact filler, query, view") {
				_ = transaction.Rollback()
				t.Fatalf("resolution/evaluation view mismatch error = %v", useErr)
			}
			if err := transaction.Rollback(); err != nil {
				t.Fatalf("roll back view-mismatch transaction: %v", err)
			}
		})
	}
}

func TestTypedMemoryStorageMigration46RejectsIncompleteObservableInputSetAtClosure(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "observable-set", 0)
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin observable-set transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture.event, 1)
	mustInsertTypedMemoryAdmission46(t, transaction, fixture, "context_slice_membership")
	mustInsertTypedMemoryReferenceFiller46(t, transaction, fixture, 0)
	if err := insertTypedMemoryReferenceResolution46(
		transaction, fixture, 0, "snapshot_reference",
		"snapshot-basis:fixture", nil, nil,
	); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert observable-set snapshot resolution: %v", err)
	}
	evaluationRef := mustInsertTypedMemoryMemberOfEvaluation46(
		t, transaction, fixture, "persisted_snapshot", 2,
	)
	_, err = transaction.Exec(`INSERT INTO typed_memory_observable_input_blobs (
		project_id, event_ref, observable_input_ref,
		observable_input_digest, canonical_observable_input_bytes
	) VALUES (?, ?, ?, ?, ?)`,
		fixture.event.projectID, fixture.event.eventRef,
		"observable:one", typedMemoryDigest45("9"), []byte("observable:one"),
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert observable-set blob: %v", err)
	}
	_, err = transaction.Exec(`INSERT INTO typed_memory_memberof_observable_inputs (
		project_id, event_ref, evaluation_ref, input_ordinal,
		observable_input_ref, observable_input_digest
	) VALUES (?, ?, ?, 0, ?, ?)`,
		fixture.event.projectID, fixture.event.eventRef, evaluationRef,
		"observable:one", typedMemoryDigest45("9"),
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert one of two declared observable inputs: %v", err)
	}
	if err := insertTypedMemoryMemberOfUse46(transaction, fixture, 0, evaluationRef); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert observable-set MemberOf use: %v", err)
	}
	counts := typedMemoryClosureCounts46{
		contextSliceCatalog:    1,
		contextSlice:           1,
		observableInputBlob:    1,
		relation:               1,
		relationSlot:           1,
		relationFiller:         1,
		referenceResolutionUse: 1,
		memberOfEvaluation:     1,
		memberOfInput:          1,
		memberOfUse:            1,
	}
	closureErr := insertTypedMemoryClosure46(transaction, fixture, "context_slice_membership", counts)
	if closureErr == nil || !strings.Contains(closureErr.Error(), "exact complete event footprint") {
		_ = transaction.Rollback()
		t.Fatalf("incomplete observable-input set closure error = %v", closureErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back observable-set transaction: %v", err)
	}
}

func TestTypedMemoryStorageMigration46RepresentsAliasReplacementChainWithoutForks(t *testing.T) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "alias-chain", 0)

	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin alias-chain transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture.event, 4)
	mustInsertTypedMemoryAliasChange46(
		t,
		transaction,
		fixture,
		0,
		"alias-change:a",
		"admit_alias",
		"A",
		nil,
		"",
	)
	mustInsertTypedMemoryAliasChange46(
		t,
		transaction,
		fixture,
		1,
		"alias-change:b",
		"supersede_alias",
		"A",
		"B",
		"alias-change:a",
	)
	mustInsertTypedMemoryAliasChange46(
		t,
		transaction,
		fixture,
		2,
		"alias-change:c",
		"supersede_alias",
		"B",
		"C",
		"alias-change:b",
	)
	forkErr := insertTypedMemoryAliasChange46(
		transaction,
		fixture,
		3,
		"alias-change:d",
		"supersede_alias",
		"B",
		"D",
		"alias-change:b",
	)
	if forkErr == nil || !strings.Contains(forkErr.Error(), "UNIQUE constraint failed") {
		_ = transaction.Rollback()
		t.Fatalf("forked alias lineage error = %v", forkErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back alias-chain transaction: %v", err)
	}
}

func openDatabaseBeforeTypedMemoryStorageMigration46(t *testing.T) *sql.DB {
	t.Helper()
	database := openDatabaseBeforeTypedMemoryStorageMigration45(t)
	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryStorageMigration45},
	); err != nil {
		_ = database.Close()
		t.Fatalf("migrate database through v45: %v", err)
	}
	assertMigrationVersionPresent(t, database, 45)
	assertMigrationVersionAbsent(t, database, 46)
	return database
}

func newTypedMemoryRawSQLDatabase46(
	t *testing.T,
	withV46 bool,
) (*sql.DB, string) {
	t.Helper()
	database := openDatabaseBeforeTypedMemoryStorageMigration46(t)
	insertProjectLedgerBinding44(t, database, "/tmp/typed-memory-v46")
	digest := typedMemoryDigest45("a")
	typeEnvRef := "typeenv:" + digest
	_, err := database.Exec(`INSERT INTO typed_memory_type_env_snapshots (
		type_env_ref, artifact_digest, snapshot_format, canonical_bytes,
		source_revision, compiler_schema_version, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		typeEnvRef,
		digest,
		"haft.typed-memory.type-env/v1",
		[]byte("canonical-type-env-v1"),
		"source-revision:v1",
		"compiler-schema:v1",
		typedMemoryRecordedAt46,
	)
	if err != nil {
		_ = database.Close()
		t.Fatalf("insert v46 fixture TypeEnv snapshot: %v", err)
	}
	if !withV46 {
		return database, typeEnvRef
	}
	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryStorageMigration46},
	); err != nil {
		_ = database.Close()
		t.Fatalf("migrate fixture database through v46: %v", err)
	}
	return database, typeEnvRef
}

func newTypedMemoryAdmissionFixture46(
	typeEnvRef string,
	suffix string,
	expectedRevision int,
) typedMemoryAdmissionFixture46 {
	event := newTypedMemoryDeclarationFixture45(
		suffix,
		"b",
		"c",
		typeEnvRef,
		expectedRevision,
	)
	return typedMemoryAdmissionFixture46{
		event:                    event,
		requestDigest:            typedMemoryDigest45("d"),
		semanticDigest:           event.changeSetDigest,
		envelopeDigest:           typedMemoryDigest45("e"),
		basisDigest:              typedMemoryDigest45("f"),
		manifestDigest:           typedMemoryDigest45("2"),
		materializationDigest:    typedMemoryDigest45("1"),
		canonicalRequest:         []byte(`{"candidate":"declare_entity"}`),
		canonicalSemantic:        []byte(`{"change":"declare_entity"}`),
		canonicalEnvelope:        []byte("admission-envelope-bytes"),
		canonicalBasis:           []byte("admission-basis-bytes"),
		canonicalManifest:        []byte("expected-materialization-manifest-bytes"),
		canonicalMaterialization: []byte("materialization-closure-bytes"),
	}
}

func commitTypedMemorySnapshotDeclaration46(
	t *testing.T,
	database *sql.DB,
	fixture typedMemoryAdmissionFixture46,
) {
	t.Helper()
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin v46 snapshot declaration: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture.event, 1)
	mustInsertTypedMemoryDeclarationMaterialization45(t, transaction, fixture.event)
	mustInsertTypedMemoryEntityDeclaration46(
		t,
		transaction,
		fixture.event,
		0,
		"local:"+fixture.event.eventRef,
		"Entity "+fixture.event.entityID,
		"provenance:"+fixture.event.entityID,
	)
	mustInsertTypedMemoryAdmission46(t, transaction, fixture, "snapshot_only")
	counts := typedMemoryClosureCounts46{entity: 1, entityContext: 1, entityDeclaration: 1}
	mustInsertTypedMemoryClosure46(t, transaction, fixture, "snapshot_only", counts)
	if err := insertTypedMemoryGraphCommit45(transaction, fixture.event); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert v46 snapshot declaration commit: %v", err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit v46 snapshot declaration: %v", err)
	}
}

func assertTypedMemoryWriterGeneration46(t *testing.T, database *sql.DB) {
	t.Helper()
	var generation int
	var digest string
	var canonical []byte
	err := database.QueryRow(`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_storage_capabilities WHERE capability_key = ?`,
		typedMemoryWriterGenerationCapability46,
	).Scan(&generation, &digest, &canonical)
	if err != nil {
		t.Fatalf("read typed-memory writer generation: %v", err)
	}
	if generation != typedMemoryWriterGeneration46 ||
		digest != typedMemoryWriterMarkerDigest46 ||
		string(canonical) != typedMemoryWriterMarkerBytes46 {
		t.Fatalf(
			"typed-memory writer generation = %d %q %q; want exact v46 capability",
			generation,
			digest,
			canonical,
		)
	}
}

func assertTypedMemoryGenesisHead46(
	t *testing.T,
	database *sql.DB,
	typeEnvRef string,
) {
	t.Helper()
	var revision int
	var activeTypeEnv string
	var lastEventRef string
	var lastCommitRef string
	err := database.QueryRow(`SELECT graph_revision, active_type_env_ref,
		last_event_ref, last_commit_ref
		FROM typed_memory_graph_heads WHERE project_id = ?`,
		typedMemoryProjectID45,
	).Scan(&revision, &activeTypeEnv, &lastEventRef, &lastCommitRef)
	if err != nil {
		t.Fatalf("read v46 typed-memory genesis head: %v", err)
	}
	if revision != 0 || activeTypeEnv != typeEnvRef || lastEventRef != "" || lastCommitRef != "" {
		t.Fatalf(
			"v46 head after denied v45 writer = revision %d typeenv %q event %q commit %q; want exact genesis",
			revision,
			activeTypeEnv,
			lastEventRef,
			lastCommitRef,
		)
	}
}

func mustInsertTypedMemoryAdmission46(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	basisKind string,
) {
	t.Helper()
	_, err := execer.Exec(`INSERT INTO typed_memory_event_writer_generations (
		project_id, event_ref, writer_generation, provenance_kind
	) VALUES (?, ?, 46, 'writer_v46')`,
		fixture.event.projectID,
		fixture.event.eventRef,
	)
	if err != nil {
		t.Fatalf("insert v46 event writer generation: %v", err)
	}
	_, err = execer.Exec(`INSERT INTO typed_memory_event_admission_bases (
		project_id, event_ref, event_digest, admission_basis_kind,
		type_env_ref, basis_graph_revision,
		request_digest, canonical_request_bytes,
		semantic_digest, canonical_semantic_bytes,
		admission_envelope_digest, canonical_admission_envelope_bytes,
		admission_basis_digest, canonical_admission_basis_bytes,
		materialization_manifest_digest, canonical_materialization_manifest_bytes,
		recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		fixture.event.eventDigest,
		basisKind,
		fixture.event.typeEnvRef,
		fixture.event.expectedRevision,
		fixture.requestDigest,
		fixture.canonicalRequest,
		fixture.semanticDigest,
		fixture.canonicalSemantic,
		fixture.envelopeDigest,
		fixture.canonicalEnvelope,
		fixture.basisDigest,
		fixture.canonicalBasis,
		fixture.manifestDigest,
		fixture.canonicalManifest,
		typedMemoryRecordedAt46,
	)
	if err != nil {
		t.Fatalf("insert v46 event admission basis: %v", err)
	}
}

func mustInsertTypedMemoryClosure46(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	basisKind string,
	counts typedMemoryClosureCounts46,
) {
	t.Helper()
	if err := insertTypedMemoryClosure46(execer, fixture, basisKind, counts); err != nil {
		t.Fatalf("insert v46 materialization closure: %v", err)
	}
}

func insertTypedMemoryClosure46(
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	basisKind string,
	counts typedMemoryClosureCounts46,
) error {
	_, err := execer.Exec(`INSERT INTO typed_memory_commit_materialization_closures (
		project_id, event_ref, commit_ref, event_digest, admission_basis_kind,
		request_digest, semantic_digest, admission_envelope_digest, admission_basis_digest,
		materialization_manifest_digest,
		materialization_digest, canonical_materialization_bytes,
		entity_count, entity_context_count, entity_declaration_count,
		context_slice_catalog_count, context_slice_count,
		value_blob_count, observable_input_blob_count,
		relation_count, relation_slot_count, relation_filler_count,
		ordered_candidate_prefix_count,
		reference_resolution_use_count,
		memberof_evaluation_count, memberof_input_count, memberof_use_count,
		alias_change_count, retraction_count, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		fixture.event.commitRef,
		fixture.event.eventDigest,
		basisKind,
		fixture.requestDigest,
		fixture.semanticDigest,
		fixture.envelopeDigest,
		fixture.basisDigest,
		fixture.manifestDigest,
		fixture.materializationDigest,
		fixture.canonicalMaterialization,
		counts.entity,
		counts.entityContext,
		counts.entityDeclaration,
		counts.contextSliceCatalog,
		counts.contextSlice,
		counts.valueBlob,
		counts.observableInputBlob,
		counts.relation,
		counts.relationSlot,
		counts.relationFiller,
		counts.orderedCandidatePrefix,
		counts.referenceResolutionUse,
		counts.memberOfEvaluation,
		counts.memberOfInput,
		counts.memberOfUse,
		counts.aliasChange,
		counts.retraction,
		typedMemoryRecordedAt46,
	)
	return err
}

func mustInsertTypedMemoryContextOnlyMaterialization46(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryDeclarationFixture45,
) {
	t.Helper()
	_, err := execer.Exec(`INSERT INTO typed_memory_entity_contexts (
		project_id, entity_id, bounded_context_ref, label, provenance_ref,
		declared_event_ref, declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.projectID,
		fixture.entityID,
		fixture.boundedContext,
		"Entity "+fixture.entityID,
		"provenance:"+fixture.entityID+":"+fixture.boundedContext,
		fixture.eventRef,
		fixture.graphRevision,
		typedMemoryRecordedAt46,
	)
	if err != nil {
		t.Fatalf("insert cross-context entity context %s: %v", fixture.boundedContext, err)
	}
	_, err = execer.Exec(`INSERT INTO typed_memory_idempotency_history (
		project_id, idempotency_key, change_set_digest,
		event_ref, graph_revision, result_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		fixture.projectID,
		fixture.idempotencyKey,
		fixture.changeSetDigest,
		fixture.eventRef,
		fixture.graphRevision,
		fixture.eventDigest,
		typedMemoryRecordedAt46,
	)
	if err != nil {
		t.Fatalf("insert cross-context idempotency row %s: %v", fixture.idempotencyKey, err)
	}
	_, err = execer.Exec(`INSERT INTO typed_memory_projection_jobs (
		project_id, projection_job_ref, semantic_event_ref,
		graph_revision, target_kind, input_event_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		fixture.projectID,
		fixture.projectionJobRef,
		fixture.eventRef,
		fixture.graphRevision,
		"project_carriers",
		fixture.eventDigest,
		typedMemoryRecordedAt46,
	)
	if err != nil {
		t.Fatalf("insert cross-context projection job %s: %v", fixture.projectionJobRef, err)
	}
}

func mustInsertTypedMemoryEntityDeclaration46(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryDeclarationFixture45,
	changeOrdinal int,
	batchLocalRef string,
	label string,
	provenanceRef string,
) {
	t.Helper()
	_, err := execer.Exec(`INSERT INTO typed_memory_entity_declarations (
		project_id, event_ref, change_ordinal, entity_id, batch_local_ref,
		bounded_context_ref, label, provenance_ref,
		declaration_digest, canonical_declaration_bytes
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.projectID,
		fixture.eventRef,
		changeOrdinal,
		fixture.entityID,
		batchLocalRef,
		fixture.boundedContext,
		label,
		provenanceRef,
		typedMemoryDigest45("a"),
		[]byte("candidate-declaration:"+fixture.eventRef),
	)
	if err != nil {
		t.Fatalf("insert exact entity declaration for %s: %v", fixture.eventRef, err)
	}
}

func insertTypedMemoryGraphCommitCounts46(
	execer typedMemoryExecer45,
	fixture typedMemoryDeclarationFixture45,
	entityCount int,
	entityContextCount int,
) error {
	_, err := execer.Exec(`INSERT INTO typed_memory_graph_commits (
		project_id, commit_ref, event_ref, event_digest,
		expected_revision, graph_revision, change_set_digest,
		idempotency_key, projection_job_ref,
		entity_count, entity_context_count, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.projectID,
		fixture.commitRef,
		fixture.eventRef,
		fixture.eventDigest,
		fixture.expectedRevision,
		fixture.graphRevision,
		fixture.changeSetDigest,
		fixture.idempotencyKey,
		fixture.projectionJobRef,
		entityCount,
		entityContextCount,
		typedMemoryRecordedAt46,
	)
	return err
}

func mustInsertTypedMemoryContextSlice46(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	digestSeed string,
	boundedContextRef string,
	canonicalBytes []byte,
) string {
	t.Helper()
	digest := typedMemoryDigest45(digestSeed)
	contextSliceRef := "context-slice:" + digest
	_, err := execer.Exec(`INSERT OR IGNORE INTO typed_memory_context_slice_catalog (
		project_id, event_ref, context_slice_ref, context_slice_digest,
		bounded_context_ref, canonical_context_slice_bytes
	) VALUES (?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		contextSliceRef,
		digest,
		boundedContextRef,
		canonicalBytes,
	)
	if err != nil {
		t.Fatalf("insert ContextSlice catalog identity: %v", err)
	}
	_, err = execer.Exec(`INSERT INTO typed_memory_context_slices (
		project_id, event_ref, context_slice_ref, context_slice_digest,
		bounded_context_ref, canonical_context_slice_bytes
	) VALUES (?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		contextSliceRef,
		digest,
		boundedContextRef,
		canonicalBytes,
	)
	if err != nil {
		t.Fatalf("insert exact event-local ContextSlice use: %v", err)
	}
	return contextSliceRef
}

func mustInsertTypedMemoryReferenceFiller46(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	relationChangeOrdinal int,
) {
	t.Helper()
	contextSliceRef := mustInsertTypedMemoryContextSlice46(
		t,
		execer,
		fixture,
		"2",
		"bounded-context:fixture",
		[]byte("context-slice:reference-fixture"),
	)
	assertionID := "assertion:reference-fixture"
	_, err := execer.Exec(`INSERT INTO typed_memory_relation_instances (
		project_id, event_ref, change_ordinal, assertion_id,
		signature_ref, context_slice_ref, relation_digest,
		canonical_relation_bytes, provenance_ref
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		relationChangeOrdinal,
		assertionID,
		"signature:fixture",
		contextSliceRef,
		typedMemoryDigest45("3"),
		[]byte("relation:reference-fixture"),
		"provenance:fixture",
	)
	if err != nil {
		t.Fatalf("insert reference-fixture relation: %v", err)
	}
	_, err = execer.Exec(`INSERT INTO typed_memory_relation_slots (
		project_id, event_ref, change_ordinal, assertion_id,
		slot_ordinal, slot_kind_ref, slot_digest, canonical_slot_bytes
	) VALUES (?, ?, ?, ?, 0, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		relationChangeOrdinal,
		assertionID,
		"slot-kind:fixture",
		typedMemoryDigest45("4"),
		[]byte("slot:reference-fixture"),
	)
	if err != nil {
		t.Fatalf("insert reference-fixture slot: %v", err)
	}
	_, err = execer.Exec(`INSERT INTO typed_memory_relation_fillers (
		project_id, event_ref, change_ordinal, assertion_id,
		slot_ordinal, filler_ordinal, filler_kind,
		reference_kind_ref, reference_id, entity_id,
		required_value_kind_ref, value_ref,
		filler_digest, canonical_filler_bytes
	) VALUES (?, ?, ?, ?, 0, 0, 'by_reference', ?, ?, ?, ?, '', ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		relationChangeOrdinal,
		assertionID,
		"reference-kind:fixture",
		fixture.event.entityID,
		fixture.event.entityID,
		"value-kind:fixture",
		typedMemoryDigest45("5"),
		[]byte("filler:reference-fixture"),
	)
	if err != nil {
		t.Fatalf("insert reference fixture filler: %v", err)
	}
}

func insertTypedMemoryReferenceResolution46(
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	relationChangeOrdinal int,
	resolutionKind string,
	resolutionBasisRef any,
	declarationChangeOrdinal any,
	batchLocalRefOverride any,
) error {
	var localReferenceKindRef any
	var batchLocalRef any
	var declarationDigest any
	var orderedCandidatePrefixDigest any
	if resolutionKind == "same_batch_declaration" {
		localReferenceKindRef = "reference-kind:fixture"
		batchLocalRef = "local:fixture"
		if batchLocalRefOverride != nil {
			batchLocalRef = batchLocalRefOverride
		}
		declarationDigest = typedMemoryDigest45("a")
		orderedCandidatePrefixDigest = typedMemoryDigest45("b")
	}
	_, err := execer.Exec(`INSERT INTO typed_memory_reference_resolution_uses (
		project_id, event_ref, change_ordinal, assertion_id,
		slot_ordinal, filler_ordinal, filler_digest, entity_id,
		resolution_kind, resolution_basis_ref, declaration_change_ordinal,
		local_reference_kind_ref, batch_local_ref, declaration_digest,
		ordered_candidate_prefix_digest,
		resolution_digest, canonical_resolution_bytes
	) VALUES (?, ?, ?, ?, 0, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		relationChangeOrdinal,
		"assertion:reference-fixture",
		typedMemoryDigest45("5"),
		fixture.event.entityID,
		resolutionKind,
		resolutionBasisRef,
		declarationChangeOrdinal,
		localReferenceKindRef,
		batchLocalRef,
		declarationDigest,
		orderedCandidatePrefixDigest,
		typedMemoryDigest45("6"),
		[]byte("resolution:reference-fixture"),
	)
	return err
}

func mustInsertTypedMemoryMemberOfEvaluation46(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	viewKind string,
	observableInputCount int,
) string {
	t.Helper()
	evaluationRef := "evaluation:" + viewKind
	var declarationOrdinal any
	var localReferenceKind any
	var batchLocalRef any
	var declarationDigest any
	var prefixEndOrdinal any
	var prefixDigest any
	if viewKind == "prospective_batch" {
		declarationOrdinal = 0
		localReferenceKind = "reference-kind:fixture"
		batchLocalRef = "local:fixture"
		declarationDigest = typedMemoryDigest45("a")
		prefixEndOrdinal = 1
		prefixDigest = typedMemoryDigest45("b")
	}
	contextSliceRef := "context-slice:" + typedMemoryDigest45("2")
	_, err := execer.Exec(`INSERT INTO typed_memory_memberof_evaluations (
		project_id, event_ref, evaluation_ref, judgement_kind,
		entity_id, value_kind_ref, context_slice_ref,
		evaluator_rule_ref, evaluation_provenance_ref,
		evaluation_view_kind, evaluation_view_digest, canonical_evaluation_view_bytes,
		view_declaration_change_ordinal, view_local_reference_kind_ref,
		view_batch_local_ref, view_declaration_digest,
		view_prefix_end_ordinal, view_ordered_candidate_prefix_digest,
		observable_input_count, observable_input_set_digest,
		query_digest, canonical_query_bytes,
		basis_digest, canonical_basis_bytes,
		judgement_digest, canonical_judgement_bytes
	) VALUES (?, ?, ?, 'member', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		evaluationRef,
		fixture.event.entityID,
		"value-kind:fixture",
		contextSliceRef,
		"rule:fixture",
		"provenance:evaluation",
		viewKind,
		typedMemoryDigest45("3"),
		[]byte("evaluation-view:"+viewKind),
		declarationOrdinal,
		localReferenceKind,
		batchLocalRef,
		declarationDigest,
		prefixEndOrdinal,
		prefixDigest,
		observableInputCount,
		typedMemoryDigest45("4"),
		typedMemoryDigest45("7"),
		[]byte("query:fixture"),
		typedMemoryDigest45("8"),
		[]byte("basis:fixture"),
		typedMemoryDigest45("9"),
		[]byte("judgement:fixture"),
	)
	if err != nil {
		t.Fatalf("insert %s MemberOf evaluation: %v", viewKind, err)
	}
	return evaluationRef
}

func insertTypedMemoryMemberOfUse46(
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	relationChangeOrdinal int,
	evaluationRef string,
) error {
	_, err := execer.Exec(`INSERT INTO typed_memory_relation_filler_memberof_uses (
		project_id, event_ref, change_ordinal, assertion_id,
		slot_ordinal, filler_ordinal, filler_digest,
		use_kind, constraint_id, queried_value_kind_ref,
		query_digest, evaluation_ref, expected_judgement_kind,
		use_digest, canonical_use_bytes
	) VALUES (?, ?, ?, ?, 0, 0, ?, 'required_member', '', ?, ?, ?, 'member', ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		relationChangeOrdinal,
		"assertion:reference-fixture",
		typedMemoryDigest45("5"),
		"value-kind:fixture",
		typedMemoryDigest45("7"),
		evaluationRef,
		typedMemoryDigest45("0"),
		[]byte("memberof-use:fixture"),
	)
	return err
}

func mustInsertTypedMemoryAliasChange46(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	changeOrdinal int,
	aliasChangeRef string,
	changeKind string,
	alias string,
	replacementAlias any,
	supersedesAliasChangeRef string,
) {
	t.Helper()
	if err := insertTypedMemoryAliasChange46(
		execer,
		fixture,
		changeOrdinal,
		aliasChangeRef,
		changeKind,
		alias,
		replacementAlias,
		supersedesAliasChangeRef,
	); err != nil {
		t.Fatalf("insert alias change %s: %v", aliasChangeRef, err)
	}
}

func insertTypedMemoryAliasChange46(
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	changeOrdinal int,
	aliasChangeRef string,
	changeKind string,
	alias string,
	replacementAlias any,
	supersedesAliasChangeRef string,
) error {
	_, err := execer.Exec(`INSERT INTO typed_memory_alias_changes (
		project_id, event_ref, change_ordinal, alias_change_ref,
		change_kind, bounded_context_ref, alias, replacement_alias,
		entity_id, supersedes_alias_change_ref,
		alias_change_digest, canonical_alias_change_bytes, provenance_ref
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		changeOrdinal,
		aliasChangeRef,
		changeKind,
		"bounded-context:fixture",
		alias,
		replacementAlias,
		"entity:fixture",
		supersedesAliasChangeRef,
		typedMemoryDigest45(string(rune('a'+changeOrdinal))),
		[]byte(aliasChangeRef),
		"provenance:fixture",
	)
	return err
}

func typedMemorySchemaObjects46(
	t *testing.T,
	database *sql.DB,
) map[string]string {
	t.Helper()
	rows, err := database.Query(`SELECT type, name, sql FROM sqlite_master
		WHERE name LIKE 'typed_memory_%' OR name LIKE 'idx_typed_memory_%'
		ORDER BY type, name`)
	if err != nil {
		t.Fatalf("read typed-memory schema objects: %v", err)
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var kind string
		var name string
		var sqlText string
		if err := rows.Scan(&kind, &name, &sqlText); err != nil {
			t.Fatalf("scan typed-memory schema object: %v", err)
		}
		result[kind+"/"+name] = sqlText
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read typed-memory schema objects: %v", err)
	}
	return result
}

func typedMemoryStorageColumns46(
	t *testing.T,
	database *sql.DB,
) map[string]map[string]bool {
	t.Helper()
	result := make(map[string]map[string]bool)
	for _, table := range typedMemoryStorageTables46 {
		rows, err := database.Query("PRAGMA table_info(" + quoteSQLiteIdentifier(table) + ")")
		if err != nil {
			t.Fatalf("read v46 table columns for %s: %v", table, err)
		}
		columns := make(map[string]bool)
		for rows.Next() {
			var columnID int
			var name string
			var kind string
			var notNull int
			var defaultValue any
			var primaryKey int
			if err := rows.Scan(
				&columnID,
				&name,
				&kind,
				&notNull,
				&defaultValue,
				&primaryKey,
			); err != nil {
				_ = rows.Close()
				t.Fatalf("scan v46 table column for %s: %v", table, err)
			}
			columns[name] = true
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			t.Fatalf("iterate v46 table columns for %s: %v", table, err)
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close v46 table columns for %s: %v", table, err)
		}
		result[table] = columns
	}
	return result
}

func assertTypedMemoryStorageColumnPresent46(
	t *testing.T,
	columns map[string]map[string]bool,
	table string,
	column string,
) {
	t.Helper()
	if !columns[table][column] {
		t.Fatalf("v46 table %s lacks required column %s", table, column)
	}
}

func assertTypedMemoryStorageColumnAbsent46(
	t *testing.T,
	columns map[string]map[string]bool,
	table string,
	column string,
) {
	t.Helper()
	if columns[table][column] {
		t.Fatalf("v46 table %s retains forbidden opaque column %s", table, column)
	}
}

func assertTypedMemoryEventCompanionCount46(
	t *testing.T,
	database *sql.DB,
	table string,
	eventRef string,
	want int,
) {
	t.Helper()
	query := "SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table) + " WHERE event_ref = ?"
	count := 0
	if err := database.QueryRow(query, eventRef).Scan(&count); err != nil {
		t.Fatalf("count v46 event companions in %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("v46 table %s event %s count = %d, want %d", table, eventRef, count, want)
	}
}
