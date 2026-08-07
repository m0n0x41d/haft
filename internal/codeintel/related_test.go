package codeintel

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/graph"
)

// A small fused graph:
//
//	f1.go: symA -> symB (call), symB -> symC (call, into f2.go)
//	dec-X affects f1.go ; dec-X -> note-Y (reasoning link)
//
// Seeding the f1.go file node should surface the artifact affecting it and the
// symbols defined in it at distance 1, with the once-removed note-Y and symC
// ranked strictly lower.
func TestBuildAndRankRelated(t *testing.T) {
	edges := []codebase.CodeEdge{
		{SrcID: "symA", DstID: "symB", Kind: codebase.EdgeCall},
		{SrcID: "symB", DstID: "symC", Kind: codebase.EdgeCall},
	}
	links := []artifact.LinkEdge{{Source: "dec-X", Target: "note-Y", Type: "relates_to"}}
	affected := []artifact.AffectedFileRef{{ArtifactID: "dec-X", FilePath: "f1.go"}}
	syms := []codebase.SymbolRef{
		{ID: "symA", FilePath: "f1.go"},
		{ID: "symB", FilePath: "f1.go"},
		{ID: "symC", FilePath: "f2.go"},
	}

	g, kind := buildFusedGraph(
		edges,
		links,
		affected,
		nil,
		syms,
		moduleFusionInputs{},
	)
	seed := map[string]float64{fileNode("f1.go"): 1}
	got := rankRelated(g, kind, seed, 0)

	idx := map[string]RelatedNode{}
	for _, n := range got {
		idx[n.ID] = n
		if n.Kind == relatedFile {
			t.Fatalf("file connector node leaked into results: %q", n.ID)
		}
		if n.ID == fileNode("f1.go") {
			t.Fatalf("seed must not appear in its own results")
		}
	}

	for _, want := range []string{"dec-X", "symA", "symB", "note-Y", "symC"} {
		if _, ok := idx[want]; !ok {
			t.Fatalf("expected %q in related results, got %v", want, got)
		}
	}
	if idx["dec-X"].Kind != RelatedArtifact {
		t.Errorf("dec-X should be RelatedArtifact")
	}
	if idx["symA"].Kind != RelatedSymbol {
		t.Errorf("symA should be RelatedSymbol")
	}
	// Proximity: distance-1 nodes outrank distance-2 nodes.
	if idx["dec-X"].Score <= idx["note-Y"].Score {
		t.Errorf("dec-X (1 hop) should outrank note-Y (2 hops): %v vs %v", idx["dec-X"].Score, idx["note-Y"].Score)
	}
	if idx["symA"].Score <= idx["symC"].Score {
		t.Errorf("symA (1 hop) should outrank symC (2 hops): %v vs %v", idx["symA"].Score, idx["symC"].Score)
	}
}

func TestRankRelatedDeterministicAndBounded(t *testing.T) {
	edges := []codebase.CodeEdge{{SrcID: "symA", DstID: "symB", Kind: codebase.EdgeCall}}
	affected := []artifact.AffectedFileRef{{ArtifactID: "dec-X", FilePath: "f1.go"}}
	syms := []codebase.SymbolRef{{ID: "symA", FilePath: "f1.go"}, {ID: "symB", FilePath: "f1.go"}}
	g, kind := buildFusedGraph(
		edges,
		nil,
		affected,
		nil,
		syms,
		moduleFusionInputs{},
	)
	seed := map[string]float64{fileNode("f1.go"): 1}

	first := rankRelated(g, kind, seed, 2)
	if len(first) != 2 {
		t.Fatalf("limit=2 must cap results, got %d", len(first))
	}
	for range 15 {
		again := rankRelated(g, kind, seed, 2)
		if len(again) != len(first) {
			t.Fatalf("non-deterministic length")
		}
		for i := range first {
			if again[i].ID != first[i].ID || again[i].Score != first[i].Score {
				t.Fatalf("rankRelated not deterministic: %v vs %v", first, again)
			}
		}
	}
}

func TestRankRelatedOffGraphSeedEmpty(t *testing.T) {
	g, kind := buildFusedGraph(
		nil,
		nil,
		[]artifact.AffectedFileRef{
			{ArtifactID: "dec-X", FilePath: "f1.go"},
		},
		nil,
		nil,
		moduleFusionInputs{},
	)
	got := rankRelated(g, kind, map[string]float64{fileNode("does/not/exist.go"): 1}, 0)
	if len(got) != 0 {
		t.Fatalf("off-graph seed must yield no related nodes, got %v", got)
	}
}

func TestBuildFusedGraphUsesOnlyCurrentDurableSymbolBindings(t *testing.T) {
	symbols := []codebase.SymbolRef{
		{
			ID:       "internal/x.go#Current#10",
			AnchorID: "sym:v2:current",
			FilePath: "internal/x.go",
		},
	}
	bindings := []artifact.SymbolBindingRef{
		{
			ArtifactID: "dec-current",
			AnchorID:   "sym:v2:current",
		},
		{
			ArtifactID: "dec-stale",
			AnchorID:   "sym:v2:stale",
		},
	}
	graph, kind := buildFusedGraph(
		nil,
		nil,
		nil,
		bindings,
		symbols,
		moduleFusionInputs{},
	)
	ranked := rankRelated(
		graph,
		kind,
		map[string]float64{"dec-current": 1},
		0,
	)
	if len(ranked) == 0 ||
		ranked[0].ID != "internal/x.go#Current#10" {
		t.Fatalf("current exact binding rank = %+v", ranked)
	}
	if graph.Has("dec-stale") {
		t.Fatal("stale anchor binding became a graph edge")
	}
}

func TestBuildFusedGraphUsesOnlyMostSpecificModuleContext(t *testing.T) {
	modules := []codebase.Module{
		{ID: "mod-root", Path: ""},
		{ID: "mod-cli", Path: "internal/cli"},
		{ID: "mod-nested", Path: "internal/cli/nested"},
	}
	symbols := []codebase.SymbolRef{
		{ID: "sym-cli", FilePath: "internal/cli/main.go"},
		{ID: "sym-nested", FilePath: "internal/cli/nested/main.go"},
	}
	moduleFusion, err := resolveModuleFusionInputs(
		modules,
		[]graph.DecisionModuleContext{{
			DecisionID: "dec-cli",
			ModuleID:   "mod-cli",
			ModulePath: "internal/cli",
			Source:     "explicit_module_binding",
		}},
		symbols,
	)
	if err != nil {
		t.Fatal(err)
	}
	fused, kinds := buildFusedGraph(
		nil,
		nil,
		nil,
		nil,
		symbols,
		moduleFusion,
	)
	ranked := rankRelated(
		fused,
		kinds,
		map[string]float64{"dec-cli": 1},
		0,
	)
	ids := make(map[string]bool)
	for _, item := range ranked {
		ids[item.ID] = true
	}
	if !ids["sym-cli"] {
		t.Fatalf("module decision did not reach owned symbol: %+v", ranked)
	}
	if ids["sym-nested"] {
		t.Fatalf("parent module stole nested symbol: %+v", ranked)
	}
}

func TestNodeFile(t *testing.T) {
	if got := nodeFile("internal/x/y.go#Foo#12"); got != "internal/x/y.go" {
		t.Errorf("nodeFile(symbol) = %q, want internal/x/y.go", got)
	}
	if got := nodeFile("dec-20260604-abc"); got != "" {
		t.Errorf("nodeFile(artifact) = %q, want empty", got)
	}
}

func TestNodeName(t *testing.T) {
	if got := nodeName("internal/x/y.go#Foo#12"); got != "Foo" {
		t.Errorf("nodeName(symbol) = %q, want Foo", got)
	}
	if got := nodeName("dec-20260604-abc"); got != "" {
		t.Errorf("nodeName(artifact) = %q, want empty", got)
	}
}

func TestCoverageFor(t *testing.T) {
	syms := []codebase.CodeSymbol{
		{ID: "internal/foo/bar.go#DoX#5", Name: "DoX", Kind: "func", Exported: true},
		{ID: "internal/foo/bar.go#Untested#20", Name: "Untested", Kind: "func", Exported: true},
		{ID: "internal/foo/bar.go#Bar#1", Name: "Bar", Kind: "type", Exported: true}, // not callable
	}
	callers := map[string][]codebase.CodeEdge{
		"internal/foo/bar.go#DoX#5": {
			{SrcID: "internal/foo/bar_test.go#TestDoX#10"}, // test caller — counted
			{SrcID: "internal/foo/other.go#UseX#3"},        // production caller — ignored
		},
	}
	cov := coverageFor(syms, callers)
	// The type 'Bar' is excluded (not callable); two callable funcs remain.
	if len(cov) != 2 {
		t.Fatalf("expected 2 callable symbols, got %d: %v", len(cov), cov)
	}
	byName := map[string]SymbolCoverage{}
	for _, c := range cov {
		byName[c.Symbol] = c
	}
	if got := byName["DoX"].TestedBy; len(got) != 1 || got[0] != "TestDoX" {
		t.Errorf("DoX TestedBy = %v, want [TestDoX] (production caller must be excluded)", got)
	}
	if len(byName["Untested"].TestedBy) != 0 {
		t.Errorf("Untested should have no test callers, got %v", byName["Untested"].TestedBy)
	}
}
