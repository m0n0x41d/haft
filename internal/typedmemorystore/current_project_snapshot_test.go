package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type nilableCurrentSnapshotTypeEnvLoader struct{}

func (*nilableCurrentSnapshotTypeEnvLoader) LoadTypeEnv(
	TypeEnvSnapshot,
) (typedmemory.TypeEnv, typedmemory.CodecRegistry, error) {
	return typedmemory.TypeEnv{}, typedmemory.CodecRegistry{}, nil
}

func TestNewSQLiteCurrentProjectSnapshotLoaderIsReadOnlyAndExact(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	loader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    fixture.registry,
	}
	reader, err := NewSQLiteCurrentProjectSnapshotLoader(fixture.base.database, loader)
	if err != nil {
		t.Fatalf("NewSQLiteCurrentProjectSnapshotLoader: %v", err)
	}
	if _, writable := reader.(CommitPort); writable {
		t.Fatal("read-only current snapshot loader exposes CommitPort")
	}
	if _, writable := reader.(SnapshotPort); writable {
		t.Fatal("read-only current snapshot loader exposes TypeEnv SnapshotPort")
	}

	loaded, err := reader.LoadCurrentProjectSnapshot(context.Background(), fixture.base.project)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot: %v", err)
	}
	if loaded.ProjectID() != fixture.base.project {
		t.Fatalf("loaded project = %q; want %q", loaded.ProjectID().String(), fixture.base.project.String())
	}
	if loaded.Environment().Ref() != fixture.environment.Ref() {
		t.Fatal("read-only constructor loaded a different active TypeEnv")
	}
	snapshot := loaded.Snapshot()
	if snapshot.GraphRevision().Value() != 2 {
		t.Fatalf("read-only snapshot revision = %d; want 2", snapshot.GraphRevision().Value())
	}
	assertBoundAliasResolution(
		t,
		snapshot.ResolveAlias(fixture.oldAlias, fixture.primary),
		fixture.anchor,
	)
	assertActiveAssertionState(t, snapshot.AssertionState(fixture.oldAssertion), fixture.oldAssertion)
}

func TestNewSQLiteCurrentProjectSnapshotLoaderRejectsMissingDependencies(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	validLoader := staticTypeEnvLoader{
		reference:   fixture.environment.Ref(),
		environment: fixture.environment,
		registry:    fixture.registry,
	}
	var nilDatabase *sql.DB
	reader, err := NewSQLiteCurrentProjectSnapshotLoader(nilDatabase, validLoader)
	if !errors.Is(err, ErrDatabaseRequired) || reader != nil {
		t.Fatalf("nil database result = (%T, %v); want nil, ErrDatabaseRequired", reader, err)
	}
	reader, err = NewSQLiteCurrentProjectSnapshotLoader(fixture.database, nil)
	if !errors.Is(err, ErrTypeEnvLoaderRequired) || reader != nil {
		t.Fatalf("nil loader result = (%T, %v); want nil, ErrTypeEnvLoaderRequired", reader, err)
	}
	var typedNil *nilableCurrentSnapshotTypeEnvLoader
	reader, err = NewSQLiteCurrentProjectSnapshotLoader(fixture.database, typedNil)
	if !errors.Is(err, ErrTypeEnvLoaderRequired) || reader != nil {
		t.Fatalf("typed-nil loader result = (%T, %v); want nil, ErrTypeEnvLoaderRequired", reader, err)
	}
}

func TestCurrentProjectSnapshotFailsClosedBeforeSQLiteRead(t *testing.T) {
	var nilAdapter *SQLiteAdapter
	project := mustProjectID(t, "qnt_a7f3b2c1")
	_, err := nilAdapter.LoadCurrentProjectSnapshot(context.Background(), project)
	if !errors.Is(err, ErrDatabaseRequired) {
		t.Fatalf("nil adapter error = %v; want ErrDatabaseRequired", err)
	}

	fixture := newSQLiteStoreFixture(t)
	withoutLoader := &SQLiteAdapter{database: fixture.database}
	_, err = withoutLoader.LoadCurrentProjectSnapshot(context.Background(), project)
	if !errors.Is(err, ErrTypeEnvLoaderRequired) {
		t.Fatalf("missing loader error = %v; want ErrTypeEnvLoaderRequired", err)
	}

	var missingProject projectledger.ProjectID
	_, err = fixture.adapter.LoadCurrentProjectSnapshot(context.Background(), missingProject)
	if err == nil || !strings.Contains(err.Error(), "project identity is required") {
		t.Fatalf("missing project error = %v; want pre-SQL project identity rejection", err)
	}
}

func TestCurrentProjectSnapshotLoadsExactIdentityAndAssertionState(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)

	loaded, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot: %v", err)
	}
	if loaded.ProjectID() != fixture.base.project {
		t.Fatalf("ProjectID = %q; want %q", loaded.ProjectID().String(), fixture.base.project.String())
	}
	if loaded.Environment().Ref() != fixture.environment.Ref() {
		t.Fatal("loaded environment does not match the active TypeEnv")
	}
	snapshot := loaded.Snapshot()
	if snapshot.GraphRevision().Value() != 2 {
		t.Fatalf("GraphRevision = %d; want 2", snapshot.GraphRevision().Value())
	}
	if snapshot.TypeEnvRef() != fixture.environment.Ref() {
		t.Fatal("snapshot TypeEnv does not match the active TypeEnv")
	}
	assertExactEntityResolution(t, snapshot.ResolveEntity(fixture.anchor, fixture.primary), fixture.anchor)
	assertAbsentEntityResolution(t, snapshot.ResolveEntity(fixture.anchor, fixture.secondary), fixture.anchor)
	foreignContext := mustContextRef(t, "ctx:not-in-active-typeenv")
	unknownEntity := snapshot.ResolveEntity(fixture.anchor, foreignContext)
	queryCorrelatedUnknown, ok := unknownEntity.(typedmemory.UnknownEntityResolution)
	if !ok || queryCorrelatedUnknown.Entity() != fixture.anchor ||
		queryCorrelatedUnknown.Context() != foreignContext ||
		len(queryCorrelatedUnknown.MissingBasis()) != 1 {
		t.Fatalf("foreign-context entity resolution = %T; want correlated UnknownEntityResolution", unknownEntity)
	}
	assertBoundAliasResolution(t, snapshot.ResolveAlias(fixture.oldAlias, fixture.primary), fixture.anchor)
	unknownAlias := mustGenericAlias(t, "unknown-anchor")
	assertUnboundAliasResolution(t, snapshot.ResolveAlias(unknownAlias, fixture.primary), unknownAlias)
	foreignAlias := snapshot.ResolveAlias(unknownAlias, foreignContext)
	queryCorrelatedUnsettled, ok := foreignAlias.(typedmemory.UnsettledAliasResolution)
	if !ok || queryCorrelatedUnsettled.Alias() != unknownAlias ||
		queryCorrelatedUnsettled.Context() != foreignContext ||
		len(queryCorrelatedUnsettled.MissingBasis()) != 1 {
		t.Fatalf("foreign-context alias resolution = %T; want correlated UnsettledAliasResolution", foreignAlias)
	}
	assertActiveAssertionState(t, snapshot.AssertionState(fixture.oldAssertion), fixture.oldAssertion)
	unknownAssertion := mustGenericAssertionID(t, "assertion:unknown")
	assertAbsentAssertionState(t, snapshot.AssertionState(unknownAssertion), unknownAssertion)
}

func TestCurrentProjectSnapshotResolvesExactPersistedReferenceFromSameRevision(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	revision := typedmemory.NewGraphRevision(7)
	basis, err := snapshotResolutionBasis(fixture.base.project, revision)
	if err != nil {
		t.Fatalf("snapshotResolutionBasis: %v", err)
	}
	referenceRepair, err := typedmemory.NewRepairPointer(referenceSnapshotRepair)
	if err != nil {
		t.Fatalf("NewRepairPointer: %v", err)
	}
	entity := mustGenericEntityID(t, "entity:authorization-service")
	referenceID, err := typedmemory.NewReferenceID(entity.String())
	if err != nil {
		t.Fatalf("NewReferenceID: %v", err)
	}
	reference, err := typedmemory.NewPersistedRef(
		fixture.entityRefKind,
		referenceID,
	)
	if err != nil {
		t.Fatalf("NewPersistedRef: %v", err)
	}
	snapshot := &currentMemorySnapshot{
		project:         fixture.base.project,
		revision:        revision,
		typeEnv:         fixture.environment.Ref(),
		environment:     fixture.environment,
		resolutionBasis: basis,
		referenceRepair: referenceRepair,
		entityContexts: map[entityContextKey]struct{}{
			{entity: entity, context: fixture.base.context}: {},
		},
	}
	resolution := snapshot.ResolveReference(reference, fixture.base.context)
	resolved, ok := resolution.(typedmemory.ResolvedStrongReference)
	if !ok {
		t.Fatalf("ResolveReference = %T; want ResolvedStrongReference", resolution)
	}
	if resolved.Reference().ReferenceKey() != reference.ReferenceKey() ||
		resolved.Entity() != entity ||
		resolved.Context() != fixture.base.context ||
		resolved.Basis() != basis {
		t.Fatal("resolved persisted reference lost exact snapshot correlation")
	}

	absentID, err := typedmemory.NewReferenceID("entity:absent")
	if err != nil {
		t.Fatalf("NewReferenceID(absent): %v", err)
	}
	absentRef, err := typedmemory.NewPersistedRef(fixture.entityRefKind, absentID)
	if err != nil {
		t.Fatalf("NewPersistedRef(absent): %v", err)
	}
	if _, ok := snapshot.ResolveReference(
		absentRef,
		fixture.base.context,
	).(typedmemory.UnresolvedStrongReference); !ok {
		t.Fatal("absent persisted entity did not remain unresolved")
	}
}

func TestCurrentProjectSnapshotDoesNotCloseWorldOutsideActiveTypeEnv(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	loaded, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot: %v", err)
	}
	snapshot := loaded.Snapshot()
	foreignContext := mustContextRef(t, "ctx:outside-active-typeenv")
	entity := mustGenericEntityID(t, "entity:not-observed")
	resolution := snapshot.ResolveEntity(entity, foreignContext)
	unknown, ok := resolution.(typedmemory.UnknownEntityResolution)
	if !ok || unknown.Entity() != entity || unknown.Context() != foreignContext {
		t.Fatalf("outside-TypeEnv entity resolution = %T; want query-correlated UnknownEntityResolution", resolution)
	}
	if len(unknown.MissingBasis()) != 1 {
		t.Fatalf("outside-TypeEnv entity missing basis = %#v; want one exact missing basis", unknown.MissingBasis())
	}
	alias := mustGenericAlias(t, "outside-active-typeenv")
	availability := snapshot.ResolveAlias(alias, foreignContext)
	unsettled, ok := availability.(typedmemory.UnsettledAliasResolution)
	if !ok || unsettled.Alias() != alias || unsettled.Context() != foreignContext {
		t.Fatalf("outside-TypeEnv alias resolution = %T; want query-correlated UnsettledAliasResolution", availability)
	}
	if len(unsettled.MissingBasis()) != 1 {
		t.Fatalf("outside-TypeEnv alias missing basis = %#v; want one exact missing basis", unsettled.MissingBasis())
	}
}

func TestCurrentProjectSnapshotKeepsUnsupportedQueriesCorrelated(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	loaded, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot: %v", err)
	}
	snapshot := loaded.Snapshot()

	refKindID, err := typedmemory.NewRefKindID("U.TestRef")
	if err != nil {
		t.Fatalf("NewRefKindID: %v", err)
	}
	refKind, err := typedmemory.NewRefKindRef(fixture.environment.Ref(), refKindID)
	if err != nil {
		t.Fatalf("NewRefKindRef: %v", err)
	}
	referenceID, err := typedmemory.NewReferenceID("entity:anchor")
	if err != nil {
		t.Fatalf("NewReferenceID: %v", err)
	}
	reference, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef: %v", err)
	}
	resolution := snapshot.ResolveReference(reference, fixture.primary)
	unresolved, ok := resolution.(typedmemory.UnresolvedStrongReference)
	if !ok {
		t.Fatalf("ResolveReference = %T; want UnresolvedStrongReference", resolution)
	}
	if unresolved.Reference().ReferenceKey() != reference.ReferenceKey() ||
		unresolved.Reference().RefKind() != reference.RefKind() ||
		unresolved.Context() != fixture.primary {
		t.Fatal("unresolved reference is not correlated to the exact query")
	}
	if unresolved.Repair().String() != referenceSnapshotRepair {
		t.Fatalf("reference repair = %q; want %q", unresolved.Repair().String(), referenceSnapshotRepair)
	}

	query, err := typedmemory.NewMemberOfQuery(
		fixture.anchor,
		fixture.textKind,
		fixture.contextSlice,
	)
	if err != nil {
		t.Fatalf("NewMemberOfQuery: %v", err)
	}
	view, err := typedmemory.NewPersistedSnapshotView(
		fixture.environment.Ref(),
		typedmemory.NewGraphRevision(2),
	)
	if err != nil {
		t.Fatalf("NewPersistedSnapshotView: %v", err)
	}
	request, err := typedmemory.NewMemberOfEvaluationRequest(query, view)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest: %v", err)
	}
	judgement := snapshot.EvaluateMemberOf(request)
	undefined, ok := judgement.(typedmemory.MemberOfUndefined)
	if !ok {
		t.Fatalf("EvaluateMemberOf = %T; want MemberOfUndefined", judgement)
	}
	if undefined.Query().Digest() != query.Digest() {
		t.Fatal("undefined MemberOf judgement is not correlated to the exact query")
	}
	missing := undefined.MissingBasis()
	if len(missing) != 1 || missing[0].Kind() != typedmemory.MissingMemberOfKindSignature {
		t.Fatalf("missing basis = %#v; want exact KindSignature basis", missing)
	}
	if undefined.Repair().String() != memberOfSnapshotRepair {
		t.Fatalf("MemberOf repair = %q; want %q", undefined.Repair().String(), memberOfSnapshotRepair)
	}
}

func TestCurrentProjectSnapshotIsImmutableAcrossLaterCommit(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	before, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot(before): %v", err)
	}
	candidate := fixture.finalCandidate(t, "New entity", "new payload")
	request := fixture.finalRequest(t, "current-snapshot-final", candidate)
	if _, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	after, err := fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if err != nil {
		t.Fatalf("LoadCurrentProjectSnapshot(after): %v", err)
	}

	beforeSnapshot := before.Snapshot()
	if beforeSnapshot.GraphRevision().Value() != 2 {
		t.Fatalf("old snapshot revision = %d; want 2", beforeSnapshot.GraphRevision().Value())
	}
	assertBoundAliasResolution(
		t,
		beforeSnapshot.ResolveAlias(fixture.oldAlias, fixture.primary),
		fixture.anchor,
	)
	assertActiveAssertionState(t, beforeSnapshot.AssertionState(fixture.oldAssertion), fixture.oldAssertion)

	afterSnapshot := after.Snapshot()
	if afterSnapshot.GraphRevision().Value() != 3 {
		t.Fatalf("new snapshot revision = %d; want 3", afterSnapshot.GraphRevision().Value())
	}
	assertUnboundAliasResolution(
		t,
		afterSnapshot.ResolveAlias(fixture.oldAlias, fixture.primary),
		fixture.oldAlias,
	)
	replacement := mustGenericAlias(t, "replacement-anchor")
	assertBoundAliasResolution(
		t,
		afterSnapshot.ResolveAlias(replacement, fixture.primary),
		fixture.anchor,
	)
	assertRetractedAssertionState(t, afterSnapshot.AssertionState(fixture.oldAssertion), fixture.oldAssertion)
	newAssertion := mustGenericAssertionID(t, "assertion:new")
	assertActiveAssertionState(t, afterSnapshot.AssertionState(newAssertion), newAssertion)
	newEntity := mustGenericEntityID(t, "entity:new")
	assertExactEntityResolution(t, afterSnapshot.ResolveEntity(newEntity, fixture.primary), newEntity)
}

func TestCurrentProjectSnapshotDistinguishesMissingHeadFromCorruption(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	missingProject := mustProjectID(t, "qnt_b7f3b2c1")
	_, err := fixture.adapter.LoadCurrentProjectSnapshot(context.Background(), missingProject)
	if !errors.Is(err, ErrProjectNotInitialized) {
		t.Fatalf("missing project error = %v; want ErrProjectNotInitialized", err)
	}

	mixed := newGenericMixedStoreFixture(t)
	if _, err := mixed.base.database.Exec(
		`DROP TRIGGER typed_memory_commit_materialization_closures_v46_no_delete`,
	); err != nil {
		t.Fatalf("drop closure immutability trigger: %v", err)
	}
	if _, err := mixed.base.database.Exec(
		`DELETE FROM typed_memory_commit_materialization_closures
		WHERE project_id = ? AND event_ref = (
			SELECT last_event_ref FROM typed_memory_graph_heads WHERE project_id = ?
		)`,
		mixed.base.project.String(),
		mixed.base.project.String(),
	); err != nil {
		t.Fatalf("delete head closure: %v", err)
	}
	_, err = mixed.adapter.LoadCurrentProjectSnapshot(context.Background(), mixed.base.project)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("partial v46 error = %v; want ErrStoredAdmissionIntegrity", err)
	}

	missingPair := newGenericMixedStoreFixture(t)
	if _, err := missingPair.base.database.Exec(
		`DROP TRIGGER typed_memory_commit_materialization_closures_v46_no_delete`,
	); err != nil {
		t.Fatalf("drop closure immutability trigger for missing-pair fixture: %v", err)
	}
	if _, err := missingPair.base.database.Exec(
		`DROP TRIGGER typed_memory_event_admission_bases_v46_no_delete`,
	); err != nil {
		t.Fatalf("drop basis immutability trigger for missing-pair fixture: %v", err)
	}
	if _, err := missingPair.base.database.Exec(
		`DELETE FROM typed_memory_commit_materialization_closures
		WHERE project_id = ? AND event_ref = (
			SELECT event_ref FROM typed_memory_graph_events
			WHERE project_id = ? AND graph_revision = 1
		)`,
		missingPair.base.project.String(),
		missingPair.base.project.String(),
	); err != nil {
		t.Fatalf("delete head closure for missing-pair fixture: %v", err)
	}
	if _, err := missingPair.base.database.Exec(
		`DELETE FROM typed_memory_event_admission_bases
		WHERE project_id = ? AND event_ref = (
			SELECT event_ref FROM typed_memory_graph_events
			WHERE project_id = ? AND graph_revision = 1
		)`,
		missingPair.base.project.String(),
		missingPair.base.project.String(),
	); err != nil {
		t.Fatalf("delete head basis for missing-pair fixture: %v", err)
	}
	_, err = missingPair.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		missingPair.base.project,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("missing v46 pair error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestCurrentProjectSnapshotRejectsMissingV46PairWithoutTopLevelProjection(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	var eventRef string
	err := fixture.base.database.QueryRow(
		`SELECT event_ref FROM typed_memory_graph_events
		WHERE project_id = ? AND graph_revision = 1`,
		fixture.base.project.String(),
	).Scan(&eventRef)
	if err != nil {
		t.Fatalf("load first v46 event ref: %v", err)
	}
	removeV46DeclarationGeneration(
		t,
		fixture.base.database,
		fixture.base.project.String(),
		eventRef,
		fixture.anchor.String(),
	)

	_, err = fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("missing v46 pair without top-level projection error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestCurrentProjectSnapshotRejectsCorruptedV46CarrierBytes(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	if _, err := fixture.base.database.Exec(
		`DROP TRIGGER typed_memory_event_admission_bases_v46_no_update`,
	); err != nil {
		t.Fatalf("drop admission carrier immutability trigger: %v", err)
	}
	result, err := fixture.base.database.Exec(
		`UPDATE typed_memory_event_admission_bases
		SET canonical_request_bytes = ?
		WHERE project_id = ? AND event_ref = (
			SELECT last_event_ref FROM typed_memory_graph_heads WHERE project_id = ?
		)`,
		[]byte("corrupted snapshot request carrier"),
		fixture.base.project.String(),
		fixture.base.project.String(),
	)
	if err != nil {
		t.Fatalf("corrupt stored request carrier: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "snapshot request carrier")

	_, err = fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("corrupted v46 carrier error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestCurrentProjectSnapshotRejectsMissingWriterGenerationMarker(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	if _, err := fixture.base.database.Exec(
		`DROP TRIGGER typed_memory_event_writer_generations_v46_no_delete`,
	); err != nil {
		t.Fatalf("drop writer-generation immutability trigger: %v", err)
	}
	result, err := fixture.base.database.Exec(
		`DELETE FROM typed_memory_event_writer_generations
		WHERE project_id = ? AND event_ref = (
			SELECT event_ref FROM typed_memory_graph_events
			WHERE project_id = ? AND graph_revision = 1
		)`,
		fixture.base.project.String(),
		fixture.base.project.String(),
	)
	if err != nil {
		t.Fatalf("delete writer-generation marker: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "writer-generation marker")

	_, err = fixture.adapter.LoadCurrentProjectSnapshot(
		context.Background(),
		fixture.base.project,
	)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("missing writer-generation marker error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestCurrentProjectSnapshotRejectsQueryVisibleMaterializationDrift(t *testing.T) {
	wanted := map[string]bool{
		"alias_provenance_ref":    true,
		"relation_provenance_ref": true,
		"retraction_reason":       true,
	}
	for _, fault := range decomposedProjectionColumnFaults() {
		if !wanted[fault.name] {
			continue
		}
		t.Run(fault.name, func(t *testing.T) {
			fixture := fault.newFixture(t, "snapshot-"+fault.name)
			receipt, err := fixture.adapter.CommitMemoryChangeSet(
				context.Background(),
				fixture.request,
			)
			if err != nil {
				t.Fatalf("seed snapshot materialization: %v", err)
			}
			allowDecomposedProjectionMutation(t, fixture.database, fault.updates)
			applyDurableDecomposedProjectionFault(t, fixture, receipt.EventRef(), fault)

			_, err = fixture.adapter.LoadCurrentProjectSnapshot(
				context.Background(),
				fixture.project,
			)
			if !errors.Is(err, ErrStoredAdmissionIntegrity) {
				t.Fatalf(
					"query-visible snapshot drift error = %v; want ErrStoredAdmissionIntegrity",
					err,
				)
			}
		})
	}
}

func TestCurrentProjectSnapshotRejectsUnexecutableActiveTypeEnv(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	wanted := errors.New("loader rejected active TypeEnv")
	adapter, err := newDeclarationSQLiteAdapter(
		fixture.database,
		rejectingTypeEnvLoader{err: wanted},
		fixture.clock,
	)
	if err != nil {
		t.Fatalf("newDeclarationSQLiteAdapter: %v", err)
	}
	_, err = adapter.LoadCurrentProjectSnapshot(context.Background(), fixture.project)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) || !errors.Is(err, wanted) {
		t.Fatalf("unexecutable TypeEnv error = %v; want integrity plus loader cause", err)
	}
}

func assertExactEntityResolution(
	t *testing.T,
	resolution typedmemory.EntityResolution,
	want typedmemory.EntityID,
) {
	t.Helper()
	exact, ok := resolution.(typedmemory.ExactEntityResolution)
	if !ok || exact.Entity() != want {
		t.Fatalf("entity resolution = %T; want ExactEntityResolution for %s", resolution, want.String())
	}
}

func assertAbsentEntityResolution(
	t *testing.T,
	resolution typedmemory.EntityResolution,
	want typedmemory.EntityID,
) {
	t.Helper()
	absent, ok := resolution.(typedmemory.AbsentEntityResolution)
	if !ok || absent.Entity() != want {
		t.Fatalf("entity resolution = %T; want AbsentEntityResolution for %s", resolution, want.String())
	}
}

func assertBoundAliasResolution(
	t *testing.T,
	resolution typedmemory.AliasAvailability,
	want typedmemory.EntityID,
) {
	t.Helper()
	bound, ok := resolution.(typedmemory.BoundAliasResolution)
	if !ok || bound.Entity() != want {
		t.Fatalf("alias resolution = %T; want BoundAliasResolution for %s", resolution, want.String())
	}
}

func assertUnboundAliasResolution(
	t *testing.T,
	resolution typedmemory.AliasAvailability,
	want typedmemory.EntityAlias,
) {
	t.Helper()
	unbound, ok := resolution.(typedmemory.UnboundAliasResolution)
	if !ok || unbound.Alias() != want {
		t.Fatalf("alias resolution = %T; want UnboundAliasResolution for %s", resolution, want.String())
	}
}

func assertActiveAssertionState(
	t *testing.T,
	state typedmemory.AssertionState,
	want typedmemory.AssertionID,
) {
	t.Helper()
	active, ok := state.(typedmemory.ActiveAssertion)
	if !ok || active.Assertion() != want {
		t.Fatalf("assertion state = %T; want ActiveAssertion for %s", state, want.String())
	}
}

func assertAbsentAssertionState(
	t *testing.T,
	state typedmemory.AssertionState,
	want typedmemory.AssertionID,
) {
	t.Helper()
	absent, ok := state.(typedmemory.AbsentAssertionState)
	if !ok || absent.Assertion() != want {
		t.Fatalf("assertion state = %T; want AbsentAssertionState for %s", state, want.String())
	}
}

func assertRetractedAssertionState(
	t *testing.T,
	state typedmemory.AssertionState,
	want typedmemory.AssertionID,
) {
	t.Helper()
	retracted, ok := state.(typedmemory.RetractedAssertionState)
	if !ok || retracted.Assertion() != want {
		t.Fatalf("assertion state = %T; want RetractedAssertionState for %s", state, want.String())
	}
}
