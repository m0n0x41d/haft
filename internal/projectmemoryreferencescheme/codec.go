package projectmemoryreferencescheme

import "fmt"

// ProjectMemoryReferenceSchemeV1Codec is the strict codec for the intrinsic
// V1 canonical value. Decode accepts canonical V1 bytes only; Encode refuses
// zero, forged, or field/byte-divergent values. It does not migrate or decode
// the legacy artifact ReferenceScheme form with Primary and Anchors fields.
type ProjectMemoryReferenceSchemeV1Codec struct{}

func NewProjectMemoryReferenceSchemeV1Codec() ProjectMemoryReferenceSchemeV1Codec {
	return ProjectMemoryReferenceSchemeV1Codec{}
}

func (ProjectMemoryReferenceSchemeV1Codec) Encode(
	scheme ProjectMemoryReferenceSchemeV1,
) ([]byte, error) {
	if err := scheme.Verify(); err != nil {
		return nil, fmt.Errorf("encode project-memory reference scheme: %w", err)
	}
	return scheme.CanonicalBytes(), nil
}

func (ProjectMemoryReferenceSchemeV1Codec) Decode(
	canonical []byte,
) (ProjectMemoryReferenceSchemeV1, error) {
	scheme, err := DecodeProjectMemoryReferenceSchemeV1(canonical)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, fmt.Errorf(
			"decode project-memory reference scheme: %w",
			err,
		)
	}
	return scheme, nil
}

func (ProjectMemoryReferenceSchemeV1Codec) Verify(
	expected ReferenceSchemeDigest,
	canonical []byte,
) (ProjectMemoryReferenceSchemeV1, error) {
	scheme, err := VerifyProjectMemoryReferenceSchemeV1(expected, canonical)
	if err != nil {
		return ProjectMemoryReferenceSchemeV1{}, fmt.Errorf(
			"verify project-memory reference scheme: %w",
			err,
		)
	}
	return scheme, nil
}
