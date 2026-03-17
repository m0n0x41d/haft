package artifact

import (
	"context"
	"strings"
	"testing"
)

func TestExploreSolutions_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	// Create a problem first
	prob, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Event infrastructure", Signal: "DB polling hitting 70% CPU", Context: "events",
	})

	input := ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{
				Title:       "Kafka",
				Description: "High throughput, battle-tested, complex ops",
				Strengths:   []string{"200k events/sec", "Proven at scale"},
				WeakestLink: "Operational complexity (ZooKeeper, rebalancing)",
				Risks:       []string{"Requires dedicated ops expertise"},
			},
			{
				Title:         "NATS JetStream",
				Description:   "Simpler ops, embedded, growing ecosystem",
				Strengths:     []string{"100k events/sec", "Minimal ops"},
				WeakestLink:   "Ecosystem maturity at scale",
				SteppingStone: true,
			},
			{
				Title:       "Redis Streams",
				Description: "Already have Redis, minimal new infra",
				WeakestLink: "Not designed for durable event sourcing",
				Risks:       []string{"Data loss risk under pressure"},
			},
		},
	}

	a, filePath, err := ExploreSolutions(ctx, store, quintDir, input)
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
			{Title: "A", WeakestLink: "something"},
			{Title: "B", WeakestLink: ""},
		},
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
			{Title: "A", WeakestLink: "x"},
			{Title: "B", WeakestLink: "y"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should work without problem ref — tactical mode
	if a.Meta.Kind != KindSolutionPortfolio {
		t.Errorf("kind = %q", a.Meta.Kind)
	}
}

func TestCompareSolutions_Success(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	// Create portfolio
	portfolio, _, _ := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		Variants: []Variant{
			{Title: "Kafka", WeakestLink: "ops complexity"},
			{Title: "NATS", WeakestLink: "ecosystem maturity"},
			{Title: "Redis Streams", WeakestLink: "durability risk"},
		},
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

	a, _, err := CompareSolutions(ctx, store, quintDir, input)
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
	quintDir := t.TempDir()

	p, _, _ := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		Variants: []Variant{{Title: "A", WeakestLink: "x"}, {Title: "B", WeakestLink: "y"}},
	})

	_, _, err := CompareSolutions(ctx, store, quintDir, CompareInput{
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
	quintDir := t.TempDir()

	p, _, _ := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		Variants: []Variant{{Title: "A", WeakestLink: "x"}, {Title: "B", WeakestLink: "y"}},
	})

	_, _, err := CompareSolutions(ctx, store, quintDir, CompareInput{
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
	quintDir := t.TempDir()

	p, _, _ := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		Variants: []Variant{{Title: "A", WeakestLink: "x"}, {Title: "B", WeakestLink: "y"}},
	})

	// First comparison
	CompareSolutions(ctx, store, quintDir, CompareInput{
		PortfolioRef: p.Meta.ID,
		Results: ComparisonResult{
			Dimensions:      []string{"speed"},
			Scores:          map[string]map[string]string{"A": {"speed": "fast"}, "B": {"speed": "slow"}},
			NonDominatedSet: []string{"A"},
			SelectedRef:     "A",
		},
	})

	// Second comparison replaces
	a, _, _ := CompareSolutions(ctx, store, quintDir, CompareInput{
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
		Context:  "test",
		Variants: []Variant{{Title: "A", WeakestLink: "x"}, {Title: "B", WeakestLink: "y"}},
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

func TestCompare_WarnsOnMissingCharacterizedDimensions(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	// Create problem with characterization
	prob, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Cache strategy", Signal: "High latency", Context: "perf",
	})
	CharacterizeProblem(ctx, store, quintDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{
			{Name: "latency"},
			{Name: "cost"},
			{Name: "complexity"},
		},
		ParityRules: "Same workload for all variants",
	})

	// Create portfolio linked to problem
	portfolio, _, _ := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{Title: "Redis", WeakestLink: "SPOF"},
			{Title: "Memcached", WeakestLink: "no persistence"},
		},
	})

	// Compare on only 1 of 3 characterized dimensions
	a, _, err := CompareSolutions(ctx, store, quintDir, CompareInput{
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
	if err != nil {
		t.Fatal(err)
	}

	// Should warn about missing cost and complexity
	if !strings.Contains(a.Body, "Comparison Warnings") {
		t.Error("missing Comparison Warnings section")
	}
	if !strings.Contains(a.Body, "cost") {
		t.Error("should warn about missing 'cost' dimension")
	}
	if !strings.Contains(a.Body, "complexity") {
		t.Error("should warn about missing 'complexity' dimension")
	}
	if !strings.Contains(a.Body, "parity rules") {
		t.Error("should remind about parity rules")
	}
}

func TestCompare_NoWarningsWhenFullCoverage(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "Cache strategy", Signal: "High latency", Context: "perf",
	})
	CharacterizeProblem(ctx, store, quintDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{
			{Name: "latency"},
			{Name: "cost"},
		},
	})

	portfolio, _, _ := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{Title: "Redis", WeakestLink: "SPOF"},
			{Title: "Memcached", WeakestLink: "no persistence"},
		},
	})

	// Compare on all characterized dimensions with full scores
	a, _, err := CompareSolutions(ctx, store, quintDir, CompareInput{
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

func TestCompare_NoWarningsWithoutCharacterization(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	// Portfolio without linked problem (no characterization possible)
	portfolio, _, _ := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		Variants: []Variant{
			{Title: "A", WeakestLink: "x"},
			{Title: "B", WeakestLink: "y"},
		},
	})

	a, _, err := CompareSolutions(ctx, store, quintDir, CompareInput{
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

	if strings.Contains(a.Body, "Comparison Warnings") {
		t.Error("should NOT have warnings without characterization")
	}
}

func TestCompare_WarnsOnAsymmetricScoring(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "DB choice", Signal: "Scale issues", Context: "data",
	})
	CharacterizeProblem(ctx, store, quintDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{{Name: "throughput"}, {Name: "cost"}},
	})

	portfolio, _, _ := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{Title: "Postgres", WeakestLink: "sharding complexity"},
			{Title: "CockroachDB", WeakestLink: "cost"},
		},
	})

	// Postgres scored on both, CockroachDB missing cost
	a, _, err := CompareSolutions(ctx, store, quintDir, CompareInput{
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
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(a.Body, "CockroachDB missing scores") {
		t.Errorf("should warn about CockroachDB missing cost score, body: %s", a.Body)
	}
}

func TestCompare_ParityChecklist(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, _ := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
		Title: "DB choice", Signal: "Scale issues", Context: "data",
	})
	CharacterizeProblem(ctx, store, quintDir, CharacterizeInput{
		ProblemRef: prob.Meta.ID,
		Dimensions: []ComparisonDimension{
			{Name: "throughput"},
			{Name: "cost"},
		},
	})

	portfolio, _, _ := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		ProblemRef: prob.Meta.ID,
		Variants: []Variant{
			{Title: "Postgres", WeakestLink: "sharding"},
			{Title: "CockroachDB", WeakestLink: "cost"},
		},
	})

	a, _, err := CompareSolutions(ctx, store, quintDir, CompareInput{
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
		{"Redis TTL cache", "Redis LRU cache", 0.3, 0.7},          // similar but distinct
		{"Redis cache", "Kafka streams", 0.0, 0.1},                 // completely different
		{"foo bar baz", "foo bar baz", 0.99, 1.01},                  // identical
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
	quintDir := t.TempDir()

	a, _, err := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		Variants: []Variant{
			{Title: "Redis with TTL-based cache eviction strategy", Description: "Use Redis TTL for cache", WeakestLink: "SPOF"},
			{Title: "Redis with LRU-based cache eviction strategy", Description: "Use Redis LRU for cache", WeakestLink: "memory"},
			{Title: "Kafka Streams for event processing", Description: "Completely different approach", WeakestLink: "complexity"},
		},
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
	quintDir := t.TempDir()

	a, _, err := ExploreSolutions(ctx, store, quintDir, ExploreInput{
		Variants: []Variant{
			{Title: "PostgreSQL with sharding", WeakestLink: "ops complexity"},
			{Title: "CockroachDB distributed SQL", WeakestLink: "cost"},
		},
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
	quintDir := t.TempDir()

	// Create an existing decision about caching
	Decide(ctx, store, quintDir, DecideInput{
		SelectedTitle: "Redis for session cache",
		WhySelected:   "Low latency, simple ops",
	})

	// Frame a new problem about caching — should recall the decision
	prob, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
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

func TestFrame_NoRecallWhenNoHistory(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	quintDir := t.TempDir()

	prob, _, err := FrameProblem(ctx, store, quintDir, ProblemFrameInput{
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
