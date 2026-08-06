package db

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"
)

func TestProjectTypeEnvHeadSelectionMigration48PreservesCompleteV47GenesisEffect(
	t *testing.T,
) {
	t.Parallel()

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

	var proofBytesBefore []byte
	var requestBytesBefore []byte
	err := database.QueryRow(
		`SELECT canonical_bytes
		FROM project_typeenv_no_prior_head_proofs
		WHERE proof_ref = ? AND proof_digest = ?`,
		fixture.proofRef,
		fixture.proofDigest,
	).Scan(&proofBytesBefore)
	if err != nil {
		t.Fatalf("read v47 no-prior-head proof: %v", err)
	}
	err = database.QueryRow(
		`SELECT canonical_bytes
		FROM project_typeenv_head_selection_requests
		WHERE request_ref = ? AND request_digest = ?`,
		fixture.requestRef,
		fixture.requestDigest,
	).Scan(&requestBytesBefore)
	if err != nil {
		t.Fatalf("read v47 head-selection request: %v", err)
	}

	migrateProjectTypeEnvHeadSelection48(t, database)

	assertMigrationVersionPresent(t, database, 48)
	var observationSchema string
	var observedAt sql.NullString
	var proofBytesAfter []byte
	err = database.QueryRow(
		`SELECT observation_schema, observed_at, canonical_bytes
		FROM project_typeenv_no_prior_head_proofs
		WHERE proof_ref = ? AND proof_digest = ?`,
		fixture.proofRef,
		fixture.proofDigest,
	).Scan(&observationSchema, &observedAt, &proofBytesAfter)
	if err != nil {
		t.Fatalf("read migrated v47 no-prior-head proof: %v", err)
	}
	if observationSchema != projectTypeEnvProofObservationLegacy47 ||
		observedAt.Valid ||
		!bytes.Equal(proofBytesAfter, proofBytesBefore) {
		t.Fatalf(
			"migrated legacy proof = schema %q observed_at %#v bytes_equal %t",
			observationSchema,
			observedAt,
			bytes.Equal(proofBytesAfter, proofBytesBefore),
		)
	}

	var requestSchema string
	var proofRef string
	var proofDigest string
	var requestBytesAfter []byte
	err = database.QueryRow(
		`SELECT request_schema, no_prior_head_proof_ref,
			no_prior_head_proof_digest, canonical_bytes
		FROM project_typeenv_head_selection_requests
		WHERE request_ref = ? AND request_digest = ?`,
		fixture.requestRef,
		fixture.requestDigest,
	).Scan(
		&requestSchema,
		&proofRef,
		&proofDigest,
		&requestBytesAfter,
	)
	if err != nil {
		t.Fatalf("read migrated v47 head-selection request: %v", err)
	}
	if requestSchema != projectTypeEnvRequestSchemaV1 ||
		proofRef != fixture.proofRef ||
		proofDigest != fixture.proofDigest ||
		!bytes.Equal(requestBytesAfter, requestBytesBefore) {
		t.Fatalf(
			"migrated legacy request = schema %q proof (%q,%q) bytes_equal %t",
			requestSchema,
			proofRef,
			proofDigest,
			bytes.Equal(requestBytesAfter, requestBytesBefore),
		)
	}

	for _, table := range projectTypeEnvProofEffectTables48 {
		var effectProofRef sql.NullString
		var effectProofDigest sql.NullString
		err = database.QueryRow(
			`SELECT no_prior_head_proof_ref, no_prior_head_proof_digest
			FROM `+table+` LIMIT 1`,
		).Scan(&effectProofRef, &effectProofDigest)
		if err != nil {
			t.Fatalf("read migrated v47 effect proof from %s: %v", table, err)
		}
		if effectProofRef.Valid || effectProofDigest.Valid {
			t.Fatalf(
				"legacy effect table %s acquired new proof ownership (%#v,%#v)",
				table,
				effectProofRef,
				effectProofDigest,
			)
		}
	}

	_, err = database.Exec(
		`INSERT INTO project_typeenv_head_selection_requests (
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
		) VALUES (
			?, ?, ?, ?, ?, 'genesis', ?, ?,
			NULL, NULL, NULL,
			?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?
		)`,
		"project-typeenv-head-selection-request:forged-v1-after-v48",
		typedMemoryDigest45("a"),
		projectTypeEnvRequestSchemaV1,
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
		"forged-v1-after-v48",
		[]byte("forged-v1-after-v48"),
		typedMemoryRecordedAt46,
	)
	if err == nil || !strings.Contains(err.Error(), "v1 is read-only") {
		t.Fatalf("new legacy v1 request error = %v", err)
	}
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestProjectTypeEnvHeadSelectionMigration48AcceptsV2GenesisRequestAndEffectObservation(
	t *testing.T,
) {
	t.Parallel()

	database, basisTypeEnvRef := newTypedMemoryRawSQLDatabase46(t, true)
	defer database.Close()
	migrateProjectTypeEnvHeadSelection47(t, database)
	resultTypeEnvRef := insertSecondProjectTypeEnvExecutable47(t, database)
	fixture := newProjectTypeEnvHeadEffectFixture47(
		basisTypeEnvRef,
		resultTypeEnvRef,
	)
	insertProjectTypeEnvCandidateStage47(t, database, fixture)
	migrateProjectTypeEnvHeadSelection48(t, database)

	effectProofRef := "project-typeenv-no-prior-head-proof:p8g-v48-effect"
	effectProofDigest := typedMemoryDigest45("b")
	_, err := database.Exec(
		`INSERT INTO project_typeenv_no_prior_head_proofs (
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
		) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`,
		effectProofRef,
		effectProofDigest,
		fixture.projectID,
		fixture.headRef,
		fixture.graphSnapshotRef,
		fixture.graphSnapshotDigest,
		projectTypeEnvProofObservationEffectV1,
		"2026-07-16T13:59:59Z",
		[]byte("canonical-effect-owned-head-absence:p8g-v48"),
		typedMemoryRecordedAt46,
	)
	if err != nil {
		t.Fatalf("insert v48 effect-owned head-absence proof: %v", err)
	}

	requestRef := "project-typeenv-head-selection-request:p8g-v48"
	requestDigest := typedMemoryDigest45("c")
	_, err = database.Exec(
		`INSERT INTO project_typeenv_head_selection_requests (
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
		) VALUES (
			?, ?, ?, ?, ?, 'genesis', NULL, NULL,
			NULL, NULL, NULL,
			?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?
		)`,
		requestRef,
		requestDigest,
		projectTypeEnvRequestSchemaV2,
		fixture.projectID,
		fixture.headRef,
		fixture.basisTypeEnvRef,
		fixture.orderedExtensionsDigest,
		fixture.canonicalOrderedExtensions,
		fixture.runtimeEvaluationBasisRef,
		fixture.resultTypeEnvRef,
		fixture.stageRef,
		fixture.stageDigest,
		"p8g-v48-public-key",
		[]byte("canonical-head-selection-request:p8g-v48"),
		typedMemoryRecordedAt46,
	)
	if err != nil {
		t.Fatalf("insert v48 Genesis request without proposal-owned proof: %v", err)
	}

	var storedRequestSchema string
	var storedProofRef sql.NullString
	var storedProofDigest sql.NullString
	err = database.QueryRow(
		`SELECT request_schema, no_prior_head_proof_ref,
			no_prior_head_proof_digest
		FROM project_typeenv_head_selection_requests
		WHERE request_ref = ?`,
		requestRef,
	).Scan(
		&storedRequestSchema,
		&storedProofRef,
		&storedProofDigest,
	)
	if err != nil {
		t.Fatalf("read v48 Genesis request: %v", err)
	}
	if storedRequestSchema != projectTypeEnvRequestSchemaV2 ||
		storedProofRef.Valid ||
		storedProofDigest.Valid {
		t.Fatalf(
			"v48 request = schema %q proof (%#v,%#v)",
			storedRequestSchema,
			storedProofRef,
			storedProofDigest,
		)
	}

	_, err = database.Exec(
		`INSERT INTO project_typeenv_no_prior_head_proofs (
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
		) VALUES (?, ?, ?, ?, ?, ?, 0, ?, NULL, ?, ?)`,
		"project-typeenv-no-prior-head-proof:forged-legacy-v48",
		typedMemoryDigest45("d"),
		fixture.projectID,
		fixture.headRef,
		fixture.graphSnapshotRef,
		fixture.graphSnapshotDigest,
		projectTypeEnvProofObservationLegacy47,
		[]byte("forged-legacy-proof-v48"),
		typedMemoryRecordedAt46,
	)
	if err == nil || !strings.Contains(err.Error(), "effect-owned observation") {
		t.Fatalf("new legacy proof error = %v", err)
	}

	_, err = database.Exec(
		`INSERT INTO project_typeenv_no_prior_head_proofs (
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
		) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`,
		"project-typeenv-no-prior-head-proof:future-observation-v48",
		typedMemoryDigest45("e"),
		fixture.projectID,
		fixture.headRef,
		fixture.graphSnapshotRef,
		fixture.graphSnapshotDigest,
		projectTypeEnvProofObservationEffectV1,
		"2026-07-16T14:00:01Z",
		[]byte("future-observation-v48"),
		typedMemoryRecordedAt46,
	)
	if err == nil {
		t.Fatal("future effect-owned observation was accepted")
	}

	assertProjectTypeEnvHeadSelectionSchema48(t, database)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func TestProjectTypeEnvHeadSelectionMigration48UpgradesOlderDatabaseInSequence(
	t *testing.T,
) {
	t.Parallel()

	database := openDatabaseBeforeTypedMemoryStorageMigration46(t)
	defer database.Close()
	if err := Migrate(
		database,
		"schema_version",
		[]Migration{
			typedMemoryStorageMigration46,
			projectTypeEnvHeadSelectionMigration47,
			projectTypeEnvHeadSelectionMigration48,
		},
	); err != nil {
		t.Fatalf("migrate older database through v48: %v", err)
	}
	for _, version := range []int{46, 47, 48} {
		assertMigrationVersionPresent(t, database, version)
	}
	assertProjectTypeEnvHeadSelectionSchema48(t, database)
	assertTypedMemoryForeignKeysClean45(t, database)
}

func migrateProjectTypeEnvHeadSelection48(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	if err := Migrate(
		database,
		"schema_version",
		[]Migration{projectTypeEnvHeadSelectionMigration48},
	); err != nil {
		t.Fatalf("migrate database through v48: %v", err)
	}
}

func assertProjectTypeEnvHeadSelectionSchema48(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	for _, coordinate := range append(
		[]struct {
			table  string
			column string
		}{
			{"project_typeenv_no_prior_head_proofs", "observation_schema"},
			{"project_typeenv_no_prior_head_proofs", "observed_at"},
			{"project_typeenv_head_selection_requests", "request_schema"},
		},
		projectTypeEnvEffectProofColumns48()...,
	) {
		if !sqliteTestColumnExists48(t, database, coordinate.table, coordinate.column) {
			t.Fatalf(
				"v48 column %s.%s is missing",
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
		assertSQLiteObjectExists(t, database, "trigger", trigger)
	}
	assertSQLiteObjectAbsent(
		t,
		database,
		"trigger",
		"project_typeenv_head_selection_permissions_v3_v47_exact_source",
	)

	var strictPermissionSQL string
	err := database.QueryRow(
		`SELECT sql
		FROM sqlite_master
		WHERE type = 'trigger'
			AND name = 'project_typeenv_head_selection_permissions_v3_v48_exact_source'`,
	).Scan(&strictPermissionSQL)
	if err != nil {
		t.Fatalf("read v48 strict Permission trigger SQL: %v", err)
	}
	for _, falseEquality := range []string{
		"mode_policy.resolver_policy_ref = NEW.context_policy_ref",
		"mode_policy.resolver_policy_digest = NEW.context_policy_digest",
	} {
		if strings.Contains(strictPermissionSQL, falseEquality) {
			t.Fatalf(
				"v48 strict Permission trigger retained false equality %q",
				falseEquality,
			)
		}
	}
	if !strings.Contains(
		strictPermissionSQL,
		"mode_policy.authority_mode = 'strict_cli_speech_act'",
	) {
		t.Fatal("v48 strict Permission trigger lost strict-mode source binding")
	}
}

func projectTypeEnvEffectProofColumns48() []struct {
	table  string
	column string
} {
	columns := make(
		[]struct {
			table  string
			column string
		},
		0,
		len(projectTypeEnvProofEffectTables48)*2,
	)
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
	return columns
}

func sqliteTestColumnExists48(
	t *testing.T,
	database *sql.DB,
	table string,
	column string,
) bool {
	t.Helper()
	rows, err := database.Query("PRAGMA table_xinfo(" + table + ")")
	if err != nil {
		t.Fatalf("inspect v48 table %s: %v", table, err)
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
			t.Fatalf("scan v48 table %s column: %v", table, err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate v48 table %s columns: %v", table, err)
	}
	return false
}
