package present

import (
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// ProjectionResponse renders one deterministic audience projection over the same artifact graph.
func ProjectionResponse(graph artifact.ProjectionGraph, view artifact.ProjectionView) string {
	switch view {
	case artifact.ProjectionViewEngineer:
		return engineerProjectionResponse(graph)
	case artifact.ProjectionViewManager:
		return managerProjectionResponse(graph)
	case artifact.ProjectionViewAudit:
		return auditProjectionResponse(graph)
	case artifact.ProjectionViewCompare:
		return compareProjectionResponse(graph)
	default:
		return fmt.Sprintf("Unsupported projection view: %s\n", view)
	}
}

func engineerProjectionResponse(graph artifact.ProjectionGraph) string {
	if projectionGraphEmpty(graph) {
		return "No active artifacts available for the engineer projection.\n"
	}

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
				sb.WriteString(fmt.Sprintf("Portfolios: %s\n", strings.Join(problem.PortfolioRefs, ", ")))
			}
			if len(problem.DecisionRefs) > 0 {
				sb.WriteString(fmt.Sprintf("Decisions: %s\n", strings.Join(problem.DecisionRefs, ", ")))
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
				sb.WriteString(fmt.Sprintf("Problems: %s\n", strings.Join(portfolio.ProblemRefs, ", ")))
			}
			if len(portfolio.DecisionRefs) > 0 {
				sb.WriteString(fmt.Sprintf("Decisions: %s\n", strings.Join(portfolio.DecisionRefs, ", ")))
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
				sb.WriteString(fmt.Sprintf("Problems: %s\n", strings.Join(decision.ProblemRefs, ", ")))
			}
			if len(decision.PortfolioRefs) > 0 {
				sb.WriteString(fmt.Sprintf("Portfolios: %s\n", strings.Join(decision.PortfolioRefs, ", ")))
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
	pending, shipped, refreshDue := projectionDecisionStages(graph.Decisions)
	compared := 0
	for _, portfolio := range graph.Portfolios {
		if portfolio.Comparison != nil {
			compared++
		}
	}

	var sb strings.Builder
	sb.WriteString("## Manager/Status View\n\n")
	sb.WriteString(fmt.Sprintf("Problems: %d backlog, %d in progress, %d addressed\n", backlog, inProgress, addressed))
	sb.WriteString(fmt.Sprintf("Decisions: %d pending follow-through, %d measured/shipped, %d refresh due\n", pending, shipped, refreshDue))
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
			statusParts := []string{}
			if decision.Measured {
				statusParts = append(statusParts, "measured")
			} else {
				statusParts = append(statusParts, "waiting for measurement")
			}
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
			sb.WriteString(fmt.Sprintf("Assurance: R_eff=%.2f | F_eff=%d | weakest CL=%d\n", decision.Evidence.WLNK.REff, decision.Evidence.WLNK.FEff, decision.Evidence.WLNK.WeakestCL))
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

func compareProjectionResponse(graph artifact.ProjectionGraph) string {
	comparedPortfolios := make([]artifact.PortfolioProjection, 0, len(graph.Portfolios))
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
			sb.WriteString(fmt.Sprintf("Problems: %s\n", strings.Join(portfolio.ProblemRefs, ", ")))
		}
		if len(portfolio.DecisionRefs) > 0 {
			sb.WriteString(fmt.Sprintf("Decisions: %s\n", strings.Join(portfolio.DecisionRefs, ", ")))
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

func projectionDecisionStages(decisions []artifact.DecisionProjection) (pending int, shipped int, refreshDue int) {
	for _, decision := range decisions {
		if decision.Measured {
			shipped++
		} else {
			pending++
		}
		if decision.NeedsRefresh {
			refreshDue++
		}
	}

	return pending, shipped, refreshDue
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
