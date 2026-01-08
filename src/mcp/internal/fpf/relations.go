package fpf

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/assurance"
	"github.com/m0n0x41d/quint-code/logger"
)

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

	if _, err := t.DB.GetHolon(ctx, contextID); err == nil {
		return "", fmt.Errorf("decision context %q already exists. Use this ID in quint_propose or choose a different title", contextID)
	}

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

		dc.Stage = t.FSM.GetContextStage(dc.ID)

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
