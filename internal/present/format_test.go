package present_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/present"
)

func TestNavStrip_AvailableGuardLine(t *testing.T) {
	state := artifact.NavState{
		DerivedStatus: artifact.DerivedFramed,
		Mode:          artifact.ModeTactical,
		NextAction:    `/h-explore (generate variants) | /h-decide (decide directly)`,
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
		NextAction:    "/h-compare (compare variants)",
	}

	output := present.NavStrip(state)

	for _, want := range []string{"Context: payments", "Mode: standard", "Status: EXPLORING", "Portfolio: API redesign", "Stale: 2", "Available:", "/h-compare"} {
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

func TestProblemResponse_ShowsProblemType(t *testing.T) {
	fields, err := json.Marshal(artifact.ProblemFields{
		ProblemType: artifact.ProblemTypeDiagnosis,
		Signal:      "signal",
	})
	if err != nil {
		t.Fatal(err)
	}

	a := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "prob-001",
			Kind:  artifact.KindProblemCard,
			Title: "Investigate webhook failures",
			Mode:  artifact.ModeStandard,
		},
		Body:           "# Test\n\n## Signal\n\nSomething\n",
		StructuredData: string(fields),
	}

	response := present.ProblemResponse("frame", a, "", "\n-- nav --\n")

	if !strings.Contains(response, "Type: diagnosis") {
		t.Fatalf("expected problem type in frame response, got:\n%s", response)
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

func TestStatusResponse_ShowsDerivedDecisionHealth(t *testing.T) {
	data := artifact.StatusData{
		HealthyDecisions: []*artifact.Artifact{
			{
				Meta: artifact.Meta{
					ID:    "dec-healthy",
					Title: "Healthy decision",
				},
			},
		},
		PendingDecisions: []*artifact.Artifact{
			{
				Meta: artifact.Meta{
					ID:    "dec-pending",
					Title: "Pending decision",
				},
			},
		},
		UnassessedDecisions: []*artifact.Artifact{
			{
				Meta: artifact.Meta{
					ID:    "dec-unassessed",
					Title: "Unassessed decision",
				},
			},
		},
		DecisionHealth: map[string]artifact.DecisionHealth{
			"dec-stale": {
				Maturity:  artifact.DecisionMaturityShipped,
				Freshness: artifact.DecisionFreshnessStale,
			},
		},
		StaleItems: []artifact.StaleItem{
			{
				ID:     "dec-stale",
				Title:  "Stale decision",
				Reason: "evidence degraded (R_eff: 0.40)",
			},
		},
		OpenCommissions: []artifact.WorkCommissionStatus{
			{
				ID:               "wc-stale",
				State:            "queued",
				DecisionRef:      "dec-stale",
				AttentionReason:  "open longer than 24h0m0s",
				SuggestedActions: []string{"inspect", "cancel"},
			},
		},
		CommissionAttention: []artifact.WorkCommissionStatus{
			{
				ID:               "wc-stale",
				State:            "queued",
				DecisionRef:      "dec-stale",
				AttentionReason:  "open longer than 24h0m0s",
				SuggestedActions: []string{"inspect", "cancel"},
			},
		},
	}

	output := present.StatusResponse(data)

	for _, want := range []string{
		"### Shipped / Healthy (1)",
		"### Pending (1)",
		"### Unassessed (1)",
		"**Stale decision** `dec-stale` — Shipped / Stale — evidence degraded (R_eff: 0.40)",
		"### WorkCommissions Need Attention (1)",
		"`wc-stale` queued → dec-stale — open longer than 24h0m0s — actions: inspect, cancel",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestStatusResponse_ShowsProblemTypeInListings(t *testing.T) {
	backlogFields, err := json.Marshal(artifact.ProblemFields{ProblemType: artifact.ProblemTypeSearch})
	if err != nil {
		t.Fatal(err)
	}
	inProgressFields, err := json.Marshal(artifact.ProblemFields{ProblemType: artifact.ProblemTypeDiagnosis})
	if err != nil {
		t.Fatal(err)
	}

	data := artifact.StatusData{
		BacklogProblems: []*artifact.Artifact{
			{
				Meta:           artifact.Meta{ID: "prob-backlog", Title: "Backlog problem"},
				StructuredData: string(backlogFields),
			},
		},
		InProgressProblems: []*artifact.Artifact{
			{
				Meta:           artifact.Meta{ID: "prob-progress", Title: "In progress problem"},
				StructuredData: string(inProgressFields),
			},
		},
		InProgressBy: map[string]string{"prob-progress": "sol-001"},
	}

	output := present.StatusResponse(data)

	for _, want := range []string{
		"**In progress problem (diagnosis)** `prob-progress` → sol-001",
		"**Backlog problem (search)** `prob-backlog`",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestProblemsListResponse_ShowsProblemTypeInHeading(t *testing.T) {
	fields, err := json.Marshal(artifact.ProblemFields{ProblemType: artifact.ProblemTypeSynthesis})
	if err != nil {
		t.Fatal(err)
	}

	output := present.ProblemsListResponse([]artifact.ProblemListItem{
		{
			Problem: &artifact.Artifact{
				Meta: artifact.Meta{
					ID:        "prob-001",
					Title:     "Design the deployment path",
					CreatedAt: mustParseTime(t, "2026-04-14T00:00:00Z"),
				},
				StructuredData: string(fields),
			},
		},
	}, "")

	if !strings.Contains(output, "### 1. Design the deployment path (synthesis) [prob-001]") {
		t.Fatalf("expected problem type in heading, got:\n%s", output)
	}
}

func TestGovernanceAttentionResponse_ShowsOrphansAndInvariantViolations(t *testing.T) {
	output := present.GovernanceAttentionResponse(artifact.GovernanceAttention{
		BacklogCount:    2,
		InProgressCount: 1,
		AddressedWithoutDecision: []artifact.AddressedProblemGap{
			{ProblemID: "prob-001", Title: "Orphan problem"},
		},
		InvariantViolations: []artifact.InvariantViolationFinding{
			{
				DecisionID:    "dec-001",
				DecisionTitle: "Boundary decision",
				Invariant:     "no dependency from api to database",
				Reason:        "Forbidden dependency detected: internal/api → internal/database",
			},
		},
	})

	for _, want := range []string{
		"Problems: 2 backlog, 1 in progress",
		"Addressed without linked decision (1)",
		"**Orphan problem** `prob-001`",
		"Invariant violations (1)",
		"**Boundary decision** `dec-001` — no dependency from api to database",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("governance attention missing %q:\n%s", want, output)
		}
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}

	return parsed
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
				AffectedFiles:   []string{"internal/transport/contracts.proto", "internal/transport/grpc.go"},
				SelectedTitle:   "gRPC",
				WhySelected:     "It meets the latency target with acceptable operating cost.",
				SelectionPolicy: "Minimize latency within the accepted cost envelope.",
				CounterArgument: "Tooling and local debugging remain weaker than the simpler HTTP baseline.",
				WeakestLink:     "Operational confidence still depends on limited production-grade evidence.",
				WhyNotOthers: []artifact.RejectionReason{
					{
						Variant: "REST",
						Reason:  "Higher latency with no compensating cost advantage.",
					},
				},
				Invariants:    []string{"p99 latency remains below 50ms during cutover"},
				Admissibility: []string{"No silent message loss during protocol migration"},
				Predictions: []artifact.DecisionPrediction{
					{
						Claim:      "Latency stays under 50ms",
						Observable: "publish latency p99",
						Threshold:  "< 50ms",
						Status:     artifact.ClaimStatusSupported,
					},
					{
						Claim:      "Throughput stays above 100k events/sec",
						Observable: "throughput",
						Threshold:  "> 100k events/sec",
						Status:     artifact.ClaimStatusInconclusive,
					},
				},
				RollbackTriggers: []string{"Error budget exceeds 2% during canary"},
				Measured:         true,
				Evidence: artifact.ProjectionEvidenceSummary{
					MeasurementCount:   1,
					MeasurementVerdict: "partial",
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
				"Predictions:",
				"supported: Latency stays under 50ms (observable: publish latency p99; threshold: < 50ms)",
			},
		},
		{
			view: artifact.ProjectionViewManager,
			wants: []string{
				"## Manager/Status View",
				"Problems: 0 backlog, 0 in progress, 1 addressed",
				"Decisions: 0 unassessed, 0 pending follow-through, 1 measured/shipped, 0 refresh due",
			},
		},
		{
			view: artifact.ProjectionViewAudit,
			wants: []string{
				"## Audit/Evidence View",
				"Selection policy: Minimize latency within the accepted cost envelope.",
				"inconclusive: Throughput stays above 100k events/sec (observable: throughput; threshold: > 100k events/sec)",
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
		{
			view: artifact.ProjectionViewDelegatedAgent,
			wants: []string{
				"## Delegated-Agent Brief",
				"Selected decision: gRPC `dec-001`",
				"Affected files: internal/transport/contracts.proto, internal/transport/grpc.go",
				"Invariants: p99 latency remains below 50ms during cutover",
				"Admissibility: No silent message loss during protocol migration",
				"Rollback triggers: Error budget exceeds 2% during canary",
				"Open claim risks:",
				"weakest link: Operational confidence still depends on limited production-grade evidence.",
				"inconclusive: Throughput stays above 100k events/sec (observable: throughput; threshold: > 100k events/sec)",
			},
		},
		{
			view: artifact.ProjectionViewChangeRationale,
			wants: []string{
				"## PR/Change Rationale",
				"Selected change: gRPC `dec-001`",
				"Problem signal: Latency variance between protocols",
				"Selected variant: gRPC",
				"Why selected: It meets the latency target with acceptable operating cost.",
				"Rejected alternatives:",
				"- REST: Higher latency with no compensating cost advantage.",
				"Rollback summary: Error budget exceeds 2% during canary",
				"Latest measurement verdict: partial",
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

func TestProjectionResponse_ChangesWhenPredictionStatusChanges(t *testing.T) {
	graph := artifact.ProjectionGraph{
		Decisions: []artifact.DecisionProjection{
			{
				Meta: artifact.Meta{
					ID:    "dec-001",
					Title: "gRPC",
				},
				Predictions: []artifact.DecisionPrediction{
					{
						Claim:      "Latency stays under 50ms",
						Observable: "publish latency p99",
						Threshold:  "< 50ms",
						Status:     artifact.ClaimStatusUnverified,
					},
				},
			},
		},
	}

	engineerBefore := present.ProjectionResponse(graph, artifact.ProjectionViewEngineer)
	auditBefore := present.ProjectionResponse(graph, artifact.ProjectionViewAudit)

	graph.Decisions[0].Predictions[0].Status = artifact.ClaimStatusSupported

	engineerAfter := present.ProjectionResponse(graph, artifact.ProjectionViewEngineer)
	auditAfter := present.ProjectionResponse(graph, artifact.ProjectionViewAudit)

	if !strings.Contains(engineerBefore, "unverified: Latency stays under 50ms") {
		t.Fatalf("expected engineer projection to show initial prediction status, got:\n%s", engineerBefore)
	}
	if !strings.Contains(auditBefore, "unverified: Latency stays under 50ms") {
		t.Fatalf("expected audit projection to show initial prediction status, got:\n%s", auditBefore)
	}
	if !strings.Contains(engineerAfter, "supported: Latency stays under 50ms") {
		t.Fatalf("expected engineer projection to show updated prediction status, got:\n%s", engineerAfter)
	}
	if !strings.Contains(auditAfter, "supported: Latency stays under 50ms") {
		t.Fatalf("expected audit projection to show updated prediction status, got:\n%s", auditAfter)
	}
	if engineerBefore == engineerAfter {
		t.Fatalf("expected engineer projection output to change after status update")
	}
	if auditBefore == auditAfter {
		t.Fatalf("expected audit projection output to change after status update")
	}
}

func TestProjectionResponse_ManagerStatusSeparatesUnassessedDecisions(t *testing.T) {
	graph := artifact.ProjectionGraph{
		Decisions: []artifact.DecisionProjection{
			{
				Meta: artifact.Meta{
					ID:    "dec-unassessed",
					Title: "Unassessed decision",
				},
				Health: artifact.DecisionHealth{
					Maturity: artifact.DecisionMaturityUnassessed,
				},
			},
			{
				Meta: artifact.Meta{
					ID:    "dec-pending",
					Title: "Pending decision",
				},
				Health: artifact.DecisionHealth{
					Maturity: artifact.DecisionMaturityPending,
				},
			},
			{
				Meta: artifact.Meta{
					ID:    "dec-shipped",
					Title: "Shipped decision",
				},
				NeedsRefresh: true,
				Health: artifact.DecisionHealth{
					Maturity: artifact.DecisionMaturityShipped,
				},
			},
		},
	}

	output := present.ProjectionResponse(graph, artifact.ProjectionViewManager)

	for _, want := range []string{
		"Decisions: 1 unassessed, 1 pending follow-through, 1 measured/shipped, 1 refresh due",
		"- **Unassessed decision** `dec-unassessed` — unassessed",
		"- **Pending decision** `dec-pending` — waiting for measurement",
		"- **Shipped decision** `dec-shipped` — measured, refresh due",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("manager projection missing %q:\n%s", want, output)
		}
	}
}

func TestProjectionResponse_DelegatedBriefKeepsSupportedClaimsOutOfOpenRiskList(t *testing.T) {
	graph := artifact.ProjectionGraph{
		Decisions: []artifact.DecisionProjection{
			{
				Meta: artifact.Meta{
					ID:    "dec-001",
					Title: "gRPC",
				},
				SelectedTitle: "gRPC",
				WeakestLink:   "Operational confidence still depends on limited production-grade evidence.",
				Predictions: []artifact.DecisionPrediction{
					{
						Claim:      "Latency stays under 50ms",
						Observable: "publish latency p99",
						Threshold:  "< 50ms",
						Status:     artifact.ClaimStatusSupported,
					},
					{
						Claim:      "Throughput stays above 100k events/sec",
						Observable: "throughput",
						Threshold:  "> 100k events/sec",
						Status:     artifact.ClaimStatusRefuted,
					},
				},
			},
		},
	}

	output := present.ProjectionResponse(graph, artifact.ProjectionViewDelegatedAgent)

	if strings.Contains(output, "supported: Latency stays under 50ms") {
		t.Fatalf("expected supported claim to stay out of open risk list, got:\n%s", output)
	}

	required := []string{
		"weakest link: Operational confidence still depends on limited production-grade evidence.",
		"refuted: Throughput stays above 100k events/sec (observable: throughput; threshold: > 100k events/sec)",
	}
	for _, want := range required {
		if !strings.Contains(output, want) {
			t.Fatalf("expected delegated brief to contain %q, got:\n%s", want, output)
		}
	}
}
