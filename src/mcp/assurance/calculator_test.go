package assurance

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	// Use cache=shared to share DB across connections in the pool
	db, err := sql.Open("sqlite", "file:memdb1?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	db.SetMaxOpenConns(1) // Ensure single connection to avoid issues

	schema := `
	CREATE TABLE holons (id TEXT PRIMARY KEY, cached_r_score REAL DEFAULT 0.0);
	CREATE TABLE evidence (id TEXT PRIMARY KEY, holon_id TEXT, type TEXT, verdict TEXT, valid_until DATETIME);
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
	_, err := db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e1', 'A', 'internal', 'pass', ?)", time.Now().Add(24*time.Hour))
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
	_, err := db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e1', 'A', 'internal', 'pass', ?)", expired)
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

	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e1', 'A', 'internal', 'pass', ?)", time.Now().Add(24*time.Hour))
	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e2', 'B', 'internal', 'fail', ?)", time.Now().Add(24*time.Hour))

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

	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e1', 'A', 'internal', 'pass', ?)", time.Now().Add(24*time.Hour))
	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e2', 'B', 'internal', 'pass', ?)", time.Now().Add(24*time.Hour))

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

func TestCalculateReliability_CycleDetection(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create A→B→C→A cycle via componentOf relations
	// A contains B, B contains C, C contains A (circular)
	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e1', 'A', 'internal', 'pass', ?)", time.Now().Add(24*time.Hour))
	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e2', 'B', 'internal', 'pass', ?)", time.Now().Add(24*time.Hour))
	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e3', 'C', 'internal', 'pass', ?)", time.Now().Add(24*time.Hour))

	// B is component of A, C is component of B, A is component of C (cycle!)
	_, _ = db.Exec("INSERT INTO relations (source_id, target_id, relation_type, congruence_level) VALUES ('B', 'A', 'componentOf', 3)")
	_, _ = db.Exec("INSERT INTO relations (source_id, target_id, relation_type, congruence_level) VALUES ('C', 'B', 'componentOf', 3)")
	_, _ = db.Exec("INSERT INTO relations (source_id, target_id, relation_type, congruence_level) VALUES ('A', 'C', 'componentOf', 3)")

	calc := New(db)
	report, err := calc.CalculateReliability(context.Background(), "A")

	// Should not error or hang - cycle should be detected and handled gracefully
	if err != nil {
		t.Fatalf("CalculateReliability failed on cycle: %v", err)
	}

	// All have passing evidence, no CL penalty, cycle should not affect final score
	// Each node has self-score 1.0, and deps also 1.0 (cycle broken by visited check)
	if report.FinalScore != 1.0 {
		t.Errorf("Expected score 1.0 (cycle handled gracefully), got %f", report.FinalScore)
	}
}

func TestCalculateReliability_ExternalEvidencePenalty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert external evidence for holon A (should get CL2 penalty: 10%)
	_, err := db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e1', 'A', 'external', 'pass', ?)", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("failed to insert evidence: %v", err)
	}

	calc := New(db)
	report, err := calc.CalculateReliability(context.Background(), "A")
	if err != nil {
		t.Fatalf("CalculateReliability failed: %v", err)
	}

	// External evidence should have 10% penalty: 1.0 - 0.1 = 0.9
	if report.FinalScore != 0.9 {
		t.Errorf("Expected score 0.9 (external CL2 penalty), got %f", report.FinalScore)
	}

	// Check that the penalty factor was recorded
	hasPenaltyFactor := false
	for _, f := range report.Factors {
		if f == "External evidence CL2 penalty applied" {
			hasPenaltyFactor = true
			break
		}
	}
	if !hasPenaltyFactor {
		t.Errorf("Expected 'External evidence CL2 penalty applied' in factors, got %v", report.Factors)
	}
}

func TestCalculateReliability_MixedEvidenceWLNK(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert both internal and external evidence for holon A
	// WLNK should use the weaker one (external with penalty)
	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e1', 'A', 'internal', 'pass', ?)", time.Now().Add(24*time.Hour))
	_, _ = db.Exec("INSERT INTO evidence (id, holon_id, type, verdict, valid_until) VALUES ('e2', 'A', 'external', 'pass', ?)", time.Now().Add(24*time.Hour))

	calc := New(db)
	report, err := calc.CalculateReliability(context.Background(), "A")
	if err != nil {
		t.Fatalf("CalculateReliability failed: %v", err)
	}

	// WLNK: min(1.0 internal, 0.9 external) = 0.9
	if report.FinalScore != 0.9 {
		t.Errorf("Expected score 0.9 (WLNK on mixed evidence), got %f", report.FinalScore)
	}
}
