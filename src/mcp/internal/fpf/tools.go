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
	"github.com/m0n0x41d/quint-code/logger"

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
	Invariants         []string          `json:"invariants,omitempty"`
	AntiPatterns       []string          `json:"anti_patterns,omitempty"`
	AcceptanceCriteria []string          `json:"acceptance_criteria,omitempty"`
	AffectedScope      []string          `json:"affected_scope,omitempty"`
	AffectedHashes     map[string]string `json:"affected_hashes,omitempty"` // file -> hash at decision time
}

// CheckResult represents a single verification check with explicit verdict and evidence
type CheckResult struct {
	Verdict   string   `json:"verdict"`   // PASS or FAIL
	Evidence  []string `json:"evidence"`  // Concrete refs: "file.go:42", "test output: X"
	Reasoning string   `json:"reasoning"` // Why this verdict
}

// VerifyResult is structured input for quint_verify (L0 -> L1)
type VerifyResult struct {
	TypeCheck       CheckResult `json:"type_check"`
	ConstraintCheck CheckResult `json:"constraint_check"`
	LogicCheck      CheckResult `json:"logic_check"`
	OverallVerdict  string      `json:"overall_verdict"` // PASS or FAIL
	Risks           []string    `json:"risks,omitempty"` // Identified risks even if PASS
}

// Observation represents a single observation during validation
type Observation struct {
	Description string   `json:"description"` // What was observed
	Evidence    []string `json:"evidence"`    // Concrete refs
	Supports    bool     `json:"supports"`    // Does this support hypothesis?
}

// TestResult is structured input for quint_test (L1 -> L2)
type TestResult struct {
	Observations   []Observation `json:"observations"`
	OverallVerdict string        `json:"overall_verdict"` // PASS or FAIL
	Reasoning      string        `json:"reasoning"`       // Why this verdict
}

func NewTools(fsm *FSM, rootDir string, database *db.Store) *Tools {
	if database == nil {
		dbPath := filepath.Join(rootDir, ".quint", "quint.db")
		var err error
		database, err = db.NewStore(dbPath)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to open database in NewTools")
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
		logger.Warn().Err(err).Msg("failed to insert audit log")
	}
}

func (t *Tools) Slugify(title string) string {
	slug := slugifyRegex.ReplaceAllString(strings.ToLower(title), "-")
	return strings.Trim(slug, "-")
}

func (t *Tools) MoveHypothesis(hypothesisID, sourceLevel, destLevel string) error {
	ctx := context.Background()

	if t.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	holon, err := t.DB.GetHolon(ctx, hypothesisID)
	if err != nil {
		t.AuditLog("quint_move", "move_hypothesis", "agent", hypothesisID, "ERROR",
			map[string]string{"from": sourceLevel, "to": destLevel}, "not found in database")
		return fmt.Errorf("hypothesis %s not found", hypothesisID)
	}

	if holon.Layer != sourceLevel {
		return fmt.Errorf("hypothesis %s is in %s, not %s", hypothesisID, holon.Layer, sourceLevel)
	}

	if err := t.DB.UpdateHolonLayer(ctx, hypothesisID, destLevel); err != nil {
		t.AuditLog("quint_move", "move_hypothesis", "agent", hypothesisID, "ERROR",
			map[string]string{"from": sourceLevel, "to": destLevel}, err.Error())
		return fmt.Errorf("failed to update layer in database: %w", err)
	}

	t.AuditLog("quint_move", "move_hypothesis", "agent", hypothesisID, "SUCCESS",
		map[string]string{"from": sourceLevel, "to": destLevel}, "")
	return nil
}

func (t *Tools) InitProject() error {
	dirs := []string{
		"evidence",
		"decisions",
		"sessions",
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
		logger.Warn().Err(err).Msg("failed to record work in DB")
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

	logger.Info().
		Str("title", title).
		Str("kind", kind).
		Str("scope", scope).
		Str("decision_context", decisionContext).
		Int("dependency_count", len(dependsOn)).
		Msg("ProposeHypothesis called")

	if t.DB == nil {
		logger.Error().Msg("ProposeHypothesis: database not initialized")
		return "", fmt.Errorf("database not initialized")
	}

	ctx := context.Background()

	// decision_context is REQUIRED - must be created explicitly via quint_context first
	if decisionContext == "" {
		return "", fmt.Errorf("decision_context is required. Create one first with quint_context(title=\"Your Decision Title\")")
	}

	// Validate provided context exists AND is correct type
	holon, err := t.DB.GetHolon(ctx, decisionContext)
	if err != nil {
		return "", fmt.Errorf("decision_context %q not found. Create it first with quint_context", decisionContext)
	}
	if holon.Type != "decision_context" {
		return "", fmt.Errorf("%q is type %q, not decision_context. Use quint_context to create a proper context, then use the dc-* ID it returns", decisionContext, holon.Type)
	}

	slug := t.Slugify(title)
	body := fmt.Sprintf("# Hypothesis: %s\n\n%s\n\n## Rationale\n%s", title, content, rationale)

	if err := t.DB.CreateHolon(ctx, slug, "hypothesis", kind, "L0", title, body, "default", scope, ""); err != nil {
		logger.Error().Err(err).Str("slug", slug).Msg("ProposeHypothesis: failed to create holon")
		t.AuditLog("quint_propose", "create_hypothesis", "agent", slug, "ERROR", map[string]string{"title": title, "kind": kind}, err.Error())
		return "", fmt.Errorf("failed to create hypothesis in database: %w", err)
	}

	logger.Debug().Str("slug", slug).Str("layer", "L0").Msg("ProposeHypothesis: holon created")

	// Link hypothesis to decision context
	if err := t.createRelation(ctx, slug, "memberOf", decisionContext, 3); err != nil {
		logger.Warn().Err(err).Msg("failed to create MemberOf relation")
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
				logger.Warn().Str("dependency", depID).Msg("dependency not found, skipping")
				continue
			}

			if cyclic, _ := t.wouldCreateCycle(ctx, depID, slug); cyclic {
				logger.Warn().Str("dependency", depID).Msg("dependency would create cycle, skipping")
				continue
			}

			if err := t.createRelation(ctx, depID, relationType, slug, dependencyCL); err != nil {
				logger.Warn().Err(err).Str("relation_type", relationType).Str("target", depID).Msg("failed to create relation")
			}
		}
	}

	t.AuditLog("quint_propose", "create_hypothesis", "agent", slug, "SUCCESS", map[string]string{"title": title, "kind": kind, "scope": scope}, "")

	logger.Info().Str("slug", slug).Str("context", decisionContext).Msg("ProposeHypothesis: completed successfully")

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

	return slug + warningBlock, nil
}

var validRelationTypes = map[string]bool{
	"componentOf":   true,
	"constituentOf": true,
	"memberOf":      true,
	"selects":       true,
	"rejects":       true,
	"closes":        true,
	"verifiedBy":    true,
	"dependsOn":     true,
}

func (t *Tools) createRelation(ctx context.Context, sourceID, relationType, targetID string, cl int) error {
	if sourceID == targetID {
		return fmt.Errorf("holon cannot relate to itself")
	}

	if !validRelationTypes[relationType] {
		return fmt.Errorf("invalid relation type: %s", relationType)
	}

	if err := t.DB.CreateRelation(ctx, sourceID, relationType, targetID, cl); err != nil {
		return err
	}

	t.AuditLog("quint_propose", "create_relation", "agent", sourceID, "SUCCESS",
		map[string]string{"relation": relationType, "target": targetID, "cl": fmt.Sprintf("%d", cl)}, "")

	return nil
}

// CreateContext explicitly creates a decision context for grouping hypotheses.
// This is the only way to create a context (no auto-creation in propose).
func (t *Tools) CreateContext(title, scope, description string) (string, error) {
	defer t.RecordWork("CreateContext", time.Now())

	logger.Info().
		Str("title", title).
		Str("scope", scope).
		Msg("CreateContext called")

	if t.DB == nil {
		logger.Error().Msg("CreateContext: database not initialized")
		return "", fmt.Errorf("database not initialized")
	}
	if title == "" {
		return "", fmt.Errorf("title is required")
	}

	ctx := context.Background()
	contextID := "dc-" + t.Slugify(title)

	// Check if context already exists
	if _, err := t.DB.GetHolon(ctx, contextID); err == nil {
		return "", fmt.Errorf("decision context %q already exists. Use this ID in quint_propose or choose a different title", contextID)
	}

	// Check active context count (max 3)
	activeContexts, err := t.GetActiveDecisionContexts(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get active contexts: %w", err)
	}
	if len(activeContexts) >= 3 {
		var contextList strings.Builder
		for _, c := range activeContexts {
			contextList.WriteString(fmt.Sprintf("\n  - %s: %s", c.ID, c.Title))
		}
		return "", fmt.Errorf("BLOCKED: maximum 3 active decision contexts allowed (have %d).\n\nActive contexts:%s\n\n⚠️ USER ACTION REQUIRED: Ask user whether to:\n  1. Use an existing context (pass one of the dc-* IDs above to quint_propose)\n  2. Complete a context via /q5-decide\n  3. Abandon a context via /q-reset with context_id parameter", len(activeContexts), contextList.String())
	}

	// Build content
	content := fmt.Sprintf("# Decision Context: %s\n\nScope: %s\n", title, scope)
	if description != "" {
		content += fmt.Sprintf("\n## Problem Statement\n\n%s\n", description)
	}
	content += "\nHypotheses will be grouped under this context for decision-making."

	if err := t.DB.CreateHolon(ctx, contextID, "decision_context", "system", "L0", title, content, "default", scope, ""); err != nil {
		return "", fmt.Errorf("failed to create decision context: %w", err)
	}

	t.AuditLog("quint_context", "create_context", "agent", contextID, "SUCCESS",
		map[string]string{"title": title, "scope": scope}, "")

	logger.Info().Str("context_id", contextID).Msg("CreateContext: completed")

	return fmt.Sprintf("%s\n\n→ Use decision_context=\"%s\" in quint_propose to add hypotheses to this context.", contextID, contextID), nil
}

// GetActiveDecisionContexts returns all unclosed decision contexts with their stages.
func (t *Tools) GetActiveDecisionContexts(ctx context.Context) ([]DecisionContextSummary, error) {
	if t.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := t.DB.GetRawDB().QueryContext(ctx, `
		SELECT h.id, h.title, COALESCE(h.scope, '') as scope
		FROM holons h
		WHERE h.type = 'decision_context'
		AND (h.context_status IS NULL OR h.context_status = 'open')
		AND h.id NOT IN (
		    SELECT target_id FROM relations WHERE relation_type = 'closes'
		)
		ORDER BY h.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contexts []DecisionContextSummary
	for rows.Next() {
		var dc DecisionContextSummary
		if err := rows.Scan(&dc.ID, &dc.Title, &dc.Scope); err != nil {
			continue
		}

		// Get stage for this context
		dc.Stage = t.FSM.GetContextStage(dc.ID)

		// Count hypotheses in this context (ignore error - 0 is acceptable default)
		_ = t.DB.GetRawDB().QueryRowContext(ctx, `
			SELECT COUNT(*) FROM relations r
			JOIN holons h ON h.id = r.source_id
			WHERE r.target_id = ? AND r.relation_type = 'memberOf'
			AND h.type = 'hypothesis'
		`, dc.ID).Scan(&dc.HypothesisCount)

		contexts = append(contexts, dc)
	}

	return contexts, nil
}

// getDecisionContext returns the decision context ID for a hypothesis (via memberOf relation).
// Returns empty string if no decision context is found.
func (t *Tools) getDecisionContext(ctx context.Context, holonID string) string {
	if t.DB == nil {
		return ""
	}
	var targetID string
	err := t.DB.GetRawDB().QueryRowContext(ctx,
		`SELECT target_id FROM relations WHERE source_id = ? AND relation_type = 'memberOf' LIMIT 1`,
		holonID).Scan(&targetID)
	if err != nil {
		return ""
	}
	return targetID
}

func (t *Tools) isDecisionContextClosed(ctx context.Context, dcID string) string {
	if t.DB == nil || dcID == "" {
		return ""
	}
	var drrID string
	err := t.DB.GetRawDB().QueryRowContext(ctx,
		`SELECT source_id FROM relations WHERE target_id = ? AND relation_type = 'closes' LIMIT 1`,
		dcID).Scan(&drrID)
	if err != nil {
		return ""
	}
	return drrID
}

func (t *Tools) isHypothesisInOpenDRR(ctx context.Context, hypID string) string {
	if t.DB == nil || hypID == "" {
		return ""
	}
	var drrID string
	err := t.DB.GetRawDB().QueryRowContext(ctx,
		`SELECT r.source_id FROM relations r
		 JOIN holons h ON h.id = r.source_id
		 WHERE r.target_id = ?
		   AND r.relation_type IN ('selects', 'rejects')
		   AND h.type = 'DRR'
		   AND NOT EXISTS (
		       SELECT 1 FROM evidence e
		       WHERE e.holon_id = h.id
		       AND e.type IN ('implementation', 'abandonment', 'supersession')
		   )
		 LIMIT 1`,
		hypID).Scan(&drrID)
	if err != nil {
		return ""
	}
	return drrID
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

	logger.Info().
		Str("source_id", sourceID).
		Str("target_id", targetID).
		Int("congruence_level", cl).
		Msg("LinkHolons called")

	if t.DB == nil {
		logger.Error().Msg("LinkHolons: database not initialized")
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

	logger.Info().
		Str("hypothesis_id", hypothesisID).
		Str("verdict", verdict).
		Str("carrier_files", carrierFiles).
		Msg("VerifyHypothesis called")

	var result VerifyResult
	if err := json.Unmarshal([]byte(checksJSON), &result); err != nil {
		logger.Error().Err(err).Str("hypothesis_id", hypothesisID).Msg("VerifyHypothesis: invalid checks_json")
		return "", fmt.Errorf("invalid checks_json: %w", err)
	}
	result.OverallVerdict = verdict

	if err := t.validateVerifyResult(result); err != nil {
		return "", fmt.Errorf("incomplete justification: %w", err)
	}

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

	if warning := t.checkDuplicateHypothesis(hypothesisID); warning != "" {
		result.Risks = append(result.Risks, warning)
	}

	evidenceJSON, _ := json.MarshalIndent(result, "", "  ")

	switch strings.ToUpper(result.OverallVerdict) {
	case "PASS":
		logger.Debug().Str("hypothesis_id", hypothesisID).Msg("VerifyHypothesis: moving L0 -> L1")
		err := t.MoveHypothesis(hypothesisID, "L0", "L1")
		if err != nil {
			logger.Error().Err(err).Str("hypothesis_id", hypothesisID).Msg("VerifyHypothesis: failed to move hypothesis")
			t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "ERROR", map[string]string{"verdict": "PASS"}, err.Error())
			return "", err
		}

		if _, err := t.ManageEvidence("verification", "add", hypothesisID, "verification", string(evidenceJSON), "pass", "L1", carrierRef, ""); err != nil {
			logger.Warn().Err(err).Str("hypothesis_id", hypothesisID).Msg("failed to record verification evidence")
		}

		logger.Info().Str("hypothesis_id", hypothesisID).Str("result", "L1").Msg("VerifyHypothesis: PASS - promoted to L1")
		t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "PASS", "result": "L1"}, "")

		var output strings.Builder
		output.WriteString(fmt.Sprintf("✅ Hypothesis %s promoted to L1\n\n", hypothesisID))
		if len(result.Risks) > 0 {
			output.WriteString("⚠️ Risks identified:\n")
			for _, r := range result.Risks {
				output.WriteString(fmt.Sprintf("  - %s\n", r))
			}
		}
		return output.String(), nil

	case "FAIL":
		logger.Debug().Str("hypothesis_id", hypothesisID).Msg("VerifyHypothesis: moving L0 -> invalid")
		err := t.MoveHypothesis(hypothesisID, "L0", "invalid")
		if err != nil {
			logger.Error().Err(err).Str("hypothesis_id", hypothesisID).Msg("VerifyHypothesis: failed to move hypothesis")
			t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "ERROR", map[string]string{"verdict": "FAIL"}, err.Error())
			return "", err
		}

		if _, err := t.ManageEvidence("verification", "add", hypothesisID, "verification", string(evidenceJSON), "fail", "invalid", carrierRef, ""); err != nil {
			logger.Warn().Err(err).Str("hypothesis_id", hypothesisID).Msg("failed to record verification evidence")
		}

		logger.Info().Str("hypothesis_id", hypothesisID).Str("result", "invalid").Msg("VerifyHypothesis: FAIL - moved to invalid")
		t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "FAIL", "result": "invalid"}, "")

		var output strings.Builder
		output.WriteString(fmt.Sprintf("⚠️ VERIFICATION FAILED: %s moved to invalid\n\n", hypothesisID))
		output.WriteString("Options:\n")
		output.WriteString("  - /q1-hypothesize — create refined hypothesis\n")
		output.WriteString("  - Address the issues and propose a new hypothesis\n\n")
		output.WriteString("Failure reasons recorded for audit.\n")
		return output.String(), nil

	default:
		return "", fmt.Errorf("overall_verdict must be PASS or FAIL, got: %s", result.OverallVerdict)
	}
}

func (t *Tools) validateVerifyResult(r VerifyResult) error {
	checks := []struct {
		name  string
		check CheckResult
	}{
		{"type_check", r.TypeCheck},
		{"constraint_check", r.ConstraintCheck},
		{"logic_check", r.LogicCheck},
	}

	for _, c := range checks {
		if c.check.Verdict == "" {
			return fmt.Errorf("%s: missing verdict", c.name)
		}
		verdict := strings.ToUpper(c.check.Verdict)
		if verdict != "PASS" && verdict != "FAIL" {
			return fmt.Errorf("%s: verdict must be PASS or FAIL, got: %s", c.name, c.check.Verdict)
		}
		if len(c.check.Evidence) == 0 {
			return fmt.Errorf("%s: verdict requires at least one evidence reference", c.name)
		}
		if c.check.Reasoning == "" {
			return fmt.Errorf("%s: missing reasoning", c.name)
		}
	}

	if r.OverallVerdict == "" {
		return fmt.Errorf("missing overall_verdict")
	}
	verdict := strings.ToUpper(r.OverallVerdict)
	if verdict != "PASS" && verdict != "FAIL" {
		return fmt.Errorf("overall_verdict must be PASS or FAIL, got: %s", r.OverallVerdict)
	}

	return nil
}

func (t *Tools) checkDuplicateHypothesis(hypothesisID string) string {
	if t.DB == nil {
		return ""
	}

	ctx := context.Background()

	current, err := t.DB.GetHolon(ctx, hypothesisID)
	if err != nil || current.Title == "" {
		return ""
	}

	rows, err := t.DB.GetRawDB().QueryContext(ctx, `
		SELECT id FROM holons
		WHERE layer = 'invalid'
		AND title = ?
		AND id != ?
	`, current.Title, hypothesisID)
	if err != nil {
		return ""
	}
	defer rows.Close() //nolint:errcheck

	var matches []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			matches = append(matches, id)
		}
	}
	if err := rows.Err(); err != nil {
		logger.Warn().Err(err).Msg("error iterating duplicate hypothesis rows")
		return ""
	}

	if len(matches) > 0 {
		return fmt.Sprintf("Similar hypothesis previously failed: %v. Ensure this version addresses the failure reasons.", matches)
	}
	return ""
}

func (t *Tools) ValidateHypothesis(hypothesisID, testType, result, verdict, carrierFiles string) (string, error) {
	defer t.RecordWork("ValidateHypothesis", time.Now())

	logger.Info().
		Str("hypothesis_id", hypothesisID).
		Str("test_type", testType).
		Str("verdict", verdict).
		Str("carrier_files", carrierFiles).
		Msg("ValidateHypothesis called")

	if result == "" {
		logger.Error().Str("hypothesis_id", hypothesisID).Msg("ValidateHypothesis: result is required")
		return "", fmt.Errorf("result is required")
	}

	carrierRef := carrierFiles
	if carrierRef == "" {
		carrierRef = "test-runner"
	}

	evidenceData := map[string]interface{}{
		"test_type":       testType,
		"result":          result,
		"overall_verdict": verdict,
	}
	evidenceJSON, _ := json.MarshalIndent(evidenceData, "", "  ")
	validUntil := computeValidUntil(testType)

	switch strings.ToUpper(verdict) {
	case "PASS":
		logger.Debug().Str("hypothesis_id", hypothesisID).Str("test_type", testType).Msg("ValidateHypothesis: adding validation evidence")
		if _, err := t.ManageEvidence("validation", "add", hypothesisID, testType, string(evidenceJSON), "pass", "L2", carrierRef, validUntil); err != nil {
			logger.Error().Err(err).Str("hypothesis_id", hypothesisID).Msg("ValidateHypothesis: failed to add evidence")
			t.AuditLog("quint_test", "validate_hypothesis", "agent", hypothesisID, "ERROR", map[string]string{"verdict": "PASS"}, err.Error())
			return "", err
		}

		logger.Info().Str("hypothesis_id", hypothesisID).Str("result", "L2").Msg("ValidateHypothesis: PASS - promoted to L2")
		t.AuditLog("quint_test", "validate_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "PASS", "result": "L2"}, "")

		var output strings.Builder
		output.WriteString(fmt.Sprintf("✅ Hypothesis %s validated (L2)\n\n", hypothesisID))
		output.WriteString(fmt.Sprintf("Test type: %s\n", testType))
		return output.String(), nil

	case "FAIL":
		logger.Debug().Str("hypothesis_id", hypothesisID).Str("test_type", testType).Msg("ValidateHypothesis: recording failed validation")
		if _, err := t.ManageEvidence("validation", "add", hypothesisID, testType, string(evidenceJSON), "fail", "L1", carrierRef, validUntil); err != nil {
			logger.Error().Err(err).Str("hypothesis_id", hypothesisID).Msg("ValidateHypothesis: failed to add evidence")
			t.AuditLog("quint_test", "validate_hypothesis", "agent", hypothesisID, "ERROR", map[string]string{"verdict": "FAIL"}, err.Error())
			return "", err
		}

		logger.Info().Str("hypothesis_id", hypothesisID).Str("result", "L1").Msg("ValidateHypothesis: FAIL - remains at L1")
		t.AuditLog("quint_test", "validate_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "FAIL", "result": "L1"}, "")

		var output strings.Builder
		output.WriteString(fmt.Sprintf("⚠️ VALIDATION FAILED: %s remains at L1\n\n", hypothesisID))
		output.WriteString("Options:\n")
		output.WriteString("  - /q1-hypothesize — create refined hypothesis\n")
		output.WriteString("  - Address the issues and re-run /q3-validate\n\n")
		output.WriteString("Failure reasons recorded for audit.\n")
		return output.String(), nil

	default:
		return "", fmt.Errorf("overall_verdict must be PASS or FAIL, got: %s", verdict)
	}
}

func computeValidUntil(testType string) string {
	var days int
	switch testType {
	case "internal":
		days = 90
	case "research", "external":
		days = 60
	default:
		days = 90
	}
	return time.Now().AddDate(0, 0, days).Format("2006-01-02")
}

func (t *Tools) validateTestResult(r TestResult) error {
	if len(r.Observations) == 0 {
		return fmt.Errorf("at least one observation is required")
	}

	for i, obs := range r.Observations {
		if obs.Description == "" {
			return fmt.Errorf("observation[%d]: missing description", i)
		}
		if len(obs.Evidence) == 0 {
			return fmt.Errorf("observation[%d]: requires at least one evidence reference", i)
		}
	}

	if r.OverallVerdict == "" {
		return fmt.Errorf("missing overall_verdict")
	}
	verdict := strings.ToUpper(r.OverallVerdict)
	if verdict != "PASS" && verdict != "FAIL" {
		return fmt.Errorf("overall_verdict must be PASS or FAIL, got: %s", r.OverallVerdict)
	}

	if r.Reasoning == "" {
		return fmt.Errorf("missing reasoning")
	}

	return nil
}

func (t *Tools) AuditEvidence(hypothesisID, risks string) (string, error) {
	defer t.RecordWork("AuditEvidence", time.Now())

	logger.Info().Str("hypothesis_id", hypothesisID).Msg("AuditEvidence called")

	_, err := t.ManageEvidence("audit", "add", hypothesisID, "audit_report", risks, "pass", "L2", "auditor", "")
	if err != nil {
		logger.Error().Err(err).Str("hypothesis_id", hypothesisID).Msg("AuditEvidence: failed to add evidence")
		return "", err
	}

	logger.Info().Str("hypothesis_id", hypothesisID).Msg("AuditEvidence: completed")
	return "Audit recorded for " + hypothesisID, nil
}

// UnifiedAudit combines audit tree visualization, R_eff calculation, and optional risk recording.
func (t *Tools) UnifiedAudit(holonID, risks string) (string, error) {
	defer t.RecordWork("UnifiedAudit", time.Now())

	logger.Info().
		Str("holon_id", holonID).
		Bool("has_risks", risks != "").
		Msg("UnifiedAudit called")

	if t.DB == nil {
		logger.Error().Msg("UnifiedAudit: database not initialized")
		return "", fmt.Errorf("database not initialized")
	}

	if holonID == "" {
		return "", fmt.Errorf("holon_id is required")
	}

	var result strings.Builder

	// 1. Calculate R_eff with detailed breakdown
	calc := assurance.New(t.DB.GetRawDB())
	report, err := calc.CalculateReliability(context.Background(), holonID)
	if err != nil {
		return "", fmt.Errorf("failed to calculate reliability: %w", err)
	}

	result.WriteString(fmt.Sprintf("# Audit Report: %s\n\n", holonID))
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

	// 2. Build audit tree
	result.WriteString("\n## Assurance Tree\n\n```\n")
	tree, err := t.buildAuditTree(holonID, 0, calc)
	if err != nil {
		result.WriteString(fmt.Sprintf("(Unable to build tree: %v)\n", err))
	} else {
		result.WriteString(tree)
	}
	result.WriteString("```\n")

	// 3. If risks provided, record audit evidence
	if risks != "" {
		_, err := t.ManageEvidence("audit", "add", holonID, "audit_report", risks, "pass", "L2", "auditor", "")
		if err != nil {
			result.WriteString(fmt.Sprintf("\n⚠️ Failed to record audit: %v\n", err))
		} else {
			result.WriteString("\n✓ Audit evidence recorded\n")
		}
	}

	return result.String(), nil
}

func (t *Tools) ManageEvidence(operation, action, targetID, evidenceType, content, verdict, assuranceLevel, carrierRef, validUntil string) (string, error) {
	// operation: "verification" (L0→L1) or "validation" (L1→L2)
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
		switch operation {
		case "verification":
			if assuranceLevel == "L1" || assuranceLevel == "L2" {
				shouldPromote = true
			}
		case "validation":
			if assuranceLevel == "L2" {
				shouldPromote = true
			}
		}
	}

	var moveErr error
	if (normalizedVerdict == "pass") && shouldPromote {
		switch operation {
		case "verification":
			moveErr = t.MoveHypothesis(targetID, "L0", "L1")
		case "validation":
			holon, err := t.DB.GetHolon(context.Background(), targetID)
			if err == nil && holon.Layer == "L0" {
				return "", fmt.Errorf("hypothesis %s is still in L0: run /q2-verify to promote it to L1 before testing", targetID)
			}
			if err == nil && holon.Layer == "L1" {
				moveErr = t.MoveHypothesis(targetID, "L1", "L2")
			}
		}
	} else if normalizedVerdict == "fail" || normalizedVerdict == "refine" {
		switch operation {
		case "verification":
			moveErr = t.MoveHypothesis(targetID, "L0", "invalid")
		case "validation":
			holon, err := t.DB.GetHolon(context.Background(), targetID)
			if err == nil && holon.Layer == "L1" {
				moveErr = t.MoveHypothesis(targetID, "L1", "invalid")
			}
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
		// Compute carrier hash for staleness detection (v5.1.0)
		carrierHash := t.hashCarrierFiles(carrierRef)

		// Get current commit for metadata (optional, kept for audit trail)
		carrierCommit := ""
		if currentHead, err := t.getCurrentHead(); err == nil {
			carrierCommit = currentHead
		}

		if err := t.DB.AddEvidence(ctx, filename, targetID, evidenceType, content, normalizedVerdict, assuranceLevel, carrierRef, carrierHash, carrierCommit, validUntil); err != nil {
			logger.Warn().Err(err).Msg("failed to add evidence to DB")
		}
		if err := t.DB.Link(ctx, filename, targetID, "verifiedBy"); err != nil {
			logger.Warn().Err(err).Msg("failed to link evidence in DB")
		}

		// Clear stale flags on successful re-validation (v5.0.0)
		if normalizedVerdict == "pass" {
			if err := t.DB.ClearAllEvidenceStaleForHolon(ctx, targetID); err != nil {
				logger.Warn().Err(err).Msg("failed to clear stale evidence")
			}
			if err := t.DB.ClearHolonReverification(ctx, targetID); err != nil {
				logger.Warn().Err(err).Msg("failed to clear reverification flag")
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

func (t *Tools) RefineLoopback(sourceLayer, parentID, insight, newTitle, newContent, scope string) (string, error) {
	// sourceLayer: "L1" for induction loopback, "L0" for deduction loopback
	defer t.RecordWork("RefineLoopback", time.Now())

	parentLevel := sourceLayer
	if parentLevel != "L0" && parentLevel != "L1" {
		return "", fmt.Errorf("loopback not applicable from layer %s (must be L0 or L1)", sourceLayer)
	}

	// Get parent's decision context for the child
	var decisionContext string
	if t.DB != nil {
		ctx := context.Background()
		rawDB := t.DB.GetRawDB()
		err := rawDB.QueryRowContext(ctx, `
			SELECT target_id FROM relations
			WHERE source_id = ? AND relation_type = 'memberOf'
			LIMIT 1
		`, parentID).Scan(&decisionContext)
		if err != nil {
			return "", fmt.Errorf("failed to get parent's decision context: parent %s has no decision context", parentID)
		}
	}

	if err := t.MoveHypothesis(parentID, parentLevel, "invalid"); err != nil {
		return "", fmt.Errorf("failed to move parent hypothesis to invalid: %v", err)
	}

	rationale := fmt.Sprintf(`{"source": "loopback", "parent_id": "%s", "insight": "%s"}`, parentID, insight)
	childPath, err := t.ProposeHypothesis(newTitle, newContent, scope, "system", rationale, decisionContext, nil, 3)
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

	logger.Info().
		Str("title", title).
		Str("winner_id", winnerID).
		Int("rejected_count", len(rejectedIDs)).
		Bool("has_contract", contractJSON != "").
		Msg("FinalizeDecision called")

	var contract Contract
	if contractJSON != "" {
		if err := json.Unmarshal([]byte(contractJSON), &contract); err != nil {
			return "", fmt.Errorf("invalid contract JSON: %w", err)
		}
		// Compute hashes for affected_scope files at decision time
		// affected_scope can contain file:class or file:method references, extract just the file path
		if len(contract.AffectedScope) > 0 {
			contract.AffectedHashes = make(map[string]string)
			for _, scopeRef := range contract.AffectedScope {
				scopeRef = strings.TrimSpace(scopeRef)
				if scopeRef == "" {
					continue
				}
				// Extract file path from "file:class" or "file:method" format
				filePath := scopeRef
				if colonIdx := strings.Index(scopeRef, ":"); colonIdx > 0 {
					filePath = scopeRef[:colonIdx]
				}
				fullPath := filepath.Join(t.RootDir, filePath)
				content, err := os.ReadFile(fullPath)
				if err != nil {
					contract.AffectedHashes[filePath] = "_missing_"
					continue
				}
				hash := sha256.Sum256(content)
				contract.AffectedHashes[filePath] = hex.EncodeToString(hash[:8])
			}
		}
	}

	// Preconditions
	if t.DB != nil {
		ctx := context.Background()
		for _, hypID := range append([]string{winnerID}, rejectedIDs...) {
			if hypID == "" {
				continue
			}
			if dcID := t.getDecisionContext(ctx, hypID); dcID != "" {
				if conflictingDRR := t.isDecisionContextClosed(ctx, dcID); conflictingDRR != "" {
					return "", fmt.Errorf("BLOCKED: decision_context '%s' already closed by DRR '%s'", dcID, conflictingDRR)
				}
			}
			if conflictingDRR := t.isHypothesisInOpenDRR(ctx, hypID); conflictingDRR != "" {
				return "", fmt.Errorf("BLOCKED: hypothesis '%s' already used in open DRR '%s'", hypID, conflictingDRR)
			}
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
		drrID := fmt.Sprintf("DRR-%s-%s", dateStr, t.Slugify(title))
		if err := t.DB.CreateHolon(ctx, drrID, "DRR", "", "DRR", title, body, "default", scopeForDB, winnerID); err != nil {
			logger.Warn().Err(err).Msg("failed to create DRR holon in DB")
		}

		if winnerID != "" {
			if err := t.createRelation(ctx, drrID, "selects", winnerID, 3); err != nil {
				logger.Warn().Err(err).Msg("failed to create selects relation")
			}
		}

		for _, rejID := range rejectedIDs {
			if rejID != "" && rejID != winnerID {
				if err := t.createRelation(ctx, drrID, "rejects", rejID, 3); err != nil {
					logger.Warn().Err(err).Str("rejected_id", rejID).Msg("failed to create rejects relation")
				}
			}
		}

		// Close decision contexts that alternatives were members of
		closedContexts := make(map[string]bool)
		allHypotheses := append([]string{winnerID}, rejectedIDs...)
		for _, hypID := range allHypotheses {
			if hypID == "" {
				continue
			}
			if dcID := t.getDecisionContext(ctx, hypID); dcID != "" && !closedContexts[dcID] {
				if err := t.createRelation(ctx, drrID, "closes", dcID, 3); err != nil {
					logger.Warn().Err(err).Str("decision_context", dcID).Msg("failed to create closes relation")
				}
				// Update context_status to 'closed'
				if err := t.DB.CloseContext(ctx, dcID); err != nil {
					logger.Warn().Err(err).Str("decision_context", dcID).Msg("failed to close context status")
				}
				closedContexts[dcID] = true
			}
		}

		// Close all orphaned hypotheses (L0/L1/L2) in closed decision contexts
		for dcID := range closedContexts {
			rows, err := t.DB.GetRawDB().QueryContext(ctx, `
				SELECT r.source_id FROM relations r
				JOIN holons h ON h.id = r.source_id
				WHERE r.target_id = ?
				  AND r.relation_type = 'memberOf'
				  AND h.layer IN ('L0', 'L1', 'L2')
			`, dcID)
			if err != nil {
				logger.Warn().Err(err).Str("decision_context", dcID).Msg("failed to query orphaned hypotheses")
				continue
			}
			for rows.Next() {
				var orphanID string
				if err := rows.Scan(&orphanID); err != nil {
					continue
				}
				if err := t.createRelation(ctx, drrID, "closes", orphanID, 3); err != nil {
					logger.Warn().Err(err).Str("orphan_id", orphanID).Msg("failed to close orphaned hypothesis")
				} else {
					logger.Info().Str("orphan_id", orphanID).Msg("closed orphaned hypothesis")
				}
			}
			rows.Close()
		}
	}

	if winnerID != "" {
		err := t.MoveHypothesis(winnerID, "L1", "L2")
		if err != nil {
			fmt.Printf("WARNING: Failed to move winner hypothesis %s to L2: %v\n", winnerID, err)
		}
	}

	t.AuditLog("quint_decide", "finalize_decision", "agent", winnerID, "SUCCESS", map[string]string{"title": title, "drr": drrName}, "")

	logger.Info().Str("drr", drrName).Str("winner_id", winnerID).Msg("FinalizeDecision: completed successfully")

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
		logger.Warn().Err(err).Str("holon_id", holonID).Msg("failed to query dependencies")
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

	if err := t.MoveHypothesis(holonID, holon.Layer, newLayer); err != nil {
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
		JOIN active_holons h ON e.holon_id = h.id
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
		JOIN active_holons h ON e.holon_id = h.id
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
			result.WriteString(fmt.Sprintf("  → /q3-validate %s (refresh evidence)\n", holonID))
			result.WriteString("  → Deprecate: downgrade holon if decision needs rethinking\n")
			result.WriteString("  → Waive: accept risk temporarily with documented rationale\n\n")
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

	archivedCount, _ := t.DB.CountArchivedStaleEvidence(ctx)
	if archivedCount > 0 {
		result.WriteString("\n---\n\n### ARCHIVED (informational, no action needed)\n\n")
		result.WriteString(fmt.Sprintf("%d stale evidence records in archived holons.\n", archivedCount))
		result.WriteString("These holons are part of resolved decisions and don't require action.\n")
	}

	return result.String(), nil
}

type InternalizeResult struct {
	Status            string            // INITIALIZED, UPDATED, READY
	Phase             string            // Session phase (explicit, set by tools)
	SuggestedPhase    string            // Derived from knowledge state (informational)
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
	// v5.0.0: Decision Contexts
	ActiveContexts []DecisionContextSummary // Active (unclosed) decision contexts
	// Code Change Awareness (v5.0.0)
	CodeChanges                *CodeChangeDetectionResult
	StaleEvidenceCount         int64
	ArchivedStaleEvidenceCount int64 // Informational: stale evidence for archived holons
	ReverificationCount        int64
	// Affected Scope Warnings (v5.1.0)
	AffectedScopeWarnings []AffectedScopeWarning
}

// DecisionContextSummary represents an active decision context
type DecisionContextSummary struct {
	ID              string
	Title           string
	Scope           string
	Stage           ContextStage
	HypothesisCount int
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

type AffectedScopeWarning struct {
	DecisionID    string
	DecisionTitle string
	FilePath      string
	ChangeType    string // "modified", "removed"
	OldHash       string
	NewHash       string
}

// Internalize is the unified entry point for FPF sessions.
// It handles initialization, context updates, decay checking, and status in one call.
func (t *Tools) Internalize() (string, error) {
	defer t.RecordWork("Internalize", time.Now())

	logger.Info().Str("root_dir", t.RootDir).Msg("Internalize called")

	// v5.0.0: Phase is now derived from active decision contexts, not globally
	// Initialize with defaults, will update after loading active contexts
	result := InternalizeResult{
		Phase:          string(StageEmpty),
		SuggestedPhase: "No hypotheses yet",
		Role:           string(RoleObserver),
		LayerCounts:    make(map[string]int),
		NextAction:     "→ /q1-hypothesize to start reasoning",
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

		result.Phase = string(StageEmpty)
		result.SuggestedPhase = "No hypotheses yet"
		result.Role = string(RoleObserver)
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
		result.ArchivedStaleEvidenceCount, _ = t.DB.CountArchivedStaleEvidence(ctx)
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
			// v5.1.0: Check affected scope for open decisions
			for _, d := range openDecisions {
				warnings := t.checkDecisionAffectedScope(d.ID, d.Title)
				result.AffectedScopeWarnings = append(result.AffectedScopeWarnings, warnings...)
			}
		}
		resolvedDecisions, err := t.GetRecentResolvedDecisions(ctx, 5)
		if err == nil {
			result.ResolvedDecisions = resolvedDecisions
		}

		// v5.0.0: Get active decision contexts and derive phase from them
		activeContexts, err := t.GetActiveDecisionContexts(ctx)
		if err == nil {
			result.ActiveContexts = activeContexts
			// Derive session phase from active contexts
			if len(activeContexts) > 0 {
				// Use the most advanced stage among active contexts
				mostAdvancedStage := t.getMostAdvancedStage(activeContexts)
				result.Phase = string(mostAdvancedStage)
				result.SuggestedPhase, result.NextAction = GetContextStageDescription(mostAdvancedStage)
			}
		}
	}

	// Fallback next action based on layer counts if no active contexts
	if len(result.ActiveContexts) == 0 {
		result.NextAction = t.getNextAction(StageEmpty, result.LayerCounts["L0"], result.LayerCounts["L1"], result.LayerCounts["L2"])
	}

	logger.Info().
		Str("status", result.Status).
		Int("active_contexts", len(result.ActiveContexts)).
		Int("decay_warnings", len(result.DecayWarnings)).
		Int("scope_warnings", len(result.AffectedScopeWarnings)).
		Msg("Internalize: completed")

	return t.formatInternalizeOutput(result), nil
}

func (t *Tools) checkDecisionAffectedScope(drrID, drrTitle string) []AffectedScopeWarning {
	var warnings []AffectedScopeWarning

	contract, err := t.getDRRContract(drrID)
	if err != nil || contract == nil {
		return warnings
	}

	if len(contract.AffectedHashes) == 0 {
		return warnings
	}

	for file, oldHash := range contract.AffectedHashes {
		if oldHash == "_missing_" {
			continue
		}
		fullPath := filepath.Join(t.RootDir, file)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			warnings = append(warnings, AffectedScopeWarning{
				DecisionID:    drrID,
				DecisionTitle: drrTitle,
				FilePath:      file,
				ChangeType:    "removed",
				OldHash:       oldHash,
			})
			continue
		}
		hash := sha256.Sum256(content)
		currentHash := hex.EncodeToString(hash[:8])
		if currentHash != oldHash {
			warnings = append(warnings, AffectedScopeWarning{
				DecisionID:    drrID,
				DecisionTitle: drrTitle,
				FilePath:      file,
				ChangeType:    "modified",
				OldHash:       oldHash,
				NewHash:       currentHash,
			})
		}
	}
	return warnings
}

func (t *Tools) formatInternalizeOutput(r InternalizeResult) string {
	var sb strings.Builder

	sb.WriteString("=== QUINT INTERNALIZE ===\n\n")
	sb.WriteString(fmt.Sprintf("Status: %s\n", r.Status))
	sb.WriteString(fmt.Sprintf("Session Phase: %s\n", r.Phase))
	if r.SuggestedPhase != "" && r.SuggestedPhase != r.Phase {
		sb.WriteString(fmt.Sprintf("Suggested Phase: %s (based on knowledge state)\n", r.SuggestedPhase))
	}
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

	// v5.0.0: Active Decision Contexts
	if len(r.ActiveContexts) > 0 {
		sb.WriteString(fmt.Sprintf("Active Decision Contexts (%d/3):\n", len(r.ActiveContexts)))
		for _, dc := range r.ActiveContexts {
			desc, _ := GetContextStageDescription(dc.Stage)
			sb.WriteString(fmt.Sprintf("  - %s: %s (%d hypotheses) [%s]\n",
				dc.ID, dc.Title, dc.HypothesisCount, desc))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("No active decision contexts. Use /q1-hypothesize to start.\n\n")
	}

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
	// Exclude archived holons from active counts - they don't need action
	activeStaleCount := r.StaleEvidenceCount - r.ArchivedStaleEvidenceCount
	if activeStaleCount < 0 {
		activeStaleCount = 0
	}

	if activeStaleCount > 0 || r.ReverificationCount > 0 {
		sb.WriteString("🔴 CODE CHANGE IMPACTS:\n")
		if activeStaleCount > 0 {
			sb.WriteString(fmt.Sprintf("  - %d evidence records marked stale\n", activeStaleCount))
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

	if r.ArchivedStaleEvidenceCount > 0 {
		sb.WriteString(fmt.Sprintf("ℹ Archived holons: %d stale evidence records (no action needed)\n\n", r.ArchivedStaleEvidenceCount))
	}

	if len(r.AffectedScopeWarnings) > 0 {
		sb.WriteString("🔴 AFFECTED SCOPE CHANGED:\n")
		grouped := make(map[string][]AffectedScopeWarning)
		for _, w := range r.AffectedScopeWarnings {
			grouped[w.DecisionID] = append(grouped[w.DecisionID], w)
		}
		for drrID, warnings := range grouped {
			title := warnings[0].DecisionTitle
			sb.WriteString(fmt.Sprintf("  %s (%s):\n", drrID, title))
			for _, w := range warnings {
				if w.ChangeType == "removed" {
					sb.WriteString(fmt.Sprintf("    - %s: file removed\n", w.FilePath))
				} else {
					sb.WriteString(fmt.Sprintf("    - %s: modified (was %s, now %s)\n", w.FilePath, w.OldHash, w.NewHash))
				}
			}
		}
		sb.WriteString("  → Check changes with 'git diff', then either:\n")
		sb.WriteString("    • /q-implement — if changes don't invalidate decision, proceed with implementation\n")
		sb.WriteString("    • /q-resolve abandoned — if changes make decision obsolete\n")
		sb.WriteString("    • /q1-hypothesize — start fresh if requirements changed\n\n")
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

	logger.Info().
		Str("query", query).
		Str("scope", scope).
		Str("layer_filter", layerFilter).
		Str("status_filter", statusFilter).
		Int("limit", limit).
		Msg("Search called")

	if t.DB == nil {
		logger.Error().Msg("Search: database not initialized")
		return "", fmt.Errorf("database not initialized - run quint_internalize first")
	}

	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	ctx := context.Background()
	results, err := t.DB.Search(ctx, query, scope, layerFilter, statusFilter, limit)
	if err != nil {
		logger.Error().Err(err).Str("query", query).Msg("Search: query failed")
		return "", fmt.Errorf("search failed: %w", err)
	}

	logger.Debug().Int("result_count", len(results)).Msg("Search: query executed")

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
	// v5.0.0: Derive stage from active decision contexts
	stage := StageEmpty
	if t.DB != nil {
		ctx := context.Background()
		if activeContexts, err := t.GetActiveDecisionContexts(ctx); err == nil && len(activeContexts) > 0 {
			stage = t.getMostAdvancedStage(activeContexts)
		}
	}

	var sb strings.Builder

	// Parseable header
	sb.WriteString(fmt.Sprintf("STAGE: %s\n", stage))
	sb.WriteString(fmt.Sprintf("ROLE: %s\n\n", RoleObserver))

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
	sb.WriteString(t.getNextAction(stage, l0, l1, l2))

	return sb.String(), nil
}

// countHolons counts markdown files in a knowledge layer directory.
func (t *Tools) countHolons(layer string) int {
	// v5.0.0: hypotheses are DB-only
	if t.DB == nil {
		return 0
	}
	var count int
	err := t.DB.GetRawDB().QueryRow(
		`SELECT COUNT(*) FROM holons WHERE layer = ? AND type = 'hypothesis'`,
		layer,
	).Scan(&count)
	if err != nil {
		return 0
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

// getMostAdvancedStage returns the most advanced stage among active contexts.
// Stage progression: EMPTY < NEEDS_VERIFY < NEEDS_VALIDATION < NEEDS_AUDIT < READY_TO_DECIDE
func (t *Tools) getMostAdvancedStage(contexts []DecisionContextSummary) ContextStage {
	stagePriority := map[ContextStage]int{
		StageEmpty:          0,
		StageNeedsVerify:    1,
		StageNeedsValidation: 2,
		StageNeedsAudit:     3,
		StageReadyToDecide:  4,
	}

	maxStage := StageEmpty
	for _, c := range contexts {
		if stagePriority[c.Stage] > stagePriority[maxStage] {
			maxStage = c.Stage
		}
	}
	return maxStage
}

// getNextAction returns guidance for the next step based on context stage.
func (t *Tools) getNextAction(stage ContextStage, l0, l1, l2 int) string {
	switch stage {
	case StageEmpty:
		return "→ /q1-hypothesize to start reasoning\n"
	case StageNeedsVerify:
		if l0 > 0 {
			return fmt.Sprintf("→ %d L0 ready for /q2-verify\n", l0)
		}
		return "→ /q1-hypothesize to generate hypotheses\n"
	case StageNeedsValidation:
		if l1 > 0 {
			return fmt.Sprintf("→ %d L1 ready for /q3-validate\n", l1)
		}
		return "→ /q2-verify to check logic\n"
	case StageNeedsAudit:
		if l2 > 0 {
			return fmt.Sprintf("→ %d L2 ready for /q4-audit\n", l2)
		}
		return "→ /q3-validate to gather evidence\n"
	case StageReadyToDecide:
		return "→ /q5-decide to finalize\n"
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

	normalizedID := decisionID
	if strings.HasPrefix(decisionID, "DRR-") {
		parts := strings.SplitN(decisionID, "-", 5)
		if len(parts) == 5 {
			normalizedID = parts[4]
		}
	}

	pattern := filepath.Join(decisionsDir, fmt.Sprintf("DRR-*-%s.md", normalizedID))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil, nil
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
	ID        string
	Title     string
	Contract  *Contract
	DependsOn []string
	WinnerID  string
}

// ============================================
// CODE CHANGE AWARENESS TYPES (v5.0.0)
// ============================================

// CodeChangeImpact represents the impact of a code change
type CodeChangeImpact struct {
	Type       string  // "evidence_stale" | "propagated"
	File       string  // Changed file path
	EvidenceID string  // Affected evidence ID
	HolonID    string  // Affected holon ID
	HolonTitle string  // Holon title for display
	HolonLayer string  // L0/L1/L2/DRR
	PreviousR  float64 // R_eff before change
	Reason     string  // Human-readable reason
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

// ============================================
// CODE CHANGE DETECTION LOGIC (v5.1.0)
// ============================================

// detectCodeChanges detects code changes using content hash comparison (v5.1.0)
// This is simpler and more reliable than commit-based detection:
// - No git dependency for staleness detection
// - Works with uncommitted changes
// - No issues with rebase/amend/force-push
// - Direct content comparison, not commit history
func (t *Tools) detectCodeChanges(ctx context.Context) (*CodeChangeDetectionResult, error) {
	// Skip if no DB
	if t.DB == nil {
		return nil, nil
	}

	// Use hash-based detection (v5.1.0)
	return t.detectStaleEvidenceByHash(ctx)
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

// hashCarrierFiles returns "file1:hash1,file2:hash2" (sorted, 16-char hex each).
func (t *Tools) hashCarrierFiles(carrierRef string) string {
	if carrierRef == "" {
		return ""
	}

	files := strings.Split(carrierRef, ",")
	var hashes []string

	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}

		fullPath := filepath.Join(t.RootDir, file)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			hashes = append(hashes, fmt.Sprintf("%s:_missing_", file))
			continue
		}

		hash := sha256.Sum256(content)
		hashes = append(hashes, fmt.Sprintf("%s:%s", file, hex.EncodeToString(hash[:8])))
	}

	// Sort for consistent comparison
	sortedHashes := make([]string, len(hashes))
	copy(sortedHashes, hashes)
	for i := 0; i < len(sortedHashes)-1; i++ {
		for j := i + 1; j < len(sortedHashes); j++ {
			if sortedHashes[i] > sortedHashes[j] {
				sortedHashes[i], sortedHashes[j] = sortedHashes[j], sortedHashes[i]
			}
		}
	}

	return strings.Join(sortedHashes, ",")
}

func parseCarrierHashes(hashStr string) map[string]string {
	result := make(map[string]string)
	if hashStr == "" {
		return result
	}

	for _, pair := range strings.Split(hashStr, ",") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

func diffCarrierHashes(oldHash, newHash string) []string {
	oldMap := parseCarrierHashes(oldHash)
	newMap := parseCarrierHashes(newHash)

	var changed []string

	for file, oldH := range oldMap {
		newH, exists := newMap[file]
		if !exists || newH != oldH {
			changed = append(changed, file)
		}
	}

	for file := range newMap {
		if _, exists := oldMap[file]; !exists {
			changed = append(changed, file)
		}
	}

	return changed
}

func (t *Tools) detectStaleEvidenceByHash(ctx context.Context) (*CodeChangeDetectionResult, error) {
	result := &CodeChangeDetectionResult{}
	affectedHolons := make(map[string]bool)

	allEvidence, err := t.DB.GetEvidenceWithCarrier(ctx)
	if err != nil {
		return nil, err
	}

	for _, e := range allEvidence {
		if !e.CarrierRef.Valid || e.CarrierRef.String == "" {
			continue
		}
		// Skip already stale
		if e.IsStale.Valid && e.IsStale.Int64 == 1 {
			continue
		}

		carrierRef := e.CarrierRef.String
		storedHash := ""
		if e.CarrierHash.Valid {
			storedHash = e.CarrierHash.String
		}

		// Compute current hash
		currentHash := t.hashCarrierFiles(carrierRef)

		// If no stored hash, this is legacy evidence - compute and store baseline
		if storedHash == "" {
			// Update evidence with current hash as baseline
			_, err := t.DB.GetRawDB().ExecContext(ctx,
				"UPDATE evidence SET carrier_hash = ? WHERE id = ?",
				currentHash, e.ID)
			if err != nil {
				logger.Warn().Err(err).Str("evidence_id", e.ID).Msg("failed to update carrier_hash baseline")
			}
			continue
		}

		// Compare hashes
		changedFiles := diffCarrierHashes(storedHash, currentHash)
		if len(changedFiles) == 0 {
			continue
		}

		// Get holon's current R for reporting
		holon, _ := t.DB.GetHolon(ctx, e.HolonID)
		previousR := 0.0
		if holon.CachedRScore.Valid {
			previousR = holon.CachedRScore.Float64
		}

		// Mark evidence as stale
		reason := fmt.Sprintf("carrier content changed: %s", strings.Join(changedFiles, ", "))
		if err := t.DB.MarkEvidenceStale(ctx, e.ID, reason); err != nil {
			logger.Warn().Err(err).Str("evidence_id", e.ID).Msg("failed to mark evidence stale")
			continue
		}

		// Record impact
		result.Impacts = append(result.Impacts, CodeChangeImpact{
			Type:       "evidence_stale",
			File:       strings.Join(changedFiles, ", "),
			EvidenceID: e.ID,
			HolonID:    e.HolonID,
			HolonTitle: holon.Title,
			HolonLayer: holon.Layer,
			PreviousR:  previousR,
			Reason:     reason,
		})

		result.TotalStale++
		affectedHolons[e.HolonID] = true
	}

	// Mark affected holons as needing reverification
	for holonID := range affectedHolons {
		reason := "evidence became stale due to carrier file changes"
		if err := t.DB.MarkHolonNeedsReverification(ctx, holonID, reason); err != nil {
			logger.Warn().Err(err).Msg("failed to mark holon for reverification")
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

	logger.Info().Str("drr_id", drrID).Msg("Implement called")

	if t.DB == nil {
		logger.Error().Msg("Implement: database not initialized")
		return "", fmt.Errorf("database not initialized - run quint_internalize first")
	}

	// Run code change detection before implementation (v5.0.0)
	// This ensures any changed carrier files are detected and evidence marked stale
	ctx := context.Background()
	if _, err := t.detectCodeChanges(ctx); err != nil {
		logger.Warn().Err(err).Msg("code change detection failed")
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

	// Check if affected_scope files changed since decision (v5.1.0)
	var affectedScopeWarnings []string
	if drr.Contract.AffectedHashes != nil && len(drr.Contract.AffectedHashes) > 0 {
		for file, oldHash := range drr.Contract.AffectedHashes {
			fullPath := filepath.Join(t.RootDir, file)
			content, err := os.ReadFile(fullPath)
			if err != nil {
				if oldHash != "_missing_" {
					affectedScopeWarnings = append(affectedScopeWarnings,
						fmt.Sprintf("⚠️ %s: file removed since decision", file))
				}
				continue
			}
			hash := sha256.Sum256(content)
			currentHash := hex.EncodeToString(hash[:8])
			if currentHash != oldHash {
				affectedScopeWarnings = append(affectedScopeWarnings,
					fmt.Sprintf("⚠️ %s: content changed since decision (was %s, now %s)", file, oldHash, currentHash))
			}
		}
	}

	// Collect inherited constraints from dependency chain
	inherited := t.collectInheritedConstraints(drr.DependsOn, make(map[string]bool))

	// Collect warnings about stale evidence (v5.0.0)
	// Include winner hypothesis in checks - evidence is attached to winner, not DRR
	dependsOnWithWinner := drr.DependsOn
	if drr.WinnerID != "" {
		dependsOnWithWinner = append([]string{drr.WinnerID}, dependsOnWithWinner...)
	}
	warnings := t.collectImplementationWarnings(ctx, normalizedID, dependsOnWithWinner)
	warningsText := t.formatImplementationWarnings(warnings)

	// Add affected_scope warnings (v5.1.0)
	var allWarnings strings.Builder
	if len(affectedScopeWarnings) > 0 {
		allWarnings.WriteString("# ⚠️ AFFECTED SCOPE CHANGED\n\n")
		allWarnings.WriteString("The following files changed since this decision was made:\n\n")
		for _, w := range affectedScopeWarnings {
			allWarnings.WriteString(fmt.Sprintf("  %s\n", w))
		}
		allWarnings.WriteString("\n**Action required:** Re-verify the decision is still valid before implementing.\n")
		allWarnings.WriteString("Run `/q2-verify` on the winning hypothesis or create a new decision.\n\n")
	}
	if warningsText != "" {
		allWarnings.WriteString(warningsText)
		allWarnings.WriteString("\n")
	}

	// Format the implementation directive
	directive := t.formatImplementDirective(drr, inherited)

	// Prepend warnings if any exist
	if allWarnings.Len() > 0 {
		return allWarnings.String() + "\n" + directive, nil
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

	logger.Info().Str("drr_id", drr.ID).Msg("Implement: directive generated")

	return sb.String()
}

// Resolve records the outcome of a decision: implemented, abandoned, or superseded.
func (t *Tools) Resolve(input ResolveInput) (string, error) {
	defer t.RecordWork("Resolve", time.Now())

	logger.Info().
		Str("decision_id", input.DecisionID).
		Str("resolution", input.Resolution).
		Str("reference", input.Reference).
		Bool("criteria_verified", input.CriteriaVerified).
		Msg("Resolve called")

	if t.DB == nil {
		logger.Error().Msg("Resolve: database not initialized")
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
			logger.Warn().Err(err).Msg("failed to create SupersededBy relation")
		}
	}

	// Compute carrier hash for staleness detection (v5.1.0)
	carrierHash := t.hashCarrierFiles(carrierRef)

	// Get current commit for metadata (optional)
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
		carrierHash,
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

	logger.Info().
		Str("decision_id", input.DecisionID).
		Str("resolution", input.Resolution).
		Msg("Resolve: completed successfully")

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
// Note: With context-based stage, reset is now primarily for audit logging.
// Context management is done via decision contexts, not global phase.
func (t *Tools) ResetCycle(reason, contextID string, abandonAll bool) (string, error) {
	defer t.RecordWork("ResetCycle", time.Now())

	logger.Info().
		Str("reason", reason).
		Str("context_id", contextID).
		Bool("abandon_all", abandonAll).
		Msg("ResetCycle called")

	if reason == "" {
		reason = "user requested reset"
	}

	ctx := context.Background()
	var sb strings.Builder

	// Handle context abandonment
	if contextID != "" {
		if t.DB == nil {
			return "", fmt.Errorf("database not initialized")
		}
		if err := t.DB.AbandonContext(ctx, contextID); err != nil {
			return "", fmt.Errorf("failed to abandon context %s: %w", contextID, err)
		}
		t.AuditLog("quint_reset", "abandon_context", "agent", contextID, "SUCCESS",
			map[string]string{"reason": reason}, "")
		sb.WriteString(fmt.Sprintf("Abandoned context: %s\n", contextID))
		sb.WriteString(fmt.Sprintf("Reason: %s\n", reason))
		return sb.String(), nil
	}

	if abandonAll {
		if t.DB == nil {
			return "", fmt.Errorf("database not initialized")
		}
		contexts, err := t.GetActiveDecisionContexts(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to get active contexts: %w", err)
		}
		if len(contexts) == 0 {
			return "No active contexts to abandon.\n", nil
		}
		var abandoned []string
		for _, c := range contexts {
			if err := t.DB.AbandonContext(ctx, c.ID); err != nil {
				logger.Warn().Err(err).Str("context_id", c.ID).Msg("failed to abandon context")
				continue
			}
			abandoned = append(abandoned, c.ID)
		}
		t.AuditLog("quint_reset", "abandon_all_contexts", "agent", "", "SUCCESS",
			map[string]string{"reason": reason, "count": fmt.Sprintf("%d", len(abandoned))}, "")
		sb.WriteString(fmt.Sprintf("Abandoned %d contexts:\n", len(abandoned)))
		for _, id := range abandoned {
			sb.WriteString(fmt.Sprintf("  - %s\n", id))
		}
		sb.WriteString(fmt.Sprintf("Reason: %s\n", reason))
		return sb.String(), nil
	}

	// Default: end session without abandoning contexts
	// v5.0.0: Derive stage from active contexts
	currentStage := StageEmpty
	if activeContexts, err := t.GetActiveDecisionContexts(ctx); err == nil && len(activeContexts) > 0 {
		currentStage = t.getMostAdvancedStage(activeContexts)
	}

	sb.WriteString(fmt.Sprintf("Stage at reset: %s\n", currentStage))
	sb.WriteString(fmt.Sprintf("L0: %d, L1: %d, L2: %d, DRR: %d\n",
		t.countHolons("L0"), t.countHolons("L1"), t.countHolons("L2"), t.countDRRs()))

	if t.DB != nil {
		openDecisions, err := t.GetOpenDecisions(ctx)
		if err == nil && len(openDecisions) > 0 {
			sb.WriteString(fmt.Sprintf("Open decisions: %d\n", len(openDecisions)))
			for _, d := range openDecisions {
				sb.WriteString(fmt.Sprintf("  - %s\n", d.ID))
			}
		}
	}

	t.AuditLog("quint_reset", "cycle_reset", "agent", "", "SUCCESS",
		map[string]string{"reason": reason, "from_stage": string(currentStage)},
		sb.String())

	return fmt.Sprintf("Cycle reset. Session ended.\nPrevious stage: %s\nReason: %s\n\n%s",
		currentStage, reason, sb.String()), nil
}

type CompactResult struct {
	Mode            string
	RetentionDays   int64
	CompactedCount  int
	EligibleCount   int64
	CompactedHolons []string
	Errors          []string
}

// Compact removes verbose data from archived holons older than retentionDays.
// Mode: "preview" (default) or "execute".
func (t *Tools) Compact(mode string, retentionDays int64) (string, error) {
	defer t.RecordWork("Compact", time.Now())

	if t.DB == nil {
		return "", fmt.Errorf("database not available")
	}

	if mode == "" {
		mode = "preview"
	}
	if retentionDays <= 0 {
		retentionDays = 90
	}

	ctx := context.Background()
	result := CompactResult{
		Mode:          mode,
		RetentionDays: retentionDays,
	}

	count, err := t.DB.CountCompactableHolons(ctx, retentionDays)
	if err != nil {
		return "", fmt.Errorf("failed to count compactable holons: %w", err)
	}
	result.EligibleCount = count

	if count == 0 {
		return fmt.Sprintf("No holons eligible for compaction (retention: %d days).\n\nAll archived holons are either:\n- Less than %d days old, or\n- Already compacted", retentionDays, retentionDays), nil
	}

	holons, err := t.DB.GetArchivedHolonsForCompaction(ctx, retentionDays)
	if err != nil {
		return "", fmt.Errorf("failed to get compactable holons: %w", err)
	}

	var sb strings.Builder

	if mode == "preview" {
		sb.WriteString(fmt.Sprintf("## Compaction Preview (retention: %d days)\n\n", retentionDays))
		sb.WriteString(fmt.Sprintf("**%d holons eligible for compaction:**\n\n", count))
		sb.WriteString("| ID | Title | Layer | Decision | Outcome | Resolved |\n")
		sb.WriteString("|-----|-------|-------|----------|---------|----------|\n")

		for _, h := range holons {
			resolvedAt := "unknown"
			if h.ResolvedAt != nil {
				if t, ok := h.ResolvedAt.(time.Time); ok {
					resolvedAt = t.Format("2006-01-02")
				} else if s, ok := h.ResolvedAt.(string); ok && len(s) >= 10 {
					resolvedAt = s[:10]
				}
			}
			outcome := h.DecisionOutcome
			if outcome == "selects" {
				outcome = "SELECTED"
			} else if outcome == "rejects" {
				outcome = "REJECTED"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
				truncateString(h.ID, 12),
				truncateString(h.Title, 30),
				h.Layer,
				truncateString(h.DecisionTitle, 20),
				outcome,
				resolvedAt))
		}

		sb.WriteString("\n**Compaction removes:**\n")
		sb.WriteString("- Evidence records\n")
		sb.WriteString("- Characteristics\n")
		sb.WriteString("- Waivers\n")
		sb.WriteString("- Detailed content (replaced with '[COMPACTED]')\n")
		sb.WriteString("\n**Preserved:**\n")
		sb.WriteString("- Holon ID, title, type, kind, layer\n")
		sb.WriteString("- Relations (decision links)\n")
		sb.WriteString("- Audit log entries\n")
		sb.WriteString("\nTo execute: `quint_compact(mode=\"execute\", retention_days=")
		sb.WriteString(fmt.Sprintf("%d)`\n", retentionDays))

	} else if mode == "execute" {
		sb.WriteString(fmt.Sprintf("## Compaction Executed (retention: %d days)\n\n", retentionDays))

		for _, h := range holons {
			err := t.DB.CompactHolon(ctx, h.ID)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", h.ID, err))
				continue
			}
			result.CompactedHolons = append(result.CompactedHolons, h.ID)
			result.CompactedCount++
		}

		sb.WriteString(fmt.Sprintf("**Compacted %d holons**\n\n", result.CompactedCount))

		if len(result.Errors) > 0 {
			sb.WriteString("**Errors:**\n")
			for _, e := range result.Errors {
				sb.WriteString(fmt.Sprintf("- %s\n", e))
			}
			sb.WriteString("\n")
		}

		t.AuditLog("quint_compact", "compaction",
			string(RoleMaintainer), "",
			"SUCCESS",
			map[string]interface{}{
				"retention_days": retentionDays,
				"compacted":      result.CompactedCount,
				"errors":         len(result.Errors),
			},
			fmt.Sprintf("Compacted %d holons", result.CompactedCount))

		sb.WriteString("Compaction complete. Holon metadata and decision links preserved.\n")

	} else {
		return "", fmt.Errorf("invalid mode: %s (use 'preview' or 'execute')", mode)
	}

	return sb.String(), nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
