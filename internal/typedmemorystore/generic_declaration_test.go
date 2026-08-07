package typedmemorystore

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestGenericCommitDeclareEntityPersistsExactV46AdmissionAndReplays(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(t, "generic", "Generic entity")
	request := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"generic-declaration",
		candidate,
	)
	request.admissionBatch = sealGenericDeclaration(t, fixture, candidate, 0)
	adapter := newGenericFixtureAdapter(t, fixture)

	receipt, err := adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	if receipt.Disposition() != CommitApplied || receipt.GraphRevision().Value() != 1 {
		t.Fatalf("receipt = %#v; want applied revision 1", receipt)
	}
	replay, err := adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("replay CommitMemoryChangeSet: %v", err)
	}
	if replay.Disposition() != CommitReplay ||
		replay.EventRef() != receipt.EventRef() ||
		replay.CommitRef() != receipt.CommitRef() ||
		replay.ResultDigest() != receipt.ResultDigest() {
		t.Fatalf("replay = %#v; want exact original receipt %#v", replay, receipt)
	}
	assertTypedMemoryRowCounts(t, fixture.database, map[string]int64{
		"typed_memory_graph_events":                    1,
		"typed_memory_graph_commits":                   1,
		"typed_memory_event_admission_bases":           1,
		"typed_memory_commit_materialization_closures": 1,
		"typed_memory_entities":                        1,
		"typed_memory_entity_contexts":                 1,
		"typed_memory_idempotency_history":             1,
		"typed_memory_projection_jobs":                 1,
		"typed_memory_context_slices":                  0,
		"typed_memory_relation_instances":              0,
		"typed_memory_alias_changes":                   0,
		"typed_memory_assertion_retractions":           0,
		"typed_memory_relation_filler_memberof_uses":   0,
		"typed_memory_reference_resolution_uses":       0,
		"typed_memory_memberof_evaluations":            0,
		"typed_memory_memberof_observable_inputs":      0,
		"typed_memory_observable_input_blobs":          0,
	})
}

func TestGenericCommitRejectsDifferentExactReplayWithoutWrites(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	adapter := newGenericFixtureAdapter(t, fixture)
	firstCandidate := fixture.declaration(t, "first-generic", "First generic")
	first := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"generic-conflict",
		firstCandidate,
	)
	first.admissionBatch = sealGenericDeclaration(t, fixture, firstCandidate, 0)
	if _, err := adapter.CommitMemoryChangeSet(context.Background(), first); err != nil {
		t.Fatalf("seed generic admission: %v", err)
	}

	conflictingCandidate := fixture.declaration(t, "other-generic", "Other generic")
	conflicting := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"generic-conflict",
		conflictingCandidate,
	)
	conflicting.admissionBatch = sealGenericDeclaration(t, fixture, conflictingCandidate, 0)
	if _, err := adapter.CommitMemoryChangeSet(context.Background(), conflicting); err == nil {
		t.Fatal("different exact replay unexpectedly committed")
	} else if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict error = %v; want ErrIdempotencyConflict", err)
	}
	assertTypedMemoryRowCounts(t, fixture.database, map[string]int64{
		"typed_memory_graph_events":                    1,
		"typed_memory_graph_commits":                   1,
		"typed_memory_event_admission_bases":           1,
		"typed_memory_commit_materialization_closures": 1,
		"typed_memory_entities":                        1,
		"typed_memory_entity_contexts":                 1,
	})
}

func TestGenericCommitRejectsCallerSealedButFalseSnapshotWithoutWrites(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	adapter := newGenericFixtureAdapter(t, fixture)
	candidate := fixture.declaration(t, "false-snapshot", "False snapshot")
	seed := fixture.request(
		t,
		0,
		fixture.environment.Ref(),
		"false-snapshot-seed",
		candidate,
	)
	seed.admissionBatch = sealGenericDeclaration(t, fixture, candidate, 0)
	if _, err := adapter.CommitMemoryChangeSet(context.Background(), seed); err != nil {
		t.Fatalf("seed generic admission: %v", err)
	}

	falseRetry := fixture.request(
		t,
		1,
		fixture.environment.Ref(),
		"false-snapshot-attempt",
		candidate,
	)
	falseRetry.admissionBatch = sealGenericDeclaration(t, fixture, candidate, 1)
	if _, err := adapter.CommitMemoryChangeSet(context.Background(), falseRetry); err == nil {
		t.Fatal("caller-sealed false absence unexpectedly committed")
	} else if !errors.Is(err, ErrRevalidationRejected) &&
		!errors.Is(err, ErrAdmissionEnvelopeMismatch) {
		t.Fatalf("false snapshot error = %v; want transaction revalidation rejection", err)
	}
	assertTypedMemoryRowCounts(t, fixture.database, map[string]int64{
		"typed_memory_graph_events":                    1,
		"typed_memory_graph_commits":                   1,
		"typed_memory_event_admission_bases":           1,
		"typed_memory_commit_materialization_closures": 1,
		"typed_memory_entities":                        1,
		"typed_memory_entity_contexts":                 1,
		"typed_memory_idempotency_history":             1,
	})
}

func sealGenericDeclaration(
	t *testing.T,
	fixture sqliteStoreFixture,
	candidate typedmemory.MemoryChangeSet,
	revision uint64,
) typedmemory.AdmissionBatch {
	t.Helper()
	return sealGenericDeclarationForTypeEnv(
		t,
		fixture,
		fixture.environment,
		fixture.registry,
		candidate,
		revision,
	)
}

func sealGenericDeclarationForTypeEnv(
	t *testing.T,
	fixture sqliteStoreFixture,
	environment typedmemory.TypeEnv,
	registry typedmemory.CodecRegistry,
	candidate typedmemory.MemoryChangeSet,
	revision uint64,
) typedmemory.AdmissionBatch {
	t.Helper()
	change := candidate.Changes()[0].(typedmemory.DeclareEntity)
	basis, err := snapshotResolutionBasis(
		fixture.project,
		typedmemory.NewGraphRevision(revision),
	)
	if err != nil {
		t.Fatalf("snapshotResolutionBasis: %v", err)
	}
	absent, err := typedmemory.NewAbsentEntityResolution(
		change.Entity(),
		change.Context(),
		basis,
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	observation, err := typedmemory.NewEntityAbsentObservation(0, absent)
	if err != nil {
		t.Fatalf("NewEntityAbsentObservation: %v", err)
	}
	sealedBasis, err := typedmemory.NewSnapshotOnlyBasis(typedmemory.SnapshotOnlyBasisInput{
		TypeEnv:       environment.Ref(),
		GraphRevision: typedmemory.NewGraphRevision(revision),
		Observations:  []typedmemory.AdmissionSnapshotObservation{observation},
	})
	if err != nil {
		t.Fatalf("NewSnapshotOnlyBasis: %v", err)
	}
	snapshot, err := newTransactionAdmissionSnapshotWithClassifications(
		sealedBasis,
		[]typedmemory.AdmissionSnapshotObservation{observation},
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("newTransactionAdmissionSnapshot: %v", err)
	}
	verdict := typedmemory.ValidateMemoryChangeSet(
		environment,
		registry,
		snapshot,
		candidate,
	)
	valid, ok := verdict.(typedmemory.Valid)
	if !ok {
		t.Fatalf("ValidateMemoryChangeSet = %T; want Valid", verdict)
	}
	return valid.AdmissionBatch()
}

func newGenericFixtureAdapter(
	t *testing.T,
	fixture sqliteStoreFixture,
) *SQLiteAdapter {
	t.Helper()
	loader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    fixture.registry,
	}
	adapter, err := NewGenericSQLiteAdapter(
		fixture.database,
		loader,
		fixture.clock,
		unexpectedMemberOfEngine{},
		unexpectedReferenceEngine{},
		unexpectedObservableProvider{},
	)
	if err != nil {
		t.Fatalf("NewGenericSQLiteAdapter: %v", err)
	}
	return adapter
}

type unexpectedMemberOfEngine struct{}

func (unexpectedMemberOfEngine) EvaluateMemberOf(
	context.Context,
	MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	return nil, fmt.Errorf("unexpected MemberOf evaluation")
}

type unexpectedReferenceEngine struct{}

func (unexpectedReferenceEngine) ResolveStrongReference(
	context.Context,
	StrongReferenceResolutionInput,
) (typedmemory.StrongReferenceResolution, error) {
	return nil, fmt.Errorf("unexpected reference resolution")
}

type unexpectedObservableProvider struct{}

func (unexpectedObservableProvider) LoadObservableInput(
	context.Context,
	projectledger.ProjectID,
	typedmemory.ObservableInputRef,
	typedmemory.SHA256Digest,
) (ObservableInputBlob, error) {
	return ObservableInputBlob{}, fmt.Errorf("unexpected observable input")
}
