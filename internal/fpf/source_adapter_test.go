package fpf

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSourceUnits_ProductionGrammarAndProvenance(t *testing.T) {
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")

	units, err := LoadSourceUnits(readmePath, specPath, "fixture-revision")
	if err != nil {
		t.Fatalf("LoadSourceUnits() error: %v", err)
	}

	counts := make(map[SourceUnitRole]int)
	var architecture SourceUnit
	var strictDistinctionBody SourceUnit
	var strictDistinctionTOC SourceUnit
	var relationOccurrenceBody SourceUnit
	var relationOccurrenceTOC SourceUnit
	var productionBody SourceUnit
	var formalityTOC SourceUnit
	var stratificationTOC SourceUnit
	for _, unit := range units {
		counts[unit.Role]++
		if unit.SourceID == "ARCHITECTURE" {
			architecture = unit
		}
		if unit.Role == SourceUnitRolePatternBody && unit.SourceID == "A.7" {
			strictDistinctionBody = unit
		}
		if unit.Role == SourceUnitRoleTOCRow && unit.PatternID == "A.7" {
			strictDistinctionTOC = unit
		}
		if unit.Role == SourceUnitRolePatternBody && unit.SourceID == "A.6.REL" {
			relationOccurrenceBody = unit
		}
		if unit.Role == SourceUnitRolePatternBody && unit.SourceID == "A.15.PROD" {
			productionBody = unit
		}
		if unit.Role == SourceUnitRoleTOCRow && unit.PatternID == "A.6.REL" {
			relationOccurrenceTOC = unit
		}
		if unit.Role == SourceUnitRoleTOCRow && unit.PatternID == "C.2.3" {
			formalityTOC = unit
		}
		if unit.Role == SourceUnitRoleTOCRow && unit.PatternID == "C.30.STRAT" {
			stratificationTOC = unit
		}
		if unit.Role == SourceUnitRolePracticalUseCard {
			if unit.UseCues.ConditionText == "" || unit.UseCues.FirstResultText == "" || unit.UseCues.StopReturnText == "" {
				t.Errorf("practical-use card %s lacks condition/result/boundary cues: %#v", unit.SourceID, unit.UseCues)
			}
		}
	}

	if counts[SourceUnitRolePracticalUseCard] == 0 || counts[SourceUnitRolePreface] == 0 || counts[SourceUnitRoleTOCRow] == 0 || counts[SourceUnitRolePatternBody] == 0 || counts[SourceUnitRolePatternSection] == 0 {
		t.Fatalf("missing required source roles: %#v", counts)
	}
	if architecture.UnitID == "" || architecture.UseCues.ConditionText == "" || architecture.UseCues.FirstResultText == "" || architecture.UseCues.StopReturnText == "" {
		t.Fatalf("architecture card lacks source-owned use cues: %#v", architecture.UseCues)
	}
	if architecture.Provenance.SourceRevision != "fixture-revision" || architecture.Provenance.StartLine <= 0 || architecture.Provenance.ContentHash != sourceContentHash(architecture.Body) {
		t.Fatalf("architecture provenance invalid: %#v", architecture.Provenance)
	}
	if strictDistinctionBody.UnitID == "" || strictDistinctionTOC.UnitID == "" {
		t.Fatal("exact A.7 pattern body and ToC row must both be structurally resolvable")
	}
	if relationOccurrenceBody.UnitID == "" || relationOccurrenceTOC.UnitID == "" {
		t.Fatal("exact A.6.REL pattern body and ToC row must both be structurally resolvable")
	}
	if !strings.Contains(relationOccurrenceBody.Body, "assertion") ||
		!strings.Contains(relationOccurrenceBody.Body, "occurrence") {
		t.Fatal("A.6.REL hydration lost the source-owned assertion/occurrence distinction")
	}
	if productionBody.UnitID == "" {
		t.Fatal("exact A.15.PROD pattern body must be structurally resolvable")
	}
	processRecoveryRelation := findSourceRelation(
		t,
		productionBody.Relations,
		SourceRelationKindCoordinatesWith,
		"A.15.6",
	)
	if processRecoveryRelation.TargetClass != SourceRelationTargetClassLocalPattern {
		t.Fatalf("A.15.PROD -> A.15.6 target class = %q; want current local pattern", processRecoveryRelation.TargetClass)
	}
	if !containsSourceString(formalityTOC.DirectRefs, "C.2") || containsSourceString(formalityTOC.DirectRefs, "F.0") {
		t.Fatalf("C.2.3 direct refs = %#v, want C.2 and no F0 scale false positive", formalityTOC.DirectRefs)
	}
	if !containsSourceString(stratificationTOC.DirectRefs, "I.2") {
		t.Fatalf("C.30.STRAT direct refs = %#v, want cross-cluster I.2 source link", stratificationTOC.DirectRefs)
	}
}

func TestBuildSourceUnits_PracticalUseCardsComeFromEmbeddedReadmeCarrier(t *testing.T) {
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")
	readme := mustReadSourceFixture(t, readmePath)
	spec := mustReadSourceFixture(t, specPath)
	bundle := SourceBundle{
		Readme: SourceDocument{Path: readmePath, SourceRevision: "rev", Markdown: readme},
		Spec:   SourceDocument{Path: specPath, SourceRevision: "rev", Markdown: spec},
	}
	units, err := BuildSourceUnits(bundle)
	if err != nil {
		t.Fatalf("independent publication carriers should be accepted: %v", err)
	}
	architecture := findSourceUnitByRoleAndSourceID(
		t,
		units,
		SourceUnitRolePracticalUseCard,
		"ARCHITECTURE",
	)
	if architecture.Provenance.SourcePath != specPath {
		t.Fatalf("architecture card source path = %q; want embedded carrier %q", architecture.Provenance.SourcePath, specPath)
	}

	const sourcePhrase = "Architecture-relevant problem pressure"
	const standaloneMarker = "STANDALONE-COMPANION-ONLY"
	mutatedReadme := strings.Replace(string(readme), sourcePhrase, standaloneMarker, 1)
	if mutatedReadme == string(readme) {
		t.Fatal("standalone README fixture did not contain the architecture-card phrase")
	}
	bundle.Readme.Markdown = []byte(mutatedReadme)
	standaloneMutatedUnits, err := BuildSourceUnits(bundle)
	if err != nil {
		t.Fatalf("standalone companion divergence should not shadow embedded cards: %v", err)
	}
	architecture = findSourceUnitByRoleAndSourceID(
		t,
		standaloneMutatedUnits,
		SourceUnitRolePracticalUseCard,
		"ARCHITECTURE",
	)
	if strings.Contains(architecture.Body, standaloneMarker) {
		t.Fatal("standalone companion text became practical-use semantic authority")
	}

	const embeddedMarker = "EMBEDDED-CURRENT-CARD"
	mutatedSpec := strings.Replace(string(spec), sourcePhrase, embeddedMarker, 1)
	if mutatedSpec == string(spec) {
		t.Fatal("embedded README fixture did not contain the architecture-card phrase")
	}
	bundle.Readme.Markdown = readme
	bundle.Spec.Markdown = []byte(mutatedSpec)
	embeddedMutatedUnits, err := BuildSourceUnits(bundle)
	if err != nil {
		t.Fatalf("embedded README card mutation should remain parseable: %v", err)
	}
	architecture = findSourceUnitByRoleAndSourceID(
		t,
		embeddedMutatedUnits,
		SourceUnitRolePracticalUseCard,
		"ARCHITECTURE",
	)
	if !strings.Contains(architecture.Body, embeddedMarker) {
		t.Fatal("embedded README card did not govern the practical-use source unit")
	}
}

func TestBuildSourceUnits_BrokenTOCDirectReferenceFailsLoudly(t *testing.T) {
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")
	readme := mustReadSourceFixture(t, readmePath)
	spec := string(mustReadSourceFixture(t, specPath))

	needle := "**Builds on:** C.2. **Constrains:** all patterns referencing F-G-R or language-state facets."
	broken := strings.Replace(
		spec,
		needle,
		"**Builds on:** A.999. **Constrains:** all patterns referencing F-G-R or language-state facets.",
		1,
	)
	if broken == spec {
		t.Fatal("test fixture did not mutate the C.2.3 ToC dependency")
	}

	bundle := SourceBundle{
		Readme: SourceDocument{Path: readmePath, SourceRevision: "rev", Markdown: readme},
		Spec:   SourceDocument{Path: specPath, SourceRevision: "rev", Markdown: []byte(broken)},
	}
	_, err := BuildSourceUnits(bundle)
	if err == nil || !strings.Contains(err.Error(), "A.999") {
		t.Fatalf("expected unresolved A.999 direct-reference failure, got %v", err)
	}
}

func TestLoadSourceUnits_PlannedTOCSearchVocabularyExcludesDependencies(t *testing.T) {
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")

	units, err := LoadSourceUnits(readmePath, specPath, "fixture-revision")
	if err != nil {
		t.Fatalf("LoadSourceUnits() error: %v", err)
	}

	var plannedTOC SourceUnit
	for _, unit := range units {
		if unit.Role == SourceUnitRoleTOCRow && unit.PatternID == "C.1" {
			plannedTOC = unit
			break
		}
	}
	if plannedTOC.UnitID == "" {
		t.Fatal("planned C.1 ToC source unit not found")
	}
	if !containsSourceString(plannedTOC.AuthoredPhrases, "How to model physical systems in FPF?") {
		t.Fatalf("C.1 authored phrases = %#v, want source-owned search phrase", plannedTOC.AuthoredPhrases)
	}
	for _, phrase := range plannedTOC.AuthoredPhrases {
		if strings.Contains(phrase, "Builds on:") || strings.Contains(phrase, "Coordinates with:") {
			t.Fatalf("C.1 authored phrase contains dependency-cell text: %q", phrase)
		}
	}
}

func TestLoadSourceUnits_ProjectsExplicitTOCRelationsWithoutInventingResolution(t *testing.T) {
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")

	units, err := LoadSourceUnits(readmePath, specPath, "relation-fixture-revision")
	if err != nil {
		t.Fatalf("LoadSourceUnits() error: %v", err)
	}

	stratificationBody := findSourceUnitByRoleAndPatternID(t, units, SourceUnitRolePatternBody, "C.30.STRAT")
	stratificationTOC := findSourceUnitByRoleAndPatternID(t, units, SourceUnitRoleTOCRow, "C.30.STRAT")
	coordination := findSourceRelation(t, stratificationBody.Relations, SourceRelationKindCoordinatesWith, "I.2")
	if coordination.TargetClass != SourceRelationTargetClassLocalPattern {
		t.Fatalf("C.30.STRAT -> I.2 target class = %q, want local", coordination.TargetClass)
	}
	if coordination.Provenance != stratificationTOC.Provenance {
		t.Fatalf("relation provenance = %#v, want exact ToC row %#v", coordination.Provenance, stratificationTOC.Provenance)
	}
	if len(stratificationTOC.Relations) != 0 {
		t.Fatalf("published pattern relations must project onto body, got ToC relations %#v", stratificationTOC.Relations)
	}

	planned := findSourceUnitByRoleAndPatternID(t, units, SourceUnitRoleTOCRow, "C.1")
	if planned.PublicationStatus != "Planned" || len(planned.Relations) == 0 {
		t.Fatalf("planned C.1 must retain authored typed relations on its ToC unit: %#v", planned)
	}
	findSourceRelation(t, planned.Relations, SourceRelationKindBuildsOn, "A.1")

	mechanismBody := findSourceUnitByRoleAndPatternID(t, units, SourceUnitRolePatternBody, "E.20")
	external := findSourceRelation(t, mechanismBody.Relations, SourceRelationKindCoordinatesWith, "G.X:EXT")
	if external.TargetClass != SourceRelationTargetClassAuthoredNonlocal {
		t.Fatalf("E.20 -> G.X:EXT target class = %q, want explicit authored nonlocal", external.TargetClass)
	}
}

func TestProjectTOCRelations_RowPermutationAndLineShiftPreserveGraphIdentity(t *testing.T) {
	bodies := []SourceUnit{
		{UnitID: "body:a1", SourceID: "A.1", Role: SourceUnitRolePatternBody, PatternID: "A.1"},
		{UnitID: "body:a2", SourceID: "A.2", Role: SourceUnitRolePatternBody, PatternID: "A.2"},
	}
	toc := []SourceUnit{
		{UnitID: "toc:a1", Role: SourceUnitRoleTOCRow, PatternID: "A.1", Provenance: SourceProvenance{SourcePath: "FPF-Spec.md", StartLine: 10, EndLine: 10, ContentHash: "row-a1", SourceRevision: "rev"}},
		{UnitID: "toc:a2", Role: SourceUnitRoleTOCRow, PatternID: "A.2", Provenance: SourceProvenance{SourcePath: "FPF-Spec.md", StartLine: 11, EndLine: 11, ContentHash: "row-a2", SourceRevision: "rev"}},
		{UnitID: "toc:c1", SourceID: "C.1", Role: SourceUnitRoleTOCRow, PatternID: "C.1", PublicationStatus: "Planned", Provenance: SourceProvenance{SourcePath: "FPF-Spec.md", StartLine: 12, EndLine: 12, ContentHash: "row-c1", SourceRevision: "rev"}},
	}
	catalog := map[string]SpecCatalogEntry{
		"A.1": {PatternID: "A.1", Edges: []SpecEdge{{FromPatternID: "A.1", ToPatternID: "A.2", EdgeType: SpecEdgeTypeBuildsOn}}},
		"C.1": {PatternID: "C.1", Edges: []SpecEdge{{FromPatternID: "C.1", ToPatternID: "A.1", EdgeType: SpecEdgeTypeCoordinatesWith}}},
	}

	projectedBodies, projectedTOC, err := projectTOCRelationsToCanonicalUnits(bodies, toc, catalog)
	if err != nil {
		t.Fatalf("first projection error: %v", err)
	}
	first := sourceRelationIdentitySet(projectedBodies, projectedTOC)

	shiftedBodies := []SourceUnit{bodies[1], bodies[0]}
	shiftedTOC := []SourceUnit{toc[2], toc[0], toc[1]}
	for index := range shiftedTOC {
		shiftedTOC[index].Provenance.StartLine += 100
		shiftedTOC[index].Provenance.EndLine += 100
	}
	projectedBodies, projectedTOC, err = projectTOCRelationsToCanonicalUnits(shiftedBodies, shiftedTOC, catalog)
	if err != nil {
		t.Fatalf("shifted projection error: %v", err)
	}
	second := sourceRelationIdentitySet(projectedBodies, projectedTOC)
	if !maps.Equal(first, second) {
		t.Fatalf("relation identity changed after row permutation/line shift: first=%#v second=%#v", first, second)
	}
	if _, exists := second["body:a1|builds_on|A.2|local_pattern"]; !exists {
		t.Fatalf("published subject relation did not remain on body UnitID: %#v", second)
	}
	if _, exists := second["toc:c1|coordinates_with|A.1|local_pattern"]; !exists {
		t.Fatalf("planned subject relation did not remain on ToC UnitID: %#v", second)
	}
}

func sourceRelationIdentitySet(groups ...[]SourceUnit) map[string]struct{} {
	identities := make(map[string]struct{})
	for _, units := range groups {
		for _, unit := range units {
			for _, relation := range unit.Relations {
				key := unit.UnitID + "|" + string(relation.Kind) + "|" + relation.TargetPatternID + "|" + string(relation.TargetClass)
				identities[key] = struct{}{}
			}
		}
	}
	return identities
}

func findSourceUnitByRoleAndPatternID(t *testing.T, units []SourceUnit, role SourceUnitRole, patternID string) SourceUnit {
	t.Helper()
	for _, unit := range units {
		if unit.Role == role && unit.PatternID == patternID {
			return unit
		}
	}
	t.Fatalf("source unit %s in role %s not found", patternID, role)
	return SourceUnit{}
}

func findSourceUnitByRoleAndSourceID(t *testing.T, units []SourceUnit, role SourceUnitRole, sourceID string) SourceUnit {
	t.Helper()
	for _, unit := range units {
		if unit.Role == role && unit.SourceID == sourceID {
			return unit
		}
	}
	t.Fatalf("source unit %s in role %s not found", sourceID, role)
	return SourceUnit{}
}

func findSourceRelation(t *testing.T, relations []SourceRelation, kind SourceRelationKind, targetPatternID string) SourceRelation {
	t.Helper()
	for _, relation := range relations {
		if relation.Kind == kind && relation.TargetPatternID == targetPatternID {
			return relation
		}
	}
	t.Fatalf("relation %s -> %s not found in %#v", kind, targetPatternID, relations)
	return SourceRelation{}
}

func TestParseSpecCatalog_RejectsClusterDividerAsPattern(t *testing.T) {
	markdown := strings.NewReader(`
| § | ID & Title | Status | Keywords & Search Queries | Dependencies |
| --- | --- | --- | --- | --- |
| ***Cluster A.I - Foundational Ontology*** | | | | |
| A.1 | Holon Foundation | Stable | Keywords: holon. Queries: "What is a holon?" | Builds on: E.1. |
`)
	catalog, err := ParseSpecCatalog(markdown)
	if err != nil {
		t.Fatalf("ParseSpecCatalog() error: %v", err)
	}
	if _, exists := catalog["A.I"]; exists {
		t.Fatal("cluster divider A.I was misclassified as a pattern")
	}
	if _, exists := catalog["A.1"]; !exists {
		t.Fatal("real A.1 row missing from catalog")
	}
}

func TestValidateSourceUnitsRejectsMissingSectionParent(t *testing.T) {
	units := minimalValidSourceUnits()
	for index := range units {
		if units[index].Role == SourceUnitRolePatternSection {
			units[index].ParentPatternID = "A.999"
		}
	}
	if err := ValidateSourceUnits(units); err == nil || !strings.Contains(err.Error(), "missing parent pattern body") {
		t.Fatalf("ValidateSourceUnits() error = %v, want missing parent failure", err)
	}
}

func containsSourceString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mustReadSourceFixture(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return content
}
