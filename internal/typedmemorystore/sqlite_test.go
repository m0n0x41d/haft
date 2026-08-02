package typedmemorystore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestSQLiteAdapterCommitDeclareEntityPersistsExactClosureAndReopens(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	request := fixture.request(t, 0, fixture.environment.Ref(), "declare:authorization", candidate)

	receipt, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("CommitDeclareEntity: %v", err)
	}
	if receipt.Disposition() != CommitApplied {
		t.Fatalf("disposition = %q; want %q", receipt.Disposition(), CommitApplied)
	}
	if receipt.GraphRevision().Value() != 1 {
		t.Fatalf("committed revision = %d; want 1", receipt.GraphRevision().Value())
	}
	if receipt.EventRef() == "" || receipt.CommitRef() == "" || receipt.ResultDigest().String() == "" {
		t.Fatal("commit receipt lost its durable event, commit, or result digest")
	}

	assertCommittedDeclaration(t, fixture.adapter, fixture, candidate, receipt)
	assertTypedMemoryRowCounts(t, fixture.database, map[string]int64{
		"typed_memory_type_env_snapshots":     1,
		"typed_memory_graph_heads":            1,
		"typed_memory_graph_events":           1,
		"typed_memory_graph_commits":          1,
		"typed_memory_entities":               1,
		"typed_memory_entity_contexts":        1,
		"typed_memory_idempotency_history":    1,
		"typed_memory_projection_jobs":        1,
		"typed_memory_projection_debt_events": 0,
	})

	if err := fixture.store.Close(); err != nil {
		t.Fatalf("close first database handle: %v", err)
	}
	reopened := openStoreAt(t, fixture.databasePath)
	reopenedAdapter := fixture.adapterFor(t, reopened.GetRawDB())
	assertCommittedDeclaration(t, reopenedAdapter, fixture, candidate, receipt)
	loadedSnapshot, err := reopenedAdapter.LoadTypeEnvSnapshot(
		context.Background(),
		fixture.environment.Ref(),
	)
	if err != nil {
		t.Fatalf("LoadTypeEnvSnapshot after reopen: %v", err)
	}
	if !equalTypeEnvSnapshots(loadedSnapshot, fixture.snapshot) {
		t.Fatal("reopened immutable TypeEnv snapshot differs from the inserted snapshot")
	}
}

func TestSQLiteAdapterCommitDeclareEntityReplaysAndRejectsIdempotencyConflict(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	request := fixture.request(t, 0, fixture.environment.Ref(), "declare:authorization", candidate)
	first, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("first CommitDeclareEntity: %v", err)
	}

	replay, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("replay CommitDeclareEntity: %v", err)
	}
	if replay.Disposition() != CommitReplay {
		t.Fatalf("replay disposition = %q; want %q", replay.Disposition(), CommitReplay)
	}
	if replay.EventRef() != first.EventRef() ||
		replay.CommitRef() != first.CommitRef() ||
		replay.GraphRevision() != first.GraphRevision() ||
		replay.ResultDigest() != first.ResultDigest() {
		t.Fatal("replay did not return the exact original durable receipt")
	}

	differentProvenance, err := typedmemory.NewProvenanceRef(
		"memory:test:different-commit-request",
	)
	if err != nil {
		t.Fatalf("NewProvenanceRef(different request): %v", err)
	}
	provenanceConflict := request
	provenanceConflict.requestProvenance = differentProvenance
	_, err = fixture.adapter.commitDeclareEntity(context.Background(), provenanceConflict)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf(
			"request-provenance replay error = %v; want ErrIdempotencyConflict",
			err,
		)
	}

	conflictingCandidate := fixture.declaration(
		t,
		"authorization-service",
		"Authorization service with altered semantics",
	)
	conflictingRequest := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:authorization",
		conflictingCandidate,
	)
	_, err = fixture.adapter.commitDeclareEntity(context.Background(), conflictingRequest)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting replay error = %v; want ErrIdempotencyConflict", err)
	}
	assertTypedMemoryRowCounts(t, fixture.database, committedDeclarationCounts())
}

func TestSQLiteAdapterDeclareReplayClassifiesCorruptV46ProvenanceAsIntegrity(
	t *testing.T,
) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:corrupt-request-provenance",
		candidate,
	)
	receipt, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("seed CommitDeclareEntity: %v", err)
	}
	if _, err := fixture.database.Exec(
		"DROP TRIGGER typed_memory_graph_events_no_update",
	); err != nil {
		t.Fatalf("allow graph-event corruption fixture: %v", err)
	}
	corrupted, err := typedmemory.NewProvenanceRef("memory:test:corrupted-declare-request")
	if err != nil {
		t.Fatalf("NewProvenanceRef(corrupted DeclareEntity request): %v", err)
	}
	result, err := fixture.database.Exec(
		`UPDATE typed_memory_graph_events
		SET request_provenance_ref = ?
		WHERE project_id = ? AND event_ref = ?`,
		corrupted.String(),
		fixture.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("inject stored DeclareEntity provenance corruption: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "stored DeclareEntity event provenance")

	_, err = fixture.adapter.commitDeclareEntity(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("corrupt DeclareEntity provenance error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("stored DeclareEntity corruption was misclassified as caller conflict: %v", err)
	}
}

func TestSQLiteAdapterReplayRejectsAReinterpretedTransactionBasis(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	request := fixture.request(t, 0, fixture.environment.Ref(), "declare:authorization", candidate)
	_, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("first CommitDeclareEntity: %v", err)
	}

	alteredBasis := mustTypeEnvRef(t, []byte("not-the-original-typeenv"))
	retry := fixture.request(t, 1, alteredBasis, "declare:authorization", candidate)
	_, err = fixture.adapter.commitDeclareEntity(context.Background(), retry)
	if !errors.Is(err, ErrAdmissionBasisMismatch) {
		t.Fatalf("reinterpreted retry error = %v; want ErrAdmissionBasisMismatch", err)
	}
	assertTypedMemoryRowCounts(t, fixture.database, committedDeclarationCounts())
}

func TestSQLiteAdapterRejectsStaleRevisionAndWrongActiveTypeEnvWithoutWrites(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	firstCandidate := fixture.declaration(t, "authorization-service", "Authorization service")
	firstRequest := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:authorization",
		firstCandidate,
	)
	if _, err := fixture.adapter.commitDeclareEntity(context.Background(), firstRequest); err != nil {
		t.Fatalf("seed CommitDeclareEntity: %v", err)
	}

	staleCandidate := fixture.declaration(t, "billing-service", "Billing service")
	staleRequest := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:billing:stale",
		staleCandidate,
	)
	_, err := fixture.adapter.commitDeclareEntity(context.Background(), staleRequest)
	if !errors.Is(err, ErrStaleGraphRevision) {
		t.Fatalf("stale commit error = %v; want ErrStaleGraphRevision", err)
	}

	wrongTypeEnv := mustTypeEnvRef(t, []byte("wrong-active-typeenv"))
	wrongEnvironment := minimalTypeEnvAtRef(t, fixture.environment, wrongTypeEnv)
	wrongRequest := fixture.request(
		t,
		1,
		wrongTypeEnv,
		"declare:billing:wrong-typeenv",
		staleCandidate,
	)
	wrongRequest.admissionBatch = sealGenericDeclarationForTypeEnv(
		t,
		fixture,
		wrongEnvironment,
		fixture.registry,
		staleCandidate,
		1,
	)
	_, err = fixture.adapter.commitDeclareEntity(context.Background(), wrongRequest)
	if !errors.Is(err, ErrActiveTypeEnvMismatch) {
		t.Fatalf("wrong-TypeEnv commit error = %v; want ErrActiveTypeEnvMismatch", err)
	}
	assertTypedMemoryRowCounts(t, fixture.database, committedDeclarationCounts())
}

func minimalTypeEnvAtRef(
	t *testing.T,
	original typedmemory.TypeEnv,
	ref typedmemory.TypeEnvRef,
) typedmemory.TypeEnv {
	t.Helper()
	builder := typedmemory.NewTypeEnvBuilder(ref).
		SetSourceRevision(original.SourceRevision()).
		SetCompilerSchemaVersion(original.CompilerSchemaVersion()).
		SetCoverageManifest(original.CoverageManifest())
	for _, boundedContext := range original.BoundedContexts() {
		builder.AddBoundedContext(boundedContext)
	}
	environment, err := builder.Build()
	if err != nil {
		t.Fatalf("build minimal TypeEnv at alternate ref: %v", err)
	}
	return environment
}

func TestSQLiteAdapterTwoConnectionsCASOneExpectedRevisionZeroCommit(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	secondStore := openStoreAt(t, fixture.databasePath)
	secondAdapter := fixture.adapterFor(t, secondStore.GetRawDB())
	firstCandidate := fixture.declaration(t, "authorization-service", "Authorization service")
	secondCandidate := fixture.declaration(t, "billing-service", "Billing service")
	firstRequest := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:authorization",
		firstCandidate,
	)
	secondRequest := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:billing",
		secondCandidate,
	)

	start := make(chan struct{})
	results := make(chan commitAttempt, 2)
	go commitAfterStart(start, results, fixture.adapter, firstRequest)
	go commitAfterStart(start, results, secondAdapter, secondRequest)
	close(start)

	applied := 0
	stale := 0
	for index := 0; index < 2; index++ {
		attempt := <-results
		if attempt.err == nil && attempt.receipt.Disposition() == CommitApplied {
			applied++
			continue
		}
		if errors.Is(attempt.err, ErrStaleGraphRevision) {
			stale++
			continue
		}
		t.Fatalf("unexpected concurrent commit result: receipt=%+v error=%v", attempt.receipt, attempt.err)
	}
	if applied != 1 || stale != 1 {
		t.Fatalf("concurrent results = %d applied, %d stale; want 1/1", applied, stale)
	}
	assertTypedMemoryRowCounts(t, fixture.database, committedDeclarationCounts())
}

func TestSQLiteAdapterProjectionFailureRollsBackEverySemanticRowAndHead(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	_, err := fixture.database.Exec(`CREATE TRIGGER test_projection_job_failure
		BEFORE INSERT ON typed_memory_projection_jobs BEGIN
			SELECT RAISE(ABORT, 'injected projection job failure');
		END`)
	if err != nil {
		t.Fatalf("install projection failure trigger: %v", err)
	}
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	request := fixture.request(t, 0, fixture.environment.Ref(), "declare:authorization", candidate)

	if _, err := fixture.adapter.commitDeclareEntity(context.Background(), request); err == nil {
		t.Fatal("CommitDeclareEntity succeeded despite injected projection failure")
	}
	assertTypedMemoryRowCounts(t, fixture.database, emptyDeclarationCounts())
	head, err := fixture.adapter.LoadHead(context.Background(), fixture.project)
	if err != nil {
		t.Fatalf("LoadHead after rollback: %v", err)
	}
	if head.Revision().Value() != 0 || head.LastEventRef() != "" || head.LastCommitRef() != "" {
		t.Fatalf(
			"head after rollback = revision %d event %q commit %q; want genesis",
			head.Revision().Value(),
			head.LastEventRef(),
			head.LastCommitRef(),
		)
	}
}

func TestCommitRequestRequiresExplicitSQLiteSafeExpectedRevision(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	key := mustIdempotencyKey(t, "declare:authorization")

	_, err := NewCommitRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(mustRequestProvenanceRef(t)).
		SetCandidate(candidate).
		Build()
	if err == nil || !strings.Contains(err.Error(), "must be explicit") {
		t.Fatalf("omitted revision error = %v; want explicit-revision rejection", err)
	}

	_, err = NewCommitRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(uint64(math.MaxInt64))).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(mustRequestProvenanceRef(t)).
		SetCandidate(candidate).
		Build()
	if !errors.Is(err, ErrRevisionOverflow) {
		t.Fatalf("overflow revision error = %v; want ErrRevisionOverflow", err)
	}

	_, err = NewCommitRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(0)).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetCandidate(candidate).
		Build()
	if err == nil || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("omitted request provenance error = %v; want explicit-provenance rejection", err)
	}

	if _, err := NewCommitRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(0)).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(mustRequestProvenanceRef(t)).
		SetCandidate(candidate).
		Build(); err != nil {
		t.Fatalf("explicit revision zero was rejected: %v", err)
	}
}

func TestReplayRequestRequiresCompleteExactReadIdentity(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	key := mustIdempotencyKey(t, "declare:authorization")
	provenance := mustRequestProvenanceRef(t)

	_, err := (*ReplayRequestBuilder)(nil).Build()
	if err == nil || !strings.Contains(err.Error(), "must be explicit") {
		t.Fatalf("nil builder error = %v; want explicit-revision rejection", err)
	}

	_, err = NewReplayRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(provenance).
		SetCandidate(candidate).
		Build()
	if err == nil || !strings.Contains(err.Error(), "must be explicit") {
		t.Fatalf("omitted revision error = %v; want explicit-revision rejection", err)
	}

	_, err = NewReplayRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(uint64(math.MaxInt64))).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(provenance).
		SetCandidate(candidate).
		Build()
	if !errors.Is(err, ErrRevisionOverflow) {
		t.Fatalf("overflow revision error = %v; want ErrRevisionOverflow", err)
	}

	_, err = NewReplayRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(0)).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetCandidate(candidate).
		Build()
	if err == nil || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("omitted provenance error = %v; want exact-identity rejection", err)
	}

	_, err = NewReplayRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(0)).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(provenance).
		Build()
	if err == nil || !strings.Contains(err.Error(), "candidate") {
		t.Fatalf("omitted candidate error = %v; want candidate rejection", err)
	}

	if _, err := NewReplayRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(0)).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(provenance).
		SetCandidate(candidate).
		Build(); err != nil {
		t.Fatalf("complete exact replay identity was rejected: %v", err)
	}
}

func TestSQLiteAdapterTypeEnvSnapshotIsImmutableAndConflictsFailClosed(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	differentFormat, err := NewSnapshotFormat("base-typeenv-artifact-payload.test-conflict")
	if err != nil {
		t.Fatalf("NewSnapshotFormat: %v", err)
	}
	conflict, err := NewTypeEnvSnapshotBuilder(fixture.snapshot.Ref()).
		SetFormat(differentFormat).
		SetCanonicalBytes(fixture.snapshot.CanonicalBytes()).
		SetSourceRevision(fixture.snapshot.SourceRevision()).
		SetCompilerSchemaVersion(fixture.snapshot.CompilerSchemaVersion()).
		Build()
	if err != nil {
		t.Fatalf("build conflicting snapshot: %v", err)
	}
	if err := fixture.adapter.PutTypeEnvSnapshot(context.Background(), conflict); !errors.Is(err, ErrSnapshotConflict) {
		t.Fatalf("snapshot conflict error = %v; want ErrSnapshotConflict", err)
	}

	_, err = fixture.database.Exec(
		`UPDATE typed_memory_type_env_snapshots SET source_revision = ? WHERE type_env_ref = ?`,
		"tampered-revision",
		fixture.snapshot.Ref().String(),
	)
	if err == nil {
		t.Fatal("immutable TypeEnv snapshot accepted direct corruption")
	}
	loaded, err := fixture.adapter.LoadTypeEnvSnapshot(
		context.Background(),
		fixture.snapshot.Ref(),
	)
	if err != nil {
		t.Fatalf("LoadTypeEnvSnapshot: %v", err)
	}
	if !equalTypeEnvSnapshots(loaded, fixture.snapshot) {
		t.Fatal("failed mutation changed the immutable TypeEnv snapshot")
	}
	assertTypedMemoryRowCounts(t, fixture.database, map[string]int64{
		"typed_memory_type_env_snapshots": 1,
	})
}

type sqliteStoreFixture struct {
	databasePath string
	store        *db.Store
	database     *sql.DB
	project      projectledger.ProjectID
	environment  typedmemory.TypeEnv
	registry     typedmemory.CodecRegistry
	snapshot     TypeEnvSnapshot
	context      typedmemory.BoundedContextRef
	clock        fixedClock
	adapter      *SQLiteAdapter
}

func newSQLiteStoreFixture(t *testing.T) sqliteStoreFixture {
	t.Helper()
	databasePath := filepath.Join(t.TempDir(), "typed-memory.db")
	store := openStoreAt(t, databasePath)
	database := store.GetRawDB()
	project := mustProjectID(t, "qnt_a7f3b2c1")
	insertProjectBinding(t, database, project, filepath.Dir(databasePath))
	canonicalBytes := []byte(`{"schema":"test.typeenv.snapshot/v1","context":"ctx:test"}`)
	typeEnvRef := mustTypeEnvRef(t, canonicalBytes)
	sourceRevision := mustSourceRevision(t, "test-fpf-revision")
	compilerVersion := mustCompilerVersion(t, "test.compiler.v1")
	contextRef := mustContextRef(t, "ctx:test")
	provenance := mustFPFProvenance(t, sourceRevision)
	contextValue, err := typedmemory.NewBoundedContext(contextRef, provenance)
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
	environment, err := typedmemory.NewTypeEnvBuilder(typeEnvRef).
		SetSourceRevision(sourceRevision).
		SetCompilerSchemaVersion(compilerVersion).
		SetCoverageManifest(coverage).
		AddBoundedContext(contextValue).
		Build()
	if err != nil {
		t.Fatalf("build minimal TypeEnv: %v", err)
	}
	format, err := NewSnapshotFormat(BaseTypeEnvSnapshotFormat)
	if err != nil {
		t.Fatalf("NewSnapshotFormat: %v", err)
	}
	snapshot, err := NewTypeEnvSnapshotBuilder(typeEnvRef).
		SetFormat(format).
		SetCanonicalBytes(canonicalBytes).
		SetSourceRevision(sourceRevision).
		SetCompilerSchemaVersion(compilerVersion).
		Build()
	if err != nil {
		t.Fatalf("build TypeEnv snapshot: %v", err)
	}
	registry := typedmemory.NewCodecRegistry()
	clock := fixedClock{value: time.Date(2026, 7, 16, 8, 30, 0, 123456789, time.UTC)}
	loader := staticTypeEnvLoader{
		reference:   typeEnvRef,
		environment: environment,
		registry:    registry,
	}
	adapter, err := NewGenericSQLiteAdapter(
		database,
		loader,
		clock,
		unexpectedMemberOfEngine{},
		unexpectedReferenceEngine{},
		unexpectedObservableProvider{},
	)
	if err != nil {
		t.Fatalf("NewSQLiteAdapter: %v", err)
	}
	if err := adapter.PutTypeEnvSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("PutTypeEnvSnapshot: %v", err)
	}
	seedGraphHead(t, database, project, typeEnvRef, clock.Now())
	return sqliteStoreFixture{
		databasePath: databasePath,
		store:        store,
		database:     database,
		project:      project,
		environment:  environment,
		registry:     registry,
		snapshot:     snapshot,
		context:      contextRef,
		clock:        clock,
		adapter:      adapter,
	}
}

func (fixture sqliteStoreFixture) adapterFor(t *testing.T, database *sql.DB) *SQLiteAdapter {
	t.Helper()
	loader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    fixture.registry,
	}
	adapter, err := NewGenericSQLiteAdapter(
		database,
		loader,
		fixture.clock,
		unexpectedMemberOfEngine{},
		unexpectedReferenceEngine{},
		unexpectedObservableProvider{},
	)
	if err != nil {
		t.Fatalf("NewSQLiteAdapter: %v", err)
	}
	return adapter
}

func (fixture sqliteStoreFixture) declaration(
	t *testing.T,
	suffix string,
	labelText string,
) typedmemory.MemoryChangeSet {
	t.Helper()
	entity := mustEntityID(t, "entity:"+suffix)
	localRef, err := typedmemory.NewBatchLocalRef("local:" + suffix)
	if err != nil {
		t.Fatalf("NewBatchLocalRef: %v", err)
	}
	label, err := typedmemory.NewEntityLabel(labelText)
	if err != nil {
		t.Fatalf("NewEntityLabel: %v", err)
	}
	provenance, err := typedmemory.NewProvenanceRef("memory:test:" + suffix)
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	declaration, err := typedmemory.NewDeclareEntity(
		entity,
		localRef,
		fixture.context,
		label,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity: %v", err)
	}
	changeSet, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{declaration})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}
	return changeSet
}

func (fixture sqliteStoreFixture) request(
	t *testing.T,
	revision uint64,
	typeEnv typedmemory.TypeEnvRef,
	keyText string,
	candidate typedmemory.MemoryChangeSet,
) CommitRequest {
	t.Helper()
	request, err := NewCommitRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(revision)).
		SetExpectedTypeEnv(typeEnv).
		SetIdempotencyKey(mustIdempotencyKey(t, keyText)).
		SetRequestProvenance(mustRequestProvenanceRef(t)).
		SetCandidate(candidate).
		Build()
	if err != nil {
		t.Fatalf("build CommitRequest: %v", err)
	}
	changes := candidate.Changes()
	if len(changes) == 1 {
		if _, declaration := changes[0].(typedmemory.DeclareEntity); declaration {
			request.admissionBatch = sealGenericDeclaration(
				t,
				fixture,
				candidate,
				revision,
			)
		}
	}
	return request
}

func mustRequestProvenanceRef(t *testing.T) typedmemory.ProvenanceRef {
	t.Helper()
	provenance, err := typedmemory.NewProvenanceRef("memory:test:commit-request")
	if err != nil {
		t.Fatalf("NewProvenanceRef(commit request): %v", err)
	}
	return provenance
}

type staticTypeEnvLoader struct {
	reference   typedmemory.TypeEnvRef
	environment typedmemory.TypeEnv
	registry    typedmemory.CodecRegistry
}

func (loader staticTypeEnvLoader) LoadTypeEnv(
	snapshot TypeEnvSnapshot,
) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error) {
	if snapshot.Ref() != loader.reference {
		return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, fmt.Errorf(
			"unexpected test TypeEnv snapshot %s",
			snapshot.Ref().String(),
		)
	}
	return loader.environment, loader.registry, nil
}

type fixedClock struct {
	value time.Time
}

func (clock fixedClock) Now() time.Time { return clock.value }

type commitAttempt struct {
	receipt CommitReceipt
	err     error
}

func commitAfterStart(
	start <-chan struct{},
	results chan<- commitAttempt,
	adapter *SQLiteAdapter,
	request CommitRequest,
) {
	<-start
	receipt, err := adapter.commitDeclareEntity(context.Background(), request)
	results <- commitAttempt{receipt: receipt, err: err}
}

func assertCommittedDeclaration(
	t *testing.T,
	adapter *SQLiteAdapter,
	fixture sqliteStoreFixture,
	candidate typedmemory.MemoryChangeSet,
	receipt CommitReceipt,
) {
	t.Helper()
	head, err := adapter.LoadHead(context.Background(), fixture.project)
	if err != nil {
		t.Fatalf("LoadHead: %v", err)
	}
	if head.Revision() != receipt.GraphRevision() ||
		head.ActiveTypeEnv() != fixture.environment.Ref() ||
		head.LastEventRef() != receipt.EventRef() ||
		head.LastCommitRef() != receipt.CommitRef() {
		t.Fatal("graph head does not name the exact committed event, revision, and TypeEnv")
	}
	declaration := candidate.Changes()[0].(typedmemory.DeclareEntity)
	stored, err := adapter.LoadEntity(context.Background(), fixture.project, declaration.Entity())
	if err != nil {
		t.Fatalf("LoadEntity: %v", err)
	}
	if stored.Entity() != declaration.Entity() ||
		stored.Context() != declaration.Context() ||
		stored.Label() != declaration.Label() ||
		stored.Provenance() != declaration.Provenance() ||
		stored.EventRef() != receipt.EventRef() ||
		stored.Revision() != receipt.GraphRevision() {
		t.Fatal("stored entity does not match its exact admitted declaration and event")
	}
}

func assertTypedMemoryRowCounts(
	t *testing.T,
	database *sql.DB,
	wanted map[string]int64,
) {
	t.Helper()
	for table, expected := range wanted {
		var actual int64
		query := "SELECT COUNT(*) FROM " + table
		if err := database.QueryRow(query).Scan(&actual); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if actual != expected {
			t.Fatalf("%s row count = %d; want %d", table, actual, expected)
		}
	}
}

func committedDeclarationCounts() map[string]int64 {
	return map[string]int64{
		"typed_memory_graph_events":           1,
		"typed_memory_graph_commits":          1,
		"typed_memory_entities":               1,
		"typed_memory_entity_contexts":        1,
		"typed_memory_idempotency_history":    1,
		"typed_memory_projection_jobs":        1,
		"typed_memory_projection_debt_events": 0,
	}
}

func emptyDeclarationCounts() map[string]int64 {
	return map[string]int64{
		"typed_memory_graph_events":           0,
		"typed_memory_graph_commits":          0,
		"typed_memory_entities":               0,
		"typed_memory_entity_contexts":        0,
		"typed_memory_idempotency_history":    0,
		"typed_memory_projection_jobs":        0,
		"typed_memory_projection_debt_events": 0,
	}
}

func openStoreAt(t *testing.T, databasePath string) *db.Store {
	t.Helper()
	store, err := kerneldbfixture.OpenCurrentStore(databasePath)
	if err != nil {
		t.Fatalf("db.NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func insertProjectBinding(
	t *testing.T,
	database *sql.DB,
	project projectledger.ProjectID,
	projectRoot string,
) {
	t.Helper()
	physicalRoot, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		t.Fatalf("resolve physical project binding root: %v", err)
	}
	projectRoot = physicalRoot
	boundAt := "2026-07-16T08:00:00Z"
	bindingJSON := fmt.Sprintf(
		`{"schema":"haft.project-ledger-binding/v1","project_id":%q,"project_root":%q,"bound_at":%q}`,
		project.String(),
		projectRoot,
		boundAt,
	)
	bindingSum := sha256.Sum256([]byte(bindingJSON))
	bindingDigest := "sha256:" + hex.EncodeToString(bindingSum[:])
	_, err = database.Exec(
		`INSERT INTO project_ledger_binding (
			binding_slot, project_id, project_root, binding_digest, binding_json, bound_at
		) VALUES (1, ?, ?, ?, ?, ?)`,
		project.String(),
		projectRoot,
		bindingDigest,
		bindingJSON,
		boundAt,
	)
	if err != nil {
		t.Fatalf("insert project ledger binding: %v", err)
	}
}

func seedGraphHead(
	t *testing.T,
	database *sql.DB,
	project projectledger.ProjectID,
	typeEnv typedmemory.TypeEnvRef,
	recordedAt time.Time,
) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO typed_memory_graph_heads (
			project_id, graph_revision, active_type_env_ref,
			last_event_ref, last_commit_ref, updated_at
		) VALUES (?, 0, ?, '', '', ?)`,
		project.String(),
		typeEnv.String(),
		canonicalTime(recordedAt),
	)
	if err != nil {
		t.Fatalf("seed typed-memory graph head: %v", err)
	}
}

func mustProjectID(t *testing.T, raw string) projectledger.ProjectID {
	t.Helper()
	value, err := projectledger.ParseProjectID(raw)
	if err != nil {
		t.Fatalf("ParseProjectID: %v", err)
	}
	return value
}

func mustTypeEnvRef(t *testing.T, canonicalBytes []byte) typedmemory.TypeEnvRef {
	t.Helper()
	sum := sha256.Sum256(canonicalBytes)
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	value, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	return value
}

func mustSourceRevision(t *testing.T, raw string) typedmemory.SourceRevision {
	t.Helper()
	value, err := typedmemory.NewSourceRevision(raw)
	if err != nil {
		t.Fatalf("NewSourceRevision: %v", err)
	}
	return value
}

func mustCompilerVersion(t *testing.T, raw string) typedmemory.CompilerSchemaVersion {
	t.Helper()
	value, err := typedmemory.NewCompilerSchemaVersion(raw)
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion: %v", err)
	}
	return value
}

func mustContextRef(t *testing.T, raw string) typedmemory.BoundedContextRef {
	t.Helper()
	value, err := typedmemory.NewBoundedContextRef(raw)
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}
	return value
}

func mustFPFProvenance(
	t *testing.T,
	revision typedmemory.SourceRevision,
) typedmemory.FPFSourceProvenance {
	t.Helper()
	unit, err := typedmemory.NewSourceUnitID("spec:test-typeenv")
	if err != nil {
		t.Fatalf("NewSourceUnitID: %v", err)
	}
	contentDigest := mustDigest(t, []byte("test TypeEnv source unit"))
	lineRange, err := typedmemory.NewSourceLineRange(1, 1)
	if err != nil {
		t.Fatalf("NewSourceLineRange: %v", err)
	}
	location, err := typedmemory.NewUnpatternedSourceLocation(
		unit,
		revision,
		contentDigest,
		lineRange,
	)
	if err != nil {
		t.Fatalf("NewUnpatternedSourceLocation: %v", err)
	}
	reference, err := typedmemory.NewProvenanceRef("prov:fpf:test-typeenv")
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	rule, err := typedmemory.NewCompilerRuleID("test.typeenv.context.v1")
	if err != nil {
		t.Fatalf("NewCompilerRuleID: %v", err)
	}
	provenance, err := typedmemory.NewFPFSourceProvenance(reference, location, rule)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance: %v", err)
	}
	return provenance
}

func mustDigest(t *testing.T, value []byte) typedmemory.SHA256Digest {
	t.Helper()
	sum := sha256.Sum256(value)
	digest, err := typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	return digest
}

func mustEntityID(t *testing.T, raw string) typedmemory.EntityID {
	t.Helper()
	value, err := typedmemory.NewEntityID(raw)
	if err != nil {
		t.Fatalf("NewEntityID: %v", err)
	}
	return value
}

func mustIdempotencyKey(t *testing.T, raw string) IdempotencyKey {
	t.Helper()
	value, err := NewIdempotencyKey(raw)
	if err != nil {
		t.Fatalf("NewIdempotencyKey: %v", err)
	}
	return value
}
