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
		Body: "# Test\n\n## Signal\n\nSomething\n\n## Related History\n\n- [DecisionRecord] **Redis cache** `dec-001`\n",
	}

	response := present.ProblemResponse("frame", a, "/tmp/test.md", "\n-- nav --\n")

	if !strings.Contains(response, "Related History") {
		t.Error("frame response should surface Related History from body")
	}
	if !strings.Contains(response, "Redis cache") {
		t.Error("frame response should show recalled artifact")
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
