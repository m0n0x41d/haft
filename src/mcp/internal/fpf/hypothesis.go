package fpf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/logger"
)

func (t *Tools) suggestDependencies(ctx context.Context, title, content string) []DependencySuggestion {
	if t.DB == nil {
		return nil
	}

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

func (t *Tools) ProposeHypothesis(ctx context.Context, title, content, scope, kind, rationale string, decisionContext string, dependsOn []string, dependencyCL int, approachType string, refines string) (string, error) {
	defer t.RecordWork("ProposeHypothesis", time.Now())

	logger.Info().
		Str("title", title).
		Str("kind", kind).
		Str("scope", scope).
		Str("decision_context", decisionContext).
		Str("approach_type", approachType).
		Str("refines", refines).
		Int("dependency_count", len(dependsOn)).
		Msg("ProposeHypothesis called")

	if t.DB == nil {
		logger.Error().Msg("ProposeHypothesis: database not initialized")
		return "", ErrDatabaseNotInitialized
	}

	var refinesTarget string
	if refines != "" {
		h, err := t.DB.GetHolon(ctx, refines)
		if err != nil {
			return "", fmt.Errorf("refines target %q not found", refines)
		}
		if h.Layer != "L0" {
			return "", fmt.Errorf("refines target %q is at %s, must be L0", refines, h.Layer)
		}
		if h.Type != "hypothesis" {
			return "", fmt.Errorf("refines target %q is type %q, must be hypothesis", refines, h.Type)
		}
		existingRefines, _ := t.DB.GetRelationsByTarget(ctx, refines, "refines")
		if len(existingRefines) > 0 {
			return "", fmt.Errorf("hypothesis %q is already refined by %q", refines, existingRefines[0].SourceID)
		}
		refinesTarget = refines
		if decisionContext == "" {
			decisionContext = t.getDecisionContext(ctx, refines)
		}
	}

	if decisionContext == "" {
		return "", fmt.Errorf("decision_context is required. Create one first with quint_context(title=\"Your Decision Title\")")
	}

	holon, err := t.DB.GetHolon(ctx, decisionContext)
	if err != nil {
		return "", fmt.Errorf("decision_context %q not found. Create it first with quint_context", decisionContext)
	}
	if holon.Type != "decision_context" {
		return "", fmt.Errorf("%q is type %q, not decision_context. Use quint_context to create a proper context, then use the dc-* ID it returns", decisionContext, holon.Type)
	}

	slug := t.Slugify(title)
	body := fmt.Sprintf("# Hypothesis: %s\n\n%s\n\n## Rationale\n%s", title, content, rationale)

	if err := t.DB.CreateHolon(ctx, slug, "hypothesis", kind, "L0", title, body, "default", scope, "", approachType); err != nil {
		logger.Error().Err(err).Str("slug", slug).Msg("ProposeHypothesis: failed to create holon")
		t.AuditLog("quint_propose", "create_hypothesis", "agent", slug, "ERROR", map[string]string{"title": title, "kind": kind}, err.Error())
		return "", fmt.Errorf("failed to create hypothesis in database: %w", err)
	}

	logger.Debug().Str("slug", slug).Str("layer", "L0").Msg("ProposeHypothesis: holon created")

	if err := t.createRelation(ctx, slug, "memberOf", decisionContext, 3); err != nil {
		logger.Warn().Err(err).Msg("failed to create MemberOf relation")
	}

	if refinesTarget != "" {
		if err := t.createRelation(ctx, slug, "refines", refinesTarget, 3); err != nil {
			logger.Warn().Err(err).Str("refines", refinesTarget).Msg("failed to create refines relation")
		} else {
			logger.Info().Str("slug", slug).Str("refines", refinesTarget).Msg("ProposeHypothesis: created refines relation")
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

	var output strings.Builder
	output.WriteString(slug)

	if refinesTarget != "" {
		output.WriteString(fmt.Sprintf("\n\n✅ Refinement of %s", refinesTarget))
		output.WriteString("\n→ Original hypothesis preserved for audit trail")
		output.WriteString("\n→ Run /q2-verify to verify the refined hypothesis")
	}

	if len(dependsOn) == 0 && refinesTarget == "" && t.DB != nil {
		suggestions := t.suggestDependencies(ctx, title, content)
		if len(suggestions) > 0 {
			output.WriteString("\n\n⚠️ POTENTIAL DEPENDENCIES DETECTED\n\n")
			output.WriteString("Related holons found (ranked by relevance):\n")
			for _, s := range suggestions {
				output.WriteString(fmt.Sprintf("  • %s [%s] %s\n",
					s.HolonID, s.Layer, s.Title))
			}
			output.WriteString("\nConsider linking with:\n")
			output.WriteString(fmt.Sprintf("  quint_link(source_id=\"%s\", target_id=\"<id>\")\n", slug))
			output.WriteString("\nThis enables:\n")
			output.WriteString("  - WLNK applies to R_eff\n")
			output.WriteString("  - Invariants inherited from dependency\n")
			output.WriteString("  - Audit trail of architectural coupling\n")
		}
	}

	return output.String(), nil
}

func (t *Tools) VerifyHypothesis(ctx context.Context, hypothesisID, checksJSON, verdict, carrierFiles string) (string, error) {
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
			holon, err := t.DB.GetHolon(ctx, hypothesisID)
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

	if warning := t.checkDuplicateHypothesis(ctx, hypothesisID); warning != "" {
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

		if _, err := t.ManageEvidence(ctx, "verification", "add", hypothesisID, "verification", string(evidenceJSON), "pass", "L1", DefaultFormalityLevel, carrierRef, ""); err != nil {
			logger.Warn().Err(err).Str("hypothesis_id", hypothesisID).Msg("failed to record verification evidence")
		}

		if t.DB != nil && len(result.Predictions) > 0 {
			for i, pred := range result.Predictions {
				predID := pred.ID
				if predID == "" {
					predID = fmt.Sprintf("%s-pred-%d", hypothesisID, i+1)
				}
				predContent := fmt.Sprintf("IF %s THEN %s (testable by: %s)", pred.If, pred.Then, pred.TestableBy)
				if err := t.DB.AddPrediction(ctx, predID, hypothesisID, predContent); err != nil {
					logger.Warn().Err(err).Str("prediction_id", predID).Msg("failed to store prediction")
				}
			}
			logger.Info().Int("count", len(result.Predictions)).Str("hypothesis_id", hypothesisID).Msg("stored predictions")
		}

		logger.Info().Str("hypothesis_id", hypothesisID).Str("result", "L1").Msg("VerifyHypothesis: PASS - promoted to L1")
		t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "PASS", "result": "L1"}, "")

		var output strings.Builder
		output.WriteString(fmt.Sprintf("✅ Hypothesis %s promoted to L1\n\n", hypothesisID))
		output.WriteString(fmt.Sprintf("📋 Predictions (%d):\n", len(result.Predictions)))
		for _, pred := range result.Predictions {
			predID := pred.ID
			if predID == "" {
				predID = "P?"
			}
			output.WriteString(fmt.Sprintf("  %s: IF %s THEN %s\n      → testable by: %s\n", predID, pred.If, pred.Then, pred.TestableBy))
		}
		output.WriteString("\nThese predictions must be validated in /q3-validate.\n")
		if len(result.Risks) > 0 {
			output.WriteString("\n⚠️ Risks identified:\n")
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

		if _, err := t.ManageEvidence(ctx, "verification", "add", hypothesisID, "verification", string(evidenceJSON), "fail", "invalid", DefaultFormalityLevel, carrierRef, ""); err != nil {
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

	case "REFINE":
		logger.Debug().Str("hypothesis_id", hypothesisID).Msg("VerifyHypothesis: REFINE - keeping at L0 with feedback")

		if _, err := t.ManageEvidence(ctx, "verification", "add", hypothesisID, "verification_feedback", string(evidenceJSON), "refine", "L0", DefaultFormalityLevel, carrierRef, ""); err != nil {
			logger.Warn().Err(err).Str("hypothesis_id", hypothesisID).Msg("failed to record verification feedback")
		}

		logger.Info().Str("hypothesis_id", hypothesisID).Str("result", "L0").Msg("VerifyHypothesis: REFINE - feedback recorded")
		t.AuditLog("quint_verify", "verify_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "REFINE", "result": "L0"}, "")

		var output strings.Builder
		output.WriteString(fmt.Sprintf("🔄 Hypothesis %s needs refinement (remains at L0)\n\n", hypothesisID))
		output.WriteString("📋 Feedback recorded:\n")
		if result.TypeCheck.Verdict == "REFINE" {
			output.WriteString(fmt.Sprintf("  - Type check: %s\n", result.TypeCheck.Reasoning))
		}
		if result.ConstraintCheck.Verdict == "REFINE" {
			output.WriteString(fmt.Sprintf("  - Constraint check: %s\n", result.ConstraintCheck.Reasoning))
		}
		if result.LogicCheck.Verdict == "REFINE" {
			output.WriteString(fmt.Sprintf("  - Logic check: %s\n", result.LogicCheck.Reasoning))
		}
		if len(result.Risks) > 0 {
			output.WriteString("\n⚠️ Risks identified:\n")
			for _, r := range result.Risks {
				output.WriteString(fmt.Sprintf("  - %s\n", r))
			}
		}
		output.WriteString("\n→ To refine: use quint_propose with refines=\"" + hypothesisID + "\" to create updated hypothesis\n")
		output.WriteString("  Example: quint_propose(title=\"...\", ..., refines=\"" + hypothesisID + "\")\n")
		return output.String(), nil

	default:
		return "", fmt.Errorf("overall_verdict must be PASS, FAIL, or REFINE, got: %s", result.OverallVerdict)
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
		if verdict != "PASS" && verdict != "FAIL" && verdict != "REFINE" {
			return fmt.Errorf("%s: verdict must be PASS, FAIL, or REFINE, got: %s", c.name, c.check.Verdict)
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
	if verdict != "PASS" && verdict != "FAIL" && verdict != "REFINE" {
		return fmt.Errorf("overall_verdict must be PASS, FAIL, or REFINE, got: %s", r.OverallVerdict)
	}

	if verdict == "PASS" && len(r.Predictions) == 0 {
		return fmt.Errorf("L1 requires at least one testable prediction (FPF B.5)")
	}

	for i, pred := range r.Predictions {
		if pred.If == "" || pred.Then == "" {
			return fmt.Errorf("prediction %d: 'if' and 'then' fields are required", i+1)
		}
	}

	return nil
}

func (t *Tools) checkDuplicateHypothesis(ctx context.Context, hypothesisID string) string {
	if t.DB == nil {
		return ""
	}

	current, err := t.DB.GetHolon(ctx, hypothesisID)
	if err != nil || current.Title == "" {
		return ""
	}

	matches, err := t.DB.GetInvalidHolonsWithTitle(ctx, current.Title, hypothesisID)
	if err != nil {
		logger.Warn().Err(err).Msg("error querying duplicate hypotheses")
		return ""
	}

	if len(matches) > 0 {
		return fmt.Sprintf("Similar hypothesis previously failed: %v. Ensure this version addresses the failure reasons.", matches)
	}
	return ""
}

func (t *Tools) ValidateHypothesis(ctx context.Context, hypothesisID, testType, result, verdict, carrierFiles string) (string, error) {
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

	var testResult TestResult
	if err := json.Unmarshal([]byte(result), &testResult); err != nil {
		logger.Warn().Err(err).Msg("result is not structured TestResult, treating as legacy format")
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
	validUntil := ""

	switch strings.ToUpper(verdict) {
	case "PASS":
		if t.DB != nil {
			predictions, err := t.DB.GetPredictionsByHolon(ctx, hypothesisID)
			if err != nil {
				logger.Warn().Err(err).Msg("failed to get predictions")
			}

			if len(predictions) > 0 {
				coveredPreds := make(map[string]bool)
				for _, obs := range testResult.Observations {
					if obs.TestsPrediction != "" && obs.Supports {
						coveredPreds[obs.TestsPrediction] = true
					}
				}

				for _, pred := range predictions {
					predNum := strings.TrimPrefix(pred.ID, hypothesisID+"-pred-")
					predKey := fmt.Sprintf("P%s", predNum)
					isCovered := pred.Covered.Valid && pred.Covered.Int64 == 1
					if coveredPreds[predKey] && !isCovered {
						if err := t.DB.MarkPredictionCovered(ctx, pred.ID, ""); err != nil {
							logger.Warn().Err(err).Str("prediction_id", pred.ID).Msg("failed to mark prediction covered")
						}
					}
				}

				uncovered, err := t.DB.GetUncoveredPredictions(ctx, hypothesisID)
				if err != nil {
					logger.Warn().Err(err).Msg("failed to check uncovered predictions")
				}
				if len(uncovered) > 0 {
					var uncoveredList []string
					for _, u := range uncovered {
						uncoveredList = append(uncoveredList, u.Content)
					}
					t.AuditLog("quint_test", "validate_hypothesis", "agent", hypothesisID, "BLOCKED", map[string]string{"reason": "uncovered_predictions"}, "")
					return "", fmt.Errorf("L1→L2 blocked: %d uncovered predictions. Add observations with tests_prediction field: %v", len(uncovered), uncoveredList)
				}
			}
		}

		logger.Debug().Str("hypothesis_id", hypothesisID).Str("test_type", testType).Msg("ValidateHypothesis: adding validation evidence")
		if _, err := t.ManageEvidence(ctx, "validation", "add", hypothesisID, testType, string(evidenceJSON), "pass", "L2", DefaultFormalityLevel, carrierRef, validUntil); err != nil {
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
		if _, err := t.ManageEvidence(ctx, "validation", "add", hypothesisID, testType, string(evidenceJSON), "fail", "L1", DefaultFormalityLevel, carrierRef, validUntil); err != nil {
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

	case "REFINE":
		logger.Debug().Str("hypothesis_id", hypothesisID).Str("test_type", testType).Msg("ValidateHypothesis: REFINE - keeping at L1 with feedback")
		if _, err := t.ManageEvidence(ctx, "validation", "add", hypothesisID, testType+"_feedback", string(evidenceJSON), "refine", "L1", DefaultFormalityLevel, carrierRef, validUntil); err != nil {
			logger.Error().Err(err).Str("hypothesis_id", hypothesisID).Msg("ValidateHypothesis: failed to add feedback evidence")
			t.AuditLog("quint_test", "validate_hypothesis", "agent", hypothesisID, "ERROR", map[string]string{"verdict": "REFINE"}, err.Error())
			return "", err
		}

		logger.Info().Str("hypothesis_id", hypothesisID).Str("result", "L1").Msg("ValidateHypothesis: REFINE - feedback recorded")
		t.AuditLog("quint_test", "validate_hypothesis", "agent", hypothesisID, "SUCCESS", map[string]string{"verdict": "REFINE", "result": "L1"}, "")

		var output strings.Builder
		output.WriteString(fmt.Sprintf("🔄 Hypothesis %s needs refinement (remains at L1)\n\n", hypothesisID))
		output.WriteString(fmt.Sprintf("Test type: %s\n", testType))
		output.WriteString("📋 Feedback recorded:\n")
		if testResult.Reasoning != "" {
			output.WriteString(fmt.Sprintf("  - %s\n", testResult.Reasoning))
		}
		for _, obs := range testResult.Observations {
			if !obs.Supports {
				output.WriteString(fmt.Sprintf("  - %s\n", obs.Description))
			}
		}
		output.WriteString("\n→ To refine: use quint_propose with refines=\"" + hypothesisID + "\" to create updated hypothesis\n")
		output.WriteString("  Example: quint_propose(title=\"...\", ..., refines=\"" + hypothesisID + "\")\n")
		output.WriteString("  Note: After refinement, re-verify (/q2-verify) before validation (/q3-validate)\n")
		return output.String(), nil

	default:
		return "", fmt.Errorf("overall_verdict must be PASS, FAIL, or REFINE, got: %s", verdict)
	}
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
	if verdict != "PASS" && verdict != "FAIL" && verdict != "REFINE" {
		return fmt.Errorf("overall_verdict must be PASS, FAIL, or REFINE, got: %s", r.OverallVerdict)
	}

	if r.Reasoning == "" {
		return fmt.Errorf("missing reasoning")
	}

	return nil
}

func (t *Tools) RefineLoopback(ctx context.Context, sourceLayer, parentID, insight, newTitle, newContent, scope string) (string, error) {
	defer t.RecordWork("RefineLoopback", time.Now())

	parentLevel := sourceLayer
	if parentLevel != "L0" && parentLevel != "L1" {
		return "", fmt.Errorf("loopback not applicable from layer %s (must be L0 or L1)", sourceLayer)
	}

	var decisionContext string
	if t.DB != nil {
		decisionContext = t.DB.GetDecisionContextForHolon(ctx, parentID)
		if decisionContext == "" {
			return "", fmt.Errorf("failed to get parent's decision context: parent %s has no decision context", parentID)
		}
	}

	if err := t.MoveHypothesis(parentID, parentLevel, "invalid"); err != nil {
		return "", fmt.Errorf("failed to move parent hypothesis to invalid: %v", err)
	}

	rationale := fmt.Sprintf(`{"source": "loopback", "parent_id": "%s", "insight": "%s"}`, parentID, insight)
	childPath, err := t.ProposeHypothesis(ctx, newTitle, newContent, scope, "system", rationale, decisionContext, nil, 3, "", "")
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
