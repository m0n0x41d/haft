package artifacts

import (
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestHistoricalV3ArtifactIsExactAndPrivatelyDecoded(t *testing.T) {
	t.Parallel()
	ref := mustArchivedRef(t, HistoricalV3Ref)
	first, err := LoadExact(ref)
	if err != nil {
		t.Fatalf("LoadExact(first) error = %v", err)
	}
	if first.CompilerSchemaVersion().String() != typeenv.BaseTypeEnvCompilerSchemaV3 {
		t.Fatalf(
			"compiler schema = %q, want %q",
			first.CompilerSchemaVersion().String(),
			typeenv.BaseTypeEnvCompilerSchemaV3,
		)
	}
	if first.SourceRevision().String() != historicalV3SourceRevision {
		t.Fatalf(
			"source revision = %q, want %q",
			first.SourceRevision().String(),
			historicalV3SourceRevision,
		)
	}
	mutated := first.CanonicalBytes()
	mutated[0] ^= 0xff
	second, err := LoadExact(ref)
	if err != nil {
		t.Fatalf("LoadExact(second) error = %v", err)
	}
	if second.CanonicalBytes()[0] == mutated[0] {
		t.Fatal("caller mutation changed the embedded historical artifact")
	}
}

func TestHistoricalArchiveHasNoFallback(t *testing.T) {
	t.Parallel()
	unknown := mustArchivedRef(
		t,
		"typeenv:sha256:28c7650b8933cbf6feb5d87965d48b4a8c7b80ae71c9c0ca4990d8ae7b6a36b6",
	)
	_, err := LoadExact(unknown)
	if !errors.Is(err, ErrExactArtifactNotFound) {
		t.Fatalf("LoadExact(unknown) error = %v, want exact not found", err)
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
