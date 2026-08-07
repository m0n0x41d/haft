package projecttypeenv

import (
	"bytes"
	"testing"
)

func TestProjectTypeEnvCompositeVerificationRecordDoesNotRecreateCapability(t *testing.T) {
	fixture := newCompositeLowererFixture(t)
	witness := fixture.verification
	record, err := DecodeProjectTypeEnvCompositeVerificationRecord(witness.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvCompositeVerificationRecord(): %v", err)
	}
	if err := record.Verify(); err != nil {
		t.Fatalf("decoded verification record is invalid: %v", err)
	}
	if record.Ref() != witness.Ref() ||
		!bytes.Equal(record.CanonicalBytes(), witness.CanonicalBytes()) {
		t.Fatal("decoded verification record changed exact identity or bytes")
	}
	if err := (ProjectTypeEnvCompositeVerification{record: record}).Verify(); err == nil {
		t.Fatal("decoded persisted record recreated the non-serializable Stage capability")
	}
}

func TestRestoreProjectTypeEnvCompositeVerificationRerunsExactLowerer(t *testing.T) {
	fixture := newCompositeLowererFixture(t)
	input := ProjectTypeEnvCompositePreparationInput{
		Base:         fixture.base,
		Linked:       fixture.linked,
		RuntimeBasis: fixture.runtime,
		Composite:    fixture.composite,
	}
	restored, err := RestoreProjectTypeEnvCompositeVerification(
		fixture.verification.Ref(),
		fixture.verification.CanonicalBytes(),
		input,
	)
	if err != nil {
		t.Fatalf("RestoreProjectTypeEnvCompositeVerification(): %v", err)
	}
	if err := restored.Verify(); err != nil {
		t.Fatalf("restored capability is invalid: %v", err)
	}
	if restored.Ref() != fixture.verification.Ref() ||
		!bytes.Equal(restored.CanonicalBytes(), fixture.verification.CanonicalBytes()) {
		t.Fatal("restored capability differs from original final-lowerer result")
	}

	mutated := fixture.verification.CanonicalBytes()
	mutated[len(mutated)-1] ^= 0xff
	if _, err := RestoreProjectTypeEnvCompositeVerification(
		fixture.verification.Ref(),
		mutated,
		input,
	); err == nil {
		t.Fatal("mutated persisted verification bytes restored a capability")
	}
}

func TestProjectTypeEnvCompositeVerificationRecordOwnsCanonicalBytes(t *testing.T) {
	fixture := newCompositeLowererFixture(t)
	canonical := fixture.verification.CanonicalBytes()
	record, err := VerifyProjectTypeEnvCompositeVerificationRecord(
		fixture.verification.Ref(),
		canonical,
	)
	if err != nil {
		t.Fatalf("VerifyProjectTypeEnvCompositeVerificationRecord(): %v", err)
	}
	canonical[0] ^= 0xff
	first := record.CanonicalBytes()
	first[0] ^= 0xff
	if err := record.Verify(); err != nil {
		t.Fatalf("caller mutation changed stored verification record: %v", err)
	}
	if bytes.Equal(first, record.CanonicalBytes()) {
		t.Fatal("CanonicalBytes returned shared mutable storage")
	}
}
