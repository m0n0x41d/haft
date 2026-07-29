package projecttypeenv

import (
	"fmt"
	"strconv"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type compositeSlotCardinality struct {
	cardinality typedmemory.Cardinality
	source      compositeSourceDeclaration
}

type compositeRelationSlotSource struct {
	key      string
	slotKind SourceScalar
	value    SourceScalar
	mode     SourceScalar
	refKind  SourceScalar
	hasRef   bool
}

func lowerCompositeRelationsAndConstraints(
	target typedmemory.TypeEnvRef,
	sources []compositeSourceDeclaration,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) (
	[]typedmemory.TypedRelationDeclarationFragment,
	[]typedmemory.ConstraintRule,
	error,
) {
	cardinalities, err := compositeSlotCardinalityIndex(sources)
	if err != nil {
		return nil, nil, err
	}
	fragments, fragmentRefs, err := lowerCompositeRelationDeclarationFragments(
		target,
		sources,
		cardinalities,
		provenance,
	)
	if err != nil {
		return nil, nil, err
	}
	constraints, err := lowerCompositeConstraints(
		sources,
		fragmentRefs,
		provenance,
	)
	if err != nil {
		return nil, nil, err
	}
	return fragments, constraints, nil
}

func compositeSlotCardinalityIndex(
	sources []compositeSourceDeclaration,
) (map[string]compositeSlotCardinality, error) {
	result := make(map[string]compositeSlotCardinality)
	for _, source := range declarationsOfKind(sources, localpractice.DeclarationConstraint) {
		kind, err := requiredDeclarationFact(source.value, "rule.kind")
		if err != nil {
			return nil, compositeDeclarationError(source, "constraint kind", err)
		}
		if localpractice.ConstraintKind(kind.Value()) != localpractice.ConstraintSlotCardinality {
			continue
		}
		relation, err := requiredDeclarationFact(source.value, "rule.relation")
		if err != nil {
			return nil, compositeDeclarationError(source, "slot cardinality relation", err)
		}
		slot, err := requiredDeclarationFact(source.value, "rule.slot")
		if err != nil {
			return nil, compositeDeclarationError(source, "slot cardinality slot", err)
		}
		cardinality, err := compositeCardinality(source.value)
		if err != nil {
			return nil, compositeDeclarationError(source, "slot cardinality", err)
		}
		key := compositeRelationSlotKey(relation.Value(), slot.Value())
		previous, exists := result[key]
		if exists && !compositeCardinalitiesEqual(previous.cardinality, cardinality) {
			return nil, fmt.Errorf(
				"conflicting slot cardinalities for relation %q slot %q in %q and %q",
				relation.Value(),
				slot.Value(),
				previous.source.value.Symbol().Value(),
				source.value.Symbol().Value(),
			)
		}
		if !exists {
			result[key] = compositeSlotCardinality{
				cardinality: cardinality,
				source:      source,
			}
		}
	}
	return result, nil
}

func lowerCompositeRelationDeclarationFragments(
	target typedmemory.TypeEnvRef,
	sources []compositeSourceDeclaration,
	cardinalities map[string]compositeSlotCardinality,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) (
	[]typedmemory.TypedRelationDeclarationFragment,
	map[string]typedmemory.TypedRelationDeclarationFragmentRef,
	error,
) {
	result := make([]typedmemory.TypedRelationDeclarationFragment, 0)
	refs := make(map[string]typedmemory.TypedRelationDeclarationFragmentRef)
	for _, source := range declarationsOfKind(sources, localpractice.DeclarationRelationSignature) {
		id, err := typedmemory.NewSignatureID(source.value.Symbol().Value())
		if err != nil {
			return nil, nil, compositeDeclarationError(
				source,
				"typed relation declaration fragment ID",
				err,
			)
		}
		ref, err := typedmemory.NewTypedRelationDeclarationFragmentRef(target, id)
		if err != nil {
			return nil, nil, compositeDeclarationError(
				source,
				"typed relation declaration fragment reference",
				err,
			)
		}
		context, err := typedmemory.NewBoundedContextRef(
			source.extension.Artifact().IR().BoundedContext().Value(),
		)
		if err != nil {
			return nil, nil, compositeDeclarationError(source, "relation context", err)
		}
		slotSources, err := compositeRelationSlotSources(source.value)
		if err != nil {
			return nil, nil, compositeDeclarationError(source, "relation slots", err)
		}
		slots := make([]typedmemory.SlotSpec, 0, len(slotSources))
		for _, slotSource := range slotSources {
			slot, err := lowerCompositeRelationSlot(
				target,
				source,
				slotSource,
				cardinalities,
				provenance,
			)
			if err != nil {
				return nil, nil, err
			}
			slots = append(slots, slot)
		}
		basis, err := provenance(source, "relation:"+id.String())
		if err != nil {
			return nil, nil, compositeDeclarationError(source, "relation provenance", err)
		}
		fragment, err := typedmemory.NewTypedRelationDeclarationFragment(
			ref,
			[]typedmemory.BoundedContextRef{context},
			slots,
			basis,
		)
		if err != nil {
			return nil, nil, compositeDeclarationError(
				source,
				"typed relation declaration fragment",
				err,
			)
		}
		result = append(result, fragment)
		refs[source.value.Symbol().Value()] = ref
	}
	return result, refs, nil
}

func compositeRelationSlotSources(
	declaration SymbolicDeclaration,
) ([]compositeRelationSlotSource, error) {
	byKey := make(map[string]*compositeRelationSlotSource)
	for _, fact := range declaration.Facts() {
		key, leaf, matches := parseKeyedFactPath(fact.Path(), "slots")
		if !matches {
			continue
		}
		record, exists := byKey[key]
		if !exists {
			record = &compositeRelationSlotSource{key: key}
			byKey[key] = record
		}
		switch leaf {
		case "slot_kind":
			record.slotKind = fact.Value()
		case "value_kind":
			record.value = fact.Value()
		case "ref_mode.kind":
			record.mode = fact.Value()
		case "ref_mode.ref_kind":
			record.refKind = fact.Value()
			record.hasRef = true
		}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sortStrings(keys)
	result := make([]compositeRelationSlotSource, 0, len(keys))
	for _, key := range keys {
		record := *byKey[key]
		if record.slotKind.Value() == "" || record.value.Value() == "" || record.mode.Value() == "" {
			return nil, fmt.Errorf("slot %q is incomplete", key)
		}
		result = append(result, record)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("typed relation declaration fragment has no slots")
	}
	return result, nil
}

func lowerCompositeRelationSlot(
	target typedmemory.TypeEnvRef,
	source compositeSourceDeclaration,
	slotSource compositeRelationSlotSource,
	cardinalities map[string]compositeSlotCardinality,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) (typedmemory.SlotSpec, error) {
	slotKind, err := typedmemory.NewSlotKindID(slotSource.slotKind.Value())
	if err != nil {
		return typedmemory.SlotSpec{}, compositeDeclarationError(source, "SlotKind", err)
	}
	valueID, err := typedmemory.NewKindID(slotSource.value.Value())
	if err != nil {
		return typedmemory.SlotSpec{}, compositeDeclarationError(source, "slot ValueKind", err)
	}
	valueKind, err := typedmemory.NewValueKindRef(target, valueID)
	if err != nil {
		return typedmemory.SlotSpec{}, compositeDeclarationError(source, "slot ValueKind reference", err)
	}
	var targetValue typedmemory.SlotTarget
	switch localpractice.ReferenceModeKind(slotSource.mode.Value()) {
	case localpractice.ReferenceByValue:
		valueTarget, err := typedmemory.NewValueSlotTarget(valueKind)
		if err != nil {
			return typedmemory.SlotSpec{}, compositeDeclarationError(source, "by-value slot target", err)
		}
		targetValue = valueTarget
	case localpractice.ReferenceByKind:
		if !slotSource.hasRef {
			return typedmemory.SlotSpec{}, compositeDeclarationError(
				source,
				"by-reference slot target",
				fmt.Errorf("ref_kind is missing"),
			)
		}
		refID, err := typedmemory.NewRefKindID(slotSource.refKind.Value())
		if err != nil {
			return typedmemory.SlotSpec{}, compositeDeclarationError(source, "slot RefKind", err)
		}
		refKind, err := typedmemory.NewRefKindRef(target, refID)
		if err != nil {
			return typedmemory.SlotSpec{}, compositeDeclarationError(source, "slot RefKind reference", err)
		}
		referenceTarget, err := typedmemory.NewReferenceSlotTarget(valueKind, refKind)
		if err != nil {
			return typedmemory.SlotSpec{}, compositeDeclarationError(source, "by-reference slot target", err)
		}
		targetValue = referenceTarget
	default:
		return typedmemory.SlotSpec{}, compositeDeclarationError(
			source,
			"slot reference mode",
			fmt.Errorf("unsupported mode %q", slotSource.mode.Value()),
		)
	}
	cardinality := typedmemory.NewUnboundedCardinality(0)
	key := compositeRelationSlotKey(source.value.Symbol().Value(), slotKind.String())
	if exact, exists := cardinalities[key]; exists {
		cardinality = exact.cardinality
	}
	basis, err := provenance(
		source,
		"relation:"+source.value.Symbol().Value()+":slot:"+slotKind.String(),
	)
	if err != nil {
		return typedmemory.SlotSpec{}, compositeDeclarationError(source, "slot provenance", err)
	}
	slot, err := typedmemory.NewSlotSpec(slotKind, targetValue, cardinality, basis)
	if err != nil {
		return typedmemory.SlotSpec{}, compositeDeclarationError(source, "SlotSpec", err)
	}
	return slot, nil
}

func lowerCompositeConstraints(
	sources []compositeSourceDeclaration,
	fragments map[string]typedmemory.TypedRelationDeclarationFragmentRef,
	provenance func(compositeSourceDeclaration, string) (typedmemory.ProjectSourceProvenance, error),
) ([]typedmemory.ConstraintRule, error) {
	result := make([]typedmemory.ConstraintRule, 0)
	for _, source := range declarationsOfKind(sources, localpractice.DeclarationConstraint) {
		id, err := typedmemory.NewConstraintID(source.value.Symbol().Value())
		if err != nil {
			return nil, compositeDeclarationError(source, "constraint ID", err)
		}
		basis, err := provenance(source, "constraint:"+id.String())
		if err != nil {
			return nil, compositeDeclarationError(source, "constraint provenance", err)
		}
		kindSource, err := requiredDeclarationFact(source.value, "rule.kind")
		if err != nil {
			return nil, compositeDeclarationError(source, "constraint kind", err)
		}
		rule, err := lowerCompositeConstraint(
			source,
			id,
			localpractice.ConstraintKind(kindSource.Value()),
			fragments,
			basis,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, rule)
	}
	return result, nil
}

func lowerCompositeConstraint(
	source compositeSourceDeclaration,
	id typedmemory.ConstraintID,
	kind localpractice.ConstraintKind,
	fragments map[string]typedmemory.TypedRelationDeclarationFragmentRef,
	provenance typedmemory.ProjectSourceProvenance,
) (typedmemory.ConstraintRule, error) {
	switch kind {
	case localpractice.ConstraintKindDisjoint:
		values, err := compositeIndexedKindIDs(source.value, "rule.disjoint_kinds")
		if err != nil {
			return nil, compositeDeclarationError(source, "kind-disjoint operands", err)
		}
		constraint, err := typedmemory.NewKindDisjointConstraint(id, values, provenance)
		if err != nil {
			return nil, compositeDeclarationError(source, "kind-disjoint constraint", err)
		}
		return constraint, nil
	case localpractice.ConstraintSlotGroup:
		fragment, err := compositeConstraintRelation(source, fragments)
		if err != nil {
			return nil, err
		}
		slots, err := compositeIndexedSlotIDs(source.value, "rule.slots")
		if err != nil {
			return nil, compositeDeclarationError(source, "slot-group operands", err)
		}
		mode, err := compositeSlotGroupMode(source.value)
		if err != nil {
			return nil, compositeDeclarationError(source, "slot-group mode", err)
		}
		constraint, err := typedmemory.NewSlotGroupConstraint(id, fragment, slots, mode, provenance)
		if err != nil {
			return nil, compositeDeclarationError(source, "slot-group constraint", err)
		}
		return constraint, nil
	case localpractice.ConstraintSlotCardinality:
		fragment, err := compositeConstraintRelation(source, fragments)
		if err != nil {
			return nil, err
		}
		slot, err := compositeConstraintSlotFact(source.value, "rule.slot")
		if err != nil {
			return nil, compositeDeclarationError(source, "slot-cardinality slot", err)
		}
		cardinality, err := compositeCardinality(source.value)
		if err != nil {
			return nil, compositeDeclarationError(source, "slot-cardinality", err)
		}
		constraint, err := typedmemory.NewSlotCardinalityConstraint(
			id,
			fragment,
			slot,
			cardinality,
			provenance,
		)
		if err != nil {
			return nil, compositeDeclarationError(source, "slot-cardinality constraint", err)
		}
		return constraint, nil
	case localpractice.ConstraintReferenceSlotSubset:
		fragment, err := compositeConstraintRelation(source, fragments)
		if err != nil {
			return nil, err
		}
		subset, err := compositeConstraintSlotFact(source.value, "rule.subset")
		if err != nil {
			return nil, compositeDeclarationError(source, "reference subset", err)
		}
		superset, err := compositeConstraintSlotFact(source.value, "rule.superset")
		if err != nil {
			return nil, compositeDeclarationError(source, "reference superset", err)
		}
		constraint, err := typedmemory.NewReferenceSlotSubsetConstraint(
			id,
			fragment,
			subset,
			superset,
			provenance,
		)
		if err != nil {
			return nil, compositeDeclarationError(source, "reference-slot subset constraint", err)
		}
		return constraint, nil
	case localpractice.ConstraintReferenceSlotPartition:
		fragment, err := compositeConstraintRelation(source, fragments)
		if err != nil {
			return nil, err
		}
		whole, err := compositeConstraintSlotFact(source.value, "rule.whole")
		if err != nil {
			return nil, compositeDeclarationError(source, "reference partition whole", err)
		}
		parts, err := compositeIndexedSlotIDs(source.value, "rule.parts")
		if err != nil {
			return nil, compositeDeclarationError(source, "reference partition parts", err)
		}
		constraint, err := typedmemory.NewReferenceSlotPartitionConstraint(
			id,
			fragment,
			whole,
			parts,
			provenance,
		)
		if err != nil {
			return nil, compositeDeclarationError(source, "reference-slot partition constraint", err)
		}
		return constraint, nil
	default:
		return nil, compositeDeclarationError(
			source,
			"constraint",
			fmt.Errorf("unsupported constraint kind %q", kind),
		)
	}
}

func compositeConstraintRelation(
	source compositeSourceDeclaration,
	fragments map[string]typedmemory.TypedRelationDeclarationFragmentRef,
) (typedmemory.TypedRelationDeclarationFragmentRef, error) {
	relationSource, err := requiredDeclarationFact(source.value, "rule.relation")
	if err != nil {
		return typedmemory.TypedRelationDeclarationFragmentRef{}, compositeDeclarationError(
			source,
			"constraint relation fragment",
			err,
		)
	}
	fragment, exists := fragments[relationSource.Value()]
	if !exists {
		return typedmemory.TypedRelationDeclarationFragmentRef{}, compositeDeclarationError(
			source,
			"constraint relation fragment",
			fmt.Errorf(
				"source symbol %q did not lower to a typed relation declaration fragment",
				relationSource.Value(),
			),
		)
	}
	return fragment, nil
}

func compositeConstraintSlotFact(
	declaration SymbolicDeclaration,
	path string,
) (typedmemory.SlotKindID, error) {
	source, err := requiredDeclarationFact(declaration, path)
	if err != nil {
		return typedmemory.SlotKindID{}, err
	}
	return typedmemory.NewSlotKindID(source.Value())
}

func compositeIndexedKindIDs(
	declaration SymbolicDeclaration,
	prefix string,
) ([]typedmemory.KindID, error) {
	sources, err := indexedDeclarationFacts(declaration, prefix)
	if err != nil {
		return nil, err
	}
	result := make([]typedmemory.KindID, 0, len(sources))
	for _, source := range sources {
		id, err := typedmemory.NewKindID(source.Value())
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}

func compositeIndexedSlotIDs(
	declaration SymbolicDeclaration,
	prefix string,
) ([]typedmemory.SlotKindID, error) {
	sources, err := indexedDeclarationFacts(declaration, prefix)
	if err != nil {
		return nil, err
	}
	result := make([]typedmemory.SlotKindID, 0, len(sources))
	for _, source := range sources {
		id, err := typedmemory.NewSlotKindID(source.Value())
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}

func compositeSlotGroupMode(
	declaration SymbolicDeclaration,
) (typedmemory.SlotGroupMode, error) {
	source, err := requiredDeclarationFact(declaration, "rule.mode")
	if err != nil {
		return 0, err
	}
	switch localpractice.SlotGroupMode(source.Value()) {
	case localpractice.SlotGroupAllOrNone:
		return typedmemory.SlotGroupAllOrNone, nil
	case localpractice.SlotGroupAtLeastOne:
		return typedmemory.SlotGroupAtLeastOne, nil
	case localpractice.SlotGroupExactlyOne:
		return typedmemory.SlotGroupExactlyOne, nil
	default:
		return 0, fmt.Errorf("unsupported slot-group mode %q", source.Value())
	}
}

func compositeCardinality(
	declaration SymbolicDeclaration,
) (typedmemory.Cardinality, error) {
	minimumSource, err := requiredDeclarationFact(declaration, "rule.cardinality.minimum")
	if err != nil {
		return typedmemory.Cardinality{}, err
	}
	minimum, err := strconv.ParseUint(minimumSource.Value(), 10, 64)
	if err != nil {
		return typedmemory.Cardinality{}, err
	}
	maximumSource, err := requiredDeclarationFact(declaration, "rule.cardinality.maximum")
	if err != nil {
		return typedmemory.Cardinality{}, err
	}
	if maximumSource.Value() == "unbounded" {
		return typedmemory.NewUnboundedCardinality(minimum), nil
	}
	maximum, err := strconv.ParseUint(maximumSource.Value(), 10, 64)
	if err != nil {
		return typedmemory.Cardinality{}, err
	}
	return typedmemory.NewBoundedCardinality(minimum, maximum)
}

func compositeCardinalitiesEqual(
	left typedmemory.Cardinality,
	right typedmemory.Cardinality,
) bool {
	if left.Minimum() != right.Minimum() {
		return false
	}
	leftMaximum, leftBounded := left.Maximum().BoundedValue()
	rightMaximum, rightBounded := right.Maximum().BoundedValue()
	return leftBounded == rightBounded && leftMaximum == rightMaximum
}

func compositeRelationSlotKey(relation string, slot string) string {
	return relation + "\x00" + slot
}

func sortStrings(values []string) {
	for index := 1; index < len(values); index++ {
		value := values[index]
		position := index
		for position > 0 && values[position-1] > value {
			values[position] = values[position-1]
			position--
		}
		values[position] = value
	}
}
