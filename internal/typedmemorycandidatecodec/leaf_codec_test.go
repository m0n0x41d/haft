package typedmemorycandidatecodec

import (
	"bytes"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestTextV1PreservesExactUnicodeAndRejectsMalformedWire(t *testing.T) {
	suite := testSuite(t)
	codec := suite.Text()
	composed := requireCanonical(t, codec.EncodeInput("é"))
	decomposed := requireCanonical(t, codec.EncodeInput("e\u0301"))
	if bytes.Equal(composed.CanonicalBytes(), decomposed.CanonicalBytes()) {
		t.Fatal("TextV1 implicitly normalized distinct Unicode scalar sequences")
	}
	assertIdempotent(t, codec, codec.Shape(), composed)
	requireRejected(t, codec.EncodeInput(""))

	trailing := append(composed.CanonicalBytes(), 0)
	requireRejected(t, codec.Canonicalize(codec.Shape(), trailing))
	truncated := composed.CanonicalBytes()[:len(composed.CanonicalBytes())-1]
	requireRejected(t, codec.Canonicalize(codec.Shape(), truncated))
	wrongDomain := newCanonicalWriter("not-text.v1").addString("value").result()
	requireRejected(t, codec.Canonicalize(codec.Shape(), wrongDomain))
	invalidUTF8 := newCanonicalWriter(textCodecDomain).addBytes([]byte{0xff}).result()
	requireRejected(t, codec.Canonicalize(codec.Shape(), invalidUTF8))

	wrongShape := suite.CanonicalInstant().Shape()
	rejected := requireRejected(t, codec.Canonicalize(wrongShape, composed.CanonicalBytes()))
	if rejected.Issues()[0].Code() != typedmemory.DiagnosticValueShapeMismatch {
		t.Fatalf("wrong-shape code = %q", rejected.Issues()[0].Code())
	}
}

func TestEvidencePolarityV1AcceptsOnlyCore5(t *testing.T) {
	suite := testSuite(t)
	codec := suite.EvidencePolarity()
	values := []EvidencePolarity{
		EvidenceConfirming,
		EvidenceRebutting,
		EvidenceUndercutting,
		EvidenceConstraining,
		EvidenceNeutral,
	}
	seen := map[string]struct{}{}
	for _, value := range values {
		canonical := requireCanonical(t, codec.EncodeInput(value))
		assertIdempotent(t, codec, codec.Shape(), canonical)
		digest := canonicalDigest(canonical)
		if _, duplicate := seen[digest]; duplicate {
			t.Fatalf("Core5 token %q collides", value)
		}
		seen[digest] = struct{}{}
	}
	for _, value := range []EvidencePolarity{
		"Confirming",
		"supporting",
		"stale",
		"approved",
		"",
	} {
		requireRejected(t, codec.EncodeInput(value))
	}
	unknownWire := newCanonicalWriter(evidencePolarityCodecDomain)
	unknownWire = unknownWire.addString("supporting")
	requireRejected(t, codec.Canonicalize(codec.Shape(), unknownWire.result()))
}

func TestCanonicalInstantV1NormalizesOffsetsAndFractions(t *testing.T) {
	suite := testSuite(t)
	codec := suite.CanonicalInstant()
	tests := []struct {
		input string
		want  string
	}{
		{input: "2026-07-17T12:34:56Z", want: "2026-07-17T12:34:56Z"},
		{input: "2026-07-17T12:34:56.120000000Z", want: "2026-07-17T12:34:56.12Z"},
		{input: "2026-01-01T00:30:00+01:00", want: "2025-12-31T23:30:00Z"},
		{input: "2026-01-01T00:00:00-14:00", want: "2026-01-01T14:00:00Z"},
		{input: "2024-02-29T23:59:59.000000001+00:00", want: "2024-02-29T23:59:59.000000001Z"},
	}
	for _, test := range tests {
		canonical := requireCanonical(t, codec.EncodeInput(test.input))
		value, ok := canonical.Value().(typedmemory.ScalarTypedValue)
		if !ok {
			t.Fatalf("%q value = %T", test.input, canonical.Value())
		}
		got, text := value.Text()
		if !text || got != test.want {
			t.Fatalf("%q canonical text = %q, want %q", test.input, got, test.want)
		}
		assertIdempotent(t, codec, codec.Shape(), canonical)
	}

	first := requireCanonical(t, codec.EncodeInput("2026-01-01T00:00:00Z"))
	second := requireCanonical(t, codec.EncodeInput("2026-01-01T01:00:00+01:00"))
	if !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatal("equal conceptual instants have unequal canonical bytes")
	}
}

func TestCanonicalInstantV1RejectsEveryForbiddenBoundary(t *testing.T) {
	codec := testSuite(t).CanonicalInstant()
	invalid := []string{
		"0000-01-01T00:00:00Z",
		"2026-02-29T00:00:00Z",
		"2024-02-30T00:00:00Z",
		"2026-01-01T24:00:00Z",
		"2026-01-01T23:60:00Z",
		"2026-01-01T23:59:60Z",
		"2026-01-01t00:00:00Z",
		"2026-01-01T00:00:00z",
		" 2026-01-01T00:00:00Z",
		"2026-01-01T00:00:00Z ",
		"2026-01-01T00:00:00",
		"2026-01-01T00:00:00-00:00",
		"2026-01-01T00:00:00+14:01",
		"2026-01-01T00:00:00-14:01",
		"2026-01-01T00:00:00+15:00",
		"2026-01-01T00:00:00.1234567890Z",
		"0001-01-01T00:00:00+14:00",
		"9999-12-31T23:59:59-14:00",
	}
	for _, value := range invalid {
		if _, err := ParseCanonicalInstant(value); err == nil {
			t.Errorf("ParseCanonicalInstant(%q) accepted forbidden input", value)
		}
		requireRejected(t, codec.EncodeInput(value))
	}
}

func TestLeafCodecGoldenDigests(t *testing.T) {
	suite := testSuite(t)
	values := map[string]typedmemory.CanonicalizedCodecValue{
		"text":     requireCanonical(t, suite.Text().EncodeInput("Haft")),
		"polarity": requireCanonical(t, suite.EvidencePolarity().EncodeInput(EvidenceNeutral)),
		"instant":  requireCanonical(t, suite.CanonicalInstant().EncodeInput("2026-07-17T12:34:56.12Z")),
	}
	want := map[string]string{
		"text":     "3a3a13fa9102aa2dad665d20ec6d0cc6bf9a1ce699618c9d94675e509212f5bc",
		"polarity": "1b8543de0a5e555de05f0703da346866f5fbf73bd323b8df8f333bde001cba8b",
		"instant":  "1dad4782aad3c9eac9c9d02ca0589da3901188a7950b486c2cfe7839683481a7",
	}
	for label, value := range values {
		if got := canonicalDigest(value); got != want[label] {
			t.Errorf("%s golden digest = %s, want %s", label, got, want[label])
		}
	}
}
