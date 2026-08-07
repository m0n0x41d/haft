package fpf

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSourceIndexMetadataRequiresExplicitGrammarAndRoundTrips(t *testing.T) {
	if SpecIndexSchemaVersion != "11" {
		t.Fatalf("source Query plus TypeEnv grammar requires schema version 11, got %s", SpecIndexSchemaVersion)
	}
	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := SetSpecMetaEntries(dbPath, map[string]string{"schema_version": SpecIndexSchemaVersion}); err == nil {
		t.Fatal("SetSpecMetaEntries() silently created missing metadata grammar")
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open metadata db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatalf("create metadata grammar: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close metadata db: %v", err)
	}

	revision := strings.Repeat("a", 40)
	body := "source metadata fixture"
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen metadata source db: %v", err)
	}
	if err := EnsureSourceQuerySchemaDB(db); err != nil {
		t.Fatalf("EnsureSourceQuerySchemaDB() error: %v", err)
	}
	relationsDigest, err := sourceRelationsDigest(nil)
	if err != nil {
		t.Fatalf("sourceRelationsDigest() error: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO source_units (
			unit_id, source_role, title, body, relations_digest,
			source_path, start_line, end_line, content_hash, source_revision
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"readme:preface:metadata-fixture",
		SourceUnitRolePreface,
		"Metadata fixture",
		body,
		relationsDigest,
		"Readme.md",
		1,
		1,
		sourceContentHash(body),
		revision,
	); err != nil {
		t.Fatalf("insert metadata source row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close metadata source db: %v", err)
	}
	readmeDigest := "sha256:" + strings.Repeat("1", 64)
	specDigest := "sha256:" + strings.Repeat("2", 64)
	entries := map[string]string{
		"schema_version":         SpecIndexSchemaVersion,
		"fpf_commit":             revision,
		"readme_document_digest": readmeDigest,
		"spec_document_digest":   specDigest,
	}
	if err := SetSpecMetaEntries(dbPath, entries); err != nil {
		t.Fatalf("SetSpecMetaEntries() error: %v", err)
	}

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen metadata db: %v", err)
	}
	defer func() { _ = db.Close() }()
	value, err := GetSpecMeta(db, "schema_version")
	if err != nil {
		t.Fatalf("GetSpecMeta() error: %v", err)
	}
	if value != SpecIndexSchemaVersion {
		t.Fatalf("schema version = %q, want %q", value, SpecIndexSchemaVersion)
	}
	snapshot, err := LoadQuerySourceSnapshot(db)
	if err != nil {
		t.Fatalf("LoadQuerySourceSnapshot() error: %v", err)
	}
	if snapshot.IndexSchemaVersion() != SpecIndexSchemaVersion ||
		snapshot.Revision() != revision ||
		snapshot.ReadmeDigest() != readmeDigest ||
		snapshot.SpecDigest() != specDigest {
		t.Fatalf("query source snapshot = %#v", snapshot)
	}
	if _, err := db.Exec(
		`UPDATE meta SET value = ? WHERE key = 'fpf_commit'`,
		strings.Repeat("b", 40),
	); err != nil {
		t.Fatalf("tamper source revision metadata: %v", err)
	}
	if _, err := LoadQuerySourceSnapshot(db); err == nil {
		t.Fatal("source metadata drifted from indexed provenance without rejection")
	}
}
