package profileonboarding

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

func insertProfileOnboardingTestLedgerBinding(
	t *testing.T,
	database *sql.DB,
	root string,
	boundAt time.Time,
) {
	t.Helper()
	canonicalBoundAt := boundAt.UTC().Format(time.RFC3339Nano)
	binding := map[string]string{
		"schema":       "haft.project-ledger-binding/v1",
		"project_id":   "qnt_a7f3b2c1",
		"project_root": root,
		"bound_at":     canonicalBoundAt,
	}
	bindingJSON, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	bindingDigest := sha256.Sum256(bindingJSON)
	_, err = database.Exec(
		`INSERT INTO project_ledger_binding (
			binding_slot, project_id, project_root,
			binding_digest, binding_json, bound_at
		) VALUES (1, ?, ?, ?, ?, ?)`,
		"qnt_a7f3b2c1",
		root,
		"sha256:"+hex.EncodeToString(bindingDigest[:]),
		string(bindingJSON),
		canonicalBoundAt,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func assertProfileAuthorityClosureCounts(
	t *testing.T,
	database *sql.DB,
	want int,
) {
	t.Helper()
	tables := []string{
		"profile_declaration_permissions_v2",
		"profile_declaration_instituted_effects_v2",
		"profile_declaration_authority_bases_v2",
	}
	assertProfileAuthorityTableCounts(t, database, tables, want)
}

func openProfileAuthoritySourceTestDatabase(
	t *testing.T,
	name string,
) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".db")
	store, err := kerneldbfixture.OpenCurrentStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store.GetRawDB()
}

func assertProfileAuthoritySourceCounts(
	t *testing.T,
	database *sql.DB,
	want int,
) {
	t.Helper()
	tables := []string{
		"profile_declaration_authorization_contents_v2",
		"profile_declaration_authorization_preparations_v2",
		"terminal_capture_records",
		"speech_acts",
	}
	assertProfileAuthorityTableCounts(t, database, tables, want)
}

func assertProfileAuthorityTableCounts(
	t *testing.T,
	database *sql.DB,
	tables []string,
	want int,
) {
	t.Helper()
	if len(tables) == 0 {
		return
	}
	table := tables[0]
	var got int
	query := "SELECT COUNT(*) FROM " + table
	if err := database.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
	assertProfileAuthorityTableCounts(t, database, tables[1:], want)
}
