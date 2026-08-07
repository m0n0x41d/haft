package typedmemory

import (
	"bytes"
	"strings"
	"testing"
)

func TestCodecRegistryIsImmutableAndDoesNotAdmitValueKinds(t *testing.T) {
	shape := valueTestShapeRef(t, "U.ClaimGraphShape", 's')
	codecRef := valueTestCodecRef(t, "U.ClaimGraphCodecV1", "1", 'c')
	codec, err := NewClaimGraphCodecV1(shape)
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1: %v", err)
	}
	empty := NewCodecRegistry()
	registered, err := empty.Register(codecRef, codec)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if empty.Contains(codecRef) || empty.Len() != 0 {
		t.Fatal("Register mutated the receiver")
	}
	if !registered.Contains(codecRef) || registered.Len() != 1 {
		t.Fatal("new registry does not contain exact CodecRef")
	}
	if _, err := registered.Register(codecRef, codec); err == nil {
		t.Fatal("registry allowed replacement under an existing CodecRef")
	}
}

func TestVerifyTypedValueRequiresBindingAndCodecThenSealsCanonicalValue(t *testing.T) {
	fixture := newValueTestFixture(t)
	encoded := valueTestEncodedGraph(t, fixture.codec, fixture.graph)
	candidate, err := NewTypedValueCandidate(
		fixture.valueKind,
		fixture.shape,
		fixture.codecRef,
		encoded,
		NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate: %v", err)
	}

	missing := VerifyTypedValue(NewCodecRegistry(), fixture.binding, candidate)
	missingResult, ok := missing.(UnderdeterminedTypedValue)
	if !ok {
		t.Fatalf("missing codec result = %T, want UnderdeterminedTypedValue", missing)
	}
	if code := missingResult.Diagnostics()[0].Code(); code != DiagnosticCodecUnavailable {
		t.Fatalf("missing codec code = %q", code)
	}

	registry := NewCodecRegistry()
	registry, err = registry.Register(fixture.codecRef, fixture.codec)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	verifiedResult := VerifyTypedValue(registry, fixture.binding, candidate)
	valid, ok := verifiedResult.(ValidTypedValue)
	if !ok {
		t.Fatalf("verification result = %T, want ValidTypedValue", verifiedResult)
	}
	verified := valid.Value()
	if !validVerifiedTypedValue(verified) {
		t.Fatal("verifier returned an invalid sealed value")
	}
	canonical := verified.CanonicalBytes()
	canonical[0] ^= 0xff
	if bytes.Equal(canonical, verified.CanonicalBytes()) {
		t.Fatal("VerifiedTypedValue leaked mutable canonical bytes")
	}
}

func TestVerifyTypedValueRejectsExactRefAndDigestMismatch(t *testing.T) {
	fixture := newValueTestFixture(t)
	encoded := valueTestEncodedGraph(t, fixture.codec, fixture.graph)
	wrongKind := valueTestValueKindRef(t, "U.Other", 'o')
	candidate, err := NewTypedValueCandidate(
		wrongKind,
		fixture.shape,
		fixture.codecRef,
		encoded,
		NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate: %v", err)
	}
	registry := NewCodecRegistry()
	registry, err = registry.Register(fixture.codecRef, fixture.codec)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	result := VerifyTypedValue(registry, fixture.binding, candidate)
	invalid, ok := result.(InvalidTypedValue)
	if !ok {
		t.Fatalf("ref mismatch result = %T, want InvalidTypedValue", result)
	}
	if code := invalid.Diagnostics()[0].Code(); code != DiagnosticValueKindMismatch {
		t.Fatalf("ref mismatch code = %q", code)
	}

	wrongDigest := valueTestDigest(t, 'd')
	asserted, err := NewExactAssertedDigest(wrongDigest)
	if err != nil {
		t.Fatalf("NewExactAssertedDigest: %v", err)
	}
	candidate, err = NewTypedValueCandidate(
		fixture.valueKind,
		fixture.shape,
		fixture.codecRef,
		encoded,
		asserted,
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate(asserted): %v", err)
	}
	result = VerifyTypedValue(registry, fixture.binding, candidate)
	invalid, ok = result.(InvalidTypedValue)
	if !ok {
		t.Fatalf("digest mismatch result = %T, want InvalidTypedValue", result)
	}
	if code := invalid.Diagnostics()[0].Code(); code != DiagnosticTypedValueDigestMismatch {
		t.Fatalf("digest mismatch code = %q", code)
	}
}

func TestVerifyTypedValueRejectsIncompleteCodecResult(t *testing.T) {
	fixture := newValueTestFixture(t)
	encoded := valueTestEncodedGraph(t, fixture.codec, fixture.graph)
	candidate, err := NewTypedValueCandidate(
		fixture.valueKind,
		fixture.shape,
		fixture.codecRef,
		encoded,
		NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate: %v", err)
	}
	registry := NewCodecRegistry()
	registry, err = registry.Register(fixture.codecRef, emptyCodecImplementation{})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	result := VerifyTypedValue(registry, fixture.binding, candidate)
	invalid, ok := result.(InvalidTypedValue)
	if !ok {
		t.Fatalf("incomplete codec result = %T, want InvalidTypedValue", result)
	}
	if code := invalid.Diagnostics()[0].Code(); code != DiagnosticMalformedValue {
		t.Fatalf("incomplete codec result code = %q", code)
	}
}

func TestTypedValueDigestCommitsToEveryExactRef(t *testing.T) {
	fixture := newValueTestFixture(t)
	canonical := valueTestEncodedGraph(t, fixture.codec, fixture.graph)
	base := digestTypedValue(fixture.valueKind, fixture.shape, fixture.codecRef, canonical)
	otherKind := valueTestValueKindRef(t, "U.Other", 'o')
	otherShape := valueTestShapeRef(t, "U.OtherShape", 'x')
	otherCodec := valueTestCodecRef(t, "U.OtherCodec", "1", 'z')

	digests := []SHA256Digest{
		digestTypedValue(otherKind, fixture.shape, fixture.codecRef, canonical),
		digestTypedValue(fixture.valueKind, otherShape, fixture.codecRef, canonical),
		digestTypedValue(fixture.valueKind, fixture.shape, otherCodec, canonical),
		digestTypedValue(fixture.valueKind, fixture.shape, fixture.codecRef, append(canonical, 0x01)),
	}
	for index, digest := range digests {
		if digest == base {
			t.Fatalf("domain-separated digest variant %d did not change", index)
		}
	}
}

func TestTypedValueDigestV1GoldenEnvelope(t *testing.T) {
	fixture := newValueTestFixture(t)
	canonical := valueTestEncodedGraph(t, fixture.codec, fixture.graph)
	digest := digestTypedValue(fixture.valueKind, fixture.shape, fixture.codecRef, canonical)
	const want = "sha256:60b58f9a738c2371d16391c09a5ebc4e8e385d89b70b5c9280d0a3b7d3ffdc7e"
	if digest.String() != want {
		t.Fatalf("typed-value golden digest = %s; want %s", digest.String(), want)
	}
}

func TestComputeTypedValueDigestMatchesVerifiedEnvelope(t *testing.T) {
	fixture := newValueTestFixture(t)
	canonical := valueTestEncodedGraph(t, fixture.codec, fixture.graph)
	digest, err := ComputeTypedValueDigest(
		fixture.valueKind,
		fixture.shape,
		fixture.codecRef,
		canonical,
	)
	if err != nil {
		t.Fatalf("ComputeTypedValueDigest(): %v", err)
	}
	if digest != digestTypedValue(fixture.valueKind, fixture.shape, fixture.codecRef, canonical) {
		t.Fatal("public typed-value digest helper diverged from the verifier envelope")
	}
	if _, err := ComputeTypedValueDigest(ValueKindRef{}, fixture.shape, fixture.codecRef, canonical); err == nil {
		t.Fatal("ComputeTypedValueDigest accepted an invalid exact reference")
	}
	if _, err := ComputeTypedValueDigest(fixture.valueKind, fixture.shape, fixture.codecRef, nil); err == nil {
		t.Fatal("ComputeTypedValueDigest accepted empty canonical bytes")
	}
}

func TestVerifyStoredTypedValueDigestParsesExactCanonicalReferences(t *testing.T) {
	fixture := newValueTestFixture(t)
	canonical := valueTestEncodedGraph(t, fixture.codec, fixture.graph)
	digest, err := ComputeTypedValueDigest(
		fixture.valueKind,
		fixture.shape,
		fixture.codecRef,
		canonical,
	)
	if err != nil {
		t.Fatalf("ComputeTypedValueDigest(): %v", err)
	}
	err = VerifyStoredTypedValueDigest(
		fixture.valueKind.String(),
		fixture.shape.String(),
		fixture.codecRef.String(),
		canonical,
		digest,
	)
	if err != nil {
		t.Fatalf("VerifyStoredTypedValueDigest(): %v", err)
	}
	wrongDigest := valueTestDigest(t, 0xfe)
	if err := VerifyStoredTypedValueDigest(
		fixture.valueKind.String(),
		fixture.shape.String(),
		fixture.codecRef.String(),
		canonical,
		wrongDigest,
	); err == nil {
		t.Fatal("VerifyStoredTypedValueDigest accepted a mismatched digest")
	}
	if err := VerifyStoredTypedValueDigest(
		fixture.valueKind.String()+"/value-kind/duplicate",
		fixture.shape.String(),
		fixture.codecRef.String(),
		canonical,
		digest,
	); err == nil {
		t.Fatal("VerifyStoredTypedValueDigest accepted a non-canonical ValueKindRef")
	}
	if err := VerifyStoredTypedValueDigest(
		fixture.valueKind.String(),
		fixture.shape.String(),
		"codec:01:x:1:v:"+fixture.codecRef.SpecificationDigest().String(),
		canonical,
		digest,
	); err == nil {
		t.Fatal("VerifyStoredTypedValueDigest accepted a non-canonical CodecRef length")
	}
}

func TestOpaqueStoredValuePreservesExactBytesWithoutBecomingVerified(t *testing.T) {
	fixture := newValueTestFixture(t)
	canonical := valueTestEncodedGraph(t, fixture.codec, fixture.graph)
	digest := digestTypedValue(fixture.valueKind, fixture.shape, fixture.codecRef, canonical)
	provenance, err := NewProvenanceRef("legacy:value:1")
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	opaque, err := NewOpaqueStoredValue(
		fixture.valueKind,
		fixture.shape,
		fixture.codecRef,
		canonical,
		digest,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewOpaqueStoredValue: %v", err)
	}
	copyBytes := opaque.CanonicalBytes()
	copyBytes[0] ^= 0xff
	if bytes.Equal(copyBytes, opaque.CanonicalBytes()) {
		t.Fatal("OpaqueStoredValue leaked mutable bytes")
	}
	if _, ok := any(opaque).(VerifiedTypedValue); ok {
		t.Fatal("OpaqueStoredValue implements VerifiedTypedValue")
	}
	if _, ok := any(opaque).(StrongRef); ok {
		t.Fatal("OpaqueStoredValue became StrongRef/blob-key semantics")
	}
}

func TestOpaqueStoredValueRejectsMismatchedDigest(t *testing.T) {
	fixture := newValueTestFixture(t)
	canonical := valueTestEncodedGraph(t, fixture.codec, fixture.graph)
	provenance, err := NewProvenanceRef("legacy:value:wrong-digest")
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	_, err = NewOpaqueStoredValue(
		fixture.valueKind,
		fixture.shape,
		fixture.codecRef,
		canonical,
		valueTestDigest(t, 'x'),
		provenance,
	)
	if err == nil {
		t.Fatal("NewOpaqueStoredValue accepted a digest for different canonical bytes")
	}
}

func TestOpaqueStoredValueRehydratesOnlyThroughExactBindingAndCodec(t *testing.T) {
	fixture := newValueTestFixture(t)
	canonical := valueTestEncodedGraph(t, fixture.codec, fixture.graph)
	digest := digestTypedValue(fixture.valueKind, fixture.shape, fixture.codecRef, canonical)
	provenance, err := NewProvenanceRef("legacy:value:rehydrate")
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	opaque, err := NewOpaqueStoredValue(
		fixture.valueKind,
		fixture.shape,
		fixture.codecRef,
		canonical,
		digest,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewOpaqueStoredValue: %v", err)
	}
	asserted, err := NewExactAssertedDigest(opaque.Digest())
	if err != nil {
		t.Fatalf("NewExactAssertedDigest: %v", err)
	}
	candidate, err := NewTypedValueCandidate(
		opaque.ValueKind(),
		opaque.ValueShape(),
		opaque.Codec(),
		opaque.CanonicalBytes(),
		asserted,
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate: %v", err)
	}

	withoutCodec := VerifyTypedValue(NewCodecRegistry(), fixture.binding, candidate)
	if _, ok := withoutCodec.(UnderdeterminedTypedValue); !ok {
		t.Fatalf("verification without codec = %T; want UnderdeterminedTypedValue", withoutCodec)
	}

	registry, err := NewCodecRegistry().Register(fixture.codecRef, fixture.codec)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	result := VerifyTypedValue(registry, fixture.binding, candidate)
	valid, ok := result.(ValidTypedValue)
	if !ok {
		t.Fatalf("verification with exact binding and codec = %T; want ValidTypedValue", result)
	}
	rehydrated := valid.Value()
	if !bytes.Equal(rehydrated.CanonicalBytes(), opaque.CanonicalBytes()) {
		t.Fatal("rehydration changed canonical bytes")
	}
	if rehydrated.Digest() != opaque.Digest() {
		t.Fatal("rehydration changed content-addressed digest")
	}
	if _, ok := any(opaque).(VerifiedTypedValue); ok {
		t.Fatal("rehydration retroactively promoted OpaqueStoredValue")
	}
}

type valueTestFixture struct {
	valueKind ValueKindRef
	shape     ValueShapeRef
	codecRef  CodecRef
	codec     ClaimGraphCodecV1
	binding   ValueBinding
	graph     ClaimGraphValue
}

type emptyCodecImplementation struct{}

func (emptyCodecImplementation) Canonicalize(ValueShapeRef, []byte) CodecCanonicalization {
	return CanonicalizedCodecValue{}
}

func newValueTestFixture(t *testing.T) valueTestFixture {
	t.Helper()
	shape := valueTestShapeRef(t, "U.ClaimGraphShape", 's')
	codecRef := valueTestCodecRef(t, "U.ClaimGraphCodecV1", "1", 'c')
	valueKind := valueTestValueKindRef(t, "U.ClaimGraph", 'k')
	codec, err := NewClaimGraphCodecV1(shape)
	if err != nil {
		t.Fatalf("NewClaimGraphCodecV1: %v", err)
	}
	provenance := valueTestDeclarationProvenance(t)
	binding, err := NewValueBinding(valueKind, shape, codecRef, provenance)
	if err != nil {
		t.Fatalf("NewValueBinding: %v", err)
	}
	graph := valueTestGraph(t)
	return valueTestFixture{
		valueKind: valueKind,
		shape:     shape,
		codecRef:  codecRef,
		codec:     codec,
		binding:   binding,
		graph:     graph,
	}
}

func valueTestEncodedGraph(t *testing.T, codec ClaimGraphCodecV1, graph ClaimGraphValue) []byte {
	t.Helper()
	result := codec.EncodeInput(graph)
	canonical, ok := result.(CanonicalizedCodecValue)
	if !ok {
		t.Fatalf("EncodeInput = %T, want CanonicalizedCodecValue", result)
	}
	return canonical.CanonicalBytes()
}

func valueTestDeclarationProvenance(t *testing.T) DeclarationProvenance {
	t.Helper()
	provenanceRef, err := NewProvenanceRef("test:declaration")
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	unitID, err := NewSourceUnitID("test-source")
	if err != nil {
		t.Fatalf("NewSourceUnitID: %v", err)
	}
	revision, err := NewSourceRevision("test-revision")
	if err != nil {
		t.Fatalf("NewSourceRevision: %v", err)
	}
	lineRange, err := NewSourceLineRange(1, 2)
	if err != nil {
		t.Fatalf("NewSourceLineRange: %v", err)
	}
	location, err := NewUnpatternedSourceLocation(unitID, revision, valueTestDigest(t, 'p'), lineRange)
	if err != nil {
		t.Fatalf("NewUnpatternedSourceLocation: %v", err)
	}
	ruleID, err := NewCompilerRuleID("test.rule")
	if err != nil {
		t.Fatalf("NewCompilerRuleID: %v", err)
	}
	provenance, err := NewFPFSourceProvenance(provenanceRef, location, ruleID)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance: %v", err)
	}
	return provenance
}

func valueTestDigest(t *testing.T, fill byte) SHA256Digest {
	t.Helper()
	hexAlphabet := "0123456789abcdef"
	hexDigit := hexAlphabet[int(fill)%len(hexAlphabet)]
	raw := "sha256:" + strings.Repeat(string(hexDigit), 64)
	digest, err := NewSHA256Digest(raw)
	if err != nil {
		t.Fatalf("NewSHA256Digest(%q): %v", raw, err)
	}
	return digest
}

func valueTestTypeEnvRef(t *testing.T, fill byte) TypeEnvRef {
	t.Helper()
	ref, err := NewTypeEnvRef(valueTestDigest(t, fill))
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	return ref
}

func valueTestValueKindRef(t *testing.T, raw string, fill byte) ValueKindRef {
	t.Helper()
	id, err := NewKindID(raw)
	if err != nil {
		t.Fatalf("NewKindID: %v", err)
	}
	ref, err := NewValueKindRef(valueTestTypeEnvRef(t, fill), id)
	if err != nil {
		t.Fatalf("NewValueKindRef: %v", err)
	}
	return ref
}

func valueTestShapeRef(t *testing.T, raw string, fill byte) ValueShapeRef {
	t.Helper()
	id, err := NewShapeID(raw)
	if err != nil {
		t.Fatalf("NewShapeID: %v", err)
	}
	ref, err := NewValueShapeRef(id, valueTestDigest(t, fill))
	if err != nil {
		t.Fatalf("NewValueShapeRef: %v", err)
	}
	return ref
}

func valueTestCodecRef(t *testing.T, raw, versionRaw string, fill byte) CodecRef {
	t.Helper()
	id, err := NewCodecID(raw)
	if err != nil {
		t.Fatalf("NewCodecID: %v", err)
	}
	version, err := NewCanonicalizationVersion(versionRaw)
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion: %v", err)
	}
	ref, err := NewCodecRef(id, version, valueTestDigest(t, fill))
	if err != nil {
		t.Fatalf("NewCodecRef: %v", err)
	}
	return ref
}

func valueTestMemberName(t *testing.T, raw string) ValueMemberName {
	t.Helper()
	name, err := NewValueMemberName(raw)
	if err != nil {
		t.Fatalf("NewValueMemberName: %v", err)
	}
	return name
}
