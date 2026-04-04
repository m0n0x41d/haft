package artifact

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestExploreSolutions_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Create a problem first
	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Event infrastructure", Signal: "DB polling hitting 70% CPU", Context: "events",
	})

	input := ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{
				Title:         "Kafka",
				Description:   "High throughput, battle-tested, complex ops",
				Strengths:     []string{"200k events/sec", "Proven at scale"},
				WeakestLink:   "Operational complexity (ZooKeeper, rebalancing)",
				NoveltyMarker: "Maximize throughput with established streaming ecosystem",
				Risks:         []string{"Requires dedicated ops expertise"},
				DiversityRole: "throughput ceiling",
			},
			{
				Title:              "NATS JetStream",
				Description:        "Simpler ops, embedded, growing ecosystem",
				Strengths:          []string{"100k events/sec", "Minimal ops"},
				WeakestLink:        "Ecosystem maturity at scale",
				NoveltyMarker:      "Collapse broker operations into a leaner embedded control plane",
				SteppingStone:      true,
				SteppingStoneBasis: "Lets the team validate event-driven flow with less operational overhead before scaling up.",
				DiversityRole:      "low-ops bridge",
			},
			{
				Title:         "Redis Streams",
				Description:   "Already have Redis, minimal new infra",
				WeakestLink:   "Not designed for durable event sourcing",
				NoveltyMarker: "Reuse existing Redis footprint for the fastest path to rollout",
				Risks:         []string{"Data loss risk under pressure"},
				DiversityRole: "minimum-change option",
			},
		},
	}

	a, filePath, err := ExploreSolutions(ctx, store, haftDir, input)
	if err != nil {
		t.Fatal(err)
	}

	if a.Meta.Kind != KindSolutionPortfolio {
		t.Errorf("kind = %q", a.Meta.Kind)
	}
	if a.Meta.Context != "events" {
		t.Errorf("context = %q, want events (inherited from problem)", a.Meta.Context)
	}
	if filePath == "" {
		t.Error("file path should not be empty")
	}

	// Check body
	if !strings.Contains(a.Body, "Kafka") {
		t.Error("missing Kafka variant")
	}
	if !strings.Contains(a.Body, "NATS JetStream") {
		t.Error("missing NATS variant")
	}
	if !strings.Contains(a.Body, "Stepping stone") {
		t.Error("missing stepping stone flag")
	}
	if !strings.Contains(a.Body, "## Summary") {
		t.Error("missing summary table")
	}

	// Check links
	links, _ := store.GetLinks(ctx, a.Meta.ID)
	if len(links) != 1 || links[0].Ref != prob.Meta.ID {
		t.Errorf("expected link to problem, got %+v", links)
	}

	fields := a.UnmarshalPortfolioFields()
	if len(fields.Variants) != 3 {
		t.Fatalf("expected 3 structured variants, got %d", len(fields.Variants))
	}
	if fields.Variants[0].ID != "V1" || fields.Variants[1].ID != "V2" || fields.Variants[2].ID != "V3" {
		t.Fatalf("expected generated IDs V1/V2/V3, got %q/%q/%q",
			fields.Variants[0].ID,
			fields.Variants[1].ID,
			fields.Variants[2].ID)
	}
	if !fields.Variants[1].SteppingStone {
		t.Error("expected structured stepping stone flag on NATS variant")
	}
}

func TestExploreSolutions_TooFewVariants(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := ExploreSolutions(ctx, store, t.TempDir(), ExploreInput{
		Variants: []Variant{
			{Title: "Only one", WeakestLink: "only option"},
		},
	})
	if err == nil {
		t.Error("expected error for <2 variants")
	}
}

func TestExploreSolutions_MissingWeakestLink(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := ExploreSolutions(ctx, store, t.TempDir(), ExploreInput{
		Variants: []Variant{
			testVariant("A", "something", "Use the familiar in-process design"),
			testVariant("B", "", "Introduce a separate worker boundary"),
		},
		NoSteppingStoneRationale: "The pair is testing two direct implementation shapes.",
	})
	if err == nil {
		t.Error("expected error for missing weakest_link")
	}
}

func TestExploreSolutions_NoProblem(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	a, _, err := ExploreSolutions(ctx, store, t.TempDir(), ExploreInput{
		Variants: []Variant{
			testVariant("A", "x", "Keep the runtime surface small"),
			testVariant("B", "y", "Bias toward operational elasticity"),
		},
		NoSteppingStoneRationale: "Both options are production-target candidates.",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should work without problem ref — tactical mode
	if a.Meta.Kind != KindSolutionPortfolio {
		t.Errorf("kind = %q", a.Meta.Kind)
	}
}

func TestExploreSolutions_RequiresSteppingStoneOrRationale(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := ExploreSolutions(ctx, store, t.TempDir(), ExploreInput{
		Variants: []Variant{
			testVariant("A", "x", "Keep the runtime surface small"),
			testVariant("B", "y", "Bias toward operational elasticity"),
		},
	})
	if err == nil {
		t.Fatal("expected stepping-stone coverage validation error")
	}
	if !strings.Contains(err.Error(), "no_stepping_stone_rationale") {
		t.Fatalf("expected no_stepping_stone_rationale error, got %v", err)
	}
}

func TestExploreSolutions_RequiresSteppingStoneBasis(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := ExploreSolutions(ctx, store, t.TempDir(), ExploreInput{
		Variants: []Variant{
			{
				Title:         "A",
				WeakestLink:   "x",
				NoveltyMarker: "Probe a simpler transport boundary first",
				SteppingStone: true,
			},
			testVariant("B", "y", "Bias toward operational elasticity"),
		},
	})
	if err == nil {
		t.Fatal("expected stepping_stone_basis validation error")
	}
	if !strings.Contains(err.Error(), "stepping_stone_basis") {
		t.Fatalf("expected stepping_stone_basis error, got %v", err)
	}
}

func TestExploreSolutions_RejectsDuplicateExplicitVariantIDs(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := ExploreSolutions(ctx, store, t.TempDir(), ExploreInput{
		Variants: []Variant{
			{ID: "V7", Title: "Kafka", WeakestLink: "ops complexity", NoveltyMarker: "Maximize throughput with established streaming ecosystem"},
			{ID: "V7", Title: "NATS", WeakestLink: "ecosystem maturity", NoveltyMarker: "Lean embedded broker with simpler cluster operations"},
		},
		NoSteppingStoneRationale: "Both options are direct implementation candidates.",
	})
	if err == nil {
		t.Fatal("expected duplicate variant identity error")
	}
	if !strings.Contains(err.Error(), `variant identity "V7" is duplicated`) {
		t.Fatalf("unexpected duplicate ID error: %v", err)
	}
}

func TestCompareSolutions_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Create portfolio
	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("Kafka", "ops complexity", "Maximize throughput with established streaming ecosystem"),
			testVariant("NATS", "ecosystem maturity", "Lean embedded broker with simpler cluster operations"),
			testVariant("Redis Streams", "durability risk", "Reuse existing Redis footprint for the fastest rollout"),
		},
		NoSteppingStoneRationale: "All three options are evaluated as direct end-state candidates.",
	})

	// Compare
	input := CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"throughput", "ops complexity", "cost"},
			Scores: map[string]map[string]string{
				"Kafka":         {"throughput": "200k/s", "ops complexity": "High", "cost": "$800"},
				"NATS":          {"throughput": "100k/s", "ops complexity": "Low", "cost": "$200"},
				"Redis Streams": {"throughput": "80k/s", "ops complexity": "Low", "cost": "$250"},
			},
			NonDominatedSet: []string{"Kafka", "NATS"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "Redis Streams",
					DominatedBy: []string{"NATS"},
					Summary:     "Lower throughput with no compensating simplicity or cost advantage over NATS.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Kafka", Summary: "Maximizes throughput, but accepts the highest ops complexity and cost."},
				{Variant: "NATS", Summary: "Minimizes ops complexity and cost, but gives up throughput headroom."},
			},
			PolicyApplied:           "Minimize ops complexity at sufficient throughput (>50k/s)",
			SelectedRef:             "NATS",
			RecommendationRationale: "NATS clears the throughput floor while minimizing operational burden.",
		},
	}

	a, _, err := CompareSolutions(ctx, store, haftDir, input)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(a.Body, "## Comparison") {
		t.Error("missing Comparison section")
	}
	if !strings.Contains(a.Body, "## Non-Dominated Set") {
		t.Error("missing Non-Dominated Set section")
	}
	if !strings.Contains(a.Body, "Kafka, NATS") {
		t.Error("missing Pareto front members")
	}
	if !strings.Contains(a.Body, "## Dominated Variant Elimination") {
		t.Error("missing dominated variant elimination section")
	}
	if !strings.Contains(a.Body, "## Pareto Front Trade-Offs") {
		t.Error("missing Pareto front trade-offs section")
	}
	if !strings.Contains(a.Body, "**Recommendation (advisory):** NATS") {
		t.Error("missing recommendation")
	}
	if !strings.Contains(a.Body, "**Recommendation rationale:** NATS clears the throughput floor while minimizing operational burden.") {
		t.Error("missing recommendation rationale")
	}
	if a.Meta.Version != 2 {
		t.Errorf("version = %d, want 2 after comparison", a.Meta.Version)
	}

	fields := a.UnmarshalPortfolioFields()
	if fields.Comparison == nil {
		t.Fatal("expected structured comparison payload")
	}
	if len(fields.Comparison.DominatedVariants) != 1 {
		t.Fatalf("expected 1 dominated variant explanation, got %+v", fields.Comparison.DominatedVariants)
	}
	if len(fields.Comparison.ParetoTradeoffs) != 2 {
		t.Fatalf("expected 2 Pareto trade-offs, got %+v", fields.Comparison.ParetoTradeoffs)
	}
}

func TestCompareSolutions_RejectsMissingDominatedVariantExplanation(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("Kafka", "ops complexity", "Maximize throughput with established streaming ecosystem"),
			testVariant("NATS", "ecosystem maturity", "Lean embedded broker with simpler cluster operations"),
			testVariant("Redis Streams", "durability risk", "Reuse existing Redis footprint for the fastest rollout"),
		},
		NoSteppingStoneRationale: "All three options are evaluated as direct end-state candidates.",
	})

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"throughput", "ops complexity"},
			Scores: map[string]map[string]string{
				"Kafka":         {"throughput": "200k/s", "ops complexity": "High"},
				"NATS":          {"throughput": "100k/s", "ops complexity": "Low"},
				"Redis Streams": {"throughput": "80k/s", "ops complexity": "Low"},
			},
			NonDominatedSet: []string{"Kafka"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "NATS",
					DominatedBy: []string{"Kafka"},
					Summary:     "Lower throughput without enough operational upside.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Kafka", Summary: "Maximizes throughput, but carries the highest operating burden."},
			},
		},
	})
	if err == nil {
		t.Fatal("expected dominated-variant coverage error")
	}
	if !strings.Contains(err.Error(), "dominated_variants must explain every compared variant outside the Pareto front") {
		t.Fatalf("unexpected dominated-variant coverage error: %v", err)
	}
	if !strings.Contains(err.Error(), "V3") {
		t.Fatalf("expected missing dominated variant ID in error, got %v", err)
	}
}

func TestCompareSolutions_RejectsDuplicateDominatedVariantExplanation(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("Kafka", "ops complexity", "Maximize throughput with established streaming ecosystem"),
			testVariant("NATS", "ecosystem maturity", "Lean embedded broker with simpler cluster operations"),
			testVariant("Redis Streams", "durability risk", "Reuse existing Redis footprint for the fastest rollout"),
		},
		NoSteppingStoneRationale: "All three options are evaluated as direct end-state candidates.",
	})

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"throughput", "ops complexity"},
			Scores: map[string]map[string]string{
				"Kafka":         {"throughput": "200k/s", "ops complexity": "High"},
				"NATS":          {"throughput": "100k/s", "ops complexity": "Low"},
				"Redis Streams": {"throughput": "80k/s", "ops complexity": "Low"},
			},
			NonDominatedSet: []string{"Kafka"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "NATS",
					DominatedBy: []string{"Kafka"},
					Summary:     "Lower throughput without enough operational upside.",
				},
				{
					Variant:     "V2",
					DominatedBy: []string{"Kafka"},
					Summary:     "Repeated explanation should fail validation.",
				},
				{
					Variant:     "Redis Streams",
					DominatedBy: []string{"Kafka"},
					Summary:     "Lower throughput and no cost advantage over the front.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Kafka", Summary: "Maximizes throughput, but carries the highest operating burden."},
			},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate dominated-variant coverage error")
	}
	if !strings.Contains(err.Error(), "dominated_variants must explain each dominated variant exactly once") {
		t.Fatalf("unexpected dominated-variant duplicate error: %v", err)
	}
}

func TestCompareSolutions_RejectsMissingParetoTradeoff(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("Kafka", "ops complexity", "Maximize throughput with established streaming ecosystem"),
			testVariant("NATS", "ecosystem maturity", "Lean embedded broker with simpler cluster operations"),
			testVariant("Redis Streams", "durability risk", "Reuse existing Redis footprint for the fastest rollout"),
		},
		NoSteppingStoneRationale: "All three options are evaluated as direct end-state candidates.",
	})

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"throughput", "ops complexity"},
			Scores: map[string]map[string]string{
				"Kafka":         {"throughput": "200k/s", "ops complexity": "High"},
				"NATS":          {"throughput": "100k/s", "ops complexity": "Low"},
				"Redis Streams": {"throughput": "80k/s", "ops complexity": "Low"},
			},
			NonDominatedSet: []string{"Kafka", "NATS"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "Redis Streams",
					DominatedBy: []string{"NATS"},
					Summary:     "Lower throughput with no compensating simplicity or cost advantage over NATS.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Kafka", Summary: "Maximizes throughput, but accepts the highest ops complexity."},
			},
		},
	})
	if err == nil {
		t.Fatal("expected Pareto trade-off coverage error")
	}
	if !strings.Contains(err.Error(), "pareto_tradeoffs must explain every Pareto-front variant") {
		t.Fatalf("unexpected pareto trade-off coverage error: %v", err)
	}
	if !strings.Contains(err.Error(), "V2") {
		t.Fatalf("expected missing Pareto variant ID in error, got %v", err)
	}
}

func TestCompareSolutions_RejectsDuplicateParetoTradeoff(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("Kafka", "ops complexity", "Maximize throughput with established streaming ecosystem"),
			testVariant("NATS", "ecosystem maturity", "Lean embedded broker with simpler cluster operations"),
			testVariant("Redis Streams", "durability risk", "Reuse existing Redis footprint for the fastest rollout"),
		},
		NoSteppingStoneRationale: "All three options are evaluated as direct end-state candidates.",
	})

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"throughput", "ops complexity"},
			Scores: map[string]map[string]string{
				"Kafka":         {"throughput": "200k/s", "ops complexity": "High"},
				"NATS":          {"throughput": "100k/s", "ops complexity": "Low"},
				"Redis Streams": {"throughput": "80k/s", "ops complexity": "Low"},
			},
			NonDominatedSet: []string{"Kafka", "NATS"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "Redis Streams",
					DominatedBy: []string{"NATS"},
					Summary:     "Lower throughput with no compensating simplicity or cost advantage over NATS.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Kafka", Summary: "Maximizes throughput, but accepts the highest ops complexity."},
				{Variant: "V1", Summary: "Repeated explanation should fail validation."},
				{Variant: "NATS", Summary: "Minimizes ops complexity, but gives up throughput headroom."},
			},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate Pareto trade-off error")
	}
	if !strings.Contains(err.Error(), "pareto_tradeoffs must explain each Pareto-front variant exactly once") {
		t.Fatalf("unexpected pareto trade-off duplicate error: %v", err)
	}
}

func TestCompareSolutions_RejectsLegacyPortfolioWithDuplicateVariantIDs(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	fields, err := json.Marshal(PortfolioFields{
		Variants: []Variant{
			{ID: "V7", Title: "Kafka", WeakestLink: "ops complexity", NoveltyMarker: "Maximize throughput with established streaming ecosystem"},
			{ID: "V7", Title: "NATS", WeakestLink: "ecosystem maturity", NoveltyMarker: "Lean embedded broker with simpler cluster operations"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	portfolio := &Artifact{
		Meta: Meta{
			ID:      "sol-legacy-duplicate-id",
			Kind:    KindSolutionPortfolio,
			Title:   "Legacy ambiguous portfolio",
			Context: "events",
			Mode:    ModeStandard,
		},
		Body: `# Legacy ambiguous portfolio

## Variants (2)

### V7. Kafka

**Weakest link:** ops complexity

### V7. NATS

**Weakest link:** ecosystem maturity
`,
		StructuredData: string(fields),
	}
	if err := store.Create(ctx, portfolio); err != nil {
		t.Fatal(err)
	}

	_, _, err = CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"V7": {"latency": "10ms"},
			},
			NonDominatedSet: []string{"V7"},
		},
	})
	if err == nil {
		t.Fatal("expected compare to reject ambiguous stored portfolio identities")
	}
	if !strings.Contains(err.Error(), `portfolio sol-legacy-duplicate-id has ambiguous variant identities`) {
		t.Fatalf("unexpected portfolio identity error: %v", err)
	}
	if !strings.Contains(err.Error(), `variant identity "V7" is duplicated`) {
		t.Fatalf("expected duplicate identity detail, got %v", err)
	}
}

func TestCompareSolutions_RejectsPortfolioWithoutRecoverableVariants(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	portfolio := &Artifact{
		Meta: Meta{
			ID:      "sol-legacy-no-variants",
			Kind:    KindSolutionPortfolio,
			Title:   "Legacy empty portfolio",
			Context: "events",
			Mode:    ModeStandard,
		},
		Body: `# Legacy empty portfolio

Comparison draft with no declared variants.
`,
		StructuredData: `{}`,
	}
	if err := store.Create(ctx, portfolio); err != nil {
		t.Fatal(err)
	}

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"V1": {"latency": "10ms"},
			},
			NonDominatedSet: []string{"V1"},
		},
	})
	if err == nil {
		t.Fatal("expected compare to reject portfolio without recoverable variants")
	}
	if !strings.Contains(err.Error(), `portfolio sol-legacy-no-variants declares no recoverable variants`) {
		t.Fatalf("unexpected missing-variants error: %v", err)
	}
}

func TestCompareSolutions_MissingPortfolio(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, _, err := CompareSolutions(ctx, store, t.TempDir(), CompareInput{
		Results: ComparisonResult{
			Dimensions:      []string{"speed"},
			NonDominatedSet: []string{"A"},
		},
	})
	if err == nil {
		t.Error("expected error for missing portfolio_ref")
	}
}

func TestCompareSolutions_NoDimensions(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	p, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("A", "x", "Keep the runtime surface small"),
			testVariant("B", "y", "Bias toward operational elasticity"),
		},
		NoSteppingStoneRationale: "Both options are direct candidates.",
	})

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: p.Meta.ID,
		Results: ComparisonResult{
			NonDominatedSet: []string{"A"},
		},
	})
	if err == nil {
		t.Error("expected error for no dimensions")
	}
}

func TestCompareSolutions_ComputesParetoFrontWithoutNonDominatedSet(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	p, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("A", "x", "Keep the runtime surface small"),
			testVariant("B", "y", "Bias toward operational elasticity"),
		},
		NoSteppingStoneRationale: "Both options are direct candidates.",
	})

	a, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: p.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"A": {"latency": "10ms"},
				"B": {"latency": "5ms"},
			},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "A",
					DominatedBy: []string{"B"},
					Summary:     "Higher latency with no offsetting advantage in this comparison.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "B", Summary: "Lowest latency in the compared pair."},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected compare to compute the Pareto front, got %v", err)
	}
	if !strings.Contains(a.Body, "**Computed Pareto front:** B") {
		t.Fatalf("expected computed Pareto front in body, got %s", a.Body)
	}
}

func TestCompareSolutions_WarnsWhenAdvisoryParetoSetDisagrees(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("Kafka", "ops complexity", "Maximize throughput with established streaming ecosystem"),
			testVariant("NATS", "ecosystem maturity", "Lean embedded broker with simpler cluster operations"),
			testVariant("Redis Streams", "durability risk", "Reuse existing Redis footprint for the fastest rollout"),
		},
		NoSteppingStoneRationale: "All options are direct implementation candidates.",
	})

	a, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"throughput", "ops complexity", "cost"},
			Scores: map[string]map[string]string{
				"Kafka":         {"throughput": "200k/s", "ops complexity": "High", "cost": "$800"},
				"NATS":          {"throughput": "100k/s", "ops complexity": "Low", "cost": "$200"},
				"Redis Streams": {"throughput": "80k/s", "ops complexity": "Low", "cost": "$250"},
			},
			NonDominatedSet: []string{"Kafka"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "Redis Streams",
					DominatedBy: []string{"NATS"},
					Summary:     "Lower throughput with no compensating simplicity or cost advantage over NATS.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Kafka", Summary: "Maximizes throughput, but carries the highest operating burden."},
				{Variant: "NATS", Summary: "Minimizes ops complexity and cost, but gives up throughput headroom."},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Body, "provided non_dominated_set disagrees with the computed Pareto front") {
		t.Fatalf("expected advisory-front mismatch warning, body: %s", a.Body)
	}

	fields := a.UnmarshalPortfolioFields()
	if fields.Comparison == nil {
		t.Fatal("expected structured comparison payload")
	}
	if got := fields.Comparison.NonDominatedSet; len(got) != 2 || got[0] != "V1" || got[1] != "V2" {
		t.Fatalf("expected stored computed front [V1 V2], got %+v", got)
	}
}

func TestComputeParetoFront_SimpleDominance(t *testing.T) {
	results := ComparisonResult{
		Dimensions: []string{"latency", "cost"},
		Scores: map[string]map[string]string{
			"V1": {"latency": "42ms", "cost": "$180"},
			"V2": {"latency": "18ms", "cost": "$120"},
		},
	}

	front, warnings := computeParetoFront(results, []string{"V1", "V2"}, []charDim{
		{Name: "latency", Role: "target", Polarity: "lower_better"},
		{Name: "cost", Role: "target", Polarity: "lower_better"},
	}, MissingDataPolicyExplicitAbstain)

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
	if len(front) != 1 || front[0] != "V2" {
		t.Fatalf("expected front [V2], got %+v", front)
	}
}

func TestComputeParetoFront_KeepsTiesAndMissingScoresOnFront(t *testing.T) {
	results := ComparisonResult{
		Dimensions: []string{"latency", "cost"},
		Scores: map[string]map[string]string{
			"V1": {"latency": "10ms", "cost": "$100"},
			"V2": {"latency": "10ms", "cost": "$100"},
			"V3": {"latency": "8ms"},
		},
	}

	front, warnings := computeParetoFront(results, []string{"V1", "V2", "V3"}, []charDim{
		{Name: "latency", Role: "target", Polarity: "lower_better"},
		{Name: "cost", Role: "target", Polarity: "lower_better"},
	}, MissingDataPolicyExplicitAbstain)

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
	if !sameTrimmedSet(front, []string{"V1", "V2", "V3"}) {
		t.Fatalf("expected missing-score abstain to keep all variants on the front, got %+v", front)
	}
}

func TestComputeParetoFront_ExcludesObservationDimensions(t *testing.T) {
	results := ComparisonResult{
		Dimensions: []string{"throughput", "page count"},
		Scores: map[string]map[string]string{
			"V1": {"throughput": "100k/s", "page count": "10"},
			"V2": {"throughput": "80k/s", "page count": "1"},
		},
	}

	front, warnings := computeParetoFront(results, []string{"V1", "V2"}, []charDim{
		{Name: "throughput", Role: "target", Polarity: "higher_better"},
		{Name: "page count", Role: "observation", Polarity: "lower_better"},
	}, MissingDataPolicyExplicitAbstain)

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
	if len(front) != 1 || front[0] != "V1" {
		t.Fatalf("expected observation dimension to be ignored and V1 to dominate, got %+v", front)
	}
}

func TestComputeParetoFront_ExcludeSkipsMissingUnitBearingDimension(t *testing.T) {
	results := ComparisonResult{
		Dimensions: []string{"latency", "cost"},
		Scores: map[string]map[string]string{
			"V1": {"latency": "18ms"},
			"V2": {"latency": "42ms", "cost": "$120"},
		},
	}

	front, warnings := computeParetoFront(results, []string{"V1", "V2"}, []charDim{
		{Name: "latency", Role: "target", Polarity: "lower_better"},
		{Name: "cost", Role: "target", Polarity: "lower_better"},
	}, MissingDataPolicyExclude)

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
	if len(front) != 1 || front[0] != "V1" {
		t.Fatalf("expected exclude policy to keep latency dominance and front [V1], got %+v", front)
	}
}

func TestCompareDimensionValues_ZeroFillInheritsNumericUnit(t *testing.T) {
	comparison, status := compareDimensionValues("", "18ms", "lower_better", MissingDataPolicyZero)
	if status != dimensionComparisonComparable {
		t.Fatalf("expected comparable numeric zero-fill status, got %v", status)
	}
	if comparison != 1 {
		t.Fatalf("expected filled zero to beat 18ms on lower_better dimension, got %d", comparison)
	}

	comparison, status = compareDimensionValues("", "$120", "lower_better", MissingDataPolicyZero)
	if status != dimensionComparisonComparable {
		t.Fatalf("expected comparable currency zero-fill status, got %v", status)
	}
	if comparison != 1 {
		t.Fatalf("expected filled zero to beat $120 on lower_better dimension, got %d", comparison)
	}
}

func TestCompareDimensionValues_ZeroFillInheritsOrdinalScale(t *testing.T) {
	comparison, status := compareDimensionValues("", "High", "lower_better", MissingDataPolicyZero)
	if status != dimensionComparisonComparable {
		t.Fatalf("expected comparable ordinal zero-fill status, got %v", status)
	}
	if comparison != 1 {
		t.Fatalf("expected filled ordinal zero to beat High on lower_better dimension, got %d", comparison)
	}

	comparison, status = compareDimensionValues("Low", "", "higher_better", MissingDataPolicyZero)
	if status != dimensionComparisonComparable {
		t.Fatalf("expected comparable ordinal zero-fill status, got %v", status)
	}
	if comparison != 1 {
		t.Fatalf("expected Low to beat filled ordinal zero on higher_better dimension, got %d", comparison)
	}
}

func TestCompareDimensionValues_ZeroFillTreatsMalformedPeerAsUnresolved(t *testing.T) {
	comparison, status := compareDimensionValues("", "unknown", "lower_better", MissingDataPolicyZero)
	if status != dimensionComparisonUnresolved {
		t.Fatalf("expected unresolved status for malformed zero-fill peer, got %v", status)
	}
	if comparison != 0 {
		t.Fatalf("expected no comparison result for malformed zero-fill peer, got %d", comparison)
	}
}

func TestCompareSolutions_ReplacesExisting(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	p, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("A", "x", "Keep the runtime surface small"),
			testVariant("B", "y", "Bias toward operational elasticity"),
		},
		NoSteppingStoneRationale: "Both options are direct candidates.",
	})

	// First comparison
	CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: p.Meta.ID,
		Results: ComparisonResult{
			Dimensions:      []string{"speed"},
			Scores:          map[string]map[string]string{"A": {"speed": "fast"}, "B": {"speed": "slow"}},
			NonDominatedSet: []string{"A"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "B",
					DominatedBy: []string{"A"},
					Summary:     "Lower speed with no offsetting advantage in this comparison.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "A", Summary: "Fastest option in the current comparison set."},
			},
			SelectedRef: "A",
		},
	})

	// Second comparison replaces
	a, _, _ := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: p.Meta.ID,
		Results: ComparisonResult{
			Dimensions:      []string{"cost"},
			Scores:          map[string]map[string]string{"A": {"cost": "high"}, "B": {"cost": "low"}},
			NonDominatedSet: []string{"B"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "A",
					DominatedBy: []string{"B"},
					Summary:     "Higher cost with no compensating advantage in this comparison.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "B", Summary: "Lowest cost option in the current comparison set."},
			},
			SelectedRef: "B",
		},
	})

	// Should have cost, not speed
	if strings.Contains(a.Body, "speed") && strings.Contains(a.Body, "## Comparison") {
		// speed should only appear in the variants section, not in comparison
		compIdx := strings.Index(a.Body, "## Comparison")
		compSection := a.Body[compIdx:]
		if strings.Contains(compSection, "speed") {
			t.Error("old comparison dimension 'speed' should be replaced in comparison section")
		}
	}
	if !strings.Contains(a.Body, "cost") {
		t.Error("missing new comparison dimension 'cost'")
	}
	if !strings.Contains(a.Body, "**Recommendation (advisory):** B") {
		t.Error("missing updated recommendation")
	}
}

func TestFindActivePortfolio(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	// None yet
	p, _ := FindActivePortfolio(ctx, store, "")
	if p != nil {
		t.Error("expected nil for no portfolios")
	}

	// Create one
	ExploreSolutions(ctx, store, t.TempDir(), ExploreInput{
		Context: "test",
		Variants: []Variant{
			testVariant("A", "x", "Keep the runtime surface small"),
			testVariant("B", "y", "Bias toward operational elasticity"),
		},
		NoSteppingStoneRationale: "Both options are direct candidates.",
	})

	p, _ = FindActivePortfolio(ctx, store, "test")
	if p == nil {
		t.Fatal("expected active portfolio")
	}
	if p.Meta.Kind != KindSolutionPortfolio {
		t.Errorf("kind = %q", p.Meta.Kind)
	}
}

// --- Characterization cross-check tests ---

func TestCompare_ErrorsOnMissingCharacterizedDimensions(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Create problem with characterization
	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Cache strategy", Signal: "High latency", Context: "perf",
	})
	CharacterizeProblem(ctx, store, haftDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{
			{Name: "latency"},
			{Name: "cost"},
			{Name: "complexity"},
		},
		ParityRules: "Same workload for all variants",
	})

	// Create portfolio linked to problem
	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("Redis", "SPOF", "Keep operational familiarity with Redis-based caching"),
			testVariant("Memcached", "no persistence", "Strip the solution down to pure volatile cache performance"),
		},
		NoSteppingStoneRationale: "Both caching options are evaluated as direct production choices.",
	})

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"Redis":     {"latency": "1ms"},
				"Memcached": {"latency": "0.5ms"},
			},
			NonDominatedSet: []string{"Memcached"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "Redis",
					DominatedBy: []string{"Memcached"},
					Summary:     "Higher latency with no offsetting benefit on the compared dimensions.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Memcached", Summary: "Wins the current comparison on latency."},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing characterized target dimensions")
	}
	if !strings.Contains(err.Error(), "cost") {
		t.Fatalf("expected error to mention missing cost dimension, got %v", err)
	}
}

func TestCompare_NoWarningsWhenFullCoverage(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Cache strategy", Signal: "High latency", Context: "perf",
	})
	CharacterizeProblem(ctx, store, haftDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{
			{Name: "latency"},
			{Name: "cost"},
		},
		ParityPlan: &ParityPlan{
			BaselineSet:       []string{"Redis", "Memcached"},
			Window:            "same 30m synthetic load window",
			Budget:            "$100/month",
			MissingDataPolicy: MissingDataPolicyExplicitAbstain,
			PinnedConditions:  []string{"Same dataset and host class for both variants"},
		},
	})

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("Redis", "SPOF", "Keep operational familiarity with Redis-based caching"),
			testVariant("Memcached", "no persistence", "Strip the solution down to pure volatile cache performance"),
		},
		NoSteppingStoneRationale: "Both caching options are evaluated as direct production choices.",
	})

	// Compare on all characterized dimensions with full scores
	a, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency", "cost"},
			Scores: map[string]map[string]string{
				"Redis":     {"latency": "1ms", "cost": "$100"},
				"Memcached": {"latency": "0.5ms", "cost": "$50"},
			},
			NonDominatedSet: []string{"Memcached"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "Redis",
					DominatedBy: []string{"Memcached"},
					Summary:     "Higher latency and higher cost than Memcached in this run.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Memcached", Summary: "Best latency and cost in the compared pair."},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Parity checklist is always shown when characterization exists (advisory)
	// But there should be NO dimension-mismatch or scoring-gap warnings
	if strings.Contains(a.Body, "Characterized dimensions not in comparison") {
		t.Error("should NOT have dimension mismatch warning when all covered")
	}
	if strings.Contains(a.Body, "missing scores") {
		t.Error("should NOT have scoring gap warning when all scored")
	}
}

func TestCompare_WarnsWithoutParityPlanInStandardMode(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Portfolio without linked problem (no characterization possible)
	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("A", "x", "Keep the runtime surface small"),
			testVariant("B", "y", "Bias toward operational elasticity"),
		},
		NoSteppingStoneRationale: "Both options are direct candidates.",
	})

	a, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions:      []string{"speed"},
			Scores:          map[string]map[string]string{"A": {"speed": "fast"}, "B": {"speed": "slow"}},
			NonDominatedSet: []string{"A"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "B",
					DominatedBy: []string{"A"},
					Summary:     "Lower speed with no compensating benefit in this comparison.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "A", Summary: "Best speed result among the compared variants."},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(a.Body, "Comparison Warnings") {
		t.Fatal("expected warning section without parity plan in standard mode")
	}
	if !strings.Contains(a.Body, "without a parity plan") {
		t.Fatalf("expected missing parity plan warning, body: %s", a.Body)
	}
}

func TestCompare_ErrorsOnAsymmetricScoring(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "DB choice", Signal: "Scale issues", Context: "data",
	})
	CharacterizeProblem(ctx, store, haftDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{{Name: "throughput"}, {Name: "cost"}},
	})

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("Postgres", "sharding complexity", "Scale by keeping the familiar Postgres toolchain"),
			testVariant("CockroachDB", "cost", "Adopt distributed SQL to avoid manual sharding"),
		},
		NoSteppingStoneRationale: "Both database options are direct production candidates.",
	})

	// Postgres scored on both, CockroachDB missing cost
	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"throughput", "cost"},
			Scores: map[string]map[string]string{
				"Postgres":    {"throughput": "50k/s", "cost": "$200"},
				"CockroachDB": {"throughput": "100k/s"},
			},
			NonDominatedSet: []string{"Postgres", "CockroachDB"},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Postgres", Summary: "Lower cost, but lower throughput headroom."},
				{Variant: "CockroachDB", Summary: "Higher throughput, but incomplete cost evidence in this fixture."},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing target score")
	}
	if !strings.Contains(err.Error(), "target dimension 'cost'") {
		t.Fatalf("expected target-dimension error, got %v", err)
	}
}

func TestCompare_ParityChecklist(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "DB choice", Signal: "Scale issues", Context: "data",
	})
	CharacterizeProblem(ctx, store, haftDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{
			{Name: "throughput"},
			{Name: "cost"},
		},
		ParityPlan: &ParityPlan{
			BaselineSet:       []string{"Postgres", "CockroachDB"},
			Window:            "same 45m load window",
			Budget:            "$800/month",
			MissingDataPolicy: MissingDataPolicyExplicitAbstain,
			PinnedConditions:  []string{"Identical dataset and region for both measurements"},
		},
	})

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("Postgres", "sharding", "Scale by keeping the familiar Postgres toolchain"),
			testVariant("CockroachDB", "cost", "Adopt distributed SQL to avoid manual sharding"),
		},
		NoSteppingStoneRationale: "Both database options are direct production candidates.",
	})

	a, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"throughput", "cost"},
			Scores: map[string]map[string]string{
				"Postgres":    {"throughput": "50k/s", "cost": "$200"},
				"CockroachDB": {"throughput": "100k/s", "cost": "$800"},
			},
			NonDominatedSet: []string{"Postgres", "CockroachDB"},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Postgres", Summary: "Lower cost, but lower throughput headroom."},
				{Variant: "CockroachDB", Summary: "Higher throughput, but materially higher cost."},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Should have parity checklist for each dimension
	if !strings.Contains(a.Body, "Parity checklist") {
		t.Error("should have parity checklist")
	}
	if !strings.Contains(a.Body, "throughput") || !strings.Contains(a.Body, "same conditions") {
		t.Error("should have parity question for throughput")
	}
	if !strings.Contains(a.Body, "cost") {
		t.Error("should have parity question for cost")
	}
}

func TestExtractCharacterizedDimensions(t *testing.T) {
	body := `# Problem

## Signal

Something broken

## Characterization v1 (2026-03-17)

| Dimension | Scale | Unit | Polarity | Measurement |
|-----------|-------|------|----------|-------------|
| latency | ratio | ms | lower_better | p99 |
| throughput | ratio | req/s | higher_better | load test |
| cost | ratio | $/mo | lower_better | invoice |

**Parity rules:** Same workload
`
	dims := extractCharacterizedDimensions(body)
	if len(dims) != 3 {
		t.Fatalf("expected 3 dimensions, got %d: %v", len(dims), dims)
	}
	if dims[0] != "latency" || dims[1] != "throughput" || dims[2] != "cost" {
		t.Errorf("dimensions = %v", dims)
	}
}

func TestExtractCharacterizedDimensions_NoCharacterization(t *testing.T) {
	body := "# Problem\n\n## Signal\n\nSomething\n"
	dims := extractCharacterizedDimensions(body)
	if dims != nil {
		t.Errorf("expected nil, got %v", dims)
	}
}

// --- Diversity check tests ---

func TestJaccardSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		min  float64
		max  float64
	}{
		{"Redis TTL cache", "Redis LRU cache", 0.3, 0.7},                              // similar but distinct
		{"Redis cache", "Kafka streams", 0.0, 0.1},                                    // completely different
		{"foo bar baz", "foo bar baz", 0.99, 1.01},                                    // identical
		{"database migration strategy", "strategy for database migration", 0.7, 1.01}, // reordered
		{"", "", -0.1, 0.1}, // both empty
	}
	for _, tc := range cases {
		sim := jaccardSimilarity(tc.a, tc.b)
		if sim < tc.min || sim > tc.max {
			t.Errorf("jaccard(%q, %q) = %.2f, want [%.2f, %.2f]", tc.a, tc.b, sim, tc.min, tc.max)
		}
	}
}

func TestExplore_DiversityWarning(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	a, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			{Title: "Redis with TTL-based cache eviction strategy", Description: "Use Redis TTL for cache", WeakestLink: "SPOF", NoveltyMarker: "Expire entries proactively via TTL-driven eviction"},
			{Title: "Redis with LRU-based cache eviction strategy", Description: "Use Redis LRU for cache", WeakestLink: "memory", NoveltyMarker: "Expire entries reactively via LRU-driven eviction"},
			{Title: "Kafka Streams for event processing", Description: "Completely different approach", WeakestLink: "complexity", NoveltyMarker: "Move cache invalidation into an event-streaming topology"},
		},
		NoSteppingStoneRationale: "All options are framed as end-state implementation choices.",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Redis pair should trigger warning, Kafka should not
	if !strings.Contains(a.Body, "Diversity Warnings") {
		t.Error("should have diversity warnings for similar Redis variants")
	}
	if !strings.Contains(a.Body, "Redis with TTL") && !strings.Contains(a.Body, "Redis with LRU") {
		t.Error("warning should mention the similar pair")
	}
}

func TestExplore_NoDiversityWarning(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	a, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("PostgreSQL with sharding", "ops complexity", "Scale by adding explicit shard routing"),
			testVariant("CockroachDB distributed SQL", "cost", "Scale via distributed SQL with automatic balancing"),
		},
		NoSteppingStoneRationale: "Both database approaches are direct production targets.",
	})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(a.Body, "Diversity Warnings") {
		t.Error("should NOT have diversity warnings for distinct variants")
	}
}

func TestFrame_RecallRelatedHistory(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Create an existing decision about caching
	Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Redis for session cache",
		WhySelected:   "Low latency, simple ops",
	}))

	// Frame a new problem about caching — should recall the decision
	prob, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:  "Cache invalidation causing stale data",
		Signal: "Users seeing old data after updates",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(prob.Body, "Related History") {
		t.Error("should show related history when similar artifacts exist")
	}
	if !strings.Contains(prob.Body, "Redis") || !strings.Contains(prob.Body, "cache") {
		t.Error("related history should include the cache decision")
	}
}

func TestFrame_RecallBySignal(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	// Create a decision about Redis
	Decide(ctx, store, haftDir, completeDecision(DecideInput{
		SelectedTitle: "Redis eviction strategy",
		WhySelected:   "TTL-based eviction for session tokens",
	}))

	// Frame a problem with different title but signal mentions Redis
	prob, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:  "Data consistency issues in user sessions",
		Signal: "Redis TTL not expiring, users seeing stale session data",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Signal contains "Redis" — should recall the Redis decision
	if !strings.Contains(prob.Body, "Related History") {
		t.Error("should recall related history via signal keyword match")
	}
}

func TestFrame_NoRecallWhenNoHistory(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title:  "Completely unique problem about quantum computing",
		Signal: "Qubits decoherence rate exceeds threshold",
	})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(prob.Body, "Related History") {
		t.Error("should NOT show related history when no similar artifacts exist")
	}
}

func TestCompare_WarnsOnExpiredDimensionMeasurement(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Performance issue", Signal: "Slow API", Context: "perf",
	})
	CharacterizeProblem(ctx, store, haftDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{
			{Name: "latency", ValidUntil: "2020-01-01T00:00:00Z"}, // expired
			{Name: "cost"}, // no expiry
		},
	})

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("A", "x", "Optimize for minimal coordination overhead"),
			testVariant("B", "y", "Optimize for cost predictability"),
		},
		NoSteppingStoneRationale: "Both options are direct candidates.",
	})

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency", "cost"},
			Scores: map[string]map[string]string{
				"A": {"latency": "10ms", "cost": "$100"},
				"B": {"latency": "5ms", "cost": "$200"},
			},
			NonDominatedSet: []string{"A", "B"},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "A", Summary: "Lower cost, but worse latency."},
				{Variant: "B", Summary: "Lower latency, but higher cost."},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for expired characterization dimension")
	}
	if !strings.Contains(err.Error(), "expired on 2020-01-01") {
		t.Fatalf("expected expiry error, got %v", err)
	}
}

func TestExtractDimensionsWithValidUntil(t *testing.T) {
	body := `# Problem

## Characterization v1 (2026-03-17)

| Dimension | Role | Scale | Unit | Polarity | Measurement | Valid Until |
|-----------|------|-------|------|----------|-------------|-------------|
| latency | constraint | ratio | ms | lower_better | p99 | 2026-06-01 |
| cost | target | ratio | $/mo | lower_better | invoice | - |
`
	dims := extractCharacterizedDimensionsWithRoles(body)
	if len(dims) != 2 {
		t.Fatalf("expected 2 dims, got %d", len(dims))
	}
	if dims[0].ValidUntil != "2026-06-01" {
		t.Errorf("latency valid_until = %q, want '2026-06-01'", dims[0].ValidUntil)
	}
	if dims[1].ValidUntil != "" {
		t.Errorf("cost valid_until = %q, want empty", dims[1].ValidUntil)
	}
}

func TestParityPlan_JSONRoundTrip(t *testing.T) {
	plan := ParityPlan{
		BaselineSet:       []string{"A", "B"},
		Window:            "same 30m load window",
		Budget:            "$100/month",
		MissingDataPolicy: MissingDataPolicyExplicitAbstain,
		Normalization:     []NormRule{{Dimension: "latency", Method: "p99"}},
		PinnedConditions:  []string{"Same dataset", "Same region"},
	}

	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ParityPlan
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}

	if err := ValidateParityPlan(decoded); err != nil {
		t.Fatalf("decoded parity plan should validate: %v", err)
	}
	if got := decoded.MissingDataPolicy; got != MissingDataPolicyExplicitAbstain {
		t.Fatalf("missing_data_policy = %q", got)
	}
	if len(decoded.Normalization) != 1 || decoded.Normalization[0].Dimension != "latency" {
		t.Fatalf("normalization round-trip failed: %+v", decoded.Normalization)
	}
}

func TestExplore_WarnsOnDuplicateNoveltyMarkers(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	a, _, err := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("A", "x", "Run the service inside the API process"),
			testVariant("B", "y", "Run the service inside the API process"),
		},
		NoSteppingStoneRationale: "Both options are intended as direct implementation candidates.",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(a.Body, "Novelty markers for 'A' and 'B'") {
		t.Fatalf("expected novelty-marker warning, body: %s", a.Body)
	}
}

func TestCompare_DeepModeRequiresParityPlan(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Transport choice", Signal: "Latency variance", Context: "api", Mode: "deep",
	})
	CharacterizeProblem(ctx, store, haftDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{{Name: "latency", Role: "target"}},
	})

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("REST", "chatty serialization", "Keep the existing HTTP semantics"),
			testVariant("gRPC", "tooling overhead", "Adopt binary RPC for lower-latency transport"),
		},
		NoSteppingStoneRationale: "Both transports are direct architecture candidates.",
	})

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"REST": {"latency": "42ms"},
				"gRPC": {"latency": "18ms"},
			},
			NonDominatedSet: []string{"gRPC"},
		},
	})
	if err == nil {
		t.Fatal("expected deep-mode parity error")
	}
	if !strings.Contains(err.Error(), "requires a parity plan") {
		t.Fatalf("expected parity-plan error, got %v", err)
	}
}

func TestCompare_PersistsStructuredComparison(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Transport choice", Signal: "Latency variance", Context: "api",
	})
	CharacterizeProblem(ctx, store, haftDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{
			{Name: "latency", Role: "target"},
			{Name: "cost", Role: "constraint"},
		},
		ParityPlan: &ParityPlan{
			BaselineSet:       []string{"REST", "gRPC"},
			Window:            "same 15m replay window",
			Budget:            "$200/month",
			MissingDataPolicy: MissingDataPolicyExplicitAbstain,
			PinnedConditions:  []string{"Same dataset and region"},
		},
	})

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("REST", "chatty serialization", "Keep the existing HTTP semantics"),
			testVariant("gRPC", "tooling overhead", "Adopt binary RPC for lower-latency transport"),
		},
		NoSteppingStoneRationale: "Both transports are direct architecture candidates.",
	})

	a, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency", "cost"},
			Scores: map[string]map[string]string{
				"REST": {"latency": "42ms", "cost": "$100"},
				"gRPC": {"latency": "18ms", "cost": "$180"},
			},
			NonDominatedSet: []string{"REST", "gRPC"},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "REST", Summary: "Lower cost, but higher latency."},
				{Variant: "gRPC", Summary: "Lower latency, but higher cost."},
			},
			SelectedRef: "gRPC",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Body, "Baseline set: REST, gRPC") {
		t.Fatalf("expected parity plan baseline to render human-readable labels, body: %s", a.Body)
	}

	fields := a.UnmarshalPortfolioFields()
	if fields.Comparison == nil {
		t.Fatal("expected structured comparison payload")
	}
	if fields.Comparison.ParityPlan == nil {
		t.Fatal("expected parity plan in structured comparison payload")
	}
	if got := fields.Comparison.ParityPlan.BaselineSet[0]; got != "V1" {
		t.Fatalf("unexpected structured parity baseline: %+v", fields.Comparison.ParityPlan.BaselineSet)
	}
}

func TestCompare_ErrorsWhenExploredVariantIsOmittedFromScores(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("A", "ops complexity", "Keep the implementation inside the existing service boundary"),
			testVariant("B", "tooling overhead", "Move the workflow to a dedicated worker service"),
			testVariant("C", "migration cost", "Adopt a managed external platform"),
		},
		NoSteppingStoneRationale: "All three options are direct implementation candidates.",
	})

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"A": {"latency": "20ms"},
				"B": {"latency": "15ms"},
			},
			NonDominatedSet: []string{"B"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "A",
					DominatedBy: []string{"B"},
					Summary:     "Higher latency than B in the compared subset.",
				},
				{
					Variant:     "C",
					DominatedBy: []string{"B"},
					Summary:     "Missing score should still require an elimination explanation.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "B", Summary: "Lowest latency among the declared variants."},
			},
		},
	})
	if err == nil {
		t.Fatal("expected missing explored variant error")
	}
	if !strings.Contains(err.Error(), "target dimension 'latency' missing scores for variants: V3") {
		t.Fatalf("unexpected omission error: %v", err)
	}
}

func TestCompare_ErrorsOnScoredVariantOutsideStructuredBaseline(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, haftDir, ProblemFrameInput{
		Title: "Transport choice", Signal: "Latency variance", Context: "api",
	})
	CharacterizeProblem(ctx, store, haftDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{{Name: "latency", Role: "target"}},
		ParityPlan: &ParityPlan{
			BaselineSet:       []string{"REST", "gRPC"},
			Window:            "same 15m replay window",
			Budget:            "$200/month",
			MissingDataPolicy: MissingDataPolicyExplicitAbstain,
		},
	})

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			testVariant("REST", "chatty serialization", "Keep the existing HTTP semantics"),
			testVariant("gRPC", "tooling overhead", "Adopt binary RPC for lower-latency transport"),
			testVariant("GraphQL", "resolver complexity", "Adopt a graph-based query boundary"),
		},
		NoSteppingStoneRationale: "All three transports are direct architecture candidates.",
	})

	_, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"latency"},
			Scores: map[string]map[string]string{
				"REST":    {"latency": "42ms"},
				"gRPC":    {"latency": "18ms"},
				"GraphQL": {"latency": "55ms"},
			},
			NonDominatedSet: []string{"gRPC"},
		},
	})
	if err == nil {
		t.Fatal("expected scored variant outside baseline error")
	}
	if !strings.Contains(err.Error(), `scored variant "V3" is outside the declared compare set`) {
		t.Fatalf("unexpected parity-set error: %v", err)
	}
}

func TestCompare_AcceptsGeneratedVariantIDs(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := t.TempDir()

	portfolio, _, _ := ExploreSolutions(ctx, store, haftDir, ExploreInput{
		Variants: []Variant{
			testVariant("Kafka", "ops complexity", "Maximize throughput with established streaming ecosystem"),
			testVariant("NATS", "ecosystem maturity", "Lean embedded broker with simpler cluster operations"),
			testVariant("Redis Streams", "durability risk", "Reuse the existing Redis footprint for the fastest rollout"),
		},
		NoSteppingStoneRationale: "All options are direct implementation candidates.",
	})

	a, _, err := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: portfolio.Meta.ID,
		Results: ComparisonResult{
			Dimensions: []string{"throughput", "cost"},
			Scores: map[string]map[string]string{
				"V1": {"throughput": "200k/s", "cost": "$800"},
				"V2": {"throughput": "100k/s", "cost": "$200"},
				"V3": {"throughput": "80k/s", "cost": "$250"},
			},
			NonDominatedSet: []string{"V1", "V2"},
			DominatedVariants: []DominatedVariantExplanation{
				{
					Variant:     "Redis Streams",
					DominatedBy: []string{"NATS"},
					Summary:     "Lower throughput with no offsetting cost advantage over NATS.",
				},
			},
			ParetoTradeoffs: []ParetoTradeoffNote{
				{Variant: "Kafka", Summary: "Highest throughput, highest cost."},
				{Variant: "NATS", Summary: "Lower cost, lower throughput headroom."},
			},
			SelectedRef:             "V2",
			RecommendationRationale: "Use the lower-cost option when both remain on the frontier.",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(a.Body, "**Recommendation (advisory):** NATS") {
		t.Fatalf("expected human-readable recommended title, body: %s", a.Body)
	}

	fields := a.UnmarshalPortfolioFields()
	if fields.Comparison == nil {
		t.Fatal("expected structured comparison payload")
	}
	if _, ok := fields.Comparison.Scores["V1"]; !ok {
		t.Fatalf("expected structured comparison to keep canonical V1 score key: %+v", fields.Comparison.Scores)
	}
	if fields.Comparison.SelectedRef != "V2" {
		t.Fatalf("expected structured comparison selected_ref to stay canonical, got %q", fields.Comparison.SelectedRef)
	}
	if got := fields.Comparison.DominatedVariants[0].Variant; got != "V3" {
		t.Fatalf("expected dominated variant explanation to stay canonical, got %q", got)
	}
	if got := fields.Comparison.DominatedVariants[0].DominatedBy[0]; got != "V2" {
		t.Fatalf("expected dominated_by alias to normalize to V2, got %q", got)
	}
	if got := fields.Comparison.ParetoTradeoffs[0].Variant; got != "V1" {
		t.Fatalf("expected Pareto trade-off alias to normalize to V1, got %q", got)
	}
	if got := fields.Comparison.RecommendationRationale; got != "Use the lower-cost option when both remain on the frontier." {
		t.Fatalf("unexpected recommendation rationale: %q", got)
	}
}
