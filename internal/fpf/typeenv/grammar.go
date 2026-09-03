package typeenv

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
)

type GrammarOutcome interface {
	UnitID() string
	grammarOutcomeVariant()
}

type GrammarNoMatch struct {
	unitID string
}

func (outcome GrammarNoMatch) UnitID() string { return outcome.unitID }

func (GrammarNoMatch) grammarOutcomeVariant() {}

type GrammarParsed struct {
	unitID       string
	declarations []StructuralDeclaration
}

func (outcome GrammarParsed) UnitID() string { return outcome.unitID }

func (outcome GrammarParsed) Declarations() []StructuralDeclaration {
	return append([]StructuralDeclaration(nil), outcome.declarations...)
}

func (GrammarParsed) grammarOutcomeVariant() {}

type GrammarMalformed struct {
	unitID      string
	diagnostics []CompilerDiagnostic
}

func (outcome GrammarMalformed) UnitID() string { return outcome.unitID }

func (outcome GrammarMalformed) Diagnostics() []CompilerDiagnostic {
	return append([]CompilerDiagnostic(nil), outcome.diagnostics...)
}

func (GrammarMalformed) grammarOutcomeVariant() {}

type CompilerDiagnostic struct {
	code    string
	unitID  string
	message string
}

func NewCompilerDiagnostic(code, unitID, message string) (CompilerDiagnostic, error) {
	values := []struct {
		label string
		value string
	}{
		{label: "diagnostic code", value: code},
		{label: "diagnostic source unit", value: unitID},
		{label: "diagnostic message", value: message},
	}
	for _, candidate := range values {
		if strings.TrimSpace(candidate.value) == "" {
			return CompilerDiagnostic{}, fmt.Errorf("%s is required", candidate.label)
		}
	}
	return CompilerDiagnostic{
		code:    strings.TrimSpace(code),
		unitID:  strings.TrimSpace(unitID),
		message: strings.TrimSpace(message),
	}, nil
}

func (diagnostic CompilerDiagnostic) Code() string { return diagnostic.code }

func (diagnostic CompilerDiagnostic) UnitID() string { return diagnostic.unitID }

func (diagnostic CompilerDiagnostic) Message() string { return diagnostic.message }

type StructuralDeclaration interface {
	Source() fpf.SourceUnit
	structuralDeclarationVariant()
}

type SlotSpecProductionDeclaration struct {
	source fpf.SourceUnit
}

func (declaration SlotSpecProductionDeclaration) Source() fpf.SourceUnit {
	return declaration.source
}

func (SlotSpecProductionDeclaration) structuralDeclarationVariant() {}

type SlotRuleDeclaration struct {
	source    fpf.SourceUnit
	ruleID    string
	label     string
	statement string
}

func (declaration SlotRuleDeclaration) Source() fpf.SourceUnit { return declaration.source }

func (declaration SlotRuleDeclaration) RuleID() string { return declaration.ruleID }

func (declaration SlotRuleDeclaration) Label() string { return declaration.label }

func (declaration SlotRuleDeclaration) Statement() string { return declaration.statement }

func (SlotRuleDeclaration) structuralDeclarationVariant() {}

type RelationRootDeclaration struct {
	source      fpf.SourceUnit
	owner       string
	subjectKind string
	relation    string
}

func (declaration RelationRootDeclaration) Source() fpf.SourceUnit { return declaration.source }

func (declaration RelationRootDeclaration) OwnerPatternID() string { return declaration.owner }

func (declaration RelationRootDeclaration) SubjectKind() string { return declaration.subjectKind }

func (declaration RelationRootDeclaration) RelationName() string { return declaration.relation }

func (RelationRootDeclaration) structuralDeclarationVariant() {}

type ReferenceModeEvidence interface {
	String() string
	referenceModeEvidenceVariant()
}

type MissingReferenceModeEvidence struct{}

func (MissingReferenceModeEvidence) String() string { return "missing" }

func (MissingReferenceModeEvidence) referenceModeEvidenceVariant() {}

type ByValueEvidence struct{}

func (ByValueEvidence) String() string { return "by_value" }

func (ByValueEvidence) referenceModeEvidenceVariant() {}

type ByReferenceEvidence struct {
	refKind string
}

func (evidence ByReferenceEvidence) String() string { return "by_reference:" + evidence.refKind }

func (evidence ByReferenceEvidence) RefKind() string { return evidence.refKind }

func (ByReferenceEvidence) referenceModeEvidenceVariant() {}

type CardinalityEvidence interface {
	Minimum() (uint64, bool)
	Maximum() (uint64, bool)
	String() string
	cardinalityEvidenceVariant()
}

type MissingCardinalityEvidence struct{}

func (MissingCardinalityEvidence) Minimum() (uint64, bool) { return 0, false }

func (MissingCardinalityEvidence) Maximum() (uint64, bool) { return 0, false }

func (MissingCardinalityEvidence) String() string { return "missing" }

func (MissingCardinalityEvidence) cardinalityEvidenceVariant() {}

type BoundedCardinalityEvidence struct {
	minimum uint64
	maximum uint64
}

func (evidence BoundedCardinalityEvidence) Minimum() (uint64, bool) {
	return evidence.minimum, true
}

func (evidence BoundedCardinalityEvidence) Maximum() (uint64, bool) {
	return evidence.maximum, true
}

func (evidence BoundedCardinalityEvidence) String() string {
	return fmt.Sprintf("%d..%d", evidence.minimum, evidence.maximum)
}

func (BoundedCardinalityEvidence) cardinalityEvidenceVariant() {}

type UnboundedCardinalityEvidence struct {
	minimum uint64
}

func (evidence UnboundedCardinalityEvidence) Minimum() (uint64, bool) {
	return evidence.minimum, true
}

func (UnboundedCardinalityEvidence) Maximum() (uint64, bool) { return 0, false }

func (evidence UnboundedCardinalityEvidence) String() string {
	return fmt.Sprintf("%d..*", evidence.minimum)
}

func (UnboundedCardinalityEvidence) cardinalityEvidenceVariant() {}

type SlotDeclarationFragment struct {
	source      fpf.SourceUnit
	owner       string
	slotKind    string
	valueKind   string
	reference   ReferenceModeEvidence
	cardinality CardinalityEvidence
}

func (declaration SlotDeclarationFragment) Source() fpf.SourceUnit { return declaration.source }

func (declaration SlotDeclarationFragment) OwnerPatternID() string { return declaration.owner }

func (declaration SlotDeclarationFragment) SlotKind() string { return declaration.slotKind }

func (declaration SlotDeclarationFragment) ValueKind() string { return declaration.valueKind }

func (declaration SlotDeclarationFragment) ReferenceMode() ReferenceModeEvidence {
	return declaration.reference
}

func (declaration SlotDeclarationFragment) Cardinality() CardinalityEvidence {
	return declaration.cardinality
}

func (SlotDeclarationFragment) structuralDeclarationVariant() {}

type SlotCardinalityRequirement struct {
	slotKind    string
	cardinality CardinalityEvidence
}

func (requirement SlotCardinalityRequirement) SlotKind() string { return requirement.slotKind }

func (requirement SlotCardinalityRequirement) Cardinality() CardinalityEvidence {
	return requirement.cardinality
}

type RelationProfileDeclaration struct {
	source       fpf.SourceUnit
	owner        string
	requirements []SlotCardinalityRequirement
}

func (declaration RelationProfileDeclaration) Source() fpf.SourceUnit { return declaration.source }

func (declaration RelationProfileDeclaration) OwnerPatternID() string { return declaration.owner }

func (declaration RelationProfileDeclaration) Requirements() []SlotCardinalityRequirement {
	return append([]SlotCardinalityRequirement(nil), declaration.requirements...)
}

func (RelationProfileDeclaration) structuralDeclarationVariant() {}

// SymbolicRelationSlotSpec is source-derived declaration content. It is not a
// runtime SlotKind and carries no inferred cardinality: the current A.6.5
// grammar leaves occurrence and participant cardinality with the direct
// relation pattern.
type SymbolicRelationSlotSpec struct {
	slotKind           string
	participantMeaning string
	valueKind          string
	reference          ReferenceModeEvidence
}

func (slot SymbolicRelationSlotSpec) SlotKind() string { return slot.slotKind }

func (slot SymbolicRelationSlotSpec) ParticipantMeaning() string {
	return slot.participantMeaning
}

func (slot SymbolicRelationSlotSpec) ValueKind() string { return slot.valueKind }

func (slot SymbolicRelationSlotSpec) ReferenceMode() ReferenceModeEvidence {
	return slot.reference
}

// SymbolicRelationSignatureDeclaration preserves the directly authored C.2.1
// declaration table without claiming that the runtime can evaluate the direct
// predicate, applicability, or occurrence-identity rule.
type SymbolicRelationSignatureDeclaration struct {
	source        fpf.SourceUnit
	owner         string
	relationName  string
	signatureName string
	slots         []SymbolicRelationSlotSpec
}

func (declaration SymbolicRelationSignatureDeclaration) Source() fpf.SourceUnit {
	return declaration.source
}

func (declaration SymbolicRelationSignatureDeclaration) OwnerPatternID() string {
	return declaration.owner
}

func (declaration SymbolicRelationSignatureDeclaration) RelationName() string {
	return declaration.relationName
}

func (declaration SymbolicRelationSignatureDeclaration) SignatureName() string {
	return declaration.signatureName
}

func (declaration SymbolicRelationSignatureDeclaration) Slots() []SymbolicRelationSlotSpec {
	return append([]SymbolicRelationSlotSpec(nil), declaration.slots...)
}

func (SymbolicRelationSignatureDeclaration) structuralDeclarationVariant() {}

// SymbolicRelationSemanticsDeclaration points at the direct source span that
// states obtaining and occurrence identity. The compiler preserves the span,
// but deliberately does not turn prose into an executable evaluator.
type SymbolicRelationSemanticsDeclaration struct {
	source        fpf.SourceUnit
	owner         string
	relationName  string
	signatureName string
}

func (declaration SymbolicRelationSemanticsDeclaration) Source() fpf.SourceUnit {
	return declaration.source
}

func (declaration SymbolicRelationSemanticsDeclaration) OwnerPatternID() string {
	return declaration.owner
}

func (declaration SymbolicRelationSemanticsDeclaration) RelationName() string {
	return declaration.relationName
}

func (declaration SymbolicRelationSemanticsDeclaration) SignatureName() string {
	return declaration.signatureName
}

func (SymbolicRelationSemanticsDeclaration) structuralDeclarationVariant() {}

// C3ContractKind is the closed set of source-native C.3 declaration families
// that Haft keeps separately recoverable. These are source contracts, not
// project-local declarations, relation occurrences, or runtime results.
type C3ContractKind uint8

const (
	C3SubkindRelationContract C3ContractKind = iota + 1
	C3SubkindOrderContract
	C3KindSignatureContract
	C3KindClassificationContract
	C3KindExtensionContract
	C3KindBridgeContract
	C3KindUseAdaptationContract
	C3KindGuardSeparationContract
)

func (kind C3ContractKind) String() string {
	switch kind {
	case C3SubkindRelationContract:
		return "subkind_relation"
	case C3SubkindOrderContract:
		return "subkind_order"
	case C3KindSignatureContract:
		return "kind_signature"
	case C3KindClassificationContract:
		return "kind_classification"
	case C3KindExtensionContract:
		return "kind_extension"
	case C3KindBridgeContract:
		return "kind_bridge"
	case C3KindUseAdaptationContract:
		return "kind_use_adaptation"
	case C3KindGuardSeparationContract:
		return "kind_guard_separation"
	default:
		return ""
	}
}

// C3ContractDeclaration retains the exact source unit and the authored
// semantic coordinates recognized in that unit. The linked source contract
// remains source-only until a project-local declaration supplies executable
// content under an exact TypeEnv.
type C3ContractDeclaration struct {
	source      fpf.SourceUnit
	kind        C3ContractKind
	designator  string
	coordinates []string
}

func (declaration C3ContractDeclaration) Source() fpf.SourceUnit {
	return declaration.source
}

func (declaration C3ContractDeclaration) Kind() C3ContractKind {
	return declaration.kind
}

func (declaration C3ContractDeclaration) Designator() string {
	return declaration.designator
}

func (declaration C3ContractDeclaration) Coordinates() []string {
	return append([]string(nil), declaration.coordinates...)
}

func (C3ContractDeclaration) structuralDeclarationVariant() {}

var currentSlotRuleMarkerRE = regexp.MustCompile(`(?m)^A6\.5-S[0-9]+\s+[A-Za-z0-9]+:`)
var currentSlotRuleRE = regexp.MustCompile(
	`(?m)^(A6\.5-S[0-9]+)\s+([A-Za-z0-9]+):\n((?:  [^\n]+(?:\n|$))+)`,
)
var currentRelationSlotRowRE = regexp.MustCompile(
	"(?m)^\\|\\s*`([^`]+Slot)`\\s*\\|\\s*([^|]+?)\\s*\\|\\s*`([^`]+)`\\s*\\|\\s*`([^`]+)`\\s*\\|$",
)

const exactSlotSpecProduction = "```text\nSlotSpec := <SlotKind, ValueKind, refMode>\nrefMode := ByValue | RefKind\n```"

func ParseStructuralUnit(unit fpf.SourceUnit) GrammarOutcome {
	if unit.Role != fpf.SourceUnitRolePatternSection {
		return GrammarNoMatch{unitID: unit.UnitID}
	}

	adapters := []func(fpf.SourceUnit) GrammarOutcome{
		parseSlotSpecProduction,
		parseSlotRules,
		parseCurrentRelationSignature,
		parseCurrentRelationSemantics,
		parseCurrentC3Contract,
	}
	declarations := make([]StructuralDeclaration, 0)
	diagnostics := make([]CompilerDiagnostic, 0)
	for _, adapter := range adapters {
		outcome := adapter(unit)
		switch parsed := outcome.(type) {
		case GrammarNoMatch:
			continue
		case GrammarParsed:
			declarations = append(declarations, parsed.Declarations()...)
		case GrammarMalformed:
			diagnostics = append(diagnostics, parsed.Diagnostics()...)
		}
	}
	if len(diagnostics) > 0 {
		return GrammarMalformed{unitID: unit.UnitID, diagnostics: diagnostics}
	}
	if len(declarations) > 0 {
		return GrammarParsed{unitID: unit.UnitID, declarations: declarations}
	}
	return GrammarNoMatch{unitID: unit.UnitID}
}

func parseSlotSpecProduction(unit fpf.SourceUnit) GrammarOutcome {
	if !isExactStructuralSection(unit, "A.6.5", "A.6.5:4.2") {
		return GrammarNoMatch{unitID: unit.UnitID}
	}
	productionCount := strings.Count(unit.Body, "SlotSpec :=")
	if productionCount == 0 {
		return GrammarNoMatch{unitID: unit.UnitID}
	}
	refModeCount := strings.Count(unit.Body, "refMode :=")
	if productionCount == 1 &&
		refModeCount == 1 &&
		strings.Count(unit.Body, exactSlotSpecProduction) == 1 {
		declaration := SlotSpecProductionDeclaration{source: unit}
		return GrammarParsed{unitID: unit.UnitID, declarations: []StructuralDeclaration{declaration}}
	}
	return malformedGrammar(
		unit,
		"slot_spec_production_malformed",
		"recognized SlotSpec production must declare exactly SlotKind, ValueKind, and refMode with ByValue or RefKind",
	)
}

func parseSlotRules(unit fpf.SourceUnit) GrammarOutcome {
	if !isExactStructuralSection(unit, "A.6.5", "A.6.5:4.3") {
		return GrammarNoMatch{unitID: unit.UnitID}
	}
	count := len(currentSlotRuleMarkerRE.FindAllStringIndex(unit.Body, -1))
	if count == 0 {
		return malformedGrammar(
			unit,
			"slot_rule_set_missing",
			"recognized A.6.5 rule section contains no labeled SlotSpec rules",
		)
	}
	matches := currentSlotRuleRE.FindAllStringSubmatch(unit.Body, -1)
	if len(matches) != count {
		return malformedGrammar(
			unit,
			"slot_rule_malformed",
			"recognized SlotSpec well-formedness constraint has unknown syntax",
		)
	}
	declarations := make([]StructuralDeclaration, 0, len(matches))
	expected := acceptedSlotRuleLabels()
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		ruleID := strings.TrimSpace(match[1])
		label := strings.TrimSpace(match[2])
		acceptedLabels, known := expected[ruleID]
		_, duplicate := seen[ruleID]
		if !known || duplicate || !slices.Contains(acceptedLabels, label) {
			return malformedGrammar(
				unit,
				"slot_rule_set_mismatch",
				"recognized A.6.5 rule section must contain the exact A6.5-S1 through A6.5-S7 labels once each",
			)
		}
		seen[ruleID] = struct{}{}
		declarations = append(declarations, SlotRuleDeclaration{
			source:    unit,
			ruleID:    ruleID,
			label:     label,
			statement: strings.Join(strings.Fields(match[3]), " "),
		})
	}
	if len(seen) != len(expected) {
		return malformedGrammar(
			unit,
			"slot_rule_set_mismatch",
			"recognized A.6.5 rule section must contain all seven current rules",
		)
	}
	return GrammarParsed{unitID: unit.UnitID, declarations: declarations}
}

type currentRelationGrammarSpec struct {
	signatureSourceID string
	semanticsSourceID string
	relationName      string
	signatureName     string
	slots             []currentRelationSlotSpec
	semanticProfiles  [][]string
}

type currentRelationSlotSpec struct {
	slotKind  string
	valueKind string
	refMode   string
}

func currentRelationGrammarSpecs() []currentRelationGrammarSpec {
	return []currentRelationGrammarSpec{
		{
			signatureSourceID: "C.2.1:4.2.1",
			semanticsSourceID: "C.2.1:4.2.2",
			relationName:      "EpistemeConstitutionRelation",
			signatureName:     "EpistemeConstitutionRelationSignature",
			slots: []currentRelationSlotSpec{
				{slotKind: "ClaimGraphSlot", valueKind: "U.ClaimGraph", refMode: "ByValue"},
				{slotKind: "EntityOfConcernSlot", valueKind: "U.Entity", refMode: "U.EntityRef"},
				{slotKind: "ReferenceSchemeSlot", valueKind: "U.ReferenceScheme", refMode: "ByValue"},
			},
			semanticProfiles: [][]string{{
				"`EpistemeConstitutionRelation` obtains exactly when",
				"relation occurrence is participant-determined",
			}},
		},
		{
			signatureSourceID: "C.2.1:4.3",
			semanticsSourceID: "C.2.1:4.3",
			relationName:      "EpistemeEmpiricalGroundingRelation",
			signatureName:     "EpistemeEmpiricalGroundingRelationSignature",
			slots: []currentRelationSlotSpec{
				{slotKind: "GroundedEpistemeSlot", valueKind: "U.Episteme", refMode: "U.EpistemeRef"},
				{slotKind: "GroundingHolonSlot", valueKind: "U.Holon", refMode: "U.HolonRef"},
			},
			semanticProfiles: [][]string{{
				"`EpistemeEmpiricalGroundingRelation` over participants `(E,H)`",
				"with `covered=C`",
				"obtains exactly while every empirical claim",
				"One occurrence is identified by `<episteme, exact covered claim subgraph, grounding holon,",
				"maximal continuous interval during which the complete coverage predicate is true>`",
			}},
		},
		{
			signatureSourceID: "C.2.1:4.5",
			semanticsSourceID: "C.2.1:4.5",
			relationName:      "EpistemeEditionRelation",
			signatureName:     "EpistemeEditionRelationSignature",
			slots: []currentRelationSlotSpec{
				{slotKind: "EarlierEpistemeSlot", valueKind: "U.Episteme", refMode: "U.EpistemeRef"},
				{slotKind: "LaterEpistemeSlot", valueKind: "U.Episteme", refMode: "U.EpistemeRef"},
			},
			semanticProfiles: [][]string{
				{
					"The relation obtains when the two epistemes have different C.2.1 identities and one exact system performed revision, refinement, or supersession work under a method whose semantics establish historical continuation.",
					"One occurrence is participant-determined by the exact `<earlier episteme, later episteme>` pair.",
					"Two work occurrences that establish the same historical continuation do not create two edition-relation occurrences.",
				},
				{
					"The relation obtains only when all of these conditions hold:",
					"the two epistemes have different C.2.1 identities;",
					"the later episteme actually uses the earlier episteme as the source for the claimed revision, refinement, or supersession;",
					"one applicable edition-continuity policy or rule states which claim, EntityOfConcern, and effective-reference-scheme features must be preserved, which may deliberately change, and what counts as continuation for this episteme family;",
					"the exact preserved and deliberately changed features satisfy that rule;",
					"no failure condition in that rule classifies the case as a fork, translation, retargeting, or independent reconstruction instead.",
					"One occurrence is identified by the exact `<earlier episteme, later episteme>` pair.",
					"Two revision Work occurrences do not create two edition occurrences for the same pair.",
				},
			},
		},
	}
}

func parseCurrentRelationSignature(unit fpf.SourceUnit) GrammarOutcome {
	if unit.ParentPatternID != "C.2.1" {
		return GrammarNoMatch{unitID: unit.UnitID}
	}
	for _, spec := range currentRelationGrammarSpecs() {
		if unit.SourceID != spec.signatureSourceID {
			continue
		}
		return parseExpectedRelationSignature(unit, spec)
	}
	if strings.Contains(unit.Body, "| SlotKind | Relation-participant meaning | ValueKind | refMode |") {
		return malformedGrammar(
			unit,
			"relation_signature_source_unrecognized",
			"C.2.1 contains a relation-signature table outside the direct adapter-v3 source inventory",
		)
	}
	return GrammarNoMatch{unitID: unit.UnitID}
}

func parseExpectedRelationSignature(
	unit fpf.SourceUnit,
	spec currentRelationGrammarSpec,
) GrammarOutcome {
	if !strings.Contains(unit.Body, "`"+spec.relationName+"`") ||
		!strings.Contains(unit.Body, "`"+spec.signatureName+"`") {
		return malformedGrammar(
			unit,
			"relation_signature_identity_malformed",
			"recognized C.2.1 relation declaration does not name its exact relation and signature episteme",
		)
	}
	matches := currentRelationSlotRowRE.FindAllStringSubmatch(unit.Body, -1)
	if len(matches) != len(spec.slots) {
		return malformedGrammar(
			unit,
			"relation_signature_slot_table_malformed",
			"recognized C.2.1 relation declaration has an incomplete or unknown SlotSpec table",
		)
	}
	expected := make(map[string]currentRelationSlotSpec, len(spec.slots))
	for _, slot := range spec.slots {
		expected[slot.slotKind] = slot
	}
	slots := make([]SymbolicRelationSlotSpec, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		slotKind := strings.TrimSpace(match[1])
		participantMeaning := strings.TrimSpace(match[2])
		valueKind := strings.TrimSpace(match[3])
		refMode := strings.TrimSpace(match[4])
		want, exists := expected[slotKind]
		_, duplicate := seen[slotKind]
		if !exists || duplicate || valueKind != want.valueKind || refMode != want.refMode {
			return malformedGrammar(
				unit,
				"relation_signature_slot_table_mismatch",
				"recognized C.2.1 relation declaration must preserve its exact SlotKind, ValueKind, and refMode rows",
			)
		}
		reference, err := parseTableReferenceMode(refMode)
		if err != nil {
			return malformedGrammar(
				unit,
				"relation_signature_ref_mode_malformed",
				"recognized C.2.1 relation declaration has an unsupported refMode",
			)
		}
		seen[slotKind] = struct{}{}
		slots = append(slots, SymbolicRelationSlotSpec{
			slotKind:           slotKind,
			participantMeaning: participantMeaning,
			valueKind:          valueKind,
			reference:          reference,
		})
	}
	declaration := SymbolicRelationSignatureDeclaration{
		source:        unit,
		owner:         unit.ParentPatternID,
		relationName:  spec.relationName,
		signatureName: spec.signatureName,
		slots:         slots,
	}
	return GrammarParsed{
		unitID:       unit.UnitID,
		declarations: []StructuralDeclaration{declaration},
	}
}

func parseTableReferenceMode(raw string) (ReferenceModeEvidence, error) {
	value := strings.TrimSpace(raw)
	if value == "ByValue" {
		return ByValueEvidence{}, nil
	}
	if strings.HasSuffix(value, "Ref") && strings.Contains(value, ".") {
		return ByReferenceEvidence{refKind: value}, nil
	}
	return MissingReferenceModeEvidence{}, fmt.Errorf("unknown refMode %q", raw)
}

func parseCurrentRelationSemantics(unit fpf.SourceUnit) GrammarOutcome {
	if unit.ParentPatternID != "C.2.1" {
		return GrammarNoMatch{unitID: unit.UnitID}
	}
	for _, spec := range currentRelationGrammarSpecs() {
		if unit.SourceID != spec.semanticsSourceID {
			continue
		}
		if !matchesCompleteWitnessProfile(unit.Body, spec.semanticProfiles) {
			return malformedGrammar(
				unit,
				"relation_semantics_source_malformed",
				"recognized C.2.1 relation semantics must retain direct obtaining and occurrence-identity statements",
			)
		}
		declaration := SymbolicRelationSemanticsDeclaration{
			source:        unit,
			owner:         unit.ParentPatternID,
			relationName:  spec.relationName,
			signatureName: spec.signatureName,
		}
		return GrammarParsed{
			unitID:       unit.UnitID,
			declarations: []StructuralDeclaration{declaration},
		}
	}
	return GrammarNoMatch{unitID: unit.UnitID}
}

func matchesCompleteWitnessProfile(body string, profiles [][]string) bool {
	return slices.ContainsFunc(profiles, func(profile []string) bool {
		return !slices.ContainsFunc(profile, func(witness string) bool {
			return !strings.Contains(body, witness)
		})
	})
}

func acceptedSlotRuleLabels() map[string][]string {
	return map[string][]string{
		"A6.5-S1": {"CompleteSlotSpec"},
		"A6.5-S2": {"LocalSlotKind"},
		"A6.5-S3": {"ExactParticipantKind"},
		"A6.5-S4": {"HonestReference"},
		"A6.5-S5": {"DirectPredicateGovernance", "DirectPredicateDefinition"},
		"A6.5-S6": {"NoHiddenUnion"},
		"A6.5-S7": {"RepresentationBoundary"},
	}
}

func isExactStructuralSection(unit fpf.SourceUnit, owner, sourceID string) bool {
	return unit.ParentPatternID == owner && unit.SourceID == sourceID
}

type c3ContractGrammarSpec struct {
	owner    string
	sourceID string
	profiles []c3ContractGrammarProfile
}

type c3ContractGrammarProfile struct {
	kind        C3ContractKind
	designator  string
	coordinates []string
	required    []string
}

func currentC3ContractGrammarSpecs() []c3ContractGrammarSpec {
	return []c3ContractGrammarSpec{
		{
			owner:    "C.3.1",
			sourceID: "C.3.1:4",
			profiles: []c3ContractGrammarProfile{
				{
					kind:       C3SubkindRelationContract,
					designator: "U.SubkindOf",
					coordinates: []string{
						"narrower_kind",
						"broader_kind",
						"declared_applicability",
						"SubkindOfObtains",
						"criterion_entailment_branch",
						"exhaustive_closed_finite_domain_branch",
						"participant_determined_occurrence_identity",
						"scheme_signature_applicability_qualifiers",
						"separate_c2_1_assertion_episteme",
					},
					required: []string{
						"| `U.SubkindOf` |",
						"within declared applicability",
						"`SubkindOfObtains(k1, k2)`",
						"It holds either because the exact membership criterion",
						"or because every candidate in a deliberately closed finite domain",
						"`R_sub : U.SubkindOf`",
						"The ordered kind participants determine occurrence identity; schemes, signatures, evidence, assertions, and publications do not.",
						"subkind assertion episteme",
						"The assertion does not make the relation obtain",
					},
				},
				{
					kind:       C3SubkindRelationContract,
					designator: "U.SubkindOf",
					coordinates: []string{
						"narrower_kind",
						"broader_kind",
						"effective_reference_scheme_edition",
						"SubkindOfObtains",
						"participant_and_reference_scheme_occurrence_identity",
						"separate_c2_1_assertion_episteme",
					},
					required: []string{
						"| `U.SubkindOf` |",
						"`SubkindOfObtains(k1, k2; RS)`",
						"`R_sub : U.SubkindOf`",
						"subkind assertion episteme",
						"Participant identities plus the exact effective reference-scheme edition determine its identity.",
					},
				},
			},
		},
		{
			owner:    "C.3.1",
			sourceID: "C.3.1:5",
			profiles: []c3ContractGrammarProfile{
				{
					kind:       C3SubkindOrderContract,
					designator: "SubkindOfObtains",
					coordinates: []string{
						"admissibility_first",
						"criterion_entailment_branch",
						"exhaustive_closed_finite_domain_branch",
						"preorder",
						"reflexive",
						"transitive",
						"mutual_facts_classification_equivalence",
						"distinct_kind_identity_preserved",
						"optional_partial_order_over_equivalence_groups",
						"separate_relation_predicate_assertion",
					},
					required: []string{
						"Check admissibility first.",
						"`not-applicable` forms no C.3.2 judgment.",
						"Select one obtaining branch.",
						"exact criterion entailment",
						"exhaustive evaluation only for a deliberately closed finite domain",
						"Keep a preorder over obtaining facts.",
						"Reflexivity and transitivity apply.",
						"Mutual facts between distinct kinds record classification equivalence",
						"they do not imply kind identity",
						"Use the equivalence groups only when a receiver needs a partial order.",
						"Separate relation, predicate, and assertion.",
					},
				},
				{
					kind:       C3SubkindOrderContract,
					designator: "SubkindOfObtains",
					coordinates: []string{
						"reflexive",
						"transitive",
						"antisymmetric",
						"same_candidate",
						"same_context_slice",
						"aligned_kind_signature_editions",
						"unknown_is_non_settlement",
					},
					required: []string{
						"Keep a partial order over obtaining facts.",
						"Reflexivity, transitivity, and antisymmetry",
						"same candidate and context slice",
						"`unknown` remains non-settlement",
					},
				},
			},
		},
		{
			owner:    "C.3.2",
			sourceID: "C.3.2:5",
			profiles: []c3ContractGrammarProfile{
				{
					kind:       C3KindSignatureContract,
					designator: "KindSignature",
					coordinates: []string{
						"kind_entity_of_concern",
						"candidate_value_kind_or_exact_value_interpretation",
						"membership_condition",
						"context_slice_applicability",
						"effective_reference_scheme",
						"assumptions_dependencies_standards_versions_units_temporal_policy",
						"formality",
						"optional_extent_rule",
						"pre_judgement_not_applicable",
					},
					required: []string{
						"the exact kind that is its `EntityOfConcern`",
						"the candidate `ValueKind` or exact value interpretation admitted as input",
						"the membership condition in terms of directly governed candidate qualities, relations, constructive grounding, epistemes, registrations, certifications, publications, legal statuses, or other exact conditions",
						"the exact `U.ContextSlice` applicability in which the evaluation may be formed",
						"the effective `U.ReferenceScheme`",
						"named assumptions, dependencies, standards, versions, units, and temporal policy",
						"its `U.Formality`",
						"an optional `ExtentRule` for a named extension-consuming use",
						"`not-applicable` is returned before this ranged evaluation",
					},
				},
				{
					kind:       C3KindSignatureContract,
					designator: "KindSignature",
					coordinates: []string{
						"local_kind_entity_of_concern",
						"candidate_value_kind",
						"direct_feature_criterion",
						"context_slice_conditions",
						"effective_reference_scheme",
						"assumptions_dependencies_versions_units_temporal_policy",
						"formality",
						"optional_extent_rule",
					},
					required: []string{
						"the exact local kind that is its `EntityOfConcern`",
						"the candidate `ValueKind`",
						"direct governed candidate qualities, relations, constructive grounding, or other features",
						"the exact `U.ContextSlice` conditions",
						"the effective `U.ReferenceScheme`",
						"named assumptions, dependencies, standards, versions, units, and temporal policy",
						"its `U.Formality`",
						"an optional `ExtentRule`",
					},
				},
			},
		},
		{
			owner:    "C.3.2",
			sourceID: "C.3.2:6",
			profiles: []c3ContractGrammarProfile{
				{
					kind:       C3KindClassificationContract,
					designator: "ClassificationAdmissibility/J",
					coordinates: []string{
						"candidate",
						"kind",
						"kind_signature_edition",
						"context_slice",
						"admissible",
						"not_applicable_no_judgement",
						"true",
						"false",
						"unknown",
						"governed_condition",
						"condition_separate_from_evidentiary_use",
						"guard_disposition_separate",
					},
					required: []string{
						"`A(candidate, kind, signatureEdition, slice) ∈ {admissible, not-applicable}`",
						"only when `A = admissible`",
						"`J(candidate, kind, signatureEdition, slice) ∈ {true, false, unknown}`",
						"Pin the inputs.",
						"return `not-applicable` and stop. Do not form `J`.",
						"Evaluate the governed condition.",
						"Missing support or an unavailable declared dependency gives `unknown`, not `false`.",
						"Distinguish condition from evidentiary use.",
						"Separate guard disposition.",
					},
				},
				{
					kind:       C3KindClassificationContract,
					designator: "J",
					coordinates: []string{
						"candidate",
						"local_kind",
						"kind_signature_edition",
						"context_slice",
						"true",
						"false",
						"unknown",
						"direct_features_separate_from_evidence",
						"guard_disposition_separate",
					},
					required: []string{
						"`J(candidate, kind, signatureEdition, slice) ∈ {true, false, unknown}`",
						"Pin all four inputs.",
						"Evaluate direct governed features.",
						"gives `unknown`, not `false`",
						"Separate support from satisfaction.",
						"Separate guard disposition.",
					},
				},
			},
		},
		{
			owner:    "C.3.2",
			sourceID: "C.3.2:7",
			profiles: []c3ContractGrammarProfile{
				{
					kind:       C3KindExtensionContract,
					designator: "KindExtension",
					coordinates: []string{
						"kind",
						"kind_signature_edition",
						"context_slice",
						"candidate_domain",
						"admissible_true_candidates_only",
						"unknown_and_not_applicable_exclusions_distinct",
						"representation_not_collection_membership_relation_or_condition",
						"named_receiving_use",
					},
					required: []string{
						"Materialize `KindExtension(k, slice)` only when",
						"Pin the signature edition",
						"candidate domain without inventing `U.EntitySet`",
						"Include exactly admissible candidates whose judgment is `true`.",
						"Keep `unknown` and `not-applicable` distinct",
						"They create neither a collection holon, A.14 membership occurrence, direct classification relation, nor criterion condition.",
					},
				},
				{
					kind:       C3KindExtensionContract,
					designator: "KindExtension",
					coordinates: []string{
						"local_kind",
						"kind_signature_edition",
						"context_slice",
						"declared_candidate_domain",
						"true_candidates_only",
						"named_receiving_use",
					},
					required: []string{
						"Materialize `KindExtension(k, slice)` only when",
						"Pin the `KindSignature` edition",
						"without inventing `U.EntitySet`",
						"whose pinned judgment is `true`",
						"They do not create a collection holon, an A.14 membership occurrence, a direct classification relation, or the candidate features.",
					},
				},
			},
		},
		{
			owner:    "C.3.3",
			sourceID: "C.3.3:5",
			profiles: []c3ContractGrammarProfile{
				{
					kind:       C3KindBridgeContract,
					designator: "KindBridge",
					coordinates: []string{
						"source_kind",
						"target_kind",
						"distinct_kinds",
						"directional_correspondence_predicate",
						"definedness",
						"participant_determined_occurrence_identity",
						"scheme_and_signature_qualifiers",
						"separate_bridge_assertion",
						"receiving_admissibility",
						"fresh_receiving_judgement",
						"source_judgement_not_receiving_truth",
						"r_only_reliance_consequence",
					},
					required: []string{
						"Compare kind definitions.",
						"Stop on same-kind reuse.",
						"A `KindBridge` occurrence is an obtaining direct relation between one exact source kind and one exact target kind.",
						"Its directional predicate states the correspondence and definedness",
						"paired `KindSignature` editions",
						"First return `admissible` or `not-applicable` under the receiving signature and slice.",
						"A source judgment may support the bridge assertion or reliance but is never copied as receiving truth.",
						"The kinds are the direct relation participants.",
						"They do not identify the occurrence.",
						"For the ordered kind pair, the direct relation is participant-determined.",
						"apply only the justified `CL^k` consequence to R",
					},
				},
				{
					kind:       C3KindBridgeContract,
					designator: "KindBridge",
					coordinates: []string{
						"source_local_kind",
						"target_local_kind",
						"source_reference_scheme_edition",
						"target_reference_scheme_edition",
						"direction",
						"definedness",
						"separate_bridge_assertion",
						"fresh_target_judgement",
					},
					required: []string{
						"A `KindBridge` occurrence is an obtaining direct relation between one exact source local `U.Kind` and one exact target local `U.Kind`.",
						"source and target scheme editions",
						"Keep the direct relation separate from the C.2.1 bridge-assertion episteme",
						"`J(candidate, targetKind, targetSignatureEdition, TargetSlice) ∈ {true, false, unknown}`",
						"is never reused as target truth",
					},
				},
			},
		},
		{
			owner:    "C.3.4",
			sourceID: "C.3.4:5",
			profiles: []c3ContractGrammarProfile{
				{
					kind:       C3KindUseAdaptationContract,
					designator: "KindUseAdaptationDeclaration",
					coordinates: []string{
						"base_kind",
						"base_kind_signature_edition",
						"receiving_use",
						"adaptation_type",
						"directly_governed_candidate_conditions",
						"vocabulary_or_notation_bindings",
						"candidate_and_slice_applicability",
						"dependencies",
						"scope_expectations_separate",
						"intended_guard_use",
						"formality",
						"adaptation_admissibility",
						"true_false_unknown",
						"vocabulary_only_preserves_base_judgement",
						"no_new_kind_or_bridge",
					},
					required: []string{
						"A `KindUseAdaptationDeclaration` is a named, versioned C.2.1 declaration episteme",
						"the exact base kind and pinned base `KindSignature` edition",
						"the receiving use and adaptation type: constraint, vocabulary, or composite",
						"additional directly governed candidate conditions",
						"vocabulary or notation bindings",
						"exact candidate and slice applicability plus dependencies",
						"scope expectations routed separately through A.2.6",
						"First evaluate adaptation admissibility.",
						"`J_kindUse(candidate, kind, kindSignatureEdition, adaptationDeclarationEdition, slice) ∈ {true, false, unknown}`",
						"A vocabulary-only declaration adds no predicate and preserves the base judgment.",
						"A guard may decline use on `not-applicable` or `unknown` without rewriting either.",
						"A declaration, correspondence, judgment, catalog row, or representation creates neither.",
					},
				},
				{
					kind:       C3KindUseAdaptationContract,
					designator: "RoleMask",
					coordinates: []string{
						"candidate",
						"base_local_kind",
						"kind_signature_edition",
						"role_mask_edition",
						"context_slice",
						"direct_candidate_feature_constraints",
						"scope_expectations_separate",
						"true_false_unknown",
					},
					required: []string{
						"A `RoleMask` is a named, versioned C.2.1 declaration episteme.",
						"additional direct candidate-feature predicates",
						"routed separately to USM Scope",
						"`J_mask(candidate, kind, kindSignatureEdition, roleMaskEdition, slice) ∈ {true, false, unknown}`",
						"that refusal is not a `false` classification",
					},
				},
			},
		},
		{
			owner:    "C.3.A",
			sourceID: "C.3.A:3",
			profiles: []c3ContractGrammarProfile{
				{
					kind:       C3KindGuardSeparationContract,
					designator: "GuardDisposition",
					coordinates: []string{
						"declaration_compatibility",
						"candidate_classification",
						"scope_coverage",
						"evidence_freshness",
						"bridge_applicability",
						"action_disposition",
						"true_false_unknown",
					},
					required: []string{
						"Three classification values.",
						"Separate guard disposition.",
						"Both `false` and `unknown` normally cause fail-closed refusal",
						"Scope separation.",
						"Bridge separation.",
					},
				},
			},
		},
	}
}

func parseCurrentC3Contract(unit fpf.SourceUnit) GrammarOutcome {
	for _, spec := range currentC3ContractGrammarSpecs() {
		if !isExactStructuralSection(unit, spec.owner, spec.sourceID) {
			continue
		}
		if len(spec.profiles) == 0 {
			return malformedGrammar(
				unit,
				"current_c3_contract_malformed",
				fmt.Sprintf("recognized %s contract has no supported semantic profiles", spec.sourceID),
			)
		}
		matching := make([]c3ContractGrammarProfile, 0, 1)
		closestKind := C3ContractKind(0)
		closestMissing := []string(nil)
		for _, profile := range spec.profiles {
			missing := missingSourceCues(unit.Body, profile.required)
			if len(missing) == 0 {
				matching = append(matching, profile)
				continue
			}
			if closestMissing == nil || len(missing) < len(closestMissing) {
				closestKind = profile.kind
				closestMissing = missing
			}
		}
		if len(matching) != 1 {
			detail := "matches more than one supported semantic profile"
			if len(matching) == 0 {
				detail = fmt.Sprintf(
					"matches no complete supported semantic profile; closest %s profile is missing source cue %q",
					closestKind.String(),
					closestMissing[0],
				)
			}
			return malformedGrammar(
				unit,
				"current_c3_contract_malformed",
				fmt.Sprintf("recognized %s contract %s", spec.sourceID, detail),
			)
		}
		profile := matching[0]
		declaration := C3ContractDeclaration{
			source:      unit,
			kind:        profile.kind,
			designator:  profile.designator,
			coordinates: append([]string(nil), profile.coordinates...),
		}
		return GrammarParsed{
			unitID:       unit.UnitID,
			declarations: []StructuralDeclaration{declaration},
		}
	}
	return GrammarNoMatch{unitID: unit.UnitID}
}

func missingSourceCues(body string, required []string) []string {
	missing := make([]string, 0)
	for _, cue := range required {
		if strings.Contains(body, cue) {
			continue
		}
		missing = append(missing, cue)
	}
	return missing
}

func malformedGrammar(unit fpf.SourceUnit, code, message string) GrammarMalformed {
	diagnostic, _ := NewCompilerDiagnostic(code, unit.UnitID, message)
	return GrammarMalformed{
		unitID:      unit.UnitID,
		diagnostics: []CompilerDiagnostic{diagnostic},
	}
}
