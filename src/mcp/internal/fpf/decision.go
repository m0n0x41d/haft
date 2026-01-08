package fpf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/logger"

	"github.com/google/uuid"
)

func (t *Tools) FinalizeDecision(ctx context.Context, title, winnerID string, rejectedIDs []string, decisionContext, decision, rationale, consequences, characteristics, contractJSON string, closeContext bool) (string, error) {
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
		if len(contract.AffectedScope) > 0 {
			contract.AffectedHashes = make(map[string]string)
			for _, scopeRef := range contract.AffectedScope {
				scopeRef = strings.TrimSpace(scopeRef)
				if scopeRef == "" {
					continue
				}
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

	if t.DB != nil {
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

		closedContexts := make(map[string]bool)
		if closeContext {
			allHypotheses := append([]string{winnerID}, rejectedIDs...)
			for _, hypID := range allHypotheses {
				if hypID == "" {
					continue
				}
				if dcID := t.getDecisionContext(ctx, hypID); dcID != "" && !closedContexts[dcID] {
					if err := t.createRelation(ctx, drrID, "closes", dcID, 3); err != nil {
						logger.Warn().Err(err).Str("decision_context", dcID).Msg("failed to create closes relation")
					}
					if err := t.DB.CloseContext(ctx, dcID); err != nil {
						logger.Warn().Err(err).Str("decision_context", dcID).Msg("failed to close context status")
					}
					closedContexts[dcID] = true
				}
			}
		}

		for dcID := range closedContexts {
			orphanIDs, err := t.DB.GetOrphanedHypotheses(ctx, dcID)
			if err != nil {
				logger.Warn().Err(err).Str("decision_context", dcID).Msg("failed to query orphaned hypotheses")
				continue
			}
			for _, orphanID := range orphanIDs {
				if err := t.createRelation(ctx, drrID, "closes", orphanID, 3); err != nil {
					logger.Warn().Err(err).Str("orphan_id", orphanID).Msg("failed to close orphaned hypothesis")
				} else {
					logger.Info().Str("orphan_id", orphanID).Msg("closed orphaned hypothesis")
				}
			}
		}
	}

	if winnerID != "" {
		err := t.MoveHypothesis(winnerID, "L1", "L2")
		if err != nil {
			logger.Warn().Err(err).Str("winner_id", winnerID).Msg("failed to move winner hypothesis to L2")
		}
	}

	t.AuditLog("quint_decide", "finalize_decision", "agent", winnerID, "SUCCESS", map[string]string{"title": title, "drr": drrName}, "")

	logger.Info().Str("drr", drrName).Str("winner_id", winnerID).Msg("FinalizeDecision: completed successfully")

	return drrPath, nil
}

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

	content, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read DRR file: %w", err)
	}

	frontmatter, _, hasFM := parseFrontmatter(string(content))
	if !hasFM {
		return nil, nil
	}

	contractPrefix := "contract: "
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.HasPrefix(line, contractPrefix) {
			contractJSON := strings.TrimPrefix(line, contractPrefix)
			var contract Contract
			if err := json.Unmarshal([]byte(contractJSON), &contract); err != nil {
				return nil, nil
			}
			return &contract, nil
		}
	}

	return nil, nil
}

func (t *Tools) Resolve(ctx context.Context, input ResolveInput) (string, error) {
	defer t.RecordWork("Resolve", time.Now())

	logger.Info().
		Str("decision_id", input.DecisionID).
		Str("resolution", input.Resolution).
		Str("reference", input.Reference).
		Bool("criteria_verified", input.CriteriaVerified).
		Msg("Resolve called")

	if t.DB == nil {
		logger.Error().Msg("Resolve: database not initialized")
		return "", ErrDatabaseNotInitialized
	}

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

		if err := t.createRelation(ctx, input.DecisionID, "supersededBy", input.SupersededBy, 3); err != nil {
			logger.Warn().Err(err).Msg("failed to create supersededBy relation")
		}
	}

	carrierHash := t.hashCarrierFiles(carrierRef)

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
		DefaultFormalityLevel,
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
