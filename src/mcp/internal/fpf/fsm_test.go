package fpf

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/quint-code/db"
)

func TestLoadState(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	// Test loading non-existent state (should initialize with defaults)
	fsm, err := LoadState("default", database.GetRawDB())
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if fsm.State.AssuranceThreshold != 0.8 {
		t.Errorf("Expected default threshold 0.8, got %f", fsm.State.AssuranceThreshold)
	}

	// Test saving and loading state with role assignment
	fsm.State.ActiveRole = RoleAssignment{Role: RoleAbductor, SessionID: "sess1", Context: "ctx1"}
	fsm.State.LastCommit = "abc123"
	if err := fsm.SaveState("default"); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	fsm2, err := LoadState("default", database.GetRawDB())
	if err != nil {
		t.Fatalf("LoadState failed for existing state: %v", err)
	}
	if fsm2.State.ActiveRole.Role != RoleAbductor {
		t.Errorf("Expected loaded role to be Abductor, got %s", fsm2.State.ActiveRole.Role)
	}
	if fsm2.State.ActiveRole.SessionID != "sess1" {
		t.Errorf("Expected loaded session ID to be sess1, got %s", fsm2.State.ActiveRole.SessionID)
	}
	if fsm2.State.LastCommit != "abc123" {
		t.Errorf("Expected last commit abc123, got %s", fsm2.State.LastCommit)
	}
}

func TestSaveState(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	fsm := &FSM{
		State: State{AssuranceThreshold: 0.75, LastCommit: "abc123"},
		DB:    database.GetRawDB(),
	}
	err = fsm.SaveState("default")
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Verify data was written
	fsm2, err := LoadState("default", database.GetRawDB())
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if fsm2.State.AssuranceThreshold != 0.75 {
		t.Errorf("Expected threshold 0.75, got %f", fsm2.State.AssuranceThreshold)
	}
	if fsm2.State.LastCommit != "abc123" {
		t.Errorf("Expected last commit abc123, got %s", fsm2.State.LastCommit)
	}
}

func TestSaveStateWithoutDB(t *testing.T) {
	fsm := &FSM{State: State{}, DB: nil}
	err := fsm.SaveState("default")
	if err == nil {
		t.Fatalf("Expected SaveState to fail without DB")
	}
}

func TestGetAssuranceThreshold(t *testing.T) {
	tests := []struct {
		name      string
		threshold float64
		expected  float64
	}{
		{"DefaultWhenZero", 0, 0.8},
		{"DefaultWhenNegative", -0.5, 0.8},
		{"CustomThreshold", 0.75, 0.75},
		{"MinThreshold", 0.1, 0.1},
		{"MaxThreshold", 1.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm := &FSM{State: State{AssuranceThreshold: tt.threshold}}
			result := fsm.GetAssuranceThreshold()
			if result != tt.expected {
				t.Errorf("GetAssuranceThreshold() got %f, expected %f", result, tt.expected)
			}
		})
	}
}

func TestGetContextStage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer database.Close()

	fsm := &FSM{
		State: State{AssuranceThreshold: 0.8},
		DB:    database.GetRawDB(),
	}

	ctx := context.Background()

	// Create a decision context
	err = database.CreateHolon(ctx, "test-context", "decision_context", "system", "L0", "Test Context", "Test content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	// Initially: StageEmpty (no hypotheses)
	stage := fsm.GetContextStage("test-context")
	if stage != StageEmpty {
		t.Errorf("Expected StageEmpty with no hypotheses, got %s", stage)
	}

	// Add L0 hypothesis with memberOf relation
	err = database.CreateHolon(ctx, "h1", "hypothesis", "system", "L0", "Test Hypo", "Test content", "default", "", "")
	if err != nil {
		t.Fatalf("Failed to create holon: %v", err)
	}
	err = database.CreateRelation(ctx, "h1", "memberOf", "test-context", 3)
	if err != nil {
		t.Fatalf("Failed to create relation: %v", err)
	}

	stage = fsm.GetContextStage("test-context")
	if stage != StageNeedsVerify {
		t.Errorf("Expected StageNeedsVerify with L0 hypothesis, got %s", stage)
	}

	// Promote to L1
	err = database.UpdateHolonLayer(ctx, "h1", "L1")
	if err != nil {
		t.Fatalf("Failed to update layer: %v", err)
	}

	stage = fsm.GetContextStage("test-context")
	if stage != StageNeedsValidation {
		t.Errorf("Expected StageNeedsValidation with L1 hypothesis, got %s", stage)
	}

	// Promote to L2
	err = database.UpdateHolonLayer(ctx, "h1", "L2")
	if err != nil {
		t.Fatalf("Failed to update layer: %v", err)
	}

	stage = fsm.GetContextStage("test-context")
	if stage != StageNeedsAudit {
		t.Errorf("Expected StageNeedsAudit with L2 hypothesis, got %s", stage)
	}

	// Add audit report evidence
	err = database.AddEvidence(ctx, "e1", "h1", "audit_report", "Audit passed", "PASS", "L2", "auditor", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create evidence: %v", err)
	}

	stage = fsm.GetContextStage("test-context")
	if stage != StageReadyToDecide {
		t.Errorf("Expected StageReadyToDecide with audited L2, got %s", stage)
	}
}

func TestGetContextStageWithoutDB(t *testing.T) {
	fsm := &FSM{State: State{}, DB: nil}
	stage := fsm.GetContextStage("any-context")
	if stage != StageEmpty {
		t.Errorf("Expected StageEmpty when DB is nil, got %s", stage)
	}
}

func TestGetContextStageDescription(t *testing.T) {
	tests := []struct {
		stage           ContextStage
		wantDescription string
		wantNextAction  string
	}{
		{StageEmpty, "No hypotheses yet", "Use /q1-hypothesize to add hypotheses"},
		{StageNeedsVerify, "Hypotheses need verification", "Use /q2-verify to verify L0 hypotheses"},
		{StageNeedsValidation, "Hypotheses need validation", "Use /q3-validate to test L1 hypotheses"},
		{StageNeedsAudit, "L2 hypotheses need audit", "Use /q4-audit to audit L2 hypotheses"},
		{StageReadyToDecide, "Ready for decision", "Use /q5-decide to finalize decision"},
	}

	for _, tt := range tests {
		t.Run(string(tt.stage), func(t *testing.T) {
			desc, next := GetContextStageDescription(tt.stage)
			if desc != tt.wantDescription {
				t.Errorf("GetContextStageDescription(%s) description = %q, want %q", tt.stage, desc, tt.wantDescription)
			}
			if next != tt.wantNextAction {
				t.Errorf("GetContextStageDescription(%s) nextAction = %q, want %q", tt.stage, next, tt.wantNextAction)
			}
		})
	}
}
