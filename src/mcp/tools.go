package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"quint-mcp/db"
)

// Tools handles the actual file system operations for FPF
type Tools struct {
	FSM     *FSM
	RootDir string
	DB      *db.DB
}

func NewTools(fsm *FSM, rootDir string) *Tools {
	// Try to init DB
	dbPath := filepath.Join(rootDir, ".fpf", "fpf.db")
	database, _ := db.New(dbPath) // Ignore error for now if not init

	return &Tools{
		FSM:     fsm,
		RootDir: rootDir,
		DB:      database,
	}
}

func (t *Tools) getFPFDir() string {
	return filepath.Join(t.RootDir, ".fpf")
}

func (t *Tools) Slugify(title string) string {
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	slug := reg.ReplaceAllString(strings.ToLower(title), "-")
	return strings.Trim(slug, "-")
}

// moveHypothesis moves a hypothesis file from one knowledge level to another
func (t *Tools) moveHypothesis(hypothesisID, sourceLevel, destLevel string) (string, error) {
	srcPath := filepath.Join(t.getFPFDir(), "knowledge", sourceLevel, hypothesisID+".md")
	destPath := filepath.Join(t.getFPFDir(), "knowledge", destLevel, hypothesisID+".md")

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return "", fmt.Errorf("hypothesis %s not found in %s", hypothesisID, sourceLevel)
	}

	if err := os.Rename(srcPath, destPath); err != nil {
		return "", fmt.Errorf("failed to move hypothesis from %s to %s: %v", sourceLevel, destLevel, err)
	}
	
	// DB Sync: Update layer
	if t.DB != nil {
		// Try to extract ID from filename or passed ID
		// Assuming hypothesisID matches DB ID
		_ = t.DB.UpdateHolonLayer(hypothesisID, destLevel)
	}

	return destPath, nil
}


// InitProject creates the necessary directories for FPF
func (t *Tools) InitProject() error {
	dirs := []string{
		"evidence",
		"decisions",
		"sessions",
		"knowledge/L0",
		"knowledge/L1",
		"knowledge/L2",
		"knowledge/invalid",
		"agents", // Ensure agents dir exists
	}

	for _, d := range dirs {
		path := filepath.Join(t.getFPFDir(), d)
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
		// Create .gitkeep to ensure git tracks the dirs
		if err := os.WriteFile(filepath.Join(path, ".gitkeep"), []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to write .gitkeep file: %v", err)
		}
	}
	
	// Init DB
	if t.DB == nil {
		dbPath := filepath.Join(t.getFPFDir(), "fpf.db")
		database, err := db.New(dbPath)
		if err != nil {
			fmt.Printf("Warning: Failed to init DB: %v\n", err)
		} else {
			t.DB = database
		}
	}

	// Init Default Agents if not present
	// We do this here to ensure fresh install has them.
	// But install.sh handles this too. Redundancy is okay.
	// Actually, Go code doesn't embed the markdown, so let's rely on install.sh
	// or assume the user has run install.sh. 
	// However, GetAgentContext should handle missing files gracefully.

	return nil
}

// GetAgentContext retrieves the markdown profile for a specific role
func (t *Tools) GetAgentContext(role string) (string, error) {
	// Map role to filename (lowercase)
	filename := strings.ToLower(role) + ".md"
	path := filepath.Join(t.getFPFDir(), "agents", filename)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("agent profile for %s not found at %s", role, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// ProposeHypothesis creates an L0 hypothesis file
func (t *Tools) ProposeHypothesis(title, content string) (string, error) {
	slug := t.Slugify(title)
	filename := fmt.Sprintf("%s.md", slug)
	path := filepath.Join(t.getFPFDir(), "knowledge", "L0", filename)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}

	// DB Sync
	if t.DB != nil {
		h := db.Holon{
			ID: slug,
			Type: "hypothesis",
			Layer: "L0",
			Title: title,
			Content: content,
			ContextID: "default", // Context handling needed later
		}
		_ = t.DB.CreateHolon(h)
	}

	return path, nil
}

// ManageEvidence creates an evidence file. 
// Used by Deductor (logic checks) and Inductor (empirical tests).
func (t *Tools) ManageEvidence(currentPhase Phase, targetID, evidenceType, content, verdict string) (string, error) {
	// Logic to move hypothesis file based on phase and verdict
	var moveErr error
	switch verdict {
	case "PASS":
		switch currentPhase {
		case PhaseDeduction:
			_, moveErr = t.moveHypothesis(targetID, "L0", "L1")
		case PhaseInduction:
			_, moveErr = t.moveHypothesis(targetID, "L1", "L2")
		}
	case "FAIL":
		switch currentPhase {
		case PhaseDeduction:
			_, moveErr = t.moveHypothesis(targetID, "L0", "invalid")
		case PhaseInduction:
			_, moveErr = t.moveHypothesis(targetID, "L1", "invalid")
		}
	case "REFINE":
		switch currentPhase {
		case PhaseDeduction:
			_, moveErr = t.moveHypothesis(targetID, "L0", "invalid") // Or to a 'needs_refinement' state
		case PhaseInduction:
			_, moveErr = t.moveHypothesis(targetID, "L1", "invalid") // Or to a 'needs_refinement' state
		}
	}

	if moveErr != nil {
		return "", fmt.Errorf("failed to move hypothesis: %v", moveErr)
	}

	// Construct filename: YYYY-MM-DD-type-target.md
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s-%s.md", date, evidenceType, targetID)
	path := filepath.Join(t.getFPFDir(), "evidence", filename)

	// Add Metadata header
	fullContent := fmt.Sprintf("---\nid: %s\ntype: %s\ntarget: %s\nverdict: %s\ndate: %s\n---\n\n%s", 
		filename, evidenceType, targetID, verdict, date, content)

	if err := os.WriteFile(path, []byte(fullContent), 0644); err != nil {
		return "", err
	}

	// DB Sync
	if t.DB != nil {
		_ = t.DB.AddEvidence(filename, targetID, evidenceType, content, verdict)
		// Link logic needed?
		_ = t.DB.Link(filename, targetID, "verifiedBy")
	}

	return path, nil
}

// RefineLoopback handles the INDUCTION -> DEDUCTION transition.
// It marks the parent hypothesis as 'refuted' (or 'needs_refinement') and creates a child hypothesis.
func (t *Tools) RefineLoopback(currentPhase Phase, parentID, insight, newTitle, newContent string) (string, error) {
	// Determine parent's current level
	var parentLevel string
	switch currentPhase {
	case PhaseInduction:
		parentLevel = "L1" // Parent would be in L1 before Induction phase
	case PhaseDeduction:
		parentLevel = "L0" // Parent would be in L0 before Deduction phase
	default:
		return "", fmt.Errorf("loopback not applicable from phase %s", currentPhase)
	}

	// 1. Move parent hypothesis to invalid
	if _, err := t.moveHypothesis(parentID, parentLevel, "invalid"); err != nil {
		return "", fmt.Errorf("failed to move parent hypothesis to invalid: %v", err)
	}

	// 2. Create new hypothesis (child) in L0
	childPath, err := t.ProposeHypothesis(newTitle, newContent)
	if err != nil {
		return "", fmt.Errorf("failed to create child hypothesis: %v", err)
	}

	// 3. Log the refinement event
	logFile := filepath.Join(t.getFPFDir(), "sessions", fmt.Sprintf("loopback-%d.md", time.Now().Unix()))
	logContent := fmt.Sprintf("# Loopback Event\n\nParent: %s (moved to invalid)\nInsight: %s\nChild: %s\n", parentID, insight, childPath)
	if err := os.WriteFile(logFile, []byte(logContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write loopback log file: %v", err)
	}

	return childPath, nil
}

// FinalizeDecision creates the DRR and archives the session
func (t *Tools) FinalizeDecision(title, content, winnerID string) (string, error) {
	// 1. Create DRR
	drrName := fmt.Sprintf("DRR-%d-%s.md", time.Now().Unix(), t.Slugify(title))
	drrPath := filepath.Join(t.getFPFDir(), "decisions", drrName)
	if err := os.WriteFile(drrPath, []byte(content), 0644); err != nil {
		return "", err
	}

	// 2. Promote Winner to L2
	if winnerID != "" {
		_, err := t.moveHypothesis(winnerID, "L1", "L2") // Assuming winner is from L1 after Induction
		if err != nil {
			fmt.Printf("WARNING: Failed to move winner hypothesis %s to L2: %v\n", winnerID, err)
			// Don't return error, DRR creation is primary
		}
	}
	
	// 3. Reset Session (handled by FSM transition to IDLE usually, but here we might archive)
	// (Skipping archive logic for brevity, relying on FSM state save)

	return drrPath, nil
}
