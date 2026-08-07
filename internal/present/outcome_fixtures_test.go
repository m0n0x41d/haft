package present

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/codeintel"
)

func fixtureScope(t *testing.T) codeintel.TraversalScope {
	t.Helper()
	scope, err := codeintel.NewTraversalScope(1, "test:index:epoch:1")
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func fixtureBudget(t *testing.T) codeintel.TraversalBudget {
	t.Helper()
	budget, err := codeintel.NewTraversalBudget(7, 8)
	if err != nil {
		t.Fatal(err)
	}
	return budget
}

func fixtureStableSymbol(
	t *testing.T,
	id string,
) codeintel.StableSymbol {
	t.Helper()
	symbol, err := codeintel.NewStableSymbol(id)
	if err != nil {
		t.Fatal(err)
	}
	return symbol
}

func fixtureResolvedSeed(
	t *testing.T,
	id string,
) codeintel.SeedResolution {
	t.Helper()
	basis, err := codeintel.ParseResolvedSeedBasis("unique_exact_name")
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := codeintel.NewResolvedSeed(
		fixtureStableSymbol(t, id),
		basis,
		fixtureScope(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

func fixtureCandidateSet(
	t *testing.T,
	basisCode string,
	ids ...string,
) codeintel.SeedResolution {
	t.Helper()
	basis, err := codeintel.ParseCandidateSetBasis(basisCode)
	if err != nil {
		t.Fatal(err)
	}
	candidates := make([]codeintel.StableSymbol, 0, len(ids))
	for _, id := range ids {
		candidates = append(candidates, fixtureStableSymbol(t, id))
	}
	outcome, err := codeintel.NewCandidateSet(
		candidates,
		basis,
		fixtureScope(t),
	)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

func fixtureSeedUnavailable(
	t *testing.T,
	reasonCode string,
) codeintel.SeedResolution {
	t.Helper()
	reason, err := codeintel.ParseSeedUnavailableReason(reasonCode)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := codeintel.NewIndexObservation(
		1,
		"test:index:unaccounted-coverage",
	)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := codeintel.NewSeedUnavailable(
		"missing",
		reason,
		observation,
	)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

func fixtureChainOutcome(
	t *testing.T,
	terminationCode string,
	bridge bool,
) codeintel.ChainOutcome {
	t.Helper()
	seed := fixtureStableSymbol(t, "sym:test:seed")
	hops := []codeintel.GraphHop{}
	visited := int64(1)
	inspected := int64(0)
	depth := int64(0)
	bridges := int64(0)
	if bridge {
		target := fixtureStableSymbol(t, "sym:test:target")
		hop, err := codeintel.NewGraphHop(
			seed,
			target,
			codebase.EdgeInterfaceDispatch,
			codebase.ProvenanceHeuristic,
		)
		if err != nil {
			t.Fatal(err)
		}
		hops = append(hops, hop)
		visited = 2
		inspected = 1
		depth = 1
		bridges = 1
	}
	stats, err := codeintel.NewTraversalStats(
		fixtureScope(t),
		fixtureBudget(t),
		visited,
		inspected,
		depth,
		bridges,
	)
	if err != nil {
		t.Fatal(err)
	}
	termination, err := codeintel.ParseChainTermination(terminationCode)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := codeintel.NewChainOutcome(
		seed,
		hops,
		termination,
		stats,
	)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

func fixturePathFound(
	t *testing.T,
	fromID string,
	toID string,
) codeintel.PathOutcome {
	t.Helper()
	from := fixtureStableSymbol(t, fromID)
	to := fixtureStableSymbol(t, toID)
	hop, err := codeintel.NewGraphHop(
		from,
		to,
		codebase.EdgeCall,
		codebase.ProvenanceStatic,
	)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := codeintel.NewTraversalStats(
		fixtureScope(t),
		fixtureBudget(t),
		2,
		1,
		1,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := codeintel.NewPathFound(
		from,
		[]codeintel.GraphHop{hop},
		stats,
	)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

func fixturePathAbsent(t *testing.T) codeintel.PathOutcome {
	t.Helper()
	stats, err := codeintel.NewTraversalStats(
		fixtureScope(t),
		fixtureBudget(t),
		2,
		1,
		1,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	completion, err := codeintel.NewCompletedTraversal(stats)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := codeintel.NewPathAbsentWithinIndexedGraph(completion)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

func fixtureBagDirection(
	t *testing.T,
	code string,
) codeintel.BagLegDirection {
	t.Helper()
	direction, err := codeintel.ParseBagLegDirection(code)
	if err != nil {
		t.Fatal(err)
	}
	return direction
}
