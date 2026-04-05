package present_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/present"
)

func TestNavStrip_AvailableGuardLine(t *testing.T) {
	state := artifact.NavState{
		DerivedStatus: artifact.DerivedFramed,
		Mode:          artifact.ModeTactical,
		NextAction:    `/q-explore (generate variants) | /q-decide (decide directly)`,
	}

	output := present.NavStrip(state)

	if !strings.Contains(output, "Available:") {
		t.Errorf("should contain 'Available:', got:\n%s", output)
	}
	if !strings.Contains(output, "do not auto-execute") {
		t.Errorf("should contain guard line, got:\n%s", output)
	}
}

func TestNavStrip_NoGuardWhenDecided(t *testing.T) {
	state := artifact.NavState{
		DerivedStatus: artifact.DerivedDecided,
		DecisionInfo:  "Use Redis",
	}

	output := present.NavStrip(state)

	if strings.Contains(output, "Available:") {
		t.Errorf("DECIDED state should NOT show Available, got:\n%s", output)
	}
}

func TestNavStrip_AllFieldsRendered(t *testing.T) {
	state := artifact.NavState{
		Context:       "payments",
		Mode:          artifact.ModeStandard,
		DerivedStatus: artifact.DerivedExploring,
		PortfolioInfo: "API redesign",
		StaleCount:    2,
		NextAction:    "/q-compare (compare variants)",
	}

	output := present.NavStrip(state)

	for _, want := range []string{"Context: payments", "Mode: standard", "Status: EXPLORING", "Portfolio: API redesign", "Stale: 2", "Available:", "/q-compare"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}

func TestDriftResponse_LikelyImplemented(t *testing.T) {
	reports := []artifact.DriftReport{
		{
			DecisionID:        "dec-001",
			DecisionTitle:     "Implemented decision",
			HasBaseline:       false,
			LikelyImplemented: true,
			Files:             []artifact.DriftItem{{Path: "app.go", Status: artifact.DriftNoBaseline}},
		},
		{
			DecisionID:    "dec-002",
			DecisionTitle: "Not started decision",
			HasBaseline:   false,
			Files:         []artifact.DriftItem{{Path: "other.go", Status: artifact.DriftNoBaseline}},
		},
	}

	output := present.DriftResponse(reports, "")

	if !strings.Contains(output, "git activity detected after decision date") {
		t.Errorf("should report git activity for decision with commits:\n%s", output)
	}
	if !strings.Contains(output, "no git activity detected after decision date") {
		t.Errorf("should report no git activity for decision without commits:\n%s", output)
	}
}

func TestProblemResponse_ShowsRecall(t *testing.T) {
	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "prob-001",
			Kind:  artifact.KindProblemCard,
			Title: "Test problem",
			Mode:  artifact.ModeStandard,
		},
		Body: "# Test\n\n## Signal\n\nSomething\n\n## Related History\n\n- [decision] **Redis cache** `dec-001`\n",
	}

	response := present.ProblemResponse("frame", a, "/tmp/test.md", "\n-- nav --\n")

	if !strings.Contains(response, "Related History") {
		t.Error("frame response should surface Related History from body")
	}
	if !strings.Contains(response, "Redis cache") {
		t.Error("frame response should show recalled artifact")
	}
}

func TestProblemResponse_PreservesBodyVerbatim(t *testing.T) {
	relatedHistory := "## Related History\n\n- [decision] **Fix DecisionRecord parser** `dec-001`\n"
	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "prob-001",
			Kind:  artifact.KindProblemCard,
			Title: "ProblemCard migration",
			Mode:  artifact.ModeStandard,
		},
		Body: "# Test\n\n" + relatedHistory,
	}

	response := present.ProblemResponse("frame", a, "", "\n-- nav --\n")

	if !strings.Contains(response, relatedHistory) {
		t.Fatalf("expected related history slice to stay verbatim, got:\n%s", response)
	}
}

func TestProblemResponse_NoRecallWhenAbsent(t *testing.T) {
	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "prob-001",
			Kind:  artifact.KindProblemCard,
			Title: "Test problem",
			Mode:  artifact.ModeStandard,
		},
		Body: "# Test\n\n## Signal\n\nSomething\n",
	}

	response := present.ProblemResponse("frame", a, "", "\n-- nav --\n")

	if strings.Contains(response, "Related History") {
		t.Error("frame response should NOT show Related History when not in body")
	}
}

func TestSolutionResponse_CompareShowsNarrativeSummary(t *testing.T) {
	fields, err := json.Marshal(artifact.PortfolioFields{
		Variants: []artifact.Variant{
			{ID: "V1", Title: "Kafka"},
			{ID: "V2", Title: "NATS"},
			{ID: "V3", Title: "Redis Streams"},
		},
		Comparison: &artifact.ComparisonResult{
			NonDominatedSet: []string{"V1", "V2"},
			DominatedVariants: []artifact.DominatedVariantExplanation{
				{
					Variant:     "V3",
					DominatedBy: []string{"V2"},
					Summary:     "Lower throughput with no compensating operations win.",
				},
			},
			ParetoTradeoffs: []artifact.ParetoTradeoffNote{
				{Variant: "V1", Summary: "Best throughput, but highest ops cost."},
				{Variant: "V2", Summary: "Best ops simplicity, but lower headroom than Kafka."},
			},
			PolicyApplied:           "Minimize operations load above the throughput floor.",
			SelectedRef:             "V2",
			RecommendationRationale: "Meets the throughput floor while minimizing operational burden.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "sol-001",
			Kind:  artifact.KindSolutionPortfolio,
			Title: "Transport portfolio",
		},
		StructuredData: string(fields),
	}

	response := present.SolutionResponse("compare", a, "/tmp/sol.md", "\n-- nav --\n")

	required := []string{
		"File: /tmp/sol.md",
		"Computed Pareto front: Kafka, NATS",
		"Dominated variant elimination:",
		"Redis Streams: dominated by NATS. Lower throughput with no compensating operations win.",
		"Pareto-front trade-offs:",
		"Kafka: Best throughput, but highest ops cost.",
		"Recommendation (advisory): NATS",
		"Recommendation rationale: Meets the throughput floor while minimizing operational burden.",
		"Human choice remains open until decide.",
	}

	for _, want := range required {
		if !strings.Contains(response, want) {
			t.Fatalf("compare response missing %q:\n%s", want, response)
		}
	}
}

func TestSearchResponse_PreservesQueryTitleAndBodyVerbatim(t *testing.T) {
	query := "DecisionRecord"
	title := "Fix DecisionRecord parser"
	body := "# Summary\n\nDecisionRecord must stay verbatim here."
	results := []*artifact.Artifact{{
		Meta: artifact.Meta{
			ID:    "dec-001",
			Kind:  artifact.KindDecisionRecord,
			Title: title,
		},
		Body: body,
	}}

	response := present.SearchResponse(results, query)

	if !strings.Contains(response, "## Search: "+query+" (1 results)") {
		t.Fatalf("expected query to stay verbatim, got:\n%s", response)
	}
	if !strings.Contains(response, title) {
		t.Fatalf("expected title to stay verbatim, got:\n%s", response)
	}
	if !strings.Contains(response, "DecisionRecord must stay verbatim here.") {
		t.Fatalf("expected body preview to stay verbatim, got:\n%s", response)
	}
}

func TestDecisionResponse_PreservesDecisionBodyVerbatim(t *testing.T) {
	body := "# Decision\n\nFix DecisionRecord parser without changing DecisionRecord wording."
	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "dec-001",
			Kind:  artifact.KindDecisionRecord,
			Title: "Fix DecisionRecord parser",
		},
		Body: body,
	}

	response := present.DecisionResponse("decide", a, "", "", "\n-- nav --\n")

	if !strings.Contains(response, body) {
		t.Fatalf("expected decision body to stay verbatim, got:\n%s", response)
	}
}

func TestProjectionResponse_RendersAudienceViewsFromSameGraph(t *testing.T) {
	graph := artifact.ProjectionGraph{
		Problems: []artifact.ProblemProjection{
			{
				Meta: artifact.Meta{
					ID:     "prob-001",
					Title:  "Transport choice",
					Mode:   artifact.ModeStandard,
					Status: artifact.StatusActive,
				},
				Signal:              "Latency variance between protocols",
				Acceptance:          "Choose the transport with the best latency trade-off",
				OptimizationTargets: []string{"latency", "operational cost"},
				PortfolioRefs:       []string{"sol-001"},
				DecisionRefs:        []string{"dec-001"},
				Evidence: artifact.ProjectionEvidenceSummary{
					WLNK: artifact.WLNKSummary{Summary: "no evidence attached"},
				},
			},
		},
		Portfolios: []artifact.PortfolioProjection{
			{
				Meta: artifact.Meta{
					ID:     "sol-001",
					Title:  "Solutions for: Transport choice",
					Mode:   artifact.ModeStandard,
					Status: artifact.StatusActive,
				},
				ProblemRefs:  []string{"prob-001"},
				DecisionRefs: []string{"dec-001"},
				Variants: []artifact.Variant{
					{ID: "V1", Title: "REST"},
					{ID: "V2", Title: "gRPC"},
				},
				Comparison: &artifact.ComparisonResult{
					NonDominatedSet: []string{"V2"},
					DominatedVariants: []artifact.DominatedVariantExplanation{
						{
							Variant:     "V1",
							DominatedBy: []string{"V2"},
							Summary:     "Higher latency with no compensating cost advantage.",
						},
					},
					ParetoTradeoffs: []artifact.ParetoTradeoffNote{
						{Variant: "V2", Summary: "Best latency, but more tooling overhead than REST."},
					},
					PolicyApplied:           "Minimize latency within the accepted cost envelope.",
					SelectedRef:             "V2",
					RecommendationRationale: "It keeps latency low while staying inside the current budget tolerance.",
				},
				Evidence: artifact.ProjectionEvidenceSummary{
					WLNK: artifact.WLNKSummary{Summary: "no evidence attached"},
				},
			},
		},
		Decisions: []artifact.DecisionProjection{
			{
				Meta: artifact.Meta{
					ID:         "dec-001",
					Title:      "gRPC",
					Mode:       artifact.ModeStandard,
					Status:     artifact.StatusActive,
					ValidUntil: "2026-12-31T00:00:00Z",
				},
				ProblemRefs:     []string{"prob-001"},
				PortfolioRefs:   []string{"sol-001"},
				SelectedTitle:   "gRPC",
				SelectionPolicy: "Minimize latency within the accepted cost envelope.",
				CounterArgument: "Tooling and local debugging remain weaker than the simpler HTTP baseline.",
				WeakestLink:     "Operational confidence still depends on limited production-grade evidence.",
				Measured:        true,
				Evidence: artifact.ProjectionEvidenceSummary{
					MeasurementCount: 1,
					WLNK: artifact.WLNKSummary{
						Summary:      "R_eff=0.60 · 2 evidence item(s) · 1 supporting · 1 weakening",
						REff:         0.60,
						FEff:         2,
						WeakestCL:    1,
						MinFreshness: "2026-07-01T00:00:00Z",
						CoverageGaps: []string{"operational-cost"},
					},
				},
			},
		},
	}

	cases := []struct {
		view  artifact.ProjectionView
		wants []string
	}{
		{
			view: artifact.ProjectionViewEngineer,
			wants: []string{
				"## Engineer View",
				"Signal: Latency variance between protocols",
				"Portfolios: sol-001",
				"Selected: gRPC",
			},
		},
		{
			view: artifact.ProjectionViewManager,
			wants: []string{
				"## Manager/Status View",
				"Problems: 0 backlog, 0 in progress, 1 addressed",
				"Decisions: 0 pending follow-through, 1 measured/shipped, 0 refresh due",
			},
		},
		{
			view: artifact.ProjectionViewAudit,
			wants: []string{
				"## Audit/Evidence View",
				"Selection policy: Minimize latency within the accepted cost envelope.",
				"Coverage gaps: operational-cost",
				"Assurance: R_eff=0.60 | F_eff=2 | weakest CL=1",
			},
		},
		{
			view: artifact.ProjectionViewCompare,
			wants: []string{
				"## Compare/Pareto View",
				"Computed Pareto front: gRPC",
				"Dominated variant elimination:",
				"Recommendation (advisory): gRPC",
			},
		},
	}

	for _, tc := range cases {
		output := present.ProjectionResponse(graph, tc.view)
		for _, want := range tc.wants {
			if !strings.Contains(output, want) {
				t.Fatalf("view %s missing %q:\n%s", tc.view, want, output)
			}
		}
	}
}
