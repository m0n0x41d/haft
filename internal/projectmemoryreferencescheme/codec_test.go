package projectmemoryreferencescheme

import (
	"bytes"
	"strings"
	"testing"
)

func TestProjectMemoryReferenceSchemeV1CodecRoundTripsExactCanonicalValue(t *testing.T) {
	designation := fixtureRulePin(
		t,
		SemanticRoleDesignation,
		"designation:resolve",
		"codec-designation",
		runtimePresent,
	)
	interpretation := fixtureRulePin(
		t,
		SemanticRoleInterpretation,
		"interpretation:claims",
		"codec-interpretation",
		runtimeAbsent,
	)
	measurement := fixtureRulePin(
		t,
		SemanticRoleMeasurement,
		"measurement:not-applicable",
		"codec-measurement",
		runtimePresent,
	)
	evaluation := fixtureRulePin(
		t,
		SemanticRoleEvaluation,
		"evaluation:claims",
		"codec-evaluation",
		runtimePresent,
	)
	scheme := fixtureScheme(
		t,
		[]ExactRulePin{designation},
		[]ExactRulePin{interpretation},
		mustMeasurementNotApplicable(t, measurement),
		mustEvaluationRules(t, []ExactRulePin{evaluation}),
	)
	codec := NewProjectMemoryReferenceSchemeV1Codec()

	encoded, err := codec.Encode(scheme)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !bytes.Equal(encoded, scheme.CanonicalBytes()) {
		t.Fatal("codec changed existing V1 canonical bytes")
	}
	decoded, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded.Digest() != scheme.Digest() {
		t.Fatalf("codec changed digest: %q != %q", decoded.Digest(), scheme.Digest())
	}
	if !bytes.Equal(decoded.CanonicalBytes(), encoded) {
		t.Fatal("codec round-trip changed canonical bytes")
	}
	verified, err := codec.Verify(scheme.Digest(), encoded)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if verified.Digest() != scheme.Digest() {
		t.Fatalf("verified digest = %q, want %q", verified.Digest(), scheme.Digest())
	}
	encoded[0] ^= 0xff
	if bytes.Equal(encoded, scheme.CanonicalBytes()) {
		t.Fatal("codec Encode returned shared canonical storage")
	}
}

func TestProjectMemoryReferenceSchemeV1CodecRejectsLegacyAndNoncanonicalBytes(t *testing.T) {
	designationA := fixtureRulePin(
		t,
		SemanticRoleDesignation,
		"designation:a",
		"codec-a",
		runtimeAbsent,
	)
	designationB := fixtureRulePin(
		t,
		SemanticRoleDesignation,
		"designation:b",
		"codec-b",
		runtimeAbsent,
	)
	interpretation := fixtureRulePin(
		t,
		SemanticRoleInterpretation,
		"interpretation:claims",
		"codec",
		runtimeAbsent,
	)
	measurement := fixtureRulePin(
		t,
		SemanticRoleMeasurement,
		"measurement:not-applicable",
		"codec",
		runtimeAbsent,
	)
	evaluation := fixtureRulePin(
		t,
		SemanticRoleEvaluation,
		"evaluation:not-applicable",
		"codec",
		runtimeAbsent,
	)
	codec := NewProjectMemoryReferenceSchemeV1Codec()
	legacy := []byte(`{"Primary":"fpf","Anchors":["A.6.0"]}`)
	if _, err := codec.Decode(legacy); err == nil || !strings.Contains(err.Error(), "domain") {
		t.Fatalf("legacy Primary/Anchors decode error = %v", err)
	}

	noncanonical := newSchemeWriter()
	noncanonical.addRulePins([]ExactRulePin{designationB, designationA})
	noncanonical.addRulePins([]ExactRulePin{interpretation})
	if err := noncanonical.addMeasurementBranch(
		mustMeasurementNotApplicable(t, measurement),
	); err != nil {
		t.Fatalf("addMeasurementBranch() error = %v", err)
	}
	if err := noncanonical.addEvaluationBranch(
		mustEvaluationNotApplicable(t, evaluation),
	); err != nil {
		t.Fatalf("addEvaluationBranch() error = %v", err)
	}
	if _, err := codec.Decode(noncanonical.bytes()); err == nil ||
		!strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("noncanonical decode error = %v", err)
	}
}

func TestProjectMemoryReferenceSchemeV1CodecRejectsInvalidValuesAndDigestMismatch(t *testing.T) {
	codec := NewProjectMemoryReferenceSchemeV1Codec()
	if _, err := codec.Encode(ProjectMemoryReferenceSchemeV1{}); err == nil {
		t.Fatal("Encode accepted a zero scheme")
	}
	base := schemeWithDesignation(
		t,
		fixtureRulePin(
			t,
			SemanticRoleDesignation,
			"designation:base",
			"codec-base",
			runtimeAbsent,
		),
	)
	other := schemeWithDesignation(
		t,
		fixtureRulePin(
			t,
			SemanticRoleDesignation,
			"designation:other",
			"codec-other",
			runtimeAbsent,
		),
	)
	if _, err := codec.Verify(base.Digest(), other.CanonicalBytes()); err == nil ||
		!strings.Contains(err.Error(), "does not match") {
		t.Fatalf("digest mismatch error = %v", err)
	}
}
