package present

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/contextgraph"
	"github.com/m0n0x41d/haft/internal/graph"
)

func mkDecision(t *testing.T, claims []artifact.DecisionClaim) *artifact.Artifact {
	t.Helper()
	sd, err := json.Marshal(artifact.DecisionFields{Claims: claims})
	if err != nil {
		t.Fatal(err)
	}
	return &artifact.Artifact{StructuredData: string(sd)}
}

func codeContextArtifact(id string, kind artifact.Kind, title string) *artifact.Artifact {
	return &artifact.Artifact{
		Meta: artifact.Meta{
			ID:     id,
			Kind:   kind,
			Status: artifact.StatusActive,
			Title:  title,
		},
	}
}

// TestDecisionVerificationTag is the trust-decay signal: a governing decision
// surfaces how many of its predictions remain unverified, so unchecked rationale
// does not read as authoritative.
func TestDecisionVerificationTag(t *testing.T) {
	mixed := mkDecision(t, []artifact.DecisionClaim{
		{Claim: "a", Observable: "o", Threshold: "t", Status: artifact.ClaimStatusSupported},
		{Claim: "b", Observable: "o", Threshold: "t", Status: artifact.ClaimStatusUnverified},
		{Claim: "c", Observable: "o", Threshold: "t", Status: artifact.ClaimStatusUnverified},
	})
	if got := decisionVerificationTag(mixed); got != " · 2/3 predictions unverified" {
		t.Errorf("mixed = %q, want ' · 2/3 predictions unverified'", got)
	}

	allVerified := mkDecision(t, []artifact.DecisionClaim{
		{Claim: "a", Observable: "o", Threshold: "t", Status: artifact.ClaimStatusSupported},
	})
	if got := decisionVerificationTag(allVerified); got != "" {
		t.Errorf("all-verified = %q, want empty", got)
	}

	if got := decisionVerificationTag(&artifact.Artifact{}); got != "" {
		t.Errorf("no-claims = %q, want empty", got)
	}
}

func TestCodeContextResponse_CompactsInvariantsAndFullRestoresThem(t *testing.T) {
	invariants := make([]graph.Invariant, 0, 20)
	for i := 1; i <= 20; i++ {
		invariants = append(invariants, graph.Invariant{
			Text:          fmt.Sprintf("invariant-%02d", i),
			DecisionTitle: "Context decision",
		})
	}
	cc := contextgraph.CodeContext{
		Target:     contextgraph.Target{File: "internal/x.go"},
		Invariants: invariants,
	}

	compact := CodeContextResponseAll(cc)
	if !strings.Contains(compact, "Invariant relevance") {
		t.Fatalf("compact response should explain broad file-level relevance:\n%s", compact)
	}
	if !strings.Contains(compact, "not proof that every invariant binds every symbol") {
		t.Fatalf("compact response should not present broad file invariants as direct symbol constraints:\n%s", compact)
	}
	if !strings.Contains(compact, `lane="symbols"`) || !strings.Contains(compact, "full=true") {
		t.Fatalf("compact response should name symbol narrowing and full audit paths:\n%s", compact)
	}
	if !strings.Contains(compact, "invariant-08") {
		t.Fatalf("compact response should include the capped visible prefix:\n%s", compact)
	}
	if strings.Contains(compact, "invariant-09") {
		t.Fatalf("compact response should omit invariant 09+ for broad file-level fanout:\n%s", compact)
	}
	if !strings.Contains(compact, "12 more omitted") {
		t.Fatalf("compact response should name omitted invariant count:\n%s", compact)
	}

	full := CodeContextResponseFull(cc)
	if !strings.Contains(full, "invariant-20") {
		t.Fatalf("full response should restore all invariants:\n%s", full)
	}
	if strings.Contains(full, "more omitted") {
		t.Fatalf("full response must not include compact omission marker:\n%s", full)
	}
}

func TestCodeContextResponse_DefaultIndexOmitsLaneDumps(t *testing.T) {
	cc := contextgraph.CodeContext{
		Target: contextgraph.Target{File: "internal/x.go"},
		Decisions: []*artifact.Artifact{
			codeContextArtifact("dec-1", artifact.KindDecisionRecord, "Decision title"),
		},
		Notes: []*artifact.Artifact{
			codeContextArtifact("note-1", artifact.KindNote, "Note title"),
		},
		Invariants: []graph.Invariant{
			{Text: "keep the widget stable", DecisionTitle: "Decision title"},
		},
		Module:   "internal",
		Governed: true,
		ModuleDecisions: []graph.Node{
			{ID: "dec-module", Name: "Module decision"},
		},
	}

	index := CodeContextResponse(cc)
	for _, want := range []string{"## Code context index", "Lane counts", "decisions: 1", `lane="decisions"`, "audit dump"} {
		if !strings.Contains(index, want) {
			t.Fatalf("index response missing %q:\n%s", want, index)
		}
	}
	for _, notWant := range []string{"### Decisions governing this code", "Note title", "keep the widget stable", "`dec-module`"} {
		if strings.Contains(index, notWant) {
			t.Fatalf("index response should omit %q:\n%s", notWant, index)
		}
	}
}

func TestCodeContextResponse_TypedLanesStaySeparate(t *testing.T) {
	cc := contextgraph.CodeContext{
		Target: contextgraph.Target{File: "internal/x.go"},
		Decisions: []*artifact.Artifact{
			codeContextArtifact("dec-1", artifact.KindDecisionRecord, "Decision title"),
		},
		Problems: []*artifact.Artifact{
			codeContextArtifact("prob-1", artifact.KindProblemCard, "Problem title"),
		},
		Portfolios: []*artifact.Artifact{
			codeContextArtifact("sol-1", artifact.KindSolutionPortfolio, "Portfolio title"),
		},
		Notes: []*artifact.Artifact{
			codeContextArtifact("note-1", artifact.KindNote, "Note title"),
		},
		Invariants: []graph.Invariant{
			{Text: "binding invariant", DecisionTitle: "Invariant source"},
		},
	}

	decisions := CodeContextResponseWithOptions(cc, CodeContextRenderOptions{Lane: CodeContextLaneDecisions})
	if !strings.Contains(decisions, "Decision title") {
		t.Fatalf("decisions lane missing decision:\n%s", decisions)
	}
	for _, notWant := range []string{"Note title", "Problem title", "Portfolio title", "binding invariant"} {
		if strings.Contains(decisions, notWant) {
			t.Fatalf("decisions lane leaked %q:\n%s", notWant, decisions)
		}
	}

	invariants := CodeContextResponseWithOptions(cc, CodeContextRenderOptions{Lane: CodeContextLaneInvariants})
	if !strings.Contains(invariants, "binding invariant") {
		t.Fatalf("invariants lane missing invariant:\n%s", invariants)
	}
	for _, notWant := range []string{"Note title", "Problem title", "Portfolio title"} {
		if strings.Contains(invariants, notWant) {
			t.Fatalf("invariants lane leaked %q:\n%s", notWant, invariants)
		}
	}

	notes := CodeContextResponseWithOptions(cc, CodeContextRenderOptions{Lane: CodeContextLaneNotes})
	if !strings.Contains(notes, "Note title") {
		t.Fatalf("notes lane missing note:\n%s", notes)
	}
	for _, notWant := range []string{"Decision title", "Problem title", "Portfolio title", "binding invariant"} {
		if strings.Contains(notes, notWant) {
			t.Fatalf("notes lane leaked %q:\n%s", notWant, notes)
		}
	}
}

func TestCodeContextSymbolsResponse_CapsByLimit(t *testing.T) {
	target := contextgraph.Target{File: "internal/x.go"}
	symbols := []CodeContextSymbolItem{
		{Name: "A", Kind: "func", StartLine: 1, EndLine: 3},
		{Name: "B", Kind: "func", StartLine: 5, EndLine: 7},
		{Name: "C", Kind: "func", StartLine: 9, EndLine: 11},
	}

	response := CodeContextSymbolsResponse(target, symbols, 2, true)
	for _, want := range []string{"Symbol index refreshed", "func `A` lines 1-3", "func `B` lines 5-7", "1 more omitted"} {
		if !strings.Contains(response, want) {
			t.Fatalf("symbols response missing %q:\n%s", want, response)
		}
	}
	if strings.Contains(response, "func `C`") {
		t.Fatalf("symbols response should cap by limit:\n%s", response)
	}
}
