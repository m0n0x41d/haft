package main

import (
	"fmt"
	"os"
	"os/exec"
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
	dbPath := filepath.Join(rootDir, ".quint", "quint.db")
	database, _ := db.New(dbPath) // Ignore error for now if not init

	return &Tools{
		FSM:     fsm,
		RootDir: rootDir,
		DB:      database,
	}
}

func (t *Tools) getFPFDir() string {
	return filepath.Join(t.RootDir, ".quint")
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
		dbPath := filepath.Join(t.getFPFDir(), "quint.db")
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
func (t *Tools) ProposeHypothesis(title, content, scope, kind string) (string, error) {
	slug := t.Slugify(title)
	filename := fmt.Sprintf("%s.md", slug)
	path := filepath.Join(t.getFPFDir(), "knowledge", "L0", filename)

	// Inject scope and kind into frontmatter if not present
	if !strings.Contains(content, "scope:") {
		content = fmt.Sprintf("---\nscope: %s\nkind: %s\n---\n\n%s", scope, kind, content)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}

	// DB Sync
	if t.DB != nil {
		h := db.Holon{
			ID: slug,
			Type: "hypothesis",
			Kind: kind, // C.3 Kind-CAL
			Layer: "L0",
			Title: title,
			Content: content,
			ContextID: "default", // Context handling needed later
			Scope: scope,
		}
		_ = t.DB.CreateHolon(h)
	}

	return path, nil
}

// ManageEvidence handles evidence operations: add or check.
func (t *Tools) ManageEvidence(currentPhase Phase, action, targetID, evidenceType, content, verdict, assuranceLevel, carrierRef, validUntil string) (string, error) {
	if action == "check" {
		if t.DB == nil {
			return "", fmt.Errorf("DB not initialized")
		}
		if targetID == "all" {
			// TODO: Implement listing all evidence or iterating holons
			return "Global evidence audit not implemented yet. Please specify a target_id.", nil
		}
		evs, err := t.DB.GetEvidence(targetID)
		if err != nil {
			return "", err
		}
		var report string
		for _, e := range evs {
			report += fmt.Sprintf("- [%s] %s (L:%s, Ref:%s): %s\n", e.Verdict, e.Type, e.AssuranceLevel, e.CarrierRef, e.Content)
		}
		if report == "" {
			return "No evidence found for " + targetID, nil
		}
		return report, nil
	}

	// Default to "add" logic
	
	// Assurance Level Logic (B.3)
	// We only promote if the assurance level is sufficient for the transition.
	shouldPromote := false
	
	switch verdict {
	case "PASS":
		switch currentPhase {
		case PhaseDeduction:
			// L0 -> L1 requires Assurance Level L1 (Substantiated)
			if assuranceLevel == "L1" || assuranceLevel == "L2" {
				shouldPromote = true
			} else {
				// verdict PASS but assurance L0? This means "Passed checks but weak". Keep in L0.
				// We do NOT promote.
				// However, we still record the evidence.
			}
		case PhaseInduction:
			// L1 -> L2 requires Assurance Level L2 (Validated/Axiomatic)
			if assuranceLevel == "L2" {
				shouldPromote = true
			} else {
				// verdict PASS but assurance L1? Keep in L1.
			}
		}
	case "FAIL":
		// Fail always demotes to invalid or stays (refutes)
		// Handled below
	}

	var moveErr error
	if verdict == "PASS" && shouldPromote {
		switch currentPhase {
		case PhaseDeduction:
			_, moveErr = t.moveHypothesis(targetID, "L0", "L1")
		case PhaseInduction:
			_, moveErr = t.moveHypothesis(targetID, "L1", "L2")
		}
	} else if verdict == "FAIL" || verdict == "REFINE" {
		// Demotion logic
		switch currentPhase {
		case PhaseDeduction:
			_, moveErr = t.moveHypothesis(targetID, "L0", "invalid")
		case PhaseInduction:
			_, moveErr = t.moveHypothesis(targetID, "L1", "invalid")
		}
	}

	if moveErr != nil {
		return "", fmt.Errorf("failed to move hypothesis: %v", moveErr)
	}

	// Construct filename: YYYY-MM-DD-type-target.md
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s-%s.md", date, evidenceType, targetID)
	path := filepath.Join(t.getFPFDir(), "evidence", filename)

	// Add Metadata header including valid_until
	fullContent := fmt.Sprintf("---\nid: %s\ntype: %s\ntarget: %s\nverdict: %s\nassurance_level: %s\ncarrier_ref: %s\nvalid_until: %s\ndate: %s\n---\n\n%s", 
		filename, evidenceType, targetID, verdict, assuranceLevel, carrierRef, validUntil, date, content)

	if err := os.WriteFile(path, []byte(fullContent), 0644); err != nil {
		return "", err
	}

	// DB Sync
	if t.DB != nil {
		_ = t.DB.AddEvidence(filename, targetID, evidenceType, content, verdict, assuranceLevel, carrierRef, validUntil)
		// Link logic needed?
		_ = t.DB.Link(filename, targetID, "verifiedBy")
	}

	if !shouldPromote && verdict == "PASS" {
		return path + " (Evidence recorded, but Assurance Level insufficient for promotion)", nil
	}
	return path, nil
}

// RefineLoopback handles the INDUCTION -> DEDUCTION transition.
// It marks the parent hypothesis as 'refuted' (or 'needs_refinement') and creates a child hypothesis.
func (t *Tools) RefineLoopback(currentPhase Phase, parentID, insight, newTitle, newContent, scope string) (string, error) {
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
	childPath, err := t.ProposeHypothesis(newTitle, newContent, scope, "system") // Default to system for loopback
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
func (t *Tools) FinalizeDecision(title, winnerID, context, decision, rationale, consequences, characteristics string) (string, error) {
	// 1. Construct DRR Content (E.9 & C.16)
	drrContent := fmt.Sprintf("# %s\n\n", title)
	drrContent += fmt.Sprintf("## Context\n%s\n\n", context)
	drrContent += fmt.Sprintf("## Decision\n**Selected Option:** %s\n\n%s\n\n", winnerID, decision)
	drrContent += fmt.Sprintf("## Rationale\n%s\n\n", rationale)
	if characteristics != "" {
		drrContent += fmt.Sprintf("### Characteristic Space (C.16)\n%s\n\n", characteristics)
	}
	drrContent += fmt.Sprintf("## Consequences\n%s\n", consequences)

	// 2. Create DRR File
	drrName := fmt.Sprintf("DRR-%d-%s.md", time.Now().Unix(), t.Slugify(title))
	drrPath := filepath.Join(t.getFPFDir(), "decisions", drrName)
	if err := os.WriteFile(drrPath, []byte(drrContent), 0644); err != nil {
		return "", err
	}

	// 3. Promote Winner to L2
	if winnerID != "" {
		_, err := t.moveHypothesis(winnerID, "L1", "L2") // Assuming winner is from L1 after Induction
		if err != nil {
			fmt.Printf("WARNING: Failed to move winner hypothesis %s to L2: %v\n", winnerID, err)
			// Don't return error, DRR creation is primary
		}
	}
	
	// 4. Reset Session (handled by FSM transition to IDLE usually, but here we might archive)
	// (Skipping archive logic for brevity, relying on FSM state save)

	return drrPath, nil
}

// Actualize performs maintenance tasks: migration, reconciliation, discovery.
func (t *Tools) Actualize() error {
	// 1. Migration: .fpf -> .quint
	fpfDir := filepath.Join(t.RootDir, ".fpf")
	quintDir := t.getFPFDir()

	// Check if legacy .fpf exists
	if _, err := os.Stat(fpfDir); err == nil {
		fmt.Println("MIGRATION: Found legacy .fpf directory.")
		
		// If .quint also exists, we have a conflict.
		if _, err := os.Stat(quintDir); err == nil {
			return fmt.Errorf("migration conflict: both .fpf and .quint exist. Please resolve manually")
		}

		fmt.Println("MIGRATION: Renaming .fpf -> .quint")
		if err := os.Rename(fpfDir, quintDir); err != nil {
			return fmt.Errorf("failed to rename .fpf: %w", err)
		}
		fmt.Println("MIGRATION: Success.")
	}

	// 2. Database Migration: fpf.db -> quint.db
	legacyDB := filepath.Join(quintDir, "fpf.db")
	newDB := filepath.Join(quintDir, "quint.db")

	if _, err := os.Stat(legacyDB); err == nil {
		fmt.Println("MIGRATION: Found legacy fpf.db.")
		if err := os.Rename(legacyDB, newDB); err != nil {
			return fmt.Errorf("failed to rename fpf.db: %w", err)
		}
		fmt.Println("MIGRATION: Renamed to quint.db.")
	}

	// 3. Reconciliation (Git Drift)
	// Check current git commit
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = t.RootDir
	output, err := cmd.Output()
	if err == nil {
		currentCommit := strings.TrimSpace(string(output))
		lastCommit := t.FSM.State.LastCommit

		if lastCommit == "" {
			fmt.Printf("RECONCILIATION: Initializing baseline commit to %s\n", currentCommit)
			t.FSM.State.LastCommit = currentCommit
			if err := t.FSM.SaveState(filepath.Join(t.getFPFDir(), "state.json")); err != nil {
				fmt.Printf("Warning: Failed to save state: %v\n", err)
			}
		} else if currentCommit != lastCommit {
			fmt.Printf("RECONCILIATION: Detected changes since %s\n", lastCommit)
			diffCmd := exec.Command("git", "diff", "--name-status", lastCommit, "HEAD")
			diffCmd.Dir = t.RootDir
			diffOutput, err := diffCmd.Output()
			if err == nil {
				fmt.Println("Changed files:")
				fmt.Println(string(diffOutput))
			} else {
				fmt.Printf("Warning: Failed to get diff: %v\n", err)
			}
			
			// Update state
			t.FSM.State.LastCommit = currentCommit
			if err := t.FSM.SaveState(filepath.Join(t.getFPFDir(), "state.json")); err != nil {
				fmt.Printf("Warning: Failed to save state: %v\n", err)
			}
		} else {
			fmt.Println("RECONCILIATION: No changes detected (Clean).")
		}
	} else {
		fmt.Println("RECONCILIATION: Not a git repository or git error.")
	}

	return nil
}
