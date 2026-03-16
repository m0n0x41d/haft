package fpf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/logger"
)

func (t *Tools) collectImplementationWarnings(ctx context.Context, drrID string, dependsOn []string) *ImplementationWarnings {
	warnings := &ImplementationWarnings{}

	holonsToCheck := append([]string{drrID}, dependsOn...)
	visited := make(map[string]bool)

	var checkHolon func(holonID string, depth int)
	checkHolon = func(holonID string, depth int) {
		if depth > 10 || visited[holonID] {
			return
		}
		visited[holonID] = true

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

func (t *Tools) formatImplementationWarnings(warnings *ImplementationWarnings) string {
	if !warnings.HasAny() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## ⚠️ WARNINGS\n\n")
	sb.WriteString("Review these issues before proceeding:\n\n")

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
	sb.WriteString("1. Re-run tests: `quint_test holon-id internal \"re-verification\" PASS`\n")
	sb.WriteString("2. Or proceed if the changes don't affect this decision\n\n")
	sb.WriteString("---\n")

	return sb.String()
}

func (t *Tools) Implement(ctx context.Context, drrID string) (string, error) {
	defer t.RecordWork("Implement", time.Now())

	logger.Info().Str("drr_id", drrID).Msg("Implement called")

	if t.DB == nil {
		logger.Error().Msg("Implement: database not initialized")
		return "", ErrDatabaseNotInitialized
	}
	if _, err := t.detectCodeChanges(ctx); err != nil {
		logger.Warn().Err(err).Msg("code change detection failed")
	}

	normalizedID := drrID
	if strings.HasPrefix(drrID, "DRR-") {
		parts := strings.SplitN(drrID, "-", 5)
		if len(parts) == 5 {
			normalizedID = parts[4]
		}
	}

	drr, err := t.loadDRRInfo(ctx, normalizedID)
	if err != nil {
		drr, err = t.loadDRRInfo(ctx, drrID)
		if err != nil {
			return "", err
		}
	}

	if drr.Contract == nil {
		return "", fmt.Errorf("DRR %s has no implementation contract - nothing to implement", drrID)
	}

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

	inherited := t.collectInheritedConstraints(ctx, drr.DependsOn, make(map[string]bool))

	dependsOnWithWinner := drr.DependsOn
	if drr.WinnerID != "" {
		dependsOnWithWinner = append([]string{drr.WinnerID}, dependsOnWithWinner...)
	}
	warnings := t.collectImplementationWarnings(ctx, normalizedID, dependsOnWithWinner)
	warningsText := t.formatImplementationWarnings(warnings)

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

	directive := t.formatImplementDirective(drr, inherited)

	if allWarnings.Len() > 0 {
		return allWarnings.String() + "\n" + directive, nil
	}
	return directive, nil
}

func (t *Tools) loadDRRInfo(ctx context.Context, drrID string) (*DRRInfo, error) {

	holon, err := t.DB.GetHolon(ctx, drrID)
	if err != nil {
		return nil, fmt.Errorf("DRR not found: %s", drrID)
	}

	if holon.Type != "DRR" && holon.Layer != "DRR" {
		return nil, fmt.Errorf("holon %s is not a DRR (type=%s, layer=%s)", drrID, holon.Type, holon.Layer)
	}

	contract, _ := t.getDRRContract(drrID)

	var dependsOn []string
	var winnerID string

	relations, err := t.DB.GetRelationsForHolon(ctx, drrID)
	if err == nil {
		for _, rel := range relations {
			if rel.RelationType == "selects" {
				winnerID = rel.TargetID
				dependsOn = append(dependsOn, rel.TargetID)
			} else if rel.RelationType == "componentOf" || rel.RelationType == "constituentOf" {
				dependsOn = append(dependsOn, rel.TargetID)
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

func (t *Tools) collectInheritedConstraints(ctx context.Context, depIDs []string, visited map[string]bool) InheritedConstraints {
	var result InheritedConstraints

	for _, depID := range depIDs {
		if visited[depID] {
			continue
		}
		visited[depID] = true

		dep, err := t.loadDRRInfo(ctx, depID)
		if err != nil || dep.Contract == nil {
			continue
		}

		laws := dep.Contract.GetLaws()
		if len(laws) > 0 {
			result.Invariants = append(result.Invariants, ConstraintSource{
				DRRID:       depID,
				DRRTitle:    dep.Title,
				Constraints: laws,
			})
		}

		admissibility := dep.Contract.GetAdmissibility()
		if len(admissibility) > 0 {
			result.AntiPatterns = append(result.AntiPatterns, ConstraintSource{
				DRRID:       depID,
				DRRTitle:    dep.Title,
				Constraints: admissibility,
			})
		}

		deeper := t.collectInheritedConstraints(ctx, dep.DependsOn, visited)
		result.Invariants = append(result.Invariants, deeper.Invariants...)
		result.AntiPatterns = append(result.AntiPatterns, deeper.AntiPatterns...)
	}

	return result
}

func (t *Tools) formatImplementDirective(drr *DRRInfo, inherited InheritedConstraints) string {
	var sb strings.Builder

	sb.WriteString("# IMPLEMENTATION DIRECTIVE\n\n")

	sb.WriteString("## Task\n\n")
	sb.WriteString(fmt.Sprintf("Implement: **%s**\n", drr.Title))
	sb.WriteString(fmt.Sprintf("Decision: %s\n", drr.ID))
	if len(drr.Contract.AffectedScope) > 0 {
		sb.WriteString(fmt.Sprintf("Scope: %s\n", strings.Join(drr.Contract.AffectedScope, ", ")))
	}
	sb.WriteString("\n")

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("Using your internal TODO/planning capabilities, implement this task.\n\n")
	sb.WriteString("If project context is insufficient, conduct preliminary investigation first.\n\n")

	sb.WriteString("## Boundary Norm Square (L/A/D/E)\n\n")
	sb.WriteString("The contract uses the FPF Boundary Norm Square (A.6.B) to classify constraints:\n\n")
	sb.WriteString("| Quadrant | Meaning | Adjudication |\n")
	sb.WriteString("|----------|---------|-------------|\n")
	sb.WriteString("| **L (Laws)** | Physical/logical constraints that CANNOT be violated | In-description: provable from spec |\n")
	sb.WriteString("| **A (Admissibility)** | What IS and IS NOT allowed (anti-patterns, gates) | In-work: runtime/operational |\n")
	sb.WriteString("| **D (Deontics)** | What SHOULD happen (obligations, acceptance criteria) | In-description: stated duties |\n")
	sb.WriteString("| **E (Evidence)** | How we VERIFY compliance (test strategy, observables) | In-work: carriers/traces |\n")
	sb.WriteString("\n")

	laws := drr.Contract.GetLaws()
	if len(laws) > 0 || len(inherited.Invariants) > 0 {
		sb.WriteString("## L: Laws & Definitions\n\n")
		sb.WriteString("*Truth-conditional constraints adjudicated in-description. If you can write code that violates it, it's not a Law — move it to Admissibility.*\n\n")
		sb.WriteString("These MUST be true in your implementation:\n\n")

		if len(laws) > 0 {
			sb.WriteString("### This decision:\n")
			for i, inv := range laws {
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

	admissibility := drr.Contract.GetAdmissibility()
	if len(admissibility) > 0 || len(inherited.AntiPatterns) > 0 {
		sb.WriteString("## A: Admissibility & Gates\n\n")
		sb.WriteString("*Boundaries of the solution space. Anti-patterns go here — things that ARE possible but NOT allowed.*\n\n")
		sb.WriteString("Your LAST todo items must verify these constraints were NOT violated:\n\n")

		if len(admissibility) > 0 {
			sb.WriteString("### This decision:\n")
			for _, ap := range admissibility {
				sb.WriteString(fmt.Sprintf("- [ ] NOT: %s\n", ap))
			}
			sb.WriteString("\n")
		}

		for _, src := range inherited.AntiPatterns {
			sb.WriteString(fmt.Sprintf("### Inherited from %s:\n", src.DRRID))
			for _, ap := range src.Constraints {
				sb.WriteString(fmt.Sprintf("- [ ] NOT: %s\n", ap))
			}
			sb.WriteString("\n")
		}
	}

	deontics := drr.Contract.GetDeontics()
	if len(deontics) > 0 {
		sb.WriteString("## D: Deontics & Commitments\n\n")
		sb.WriteString("*Obligations and recommendations. Acceptance criteria — what SHOULD happen for success.*\n\n")
		sb.WriteString("Before calling quint_resolve, verify:\n\n")
		for _, ac := range deontics {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", ac))
		}
		sb.WriteString("\n")
	}

	evidence := drr.Contract.GetEvidence()
	if len(evidence) > 0 {
		sb.WriteString("## E: Evidence & Verification\n\n")
		sb.WriteString("*How to verify compliance. Test strategies, observables, metrics, carrier classes.*\n\n")
		sb.WriteString("Verification approach:\n\n")
		for _, ev := range evidence {
			sb.WriteString(fmt.Sprintf("- %s\n", ev))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("When complete: `quint_resolve %s implemented criteria_verified=true`\n", drr.ID))

	logger.Info().Str("drr_id", drr.ID).Msg("Implement: directive generated")

	return sb.String()
}
