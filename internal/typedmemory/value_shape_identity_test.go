package typedmemory

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDeriveValueShapeRefCoversClosedAlgebra(t *testing.T) {
	t.Parallel()

	textShape := mustValueShapeIdentityScalar(t, ScalarText)
	booleanShape := mustValueShapeIdentityScalar(t, ScalarBoolean)
	textRef := mustDerivedValueShapeIdentityRef(t, "test.ChildText", textShape)
	booleanRef := mustDerivedValueShapeIdentityRef(t, "test.ChildBoolean", booleanShape)
	recordShape := mustValueShapeIdentityRecord(t, []RecordFieldShape{
		mustValueShapeIdentityRecordField(t, "text", textRef),
		mustValueShapeIdentityRecordField(t, "flag", booleanRef),
	})
	sumShape := mustValueShapeIdentitySum(t, []SumVariantShape{
		mustValueShapeIdentitySumVariant(t, "some", textRef),
		mustValueShapeIdentitySumVariant(t, "none", booleanRef),
	})
	orderedShape := mustValueShapeIdentityOrdered(t, textRef)
	unorderedShape := mustValueShapeIdentityUnordered(t, textRef)

	testCases := []struct {
		name           string
		shape          ValueShape
		expectedDigest string
	}{
		{
			name:           "scalar-text",
			shape:          textShape,
			expectedDigest: "sha256:a31ad416aa411617d5859ea365f36329f433ad8e5cd1b986a67db69836ee5917",
		},
		{
			name:           "scalar-boolean",
			shape:          booleanShape,
			expectedDigest: "sha256:0b4fd59b01801d16c6e6406a6e6d88ca8e4814880d1bc4a53211af3d858f1197",
		},
		{
			name:           "scalar-signed-integer",
			shape:          mustValueShapeIdentityScalar(t, ScalarSignedInteger),
			expectedDigest: "sha256:0621b8d79c01430ef757bd7d50585dce93a1641a75b3b5788a317e83b73887fa",
		},
		{
			name:           "scalar-unsigned-integer",
			shape:          mustValueShapeIdentityScalar(t, ScalarUnsignedInteger),
			expectedDigest: "sha256:b81de5d0c75c1c5fbcd066cd0b8d16f9e1fde62333f3e78e6bfec4485ab0e9cc",
		},
		{
			name:           "scalar-bytes",
			shape:          mustValueShapeIdentityScalar(t, ScalarBytes),
			expectedDigest: "sha256:cd99cdf5c0c8823cb4d2681509b121f4ac90f68c2e7f3b2115dd4e487d5fd748",
		},
		{
			name:           "record",
			shape:          recordShape,
			expectedDigest: "sha256:a285f6df154eb4d65c1d21780911c48e464bfd8e87a99c59c59c47a4e2030f92",
		},
		{
			name:           "sum",
			shape:          sumShape,
			expectedDigest: "sha256:5975a5ff7b18c8e2a107f028dd55b29d5620499ed5129a08382560cca9fb321a",
		},
		{
			name:           "ordered-sequence",
			shape:          orderedShape,
			expectedDigest: "sha256:4952aefafa4bba9f389576ac7c9219079a9c88511bc194ff88a5b498059465fe",
		},
		{
			name:           "unordered-set",
			shape:          unorderedShape,
			expectedDigest: "sha256:5c93088aeba767972493dcc542e5346e0b8f5751da62a07501d574bf02f2ac9e",
		},
		{
			name:           "claim-graph",
			shape:          NewClaimGraphShape(),
			expectedDigest: "sha256:92dd219146a4ed28af7b0b1ed7a468b80a7614f08b691994648b4eece35748e5",
		},
	}

	id := mustValueShapeIdentityID(t, "test.AllClosedVariants")
	seen := make(map[SHA256Digest]string, len(testCases))
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ref, err := DeriveValueShapeRef(id, testCase.shape)
			if err != nil {
				t.Fatalf("DeriveValueShapeRef(): %v", err)
			}
			if ref.ID() != id {
				t.Fatalf("derived ShapeID = %q, want %q", ref.ID().String(), id.String())
			}
			if ref.Digest().String() != testCase.expectedDigest {
				t.Fatalf(
					"derived digest = %q, want golden %q",
					ref.Digest().String(),
					testCase.expectedDigest,
				)
			}
			if err := VerifyValueShapeRef(ref, testCase.shape); err != nil {
				t.Fatalf("VerifyValueShapeRef(): %v", err)
			}
			second, err := DeriveValueShapeRef(id, testCase.shape)
			if err != nil {
				t.Fatalf("second DeriveValueShapeRef(): %v", err)
			}
			if second != ref {
				t.Fatalf("derivation is not deterministic: %q != %q", second.String(), ref.String())
			}
			if previous, found := seen[ref.Digest()]; found {
				t.Fatalf("shape digest collides with %s", previous)
			}
			seen[ref.Digest()] = testCase.name
		})
	}
}

func TestDeriveValueShapeRefCanonicalizesRecordAndSumOrder(t *testing.T) {
	t.Parallel()

	left := mustDerivedValueShapeIdentityRef(
		t,
		"test.Left",
		mustValueShapeIdentityScalar(t, ScalarText),
	)
	right := mustDerivedValueShapeIdentityRef(
		t,
		"test.Right",
		mustValueShapeIdentityScalar(t, ScalarBoolean),
	)
	fieldA := mustValueShapeIdentityRecordField(t, "a", left)
	fieldB := mustValueShapeIdentityRecordField(t, "b", right)
	variantA := mustValueShapeIdentitySumVariant(t, "a", left)
	variantB := mustValueShapeIdentitySumVariant(t, "b", right)

	recordID := mustValueShapeIdentityID(t, "test.OrderedRecord")
	recordForward := mustValueShapeIdentityRecord(t, []RecordFieldShape{fieldA, fieldB})
	recordReverse := recordValueShape{fields: []RecordFieldShape{fieldB, fieldA}}
	forwardRecordRef := mustDerivedValueShapeIdentityRefFromID(t, recordID, recordForward)
	reverseRecordRef := mustDerivedValueShapeIdentityRefFromID(t, recordID, recordReverse)
	if forwardRecordRef != reverseRecordRef {
		t.Fatalf("record source order changed identity: %q != %q", forwardRecordRef.String(), reverseRecordRef.String())
	}

	sumID := mustValueShapeIdentityID(t, "test.OrderedSum")
	sumForward := mustValueShapeIdentitySum(t, []SumVariantShape{variantA, variantB})
	sumReverse := sumValueShape{variants: []SumVariantShape{variantB, variantA}}
	forwardSumRef := mustDerivedValueShapeIdentityRefFromID(t, sumID, sumForward)
	reverseSumRef := mustDerivedValueShapeIdentityRefFromID(t, sumID, sumReverse)
	if forwardSumRef != reverseSumRef {
		t.Fatalf("sum source order changed identity: %q != %q", forwardSumRef.String(), reverseSumRef.String())
	}
}

func TestDeriveValueShapeRefIsSensitiveToIdentityAndPayloadMutation(t *testing.T) {
	t.Parallel()

	textRef := mustDerivedValueShapeIdentityRef(
		t,
		"test.Text",
		mustValueShapeIdentityScalar(t, ScalarText),
	)
	booleanRef := mustDerivedValueShapeIdentityRef(
		t,
		"test.Boolean",
		mustValueShapeIdentityScalar(t, ScalarBoolean),
	)
	baseID := mustValueShapeIdentityID(t, "test.Sensitive")
	baseShape := mustValueShapeIdentityRecord(t, []RecordFieldShape{
		mustValueShapeIdentityRecordField(t, "value", textRef),
	})
	base := mustDerivedValueShapeIdentityRefFromID(t, baseID, baseShape)

	mutations := []struct {
		name  string
		id    ShapeID
		shape ValueShape
	}{
		{
			name:  "shape-id",
			id:    mustValueShapeIdentityID(t, "test.SensitiveRenamed"),
			shape: baseShape,
		},
		{
			name: "member-name",
			id:   baseID,
			shape: mustValueShapeIdentityRecord(t, []RecordFieldShape{
				mustValueShapeIdentityRecordField(t, "renamed", textRef),
			}),
		},
		{
			name: "child-ref",
			id:   baseID,
			shape: mustValueShapeIdentityRecord(t, []RecordFieldShape{
				mustValueShapeIdentityRecordField(t, "value", booleanRef),
			}),
		},
		{
			name:  "record-to-sum-kind",
			id:    baseID,
			shape: mustValueShapeIdentitySum(t, []SumVariantShape{mustValueShapeIdentitySumVariant(t, "value", textRef)}),
		},
		{
			name:  "ordered-to-unordered-kind",
			id:    baseID,
			shape: mustValueShapeIdentityUnordered(t, textRef),
		},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			mutated := mustDerivedValueShapeIdentityRefFromID(t, mutation.id, mutation.shape)
			if mutated.Digest() == base.Digest() {
				t.Fatalf("mutation did not change digest %q", base.Digest().String())
			}
		})
	}

	ordered := mustDerivedValueShapeIdentityRefFromID(
		t,
		baseID,
		mustValueShapeIdentityOrdered(t, textRef),
	)
	unordered := mustDerivedValueShapeIdentityRefFromID(
		t,
		baseID,
		mustValueShapeIdentityUnordered(t, textRef),
	)
	if ordered.Digest() == unordered.Digest() {
		t.Fatal("ordered and unordered shapes have the same digest")
	}

	if err := VerifyValueShapeRef(base, mustValueShapeIdentityScalar(t, ScalarText)); err == nil {
		t.Fatal("VerifyValueShapeRef accepted a different payload")
	}
	wrongDigest := mustValueShapeIdentityDigest(t, 0xa7)
	wrongRef, err := NewValueShapeRef(base.ID(), wrongDigest)
	if err != nil {
		t.Fatalf("NewValueShapeRef(): %v", err)
	}
	if err := VerifyValueShapeRef(wrongRef, baseShape); err == nil {
		t.Fatal("VerifyValueShapeRef accepted a caller-supplied unrelated digest")
	}
}

func TestValueShapeDependenciesAreCanonicalDirectCoordinates(t *testing.T) {
	t.Parallel()

	a := mustDerivedValueShapeIdentityRef(
		t,
		"test.DependencyA",
		mustValueShapeIdentityScalar(t, ScalarText),
	)
	z := mustDerivedValueShapeIdentityRef(
		t,
		"test.DependencyZ",
		mustValueShapeIdentityScalar(t, ScalarBoolean),
	)
	record := recordValueShape{fields: []RecordFieldShape{
		mustValueShapeIdentityRecordField(t, "z", z),
		mustValueShapeIdentityRecordField(t, "a", a),
		mustValueShapeIdentityRecordField(t, "z-again", z),
	}}

	dependencies, err := valueShapeDependencies(record)
	if err != nil {
		t.Fatalf("valueShapeDependencies(): %v", err)
	}
	expected := []ValueShapeRef{a, z}
	if !slices.Equal(dependencies, expected) {
		t.Fatalf("dependencies = %#v, want %#v", dependencies, expected)
	}
	dependencies[0] = z
	reloaded, err := valueShapeDependencies(record)
	if err != nil {
		t.Fatalf("second valueShapeDependencies(): %v", err)
	}
	if !slices.Equal(reloaded, expected) {
		t.Fatal("caller mutation changed dependency extraction")
	}

	leafShapes := []ValueShape{
		mustValueShapeIdentityScalar(t, ScalarBytes),
		NewClaimGraphShape(),
	}
	for _, shape := range leafShapes {
		leafDependencies, err := valueShapeDependencies(shape)
		if err != nil {
			t.Fatalf("leaf valueShapeDependencies(): %v", err)
		}
		if len(leafDependencies) != 0 {
			t.Fatalf("leaf dependencies = %d, want 0", len(leafDependencies))
		}
	}

	for _, shape := range []ValueShape{
		mustValueShapeIdentityOrdered(t, a),
		mustValueShapeIdentityUnordered(t, a),
	} {
		childDependencies, err := valueShapeDependencies(shape)
		if err != nil {
			t.Fatalf("child valueShapeDependencies(): %v", err)
		}
		if !slices.Equal(childDependencies, []ValueShapeRef{a}) {
			t.Fatalf("child dependencies = %#v, want %#v", childDependencies, []ValueShapeRef{a})
		}
	}
}

func TestDeriveValueShapeRefRejectsNonCanonicalOrUnboundedInput(t *testing.T) {
	t.Parallel()

	validShape := mustValueShapeIdentityScalar(t, ScalarText)
	validChild := mustDerivedValueShapeIdentityRef(t, "test.ValidChild", validShape)

	testCases := []struct {
		name  string
		id    ShapeID
		shape ValueShape
		want  string
	}{
		{
			name:  "missing-shape",
			id:    mustValueShapeIdentityID(t, "test.MissingShape"),
			shape: nil,
			want:  "ValueShape is required",
		},
		{
			name:  "invalid-utf8-shape-id",
			id:    ShapeID{value: string([]byte{0xff})},
			shape: validShape,
			want:  "ShapeID must be valid UTF-8",
		},
		{
			name: "oversized-shape-id",
			id: ShapeID{
				value: strings.Repeat("x", maximumValueShapeIdentityTextBytes+1),
			},
			shape: validShape,
			want:  "ShapeID exceeds",
		},
		{
			name:  "noncanonical-shape-id",
			id:    ShapeID{value: " test.NonCanonical"},
			shape: validShape,
			want:  "ShapeID must be exact canonical UTF-8",
		},
		{
			name: "invalid-utf8-member",
			id:   mustValueShapeIdentityID(t, "test.InvalidMember"),
			shape: recordValueShape{fields: []RecordFieldShape{{
				name:  ValueMemberName{value: string([]byte{0xff})},
				shape: validChild,
			}}},
			want: "member name must be valid UTF-8",
		},
		{
			name: "oversized-member",
			id:   mustValueShapeIdentityID(t, "test.OversizedMember"),
			shape: recordValueShape{fields: []RecordFieldShape{{
				name: ValueMemberName{
					value: strings.Repeat("x", maximumValueShapeIdentityTextBytes+1),
				},
				shape: validChild,
			}}},
			want: "member name exceeds",
		},
		{
			name: "noncanonical-member",
			id:   mustValueShapeIdentityID(t, "test.NonCanonicalMember"),
			shape: recordValueShape{fields: []RecordFieldShape{{
				name:  ValueMemberName{value: " member"},
				shape: validChild,
			}}},
			want: "member name must be exact canonical UTF-8",
		},
		{
			name: "invalid-child-coordinate",
			id:   mustValueShapeIdentityID(t, "test.InvalidChild"),
			shape: recordValueShape{fields: []RecordFieldShape{{
				name: ValueMemberName{value: "child"},
				shape: ValueShapeRef{
					id:     ShapeID{value: string([]byte{0xff})},
					digest: mustValueShapeIdentityDigest(t, 0x74),
				},
			}}},
			want: "ValueShapeRef ID",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := DeriveValueShapeRef(testCase.id, testCase.shape)
			if err == nil {
				t.Fatal("DeriveValueShapeRef accepted invalid input")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), testCase.want)
			}
		})
	}
}

func TestDeriveValueShapeRefPreservesExactUTF8Bytes(t *testing.T) {
	t.Parallel()

	child := mustDerivedValueShapeIdentityRef(
		t,
		"test.ДочерняяФорма",
		mustValueShapeIdentityScalar(t, ScalarText),
	)
	id := mustValueShapeIdentityID(t, "test.ЮникодФорма")
	exactBoundaryName := strings.Repeat("é", maximumValueShapeIdentityTextBytes/2)
	if len(exactBoundaryName) != maximumValueShapeIdentityTextBytes {
		t.Fatalf("boundary UTF-8 bytes = %d", len(exactBoundaryName))
	}
	if utf8.RuneCountInString(exactBoundaryName) != maximumValueShapeIdentityTextBytes/2 {
		t.Fatalf("boundary UTF-8 rune count is unexpected")
	}
	boundaryShape := recordValueShape{fields: []RecordFieldShape{{
		name:  ValueMemberName{value: exactBoundaryName},
		shape: child,
	}}}
	if _, err := DeriveValueShapeRef(id, boundaryShape); err != nil {
		t.Fatalf("DeriveValueShapeRef rejected exact byte boundary: %v", err)
	}

	overBoundaryName := exactBoundaryName + "é"
	overBoundaryShape := recordValueShape{fields: []RecordFieldShape{{
		name:  ValueMemberName{value: overBoundaryName},
		shape: child,
	}}}
	if _, err := DeriveValueShapeRef(id, overBoundaryShape); err == nil {
		t.Fatal("DeriveValueShapeRef accepted a multibyte name over the byte limit")
	}

	precomposed := recordValueShape{fields: []RecordFieldShape{{
		name:  ValueMemberName{value: "é"},
		shape: child,
	}}}
	decomposed := recordValueShape{fields: []RecordFieldShape{{
		name:  ValueMemberName{value: "e\u0301"},
		shape: child,
	}}}
	precomposedRef := mustDerivedValueShapeIdentityRefFromID(t, id, precomposed)
	decomposedRef := mustDerivedValueShapeIdentityRefFromID(t, id, decomposed)
	if precomposedRef == decomposedRef {
		t.Fatal("canonically equivalent but byte-distinct UTF-8 names collapsed")
	}

	fieldPrecomposed := RecordFieldShape{
		name:  ValueMemberName{value: "é"},
		shape: child,
	}
	fieldDecomposed := RecordFieldShape{
		name:  ValueMemberName{value: "e\u0301"},
		shape: child,
	}
	forward := recordValueShape{fields: []RecordFieldShape{
		fieldPrecomposed,
		fieldDecomposed,
	}}
	reverse := recordValueShape{fields: []RecordFieldShape{
		fieldDecomposed,
		fieldPrecomposed,
	}}
	forwardRef := mustDerivedValueShapeIdentityRefFromID(t, id, forward)
	reverseRef := mustDerivedValueShapeIdentityRefFromID(t, id, reverse)
	if forwardRef != reverseRef {
		t.Fatal("exact UTF-8 byte ordering depends on source order")
	}
}

func TestDeriveValueShapeRefEnforcesMemberAndCanonicalByteBounds(t *testing.T) {
	validChild := mustDerivedValueShapeIdentityRef(
		t,
		"test.BoundedChild",
		mustValueShapeIdentityScalar(t, ScalarText),
	)
	id := mustValueShapeIdentityID(t, "test.BoundedRecord")

	tooMany := make([]RecordFieldShape, 0, maximumValueShapeIdentityMembers+1)
	for index := 0; index <= maximumValueShapeIdentityMembers; index++ {
		name := ValueMemberName{value: fmt.Sprintf("field-%04d", index)}
		field := RecordFieldShape{name: name, shape: validChild}
		tooMany = append(tooMany, field)
	}
	_, err := DeriveValueShapeRef(id, recordValueShape{fields: tooMany})
	if err == nil || !strings.Contains(err.Error(), "count exceeds") {
		t.Fatalf("member-count error = %v", err)
	}

	nameSuffix := strings.Repeat("x", maximumValueShapeIdentityTextBytes-16)
	memberCount := maximumValueShapeIdentityCanonicalBytes/len(nameSuffix) + 8
	large := make([]RecordFieldShape, 0, memberCount)
	for index := 0; index < memberCount; index++ {
		name := fmt.Sprintf("field-%04d-%s", index, nameSuffix)
		field := RecordFieldShape{
			name:  ValueMemberName{value: name},
			shape: validChild,
		}
		large = append(large, field)
	}
	_, err = DeriveValueShapeRef(id, recordValueShape{fields: large})
	if err == nil || !strings.Contains(err.Error(), "canonical bytes") {
		t.Fatalf("canonical-byte error = %v", err)
	}
}

func mustValueShapeIdentityID(t *testing.T, raw string) ShapeID {
	t.Helper()
	id, err := NewShapeID(raw)
	if err != nil {
		t.Fatalf("NewShapeID(%q): %v", raw, err)
	}
	return id
}

func mustValueShapeIdentityDigest(t *testing.T, fill byte) SHA256Digest {
	t.Helper()
	raw := fmt.Sprintf("sha256:%064x", fill)
	digest, err := NewSHA256Digest(raw)
	if err != nil {
		t.Fatalf("NewSHA256Digest(%q): %v", raw, err)
	}
	return digest
}

func mustValueShapeIdentityScalar(t *testing.T, kind ScalarKind) ScalarValueShape {
	t.Helper()
	shape, err := NewScalarShape(kind)
	if err != nil {
		t.Fatalf("NewScalarShape(%q): %v", kind, err)
	}
	return shape
}

func mustValueShapeIdentityRecordField(
	t *testing.T,
	nameRaw string,
	shape ValueShapeRef,
) RecordFieldShape {
	t.Helper()
	name, err := NewValueMemberName(nameRaw)
	if err != nil {
		t.Fatalf("NewValueMemberName(%q): %v", nameRaw, err)
	}
	field, err := NewRecordFieldShape(name, shape)
	if err != nil {
		t.Fatalf("NewRecordFieldShape(%q): %v", nameRaw, err)
	}
	return field
}

func mustValueShapeIdentitySumVariant(
	t *testing.T,
	nameRaw string,
	shape ValueShapeRef,
) SumVariantShape {
	t.Helper()
	name, err := NewValueMemberName(nameRaw)
	if err != nil {
		t.Fatalf("NewValueMemberName(%q): %v", nameRaw, err)
	}
	variant, err := NewSumVariantShape(name, shape)
	if err != nil {
		t.Fatalf("NewSumVariantShape(%q): %v", nameRaw, err)
	}
	return variant
}

func mustValueShapeIdentityRecord(
	t *testing.T,
	fields []RecordFieldShape,
) RecordValueShape {
	t.Helper()
	shape, err := NewRecordShape(fields)
	if err != nil {
		t.Fatalf("NewRecordShape(): %v", err)
	}
	return shape
}

func mustValueShapeIdentitySum(
	t *testing.T,
	variants []SumVariantShape,
) SumValueShape {
	t.Helper()
	shape, err := NewSumShape(variants)
	if err != nil {
		t.Fatalf("NewSumShape(): %v", err)
	}
	return shape
}

func mustValueShapeIdentityOrdered(
	t *testing.T,
	element ValueShapeRef,
) OrderedSequenceValueShape {
	t.Helper()
	shape, err := NewOrderedSequenceShape(element)
	if err != nil {
		t.Fatalf("NewOrderedSequenceShape(): %v", err)
	}
	return shape
}

func mustValueShapeIdentityUnordered(
	t *testing.T,
	element ValueShapeRef,
) UnorderedSetValueShape {
	t.Helper()
	shape, err := NewUnorderedSetShape(element)
	if err != nil {
		t.Fatalf("NewUnorderedSetShape(): %v", err)
	}
	return shape
}

func mustDerivedValueShapeIdentityRef(
	t *testing.T,
	idRaw string,
	shape ValueShape,
) ValueShapeRef {
	t.Helper()
	id := mustValueShapeIdentityID(t, idRaw)
	return mustDerivedValueShapeIdentityRefFromID(t, id, shape)
}

func mustDerivedValueShapeIdentityRefFromID(
	t *testing.T,
	id ShapeID,
	shape ValueShape,
) ValueShapeRef {
	t.Helper()
	ref, err := DeriveValueShapeRef(id, shape)
	if err != nil {
		t.Fatalf("DeriveValueShapeRef(%q): %v", id.String(), err)
	}
	return ref
}
