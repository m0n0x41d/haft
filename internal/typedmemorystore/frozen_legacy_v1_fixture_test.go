package typedmemorystore

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/db"
)

// frozenLegacyV1CurrentSchemaSQL is an immutable, sanitized SQL dump of one
// exact writer-v46 legacy admission after the database reached schema 54. It
// is a historical storage carrier, not a recipe for reopening legacy writes.
//
//go:embed testdata/historical_legacy_v1_current_schema.sql.b64.gz
var frozenLegacyV1CurrentSchemaSQL string

func newFrozenLegacyV1GenericMixedStoreFixture(
	t *testing.T,
) genericMixedStoreFixture {
	t.Helper()
	fixture := newUnbootstrappedGenericMixedStoreFixture(t)
	databasePath := fixture.base.databasePath
	if err := fixture.base.store.Close(); err != nil {
		t.Fatalf("close generated fixture before frozen restore: %v", err)
	}
	removeFrozenFixtureFile(t, databasePath)
	removeFrozenFixtureFile(t, databasePath+"-wal")
	removeFrozenFixtureFile(t, databasePath+"-shm")

	dump := decodeFrozenLegacyV1CurrentSchemaSQL(t)
	restoreFrozenSQLDump(t, databasePath, dump)
	store, err := db.NewStore(databasePath)
	if err != nil {
		t.Fatalf("open frozen legacy-v1 store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	base := fixture.base
	base.store = store
	base.database = store.GetRawDB()
	base.adapter = base.adapterFor(t, base.database)
	fixture.base = base
	loader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    fixture.registry,
	}
	adapter, err := NewGenericSQLiteAdapter(
		base.database,
		loader,
		base.clock,
		unexpectedMemberOfEngine{},
		unexpectedReferenceEngine{},
		unexpectedObservableProvider{},
	)
	if err != nil {
		t.Fatalf("open generic adapter over frozen legacy-v1 store: %v", err)
	}
	fixture.adapter = adapter
	assertFrozenLegacyV1StorageBoundary(t, base.database)
	return fixture
}

func decodeFrozenLegacyV1CurrentSchemaSQL(t *testing.T) []byte {
	t.Helper()
	encoded := strings.TrimSpace(frozenLegacyV1CurrentSchemaSQL)
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode frozen legacy-v1 SQL carrier: %v", err)
	}
	compressedReader := bytes.NewReader(compressed)
	reader, err := gzip.NewReader(compressedReader)
	if err != nil {
		t.Fatalf("open frozen legacy-v1 SQL carrier: %v", err)
	}
	defer reader.Close()
	dump, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("inflate frozen legacy-v1 SQL carrier: %v", err)
	}
	return dump
}

func restoreFrozenSQLDump(
	t *testing.T,
	databasePath string,
	dump []byte,
) {
	t.Helper()
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open empty frozen legacy-v1 database: %v", err)
	}
	closed := false
	defer func() {
		if closed {
			return
		}
		_ = database.Close()
	}()
	if _, err := database.Exec(string(dump)); err != nil {
		t.Fatalf("restore frozen legacy-v1 SQL carrier: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close restored frozen legacy-v1 database: %v", err)
	}
	closed = true
}

func removeFrozenFixtureFile(t *testing.T, path string) {
	t.Helper()
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return
	}
	t.Fatalf("remove generated fixture file %q: %v", path, err)
}

func assertFrozenLegacyV1StorageBoundary(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	const freezeTrigger = "typed_memory_relation_instances_v53_legacy_insert_frozen"
	var triggerSQL string
	err := database.QueryRow(
		"SELECT sql FROM sqlite_master WHERE type = 'trigger' AND name = ?",
		freezeTrigger,
	).Scan(&triggerSQL)
	if err != nil || triggerSQL == "" {
		t.Fatalf("frozen legacy-v1 insert guard = %q, %v", triggerSQL, err)
	}
	var assertionID string
	err = database.QueryRow(
		`SELECT assertion_id
		FROM typed_memory_relation_instances
		WHERE project_id = 'qnt_a7f3b2c1'`,
	).Scan(&assertionID)
	if err != nil || assertionID != "assertion:historical-legacy" {
		t.Fatalf("frozen legacy-v1 assertion = %q, %v", assertionID, err)
	}
	var integrity string
	if err := database.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatalf("inspect frozen legacy-v1 integrity: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("frozen legacy-v1 integrity = %q; want ok", integrity)
	}
	var maximumVersion int
	if err := database.QueryRow("SELECT MAX(version) FROM schema_version").Scan(
		&maximumVersion,
	); err != nil {
		t.Fatalf("inspect frozen legacy-v1 schema frontier: %v", err)
	}
	if maximumVersion != 57 {
		t.Fatalf(
			"frozen legacy-v1 schema frontier = %d; want 57",
			maximumVersion,
		)
	}
}
