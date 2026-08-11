package artifacts

import (
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestHistoricalArtifactsAreExactAndPrivatelyDecoded(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		ref            string
		compilerSchema string
		sourceRevision string
		canonicalSize  int
	}{
		{
			name:           "v3",
			ref:            HistoricalV3Ref,
			compilerSchema: typeenv.BaseTypeEnvCompilerSchemaV3,
			sourceRevision: historicalV3SourceRevision,
			canonicalSize:  112193,
		},
		{
			name:           "v4",
			ref:            HistoricalV4Ref,
			compilerSchema: typeenv.BaseTypeEnvCompilerSchemaV4,
			sourceRevision: historicalV4SourceRevision,
			canonicalSize:  139574,
		},
		{
			name:           "v5",
			ref:            HistoricalV5Ref,
			compilerSchema: typeenv.BaseTypeEnvCompilerSchemaV5,
			sourceRevision: historicalV5SourceRevision,
			canonicalSize:  133848,
		},
		{
			name:           "v6",
			ref:            HistoricalV6Ref,
			compilerSchema: typeenv.BaseTypeEnvCompilerSchemaV5,
			sourceRevision: historicalV6SourceRevision,
			canonicalSize:  128624,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ref := mustArchivedRef(t, test.ref)
			first := mustLoadExact(t, ref)
			assertExactMetadata(
				t,
				first,
				test.ref,
				test.compilerSchema,
				test.sourceRevision,
				test.canonicalSize,
			)

			mutated := first.CanonicalBytes()
			mutated[0] ^= 0xff

			second := mustLoadExact(t, ref)
			assertExactMetadata(
				t,
				second,
				test.ref,
				test.compilerSchema,
				test.sourceRevision,
				test.canonicalSize,
			)
			secondCanonical := second.CanonicalBytes()
			if secondCanonical[0] == mutated[0] {
				t.Fatal("caller mutation changed the embedded historical artifact")
			}
		})
	}
}

func TestHistoricalArchiveHasNoFallback(t *testing.T) {
	t.Parallel()
	unknown := mustArchivedRef(
		t,
		"typeenv:sha256:0000000000000000000000000000000000000000000000000000000000000000",
	)
	_, err := LoadExact(unknown)
	if !errors.Is(err, ErrExactArtifactNotFound) {
		t.Fatalf("LoadExact(unknown) error = %v, want exact not found", err)
	}
}

func mustLoadExact(
	t *testing.T,
	ref typedmemory.TypeEnvRef,
) typeenv.BaseTypeEnvArtifact {
	t.Helper()
	artifact, err := LoadExact(ref)
	if err != nil {
		reference := ref.String()
		t.Fatalf("LoadExact(%s) error = %v", reference, err)
	}
	return artifact
}

func assertExactMetadata(
	t *testing.T,
	artifact typeenv.BaseTypeEnvArtifact,
	wantRef string,
	wantCompilerSchema string,
	wantSourceRevision string,
	wantCanonicalSize int,
) {
	t.Helper()
	ref, present := artifact.TypeEnvRef()
	reference := ref.String()
	if !present || reference != wantRef {
		t.Fatalf(
			"TypeEnvRef = %q, present = %t, want %q",
			reference,
			present,
			wantRef,
		)
	}
	wantDigest := "sha256:" + wantRef[len("typeenv:sha256:"):]
	digest := artifact.Digest().String()
	if digest != wantDigest {
		t.Fatalf(
			"artifact digest = %q, want %q",
			digest,
			wantDigest,
		)
	}
	compilerSchema := artifact.CompilerSchemaVersion().String()
	if compilerSchema != wantCompilerSchema {
		t.Fatalf(
			"compiler schema = %q, want %q",
			compilerSchema,
			wantCompilerSchema,
		)
	}
	sourceRevision := artifact.SourceRevision().String()
	if sourceRevision != wantSourceRevision {
		t.Fatalf(
			"source revision = %q, want %q",
			sourceRevision,
			wantSourceRevision,
		)
	}
	canonical := artifact.CanonicalBytes()
	canonicalSize := len(canonical)
	if canonicalSize != wantCanonicalSize {
		t.Fatalf(
			"canonical size = %d, want %d",
			canonicalSize,
			wantCanonicalSize,
		)
	}
}

func mustArchivedRef(t *testing.T, raw string) typedmemory.TypeEnvRef {
	t.Helper()
	ref, err := typedmemory.ParseTypeEnvRef(raw)
	if err != nil {
		t.Fatalf("ParseTypeEnvRef(%q) error = %v", raw, err)
	}
	return ref
}
