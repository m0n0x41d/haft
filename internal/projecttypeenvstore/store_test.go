package projecttypeenvstore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"

	_ "modernc.org/sqlite"
)

func TestArtifactClosureRoundTripsExactArtifactsAndMetadata(t *testing.T) {
	ctx := context.Background()
	database, store := openStoreFixture(t, ctx)
	fixture := newArtifactClosureFixture(t)

	if err := store.PutArtifactClosure(ctx, fixture.closure); err != nil {
		t.Fatalf("PutArtifactClosure(): %v", err)
	}
	baseRef, exists := fixture.base.TypeEnvRef()
	if !exists {
		t.Fatal("base fixture lost TypeEnvRef")
	}
	loadedBase, err := store.GetBaseTypeEnvArtifact(ctx, baseRef)
	if err != nil {
		t.Fatalf("GetBaseTypeEnvArtifact(): %v", err)
	}
	loadedExtension, err := store.GetProjectTypeEnvExtensionArtifact(ctx, fixture.extension.Ref())
	if err != nil {
		t.Fatalf("GetProjectTypeEnvExtensionArtifact(): %v", err)
	}
	loadedRuntime, err := store.GetRuntimeEvaluationBasisArtifact(ctx, fixture.runtime.Ref())
	if err != nil {
		t.Fatalf("GetRuntimeEvaluationBasisArtifact(): %v", err)
	}
	loadedComposite, err := store.GetProjectTypeEnvCompositeArtifact(ctx, fixture.composite.Ref())
	if err != nil {
		t.Fatalf("GetProjectTypeEnvCompositeArtifact(): %v", err)
	}
	assertExactBytes(t, "B", loadedBase.CanonicalBytes(), fixture.base.CanonicalBytes())
	assertExactBytes(t, "E", loadedExtension.CanonicalBytes(), fixture.extension.CanonicalBytes())
	assertExactBytes(t, "X", loadedRuntime.CanonicalBytes(), fixture.runtime.CanonicalBytes())
	assertExactBytes(t, "C", loadedComposite.CanonicalBytes(), fixture.composite.CanonicalBytes())

	rows, err := database.QueryContext(
		ctx,
		`SELECT artifact_kind, canonical_schema_version, producer_schema_version
		 FROM project_typeenv_artifacts ORDER BY artifact_kind`,
	)
	if err != nil {
		t.Fatalf("query artifact metadata: %v", err)
	}
	defer rows.Close()
	metadata := make(map[string][2]string)
	for rows.Next() {
		var kind, canonicalSchema, producerSchema string
		if err := rows.Scan(&kind, &canonicalSchema, &producerSchema); err != nil {
			t.Fatalf("scan artifact metadata: %v", err)
		}
		metadata[kind] = [2]string{canonicalSchema, producerSchema}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("artifact metadata rows: %v", err)
	}
	want := map[string][2]string{
		string(ArtifactBaseTypeEnv): {
			baseArtifactCanonicalSchema,
			fixture.base.CompilerSchemaVersion().String(),
		},
		string(ArtifactExtensionTypeEnv): {
			extensionArtifactCanonicalSchema,
			fixture.extension.IR().CompilerVersion().Value(),
		},
		string(ArtifactRuntimeBasis): {
			runtimeBasisCanonicalSchema,
			runtimeBasisCanonicalSchema,
		},
		string(ArtifactCompositeTypeEnv): {
			compositeArtifactCanonicalSchema,
			fixture.composite.LowererSchemaVersion(),
		},
	}
	if fmt.Sprint(metadata) != fmt.Sprint(want) {
		t.Fatalf("stored metadata = %#v, want %#v", metadata, want)
	}
}

func TestPrepareArtifactClosurePreservesSealedHistoricalLowererRecipe(t *testing.T) {
	fixture := newArtifactClosureFixture(t)
	linked := acceptedLinkedFixture(
		t,
		projecttypeenv.LinkProjectTypeEnvCompositeIR(
			fixture.base,
			[]projecttypeenv.ProjectTypeEnvExtensionArtifact{fixture.extension},
		),
	)
	historical, err := projecttypeenv.ResealHistoricalProjectTypeEnvCompositeV1(
		linked,
		fixture.runtime,
	)
	if err != nil {
		t.Fatalf("ResealHistoricalProjectTypeEnvCompositeV1(): %v", err)
	}
	closure, err := PrepareArtifactClosure(
		fixture.base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{fixture.extension},
		fixture.runtime,
		historical,
	)
	if err != nil {
		t.Fatalf("PrepareArtifactClosure(historical): %v", err)
	}
	if closure.Composite().Ref() != historical.Ref() ||
		closure.Composite().LowererSchemaVersion() != projecttypeenv.ProjectTypeEnvCompositeLowererSchemaV1 {
		t.Fatalf(
			"historical closure composite = %s at %s",
			closure.Composite().Ref().String(),
			closure.Composite().LowererSchemaVersion(),
		)
	}
}

func TestArtifactClosureIsIdempotentAndRejectsCoordinateConflict(t *testing.T) {
	ctx := context.Background()
	database, store := openStoreFixture(t, ctx)
	fixture := newArtifactClosureFixture(t)
	if err := store.PutArtifactClosure(ctx, fixture.closure); err != nil {
		t.Fatalf("first PutArtifactClosure(): %v", err)
	}
	if err := store.PutArtifactClosure(ctx, fixture.closure); err != nil {
		t.Fatalf("idempotent PutArtifactClosure(): %v", err)
	}
	if got := artifactRowCount(t, ctx, database); got != 4 {
		t.Fatalf("artifact row count = %d, want 4", got)
	}

	_, err := database.ExecContext(
		ctx,
		`UPDATE project_typeenv_artifacts
		 SET canonical_bytes = ?
		 WHERE artifact_kind = ? AND artifact_ref = ?`,
		[]byte("different bytes at the same exact coordinate"),
		string(ArtifactExtensionTypeEnv),
		fixture.extension.Ref().String(),
	)
	if err != nil {
		t.Fatalf("inject coordinate conflict: %v", err)
	}
	err = store.PutProjectTypeEnvExtensionArtifact(ctx, fixture.extension)
	if !errors.Is(err, ErrArtifactConflict) {
		t.Fatalf("coordinate conflict error = %v, want ErrArtifactConflict", err)
	}
}

func TestEveryArtifactPutRejectsClaimedMetadataThatDoesNotMatchCanonicalBytes(
	t *testing.T,
) {
	fixture := newArtifactClosureFixture(t)
	tests := []struct {
		name   string
		record artifactRecord
	}{
		{name: "B", record: fixture.closure.records[0]},
		{name: "E", record: fixture.closure.records[1]},
		{name: "X", record: fixture.closure.records[2]},
		{name: "C", record: fixture.closure.records[3]},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			database, store := openStoreFixture(t, ctx)
			forged := test.record.clone()
			forged.producerSchema = "forged.producer/v9"
			err := store.putOne(ctx, forged)
			if !errors.Is(err, ErrArtifactIntegrity) {
				t.Fatalf("put forged %s metadata error = %v", test.name, err)
			}
			if got := artifactRowCount(t, ctx, database); got != 0 {
				t.Fatalf("artifact row count after forged %s = %d, want 0", test.name, got)
			}
		})
	}
}

func TestGetFailsClosedOnCorruptionMetadataDriftAndCrossKindSubstitution(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(context.Context, *testing.T, *sql.DB, artifactClosureFixture)
	}{
		{
			name: "trailing corruption",
			mutate: func(ctx context.Context, t *testing.T, database *sql.DB, fixture artifactClosureFixture) {
				t.Helper()
				corrupt := append(fixture.base.CanonicalBytes(), 0x00)
				updateBaseCanonical(t, ctx, database, fixture, corrupt)
			},
		},
		{
			name: "metadata drift",
			mutate: func(ctx context.Context, t *testing.T, database *sql.DB, fixture artifactClosureFixture) {
				t.Helper()
				baseRef, _ := fixture.base.TypeEnvRef()
				_, err := database.ExecContext(
					ctx,
					`UPDATE project_typeenv_artifacts
					 SET producer_schema_version = 'forged.compiler/v9'
					 WHERE artifact_kind = ? AND artifact_ref = ?`,
					string(ArtifactBaseTypeEnv),
					baseRef.String(),
				)
				if err != nil {
					t.Fatalf("inject metadata drift: %v", err)
				}
			},
		},
		{
			name: "cross-kind substitution",
			mutate: func(ctx context.Context, t *testing.T, database *sql.DB, fixture artifactClosureFixture) {
				t.Helper()
				updateBaseCanonical(
					t,
					ctx,
					database,
					fixture,
					fixture.composite.CanonicalBytes(),
				)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			database, store := openStoreFixture(t, ctx)
			fixture := newArtifactClosureFixture(t)
			if err := store.PutArtifactClosure(ctx, fixture.closure); err != nil {
				t.Fatalf("PutArtifactClosure(): %v", err)
			}
			test.mutate(ctx, t, database, fixture)
			baseRef, _ := fixture.base.TypeEnvRef()
			_, err := store.GetBaseTypeEnvArtifact(ctx, baseRef)
			if !errors.Is(err, ErrArtifactIntegrity) {
				t.Fatalf("GetBaseTypeEnvArtifact() error = %v, want ErrArtifactIntegrity", err)
			}
		})
	}
}

func TestEveryTypedGetRejectsTrailingCanonicalBytes(t *testing.T) {
	kinds := []ArtifactKind{
		ArtifactBaseTypeEnv,
		ArtifactExtensionTypeEnv,
		ArtifactRuntimeBasis,
		ArtifactCompositeTypeEnv,
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			ctx := context.Background()
			database, store := openStoreFixture(t, ctx)
			fixture := newArtifactClosureFixture(t)
			if err := store.PutArtifactClosure(ctx, fixture.closure); err != nil {
				t.Fatalf("PutArtifactClosure(): %v", err)
			}
			ref, canonical := artifactFixtureCoordinate(fixture, kind)
			corrupt := append(canonical, 0x00)
			_, err := database.ExecContext(
				ctx,
				`UPDATE project_typeenv_artifacts
				 SET canonical_bytes = ?
				 WHERE artifact_kind = ? AND artifact_ref = ?`,
				corrupt,
				string(kind),
				ref,
			)
			if err != nil {
				t.Fatalf("inject trailing bytes: %v", err)
			}
			err = getFixtureArtifact(ctx, store, fixture, kind)
			if !errors.Is(err, ErrArtifactIntegrity) {
				t.Fatalf("typed Get(%s) error = %v, want integrity failure", kind, err)
			}
		})
	}
}

func TestArtifactClosureRollsBackEveryArtifactWhenOneWriteFails(t *testing.T) {
	ctx := context.Background()
	database, store := openStoreFixture(t, ctx)
	base := compiledBaseFixture(t)
	extension := extensionFixture(t, base)
	runtime := newNonEmptyRuntimeFixture(t)
	linked := acceptedLinkedFixture(
		t,
		projecttypeenv.LinkProjectTypeEnvCompositeIR(
			base,
			[]projecttypeenv.ProjectTypeEnvExtensionArtifact{extension},
		),
	)
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(linked, runtime.basis)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(): %v", err)
	}
	closure, err := PrepareArtifactClosureWithRuntimeMechanisms(
		base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{extension},
		runtime.basis,
		composite,
		[]runtimemechanism.RuntimeMechanismArtifactV1{runtime.mechanism},
	)
	if err != nil {
		t.Fatalf("PrepareArtifactClosureWithRuntimeMechanisms(): %v", err)
	}
	_, err = database.ExecContext(
		ctx,
		`CREATE TRIGGER inject_runtime_basis_failure
		 BEFORE INSERT ON project_typeenv_artifacts
		 WHEN NEW.artifact_kind = 'runtime_evaluation_basis'
		 BEGIN
			SELECT RAISE(ABORT, 'injected runtime basis failure');
		 END`,
	)
	if err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	err = store.PutArtifactClosure(ctx, closure)
	if err == nil {
		t.Fatal("PutArtifactClosure() accepted injected X write failure")
	}
	if got := artifactRowCount(t, ctx, database); got != 0 {
		t.Fatalf("artifact row count after rollback = %d, want 0", got)
	}
	if got := runtimeMechanismRowCount(t, ctx, database); got != 0 {
		t.Fatalf("runtime mechanism row count after rollback = %d, want 0", got)
	}
}

func TestConcurrentIdenticalClosurePutsConvergeOnOneImmutableSet(t *testing.T) {
	ctx := context.Background()
	database, store := openStoreFixture(t, ctx)
	fixture := newArtifactClosureFixture(t)
	const workerCount = 12
	start := make(chan struct{})
	errorsByWorker := make(chan error, workerCount)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for index := 0; index < workerCount; index++ {
		go func() {
			defer workers.Done()
			<-start
			errorsByWorker <- store.PutArtifactClosure(ctx, fixture.closure)
		}()
	}
	close(start)
	workers.Wait()
	close(errorsByWorker)
	for err := range errorsByWorker {
		if err != nil {
			t.Fatalf("concurrent PutArtifactClosure(): %v", err)
		}
	}
	if got := artifactRowCount(t, ctx, database); got != 4 {
		t.Fatalf("artifact row count = %d, want 4", got)
	}
}

func TestCoverageOnlyBaseCannotEnterArtifactStoreOrClosure(t *testing.T) {
	ctx := context.Background()
	_, store := openStoreFixture(t, ctx)
	coverageOnly := coverageOnlyBaseFixture(t)
	err := store.PutBaseTypeEnvArtifact(ctx, coverageOnly)
	if !errors.Is(err, ErrBaseNotExecutable) {
		t.Fatalf("PutBaseTypeEnvArtifact(coverage-only) error = %v", err)
	}

	fixture := newArtifactClosureFixture(t)
	_, err = PrepareArtifactClosure(
		coverageOnly,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{fixture.extension},
		fixture.runtime,
		fixture.composite,
	)
	if !errors.Is(err, ErrBaseNotExecutable) {
		t.Fatalf("PrepareArtifactClosure(coverage-only) error = %v", err)
	}
}

func TestPrepareArtifactClosureRejectsCompositeFromDifferentEDAG(t *testing.T) {
	fixture := newArtifactClosureFixture(t)
	linkedWithoutExtension := acceptedLinkedFixture(
		t,
		projecttypeenv.LinkProjectTypeEnvCompositeIR(fixture.base, nil),
	)
	compositeWithoutExtension, err := projecttypeenv.SealProjectTypeEnvComposite(
		linkedWithoutExtension,
		fixture.runtime,
	)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(without E): %v", err)
	}
	_, err = PrepareArtifactClosure(
		fixture.base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{fixture.extension},
		fixture.runtime,
		compositeWithoutExtension,
	)
	if !errors.Is(err, ErrClosureInconsistent) {
		t.Fatalf("PrepareArtifactClosure(wrong E DAG) error = %v", err)
	}
}

func TestArtifactClosureCanonicalizesExtensionPermutationAndRoundTripsEveryE(
	t *testing.T,
) {
	ctx := context.Background()
	database, store := openStoreFixture(t, ctx)
	base := compiledBaseFixture(t)
	alpha := extensionFixtureNamed(
		t,
		base,
		"haft.alpha-store-fixture",
		"haft-alpha-store-fixture",
		"Haft.AlphaStoreFixture",
	)
	beta := extensionFixtureNamed(
		t,
		base,
		"haft.beta-store-fixture",
		"haft-beta-store-fixture",
		"Haft.BetaStoreFixture",
	)
	runtime, err := projecttypeenv.SealRuntimeEvaluationBasis(nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(): %v", err)
	}
	linked := acceptedLinkedFixture(
		t,
		projecttypeenv.LinkProjectTypeEnvCompositeIR(
			base,
			[]projecttypeenv.ProjectTypeEnvExtensionArtifact{beta, alpha},
		),
	)
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(linked, runtime)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(): %v", err)
	}
	forward, err := PrepareArtifactClosure(
		base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{alpha, beta},
		runtime,
		composite,
	)
	if err != nil {
		t.Fatalf("PrepareArtifactClosure(forward): %v", err)
	}
	reverse, err := PrepareArtifactClosure(
		base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{beta, alpha},
		runtime,
		composite,
	)
	if err != nil {
		t.Fatalf("PrepareArtifactClosure(reverse): %v", err)
	}
	forwardExtensions := forward.Extensions()
	reverseExtensions := reverse.Extensions()
	if len(forwardExtensions) != 2 || len(reverseExtensions) != 2 {
		t.Fatalf(
			"canonical extension lengths = %d and %d, want 2",
			len(forwardExtensions),
			len(reverseExtensions),
		)
	}
	for index := range forwardExtensions {
		if forwardExtensions[index].Ref() != reverseExtensions[index].Ref() ||
			forwardExtensions[index].Ref() != composite.ExtensionRefs()[index] {
			t.Fatalf(
				"canonical E[%d] = %s / %s; C expects %s",
				index,
				forwardExtensions[index].Ref(),
				reverseExtensions[index].Ref(),
				composite.ExtensionRefs()[index],
			)
		}
	}
	if err := store.PutArtifactClosure(ctx, reverse); err != nil {
		t.Fatalf("PutArtifactClosure(reverse): %v", err)
	}
	for _, extension := range forwardExtensions {
		loaded, err := store.GetProjectTypeEnvExtensionArtifact(ctx, extension.Ref())
		if err != nil {
			t.Fatalf("GetProjectTypeEnvExtensionArtifact(%s): %v", extension.Ref(), err)
		}
		assertExactBytes(
			t,
			extension.Ref().String(),
			loaded.CanonicalBytes(),
			extension.CanonicalBytes(),
		)
	}
	if got := artifactRowCount(t, ctx, database); got != 5 {
		t.Fatalf("artifact row count = %d, want 5", got)
	}
}

func TestNonEmptyRuntimeBasisRoundTripsItsExactResolvedMechanismClosure(t *testing.T) {
	ctx := context.Background()
	database, store := openStoreFixture(t, ctx)
	base := compiledBaseFixture(t)
	extension := extensionFixture(t, base)
	runtime := newNonEmptyRuntimeFixture(t)
	linked := acceptedLinkedFixture(
		t,
		projecttypeenv.LinkProjectTypeEnvCompositeIR(
			base,
			[]projecttypeenv.ProjectTypeEnvExtensionArtifact{extension},
		),
	)
	composite, err := projecttypeenv.SealProjectTypeEnvComposite(linked, runtime.basis)
	if err != nil {
		t.Fatalf("SealProjectTypeEnvComposite(): %v", err)
	}
	_, err = PrepareArtifactClosure(
		base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{extension},
		runtime.basis,
		composite,
	)
	if !errors.Is(err, ErrRuntimeClosureRequired) {
		t.Fatalf("bare non-empty X preparation error = %v", err)
	}
	closure, err := PrepareArtifactClosureWithRuntimeMechanisms(
		base,
		[]projecttypeenv.ProjectTypeEnvExtensionArtifact{extension},
		runtime.basis,
		composite,
		[]runtimemechanism.RuntimeMechanismArtifactV1{runtime.mechanism},
	)
	if err != nil {
		t.Fatalf("PrepareArtifactClosureWithRuntimeMechanisms(): %v", err)
	}
	if err := store.PutArtifactClosure(ctx, closure); err != nil {
		t.Fatalf("PutArtifactClosure(): %v", err)
	}
	if err := store.PutRuntimeEvaluationBasisClosure(
		ctx,
		runtime.basis,
		[]runtimemechanism.RuntimeMechanismArtifactV1{runtime.mechanism},
	); err != nil {
		t.Fatalf("idempotent PutRuntimeEvaluationBasisClosure(): %v", err)
	}
	loaded, err := store.GetRuntimeEvaluationBasisArtifact(ctx, runtime.basis.Ref())
	if err != nil {
		t.Fatalf("GetRuntimeEvaluationBasisArtifact(): %v", err)
	}
	if err := loaded.VerifyResolvedClosure(); err != nil {
		t.Fatalf("reread X VerifyResolvedClosure(): %v", err)
	}
	assertExactBytes(
		t,
		"non-empty X",
		loaded.CanonicalBytes(),
		runtime.basis.CanonicalBytes(),
	)
	if got := runtimeMechanismRowCount(t, ctx, database); got != 1 {
		t.Fatalf("runtime mechanism row count = %d, want 1", got)
	}
}

func TestNonEmptyRuntimeBasisRereadRejectsMissingAndCorruptMechanismArtifact(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(context.Context, *testing.T, *sql.DB, nonEmptyRuntimeFixture)
	}{
		{
			name: "missing",
			mutate: func(ctx context.Context, t *testing.T, database *sql.DB, runtime nonEmptyRuntimeFixture) {
				t.Helper()
				identity := runtime.mechanism.Identity()
				_, err := database.ExecContext(
					ctx,
					`DELETE FROM project_typeenv_runtime_mechanisms
					 WHERE artifact_ref = ? AND edition = ?`,
					identity.Artifact().String(),
					identity.Edition().String(),
				)
				if err != nil {
					t.Fatalf("delete runtime mechanism: %v", err)
				}
			},
		},
		{
			name: "corrupt",
			mutate: func(ctx context.Context, t *testing.T, database *sql.DB, runtime nonEmptyRuntimeFixture) {
				t.Helper()
				identity := runtime.mechanism.Identity()
				corrupt := append(runtime.mechanism.CanonicalBytes(), 0x00)
				_, err := database.ExecContext(
					ctx,
					`UPDATE project_typeenv_runtime_mechanisms
					 SET canonical_bytes = ?
					 WHERE artifact_ref = ? AND edition = ?`,
					corrupt,
					identity.Artifact().String(),
					identity.Edition().String(),
				)
				if err != nil {
					t.Fatalf("corrupt runtime mechanism: %v", err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			database, store := openStoreFixture(t, ctx)
			runtime := newNonEmptyRuntimeFixture(t)
			if err := store.PutRuntimeEvaluationBasisClosure(
				ctx,
				runtime.basis,
				[]runtimemechanism.RuntimeMechanismArtifactV1{runtime.mechanism},
			); err != nil {
				t.Fatalf("PutRuntimeEvaluationBasisClosure(): %v", err)
			}
			test.mutate(ctx, t, database, runtime)
			_, err := store.GetRuntimeEvaluationBasisArtifact(ctx, runtime.basis.Ref())
			if !errors.Is(err, ErrArtifactIntegrity) {
				t.Fatalf("GetRuntimeEvaluationBasisArtifact() error = %v, want integrity failure", err)
			}
		})
	}
}

func TestSchemaMigrationIsAdditiveIdempotentAndRejectsUnknownFuture(t *testing.T) {
	t.Run("additive and idempotent", func(t *testing.T) {
		ctx := context.Background()
		database := openDatabaseFixture(t)
		_, err := database.ExecContext(ctx, `CREATE TABLE unrelated_fixture (value TEXT NOT NULL)`)
		if err != nil {
			t.Fatalf("create unrelated table: %v", err)
		}
		if _, err := New(ctx, database); err != nil {
			t.Fatalf("first New(): %v", err)
		}
		if _, err := New(ctx, database); err != nil {
			t.Fatalf("second New(): %v", err)
		}
		var version int
		if err := database.QueryRowContext(
			ctx,
			`SELECT version FROM project_typeenv_artifact_store_schema WHERE singleton = 1`,
		).Scan(&version); err != nil {
			t.Fatalf("read schema version: %v", err)
		}
		if version != CurrentSchemaVersion {
			t.Fatalf("schema version = %d, want %d", version, CurrentSchemaVersion)
		}
		var unrelated string
		if err := database.QueryRowContext(
			ctx,
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'unrelated_fixture'`,
		).Scan(&unrelated); err != nil {
			t.Fatalf("unrelated table was not preserved: %v", err)
		}
	})

	t.Run("unknown future", func(t *testing.T) {
		ctx := context.Background()
		database := openDatabaseFixture(t)
		_, err := database.ExecContext(ctx, createSchemaVersionTable)
		if err != nil {
			t.Fatalf("create schema version table: %v", err)
		}
		_, err = database.ExecContext(
			ctx,
			`INSERT INTO project_typeenv_artifact_store_schema (singleton, version) VALUES (1, ?)`,
			CurrentSchemaVersion+1,
		)
		if err != nil {
			t.Fatalf("seed future schema version: %v", err)
		}
		if _, err := New(ctx, database); err == nil {
			t.Fatal("New() accepted unknown future schema version")
		}
	})
}

func openStoreFixture(
	t *testing.T,
	ctx context.Context,
) (*sql.DB, *Store) {
	t.Helper()
	database := openDatabaseFixture(t)
	store, err := New(ctx, database)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	return database, store
}

func openDatabaseFixture(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "project-typeenv-store.sqlite")
	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)",
		path,
	)
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open(): %v", err)
	}
	database.SetMaxOpenConns(16)
	t.Cleanup(func() { _ = database.Close() })
	if err := database.Ping(); err != nil {
		t.Fatalf("database.Ping(): %v", err)
	}
	return database
}

func artifactRowCount(t *testing.T, ctx context.Context, database *sql.DB) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM project_typeenv_artifacts`,
	).Scan(&count); err != nil {
		t.Fatalf("count project TypeEnv artifacts: %v", err)
	}
	return count
}

func runtimeMechanismRowCount(t *testing.T, ctx context.Context, database *sql.DB) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM project_typeenv_runtime_mechanisms`,
	).Scan(&count); err != nil {
		t.Fatalf("count runtime mechanism artifacts: %v", err)
	}
	return count
}

func updateBaseCanonical(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	fixture artifactClosureFixture,
	canonical []byte,
) {
	t.Helper()
	baseRef, _ := fixture.base.TypeEnvRef()
	_, err := database.ExecContext(
		ctx,
		`UPDATE project_typeenv_artifacts
		 SET canonical_bytes = ?
		 WHERE artifact_kind = ? AND artifact_ref = ?`,
		canonical,
		string(ArtifactBaseTypeEnv),
		baseRef.String(),
	)
	if err != nil {
		t.Fatalf("update base canonical bytes: %v", err)
	}
}

func artifactFixtureCoordinate(
	fixture artifactClosureFixture,
	kind ArtifactKind,
) (string, []byte) {
	switch kind {
	case ArtifactBaseTypeEnv:
		ref, _ := fixture.base.TypeEnvRef()
		return ref.String(), fixture.base.CanonicalBytes()
	case ArtifactExtensionTypeEnv:
		return fixture.extension.Ref().String(), fixture.extension.CanonicalBytes()
	case ArtifactRuntimeBasis:
		return fixture.runtime.Ref().String(), fixture.runtime.CanonicalBytes()
	case ArtifactCompositeTypeEnv:
		return fixture.composite.Ref().String(), fixture.composite.CanonicalBytes()
	default:
		return "", nil
	}
}

func getFixtureArtifact(
	ctx context.Context,
	store *Store,
	fixture artifactClosureFixture,
	kind ArtifactKind,
) error {
	switch kind {
	case ArtifactBaseTypeEnv:
		ref, _ := fixture.base.TypeEnvRef()
		_, err := store.GetBaseTypeEnvArtifact(ctx, ref)
		return err
	case ArtifactExtensionTypeEnv:
		_, err := store.GetProjectTypeEnvExtensionArtifact(ctx, fixture.extension.Ref())
		return err
	case ArtifactRuntimeBasis:
		_, err := store.GetRuntimeEvaluationBasisArtifact(ctx, fixture.runtime.Ref())
		return err
	case ArtifactCompositeTypeEnv:
		_, err := store.GetProjectTypeEnvCompositeArtifact(ctx, fixture.composite.Ref())
		return err
	default:
		return fmt.Errorf("unknown fixture artifact kind %q", kind)
	}
}

func assertExactBytes(t *testing.T, name string, got []byte, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("%s canonical bytes changed across persistence", name)
	}
}
