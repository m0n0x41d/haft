package authority

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

const (
	frozenLegacyAuthoritySchema43CarrierDigest    = "sha256:17cb3f85c7e8208c3abbd4671e0e43ec4aaa4f618c7d93f2da191a1ece1be607"
	frozenLegacyAuthoritySchema43CompressedDigest = "sha256:219f63e8720dab5586f8531c0b2247416b200127a1e17e98b4b1840c03343b7f"
	frozenLegacyAuthoritySchema43DatabaseDigest   = "sha256:847e55beb1ad3baf3a01edb2d8ece974f4346d3c2b2616c856b70e535c7dbd67"
)

//go:embed testdata/legacy_authority_schema43.sqlite.b64.gz
var frozenLegacyAuthoritySchema43Carrier []byte

func openFrozenLegacyAuthoritySchema43(t *testing.T) *sql.DB {
	t.Helper()
	assertFixtureDigest(
		t,
		"base64 carrier",
		frozenLegacyAuthoritySchema43Carrier,
		frozenLegacyAuthoritySchema43CarrierDigest,
	)
	carrierText := strings.TrimSpace(string(frozenLegacyAuthoritySchema43Carrier))
	compressed, err := base64.StdEncoding.DecodeString(carrierText)
	if err != nil {
		t.Fatalf("decode frozen schema43 carrier: %v", err)
	}
	assertFixtureDigest(
		t,
		"compressed database",
		compressed,
		frozenLegacyAuthoritySchema43CompressedDigest,
	)
	compressedReader := bytes.NewReader(compressed)
	zipper, err := gzip.NewReader(compressedReader)
	if err != nil {
		t.Fatalf("open frozen schema43 gzip stream: %v", err)
	}
	databaseBytes, err := io.ReadAll(zipper)
	if err != nil {
		_ = zipper.Close()
		t.Fatalf("read frozen schema43 database: %v", err)
	}
	if err := zipper.Close(); err != nil {
		t.Fatalf("close frozen schema43 gzip stream: %v", err)
	}
	assertFixtureDigest(
		t,
		"database",
		databaseBytes,
		frozenLegacyAuthoritySchema43DatabaseDigest,
	)
	databasePath := filepath.Join(t.TempDir(), "legacy-authority-schema43.sqlite")
	if err := os.WriteFile(databasePath, databaseBytes, 0o600); err != nil {
		t.Fatalf("restore frozen schema43 database: %v", err)
	}
	query := url.Values{}
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "foreign_keys(1)")
	location := url.URL{
		Scheme:   "file",
		Path:     databasePath,
		RawQuery: query.Encode(),
	}
	dsn := location.String()
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open frozen schema43 database: %v", err)
	}
	if err := database.Ping(); err != nil {
		_ = database.Close()
		t.Fatalf("ping frozen schema43 database: %v", err)
	}
	database.SetMaxOpenConns(1)
	assertFrozenLegacyAuthoritySchema43(t, database)
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close frozen schema43 database: %v", err)
		}
	})
	return database
}

func assertFixtureDigest(
	t *testing.T,
	label string,
	payload []byte,
	want string,
) {
	t.Helper()
	sum := sha256.Sum256(payload)
	encoded := hex.EncodeToString(sum[:])
	got := "sha256:" + encoded
	if got != want {
		t.Fatalf("%s digest = %q, want %q", label, got, want)
	}
}

func assertFrozenLegacyAuthoritySchema43(
	t *testing.T,
	database *sql.DB,
) {
	t.Helper()
	maximumVersion := 0
	if err := database.QueryRow(
		`SELECT COALESCE(MAX(version), 0) FROM schema_version`,
	).Scan(&maximumVersion); err != nil {
		t.Fatalf("read frozen schema frontier: %v", err)
	}
	if maximumVersion != 43 {
		t.Fatalf("frozen schema frontier = %d, want 43", maximumVersion)
	}
	assertSQLiteObjectCount(
		t,
		database,
		"table",
		"profile_declaration_authority_bases_v2",
		1,
	)
	assertSQLiteObjectCount(
		t,
		database,
		"table",
		"profile_declaration_authority_resolutions_v2",
		0,
	)
	sealCount := 0
	if err := database.QueryRow(
		`SELECT COUNT(*)
		 FROM sqlite_master
		 WHERE type = 'trigger'
		   AND name LIKE '%_v44_writes_sealed'`,
	).Scan(&sealCount); err != nil {
		t.Fatalf("inspect frozen schema write seals: %v", err)
	}
	if sealCount != 0 {
		t.Fatalf("frozen schema43 contains %d schema44 write seal(s)", sealCount)
	}
	integrity := ""
	if err := database.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatalf("check frozen schema43 integrity: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("frozen schema43 integrity = %q", integrity)
	}
}

func assertSQLiteObjectCount(
	t *testing.T,
	database *sql.DB,
	kind string,
	name string,
	want int,
) {
	t.Helper()
	count := 0
	if err := database.QueryRow(
		`SELECT COUNT(*)
		 FROM sqlite_master
		 WHERE type = ? AND name = ?`,
		kind,
		name,
	).Scan(&count); err != nil {
		t.Fatalf("inspect SQLite %s %s: %v", kind, name, err)
	}
	if count != want {
		t.Fatalf("SQLite %s %s count = %d, want %d", kind, name, count, want)
	}
}
