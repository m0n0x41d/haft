package typedmemorycandidatecodec

import (
	"fmt"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const textCodecDomain = "haft.local-practice.typed-memory.candidate-codec.text.v1"

// TextV1 implements the candidate Haft.Text contract. It preserves the exact
// Unicode scalar sequence and performs no trimming or normalization.
type TextV1 struct {
	shape typedmemory.ValueShapeRef
}

func (codec TextV1) Shape() typedmemory.ValueShapeRef { return codec.shape }

func (codec TextV1) Canonicalize(
	expectedShape typedmemory.ValueShapeRef,
	inputBytes []byte,
) typedmemory.CodecCanonicalization {
	if expectedShape != codec.shape {
		return rejectShape("TextV1", codec.shape, expectedShape)
	}
	value, err := decodeTextWire(inputBytes)
	if err != nil {
		return rejectMalformed("TextV1", "typed_value.text", err)
	}
	canonical := encodeTextWire(value)
	return acceptCanonical("TextV1", typedmemory.NewTextValue(value), canonical)
}

func (codec TextV1) EncodeInput(value string) typedmemory.CodecCanonicalization {
	if err := validateText(value); err != nil {
		return rejectMalformed("TextV1", "typed_value.text", err)
	}
	canonical := encodeTextWire(value)
	return acceptCanonical("TextV1", typedmemory.NewTextValue(value), canonical)
}

func validateText(value string) error {
	if value == "" {
		return fmt.Errorf("text is empty")
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("text is not valid UTF-8")
	}
	return nil
}

func encodeTextWire(value string) []byte {
	writer := newCanonicalWriter(textCodecDomain)
	writer = writer.addString(value)
	return writer.result()
}

func decodeTextWire(input []byte) (string, error) {
	reader, err := newCanonicalReader(input, textCodecDomain)
	if err != nil {
		return "", err
	}
	value, reader, err := reader.readString()
	if err != nil {
		return "", err
	}
	if err := reader.requireEnd(); err != nil {
		return "", err
	}
	if err := validateText(value); err != nil {
		return "", err
	}
	return value, nil
}

var _ typedmemory.CodecImplementation = TextV1{}
