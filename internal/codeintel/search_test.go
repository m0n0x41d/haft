package codeintel

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/codebase"

	_ "modernc.org/sqlite"
)

func TestShortestPath_FindsAndReportsNoPath(t *testing.T) {
	out := map[string][]codebase.CodeEdge{
		"A": {edge("A", "B")},
		"B": {edge("B", "C")},
		"C": {edge("C", "D")},
		// X is isolated.
	}
	src := fakeEdges{out: out}
	ctx := context.Background()
	scope, budget := testTraversalBasis(t, 7, ChainVisitBudget)

	outcome, err := shortestPath(ctx, src, "A", "C", scope, budget)
	if err != nil {
		t.Fatal(err)
	}
	found, ok := outcome.(pathFound)
	if !ok {
		t.Fatalf("A→C outcome = %s, want path_found", outcome.Kind().String())
	}
	got := chainIDs(chainHopsFromPrefix(found.path))
	if len(got) != 3 || got[0] != "A" || got[1] != "B" || got[2] != "C" {
		t.Fatalf("path A→B→C expected, got %v", got)
	}
	outcome, err = shortestPath(ctx, src, "A", "X", scope, budget)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Kind().String() != "path_absent_within_indexed_graph" {
		t.Fatalf("A→X outcome = %s, want bounded absence", outcome.Kind().String())
	}
	outcome, err = shortestPath(ctx, src, "A", "A", scope, budget)
	if err != nil {
		t.Fatal(err)
	}
	found, ok = outcome.(pathFound)
	if !ok || len(chainHopsFromPrefix(found.path)) != 1 {
		t.Fatalf("from==to outcome = %s, want one-node path", outcome.Kind().String())
	}
}

func TestShortestPath_BoundsAndStoreErrorsStayDistinct(t *testing.T) {
	ctx := context.Background()
	out := map[string][]codebase.CodeEdge{
		"A": {edge("A", "B"), edge("A", "X")},
		"B": {edge("B", "C")},
	}
	scope, maxHopBudget := testTraversalBasis(t, 1, 8)
	outcome, err := shortestPath(
		ctx,
		fakeEdges{out: out},
		"A",
		"C",
		scope,
		maxHopBudget,
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Kind().String() != "path_truncated" ||
		outcome.DetailCode() != "max_hops" {
		t.Fatalf(
			"max-hop outcome = %s/%s",
			outcome.Kind().String(),
			outcome.DetailCode(),
		)
	}

	_, visitBudget := testTraversalBasis(t, 7, 2)
	outcome, err = shortestPath(
		ctx,
		fakeEdges{out: out},
		"A",
		"C",
		scope,
		visitBudget,
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Kind().String() != "path_truncated" ||
		outcome.DetailCode() != "visit_budget" {
		t.Fatalf(
			"visit-budget outcome = %s/%s",
			outcome.Kind().String(),
			outcome.DetailCode(),
		)
	}

	wantErr := errors.New("edge store unavailable")
	_, err = shortestPath(
		ctx,
		failingEdgeSource{err: wantErr},
		"A",
		"C",
		scope,
		maxHopBudget,
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("store error = %v, want %v", err, wantErr)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = shortestPath(
		cancelled,
		contextEdgeSource{},
		"A",
		"C",
		scope,
		maxHopBudget,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v, want context.Canceled", err)
	}
}

type failingEdgeSource struct {
	err error
}

func (f failingEdgeSource) OutEdges(
	context.Context,
	string,
) ([]codebase.CodeEdge, error) {
	return nil, f.err
}

func (f failingEdgeSource) InEdges(
	context.Context,
	string,
) ([]codebase.CodeEdge, error) {
	return nil, f.err
}

type contextEdgeSource struct{}

func (contextEdgeSource) OutEdges(
	ctx context.Context,
	_ string,
) ([]codebase.CodeEdge, error) {
	return nil, ctx.Err()
}

func (contextEdgeSource) InEdges(
	ctx context.Context,
	_ string,
) ([]codebase.CodeEdge, error) {
	return nil, ctx.Err()
}

func TestExploreBagDirection_MixedOutcomes(t *testing.T) {
	scope, budget := testTraversalBasis(t, 1, 8)
	out := map[string][]codebase.CodeEdge{
		"A": {edge("A", "B")},
		"B": {edge("B", "C")},
		"C": {edge("C", "A")},
	}
	forward, err := shortestPath(
		context.Background(),
		fakeEdges{out: out},
		"A",
		"C",
		scope,
		budget,
	)
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := shortestPath(
		context.Background(),
		fakeEdges{out: out},
		"C",
		"A",
		scope,
		budget,
	)
	if err != nil {
		t.Fatal(err)
	}
	if forward.Kind().String() != "path_truncated" {
		t.Fatalf("forward = %s, want path_truncated", forward.Kind().String())
	}
	if reverse.Kind().String() != "path_found" {
		t.Fatalf("reverse = %s, want path_found", reverse.Kind().String())
	}
	if got := selectBagLegDirection(forward, reverse).String(); got != "reverse" {
		t.Fatalf("selected direction = %s, want reverse", got)
	}
	if got := selectBagLegDirection(reverse, reverse).String(); got != "forward" {
		t.Fatalf("two found paths must deterministically prefer forward, got %s", got)
	}
}

// The decision's hard invariant: when a fuzzy seed is AMBIGUOUS (>1 substring
// match), resolveSeed must return candidates and NEVER silently pick one.
func TestResolveSeed_FuzzyFallbackNeverSilentlyPicks(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "cg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := codebase.NewSymbolStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	src := `package p
func FrameProblem() {}
func FrameThing() {}
func Lonely() {}
`
	if err := os.WriteFile(filepath.Join(root, "p.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.IndexFileSymbols(ctx, root, "p.go"); err != nil {
		t.Fatal(err)
	}
	svc := &Service{symbols: store}

	// Exact name → seed, not fuzzy.
	seed, cands, fuzzy, err := svc.resolveSeed(ctx, "FrameProblem", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if seed.Name != "FrameProblem" || len(cands) != 0 || fuzzy {
		t.Fatalf("exact name should resolve directly, got seed=%q cands=%d fuzzy=%v", seed.Name, len(cands), fuzzy)
	}
	scope, _ := testTraversalBasis(t, 7, ChainVisitBudget)
	outcome, _, err := classifySeedResolution(
		"FrameProblem",
		"",
		0,
		seed,
		cands,
		fuzzy,
		scope,
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Kind().String() != "resolved_seed" {
		t.Fatalf("exact classification = %s, want resolved_seed", outcome.Kind().String())
	}

	// Partial name matching >1 → candidates, fuzzy, NO seed (never silently pick).
	seed, cands, fuzzy, err = svc.resolveSeed(ctx, "Frame", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if seed.ID != "" {
		t.Fatalf("ambiguous fuzzy MUST NOT pick a seed silently, got %q", seed.Name)
	}
	if len(cands) != 2 || !fuzzy {
		t.Fatalf("ambiguous fuzzy should return 2 candidates + fuzzy=true, got cands=%d fuzzy=%v", len(cands), fuzzy)
	}

	// Partial name matching exactly 1 → that single fuzzy hit, labeled fuzzy.
	seed, cands, fuzzy, err = svc.resolveSeed(ctx, "Lone", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if seed.Name != "Lonely" || len(cands) != 0 || !fuzzy {
		t.Fatalf("single fuzzy hit should resolve labeled fuzzy, got seed=%q cands=%d fuzzy=%v", seed.Name, len(cands), fuzzy)
	}
	outcome, displayCandidates, err := classifySeedResolution(
		"Lone",
		"",
		0,
		seed,
		cands,
		fuzzy,
		scope,
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Kind().String() != "candidate_set" ||
		outcome.DetailCode() != "fuzzy_candidates" ||
		len(displayCandidates) != 1 {
		t.Fatalf(
			"single fuzzy classification = %s/%s candidates=%d",
			outcome.Kind().String(),
			outcome.DetailCode(),
			len(displayCandidates),
		)
	}

	// No match at all → nothing.
	seed, cands, _, err = svc.resolveSeed(ctx, "Zzz", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if seed.ID != "" || len(cands) != 0 {
		t.Fatalf("no match should resolve to nothing, got seed=%q cands=%d", seed.Name, len(cands))
	}
	incompleteScope, err := NewTraversalScopeWithCoverage(
		scope.Epoch(),
		"test:index:bounded-with-exclusions",
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	outcome, _, err = classifySeedResolution(
		"Zzz",
		"",
		0,
		seed,
		cands,
		false,
		incompleteScope,
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Kind().String() != "seed_unavailable" ||
		outcome.DetailCode() != "index_incomplete" {
		t.Fatalf(
			"legacy unaccounted corpus classification = %s/%s",
			outcome.Kind().String(),
			outcome.DetailCode(),
		)
	}
	outcome, _, err = classifySeedResolution(
		"Zzz",
		"",
		0,
		seed,
		cands,
		false,
		scope,
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Kind().String() != "seed_not_found" {
		t.Fatalf(
			"complete corpus classification = %s/%s",
			outcome.Kind().String(),
			outcome.DetailCode(),
		)
	}
}
