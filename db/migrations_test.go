package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunMigrations_FreshDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Check schema_version table exists and has entries
	var count int
	err = store.conn.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query schema_version: %v", err)
	}
	if count != len(kernelMigrations) {
		t.Errorf("Expected %d kernelMigrations recorded, got %d", len(kernelMigrations), count)
	}
}

func TestRunMigrations_ExistingDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create database with old schema (no parent_id, no cached_r_score)
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}

	oldSchema := `CREATE TABLE holons (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		kind TEXT,
		layer TEXT NOT NULL,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		context_id TEXT NOT NULL,
		scope TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := conn.Exec(oldSchema); err != nil {
		t.Fatalf("Failed to create old schema: %v", err)
	}
	conn.Close()

	// Now open with NewStore which runs kernelMigrations
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	// Verify new columns exist by querying them
	var parentID sql.NullString
	var cachedRScore sql.NullFloat64
	err = store.conn.QueryRow("SELECT parent_id, cached_r_score FROM holons LIMIT 1").Scan(&parentID, &cachedRScore)
	// Will get sql.ErrNoRows since table is empty, but query should not fail due to missing columns
	if err != nil && err != sql.ErrNoRows {
		t.Errorf("New columns should exist: %v", err)
	}

	// Verify kernelMigrations are recorded
	var count int
	store.conn.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count)
	if count != len(kernelMigrations) {
		t.Errorf("Expected %d kernelMigrations recorded, got %d", len(kernelMigrations), count)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Run kernelMigrations twice
	store1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("First NewStore failed: %v", err)
	}
	store1.Close()

	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("Second NewStore failed: %v", err)
	}
	defer store2.Close()

	// Should still have same number of migration records
	var count int
	store2.conn.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count)
	if count != len(kernelMigrations) {
		t.Errorf("Expected %d kernelMigrations, got %d (not idempotent)", len(kernelMigrations), count)
	}
}

func TestRunMigrations_AddsEpistemicDebtBudget(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	var budget sql.NullFloat64
	err = store.conn.QueryRow(
		"SELECT epistemic_debt_budget FROM fpf_state LIMIT 1",
	).Scan(&budget)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("query epistemic_debt_budget: %v", err)
	}
}
