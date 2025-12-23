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

func TestCheckPhaseGate_Blocked(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		phase       Phase
		wantBlocked bool
	}{
		// quint_internalize - allowed anywhere (no gate)
		{"internalize_allowed_in_idle", "quint_internalize", PhaseIdle, false},
		{"internalize_allowed_in_abduction", "quint_internalize", PhaseAbduction, false},
		{"internalize_allowed_in_audit", "quint_internalize", PhaseAudit, false},

		// quint_search - allowed anywhere (no gate)
		{"search_allowed_in_idle", "quint_search", PhaseIdle, false},
		{"search_allowed_in_audit", "quint_search", PhaseAudit, false},

		// quint_verify in ABDUCTION or DEDUCTION
		{"verify_blocked_in_idle", "quint_verify", PhaseIdle, true},
		{"verify_allowed_in_abduction", "quint_verify", PhaseAbduction, false},
		{"verify_blocked_in_audit", "quint_verify", PhaseAudit, true},

		// quint_propose - allowed in IDLE, ABD, DED, IND; blocked in AUDIT, DECISION
		{"propose_allowed_in_idle", "quint_propose", PhaseIdle, false},
		{"propose_allowed_in_deduction", "quint_propose", PhaseDeduction, false},
		{"propose_blocked_in_audit", "quint_propose", PhaseAudit, true},
		{"propose_blocked_in_decision", "quint_propose", PhaseDecision, true},

		// quint_audit - only in INDUCTION or AUDIT
		{"audit_blocked_in_idle", "quint_audit", PhaseIdle, true},
		{"audit_allowed_in_induction", "quint_audit", PhaseInduction, false},
		{"audit_blocked_in_decision", "quint_audit", PhaseDecision, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools, _ := setupToolsWithPhase(t, tt.phase)
			err := tools.checkPhaseGate(tt.tool)

			isBlocked := err != nil
			if isBlocked != tt.wantBlocked {
				t.Errorf("checkPhaseGate(%q) in phase %s: blocked=%v, want blocked=%v (err=%v)",
					tt.tool, tt.phase, isBlocked, tt.wantBlocked, err)
			}
		})
	}
}

func TestProposeRegression(t *testing.T) {
	// quint_propose should be allowed in DEDUCTION and INDUCTION (regression case)
	phases := []Phase{PhaseIdle, PhaseAbduction, PhaseDeduction, PhaseInduction}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			tools, _ := setupToolsWithPhase(t, phase)

			args := map[string]string{
				"title":     "Test Hypothesis",
				"content":   "Description",
				"kind":      "system",
				"scope":     "global",
				"rationale": "{}",
			}

			err := tools.CheckPreconditions("quint_propose", args)
			if err != nil {
				t.Errorf("quint_propose should be allowed in %s phase, got error: %v", phase, err)
			}
		})
	}

	// Should be blocked in AUDIT and DECISION
	blockedPhases := []Phase{PhaseAudit, PhaseDecision}
	for _, phase := range blockedPhases {
		t.Run("blocked_in_"+string(phase), func(t *testing.T) {
			tools, _ := setupToolsWithPhase(t, phase)

			args := map[string]string{
				"title":     "Test Hypothesis",
				"content":   "Description",
				"kind":      "system",
				"scope":     "global",
				"rationale": "{}",
			}

			err := tools.CheckPreconditions("quint_propose", args)
			if err == nil {
				t.Errorf("quint_propose should be BLOCKED in %s phase", phase)
			}
		})
	}
}

func TestL2RefreshBypassesPhaseGate(t *testing.T) {
	tools, tempDir := setupToolsWithPhase(t, PhaseIdle) // Start in IDLE

	// Create an L2 holon
	l2HypoID := "l2-refresh-test"
	l2Path := filepath.Join(tempDir, ".quint", "knowledge", "L2", l2HypoID+".md")
	if err := os.WriteFile(l2Path, []byte("L2 content"), 0644); err != nil {
		t.Fatalf("Failed to create L2 hypothesis: %v", err)
	}

	// quint_test on L2 should bypass phase gate (allowed even in IDLE)
	args := map[string]string{
		"hypothesis_id": l2HypoID,
		"test_type":     "internal",
		"result":        "Refreshed test results",
		"verdict":       "PASS",
	}

	err := tools.CheckPreconditions("quint_test", args)
	if err != nil {
		t.Errorf("quint_test on L2 should bypass phase gate, got error: %v", err)
	}
}

func TestL1PromotionRequiresCorrectPhase(t *testing.T) {
	tools, tempDir := setupToolsWithPhase(t, PhaseIdle) // Wrong phase for L1 promotion

	// Create an L1 holon
	l1HypoID := "l1-promotion-test"
	l1Path := filepath.Join(tempDir, ".quint", "knowledge", "L1", l1HypoID+".md")
	if err := os.WriteFile(l1Path, []byte("L1 content"), 0644); err != nil {
		t.Fatalf("Failed to create L1 hypothesis: %v", err)
	}

	// quint_test on L1 should require DEDUCTION or INDUCTION phase
	args := map[string]string{
		"hypothesis_id": l1HypoID,
		"test_type":     "internal",
		"result":        "Test results",
		"verdict":       "PASS",
	}

	err := tools.CheckPreconditions("quint_test", args)
	if err == nil {
		t.Error("quint_test on L1 in IDLE phase should be blocked by phase gate")
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
