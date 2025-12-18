package fpf_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"quint-mcp/db"
	"quint-mcp/internal/fpf"
)

func TestActualize_GitReconciliation(t *testing.T) {
	// 1. Setup a temp dir that is also a git repo
	tempDir := t.TempDir()
	
	// Initialize git repo
	runGit := func(args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		return cmd.Run()
	}

	if err := runGit("init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if err := runGit("config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config email failed: %v", err)
	}
	if err := runGit("config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config name failed: %v", err)
	}

	// Create initial file and commit
	initialFile := filepath.Join(tempDir, "initial.txt")
	if err := os.WriteFile(initialFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}
	if err := runGit("add", "initial.txt"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := runGit("commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// 2. Initialize FPF Tools
	quintDir := filepath.Join(tempDir, ".quint")
	if err := os.MkdirAll(quintDir, 0755); err != nil {
		t.Fatalf("Failed to create .quint dir: %v", err)
	}
	dbPath := filepath.Join(quintDir, "quint.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	
	fsm := &fpf.FSM{
		State: fpf.State{Phase: fpf.PhaseIdle},
		DB:    database.GetRawDB(),
	}
	tools := fpf.NewTools(fsm, tempDir, database)

	// 3. First Actualize call: Should initialize baseline
	report1, err := tools.Actualize()
	if err != nil {
		t.Fatalf("First Actualize failed: %v", err)
	}
	if !strings.Contains(report1, "Initializing baseline commit") {
		t.Errorf("Expected 'Initializing baseline commit', got: %s", report1)
	}

	// 4. Modify file and commit
	if err := os.WriteFile(initialFile, []byte("changed"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}
	if err := runGit("add", "initial.txt"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := runGit("commit", "-m", "Changed file"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// 5. Second Actualize call: Should detect changes
	report2, err := tools.Actualize()
	if err != nil {
		t.Fatalf("Second Actualize failed: %v", err)
	}
	if !strings.Contains(report2, "Detected changes since") {
		t.Errorf("Expected 'Detected changes since', got: %s", report2)
	}
	if !strings.Contains(report2, "initial.txt") {
		t.Errorf("Expected report to list 'initial.txt', got: %s", report2)
	}

	// 6. Third Actualize call: Should be clean
	report3, err := tools.Actualize()
	if err != nil {
		t.Fatalf("Third Actualize failed: %v", err)
	}
	if !strings.Contains(report3, "No changes detected (Clean)") {
		t.Errorf("Expected 'No changes detected', got: %s", report3)
	}
}

func TestActualize_LegacyMigration(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create legacy .fpf dir and fpf.db
	legacyDir := filepath.Join(tempDir, ".fpf")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatalf("Failed to create .fpf dir: %v", err)
	}
	legacyDBPath := filepath.Join(legacyDir, "fpf.db")
	if err := os.WriteFile(legacyDBPath, []byte("dummy db content"), 0644); err != nil {
		t.Fatalf("Failed to create dummy fpf.db: %v", err)
	}

	// Initialize tools (pointing to root, not existing .quint yet)
	// We pass nil DB initially to let tools handle it, or create a temporary one if needed by NewTools logic
	// But Actualize handles .quint creation. Let's see if NewTools fails if .quint doesn't exist.
	// NewTools attempts to open DB. If it fails, it prints warning but continues.
	
	fsm := &fpf.FSM{State: fpf.State{Phase: fpf.PhaseIdle}}
	tools := fpf.NewTools(fsm, tempDir, nil)

	// Run Actualize
	report, err := tools.Actualize()
	if err != nil {
		t.Fatalf("Actualize failed during migration: %v", err)
	}

	// Check report
	if !strings.Contains(report, "Renaming .fpf -> .quint") {
		t.Errorf("Report missing .fpf rename msg: %s", report)
	}
	if !strings.Contains(report, "Renamed to quint.db") {
		t.Errorf("Report missing fpf.db rename msg: %s", report)
	}

	// Verify filesystem changes
	if _, err := os.Stat(filepath.Join(tempDir, ".fpf")); !os.IsNotExist(err) {
		t.Errorf(".fpf directory still exists")
	}
	if _, err := os.Stat(filepath.Join(tempDir, ".quint")); os.IsNotExist(err) {
		t.Errorf(".quint directory not created")
	}
	if _, err := os.Stat(filepath.Join(tempDir, ".quint", "quint.db")); os.IsNotExist(err) {
		t.Errorf("quint.db not found")
	}
}
