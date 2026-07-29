package typeenv

import (
	"fmt"
	"regexp"
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
	C3KindClassificationJudgementContract
	C3KindExtensionContract
	C3KindBridgeContract
	C3RoleMaskContract
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
	case C3KindClassificationJudgementContract:
		return "kind_classification_judgement"
	case C3KindExtensionContract:
		return "kind_extension"
	case C3KindBridgeContract:
		return "kind_bridge"
	case C3RoleMaskContract:
		return "role_mask"
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
	expected := currentSlotRuleLabels()
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		ruleID := strings.TrimSpace(match[1])
		label := strings.TrimSpace(match[2])
		expectedLabel, known := expected[ruleID]
		_, duplicate := seen[ruleID]
		if !known || duplicate || label != expectedLabel {
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
	predicateCue      string
	identityCue       string
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
			predicateCue: "`EpistemeConstitutionRelation` obtains exactly when",
			identityCue:  "relation occurrence is participant-determined",
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
			predicateCue: "`EpistemeEmpiricalGroundingRelation(E,H)` obtains exactly while",
			identityCue:  "One occurrence is identified by",
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
			predicateCue: "The relation obtains when",
			identityCue:  "One occurrence is participant-determined",
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
		if !strings.Contains(unit.Body, spec.predicateCue) ||
			!strings.Contains(unit.Body, spec.identityCue) {
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

func currentSlotRuleLabels() map[string]string {
	return map[string]string{
		"A6.5-S1": "CompleteSlotSpec",
		"A6.5-S2": "LocalSlotKind",
		"A6.5-S3": "ExactParticipantKind",
		"A6.5-S4": "HonestReference",
		"A6.5-S5": "DirectPredicateGovernance",
		"A6.5-S6": "NoHiddenUnion",
		"A6.5-S7": "RepresentationBoundary",
	}
}

func isExactStructuralSection(unit fpf.SourceUnit, owner, sourceID string) bool {
	return unit.ParentPatternID == owner && unit.SourceID == sourceID
}

type c3ContractGrammarSpec struct {
	owner       string
	sourceID    string
	kind        C3ContractKind
	designator  string
	coordinates []string
	required    []string
}

func currentC3ContractGrammarSpecs() []c3ContractGrammarSpec {
	return []c3ContractGrammarSpec{
		{
			owner:      "C.3.1",
			sourceID:   "C.3.1:4",
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
		{
			owner:      "C.3.1",
			sourceID:   "C.3.1:5",
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
		{
			owner:      "C.3.2",
			sourceID:   "C.3.2:5",
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
		{
			owner:      "C.3.2",
			sourceID:   "C.3.2:6",
			kind:       C3KindClassificationJudgementContract,
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
		{
			owner:      "C.3.2",
			sourceID:   "C.3.2:7",
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
		{
			owner:      "C.3.3",
			sourceID:   "C.3.3:5",
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
		{
			owner:      "C.3.4",
			sourceID:   "C.3.4:5",
			kind:       C3RoleMaskContract,
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
		{
			owner:      "C.3.A",
			sourceID:   "C.3.A:3",
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
	}
}

func parseCurrentC3Contract(unit fpf.SourceUnit) GrammarOutcome {
	for _, spec := range currentC3ContractGrammarSpecs() {
		if !isExactStructuralSection(unit, spec.owner, spec.sourceID) {
			continue
		}
		missing := missingSourceCues(unit.Body, spec.required)
		if len(missing) > 0 {
			return malformedGrammar(
				unit,
				"current_c3_contract_malformed",
				fmt.Sprintf(
					"recognized %s contract is missing current source cue %q",
					spec.kind.String(),
					missing[0],
				),
			)
		}
		declaration := C3ContractDeclaration{
			source:      unit,
			kind:        spec.kind,
			designator:  spec.designator,
			coordinates: append([]string(nil), spec.coordinates...),
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
