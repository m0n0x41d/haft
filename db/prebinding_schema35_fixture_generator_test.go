package db

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const generatePrebindingSchema35FixtureEnv = "HAFT_GENERATE_PREBINDING_SCHEMA35_FIXTURE"

// TestGeneratePrebindingSchema35Fixture regenerates the empty released-schema
// fixture consumed by the explicit project-ledger migration integration test.
// It is intentionally opt-in and refuses to overwrite an existing fixture.
func TestGeneratePrebindingSchema35Fixture(t *testing.T) {
	t.Parallel()

	if os.Getenv(generatePrebindingSchema35FixtureEnv) != "1" {
		t.Skip(
			"set " + generatePrebindingSchema35FixtureEnv +
				"=1 to regenerate the schema-35 fixture",
		)
	}
	output := filepath.Join(
		"..",
		"internal",
		"projectledgermigration",
		"testdata",
		"schema35.db",
	)
	if _, err := os.Lstat(output); err == nil {
		t.Fatalf("refusing to overwrite existing fixture %s", output)
	} else if !os.IsNotExist(err) {
		t.Fatalf("inspect fixture destination %s: %v", output, err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	generatePrebindingSchema35Fixture(t, output)
}

func TestPrebindingSchema35FixtureMatchesGenerator(t *testing.T) {
	t.Parallel()

	generated := filepath.Join(t.TempDir(), "schema35.db")
	generatePrebindingSchema35Fixture(t, generated)
	committed := filepath.Join(
		"..",
		"internal",
		"projectledgermigration",
		"testdata",
		"schema35.db",
	)
	generatedBytes, err := os.ReadFile(generated)
	if err != nil {
		t.Fatalf("read generated schema-35 fixture: %v", err)
	}
	committedBytes, err := os.ReadFile(committed)
	if err != nil {
		t.Fatalf("read committed schema-35 fixture: %v", err)
	}
	if !bytes.Equal(generatedBytes, committedBytes) {
		t.Fatal(
			"committed schema-35 fixture differs from the deterministic generator",
		)
	}
}

func generatePrebindingSchema35Fixture(t *testing.T, output string) {
	t.Helper()
	database := openReleasedUpgradeDatabase(t, output, 35)
	if _, err := database.Exec(
		`UPDATE schema_version
		 SET applied_at = '2026-07-30T00:00:00Z'`,
	); err != nil {
		_ = database.Close()
		t.Fatalf("normalize schema-35 migration timestamps: %v", err)
	}
	if _, err := database.Exec("VACUUM"); err != nil {
		_ = database.Close()
		t.Fatalf("compact schema-35 fixture: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close schema-35 fixture: %v", err)
	}
	if err := os.Chmod(output, 0o600); err != nil {
		t.Fatalf("restrict schema-35 fixture mode: %v", err)
	}
}
