package present

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/codeintel"
	"github.com/m0n0x41d/haft/internal/contextgraph"
	"github.com/m0n0x41d/haft/internal/graph"
)

func TestExploreResponse_SpineBlastSourceFused(t *testing.T) {
	res := codeintel.ExploreResult{
		Seed:           codebase.CodeSymbol{Name: "FrameProblem", FilePath: "internal/artifact/problem.go", StartLine: 147},
		SeedResolution: fixtureResolvedSeed(t, "sym:test:frame-problem"),
		SeedBody:       "func FrameProblem() error { return nil }",
		SeedBodyOK:     true,
		ChainOutcome:   fixtureChainOutcome(t, "leaf_reached", true),
		Chain: []codeintel.ChainStep{
			{
				Symbol:   codebase.CodeSymbol{Name: "FrameProblem", FilePath: "internal/artifact/problem.go", StartLine: 147},
				Distance: 0,
				Context:  contextgraph.CodeContext{Decisions: []*artifact.Artifact{decArtifact("dec-seed", "framing rule")}},
			},
			{
				Symbol:     codebase.CodeSymbol{Name: "Create", Receiver: "Store", FilePath: "internal/artifact/store.go", StartLine: 44},
				Distance:   1,
				ViaKind:    codebase.EdgeInterfaceDispatch,
				Provenance: codebase.ProvenanceHeuristic,
				Context:    contextgraph.CodeContext{Decisions: []*artifact.Artifact{decArtifact("dec-store", "store contract")}},
			},
		},
		BlastRadius: []codeintel.FusedHop{
			{Symbol: codebase.CodeSymbol{Name: "handleProblem", FilePath: "internal/cli/serve.go", StartLine: 400}, Context: contextgraph.CodeContext{Decisions: []*artifact.Artifact{decArtifact("dec-cli", "cli")}}},
		},
	}

	out := ExploreResponse(res, "FrameProblem", "go")

	if !strings.Contains(out, "spine of 2") {
		t.Errorf("chain length not reported:\n%s", out)
	}
	if !strings.Contains(out, "dec-seed") || !strings.Contains(out, "dec-store") {
		t.Errorf("per-on-chain-symbol fusion must render (the moat):\n%s", out)
	}
	if !strings.Contains(out, "heuristic boundary") {
		t.Errorf("dispatch bridge on the spine must be flagged:\n%s", out)
	}
	if !strings.Contains(out, "Blast radius") || !strings.Contains(out, "handleProblem") || !strings.Contains(out, "dec-cli") {
		t.Errorf("blast radius + covering decisions must render:\n%s", out)
	}
	if !strings.Contains(out, "func FrameProblem()") {
		t.Errorf("verbatim seed source must be included (0–1 Read sufficiency):\n%s", out)
	}
}

// Module invariants are shared across on-chain symbols; the spine must collect
// them ONCE in "Invariants in play", not repeat them on every hop (the verbosity
// defect that buried the signal).
func TestExploreResponse_InvariantsDedupedNotPerNode(t *testing.T) {
	inv := []graph.Invariant{{Text: "no second runtime", DecisionTitle: "extend in place"}}
	res := codeintel.ExploreResult{
		Seed:           codebase.CodeSymbol{Name: "A", FilePath: "x.go", StartLine: 1},
		SeedResolution: fixtureResolvedSeed(t, "sym:test:a"),
		SeedBody:       "func A() {}",
		SeedBodyOK:     true,
		ChainOutcome:   fixtureChainOutcome(t, "leaf_reached", false),
		Chain: []codeintel.ChainStep{
			{Symbol: codebase.CodeSymbol{Name: "A", FilePath: "x.go", StartLine: 1}, Distance: 0, Context: contextgraph.CodeContext{Invariants: inv}},
			{Symbol: codebase.CodeSymbol{Name: "B", FilePath: "x.go", StartLine: 9}, Distance: 1, ViaKind: codebase.EdgeCall, Context: contextgraph.CodeContext{Invariants: inv}},
			{Symbol: codebase.CodeSymbol{Name: "C", FilePath: "x.go", StartLine: 20}, Distance: 2, ViaKind: codebase.EdgeCall, Context: contextgraph.CodeContext{Invariants: inv}},
		},
	}
	out := ExploreResponse(res, "A", "go")
	if n := strings.Count(out, "no second runtime"); n != 1 {
		t.Errorf("shared invariant must appear exactly once (was the per-node verbosity bug), appeared %d times:\n%s", n, out)
	}
	if !strings.Contains(out, "Invariants in play (1)") {
		t.Errorf("invariants must be collected in a single section:\n%s", out)
	}
}

func TestExploreResponse_UnresolvedBoundaryHonest(t *testing.T) {
	res := codeintel.ExploreResult{
		Seed:           codebase.CodeSymbol{Name: "Dispatch", FilePath: "x.go", StartLine: 1},
		SeedResolution: fixtureResolvedSeed(t, "sym:test:dispatch"),
		ChainOutcome:   fixtureChainOutcome(t, "unresolved_dispatch_boundary", false),
		SeedBodyOK:     false,
		Chain: []codeintel.ChainStep{
			{Symbol: codebase.CodeSymbol{Name: "Dispatch", FilePath: "x.go", StartLine: 1}},
		},
	}
	out := ExploreResponse(res, "Dispatch", "go")
	if !strings.Contains(out, "unresolved dispatch boundary") {
		t.Errorf("a chain stopping at a boundary must say so, not imply completeness:\n%s", out)
	}
	if strings.Contains(out, "```go") {
		t.Errorf("unverified seed body must not be printed:\n%s", out)
	}
}

func TestExploreResponse_AmbiguousAndNotFound(t *testing.T) {
	amb := ExploreResponse(codeintel.ExploreResult{
		SeedResolution: fixtureCandidateSet(
			t,
			"ambiguous_exact_name",
			"sym:test:s:a",
			"sym:test:s:b",
		),
		Candidates: []codebase.CodeSymbol{
			{Name: "S", FilePath: "a.go", StartLine: 1},
			{Name: "S", FilePath: "b.go", StartLine: 2},
		},
	}, "S", "go")
	if !strings.Contains(amb, "ambiguous") {
		t.Errorf("ambiguous seed must be surfaced:\n%s", amb)
	}
	nf := ExploreResponse(codeintel.ExploreResult{}, "Nope", "go")
	if !strings.Contains(nf, "not found") {
		t.Errorf("missing seed must say not found:\n%s", nf)
	}
}

func TestExploreBagResponse_ConnectedNoPathAndUnresolved(t *testing.T) {
	res := codeintel.ExploreBagResult{
		Index:      codebase.IndexState{Epoch: 7},
		Resolution: codebase.ResolutionCounts{Resolved: 4, Ambiguous: 1, Unresolved: 2},
		Seeds: []codebase.CodeSymbol{
			{Name: "A", FilePath: "a.go", StartLine: 1},
			{Name: "C", FilePath: "c.go", StartLine: 9},
			{Name: "Z", FilePath: "z.go", StartLine: 3},
		},
		Unresolved: []string{"Huh"},
		Legs: []codeintel.BagLeg{
			{
				From:      codebase.CodeSymbol{Name: "A", FilePath: "a.go", StartLine: 1},
				To:        codebase.CodeSymbol{Name: "C", FilePath: "c.go", StartLine: 9},
				Forward:   fixturePathFound(t, "sym:test:a", "sym:test:c"),
				Reverse:   fixturePathAbsent(t),
				Direction: fixtureBagDirection(t, "forward"),
				Steps: []codeintel.ChainStep{
					{Symbol: codebase.CodeSymbol{Name: "A", FilePath: "a.go", StartLine: 1}, Distance: 0},
					{Symbol: codebase.CodeSymbol{Name: "B", FilePath: "b.go", StartLine: 5}, Distance: 1, ViaKind: codebase.EdgeCall},
					{Symbol: codebase.CodeSymbol{Name: "C", FilePath: "c.go", StartLine: 9}, Distance: 2, ViaKind: codebase.EdgeCall},
				},
			},
			{
				From:      codebase.CodeSymbol{Name: "C", FilePath: "c.go", StartLine: 9},
				To:        codebase.CodeSymbol{Name: "Z", FilePath: "z.go", StartLine: 3},
				Forward:   fixturePathAbsent(t),
				Reverse:   fixturePathAbsent(t),
				Direction: fixtureBagDirection(t, "none"),
			},
		},
	}
	out := ExploreBagResponse(res)
	if !strings.Contains(out, "A → C") || !strings.Contains(out, "**B**") {
		t.Errorf("connected leg should render the path A→...→C:\n%s", out)
	}
	if !strings.Contains(out, "C ⇸ Z") || !strings.Contains(out, "no selected static path") {
		t.Errorf("disconnected leg must say no static path, not bridge it:\n%s", out)
	}
	if !strings.Contains(out, "Huh") {
		t.Errorf("unresolved seed must be surfaced:\n%s", out)
	}
	if !strings.Contains(out, "Index epoch: 7") || !strings.Contains(out, "4 resolved • 1 ambiguous • 2 unresolved") {
		t.Errorf("multi-seed explore must retain epoch and aggregate resolution truth:\n%s", out)
	}
}
