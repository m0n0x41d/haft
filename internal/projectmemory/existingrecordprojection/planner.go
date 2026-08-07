// Package existingrecordprojection plans typed-memory projection routes for
// already persisted project artifacts. It is a pure inventory boundary: it
// does not expose a public action, construct adapter candidates, or write.
package existingrecordprojection

import (
	"fmt"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// Projection identifies one selected task-oriented typed-memory adapter result.
type Projection string

const (
	ProjectionNoteAtConcern                Projection = "Haft.NoteAtConcern"
	ProjectionProblemCardAtConcern         Projection = "Haft.ProblemCardAtConcern"
	ProjectionSolutionPortfolioAtConcern   Projection = "Haft.SolutionPortfolioAtConcern"
	ProjectionPortfolioComparisonAtConcern Projection = "Haft.PortfolioComparisonAtConcern"
	ProjectionDecisionChoiceAtConcern      Projection = "Haft.DecisionChoiceAtConcern"
)

// Requirement is an exact prerequisite that an executor must resolve before
// it constructs the selected adapter candidate.
type Requirement string

const (
	RequirementExactConcern               Requirement = "exact_entity_of_concern_and_bounded_context"
	RequirementProjectedSolutionPortfolio Requirement = "projected_solution_portfolio_basis"
)

// DeferredReason explains why an existing artifact cannot be routed through
// one of the currently selected task-oriented adapters.
type DeferredReason string

const (
	DeferredAuthorityCarrierIsNotPerformedWork DeferredReason = "authority_carrier_is_not_performed_work"
	DeferredEvidenceCarrierNeedsWorkSource     DeferredReason = "evidence_carrier_needs_exact_work_and_evidence_source"
	DeferredMethodRunIsNotPerformedWork        DeferredReason = "method_run_is_not_performed_work"
	DeferredNoSelectedTaskAdapter              DeferredReason = "no_selected_task_adapter"
)

// Route is a closed supported projection route. Its private fields prevent
// callers from constructing an artifact/projection combination the planner
// does not own.
type Route struct {
	artifactRef     string
	artifactKind    artifact.Kind
	artifactVersion int
	projection      Projection
	requirements    []Requirement
}

// ArtifactRef returns the exact persisted carrier identity.
func (route Route) ArtifactRef() string {
	return route.artifactRef
}

// ArtifactKind returns the persisted carrier kind.
func (route Route) ArtifactKind() artifact.Kind {
	return route.artifactKind
}

// ArtifactVersion returns the persisted carrier version.
func (route Route) ArtifactVersion() int {
	return route.artifactVersion
}

// Projection returns the selected typed-memory projection kind.
func (route Route) Projection() Projection {
	return route.projection
}

// Requirements returns a defensive copy of the route prerequisites.
func (route Route) Requirements() []Requirement {
	return slices.Clone(route.requirements)
}

// Deferred is a closed unsupported projection result. It preserves the source
// artifact identity without reinterpreting its meaning.
type Deferred struct {
	artifactRef     string
	artifactKind    artifact.Kind
	artifactVersion int
	reason          DeferredReason
}

// ArtifactRef returns the exact persisted carrier identity.
func (deferred Deferred) ArtifactRef() string {
	return deferred.artifactRef
}

// ArtifactKind returns the persisted carrier kind.
func (deferred Deferred) ArtifactKind() artifact.Kind {
	return deferred.artifactKind
}

// ArtifactVersion returns the persisted carrier version.
func (deferred Deferred) ArtifactVersion() int {
	return deferred.artifactVersion
}

// Reason returns the explicit non-projection reason.
func (deferred Deferred) Reason() DeferredReason {
	return deferred.reason
}

// Plan partitions one exact artifact inventory into supported and deferred
// routes.
type Plan struct {
	routes   []Route
	deferred []Deferred
}

// Routes returns supported routes in dependency order.
func (plan Plan) Routes() []Route {
	return slices.Clone(plan.routes)
}

// Deferred returns unsupported carriers in stable artifact-ref order.
func (plan Plan) Deferred() []Deferred {
	return slices.Clone(plan.deferred)
}

type routeFactory func(*artifact.Artifact) []Route

var supportedRoutes = map[artifact.Kind]routeFactory{
	artifact.KindNote:              noteRoutes,
	artifact.KindProblemCard:       problemRoutes,
	artifact.KindSolutionPortfolio: solutionRoutes,
	artifact.KindDecisionRecord:    decisionRoutes,
}

var deferredReasons = map[artifact.Kind]DeferredReason{
	artifact.KindWorkCommission: DeferredAuthorityCarrierIsNotPerformedWork,
	artifact.KindMethodRun:      DeferredMethodRunIsNotPerformedWork,
	artifact.KindEvidencePack:   DeferredEvidenceCarrierNeedsWorkSource,
	artifact.KindRefreshReport:  DeferredNoSelectedTaskAdapter,
}

var projectionOrder = map[Projection]int{
	ProjectionNoteAtConcern:                10,
	ProjectionProblemCardAtConcern:         20,
	ProjectionSolutionPortfolioAtConcern:   30,
	ProjectionPortfolioComparisonAtConcern: 40,
	ProjectionDecisionChoiceAtConcern:      50,
}

// Build constructs a deterministic, write-free projection plan.
func Build(records []*artifact.Artifact) (Plan, error) {
	if err := validateInventory(records); err != nil {
		return Plan{}, err
	}
	ordered := slices.Clone(records)
	slices.SortFunc(ordered, compareArtifacts)
	routes, deferred := classifyRecords(ordered)
	slices.SortFunc(routes, compareRoutes)
	slices.SortFunc(deferred, compareDeferred)
	return Plan{
		routes:   routes,
		deferred: deferred,
	}, nil
}

func validateInventory(records []*artifact.Artifact) error {
	seen := make(map[string]struct{}, len(records))
	for index, record := range records {
		if record == nil {
			return fmt.Errorf(
				"plan existing-record projection: record %d is absent",
				index,
			)
		}
		ref := strings.TrimSpace(record.Meta.ID)
		if ref == "" || ref != record.Meta.ID {
			return fmt.Errorf(
				"plan existing-record projection: record %d has no exact identity",
				index,
			)
		}
		if !record.Meta.Kind.IsValid() {
			return fmt.Errorf(
				"plan existing-record projection: artifact %s has unsupported kind %q",
				ref,
				record.Meta.Kind,
			)
		}
		if _, duplicate := seen[ref]; duplicate {
			return fmt.Errorf(
				"plan existing-record projection: duplicate artifact identity %s",
				ref,
			)
		}
		seen[ref] = struct{}{}
	}
	return nil
}

func classifyRecords(
	records []*artifact.Artifact,
) ([]Route, []Deferred) {
	routes := make([]Route, 0, len(records))
	deferred := make([]Deferred, 0)
	for _, record := range records {
		factory, supported := supportedRoutes[record.Meta.Kind]
		if supported {
			routes = append(routes, factory(record)...)
			continue
		}
		deferred = append(
			deferred,
			newDeferred(
				record,
				deferredReasons[record.Meta.Kind],
			),
		)
	}
	return routes, deferred
}

func noteRoutes(record *artifact.Artifact) []Route {
	return []Route{newRoute(
		record,
		ProjectionNoteAtConcern,
		[]Requirement{RequirementExactConcern},
	)}
}

func problemRoutes(record *artifact.Artifact) []Route {
	return []Route{newRoute(
		record,
		ProjectionProblemCardAtConcern,
		[]Requirement{RequirementExactConcern},
	)}
}

func solutionRoutes(record *artifact.Artifact) []Route {
	routes := []Route{newRoute(
		record,
		ProjectionSolutionPortfolioAtConcern,
		[]Requirement{RequirementExactConcern},
	)}
	if !artifact.PortfolioHasComparison(record) {
		return routes
	}
	comparison := newRoute(
		record,
		ProjectionPortfolioComparisonAtConcern,
		[]Requirement{
			RequirementExactConcern,
			RequirementProjectedSolutionPortfolio,
		},
	)
	return append(routes, comparison)
}

func decisionRoutes(record *artifact.Artifact) []Route {
	return []Route{newRoute(
		record,
		ProjectionDecisionChoiceAtConcern,
		[]Requirement{RequirementProjectedSolutionPortfolio},
	)}
}

func newRoute(
	record *artifact.Artifact,
	projection Projection,
	requirements []Requirement,
) Route {
	return Route{
		artifactRef:     record.Meta.ID,
		artifactKind:    record.Meta.Kind,
		artifactVersion: record.Meta.Version,
		projection:      projection,
		requirements:    slices.Clone(requirements),
	}
}

func newDeferred(
	record *artifact.Artifact,
	reason DeferredReason,
) Deferred {
	return Deferred{
		artifactRef:     record.Meta.ID,
		artifactKind:    record.Meta.Kind,
		artifactVersion: record.Meta.Version,
		reason:          reason,
	}
}

func compareArtifacts(
	left *artifact.Artifact,
	right *artifact.Artifact,
) int {
	return strings.Compare(left.Meta.ID, right.Meta.ID)
}

func compareRoutes(left Route, right Route) int {
	leftOrder := projectionOrder[left.projection]
	rightOrder := projectionOrder[right.projection]
	if leftOrder != rightOrder {
		return leftOrder - rightOrder
	}
	if byRef := strings.Compare(
		left.artifactRef,
		right.artifactRef,
	); byRef != 0 {
		return byRef
	}
	return strings.Compare(
		string(left.projection),
		string(right.projection),
	)
}

func compareDeferred(left Deferred, right Deferred) int {
	return strings.Compare(left.artifactRef, right.artifactRef)
}
