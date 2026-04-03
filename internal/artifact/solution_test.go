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
				"Redis Streams": {"throughput": "80k/s", "ops complexity": "Low", "cost": "$100"},
			},
			NonDominatedSet: []string{"Kafka", "NATS"},
			PolicyApplied:   "Minimize ops complexity at sufficient throughput (>50k/s)",
			SelectedRef:     "NATS",
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
	if !strings.Contains(a.Body, "**Recommended:** NATS") {
		t.Error("missing recommendation")
	}
	if a.Meta.Version != 2 {
		t.Errorf("version = %d, want 2 after comparison", a.Meta.Version)
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

func TestCompareSolutions_NoPareto(t *testing.T) {
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
			Dimensions: []string{"speed"},
		},
	})
	if err == nil {
		t.Error("expected error for missing non_dominated_set")
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
			SelectedRef:     "A",
		},
	})

	// Second comparison replaces
	a, _, _ := CompareSolutions(ctx, store, haftDir, CompareInput{
		PortfolioRef: p.Meta.ID,
		Results: ComparisonResult{
			Dimensions:      []string{"cost"},
			Scores:          map[string]map[string]string{"A": {"cost": "high"}, "B": {"cost": "low"}},
			NonDominatedSet: []string{"B"},
			SelectedRef:     "B",
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
	if !strings.Contains(a.Body, "**Recommended:** B") {
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
	Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Redis for session cache",
		WhySelected:   "Low latency, simple ops",
	})

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
	Decide(ctx, store, haftDir, DecideInput{
		SelectedTitle: "Redis eviction strategy",
		WhySelected:   "TTL-based eviction for session tokens",
	})

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
			SelectedRef:     "gRPC",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	fields := a.UnmarshalPortfolioFields()
	if fields.Comparison == nil {
		t.Fatal("expected structured comparison payload")
	}
	if fields.Comparison.ParityPlan == nil {
		t.Fatal("expected parity plan in structured comparison payload")
	}
	if got := fields.Comparison.ParityPlan.BaselineSet[0]; got != "REST" {
		t.Fatalf("unexpected structured parity baseline: %+v", fields.Comparison.ParityPlan.BaselineSet)
	}
}
