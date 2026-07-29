package runtimemechanism

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	runtimeMechanismArtifactDomain = "haft.runtime-mechanism-artifact.canonical.v1"
	runtimeMechanismArtifactSchema = "haft.runtime-mechanism-artifact/v1"
)

// RuntimeMechanismArtifactIdentityV1 identifies one exact catalog carrier,
// edition, and its content-derived canonical digest.
type RuntimeMechanismArtifactIdentityV1 struct {
	artifact typedmemory.CarrierRef
	edition  typedmemory.CarrierEdition
	digest   typedmemory.SHA256Digest
}

func (identity RuntimeMechanismArtifactIdentityV1) Artifact() typedmemory.CarrierRef {
	return identity.artifact
}

func (identity RuntimeMechanismArtifactIdentityV1) Edition() typedmemory.CarrierEdition {
	return identity.edition
}

func (identity RuntimeMechanismArtifactIdentityV1) Digest() typedmemory.SHA256Digest {
	return identity.digest
}

func (identity RuntimeMechanismArtifactIdentityV1) valid() bool {
	artifact, artifactErr := validateCarrierRef(identity.artifact)
	edition, editionErr := validateEdition(identity.edition)
	digest, digestErr := validateDigest(identity.digest)
	return artifactErr == nil &&
		editionErr == nil &&
		digestErr == nil &&
		artifact == identity.artifact &&
		edition == identity.edition &&
		digest == identity.digest
}

// RuntimeMechanismArtifactV1 is an immutable content-addressed catalog of
// declared runtime entrypoints. It authenticates the declaration bytes only;
// it neither identifies nor attests executable code or observed execution.
type RuntimeMechanismArtifactV1 struct {
	identity  RuntimeMechanismArtifactIdentityV1
	entries   []RuntimeMechanismEntryV1
	canonical []byte
}

func (artifact RuntimeMechanismArtifactV1) Identity() RuntimeMechanismArtifactIdentityV1 {
	return artifact.identity
}

func (artifact RuntimeMechanismArtifactV1) Entries() []RuntimeMechanismEntryV1 {
	return cloneRuntimeMechanismEntries(artifact.entries)
}

func (artifact RuntimeMechanismArtifactV1) CanonicalBytes() []byte {
	return append([]byte(nil), artifact.canonical...)
}

func (artifact RuntimeMechanismArtifactV1) Verify() error {
	if len(artifact.canonical) == 0 {
		return fmt.Errorf("runtime mechanism artifact is empty")
	}
	decoded, err := DecodeRuntimeMechanismArtifactV1(artifact.canonical)
	if err != nil {
		return fmt.Errorf("verify runtime mechanism artifact bytes: %w", err)
	}
	if decoded.identity != artifact.identity {
		return fmt.Errorf("runtime mechanism artifact identity is not derived from its bytes")
	}
	if !runtimeMechanismEntriesEqual(decoded.entries, artifact.entries) {
		return fmt.Errorf("runtime mechanism artifact entries do not match its bytes")
	}
	if !bytes.Equal(decoded.canonical, artifact.canonical) {
		return fmt.Errorf("runtime mechanism artifact canonical bytes do not match")
	}
	return nil
}

// SealRuntimeMechanismArtifactV1 canonicalizes an unordered entry set and
// derives the only identity returned for those bytes.
func SealRuntimeMechanismArtifactV1(
	artifactRef typedmemory.CarrierRef,
	edition typedmemory.CarrierEdition,
	entries []RuntimeMechanismEntryV1,
) (RuntimeMechanismArtifactV1, error) {
	artifact, err := validateCarrierRef(artifactRef)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	exactEdition, err := validateEdition(edition)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	normalized, err := normalizeRuntimeMechanismEntries(entries)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	canonical, err := encodeRuntimeMechanismArtifactV1(
		artifact,
		exactEdition,
		normalized,
	)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	sealed, err := DecodeRuntimeMechanismArtifactV1(canonical)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf(
			"reseal runtime mechanism artifact: %w",
			err,
		)
	}
	return sealed, nil
}

// DecodeRuntimeMechanismArtifactV1 accepts exact canonical bytes only.
func DecodeRuntimeMechanismArtifactV1(
	canonical []byte,
) (RuntimeMechanismArtifactV1, error) {
	if len(canonical) == 0 {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf("runtime mechanism artifact is empty")
	}
	if len(canonical) > MaximumArtifactBytes {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf(
			"runtime mechanism artifact exceeds %d bytes",
			MaximumArtifactBytes,
		)
	}
	reader := newCanonicalReader(canonical)
	domain, err := reader.readString("domain", len(runtimeMechanismArtifactDomain))
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	if domain != runtimeMechanismArtifactDomain {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf("runtime mechanism artifact domain is invalid")
	}
	schema, err := reader.readString("schema", len(runtimeMechanismArtifactSchema))
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	if schema != runtimeMechanismArtifactSchema {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf("runtime mechanism artifact schema is invalid")
	}
	artifactText, err := reader.readString("artifact reference", MaximumCoordinateBytes)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	artifactRef, err := typedmemory.NewCarrierRef(artifactText)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf("decode runtime mechanism artifact reference: %w", err)
	}
	artifact, err := validateCarrierRef(artifactRef)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	editionText, err := reader.readString("edition", MaximumCoordinateBytes)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	editionRef, err := typedmemory.NewCarrierEdition(editionText)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf("decode runtime mechanism edition: %w", err)
	}
	edition, err := validateEdition(editionRef)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	entryCount, err := reader.readCount("entry count")
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	if entryCount == 0 {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf("runtime mechanism artifact entries are required")
	}
	if entryCount > MaximumArtifactEntries {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf(
			"runtime mechanism artifact exceeds %d entries",
			MaximumArtifactEntries,
		)
	}
	entries := make([]RuntimeMechanismEntryV1, 0, entryCount)
	for index := uint64(0); index < entryCount; index++ {
		entryBytes, readErr := reader.readFrame("entry", maximumEncodedEntryBytes)
		if readErr != nil {
			return RuntimeMechanismArtifactV1{}, readErr
		}
		entry, decodeErr := decodeRuntimeMechanismEntryV1(entryBytes)
		if decodeErr != nil {
			return RuntimeMechanismArtifactV1{}, fmt.Errorf(
				"decode runtime mechanism entry %d: %w",
				index,
				decodeErr,
			)
		}
		entries = append(entries, entry)
	}
	if reader.remaining() != 0 {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf("runtime mechanism artifact has trailing bytes")
	}
	normalized, err := normalizeRuntimeMechanismEntries(entries)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	reencoded, err := encodeRuntimeMechanismArtifactV1(
		artifact,
		edition,
		normalized,
	)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf("runtime mechanism artifact is not canonical")
	}
	digest, err := deriveRuntimeMechanismDigest(canonical)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	identity := RuntimeMechanismArtifactIdentityV1{
		artifact: artifact,
		edition:  edition,
		digest:   digest,
	}
	return RuntimeMechanismArtifactV1{
		identity:  identity,
		entries:   cloneRuntimeMechanismEntries(normalized),
		canonical: append([]byte(nil), canonical...),
	}, nil
}

// VerifyRuntimeMechanismArtifactV1 checks exact bytes against an already
// derived expected identity.
func VerifyRuntimeMechanismArtifactV1(
	expected RuntimeMechanismArtifactIdentityV1,
	canonical []byte,
) (RuntimeMechanismArtifactV1, error) {
	if !expected.valid() {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf("expected runtime mechanism artifact identity is invalid")
	}
	decoded, err := DecodeRuntimeMechanismArtifactV1(canonical)
	if err != nil {
		return RuntimeMechanismArtifactV1{}, err
	}
	if decoded.identity != expected {
		return RuntimeMechanismArtifactV1{}, fmt.Errorf("runtime mechanism artifact identity mismatch")
	}
	return decoded, nil
}

func normalizeRuntimeMechanismEntries(
	entries []RuntimeMechanismEntryV1,
) ([]RuntimeMechanismEntryV1, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("runtime mechanism artifact entries are required")
	}
	if len(entries) > MaximumArtifactEntries {
		return nil, fmt.Errorf(
			"runtime mechanism artifact exceeds %d entries",
			MaximumArtifactEntries,
		)
	}
	normalized := make([]RuntimeMechanismEntryV1, 0, len(entries))
	for index, entry := range entries {
		rebuilt, err := validateRuntimeMechanismEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("runtime mechanism entry %d: %w", index, err)
		}
		normalized = append(normalized, rebuilt)
	}
	sort.Slice(normalized, func(leftIndex int, rightIndex int) bool {
		left := runtimeMechanismEntryCanonicalKey(normalized[leftIndex])
		right := runtimeMechanismEntryCanonicalKey(normalized[rightIndex])
		return left < right
	})
	seenTuples := make(map[string]InvocationContract, len(normalized))
	for _, entry := range normalized {
		tupleKey := runtimeMechanismEntryCanonicalKey(entry)
		if existing, found := seenTuples[tupleKey]; found {
			return nil, newEntryConflict(
				EntryConflictDuplicate,
				entry,
				existing,
			)
		}
		seenTuples[tupleKey] = entry.contract
	}
	return normalized, nil
}

func newEntryConflict(
	kind EntryConflictKind,
	entry RuntimeMechanismEntryV1,
	existing InvocationContract,
) *EntryConflictError {
	return &EntryConflictError{
		kind:             kind,
		role:             entry.role,
		semantic:         semanticCoordinateText(entry.semantic),
		existingContract: existing,
		incomingContract: entry.contract,
	}
}

func runtimeMechanismEntryCanonicalKey(entry RuntimeMechanismEntryV1) string {
	values := []string{
		entry.role.String(),
		entry.contract.String(),
		entry.semantic.Kind().String(),
		semanticCoordinateText(entry.semantic),
	}
	return joinCanonicalKey(values)
}

func joinCanonicalKey(values []string) string {
	var result string
	for _, value := range values {
		length := fmt.Sprintf("%d:", len(value))
		result += length + value
	}
	return result
}

func semanticCoordinateText(coordinate SemanticCoordinate) string {
	switch value := coordinate.(type) {
	case CodecSemanticCoordinate:
		return value.ref.String()
	case RuleSemanticCoordinate:
		return value.ref.String()
	default:
		return ""
	}
}

func cloneRuntimeMechanismEntries(
	entries []RuntimeMechanismEntryV1,
) []RuntimeMechanismEntryV1 {
	return append([]RuntimeMechanismEntryV1(nil), entries...)
}

func runtimeMechanismEntriesEqual(
	left []RuntimeMechanismEntryV1,
	right []RuntimeMechanismEntryV1,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		leftKey := runtimeMechanismEntryCanonicalKey(left[index])
		rightKey := runtimeMechanismEntryCanonicalKey(right[index])
		if leftKey != rightKey {
			return false
		}
	}
	return true
}

func encodeRuntimeMechanismArtifactV1(
	artifact typedmemory.CarrierRef,
	edition typedmemory.CarrierEdition,
	entries []RuntimeMechanismEntryV1,
) ([]byte, error) {
	writer := newCanonicalWriter(MaximumArtifactBytes)
	if err := writer.writeString(runtimeMechanismArtifactDomain); err != nil {
		return nil, err
	}
	if err := writer.writeString(runtimeMechanismArtifactSchema); err != nil {
		return nil, err
	}
	if err := writer.writeString(artifact.String()); err != nil {
		return nil, err
	}
	if err := writer.writeString(edition.String()); err != nil {
		return nil, err
	}
	if err := writer.writeCount(uint64(len(entries))); err != nil {
		return nil, err
	}
	for index, entry := range entries {
		encoded, err := encodeRuntimeMechanismEntryV1(entry)
		if err != nil {
			return nil, fmt.Errorf("encode runtime mechanism entry %d: %w", index, err)
		}
		if err := writer.writeFrame(encoded); err != nil {
			return nil, err
		}
	}
	return writer.bytes(), nil
}

func encodeRuntimeMechanismEntryV1(
	entry RuntimeMechanismEntryV1,
) ([]byte, error) {
	writer := newCanonicalWriter(maximumEncodedEntryBytes)
	if err := writer.writeString(entry.role.String()); err != nil {
		return nil, err
	}
	if err := writer.writeString(entry.contract.String()); err != nil {
		return nil, err
	}
	if err := writer.writeString(entry.semantic.Kind().String()); err != nil {
		return nil, err
	}
	switch coordinate := entry.semantic.(type) {
	case CodecSemanticCoordinate:
		if err := writer.writeString(coordinate.ref.ID().String()); err != nil {
			return nil, err
		}
		if err := writer.writeString(coordinate.ref.Version().String()); err != nil {
			return nil, err
		}
		if err := writer.writeString(coordinate.ref.SpecificationDigest().String()); err != nil {
			return nil, err
		}
	case RuleSemanticCoordinate:
		if err := writer.writeString(coordinate.ref.String()); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("runtime mechanism semantic coordinate is required")
	}
	return writer.bytes(), nil
}

func decodeRuntimeMechanismEntryV1(
	canonical []byte,
) (RuntimeMechanismEntryV1, error) {
	reader := newCanonicalReader(canonical)
	roleText, err := reader.readString("role", MaximumTextBytes)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	role, err := parseRuntimeMechanismRole(roleText)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	contractText, err := reader.readString("invocation contract", MaximumTextBytes)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	contract, err := parseInvocationContract(contractText)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	kindText, err := reader.readString("semantic coordinate kind", MaximumTextBytes)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	semantic, err := decodeSemanticCoordinate(kindText, reader)
	if err != nil {
		return RuntimeMechanismEntryV1{}, err
	}
	if reader.remaining() != 0 {
		return RuntimeMechanismEntryV1{}, fmt.Errorf("runtime mechanism entry has trailing bytes")
	}
	entry := RuntimeMechanismEntryV1{
		role:     role,
		contract: contract,
		semantic: semantic,
	}
	return validateRuntimeMechanismEntry(entry)
}

func decodeSemanticCoordinate(
	kind string,
	reader *canonicalReader,
) (SemanticCoordinate, error) {
	switch kind {
	case "codec_ref":
		return decodeCodecSemanticCoordinate(reader)
	case "rule_ref":
		return decodeRuleSemanticCoordinate(reader)
	default:
		return nil, fmt.Errorf("semantic coordinate kind %q is not defined", kind)
	}
}

func decodeCodecSemanticCoordinate(
	reader *canonicalReader,
) (SemanticCoordinate, error) {
	idText, err := reader.readString("codec ID", MaximumTextBytes)
	if err != nil {
		return nil, err
	}
	id, err := typedmemory.NewCodecID(idText)
	if err != nil {
		return nil, fmt.Errorf("decode runtime codec ID: %w", err)
	}
	versionText, err := reader.readString("canonicalization version", MaximumTextBytes)
	if err != nil {
		return nil, err
	}
	version, err := typedmemory.NewCanonicalizationVersion(versionText)
	if err != nil {
		return nil, fmt.Errorf("decode runtime codec version: %w", err)
	}
	digestText, err := reader.readString("codec specification digest", MaximumTextBytes)
	if err != nil {
		return nil, err
	}
	digest, err := typedmemory.NewSHA256Digest(digestText)
	if err != nil {
		return nil, fmt.Errorf("decode runtime codec digest: %w", err)
	}
	ref, err := typedmemory.NewCodecRef(id, version, digest)
	if err != nil {
		return nil, fmt.Errorf("decode runtime codec reference: %w", err)
	}
	return NewCodecSemanticCoordinate(ref)
}

func decodeRuleSemanticCoordinate(
	reader *canonicalReader,
) (SemanticCoordinate, error) {
	ruleText, err := reader.readString("RuleRef", MaximumRuleRefBytes)
	if err != nil {
		return nil, err
	}
	ref, err := typedmemory.NewRuleRef(ruleText)
	if err != nil {
		return nil, fmt.Errorf("decode runtime RuleRef: %w", err)
	}
	return NewRuleSemanticCoordinate(ref)
}

func deriveRuntimeMechanismDigest(
	canonical []byte,
) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	hexValue := hex.EncodeToString(sum[:])
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hexValue)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	return digest, nil
}

type canonicalWriter struct {
	buffer bytes.Buffer
	limit  int
}

func newCanonicalWriter(limit int) *canonicalWriter {
	return &canonicalWriter{limit: limit}
}

func (writer *canonicalWriter) writeString(value string) error {
	return writer.writeFrame([]byte(value))
}

func (writer *canonicalWriter) writeCount(value uint64) error {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, value)
	return writer.writeFrame(encoded)
}

func (writer *canonicalWriter) writeFrame(value []byte) error {
	required := 8 + len(value)
	if required > writer.limit-writer.buffer.Len() {
		return fmt.Errorf("canonical runtime mechanism encoding exceeds %d bytes", writer.limit)
	}
	length := make([]byte, 8)
	binary.BigEndian.PutUint64(length, uint64(len(value)))
	writer.buffer.Write(length)
	writer.buffer.Write(value)
	return nil
}

func (writer *canonicalWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type canonicalReader struct {
	canonical []byte
	offset    int
}

func newCanonicalReader(canonical []byte) *canonicalReader {
	return &canonicalReader{canonical: canonical}
}

func (reader *canonicalReader) readString(
	label string,
	maximum int,
) (string, error) {
	encoded, err := reader.readFrame(label, maximum)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(encoded) {
		return "", fmt.Errorf("runtime mechanism %s contains invalid UTF-8", label)
	}
	value := string(encoded)
	if err := validateText(value); err != nil {
		return "", fmt.Errorf("runtime mechanism %s: %w", label, err)
	}
	return value, nil
}

func (reader *canonicalReader) readCount(label string) (uint64, error) {
	encoded, err := reader.readFrame(label, 8)
	if err != nil {
		return 0, err
	}
	if len(encoded) != 8 {
		return 0, fmt.Errorf("runtime mechanism %s must contain exactly 8 bytes", label)
	}
	return binary.BigEndian.Uint64(encoded), nil
}

func (reader *canonicalReader) readFrame(
	label string,
	maximum int,
) ([]byte, error) {
	if reader.remaining() < 8 {
		return nil, fmt.Errorf("runtime mechanism %s length is truncated", label)
	}
	lengthBytes := reader.canonical[reader.offset : reader.offset+8]
	reader.offset += 8
	length := binary.BigEndian.Uint64(lengthBytes)
	maximumValue, err := strconv.ParseUint(strconv.Itoa(maximum), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("runtime mechanism %s maximum is invalid: %w", label, err)
	}
	if length > maximumValue {
		return nil, fmt.Errorf("runtime mechanism %s exceeds %d bytes", label, maximum)
	}
	remaining := reader.remaining()
	remainingValue, err := strconv.ParseUint(strconv.Itoa(remaining), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("runtime mechanism %s remaining byte count is invalid: %w", label, err)
	}
	if length > remainingValue {
		return nil, fmt.Errorf("runtime mechanism %s is truncated", label)
	}
	lengthValue, err := strconv.Atoi(strconv.FormatUint(length, 10))
	if err != nil {
		return nil, fmt.Errorf("runtime mechanism %s length does not fit this runtime: %w", label, err)
	}
	end := reader.offset + lengthValue
	value := reader.canonical[reader.offset:end]
	reader.offset = end
	return value, nil
}

func (reader *canonicalReader) remaining() int {
	return len(reader.canonical) - reader.offset
}
