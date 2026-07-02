package fpf

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestPatternAtlasBuild_PreservesPatternRootAndSubtreeRanges(t *testing.T) {
	atlas := mustBuildPatternAtlasFixture(t)

	if len(atlas.Nodes) == 0 {
		t.Fatal("expected atlas nodes")
	}
	if len(atlas.Cards) < 5 {
		t.Fatalf("expected at least five pattern cards, got %d", len(atlas.Cards))
	}

	db := mustStorePatternAtlasFixture(t, atlas)

	tests := []struct {
		patternID string
		contains  string
	}{
		{patternID: "F.18", contains: "NameCard"},
		{patternID: "C.30", contains: "ArchitectureQuestionCard"},
		{patternID: "A.10", contains: "EvidenceRelation"},
		{patternID: "A.7", contains: "ObjectDescriptionCarrierEvidence"},
		{patternID: "B.3", contains: "CongruenceLevel"},
	}

	for _, tt := range tests {
		t.Run(tt.patternID, func(t *testing.T) {
			card, err := GetPatternCard(db, tt.patternID)
			if err != nil {
				t.Fatalf("GetPatternCard(%q): %v", tt.patternID, err)
			}
			if !strings.Contains(card.Body, tt.contains) {
				t.Fatalf("GetPatternCard(%q) body missing %q:\n%s", tt.patternID, tt.contains, card.Body)
			}
			if card.NodeCount <= 1 {
				t.Fatalf("expected full card range with child nodes, got node count %d", card.NodeCount)
			}
			if card.BodyKind != PatternAtlasBodyKindFullCardRange {
				t.Fatalf("body kind = %q, want %q", card.BodyKind, PatternAtlasBodyKindFullCardRange)
			}
		})
	}
}

func TestPatternAtlasLint_DetectsMalformedMarkdownHeading(t *testing.T) {
	atlas := mustBuildPatternAtlasFixture(t)

	var found bool
	for _, lint := range atlas.Lints {
		if strings.Contains(lint.RawLine, "#### E.3:4.2") && lint.LintKind == PatternAtlasLintLeadingSpace {
			found = true
			if lint.LineNumber <= 0 {
				t.Fatalf("unexpected lint raw line: %q", lint.RawLine)
			}
		}
	}
	if !found {
		t.Fatalf("expected leading-space heading lint, got %#v", atlas.Lints)
	}

	var foundNode bool
	for _, node := range atlas.Nodes {
		if node.Heading == "E.3:4.2 - **Precedence Stack**" {
			foundNode = true
		}
	}
	if !foundNode {
		t.Fatal("malformed heading was not normalized into an atlas node")
	}
}

func TestPatternAtlasHashIntegrityErrorsDetectStaleCardHash(t *testing.T) {
	atlas := mustBuildPatternAtlasFixture(t)
	db := mustStorePatternAtlasFixture(t, atlas)

	if errs, err := PatternAtlasHashIntegrityErrors(db); err != nil || len(errs) != 0 {
		t.Fatalf("expected clean atlas hashes, errs=%v err=%v", errs, err)
	}

	if _, err := db.Exec(`UPDATE pattern_atlas_cards SET content_hash='stale' WHERE pattern_id='F.18'`); err != nil {
		t.Fatalf("stale card hash: %v", err)
	}

	errs, err := PatternAtlasHashIntegrityErrors(db)
	if err != nil {
		t.Fatalf("PatternAtlasHashIntegrityErrors: %v", err)
	}
	if len(errs) == 0 || !strings.Contains(errs[0], "F.18") {
		t.Fatalf("expected F.18 hash mismatch, got %#v", errs)
	}
}

func TestPatternAtlasProductionSpec_KnownCardsAndLint(t *testing.T) {
	path := filepath.Join(testRepoRoot(t), "data", "FPF", "FPF-Spec.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("FPF submodule not initialized — run 'git submodule update --init' (%s)", path)
	}

	atlas, err := LoadPatternAtlas(path, "production-fixture")
	if err != nil {
		t.Fatalf("LoadPatternAtlas: %v", err)
	}
	if len(atlas.Nodes) < 7000 {
		t.Fatalf("expected production atlas to have at least 7000 nodes, got %d", len(atlas.Nodes))
	}
	if len(atlas.Cards) < 250 {
		t.Fatalf("expected production atlas to have at least 250 cards, got %d", len(atlas.Cards))
	}

	db := mustStorePatternAtlasFixture(t, atlas)
	tests := []struct {
		patternID string
		contains  string
	}{
		{patternID: "F.18", contains: "NameCard"},
		{patternID: "C.30", contains: "Architecture"},
		{patternID: "A.10", contains: "Evidence"},
		{patternID: "A.7", contains: "Strict Distinction"},
		{patternID: "B.3", contains: "Congruence"},
	}
	for _, tt := range tests {
		card, err := GetPatternCard(db, tt.patternID)
		if err != nil {
			t.Fatalf("GetPatternCard(%q): %v", tt.patternID, err)
		}
		if !strings.Contains(card.Body, tt.contains) {
			t.Fatalf("GetPatternCard(%q) body missing %q", tt.patternID, tt.contains)
		}
	}

	var foundMalformedHeading bool
	for _, lint := range atlas.Lints {
		if strings.Contains(lint.RawLine, "#### E.3:4.2") {
			foundMalformedHeading = true
		}
	}
	if !foundMalformedHeading {
		t.Fatal("expected production atlas to lint the known leading-space E.3:4.2 heading")
	}
}

func mustBuildPatternAtlasFixture(t *testing.T) PatternAtlas {
	t.Helper()

	atlas, err := BuildPatternAtlas([]byte(patternAtlasFixtureMarkdown()), "fixture.md", "fixture-commit")
	if err != nil {
		t.Fatalf("BuildPatternAtlas: %v", err)
	}
	return atlas
}

func mustStorePatternAtlasFixture(t *testing.T, atlas PatternAtlas) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := BuildSpecIndex(dbPath, nil, nil); err != nil {
		t.Fatalf("BuildSpecIndex: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := StorePatternAtlasDB(db, atlas); err != nil {
		t.Fatalf("StorePatternAtlasDB: %v", err)
	}
	return db
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
EvidenceRelation:
  ClaimRef

 #### E.3:4.2 - **Precedence Stack**
This heading is malformed but should not disappear.

## A.7 - Strict Distinction (Clarity Lattice)
Intro.

### A.7:1 - Strict table
ObjectDescriptionCarrierEvidence:
  Object
  Description
  Carrier
  Evidence

## B.3 - Evidence Congruence and Decay
Intro.

### B.3:1 - Congruence
CongruenceLevel:
  CL3
`
}
