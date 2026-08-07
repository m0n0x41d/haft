package projecttypeenv

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCodecSpecificationV1RoundTripAndIsolation(t *testing.T) {
	id, version, shape := codecSpecificationFixtureCoordinates(t, "Haft.Codec.Text", "v1", "Haft.Shape.Text", 0x11)
	contract := []string{
		"Equal conceptual values produce equal canonical bytes.",
		"Decode then encode preserves canonical bytes.",
	}
	specification, err := DeriveCodecSpecificationV1(id, version, shape, contract)
	if err != nil {
		t.Fatalf("DeriveCodecSpecificationV1() error = %v", err)
	}
	if err := specification.Verify(); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	decoded, err := VerifyCodecSpecificationV1(
		specification.Ref(),
		specification.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("VerifyCodecSpecificationV1() error = %v", err)
	}
	if decoded.Ref() != specification.Ref() {
		t.Fatalf("decoded ref = %q, want %q", decoded.Ref(), specification.Ref())
	}
	if decoded.ValueShape() != shape {
		t.Fatalf("decoded shape = %q, want %q", decoded.ValueShape(), shape)
	}

	contract[0] = "mutated caller input"
	returnedContract := specification.Contract()
	returnedContract[0] = "mutated accessor result"
	canonical := specification.CanonicalBytes()
	canonical[0] ^= 0xff
	if specification.Contract()[0] != "Equal conceptual values produce equal canonical bytes." {
		t.Fatal("codec specification retained mutable contract storage")
	}
	if bytes.Equal(canonical, specification.CanonicalBytes()) {
		t.Fatal("codec specification retained mutable canonical-byte storage")
	}
}

func TestCodecSpecificationV1IdentityIsSensitiveToEverySemanticCoordinate(t *testing.T) {
	baseID, baseVersion, baseShape := codecSpecificationFixtureCoordinates(
		t,
		"Haft.Codec.Text",
		"v1",
		"Haft.Shape.Text",
		0x21,
	)
	baseContract := []string{"first", "second"}
	base := mustDeriveCodecSpecification(t, baseID, baseVersion, baseShape, baseContract)

	otherID, _, _ := codecSpecificationFixtureCoordinates(t, "Haft.Codec.Other", "v1", "Haft.Shape.Text", 0x21)
	_, otherVersion, _ := codecSpecificationFixtureCoordinates(t, "Haft.Codec.Text", "v2", "Haft.Shape.Text", 0x21)
	_, _, otherShape := codecSpecificationFixtureCoordinates(t, "Haft.Codec.Text", "v1", "Haft.Shape.Other", 0x22)
	cases := []struct {
		name     string
		id       typedmemory.CodecID
		version  typedmemory.CanonicalizationVersion
		shape    typedmemory.ValueShapeRef
		contract []string
	}{
		{name: "codec ID", id: otherID, version: baseVersion, shape: baseShape, contract: baseContract},
		{name: "canonicalization version", id: baseID, version: otherVersion, shape: baseShape, contract: baseContract},
		{name: "value shape", id: baseID, version: baseVersion, shape: otherShape, contract: baseContract},
		{name: "contract text", id: baseID, version: baseVersion, shape: baseShape, contract: []string{"first", "changed"}},
		{name: "contract order", id: baseID, version: baseVersion, shape: baseShape, contract: []string{"second", "first"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := mustDeriveCodecSpecification(
				t,
				testCase.id,
				testCase.version,
				testCase.shape,
				testCase.contract,
			)
			if candidate.Ref() == base.Ref() {
				t.Fatalf("semantic change did not change CodecRef %q", base.Ref())
			}
			if bytes.Equal(candidate.CanonicalBytes(), base.CanonicalBytes()) {
				t.Fatal("semantic change did not change canonical bytes")
			}
		})
	}
}

func TestCodecSpecificationV1RejectsInvalidContracts(t *testing.T) {
	id, version, shape := codecSpecificationFixtureCoordinates(t, "Haft.Codec.Text", "v1", "Haft.Shape.Text", 0x31)
	cases := []struct {
		name     string
		contract []string
		contains string
	}{
		{name: "empty set", contract: nil, contains: "at least one"},
		{name: "empty clause", contract: []string{""}, contains: "is empty"},
		{name: "duplicate", contract: []string{"same", "same"}, contains: "duplicate"},
		{name: "control", contract: []string{"line\nbreak"}, contains: "control character"},
		{name: "oversized", contract: []string{strings.Repeat("x", maximumCodecContractTextBytes+1)}, contains: "exceeds"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := DeriveCodecSpecificationV1(id, version, shape, testCase.contract)
			if err == nil || !strings.Contains(err.Error(), testCase.contains) {
				t.Fatalf("error = %v, want substring %q", err, testCase.contains)
			}
		})
	}
}

func TestCodecSpecificationV1DecoderRejectsNonCanonicalAndWrongReference(t *testing.T) {
	id, version, shape := codecSpecificationFixtureCoordinates(t, "Haft.Codec.Text", "v1", "Haft.Shape.Text", 0x41)
	specification := mustDeriveCodecSpecification(t, id, version, shape, []string{"exact contract"})

	trailing := append(specification.CanonicalBytes(), byte(0))
	if _, err := DecodeCodecSpecificationV1(trailing); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing-byte error = %v", err)
	}

	writer := newCodecSpecificationWriter(codecSpecificationArtifactDomain)
	payload := fmt.Sprintf(
		`{"codec_id":"Haft.Codec.Text","canonicalization_version":"v1","value_shape_id":"Haft.Shape.Text","value_shape_digest":"%s","ordered_contract_clauses":["exact contract"],"unknown":true}`,
		shape.Digest().String(),
	)
	writer.addBytes([]byte(payload))
	if _, err := DecodeCodecSpecificationV1(writer.bytes()); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("unknown-field error = %v", err)
	}

	otherID, _, _ := codecSpecificationFixtureCoordinates(t, "Haft.Codec.Other", "v1", "Haft.Shape.Text", 0x41)
	other := mustDeriveCodecSpecification(t, otherID, version, shape, []string{"exact contract"})
	if _, err := VerifyCodecSpecificationV1(other.Ref(), specification.CanonicalBytes()); err == nil ||
		!strings.Contains(err.Error(), "does not match") {
		t.Fatalf("wrong-reference error = %v", err)
	}
}

func codecSpecificationFixtureCoordinates(
	t *testing.T,
	codecRaw string,
	versionRaw string,
	shapeRaw string,
	digestByte byte,
) (typedmemory.CodecID, typedmemory.CanonicalizationVersion, typedmemory.ValueShapeRef) {
	t.Helper()
	id, err := typedmemory.NewCodecID(codecRaw)
	if err != nil {
		t.Fatalf("NewCodecID() error = %v", err)
	}
	version, err := typedmemory.NewCanonicalizationVersion(versionRaw)
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion() error = %v", err)
	}
	shapeID, err := typedmemory.NewShapeID(shapeRaw)
	if err != nil {
		t.Fatalf("NewShapeID() error = %v", err)
	}
	hexDigits := "0123456789abcdef"
	nibble := hexDigits[int(digestByte)%len(hexDigits)]
	digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(string(nibble), 64))
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	shape, err := typedmemory.NewValueShapeRef(shapeID, digest)
	if err != nil {
		t.Fatalf("NewValueShapeRef() error = %v", err)
	}
	return id, version, shape
}

func mustDeriveCodecSpecification(
	t *testing.T,
	id typedmemory.CodecID,
	version typedmemory.CanonicalizationVersion,
	shape typedmemory.ValueShapeRef,
	contract []string,
) CodecSpecificationV1 {
	t.Helper()
	specification, err := DeriveCodecSpecificationV1(id, version, shape, contract)
	if err != nil {
		t.Fatalf("DeriveCodecSpecificationV1() error = %v", err)
	}
	return specification
}
