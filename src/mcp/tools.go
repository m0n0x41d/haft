package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"quint-mcp/assurance"
	"quint-mcp/db"
)

// Tools handles the actual file system operations for FPF
type Tools struct {
	FSM     *FSM
	RootDir string
	DB      *db.DB
}

func NewTools(fsm *FSM, rootDir string, database *db.DB) *Tools {
	// If database is nil, try to init (fallback for some cases, though main should handle it)
	if database == nil {
		dbPath := filepath.Join(rootDir, ".quint", "quint.db")
		database, _ = db.New(dbPath) 
	}

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
	
	// Init DB if not already
	if t.DB == nil {
		dbPath := filepath.Join(t.getFPFDir(), "quint.db")
		database, err := db.New(dbPath)
		if err != nil {
			fmt.Printf("Warning: Failed to init DB: %v\n", err)
		} else {
			t.DB = database
		}
	}

	return nil
}

// RecordContext saves the Bounded Context definition
func (t *Tools) RecordContext(vocabulary, invariants string) (string, error) {
	content := fmt.Sprintf("# Bounded Context\n\n## Vocabulary\n%s\n\n## Invariants\n%s\n", vocabulary, invariants)
	path := filepath.Join(t.getFPFDir(), "context.md")
	
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
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

// RecordWork logs the execution of a method to the DB
func (t *Tools) RecordWork(methodName string, start time.Time) {
	if t.DB == nil {
		return
	}
	end := time.Now()
	id := fmt.Sprintf("work-%d", start.UnixNano())
	
	performer := string(t.FSM.State.ActiveRole.Role)
	if performer == "" {
		performer = "System"
	}
	
	// Simple ledger for now
	ledger := fmt.Sprintf(`{"duration_ms": %d}`, end.Sub(start).Milliseconds())
	
	_ = t.DB.RecordWork(id, methodName, performer, start, end, ledger)
}

// ProposeHypothesis creates an L0 hypothesis file
func (t *Tools) ProposeHypothesis(title, content, scope, kind, rationale string) (string, error) {
	defer t.RecordWork("ProposeHypothesis", time.Now())

	slug := t.Slugify(title)

	filename := fmt.Sprintf("%s.md", slug)

	path := filepath.Join(t.getFPFDir(), "knowledge", "L0", filename)

	// Inject scope, kind, and rationale into frontmatter/content
	fileContent := fmt.Sprintf("---\nscope: %s\nkind: %s\n---\n\n# Hypothesis: %s\n\n%s\n\n## Rationale\n%s", scope, kind, title, content, rationale)

	if err := os.WriteFile(path, []byte(fileContent), 0644); err != nil {
		return "", err
	}

	// DB Sync
	if t.DB != nil {
		h := db.Holon{
			ID:        slug,
			Type:      "hypothesis",
			Kind:      kind,
			Layer:     "L0",
			Title:     title,
			Content:   fileContent, // Storing full content including rationale
			ContextID: "default",
			Scope:     scope,
		}
		_ = t.DB.CreateHolon(h)
	}

	return path, nil
}

// VerifyHypothesis promotes a hypothesis to L1 if checks pass
func (t *Tools) VerifyHypothesis(hypothesisID, checksJSON, verdict string) (string, error) {
	defer t.RecordWork("VerifyHypothesis", time.Now())
	
	if verdict == "PASS" {
		_, err := t.moveHypothesis(hypothesisID, "L0", "L1")
		if err != nil {
			return "", err
		}
		
		evidenceContent := fmt.Sprintf("Verification Checks:\n%s", checksJSON)
		_, _ = t.ManageEvidence(PhaseDeduction, "add", hypothesisID, "verification", evidenceContent, "PASS", "L1", "internal-logic", "")
		
		return fmt.Sprintf("Hypothesis %s promoted to L1", hypothesisID), nil
	} else if verdict == "FAIL" {
		_, err := t.moveHypothesis(hypothesisID, "L0", "invalid")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Hypothesis %s moved to invalid", hypothesisID), nil
	} else if verdict == "REFINE" {
		// Keep in L0, append notes?
		// For now just return status
		return fmt.Sprintf("Hypothesis %s requires refinement (staying in L0)", hypothesisID), nil
	}

	return "", fmt.Errorf("unknown verdict: %s", verdict)
}

// AuditEvidence calculates R_eff and records audit results
func (t *Tools) AuditEvidence(hypothesisID, risks string) (string, error) {
	defer t.RecordWork("AuditEvidence", time.Now())
    
    _, err := t.ManageEvidence(PhaseDecision, "add", hypothesisID, "audit_report", risks, "PASS", "L2", "auditor", "")
    return "Audit recorded for " + hypothesisID, err
}

// ManageEvidence handles evidence operations: add or check.
func (t *Tools) ManageEvidence(currentPhase Phase, action, targetID, evidenceType, content, verdict, assuranceLevel, carrierRef, validUntil string) (string, error) {
	defer t.RecordWork("ManageEvidence", time.Now())
	if action == "check" {
		if t.DB == nil {
			return "", fmt.Errorf("DB not initialized")
		}
		if targetID == "all" {
			// TODO: Implement listing all evidence or iterating holons
			return "Global evidence audit not implemented yet. Please specify a target_id.", nil
		}
		ev, err := t.DB.GetEvidence(targetID)
		if err != nil {
			return "", err
		}
		var report string
		for _, e := range ev {
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
			}
		case PhaseInduction:
			// L1 -> L2 requires Assurance Level L2 (Validated/Axiomatic)
			if assuranceLevel == "L2" {
				shouldPromote = true
			}
		}
	}

	var moveErr error
	if verdict == "PASS" && shouldPromote {
		switch currentPhase {
		case PhaseDeduction:
			_, moveErr = t.moveHypothesis(targetID, "L0", "L1")
		case PhaseInduction:
			// Check if it's still in L0
			if _, err := os.Stat(filepath.Join(t.getFPFDir(), "knowledge", "L0", targetID+".md")); err == nil {
				return "", fmt.Errorf("Hypothesis %s is still in L0. Run /q2-verify to promote it to L1 before testing.", targetID)
			}
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
	defer t.RecordWork("RefineLoopback", time.Now())
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
	rationale := fmt.Sprintf(`{"source": "loopback", "parent_id": "%s", "insight": "%s"}`, parentID, insight)
	childPath, err := t.ProposeHypothesis(newTitle, newContent, scope, "system", rationale) // Default to system for loopback
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
	defer t.RecordWork("FinalizeDecision", time.Now())
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
	
	return drrPath, nil
}

// RunDecay updates R-scores for all holons based on evidence decay
func (t *Tools) RunDecay() error {
	defer t.RecordWork("RunDecay", time.Now())
	if t.DB == nil {
		return fmt.Errorf("DB not initialized")
	}
	
	// Get all holon IDs

rows, err := t.DB.GetRawDB().Query("SELECT id FROM holons")
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}

	calc := assurance.New(t.DB.GetRawDB())
	updatedCount := 0
	
	for _, id := range ids {
		report, err := calc.CalculateReliability(context.Background(), id)
		if err != nil {
			fmt.Printf("Error calculating R for %s: %v\n", id, err)
			continue
		}
		// CalculateReliability already updates the cache in DB
		if report.FinalScore < 1.0 { // Just logging interesting ones
			// fmt.Printf("Holon %s R-score updated to %.2f\n", id, report.FinalScore)
		}
		updatedCount++
	}
	
	fmt.Printf("Decay update complete. Processed %d holons.\n", updatedCount)
	return nil
}

// VisualizeAudit generates an ASCII tree of dependencies with R-scores
func (t *Tools) VisualizeAudit(rootID string) (string, error) {
	defer t.RecordWork("VisualizeAudit", time.Now())
	if t.DB == nil {
		return "", fmt.Errorf("DB not initialized")
	}
	
	if rootID == "all" {
		// List top-level holons (those that are not components of others?)
		// For simplicity, just list all with R scores
		// This might be too verbose.
		return "Please specify a root ID for the audit tree.", nil
	}

	calc := assurance.New(t.DB.GetRawDB())
	return t.buildAuditTree(rootID, 0, calc)
}

func (t *Tools) buildAuditTree(holonID string, level int, calc *assurance.Calculator) (string, error) {
	// Get R-score report
	report, err := calc.CalculateReliability(context.Background(), holonID)
	if err != nil {
		return "", err
	}

	indent := strings.Repeat("  ", level)
	tree := fmt.Sprintf("%s[%s R:%.2f] %s\n", indent, holonID, report.FinalScore, t.getHolonTitle(holonID))
	
	// Add factors/penalties note if any
	if len(report.Factors) > 0 {
		for _, f := range report.Factors {
			tree += fmt.Sprintf("%s  ! %s\n", indent, f)
		}
	}

	// Get dependencies
	// Assuming relations are available. Using raw query again as db package doesn't expose it nicely yet.

rows, err := t.DB.GetRawDB().Query("SELECT source_id, congruence_level FROM relations WHERE target_id = ? AND relation_type = 'componentOf'", holonID)
	if err != nil {
		return tree, nil // Return what we have
	}
	defer rows.Close()

	for rows.Next() {
		var depID string
		var cl int
		if err := rows.Scan(&depID, &cl); err == nil {
			clStr := fmt.Sprintf("CL:%d", cl)
			tree += fmt.Sprintf("%s  --(%s)-->\n", indent, clStr)
			subTree, _ := t.buildAuditTree(depID, level+1, calc)
			tree += subTree
		}
	}
	
	return tree, nil
}

func (t *Tools) getHolonTitle(id string) string {
	var title string
	_ = t.DB.GetRawDB().QueryRow("SELECT title FROM holons WHERE id = ?", id).Scan(&title)
	if title == "" {
		return id
	}
	return title
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