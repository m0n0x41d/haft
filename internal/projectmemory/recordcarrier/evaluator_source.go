package recordcarrier

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// RecordMembershipSourceDeliveryV1 is the closed delivery posture consumed by
// the pure evaluator. It is not a membership result and carries no approval or
// authority semantics.
type RecordMembershipSourceDeliveryV1 interface {
	expectedObservableRef() typedmemory.ObservableInputRef
	recordMembershipSourceDeliveryV1()
}

// trustedRecordMembershipSourceDeliveryV1 has no exported constructor. A
// future immutable-store adapter must add the only production bridge that can
// create it after verifying its own producer/storage trust contract. Merely
// calling SealRecordMembershipSourceV1 cannot produce this capability.
type trustedRecordMembershipSourceDeliveryV1 struct {
	expected  typedmemory.MemberOfObservableInput
	canonical []byte
}

func (delivery *trustedRecordMembershipSourceDeliveryV1) expectedObservableRef() typedmemory.ObservableInputRef {
	return delivery.expected.Reference()
}

func (*trustedRecordMembershipSourceDeliveryV1) recordMembershipSourceDeliveryV1() {}

func newTrustedRecordMembershipSourceDeliveryV1(
	expected typedmemory.MemberOfObservableInput,
	canonical []byte,
) (*trustedRecordMembershipSourceDeliveryV1, error) {
	if err := validateObservableInput(expected); err != nil {
		return nil, err
	}
	return &trustedRecordMembershipSourceDeliveryV1{
		expected:  expected,
		canonical: append([]byte(nil), canonical...),
	}, nil
}

// untrustedRecordMembershipSourceDeliveryV1 preserves candidate bytes for
// strict verification and diagnostics, but can never produce Member or
// NotMember.
type untrustedRecordMembershipSourceDeliveryV1 struct {
	expected  typedmemory.MemberOfObservableInput
	canonical []byte
}

// NewUntrustedRecordMembershipSourceDeliveryV1 snapshots candidate bytes for
// diagnostics and strict verification. The resulting delivery can only yield
// Undefined, even when the bytes are otherwise valid.
func NewUntrustedRecordMembershipSourceDeliveryV1(
	expected typedmemory.MemberOfObservableInput,
	canonical []byte,
) (RecordMembershipSourceDeliveryV1, error) {
	if err := validateObservableInput(expected); err != nil {
		return nil, err
	}
	return &untrustedRecordMembershipSourceDeliveryV1{
		expected:  expected,
		canonical: append([]byte(nil), canonical...),
	}, nil
}

func (delivery *untrustedRecordMembershipSourceDeliveryV1) expectedObservableRef() typedmemory.ObservableInputRef {
	return delivery.expected.Reference()
}

func (*untrustedRecordMembershipSourceDeliveryV1) recordMembershipSourceDeliveryV1() {}

type missingRecordMembershipSourceDeliveryV1 struct {
	expected typedmemory.ObservableInputRef
}

// NewMissingRecordMembershipSourceDeliveryV1 identifies the exact observable
// reference whose trusted bytes are unavailable. It does not synthesize an
// empty source or turn absence into NotMember.
func NewMissingRecordMembershipSourceDeliveryV1(
	expected typedmemory.ObservableInputRef,
) (RecordMembershipSourceDeliveryV1, error) {
	if err := validateObservableInputRef(expected); err != nil {
		return nil, err
	}
	return &missingRecordMembershipSourceDeliveryV1{expected: expected}, nil
}

func (delivery *missingRecordMembershipSourceDeliveryV1) expectedObservableRef() typedmemory.ObservableInputRef {
	return delivery.expected
}

func (*missingRecordMembershipSourceDeliveryV1) recordMembershipSourceDeliveryV1() {}

func validateRecordMembershipSourceDeliveryV1(
	delivery RecordMembershipSourceDeliveryV1,
) error {
	switch value := delivery.(type) {
	case *trustedRecordMembershipSourceDeliveryV1:
		if value == nil {
			return fmt.Errorf("trusted record membership source delivery is nil")
		}
		return validateObservableInput(value.expected)
	case *untrustedRecordMembershipSourceDeliveryV1:
		if value == nil {
			return fmt.Errorf("untrusted record membership source delivery is nil")
		}
		return validateObservableInput(value.expected)
	case *missingRecordMembershipSourceDeliveryV1:
		if value == nil {
			return fmt.Errorf("missing record membership source delivery is nil")
		}
		return validateObservableInputRef(value.expected)
	default:
		return fmt.Errorf("record membership source delivery is required or unsupported")
	}
}

func validateObservableInput(input typedmemory.MemberOfObservableInput) error {
	if err := validateObservableInputRef(input.Reference()); err != nil {
		return err
	}
	digest, err := typedmemory.NewSHA256Digest(input.Digest().String())
	if err != nil || digest != input.Digest() {
		return fmt.Errorf("record membership observable digest is invalid")
	}
	return nil
}

func validateObservableInputRef(ref typedmemory.ObservableInputRef) error {
	parsed, err := typedmemory.NewObservableInputRef(ref.String())
	if err != nil || parsed != ref || parsed.String() != ref.String() {
		return fmt.Errorf("record membership observable reference is invalid")
	}
	return nil
}
