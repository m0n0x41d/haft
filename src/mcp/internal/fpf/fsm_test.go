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
	err = database.CreateHolon(ctx, "test-context", "decision_context", "system", "L0", "Test Context", "Test content", "default", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}

	// Initially: StageEmpty (no hypotheses)
	stage := fsm.GetContextStage("test-context")
	if stage != StageEmpty {
		t.Errorf("Expected StageEmpty with no hypotheses, got %s", stage)
	}

	// Add L0 hypothesis with memberOf relation
	err = database.CreateHolon(ctx, "h1", "hypothesis", "system", "L0", "Test Hypo", "Test content", "default", "", "", "")
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
	err = database.AddEvidence(ctx, "e1", "h1", "audit_report", "Audit passed", "PASS", "L2", 5, "auditor", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create evidence: %v", err)
	}

	stage = fsm.GetContextStage("test-context")
	if stage != StageReadyToDecide {
		t.Errorf("Expected StageReadyToDecide with audited L2, got %s", stage)
	}
}

func TestGetContextStageMixedLayers(t *testing.T) {
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

	t.Run("L0_plus_L2_returns_StageNeedsVerify", func(t *testing.T) {
		err := database.CreateHolon(ctx, "ctx-mixed-1", "decision_context", "system", "L0", "Mixed Context 1", "", "default", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create context: %v", err)
		}

		err = database.CreateHolon(ctx, "h-l0", "hypothesis", "system", "L0", "L0 Hypothesis", "", "default", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create L0 holon: %v", err)
		}
		err = database.CreateRelation(ctx, "h-l0", "memberOf", "ctx-mixed-1", 3)
		if err != nil {
			t.Fatalf("Failed to create relation: %v", err)
		}

		err = database.CreateHolon(ctx, "h-l2", "hypothesis", "system", "L2", "L2 Hypothesis", "", "default", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create L2 holon: %v", err)
		}
		err = database.CreateRelation(ctx, "h-l2", "memberOf", "ctx-mixed-1", 3)
		if err != nil {
			t.Fatalf("Failed to create relation: %v", err)
		}

		stage := fsm.GetContextStage("ctx-mixed-1")
		if stage != StageNeedsVerify {
			t.Errorf("L0 + L2: expected StageNeedsVerify, got %s", stage)
		}
	})

	t.Run("L1_plus_L2_returns_StageNeedsValidation", func(t *testing.T) {
		err := database.CreateHolon(ctx, "ctx-mixed-2", "decision_context", "system", "L0", "Mixed Context 2", "", "default", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create context: %v", err)
		}

		err = database.CreateHolon(ctx, "h-l1", "hypothesis", "system", "L1", "L1 Hypothesis", "", "default", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create L1 holon: %v", err)
		}
		err = database.CreateRelation(ctx, "h-l1", "memberOf", "ctx-mixed-2", 3)
		if err != nil {
			t.Fatalf("Failed to create relation: %v", err)
		}

		err = database.CreateHolon(ctx, "h-l2-2", "hypothesis", "system", "L2", "L2 Hypothesis 2", "", "default", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create L2 holon: %v", err)
		}
		err = database.CreateRelation(ctx, "h-l2-2", "memberOf", "ctx-mixed-2", 3)
		if err != nil {
			t.Fatalf("Failed to create relation: %v", err)
		}

		stage := fsm.GetContextStage("ctx-mixed-2")
		if stage != StageNeedsValidation {
			t.Errorf("L1 + L2: expected StageNeedsValidation, got %s", stage)
		}
	})

	t.Run("Only_L2_audited_returns_StageReadyToDecide", func(t *testing.T) {
		err := database.CreateHolon(ctx, "ctx-l2-only", "decision_context", "system", "L0", "L2 Only Context", "", "default", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create context: %v", err)
		}

		err = database.CreateHolon(ctx, "h-l2-audited", "hypothesis", "system", "L2", "Audited L2", "", "default", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create L2 holon: %v", err)
		}
		err = database.CreateRelation(ctx, "h-l2-audited", "memberOf", "ctx-l2-only", 3)
		if err != nil {
			t.Fatalf("Failed to create relation: %v", err)
		}

		err = database.AddEvidence(ctx, "e-audit", "h-l2-audited", "audit_report", "Audit passed", "PASS", "L2", 5, "auditor", "", "", "")
		if err != nil {
			t.Fatalf("Failed to add audit evidence: %v", err)
		}

		stage := fsm.GetContextStage("ctx-l2-only")
		if stage != StageReadyToDecide {
			t.Errorf("Only L2 audited: expected StageReadyToDecide, got %s", stage)
		}
	})

	t.Run("L2_not_audited_returns_StageNeedsAudit", func(t *testing.T) {
		err := database.CreateHolon(ctx, "ctx-l2-unaudited", "decision_context", "system", "L0", "L2 Unaudited Context", "", "default", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create context: %v", err)
		}

		err = database.CreateHolon(ctx, "h-l2-unaudited", "hypothesis", "system", "L2", "Unaudited L2", "", "default", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create L2 holon: %v", err)
		}
		err = database.CreateRelation(ctx, "h-l2-unaudited", "memberOf", "ctx-l2-unaudited", 3)
		if err != nil {
			t.Fatalf("Failed to create relation: %v", err)
		}

		stage := fsm.GetContextStage("ctx-l2-unaudited")
		if stage != StageNeedsAudit {
			t.Errorf("L2 not audited: expected StageNeedsAudit, got %s", stage)
		}
	})
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
