package db

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvheadstore"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstage"
	"github.com/m0n0x41d/haft/internal/projecttypeenvstore"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestProjectTypeEnvHeadSelectionMigration47InstallsExactEmptyFootprintAndPreservesV46History(
	t *testing.T,
) {
	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	fixture := newTypedMemoryAdmissionFixture46(
		typeEnvRef,
		"preserved-through-v47",
		0,
	)
	commitTypedMemorySnapshotDeclaration46(t, database, fixture)

	var eventBytesBefore []byte
	var eventDigestBefore string
	err := database.QueryRow(
		`SELECT canonical_change_set_bytes, event_digest
		FROM typed_memory_graph_events
		WHERE project_id = ? AND event_ref = ?`,
		fixture.event.projectID,
		fixture.event.eventRef,
	).Scan(&eventBytesBefore, &eventDigestBefore)
	if err != nil {
		t.Fatalf("read v46 event before v47: %v", err)
	}
	var closureBytesBefore []byte
	var closureDigestBefore string
	err = database.QueryRow(
		`SELECT canonical_materialization_bytes, materialization_digest
		FROM typed_memory_commit_materialization_closures
		WHERE project_id = ? AND event_ref = ?`,
		fixture.event.projectID,
		fixture.event.eventRef,
	).Scan(&closureBytesBefore, &closureDigestBefore)
	if err != nil {
		t.Fatalf("read v46 closure before v47: %v", err)
	}

	migrateProjectTypeEnvHeadSelection47(t, database)

	assertMigrationVersionPresent(t, database, 47)
	for _, table := range projectTypeEnvEffectTables47 {
		assertSQLiteObjectExists(t, database, "table", table)
		assertTypedMemoryTableRowCount45(t, database, table, 0)
	}
	for _, index := range projectTypeEnvEffectIndexes47 {
		assertSQLiteObjectExists(t, database, "index", index)
	}
	for _, trigger := range projectTypeEnvEffectSpecificTriggers47 {
		assertSQLiteObjectExists(t, database, "trigger", trigger)
	}
	for _, trigger := range projectTypeEnvCandidateAnnexTriggerNames47() {
		assertSQLiteObjectExists(t, database, "trigger", trigger)
	}
	for _, policy := range projectTypeEnvEffectPolicies47() {
		for _, operation := range []string{"insert", "update", "delete"} {
			assertSQLiteObjectExists(
				t,
				database,
				"trigger",
				policy.table+"_v47_no_"+operation,
			)
		}
	}

	var eventBytesAfter []byte
	var eventDigestAfter string
	err = database.QueryRow(
		`SELECT canonical_change_set_bytes, event_digest
		FROM typed_memory_graph_events
		WHERE project_id = ? AND event_ref = ?`,
		fixture.event.projectID,
		fixture.event.eventRef,
	).Scan(&eventBytesAfter, &eventDigestAfter)
	if err != nil {
		t.Fatalf("read v46 event after v47: %v", err)
	}
	if !bytes.Equal(eventBytesAfter, eventBytesBefore) ||
		eventDigestAfter != eventDigestBefore {
		t.Fatal("v47 changed preserved v46 event bytes or digest")
	}
	var closureBytesAfter []byte
	var closureDigestAfter string
	var activationCount int
	err = database.QueryRow(
		`SELECT canonical_materialization_bytes, materialization_digest,
			type_env_activation_count
		FROM typed_memory_commit_materialization_closures
		WHERE project_id = ? AND event_ref = ?`,
		fixture.event.projectID,
		fixture.event.eventRef,
	).Scan(&closureBytesAfter, &closureDigestAfter, &activationCount)
	if err != nil {
		t.Fatalf("read v46 closure after v47: %v", err)
	}
	if !bytes.Equal(closureBytesAfter, closureBytesBefore) ||
		closureDigestAfter != closureDigestBefore ||
		activationCount != 0 {
		t.Fatalf(
			"v47 changed legacy closure: digest=%q activation_count=%d",
			closureDigestAfter,
			activationCount,
		)
	}
	var footprintActivationCount int
	var topLevelCount int
	err = database.QueryRow(
		`SELECT type_env_activation_count, top_level_change_count
		FROM typed_memory_event_materialization_footprints_v46
		WHERE project_id = ? AND event_ref = ?`,
		fixture.event.projectID,
		fixture.event.eventRef,
	).Scan(&footprintActivationCount, &topLevelCount)
	if err != nil {
		t.Fatalf("read v47 materialization footprint: %v", err)
	}
	if footprintActivationCount != 0 || topLevelCount != 1 {
		t.Fatalf(
			"legacy footprint activation/top-level counts = %d/%d; want 0/1",
			footprintActivationCount,
			topLevelCount,
		)
	}

	ctx := context.Background()
	if _, err := projecttypeenvstore.New(ctx, database); err != nil {
		t.Fatalf("open artifact store after kernel migration: %v", err)
	}
	if _, err := projecttypeenvstage.New(ctx, database); err != nil {
		t.Fatalf("open Stage store after kernel migration: %v", err)
	}
	if _, err := projecttypeenvheadstore.New(ctx, database); err != nil {
		t.Fatalf("open head store after kernel migration: %v", err)
	}
	assertProjectTypeEnvVerifierEditionSchema47(t, database)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestProjectTypeEnvHeadSelectionMigration47BackfillsAndRegistersCoordinateOwners(
	t *testing.T,
) {
	database, baseTypeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	ctx := context.Background()
	if _, err := projecttypeenvstage.New(ctx, database); err != nil {
		t.Fatalf("install live Stage store before v47: %v", err)
	}
	preexistingExecutableRef := "typeenv:" + typedMemoryDigest45("8")
	insertProjectTypeEnvExecutableSnapshot47(
		t,
		database,
		preexistingExecutableRef,
		"preexisting-v47",
	)

	migrateProjectTypeEnvHeadSelection47(t, database)

	assertTypeEnvCoordinateOwner47(
		t,
		database,
		baseTypeEnvRef,
		"generic_snapshot",
		baseTypeEnvRef,
		"",
	)
	assertTypeEnvCoordinateOwner47(
		t,
		database,
		preexistingExecutableRef,
		"project_executable",
		"",
		preexistingExecutableRef,
	)

	futureGenericRef := insertGenericTypeEnvSnapshot47(t, database, "d")
	assertTypeEnvCoordinateOwner47(
		t,
		database,
		futureGenericRef,
		"generic_snapshot",
		futureGenericRef,
		"",
	)
	futureExecutableRef := "typeenv:" + typedMemoryDigest45("b")
	insertProjectTypeEnvExecutableSnapshot47(
		t,
		database,
		futureExecutableRef,
		"future-v47",
	)
	assertTypeEnvCoordinateOwner47(
		t,
		database,
		futureExecutableRef,
		"project_executable",
		"",
		futureExecutableRef,
	)

	_, collisionErr := database.Exec(
		`INSERT INTO typed_memory_type_env_snapshots (
			type_env_ref,
			artifact_digest,
			snapshot_format,
			canonical_bytes,
			source_revision,
			compiler_schema_version,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		futureExecutableRef,
		typedMemoryDigest45("b"),
		"haft.typed-memory.type-env/v1",
		[]byte("colliding-generic-type-env"),
		"source-revision:collision",
		"compiler-schema:v1",
		typedMemoryRecordedAt46,
	)
	if collisionErr == nil ||
		!strings.Contains(collisionErr.Error(), "ownership") {
		t.Fatalf("cross-owner coordinate collision error = %v", collisionErr)
	}
	assertTypedMemoryTableRowCountWhere47(
		t,
		database,
		"typed_memory_type_env_snapshots",
		"type_env_ref",
		futureExecutableRef,
		0,
	)
	_, updateErr := database.Exec(
		`UPDATE typed_memory_type_env_coordinates
		SET representation_kind = 'project_executable'
		WHERE type_env_ref = ?`,
		baseTypeEnvRef,
	)
	if updateErr == nil ||
		(!strings.Contains(updateErr.Error(), "immutable") &&
			!strings.Contains(updateErr.Error(), "append-only")) {
		t.Fatalf("coordinate mutation error = %v", updateErr)
	}
	_, deleteErr := database.Exec(
		`DELETE FROM typed_memory_type_env_coordinates
		WHERE type_env_ref = ?`,
		baseTypeEnvRef,
	)
	if deleteErr == nil ||
		(!strings.Contains(deleteErr.Error(), "immutable") &&
			!strings.Contains(deleteErr.Error(), "append-only")) {
		t.Fatalf("coordinate deletion error = %v", deleteErr)
	}
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestProjectTypeEnvHeadSelectionMigration47RejectsPreexistingCoordinateOwnerCollisionAtomically(
	t *testing.T,
) {
	database, baseTypeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	ctx := context.Background()
	if _, err := projecttypeenvstage.New(ctx, database); err != nil {
		t.Fatalf("install live Stage store before collision: %v", err)
	}
	insertProjectTypeEnvExecutableSnapshot47(
		t,
		database,
		baseTypeEnvRef,
		"owner-collision-v47",
	)
	before := sqliteMasterSnapshot47(t, database)

	err := Migrate(
		database,
		"schema_version",
		[]Migration{projectTypeEnvHeadSelectionMigration47},
	)
	if err == nil {
		t.Fatal("preexisting TypeEnv owner collision migration unexpectedly succeeded")
	}
	assertMigrationVersionAbsent(t, database, 47)
	after := sqliteMasterSnapshot47(t, database)
	if !equalSQLiteMasterSnapshots47(before, after) {
		t.Fatal("owner-collision rollback changed sqlite_master")
	}
	requireSQLiteObjectCount(
		t,
		database,
		"table",
		"typed_memory_type_env_coordinates",
		0,
	)
	assertTypedMemoryTableRowCountWhere47(
		t,
		database,
		"typed_memory_type_env_snapshots",
		"type_env_ref",
		baseTypeEnvRef,
		1,
	)
	assertTypedMemoryTableRowCountWhere47(
		t,
		database,
		"project_typeenv_executable_snapshots",
		"type_env_ref",
		baseTypeEnvRef,
		1,
	)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestProjectTypeEnvHeadSelectionMigration47RejectsGenericOnlySelectedComposite(
	t *testing.T,
) {
	database, baseTypeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	migrateProjectTypeEnvHeadSelection47(t, database)
	ctx := context.Background()
	store, err := projecttypeenvheadstore.New(ctx, database)
	if err != nil {
		t.Fatalf("open v47 head store: %v", err)
	}
	state := newProjectTypeEnvHeadState47(
		t,
		typedMemoryProjectID45,
		baseTypeEnvRef,
		1,
	)
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("begin generic-only head CAS: %v", err)
	}
	err = store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		state,
	)
	if err == nil || !strings.Contains(err.Error(), "executable composite owner") {
		_ = transaction.Rollback(ctx)
		t.Fatalf("generic-only selected composite error = %v", err)
	}
	if finish := transaction.Rollback(ctx); !finish.Succeeded() {
		t.Fatalf("roll back generic-only head CAS: %v", finish.Err())
	}
}

func TestProjectTypeEnvHeadSelectionMigration47AnnexesExactLiveCandidateStores(
	t *testing.T,
) {
	database, _ := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	ctx := context.Background()
	if _, err := projecttypeenvstore.New(ctx, database); err != nil {
		t.Fatalf("install live artifact store: %v", err)
	}
	if _, err := projecttypeenvstage.New(ctx, database); err != nil {
		t.Fatalf("install live Stage store: %v", err)
	}
	if _, err := projecttypeenvheadstore.New(ctx, database); err != nil {
		t.Fatalf("install live head store: %v", err)
	}
	candidateBytes := []byte("preserved-candidate-bytes")
	_, err := database.Exec(
		`INSERT INTO project_typeenv_artifacts (
			artifact_kind,
			artifact_ref,
			artifact_digest,
			canonical_schema_version,
			producer_schema_version,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?)`,
		"base_type_env",
		"candidate:base:v47",
		"candidate-digest:v47",
		"candidate-schema:v1",
		"producer-schema:v1",
		candidateBytes,
	)
	if err != nil {
		t.Fatalf("seed exact live candidate store: %v", err)
	}

	migrateProjectTypeEnvHeadSelection47(t, database)

	var preserved []byte
	err = database.QueryRow(
		`SELECT canonical_bytes
		FROM project_typeenv_artifacts
		WHERE artifact_kind = ? AND artifact_ref = ?`,
		"base_type_env",
		"candidate:base:v47",
	).Scan(&preserved)
	if err != nil {
		t.Fatalf("read annexed candidate: %v", err)
	}
	if !bytes.Equal(preserved, candidateBytes) {
		t.Fatalf("annex changed candidate bytes: %q", preserved)
	}
	_, updateErr := database.Exec(
		`UPDATE project_typeenv_artifacts
		SET canonical_bytes = ?
		WHERE artifact_kind = ? AND artifact_ref = ?`,
		[]byte("forged"),
		"base_type_env",
		"candidate:base:v47",
	)
	assertProjectTypeEnvImmutableError47(t, updateErr)
	_, deleteErr := database.Exec(
		`DELETE FROM project_typeenv_artifacts
		WHERE artifact_kind = ? AND artifact_ref = ?`,
		"base_type_env",
		"candidate:base:v47",
	)
	assertProjectTypeEnvImmutableError47(t, deleteErr)
	_, replaceErr := database.Exec(
		`INSERT OR REPLACE INTO project_typeenv_artifacts (
			artifact_kind,
			artifact_ref,
			artifact_digest,
			canonical_schema_version,
			producer_schema_version,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?)`,
		"base_type_env",
		"candidate:base:v47",
		"candidate-digest:forged",
		"candidate-schema:v1",
		"producer-schema:v1",
		[]byte("forged"),
	)
	assertProjectTypeEnvImmutableError47(t, replaceErr)

	if _, err := projecttypeenvstore.New(ctx, database); err != nil {
		t.Fatalf("reopen annexed artifact store: %v", err)
	}
	if _, err := projecttypeenvstage.New(ctx, database); err != nil {
		t.Fatalf("reopen annexed Stage store: %v", err)
	}
	if _, err := projecttypeenvheadstore.New(ctx, database); err != nil {
		t.Fatalf("reopen annexed head store: %v", err)
	}
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestProjectTypeEnvHeadSelectionMigration47RejectsNonEmptyPreReleaseHeadStore(
	t *testing.T,
) {
	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	ctx := context.Background()
	store, err := projecttypeenvheadstore.New(ctx, database)
	if err != nil {
		t.Fatalf("install pre-release head store: %v", err)
	}
	state := newProjectTypeEnvHeadState47(
		t,
		typedMemoryProjectID45,
		typeEnvRef,
		1,
	)
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("begin pre-release head CAS: %v", err)
	}
	if err := store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		state,
	); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("write pre-release head: %v", err)
	}
	if finish := transaction.Commit(ctx); !finish.Succeeded() {
		t.Fatalf("commit pre-release head: %v", finish.Err())
	}
	before := sqliteMasterSnapshot47(t, database)

	err = Migrate(
		database,
		"schema_version",
		[]Migration{projectTypeEnvHeadSelectionMigration47},
	)
	if err == nil || !strings.Contains(err.Error(), "non-empty pre-release") {
		t.Fatalf("non-empty pre-release head migration error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 47)
	after := sqliteMasterSnapshot47(t, database)
	if !equalSQLiteMasterSnapshots47(before, after) {
		t.Fatal("failed non-empty-head migration changed sqlite_master")
	}
	assertTypedMemoryTableRowCount45(t, database, "project_typeenv_heads", 1)
	assertTypedMemoryTableRowCount45(t, database, "project_typeenv_head_states", 1)
}

func TestProjectTypeEnvHeadSelectionMigration47RejectsPartialCandidateFootprintAtomically(
	t *testing.T,
) {
	database, _ := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	_, err := database.Exec(
		`CREATE TABLE project_typeenv_artifacts (
			artifact_kind TEXT NOT NULL,
			artifact_ref TEXT NOT NULL
		)`,
	)
	if err != nil {
		t.Fatalf("create partial candidate footprint: %v", err)
	}
	before := sqliteMasterSnapshot47(t, database)

	err = Migrate(
		database,
		"schema_version",
		[]Migration{projectTypeEnvHeadSelectionMigration47},
	)
	if err == nil || !strings.Contains(err.Error(), "partial footprint") {
		t.Fatalf("partial candidate footprint error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 47)
	after := sqliteMasterSnapshot47(t, database)
	if !equalSQLiteMasterSnapshots47(before, after) {
		t.Fatal("failed v47 candidate-footprint migration changed sqlite_master")
	}
}

func TestProjectTypeEnvHeadSelectionMigration47RejectsDriftedV46SourceAtomically(
	t *testing.T,
) {
	database, _ := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	_, err := database.Exec(
		"ALTER TABLE typed_memory_graph_events ADD COLUMN forged_v47_source TEXT",
	)
	if err != nil {
		t.Fatalf("drift v46 source table: %v", err)
	}
	before := sqliteMasterSnapshot47(t, database)

	err = Migrate(
		database,
		"schema_version",
		[]Migration{projectTypeEnvHeadSelectionMigration47},
	)
	if err == nil || !strings.Contains(err.Error(), "drifted") {
		t.Fatalf("drifted v46 source error = %v", err)
	}
	assertMigrationVersionAbsent(t, database, 47)
	after := sqliteMasterSnapshot47(t, database)
	if !equalSQLiteMasterSnapshots47(before, after) {
		t.Fatal("failed v47 source-drift migration changed sqlite_master")
	}
}

func TestProjectTypeEnvHeadSelectionMigration47SealsEffectRows(
	t *testing.T,
) {
	database, _ := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	migrateProjectTypeEnvHeadSelection47(t, database)
	basisDigest := typedMemoryDigest45("7")
	_, err := database.Exec(
		`INSERT INTO project_typeenv_head_selection_config_authority_bases (
			config_authority_basis_ref,
			config_authority_basis_digest,
			project_id,
			authority_mode,
			config_carrier_ref,
			config_carrier_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"config-authority-basis:v47",
		basisDigest,
		typedMemoryProjectID45,
		"explicit_h_decide",
		".haft/config.yaml",
		typedMemoryDigest45("8"),
		[]byte("canonical-config-authority-basis"),
		typedMemoryRecordedAt46,
	)
	if err != nil {
		t.Fatalf("insert v47 config authority basis: %v", err)
	}
	_, updateErr := database.Exec(
		`UPDATE project_typeenv_head_selection_config_authority_bases
		SET canonical_bytes = ? WHERE config_authority_basis_ref = ?`,
		[]byte("forged"),
		"config-authority-basis:v47",
	)
	assertProjectTypeEnvImmutableError47(t, updateErr)
	_, deleteErr := database.Exec(
		`DELETE FROM project_typeenv_head_selection_config_authority_bases
		WHERE config_authority_basis_ref = ?`,
		"config-authority-basis:v47",
	)
	assertProjectTypeEnvImmutableError47(t, deleteErr)
	_, replaceErr := database.Exec(
		`INSERT OR REPLACE INTO project_typeenv_head_selection_config_authority_bases (
			config_authority_basis_ref,
			config_authority_basis_digest,
			project_id,
			authority_mode,
			config_carrier_ref,
			config_carrier_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"config-authority-basis:v47",
		basisDigest,
		typedMemoryProjectID45,
		"explicit_h_decide",
		".haft/config.yaml",
		typedMemoryDigest45("8"),
		[]byte("forged"),
		typedMemoryRecordedAt46,
	)
	assertProjectTypeEnvImmutableError47(t, replaceErr)
}

func TestProjectTypeEnvHeadSelectionMigration47RejectsActivationRowForNonActivationEvent(
	t *testing.T,
) {
	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	migrateProjectTypeEnvHeadSelection47(t, database)
	resultTypeEnvRef := insertSecondProjectTypeEnvExecutable47(t, database)
	fixture := newTypedMemoryDeclarationFixture45(
		"non-activation-row-v47",
		"4",
		"5",
		typeEnvRef,
		0,
	)
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin non-activation row transaction: %v", err)
	}
	mustInsertTypedMemoryEvent45(t, transaction, fixture, 1)
	activationErr := insertMinimalTypeEnvActivation47(
		transaction,
		fixture,
		resultTypeEnvRef,
	)
	if activationErr == nil ||
		!strings.Contains(activationErr.Error(), "exact open authority effect") {
		_ = transaction.Rollback()
		t.Fatalf("non-activation semantic row error = %v", activationErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back non-activation row transaction: %v", err)
	}
}

func TestProjectTypeEnvHeadSelectionMigration47RejectsActivationCommitWithoutP8GClosure(
	t *testing.T,
) {
	database, typeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, typeEnvRef)
	migrateProjectTypeEnvHeadSelection47(t, database)
	resultTypeEnvRef := insertSecondProjectTypeEnvExecutable47(t, database)
	fixture := newTypedMemoryDeclarationFixture45(
		"missing-p8g-closure-v47",
		"6",
		"7",
		typeEnvRef,
		0,
	)
	transaction, err := database.Begin()
	if err != nil {
		t.Fatalf("begin missing-P8G activation transaction: %v", err)
	}
	_, err = transaction.Exec(
		`INSERT INTO typed_memory_graph_events (
			project_id,
			event_ref,
			commit_ref,
			event_digest,
			expected_revision,
			graph_revision,
			basis_type_env_ref,
			result_type_env_ref,
			change_set_digest,
			canonical_change_set_bytes,
			change_count,
			event_kind,
			authority_class,
			request_provenance_ref,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1,
			'activate_type_env', 'manual_type_env_activation', ?, ?)`,
		fixture.projectID,
		fixture.eventRef,
		fixture.commitRef,
		fixture.eventDigest,
		fixture.expectedRevision,
		fixture.graphRevision,
		typeEnvRef,
		resultTypeEnvRef,
		fixture.changeSetDigest,
		[]byte("activation-delta"),
		"request:missing-p8g",
		typedMemoryRecordedAt46,
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("insert open activation event: %v", err)
	}
	commitErr := insertTypedMemoryGraphCommit45(transaction, fixture)
	if commitErr == nil ||
		!strings.Contains(commitErr.Error(), "exact TypeEnv activation effect") {
		_ = transaction.Rollback()
		t.Fatalf("activation commit without P8G closure error = %v", commitErr)
	}
	if err := transaction.Rollback(); err != nil {
		t.Fatalf("roll back missing-P8G activation transaction: %v", err)
	}
}

func TestProjectTypeEnvHeadSelectionMigration47RejectsHeadOnlyCASAtCommit(
	t *testing.T,
) {
	database, _ := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	migrateProjectTypeEnvHeadSelection47(t, database)
	selectedTypeEnvRef := insertSecondProjectTypeEnvExecutable47(t, database)
	ctx := context.Background()
	store, err := projecttypeenvheadstore.New(ctx, database)
	if err != nil {
		t.Fatalf("open v47 head store: %v", err)
	}
	successor := newProjectTypeEnvHeadState47(
		t,
		typedMemoryProjectID45,
		selectedTypeEnvRef,
		1,
	)
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("begin head-only CAS: %v", err)
	}
	if err := store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		successor,
	); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("head-only Genesis CAS: %v", err)
	}
	finish := transaction.Commit(ctx)
	if finish.StatementError() == nil ||
		!strings.Contains(finish.StatementError().Error(), "FOREIGN KEY constraint failed") {
		t.Fatalf("head-only CAS commit error = %v", finish.Err())
	}
	if finish.CleanupError() != nil || finish.CloseError() != nil {
		t.Fatalf("head-only CAS cleanup error = %v", finish.Err())
	}
	for _, table := range []string{
		"project_typeenv_heads",
		"project_typeenv_head_states",
		"project_typeenv_head_effect_obligations",
	} {
		assertTypedMemoryTableRowCount45(t, database, table, 0)
	}
}

func TestProjectTypeEnvHeadSelectionMigration47AcceptsCompleteSameTransactionHeadEffect(
	t *testing.T,
) {
	database, basisTypeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, basisTypeEnvRef)
	migrateProjectTypeEnvHeadSelection47(t, database)
	resultTypeEnvRef := insertSecondProjectTypeEnvExecutable47(t, database)
	fixture := newProjectTypeEnvHeadEffectFixture47(
		basisTypeEnvRef,
		resultTypeEnvRef,
	)
	insertProjectTypeEnvCandidateStage47(t, database, fixture)
	commitCompleteGenesisHeadEffect47(t, database, fixture)
	for _, table := range []string{
		"project_typeenv_heads",
		"project_typeenv_head_states",
		"project_typeenv_head_effect_obligations",
		"project_typeenv_head_history",
		"project_typeenv_head_selection_closures",
		"typed_memory_type_env_activations",
	} {
		assertTypedMemoryTableRowCount45(t, database, table, 1)
	}
	assertTypedMemoryHeadRevision45(t, database, 1)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func commitCompleteGenesisHeadEffect47(
	t *testing.T,
	database *sql.DB,
	fixture projectTypeEnvHeadEffectFixture47,
) {
	t.Helper()
	ctx := context.Background()
	store, err := projecttypeenvheadstore.New(ctx, database)
	if err != nil {
		t.Fatalf("open v47 head store: %v", err)
	}
	successor := newProjectTypeEnvHeadState47(
		t,
		fixture.projectID,
		fixture.resultTypeEnvRef,
		1,
	)
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("begin complete P8G effect: %v", err)
	}
	insertProjectTypeEnvHeadEffectSources47(t, ctx, transaction, fixture)
	if err := store.CompareAndSwapGenesisProjectTypeEnvHeadTx(
		ctx,
		transaction,
		successor,
	); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("complete-effect Genesis CAS: %v", err)
	}
	var stateDigest string
	var canonicalState []byte
	err = transaction.ScanOne(
		ctx,
		`SELECT state_digest, canonical_bytes
		FROM project_typeenv_heads
		WHERE project_id = ?`,
		[]any{fixture.projectID},
		[]any{&stateDigest, &canonicalState},
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("read same-transaction head state: %v", err)
	}
	insertProjectTypeEnvHeadEffectClosure47(
		t,
		ctx,
		transaction,
		fixture,
		stateDigest,
		canonicalState,
	)
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit complete P8G effect: %v", finish.Err())
	}
}

func TestProjectTypeEnvHeadSelectionMigration47AcceptsTransitionRequestWithGenericBaseAndPriorExecutable(
	t *testing.T,
) {
	database, baseTypeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	insertTypedMemoryGenesisHead45(t, database, baseTypeEnvRef)
	migrateProjectTypeEnvHeadSelection47(t, database)
	firstCompositeRef := insertSecondProjectTypeEnvExecutable47(t, database)
	genesisFixture := newProjectTypeEnvHeadEffectFixture47(
		baseTypeEnvRef,
		firstCompositeRef,
	)
	insertProjectTypeEnvCandidateStage47(t, database, genesisFixture)
	commitCompleteGenesisHeadEffect47(t, database, genesisFixture)

	secondCompositeRef := "typeenv:" + typedMemoryDigest45("e")
	insertProjectTypeEnvExecutableSnapshot47(
		t,
		database,
		secondCompositeRef,
		"transition-v47",
	)
	stageRef := "project-typeenv-stage:transition-v47"
	stageDigest := typedMemoryDigest45("f")
	_, err := database.Exec(
		`INSERT INTO project_typeenv_stages (
			stage_ref,
			stage_digest,
			project_id,
			composite_verification_ref,
			canonical_schema_version,
			canonical_bytes,
			executable_type_env_ref
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		stageRef,
		stageDigest,
		genesisFixture.projectID,
		"project-typeenv-verification:transition-v47",
		"stage-schema:p8g-v47",
		[]byte("canonical-transition-stage:v47"),
		secondCompositeRef,
	)
	if err != nil {
		t.Fatalf("insert Transition Stage: %v", err)
	}
	insertTransitionRequest := func(
		requestRef string,
		requestDigest string,
		baseRef string,
		idempotencyKey string,
	) error {
		_, insertErr := database.Exec(
			`INSERT INTO project_typeenv_head_selection_requests (
				request_ref,
				request_digest,
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
			) VALUES (
				?, ?, ?, ?, 'transition',
				NULL, NULL, ?, 1, ?,
				?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?
			)`,
			requestRef,
			requestDigest,
			genesisFixture.projectID,
			genesisFixture.headRef,
			genesisFixture.headRef,
			firstCompositeRef,
			baseRef,
			typedMemoryDigest45("1"),
			[]byte("[]"),
			"runtime-evaluation-basis:transition-v47",
			secondCompositeRef,
			stageRef,
			stageDigest,
			idempotencyKey,
			[]byte("canonical-transition-request:v47"),
			typedMemoryRecordedAt46,
		)
		return insertErr
	}
	err = insertTransitionRequest(
		"project-typeenv-request:transition-v47",
		typedMemoryDigest45("2"),
		baseTypeEnvRef,
		"transition-v47-idempotency-key",
	)
	if err != nil {
		t.Fatalf("insert valid Transition request: %v", err)
	}
	err = insertTransitionRequest(
		"project-typeenv-request:wrong-base-v47",
		typedMemoryDigest45("3"),
		firstCompositeRef,
		"wrong-transition-base-v47-idempotency-key",
	)
	if err == nil {
		t.Fatal("Transition request accepted executable C as generic base B")
	}
	assertTypedMemoryTableRowCountWhere47(
		t,
		database,
		"project_typeenv_head_selection_requests",
		"request_ref",
		"project-typeenv-request:transition-v47",
		1,
	)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestProjectTypeEnvHeadSelectionMigration47AcceptsStrictPermissionResolutionUnion(
	t *testing.T,
) {
	database, basisTypeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	migrateProjectTypeEnvHeadSelection47(t, database)
	resultTypeEnvRef := insertSecondProjectTypeEnvExecutable47(t, database)
	fixture := newProjectTypeEnvHeadEffectFixture47(
		basisTypeEnvRef,
		resultTypeEnvRef,
	)
	insertProjectTypeEnvCandidateStage47(t, database, fixture)
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
	if err != nil {
		t.Fatalf("begin strict authority union fixture: %v", err)
	}
	configRef := "project-typeenv-config-basis:strict-v47"
	configDigest := typedMemoryDigest45("1")
	modeRef := "project-typeenv-mode-policy:strict-v47"
	modeDigest := typedMemoryDigest45("2")
	resolverRef := "resolver-policy:strict-v47"
	resolverDigest := typedMemoryDigest45("3")
	speechRef := "project-typeenv-speech-record:strict-v47"
	speechDigest := typedMemoryDigest45("4")
	permissionRef := "project-typeenv-permission:strict-v47"
	permissionDigest := typedMemoryDigest45("5")
	subjectDigest := typedMemoryDigest45("a")
	subjectPolicyDigest := typedMemoryDigest45("b")
	systemAdmissionDigest := typedMemoryDigest45("c")
	roleAdmissionDigest := typedMemoryDigest45("d")
	assignmentJustificationDigest := typedMemoryDigest45("e")
	assignmentProvenanceDigest := typedMemoryDigest45("f")
	basisRef := "project-typeenv-authority-resolution-basis:strict-v47"
	basisDigest := typedMemoryDigest45("6")
	resolutionRef := "project-typeenv-authority-resolution:strict-v47"
	resolutionDigest := typedMemoryDigest45("7")
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_config_authority_bases (
			config_authority_basis_ref,
			config_authority_basis_digest,
			project_id,
			authority_mode,
			config_carrier_ref,
			config_carrier_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, 'strict_cli_speech_act', ?, ?, ?, ?)`,
		configRef,
		configDigest,
		fixture.projectID,
		".haft/config.yaml",
		typedMemoryDigest45("8"),
		[]byte("canonical-strict-config-basis:v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_mode_policies (
			mode_policy_ref,
			mode_policy_digest,
			project_id,
			authority_mode,
			config_authority_basis_ref,
			config_authority_basis_digest,
			resolver_policy_ref,
			resolver_policy_edition,
			resolver_policy_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, 'strict_cli_speech_act', ?, ?, ?, 'v1', ?, ?, ?)`,
		modeRef,
		modeDigest,
		fixture.projectID,
		configRef,
		configDigest,
		resolverRef,
		resolverDigest,
		[]byte("canonical-strict-mode-policy:v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_no_prior_head_proofs (
			proof_ref,
			proof_digest,
			project_id,
			head_ref,
			graph_snapshot_ref,
			graph_snapshot_digest,
			expected_graph_revision,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		fixture.proofRef,
		fixture.proofDigest,
		fixture.projectID,
		fixture.headRef,
		fixture.graphSnapshotRef,
		fixture.graphSnapshotDigest,
		[]byte("canonical-strict-no-prior-head-proof:v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_requests (
			request_ref,
			request_digest,
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
		) VALUES (
			?, ?, ?, ?, 'genesis', ?, ?,
			NULL, NULL, NULL,
			?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?
		)`,
		fixture.requestRef,
		fixture.requestDigest,
		fixture.projectID,
		fixture.headRef,
		fixture.proofRef,
		fixture.proofDigest,
		fixture.basisTypeEnvRef,
		fixture.orderedExtensionsDigest,
		fixture.canonicalOrderedExtensions,
		fixture.runtimeEvaluationBasisRef,
		fixture.resultTypeEnvRef,
		fixture.stageRef,
		fixture.stageDigest,
		"strict-v47-idempotency-key",
		[]byte("canonical-strict-selection-request:v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_authorization_contents (
			content_ref,
			content_ref_kind,
			content_digest,
			project_id,
			request_ref,
			request_digest,
			judgement_context_ref,
			action_kind,
			valid_from,
			valid_until,
			canonical_bytes,
			recorded_at
		) VALUES (?, 'claim_id', ?, ?, ?, ?, ?, 'genesis', ?, ?, ?, ?)`,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.projectID,
		fixture.requestRef,
		fixture.requestDigest,
		"judgement-context:p8g-v47",
		"2026-07-16T13:00:00Z",
		"2026-07-16T15:00:00Z",
		[]byte("canonical-strict-authorization-content:v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_speech_act_records (
			speech_act_record_ref,
			speech_act_record_digest,
			project_id,
			speech_act_ref,
			human_work_ref,
			source_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			permission_ref,
			permission_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		speechRef,
		speechDigest,
		fixture.projectID,
		"speech-act:strict-v47",
		"work:human-authority-speech-act:strict-v47",
		typedMemoryDigest45("9"),
		fixture.contentRef,
		fixture.contentDigest,
		fixture.requestRef,
		fixture.requestDigest,
		permissionRef,
		permissionDigest,
		[]byte("canonical-verified-speech-act-record:v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_permissions_v3 (
			permission_ref,
			permission_digest,
			project_id,
			subject_role_assignment_ref,
			subject_role_assignment_digest,
			subject_schema,
			subject_holder_system_ref,
			subject_holder_kind,
			subject_role_ref,
			subject_context_ref,
			subject_assignment_from,
			subject_assignment_until,
			subject_assignment_policy_ref,
			subject_assignment_policy_digest,
			subject_assignment_policy_edition_ref,
			subject_assignment_policy_selection,
			subject_system_admission_ref,
			subject_system_admission_digest,
			subject_role_admission_ref,
			subject_role_admission_digest,
			subject_assignment_justification_ref,
			subject_assignment_justification_digest,
			subject_assignment_provenance_ref,
			subject_assignment_provenance_digest,
			subject_authorization_description_kind,
			subject_authorization_description_ref,
			subject_authorization_content_digest,
			subject_canonical_bytes,
			modality,
			claim_scope_ref,
			claim_scope_digest,
			context_policy_ref,
			context_policy_digest,
			referents_canonical_bytes,
			effective_from,
			validity_until,
			speech_act_record_ref,
			speech_act_record_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			canonical_bytes
		) VALUES (
			?, ?, ?, ?, ?,
			'haft.project-typeenv.head-selection-permission-subject-role-assignment/v1',
			'system:haft-software-system',
			'U.System',
			'role:project-governance-substrate',
			?, ?, ?, ?,
			?,
			'policy-edition:project-typeenv-head-selection-execution-role/v1',
			'current_for_new_write_at_seal',
			?, ?, ?, ?, ?, ?, ?, ?,
			'claim_id', ?, ?, ?,
			'MAY', ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?, ?
		)`,
		permissionRef,
		permissionDigest,
		fixture.projectID,
		"role-assignment:haft-software-system:project-governance-substrate:"+subjectDigest,
		subjectDigest,
		"judgement-context:p8g-v47",
		"2026-07-16T13:00:00Z",
		"2026-07-16T15:00:00Z",
		"project-typeenv-head-selection-execution-role-policy:"+subjectPolicyDigest,
		subjectPolicyDigest,
		"system-admission:project-typeenv-head-selection:"+systemAdmissionDigest,
		systemAdmissionDigest,
		"role-admission:project-typeenv-head-selection:"+roleAdmissionDigest,
		roleAdmissionDigest,
		"role-assignment-justification:project-typeenv-head-selection:"+
			assignmentJustificationDigest,
		assignmentJustificationDigest,
		"role-assignment-provenance:project-typeenv-head-selection:"+
			assignmentProvenanceDigest,
		assignmentProvenanceDigest,
		fixture.contentRef,
		fixture.contentDigest,
		[]byte("canonical-haft-system-role-assignment:v47"),
		"claim-scope:project-typeenv-head-selection:strict-v47",
		typedMemoryDigest45("b"),
		resolverRef,
		resolverDigest,
		[]byte(`[{"kind":"authorization_content"},{"kind":"project_typeenv_head_selection_request"}]`),
		"2026-07-16T14:00:00Z",
		"2026-07-16T14:30:00Z",
		speechRef,
		speechDigest,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.requestRef,
		fixture.requestDigest,
		[]byte("canonical-permission-v3:v47"),
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_authority_resolution_bases (
			basis_ref,
			basis_digest,
			project_id,
			resolver_policy_ref,
			resolver_policy_edition,
			resolver_policy_digest,
			speech_act_record_ref,
			speech_act_record_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			stage_ref,
			stage_digest,
			evaluated_at,
			canonical_bytes
		) VALUES (?, ?, ?, ?, 'v1', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		basisRef,
		basisDigest,
		fixture.projectID,
		resolverRef,
		resolverDigest,
		speechRef,
		speechDigest,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.requestRef,
		fixture.requestDigest,
		fixture.stageRef,
		fixture.stageDigest,
		typedMemoryRecordedAt46,
		[]byte("canonical-strict-resolution-basis:v47"),
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_authority_resolutions (
			authority_resolution_ref,
			authority_resolution_digest,
			project_id,
			authority_resolution_kind,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			trusted_cli_source_ref,
			trusted_cli_source_digest,
			strict_basis_ref,
			strict_basis_digest,
			explicit_resolution_ref,
			explicit_resolution_digest,
			strict_resolution_ref,
			strict_resolution_digest,
			evaluated_at,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, 'strict_permission',
			?, ?, ?, ?,
			NULL, NULL, ?, ?,
			NULL, NULL, ?, ?, ?, ?, ?
		)`,
		resolutionRef,
		resolutionDigest,
		fixture.projectID,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.requestRef,
		fixture.requestDigest,
		basisRef,
		basisDigest,
		resolutionRef,
		resolutionDigest,
		typedMemoryRecordedAt46,
		[]byte("canonical-strict-permission-resolution:v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_strict_permission_resolutions (
			authority_resolution_ref,
			authority_resolution_digest,
			basis_ref,
			basis_digest,
			speech_act_record_ref,
			speech_act_record_digest,
			permission_ref,
			permission_digest
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		resolutionRef,
		resolutionDigest,
		basisRef,
		basisDigest,
		speechRef,
		speechDigest,
		permissionRef,
		permissionDigest,
	)
	finish := transaction.Commit(ctx)
	if !finish.Succeeded() {
		t.Fatalf("commit strict Permission resolution union: %v", finish.Err())
	}
	for _, table := range []string{
		"project_typeenv_head_selection_speech_act_records",
		"project_typeenv_head_selection_permissions_v3",
		"project_typeenv_head_selection_authority_resolution_bases",
		"project_typeenv_head_selection_authority_resolutions",
		"project_typeenv_head_selection_strict_permission_resolutions",
	} {
		assertTypedMemoryTableRowCount45(t, database, table, 1)
	}
	assertTypedMemoryForeignKeysClean45(t, database)
}

func migrateProjectTypeEnvHeadSelection47(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	if err := Migrate(
		database,
		"schema_version",
		[]Migration{projectTypeEnvHeadSelectionMigration47},
	); err != nil {
		t.Fatalf("migrate database through v47: %v", err)
	}
}

func assertProjectTypeEnvImmutableError47(
	t *testing.T,
	err error,
) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("ProjectTypeEnv immutable-row error = %v", err)
	}
}

func assertProjectTypeEnvVerifierEditionSchema47(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	rows, err := database.Query(
		"PRAGMA table_xinfo(project_typeenv_head_selection_authority_uses)",
	)
	if err != nil {
		t.Fatalf("inspect v47 authority-use verifier columns: %v", err)
	}
	defer rows.Close()
	editionCount := 0
	digestCount := 0
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
			t.Fatalf("scan v47 authority-use verifier column: %v", err)
		}
		if name == "verifier_digest" {
			digestCount++
		}
		if name != "verifier_edition" {
			continue
		}
		editionCount++
		if strings.ToUpper(columnType) != "INTEGER" ||
			notNull != 1 ||
			defaultValue.Valid ||
			primaryKey != 0 ||
			hidden != 0 {
			t.Fatalf(
				"v47 verifier edition column shape = %q notNull=%d default=%v pk=%d hidden=%d",
				columnType,
				notNull,
				defaultValue,
				primaryKey,
				hidden,
			)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate v47 authority-use verifier columns: %v", err)
	}
	if editionCount != 1 || digestCount != 0 {
		t.Fatalf(
			"v47 authority-use verifier columns edition/digest = %d/%d; want 1/0",
			editionCount,
			digestCount,
		)
	}
	var tableSQL string
	err = database.QueryRow(
		`SELECT sql
		FROM sqlite_master
		WHERE type = 'table'
			AND name = 'project_typeenv_head_selection_authority_uses'`,
	).Scan(&tableSQL)
	if err != nil {
		t.Fatalf("read v47 authority-use table DDL: %v", err)
	}
	normalized := strings.ToLower(normalizeSQLiteDDL47(tableSQL))
	if !strings.Contains(
		normalized,
		"verifier_edition integer not null check(verifier_edition > 0)",
	) {
		t.Fatalf("v47 authority-use DDL lacks positive verifier-edition guard")
	}
}

func sqliteMasterSnapshot47(
	t *testing.T,
	database *sql.DB,
) map[string]string {
	t.Helper()
	rows, err := database.Query(
		`SELECT type, name, COALESCE(sql, '')
		FROM sqlite_master
		ORDER BY type, name`,
	)
	if err != nil {
		t.Fatalf("read sqlite_master snapshot: %v", err)
	}
	defer rows.Close()
	snapshot := make(map[string]string)
	for rows.Next() {
		var kind string
		var name string
		var sqlText string
		if err := rows.Scan(&kind, &name, &sqlText); err != nil {
			t.Fatalf("scan sqlite_master snapshot: %v", err)
		}
		snapshot[kind+"/"+name] = sqlText
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master snapshot: %v", err)
	}
	return snapshot
}

func equalSQLiteMasterSnapshots47(
	left map[string]string,
	right map[string]string,
) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func insertSecondProjectTypeEnvExecutable47(
	t *testing.T,
	database *sql.DB,
) string {
	t.Helper()
	digest := typedMemoryDigest45("9")
	typeEnvRef := "typeenv:" + digest
	insertProjectTypeEnvExecutableSnapshot47(
		t,
		database,
		typeEnvRef,
		"p8g-v47",
	)
	return typeEnvRef
}

func insertProjectTypeEnvExecutableSnapshot47(
	t *testing.T,
	database *sql.DB,
	typeEnvRef string,
	suffix string,
) {
	t.Helper()
	verificationRef := "project-typeenv-verification:" + suffix
	_, err := database.Exec(
		`INSERT INTO project_typeenv_composite_verifications (
			verification_ref,
			verification_digest,
			lowerer_schema_version,
			canonical_schema_version,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?)`,
		verificationRef,
		"verification-digest:"+suffix,
		"lowerer-schema:p8g-v47",
		"verification-schema:p8g-v47",
		[]byte("canonical-composite-verification:"+suffix),
	)
	if err != nil {
		t.Fatalf("insert executable TypeEnv verification %s: %v", suffix, err)
	}
	_, err = database.Exec(
		`INSERT INTO project_typeenv_executable_snapshots (
			type_env_ref,
			snapshot_digest,
			lowered_environment_digest,
			source_revision,
			compiler_schema_version,
			lowerer_schema_version,
			verification_ref,
			canonical_schema_version,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		typeEnvRef,
		"snapshot-digest:"+suffix,
		"lowered-environment-digest:"+suffix,
		"source-revision:"+suffix,
		"compiler-schema:v1",
		"lowerer-schema:p8g-v47",
		verificationRef,
		"executable-snapshot-schema:p8g-v47",
		[]byte("canonical-executable-type-env:"+suffix),
	)
	if err != nil {
		t.Fatalf("insert executable TypeEnv snapshot %s: %v", suffix, err)
	}
}

func insertGenericTypeEnvSnapshot47(
	t *testing.T,
	database *sql.DB,
	digestSeed string,
) string {
	t.Helper()
	digest := typedMemoryDigest45(digestSeed)
	typeEnvRef := "typeenv:" + digest
	_, err := database.Exec(
		`INSERT INTO typed_memory_type_env_snapshots (
			type_env_ref,
			artifact_digest,
			snapshot_format,
			canonical_bytes,
			source_revision,
			compiler_schema_version,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		typeEnvRef,
		digest,
		"haft.typed-memory.type-env/v1",
		[]byte("canonical-generic-type-env:"+digestSeed),
		"source-revision:"+digestSeed,
		"compiler-schema:v1",
		typedMemoryRecordedAt46,
	)
	if err != nil {
		t.Fatalf("insert generic TypeEnv snapshot %s: %v", digestSeed, err)
	}
	return typeEnvRef
}

func assertTypeEnvCoordinateOwner47(
	t *testing.T,
	database *sql.DB,
	typeEnvRef string,
	expectedKind string,
	expectedGenericRef string,
	expectedExecutableRef string,
) {
	t.Helper()
	var kind string
	var genericRef string
	var executableRef string
	err := database.QueryRow(
		`SELECT
			representation_kind,
			COALESCE(generic_snapshot_ref, ''),
			COALESCE(project_executable_ref, '')
		FROM typed_memory_type_env_coordinates
		WHERE type_env_ref = ?`,
		typeEnvRef,
	).Scan(&kind, &genericRef, &executableRef)
	if err != nil {
		t.Fatalf("read TypeEnv coordinate %s: %v", typeEnvRef, err)
	}
	if kind != expectedKind ||
		genericRef != expectedGenericRef ||
		executableRef != expectedExecutableRef {
		t.Fatalf(
			"TypeEnv coordinate %s = (%q, %q, %q); want (%q, %q, %q)",
			typeEnvRef,
			kind,
			genericRef,
			executableRef,
			expectedKind,
			expectedGenericRef,
			expectedExecutableRef,
		)
	}
}

func assertTypedMemoryTableRowCountWhere47(
	t *testing.T,
	database *sql.DB,
	table string,
	column string,
	value string,
	expected int,
) {
	t.Helper()
	var actual int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM "+quoteSQLiteIdentifier(table)+
			" WHERE "+quoteSQLiteIdentifier(column)+" = ?",
		value,
	).Scan(&actual)
	if err != nil {
		t.Fatalf("count %s rows by %s: %v", table, column, err)
	}
	if actual != expected {
		t.Fatalf(
			"%s rows where %s=%q = %d; want %d",
			table,
			column,
			value,
			actual,
			expected,
		)
	}
}

func insertMinimalTypeEnvActivation47(
	execer typedMemoryExecer45,
	fixture typedMemoryDeclarationFixture45,
	resultTypeEnvRef string,
) error {
	_, err := execer.Exec(
		`INSERT INTO typed_memory_type_env_activations (
			project_id,
			event_ref,
			change_ordinal,
			activation_ref,
			activation_digest,
			canonical_activation_bytes,
			request_ref,
			request_digest,
			content_ref,
			content_digest,
			authority_use_ref,
			authority_use_digest,
			work_ref,
			basis_type_env_ref,
			result_type_env_ref,
			stage_ref,
			stage_digest,
			head_ref,
			expected_graph_revision,
			committed_graph_revision,
			committed_head_revision,
			recorded_at
		) VALUES (?, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
		fixture.projectID,
		fixture.eventRef,
		"activation:"+fixture.eventRef,
		fixture.changeSetDigest,
		[]byte("activation-delta"),
		"request:"+fixture.eventRef,
		typedMemoryDigest45("1"),
		"content:"+fixture.eventRef,
		typedMemoryDigest45("2"),
		"authority-use:"+fixture.eventRef,
		typedMemoryDigest45("3"),
		"work:"+fixture.eventRef,
		fixture.typeEnvRef,
		resultTypeEnvRef,
		"stage:"+fixture.eventRef,
		typedMemoryDigest45("4"),
		"project-typeenv-head:"+fixture.projectID,
		fixture.expectedRevision,
		fixture.graphRevision,
		typedMemoryRecordedAt46,
	)
	return err
}

func newProjectTypeEnvHeadState47(
	t *testing.T,
	projectText string,
	typeEnvRefText string,
	revisionValue uint64,
) projecttypeenvselection.ProjectTypeEnvHeadState {
	t.Helper()
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		t.Fatalf("parse v47 project ID: %v", err)
	}
	typeEnvRef, err := typedmemory.ParseTypeEnvRef(typeEnvRefText)
	if err != nil {
		t.Fatalf("parse v47 TypeEnv ref: %v", err)
	}
	revision, err := projecttypeenvselection.NewHeadRevision(revisionValue)
	if err != nil {
		t.Fatalf("build v47 head revision: %v", err)
	}
	state, err := projecttypeenvselection.SealProjectTypeEnvHeadState(
		projecttypeenvselection.ProjectTypeEnvHeadStateInput{
			Project:           project,
			SelectedComposite: typeEnvRef,
			Revision:          revision,
		},
	)
	if err != nil {
		t.Fatalf("seal v47 head state: %v", err)
	}
	return state
}

type projectTypeEnvHeadEffectFixture47 struct {
	projectID                  string
	headRef                    string
	basisTypeEnvRef            string
	resultTypeEnvRef           string
	stageRef                   string
	stageDigest                string
	verificationRef            string
	verificationDigest         string
	proofRef                   string
	proofDigest                string
	graphSnapshotRef           string
	graphSnapshotDigest        string
	requestRef                 string
	requestDigest              string
	originalIdempotencyKey     string
	orderedExtensionsDigest    string
	canonicalOrderedExtensions []byte
	runtimeEvaluationBasisRef  string
	configBasisRef             string
	configBasisDigest          string
	configCarrierDigest        string
	modePolicyRef              string
	modePolicyDigest           string
	contentRef                 string
	contentDigest              string
	invocationRef              string
	invocationDigest           string
	resolutionRef              string
	resolutionDigest           string
	authorityUseRef            string
	authorityUseDigest         string
	workRecordRef              string
	workRecordDigest           string
	workRef                    string
	activationRef              string
	activationDigest           string
	activationBytes            []byte
	receiptRef                 string
	receiptDigest              string
	closureRef                 string
	closureDigest              string
	eventRef                   string
	eventDigest                string
	commitRef                  string
	storageIdempotencyKey      string
	projectionJobRef           string
	admissionEnvelopeDigest    string
	admissionBasisDigest       string
	manifestDigest             string
	materializationDigest      string
	verifierRef                string
	verifierEdition            int64
}

func newProjectTypeEnvHeadEffectFixture47(
	basisTypeEnvRef string,
	resultTypeEnvRef string,
) projectTypeEnvHeadEffectFixture47 {
	projectID := typedMemoryProjectID45
	return projectTypeEnvHeadEffectFixture47{
		projectID:                  projectID,
		headRef:                    "project-typeenv-head:" + projectID,
		basisTypeEnvRef:            basisTypeEnvRef,
		resultTypeEnvRef:           resultTypeEnvRef,
		stageRef:                   "project-typeenv-stage:p8g-v47",
		stageDigest:                typedMemoryDigest45("d"),
		verificationRef:            "project-typeenv-verification:p8g-v47",
		verificationDigest:         typedMemoryDigest45("0"),
		proofRef:                   "project-typeenv-no-prior-head-proof:p8g-v47",
		proofDigest:                typedMemoryDigest45("3"),
		graphSnapshotRef:           "project-graph-snapshot:p8g-v47",
		graphSnapshotDigest:        typedMemoryDigest45("f"),
		requestRef:                 "project-typeenv-head-selection-request:p8g-v47",
		requestDigest:              typedMemoryDigest45("4"),
		originalIdempotencyKey:     "p8g-v47-public-key",
		orderedExtensionsDigest:    typedMemoryDigest45("e"),
		canonicalOrderedExtensions: []byte("[]"),
		runtimeEvaluationBasisRef:  "runtime-evaluation-basis:p8g-v47",
		configBasisRef:             "project-typeenv-config-basis:p8g-v47",
		configBasisDigest:          typedMemoryDigest45("1"),
		configCarrierDigest:        typedMemoryDigest45("2"),
		modePolicyRef:              "project-typeenv-mode-policy:p8g-v47",
		modePolicyDigest:           typedMemoryDigest45("2"),
		contentRef:                 "claim:p8g-v47",
		contentDigest:              typedMemoryDigest45("5"),
		invocationRef:              "h-decide-invocation:p8g-v47",
		invocationDigest:           typedMemoryDigest45("6"),
		resolutionRef:              "project-typeenv-authority-resolution:p8g-v47",
		resolutionDigest:           typedMemoryDigest45("7"),
		authorityUseRef:            "project-typeenv-authority-use:p8g-v47",
		authorityUseDigest:         typedMemoryDigest45("8"),
		workRecordRef:              "project-typeenv-cas-work-record:p8g-v47",
		workRecordDigest:           typedMemoryDigest45("9"),
		workRef:                    "work:project-typeenv-head-cas:p8g-v47",
		activationRef:              "type-env-activation:p8g-v47",
		activationDigest:           typedMemoryDigest45("a"),
		activationBytes:            []byte("canonical-type-env-activation:p8g-v47"),
		receiptRef:                 "project-typeenv-selection-receipt:p8g-v47",
		receiptDigest:              typedMemoryDigest45("b"),
		closureRef:                 "project-typeenv-selection-closure:p8g-v47",
		closureDigest:              typedMemoryDigest45("c"),
		eventRef:                   "event:project-typeenv-activation:p8g-v47",
		eventDigest:                typedMemoryDigest45("1"),
		commitRef:                  "commit:project-typeenv-activation:p8g-v47",
		storageIdempotencyKey:      "p8g-v47-storage-key",
		projectionJobRef:           "projection-job:p8g-v47",
		admissionEnvelopeDigest:    typedMemoryDigest45("6"),
		admissionBasisDigest:       typedMemoryDigest45("7"),
		manifestDigest:             typedMemoryDigest45("8"),
		materializationDigest:      typedMemoryDigest45("9"),
		verifierRef:                "project-typeenv-head-selection-verifier:p8g-v47",
		verifierEdition:            1,
	}
}

func insertProjectTypeEnvCandidateStage47(
	t *testing.T,
	database *sql.DB,
	fixture projectTypeEnvHeadEffectFixture47,
) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO project_typeenv_artifacts (
			artifact_kind,
			artifact_ref,
			artifact_digest,
			canonical_schema_version,
			producer_schema_version,
			canonical_bytes
		) VALUES ('base_type_env', ?, ?, ?, ?, ?)`,
		fixture.basisTypeEnvRef,
		typedMemoryDigest45("2"),
		"base-type-env-schema:p8g-v47",
		"compiler-schema:p8g-v47",
		[]byte("canonical-base-type-env-artifact:p8g-v47"),
	)
	if err != nil {
		t.Fatalf("insert v47 base TypeEnv artifact: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO project_typeenv_stages (
			stage_ref,
			stage_digest,
			project_id,
			composite_verification_ref,
			canonical_schema_version,
			canonical_bytes,
			executable_type_env_ref
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		fixture.stageRef,
		fixture.stageDigest,
		fixture.projectID,
		fixture.verificationRef,
		"stage-schema:p8g-v47",
		[]byte("canonical-project-typeenv-stage:p8g-v47"),
		fixture.resultTypeEnvRef,
	)
	if err != nil {
		t.Fatalf("insert v47 Stage: %v", err)
	}
}

func insertProjectTypeEnvHeadEffectSources47(
	t *testing.T,
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	fixture projectTypeEnvHeadEffectFixture47,
) {
	t.Helper()
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_config_authority_bases (
			config_authority_basis_ref,
			config_authority_basis_digest,
			project_id,
			authority_mode,
			config_carrier_ref,
			config_carrier_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, 'explicit_h_decide', ?, ?, ?, ?)`,
		fixture.configBasisRef,
		fixture.configBasisDigest,
		fixture.projectID,
		".haft/config.yaml",
		fixture.configCarrierDigest,
		[]byte("canonical-config-authority-basis:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_mode_policies (
			mode_policy_ref,
			mode_policy_digest,
			project_id,
			authority_mode,
			config_authority_basis_ref,
			config_authority_basis_digest,
			resolver_policy_ref,
			resolver_policy_edition,
			resolver_policy_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, 'explicit_h_decide', ?, ?, NULL, NULL, NULL, ?, ?)`,
		fixture.modePolicyRef,
		fixture.modePolicyDigest,
		fixture.projectID,
		fixture.configBasisRef,
		fixture.configBasisDigest,
		[]byte("canonical-mode-policy:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_no_prior_head_proofs (
			proof_ref,
			proof_digest,
			project_id,
			head_ref,
			graph_snapshot_ref,
			graph_snapshot_digest,
			expected_graph_revision,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		fixture.proofRef,
		fixture.proofDigest,
		fixture.projectID,
		fixture.headRef,
		fixture.graphSnapshotRef,
		fixture.graphSnapshotDigest,
		[]byte("canonical-no-prior-head-proof:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_requests (
			request_ref,
			request_digest,
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
		) VALUES (
			?, ?, ?, ?, 'genesis', ?, ?,
			NULL, NULL, NULL,
			?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?
		)`,
		fixture.requestRef,
		fixture.requestDigest,
		fixture.projectID,
		fixture.headRef,
		fixture.proofRef,
		fixture.proofDigest,
		fixture.basisTypeEnvRef,
		fixture.orderedExtensionsDigest,
		fixture.canonicalOrderedExtensions,
		fixture.runtimeEvaluationBasisRef,
		fixture.resultTypeEnvRef,
		fixture.stageRef,
		fixture.stageDigest,
		fixture.originalIdempotencyKey,
		[]byte("canonical-head-selection-request:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_authorization_contents (
			content_ref,
			content_ref_kind,
			content_digest,
			project_id,
			request_ref,
			request_digest,
			judgement_context_ref,
			action_kind,
			valid_from,
			valid_until,
			canonical_bytes,
			recorded_at
		) VALUES (?, 'claim_id', ?, ?, ?, ?, ?, 'genesis', ?, ?, ?, ?)`,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.projectID,
		fixture.requestRef,
		fixture.requestDigest,
		"judgement-context:p8g-v47",
		"2026-07-16T13:00:00Z",
		"2026-07-16T15:00:00Z",
		[]byte("canonical-authorization-content:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_trusted_cli_sources (
			trusted_cli_source_ref,
			trusted_cli_source_digest,
			project_id,
			mode_policy_ref,
			mode_policy_digest,
			config_authority_basis_ref,
			config_authority_basis_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.invocationRef,
		fixture.invocationDigest,
		fixture.projectID,
		fixture.modePolicyRef,
		fixture.modePolicyDigest,
		fixture.configBasisRef,
		fixture.configBasisDigest,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.requestRef,
		fixture.requestDigest,
		[]byte("canonical-trusted-dedicated-cli-source:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_authority_resolutions (
			authority_resolution_ref,
			authority_resolution_digest,
			project_id,
			authority_resolution_kind,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			trusted_cli_source_ref,
			trusted_cli_source_digest,
			strict_basis_ref,
			strict_basis_digest,
			explicit_resolution_ref,
			explicit_resolution_digest,
			strict_resolution_ref,
			strict_resolution_digest,
			evaluated_at,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, 'explicit_policy_acceptance',
			?, ?, ?, ?, ?, ?,
			NULL, NULL, ?, ?, NULL, NULL, ?, ?, ?
		)`,
		fixture.resolutionRef,
		fixture.resolutionDigest,
		fixture.projectID,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.requestRef,
		fixture.requestDigest,
		fixture.invocationRef,
		fixture.invocationDigest,
		fixture.resolutionRef,
		fixture.resolutionDigest,
		typedMemoryRecordedAt46,
		[]byte("canonical-authority-resolution:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_explicit_policy_acceptance_resolutions (
			authority_resolution_ref,
			authority_resolution_digest,
			trusted_cli_source_ref,
			trusted_cli_source_digest
		) VALUES (?, ?, ?, ?)`,
		fixture.resolutionRef,
		fixture.resolutionDigest,
		fixture.invocationRef,
		fixture.invocationDigest,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_authority_uses (
			authority_use_ref,
			authority_use_digest,
			project_id,
			original_idempotency_key,
			authority_resolution_kind,
			authority_resolution_ref,
			authority_resolution_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			work_ref,
			receipt_ref,
			predecessor_kind,
			predecessor_head_ref,
			predecessor_head_revision,
			predecessor_selected_composite_ref,
			base_type_env_ref,
			ordered_extension_refs_digest,
			canonical_ordered_extension_refs,
			runtime_evaluation_basis_ref,
			selected_composite_ref,
			stage_ref,
			stage_digest,
			expected_graph_revision,
			committed_head_revision,
			committed_graph_revision,
			verifier_ref,
			verifier_edition,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, ?, 'explicit_policy_acceptance',
			?, ?, ?, ?, ?, ?, ?, ?,
			'genesis', NULL, NULL, NULL,
			?, ?, ?, ?, ?, ?, ?, 0, 1, 1, ?, ?, ?, ?
		)`,
		fixture.authorityUseRef,
		fixture.authorityUseDigest,
		fixture.projectID,
		fixture.originalIdempotencyKey,
		fixture.resolutionRef,
		fixture.resolutionDigest,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.requestRef,
		fixture.requestDigest,
		fixture.workRef,
		fixture.receiptRef,
		fixture.basisTypeEnvRef,
		fixture.orderedExtensionsDigest,
		fixture.canonicalOrderedExtensions,
		fixture.runtimeEvaluationBasisRef,
		fixture.resultTypeEnvRef,
		fixture.stageRef,
		fixture.stageDigest,
		fixture.verifierRef,
		fixture.verifierEdition,
		[]byte("canonical-authority-use:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_cas_work_records (
			cas_work_record_ref,
			cas_work_record_digest,
			work_ref,
			project_id,
			authority_use_ref,
			authority_use_digest,
			receipt_ref,
			activation_ref,
			method_description_ref,
			executor_system_ref,
			executor_role_ref,
			bounded_context_ref,
			work_started_at,
			effect_sealed_at,
			committed_head_revision,
			committed_graph_revision,
			selected_composite_ref,
			canonical_bytes,
			recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?, ?)`,
		fixture.workRecordRef,
		fixture.workRecordDigest,
		fixture.workRef,
		fixture.projectID,
		fixture.authorityUseRef,
		fixture.authorityUseDigest,
		fixture.receiptRef,
		fixture.activationRef,
		"method-description:project-typeenv-head-cas:p8g-v47",
		"system:haft-software-system",
		"role:project-governance-substrate",
		"bounded-context:haft-project-memory",
		typedMemoryRecordedAt46,
		typedMemoryRecordedAt46,
		fixture.resultTypeEnvRef,
		[]byte("canonical-project-typeenv-cas-work-record:p8g-v47"),
		typedMemoryRecordedAt46,
	)
}

func insertProjectTypeEnvHeadEffectClosure47(
	t *testing.T,
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	fixture projectTypeEnvHeadEffectFixture47,
	headStateDigest string,
	canonicalHeadState []byte,
) {
	t.Helper()
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO typed_memory_graph_events (
			project_id,
			event_ref,
			commit_ref,
			event_digest,
			expected_revision,
			graph_revision,
			basis_type_env_ref,
			result_type_env_ref,
			change_set_digest,
			canonical_change_set_bytes,
			change_count,
			event_kind,
			authority_class,
			request_provenance_ref,
			recorded_at
		) VALUES (
			?, ?, ?, ?, 0, 1, ?, ?, ?, ?, 1,
			'activate_type_env', 'manual_type_env_activation', ?, ?
		)`,
		fixture.projectID,
		fixture.eventRef,
		fixture.commitRef,
		fixture.eventDigest,
		fixture.basisTypeEnvRef,
		fixture.resultTypeEnvRef,
		fixture.activationDigest,
		fixture.activationBytes,
		fixture.requestRef,
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO typed_memory_event_writer_generations (
			project_id,
			event_ref,
			writer_generation,
			provenance_kind
		) VALUES (?, ?, 46, 'writer_v46')`,
		fixture.projectID,
		fixture.eventRef,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO typed_memory_idempotency_history (
			project_id,
			idempotency_key,
			change_set_digest,
			event_ref,
			graph_revision,
			result_digest,
			recorded_at
		) VALUES (?, ?, ?, ?, 1, ?, ?)`,
		fixture.projectID,
		fixture.storageIdempotencyKey,
		fixture.activationDigest,
		fixture.eventRef,
		fixture.eventDigest,
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO typed_memory_projection_jobs (
			project_id,
			projection_job_ref,
			semantic_event_ref,
			graph_revision,
			target_kind,
			input_event_digest,
			recorded_at
		) VALUES (?, ?, ?, 1, 'project_carriers', ?, ?)`,
		fixture.projectID,
		fixture.projectionJobRef,
		fixture.eventRef,
		fixture.eventDigest,
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO typed_memory_event_admission_bases (
			project_id,
			event_ref,
			event_digest,
			admission_basis_kind,
			type_env_ref,
			basis_graph_revision,
			request_digest,
			canonical_request_bytes,
			semantic_digest,
			canonical_semantic_bytes,
			admission_envelope_digest,
			canonical_admission_envelope_bytes,
			admission_basis_digest,
			canonical_admission_basis_bytes,
			materialization_manifest_digest,
			canonical_materialization_manifest_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, 'snapshot_only', ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)`,
		fixture.projectID,
		fixture.eventRef,
		fixture.eventDigest,
		fixture.basisTypeEnvRef,
		fixture.requestDigest,
		[]byte("canonical-activation-request:p8g-v47"),
		fixture.activationDigest,
		fixture.activationBytes,
		fixture.admissionEnvelopeDigest,
		[]byte("canonical-activation-envelope:p8g-v47"),
		fixture.admissionBasisDigest,
		[]byte("canonical-activation-basis:p8g-v47"),
		fixture.manifestDigest,
		[]byte("canonical-activation-manifest:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO typed_memory_type_env_activations (
			project_id,
			event_ref,
			change_ordinal,
			activation_ref,
			activation_digest,
			canonical_activation_bytes,
			request_ref,
			request_digest,
			content_ref,
			content_digest,
			authority_use_ref,
			authority_use_digest,
			work_ref,
			basis_type_env_ref,
			result_type_env_ref,
			stage_ref,
			stage_digest,
			head_ref,
			expected_graph_revision,
			committed_graph_revision,
			committed_head_revision,
			recorded_at
		) VALUES (
			?, ?, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 1, 1, ?
		)`,
		fixture.projectID,
		fixture.eventRef,
		fixture.activationRef,
		fixture.activationDigest,
		fixture.activationBytes,
		fixture.requestRef,
		fixture.requestDigest,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.authorityUseRef,
		fixture.authorityUseDigest,
		fixture.workRef,
		fixture.basisTypeEnvRef,
		fixture.resultTypeEnvRef,
		fixture.stageRef,
		fixture.stageDigest,
		fixture.headRef,
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_history (
			project_id,
			head_ref,
			head_revision,
			selected_composite_ref,
			graph_revision,
			graph_event_ref,
			graph_commit_ref,
			activation_ref,
			activation_digest,
			request_ref,
			request_digest,
			authority_use_ref,
			authority_use_digest,
			work_ref,
			receipt_ref,
			head_state_digest,
			canonical_head_state_bytes,
			recorded_at
		) VALUES (
			?, ?, 1, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)`,
		fixture.projectID,
		fixture.headRef,
		fixture.resultTypeEnvRef,
		fixture.eventRef,
		fixture.commitRef,
		fixture.activationRef,
		fixture.activationDigest,
		fixture.requestRef,
		fixture.requestDigest,
		fixture.authorityUseRef,
		fixture.authorityUseDigest,
		fixture.workRef,
		fixture.receiptRef,
		headStateDigest,
		canonicalHeadState,
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_receipts (
			receipt_ref,
			receipt_digest,
			project_id,
			authority_use_ref,
			authority_use_digest,
			cas_work_record_ref,
			cas_work_record_digest,
			work_ref,
			activation_ref,
			activation_digest,
			authority_resolution_ref,
			authority_resolution_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			head_ref,
			head_revision,
			selected_composite_ref,
			graph_revision,
			graph_event_ref,
			graph_commit_ref,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, 1, ?, 1, ?, ?, ?, ?
		)`,
		fixture.receiptRef,
		fixture.receiptDigest,
		fixture.projectID,
		fixture.authorityUseRef,
		fixture.authorityUseDigest,
		fixture.workRecordRef,
		fixture.workRecordDigest,
		fixture.workRef,
		fixture.activationRef,
		fixture.activationDigest,
		fixture.resolutionRef,
		fixture.resolutionDigest,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.requestRef,
		fixture.requestDigest,
		fixture.headRef,
		fixture.resultTypeEnvRef,
		fixture.eventRef,
		fixture.commitRef,
		[]byte("canonical-project-typeenv-selection-receipt:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO project_typeenv_head_selection_closures (
			closure_ref,
			closure_digest,
			project_id,
			authority_use_ref,
			authority_use_digest,
			cas_work_record_ref,
			cas_work_record_digest,
			receipt_ref,
			receipt_digest,
			activation_ref,
			activation_digest,
			authority_resolution_ref,
			authority_resolution_digest,
			content_ref,
			content_digest,
			request_ref,
			request_digest,
			head_ref,
			head_revision,
			head_state_digest,
			graph_revision,
			graph_event_ref,
			graph_event_digest,
			graph_commit_ref,
			canonical_bytes,
			recorded_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, 1, ?, 1, ?, ?, ?, ?, ?
		)`,
		fixture.closureRef,
		fixture.closureDigest,
		fixture.projectID,
		fixture.authorityUseRef,
		fixture.authorityUseDigest,
		fixture.workRecordRef,
		fixture.workRecordDigest,
		fixture.receiptRef,
		fixture.receiptDigest,
		fixture.activationRef,
		fixture.activationDigest,
		fixture.resolutionRef,
		fixture.resolutionDigest,
		fixture.contentRef,
		fixture.contentDigest,
		fixture.requestRef,
		fixture.requestDigest,
		fixture.headRef,
		headStateDigest,
		fixture.eventRef,
		fixture.eventDigest,
		fixture.commitRef,
		[]byte("canonical-project-typeenv-selection-closure:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO typed_memory_commit_materialization_closures (
			project_id,
			event_ref,
			commit_ref,
			event_digest,
			admission_basis_kind,
			request_digest,
			semantic_digest,
			admission_envelope_digest,
			admission_basis_digest,
			materialization_manifest_digest,
			materialization_digest,
			canonical_materialization_bytes,
			entity_count,
			entity_context_count,
			entity_declaration_count,
			context_slice_catalog_count,
			context_slice_count,
			value_blob_count,
			observable_input_blob_count,
			relation_count,
			relation_slot_count,
			relation_filler_count,
			ordered_candidate_prefix_count,
			reference_resolution_use_count,
			memberof_evaluation_count,
			memberof_input_count,
			memberof_use_count,
			alias_change_count,
			retraction_count,
			recorded_at,
			type_env_activation_count
		) VALUES (
			?, ?, ?, ?, 'snapshot_only',
			?, ?, ?, ?, ?, ?, ?,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, ?, 1
		)`,
		fixture.projectID,
		fixture.eventRef,
		fixture.commitRef,
		fixture.eventDigest,
		fixture.requestDigest,
		fixture.activationDigest,
		fixture.admissionEnvelopeDigest,
		fixture.admissionBasisDigest,
		fixture.manifestDigest,
		fixture.materializationDigest,
		[]byte("canonical-activation-materialization:p8g-v47"),
		typedMemoryRecordedAt46,
	)
	mustExecuteProjectTypeEnvHeadEffect47(
		t,
		ctx,
		transaction,
		`INSERT INTO typed_memory_graph_commits (
			project_id,
			commit_ref,
			event_ref,
			event_digest,
			expected_revision,
			graph_revision,
			change_set_digest,
			idempotency_key,
			projection_job_ref,
			entity_count,
			entity_context_count,
			recorded_at
		) VALUES (?, ?, ?, ?, 0, 1, ?, ?, ?, 0, 0, ?)`,
		fixture.projectID,
		fixture.commitRef,
		fixture.eventRef,
		fixture.eventDigest,
		fixture.activationDigest,
		fixture.storageIdempotencyKey,
		fixture.projectionJobRef,
		typedMemoryRecordedAt46,
	)
}

func mustExecuteProjectTypeEnvHeadEffect47(
	t *testing.T,
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	statement string,
	arguments ...any,
) {
	t.Helper()
	_, err := transaction.Execute(ctx, statement, arguments)
	if err != nil {
		t.Fatalf("write complete v47 ProjectTypeEnv head effect: %v", err)
	}
}
