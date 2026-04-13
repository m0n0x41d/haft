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

	_, err = handleQuintProblem(ctx, store, haftDir, map[string]any{
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

func TestHandleQuintSolution_CompareSurfacesMissingParityPlanWarning(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	portfolio := mustExploreServeComparePortfolio(t, ctx, store, haftDir, "")

	result, err := handleQuintSolution(ctx, store, haftDir, map[string]any{
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

func TestHandleQuintSolution_CompareSurfacesUnstructuredParityPlanWarning(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx := context.Background()
	haftDir := t.TempDir()
	portfolio := mustExploreServeComparePortfolio(t, ctx, store, haftDir, "deep")

	result, err := handleQuintSolution(ctx, store, haftDir, map[string]any{
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
