// Package projecttypeenvtransitioncompatibility owns the low-level immutable
// envelope that binds one complete successor diff to one exact installed
// ProjectionProfile compatibility-set carrier. It deliberately treats the
// profile carrier as opaque canonical bytes so the TypeEnv selection core does
// not depend on read-side projection packages.
package projecttypeenvtransitioncompatibility

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	canonicalDomain       = "haft.transition-projection-profile-compatibility-set.v1"
	refPrefix             = "transition-projection-profile-compatibility-set:"
	maximumCanonicalBytes = 64 << 20
	maximumCoordinate     = 16 << 10
)

type Set struct {
	ref            Ref
	diff           projecttypeenvcompatibility.SuccessorDiff
	profiles       []byte
	profilesDigest typedmemory.SHA256Digest
	digest         typedmemory.SHA256Digest
}

type Ref struct {
	digest typedmemory.SHA256Digest
}

func ParseRef(raw string) (Ref, error) {
	if raw != strings.TrimSpace(raw) || !strings.HasPrefix(raw, refPrefix) {
		return Ref{}, fmt.Errorf(
			"transition projection-profile compatibility ref must start with %q",
			refPrefix,
		)
	}
	digest, err := typedmemory.NewSHA256Digest(strings.TrimPrefix(raw, refPrefix))
	if err != nil {
		return Ref{}, err
	}
	return Ref{digest: digest}, nil
}

func (ref Ref) Digest() typedmemory.SHA256Digest { return ref.digest }

func (ref Ref) String() string { return refPrefix + ref.digest.String() }

func New(
	diff projecttypeenvcompatibility.SuccessorDiff,
	profilesCanonical []byte,
) (Set, error) {
	profilesDigest, err := digestBytes(profilesCanonical)
	if err != nil {
		return Set{}, fmt.Errorf("installed projection-profile carrier: %w", err)
	}
	value := Set{
		diff:           diff,
		profiles:       append([]byte(nil), profilesCanonical...),
		profilesDigest: profilesDigest,
	}
	digest, err := digestBytes(value.CanonicalBytes())
	if err != nil {
		return Set{}, err
	}
	value.digest = digest
	value.ref = Ref{digest: digest}
	if err := value.Verify(); err != nil {
		return Set{}, err
	}
	return value, nil
}

func (value Set) Ref() Ref { return value.ref }

func (value Set) Digest() typedmemory.SHA256Digest { return value.digest }

func (value Set) SuccessorDiff() projecttypeenvcompatibility.SuccessorDiff {
	decoded, _ := projecttypeenvcompatibility.DecodeSuccessorDiff(
		value.diff.CanonicalBytes(),
	)
	return decoded
}

func (value Set) ProjectionProfilesCanonicalBytes() []byte {
	return append([]byte(nil), value.profiles...)
}

func (value Set) ProjectionProfilesDigest() typedmemory.SHA256Digest {
	return value.profilesDigest
}

func (value Set) CanonicalBytes() []byte {
	writer := canonicalWriter{}
	writer.addString(canonicalDomain)
	writer.addBytes(value.diff.CanonicalBytes())
	writer.addBytes(value.profiles)
	return writer.bytes()
}

func (value Set) Verify() error {
	if err := value.diff.Verify(); err != nil {
		return fmt.Errorf("transition successor diff: %w", err)
	}
	if len(value.profiles) == 0 || len(value.profiles) > maximumCanonicalBytes {
		return fmt.Errorf("installed projection-profile carrier byte length is invalid")
	}
	profilesDigest, err := digestBytes(value.profiles)
	if err != nil {
		return err
	}
	if profilesDigest != value.profilesDigest {
		return fmt.Errorf("installed projection-profile carrier digest mismatch")
	}
	canonical := value.CanonicalBytes()
	if len(canonical) == 0 || len(canonical) > maximumCanonicalBytes {
		return fmt.Errorf("transition projection-profile compatibility byte length is invalid")
	}
	digest, err := digestBytes(canonical)
	if err != nil {
		return err
	}
	if digest != value.digest || value.ref.digest != digest {
		return fmt.Errorf("transition projection-profile compatibility identity mismatch")
	}
	return nil
}

func Decode(canonical []byte) (Set, error) {
	if len(canonical) == 0 || len(canonical) > maximumCanonicalBytes {
		return Set{}, fmt.Errorf("transition projection-profile compatibility byte length is invalid")
	}
	reader := canonicalReader{value: canonical}
	domain, err := reader.readString("domain")
	if err != nil {
		return Set{}, err
	}
	if domain != canonicalDomain {
		return Set{}, fmt.Errorf("transition projection-profile compatibility domain is invalid")
	}
	diffBytes, err := reader.readBytes("successor diff", maximumCanonicalBytes)
	if err != nil {
		return Set{}, err
	}
	diff, err := projecttypeenvcompatibility.DecodeSuccessorDiff(diffBytes)
	if err != nil {
		return Set{}, err
	}
	profiles, err := reader.readBytes("installed projection-profile carrier", maximumCanonicalBytes)
	if err != nil {
		return Set{}, err
	}
	if reader.remaining() != 0 {
		return Set{}, fmt.Errorf("transition projection-profile compatibility has trailing bytes")
	}
	value, err := New(diff, profiles)
	if err != nil {
		return Set{}, err
	}
	if !bytes.Equal(value.CanonicalBytes(), canonical) {
		return Set{}, fmt.Errorf("transition projection-profile compatibility is not canonical")
	}
	return value, nil
}

func digestBytes(value []byte) (typedmemory.SHA256Digest, error) {
	if len(value) == 0 {
		return typedmemory.SHA256Digest{}, fmt.Errorf("canonical bytes are required")
	}
	sum := sha256.Sum256(value)
	return typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
}

type canonicalWriter struct{ buffer bytes.Buffer }

func (writer *canonicalWriter) addString(value string) { writer.addBytes([]byte(value)) }

func (writer *canonicalWriter) addBytes(value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	writer.buffer.Write(length[:])
	writer.buffer.Write(value)
}

func (writer canonicalWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type canonicalReader struct {
	value  []byte
	offset int
}

func (reader *canonicalReader) readString(label string) (string, error) {
	value, err := reader.readBytes(label, maximumCoordinate)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", fmt.Errorf("transition projection-profile compatibility %s is not UTF-8", label)
	}
	return string(value), nil
}

func (reader *canonicalReader) readBytes(label string, maximum int) ([]byte, error) {
	if len(reader.value)-reader.offset < 8 {
		return nil, fmt.Errorf("transition projection-profile compatibility %s is truncated", label)
	}
	length := binary.BigEndian.Uint64(reader.value[reader.offset : reader.offset+8])
	reader.offset += 8
	boundedLength, exact := canonicalSliceLength(length)
	if !exact || boundedLength > maximum {
		return nil, fmt.Errorf("transition projection-profile compatibility %s exceeds limit", label)
	}
	if boundedLength > reader.remaining() {
		return nil, fmt.Errorf("transition projection-profile compatibility %s is truncated", label)
	}
	end := reader.offset + boundedLength
	value := append([]byte(nil), reader.value[reader.offset:end]...)
	reader.offset = end
	return value, nil
}

func canonicalSliceLength(value uint64) (int, bool) {
	if value > math.MaxInt {
		return 0, false
	}
	return int(value), true
}

func (reader canonicalReader) remaining() int { return len(reader.value) - reader.offset }
