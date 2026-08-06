package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	artifactstore "github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

const (
	releasedUpgradeArtifactID       = "note-released-upgrade"
	releasedUpgradeTargetArtifactID = "note-released-upgrade-target"
	releasedUpgradeHolonID          = "entity:released-upgrade"
	releasedUpgradeProjectID        = "qnt_v810spec"
	releasedUpgradeSpecSectionID    = "SS.interfaces.memory.001"
	releasedUpgradeSpecSectionHash  = "sha256:0ad86447401f06222910450549499047546767749f2e172750327216ef51c74f"
	releasedUpgradeSpecApprovedBy   = "v8.1-release-fixture"
	releasedUpgradeStructuredData   = `{"claim":"legacy artifact bytes survive","edition":1}`
	releasedUpgradeRecordedAt       = "2026-06-01T10:00:00Z"
)

type releasedUpgradeFixture struct {
	releaseName          string
	schemaVersion        int
	includesEmbeddings   bool
	includesSpecBaseline bool
}

func TestReleasedV6AndV8DatabasesUpgradeAdditivelyToCurrentSchema(
	t *testing.T,
) {
	t.Parallel()

	// The repository's v6.0.0 and v8.1.0 tags share the current base schema
	// and exact kernel-migration prefixes through versions 25 and 29. Freeze
	// those public release frontiers here instead of approximating an old DB by
	// deleting columns from a current one.
	t.Run("v6.0.0_schema_25", func(t *testing.T) {
		assertReleasedDatabaseUpgrade(t, releasedUpgradeFixture{
			releaseName:        "v6.0.0",
			schemaVersion:      25,
			includesEmbeddings: false,
		})
	})
	t.Run("v8.1.0_schema_29", func(t *testing.T) {
		assertReleasedDatabaseUpgrade(t, releasedUpgradeFixture{
			releaseName:          "v8.1.0",
			schemaVersion:        29,
			includesEmbeddings:   true,
			includesSpecBaseline: true,
		})
	})
}

func assertReleasedDatabaseUpgrade(
	t *testing.T,
	fixture releasedUpgradeFixture,
) {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), fixture.releaseName+".db")
	database := openReleasedUpgradeDatabase(
		t,
		databasePath,
		fixture.schemaVersion,
	)
	seedReleasedUpgradeRows(t, database, fixture)
	beforeTables := snapshotUpgradeTables(t, database)
	beforeTriggers := snapshotUpgradeSchemaObjects(t, database, "trigger")

	if err := RunMigrations(database); err != nil {
		_ = database.Close()
		t.Fatalf("upgrade %s database: %v", fixture.releaseName, err)
	}
	assertCurrentMigrationFrontier(t, database)
	assertUpgradeTableRowsPreserved(
		t,
		beforeTables,
		snapshotUpgradeTables(t, database),
		fixture.schemaVersion,
	)
	assertUpgradeTriggersPreserved(
		t,
		beforeTriggers,
		snapshotUpgradeSchemaObjects(t, database, "trigger"),
	)
	assertUpgradeDatabaseHealthy(t, database)
	assertUpgradeCreatedNoRuntimeState(t, database)
	assertReleasedUpgradeRowsReadable(
		t,
		database,
		fixture,
	)
	if err := database.Close(); err != nil {
		t.Fatalf("close upgraded %s database: %v", fixture.releaseName, err)
	}

	store, err := NewStore(databasePath)
	if err != nil {
		t.Fatalf("reopen upgraded %s database: %v", fixture.releaseName, err)
	}
	defer store.Close()
	if err := RunMigrations(store.conn); err != nil {
		t.Fatalf("retry current migrations for %s database: %v", fixture.releaseName, err)
	}
	assertCurrentMigrationFrontier(t, store.conn)
	assertUpgradeTableRowsPreserved(
		t,
		beforeTables,
		snapshotUpgradeTables(t, store.conn),
		fixture.schemaVersion,
	)
	assertUpgradeDatabaseHealthy(t, store.conn)
	assertUpgradeCreatedNoRuntimeState(t, store.conn)
	assertReleasedUpgradeRowsReadable(
		t,
		store.conn,
		fixture,
	)
}

func openReleasedUpgradeDatabase(
	t *testing.T,
	databasePath string,
	schemaVersion int,
) *sql.DB {
	t.Helper()
	dsn, err := sqliteConnectionDSN(databasePath)
	if err != nil {
		t.Fatalf("build released-upgrade SQLite DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open released-upgrade SQLite database: %v", err)
	}
	database.SetMaxOpenConns(1)
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping released-upgrade SQLite database: %v", err)
	}
	if _, err := database.Exec(schema); err != nil {
		_ = database.Close()
		t.Fatalf("install released base schema: %v", err)
	}
	releasedMigrations := migrationsBeforeVersion(
		kernelMigrations,
		schemaVersion+1,
		0,
		nil,
	)
	if err := Migrate(database, "schema_version", releasedMigrations); err != nil {
		_ = database.Close()
		t.Fatalf("install released schema %d: %v", schemaVersion, err)
	}
	assertExactMigrationFrontier(t, database, schemaVersion, len(releasedMigrations))
	return database
}

func seedReleasedUpgradeRows(
	t *testing.T,
	database *sql.DB,
	fixture releasedUpgradeFixture,
) {
	t.Helper()
	_, err := database.Exec(`INSERT INTO holons (
		id, type, kind, layer, title, content, context_id,
		scope, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		releasedUpgradeHolonID,
		"EntityOfConcern",
		"U.System",
		"L1",
		"Released project object",
		"legacy holon remains readable",
		"released-upgrade",
		"project",
		releasedUpgradeRecordedAt,
		releasedUpgradeRecordedAt,
	)
	if err != nil {
		t.Fatalf("seed released holon: %v", err)
	}
	_, err = database.Exec(`INSERT INTO artifacts (
		id, kind, version, status, context, mode, title, content,
		valid_until, created_at, updated_at, search_keywords,
		structured_data
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		releasedUpgradeArtifactID,
		"Note",
		1,
		"active",
		"released-upgrade",
		"standard",
		"Released artifact",
		"legacy artifact body remains readable",
		"",
		releasedUpgradeRecordedAt,
		releasedUpgradeRecordedAt,
		"legacy upgrade compatibility",
		releasedUpgradeStructuredData,
	)
	if err != nil {
		t.Fatalf("seed released artifact: %v", err)
	}
	_, err = database.Exec(`INSERT INTO artifacts (
		id, kind, version, status, context, mode, title, content,
		valid_until, created_at, updated_at, search_keywords,
		structured_data
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		releasedUpgradeTargetArtifactID,
		"Note",
		1,
		"active",
		"released-upgrade",
		"standard",
		"Released link target",
		"legacy target body remains readable",
		"",
		releasedUpgradeRecordedAt,
		releasedUpgradeRecordedAt,
		"legacy upgrade target",
		`{"claim":"legacy link target survives"}`,
	)
	if err != nil {
		t.Fatalf("seed released target artifact: %v", err)
	}
	_, err = database.Exec(`INSERT INTO artifact_links (
		source_id, target_id, link_type, created_at
	) VALUES (?, ?, ?, ?)`,
		releasedUpgradeArtifactID,
		releasedUpgradeTargetArtifactID,
		"supports",
		releasedUpgradeRecordedAt,
	)
	if err != nil {
		t.Fatalf("seed released artifact link: %v", err)
	}
	if fixture.includesSpecBaseline {
		_, err = database.Exec(`INSERT INTO spec_section_baselines (
			project_id, section_id, hash, captured_at, approved_by
		) VALUES (?, ?, ?, ?, ?)`,
			releasedUpgradeProjectID,
			releasedUpgradeSpecSectionID,
			releasedUpgradeSpecSectionHash,
			releasedUpgradeRecordedAt,
			releasedUpgradeSpecApprovedBy,
		)
		if err != nil {
			t.Fatalf("seed released spec section baseline: %v", err)
		}
	}
	if !fixture.includesEmbeddings {
		return
	}
	_, err = database.Exec(`INSERT INTO artifact_embeddings (
		artifact_id, provider, model, dim, content_hash, vector,
		updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		releasedUpgradeArtifactID,
		"released-fixture",
		"released-fixture-model",
		2,
		"released-content-hash",
		[]byte{0x00, 0x7f, 0x80, 0xff, 0x10, 0x20, 0x30, 0x40},
		releasedUpgradeRecordedAt,
	)
	if err != nil {
		t.Fatalf("seed released embedding bytes: %v", err)
	}
}

func assertReleasedUpgradeRowsReadable(
	t *testing.T,
	database *sql.DB,
	fixture releasedUpgradeFixture,
) {
	t.Helper()
	artifactReader := artifactstore.NewStore(database)
	artifact, err := artifactReader.Get(
		context.Background(),
		releasedUpgradeArtifactID,
	)
	if err != nil {
		t.Fatalf("read upgraded legacy artifact: %v", err)
	}
	if artifact.Meta.Title != "Released artifact" ||
		artifact.Body != "legacy artifact body remains readable" ||
		artifact.StructuredData != releasedUpgradeStructuredData {
		t.Fatalf("upgraded legacy artifact changed: %#v", artifact)
	}
	if len(artifact.Meta.Links) != 1 ||
		artifact.Meta.Links[0].Ref != releasedUpgradeTargetArtifactID ||
		artifact.Meta.Links[0].Type != "supports" {
		t.Fatalf("upgraded legacy artifact links changed: %#v", artifact.Meta.Links)
	}

	var holonContent string
	err = database.QueryRow(
		"SELECT content FROM holons WHERE id = ?",
		releasedUpgradeHolonID,
	).Scan(&holonContent)
	if err != nil {
		t.Fatalf("read upgraded legacy holon: %v", err)
	}
	if holonContent != "legacy holon remains readable" {
		t.Fatalf("upgraded legacy holon content = %q", holonContent)
	}

	var legacyMode string
	err = database.QueryRow(
		"SELECT mode FROM legacy_semantic_write_policy WHERE singleton = 1",
	).Scan(&legacyMode)
	if err != nil {
		t.Fatalf("read post-upgrade legacy write policy: %v", err)
	}
	if legacyMode != "legacy_compatible" {
		t.Fatalf("post-upgrade legacy write policy = %q", legacyMode)
	}

	if fixture.includesSpecBaseline {
		baselineStore := specflow.NewSQLiteBaselineStore(database)
		baseline, baselineErr := baselineStore.Get(
			releasedUpgradeProjectID,
			releasedUpgradeSpecSectionID,
		)
		if baselineErr != nil {
			t.Fatalf("read upgraded spec section baseline: %v", baselineErr)
		}
		if baseline.ProjectID != releasedUpgradeProjectID ||
			baseline.SectionID != releasedUpgradeSpecSectionID ||
			baseline.Hash != releasedUpgradeSpecSectionHash ||
			baseline.ApprovedBy != releasedUpgradeSpecApprovedBy ||
			baseline.CapturedAt.UTC().Format(time.RFC3339) !=
				releasedUpgradeRecordedAt {
			t.Fatalf(
				"upgraded spec section baseline changed: %#v",
				baseline,
			)
		}
	}

	if !fixture.includesEmbeddings {
		return
	}
	var vector []byte
	err = database.QueryRow(`SELECT vector FROM artifact_embeddings
		WHERE artifact_id = ? AND provider = ? AND model = ? AND dim = ?`,
		releasedUpgradeArtifactID,
		"released-fixture",
		"released-fixture-model",
		2,
	).Scan(&vector)
	if err != nil {
		t.Fatalf("read upgraded legacy embedding bytes: %v", err)
	}
	expected := []byte{0x00, 0x7f, 0x80, 0xff, 0x10, 0x20, 0x30, 0x40}
	if !bytes.Equal(vector, expected) {
		t.Fatalf("upgraded legacy embedding bytes = %x, want %x", vector, expected)
	}
}

func assertCurrentMigrationFrontier(t *testing.T, database *sql.DB) {
	t.Helper()
	maximum := kernelMigrations[len(kernelMigrations)-1].Version
	assertExactMigrationFrontier(t, database, maximum, len(kernelMigrations))
}

func assertExactMigrationFrontier(
	t *testing.T,
	database *sql.DB,
	maximum int,
	count int,
) {
	t.Helper()
	var observedCount int
	var observedMaximum int
	err := database.QueryRow(
		"SELECT COUNT(*), COALESCE(MAX(version), 0) FROM schema_version",
	).Scan(&observedCount, &observedMaximum)
	if err != nil {
		t.Fatalf("read migration frontier: %v", err)
	}
	if observedCount != count || observedMaximum != maximum {
		t.Fatalf(
			"migration frontier = count %d max %d, want count %d max %d",
			observedCount,
			observedMaximum,
			count,
			maximum,
		)
	}
}

func TestSchema50DatabaseCopyUpgradesToCurrentSchemaWithoutInventingState(
	t *testing.T,
) {
	t.Parallel()

	temporaryDirectory := t.TempDir()
	sourcePath := filepath.Join(temporaryDirectory, "schema50-source.db")
	copyPath := filepath.Join(temporaryDirectory, "schema50-upgrade-copy.db")
	source := openSchema50UpgradeFixture(t, sourcePath)
	beforeTables := snapshotUpgradeTables(t, source)
	beforeTriggers := snapshotUpgradeSchemaObjects(t, source, "trigger")
	assertUpgradeDatabaseHealthy(t, source)
	assertUpgradeCreatedNoRuntimeState(t, source)
	assertSealedLegacyProfileHistoryReadable(t, source)
	if err := source.Close(); err != nil {
		t.Fatalf("close schema-50 source fixture: %v", err)
	}

	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read schema-50 source fixture: %v", err)
	}
	sourceDigest := sha256.Sum256(sourceBytes)
	if err := os.WriteFile(copyPath, sourceBytes, 0o600); err != nil {
		t.Fatalf("copy schema-50 fixture: %v", err)
	}

	upgradeCopy := openExistingUpgradeDatabase(t, copyPath)
	if err := RunMigrations(upgradeCopy); err != nil {
		_ = upgradeCopy.Close()
		t.Fatalf("upgrade schema-50 copy through current schema: %v", err)
	}
	assertSchema50UpgradeResult(
		t,
		upgradeCopy,
		beforeTables,
		beforeTriggers,
	)
	if err := upgradeCopy.Close(); err != nil {
		t.Fatalf("close upgraded schema-50 copy: %v", err)
	}

	reopened := openExistingUpgradeDatabase(t, copyPath)
	defer reopened.Close()
	if err := RunMigrations(reopened); err != nil {
		t.Fatalf("repeat current-schema migration after physical reopen: %v", err)
	}
	assertSchema50UpgradeResult(
		t,
		reopened,
		beforeTables,
		beforeTriggers,
	)
	sourceBytesAfter, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("reread schema-50 source fixture: %v", err)
	}
	if sha256.Sum256(sourceBytesAfter) != sourceDigest {
		t.Fatal("upgrading the physical copy changed the schema-50 source fixture")
	}
}

func TestSchema50UpgradeRollsBackV51WhenVersionRecordingFails(
	t *testing.T,
) {
	t.Parallel()

	database := openSchema50UpgradeFixture(
		t,
		filepath.Join(t.TempDir(), "schema50-rollback.db"),
	)
	defer database.Close()
	_, err := database.Exec(`CREATE TRIGGER reject_schema_51_version
		BEFORE INSERT ON schema_version
		WHEN NEW.version = 51
		BEGIN
			SELECT RAISE(ABORT, 'injected schema-51 version failure');
		END`)
	if err != nil {
		t.Fatalf("install schema-51 rollback fault: %v", err)
	}
	beforeTables := snapshotUpgradeTables(t, database)
	beforeTriggers := snapshotUpgradeSchemaObjects(t, database, "trigger")
	beforeViews := snapshotUpgradeSchemaObjects(t, database, "view")

	err = RunMigrations(database)
	if err == nil || !strings.Contains(err.Error(), "injected schema-51 version failure") {
		t.Fatalf("schema-51 rollback fault error = %v", err)
	}
	assertExactUpgradeSnapshots(
		t,
		"table rows after schema-51 rollback",
		beforeTables,
		snapshotUpgradeTables(t, database),
	)
	assertExactUpgradeSnapshots(
		t,
		"triggers after schema-51 rollback",
		beforeTriggers,
		snapshotUpgradeSchemaObjects(t, database, "trigger"),
	)
	assertExactUpgradeSnapshots(
		t,
		"views after schema-51 rollback",
		beforeViews,
		snapshotUpgradeSchemaObjects(t, database, "view"),
	)
	assertExactMigrationFrontier(t, database, 50, 50)
	assertUpgradeDatabaseHealthy(t, database)
	assertUpgradeCreatedNoRuntimeState(t, database)

	if _, err := database.Exec("DROP TRIGGER reject_schema_51_version"); err != nil {
		t.Fatalf("remove schema-51 rollback fault: %v", err)
	}
	if err := RunMigrations(database); err != nil {
		t.Fatalf("retry schema-50 upgrade after rollback: %v", err)
	}
	assertCurrentMigrationFrontier(t, database)
	assertUpgradeDatabaseHealthy(t, database)
	assertUpgradeCreatedNoRuntimeState(t, database)
	assertSealedLegacyProfileHistoryReadable(t, database)
}

func openSchema50UpgradeFixture(t *testing.T, path string) *sql.DB {
	t.Helper()
	database := openReleasedUpgradeDatabase(t, path, 43)
	seedHistoricalV1ProfileRevision44(t, database)
	throughSchema50 := migrationsBeforeVersion(kernelMigrations, 51, 0, nil)
	if err := Migrate(database, "schema_version", throughSchema50); err != nil {
		_ = database.Close()
		t.Fatalf("build schema-50 fixture: %v", err)
	}
	assertExactMigrationFrontier(t, database, 50, 50)
	seedReleasedUpgradeRows(t, database, releasedUpgradeFixture{
		includesEmbeddings:   true,
		includesSpecBaseline: true,
	})
	return database
}

func openExistingUpgradeDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
	dsn, err := sqliteConnectionDSN(path)
	if err != nil {
		t.Fatalf("build upgrade-copy SQLite DSN: %v", err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open upgrade-copy SQLite database: %v", err)
	}
	database.SetMaxOpenConns(1)
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping upgrade-copy SQLite database: %v", err)
	}
	return database
}

func assertSchema50UpgradeResult(
	t *testing.T,
	database *sql.DB,
	beforeTables map[string][]string,
	beforeTriggers map[string][]string,
) {
	t.Helper()
	assertCurrentMigrationFrontier(t, database)
	assertUpgradeTableRowsPreserved(
		t,
		beforeTables,
		snapshotUpgradeTables(t, database),
		50,
	)
	assertUpgradeTriggersPreserved(
		t,
		beforeTriggers,
		snapshotUpgradeSchemaObjects(t, database, "trigger"),
	)
	assertUpgradeDatabaseHealthy(t, database)
	assertUpgradeCreatedNoRuntimeState(t, database)
	assertSealedLegacyProfileHistoryReadable(t, database)
	assertReleasedUpgradeRowsReadable(t, database, releasedUpgradeFixture{
		includesEmbeddings:   true,
		includesSpecBaseline: true,
	})
	for _, table := range profileAuthorityUnionTables51 {
		assertUpgradeTableRowCount(t, database, table, 0)
	}
	for _, table := range []string{
		typedMemoryIdentityReconciliationsTable52,
		typedMemoryIdentityParticipantsTable52,
		typedMemoryIdentityRedirectsTable52,
		typedMemoryIdentityClosuresTable52,
	} {
		assertUpgradeTableRowCount(t, database, table, 0)
	}
	for _, table := range profileAdmissionV2WriteTables51 {
		assertSQLiteObjectExists(t, database, "trigger", table+"_v51_writes_sealed")
	}
}

func snapshotUpgradeTables(
	t *testing.T,
	database *sql.DB,
) map[string][]string {
	t.Helper()
	names := loadUpgradeSchemaObjectNames(t, database, "table")
	snapshot := make(map[string][]string, len(names))
	for _, name := range names {
		snapshot[name] = snapshotUpgradeTableRows(t, database, name)
	}
	return snapshot
}

func snapshotUpgradeSchemaObjects(
	t *testing.T,
	database *sql.DB,
	kind string,
) map[string][]string {
	t.Helper()
	rows, err := database.Query(`SELECT name, sql
		FROM sqlite_master
		WHERE type = ? AND name NOT LIKE 'sqlite_%'
		ORDER BY name`, kind)
	if err != nil {
		t.Fatalf("snapshot SQLite %s objects: %v", kind, err)
	}
	defer rows.Close()
	snapshot := map[string][]string{}
	for rows.Next() {
		var name string
		var statement string
		if err := rows.Scan(&name, &statement); err != nil {
			t.Fatalf("read SQLite %s object: %v", kind, err)
		}
		snapshot[name] = []string{statement}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate SQLite %s objects: %v", kind, err)
	}
	return snapshot
}

func loadUpgradeSchemaObjectNames(
	t *testing.T,
	database *sql.DB,
	kind string,
) []string {
	t.Helper()
	rows, err := database.Query(`SELECT name
		FROM sqlite_master
		WHERE type = ? AND name NOT LIKE 'sqlite_%'
		ORDER BY name`, kind)
	if err != nil {
		t.Fatalf("list SQLite %s objects: %v", kind, err)
	}
	defer rows.Close()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("read SQLite %s name: %v", kind, err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate SQLite %s names: %v", kind, err)
	}
	return names
}

func snapshotUpgradeTableRows(
	t *testing.T,
	database *sql.DB,
	table string,
) []string {
	t.Helper()
	rows, err := database.Query(
		"SELECT * FROM " + quoteSQLiteIdentifier(table),
	)
	if err != nil {
		t.Fatalf("snapshot table %s: %v", table, err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatalf("read table %s columns: %v", table, err)
	}
	fingerprints := []string{}
	for rows.Next() {
		values := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for index := range values {
			destinations[index] = &values[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			t.Fatalf("read table %s row: %v", table, err)
		}
		fingerprints = append(
			fingerprints,
			upgradeRowFingerprint(values),
		)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table %s rows: %v", table, err)
	}
	sort.Strings(fingerprints)
	return fingerprints
}

func upgradeRowFingerprint(values []any) string {
	encoded := make([]string, len(values))
	for index, value := range values {
		encoded[index] = upgradeValueFingerprint(value)
	}
	return strings.Join(encoded, "\x1f")
}

func upgradeValueFingerprint(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case []byte:
		return "blob:" + hex.EncodeToString(typed)
	case string:
		return "text:" + hex.EncodeToString([]byte(typed))
	default:
		return fmt.Sprintf("%T:%v", value, value)
	}
}

func assertUpgradeTableRowsPreserved(
	t *testing.T,
	before map[string][]string,
	after map[string][]string,
	sourceVersion int,
) {
	t.Helper()
	for table, beforeRows := range before {
		afterRows, ok := after[table]
		if !ok {
			t.Fatalf("pre-existing table %s disappeared during upgrade", table)
		}
		if table == "schema_version" {
			assertUpgradeRowsRemainSubset(t, table, beforeRows, afterRows)
			expected := len(beforeRows) + pendingUpgradeMigrationCount(sourceVersion)
			if len(afterRows) != expected {
				t.Fatalf(
					"schema_version rows = %d, want %d after source schema %d",
					len(afterRows),
					expected,
					sourceVersion,
				)
			}
			continue
		}
		if !slices.Equal(beforeRows, afterRows) {
			t.Fatalf(
				"pre-existing table %s changed rows: before=%v after=%v",
				table,
				beforeRows,
				afterRows,
			)
		}
	}
}

func assertUpgradeRowsRemainSubset(
	t *testing.T,
	table string,
	before []string,
	after []string,
) {
	t.Helper()
	remaining := append([]string(nil), after...)
	for _, row := range before {
		index := slices.Index(remaining, row)
		if index < 0 {
			t.Fatalf("pre-existing table %s lost row %q", table, row)
		}
		remaining = slices.Delete(remaining, index, index+1)
	}
}

func pendingUpgradeMigrationCount(sourceVersion int) int {
	count := 0
	for _, migration := range kernelMigrations {
		if migration.Version > sourceVersion {
			count++
		}
	}
	return count
}

func assertUpgradeTriggersPreserved(
	t *testing.T,
	before map[string][]string,
	after map[string][]string,
) {
	t.Helper()
	for name, beforeSQL := range before {
		afterSQL, ok := after[name]
		if name == "typed_memory_event_writer_generations_v46_exact_boundary" {
			if ok {
				t.Fatalf("superseded v46 writer-boundary trigger survived v54")
			}
			if _, exists := after["typed_memory_event_writer_generations_v54_exact_boundary"]; !exists {
				t.Fatal("v54 writer-boundary trigger is missing after upgrade")
			}
			continue
		}
		if v56SupersededTypeEnvAuthorityTrigger(name) {
			if ok {
				t.Fatalf("superseded TypeEnv authority trigger %s survived v56", name)
			}
			continue
		}
		if !ok {
			t.Fatalf("pre-existing trigger %s disappeared during upgrade", name)
		}
		if name == "typed_memory_graph_commits_exact_closure" {
			if slices.Equal(beforeSQL, afterSQL) ||
				!strings.Contains(afterSQL[0], typedMemoryIdentityReconciliationsTable52) ||
				!strings.Contains(afterSQL[0], "generation.writer_generation = 54") ||
				!strings.Contains(afterSQL[0], "kind_classification_evaluation_count") ||
				!strings.Contains(afterSQL[0], hostRoutedAuthorityMode56) ||
				!strings.Contains(afterSQL[0], typeEnvCompatibleAuthorityGeneration57) {
				t.Fatalf("current graph-commit trigger lacks its v52, v54, or v57 branches: %v", afterSQL)
			}
			continue
		}
		if name == "typed_memory_type_env_activations_v47_exact_effect" ||
			name == "typed_memory_graph_commits_v47_activation_effect" {
			if slices.Equal(beforeSQL, afterSQL) ||
				!strings.Contains(afterSQL[0], hostRoutedAuthorityMode56) ||
				!strings.Contains(afterSQL[0], typeEnvCompatibleAuthorityGeneration57) {
				t.Fatalf("v57 TypeEnv activation trigger lacks exact current authority classes: %v", afterSQL)
			}
			continue
		}
		if name == "typed_memory_commit_materialization_closures_v46_exact_footprint" {
			if slices.Equal(beforeSQL, afterSQL) ||
				!strings.Contains(afterSQL[0], typedMemoryRelationalAssertionsTable53) ||
				!strings.Contains(afterSQL[0], typedMemoryKindClassificationSourceBlobsTable54) {
				t.Fatalf("v54 materialization closure lacks assertion or classification lanes: %v", afterSQL)
			}
			continue
		}
		if name == "typed_memory_commit_materialization_closures_v46_basis_kind" {
			if slices.Equal(beforeSQL, afterSQL) ||
				!strings.Contains(afterSQL[0], "context_slice_classification") {
				t.Fatalf("v54 materialization closure lacks the classification basis: %v", afterSQL)
			}
			continue
		}
		if v54RebuiltStableTrigger(name) {
			beforeNormalized := strings.Join(strings.Fields(beforeSQL[0]), " ")
			afterNormalized := strings.Join(strings.Fields(afterSQL[0]), " ")
			if beforeNormalized != afterNormalized {
				t.Fatalf(
					"v54 rebuilt trigger %s with a semantic SQL change: before=%v after=%v",
					name,
					beforeSQL,
					afterSQL,
				)
			}
			continue
		}
		if !slices.Equal(beforeSQL, afterSQL) {
			t.Fatalf(
				"pre-existing trigger %s changed unexpectedly: before=%v after=%v",
				name,
				beforeSQL,
				afterSQL,
			)
		}
	}
}

func v56SupersededTypeEnvAuthorityTrigger(name string) bool {
	superseded := map[string]struct{}{
		"project_typeenv_head_selection_authority_resolutions_v47_no_insert":   {},
		"project_typeenv_head_selection_authority_resolutions_v47_no_update":   {},
		"project_typeenv_head_selection_authority_resolutions_v47_no_delete":   {},
		"project_typeenv_head_selection_authority_resolutions_v47_exact_basis": {},
		"project_typeenv_head_selection_authority_uses_v47_no_insert":          {},
		"project_typeenv_head_selection_authority_uses_v47_no_update":          {},
		"project_typeenv_head_selection_authority_uses_v47_no_delete":          {},
		"project_typeenv_head_selection_authority_uses_v47_exact_source":       {},
	}
	_, ok := superseded[name]
	return ok
}

func v54RebuiltStableTrigger(name string) bool {
	rebuilt := map[string]struct{}{
		"typed_memory_event_writer_generations_v46_no_update":         {},
		"typed_memory_event_writer_generations_v46_no_delete":         {},
		"typed_memory_event_writer_generations_v46_open_event":        {},
		"typed_memory_event_admission_bases_v46_no_update":            {},
		"typed_memory_event_admission_bases_v46_no_delete":            {},
		"typed_memory_event_admission_bases_v46_open_event":           {},
		"typed_memory_event_admission_bases_v46_exact_event":          {},
		"typed_memory_commit_materialization_closures_v46_no_update":  {},
		"typed_memory_commit_materialization_closures_v46_no_delete":  {},
		"typed_memory_commit_materialization_closures_v46_open_event": {},
	}
	_, ok := rebuilt[name]
	return ok
}

func assertExactUpgradeSnapshots(
	t *testing.T,
	label string,
	expected map[string][]string,
	actual map[string][]string,
) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("%s object count = %d, want %d", label, len(actual), len(expected))
	}
	for name, expectedValues := range expected {
		actualValues, ok := actual[name]
		if !ok || !slices.Equal(expectedValues, actualValues) {
			t.Fatalf(
				"%s changed object %s: expected=%v actual=%v",
				label,
				name,
				expectedValues,
				actualValues,
			)
		}
	}
}

func assertUpgradeDatabaseHealthy(t *testing.T, database *sql.DB) {
	t.Helper()
	var foreignKeys int
	if err := database.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign-key enforcement: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign-key enforcement = %d, want 1", foreignKeys)
	}
	rows, err := database.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatalf("run upgrade foreign_key_check: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("upgrade foreign_key_check returned a violation")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read upgrade foreign_key_check: %v", err)
	}
	var integrity string
	if err := database.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatalf("run upgrade integrity_check: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("upgrade integrity_check = %q, want ok", integrity)
	}
}

func assertUpgradeCreatedNoRuntimeState(t *testing.T, database *sql.DB) {
	t.Helper()
	for _, table := range []string{
		"typed_memory_graph_heads",
		"typed_memory_graph_events",
		"typed_memory_graph_commits",
		"typed_memory_type_env_snapshots",
		"typed_memory_type_env_coordinates",
		"project_typeenv_artifacts",
		"project_typeenv_composite_verifications",
		"project_typeenv_stages",
		"project_typeenv_executable_snapshots",
		"project_typeenv_heads",
		"project_typeenv_head_history",
		"typed_memory_type_env_activations",
		"project_typeenv_head_selection_requests",
		"project_typeenv_head_selection_receipts",
		"project_typeenv_head_selection_closures",
		"legacy_import_runs",
		"legacy_semantic_imports",
		typedMemoryIdentityReconciliationsTable52,
	} {
		if !upgradeTableExists(t, database, table) {
			continue
		}
		assertUpgradeTableRowCount(t, database, table, 0)
	}
}

func upgradeTableExists(t *testing.T, database *sql.DB, table string) bool {
	t.Helper()
	var count int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count)
	if err != nil {
		t.Fatalf("inspect upgrade table %s: %v", table, err)
	}
	return count == 1
}

func assertUpgradeTableRowCount(
	t *testing.T,
	database *sql.DB,
	table string,
	expected int,
) {
	t.Helper()
	var actual int
	err := database.QueryRow(
		"SELECT COUNT(*) FROM " + quoteSQLiteIdentifier(table),
	).Scan(&actual)
	if err != nil {
		t.Fatalf("count upgrade table %s: %v", table, err)
	}
	if actual != expected {
		t.Fatalf("upgrade table %s rows = %d, want %d", table, actual, expected)
	}
}

func assertSealedLegacyProfileHistoryReadable(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	var generation string
	var revision int
	var admissionID string
	err := database.QueryRow(`SELECT
		storage_generation, ledger_revision, admission_id
		FROM current_project_profiles
		WHERE project_root = '/tmp/project'`).Scan(
		&generation,
		&revision,
		&admissionID,
	)
	if err != nil {
		t.Fatalf("read sealed legacy profile history: %v", err)
	}
	if generation != "v1" || revision != 1 || admissionID != "admission:test" {
		t.Fatalf(
			"sealed legacy profile history = %q revision %d admission %q",
			generation,
			revision,
			admissionID,
		)
	}
	_, err = database.Exec(`INSERT INTO project_profile_revisions (project_root)
		VALUES ('/tmp/forbidden-legacy-write')`)
	if err == nil || !strings.Contains(err.Error(), "legacy profile writes are sealed") {
		t.Fatalf("sealed legacy profile insert error = %v", err)
	}
	for _, table := range legacyProfileWriteTables44 {
		assertSQLiteObjectExists(t, database, "trigger", table+"_v44_writes_sealed")
	}
}

func TestReportedLegacyV36FailureRetriesAfterUnrelatedViolationIsRemoved(
	t *testing.T,
) {
	t.Parallel()

	database := openLegacyV34Database(
		t,
		filepath.Join(t.TempDir(), "reported-v36-retry.db"),
	)
	defer database.Close()
	seedWitnessedLegacyDecisionSpecSectionRelations(t, database, 9)
	if _, err := database.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatalf("disable foreign keys for retry blocker: %v", err)
	}
	_, err := database.Exec(`INSERT INTO affected_files (
		artifact_id, file_path
	) VALUES ('unrelated-missing-artifact', 'unrelated-missing.go')`)
	if _, restoreErr := database.Exec("PRAGMA foreign_keys=ON"); restoreErr != nil {
		t.Fatalf("restore foreign keys after retry blocker: %v", restoreErr)
	}
	if err != nil {
		t.Fatalf("seed unrelated retry blocker: %v", err)
	}

	firstErr := RunMigrations(database)
	if firstErr == nil {
		t.Fatal("first migration unexpectedly accepted unrelated FK violation")
	}
	assertMigrationVersionCount(t, database, 36, 0)
	assertWitnessedLegacyLinkCount(t, database, 9)
	if _, err := database.Exec(`DELETE FROM affected_files
		WHERE artifact_id = 'unrelated-missing-artifact'
			AND file_path = 'unrelated-missing.go'`); err != nil {
		t.Fatalf("remove exact unrelated retry blocker: %v", err)
	}

	if err := RunMigrations(database); err != nil {
		t.Fatalf("retry migration after exact repair: %v", err)
	}
	assertCurrentMigrationFrontier(t, database)
	assertWitnessedLegacyLinkCount(t, database, 9)
	assertOnlyWitnessedLegacyForeignKeyViolations(t, database, 9)
}

func assertWitnessedLegacyLinkCount(
	t *testing.T,
	database *sql.DB,
	expected int,
) {
	t.Helper()
	var count int
	err := database.QueryRow(`SELECT COUNT(*)
		FROM artifact_links
		WHERE source_id = 'dec-personal-brand-shape'
			AND link_type = 'governs'`).Scan(&count)
	if err != nil {
		t.Fatalf("count witnessed legacy links: %v", err)
	}
	if count != expected {
		t.Fatalf("witnessed legacy link count = %d, want %d", count, expected)
	}
}
