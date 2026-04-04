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

	sb.WriteString(fmt.Sprintf("Recorded: %s\n", a.Meta.Title))
	sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
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
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s] (%s)\n", i+1, item.Title, item.ID, kindLabel))
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
			sb.WriteString(fmt.Sprintf("New ProblemCard: %s (%s)\n", newProb.Meta.Title, newProb.Meta.ID))
			sb.WriteString("Use /q-explore to find new solutions.\n")
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
func BaselineResponse(decisionRef string, files []artifact.AffectedFile, navStrip string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Baseline set for %s. Monitoring %d file(s).\n\n", decisionRef, len(files)))
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("  %s — %s\n", f.Path, f.Hash[:12]))
	}
	sb.WriteString(navStrip)
	return sb.String()
}

// DriftResponse formats drift check results for the agent.
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
			sb.WriteString(fmt.Sprintf("### %s [%s]\n\n", r.DecisionTitle, r.DecisionID))
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
			sb.WriteString(fmt.Sprintf("**Impact propagation for %s:**\n", r.DecisionID))
			for _, impact := range r.ImpactedModules {
				if impact.IsBlind {
					sb.WriteString(fmt.Sprintf("  ⚠ %s (blind) — no decisions, potential unmonitored impact\n", impact.ModulePath))
				} else {
					sb.WriteString(fmt.Sprintf("  → %s — governed by %s\n", impact.ModulePath, strings.Join(impact.DecisionIDs, ", ")))
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
			sb.WriteString(fmt.Sprintf("- **%s** [%s] — %d file(s) unmonitored, %s\n",
				r.DecisionTitle, r.DecisionID, len(r.Files), gitHint))
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
		sb.WriteString(fmt.Sprintf("Decision recorded: %s\nID: %s\n", a.Meta.Title, a.Meta.ID))
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
		sb.WriteString(fmt.Sprintf("Impact measured: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
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
		sb.WriteString(fmt.Sprintf("Portfolio created: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
		if filePath != "" {
			sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
		}
	case "compare":
		sb.WriteString(fmt.Sprintf("Comparison added to: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
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
			labels[id] = title
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
	return "No active ProblemCard found.\n\n" +
		"Frame the problem first:\n" +
		"  /q-frame — define what's anomalous, constraints, acceptance criteria\n\n" +
		"Or explore directly in tactical mode:\n" +
		"  haft_solution(action=\"explore\", variants=[...])\n" +
		"  → will create a lightweight ProblemCard from context\n" +
		navStrip
}

// ProblemResponse builds the MCP tool response for a framed problem.
func ProblemResponse(action string, a *artifact.Artifact, filePath string, navStrip string) string {
	var sb strings.Builder

	switch action {
	case "frame":
		sb.WriteString(fmt.Sprintf("Problem framed: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
		sb.WriteString(fmt.Sprintf("Mode: %s\n", a.Meta.Mode))
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
		sb.WriteString(fmt.Sprintf("Characterization added to: %s\n", a.Meta.Title))
		sb.WriteString(fmt.Sprintf("ID: %s\n", a.Meta.ID))
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
		sb.WriteString(fmt.Sprintf("%d. **%s** [%s] `%s`\n", i+1, a.Meta.Title, a.Meta.Kind, a.Meta.ID))
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
			line := fmt.Sprintf("- **%s** `%s`", d.Meta.Title, d.Meta.ID)
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

	if len(data.PendingDecisions) > 0 {
		sb.WriteString(fmt.Sprintf("### Pending Implementation (%d)\n\n", len(data.PendingDecisions)))
		formatDecisionList(data.PendingDecisions, 5)
		sb.WriteString("\n")
	}

	if len(data.ShippedDecisions) > 0 {
		sb.WriteString(fmt.Sprintf("### Shipped (%d)\n\n", len(data.ShippedDecisions)))
		formatDecisionList(data.ShippedDecisions, 5)
		sb.WriteString("\n")
	}

	if len(data.StaleItems) > 0 {
		sb.WriteString(fmt.Sprintf("### Refresh Due (%d)\n\n", len(data.StaleItems)))
		cap := 5
		for i, s := range data.StaleItems {
			if i >= cap {
				sb.WriteString(fmt.Sprintf("- ... and %d more (use /q-refresh to see all)\n", len(data.StaleItems)-cap))
				break
			}
			sb.WriteString(fmt.Sprintf("- **%s** `%s` — %s\n", s.Title, s.ID, s.Reason))
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
			sb.WriteString(fmt.Sprintf("- **%s** `%s` → %s\n", p.Meta.Title, p.Meta.ID, data.InProgressBy[p.Meta.ID]))
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
			sb.WriteString(fmt.Sprintf("- **%s** `%s`\n", p.Meta.Title, p.Meta.ID))
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
			sb.WriteString(fmt.Sprintf("- **%s** `%s` → %s\n", p.Meta.Title, p.Meta.ID, data.AddressedBy[p.Meta.ID]))
		}
		sb.WriteString("\n")
	}

	if len(data.RecentNotes) > 0 {
		sb.WriteString(fmt.Sprintf("### Recent Notes (%d)\n\n", len(data.RecentNotes)))
		for _, n := range data.RecentNotes {
			sb.WriteString(fmt.Sprintf("- %s `%s` (%s)\n", n.Meta.Title, n.Meta.ID, n.Meta.CreatedAt.Format("2006-01-02")))
		}
		sb.WriteString("\n")
	}

	hasAny := len(data.PendingDecisions) > 0 ||
		len(data.ShippedDecisions) > 0 ||
		len(data.StaleItems) > 0 ||
		len(data.InProgressProblems) > 0 ||
		len(data.BacklogProblems) > 0 ||
		len(data.AddressedProblems) > 0 ||
		len(data.RecentNotes) > 0
	if !hasAny {
		sb.WriteString("No artifacts found. Use /q-note or /q-frame to get started.\n")
	}

	return sb.String()
}

// ListResponse formats artifacts of a given kind as markdown.
func ListResponse(data artifact.ListData) string {
	if len(data.Artifacts) == 0 {
		return fmt.Sprintf("No %s artifacts found.\n", data.Kind)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%d)\n\n", data.Kind, len(data.Artifacts)))

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
		sb.WriteString(fmt.Sprintf("- **%s** [%s] `%s`", a.Meta.Title, a.Meta.Kind, a.Meta.ID))
		if a.Meta.Status == artifact.StatusRefreshDue {
			sb.WriteString(" ⚠ REFRESH DUE")
		} else if a.Meta.Status == artifact.StatusSuperseded {
			sb.WriteString(" (superseded)")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ProblemsListResponse formats a list of problems with pre-fetched enrichment data. Pure.
func ProblemsListResponse(items []artifact.ProblemListItem, navStrip string) string {
	var sb strings.Builder

	if len(items) == 0 {
		sb.WriteString("No active problems found.\n")
		sb.WriteString("Use /q-frame to frame a new problem.\n")
		sb.WriteString(navStrip)
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("## Active Problems (%d)\n\n", len(items)))
	sb.WriteString("Goldilocks guide: pick problems in the growth zone — not too trivial, not too impossible for your current capacity.\n\n")

	for i, item := range items {
		p := item.Problem
		sb.WriteString(fmt.Sprintf("### %d. %s [%s]\n", i+1, p.Meta.Title, p.Meta.ID))
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
