package typedmemorycandidatecodec

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCandidateCodecsRejectFramingAttacksAndRemainIdempotent(t *testing.T) {
	suite := testSuite(t)
	qualifier, _ := NewEvidenceUseQualifier(EvidenceNeutral)
	interval := mustInFlightInterval(t, "2026-07-17T12:34:56Z")
	target, _ := NewFileCodeAnchorTarget("internal/auth.go")
	locator, _ := NewCodeAnchorLocator("repo", "revision", target)
	cases := []struct {
		name  string
		codec typedmemory.CodecImplementation
		shape typedmemory.ValueShapeRef
		value typedmemory.CanonicalizedCodecValue
	}{
		{name: "text", codec: suite.Text(), shape: suite.Text().Shape(), value: requireCanonical(t, suite.Text().EncodeInput("text"))},
		{name: "polarity", codec: suite.EvidencePolarity(), shape: suite.EvidencePolarity().Shape(), value: requireCanonical(t, suite.EvidencePolarity().EncodeInput(EvidenceNeutral))},
		{name: "instant", codec: suite.CanonicalInstant(), shape: suite.CanonicalInstant().Shape(), value: requireCanonical(t, suite.CanonicalInstant().EncodeInput("2026-07-17T12:34:56Z"))},
		{name: "qualifier", codec: suite.EvidenceUseQualifier(), shape: suite.EvidenceUseQualifier().Shape(), value: requireCanonical(t, suite.EvidenceUseQualifier().EncodeInput(qualifier))},
		{name: "interval", codec: suite.PerformedInterval(), shape: suite.PerformedInterval().Shape(), value: requireCanonical(t, suite.PerformedInterval().EncodeInput(interval))},
		{name: "anchor", codec: suite.CodeAnchorLocator(), shape: suite.CodeAnchorLocator().Shape(), value: requireCanonical(t, suite.CodeAnchorLocator().EncodeInput(locator))},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			canonical := test.value.CanonicalBytes()
			assertIdempotent(t, test.codec, test.shape, test.value)
			requireRejected(t, test.codec.Canonicalize(test.shape, append(canonical, 0)))
			requireRejected(t, test.codec.Canonicalize(test.shape, canonical[:len(canonical)-1]))

			oversizedDomain := append([]byte(nil), canonical...)
			binary.BigEndian.PutUint64(oversizedDomain[:8], ^uint64(0))
			requireRejected(t, test.codec.Canonicalize(test.shape, oversizedDomain))

			wrongEnvelope := append([]byte(nil), canonical...)
			wrongEnvelope[8] ^= 0x01
			requireRejected(t, test.codec.Canonicalize(test.shape, wrongEnvelope))

			for index := range canonical {
				mutated := append([]byte(nil), canonical...)
				mutated[index] ^= 0x01
				result := test.codec.Canonicalize(test.shape, mutated)
				accepted, ok := result.(typedmemory.CanonicalizedCodecValue)
				if !ok {
					requireRejected(t, result)
					continue
				}
				roundTrip := requireCanonical(
					t,
					test.codec.Canonicalize(test.shape, accepted.CanonicalBytes()),
				)
				if !bytes.Equal(accepted.CanonicalBytes(), roundTrip.CanonicalBytes()) {
					t.Fatalf("accepted byte mutation %d is not idempotent", index)
				}
			}
		})
	}
}

func TestCanonicalInstantPropertyCorpus(t *testing.T) {
	codec := testSuite(t).CanonicalInstant()
	fractions := []string{"", ".1", ".010", ".123456789"}
	offsets := []string{"Z", "+00:00", "+05:30", "-04:00", "+14:00", "-14:00"}
	for _, fraction := range fractions {
		for _, offset := range offsets {
			raw := "2026-07-17T12:34:56" + fraction + offset
			wire := encodeRawInstantWireForTest(raw)
			canonical := requireCanonical(t, codec.Canonicalize(codec.Shape(), wire))
			assertIdempotent(t, codec, codec.Shape(), canonical)
		}
	}
}
