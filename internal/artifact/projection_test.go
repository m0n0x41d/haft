package artifact

import (
	"context"
	"testing"
)

func TestFetchProjectionGraph_BuildsSharedGraphFromArtifacts(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	problem, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:      "Transport choice",
		Signal:     "Latency variance between protocols",
		Acceptance: "Choose the transport with the best latency trade-off",
		Context:    "payments",
	})
	if err != nil {
		t.Fatal(err)
	}

	portfolio, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: problem.Meta.ID,
		Variants: []Variant{
			{
				ID:            "V1",
				Title:         "REST",
				WeakestLink:   "chatty payloads",
				NoveltyMarker: "Keep JSON request-response semantics",
			},
			{
				ID:            "V2",
				Title:         "gRPC",
				WeakestLink:   "tooling overhead",
				NoveltyMarker: "Move to binary RPC with generated clients",
			},
		},
		NoSteppingStoneRationale: "Both variants are direct target architectures.",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency", "cost"},
			Scores: map[string]map[string]string{
				"V1": {"latency": "42ms", "cost": "$120"},
				"V2": {"latency": "18ms", "cost": "$180"},
			},
			NonDominatedSet: []string{"V1", "V2"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "V1",
					DominatedBy: []string{"V2"},
					Summary:     "Higher latency with no compensating cost win.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "V1", Summary: "Lower cost, but higher latency."},
				{Variant: "V2", Summary: "Lower latency, but higher cost."},
			},
			PolicyApplied:           "Minimize latency within the accepted cost envelope.",
			SelectedRef:             "V2",
			RecommendationRationale: "It keeps latency low while staying inside the current budget tolerance.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		ProblemRef:    problem.Meta.ID,
		PortfolioRef:  portfolio.Meta.ID,
		SelectedTitle: "gRPC",
		WhySelected:   "It meets the latency target with acceptable operating cost.",
		Context:       "payments",
	}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = AttachEvidence(ctx, store, EvidenceInput{
		ArtifactRef: decision.Meta.ID,
		Content:     "Replay benchmark kept p95 latency below 25ms in the candidate environment.",
		Type:        "measurement",
		Verdict:     "supports",
		ClaimScope:  []string{"latency"},
	})
	if err != nil {
		t.Fatal(err)
	}

	graph, err := FetchProjectionGraph(ctx, store, "payments")
	if err != nil {
		t.Fatal(err)
	}

	if len(graph.Problems) != 1 {
		t.Fatalf("expected 1 problem, got %d", len(graph.Problems))
	}
	if len(graph.Portfolios) != 1 {
		t.Fatalf("expected 1 portfolio, got %d", len(graph.Portfolios))
	}
	if len(graph.Decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(graph.Decisions))
	}

	problemNode := graph.Problems[0]
	if problemNode.Signal != "Latency variance between protocols" {
		t.Fatalf("unexpected problem signal %q", problemNode.Signal)
	}
	if len(problemNode.PortfolioRefs) != 1 || problemNode.PortfolioRefs[0] != portfolio.Meta.ID {
		t.Fatalf("unexpected problem portfolio refs: %+v", problemNode.PortfolioRefs)
	}
	if len(problemNode.DecisionRefs) != 1 || problemNode.DecisionRefs[0] != decision.Meta.ID {
		t.Fatalf("unexpected problem decision refs: %+v", problemNode.DecisionRefs)
	}

	portfolioNode := graph.Portfolios[0]
	if portfolioNode.Comparison == nil {
		t.Fatal("expected compared portfolio in projection graph")
	}
	if len(portfolioNode.DecisionRefs) != 1 || portfolioNode.DecisionRefs[0] != decision.Meta.ID {
		t.Fatalf("unexpected portfolio decision refs: %+v", portfolioNode.DecisionRefs)
	}

	decisionNode := graph.Decisions[0]
	if decisionNode.SelectedTitle != "gRPC" {
		t.Fatalf("unexpected selected title %q", decisionNode.SelectedTitle)
	}
	if !decisionNode.Measured {
		t.Fatal("expected measurement evidence to mark the decision as measured")
	}
	if decisionNode.Evidence.MeasurementCount != 1 {
		t.Fatalf("expected 1 measurement evidence item, got %d", decisionNode.Evidence.MeasurementCount)
	}
	if len(decisionNode.PortfolioRefs) != 1 || decisionNode.PortfolioRefs[0] != portfolio.Meta.ID {
		t.Fatalf("unexpected decision portfolio refs: %+v", decisionNode.PortfolioRefs)
	}
}

func TestParseProjectionView_SupportsAliases(t *testing.T) {
	cases := map[string]ProjectionView{
		"":               ProjectionViewEngineer,
		"status":         ProjectionViewManager,
		"evidence":       ProjectionViewAudit,
		"pareto":         ProjectionViewCompare,
		"manager":        ProjectionViewManager,
		"compare":        ProjectionViewCompare,
		"manager/status": ProjectionViewManager,
	}

	for input, want := range cases {
		got, err := ParseProjectionView(input)
		if err != nil {
			t.Fatalf("ParseProjectionView(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseProjectionView(%q) = %q, want %q", input, got, want)
		}
	}
}
