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
