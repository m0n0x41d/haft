package typeenv

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf"
)

func TestPinnedPublicationStructuralGrammarOutcomes(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	tests := []struct {
		sourceID    string
		declaration any
	}{
		{sourceID: "A.6.5:4.2", declaration: SlotSpecProductionDeclaration{}},
		{sourceID: "A.6.5:4.3", declaration: SlotRuleDeclaration{}},
		{sourceID: "C.2.1:4.2.1", declaration: SymbolicRelationSignatureDeclaration{}},
		{sourceID: "C.2.1:4.2.2", declaration: SymbolicRelationSemanticsDeclaration{}},
		{sourceID: "C.2.1:4.3", declaration: SymbolicRelationSignatureDeclaration{}},
		{sourceID: "C.2.1:4.5", declaration: SymbolicRelationSignatureDeclaration{}},
		{sourceID: "C.3.1:4", declaration: C3ContractDeclaration{kind: C3SubkindRelationContract}},
		{sourceID: "C.3.1:5", declaration: C3ContractDeclaration{kind: C3SubkindOrderContract}},
		{sourceID: "C.3.2:5", declaration: C3ContractDeclaration{kind: C3KindSignatureContract}},
		{sourceID: "C.3.2:6", declaration: C3ContractDeclaration{kind: C3KindClassificationJudgementContract}},
		{sourceID: "C.3.2:7", declaration: C3ContractDeclaration{kind: C3KindExtensionContract}},
		{sourceID: "C.3.3:5", declaration: C3ContractDeclaration{kind: C3KindBridgeContract}},
		{sourceID: "C.3.4:5", declaration: C3ContractDeclaration{kind: C3RoleMaskContract}},
		{sourceID: "C.3.A:3", declaration: C3ContractDeclaration{kind: C3KindGuardSeparationContract}},
	}
	for _, test := range tests {
		t.Run(test.sourceID, func(t *testing.T) {
			unit := resolveGrammarSourceID(t, snapshot, test.sourceID)
			outcome := ParseStructuralUnit(unit)
			parsed, ok := outcome.(GrammarParsed)
			if !ok {
				t.Fatalf("ParseStructuralUnit(%s) = %T, want GrammarParsed", test.sourceID, outcome)
			}
			assertGrammarDeclarationType(t, parsed.Declarations(), test.declaration)
		})
	}
}

func TestPinnedA65RuleSetIsExactAndMultiline(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	unit := resolveGrammarSourceID(t, snapshot, "A.6.5:4.3")
	parsed := ParseStructuralUnit(unit).(GrammarParsed)
	want := currentSlotRuleLabels()
	got := map[string]string{}
	for _, declaration := range parsed.Declarations() {
		rule, ok := declaration.(SlotRuleDeclaration)
		if !ok {
			continue
		}
		got[rule.RuleID()] = rule.Label()
		if strings.Contains(rule.Statement(), "\n") || strings.TrimSpace(rule.Statement()) == "" {
			t.Fatalf("rule %s statement was not normalized from its multiline source", rule.RuleID())
		}
	}
	if len(got) != 7 {
		t.Fatalf("parsed rule count = %d, want 7", len(got))
	}
	for ruleID, label := range want {
		if got[ruleID] != label {
			t.Fatalf("rule %s label = %q, want %q", ruleID, got[ruleID], label)
		}
	}
}

func TestPinnedC21RelationsRemainThreeIndependentSymbolicAssemblies(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	tests := []struct {
		sourceID      string
		relationName  string
		signatureName string
		wantSlots     map[string]string
	}{
		{
			sourceID:      "C.2.1:4.2.1",
			relationName:  "EpistemeConstitutionRelation",
			signatureName: "EpistemeConstitutionRelationSignature",
			wantSlots: map[string]string{
				"ClaimGraphSlot":      "U.ClaimGraph",
				"EntityOfConcernSlot": "U.Entity",
				"ReferenceSchemeSlot": "U.ReferenceScheme",
			},
		},
		{
			sourceID:      "C.2.1:4.3",
			relationName:  "EpistemeEmpiricalGroundingRelation",
			signatureName: "EpistemeEmpiricalGroundingRelationSignature",
			wantSlots: map[string]string{
				"GroundedEpistemeSlot": "U.Episteme",
				"GroundingHolonSlot":   "U.Holon",
			},
		},
		{
			sourceID:      "C.2.1:4.5",
			relationName:  "EpistemeEditionRelation",
			signatureName: "EpistemeEditionRelationSignature",
			wantSlots: map[string]string{
				"EarlierEpistemeSlot": "U.Episteme",
				"LaterEpistemeSlot":   "U.Episteme",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.relationName, func(t *testing.T) {
			unit := resolveGrammarSourceID(t, snapshot, test.sourceID)
			parsed := ParseStructuralUnit(unit).(GrammarParsed)
			declaration := findSymbolicSignature(t, parsed.Declarations())
			if declaration.RelationName() != test.relationName ||
				declaration.SignatureName() != test.signatureName {
				t.Fatalf(
					"identity = %s / %s, want %s / %s",
					declaration.RelationName(),
					declaration.SignatureName(),
					test.relationName,
					test.signatureName,
				)
			}
			if len(declaration.Slots()) != len(test.wantSlots) {
				t.Fatalf("slot count = %d, want %d", len(declaration.Slots()), len(test.wantSlots))
			}
			for _, slot := range declaration.Slots() {
				if test.wantSlots[slot.SlotKind()] != slot.ValueKind() {
					t.Fatalf("slot %s ValueKind = %s", slot.SlotKind(), slot.ValueKind())
				}
			}
		})
	}
}

func TestPinnedEmpiricalGroundingSemanticsKeepCoverageOutsideTwoParticipants(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	unit := resolveGrammarSourceID(t, snapshot, "C.2.1:4.3")
	outcome := ParseStructuralUnit(unit)
	parsed, ok := outcome.(GrammarParsed)
	if !ok {
		t.Fatalf("ParseStructuralUnit(C.2.1:4.3) = %T, want GrammarParsed", outcome)
	}
	declarations := parsed.Declarations()
	signature := findSymbolicSignature(t, declarations)
	if len(signature.Slots()) != 2 {
		t.Fatalf("empirical-grounding participant slots = %d, want 2", len(signature.Slots()))
	}
	for _, slot := range signature.Slots() {
		if strings.Contains(slot.SlotKind(), "Covered") {
			t.Fatalf("covered claim subgraph leaked into participant SlotKind %q", slot.SlotKind())
		}
	}
	assertGrammarDeclarationType(
		t,
		declarations,
		SymbolicRelationSemanticsDeclaration{},
	)
}

func TestPinnedEmpiricalGroundingSemanticsRejectMissingCurrentWitness(t *testing.T) {
	tests := []struct {
		name        string
		witness     string
		replacement string
	}{
		{
			name:        "exact participants",
			witness:     "`EpistemeEmpiricalGroundingRelation` over participants `(E,H)`",
			replacement: "`EpistemeEmpiricalGroundingRelation(E,H)`",
		},
		{
			name:        "covered predicate content",
			witness:     "with `covered=C`",
			replacement: "with designated coverage",
		},
		{
			name:        "direct obtaining",
			witness:     "obtains exactly while every empirical claim",
			replacement: "is described while every empirical claim",
		},
		{
			name:        "covered identity discriminator",
			witness:     "One occurrence is identified by `<episteme, exact covered claim subgraph, grounding holon,",
			replacement: "One occurrence is identified by `<episteme, grounding holon,",
		},
		{
			name:        "continuous coverage interval",
			witness:     "maximal continuous interval during which the complete coverage predicate is true>`",
			replacement: "maximal continuous grounding interval>`",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := loadPinnedGrammarSnapshot(t)
			unit := resolveGrammarSourceID(t, snapshot, "C.2.1:4.3")
			if strings.Count(unit.Body, test.witness) != 1 {
				t.Fatalf(
					"source contains %d copies of witness %q, want 1",
					strings.Count(unit.Body, test.witness),
					test.witness,
				)
			}
			unit.Body = strings.Replace(unit.Body, test.witness, test.replacement, 1)
			outcome := ParseStructuralUnit(unit)
			malformed, ok := outcome.(GrammarMalformed)
			if !ok {
				t.Fatalf("mutated empirical-grounding semantics = %T, want GrammarMalformed", outcome)
			}
			diagnostics := malformed.Diagnostics()
			if len(diagnostics) != 1 ||
				diagnostics[0].Code() != "relation_semantics_source_malformed" {
				t.Fatalf("mutated diagnostics = %#v", diagnostics)
			}
		})
	}
}

func TestPinnedClaimGraphSlotIsByValueWithoutInventedCardinality(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	unit := resolveGrammarSourceID(t, snapshot, "C.2.1:4.2.1")
	declaration := findSymbolicSignature(
		t,
		ParseStructuralUnit(unit).(GrammarParsed).Declarations(),
	)
	slot := findSymbolicSlot(t, declaration, "ClaimGraphSlot")
	if slot.ValueKind() != "U.ClaimGraph" {
		t.Fatalf("ClaimGraph ValueKind = %q", slot.ValueKind())
	}
	if _, ok := slot.ReferenceMode().(ByValueEvidence); !ok {
		t.Fatalf("ClaimGraph reference mode = %T, want ByValueEvidence", slot.ReferenceMode())
	}
}

func TestPinnedReferenceSchemeSlotIsExactByValue(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	unit := resolveGrammarSourceID(t, snapshot, "C.2.1:4.2.1")
	declaration := findSymbolicSignature(
		t,
		ParseStructuralUnit(unit).(GrammarParsed).Declarations(),
	)
	slot := findSymbolicSlot(t, declaration, "ReferenceSchemeSlot")
	if slot.ValueKind() != "U.ReferenceScheme" {
		t.Fatalf("ReferenceScheme ValueKind = %q", slot.ValueKind())
	}
	if _, ok := slot.ReferenceMode().(ByValueEvidence); !ok {
		t.Fatalf("ReferenceScheme ref mode = %T, want ByValueEvidence", slot.ReferenceMode())
	}
}

func TestRecognizedGrammarMutationFailsMalformed(t *testing.T) {
	unit := fpf.SourceUnit{
		UnitID:          "fixture:slot-production",
		SourceID:        "A.6.5:4.2",
		Role:            fpf.SourceUnitRolePatternSection,
		ParentPatternID: "A.6.5",
		Body:            "```text\nSlotSpec := <SlotKind, ValueKind>\n```",
	}
	outcome := ParseStructuralUnit(unit)
	malformed, ok := outcome.(GrammarMalformed)
	if !ok || malformed.Diagnostics()[0].Code() != "slot_spec_production_malformed" {
		t.Fatalf("mutated production = %#v", outcome)
	}
}

func TestRecognizedRuleSetRejectsMissingIndentedBody(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	unit := resolveGrammarSourceID(t, snapshot, "A.6.5:4.3")
	unit.Body = strings.Replace(unit.Body, "  every relation-participant", "every relation-participant", 1)
	malformed, ok := ParseStructuralUnit(unit).(GrammarMalformed)
	if !ok || malformed.Diagnostics()[0].Code() != "slot_rule_malformed" {
		t.Fatalf("malformed rule set = %#v", ParseStructuralUnit(unit))
	}
}

func TestRecognizedRelationTableRejectsUnknownRefMode(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	unit := resolveGrammarSourceID(t, snapshot, "C.2.1:4.2.1")
	unit.Body = strings.Replace(unit.Body, "`U.EntityRef`", "`byRef`", 1)
	malformed, ok := ParseStructuralUnit(unit).(GrammarMalformed)
	if !ok || malformed.Diagnostics()[0].Code() != "relation_signature_slot_table_mismatch" {
		t.Fatalf("unknown refMode = %#v", ParseStructuralUnit(unit))
	}
}

func TestDidacticSlotWordsDoNotMintDeclarations(t *testing.T) {
	unit := fpf.SourceUnit{
		UnitID: "fixture:didactic",
		Role:   fpf.SourceUnitRolePatternSection,
		Body:   "Example: a System may fill ExampleSlot. This is not a declaration table.",
	}
	if outcome := ParseStructuralUnit(unit); outcome != (GrammarNoMatch{unitID: unit.UnitID}) {
		t.Fatalf("didactic-only source = %T, want GrammarNoMatch", outcome)
	}
}

func TestUnrelatedNormativeProseDoesNotEnterStructuralGrammar(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	sourceIDs := []string{
		"A.14:1",
		"A.14:2",
		"A.15:1",
		"A.6.3.CSC:1",
		"C.2.2:1",
	}
	for _, sourceID := range sourceIDs {
		unit := resolveGrammarSourceID(t, snapshot, sourceID)
		outcome := ParseStructuralUnit(unit)
		if outcome.UnitID() != unit.UnitID {
			t.Fatalf("ParseStructuralUnit(%s) changed source identity", sourceID)
		}
		if _, ok := outcome.(GrammarNoMatch); !ok {
			t.Fatalf("ParseStructuralUnit(%s) = %T, want GrammarNoMatch", sourceID, outcome)
		}
	}
}

func loadPinnedGrammarSnapshot(t *testing.T) fpf.PublicationSnapshot {
	t.Helper()
	readmePath := filepath.Join("..", "..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "..", "data", "FPF", "FPF-Spec.md")
	snapshot, err := fpf.LoadPublicationSnapshot(readmePath, specPath, "")
	if err != nil {
		t.Fatalf("LoadPublicationSnapshot(): %v", err)
	}
	return snapshot
}

func resolveGrammarUnit(
	t *testing.T,
	snapshot fpf.PublicationSnapshot,
	unitID string,
) fpf.SourceUnit {
	t.Helper()
	unit, ok := snapshot.ResolveSourceUnit(unitID)
	if !ok {
		t.Fatalf("pinned source unit %q not found", unitID)
	}
	return unit
}

func resolveGrammarSourceID(
	t *testing.T,
	snapshot fpf.PublicationSnapshot,
	sourceID string,
) fpf.SourceUnit {
	t.Helper()
	for _, unit := range snapshot.SourceUnits() {
		if unit.SourceID == sourceID && unit.Role == fpf.SourceUnitRolePatternSection {
			return unit
		}
	}
	t.Fatalf("pinned source ID %q not found", sourceID)
	return fpf.SourceUnit{}
}

func findSymbolicSignature(
	t *testing.T,
	declarations []StructuralDeclaration,
) SymbolicRelationSignatureDeclaration {
	t.Helper()
	for _, declaration := range declarations {
		if signature, ok := declaration.(SymbolicRelationSignatureDeclaration); ok {
			return signature
		}
	}
	t.Fatal("parsed declarations contain no symbolic relation signature")
	return SymbolicRelationSignatureDeclaration{}
}

func findSymbolicSlot(
	t *testing.T,
	declaration SymbolicRelationSignatureDeclaration,
	slotKind string,
) SymbolicRelationSlotSpec {
	t.Helper()
	for _, slot := range declaration.Slots() {
		if slot.SlotKind() == slotKind {
			return slot
		}
	}
	t.Fatalf("signature %s has no %s", declaration.SignatureName(), slotKind)
	return SymbolicRelationSlotSpec{}
}

func assertGrammarDeclarationType(
	t *testing.T,
	declarations []StructuralDeclaration,
	want any,
) {
	t.Helper()
	for _, declaration := range declarations {
		switch want.(type) {
		case SlotSpecProductionDeclaration:
			if _, ok := declaration.(SlotSpecProductionDeclaration); ok {
				return
			}
		case SlotRuleDeclaration:
			if _, ok := declaration.(SlotRuleDeclaration); ok {
				return
			}
		case SymbolicRelationSignatureDeclaration:
			if _, ok := declaration.(SymbolicRelationSignatureDeclaration); ok {
				return
			}
		case SymbolicRelationSemanticsDeclaration:
			if _, ok := declaration.(SymbolicRelationSemanticsDeclaration); ok {
				return
			}
		case C3ContractDeclaration:
			candidate, ok := declaration.(C3ContractDeclaration)
			expected := want.(C3ContractDeclaration)
			if ok && candidate.Kind() == expected.Kind() {
				return
			}
		}
	}
	t.Fatalf("declarations did not contain %T", want)
}
