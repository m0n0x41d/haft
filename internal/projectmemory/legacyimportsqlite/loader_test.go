package legacyimportsqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
	_ "modernc.org/sqlite"
)

func TestCoreSnapshotDryRunReadsWithoutWritingAndDoesNotGuessRelations(
	t *testing.T,
) {
	database := openLegacyDatabase(t, "read-only")
	installCoreLegacySchema(t, database)
	insertCoreLegacyRows(t, database, []string{
		`INSERT INTO artifacts (id, title, content) VALUES ('shared-1', 'artifact title', 'artifact body')`,
		`INSERT INTO holons (id, title, content) VALUES ('shared-1', 'holon title', 'different historical body')`,
		`INSERT INTO artifact_links (source_id, target_id, link_type, created_at) VALUES ('shared-1', 'other-1', 'governs', '2026-07-16T00:00:00Z')`,
		`INSERT INTO relations (source_id, target_id, relation_type, congruence_level) VALUES ('shared-1', 'other-1', 'dependsOnProjected', 3)`,
	})
	before := totalChanges(t, database)

	loader, err := NewCoreSnapshotLoader(database)
	if err != nil {
		t.Fatalf("NewCoreSnapshotLoader() error = %v", err)
	}
	projectID := mustProjectID(t)
	report, err := loader.DryRun(context.Background(), projectID)
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}
	after := totalChanges(t, database)

	if after != before {
		t.Fatalf("DryRun() total changes = %d, want unchanged %d", after, before)
	}
	if report.ProjectID() != projectID {
		t.Fatalf(
			"DryRun() project = %q, want %q",
			report.ProjectID().String(),
			projectID.String(),
		)
	}
	if got := report.Summary().CarrierOnly(); got != 1 {
		t.Fatalf("carrier-only count = %d, want 1 coalesced object", got)
	}
	if got := report.Summary().LegacyUnbound(); got != 2 {
		t.Fatalf("legacy-unbound count = %d, want 2", got)
	}
	if got := report.Summary().Unresolved(); got != 0 {
		t.Fatalf("unresolved count = %d, want 0", got)
	}
	if got := report.CarrierCatalog().Len(); got != 4 {
		t.Fatalf("carrier count = %d, want 4 exact rows", got)
	}
	assertSharedObjectHasBothHistoricalCarriers(t, report)
	assertAssociationLabelsRemainOpaque(t, report)
}

func TestCoreSnapshotDryRunIsStableAcrossInsertionOrder(t *testing.T) {
	first := openLegacyDatabase(t, "permutation-first")
	installCoreLegacySchema(t, first)
	insertCoreLegacyRows(t, first, []string{
		`INSERT INTO artifacts (id, title, content) VALUES ('a', 'A', 'artifact A')`,
		`INSERT INTO artifacts (id, title, content) VALUES ('b', 'B', 'artifact B')`,
		`INSERT INTO holons (id, title, content) VALUES ('a', 'A old', 'holon A')`,
		`INSERT INTO holons (id, title, content) VALUES ('b', 'B old', 'holon B')`,
		`INSERT INTO artifact_links (source_id, target_id, link_type, created_at) VALUES ('a', 'b', 'informs', '2026-07-16T00:00:00Z')`,
		`INSERT INTO relations (source_id, target_id, relation_type, congruence_level) VALUES ('b', 'a', 'related', 2)`,
	})
	second := openLegacyDatabase(t, "permutation-second")
	installCoreLegacySchema(t, second)
	insertCoreLegacyRows(t, second, []string{
		`INSERT INTO relations (source_id, target_id, relation_type, congruence_level) VALUES ('b', 'a', 'related', 2)`,
		`INSERT INTO artifact_links (source_id, target_id, link_type, created_at) VALUES ('a', 'b', 'informs', '2026-07-16T00:00:00Z')`,
		`INSERT INTO holons (id, title, content) VALUES ('b', 'B old', 'holon B')`,
		`INSERT INTO holons (id, title, content) VALUES ('a', 'A old', 'holon A')`,
		`INSERT INTO artifacts (id, title, content) VALUES ('b', 'B', 'artifact B')`,
		`INSERT INTO artifacts (id, title, content) VALUES ('a', 'A', 'artifact A')`,
	})

	firstReport := dryRunCore(t, first)
	secondReport := dryRunCore(t, second)
	if !bytes.Equal(firstReport.CanonicalBytes(), secondReport.CanonicalBytes()) {
		t.Fatalf(
			"permuted inserts changed dry-run report\nfirst:  %s\nsecond: %s",
			firstReport.CanonicalBytes(),
			secondReport.CanonicalBytes(),
		)
	}
}

func TestCoreSnapshotPreservesEmbeddedNULIdentityWithoutCoordinateCollision(
	t *testing.T,
) {
	database := openLegacyDatabase(t, "nul-identity")
	installCoreLegacySchema(t, database)
	insertCoreLegacyRows(t, database, []string{
		`INSERT INTO artifacts (id, title, content) VALUES (char(97,0,98), 'NUL', 'artifact')`,
		`INSERT INTO holons (id, title, content) VALUES (char(97,0,98), 'NUL old', 'holon')`,
	})

	report := dryRunCore(t, database)
	if got := report.Summary().CarrierOnly(); got != 1 {
		t.Fatalf("carrier-only count = %d, want one coalesced NUL identity", got)
	}
	if got := report.CarrierCatalog().Len(); got != 2 {
		t.Fatalf("carrier count = %d, want 2", got)
	}
}

func TestCoreSnapshotRejectsMissingOrWeakKeySchema(t *testing.T) {
	t.Run("missing required table", func(t *testing.T) {
		database := openLegacyDatabase(t, "missing-table")
		mustExec(t, database, `
			CREATE TABLE artifacts (
				id TEXT PRIMARY KEY,
				title TEXT NOT NULL,
				content TEXT NOT NULL
			)
		`)
		loader, err := NewCoreSnapshotLoader(database)
		if err != nil {
			t.Fatalf("NewCoreSnapshotLoader() error = %v", err)
		}
		_, err = loader.Load(context.Background())
		if !errors.Is(err, ErrUnsupportedLegacySchema) {
			t.Fatalf(
				"Load() error = %v, want ErrUnsupportedLegacySchema",
				err,
			)
		}
	})

	t.Run("non-text identity", func(t *testing.T) {
		database := openLegacyDatabase(t, "weak-key")
		installCoreLegacySchema(t, database)
		insertCoreLegacyRows(t, database, []string{
			`INSERT INTO artifacts (id, title, content) VALUES (x'3432', 'blob', 'identity')`,
		})
		loader, err := NewCoreSnapshotLoader(database)
		if err != nil {
			t.Fatalf("NewCoreSnapshotLoader() error = %v", err)
		}
		_, err = loader.Load(context.Background())
		if !errors.Is(err, ErrUnsupportedLegacySchema) {
			t.Fatalf(
				"Load() error = %v, want ErrUnsupportedLegacySchema",
				err,
			)
		}
	})
}

func TestCoreSnapshotChangesEditionWhenExactRowChanges(t *testing.T) {
	database := openLegacyDatabase(t, "row-change")
	installCoreLegacySchema(t, database)
	insertCoreLegacyRows(t, database, []string{
		`INSERT INTO artifacts (id, title, content) VALUES ('note-1', 'title', 'before')`,
	})
	before := dryRunCore(t, database)

	mustExec(
		t,
		database,
		`UPDATE artifacts SET content = 'after' WHERE id = 'note-1'`,
	)
	after := dryRunCore(t, database)

	if before.SourceSnapshotDigest() == after.SourceSnapshotDigest() {
		t.Fatal("exact legacy row change did not change source snapshot digest")
	}
	beforeCoordinate := carrierCoordinateByTable(t, before, "artifacts")
	afterCoordinate := carrierCoordinateByTable(t, after, "artifacts")
	if beforeCoordinate != afterCoordinate {
		t.Fatalf(
			"logical row coordinate changed across edition: %q != %q",
			beforeCoordinate,
			afterCoordinate,
		)
	}
}

func TestCoreSnapshotReadsCurrentEmptyHaftSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "haft.db")
	store, err := kerneldbfixture.OpenCurrentStore(path)
	if err != nil {
		t.Fatalf("db.NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("store.Close() error = %v", err)
		}
	})
	before := totalChanges(t, store.GetRawDB())

	loader, err := NewCoreSnapshotLoader(store.GetRawDB())
	if err != nil {
		t.Fatalf("NewCoreSnapshotLoader() error = %v", err)
	}
	report, err := loader.DryRun(context.Background(), mustProjectID(t))
	if err != nil {
		t.Fatalf("DryRun(current schema) error = %v", err)
	}

	if got := report.Summary().Total(); got != 0 {
		t.Fatalf("empty current schema dry-run total = %d, want 0", got)
	}
	if after := totalChanges(t, store.GetRawDB()); after != before {
		t.Fatalf(
			"current-schema DryRun() total changes = %d, want unchanged %d",
			after,
			before,
		)
	}
}

func assertSharedObjectHasBothHistoricalCarriers(
	t *testing.T,
	report legacyimport.DryRunReport,
) {
	t.Helper()
	for _, item := range report.Items() {
		carrierOnly, ok := item.(legacyimport.CarrierOnly)
		if !ok {
			continue
		}
		if len(carrierOnly.Observations()) != 2 {
			t.Fatalf(
				"coalesced object observations = %d, want artifact + holon",
				len(carrierOnly.Observations()),
			)
		}
		return
	}
	t.Fatal("coalesced carrier-only object was not found")
}

func assertAssociationLabelsRemainOpaque(
	t *testing.T,
	report legacyimport.DryRunReport,
) {
	t.Helper()
	found := 0
	for _, item := range report.Items() {
		unbound, ok := item.(legacyimport.LegacyUnbound)
		if !ok {
			continue
		}
		for _, observation := range unbound.Observations() {
			association, associationOK :=
				observation.(legacyimport.AssociationObservation)
			if !associationOK {
				t.Fatalf("legacy-unbound observation = %T", observation)
			}
			if association.Label().String() == "governs" ||
				association.Label().String() == "dependsOnProjected" {
				t.Fatalf(
					"legacy label %q escaped opaque namespace",
					association.Label().String(),
				)
			}
			found++
		}
	}
	if found != 2 {
		t.Fatalf("association observations = %d, want 2", found)
	}
}

func carrierCoordinateByTable(
	t *testing.T,
	report legacyimport.DryRunReport,
	table string,
) string {
	t.Helper()
	prefix := "sqlite/" + table + "/"
	for _, carrier := range report.CarrierCatalog().Snapshots() {
		coordinate := carrier.SourceCoordinate().String()
		if len(coordinate) >= len(prefix) &&
			coordinate[:len(prefix)] == prefix {
			return coordinate
		}
	}
	t.Fatalf("carrier table %s was not found", table)
	return ""
}

func dryRunCore(
	t *testing.T,
	database *sql.DB,
) legacyimport.DryRunReport {
	t.Helper()
	loader, err := NewCoreSnapshotLoader(database)
	if err != nil {
		t.Fatalf("NewCoreSnapshotLoader() error = %v", err)
	}
	report, err := loader.DryRun(context.Background(), mustProjectID(t))
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}
	return report
}

func openLegacyDatabase(t *testing.T, name string) *sql.DB {
	t.Helper()
	database, err := sql.Open(
		"sqlite",
		"file:legacyimportsqlite-"+name+"?mode=memory&cache=shared",
	)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("database.Close() error = %v", err)
		}
	})
	return database
}

func installCoreLegacySchema(t *testing.T, database *sql.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL
		)`,
		`CREATE TABLE artifact_links (
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			link_type TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (source_id, target_id, link_type)
		)`,
		`CREATE TABLE holons (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL
		)`,
		`CREATE TABLE relations (
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			relation_type TEXT NOT NULL,
			congruence_level INTEGER NOT NULL,
			PRIMARY KEY (source_id, target_id, relation_type)
		)`,
	}
	for _, statement := range statements {
		mustExec(t, database, statement)
	}
}

func insertCoreLegacyRows(
	t *testing.T,
	database *sql.DB,
	statements []string,
) {
	t.Helper()
	for _, statement := range statements {
		mustExec(t, database, statement)
	}
}

func mustExec(t *testing.T, database *sql.DB, statement string) {
	t.Helper()
	if _, err := database.Exec(statement); err != nil {
		t.Fatalf("Exec(%q) error = %v", statement, err)
	}
}

func totalChanges(t *testing.T, database *sql.DB) int64 {
	t.Helper()
	var total int64
	if err := database.QueryRow(`SELECT total_changes()`).Scan(&total); err != nil {
		t.Fatalf("total_changes() error = %v", err)
	}
	return total
}

func mustProjectID(t *testing.T) projectidentity.ProjectID {
	t.Helper()
	projectID, err := projectidentity.ParseProjectID("qnt_e3149c17")
	if err != nil {
		t.Fatalf("ParseProjectID() error = %v", err)
	}
	return projectID
}
