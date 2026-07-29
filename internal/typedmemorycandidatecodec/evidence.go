package typedmemorycandidatecodec

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const evidenceUseQualifierCodecDomain = "haft.local-practice.typed-memory.candidate-codec.evidence-use-qualifier.v1"

// EvidenceUseQualifier is the candidate Haft-local record containing exactly
// one polarity value. It is not an exact U.Evidence object.
type EvidenceUseQualifier struct {
	polarity EvidencePolarity
}

func NewEvidenceUseQualifier(
	polarity EvidencePolarity,
) (EvidenceUseQualifier, error) {
	if !polarity.valid() {
		return EvidenceUseQualifier{}, fmt.Errorf("evidence polarity %q is outside Core5", polarity)
	}
	return EvidenceUseQualifier{polarity: polarity}, nil
}

func (qualifier EvidenceUseQualifier) Polarity() EvidencePolarity {
	return qualifier.polarity
}

func (qualifier EvidenceUseQualifier) valid() bool {
	return qualifier.polarity.valid()
}

// EvidenceUseQualifierV1 implements the candidate record codec and delegates
// the nested polarity semantics to the exact EvidencePolarityV1 mechanism.
type EvidenceUseQualifierV1 struct {
	shape    typedmemory.ValueShapeRef
	polarity EvidencePolarityV1
}

func (codec EvidenceUseQualifierV1) Shape() typedmemory.ValueShapeRef {
	return codec.shape
}

func (codec EvidenceUseQualifierV1) Canonicalize(
	expectedShape typedmemory.ValueShapeRef,
	inputBytes []byte,
) typedmemory.CodecCanonicalization {
	if expectedShape != codec.shape {
		return rejectShape("EvidenceUseQualifierV1", codec.shape, expectedShape)
	}
	value, err := decodeEvidenceUseQualifierWire(inputBytes)
	if err != nil {
		return rejectMalformed(
			"EvidenceUseQualifierV1",
			"typed_value.evidence_use_qualifier",
			err,
		)
	}
	return codec.canonicalizeValue(value)
}

func (codec EvidenceUseQualifierV1) EncodeInput(
	value EvidenceUseQualifier,
) typedmemory.CodecCanonicalization {
	if !value.valid() {
		err := fmt.Errorf("evidence-use qualifier is incomplete")
		return rejectMalformed(
			"EvidenceUseQualifierV1",
			"typed_value.evidence_use_qualifier",
			err,
		)
	}
	return codec.canonicalizeValue(value)
}

func (codec EvidenceUseQualifierV1) canonicalizeValue(
	value EvidenceUseQualifier,
) typedmemory.CodecCanonicalization {
	polarityResult := codec.polarity.EncodeInput(value.Polarity())
	polarityCanonical, ok := polarityResult.(typedmemory.CanonicalizedCodecValue)
	if !ok {
		return polarityResult
	}
	typed, err := newTypedRecord([]typedField{
		{name: "polarity", value: polarityCanonical.Value()},
	})
	if err != nil {
		return rejectMalformed(
			"EvidenceUseQualifierV1",
			"typed_value.evidence_use_qualifier",
			err,
		)
	}
	canonical := encodeEvidenceUseQualifierWire(value)
	return acceptCanonical("EvidenceUseQualifierV1", typed, canonical)
}

func encodeEvidenceUseQualifierWire(value EvidenceUseQualifier) []byte {
	polarity := encodeEvidencePolarityWire(value.Polarity())
	writer := newCanonicalWriter(evidenceUseQualifierCodecDomain)
	writer = writer.addBytes(polarity)
	return writer.result()
}

func decodeEvidenceUseQualifierWire(input []byte) (EvidenceUseQualifier, error) {
	reader, err := newCanonicalReader(input, evidenceUseQualifierCodecDomain)
	if err != nil {
		return EvidenceUseQualifier{}, err
	}
	polarityBytes, reader, err := reader.readBytes()
	if err != nil {
		return EvidenceUseQualifier{}, err
	}
	if err := reader.requireEnd(); err != nil {
		return EvidenceUseQualifier{}, err
	}
	polarity, err := decodeEvidencePolarityWire(polarityBytes)
	if err != nil {
		return EvidenceUseQualifier{}, fmt.Errorf("polarity: %w", err)
	}
	return NewEvidenceUseQualifier(polarity)
}

var _ typedmemory.CodecImplementation = EvidenceUseQualifierV1{}
