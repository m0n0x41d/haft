package present

import (
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/reff"
)

// ProjectionResponse renders one deterministic audience projection over the same artifact graph.
func ProjectionResponse(graph artifact.ProjectionGraph, view artifact.ProjectionView) string {
	var body string
	switch view {
	case artifact.ProjectionViewEngineer:
		body = engineerProjectionResponse(graph)
	case artifact.ProjectionViewManager:
		body = managerProjectionResponse(graph)
	case artifact.ProjectionViewAudit:
		body = auditProjectionResponse(graph)
	case artifact.ProjectionViewCompare:
		body = compareProjectionResponse(graph)
	case artifact.ProjectionViewDelegatedAgent:
		body = delegatedAgentProjectionResponse(graph)
	case artifact.ProjectionViewChangeRationale:
		body = changeRationaleProjectionResponse(graph)
	default:
		return fmt.Sprintf("Unsupported projection view: %s\n", view)
	}
	return body + renderCarrierFootnote(collectProjectionSources(graph))
}

// projectionSourceRef is one underlying artifact referenced by a projection.
// Per FPF A.15.4 ("Work-Relevant Source Restoration") a projection is a
// carrier — not authorization, evidence, or work. The footer lists these so
// the consumer can recover source before acting on the rendered carrier.
type projectionSourceRef struct {
	Kind  artifact.Kind
	Ref   string
	Title string
}

// renderCarrierFootnote emits the A.15.4 source-restoration footer appended to
// every projection. Empty sources still render the section with an explicit
// "no underlying source artifacts" note so the carrier semantics stay visible.
func renderCarrierFootnote(sources []projectionSourceRef) string {
	var sb strings.Builder
	sb.WriteString("\n---\n\n## Carrier — Not Source of Truth (A.15.4)\n\n")
	sb.WriteString("This is a projection rendered for boundary-crossing handoff. Per FPF A.15.4, before action (approval, gate passage, evidence use, engineering justification, release), recover the underlying project source:\n\n")
	if len(sources) == 0 {
		sb.WriteString("- (no underlying source artifacts; this projection is informational only)\n\n")
	} else {
		for _, source := range sources {
			label := source.Kind.UserFacingLabel()
			if title := strings.TrimSpace(source.Title); title != "" {
				sb.WriteString(fmt.Sprintf("- %s: `%s` — %q\n", label, source.Ref, title))
				continue
			}
			sb.WriteString(fmt.Sprintf("- %s: `%s`\n", label, source.Ref))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Recover via `haft_query(action=\"get\", ref=\"<id>\")` or read directly from `.haft/{decisions|problems|solutions|evidence}/<id>.md`.\n")
	return sb.String()
}

// collectProjectionSources enumerates every artifact node that participated in
// the projection so the carrier footer can list them. Deterministic ordering:
// problems → portfolios → decisions, each in the graph's existing sort order.
func collectProjectionSources(graph artifact.ProjectionGraph) []projectionSourceRef {
	sources := make([]projectionSourceRef, 0, len(graph.Problems)+len(graph.Portfolios)+len(graph.Decisions))
	for _, problem := range graph.Problems {
		sources = append(sources, projectionSourceRef{
			Kind:  artifact.KindProblemCard,
			Ref:   problem.Meta.ID,
			Title: problem.Meta.Title,
		})
	}
	for _, portfolio := range graph.Portfolios {
		sources = append(sources, projectionSourceRef{
			Kind:  artifact.KindSolutionPortfolio,
			Ref:   portfolio.Meta.ID,
			Title: portfolio.Meta.Title,
		})
	}
	for _, decision := range graph.Decisions {
		sources = append(sources, projectionSourceRef{
			Kind:  artifact.KindDecisionRecord,
			Ref:   decision.Meta.ID,
			Title: decision.Meta.Title,
		})
	}
	return sources
}

func projectionArtifactLabels(graph artifact.ProjectionGraph) map[string]string {
	labels := make(map[string]string, len(graph.Problems)+len(graph.Portfolios)+len(graph.Decisions))
	for _, problem := range graph.Problems {
		labels[problem.Meta.ID] = formatArtifactLabel(problem.Meta.Title, problem.Meta.ID)
	}
	for _, portfolio := range graph.Portfolios {
		labels[portfolio.Meta.ID] = formatArtifactLabel(portfolio.Meta.Title, portfolio.Meta.ID)
	}
	for _, decision := range graph.Decisions {
		labels[decision.Meta.ID] = formatArtifactLabel(decision.Meta.Title, decision.Meta.ID)
	}
	return labels
}

func formatProjectionRefs(refs []string, labels map[string]string) string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if label := labels[ref]; label != "" {
			out = append(out, label)
			continue
		}
		out = append(out, formatArtifactLabel("", ref))
	}
	return strings.Join(out, ", ")
}

func engineerProjectionResponse(graph artifact.ProjectionGraph) string {
	if projectionGraphEmpty(graph) {
		return "No active artifacts available for the engineer projection.\n"
	}

	labels := projectionArtifactLabels(graph)
	var sb strings.Builder
	sb.WriteString("## Engineer View\n\n")

	if len(graph.Problems) > 0 {
		sb.WriteString("### Problems\n\n")
		for _, problem := range graph.Problems {
			sb.WriteString(fmt.Sprintf("#### %s `%s`\n\n", problem.Meta.Title, problem.Meta.ID))
			sb.WriteString(fmt.Sprintf("Mode: %s | Status: %s\n", problem.Meta.Mode, problem.Meta.Status))
			if problem.Signal != "" {
				sb.WriteString(fmt.Sprintf("Signal: %s\n", problem.Signal))
			}
			if problem.Acceptance != "" {
				sb.WriteString(fmt.Sprintf("Acceptance: %s\n", problem.Acceptance))
			}
			if len(problem.OptimizationTargets) > 0 {
				sb.WriteString(fmt.Sprintf("Targets: %s\n", strings.Join(problem.OptimizationTargets, ", ")))
			}
			if len(problem.PortfolioRefs) > 0 {
				sb.WriteString(fmt.Sprintf("Portfolios: %s\n", formatProjectionRefs(problem.PortfolioRefs, labels)))
			}
			if len(problem.DecisionRefs) > 0 {
				sb.WriteString(fmt.Sprintf("Decisions: %s\n", formatProjectionRefs(problem.DecisionRefs, labels)))
			}
			sb.WriteString("\n")
		}
	}

	if len(graph.Portfolios) > 0 {
		sb.WriteString("### Portfolios\n\n")
		for _, portfolio := range graph.Portfolios {
			sb.WriteString(fmt.Sprintf("#### %s `%s`\n\n", portfolio.Meta.Title, portfolio.Meta.ID))
			sb.WriteString(fmt.Sprintf("Mode: %s | Status: %s\n", portfolio.Meta.Mode, portfolio.Meta.Status))
			if len(portfolio.ProblemRefs) > 0 {
				sb.WriteString(fmt.Sprintf("Problems: %s\n", formatProjectionRefs(portfolio.ProblemRefs, labels)))
			}
			if len(portfolio.DecisionRefs) > 0 {
				sb.WriteString(fmt.Sprintf("Decisions: %s\n", formatProjectionRefs(portfolio.DecisionRefs, labels)))
			}
			sb.WriteString(fmt.Sprintf("Variants (%d): %s\n", len(portfolio.Variants), strings.Join(projectionVariantTitles(portfolio.Variants), ", ")))
			if portfolio.Comparison != nil {
				sb.WriteString(fmt.Sprintf("Computed Pareto front: %s\n", strings.Join(displayComparisonVariantLabels(portfolio.Comparison.NonDominatedSet, solutionVariantLabels(portfolio.Variants)), ", ")))
			}
			sb.WriteString("\n")
		}
	}

	if len(graph.Decisions) > 0 {
		sb.WriteString("### Decisions\n\n")
		for _, decision := range graph.Decisions {
			sb.WriteString(fmt.Sprintf("#### %s `%s`\n\n", decision.Meta.Title, decision.Meta.ID))
			sb.WriteString(fmt.Sprintf("Mode: %s | Status: %s\n", decision.Meta.Mode, decision.Meta.Status))
			if len(decision.ProblemRefs) > 0 {
				sb.WriteString(fmt.Sprintf("Problems: %s\n", formatProjectionRefs(decision.ProblemRefs, labels)))
			}
			if len(decision.PortfolioRefs) > 0 {
				sb.WriteString(fmt.Sprintf("Portfolios: %s\n", formatProjectionRefs(decision.PortfolioRefs, labels)))
			}
			if decision.SelectedTitle != "" {
				sb.WriteString(fmt.Sprintf("Selected: %s\n", decision.SelectedTitle))
			}
			if decision.WeakestLink != "" {
				sb.WriteString(fmt.Sprintf("Weakest link: %s\n", decision.WeakestLink))
			}
			if decision.SelectionPolicy != "" {
				sb.WriteString(fmt.Sprintf("Policy: %s\n", decision.SelectionPolicy))
			}
			writeProjectionDecisionPredictions(&sb, "Predictions", decision.Predictions)
			writeProjectionDecisionSlice(&sb, "Pre-conditions", decision.PreConditions)
			writeProjectionDecisionSlice(&sb, "Evidence requirements", decision.EvidenceRequirements)
			writeProjectionDecisionSlice(&sb, "Rollback triggers", decision.RollbackTriggers)
			writeProjectionDecisionSlice(&sb, "Refresh triggers", decision.RefreshTriggers)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func managerProjectionResponse(graph artifact.ProjectionGraph) string {
	if projectionGraphEmpty(graph) {
		return "No active artifacts available for the manager/status projection.\n"
	}

	backlog, inProgress, addressed := projectionProblemStages(graph.Problems)
	unassessed, pending, shipped, refreshDue := projectionDecisionStages(graph.Decisions)
	compared := 0
	for _, portfolio := range graph.Portfolios {
		if portfolio.Comparison != nil {
			compared++
		}
	}

	var sb strings.Builder
	sb.WriteString("## Manager/Status View\n\n")
	sb.WriteString(fmt.Sprintf("Problems: %d backlog, %d in progress, %d addressed\n", backlog, inProgress, addressed))
	sb.WriteString(
		fmt.Sprintf(
			"Decisions: %d unassessed, %d pending follow-through, %d measured/shipped, %d refresh due\n",
			unassessed,
			pending,
			shipped,
			refreshDue,
		),
	)
	sb.WriteString(fmt.Sprintf("Compared portfolios: %d\n\n", compared))

	if len(graph.Problems) > 0 {
		sb.WriteString("### Work Queue\n\n")
		for _, problem := range graph.Problems {
			stage := "backlog"
			switch {
			case len(problem.DecisionRefs) > 0:
				stage = "addressed"
			case len(problem.PortfolioRefs) > 0:
				stage = "in progress"
			}

			sb.WriteString(fmt.Sprintf("- **%s** `%s` — %s\n", problem.Meta.Title, problem.Meta.ID, stage))
		}
		sb.WriteString("\n")
	}

	if len(graph.Decisions) > 0 {
		sb.WriteString("### Decision Watchlist\n\n")
		for _, decision := range graph.Decisions {
			statusParts := []string{projectionDecisionWatchStatus(decision)}
			if decision.NeedsRefresh {
				statusParts = append(statusParts, "refresh due")
			}
			sb.WriteString(fmt.Sprintf("- **%s** `%s` — %s\n", decision.Meta.Title, decision.Meta.ID, strings.Join(statusParts, ", ")))
		}
	}

	return sb.String()
}

func auditProjectionResponse(graph artifact.ProjectionGraph) string {
	if projectionGraphEmpty(graph) {
		return "No active artifacts available for the audit/evidence projection.\n"
	}

	var sb strings.Builder
	sb.WriteString("## Audit/Evidence View\n\n")

	if len(graph.Decisions) > 0 {
		sb.WriteString("### Decisions\n\n")
		for _, decision := range graph.Decisions {
			sb.WriteString(fmt.Sprintf("#### %s `%s`\n\n", decision.Meta.Title, decision.Meta.ID))
			if decision.Meta.ValidUntil != "" {
				sb.WriteString(fmt.Sprintf("Valid until: %s\n", decision.Meta.ValidUntil))
			}
			if decision.SelectionPolicy != "" {
				sb.WriteString(fmt.Sprintf("Selection policy: %s\n", decision.SelectionPolicy))
			}
			if decision.CounterArgument != "" {
				sb.WriteString(fmt.Sprintf("Counterargument: %s\n", decision.CounterArgument))
			}
			if decision.WeakestLink != "" {
				sb.WriteString(fmt.Sprintf("Weakest link: %s\n", decision.WeakestLink))
			}
			writeProjectionDecisionPredictions(&sb, "Predictions", decision.Predictions)
			sb.WriteString(fmt.Sprintf("Evidence: %s\n", decision.Evidence.WLNK.Summary))
			if len(decision.Evidence.WLNK.CoverageGaps) > 0 {
				sb.WriteString(fmt.Sprintf("Coverage gaps: %s\n", strings.Join(decision.Evidence.WLNK.CoverageGaps, ", ")))
			}
			if decision.Evidence.WLNK.MinFreshness != "" {
				sb.WriteString(fmt.Sprintf("Freshness floor: %s\n", decision.Evidence.WLNK.MinFreshness))
			}
			if decision.NeedsRefresh {
				sb.WriteString("Refresh state: due\n")
			}
			sb.WriteString(fmt.Sprintf(
				"Assurance: R_eff=%.2f | F_eff=%d scale=%s bridge_loss=%s | weakest CL=%d\n",
				decision.Evidence.WLNK.REff,
				decision.Evidence.WLNK.FEff,
				projectionFormalityScaleID(decision.Evidence.WLNK),
				projectionFormalityBridgeLoss(decision.Evidence.WLNK),
				decision.Evidence.WLNK.WeakestCL,
			))
			sb.WriteString("\n")
		}
	}

	if len(graph.Portfolios) > 0 {
		sb.WriteString("### Portfolios\n\n")
		for _, portfolio := range graph.Portfolios {
			sb.WriteString(fmt.Sprintf("- **%s** `%s` — %s\n", portfolio.Meta.Title, portfolio.Meta.ID, portfolio.Evidence.WLNK.Summary))
		}
		sb.WriteString("\n")
	}

	if len(graph.Problems) > 0 {
		sb.WriteString("### Problems\n\n")
		for _, problem := range graph.Problems {
			sb.WriteString(fmt.Sprintf("- **%s** `%s` — %s\n", problem.Meta.Title, problem.Meta.ID, problem.Evidence.WLNK.Summary))
		}
	}

	return sb.String()
}

func projectionFormalityScaleID(summary artifact.WLNKSummary) string {
	scaleID := strings.TrimSpace(summary.FormalityScaleID)
	if scaleID == "" {
		return reff.FormalityScaleCurrent
	}

	return scaleID
}

func projectionFormalityBridgeLoss(summary artifact.WLNKSummary) string {
	loss := strings.TrimSpace(summary.FormalityBridgeLoss)
	if loss == "" {
		return reff.FormalityBridgeNoLoss
	}

	return loss
}

func compareProjectionResponse(graph artifact.ProjectionGraph) string {
	comparedPortfolios := make([]artifact.PortfolioProjection, 0, len(graph.Portfolios))
	labels := projectionArtifactLabels(graph)
	for _, portfolio := range graph.Portfolios {
		if portfolio.Comparison == nil {
			continue
		}

		comparedPortfolios = append(comparedPortfolios, portfolio)
	}

	if len(comparedPortfolios) == 0 {
		return "No compared portfolios available for the compare/Pareto projection.\n"
	}

	var sb strings.Builder
	sb.WriteString("## Compare/Pareto View\n\n")

	for _, portfolio := range comparedPortfolios {
		sb.WriteString(fmt.Sprintf("### %s `%s`\n\n", portfolio.Meta.Title, portfolio.Meta.ID))
		if len(portfolio.ProblemRefs) > 0 {
			sb.WriteString(fmt.Sprintf("Problems: %s\n", formatProjectionRefs(portfolio.ProblemRefs, labels)))
		}
		if len(portfolio.DecisionRefs) > 0 {
			sb.WriteString(fmt.Sprintf("Decisions: %s\n", formatProjectionRefs(portfolio.DecisionRefs, labels)))
		}
		sb.WriteString(fmt.Sprintf("Variants: %s\n", strings.Join(projectionVariantTitles(portfolio.Variants), ", ")))

		summary := structuredComparisonSummary(*portfolio.Comparison, solutionVariantLabels(portfolio.Variants))
		if summary != "" {
			sb.WriteString(summary)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func delegatedAgentProjectionResponse(graph artifact.ProjectionGraph) string {
	if len(graph.Decisions) == 0 {
		return "No active decisions available for the delegated-agent brief.\n"
	}

	var sb strings.Builder
	sb.WriteString("## Delegated-Agent Brief\n\n")

	for _, decision := range graph.Decisions {
		sb.WriteString(fmt.Sprintf("### Selected decision: %s `%s`\n\n", projectionDecisionLabel(decision), decision.Meta.ID))
		writeProjectionDecisionSlice(&sb, "Affected files", decision.AffectedFiles)
		writeProjectionDecisionSlice(&sb, "Invariants", decision.Invariants)
		writeProjectionDecisionSlice(&sb, "Admissibility", decision.Admissibility)
		writeProjectionDecisionSlice(&sb, "Rollback triggers", decision.RollbackTriggers)
		writeProjectionDecisionSlice(&sb, "Open claim risks", projectionDecisionOpenClaimRisks(decision))
		sb.WriteString("\n")
	}

	return sb.String()
}

func changeRationaleProjectionResponse(graph artifact.ProjectionGraph) string {
	if len(graph.Decisions) == 0 {
		return "No active decisions available for the PR/change rationale projection.\n"
	}

	problemSignals := projectionProblemSignalsByID(graph.Problems)

	var sb strings.Builder
	sb.WriteString("## PR/Change Rationale\n\n")

	for _, decision := range graph.Decisions {
		sb.WriteString(fmt.Sprintf("### Selected change: %s `%s`\n\n", projectionDecisionLabel(decision), decision.Meta.ID))
		writeProjectionDecisionSlice(&sb, "Problem signal", projectionDecisionProblemSignals(decision, problemSignals))
		sb.WriteString(fmt.Sprintf("Selected variant: %s\n", projectionDecisionLabel(decision)))
		if decision.WhySelected != "" {
			sb.WriteString(fmt.Sprintf("Why selected: %s\n", decision.WhySelected))
		}
		writeProjectionDecisionRejections(&sb, "Rejected alternatives", decision.WhyNotOthers)
		writeProjectionDecisionSlice(&sb, "Rollback summary", decision.RollbackTriggers)
		if decision.Evidence.MeasurementVerdict != "" {
			sb.WriteString(fmt.Sprintf("Latest measurement verdict: %s\n", decision.Evidence.MeasurementVerdict))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func projectionGraphEmpty(graph artifact.ProjectionGraph) bool {
	return len(graph.Problems) == 0 && len(graph.Portfolios) == 0 && len(graph.Decisions) == 0
}

func projectionProblemStages(problems []artifact.ProblemProjection) (backlog int, inProgress int, addressed int) {
	for _, problem := range problems {
		switch {
		case len(problem.DecisionRefs) > 0:
			addressed++
		case len(problem.PortfolioRefs) > 0:
			inProgress++
		default:
			backlog++
		}
	}

	return backlog, inProgress, addressed
}

func projectionDecisionStages(decisions []artifact.DecisionProjection) (unassessed int, pending int, shipped int, refreshDue int) {
	for _, decision := range decisions {
		health := projectionDecisionHealth(decision)

		switch health.Maturity {
		case artifact.DecisionMaturityUnassessed:
			unassessed++
		case artifact.DecisionMaturityPending:
			pending++
		case artifact.DecisionMaturityShipped:
			shipped++
		default:
			pending++
		}

		if decision.NeedsRefresh {
			refreshDue++
		}
	}

	return unassessed, pending, shipped, refreshDue
}

func projectionDecisionHealth(decision artifact.DecisionProjection) artifact.DecisionHealth {
	if decision.Health.Maturity != "" {
		return decision.Health
	}

	if decision.Measured {
		return artifact.DecisionHealth{Maturity: artifact.DecisionMaturityShipped}
	}

	return artifact.DecisionHealth{Maturity: artifact.DecisionMaturityPending}
}

func projectionDecisionWatchStatus(decision artifact.DecisionProjection) string {
	health := projectionDecisionHealth(decision)

	switch health.Maturity {
	case artifact.DecisionMaturityUnassessed:
		return "unassessed"
	case artifact.DecisionMaturityShipped:
		return "measured"
	default:
		return "waiting for measurement"
	}
}

func projectionVariantTitles(variants []artifact.Variant) []string {
	titles := make([]string, 0, len(variants))
	for _, variant := range variants {
		title := strings.TrimSpace(variant.Title)
		if title == "" {
			continue
		}

		titles = append(titles, title)
	}

	sort.Strings(titles)
	return titles
}

func projectionDecisionLabel(decision artifact.DecisionProjection) string {
	label := strings.TrimSpace(decision.SelectedTitle)
	if label != "" {
		return label
	}

	label = strings.TrimSpace(decision.Meta.Title)
	if label != "" {
		return label
	}

	return decision.Meta.ID
}

func projectionDecisionOpenClaimRisks(decision artifact.DecisionProjection) []string {
	risks := make([]string, 0, len(decision.Predictions)+1)
	weakestLink := strings.TrimSpace(decision.WeakestLink)
	if weakestLink != "" {
		risks = append(risks, "weakest link: "+weakestLink)
	}

	for _, prediction := range decision.Predictions {
		status := prediction.Status
		if status == "" {
			status = artifact.ClaimStatusUnverified
		}
		if status == artifact.ClaimStatusSupported {
			continue
		}

		prediction.Status = status
		risks = append(risks, formatProjectionDecisionPrediction(prediction))
	}

	return risks
}

func projectionProblemSignalsByID(problems []artifact.ProblemProjection) map[string]string {
	signalsByID := make(map[string]string, len(problems))

	for _, problem := range problems {
		signal := strings.TrimSpace(problem.Signal)
		if signal == "" {
			continue
		}

		signalsByID[problem.Meta.ID] = signal
	}

	return signalsByID
}

func projectionDecisionProblemSignals(decision artifact.DecisionProjection, signalsByID map[string]string) []string {
	signals := make([]string, 0, len(decision.ProblemRefs))
	seen := make(map[string]struct{}, len(decision.ProblemRefs))

	for _, problemRef := range decision.ProblemRefs {
		signal := strings.TrimSpace(signalsByID[problemRef])
		if signal == "" {
			continue
		}
		if _, ok := seen[signal]; ok {
			continue
		}

		seen[signal] = struct{}{}
		signals = append(signals, signal)
	}

	return signals
}

func writeProjectionDecisionSlice(sb *strings.Builder, label string, values []string) {
	if len(values) == 0 {
		return
	}

	sb.WriteString(fmt.Sprintf("%s: %s\n", label, strings.Join(values, ", ")))
}

func writeProjectionDecisionRejections(sb *strings.Builder, label string, reasons []artifact.RejectionReason) {
	if len(reasons) == 0 {
		return
	}

	sb.WriteString(label)
	sb.WriteString(":\n")

	for _, reason := range reasons {
		var parts []string

		variant := strings.TrimSpace(reason.Variant)
		if variant != "" {
			parts = append(parts, variant)
		}

		detail := strings.TrimSpace(reason.Reason)
		switch {
		case len(parts) > 0 && detail != "":
			sb.WriteString(fmt.Sprintf("- %s: %s\n", strings.Join(parts, " "), detail))
		case detail != "":
			sb.WriteString(fmt.Sprintf("- %s\n", detail))
		case len(parts) > 0:
			sb.WriteString(fmt.Sprintf("- %s\n", strings.Join(parts, " ")))
		}
	}
}

func writeProjectionDecisionPredictions(sb *strings.Builder, label string, predictions []artifact.DecisionPrediction) {
	if len(predictions) == 0 {
		return
	}

	sb.WriteString(label)
	sb.WriteString(":\n")

	for _, prediction := range predictions {
		sb.WriteString("- ")
		sb.WriteString(formatProjectionDecisionPrediction(prediction))
		sb.WriteString("\n")
	}
}

func formatProjectionDecisionPrediction(prediction artifact.DecisionPrediction) string {
	parts := make([]string, 0, 2)
	claim := strings.TrimSpace(prediction.Claim)
	observable := strings.TrimSpace(prediction.Observable)
	threshold := strings.TrimSpace(prediction.Threshold)

	if claim != "" {
		parts = append(parts, claim)
	}
	if observable != "" || threshold != "" {
		details := make([]string, 0, 2)
		if observable != "" {
			details = append(details, "observable: "+observable)
		}
		if threshold != "" {
			details = append(details, "threshold: "+threshold)
		}
		parts = append(parts, "("+strings.Join(details, "; ")+")")
	}

	status := strings.TrimSpace(string(prediction.Status))
	return status + ": " + strings.Join(parts, " ")
}
