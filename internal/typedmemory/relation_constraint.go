package typedmemory

import (
	"fmt"
	"sort"
)

// SlotCardinalityConstraint preserves the identity and provenance of one
// named cardinality law. The law is structural: TypeEnv construction accepts
// it only when it exactly matches the owning SlotSpec cardinality.
type SlotCardinalityConstraint struct {
	id          ConstraintID
	signature   RelationSignatureRef
	slot        SlotKindID
	cardinality Cardinality
	provenance  DeclarationProvenance
}

func NewSlotCardinalityConstraint(
	id ConstraintID,
	signature RelationSignatureRef,
	slot SlotKindID,
	cardinality Cardinality,
	provenance DeclarationProvenance,
) (SlotCardinalityConstraint, error) {
	if !id.valid() {
		return SlotCardinalityConstraint{}, fmt.Errorf("slot-cardinality constraint ID is required")
	}
	if !signature.valid() {
		return SlotCardinalityConstraint{}, fmt.Errorf(
			"slot-cardinality typed relation declaration fragment is required",
		)
	}
	if !slot.valid() {
		return SlotCardinalityConstraint{}, fmt.Errorf("slot-cardinality slot is required")
	}
	if !cardinality.valid() {
		return SlotCardinalityConstraint{}, fmt.Errorf("slot-cardinality cardinality is invalid")
	}
	if !validDeclarationProvenance(provenance) {
		return SlotCardinalityConstraint{}, fmt.Errorf("slot-cardinality constraint provenance is required")
	}
	return SlotCardinalityConstraint{
		id:          id,
		signature:   signature,
		slot:        slot,
		cardinality: cardinality,
		provenance:  provenance,
	}, nil
}

func (constraint SlotCardinalityConstraint) ID() ConstraintID { return constraint.id }

func (constraint SlotCardinalityConstraint) Signature() RelationSignatureRef {
	return constraint.signature
}

func (constraint SlotCardinalityConstraint) RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef {
	return constraint.signature
}

func (constraint SlotCardinalityConstraint) Slot() SlotKindID { return constraint.slot }

func (constraint SlotCardinalityConstraint) Cardinality() Cardinality {
	return constraint.cardinality
}

func (constraint SlotCardinalityConstraint) Provenance() DeclarationProvenance {
	return constraint.provenance
}

func (constraint SlotCardinalityConstraint) CanonicalBytes() []byte {
	writer := newCanonicalWriter("slot-cardinality-constraint.v1")
	writer.addString(constraint.id.String())
	writer.addString(constraint.signature.String())
	writer.addString(constraint.slot.String())
	addCanonicalCardinality(&writer, constraint.cardinality)
	writer.addBytes(constraint.provenance.CanonicalBytes())
	return writer.bytes()
}

func (constraint SlotCardinalityConstraint) valid() bool {
	return constraint.id.valid() &&
		constraint.signature.valid() &&
		constraint.slot.valid() &&
		constraint.cardinality.valid() &&
		validDeclarationProvenance(constraint.provenance)
}

func (SlotCardinalityConstraint) constraintRuleVariant() {}

// ReferenceSlotSubsetConstraint states that the resolved stable EntityIDs in
// Subset are a subset of those in Superset. TypeEnv construction proves that
// both coordinates are ByReference slots with one exact reference target.
type ReferenceSlotSubsetConstraint struct {
	id         ConstraintID
	signature  RelationSignatureRef
	subset     SlotKindID
	superset   SlotKindID
	provenance DeclarationProvenance
}

func NewReferenceSlotSubsetConstraint(
	id ConstraintID,
	signature RelationSignatureRef,
	subset SlotKindID,
	superset SlotKindID,
	provenance DeclarationProvenance,
) (ReferenceSlotSubsetConstraint, error) {
	if !id.valid() {
		return ReferenceSlotSubsetConstraint{}, fmt.Errorf("reference-slot-subset constraint ID is required")
	}
	if !signature.valid() {
		return ReferenceSlotSubsetConstraint{}, fmt.Errorf(
			"reference-slot-subset typed relation declaration fragment is required",
		)
	}
	if !subset.valid() {
		return ReferenceSlotSubsetConstraint{}, fmt.Errorf("reference-slot-subset subset slot is required")
	}
	if !superset.valid() {
		return ReferenceSlotSubsetConstraint{}, fmt.Errorf("reference-slot-subset superset slot is required")
	}
	if subset == superset {
		return ReferenceSlotSubsetConstraint{}, fmt.Errorf("reference-slot-subset coordinates must be distinct")
	}
	if !validDeclarationProvenance(provenance) {
		return ReferenceSlotSubsetConstraint{}, fmt.Errorf("reference-slot-subset constraint provenance is required")
	}
	return ReferenceSlotSubsetConstraint{
		id:         id,
		signature:  signature,
		subset:     subset,
		superset:   superset,
		provenance: provenance,
	}, nil
}

func (constraint ReferenceSlotSubsetConstraint) ID() ConstraintID { return constraint.id }

func (constraint ReferenceSlotSubsetConstraint) Signature() RelationSignatureRef {
	return constraint.signature
}

func (constraint ReferenceSlotSubsetConstraint) RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef {
	return constraint.signature
}

func (constraint ReferenceSlotSubsetConstraint) Subset() SlotKindID {
	return constraint.subset
}

func (constraint ReferenceSlotSubsetConstraint) Superset() SlotKindID {
	return constraint.superset
}

func (constraint ReferenceSlotSubsetConstraint) Provenance() DeclarationProvenance {
	return constraint.provenance
}

func (constraint ReferenceSlotSubsetConstraint) CanonicalBytes() []byte {
	writer := newCanonicalWriter("reference-slot-subset-constraint.v1")
	writer.addString(constraint.id.String())
	writer.addString(constraint.signature.String())
	writer.addString(constraint.subset.String())
	writer.addString(constraint.superset.String())
	writer.addBytes(constraint.provenance.CanonicalBytes())
	return writer.bytes()
}

func (constraint ReferenceSlotSubsetConstraint) valid() bool {
	return constraint.id.valid() &&
		constraint.signature.valid() &&
		constraint.subset.valid() &&
		constraint.superset.valid() &&
		constraint.subset != constraint.superset &&
		validDeclarationProvenance(constraint.provenance)
}

func (ReferenceSlotSubsetConstraint) constraintRuleVariant() {}

// ReferenceSlotPartitionConstraint states that Whole is the disjoint union of
// Parts after all references have resolved to stable EntityIDs. This type only
// closes the structural law; relation-instance evaluation is a later slice.
type ReferenceSlotPartitionConstraint struct {
	id         ConstraintID
	signature  RelationSignatureRef
	whole      SlotKindID
	parts      []SlotKindID
	provenance DeclarationProvenance
}

func NewReferenceSlotPartitionConstraint(
	id ConstraintID,
	signature RelationSignatureRef,
	whole SlotKindID,
	parts []SlotKindID,
	provenance DeclarationProvenance,
) (ReferenceSlotPartitionConstraint, error) {
	if !id.valid() {
		return ReferenceSlotPartitionConstraint{}, fmt.Errorf("reference-slot-partition constraint ID is required")
	}
	if !signature.valid() {
		return ReferenceSlotPartitionConstraint{}, fmt.Errorf(
			"reference-slot-partition typed relation declaration fragment is required",
		)
	}
	if !whole.valid() {
		return ReferenceSlotPartitionConstraint{}, fmt.Errorf("reference-slot-partition whole slot is required")
	}
	if len(parts) < 2 {
		return ReferenceSlotPartitionConstraint{}, fmt.Errorf("reference-slot-partition requires at least two part slots")
	}
	if !validDeclarationProvenance(provenance) {
		return ReferenceSlotPartitionConstraint{}, fmt.Errorf("reference-slot-partition constraint provenance is required")
	}
	ownedParts := append([]SlotKindID(nil), parts...)
	sort.Slice(ownedParts, func(left, right int) bool {
		return ownedParts[left].String() < ownedParts[right].String()
	})
	for index, part := range ownedParts {
		if !part.valid() {
			return ReferenceSlotPartitionConstraint{}, fmt.Errorf("reference-slot-partition part %d is invalid", index)
		}
		if part == whole {
			return ReferenceSlotPartitionConstraint{}, fmt.Errorf("reference-slot-partition whole cannot also be a part")
		}
		if index > 0 && part == ownedParts[index-1] {
			return ReferenceSlotPartitionConstraint{}, fmt.Errorf("duplicate reference-slot-partition part %q", part.String())
		}
	}
	return ReferenceSlotPartitionConstraint{
		id:         id,
		signature:  signature,
		whole:      whole,
		parts:      ownedParts,
		provenance: provenance,
	}, nil
}

func (constraint ReferenceSlotPartitionConstraint) ID() ConstraintID { return constraint.id }

func (constraint ReferenceSlotPartitionConstraint) Signature() RelationSignatureRef {
	return constraint.signature
}

func (constraint ReferenceSlotPartitionConstraint) RelationDeclarationFragmentRef() TypedRelationDeclarationFragmentRef {
	return constraint.signature
}

func (constraint ReferenceSlotPartitionConstraint) Whole() SlotKindID {
	return constraint.whole
}

func (constraint ReferenceSlotPartitionConstraint) Parts() []SlotKindID {
	return append([]SlotKindID(nil), constraint.parts...)
}

func (constraint ReferenceSlotPartitionConstraint) Provenance() DeclarationProvenance {
	return constraint.provenance
}

func (constraint ReferenceSlotPartitionConstraint) CanonicalBytes() []byte {
	writer := newCanonicalWriter("reference-slot-partition-constraint.v1")
	writer.addString(constraint.id.String())
	writer.addString(constraint.signature.String())
	writer.addString(constraint.whole.String())
	for _, part := range constraint.parts {
		writer.addString(part.String())
	}
	writer.addBytes(constraint.provenance.CanonicalBytes())
	return writer.bytes()
}

func (constraint ReferenceSlotPartitionConstraint) valid() bool {
	if !constraint.id.valid() ||
		!constraint.signature.valid() ||
		!constraint.whole.valid() ||
		len(constraint.parts) < 2 ||
		!validDeclarationProvenance(constraint.provenance) {
		return false
	}
	for index, part := range constraint.parts {
		if !part.valid() || part == constraint.whole {
			return false
		}
		if index > 0 && part.String() <= constraint.parts[index-1].String() {
			return false
		}
	}
	return true
}

func (ReferenceSlotPartitionConstraint) constraintRuleVariant() {}

func addCanonicalCardinality(writer *canonicalWriter, cardinality Cardinality) {
	writer.addUint64(cardinality.minimum)
	maximum, bounded := cardinality.maximum.BoundedValue()
	maximumKind := "unbounded"
	if bounded {
		maximumKind = "finite"
	}
	writer.addString(maximumKind)
	writer.addUint64(maximum)
}

func equalCardinality(left, right Cardinality) bool {
	if !left.valid() || !right.valid() || left.minimum != right.minimum {
		return false
	}
	leftMaximum, leftBounded := left.maximum.BoundedValue()
	rightMaximum, rightBounded := right.maximum.BoundedValue()
	return leftBounded == rightBounded && leftMaximum == rightMaximum
}
