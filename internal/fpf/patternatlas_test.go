package fpf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatternAtlasBuildPreservesSourceRanges(t *testing.T) {
	markdown := patternAtlasFixtureMarkdown()
	atlas, err := BuildPatternAtlas([]byte(markdown), "fixture.md", "fixture-revision")
	if err != nil {
		t.Fatalf("BuildPatternAtlas() error: %v", err)
	}

	if len(atlas.Nodes) == 0 {
		t.Fatal("BuildPatternAtlas() produced no source nodes")
	}
	if len(atlas.Cards) != 5 {
		t.Fatalf("pattern cards = %d, want 5", len(atlas.Cards))
	}

	lines := splitPatternAtlasLines([]byte(markdown))
	tests := []struct {
		patternID string
		contains  string
	}{
		{patternID: "F.18", contains: "NameCard"},
		{patternID: "C.30", contains: "ArchitectureQuestionCard"},
		{patternID: "A.10", contains: "evidence-provenance graph relation"},
		{patternID: "A.7", contains: "EntityOfConcern and Description-episteme boundary"},
		{patternID: "B.3", contains: "Congruence Level (CL)"},
	}
	for _, test := range tests {
		card, found := patternAtlasCardByID(atlas.Cards, test.patternID)
		if !found {
			t.Fatalf("pattern card %s not found", test.patternID)
		}
		body := patternAtlasLineRange(lines, card.CardStartLine, card.CardEndLine)
		if !strings.Contains(body, test.contains) {
			t.Fatalf("pattern card %s range missing %q:\n%s", test.patternID, test.contains, body)
		}
		if card.ContentHash != patternAtlasHash(body) {
			t.Fatalf("pattern card %s content hash does not cover its source range", test.patternID)
		}
	}
}

func TestPatternAtlasLintDetectsMalformedMarkdownHeading(t *testing.T) {
	atlas, err := BuildPatternAtlas(
		[]byte(patternAtlasFixtureMarkdown()),
		"fixture.md",
		"fixture-revision",
	)
	if err != nil {
		t.Fatalf("BuildPatternAtlas() error: %v", err)
	}

	for _, lint := range atlas.Lints {
		if strings.Contains(lint.RawLine, "#### E.3:4.2") &&
			lint.LintKind == PatternAtlasLintLeadingSpace {
			return
		}
	}
	t.Fatalf("expected leading-space heading lint, got %#v", atlas.Lints)
}

func TestPatternAtlasProductionSourceHasKnownAddressableCards(t *testing.T) {
	path := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")
	markdown, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skipf("FPF submodule not initialized: %s", path)
	}
	if err != nil {
		t.Fatalf("read production FPF source: %v", err)
	}

	atlas, err := BuildPatternAtlas(markdown, path, "production-fixture")
	if err != nil {
		t.Fatalf("BuildPatternAtlas() error: %v", err)
	}
	if len(atlas.Nodes) == 0 {
		t.Fatal("production source produced no structural nodes")
	}
	if len(atlas.Cards) == 0 {
		t.Fatal("production source produced no addressable pattern cards")
	}

	lines := splitPatternAtlasLines(markdown)
	for _, patternID := range []string{"F.18", "C.30", "A.10", "A.7", "B.3"} {
		card, found := patternAtlasCardByID(atlas.Cards, patternID)
		if !found {
			t.Fatalf("production pattern card %s not found", patternID)
		}
		body := patternAtlasLineRange(lines, card.CardStartLine, card.CardEndLine)
		if strings.TrimSpace(body) == "" {
			t.Fatalf("production pattern card %s has an empty source range", patternID)
		}
	}
}

func patternAtlasCardByID(cards []PatternAtlasCard, patternID string) (PatternAtlasCard, bool) {
	for _, card := range cards {
		if card.PatternID == patternID {
			return card, true
		}
	}
	return PatternAtlasCard{}, false
}

func patternAtlasFixtureMarkdown() string {
	return `# Fixture

## F.18 - Local-First Unification Naming Protocol
Intro.

### F.18:1 - Context
NameCard:
  GovernedValueRef

## C.30 - Grounded Architecture and Selected-Structure Adequacy
Intro.

### C.30:1 - Problem frame
ArchitectureQuestionCard:
  SelectedStructures

## A.10 - Evidence Graph Referring: Claim-Bound Evidence and Provenance Graph
Intro.

### A.10:1 - Evidence relation
evidence-provenance graph relation:
  ClaimRef

 #### E.3:4.2 - **Precedence Stack**
This heading is malformed but should not disappear.

## A.7 - Strict Distinction (Clarity Lattice)
Intro.

### A.7:1 - Strict table
EntityOfConcern and Description-episteme boundary:
  Object
  Description
  Carrier
  Evidence

## B.3 - Evidence Congruence and Decay
Intro.

### B.3:1 - Congruence
Congruence Level (CL):
  CL3
`
}
