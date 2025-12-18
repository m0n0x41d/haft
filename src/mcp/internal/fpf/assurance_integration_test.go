package fpf_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"quint-mcp/assurance"
	"quint-mcp/db"
	"quint-mcp/internal/fpf"
)

func setupAssuranceTestEnv(t *testing.T) (*fpf.FSM, *db.DB, string) {
	tempDir := t.TempDir()
	quintDir := filepath.Join(tempDir, ".quint")
	if err := os.MkdirAll(quintDir, 0755); err != nil {
		t.Fatalf("Failed to create .quint directory: %v", err)
	}

	dbPath := filepath.Join(quintDir, "quint.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}

	fsm := &fpf.FSM{
		State: fpf.State{Phase: fpf.PhaseDecision},
		DB:    database.GetRawDB(),
	}

	return fsm, database, tempDir
}

func TestAssuranceGuard_BlocksLowR(t *testing.T) {
	fsm, database, tempDir := setupAssuranceTestEnv(t)
	rawDB := database.GetRawDB()

	// Create L2 evidence file (required for transition validation)
	l2Dir := filepath.Join(tempDir, ".quint", "knowledge", "L2")
	os.MkdirAll(l2Dir, 0755)
	l2File := filepath.Join(l2Dir, "test-holon.md")
	os.WriteFile(l2File, []byte("Test hypothesis"), 0644)

	// Create holon with failing evidence (R = 0.0)
	_, err := rawDB.Exec("INSERT INTO holons (id, type, layer, title, content, context_id) VALUES ('test-holon', 'hypothesis', 'L2', 'Test', 'Content', 'ctx')")
	if err != nil {
		t.Fatalf("Failed to insert holon: %v", err)
	}

	_, err = rawDB.Exec("INSERT INTO evidence (id, holon_id, type, content, verdict, valid_until) VALUES ('e1', 'test-holon', 'test', 'Failed test', 'fail', ?)", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Failed to insert evidence: %v", err)
	}

	// Try to transition to Operation - should be blocked
	ra := fpf.RoleAssignment{Role: fpf.RoleDecider, SessionID: "test", Context: "test"}
	ev := &fpf.EvidenceStub{URI: l2File, Type: "hypothesis", HolonID: "test-holon"}

	ok, msg := fsm.CanTransition(fpf.PhaseOperation, ra, ev)

	if ok {
		t.Errorf("Expected transition to be BLOCKED due to low R-score, but it was allowed")
	}
	if msg == "" || msg == "OK" {
		t.Errorf("Expected denial message, got: %s", msg)
	}
	t.Logf("Correctly blocked with message: %s", msg)
}

func TestAssuranceGuard_AllowsHighR(t *testing.T) {
	fsm, database, tempDir := setupAssuranceTestEnv(t)
	rawDB := database.GetRawDB()

	// Create L2 evidence file
	l2Dir := filepath.Join(tempDir, ".quint", "knowledge", "L2")
	os.MkdirAll(l2Dir, 0755)
	l2File := filepath.Join(l2Dir, "good-holon.md")
	os.WriteFile(l2File, []byte("Good hypothesis"), 0644)

	// Create holon with passing evidence (R = 1.0)
	_, err := rawDB.Exec("INSERT INTO holons (id, type, layer, title, content, context_id) VALUES ('good-holon', 'hypothesis', 'L2', 'Good', 'Content', 'ctx')")
	if err != nil {
		t.Fatalf("Failed to insert holon: %v", err)
	}

	_, err = rawDB.Exec("INSERT INTO evidence (id, holon_id, type, content, verdict, valid_until) VALUES ('e1', 'good-holon', 'test', 'Passed test', 'pass', ?)", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Failed to insert evidence: %v", err)
	}

	// Try to transition to Operation - should be allowed
	ra := fpf.RoleAssignment{Role: fpf.RoleDecider, SessionID: "test", Context: "test"}
	ev := &fpf.EvidenceStub{URI: l2File, Type: "hypothesis", HolonID: "good-holon"}

	ok, msg := fsm.CanTransition(fpf.PhaseOperation, ra, ev)

	if !ok {
		t.Errorf("Expected transition to be ALLOWED with high R-score, but got: %s", msg)
	}
}

func TestAssuranceGuard_RespectsConfigurableThreshold(t *testing.T) {
	fsm, database, tempDir := setupAssuranceTestEnv(t)
	rawDB := database.GetRawDB()

	// Create L2 evidence file
	l2Dir := filepath.Join(tempDir, ".quint", "knowledge", "L2")
	os.MkdirAll(l2Dir, 0755)
	l2File := filepath.Join(l2Dir, "medium-holon.md")
	os.WriteFile(l2File, []byte("Medium hypothesis"), 0644)

	// Create holon with degraded evidence (R = 0.5)
	_, err := rawDB.Exec("INSERT INTO holons (id, type, layer, title, content, context_id) VALUES ('medium-holon', 'hypothesis', 'L2', 'Medium', 'Content', 'ctx')")
	if err != nil {
		t.Fatalf("Failed to insert holon: %v", err)
	}

	_, err = rawDB.Exec("INSERT INTO evidence (id, holon_id, type, content, verdict, valid_until) VALUES ('e1', 'medium-holon', 'test', 'Degraded', 'degrade', ?)", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("Failed to insert evidence: %v", err)
	}

	ra := fpf.RoleAssignment{Role: fpf.RoleDecider, SessionID: "test", Context: "test"}
	ev := &fpf.EvidenceStub{URI: l2File, Type: "hypothesis", HolonID: "medium-holon"}

	// With default threshold (0.8), should be blocked
	ok, _ := fsm.CanTransition(fpf.PhaseOperation, ra, ev)
	if ok {
		t.Errorf("Expected transition to be BLOCKED with default threshold 0.8")
	}

	// Lower the threshold to 0.4
	fsm.State.AssuranceThreshold = 0.4

	ok, msg := fsm.CanTransition(fpf.PhaseOperation, ra, ev)
	if !ok {
		t.Errorf("Expected transition to be ALLOWED with lowered threshold 0.4, got: %s", msg)
	}
}

func TestEvidenceDecay_PenalizesExpired(t *testing.T) {
	fsm, database, _ := setupAssuranceTestEnv(t)
	rawDB := database.GetRawDB()

	// Create holon with expired evidence
	_, err := rawDB.Exec("INSERT INTO holons (id, type, layer, title, content, context_id) VALUES ('decay-holon', 'hypothesis', 'L2', 'Decay', 'Content', 'ctx')")
	if err != nil {
		t.Fatalf("Failed to insert holon: %v", err)
	}

	// Insert expired evidence (valid_until in the past)
	expired := time.Now().Add(-24 * time.Hour)
	_, err = rawDB.Exec("INSERT INTO evidence (id, holon_id, type, content, verdict, valid_until) VALUES ('e1', 'decay-holon', 'test', 'Old test', 'pass', ?)", expired)
	if err != nil {
		t.Fatalf("Failed to insert evidence: %v", err)
	}

	calc := assurance.New(fsm.DB)
	report, err := calc.CalculateReliability(context.Background(), "decay-holon")
	if err != nil {
		t.Fatalf("CalculateReliability failed: %v", err)
	}

	// Expired evidence should be penalized to 0.1
	if report.FinalScore != 0.1 {
		t.Errorf("Expected score 0.1 due to decay, got %f", report.FinalScore)
	}

	// Check that decay was noted in factors
	hasDecayFactor := false
	for _, f := range report.Factors {
		if f == "Evidence expired (Decay applied)" {
			hasDecayFactor = true
			break
		}
	}
	if !hasDecayFactor {
		t.Errorf("Expected 'Evidence expired' factor in report, got: %v", report.Factors)
	}
}

func TestAuditVisualization_ReturnsTree(t *testing.T) {
	_, database, tempDir := setupAssuranceTestEnv(t)
	rawDB := database.GetRawDB()

	// Create holon hierarchy: Parent -> Child
	_, _ = rawDB.Exec("INSERT INTO holons (id, type, layer, title, content, context_id) VALUES ('parent', 'hypothesis', 'L2', 'Parent', 'Content', 'ctx')")
	_, _ = rawDB.Exec("INSERT INTO holons (id, type, layer, title, content, context_id) VALUES ('child', 'hypothesis', 'L2', 'Child', 'Content', 'ctx')")

	// Add passing evidence
	future := time.Now().Add(24 * time.Hour)
	_, _ = rawDB.Exec("INSERT INTO evidence (id, holon_id, type, content, verdict, valid_until) VALUES ('e1', 'parent', 'test', 'Pass', 'pass', ?)", future)
	_, _ = rawDB.Exec("INSERT INTO evidence (id, holon_id, type, content, verdict, valid_until) VALUES ('e2', 'child', 'test', 'Pass', 'pass', ?)", future)

	// Create componentOf relation: child is component of parent
	_, _ = rawDB.Exec("INSERT INTO relations (source_id, target_id, relation_type, congruence_level) VALUES ('child', 'parent', 'componentOf', 3)")

	// Create tools and call VisualizeAudit
	stateFile := filepath.Join(tempDir, ".quint", "state.json")
	fsm, _ := fpf.LoadState(stateFile, rawDB)
	tools := fpf.NewTools(fsm, tempDir, database)

	tree, err := tools.VisualizeAudit("parent")
	if err != nil {
		t.Fatalf("VisualizeAudit failed: %v", err)
	}

	// Should contain parent holon info
	if tree == "" {
		t.Errorf("Expected non-empty audit tree")
	}

	// Should contain the parent ID
	if !strings.Contains(tree, "parent") {
		t.Errorf("Expected tree to contain 'parent', got: %s", tree)
	}

	t.Logf("Audit tree:\n%s", tree)
}
