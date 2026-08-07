package artifact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"
)

// EncodeDecideInputCanonicalJSON applies the same normalization used by
// BuildDecisionArtifact and returns the one canonical JSON representation of
// those semantics. It is pure and performs no decision validation or I/O.
func EncodeDecideInputCanonicalJSON(input DecideInput) ([]byte, error) {
	normalized := normalizeDecisionInput(input)
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode canonical DecideInput: %w", err)
	}
	return canonical, nil
}

// DecodeDecideInputCanonicalJSON admits only exact bytes produced by
// EncodeDecideInputCanonicalJSON. Unknown fields, duplicate-key layouts,
// alternate whitespace/order, trailing values, and semantically
// non-normalized inputs fail closed.
func DecodeDecideInputCanonicalJSON(data []byte) (DecideInput, error) {
	if len(data) == 0 {
		return DecideInput{}, fmt.Errorf("canonical DecideInput bytes are required")
	}
	if !utf8.Valid(data) {
		return DecideInput{}, fmt.Errorf("canonical DecideInput must be valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoded := DecideInput{}
	if err := decoder.Decode(&decoded); err != nil {
		return DecideInput{}, fmt.Errorf("decode exact DecideInput: %w", err)
	}
	trailing := json.RawMessage{}
	trailingErr := decoder.Decode(&trailing)
	if trailingErr != io.EOF {
		return DecideInput{}, fmt.Errorf("DecideInput must contain exactly one JSON value")
	}
	normalized := normalizeDecisionInput(decoded)
	canonical, err := EncodeDecideInputCanonicalJSON(normalized)
	if err != nil {
		return DecideInput{}, err
	}
	if !bytes.Equal(data, canonical) {
		return DecideInput{}, fmt.Errorf(
			"DecideInput bytes are not the exact canonical representation of normalized decision semantics",
		)
	}
	return normalized, nil
}
