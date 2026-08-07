package typedmemory

import "testing"

func TestValueShapeAlgebraNormalizesNamedMembers(t *testing.T) {
	shapeA := valueTestShapeRef(t, "shape.A", 'a')
	shapeB := valueTestShapeRef(t, "shape.B", 'b')
	nameA := valueTestMemberName(t, "a")
	nameB := valueTestMemberName(t, "b")
	fieldB, err := NewRecordFieldShape(nameB, shapeB)
	if err != nil {
		t.Fatalf("NewRecordFieldShape(b): %v", err)
	}
	fieldA, err := NewRecordFieldShape(nameA, shapeA)
	if err != nil {
		t.Fatalf("NewRecordFieldShape(a): %v", err)
	}
	input := []RecordFieldShape{fieldB, fieldA}

	shape, err := NewRecordShape(input)
	if err != nil {
		t.Fatalf("NewRecordShape: %v", err)
	}
	input[0] = fieldA
	fields := shape.Fields()
	if got := fields[0].Name().String(); got != "a" {
		t.Fatalf("first normalized field = %q, want a", got)
	}
	fields[0] = fieldB
	if got := shape.Fields()[0].Name().String(); got != "a" {
		t.Fatalf("shape leaked mutable field slice, first = %q", got)
	}
}

func TestValueShapeAlgebraRejectsDuplicateMembersAndUnknownScalar(t *testing.T) {
	shapeRef := valueTestShapeRef(t, "shape.A", 'a')
	name := valueTestMemberName(t, "same")
	field, err := NewRecordFieldShape(name, shapeRef)
	if err != nil {
		t.Fatalf("NewRecordFieldShape: %v", err)
	}
	if _, err := NewRecordShape([]RecordFieldShape{field, field}); err == nil {
		t.Fatal("NewRecordShape accepted duplicate member")
	}
	if _, err := NewScalarShape(ScalarKind("dynamic")); err == nil {
		t.Fatal("NewScalarShape accepted an ungoverned scalar kind")
	}
}

func TestValueShapeAlgebraContainsOnlyClosedVariants(t *testing.T) {
	scalar, err := NewScalarShape(ScalarText)
	if err != nil {
		t.Fatalf("NewScalarShape: %v", err)
	}
	element := valueTestShapeRef(t, "shape.element", 'e')
	ordered, err := NewOrderedSequenceShape(element)
	if err != nil {
		t.Fatalf("NewOrderedSequenceShape: %v", err)
	}
	unordered, err := NewUnorderedSetShape(element)
	if err != nil {
		t.Fatalf("NewUnorderedSetShape: %v", err)
	}
	claimGraph := NewClaimGraphShape()

	kinds := []ValueShapeKind{
		scalar.Kind(),
		ordered.Kind(),
		unordered.Kind(),
		claimGraph.Kind(),
	}
	want := []ValueShapeKind{
		ValueShapeScalar,
		ValueShapeOrderedSequence,
		ValueShapeUnorderedSet,
		ValueShapeClaimGraph,
	}
	for index := range want {
		if kinds[index] != want[index] {
			t.Fatalf("kind[%d] = %q, want %q", index, kinds[index], want[index])
		}
	}
}
