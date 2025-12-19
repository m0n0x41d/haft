package fpf

import (
	"os"
	"path/filepath"
	"testing"

	"quint-mcp/db"
)

func TestCheckPreconditions_Propose(t *testing.T) {
	tools, _, _ := setupTools(t)

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
	tools, _, tempDir := setupTools(t)

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
	tools, _, tempDir := setupTools(t)

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
