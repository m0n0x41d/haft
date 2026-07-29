package carrierfamily

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestClassificationSourceIsCurrentAndByteDisjointFromMembership(
	t *testing.T,
) {
	fixture := testSourceFixture(t)
	historical := fixture.source
	current, err := SealClassificationSourceV1(
		historical.ProjectID(),
		historical.EntityID(),
		historical.BoundedContext(),
		historical.Carrier(),
		historical.Binding(),
	)
	if err != nil {
		t.Fatalf("SealClassificationSourceV1: %v", err)
	}
	if current.SchemaVersion() != ClassificationSourceSchemaVersionV1 ||
		!strings.HasPrefix(
			current.Ref().String(),
			"carrier-family-classification-source:",
		) {
		t.Fatal("current carrier-family source lost its schema or reference domain")
	}
	if bytes.Equal(current.CanonicalBytes(), historical.CanonicalBytes()) ||
		current.Digest() == historical.ObservableInput().Digest() {
		t.Fatal("current classification source aliases historical membership bytes")
	}
	decoded, err := DecodeClassificationSourceV1(current.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeClassificationSourceV1: %v", err)
	}
	if decoded.Ref() != current.Ref() ||
		decoded.Digest() != current.Digest() ||
		decoded.FamilyToken() != "project_claim" {
		t.Fatal("current carrier-family source round trip changed its exact coordinate")
	}
	if _, err := VerifyClassificationSourceV1(
		current.Ref(),
		current.Digest(),
		current.CanonicalBytes(),
	); err != nil {
		t.Fatalf("VerifyClassificationSourceV1: %v", err)
	}
	if reflect.TypeOf(current) == reflect.TypeOf(historical) {
		t.Fatal("current and historical sources share one Go carrier type")
	}
}

func TestClassificationSourceRejectsTamperAndOuterFamilySubstitution(
	t *testing.T,
) {
	fixture := testSourceFixture(t)
	current, err := SealClassificationSourceV1(
		fixture.source.ProjectID(),
		fixture.source.EntityID(),
		fixture.source.BoundedContext(),
		fixture.source.Carrier(),
		fixture.source.Binding(),
	)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(
		string(current.CanonicalBytes()),
		`"family":"project_claim"`,
		`"family":"code_anchor"`,
		1,
	)
	if _, err := DecodeClassificationSourceV1([]byte(mutated)); err == nil {
		t.Fatal("classification source accepted an outer family substitution")
	}
	tampered := append([]byte(nil), current.CanonicalBytes()...)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := DecodeClassificationSourceV1(tampered); err == nil {
		t.Fatal("classification source accepted tampered canonical bytes")
	}
	trailing := append(current.CanonicalBytes(), '\n')
	if _, err := DecodeClassificationSourceV1(trailing); err == nil {
		t.Fatal("classification source accepted trailing bytes")
	}
}
