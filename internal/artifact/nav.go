package artifact

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

type projectStateInput struct {
	Context        string
	Artifacts      []*Artifact
	ArtifactsKnown bool
	StaleDecisions []*Artifact
	EvidenceKnown  bool
	Commissions    []WorkCommissionStatus
	WorkKnown      bool
	Now            time.Time
}

// ComputeProjectStateView reads the available governance facts and projects
// them as independent facets. It never derives a project phase or a global
// next action from artifact presence.
func ComputeProjectStateView(
	ctx context.Context,
	store ArtifactStore,
	contextName string,
) ProjectStateView {
	artifacts, artifactsKnown := loadProjectStateArtifacts(ctx, store, contextName)
	staleDecisions, evidenceKnown := loadProjectStateStaleDecisions(ctx, store)
	commissions, workKnown := loadProjectStateCommissions(ctx, store)

	input := projectStateInput{
		Context:        contextName,
		Artifacts:      artifacts,
		ArtifactsKnown: artifactsKnown,
		StaleDecisions: staleDecisions,
		EvidenceKnown:  evidenceKnown,
		Commissions:    commissions,
		WorkKnown:      workKnown,
		Now:            time.Now().UTC(),
	}

	return deriveProjectStateView(input)
}

// ComputeNavState is the compatibility entrypoint used by existing shells.
// Its result is a ProjectStateView; the former phase/navigation model no
// longer exists.
func ComputeNavState(
	ctx context.Context,
	store ArtifactStore,
	contextName string,
) ProjectStateView {
	return ComputeProjectStateView(ctx, store, contextName)
}

func loadProjectStateArtifacts(
	ctx context.Context,
	store ArtifactStore,
	contextName string,
) ([]*Artifact, bool) {
	items, err := listProjectStateArtifacts(ctx, store, contextName)
	if err != nil {
		return []*Artifact{}, false
	}

	hydrated := hydrateProjectStateOptionSets(ctx, store, items)
	return hydrated, true
}

func listProjectStateArtifacts(
	ctx context.Context,
	store ArtifactStore,
	contextName string,
) ([]*Artifact, error) {
	if contextName != "" {
		return store.ListByContext(ctx, contextName)
	}

	return store.ListActive(ctx, 0)
}

func hydrateProjectStateOptionSets(
	ctx context.Context,
	store ArtifactStore,
	items []*Artifact,
) []*Artifact {
	hydrated := make([]*Artifact, 0, len(items))
	for _, item := range items {
		if item.Meta.Kind != KindSolutionPortfolio {
			hydrated = append(hydrated, item)
			continue
		}

		full, err := store.Get(ctx, item.Meta.ID)
		if err != nil {
			hydrated = append(hydrated, item)
			continue
		}

		hydrated = append(hydrated, full)
	}

	return hydrated
}

func loadProjectStateStaleDecisions(
	ctx context.Context,
	store ArtifactStore,
) ([]*Artifact, bool) {
	items, err := store.FindStaleDecisions(ctx)
	if err != nil {
		return []*Artifact{}, false
	}

	return items, true
}

func loadProjectStateCommissions(
	ctx context.Context,
	store ArtifactStore,
) ([]WorkCommissionStatus, bool) {
	items, err := FetchWorkCommissionStatuses(ctx, store)
	if err != nil {
		return []WorkCommissionStatus{}, false
	}

	return items, true
}

func deriveProjectStateView(input projectStateInput) ProjectStateView {
	view := emptyProjectStateView(input)
	collectProjectArtifactFacets(&view, input.Artifacts)
	collectProjectEvidencePressure(&view, input)
	collectProjectWorkFacet(&view, input)
	sortProjectStateView(&view)
	return view
}

func emptyProjectStateView(input projectStateInput) ProjectStateView {
	return ProjectStateView{
		Context: input.Context,
		Problems: ProjectProblemFacet{
			Known: input.ArtifactsKnown,
			Open:  []ProjectArtifactState{},
		},
		Options: ProjectOptionFacet{
			Known: input.ArtifactsKnown,
			Sets:  []ProjectOptionSetState{},
		},
		Decisions: ProjectDecisionFacet{
			Known:  input.ArtifactsKnown,
			Active: []ProjectArtifactState{},
		},
		Work: ProjectWorkFacet{
			Known:  input.WorkKnown,
			Active: []WorkCommissionStatus{},
		},
		ProjectPressureFacet: ProjectPressureFacet{
			EvidenceKnown: input.EvidenceKnown,
			StaleItems:    []string{},
			DriftItems:    []string{},
		},
		SpecHealth: ProjectSpecHealthFacet{
			Findings: []string{},
		},
	}
}

func collectProjectArtifactFacets(view *ProjectStateView, items []*Artifact) {
	collectors := map[Kind]func(*ProjectStateView, *Artifact){
		KindProblemCard:       collectProjectProblem,
		KindSolutionPortfolio: collectProjectOptionSet,
		KindDecisionRecord:    collectProjectDecision,
	}

	for _, item := range items {
		if !projectStateArtifactCurrent(item) {
			continue
		}

		collector, found := collectors[item.Meta.Kind]
		if !found {
			continue
		}

		collector(view, item)
	}
}

func collectProjectProblem(view *ProjectStateView, item *Artifact) {
	state := projectArtifactState(item)
	view.Problems.Open = append(view.Problems.Open, state)
}

func collectProjectOptionSet(view *ProjectStateView, item *Artifact) {
	state := ProjectOptionSetState{
		Artifact:           projectArtifactState(item),
		ComparisonRecorded: PortfolioHasComparison(item),
	}
	view.Options.Sets = append(view.Options.Sets, state)
}

func collectProjectDecision(view *ProjectStateView, item *Artifact) {
	state := projectArtifactState(item)
	view.Decisions.Active = append(view.Decisions.Active, state)
}

func projectArtifactState(item *Artifact) ProjectArtifactState {
	return ProjectArtifactState{
		ID:     item.Meta.ID,
		Title:  item.Meta.Title,
		Status: item.Meta.Status,
		Mode:   item.Meta.Mode,
	}
}

func projectStateArtifactCurrent(item *Artifact) bool {
	if item == nil {
		return false
	}

	return item.Meta.Status == StatusActive || item.Meta.Status == StatusRefreshDue
}

func collectProjectEvidencePressure(view *ProjectStateView, input projectStateInput) {
	for _, item := range input.StaleDecisions {
		if !projectStateArtifactCurrent(item) {
			continue
		}
		if !projectStateArtifactInContext(item, input.Context) {
			continue
		}

		reason := projectStateStaleReason(item, input.Now)
		label := fmt.Sprintf("%s: %s (%s)", item.Meta.ID, item.Meta.Title, reason)
		view.StaleItems = append(view.StaleItems, label)
	}

	view.StaleCount = len(view.StaleItems)
}

func projectStateArtifactInContext(item *Artifact, contextName string) bool {
	if contextName == "" {
		return true
	}

	return item.Meta.Context == contextName
}

func projectStateStaleReason(item *Artifact, now time.Time) string {
	reason := "refresh_due"
	expiry, parsed := reff.ParseValidUntil(item.Meta.ValidUntil)
	if !parsed {
		return reason
	}
	if !expiry.Before(now) {
		return reason
	}

	return fmt.Sprintf("expired %s", expiry.Format("2006-01-02"))
}

func collectProjectWorkFacet(view *ProjectStateView, input projectStateInput) {
	scope := projectStateScope(input.Artifacts)
	for _, commission := range input.Commissions {
		if commission.Terminal {
			continue
		}
		if !projectStateCommissionInScope(commission, input.Context, scope) {
			continue
		}

		view.Work.Active = append(view.Work.Active, commission)
	}
}

func projectStateScope(items []*Artifact) map[string]struct{} {
	scope := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}

		scope[item.Meta.ID] = struct{}{}
	}

	return scope
}

func projectStateCommissionInScope(
	commission WorkCommissionStatus,
	contextName string,
	scope map[string]struct{},
) bool {
	if contextName == "" {
		return true
	}

	_, commissionInScope := scope[commission.ID]
	_, decisionInScope := scope[commission.DecisionRef]
	return commissionInScope || decisionInScope
}

func sortProjectStateView(view *ProjectStateView) {
	slices.SortFunc(view.Problems.Open, compareProjectArtifactState)
	slices.SortFunc(view.Options.Sets, compareProjectOptionSetState)
	slices.SortFunc(view.Decisions.Active, compareProjectArtifactState)
	slices.SortFunc(view.Work.Active, compareProjectWorkState)
	slices.Sort(view.StaleItems)
	slices.Sort(view.DriftItems)
	slices.Sort(view.SpecHealth.Findings)
}

func compareProjectArtifactState(left ProjectArtifactState, right ProjectArtifactState) int {
	return cmp.Compare(left.ID, right.ID)
}

func compareProjectOptionSetState(left ProjectOptionSetState, right ProjectOptionSetState) int {
	return cmp.Compare(left.Artifact.ID, right.Artifact.ID)
}

func compareProjectWorkState(left WorkCommissionStatus, right WorkCommissionStatus) int {
	return cmp.Compare(left.ID, right.ID)
}
