package projectmemory

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
)

type entityRuntimeStore struct {
	replayErr   error
	commitErr   error
	replayCalls int
	commitCalls int
}

func (store *entityRuntimeStore) ReplayMemoryChangeSetByIdempotencyKey(
	context.Context,
	typedmemorystore.IdempotencyReplayRequest,
) (typedmemorystore.CommitReceipt, bool, error) {
	store.replayCalls++
	return typedmemorystore.CommitReceipt{},
		store.replayErr != nil,
		store.replayErr
}

func (store *entityRuntimeStore) CommitMemoryChangeSet(
	context.Context,
	typedmemorystore.CommitRequest,
) (typedmemorystore.CommitReceipt, error) {
	store.commitCalls++
	return typedmemorystore.CommitReceipt{}, store.commitErr
}

type entityRuntimeReader struct {
	calls int
}

func (reader *entityRuntimeReader) LoadEntityContext(
	context.Context,
	projectledger.ProjectID,
	typedmemory.EntityID,
	typedmemory.BoundedContextRef,
) (typedmemorystore.StoredEntity, error) {
	reader.calls++
	return typedmemorystore.StoredEntity{}, errors.New(
		"unexpected entity context read",
	)
}

type entityRuntimeSnapshot struct {
	typeEnv  typedmemory.TypeEnvRef
	revision typedmemory.GraphRevision
	entity   typedmemory.EntityResolution
	aliases  map[string]typedmemory.AliasAvailability
}

func (snapshot *entityRuntimeSnapshot) GraphRevision() typedmemory.GraphRevision {
	return snapshot.revision
}

func (snapshot *entityRuntimeSnapshot) TypeEnvRef() typedmemory.TypeEnvRef {
	return snapshot.typeEnv
}

func (snapshot *entityRuntimeSnapshot) ResolveEntity(
	typedmemory.EntityID,
	typedmemory.BoundedContextRef,
) typedmemory.EntityResolution {
	return snapshot.entity
}

func (snapshot *entityRuntimeSnapshot) ResolveAlias(
	alias typedmemory.EntityAlias,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.AliasAvailability {
	return snapshot.aliases[alias.String()+"\x00"+contextRef.String()]
}

func (*entityRuntimeSnapshot) ResolveReference(
	typedmemory.StrongRef,
	typedmemory.BoundedContextRef,
) typedmemory.StrongReferenceResolution {
	return nil
}

func (*entityRuntimeSnapshot) EvaluateMemberOf(
	typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	return nil
}

func (*entityRuntimeSnapshot) AssertionState(
	typedmemory.AssertionID,
) typedmemory.AssertionState {
	return nil
}

func (*entityRuntimeSnapshot) ResolveReconciliationBasis(
	typedmemory.ReconciliationBasisRef,
	typedmemory.BoundedContextRef,
) typedmemory.ReconciliationBasisResolution {
	return nil
}

func TestEntityEstablishmentRuntimeReturnsEnablementChoiceWithoutWrite(
	t *testing.T,
) {
	t.Parallel()

	source := &recordingBasisSource{
		resolution: typedmemoryvalidation.NewProjectBasisUnavailable(),
	}
	store := &entityRuntimeStore{}
	runtime := newEntityRuntimeForTest(
		t,
		runtimeProjectID(t),
		source,
		store,
	)
	request := entityRuntimeRequest(t, "context:software-system", nil)

	result, err := runtime.Establish(context.Background(), request)
	if err != nil {
		t.Fatalf("Establish() error = %v", err)
	}
	if result.Kind() != EntityEnablementChoiceRequiredResult ||
		store.commitCalls != 0 ||
		store.replayCalls != 1 {
		t.Fatalf(
			"result/commit/replay = %s/%d/%d",
			result.Kind(),
			store.commitCalls,
			store.replayCalls,
		)
	}
}

func TestEntityEstablishmentRuntimeReturnsAliasConflictBeforeWrite(
	t *testing.T,
) {
	t.Parallel()

	fixture := newRuntimeCandidateFixture(t, 81)
	request := entityRuntimeRequest(
		t,
		fixture.contextRef.String(),
		[]string{"auth"},
	)
	other := mustRuntimeCandidateValue(
		typedmemory.NewEntityID("service:other"),
	)
	alias := request.Aliases()[0]
	basisRef := mustRuntimeCandidateValue(
		typedmemory.NewResolutionBasisRef("basis:entity-runtime-alias"),
	)
	absent := mustRuntimeCandidateValue(
		typedmemory.NewAbsentEntityResolution(
			request.EntityID(),
			fixture.contextRef,
			basisRef,
		),
	)
	bound := mustRuntimeCandidateValue(
		typedmemory.NewBoundAliasResolution(
			alias,
			other,
			fixture.contextRef,
			basisRef,
		),
	)
	snapshot := &entityRuntimeSnapshot{
		typeEnv:  fixture.typeEnv,
		revision: fixture.revision,
		entity:   absent,
		aliases: map[string]typedmemory.AliasAvailability{
			alias.String() + "\x00" + fixture.contextRef.String(): bound,
		},
	}
	original := fixture.resolution.(*typedmemoryvalidation.ResolvedProjectBasis)
	resolution := mustRuntimeCandidateValue(
		typedmemoryvalidation.NewResolvedProjectBasis(
			original.Environment(),
			original.Codecs(),
			snapshot,
		),
	)
	source := &recordingBasisSource{resolution: resolution}
	store := &entityRuntimeStore{}
	runtime := newEntityRuntimeForTest(
		t,
		fixture.projectID,
		source,
		store,
	)

	result, err := runtime.Establish(context.Background(), request)
	if err != nil {
		t.Fatalf("Establish() error = %v", err)
	}
	if result.Kind() != EntityAliasConflictResult ||
		store.commitCalls != 0 {
		t.Fatalf("result/commit = %s/%d", result.Kind(), store.commitCalls)
	}
}

func TestEntityEstablishmentRuntimeClosesIdempotencyConflict(t *testing.T) {
	t.Parallel()

	source := &recordingBasisSource{
		resolution: typedmemoryvalidation.NewProjectBasisUnavailable(),
	}
	store := &entityRuntimeStore{
		replayErr: typedmemorystore.ErrIdempotencyConflict,
	}
	runtime := newEntityRuntimeForTest(
		t,
		runtimeProjectID(t),
		source,
		store,
	)
	result, err := runtime.Establish(
		context.Background(),
		entityRuntimeRequest(t, "context:software-system", nil),
	)
	if err != nil {
		t.Fatalf("Establish() error = %v", err)
	}
	if result.Kind() != EntityIdempotencyConflictResult ||
		source.calls != 0 ||
		store.commitCalls != 0 {
		t.Fatalf(
			"result/source/commit = %s/%d/%d",
			result.Kind(),
			source.calls,
			store.commitCalls,
		)
	}
}

func TestEntityEstablishmentRuntimeRetriesCASAndRejectsWithoutPartialClaim(
	t *testing.T,
) {
	t.Parallel()

	fixture := newRuntimeCandidateFixture(t, 83)
	source := &recordingBasisSource{resolution: fixture.resolution}
	store := &entityRuntimeStore{
		commitErr: typedmemorystore.ErrStaleGraphRevision,
	}
	runtime := newEntityRuntimeForTest(
		t,
		fixture.projectID,
		source,
		store,
	)
	result, err := runtime.Establish(
		context.Background(),
		entityRuntimeRequest(t, fixture.contextRef.String(), nil),
	)
	if err != nil {
		t.Fatalf("Establish() error = %v", err)
	}
	if result.Kind() != EntityRejectedResult ||
		store.commitCalls != maximumEntityEstablishmentCASAttempts {
		t.Fatalf(
			"result/commit calls = %s/%d",
			result.Kind(),
			store.commitCalls,
		)
	}
}

func TestEntityEstablishmentRuntimeClosesCommitOutcomeUnknown(t *testing.T) {
	t.Parallel()

	fixture := newRuntimeCandidateFixture(t, 89)
	source := &recordingBasisSource{resolution: fixture.resolution}
	store := &entityRuntimeStore{
		commitErr: typedmemorystore.ErrCommitOutcomeUnknown,
	}
	runtime := newEntityRuntimeForTest(
		t,
		fixture.projectID,
		source,
		store,
	)
	result, err := runtime.Establish(
		context.Background(),
		entityRuntimeRequest(t, fixture.contextRef.String(), nil),
	)
	if err != nil {
		t.Fatalf("Establish() error = %v", err)
	}
	if result.Kind() != EntityCommitOutcomeUnknownResult ||
		store.commitCalls != 1 {
		t.Fatalf(
			"result/commit calls = %s/%d",
			result.Kind(),
			store.commitCalls,
		)
	}
}

func newEntityRuntimeForTest(
	t *testing.T,
	projectID projectledger.ProjectID,
	source ProjectBasisSource,
	store *entityRuntimeStore,
) *EntityEstablishmentRuntime {
	t.Helper()
	admission, err := NewAdmissionRuntime(projectID, source, store)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewEntityEstablishmentRuntime(
		projectID,
		source,
		admission,
		&entityRuntimeReader{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

func entityRuntimeRequest(
	t *testing.T,
	contextRef string,
	aliases []string,
) EntityEstablishmentRequest {
	t.Helper()
	aliasJSON := "[]"
	if len(aliases) > 0 {
		aliasJSON = fmt.Sprintf(`["%s"]`, aliases[0])
	}
	payload := fmt.Sprintf(`{
		"action":"establish",
		"entity_id":"service:auth",
		"label":"Authentication service",
		"bounded_context_ref":%q,
		"aliases":%s,
		"persistence_reason":"explicit_operator_request",
		"request_provenance_ref":"operator:chat",
		"idempotency_key":"entity:service:auth:v1"
	}`, contextRef, aliasJSON)
	request, err := DecodeEntityEstablishmentRequest([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	return request
}
