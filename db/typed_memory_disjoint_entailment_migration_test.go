package db

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"
)

func TestTypedMemoryDisjointEntailmentMigration49FreshKernelPath(t *testing.T) {
	t.Parallel()

	database := openDatabaseBeforeTypedMemoryStorageMigration46(t)
	defer database.Close()

	err := Migrate(
		database,
		"schema_version",
		[]Migration{
			typedMemoryStorageMigration46,
			projectTypeEnvHeadSelectionMigration47,
			projectTypeEnvHeadSelectionMigration48,
			typedMemoryDisjointEntailmentMigration49,
		},
	)
	if err != nil {
		t.Fatalf("migrate fresh kernel path through v49: %v", err)
	}

	for _, version := range []int{46, 47, 48, 49} {
		assertMigrationVersionPresent(t, database, version)
	}
	assertTypedMemoryDisjointEntailmentSchema49(t, database)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestTypedMemoryDisjointEntailmentMigration49PreservesV48History(
	t *testing.T,
) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(
		typeEnvRef,
		"preserved-through-v49",
		0,
	)
	commitTypedMemorySnapshotDeclaration46(t, database, fixture)
	migrateProjectTypeEnvHeadSelection47(t, database)
	migrateProjectTypeEnvHeadSelection48(t, database)

	eventBytesBefore, eventDigestBefore := readTypedMemoryEventIdentity49(
		t,
		database,
		fixture,
	)
	closureBytesBefore, closureDigestBefore := readTypedMemoryClosureIdentity49(
		t,
		database,
		fixture,
	)
	memberUseCountBefore, activationCountBefore := readTypedMemoryFootprintCounts49(
		t,
		database,
		fixture,
	)

	migrateTypedMemoryDisjointEntailment49(t, database)

	eventBytesAfter, eventDigestAfter := readTypedMemoryEventIdentity49(
		t,
		database,
		fixture,
	)
	closureBytesAfter, closureDigestAfter := readTypedMemoryClosureIdentity49(
		t,
		database,
		fixture,
	)
	memberUseCountAfter, activationCountAfter := readTypedMemoryFootprintCounts49(
		t,
		database,
		fixture,
	)

	if !bytes.Equal(eventBytesAfter, eventBytesBefore) ||
		eventDigestAfter != eventDigestBefore {
		t.Fatal("v49 changed preserved v46 event bytes or digest")
	}
	if !bytes.Equal(closureBytesAfter, closureBytesBefore) ||
		closureDigestAfter != closureDigestBefore {
		t.Fatal("v49 changed preserved v46 materialization closure bytes or digest")
	}
	if memberUseCountAfter != memberUseCountBefore ||
		activationCountAfter != activationCountBefore {
		t.Fatalf(
			"v49 changed preserved footprint counts: member uses %d -> %d, activations %d -> %d",
			memberUseCountBefore,
			memberUseCountAfter,
			activationCountBefore,
			activationCountAfter,
		)
	}

	assertTypedMemoryDisjointEntailmentSchema49(t, database)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestTypedMemoryDisjointEntailmentMigration49RollsBackOnHistoricalFootprintMismatch(
	t *testing.T,
) {
	t.Parallel()

	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(
		typeEnvRef,
		"rollback-v49",
		0,
	)
	commitTypedMemorySnapshotDeclaration46(t, database, fixture)
	migrateProjectTypeEnvHeadSelection47(t, database)
	migrateProjectTypeEnvHeadSelection48(t, database)

	_, err := database.Exec(
		"DROP TRIGGER typed_memory_commit_materialization_closures_v46_no_update",
	)
	if err != nil {
		t.Fatalf("detach closure immutability guard for mismatch fixture: %v", err)
	}
	_, err = database.Exec(
		`UPDATE typed_memory_commit_materialization_closures
		SET memberof_use_count = memberof_use_count + 1
		WHERE project_id = ? AND event_ref = ?`,
		fixture.event.projectID,
		fixture.event.eventRef,
	)
	if err != nil {
		t.Fatalf("install historical footprint mismatch fixture: %v", err)
	}
	_, err = database.Exec(
		immutableTypedMemoryTrigger46(
			"typed_memory_commit_materialization_closures",
			"update",
		),
	)
	if err != nil {
		t.Fatalf("restore closure immutability guard: %v", err)
	}

	err = Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryDisjointEntailmentMigration49},
	)
	if err == nil || !strings.Contains(
		err.Error(),
		"historical materialization footprint mismatches",
	) {
		t.Fatalf("v49 historical mismatch error = %v", err)
	}

	assertMigrationVersionAbsent(t, database, 49)
	assertSQLiteObjectAbsent49(
		t,
		database,
		"table",
		typedMemoryDisjointEntailmentUsesTable49,
	)
	assertSQLiteObjectMatches49(
		t,
		database,
		"view",
		"typed_memory_event_materialization_footprints_v46",
		mustTypedMemoryEventMaterializationFootprintsView47(t),
	)
	assertSQLiteObjectMatches49(
		t,
		database,
		"trigger",
		"typed_memory_commit_materialization_closures_v46_exact_footprint",
		mustTypedMemoryCommitClosureExactFootprintTrigger47(t),
	)
	assertSQLiteObjectMatches49(
		t,
		database,
		"trigger",
		"typed_memory_graph_commits_exact_closure",
		typedMemoryGraphCommitExactClosureTrigger46(),
	)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func migrateTypedMemoryDisjointEntailment49(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	if err := Migrate(
		database,
		"schema_version",
		[]Migration{typedMemoryDisjointEntailmentMigration49},
	); err != nil {
		t.Fatalf("migrate database through v49: %v", err)
	}
}

func assertTypedMemoryDisjointEntailmentSchema49(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	assertSQLiteObjectExists(
		t,
		database,
		"table",
		typedMemoryDisjointEntailmentUsesTable49,
	)
	for _, trigger := range []string{
		typedMemoryDisjointEntailmentUsesTable49 + "_v49_exact_use",
		typedMemoryDisjointEntailmentUsesTable49 + "_v49_open_event",
		typedMemoryDisjointEntailmentUsesTable49 + "_v49_no_update",
		typedMemoryDisjointEntailmentUsesTable49 + "_v49_no_delete",
	} {
		assertSQLiteObjectExists(t, database, "trigger", trigger)
	}

	wantColumns := []string{
		"project_id",
		"event_ref",
		"change_ordinal",
		"assertion_id",
		"slot_ordinal",
		"filler_ordinal",
		"filler_digest",
		"constraint_id",
		"constraint_digest",
		"canonical_constraint_bytes",
		"matched_operand_kind_id",
		"excluded_operand_kind_id",
		"counter_value_kind_ref",
		"counter_query_digest",
		"canonical_counter_query_bytes",
		"supporting_evaluation_ref",
		"use_digest",
		"canonical_use_bytes",
	}
	rows, err := database.Query(
		"PRAGMA table_info(" + typedMemoryDisjointEntailmentUsesTable49 + ")",
	)
	if err != nil {
		t.Fatalf("inspect v49 disjoint-entailment columns: %v", err)
	}
	defer rows.Close()
	gotColumns := make([]string, 0, len(wantColumns))
	for rows.Next() {
		var ordinal int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var primaryKeyOrdinal int
		if err := rows.Scan(
			&ordinal,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKeyOrdinal,
		); err != nil {
			t.Fatalf("scan v49 disjoint-entailment column: %v", err)
		}
		gotColumns = append(gotColumns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate v49 disjoint-entailment columns: %v", err)
	}
	if strings.Join(gotColumns, "\n") != strings.Join(wantColumns, "\n") {
		t.Fatalf("v49 disjoint-entailment columns = %v; want %v", gotColumns, wantColumns)
	}

	var writerGeneration int
	var capabilityDigest string
	var canonicalCapability []byte
	err = database.QueryRow(
		`SELECT writer_generation, capability_digest, canonical_bytes
		FROM typed_memory_storage_capabilities
		WHERE capability_key = ?`,
		typedMemoryWriterGenerationCapability46,
	).Scan(&writerGeneration, &capabilityDigest, &canonicalCapability)
	if err != nil {
		t.Fatalf("read preserved writer capability after v49: %v", err)
	}
	if writerGeneration != typedMemoryWriterGeneration46 ||
		capabilityDigest != typedMemoryWriterMarkerDigest46 ||
		!bytes.Equal(canonicalCapability, []byte(typedMemoryWriterMarkerBytes46)) {
		t.Fatal("v49 changed the sealed v46 writer capability")
	}
	assertTypedMemoryTableRowCount45(
		t,
		database,
		typedMemoryDisjointEntailmentUsesTable49,
		0,
	)
}

func readTypedMemoryEventIdentity49(
	t *testing.T,
	database *sql.DB,
	fixture typedMemoryAdmissionFixture46,
) ([]byte, string) {
	t.Helper()
	var canonical []byte
	var digest string
	err := database.QueryRow(
		`SELECT canonical_change_set_bytes, event_digest
		FROM typed_memory_graph_events
		WHERE project_id = ? AND event_ref = ?`,
		fixture.event.projectID,
		fixture.event.eventRef,
	).Scan(&canonical, &digest)
	if err != nil {
		t.Fatalf("read preserved typed-memory event: %v", err)
	}
	return canonical, digest
}

func readTypedMemoryClosureIdentity49(
	t *testing.T,
	database *sql.DB,
	fixture typedMemoryAdmissionFixture46,
) ([]byte, string) {
	t.Helper()
	var canonical []byte
	var digest string
	err := database.QueryRow(
		`SELECT canonical_materialization_bytes, materialization_digest
		FROM typed_memory_commit_materialization_closures
		WHERE project_id = ? AND event_ref = ?`,
		fixture.event.projectID,
		fixture.event.eventRef,
	).Scan(&canonical, &digest)
	if err != nil {
		t.Fatalf("read preserved typed-memory closure: %v", err)
	}
	return canonical, digest
}

func readTypedMemoryFootprintCounts49(
	t *testing.T,
	database *sql.DB,
	fixture typedMemoryAdmissionFixture46,
) (int, int) {
	t.Helper()
	var memberUseCount int
	var activationCount int
	err := database.QueryRow(
		`SELECT memberof_use_count, type_env_activation_count
		FROM typed_memory_event_materialization_footprints_v46
		WHERE project_id = ? AND event_ref = ?`,
		fixture.event.projectID,
		fixture.event.eventRef,
	).Scan(&memberUseCount, &activationCount)
	if err != nil {
		t.Fatalf("read typed-memory v49 footprint counts: %v", err)
	}
	return memberUseCount, activationCount
}

func assertSQLiteObjectAbsent49(
	t *testing.T,
	database *sql.DB,
	kind string,
	name string,
) {
	t.Helper()
	var count int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?",
		kind,
		name,
	).Scan(&count)
	if err != nil {
		t.Fatalf("inspect absent %s %s: %v", kind, name, err)
	}
	if count != 0 {
		t.Fatalf("%s %s survived rolled-back v49 migration", kind, name)
	}
}

func assertSQLiteObjectMatches49(
	t *testing.T,
	database *sql.DB,
	kind string,
	name string,
	want string,
) {
	t.Helper()
	var got string
	err := database.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = ? AND name = ?",
		kind,
		name,
	).Scan(&got)
	if err != nil {
		t.Fatalf("read %s %s after rolled-back v49: %v", kind, name, err)
	}
	if normalizeSQLiteDDL46(got) != normalizeSQLiteDDL46(want) {
		t.Fatalf("%s %s did not roll back to its exact v48 definition", kind, name)
	}
}

func mustTypedMemoryEventMaterializationFootprintsView47(t *testing.T) string {
	t.Helper()
	view, err := typedMemoryEventMaterializationFootprintsView47()
	if err != nil {
		t.Fatalf("derive exact v48 typed-memory footprint view: %v", err)
	}
	return view
}

func mustTypedMemoryCommitClosureExactFootprintTrigger47(t *testing.T) string {
	t.Helper()
	trigger, err := typedMemoryCommitClosureExactFootprintTrigger47()
	if err != nil {
		t.Fatalf("derive exact v48 typed-memory closure trigger: %v", err)
	}
	return trigger
}
