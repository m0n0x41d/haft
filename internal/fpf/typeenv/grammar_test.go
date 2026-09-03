package typeenv

import (
	"path/filepath"
	"slices"
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
		{sourceID: "C.3.2:6", declaration: C3ContractDeclaration{kind: C3KindClassificationContract}},
		{sourceID: "C.3.2:7", declaration: C3ContractDeclaration{kind: C3KindExtensionContract}},
		{sourceID: "C.3.3:5", declaration: C3ContractDeclaration{kind: C3KindBridgeContract}},
		{sourceID: "C.3.4:5", declaration: C3ContractDeclaration{kind: C3KindUseAdaptationContract}},
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

func TestCurrentC3SemanticProfilesAreExactAndSourceOnly(t *testing.T) {
	tests := []struct {
		sourceID        string
		owner           string
		kind            C3ContractKind
		designator      string
		wantCoordinates []string
	}{
		{
			sourceID:   "C.3.1:4",
			owner:      "C.3.1",
			kind:       C3SubkindRelationContract,
			designator: "U.SubkindOf",
			wantCoordinates: []string{
				"narrower_kind", "broader_kind", "declared_applicability",
				"SubkindOfObtains", "criterion_entailment_branch",
				"exhaustive_closed_finite_domain_branch",
				"participant_determined_occurrence_identity",
				"scheme_signature_applicability_qualifiers",
				"separate_c2_1_assertion_episteme",
			},
		},
		{
			sourceID:   "C.3.1:5",
			owner:      "C.3.1",
			kind:       C3SubkindOrderContract,
			designator: "SubkindOfObtains",
			wantCoordinates: []string{
				"admissibility_first", "criterion_entailment_branch",
				"exhaustive_closed_finite_domain_branch", "preorder", "reflexive",
				"transitive", "mutual_facts_classification_equivalence",
				"distinct_kind_identity_preserved",
				"optional_partial_order_over_equivalence_groups",
				"separate_relation_predicate_assertion",
			},
		},
		{
			sourceID:   "C.3.2:5",
			owner:      "C.3.2",
			kind:       C3KindSignatureContract,
			designator: "KindSignature",
			wantCoordinates: []string{
				"kind_entity_of_concern",
				"candidate_value_kind_or_exact_value_interpretation",
				"membership_condition", "context_slice_applicability",
				"effective_reference_scheme",
				"assumptions_dependencies_standards_versions_units_temporal_policy",
				"formality", "optional_extent_rule", "pre_judgement_not_applicable",
			},
		},
		{
			sourceID:   "C.3.2:6",
			owner:      "C.3.2",
			kind:       C3KindClassificationContract,
			designator: "ClassificationAdmissibility/J",
			wantCoordinates: []string{
				"candidate", "kind", "kind_signature_edition", "context_slice",
				"admissible", "not_applicable_no_judgement", "true", "false",
				"unknown", "governed_condition",
				"condition_separate_from_evidentiary_use",
				"guard_disposition_separate",
			},
		},
		{
			sourceID:   "C.3.2:7",
			owner:      "C.3.2",
			kind:       C3KindExtensionContract,
			designator: "KindExtension",
			wantCoordinates: []string{
				"kind", "kind_signature_edition", "context_slice", "candidate_domain",
				"admissible_true_candidates_only",
				"unknown_and_not_applicable_exclusions_distinct",
				"representation_not_collection_membership_relation_or_condition",
				"named_receiving_use",
			},
		},
		{
			sourceID:   "C.3.3:5",
			owner:      "C.3.3",
			kind:       C3KindBridgeContract,
			designator: "KindBridge",
			wantCoordinates: []string{
				"source_kind", "target_kind", "distinct_kinds",
				"directional_correspondence_predicate", "definedness",
				"participant_determined_occurrence_identity",
				"scheme_and_signature_qualifiers", "separate_bridge_assertion",
				"receiving_admissibility", "fresh_receiving_judgement",
				"source_judgement_not_receiving_truth", "r_only_reliance_consequence",
			},
		},
		{
			sourceID:   "C.3.4:5",
			owner:      "C.3.4",
			kind:       C3KindUseAdaptationContract,
			designator: "KindUseAdaptationDeclaration",
			wantCoordinates: []string{
				"base_kind", "base_kind_signature_edition", "receiving_use",
				"adaptation_type", "directly_governed_candidate_conditions",
				"vocabulary_or_notation_bindings", "candidate_and_slice_applicability",
				"dependencies", "scope_expectations_separate", "intended_guard_use",
				"formality", "adaptation_admissibility", "true_false_unknown",
				"vocabulary_only_preserves_base_judgement", "no_new_kind_or_bridge",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.sourceID, func(t *testing.T) {
			profile := currentC3ProfileForSourceID(t, test.sourceID)
			if profile.kind != test.kind || profile.designator != test.designator ||
				!slices.Equal(profile.coordinates, test.wantCoordinates) {
				t.Fatalf("current profile = %#v, want kind %s designator %q coordinates %#v", profile, test.kind, test.designator, test.wantCoordinates)
			}
			unit := fpf.SourceUnit{
				UnitID:          "fixture:current-c3:" + test.sourceID,
				SourceID:        test.sourceID,
				Role:            fpf.SourceUnitRolePatternSection,
				ParentPatternID: test.owner,
				Body:            strings.Join(profile.required, "\n"),
			}
			parsed, ok := ParseStructuralUnit(unit).(GrammarParsed)
			if !ok {
				t.Fatalf("current profile parse = %T, want GrammarParsed", ParseStructuralUnit(unit))
			}
			declaration := findC3Contract(t, parsed.Declarations())
			if declaration.Kind() != test.kind || declaration.Designator() != test.designator ||
				!slices.Equal(declaration.Coordinates(), test.wantCoordinates) {
				t.Fatalf("current declaration = %#v", declaration)
			}
			for index, cue := range profile.required {
				mutated := unit
				mutated.Body = strings.Replace(mutated.Body, cue, "removed semantic cue", 1)
				malformed, ok := ParseStructuralUnit(mutated).(GrammarMalformed)
				if !ok || malformed.Diagnostics()[0].Code() != "current_c3_contract_malformed" {
					t.Fatalf("missing cue[%d] %q = %#v", index, cue, ParseStructuralUnit(mutated))
				}
			}
		})
	}
}

func currentC3ProfileForSourceID(t *testing.T, sourceID string) c3ContractGrammarProfile {
	t.Helper()
	for _, spec := range currentC3ContractGrammarSpecs() {
		if spec.sourceID == sourceID && len(spec.profiles) > 0 {
			return spec.profiles[0]
		}
	}
	t.Fatalf("current C.3 source profile %s not found", sourceID)
	return c3ContractGrammarProfile{}
}

func TestCurrentC3ContractSpecsDeclareProfiles(t *testing.T) {
	for _, spec := range currentC3ContractGrammarSpecs() {
		if len(spec.profiles) == 0 {
			t.Errorf("current C.3 source profile %s has no supported semantic profile", spec.sourceID)
		}
	}
}

func TestCurrentC3ContractRejectsMultipleMatchingProfiles(t *testing.T) {
	var target c3ContractGrammarSpec
	for _, spec := range currentC3ContractGrammarSpecs() {
		if spec.sourceID == "C.3.4:5" {
			target = spec
			break
		}
	}
	if len(target.profiles) < 2 {
		t.Fatalf("C.3.4:5 profiles = %d, want at least 2", len(target.profiles))
	}

	required := append([]string(nil), target.profiles[0].required...)
	required = append(required, target.profiles[1].required...)
	unit := fpf.SourceUnit{
		UnitID:          "fixture:current-c3:multiple-profiles",
		SourceID:        target.sourceID,
		Role:            fpf.SourceUnitRolePatternSection,
		ParentPatternID: target.owner,
		Body:            strings.Join(required, "\n"),
	}
	outcome := ParseStructuralUnit(unit)
	malformed, ok := outcome.(GrammarMalformed)
	if !ok || malformed.Diagnostics()[0].Code() != "current_c3_contract_malformed" {
		t.Fatalf("multiple matching profiles = %#v, want current_c3_contract_malformed", outcome)
	}
}

func TestPinnedA65RuleSetIsExactAndMultiline(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	unit := resolveGrammarSourceID(t, snapshot, "A.6.5:4.3")
	parsed := ParseStructuralUnit(unit).(GrammarParsed)
	want := acceptedSlotRuleLabels()
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
	for ruleID, labels := range want {
		if !slices.Contains(labels, got[ruleID]) {
			t.Fatalf("rule %s label = %q, want one of %q", ruleID, got[ruleID], labels)
		}
	}
}

func TestRecognizedRuleSetAcceptsOnlyKnownS5Editions(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	unit := resolveGrammarSourceID(t, snapshot, "A.6.5:4.3")
	current := "A6.5-S5 DirectPredicateDefinition:"
	if strings.Count(unit.Body, current) != 1 {
		t.Fatalf("pinned source contains %d copies of %q, want 1", strings.Count(unit.Body, current), current)
	}

	legacy := unit
	legacy.Body = strings.Replace(
		legacy.Body,
		current,
		"A6.5-S5 DirectPredicateGovernance:",
		1,
	)
	parsed, ok := ParseStructuralUnit(legacy).(GrammarParsed)
	if !ok {
		t.Fatalf("legacy S5 edition = %T, want GrammarParsed", ParseStructuralUnit(legacy))
	}
	gotLabel := ""
	for _, declaration := range parsed.Declarations() {
		rule, isRule := declaration.(SlotRuleDeclaration)
		if isRule && rule.RuleID() == "A6.5-S5" {
			gotLabel = rule.Label()
		}
	}
	if gotLabel != "DirectPredicateGovernance" {
		t.Fatalf("legacy S5 label = %q", gotLabel)
	}

	unknown := unit
	unknown.Body = strings.Replace(
		unknown.Body,
		current,
		"A6.5-S5 DirectPredicateAlias:",
		1,
	)
	malformed, ok := ParseStructuralUnit(unknown).(GrammarMalformed)
	if !ok || malformed.Diagnostics()[0].Code() != "slot_rule_set_mismatch" {
		t.Fatalf("unknown S5 edition = %#v, want slot_rule_set_mismatch", ParseStructuralUnit(unknown))
	}
}

func TestEditionRelationSemanticProfilesRejectIncompleteAndHybridSources(t *testing.T) {
	snapshot := loadPinnedGrammarSnapshot(t)
	legacy := resolveGrammarSourceID(t, snapshot, "C.2.1:4.5")
	legacyPredicate := "The relation obtains when the two epistemes have different C.2.1 identities and one exact system performed revision, refinement, or supersession work under a method whose semantics establish historical continuation."
	legacyIdentity := "One occurrence is participant-determined by the exact `<earlier episteme, later episteme>` pair."
	legacyNonDuplication := "Two work occurrences that establish the same historical continuation do not create two edition-relation occurrences."
	candidatePredicate := strings.Join([]string{
		"The relation obtains only when all of these conditions hold:",
		"",
		"1. the two epistemes have different C.2.1 identities;",
		"2. the later episteme actually uses the earlier episteme as the source for the claimed revision, refinement, or supersession;",
		"3. one applicable edition-continuity policy or rule states which claim, EntityOfConcern, and effective-reference-scheme features must be preserved, which may deliberately change, and what counts as continuation for this episteme family;",
		"4. the exact preserved and deliberately changed features satisfy that rule;",
		"5. no failure condition in that rule classifies the case as a fork, translation, retargeting, or independent reconstruction instead.",
	}, "\n")
	candidateIdentity := "One occurrence is identified by the exact `<earlier episteme, later episteme>` pair."
	candidateNonDuplication := "Two revision Work occurrences do not create two edition occurrences for the same pair."

	candidate := legacy
	candidate.Body = strings.Replace(candidate.Body, legacyPredicate, candidatePredicate, 1)
	candidate.Body = strings.Replace(candidate.Body, legacyIdentity, candidateIdentity, 1)
	candidate.Body = strings.Replace(candidate.Body, legacyNonDuplication, candidateNonDuplication, 1)
	if _, ok := ParseStructuralUnit(candidate).(GrammarParsed); !ok {
		t.Fatalf("complete candidate edition profile = %T, want GrammarParsed", ParseStructuralUnit(candidate))
	}

	for _, witness := range []string{
		"the two epistemes have different C.2.1 identities;",
		"the later episteme actually uses the earlier episteme as the source for the claimed revision, refinement, or supersession;",
		"one applicable edition-continuity policy or rule states which claim, EntityOfConcern, and effective-reference-scheme features must be preserved, which may deliberately change, and what counts as continuation for this episteme family;",
		"the exact preserved and deliberately changed features satisfy that rule;",
		"no failure condition in that rule classifies the case as a fork, translation, retargeting, or independent reconstruction instead.",
		candidateIdentity,
		candidateNonDuplication,
	} {
		t.Run(witness, func(t *testing.T) {
			mutated := candidate
			mutated.Body = strings.Replace(mutated.Body, witness, "removed candidate semantic witness", 1)
			malformed, ok := ParseStructuralUnit(mutated).(GrammarMalformed)
			if !ok || malformed.Diagnostics()[0].Code() != "relation_semantics_source_malformed" {
				t.Fatalf("missing candidate witness = %#v", ParseStructuralUnit(mutated))
			}
		})
	}

	hybrid := candidate
	hybrid.Body = strings.Replace(hybrid.Body, candidateIdentity, legacyIdentity, 1)
	if _, ok := ParseStructuralUnit(hybrid).(GrammarMalformed); !ok {
		t.Fatalf("hybrid edition profile = %T, want GrammarMalformed", ParseStructuralUnit(hybrid))
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

func findC3Contract(
	t *testing.T,
	declarations []StructuralDeclaration,
) C3ContractDeclaration {
	t.Helper()
	for _, declaration := range declarations {
		if contract, ok := declaration.(C3ContractDeclaration); ok {
			return contract
		}
	}
	t.Fatal("parsed declarations contain no C.3 source contract")
	return C3ContractDeclaration{}
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
