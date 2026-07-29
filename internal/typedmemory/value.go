package typedmemory

import (
	"fmt"
	"sort"
)

type TypedValueKind string

const (
	TypedValueScalar          TypedValueKind = "scalar"
	TypedValueRecord          TypedValueKind = "record"
	TypedValueSum             TypedValueKind = "sum"
	TypedValueOrderedSequence TypedValueKind = "ordered_sequence"
	TypedValueUnorderedSet    TypedValueKind = "unordered_set"
	TypedValueClaimGraph      TypedValueKind = "claim_graph"
)

// TypedValue is the closed in-memory value algebra accepted by codecs. It has
// no map[string]any, raw JSON, or caller-defined variant.
type TypedValue interface {
	Kind() TypedValueKind
	typedValueVariant()
}

type ScalarTypedValue interface {
	TypedValue
	ScalarKind() ScalarKind
	Text() (string, bool)
	Boolean() (bool, bool)
	SignedInteger() (int64, bool)
	UnsignedInteger() (uint64, bool)
	Bytes() ([]byte, bool)
	scalarTypedValueVariant()
}

type scalarTypedValue struct {
	kind          ScalarKind
	text          string
	boolean       bool
	signedInteger int64
	unsignedInt   uint64
	bytes         []byte
}

func NewTextValue(value string) ScalarTypedValue {
	return scalarTypedValue{kind: ScalarText, text: value}
}

func NewBooleanValue(value bool) ScalarTypedValue {
	return scalarTypedValue{kind: ScalarBoolean, boolean: value}
}

func NewSignedIntegerValue(value int64) ScalarTypedValue {
	return scalarTypedValue{kind: ScalarSignedInteger, signedInteger: value}
}

func NewUnsignedIntegerValue(value uint64) ScalarTypedValue {
	return scalarTypedValue{kind: ScalarUnsignedInteger, unsignedInt: value}
}

func NewBytesValue(value []byte) ScalarTypedValue {
	return scalarTypedValue{kind: ScalarBytes, bytes: append([]byte(nil), value...)}
}

func (scalarTypedValue) Kind() TypedValueKind { return TypedValueScalar }

func (value scalarTypedValue) ScalarKind() ScalarKind { return value.kind }

func (value scalarTypedValue) Text() (string, bool) {
	return value.text, value.kind == ScalarText
}

func (value scalarTypedValue) Boolean() (bool, bool) {
	return value.boolean, value.kind == ScalarBoolean
}

func (value scalarTypedValue) SignedInteger() (int64, bool) {
	return value.signedInteger, value.kind == ScalarSignedInteger
}

func (value scalarTypedValue) UnsignedInteger() (uint64, bool) {
	return value.unsignedInt, value.kind == ScalarUnsignedInteger
}

func (value scalarTypedValue) Bytes() ([]byte, bool) {
	return append([]byte(nil), value.bytes...), value.kind == ScalarBytes
}

func (scalarTypedValue) typedValueVariant() {}

func (scalarTypedValue) scalarTypedValueVariant() {}

type RecordFieldValue struct {
	name  ValueMemberName
	value TypedValue
}

func NewRecordFieldValue(name ValueMemberName, value TypedValue) (RecordFieldValue, error) {
	if !name.valid() {
		return RecordFieldValue{}, fmt.Errorf("record value field name is required")
	}
	if !validTypedValue(value) {
		return RecordFieldValue{}, fmt.Errorf("record field %q requires a valid closed TypedValue", name.String())
	}
	return RecordFieldValue{name: name, value: value}, nil
}

func (field RecordFieldValue) Name() ValueMemberName { return field.name }

func (field RecordFieldValue) Value() TypedValue { return field.value }

type RecordTypedValue interface {
	TypedValue
	Fields() []RecordFieldValue
	recordTypedValueVariant()
}

type recordTypedValue struct {
	fields []RecordFieldValue
}

func NewRecordValue(fields []RecordFieldValue) (RecordTypedValue, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("record value requires at least one named field")
	}

	owned := append([]RecordFieldValue(nil), fields...)
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].name.String() < owned[right].name.String()
	})
	for index, field := range owned {
		if !field.name.valid() || !validTypedValue(field.value) {
			return nil, fmt.Errorf("record value field at index %d is incomplete", index)
		}
		if index > 0 && owned[index-1].name == field.name {
			return nil, fmt.Errorf("record value contains duplicate field %q", field.name.String())
		}
	}

	return recordTypedValue{fields: owned}, nil
}

func (recordTypedValue) Kind() TypedValueKind { return TypedValueRecord }

func (value recordTypedValue) Fields() []RecordFieldValue {
	return append([]RecordFieldValue(nil), value.fields...)
}

func (recordTypedValue) typedValueVariant() {}

func (recordTypedValue) recordTypedValueVariant() {}

type SumTypedValue interface {
	TypedValue
	Variant() ValueMemberName
	Value() TypedValue
	sumTypedValueVariant()
}

type sumTypedValue struct {
	variant ValueMemberName
	value   TypedValue
}

func NewSumValue(variant ValueMemberName, value TypedValue) (SumTypedValue, error) {
	if !variant.valid() {
		return nil, fmt.Errorf("sum value variant is required")
	}
	if !validTypedValue(value) {
		return nil, fmt.Errorf("sum variant %q requires a valid closed TypedValue", variant.String())
	}
	return sumTypedValue{variant: variant, value: value}, nil
}

func (sumTypedValue) Kind() TypedValueKind { return TypedValueSum }

func (value sumTypedValue) Variant() ValueMemberName { return value.variant }

func (value sumTypedValue) Value() TypedValue { return value.value }

func (sumTypedValue) typedValueVariant() {}

func (sumTypedValue) sumTypedValueVariant() {}

type OrderedSequenceTypedValue interface {
	TypedValue
	Items() []TypedValue
	orderedSequenceTypedValueVariant()
}

type orderedSequenceTypedValue struct {
	items []TypedValue
}

func NewOrderedSequenceValue(items []TypedValue) (OrderedSequenceTypedValue, error) {
	owned, err := copyValidTypedValues("ordered-sequence", items)
	if err != nil {
		return nil, err
	}
	return orderedSequenceTypedValue{items: owned}, nil
}

func (orderedSequenceTypedValue) Kind() TypedValueKind { return TypedValueOrderedSequence }

func (value orderedSequenceTypedValue) Items() []TypedValue {
	return append([]TypedValue(nil), value.items...)
}

func (orderedSequenceTypedValue) typedValueVariant() {}

func (orderedSequenceTypedValue) orderedSequenceTypedValueVariant() {}

type UnorderedSetTypedValue interface {
	TypedValue
	Items() []TypedValue
	unorderedSetTypedValueVariant()
}

type unorderedSetTypedValue struct {
	items []TypedValue
}

func NewUnorderedSetValue(items []TypedValue) (UnorderedSetTypedValue, error) {
	owned, err := copyValidTypedValues("unordered-set", items)
	if err != nil {
		return nil, err
	}
	return unorderedSetTypedValue{items: owned}, nil
}

func (unorderedSetTypedValue) Kind() TypedValueKind { return TypedValueUnorderedSet }

func (value unorderedSetTypedValue) Items() []TypedValue {
	return append([]TypedValue(nil), value.items...)
}

func (unorderedSetTypedValue) typedValueVariant() {}

func (unorderedSetTypedValue) unorderedSetTypedValueVariant() {}

func copyValidTypedValues(label string, items []TypedValue) ([]TypedValue, error) {
	owned := append([]TypedValue(nil), items...)
	for index, item := range owned {
		if !validTypedValue(item) {
			return nil, fmt.Errorf("%s item at index %d is not a valid closed TypedValue", label, index)
		}
	}
	return owned, nil
}

func validTypedValue(value TypedValue) bool {
	if value == nil {
		return false
	}

	switch typed := value.(type) {
	case scalarTypedValue:
		return typed.kind.valid()
	case recordTypedValue:
		return len(typed.fields) > 0
	case sumTypedValue:
		return typed.variant.valid() && validTypedValue(typed.value)
	case orderedSequenceTypedValue:
		return allTypedValuesValid(typed.items)
	case unorderedSetTypedValue:
		return allTypedValuesValid(typed.items)
	case claimGraphValue:
		return true
	default:
		return false
	}
}

func allTypedValuesValid(values []TypedValue) bool {
	for _, value := range values {
		if !validTypedValue(value) {
			return false
		}
	}
	return true
}

type AssertedTypedValueDigest interface {
	Digest() (SHA256Digest, bool)
	assertedTypedValueDigestVariant()
}

type NoAssertedDigest struct{}

func (NoAssertedDigest) Digest() (SHA256Digest, bool) { return SHA256Digest{}, false }

func (NoAssertedDigest) assertedTypedValueDigestVariant() {}

type ExactAssertedDigest struct {
	digest SHA256Digest
}

func NewExactAssertedDigest(digest SHA256Digest) (ExactAssertedDigest, error) {
	if !digest.valid() {
		return ExactAssertedDigest{}, fmt.Errorf("asserted typed-value digest is required")
	}
	return ExactAssertedDigest{digest: digest}, nil
}

func (asserted ExactAssertedDigest) Digest() (SHA256Digest, bool) {
	return asserted.digest, asserted.digest.valid()
}

func (ExactAssertedDigest) assertedTypedValueDigestVariant() {}

type TypedValueCandidate struct {
	valueKind      ValueKindRef
	valueShape     ValueShapeRef
	codec          CodecRef
	inputBytes     []byte
	assertedDigest AssertedTypedValueDigest
}

func NewTypedValueCandidate(
	valueKind ValueKindRef,
	valueShape ValueShapeRef,
	codec CodecRef,
	inputBytes []byte,
	assertedDigest AssertedTypedValueDigest,
) (TypedValueCandidate, error) {
	if !valueKind.valid() {
		return TypedValueCandidate{}, fmt.Errorf("typed-value candidate ValueKindRef is required")
	}
	if !valueShape.valid() {
		return TypedValueCandidate{}, fmt.Errorf("typed-value candidate ValueShapeRef is required")
	}
	if !codec.valid() {
		return TypedValueCandidate{}, fmt.Errorf("typed-value candidate CodecRef is required")
	}
	if len(inputBytes) == 0 {
		return TypedValueCandidate{}, fmt.Errorf("typed-value candidate input bytes are required")
	}
	if !validAssertedTypedValueDigest(assertedDigest) {
		return TypedValueCandidate{}, fmt.Errorf("typed-value candidate requires an explicit digest posture")
	}

	return TypedValueCandidate{
		valueKind:      valueKind,
		valueShape:     valueShape,
		codec:          codec,
		inputBytes:     append([]byte(nil), inputBytes...),
		assertedDigest: assertedDigest,
	}, nil
}

func (candidate TypedValueCandidate) ValueKind() ValueKindRef { return candidate.valueKind }

func (candidate TypedValueCandidate) ValueShape() ValueShapeRef { return candidate.valueShape }

func (candidate TypedValueCandidate) Codec() CodecRef { return candidate.codec }

func (candidate TypedValueCandidate) InputBytes() []byte {
	return append([]byte(nil), candidate.inputBytes...)
}

func (candidate TypedValueCandidate) AssertedDigest() AssertedTypedValueDigest {
	return candidate.assertedDigest
}

func (candidate TypedValueCandidate) valid() bool {
	return candidate.valueKind.valid() &&
		candidate.valueShape.valid() &&
		candidate.codec.valid() &&
		len(candidate.inputBytes) > 0 &&
		validAssertedTypedValueDigest(candidate.assertedDigest)
}

func validAssertedTypedValueDigest(asserted AssertedTypedValueDigest) bool {
	switch value := asserted.(type) {
	case NoAssertedDigest:
		return true
	case ExactAssertedDigest:
		return value.digest.valid()
	default:
		return false
	}
}

// VerifiedTypedValue is sealed and has no exported constructor. The only
// implementation is created by VerifyTypedValue after active-binding and
// codec checks succeed.
type VerifiedTypedValue interface {
	ValueKind() ValueKindRef
	ValueShape() ValueShapeRef
	Codec() CodecRef
	CanonicalBytes() []byte
	Digest() SHA256Digest
	verifiedTypedValueVariant()
}

type verifiedTypedValue struct {
	valueKind      ValueKindRef
	valueShape     ValueShapeRef
	codec          CodecRef
	canonicalBytes []byte
	digest         SHA256Digest
}

func (value verifiedTypedValue) ValueKind() ValueKindRef { return value.valueKind }

func (value verifiedTypedValue) ValueShape() ValueShapeRef { return value.valueShape }

func (value verifiedTypedValue) Codec() CodecRef { return value.codec }

func (value verifiedTypedValue) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value verifiedTypedValue) Digest() SHA256Digest { return value.digest }

func (verifiedTypedValue) verifiedTypedValueVariant() {}

func validVerifiedTypedValue(value VerifiedTypedValue) bool {
	verified, ok := value.(verifiedTypedValue)
	if !ok {
		return false
	}
	return verified.valueKind.valid() &&
		verified.valueShape.valid() &&
		verified.codec.valid() &&
		len(verified.canonicalBytes) > 0 &&
		verified.digest.valid()
}

type OpaqueStoredValue struct {
	valueKind      ValueKindRef
	valueShape     ValueShapeRef
	codec          CodecRef
	canonicalBytes []byte
	digest         SHA256Digest
	provenance     ProvenanceRef
}

func NewOpaqueStoredValue(
	valueKind ValueKindRef,
	valueShape ValueShapeRef,
	codec CodecRef,
	canonicalBytes []byte,
	digest SHA256Digest,
	provenance ProvenanceRef,
) (OpaqueStoredValue, error) {
	if !valueKind.valid() || !valueShape.valid() || !codec.valid() {
		return OpaqueStoredValue{}, fmt.Errorf("opaque stored value requires exact kind, shape, and codec refs")
	}
	if len(canonicalBytes) == 0 {
		return OpaqueStoredValue{}, fmt.Errorf("opaque stored value canonical bytes are required")
	}
	if !digest.valid() {
		return OpaqueStoredValue{}, fmt.Errorf("opaque stored value digest is required")
	}
	expectedDigest := digestTypedValue(valueKind, valueShape, codec, canonicalBytes)
	if digest != expectedDigest {
		return OpaqueStoredValue{}, fmt.Errorf("opaque stored value digest does not match its exact canonical envelope")
	}
	if !provenance.valid() {
		return OpaqueStoredValue{}, fmt.Errorf("opaque stored value provenance is required")
	}

	return OpaqueStoredValue{
		valueKind:      valueKind,
		valueShape:     valueShape,
		codec:          codec,
		canonicalBytes: append([]byte(nil), canonicalBytes...),
		digest:         digest,
		provenance:     provenance,
	}, nil
}

func (value OpaqueStoredValue) ValueKind() ValueKindRef { return value.valueKind }

func (value OpaqueStoredValue) ValueShape() ValueShapeRef { return value.valueShape }

func (value OpaqueStoredValue) Codec() CodecRef { return value.codec }

func (value OpaqueStoredValue) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value OpaqueStoredValue) Digest() SHA256Digest { return value.digest }

func (value OpaqueStoredValue) Provenance() ProvenanceRef { return value.provenance }
