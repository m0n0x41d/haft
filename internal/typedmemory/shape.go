package typedmemory

import (
	"fmt"
	"sort"
	"strings"
)

// ValueShapeKind is the closed set of structural value forms understood by
// the typed-memory core. It deliberately has no dynamic-map or "any" member.
type ValueShapeKind string

const (
	ValueShapeScalar          ValueShapeKind = "scalar"
	ValueShapeRecord          ValueShapeKind = "record"
	ValueShapeSum             ValueShapeKind = "sum"
	ValueShapeOrderedSequence ValueShapeKind = "ordered_sequence"
	ValueShapeUnorderedSet    ValueShapeKind = "unordered_set"
	ValueShapeClaimGraph      ValueShapeKind = "claim_graph"
)

// ValueShape is a sealed algebra. Callers can select one of the exported
// constructors, but cannot add an ungoverned shape variant.
type ValueShape interface {
	Kind() ValueShapeKind
	valueShapeVariant()
}

type ScalarKind string

const (
	ScalarText            ScalarKind = "text"
	ScalarBoolean         ScalarKind = "boolean"
	ScalarSignedInteger   ScalarKind = "signed_integer"
	ScalarUnsignedInteger ScalarKind = "unsigned_integer"
	ScalarBytes           ScalarKind = "bytes"
)

func (kind ScalarKind) valid() bool {
	switch kind {
	case ScalarText, ScalarBoolean, ScalarSignedInteger, ScalarUnsignedInteger, ScalarBytes:
		return true
	default:
		return false
	}
}

type ScalarValueShape interface {
	ValueShape
	ScalarKind() ScalarKind
	scalarValueShapeVariant()
}

type scalarValueShape struct {
	scalarKind ScalarKind
}

func NewScalarShape(kind ScalarKind) (ScalarValueShape, error) {
	if !kind.valid() {
		return nil, fmt.Errorf("scalar kind is not part of the closed ValueShape algebra: %q", kind)
	}
	return scalarValueShape{scalarKind: kind}, nil
}

func (shape scalarValueShape) Kind() ValueShapeKind { return ValueShapeScalar }

func (shape scalarValueShape) ScalarKind() ScalarKind { return shape.scalarKind }

func (scalarValueShape) valueShapeVariant() {}

func (scalarValueShape) scalarValueShapeVariant() {}

type ValueMemberName struct {
	value string
}

func NewValueMemberName(raw string) (ValueMemberName, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ValueMemberName{}, fmt.Errorf("value member name is required")
	}
	if strings.ContainsAny(value, "\r\n\t/\\") {
		return ValueMemberName{}, fmt.Errorf("value member name must be one line without slash or backslash")
	}
	return ValueMemberName{value: value}, nil
}

func (name ValueMemberName) String() string { return name.value }

func (name ValueMemberName) valid() bool { return name.value != "" }

type RecordFieldShape struct {
	name  ValueMemberName
	shape ValueShapeRef
}

func NewRecordFieldShape(name ValueMemberName, shape ValueShapeRef) (RecordFieldShape, error) {
	if !name.valid() {
		return RecordFieldShape{}, fmt.Errorf("record field name is required")
	}
	if !shape.valid() {
		return RecordFieldShape{}, fmt.Errorf("record field ValueShapeRef is required")
	}
	return RecordFieldShape{name: name, shape: shape}, nil
}

func (field RecordFieldShape) Name() ValueMemberName { return field.name }

func (field RecordFieldShape) Shape() ValueShapeRef { return field.shape }

type RecordValueShape interface {
	ValueShape
	Fields() []RecordFieldShape
	recordValueShapeVariant()
}

type recordValueShape struct {
	fields []RecordFieldShape
}

func NewRecordShape(fields []RecordFieldShape) (RecordValueShape, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("record shape requires at least one named field")
	}

	owned := append([]RecordFieldShape(nil), fields...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].name.String() < owned[right].name.String()
	})
	for index, field := range owned {
		if !field.name.valid() || !field.shape.valid() {
			return nil, fmt.Errorf("record field at index %d is incomplete", index)
		}
		if index > 0 && owned[index-1].name == field.name {
			return nil, fmt.Errorf("record shape contains duplicate field %q", field.name.String())
		}
	}

	return recordValueShape{fields: owned}, nil
}

func (recordValueShape) Kind() ValueShapeKind { return ValueShapeRecord }

func (shape recordValueShape) Fields() []RecordFieldShape {
	return append([]RecordFieldShape(nil), shape.fields...)
}

func (recordValueShape) valueShapeVariant() {}

func (recordValueShape) recordValueShapeVariant() {}

type SumVariantShape struct {
	name  ValueMemberName
	shape ValueShapeRef
}

func NewSumVariantShape(name ValueMemberName, shape ValueShapeRef) (SumVariantShape, error) {
	if !name.valid() {
		return SumVariantShape{}, fmt.Errorf("sum variant name is required")
	}
	if !shape.valid() {
		return SumVariantShape{}, fmt.Errorf("sum variant ValueShapeRef is required")
	}
	return SumVariantShape{name: name, shape: shape}, nil
}

func (variant SumVariantShape) Name() ValueMemberName { return variant.name }

func (variant SumVariantShape) Shape() ValueShapeRef { return variant.shape }

type SumValueShape interface {
	ValueShape
	Variants() []SumVariantShape
	sumValueShapeVariant()
}

type sumValueShape struct {
	variants []SumVariantShape
}

func NewSumShape(variants []SumVariantShape) (SumValueShape, error) {
	if len(variants) == 0 {
		return nil, fmt.Errorf("sum shape requires at least one named variant")
	}

	owned := append([]SumVariantShape(nil), variants...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].name.String() < owned[right].name.String()
	})
	for index, variant := range owned {
		if !variant.name.valid() || !variant.shape.valid() {
			return nil, fmt.Errorf("sum variant at index %d is incomplete", index)
		}
		if index > 0 && owned[index-1].name == variant.name {
			return nil, fmt.Errorf("sum shape contains duplicate variant %q", variant.name.String())
		}
	}

	return sumValueShape{variants: owned}, nil
}

func (sumValueShape) Kind() ValueShapeKind { return ValueShapeSum }

func (shape sumValueShape) Variants() []SumVariantShape {
	return append([]SumVariantShape(nil), shape.variants...)
}

func (sumValueShape) valueShapeVariant() {}

func (sumValueShape) sumValueShapeVariant() {}

type OrderedSequenceValueShape interface {
	ValueShape
	ElementShape() ValueShapeRef
	orderedSequenceValueShapeVariant()
}

type orderedSequenceValueShape struct {
	element ValueShapeRef
}

func NewOrderedSequenceShape(element ValueShapeRef) (OrderedSequenceValueShape, error) {
	if !element.valid() {
		return nil, fmt.Errorf("ordered-sequence element ValueShapeRef is required")
	}
	return orderedSequenceValueShape{element: element}, nil
}

func (orderedSequenceValueShape) Kind() ValueShapeKind { return ValueShapeOrderedSequence }

func (shape orderedSequenceValueShape) ElementShape() ValueShapeRef { return shape.element }

func (orderedSequenceValueShape) valueShapeVariant() {}

func (orderedSequenceValueShape) orderedSequenceValueShapeVariant() {}

type UnorderedSetValueShape interface {
	ValueShape
	ElementShape() ValueShapeRef
	unorderedSetValueShapeVariant()
}

type unorderedSetValueShape struct {
	element ValueShapeRef
}

func NewUnorderedSetShape(element ValueShapeRef) (UnorderedSetValueShape, error) {
	if !element.valid() {
		return nil, fmt.Errorf("unordered-set element ValueShapeRef is required")
	}
	return unorderedSetValueShape{element: element}, nil
}

func (unorderedSetValueShape) Kind() ValueShapeKind { return ValueShapeUnorderedSet }

func (shape unorderedSetValueShape) ElementShape() ValueShapeRef { return shape.element }

func (unorderedSetValueShape) valueShapeVariant() {}

func (unorderedSetValueShape) unorderedSetValueShapeVariant() {}

type ClaimGraphValueShape interface {
	ValueShape
	claimGraphValueShapeVariant()
}

type claimGraphValueShape struct{}

func NewClaimGraphShape() ClaimGraphValueShape { return claimGraphValueShape{} }

func (claimGraphValueShape) Kind() ValueShapeKind { return ValueShapeClaimGraph }

func (claimGraphValueShape) valueShapeVariant() {}

func (claimGraphValueShape) claimGraphValueShapeVariant() {}
