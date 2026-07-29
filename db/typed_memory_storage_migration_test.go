package db

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

const (
	typedMemoryProjectID45  = "qnt_a7f3b2c1"
	typedMemoryRecordedAt45 = "2026-07-16T12:00:00Z"
)

type typedMemoryDeclarationFixture45 struct {
	projectID        string
	eventRef         string
	commitRef        string
	entityID         string
	boundedContext   string
	idempotencyKey   string
	projectionJobRef string
	eventDigest      string
	changeSetDigest  string
	typeEnvRef       string
	expectedRevision int
	graphRevision    int
}

type typedMemoryExecer45 interface {
	Exec(query string, args ...any) (sql.Result, error)
}

type typedMemoryConnectionExecer45 struct {
	context    context.Context
	connection *sql.Conn
}

func (execer typedMemoryConnectionExecer45) Exec(
	query string,
	args ...any,
) (sql.Result, error) {
	return execer.connection.ExecContext(execer.context, query, args...)
}

func TestTypedMemoryStorageMigration45InstallsEmptyTransactionalSchema(t *testing.T) {
	store := newStoreThroughTypedMemoryStorageMigration45(t)
	defer store.Close()

	assertMigrationVersionPresent(t, store.conn, 45)
	assertMigrationVersionAbsent(t, store.conn, 46)
	for _, table := range typedMemoryStorageTables45 {
		assertSQLiteObjectExists(t, store.conn, "table", table)
		assertTypedMemoryTableRowCount45(t, store.conn, table, 0)
	}
	for _, trigger := range []string{
		"typed_memory_type_env_snapshots_no_replace",
		"typed_memory_type_env_snapshots_no_update",
		"typed_memory_graph_heads_genesis_only",
		"typed_memory_graph_heads_revision_cas",
		"typed_memory_graph_events_exact_head",
		"typed_memory_entities_exact_event",
		"typed_memory_entity_contexts_exact_event",
		"typed_memory_idempotency_exact_event",
		"typed_memory_projection_jobs_exact_event",
		"typed_memory_graph_commits_exact_closure",
		"typed_memory_graph_commits_advance_head",
		"typed_memory_projection_debt_exact_job",
		"typed_memory_projection_debt_resolution_chain",
	} {
		assertSQLiteObjectExists(t, store.conn, "trigger", trigger)
	}
	assertTypedMemoryForeignKeysClean45(t, store.conn)
}

func TestTypedMemoryStorageMigration45RejectsUnknownPartialFootprintAtomically(t *testing.T) {
	database := openDatabaseBeforeTypedMemoryStorageMigration45(t)
	defer database.Close()
	partialTable := "typed_memory_graph_heads"

	_, err := database.Exec(
		"CREATE TABLE " + quoteSQLiteIdentifier(partialTable) + " (unknown TEXT)",
	)
	if err != nil {
		t.Fatalf("seed unknown partial v45 footprint: %v", err)
	}
	err = Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryStorageMigration45},
	)
	if err == nil || !strings.Contains(err.Error(), "unknown partial schema") {
		t.Fatalf("partial v45 footprint error = %v", err)
	}

	assertMigrationVersionAbsent(t, database, 45)
	assertSQLiteObjectExists(t, database, "table", partialTable)
	for _, table := range typedMemoryStorageTables45 {
		if table == partialTable {
			continue
		}
		assertSQLiteObjectAbsent(t, database, "table", table)
	}
	var otherObjectCount int
	err = database.QueryRow(`SELECT COUNT(*) FROM sqlite_master
		WHERE (name LIKE 'typed_memory_%' OR name LIKE 'idx_typed_memory_%')
			AND NOT (type = 'table' AND name = ?)`, partialTable).Scan(&otherObjectCount)
	if err != nil {
		t.Fatalf("inspect residual v45 objects after atomic refusal: %v", err)
	}
	if otherObjectCount != 0 {
		t.Fatalf("failed v45 migration left %d object(s) beyond the seeded partial table", otherObjectCount)
	}
}

func TestTypedMemoryStorageMigration45RequiresV44Source(t *testing.T) {
	database := openDatabaseBeforeProfileAuthorityAdmissionV2Migration44(t)
	defer database.Close()

	err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryStorageMigration45},
	)
	if err == nil || !strings.Contains(err.Error(), "requires schema version 44") {
		t.Fatalf("missing v44 source error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 44)
	assertMigrationVersionAbsent(t, database, 45)
	for _, table := range typedMemoryStorageTables45 {
		assertSQLiteObjectAbsent(t, database, "table", table)
	}
}

func TestTypedMemoryStorageMigration45RejectsFabricatedGenesisHead(t *testing.T) {
	store, typeEnvRef := newTypedMemoryRawSQLStore45(t)
	defer store.Close()

	_, err := store.conn.Exec(`INSERT INTO typed_memory_graph_heads (
		project_id, graph_revision, active_type_env_ref,
		last_event_ref, last_commit_ref, updated_at
	) VALUES (?, 1, ?, 'event:fabricated', 'commit:fabricated', ?)`,
		typedMemoryProjectID45,
		typeEnvRef,
		typedMemoryRecordedAt45,
	)
	if err == nil || !strings.Contains(err.Error(), "must begin at revision zero") {
		t.Fatalf("fabricated genesis head error = %v", err)
	}
	assertTypedMemoryTableRowCount45(t, store.conn, "typed_memory_graph_heads", 0)
}

func TestTypedMemoryStorageMigration45RejectsHeadForUnboundProject(t *testing.T) {
	store, typeEnvRef := newTypedMemoryRawSQLStore45(t)
	defer store.Close()

	_, err := store.conn.Exec(`INSERT INTO typed_memory_graph_heads (
		project_id, graph_revision, active_type_env_ref,
		last_event_ref, last_commit_ref, updated_at
	) VALUES (?, 0, ?, '', '', ?)`,
		"qnt_deadbeef",
		typeEnvRef,
		typedMemoryRecordedAt45,
	)
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		t.Fatalf("unbound project head error = %v", err)
	}
	assertTypedMemoryTableRowCount45(t, store.conn, "typed_memory_graph_heads", 0)
}

func TestTypedMemoryStorageMigration45RejectsEventOnlyTransactionAtCommit(t *testing.T) {
	store, typeEnvRef := newTypedMemoryRawSQLStore45(t)
	defer store.Close()
	insertTypedMemoryGenesisHead45(t, store.conn, typeEnvRef)
	fixture := newTypedMemoryDeclarationFixture45("orphan", "b", "c", typeEnvRef, 0)

	transactionContext := context.Background()
	connection, err := store.conn.Conn(transactionContext)
	if err != nil {
		t.Fatalf("reserve event-only transaction connection: %v", err)
	}
	defer connection.Close()
	execer := typedMemoryConnectionExecer45{
		context:    transactionContext,
		connection: connection,
	}
	if _, err := execer.Exec("BEGIN IMMEDIATE"); err != nil {
		t.Fatalf("begin event-only transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, execer, fixture, 1)
	_, commitErr := execer.Exec("COMMIT")
	if commitErr == nil || !strings.Contains(commitErr.Error(), "FOREIGN KEY constraint failed") {
		t.Fatalf("event-only deferred-FK commit error = %v", commitErr)
	}
	if _, err := execer.Exec("ROLLBACK"); err != nil {
		t.Fatalf("roll back event-only transaction after deferred-FK refusal: %v", err)
	}

	assertTypedMemoryTableRowCount45(t, store.conn, "typed_memory_graph_events", 0)
	assertTypedMemoryHeadRevision45(t, store.conn, 0)
}

func TestTypedMemoryStorageMigration45RejectsEntityContextFromDifferentEvent(t *testing.T) {
	store, typeEnvRef := newTypedMemoryRawSQLStore45(t)
	defer store.Close()
	insertTypedMemoryGenesisHead45(t, store.conn, typeEnvRef)
	declared := newTypedMemoryDeclarationFixture45("declared", "d", "e", typeEnvRef, 0)
	commitTypedMemoryDeclaration45(t, store.conn, declared)

	other := newTypedMemoryDeclarationFixture45("other", "f", "1", typeEnvRef, 1)
	transaction, err := store.conn.Begin()
	if err != nil {
		t.Fatalf("begin mismatched-context transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, other, 1)
	_, insertErr := transaction.Exec(`INSERT INTO typed_memory_entity_contexts (
		project_id, entity_id, bounded_context_ref, label, provenance_ref,
		declared_event_ref, declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		declared.projectID,
		declared.entityID,
		"bounded-context:wrong-event",
		"Wrong event",
		"provenance:wrong-event",
		other.eventRef,
		other.graphRevision,
		typedMemoryRecordedAt45,
	)
	if insertErr == nil || !strings.Contains(insertErr.Error(), "does not match its declaration event") {
		t.Fatalf("mismatched entity-context event error = %v", insertErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back mismatched-context transaction: %v", err)
	}
	assertTypedMemoryHeadRevision45(t, store.conn, 1)
}

func TestTypedMemoryStorageMigration45RejectsMaterializationAfterCommitClosure(t *testing.T) {
	store, typeEnvRef := newTypedMemoryRawSQLStore45(t)
	defer store.Close()
	insertTypedMemoryGenesisHead45(t, store.conn, typeEnvRef)
	fixture := newTypedMemoryDeclarationFixture45("closed", "4", "5", typeEnvRef, 0)
	commitTypedMemoryDeclaration45(t, store.conn, fixture)
	assertTypedMemoryTableRowCount45(t, store.conn, "typed_memory_entities", 1)
	assertTypedMemoryTableRowCount45(t, store.conn, "typed_memory_entity_contexts", 1)

	_, entityErr := store.conn.Exec(`INSERT INTO typed_memory_entities (
		project_id, entity_id, first_declared_event_ref,
		first_declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?)`,
		fixture.projectID,
		"entity:late",
		fixture.eventRef,
		fixture.graphRevision,
		typedMemoryRecordedAt45,
	)
	if entityErr == nil || !strings.Contains(entityErr.Error(), "does not match its declaration event") {
		t.Fatalf("closed-event entity materialization error = %v", entityErr)
	}
	_, contextErr := store.conn.Exec(`INSERT INTO typed_memory_entity_contexts (
		project_id, entity_id, bounded_context_ref, label, provenance_ref,
		declared_event_ref, declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.projectID,
		fixture.entityID,
		"bounded-context:late",
		"Late context",
		"provenance:late-context",
		fixture.eventRef,
		fixture.graphRevision,
		typedMemoryRecordedAt45,
	)
	if contextErr == nil || !strings.Contains(contextErr.Error(), "does not match its declaration event") {
		t.Fatalf("closed-event entity-context materialization error = %v", contextErr)
	}

	assertTypedMemoryTableRowCount45(t, store.conn, "typed_memory_entities", 1)
	assertTypedMemoryTableRowCount45(t, store.conn, "typed_memory_entity_contexts", 1)
	assertTypedMemoryHeadRevision45(t, store.conn, 1)
}

func TestTypedMemoryStorageMigration45RejectsCommitWhoseChangeCountIsNotOne(t *testing.T) {
	store, typeEnvRef := newTypedMemoryRawSQLStore45(t)
	defer store.Close()
	insertTypedMemoryGenesisHead45(t, store.conn, typeEnvRef)
	fixture := newTypedMemoryDeclarationFixture45("multi", "2", "3", typeEnvRef, 0)

	transaction, err := store.conn.Begin()
	if err != nil {
		t.Fatalf("begin multi-change transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture, 2)
	mustInsertTypedMemoryDeclarationMaterialization45(t, transaction, fixture)
	commitErr := insertTypedMemoryGraphCommit45(transaction, fixture)
	if commitErr == nil || !strings.Contains(commitErr.Error(), "lacks its exact event") {
		t.Fatalf("multi-change final commit error = %v", commitErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back multi-change transaction: %v", err)
	}
	assertTypedMemoryTableRowCount45(t, store.conn, "typed_memory_graph_commits", 0)
	assertTypedMemoryHeadRevision45(t, store.conn, 0)
}

func TestTypedMemoryStorageMigration45TypeEnvSnapshotsAreImmutable(t *testing.T) {
	store, typeEnvRef := newTypedMemoryRawSQLStore45(t)
	defer store.Close()
	originalBytes := []byte("canonical-type-env-v1")

	_, replaceErr := store.conn.Exec(`INSERT OR REPLACE INTO typed_memory_type_env_snapshots (
		type_env_ref, artifact_digest, snapshot_format, canonical_bytes,
		source_revision, compiler_schema_version, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		typeEnvRef,
		strings.TrimPrefix(typeEnvRef, "typeenv:"),
		"haft.typed-memory.type-env/v1",
		[]byte("conflicting-type-env-bytes"),
		"source-revision:conflict",
		"compiler-schema:v1",
		typedMemoryRecordedAt45,
	)
	if replaceErr == nil || !strings.Contains(replaceErr.Error(), "snapshots are immutable") {
		t.Fatalf("conflicting snapshot replacement error = %v", replaceErr)
	}
	_, updateErr := store.conn.Exec(
		"UPDATE typed_memory_type_env_snapshots SET canonical_bytes = ? WHERE type_env_ref = ?",
		[]byte("updated-type-env-bytes"),
		typeEnvRef,
	)
	if updateErr == nil || !strings.Contains(updateErr.Error(), "history is append-only") {
		t.Fatalf("snapshot update error = %v", updateErr)
	}

	var storedBytes []byte
	err := store.conn.QueryRow(
		"SELECT canonical_bytes FROM typed_memory_type_env_snapshots WHERE type_env_ref = ?",
		typeEnvRef,
	).Scan(&storedBytes)
	if err != nil {
		t.Fatalf("read immutable snapshot bytes: %v", err)
	}
	if !bytes.Equal(storedBytes, originalBytes) {
		t.Fatalf("immutable snapshot bytes changed: got %x want %x", storedBytes, originalBytes)
	}
}

func TestTypedMemoryStorageMigration45PreservesV44LegacyRowsAndBytes(t *testing.T) {
	database := openDatabaseBeforeTypedMemoryStorageMigration45(t)
	defer database.Close()
	legacyVector := []byte{0x00, 0x7f, 0x80, 0xff, 0x10, 0x20, 0x30, 0x40}

	_, err := database.Exec(`INSERT INTO artifacts (
		id, kind, title, content, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?)`,
		"artifact:legacy-v44",
		"Note",
		"Legacy v44 row",
		"must survive typed-memory storage migration",
		typedMemoryRecordedAt45,
		typedMemoryRecordedAt45,
	)
	if err != nil {
		t.Fatalf("seed legacy v44 artifact row: %v", err)
	}
	_, err = database.Exec(`INSERT INTO artifact_embeddings (
		artifact_id, provider, model, dim, content_hash, vector, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"artifact:legacy-v44",
		"fixture-provider",
		"fixture-model",
		2,
		"legacy-content-hash",
		legacyVector,
		typedMemoryRecordedAt45,
	)
	if err != nil {
		t.Fatalf("seed legacy v44 vector bytes: %v", err)
	}

	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryStorageMigration45},
	); err != nil {
		t.Fatalf("migrate v44 database through v45: %v", err)
	}
	assertMigrationVersionPresent(t, database, 45)

	var title string
	var content string
	err = database.QueryRow(
		"SELECT title, content FROM artifacts WHERE id = ?",
		"artifact:legacy-v44",
	).Scan(&title, &content)
	if err != nil {
		t.Fatalf("read preserved legacy artifact row: %v", err)
	}
	if title != "Legacy v44 row" || content != "must survive typed-memory storage migration" {
		t.Fatalf("legacy artifact changed: title=%q content=%q", title, content)
	}
	var storedVector []byte
	err = database.QueryRow(`SELECT vector FROM artifact_embeddings
		WHERE artifact_id = ? AND provider = ? AND model = ? AND dim = ?`,
		"artifact:legacy-v44",
		"fixture-provider",
		"fixture-model",
		2,
	).Scan(&storedVector)
	if err != nil {
		t.Fatalf("read preserved legacy vector bytes: %v", err)
	}
	if !bytes.Equal(storedVector, legacyVector) {
		t.Fatalf("legacy vector bytes changed: got %x want %x", storedVector, legacyVector)
	}
	for _, table := range typedMemoryStorageTables45 {
		assertTypedMemoryTableRowCount45(t, database, table, 0)
	}
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestSchema44UpgradePreservesHistoricalDecisionSpecSectionRelation(
	t *testing.T,
) {
	database := openDatabaseBeforeTypedMemoryStorageMigration45(t)
	defer database.Close()
	database.SetMaxOpenConns(1)

	structuredData := `{"section_refs":["ES.retired.001","ES.retired.001"],"claim":"historical relation remains readable"}`
	_, err := database.Exec(`
		INSERT INTO artifacts (
			id, kind, version, status, context, mode, title, content,
			created_at, updated_at, structured_data
		) VALUES (
			'dec-historical-spec-relation', 'DecisionRecord', 1, 'active',
			'migration-compatibility', 'standard',
			'Historical SpecSection relation', 'preserve exact decision bytes',
			'2026-07-18T00:00:00Z', '2026-07-18T00:00:00Z', ?
		)`,
		structuredData,
	)
	if err != nil {
		t.Fatalf("seed historical DecisionRecord: %v", err)
	}
	if _, err := database.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatalf("disable foreign keys for historical polymorphic relation: %v", err)
	}
	_, linkErr := database.Exec(`
		INSERT INTO artifact_links (
			source_id, target_id, link_type, created_at
		) VALUES (
			'dec-historical-spec-relation', 'ES.retired.001',
			'governs', '2026-07-18T00:00:00Z'
		)`)
	if _, err := database.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("restore foreign keys after historical polymorphic relation: %v", err)
	}
	if linkErr != nil {
		t.Fatalf("seed historical polymorphic relation: %v", linkErr)
	}

	if err := RunMigrations(database); err != nil {
		t.Fatalf("upgrade schema-44 database with historical SpecSection relation: %v", err)
	}
	assertMigrationVersionPresent(t, database, 49)

	var linkCount int
	err = database.QueryRow(`
		SELECT COUNT(*)
		FROM artifact_links
		WHERE source_id = 'dec-historical-spec-relation'
			AND target_id = 'ES.retired.001'
			AND link_type = 'governs'`,
	).Scan(&linkCount)
	if err != nil {
		t.Fatalf("read preserved historical relation: %v", err)
	}
	if linkCount != 1 {
		t.Fatalf("preserved historical relation count = %d, want 1", linkCount)
	}

	var storedStructuredData string
	err = database.QueryRow(
		"SELECT structured_data FROM artifacts WHERE id = 'dec-historical-spec-relation'",
	).Scan(&storedStructuredData)
	if err != nil {
		t.Fatalf("read preserved DecisionRecord bytes: %v", err)
	}
	if storedStructuredData != structuredData {
		t.Fatalf(
			"DecisionRecord structured_data = %q, want exact %q",
			storedStructuredData,
			structuredData,
		)
	}
	if err := verifyForeignKeys(database); err != nil {
		t.Fatalf("semantic foreign-key verification rejected historical relation: %v", err)
	}
}

func newTypedMemoryRawSQLStore45(t *testing.T) (*Store, string) {
	t.Helper()
	store := newStoreThroughTypedMemoryStorageMigration45(t)
	insertProjectLedgerBinding44(t, store.conn, "/tmp/typed-memory-v45")
	digest := typedMemoryDigest45("a")
	typeEnvRef := "typeenv:" + digest
	_, err := store.conn.Exec(`INSERT INTO typed_memory_type_env_snapshots (
		type_env_ref, artifact_digest, snapshot_format, canonical_bytes,
		source_revision, compiler_schema_version, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		typeEnvRef,
		digest,
		"haft.typed-memory.type-env/v1",
		[]byte("canonical-type-env-v1"),
		"source-revision:v1",
		"compiler-schema:v1",
		typedMemoryRecordedAt45,
	)
	if err != nil {
		_ = store.Close()
		t.Fatalf("insert TypeEnv snapshot: %v", err)
	}
	return store, typeEnvRef
}

func newStoreThroughTypedMemoryStorageMigration45(t *testing.T) *Store {
	t.Helper()
	database := openDatabaseBeforeTypedMemoryStorageMigration45(t)
	err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryStorageMigration45},
	)
	if err != nil {
		_ = database.Close()
		t.Fatalf("open store frozen through typed-memory migration v45: %v", err)
	}
	assertMigrationVersionPresent(t, database, 45)
	assertMigrationVersionAbsent(t, database, 46)
	return &Store{conn: database, q: New()}
}

func insertTypedMemoryGenesisHead45(t *testing.T, database *sql.DB, typeEnvRef string) {
	t.Helper()
	_, err := database.Exec(`INSERT INTO typed_memory_graph_heads (
		project_id, graph_revision, active_type_env_ref,
		last_event_ref, last_commit_ref, updated_at
	) VALUES (?, 0, ?, '', '', ?)`,
		typedMemoryProjectID45,
		typeEnvRef,
		typedMemoryRecordedAt45,
	)
	if err != nil {
		t.Fatalf("insert typed-memory genesis head: %v", err)
	}
}

func newTypedMemoryDeclarationFixture45(
	suffix string,
	eventDigestSeed string,
	changeSetDigestSeed string,
	typeEnvRef string,
	expectedRevision int,
) typedMemoryDeclarationFixture45 {
	return typedMemoryDeclarationFixture45{
		projectID:        typedMemoryProjectID45,
		eventRef:         "event:" + suffix,
		commitRef:        "commit:" + suffix,
		entityID:         "entity:" + suffix,
		boundedContext:   "bounded-context:" + suffix,
		idempotencyKey:   "idempotency:" + suffix,
		projectionJobRef: "projection-job:" + suffix,
		eventDigest:      typedMemoryDigest45(eventDigestSeed),
		changeSetDigest:  typedMemoryDigest45(changeSetDigestSeed),
		typeEnvRef:       typeEnvRef,
		expectedRevision: expectedRevision,
		graphRevision:    expectedRevision + 1,
	}
}

func mustInsertTypedMemoryEvent45(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryDeclarationFixture45,
	changeCount int,
) {
	t.Helper()
	_, err := execer.Exec(`INSERT INTO typed_memory_graph_events (
		project_id, event_ref, commit_ref, event_digest,
		expected_revision, graph_revision,
		basis_type_env_ref, result_type_env_ref,
		change_set_digest, canonical_change_set_bytes, change_count,
		event_kind, authority_class, request_provenance_ref, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.projectID,
		fixture.eventRef,
		fixture.commitRef,
		fixture.eventDigest,
		fixture.expectedRevision,
		fixture.graphRevision,
		fixture.typeEnvRef,
		fixture.typeEnvRef,
		fixture.changeSetDigest,
		[]byte(`{"change":"declare_entity"}`),
		changeCount,
		"declare_entity",
		"non_binding_semantic_assertion",
		"request-provenance:"+fixture.eventRef,
		typedMemoryRecordedAt45,
	)
	if err != nil {
		t.Fatalf("insert typed-memory event %s: %v", fixture.eventRef, err)
	}
}

func mustInsertTypedMemoryDeclarationMaterialization45(
	t *testing.T,
	execer typedMemoryExecer45,
	fixture typedMemoryDeclarationFixture45,
) {
	t.Helper()
	_, err := execer.Exec(`INSERT INTO typed_memory_entities (
		project_id, entity_id, first_declared_event_ref,
		first_declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?)`,
		fixture.projectID,
		fixture.entityID,
		fixture.eventRef,
		fixture.graphRevision,
		typedMemoryRecordedAt45,
	)
	if err != nil {
		t.Fatalf("insert typed-memory entity %s: %v", fixture.entityID, err)
	}
	_, err = execer.Exec(`INSERT INTO typed_memory_entity_contexts (
		project_id, entity_id, bounded_context_ref, label, provenance_ref,
		declared_event_ref, declared_revision, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.projectID,
		fixture.entityID,
		fixture.boundedContext,
		"Entity "+fixture.entityID,
		"provenance:"+fixture.entityID,
		fixture.eventRef,
		fixture.graphRevision,
		typedMemoryRecordedAt45,
	)
	if err != nil {
		t.Fatalf("insert typed-memory entity context %s: %v", fixture.boundedContext, err)
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
		typedMemoryRecordedAt45,
	)
	if err != nil {
		t.Fatalf("insert typed-memory idempotency row %s: %v", fixture.idempotencyKey, err)
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
		typedMemoryRecordedAt45,
	)
	if err != nil {
		t.Fatalf("insert typed-memory projection job %s: %v", fixture.projectionJobRef, err)
	}
}

func insertTypedMemoryGraphCommit45(
	execer typedMemoryExecer45,
	fixture typedMemoryDeclarationFixture45,
) error {
	_, err := execer.Exec(`INSERT INTO typed_memory_graph_commits (
		project_id, commit_ref, event_ref, event_digest,
		expected_revision, graph_revision, change_set_digest,
		idempotency_key, projection_job_ref,
		entity_count, entity_context_count, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?)`,
		fixture.projectID,
		fixture.commitRef,
		fixture.eventRef,
		fixture.eventDigest,
		fixture.expectedRevision,
		fixture.graphRevision,
		fixture.changeSetDigest,
		fixture.idempotencyKey,
		fixture.projectionJobRef,
		typedMemoryRecordedAt45,
	)
	return err
}

func commitTypedMemoryDeclaration45(
	t *testing.T,
	database *sql.DB,
	fixture typedMemoryDeclarationFixture45,
) {
	t.Helper()
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin typed-memory declaration transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture, 1)
	mustInsertTypedMemoryDeclarationMaterialization45(t, transaction, fixture)
	if err := insertTypedMemoryGraphCommit45(transaction, fixture); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert typed-memory graph commit %s: %v", fixture.commitRef, err)
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit typed-memory declaration %s: %v", fixture.eventRef, err)
	}
	assertTypedMemoryHeadRevision45(t, database, fixture.graphRevision)
}

func openDatabaseBeforeTypedMemoryStorageMigration45(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "pre-v45.db")
	dsn, err := sqliteConnectionDSN(dbPath)
	if err != nil {
		t.Fatalf("build pre-v45 SQLite DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open pre-v45 SQLite database: %v", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping pre-v45 SQLite database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install base schema for pre-v45 database: %v", err)
	}
	migrations := migrationsBeforeVersion(kernelMigrations, 45, 0, nil)
	if err := Migrate(database, "schema_version", migrations); err != nil {
		_ = database.Close()
		t.Fatalf("migrate database through v44: %v", err)
	}
	assertMigrationVersionPresent(t, database, 44)
	assertMigrationVersionAbsent(t, database, 45)
	return database
}

func assertTypedMemoryHeadRevision45(t *testing.T, database *sql.DB, want int) {
	t.Helper()
	var revision int
	err := database.QueryRow(
		"SELECT graph_revision FROM typed_memory_graph_heads WHERE project_id = ?",
		typedMemoryProjectID45,
	).Scan(&revision)
	if err != nil {
		t.Fatalf("read typed-memory graph head revision: %v", err)
	}
	if revision != want {
		t.Fatalf("typed-memory graph head revision = %d, want %d", revision, want)
	}
}

func assertTypedMemoryTableRowCount45(
	t *testing.T,
	database *sql.DB,
	table string,
	want int,
) {
	t.Helper()
	var count int
	query := "SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)
	if err := database.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("count typed-memory table %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("typed-memory table %s row count = %d, want %d", table, count, want)
	}
}

func assertTypedMemoryForeignKeysClean45(t *testing.T, database *sql.DB) {
	t.Helper()
	rows, err := database.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("run typed-memory foreign-key check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		var table string
		var rowID any
		var parent string
		var foreignKeyID int
		if err := rows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
			t.Fatalf("read typed-memory foreign-key violation: %v", err)
		}
		t.Fatalf(
			"foreign-key violation: table=%s row=%v parent=%s fk=%d",
			table,
			rowID,
			parent,
			foreignKeyID,
		)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read typed-memory foreign-key check: %v", err)
	}
}

func typedMemoryDigest45(seed string) string {
	return "sha256:" + strings.Repeat(seed, 64)
}
