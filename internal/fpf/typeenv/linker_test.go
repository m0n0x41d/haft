package typeenv

import (
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestLinkerUsesParentPatternAndEmitsOnlyTypedRelationFragment(t *testing.T) {
	revision, _ := typedmemory.NewSourceRevision("fixture-revision")
	compiler, _ := typedmemory.NewCompilerSchemaVersion(baseTypeEnvCompilerSchema)
	rootUnit := linkerSourceUnit(
		"fixture:root",
		"Fixture.Root.Section",
		"Fixture.Owner",
		10,
	)
	slotUnit := linkerSourceUnit(
		"fixture:slot",
		"Fixture.Slot.Section",
		"Fixture.Owner",
		20,
	)
	root := RelationRootDeclaration{
		source:      rootUnit,
		owner:       rootUnit.ParentPatternID,
		subjectKind: "Fixture.Subject",
		relation:    "Fixture.Relation",
	}
	slot := SlotDeclarationFragment{
		source:      slotUnit,
		owner:       slotUnit.ParentPatternID,
		slotKind:    "FixtureSlot",
		valueKind:   "Fixture.Value",
		reference:   ByValueEvidence{},
		cardinality: BoundedCardinalityEvidence{minimum: 1, maximum: 1},
	}
	artifact, err := linkStructuralDeclarations(
		revision,
		compiler,
		[]StructuralDeclaration{slot, root},
		nil,
	)
	if err != nil {
		t.Fatalf("linkStructuralDeclarations() error = %v", err)
	}
	symbols := artifactSymbolStrings(artifact)
	assertContainsString(t, symbols, "signature:Fixture.Relation")
	assertContainsString(t, symbols, "slot_kind:Fixture.Relation/slot/FixtureSlot")
	assertNotContainsString(t, symbols, "signature:Fixture.Owner")

	signatureID, _ := typedmemory.NewSignatureID("Fixture.Relation")
	signatureSymbol, _ := typedmemory.RelationSymbolRef(signatureID)
	subject, _ := typedmemory.SchemaSymbolCoverage(signatureSymbol)
	entry, exists := artifact.CoverageManifest().Entry(subject)
	if !exists || entry.Posture() != typedmemory.CoverageCompiled {
		t.Fatalf("closed relation coverage = (%v, %s), want compiled", exists, entry.Posture())
	}
	environment, err := LowerBaseTypeEnvArtifact(artifact)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifact() error = %v", err)
	}
	fragments := environment.TypedRelationDeclarationFragments()
	if len(fragments) != 1 {
		t.Fatalf("lowered relation fragment count = %d, want 1", len(fragments))
	}
	if fragments[0].Posture() != typedmemory.RelationDeclarationTypedFragment {
		t.Fatalf("lowered relation posture = %q", fragments[0].Posture())
	}
	if len(fragments[0].Slots()) != 1 {
		t.Fatalf("lowered slot count = %d, want 1", len(fragments[0].Slots()))
	}
}

func TestLinkerDoesNotEmitIncompleteRelationSignature(t *testing.T) {
	revision, _ := typedmemory.NewSourceRevision("fixture-revision")
	compiler, _ := typedmemory.NewCompilerSchemaVersion(baseTypeEnvCompilerSchema)
	rootUnit := linkerSourceUnit("fixture:root", "Fixture.Root", "Fixture.Owner", 10)
	slotUnit := linkerSourceUnit("fixture:slot", "Fixture.Slot", "Fixture.Owner", 20)
	root := RelationRootDeclaration{
		source:      rootUnit,
		owner:       rootUnit.ParentPatternID,
		subjectKind: "Fixture.Subject",
		relation:    "Fixture.Relation",
	}
	slot := SlotDeclarationFragment{
		source:      slotUnit,
		owner:       slotUnit.ParentPatternID,
		slotKind:    "FixtureSlot",
		valueKind:   "Fixture.Value",
		reference:   ByValueEvidence{},
		cardinality: MissingCardinalityEvidence{},
	}
	artifact, err := linkStructuralDeclarations(
		revision,
		compiler,
		[]StructuralDeclaration{root, slot},
		nil,
	)
	if err == nil {
		symbols := artifactSymbolStrings(artifact)
		assertNotContainsString(t, symbols, "signature:Fixture.Relation")
		return
	}
	if !strings.Contains(err.Error(), "compiled artifact requires at least one complete declaration") {
		t.Fatalf("incomplete relation error = %v", err)
	}
}

func TestLinkerKeepsSameParentSymbolicRelationsIndependentAndSourceOnly(t *testing.T) {
	revision, _ := typedmemory.NewSourceRevision("fixture-revision")
	compiler, _ := typedmemory.NewCompilerSchemaVersion(baseTypeEnvCompilerSchema)
	structural := make([]StructuralDeclaration, 0, 6)
	relationNames := []string{"Fixture.FirstRelation", "Fixture.SecondRelation", "Fixture.ThirdRelation"}
	for index, relationName := range relationNames {
		signatureUnit := linkerSourceUnit(
			fmt.Sprintf("fixture:signature:%d", index),
			fmt.Sprintf("Fixture.Signature.%d", index),
			"C.2.1",
			100+(index*20),
		)
		semanticsUnit := linkerSourceUnit(
			fmt.Sprintf("fixture:semantics:%d", index),
			fmt.Sprintf("Fixture.Semantics.%d", index),
			"C.2.1",
			110+(index*20),
		)
		signature := SymbolicRelationSignatureDeclaration{
			source:        signatureUnit,
			owner:         "C.2.1",
			relationName:  relationName,
			signatureName: relationName + "Signature",
			slots: []SymbolicRelationSlotSpec{{
				slotKind:           fmt.Sprintf("FixtureSlot%d", index),
				participantMeaning: "fixture participant",
				valueKind:          fmt.Sprintf("Fixture.Value%d", index),
				reference:          ByValueEvidence{},
			}},
		}
		semantics := SymbolicRelationSemanticsDeclaration{
			source:        semanticsUnit,
			owner:         "C.2.1",
			relationName:  relationName,
			signatureName: relationName + "Signature",
		}
		structural = append(structural, signature, semantics)
	}
	artifact, err := linkStructuralDeclarations(revision, compiler, structural, nil)
	if err != nil {
		t.Fatalf("linkStructuralDeclarations() error = %v", err)
	}
	for _, relationName := range relationNames {
		id, _ := typedmemory.NewSignatureID(relationName)
		symbol, _ := typedmemory.RelationSymbolRef(id)
		if _, exists := artifactDeclaration(artifact, symbol); !exists {
			t.Fatalf("symbolic declaration %s was merged into another C.2.1 relation", relationName)
		}
		subject, _ := typedmemory.SchemaSymbolCoverage(symbol)
		entry, exists := artifact.CoverageManifest().Entry(subject)
		if !exists || entry.Posture() != typedmemory.CoverageSourceOnly {
			t.Fatalf("%s coverage = %#v, want source_only", relationName, entry)
		}
	}
	environment, err := LowerBaseTypeEnvArtifact(artifact)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifact() error = %v", err)
	}
	if len(environment.RelationSignatures()) != 0 {
		t.Fatalf("lowered %d symbolic relation signatures", len(environment.RelationSignatures()))
	}
}

func TestCoverageDedupRejectsConflictingEntry(t *testing.T) {
	unit := linkerSourceUnit("fixture:coverage", "Fixture.Coverage", "Fixture.Owner", 40)
	first, err := sourceOnlyUnitGap(unit, "first_reason")
	if err != nil {
		t.Fatalf("sourceOnlyUnitGap(first) error = %v", err)
	}
	equivalent, err := sourceOnlyUnitGap(unit, "first_reason")
	if err != nil {
		t.Fatalf("sourceOnlyUnitGap(equivalent) error = %v", err)
	}
	conflicting, err := sourceOnlyUnitGap(unit, "second_reason")
	if err != nil {
		t.Fatalf("sourceOnlyUnitGap(conflicting) error = %v", err)
	}
	entries, err := appendUniqueCoverageEntry(nil, first)
	if err != nil {
		t.Fatalf("appendUniqueCoverageEntry(first) error = %v", err)
	}
	entries, err = appendUniqueCoverageEntry(entries, equivalent)
	if err != nil {
		t.Fatalf("appendUniqueCoverageEntry(equivalent) error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("equivalent duplicate count = %d, want 1", len(entries))
	}
	if _, err := appendUniqueCoverageEntry(entries, conflicting); err == nil {
		t.Fatal("conflicting duplicate coverage was silently collapsed")
	}
}

func linkerSourceUnit(
	unitID string,
	patternID string,
	parentPatternID string,
	line int,
) fpf.SourceUnit {
	return fpf.SourceUnit{
		UnitID:          unitID,
		Role:            fpf.SourceUnitRolePatternSection,
		PatternID:       patternID,
		ParentPatternID: parentPatternID,
		Provenance: fpf.SourceProvenance{
			SourcePath:     "fixture.md",
			StartLine:      line,
			EndLine:        line + 4,
			ContentHash:    strings.Repeat("a", 64),
			SourceRevision: "fixture-revision",
		},
	}
}
