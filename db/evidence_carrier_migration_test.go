package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestEvidenceCarrierMigration59PreservesRowsAndBackfillsUpdatedAt(t *testing.T) {
	t.Parallel()

	databasePath := filepath.Join(t.TempDir(), "schema-58.db")
	dsn, err := sqliteConnectionDSN(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(
		database,
		"schema_version",
		migrationsBeforeVersion(kernelMigrations, evidenceCarrierSchemaVersion59, 0, nil),
	); err != nil {
		t.Fatal(err)
	}
	createdAt := "2026-08-10T12:34:56Z"
	if _, err := database.Exec(`
		INSERT INTO artifacts (
			id, kind, version, status, title, content, created_at, updated_at
		) VALUES (?, 'Note', 1, 'active', 'Migration parent', 'Preserve me', ?, ?)`,
		"note-migration-59-parent",
		createdAt,
		createdAt,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO evidence_items (
			id, artifact_ref, type, content, verdict,
			congruence_level, formality_level, valid_until, created_at
		) VALUES (?, ?, 'test', 'preserved observation', 'supports', 3, 5, ?, ?)`,
		"evid-migration-59",
		"note-migration-59-parent",
		"2026-09-10",
		createdAt,
	); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(database, "schema_version", kernelMigrations); err != nil {
		t.Fatal(err)
	}
	var content, causalBasis, updatedAt string
	if err := database.QueryRow(`
		SELECT content, causal_support_basis, updated_at
		FROM evidence_items
		WHERE id = 'evid-migration-59'`,
	).Scan(&content, &causalBasis, &updatedAt); err != nil {
		t.Fatal(err)
	}
	if content != "preserved observation" || causalBasis != "" || updatedAt != createdAt {
		t.Fatalf(
			"migrated evidence = content %q causal_basis %q updated_at %q",
			content,
			causalBasis,
			updatedAt,
		)
	}
	var debtEvidenceID, debtArtifactRef, debtCarrierPath, debtLastError string
	if err := database.QueryRow(`
		SELECT evidence_id, artifact_ref, carrier_path, last_error
		FROM evidence_carrier_projection_debt
		WHERE evidence_id = 'evid-migration-59'`,
	).Scan(&debtEvidenceID, &debtArtifactRef, &debtCarrierPath, &debtLastError); err != nil {
		t.Fatal(err)
	}
	if debtEvidenceID != "evid-migration-59" ||
		debtArtifactRef != "note-migration-59-parent" ||
		debtCarrierPath != ".haft/evidence/evid-migration-59.md" ||
		debtLastError != "legacy EvidenceRecord carrier backfill required; run haft sync" {
		t.Fatalf(
			"backfill debt = evidence %q parent %q path %q error %q",
			debtEvidenceID,
			debtArtifactRef,
			debtCarrierPath,
			debtLastError,
		)
	}
}
