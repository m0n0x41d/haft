package typeenv

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	valueKindCompilerRule          = "fpf.value-kind.declaration.v1"
	refKindCompilerRule            = "fpf.ref-kind.declaration.v1"
	slotKindCompilerRule           = "fpf.slot-kind.declaration.v1"
	typedRelationFragmentRule      = "fpf.typed-relation-declaration-fragment.v3"
	publicationContextCompilerRule = "fpf.publication-context.v1"
	c3SourceContractCompilerRule   = "fpf.c3.source-contract.v1"
)

type relationAssembly struct {
	owner    string
	roots    []RelationRootDeclaration
	slots    []SlotDeclarationFragment
	profiles []RelationProfileDeclaration
}

type symbolicRelationAssembly struct {
	relationName string
	signatures   []SymbolicRelationSignatureDeclaration
	semantics    []SymbolicRelationSemanticsDeclaration
}

type slotResolution struct {
	fragment    SlotDeclarationFragment
	cardinality CardinalityEvidence
	basisUnits  []fpf.SourceUnit
	complete    bool
	reason      string
}

type declarationSet struct {
	bySymbol map[string]LinkedDeclaration
}

func newDeclarationSet() declarationSet {
	return declarationSet{bySymbol: map[string]LinkedDeclaration{}}
}

func (set declarationSet) Add(declaration LinkedDeclaration) error {
	key := declaration.Symbol().String()
	existing, exists := set.bySymbol[key]
	if !exists {
		set.bySymbol[key] = declaration
		return nil
	}
	existingBody := canonicalFields(existing.body.fields, "declaration-body-fields.v1")
	candidateBody := canonicalFields(declaration.body.fields, "declaration-body-fields.v1")
	if !bytes.Equal(existingBody, candidateBody) {
		return fmt.Errorf("conflicting source declarations for %q", key)
	}
	if existing.RuleID().String() != declaration.RuleID().String() {
		return fmt.Errorf("conflicting compiler rules for %q", key)
	}
	existingBasis := canonicalDeclarationBasis(existing.basis)
	candidateBasis := canonicalDeclarationBasis(declaration.basis)
	if bytes.Equal(existingBasis, candidateBasis) {
		return nil
	}
	locations := append(existing.Basis().SourceLocations(), declaration.Basis().SourceLocations()...)
	basis, err := NewCompilerDerivedDeclarationBasis(existing.RuleID(), locations)
	if err != nil {
		return fmt.Errorf("merge exact source basis for %q: %w", key, err)
	}
	merged, err := NewLinkedDeclaration(
		existing.Symbol(),
		existing.RuleID(),
		existing.Body(),
		basis,
	)
	if err != nil {
		return fmt.Errorf("merge declaration %q: %w", key, err)
	}
	set.bySymbol[key] = merged
	return nil
}

func (set declarationSet) Declarations() []LinkedDeclaration {
	declarations := make([]LinkedDeclaration, 0, len(set.bySymbol))
	for _, declaration := range set.bySymbol {
		declarations = append(declarations, declaration)
	}
	sort.Slice(declarations, func(left, right int) bool {
		return declarations[left].Symbol().String() < declarations[right].Symbol().String()
	})
	return declarations
}

func linkStructuralDeclarations(
	revision typedmemory.SourceRevision,
	compiler typedmemory.CompilerSchemaVersion,
	structural []StructuralDeclaration,
	scopeGaps []typedmemory.CoverageEntry,
) (BaseTypeEnvArtifact, error) {
	declarations := newDeclarationSet()
	relations := map[string]*relationAssembly{}
	symbolicRelations := map[string]*symbolicRelationAssembly{}
	gaps := append([]typedmemory.CoverageEntry(nil), scopeGaps...)

	for _, declaration := range structural {
		switch typed := declaration.(type) {
		case SlotSpecProductionDeclaration:
			gap, err := sourceOnlyUnitGap(
				typed.Source(),
				"slot_spec_production_not_materializable_in_runtime_typeenv",
			)
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
			gaps, err = appendUniqueCoverageEntry(gaps, gap)
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
		case SlotRuleDeclaration:
			gap, err := sourceOnlyUnitGap(
				typed.Source(),
				"slot_constraint_not_materializable_in_runtime_typeenv",
			)
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
			gaps, err = appendUniqueCoverageEntry(gaps, gap)
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
		case C3ContractDeclaration:
			linked, gap, err := linkC3SourceContract(typed)
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
			if err := declarations.Add(linked); err != nil {
				return BaseTypeEnvArtifact{}, err
			}
			gaps, err = appendUniqueCoverageEntry(gaps, gap)
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
		case RelationRootDeclaration:
			group, err := relationGroup(relations, typed.OwnerPatternID())
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
			group.roots = append(group.roots, typed)
		case SlotDeclarationFragment:
			group, err := relationGroup(relations, typed.OwnerPatternID())
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
			group.slots = append(group.slots, typed)
		case RelationProfileDeclaration:
			group, err := relationGroup(relations, typed.OwnerPatternID())
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
			group.profiles = append(group.profiles, typed)
		case SymbolicRelationSignatureDeclaration:
			group, err := symbolicRelationGroup(symbolicRelations, typed.RelationName())
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
			group.signatures = append(group.signatures, typed)
		case SymbolicRelationSemanticsDeclaration:
			group, err := symbolicRelationGroup(symbolicRelations, typed.RelationName())
			if err != nil {
				return BaseTypeEnvArtifact{}, err
			}
			group.semantics = append(group.semantics, typed)
		}
	}
	context, err := linkPublicationContext(structural, revision)
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	if err := declarations.Add(context); err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	claimGraph, hasClaimGraph, err := linkClaimGraphRepresentation(structural)
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	if hasClaimGraph {
		if err := declarations.Add(claimGraph.shape); err != nil {
			return BaseTypeEnvArtifact{}, err
		}
		if err := declarations.Add(claimGraph.codec); err != nil {
			return BaseTypeEnvArtifact{}, err
		}
	}

	for _, relationName := range sortedSymbolicRelationNames(symbolicRelations) {
		group := symbolicRelations[relationName]
		relationGap, err := linkSymbolicRelationAssembly(group, declarations)
		if err != nil {
			return BaseTypeEnvArtifact{}, err
		}
		gaps, err = appendUniqueCoverageEntry(gaps, relationGap)
		if err != nil {
			return BaseTypeEnvArtifact{}, err
		}
	}

	owners := sortedRelationOwners(relations)
	for _, owner := range owners {
		group := relations[owner]
		relationGaps, err := linkRelationAssembly(group, declarations)
		if err != nil {
			return BaseTypeEnvArtifact{}, err
		}
		gaps = append(gaps, relationGaps...)
	}

	linked := declarations.Declarations()
	coverage, err := buildCoverageManifest(linked, gaps)
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	ir, err := NewCompiledLinkedTypeEnvIR(revision, compiler, coverage, linked)
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	artifact, err := SealBaseTypeEnv(ir)
	if err != nil {
		return BaseTypeEnvArtifact{}, err
	}
	return artifact, nil
}

func linkC3SourceContract(
	declaration C3ContractDeclaration,
) (LinkedDeclaration, typedmemory.CoverageEntry, error) {
	symbol, err := c3SourceContractSymbol(declaration.Kind())
	if err != nil {
		return LinkedDeclaration{}, typedmemory.CoverageEntry{}, err
	}
	values := make([]DeclarationValue, 0, len(declaration.Coordinates()))
	for _, coordinate := range declaration.Coordinates() {
		values = append(values, NewTextValue(coordinate))
	}
	coordinates, err := NewSetValue(values)
	if err != nil {
		return LinkedDeclaration{}, typedmemory.CoverageEntry{}, err
	}
	body, err := newDeclarationBody([]fieldInput{
		{name: "carrier_kind", value: NewTextValue("source_contract")},
		{name: "contract_kind", value: NewTextValue(declaration.Kind().String())},
		{name: "designator", value: NewTextValue(declaration.Designator())},
		{name: "coordinates", value: coordinates},
	})
	if err != nil {
		return LinkedDeclaration{}, typedmemory.CoverageEntry{}, err
	}
	linked, err := sourceLinkedDeclaration(
		symbol,
		c3SourceContractCompilerRule,
		body,
		declaration.Source(),
	)
	if err != nil {
		return LinkedDeclaration{}, typedmemory.CoverageEntry{}, err
	}
	subject, err := typedmemory.SchemaSymbolCoverage(symbol)
	if err != nil {
		return LinkedDeclaration{}, typedmemory.CoverageEntry{}, err
	}
	location, err := sourceLocation(declaration.Source())
	if err != nil {
		return LinkedDeclaration{}, typedmemory.CoverageEntry{}, err
	}
	gap, err := typedmemory.NewSourceOnlyCoverageEntry(
		subject,
		location,
		"current_c3_contract_requires_project_local_declaration_and_exact_runtime",
	)
	if err != nil {
		return LinkedDeclaration{}, typedmemory.CoverageEntry{}, err
	}
	return linked, gap, nil
}

func c3SourceContractSymbol(
	kind C3ContractKind,
) (typedmemory.SchemaSymbolRef, error) {
	switch kind {
	case C3SubkindRelationContract:
		id, err := typedmemory.NewSignatureID("U.SubkindOf")
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.RelationSymbolRef(id)
	case C3SubkindOrderContract:
		id, err := typedmemory.NewConstraintID("FPF.C3.SubkindOfOrder")
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ConstraintSymbolRef(id)
	case C3KindSignatureContract:
		id, err := typedmemory.NewKindSignatureSymbolID("FPF.C3.KindSignature")
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.KindSignatureSymbolRef(id)
	case C3KindClassificationJudgementContract:
		id, err := typedmemory.NewShapeID("FPF.C3.KindClassificationJudgement")
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ValueShapeSymbolRef(id)
	case C3KindExtensionContract:
		id, err := typedmemory.NewShapeID("FPF.C3.KindExtension")
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ValueShapeSymbolRef(id)
	case C3KindBridgeContract:
		id, err := typedmemory.NewContextBridgeID("FPF.C3.KindBridge")
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ContextBridgeSymbolRef(id)
	case C3RoleMaskContract:
		id, err := typedmemory.NewShapeID("FPF.C3.RoleMaskDeclaration")
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ValueShapeSymbolRef(id)
	case C3KindGuardSeparationContract:
		id, err := typedmemory.NewConstraintID("FPF.C3.GuardSeparation")
		if err != nil {
			return typedmemory.SchemaSymbolRef{}, err
		}
		return typedmemory.ConstraintSymbolRef(id)
	default:
		return typedmemory.SchemaSymbolRef{}, fmt.Errorf(
			"unknown current C.3 contract kind %d",
			kind,
		)
	}
}

func relationGroup(
	groups map[string]*relationAssembly,
	owner string,
) (*relationAssembly, error) {
	key := strings.TrimSpace(owner)
	if key == "" {
		return nil, fmt.Errorf("relation declaration requires source ParentPatternID")
	}
	group, exists := groups[key]
	if exists {
		return group, nil
	}
	group = &relationAssembly{owner: key}
	groups[key] = group
	return group, nil
}

func symbolicRelationGroup(
	groups map[string]*symbolicRelationAssembly,
	relationName string,
) (*symbolicRelationAssembly, error) {
	key := strings.TrimSpace(relationName)
	if key == "" {
		return nil, fmt.Errorf("symbolic relation declaration requires direct relation identity")
	}
	group, exists := groups[key]
	if exists {
		return group, nil
	}
	group = &symbolicRelationAssembly{relationName: key}
	groups[key] = group
	return group, nil
}

func sortedSymbolicRelationNames(
	groups map[string]*symbolicRelationAssembly,
) []string {
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedRelationOwners(groups map[string]*relationAssembly) []string {
	owners := make([]string, 0, len(groups))
	for owner := range groups {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	return owners
}

func linkSymbolicRelationAssembly(
	group *symbolicRelationAssembly,
	declarations declarationSet,
) (typedmemory.CoverageEntry, error) {
	if group == nil {
		return typedmemory.CoverageEntry{}, fmt.Errorf("symbolic relation assembly is required")
	}
	if len(group.signatures) != 1 || len(group.semantics) != 1 {
		return typedmemory.CoverageEntry{}, fmt.Errorf(
			"symbolic relation %q requires one declaration table and one direct semantics source, got %d and %d",
			group.relationName,
			len(group.signatures),
			len(group.semantics),
		)
	}
	signature := group.signatures[0]
	semantics := group.semantics[0]
	if signature.SignatureName() != semantics.SignatureName() {
		return typedmemory.CoverageEntry{}, fmt.Errorf(
			"symbolic relation %q joins different signature epistemes",
			group.relationName,
		)
	}
	for _, slot := range signature.Slots() {
		linked, err := linkSymbolicSlotValueDeclarations(signature.Source(), slot)
		if err != nil {
			return typedmemory.CoverageEntry{}, err
		}
		for _, declaration := range linked {
			if err := declarations.Add(declaration); err != nil {
				return typedmemory.CoverageEntry{}, err
			}
		}
	}
	body, err := symbolicRelationDeclarationBody(signature, semantics)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	id, err := typedmemory.NewSignatureID(group.relationName)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	symbol, err := typedmemory.RelationSymbolRef(id)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	units := uniqueSourceUnits([]fpf.SourceUnit{signature.Source(), semantics.Source()})
	fragment, err := linkedDeclarationFromUnits(
		symbol,
		typedRelationFragmentRule,
		body,
		units,
	)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	if err := declarations.Add(fragment); err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	return sourceOnlySymbolicRelationGap(
		signature,
		"typed_relation_declaration_fragment_not_executable:applicability_dependency_closure_and_runtime_evaluator_unavailable",
	)
}

func linkSymbolicSlotValueDeclarations(
	source fpf.SourceUnit,
	slot SymbolicRelationSlotSpec,
) ([]LinkedDeclaration, error) {
	fragment := SlotDeclarationFragment{
		source:      source,
		owner:       source.ParentPatternID,
		slotKind:    slot.SlotKind(),
		valueKind:   slot.ValueKind(),
		reference:   slot.ReferenceMode(),
		cardinality: MissingCardinalityEvidence{},
	}
	return linkSlotValueDeclarations(fragment)
}

func symbolicRelationDeclarationBody(
	signature SymbolicRelationSignatureDeclaration,
	semantics SymbolicRelationSemanticsDeclaration,
) (DeclarationBody, error) {
	slotValues := make([]DeclarationValue, 0, len(signature.Slots()))
	for _, slot := range signature.Slots() {
		value, err := symbolicRelationSlotValue(slot)
		if err != nil {
			return DeclarationBody{}, err
		}
		slotValues = append(slotValues, value)
	}
	slots, err := NewSetValue(slotValues)
	if err != nil {
		return DeclarationBody{}, err
	}
	dimensions, err := symbolicRelationSourceDimensions(signature, semantics)
	if err != nil {
		return DeclarationBody{}, err
	}
	return newDeclarationBody([]fieldInput{
		{name: "carrier_kind", value: NewTextValue("typed_relation_declaration_fragment")},
		{name: "declaration_episteme", value: NewTextValue(signature.SignatureName())},
		{name: "relation_kind_entity_of_concern", value: NewTextValue(signature.RelationName())},
		{name: "slots", value: slots},
		{name: "source_dimensions", value: dimensions},
	})
}

func symbolicRelationSlotValue(slot SymbolicRelationSlotSpec) (RecordValue, error) {
	inputs := []fieldInput{
		{name: "participant_meaning", value: NewTextValue(slot.ParticipantMeaning())},
		{name: "reference_mode", value: NewTextValue(slot.ReferenceMode().String())},
		{name: "slot_kind", value: NewTextValue(slot.SlotKind())},
		{name: "value_kind", value: NewTextValue(slot.ValueKind())},
	}
	fields := make([]DeclarationField, 0, len(inputs))
	for _, input := range inputs {
		field, err := NewDeclarationField(input.name, input.value)
		if err != nil {
			return RecordValue{}, err
		}
		fields = append(fields, field)
	}
	return NewRecordValue(fields)
}

func symbolicRelationSourceDimensions(
	signature SymbolicRelationSignatureDeclaration,
	semantics SymbolicRelationSemanticsDeclaration,
) (SetValue, error) {
	inputs := []struct {
		dimension string
		unit      fpf.SourceUnit
	}{
		{dimension: "declaration_identity_participant_meanings_and_slot_specs", unit: signature.Source()},
		{dimension: "obtaining_predicate_and_occurrence_identity", unit: semantics.Source()},
	}
	values := make([]DeclarationValue, 0, len(inputs))
	for _, input := range inputs {
		dimension, err := NewDeclarationField("dimension", NewTextValue(input.dimension))
		if err != nil {
			return SetValue{}, err
		}
		unitID, err := NewDeclarationField("source_unit_id", NewTextValue(input.unit.UnitID))
		if err != nil {
			return SetValue{}, err
		}
		value, err := NewRecordValue([]DeclarationField{dimension, unitID})
		if err != nil {
			return SetValue{}, err
		}
		values = append(values, value)
	}
	return NewSetValue(values)
}

func linkRelationAssembly(
	group *relationAssembly,
	declarations declarationSet,
) ([]typedmemory.CoverageEntry, error) {
	if group == nil {
		return nil, fmt.Errorf("relation assembly is required")
	}
	if len(group.roots) == 0 {
		return sourceOnlySlotGaps(group.slots, "governing_relation_root_missing")
	}
	if len(group.roots) != 1 {
		return nil, fmt.Errorf("source pattern %q declares %d relation roots", group.owner, len(group.roots))
	}

	root := group.roots[0]
	slots := append([]SlotDeclarationFragment(nil), group.slots...)
	sort.Slice(slots, func(left, right int) bool {
		return slots[left].SlotKind() < slots[right].SlotKind()
	})
	incomplete := make([]slotResolution, 0)
	complete := make([]slotResolution, 0)
	gaps := make([]typedmemory.CoverageEntry, 0)
	for _, slot := range slots {
		valueDeclarations, err := linkSlotValueDeclarations(slot)
		if err != nil {
			return nil, err
		}
		for _, declaration := range valueDeclarations {
			if err := declarations.Add(declaration); err != nil {
				return nil, err
			}
		}
		resolution := resolveSlot(slot, group.profiles)
		if !resolution.complete {
			incomplete = append(incomplete, resolution)
			continue
		}
		complete = append(complete, resolution)
	}
	if len(incomplete) == 0 && len(complete) > 0 {
		for _, resolution := range complete {
			linked, err := linkCompleteSlot(root, resolution)
			if err != nil {
				return nil, err
			}
			for _, declaration := range linked {
				if err := declarations.Add(declaration); err != nil {
					return nil, err
				}
			}
		}
		relation, err := linkCompleteRelation(root, complete)
		if err != nil {
			return nil, err
		}
		if err := declarations.Add(relation); err != nil {
			return nil, err
		}
		return gaps, nil
	}
	for _, resolution := range complete {
		gap, err := sourceOnlyUnitGap(
			resolution.fragment.Source(),
			"slot_requires_closed_relation_signature",
		)
		if err != nil {
			return nil, err
		}
		gaps, err = appendUniqueCoverageEntry(gaps, gap)
		if err != nil {
			return nil, err
		}
	}
	for _, resolution := range incomplete {
		gap, err := sourceOnlyUnitGap(resolution.fragment.Source(), resolution.reason)
		if err != nil {
			return nil, err
		}
		gaps, err = appendUniqueCoverageEntry(gaps, gap)
		if err != nil {
			return nil, err
		}
	}

	gapReason := relationGapReason(incomplete)
	relationGap, err := sourceOnlyRelationGap(root, gapReason)
	if err != nil {
		return nil, err
	}
	gaps = append(gaps, relationGap)
	return gaps, nil
}

func resolveSlot(
	fragment SlotDeclarationFragment,
	profiles []RelationProfileDeclaration,
) slotResolution {
	resolution := slotResolution{
		fragment:   fragment,
		basisUnits: []fpf.SourceUnit{fragment.Source()},
	}
	if _, missing := fragment.ReferenceMode().(MissingReferenceModeEvidence); missing {
		resolution.reason = "slot_reference_mode_missing"
		return resolution
	}

	local := fragment.Cardinality()
	profileCardinality, profileSources, profileKnown, profileConflict := profileRequirement(
		fragment.SlotKind(),
		profiles,
	)
	if profileConflict {
		resolution.reason = "slot_cardinality_profile_conflict"
		return resolution
	}
	if cardinalityKnown(local) {
		if profileKnown && !sameCardinality(local, profileCardinality) {
			resolution.reason = "slot_cardinality_local_profile_conflict"
			return resolution
		}
		resolution.cardinality = local
		resolution.complete = true
		return resolution
	}
	if !profileKnown {
		resolution.reason = "slot_cardinality_missing"
		return resolution
	}
	if hasOptionalFieldCue(fragment.Source().Body) && cardinalityRequiresValue(profileCardinality) {
		resolution.reason = "slot_optional_field_vs_required_profile_conflict"
		return resolution
	}
	resolution.cardinality = profileCardinality
	resolution.basisUnits = append(resolution.basisUnits, profileSources...)
	resolution.complete = true
	return resolution
}

func profileRequirement(
	slotKind string,
	profiles []RelationProfileDeclaration,
) (CardinalityEvidence, []fpf.SourceUnit, bool, bool) {
	var selected CardinalityEvidence
	sources := make([]fpf.SourceUnit, 0)
	found := false
	for _, profile := range profiles {
		for _, requirement := range profile.Requirements() {
			if requirement.SlotKind() != slotKind {
				continue
			}
			candidate := requirement.Cardinality()
			if found && !sameCardinality(selected, candidate) {
				return MissingCardinalityEvidence{}, nil, false, true
			}
			selected = candidate
			sources = append(sources, profile.Source())
			found = true
		}
	}
	return selected, sources, found, false
}

func cardinalityKnown(cardinality CardinalityEvidence) bool {
	_, minimumKnown := cardinality.Minimum()
	return minimumKnown
}

func sameCardinality(left, right CardinalityEvidence) bool {
	leftMinimum, leftMinimumKnown := left.Minimum()
	rightMinimum, rightMinimumKnown := right.Minimum()
	leftMaximum, leftMaximumKnown := left.Maximum()
	rightMaximum, rightMaximumKnown := right.Maximum()
	return leftMinimumKnown == rightMinimumKnown &&
		leftMaximumKnown == rightMaximumKnown &&
		leftMinimum == rightMinimum &&
		leftMaximum == rightMaximum
}

func cardinalityRequiresValue(cardinality CardinalityEvidence) bool {
	minimum, known := cardinality.Minimum()
	return known && minimum > 0
}

func hasOptionalFieldCue(body string) bool {
	return strings.Contains(body, "? :") || strings.Contains(body, "?:")
}

func relationGapReason(incomplete []slotResolution) string {
	if len(incomplete) == 0 {
		return "relation_signature_not_closed_by_supported_grammar"
	}
	sorted := append([]slotResolution(nil), incomplete...)
	sort.Slice(sorted, func(left, right int) bool {
		return sorted[left].fragment.SlotKind() < sorted[right].fragment.SlotKind()
	})
	first := sorted[0]
	slot := identifierFragment(first.fragment.SlotKind())
	reason := identifierFragment(first.reason)
	return "relation_signature_incomplete:" + slot + ":" + reason
}

func identifierFragment(raw string) string {
	var builder strings.Builder
	for _, value := range strings.TrimSpace(raw) {
		if unicode.IsLetter(value) || unicode.IsDigit(value) || value == '_' || value == '-' {
			builder.WriteRune(value)
			continue
		}
		builder.WriteRune('_')
	}
	return builder.String()
}

func linkCompleteSlot(
	root RelationRootDeclaration,
	resolution slotResolution,
) ([]LinkedDeclaration, error) {
	linked, err := linkSlotValueDeclarations(resolution.fragment)
	if err != nil {
		return nil, err
	}
	fragment := resolution.fragment
	valueSymbol, err := kindSymbol(fragment.ValueKind())
	if err != nil {
		return nil, err
	}
	refSymbol, hasRef, err := linkRefKind(fragment, valueSymbol)
	if err != nil {
		return nil, err
	}

	slotDeclaration, err := linkSlotKind(root, resolution, valueSymbol, refSymbol, hasRef)
	if err != nil {
		return nil, err
	}
	linked = append(linked, slotDeclaration)
	return linked, nil
}

func linkSlotValueDeclarations(
	fragment SlotDeclarationFragment,
) ([]LinkedDeclaration, error) {
	valueSymbol, err := kindSymbol(fragment.ValueKind())
	if err != nil {
		return nil, err
	}
	valueBody, err := newDeclarationBody([]fieldInput{
		{name: "kind_id", value: NewTextValue(fragment.ValueKind())},
		{name: "semantic_role", value: NewTextValue("value_kind")},
	})
	if err != nil {
		return nil, err
	}
	valueDeclaration, err := sourceLinkedDeclaration(
		valueSymbol,
		valueKindCompilerRule,
		valueBody,
		fragment.Source(),
	)
	if err != nil {
		return nil, err
	}
	linked := []LinkedDeclaration{valueDeclaration}
	refDeclaration, hasRef, err := linkRefKind(fragment, valueSymbol)
	if err != nil {
		return nil, err
	}
	if hasRef {
		linked = append(linked, refDeclaration)
	}
	return linked, nil
}

func linkCompleteRelation(
	root RelationRootDeclaration,
	slots []slotResolution,
) (LinkedDeclaration, error) {
	id, err := typedmemory.NewSignatureID(root.RelationName())
	if err != nil {
		return LinkedDeclaration{}, err
	}
	symbol, err := typedmemory.RelationSymbolRef(id)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	values := make([]DeclarationValue, 0, len(slots))
	basisUnits := []fpf.SourceUnit{root.Source()}
	for _, slot := range slots {
		slotID, slotErr := typedmemory.NewSlotKindID(slot.fragment.SlotKind())
		if slotErr != nil {
			return LinkedDeclaration{}, slotErr
		}
		slotSymbol, slotErr := typedmemory.SlotKindSymbolRef(id, slotID)
		if slotErr != nil {
			return LinkedDeclaration{}, slotErr
		}
		value, valueErr := NewSymbolValue(slotSymbol)
		if valueErr != nil {
			return LinkedDeclaration{}, valueErr
		}
		values = append(values, value)
		basisUnits = append(basisUnits, slot.basisUnits...)
	}
	slotSet, err := NewSetValue(values)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	body, err := newDeclarationBody([]fieldInput{
		{name: "carrier_kind", value: NewTextValue("typed_relation_declaration_fragment")},
		{name: "relation_designator", value: NewTextValue(root.RelationName())},
		{name: "subject_kind", value: NewTextValue(root.SubjectKind())},
		{name: "slots", value: slotSet},
		{name: "structural_check_scope", value: NewTextValue("local_structural_assertion_checks_only")},
	})
	if err != nil {
		return LinkedDeclaration{}, err
	}
	return linkedDeclarationFromUnits(
		symbol,
		typedRelationFragmentRule,
		body,
		uniqueSourceUnits(basisUnits),
	)
}

func uniqueSourceUnits(units []fpf.SourceUnit) []fpf.SourceUnit {
	byID := make(map[string]fpf.SourceUnit, len(units))
	for _, unit := range units {
		byID[unit.UnitID] = unit
	}
	identifiers := make([]string, 0, len(byID))
	for identifier := range byID {
		identifiers = append(identifiers, identifier)
	}
	sort.Strings(identifiers)
	result := make([]fpf.SourceUnit, 0, len(identifiers))
	for _, identifier := range identifiers {
		result = append(result, byID[identifier])
	}
	return result
}

func linkRefKind(
	fragment SlotDeclarationFragment,
	valueSymbol typedmemory.SchemaSymbolRef,
) (LinkedDeclaration, bool, error) {
	byReference, ok := fragment.ReferenceMode().(ByReferenceEvidence)
	if !ok {
		return LinkedDeclaration{}, false, nil
	}
	symbol, err := refKindSymbol(byReference.RefKind())
	if err != nil {
		return LinkedDeclaration{}, false, err
	}
	valueReference, err := NewSymbolValue(valueSymbol)
	if err != nil {
		return LinkedDeclaration{}, false, err
	}
	body, err := newDeclarationBody([]fieldInput{
		{name: "ref_kind", value: NewTextValue(byReference.RefKind())},
		{name: "referent_value_kind", value: valueReference},
	})
	if err != nil {
		return LinkedDeclaration{}, false, err
	}
	declaration, err := sourceLinkedDeclaration(
		symbol,
		refKindCompilerRule,
		body,
		fragment.Source(),
	)
	if err != nil {
		return LinkedDeclaration{}, false, err
	}
	return declaration, true, nil
}

func linkSlotKind(
	root RelationRootDeclaration,
	resolution slotResolution,
	valueSymbol typedmemory.SchemaSymbolRef,
	refDeclaration LinkedDeclaration,
	hasRef bool,
) (LinkedDeclaration, error) {
	fragment := resolution.fragment
	signatureID, err := typedmemory.NewSignatureID(root.RelationName())
	if err != nil {
		return LinkedDeclaration{}, err
	}
	slotID, err := typedmemory.NewSlotKindID(fragment.SlotKind())
	if err != nil {
		return LinkedDeclaration{}, err
	}
	symbol, err := typedmemory.SlotKindSymbolRef(signatureID, slotID)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	valueReference, err := NewSymbolValue(valueSymbol)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	cardinality, err := cardinalityRecord(resolution.cardinality)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	fields := []fieldInput{
		{name: "governing_relation", value: NewTextValue(root.RelationName())},
		{name: "slot_kind", value: NewTextValue(fragment.SlotKind())},
		{name: "value_kind", value: valueReference},
		{name: "reference_mode", value: NewTextValue(fragment.ReferenceMode().String())},
		{name: "cardinality", value: cardinality},
	}
	if hasRef {
		refValue, valueErr := NewSymbolValue(refDeclaration.Symbol())
		if valueErr != nil {
			return LinkedDeclaration{}, valueErr
		}
		fields = append(fields, fieldInput{name: "ref_kind", value: refValue})
	}
	body, err := newDeclarationBody(fields)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	return linkedDeclarationFromUnits(
		symbol,
		slotKindCompilerRule,
		body,
		resolution.basisUnits,
	)
}

func cardinalityRecord(cardinality CardinalityEvidence) (RecordValue, error) {
	minimum, _ := cardinality.Minimum()
	maximum, bounded := cardinality.Maximum()
	minimumField, err := NewDeclarationField("minimum", NewUnsignedValue(minimum))
	if err != nil {
		return RecordValue{}, err
	}
	fields := []DeclarationField{minimumField}
	var maximumValue DeclarationValue
	if bounded {
		maximumValue = NewUnsignedValue(maximum)
	} else {
		maximumValue = NewTextValue("unbounded")
	}
	maximumField, err := NewDeclarationField("maximum", maximumValue)
	if err != nil {
		return RecordValue{}, err
	}
	fields = append(fields, maximumField)
	return NewRecordValue(fields)
}

type fieldInput struct {
	name  string
	value DeclarationValue
}

func newDeclarationBody(inputs []fieldInput) (DeclarationBody, error) {
	fields := make([]DeclarationField, 0, len(inputs))
	for _, input := range inputs {
		field, err := NewDeclarationField(input.name, input.value)
		if err != nil {
			return DeclarationBody{}, err
		}
		fields = append(fields, field)
	}
	return NewDeclarationBody(fields)
}

func kindSymbol(raw string) (typedmemory.SchemaSymbolRef, error) {
	id, err := typedmemory.NewKindID(raw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	return typedmemory.KindSymbolRef(id)
}

func refKindSymbol(raw string) (typedmemory.SchemaSymbolRef, error) {
	id, err := typedmemory.NewRefKindID(raw)
	if err != nil {
		return typedmemory.SchemaSymbolRef{}, err
	}
	return typedmemory.RefKindSymbolRef(id)
}

func linkPublicationContext(
	structural []StructuralDeclaration,
	revision typedmemory.SourceRevision,
) (LinkedDeclaration, error) {
	contextRef, err := typedmemory.NewBoundedContextRef("fpf:publication")
	if err != nil {
		return LinkedDeclaration{}, err
	}
	symbol, err := typedmemory.BoundedContextSymbolRef(contextRef)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	units := make([]fpf.SourceUnit, 0, len(structural))
	for _, declaration := range structural {
		units = append(units, declaration.Source())
	}
	units = uniqueSourceUnits(units)
	if len(units) == 0 {
		return LinkedDeclaration{}, fmt.Errorf("publication context requires parsed structural source inputs")
	}
	body, err := newDeclarationBody([]fieldInput{
		{name: "context_ref", value: NewTextValue(contextRef.String())},
		{name: "source_revision", value: NewTextValue(revision.String())},
	})
	if err != nil {
		return LinkedDeclaration{}, err
	}
	return compilerDerivedLinkedDeclaration(
		symbol,
		publicationContextCompilerRule,
		body,
		units,
	)
}

func sourceLinkedDeclaration(
	symbol typedmemory.SchemaSymbolRef,
	rule string,
	body DeclarationBody,
	unit fpf.SourceUnit,
) (LinkedDeclaration, error) {
	ruleID, err := typedmemory.NewCompilerRuleID(rule)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	provenance, err := sourceFPFProvenance(unit, ruleID)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	basis, err := NewSourceDeclarationBasis(provenance)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	return NewLinkedDeclaration(symbol, ruleID, body, basis)
}

func linkedDeclarationFromUnits(
	symbol typedmemory.SchemaSymbolRef,
	rule string,
	body DeclarationBody,
	units []fpf.SourceUnit,
) (LinkedDeclaration, error) {
	if len(units) == 1 {
		return sourceLinkedDeclaration(symbol, rule, body, units[0])
	}
	ruleID, err := typedmemory.NewCompilerRuleID(rule)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	locations := make([]typedmemory.SourceLocation, 0, len(units))
	for _, unit := range units {
		location, locationErr := sourceLocation(unit)
		if locationErr != nil {
			return LinkedDeclaration{}, locationErr
		}
		locations = append(locations, location)
	}
	basis, err := NewCompilerDerivedDeclarationBasis(ruleID, locations)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	return NewLinkedDeclaration(symbol, ruleID, body, basis)
}

func compilerDerivedLinkedDeclaration(
	symbol typedmemory.SchemaSymbolRef,
	rule string,
	body DeclarationBody,
	units []fpf.SourceUnit,
) (LinkedDeclaration, error) {
	ruleID, err := typedmemory.NewCompilerRuleID(rule)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	locations := make([]typedmemory.SourceLocation, 0, len(units))
	for _, unit := range units {
		location, locationErr := sourceLocation(unit)
		if locationErr != nil {
			return LinkedDeclaration{}, locationErr
		}
		locations = append(locations, location)
	}
	basis, err := NewCompilerDerivedDeclarationBasis(ruleID, locations)
	if err != nil {
		return LinkedDeclaration{}, err
	}
	return NewLinkedDeclaration(symbol, ruleID, body, basis)
}

func buildCoverageManifest(
	declarations []LinkedDeclaration,
	gaps []typedmemory.CoverageEntry,
) (typedmemory.CoverageManifest, error) {
	entries := append([]typedmemory.CoverageEntry(nil), gaps...)
	for _, declaration := range declarations {
		subject, err := typedmemory.SchemaSymbolCoverage(declaration.Symbol())
		if err != nil {
			return typedmemory.CoverageManifest{}, err
		}
		sources := declaration.Basis().SourceLocations()
		if explicit, exists := coverageEntryBySubject(entries, subject); exists {
			if explicit.Posture() == typedmemory.CoverageCompiled {
				return typedmemory.CoverageManifest{}, fmt.Errorf(
					"explicit compiled coverage for %q must be derived from its declaration",
					declaration.Symbol().String(),
				)
			}
			if !sourceLocationIn(explicit.Source(), sources) {
				return typedmemory.CoverageManifest{}, fmt.Errorf(
					"explicit source-only coverage for %q is outside its declaration basis",
					declaration.Symbol().String(),
				)
			}
			continue
		}
		entry, err := typedmemory.NewCompiledCoverageEntry(subject, sources[0])
		if err != nil {
			return typedmemory.CoverageManifest{}, err
		}
		entries = append(entries, entry)
	}
	return typedmemory.NewCoverageManifest(entries)
}

func coverageEntryBySubject(
	entries []typedmemory.CoverageEntry,
	subject typedmemory.CoverageSubject,
) (typedmemory.CoverageEntry, bool) {
	key := subject.String()
	for _, entry := range entries {
		if entry.Subject().String() == key {
			return entry, true
		}
	}
	return typedmemory.CoverageEntry{}, false
}

func appendUniqueCoverageEntry(
	entries []typedmemory.CoverageEntry,
	candidate typedmemory.CoverageEntry,
) ([]typedmemory.CoverageEntry, error) {
	key := candidate.Subject().String()
	for _, entry := range entries {
		if entry.Subject().String() != key {
			continue
		}
		if equalCoverageEntry(entry, candidate) {
			return entries, nil
		}
		return nil, fmt.Errorf("conflicting coverage entries for %q", key)
	}
	return append(entries, candidate), nil
}

func equalCoverageEntry(
	left typedmemory.CoverageEntry,
	right typedmemory.CoverageEntry,
) bool {
	return left.Subject().String() == right.Subject().String() &&
		left.Posture() == right.Posture() &&
		left.Rationale() == right.Rationale() &&
		equalSourceLocation(left.Source(), right.Source())
}

func equalSourceLocation(
	left typedmemory.SourceLocation,
	right typedmemory.SourceLocation,
) bool {
	leftRange := left.LineRange()
	rightRange := right.LineRange()
	leftPattern, leftHasPattern := left.PatternID()
	rightPattern, rightHasPattern := right.PatternID()
	return left.UnitID().String() == right.UnitID().String() &&
		left.Revision().String() == right.Revision().String() &&
		left.ContentHash().String() == right.ContentHash().String() &&
		leftRange.Start() == rightRange.Start() &&
		leftRange.End() == rightRange.End() &&
		leftHasPattern == rightHasPattern &&
		leftPattern.String() == rightPattern.String()
}

func sourceOnlyRelationGap(
	root RelationRootDeclaration,
	reason string,
) (typedmemory.CoverageEntry, error) {
	id, err := typedmemory.NewSignatureID(root.RelationName())
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	symbol, err := typedmemory.RelationSymbolRef(id)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	subject, err := typedmemory.SchemaSymbolCoverage(symbol)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	location, err := sourceLocation(root.Source())
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	return typedmemory.NewSourceOnlyCoverageEntry(subject, location, reason)
}

func sourceOnlySymbolicRelationGap(
	declaration SymbolicRelationSignatureDeclaration,
	reason string,
) (typedmemory.CoverageEntry, error) {
	id, err := typedmemory.NewSignatureID(declaration.RelationName())
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	symbol, err := typedmemory.RelationSymbolRef(id)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	subject, err := typedmemory.SchemaSymbolCoverage(symbol)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	location, err := sourceLocation(declaration.Source())
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	return typedmemory.NewSourceOnlyCoverageEntry(subject, location, reason)
}

func sourceOnlyUnitGap(
	unit fpf.SourceUnit,
	reason string,
) (typedmemory.CoverageEntry, error) {
	unitID, err := typedmemory.NewSourceUnitID(unit.UnitID)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	subject, err := typedmemory.SourceUnitCoverage(unitID)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	location, err := sourceLocation(unit)
	if err != nil {
		return typedmemory.CoverageEntry{}, err
	}
	return typedmemory.NewSourceOnlyCoverageEntry(subject, location, reason)
}

func sourceOnlySlotGaps(
	slots []SlotDeclarationFragment,
	reason string,
) ([]typedmemory.CoverageEntry, error) {
	gaps := make([]typedmemory.CoverageEntry, 0, len(slots))
	for _, slot := range slots {
		gap, err := sourceOnlyUnitGap(slot.Source(), reason)
		if err != nil {
			return nil, err
		}
		gaps = append(gaps, gap)
	}
	return gaps, nil
}

var heterogeneousCoverageScope = map[string]struct{}{
	"A.14":      {},
	"A.15":      {},
	"A.22.CGUS": {},
}

func publicationScopeCoverage(
	units []fpf.SourceUnit,
) ([]typedmemory.CoverageEntry, error) {
	gaps := make([]typedmemory.CoverageEntry, 0)
	for _, unit := range units {
		if unit.Role != fpf.SourceUnitRolePatternBody &&
			unit.Role != fpf.SourceUnitRolePatternSection {
			continue
		}
		owner := unit.PatternID
		if unit.Role == fpf.SourceUnitRolePatternSection {
			owner = unit.ParentPatternID
		}
		reason, included := publicationSourceOnlyReason(unit, owner)
		if !included {
			continue
		}
		gap, err := sourceOnlyUnitGap(unit, reason)
		if err != nil {
			return nil, err
		}
		gaps = append(gaps, gap)
	}
	return gaps, nil
}

func publicationSourceOnlyReason(
	unit fpf.SourceUnit,
	owner string,
) (string, bool) {
	if roleAssignmentSourceConflict(unit, owner) {
		return "source_conflict_with_direct_governor:role_assignment_slot_model", true
	}
	if retiredRelationOntologyReference(unit, owner) {
		return "source_conflict_with_direct_governor:retired_relation_ontology", true
	}
	if _, included := heterogeneousCoverageScope[owner]; included {
		return "heterogeneous_normative_prose_outside_cov2_grammar", true
	}
	return "", false
}

func roleAssignmentSourceConflict(unit fpf.SourceUnit, owner string) bool {
	if owner != "A.2.1" {
		return false
	}
	return strings.Contains(unit.Body, "BoundedContextSlot") &&
		strings.Contains(unit.Body, "AssignmentWindowSlot") &&
		strings.Contains(unit.Body, "RoleValueSlot")
}

func retiredRelationOntologyReference(unit fpf.SourceUnit, owner string) bool {
	if owner == "A.6.5" {
		return false
	}
	retired := []string{
		"U.EpistemeSlotRelation",
		"U.RelationSlotDiscipline",
		"U.EpistemeKind",
	}
	for _, token := range retired {
		if strings.Contains(unit.Body, token) {
			return true
		}
	}
	return false
}

// LowerBaseTypeEnvArtifact materializes the immutable runtime environment from
// one verified linked artifact. It lowers only declarations representable by
// the current typed-memory algebra. In particular, it does not fabricate a
// C.2.1 signature or context-kind availability when the source artifact marks
// them as source-only.
func LowerBaseTypeEnvArtifact(
	artifact BaseTypeEnvArtifact,
) (typedmemory.TypeEnv, error) {
	environment, _, err := LowerBaseTypeEnvArtifactWithCodecs(artifact)
	return environment, err
}

// LowerBaseTypeEnvArtifactWithCodecs returns the executable environment and
// its exact immutable codec mechanisms as one lowering result. Registry
// presence remains mechanism, while the ValueBinding in the environment is
// project admission.
func LowerBaseTypeEnvArtifactWithCodecs(
	artifact BaseTypeEnvArtifact,
) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error) {
	ref, err := verifyExecutableBaseTypeEnvArtifact(artifact)
	if err != nil {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, err
	}
	return lowerVerifiedBaseTypeEnvArtifactWithCodecsAtRef(artifact, ref)
}

// LowerBaseTypeEnvArtifactWithCodecsAtRef materializes one verified immutable
// base artifact with every TypeEnv-scoped runtime reference bound to target.
//
// This is a pure lowering seam for a composite builder that has already
// authenticated and derived target from the exact base artifact and extension
// DAG. A raw target supplied to this function does not authenticate composite
// identity, authorize activation, or mutate/relabel the base artifact.
func LowerBaseTypeEnvArtifactWithCodecsAtRef(
	artifact BaseTypeEnvArtifact,
	target typedmemory.TypeEnvRef,
) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error) {
	_, err := verifyExecutableBaseTypeEnvArtifact(artifact)
	if err != nil {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, err
	}
	validatedTarget, err := typedmemory.ParseTypeEnvRef(target.String())
	if err != nil || validatedTarget != target {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, fmt.Errorf(
			"target TypeEnv reference is required",
		)
	}
	return lowerVerifiedBaseTypeEnvArtifactWithCodecsAtRef(artifact, target)
}

func verifyExecutableBaseTypeEnvArtifact(
	artifact BaseTypeEnvArtifact,
) (typedmemory.TypeEnvRef, error) {
	if err := artifact.Verify(); err != nil {
		return typedmemory.TypeEnvRef{}, err
	}
	if err := validateExecutableDeclarationSchemas(artifact); err != nil {
		return typedmemory.TypeEnvRef{}, err
	}
	ref, exists := artifact.TypeEnvRef()
	if !exists {
		return typedmemory.TypeEnvRef{}, fmt.Errorf(
			"coverage-only artifact has no runtime TypeEnv identity",
		)
	}
	return ref, nil
}

func lowerVerifiedBaseTypeEnvArtifactWithCodecsAtRef(
	artifact BaseTypeEnvArtifact,
	ref typedmemory.TypeEnvRef,
) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error) {
	contexts, err := lowerBoundedContexts(artifact)
	if err != nil {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, err
	}
	claimGraph, hasClaimGraph, err := lowerClaimGraphRepresentation(artifact, ref)
	if err != nil {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, err
	}

	definitions := make([]typedmemory.KindDefinition, 0)
	refDefinitions := make([]typedmemory.RefKindDefinition, 0)
	fragmentDeclarations := make([]LinkedDeclaration, 0)
	for _, declaration := range artifact.Declarations() {
		if !linkedDeclarationIsCompiled(artifact, declaration) {
			continue
		}
		switch declaration.Symbol().Kind() {
		case typedmemory.KindSymbol:
			definition, definitionErr := lowerKindDefinition(declaration)
			if definitionErr != nil {
				return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, definitionErr
			}
			definitions = append(definitions, definition)
		case typedmemory.RefKindSymbol:
			definition, definitionErr := lowerRefKindDefinition(ref, declaration)
			if definitionErr != nil {
				return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, definitionErr
			}
			refDefinitions = append(refDefinitions, definition)
		case typedmemory.SignatureSymbol:
			fragmentDeclarations = append(fragmentDeclarations, declaration)
		}
	}

	builder := typedmemory.NewTypeEnvBuilder(ref).
		SetSourceRevision(artifact.SourceRevision()).
		SetCompilerSchemaVersion(artifact.CompilerSchemaVersion()).
		SetCoverageManifest(artifact.CoverageManifest())
	for _, context := range contexts {
		builder = builder.AddBoundedContext(context)
	}
	for _, definition := range definitions {
		builder = builder.AddKindDefinition(definition)
	}
	for _, definition := range refDefinitions {
		builder = builder.AddRefKindDefinition(definition)
	}
	registry := typedmemory.NewCodecRegistry()
	if hasClaimGraph {
		builder = builder.
			AddValueShape(claimGraph.ShapeDeclaration()).
			AddValueBinding(claimGraph.ValueBinding())
		registry = claimGraph.Registry()
	}
	contextRefs := make([]typedmemory.BoundedContextRef, 0, len(contexts))
	for _, context := range contexts {
		contextRefs = append(contextRefs, context.Ref())
	}
	for _, declaration := range fragmentDeclarations {
		fragment, fragmentErr := lowerTypedRelationDeclarationFragment(
			ref,
			declaration,
			artifact.Declarations(),
			contextRefs,
		)
		if fragmentErr != nil {
			return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, fragmentErr
		}
		builder = builder.AddTypedRelationDeclarationFragment(fragment)
	}
	environment, err := builder.Build()
	if err != nil {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, err
	}
	if environment.Ref().String() != ref.String() {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, fmt.Errorf(
			"lowered TypeEnv identity differs from lowering target",
		)
	}
	if err := verifyArtifactMaterialization(artifact, environment); err != nil {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, err
	}
	return environment, registry, nil
}

func verifyArtifactMaterialization(
	artifact BaseTypeEnvArtifact,
	environment typedmemory.TypeEnv,
) error {
	declared := make([]string, 0, len(artifact.Declarations()))
	for _, declaration := range artifact.Declarations() {
		if !linkedDeclarationIsCompiled(artifact, declaration) {
			continue
		}
		declared = append(declared, declaration.Symbol().String())
	}
	sort.Strings(declared)
	materialized, err := materializedTypeEnvSymbols(environment)
	if err != nil {
		return err
	}
	if len(declared) != len(materialized) {
		return fmt.Errorf(
			"artifact declares %d compiled symbols but runtime TypeEnv materializes %d",
			len(declared),
			len(materialized),
		)
	}
	for index := range declared {
		if declared[index] != materialized[index] {
			return fmt.Errorf(
				"artifact compiled symbol %q differs from runtime materialization %q",
				declared[index],
				materialized[index],
			)
		}
	}
	return nil
}

func materializedTypeEnvSymbols(
	environment typedmemory.TypeEnv,
) ([]string, error) {
	byKey := map[string]struct{}{}
	for _, context := range environment.BoundedContexts() {
		symbol, err := typedmemory.BoundedContextSymbolRef(context.Ref())
		if err != nil {
			return nil, err
		}
		byKey[symbol.String()] = struct{}{}
	}
	for _, definition := range environment.KindDefinitions() {
		symbol, err := typedmemory.KindSymbolRef(definition.ID())
		if err != nil {
			return nil, err
		}
		byKey[symbol.String()] = struct{}{}
	}
	for _, definition := range environment.RefKindDefinitions() {
		symbol, err := typedmemory.RefKindSymbolRef(definition.Ref().ID())
		if err != nil {
			return nil, err
		}
		byKey[symbol.String()] = struct{}{}
	}
	for _, bridge := range environment.ContextBridges() {
		symbol, err := typedmemory.ContextBridgeSymbolRef(bridge.ID())
		if err != nil {
			return nil, err
		}
		byKey[symbol.String()] = struct{}{}
	}
	for _, fragment := range environment.TypedRelationDeclarationFragments() {
		symbol, err := typedmemory.RelationSymbolRef(fragment.Ref().ID())
		if err != nil {
			return nil, err
		}
		byKey[symbol.String()] = struct{}{}
		for _, slot := range fragment.Slots() {
			slotSymbol, slotErr := typedmemory.SlotKindSymbolRef(
				fragment.Ref().ID(),
				slot.SlotKind(),
			)
			if slotErr != nil {
				return nil, slotErr
			}
			byKey[slotSymbol.String()] = struct{}{}
		}
	}
	for _, shape := range environment.ValueShapes() {
		symbol, err := typedmemory.ValueShapeSymbolRef(shape.Ref().ID())
		if err != nil {
			return nil, err
		}
		byKey[symbol.String()] = struct{}{}
	}
	for _, binding := range environment.ValueBindings() {
		symbol, err := typedmemory.CodecSymbolRef(binding.Codec().ID())
		if err != nil {
			return nil, err
		}
		byKey[symbol.String()] = struct{}{}
	}
	for _, constraint := range environment.Constraints() {
		symbol, err := typedmemory.ConstraintSymbolRef(constraint.ID())
		if err != nil {
			return nil, err
		}
		byKey[symbol.String()] = struct{}{}
	}
	result := make([]string, 0, len(byKey))
	for key := range byKey {
		result = append(result, key)
	}
	sort.Strings(result)
	return result, nil
}

func lowerBoundedContexts(
	artifact BaseTypeEnvArtifact,
) ([]typedmemory.BoundedContext, error) {
	contexts := make([]typedmemory.BoundedContext, 0)
	for _, declaration := range artifact.Declarations() {
		if !linkedDeclarationIsCompiled(artifact, declaration) {
			continue
		}
		if declaration.Symbol().Kind() != typedmemory.ContextSymbol {
			continue
		}
		contextRef, err := typedmemory.NewBoundedContextRef(declaration.Symbol().Key())
		if err != nil {
			return nil, err
		}
		provenance, err := lowerDeclarationProvenance(declaration)
		if err != nil {
			return nil, err
		}
		context, err := typedmemory.NewBoundedContext(contextRef, provenance)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, context)
	}
	if len(contexts) == 0 {
		return nil, fmt.Errorf("compiled artifact has no materialized bounded context")
	}
	return contexts, nil
}

func lowerKindDefinition(
	declaration LinkedDeclaration,
) (typedmemory.KindDefinition, error) {
	id, err := typedmemory.NewKindID(declaration.Symbol().Key())
	if err != nil {
		return typedmemory.KindDefinition{}, err
	}
	provenance, err := lowerDeclarationProvenance(declaration)
	if err != nil {
		return typedmemory.KindDefinition{}, err
	}
	return typedmemory.NewKindDefinition(id, provenance)
}

func lowerRefKindDefinition(
	ref typedmemory.TypeEnvRef,
	declaration LinkedDeclaration,
) (typedmemory.RefKindDefinition, error) {
	id, err := typedmemory.NewRefKindID(declaration.Symbol().Key())
	if err != nil {
		return typedmemory.RefKindDefinition{}, err
	}
	refKind, err := typedmemory.NewRefKindRef(ref, id)
	if err != nil {
		return typedmemory.RefKindDefinition{}, err
	}
	valueSymbol, err := declarationSymbolField(declaration, "referent_value_kind")
	if err != nil {
		return typedmemory.RefKindDefinition{}, err
	}
	if valueSymbol.Kind() != typedmemory.KindSymbol {
		return typedmemory.RefKindDefinition{}, fmt.Errorf(
			"RefKind %q referent is not a ValueKind symbol",
			id.String(),
		)
	}
	valueID, err := typedmemory.NewKindID(valueSymbol.Key())
	if err != nil {
		return typedmemory.RefKindDefinition{}, err
	}
	valueKind, err := typedmemory.NewValueKindRef(ref, valueID)
	if err != nil {
		return typedmemory.RefKindDefinition{}, err
	}
	provenance, err := lowerDeclarationProvenance(declaration)
	if err != nil {
		return typedmemory.RefKindDefinition{}, err
	}
	return typedmemory.NewRefKindDefinition(refKind, valueKind, provenance)
}

func lowerTypedRelationDeclarationFragment(
	ref typedmemory.TypeEnvRef,
	declaration LinkedDeclaration,
	declarations []LinkedDeclaration,
	contexts []typedmemory.BoundedContextRef,
) (typedmemory.TypedRelationDeclarationFragment, error) {
	id, err := typedmemory.NewSignatureID(declaration.Symbol().Key())
	if err != nil {
		return typedmemory.TypedRelationDeclarationFragment{}, err
	}
	fragmentRef, err := typedmemory.NewTypedRelationDeclarationFragmentRef(ref, id)
	if err != nil {
		return typedmemory.TypedRelationDeclarationFragment{}, err
	}
	slotSymbols, err := declarationSymbolSetField(declaration, "slots")
	if err != nil {
		return typedmemory.TypedRelationDeclarationFragment{}, err
	}
	bySymbol := make(map[string]LinkedDeclaration, len(declarations))
	for _, candidate := range declarations {
		bySymbol[candidate.Symbol().String()] = candidate
	}
	slots := make([]typedmemory.SlotSpec, 0, len(slotSymbols))
	for _, symbol := range slotSymbols {
		if symbol.Kind() != typedmemory.SlotKindSymbol {
			return typedmemory.TypedRelationDeclarationFragment{}, fmt.Errorf(
				"typed relation declaration fragment %q references non-slot symbol %q",
				id.String(),
				symbol.String(),
			)
		}
		slotDeclaration, exists := bySymbol[symbol.String()]
		if !exists {
			return typedmemory.TypedRelationDeclarationFragment{}, fmt.Errorf(
				"typed relation declaration fragment %q references missing slot declaration %q",
				id.String(),
				symbol.String(),
			)
		}
		slot, slotErr := lowerSlotSpec(ref, slotDeclaration)
		if slotErr != nil {
			return typedmemory.TypedRelationDeclarationFragment{}, slotErr
		}
		slots = append(slots, slot)
	}
	provenance, err := lowerDeclarationProvenance(declaration)
	if err != nil {
		return typedmemory.TypedRelationDeclarationFragment{}, err
	}
	return typedmemory.NewTypedRelationDeclarationFragment(
		fragmentRef,
		contexts,
		slots,
		provenance,
	)
}

func lowerSlotSpec(
	ref typedmemory.TypeEnvRef,
	declaration LinkedDeclaration,
) (typedmemory.SlotSpec, error) {
	keyParts := strings.Split(declaration.Symbol().Key(), "/slot/")
	if len(keyParts) != 2 {
		return typedmemory.SlotSpec{}, fmt.Errorf(
			"slot declaration %q has non-canonical key",
			declaration.Symbol().String(),
		)
	}
	slotID, err := typedmemory.NewSlotKindID(keyParts[1])
	if err != nil {
		return typedmemory.SlotSpec{}, err
	}
	valueSymbol, err := declarationSymbolField(declaration, "value_kind")
	if err != nil {
		return typedmemory.SlotSpec{}, err
	}
	if valueSymbol.Kind() != typedmemory.KindSymbol {
		return typedmemory.SlotSpec{}, fmt.Errorf("slot %q ValueKind is not a kind symbol", slotID.String())
	}
	valueID, err := typedmemory.NewKindID(valueSymbol.Key())
	if err != nil {
		return typedmemory.SlotSpec{}, err
	}
	valueKind, err := typedmemory.NewValueKindRef(ref, valueID)
	if err != nil {
		return typedmemory.SlotSpec{}, err
	}
	mode, err := declarationTextField(declaration, "reference_mode")
	if err != nil {
		return typedmemory.SlotSpec{}, err
	}
	target, err := lowerSlotTarget(ref, declaration, valueKind, mode)
	if err != nil {
		return typedmemory.SlotSpec{}, err
	}
	cardinality, err := lowerCardinality(declaration)
	if err != nil {
		return typedmemory.SlotSpec{}, err
	}
	provenance, err := lowerDeclarationProvenance(declaration)
	if err != nil {
		return typedmemory.SlotSpec{}, err
	}
	return typedmemory.NewSlotSpec(slotID, target, cardinality, provenance)
}

func lowerSlotTarget(
	ref typedmemory.TypeEnvRef,
	declaration LinkedDeclaration,
	valueKind typedmemory.ValueKindRef,
	mode string,
) (typedmemory.SlotTarget, error) {
	if mode == "by_value" {
		return typedmemory.NewValueSlotTarget(valueKind)
	}
	if !strings.HasPrefix(mode, "by_reference:") {
		return nil, fmt.Errorf("slot %q has unknown reference mode %q", declaration.Symbol().String(), mode)
	}
	refSymbol, err := declarationSymbolField(declaration, "ref_kind")
	if err != nil {
		return nil, err
	}
	if refSymbol.Kind() != typedmemory.RefKindSymbol {
		return nil, fmt.Errorf("slot %q RefKind is not a reference-kind symbol", declaration.Symbol().String())
	}
	refID, err := typedmemory.NewRefKindID(refSymbol.Key())
	if err != nil {
		return nil, err
	}
	refKind, err := typedmemory.NewRefKindRef(ref, refID)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewReferenceSlotTarget(valueKind, refKind)
}

func lowerCardinality(declaration LinkedDeclaration) (typedmemory.Cardinality, error) {
	record, err := declarationRecordField(declaration, "cardinality")
	if err != nil {
		return typedmemory.Cardinality{}, err
	}
	minimum, err := recordUnsignedField(record, "minimum")
	if err != nil {
		return typedmemory.Cardinality{}, err
	}
	maximumField, exists := recordField(record, "maximum")
	if !exists {
		return typedmemory.Cardinality{}, fmt.Errorf("slot %q cardinality has no maximum", declaration.Symbol().String())
	}
	switch maximum := maximumField.Value().(type) {
	case UnsignedValue:
		return typedmemory.NewBoundedCardinality(minimum, maximum.Value())
	case TextValue:
		if maximum.Value() == "unbounded" {
			return typedmemory.NewUnboundedCardinality(minimum), nil
		}
	}
	return typedmemory.Cardinality{}, fmt.Errorf("slot %q cardinality maximum is invalid", declaration.Symbol().String())
}

func declarationSymbolField(
	declaration LinkedDeclaration,
	name string,
) (typedmemory.SchemaSymbolRef, error) {
	for _, field := range declaration.Body().Fields() {
		if field.Name() != name {
			continue
		}
		value, ok := field.Value().(SymbolValue)
		if !ok {
			return typedmemory.SchemaSymbolRef{}, fmt.Errorf(
				"declaration %q field %q is not a symbol",
				declaration.Symbol().String(),
				name,
			)
		}
		return value.Symbol(), nil
	}
	return typedmemory.SchemaSymbolRef{}, fmt.Errorf(
		"declaration %q has no %q field",
		declaration.Symbol().String(),
		name,
	)
}

func declarationSymbolSetField(
	declaration LinkedDeclaration,
	name string,
) ([]typedmemory.SchemaSymbolRef, error) {
	for _, field := range declaration.Body().Fields() {
		if field.Name() != name {
			continue
		}
		set, ok := field.Value().(SetValue)
		if !ok {
			return nil, fmt.Errorf("declaration %q field %q is not a set", declaration.Symbol().String(), name)
		}
		result := make([]typedmemory.SchemaSymbolRef, 0, len(set.Values()))
		for _, member := range set.Values() {
			symbol, symbolOK := member.(SymbolValue)
			if !symbolOK {
				return nil, fmt.Errorf("declaration %q field %q contains a non-symbol", declaration.Symbol().String(), name)
			}
			result = append(result, symbol.Symbol())
		}
		return result, nil
	}
	return nil, fmt.Errorf("declaration %q has no %q field", declaration.Symbol().String(), name)
}

func declarationTextField(
	declaration LinkedDeclaration,
	name string,
) (string, error) {
	for _, field := range declaration.Body().Fields() {
		if field.Name() != name {
			continue
		}
		value, ok := field.Value().(TextValue)
		if !ok {
			return "", fmt.Errorf("declaration %q field %q is not text", declaration.Symbol().String(), name)
		}
		return value.Value(), nil
	}
	return "", fmt.Errorf("declaration %q has no %q field", declaration.Symbol().String(), name)
}

func declarationRecordField(
	declaration LinkedDeclaration,
	name string,
) (RecordValue, error) {
	for _, field := range declaration.Body().Fields() {
		if field.Name() != name {
			continue
		}
		value, ok := field.Value().(RecordValue)
		if !ok {
			return RecordValue{}, fmt.Errorf("declaration %q field %q is not a record", declaration.Symbol().String(), name)
		}
		return value, nil
	}
	return RecordValue{}, fmt.Errorf("declaration %q has no %q field", declaration.Symbol().String(), name)
}

func recordUnsignedField(record RecordValue, name string) (uint64, error) {
	field, exists := recordField(record, name)
	if !exists {
		return 0, fmt.Errorf("record has no %q field", name)
	}
	value, ok := field.Value().(UnsignedValue)
	if !ok {
		return 0, fmt.Errorf("record field %q is not unsigned", name)
	}
	return value.Value(), nil
}

func recordField(record RecordValue, name string) (DeclarationField, bool) {
	for _, field := range record.Fields() {
		if field.Name() == name {
			return field, true
		}
	}
	return DeclarationField{}, false
}

func lowerDeclarationProvenance(
	declaration LinkedDeclaration,
) (typedmemory.DeclarationProvenance, error) {
	switch basis := declaration.Basis().(type) {
	case SourceBasis:
		return basis.Provenance(), nil
	case DerivedBasis:
		reference, err := typedmemory.NewProvenanceRef(
			"compiler-derived:" + declaration.Symbol().String() + ":" + declaration.Digest().String(),
		)
		if err != nil {
			return nil, err
		}
		return typedmemory.NewCompilerDerivedProvenance(
			reference,
			basis.SourceLocations(),
			basis.RuleID(),
		)
	default:
		return nil, fmt.Errorf("declaration %q has unknown basis", declaration.Symbol().String())
	}
}
