package projecttypeenv

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	codecSpecificationCanonicalDomain = "haft.fpf.projecttypeenv.codec-specification.canonical.v1"
	codecSpecificationArtifactDomain  = "codec-specification.v1"
	maximumCodecSpecificationBytes    = 4 << 20
	maximumCodecContractStatements    = 1 << 10
	maximumCodecContractTextBytes     = 16 << 10
)

// CodecSpecificationV1 is the compiler-owned semantic identity of one
// Local-Practice codec declaration. Its digest commits to the exact CodecID,
// canonicalization version, derived ValueShapeRef, and authored contract
// order. ValueKind is deliberately outside this identity: ValueBinding binds
// kind -> shape -> codec separately.
type CodecSpecificationV1 struct {
	ref       typedmemory.CodecRef
	shape     typedmemory.ValueShapeRef
	contract  []string
	canonical []byte
}

func (specification CodecSpecificationV1) Ref() typedmemory.CodecRef {
	return specification.ref
}

func (specification CodecSpecificationV1) ValueShape() typedmemory.ValueShapeRef {
	return specification.shape
}

func (specification CodecSpecificationV1) Contract() []string {
	return append([]string(nil), specification.contract...)
}

func (specification CodecSpecificationV1) CanonicalBytes() []byte {
	return append([]byte(nil), specification.canonical...)
}

func (specification CodecSpecificationV1) Verify() error {
	decoded, err := DecodeCodecSpecificationV1(specification.canonical)
	if err != nil {
		return fmt.Errorf("verify codec specification canonical bytes: %w", err)
	}
	if decoded.ref != specification.ref {
		return fmt.Errorf("codec specification reference is not derived from its bytes")
	}
	if decoded.shape != specification.shape {
		return fmt.Errorf("codec specification shape does not match its canonical bytes")
	}
	if !stringSlicesEqual(decoded.contract, specification.contract) {
		return fmt.Errorf("codec specification contract does not match its canonical bytes")
	}
	if !bytes.Equal(decoded.canonical, specification.canonical) {
		return fmt.Errorf("codec specification bytes are not canonical")
	}
	return nil
}

// DeriveCodecSpecificationV1 derives the only accepted CodecRef for the exact
// semantic declaration. It round-trips through the strict decoder so callers
// never receive an identity that the verifier cannot reconstruct.
func DeriveCodecSpecificationV1(
	id typedmemory.CodecID,
	version typedmemory.CanonicalizationVersion,
	shape typedmemory.ValueShapeRef,
	contract []string,
) (CodecSpecificationV1, error) {
	identity := codecSpecificationIdentity{
		id:       id,
		version:  version,
		shape:    shape,
		contract: append([]string(nil), contract...),
	}
	canonical, err := encodeCodecSpecificationIdentity(identity)
	if err != nil {
		return CodecSpecificationV1{}, err
	}
	specification, err := DecodeCodecSpecificationV1(canonical)
	if err != nil {
		return CodecSpecificationV1{}, fmt.Errorf("reseal codec specification: %w", err)
	}
	return specification, nil
}

// DecodeCodecSpecificationV1 accepts exact canonical bytes only.
func DecodeCodecSpecificationV1(canonical []byte) (CodecSpecificationV1, error) {
	payload, err := decodeCodecSpecificationEnvelope(canonical)
	if err != nil {
		return CodecSpecificationV1{}, err
	}
	if !utf8.Valid(payload) {
		return CodecSpecificationV1{}, fmt.Errorf("codec specification payload contains invalid UTF-8")
	}
	encoded := codecSpecificationCanonicalV1{}
	if err := decodeStrictCodecSpecificationJSON(payload, &encoded); err != nil {
		return CodecSpecificationV1{}, err
	}
	identity, err := codecSpecificationIdentityFromCanonical(encoded)
	if err != nil {
		return CodecSpecificationV1{}, err
	}
	reencoded, err := encodeCodecSpecificationIdentity(identity)
	if err != nil {
		return CodecSpecificationV1{}, err
	}
	if !bytes.Equal(reencoded, canonical) {
		return CodecSpecificationV1{}, fmt.Errorf("codec specification payload is not canonical")
	}
	digest, err := codecSpecificationDigest(reencoded)
	if err != nil {
		return CodecSpecificationV1{}, err
	}
	ref, err := typedmemory.NewCodecRef(identity.id, identity.version, digest)
	if err != nil {
		return CodecSpecificationV1{}, fmt.Errorf("derive codec specification reference: %w", err)
	}
	return CodecSpecificationV1{
		ref:       ref,
		shape:     identity.shape,
		contract:  append([]string(nil), identity.contract...),
		canonical: append([]byte(nil), reencoded...),
	}, nil
}

func VerifyCodecSpecificationV1(
	expected typedmemory.CodecRef,
	canonical []byte,
) (CodecSpecificationV1, error) {
	rebuiltID, err := typedmemory.NewCodecID(expected.ID().String())
	if err != nil || rebuiltID != expected.ID() {
		return CodecSpecificationV1{}, fmt.Errorf("expected codec specification ID is invalid")
	}
	rebuiltVersion, err := typedmemory.NewCanonicalizationVersion(expected.Version().String())
	if err != nil || rebuiltVersion != expected.Version() {
		return CodecSpecificationV1{}, fmt.Errorf("expected codec canonicalization version is invalid")
	}
	rebuiltDigest, err := typedmemory.NewSHA256Digest(expected.SpecificationDigest().String())
	if err != nil || rebuiltDigest != expected.SpecificationDigest() {
		return CodecSpecificationV1{}, fmt.Errorf("expected codec specification digest is invalid")
	}
	rebuilt, err := typedmemory.NewCodecRef(rebuiltID, rebuiltVersion, rebuiltDigest)
	if err != nil || rebuilt != expected {
		return CodecSpecificationV1{}, fmt.Errorf("expected codec specification reference is invalid")
	}
	specification, err := DecodeCodecSpecificationV1(canonical)
	if err != nil {
		return CodecSpecificationV1{}, err
	}
	if specification.ref != expected {
		return CodecSpecificationV1{}, fmt.Errorf(
			"codec specification reference %q does not match canonical bytes %q",
			expected.String(),
			specification.ref.String(),
		)
	}
	return specification, nil
}

type codecSpecificationIdentity struct {
	id       typedmemory.CodecID
	version  typedmemory.CanonicalizationVersion
	shape    typedmemory.ValueShapeRef
	contract []string
}

type codecSpecificationCanonicalV1 struct {
	CodecID                string   `json:"codec_id"`
	Canonicalization       string   `json:"canonicalization_version"`
	ValueShapeID           string   `json:"value_shape_id"`
	ValueShapeDigest       string   `json:"value_shape_digest"`
	OrderedContractClauses []string `json:"ordered_contract_clauses"`
}

func codecSpecificationIdentityFromCanonical(
	encoded codecSpecificationCanonicalV1,
) (codecSpecificationIdentity, error) {
	id, err := typedmemory.NewCodecID(encoded.CodecID)
	if err != nil {
		return codecSpecificationIdentity{}, fmt.Errorf("codec specification ID: %w", err)
	}
	version, err := typedmemory.NewCanonicalizationVersion(encoded.Canonicalization)
	if err != nil {
		return codecSpecificationIdentity{}, fmt.Errorf("codec specification canonicalization version: %w", err)
	}
	shapeID, err := typedmemory.NewShapeID(encoded.ValueShapeID)
	if err != nil {
		return codecSpecificationIdentity{}, fmt.Errorf("codec specification ValueShape ID: %w", err)
	}
	shapeDigest, err := typedmemory.NewSHA256Digest(encoded.ValueShapeDigest)
	if err != nil {
		return codecSpecificationIdentity{}, fmt.Errorf("codec specification ValueShape digest: %w", err)
	}
	shape, err := typedmemory.NewValueShapeRef(shapeID, shapeDigest)
	if err != nil {
		return codecSpecificationIdentity{}, fmt.Errorf("codec specification ValueShape reference: %w", err)
	}
	return codecSpecificationIdentity{
		id:       id,
		version:  version,
		shape:    shape,
		contract: append([]string(nil), encoded.OrderedContractClauses...),
	}, nil
}

func encodeCodecSpecificationIdentity(
	identity codecSpecificationIdentity,
) ([]byte, error) {
	normalized, err := normalizeCodecSpecificationIdentity(identity)
	if err != nil {
		return nil, err
	}
	encoded := codecSpecificationCanonicalV1{
		CodecID:                normalized.id.String(),
		Canonicalization:       normalized.version.String(),
		ValueShapeID:           normalized.shape.ID().String(),
		ValueShapeDigest:       normalized.shape.Digest().String(),
		OrderedContractClauses: append([]string(nil), normalized.contract...),
	}
	payload, err := json.Marshal(encoded)
	if err != nil {
		return nil, fmt.Errorf("encode codec specification payload: %w", err)
	}
	writer := newCodecSpecificationWriter(codecSpecificationArtifactDomain)
	writer.addBytes(payload)
	canonical := writer.bytes()
	if len(canonical) > maximumCodecSpecificationBytes {
		return nil, fmt.Errorf(
			"codec specification exceeds %d bytes",
			maximumCodecSpecificationBytes,
		)
	}
	return canonical, nil
}

func normalizeCodecSpecificationIdentity(
	identity codecSpecificationIdentity,
) (codecSpecificationIdentity, error) {
	id, err := typedmemory.NewCodecID(identity.id.String())
	if err != nil || id != identity.id {
		return codecSpecificationIdentity{}, fmt.Errorf("codec specification ID is invalid")
	}
	version, err := typedmemory.NewCanonicalizationVersion(identity.version.String())
	if err != nil || version != identity.version {
		return codecSpecificationIdentity{}, fmt.Errorf("codec canonicalization version is invalid")
	}
	shapeID, err := typedmemory.NewShapeID(identity.shape.ID().String())
	if err != nil || shapeID != identity.shape.ID() {
		return codecSpecificationIdentity{}, fmt.Errorf("codec ValueShape ID is invalid")
	}
	shapeDigest, err := typedmemory.NewSHA256Digest(identity.shape.Digest().String())
	if err != nil || shapeDigest != identity.shape.Digest() {
		return codecSpecificationIdentity{}, fmt.Errorf("codec ValueShape digest is invalid")
	}
	shape, err := typedmemory.NewValueShapeRef(shapeID, shapeDigest)
	if err != nil || shape != identity.shape {
		return codecSpecificationIdentity{}, fmt.Errorf("codec ValueShape reference is invalid")
	}
	if len(identity.contract) == 0 {
		return codecSpecificationIdentity{}, fmt.Errorf("codec specification requires at least one contract clause")
	}
	if len(identity.contract) > maximumCodecContractStatements {
		return codecSpecificationIdentity{}, fmt.Errorf(
			"codec specification contains %d contract clauses; limit is %d",
			len(identity.contract),
			maximumCodecContractStatements,
		)
	}
	owned := append([]string(nil), identity.contract...)
	seen := make(map[string]struct{}, len(owned))
	for index, clause := range owned {
		if err := validateCodecContractClause(clause); err != nil {
			return codecSpecificationIdentity{}, fmt.Errorf("codec contract clause %d: %w", index, err)
		}
		if _, exists := seen[clause]; exists {
			return codecSpecificationIdentity{}, fmt.Errorf("duplicate codec contract clause %q", clause)
		}
		seen[clause] = struct{}{}
	}
	return codecSpecificationIdentity{
		id:       id,
		version:  version,
		shape:    shape,
		contract: owned,
	}, nil
}

func validateCodecContractClause(value string) error {
	if value == "" {
		return fmt.Errorf("is empty")
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("contains invalid UTF-8")
	}
	if len(value) > maximumCodecContractTextBytes {
		return fmt.Errorf("exceeds %d bytes", maximumCodecContractTextBytes)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return fmt.Errorf("contains a control character")
		}
	}
	return nil
}

func codecSpecificationDigest(canonical []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	return digest, nil
}

func decodeStrictCodecSpecificationJSON(
	payload []byte,
	target *codecSpecificationCanonicalV1,
) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode codec specification payload: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("codec specification JSON has a trailing value")
	}
	return fmt.Errorf("decode codec specification trailing JSON: %w", err)
}

type codecSpecificationWriter struct {
	buffer bytes.Buffer
}

func newCodecSpecificationWriter(domain string) codecSpecificationWriter {
	writer := codecSpecificationWriter{}
	writer.addString(codecSpecificationCanonicalDomain)
	writer.addString(domain)
	return writer
}

func (writer *codecSpecificationWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *codecSpecificationWriter) addBytes(value []byte) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(len(value)))
	writer.buffer.Write(encoded[:])
	writer.buffer.Write(value)
}

func (writer codecSpecificationWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type codecSpecificationReader struct {
	data   []byte
	offset int
}

func decodeCodecSpecificationEnvelope(canonical []byte) ([]byte, error) {
	if len(canonical) == 0 {
		return nil, fmt.Errorf("codec specification canonical bytes are required")
	}
	if len(canonical) > maximumCodecSpecificationBytes {
		return nil, fmt.Errorf(
			"codec specification canonical bytes exceed %d-byte limit",
			maximumCodecSpecificationBytes,
		)
	}
	reader := &codecSpecificationReader{data: canonical}
	root, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode codec specification root domain: %w", err)
	}
	if root != codecSpecificationCanonicalDomain {
		return nil, fmt.Errorf("unexpected codec specification root domain %q", root)
	}
	domain, err := reader.readString()
	if err != nil {
		return nil, fmt.Errorf("decode codec specification artifact domain: %w", err)
	}
	if domain != codecSpecificationArtifactDomain {
		return nil, fmt.Errorf("unexpected codec specification artifact domain %q", domain)
	}
	payload, err := reader.readBytes()
	if err != nil {
		return nil, fmt.Errorf("decode codec specification payload: %w", err)
	}
	if reader.offset != len(reader.data) {
		return nil, fmt.Errorf(
			"codec specification payload has %d trailing bytes",
			len(reader.data)-reader.offset,
		)
	}
	return append([]byte(nil), payload...), nil
}

func (reader *codecSpecificationReader) readString() (string, error) {
	value, err := reader.readBytes()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", fmt.Errorf("canonical domain contains invalid UTF-8")
	}
	return string(value), nil
}

func (reader *codecSpecificationReader) readBytes() ([]byte, error) {
	if reader == nil || len(reader.data)-reader.offset < 8 {
		return nil, fmt.Errorf("unexpected end of length-prefixed field")
	}
	lengthEnd := reader.offset + 8
	length := binary.BigEndian.Uint64(reader.data[reader.offset:lengthEnd])
	reader.offset = lengthEnd
	remaining := len(reader.data) - reader.offset
	//nolint:gosec // remaining is non-negative after the reader bounds check above.
	if length > uint64(remaining) {
		return nil, fmt.Errorf("length-prefixed field %d exceeds remaining payload %d", length, remaining)
	}
	if length > maximumCodecSpecificationBytes {
		return nil, fmt.Errorf("length-prefixed field exceeds %d bytes", maximumCodecSpecificationBytes)
	}
	boundedLength := int(length)
	end := reader.offset + boundedLength
	value := reader.data[reader.offset:end]
	reader.offset = end
	return value, nil
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
