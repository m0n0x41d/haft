package fpf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"quint-mcp/assurance"
	"quint-mcp/db"

	"github.com/google/uuid"
)

type Tools struct {
	FSM     *FSM
	RootDir string
	DB      *db.Store
}

func NewTools(fsm *FSM, rootDir string, database *db.Store) *Tools {
	if database == nil {
		dbPath := filepath.Join(rootDir, ".quint", "quint.db")
		var err error
		database, err = db.NewStore(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open database in NewTools: %v\n", err)
		}
	}

	return &Tools{
		FSM:     fsm,
		RootDir: rootDir,
		DB:      database,
	}
}

func (t *Tools) GetFPFDir() string {
	return filepath.Join(t.RootDir, ".quint")
}

func (t *Tools) AuditLog(toolName, operation, actor, targetID, result string, input interface{}, details string) {
	if t.DB == nil {
		return
	}

	var inputHash string
	if input != nil {
		data, err := json.Marshal(input)
		if err == nil {
			hash := sha256.Sum256(data)
			inputHash = hex.EncodeToString(hash[:8])
		}
	}

	id := uuid.New().String()
	ctx := context.Background()
	if err := t.DB.InsertAuditLog(ctx, id, toolName, operation, actor, targetID, inputHash, result, details, "default"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to insert audit log: %v\n", err)
	}
}

func (t *Tools) Slugify(title string) string {
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	slug := reg.ReplaceAllString(strings.ToLower(title), "-")
	return strings.Trim(slug, "-")
}

func (t *Tools) MoveHypothesis(hypothesisID, sourceLevel, destLevel string) (string, error) {
	srcPath := filepath.Join(t.GetFPFDir(), "knowledge", sourceLevel, hypothesisID+".md")
	destPath := filepath.Join(t.GetFPFDir(), "knowledge", destLevel, hypothesisID+".md")

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		t.AuditLog("quint_move", "move_hypothesis", "agent", hypothesisID, "ERROR", map[string]string{"from": sourceLevel, "to": destLevel}, "not found")
		return "", fmt.Errorf("hypothesis %s not found in %s", hypothesisID, sourceLevel)
	}

	if err := os.Rename(srcPath, destPath); err != nil {
		t.AuditLog("quint_move", "move_hypothesis", "agent", hypothesisID, "ERROR", map[string]string{"from": sourceLevel, "to": destLevel}, err.Error())
		return "", fmt.Errorf("failed to move hypothesis from %s to %s: %v", sourceLevel, destLevel, err)
	}

	if t.DB != nil {
		if err := t.DB.UpdateHolonLayer(context.Background(), hypothesisID, destLevel); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update holon layer in DB: %v\n", err)
		}
	}

	t.AuditLog("quint_move", "move_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"from": sourceLevel, "to": destLevel}, "")
	return destPath, nil
}

func (t *Tools) InitProject() error {
	dirs := []string{
		"evidence",
		"decisions",
		"sessions",
		"knowledge/L0",
		"knowledge/L1",
		"knowledge/L2",
		"knowledge/invalid",
		"agents",
	}

	for _, d := range dirs {
		path := filepath.Join(t.GetFPFDir(), d)
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(path, ".gitkeep"), []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to write .gitkeep file: %v", err)
		}
	}

	if t.DB == nil {
		dbPath := filepath.Join(t.GetFPFDir(), "quint.db")
		database, err := db.NewStore(dbPath)
		if err != nil {
			fmt.Printf("Warning: Failed to init DB: %v\n", err)
		} else {
			t.DB = database
		}
	}

	return nil
}

func (t *Tools) RecordContext(vocabulary, invariants string) (string, error) {
	content := fmt.Sprintf("# Bounded Context\n\n## Vocabulary\n%s\n\n## Invariants\n%s\n", vocabulary, invariants)
	path := filepath.Join(t.GetFPFDir(), "context.md")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func (t *Tools) GetAgentContext(role string) (string, error) {
	filename := strings.ToLower(role) + ".md"
	path := filepath.Join(t.GetFPFDir(), "agents", filename)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("agent profile for %s not found at %s", role, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

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

	ledger := fmt.Sprintf(`{"duration_ms": %d}`, end.Sub(start).Milliseconds())
	if err := t.DB.RecordWork(context.Background(), id, methodName, performer, start, end, ledger); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to record work in DB: %v\n", err)
	}
}

func (t *Tools) ProposeHypothesis(title, content, scope, kind, rationale string) (string, error) {
	defer t.RecordWork("ProposeHypothesis", time.Now())

	slug := t.Slugify(title)
	filename := fmt.Sprintf("%s.md", slug)
	path := filepath.Join(t.GetFPFDir(), "knowledge", "L0", filename)

	body := fmt.Sprintf("\n# Hypothesis: %s\n\n%s\n\n## Rationale\n%s", title, content, rationale)
	fields := map[string]string{
		"scope": scope,
		"kind":  kind,
	}

	if err := WriteWithHash(path, fields, body); err != nil {
		t.AuditLog("quint_propose", "create_hypothesis", "agent", slug, "ERROR", map[string]string{"title": title, "kind": kind}, err.Error())
		return "", err
	}

	if t.DB != nil {
		if err := t.DB.CreateHolon(context.Background(), slug, "hypothesis", kind, "L0", title, body, "default", scope, ""); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create holon in DB: %v\n", err)
		}
	}

	t.AuditLog("quint_propose", "create_hypothesis", "agent", slug, "SUCCESS", map[string]string{"title": title, "kind": kind, "scope": scope}, "")

	return path, nil
}

func (t *Tools) VerifyHypothesis(hypothesisID, checksJSON, verdict string) (string, error) {
	defer t.RecordWork("VerifyHypothesis", time.Now())

	carrierRef := "internal-logic"
	if t.DB != nil {
		holon, err := t.DB.GetHolon(context.Background(), hypothesisID)
		if err == nil && holon.Kind.Valid {
			switch holon.Kind.String {
			case "system":
				carrierRef = "internal-logic"
			case "episteme":
				carrierRef = "formal-logic"
			}
		}
	}

	v := strings.ToLower(verdict)
	if v == "pass" {
		_, err := t.MoveHypothesis(hypothesisID, "L0", "L1")
		if err != nil {
			t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "ERROR", map[string]string{"verdict": verdict}, err.Error())
			return "", err
		}

		evidenceContent := fmt.Sprintf("Verification Checks:\n%s", checksJSON)
		if _, err := t.ManageEvidence(PhaseDeduction, "add", hypothesisID, "verification", evidenceContent, "pass", "L1", carrierRef, ""); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to record verification evidence for %s: %v\n", hypothesisID, err)
		}

		t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "PASS", "result": "L1"}, "")
		return fmt.Sprintf("Hypothesis %s (kind: %s) promoted to L1", hypothesisID, carrierRef), nil
	} else if v == "fail" {
		_, err := t.MoveHypothesis(hypothesisID, "L0", "invalid")
		if err != nil {
			t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "ERROR", map[string]string{"verdict": verdict}, err.Error())
			return "", err
		}
		t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "FAIL", "result": "invalid"}, "")
		return fmt.Sprintf("Hypothesis %s moved to invalid", hypothesisID), nil
	} else if v == "refine" {
		t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "REFINE", "result": "L0"}, "")
		return fmt.Sprintf("Hypothesis %s requires refinement (staying in L0)", hypothesisID), nil
	}

	return "", fmt.Errorf("unknown verdict: %s", verdict)
}

func (t *Tools) AuditEvidence(hypothesisID, risks string) (string, error) {
	defer t.RecordWork("AuditEvidence", time.Now())
	_, err := t.ManageEvidence(PhaseDecision, "add", hypothesisID, "audit_report", risks, "pass", "L2", "auditor", "")
	return "Audit recorded for " + hypothesisID, err
}

func (t *Tools) ManageEvidence(currentPhase Phase, action, targetID, evidenceType, content, verdict, assuranceLevel, carrierRef, validUntil string) (string, error) {
	defer t.RecordWork("ManageEvidence", time.Now())
	ctx := context.Background()

	if action == "check" {
		if t.DB == nil {
			return "", fmt.Errorf("DB not initialized")
		}
		if targetID == "all" {
			return "Global evidence audit not implemented yet. Please specify a target_id.", nil
		}
		ev, err := t.DB.GetEvidence(ctx, targetID)
		if err != nil {
			return "", err
		}
		var report string
		for _, e := range ev {
			report += fmt.Sprintf("- [%s] %s (L:%s, Ref:%s): %s\n", e.Verdict, e.Type, e.AssuranceLevel.String, e.CarrierRef.String, e.Content)
		}
		if report == "" {
			return "No evidence found for " + targetID, nil
		}
		return report, nil
	}

	shouldPromote := false

	normalizedVerdict := strings.ToLower(verdict)

	switch normalizedVerdict {
	case "pass":
		switch currentPhase {
		case PhaseDeduction:
			if assuranceLevel == "L1" || assuranceLevel == "L2" {
				shouldPromote = true
			}
		case PhaseInduction:
			if assuranceLevel == "L2" {
				shouldPromote = true
			}
		}
	}

	var moveErr error
	if (normalizedVerdict == "pass") && shouldPromote {
		switch currentPhase {
		case PhaseDeduction:
			_, moveErr = t.MoveHypothesis(targetID, "L0", "L1")
		case PhaseInduction:
			if _, err := os.Stat(filepath.Join(t.GetFPFDir(), "knowledge", "L0", targetID+".md")); err == nil {
				return "", fmt.Errorf("Hypothesis %s is still in L0. Run /q2-verify to promote it to L1 before testing.", targetID)
			}
			_, moveErr = t.MoveHypothesis(targetID, "L1", "L2")
		}
	} else if normalizedVerdict == "fail" || normalizedVerdict == "refine" {
		switch currentPhase {
		case PhaseDeduction:
			_, moveErr = t.MoveHypothesis(targetID, "L0", "invalid")
		case PhaseInduction:
			_, moveErr = t.MoveHypothesis(targetID, "L1", "invalid")
		}
	}

	if moveErr != nil {
		return "", fmt.Errorf("failed to move hypothesis: %v", moveErr)
	}

	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s-%s-%s.md", date, evidenceType, targetID)
	path := filepath.Join(t.GetFPFDir(), "evidence", filename)

	body := fmt.Sprintf("\n%s", content)
	fields := map[string]string{
		"id":              filename,
		"type":            evidenceType,
		"target":          targetID,
		"verdict":         normalizedVerdict,
		"assurance_level": assuranceLevel,
		"carrier_ref":     carrierRef,
		"valid_until":     validUntil,
		"date":            date,
	}

	if err := WriteWithHash(path, fields, body); err != nil {
		return "", err
	}

	if t.DB != nil {
		if err := t.DB.AddEvidence(ctx, filename, targetID, evidenceType, content, normalizedVerdict, assuranceLevel, carrierRef, validUntil); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to add evidence to DB: %v\n", err)
		}
		if err := t.DB.Link(ctx, filename, targetID, "verifiedBy"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to link evidence in DB: %v\n", err)
		}
	}

	if !shouldPromote && verdict == "PASS" {
		return path + " (Evidence recorded, but Assurance Level insufficient for promotion)", nil
	}
	return path, nil
}

func (t *Tools) RefineLoopback(currentPhase Phase, parentID, insight, newTitle, newContent, scope string) (string, error) {
	defer t.RecordWork("RefineLoopback", time.Now())

	var parentLevel string
	switch currentPhase {
	case PhaseInduction:
		parentLevel = "L1"
	case PhaseDeduction:
		parentLevel = "L0"
	default:
		return "", fmt.Errorf("loopback not applicable from phase %s", currentPhase)
	}

	if _, err := t.MoveHypothesis(parentID, parentLevel, "invalid"); err != nil {
		return "", fmt.Errorf("failed to move parent hypothesis to invalid: %v", err)
	}

	rationale := fmt.Sprintf(`{"source": "loopback", "parent_id": "%s", "insight": "%s"}`, parentID, insight)
	childPath, err := t.ProposeHypothesis(newTitle, newContent, scope, "system", rationale)
	if err != nil {
		return "", fmt.Errorf("failed to create child hypothesis: %v", err)
	}

	logFile := filepath.Join(t.GetFPFDir(), "sessions", fmt.Sprintf("loopback-%d.md", time.Now().Unix()))
	logContent := fmt.Sprintf("# Loopback Event\n\nParent: %s (moved to invalid)\nInsight: %s\nChild: %s\n", parentID, insight, childPath)
	if err := os.WriteFile(logFile, []byte(logContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write loopback log file: %v", err)
	}

	return childPath, nil
}

func (t *Tools) FinalizeDecision(title, winnerID, decisionContext, decision, rationale, consequences, characteristics string) (string, error) {
	defer t.RecordWork("FinalizeDecision", time.Now())

	body := fmt.Sprintf("\n# %s\n\n", title)
	body += fmt.Sprintf("## Context\n%s\n\n", decisionContext)
	body += fmt.Sprintf("## Decision\n**Selected Option:** %s\n\n%s\n\n", winnerID, decision)
	body += fmt.Sprintf("## Rationale\n%s\n\n", rationale)
	if characteristics != "" {
		body += fmt.Sprintf("### Characteristic Space (C.16)\n%s\n\n", characteristics)
	}
	body += fmt.Sprintf("## Consequences\n%s\n", consequences)

	timestamp := time.Now().Unix()
	drrName := fmt.Sprintf("DRR-%d-%s.md", timestamp, t.Slugify(title))
	drrPath := filepath.Join(t.GetFPFDir(), "decisions", drrName)

	fields := map[string]string{
		"type":      "DRR",
		"winner_id": winnerID,
		"timestamp": fmt.Sprintf("%d", timestamp),
	}

	if err := WriteWithHash(drrPath, fields, body); err != nil {
		t.AuditLog("quint_decide", "finalize_decision", "agent", winnerID, "ERROR", map[string]string{"title": title}, err.Error())
		return "", err
	}

	if t.DB != nil {
		drrID := t.Slugify(title)
		if err := t.DB.CreateHolon(context.Background(), drrID, "DRR", "", "DRR", title, body, "default", "", winnerID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create DRR holon in DB: %v\n", err)
		}
	}

	if winnerID != "" {
		_, err := t.MoveHypothesis(winnerID, "L1", "L2")
		if err != nil {
			fmt.Printf("WARNING: Failed to move winner hypothesis %s to L2: %v\n", winnerID, err)
		}
	}

	t.AuditLog("quint_decide", "finalize_decision", "agent", winnerID, "SUCCESS", map[string]string{"title": title, "drr": drrName}, "")
	return drrPath, nil
}

func (t *Tools) RunDecay() error {
	defer t.RecordWork("RunDecay", time.Now())
	if t.DB == nil {
		return fmt.Errorf("DB not initialized")
	}

	ctx := context.Background()
	ids, err := t.DB.ListAllHolonIDs(ctx)
	if err != nil {
		return err
	}

	calc := assurance.New(t.DB.GetRawDB())
	updatedCount := 0

	for _, id := range ids {
		_, err := calc.CalculateReliability(ctx, id)
		if err != nil {
			fmt.Printf("Error calculating R for %s: %v\n", id, err)
			continue
		}
		updatedCount++
	}

	fmt.Printf("Decay update complete. Processed %d holons.\n", updatedCount)
	return nil
}

func (t *Tools) VisualizeAudit(rootID string) (string, error) {
	defer t.RecordWork("VisualizeAudit", time.Now())
	if t.DB == nil {
		return "", fmt.Errorf("DB not initialized")
	}

	if rootID == "all" {
		return "Please specify a root ID for the audit tree.", nil
	}

	calc := assurance.New(t.DB.GetRawDB())
	return t.buildAuditTree(rootID, 0, calc)
}

func (t *Tools) buildAuditTree(holonID string, level int, calc *assurance.Calculator) (string, error) {
	ctx := context.Background()
	report, err := calc.CalculateReliability(ctx, holonID)
	if err != nil {
		return "", err
	}

	indent := strings.Repeat("  ", level)
	tree := fmt.Sprintf("%s[%s R:%.2f] %s\n", indent, holonID, report.FinalScore, t.getHolonTitle(holonID))

	if len(report.Factors) > 0 {
		for _, f := range report.Factors {
			tree += fmt.Sprintf("%s  ! %s\n", indent, f)
		}
	}

	components, err := t.DB.GetComponentsOf(ctx, holonID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to query dependencies for %s: %v\n", holonID, err)
		return tree, nil
	}

	for _, c := range components {
		cl := int64(3)
		if c.CongruenceLevel.Valid {
			cl = c.CongruenceLevel.Int64
		}
		clStr := fmt.Sprintf("CL:%d", cl)
		tree += fmt.Sprintf("%s  --(%s)-->\n", indent, clStr)
		subTree, _ := t.buildAuditTree(c.SourceID, level+1, calc)
		tree += subTree
	}

	return tree, nil
}

func (t *Tools) getHolonTitle(id string) string {
	ctx := context.Background()
	title, err := t.DB.GetHolonTitle(ctx, id)
	if err != nil || title == "" {
		return id
	}
	return title
}

func (t *Tools) Actualize() (string, error) {
	var report strings.Builder
	fpfDir := filepath.Join(t.RootDir, ".fpf")
	quintDir := t.GetFPFDir()

	if _, err := os.Stat(fpfDir); err == nil {
		report.WriteString("MIGRATION: Found legacy .fpf directory.\n")

		if _, err := os.Stat(quintDir); err == nil {
			return report.String(), fmt.Errorf("migration conflict: both .fpf and .quint exist. Please resolve manually")
		}

		report.WriteString("MIGRATION: Renaming .fpf -> .quint\n")
		if err := os.Rename(fpfDir, quintDir); err != nil {
			return report.String(), fmt.Errorf("failed to rename .fpf: %w", err)
		}
		report.WriteString("MIGRATION: Success.\n")
	}

	legacyDB := filepath.Join(quintDir, "fpf.db")
	newDB := filepath.Join(quintDir, "quint.db")

	if _, err := os.Stat(legacyDB); err == nil {
		report.WriteString("MIGRATION: Found legacy fpf.db.\n")
		if err := os.Rename(legacyDB, newDB); err != nil {
			return report.String(), fmt.Errorf("failed to rename fpf.db: %w", err)
		}
		report.WriteString("MIGRATION: Renamed to quint.db.\n")
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = t.RootDir
	output, err := cmd.Output()
	if err == nil {
		currentCommit := strings.TrimSpace(string(output))
		lastCommit := t.FSM.State.LastCommit

		if lastCommit == "" {
			report.WriteString(fmt.Sprintf("RECONCILIATION: Initializing baseline commit to %s\n", currentCommit))
			t.FSM.State.LastCommit = currentCommit
			if err := t.FSM.SaveState(filepath.Join(t.GetFPFDir(), "state.json")); err != nil {
				report.WriteString(fmt.Sprintf("Warning: Failed to save state: %v\n", err))
			}
		} else if currentCommit != lastCommit {
			report.WriteString(fmt.Sprintf("RECONCILIATION: Detected changes since %s\n", lastCommit))
			diffCmd := exec.Command("git", "diff", "--name-status", lastCommit, "HEAD")
			diffCmd.Dir = t.RootDir
			diffOutput, err := diffCmd.Output()
			if err == nil {
				report.WriteString("Changed files:\n")
				report.WriteString(string(diffOutput))
			} else {
				report.WriteString(fmt.Sprintf("Warning: Failed to get diff: %v\n", err))
			}

			t.FSM.State.LastCommit = currentCommit
			if err := t.FSM.SaveState(filepath.Join(t.GetFPFDir(), "state.json")); err != nil {
				report.WriteString(fmt.Sprintf("Warning: Failed to save state: %v\n", err))
			}
		} else {
			report.WriteString("RECONCILIATION: No changes detected (Clean).\n")
		}
	} else {
		report.WriteString("RECONCILIATION: Not a git repository or git error.\n")
	}

	return report.String(), nil
}

func (t *Tools) GetHolon(id string) (db.Holon, error) {
	if t.DB == nil {
		return db.Holon{}, fmt.Errorf("DB not initialized")
	}
	return t.DB.GetHolon(context.Background(), id)
}

func (t *Tools) CalculateR(holonID string) (string, error) {
	defer t.RecordWork("CalculateR", time.Now())
	if t.DB == nil {
		return "", fmt.Errorf("DB not initialized")
	}

	calc := assurance.New(t.DB.GetRawDB())
	report, err := calc.CalculateReliability(context.Background(), holonID)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("## Reliability Report: %s\n\n", holonID))
	result.WriteString(fmt.Sprintf("**R_eff: %.2f**\n", report.FinalScore))
	result.WriteString(fmt.Sprintf("- Self Score: %.2f\n", report.SelfScore))
	if report.WeakestLink != "" {
		result.WriteString(fmt.Sprintf("- Weakest Link: %s\n", report.WeakestLink))
	}
	if report.DecayPenalty > 0 {
		result.WriteString(fmt.Sprintf("- Decay Penalty: %.2f\n", report.DecayPenalty))
	}
	if len(report.Factors) > 0 {
		result.WriteString("\n**Factors:**\n")
		for _, f := range report.Factors {
			result.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	return result.String(), nil
}

func (t *Tools) CheckDecay() (string, error) {
	defer t.RecordWork("CheckDecay", time.Now())
	if t.DB == nil {
		return "", fmt.Errorf("DB not initialized")
	}

	ctx := context.Background()
	rawDB := t.DB.GetRawDB()

	rows, err := rawDB.QueryContext(ctx, `
		SELECT e.holon_id, h.title, COUNT(*) as expired_count,
		       MAX(JULIANDAY('now') - JULIANDAY(substr(e.valid_until, 1, 10))) as max_days_overdue
		FROM evidence e
		JOIN holons h ON e.holon_id = h.id
		WHERE e.valid_until IS NOT NULL
		  AND substr(e.valid_until, 1, 10) < date('now')
		GROUP BY e.holon_id
		ORDER BY max_days_overdue DESC
	`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var result strings.Builder
	result.WriteString("## Evidence Decay Report\n\n")

	count := 0
	for rows.Next() {
		var holonID, title string
		var expiredCount int
		var daysOverdue float64
		if err := rows.Scan(&holonID, &title, &expiredCount, &daysOverdue); err != nil {
			continue
		}
		count++
		result.WriteString(fmt.Sprintf("### %s (%s)\n", title, holonID))
		result.WriteString(fmt.Sprintf("- Expired evidence: %d\n", expiredCount))
		result.WriteString(fmt.Sprintf("- Max days overdue: %.0f\n\n", daysOverdue))
	}

	if count == 0 {
		result.WriteString("No expired evidence found. All holons are fresh.\n")
	} else {
		result.WriteString(fmt.Sprintf("---\n**Total holons with expired evidence: %d**\n", count))
		result.WriteString("\nRecommendation: Run `/q3-validate` to refresh evidence for affected holons.\n")
	}

	return result.String(), nil
}
