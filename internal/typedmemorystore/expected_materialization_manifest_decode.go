package typedmemorystore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const canonicalStorageEnvelopeDomain = "haft.typedmemorystore.canonical.v1"

func decodeExpectedMaterializationManifest(
	canonical []byte,
	storedDigest typedmemory.SHA256Digest,
	basisRevision uint64,
) (expectedMaterializationManifest, error) {
	actualDigest, err := digestBytes(canonical)
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	if actualDigest != storedDigest {
		return expectedMaterializationManifest{}, storedAdmissionIntegrity(
			"expected-materialization manifest digest",
			nil,
		)
	}
	fields, err := decodeCanonicalStorageFields(
		canonical,
		"typed-memory-expected-materialization-manifest.v1",
	)
	if err != nil {
		return expectedMaterializationManifest{}, storedAdmissionIntegrity(
			"expected-materialization manifest carrier",
			err,
		)
	}
	reader := manifestFieldReader{fields: fields}
	requestDigest, err := reader.digest("request digest")
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	semanticDigest, err := reader.digest("semantic digest")
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	basisDigest, err := reader.digest("basis digest")
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	declarations, err := decodeManifestSet(
		&reader,
		"declarations",
		decodeExpectedDeclarationCoordinate,
	)
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	semanticRows, err := decodeManifestSet(
		&reader,
		"semantic_rows",
		decodeExpectedSemanticRowIdentity,
	)
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	resolutions, err := decodeManifestSet(
		&reader,
		"resolutions",
		decodeExpectedResolutionWitness,
	)
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	evaluations, err := decodeManifestSet(
		&reader,
		"evaluations",
		decodeExpectedEvaluationWitness,
	)
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	inputs, err := decodeManifestSet(
		&reader,
		"observable_inputs",
		decodeExpectedObservableInputTuple,
	)
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	uses, err := decodeManifestSet(
		&reader,
		"member_uses",
		decodeExpectedMemberUseCoordinate,
	)
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	prefixes, err := decodeManifestSet(
		&reader,
		"ordered_prefixes",
		decodeExpectedOrderedCandidatePrefix,
	)
	if err != nil {
		return expectedMaterializationManifest{}, err
	}
	if !reader.finished() {
		return expectedMaterializationManifest{}, storedAdmissionIntegrity(
			"expected-materialization manifest trailing fields",
			nil,
		)
	}
	manifest := expectedMaterializationManifest{
		requestDigest:    requestDigest,
		semanticDigest:   semanticDigest,
		basisDigest:      basisDigest,
		basisRevision:    basisRevision,
		declarations:     declarations,
		semanticRows:     semanticRows,
		resolutions:      resolutions,
		evaluations:      evaluations,
		observableInputs: inputs,
		memberUses:       uses,
		orderedPrefixes:  prefixes,
		canonicalBytes:   append([]byte(nil), canonical...),
		digest:           actualDigest,
	}
	rebuilt := canonicalExpectedMaterializationManifest(manifest)
	if !bytes.Equal(rebuilt, canonical) {
		return expectedMaterializationManifest{}, storedAdmissionIntegrity(
			"expected-materialization manifest canonical round trip",
			nil,
		)
	}
	return manifest, nil
}

func canonicalExpectedMaterializationManifest(
	manifest expectedMaterializationManifest,
) []byte {
	fields := []string{
		manifest.requestDigest.String(),
		manifest.semanticDigest.String(),
		manifest.basisDigest.String(),
	}
	fields = appendManifestSet(fields, "declarations", manifest.declarations, func(value expectedDeclarationCoordinate) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "semantic_rows", manifest.semanticRows, func(value expectedSemanticRowIdentity) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "resolutions", manifest.resolutions, func(value expectedResolutionWitness) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "evaluations", manifest.evaluations, func(value expectedEvaluationWitness) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "observable_inputs", manifest.observableInputs, func(value expectedObservableInputTuple) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "member_uses", manifest.memberUses, func(value expectedMemberUseCoordinate) []byte {
		return value.canonicalBytes
	})
	fields = appendManifestSet(fields, "ordered_prefixes", manifest.orderedPrefixes, func(value expectedOrderedCandidatePrefix) []byte {
		return value.canonicalBytes
	})
	return canonicalStorageFields(
		"typed-memory-expected-materialization-manifest.v1",
		fields,
	)
}

type manifestFieldReader struct {
	fields []string
	index  int
}

func (reader *manifestFieldReader) next(detail string) (string, error) {
	if reader.index >= len(reader.fields) {
		return "", storedAdmissionIntegrity("expected-materialization manifest missing "+detail, nil)
	}
	value := reader.fields[reader.index]
	reader.index++
	return value, nil
}

func (reader *manifestFieldReader) digest(detail string) (typedmemory.SHA256Digest, error) {
	raw, err := reader.next(detail)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity(
			"expected-materialization manifest "+detail,
			err,
		)
	}
	return digest, nil
}

func (reader *manifestFieldReader) finished() bool {
	return reader.index == len(reader.fields)
}

func decodeManifestSet[T any](
	reader *manifestFieldReader,
	name string,
	decode func([]byte) (T, error),
) ([]T, error) {
	actualName, err := reader.next(name + " name")
	if err != nil {
		return nil, err
	}
	if actualName != name {
		return nil, storedAdmissionIntegrity("expected-materialization manifest set order", nil)
	}
	countRaw, err := reader.next(name + " count")
	if err != nil {
		return nil, err
	}
	count, err := strconv.ParseUint(countRaw, 10, 31)
	if err != nil {
		return nil, storedAdmissionIntegrity("expected-materialization manifest "+name+" count", err)
	}
	boundedCount, exact := sliceIndexFromUint64(count)
	if !exact {
		return nil, storedAdmissionIntegrity(
			"expected-materialization manifest "+name+" count exceeds platform capacity",
			nil,
		)
	}
	remainingFields := len(reader.fields) - reader.index
	if boundedCount > remainingFields {
		return nil, storedAdmissionIntegrity(
			"expected-materialization manifest "+name+" count exceeds carrier",
			nil,
		)
	}
	result := make([]T, 0, boundedCount)
	var previous []byte
	for index := 0; index < boundedCount; index++ {
		raw, nextErr := reader.next(name + " entry")
		if nextErr != nil {
			return nil, nextErr
		}
		canonical := []byte(raw)
		if previous != nil && bytes.Compare(previous, canonical) >= 0 {
			return nil, storedAdmissionIntegrity("expected-materialization manifest "+name+" order", nil)
		}
		value, decodeErr := decode(canonical)
		if decodeErr != nil {
			return nil, decodeErr
		}
		result = append(result, value)
		previous = append([]byte(nil), canonical...)
	}
	return result, nil
}

func decodeExpectedDeclarationCoordinate(raw []byte) (expectedDeclarationCoordinate, error) {
	fields, err := requireCanonicalFields(raw, "typed-memory-expected-declaration-coordinate.v1", 8)
	if err != nil {
		return expectedDeclarationCoordinate{}, err
	}
	ordinal, err := parseManifestUint(fields[0], "declaration ordinal")
	if err != nil {
		return expectedDeclarationCoordinate{}, err
	}
	digest, err := parseManifestDigest(fields[6], "declaration digest")
	if err != nil {
		return expectedDeclarationCoordinate{}, err
	}
	value := expectedDeclarationCoordinate{
		changeOrdinal:     ordinal,
		entityID:          fields[1],
		batchLocalRef:     fields[2],
		boundedContextRef: fields[3],
		label:             fields[4],
		provenanceRef:     fields[5],
		declarationDigest: digest,
		declarationBytes:  []byte(fields[7]),
		canonicalBytes:    append([]byte(nil), raw...),
	}
	return value, requireCanonicalRoundTrip(raw, canonicalDeclarationCoordinate(value), "declaration coordinate")
}

func decodeExpectedSemanticRowIdentity(raw []byte) (expectedSemanticRowIdentity, error) {
	fields, err := decodeCanonicalStorageFields(raw, "typed-memory-expected-semantic-row.v1")
	if err != nil || len(fields) < 5 {
		return expectedSemanticRowIdentity{}, storedAdmissionIntegrity("expected semantic row carrier", err)
	}
	count, err := strconv.ParseUint(fields[2], 10, 31)
	if err != nil || uint64(len(fields)) != count+5 {
		return expectedSemanticRowIdentity{}, storedAdmissionIntegrity("expected semantic row coordinate count", err)
	}
	conditional := fields[1] == "conditional"
	if !conditional && fields[1] != "required" {
		return expectedSemanticRowIdentity{}, storedAdmissionIntegrity("expected semantic row condition", nil)
	}
	digestIndex := 3 + int(count)
	digest, err := parseManifestDigest(fields[digestIndex], "semantic row digest")
	if err != nil {
		return expectedSemanticRowIdentity{}, err
	}
	value := newExpectedSemanticRowIdentity(
		fields[0],
		fields[3:digestIndex],
		digest,
		[]byte(fields[digestIndex+1]),
		conditional,
	)
	return value, requireCanonicalRoundTrip(raw, value.canonicalBytes, "semantic row")
}

func decodeExpectedResolutionWitness(raw []byte) (expectedResolutionWitness, error) {
	fields, err := requireCanonicalFields(raw, "typed-memory-expected-resolution-witness.v1", 15)
	if err != nil {
		return expectedResolutionWitness{}, err
	}
	filler, err := decodeExpectedFillerCoordinate(fields[:5])
	if err != nil {
		return expectedResolutionWitness{}, err
	}
	resolutionDigest, err := parseManifestDigest(fields[7], "resolution digest")
	if err != nil {
		return expectedResolutionWitness{}, err
	}
	value := expectedResolutionWitness{
		coordinate:                   filler,
		entityID:                     fields[5],
		resolutionKind:               fields[6],
		resolutionDigest:             resolutionDigest,
		resolutionBytes:              []byte(fields[8]),
		resolutionBasisRef:           fields[9],
		declarationChangeOrdinal:     fields[10],
		localReferenceKindRef:        fields[11],
		batchLocalRef:                fields[12],
		declarationDigest:            fields[13],
		orderedCandidatePrefixDigest: fields[14],
		canonicalBytes:               append([]byte(nil), raw...),
	}
	return value, requireCanonicalRoundTrip(raw, canonicalResolutionWitness(value), "resolution witness")
}

func decodeExpectedEvaluationWitness(raw []byte) (expectedEvaluationWitness, error) {
	fields, err := requireCanonicalFields(raw, "typed-memory-expected-evaluation-witness.v1", 24)
	if err != nil {
		return expectedEvaluationWitness{}, err
	}
	digests := make([]typedmemory.SHA256Digest, 0, 7)
	for _, index := range []int{8, 17, 18, 20, 22} {
		digest, digestErr := parseManifestDigest(fields[index], "evaluation witness digest")
		if digestErr != nil {
			return expectedEvaluationWitness{}, digestErr
		}
		digests = append(digests, digest)
	}
	count, err := parseManifestUint(fields[16], "observable input count")
	if err != nil {
		return expectedEvaluationWitness{}, err
	}
	value := expectedEvaluationWitness{
		evaluationRef:                    fields[0],
		judgementKind:                    fields[1],
		entityID:                         fields[2],
		valueKindRef:                     fields[3],
		contextSliceRef:                  fields[4],
		evaluatorRuleRef:                 fields[5],
		evaluationProvenanceRef:          fields[6],
		evaluationViewKind:               fields[7],
		evaluationViewDigest:             digests[0],
		evaluationViewBytes:              []byte(fields[9]),
		viewDeclarationChangeOrdinal:     fields[10],
		viewLocalReferenceKindRef:        fields[11],
		viewBatchLocalRef:                fields[12],
		viewDeclarationDigest:            fields[13],
		viewPrefixEndOrdinal:             fields[14],
		viewOrderedCandidatePrefixDigest: fields[15],
		observableInputCount:             count,
		observableInputSetDigest:         digests[1],
		queryDigest:                      digests[2],
		queryBytes:                       []byte(fields[19]),
		basisDigest:                      digests[3],
		basisBytes:                       []byte(fields[21]),
		judgementDigest:                  digests[4],
		judgementBytes:                   []byte(fields[23]),
		canonicalBytes:                   append([]byte(nil), raw...),
	}
	return value, requireCanonicalRoundTrip(raw, canonicalEvaluationWitness(value), "evaluation witness")
}

func decodeExpectedObservableInputTuple(raw []byte) (expectedObservableInputTuple, error) {
	fields, err := requireCanonicalFields(raw, "typed-memory-expected-observable-input-tuple.v1", 5)
	if err != nil {
		return expectedObservableInputTuple{}, err
	}
	ordinal, err := parseManifestUint(fields[1], "observable input ordinal")
	if err != nil {
		return expectedObservableInputTuple{}, err
	}
	digest, err := parseManifestDigest(fields[3], "observable input digest")
	if err != nil {
		return expectedObservableInputTuple{}, err
	}
	value := expectedObservableInputTuple{
		evaluationRef:      fields[0],
		inputOrdinal:       ordinal,
		observableInputRef: fields[2],
		observableDigest:   digest,
		canonicalBytes:     append([]byte(nil), raw...),
	}
	return value, nil
}

func decodeExpectedMemberUseCoordinate(raw []byte) (expectedMemberUseCoordinate, error) {
	fields, err := requireCanonicalFields(raw, "typed-memory-expected-member-use-coordinate.v1", 13)
	if err != nil {
		return expectedMemberUseCoordinate{}, err
	}
	filler, err := decodeExpectedFillerCoordinate(fields[:5])
	if err != nil {
		return expectedMemberUseCoordinate{}, err
	}
	queryDigest, err := parseManifestDigest(fields[8], "MemberOf-use query digest")
	if err != nil {
		return expectedMemberUseCoordinate{}, err
	}
	useDigest, err := parseManifestDigest(fields[11], "MemberOf-use digest")
	if err != nil {
		return expectedMemberUseCoordinate{}, err
	}
	value := expectedMemberUseCoordinate{
		filler:                filler,
		useKind:               fields[5],
		constraintID:          fields[6],
		queriedValueKindRef:   fields[7],
		queryDigest:           queryDigest,
		evaluationRef:         fields[9],
		expectedJudgementKind: fields[10],
		useDigest:             useDigest,
		useBytes:              []byte(fields[12]),
		canonicalBytes:        append([]byte(nil), raw...),
	}
	return value, requireCanonicalRoundTrip(raw, canonicalMemberUseCoordinate(value), "MemberOf-use coordinate")
}

func decodeExpectedOrderedCandidatePrefix(raw []byte) (expectedOrderedCandidatePrefix, error) {
	fields, err := requireCanonicalFields(raw, "typed-memory-expected-ordered-candidate-prefix.v1", 3)
	if err != nil {
		return expectedOrderedCandidatePrefix{}, err
	}
	ordinal, err := parseManifestUint(fields[0], "ordered-prefix ordinal")
	if err != nil {
		return expectedOrderedCandidatePrefix{}, err
	}
	digest, err := parseManifestDigest(fields[1], "ordered-prefix digest")
	if err != nil {
		return expectedOrderedCandidatePrefix{}, err
	}
	value := expectedOrderedCandidatePrefix{
		endOrdinal:     ordinal,
		prefixDigest:   digest,
		prefixBytes:    []byte(fields[2]),
		canonicalBytes: append([]byte(nil), raw...),
	}
	return value, requireCanonicalRoundTrip(raw, canonicalOrderedCandidatePrefix(value), "ordered prefix")
}

func decodeExpectedFillerCoordinate(fields []string) (expectedFillerCoordinate, error) {
	if len(fields) != 5 {
		return expectedFillerCoordinate{}, storedAdmissionIntegrity("expected filler coordinate fields", nil)
	}
	changeOrdinal, err := parseManifestUint(fields[0], "filler change ordinal")
	if err != nil {
		return expectedFillerCoordinate{}, err
	}
	slotOrdinal, err := parseManifestUint(fields[2], "filler slot ordinal")
	if err != nil {
		return expectedFillerCoordinate{}, err
	}
	fillerOrdinal, err := parseManifestUint(fields[3], "filler ordinal")
	if err != nil {
		return expectedFillerCoordinate{}, err
	}
	digest, err := parseManifestDigest(fields[4], "filler digest")
	if err != nil {
		return expectedFillerCoordinate{}, err
	}
	return newExpectedFillerCoordinate(
		changeOrdinal,
		fields[1],
		slotOrdinal,
		fillerOrdinal,
		digest,
	), nil
}

func requireCanonicalFields(raw []byte, domain string, count int) ([]string, error) {
	fields, err := decodeCanonicalStorageFields(raw, domain)
	if err != nil {
		return nil, storedAdmissionIntegrity("expected-materialization "+domain, err)
	}
	if len(fields) != count {
		return nil, storedAdmissionIntegrity("expected-materialization "+domain+" field count", nil)
	}
	return fields, nil
}

func decodeCanonicalStorageFields(raw []byte, expectedDomain string) ([]string, error) {
	segments := make([]string, 0)
	remaining := raw
	for len(remaining) != 0 {
		if len(remaining) < 8 {
			return nil, fmt.Errorf("truncated canonical field length")
		}
		length := binary.BigEndian.Uint64(remaining[:8])
		remaining = remaining[8:]
		boundedLength, exact := sliceIndexFromUint64(length)
		if !exact || boundedLength > len(remaining) {
			return nil, fmt.Errorf("truncated canonical field value")
		}
		segments = append(segments, string(remaining[:boundedLength]))
		remaining = remaining[boundedLength:]
	}
	if len(segments) < 2 || segments[0] != canonicalStorageEnvelopeDomain || segments[1] != expectedDomain {
		return nil, fmt.Errorf("unexpected canonical storage domain")
	}
	return segments[2:], nil
}

func parseManifestUint(raw string, detail string) (uint64, error) {
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != raw {
		return 0, storedAdmissionIntegrity("expected-materialization "+detail, err)
	}
	return value, nil
}

func parseManifestDigest(raw string, detail string) (typedmemory.SHA256Digest, error) {
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		return typedmemory.SHA256Digest{}, storedAdmissionIntegrity("expected-materialization "+detail, err)
	}
	return digest, nil
}

func requireCanonicalRoundTrip(raw []byte, rebuilt []byte, detail string) error {
	if !bytes.Equal(raw, rebuilt) {
		return storedAdmissionIntegrity("expected-materialization "+detail+" canonical round trip", nil)
	}
	return nil
}
