package typedmemorystore

import (
	"context"
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestSQLiteAdapterSemanticWriteFailuresAreAtomic(t *testing.T) {
	cases := []struct {
		name       string
		triggerSQL string
	}{
		{
			name: "graph event",
			triggerSQL: `CREATE TRIGGER test_semantic_write_failure
				BEFORE INSERT ON typed_memory_graph_events BEGIN
					SELECT RAISE(ABORT, 'injected graph event failure');
				END`,
		},
		{
			name: "entity",
			triggerSQL: `CREATE TRIGGER test_semantic_write_failure
				BEFORE INSERT ON typed_memory_entities BEGIN
					SELECT RAISE(ABORT, 'injected entity failure');
				END`,
		},
		{
			name: "entity context",
			triggerSQL: `CREATE TRIGGER test_semantic_write_failure
				BEFORE INSERT ON typed_memory_entity_contexts BEGIN
					SELECT RAISE(ABORT, 'injected entity context failure');
				END`,
		},
		{
			name: "idempotency history",
			triggerSQL: `CREATE TRIGGER test_semantic_write_failure
				BEFORE INSERT ON typed_memory_idempotency_history BEGIN
					SELECT RAISE(ABORT, 'injected idempotency failure');
				END`,
		},
		{
			name: "projection outbox",
			triggerSQL: `CREATE TRIGGER test_semantic_write_failure
				BEFORE INSERT ON typed_memory_projection_jobs BEGIN
					SELECT RAISE(ABORT, 'injected projection outbox failure');
				END`,
		},
		{
			name: "graph commit closure",
			triggerSQL: `CREATE TRIGGER test_semantic_write_failure
				BEFORE INSERT ON typed_memory_graph_commits BEGIN
					SELECT RAISE(ABORT, 'injected graph commit failure');
				END`,
		},
		{
			name: "graph head finalization",
			triggerSQL: `CREATE TRIGGER test_semantic_write_failure
				BEFORE UPDATE ON typed_memory_graph_heads BEGIN
					SELECT RAISE(ABORT, 'injected graph head finalization failure');
				END`,
		},
	}

	for _, current := range cases {
		current := current
		t.Run(current.name, func(t *testing.T) {
			fixture := newSQLiteStoreFixture(t)
			_, err := fixture.database.Exec(current.triggerSQL)
			if err != nil {
				t.Fatalf("install failure trigger: %v", err)
			}
			candidate := fixture.declaration(
				t,
				"authorization-service",
				"Authorization service",
			)
			request := fixture.request(
				t,
				0,
				fixture.environment.Ref(),
				"declare:authorization",
				candidate,
			)

			_, err = fixture.adapter.commitDeclareEntity(context.Background(), request)
			if err == nil {
				t.Fatal("CommitDeclareEntity succeeded despite the injected write failure")
			}
			assertTypedMemoryRowCounts(t, fixture.database, emptyDeclarationCounts())
			assertTypedMemoryRowCounts(t, fixture.database, map[string]int64{
				"typed_memory_type_env_snapshots": 1,
				"typed_memory_graph_heads":        1,
			})
			assertAtomicityGenesisHead(t, fixture)
		})
	}
}

func TestSQLiteAdapterCommitThenReportedFailureRecoversExactDurableReceipt(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	fixture.adapter.finisher = commitThenReportFailureFinisher{
		reported: errors.New("synthetic lost COMMIT acknowledgement"),
	}
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:authorization",
		candidate,
	)

	recovered, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("recover committed declaration after reported failure: %v", err)
	}
	if recovered.Disposition() != CommitRecovered {
		t.Fatalf(
			"recovered disposition = %q; want %q",
			recovered.Disposition(),
			CommitRecovered,
		)
	}
	assertCommittedDeclaration(t, fixture.adapter, fixture, candidate, recovered)
	assertTypedMemoryRowCounts(t, fixture.database, committedDeclarationCounts())

	replay, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("replay after recovered commit: %v", err)
	}
	if replay.Disposition() != CommitReplay ||
		replay.EventRef() != recovered.EventRef() ||
		replay.CommitRef() != recovered.CommitRef() ||
		replay.ResultDigest() != recovered.ResultDigest() {
		t.Fatal("replay did not recover the same exact durable result")
	}
	assertTypedMemoryRowCounts(t, fixture.database, committedDeclarationCounts())
}

func TestSQLiteAdapterRollbackThenReportedCommitFailureIsUnknownAndAtomic(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	fixture.adapter.finisher = rollbackThenReportFailureFinisher{
		reported: errors.New("synthetic COMMIT delivery failure before commit"),
	}
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:authorization",
		candidate,
	)

	_, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if !errors.Is(err, ErrCommitOutcomeUnknown) {
		t.Fatalf("commit error = %v; want ErrCommitOutcomeUnknown", err)
	}
	assertTypedMemoryRowCounts(t, fixture.database, emptyDeclarationCounts())
	assertAtomicityGenesisHead(t, fixture)
}

func TestSQLiteAdapterRejectsMalformedSelfHashedTypeEnvBeforePersistence(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	malformedCanonicalBytes := []byte(`{"schema":"base-typeenv-artifact-payload.v1","contexts":`)
	malformedRef := mustTypeEnvRef(t, malformedCanonicalBytes)
	malformed, err := NewTypeEnvSnapshotBuilder(malformedRef).
		SetFormat(fixture.snapshot.Format()).
		SetCanonicalBytes(malformedCanonicalBytes).
		SetSourceRevision(fixture.snapshot.SourceRevision()).
		SetCompilerSchemaVersion(fixture.snapshot.CompilerSchemaVersion()).
		Build()
	if err != nil {
		t.Fatalf("build self-hashed malformed snapshot carrier: %v", err)
	}
	if malformed.Ref().String() != "typeenv:"+malformed.ArtifactDigest().String() {
		t.Fatal("malformed fixture is not self-hashed and cannot exercise loader rejection")
	}
	loaderErr := errors.New("malformed executable TypeEnv payload")
	rejectingAdapter, err := newDeclarationSQLiteAdapter(
		fixture.database,
		rejectingTypeEnvLoader{err: loaderErr},
		fixture.clock,
	)
	if err != nil {
		t.Fatalf("NewSQLiteAdapter: %v", err)
	}

	err = rejectingAdapter.PutTypeEnvSnapshot(context.Background(), malformed)
	if !errors.Is(err, loaderErr) {
		t.Fatalf("PutTypeEnvSnapshot error = %v; want loader rejection", err)
	}
	assertTypedMemoryRowCounts(t, fixture.database, map[string]int64{
		"typed_memory_type_env_snapshots": 1,
	})
}

func TestSQLiteAdapterReopenRejectsCorruptDurableDeclaration(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "authorization-service", "Authorization service")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"declare:authorization",
		candidate,
	)
	receipt, err := fixture.adapter.commitDeclareEntity(context.Background(), request)
	if err != nil {
		t.Fatalf("seed durable declaration: %v", err)
	}
	_, err = fixture.database.Exec(`DROP TRIGGER typed_memory_graph_events_no_update`)
	if err != nil {
		t.Fatalf("disable immutable-event trigger for corruption fixture: %v", err)
	}
	result, err := fixture.database.Exec(
		`UPDATE typed_memory_graph_events
		SET canonical_change_set_bytes = ?
		WHERE project_id = ? AND event_ref = ?`,
		[]byte("corrupt but non-empty canonical change set"),
		fixture.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("install durable corruption fixture: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("inspect durable corruption fixture: %v", err)
	}
	if rows != 1 {
		t.Fatalf("corrupted rows = %d; want 1", rows)
	}
	if err := fixture.store.Close(); err != nil {
		t.Fatalf("close corrupted store: %v", err)
	}
	reopenedStore := openStoreAt(t, fixture.databasePath)
	reopenedAdapter := fixture.adapterFor(t, reopenedStore.GetRawDB())

	_, err = reopenedAdapter.commitDeclareEntity(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("replay error = %v; want durable corruption rejection", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("durable corruption was misclassified as caller conflict: %v", err)
	}
	assertTypedMemoryRowCounts(
		t,
		reopenedStore.GetRawDB(),
		committedDeclarationCounts(),
	)
}

type syntheticFinishEvidence struct {
	err error
}

func (evidence syntheticFinishEvidence) Succeeded() bool { return false }

func (evidence syntheticFinishEvidence) Err() error { return evidence.err }

type commitThenReportFailureFinisher struct {
	reported error
}

func (finisher commitThenReportFailureFinisher) Commit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) transactionFinishEvidence {
	actual := transaction.Commit(ctx)
	if !actual.Succeeded() {
		return actual
	}
	return syntheticFinishEvidence{err: finisher.reported}
}

type rollbackThenReportFailureFinisher struct {
	reported error
}

func (finisher rollbackThenReportFailureFinisher) Commit(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
) transactionFinishEvidence {
	actual := transaction.Rollback(ctx)
	if !actual.Succeeded() {
		return actual
	}
	return syntheticFinishEvidence{err: finisher.reported}
}

type rejectingTypeEnvLoader struct {
	err error
}

func (loader rejectingTypeEnvLoader) LoadTypeEnv(
	TypeEnvSnapshot,
) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error) {
	return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, loader.err
}

func assertAtomicityGenesisHead(t *testing.T, fixture sqliteStoreFixture) {
	t.Helper()
	head, err := fixture.adapter.LoadHead(context.Background(), fixture.project)
	if err != nil {
		t.Fatalf("LoadHead after failed commit: %v", err)
	}
	if head.Revision().Value() != 0 ||
		head.ActiveTypeEnv() != fixture.environment.Ref() ||
		head.LastEventRef() != "" ||
		head.LastCommitRef() != "" {
		t.Fatalf(
			"head after failed commit = revision %d, TypeEnv %q, event %q, commit %q; want unchanged genesis",
			head.Revision().Value(),
			head.ActiveTypeEnv().String(),
			head.LastEventRef(),
			head.LastCommitRef(),
		)
	}
}
