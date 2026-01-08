package fpf_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/quint-code/assurance"
	"github.com/m0n0x41d/quint-code/db"
	"github.com/m0n0x41d/quint-code/internal/fpf"
)

func setupAssuranceTestEnv(t *testing.T) (*fpf.FSM, *db.Store, string) {
	tempDir := t.TempDir()
	quintDir := filepath.Join(tempDir, ".quint")
	if err := os.MkdirAll(quintDir, 0755); err != nil {
		t.Fatalf("Failed to create .quint directory: %v", err)
	}

	dbPath := filepath.Join(quintDir, "quint.db")
	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}

	rawDB := database.GetRawDB()
	_, err = rawDB.Exec("INSERT INTO holons (id, type, layer, title, content, context_id) VALUES ('drr-setup', 'decision', 'DRR', 'Setup', 'Content', 'default')")
	if err != nil {
		t.Fatalf("Failed to insert DRR holon for setup: %v", err)
	}

	fsm := &fpf.FSM{
		State: fpf.State{},
		DB:    rawDB,
	}

	return fsm, database, tempDir
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
	fsm, _ := fpf.LoadState("default", rawDB)
	tools := fpf.NewTools(fsm, tempDir, database)
	ctx := context.Background()

	tree, err := tools.VisualizeAudit(ctx, "parent")
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
