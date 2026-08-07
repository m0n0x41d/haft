package projectmemory

import (
	"bytes"
	"context"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhoodcache"
	"github.com/m0n0x41d/haft/internal/projectmemory/scopedrecall"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

func TestCurrentReadRuntimeRejectsStaleProcessAcrossTypeEnvTransitionAndReusesRollbackSnapshot(
	t *testing.T,
) {
	project := transitionReadProject(t)
	oldEnvironment := transitionReadEnvironment(t, "old-selected-c")
	successorEnvironment := transitionReadEnvironment(t, "successor-selected-c")
	oldFrame := transitionReadFrame(t, project, oldEnvironment)
	successorFrame := transitionReadFrame(t, project, successorEnvironment)
	oldRequest := transitionReadRequest(t, oldEnvironment)
	successorRequest := transitionReadRequest(t, successorEnvironment)
	loader := &switchingReadFrameLoader{frame: oldFrame}
	store := newTransitionRecordingNeighborhoodStore()
	cache, err := neighborhoodcache.NewShell(store)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewCurrentMemoryReadRuntimeWithNeighborhoodCache(
		project,
		loader,
		cache,
	)
	if err != nil {
		t.Fatal(err)
	}

	initial, err := runtime.Neighborhood(context.Background(), oldRequest)
	if err != nil {
		t.Fatal(err)
	}
	oldExact, ok := initial.(neighborhood.ExactNeighborhood)
	if !ok {
		t.Fatalf("initial result = %T, want ExactNeighborhood", initial)
	}
	if got := store.lastLookupKind(); got != neighborhoodcache.LookupMiss {
		t.Fatalf("initial cache lookup = %q, want miss", got)
	}
	lookupsBeforeTransition := store.lookupCount()

	loader.frame = successorFrame
	stale, err := runtime.Neighborhood(context.Background(), oldRequest)
	if err != nil {
		t.Fatal(err)
	}
	retry, ok := stale.(neighborhood.RetryRequiredResult)
	if !ok {
		t.Fatalf("stale process result = %T, want RetryRequiredResult", stale)
	}
	if retry.Cause().Kind() != neighborhood.RetryStaleSnapshot ||
		retry.RequiredSnapshot().TypeEnv() != successorEnvironment.Ref() {
		t.Fatal("stale process retry did not name the successor TypeEnv")
	}
	if store.lookupCount() != lookupsBeforeTransition {
		t.Fatal("stale request reached the cache before current-basis rejection")
	}

	successor, err := runtime.Neighborhood(
		context.Background(),
		successorRequest,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := successor.(neighborhood.ExactNeighborhood); !ok {
		t.Fatalf("successor result = %T, want ExactNeighborhood", successor)
	}
	if got := store.lastLookupKind(); got != neighborhoodcache.LookupMiss {
		t.Fatalf("successor cache lookup = %q, want miss", got)
	}

	loader.frame = oldFrame
	rolledBack, err := runtime.Neighborhood(context.Background(), oldRequest)
	if err != nil {
		t.Fatal(err)
	}
	rollbackExact, ok := rolledBack.(neighborhood.ExactNeighborhood)
	if !ok {
		t.Fatalf("rollback result = %T, want ExactNeighborhood", rolledBack)
	}
	if got := store.lastLookupKind(); got != neighborhoodcache.LookupHit {
		t.Fatalf("rollback cache lookup = %q, want hit", got)
	}
	if !bytes.Equal(
		rollbackExact.CanonicalBytes(),
		oldExact.CanonicalBytes(),
	) {
		t.Fatal("rollback did not preserve the exact old snapshot bytes")
	}
}

func TestCurrentReadRuntimeKeepsUnknownEntityAndRecallAsSeparateAbstentions(
	t *testing.T,
) {
	project := transitionReadProject(t)
	environment := transitionReadEnvironment(t, "unknown-entity")
	frame := transitionReadFrame(t, project, environment)
	request := transitionReadRequestForEntity(
		t,
		environment,
		"entity:not-in-current-directory",
	)
	runtime, err := NewCurrentMemoryReadRuntime(
		project,
		&switchingReadFrameLoader{frame: frame},
	)
	if err != nil {
		t.Fatal(err)
	}

	projected, err := runtime.Neighborhood(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	unknown, ok := projected.(neighborhood.AbstainedResult)
	if !ok {
		t.Fatalf("unknown neighborhood = %T, want AbstainedResult", projected)
	}
	if unknown.Basis().Kind() !=
		neighborhood.AbstainEntityOrContextNotFound ||
		unknown.Interpretation().RelationalRecords() !=
			neighborhood.RelationalRecordsUnavailable {
		t.Fatal("unknown entity fabricated a current project neighborhood")
	}

	query, err := scopedrecall.NewRecallQuery("authentication service")
	if err != nil {
		t.Fatal(err)
	}
	budget, err := scopedrecall.NewCandidateBudget(4)
	if err != nil {
		t.Fatal(err)
	}
	recalled, err := runtime.Recall(
		context.Background(),
		request,
		query,
		budget,
	)
	if err != nil {
		t.Fatal(err)
	}
	abstained, ok := recalled.(scopedrecall.ScopedRecallAbstained)
	if !ok {
		t.Fatalf("unknown recall = %T, want ScopedRecallAbstained", recalled)
	}
	if abstained.Basis().Kind() !=
		scopedrecall.AbstentionNoUsableProducer ||
		abstained.Interpretation().Authority() !=
			neighborhood.AuthorityNotGranted ||
		abstained.Interpretation().WorkOrder() !=
			neighborhood.WorkOrderNotImplied {
		t.Fatal("unknown-entity recall fabricated candidates or authority")
	}
}

type switchingReadFrameLoader struct {
	frame typedmemorystore.CurrentProjectReadFrame
}

func (loader *switchingReadFrameLoader) LoadCurrentProjectReadFrame(
	context.Context,
	projectledger.ProjectID,
) (typedmemorystore.CurrentProjectReadFrame, error) {
	return loader.frame, nil
}

type transitionRecordingNeighborhoodStore struct {
	delegate *neighborhoodcache.AtomicStore
	lookups  []neighborhoodcache.LookupKind
}

func newTransitionRecordingNeighborhoodStore() *transitionRecordingNeighborhoodStore {
	return &transitionRecordingNeighborhoodStore{
		delegate: neighborhoodcache.NewAtomicStore(),
	}
}

func (store *transitionRecordingNeighborhoodStore) Lookup(
	ctx context.Context,
	key neighborhoodcache.Key,
) neighborhoodcache.LookupResult {
	result := store.delegate.Lookup(ctx, key)
	store.lookups = append(store.lookups, result.Kind())
	return result
}

func (store *transitionRecordingNeighborhoodStore) Put(
	ctx context.Context,
	entry neighborhoodcache.Entry,
) neighborhoodcache.StoreResult {
	return store.delegate.Put(ctx, entry)
}

func (store *transitionRecordingNeighborhoodStore) lookupCount() int {
	return len(store.lookups)
}

func (store *transitionRecordingNeighborhoodStore) lastLookupKind() neighborhoodcache.LookupKind {
	return store.lookups[len(store.lookups)-1]
}

func transitionReadProject(t *testing.T) projectledger.ProjectID {
	t.Helper()
	project, err := projectledger.ParseProjectID("qnt_cac4e001")
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func transitionReadEnvironment(
	t *testing.T,
	seed string,
) typedmemory.TypeEnv {
	t.Helper()
	baseArtifact := loaderTestBundledArtifact(t)
	base, _, err := typeenv.LowerBaseTypeEnvArtifactWithCodecs(baseArtifact)
	if err != nil {
		t.Fatal(err)
	}
	seedBytes := []byte(seed)
	digest := loaderTestDigest(t, seedBytes)
	reference, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatal(err)
	}
	contexts := base.BoundedContexts()
	if len(contexts) == 0 {
		t.Fatal("bundled TypeEnv has no bounded context")
	}
	environment, err := typedmemory.NewTypeEnvBuilder(reference).
		SetSourceRevision(base.SourceRevision()).
		SetCompilerSchemaVersion(base.CompilerSchemaVersion()).
		SetCoverageManifest(base.CoverageManifest()).
		AddBoundedContext(contexts[0]).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	return environment
}

func transitionReadFrame(
	t *testing.T,
	project projectledger.ProjectID,
	environment typedmemory.TypeEnv,
) typedmemorystore.CurrentProjectReadFrame {
	t.Helper()
	revision := typedmemory.NewGraphRevision(1)
	basis := transitionReadGraphBasis(t, project, revision)
	entry := transitionReadDirectoryEntry(t, basis)
	directory, err := typedmemorystore.NewCurrentEntityDirectory(
		project,
		basis,
		environment.Ref(),
		[]typedmemorystore.CurrentEntityDirectoryEntry{entry},
	)
	if err != nil {
		t.Fatal(err)
	}
	active, err := projectgraphobservation.NewCurrentActiveAssertionSet(
		project,
		revision,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := projectgraphobservation.NewCurrentProjectGraphObservation(
		basis,
		environment.Ref(),
		active,
	)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &sqliteBasisSnapshot{
		revision: revision,
		typeEnv:  environment.Ref(),
	}
	codecs := typedmemory.NewCodecRegistry()
	current, err := typedmemorystore.NewCurrentProjectSnapshot(
		project,
		environment,
		codecs,
		snapshot,
	)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := typedmemorystore.NewCurrentProjectReadFrame(
		current,
		directory,
		graph,
	)
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func transitionReadGraphBasis(
	t *testing.T,
	project projectledger.ProjectID,
	revision typedmemory.GraphRevision,
) projecttypeenvselection.ProjectGraphSnapshotBasis {
	t.Helper()
	eventFill := bytes.Repeat([]byte("a"), 64)
	eventText := string(eventFill)
	event, err := projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + eventText,
	)
	if err != nil {
		t.Fatal(err)
	}
	commitFill := bytes.Repeat([]byte("b"), 64)
	commitText := string(commitFill)
	commit, err := projecttypeenvselection.ParseGraphCommitRef(
		"typed-memory-commit:" + commitText,
	)
	if err != nil {
		t.Fatal(err)
	}
	materializationBytes := []byte("graph")
	materializationDigest := loaderTestDigest(t, materializationBytes)
	closure, err := projecttypeenvselection.NewCommittedProjectGraphClosure(
		projecttypeenvselection.CommittedProjectGraphClosureInput{
			Event:                 event,
			Commit:                commit,
			MaterializationDigest: materializationDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	basis, err := projecttypeenvselection.SealProjectGraphSnapshotBasis(
		projecttypeenvselection.ProjectGraphSnapshotBasisInput{
			Project:       project,
			GraphRevision: revision,
			Closure:       closure,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return basis
}

func transitionReadDirectoryEntry(
	t *testing.T,
	basis projecttypeenvselection.ProjectGraphSnapshotBasis,
) typedmemorystore.CurrentEntityDirectoryEntry {
	t.Helper()
	entity, err := typedmemory.NewEntityID("entity:authentication-service")
	if err != nil {
		t.Fatal(err)
	}
	contextRef, err := typedmemory.NewBoundedContextRef("context:project")
	if err != nil {
		t.Fatal(err)
	}
	label, err := typedmemory.NewEntityLabel("Authentication service")
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewProvenanceRef("memory:test:transition")
	if err != nil {
		t.Fatal(err)
	}
	closure, ok := basis.Closure().(projecttypeenvselection.CommittedProjectGraphClosure)
	if !ok {
		t.Fatal("transition fixture graph closure is not committed")
	}
	entry, err := typedmemorystore.NewCurrentEntityDirectoryEntry(
		typedmemorystore.CurrentEntityDirectoryEntryInput{
			Entity:           entity,
			Context:          contextRef,
			Label:            label,
			Provenance:       provenance,
			DeclaredEvent:    closure.Event(),
			DeclaredRevision: basis.GraphRevision(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func transitionReadRequest(
	t *testing.T,
	environment typedmemory.TypeEnv,
) neighborhood.NeighborhoodRequest {
	t.Helper()
	return transitionReadRequestForEntity(
		t,
		environment,
		"entity:authentication-service",
	)
}

func transitionReadRequestForEntity(
	t *testing.T,
	environment typedmemory.TypeEnv,
	referenceIDValue string,
) neighborhood.NeighborhoodRequest {
	t.Helper()
	refKindID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatal(err)
	}
	refKind, err := typedmemory.NewRefKindRef(environment.Ref(), refKindID)
	if err != nil {
		t.Fatal(err)
	}
	referenceID, err := typedmemory.NewReferenceID(referenceIDValue)
	if err != nil {
		t.Fatal(err)
	}
	entity, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatal(err)
	}
	contextRef, err := typedmemory.NewBoundedContextRef("context:project")
	if err != nil {
		t.Fatal(err)
	}
	profileRef, err := neighborhood.ParseProjectionProfileRef(
		"decision_rationale.v1",
	)
	if err != nil {
		t.Fatal(err)
	}
	view, err := neighborhood.NewNeighborhoodViewSpec(
		profileRef,
		[]neighborhood.FacetKind{neighborhood.FacetProblems},
		neighborhood.DetailStandard,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	budget, err := neighborhood.NewReadBudgetBuilder().
		SetMaxFacets(1).
		SetMaxItemsPerFacet(2).
		SetMaxRelationPathsPerItem(2).
		SetMaxCarrierExcerptCharacters(1024).
		SetMaxProvenanceDepth(3).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	revision := typedmemory.NewGraphRevision(1)
	request, err := neighborhood.NewNeighborhoodRequestBuilder().
		SetEntity(entity).
		SetContext(contextRef).
		SetTypeEnv(environment.Ref()).
		SetGraphRevision(revision).
		SetView(view).
		SetBudget(budget).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	return request
}
