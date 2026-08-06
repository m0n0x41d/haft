package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestAuthorityProfileReconciliationFreshDatabase(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatalf("open fresh store: %v", err)
	}
	defer store.Close()

	assertMigrationVersionCount(t, store.conn, 36, 1)
	assertCanonicalAuthorityProfileSchema(t, store.conn)
}

func TestAuthorityProfileReconciliationReplacesExactEmptyLegacyV34(t *testing.T) {
	t.Parallel()

	database := openLegacyV34Database(t, filepath.Join(t.TempDir(), "legacy-empty.db"))
	defer database.Close()
	insertUnrelatedGraphAndArtifactData(t, database)

	fingerprint, err := authorityProfileSchemaFingerprint(database)
	if err != nil {
		t.Fatalf("fingerprint legacy schema: %v", err)
	}
	if fingerprint != legacyV34AuthorityProfileFingerprint {
		t.Fatalf("legacy fixture fingerprint = %s, want %s", fingerprint, legacyV34AuthorityProfileFingerprint)
	}
	if err := RunMigrations(database); err != nil {
		t.Fatalf("reconcile exact empty legacy schema: %v", err)
	}

	assertMigrationVersionCount(t, database, 35, 1)
	assertMigrationVersionCount(t, database, 36, 1)
	assertCanonicalAuthorityProfileSchema(t, database)
	assertUnrelatedGraphAndArtifactData(t, database)
}

func TestAuthorityProfileReconciliationRejectsNonEmptyLegacyV34(t *testing.T) {
	t.Parallel()

	database := openLegacyV34Database(t, filepath.Join(t.TempDir(), "legacy-nonempty.db"))
	defer database.Close()
	_, err := database.Exec(`INSERT INTO project_profile_projection_debt (
		event_id, debt_id, admission_id, admission_digest, project_root,
		ledger_revision, profile_payload_digest, projection_path, event_kind,
		reason_code, detail, expected_projection_digest,
		observed_projection_digest, supersedes_event_id, recorded_at
	) VALUES (
		'event:legacy', 'debt:legacy', 'admission:legacy', 'digest:legacy', '/tmp/project',
		1, 'payload:legacy', '.haft/project.yaml', 'opened',
		'legacy', 'must survive refusal', 'expected:legacy',
		'', NULL, '2026-07-15T00:00:00Z'
	)`)
	if err != nil {
		t.Fatalf("seed non-empty legacy table: %v", err)
	}

	err = RunMigrations(database)
	if err == nil {
		t.Fatal("expected non-empty legacy schema to fail closed")
	}
	for _, fragment := range []string{
		"project_profile_projection_debt contains 1 row(s)",
		"manual migration required",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("reconciliation error %q does not contain %q", err, fragment)
		}
	}
	assertMigrationVersionCount(t, database, 35, 1)
	assertMigrationVersionCount(t, database, 36, 0)
	assertTableRowCount(t, database, "project_profile_projection_debt", 1)
	assertSchemaFingerprint(t, database, legacyV34AuthorityProfileFingerprint)
}

func TestAuthorityProfileReconciliationRejectsUnknownLegacyMutation(t *testing.T) {
	t.Parallel()

	database := openLegacyV34Database(t, filepath.Join(t.TempDir(), "legacy-unknown.db"))
	defer database.Close()
	if _, err := database.Exec(`ALTER TABLE authority_presentations ADD COLUMN unexpected_legacy_field TEXT`); err != nil {
		t.Fatalf("mutate legacy schema: %v", err)
	}

	err := RunMigrations(database)
	if err == nil {
		t.Fatal("expected unknown legacy schema to fail closed")
	}
	for _, fragment := range []string{
		"neither canonical nor the exact known legacy-v34 schema",
		"manual migration required",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("reconciliation error %q does not contain %q", err, fragment)
		}
	}
	assertMigrationVersionCount(t, database, 35, 1)
	assertMigrationVersionCount(t, database, 36, 0)
	assertColumnExists(t, database, "authority_presentations", "unexpected_legacy_field")
}

func TestAuthorityProfileReconciliationRejectsDroppedMigrationReviewTrigger(t *testing.T) {
	t.Parallel()

	database := openLegacyV34Database(t, filepath.Join(t.TempDir(), "legacy-missing-review-trigger.db"))
	defer database.Close()
	installMigration35(t, database)
	if _, err := database.Exec("DROP TRIGGER migration_review_admissions_exact_speech_act"); err != nil {
		t.Fatalf("drop migration-review trigger: %v", err)
	}

	err := RunMigrations(database)
	assertMigrationReviewSchemaRefusal(t, database, err)
	assertSchemaFingerprint(t, database, legacyV34AuthorityProfileFingerprint)
}

func TestAuthorityProfileReconciliationRejectsMutatedMigrationReviewTrigger(t *testing.T) {
	t.Parallel()

	database := openLegacyV34Database(t, filepath.Join(t.TempDir(), "legacy-mutated-review-trigger.db"))
	defer database.Close()
	installMigration35(t, database)
	if _, err := database.Exec("DROP TRIGGER migration_review_admissions_exact_speech_act"); err != nil {
		t.Fatalf("drop canonical migration-review trigger: %v", err)
	}
	migration35, ok := findMigration(kernelMigrations, 35, 0)
	if !ok {
		t.Fatal("migration 35 is unavailable")
	}
	canonicalTrigger := migrationStatementContaining(
		t,
		migration35.Statements,
		"CREATE TRIGGER IF NOT EXISTS migration_review_admissions_exact_speech_act",
	)
	mutatedTrigger := strings.Replace(
		canonicalTrigger,
		"does not bind the exact human SpeechAct",
		"does not bind  the exact human SpeechAct",
		1,
	)
	if mutatedTrigger == canonicalTrigger {
		t.Fatal("migration 35 no longer exposes the expected exact-speech-act trigger message")
	}
	if _, err := database.Exec(mutatedTrigger); err != nil {
		t.Fatalf("install mutated migration-review trigger: %v", err)
	}

	err := RunMigrations(database)
	assertMigrationReviewSchemaRefusal(t, database, err)
	assertSchemaFingerprint(t, database, legacyV34AuthorityProfileFingerprint)
}

func TestAuthorityProfileReconciliationRejectsWeakenedMigrationReviewConstraint(t *testing.T) {
	t.Parallel()

	database := openLegacyV34Database(t, filepath.Join(t.TempDir(), "legacy-weakened-review-check.db"))
	defer database.Close()
	installMigration35(t, database)
	if _, err := database.Exec("DROP TABLE migration_review_admissions"); err != nil {
		t.Fatalf("drop migration-review admissions table: %v", err)
	}
	migration35, ok := findMigration(kernelMigrations, 35, 0)
	if !ok {
		t.Fatal("migration 35 is unavailable")
	}
	canonicalCreate := migrationStatementContaining(
		t,
		migration35.Statements,
		"CREATE TABLE IF NOT EXISTS migration_review_admissions",
	)
	weakenedCreate := strings.Replace(
		canonicalCreate,
		"CHECK(partition_audit_status = 'verified')",
		"CHECK(partition_audit_status IN ('verified', 'unchecked'))",
		1,
	)
	if weakenedCreate == canonicalCreate {
		t.Fatal("migration 35 no longer exposes the expected partition-audit status constraint")
	}
	if _, err := database.Exec(weakenedCreate); err != nil {
		t.Fatalf("install weakened migration-review admissions table: %v", err)
	}
	if err := executeStatements(database, migration35.Statements, 0); err != nil {
		t.Fatalf("restore migration-review indexes and triggers around weakened table: %v", err)
	}

	err := RunMigrations(database)
	assertMigrationReviewSchemaRefusal(t, database, err)
	assertSchemaFingerprint(t, database, legacyV34AuthorityProfileFingerprint)
}

func installMigration35(t *testing.T, database *sql.DB) {
	t.Helper()
	migration35, ok := findMigration(kernelMigrations, 35, 0)
	if !ok {
		t.Fatal("migration 35 is unavailable")
	}
	if err := Migrate(database, "schema_version", []Migration{migration35}); err != nil {
		t.Fatalf("install migration 35: %v", err)
	}
}

func migrationStatementContaining(t *testing.T, statements []string, fragment string) string {
	t.Helper()
	for _, statement := range statements {
		if strings.Contains(statement, fragment) {
			return statement
		}
	}
	t.Fatalf("migration statement containing %q is unavailable", fragment)
	return ""
}

func assertMigrationReviewSchemaRefusal(t *testing.T, database *sql.DB, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected non-canonical migration-review schema to fail closed")
	}
	for _, fragment := range []string{
		"migration-review schema reconciliation refused",
		"manual migration required",
	} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("reconciliation error %q does not contain %q", err, fragment)
		}
	}
	assertMigrationVersionCount(t, database, 35, 1)
	assertMigrationVersionCount(t, database, 36, 0)
}

func TestAuthorityProfileReconciliationIsStableAcrossRestart(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "legacy-restart.db")
	database := openLegacyV34Database(t, databasePath)
	insertUnrelatedGraphAndArtifactData(t, database)
	if err := RunMigrations(database); err != nil {
		t.Fatalf("first reconciliation: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close reconciled database: %v", err)
	}

	store, err := NewStore(databasePath)
	if err != nil {
		t.Fatalf("restart reconciled store: %v", err)
	}
	defer store.Close()
	assertMigrationVersionCount(t, store.conn, 36, 1)
	assertCanonicalAuthorityProfileSchema(t, store.conn)
	assertUnrelatedGraphAndArtifactData(t, store.conn)
}

func TestAuthorityProfileReconciliationConvergesAcrossConcurrentConnections(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "legacy-concurrent.db")
	first := openLegacyV34Database(t, databasePath)
	defer first.Close()
	if _, err := first.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("enable WAL for concurrent reconciliation: %v", err)
	}
	installMigration35(t, first)

	dsn, err := sqliteConnectionDSN(databasePath)
	if err != nil {
		t.Fatalf("build second SQLite DSN: %v", err)
	}
	second, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open second SQLite connection: %v", err)
	}
	second.SetMaxOpenConns(1)
	defer second.Close()
	if err := second.Ping(); err != nil {
		t.Fatalf("ping second SQLite connection: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	through36 := migrationsBeforeVersion(kernelMigrations, 37, 0, nil)
	run := func(database *sql.DB) {
		<-start
		results <- Migrate(database, "schema_version", through36)
	}
	go run(first)
	go run(second)
	close(start)
	for index := 0; index < 2; index++ {
		if err := <-results; err != nil {
			t.Fatalf("concurrent reconciliation %d: %v", index+1, err)
		}
	}

	assertMigrationVersionCount(t, first, 36, 1)
	if err := Migrate(first, "schema_version", kernelMigrations); err != nil {
		t.Fatalf("install later migrations after concurrent v36 convergence: %v", err)
	}
	assertCanonicalAuthorityProfileSchema(t, first)
}

func TestCanonicalAuthorityProfileContractTracksSuppliedMigrations(t *testing.T) {
	t.Parallel()

	baseline, err := loadCanonicalAuthorityProfileContract(kernelMigrations)
	if err != nil {
		t.Fatalf("load baseline canonical contract: %v", err)
	}
	modified := cloneMigrations(kernelMigrations)
	for index := range modified {
		if modified[index].Version != 35 {
			continue
		}
		modified[index].Statements = append(
			modified[index].Statements,
			"ALTER TABLE migration_review_speech_acts ADD COLUMN contract_probe TEXT NOT NULL DEFAULT ''",
		)
	}
	changed, err := loadCanonicalAuthorityProfileContract(modified)
	if err != nil {
		t.Fatalf("load changed canonical contract: %v", err)
	}
	baselineColumns := baseline.columns["migration_review_speech_acts"]
	changedColumns := changed.columns["migration_review_speech_acts"]
	if strings.Join(baselineColumns, "\x00") == strings.Join(changedColumns, "\x00") {
		t.Fatalf("canonical contract ignored supplied migration change: %v", changedColumns)
	}
	if changed.migrationReviewFingerprint == baseline.migrationReviewFingerprint {
		t.Fatal("canonical migration-review fingerprint ignored supplied migration change")
	}
	repeated, err := loadCanonicalAuthorityProfileContract(kernelMigrations)
	if err != nil {
		t.Fatalf("reload baseline canonical contract: %v", err)
	}
	if repeated.fingerprint != baseline.fingerprint {
		t.Fatalf("canonical authority/profile fingerprint is unstable: first=%s second=%s", baseline.fingerprint, repeated.fingerprint)
	}
	if repeated.migrationReviewFingerprint != baseline.migrationReviewFingerprint {
		t.Fatalf(
			"canonical migration-review fingerprint is unstable: first=%s second=%s",
			baseline.migrationReviewFingerprint,
			repeated.migrationReviewFingerprint,
		)
	}
}

func cloneMigrations(source []Migration) []Migration {
	cloned := append([]Migration(nil), source...)
	for index := range cloned {
		cloned[index].Statements = append([]string(nil), source[index].Statements...)
	}
	return cloned
}

func TestAuthorityProfileReconciliationRollsBackWhenFinalVerificationFails(t *testing.T) {
	t.Parallel()

	database := openLegacyV34Database(t, filepath.Join(t.TempDir(), "legacy-rollback.db"))
	defer database.Close()
	if _, err := database.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatalf("disable foreign keys for invalid preexisting witness: %v", err)
	}
	_, err := database.Exec(`INSERT INTO affected_files (artifact_id, file_path) VALUES ('missing-artifact', 'missing.go')`)
	if err != nil {
		t.Fatalf("seed preexisting foreign-key violation: %v", err)
	}
	if _, err := database.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("restore foreign-key enforcement: %v", err)
	}

	err = RunMigrations(database)
	if err == nil {
		t.Fatal("expected final foreign-key verification to reject migration 36")
	}
	if !strings.Contains(err.Error(), "verify reconciled schema foreign keys") {
		t.Fatalf("reconciliation error %q does not identify foreign-key verification", err)
	}
	assertMigrationVersionCount(t, database, 36, 0)
	assertSchemaFingerprint(t, database, legacyV34AuthorityProfileFingerprint)
}

func TestAuthorityProfileReconciliationAdmitsWitnessedDecisionSpecSectionRelation(t *testing.T) {
	t.Parallel()

	database := openLegacyV34Database(t, filepath.Join(t.TempDir(), "legacy-spec-relation.db"))
	defer database.Close()
	seedLegacyDecisionSpecSectionRelation(t, database, true)

	through36 := migrationsBeforeVersion(kernelMigrations, 37, 0, nil)
	if err := Migrate(database, "schema_version", through36); err != nil {
		t.Fatalf("migrate witnessed DecisionRecord to SpecSection relation: %v", err)
	}
	assertMigrationVersionCount(t, database, 36, 1)

	var count int
	err := database.QueryRow(`
		SELECT COUNT(*)
		FROM artifact_links
		WHERE source_id = 'dec-spec-relation'
			AND target_id = 'TS.relation.001'
			AND link_type = 'governs'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("load preserved DecisionRecord to SpecSection relation: %v", err)
	}
	if count != 1 {
		t.Fatalf("preserved DecisionRecord to SpecSection relation count = %d, want 1", count)
	}
}

func TestCurrentMigrationsPreserveExactNineWitnessedLegacySpecRelations(
	t *testing.T,
) {
	t.Parallel()

	database := openLegacyV34Database(
		t,
		filepath.Join(t.TempDir(), "personal-brand-shape.db"),
	)
	defer database.Close()
	seedWitnessedLegacyDecisionSpecSectionRelations(t, database, 9)

	if err := RunMigrations(database); err != nil {
		t.Fatalf("migrate exact witnessed legacy relation shape: %v", err)
	}
	var migrationCount int
	var maximumVersion int
	err := database.QueryRow(
		"SELECT COUNT(*), MAX(version) FROM schema_version",
	).Scan(&migrationCount, &maximumVersion)
	if err != nil {
		t.Fatal(err)
	}
	if migrationCount != len(kernelMigrations) ||
		maximumVersion != kernelMigrations[len(kernelMigrations)-1].Version {
		t.Fatalf(
			"migration frontier = count %d max %d, want count %d max %d",
			migrationCount,
			maximumVersion,
			len(kernelMigrations),
			kernelMigrations[len(kernelMigrations)-1].Version,
		)
	}
	var preserved int
	err = database.QueryRow(`
		SELECT COUNT(*)
		FROM artifact_links
		WHERE source_id = 'dec-personal-brand-shape'
			AND link_type = 'governs'`,
	).Scan(&preserved)
	if err != nil {
		t.Fatal(err)
	}
	if preserved != 9 {
		t.Fatalf("preserved witnessed legacy links = %d, want 9", preserved)
	}
	assertOnlyWitnessedLegacyForeignKeyViolations(t, database, 9)
}

func TestAuthorityProfileReconciliationRejectsUnwitnessedDecisionSpecSectionRelation(t *testing.T) {
	t.Parallel()

	database := openLegacyV34Database(t, filepath.Join(t.TempDir(), "legacy-unwitnessed-spec-relation.db"))
	defer database.Close()
	seedLegacyDecisionSpecSectionRelation(t, database, false)

	err := RunMigrations(database)
	if err == nil {
		t.Fatal("expected an unwitnessed DecisionRecord to SpecSection relation to fail")
	}
	if !strings.Contains(err.Error(), "verify reconciled schema foreign keys") {
		t.Fatalf("unwitnessed relation error %q does not identify foreign-key verification", err)
	}
	assertMigrationVersionCount(t, database, 36, 0)
	assertSchemaFingerprint(t, database, legacyV34AuthorityProfileFingerprint)
}

func seedLegacyDecisionSpecSectionRelation(
	t *testing.T,
	database *sql.DB,
	witnessed bool,
) {
	t.Helper()
	structuredData := `{"section_refs":[]}`
	if witnessed {
		structuredData = `{"section_refs":["TS.relation.001"]}`
	}
	_, err := database.Exec(`
		INSERT INTO artifacts (
			id, kind, version, status, context, mode, title, content,
			created_at, updated_at, structured_data
		) VALUES (
			'dec-spec-relation', 'DecisionRecord', 1, 'active', 'test', 'standard',
			'Govern the typed section', 'test fixture',
			'2026-07-15T00:00:00Z', '2026-07-15T00:00:00Z', ?
		)`,
		structuredData,
	)
	if err != nil {
		t.Fatalf("seed DecisionRecord source: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO spec_section_editions (
			project_id, section_id, semantic_hash, section_json, source_kind, carrier_path
		) VALUES (
			'qnt_12345678', 'TS.relation.001', 'sha256:test',
			'{"id":"TS.relation.001"}', 'carrier_import', '.haft/specs/target-system.md'
		)`)
	if err != nil {
		t.Fatalf("seed SpecSection target: %v", err)
	}
	if _, err := database.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatalf("disable foreign keys for legacy polymorphic relation: %v", err)
	}
	_, err = database.Exec(`
		INSERT INTO artifact_links (source_id, target_id, link_type, created_at)
		VALUES (
			'dec-spec-relation', 'TS.relation.001', 'governs', '2026-07-15T00:00:00Z'
		)`)
	if err != nil {
		t.Fatalf("seed legacy polymorphic relation: %v", err)
	}
	if _, err := database.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("restore foreign keys after legacy polymorphic relation: %v", err)
	}
}

func seedWitnessedLegacyDecisionSpecSectionRelations(
	t *testing.T,
	database *sql.DB,
	count int,
) {
	t.Helper()
	sectionRefs := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		sectionRefs = append(
			sectionRefs,
			fmt.Sprintf("TS.personal-brand.%03d", index),
		)
	}
	structuredData, err := json.Marshal(map[string]any{
		"section_refs": sectionRefs,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`
		INSERT INTO artifacts (
			id, kind, version, status, context, mode, title, content,
			created_at, updated_at, structured_data
		) VALUES (
			'dec-personal-brand-shape', 'DecisionRecord', 1, 'active',
			'test', 'standard', 'Preserve exact legacy relations',
			'regression fixture shaped after the witnessed v35 project ledger',
			'2026-07-18T00:00:00Z', '2026-07-18T00:00:00Z', ?
		)`,
		string(structuredData),
	)
	if err != nil {
		t.Fatalf("seed legacy DecisionRecord: %v", err)
	}
	for _, sectionRef := range sectionRefs {
		_, err = database.Exec(`
			INSERT INTO spec_section_editions (
				project_id, section_id, semantic_hash, section_json,
				source_kind, carrier_path
			) VALUES (
				'qnt_b300d2fe', ?, 'sha256:test',
				json_object('id', ?), 'carrier_import',
				'.haft/specs/target-system.md'
			)`,
			sectionRef,
			sectionRef,
		)
		if err != nil {
			t.Fatalf("seed legacy SpecSection %s: %v", sectionRef, err)
		}
	}
	if _, err := database.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatalf("disable foreign keys for witnessed legacy relations: %v", err)
	}
	for _, sectionRef := range sectionRefs {
		_, err = database.Exec(`
			INSERT INTO artifact_links (
				source_id, target_id, link_type, created_at
			) VALUES (
				'dec-personal-brand-shape', ?, 'governs',
				'2026-07-18T00:00:00Z'
			)`,
			sectionRef,
		)
		if err != nil {
			t.Fatalf("seed legacy relation to %s: %v", sectionRef, err)
		}
	}
	if _, err := database.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("restore foreign keys after legacy relation seed: %v", err)
	}
}

func assertOnlyWitnessedLegacyForeignKeyViolations(
	t *testing.T,
	database *sql.DB,
	expected int,
) {
	t.Helper()
	rows, err := database.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	violations, err := loadForeignKeyViolations(rows, nil)
	closeErr := rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if len(violations) != expected {
		t.Fatalf(
			"foreign-key violations = %d, want exactly %d witnessed rows",
			len(violations),
			expected,
		)
	}
	for _, violation := range violations {
		admitted, err := isLegacyDecisionSpecSectionProjection(
			database,
			violation,
		)
		if err != nil {
			t.Fatal(err)
		}
		if !admitted {
			t.Fatalf("unexpected foreign-key violation: %#v", violation)
		}
	}
}

func openLegacyV34Database(t *testing.T, databasePath string) *sql.DB {
	t.Helper()
	dsn, err := sqliteConnectionDSN(databasePath)
	if err != nil {
		t.Fatalf("build SQLite DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	database.SetMaxOpenConns(1)
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping SQLite database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install base schema: %v", err)
	}
	preV34 := migrationsBeforeVersion(kernelMigrations, 34, 0, nil)
	if err := Migrate(database, "schema_version", preV34); err != nil {
		_ = database.Close()
		t.Fatalf("install migrations before v34: %v", err)
	}
	legacyDDL, err := os.ReadFile(filepath.Join("testdata", "legacy_v34_authority_profile.sql"))
	if err != nil {
		_ = database.Close()
		t.Fatalf("read exact legacy-v34 fixture: %v", err)
	}
	if _, err := database.Exec(string(legacyDDL)); err != nil {
		_ = database.Close()
		t.Fatalf("install exact legacy-v34 fixture: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO schema_version (version) VALUES (34)`); err != nil {
		_ = database.Close()
		t.Fatalf("record legacy migration 34: %v", err)
	}
	return database
}

func migrationsBeforeVersion(
	migrations []Migration,
	version int,
	index int,
	accumulator []Migration,
) []Migration {
	if index >= len(migrations) {
		return accumulator
	}
	if migrations[index].Version >= version {
		return migrationsBeforeVersion(migrations, version, index+1, accumulator)
	}
	next := slices.Clone(accumulator)
	next = append(next, migrations[index])
	return migrationsBeforeVersion(migrations, version, index+1, next)
}

func insertUnrelatedGraphAndArtifactData(t *testing.T, database *sql.DB) {
	t.Helper()
	_, err := database.Exec(`INSERT INTO holons (
		id, type, layer, title, content, context_id
	) VALUES (
		'holon:survives-v36', 'EntityOfConcern', 'L1',
		'Unrelated graph witness', 'must survive v36', 'reconciliation-test'
	)`)
	if err != nil {
		t.Fatalf("seed unrelated graph data: %v", err)
	}
	_, err = database.Exec(`INSERT INTO artifacts (
		id, kind, title, content, created_at, updated_at
	) VALUES (
		'artifact:survives-v36', 'Note', 'Unrelated artifact witness',
		'must survive v36', '2026-07-15T00:00:00Z', '2026-07-15T00:00:00Z'
	)`)
	if err != nil {
		t.Fatalf("seed unrelated artifact data: %v", err)
	}
}

func assertUnrelatedGraphAndArtifactData(t *testing.T, database *sql.DB) {
	t.Helper()
	assertTableRowByID(t, database, "holons", "holon:survives-v36")
	assertTableRowByID(t, database, "artifacts", "artifact:survives-v36")
}

func assertTableRowByID(t *testing.T, database *sql.DB, table string, id string) {
	t.Helper()
	query := "SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table) + " WHERE id = ?"
	var count int
	if err := database.QueryRow(query, id).Scan(&count); err != nil {
		t.Fatalf("inspect retained row %s/%s: %v", table, id, err)
	}
	if count != 1 {
		t.Fatalf("retained row %s/%s count = %d, want 1", table, id, count)
	}
}

func assertCanonicalAuthorityProfileSchema(t *testing.T, database *sql.DB) {
	t.Helper()
	versions := statementMigrationVersionsFrom(kernelMigrations, 34)
	contract, err := buildAuthorityProfileSchemaContract(kernelMigrations, versions)
	if err != nil {
		t.Fatalf("load canonical schema contract: %v", err)
	}
	assertSchemaFingerprint(t, database, fullyMigratedAuthorityProfileFingerprint(t))
	actualMigrationReviewFingerprint, err := migrationReviewSchemaFingerprint(database)
	if err != nil {
		t.Fatalf("fingerprint canonical migration-review schema: %v", err)
	}
	if actualMigrationReviewFingerprint != contract.migrationReviewFingerprint {
		t.Fatalf(
			"migration-review fingerprint = %s, want %s",
			actualMigrationReviewFingerprint,
			contract.migrationReviewFingerprint,
		)
	}
	assertSQLiteMasterObjectCount(t, database, "table", reconciledRequiredTables, 9)
	assertSQLiteMasterObjectCount(t, database, "trigger", reconciledRequiredTriggers, 27)
	actualColumns, err := loadRequiredColumns(database, reconciledRequiredTables, 0, map[string][]string{})
	if err != nil {
		t.Fatalf("load reconciled columns: %v", err)
	}
	for table, expectedColumns := range contract.columns {
		actual := actualColumns[table]
		if strings.Join(actual, "\x00") != strings.Join(expectedColumns, "\x00") {
			t.Fatalf("table %s columns = %v, want %v", table, actual, expectedColumns)
		}
	}
	rows, err := database.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("run foreign_key_check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign_key_check returned a violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read foreign_key_check: %v", err)
	}
}

func fullyMigratedAuthorityProfileFingerprint(t *testing.T) string {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open fully migrated schema fixture: %v", err)
	}
	database.SetMaxOpenConns(1)
	defer database.Close()
	if _, err := database.Exec(schema); err != nil {
		t.Fatalf("install base schema in fully migrated fixture: %v", err)
	}
	if err := RunMigrations(database); err != nil {
		t.Fatalf("install all migrations in fully migrated fixture: %v", err)
	}
	fingerprint, err := authorityProfileSchemaFingerprint(database)
	if err != nil {
		t.Fatalf("fingerprint fully migrated schema fixture: %v", err)
	}
	return fingerprint
}

func statementMigrationVersionsFrom(migrations []Migration, minimum int) []int {
	return appendStatementMigrationVersions(migrations, minimum, 0, nil)
}

func appendStatementMigrationVersions(
	migrations []Migration,
	minimum int,
	index int,
	versions []int,
) []int {
	if index >= len(migrations) {
		return versions
	}
	migration := migrations[index]
	if migration.Version < minimum || migration.Apply != nil {
		return appendStatementMigrationVersions(migrations, minimum, index+1, versions)
	}
	next := slices.Clone(versions)
	next = append(next, migration.Version)
	return appendStatementMigrationVersions(migrations, minimum, index+1, next)
}

func assertSQLiteMasterObjectCount(
	t *testing.T,
	database *sql.DB,
	objectType string,
	names []string,
	expected int,
) {
	t.Helper()
	query, args := sqliteMasterCountQuery(objectType, names)
	var count int
	if err := database.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count %s objects: %v", objectType, err)
	}
	if count != expected {
		t.Fatalf("%s object count = %d, want %d", objectType, count, expected)
	}
}

func assertSchemaFingerprint(t *testing.T, database *sql.DB, expected string) {
	t.Helper()
	actual, err := authorityProfileSchemaFingerprint(database)
	if err != nil {
		t.Fatalf("fingerprint authority/profile schema: %v", err)
	}
	if actual != expected {
		t.Fatalf("authority/profile fingerprint = %s, want %s", actual, expected)
	}
}

func assertMigrationVersionCount(t *testing.T, database *sql.DB, version int, expected int) {
	t.Helper()
	var count int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM schema_version WHERE version = ?`,
		version,
	).Scan(&count); err != nil {
		t.Fatalf("inspect migration %d: %v", version, err)
	}
	if count != expected {
		t.Fatalf("migration %d count = %d, want %d", version, count, expected)
	}
}

func assertTableRowCount(t *testing.T, database *sql.DB, table string, expected int) {
	t.Helper()
	query := "SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table)
	var count int
	if err := database.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("count rows in %s: %v", table, err)
	}
	if count != expected {
		t.Fatalf("table %s row count = %d, want %d", table, count, expected)
	}
}

func assertColumnExists(t *testing.T, database *sql.DB, table string, column string) {
	t.Helper()
	columns, err := loadTableColumns(database, table)
	if err != nil {
		t.Fatalf("load %s columns: %v", table, err)
	}
	for _, candidate := range columns {
		if candidate == column {
			return
		}
	}
	t.Fatalf("table %s does not retain column %s after refused reconciliation", table, column)
}
