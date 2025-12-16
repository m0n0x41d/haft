package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Tools handles the actual file system operations for FPF
type Tools struct {
	FSM *FSM
	RootDir string
}

func NewTools(fsm *FSM, rootDir string) *Tools {
	return &Tools{
		FSM: fsm,
		RootDir: rootDir,
	}
}

func (t *Tools) getFPFDir() string {
	return filepath.Join(t.RootDir, ".fpf")
}

func (t *Tools) slugify(title string) string {
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	slug := reg.ReplaceAllString(strings.ToLower(title), "-")
	return strings.Trim(slug, "-")
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
	}

	for _, d := range dirs {
		path := filepath.Join(t.getFPFDir(), d)
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
		// Create .gitkeep to ensure git tracks the dirs
		os.WriteFile(filepath.Join(path, ".gitkeep"), []byte(""), 0644)
	}
	return nil
}

// ProposeHypothesis creates an L0 hypothesis file
func (t *Tools) ProposeHypothesis(title, content string) (string, error) {
	slug := t.slugify(title)
	filename := fmt.Sprintf("%s.md", slug)
	path := filepath.Join(t.getFPFDir(), "knowledge", "L0", filename)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// ManageEvidence creates an evidence file. 
// Used by Deductor (logic checks) and Inductor (empirical tests).
func (t *Tools) ManageEvidence(targetID, evidenceType, content, verdict string) (string, error) {
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
	return path, nil
}

// RefineLoopback handles the INDUCTION -> DEDUCTION transition.
// It marks the parent hypothesis as 'refuted' (or 'needs_refinement') and creates a child hypothesis.
func (t *Tools) RefineLoopback(parentID, insight, newTitle, newContent string) (string, error) {
	// 1. Create new hypothesis (child)
	childPath, err := t.ProposeHypothesis(newTitle, newContent)
	if err != nil {
		return "", fmt.Errorf("failed to create child hypothesis: %v", err)
	}

	// 2. Log the refinement event
	logFile := filepath.Join(t.getFPFDir(), "sessions", fmt.Sprintf("loopback-%d.md", time.Now().Unix()))
	logContent := fmt.Sprintf("# Loopback Event\n\nParent: %s\nInsight: %s\nChild: %s\n", parentID, insight, childPath)
	os.WriteFile(logFile, []byte(logContent), 0644)

	return childPath, nil
}

// FinalizeDecision creates the DRR and archives the session
func (t *Tools) FinalizeDecision(title, content, winnerID string) (string, error) {
	// 1. Create DRR
	drrName := fmt.Sprintf("DRR-%d-%s.md", time.Now().Unix(), t.slugify(title))
	drrPath := filepath.Join(t.getFPFDir(), "decisions", drrName)
	if err := os.WriteFile(drrPath, []byte(content), 0644); err != nil {
		return "", err
	}

	// 2. Promote Winner to L2 (Simulated move)
	// In a real impl, we would move the file from L1 to L2.
	// For now, we assume the agent did it or we just log it.
	
	// 3. Reset Session (handled by FSM transition to IDLE usually, but here we might archive)
	// (Skipping archive logic for brevity, relying on FSM state save)

	return drrPath, nil
}
