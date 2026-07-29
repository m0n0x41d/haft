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

func decArtifact(id, title string) *artifact.Artifact {
	a := &artifact.Artifact{}
	a.Meta.ID = id
	a.Meta.Title = title
	a.Meta.Kind = artifact.KindDecisionRecord
	a.Meta.Status = artifact.StatusActive
	return a
}

// The P2 gate, asserted on the presentation contract: a hop governed only at
// the module level must render "module governed by dec-Y", never blank — a
// blank row reads as "safe to change", the exact failure the fusion exists to
// prevent.
func TestFlowResponse_GovernanceNeverBlank(t *testing.T) {
	res := codeintel.FlowResult{
		Seed:           codebase.CodeSymbol{Name: "Edit", FilePath: "internal/x/edit.go", StartLine: 10},
		SeedResolution: fixtureResolvedSeed(t, "sym:test:edit"),
		Direction:      codeintel.Callers,
		Depth:          2,
		Hops: []codeintel.FusedHop{
			{ // symbol-level governance
				Symbol:   codebase.CodeSymbol{Name: "Apply", FilePath: "internal/x/apply.go", StartLine: 40},
				Distance: 1,
				ViaKind:  codebase.EdgeCall,
				Context:  contextgraph.CodeContext{Decisions: []*artifact.Artifact{decArtifact("dec-AAA", "apply contract")}},
			},
			{ // module-level fallback only
				Symbol:     codebase.CodeSymbol{Name: "Dispatch", FilePath: "internal/y/run.go", StartLine: 70},
				Distance:   2,
				ViaKind:    codebase.EdgeInterfaceDispatch,
				Provenance: codebase.ProvenanceHeuristic,
				Context:    contextgraph.CodeContext{Module: "internal/y", Governed: true, ModuleDecisions: []graph.Node{{ID: "dec-BBB", Name: "y boundary"}}},
			},
			{ // genuinely ungoverned
				Symbol:   codebase.CodeSymbol{Name: "helper", FilePath: "internal/z/util.go", StartLine: 5},
				Distance: 2,
				ViaKind:  codebase.EdgeCall,
				Context:  contextgraph.CodeContext{},
			},
		},
	}

	out := FlowResponse(res, "impact", "Edit")

	if !strings.Contains(out, "dec-AAA") || !strings.Contains(out, "governs:") {
		t.Errorf("symbol-level decision not surfaced:\n%s", out)
	}
	if !strings.Contains(out, "module governed by") || !strings.Contains(out, "dec-BBB") {
		t.Errorf("module-governed hop rendered blank — the exact P2 failure mode:\n%s", out)
	}
	if !strings.Contains(out, "no recorded reasoning") {
		t.Errorf("ungoverned hop should be explicit, not silent:\n%s", out)
	}
	if !strings.Contains(out, "heuristic") {
		t.Errorf("heuristic dispatch edge must be flagged:\n%s", out)
	}
	if !strings.Contains(out, "2 carry recorded reasoning") {
		t.Errorf("governed count wrong (want 2 of 3):\n%s", out)
	}
}

// An ambiguous seed must list candidates for disambiguation, never silently
// traverse one (the keystone discipline at the query surface).
func TestFlowResponse_AmbiguousSeedSurfacesCandidates(t *testing.T) {
	res := codeintel.FlowResult{
		Direction: codeintel.Callers,
		SeedResolution: fixtureCandidateSet(
			t,
			"ambiguous_exact_name",
			"sym:test:search:artifact",
			"sym:test:search:db",
		),
		Candidates: []codebase.CodeSymbol{
			{Name: "Search", FilePath: "internal/artifact/store.go", StartLine: 100, Receiver: "Store"},
			{Name: "Search", FilePath: "internal/db/store.go", StartLine: 55, Receiver: "Store"},
		},
	}
	out := FlowResponse(res, "callers", "Search")
	if !strings.Contains(out, "ambiguous") {
		t.Errorf("ambiguous seed not announced:\n%s", out)
	}
	if !strings.Contains(out, "internal/artifact/store.go:100") || !strings.Contains(out, "internal/db/store.go:55") {
		t.Errorf("both candidates must be listed with file:line:\n%s", out)
	}
}

func TestFlowResponse_SeedNotFound(t *testing.T) {
	out := FlowResponse(codeintel.FlowResult{Direction: codeintel.Callees}, "callees", "Nope")
	if !strings.Contains(out, "not found") {
		t.Errorf("missing seed should say not found:\n%s", out)
	}
}

func TestFlowResponse_IncompleteIndexNeverClaimsAbsence(t *testing.T) {
	out := FlowResponse(codeintel.FlowResult{
		Direction:      codeintel.Callees,
		SeedResolution: fixtureSeedUnavailable(t, "index_incomplete"),
	}, "callees", "Nope")
	if !strings.Contains(out, "unavailable") ||
		!strings.Contains(out, "not evidence that the symbol is absent") {
		t.Errorf("incomplete index must not render false absence:\n%s", out)
	}
	if strings.Contains(out, "`Nope` not found") {
		t.Errorf("incomplete index must not say not found:\n%s", out)
	}
}

func TestFlowResponseCarriesExactCoverageBasis(t *testing.T) {
	res := codeintel.FlowResult{
		SeedResolution: fixtureResolvedSeed(t, "sym:v2:a"),
		Direction:      codeintel.Callees,
		Index: codebase.IndexState{
			Epoch: 7,
			Basis: codebase.IndexBasisSnapshot{
				Epoch:        7,
				CorpusDigest: "corpus-digest",
				BasisDigest:  "basis-digest",
				Coverage: codebase.IndexCoverageSnapshot{
					Posture:         codebase.IndexCoverageComplete,
					DiscoveredFiles: 3,
					AdmittedFiles:   3,
				},
			},
		},
	}
	out := FlowResponse(res, "callees", "A")
	for _, want := range []string{
		"coverage: complete (3 discovered, 3 admitted, 0 skipped)",
		"Index basis: `sha256:basis-digest`",
		"corpus: `sha256:corpus-digest`",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("basis output missing %q:\n%s", want, out)
		}
	}
}

func TestNodeResponseDoesNotRenderNotFoundForIncompleteIndex(t *testing.T) {
	scope, err := codeintel.NewTraversalScopeWithCoverage(
		4,
		"code-index:v5:basis:sha256:partial",
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	observation, err := codeintel.NewIndexObservation(
		scope.Epoch(),
		scope.CoverageRef(),
	)
	if err != nil {
		t.Fatal(err)
	}
	reason, err := codeintel.ParseSeedUnavailableReason(
		"index_incomplete",
	)
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := codeintel.NewSeedUnavailable(
		"Missing",
		reason,
		observation,
	)
	if err != nil {
		t.Fatal(err)
	}
	body := NodeResponse(codeintel.NodeView{
		Name:           "Missing",
		NameResolution: resolution,
		Index: codebase.IndexState{
			Epoch: 4,
			Basis: codebase.IndexBasisSnapshot{
				Epoch:        4,
				CorpusDigest: "corpus",
				BasisDigest:  "partial",
				Coverage: codebase.IndexCoverageSnapshot{
					Posture:         codebase.IndexCoverageBoundedWithExclusions,
					DiscoveredFiles: 2,
					AdmittedFiles:   1,
					SkippedFiles:    1,
				},
			},
		},
	}, "go")
	if strings.Contains(body, "not found") ||
		!strings.Contains(body, "unavailable") ||
		!strings.Contains(body, "not evidence") {
		t.Fatalf("incomplete node response = %q", body)
	}
}
