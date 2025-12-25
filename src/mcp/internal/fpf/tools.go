package fpf

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/assurance"
	"github.com/m0n0x41d/quint-code/db"

	"github.com/google/uuid"
)

var slugifyRegex = regexp.MustCompile("[^a-zA-Z0-9]+")

type DependencySuggestion struct {
	HolonID string
	Title   string
	Type    string
	Layer   string
}

type Tools struct {
	FSM     *FSM
	RootDir string
	DB      *db.Store
}

// Contract represents the implementation contract for a DRR
type Contract struct {
	Invariants         []string `json:"invariants,omitempty"`
	AntiPatterns       []string `json:"anti_patterns,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	AffectedScope      []string `json:"affected_scope,omitempty"`
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

// AuditLog records an audit entry. The actor is derived from the tool name
// using GetRoleForTool to ensure proper role traceability.
func (t *Tools) AuditLog(toolName, operation, actor, targetID, result string, input interface{}, details string) {
	if t.DB == nil {
		return
	}

	// Derive role from tool name (implicit role enforcement)
	// If actor is "agent" (legacy) or empty, use the implicit role
	if actor == "" || actor == "agent" {
		actor = string(GetRoleForTool(toolName))
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
	slug := slugifyRegex.ReplaceAllString(strings.ToLower(title), "-")
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
	// Normalize vocabulary: "Term1: Def1. Term2: Def2." → "- **Term1**: Def1.\n- **Term2**: Def2."
	vocabFormatted := formatVocabulary(vocabulary)

	// Normalize invariants: "1. Item1. 2. Item2." → "1. Item1.\n2. Item2."
	invFormatted := formatInvariants(invariants)

	content := fmt.Sprintf("# Bounded Context\n\n## Vocabulary\n\n%s\n\n## Invariants\n\n%s\n", vocabFormatted, invFormatted)
	path := filepath.Join(t.GetFPFDir(), "context.md")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func formatVocabulary(vocab string) string {
	// Pattern: "Term: definition." or "Term: definition" followed by another "Term:"
	// Split on pattern where a new term definition starts
	termPattern := regexp.MustCompile(`([A-Z][a-zA-Z0-9_\[\],<>]+):\s*`)
	matches := termPattern.FindAllStringSubmatchIndex(vocab, -1)

	if len(matches) == 0 {
		return vocab // No terms found, return as-is
	}

	var lines []string
	for i, match := range matches {
		termStart := match[2]
		termEnd := match[3]
		defStart := match[1]

		var defEnd int
		if i+1 < len(matches) {
			defEnd = matches[i+1][0]
		} else {
			defEnd = len(vocab)
		}

		term := vocab[termStart:termEnd]
		def := strings.TrimSpace(vocab[defStart:defEnd])

		lines = append(lines, fmt.Sprintf("- **%s**: %s", term, def))
	}

	return strings.Join(lines, "\n")
}

func formatInvariants(inv string) string {
	// Pattern: "1. ...", "2. ...", etc. possibly all on one line
	numPattern := regexp.MustCompile(`(\d+)\.\s+`)
	matches := numPattern.FindAllStringSubmatchIndex(inv, -1)

	if len(matches) == 0 {
		return inv // No numbered items found, return as-is
	}

	var lines []string
	for i, match := range matches {
		numStart := match[2]
		numEnd := match[3]
		contentStart := match[1]

		var contentEnd int
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		} else {
			contentEnd = len(inv)
		}

		num := inv[numStart:numEnd]
		content := strings.TrimSpace(inv[contentStart:contentEnd])

		lines = append(lines, fmt.Sprintf("%s. %s", num, content))
	}

	return strings.Join(lines, "\n")
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

func (t *Tools) suggestDependencies(title, content string) []DependencySuggestion {
	if t.DB == nil {
		return nil
	}

	ctx := context.Background()

	searchText := title
	if len(content) > 200 {
		searchText += " " + content[:200]
	} else if len(content) > 0 {
		searchText += " " + content
	}

	results, err := t.DB.SearchOR(ctx, searchText, "holons", "", "", 10)
	if err != nil {
		return nil
	}

	var suggestions []DependencySuggestion
	for _, r := range results {
		if r.Layer == "DRR" || r.Layer == "L2" || r.Layer == "L1" {
			suggestions = append(suggestions, DependencySuggestion{
				HolonID: r.ID,
				Title:   r.Title,
				Type:    r.Type,
				Layer:   r.Layer,
			})
		}
	}

	return suggestions
}

func (t *Tools) ProposeHypothesis(title, content, scope, kind, rationale string, decisionContext string, dependsOn []string, dependencyCL int) (string, error) {
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

	ctx := context.Background()

	if decisionContext != "" && t.DB != nil {
		if _, err := t.DB.GetHolon(ctx, decisionContext); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: decision_context '%s' not found, skipping MemberOf\n", decisionContext)
		} else {
			if err := t.createRelation(ctx, slug, "memberOf", decisionContext, 3); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create MemberOf relation: %v\n", err)
			}
		}
	}

	if len(dependsOn) > 0 && t.DB != nil {
		if dependencyCL < 1 || dependencyCL > 3 {
			dependencyCL = 3
		}

		relationType := "componentOf"
		if kind == "episteme" {
			relationType = "constituentOf"
		}

		for _, depID := range dependsOn {
			if _, err := t.DB.GetHolon(ctx, depID); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: dependency '%s' not found, skipping\n", depID)
				continue
			}

			if cyclic, _ := t.wouldCreateCycle(ctx, depID, slug); cyclic {
				fmt.Fprintf(os.Stderr, "Warning: dependency on '%s' would create cycle, skipping\n", depID)
				continue
			}

			if err := t.createRelation(ctx, depID, relationType, slug, dependencyCL); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create %s relation to %s: %v\n",
					relationType, depID, err)
			}
		}
	}

	t.AuditLog("quint_propose", "create_hypothesis", "agent", slug, "SUCCESS", map[string]string{"title": title, "kind": kind, "scope": scope}, "")

	var warningBlock string
	if len(dependsOn) == 0 && t.DB != nil {
		suggestions := t.suggestDependencies(title, content)
		if len(suggestions) > 0 {
			var sb strings.Builder
			sb.WriteString("\n\n⚠️ POTENTIAL DEPENDENCIES DETECTED\n\n")
			sb.WriteString("Related holons found (ranked by relevance):\n")
			for _, s := range suggestions {
				sb.WriteString(fmt.Sprintf("  • %s [%s] %s\n",
					s.HolonID, s.Layer, s.Title))
			}
			sb.WriteString("\nConsider linking with:\n")
			sb.WriteString(fmt.Sprintf("  quint_link(source_id=\"%s\", target_id=\"<id>\")\n", slug))
			sb.WriteString("\nThis enables:\n")
			sb.WriteString("  - WLNK applies to R_eff\n")
			sb.WriteString("  - Invariants inherited from dependency\n")
			sb.WriteString("  - Audit trail of architectural coupling\n")
			warningBlock = sb.String()
		}
	}

	return path + warningBlock, nil
}

func (t *Tools) createRelation(ctx context.Context, sourceID, relationType, targetID string, cl int) error {
	if sourceID == targetID {
		return fmt.Errorf("holon cannot relate to itself")
	}

	if err := t.DB.CreateRelation(ctx, sourceID, relationType, targetID, cl); err != nil {
		return err
	}

	t.AuditLog("quint_propose", "create_relation", "agent", sourceID, "SUCCESS",
		map[string]string{"relation": relationType, "target": targetID, "cl": fmt.Sprintf("%d", cl)}, "")

	return nil
}

func (t *Tools) wouldCreateCycle(ctx context.Context, sourceID, targetID string) (bool, error) {
	visited := make(map[string]bool)
	return t.isReachable(ctx, targetID, sourceID, visited)
}

func (t *Tools) isReachable(ctx context.Context, from, to string, visited map[string]bool) (bool, error) {
	if from == to {
		return true, nil
	}
	if visited[from] {
		return false, nil
	}
	visited[from] = true

	deps, err := t.DB.GetDependencies(ctx, from)
	if err != nil {
		return false, err
	}

	for _, dep := range deps {
		if reachable, err := t.isReachable(ctx, dep.TargetID, to, visited); err != nil {
			return false, err
		} else if reachable {
			return true, nil
		}
	}
	return false, nil
}

func (t *Tools) LinkHolons(sourceID, targetID string, cl int) (string, error) {
	defer t.RecordWork("LinkHolons", time.Now())

	if t.DB == nil {
		return "", fmt.Errorf("database not initialized - run quint_internalize first")
	}

	ctx := context.Background()

	source, err := t.DB.GetHolon(ctx, sourceID)
	if err != nil {
		return "", fmt.Errorf("source holon '%s' not found", sourceID)
	}

	_, err = t.DB.GetHolon(ctx, targetID)
	if err != nil {
		return "", fmt.Errorf("target holon '%s' not found", targetID)
	}

	if cyclic, _ := t.wouldCreateCycle(ctx, sourceID, targetID); cyclic {
		return "", fmt.Errorf("link would create dependency cycle")
	}

	// system → componentOf, episteme → constituentOf
	relationType := "componentOf"
	if source.Kind.Valid && source.Kind.String == "episteme" {
		relationType = "constituentOf"
	}

	if cl < 1 || cl > 3 {
		cl = 3
	}
	if err := t.createRelation(ctx, sourceID, relationType, targetID, cl); err != nil {
		return "", fmt.Errorf("failed to create link: %w", err)
	}

	calc := assurance.New(t.DB.GetRawDB())
	report, _ := calc.CalculateReliability(ctx, targetID)
	newR := 0.0
	if report != nil {
		newR = report.FinalScore
	}

	t.AuditLog("quint_link", "link_holons", "", targetID, "SUCCESS",
		map[string]string{"source": sourceID, "relation": relationType, "cl": fmt.Sprintf("%d", cl)}, "")

	return fmt.Sprintf("✅ Linked: %s --%s--> %s\n   New R_eff for %s: %.2f\n\n"+
		"WLNK now applies: %s.R_eff ≤ %s.R_eff",
		sourceID, relationType, targetID, targetID, newR, targetID, sourceID), nil
}

func (t *Tools) VerifyHypothesis(hypothesisID, checksJSON, verdict, carrierFiles string) (string, error) {
	defer t.RecordWork("VerifyHypothesis", time.Now())

	carrierRef := carrierFiles
	if carrierRef == "" {
		carrierRef = "internal-logic"
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
	}

	switch strings.ToLower(verdict) {
	case "pass":
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
	case "fail":
		_, err := t.MoveHypothesis(hypothesisID, "L0", "invalid")
		if err != nil {
			t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "ERROR", map[string]string{"verdict": verdict}, err.Error())
			return "", err
		}
		t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "FAIL", "result": "invalid"}, "")
		return fmt.Sprintf("Hypothesis %s moved to invalid", hypothesisID), nil
	case "refine":
		t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "REFINE", "result": "L0"}, "")
		return fmt.Sprintf("Hypothesis %s requires refinement (staying in L0)", hypothesisID), nil
	default:
		return "", fmt.Errorf("unknown verdict: %s", verdict)
	}
}

func (t *Tools) AuditEvidence(hypothesisID, risks string) (string, error) {
	defer t.RecordWork("AuditEvidence", time.Now())
	_, err := t.ManageEvidence(PhaseDecision, "add", hypothesisID, "audit_report", risks, "pass", "L2", "auditor", "")
	return "Audit recorded for " + hypothesisID, err
}

func (t *Tools) ManageEvidence(currentPhase Phase, action, targetID, evidenceType, content, verdict, assuranceLevel, carrierRef, validUntil string) (string, error) {
	defer t.RecordWork("ManageEvidence", time.Now())

	if validUntil == "" && action != "check" {
		validUntil = time.Now().AddDate(0, 0, 90).Format("2006-01-02")
	}
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
				return "", fmt.Errorf("hypothesis %s is still in L0: run /q2-verify to promote it to L1 before testing", targetID)
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
		// Get current commit for carrier_commit tracking (v5.0.0)
		carrierCommit := ""
		if currentHead, err := t.getCurrentHead(); err == nil {
			carrierCommit = currentHead
		}

		if err := t.DB.AddEvidence(ctx, filename, targetID, evidenceType, content, normalizedVerdict, assuranceLevel, carrierRef, carrierCommit, validUntil); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to add evidence to DB: %v\n", err)
		}
		if err := t.DB.Link(ctx, filename, targetID, "verifiedBy"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to link evidence in DB: %v\n", err)
		}

		// Clear stale flags on successful re-validation (v5.0.0)
		if normalizedVerdict == "pass" {
			if err := t.DB.ClearAllEvidenceStaleForHolon(ctx, targetID); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to clear stale evidence: %v\n", err)
			}
			if err := t.DB.ClearHolonReverification(ctx, targetID); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to clear reverification flag: %v\n", err)
			}
			// Recalculate R_eff after clearing staleness
			t.recalculateHolonR(ctx, targetID)
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
	childPath, err := t.ProposeHypothesis(newTitle, newContent, scope, "system", rationale, "", nil, 3)
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

func (t *Tools) FinalizeDecision(title, winnerID string, rejectedIDs []string, decisionContext, decision, rationale, consequences, characteristics, contractJSON string) (string, error) {
	defer t.RecordWork("FinalizeDecision", time.Now())

	var contract Contract
	if contractJSON != "" {
		if err := json.Unmarshal([]byte(contractJSON), &contract); err != nil {
			return "", fmt.Errorf("invalid contract JSON: %w", err)
		}
	}

	body := fmt.Sprintf("\n# %s\n\n", title)
	body += fmt.Sprintf("## Context\n%s\n\n", decisionContext)
	body += fmt.Sprintf("## Decision\n**Selected Option:** %s\n\n%s\n\n", winnerID, decision)
	body += fmt.Sprintf("## Rationale\n%s\n\n", rationale)
	if characteristics != "" {
		body += fmt.Sprintf("### Characteristic Space (C.16)\n%s\n\n", characteristics)
	}
	body += fmt.Sprintf("## Consequences\n%s\n\n", consequences)

	if contractJSON != "" {
		body += "## Implementation Contract\n\n"
		if len(contract.Invariants) > 0 {
			body += "### Invariants (MUST remain true)\n"
			for _, inv := range contract.Invariants {
				body += fmt.Sprintf("- %s\n", inv)
			}
			body += "\n"
		}
		if len(contract.AntiPatterns) > 0 {
			body += "### Anti-Patterns (MUST NOT happen)\n"
			for _, ap := range contract.AntiPatterns {
				body += fmt.Sprintf("- %s\n", ap)
			}
			body += "\n"
		}
		if len(contract.AcceptanceCriteria) > 0 {
			body += "### Acceptance Criteria\n"
			for _, ac := range contract.AcceptanceCriteria {
				body += fmt.Sprintf("- [ ] %s\n", ac)
			}
			body += "\n"
		}
		if len(contract.AffectedScope) > 0 {
			body += "### Affected Scope\n"
			for _, scope := range contract.AffectedScope {
				body += fmt.Sprintf("- `%s`\n", scope)
			}
			body += "\n"
		}
	}

	now := time.Now()
	dateStr := now.Format("2006-01-02")
	drrName := fmt.Sprintf("DRR-%s-%s.md", dateStr, t.Slugify(title))
	drrPath := filepath.Join(t.GetFPFDir(), "decisions", drrName)

	fields := map[string]string{
		"type":      "DRR",
		"winner_id": winnerID,
		"created":   now.Format(time.RFC3339),
	}
	if contractJSON != "" {
		compactContract, _ := json.Marshal(contract)
		fields["contract"] = string(compactContract)
	}

	if err := WriteWithHash(drrPath, fields, body); err != nil {
		t.AuditLog("quint_decide", "finalize_decision", "agent", winnerID, "ERROR", map[string]string{"title": title}, err.Error())
		return "", err
	}

	var scopeForDB string
	if len(contract.AffectedScope) > 0 {
		scopeBytes, _ := json.Marshal(contract.AffectedScope)
		scopeForDB = string(scopeBytes)
	}

	if t.DB != nil {
		ctx := context.Background()
		drrID := t.Slugify(title)
		if err := t.DB.CreateHolon(ctx, drrID, "DRR", "", "DRR", title, body, "default", scopeForDB, winnerID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create DRR holon in DB: %v\n", err)
		}

		if winnerID != "" {
			if err := t.createRelation(ctx, drrID, "selects", winnerID, 3); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create selects relation: %v\n", err)
			}
		}

		for _, rejID := range rejectedIDs {
			if rejID != "" && rejID != winnerID {
				if err := t.createRelation(ctx, drrID, "rejects", rejID, 3); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to create rejects relation to %s: %v\n", rejID, err)
				}
			}
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

	// Show componentOf/constituentOf dependencies (these propagate WLNK)
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

	// Show memberOf relations (alternatives grouped under decision context)
	// Note: memberOf does NOT propagate R, shown for visibility only
	members, err := t.DB.GetCollectionMembers(ctx, holonID)
	if err == nil && len(members) > 0 {
		tree += fmt.Sprintf("%s  [members]\n", indent)
		for _, m := range members {
			memberReport, mErr := calc.CalculateReliability(ctx, m.SourceID)
			if mErr != nil {
				tree += fmt.Sprintf("%s    - %s (error)\n", indent, m.SourceID)
				continue
			}
			tree += fmt.Sprintf("%s    - [%s R:%.2f] %s\n", indent, m.SourceID, memberReport.FinalScore, t.getHolonTitle(m.SourceID))
		}
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
			if err := t.FSM.SaveState("default"); err != nil {
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
			if err := t.FSM.SaveState("default"); err != nil {
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

func (t *Tools) CheckDecay(deprecate, waiveID, waiveUntil, waiveRationale string) (string, error) {
	defer t.RecordWork("CheckDecay", time.Now())
	if t.DB == nil {
		return "", fmt.Errorf("DB not initialized")
	}

	switch {
	case deprecate != "":
		return t.deprecateHolon(deprecate)
	case waiveID != "":
		if waiveUntil == "" || waiveRationale == "" {
			return "", fmt.Errorf("waive requires both --until and --rationale parameters")
		}
		return t.createWaiver(waiveID, waiveUntil, waiveRationale)
	default:
		return t.generateFreshnessReport()
	}
}

func (t *Tools) deprecateHolon(holonID string) (string, error) {
	ctx := context.Background()
	holon, err := t.DB.GetHolon(ctx, holonID)
	if err != nil {
		return "", fmt.Errorf("holon not found: %s", holonID)
	}

	var newLayer string
	switch holon.Layer {
	case "L2":
		newLayer = "L1"
	case "L1":
		newLayer = "L0"
	default:
		return "", fmt.Errorf("cannot deprecate %s from %s (only L2 and L1 can be deprecated)", holonID, holon.Layer)
	}

	if _, err := t.MoveHypothesis(holonID, holon.Layer, newLayer); err != nil {
		return "", err
	}

	t.AuditLog("quint_check_decay", "deprecate", "user", holonID, "SUCCESS",
		map[string]string{"from": holon.Layer, "to": newLayer}, "Evidence expired, holon deprecated")

	return fmt.Sprintf("Deprecated: %s %s → %s\n\nThis decision now requires re-evaluation.\nNext step: Run /q1-hypothesize to explore alternatives.", holonID, holon.Layer, newLayer), nil
}

func (t *Tools) createWaiver(evidenceID, until, rationale string) (string, error) {
	ctx := context.Background()

	_, err := t.DB.GetEvidenceByID(ctx, evidenceID)
	if err != nil {
		return "", fmt.Errorf("evidence not found: %s", evidenceID)
	}

	untilTime, err := time.Parse("2006-01-02", until)
	if err != nil {
		untilTime, err = time.Parse(time.RFC3339, until)
		if err != nil {
			return "", fmt.Errorf("invalid date format: %s (use YYYY-MM-DD or RFC3339)", until)
		}
	}

	if untilTime.Before(time.Now()) {
		return "", fmt.Errorf("waive_until must be a future date")
	}

	id := uuid.New().String()
	if err := t.DB.CreateWaiver(ctx, id, evidenceID, "user", untilTime, rationale); err != nil {
		return "", fmt.Errorf("failed to create waiver: %v", err)
	}

	t.AuditLog("quint_check_decay", "waive", "user", evidenceID, "SUCCESS",
		map[string]string{"until": until, "rationale": rationale}, "")

	return fmt.Sprintf(`Waiver recorded:
- Evidence: %s
- Waived until: %s
- Rationale: %s

⚠️ This evidence returns to EXPIRED status after %s.
   Set a reminder to run /q3-validate before then.`, evidenceID, until, rationale, until), nil
}

func (t *Tools) generateFreshnessReport() (string, error) {
	ctx := context.Background()
	rawDB := t.DB.GetRawDB()

	rows, err := rawDB.QueryContext(ctx, `
		SELECT
			e.id as evidence_id,
			e.holon_id,
			h.title,
			h.layer,
			e.type as evidence_type,
			CAST(JULIANDAY('now') - JULIANDAY(substr(e.valid_until, 1, 10)) AS INTEGER) as days_overdue
		FROM evidence e
		JOIN holons h ON e.holon_id = h.id
		LEFT JOIN (
			SELECT evidence_id, MAX(waived_until) as latest_waiver
			FROM waivers
			GROUP BY evidence_id
		) w ON e.id = w.evidence_id
		WHERE e.valid_until IS NOT NULL
		  AND substr(e.valid_until, 1, 10) < date('now')
		  AND (w.latest_waiver IS NULL OR w.latest_waiver < datetime('now'))
		ORDER BY h.id, days_overdue DESC
	`)
	if err != nil {
		return "", err
	}
	defer rows.Close() //nolint:errcheck

	type evidenceInfo struct {
		ID          string
		Type        string
		DaysOverdue int
	}

	staleHolons := make(map[string][]evidenceInfo)
	holonTitles := make(map[string]string)
	holonLayers := make(map[string]string)

	for rows.Next() {
		var evidenceID, holonID, title, layer, evidenceType string
		var daysOverdue int
		if err := rows.Scan(&evidenceID, &holonID, &title, &layer, &evidenceType, &daysOverdue); err != nil {
			continue
		}
		holonTitles[holonID] = title
		holonLayers[holonID] = layer
		staleHolons[holonID] = append(staleHolons[holonID], evidenceInfo{
			ID:          evidenceID,
			Type:        evidenceType,
			DaysOverdue: daysOverdue,
		})
	}

	waivedRows, err := rawDB.QueryContext(ctx, `
		SELECT w.evidence_id, e.holon_id, h.title, w.waived_until, w.waived_by, w.rationale,
		       CAST(JULIANDAY(w.waived_until) - JULIANDAY('now') AS INTEGER) as days_until_expiry
		FROM waivers w
		JOIN evidence e ON w.evidence_id = e.id
		JOIN holons h ON e.holon_id = h.id
		WHERE w.waived_until > datetime('now')
		ORDER BY w.waived_until ASC
	`)
	if err != nil {
		return "", err
	}
	defer waivedRows.Close() //nolint:errcheck

	type waiverInfo struct {
		EvidenceID      string
		HolonID         string
		HolonTitle      string
		WaivedUntil     string
		WaivedBy        string
		Rationale       string
		DaysUntilExpiry int
	}

	var activeWaivers []waiverInfo
	for waivedRows.Next() {
		var info waiverInfo
		if err := waivedRows.Scan(&info.EvidenceID, &info.HolonID, &info.HolonTitle, &info.WaivedUntil, &info.WaivedBy, &info.Rationale, &info.DaysUntilExpiry); err != nil {
			continue
		}
		activeWaivers = append(activeWaivers, info)
	}

	var result strings.Builder
	result.WriteString("## Evidence Freshness Report\n\n")

	if len(staleHolons) == 0 {
		result.WriteString("### All holons FRESH ✓\n\nNo expired evidence found.\n")
	} else {
		result.WriteString(fmt.Sprintf("### STALE (%d holons require action)\n\n", len(staleHolons)))

		for holonID, evidenceItems := range staleHolons {
			result.WriteString(fmt.Sprintf("#### %s (%s)\n", holonTitles[holonID], holonLayers[holonID]))
			result.WriteString("| ID | Type | Status | Details |\n")
			result.WriteString("|-----|------|--------|--------|\n")
			for _, item := range evidenceItems {
				result.WriteString(fmt.Sprintf("| %s | %s | EXPIRED | %d days overdue |\n", item.ID, item.Type, item.DaysOverdue))
			}
			result.WriteString("\nActions:\n")
			result.WriteString(fmt.Sprintf("  → /q3-validate %s (refresh)\n", holonID))
			result.WriteString(fmt.Sprintf("  → /q-decay --deprecate %s (downgrade)\n", holonID))
			result.WriteString("  → /q-decay --waive <evidence_id> --until <date> --rationale \"...\"\n\n")
		}
	}

	if len(activeWaivers) > 0 {
		result.WriteString("---\n\n### WAIVED (temporary risk acceptance)\n\n")
		result.WriteString("| Holon | Evidence | Waived Until | By | Rationale |\n")
		result.WriteString("|-------|----------|--------------|----|-----------|\n")
		for _, w := range activeWaivers {
			waivedUntilShort := w.WaivedUntil
			if len(waivedUntilShort) > 10 {
				waivedUntilShort = waivedUntilShort[:10]
			}
			result.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", w.HolonTitle, w.EvidenceID, waivedUntilShort, w.WaivedBy, w.Rationale))
		}
		for _, w := range activeWaivers {
			if w.DaysUntilExpiry <= 30 {
				result.WriteString(fmt.Sprintf("\n⚠️ Waiver for %s expires in %d days\n", w.EvidenceID, w.DaysUntilExpiry))
			}
		}
	}

	return result.String(), nil
}

type InternalizeResult struct {
	Status            string            // INITIALIZED, UPDATED, READY
	Phase             string            // IDLE, ABDUCTION, DEDUCTION, INDUCTION, AUDIT, DECISION
	Role              string            // Observer, Initializer, Abductor, etc.
	ContextID         string            // Current bounded context identifier
	ContextChanges    []string          // What changed (if INITIALIZED or UPDATED)
	LayerCounts       map[string]int    // Active holons: {"L0": 5, "L1": 3, "L2": 1}
	ArchivedCounts    map[string]int    // Holons in resolved decisions (historical)
	RecentHolons      []HolonSummary    // Last N active holons (not in resolved decisions)
	DecayWarnings     []DecayWarning    // Evidence expiring soon
	OpenDecisions     []DecisionSummary // Decisions awaiting resolution
	ResolvedDecisions []DecisionSummary // Recently resolved decisions
	NextAction        string            // Phase-appropriate suggestion
	// Code Change Awareness (v5.0.0)
	CodeChanges         *CodeChangeDetectionResult
	StaleEvidenceCount  int64
	ReverificationCount int64
}

type HolonSummary struct {
	ID        string
	Title     string
	Layer     string
	Kind      string
	RScore    float64
	UpdatedAt time.Time
}

type DecayWarning struct {
	EvidenceID string
	HolonID    string
	HolonTitle string
	ExpiresAt  time.Time
	DaysLeft   int
}

// Internalize is the unified entry point for FPF sessions.
// It handles initialization, context updates, decay checking, and status in one call.
func (t *Tools) Internalize() (string, error) {
	defer t.RecordWork("Internalize", time.Now())

	result := InternalizeResult{
		Phase:       string(t.FSM.GetPhase()),
		Role:        string(GetExpectedRole(t.FSM.GetPhase())),
		LayerCounts: make(map[string]int),
	}

	if !t.IsInitialized() {
		if err := t.InitProject(); err != nil {
			return "", fmt.Errorf("initialization failed: %w", err)
		}
		result.Status = "INITIALIZED"
		result.ContextChanges = []string{"Created .quint/ structure"}

		// Auto-analyze project and record context
		ctx, err := t.AnalyzeProject()
		if err != nil {
			result.ContextChanges = append(result.ContextChanges, fmt.Sprintf("Warning: auto-analysis failed: %v", err))
		} else {
			if _, err := t.RecordContext(ctx.Vocabulary, ctx.Invariants); err != nil {
				result.ContextChanges = append(result.ContextChanges, fmt.Sprintf("Warning: failed to record context: %v", err))
			} else {
				result.ContextChanges = append(result.ContextChanges, "Auto-generated context from project analysis")
			}
		}

		// Set phase to ABDUCTION after init
		t.FSM.State.Phase = PhaseAbduction
		if err := t.FSM.SaveState("default"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save state: %v\n", err)
		}
		result.Phase = string(PhaseAbduction)
		result.Role = string(GetExpectedRole(PhaseAbduction))
	} else {
		stale, signals := t.IsContextStale()
		if stale {
			ctx, err := t.AnalyzeProject()
			if err != nil {
				result.ContextChanges = append(result.ContextChanges, fmt.Sprintf("Warning: re-analysis failed: %v", err))
			} else {
				if _, err := t.RecordContext(ctx.Vocabulary, ctx.Invariants); err != nil {
					result.ContextChanges = append(result.ContextChanges, fmt.Sprintf("Warning: failed to update context: %v", err))
				}
			}
			result.Status = "UPDATED"
			result.ContextChanges = signals
		} else {
			result.Status = "READY"
		}
	}

	result.ContextID = "default"
	result.ArchivedCounts = make(map[string]int)

	// Code Change Detection (v5.0.0)
	if t.DB != nil && t.isGitRepo() {
		ctx := context.Background()
		codeChanges, err := t.detectCodeChanges(ctx)
		if err != nil {
			result.ContextChanges = append(result.ContextChanges,
				fmt.Sprintf("Warning: code change detection failed: %v", err))
		} else if codeChanges != nil && len(codeChanges.Impacts) > 0 {
			result.CodeChanges = codeChanges
			result.ContextChanges = append(result.ContextChanges,
				fmt.Sprintf("Detected %d code change impacts across %d files",
					len(codeChanges.Impacts), len(codeChanges.ChangedFiles)))
		}

		// Get staleness counts for summary
		result.StaleEvidenceCount, _ = t.DB.CountStaleEvidence(ctx)
		result.ReverificationCount, _ = t.DB.CountHolonsNeedingReverification(ctx)
	}

	if t.DB != nil {
		ctx := context.Background()

		activeCounts, err := t.DB.CountActiveHolonsByLayer(ctx)
		if err == nil {
			for _, c := range activeCounts {
				result.LayerCounts[c.Layer] = int(c.Count)
			}
		} else {
			result.LayerCounts["L0"] = t.countHolons("L0")
			result.LayerCounts["L1"] = t.countHolons("L1")
			result.LayerCounts["L2"] = t.countHolons("L2")
		}

		archivedCounts, err := t.DB.CountArchivedHolonsByLayer(ctx)
		if err == nil {
			for _, c := range archivedCounts {
				result.ArchivedCounts[c.Layer] = int(c.Count)
			}
		}
	} else {
		result.LayerCounts["L0"] = t.countHolons("L0")
		result.LayerCounts["L1"] = t.countHolons("L1")
		result.LayerCounts["L2"] = t.countHolons("L2")
	}
	result.LayerCounts["DRR"] = t.countDRRs()

	if t.DB != nil {
		ctx := context.Background()
		holons, err := t.DB.GetActiveRecentHolons(ctx, 10)
		if err == nil {
			for _, h := range holons {
				summary := HolonSummary{
					ID:    h.ID,
					Title: h.Title,
					Layer: h.Layer,
				}
				if h.Kind.Valid {
					summary.Kind = h.Kind.String
				}
				if h.CachedRScore.Valid {
					summary.RScore = h.CachedRScore.Float64
				}
				if h.UpdatedAt.Valid {
					summary.UpdatedAt = h.UpdatedAt.Time
				}
				result.RecentHolons = append(result.RecentHolons, summary)
			}
		}

		// Surface evidence expiring within 7 days
		evidence, err := t.DB.GetDecayingEvidence(ctx, 7)
		if err == nil {
			for _, e := range evidence {
				warning := DecayWarning{
					EvidenceID: e.ID,
					HolonID:    e.HolonID,
				}
				if e.ValidUntil.Valid {
					warning.ExpiresAt = e.ValidUntil.Time
					warning.DaysLeft = int(time.Until(e.ValidUntil.Time).Hours() / 24)
				}
				if title, err := t.DB.GetHolonTitle(ctx, e.HolonID); err == nil {
					warning.HolonTitle = title
				}
				result.DecayWarnings = append(result.DecayWarnings, warning)
			}
		}

		openDecisions, err := t.GetOpenDecisions(ctx)
		if err == nil {
			result.OpenDecisions = openDecisions
		}
		resolvedDecisions, err := t.GetRecentResolvedDecisions(ctx, 5)
		if err == nil {
			result.ResolvedDecisions = resolvedDecisions
		}
	}

	result.NextAction = t.getNextAction(t.FSM.GetPhase(), result.LayerCounts["L0"], result.LayerCounts["L1"], result.LayerCounts["L2"])
	return t.formatInternalizeOutput(result), nil
}

func (t *Tools) formatInternalizeOutput(r InternalizeResult) string {
	var sb strings.Builder

	sb.WriteString("=== QUINT INTERNALIZE ===\n\n")
	sb.WriteString(fmt.Sprintf("Status: %s\n", r.Status))
	sb.WriteString(fmt.Sprintf("Phase: %s\n", r.Phase))
	sb.WriteString(fmt.Sprintf("Role: %s\n", r.Role))
	sb.WriteString(fmt.Sprintf("Context: %s\n\n", r.ContextID))

	if len(r.ContextChanges) > 0 {
		sb.WriteString("Context Changes:\n")
		for _, c := range r.ContextChanges {
			sb.WriteString(fmt.Sprintf("  - %s\n", c))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Knowledge State (Active):\n")
	sb.WriteString(fmt.Sprintf("  L0 (Conjecture): %d\n", r.LayerCounts["L0"]))
	sb.WriteString(fmt.Sprintf("  L1 (Substantiated): %d\n", r.LayerCounts["L1"]))
	sb.WriteString(fmt.Sprintf("  L2 (Corroborated): %d\n", r.LayerCounts["L2"]))
	if r.LayerCounts["DRR"] > 0 {
		sb.WriteString(fmt.Sprintf("  DRRs: %d\n", r.LayerCounts["DRR"]))
	}

	// Show archived counts if any exist
	totalArchived := r.ArchivedCounts["L0"] + r.ArchivedCounts["L1"] + r.ArchivedCounts["L2"]
	if totalArchived > 0 {
		sb.WriteString(fmt.Sprintf("  (Archived: %d holons in resolved decisions)\n", totalArchived))
	}
	sb.WriteString("\n")

	if len(r.RecentHolons) > 0 {
		sb.WriteString("Recent Active Holons:\n")
		for _, h := range r.RecentHolons {
			age := formatAge(h.UpdatedAt)
			sb.WriteString(fmt.Sprintf("  - %s [%s] R=%.2f - %s\n", h.ID, h.Layer, h.RScore, age))
		}
		sb.WriteString("\n")
	}

	if len(r.DecayWarnings) > 0 {
		sb.WriteString("⚠ Attention Required:\n")
		for _, w := range r.DecayWarnings {
			sb.WriteString(fmt.Sprintf("  - Evidence \"%s\" for \"%s\" expires in %d days\n",
				w.EvidenceID, w.HolonTitle, w.DaysLeft))
		}
		sb.WriteString("\n")
	}

	// Code Change Warnings (v5.0.0)
	if r.StaleEvidenceCount > 0 || r.ReverificationCount > 0 {
		sb.WriteString("🔴 CODE CHANGE IMPACTS:\n")
		if r.StaleEvidenceCount > 0 {
			sb.WriteString(fmt.Sprintf("  - %d evidence records marked stale\n", r.StaleEvidenceCount))
		}
		if r.ReverificationCount > 0 {
			sb.WriteString(fmt.Sprintf("  - %d holons need re-verification\n", r.ReverificationCount))
		}
		if r.CodeChanges != nil && len(r.CodeChanges.Impacts) > 0 {
			sb.WriteString("  Details:\n")
			shown := 0
			for _, impact := range r.CodeChanges.Impacts {
				if shown >= 5 {
					sb.WriteString(fmt.Sprintf("    ... and %d more\n", len(r.CodeChanges.Impacts)-5))
					break
				}
				if impact.File != "" {
					sb.WriteString(fmt.Sprintf("    - %s: %s → %s [%s]\n",
						impact.File, impact.HolonID, impact.HolonTitle, impact.HolonLayer))
				} else {
					sb.WriteString(fmt.Sprintf("    - %s [%s]: %s\n",
						impact.HolonID, impact.HolonLayer, impact.Reason))
				}
				shown++
			}
		}
		sb.WriteString("  Use quint_test or quint_verify to clear stale flags after re-validation.\n\n")
	}

	if len(r.OpenDecisions) > 0 {
		sb.WriteString("⚠ Open Decisions (awaiting resolution):\n")
		for _, d := range r.OpenDecisions {
			age := formatAge(d.CreatedAt)
			sb.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", d.ID, d.Title, age))
		}
		sb.WriteString("\n")
	}

	if len(r.ResolvedDecisions) > 0 {
		sb.WriteString("Recent Resolutions:\n")
		for _, d := range r.ResolvedDecisions {
			age := formatAge(d.ResolvedAt)
			sb.WriteString(fmt.Sprintf("  - %s: %s [%s] %s\n", d.ID, d.Title, d.Resolution, age))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Next Action: %s", r.NextAction))

	return sb.String()
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// IsInitialized checks if .quint/ directory exists.
func (t *Tools) IsInitialized() bool {
	_, err := os.Stat(t.GetFPFDir())
	return err == nil
}

// ProjectContext holds auto-analyzed project information.
type ProjectContext struct {
	Vocabulary string
	Invariants string
	TechStack  []string
}

// AnalyzeProject scans the project to extract context automatically.
func (t *Tools) AnalyzeProject() (ProjectContext, error) {
	ctx := ProjectContext{}
	var vocab []string
	var invariants []string

	// Check go.mod
	goModPath := filepath.Join(t.RootDir, "go.mod")
	if content, err := os.ReadFile(goModPath); err == nil {
		ctx.TechStack = append(ctx.TechStack, "Go")
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "module ") {
				modName := strings.TrimPrefix(line, "module ")
				vocab = append(vocab, fmt.Sprintf("Module: %s", strings.TrimSpace(modName)))
			}
		}
		invariants = append(invariants, "Go module project")
	}

	// Check package.json
	pkgPath := filepath.Join(t.RootDir, "package.json")
	if _, err := os.Stat(pkgPath); err == nil {
		ctx.TechStack = append(ctx.TechStack, "Node.js")
		invariants = append(invariants, "Node.js project")
	}

	// Check Python markers
	pythonMarkers := []string{"requirements.txt", "pyproject.toml", "setup.py", "Pipfile"}
	for _, marker := range pythonMarkers {
		if _, err := os.Stat(filepath.Join(t.RootDir, marker)); err == nil {
			ctx.TechStack = append(ctx.TechStack, "Python")
			invariants = append(invariants, "Python project")
			break
		}
	}

	// Check Rust (Cargo.toml)
	if _, err := os.Stat(filepath.Join(t.RootDir, "Cargo.toml")); err == nil {
		ctx.TechStack = append(ctx.TechStack, "Rust")
		invariants = append(invariants, "Rust project")
	}

	// Check Java/Kotlin (Maven or Gradle)
	if _, err := os.Stat(filepath.Join(t.RootDir, "pom.xml")); err == nil {
		ctx.TechStack = append(ctx.TechStack, "Java (Maven)")
		invariants = append(invariants, "Maven project")
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "build.gradle")); err == nil {
		ctx.TechStack = append(ctx.TechStack, "Java/Kotlin (Gradle)")
		invariants = append(invariants, "Gradle project")
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "build.gradle.kts")); err == nil {
		ctx.TechStack = append(ctx.TechStack, "Kotlin (Gradle KTS)")
		invariants = append(invariants, "Gradle Kotlin DSL project")
	}

	// Check Ruby (Gemfile)
	if _, err := os.Stat(filepath.Join(t.RootDir, "Gemfile")); err == nil {
		ctx.TechStack = append(ctx.TechStack, "Ruby")
		invariants = append(invariants, "Ruby project")
	}

	// Check Make-based build
	if _, err := os.Stat(filepath.Join(t.RootDir, "Makefile")); err == nil {
		ctx.TechStack = append(ctx.TechStack, "Make")
		invariants = append(invariants, "Make-based build")
	}

	// Fallback: if git repo but no recognized markers
	if len(ctx.TechStack) == 0 {
		if _, err := os.Stat(filepath.Join(t.RootDir, ".git")); err == nil {
			ctx.TechStack = append(ctx.TechStack, "Unknown")
			invariants = append(invariants, "Git repository (unknown project type)")
		}
	}

	// Check for common directories
	if _, err := os.Stat(filepath.Join(t.RootDir, "src")); err == nil {
		vocab = append(vocab, "src: Source code directory")
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "internal")); err == nil {
		vocab = append(vocab, "internal: Private Go packages")
	}
	if _, err := os.Stat(filepath.Join(t.RootDir, "cmd")); err == nil {
		vocab = append(vocab, "cmd: Command-line entry points")
	}

	ctx.Vocabulary = strings.Join(vocab, ". ")
	ctx.Invariants = strings.Join(invariants, ". ")

	return ctx, nil
}

// IsContextStale checks if context.md is stale relative to project files.
func (t *Tools) IsContextStale() (bool, []string) {
	var signals []string

	contextPath := filepath.Join(t.GetFPFDir(), "context.md")
	contextInfo, err := os.Stat(contextPath)
	if err != nil {
		// context.md doesn't exist - needs to be created
		return true, []string{"No context.md found, creating initial context"}
	}
	contextMod := contextInfo.ModTime()

	// Check go.mod
	goModPath := filepath.Join(t.RootDir, "go.mod")
	if info, err := os.Stat(goModPath); err == nil {
		if info.ModTime().After(contextMod) {
			signals = append(signals, "go.mod modified since last context update")
		}
	}

	// Check package.json
	pkgPath := filepath.Join(t.RootDir, "package.json")
	if info, err := os.Stat(pkgPath); err == nil {
		if info.ModTime().After(contextMod) {
			signals = append(signals, "package.json modified since last context update")
		}
	}

	// Check if context is older than 7 days
	if time.Since(contextMod) > 7*24*time.Hour {
		signals = append(signals, fmt.Sprintf("Context is %d days old", int(time.Since(contextMod).Hours()/24)))
	}

	return len(signals) > 0, signals
}

// Search performs full-text search across the knowledge base.
func (t *Tools) Search(query, scope, layerFilter, statusFilter, affectedScopeFilter string, limit int) (string, error) {
	defer t.RecordWork("Search", time.Now())

	if t.DB == nil {
		return "", fmt.Errorf("database not initialized - run quint_internalize first")
	}

	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	ctx := context.Background()
	results, err := t.DB.Search(ctx, query, scope, layerFilter, statusFilter, limit)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	// Filter by affected_scope if provided
	if affectedScopeFilter != "" {
		results = filterByAffectedScope(results, affectedScopeFilter)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No results found for: %s", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Search Results for: %s\n\n", query))
	sb.WriteString(fmt.Sprintf("Found %d results\n\n", len(results)))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("- **ID:** %s\n", r.ID))
		sb.WriteString(fmt.Sprintf("- **Type:** %s\n", r.Type))
		if r.Layer != "" {
			sb.WriteString(fmt.Sprintf("- **Layer:** %s\n", r.Layer))
		}
		if r.RScore > 0 {
			sb.WriteString(fmt.Sprintf("- **R_eff:** %.2f\n", r.RScore))
		}
		if !r.UpdatedAt.IsZero() {
			sb.WriteString(fmt.Sprintf("- **Updated:** %s\n", formatAge(r.UpdatedAt)))
		}
		if r.Snippet != "" {
			sb.WriteString(fmt.Sprintf("- **Snippet:** %s\n", r.Snippet))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// filterByAffectedScope filters search results by matching a file path against affected_scope patterns.
// The affectedScopeFilter is a file path, and each result's Scope contains a JSON array of glob patterns.
func filterByAffectedScope(results []db.SearchResult, affectedScopeFilter string) []db.SearchResult {
	var filtered []db.SearchResult

	for _, r := range results {
		if r.Scope == "" {
			continue
		}

		// Parse the scope as JSON array of patterns
		var patterns []string
		if err := json.Unmarshal([]byte(r.Scope), &patterns); err != nil {
			// Not a JSON array, try as a single pattern
			patterns = []string{r.Scope}
		}

		// Check if the filter path matches any pattern
		for _, pattern := range patterns {
			// Try glob match
			matched, err := filepath.Match(pattern, affectedScopeFilter)
			if err == nil && matched {
				filtered = append(filtered, r)
				break
			}

			// Also check if the pattern is a prefix (for directory patterns like "src/mcp/*")
			if strings.HasSuffix(pattern, "/*") || strings.HasSuffix(pattern, "/**") {
				prefix := strings.TrimSuffix(strings.TrimSuffix(pattern, "/*"), "/**")
				if strings.HasPrefix(affectedScopeFilter, prefix) {
					filtered = append(filtered, r)
					break
				}
			}

			// Direct substring check for simple patterns
			if strings.Contains(affectedScopeFilter, pattern) || strings.Contains(pattern, affectedScopeFilter) {
				filtered = append(filtered, r)
				break
			}
		}
	}

	return filtered
}

// GetStatus returns the current FPF status with enhanced output for agent parsing.
func (t *Tools) GetStatus() (string, error) {
	phase := t.FSM.GetPhase()

	var sb strings.Builder

	// Parseable header
	sb.WriteString(fmt.Sprintf("PHASE: %s\n", phase))
	sb.WriteString(fmt.Sprintf("EXPECTED_ROLE: %s\n\n", GetExpectedRole(phase)))

	// Knowledge counts from filesystem
	l0 := t.countHolons("L0")
	l1 := t.countHolons("L1")
	l2 := t.countHolons("L2")
	drr := t.countDRRs()

	sb.WriteString("## Knowledge\n")
	sb.WriteString(fmt.Sprintf("- L0 (Conjecture): %d\n", l0))
	sb.WriteString(fmt.Sprintf("- L1 (Substantiated): %d\n", l1))
	sb.WriteString(fmt.Sprintf("- L2 (Corroborated): %d\n", l2))
	if drr > 0 {
		sb.WriteString(fmt.Sprintf("- DRR (Decisions): %d\n", drr))
	}
	sb.WriteString("\n")

	// Next action guidance
	sb.WriteString("## Next\n")
	sb.WriteString(t.getNextAction(phase, l0, l1, l2))

	return sb.String(), nil
}

// countHolons counts markdown files in a knowledge layer directory.
func (t *Tools) countHolons(layer string) int {
	dir := filepath.Join(t.GetFPFDir(), "knowledge", layer)
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") && f.Name() != ".gitkeep" {
			count++
		}
	}
	return count
}

// countDRRs counts decision records in the decisions directory.
func (t *Tools) countDRRs() int {
	dir := filepath.Join(t.GetFPFDir(), "decisions")
	files, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") && strings.HasPrefix(f.Name(), "DRR-") {
			count++
		}
	}
	return count
}

// getNextAction returns guidance for the next step based on current phase.
func (t *Tools) getNextAction(phase Phase, l0, l1, l2 int) string {
	switch phase {
	case PhaseIdle:
		return "→ /q-internalize or /q1-hypothesize\n"
	case PhaseAbduction:
		if l0 > 0 {
			return fmt.Sprintf("→ %d L0 ready for /q2-verify\n", l0)
		}
		return "→ /q1-hypothesize to generate hypotheses\n"
	case PhaseDeduction:
		if l1 > 0 {
			return fmt.Sprintf("→ %d L1 ready for /q3-validate\n", l1)
		}
		return "→ /q2-verify to check logic\n"
	case PhaseInduction:
		if l2 > 0 {
			return fmt.Sprintf("→ %d L2 ready for /q4-audit\n", l2)
		}
		return "→ /q3-validate to gather evidence\n"
	case PhaseAudit:
		return "→ /q4-audit then /q5-decide\n"
	case PhaseDecision:
		return "→ /q5-decide to finalize\n"
	case PhaseOperation:
		return "→ In operation\n"
	default:
		return ""
	}
}

// ResolveInput defines the input for resolving a decision.
type ResolveInput struct {
	DecisionID       string `json:"decision_id"`
	Resolution       string `json:"resolution"`
	Reference        string `json:"reference"`
	SupersededBy     string `json:"superseded_by"`
	Notes            string `json:"notes"`
	ValidUntil       string `json:"valid_until"`
	CriteriaVerified bool   `json:"criteria_verified"`
}

// DecisionSummary represents a decision with its resolution status.
type DecisionSummary struct {
	ID         string
	Title      string
	CreatedAt  time.Time
	Resolution string
	ResolvedAt time.Time
	Notes      string
	Reference  string
}

// getDRRContract reads the contract from a DRR markdown file.
func (t *Tools) getDRRContract(decisionID string) (*Contract, error) {
	decisionsDir := filepath.Join(t.GetFPFDir(), "decisions")

	// Find DRR file by pattern: DRR-*-{decisionID}.md
	pattern := filepath.Join(decisionsDir, fmt.Sprintf("DRR-*-%s.md", decisionID))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil, nil // No DRR file found, not an error
	}

	// Read the file
	content, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read DRR file: %w", err)
	}

	// Parse frontmatter using existing function
	frontmatter, _, hasFM := parseFrontmatter(string(content))
	if !hasFM {
		return nil, nil // No valid frontmatter
	}

	// Extract contract JSON from frontmatter
	// Format: contract: {"invariants":...}
	contractPrefix := "contract: "
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.HasPrefix(line, contractPrefix) {
			contractJSON := strings.TrimPrefix(line, contractPrefix)
			var contract Contract
			if err := json.Unmarshal([]byte(contractJSON), &contract); err != nil {
				return nil, nil // Invalid contract JSON
			}
			return &contract, nil
		}
	}

	return nil, nil // No contract in frontmatter
}

// ConstraintSource tracks where a constraint came from
type ConstraintSource struct {
	DRRID       string
	DRRTitle    string
	Constraints []string
}

// InheritedConstraints holds constraints from dependency chain
type InheritedConstraints struct {
	Invariants   []ConstraintSource
	AntiPatterns []ConstraintSource
}

// DRRInfo holds loaded DRR data for implementation
type DRRInfo struct {
	ID         string
	Title      string
	Contract   *Contract
	DependsOn  []string
	WinnerID   string
}

// ============================================
// CODE CHANGE AWARENESS TYPES (v5.0.0)
// ============================================

// CodeChangeImpact represents the impact of a code change
type CodeChangeImpact struct {
	Type        string  // "evidence_stale" | "propagated"
	File        string  // Changed file path
	EvidenceID  string  // Affected evidence ID
	HolonID     string  // Affected holon ID
	HolonTitle  string  // Holon title for display
	HolonLayer  string  // L0/L1/L2/DRR
	PreviousR   float64 // R_eff before change
	Reason      string  // Human-readable reason
}

// CodeChangeDetectionResult holds all detection results
type CodeChangeDetectionResult struct {
	FromCommit    string
	ToCommit      string
	ChangedFiles  []string
	Impacts       []CodeChangeImpact
	TotalStale    int
	TotalAffected int
}

// ImplementationWarnings holds all warnings for quint_implement
type ImplementationWarnings struct {
	StaleEvidence    []StaleEvidenceWarning
	ChangedFiles     []ChangedFileWarning
	DependencyIssues []DependencyIssueWarning
}

// StaleEvidenceWarning represents stale evidence in dependency chain
type StaleEvidenceWarning struct {
	EvidenceID string
	CarrierRef string
	HolonID    string
	HolonTitle string
	HolonLayer string
	StaleSince time.Time
	Reason     string
}

// ChangedFileWarning represents a file that changed since DRR creation
type ChangedFileWarning struct {
	FilePath    string
	CommitCount int
}

// DependencyIssueWarning represents a problematic dependency
type DependencyIssueWarning struct {
	HolonID    string
	HolonTitle string
	Layer      string
	REff       float64
	Reason     string
}

// HasAny returns true if there are any warnings
func (w *ImplementationWarnings) HasAny() bool {
	return len(w.StaleEvidence) > 0 ||
		len(w.ChangedFiles) > 0 ||
		len(w.DependencyIssues) > 0
}

// ============================================
// GIT INTEGRATION (v5.0.0)
// ============================================

// isGitRepo checks if the root directory is a git repository
func (t *Tools) isGitRepo() bool {
	gitDir := filepath.Join(t.RootDir, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}

// getCurrentHead returns the current HEAD commit hash
func (t *Tools) getCurrentHead() (string, error) {
	if !t.isGitRepo() {
		return "", fmt.Errorf("not a git repository")
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = t.RootDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// getChangedFiles returns files changed between two commits
func (t *Tools) getChangedFiles(fromCommit, toCommit string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", fromCommit, toCommit)
	cmd.Dir = t.RootDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// getFileHash returns SHA256 hash of file contents
func (t *Tools) getFileHash(filePath string) (string, error) {
	fullPath := filepath.Join(t.RootDir, filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:]), nil
}

// ============================================
// CODE CHANGE DETECTION LOGIC (v5.0.0)
// ============================================

// detectCodeChanges detects code changes and marks affected evidence as stale
func (t *Tools) detectCodeChanges(ctx context.Context) (*CodeChangeDetectionResult, error) {
	result := &CodeChangeDetectionResult{}

	// Skip if not a git repo
	if !t.isGitRepo() {
		return nil, nil
	}

	// Skip if no DB
	if t.DB == nil {
		return nil, nil
	}

	// 1. Get last known commit
	lastCommit, _, err := t.DB.GetLastCommit(ctx, "default")
	if err != nil || lastCommit == "" {
		// No previous commit recorded — check each evidence's carrier_commit (v5.0.0)
		currentHead, err := t.getCurrentHead()
		if err != nil {
			return nil, nil // Silent fail for non-git repos
		}
		// On first run, check carrier_commit instead of skipping
		result, err := t.detectStaleEvidenceByCarrierCommit(ctx, currentHead)
		// Save baseline for future incremental checks
		t.DB.UpdateLastCommit(ctx, "default", currentHead)
		return result, err
	}

	// 2. Get current HEAD
	currentHead, err := t.getCurrentHead()
	if err != nil {
		return nil, nil
	}

	result.FromCommit = lastCommit
	result.ToCommit = currentHead

	// No changes
	if currentHead == lastCommit {
		return nil, nil
	}

	// 3. Get changed files
	changedFiles, err := t.getChangedFiles(lastCommit, currentHead)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	if len(changedFiles) == 0 {
		t.DB.UpdateLastCommit(ctx, "default", currentHead)
		return nil, nil
	}

	result.ChangedFiles = changedFiles

	// 4. Find affected evidence for each changed file
	affectedHolons := make(map[string]bool)

	for _, file := range changedFiles {
		// Search for evidence with carrier_ref matching this file
		evidenceList, err := t.DB.GetEvidenceByCarrierPattern(ctx, "%"+file+"%")
		if err != nil {
			continue
		}

		for _, e := range evidenceList {
			// Skip already stale
			if e.IsStale.Valid && e.IsStale.Int64 == 1 {
				continue
			}

			// Get holon's current R for reporting
			holon, _ := t.DB.GetHolon(ctx, e.HolonID)
			previousR := 0.0
			if holon.CachedRScore.Valid {
				previousR = holon.CachedRScore.Float64
			}

			// Mark evidence as stale
			shortFrom := lastCommit
			if len(shortFrom) > 7 {
				shortFrom = shortFrom[:7]
			}
			shortTo := currentHead
			if len(shortTo) > 7 {
				shortTo = shortTo[:7]
			}
			reason := fmt.Sprintf("carrier file '%s' changed (%s → %s)",
				file, shortFrom, shortTo)

			if err := t.DB.MarkEvidenceStale(ctx, e.ID, reason); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to mark evidence stale: %v\n", err)
				continue
			}

			// Record impact
			result.Impacts = append(result.Impacts, CodeChangeImpact{
				Type:       "evidence_stale",
				File:       file,
				EvidenceID: e.ID,
				HolonID:    e.HolonID,
				HolonTitle: e.HolonTitle,
				HolonLayer: e.HolonLayer,
				PreviousR:  previousR,
				Reason:     reason,
			})

			result.TotalStale++
			affectedHolons[e.HolonID] = true
		}
	}

	// 5. Mark affected holons as needing reverification
	for holonID := range affectedHolons {
		reason := "evidence became stale due to code changes"
		if err := t.DB.MarkHolonNeedsReverification(ctx, holonID, reason); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to mark holon for reverification: %v\n", err)
		}
		result.TotalAffected++
	}

	// 6. Propagate staleness through WLNK chain
	propagated := t.propagateStalenessToDependent(ctx, affectedHolons)
	result.Impacts = append(result.Impacts, propagated...)

	// 7. Recalculate R_eff for all affected holons
	for holonID := range affectedHolons {
		t.recalculateHolonR(ctx, holonID)
	}

	// 8. Update last commit
	t.DB.UpdateLastCommit(ctx, "default", currentHead)

	return result, nil
}

// propagateStalenessToDependent marks holons that depend on stale holons
func (t *Tools) propagateStalenessToDependent(ctx context.Context, staleHolons map[string]bool) []CodeChangeImpact {
	var impacts []CodeChangeImpact

	for staleID := range staleHolons {
		// Find holons that depend ON this stale holon
		dependents, err := t.DB.GetDependents(ctx, staleID)
		if err != nil {
			continue
		}

		for _, dep := range dependents {
			// Skip if already marked
			if staleHolons[dep.SourceID] {
				continue
			}

			holon, err := t.DB.GetHolon(ctx, dep.SourceID)
			if err != nil {
				continue
			}

			previousR := 0.0
			if holon.CachedRScore.Valid {
				previousR = holon.CachedRScore.Float64
			}

			reason := fmt.Sprintf("depends on stale holon '%s'", staleID)
			t.DB.MarkHolonNeedsReverification(ctx, dep.SourceID, reason)

			impacts = append(impacts, CodeChangeImpact{
				Type:       "propagated",
				HolonID:    dep.SourceID,
				HolonTitle: holon.Title,
				HolonLayer: holon.Layer,
				PreviousR:  previousR,
				Reason:     reason,
			})

			// Mark for cascade
			staleHolons[dep.SourceID] = true
		}
	}

	return impacts
}

// recalculateHolonR triggers R_eff recalculation for a holon
func (t *Tools) recalculateHolonR(ctx context.Context, holonID string) {
	calc := assurance.New(t.DB.GetRawDB())
	report, err := calc.CalculateReliability(ctx, holonID)
	if err == nil && report != nil {
		t.DB.UpdateHolonRScore(ctx, holonID, report.FinalScore)
	}
}

// fileChangedBetween checks if a file changed between two commits
func (t *Tools) fileChangedBetween(fromCommit, toCommit, filePath string) (bool, error) {
	changedFiles, err := t.getChangedFiles(fromCommit, toCommit)
	if err != nil {
		return false, err
	}
	for _, f := range changedFiles {
		if f == filePath || strings.HasSuffix(f, "/"+filePath) || strings.HasPrefix(filePath, f) {
			return true, nil
		}
	}
	return false, nil
}

// detectStaleEvidenceByCarrierCommit checks each evidence's carrier_commit vs current HEAD (v5.0.0)
// Used on first internalize when no last_commit baseline exists.
func (t *Tools) detectStaleEvidenceByCarrierCommit(ctx context.Context, currentHead string) (*CodeChangeDetectionResult, error) {
	result := &CodeChangeDetectionResult{ToCommit: currentHead}
	affectedHolons := make(map[string]bool)

	// Get all evidence with carrier_commit
	allEvidence, err := t.DB.GetEvidenceWithCarrierCommit(ctx)
	if err != nil {
		return nil, err
	}

	for _, e := range allEvidence {
		if !e.CarrierCommit.Valid || e.CarrierCommit.String == "" {
			continue
		}
		if !e.CarrierRef.Valid || e.CarrierRef.String == "" {
			continue
		}

		carrierCommit := e.CarrierCommit.String
		carrierRef := e.CarrierRef.String

		// Skip if evidence was created at current HEAD
		if carrierCommit == currentHead {
			continue
		}

		// Check if file changed between evidence creation and now
		changed, err := t.fileChangedBetween(carrierCommit, currentHead, carrierRef)
		if err != nil || !changed {
			continue
		}

		// Get holon's current R for reporting
		holon, _ := t.DB.GetHolon(ctx, e.HolonID)
		previousR := 0.0
		if holon.CachedRScore.Valid {
			previousR = holon.CachedRScore.Float64
		}

		// Mark evidence as stale
		shortFrom := carrierCommit
		if len(shortFrom) > 7 {
			shortFrom = shortFrom[:7]
		}
		shortTo := currentHead
		if len(shortTo) > 7 {
			shortTo = shortTo[:7]
		}
		reason := fmt.Sprintf("carrier changed since evidence creation (%s → %s)", shortFrom, shortTo)

		if err := t.DB.MarkEvidenceStale(ctx, e.ID, reason); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to mark evidence stale: %v\n", err)
			continue
		}

		// Record impact
		result.Impacts = append(result.Impacts, CodeChangeImpact{
			Type:       "evidence_stale",
			File:       carrierRef,
			EvidenceID: e.ID,
			HolonID:    e.HolonID,
			HolonTitle: e.HolonTitle,
			HolonLayer: e.HolonLayer,
			PreviousR:  previousR,
			Reason:     reason,
		})

		result.TotalStale++
		affectedHolons[e.HolonID] = true
	}

	// Mark affected holons as needing reverification
	for holonID := range affectedHolons {
		reason := "evidence became stale due to code changes"
		if err := t.DB.MarkHolonNeedsReverification(ctx, holonID, reason); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to mark holon for reverification: %v\n", err)
		}
		result.TotalAffected++
	}

	// Propagate staleness through WLNK chain
	propagated := t.propagateStalenessToDependent(ctx, affectedHolons)
	result.Impacts = append(result.Impacts, propagated...)

	// Recalculate R_eff for all affected holons
	for holonID := range affectedHolons {
		t.recalculateHolonR(ctx, holonID)
	}

	return result, nil
}

// collectImplementationWarnings gathers warnings about stale evidence in dependency chain
func (t *Tools) collectImplementationWarnings(ctx context.Context, drrID string, dependsOn []string) *ImplementationWarnings {
	warnings := &ImplementationWarnings{}

	// Check for stale evidence in this DRR and its dependencies
	holonsToCheck := append([]string{drrID}, dependsOn...)
	visited := make(map[string]bool)

	var checkHolon func(holonID string, depth int)
	checkHolon = func(holonID string, depth int) {
		if depth > 10 || visited[holonID] {
			return
		}
		visited[holonID] = true

		// Get stale evidence for this holon
		staleEvidence, err := t.DB.GetStaleEvidenceByHolon(ctx, holonID)
		if err == nil && len(staleEvidence) > 0 {
			holon, _ := t.DB.GetHolon(ctx, holonID)
			for _, e := range staleEvidence {
				carrierRef := ""
				if e.CarrierRef.Valid {
					carrierRef = e.CarrierRef.String
				}
				var staleSince time.Time
				if e.StaleSince.Valid {
					staleSince = e.StaleSince.Time
				}
				reason := ""
				if e.StaleReason.Valid {
					reason = e.StaleReason.String
				}

				warnings.StaleEvidence = append(warnings.StaleEvidence, StaleEvidenceWarning{
					EvidenceID: e.ID,
					CarrierRef: carrierRef,
					HolonID:    holonID,
					HolonTitle: holon.Title,
					HolonLayer: holon.Layer,
					StaleSince: staleSince,
					Reason:     reason,
				})
			}
		}

		// Check if holon needs reverification
		holon, err := t.DB.GetHolon(ctx, holonID)
		if err == nil {
			if holon.NeedsReverification.Valid && holon.NeedsReverification.Int64 == 1 {
				reason := ""
				if holon.ReverificationReason.Valid {
					reason = holon.ReverificationReason.String
				}
				rEff := 0.0
				if holon.CachedRScore.Valid {
					rEff = holon.CachedRScore.Float64
				}
				warnings.DependencyIssues = append(warnings.DependencyIssues, DependencyIssueWarning{
					HolonID:    holonID,
					HolonTitle: holon.Title,
					Layer:      holon.Layer,
					REff:       rEff,
					Reason:     reason,
				})
			}
		}

		// Check dependencies of this holon
		deps, err := t.DB.GetDependencies(ctx, holonID)
		if err == nil {
			for _, dep := range deps {
				checkHolon(dep.TargetID, depth+1)
			}
		}
	}

	for _, holonID := range holonsToCheck {
		checkHolon(holonID, 0)
	}

	return warnings
}

// formatImplementationWarnings formats warnings for output
func (t *Tools) formatImplementationWarnings(warnings *ImplementationWarnings) string {
	if !warnings.HasAny() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## ⚠️ WARNINGS\n\n")
	sb.WriteString("**Evidence in the dependency chain has become stale due to code changes.**\n\n")
	sb.WriteString("Review these issues before proceeding:\n\n")

	if len(warnings.StaleEvidence) > 0 {
		sb.WriteString("### Stale Evidence\n")
		for _, w := range warnings.StaleEvidence {
			sb.WriteString(fmt.Sprintf("- **%s** [%s]: `%s`\n",
				w.HolonTitle, w.HolonLayer, w.CarrierRef))
			if w.Reason != "" {
				sb.WriteString(fmt.Sprintf("  - Reason: %s\n", w.Reason))
			}
		}
		sb.WriteString("\n")
	}

	if len(warnings.DependencyIssues) > 0 {
		sb.WriteString("### Holons Needing Re-verification\n")
		for _, w := range warnings.DependencyIssues {
			sb.WriteString(fmt.Sprintf("- **%s** [%s] R_eff=%.2f\n",
				w.HolonTitle, w.Layer, w.REff))
			if w.Reason != "" {
				sb.WriteString(fmt.Sprintf("  - %s\n", w.Reason))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### Recommended Actions\n\n")
	sb.WriteString("1. Review the changed carrier files\n")
	sb.WriteString("2. Re-run tests: `quint_test holon-id internal \"re-verification\" PASS`\n")
	sb.WriteString("3. Or acknowledge stale state if changes are compatible\n\n")
	sb.WriteString("---\n")

	return sb.String()
}

// Implement transforms a DRR into an implementation directive that programs the agent.
func (t *Tools) Implement(drrID string) (string, error) {
	defer t.RecordWork("Implement", time.Now())

	if t.DB == nil {
		return "", fmt.Errorf("database not initialized - run quint_internalize first")
	}

	// Run code change detection before implementation (v5.0.0)
	// This ensures any changed carrier files are detected and evidence marked stale
	ctx := context.Background()
	if _, err := t.detectCodeChanges(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: code change detection failed: %v\n", err)
	}

	// Normalize DRR ID: strip "DRR-YYYY-MM-DD-" prefix if present
	normalizedID := drrID
	if strings.HasPrefix(drrID, "DRR-") {
		// Pattern: DRR-2025-12-24-actual-id -> actual-id
		parts := strings.SplitN(drrID, "-", 5) // DRR, YYYY, MM, DD, rest
		if len(parts) == 5 {
			normalizedID = parts[4]
		}
	}

	// Load the DRR
	drr, err := t.loadDRRInfo(normalizedID)
	if err != nil {
		// Try original ID as fallback
		drr, err = t.loadDRRInfo(drrID)
		if err != nil {
			return "", err
		}
	}

	if drr.Contract == nil {
		return "", fmt.Errorf("DRR %s has no implementation contract - nothing to implement", drrID)
	}

	// Collect inherited constraints from dependency chain
	inherited := t.collectInheritedConstraints(drr.DependsOn, make(map[string]bool))

	// Collect warnings about stale evidence (v5.0.0)
	warnings := t.collectImplementationWarnings(ctx, normalizedID, drr.DependsOn)
	warningsText := t.formatImplementationWarnings(warnings)

	// Format the implementation directive
	directive := t.formatImplementDirective(drr, inherited)

	// Prepend warnings if any exist
	if warningsText != "" {
		return warningsText + "\n" + directive, nil
	}
	return directive, nil
}

// loadDRRInfo loads DRR metadata including contract and dependencies
func (t *Tools) loadDRRInfo(drrID string) (*DRRInfo, error) {
	ctx := context.Background()

	// Get holon from DB
	holon, err := t.DB.GetHolon(ctx, drrID)
	if err != nil {
		return nil, fmt.Errorf("DRR not found: %s", drrID)
	}

	if holon.Type != "DRR" && holon.Layer != "DRR" {
		return nil, fmt.Errorf("holon %s is not a DRR (type=%s, layer=%s)", drrID, holon.Type, holon.Layer)
	}

	// Load contract from markdown file
	contract, _ := t.getDRRContract(drrID)

	// Get dependencies via relations (query by source_id to find what this DRR depends on)
	var dependsOn []string
	var winnerID string

	rows, err := t.DB.GetRawDB().QueryContext(ctx,
		`SELECT target_id, relation_type FROM relations WHERE source_id = ?`, drrID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var targetID, relType string
			if err := rows.Scan(&targetID, &relType); err == nil {
				if relType == "selects" {
					winnerID = targetID
					// The winner's dependencies become our dependencies
					dependsOn = append(dependsOn, targetID)
				} else if relType == "componentOf" || relType == "constituentOf" {
					dependsOn = append(dependsOn, targetID)
				}
			}
		}
	}

	return &DRRInfo{
		ID:        drrID,
		Title:     holon.Title,
		Contract:  contract,
		DependsOn: dependsOn,
		WinnerID:  winnerID,
	}, nil
}

// collectInheritedConstraints recursively collects constraints from dependency chain
func (t *Tools) collectInheritedConstraints(depIDs []string, visited map[string]bool) InheritedConstraints {
	var result InheritedConstraints

	for _, depID := range depIDs {
		if visited[depID] {
			continue // Prevent cycles
		}
		visited[depID] = true

		dep, err := t.loadDRRInfo(depID)
		if err != nil || dep.Contract == nil {
			continue
		}

		if len(dep.Contract.Invariants) > 0 {
			result.Invariants = append(result.Invariants, ConstraintSource{
				DRRID:       depID,
				DRRTitle:    dep.Title,
				Constraints: dep.Contract.Invariants,
			})
		}

		if len(dep.Contract.AntiPatterns) > 0 {
			result.AntiPatterns = append(result.AntiPatterns, ConstraintSource{
				DRRID:       depID,
				DRRTitle:    dep.Title,
				Constraints: dep.Contract.AntiPatterns,
			})
		}

		// Recurse into dependencies of dependencies
		deeper := t.collectInheritedConstraints(dep.DependsOn, visited)
		result.Invariants = append(result.Invariants, deeper.Invariants...)
		result.AntiPatterns = append(result.AntiPatterns, deeper.AntiPatterns...)
	}

	return result
}

// formatImplementDirective formats the implementation directive prompt
func (t *Tools) formatImplementDirective(drr *DRRInfo, inherited InheritedConstraints) string {
	var sb strings.Builder

	sb.WriteString("# IMPLEMENTATION DIRECTIVE\n\n")

	// Task section
	sb.WriteString("## Task\n\n")
	sb.WriteString(fmt.Sprintf("Implement: **%s**\n", drr.Title))
	sb.WriteString(fmt.Sprintf("Decision: %s\n", drr.ID))
	if len(drr.Contract.AffectedScope) > 0 {
		sb.WriteString(fmt.Sprintf("Scope: %s\n", strings.Join(drr.Contract.AffectedScope, ", ")))
	}
	sb.WriteString("\n")

	// Instructions
	sb.WriteString("## Instructions\n\n")
	sb.WriteString("Using your internal TODO/planning capabilities, implement this task.\n\n")
	sb.WriteString("If project context is insufficient, conduct preliminary investigation first.\n\n")

	// Invariants section
	if len(drr.Contract.Invariants) > 0 || len(inherited.Invariants) > 0 {
		sb.WriteString("## Invariants to Implement\n\n")
		sb.WriteString("These MUST be true in your implementation:\n\n")

		if len(drr.Contract.Invariants) > 0 {
			sb.WriteString("### This decision:\n")
			for i, inv := range drr.Contract.Invariants {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, inv))
			}
			sb.WriteString("\n")
		}

		for _, src := range inherited.Invariants {
			sb.WriteString(fmt.Sprintf("### Inherited from %s:\n", src.DRRID))
			for _, inv := range src.Constraints {
				sb.WriteString(fmt.Sprintf("- %s\n", inv))
			}
			sb.WriteString("\n")
		}

		if len(inherited.Invariants) > 0 {
			sb.WriteString("⚠️ Inherited constraints come from dependency chain — violating them breaks the foundation.\n\n")
		}
	}

	// Final Verification section (anti-patterns)
	if len(drr.Contract.AntiPatterns) > 0 || len(inherited.AntiPatterns) > 0 {
		sb.WriteString("## Final Verification\n\n")
		sb.WriteString("Your LAST todo items must verify these constraints were NOT violated:\n\n")

		if len(drr.Contract.AntiPatterns) > 0 {
			sb.WriteString("### This decision:\n")
			for _, ap := range drr.Contract.AntiPatterns {
				sb.WriteString(fmt.Sprintf("- [ ] %s\n", ap))
			}
			sb.WriteString("\n")
		}

		for _, src := range inherited.AntiPatterns {
			sb.WriteString(fmt.Sprintf("### Inherited from %s:\n", src.DRRID))
			for _, ap := range src.Constraints {
				sb.WriteString(fmt.Sprintf("- [ ] %s\n", ap))
			}
			sb.WriteString("\n")
		}
	}

	// Acceptance Criteria section
	if len(drr.Contract.AcceptanceCriteria) > 0 {
		sb.WriteString("## Acceptance Criteria\n\n")
		sb.WriteString("Before calling quint_resolve, verify:\n\n")
		for _, ac := range drr.Contract.AcceptanceCriteria {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", ac))
		}
		sb.WriteString("\n")
	}

	// Completion instruction
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("When complete: `quint_resolve %s implemented criteria_verified=true`\n", drr.ID))

	return sb.String()
}

// Resolve records the outcome of a decision: implemented, abandoned, or superseded.
func (t *Tools) Resolve(input ResolveInput) (string, error) {
	defer t.RecordWork("Resolve", time.Now())

	if t.DB == nil {
		return "", fmt.Errorf("database not initialized - run quint_internalize first")
	}

	ctx := context.Background()

	holon, err := t.DB.GetHolon(ctx, input.DecisionID)
	if err != nil {
		return "", fmt.Errorf("decision not found: %s", input.DecisionID)
	}
	if holon.Type != "DRR" && holon.Layer != "DRR" {
		return "", fmt.Errorf("holon %s is not a decision (type=%s, layer=%s)", input.DecisionID, holon.Type, holon.Layer)
	}

	validResolutions := map[string]bool{
		"implemented": true,
		"abandoned":   true,
		"superseded":  true,
	}
	if !validResolutions[input.Resolution] {
		return "", fmt.Errorf("invalid resolution: %s (must be: implemented, abandoned, superseded)", input.Resolution)
	}

	var contract *Contract
	switch input.Resolution {
	case "implemented":
		if input.Reference == "" {
			return "", fmt.Errorf("reference required for 'implemented' resolution (e.g., commit:SHA, pr:NUM)")
		}
		// Check for acceptance criteria
		contract, _ = t.getDRRContract(input.DecisionID)
		if contract != nil && len(contract.AcceptanceCriteria) > 0 && !input.CriteriaVerified {
			var criteriaList strings.Builder
			criteriaList.WriteString("This decision has acceptance criteria that must be verified:\n\n")
			for i, criterion := range contract.AcceptanceCriteria {
				criteriaList.WriteString(fmt.Sprintf("%d. %s\n", i+1, criterion))
			}
			criteriaList.WriteString("\nTo resolve, set criteria_verified=true after confirming these criteria are met.")
			return "", fmt.Errorf("acceptance criteria not verified:\n%s", criteriaList.String())
		}
	case "superseded":
		if input.SupersededBy == "" {
			return "", fmt.Errorf("superseded_by required for 'superseded' resolution")
		}
		superseding, err := t.DB.GetHolon(ctx, input.SupersededBy)
		if err != nil {
			return "", fmt.Errorf("superseding decision not found: %s", input.SupersededBy)
		}
		if superseding.Type != "DRR" && superseding.Layer != "DRR" {
			return "", fmt.Errorf("superseding holon %s is not a decision", input.SupersededBy)
		}
	case "abandoned":
		if input.Notes == "" {
			return "", fmt.Errorf("notes (reason) required for 'abandoned' resolution")
		}
	}

	evidences, _ := t.DB.GetEvidence(ctx, input.DecisionID)
	for _, e := range evidences {
		if e.Type == "implementation" || e.Type == "abandonment" || e.Type == "supersession" {
			return "", fmt.Errorf("decision already resolved (evidence: %s, type: %s)", e.ID, e.Type)
		}
	}

	evidenceID := uuid.New().String()
	var evidenceType, content, carrierRef string

	switch input.Resolution {
	case "implemented":
		evidenceType = "implementation"
		content = input.Notes
		if content == "" {
			content = "Decision implemented"
		}
		carrierRef = input.Reference

	case "abandoned":
		evidenceType = "abandonment"
		content = input.Notes
		carrierRef = ""

	case "superseded":
		evidenceType = "supersession"
		content = input.Notes
		if content == "" {
			content = fmt.Sprintf("Superseded by %s", input.SupersededBy)
		}
		carrierRef = "superseded_by:" + input.SupersededBy

		// Create SupersededBy relation
		if err := t.DB.CreateRelation(ctx, input.DecisionID, "SupersededBy", input.SupersededBy, 3); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create SupersededBy relation: %v\n", err)
		}
	}

	// Get current commit for carrier_commit tracking
	carrierCommit := ""
	if currentHead, headErr := t.getCurrentHead(); headErr == nil {
		carrierCommit = currentHead
	}

	err = t.DB.AddEvidence(ctx,
		evidenceID,
		input.DecisionID,
		evidenceType,
		content,
		"PASS",
		"",
		carrierRef,
		carrierCommit,
		input.ValidUntil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create evidence: %v", err)
	}

	t.AuditLog("quint_resolve", "resolve_decision",
		string(t.FSM.State.ActiveRole.Role),
		input.DecisionID, "SUCCESS", input, "")

	result := fmt.Sprintf("Decision '%s' resolved as: %s", holon.Title, input.Resolution)
	switch input.Resolution {
	case "implemented":
		result += fmt.Sprintf("\nReference: %s", input.Reference)
	case "abandoned":
		result += fmt.Sprintf("\nReason: %s", input.Notes)
	case "superseded":
		result += fmt.Sprintf("\nSuperseded by: %s", input.SupersededBy)
	}

	return result, nil
}

// GetOpenDecisions returns decisions that have not been resolved.
func (t *Tools) GetOpenDecisions(ctx context.Context) ([]DecisionSummary, error) {
	if t.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT h.id, h.title, h.created_at
		FROM holons h
		WHERE (h.type = 'DRR' OR h.layer = 'DRR')
		AND NOT EXISTS (
			SELECT 1 FROM evidence e
			WHERE e.holon_id = h.id
			AND e.type IN ('implementation', 'abandonment', 'supersession')
		)
		ORDER BY h.created_at DESC
	`
	rows, err := t.DB.GetRawDB().QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DecisionSummary
	for rows.Next() {
		var d DecisionSummary
		var createdAt sql.NullTime
		if err := rows.Scan(&d.ID, &d.Title, &createdAt); err != nil {
			continue
		}
		if createdAt.Valid {
			d.CreatedAt = createdAt.Time
		}
		d.Resolution = "open"
		results = append(results, d)
	}
	return results, nil
}

// GetResolvedDecisions returns decisions with a specific resolution status.
func (t *Tools) GetResolvedDecisions(ctx context.Context, resolution string, limit int) ([]DecisionSummary, error) {
	if t.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	evidenceType := map[string]string{
		"implemented": "implementation",
		"abandoned":   "abandonment",
		"superseded":  "supersession",
	}[resolution]

	if evidenceType == "" {
		return nil, fmt.Errorf("invalid resolution filter: %s", resolution)
	}

	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT h.id, h.title, h.created_at, e.type, e.created_at as resolved_at, e.content, e.carrier_ref
		FROM holons h
		JOIN evidence e ON e.holon_id = h.id
		WHERE (h.type = 'DRR' OR h.layer = 'DRR')
		AND e.type = ?
		ORDER BY e.created_at DESC
		LIMIT ?
	`
	rows, err := t.DB.GetRawDB().QueryContext(ctx, query, evidenceType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DecisionSummary
	for rows.Next() {
		var d DecisionSummary
		var createdAt, resolvedAt sql.NullTime
		var evidenceType string
		var carrierRef sql.NullString
		if err := rows.Scan(&d.ID, &d.Title, &createdAt, &evidenceType, &resolvedAt, &d.Notes, &carrierRef); err != nil {
			continue
		}
		if createdAt.Valid {
			d.CreatedAt = createdAt.Time
		}
		if resolvedAt.Valid {
			d.ResolvedAt = resolvedAt.Time
		}
		if carrierRef.Valid {
			d.Reference = carrierRef.String
		}
		d.Resolution = resolution
		results = append(results, d)
	}
	return results, nil
}

// GetRecentResolvedDecisions returns recently resolved decisions of any type.
func (t *Tools) GetRecentResolvedDecisions(ctx context.Context, limit int) ([]DecisionSummary, error) {
	if t.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	if limit <= 0 {
		limit = 5
	}

	query := `
		SELECT h.id, h.title, h.created_at, e.type, e.created_at as resolved_at, e.content, e.carrier_ref
		FROM holons h
		JOIN evidence e ON e.holon_id = h.id
		WHERE (h.type = 'DRR' OR h.layer = 'DRR')
		AND e.type IN ('implementation', 'abandonment', 'supersession')
		ORDER BY e.created_at DESC
		LIMIT ?
	`
	rows, err := t.DB.GetRawDB().QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	evidenceToResolution := map[string]string{
		"implementation": "implemented",
		"abandonment":    "abandoned",
		"supersession":   "superseded",
	}

	var results []DecisionSummary
	for rows.Next() {
		var d DecisionSummary
		var createdAt, resolvedAt sql.NullTime
		var evidenceType string
		var carrierRef sql.NullString
		if err := rows.Scan(&d.ID, &d.Title, &createdAt, &evidenceType, &resolvedAt, &d.Notes, &carrierRef); err != nil {
			continue
		}
		if createdAt.Valid {
			d.CreatedAt = createdAt.Time
		}
		if resolvedAt.Valid {
			d.ResolvedAt = resolvedAt.Time
		}
		if carrierRef.Valid {
			d.Reference = carrierRef.String
		}
		d.Resolution = evidenceToResolution[evidenceType]
		results = append(results, d)
	}
	return results, nil
}

// ResetCycle ends the current FPF session and returns to IDLE.
// This is an operational action, NOT a decision - no DRR is created.
// Records session state in audit_log for traceability.
func (t *Tools) ResetCycle(reason string) (string, error) {
	defer t.RecordWork("ResetCycle", time.Now())

	if reason == "" {
		reason = "user requested reset"
	}

	currentPhase := t.FSM.GetPhase()

	var stateSummary strings.Builder
	stateSummary.WriteString(fmt.Sprintf("Phase at reset: %s\n", currentPhase))
	stateSummary.WriteString(fmt.Sprintf("L0: %d, L1: %d, L2: %d, DRR: %d\n",
		t.countHolons("L0"), t.countHolons("L1"), t.countHolons("L2"), t.countDRRs()))

	if t.DB != nil {
		ctx := context.Background()
		openDecisions, err := t.GetOpenDecisions(ctx)
		if err == nil && len(openDecisions) > 0 {
			stateSummary.WriteString(fmt.Sprintf("Open decisions: %d\n", len(openDecisions)))
			for _, d := range openDecisions {
				stateSummary.WriteString(fmt.Sprintf("  - %s\n", d.ID))
			}
		}
	}

	t.AuditLog("quint_reset", "cycle_reset", "agent", "", "SUCCESS",
		map[string]string{"reason": reason, "from_phase": string(currentPhase)},
		stateSummary.String())

	t.FSM.State.Phase = PhaseIdle
	if err := t.FSM.SaveState("default"); err != nil {
		return "", fmt.Errorf("failed to save state: %w", err)
	}

	return fmt.Sprintf("Cycle reset to IDLE.\nPrevious phase: %s\nReason: %s\n\n%s",
		currentPhase, reason, stateSummary.String()), nil
}
