package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestHandleQuintProblem_CharacterizePersistsStructuredParityPlan(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := artifact.FrameProblem(ctx, store, haftDir, artifact.ProblemFrameInput{
		Title:   "Transport choice",
		Signal:  "Latency variance",
		Context: "api",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = handleQuintProblemWithCreatedRef(ctx, store, haftDir, map[string]any{
		"action":      "characterize",
		"problem_ref": problem.Meta.ID,
		"dimensions": []any{
			map[string]any{"name": "latency"},
		},
		"parity_plan": `{"baseline_set":["REST","gRPC"],"window":"same 15m replay window","budget":"$200/month","missing_data_policy":"explicit_abstain"}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.Get(ctx, problem.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	fields := reloaded.UnmarshalProblemFields()
	if len(fields.Characterizations) != 1 {
		t.Fatalf("expected 1 characterization, got %+v", fields.Characterizations)
	}
	if fields.Characterizations[0].ParityPlan == nil {
		t.Fatal("expected structured parity plan to be persisted")
	}
	if got := fields.Characterizations[0].ParityPlan.Window; got != "same 15m replay window" {
		t.Fatalf("window = %q", got)
	}
}

func TestHandleQuintProblem_FramePersistsProblemType(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	result, _, err := handleQuintProblemWithCreatedRef(ctx, store, haftDir, map[string]any{
		"action":       "frame",
		"title":        "Search for a transport",
		"problem_type": "search",
		"signal":       "Existing options do not satisfy the deployment constraints",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Type: search") {
		t.Fatalf("expected frame response to show problem type, got %s", result)
	}

	problems, err := artifact.SelectProblems(ctx, store, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(problems))
	}

	reloaded, err := store.Get(ctx, problems[0].Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.UnmarshalProblemFields().ProblemType; got != artifact.ProblemTypeSearch {
		t.Fatalf("problem_type = %q", got)
	}
}

func TestHandleQuintSolution_CompareSurfacesMissingParityPlanWarning(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	portfolio := mustExploreServeComparePortfolio(t, ctx, store, haftDir, "")

	result, _, err := handleQuintSolutionWithCreatedRef(ctx, store, haftDir, map[string]any{
		"action":        "compare",
		"portfolio_ref": portfolio.Meta.ID,
		"dimensions":    []any{"latency"},
		"scores": map[string]any{
			"REST": map[string]any{"latency": "42ms"},
			"gRPC": map[string]any{"latency": "18ms"},
		},
		"non_dominated_set": []any{"gRPC"},
		"dominated_variants": []map[string]any{{
			"variant":      "REST",
			"dominated_by": []string{"gRPC"},
			"summary":      "Higher latency with no compensating benefit in this comparison.",
		}},
		"pareto_tradeoffs": []map[string]any{{
			"variant": "gRPC",
			"summary": "Lowest latency result among the compared variants.",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Comparison warnings:") {
		t.Fatalf("expected compare response to surface warnings, got %s", result)
	}
	if !strings.Contains(result, "without a parity_plan") {
		t.Fatalf("expected missing parity-plan warning, got %s", result)
	}
}

func TestHandleQuintSolution_CompareAcceptsLegacyRecommendationRef(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	portfolio := mustExploreServeComparePortfolio(t, ctx, store, haftDir, "")

	_, _, err := handleQuintSolutionWithCreatedRef(ctx, store, haftDir, map[string]any{
		"action":        "compare",
		"portfolio_ref": portfolio.Meta.ID,
		"dimensions":    []any{"latency"},
		"scores": map[string]any{
			"REST": map[string]any{"latency": "42ms"},
			"gRPC": map[string]any{"latency": "18ms"},
		},
		"non_dominated_set": []any{"gRPC"},
		"dominated_variants": []map[string]any{{
			"variant":      "REST",
			"dominated_by": []string{"gRPC"},
			"summary":      "Higher latency with no compensating benefit in this comparison.",
		}},
		"pareto_tradeoffs": []map[string]any{{
			"variant": "gRPC",
			"summary": "Lowest latency result among the compared variants.",
		}},
		"legacy_recommendation_ref": "gRPC",
	})
	if err != nil {
		t.Fatal(err)
	}

	reloaded, err := store.Get(ctx, portfolio.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	comparison := reloaded.UnmarshalPortfolioFields().Comparison
	if comparison == nil {
		t.Fatal("expected persisted comparison")
	}
	if comparison.LegacyRecommendationRef != "V2" {
		t.Fatalf("legacy_recommendation_ref = %q, want V2", comparison.LegacyRecommendationRef)
	}
	if comparison.SelectedRef != "V2" {
		t.Fatalf("selected_ref compatibility alias = %q, want V2", comparison.SelectedRef)
	}
}

func TestHandleQuintSolution_CompareSurfacesUnstructuredParityPlanWarning(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	portfolio := mustExploreServeComparePortfolio(t, ctx, store, haftDir, "deep")

	result, _, err := handleQuintSolutionWithCreatedRef(ctx, store, haftDir, map[string]any{
		"action":        "compare",
		"portfolio_ref": portfolio.Meta.ID,
		"dimensions":    []any{"latency"},
		"scores": map[string]any{
			"REST": map[string]any{"latency": "42ms"},
			"gRPC": map[string]any{"latency": "18ms"},
		},
		"non_dominated_set": []any{"gRPC"},
		"dominated_variants": []map[string]any{{
			"variant":      "REST",
			"dominated_by": []string{"gRPC"},
			"summary":      "Higher latency with no compensating benefit in this comparison.",
		}},
		"pareto_tradeoffs": []map[string]any{{
			"variant": "gRPC",
			"summary": "Lowest latency result among the compared variants.",
		}},
		"parity_plan": `{"window":"same 15m replay window"}`,
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Comparison warnings:") {
		t.Fatalf("expected compare response to surface warnings, got %s", result)
	}
	if !strings.Contains(result, "received an unstructured parity_plan") {
		t.Fatalf("expected unstructured parity-plan warning, got %s", result)
	}
}

func TestHandleQuintSolution_CompareRejectsDimensionFirstScoresWithShapeHint(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	portfolio := mustExploreServeComparePortfolio(t, ctx, store, haftDir, "")

	_, _, err := handleQuintSolutionWithCreatedRef(ctx, store, haftDir, map[string]any{
		"action":        "compare",
		"portfolio_ref": portfolio.Meta.ID,
		"dimensions":    []any{"latency"},
		"scores": map[string]any{
			"latency": map[string]any{
				"REST": "42ms",
				"gRPC": "18ms",
			},
		},
	})
	if err == nil {
		t.Fatal("expected dimension-first scores to fail")
	}
	for _, want := range []string{"scores shape", "variant_id -> dimension_name -> string score"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func mustExploreServeComparePortfolio(
	t *testing.T,
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	mode string,
) *artifact.Artifact {
	t.Helper()

	portfolio, _, err := artifact.ExploreSolutions(ctx, store, haftDir, artifact.ExploreInput{
		Mode: mode,
		Variants: []artifact.Variant{
			{
				Title:         "REST",
				WeakestLink:   "chatty serialization",
				NoveltyMarker: "Keep the existing HTTP semantics",
			},
			{
				Title:         "gRPC",
				WeakestLink:   "tooling overhead",
				NoveltyMarker: "Adopt binary RPC for lower-latency transport",
			},
		},
		NoSteppingStoneRationale: "Both transports are direct architecture candidates.",
	})
	if err != nil {
		t.Fatal(err)
	}

	return portfolio
}
