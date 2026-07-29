package projectmemory

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	_ "modernc.org/sqlite"
)

func TestBaseTypeEnvLoaderLoadsExactImmutableArtifact(t *testing.T) {
	t.Parallel()

	artifact := loaderTestBundledArtifact(t)
	snapshot := loaderTestSnapshot(t, artifact, typedmemorystore.BaseTypeEnvSnapshotFormat)
	loader := NewBaseTypeEnvLoader()

	environment, registry, err := loader.LoadTypeEnv(snapshot)
	if err != nil {
		t.Fatalf("LoadTypeEnv() error = %v", err)
	}
	reference, exists := artifact.TypeEnvRef()
	if !exists {
		t.Fatal("compiled fixture has no TypeEnvRef")
	}
	if environment.Ref() != reference {
		t.Fatalf("environment ref = %q, want %q", environment.Ref().String(), reference.String())
	}
	if environment.Ref().Digest() != artifact.Digest() {
		t.Fatalf(
			"environment digest = %q, want artifact digest %q",
			environment.Ref().Digest().String(),
			artifact.Digest().String(),
		)
	}
	if environment.SourceRevision() != artifact.SourceRevision() {
		t.Fatalf(
			"source revision = %q, want %q",
			environment.SourceRevision().String(),
			artifact.SourceRevision().String(),
		)
	}
	if environment.CompilerSchemaVersion() != artifact.CompilerSchemaVersion() {
		t.Fatalf(
			"compiler schema = %q, want %q",
			environment.CompilerSchemaVersion().String(),
			artifact.CompilerSchemaVersion().String(),
		)
	}
	assertLoaderTestCoverageEqual(t, environment.CoverageManifest(), artifact.CoverageManifest())
	assertLoaderTestCodecsComplete(t, environment, registry)

	callerBytes := snapshot.CanonicalBytes()
	callerBytes[0] ^= 0xff
	reloaded, _, err := loader.LoadTypeEnv(snapshot)
	if err != nil {
		t.Fatalf("LoadTypeEnv() after caller mutation error = %v", err)
	}
	if reloaded.Ref() != environment.Ref() {
		t.Fatal("mutating caller-owned snapshot bytes changed a later load")
	}

	callerCoverage := environment.CoverageManifest().Entries()
	callerCoverage[0] = typedmemory.CoverageEntry{}
	assertLoaderTestCoverageEqual(t, environment.CoverageManifest(), artifact.CoverageManifest())
}

func TestNewBaseTypeEnvSnapshotBuildsExactExecutableArtifactSnapshot(
	t *testing.T,
) {
	t.Parallel()

	artifact := loaderTestBundledArtifact(t)
	snapshot, err := NewBaseTypeEnvSnapshot(artifact)
	if err != nil {
		t.Fatalf("NewBaseTypeEnvSnapshot() error = %v", err)
	}
	reference, exists := artifact.TypeEnvRef()
	if !exists {
		t.Fatal("compiled fixture has no TypeEnvRef")
	}
	if snapshot.Ref() != reference ||
		snapshot.ArtifactDigest() != artifact.Digest() ||
		snapshot.Format().String() != typedmemorystore.BaseTypeEnvSnapshotFormat ||
		snapshot.SourceRevision() != artifact.SourceRevision() ||
		snapshot.CompilerSchemaVersion() != artifact.CompilerSchemaVersion() {
		t.Fatalf("base snapshot coordinates differ from artifact: %#v", snapshot)
	}
	if !bytes.Equal(snapshot.CanonicalBytes(), artifact.CanonicalBytes()) {
		t.Fatal("base snapshot bytes differ from artifact")
	}
	environment, _, err := NewBaseTypeEnvLoader().LoadTypeEnv(snapshot)
	if err != nil {
		t.Fatalf("LoadTypeEnv(NewBaseTypeEnvSnapshot()) error = %v", err)
	}
	if environment.Ref() != reference {
		t.Fatal("base snapshot does not round-trip through the production loader")
	}
}

func TestNewBaseTypeEnvSnapshotRejectsCoverageOnlyArtifact(t *testing.T) {
	t.Parallel()

	compiled := loaderTestBundledArtifact(t)
	coverageOnly := loaderTestCoverageOnlyArtifact(t, compiled)
	_, err := NewBaseTypeEnvSnapshot(coverageOnly)
	if !errors.Is(err, ErrTypeEnvSnapshotPayload) {
		t.Fatalf(
			"NewBaseTypeEnvSnapshot(coverage-only) error = %v, want errors.Is(%v)",
			err,
			ErrTypeEnvSnapshotPayload,
		)
	}
}

func TestBaseTypeEnvLoaderFailsClosed(t *testing.T) {
	t.Parallel()

	compiled := loaderTestBundledArtifact(t)
	tests := []struct {
		name     string
		snapshot typedmemorystore.TypeEnvSnapshot
		want     error
	}{
		{
			name: "missing snapshot",
			want: ErrTypeEnvSnapshotMissing,
		},
		{
			name:     "unsupported format",
			snapshot: loaderTestSnapshot(t, compiled, "project-typeenv-extension.v1"),
			want:     ErrTypeEnvSnapshotFormat,
		},
		{
			name: "malformed canonical payload",
			snapshot: loaderTestRawSnapshot(
				t,
				[]byte("not-a-base-typeenv-artifact"),
				compiled.SourceRevision(),
				compiled.CompilerSchemaVersion(),
			),
			want: ErrTypeEnvSnapshotPayload,
		},
		{
			name:     "coverage-only payload",
			snapshot: loaderTestSnapshot(t, loaderTestCoverageOnlyArtifact(t, compiled), typedmemorystore.BaseTypeEnvSnapshotFormat),
			want:     ErrTypeEnvSnapshotPayload,
		},
		{
			name:     "source revision mismatch",
			snapshot: loaderTestSnapshotWithBasis(t, compiled, "other-source-revision", compiled.CompilerSchemaVersion().String()),
			want:     ErrTypeEnvSnapshotBasis,
		},
		{
			name:     "compiler schema mismatch",
			snapshot: loaderTestSnapshotWithBasis(t, compiled, compiled.SourceRevision().String(), "other-compiler.v1"),
			want:     ErrTypeEnvSnapshotBasis,
		},
	}

	loader := NewBaseTypeEnvLoader()
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := loader.LoadTypeEnv(test.snapshot)
			if !errors.Is(err, test.want) {
				t.Fatalf("LoadTypeEnv() error = %v, want errors.Is(%v)", err, test.want)
			}
		})
	}
}

func loaderTestBundledArtifact(t *testing.T) typeenv.BaseTypeEnvArtifact {
	t.Helper()

	path := filepath.Join("..", "cli", "fpf.db")
	database, err := sql.Open("sqlite", "file:"+path+"?mode=ro&immutable=1")
	if err != nil {
		t.Fatalf("open bundled FPF database read-only: %v", err)
	}
	defer func() { _ = database.Close() }()

	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		t.Fatalf("LoadArtifactReadOnlyDB() error = %v", err)
	}
	return artifact
}

func loaderTestCoverageOnlyArtifact(
	t *testing.T,
	base typeenv.BaseTypeEnvArtifact,
) typeenv.BaseTypeEnvArtifact {
	t.Helper()

	entries := base.CoverageManifest().Entries()
	if len(entries) == 0 {
		t.Fatal("base artifact has no coverage entry for coverage-only fixture")
	}
	entry, err := typedmemory.NewSourceOnlyCoverageEntry(
		entries[0].Subject(),
		entries[0].Source(),
		"fixture_source_only",
	)
	if err != nil {
		t.Fatalf("NewSourceOnlyCoverageEntry() error = %v", err)
	}
	coverage, err := typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{entry})
	if err != nil {
		t.Fatalf("NewCoverageManifest() error = %v", err)
	}
	ir, err := typeenv.NewCoverageOnlyLinkedTypeEnvIR(
		base.SourceRevision(),
		base.CompilerSchemaVersion(),
		coverage,
		"fixture source is not executable",
	)
	if err != nil {
		t.Fatalf("NewCoverageOnlyLinkedTypeEnvIR() error = %v", err)
	}
	artifact, err := typeenv.SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv() error = %v", err)
	}
	return artifact
}

func loaderTestSnapshot(
	t *testing.T,
	artifact typeenv.BaseTypeEnvArtifact,
	formatText string,
) typedmemorystore.TypeEnvSnapshot {
	t.Helper()
	return loaderTestRawSnapshotWithFormat(
		t,
		artifact.CanonicalBytes(),
		artifact.SourceRevision(),
		artifact.CompilerSchemaVersion(),
		formatText,
	)
}

func loaderTestSnapshotWithBasis(
	t *testing.T,
	artifact typeenv.BaseTypeEnvArtifact,
	revisionText string,
	compilerText string,
) typedmemorystore.TypeEnvSnapshot {
	t.Helper()
	revision := loaderTestSourceRevision(t, revisionText)
	compiler := loaderTestCompilerVersion(t, compilerText)
	return loaderTestRawSnapshot(t, artifact.CanonicalBytes(), revision, compiler)
}

func loaderTestRawSnapshot(
	t *testing.T,
	canonical []byte,
	revision typedmemory.SourceRevision,
	compiler typedmemory.CompilerSchemaVersion,
) typedmemorystore.TypeEnvSnapshot {
	t.Helper()
	return loaderTestRawSnapshotWithFormat(
		t,
		canonical,
		revision,
		compiler,
		typedmemorystore.BaseTypeEnvSnapshotFormat,
	)
}

func loaderTestRawSnapshotWithFormat(
	t *testing.T,
	canonical []byte,
	revision typedmemory.SourceRevision,
	compiler typedmemory.CompilerSchemaVersion,
	formatText string,
) typedmemorystore.TypeEnvSnapshot {
	t.Helper()
	digest := loaderTestDigest(t, canonical)
	reference, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef() error = %v", err)
	}
	format, err := typedmemorystore.NewSnapshotFormat(formatText)
	if err != nil {
		t.Fatalf("NewSnapshotFormat() error = %v", err)
	}
	snapshot, err := typedmemorystore.NewTypeEnvSnapshotBuilder(reference).
		SetFormat(format).
		SetCanonicalBytes(canonical).
		SetSourceRevision(revision).
		SetCompilerSchemaVersion(compiler).
		Build()
	if err != nil {
		t.Fatalf("TypeEnvSnapshotBuilder.Build() error = %v", err)
	}
	return snapshot
}

func loaderTestSourceRevision(t *testing.T, raw string) typedmemory.SourceRevision {
	t.Helper()
	value, err := typedmemory.NewSourceRevision(raw)
	if err != nil {
		t.Fatalf("NewSourceRevision() error = %v", err)
	}
	return value
}

func loaderTestCompilerVersion(t *testing.T, raw string) typedmemory.CompilerSchemaVersion {
	t.Helper()
	value, err := typedmemory.NewCompilerSchemaVersion(raw)
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion() error = %v", err)
	}
	return value
}

func loaderTestDigest(t *testing.T, value []byte) typedmemory.SHA256Digest {
	t.Helper()
	sum := sha256.Sum256(value)
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + hex.EncodeToString(sum[:]),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	return digest
}

func assertLoaderTestCoverageEqual(
	t *testing.T,
	got typedmemory.CoverageManifest,
	want typedmemory.CoverageManifest,
) {
	t.Helper()
	gotEntries := got.Entries()
	wantEntries := want.Entries()
	if len(gotEntries) != len(wantEntries) {
		t.Fatalf("coverage entries = %d, want %d", len(gotEntries), len(wantEntries))
	}
	for index := range wantEntries {
		gotEntry := gotEntries[index]
		wantEntry := wantEntries[index]
		gotSource := gotEntry.Source()
		wantSource := wantEntry.Source()
		gotPattern, gotHasPattern := gotSource.PatternID()
		wantPattern, wantHasPattern := wantSource.PatternID()
		if gotEntry.Subject().String() != wantEntry.Subject().String() ||
			gotEntry.Posture() != wantEntry.Posture() ||
			gotEntry.Rationale() != wantEntry.Rationale() ||
			gotSource.UnitID() != wantSource.UnitID() ||
			gotSource.ContentHash() != wantSource.ContentHash() ||
			gotSource.Revision() != wantSource.Revision() ||
			gotSource.LineRange() != wantSource.LineRange() ||
			gotHasPattern != wantHasPattern ||
			gotPattern != wantPattern {
			t.Fatalf("coverage entry %d differs from sealed artifact", index)
		}
	}
}

func assertLoaderTestCodecsComplete(
	t *testing.T,
	environment typedmemory.TypeEnv,
	registry typedmemory.CodecRegistry,
) {
	t.Helper()
	bindings := environment.ValueBindings()
	if len(bindings) == 0 {
		t.Fatal("bundled TypeEnv fixture has no executable value bindings")
	}
	for _, binding := range bindings {
		if !registry.Contains(binding.Codec()) {
			t.Fatalf("codec registry omitted %q", binding.Codec().String())
		}
	}
}
