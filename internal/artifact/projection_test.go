package artifact

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
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

func TestFetchProjectionGraph_DerivesDecisionHealthBuckets(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	mustCreateDecision := func(id string, title string) {
		t.Helper()

		err := store.Create(ctx, &Artifact{
			Meta: Meta{
				ID:        id,
				Kind:      KindDecisionRecord,
				Title:     title,
				Status:    StatusActive,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Body: title,
		})
		if err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}

	mustAddEvidence := func(decisionID string, item EvidenceItem) {
		t.Helper()

		err := store.AddEvidenceItem(ctx, &item, decisionID)
		if err != nil {
			t.Fatalf("add evidence to %s: %v", decisionID, err)
		}
	}

	mustCreateDecision("dec-unassessed", "Unassessed decision")

	mustCreateDecision("dec-pending", "Pending decision")
	mustAddEvidence("dec-pending", EvidenceItem{
		ID:              "evid-pending",
		Type:            "research",
		Content:         "Design review completed",
		Verdict:         "supports",
		CongruenceLevel: 3,
		ValidUntil:      now.Add(24 * time.Hour).Format(time.RFC3339),
	})

	mustCreateDecision("dec-shipped", "Shipped decision")
	mustAddEvidence("dec-shipped", EvidenceItem{
		ID:              "evid-shipped",
		Type:            "measurement",
		Content:         "Latency target met in rollout",
		Verdict:         "accepted",
		CongruenceLevel: 3,
		ValidUntil:      now.Add(24 * time.Hour).Format(time.RFC3339),
	})

	graph, err := FetchProjectionGraph(ctx, store, "")
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]DecisionMaturity{}
	for _, decision := range graph.Decisions {
		got[decision.Meta.ID] = decision.Health.Maturity
	}

	want := map[string]DecisionMaturity{
		"dec-unassessed": DecisionMaturityUnassessed,
		"dec-pending":    DecisionMaturityPending,
		"dec-shipped":    DecisionMaturityShipped,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projection decision health = %#v, want %#v", got, want)
	}
}

func TestFetchProjectionGraph_DerivesDecisionProblemRefsThroughPortfolio(t *testing.T) {
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

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		PortfolioRef:  portfolio.Meta.ID,
		SelectedTitle: "gRPC",
		WhySelected:   "It meets the latency target with acceptable operating cost.",
		Context:       "payments",
	}))
	if err != nil {
		t.Fatal(err)
	}

	graph, err := FetchProjectionGraph(ctx, store, "payments")
	if err != nil {
		t.Fatal(err)
	}

	if len(graph.Problems) != 1 || len(graph.Decisions) != 1 {
		t.Fatalf("unexpected graph sizes: problems=%d decisions=%d", len(graph.Problems), len(graph.Decisions))
	}

	problemNode := graph.Problems[0]
	if len(problemNode.DecisionRefs) != 1 || problemNode.DecisionRefs[0] != decision.Meta.ID {
		t.Fatalf("expected portfolio-linked decision to back-propagate to problem, got %+v", problemNode.DecisionRefs)
	}

	decisionNode := graph.Decisions[0]
	if len(decisionNode.ProblemRefs) != 1 || decisionNode.ProblemRefs[0] != problem.Meta.ID {
		t.Fatalf("expected decision problem refs to be derived through the linked portfolio, got %+v", decisionNode.ProblemRefs)
	}
}

func TestFetchProjectionGraph_DerivesDecisionProblemRefsThroughHiddenPortfolio(t *testing.T) {
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

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		PortfolioRef:  portfolio.Meta.ID,
		SelectedTitle: "gRPC",
		WhySelected:   "It meets the latency target with acceptable operating cost.",
		Context:       "payments",
	}))
	if err != nil {
		t.Fatal(err)
	}

	storedPortfolio, err := store.Get(ctx, portfolio.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	storedPortfolio.Meta.Status = StatusSuperseded
	if err := store.Update(ctx, storedPortfolio); err != nil {
		t.Fatal(err)
	}

	graph, err := FetchProjectionGraph(ctx, store, "payments")
	if err != nil {
		t.Fatal(err)
	}

	if len(graph.Portfolios) != 0 {
		t.Fatalf("expected superseded portfolio to stay out of projected portfolio set, got %d", len(graph.Portfolios))
	}
	if len(graph.Decisions) != 1 || len(graph.Problems) != 1 {
		t.Fatalf("unexpected graph sizes: problems=%d decisions=%d", len(graph.Problems), len(graph.Decisions))
	}

	decisionNode := graph.Decisions[0]
	if len(decisionNode.PortfolioRefs) != 0 {
		t.Fatalf("expected hidden portfolio to stay out of display refs, got %+v", decisionNode.PortfolioRefs)
	}
	if len(decisionNode.ProblemRefs) != 1 || decisionNode.ProblemRefs[0] != problem.Meta.ID {
		t.Fatalf("expected hidden portfolio lineage to recover the projected problem, got %+v", decisionNode.ProblemRefs)
	}

	problemNode := graph.Problems[0]
	if len(problemNode.DecisionRefs) != 1 || problemNode.DecisionRefs[0] != decision.Meta.ID {
		t.Fatalf("expected hidden portfolio lineage to back-propagate the decision, got %+v", problemNode.DecisionRefs)
	}
}

func TestFetchProjectionGraph_UsesStructuredDecisionFieldsForQueryableDecisionState(t *testing.T) {
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

	structuredData, err := json.Marshal(DecisionFields{
		ProblemRefs:     []string{problem.Meta.ID},
		SelectedTitle:   "gRPC",
		WhySelected:     "It meets the latency target with acceptable operating cost.",
		SelectionPolicy: "Minimize latency within the accepted cost envelope.",
		CounterArgument: "Tooling and rollout complexity are still meaningful costs.",
		WeakestLink:     "Operational confidence still depends on limited production-grade evidence.",
		Invariants:      []string{"p99 latency remains below 50ms during cutover"},
		Admissibility:   []string{"No silent message loss during protocol migration"},
		Predictions: []DecisionPrediction{
			{
				Claim:      "Latency stays under 50ms",
				Observable: "publish latency p99",
				Threshold:  "< 50ms",
				Status:     ClaimStatusSupported,
			},
		},
		PreConditions:        []string{"Replay benchmark harness ready"},
		EvidenceRequirements: []string{"Replay benchmark with pinned protocol versions"},
		RefreshTriggers:      []string{"Latency budget regresses under production mix"},
		RollbackTriggers:     []string{"Cutover error rate exceeds the accepted ceiling"},
	})
	if err != nil {
		t.Fatal(err)
	}

	decision := &Artifact{
		Meta: Meta{
			ID:      "dec-structured-problem-refs",
			Kind:    KindDecisionRecord,
			Status:  StatusActive,
			Context: "payments",
			Title:   "gRPC",
		},
		Body:           "# gRPC\n",
		StructuredData: string(structuredData),
	}
	if err := store.Create(ctx, decision); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAffectedFiles(ctx, decision.Meta.ID, []AffectedFile{
		{Path: "internal/transport/contracts.proto"},
		{Path: "internal/transport/grpc.go"},
	}); err != nil {
		t.Fatal(err)
	}

	graph, err := FetchProjectionGraph(ctx, store, "payments")
	if err != nil {
		t.Fatal(err)
	}

	if len(graph.Problems) != 1 || len(graph.Decisions) != 1 {
		t.Fatalf("unexpected graph sizes: problems=%d decisions=%d", len(graph.Problems), len(graph.Decisions))
	}

	decisionNode := graph.Decisions[0]
	if !reflect.DeepEqual(decisionNode.ProblemRefs, []string{problem.Meta.ID}) {
		t.Fatalf("expected structured problem refs to survive into the projection graph, got %+v", decisionNode.ProblemRefs)
	}
	if !reflect.DeepEqual(decisionNode.Predictions, []DecisionPrediction{{
		Claim:      "Latency stays under 50ms",
		Observable: "publish latency p99",
		Threshold:  "< 50ms",
		Status:     ClaimStatusSupported,
	}}) {
		t.Fatalf("expected structured predictions in projection graph, got %+v", decisionNode.Predictions)
	}
	if !reflect.DeepEqual(decisionNode.AffectedFiles, []string{"internal/transport/contracts.proto", "internal/transport/grpc.go"}) {
		t.Fatalf("expected affected files in projection graph, got %+v", decisionNode.AffectedFiles)
	}
	if !reflect.DeepEqual(decisionNode.Invariants, []string{"p99 latency remains below 50ms during cutover"}) {
		t.Fatalf("expected invariants in projection graph, got %+v", decisionNode.Invariants)
	}
	if !reflect.DeepEqual(decisionNode.Admissibility, []string{"No silent message loss during protocol migration"}) {
		t.Fatalf("expected admissibility in projection graph, got %+v", decisionNode.Admissibility)
	}
	if !reflect.DeepEqual(decisionNode.PreConditions, []string{"Replay benchmark harness ready"}) {
		t.Fatalf("expected structured pre-conditions in projection graph, got %+v", decisionNode.PreConditions)
	}
	if !reflect.DeepEqual(decisionNode.EvidenceRequirements, []string{"Replay benchmark with pinned protocol versions"}) {
		t.Fatalf("expected structured evidence requirements in projection graph, got %+v", decisionNode.EvidenceRequirements)
	}
	if !reflect.DeepEqual(decisionNode.RefreshTriggers, []string{"Latency budget regresses under production mix"}) {
		t.Fatalf("expected structured refresh triggers in projection graph, got %+v", decisionNode.RefreshTriggers)
	}

	problemNode := graph.Problems[0]
	if len(problemNode.DecisionRefs) != 1 || problemNode.DecisionRefs[0] != decision.Meta.ID {
		t.Fatalf("expected structured decision problem refs to back-propagate to the problem, got %+v", problemNode.DecisionRefs)
	}
}

func TestFetchProjectionGraph_UpdatesProjectedPredictionStatusesAfterMeasure(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	decision, _, err := Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "JetStream",
		WhySelected:   "Predictable operational envelope",
		Context:       "payments",
		Predictions: []PredictionInput{
			{
				Claim:      "Latency stays under 50ms",
				Observable: "publish latency p99",
				Threshold:  "< 50ms",
			},
			{
				Claim:      "Throughput stays above 100k events/sec",
				Observable: "throughput",
				Threshold:  "> 100k events/sec",
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	graphBeforeMeasure, err := FetchProjectionGraph(ctx, store, "payments")
	if err != nil {
		t.Fatal(err)
	}
	if got := []ClaimStatus{
		graphBeforeMeasure.Decisions[0].Predictions[0].Status,
		graphBeforeMeasure.Decisions[0].Predictions[1].Status,
	}; !reflect.DeepEqual(got, []ClaimStatus{ClaimStatusUnverified, ClaimStatusUnverified}) {
		t.Fatalf("expected unverified predictions before measure, got %#v", got)
	}

	_, err = Measure(ctx, store, haftDir, MeasureInput{
		DecisionRef: decision.Meta.ID,
		Findings:    "Latency passed, throughput regressed under peak load.",
		CriteriaMet: []string{
			"publish latency p99 < 50ms (observed: 44ms)",
		},
		CriteriaNotMet: []string{
			"Throughput stays above 100k events/sec (observed: 87k events/sec)",
		},
		Verdict: "partial",
	})
	if err != nil {
		t.Fatal(err)
	}

	graphAfterMeasure, err := FetchProjectionGraph(ctx, store, "payments")
	if err != nil {
		t.Fatal(err)
	}

	got := []ClaimStatus{
		graphAfterMeasure.Decisions[0].Predictions[0].Status,
		graphAfterMeasure.Decisions[0].Predictions[1].Status,
	}
	want := []ClaimStatus{
		ClaimStatusSupported,
		ClaimStatusRefuted,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projected prediction statuses = %#v, want %#v", got, want)
	}
	if got := graphAfterMeasure.Decisions[0].Evidence.MeasurementVerdict; got != "weakens" {
		t.Fatalf("projected measurement verdict = %q, want %q", got, "weakens")
	}
}

func TestParseProjectionView_SupportsAliases(t *testing.T) {
	cases := map[string]ProjectionView{
		"":                 ProjectionViewEngineer,
		"status":           ProjectionViewManager,
		"evidence":         ProjectionViewAudit,
		"pareto":           ProjectionViewCompare,
		"brief":            ProjectionViewDelegatedAgent,
		"delegated":        ProjectionViewDelegatedAgent,
		"handoff":          ProjectionViewDelegatedAgent,
		"delegated-agent":  ProjectionViewDelegatedAgent,
		"manager":          ProjectionViewManager,
		"compare":          ProjectionViewCompare,
		"manager/status":   ProjectionViewManager,
		"change-rationale": ProjectionViewChangeRationale,
		"rationale":        ProjectionViewChangeRationale,
		"pr":               ProjectionViewChangeRationale,
		"pr/change":        ProjectionViewChangeRationale,
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
