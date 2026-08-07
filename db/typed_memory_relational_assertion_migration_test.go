package db

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type typedMemoryGraphEventSnapshot53 struct {
	projectID            string
	eventRef             string
	commitRef            string
	eventDigest          string
	expectedRevision     int
	graphRevision        int
	basisTypeEnvRef      string
	resultTypeEnvRef     string
	changeSetDigest      string
	canonicalChangeSet   []byte
	changeCount          int
	eventKind            string
	authorityClass       string
	requestProvenanceRef string
	recordedAt           string
}

type typedMemoryWriterGenerationSnapshot53 struct {
	projectID      string
	eventRef       string
	generation     int
	provenanceKind string
}

type typedMemoryLegacyRelationSnapshot53 struct {
	projectID       string
	eventRef        string
	changeOrdinal   int
	assertionID     string
	signatureRef    string
	contextSliceRef string
	relationDigest  string
	canonicalBytes  []byte
	provenanceRef   string
}

func TestTypedMemoryRelationalAssertionMigration53PreservesV52HistoryByteExactly(
	t *testing.T,
) {
	t.Parallel()

	database, _, fixture := openDatabaseBeforeRelationalAssertion53(t, true)
	defer database.Close()
	eventBefore := loadTypedMemoryGraphEventSnapshot53(t, database, fixture.eventRef)
	witerBefore := loadTypedMemoryWriterGenerationSnapshot53(t, database, fixture.eventRef)
	relationBefore := loadTypedMemoryLegacyRelationSnapshot53(t, database, fixture.eventRef)
	commitClosureBefore := sqliteObjectSQL44(
		t,
		database,
		"trigger",
		"typed_memory_graph_commits_exact_closure",
	)

	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryRelationalAssertionMigration53},
	); err != nil {
		t.Fatalf("migrate v52 database through v53: %v", err)
	}

	eventAfter := loadTypedMemoryGraphEventSnapshot53(t, database, fixture.eventRef)
	writerAfter := loadTypedMemoryWriterGenerationSnapshot53(t, database, fixture.eventRef)
	relationAfter := loadTypedMemoryLegacyRelationSnapshot53(t, database, fixture.eventRef)
	if !reflect.DeepEqual(eventAfter, eventBefore) {
		t.Fatalf("v53 changed historical graph-event bytes:\n before=%+v\n after=%+v", eventBefore, eventAfter)
	}
	if !reflect.DeepEqual(writerAfter, witerBefore) {
		t.Fatalf("v53 changed historical writer-generation row: before=%+v after=%+v", witerBefore, writerAfter)
	}
	if !reflect.DeepEqual(relationAfter, relationBefore) {
		t.Fatalf("v53 changed historical legacy relation bytes: before=%+v after=%+v", relationBefore, relationAfter)
	}
	if writerAfter.generation != 46 || writerAfter.provenanceKind != "writer_v46" {
		t.Fatalf("historical writer = %+v; want exact writer 46 witness", writerAfter)
	}
	assertTypedMemoryWriterGeneration46(t, database)
	assertTypedMemoryWriterCapability53(t, database)
	assertMigrationVersionPresent(t, database, 53)
	assertNoForeignKeyViolationsV38(t, database)
	assertSQLiteIntegrity52(t, database)

	commitClosureAfter := sqliteObjectSQL44(
		t,
		database,
		"trigger",
		"typed_memory_graph_commits_exact_closure",
	)
	if commitClosureAfter == commitClosureBefore {
		t.Fatal("v53 did not add its writer-53 commit-closure branch")
	}
	v52Source, err := typedMemoryGraphCommitExactClosureTrigger52()
	if err != nil {
		t.Fatalf("derive v52 closure source: %v", err)
	}
	marker := "\n\t) BEGIN\n\t\tSELECT RAISE(ABORT"
	index := strings.LastIndex(v52Source, marker)
	if index < 0 || !strings.HasPrefix(commitClosureAfter, v52Source[:index]) {
		t.Fatal("v53 did not preserve the exact v52 normal and identity-reconciliation branches")
	}
}

func TestTypedMemoryRelationalAssertionClosure53AcceptsLegacyAndV3PrefixOwners(
	t *testing.T,
) {
	t.Parallel()

	t.Helper()
	trigger, err := typedMemoryCommitClosureExactFootprintTrigger53()
	if err != nil {
		t.Fatalf("derive v53 exact closure: %v", err)
	}
	legacyBranch := `SELECT 1 FROM typed_memory_reference_resolution_uses resolution_use
						WHERE resolution_use.project_id = prefix.project_id
							AND resolution_use.event_ref = prefix.event_ref
							AND resolution_use.change_ordinal = prefix.prefix_end_ordinal
							AND resolution_use.ordered_candidate_prefix_digest = prefix.prefix_digest`
	if strings.Count(trigger, legacyBranch) != 1 {
		t.Fatal("v53 closure did not preserve one exact legacy prefix-owner branch")
	}
	v3Branch := `FROM typed_memory_relational_assertion_reference_resolution_uses_v3 resolution_use
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
							AND filler.filler_kind = 'by_reference'`
	if strings.Count(trigger, v3Branch) != 1 {
		t.Fatal("v53 closure lacks one exact same-batch v3 prefix-owner branch")
	}
}

func TestTypedMemoryRelationalAssertionClosure53RejectsOrphanCandidatePrefix(
	t *testing.T,
) {
	t.Parallel()

	database, typeEnvRef, _ := openDatabaseBeforeRelationalAssertion53(t, false)
	defer database.Close()
	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryRelationalAssertionMigration53},
	); err != nil {
		t.Fatalf("migrate orphan-prefix fixture through v53: %v", err)
	}
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "orphan-prefix-v53", 0)
	fixture.canonicalSemantic = []byte(`{"change":"assert_relation"}`)
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin orphan-prefix v53 transaction: %v", err)
	}
	defer transaction.Rollback()
	insertTypedMemoryAssertionEvent53(t, transaction, fixture, 1)
	mustInsertTypedMemoryAdmission53(t, transaction, fixture, "snapshot_only")
	contextSliceRef := mustInsertTypedMemoryContextSlice46(
		t,
		transaction,
		fixture,
		"7",
		"bounded-context:orphan-prefix-v53",
		[]byte("context-slice:orphan-prefix-v53"),
	)
	insertTypedMemoryRelationalAssertion53(
		t,
		transaction,
		fixture,
		0,
		"assertion:orphan-prefix-v53",
		contextSliceRef,
		"obtaining_unknown",
	)
	_, err = transaction.Exec(`INSERT INTO typed_memory_relational_assertion_slots_v3 (
		project_id, event_ref, change_ordinal, assertion_id,
		slot_ordinal, slot_kind_ref, slot_digest, canonical_slot_bytes
	) VALUES (?, ?, 0, 'assertion:orphan-prefix-v53', 0, 'slot-kind:v53', ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		typedMemoryDigest45("4"),
		[]byte("slot:orphan-prefix-v53"),
	)
	if err != nil {
		t.Fatalf("insert orphan-prefix v3 slot: %v", err)
	}
	_, err = transaction.Exec(`INSERT INTO typed_memory_value_blobs (
		project_id, event_ref, value_ref, value_kind_ref,
		value_shape_ref, codec_ref, value_digest, canonical_value_bytes
	) VALUES (?, ?, 'value:orphan-prefix-v53', 'value-kind:v53',
		'value-shape:v53', 'codec:v53', ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		typedMemoryDigest45("6"),
		[]byte("value:orphan-prefix-v53"),
	)
	if err != nil {
		t.Fatalf("insert orphan-prefix v3 value blob: %v", err)
	}
	_, err = transaction.Exec(`INSERT INTO typed_memory_relational_assertion_fillers_v3 (
		project_id, event_ref, change_ordinal, assertion_id,
		slot_ordinal, filler_ordinal, filler_kind,
		reference_kind_ref, reference_id, entity_id,
		required_value_kind_ref, value_ref,
		filler_digest, canonical_filler_bytes
	) VALUES (?, ?, 0, 'assertion:orphan-prefix-v53', 0, 0, 'by_value',
		'', '', '', '', 'value:orphan-prefix-v53', ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		typedMemoryDigest45("5"),
		[]byte("filler:orphan-prefix-v53"),
	)
	if err != nil {
		t.Fatalf("insert orphan-prefix v3 filler: %v", err)
	}
	_, err = transaction.Exec(
		"DROP TRIGGER typed_memory_commit_materialization_closures_v46_basis_kind",
	)
	if err != nil {
		t.Fatalf("isolate exact-footprint trigger for orphan-prefix fixture: %v", err)
	}
	_, err = transaction.Exec(`INSERT INTO typed_memory_ordered_candidate_prefixes (
		project_id, event_ref, prefix_end_ordinal, request_digest, prefix_digest
	) VALUES (?, ?, 1, ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		fixture.requestDigest,
		typedMemoryDigest45("b"),
	)
	if err != nil {
		t.Fatalf("insert orphan v53 candidate prefix: %v", err)
	}
	counts := typedMemoryClosureCounts46{
		contextSliceCatalog:    1,
		contextSlice:           1,
		valueBlob:              1,
		relation:               1,
		relationSlot:           1,
		relationFiller:         1,
		orderedCandidatePrefix: 1,
	}
	closureErr := insertTypedMemoryClosure46(
		transaction,
		fixture,
		"snapshot_only",
		counts,
	)
	if closureErr == nil || !strings.Contains(
		closureErr.Error(),
		"exact complete event footprint",
	) {
		t.Fatalf("orphan v53 prefix closure error = %v", closureErr)
	}
}

func TestTypedMemoryRelationalAssertionMigration53RejectsUnknownDDLWithoutMutation(
	t *testing.T,
) {
	t.Parallel()

	database, _, _ := openDatabaseBeforeRelationalAssertion53(t, false)
	defer database.Close()
	graphEventsBefore := sqliteObjectSQL44(t, database, "table", "typed_memory_graph_events")
	writerBefore := sqliteObjectSQL44(t, database, "table", "typed_memory_event_writer_generations")
	commitBefore := sqliteObjectSQL44(t, database, "trigger", "typed_memory_graph_commits_exact_closure")
	_, err := database.Exec(
		`CREATE TABLE typed_memory_relational_assertions_v3 (
			sentinel TEXT PRIMARY KEY
		) WITHOUT ROWID`,
	)
	if err != nil {
		t.Fatalf("install unknown v53 DDL: %v", err)
	}

	err = Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryRelationalAssertionMigration53},
	)
	if err == nil || !strings.Contains(err.Error(), "unknown partial v53 footprint") {
		t.Fatalf("unknown v53 DDL error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 53)
	assertSQLiteSQL53(t, database, "table", "typed_memory_graph_events", graphEventsBefore)
	assertSQLiteSQL53(t, database, "table", "typed_memory_event_writer_generations", writerBefore)
	assertSQLiteSQL53(t, database, "trigger", "typed_memory_graph_commits_exact_closure", commitBefore)
	assertSQLiteObjectAbsent49(t, database, "table", typedMemoryWriterCapabilitiesTable53)
	assertNoForeignKeyViolationsV38(t, database)
}

func TestTypedMemoryRelationalAssertionMigration53RollsBackInjectedFailure(
	t *testing.T,
) {
	t.Parallel()

	database, _, fixture := openDatabaseBeforeRelationalAssertion53(t, true)
	defer database.Close()
	eventBefore := loadTypedMemoryGraphEventSnapshot53(t, database, fixture.eventRef)
	writerBefore := loadTypedMemoryWriterGenerationSnapshot53(t, database, fixture.eventRef)
	injected := errors.New("injected v53 post-install failure")
	migration := typedMemoryRelationalAssertionMigration53
	migration.Apply = func(tx MigrationTransaction, migrations []Migration) error {
		if err := applyTypedMemoryRelationalAssertionMigration53(tx, migrations); err != nil {
			return err
		}
		return injected
	}

	err := Migrate(database, "schema_version", []Migration{migration})
	if !errors.Is(err, injected) {
		t.Fatalf("injected v53 failure = %v; want injected error", err)
	}
	assertMigrationVersionAbsent(t, database, 53)
	if got := loadTypedMemoryGraphEventSnapshot53(t, database, fixture.eventRef); !reflect.DeepEqual(got, eventBefore) {
		t.Fatalf("rolled-back v53 changed graph event: before=%+v after=%+v", eventBefore, got)
	}
	if got := loadTypedMemoryWriterGenerationSnapshot53(t, database, fixture.eventRef); !reflect.DeepEqual(got, writerBefore) {
		t.Fatalf("rolled-back v53 changed writer row: before=%+v after=%+v", writerBefore, got)
	}
	assertSQLiteObjectAbsent49(t, database, "table", typedMemoryWriterCapabilitiesTable53)
	assertSQLiteObjectAbsent49(t, database, "table", typedMemoryRelationalAssertionSlotsTable53)
	assertNoForeignKeyViolationsV38(t, database)
	assertSQLiteIntegrity52(t, database)
}

func TestTypedMemoryRelationalAssertionMigration53SealsModalityAndCrossLaneIdentity(
	t *testing.T,
) {
	t.Parallel()

	database, typeEnvRef, _ := openDatabaseBeforeRelationalAssertion53(t, false)
	defer database.Close()
	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryRelationalAssertionMigration53},
	); err != nil {
		t.Fatalf("migrate modality fixture through v53: %v", err)
	}
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "assertion-v53", 0)
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin v53 modality transaction: %v", err)
	}
	defer transaction.Rollback()
	insertTypedMemoryAssertionEvent53(t, transaction, fixture, 4)
	_, err = transaction.Exec(`INSERT INTO typed_memory_event_writer_generations (
		project_id, event_ref, writer_generation, provenance_kind
	) VALUES (?, ?, 53, 'writer_v53')`, fixture.event.projectID, fixture.event.eventRef)
	if err != nil {
		t.Fatalf("insert writer-53 event coordinate: %v", err)
	}
	contextSliceRef := mustInsertTypedMemoryContextSlice46(
		t,
		transaction,
		fixture,
		"7",
		"bounded-context:assertion-v53",
		[]byte("context-slice:assertion-v53"),
	)

	for ordinal, modality := range []string{
		"affirms_obtaining",
		"denies_obtaining",
		"obtaining_unknown",
	} {
		insertTypedMemoryRelationalAssertion53(
			t,
			transaction,
			fixture,
			ordinal,
			"assertion:modality:"+modality,
			contextSliceRef,
			modality,
		)
	}
	_, invalidErr := transaction.Exec(`INSERT INTO typed_memory_relational_assertions_v3 (
		project_id, event_ref, change_ordinal, assertion_id,
		signature_ref, context_slice_ref, modality,
		assertion_digest, canonical_assertion_bytes, provenance_ref
	) VALUES (?, ?, 3, 'assertion:invalid-modality',
		'signature:v53', ?, 'maybe_obtaining', ?, ?, 'provenance:v53')`,
		fixture.event.projectID,
		fixture.event.eventRef,
		contextSliceRef,
		typedMemoryDigest45("8"),
		[]byte("invalid-modality"),
	)
	if invalidErr == nil || !strings.Contains(invalidErr.Error(), "CHECK constraint failed") {
		t.Fatalf("invalid modality error = %v; want closed-union CHECK rejection", invalidErr)
	}

	_, err = transaction.Exec("DROP TRIGGER typed_memory_relation_instances_v53_legacy_insert_frozen")
	if err != nil {
		t.Fatalf("open legacy lane for corruption fixture: %v", err)
	}
	_, err = transaction.Exec(`INSERT INTO typed_memory_relation_instances (
		project_id, event_ref, change_ordinal, assertion_id,
		signature_ref, context_slice_ref, relation_digest,
		canonical_relation_bytes, provenance_ref
	) VALUES (?, ?, 3, 'assertion:cross-lane',
		'signature:legacy', ?, ?, ?, 'provenance:legacy')`,
		fixture.event.projectID,
		fixture.event.eventRef,
		contextSliceRef,
		typedMemoryDigest45("9"),
		[]byte("legacy-relation"),
	)
	if err != nil {
		t.Fatalf("seed cross-lane legacy corruption fixture: %v", err)
	}
	_, duplicateErr := transaction.Exec(`INSERT INTO typed_memory_relational_assertions_v3 (
		project_id, event_ref, change_ordinal, assertion_id,
		signature_ref, context_slice_ref, modality,
		assertion_digest, canonical_assertion_bytes, provenance_ref
	) VALUES (?, ?, 3, 'assertion:cross-lane',
		'signature:v53', ?, 'obtaining_unknown', ?, ?, 'provenance:v53')`,
		fixture.event.projectID,
		fixture.event.eventRef,
		contextSliceRef,
		typedMemoryDigest45("0"),
		[]byte("v3-duplicate"),
	)
	if duplicateErr == nil || !strings.Contains(duplicateErr.Error(), "legacy relation lane") {
		t.Fatalf("cross-lane duplicate error = %v; want explicit DB guard", duplicateErr)
	}
}

func openDatabaseBeforeRelationalAssertion53(
	t *testing.T,
	withHistory bool,
) (*sql.DB, string, typedMemoryDeclarationFixture45) {
	t.Helper()
	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	fixture := newTypedMemoryAdmissionFixture46(typeEnvRef, "preserved-v46", 0)
	if withHistory {
		insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
		commitTypedMemoryLegacyRelation53(t, database, fixture)
	}
	migrations := migrationsBeforeVersion(
		kernelMigrations,
		typedMemoryRelationalAssertionSchemaVersion53,
		0,
		nil,
	)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate fixture database through v52: %v", err)
	}
	assertMigrationVersionPresent(t, database, 52)
	assertMigrationVersionAbsent(t, database, 53)
	return database, typeEnvRef, fixture.event
}

func commitTypedMemoryLegacyRelation53(
	t *testing.T,
	database *sql.DB,
	fixture typedMemoryAdmissionFixture46,
) {
	t.Helper()
	fixture.canonicalSemantic = []byte(`{"change":"instantiate_relation"}`)
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin legacy relation fixture: %v", err)
	}
	_, err = transaction.Exec(`INSERT INTO typed_memory_graph_events (
		project_id, event_ref, commit_ref, event_digest,
		expected_revision, graph_revision,
		basis_type_env_ref, result_type_env_ref,
		change_set_digest, canonical_change_set_bytes, change_count,
		event_kind, authority_class, request_provenance_ref, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1,
		'instantiate_relation', 'non_binding_semantic_assertion', ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		fixture.event.commitRef,
		fixture.event.eventDigest,
		fixture.event.expectedRevision,
		fixture.event.graphRevision,
		fixture.event.typeEnvRef,
		fixture.event.typeEnvRef,
		fixture.event.changeSetDigest,
		fixture.canonicalSemantic,
		"request-provenance:"+fixture.event.eventRef,
		typedMemoryRecordedAt46,
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert legacy relation event: %v", err)
	}
	mustInsertTypedMemoryAdmission46(t, transaction, fixture, "context_slice_membership")
	mustInsertTypedMemoryReferenceFiller46(t, transaction, fixture, 0)
	if err := insertTypedMemoryReferenceResolution46(
		transaction,
		fixture,
		0,
		"snapshot_reference",
		"snapshot-basis:fixture",
		nil,
		nil,
	); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert legacy relation resolution: %v", err)
	}
	evaluationRef := mustInsertTypedMemoryMemberOfEvaluation46(
		t,
		transaction,
		fixture,
		"persisted_snapshot",
		1,
	)
	_, err = transaction.Exec(`INSERT INTO typed_memory_observable_input_blobs (
		project_id, event_ref, observable_input_ref,
		observable_input_digest, canonical_observable_input_bytes
	) VALUES (?, ?, 'observable:legacy-v2', ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		typedMemoryDigest45("d"),
		[]byte("observable:legacy-v2"),
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert legacy relation observable blob: %v", err)
	}
	_, err = transaction.Exec(`INSERT INTO typed_memory_memberof_observable_inputs (
		project_id, event_ref, evaluation_ref, input_ordinal,
		observable_input_ref, observable_input_digest
	) VALUES (?, ?, ?, 0, 'observable:legacy-v2', ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		evaluationRef,
		typedMemoryDigest45("d"),
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert legacy relation observable input: %v", err)
	}
	if err := insertTypedMemoryMemberOfUse46(transaction, fixture, 0, evaluationRef); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert legacy relation MemberOf use: %v", err)
	}
	_, err = transaction.Exec(`INSERT INTO typed_memory_idempotency_history (
		project_id, idempotency_key, change_set_digest,
		event_ref, graph_revision, result_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		fixture.event.projectID,
		fixture.event.idempotencyKey,
		fixture.event.changeSetDigest,
		fixture.event.eventRef,
		fixture.event.graphRevision,
		fixture.event.eventDigest,
		typedMemoryRecordedAt46,
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert legacy relation idempotency row: %v", err)
	}
	_, err = transaction.Exec(`INSERT INTO typed_memory_projection_jobs (
		project_id, projection_job_ref, semantic_event_ref,
		graph_revision, target_kind, input_event_digest, recorded_at
	) VALUES (?, ?, ?, ?, 'project_carriers', ?, ?)`,
		fixture.event.projectID,
		fixture.event.projectionJobRef,
		fixture.event.eventRef,
		fixture.event.graphRevision,
		fixture.event.eventDigest,
		typedMemoryRecordedAt46,
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert legacy relation projection job: %v", err)
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
	mustInsertTypedMemoryClosure46(
		t,
		transaction,
		fixture,
		"context_slice_membership",
		counts,
	)
	if err := insertTypedMemoryGraphCommitCounts46(transaction, fixture.event, 0, 0); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert legacy relation graph commit: %v", err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit legacy relation fixture: %v", err)
	}
}

func loadTypedMemoryGraphEventSnapshot53(
	t *testing.T,
	database *sql.DB,
	eventRef string,
) typedMemoryGraphEventSnapshot53 {
	t.Helper()
	var snapshot typedMemoryGraphEventSnapshot53
	err := database.QueryRow(`SELECT
		project_id, event_ref, commit_ref, event_digest,
		expected_revision, graph_revision,
		basis_type_env_ref, result_type_env_ref,
		change_set_digest, canonical_change_set_bytes,
		change_count, event_kind, authority_class,
		request_provenance_ref, recorded_at
	FROM typed_memory_graph_events WHERE event_ref = ?`, eventRef).Scan(
		&snapshot.projectID,
		&snapshot.eventRef,
		&snapshot.commitRef,
		&snapshot.eventDigest,
		&snapshot.expectedRevision,
		&snapshot.graphRevision,
		&snapshot.basisTypeEnvRef,
		&snapshot.resultTypeEnvRef,
		&snapshot.changeSetDigest,
		&snapshot.canonicalChangeSet,
		&snapshot.changeCount,
		&snapshot.eventKind,
		&snapshot.authorityClass,
		&snapshot.requestProvenanceRef,
		&snapshot.recordedAt,
	)
	if err != nil {
		t.Fatalf("load graph-event snapshot %s: %v", eventRef, err)
	}
	return snapshot
}

func loadTypedMemoryWriterGenerationSnapshot53(
	t *testing.T,
	database *sql.DB,
	eventRef string,
) typedMemoryWriterGenerationSnapshot53 {
	t.Helper()
	var snapshot typedMemoryWriterGenerationSnapshot53
	err := database.QueryRow(`SELECT
		project_id, event_ref, writer_generation, provenance_kind
	FROM typed_memory_event_writer_generations WHERE event_ref = ?`, eventRef).Scan(
		&snapshot.projectID,
		&snapshot.eventRef,
		&snapshot.generation,
		&snapshot.provenanceKind,
	)
	if err != nil {
		t.Fatalf("load writer-generation snapshot %s: %v", eventRef, err)
	}
	return snapshot
}

func loadTypedMemoryLegacyRelationSnapshot53(
	t *testing.T,
	database *sql.DB,
	eventRef string,
) typedMemoryLegacyRelationSnapshot53 {
	t.Helper()
	var snapshot typedMemoryLegacyRelationSnapshot53
	err := database.QueryRow(`SELECT
		project_id, event_ref, change_ordinal, assertion_id,
		signature_ref, context_slice_ref, relation_digest,
		canonical_relation_bytes, provenance_ref
	FROM typed_memory_relation_instances WHERE event_ref = ?`, eventRef).Scan(
		&snapshot.projectID,
		&snapshot.eventRef,
		&snapshot.changeOrdinal,
		&snapshot.assertionID,
		&snapshot.signatureRef,
		&snapshot.contextSliceRef,
		&snapshot.relationDigest,
		&snapshot.canonicalBytes,
		&snapshot.provenanceRef,
	)
	if err != nil {
		t.Fatalf("load legacy relation snapshot %s: %v", eventRef, err)
	}
	return snapshot
}

func assertTypedMemoryWriterCapability53(t *testing.T, database *sql.DB) {
	t.Helper()
	var generation int
	var digest string
	var canonical []byte
	err := database.QueryRow(`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_writer_capabilities_v53 WHERE capability_key = ?`,
		typedMemoryWriterGenerationCapability53,
	).Scan(&generation, &digest, &canonical)
	if err != nil {
		t.Fatalf("read v53 writer capability: %v", err)
	}
	if generation != typedMemoryWriterGeneration53 ||
		digest != typedMemoryWriterMarkerDigest53 ||
		string(canonical) != typedMemoryWriterMarkerBytes53 {
		t.Fatalf("v53 writer capability = %d %q %q", generation, digest, canonical)
	}
}

func assertSQLiteSQL53(
	t *testing.T,
	database *sql.DB,
	kind string,
	name string,
	want string,
) {
	t.Helper()
	got := sqliteObjectSQL44(t, database, kind, name)
	if got != want {
		t.Fatalf("SQLite %s %s changed after rejected v53 migration", kind, name)
	}
}

func insertTypedMemoryAssertionEvent53(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	changeCount int,
) {
	t.Helper()
	_, err := execer.Exec(`INSERT INTO typed_memory_graph_events (
		project_id, event_ref, commit_ref, event_digest,
		expected_revision, graph_revision,
		basis_type_env_ref, result_type_env_ref,
		change_set_digest, canonical_change_set_bytes, change_count,
		event_kind, authority_class, request_provenance_ref, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		'assert_relation', 'non_binding_semantic_assertion', ?, ?)`,
		fixture.event.projectID,
		fixture.event.eventRef,
		fixture.event.commitRef,
		fixture.event.eventDigest,
		fixture.event.expectedRevision,
		fixture.event.graphRevision,
		fixture.event.typeEnvRef,
		fixture.event.typeEnvRef,
		fixture.event.changeSetDigest,
		[]byte(`{"change":"assert_relation"}`),
		changeCount,
		"request-provenance:"+fixture.event.eventRef,
		typedMemoryRecordedAt46,
	)
	if err != nil {
		t.Fatalf("insert writer-53 assertion event: %v", err)
	}
}

func mustInsertTypedMemoryAdmission53(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	basisKind string,
) {
	t.Helper()
	_, err := execer.Exec(`INSERT INTO typed_memory_event_writer_generations (
		project_id, event_ref, writer_generation, provenance_kind
	) VALUES (?, ?, 53, 'writer_v53')`,
		fixture.event.projectID,
		fixture.event.eventRef,
	)
	if err != nil {
		t.Fatalf("insert v53 event writer generation: %v", err)
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
		t.Fatalf("insert v53 event admission basis: %v", err)
	}
}

func insertTypedMemoryRelationalAssertion53(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryAdmissionFixture46,
	changeOrdinal int,
	assertionID string,
	contextSliceRef string,
	modality string,
) {
	t.Helper()
	seed := string(rune('a' + changeOrdinal))
	_, err := execer.Exec(`INSERT INTO typed_memory_relational_assertions_v3 (
		project_id, event_ref, change_ordinal, assertion_id,
		signature_ref, context_slice_ref, modality,
		assertion_digest, canonical_assertion_bytes, provenance_ref
	) VALUES (?, ?, ?, ?, 'signature:v53', ?, ?, ?, ?, 'provenance:v53')`,
		fixture.event.projectID,
		fixture.event.eventRef,
		changeOrdinal,
		assertionID,
		contextSliceRef,
		modality,
		typedMemoryDigest45(seed),
		[]byte("assertion:"+modality),
	)
	if err != nil {
		t.Fatalf("insert %s relational assertion: %v", modality, err)
	}
}
