package graphrank

import (
	"math"
	"strings"
	"testing"
)

func chain() *Graph {
	g := NewGraph()
	g.AddEdge("A", "B", 1)
	g.AddEdge("B", "C", 1)
	g.AddEdge("C", "D", 1)
	return g
}

func TestRankProximityDecaysWithDistance(t *testing.T) {
	scores := chain().Rank(map[string]float64{"A": 1}, DefaultParams())
	// Seeded at A, mass decays along the chain: A > B > C > D.
	if !(scores["A"] > scores["B"] && scores["B"] > scores["C"] && scores["C"] > scores["D"]) {
		t.Fatalf("proximity ordering violated: %v", scores)
	}
	if scores["D"] <= 0 {
		t.Errorf("reachable node D should get nonzero mass: %v", scores)
	}
}

func TestRankLocalityAcrossComponents(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B", 1) // component 1
	g.AddEdge("X", "Y", 1) // component 2 — unreachable from A
	scores := g.Rank(map[string]float64{"A": 1}, DefaultParams())
	if scores["X"] != 0 || scores["Y"] != 0 {
		t.Fatalf("nodes in a disconnected component must get zero PPR mass, got X=%v Y=%v", scores["X"], scores["Y"])
	}
	if scores["A"] <= scores["B"] || scores["B"] <= 0 {
		t.Errorf("seed component should hold the mass: %v", scores)
	}
}

func TestRankProbabilityConserved(t *testing.T) {
	// Includes a dangling node (D has no out-edges) — its mass must teleport
	// back to the seed, not vanish, so the distribution still sums to ~1.
	scores := chain().Rank(map[string]float64{"A": 1}, DefaultParams())
	var sum float64
	for _, v := range scores {
		sum += v
	}
	if math.Abs(sum-1.0) > 1e-6 {
		t.Fatalf("PPR distribution must sum to ~1 (dangling mass conserved), got %v", sum)
	}
}

func TestRankWeightInfluence(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B", 1)
	g.AddEdge("A", "C", 3) // C draws 3x the flow of B
	scores := g.Rank(map[string]float64{"A": 1}, DefaultParams())
	if scores["C"] <= scores["B"] {
		t.Fatalf("heavier edge target C should outrank B: %v", scores)
	}
}

func TestRankDeterministic(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "B", 1)
	g.AddEdge("A", "C", 2)
	g.AddEdge("B", "C", 1)
	g.AddEdge("C", "A", 1)
	seeds := map[string]float64{"A": 1, "B": 0.5}
	first := g.Rank(seeds, DefaultParams())
	for range 25 {
		again := g.Rank(seeds, DefaultParams())
		for k, v := range first {
			if again[k] != v {
				t.Fatalf("Rank not deterministic for %q: %v vs %v", k, v, again[k])
			}
		}
	}
}

func TestRankNoValidSeedReturnsNil(t *testing.T) {
	g := chain()
	if r := g.Rank(map[string]float64{"Zzz": 1}, DefaultParams()); r != nil {
		t.Errorf("off-graph seed must yield nil, got %v", r)
	}
	if r := g.Rank(map[string]float64{}, DefaultParams()); r != nil {
		t.Errorf("empty seed must yield nil, got %v", r)
	}
	if r := NewGraph().Rank(map[string]float64{"A": 1}, DefaultParams()); r != nil {
		t.Errorf("empty graph must yield nil, got %v", r)
	}
}

func TestRankMultiSeedFavorsBothNeighborhoods(t *testing.T) {
	// Two stars; seeding both centers should rank both their leaves above an
	// unrelated, unseeded node only reachable from neither.
	g := NewGraph()
	g.AddEdge("c1", "l1", 1)
	g.AddEdge("c2", "l2", 1)
	g.AddEdge("iso", "void", 1)
	scores := g.Rank(map[string]float64{"c1": 1, "c2": 1}, DefaultParams())
	if scores["l1"] <= 0 || scores["l2"] <= 0 {
		t.Fatalf("both seeded neighborhoods should receive mass: %v", scores)
	}
	if scores["void"] != 0 {
		t.Errorf("unseeded, unreachable node must stay at 0: %v", scores)
	}
}

func TestAddEdgeIgnoresSelfLoopAndNonPositive(t *testing.T) {
	g := NewGraph()
	g.AddEdge("A", "A", 1)  // self-loop ignored
	g.AddEdge("A", "B", 0)  // zero weight ignored
	g.AddEdge("A", "C", -1) // negative ignored
	g.AddEdge("A", "D", 2)  // kept
	if g.Len() != 2 {       // only A and D registered
		t.Fatalf("expected 2 nodes (A,D), got %d: %v", g.Len(), g.Nodes())
	}
}

func TestInduceIsDeterministicBoundedAndLeavesCanonicalGraphUntouched(
	t *testing.T,
) {
	graph := NewGraph()
	graph.AddEdge("A", "B", 1)
	graph.AddEdge("B", "C", 1)
	graph.AddEdge("C", "D", 1)
	graph.AddEdge("C", "B", 1)
	graph.AddEdge("B", "E", 1)
	budget, err := NewInductionBudget(2, 3)
	if err != nil {
		t.Fatal(err)
	}
	first := graph.Induce([]string{"A"}, budget)
	second := graph.Induce([]string{"A"}, budget)
	if !first.NodeCapReached ||
		first.Graph.Len() != 3 ||
		graph.Len() != 5 {
		t.Fatalf(
			"induced=%v/%d canonical=%d",
			first.NodeCapReached,
			first.Graph.Len(),
			graph.Len(),
		)
	}
	if strings.Join(first.Graph.Nodes(), ",") !=
		strings.Join(second.Graph.Nodes(), ",") {
		t.Fatalf(
			"induction drift: %v vs %v",
			first.Graph.Nodes(),
			second.Graph.Nodes(),
		)
	}
	if len(first.Graph.out["C"]) != 1 ||
		first.Graph.out["C"][0].To != "B" {
		t.Fatalf(
			"boundary-to-internal edge was not preserved: %v",
			first.Graph.out["C"],
		)
	}
}
