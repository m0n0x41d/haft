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

func codeContextDecisionWithSections(t *testing.T, id string, title string, sectionRefs []string) *artifact.Artifact {
	t.Helper()
	item := codeContextArtifact(id, artifact.KindDecisionRecord, title)
	data, err := json.Marshal(artifact.DecisionFields{SectionRefs: sectionRefs})
	if err != nil {
		t.Fatal(err)
	}
	item.StructuredData = string(data)
	return item
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
		t.Fatalf("compact response should explain file-level relevance:\n%s", compact)
	}
	if !strings.Contains(compact, "not proof that every invariant binds every symbol in the file") {
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

func TestCodeContextResponse_HighFanoutInvariantLaneSummarizesBySource(t *testing.T) {
	invariants := make([]graph.Invariant, 0, 55)
	for i := 1; i <= 55; i++ {
		sourceID := "dec-alpha"
		sourceTitle := "Alpha decision"
		if i > 30 {
			sourceID = "dec-beta"
			sourceTitle = "Beta decision"
		}
		invariants = append(invariants, graph.Invariant{
			Text:          fmt.Sprintf("invariant-%02d", i),
			DecisionID:    sourceID,
			DecisionTitle: sourceTitle,
		})
	}
	cc := contextgraph.CodeContext{
		Target:     contextgraph.Target{File: "internal/x.go", Symbol: "Run"},
		Invariants: invariants,
	}

	lane := CodeContextResponseWithOptions(cc, CodeContextRenderOptions{Lane: CodeContextLaneInvariants})
	for _, want := range []string{
		"High fanout: 55 invariant(s) from 2 source group(s)",
		"Default lane shows source groups",
		"**Alpha decision** `dec-alpha`: 30 invariant(s)",
		"**Beta decision** `dec-beta`: 25 invariant(s)",
		"full=true",
	} {
		if !strings.Contains(lane, want) {
			t.Fatalf("high-fanout lane missing %q:\n%s", want, lane)
		}
	}
	if strings.Contains(lane, "invariant-42") {
		t.Fatalf("high-fanout default lane should not inline every invariant sentence:\n%s", lane)
	}

	full := CodeContextResponseWithOptions(cc, CodeContextRenderOptions{Lane: CodeContextLaneInvariants, Full: true})
	if !strings.Contains(full, "invariant-55") {
		t.Fatalf("full invariant lane should restore every invariant sentence:\n%s", full)
	}
	if strings.Contains(full, "High fanout") {
		t.Fatalf("full invariant lane should render the audit list, not the summary:\n%s", full)
	}
}

func TestCodeContextResponse_DefaultIndexOmitsLaneDumps(t *testing.T) {
	cc := contextgraph.CodeContext{
		Target: contextgraph.Target{File: "internal/x.go"},
		Decisions: []*artifact.Artifact{
			codeContextDecisionWithSections(t, "dec-1", "Decision title", []string{"TS.environment.001"}),
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

func TestCodeContextResponse_DefaultIndexLabelsFileLevelInvariantCandidates(t *testing.T) {
	invariants := make([]graph.Invariant, 0, 10)
	for i := 1; i <= 10; i++ {
		invariants = append(invariants, graph.Invariant{
			Text:          fmt.Sprintf("invariant-%02d", i),
			DecisionTitle: "Context decision",
		})
	}
	cc := contextgraph.CodeContext{
		Target:     contextgraph.Target{File: "internal/x.go"},
		Invariants: invariants,
	}

	index := CodeContextResponse(cc)
	for _, want := range []string{
		"invariants: 10 file-level candidate(s)",
		"narrow by symbol",
		"file-level invariant candidates exist",
	} {
		if !strings.Contains(index, want) {
			t.Fatalf("index response missing %q:\n%s", want, index)
		}
	}
	if strings.Contains(index, "invariants: 10 binding") {
		t.Fatalf("index response should not label broad file-level fanout as binding:\n%s", index)
	}
}

func TestCodeContextResponse_FileLevelInvariantDoesNotReadAsBinding(t *testing.T) {
	cc := contextgraph.CodeContext{
		Target: contextgraph.Target{File: "internal/x.go"},
		Invariants: []graph.Invariant{
			{Text: "single file-level invariant", DecisionTitle: "Context decision"},
		},
	}

	index := CodeContextResponse(cc)
	for _, want := range []string{
		"invariants: 1 file-level candidate(s)",
		"file-level invariant candidates exist",
	} {
		if !strings.Contains(index, want) {
			t.Fatalf("index response missing %q:\n%s", want, index)
		}
	}
	if strings.Contains(index, "1 binding") {
		t.Fatalf("file-level index should not label invariant as binding:\n%s", index)
	}

	lane := CodeContextResponseWithOptions(cc, CodeContextRenderOptions{Lane: CodeContextLaneInvariants})
	for _, want := range []string{
		"### Invariant relevance",
		"### File-level invariant candidates",
		"single file-level invariant",
	} {
		if !strings.Contains(lane, want) {
			t.Fatalf("invariants lane missing %q:\n%s", want, lane)
		}
	}
	if strings.Contains(lane, "### Invariants that must hold here") {
		t.Fatalf("file-level invariants lane must not claim symbol-local authority:\n%s", lane)
	}
}

func TestCodeContextResponse_SymbolInvariantStillReadsAsBinding(t *testing.T) {
	cc := contextgraph.CodeContext{
		Target: contextgraph.Target{File: "internal/x.go", Symbol: "Run"},
		Invariants: []graph.Invariant{
			{Text: "symbol invariant", DecisionTitle: "Symbol decision"},
		},
	}

	index := CodeContextResponse(cc)
	for _, want := range []string{
		"invariants: 1 binding",
		"symbol-binding invariants exist",
	} {
		if !strings.Contains(index, want) {
			t.Fatalf("symbol index missing %q:\n%s", want, index)
		}
	}

	lane := CodeContextResponseWithOptions(cc, CodeContextRenderOptions{Lane: CodeContextLaneInvariants})
	for _, want := range []string{
		"### Invariants that must hold here",
		"symbol invariant",
	} {
		if !strings.Contains(lane, want) {
			t.Fatalf("symbol invariants lane missing %q:\n%s", want, lane)
		}
	}
}

func TestCodeContextResponse_TypedLanesStaySeparate(t *testing.T) {
	cc := contextgraph.CodeContext{
		Target: contextgraph.Target{File: "internal/x.go"},
		Decisions: []*artifact.Artifact{
			codeContextDecisionWithSections(t, "dec-1", "Decision title", []string{"TS.environment.001"}),
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
	if !strings.Contains(decisions, "SpecSections: `TS.environment.001`") {
		t.Fatalf("decisions lane missing decision section refs:\n%s", decisions)
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

func TestCodeContextResponse_RendersTypedSpecLifecycleWithRecovery(t *testing.T) {
	claims := []contextgraph.SpecClaimContext{
		{ID: "claim-1", Class: "requirement", Statement: "first claim"},
		{ID: "claim-2", Class: "requirement", Statement: "second claim"},
		{ID: "claim-3", Class: "requirement", Statement: "third claim"},
		{ID: "claim-4", Class: "requirement", Statement: "fourth claim"},
	}
	cc := contextgraph.CodeContext{
		Target: contextgraph.Target{File: "internal/x.go"},
		Decisions: []*artifact.Artifact{
			codeContextDecisionWithSections(t, "dec-1", "Decision title", []string{"TS.current.001", "TS.missing.001"}),
		},
		Notes: []*artifact.Artifact{
			codeContextArtifact("note-1", artifact.KindNote, "Note title"),
		},
		Specs: []contextgraph.SpecSectionContext{
			{
				ID:                 "TS.current.001",
				Title:              "Current target behavior",
				LifecycleState:     "active",
				ValidUntil:         "2026-12-31",
				Claims:             claims,
				DecisionRefs:       []string{"dec-1"},
				Resolution:         contextgraph.SpecResolutionResolved,
				SourceKind:         "carrier_import",
				CarrierPath:        ".haft/specs/target-system.md",
				BaselineState:      contextgraph.SpecBaselineCurrent,
				BaselineApprovedBy: "operator",
			},
			{
				ID:               "TS.missing.001",
				DecisionRefs:     []string{"dec-1"},
				Resolution:       contextgraph.SpecResolutionMissing,
				ResolutionDetail: "no current edition",
				BaselineState:    contextgraph.SpecBaselineMissing,
			},
		},
	}

	index := CodeContextResponse(cc)
	for _, want := range []string{
		"specs: 2 referenced; 1 resolved, 1 unresolved",
		"1 referenced SpecSection(s) are non-current or unresolved",
		`lane="decisions"`,
	} {
		if !strings.Contains(index, want) {
			t.Fatalf("spec index missing %q:\n%s", want, index)
		}
	}

	decisions := CodeContextResponseWithOptions(cc, CodeContextRenderOptions{Lane: CodeContextLaneDecisions})
	for _, want := range []string{
		"### Referenced SpecSections",
		"**Current target behavior** `TS.current.001`",
		"resolution=resolved",
		"baseline=current",
		"lifecycle=active",
		"**Unresolved SpecSection** `TS.missing.001`",
		"resolution=missing",
		"resolution_detail=no current edition",
		"1 more claim(s) omitted",
	} {
		if !strings.Contains(decisions, want) {
			t.Fatalf("spec decisions lane missing %q:\n%s", want, decisions)
		}
	}
	if strings.Contains(decisions, "fourth claim") {
		t.Fatalf("compact decisions lane should cap claims:\n%s", decisions)
	}

	full := CodeContextResponseFull(cc)
	if !strings.Contains(full, "fourth claim") || strings.Contains(full, "more claim(s) omitted") {
		t.Fatalf("full response must restore all SpecSection claims:\n%s", full)
	}

	notes := CodeContextResponseWithOptions(cc, CodeContextRenderOptions{Lane: CodeContextLaneNotes})
	if strings.Contains(notes, "Referenced SpecSections") || strings.Contains(notes, "Current target behavior") {
		t.Fatalf("typed specs leaked into notes lane:\n%s", notes)
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
