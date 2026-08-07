package typedmemorystore

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestSQLiteAdapterPutTypeEnvSnapshotRejectsLoaderFailureBeforePersistence(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "rejected-loader.db")
	store := openStoreAt(t, databasePath)
	snapshot := newLoaderContractSnapshot(t)
	loaderFailure := errors.New("injected TypeEnv loader rejection")
	loader := snapshotRejectingTypeEnvLoader{err: loaderFailure}
	clock := fixedClock{value: time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC)}
	adapter, err := newDeclarationSQLiteAdapter(store.GetRawDB(), loader, clock)
	if err != nil {
		t.Fatalf("NewSQLiteAdapter: %v", err)
	}

	err = adapter.PutTypeEnvSnapshot(context.Background(), snapshot)
	if !errors.Is(err, loaderFailure) {
		t.Fatalf("PutTypeEnvSnapshot error = %v; want injected loader rejection", err)
	}
	assertTypedMemoryRowCounts(t, store.GetRawDB(), map[string]int64{
		"typed_memory_type_env_snapshots": 0,
	})
}

func TestSQLiteAdapterPutTypeEnvSnapshotRejectsLoadedMetadataDriftBeforePersistence(t *testing.T) {
	snapshot := newLoaderContractSnapshot(t)
	tests := []struct {
		name            string
		sourceRevision  typedmemory.SourceRevision
		compilerVersion typedmemory.CompilerSchemaVersion
	}{
		{
			name:            "source revision",
			sourceRevision:  mustSourceRevision(t, "different-fpf-revision"),
			compilerVersion: snapshot.CompilerSchemaVersion(),
		},
		{
			name:            "compiler schema version",
			sourceRevision:  snapshot.SourceRevision(),
			compilerVersion: mustCompilerVersion(t, "different.compiler.v2"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			databasePath := filepath.Join(t.TempDir(), "metadata-drift.db")
			store := openStoreAt(t, databasePath)
			environment := newLoaderContractEnvironment(
				t,
				snapshot.Ref(),
				test.sourceRevision,
				test.compilerVersion,
			)
			loader := staticTypeEnvLoader{
				reference:   snapshot.Ref(),
				environment: environment,
				registry:    typedmemory.NewCodecRegistry(),
			}
			clock := fixedClock{value: time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC)}
			adapter, err := newDeclarationSQLiteAdapter(store.GetRawDB(), loader, clock)
			if err != nil {
				t.Fatalf("NewSQLiteAdapter: %v", err)
			}

			err = adapter.PutTypeEnvSnapshot(context.Background(), snapshot)
			if err == nil {
				t.Fatal("PutTypeEnvSnapshot accepted loader metadata drift")
			}
			assertTypedMemoryRowCounts(t, store.GetRawDB(), map[string]int64{
				"typed_memory_type_env_snapshots": 0,
			})
		})
	}
}

type snapshotRejectingTypeEnvLoader struct {
	err error
}

func (loader snapshotRejectingTypeEnvLoader) LoadTypeEnv(
	TypeEnvSnapshot,
) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error) {
	return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, loader.err
}

func newLoaderContractSnapshot(t *testing.T) TypeEnvSnapshot {
	t.Helper()
	canonicalBytes := []byte(`{"schema":"test.loader-contract.typeenv/v1"}`)
	reference := mustTypeEnvRef(t, canonicalBytes)
	format, err := NewSnapshotFormat(BaseTypeEnvSnapshotFormat)
	if err != nil {
		t.Fatalf("NewSnapshotFormat: %v", err)
	}
	snapshot, err := NewTypeEnvSnapshotBuilder(reference).
		SetFormat(format).
		SetCanonicalBytes(canonicalBytes).
		SetSourceRevision(mustSourceRevision(t, "test-loader-fpf-revision")).
		SetCompilerSchemaVersion(mustCompilerVersion(t, "test.loader.compiler.v1")).
		Build()
	if err != nil {
		t.Fatalf("build loader-contract TypeEnv snapshot: %v", err)
	}
	return snapshot
}

func newLoaderContractEnvironment(
	t *testing.T,
	reference typedmemory.TypeEnvRef,
	sourceRevision typedmemory.SourceRevision,
	compilerVersion typedmemory.CompilerSchemaVersion,
) typedmemory.TypeEnv {
	t.Helper()
	contextRef := mustContextRef(t, "ctx:test-loader-contract")
	provenance := mustFPFProvenance(t, sourceRevision)
	boundedContext, err := typedmemory.NewBoundedContext(contextRef, provenance)
	if err != nil {
		t.Fatalf("NewBoundedContext: %v", err)
	}
	coverageSubject, err := typedmemory.SourceUnitCoverage(provenance.Location().UnitID())
	if err != nil {
		t.Fatalf("SourceUnitCoverage: %v", err)
	}
	coverageEntry, err := typedmemory.NewCompiledCoverageEntry(
		coverageSubject,
		provenance.Location(),
	)
	if err != nil {
		t.Fatalf("NewCompiledCoverageEntry: %v", err)
	}
	coverage, err := typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{coverageEntry})
	if err != nil {
		t.Fatalf("NewCoverageManifest: %v", err)
	}
	environment, err := typedmemory.NewTypeEnvBuilder(reference).
		SetSourceRevision(sourceRevision).
		SetCompilerSchemaVersion(compilerVersion).
		SetCoverageManifest(coverage).
		AddBoundedContext(boundedContext).
		Build()
	if err != nil {
		t.Fatalf("build loader-contract TypeEnv: %v", err)
	}
	return environment
}
