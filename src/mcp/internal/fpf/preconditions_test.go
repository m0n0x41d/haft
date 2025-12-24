package fpf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/quint-code/db"
)

func TestCheckPreconditions_Propose(t *testing.T) {
	// Use PhaseIdle - quint_propose is allowed in IDLE
	tools, _ := setupToolsWithPhase(t, PhaseIdle)

	tests := []struct {
		name    string
		args    map[string]string
		wantErr bool
	}{
		{
			name: "valid proposal",
			args: map[string]string{
				"title":     "Test Hypothesis",
				"content":   "Description",
				"kind":      "system",
				"scope":     "global",
				"rationale": "{}",
			},
			wantErr: false,
		},
		{
			name: "missing title",
			args: map[string]string{
				"content":   "Description",
				"kind":      "system",
				"scope":     "global",
				"rationale": "{}",
			},
			wantErr: true,
		},
		{
			name: "missing content",
			args: map[string]string{
				"title":     "Test",
				"kind":      "system",
				"scope":     "global",
				"rationale": "{}",
			},
			wantErr: true,
		},
		{
			name: "invalid kind",
			args: map[string]string{
				"title":     "Test",
				"content":   "Description",
				"kind":      "invalid",
				"scope":     "global",
				"rationale": "{}",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tools.CheckPreconditions("quint_propose", tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPreconditions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckPreconditions_Verify(t *testing.T) {
	// quint_verify requires ABDUCTION or DEDUCTION phase
	tools, tempDir := setupToolsWithPhase(t, PhaseAbduction)

	hypoID := "test-hypo"
	l0Path := filepath.Join(tempDir, ".quint", "knowledge", "L0", hypoID+".md")
	if err := os.WriteFile(l0Path, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test hypothesis: %v", err)
	}

	tests := []struct {
		name    string
		args    map[string]string
		wantErr bool
	}{
		{
			name: "valid verify with existing L0 hypo",
			args: map[string]string{
				"hypothesis_id": hypoID,
				"checks_json":   "{}",
				"verdict":       "PASS",
			},
			wantErr: false,
		},
		{
			name: "missing hypothesis_id",
			args: map[string]string{
				"checks_json": "{}",
				"verdict":     "PASS",
			},
			wantErr: true,
		},
		{
			name: "non-existent hypothesis",
			args: map[string]string{
				"hypothesis_id": "non-existent",
				"checks_json":   "{}",
				"verdict":       "PASS",
			},
			wantErr: true,
		},
		{
			name: "invalid verdict",
			args: map[string]string{
				"hypothesis_id": hypoID,
				"checks_json":   "{}",
				"verdict":       "INVALID",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tools.CheckPreconditions("quint_verify", tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPreconditions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckPreconditions_Test(t *testing.T) {
	// quint_test requires DEDUCTION or INDUCTION phase
	tools, tempDir := setupToolsWithPhase(t, PhaseDeduction)

	l0HypoID := "l0-hypo"
	l0Path := filepath.Join(tempDir, ".quint", "knowledge", "L0", l0HypoID+".md")
	if err := os.WriteFile(l0Path, []byte("L0 content"), 0644); err != nil {
		t.Fatalf("Failed to create L0 hypothesis: %v", err)
	}

	l1HypoID := "l1-hypo"
	l1Path := filepath.Join(tempDir, ".quint", "knowledge", "L1", l1HypoID+".md")
	if err := os.WriteFile(l1Path, []byte("L1 content"), 0644); err != nil {
		t.Fatalf("Failed to create L1 hypothesis: %v", err)
	}

	tests := []struct {
		name    string
		args    map[string]string
		wantErr bool
	}{
		{
			name: "valid test with L1 hypo",
			args: map[string]string{
				"hypothesis_id": l1HypoID,
				"test_type":     "internal",
				"result":        "All tests pass",
				"verdict":       "PASS",
			},
			wantErr: false,
		},
		{
			name: "hypothesis still in L0",
			args: map[string]string{
				"hypothesis_id": l0HypoID,
				"test_type":     "internal",
				"result":        "Test result",
				"verdict":       "PASS",
			},
			wantErr: true,
		},
		{
			name: "missing hypothesis_id",
			args: map[string]string{
				"test_type": "internal",
				"result":    "Test result",
				"verdict":   "PASS",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tools.CheckPreconditions("quint_test", tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPreconditions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckPreconditions_Decide(t *testing.T) {
	tempDir := t.TempDir()
	quintDir := filepath.Join(tempDir, ".quint")
	os.MkdirAll(filepath.Join(quintDir, "knowledge", "L0"), 0755)
	os.MkdirAll(filepath.Join(quintDir, "knowledge", "L1"), 0755)
	os.MkdirAll(filepath.Join(quintDir, "knowledge", "L2"), 0755)
	os.MkdirAll(filepath.Join(quintDir, "decisions"), 0755)

	dbPath := filepath.Join(quintDir, "quint.db")
	store, _ := db.NewStore(dbPath)
	defer store.Close()

	fsm := &FSM{State: State{Phase: PhaseDecision}}
	tools := NewTools(fsm, tempDir, store)

	tests := []struct {
		name    string
		args    map[string]string
		setup   func()
		wantErr bool
	}{
		{
			name: "missing winner_id",
			args: map[string]string{
				"title":        "Test Decision",
				"context":      "ctx",
				"decision":     "dec",
				"rationale":    "rat",
				"consequences": "con",
			},
			wantErr: true,
		},
		{
			name: "missing title",
			args: map[string]string{
				"winner_id":    "test",
				"context":      "ctx",
				"decision":     "dec",
				"rationale":    "rat",
				"consequences": "con",
			},
			wantErr: true,
		},
		{
			name: "no L2 hypotheses",
			args: map[string]string{
				"title":        "Test Decision",
				"winner_id":    "test",
				"context":      "ctx",
				"decision":     "dec",
				"rationale":    "rat",
				"consequences": "con",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			err := tools.CheckPreconditions("quint_decide", tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPreconditions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckPreconditions_CalculateR(t *testing.T) {
	tempDir := t.TempDir()
	quintDir := filepath.Join(tempDir, ".quint")
	os.MkdirAll(quintDir, 0755)

	dbPath := filepath.Join(quintDir, "quint.db")
	store, _ := db.NewStore(dbPath)
	defer store.Close()

	store.CreateHolon(ctx, "existing-holon", "hypothesis", "system", "L0", "Test", "Content", "default", "", "")

	fsm := &FSM{State: State{Phase: PhaseIdle}}
	tools := NewTools(fsm, tempDir, store)

	tests := []struct {
		name    string
		args    map[string]string
		wantErr bool
	}{
		{
			name:    "missing holon_id",
			args:    map[string]string{},
			wantErr: true,
		},
		{
			name: "non-existent holon",
			args: map[string]string{
				"holon_id": "non-existent",
			},
			wantErr: true,
		},
		{
			name: "existing holon",
			args: map[string]string{
				"holon_id": "existing-holon",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tools.CheckPreconditions("quint_calculate_r", tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPreconditions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPreconditionError_Format(t *testing.T) {
	err := &PreconditionError{
		Tool:       "quint_verify",
		Condition:  "hypothesis not found",
		Suggestion: "Create a hypothesis first",
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error string should not be empty")
	}
	if !containsString(errStr, "quint_verify") {
		t.Error("Error should contain tool name")
	}
	if !containsString(errStr, "hypothesis not found") {
		t.Error("Error should contain condition")
	}
	if !containsString(errStr, "Create a hypothesis first") {
		t.Error("Error should contain suggestion")
	}
}

// setupToolsWithPhase creates a Tools instance with a specific phase.
// Note: FSM.DB is set to nil so GetPhase() returns State.Phase directly
// instead of deriving from DB contents. This allows testing phase gates in isolation.
func setupToolsWithPhase(t *testing.T, phase Phase) (*Tools, string) {
	tempDir := t.TempDir()
	quintDir := filepath.Join(tempDir, ".quint")
	os.MkdirAll(filepath.Join(quintDir, "knowledge", "L0"), 0755)
	os.MkdirAll(filepath.Join(quintDir, "knowledge", "L1"), 0755)
	os.MkdirAll(filepath.Join(quintDir, "knowledge", "L2"), 0755)
	os.MkdirAll(filepath.Join(quintDir, "decisions"), 0755)

	dbPath := filepath.Join(quintDir, "quint.db")
	store, err := db.NewStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}

	// FSM.DB = nil so GetPhase() returns State.Phase directly
	// (not derived from DB contents)
	fsm := &FSM{State: State{Phase: phase}, DB: nil}
	tools := NewTools(fsm, tempDir, store)

	return tools, tempDir
}

// TestNoPhaseGates verifies that all tools are allowed in any phase.
// Phase gates were removed - semantic preconditions are sufficient.
// See roles.go for design decision.
func TestNoPhaseGates(t *testing.T) {
	tools, tempDir := setupToolsWithPhase(t, PhaseIdle)

	// Create test holons in appropriate layers
	l0HypoID := "l0-test"
	l0Path := filepath.Join(tempDir, ".quint", "knowledge", "L0", l0HypoID+".md")
	os.WriteFile(l0Path, []byte("L0 content"), 0644)

	l1HypoID := "l1-test"
	l1Path := filepath.Join(tempDir, ".quint", "knowledge", "L1", l1HypoID+".md")
	os.WriteFile(l1Path, []byte("L1 content"), 0644)

	l2HypoID := "l2-test"
	l2Path := filepath.Join(tempDir, ".quint", "knowledge", "L2", l2HypoID+".md")
	os.WriteFile(l2Path, []byte("L2 content"), 0644)

	tests := []struct {
		name    string
		tool    string
		args    map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "propose allowed in any phase",
			tool: "quint_propose",
			args: map[string]string{
				"title":   "Test",
				"content": "Content",
				"kind":    "system",
			},
			wantErr: false,
		},
		{
			name: "verify allowed - checks L0 existence",
			tool: "quint_verify",
			args: map[string]string{
				"hypothesis_id": l0HypoID,
				"checks_json":   "{}",
				"verdict":       "PASS",
			},
			wantErr: false,
		},
		{
			name: "test allowed on L1",
			tool: "quint_test",
			args: map[string]string{
				"hypothesis_id": l1HypoID,
				"test_type":     "internal",
				"result":        "Pass",
				"verdict":       "PASS",
			},
			wantErr: false,
		},
		{
			name: "test allowed on L2 (refresh)",
			tool: "quint_test",
			args: map[string]string{
				"hypothesis_id": l2HypoID,
				"test_type":     "internal",
				"result":        "Refreshed",
				"verdict":       "PASS",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tools.CheckPreconditions(tt.tool, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPreconditions(%s) error = %v, wantErr %v", tt.tool, err, tt.wantErr)
			}
		})
	}
}

// TestSemanticPreconditionsEnforced verifies that semantic checks still work.
// These are the real guards - not phase gates.
func TestSemanticPreconditionsEnforced(t *testing.T) {
	tools, tempDir := setupToolsWithPhase(t, PhaseIdle)

	// Create an L0 holon only
	l0HypoID := "l0-only"
	l0Path := filepath.Join(tempDir, ".quint", "knowledge", "L0", l0HypoID+".md")
	os.WriteFile(l0Path, []byte("L0 content"), 0644)

	tests := []struct {
		name    string
		tool    string
		args    map[string]string
		wantErr bool
	}{
		{
			name: "verify rejects non-L0 hypothesis",
			tool: "quint_verify",
			args: map[string]string{
				"hypothesis_id": "non-existent",
				"checks_json":   "{}",
				"verdict":       "PASS",
			},
			wantErr: true,
		},
		{
			name: "test rejects L0 hypothesis (must be L1 or L2)",
			tool: "quint_test",
			args: map[string]string{
				"hypothesis_id": l0HypoID,
				"test_type":     "internal",
				"result":        "Test",
				"verdict":       "PASS",
			},
			wantErr: true,
		},
		{
			name: "test rejects non-existent hypothesis",
			tool: "quint_test",
			args: map[string]string{
				"hypothesis_id": "non-existent",
				"test_type":     "internal",
				"result":        "Test",
				"verdict":       "PASS",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tools.CheckPreconditions(tt.tool, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPreconditions(%s) error = %v, wantErr %v", tt.tool, err, tt.wantErr)
			}
		})
	}
}

func TestSearchPreconditions(t *testing.T) {
	tools, _ := setupToolsWithPhase(t, PhaseIdle)

	tests := []struct {
		name    string
		args    map[string]string
		wantErr bool
	}{
		{
			name: "valid search",
			args: map[string]string{
				"query": "authentication",
			},
			wantErr: false,
		},
		{
			name: "missing query",
			args: map[string]string{},
			wantErr: true,
		},
		{
			name: "empty query",
			args: map[string]string{
				"query": "",
			},
			wantErr: true,
		},
		{
			name: "search with filters",
			args: map[string]string{
				"query":        "caching",
				"layer_filter": "L2",
				"scope":        "decisions",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tools.CheckPreconditions("quint_search", tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPreconditions(quint_search) error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
