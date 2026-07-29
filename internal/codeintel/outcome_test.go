package codeintel_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/codeintel"
)

func TestHGTraversalOutcomeCorpus(t *testing.T) {
	scope := mustTraversalScope(t)
	budget := mustTraversalBudget(t, 4, 8)
	seed := mustStableSymbol(t, "sym:v2:a")
	second := mustStableSymbol(t, "sym:v2:b")
	third := mustStableSymbol(t, "sym:v2:c")

	t.Run("resolved seed", func(t *testing.T) {
		basis := mustResolvedSeedBasis(t, "exact_stable_id")
		outcome, err := codeintel.NewResolvedSeed(seed, basis, scope)
		if err != nil {
			t.Fatal(err)
		}
		requireKind(t, outcome.Kind().String(), "resolved_seed")
		requireJSONField(t, outcome, "basis", "exact_stable_id")
	})

	t.Run("candidate set remains unselected and stable ordered", func(t *testing.T) {
		basis := mustCandidateSetBasis(t, "ambiguous_exact_name")
		outcome, err := codeintel.NewCandidateSet(
			[]codeintel.StableSymbol{third, seed, second},
			basis,
			scope,
		)
		if err != nil {
			t.Fatal(err)
		}
		requireKind(t, outcome.Kind().String(), "candidate_set")
		raw := marshalCanonical(t, outcome)
		want := `"candidates":["sym:v2:a","sym:v2:b","sym:v2:c"]`
		if !strings.Contains(raw, want) {
			t.Fatalf("candidate order = %s", raw)
		}
	})

	t.Run("known seed absence requires scope", func(t *testing.T) {
		outcome, err := codeintel.NewSeedNotFound("missing symbol", scope)
		if err != nil {
			t.Fatal(err)
		}
		requireKind(t, outcome.Kind().String(), "seed_not_found")
		requireJSONField(t, outcome, "query", "missing symbol")
	})

	t.Run("seed unavailability does not fabricate coverage", func(t *testing.T) {
		reason := mustSeedUnavailableReason(t, "index_unavailable")
		observation, err := codeintel.NewIndexObservation(0, "index:none")
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := codeintel.NewSeedUnavailable(
			"current concern",
			reason,
			observation,
		)
		if err != nil {
			t.Fatal(err)
		}
		requireKind(t, outcome.Kind().String(), "seed_unavailable")
		requireJSONField(t, outcome, "reason", "index_unavailable")
	})

	oneHop := mustGraphHop(
		t,
		seed,
		second,
		codebase.EdgeCall,
		codebase.ProvenanceStatic,
	)
	twoHop := mustGraphHop(
		t,
		second,
		third,
		codebase.EdgeInterfaceDispatch,
		codebase.ProvenanceHeuristic,
	)

	t.Run("source equals target is a found zero-edge path", func(t *testing.T) {
		stats := mustTraversalStats(t, scope, budget, 1, 0, 0, 0)
		outcome, err := codeintel.NewPathFound(
			seed,
			[]codeintel.GraphHop{},
			stats,
		)
		if err != nil {
			t.Fatal(err)
		}
		requireKind(t, outcome.Kind().String(), "path_found")
	})

	t.Run("multi-hop found path keeps ordered provenance", func(t *testing.T) {
		stats := mustTraversalStats(t, scope, budget, 3, 4, 2, 1)
		outcome, err := codeintel.NewPathFound(
			seed,
			[]codeintel.GraphHop{oneHop, twoHop},
			stats,
		)
		if err != nil {
			t.Fatal(err)
		}
		requireKind(t, outcome.Kind().String(), "path_found")
		raw := marshalCanonical(t, outcome)
		if !strings.Contains(raw, `"provenance":"heuristic"`) {
			t.Fatalf("found path lost provenance: %s", raw)
		}
	})

	t.Run("complete bounded absence is its own variant", func(t *testing.T) {
		stats := mustTraversalStats(t, scope, budget, 3, 2, 1, 0)
		completion, err := codeintel.NewCompletedTraversal(stats)
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := codeintel.NewPathAbsentWithinIndexedGraph(completion)
		if err != nil {
			t.Fatal(err)
		}
		requireKind(
			t,
			outcome.Kind().String(),
			"path_absent_within_indexed_graph",
		)
	})

	t.Run("max hops is truncation not absence", func(t *testing.T) {
		tightBudget := mustTraversalBudget(t, 2, 8)
		stats := mustTraversalStats(t, scope, tightBudget, 3, 4, 2, 1)
		reason := mustPathTruncationReason(t, "max_hops")
		outcome, err := codeintel.NewPathTruncated(
			reason,
			seed,
			[]codeintel.GraphHop{oneHop, twoHop},
			stats,
		)
		if err != nil {
			t.Fatal(err)
		}
		requireKind(t, outcome.Kind().String(), "path_truncated")
		requireJSONField(t, outcome, "reason", "max_hops")
	})

	t.Run("visit budget is truncation not absence", func(t *testing.T) {
		tightBudget := mustTraversalBudget(t, 4, 2)
		stats := mustTraversalStats(t, scope, tightBudget, 2, 2, 1, 0)
		reason := mustPathTruncationReason(t, "visit_budget")
		outcome, err := codeintel.NewPathTruncated(
			reason,
			seed,
			[]codeintel.GraphHop{oneHop},
			stats,
		)
		if err != nil {
			t.Fatal(err)
		}
		requireKind(t, outcome.Kind().String(), "path_truncated")
		requireJSONField(t, outcome, "reason", "visit_budget")
	})

	t.Run("missing resolver capability is not absence", func(t *testing.T) {
		stats := mustTraversalStats(t, scope, budget, 1, 0, 0, 0)
		reason := mustPathUnavailableReason(t, "resolver_capability")
		outcome, err := codeintel.NewPathUnavailable(reason, stats)
		if err != nil {
			t.Fatal(err)
		}
		requireKind(t, outcome.Kind().String(), "path_unavailable")
		requireJSONField(t, outcome, "reason", "resolver_capability")
	})

	t.Run("chain path and termination are one outcome", func(t *testing.T) {
		stats := mustTraversalStats(t, scope, budget, 3, 4, 2, 1)
		termination, err := codeintel.ParseChainTermination("leaf_reached")
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := codeintel.NewChainOutcome(
			seed,
			[]codeintel.GraphHop{oneHop, twoHop},
			termination,
			stats,
		)
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Termination().String() != "leaf_reached" {
			t.Fatalf(
				"chain termination = %s",
				outcome.Termination().String(),
			)
		}
		requireJSONField(t, outcome, "termination", "leaf_reached")
	})

	t.Run("chain terminations remain exhaustive and distinct", func(t *testing.T) {
		codes := []string{
			"leaf_reached",
			"unresolved_dispatch_boundary",
			"max_hops_reached",
			"visit_budget_reached",
		}
		for _, code := range codes {
			termination, err := codeintel.ParseChainTermination(code)
			if err != nil {
				t.Fatal(err)
			}
			if termination.String() != code {
				t.Fatalf("termination = %q, want %q", termination.String(), code)
			}
			raw := marshalCanonical(t, termination)
			want := `"` + code + `"`
			if raw != want {
				t.Fatalf("termination JSON = %s, want %s", raw, want)
			}
		}
	})
}

func TestHGOutcomeConstructorsRejectInvalidStates(t *testing.T) {
	scope := mustTraversalScope(t)
	budget := mustTraversalBudget(t, 3, 4)
	seed := mustStableSymbol(t, "sym:v2:a")
	second := mustStableSymbol(t, "sym:v2:b")
	oneHop := mustGraphHop(
		t,
		seed,
		second,
		codebase.EdgeCall,
		codebase.ProvenanceStatic,
	)

	cases := []struct {
		name string
		run  func() error
	}{
		{
			name: "zero max hops",
			run: func() error {
				_, err := codeintel.NewTraversalBudget(0, 1)
				return err
			},
		},
		{
			name: "negative visit budget",
			run: func() error {
				_, err := codeintel.NewTraversalBudget(1, -1)
				return err
			},
		},
		{
			name: "overflow text budget",
			run: func() error {
				raw := "9223372036854775808"
				_, err := codeintel.ParseTraversalBudget("1", raw)
				return err
			},
		},
		{
			name: "zero published epoch",
			run: func() error {
				_, err := codeintel.NewTraversalScope(0, "coverage:complete")
				return err
			},
		},
		{
			name: "non-canonical symbol",
			run: func() error {
				_, err := codeintel.NewStableSymbol(" sym:v2:a")
				return err
			},
		},
		{
			name: "visited nodes exceed budget",
			run: func() error {
				_, err := codeintel.NewTraversalStats(scope, budget, 5, 1, 1, 0)
				return err
			},
		},
		{
			name: "bridges exceed depth",
			run: func() error {
				_, err := codeintel.NewTraversalStats(scope, budget, 2, 2, 1, 2)
				return err
			},
		},
		{
			name: "duplicate candidate",
			run: func() error {
				basis := mustCandidateSetBasis(t, "lexical_candidates")
				_, err := codeintel.NewCandidateSet(
					[]codeintel.StableSymbol{seed, seed},
					basis,
					scope,
				)
				return err
			},
		},
		{
			name: "empty not-found query",
			run: func() error {
				_, err := codeintel.NewSeedNotFound("  ", scope)
				return err
			},
		},
		{
			name: "discontinuous path",
			run: func() error {
				third := mustStableSymbol(t, "sym:v2:c")
				broken := mustGraphHop(
					t,
					third,
					second,
					codebase.EdgeCall,
					codebase.ProvenanceStatic,
				)
				stats := mustTraversalStats(t, scope, budget, 3, 2, 2, 0)
				_, err := codeintel.NewPathFound(
					seed,
					[]codeintel.GraphHop{oneHop, broken},
					stats,
				)
				return err
			},
		},
		{
			name: "path depth differs from hops",
			run: func() error {
				stats := mustTraversalStats(t, scope, budget, 2, 2, 2, 0)
				_, err := codeintel.NewPathFound(
					seed,
					[]codeintel.GraphHop{oneHop},
					stats,
				)
				return err
			},
		},
		{
			name: "max-hops reason without max-hops stats",
			run: func() error {
				stats := mustTraversalStats(t, scope, budget, 2, 2, 1, 0)
				reason := mustPathTruncationReason(t, "max_hops")
				_, err := codeintel.NewPathTruncated(
					reason,
					seed,
					[]codeintel.GraphHop{oneHop},
					stats,
				)
				return err
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.run(); err == nil {
				t.Fatal("invalid state was accepted")
			}
		})
	}

	unknownReasonParsers := []struct {
		name string
		run  func() error
	}{
		{
			name: "resolved seed basis",
			run: func() error {
				_, err := codeintel.ParseResolvedSeedBasis("ranked")
				return err
			},
		},
		{
			name: "candidate basis",
			run: func() error {
				_, err := codeintel.ParseCandidateSetBasis("truth_score")
				return err
			},
		},
		{
			name: "seed unavailable",
			run: func() error {
				_, err := codeintel.ParseSeedUnavailableReason("database_error")
				return err
			},
		},
		{
			name: "path truncation",
			run: func() error {
				_, err := codeintel.ParsePathTruncationReason("not_found")
				return err
			},
		},
		{
			name: "path unavailable",
			run: func() error {
				_, err := codeintel.ParsePathUnavailableReason("sqlite_error")
				return err
			},
		},
		{
			name: "chain termination",
			run: func() error {
				_, err := codeintel.ParseChainTermination("unknown")
				return err
			},
		},
	}
	for _, parser := range unknownReasonParsers {
		t.Run("unknown "+parser.name, func(t *testing.T) {
			if err := parser.run(); err == nil {
				t.Fatal("unknown closed code was accepted")
			}
		})
	}

	if _, err := json.Marshal(codeintel.StableSymbol{}); err == nil {
		t.Fatal("zero stable symbol serialized as a semantic value")
	}
	if _, err := json.Marshal(codeintel.TraversalBudget{}); err == nil {
		t.Fatal("zero traversal budget serialized as a semantic value")
	}
	if _, err := codeintel.NewPathAbsentWithinIndexedGraph(
		codeintel.CompletedTraversal{},
	); err == nil {
		t.Fatal("absence was constructed without completed traversal proof")
	}
	if _, err := codeintel.NewTraversalBudget(math.MaxInt64, 1); err == nil {
		t.Fatal("overflow-prone traversal budget was accepted")
	}
}

func TestHGOutcomeSerializationDeterministic(t *testing.T) {
	scope := mustTraversalScope(t)
	first := mustStableSymbol(t, "sym:v2:a")
	second := mustStableSymbol(t, "sym:v2:b")
	third := mustStableSymbol(t, "sym:v2:c")
	basis := mustCandidateSetBasis(t, "reasoning_candidates")
	outcome, err := codeintel.NewCandidateSet(
		[]codeintel.StableSymbol{third, first, second},
		basis,
		scope,
	)
	if err != nil {
		t.Fatal(err)
	}

	baseline := marshalCanonical(t, outcome)
	for range 20 {
		observed := marshalCanonical(t, outcome)
		if observed != baseline {
			t.Fatalf("serialization drifted:\n%s\n%s", baseline, observed)
		}
	}
}

func TestKnownAbsenceConstructorsRejectIncompleteCoverage(t *testing.T) {
	scope, err := codeintel.NewTraversalScopeWithCoverage(
		7,
		"coverage:bounded-with-exclusions",
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := codeintel.NewSeedNotFound(
		"missing",
		scope,
	); err == nil {
		t.Fatal("seed absence must require complete coverage")
	}
	stats := mustTraversalStats(
		t,
		scope,
		mustTraversalBudget(t, 4, 8),
		1,
		0,
		0,
		0,
	)
	if _, err := codeintel.NewCompletedTraversal(stats); err == nil {
		t.Fatal("completed traversal must require complete coverage")
	}
}

func mustStableSymbol(t *testing.T, id string) codeintel.StableSymbol {
	t.Helper()
	symbol, err := codeintel.NewStableSymbol(id)
	if err != nil {
		t.Fatal(err)
	}
	return symbol
}

func mustTraversalScope(t *testing.T) codeintel.TraversalScope {
	t.Helper()
	scope, err := codeintel.NewTraversalScope(7, "coverage:sha256:test")
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func mustTraversalBudget(
	t *testing.T,
	maxHops int64,
	visitBudget int64,
) codeintel.TraversalBudget {
	t.Helper()
	budget, err := codeintel.NewTraversalBudget(maxHops, visitBudget)
	if err != nil {
		t.Fatal(err)
	}
	return budget
}

func mustTraversalStats(
	t *testing.T,
	scope codeintel.TraversalScope,
	budget codeintel.TraversalBudget,
	visited int64,
	edges int64,
	depth int64,
	bridges int64,
) codeintel.TraversalStats {
	t.Helper()
	stats, err := codeintel.NewTraversalStats(
		scope,
		budget,
		visited,
		edges,
		depth,
		bridges,
	)
	if err != nil {
		t.Fatal(err)
	}
	return stats
}

func mustResolvedSeedBasis(
	t *testing.T,
	raw string,
) codeintel.ResolvedSeedBasis {
	t.Helper()
	basis, err := codeintel.ParseResolvedSeedBasis(raw)
	if err != nil {
		t.Fatal(err)
	}
	return basis
}

func mustCandidateSetBasis(
	t *testing.T,
	raw string,
) codeintel.CandidateSetBasis {
	t.Helper()
	basis, err := codeintel.ParseCandidateSetBasis(raw)
	if err != nil {
		t.Fatal(err)
	}
	return basis
}

func mustSeedUnavailableReason(
	t *testing.T,
	raw string,
) codeintel.SeedUnavailableReason {
	t.Helper()
	reason, err := codeintel.ParseSeedUnavailableReason(raw)
	if err != nil {
		t.Fatal(err)
	}
	return reason
}

func mustGraphHop(
	t *testing.T,
	from codeintel.StableSymbol,
	to codeintel.StableSymbol,
	kind codebase.EdgeKind,
	provenance codebase.Provenance,
) codeintel.GraphHop {
	t.Helper()
	hop, err := codeintel.NewGraphHop(from, to, kind, provenance)
	if err != nil {
		t.Fatal(err)
	}
	return hop
}

func mustPathTruncationReason(
	t *testing.T,
	raw string,
) codeintel.PathTruncationReason {
	t.Helper()
	reason, err := codeintel.ParsePathTruncationReason(raw)
	if err != nil {
		t.Fatal(err)
	}
	return reason
}

func mustPathUnavailableReason(
	t *testing.T,
	raw string,
) codeintel.PathUnavailableReason {
	t.Helper()
	reason, err := codeintel.ParsePathUnavailableReason(raw)
	if err != nil {
		t.Fatal(err)
	}
	return reason
}

func requireKind(t *testing.T, observed string, expected string) {
	t.Helper()
	if observed != expected {
		t.Fatalf("kind = %q, want %q", observed, expected)
	}
}

func requireJSONField(
	t *testing.T,
	value json.Marshaler,
	field string,
	expected string,
) {
	t.Helper()
	raw := marshalCanonical(t, value)
	decoded := make(map[string]any)
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	observed, found := decoded[field]
	if !found {
		t.Fatalf("JSON field %q is absent: %s", field, raw)
	}
	if observed != expected {
		t.Fatalf("JSON field %q = %#v, want %q", field, observed, expected)
	}
}

func marshalCanonical(t *testing.T, value json.Marshaler) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
