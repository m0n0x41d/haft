package typedmemorycandidatecodec

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const evidencePolarityCodecDomain = "haft.local-practice.typed-memory.candidate-codec.evidence-polarity.v1"

// EvidencePolarity is the closed Core5 token set selected by the candidate
// local-practice contract. It has no freshness, assurance, authority, gate, or
// lifecycle meaning.
type EvidencePolarity string

const (
	EvidenceConfirming   EvidencePolarity = "confirming"
	EvidenceRebutting    EvidencePolarity = "rebutting"
	EvidenceUndercutting EvidencePolarity = "undercutting"
	EvidenceConstraining EvidencePolarity = "constraining"
	EvidenceNeutral      EvidencePolarity = "neutral"
)

func (polarity EvidencePolarity) String() string { return string(polarity) }

func (polarity EvidencePolarity) valid() bool {
	switch polarity {
	case EvidenceConfirming,
		EvidenceRebutting,
		EvidenceUndercutting,
		EvidenceConstraining,
		EvidenceNeutral:
		return true
	default:
		return false
	}
}

// EvidencePolarityV1 implements only the candidate Haft-local polarity token
// contract. It does not assert exact FPF Evidence semantics.
type EvidencePolarityV1 struct {
	shape typedmemory.ValueShapeRef
}

func (codec EvidencePolarityV1) Shape() typedmemory.ValueShapeRef {
	return codec.shape
}

func (codec EvidencePolarityV1) Canonicalize(
	expectedShape typedmemory.ValueShapeRef,
	inputBytes []byte,
) typedmemory.CodecCanonicalization {
	if expectedShape != codec.shape {
		return rejectShape("EvidencePolarityV1", codec.shape, expectedShape)
	}
	value, err := decodeEvidencePolarityWire(inputBytes)
	if err != nil {
		return rejectMalformed(
			"EvidencePolarityV1",
			"typed_value.evidence_polarity",
			err,
		)
	}
	canonical := encodeEvidencePolarityWire(value)
	typed := typedmemory.NewTextValue(value.String())
	return acceptCanonical("EvidencePolarityV1", typed, canonical)
}

func (codec EvidencePolarityV1) EncodeInput(
	value EvidencePolarity,
) typedmemory.CodecCanonicalization {
	if !value.valid() {
		err := fmt.Errorf("evidence polarity token %q is outside Core5", value)
		return rejectMalformed(
			"EvidencePolarityV1",
			"typed_value.evidence_polarity",
			err,
		)
	}
	canonical := encodeEvidencePolarityWire(value)
	typed := typedmemory.NewTextValue(value.String())
	return acceptCanonical("EvidencePolarityV1", typed, canonical)
}

func encodeEvidencePolarityWire(value EvidencePolarity) []byte {
	writer := newCanonicalWriter(evidencePolarityCodecDomain)
	writer = writer.addString(value.String())
	return writer.result()
}

func decodeEvidencePolarityWire(input []byte) (EvidencePolarity, error) {
	reader, err := newCanonicalReader(input, evidencePolarityCodecDomain)
	if err != nil {
		return "", err
	}
	token, reader, err := reader.readString()
	if err != nil {
		return "", err
	}
	if err := reader.requireEnd(); err != nil {
		return "", err
	}
	value := EvidencePolarity(token)
	if !value.valid() {
		return "", fmt.Errorf("evidence polarity token %q is outside Core5", token)
	}
	return value, nil
}

var _ typedmemory.CodecImplementation = EvidencePolarityV1{}
