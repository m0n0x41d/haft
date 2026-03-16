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
	"supersededBy":  true,
	"refines":       true,
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

func (t *Tools) CreateContext(ctx context.Context, title, scope, description string) (string, error) {
	defer t.RecordWork("CreateContext", time.Now())

	logger.Info().
		Str("title", title).
		Str("scope", scope).
		Msg("CreateContext called")

	if t.DB == nil {
		logger.Error().Msg("CreateContext: database not initialized")
		return "", ErrDatabaseNotInitialized
	}
	if title == "" {
		return "", fmt.Errorf("title is required")
	}
	contextID := "dc-" + t.Slugify(title)

	if _, err := t.DB.GetHolon(ctx, contextID); err == nil {
		return "", fmt.Errorf("decision context %q already exists. Use this ID in quint_propose or choose a different title", contextID)
	}

	activeContexts, err := t.GetActiveDecisionContexts(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get active contexts: %w", err)
	}
	if len(activeContexts) >= MaxActiveContexts {
		var contextList strings.Builder
		for _, c := range activeContexts {
			contextList.WriteString(fmt.Sprintf("\n  - %s: %s", c.ID, c.Title))
		}
		return "", fmt.Errorf("BLOCKED: maximum %d active decision contexts allowed (have %d).\n\nActive contexts:%s\n\n⚠️ USER ACTION REQUIRED: Ask user whether to:\n  1. Use an existing context (pass one of the dc-* IDs above to quint_propose)\n  2. Complete a context via /q5-decide\n  3. Abandon a context via /q-reset with context_id parameter", MaxActiveContexts, len(activeContexts), contextList.String())
	}

	content := fmt.Sprintf("# Decision Context: %s\n\nScope: %s\n", title, scope)
	if description != "" {
		content += fmt.Sprintf("\n## Problem Statement\n\n%s\n", description)
	}
	content += "\nHypotheses will be grouped under this context for decision-making."

	if err := t.DB.CreateHolon(ctx, contextID, "decision_context", "system", "L0", title, content, "default", scope, "", ""); err != nil {
		return "", fmt.Errorf("failed to create decision context: %w", err)
	}

	t.AuditLog("quint_context", "create_context", "agent", contextID, "SUCCESS",
		map[string]string{"title": title, "scope": scope}, "")

	logger.Info().Str("context_id", contextID).Msg("CreateContext: completed")

	return fmt.Sprintf("%s\n\n→ Use decision_context=\"%s\" in quint_propose to add hypotheses to this context.", contextID, contextID), nil
}

func (t *Tools) GetActiveDecisionContexts(ctx context.Context) ([]DecisionContextSummary, error) {
	if t.DB == nil {
		return nil, ErrDatabaseNotInitialized
	}

	rows, err := t.DB.GetActiveDecisionContexts(ctx)
	if err != nil {
		return nil, err
	}

	var contexts []DecisionContextSummary
	for _, row := range rows {
		dc := DecisionContextSummary{
			ID:              row.ID,
			Title:           row.Title,
			Scope:           row.Scope,
			Stage:           t.FSM.GetContextStage(row.ID),
			HypothesisCount: int(t.DB.GetHypothesisCountForContext(ctx, row.ID)),
		}
		dc.DiversityWarning = t.checkApproachDiversity(ctx, row.ID)
		contexts = append(contexts, dc)
	}

	return contexts, nil
}

func (t *Tools) checkApproachDiversity(ctx context.Context, dcID string) string {
	if t.DB == nil {
		return ""
	}

	stats := t.DB.GetApproachTypeDistribution(ctx, dcID)
	if len(stats) == 0 {
		return ""
	}

	var totalWithType int64
	var typedApproach string
	for _, stat := range stats {
		if stat.ApproachType != "" {
			totalWithType += stat.Count
			if typedApproach == "" {
				typedApproach = stat.ApproachType
			} else if typedApproach != stat.ApproachType {
				return ""
			}
		}
	}

	if totalWithType > 1 && typedApproach != "" {
		return fmt.Sprintf("All %d hypotheses use '%s' approach. Consider exploring alternative approaches for comprehensive coverage.", totalWithType, typedApproach)
	}

	return ""
}

func (t *Tools) getDecisionContext(ctx context.Context, holonID string) string {
	if t.DB == nil {
		return ""
	}
	return t.DB.GetDecisionContextForHolon(ctx, holonID)
}

func (t *Tools) isDecisionContextClosed(ctx context.Context, dcID string) string {
	if t.DB == nil || dcID == "" {
		return ""
	}
	return t.DB.GetClosingDRRForContext(ctx, dcID)
}

func (t *Tools) isHypothesisInOpenDRR(ctx context.Context, hypID string) string {
	if t.DB == nil || hypID == "" {
		return ""
	}
	return t.DB.GetOpenDRRForHypothesis(ctx, hypID)
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

func (t *Tools) LinkHolons(ctx context.Context, sourceID, targetID string, cl int) (string, error) {
	defer t.RecordWork("LinkHolons", time.Now())

	logger.Info().
		Str("source_id", sourceID).
		Str("target_id", targetID).
		Int("congruence_level", cl).
		Msg("LinkHolons called")

	if t.DB == nil {
		logger.Error().Msg("LinkHolons: database not initialized")
		return "", ErrDatabaseNotInitialized
	}

	source, err := t.DB.GetHolon(ctx, sourceID)
	if err != nil {
		return "", fmt.Errorf("source holon '%s' not found", sourceID)
	}

	_, err = t.DB.GetHolon(ctx, targetID)
	if err != nil {
		return "", fmt.Errorf("target holon '%s' not found", targetID)
	}

	if cyclic, _ := t.wouldCreateCycle(ctx, sourceID, targetID); cyclic {
		return "", ErrCyclicDependency("quint_link")
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
