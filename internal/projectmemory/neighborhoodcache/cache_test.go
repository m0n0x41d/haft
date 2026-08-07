package neighborhoodcache

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestReadThroughCacheHitAndMissAreByteIdentical(t *testing.T) {
	fixture := newCacheFixture(
		t,
		"decision_rationale.v1",
		11,
		"6",
		2,
	)
	store := NewAtomicStore()
	shell, err := NewShell(store)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	compute := func(
		context.Context,
	) (neighborhood.ExactNeighborhood, error) {
		calls++
		return fixture.result, nil
	}

	miss, err := shell.ReadThrough(
		context.Background(),
		fixture.key,
		compute,
	)
	if err != nil {
		t.Fatal(err)
	}
	hit, err := shell.ReadThrough(
		context.Background(),
		fixture.key,
		compute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("assembler calls = %d, want one", calls)
	}
	if miss.CacheUse().Kind() != UseMissStored ||
		hit.CacheUse().Kind() != UseHit {
		t.Fatalf(
			"cache uses = %q then %q",
			miss.CacheUse().Kind(),
			hit.CacheUse().Kind(),
		)
	}
	if !bytes.Equal(miss.CanonicalBytes(), hit.CanonicalBytes()) ||
		!bytes.Equal(hit.CanonicalBytes(), fixture.result.CanonicalBytes()) {
		t.Fatal("cache hit and miss changed canonical neighborhood bytes")
	}
}

func TestCacheKeyInvalidatesEveryExactProjectionCoordinate(t *testing.T) {
	base := newCacheFixture(
		t,
		"decision_rationale.v1",
		11,
		"6",
		2,
	)
	variants := []Key{
		newCacheFixture(
			t,
			"decision_rationale.v1",
			12,
			"6",
			2,
		).key,
		newCacheFixture(
			t,
			"decision_rationale.v1",
			11,
			"7",
			2,
		).key,
		newCacheFixture(
			t,
			"agent_orientation.v1",
			11,
			"6",
			2,
		).key,
		newCacheFixture(
			t,
			"agent_orientation.v2",
			11,
			"6",
			2,
		).key,
		newCacheFixture(
			t,
			"decision_rationale.v1",
			11,
			"6",
			3,
		).key,
		mustVariantKey(
			t,
			base,
			base.key.DirectoryDigest(),
			mustGraphBasis(
				t,
				base.key.Project(),
				base.request.GraphRevision().Value(),
				"e",
			),
			base.key.ProjectionSchema(),
		),
		mustVariantKey(
			t,
			base,
			mustDigest(t, "9"),
			base.graphBasis,
			base.key.ProjectionSchema(),
		),
		mustVariantKey(
			t,
			base,
			base.key.DirectoryDigest(),
			base.graphBasis,
			mustProjectionSchema(t, "haft.projection-profile/v2"),
		),
	}
	seen := map[string]struct{}{base.key.Digest().String(): {}}
	for index, variant := range variants {
		if variant.Digest() == base.key.Digest() {
			t.Fatalf("variant %d did not invalidate cache key", index)
		}
		if _, found := seen[variant.Digest().String()]; found {
			t.Fatalf("variant %d collided with another cache key", index)
		}
		seen[variant.Digest().String()] = struct{}{}
	}
}

func TestProjectionProfileSuccessorCannotReuseLegacyCacheEntry(t *testing.T) {
	legacy := newCacheFixture(
		t,
		"agent_orientation.v1",
		11,
		"6",
		2,
	)
	current := newCacheFixture(
		t,
		"agent_orientation.v2",
		11,
		"6",
		2,
	)
	if legacy.request.View().ProfileDigest() ==
		current.request.View().ProfileDigest() {
		t.Fatal("profile successor reused the legacy profile digest")
	}
	if legacy.key.Digest() == current.key.Digest() {
		t.Fatal("profile successor reused the legacy cache coordinate")
	}

	entry, err := NewEntry(legacy.key, legacy.result)
	if err != nil {
		t.Fatal(err)
	}
	store := NewAtomicStore()
	stored := store.Put(context.Background(), entry)
	if !stored.Stored() {
		t.Fatal("legacy cache entry was not stored")
	}
	lookup := store.Lookup(context.Background(), current.key)
	if lookup.Kind() != LookupMiss {
		t.Fatalf("successor cache lookup = %q, want miss", lookup.Kind())
	}
}

func TestTypeEnvTransitionMissesWhileRollbackCanReuseExactOldSnapshot(
	t *testing.T,
) {
	old := newCacheFixture(
		t,
		"decision_rationale.v1",
		11,
		"6",
		2,
	)
	successor := newCacheFixture(
		t,
		"decision_rationale.v1",
		11,
		"7",
		2,
	)
	store := NewAtomicStore()
	shell, err := NewShell(store)
	if err != nil {
		t.Fatal(err)
	}
	oldComputes := 0
	successorComputes := 0

	first, err := shell.ReadThrough(
		context.Background(),
		old.key,
		func(context.Context) (neighborhood.ExactNeighborhood, error) {
			oldComputes++
			return old.result, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	afterTransition, err := shell.ReadThrough(
		context.Background(),
		successor.key,
		func(context.Context) (neighborhood.ExactNeighborhood, error) {
			successorComputes++
			return successor.result, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	afterRollback, err := shell.ReadThrough(
		context.Background(),
		old.key,
		func(context.Context) (neighborhood.ExactNeighborhood, error) {
			oldComputes++
			return old.result, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if first.CacheUse().Kind() != UseMissStored ||
		afterTransition.CacheUse().Kind() != UseMissStored ||
		afterRollback.CacheUse().Kind() != UseHit {
		t.Fatalf(
			"cache uses = %q, %q, %q; want miss, miss, hit",
			first.CacheUse().Kind(),
			afterTransition.CacheUse().Kind(),
			afterRollback.CacheUse().Kind(),
		)
	}
	if oldComputes != 1 || successorComputes != 1 {
		t.Fatalf(
			"compute counts = old:%d successor:%d; want 1 each",
			oldComputes,
			successorComputes,
		)
	}
	if old.key.Digest() == successor.key.Digest() {
		t.Fatal("successor TypeEnv reused the old semantic cache coordinate")
	}
	if !bytes.Equal(
		afterRollback.CanonicalBytes(),
		first.CanonicalBytes(),
	) {
		t.Fatal("rollback did not recover the exact old snapshot bytes")
	}
}

func TestNewKeyRejectsUncorrelatedGraphBasis(t *testing.T) {
	fixture := newCacheFixture(
		t,
		"decision_rationale.v1",
		11,
		"6",
		2,
	)
	otherProject, err := projectledger.ParseProjectID("qnt_abcdef12")
	if err != nil {
		t.Fatal(err)
	}
	cases := []projecttypeenvselection.ProjectGraphSnapshotBasis{
		mustGraphBasis(t, otherProject, 11, "b"),
		mustGraphBasis(t, fixture.key.Project(), 12, "c"),
	}
	for _, basis := range cases {
		_, err := NewKey(
			fixture.key.Project(),
			fixture.request,
			fixture.key.DirectoryDigest(),
			basis,
			fixture.key.ProjectionSchema(),
		)
		if err == nil {
			t.Fatal("uncorrelated graph snapshot basis was accepted")
		}
	}
}

func TestBudgetChangeCannotReuseNarrowerCachedNeighborhood(t *testing.T) {
	narrow := newCacheFixture(
		t,
		"decision_rationale.v1",
		11,
		"6",
		1,
	)
	wide := newCacheFixture(
		t,
		"decision_rationale.v1",
		11,
		"6",
		3,
	)
	store := NewAtomicStore()
	shell, err := NewShell(store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := shell.ReadThrough(
		context.Background(),
		narrow.key,
		fixedCompute(narrow.result),
	); err != nil {
		t.Fatal(err)
	}
	read, err := shell.ReadThrough(
		context.Background(),
		wide.key,
		fixedCompute(wide.result),
	)
	if err != nil {
		t.Fatal(err)
	}
	if read.CacheUse().Kind() != UseMissStored {
		t.Fatalf(
			"wide budget reused narrow entry: %q",
			read.CacheUse().Kind(),
		)
	}
	if !bytes.Equal(read.CanonicalBytes(), wide.result.CanonicalBytes()) ||
		bytes.Equal(read.CanonicalBytes(), narrow.result.CanonicalBytes()) {
		t.Fatal("budget change returned a differently budgeted view")
	}
}

func TestCorruptAndUnavailableCacheOnlyDegradePerformance(t *testing.T) {
	fixture := newCacheFixture(
		t,
		"decision_rationale.v1",
		11,
		"6",
		2,
	)
	entry, err := NewEntry(fixture.key, fixture.result)
	if err != nil {
		t.Fatal(err)
	}
	entry.canonical[0] ^= 0xff
	corruptStore := &AtomicStore{
		snapshot: Snapshot{
			entries: map[string]Entry{
				fixture.key.Digest().String(): entry,
			},
		},
	}
	corruptShell, err := NewShell(corruptStore)
	if err != nil {
		t.Fatal(err)
	}
	repaired, err := corruptShell.ReadThrough(
		context.Background(),
		fixture.key,
		fixedCompute(fixture.result),
	)
	if err != nil {
		t.Fatal(err)
	}
	if repaired.CacheUse().Kind() != UseCorruptReplaced ||
		!bytes.Equal(
			repaired.CanonicalBytes(),
			fixture.result.CanonicalBytes(),
		) {
		t.Fatal("corrupt cache changed exact neighborhood semantics")
	}

	unavailableShell := NewUnavailableShell()
	bypassed, err := unavailableShell.ReadThrough(
		context.Background(),
		fixture.key,
		fixedCompute(fixture.result),
	)
	if err != nil {
		t.Fatal(err)
	}
	if bypassed.CacheUse().Kind() != UseUnavailableBypass ||
		!bytes.Equal(
			bypassed.CanonicalBytes(),
			fixture.result.CanonicalBytes(),
		) {
		t.Fatal("unavailable cache changed exact neighborhood semantics")
	}
}

func TestAtomicStoreConcurrentReadsRemainDeterministic(t *testing.T) {
	fixture := newCacheFixture(
		t,
		"decision_rationale.v1",
		11,
		"6",
		2,
	)
	shell, err := NewShell(NewAtomicStore())
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Uint64
	compute := func(
		context.Context,
	) (neighborhood.ExactNeighborhood, error) {
		calls.Add(1)
		return fixture.result, nil
	}

	const readers = 48
	results := make(chan []byte, readers)
	errors := make(chan error, readers)
	var group sync.WaitGroup
	group.Add(readers)
	for range readers {
		go func() {
			defer group.Done()
			read, readErr := shell.ReadThrough(
				context.Background(),
				fixture.key,
				compute,
			)
			if readErr != nil {
				errors <- readErr
				return
			}
			results <- read.CanonicalBytes()
		}()
	}
	group.Wait()
	close(results)
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
	for result := range results {
		if !bytes.Equal(result, fixture.result.CanonicalBytes()) {
			t.Fatal("concurrent cache read changed canonical bytes")
		}
	}
	if calls.Load() == 0 || calls.Load() > readers {
		t.Fatalf("unexpected compute count %d", calls.Load())
	}
	read, err := shell.ReadThrough(
		context.Background(),
		fixture.key,
		compute,
	)
	if err != nil {
		t.Fatal(err)
	}
	if read.CacheUse().Kind() != UseHit {
		t.Fatalf("settled concurrent cache use = %q", read.CacheUse().Kind())
	}
}

func TestStoreFailureReturnsTypedUnstoredParity(t *testing.T) {
	fixture := newCacheFixture(
		t,
		"decision_rationale.v1",
		11,
		"6",
		2,
	)
	shell, err := NewShell(missUnstoredStore{})
	if err != nil {
		t.Fatal(err)
	}
	read, err := shell.ReadThrough(
		context.Background(),
		fixture.key,
		fixedCompute(fixture.result),
	)
	if err != nil {
		t.Fatal(err)
	}
	if read.CacheUse().Kind() != UseMissUnstored ||
		!bytes.Equal(
			read.CanonicalBytes(),
			fixture.result.CanonicalBytes(),
		) {
		t.Fatal("cache store failure changed exact result")
	}
}

type missUnstoredStore struct{}

func (missUnstoredStore) Lookup(
	context.Context,
	Key,
) LookupResult {
	return Miss{}
}

func (missUnstoredStore) Put(
	context.Context,
	Entry,
) StoreResult {
	return StoreUnavailable{}
}

type cacheFixture struct {
	key        Key
	request    neighborhood.NeighborhoodRequest
	result     neighborhood.ExactNeighborhood
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis
}

func newCacheFixture(
	t *testing.T,
	profileRaw string,
	graphRevision uint64,
	typeEnvFill string,
	maxItems uint32,
) cacheFixture {
	t.Helper()
	profileRef, err := neighborhood.ParseProjectionProfileRef(profileRaw)
	if err != nil {
		t.Fatal(err)
	}
	profile, found := neighborhood.LookupProjectionProfile(profileRef)
	if !found {
		t.Fatalf("profile %q is unavailable", profileRaw)
	}
	typeEnv := mustTypeEnv(t, typeEnvFill)
	entity := mustPersistedRef(t, typeEnv, "service:auth")
	contextRef, err := typedmemory.NewBoundedContextRef("context:project")
	if err != nil {
		t.Fatal(err)
	}
	view, err := neighborhood.NewNeighborhoodViewSpec(
		profile.Ref(),
		[]neighborhood.FacetKind{neighborhood.FacetProblems},
		neighborhood.DetailStandard,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	budget, err := neighborhood.NewReadBudgetBuilder().
		SetMaxFacets(1).
		SetMaxItemsPerFacet(maxItems).
		SetMaxRelationPathsPerItem(2).
		SetMaxCarrierExcerptCharacters(1024).
		SetMaxProvenanceDepth(3).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	request, err := neighborhood.NewNeighborhoodRequestBuilder().
		SetEntity(entity).
		SetContext(contextRef).
		SetTypeEnv(typeEnv).
		SetGraphRevision(typedmemory.NewGraphRevision(graphRevision)).
		SetView(view).
		SetBudget(budget).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	result := assembleEmptyNeighborhood(t, request, profile)
	project, err := projectledger.ParseProjectID("qnt_1234abcd")
	if err != nil {
		t.Fatal(err)
	}
	graphBasis := mustGraphBasis(t, project, graphRevision, "a")
	key, err := NewKey(
		project,
		request,
		mustDigest(t, "8"),
		graphBasis,
		mustProjectionSchema(t, profile.SchemaVersion()),
	)
	if err != nil {
		t.Fatal(err)
	}
	return cacheFixture{
		key:        key,
		request:    request,
		result:     result,
		graphBasis: graphBasis,
	}
}

func assembleEmptyNeighborhood(
	t *testing.T,
	request neighborhood.NeighborhoodRequest,
	profile neighborhood.ProjectionProfileDefinition,
) neighborhood.ExactNeighborhood {
	t.Helper()
	snapshot, err := neighborhood.NewSnapshotBasis(
		request.GraphRevision(),
		request.TypeEnv(),
		request.TypeEnv().Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	inputRef, err := neighborhood.NewProjectionInputRef(
		"canonical:cache-fixture",
	)
	if err != nil {
		t.Fatal(err)
	}
	inputDigest := mustDigest(t, "5")
	canonical, err := neighborhood.NewCanonicalInputCoordinate(
		inputRef,
		inputDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	itemInput, err := neighborhood.NewProjectionInputCoordinate(
		inputRef,
		inputDigest,
	)
	if err != nil {
		t.Fatal(err)
	}
	postures, valid := neighborhood.NewItemPostures(
		neighborhood.SemanticTypedActive,
		neighborhood.LifecycleActive,
		neighborhood.EvidenceUnknown,
		neighborhood.ProjectionCurrent,
	)
	if !valid {
		t.Fatal("fixture postures are invalid")
	}
	rootCoordinate, err := neighborhood.NewRootOutputCoordinate(
		request.Entity(),
	)
	if err != nil {
		t.Fatal(err)
	}
	rootText, err := neighborhood.NewReadableItemText(
		"Authentication service",
	)
	if err != nil {
		t.Fatal(err)
	}
	provenance, err := typedmemory.NewProvenanceRef("event:cache-root")
	if err != nil {
		t.Fatal(err)
	}
	root, err := neighborhood.NewProjectedRoot(
		rootCoordinate,
		rootText,
		postures,
		provenance,
	)
	if err != nil {
		t.Fatal(err)
	}
	rootBasis, err := neighborhood.NewDirectProjectionItemBasis(
		rootCoordinate,
		[]neighborhood.ProjectionInputCoordinate{itemInput},
		neighborhood.TransformFieldSelection,
		profile.IntentionalLosses(),
	)
	if err != nil {
		t.Fatal(err)
	}
	rootSource, err := neighborhood.NewRootProjectionSource(
		root,
		rootBasis,
	)
	if err != nil {
		t.Fatal(err)
	}
	facet, err := neighborhood.NewExactFacetInput(
		neighborhood.FacetProblems,
		itemInput,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	input, err := neighborhood.NewPinnedNeighborhoodInputBuilder().
		SetRequest(request).
		SetSnapshot(snapshot).
		SetRoot(rootSource).
		AddCanonicalInput(canonical).
		AddFacet(facet).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	result, err := neighborhood.Assemble(input)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mustVariantKey(
	t *testing.T,
	base cacheFixture,
	directoryDigest typedmemory.SHA256Digest,
	graphBasis projecttypeenvselection.ProjectGraphSnapshotBasis,
	projectionSchema ProjectionSchemaVersion,
) Key {
	t.Helper()
	key, err := NewKey(
		base.key.Project(),
		base.request,
		directoryDigest,
		graphBasis,
		projectionSchema,
	)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func fixedCompute(
	result neighborhood.ExactNeighborhood,
) Compute {
	return func(
		context.Context,
	) (neighborhood.ExactNeighborhood, error) {
		return result, nil
	}
}

func mustTypeEnv(
	t *testing.T,
	fill string,
) typedmemory.TypeEnvRef {
	t.Helper()
	digest := mustDigest(t, fill)
	ref, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func mustPersistedRef(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	raw string,
) typedmemory.PersistedRef {
	t.Helper()
	kindID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatal(err)
	}
	kind, err := typedmemory.NewRefKindRef(typeEnv, kindID)
	if err != nil {
		t.Fatal(err)
	}
	referenceID, err := typedmemory.NewReferenceID(raw)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := typedmemory.NewPersistedRef(kind, referenceID)
	if err != nil {
		t.Fatal(err)
	}
	return reference
}

func mustDigest(
	t *testing.T,
	fill string,
) typedmemory.SHA256Digest {
	t.Helper()
	raw := strings.Repeat(fill, 64)
	digest, err := typedmemory.NewSHA256Digest("sha256:" + raw[:64])
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func mustGraphBasis(
	t *testing.T,
	project projectledger.ProjectID,
	revision uint64,
	fill string,
) projecttypeenvselection.ProjectGraphSnapshotBasis {
	t.Helper()
	identity, err := projectidentity.ParseProjectID(project.String())
	if err != nil {
		t.Fatal(err)
	}
	graphRevision := typedmemory.NewGraphRevision(revision)
	if revision == 0 {
		return mustSealGraphBasis(
			t,
			identity,
			graphRevision,
			projecttypeenvselection.EmptyProjectGraphClosure{},
		)
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(
		"typed-memory-event:" + strings.Repeat(fill, 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(
		"typed-memory-commit:" + strings.Repeat(fill, 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	closure, err := projecttypeenvselection.NewCommittedProjectGraphClosure(
		projecttypeenvselection.CommittedProjectGraphClosureInput{
			Event:                 event,
			Commit:                commit,
			MaterializationDigest: mustDigest(t, fill),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return mustSealGraphBasis(t, identity, graphRevision, closure)
}

func mustSealGraphBasis(
	t *testing.T,
	project projectidentity.ProjectID,
	revision typedmemory.GraphRevision,
	closure projecttypeenvselection.ProjectGraphClosure,
) projecttypeenvselection.ProjectGraphSnapshotBasis {
	t.Helper()
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

func mustProjectionSchema(
	t *testing.T,
	raw string,
) ProjectionSchemaVersion {
	t.Helper()
	version, err := NewProjectionSchemaVersion(raw)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
