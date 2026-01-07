package fpf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m0n0x41d/quint-code/db"
)

func (t *Tools) computeFileHash(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return "_missing_"
	}
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:8])
}

// Helper to create a dummy Tools instance for testing
func setupTools(t *testing.T) (*Tools, *FSM, string) {
	tempDir := t.TempDir()
	quintDir := filepath.Join(tempDir, ".quint")
	if err := os.MkdirAll(quintDir, 0755); err != nil { // Ensure .quint exists
		t.Fatalf("Failed to create .quint directory: %v", err)
	}

	// Create a dummy DB file
	dbPath := filepath.Join(quintDir, "quint.db")
	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}

	fsm := &FSM{State: State{}, DB: database.GetRawDB()} // Initial FSM state with DB

	tools := NewTools(fsm, tempDir, database)

	// Initialize the project structure for tools to operate
	err = tools.InitProject()
	if err != nil {
		t.Fatalf("Failed to initialize project: %v", err)
	}

	return tools, fsm, tempDir
}

func TestSlugify(t *testing.T) {

	tools, _, _ := setupTools(t)
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Another_Test-Case", "another-test-case"},
		{"123 FPF Hypo!", "123-fpf-hypo"},
		{"  leading and trailing   ", "leading-and-trailing"},
		{"-dash-start-and-end-", "dash-start-and-end"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Input:%s", tt.input), func(t *testing.T) {
			result := tools.Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("slugify(%q) got %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestInitProject(t *testing.T) {
	_, _, tempDir := setupTools(t) // setupTools already calls InitProject

	// v5.0.0: hypotheses are DB-only, no knowledge/ directories
	expectedDirs := []string{
		"evidence", "decisions", "sessions", "agents",
	}

	for _, d := range expectedDirs {
		path := filepath.Join(tempDir, ".quint", d)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", path)
		}
		gitkeepPath := filepath.Join(path, ".gitkeep")
		if _, err := os.Stat(gitkeepPath); os.IsNotExist(err) {
			t.Errorf(".gitkeep file in %s was not created", path)
		}
	}

	// Verify knowledge directories are NOT created
	knowledgeDirs := []string{"knowledge/L0", "knowledge/L1", "knowledge/L2", "knowledge/invalid"}
	for _, d := range knowledgeDirs {
		path := filepath.Join(tempDir, ".quint", d)
		if _, err := os.Stat(path); err == nil {
			t.Errorf("Directory %s should not exist (DB-only hypotheses)", path)
		}
	}
}

func TestProposeHypothesis(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create decision context first (required since v5.0.0)
	dcID := "dc-test-context"
	err := tools.DB.CreateHolon(ctx, dcID, "decision_context", "system", "L0", "Test Context", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	title := "My First Hypothesis"
	content := "This is the content of my hypothesis."
	scope := "global"
	kind := "system"
	rationale := "This is the rationale."

	holonID, err := tools.ProposeHypothesis(title, content, scope, kind, rationale, dcID, nil, 3)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	// v5.0.0: returns holon ID, not file path
	expectedID := "my-first-hypothesis"
	if holonID != expectedID {
		t.Errorf("Returned holon ID %q, expected %q", holonID, expectedID)
	}

	// Verify hypothesis exists in DB with correct attributes
	holon, err := tools.DB.GetHolon(ctx, expectedID)
	if err != nil {
		t.Fatalf("Failed to get holon from DB: %v", err)
	}
	if holon.Layer != "L0" {
		t.Errorf("Expected layer L0, got %s", holon.Layer)
	}
	if holon.Kind.String != kind {
		t.Errorf("Expected kind %s, got %s", kind, holon.Kind.String)
	}
	if holon.Scope.String != scope {
		t.Errorf("Expected scope %s, got %s", scope, holon.Scope.String)
	}
	if holon.Title != title {
		t.Errorf("Expected title %q, got %q", title, holon.Title)
	}
	if !strings.Contains(holon.Content, content) {
		t.Errorf("Content should contain %q", content)
	}
	if !strings.Contains(holon.Content, "## Rationale") {
		t.Errorf("Content should contain rationale section")
	}
}

func TestManageEvidence(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	tests := []struct {
		name              string
		operation         string // "verification" or "validation"
		srcLevel          string // L0 for verification, L1 for validation
		targetID          string
		evidenceType      string
		content           string
		verdict           string
		assuranceLevel    string
		expectedMove      bool
		expectedDestLevel string
		expectErr         bool
	}{
		{"VerificationPass", "verification", "L0", "test-hypo-pass", "logic", "Logic check passed.", "PASS", "L1", true, "L1", false},
		{"VerificationFail", "verification", "L0", "test-hypo-fail", "logic", "Logic check failed.", "FAIL", "L1", true, "invalid", false},
		{"VerificationRefine", "verification", "L0", "test-hypo-refine", "logic", "Needs more refinement.", "REFINE", "L1", true, "invalid", false},
		{"ValidationPass", "validation", "L1", "hypo-L1-pass", "empirical", "Experiment passed.", "PASS", "L2", true, "L2", false},
		{"ValidationFail", "validation", "L1", "hypo-L1-fail", "empirical", "Experiment failed.", "FAIL", "L2", true, "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// v5.0.0: hypotheses are DB-only, no file creation
			if tt.expectedMove {
				if err := tools.DB.CreateHolon(ctx, tt.targetID, "hypothesis", "system", tt.srcLevel, "Test "+tt.targetID, "Content", "default", "", ""); err != nil {
					t.Fatalf("Failed to create holon in DB: %v", err)
				}
			}

			evidencePath, err := tools.ManageEvidence(tt.operation, "add", tt.targetID, tt.evidenceType, tt.content, tt.verdict, tt.assuranceLevel, "file://carrier", "2025-12-31")

			if (err != nil) != tt.expectErr {
				t.Errorf("ManageEvidence() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if tt.expectErr {
				return
			}

			// Verify evidence file creation (evidence still goes to files)
			if _, err := os.Stat(evidencePath); os.IsNotExist(err) {
				t.Errorf("Evidence file was not created at %s", evidencePath)
			}

			// Verify hypothesis layer change in DB (not file move)
			if tt.expectedMove {
				holon, err := tools.DB.GetHolon(ctx, tt.targetID)
				if err != nil {
					t.Fatalf("Failed to get holon from DB: %v", err)
				}
				if holon.Layer != tt.expectedDestLevel {
					t.Errorf("Hypothesis %s layer = %s, want %s", tt.targetID, holon.Layer, tt.expectedDestLevel)
				}
			}
		})
	}
}

func TestRefineLoopback(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	// Create decision context first (required for parent hypothesis)
	dcID := "dc-refine-test"
	if err := tools.DB.CreateHolon(ctx, dcID, "decision_context", "system", "L0", "Refine Test", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	// v5.0.0: hypotheses are DB-only
	parentID := "parent-hypo"
	if err := tools.DB.CreateHolon(ctx, parentID, "hypothesis", "system", "L1", "Parent Hypothesis", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create parent holon in DB: %v", err)
	}

	// Link parent to decision context
	if err := tools.DB.CreateRelation(ctx, parentID, "memberOf", dcID, 3); err != nil {
		t.Fatalf("Failed to link parent to decision context: %v", err)
	}

	insight := "New insight from failure"
	newTitle := "Refined Child Hypothesis"
	newContent := "This is the refined content."
	scope := "system"

	childID, err := tools.RefineLoopback("L1", parentID, insight, newTitle, newContent, scope)
	if err != nil {
		t.Fatalf("RefineLoopback failed: %v", err)
	}

	// Verify parent moved to invalid in DB
	parentHolon, err := tools.DB.GetHolon(ctx, parentID)
	if err != nil {
		t.Fatalf("Failed to get parent holon: %v", err)
	}
	if parentHolon.Layer != "invalid" {
		t.Errorf("Parent hypothesis layer = %s, want invalid", parentHolon.Layer)
	}

	// Verify child created in L0 (returns holon ID, not path)
	expectedChildID := "refined-child-hypothesis"
	if childID != expectedChildID {
		t.Errorf("Returned child ID %q, expected %q", childID, expectedChildID)
	}
	childHolon, err := tools.DB.GetHolon(ctx, expectedChildID)
	if err != nil {
		t.Fatalf("Failed to get child holon: %v", err)
	}
	if childHolon.Layer != "L0" {
		t.Errorf("Child hypothesis layer = %s, want L0", childHolon.Layer)
	}

	// Verify log file created (sessions still use files)
	sessionDir := filepath.Join(tempDir, ".quint", "sessions")
	matches, err := filepath.Glob(filepath.Join(sessionDir, "loopback-*.md"))
	if err != nil || len(matches) == 0 {
		t.Errorf("Loopback log file was not created")
	}
}

func TestFinalizeDecision(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	// v5.0.0: hypotheses are DB-only
	winnerID := "final-winner"
	if err := tools.DB.CreateHolon(ctx, winnerID, "hypothesis", "system", "L2", "Final Winner", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create winner holon in DB: %v", err)
	}

	title := "Final Project Decision"
	content := "This is the DRR content for the decision."

	drrPath, err := tools.FinalizeDecision(title, winnerID, nil, "Context", content, "Rationale", "Consequences", "Characteristics", "", true)
	if err != nil {
		t.Fatalf("FinalizeDecision failed: %v", err)
	}

	// Verify DRR file creation (decisions still go to files)
	drrPattern := filepath.Join(tempDir, ".quint", "decisions", fmt.Sprintf("DRR-*-%s.md", tools.Slugify(title)))
	matches, err := filepath.Glob(drrPattern)
	if err != nil {
		t.Fatalf("Failed to glob for DRR file: %v", err)
	}
	if len(matches) == 0 {
		t.Errorf("DRR file was not created with expected pattern")
	}
	// Check if the returned path is one of the matched paths
	found := false
	for _, match := range matches {
		if match == drrPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Returned DRR path %q does not match any expected pattern %q", drrPath, drrPattern)
	}

	// Verify DRR exists in DB
	dateStr := drrPath[len(filepath.Join(tempDir, ".quint", "decisions", "DRR-")):len(filepath.Join(tempDir, ".quint", "decisions", "DRR-"))+10]
	drrID := fmt.Sprintf("DRR-%s-%s", dateStr, tools.Slugify(title))
	drrHolon, err := tools.DB.GetHolon(ctx, drrID)
	if err != nil {
		t.Fatalf("DRR not found in DB: %v", err)
	}
	if drrHolon.Layer != "DRR" {
		t.Errorf("DRR layer = %s, want DRR", drrHolon.Layer)
	}
}

func TestFinalizeDecision_BlockedWhenDecisionContextClosed(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a decision context
	dcID := "test-decision-context"
	if err := tools.DB.CreateHolon(ctx, dcID, "decision_context", "system", "L0", "Test Decision Context", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	// v5.0.0: hypotheses are DB-only
	hypID := "hyp-in-closed-dc"
	if err := tools.DB.CreateHolon(ctx, hypID, "hypothesis", "system", "L2", "Hypothesis in Closed DC", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create hypothesis: %v", err)
	}

	// Create memberOf relation (hypothesis belongs to decision context)
	if _, err := tools.DB.GetRawDB().ExecContext(ctx,
		`INSERT INTO relations (source_id, relation_type, target_id, congruence_level) VALUES (?, 'memberOf', ?, 3)`,
		hypID, dcID); err != nil {
		t.Fatalf("Failed to create memberOf relation: %v", err)
	}

	// Create a DRR that closes the decision context
	closingDRRID := "existing-drr"
	if err := tools.DB.CreateHolon(ctx, closingDRRID, "DRR", "", "DRR", "Existing DRR", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create closing DRR: %v", err)
	}
	if _, err := tools.DB.GetRawDB().ExecContext(ctx,
		`INSERT INTO relations (source_id, relation_type, target_id, congruence_level) VALUES (?, 'closes', ?, 3)`,
		closingDRRID, dcID); err != nil {
		t.Fatalf("Failed to create closes relation: %v", err)
	}

	// Try to create a new DRR with the hypothesis - should be BLOCKED
	_, err := tools.FinalizeDecision("New Decision", hypID, nil, "Context", "Decision", "Rationale", "Consequences", "", "", true)
	if err == nil {
		t.Fatal("Expected FinalizeDecision to return BLOCKED error, got nil")
	}
	if !strings.Contains(err.Error(), "BLOCKED") {
		t.Errorf("Expected error to contain 'BLOCKED', got: %v", err)
	}
	if !strings.Contains(err.Error(), dcID) {
		t.Errorf("Expected error to mention decision_context ID %q, got: %v", dcID, err)
	}
	if !strings.Contains(err.Error(), closingDRRID) {
		t.Errorf("Expected error to mention conflicting DRR ID %q, got: %v", closingDRRID, err)
	}
}

func TestFinalizeDecision_BlockedWhenHypothesisInOpenDRR(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// v5.0.0: hypotheses are DB-only
	hypID := "hyp-already-in-drr"
	if err := tools.DB.CreateHolon(ctx, hypID, "hypothesis", "system", "L2", "Already Used Hypothesis", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create hypothesis: %v", err)
	}

	// Create an existing open DRR that selects this hypothesis
	existingDRRID := "existing-open-drr"
	if err := tools.DB.CreateHolon(ctx, existingDRRID, "DRR", "", "DRR", "Existing Open DRR", "Content without status", "default", "", ""); err != nil {
		t.Fatalf("Failed to create existing DRR: %v", err)
	}
	if _, err := tools.DB.GetRawDB().ExecContext(ctx,
		`INSERT INTO relations (source_id, relation_type, target_id, congruence_level) VALUES (?, 'selects', ?, 3)`,
		existingDRRID, hypID); err != nil {
		t.Fatalf("Failed to create selects relation: %v", err)
	}

	// Try to create a new DRR with the same hypothesis - should be BLOCKED
	_, err := tools.FinalizeDecision("Another Decision", hypID, nil, "Context", "Decision", "Rationale", "Consequences", "", "", true)
	if err == nil {
		t.Fatal("Expected FinalizeDecision to return BLOCKED error, got nil")
	}
	if !strings.Contains(err.Error(), "BLOCKED") {
		t.Errorf("Expected error to contain 'BLOCKED', got: %v", err)
	}
	if !strings.Contains(err.Error(), hypID) {
		t.Errorf("Expected error to mention hypothesis ID %q, got: %v", hypID, err)
	}
	if !strings.Contains(err.Error(), existingDRRID) {
		t.Errorf("Expected error to mention conflicting DRR ID %q, got: %v", existingDRRID, err)
	}
}

func TestFinalizeDecision_AllowsWhenDRRResolved(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// v5.0.0: hypotheses are DB-only
	hypID := "hyp-in-resolved-drr"
	if err := tools.DB.CreateHolon(ctx, hypID, "hypothesis", "system", "L2", "Hypothesis in Resolved DRR", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create hypothesis: %v", err)
	}

	// Create an existing DRR that selects this hypothesis
	resolvedDRRID := "resolved-drr"
	if err := tools.DB.CreateHolon(ctx, resolvedDRRID, "DRR", "", "DRR", "Resolved DRR", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create resolved DRR: %v", err)
	}
	if _, err := tools.DB.GetRawDB().ExecContext(ctx,
		`INSERT INTO relations (source_id, relation_type, target_id, congruence_level) VALUES (?, 'selects', ?, 3)`,
		resolvedDRRID, hypID); err != nil {
		t.Fatalf("Failed to create selects relation: %v", err)
	}

	// Mark the DRR as resolved by adding implementation evidence
	if err := tools.DB.AddEvidence(ctx, "ev-impl", resolvedDRRID, "implementation", "commit:abc123", "PASS", "", "commit:abc123", "", "", ""); err != nil {
		t.Fatalf("Failed to add implementation evidence: %v", err)
	}

	// Now creating a new DRR with the same hypothesis should succeed (no blocking)
	_, err := tools.FinalizeDecision("New Decision After Resolution", hypID, nil, "Context", "Decision", "Rationale", "Consequences", "", "", true)
	if err != nil {
		t.Fatalf("Expected FinalizeDecision to succeed for resolved DRR, got error: %v", err)
	}
}

func TestVerifyHypothesis(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// v5.0.0: hypotheses are DB-only
	hypoID := "test-verify-hypo"
	if err := tools.DB.CreateHolon(ctx, hypoID, "hypothesis", "system", "L0", "Test Verify", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create holon in DB: %v", err)
	}

	passJSON := `{
		"type_check": {"verdict": "PASS", "evidence": ["test-ref"], "reasoning": "Type is correct"},
		"constraint_check": {"verdict": "PASS", "evidence": ["constraint-ref"], "reasoning": "Constraints satisfied"},
		"logic_check": {"verdict": "PASS", "evidence": ["logic-ref"], "reasoning": "Logic is sound"}
	}`
	msg, err := tools.VerifyHypothesis(hypoID, passJSON, "PASS", "")
	if err != nil {
		t.Errorf("VerifyHypothesis(PASS) failed: %v", err)
	}
	if !strings.Contains(msg, "promoted to L1") {
		t.Errorf("Expected message to contain 'promoted to L1', got %q", msg)
	}
	// Verify layer change in DB
	holon, err := tools.DB.GetHolon(ctx, hypoID)
	if err != nil {
		t.Fatalf("Failed to get holon: %v", err)
	}
	if holon.Layer != "L1" {
		t.Errorf("Hypothesis layer = %s, want L1", holon.Layer)
	}

	hypoID2 := "test-fail-hypo"
	if err := tools.DB.CreateHolon(ctx, hypoID2, "hypothesis", "system", "L0", "Test Fail", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create holon in DB: %v", err)
	}

	failJSON := `{
		"type_check": {"verdict": "PASS", "evidence": ["test-ref"], "reasoning": "Type ok"},
		"constraint_check": {"verdict": "FAIL", "evidence": ["constraint-ref"], "reasoning": "Constraint violated"},
		"logic_check": {"verdict": "PASS", "evidence": ["logic-ref"], "reasoning": "Logic ok"}
	}`
	msg, err = tools.VerifyHypothesis(hypoID2, failJSON, "FAIL", "")
	if err != nil {
		t.Errorf("VerifyHypothesis(FAIL) failed: %v", err)
	}
	if !strings.Contains(msg, "VERIFICATION FAILED") {
		t.Errorf("Expected message to contain 'VERIFICATION FAILED', got %q", msg)
	}
	// Verify layer change in DB
	holon2, err := tools.DB.GetHolon(ctx, hypoID2)
	if err != nil {
		t.Fatalf("Failed to get holon: %v", err)
	}
	if holon2.Layer != "invalid" {
		t.Errorf("Hypothesis layer = %s, want invalid", holon2.Layer)
	}
}

func TestVerifyHypothesis_ValidationErrors(t *testing.T) {
	tools, _, _ := setupTools(t)

	tests := []struct {
		name        string
		json        string
		verdict     string
		errContains string
	}{
		{
			name:        "invalid JSON",
			json:        `{not valid json}`,
			verdict:     "PASS",
			errContains: "invalid checks_json",
		},
		{
			name:        "missing type_check verdict",
			json:        `{"type_check": {"evidence": ["ref"], "reasoning": "why"}, "constraint_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}, "logic_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}}`,
			verdict:     "PASS",
			errContains: "type_check: missing verdict",
		},
		{
			name:        "invalid verdict value",
			json:        `{"type_check": {"verdict": "MAYBE", "evidence": ["ref"], "reasoning": "why"}, "constraint_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}, "logic_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}}`,
			verdict:     "PASS",
			errContains: "type_check: verdict must be PASS or FAIL",
		},
		{
			name:        "missing evidence",
			json:        `{"type_check": {"verdict": "PASS", "evidence": [], "reasoning": "why"}, "constraint_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}, "logic_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}}`,
			verdict:     "PASS",
			errContains: "type_check: verdict requires at least one evidence reference",
		},
		{
			name:        "missing reasoning",
			json:        `{"type_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": ""}, "constraint_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}, "logic_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}}`,
			verdict:     "PASS",
			errContains: "type_check: missing reasoning",
		},
		{
			name:        "invalid overall_verdict",
			json:        `{"type_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}, "constraint_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}, "logic_check": {"verdict": "PASS", "evidence": ["ref"], "reasoning": "why"}}`,
			verdict:     "UNKNOWN",
			errContains: "overall_verdict must be PASS or FAIL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tools.VerifyHypothesis("any-id", tt.json, tt.verdict, "")
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.errContains)
				return
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Expected error containing %q, got %q", tt.errContains, err.Error())
			}
		})
	}
}

func TestValidateHypothesis(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	hypoID := "test-validate-hypo"
	if err := tools.DB.CreateHolon(ctx, hypoID, "hypothesis", "system", "L1", "Test Validate", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create holon in DB: %v", err)
	}
	hypoPath := filepath.Join(tempDir, ".quint", "knowledge", "L1", hypoID+".md")
	if err := os.MkdirAll(filepath.Dir(hypoPath), 0755); err != nil {
		t.Fatalf("Failed to create L1 dir: %v", err)
	}
	if err := os.WriteFile(hypoPath, []byte("L1 content"), 0644); err != nil {
		t.Fatalf("Failed to create L1 hypothesis file: %v", err)
	}

	passResult := "All tests passed successfully - test output: OK"
	msg, err := tools.ValidateHypothesis(hypoID, "internal", passResult, "PASS", "")
	if err != nil {
		t.Errorf("ValidateHypothesis(PASS) failed: %v", err)
	}
	if !strings.Contains(msg, "validated (L2)") {
		t.Errorf("Expected message to contain 'validated (L2)', got %q", msg)
	}

	hypoID2 := "test-validate-fail"
	if err := tools.DB.CreateHolon(ctx, hypoID2, "hypothesis", "system", "L1", "Test Validate Fail", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create holon in DB: %v", err)
	}
	hypoPath2 := filepath.Join(tempDir, ".quint", "knowledge", "L1", hypoID2+".md")
	if err := os.WriteFile(hypoPath2, []byte("L1 content"), 0644); err != nil {
		t.Fatalf("Failed to create L1 hypothesis file: %v", err)
	}

	failResult := "Integration test failed due to connectivity issues - error: connection refused"
	msg, err = tools.ValidateHypothesis(hypoID2, "internal", failResult, "FAIL", "")
	if err != nil {
		t.Errorf("ValidateHypothesis(FAIL) failed: %v", err)
	}
	if !strings.Contains(msg, "VALIDATION FAILED") {
		t.Errorf("Expected message to contain 'VALIDATION FAILED', got %q", msg)
	}
}

func TestValidateHypothesis_ValidationErrors(t *testing.T) {
	tools, _, _ := setupTools(t)

	tests := []struct {
		name        string
		result      string
		verdict     string
		errContains string
	}{
		{
			name:        "empty result",
			result:      "",
			verdict:     "PASS",
			errContains: "result is required",
		},
		{
			name:        "invalid overall_verdict",
			result:      "some test result",
			verdict:     "MAYBE",
			errContains: "overall_verdict must be PASS or FAIL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tools.ValidateHypothesis("any-id", "internal", tt.result, tt.verdict, "")
			if err == nil {
				t.Errorf("Expected error containing %q, got nil", tt.errContains)
				return
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Expected error containing %q, got %q", tt.errContains, err.Error())
			}
		})
	}
}

func TestCheckDuplicateHypothesis(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	if err := tools.DB.CreateHolon(ctx, "failed-hypo-1", "hypothesis", "system", "invalid", "Duplicate Title", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create invalid holon: %v", err)
	}

	if err := tools.DB.CreateHolon(ctx, "new-hypo", "hypothesis", "system", "L0", "Duplicate Title", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create L0 holon: %v", err)
	}

	warning := tools.checkDuplicateHypothesis("new-hypo")
	if warning == "" {
		t.Error("Expected duplicate warning, got empty string")
	}
	if !strings.Contains(warning, "failed-hypo-1") {
		t.Errorf("Expected warning to contain 'failed-hypo-1', got %q", warning)
	}

	if err := tools.DB.CreateHolon(ctx, "unique-hypo", "hypothesis", "system", "L0", "Unique Title", "Content", "default", "", ""); err != nil {
		t.Fatalf("Failed to create unique holon: %v", err)
	}

	warning = tools.checkDuplicateHypothesis("unique-hypo")
	if warning != "" {
		t.Errorf("Expected no warning for unique title, got %q", warning)
	}

	warning = tools.checkDuplicateHypothesis("nonexistent-id")
	if warning != "" {
		t.Errorf("Expected no warning for nonexistent id, got %q", warning)
	}
}

func TestAuditEvidence(t *testing.T) {
	tools, _, _ := setupTools(t)

	hypoID := "audit-hypo"

	msg, err := tools.AuditEvidence(hypoID, "Risk analysis content")
	if err != nil {
		t.Errorf("AuditEvidence failed: %v", err)
	}
	expectedMsg := "Audit recorded for " + hypoID
	if msg != expectedMsg {
		t.Errorf("Expected message %q, got %q", expectedMsg, msg)
	}

	// We could verify DB side effects if we exposed DB in tests more directly,
	// but for now we verify no error and correct return message.
}

func TestCalculateR(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon with evidence
	err := tools.DB.CreateHolon(ctx, "calc-r-test", "hypothesis", "system", "L1", "Test Holon", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add passing evidence
	err = tools.DB.AddEvidence(ctx, "e1", "calc-r-test", "test", "Test passed", "pass", "L1", "test-runner", "", "", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Calculate R
	result, err := tools.CalculateR("calc-r-test")
	if err != nil {
		t.Fatalf("CalculateR failed: %v", err)
	}

	// Verify output contains expected elements
	if !strings.Contains(result, "Reliability Report") {
		t.Errorf("Expected 'Reliability Report' in output, got: %s", result)
	}
	if !strings.Contains(result, "R_eff:") {
		t.Errorf("Expected 'R_eff:' in output, got: %s", result)
	}
	if !strings.Contains(result, "1.00") {
		t.Errorf("Expected R_eff of 1.00 for passing evidence, got: %s", result)
	}
}

func TestCalculateR_WithDecay(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon with expired evidence
	err := tools.DB.CreateHolon(ctx, "decay-r-test", "hypothesis", "system", "L1", "Decay Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add expired evidence (past date)
	err = tools.DB.AddEvidence(ctx, "e-expired", "decay-r-test", "test", "Old test", "pass", "L1", "test-runner", "", "", "2020-01-01")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Calculate R
	result, err := tools.CalculateR("decay-r-test")
	if err != nil {
		t.Fatalf("CalculateR failed: %v", err)
	}

	// Verify decay is mentioned
	if !strings.Contains(result, "Decay") || !strings.Contains(result, "expired") {
		t.Errorf("Expected decay/expired mention in output, got: %s", result)
	}
}

func TestCheckDecay_NoExpired(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon with fresh evidence
	err := tools.DB.CreateHolon(ctx, "fresh-holon", "hypothesis", "system", "L2", "Fresh", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add future-dated evidence
	err = tools.DB.AddEvidence(ctx, "e-fresh", "fresh-holon", "test", "Fresh test", "pass", "L2", "test-runner", "", "", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Check decay (freshness report mode - all empty params)
	result, err := tools.CheckDecay("", "", "", "")
	if err != nil {
		t.Fatalf("CheckDecay failed: %v", err)
	}

	// Should report all fresh
	if !strings.Contains(result, "All holons FRESH") && !strings.Contains(result, "No expired evidence") {
		t.Errorf("Expected fresh holons message, got: %s", result)
	}
}

func TestCheckDecay_WithExpired(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon with expired evidence
	err := tools.DB.CreateHolon(ctx, "stale-holon", "hypothesis", "system", "L2", "Stale Holon", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add expired evidence
	err = tools.DB.AddEvidence(ctx, "e-stale", "stale-holon", "test", "Old test", "pass", "L2", "test-runner", "", "", "2020-01-01")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Check decay (freshness report mode - all empty params)
	result, err := tools.CheckDecay("", "", "", "")
	if err != nil {
		t.Fatalf("CheckDecay failed: %v", err)
	}

	// Should report the expired evidence
	if !strings.Contains(result, "stale-holon") && !strings.Contains(result, "Stale Holon") {
		t.Errorf("Expected stale-holon in output, got: %s", result)
	}
	if !strings.Contains(result, "STALE") && !strings.Contains(result, "EXPIRED") {
		t.Errorf("Expected STALE or EXPIRED in output, got: %s", result)
	}
}

func TestCheckDecay_Deprecate(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// v5.0.0: hypotheses are DB-only
	holonID := "deprecate-test"
	err := tools.DB.CreateHolon(ctx, holonID, "hypothesis", "system", "L2", "Deprecate Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Deprecate (L2 -> L1)
	result, err := tools.CheckDecay(holonID, "", "", "")
	if err != nil {
		t.Fatalf("CheckDecay deprecate failed: %v", err)
	}

	if !strings.Contains(result, "Deprecated") || !strings.Contains(result, "L2 → L1") {
		t.Errorf("Expected deprecation message, got: %s", result)
	}

	// Verify holon moved to L1 in DB
	holon, err := tools.DB.GetHolon(ctx, holonID)
	if err != nil {
		t.Fatalf("Failed to get holon: %v", err)
	}
	if holon.Layer != "L1" {
		t.Errorf("Expected layer L1, got: %s", holon.Layer)
	}
}

func TestCheckDecay_Waive(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create holon with expired evidence
	holonID := "waive-test-holon"
	evidenceID := "waive-test-evidence"
	err := tools.DB.CreateHolon(ctx, holonID, "hypothesis", "system", "L2", "Waive Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	err = tools.DB.AddEvidence(ctx, evidenceID, holonID, "test", "Old test", "pass", "L2", "test-runner", "", "", "2020-01-01")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Verify initially shows as stale
	result, err := tools.CheckDecay("", "", "", "")
	if err != nil {
		t.Fatalf("CheckDecay failed: %v", err)
	}
	if !strings.Contains(result, "STALE") {
		t.Errorf("Expected STALE before waive, got: %s", result)
	}

	// Waive the evidence
	futureDate := "2099-12-31"
	rationale := "Test waiver"
	result, err = tools.CheckDecay("", evidenceID, futureDate, rationale)
	if err != nil {
		t.Fatalf("CheckDecay waive failed: %v", err)
	}

	if !strings.Contains(result, "Waiver recorded") {
		t.Errorf("Expected waiver confirmation, got: %s", result)
	}
	if !strings.Contains(result, evidenceID) {
		t.Errorf("Expected evidence ID in output, got: %s", result)
	}

	// Check that it no longer shows as stale
	result, err = tools.CheckDecay("", "", "", "")
	if err != nil {
		t.Fatalf("CheckDecay report failed: %v", err)
	}

	// Should show waived instead of stale
	if strings.Contains(result, holonID) && strings.Contains(result, "STALE") && !strings.Contains(result, "WAIVED") {
		t.Errorf("Expected waived evidence to not show as STALE, got: %s", result)
	}
}

func TestCheckDecay_WaiveMissingParams(t *testing.T) {
	tools, _, _ := setupTools(t)

	// Waive without until date
	_, err := tools.CheckDecay("", "some-evidence", "", "some rationale")
	if err == nil {
		t.Error("Expected error when waive_until is missing")
	}

	// Waive without rationale
	_, err = tools.CheckDecay("", "some-evidence", "2099-12-31", "")
	if err == nil {
		t.Error("Expected error when rationale is missing")
	}
}

func TestCheckDecay_DeprecateL0Fails(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create L0 holon
	holonID := "l0-deprecate-test"
	err := tools.DB.CreateHolon(ctx, holonID, "hypothesis", "system", "L0", "L0 Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Try to deprecate L0 - should fail
	_, err = tools.CheckDecay(holonID, "", "", "")
	if err == nil {
		t.Error("Expected error when deprecating L0 holon")
	}
	if err != nil && !strings.Contains(err.Error(), "cannot deprecate") {
		t.Errorf("Expected 'cannot deprecate' error, got: %v", err)
	}
}

func TestVisualizeAudit(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon
	err := tools.DB.CreateHolon(ctx, "audit-viz-test", "hypothesis", "system", "L2", "Audit Viz Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add evidence
	err = tools.DB.AddEvidence(ctx, "e-viz", "audit-viz-test", "test", "Test", "pass", "L2", "test-runner", "", "", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Visualize audit
	result, err := tools.VisualizeAudit("audit-viz-test")
	if err != nil {
		t.Fatalf("VisualizeAudit failed: %v", err)
	}

	// Should contain the holon ID and R score
	if !strings.Contains(result, "audit-viz-test") {
		t.Errorf("Expected 'audit-viz-test' in output, got: %s", result)
	}
	if !strings.Contains(result, "R:") {
		t.Errorf("Expected 'R:' score in output, got: %s", result)
	}
}

func TestUnifiedAudit_Basic(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon
	err := tools.DB.CreateHolon(ctx, "unified-audit-test", "hypothesis", "system", "L2", "Unified Audit Test", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Add evidence
	err = tools.DB.AddEvidence(ctx, "e-unified", "unified-audit-test", "test", "Test result", "pass", "L2", "test-runner", "", "", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Run unified audit without risks
	result, err := tools.UnifiedAudit("unified-audit-test", "")
	if err != nil {
		t.Fatalf("UnifiedAudit failed: %v", err)
	}

	// Should contain R_eff header
	if !strings.Contains(result, "R_eff:") {
		t.Errorf("Expected 'R_eff:' in output, got: %s", result)
	}
	// Should contain audit tree
	if !strings.Contains(result, "Assurance Tree") {
		t.Errorf("Expected 'Assurance Tree' in output, got: %s", result)
	}
	// Should NOT contain audit recorded message (no risks provided)
	if strings.Contains(result, "Audit evidence recorded") {
		t.Errorf("Should not have recorded audit when no risks provided")
	}
}

func TestUnifiedAudit_WithRisks(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon
	err := tools.DB.CreateHolon(ctx, "unified-audit-risks", "hypothesis", "system", "L2", "Unified Audit Risks", "Content", "ctx", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Run unified audit with risks
	result, err := tools.UnifiedAudit("unified-audit-risks", "Risk: dependency might fail under high load")
	if err != nil {
		t.Fatalf("UnifiedAudit failed: %v", err)
	}

	// Should contain audit recorded message
	if !strings.Contains(result, "Audit evidence recorded") {
		t.Errorf("Expected 'Audit evidence recorded' in output, got: %s", result)
	}

	// Verify evidence was actually recorded
	ev, err := tools.DB.GetEvidence(ctx, "unified-audit-risks")
	if err != nil {
		t.Fatalf("Failed to get evidence: %v", err)
	}
	found := false
	for _, e := range ev {
		if e.Type == "audit_report" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected audit_report evidence to be recorded")
	}
}

func TestPropose_WithDecisionContext(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// First create a decision context holon (must be type "decision_context")
	err := tools.DB.CreateHolon(ctx, "caching-decision", "decision_context", "episteme", "L0", "Caching Decision", "Content", "default", "backend", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	// Propose hypothesis with decision_context
	_, err = tools.ProposeHypothesis(
		"Use Redis",
		"Use Redis for caching",
		"backend",
		"system",
		`{"approach": "distributed cache"}`,
		"caching-decision", // decision_context
		nil,                // no depends_on
		3,
	)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	// Verify MemberOf relation was created
	rawDB := tools.DB.GetRawDB()
	var count int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE source_id = 'use-redis'
		AND target_id = 'caching-decision'
		AND relation_type = 'memberOf'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query relations: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 MemberOf relation, got %d", count)
	}
}

func TestPropose_WithDependsOn(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create decision context first (required since v5.0.0)
	err := tools.DB.CreateHolon(ctx, "dc-api-gateway", "decision_context", "system", "L0", "API Gateway Decision", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	// Create dependency holons
	err = tools.DB.CreateHolon(ctx, "auth-module", "hypothesis", "system", "L2", "Auth Module", "Content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create auth-module: %v", err)
	}
	err = tools.DB.CreateHolon(ctx, "rate-limiter", "hypothesis", "system", "L2", "Rate Limiter", "Content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create rate-limiter: %v", err)
	}

	// Propose hypothesis with depends_on
	_, err = tools.ProposeHypothesis(
		"API Gateway",
		"Gateway with auth and rate limiting",
		"external traffic",
		"system",
		`{"anomaly": "need unified entry point"}`,
		"dc-api-gateway",                        // decision_context required
		[]string{"auth-module", "rate-limiter"}, // depends_on
		3,                                       // CL3
	)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	// Verify componentOf relations were created
	rawDB := tools.DB.GetRawDB()
	var count int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE target_id = 'api-gateway'
		AND relation_type = 'componentOf'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query relations: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 componentOf relations, got %d", count)
	}
}

func TestPropose_CycleDetection(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create decision context first (required since v5.0.0)
	err := tools.DB.CreateHolon(ctx, "dc-cycle-test", "decision_context", "system", "L0", "Cycle Test", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	// Create holon A
	err = tools.DB.CreateHolon(ctx, "holon-a", "hypothesis", "system", "L1", "Holon A", "Content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create holon-a: %v", err)
	}

	// Create holon B that depends on A
	_, err = tools.ProposeHypothesis("Holon B", "B depends on A", "global", "system", "{}", "dc-cycle-test", []string{"holon-a"}, 3)
	if err != nil {
		t.Fatalf("ProposeHypothesis for B failed: %v", err)
	}

	// Now try to create holon C that would create a cycle: A → B → C → A
	// First add B→C relation manually
	err = tools.DB.CreateRelation(ctx, "holon-b", "componentOf", "holon-c-temp", 3)
	if err != nil {
		// This is okay, C doesn't exist yet
	}

	// Try to make A depend on B (would create cycle since B already depends on A)
	// This should be skipped with a warning, not error
	_, err = tools.ProposeHypothesis("Holon C Cyclic", "C tries to depend on B", "global", "system", "{}", "dc-cycle-test", []string{"holon-b"}, 3)
	// Should NOT error - cycles are skipped with warning
	if err != nil {
		t.Fatalf("ProposeHypothesis should not error on cycle, got: %v", err)
	}

	// The relation should still be created since holon-c-cyclic → holon-b is not itself a cycle
	// (holon-b → holon-a exists, but holon-a doesn't depend on holon-c-cyclic)
	rawDB := tools.DB.GetRawDB()
	var count int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE target_id = 'holon-c-cyclic'
		AND source_id = 'holon-b'
		AND relation_type = 'componentOf'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query relations: %v", err)
	}
	// This should exist since it's not actually a cycle
	if count != 1 {
		t.Errorf("Expected 1 componentOf relation for non-cyclic dependency, got %d", count)
	}
}

func TestPropose_InvalidDependency(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create decision context first (required since v5.0.0)
	err := tools.DB.CreateHolon(ctx, "dc-invalid-dep", "decision_context", "system", "L0", "Invalid Dep Test", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	// Propose hypothesis with non-existent dependency
	_, err = tools.ProposeHypothesis(
		"Orphan Hypo",
		"Depends on non-existent holon",
		"global",
		"system",
		"{}",
		"dc-invalid-dep",
		[]string{"does-not-exist", "also-missing"}, // These don't exist
		3,
	)
	// Should NOT error - invalid deps are skipped with warning
	if err != nil {
		t.Fatalf("ProposeHypothesis should not error on invalid deps, got: %v", err)
	}

	// Verify no relations were created
	rawDB := tools.DB.GetRawDB()
	var count int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE target_id = 'orphan-hypo'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query relations: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 relations for invalid deps, got %d", count)
	}
}

func TestPropose_KindDeterminesRelation(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create decision context first (required since v5.0.0)
	err := tools.DB.CreateHolon(ctx, "dc-kind-test", "decision_context", "system", "L0", "Kind Test", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	// Create a dependency holon
	err = tools.DB.CreateHolon(ctx, "base-claim", "hypothesis", "episteme", "L2", "Base Claim", "Content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create base-claim: %v", err)
	}

	// Propose system hypothesis - should create componentOf
	_, err = tools.ProposeHypothesis("System Hypo", "A system thing", "global", "system", "{}", "dc-kind-test", []string{"base-claim"}, 3)
	if err != nil {
		t.Fatalf("ProposeHypothesis for system failed: %v", err)
	}

	// Propose episteme hypothesis - should create constituentOf
	_, err = tools.ProposeHypothesis("Episteme Hypo", "An epistemic claim", "global", "episteme", "{}", "dc-kind-test", []string{"base-claim"}, 3)
	if err != nil {
		t.Fatalf("ProposeHypothesis for episteme failed: %v", err)
	}

	rawDB := tools.DB.GetRawDB()

	// Check system → componentOf
	var componentCount int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE target_id = 'system-hypo'
		AND relation_type = 'componentOf'
	`).Scan(&componentCount)
	if err != nil {
		t.Fatalf("Failed to query componentOf: %v", err)
	}
	if componentCount != 1 {
		t.Errorf("Expected 1 componentOf for system kind, got %d", componentCount)
	}

	// Check episteme → constituentOf
	var constituentCount int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations
		WHERE target_id = 'episteme-hypo'
		AND relation_type = 'constituentOf'
	`).Scan(&constituentCount)
	if err != nil {
		t.Fatalf("Failed to query constituentOf: %v", err)
	}
	if constituentCount != 1 {
		t.Errorf("Expected 1 constituentOf for episteme kind, got %d", constituentCount)
	}
}

func TestWLNK_MemberOf_NoPropagation(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create decision context with low R (failing evidence)
	// v5.0.0: must use type "decision_context" for decision_context param validation
	err := tools.DB.CreateHolon(ctx, "bad-decision", "decision_context", "episteme", "L1", "Bad Decision", "Content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create bad-decision: %v", err)
	}
	err = tools.DB.AddEvidence(ctx, "e-bad", "bad-decision", "test", "Failed", "fail", "L1", "test", "", "", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add failing evidence: %v", err)
	}

	// Create good hypothesis that is member of bad decision
	_, err = tools.ProposeHypothesis(
		"Good Member",
		"A good hypothesis",
		"global",
		"system",
		"{}",
		"bad-decision", // MemberOf the bad decision
		nil,
		3,
	)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	// Add passing evidence to good-member
	err = tools.DB.AddEvidence(ctx, "e-good", "good-member", "test", "Passed", "pass", "L1", "test", "", "", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add passing evidence: %v", err)
	}

	// Calculate R for good-member
	result, err := tools.CalculateR("good-member")
	if err != nil {
		t.Fatalf("CalculateR failed: %v", err)
	}

	// MemberOf should NOT propagate R - good-member should have R=1.00
	// despite bad-decision having R=0.00
	if !strings.Contains(result, "1.00") {
		t.Errorf("Expected R=1.00 (MemberOf should not propagate), got: %s", result)
	}
}

func TestFormatVocabulary(t *testing.T) {
	input := "Channel: A Telegram channel or chat being monitored (has telegram_id, name, kind, is_active status). Message: A post from a monitored channel (has id, content, author_id, telegram_url, processing state). Result[T,E]: Either Ok(value) or Err(error) - functional error handling pattern."

	result := formatVocabulary(input)

	// Should have separate lines for each term
	if !strings.Contains(result, "- **Channel**:") {
		t.Errorf("Expected '- **Channel**:', got: %s", result)
	}
	if !strings.Contains(result, "- **Message**:") {
		t.Errorf("Expected '- **Message**:', got: %s", result)
	}
	if !strings.Contains(result, "- **Result[T,E]**:") {
		t.Errorf("Expected '- **Result[T,E]**:', got: %s", result)
	}

	// Should have newlines between entries
	lines := strings.Split(result, "\n")
	if len(lines) < 3 {
		t.Errorf("Expected at least 3 lines, got %d: %s", len(lines), result)
	}
}

func TestFormatInvariants(t *testing.T) {
	input := "1. Python 3.12+ with strict mypy type checking. 2. DuckDB as the only database (file-based, path from config.yaml). 3. Telethon for Telegram API interaction (requires session file)."

	result := formatInvariants(input)

	// Should have separate lines for each numbered item
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %s", len(lines), result)
	}

	if !strings.HasPrefix(lines[0], "1. Python") {
		t.Errorf("Expected line 1 to start with '1. Python', got: %s", lines[0])
	}
	if !strings.HasPrefix(lines[1], "2. DuckDB") {
		t.Errorf("Expected line 2 to start with '2. DuckDB', got: %s", lines[1])
	}
	if !strings.HasPrefix(lines[2], "3. Telethon") {
		t.Errorf("Expected line 3 to start with '3. Telethon', got: %s", lines[2])
	}
}

func TestManageEvidence_ValidUntilNullByDefault(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// v5.1.0: valid_until defaults to NULL (perpetual) for code evidence
	// FPF B.3.4: Code evidence validity is tied to code changes, not time.
	hypoID := "valid-until-test-hypo"
	err := tools.DB.CreateHolon(ctx, hypoID, "hypothesis", "system", "L1", "Test Hypothesis", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Call ManageEvidence with EMPTY validUntil - should remain NULL
	_, err = tools.ManageEvidence("validation", "add", hypoID, "internal", "Test passed", "pass", "L2", "test-runner", "")
	if err != nil {
		t.Fatalf("ManageEvidence failed: %v", err)
	}

	// Query evidence from DB
	evidence, err := tools.DB.GetEvidence(ctx, hypoID)
	if err != nil {
		t.Fatalf("GetEvidence failed: %v", err)
	}

	if len(evidence) == 0 {
		t.Fatal("No evidence found in DB")
	}

	e := evidence[0]
	t.Logf("Evidence ID: %s", e.ID)
	t.Logf("ValidUntil.Valid: %v", e.ValidUntil.Valid)

	// v5.1.0: valid_until SHOULD be NULL (perpetual evidence)
	if e.ValidUntil.Valid {
		t.Errorf("valid_until should be NULL (perpetual), got: %v", e.ValidUntil.Time)
	} else {
		t.Log("OK: valid_until is NULL (perpetual evidence per FPF B.3.4)")
	}
}

func TestManageEvidence_ValidUntilExplicit(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// When valid_until is explicitly set, it should be honored
	hypoID := "valid-until-explicit-hypo"
	err := tools.DB.CreateHolon(ctx, hypoID, "hypothesis", "system", "L1", "Test Hypothesis", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	// Call ManageEvidence with explicit validUntil
	explicitDate := "2025-06-15"
	_, err = tools.ManageEvidence("validation", "add", hypoID, "internal", "Test passed", "pass", "L2", "test-runner", explicitDate)
	if err != nil {
		t.Fatalf("ManageEvidence failed: %v", err)
	}

	evidence, err := tools.DB.GetEvidence(ctx, hypoID)
	if err != nil {
		t.Fatalf("GetEvidence failed: %v", err)
	}

	if len(evidence) == 0 {
		t.Fatal("No evidence found in DB")
	}

	e := evidence[0]
	if !e.ValidUntil.Valid {
		t.Error("valid_until should be set when explicitly provided")
	} else {
		expected, _ := time.Parse("2006-01-02", explicitDate)
		if !e.ValidUntil.Time.Equal(expected) {
			t.Errorf("valid_until %v does not match expected %v", e.ValidUntil.Time, expected)
		} else {
			t.Logf("OK: valid_until correctly set to explicit date %v", e.ValidUntil.Time)
		}
	}
}

func TestInternalize_FirstCall(t *testing.T) {
	tempDir := t.TempDir()
	// Do NOT create .quint - let Internalize do it
	dbPath := filepath.Join(tempDir, ".quint", "quint.db")
	fsm := &FSM{State: State{}}
	tools := &Tools{FSM: fsm, RootDir: tempDir, DB: nil}

	result, err := tools.Internalize()
	if err != nil {
		t.Fatalf("Internalize() error = %v", err)
	}

	if !strings.Contains(result, "Status: INITIALIZED") {
		t.Errorf("First call should return INITIALIZED, got: %s", result)
	}
	if !strings.Contains(result, "Session Phase: EMPTY") {
		t.Errorf("Phase should be EMPTY after init (no hypotheses yet), got: %s", result)
	}

	// Verify .quint was created
	if _, err := os.Stat(filepath.Join(tempDir, ".quint")); os.IsNotExist(err) {
		t.Error(".quint directory should be created")
	}

	// Verify DB was initialized
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("quint.db should be created")
	}
}

func TestInternalize_SubsequentCall(t *testing.T) {
	tools, _, _ := setupTools(t)

	// First call - sets up everything
	_, err := tools.Internalize()
	if err != nil {
		t.Fatalf("First Internalize() error = %v", err)
	}

	// Second call - should return READY (context is fresh, nothing changed)
	result, err := tools.Internalize()
	if err != nil {
		t.Fatalf("Second Internalize() error = %v", err)
	}

	if !strings.Contains(result, "Status: READY") {
		t.Errorf("Subsequent call should return READY, got: %s", result)
	}
}

func TestInternalize_LayerCounts(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create some L0 holons in the database
	err := tools.DB.CreateHolon(ctx, "layer-count-hypo1", "hypothesis", "system", "L0",
		"Test Hypo 1", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}
	err = tools.DB.CreateHolon(ctx, "layer-count-hypo2", "hypothesis", "system", "L0",
		"Test Hypo 2", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	result, err := tools.Internalize()
	if err != nil {
		t.Fatalf("Internalize() error = %v", err)
	}

	if !strings.Contains(result, "L0 (Conjecture): 2") {
		t.Errorf("Should show 2 L0 holons, got: %s", result)
	}
}

func TestInternalize_ArchivedHolons(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a DRR (decision)
	err := tools.DB.CreateHolon(ctx, "DRR-archive-test", "DRR", "system", "DRR",
		"Archive Test Decision", "Decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Create an L2 holon that was selected by this decision
	err = tools.DB.CreateHolon(ctx, "archived-hypo", "hypothesis", "system", "L2",
		"Archived Hypothesis", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create hypothesis: %v", err)
	}

	// Create a 'selects' relation: DRR → winner hypothesis
	// (This is how FinalizeDecision creates relations)
	// Signature: CreateRelation(sourceID, relationType, targetID, cl)
	err = tools.DB.CreateRelation(ctx, "DRR-archive-test", "selects", "archived-hypo", 3)
	if err != nil {
		t.Fatalf("Failed to create selects relation: %v", err)
	}

	// Create an L0 holon that's NOT part of any decision (should be active)
	err = tools.DB.CreateHolon(ctx, "active-hypo", "hypothesis", "system", "L0",
		"Active Hypothesis", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create active hypothesis: %v", err)
	}

	// After selects relation: archived-hypo should immediately be excluded from active count
	// (New behavior: L2s are excluded immediately after decision, not after resolution)
	result, err := tools.Internalize()
	if err != nil {
		t.Fatalf("Internalize() error = %v", err)
	}

	// L2 should be 0 immediately after decision (selects relation exists)
	if !strings.Contains(result, "L2 (Corroborated): 0") {
		t.Errorf("After decision, L2 holon should be excluded from active count, got: %s", result)
	}

	// Resolve the decision (add implementation evidence)
	err = tools.DB.AddEvidence(ctx, "resolve-evidence", "DRR-archive-test", "implementation",
		"Implemented via commit:abc123", "pass", "L2", "commit:abc123", "", "", "")
	if err != nil {
		t.Fatalf("Failed to add resolution evidence: %v", err)
	}

	// After resolution: still excluded (unchanged)
	result, err = tools.Internalize()
	if err != nil {
		t.Fatalf("Internalize() after resolution error = %v", err)
	}

	// L2 should still be 0
	if !strings.Contains(result, "L2 (Corroborated): 0") {
		t.Errorf("After resolution, L2 holon should still be excluded, got: %s", result)
	}

	// Should show archived count
	if !strings.Contains(result, "Archived: 1 holons in resolved decisions") {
		t.Errorf("Should show 1 archived holon, got: %s", result)
	}

	// L0 should still be 1 (active-hypo is not in any decision)
	if !strings.Contains(result, "L0 (Conjecture): 1") {
		t.Errorf("L0 holon should still be active, got: %s", result)
	}
}

func TestSearch_NoResults(t *testing.T) {
	tools, _, _ := setupTools(t)

	result, err := tools.Search("xyznonexistentquery", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !strings.Contains(result, "No results found") {
		t.Errorf("Should return 'No results found', got: %s", result)
	}
}

func TestSearch_WithResults(t *testing.T) {
	tools, _, _ := setupTools(t)

	// Create a holon that will be indexed
	ctx := context.Background()
	tools.DB.CreateHolon(ctx, "search-test-holon", "hypothesis", "system", "L0",
		"Authentication Decision", "How to handle user authentication", "default", "", "")

	// Search for it
	result, err := tools.Search("authentication", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if strings.Contains(result, "No results found") {
		t.Errorf("Should find results, got: %s", result)
	}
	if !strings.Contains(result, "Authentication Decision") {
		t.Errorf("Should find 'Authentication Decision', got: %s", result)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	tools, _, _ := setupTools(t)

	_, err := tools.Search("", "", "", "", "", 10)
	if err == nil {
		t.Error("Empty query should return error")
	}
}

func TestSearch_NoDB(t *testing.T) {
	tempDir := t.TempDir()
	fsm := &FSM{State: State{}}
	tools := &Tools{FSM: fsm, RootDir: tempDir, DB: nil}

	_, err := tools.Search("test", "", "", "", "", 10)
	if err == nil {
		t.Error("Search without DB should return error")
	}
	if !strings.Contains(err.Error(), "database not initialized") {
		t.Errorf("Should mention database not initialized, got: %v", err)
	}
}

func TestSearch_LayerFilter(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create holons in different layers
	tools.DB.CreateHolon(ctx, "l0-holon", "hypothesis", "system", "L0",
		"L0 Test Holon", "Content for L0", "default", "", "")
	tools.DB.CreateHolon(ctx, "l2-holon", "hypothesis", "system", "L2",
		"L2 Test Holon", "Content for L2", "default", "", "")

	// Search with L2 filter
	result, err := tools.Search("Test Holon", "", "L2", "", "", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if strings.Contains(result, "L0 Test Holon") {
		t.Errorf("L2 filter should exclude L0 holons, got: %s", result)
	}
}

func TestSearch_SpecialCharacters(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a holon with hyphenated title
	tools.DB.CreateHolon(ctx, "special-char-holon", "hypothesis", "system", "L0",
		"Redis-backed Cache Strategy", "Use redis-cluster for caching", "default", "", "")

	// Search with hyphenated query (previously would cause FTS5 parse error)
	result, err := tools.Search("redis-backed", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("Search with hyphens should not error: %v", err)
	}

	if strings.Contains(result, "No results found") {
		t.Errorf("Should find hyphenated content, got: %s", result)
	}
}

func TestResolve_Implemented(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a DRR holon
	err := tools.DB.CreateHolon(ctx, "DRR-test-decision", "DRR", "system", "DRR",
		"Test Decision", "Decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Resolve as implemented
	input := ResolveInput{
		DecisionID: "DRR-test-decision",
		Resolution: "implemented",
		Reference:  "commit:abc1234",
	}
	result, err := tools.Resolve(input)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !strings.Contains(result, "implemented") {
		t.Errorf("Should confirm implemented status, got: %s", result)
	}
	if !strings.Contains(result, "commit:abc1234") {
		t.Errorf("Should include reference, got: %s", result)
	}

	// Verify evidence was created
	evidence, err := tools.DB.GetEvidence(ctx, "DRR-test-decision")
	if err != nil {
		t.Fatalf("GetEvidence() error = %v", err)
	}
	if len(evidence) == 0 {
		t.Error("Expected evidence to be created")
	}
	if evidence[0].Type != "implementation" {
		t.Errorf("Evidence type should be 'implementation', got: %s", evidence[0].Type)
	}
}

func TestResolve_Abandoned(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a DRR holon
	err := tools.DB.CreateHolon(ctx, "DRR-abandoned-test", "DRR", "system", "DRR",
		"Abandoned Decision", "Decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Resolve as abandoned
	input := ResolveInput{
		DecisionID: "DRR-abandoned-test",
		Resolution: "abandoned",
		Notes:      "Requirements changed",
	}
	result, err := tools.Resolve(input)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !strings.Contains(result, "abandoned") {
		t.Errorf("Should confirm abandoned status, got: %s", result)
	}

	// Verify evidence was created
	evidence, err := tools.DB.GetEvidence(ctx, "DRR-abandoned-test")
	if err != nil {
		t.Fatalf("GetEvidence() error = %v", err)
	}
	if len(evidence) == 0 {
		t.Error("Expected evidence to be created")
	}
	if evidence[0].Type != "abandonment" {
		t.Errorf("Evidence type should be 'abandonment', got: %s", evidence[0].Type)
	}
}

func TestResolve_Superseded(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create old DRR
	err := tools.DB.CreateHolon(ctx, "DRR-old-decision", "DRR", "system", "DRR",
		"Old Decision", "Old decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create old DRR: %v", err)
	}

	// Create new DRR that supersedes old
	err = tools.DB.CreateHolon(ctx, "DRR-new-decision", "DRR", "system", "DRR",
		"New Decision", "New decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create new DRR: %v", err)
	}

	// Resolve old as superseded
	input := ResolveInput{
		DecisionID:   "DRR-old-decision",
		Resolution:   "superseded",
		SupersededBy: "DRR-new-decision",
		Notes:        "Better approach found",
	}
	result, err := tools.Resolve(input)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if !strings.Contains(result, "superseded") {
		t.Errorf("Should confirm superseded status, got: %s", result)
	}
	if !strings.Contains(result, "DRR-new-decision") {
		t.Errorf("Should mention replacement DRR, got: %s", result)
	}
}

func TestResolve_InvalidDecision(t *testing.T) {
	tools, _, _ := setupTools(t)

	// Try to resolve non-existent decision
	input := ResolveInput{
		DecisionID: "DRR-does-not-exist",
		Resolution: "implemented",
		Reference:  "commit:abc1234",
	}
	_, err := tools.Resolve(input)
	if err == nil {
		t.Error("Should error on non-existent decision")
	}
}

func TestResolve_InvalidResolution(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a DRR holon
	err := tools.DB.CreateHolon(ctx, "DRR-invalid-res", "DRR", "system", "DRR",
		"Test Decision", "Decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Try invalid resolution type
	input := ResolveInput{
		DecisionID: "DRR-invalid-res",
		Resolution: "invalid_type",
	}
	_, err = tools.Resolve(input)
	if err == nil {
		t.Error("Should error on invalid resolution type")
	}
}

func TestResolve_MissingRequiredParams(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a DRR holon
	err := tools.DB.CreateHolon(ctx, "DRR-missing-params", "DRR", "system", "DRR",
		"Test Decision", "Decision content", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// implemented without reference
	input := ResolveInput{
		DecisionID: "DRR-missing-params",
		Resolution: "implemented",
	}
	_, err = tools.Resolve(input)
	if err == nil {
		t.Error("implemented should require reference")
	}

	// abandoned without notes
	input = ResolveInput{
		DecisionID: "DRR-missing-params",
		Resolution: "abandoned",
	}
	_, err = tools.Resolve(input)
	if err == nil {
		t.Error("abandoned should require notes")
	}

	// superseded without superseded_by
	input = ResolveInput{
		DecisionID: "DRR-missing-params",
		Resolution: "superseded",
	}
	_, err = tools.Resolve(input)
	if err == nil {
		t.Error("superseded should require superseded_by")
	}
}

func TestSearch_StatusFilterOpen(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create open DRR (no resolution evidence)
	err := tools.DB.CreateHolon(ctx, "DRR-open-test", "DRR", "system", "DRR",
		"Open Decision", "Pending resolution", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create open DRR: %v", err)
	}

	// Create resolved DRR (with implementation evidence)
	err = tools.DB.CreateHolon(ctx, "DRR-resolved-test", "DRR", "system", "DRR",
		"Resolved Decision", "Already implemented", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create resolved DRR: %v", err)
	}
	err = tools.DB.AddEvidence(ctx, "impl-evidence", "DRR-resolved-test", "implementation",
		"Implemented in commit abc123", "pass", "L2", "developer", "", "", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Search for open decisions
	result, err := tools.Search("Decision", "", "", "open", "", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !strings.Contains(result, "Open Decision") {
		t.Errorf("Should find open decision, got: %s", result)
	}
	if strings.Contains(result, "Resolved Decision") {
		t.Errorf("Should NOT find resolved decision with open filter, got: %s", result)
	}
}

func TestSearch_StatusFilterImplemented(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create implemented DRR
	err := tools.DB.CreateHolon(ctx, "DRR-impl-search", "DRR", "system", "DRR",
		"Implemented Decision", "Already done", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}
	err = tools.DB.AddEvidence(ctx, "impl-evidence-2", "DRR-impl-search", "implementation",
		"Done", "pass", "L2", "developer", "", "", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	// Search for implemented decisions
	result, err := tools.Search("Decision", "", "", "implemented", "", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if !strings.Contains(result, "Implemented Decision") {
		t.Errorf("Should find implemented decision, got: %s", result)
	}
}

func TestInternalize_ShowsOpenDecisions(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create an open DRR
	err := tools.DB.CreateHolon(ctx, "DRR-open-internalize", "DRR", "system", "DRR",
		"Pending Decision", "Awaiting resolution", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Internalize and check output
	result, err := tools.Internalize()
	if err != nil {
		t.Fatalf("Internalize() error = %v", err)
	}

	if !strings.Contains(result, "Open Decisions") || !strings.Contains(result, "Pending Decision") {
		t.Errorf("Should show open decision in internalize output, got: %s", result)
	}
}

func TestGetOpenDecisions(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create open DRR
	err := tools.DB.CreateHolon(ctx, "DRR-get-open", "DRR", "system", "DRR",
		"Open for Query", "Test", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	decisions, err := tools.GetOpenDecisions(ctx)
	if err != nil {
		t.Fatalf("GetOpenDecisions() error = %v", err)
	}

	if len(decisions) == 0 {
		t.Error("Expected at least one open decision")
	}

	found := false
	for _, d := range decisions {
		if d.ID == "DRR-get-open" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find DRR-get-open in open decisions")
	}
}

func TestGetResolvedDecisions(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create and resolve a DRR
	err := tools.DB.CreateHolon(ctx, "DRR-get-resolved", "DRR", "system", "DRR",
		"Resolved for Query", "Test", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Add resolution evidence
	err = tools.DB.AddEvidence(ctx, "impl-get-resolved", "DRR-get-resolved", "implementation",
		"Done", "pass", "L2", "dev", "", "", "2099-12-31")
	if err != nil {
		t.Fatalf("Failed to add evidence: %v", err)
	}

	decisions, err := tools.GetResolvedDecisions(ctx, "implemented", 10)
	if err != nil {
		t.Fatalf("GetResolvedDecisions() error = %v", err)
	}

	found := false
	for _, d := range decisions {
		if d.ID == "DRR-get-resolved" {
			found = true
			if d.Resolution != "implemented" {
				t.Errorf("Expected resolution 'implemented', got: %s", d.Resolution)
			}
			break
		}
	}
	if !found {
		t.Error("Expected to find DRR-get-resolved in resolved decisions")
	}
}

func TestResetCycle_Basic(t *testing.T) {
	tools, _, _ := setupTools(t)

	result, err := tools.ResetCycle("test reset", "", false)
	if err != nil {
		t.Fatalf("ResetCycle() error = %v", err)
	}

	// Verify output contains expected information
	if !strings.Contains(result, "Cycle reset") {
		t.Errorf("Should confirm reset, got: %s", result)
	}
	if !strings.Contains(result, "Previous stage:") {
		t.Errorf("Should show previous stage, got: %s", result)
	}
	if !strings.Contains(result, "test reset") {
		t.Errorf("Should show reason, got: %s", result)
	}
}

func TestResetCycle_DefaultReason(t *testing.T) {
	tools, _, _ := setupTools(t)

	result, err := tools.ResetCycle("", "", false) // Empty reason
	if err != nil {
		t.Fatalf("ResetCycle() error = %v", err)
	}

	if !strings.Contains(result, "user requested reset") {
		t.Errorf("Should use default reason, got: %s", result)
	}
}

func TestResetCycle_ShowsOpenDecisions(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create an open DRR
	err := tools.DB.CreateHolon(ctx, "DRR-open-during-reset", "DRR", "system", "DRR",
		"Open Decision", "Pending", "default", "global", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	result, err := tools.ResetCycle("ending session", "", false)
	if err != nil {
		t.Fatalf("ResetCycle() error = %v", err)
	}

	// Should mention open decisions in output
	if !strings.Contains(result, "Open decisions:") || !strings.Contains(result, "DRR-open-during-reset") {
		t.Errorf("Should list open decisions, got: %s", result)
	}
}

func TestResetCycle_ShowsLayerCounts(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// v5.0.0: hypotheses are DB-only
	tools.DB.CreateHolon(ctx, "hypo1", "hypothesis", "system", "L0", "Hypo 1", "Content", "default", "", "")
	tools.DB.CreateHolon(ctx, "hypo2", "hypothesis", "system", "L0", "Hypo 2", "Content", "default", "", "")
	tools.DB.CreateHolon(ctx, "hypo3", "hypothesis", "system", "L1", "Hypo 3", "Content", "default", "", "")

	result, err := tools.ResetCycle("session complete", "", false)
	if err != nil {
		t.Fatalf("ResetCycle() error = %v", err)
	}

	// Should show layer counts
	if !strings.Contains(result, "L0: 2") {
		t.Errorf("Should show L0 count of 2, got: %s", result)
	}
	if !strings.Contains(result, "L1: 1") {
		t.Errorf("Should show L1 count of 1, got: %s", result)
	}
}

func TestResetCycle_NoDRRCreated(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	// Count DRRs before reset
	decisionsDir := filepath.Join(tempDir, ".quint", "decisions")
	beforeFiles, _ := os.ReadDir(decisionsDir)
	beforeCount := 0
	for _, f := range beforeFiles {
		if strings.HasPrefix(f.Name(), "DRR-") {
			beforeCount++
		}
	}

	// Reset
	_, err := tools.ResetCycle("testing no DRR", "", false)
	if err != nil {
		t.Fatalf("ResetCycle() error = %v", err)
	}

	// Count DRRs after reset
	afterFiles, _ := os.ReadDir(decisionsDir)
	afterCount := 0
	for _, f := range afterFiles {
		if strings.HasPrefix(f.Name(), "DRR-") {
			afterCount++
		}
	}

	if afterCount != beforeCount {
		t.Errorf("ResetCycle should NOT create DRR files. Before: %d, After: %d", beforeCount, afterCount)
	}

	// Also check DB for DRR holons
	rawDB := tools.DB.GetRawDB()
	var drrCount int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM holons
		WHERE type = 'DRR' OR layer = 'DRR'
	`).Scan(&drrCount)
	if err != nil {
		t.Fatalf("Failed to query DRRs: %v", err)
	}
	if drrCount != 0 {
		t.Errorf("ResetCycle should NOT create DRR holons. Found: %d", drrCount)
	}
}

func TestResetCycle_AuditLogEntry(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	_, err := tools.ResetCycle("audit log test", "", false)
	if err != nil {
		t.Fatalf("ResetCycle() error = %v", err)
	}

	// Check audit log for reset entry
	rawDB := tools.DB.GetRawDB()
	var count int
	err = rawDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_log
		WHERE operation = 'cycle_reset'
		AND tool_name = 'quint_reset'
	`).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query audit_log: %v", err)
	}
	if count == 0 {
		t.Error("Expected audit_log entry for cycle_reset")
	}
}

// Tests for quint_implement

func TestImplement_BasicContract(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	// Create a DRR with contract
	contractJSON := `{"invariants":["Cache must be transparent","TTL configurable"],"anti_patterns":["No hardcoded TTL","No silent failures"],"acceptance_criteria":["Cache hit skips DB","Write invalidates cache"],"affected_scope":["internal/cache/*.go"]}`

	drrID := "test-implement-drr"
	err := tools.DB.CreateHolon(ctx, drrID, "DRR", "system", "DRR",
		"Test Implementation", "Test DRR for implement", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Write DRR markdown file with contract in frontmatter
	decisionsDir := filepath.Join(tempDir, ".quint", "decisions")
	os.MkdirAll(decisionsDir, 0755)
	drrPath := filepath.Join(decisionsDir, fmt.Sprintf("DRR-2025-01-01-%s.md", drrID))
	drrContent := fmt.Sprintf(`---
title: Test Implementation
contract: %s
created: 2025-01-01T00:00:00Z
content_hash: abc123
---

# Test Implementation

Test content.
`, contractJSON)
	if err := os.WriteFile(drrPath, []byte(drrContent), 0644); err != nil {
		t.Fatalf("Failed to write DRR file: %v", err)
	}

	// Call Implement
	result, err := tools.Implement(drrID)
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	// Verify output contains key sections
	if !strings.Contains(result, "# IMPLEMENTATION DIRECTIVE") {
		t.Error("Missing IMPLEMENTATION DIRECTIVE header")
	}
	if !strings.Contains(result, "Test Implementation") {
		t.Error("Missing task title")
	}
	if !strings.Contains(result, "Cache must be transparent") {
		t.Error("Missing invariant")
	}
	if !strings.Contains(result, "No hardcoded TTL") {
		t.Error("Missing anti-pattern")
	}
	if !strings.Contains(result, "Cache hit skips DB") {
		t.Error("Missing acceptance criteria")
	}
	if !strings.Contains(result, "internal/cache/*.go") {
		t.Error("Missing affected scope")
	}
	if !strings.Contains(result, "quint_resolve") {
		t.Error("Missing resolve instruction")
	}
}

func TestImplement_NoContract(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a DRR without contract
	drrID := "no-contract-drr"
	err := tools.DB.CreateHolon(ctx, drrID, "DRR", "system", "DRR",
		"No Contract DRR", "Test", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Call Implement - should fail
	_, err = tools.Implement(drrID)
	if err == nil {
		t.Error("Expected error for DRR without contract")
	}
	if !strings.Contains(err.Error(), "no implementation contract") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestImplement_NotFound(t *testing.T) {
	tools, _, _ := setupTools(t)

	_, err := tools.Implement("nonexistent-drr")
	if err == nil {
		t.Error("Expected error for nonexistent DRR")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestImplement_NotDRR(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a regular hypothesis, not a DRR
	err := tools.DB.CreateHolon(ctx, "regular-hypo", "hypothesis", "system", "L0",
		"Regular Hypothesis", "Not a DRR", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}

	_, err = tools.Implement("regular-hypo")
	if err == nil {
		t.Error("Expected error for non-DRR holon")
	}
	if !strings.Contains(err.Error(), "not a DRR") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestImplement_InheritedConstraints(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	// Create parent DRR with contract
	parentContract := `{"invariants":["Parent invariant"],"anti_patterns":["Parent anti-pattern"]}`
	parentID := "parent-drr"
	err := tools.DB.CreateHolon(ctx, parentID, "DRR", "system", "DRR",
		"Parent Decision", "Parent", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create parent DRR: %v", err)
	}

	// Write parent DRR file
	decisionsDir := filepath.Join(tempDir, ".quint", "decisions")
	os.MkdirAll(decisionsDir, 0755)
	parentPath := filepath.Join(decisionsDir, fmt.Sprintf("DRR-2025-01-01-%s.md", parentID))
	parentContent := fmt.Sprintf(`---
title: Parent Decision
contract: %s
content_hash: abc123
---
# Parent Decision
`, parentContract)
	os.WriteFile(parentPath, []byte(parentContent), 0644)

	// Create child DRR with its own contract
	childContract := `{"invariants":["Child invariant"],"anti_patterns":["Child anti-pattern"],"acceptance_criteria":["Child criteria"]}`
	childID := "child-drr"
	err = tools.DB.CreateHolon(ctx, childID, "DRR", "system", "DRR",
		"Child Decision", "Child", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create child DRR: %v", err)
	}

	// Create dependency relation: child -> parent
	err = tools.DB.CreateRelation(ctx, childID, "selects", parentID, 3)
	if err != nil {
		t.Fatalf("Failed to create relation: %v", err)
	}

	// Write child DRR file
	childPath := filepath.Join(decisionsDir, fmt.Sprintf("DRR-2025-01-01-%s.md", childID))
	childContent := fmt.Sprintf(`---
title: Child Decision
contract: %s
content_hash: def456
---
# Child Decision
`, childContract)
	os.WriteFile(childPath, []byte(childContent), 0644)

	// Call Implement on child
	result, err := tools.Implement(childID)
	if err != nil {
		t.Fatalf("Implement() error = %v", err)
	}

	// Should contain both child's own constraints
	if !strings.Contains(result, "Child invariant") {
		t.Error("Missing child invariant")
	}
	if !strings.Contains(result, "Child anti-pattern") {
		t.Error("Missing child anti-pattern")
	}

	// And inherited constraints from parent
	if !strings.Contains(result, "Parent invariant") {
		t.Error("Missing inherited parent invariant")
	}
	if !strings.Contains(result, "Parent anti-pattern") {
		t.Error("Missing inherited parent anti-pattern")
	}

	// Should have warning about inherited constraints
	if !strings.Contains(result, "Inherited") {
		t.Error("Missing inheritance indicator")
	}
}

func TestImplement_NoDB(t *testing.T) {
	tempDir := t.TempDir()
	fsm := &FSM{State: State{}}
	tools := &Tools{FSM: fsm, RootDir: tempDir, DB: nil}

	_, err := tools.Implement("any-drr")
	if err == nil {
		t.Error("Expected error when DB not initialized")
	}
	if !strings.Contains(err.Error(), "database not initialized") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestImplement_FullFilenameID(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	// Create DRR with slugified ID (as quint_decide does)
	slugifiedID := "redis-cache-with-monitoring"
	contractJSON := `{"invariants":["Cache transparent"],"acceptance_criteria":["Works"]}`

	err := tools.DB.CreateHolon(ctx, slugifiedID, "DRR", "system", "DRR",
		"Redis Cache with Monitoring", "Test", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// Write DRR file with dated filename
	decisionsDir := filepath.Join(tempDir, ".quint", "decisions")
	os.MkdirAll(decisionsDir, 0755)
	drrPath := filepath.Join(decisionsDir, fmt.Sprintf("DRR-2025-12-24-%s.md", slugifiedID))
	drrContent := fmt.Sprintf(`---
title: Redis Cache with Monitoring
contract: %s
content_hash: abc123
---
# Redis Cache with Monitoring
`, contractJSON)
	os.WriteFile(drrPath, []byte(drrContent), 0644)

	// Call Implement with FULL filename (what agent typically uses)
	fullFilenameID := "DRR-2025-12-24-redis-cache-with-monitoring"
	result, err := tools.Implement(fullFilenameID)
	if err != nil {
		t.Fatalf("Implement() with full filename ID should work, got error: %v", err)
	}

	if !strings.Contains(result, "Cache transparent") {
		t.Error("Missing invariant in output")
	}

	// Also verify short ID still works
	result2, err := tools.Implement(slugifiedID)
	if err != nil {
		t.Fatalf("Implement() with short ID should work, got error: %v", err)
	}

	if !strings.Contains(result2, "Cache transparent") {
		t.Error("Missing invariant in output for short ID")
	}
}

func TestLinkHolons_Basic(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	id1 := uuid.New().String()
	id2 := uuid.New().String()

	tools.DB.CreateHolon(ctx, id1, "hypothesis", "system", "L0", "Source Holon", "content", "default", "", "")
	tools.DB.CreateHolon(ctx, id2, "hypothesis", "system", "L0", "Target Holon", "content", "default", "", "")

	result, err := tools.LinkHolons(id1, id2, 3)
	if err != nil {
		t.Fatalf("LinkHolons failed: %v", err)
	}

	if !strings.Contains(result, "✅ Linked") {
		t.Error("Missing success indicator")
	}
	if !strings.Contains(result, "componentOf") {
		t.Error("Expected componentOf relation for system kind")
	}
	if !strings.Contains(result, "WLNK now applies") {
		t.Error("Missing WLNK explanation")
	}

	deps, err := tools.DB.GetDependencies(ctx, id1)
	if err != nil {
		t.Fatalf("GetDependencies failed: %v", err)
	}
	if len(deps) != 1 || deps[0].TargetID != id2 {
		t.Error("Relation not created correctly")
	}
}

func TestLinkHolons_EpistemeKind(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	id1 := uuid.New().String()
	id2 := uuid.New().String()

	tools.DB.CreateHolon(ctx, id1, "hypothesis", "episteme", "L0", "Episteme Source", "content", "default", "", "")
	tools.DB.CreateHolon(ctx, id2, "hypothesis", "system", "L0", "Target Holon", "content", "default", "", "")

	result, err := tools.LinkHolons(id1, id2, 3)
	if err != nil {
		t.Fatalf("LinkHolons failed: %v", err)
	}

	if !strings.Contains(result, "constituentOf") {
		t.Error("Expected constituentOf relation for episteme kind")
	}
}

func TestLinkHolons_SourceNotFound(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	id2 := uuid.New().String()
	tools.DB.CreateHolon(ctx, id2, "hypothesis", "system", "L0", "Target Holon", "content", "default", "", "")

	_, err := tools.LinkHolons("nonexistent", id2, 3)
	if err == nil {
		t.Error("Expected error for nonexistent source")
	}
	if !strings.Contains(err.Error(), "source holon") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestLinkHolons_TargetNotFound(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	id1 := uuid.New().String()
	tools.DB.CreateHolon(ctx, id1, "hypothesis", "system", "L0", "Source Holon", "content", "default", "", "")

	_, err := tools.LinkHolons(id1, "nonexistent", 3)
	if err == nil {
		t.Error("Expected error for nonexistent target")
	}
	if !strings.Contains(err.Error(), "target holon") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestLinkHolons_CyclePrevention(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	id1 := uuid.New().String()
	id2 := uuid.New().String()

	tools.DB.CreateHolon(ctx, id1, "hypothesis", "system", "L0", "A", "content", "default", "", "")
	tools.DB.CreateHolon(ctx, id2, "hypothesis", "system", "L0", "B", "content", "default", "", "")

	_, err := tools.LinkHolons(id1, id2, 3)
	if err != nil {
		t.Fatalf("First link failed: %v", err)
	}

	_, err = tools.LinkHolons(id2, id1, 3)
	if err == nil {
		t.Error("Expected error for cycle creation")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestLinkHolons_NoDB(t *testing.T) {
	tempDir := t.TempDir()
	fsm := &FSM{State: State{}}
	tools := &Tools{FSM: fsm, RootDir: tempDir, DB: nil}

	_, err := tools.LinkHolons("a", "b", 3)
	if err == nil {
		t.Error("Expected error when DB is nil")
	}
	if !strings.Contains(err.Error(), "database not initialized") {
		t.Errorf("Wrong error: %v", err)
	}
}

func TestLinkHolons_CLValidation(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	id1 := uuid.New().String()
	id2 := uuid.New().String()

	tools.DB.CreateHolon(ctx, id1, "hypothesis", "system", "L0", "Source", "content", "default", "", "")
	tools.DB.CreateHolon(ctx, id2, "hypothesis", "system", "L0", "Target", "content", "default", "", "")

	_, err := tools.LinkHolons(id1, id2, 0)
	if err != nil {
		t.Fatalf("LinkHolons with CL=0 should default to 3: %v", err)
	}

	deps, _ := tools.DB.GetDependencies(ctx, id1)
	if len(deps) != 1 || deps[0].CongruenceLevel.Int64 != 3 {
		t.Errorf("Expected CL=3 (default), got %d", deps[0].CongruenceLevel.Int64)
	}
}

func TestProposeHypothesis_ActiveSuggestions(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	// Create decision context first (required since v5.0.0)
	err := tools.DB.CreateHolon(ctx, "dc-suggestions-test", "decision_context", "system", "L0", "Suggestions Test", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	os.MkdirAll(filepath.Join(tempDir, ".quint", "knowledge", "L0"), 0755)
	os.MkdirAll(filepath.Join(tempDir, ".quint", "knowledge", "DRR"), 0755)

	tools.DB.CreateHolon(ctx, "redis-cache-drr", "DRR", "system", "DRR",
		"Redis Cache Layer", "Implement caching with Redis", "default", "src/cache/*", "")

	result, err := tools.ProposeHypothesis(
		"Token Bucket Rate Limiter using Redis",
		"Implement rate limiting that stores counters in Redis",
		"src/api/*", "system",
		`{"anomaly": "API abuse", "approach": "Token bucket"}`,
		"dc-suggestions-test", nil, 3,
	)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	if !strings.Contains(result, "POTENTIAL DEPENDENCIES DETECTED") {
		t.Error("Expected dependency suggestion when FTS5 matches 'redis'")
	}
	if !strings.Contains(result, "redis-cache-drr") {
		t.Error("Expected redis-cache-drr to be suggested")
	}
	if !strings.Contains(result, "quint_link") {
		t.Error("Expected quint_link suggestion")
	}
	if !strings.Contains(result, "ranked by relevance") {
		t.Error("Expected FTS5-based ranking message")
	}
}

func TestProposeHypothesis_NoSuggestionsWhenDependsOnProvided(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	// Create decision context first (required since v5.0.0)
	err := tools.DB.CreateHolon(ctx, "dc-no-suggestions", "decision_context", "system", "L0", "No Suggestions Test", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	os.MkdirAll(filepath.Join(tempDir, ".quint", "knowledge", "L0"), 0755)

	tools.DB.CreateHolon(ctx, "redis-cache-drr", "DRR", "system", "DRR",
		"Redis Cache", "Redis caching", "default", "", "")

	result, err := tools.ProposeHypothesis(
		"Rate Limiter using Redis",
		"Implement rate limiting with Redis",
		"src/api/*", "system",
		`{"anomaly": "test"}`,
		"dc-no-suggestions", []string{"redis-cache-drr"}, 3,
	)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	if strings.Contains(result, "POTENTIAL DEPENDENCIES DETECTED") {
		t.Error("Should not show suggestions when depends_on is already provided")
	}
}

func TestProposeHypothesis_NoSuggestionsWhenNoMatches(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	// Create decision context first (required since v5.0.0)
	err := tools.DB.CreateHolon(ctx, "dc-no-matches", "decision_context", "system", "L0", "No Matches Test", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	os.MkdirAll(filepath.Join(tempDir, ".quint", "knowledge", "L0"), 0755)

	result, err := tools.ProposeHypothesis(
		"Standalone Feature XYZ",
		"Something completely unrelated to existing holons",
		"src/xyz/*", "system",
		`{"anomaly": "test"}`,
		"dc-no-matches", nil, 3,
	)
	if err != nil {
		t.Fatalf("ProposeHypothesis failed: %v", err)
	}

	if strings.Contains(result, "POTENTIAL DEPENDENCIES DETECTED") {
		t.Error("Should not show suggestions when no keywords match")
	}
}

// ============================================
// DECISION CONTEXT TESTS (v5.0.0)
// ============================================
// Note: Evidence staleness by carrier-file hash was removed in v5.1.0.
// Time-based decay via valid_until remains as per FPF spec B.3.4.

func TestGetActiveDecisionContexts_ReturnsActive(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	err := tools.DB.CreateHolon(ctx, "dc-active-1", "decision_context", "system", "L0", "Active 1", "Content", "default", "scope1", "")
	if err != nil {
		t.Fatalf("Failed to create context 1: %v", err)
	}
	err = tools.DB.CreateHolon(ctx, "dc-active-2", "decision_context", "system", "L0", "Active 2", "Content", "default", "scope2", "")
	if err != nil {
		t.Fatalf("Failed to create context 2: %v", err)
	}

	contexts, err := tools.GetActiveDecisionContexts(ctx)
	if err != nil {
		t.Fatalf("GetActiveDecisionContexts failed: %v", err)
	}

	if len(contexts) != 2 {
		t.Errorf("Expected 2 active contexts, got %d", len(contexts))
	}
}

func TestGetActiveDecisionContexts_ExcludesClosed(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	err := tools.DB.CreateHolon(ctx, "dc-closed-test", "decision_context", "system", "L0", "Closed Context", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	err = tools.DB.CreateHolon(ctx, "closing-drr", "decision", "system", "DRR", "Closing DRR", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	rawDB := tools.DB.GetRawDB()
	_, err = rawDB.ExecContext(ctx, `INSERT INTO relations (source_id, target_id, relation_type, congruence_level) VALUES ('closing-drr', 'dc-closed-test', 'closes', 3)`)
	if err != nil {
		t.Fatalf("Failed to create closes relation: %v", err)
	}

	contexts, err := tools.GetActiveDecisionContexts(ctx)
	if err != nil {
		t.Fatalf("GetActiveDecisionContexts failed: %v", err)
	}

	for _, c := range contexts {
		if c.ID == "dc-closed-test" {
			t.Errorf("Closed context dc-closed-test should not appear in active contexts")
		}
	}
}

func TestProposeHypothesis_RequiresDecisionContext(t *testing.T) {
	tools, _, _ := setupTools(t)

	// decision_context is now REQUIRED (v5.0.0 refactoring)
	_, err := tools.ProposeHypothesis(
		"Test Hypothesis",
		"Testing required decision_context",
		"test scope",
		"system",
		`{"anomaly": "test"}`,
		"", // no decision_context - should error
		nil,
		3,
	)
	if err == nil {
		t.Fatal("Expected error when decision_context is empty")
	}
	if !strings.Contains(err.Error(), "decision_context is required") {
		t.Errorf("Expected 'decision_context is required' error, got: %v", err)
	}
}

func TestCreateContext_Success(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	result, err := tools.CreateContext("Database Selection", "backend services", "Choosing between PostgreSQL and MySQL")
	if err != nil {
		t.Fatalf("CreateContext failed: %v", err)
	}

	if !strings.Contains(result, "dc-database-selection") {
		t.Errorf("Expected dc-database-selection in result, got: %s", result)
	}

	// Verify context exists in DB
	holon, err := tools.DB.GetHolon(ctx, "dc-database-selection")
	if err != nil {
		t.Fatalf("Failed to get context from DB: %v", err)
	}
	if holon.Type != "decision_context" {
		t.Errorf("Expected type decision_context, got %s", holon.Type)
	}
	if holon.Title != "Database Selection" {
		t.Errorf("Expected title 'Database Selection', got %s", holon.Title)
	}
}

func TestCreateContext_AlreadyExists(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create context first
	err := tools.DB.CreateHolon(ctx, "dc-existing-context", "decision_context", "system", "L0", "Existing", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	// Try to create same context again
	_, err = tools.CreateContext("Existing Context", "scope", "")
	if err == nil {
		t.Fatal("Expected error when creating duplicate context")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}
}

func TestCreateContext_Max3Limit(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create 3 contexts
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("dc-limit-test-%d", i)
		err := tools.DB.CreateHolon(ctx, id, "decision_context", "system", "L0", fmt.Sprintf("Context %d", i), "Content", "default", "", "")
		if err != nil {
			t.Fatalf("Failed to create context %d: %v", i, err)
		}
	}

	// Try to create 4th
	_, err := tools.CreateContext("Fourth Context", "scope", "")
	if err == nil {
		t.Fatal("Expected error when creating 4th context")
	}
	if !strings.Contains(err.Error(), "maximum 3 active") {
		t.Errorf("Expected 'maximum 3 active' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "USER ACTION REQUIRED") {
		t.Errorf("Expected USER ACTION REQUIRED in error, got: %v", err)
	}
}

func TestProposeHypothesis_DecisionContextTypeValidation(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	// Create a hypothesis (not a decision_context)
	err := tools.DB.CreateHolon(ctx, "not-a-context", "hypothesis", "system", "L0", "Not A Context", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create hypothesis: %v", err)
	}

	// Try to use the hypothesis as decision_context - should fail with type validation error
	_, err = tools.ProposeHypothesis(
		"Test Hypothesis",
		"Testing type validation",
		"test scope",
		"system",
		`{"anomaly": "test"}`,
		"not-a-context", // This is a hypothesis, not a decision_context
		nil,
		3,
	)
	if err == nil {
		t.Error("Expected error when using hypothesis as decision_context")
	}
	if err != nil && !strings.Contains(err.Error(), "not decision_context") {
		t.Errorf("Expected type validation error, got: %v", err)
	}
}

func TestResetCycle_AbandonContext(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	err := tools.DB.CreateHolon(ctx, "dc-abandon-test", "decision_context", "system", "L0", "Abandon Test", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	result, err := tools.ResetCycle("testing abandon", "dc-abandon-test", false)
	if err != nil {
		t.Fatalf("ResetCycle failed: %v", err)
	}

	if !strings.Contains(result, "Abandoned context: dc-abandon-test") {
		t.Errorf("Expected abandon message in result, got: %s", result)
	}

	// Verify context is not in active contexts
	contexts, err := tools.GetActiveDecisionContexts(ctx)
	if err != nil {
		t.Fatalf("GetActiveDecisionContexts failed: %v", err)
	}
	for _, c := range contexts {
		if c.ID == "dc-abandon-test" {
			t.Errorf("Abandoned context should not appear in active contexts")
		}
	}
}

func TestResetCycle_AbandonAll(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	for i := 1; i <= 2; i++ {
		id := fmt.Sprintf("dc-all-test-%d", i)
		err := tools.DB.CreateHolon(ctx, id, "decision_context", "system", "L0", fmt.Sprintf("All Test %d", i), "Content", "default", "", "")
		if err != nil {
			t.Fatalf("Failed to create context %d: %v", i, err)
		}
	}

	result, err := tools.ResetCycle("abandon all test", "", true)
	if err != nil {
		t.Fatalf("ResetCycle failed: %v", err)
	}

	if !strings.Contains(result, "Abandoned 2 contexts") {
		t.Errorf("Expected 'Abandoned 2 contexts' in result, got: %s", result)
	}

	// Verify no active contexts
	contexts, err := tools.GetActiveDecisionContexts(ctx)
	if err != nil {
		t.Fatalf("GetActiveDecisionContexts failed: %v", err)
	}
	if len(contexts) != 0 {
		t.Errorf("Expected 0 active contexts after abandon_all, got %d", len(contexts))
	}
}

func TestFinalizeDecision_ClosesContext(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	err := tools.DB.CreateHolon(ctx, "dc-finalize-close", "decision_context", "system", "L0", "Finalize Close Test", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	err = tools.DB.CreateHolon(ctx, "winner-hypo", "hypothesis", "system", "L2", "Winner", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create winner: %v", err)
	}

	if err := tools.createRelation(ctx, "winner-hypo", "memberOf", "dc-finalize-close", 3); err != nil {
		t.Fatalf("Failed to create memberOf relation: %v", err)
	}

	_, err = tools.FinalizeDecision("Close Context Test", "winner-hypo", nil, "Test", "Decision", "Rationale", "Consequences", "", "", true)
	if err != nil {
		t.Fatalf("FinalizeDecision failed: %v", err)
	}

	// Verify context is closed (not in active contexts)
	contexts, err := tools.GetActiveDecisionContexts(ctx)
	if err != nil {
		t.Fatalf("GetActiveDecisionContexts failed: %v", err)
	}
	for _, c := range contexts {
		if c.ID == "dc-finalize-close" {
			t.Errorf("Closed context should not appear in active contexts")
		}
	}
}

func TestFinalizeDecision_CloseContextFalse_KeepsContextOpen(t *testing.T) {
	tools, _, _ := setupTools(t)
	ctx := context.Background()

	dcID := "dc-multi-drr-test"
	err := tools.DB.CreateHolon(ctx, dcID, "decision_context", "", "L0", "Multi-DRR Test Context", "", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	hyp1ID := "hyp-multi-drr-1"
	err = tools.DB.CreateHolon(ctx, hyp1ID, "hypothesis", "system", "L2", "First Improvement", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create first hypothesis: %v", err)
	}
	if err := tools.createRelation(ctx, hyp1ID, "memberOf", dcID, 3); err != nil {
		t.Fatalf("Failed to create memberOf relation for hyp1: %v", err)
	}

	hyp2ID := "hyp-multi-drr-2"
	err = tools.DB.CreateHolon(ctx, hyp2ID, "hypothesis", "system", "L2", "Second Improvement", "Content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create second hypothesis: %v", err)
	}
	if err := tools.createRelation(ctx, hyp2ID, "memberOf", dcID, 3); err != nil {
		t.Fatalf("Failed to create memberOf relation for hyp2: %v", err)
	}

	_, err = tools.FinalizeDecision("First DRR", hyp1ID, nil, "Test", "Decision 1", "Rationale 1", "Consequences 1", "", "", false)
	if err != nil {
		t.Fatalf("First FinalizeDecision with closeContext=false failed: %v", err)
	}

	contexts, err := tools.GetActiveDecisionContexts(ctx)
	if err != nil {
		t.Fatalf("GetActiveDecisionContexts failed: %v", err)
	}
	found := false
	for _, c := range contexts {
		if c.ID == dcID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Context %s should still be active after closeContext=false", dcID)
	}

	_, err = tools.FinalizeDecision("Second DRR", hyp2ID, nil, "Test", "Decision 2", "Rationale 2", "Consequences 2", "", "", true)
	if err != nil {
		t.Fatalf("Second FinalizeDecision with closeContext=true failed: %v", err)
	}

	contexts, err = tools.GetActiveDecisionContexts(ctx)
	if err != nil {
		t.Fatalf("GetActiveDecisionContexts failed after second DRR: %v", err)
	}
	for _, c := range contexts {
		if c.ID == dcID {
			t.Errorf("Context %s should be closed after closeContext=true", dcID)
		}
	}
}

func TestImplement_AffectedScopeHashTracking(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	testFile := "src/calculator.go"
	testFilePath := filepath.Join(tempDir, testFile)
	os.MkdirAll(filepath.Dir(testFilePath), 0755)
	originalContent := "package calculator\nfunc Add(a, b int) int { return a + b }"
	os.WriteFile(testFilePath, []byte(originalContent), 0644)

	originalHash := "abc12345"

	drrID := "affected-hash-test-drr"
	err := tools.DB.CreateHolon(ctx, drrID, "DRR", "system", "DRR",
		"Affected Hash Test", "Test DRR", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	contractJSON := fmt.Sprintf(`{"invariants":["Must work"],"affected_scope":["%s"],"affected_hashes":{"%s":"%s"}}`,
		testFile, testFile, originalHash)
	decisionsDir := filepath.Join(tempDir, ".quint", "decisions")
	os.MkdirAll(decisionsDir, 0755)
	drrPath := filepath.Join(decisionsDir, fmt.Sprintf("DRR-2025-01-01-%s.md", drrID))
	drrContent := fmt.Sprintf(`---
title: Affected Hash Test
contract: %s
content_hash: abc123
---

# Affected Hash Test

Test content.
`, contractJSON)
	if err := os.WriteFile(drrPath, []byte(drrContent), 0644); err != nil {
		t.Fatalf("Failed to write DRR file: %v", err)
	}

	modifiedContent := "package calculator\nfunc Add(a, b int) int { return a + b + 0 } // CHANGED"
	os.WriteFile(testFilePath, []byte(modifiedContent), 0644)

	result, err := tools.Implement(drrID)
	if err != nil {
		t.Fatalf("Implement() failed: %v", err)
	}

	if !strings.Contains(result, "AFFECTED SCOPE CHANGED") {
		t.Error("Missing AFFECTED SCOPE CHANGED warning")
	}
	if !strings.Contains(result, testFile) {
		t.Errorf("Missing affected file in warning: %s", testFile)
	}
	if !strings.Contains(result, "content changed since decision") {
		t.Error("Missing content change description")
	}
}

func TestImplement_AffectedScopeNoChange(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	testFile := "src/unchanged.go"
	testFilePath := filepath.Join(tempDir, testFile)
	os.MkdirAll(filepath.Dir(testFilePath), 0755)
	content := "package unchanged\nfunc Nothing() {}"
	os.WriteFile(testFilePath, []byte(content), 0644)

	fileHash := tools.computeFileHash(testFilePath)

	drrID := "no-change-test-drr"
	err := tools.DB.CreateHolon(ctx, drrID, "DRR", "system", "DRR",
		"No Change Test", "Test DRR", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	contractJSON := fmt.Sprintf(`{"invariants":["Must work"],"affected_scope":["%s"],"affected_hashes":{"%s":"%s"}}`,
		testFile, testFile, fileHash)
	decisionsDir := filepath.Join(tempDir, ".quint", "decisions")
	os.MkdirAll(decisionsDir, 0755)
	drrPath := filepath.Join(decisionsDir, fmt.Sprintf("DRR-2025-01-01-%s.md", drrID))
	drrContent := fmt.Sprintf(`---
title: No Change Test
contract: %s
content_hash: abc123
---

# No Change Test

Test content.
`, contractJSON)
	if err := os.WriteFile(drrPath, []byte(drrContent), 0644); err != nil {
		t.Fatalf("Failed to write DRR file: %v", err)
	}

	result, err := tools.Implement(drrID)
	if err != nil {
		t.Fatalf("Implement() failed: %v", err)
	}

	if strings.Contains(result, "AFFECTED SCOPE CHANGED") {
		t.Error("Should NOT show AFFECTED SCOPE CHANGED warning when file unchanged")
	}
}

func TestImplement_AffectedScopeFileRemoved(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	testFile := "src/removed.go"
	testFilePath := filepath.Join(tempDir, testFile)
	os.MkdirAll(filepath.Dir(testFilePath), 0755)
	content := "package removed\nfunc WillBeRemoved() {}"
	os.WriteFile(testFilePath, []byte(content), 0644)

	fileHash := tools.computeFileHash(testFilePath)

	drrID := "remove-test-drr"
	err := tools.DB.CreateHolon(ctx, drrID, "DRR", "system", "DRR",
		"Remove Test", "Test DRR", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	contractJSON := fmt.Sprintf(`{"invariants":["Must work"],"affected_scope":["%s"],"affected_hashes":{"%s":"%s"}}`,
		testFile, testFile, fileHash)
	decisionsDir := filepath.Join(tempDir, ".quint", "decisions")
	os.MkdirAll(decisionsDir, 0755)
	drrPath := filepath.Join(decisionsDir, fmt.Sprintf("DRR-2025-01-01-%s.md", drrID))
	drrContent := fmt.Sprintf(`---
title: Remove Test
contract: %s
content_hash: abc123
---

# Remove Test

Test content.
`, contractJSON)
	if err := os.WriteFile(drrPath, []byte(drrContent), 0644); err != nil {
		t.Fatalf("Failed to write DRR file: %v", err)
	}

	os.Remove(testFilePath)

	result, err := tools.Implement(drrID)
	if err != nil {
		t.Fatalf("Implement() failed: %v", err)
	}

	if !strings.Contains(result, "AFFECTED SCOPE CHANGED") {
		t.Error("Missing AFFECTED SCOPE CHANGED warning for removed file")
	}
	if !strings.Contains(result, "file removed since decision") {
		t.Error("Missing file removed description")
	}
}

func TestFinalizeDecision_StoresAffectedHashes(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	testFile := "src/hashtest.go"
	testFilePath := filepath.Join(tempDir, testFile)
	os.MkdirAll(filepath.Dir(testFilePath), 0755)
	content := "package hashtest\nfunc Test() {}"
	os.WriteFile(testFilePath, []byte(content), 0644)

	dcID := "dc-storehash-test"
	err := tools.DB.CreateHolon(ctx, dcID, "decision_context", "", "L0", "Store Hash Test", "", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	winnerID := "storehash-winner"
	err = tools.DB.CreateHolon(ctx, winnerID, "hypothesis", "system", "L2", "Winner", "content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create winner: %v", err)
	}
	tools.createRelation(ctx, winnerID, "memberOf", dcID, 3)

	contractJSON := fmt.Sprintf(`{"invariants":["Must work"],"affected_scope":["%s"]}`, testFile)
	drrPath, err := tools.FinalizeDecision("Store Hash Test", winnerID, nil, "Test", "Decision", "Rationale", "Consequences", "", contractJSON, true)
	if err != nil {
		t.Fatalf("FinalizeDecision failed: %v", err)
	}

	if drrPath == "" {
		t.Fatal("FinalizeDecision should return DRR path")
	}

	drrContent, err := os.ReadFile(drrPath)
	if err != nil {
		t.Fatalf("Failed to read DRR file: %v", err)
	}

	if !strings.Contains(string(drrContent), "affected_hashes") {
		t.Error("DRR file should contain affected_hashes")
	}

	expectedHash := tools.computeFileHash(testFilePath)
	if !strings.Contains(string(drrContent), expectedHash) {
		t.Errorf("DRR file should contain computed hash %s", expectedHash)
	}
}

func TestFinalizeDecision_AffectedScopeWithClassRef(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	testFile := "src/calculator.py"
	testFilePath := filepath.Join(tempDir, testFile)
	os.MkdirAll(filepath.Dir(testFilePath), 0755)
	content := "class Calculator:\n    def add(self, a, b): return a + b"
	os.WriteFile(testFilePath, []byte(content), 0644)

	dcID := "dc-classref-test"
	err := tools.DB.CreateHolon(ctx, dcID, "decision_context", "", "L0", "Class Ref Test", "", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create decision context: %v", err)
	}

	winnerID := "classref-winner"
	err = tools.DB.CreateHolon(ctx, winnerID, "hypothesis", "system", "L2", "Winner", "content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create winner: %v", err)
	}
	tools.createRelation(ctx, winnerID, "memberOf", dcID, 3)

	// Use file:class format in affected_scope
	scopeRef := "src/calculator.py:Calculator"
	contractJSON := fmt.Sprintf(`{"invariants":["Must work"],"affected_scope":["%s"]}`, scopeRef)
	drrPath, err := tools.FinalizeDecision("Class Ref Test", winnerID, nil, "Test", "Decision", "Rationale", "Consequences", "", contractJSON, true)
	if err != nil {
		t.Fatalf("FinalizeDecision failed: %v", err)
	}

	drrContent, err := os.ReadFile(drrPath)
	if err != nil {
		t.Fatalf("Failed to read DRR file: %v", err)
	}

	if !strings.Contains(string(drrContent), "affected_hashes") {
		t.Error("DRR file should contain affected_hashes")
	}

	// Hash should be keyed by file path only (without :Calculator)
	expectedHash := tools.computeFileHash(testFilePath)
	if !strings.Contains(string(drrContent), expectedHash) {
		t.Errorf("DRR file should contain computed hash %s", expectedHash)
	}

	// Should NOT contain _missing_ since file exists
	if strings.Contains(string(drrContent), "_missing_") {
		t.Error("DRR should not have _missing_ for existing file")
	}
}

func TestImplement_AffectedScopeWithClassRef(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	testFile := "src/calculator.py"
	testFilePath := filepath.Join(tempDir, testFile)
	os.MkdirAll(filepath.Dir(testFilePath), 0755)
	originalContent := "class Calculator:\n    def add(self, a, b): return a + b"
	os.WriteFile(testFilePath, []byte(originalContent), 0644)

	originalHash := "abc12345"

	drrID := "classref-impl-drr"
	err := tools.DB.CreateHolon(ctx, drrID, "DRR", "system", "DRR",
		"Class Ref Impl Test", "Test DRR", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	// affected_scope has file:class but affected_hashes should have just file path
	contractJSON := fmt.Sprintf(`{"invariants":["Must work"],"affected_scope":["src/calculator.py:Calculator"],"affected_hashes":{"%s":"%s"}}`,
		testFile, originalHash)
	decisionsDir := filepath.Join(tempDir, ".quint", "decisions")
	os.MkdirAll(decisionsDir, 0755)
	drrPath := filepath.Join(decisionsDir, fmt.Sprintf("DRR-2025-01-01-%s.md", drrID))
	drrContent := fmt.Sprintf(`---
title: Class Ref Impl Test
contract: %s
content_hash: abc123
---

# Class Ref Impl Test

Test content.
`, contractJSON)
	if err := os.WriteFile(drrPath, []byte(drrContent), 0644); err != nil {
		t.Fatalf("Failed to write DRR file: %v", err)
	}

	// Modify the file
	modifiedContent := "class Calculator:\n    def add(self, a, b): return a + b  # modified"
	os.WriteFile(testFilePath, []byte(modifiedContent), 0644)

	result, err := tools.Implement(drrID)
	if err != nil {
		t.Fatalf("Implement() failed: %v", err)
	}

	if !strings.Contains(result, "AFFECTED SCOPE CHANGED") {
		t.Error("Missing AFFECTED SCOPE CHANGED warning")
	}
	if !strings.Contains(result, testFile) {
		t.Errorf("Missing affected file in warning: %s", testFile)
	}
}

func TestInternalize_AffectedScopeWarnings(t *testing.T) {
	tools, _, tempDir := setupTools(t)
	ctx := context.Background()

	testFile := "src/target.py"
	testFilePath := filepath.Join(tempDir, testFile)
	os.MkdirAll(filepath.Dir(testFilePath), 0755)
	originalContent := "class Target:\n    def method(self): pass\n"
	os.WriteFile(testFilePath, []byte(originalContent), 0644)

	originalHash := tools.computeFileHash(testFilePath)

	drrID := "internalize-affected-test-drr"
	err := tools.DB.CreateHolon(ctx, drrID, "DRR", "system", "DRR",
		"Affected Scope Test", "Test DRR", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create DRR: %v", err)
	}

	contractJSON := fmt.Sprintf(`{"invariants":["Must work"],"affected_scope":["%s"],"affected_hashes":{"%s":"%s"}}`,
		testFile, testFile, originalHash)
	decisionsDir := filepath.Join(tempDir, ".quint", "decisions")
	os.MkdirAll(decisionsDir, 0755)
	drrPath := filepath.Join(decisionsDir, fmt.Sprintf("DRR-2025-01-01-%s.md", drrID))
	drrContent := fmt.Sprintf(`---
title: Affected Scope Test
contract: %s
content_hash: abc123
---

# Affected Scope Test

Test content.
`, contractJSON)
	if err := os.WriteFile(drrPath, []byte(drrContent), 0644); err != nil {
		t.Fatalf("Failed to write DRR file: %v", err)
	}

	modifiedContent := "class Target:\n    # modified\n    def method(self): pass\n"
	os.WriteFile(testFilePath, []byte(modifiedContent), 0644)

	result, err := tools.Internalize()
	if err != nil {
		t.Fatalf("Internalize() failed: %v", err)
	}

	if !strings.Contains(result, "AFFECTED SCOPE CHANGED") {
		t.Error("Missing AFFECTED SCOPE CHANGED warning in Internalize output")
	}
	if !strings.Contains(result, testFile) {
		t.Errorf("Missing affected file in warning: %s", testFile)
	}
	if !strings.Contains(result, "modified") {
		t.Error("Should indicate file was modified")
	}
}
