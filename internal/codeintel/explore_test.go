package codeintel

import (
	"context"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/codebase"
)

// dedge is a heuristic interface-dispatch edge (a "bridge").
func dedge(s, d string) codebase.CodeEdge {
	return codebase.CodeEdge{SrcID: s, DstID: d, Kind: codebase.EdgeInterfaceDispatch, Provenance: codebase.ProvenanceHeuristic}
}

func chainIDs(hops []chainHop) []string {
	out := make([]string, len(hops))
	for i, h := range hops {
		out[i] = h.NodeID
	}
	return out
}

func testTraversalBasis(
	t *testing.T,
	maxHops int64,
	visitBudget int64,
) (TraversalScope, TraversalBudget) {
	t.Helper()
	scope, err := NewTraversalScope(1, "test:index:epoch:1")
	if err != nil {
		t.Fatal(err)
	}
	budget, err := NewTraversalBudget(maxHops, visitBudget)
	if err != nil {
		t.Fatal(err)
	}
	return scope, budget
}

func chainOutcomeIDs(outcome ChainOutcome) []string {
	return chainIDs(chainHopsFromPrefix(outcome.path))
}

func TestLongestChain_DeepestStaticPath(t *testing.T) {
	out := map[string][]codebase.CodeEdge{
		"A": {edge("A", "B"), edge("A", "X")}, // B-branch is deeper than X
		"B": {edge("B", "C")},
		"C": {edge("C", "D")},
	}
	src := fakeEdges{out: out}
	scope, budget := testTraversalBasis(t, 7, ChainVisitBudget)
	outcome, err := longestChain(
		context.Background(),
		src,
		"A",
		scope,
		budget,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	got := chainOutcomeIDs(outcome)
	if len(got) != 4 || got[0] != "A" || got[3] != "D" {
		t.Fatalf("deepest path A→B→C→D expected, got %v", got)
	}
	if outcome.Termination().String() != "leaf_reached" ||
		outcome.Stats().BridgeCount() != 0 {
		t.Fatalf(
			"static path: termination=%s bridges=%d",
			outcome.Termination().String(),
			outcome.Stats().BridgeCount(),
		)
	}
}

func TestLongestChain_MaxHopsCap(t *testing.T) {
	out := map[string][]codebase.CodeEdge{
		"A": {edge("A", "B")},
		"B": {edge("B", "C")},
		"C": {edge("C", "D")},
	}
	scope, budget := testTraversalBasis(t, 2, ChainVisitBudget)
	outcome, err := longestChain(
		context.Background(),
		fakeEdges{out: out},
		"A",
		scope,
		budget,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := chainOutcomeIDs(outcome); len(got) != 3 { // seed + 2 hops
		t.Fatalf("maxHops 2 should cap chain at 3 nodes, got %v", got)
	}
	if outcome.Termination().String() != "max_hops_reached" {
		t.Fatalf("termination = %s, want max_hops_reached", outcome.Termination().String())
	}
}

// One bridge may be crossed; a second is refused and the chain reports that it
// ends at an unresolved dispatch boundary rather than crossing it silently.
func TestLongestChain_BridgeBudgetAndUnresolvedEnd(t *testing.T) {
	out := map[string][]codebase.CodeEdge{
		"A": {dedge("A", "B")}, // bridge 1 — allowed
		"B": {dedge("B", "C")}, // bridge 2 — refused (budget 1)
	}
	scope, budget := testTraversalBasis(t, 7, ChainVisitBudget)
	outcome, err := longestChain(
		context.Background(),
		fakeEdges{out: out},
		"A",
		scope,
		budget,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	got := chainOutcomeIDs(outcome)
	if len(got) != 2 || got[1] != "B" {
		t.Fatalf("chain should cross exactly one bridge to B, got %v", got)
	}
	if outcome.Stats().BridgeCount() != 1 {
		t.Fatalf("expected 1 bridge used, got %d", outcome.Stats().BridgeCount())
	}
	if outcome.Termination().String() != "unresolved_dispatch_boundary" {
		t.Fatalf(
			"termination = %s, want unresolved_dispatch_boundary",
			outcome.Termination().String(),
		)
	}
}

func TestLongestChain_PrefersStaticOverBridge(t *testing.T) {
	// From A: a static edge to a shallow node, and a bridge to a deep chain.
	// Static is preferred at equal opportunity, but longest still wins overall;
	// here the static branch is shorter, so the bridge chain (longer) is chosen
	// — verifying length dominates, with the bridge counted.
	out := map[string][]codebase.CodeEdge{
		"A": {edge("A", "S"), dedge("A", "B")},
		"B": {edge("B", "C")},
		"C": {edge("C", "D")},
	}
	scope, budget := testTraversalBasis(t, 7, ChainVisitBudget)
	outcome, err := longestChain(
		context.Background(),
		fakeEdges{out: out},
		"A",
		scope,
		budget,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := chainOutcomeIDs(outcome); len(got) != 4 || got[1] != "B" {
		t.Fatalf("longer bridge path A→B→C→D should win, got %v", got)
	}
	if outcome.Stats().BridgeCount() != 1 {
		t.Fatalf("bridge crossing must be counted, got %d", outcome.Stats().BridgeCount())
	}
}

func TestLongestChain_VisitBudgetIsExplicit(t *testing.T) {
	out := map[string][]codebase.CodeEdge{
		"A": {edge("A", "B"), edge("A", "C")},
		"B": {edge("B", "D")},
	}
	scope, budget := testTraversalBasis(t, 7, 2)
	outcome, err := longestChain(
		context.Background(),
		fakeEdges{out: out},
		"A",
		scope,
		budget,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Termination().String() != "visit_budget_reached" {
		t.Fatalf(
			"termination = %s, want visit_budget_reached",
			outcome.Termination().String(),
		)
	}
}

func TestLongestChain_StoreErrorPropagates(t *testing.T) {
	scope, budget := testTraversalBasis(t, 7, ChainVisitBudget)
	wantErr := errors.New("out-edge read failed")
	_, err := longestChain(
		context.Background(),
		failingEdgeSource{err: wantErr},
		"A",
		scope,
		budget,
		1,
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("store error = %v, want %v", err, wantErr)
	}
}

func TestIncompleteCoverageCannotClaimLeafOrDisconnectedPath(t *testing.T) {
	scope, err := NewTraversalScopeWithCoverage(
		1,
		"test:index:bounded-with-exclusions",
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	budget, err := NewTraversalBudget(7, ChainVisitBudget)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := longestChain(
		context.Background(),
		fakeEdges{},
		"A",
		scope,
		budget,
		1,
	)
	if err != nil {
		t.Fatal(err)
	}
	if chain.Termination().String() != "coverage_incomplete" {
		t.Fatalf(
			"incomplete leaf termination = %s",
			chain.Termination().String(),
		)
	}
	path, err := shortestPath(
		context.Background(),
		fakeEdges{},
		"A",
		"B",
		scope,
		budget,
	)
	if err != nil {
		t.Fatal(err)
	}
	if path.Kind().String() != "path_unavailable" ||
		path.DetailCode() != "index_capability" {
		t.Fatalf(
			"incomplete disconnected path = %s/%s",
			path.Kind().String(),
			path.DetailCode(),
		)
	}
}
