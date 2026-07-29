package projectmemory

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
	"github.com/m0n0x41d/haft/internal/typedmemoryvalidation"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

type runtimeCandidateSnapshot struct {
	typeEnv  typedmemory.TypeEnvRef
	revision typedmemory.GraphRevision
}

func (snapshot *runtimeCandidateSnapshot) GraphRevision() typedmemory.GraphRevision {
	return snapshot.revision
}

func (snapshot *runtimeCandidateSnapshot) TypeEnvRef() typedmemory.TypeEnvRef {
	return snapshot.typeEnv
}

func (snapshot *runtimeCandidateSnapshot) ResolveEntity(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.EntityResolution {
	basis := mustRuntimeCandidateValue(
		typedmemory.NewResolutionBasisRef("fixture:runtime-candidate:absence"),
	)
	return mustRuntimeCandidateValue(
		typedmemory.NewAbsentEntityResolution(entity, contextRef, basis),
	)
}

func (*runtimeCandidateSnapshot) ResolveReference(
	typedmemory.StrongRef,
	typedmemory.BoundedContextRef,
) typedmemory.StrongReferenceResolution {
	return nil
}

func (*runtimeCandidateSnapshot) EvaluateMemberOf(
	typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	return nil
}

func (*runtimeCandidateSnapshot) AssertionState(
	typedmemory.AssertionID,
) typedmemory.AssertionState {
	return nil
}

func (*runtimeCandidateSnapshot) ResolveAlias(
	typedmemory.EntityAlias,
	typedmemory.BoundedContextRef,
) typedmemory.AliasAvailability {
	return nil
}

func (*runtimeCandidateSnapshot) ResolveReconciliationBasis(
	typedmemory.ReconciliationBasisRef,
	typedmemory.BoundedContextRef,
) typedmemory.ReconciliationBasisResolution {
	return nil
}

type runtimeCandidateCommitPort struct {
	calls int
}

func (port *runtimeCandidateCommitPort) CommitMemoryChangeSet(
	context.Context,
	typedmemorystore.CommitRequest,
) (typedmemorystore.CommitReceipt, error) {
	port.calls++
	return typedmemorystore.CommitReceipt{}, nil
}

type runtimeUnknownThenReplayStore struct {
	commitCalls    int
	replayCalls    int
	durableEffects int
	commitVersions []typedmemorystore.AdmissionContractVersion
	replayVersions []typedmemorystore.AdmissionContractVersion
}

type runtimeExactReplayStore struct {
	replayRequests []typedmemorystore.ReplayRequest
}

func (*runtimeExactReplayStore) CommitMemoryChangeSet(
	context.Context,
	typedmemorystore.CommitRequest,
) (typedmemorystore.CommitReceipt, error) {
	return typedmemorystore.CommitReceipt{}, fmt.Errorf(
		"fresh commit must not run after an exact replay hit",
	)
}

func (store *runtimeExactReplayStore) ReplayMemoryChangeSet(
	_ context.Context,
	request typedmemorystore.ReplayRequest,
) (typedmemorystore.CommitReceipt, bool, error) {
	store.replayRequests = append(store.replayRequests, request)
	return typedmemorystore.CommitReceipt{}, true, nil
}

func (store *runtimeUnknownThenReplayStore) CommitMemoryChangeSet(
	_ context.Context,
	request typedmemorystore.CommitRequest,
) (typedmemorystore.CommitReceipt, error) {
	store.commitCalls++
	store.commitVersions = append(store.commitVersions, request.ContractVersion())
	store.durableEffects++
	return typedmemorystore.CommitReceipt{}, fmt.Errorf(
		"%w: injected after one durable effect",
		typedmemorystore.ErrCommitOutcomeUnknown,
	)
}

func (store *runtimeUnknownThenReplayStore) ReplayMemoryChangeSet(
	_ context.Context,
	request typedmemorystore.ReplayRequest,
) (typedmemorystore.CommitReceipt, bool, error) {
	store.replayCalls++
	store.replayVersions = append(store.replayVersions, request.ContractVersion())
	return typedmemorystore.CommitReceipt{},
		store.durableEffects > 0,
		nil
}

func TestValidationRuntimeEvaluateCandidateMatchesStrictWirePath(
	t *testing.T,
) {
	t.Parallel()

	fixture := newRuntimeCandidateFixture(t, 61)
	candidate := runtimeTypedCandidate(t, fixture.contextRef)
	wireRequest := runtimeWireCandidateRequest(t, fixture.contextRef)
	source := &recordingBasisSource{resolution: fixture.resolution}
	runtime := runtimeValidationRuntime(t, fixture.projectID, source)

	wireOutcome, err := runtime.Evaluate(context.Background(), wireRequest)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	candidateOutcome, err := runtime.EvaluateCandidate(
		context.Background(),
		typedmemorywire.ProjectCurrentSelector{},
		candidate,
	)
	if err != nil {
		t.Fatalf("EvaluateCandidate() error = %v", err)
	}
	wireValid, wireReady := wireOutcome.(typedmemoryvalidation.ValidOutcome)
	candidateValid, candidateReady := candidateOutcome.(typedmemoryvalidation.ValidOutcome)
	if !wireReady || !candidateReady {
		t.Fatalf(
			"outcomes = %T/%T, want two ValidOutcome values",
			wireOutcome,
			candidateOutcome,
		)
	}
	if source.calls != 2 {
		t.Fatalf("basis source calls = %d, want 2", source.calls)
	}
	if wireValid.SemanticChangeDigest() != candidateValid.SemanticChangeDigest() {
		t.Fatal("typed candidate semantic digest differs from strict wire validation")
	}
	wireBasis := wireValid.AdmissionBasis()
	candidateBasis := candidateValid.AdmissionBasis()
	if wireBasis.Digest() != candidateBasis.Digest() ||
		!bytes.Equal(wireBasis.CanonicalBytes(), candidateBasis.CanonicalBytes()) {
		t.Fatal("typed candidate admission basis differs from strict wire validation")
	}
	candidateDigest := mustRuntimeCandidateValue(candidate.Digest())
	if candidateValid.Candidate().Changes()[0] != candidate.Changes()[0] ||
		candidateValid.AdmissionBatch().RequestDigest() != candidateDigest {
		t.Fatal("EvaluateCandidate() did not retain the exact trusted typed candidate")
	}
}

func TestValidationRuntimeEvaluateCandidateRejectsZeroBeforeBasisResolution(
	t *testing.T,
) {
	t.Parallel()

	source := &recordingBasisSource{
		resolution: typedmemoryvalidation.NewProjectBasisUnavailable(),
	}
	runtime := runtimeValidationRuntime(t, runtimeProjectID(t), source)

	outcome, err := runtime.EvaluateCandidate(
		context.Background(),
		typedmemorywire.ProjectCurrentSelector{},
		typedmemory.MemoryChangeSet{},
	)
	if err != ErrProjectBasisRequestInvalid {
		t.Fatalf("EvaluateCandidate() error = %v, want ErrProjectBasisRequestInvalid", err)
	}
	if outcome != nil {
		t.Fatalf("EvaluateCandidate() outcome = %T, want nil", outcome)
	}
	if source.calls != 0 {
		t.Fatalf("invalid typed candidate reached basis source %d time(s)", source.calls)
	}
}

func TestAdmissionRuntimePrepareCandidateReturnsSameSealedCapabilityWithoutWrite(
	t *testing.T,
) {
	t.Parallel()

	fixture := newRuntimeCandidateFixture(t, 67)
	candidate := runtimeTypedCandidate(t, fixture.contextRef)
	source := &recordingBasisSource{resolution: fixture.resolution}
	store := &runtimeCandidateCommitPort{}
	runtime, err := NewAdmissionRuntime(fixture.projectID, source, store)
	if err != nil {
		t.Fatalf("NewAdmissionRuntime() error = %v", err)
	}
	if runtime.ProjectID() != fixture.projectID {
		t.Fatalf(
			"AdmissionRuntime.ProjectID() = %q, want %q",
			runtime.ProjectID().String(),
			fixture.projectID.String(),
		)
	}

	valid, err := runtime.PrepareCandidate(
		context.Background(),
		typedmemorywire.ProjectCurrentSelector{},
		candidate,
	)
	if err != nil {
		t.Fatalf("PrepareCandidate() error = %v", err)
	}
	if source.calls != 1 {
		t.Fatalf("basis source calls = %d, want 1", source.calls)
	}
	if store.calls != 0 {
		t.Fatalf("PrepareCandidate() writes = %d, want 0", store.calls)
	}
	candidateDigest := mustRuntimeCandidateValue(candidate.Digest())
	if valid.AdmissionBatch().RequestDigest() != candidateDigest ||
		valid.Candidate().Changes()[0] != candidate.Changes()[0] {
		t.Fatal("PrepareCandidate() capability does not retain the exact typed candidate")
	}
	if !valid.AdmissionBatch().IsValid() {
		t.Fatal("PrepareCandidate() did not return a sealed AdmissionBatch")
	}
	if valid.ContractVersion() != typedmemorywire.ContractVersionV2 {
		t.Fatalf(
			"PrepareCandidate() contract version = %q; want v2",
			valid.ContractVersion(),
		)
	}
}

func TestAdmissionRuntimePrepareCandidateRejectsZeroBeforeBasisResolution(
	t *testing.T,
) {
	t.Parallel()

	source := &recordingBasisSource{
		resolution: typedmemoryvalidation.NewProjectBasisUnavailable(),
	}
	store := &runtimeCandidateCommitPort{}
	runtime, err := NewAdmissionRuntime(runtimeProjectID(t), source, store)
	if err != nil {
		t.Fatalf("NewAdmissionRuntime() error = %v", err)
	}

	valid, err := runtime.PrepareCandidate(
		context.Background(),
		typedmemorywire.ProjectCurrentSelector{},
		typedmemory.MemoryChangeSet{},
	)
	if err != ErrProjectBasisRequestInvalid {
		t.Fatalf("PrepareCandidate() error = %v, want ErrProjectBasisRequestInvalid", err)
	}
	if valid != nil {
		t.Fatalf("PrepareCandidate() result = %T, want nil", valid)
	}
	if source.calls != 0 || store.calls != 0 {
		t.Fatalf(
			"zero candidate effects = source:%d store:%d, want 0/0",
			source.calls,
			store.calls,
		)
	}
}

func TestAdmissionRuntimeClosesCommitUnknownAndSameKeyReplayWithoutSecondEffect(
	t *testing.T,
) {
	t.Parallel()

	fixture := newRuntimeCandidateFixture(t, 71)
	source := &recordingBasisSource{resolution: fixture.resolution}
	store := &runtimeUnknownThenReplayStore{}
	runtime, err := NewAdmissionRuntime(fixture.projectID, source, store)
	if err != nil {
		t.Fatal(err)
	}
	request := runtimeWireAdmitCandidateRequest(
		t,
		fixture,
		typedmemorywire.ContractVersionV2,
	)

	first, err := runtime.Admit(context.Background(), request)
	if err != nil {
		t.Fatalf("closed unknown became runtime error: %v", err)
	}
	unknown, ok := first.(AdmissionCommitOutcomeUnknown)
	if !ok {
		t.Fatalf("first result = %T, want AdmissionCommitOutcomeUnknown", first)
	}
	retry := unknown.RetryCoordinates()
	if retry.ContractVersion() != typedmemorystore.AdmissionContractV2() ||
		retry.ProjectID() != fixture.projectID ||
		retry.IdempotencyKey().String() != "runtime-candidate-replay" ||
		retry.TypeEnv() != fixture.typeEnv ||
		retry.GraphRevision() != fixture.revision ||
		retry.CandidateDigest().String() == "" ||
		retry.RequestIdentityDigest().String() == "" {
		t.Fatalf("unknown retry coordinates = %#v", retry)
	}

	second, err := runtime.Admit(context.Background(), request)
	if err != nil {
		t.Fatalf("same-key replay error = %v", err)
	}
	committed, ok := second.(AdmissionCommitted)
	if !ok {
		t.Fatalf("same-key replay result = %T, want AdmissionCommitted", second)
	}
	if committed.ContractVersion() != typedmemorywire.ContractVersionV2 {
		t.Fatalf(
			"same-key replay contract version = %q; want v2",
			committed.ContractVersion(),
		)
	}
	if store.commitCalls != 1 ||
		store.replayCalls != 2 ||
		store.durableEffects != 1 {
		t.Fatalf(
			"commit/replay/effects = %d/%d/%d, want 1/2/1",
			store.commitCalls,
			store.replayCalls,
			store.durableEffects,
		)
	}
	if len(store.commitVersions) != 1 ||
		store.commitVersions[0] != typedmemorystore.AdmissionContractV2() {
		t.Fatalf("commit versions = %#v; want one v2", store.commitVersions)
	}
	if len(store.replayVersions) != 2 {
		t.Fatalf("replay versions = %#v; want two v2", store.replayVersions)
	}
	for _, version := range store.replayVersions {
		if version != typedmemorystore.AdmissionContractV2() {
			t.Fatalf("replay version = %q; want v2", version.String())
		}
	}
}

func TestAdmissionRuntimeThreadsDecodedV1OnlyIntoExactReplayIdentity(t *testing.T) {
	t.Parallel()

	fixture := newRuntimeCandidateFixture(t, 73)
	source := &recordingBasisSource{resolution: fixture.resolution}
	store := &runtimeExactReplayStore{}
	runtime, err := NewAdmissionRuntime(fixture.projectID, source, store)
	if err != nil {
		t.Fatal(err)
	}
	request := runtimeWireAdmitCandidateRequest(
		t,
		fixture,
		typedmemorywire.ContractVersionV1,
	)

	result, err := runtime.Admit(context.Background(), request)
	if err != nil {
		t.Fatalf("Admit(v1 exact replay): %v", err)
	}
	committed, ok := result.(AdmissionCommitted)
	if !ok || committed.ContractVersion() != typedmemorywire.ContractVersionV1 {
		t.Fatalf("v1 replay result = %#v; want v1 AdmissionCommitted", result)
	}
	if source.calls != 0 {
		t.Fatalf("v1 exact replay reached validation source %d time(s)", source.calls)
	}
	if len(store.replayRequests) != 1 ||
		store.replayRequests[0].ContractVersion() !=
			typedmemorystore.AdmissionContractV1() {
		t.Fatalf("v1 replay requests = %#v", store.replayRequests)
	}
}

type runtimeCandidateFixture struct {
	projectID  projectledger.ProjectID
	contextRef typedmemory.BoundedContextRef
	typeEnv    typedmemory.TypeEnvRef
	revision   typedmemory.GraphRevision
	resolution typedmemoryvalidation.BasisResolution
}

func newRuntimeCandidateFixture(
	t *testing.T,
	revision uint64,
) runtimeCandidateFixture {
	t.Helper()
	fixture := newSQLiteBasisFixture(t, revision)
	contexts := fixture.environment.BoundedContexts()
	if len(contexts) == 0 {
		t.Fatal("runtime candidate TypeEnv has no bounded context")
	}
	snapshot := &runtimeCandidateSnapshot{
		typeEnv:  fixture.environment.Ref(),
		revision: typedmemory.NewGraphRevision(revision),
	}
	resolution := mustRuntimeCandidateValue(
		typedmemoryvalidation.NewResolvedProjectBasis(
			fixture.environment,
			fixture.current.Codecs(),
			snapshot,
		),
	)
	return runtimeCandidateFixture{
		projectID:  fixture.projectID,
		contextRef: contexts[0].Ref(),
		typeEnv:    fixture.environment.Ref(),
		revision:   snapshot.revision,
		resolution: resolution,
	}
}

func runtimeTypedCandidate(
	t *testing.T,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.MemoryChangeSet {
	t.Helper()
	entity := mustRuntimeCandidateValue(
		typedmemory.NewEntityID("entity:runtime-candidate"),
	)
	local := mustRuntimeCandidateValue(
		typedmemory.NewBatchLocalRef("local:runtime-candidate"),
	)
	label := mustRuntimeCandidateValue(
		typedmemory.NewEntityLabel("Runtime candidate"),
	)
	provenance := mustRuntimeCandidateValue(
		typedmemory.NewProvenanceRef("provenance:runtime-candidate"),
	)
	declaration := mustRuntimeCandidateValue(
		typedmemory.NewDeclareEntity(
			entity,
			local,
			contextRef,
			label,
			provenance,
		),
	)
	return mustRuntimeCandidateValue(
		typedmemory.NewMemoryChangeSet(
			[]typedmemory.MemoryChange{declaration},
		),
	)
}

func runtimeWireCandidateRequest(
	t *testing.T,
	contextRef typedmemory.BoundedContextRef,
) typedmemorywire.ValidateRequest {
	t.Helper()
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "validate",
  "basis": {"kind":"project_current"},
  "change_set": {
    "changes": [{
      "kind": "declare_entity",
      "entity_id": "entity:runtime-candidate",
      "local_ref": "local:runtime-candidate",
      "context": %q,
      "label": "Runtime candidate",
      "provenance": "provenance:runtime-candidate"
    }]
  }
}`, typedmemorywire.ContractVersionV2, contextRef.String())
	request, err := typedmemorywire.DecodeValidateRequest([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v\npayload=%s", err, payload)
	}
	return request
}

func runtimeWireAdmitCandidateRequest(
	t *testing.T,
	fixture runtimeCandidateFixture,
	contractVersion string,
) typedmemorywire.AdmitRequest {
	t.Helper()
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "admit",
  "basis": {
    "kind": "exact_project",
    "type_env_digest": %q,
    "graph_revision": %d
  },
  "authority_class": "non_binding_semantic_assertion",
  "idempotency_key": "runtime-candidate-replay",
  "request_provenance_ref": "provenance:runtime-candidate-replay",
  "change_set": {
    "changes": [{
      "kind": "declare_entity",
      "entity_id": "entity:runtime-candidate",
      "local_ref": "local:runtime-candidate",
      "context": %q,
      "label": "Runtime candidate",
      "provenance": "provenance:runtime-candidate"
    }]
  }
}`,
		contractVersion,
		fixture.typeEnv.Digest().String(),
		fixture.revision.Value(),
		fixture.contextRef.String(),
	)
	request, err := typedmemorywire.DecodeAdmitRequest([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeAdmitRequest() error = %v\npayload=%s", err, payload)
	}
	return request
}

func mustRuntimeCandidateValue[T any](
	value T,
	err error,
) T {
	if err != nil {
		panic(err)
	}
	return value
}
