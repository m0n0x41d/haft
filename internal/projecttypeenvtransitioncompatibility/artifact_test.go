package projecttypeenvtransitioncompatibility

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCanonicalReaderPreservesLimitAndTruncationErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		length  uint64
		payload []byte
		want    string
	}{
		{
			name:   "hostile max uint64 exceeds limit",
			length: math.MaxUint64,
			want: "transition projection-profile compatibility successor diff " +
				"exceeds limit",
		},
		{
			name:    "bounded field is truncated",
			length:  2,
			payload: []byte{0x01},
			want: "transition projection-profile compatibility successor diff " +
				"is truncated",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			raw := make([]byte, 8, 8+len(test.payload))
			binary.BigEndian.PutUint64(raw, test.length)
			raw = append(raw, test.payload...)
			reader := canonicalReader{value: raw}

			_, err := reader.readBytes(
				"successor diff",
				maximumCanonicalBytes,
			)
			if err == nil || err.Error() != test.want {
				t.Fatalf("readBytes() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestSetBindsExactSuccessorDiffAndOwnsProfileCarrier(t *testing.T) {
	diff := transitionCompatibilitySuccessorDiff(t)
	profiles := []byte("projection-profile-compatibility-set:v1")
	value := mustTransitionCompatibility(New(diff, profiles))

	if value.Ref().Digest() != value.Digest() {
		t.Fatal("set ref and digest diverged")
	}
	if value.SuccessorDiff().Digest() != diff.Digest() ||
		!bytes.Equal(value.SuccessorDiff().CanonicalBytes(), diff.CanonicalBytes()) {
		t.Fatal("set lost the exact complete successor diff")
	}
	if !bytes.Equal(value.ProjectionProfilesCanonicalBytes(), profiles) {
		t.Fatal("set lost the exact projection-profile carrier")
	}

	expectedCanonical := value.CanonicalBytes()
	expectedRef := value.Ref()
	profiles[0] ^= 0xff
	returnedProfiles := value.ProjectionProfilesCanonicalBytes()
	returnedProfiles[0] ^= 0xff
	returnedCanonical := value.CanonicalBytes()
	returnedCanonical[0] ^= 0xff
	if value.Ref() != expectedRef ||
		!bytes.Equal(value.CanonicalBytes(), expectedCanonical) {
		t.Fatal("caller mutation changed the retained transition artifact")
	}

	decoded := mustTransitionCompatibility(Decode(expectedCanonical))
	if decoded.Ref() != expectedRef ||
		!bytes.Equal(decoded.CanonicalBytes(), expectedCanonical) {
		t.Fatal("canonical round-trip changed transition artifact identity")
	}
}

func TestSetRejectsMalformedEnvelopeAndChangedContentCannotAlias(t *testing.T) {
	diff := transitionCompatibilitySuccessorDiff(t)
	profiles := []byte("projection-profile-compatibility-set:v1")
	value := mustTransitionCompatibility(New(diff, profiles))

	if _, err := New(diff, nil); err == nil {
		t.Fatal("set accepted an empty projection-profile carrier")
	}
	if _, err := ParseRef(" " + value.Ref().String()); err == nil {
		t.Fatal("ref parser accepted surrounding whitespace")
	}
	if _, err := ParseRef("project-typeenv-stage:" + value.Digest().String()); err == nil {
		t.Fatal("ref parser accepted another artifact namespace")
	}
	trailing := append(value.CanonicalBytes(), 0x01)
	if _, err := Decode(trailing); err == nil {
		t.Fatal("decoder accepted trailing bytes")
	}
	tamperedDiff := bytes.Replace(
		value.CanonicalBytes(),
		[]byte("haft.project-typeenv-successor-diff.v1"),
		[]byte("haft.project-typeenv-successor-diff.x1"),
		1,
	)
	if bytes.Equal(tamperedDiff, value.CanonicalBytes()) {
		t.Fatal("successor-diff tamper fixture did not change canonical bytes")
	}
	if _, err := Decode(tamperedDiff); err == nil {
		t.Fatal("decoder accepted a tampered embedded successor diff")
	}

	changedProfiles := append([]byte(nil), profiles...)
	changedProfiles[len(changedProfiles)-1] = '2'
	changed := mustTransitionCompatibility(New(diff, changedProfiles))
	if changed.Ref() == value.Ref() || changed.Digest() == value.Digest() {
		t.Fatal("changed profile content aliased the original transition artifact")
	}
}

func transitionCompatibilitySuccessorDiff(
	t *testing.T,
) projecttypeenvcompatibility.SuccessorDiff {
	t.Helper()
	before := transitionCompatibilityTypeEnv(t, "before")
	after := transitionCompatibilityTypeEnv(t, "after")
	return mustTransitionCompatibility(
		projecttypeenvcompatibility.CompareSuccessor(before, after),
	)
}

func transitionCompatibilityTypeEnv(
	t *testing.T,
	seed string,
) typedmemory.TypeEnv {
	t.Helper()
	ref := mustTransitionCompatibility(
		typedmemory.NewTypeEnvRef(transitionCompatibilityDigest(t, "typeenv-"+seed)),
	)
	revision := mustTransitionCompatibility(
		typedmemory.NewSourceRevision("transition-compatibility-source-v1"),
	)
	compiler := mustTransitionCompatibility(
		typedmemory.NewCompilerSchemaVersion("transition-compatibility-compiler-v1"),
	)
	unit := mustTransitionCompatibility(
		typedmemory.NewSourceUnitID("transition.compatibility.fixture"),
	)
	lineRange := mustTransitionCompatibility(
		typedmemory.NewSourceLineRange(1, 1),
	)
	location := mustTransitionCompatibility(
		typedmemory.NewUnpatternedSourceLocation(
			unit,
			revision,
			transitionCompatibilityDigest(t, "source"),
			lineRange,
		),
	)
	subject := mustTransitionCompatibility(typedmemory.SourceUnitCoverage(unit))
	entry := mustTransitionCompatibility(
		typedmemory.NewCompiledCoverageEntry(subject, location),
	)
	coverage := mustTransitionCompatibility(
		typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{entry}),
	)
	provenanceRef := mustTransitionCompatibility(
		typedmemory.NewProvenanceRef("prov:transition.compatibility.fixture"),
	)
	rule := mustTransitionCompatibility(
		typedmemory.NewCompilerRuleID("transition.compatibility.fixture"),
	)
	provenance := mustTransitionCompatibility(
		typedmemory.NewFPFSourceProvenance(provenanceRef, location, rule),
	)
	contextRef := mustTransitionCompatibility(
		typedmemory.NewBoundedContextRef("context:transition.compatibility.fixture"),
	)
	contextValue := mustTransitionCompatibility(
		typedmemory.NewBoundedContext(contextRef, provenance),
	)
	builder := typedmemory.NewTypeEnvBuilder(ref).
		SetSourceRevision(revision).
		SetCompilerSchemaVersion(compiler).
		SetCoverageManifest(coverage).
		AddBoundedContext(contextValue)
	return mustTransitionCompatibility(builder.Build())
}

func transitionCompatibilityDigest(
	t *testing.T,
	seed string,
) typedmemory.SHA256Digest {
	t.Helper()
	sum := sha256.Sum256([]byte(seed))
	encoded := hex.EncodeToString(sum[:])
	return mustTransitionCompatibility(
		typedmemory.NewSHA256Digest("sha256:" + encoded),
	)
}

func mustTransitionCompatibility[T any](value T, err error) T {
	if err != nil {
		panic("transition compatibility fixture: " + err.Error())
	}
	return value
}
