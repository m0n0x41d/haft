package typedmemory

import (
	"bytes"
	"strings"
	"testing"
)

func TestReferenceSlotPartitionConstraintCanonicalizesAndOwnsParts(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	id := typeEnvTestConstraintID(t, "constraint:partition")
	whole := typeEnvTestSlotKindID(t, "WholeSlot")
	left := typeEnvTestSlotKindID(t, "LeftSlot")
	right := typeEnvTestSlotKindID(t, "RightSlot")
	input := []SlotKindID{right, left}

	constraint, err := NewReferenceSlotPartitionConstraint(
		id,
		fixture.signature.Ref(),
		whole,
		input,
		fixture.provenance,
	)
	if err != nil {
		t.Fatalf("NewReferenceSlotPartitionConstraint() error = %v", err)
	}
	reversed, err := NewReferenceSlotPartitionConstraint(
		id,
		fixture.signature.Ref(),
		whole,
		[]SlotKindID{left, right},
		fixture.provenance,
	)
	if err != nil {
		t.Fatalf("NewReferenceSlotPartitionConstraint(reversed) error = %v", err)
	}
	if !bytes.Equal(constraint.CanonicalBytes(), reversed.CanonicalBytes()) {
		t.Fatal("partition part order changed canonical bytes")
	}

	input[0] = SlotKindID{}
	parts := constraint.Parts()
	if len(parts) != 2 || parts[0] != left || parts[1] != right {
		t.Fatalf("canonical partition parts = %v; want LeftSlot, RightSlot", parts)
	}
	parts[0] = SlotKindID{}
	if constraint.Parts()[0] != left {
		t.Fatal("Parts accessor leaked mutable storage")
	}
}

func TestReferenceConstraintConstructorsRejectMalformedCoordinates(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	id := typeEnvTestConstraintID(t, "constraint:coordinates")
	signature := fixture.signature.Ref()
	whole := typeEnvTestSlotKindID(t, "WholeSlot")
	left := typeEnvTestSlotKindID(t, "LeftSlot")
	right := typeEnvTestSlotKindID(t, "RightSlot")

	if _, err := NewReferenceSlotSubsetConstraint(
		id,
		signature,
		left,
		left,
		fixture.provenance,
	); err == nil || !strings.Contains(err.Error(), "distinct") {
		t.Fatalf("equal subset coordinates error = %v", err)
	}
	if _, err := NewReferenceSlotSubsetConstraint(
		ConstraintID{},
		signature,
		left,
		right,
		fixture.provenance,
	); err == nil {
		t.Fatal("subset constructor accepted an empty constraint ID")
	}
	if _, err := NewReferenceSlotPartitionConstraint(
		id,
		signature,
		whole,
		[]SlotKindID{left},
		fixture.provenance,
	); err == nil || !strings.Contains(err.Error(), "at least two") {
		t.Fatalf("short partition error = %v", err)
	}
	if _, err := NewReferenceSlotPartitionConstraint(
		id,
		signature,
		whole,
		[]SlotKindID{left, left},
		fixture.provenance,
	); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate partition part error = %v", err)
	}
	if _, err := NewReferenceSlotPartitionConstraint(
		id,
		signature,
		whole,
		[]SlotKindID{left, whole},
		fixture.provenance,
	); err == nil || !strings.Contains(err.Error(), "whole") {
		t.Fatalf("whole-as-part error = %v", err)
	}
	if _, err := NewSlotCardinalityConstraint(
		id,
		signature,
		SlotKindID{},
		ExactlyOneCardinality(),
		fixture.provenance,
	); err == nil {
		t.Fatal("slot-cardinality constructor accepted an empty slot coordinate")
	}
}

func TestTypeEnvValidatesClosedReferenceConstraintAlgebra(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	relation := newReferenceConstraintRelation(t, fixture)
	matchingCardinality := mustTypedMemoryValue(NewSlotCardinalityConstraint(
		typeEnvTestConstraintID(t, "constraint:whole-cardinality"),
		relation.signature.Ref(),
		relation.whole,
		NewUnboundedCardinality(0),
		fixture.provenance,
	))
	subset := mustTypedMemoryValue(NewReferenceSlotSubsetConstraint(
		typeEnvTestConstraintID(t, "constraint:left-subset-whole"),
		relation.signature.Ref(),
		relation.left,
		relation.whole,
		fixture.provenance,
	))
	partition := mustTypedMemoryValue(NewReferenceSlotPartitionConstraint(
		typeEnvTestConstraintID(t, "constraint:whole-partition"),
		relation.signature.Ref(),
		relation.whole,
		[]SlotKindID{relation.right, relation.left},
		fixture.provenance,
	))

	environment, err := fixture.builder().
		AddRefKindDefinition(relation.otherReferenceDefinition).
		AddRelationSignature(relation.signature).
		AddConstraint(partition).
		AddConstraint(subset).
		AddConstraint(matchingCardinality).
		Build()
	if err != nil {
		t.Fatalf("TypeEnv rejected valid closed constraints: %v", err)
	}
	if len(environment.Constraints()) != 4 {
		t.Fatalf("TypeEnv retained %d constraints; want fixture + 3", len(environment.Constraints()))
	}
}

func TestTypeEnvRejectsInvalidReferenceConstraintCoordinates(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	relation := newReferenceConstraintRelation(t, fixture)
	unknownSlot := typeEnvTestSlotKindID(t, "UnknownSlot")
	cases := []struct {
		name       string
		constraint ConstraintRule
		want       string
	}{
		{
			name: "cardinality mismatch",
			constraint: mustTypedMemoryValue(NewSlotCardinalityConstraint(
				typeEnvTestConstraintID(t, "constraint:mismatched-cardinality"),
				relation.signature.Ref(),
				relation.whole,
				ExactlyOneCardinality(),
				fixture.provenance,
			)),
			want: "does not exactly match",
		},
		{
			name: "unknown slot",
			constraint: mustTypedMemoryValue(NewSlotCardinalityConstraint(
				typeEnvTestConstraintID(t, "constraint:unknown-slot"),
				relation.signature.Ref(),
				unknownSlot,
				ExactlyOneCardinality(),
				fixture.provenance,
			)),
			want: "unknown slot",
		},
		{
			name: "ByValue subset operand",
			constraint: mustTypedMemoryValue(NewReferenceSlotSubsetConstraint(
				typeEnvTestConstraintID(t, "constraint:value-subset"),
				relation.signature.Ref(),
				relation.byValue,
				relation.whole,
				fixture.provenance,
			)),
			want: "must be ByReference",
		},
		{
			name: "different exact reference target",
			constraint: mustTypedMemoryValue(NewReferenceSlotSubsetConstraint(
				typeEnvTestConstraintID(t, "constraint:different-target"),
				relation.signature.Ref(),
				relation.otherTarget,
				relation.whole,
				fixture.provenance,
			)),
			want: "one exact target",
		},
		{
			name: "ByValue partition part",
			constraint: mustTypedMemoryValue(NewReferenceSlotPartitionConstraint(
				typeEnvTestConstraintID(t, "constraint:value-partition"),
				relation.signature.Ref(),
				relation.whole,
				[]SlotKindID{relation.left, relation.byValue},
				fixture.provenance,
			)),
			want: "must be ByReference",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := fixture.builder().
				AddRefKindDefinition(relation.otherReferenceDefinition).
				AddRelationSignature(relation.signature).
				AddConstraint(test.constraint).
				Build()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Build() error = %v; want %q", err, test.want)
			}
		})
	}
}

type referenceConstraintRelationFixture struct {
	signature                RelationSignature
	otherReferenceDefinition RefKindDefinition
	whole                    SlotKindID
	left                     SlotKindID
	right                    SlotKindID
	byValue                  SlotKindID
	otherTarget              SlotKindID
}

func newReferenceConstraintRelation(
	t *testing.T,
	fixture typeEnvFixture,
) referenceConstraintRelationFixture {
	t.Helper()
	whole := typeEnvTestSlotKindID(t, "WholeSlot")
	left := typeEnvTestSlotKindID(t, "LeftSlot")
	right := typeEnvTestSlotKindID(t, "RightSlot")
	byValue := typeEnvTestSlotKindID(t, "ValueSlot")
	otherTarget := typeEnvTestSlotKindID(t, "OtherTargetSlot")
	commonTarget := mustTypedMemoryValue(NewReferenceSlotTarget(
		fixture.entityValueKind,
		fixture.entityRefKind,
	))
	claimRefID := typeEnvTestRefKindID(t, "U.ClaimGraphRef")
	claimRef := typeEnvTestRefKindRef(t, fixture.ref, claimRefID)
	claimReferenceDefinition := mustTypedMemoryValue(NewRefKindDefinition(
		claimRef,
		fixture.claimGraphValueKind,
		fixture.provenance,
	))
	claimReferenceTarget := mustTypedMemoryValue(NewReferenceSlotTarget(
		fixture.claimGraphValueKind,
		claimRef,
	))
	claimValueTarget := mustTypedMemoryValue(NewValueSlotTarget(fixture.claimGraphValueKind))
	slots := []SlotSpec{
		mustTypedMemoryValue(NewSlotSpec(whole, commonTarget, NewUnboundedCardinality(0), fixture.provenance)),
		mustTypedMemoryValue(NewSlotSpec(left, commonTarget, NewUnboundedCardinality(0), fixture.provenance)),
		mustTypedMemoryValue(NewSlotSpec(right, commonTarget, NewUnboundedCardinality(0), fixture.provenance)),
		mustTypedMemoryValue(NewSlotSpec(byValue, claimValueTarget, ExactlyOneCardinality(), fixture.provenance)),
		mustTypedMemoryValue(NewSlotSpec(otherTarget, claimReferenceTarget, NewUnboundedCardinality(0), fixture.provenance)),
	}
	ref := typeEnvTestSignatureRef(t, fixture.ref, "Haft.ReferenceConstraintRelation")
	signature := mustTypedMemoryValue(NewRelationSignature(
		ref,
		[]BoundedContextRef{fixture.primaryContext.Ref()},
		slots,
		fixture.provenance,
	))
	return referenceConstraintRelationFixture{
		signature:                signature,
		otherReferenceDefinition: claimReferenceDefinition,
		whole:                    whole,
		left:                     left,
		right:                    right,
		byValue:                  byValue,
		otherTarget:              otherTarget,
	}
}

func mustTypedMemoryValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
