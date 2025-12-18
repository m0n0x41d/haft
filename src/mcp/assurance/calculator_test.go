package assurance

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	// Use cache=shared to share DB across connections in the pool
	db, err := sql.Open("sqlite3", "file:memdb1?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.SetMaxOpenConns(1) // Ensure single connection to avoid issues

	schema := `
	CREATE TABLE holons (id TEXT PRIMARY KEY, cached_r_score REAL DEFAULT 0.0);
	CREATE TABLE evidence (id TEXT PRIMARY KEY, holon_id TEXT, verdict TEXT, valid_until DATETIME);
	CREATE TABLE relations (source_id TEXT, target_id TEXT, relation_type TEXT, congruence_level INTEGER);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}
	return db
}

func TestCalculateReliability_SelfScore(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert evidence for holon A (PASS)
	_, err := db.Exec("INSERT INTO evidence (id, holon_id, verdict, valid_until) VALUES ('e1', 'A', 'pass', ?)", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("failed to insert evidence: %v", err)
	}

	calc := New(db)
	report, err := calc.CalculateReliability(context.Background(), "A")
	if err != nil {
		t.Fatalf("CalculateReliability failed: %v", err)
	}

	if report.FinalScore != 1.0 {
		t.Errorf("Expected score 1.0, got %f", report.FinalScore)
	}
}

func TestCalculateReliability_EvidenceDecay(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert expired evidence for holon A
	expired := time.Now().Add(-24 * time.Hour)
	_, err := db.Exec("INSERT INTO evidence (id, holon_id, verdict, valid_until) VALUES ('e1', 'A', 'pass', ?)", expired)
	if err != nil {
		t.Fatalf("failed to insert evidence: %v", err)
	}

	calc := New(db)
	report, err := calc.CalculateReliability(context.Background(), "A")
	if err != nil {
		t.Fatalf("CalculateReliability failed: %v", err)
	}

	// Should be penalized (0.1)
	if report.FinalScore != 0.1 {
		t.Errorf("Expected score 0.1 due to decay, got %f", report.FinalScore)
	}
}

func TestCalculateReliability_WeakestLink(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, verdict, valid_until) VALUES ('e1', 'A', 'pass', ?)", time.Now().Add(24*time.Hour))
	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, verdict, valid_until) VALUES ('e2', 'B', 'fail', ?)", time.Now().Add(24*time.Hour))
	
	// B is component of A
	_, _ = db.Exec("INSERT INTO relations (source_id, target_id, relation_type, congruence_level) VALUES ('B', 'A', 'componentOf', 3)")

	calc := New(db)
	report, err := calc.CalculateReliability(context.Background(), "A")
	if err != nil {
		t.Fatalf("CalculateReliability failed: %v", err)
	}

	// B has 0.0. A has 1.0. Weakest link is B. Result should be 0.0.
	if report.FinalScore != 0.0 {
		t.Errorf("Expected score 0.0 (weakest link), got %f", report.FinalScore)
	}
}

func TestCalculateReliability_CLPenalty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, verdict, valid_until) VALUES ('e1', 'A', 'pass', ?)", time.Now().Add(24*time.Hour))
	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, verdict, valid_until) VALUES ('e2', 'B', 'pass', ?)", time.Now().Add(24*time.Hour))
	
	_, _ = db.Exec("INSERT INTO relations (source_id, target_id, relation_type, congruence_level) VALUES ('B', 'A', 'componentOf', 1)")

	calc := New(db)
	report, err := calc.CalculateReliability(context.Background(), "A")
	if err != nil {
		t.Fatalf("CalculateReliability failed: %v", err)
	}

	if report.FinalScore != 0.6 {
		t.Errorf("Expected score 0.6 (CL penalty), got %f", report.FinalScore)
	}
}