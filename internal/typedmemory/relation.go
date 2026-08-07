package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
)

type CandidateSlotFiller interface {
	candidateSlotFillerVariant()
	validCandidateSlotFiller() bool
}

type ByReferenceCandidate struct {
	reference StrongRef
}

func NewByReferenceCandidate(reference StrongRef) (ByReferenceCandidate, error) {
	if !validStrongRef(reference) {
		return ByReferenceCandidate{}, fmt.Errorf("reference filler requires a valid strong reference")
	}
	return ByReferenceCandidate{reference: reference}, nil
}

func (candidate ByReferenceCandidate) Reference() StrongRef { return candidate.reference }

func (ByReferenceCandidate) candidateSlotFillerVariant() {}

func (candidate ByReferenceCandidate) validCandidateSlotFiller() bool {
	return validStrongRef(candidate.reference)
}

type ByValueCandidate struct {
	value TypedValueCandidate
}

func NewByValueCandidate(value TypedValueCandidate) (ByValueCandidate, error) {
	if !value.valid() {
		return ByValueCandidate{}, fmt.Errorf("value filler requires an exact typed-value candidate")
	}
	return ByValueCandidate{value: value}, nil
}

func (candidate ByValueCandidate) Value() TypedValueCandidate { return candidate.value }

func (ByValueCandidate) candidateSlotFillerVariant() {}

func (candidate ByValueCandidate) validCandidateSlotFiller() bool {
	return candidate.value.valid()
}

type CandidateSlotBinding struct {
	name    SlotKindID
	fillers []CandidateSlotFiller
}

// Fillers form a cardinality-counted multiset. Repetition is preserved in the
// canonical digest and counts toward SlotSpec cardinality; uniqueness exists
// only when an explicit source-backed constraint introduces it. Duplicate
// SlotKinds in one relational candidate remain structurally forbidden.

func NewCandidateSlotBinding(
	name SlotKindID,
	fillers []CandidateSlotFiller,
) (CandidateSlotBinding, error) {
	if !name.valid() {
		return CandidateSlotBinding{}, fmt.Errorf("candidate slot binding requires a name")
	}
	if len(fillers) == 0 {
		return CandidateSlotBinding{}, fmt.Errorf("candidate slot binding %q requires at least one filler", name.String())
	}

	owned := append([]CandidateSlotFiller(nil), fillers...)
	for index, filler := range owned {
		if !validCandidateSlotFillerVariant(filler) {
			return CandidateSlotBinding{}, fmt.Errorf("candidate slot %q has invalid filler at index %d", name.String(), index)
		}
	}
	return CandidateSlotBinding{name: name, fillers: owned}, nil
}

func (binding CandidateSlotBinding) Name() SlotKindID { return binding.name }

func (binding CandidateSlotBinding) Fillers() []CandidateSlotFiller {
	return append([]CandidateSlotFiller(nil), binding.fillers...)
}

func (binding CandidateSlotBinding) valid() bool {
	if !binding.name.valid() || len(binding.fillers) == 0 {
		return false
	}
	for _, filler := range binding.fillers {
		if !validCandidateSlotFillerVariant(filler) {
			return false
		}
	}
	return true
}

func validCandidateSlotFillerVariant(filler CandidateSlotFiller) bool {
	switch value := filler.(type) {
	case ByReferenceCandidate:
		return value.validCandidateSlotFiller()
	case ByValueCandidate:
		return value.validCandidateSlotFiller()
	default:
		return false
	}
}

type RelationInstantiation struct {
	assertion  AssertionID
	signature  RelationSignatureRef
	slice      ContextSlice
	bindings   []CandidateSlotBinding
	provenance ProvenanceRef
}

func NewRelationInstantiation(
	assertion AssertionID,
	signature RelationSignatureRef,
	slice ContextSlice,
	bindings []CandidateSlotBinding,
	provenance ProvenanceRef,
) (RelationInstantiation, error) {
	if !assertion.valid() {
		return RelationInstantiation{}, fmt.Errorf("relation instantiation requires an assertion ID")
	}
	if !signature.valid() {
		return RelationInstantiation{}, fmt.Errorf("relation instantiation requires a signature")
	}
	if !slice.valid() {
		return RelationInstantiation{}, fmt.Errorf("relation instantiation requires a complete ContextSlice")
	}
	if !provenance.valid() {
		return RelationInstantiation{}, fmt.Errorf("relation instantiation requires provenance")
	}

	normalized, err := normalizeCandidateBindings(bindings)
	if err != nil {
		return RelationInstantiation{}, err
	}
	return RelationInstantiation{
		assertion:  assertion,
		signature:  signature,
		slice:      slice,
		bindings:   normalized,
		provenance: provenance,
	}, nil
}

func (relation RelationInstantiation) Assertion() AssertionID { return relation.assertion }

func (relation RelationInstantiation) Signature() RelationSignatureRef { return relation.signature }

func (relation RelationInstantiation) RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef {
	return relation.signature
}

func (RelationInstantiation) RelationDeclarationPosture() RelationDeclarationPosture {
	return RelationDeclarationTypedFragment
}

func (relation RelationInstantiation) Slice() ContextSlice { return relation.slice }

func (relation RelationInstantiation) Context() BoundedContextRef { return relation.slice.Context() }

func (relation RelationInstantiation) Bindings() []CandidateSlotBinding {
	return append([]CandidateSlotBinding(nil), relation.bindings...)
}

func (relation RelationInstantiation) Provenance() ProvenanceRef { return relation.provenance }

func (relation RelationInstantiation) valid() bool {
	return relation.assertion.valid() &&
		relation.signature.valid() &&
		relation.slice.valid() &&
		len(relation.bindings) > 0 &&
		relation.provenance.valid()
}

type SlotFiller interface {
	slotFillerVariant()
	validSlotFiller() bool
}

type ReferenceFiller struct {
	reference PersistedRef
	entity    EntityID
}

func newReferenceFiller(reference PersistedRef, entity EntityID) ReferenceFiller {
	return ReferenceFiller{reference: reference, entity: entity}
}

func (filler ReferenceFiller) Reference() PersistedRef { return filler.reference }

func (filler ReferenceFiller) Entity() EntityID { return filler.entity }

func (ReferenceFiller) slotFillerVariant() {}

func (filler ReferenceFiller) validSlotFiller() bool {
	return filler.reference.kind.valid() &&
		filler.reference.id.valid() &&
		filler.entity.valid()
}

type ValueFiller struct {
	value VerifiedTypedValue
}

func newValueFiller(value VerifiedTypedValue) ValueFiller {
	return ValueFiller{value: value}
}

func (filler ValueFiller) Value() VerifiedTypedValue { return filler.value }

func (ValueFiller) slotFillerVariant() {}

func (filler ValueFiller) validSlotFiller() bool { return validVerifiedTypedValue(filler.value) }

type SlotBinding struct {
	name    SlotKindID
	fillers []SlotFiller
}

func newSlotBinding(name SlotKindID, fillers []SlotFiller) SlotBinding {
	owned := append([]SlotFiller(nil), fillers...)
	sort.SliceStable(owned, func(left, right int) bool {
		leftBytes := canonicalSlotFiller(owned[left])
		rightBytes := canonicalSlotFiller(owned[right])
		return bytes.Compare(leftBytes, rightBytes) < 0
	})
	return SlotBinding{name: name, fillers: owned}
}

func (binding SlotBinding) Name() SlotKindID { return binding.name }

func (binding SlotBinding) Fillers() []SlotFiller {
	return append([]SlotFiller(nil), binding.fillers...)
}

func (binding SlotBinding) valid() bool {
	if !binding.name.valid() || len(binding.fillers) == 0 {
		return false
	}
	for index, filler := range binding.fillers {
		switch value := filler.(type) {
		case ReferenceFiller:
			if !value.validSlotFiller() {
				return false
			}
		case ValueFiller:
			if !value.validSlotFiller() {
				return false
			}
		default:
			return false
		}
		if index > 0 && bytes.Compare(
			canonicalSlotFiller(binding.fillers[index-1]),
			canonicalSlotFiller(filler),
		) > 0 {
			return false
		}
	}
	return true
}

type RelationInstance struct {
	assertion  AssertionID
	signature  RelationSignatureRef
	slice      ContextSlice
	bindings   []SlotBinding
	provenance ProvenanceRef
}

func newRelationInstance(candidate RelationInstantiation, bindings []SlotBinding) RelationInstance {
	return RelationInstance{
		assertion:  candidate.assertion,
		signature:  candidate.signature,
		slice:      candidate.slice,
		bindings:   append([]SlotBinding(nil), bindings...),
		provenance: candidate.provenance,
	}
}

func (relation RelationInstance) Assertion() AssertionID { return relation.assertion }

func (relation RelationInstance) Signature() RelationSignatureRef { return relation.signature }

func (relation RelationInstance) RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef {
	return relation.signature
}

func (RelationInstance) RelationDeclarationPosture() RelationDeclarationPosture {
	return RelationDeclarationTypedFragment
}

func (relation RelationInstance) Slice() ContextSlice { return relation.slice }

func (relation RelationInstance) Context() BoundedContextRef { return relation.slice.Context() }

func (relation RelationInstance) Bindings() []SlotBinding {
	return append([]SlotBinding(nil), relation.bindings...)
}

func (relation RelationInstance) Provenance() ProvenanceRef { return relation.provenance }

func (relation RelationInstance) valid() bool {
	if !relation.assertion.valid() ||
		!relation.signature.valid() ||
		!relation.slice.valid() ||
		len(relation.bindings) == 0 ||
		!relation.provenance.valid() {
		return false
	}
	for index, binding := range relation.bindings {
		if !binding.valid() {
			return false
		}
		if index > 0 && relation.bindings[index-1].name.String() >= binding.name.String() {
			return false
		}
	}
	return true
}

func normalizeCandidateBindings(values []CandidateSlotBinding) ([]CandidateSlotBinding, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("relational candidate requires at least one named slot binding")
	}

	result := append([]CandidateSlotBinding(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return result[left].Name().String() < result[right].Name().String()
	})
	for index, binding := range result {
		if !binding.valid() {
			return nil, fmt.Errorf("relational candidate contains invalid slot binding at index %d", index)
		}
		if index > 0 && result[index-1].Name() == binding.Name() {
			return nil, fmt.Errorf("relational candidate repeats slot %q", binding.Name().String())
		}
	}
	return result, nil
}

func validStrongRef(reference StrongRef) bool {
	if reference == nil {
		return false
	}
	switch value := reference.(type) {
	case PersistedRef:
		return value.kind.valid() && value.id.valid()
	case LocalRef:
		return value.kind.valid() && value.ref.valid()
	default:
		return false
	}
}
