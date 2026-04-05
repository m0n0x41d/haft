package artifact

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ProjectionView selects an audience-specific rendering over the same artifact graph.
type ProjectionView string

const (
	ProjectionViewEngineer ProjectionView = "engineer"
	ProjectionViewManager  ProjectionView = "manager"
	ProjectionViewAudit    ProjectionView = "audit"
	ProjectionViewCompare  ProjectionView = "compare"
)

// ParseProjectionView normalizes supported aliases into one canonical view name.
func ParseProjectionView(raw string) (ProjectionView, error) {
	normalized := strings.TrimSpace(strings.ToLower(raw))

	switch normalized {
	case "", string(ProjectionViewEngineer):
		return ProjectionViewEngineer, nil
	case "manager", "status", "manager/status":
		return ProjectionViewManager, nil
	case "audit", "evidence", "audit/evidence":
		return ProjectionViewAudit, nil
	case "compare", "pareto", "compare/pareto":
		return ProjectionViewCompare, nil
	default:
		return "", fmt.Errorf("invalid projection view %q (valid: engineer, manager, audit, compare)", raw)
	}
}

// ProjectionGraph is a deterministic snapshot of the active problem/portfolio/decision graph.
// Different views render this same graph without adding new semantics.
type ProjectionGraph struct {
	Context     string
	GeneratedAt time.Time
	Problems    []ProblemProjection
	Portfolios  []PortfolioProjection
	Decisions   []DecisionProjection
}

// ProjectionEvidenceSummary is a compact, projection-friendly evidence rollup.
type ProjectionEvidenceSummary struct {
	MeasurementCount int
	WLNK             WLNKSummary
}

// ProblemProjection is the graph node used for audience-specific rendering.
type ProblemProjection struct {
	Meta                  Meta
	NeedsRefresh          bool
	Signal                string
	Acceptance            string
	Constraints           []string
	OptimizationTargets   []string
	ObservationIndicators []string
	CharacterizationCount int
	PortfolioRefs         []string
	DecisionRefs          []string
	Evidence              ProjectionEvidenceSummary
}

// PortfolioProjection is the graph node used for audience-specific rendering.
type PortfolioProjection struct {
	Meta                     Meta
	NeedsRefresh             bool
	ProblemRefs              []string
	DecisionRefs             []string
	Variants                 []Variant
	Comparison               *ComparisonResult
	NoSteppingStoneRationale string
	Evidence                 ProjectionEvidenceSummary
}

// DecisionProjection is the graph node used for audience-specific rendering.
type DecisionProjection struct {
	Meta             Meta
	NeedsRefresh     bool
	ProblemRefs      []string
	PortfolioRefs    []string
	SelectedTitle    string
	WhySelected      string
	SelectionPolicy  string
	CounterArgument  string
	WeakestLink      string
	WhyNotOthers     []RejectionReason
	RollbackTriggers []string
	Evidence         ProjectionEvidenceSummary
	Measured         bool
}

// FetchProjectionGraph builds a deterministic graph snapshot that can be rendered
// into multiple view-only projections.
func FetchProjectionGraph(ctx context.Context, store ArtifactStore, contextName string) (ProjectionGraph, error) {
	now := time.Now().UTC()
	problems, err := loadProjectionArtifacts(ctx, store, KindProblemCard, contextName)
	if err != nil {
		return ProjectionGraph{}, err
	}

	portfolios, err := loadProjectionArtifacts(ctx, store, KindSolutionPortfolio, contextName)
	if err != nil {
		return ProjectionGraph{}, err
	}

	decisions, err := loadProjectionArtifacts(ctx, store, KindDecisionRecord, contextName)
	if err != nil {
		return ProjectionGraph{}, err
	}

	graph := ProjectionGraph{
		Context:     strings.TrimSpace(contextName),
		GeneratedAt: now,
		Problems:    make([]ProblemProjection, 0, len(problems)),
		Portfolios:  make([]PortfolioProjection, 0, len(portfolios)),
		Decisions:   make([]DecisionProjection, 0, len(decisions)),
	}

	problemIDs := make(map[string]struct{}, len(problems))
	portfolioIDs := make(map[string]struct{}, len(portfolios))
	portfolioProblemRefs := make(map[string][]string, len(portfolios))

	for _, problem := range problems {
		problemIDs[problem.Meta.ID] = struct{}{}
		fields := problem.UnmarshalProblemFields()

		graph.Problems = append(graph.Problems, ProblemProjection{
			Meta:                  problem.Meta,
			NeedsRefresh:          projectionNeedsRefresh(problem.Meta, now),
			Signal:                coalesceProjectionProblemField(fields.Signal, extractSection(problem.Body, "Signal")),
			Acceptance:            coalesceProjectionProblemField(fields.Acceptance, extractSection(problem.Body, "Acceptance")),
			Constraints:           cloneStringSlice(fields.Constraints),
			OptimizationTargets:   cloneStringSlice(fields.OptimizationTargets),
			ObservationIndicators: cloneStringSlice(fields.ObservationIndicators),
			CharacterizationCount: maxInt(len(fields.Characterizations), countCharacterizations(problem)),
			Evidence:              buildProjectionEvidenceSummary(ctx, store, problem.Meta.ID),
		})
	}

	for _, portfolio := range portfolios {
		portfolioIDs[portfolio.Meta.ID] = struct{}{}
		fields := portfolio.UnmarshalPortfolioFields()
		problemRefs := projectionLinkRefs(portfolio.Meta.Links, problemIDs)

		portfolioProblemRefs[portfolio.Meta.ID] = cloneStringSlice(problemRefs)

		graph.Portfolios = append(graph.Portfolios, PortfolioProjection{
			Meta:                     portfolio.Meta,
			NeedsRefresh:             projectionNeedsRefresh(portfolio.Meta, now),
			ProblemRefs:              problemRefs,
			Variants:                 cloneVariants(fields.Variants),
			Comparison:               cloneProjectionComparisonResult(fields.Comparison),
			NoSteppingStoneRationale: strings.TrimSpace(fields.NoSteppingStoneRationale),
			Evidence:                 buildProjectionEvidenceSummary(ctx, store, portfolio.Meta.ID),
		})
	}

	for _, decision := range decisions {
		fields := decision.UnmarshalDecisionFields()
		portfolioRefs := projectionLinkRefs(decision.Meta.Links, portfolioIDs)
		problemRefs := projectionDecisionProblemRefs(
			projectionLinkRefs(decision.Meta.Links, problemIDs),
			portfolioRefs,
			portfolioProblemRefs,
		)

		graph.Decisions = append(graph.Decisions, DecisionProjection{
			Meta:             decision.Meta,
			NeedsRefresh:     projectionNeedsRefresh(decision.Meta, now),
			ProblemRefs:      problemRefs,
			PortfolioRefs:    portfolioRefs,
			SelectedTitle:    strings.TrimSpace(fields.SelectedTitle),
			WhySelected:      strings.TrimSpace(fields.WhySelected),
			SelectionPolicy:  strings.TrimSpace(fields.SelectionPolicy),
			CounterArgument:  strings.TrimSpace(fields.CounterArgument),
			WeakestLink:      strings.TrimSpace(fields.WeakestLink),
			WhyNotOthers:     cloneRejectionReasons(fields.WhyNotOthers),
			RollbackTriggers: cloneStringSlice(fields.RollbackTriggers),
			Evidence:         buildProjectionEvidenceSummary(ctx, store, decision.Meta.ID),
			Measured:         hasMeasurement(ctx, store, decision.Meta.ID),
		})
	}

	attachProjectionBacklinks(&graph)
	sortProjectionGraph(&graph)

	return graph, nil
}

func loadProjectionArtifacts(ctx context.Context, store ArtifactStore, kind Kind, contextName string) ([]*Artifact, error) {
	var (
		candidates []*Artifact
		err        error
	)

	contextName = strings.TrimSpace(contextName)
	if contextName != "" {
		all, listErr := store.ListByContext(ctx, contextName)
		if listErr != nil {
			return nil, listErr
		}

		for _, item := range all {
			if item.Meta.Kind != kind || !projectionIncludesStatus(item.Meta.Status) {
				continue
			}

			candidates = append(candidates, item)
		}
	} else {
		candidates, err = store.ListByKind(ctx, kind, 0)
		if err != nil {
			return nil, err
		}
	}

	result := make([]*Artifact, 0, len(candidates))
	for _, candidate := range candidates {
		if !projectionIncludesStatus(candidate.Meta.Status) {
			continue
		}

		full, getErr := store.Get(ctx, candidate.Meta.ID)
		if getErr != nil {
			return nil, getErr
		}
		if !projectionIncludesStatus(full.Meta.Status) {
			continue
		}

		result = append(result, full)
	}

	return result, nil
}

func projectionIncludesStatus(status Status) bool {
	return status != StatusSuperseded && status != StatusDeprecated
}

func projectionNeedsRefresh(meta Meta, now time.Time) bool {
	if meta.Status == StatusRefreshDue {
		return true
	}

	return isExpiredValidUntil(meta.ValidUntil, now)
}

func buildProjectionEvidenceSummary(ctx context.Context, store ArtifactStore, artifactID string) ProjectionEvidenceSummary {
	items, err := store.GetEvidenceItems(ctx, artifactID)
	if err != nil {
		return ProjectionEvidenceSummary{WLNK: ComputeWLNKSummary(ctx, store, artifactID)}
	}

	activeItems := make([]EvidenceItem, 0, len(items))
	for _, item := range items {
		if item.Verdict == "superseded" {
			continue
		}

		activeItems = append(activeItems, item)
	}

	measurementCount := 0
	for _, item := range activeItems {
		if item.Type == "measurement" {
			measurementCount++
		}
	}

	return ProjectionEvidenceSummary{
		MeasurementCount: measurementCount,
		WLNK:             ComputeWLNKSummary(ctx, store, artifactID),
	}
}

func projectionLinkRefs(links []Link, allowed map[string]struct{}) []string {
	refs := make([]string, 0, len(links))
	seen := make(map[string]struct{}, len(links))

	for _, link := range links {
		if link.Type != "based_on" {
			continue
		}
		if _, ok := allowed[link.Ref]; !ok {
			continue
		}
		if _, ok := seen[link.Ref]; ok {
			continue
		}

		seen[link.Ref] = struct{}{}
		refs = append(refs, link.Ref)
	}

	sort.Strings(refs)
	return refs
}

func projectionDecisionProblemRefs(
	directProblemRefs []string,
	portfolioRefs []string,
	portfolioProblemRefs map[string][]string,
) []string {
	resolvedRefs := cloneStringSlice(directProblemRefs)

	for _, portfolioRef := range portfolioRefs {
		problemRefs := portfolioProblemRefs[portfolioRef]

		for _, problemRef := range problemRefs {
			resolvedRefs = appendUniqueString(resolvedRefs, problemRef)
		}
	}

	sort.Strings(resolvedRefs)
	return resolvedRefs
}

func attachProjectionBacklinks(graph *ProjectionGraph) {
	problemIndex := make(map[string]int, len(graph.Problems))
	portfolioIndex := make(map[string]int, len(graph.Portfolios))

	for index, problem := range graph.Problems {
		problemIndex[problem.Meta.ID] = index
	}
	for index, portfolio := range graph.Portfolios {
		portfolioIndex[portfolio.Meta.ID] = index
	}

	for _, portfolio := range graph.Portfolios {
		for _, problemRef := range portfolio.ProblemRefs {
			index, ok := problemIndex[problemRef]
			if !ok {
				continue
			}

			graph.Problems[index].PortfolioRefs = appendUniqueString(graph.Problems[index].PortfolioRefs, portfolio.Meta.ID)
		}
	}

	for _, decision := range graph.Decisions {
		for _, problemRef := range decision.ProblemRefs {
			index, ok := problemIndex[problemRef]
			if !ok {
				continue
			}

			graph.Problems[index].DecisionRefs = appendUniqueString(graph.Problems[index].DecisionRefs, decision.Meta.ID)
		}

		for _, portfolioRef := range decision.PortfolioRefs {
			index, ok := portfolioIndex[portfolioRef]
			if !ok {
				continue
			}

			graph.Portfolios[index].DecisionRefs = appendUniqueString(graph.Portfolios[index].DecisionRefs, decision.Meta.ID)
		}
	}

	for index := range graph.Problems {
		sort.Strings(graph.Problems[index].PortfolioRefs)
		sort.Strings(graph.Problems[index].DecisionRefs)
	}
	for index := range graph.Portfolios {
		sort.Strings(graph.Portfolios[index].DecisionRefs)
	}
}

func sortProjectionGraph(graph *ProjectionGraph) {
	sort.SliceStable(graph.Problems, func(left, right int) bool {
		return graph.Problems[left].Meta.ID < graph.Problems[right].Meta.ID
	})
	sort.SliceStable(graph.Portfolios, func(left, right int) bool {
		return graph.Portfolios[left].Meta.ID < graph.Portfolios[right].Meta.ID
	})
	sort.SliceStable(graph.Decisions, func(left, right int) bool {
		return graph.Decisions[left].Meta.ID < graph.Decisions[right].Meta.ID
	})
}

func coalesceProjectionProblemField(primary string, fallback string) string {
	if trimmed := strings.TrimSpace(primary); trimmed != "" {
		return trimmed
	}

	return strings.TrimSpace(fallback)
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneRejectionReasons(values []RejectionReason) []RejectionReason {
	if len(values) == 0 {
		return nil
	}

	cloned := make([]RejectionReason, len(values))
	copy(cloned, values)
	return cloned
}

func cloneProjectionComparisonResult(input *ComparisonResult) *ComparisonResult {
	if input == nil {
		return nil
	}

	return cloneComparisonResult(*input)
}

func appendUniqueString(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}

	return append(values, candidate)
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
