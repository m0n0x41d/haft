package existingrecordprojection_test

import (
	"reflect"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/projectmemory/existingrecordprojection"
)

func TestBuildOrdersSupportedRoutesByDependency(t *testing.T) {
	t.Parallel()

	records := []*artifact.Artifact{
		existingRecord(
			"dec-existing",
			artifact.KindDecisionRecord,
			"",
		),
		existingRecord(
			"sol-existing",
			artifact.KindSolutionPortfolio,
			`{"comparison":{"scores":{"option-a":{"latency":"10ms"}}}}`,
		),
		existingRecord(
			"note-existing",
			artifact.KindNote,
			"",
		),
		existingRecord(
			"prob-existing",
			artifact.KindProblemCard,
			"",
		),
	}

	plan, err := existingrecordprojection.Build(records)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	got := projectableRouteSummaries(plan.Routes())
	want := []routeSummary{
		{
			ref:        "note-existing",
			projection: existingrecordprojection.ProjectionNoteAtConcern,
			requirements: []existingrecordprojection.Requirement{
				existingrecordprojection.RequirementExactConcern,
			},
		},
		{
			ref:        "prob-existing",
			projection: existingrecordprojection.ProjectionProblemCardAtConcern,
			requirements: []existingrecordprojection.Requirement{
				existingrecordprojection.RequirementExactConcern,
			},
		},
		{
			ref:        "sol-existing",
			projection: existingrecordprojection.ProjectionSolutionPortfolioAtConcern,
			requirements: []existingrecordprojection.Requirement{
				existingrecordprojection.RequirementExactConcern,
			},
		},
		{
			ref:        "sol-existing",
			projection: existingrecordprojection.ProjectionPortfolioComparisonAtConcern,
			requirements: []existingrecordprojection.Requirement{
				existingrecordprojection.RequirementExactConcern,
				existingrecordprojection.RequirementProjectedSolutionPortfolio,
			},
		},
		{
			ref:        "dec-existing",
			projection: existingrecordprojection.ProjectionDecisionChoiceAtConcern,
			requirements: []existingrecordprojection.Requirement{
				existingrecordprojection.RequirementProjectedSolutionPortfolio,
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes = %#v, want %#v", got, want)
	}
	if len(plan.Deferred()) != 0 {
		t.Fatalf("deferred = %#v, want none", plan.Deferred())
	}
}

func TestBuildKeepsUnsupportedCarrierMeaningsExplicit(t *testing.T) {
	t.Parallel()

	records := []*artifact.Artifact{
		existingRecord(
			"refresh-existing",
			artifact.KindRefreshReport,
			"",
		),
		existingRecord(
			"method-existing",
			artifact.KindMethodRun,
			"",
		),
		existingRecord(
			"evidence-existing",
			artifact.KindEvidencePack,
			"",
		),
		existingRecord(
			"commission-existing",
			artifact.KindWorkCommission,
			"",
		),
	}

	plan, err := existingrecordprojection.Build(records)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if len(plan.Routes()) != 0 {
		t.Fatalf("routes = %#v, want none", plan.Routes())
	}
	got := deferredRouteSummaries(plan.Deferred())
	want := []deferredSummary{
		{
			ref:    "commission-existing",
			reason: existingrecordprojection.DeferredAuthorityCarrierIsNotPerformedWork,
		},
		{
			ref:    "evidence-existing",
			reason: existingrecordprojection.DeferredEvidenceCarrierNeedsWorkSource,
		},
		{
			ref:    "method-existing",
			reason: existingrecordprojection.DeferredMethodRunIsNotPerformedWork,
		},
		{
			ref:    "refresh-existing",
			reason: existingrecordprojection.DeferredNoSelectedTaskAdapter,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deferred = %#v, want %#v", got, want)
	}
}

func TestBuildDoesNotPromoteRichLegacyCarriersToEvidenceWorkOrCodeAnchors(
	t *testing.T,
) {
	t.Parallel()

	records := []*artifact.Artifact{
		existingRecord(
			"commission-completed",
			artifact.KindWorkCommission,
			`{"status":"completed","runner_id":"runner-1","affected_files":["internal/projectmemory/runtime.go"]}`,
		),
		existingRecord(
			"evidence-passed",
			artifact.KindEvidencePack,
			`{"verdict":"pass","claim_ref":"claim-1","work_ref":"work-1"}`,
		),
		existingRecord(
			"method-closed",
			artifact.KindMethodRun,
			`{"status":"closed","verification":{"result":"pass"},"changed_files":["internal/projectmemory/runtime.go"]}`,
		),
		existingRecord(
			"decision-with-code-binding",
			artifact.KindDecisionRecord,
			`{"binding_targets":[{"kind":"symbol","file_path":"internal/projectmemory/runtime.go","symbol_name":"Run"}]}`,
		),
	}

	plan, err := existingrecordprojection.Build(records)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	gotRoutes := projectableRouteSummaries(plan.Routes())
	wantRoutes := []routeSummary{{
		ref:        "decision-with-code-binding",
		projection: existingrecordprojection.ProjectionDecisionChoiceAtConcern,
		requirements: []existingrecordprojection.Requirement{
			existingrecordprojection.RequirementProjectedSolutionPortfolio,
		},
	}}
	if !reflect.DeepEqual(gotRoutes, wantRoutes) {
		t.Fatalf("routes = %#v, want %#v", gotRoutes, wantRoutes)
	}

	gotDeferred := deferredRouteSummaries(plan.Deferred())
	wantDeferred := []deferredSummary{
		{
			ref:    "commission-completed",
			reason: existingrecordprojection.DeferredAuthorityCarrierIsNotPerformedWork,
		},
		{
			ref:    "evidence-passed",
			reason: existingrecordprojection.DeferredEvidenceCarrierNeedsWorkSource,
		},
		{
			ref:    "method-closed",
			reason: existingrecordprojection.DeferredMethodRunIsNotPerformedWork,
		},
	}
	if !reflect.DeepEqual(gotDeferred, wantDeferred) {
		t.Fatalf("deferred = %#v, want %#v", gotDeferred, wantDeferred)
	}
}

func TestBuildDoesNotInventComparisonProjection(t *testing.T) {
	t.Parallel()

	record := existingRecord(
		"sol-uncompared",
		artifact.KindSolutionPortfolio,
		`{"variants":[{"id":"option-a","title":"A"}]}`,
	)

	plan, err := existingrecordprojection.Build(
		[]*artifact.Artifact{record},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	got := projectableRouteSummaries(plan.Routes())
	want := []routeSummary{{
		ref:        "sol-uncompared",
		projection: existingrecordprojection.ProjectionSolutionPortfolioAtConcern,
		requirements: []existingrecordprojection.Requirement{
			existingrecordprojection.RequirementExactConcern,
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes = %#v, want %#v", got, want)
	}
}

func TestBuildRejectsAmbiguousInventory(t *testing.T) {
	t.Parallel()

	valid := existingRecord(
		"note-duplicate",
		artifact.KindNote,
		"",
	)

	tests := []struct {
		name    string
		records []*artifact.Artifact
	}{
		{
			name:    "nil record",
			records: []*artifact.Artifact{nil},
		},
		{
			name: "duplicate identity",
			records: []*artifact.Artifact{
				valid,
				valid,
			},
		},
		{
			name: "unknown artifact kind",
			records: []*artifact.Artifact{
				existingRecord(
					"unknown-existing",
					artifact.Kind("Unknown"),
					"",
				),
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := existingrecordprojection.Build(
				test.records,
			); err == nil {
				t.Fatal("Build() error = nil, want explicit inventory failure")
			}
		})
	}
}

type routeSummary struct {
	ref          string
	projection   existingrecordprojection.Projection
	requirements []existingrecordprojection.Requirement
}

func projectableRouteSummaries(
	routes []existingrecordprojection.Route,
) []routeSummary {
	summaries := make([]routeSummary, 0, len(routes))
	for _, route := range routes {
		summaries = append(summaries, routeSummary{
			ref:          route.ArtifactRef(),
			projection:   route.Projection(),
			requirements: route.Requirements(),
		})
	}
	return summaries
}

type deferredSummary struct {
	ref    string
	reason existingrecordprojection.DeferredReason
}

func deferredRouteSummaries(
	deferred []existingrecordprojection.Deferred,
) []deferredSummary {
	summaries := make([]deferredSummary, 0, len(deferred))
	for _, item := range deferred {
		summaries = append(summaries, deferredSummary{
			ref:    item.ArtifactRef(),
			reason: item.Reason(),
		})
	}
	return summaries
}

func existingRecord(
	ref string,
	kind artifact.Kind,
	structuredData string,
) *artifact.Artifact {
	return &artifact.Artifact{
		Meta: artifact.Meta{
			ID:      ref,
			Kind:    kind,
			Version: 1,
			Title:   ref,
		},
		StructuredData: structuredData,
	}
}
