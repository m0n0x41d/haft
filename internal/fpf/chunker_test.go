package fpf

import (
	"reflect"
	"strings"
	"testing"
)

func TestChunkMarkdown_BasicSections(t *testing.T) {
	input := `# Title

Some intro text.

## Section One

Content of section one.
More content here.

## Section Two

Content of section two.

### Subsection 2.1

Nested content.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(chunks))
	}

	tests := []struct {
		heading      string
		level        int
		bodyContains string
	}{
		{"Title", 1, "Some intro text."},
		{"Section One", 2, "Content of section one."},
		{"Section Two", 2, "Content of section two."},
		{"Subsection 2.1", 3, "Nested content."},
	}

	for i, tt := range tests {
		if chunks[i].Heading != tt.heading {
			t.Errorf("chunk[%d].Heading = %q, want %q", i, chunks[i].Heading, tt.heading)
		}
		if chunks[i].Level != tt.level {
			t.Errorf("chunk[%d].Level = %d, want %d", i, chunks[i].Level, tt.level)
		}
		if !strings.Contains(chunks[i].Body, tt.bodyContains) {
			t.Errorf("chunk[%d].Body should contain %q, got %q", i, tt.bodyContains, chunks[i].Body)
		}
	}
}

func TestChunkMarkdown_ExtractsPatternAndParentIDs(t *testing.T) {
	input := `## A.6 - Signature Stack & Boundary Discipline

Top body.

### A.6:4 - Solution

Body.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].PatternID != "A.6" {
		t.Fatalf("expected A.6, got %q", chunks[0].PatternID)
	}
	if chunks[1].PatternID != "A.6:4" {
		t.Fatalf("expected A.6:4, got %q", chunks[1].PatternID)
	}
	if chunks[1].ParentPatternID != "A.6" {
		t.Fatalf("expected parent A.6, got %q", chunks[1].ParentPatternID)
	}
}

func TestParseSpecCatalog_ExtractsMetadata(t *testing.T) {
	input := `| A.6 | **Signature Stack & Boundary Discipline** | Stable | *Keywords:* boundary, routing. *Queries:* "What is A.6?", "How do I route boundary statements?" | **Builds on:** E.8, A.6.B. |
| A.16 | **Language-State Transduction Coordination** | Stable | *Keywords:* language-state, route. *Queries:* "How do cues get routed?" | **Coordinates with:** B.4.1 |
`
	catalog, err := ParseSpecCatalog(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := catalog["A.6"]
	if !ok {
		t.Fatal("expected A.6 entry")
	}
	if len(entry.Keywords) == 0 || entry.Keywords[0] != "boundary" {
		t.Fatalf("unexpected keywords: %#v", entry.Keywords)
	}
	if len(entry.Queries) != 2 {
		t.Fatalf("unexpected queries: %#v", entry.Queries)
	}
	if len(entry.Edges) != 2 {
		t.Fatalf("unexpected typed edges: %#v", entry.Edges)
	}
	if entry.Edges[0].EdgeType != SpecEdgeTypeBuildsOn {
		t.Fatalf("unexpected edge type: %#v", entry.Edges)
	}
}

func TestParseSpecCatalog_ExtractsQueriesFromSmartQuotes(t *testing.T) {
	input := `| E.17.0 | **U.MultiViewDescribing — Viewpoints, Views & Correspondences** | New | Keywords: multi-view describing, viewpoint, view. Queries: “How to organise multiple descriptions of one object-of-talk?”, “How are viewpoints, views and correspondences structured in FPF?”, “How do viewpoint libraries generalise ISO 42010 for non-architectural descriptions?” | Builds on: C.2.1. |
`

	catalog, err := ParseSpecCatalog(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	entry := catalog["E.17.0"]
	want := []string{
		"How to organise multiple descriptions of one object-of-talk?",
		"How are viewpoints, views and correspondences structured in FPF?",
		"How do viewpoint libraries generalise ISO 42010 for non-architectural descriptions?",
	}
	if !reflect.DeepEqual(entry.Queries, want) {
		t.Fatalf("queries = %#v, want %#v", entry.Queries, want)
	}
}

func TestParseSpecCatalog_NormalizesMixedQueryForms(t *testing.T) {
	input := `| A.6.8 | **Service Polysemy Unpacking (RPR-SERV)** | Stable | *Keywords:* service polysemy, facet unpacking. *Queries:* "How to unpack service talk in FPF?";  serviceSituation lens , "promise content vs service access point", “RPR-SERV rules”. | **Builds on:** A.6.P. |
`

	catalog, err := ParseSpecCatalog(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	entry := catalog["A.6.8"]
	want := []string{
		"How to unpack service talk in FPF?",
		"serviceSituation lens",
		"promise content vs service access point",
		"RPR-SERV rules",
	}
	if !reflect.DeepEqual(entry.Queries, want) {
		t.Fatalf("queries = %#v, want %#v", entry.Queries, want)
	}
}

func TestEnrichChunks_OverlaysCatalogMetadata(t *testing.T) {
	chunks := []SpecChunk{{ID: 0, Heading: "A.6 - Signature Stack & Boundary Discipline", Level: 2, Body: "Body", PatternID: "A.6"}}
	catalog := map[string]SpecCatalogEntry{
		"A.6": {
			PatternID:  "A.6",
			Title:      "Signature Stack & Boundary Discipline",
			Keywords:   []string{"boundary", "routing"},
			Queries:    []string{"How do I route boundary statements?"},
			RelatedIDs: []string{"A.6.B"},
			Edges: []SpecEdge{{
				FromPatternID: "A.6",
				ToPatternID:   "E.8",
				EdgeType:      SpecEdgeTypeBuildsOn,
			}},
		},
	}

	enriched := EnrichChunks(chunks, catalog)
	if len(enriched[0].Keywords) != 2 {
		t.Fatalf("expected keywords, got %#v", enriched[0].Keywords)
	}
	if len(enriched[0].Queries) != 1 {
		t.Fatalf("expected queries, got %#v", enriched[0].Queries)
	}
	if len(enriched[0].RelatedIDs) != 1 || enriched[0].RelatedIDs[0] != "A.6.B" {
		t.Fatalf("unexpected related ids: %#v", enriched[0].RelatedIDs)
	}
	if len(enriched[0].Edges) != 1 || enriched[0].Edges[0].EdgeType != SpecEdgeTypeBuildsOn {
		t.Fatalf("unexpected edges: %#v", enriched[0].Edges)
	}
}

func TestBuildSectionSummary_PrefersQuotedPurposeLine(t *testing.T) {
	heading := "A.6 - Signature Stack & Boundary Discipline"
	body := `> **Type:** Architectural (A)
> **Status:** Stable
> **Purpose (one line):** Keep boundary claims evolvable by routing each statement to the right layer.

Canonical companion. The square itself is specified elsewhere.`

	got := buildSectionSummary(heading, body)
	want := "Keep boundary claims evolvable by routing each statement to the right layer."
	if got != want {
		t.Fatalf("buildSectionSummary() = %q, want %q", got, want)
	}
}

func TestBuildSectionSummary_PrefersLeadingSentence(t *testing.T) {
	heading := "A.2.8:4.1 - Normative definition"
	body := "A `U.Commitment` is a governance object with explicit scope. Additional elaboration follows in later sentences."

	got := buildSectionSummary(heading, body)
	want := "A U.Commitment is a governance object with explicit scope."
	if got != want {
		t.Fatalf("buildSectionSummary() = %q, want %q", got, want)
	}
}

func TestBuildSectionSummary_StripsOrderedListMarkers(t *testing.T) {
	heading := "A.5:4 - Solution"
	body := `1. **Kernel minimality (C-5).** Domain knowledge stays outside the kernel by default.
2. **Boundary packaging.** Reusable bundles are published as signatures.`

	got := buildSectionSummary(heading, body)
	want := "Kernel minimality (C-5)."
	if got != want {
		t.Fatalf("buildSectionSummary() = %q, want %q", got, want)
	}
}

func TestParseSpecCatalog_ExtractsTypedDependencyEdges(t *testing.T) {
	input := `| A.1 | **Builds On** | Stable | *Keywords:* build. | **Builds on:** B.1. |
| A.2 | **Prerequisite** | Stable | *Keywords:* pre. | **Is a prerequisite for:** B.2. |
| A.3 | **Coordinates** | Stable | *Keywords:* coord. | **Coordinates with:** B.3. |
| A.4 | **Constrains** | Stable | *Keywords:* constrain. | **Constrains:** B.4. |
| A.5 | **Informs** | Stable | *Keywords:* inform. | **Informs:** B.5. |
| A.6 | **Used** | Stable | *Keywords:* used. | **Used by:** B.6. |
| A.7 | **Refines** | Stable | *Keywords:* refine. | **Refines:** B.7. |
| A.8 | **Specialised** | Stable | *Keywords:* special. | **Specialised by:** B.8. |
`
	catalog, err := ParseSpecCatalog(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		patternID string
		want      SpecEdge
	}{
		{
			patternID: "A.1",
			want: SpecEdge{
				FromPatternID: "A.1",
				ToPatternID:   "B.1",
				EdgeType:      SpecEdgeTypeBuildsOn,
			},
		},
		{
			patternID: "A.2",
			want: SpecEdge{
				FromPatternID: "A.2",
				ToPatternID:   "B.2",
				EdgeType:      SpecEdgeTypePrerequisiteFor,
			},
		},
		{
			patternID: "A.3",
			want: SpecEdge{
				FromPatternID: "A.3",
				ToPatternID:   "B.3",
				EdgeType:      SpecEdgeTypeCoordinatesWith,
			},
		},
		{
			patternID: "A.4",
			want: SpecEdge{
				FromPatternID: "A.4",
				ToPatternID:   "B.4",
				EdgeType:      SpecEdgeTypeConstrains,
			},
		},
		{
			patternID: "A.5",
			want: SpecEdge{
				FromPatternID: "A.5",
				ToPatternID:   "B.5",
				EdgeType:      SpecEdgeTypeInforms,
			},
		},
		{
			patternID: "A.6",
			want: SpecEdge{
				FromPatternID: "A.6",
				ToPatternID:   "B.6",
				EdgeType:      SpecEdgeTypeUsedBy,
			},
		},
		{
			patternID: "A.7",
			want: SpecEdge{
				FromPatternID: "A.7",
				ToPatternID:   "B.7",
				EdgeType:      SpecEdgeTypeRefines,
			},
		},
		{
			patternID: "A.8",
			want: SpecEdge{
				FromPatternID: "A.8",
				ToPatternID:   "B.8",
				EdgeType:      SpecEdgeTypeSpecialisedBy,
			},
		},
	}

	for _, tt := range tests {
		entry := catalog[tt.patternID]
		if len(entry.Edges) != 1 {
			t.Fatalf("%s: unexpected edges: %#v", tt.patternID, entry.Edges)
		}
		if entry.Edges[0] != tt.want {
			t.Fatalf("%s: edge = %#v, want %#v", tt.patternID, entry.Edges[0], tt.want)
		}
	}
}

func TestParseSpecCatalog_FallsBackToRelatedIDsForUntypedDependencies(t *testing.T) {
	input := `| A.9 | **Fallback** | Stable | *Keywords:* fallback. | **Links to:** B.9, C.9. |
`
	catalog, err := ParseSpecCatalog(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	entry := catalog["A.9"]
	if len(entry.Edges) != 0 {
		t.Fatalf("unexpected typed edges: %#v", entry.Edges)
	}
	if len(entry.RelatedIDs) != 2 {
		t.Fatalf("unexpected related ids: %#v", entry.RelatedIDs)
	}
	if entry.RelatedIDs[0] != "B.9" || entry.RelatedIDs[1] != "C.9" {
		t.Fatalf("unexpected related ids: %#v", entry.RelatedIDs)
	}
}

func TestBuildAliases_ExtractsHeadingVariants(t *testing.T) {
	tests := []struct {
		name      string
		heading   string
		patternID string
		want      []string
	}{
		{
			name:      "boundary title without parenthetical noise",
			heading:   "A.6.B — Boundary Norm Square (Laws / Admissibility / Deontics / Work‑Effects)",
			patternID: "A.6.B",
			want: []string{
				"A.6.B",
				"Boundary Norm Square",
			},
		},
		{
			name:      "catalog title splits technical and prose names",
			heading:   "A.CHR‑NORM — Canonical “Characteristic” & rename (Dimension/Axis → Characteristic)",
			patternID: "A.17",
			want: []string{
				"A.CHR-NORM",
				"Canonical Characteristic & rename",
			},
		},
		{
			name:      "technical alias in trailing parenthetical",
			heading:   "A.17 - Canonical “Characteristic” (A.CHR‑NORM)",
			patternID: "A.17",
			want: []string{
				"Canonical Characteristic",
				"A.CHR-NORM",
			},
		},
		{
			name:      "technical and prose split by dash",
			heading:   "A.6.3.CR - ConservativeRetextualization — same-described-entity textual re-expression",
			patternID: "A.6.3.CR",
			want: []string{
				"ConservativeRetextualization",
				"same-described-entity textual re-expression",
			},
		},
		{
			name:      "parenthetical technical alias",
			heading:   "A.6.8 - Service Polysemy Unpacking (RPR‑SERV)",
			patternID: "A.6.8",
			want: []string{
				"Service Polysemy Unpacking",
				"RPR-SERV",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aliases := buildAliases(tt.heading, tt.patternID)
			for _, alias := range tt.want {
				if !containsAlias(aliases, alias) {
					t.Fatalf("aliases for %q missing %q: %#v", tt.heading, alias, aliases)
				}
			}
		})
	}
}

func TestNormalizeAliases_DeduplicatesVariants(t *testing.T) {
	aliases := normalizeAliases([]string{
		` Canonical “Characteristic” `,
		`Canonical "Characteristic"`,
		`canonical characteristic`,
		`A.CHR‑NORM`,
		`A.CHR-NORM`,
		``,
	})

	want := []string{
		"Canonical Characteristic",
		"A.CHR-NORM",
	}
	if !reflect.DeepEqual(aliases, want) {
		t.Fatalf("normalizeAliases() = %#v, want %#v", aliases, want)
	}
}

func TestNormalizePatternID_CommonForms(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "a.6", want: "A.6"},
		{input: "A6", want: "A.6"},
		{input: "A.6:", want: "A.6"},
		{input: "A.6.B", want: "A.6.B"},
		{input: "A.6:4.1", want: "A.6:4.1"},
		{input: "c.2.2A", want: "C.2.2a"},
		{input: "a.19.cn", want: "A.19.CN"},
		{input: "g.core", want: "G.CORE"},
		{input: "g.core:1", want: "G.CORE:1"},
		{input: "a.0:end", want: "A.0:END"},
		{input: "c.3.a:a.1", want: "C.3.A:A.1"},
	}

	for _, tt := range tests {
		got := normalizePatternID(tt.input)
		if got != tt.want {
			t.Fatalf("normalizePatternID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func containsAlias(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestChunkMarkdown_NormalizesPatternIDs(t *testing.T) {
	input := `## a6 - Signature Stack & Boundary Discipline

Top body.

### c.2.2A - Language-State Space

Nested body.

## G.Core - Part G Core Invariants

Core body.

### G.Core:1 - Problem frame

Problem frame body.
`

	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(chunks))
	}
	if chunks[0].PatternID != "A.6" {
		t.Fatalf("expected A.6, got %q", chunks[0].PatternID)
	}
	if chunks[1].PatternID != "C.2.2a" {
		t.Fatalf("expected C.2.2a, got %q", chunks[1].PatternID)
	}
	if chunks[1].ParentPatternID != "A.6" {
		t.Fatalf("expected parent A.6, got %q", chunks[1].ParentPatternID)
	}
	if chunks[2].PatternID != "G.CORE" {
		t.Fatalf("expected G.CORE, got %q", chunks[2].PatternID)
	}
	if chunks[3].PatternID != "G.CORE:1" {
		t.Fatalf("expected G.CORE:1, got %q", chunks[3].PatternID)
	}
	if chunks[3].ParentPatternID != "G.CORE" {
		t.Fatalf("expected parent G.CORE, got %q", chunks[3].ParentPatternID)
	}
}

func TestParseSpecCatalog_NormalizesPatternVariants(t *testing.T) {
	input := `| a6 | **Signature Stack & Boundary Discipline** | Stable | *Keywords:* boundary. | **Builds on:** a.6.b, c.2.2A. |
| g.core | **Part G Core Invariants** | Draft | *Keywords:* core. | **Builds on:** g.0. **Used by:** g.core:1. |
`

	catalog, err := ParseSpecCatalog(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	entry, ok := catalog["A.6"]
	if !ok {
		t.Fatal("expected normalized A.6 entry")
	}
	if len(entry.Edges) != 2 {
		t.Fatalf("unexpected typed edges: %#v", entry.Edges)
	}

	gotTargets := []string{entry.Edges[0].ToPatternID, entry.Edges[1].ToPatternID}
	wantTargets := []string{"A.6.B", "C.2.2a"}
	if !strings.EqualFold(strings.Join(gotTargets, ","), strings.Join(wantTargets, ",")) {
		t.Fatalf("unexpected edge targets: got %v want %v", gotTargets, wantTargets)
	}

	coreEntry, ok := catalog["G.CORE"]
	if !ok {
		t.Fatal("expected normalized G.CORE entry")
	}
	if len(coreEntry.Edges) != 2 {
		t.Fatalf("unexpected G.CORE typed edges: %#v", coreEntry.Edges)
	}
	if coreEntry.Edges[0].ToPatternID != "G.0" && coreEntry.Edges[1].ToPatternID != "G.0" {
		t.Fatalf("expected G.0 dependency in %#v", coreEntry.Edges)
	}
	if coreEntry.Edges[0].ToPatternID != "G.CORE:1" && coreEntry.Edges[1].ToPatternID != "G.CORE:1" {
		t.Fatalf("expected G.CORE:1 dependency in %#v", coreEntry.Edges)
	}
}

func TestParseDependencyEdges_NormalizesNamedRootDescendants(t *testing.T) {
	edges, relatedIDs := parseDependencyEdges("G.CORE", "Builds on: g.0. Used by: g.core:1.")

	if len(relatedIDs) != 0 {
		t.Fatalf("unexpected fallback related ids: %#v", relatedIDs)
	}
	if len(edges) != 2 {
		t.Fatalf("unexpected typed edges: %#v", edges)
	}

	gotTargets := []string{edges[0].ToPatternID, edges[1].ToPatternID}
	wantTargets := []string{"G.0", "G.CORE:1"}
	if !strings.EqualFold(strings.Join(gotTargets, ","), strings.Join(wantTargets, ",")) {
		t.Fatalf("unexpected edge targets: got %v want %v", gotTargets, wantTargets)
	}
}

func TestExtractPatternIDs_PrefersColonDescendants(t *testing.T) {
	got := extractPatternIDs("g.core:1")
	want := []string{"G.CORE:1"}

	if !strings.EqualFold(strings.Join(got, ","), strings.Join(want, ",")) {
		t.Fatalf("unexpected pattern ids: got %v want %v", got, want)
	}
}

func TestParseDependencyClauses_SplitsMultipleClauses(t *testing.T) {
	got := parseDependencyClauses("Builds on: g.0. Used by: g.core:1.")

	if len(got) != 2 {
		t.Fatalf("unexpected clauses: %#v", got)
	}
	if got[0].Label != "Builds on" || got[0].Value != "g.0" {
		t.Fatalf("unexpected first clause: %#v", got[0])
	}
	if got[1].Label != "Used by" || got[1].Value != "g.core:1" {
		t.Fatalf("unexpected second clause: %#v", got[1])
	}
}

func TestChunkMarkdown_EmptyBodiesSkipped(t *testing.T) {
	input := `## Empty Section
## Has Content

Real content here.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk (empty body skipped), got %d", len(chunks))
	}

	if chunks[0].Heading != "Has Content" {
		t.Errorf("expected heading 'Has Content', got %q", chunks[0].Heading)
	}
}

func TestChunkMarkdown_KeepsHeadingOnlyRootPatternSections(t *testing.T) {
	input := `## A.17 - Canonical “Characteristic” (A.CHR‑NORM)

### A.17:1 - Context

To have reproducibility and explainability there is a need to measure various aspects.
`

	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	root := chunks[0]
	if root.PatternID != "A.17" {
		t.Fatalf("expected root pattern A.17, got %q", root.PatternID)
	}
	if root.Body != "" {
		t.Fatalf("expected empty body for heading-only root pattern, got %q", root.Body)
	}
	if root.Summary != "Canonical Characteristic (A.CHR-NORM)" {
		t.Fatalf("expected summary fallback, got %q", root.Summary)
	}
	if !containsAlias(root.Aliases, "A.CHR-NORM") {
		t.Fatalf("expected technical alias to be preserved, got %#v", root.Aliases)
	}

	child := chunks[1]
	if child.PatternID != "A.17:1" {
		t.Fatalf("expected child pattern A.17:1, got %q", child.PatternID)
	}
	if child.ParentPatternID != "A.17" {
		t.Fatalf("expected child parent A.17, got %q", child.ParentPatternID)
	}
}

func TestFilterIndexChunks_PreservesHeadingOnlyRootPatternShells(t *testing.T) {
	chunks := []SpecChunk{
		{
			ID:        0,
			Heading:   "A.17 - Canonical Characteristic (A.CHR-NORM)",
			Level:     2,
			Body:      "",
			PatternID: "A.17",
		},
		{
			ID:        1,
			Heading:   "Empty Section",
			Level:     2,
			Body:      "",
			PatternID: "",
		},
		{
			ID:        2,
			Heading:   "Short Body",
			Level:     2,
			Body:      "too short",
			PatternID: "B.1",
		},
		{
			ID:        3,
			Heading:   "Long Body",
			Level:     2,
			Body:      "This body is definitely longer than twenty characters.",
			PatternID: "B.2",
		},
	}

	filtered := FilterIndexChunks(chunks)

	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered chunks, got %#v", filtered)
	}
	if filtered[0].PatternID != "A.17" {
		t.Fatalf("expected heading-only root shell first, got %#v", filtered[0])
	}
	if filtered[1].PatternID != "B.2" {
		t.Fatalf("expected long-body chunk second, got %#v", filtered[1])
	}
}

func TestChunkMarkdown_ContentBeforeFirstHeadingSkipped(t *testing.T) {
	input := `This content has no heading above it.

## First Real Section

Section body.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Heading != "First Real Section" {
		t.Errorf("expected 'First Real Section', got %q", chunks[0].Heading)
	}
}

func TestChunkMarkdown_SixLevelHeadings(t *testing.T) {
	input := `###### Deep heading

Deep content.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Level != 6 {
		t.Errorf("expected level 6, got %d", chunks[0].Level)
	}
}

func TestChunkMarkdown_NotAHeading(t *testing.T) {
	input := `## Real heading

##NotAHeading because no space.
#######TooManyHashes
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Body, "##NotAHeading") {
		t.Error("non-heading line should be in body")
	}
}

func TestParseMarkdownHeading(t *testing.T) {
	tests := []struct {
		line    string
		wantOK  bool
		wantLvl int
		wantTxt string
	}{
		{"# Title", true, 1, "Title"},
		{"## Section", true, 2, "Section"},
		{"### Sub", true, 3, "Sub"},
		{"###### Deep", true, 6, "Deep"},
		{"####### TooDeep", false, 0, ""},
		{"##NoSpace", false, 0, ""},
		{"Not a heading", false, 0, ""},
		{"", false, 0, ""},
		{"  ## Indented", true, 2, "Indented"},
	}

	for _, tt := range tests {
		level, text, ok := parseMarkdownHeading(tt.line)
		if ok != tt.wantOK {
			t.Errorf("parseMarkdownHeading(%q): ok=%v, want %v", tt.line, ok, tt.wantOK)
		}
		if ok {
			if level != tt.wantLvl {
				t.Errorf("parseMarkdownHeading(%q): level=%d, want %d", tt.line, level, tt.wantLvl)
			}
			if text != tt.wantTxt {
				t.Errorf("parseMarkdownHeading(%q): text=%q, want %q", tt.line, text, tt.wantTxt)
			}
		}
	}
}

func TestChunkMarkdown_IDsAreSequential(t *testing.T) {
	input := `## A
Content A.
## B
Content B.
## C
Content C.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, c := range chunks {
		if c.ID != i {
			t.Errorf("chunk[%d].ID = %d, want %d", i, c.ID, i)
		}
	}
}
