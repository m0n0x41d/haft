package typedmemory

import (
	"fmt"
	"slices"
	"sort"
	"unicode/utf8"
)

const (
	valueShapeIdentityDomain = "haft.typedmemory.value-shape.v1"

	maximumValueShapeIdentityMembers        = 1 << 10
	maximumValueShapeIdentityTextBytes      = 16 << 10
	maximumValueShapeIdentityCanonicalBytes = 4 << 20
)

type canonicalValueShapeMember struct {
	name  string
	shape ValueShapeRef
}

type canonicalValueShapeIdentity struct {
	kind       ValueShapeKind
	scalarKind ScalarKind
	members    []canonicalValueShapeMember
	element    ValueShapeRef
}

// DeriveValueShapeRef is the only content-bearing ValueShapeRef constructor.
// The digest commits to the exact ShapeID and the canonical closed-algebra
// payload. Provenance is deliberately outside this identity.
func DeriveValueShapeRef(
	id ShapeID,
	shape ValueShape,
) (ValueShapeRef, error) {
	if err := validateCanonicalValueShapeID(id); err != nil {
		return ValueShapeRef{}, err
	}
	identity, err := newCanonicalValueShapeIdentity(shape)
	if err != nil {
		return ValueShapeRef{}, err
	}
	canonical, err := canonicalValueShapeIdentityBytes(id, identity)
	if err != nil {
		return ValueShapeRef{}, err
	}
	digest := digestCanonicalBytes(canonical)
	ref, err := NewValueShapeRef(id, digest)
	if err != nil {
		return ValueShapeRef{}, fmt.Errorf("derive ValueShapeRef: %w", err)
	}
	return ref, nil
}

// VerifyValueShapeRef proves that ref is the canonical content identity of
// shape under ref's exact ShapeID. It does not prove that referenced child
// shapes are present in a particular TypeEnv; that graph-closure check belongs
// to TypeEnv admission.
func VerifyValueShapeRef(ref ValueShapeRef, shape ValueShape) error {
	if err := validateCanonicalValueShapeCoordinate(ref); err != nil {
		return err
	}
	expected, err := DeriveValueShapeRef(ref.ID(), shape)
	if err != nil {
		return err
	}
	if expected != ref {
		return fmt.Errorf(
			"ValueShapeRef %q does not match canonical shape identity %q",
			ref.String(),
			expected.String(),
		)
	}
	return nil
}

// valueShapeDependencies returns the canonical set of direct child shape
// coordinates. It validates the same closed payload used by identity
// derivation, but it does not resolve those coordinates against a TypeEnv.
func valueShapeDependencies(shape ValueShape) ([]ValueShapeRef, error) {
	identity, err := newCanonicalValueShapeIdentity(shape)
	if err != nil {
		return nil, err
	}
	dependencies := directValueShapeDependencies(identity)
	return dependencies, nil
}

func newCanonicalValueShapeIdentity(
	shape ValueShape,
) (canonicalValueShapeIdentity, error) {
	if shape == nil {
		return canonicalValueShapeIdentity{}, fmt.Errorf("ValueShape is required")
	}
	switch value := shape.(type) {
	case scalarValueShape:
		return newCanonicalScalarShapeIdentity(value)
	case recordValueShape:
		return newCanonicalRecordShapeIdentity(value)
	case sumValueShape:
		return newCanonicalSumShapeIdentity(value)
	case orderedSequenceValueShape:
		return newCanonicalOrderedShapeIdentity(value)
	case unorderedSetValueShape:
		return newCanonicalUnorderedShapeIdentity(value)
	case claimGraphValueShape:
		return newCanonicalClaimGraphShapeIdentity()
	default:
		return canonicalValueShapeIdentity{}, fmt.Errorf(
			"ValueShape variant %T is outside the closed algebra",
			shape,
		)
	}
}

func newCanonicalScalarShapeIdentity(
	shape scalarValueShape,
) (canonicalValueShapeIdentity, error) {
	if !shape.scalarKind.valid() {
		return canonicalValueShapeIdentity{}, fmt.Errorf("scalar ValueShape is invalid")
	}
	return canonicalValueShapeIdentity{
		kind:       ValueShapeScalar,
		scalarKind: shape.scalarKind,
	}, nil
}

func newCanonicalRecordShapeIdentity(
	shape recordValueShape,
) (canonicalValueShapeIdentity, error) {
	if err := validateValueShapeMemberCount("record field", len(shape.fields)); err != nil {
		return canonicalValueShapeIdentity{}, err
	}
	members := make([]canonicalValueShapeMember, 0, len(shape.fields))
	for _, field := range shape.fields {
		member := canonicalValueShapeMember{
			name:  field.Name().String(),
			shape: field.Shape(),
		}
		members = append(members, member)
	}
	canonical, err := canonicalizeValueShapeMembers("record field", members)
	if err != nil {
		return canonicalValueShapeIdentity{}, err
	}
	return canonicalValueShapeIdentity{
		kind:    ValueShapeRecord,
		members: canonical,
	}, nil
}

func newCanonicalSumShapeIdentity(
	shape sumValueShape,
) (canonicalValueShapeIdentity, error) {
	if err := validateValueShapeMemberCount("sum variant", len(shape.variants)); err != nil {
		return canonicalValueShapeIdentity{}, err
	}
	members := make([]canonicalValueShapeMember, 0, len(shape.variants))
	for _, variant := range shape.variants {
		member := canonicalValueShapeMember{
			name:  variant.Name().String(),
			shape: variant.Shape(),
		}
		members = append(members, member)
	}
	canonical, err := canonicalizeValueShapeMembers("sum variant", members)
	if err != nil {
		return canonicalValueShapeIdentity{}, err
	}
	return canonicalValueShapeIdentity{
		kind:    ValueShapeSum,
		members: canonical,
	}, nil
}

func newCanonicalOrderedShapeIdentity(
	shape orderedSequenceValueShape,
) (canonicalValueShapeIdentity, error) {
	element := shape.element
	if err := validateCanonicalValueShapeCoordinate(element); err != nil {
		return canonicalValueShapeIdentity{}, fmt.Errorf(
			"ordered-sequence element: %w",
			err,
		)
	}
	return canonicalValueShapeIdentity{
		kind:    ValueShapeOrderedSequence,
		element: element,
	}, nil
}

func newCanonicalUnorderedShapeIdentity(
	shape unorderedSetValueShape,
) (canonicalValueShapeIdentity, error) {
	element := shape.element
	if err := validateCanonicalValueShapeCoordinate(element); err != nil {
		return canonicalValueShapeIdentity{}, fmt.Errorf(
			"unordered-set element: %w",
			err,
		)
	}
	return canonicalValueShapeIdentity{
		kind:    ValueShapeUnorderedSet,
		element: element,
	}, nil
}

func newCanonicalClaimGraphShapeIdentity() (canonicalValueShapeIdentity, error) {
	return canonicalValueShapeIdentity{kind: ValueShapeClaimGraph}, nil
}

func canonicalizeValueShapeMembers(
	label string,
	members []canonicalValueShapeMember,
) ([]canonicalValueShapeMember, error) {
	if err := validateValueShapeMemberCount(label, len(members)); err != nil {
		return nil, err
	}
	canonical := append([]canonicalValueShapeMember(nil), members...)
	for index, member := range canonical {
		if err := validateCanonicalValueShapeMemberName(member.name); err != nil {
			return nil, fmt.Errorf("ValueShape %s %d: %w", label, index, err)
		}
		if err := validateCanonicalValueShapeCoordinate(member.shape); err != nil {
			return nil, fmt.Errorf("ValueShape %s %q: %w", label, member.name, err)
		}
	}
	sort.Slice(canonical, func(left, right int) bool {
		return canonical[left].name < canonical[right].name
	})
	for index := 1; index < len(canonical); index++ {
		if canonical[index-1].name == canonical[index].name {
			return nil, fmt.Errorf(
				"ValueShape repeats %s %q",
				label,
				canonical[index].name,
			)
		}
	}
	return canonical, nil
}

func validateValueShapeMemberCount(label string, count int) error {
	if count == 0 {
		return fmt.Errorf("ValueShape requires at least one %s", label)
	}
	if count > maximumValueShapeIdentityMembers {
		return fmt.Errorf(
			"ValueShape %s count exceeds %d",
			label,
			maximumValueShapeIdentityMembers,
		)
	}
	return nil
}

func canonicalValueShapeIdentityBytes(
	id ShapeID,
	identity canonicalValueShapeIdentity,
) ([]byte, error) {
	expectedSize, err := canonicalValueShapeIdentitySize(id, identity)
	if err != nil {
		return nil, err
	}
	writer := newCanonicalWriter(valueShapeIdentityDomain)
	writer.addString(id.String())
	writer.addString(string(identity.kind))
	switch identity.kind {
	case ValueShapeScalar:
		writer.addString(string(identity.scalarKind))
	case ValueShapeRecord, ValueShapeSum:
		writer.addUint64(uint64(len(identity.members)))
		for _, member := range identity.members {
			writer.addString(member.name)
			writer.addString(member.shape.String())
		}
	case ValueShapeOrderedSequence, ValueShapeUnorderedSet:
		writer.addString(identity.element.String())
	case ValueShapeClaimGraph:
	default:
		return nil, fmt.Errorf("ValueShape identity kind %q is invalid", identity.kind)
	}
	canonical := writer.bytes()
	if uint64(len(canonical)) != expectedSize {
		return nil, fmt.Errorf("ValueShape identity canonical-size invariant failed")
	}
	return canonical, nil
}

func canonicalValueShapeIdentitySize(
	id ShapeID,
	identity canonicalValueShapeIdentity,
) (uint64, error) {
	size := uint64(0)
	strings := []string{
		canonicalEnvelopeDomain,
		valueShapeIdentityDomain,
		id.String(),
		string(identity.kind),
	}
	for _, value := range strings {
		var err error
		size, err = addCanonicalValueShapeIdentitySize(size, uint64(len(value)))
		if err != nil {
			return 0, err
		}
	}
	switch identity.kind {
	case ValueShapeScalar:
		return addCanonicalValueShapeIdentitySize(
			size,
			uint64(len(identity.scalarKind)),
		)
	case ValueShapeRecord, ValueShapeSum:
		var err error
		size, err = addCanonicalValueShapeIdentitySize(size, 8)
		if err != nil {
			return 0, err
		}
		for _, member := range identity.members {
			size, err = addCanonicalValueShapeIdentitySize(
				size,
				uint64(len(member.name)),
			)
			if err != nil {
				return 0, err
			}
			size, err = addCanonicalValueShapeIdentitySize(
				size,
				uint64(len(member.shape.String())),
			)
			if err != nil {
				return 0, err
			}
		}
		return size, nil
	case ValueShapeOrderedSequence, ValueShapeUnorderedSet:
		return addCanonicalValueShapeIdentitySize(
			size,
			uint64(len(identity.element.String())),
		)
	case ValueShapeClaimGraph:
		return size, nil
	default:
		return 0, fmt.Errorf("ValueShape identity kind %q is invalid", identity.kind)
	}
}

func addCanonicalValueShapeIdentitySize(
	current uint64,
	payloadSize uint64,
) (uint64, error) {
	const canonicalLengthPrefixBytes = uint64(8)
	limit := uint64(maximumValueShapeIdentityCanonicalBytes)
	if payloadSize > limit-canonicalLengthPrefixBytes {
		return 0, fmt.Errorf(
			"ValueShape identity exceeds %d canonical bytes",
			maximumValueShapeIdentityCanonicalBytes,
		)
	}
	step := canonicalLengthPrefixBytes + payloadSize
	if current > limit-step {
		return 0, fmt.Errorf(
			"ValueShape identity exceeds %d canonical bytes",
			maximumValueShapeIdentityCanonicalBytes,
		)
	}
	return current + step, nil
}

func directValueShapeDependencies(
	identity canonicalValueShapeIdentity,
) []ValueShapeRef {
	dependencies := make([]ValueShapeRef, 0, len(identity.members)+1)
	for _, member := range identity.members {
		dependencies = append(dependencies, member.shape)
	}
	switch identity.kind {
	case ValueShapeOrderedSequence, ValueShapeUnorderedSet:
		dependencies = append(dependencies, identity.element)
	}
	sort.Slice(dependencies, func(left, right int) bool {
		return dependencies[left].String() < dependencies[right].String()
	})
	dependencies = slices.Compact(dependencies)
	return dependencies
}

func validateCanonicalValueShapeID(id ShapeID) error {
	if !id.valid() {
		return fmt.Errorf("ValueShape identity requires a ShapeID")
	}
	raw := id.String()
	if !utf8.ValidString(raw) {
		return fmt.Errorf("ValueShape ShapeID must be valid UTF-8")
	}
	if len(raw) > maximumValueShapeIdentityTextBytes {
		return fmt.Errorf(
			"ValueShape ShapeID exceeds %d bytes",
			maximumValueShapeIdentityTextBytes,
		)
	}
	rebuilt, err := NewShapeID(raw)
	if err != nil {
		return fmt.Errorf("ValueShape ShapeID: %w", err)
	}
	if rebuilt != id || rebuilt.String() != raw {
		return fmt.Errorf("ValueShape ShapeID must be exact canonical UTF-8")
	}
	return nil
}

func validateCanonicalValueShapeMemberName(raw string) error {
	if !utf8.ValidString(raw) {
		return fmt.Errorf("member name must be valid UTF-8")
	}
	if len(raw) > maximumValueShapeIdentityTextBytes {
		return fmt.Errorf(
			"member name exceeds %d bytes",
			maximumValueShapeIdentityTextBytes,
		)
	}
	rebuilt, err := NewValueMemberName(raw)
	if err != nil {
		return err
	}
	if rebuilt.String() != raw {
		return fmt.Errorf("member name must be exact canonical UTF-8")
	}
	return nil
}

func validateCanonicalValueShapeCoordinate(ref ValueShapeRef) error {
	if !ref.valid() {
		return fmt.Errorf("ValueShapeRef is required")
	}
	if err := validateCanonicalValueShapeID(ref.ID()); err != nil {
		return fmt.Errorf("ValueShapeRef ID: %w", err)
	}
	digest, err := NewSHA256Digest(ref.Digest().String())
	if err != nil {
		return fmt.Errorf("ValueShapeRef digest: %w", err)
	}
	if digest != ref.Digest() {
		return fmt.Errorf("ValueShapeRef digest is not canonical")
	}
	rebuilt, err := NewValueShapeRef(ref.ID(), digest)
	if err != nil {
		return fmt.Errorf("ValueShapeRef: %w", err)
	}
	if rebuilt != ref || rebuilt.String() != ref.String() {
		return fmt.Errorf("ValueShapeRef is not canonical")
	}
	return nil
}
