// Package present contains pure presentation functions for formatting
// artifact data as MCP tool responses. No side effects, no store access.
// Depends on artifact package for domain types only.
package present

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// NavStrip renders the nav state as a compact text block.
func NavStrip(state artifact.NavState) string {
	var sb strings.Builder

	sb.WriteString("\n── Haft ───────────────────────────\n")

	if state.Context != "" {
		sb.WriteString(fmt.Sprintf("Context: %s\n", state.Context))
	}
	if state.Mode != "" {
		sb.WriteString(fmt.Sprintf("Mode: %s\n", state.Mode))
	}

	sb.WriteString(fmt.Sprintf("Status: %s\n", state.DerivedStatus))

	if state.ProblemTitle != "" {
		sb.WriteString(fmt.Sprintf("Problem: %s", state.ProblemTitle))
		if state.ProblemStatus != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", state.ProblemStatus))
		}
		sb.WriteString("\n")
	}
	if state.PortfolioInfo != "" {
		sb.WriteString(fmt.Sprintf("Portfolio: %s\n", state.PortfolioInfo))
	}
	if state.DecisionInfo != "" {
		sb.WriteString(fmt.Sprintf("Decision: %s\n", state.DecisionInfo))
	}

	if state.StaleCount > 0 {
		sb.WriteString(fmt.Sprintf("Stale: %d decision(s) need refresh\n", state.StaleCount))
	}

	if state.NextAction != "" {
		sb.WriteString(fmt.Sprintf("Available: %s\n", state.NextAction))
		sb.WriteString("↑ Present to user — do not auto-execute.\n")
	}

	sb.WriteString("───────────────────────────────────\n")

	return sb.String()
}

// NoteResponse builds the MCP tool response for a note.
func NoteResponse(a *artifact.Artifact, filePath string, validation artifact.NoteValidation, navStrip string) string {
	var sb strings.Builder

	if len(validation.Warnings) > 0 && validation.OK {
		for _, w := range validation.Warnings {
			sb.WriteString(fmt.Sprintf("⚠ %s\n", w))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Recorded: %s\n", formatArtifactLabel(a.Meta.Title, a.Meta.ID)))
	if filePath != "" {
		sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
	}

	sb.WriteString(navStrip)
	return sb.String()
}

// NoteRejection builds the response when a note is rejected.
func NoteRejection(validation artifact.NoteValidation, navStrip string) string {
	var sb strings.Builder

	for _, w := range validation.Warnings {
		sb.WriteString(fmt.Sprintf("⚠ %s\n", w))
	}

	if len(validation.Conflicts) > 0 {
		sb.WriteString("\nConflicting decisions:\n")
		for _, c := range validation.Conflicts {
			sb.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", c.DecisionID, c.DecisionTitle, c.Reason))
		}
	}

	sb.WriteString("\nOptions:\n")
	if validation.Suggest != "" {
		sb.WriteString(fmt.Sprintf("  1. Use %s to start a proper exploration\n", validation.Suggest))
		sb.WriteString("  2. Add rationale and retry\n")
	} else {
		sb.WriteString("  1. Add rationale explaining why this choice\n")
		sb.WriteString("  2. Provide evidence supporting the decision\n")
	}

	sb.WriteString(navStrip)
	return sb.String()
}

// ReconcileResponse formats the reconcile results.
func ReconcileResponse(overlaps []artifact.ReconcileOverlap, navStrip string) string {
	var sb strings.Builder

	if len(overlaps) == 0 {
		sb.WriteString("No note-decision overlaps found. Notes and decisions are clean.\n")
		sb.WriteString(navStrip)
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("## Note-Decision Overlaps (%d found)\n\n", len(overlaps)))
	for _, o := range overlaps {
		action := "consider deprecating"
		if o.Similarity > 0.7 {
			action = "should deprecate"
		}
		sb.WriteString(fmt.Sprintf("- **%s** [%s] overlaps with **%s** [%s] (%.0f%% overlap) — %s\n",
			o.NoteTitle, o.NoteID, o.DecisionTitle, o.DecisionID, o.Similarity*100, action))
	}
	sb.WriteString("\nUse `haft_refresh(action=\"deprecate\", artifact_ref=\"<note-id>\", reason=\"superseded by decision\")` to clean up.\n")
	sb.WriteString(navStrip)
	return sb.String()
}

// ScanResponse formats the stale scan results.
func ScanResponse(items []artifact.StaleItem, navStrip string) string {
	var sb strings.Builder

	if len(items) == 0 {
		sb.WriteString("No stale artifacts found. All decisions, problems, and notes are current.\n")
		sb.WriteString(navStrip)
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("## Refresh Due (%d artifact(s))\n\n", len(items)))
	for i, item := range items {
		kindLabel := item.Kind
		if kindLabel == "" {
			kindLabel = "DecisionRecord"
		}
		kindLabel = UserFacingArtifactKindLabel(kindLabel)
		sb.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, formatArtifactLabel(item.Title, item.ID), kindLabel))
		sb.WriteString(fmt.Sprintf("   Reason: %s\n\n", item.Reason))
	}

	sb.WriteString("**Actions** (work on any artifact type):\n")
	sb.WriteString("- `waive` — extend validity with justification\n")
	sb.WriteString("- `reopen` — start new problem cycle (decisions only)\n")
	sb.WriteString("- `supersede` — replace with another artifact\n")
	sb.WriteString("- `deprecate` — archive as no longer relevant\n")
	sb.WriteString("\nUse `artifact_ref` parameter with any artifact ID (note, problem, decision, portfolio).\n")

	sb.WriteString(navStrip)
	return sb.String()
}

// ScanResponseSummary formats stale scan results for default agent output.
// The full stale list remains available through ScanResponse when the caller
// explicitly asks for verbose refresh output.
func ScanResponseSummary(items []artifact.StaleItem, navStrip string) string {
	var sb strings.Builder

	if len(items) == 0 {
		sb.WriteString("No stale artifacts found. All decisions, problems, and notes are current.\n")
		sb.WriteString(navStrip)
		return sb.String()
	}

	const topStaleItems = 10
	sb.WriteString(fmt.Sprintf("## Refresh Due (%d artifact(s)) — summary\n\n", len(items)))
	for i, item := range items {
		if i >= topStaleItems {
			break
		}
		kindLabel := item.Kind
		if kindLabel == "" {
			kindLabel = "DecisionRecord"
		}
		kindLabel = UserFacingArtifactKindLabel(kindLabel)
		sb.WriteString(fmt.Sprintf("%d. %s (%s)\n", i+1, formatArtifactLabel(item.Title, item.ID), kindLabel))
		sb.WriteString(fmt.Sprintf("   Reason: %s\n\n", item.Reason))
	}
	if omitted := len(items) - topStaleItems; omitted > 0 {
		sb.WriteString(fmt.Sprintf("... and %d more refresh-due artifact(s) omitted from summary\n\n", omitted))
	}

	sb.WriteString("Full stale list: pass `verbose: true` to haft_refresh(action=\"scan\").\n")
	sb.WriteString("Actions: waive | reopen | supersede | deprecate with `artifact_ref`.\n")

	sb.WriteString(navStrip)
	return sb.String()
}

func GovernanceAttentionResponse(attention artifact.GovernanceAttention) string {
	if attention.BacklogCount == 0 &&
		attention.InProgressCount == 0 &&
		len(attention.AddressedWithoutDecision) == 0 &&
		len(attention.InvariantViolations) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Governance State\n\n")
	sb.WriteString(fmt.Sprintf("Problems: %d backlog, %d in progress\n", attention.BacklogCount, attention.InProgressCount))

	if len(attention.AddressedWithoutDecision) > 0 {
		sb.WriteString(fmt.Sprintf("\nAddressed without linked decision (%d)\n", len(attention.AddressedWithoutDecision)))
		for _, gap := range attention.AddressedWithoutDecision {
			sb.WriteString(fmt.Sprintf("- **%s** `%s`\n", gap.Title, gap.ProblemID))
		}
	}

	if len(attention.InvariantViolations) > 0 {
		sb.WriteString(fmt.Sprintf("\nInvariant violations (%d)\n", len(attention.InvariantViolations)))
		for _, violation := range attention.InvariantViolations {
			sb.WriteString(fmt.Sprintf("- **%s** `%s` — %s\n", violation.DecisionTitle, violation.DecisionID, violation.Invariant))
			if strings.TrimSpace(violation.Reason) != "" {
				sb.WriteString(fmt.Sprintf("  Reason: %s\n", strings.TrimSpace(violation.Reason)))
			}
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// RefreshActionResponse formats the result of a refresh action.
func RefreshActionResponse(action artifact.RefreshAction, dec *artifact.Artifact, newProb *artifact.Artifact, navStrip string) string {
	var sb strings.Builder

	switch action {
	case artifact.RefreshWaive:
		sb.WriteString(fmt.Sprintf("Waived: %s\n", dec.Meta.Title))
		sb.WriteString(fmt.Sprintf("New valid_until: %s\n", dec.Meta.ValidUntil))
	case artifact.RefreshReopen:
		sb.WriteString(fmt.Sprintf("Reopened: %s → status: refresh_due\n", dec.Meta.Title))
		if newProb != nil {
			sb.WriteString(fmt.Sprintf("New problem: %s (%s)\n", newProb.Meta.Title, newProb.Meta.ID))
			sb.WriteString("Use /h-explore to find new solutions.\n")
		}
	case artifact.RefreshSupersede:
		sb.WriteString(fmt.Sprintf("Superseded: %s\n", dec.Meta.Title))
	case artifact.RefreshDeprecate:
		sb.WriteString(fmt.Sprintf("Deprecated: %s\n", dec.Meta.Title))
	}

	sb.WriteString(navStrip)
	return sb.String()
}

// BaselineResponse formats the result of a baseline action.
func BaselineResponse(decisionTitle string, decisionRef string, files []artifact.AffectedFile, navStrip string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Baseline set for %s. Monitoring %d file(s).\n\n", formatArtifactLabel(decisionTitle, decisionRef), len(files)))
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("  %s — %s\n", f.Path, f.Hash[:12]))
	}
	sb.WriteString(navStrip)
	return sb.String()
}

// symbolVerdictHint renders the operator-facing action implied by a report's
// symbol-level triage verdict — the deterministic floor for session-start drift.
func symbolVerdictHint(verdict string) string {
	switch verdict {
	case artifact.SymbolVerdictAdditiveOnly:
		return " — additive only; safe to re-baseline without review"
	case artifact.SymbolVerdictGovernedModified:
		return " — a governed symbol body changed; surface to the operator"
	default:
		return " — could not prove benign; surface to the operator"
	}
}

// DriftResponseSummary formats drift results compactly: per-report counts +
// up to 5 modified file paths (the actionable ones). Suitable as the default
// reply from haft_refresh scan; the full per-file dump (DriftResponse) can
// span thousands of lines on repos with vendor subtrees or large added-files
// sets and overflows the agent's context. Verbose mode opt-in only.
func DriftResponseSummary(reports []artifact.DriftReport, navStrip string) string {
	var sb strings.Builder

	if len(reports) == 0 {
		sb.WriteString("No drift detected. All baselined decisions match current file state.\n")
		sb.WriteString(navStrip)
		return sb.String()
	}

	driftCount := 0
	noBaselineCount := 0
	for _, r := range reports {
		if r.HasBaseline {
			driftCount++
		} else {
			noBaselineCount++
		}
	}

	const topModifiedPerReport = 5
	const topDriftReports = 5
	const topImpactReports = 3
	const topImpactedModulesPerReport = 5
	const topDecisionIDsPerImpact = 4

	if driftCount > 0 {
		sb.WriteString(fmt.Sprintf("## Drift Detected (%d decision(s)) — summary\n\n", driftCount))
		sb.WriteString("Counts per baselined decision. For full per-file dump pass `verbose: true` to haft_refresh.\n\n")
		renderedDriftReports := 0
		omittedDriftReports := 0
		for _, r := range reports {
			if !r.HasBaseline {
				continue
			}
			if renderedDriftReports >= topDriftReports {
				omittedDriftReports++
				continue
			}
			renderedDriftReports++

			var modified, added, missing int
			var modifiedPaths []string
			var changedSymbols []string // non-added symbols across modified files — the actionable ones
			for _, f := range r.Files {
				switch f.Status {
				case artifact.DriftModified:
					modified++
					if len(modifiedPaths) < topModifiedPerReport {
						modifiedPaths = append(modifiedPaths, f.Path)
					}
					for _, s := range f.Symbols {
						if s.Status != "added" && len(changedSymbols) < topModifiedPerReport {
							changedSymbols = append(changedSymbols, fmt.Sprintf("%s %s %s", s.Status, s.SymbolKind, s.SymbolName))
						}
					}
				case artifact.DriftAdded:
					added++
				case artifact.DriftMissing:
					missing++
				}
			}
			sb.WriteString(fmt.Sprintf("### %s\n", formatArtifactLabel(r.DecisionTitle, r.DecisionID)))
			sb.WriteString(fmt.Sprintf("  %d modified, %d added, %d missing\n", modified, added, missing))
			sb.WriteString(fmt.Sprintf("  Symbol verdict: %s%s\n", r.SymbolVerdict(), symbolVerdictHint(r.SymbolVerdict())))
			if len(changedSymbols) > 0 {
				sb.WriteString("  Governed symbols changed:\n")
				for _, s := range changedSymbols {
					sb.WriteString(fmt.Sprintf("    ~ %s\n", s))
				}
			}
			if len(modifiedPaths) > 0 {
				sb.WriteString("  Top modified:\n")
				for _, p := range modifiedPaths {
					sb.WriteString(fmt.Sprintf("    - %s\n", p))
				}
				if modified > topModifiedPerReport {
					sb.WriteString(fmt.Sprintf("    ... and %d more modified\n", modified-topModifiedPerReport))
				}
			}
			sb.WriteString("\n")
		}
		if omittedDriftReports > 0 {
			sb.WriteString(fmt.Sprintf("... and %d more drifted decision(s) omitted from summary\n\n", omittedDriftReports))
		}
		impactReports := 0
		omittedImpactReports := 0
		for _, r := range reports {
			if !r.HasBaseline || len(r.ImpactedModules) == 0 {
				continue
			}
			if impactReports >= topImpactReports {
				omittedImpactReports++
				continue
			}
			impactReports++

			sb.WriteString(fmt.Sprintf("**Impact propagation for %s:**\n", formatArtifactLabel(r.DecisionTitle, r.DecisionID)))
			for i, impact := range r.ImpactedModules {
				if i >= topImpactedModulesPerReport {
					break
				}
				if impact.IsBlind {
					sb.WriteString(fmt.Sprintf("  ⚠ %s (blind) — no decisions, potential unmonitored impact\n", impact.ModulePath))
				} else {
					sb.WriteString(fmt.Sprintf("  → %s — governed by %s\n", impact.ModulePath, summarizedDecisionLabels(impact.DecisionIDs, impact.DecisionTitles, topDecisionIDsPerImpact)))
				}
			}
			if omitted := len(r.ImpactedModules) - topImpactedModulesPerReport; omitted > 0 {
				sb.WriteString(fmt.Sprintf("  ... and %d more impacted module(s)\n", omitted))
			}
			sb.WriteString("\n")
		}
		if omittedImpactReports > 0 {
			sb.WriteString(fmt.Sprintf("... and %d more decision(s) with impact propagation omitted from summary\n\n", omittedImpactReports))
		}
		sb.WriteString("**Classify each:** cosmetic (re-baseline) | material (flag to user or reopen) | incidental (shared file changed by unrelated work — re-baseline)\n\n")
		sb.WriteString("For one specific decision use `haft_query(action=\"related\", file=...)`; for full dump pass `verbose: true` to haft_refresh.\n\n")
	}

	if noBaselineCount > 0 {
		sb.WriteString(fmt.Sprintf("## No Baseline (%d decision(s))\n\n", noBaselineCount))
		for _, r := range reports {
			if r.HasBaseline {
				continue
			}
			gitHint := "no git activity detected after decision date"
			if r.LikelyImplemented {
				gitHint = "git activity detected after decision date"
			}
			sb.WriteString(fmt.Sprintf("- %s — %d file(s) unmonitored, %s\n",
				formatArtifactLabel(r.DecisionTitle, r.DecisionID), len(r.Files), gitHint))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(navStrip)
	return sb.String()
}

func summarizedDecisionLabels(ids []string, titles map[string]string, limit int) string {
	if len(ids) == 0 {
		return ""
	}
	if limit <= 0 || limit > len(ids) {
		limit = len(ids)
	}

	shown := make([]string, 0, limit+1)
	for _, id := range ids[:limit] {
		shown = append(shown, formatArtifactLabel(titles[id], id))
	}
	if len(ids) > limit {
		shown = append(shown, fmt.Sprintf("... +%d", len(ids)-limit))
	}
	return strings.Join(shown, ", ")
}

// DriftResponse formats drift check results for the agent (verbose: full
// per-file dump). Opt-in via verbose=true on haft_refresh; default callers
// should prefer DriftResponseSummary to keep output within an agent's
// context budget.
func DriftResponse(reports []artifact.DriftReport, navStrip string) string {
	var sb strings.Builder

	if len(reports) == 0 {
		sb.WriteString("No drift detected. All baselined decisions match current file state.\n")
		sb.WriteString(navStrip)
		return sb.String()
	}

	driftCount := 0
	noBaselineCount := 0
	for _, r := range reports {
		if r.HasBaseline {
			driftCount++
		} else {
			noBaselineCount++
		}
	}

	if driftCount > 0 {
		sb.WriteString(fmt.Sprintf("## Drift Detected (%d decision(s))\n\n", driftCount))
		sb.WriteString("⚠ REQUIRED: For each decision below, read `git diff` on modified files before taking action.\n")
		sb.WriteString("Do not summarize drift as \"expected\" without reading the diffs — that is treating description as evidence.\n\n")
		for _, r := range reports {
			if !r.HasBaseline {
				continue
			}
			sb.WriteString(fmt.Sprintf("### %s\n\n", formatArtifactLabel(r.DecisionTitle, r.DecisionID)))
			for _, f := range r.Files {
				switch f.Status {
				case artifact.DriftModified:
					sb.WriteString(fmt.Sprintf("  **MODIFIED** %s %s\n", f.Path, f.LinesChanged))
				case artifact.DriftAdded:
					sb.WriteString(fmt.Sprintf("  **ADDED** %s\n", f.Path))
				case artifact.DriftMissing:
					sb.WriteString(fmt.Sprintf("  **FILE MISSING** %s\n", f.Path))
				}
			}
			sb.WriteString("\n")
		}
		for _, r := range reports {
			if !r.HasBaseline || len(r.ImpactedModules) == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("**Impact propagation for %s:**\n", formatArtifactLabel(r.DecisionTitle, r.DecisionID)))
			for _, impact := range r.ImpactedModules {
				if impact.IsBlind {
					sb.WriteString(fmt.Sprintf("  ⚠ %s (blind) — no decisions, potential unmonitored impact\n", impact.ModulePath))
				} else {
					sb.WriteString(fmt.Sprintf("  → %s — governed by %s\n", impact.ModulePath, summarizedDecisionLabels(impact.DecisionIDs, impact.DecisionTitles, len(impact.DecisionIDs))))
				}
			}
			sb.WriteString("\n")
		}

		sb.WriteString("**Classify each:** cosmetic (re-baseline) | material (flag to user or reopen) | incidental (shared file changed by unrelated work — re-baseline)\n\n")
	}

	if noBaselineCount > 0 {
		sb.WriteString(fmt.Sprintf("## No Baseline (%d decision(s))\n\n", noBaselineCount))
		for _, r := range reports {
			if r.HasBaseline {
				continue
			}
			gitHint := "no git activity detected after decision date"
			if r.LikelyImplemented {
				gitHint = "git activity detected after decision date"
			}
			sb.WriteString(fmt.Sprintf("- %s — %d file(s) unmonitored, %s\n",
				formatArtifactLabel(r.DecisionTitle, r.DecisionID), len(r.Files), gitHint))
		}
		sb.WriteString("\n**Action:** Verify implementation status by reading affected files before baselining.\n\n")
	}

	sb.WriteString(navStrip)
	return sb.String()
}

// DecisionResponse builds the MCP tool response.
func DecisionResponse(action string, a *artifact.Artifact, filePath string, extra string, navStrip string) string {
	var sb strings.Builder

	switch action {
	case "decide":
		sb.WriteString(fmt.Sprintf("Decision recorded: %s\n", formatArtifactLabel(a.Meta.Title, a.Meta.ID)))
		if a.Meta.ValidUntil != "" {
			sb.WriteString(fmt.Sprintf("Valid until: %s\n", a.Meta.ValidUntil))
		}
		if filePath != "" {
			sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
		}
		sb.WriteString("\n---\n\n")
		sb.WriteString(a.Body)
	case "apply":
		sb.WriteString(extra)
	case "measure":
		sb.WriteString(fmt.Sprintf("Impact measured: %s\n", formatArtifactLabel(a.Meta.Title, a.Meta.ID)))
		sb.WriteString(extra)
	case "evidence":
		sb.WriteString(extra)
	}

	sb.WriteString(navStrip)
	return sb.String()
}

// SolutionResponse builds the MCP tool response.
func SolutionResponse(action string, a *artifact.Artifact, filePath string, navStrip string) string {
	var sb strings.Builder

	switch action {
	case "explore":
		sb.WriteString(fmt.Sprintf("Portfolio created: %s\n", formatArtifactLabel(a.Meta.Title, a.Meta.ID)))
		if filePath != "" {
			sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
		}
		sb.WriteString(formatVariantsIndex(a))
	case "compare":
		sb.WriteString(fmt.Sprintf("Comparison added to: %s\n", formatArtifactLabel(a.Meta.Title, a.Meta.ID)))
		if filePath != "" {
			sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
		}
		summary := ComparisonSummary(a)
		if summary != "" {
			sb.WriteString(summary)
		}
	}

	sb.WriteString(navStrip)
	return sb.String()
}

// formatVariantsIndex renders the canonical variant id -> title mapping that
// callers need to drive `haft_solution(action="compare")` without scraping the
// rendered markdown body. Closes the discoverability gap from issue #71.
//
// Surface form:
//
//	Variants:
//	  V1 — First variant title
//	  V2 — Second variant title
//
//	Use these IDs verbatim as keys in `scores`, `dominated_variants[].variant`,
//	`pareto_tradeoffs[].variant`, `non_dominated_set`, and `selected_ref` when
//	calling `haft_solution(action="compare")`.
func formatVariantsIndex(a *artifact.Artifact) string {
	if a == nil {
		return ""
	}
	fields := a.UnmarshalPortfolioFields()
	variants := artifact.MaterializeVariantIDs(fields.Variants)
	if len(variants) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nVariants:\n")
	for _, variant := range variants {
		id := strings.TrimSpace(variant.ID)
		if id == "" {
			continue
		}
		title := strings.TrimSpace(variant.Title)
		if title == "" {
			sb.WriteString(fmt.Sprintf("  %s\n", id))
		} else {
			sb.WriteString(fmt.Sprintf("  %s — %s\n", id, title))
		}
	}
	sb.WriteString("\nUse these IDs verbatim as keys in `scores`, `dominated_variants[].variant`,\n")
	sb.WriteString("`pareto_tradeoffs[].variant`, `non_dominated_set`, and `selected_ref` when\n")
	sb.WriteString("calling `haft_solution(action=\"compare\")`.\n")
	return sb.String()
}

// ComparisonSummary builds a user-facing summary for a compared portfolio.
func ComparisonSummary(a *artifact.Artifact) string {
	if a == nil {
		return ""
	}

	fields := a.UnmarshalPortfolioFields()
	if fields.Comparison != nil {
		return structuredComparisonSummary(*fields.Comparison, solutionVariantLabels(fields.Variants))
	}

	return legacyComparisonSummary(a.Body)
}

func structuredComparisonSummary(result artifact.ComparisonResult, labels map[string]string) string {
	lines := make([]string, 0, 8)
	paretoFront := strings.Join(displayComparisonVariantLabels(result.NonDominatedSet, labels), ", ")
	if paretoFront != "" {
		lines = append(lines, fmt.Sprintf("Computed Pareto front: %s", paretoFront))
	}

	if len(result.DominatedVariants) > 0 {
		lines = append(lines, "Dominated variant elimination:")
		for _, note := range result.DominatedVariants {
			variantLabel := displayComparisonVariantLabel(note.Variant, labels)
			summary := strings.TrimSpace(note.Summary)
			dominatedBy := strings.Join(displayComparisonVariantLabels(note.DominatedBy, labels), ", ")
			switch {
			case dominatedBy != "":
				lines = append(lines, fmt.Sprintf("- %s: dominated by %s. %s", variantLabel, dominatedBy, summary))
			default:
				lines = append(lines, fmt.Sprintf("- %s: %s", variantLabel, summary))
			}
		}
	}

	if len(result.ParetoTradeoffs) > 0 {
		lines = append(lines, "Pareto-front trade-offs:")
		for _, note := range result.ParetoTradeoffs {
			variantLabel := displayComparisonVariantLabel(note.Variant, labels)
			lines = append(lines, fmt.Sprintf("- %s: %s", variantLabel, strings.TrimSpace(note.Summary)))
		}
	}

	if strings.TrimSpace(result.PolicyApplied) != "" {
		lines = append(lines, fmt.Sprintf("Selection policy: %s", strings.TrimSpace(result.PolicyApplied)))
	}

	if strings.TrimSpace(result.SelectedRef) != "" {
		lines = append(lines, fmt.Sprintf("Recommendation (advisory): %s", displayComparisonVariantLabel(result.SelectedRef, labels)))
		if strings.TrimSpace(result.RecommendationRationale) != "" {
			lines = append(lines, fmt.Sprintf("Recommendation rationale: %s", strings.TrimSpace(result.RecommendationRationale)))
		}
		lines = append(lines, "Human choice remains open until decide.")
	}

	if len(lines) == 0 {
		return ""
	}

	return strings.Join(lines, "\n") + "\n"
}

func legacyComparisonSummary(body string) string {
	lines := make([]string, 0, 2)
	markers := []struct {
		Needle string
		Label  string
	}{
		{Needle: "**Computed Pareto front:**", Label: "Computed Pareto front:"},
		{Needle: "**Pareto front:**", Label: "Pareto front:"},
		{Needle: "**Recommendation (advisory):**", Label: "Recommendation (advisory):"},
		{Needle: "**Recommended:**", Label: "Recommendation (advisory):"},
	}

	for _, marker := range markers {
		idx := strings.Index(body, marker.Needle)
		if idx == -1 {
			continue
		}

		end := strings.Index(body[idx:], "\n")
		if end <= 0 {
			continue
		}

		value := strings.TrimSpace(strings.TrimPrefix(body[idx:idx+end], marker.Needle))
		if value == "" {
			continue
		}

		lines = append(lines, fmt.Sprintf("%s %s", marker.Label, value))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func solutionVariantLabels(variants []artifact.Variant) map[string]string {
	labels := make(map[string]string, len(variants))
	for _, variant := range variants {
		title := strings.TrimSpace(variant.Title)
		id := strings.TrimSpace(variant.ID)
		if id != "" && title != "" {
			labels[id] = fmt.Sprintf("%s `%s`", title, id)
		}
		if title != "" {
			labels[title] = title
		}
	}
	return labels
}

func displayComparisonVariantLabels(values []string, labels map[string]string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, displayComparisonVariantLabel(value, labels))
	}
	return result
}

func displayComparisonVariantLabel(value string, labels map[string]string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if label, ok := labels[trimmed]; ok {
		return label
	}
	return trimmed
}

// MissingProblemResponse returns prescriptive guidance when problem is missing.
func MissingProblemResponse(navStrip string) string {
	return "No active problem found.\n\n" +
		"Frame the problem first:\n" +
		"  /h-frame — define what's anomalous, constraints, acceptance criteria\n\n" +
		"Or explore directly in tactical mode:\n" +
		"  haft_solution(action=\"explore\", variants=[...])\n" +
		"  → will create a lightweight problem from context\n" +
		navStrip
}

// ProblemResponse builds the MCP tool response for a framed problem.
func ProblemResponse(action string, a *artifact.Artifact, filePath string, navStrip string) string {
	var sb strings.Builder

	switch action {
	case "frame":
		sb.WriteString(fmt.Sprintf("Problem framed: %s\n", formatArtifactLabel(a.Meta.Title, a.Meta.ID)))
		sb.WriteString(fmt.Sprintf("Mode: %s\n", a.Meta.Mode))
		if problemType := artifact.ProblemTypeLabel(a); problemType != "" {
			sb.WriteString(fmt.Sprintf("Type: %s\n", problemType))
		}
		if filePath != "" {
			sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
		}
		if a.Meta.Mode == artifact.ModeStandard || a.Meta.Mode == artifact.ModeDeep {
			sb.WriteString("\nValidate this signal with evidence before exploring. Run tests, check metrics, research data.\n")
			sb.WriteString(fmt.Sprintf("  haft_decision(action=\"evidence\", artifact_ref=\"%s\", evidence_content=\"...\", evidence_type=\"measurement\", evidence_verdict=\"supports\")\n", a.Meta.ID))
		}
		if strings.Contains(a.Body, "## Related History") {
			idx := strings.Index(a.Body, "## Related History")
			sb.WriteString("\n" + a.Body[idx:])
		}
	case "characterize":
		sb.WriteString(fmt.Sprintf("Characterization added to: %s\n", formatArtifactLabel(a.Meta.Title, a.Meta.ID)))
	}

	sb.WriteString(navStrip)
	return sb.String()
}

// SearchResponse formats FTS5 search results as markdown.
func SearchResponse(results []*artifact.Artifact, query string) string {
	if len(results) == 0 {
		return fmt.Sprintf("No results found for: %s\n", query)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Search: %s (%d results)\n\n", query, len(results)))

	for i, a := range results {
		kindLabel := UserFacingArtifactKindLabel(string(a.Meta.Kind))
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s] `%s`\n", i+1, a.Meta.Title, kindLabel, a.Meta.ID))
		if a.Meta.Context != "" {
			sb.WriteString(fmt.Sprintf("   Context: %s", a.Meta.Context))
		}
		if a.Meta.Status != artifact.StatusActive {
			sb.WriteString(fmt.Sprintf(" | Status: %s", a.Meta.Status))
		}
		sb.WriteString(fmt.Sprintf(" | %s\n", a.Meta.CreatedAt.Format("2006-01-02")))

		// Show first 120 chars of body as preview
		preview := strings.TrimSpace(a.Body)
		if idx := strings.Index(preview, "\n"); idx > 0 {
			preview = strings.TrimSpace(preview[idx:])
		}
		if len(preview) > 120 {
			preview = preview[:117] + "..."
		}
		if preview != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", preview))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// StatusResponse formats the status dashboard from pre-fetched data.
func StatusResponse(data artifact.StatusData) string {
	var sb strings.Builder
	sb.WriteString("## Haft Status\n\n")

	formatDecisionList := func(items []*artifact.Artifact, cap int) {
		for i, d := range items {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(items)-cap))
				break
			}
			line := "- " + formatArtifactLabel(d.Meta.Title, d.Meta.ID)
			if d.Meta.ValidUntil != "" {
				vu := d.Meta.ValidUntil
				if len(vu) > 10 {
					vu = vu[:10]
				}
				line += fmt.Sprintf(" (valid until %s)", vu)
			}
			sb.WriteString(line + "\n")
		}
	}

	if len(data.HealthyDecisions) > 0 {
		sb.WriteString(fmt.Sprintf("### Shipped / Healthy (%d)\n\n", len(data.HealthyDecisions)))
		formatDecisionList(data.HealthyDecisions, 5)
		sb.WriteString("\n")
	}

	if len(data.PendingDecisions) > 0 {
		sb.WriteString(fmt.Sprintf("### Pending (%d)\n\n", len(data.PendingDecisions)))
		formatDecisionList(data.PendingDecisions, 5)
		sb.WriteString("\n")
	}

	if len(data.UnassessedDecisions) > 0 {
		sb.WriteString(fmt.Sprintf("### Unassessed (%d)\n\n", len(data.UnassessedDecisions)))
		formatDecisionList(data.UnassessedDecisions, 5)
		sb.WriteString("\n")
	}

	if len(data.StaleItems) > 0 {
		sb.WriteString(fmt.Sprintf("### Refresh Due (%d)\n\n", len(data.StaleItems)))
		cap := 5
		for i, s := range data.StaleItems {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more (use /h-refresh to see all)\n", len(data.StaleItems)-cap))
				break
			}
			line := "- " + formatArtifactLabel(s.Title, s.ID)
			if health, ok := data.DecisionHealth[s.ID]; ok {
				line += fmt.Sprintf(" — %s", health.Label())
			}
			line += fmt.Sprintf(" — %s", s.Reason)
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	// Drift section (H1 of V2 — dec-20260526-9fdd33ed). Summary-mode
	// against noise overload per the DRR weakest_link mitigation:
	// top-3 drifted decisions with counts; full detail via /h-refresh
	// scan or /h-verify.
	if len(data.Drift) > 0 {
		sb.WriteString(fmt.Sprintf("### Drift Detected (%d decision(s))\n\n", len(data.Drift)))
		cap := 3
		for i, r := range data.Drift {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more (use /h-refresh scan or /h-verify for details)\n", len(data.Drift)-cap))
				break
			}
			mod, added, missing := 0, 0, 0
			for _, f := range r.Files {
				switch f.Status {
				case artifact.DriftModified:
					mod++
				case artifact.DriftAdded:
					added++
				case artifact.DriftMissing:
					missing++
				}
			}
			parts := []string{}
			if mod > 0 {
				parts = append(parts, fmt.Sprintf("%d modified", mod))
			}
			if added > 0 {
				parts = append(parts, fmt.Sprintf("%d added", added))
			}
			if missing > 0 {
				parts = append(parts, fmt.Sprintf("%d missing", missing))
			}
			summary := strings.Join(parts, ", ")
			if summary == "" {
				summary = "no file changes"
			}
			sb.WriteString(fmt.Sprintf("- %s — %s\n", formatArtifactLabel(r.DecisionTitle, r.DecisionID), summary))
		}
		sb.WriteString("\n→ Run /h-verify on a drifted decision to gather evidence; /h-refresh scan for full file-level diff.\n\n")
	}

	if len(data.CommissionAttention) > 0 {
		sb.WriteString(fmt.Sprintf("### WorkCommissions Need Attention (%d)\n\n", len(data.CommissionAttention)))
		cap := 5
		for i, commission := range data.CommissionAttention {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(data.CommissionAttention)-cap))
				break
			}
			sb.WriteString(formatCommissionStatusEntry(commission) + "\n")
		}
		sb.WriteString("\n")
	} else if len(data.OpenCommissions) > 0 {
		sb.WriteString(fmt.Sprintf("### Open WorkCommissions (%d)\n\n", len(data.OpenCommissions)))
		cap := 5
		for i, commission := range data.OpenCommissions {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(data.OpenCommissions)-cap))
				break
			}
			sb.WriteString(formatCommissionStatusEntry(commission) + "\n")
		}
		sb.WriteString("\n")
	}

	if len(data.InProgressProblems) > 0 {
		sb.WriteString(fmt.Sprintf("### In Progress (%d)\n\n", len(data.InProgressProblems)))
		cap := 5
		for i, p := range data.InProgressProblems {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(data.InProgressProblems)-cap))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s → %s\n", formatProblemListEntry(p), statusPortfolioLabel(data, p.Meta.ID)))
		}
		sb.WriteString("\n")
	}

	if len(data.BacklogProblems) > 0 {
		sb.WriteString(fmt.Sprintf("### Backlog (%d)\n\n", len(data.BacklogProblems)))
		cap := 5
		for i, p := range data.BacklogProblems {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(data.BacklogProblems)-cap))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", formatProblemListEntry(p)))
		}
		sb.WriteString("\n")
	}

	if len(data.AddressedProblems) > 0 {
		sb.WriteString(fmt.Sprintf("### Addressed (%d)\n\n", len(data.AddressedProblems)))
		cap := 3
		for i, p := range data.AddressedProblems {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(data.AddressedProblems)-cap))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s → %s\n", formatProblemListEntry(p), statusDecisionLabel(data, p.Meta.ID)))
		}
		sb.WriteString("\n")
	}

	if len(data.RecentNotes) > 0 {
		sb.WriteString(fmt.Sprintf("### Recent Notes (%d)\n\n", len(data.RecentNotes)))
		for _, n := range data.RecentNotes {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", formatArtifactLabel(n.Meta.Title, n.Meta.ID), n.Meta.CreatedAt.Format("2006-01-02")))
		}
		sb.WriteString("\n")
	}

	hasAny := len(data.HealthyDecisions) > 0 ||
		len(data.PendingDecisions) > 0 ||
		len(data.UnassessedDecisions) > 0 ||
		len(data.StaleItems) > 0 ||
		len(data.Drift) > 0 ||
		len(data.OpenCommissions) > 0 ||
		len(data.CommissionAttention) > 0 ||
		len(data.InProgressProblems) > 0 ||
		len(data.BacklogProblems) > 0 ||
		len(data.AddressedProblems) > 0 ||
		len(data.RecentNotes) > 0
	if !hasAny {
		sb.WriteString("No artifacts found. Use /h-note or /h-frame to get started.\n")
	}

	return sb.String()
}

// CockpitStatusResponse formats the default operator status surface.
// It preserves StatusResponse as the detailed renderer for explicit full=true
// calls, while keeping the default path bounded and drill-down oriented.
func CockpitStatusResponse(data artifact.StatusData) string {
	var sb strings.Builder
	sb.WriteString("## Haft Status\n\n")
	sb.WriteString("### Operator Cockpit\n\n")

	appendCockpitAttention(&sb, data)
	appendCockpitActiveWork(&sb, data)
	appendCockpitDecisionHealth(&sb, data)
	appendCockpitDrillDown(&sb)

	return sb.String()
}

func appendCockpitAttention(sb *strings.Builder, data artifact.StatusData) {
	driftEvents := cockpitDriftEvents(data)
	openDriftEvents := cockpitOpenDriftEvents(driftEvents.Events)
	hasAttention := len(data.StaleItems) > 0 ||
		len(openDriftEvents) > 0 ||
		len(data.ReconciliationCues.Cues) > 0 ||
		len(data.CommissionAttention) > 0

	if !hasAttention {
		sb.WriteString("- No operator-blocking refresh, drift, or commission items in this status payload.\n\n")
		return
	}

	const staleCap = 2
	if len(data.StaleItems) > 0 {
		sb.WriteString(fmt.Sprintf("- **Refresh due** (%d):\n", len(data.StaleItems)))
		for i, stale := range data.StaleItems {
			if i >= staleCap {
				sb.WriteString(fmt.Sprintf("  - ... and %d more; run `haft_refresh(action=\"scan\", verbose=true)`.\n", len(data.StaleItems)-staleCap))
				break
			}
			line := "  - " + formatArtifactLabel(stale.Title, stale.ID)
			if health, ok := data.DecisionHealth[stale.ID]; ok {
				line += fmt.Sprintf(" — %s", health.Label())
			}
			line += fmt.Sprintf(" — %s", stale.Reason)
			sb.WriteString(line + "\n")
		}
	}

	const driftCap = 2
	if len(openDriftEvents) > 0 {
		materialEvents, auditEvents, unresolvedEvents := partitionCockpitDriftEvents(openDriftEvents)
		if len(materialEvents) > 0 {
			uniqueEvents, impactedDecisions, maxFanout := summarizeCockpitDriftEvents(openDriftEvents)
			sb.WriteString(fmt.Sprintf(
				"- **Drift events** (%d unique; %d impacted decision(s); max fanout %d):\n",
				uniqueEvents,
				impactedDecisions,
				maxFanout,
			))
		}
		for i, event := range materialEvents {
			if i >= driftCap {
				sb.WriteString(fmt.Sprintf("  - ... and %d more event(s); run `%s`.\n", len(materialEvents)-driftCap, artifact.StatusCompactDriftEventsCommand))
				break
			}
			sb.WriteString(fmt.Sprintf(
				"  - %s — fanout=%d materiality=%s resolution=%s\n",
				event.ChangedTargetRef,
				event.Fanout,
				event.Materiality,
				event.ResolutionStatus,
			))
		}
		if len(auditEvents) > 0 {
			sb.WriteString(fmt.Sprintf(
				"- **Audit-only drift events**: %s; run `%s` for compact paths or `haft_query(action=\"drift_events\", full=true)` for full audit.\n",
				formatAuditOnlyDriftEventSummary(auditEvents),
				artifact.StatusCompactDriftEventsCommand,
			))
		}
		if len(unresolvedEvents) > 0 {
			sb.WriteString(fmt.Sprintf(
				"- **Binding resolution needed**: %s; run `haft drift bindings --dry-run --json` or `%s`.\n",
				formatBindingResolutionDriftEventSummary(unresolvedEvents),
				artifact.StatusCompactDriftEventsCommand,
			))
		}
	}

	if len(data.ReconciliationCues.Cues) > 0 {
		sb.WriteString("- **Reconciliation cues**: ")
		sb.WriteString(ReconciliationCueSummary(data.ReconciliationCues))
		sb.WriteString("\n")
	}

	const commissionCap = 2
	if len(data.CommissionAttention) > 0 {
		sb.WriteString(fmt.Sprintf("- **WorkCommissions need attention** (%d):\n", len(data.CommissionAttention)))
		for i, commission := range data.CommissionAttention {
			if i >= commissionCap {
				sb.WriteString(fmt.Sprintf("  - ... and %d more; run `haft_commission(action=\"list\", state=\"open\")`.\n", len(data.CommissionAttention)-commissionCap))
				break
			}
			sb.WriteString("  " + formatCommissionStatusEntry(commission) + "\n")
		}
	}

	sb.WriteString("\n")
}

func appendCockpitActiveWork(sb *strings.Builder, data artifact.StatusData) {
	if len(data.InProgressProblems) == 0 && len(data.BacklogProblems) == 0 {
		return
	}

	sb.WriteString("### Active Work\n\n")

	const progressCap = 3
	for i, problem := range data.InProgressProblems {
		if i >= progressCap {
			sb.WriteString(fmt.Sprintf("- ... and %d more in progress; run `haft_query(action=\"status\", full=true)`.\n", len(data.InProgressProblems)-progressCap))
			break
		}
		sb.WriteString(fmt.Sprintf("- %s → %s\n", formatProblemListEntry(problem), statusPortfolioLabel(data, problem.Meta.ID)))
	}

	if len(data.BacklogProblems) > 0 {
		sb.WriteString(fmt.Sprintf("- Backlog: %d problem(s); run `haft_query(action=\"status\", full=true)` for the list.\n", len(data.BacklogProblems)))
	}

	sb.WriteString("\n")
}

func appendCockpitDecisionHealth(sb *strings.Builder, data artifact.StatusData) {
	driftEvents := cockpitDriftEvents(data)
	openDriftEvents := cockpitOpenDriftEvents(driftEvents.Events)
	totalDecisions := len(data.HealthyDecisions) +
		len(data.PendingDecisions) +
		len(data.UnassessedDecisions)

	if totalDecisions == 0 && len(data.StaleItems) == 0 && len(openDriftEvents) == 0 {
		return
	}

	sb.WriteString("### Decision Health\n\n")
	materialEvents, auditEvents, unresolvedEvents := partitionCockpitDriftEvents(openDriftEvents)
	if len(auditEvents) > 0 || len(unresolvedEvents) > 0 {
		sb.WriteString(fmt.Sprintf(
			"- Healthy: %d; Pending: %d; Unassessed: %d; Refresh due: %d; Drift: %d material event(s), %d audit-only event(s), %d needs-binding event(s).\n\n",
			len(data.HealthyDecisions),
			len(data.PendingDecisions),
			len(data.UnassessedDecisions),
			len(data.StaleItems),
			len(materialEvents),
			len(auditEvents),
			len(unresolvedEvents),
		))
		return
	}
	sb.WriteString(fmt.Sprintf(
		"- Healthy: %d; Pending: %d; Unassessed: %d; Refresh due: %d; Drift: %d material event(s).\n\n",
		len(data.HealthyDecisions),
		len(data.PendingDecisions),
		len(data.UnassessedDecisions),
		len(data.StaleItems),
		len(materialEvents),
	))
}

func appendCockpitDrillDown(sb *strings.Builder) {
	sb.WriteString("### Drill-down\n\n")
	sb.WriteString("- Full status: `haft_query(action=\"status\", full=true)`.\n")
	sb.WriteString("- Coverage: `haft_query(action=\"coverage\")`.\n")
	sb.WriteString("- Drift/stale detail: `haft_refresh(action=\"scan\", verbose=true)`.\n")
	sb.WriteString(fmt.Sprintf("- Drift events: `%s`; decision reconciliation: `%s`; governing set: `%s`.\n",
		artifact.StatusCompactDriftEventsCommand,
		artifact.StatusCompactDecisionReconcileCommand,
		artifact.StatusCompactGoverningSetCommand,
	))
	sb.WriteString("- Maintenance plan: `haft_refresh(action=\"plan\")`.\n")
	sb.WriteString("- Judgment review: `haft_refresh(action=\"review\")` / `haft overseer judgment --json --limit 20`.\n")
	sb.WriteString("- Safe drain preview: `haft_refresh(action=\"drain\", dry_run=true)`.\n")
	sb.WriteString("\nDefault status omits shipped/pending decision lists, full module coverage, recent notes, and full drift/stale tails.\n")
}

func ReconciliationCueSummary(report artifact.ReconciliationCueReport) string {
	parts := []string{}
	if report.Summary.HighFanoutEvents > 0 {
		parts = append(parts, fmt.Sprintf(
			"%d high-fanout drift event(s), max fanout %d",
			report.Summary.HighFanoutEvents,
			report.Summary.MaxFanout,
		))
	}
	if report.Summary.ReconciliationGroups > 0 {
		parts = append(parts, fmt.Sprintf(
			"%d reconciliation group(s), %d operator-required",
			report.Summary.ReconciliationGroups,
			report.Summary.OperatorRequiredGroups,
		))
	}
	if report.Summary.GoverningConflictSets > 0 || report.Summary.GoverningOverlapSets > 0 {
		parts = append(parts, fmt.Sprintf(
			"%d governing conflict set(s), %d overlap set(s)",
			report.Summary.GoverningConflictSets,
			report.Summary.GoverningOverlapSets,
		))
	}
	if len(parts) == 0 {
		return ""
	}
	if len(report.Commands) == 0 {
		return strings.Join(parts, "; ")
	}
	return strings.Join(parts, "; ") + "; drill down with " + strings.Join(report.Commands, " / ")
}

func formatDriftFileSummary(files []artifact.DriftItem) string {
	modified := 0
	added := 0
	missing := 0

	for _, file := range files {
		switch file.Status {
		case artifact.DriftModified:
			modified++
		case artifact.DriftAdded:
			added++
		case artifact.DriftMissing:
			missing++
		}
	}

	parts := []string{}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", modified))
	}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", added))
	}
	if missing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", missing))
	}
	if len(parts) == 0 {
		return "no file changes"
	}

	return strings.Join(parts, ", ")
}

func cockpitDriftEvents(data artifact.StatusData) artifact.DriftEventReport {
	if len(data.DriftEvents.Events) > 0 || data.DriftEvents.SchemaVersion != 0 {
		return data.DriftEvents
	}
	if len(data.Drift) == 0 {
		return artifact.DriftEventReport{}
	}
	return artifact.BuildDriftEventReport(data.Drift)
}

func cockpitOpenDriftEvents(events []artifact.DriftEvent) []artifact.DriftEvent {
	out := []artifact.DriftEvent{}
	for _, event := range events {
		if event.ResolutionRecord == nil {
			out = append(out, event)
			continue
		}
		switch event.ResolutionStatus {
		case artifact.DriftEventResolutionResolved, artifact.DriftEventResolutionWaivedUntil:
			continue
		default:
			out = append(out, event)
		}
	}
	return out
}

func summarizeCockpitDriftEvents(events []artifact.DriftEvent) (int, int, int) {
	decisions := map[string]struct{}{}
	maxFanout := 0
	for _, event := range events {
		if event.Fanout > maxFanout {
			maxFanout = event.Fanout
		}
		for _, decision := range event.ImpactedDecisions {
			decisions[decision.DecisionID] = struct{}{}
		}
	}
	return len(events), len(decisions), maxFanout
}

func partitionCockpitDriftEvents(events []artifact.DriftEvent) (material, audit, unresolved []artifact.DriftEvent) {
	for _, event := range events {
		if cockpitDriftEventNeedsBindingResolution(event) {
			unresolved = append(unresolved, event)
			continue
		}
		switch event.Materiality {
		case artifact.DriftMaterialityAdjacentFileChurn,
			artifact.DriftMaterialityCarrierOnly,
			artifact.DriftMaterialityGeneratedOrIgnored:
			audit = append(audit, event)
		default:
			material = append(material, event)
		}
	}
	return material, audit, unresolved
}

func cockpitDriftEventNeedsBindingResolution(event artifact.DriftEvent) bool {
	if event.Materiality == artifact.DriftMaterialityNeedsBindingResolution {
		return true
	}
	return event.Materiality == artifact.DriftMaterialityUnknownLegacyFileScope &&
		event.FallbackKind == artifact.BindingTargetWholeFileFallback
}

func partitionCockpitDrift(reports []artifact.DriftReport) (material, audit, unresolved []artifact.DriftReport) {
	for _, report := range reports {
		switch report.EffectiveMateriality() {
		case artifact.DriftMaterialityAdjacentFileChurn,
			artifact.DriftMaterialityCarrierOnly,
			artifact.DriftMaterialityGeneratedOrIgnored:
			audit = append(audit, report)
		case artifact.DriftMaterialityNeedsBindingResolution:
			unresolved = append(unresolved, report)
		default:
			material = append(material, report)
		}
	}
	return material, audit, unresolved
}

func formatAuditOnlyDriftEventSummary(events []artifact.DriftEvent) string {
	return fmt.Sprintf(
		"%d unique event(s), %d impacted decision(s), 0 material governed-symbol changes; audit details available",
		len(events),
		countDriftEventImpactedDecisions(events),
	)
}

func formatAuditOnlyDriftSummary(reports []artifact.DriftReport) string {
	triggerPaths := make(map[string]struct{})
	for _, report := range reports {
		for _, file := range report.Files {
			if file.Path == "" {
				continue
			}
			triggerPaths[file.Path] = struct{}{}
		}
	}

	return fmt.Sprintf(
		"%d trigger path(s), %d decision(s) checked, 0 material governed-symbol changes; audit details available",
		len(triggerPaths),
		len(reports),
	)
}

func formatBindingResolutionDriftEventSummary(events []artifact.DriftEvent) string {
	return fmt.Sprintf(
		"%d unique event(s), %d impacted decision(s) need precise binding targets",
		len(events),
		countDriftEventImpactedDecisions(events),
	)
}

func countDriftEventImpactedDecisions(events []artifact.DriftEvent) int {
	seen := map[string]struct{}{}
	for _, event := range events {
		for _, decision := range event.ImpactedDecisions {
			if decision.DecisionID == "" {
				continue
			}
			seen[decision.DecisionID] = struct{}{}
		}
	}
	return len(seen)
}

func formatBindingResolutionDriftSummary(reports []artifact.DriftReport) string {
	triggerPaths := make(map[string]struct{})
	for _, report := range reports {
		for _, file := range report.Files {
			if file.Path == "" {
				continue
			}
			triggerPaths[file.Path] = struct{}{}
		}
	}

	return fmt.Sprintf(
		"%d trigger path(s), %d decision(s) need precise binding targets",
		len(triggerPaths),
		len(reports),
	)
}

func formatCommissionStatusEntry(commission artifact.WorkCommissionStatus) string {
	line := fmt.Sprintf("- %s %s", formatArtifactLabel(commission.Title, commission.ID), commission.State)
	if commission.DecisionRef != "" {
		line += fmt.Sprintf(" → %s", formatArtifactLabel(commission.DecisionTitle, commission.DecisionRef))
	}
	if commission.AttentionReason != "" {
		line += " — " + commission.AttentionReason
	}
	if len(commission.SuggestedActions) > 0 {
		line += " — actions: " + strings.Join(commission.SuggestedActions, ", ")
	}
	return line
}

func statusPortfolioLabel(data artifact.StatusData, problemID string) string {
	ref := strings.TrimSpace(data.InProgressBy[problemID])
	return formatArtifactLabel(data.PortfolioTitles[ref], ref)
}

func statusDecisionLabel(data artifact.StatusData, problemID string) string {
	ref := strings.TrimSpace(data.AddressedBy[problemID])
	return formatArtifactLabel(data.DecisionTitles[ref], ref)
}

// ListResponse formats artifacts of a given kind as markdown.
func ListResponse(data artifact.ListData) string {
	if len(data.Artifacts) == 0 {
		kindHeading := UserFacingArtifactKindHeading(string(data.Kind), 0)
		kindHeading = strings.ToLower(kindHeading)
		if kindHeading == "" {
			kindHeading = strings.ToLower(strings.TrimSpace(string(data.Kind)))
		}
		return fmt.Sprintf("No %s found.\n", kindHeading)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%d)\n\n", UserFacingArtifactKindHeading(string(data.Kind), len(data.Artifacts)), len(data.Artifacts)))

	for i, a := range data.Artifacts {
		line := fmt.Sprintf("%d. **%s** `%s`", i+1, a.Meta.Title, a.Meta.ID)
		if a.Meta.Status != artifact.StatusActive {
			line += fmt.Sprintf(" [%s]", a.Meta.Status)
		}
		if a.Meta.ValidUntil != "" {
			vu := a.Meta.ValidUntil
			if len(vu) > 10 {
				vu = vu[:10]
			}
			line += fmt.Sprintf(" (valid until %s)", vu)
		}
		if a.Meta.Context != "" {
			line += fmt.Sprintf(" ctx:%s", a.Meta.Context)
		}
		sb.WriteString(line + "\n")
	}

	return sb.String()
}

// RelatedResponse formats artifacts linked to a file path as markdown.
func RelatedResponse(results []*artifact.Artifact, filePath string) string {
	if len(results) == 0 {
		return fmt.Sprintf("No decisions found affecting: %s\n", filePath)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Decisions affecting: %s\n\n", filePath))

	for _, a := range results {
		kindLabel := UserFacingArtifactKindLabel(string(a.Meta.Kind))
		sb.WriteString(fmt.Sprintf("- **%s** [%s] `%s`", a.Meta.Title, kindLabel, a.Meta.ID))
		switch a.Meta.Status {
		case artifact.StatusRefreshDue:
			sb.WriteString(" ⚠ REFRESH DUE")
		case artifact.StatusSuperseded:
			sb.WriteString(" (superseded)")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// RelatedProximityItem is one graph-proximity-ranked related node, resolved to a
// display title — the phase-2 PPR section of the related action (dec-20260604-3aaad199).
type RelatedProximityItem struct {
	Title string
	Label string // user-facing kind: "symbol" or "reasoning"
	Ref   string
}

// TestedByItem is one callable symbol of a file and the tests exercising it via
// call edges — the structural-coverage lane of related (dec-20260604-ef966a11).
type TestedByItem struct {
	Symbol   string
	Exported bool
	TestedBy []string // test function names; empty = not exercised
}

// TestedByResponse formats the structural test-coverage lane: callable symbols
// exercised by tests, then untested EXPORTED symbols as a gap signal. Honest by
// construction — "exercised by", never "verified" (a call edge is not an
// assertion). Returns "" when there is nothing worth showing. Pure.
func TestedByResponse(items []TestedByItem) string {
	var tested, gaps []TestedByItem
	for _, it := range items {
		if len(it.TestedBy) > 0 {
			tested = append(tested, it)
		} else if it.Exported {
			gaps = append(gaps, it)
		}
	}
	if len(tested) == 0 && len(gaps) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n## Tested by (exercised, not asserted)\n\n")
	sb.WriteString("_Test functions whose call edges reach this file's symbols — structural coverage, not verification._\n\n")
	for _, it := range tested {
		sb.WriteString(fmt.Sprintf("- **%s** ← %s\n", it.Symbol, strings.Join(it.TestedBy, ", ")))
	}
	for _, it := range gaps {
		sb.WriteString(fmt.Sprintf("- **%s** — _(no test exercises this)_\n", it.Symbol))
	}
	return sb.String()
}

// RelatedProximityResponse formats the graph-proximity recall section appended
// to the exact affected-file list. Empty input yields an empty string (the
// caller simply appends nothing). Pure.
func RelatedProximityResponse(items []RelatedProximityItem) string {
	if len(items) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n## Related by graph proximity\n\n")
	sb.WriteString("_Ranked by distance in the fused code+reasoning graph (deterministic PPR, no embeddings)._\n\n")
	for _, it := range items {
		sb.WriteString(fmt.Sprintf("- **%s** [%s] `%s`\n", it.Title, it.Label, it.Ref))
	}
	return sb.String()
}

// ProblemsListResponse formats a list of problems with pre-fetched enrichment data. Pure.
func ProblemsListResponse(items []artifact.ProblemListItem, navStrip string) string {
	var sb strings.Builder

	if len(items) == 0 {
		sb.WriteString("No active problems found.\n")
		sb.WriteString("Use /h-frame to frame a new problem.\n")
		sb.WriteString(navStrip)
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("## Active Problems (%d)\n\n", len(items)))
	sb.WriteString("Goldilocks guide: pick problems in the growth zone — not too trivial, not too impossible for your current capacity.\n\n")

	for i, item := range items {
		p := item.Problem
		sb.WriteString(fmt.Sprintf("### %d. %s [%s]\n", i+1, formatProblemTitleWithType(p), p.Meta.ID))
		if p.Meta.Context != "" {
			sb.WriteString(fmt.Sprintf("Context: %s | ", p.Meta.Context))
		}
		sb.WriteString(fmt.Sprintf("Mode: %s | Created: %s\n", p.Meta.Mode, p.Meta.CreatedAt.Format("2006-01-02")))

		if item.Signals != "" {
			sb.WriteString(item.Signals)
		}

		if item.CharCount > 0 {
			sb.WriteString(fmt.Sprintf("Characterization: %d version(s) defined\n", item.CharCount))
		} else {
			sb.WriteString("Characterization: not yet defined\n")
		}

		if item.EvidenceTotal > 0 {
			sb.WriteString(fmt.Sprintf("Evidence: %d item(s)", item.EvidenceTotal))
			if item.EvidenceSupp > 0 {
				sb.WriteString(fmt.Sprintf(", %d supporting", item.EvidenceSupp))
			}
			if item.EvidenceWeak > 0 {
				sb.WriteString(fmt.Sprintf(", %d weakening", item.EvidenceWeak))
			}
			if item.EvidenceRefute > 0 {
				sb.WriteString(fmt.Sprintf(", %d REFUTING", item.EvidenceRefute))
			}
			sb.WriteString("\n")
		}

		if item.ForwardLinks+item.BackLinks > 0 {
			sb.WriteString(fmt.Sprintf("Links: %d forward, %d back\n", item.ForwardLinks, item.BackLinks))
		}

		if p.Meta.ValidUntil != "" {
			vu := p.Meta.ValidUntil
			if len(vu) > 10 {
				vu = vu[:10]
			}
			sb.WriteString(fmt.Sprintf("Valid until: %s\n", vu))
		}

		sb.WriteString("\n")
	}

	sb.WriteString(navStrip)
	return sb.String()
}

func formatArtifactLabel(title string, ref string) string {
	title = strings.TrimSpace(title)
	ref = strings.TrimSpace(ref)

	switch {
	case title != "" && ref != "":
		return fmt.Sprintf("**%s** `%s`", title, ref)
	case title != "":
		return fmt.Sprintf("**%s**", title)
	case ref != "":
		return fmt.Sprintf("**untitled artifact** `%s`", ref)
	default:
		return "**unnamed artifact**"
	}
}

// ArtifactLabel renders an operator-facing artifact reference with semantic
// context first and the stable ref second.
func ArtifactLabel(title string, ref string) string {
	return formatArtifactLabel(title, ref)
}

func formatProblemListEntry(problem *artifact.Artifact) string {
	return formatArtifactLabel(formatProblemTitleWithType(problem), problem.Meta.ID)
}

func formatProblemTitleWithType(problem *artifact.Artifact) string {
	if problem == nil {
		return ""
	}

	problemType := artifact.ProblemTypeLabel(problem)
	if problemType == "" {
		return problem.Meta.Title
	}

	return fmt.Sprintf("%s (%s)", problem.Meta.Title, problemType)
}
