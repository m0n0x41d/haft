package sqlitepolicy

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestWritableProjectLedgerConnectionsPersistOneWALGeneration(
	t *testing.T,
) {
	databasePath := filepath.Join(
		t.TempDir(),
		".haft",
		"projects",
		"qnt_a7f3b2c1",
		"haft.db",
	)
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", writableProjectLedgerDSN(databasePath))
	if err != nil {
		t.Fatalf("open project ledger: %v", err)
	}
	database.SetMaxOpenConns(3)
	if _, err := database.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = database.Close()
		t.Fatalf("enable WAL mode: %v", err)
	}
	if _, err := database.Exec(
		"CREATE TABLE persistent_wal_probe (probe_id TEXT PRIMARY KEY)",
	); err != nil {
		_ = database.Close()
		t.Fatalf("create WAL probe: %v", err)
	}

	connections := acquirePersistentWALConnections(t, database, 3)
	for _, connection := range connections {
		if err := connection.Close(); err != nil {
			_ = database.Close()
			t.Fatalf("close pooled project ledger connection: %v", err)
		}
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close project ledger: %v", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(databasePath + suffix); err != nil {
			t.Fatalf("persistent project ledger sidecar %s: %v", suffix, err)
		}
	}
}

func TestProjectLedgerConnectionPolicyIsNarrowAndReadOnlySafe(t *testing.T) {
	projectLedgerPath := filepath.Join(
		t.TempDir(),
		".haft",
		"projects",
		"qnt_b7f3b2c1",
		"haft.db",
	)
	cases := []struct {
		name string
		dsn  string
		want bool
	}{
		{
			name: "writable project ledger URI",
			dsn:  writableProjectLedgerDSN(projectLedgerPath),
			want: true,
		},
		{
			name: "read-only project ledger URI",
			dsn:  writableProjectLedgerDSN(projectLedgerPath) + "&mode=ro",
			want: false,
		},
		{
			name: "ordinary SQLite database",
			dsn:  filepath.Join(t.TempDir(), "haft.db"),
			want: false,
		},
		{
			name: "in-memory SQLite database",
			dsn:  ":memory:",
			want: false,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := isWritableProjectLedgerDSN(testCase.dsn)
			if got != testCase.want {
				t.Fatalf(
					"isWritableProjectLedgerDSN(%q) = %v, want %v",
					testCase.dsn,
					got,
					testCase.want,
				)
			}
		})
	}
}

func acquirePersistentWALConnections(
	t *testing.T,
	database *sql.DB,
	count int,
) []*sql.Conn {
	t.Helper()
	connections := make([]*sql.Conn, 0, count)
	for range count {
		connection, err := database.Conn(context.Background())
		if err != nil {
			t.Fatalf("acquire pooled project ledger connection: %v", err)
		}
		connections = append(connections, connection)
	}
	return connections
}

func writableProjectLedgerDSN(databasePath string) string {
	query := url.Values{}
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "foreign_keys(1)")
	dsn := url.URL{
		Scheme:   "file",
		Path:     databasePath,
		RawQuery: query.Encode(),
	}
	return dsn.String()
}
