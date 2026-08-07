package typedmemorystore

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestGenericCommitPersistsSnapshotOnlyMixedBatchAndExactReplay(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "New entity", "new payload")
	request := fixture.finalRequest(t, "generic-mixed", candidate)

	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	if receipt.Disposition() != CommitApplied || receipt.GraphRevision().Value() != 3 {
		t.Fatalf("receipt = %#v; want applied revision 3", receipt)
	}

	replay, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("replay CommitMemoryChangeSet: %v", err)
	}
	if replay.Disposition() != CommitReplay ||
		replay.EventRef() != receipt.EventRef() ||
		replay.CommitRef() != receipt.CommitRef() ||
		replay.ResultDigest() != receipt.ResultDigest() {
		t.Fatalf("replay = %#v; want exact original receipt %#v", replay, receipt)
	}

	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_graph_events":                    3,
		"typed_memory_graph_commits":                   3,
		"typed_memory_event_admission_bases":           3,
		"typed_memory_commit_materialization_closures": 3,
		"typed_memory_entities":                        2,
		"typed_memory_entity_contexts":                 2,
		"typed_memory_alias_changes":                   3,
		"typed_memory_context_slices":                  2,
		"typed_memory_value_blobs":                     2,
		"typed_memory_relation_instances":              0,
		"typed_memory_relation_slots":                  0,
		"typed_memory_relation_fillers":                0,
		"typed_memory_relational_assertions_v3":        2,
		"typed_memory_relational_assertion_slots_v3":   2,
		"typed_memory_relational_assertion_fillers_v3": 2,
		"typed_memory_assertion_retractions":           1,
		"typed_memory_reference_resolution_uses":       0,
		"typed_memory_memberof_evaluations":            0,
		"typed_memory_memberof_observable_inputs":      0,
		"typed_memory_observable_input_blobs":          0,
	})
	assertMixedAliasLineage(t, fixture, receipt.EventRef())
	var storedRequestProvenance string
	err = fixture.base.database.QueryRow(
		`SELECT request_provenance_ref FROM typed_memory_graph_events
		WHERE project_id = ? AND event_ref = ?`,
		fixture.base.project.String(),
		receipt.EventRef(),
	).Scan(&storedRequestProvenance)
	if err != nil {
		t.Fatalf("load mixed request provenance: %v", err)
	}
	if storedRequestProvenance != mustRequestProvenanceRef(t).String() {
		t.Fatalf(
			"mixed request provenance = %q; want explicit application provenance %q",
			storedRequestProvenance,
			mustRequestProvenanceRef(t).String(),
		)
	}
}

func TestGenericCommitAtomicallyDeclaresEntityWithAliasesAndReplays(
	t *testing.T,
) {
	fixture := newGenericMixedStoreFixture(t)
	entity := mustGenericEntityID(t, "entity:first-concern")
	firstAlias := mustGenericAlias(t, "first concern")
	secondAlias := mustGenericAlias(t, "primary concern")
	declaration := fixture.declarationChange(
		t,
		entity,
		"local:first-concern",
		fixture.primary,
		"First concern",
		"memory:test:first-concern",
	)
	firstAliasChange := genericAliasAdmissionForEntity(
		t,
		entity,
		firstAlias,
		fixture.primary,
		"memory:test:first-concern-alias",
	)
	secondAliasChange := genericAliasAdmissionForEntity(
		t,
		entity,
		secondAlias,
		fixture.primary,
		"memory:test:primary-concern-alias",
	)
	candidate, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{
			declaration,
			firstAliasChange,
			secondAliasChange,
		},
	)
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}
	request := fixture.requestAt(
		t,
		2,
		"generic-first-concern",
		candidate,
		func(snapshot *genericMixedSnapshot) {
			snapshot.entityAbsent(t, entity, fixture.primary)
			snapshot.aliasUnbound(t, firstAlias, fixture.primary)
			snapshot.aliasUnbound(t, secondAlias, fixture.primary)
		},
	)

	receipt, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	)
	if err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	if receipt.Disposition() != CommitApplied ||
		receipt.GraphRevision().Value() != 3 {
		t.Fatalf("receipt = %#v; want applied revision 3", receipt)
	}
	replay, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	)
	if err != nil {
		t.Fatalf("replay CommitMemoryChangeSet: %v", err)
	}
	if replay.Disposition() != CommitReplay ||
		replay.EventRef() != receipt.EventRef() ||
		replay.CommitRef() != receipt.CommitRef() ||
		replay.ResultDigest() != receipt.ResultDigest() {
		t.Fatalf("replay = %#v; want exact original receipt %#v", replay, receipt)
	}

	var aliases int64
	err = fixture.base.database.QueryRow(
		`SELECT COUNT(*)
		FROM typed_memory_alias_changes
		WHERE project_id = ? AND event_ref = ? AND entity_id = ?`,
		fixture.base.project.String(),
		receipt.EventRef(),
		entity.String(),
	).Scan(&aliases)
	if err != nil {
		t.Fatalf("load same-batch alias rows: %v", err)
	}
	if aliases != 2 {
		t.Fatalf("same-batch alias rows = %d; want 2", aliases)
	}
	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_graph_events":                    3,
		"typed_memory_graph_commits":                   3,
		"typed_memory_event_admission_bases":           3,
		"typed_memory_commit_materialization_closures": 3,
		"typed_memory_entities":                        2,
		"typed_memory_entity_contexts":                 2,
		"typed_memory_alias_changes":                   3,
	})
}

func TestGenericIdempotencyReplayRecoversHiddenOriginalBasisAndConflicts(
	t *testing.T,
) {
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "New entity", "new payload")
	request := fixture.finalRequest(t, "generic-keyed-replay", candidate)
	committed, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	)
	if err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	replayRequest, err := NewIdempotencyReplayRequestBuilder().
		SetContractVersion(request.contractVersion).
		SetProject(request.project).
		SetIdempotencyKey(request.idempotencyKey).
		SetRequestProvenance(request.requestProvenance).
		SetCandidate(request.candidate).
		Build()
	if err != nil {
		t.Fatalf("Build idempotency replay request: %v", err)
	}
	replayed, found, err :=
		fixture.adapter.ReplayMemoryChangeSetByIdempotencyKey(
			context.Background(),
			replayRequest,
		)
	if err != nil {
		t.Fatalf("ReplayMemoryChangeSetByIdempotencyKey: %v", err)
	}
	if !found ||
		replayed.Disposition() != CommitReplay ||
		replayed.EventRef() != committed.EventRef() ||
		replayed.CommitRef() != committed.CommitRef() {
		t.Fatalf("keyed replay = (%#v, %v), want original receipt", replayed, found)
	}

	conflictingProvenance := mustGenericProvenanceRef(
		t,
		"memory:test:generic-keyed-replay-conflict",
	)
	conflictingRequest, err := NewIdempotencyReplayRequestBuilder().
		SetContractVersion(request.contractVersion).
		SetProject(request.project).
		SetIdempotencyKey(request.idempotencyKey).
		SetRequestProvenance(conflictingProvenance).
		SetCandidate(request.candidate).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_, found, err = fixture.adapter.ReplayMemoryChangeSetByIdempotencyKey(
		context.Background(),
		conflictingRequest,
	)
	if !found || !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf(
			"conflicting keyed replay = (found=%v, err=%v), want occupied conflict",
			found,
			err,
		)
	}
}

func TestGenericCommitAliasRaceRollsBackSameBatchEntityDeclaration(
	t *testing.T,
) {
	fixture := newGenericMixedStoreFixture(t)
	entity := mustGenericEntityID(t, "entity:conflicting-concern")
	declaration := fixture.declarationChange(
		t,
		entity,
		"local:conflicting-concern",
		fixture.primary,
		"Conflicting concern",
		"memory:test:conflicting-concern",
	)
	aliasChange := genericAliasAdmissionForEntity(
		t,
		entity,
		fixture.oldAlias,
		fixture.primary,
		"memory:test:conflicting-concern-alias",
	)
	candidate, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{declaration, aliasChange},
	)
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}
	request := fixture.requestAt(
		t,
		2,
		"generic-conflicting-concern",
		candidate,
		func(snapshot *genericMixedSnapshot) {
			snapshot.entityAbsent(t, entity, fixture.primary)
			snapshot.aliasUnbound(t, fixture.oldAlias, fixture.primary)
		},
	)

	_, err = fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	)
	if !errors.Is(err, ErrRevalidationRejected) &&
		!errors.Is(err, ErrAdmissionEnvelopeMismatch) {
		t.Fatalf(
			"conflicting alias commit error = %v; want transaction rejection",
			err,
		)
	}
	var entityRows int64
	err = fixture.base.database.QueryRow(
		`SELECT COUNT(*)
		FROM typed_memory_entity_contexts
		WHERE project_id = ? AND entity_id = ?`,
		fixture.base.project.String(),
		entity.String(),
	).Scan(&entityRows)
	if err != nil {
		t.Fatalf("load rolled-back entity rows: %v", err)
	}
	if entityRows != 0 {
		t.Fatalf("rolled-back entity rows = %d; want 0", entityRows)
	}
	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_graph_events":                    2,
		"typed_memory_graph_commits":                   2,
		"typed_memory_event_admission_bases":           2,
		"typed_memory_commit_materialization_closures": 2,
		"typed_memory_entities":                        1,
		"typed_memory_entity_contexts":                 1,
		"typed_memory_alias_changes":                   1,
	})
}

func genericAliasAdmissionForEntity(
	t *testing.T,
	entity typedmemory.EntityID,
	alias typedmemory.EntityAlias,
	contextRef typedmemory.BoundedContextRef,
	provenance string,
) typedmemory.ApplyIdentityChange {
	t.Helper()
	change, err := typedmemory.NewAdmitAlias(
		entity,
		alias,
		contextRef,
		mustGenericProvenanceRef(t, provenance),
	)
	if err != nil {
		t.Fatalf("NewAdmitAlias: %v", err)
	}
	effect, err := typedmemory.NewApplyIdentityChange(change)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange: %v", err)
	}
	return effect
}

func TestGenericCommitTreatsDifferentExplicitRequestProvenanceAsConflict(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.finalCandidate(t, "New entity", "new payload")
	request := fixture.finalRequest(t, "generic-request-provenance", candidate)
	if _, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request); err != nil {
		t.Fatalf("seed explicit request provenance: %v", err)
	}
	different, err := typedmemory.NewProvenanceRef("memory:test:different-request-provenance")
	if err != nil {
		t.Fatalf("NewProvenanceRef(different request): %v", err)
	}
	request.requestProvenance = different
	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("different request provenance error = %v; want ErrIdempotencyConflict", err)
	}
}

func TestGenericReplayClassifiesCorruptedStoredRequestProvenanceAsIntegrityFailure(
	t *testing.T,
) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "generic-corrupt-request-provenance")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed explicit request provenance: %v", err)
	}
	if _, err := fixture.base.database.Exec(
		"DROP TRIGGER typed_memory_graph_events_no_update",
	); err != nil {
		t.Fatalf("allow graph-event corruption fixture: %v", err)
	}
	corrupted, err := typedmemory.NewProvenanceRef("memory:test:corrupted-request-provenance")
	if err != nil {
		t.Fatalf("NewProvenanceRef(corrupted request): %v", err)
	}
	result, err := fixture.base.database.Exec(
		`UPDATE typed_memory_graph_events
		SET request_provenance_ref = ?
		WHERE project_id = ? AND event_ref = ?`,
		corrupted.String(),
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("inject stored request-provenance corruption: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "stored graph-event provenance")

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("corrupted stored provenance error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("stored provenance corruption was misclassified as caller conflict: %v", err)
	}
}

func TestGenericCommitRejectsDifferentExactMixedReplayWithoutWrites(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	firstCandidate := fixture.finalCandidate(t, "New entity", "first payload")
	first := fixture.finalRequest(t, "generic-mixed-conflict", firstCandidate)
	if _, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), first); err != nil {
		t.Fatalf("seed mixed admission: %v", err)
	}

	conflictingCandidate := fixture.finalCandidate(t, "New entity", "other payload")
	conflicting := fixture.finalRequest(t, "generic-mixed-conflict", conflictingCandidate)
	_, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), conflicting)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting replay error = %v; want ErrIdempotencyConflict", err)
	}
	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_graph_events":                    3,
		"typed_memory_graph_commits":                   3,
		"typed_memory_event_admission_bases":           3,
		"typed_memory_commit_materialization_closures": 3,
		"typed_memory_entities":                        2,
		"typed_memory_entity_contexts":                 2,
		"typed_memory_alias_changes":                   3,
		"typed_memory_relation_instances":              0,
		"typed_memory_relational_assertions_v3":        2,
		"typed_memory_assertion_retractions":           1,
	})
}

func TestGenericCommitFailureRollsBackWholeMixedBatch(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	_, err := fixture.base.database.Exec(`CREATE TRIGGER generic_mixed_test_fail_retraction
		BEFORE INSERT ON typed_memory_assertion_retractions
		BEGIN
			SELECT RAISE(ABORT, 'generic mixed test retraction failure');
		END`)
	if err != nil {
		t.Fatalf("install failure trigger: %v", err)
	}
	candidate := fixture.finalCandidate(t, "New entity", "new payload")
	request := fixture.finalRequest(t, "generic-mixed-failure", candidate)
	if _, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request); err == nil {
		t.Fatal("mixed admission unexpectedly survived injected retraction failure")
	}

	head, err := fixture.adapter.LoadHead(context.Background(), fixture.base.project)
	if err != nil {
		t.Fatalf("LoadHead: %v", err)
	}
	if head.Revision().Value() != 2 {
		t.Fatalf("head revision = %d; want unchanged revision 2", head.Revision().Value())
	}
	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_graph_events":                    2,
		"typed_memory_graph_commits":                   2,
		"typed_memory_event_admission_bases":           2,
		"typed_memory_commit_materialization_closures": 2,
		"typed_memory_entities":                        1,
		"typed_memory_entity_contexts":                 1,
		"typed_memory_alias_changes":                   1,
		"typed_memory_context_slices":                  1,
		"typed_memory_value_blobs":                     1,
		"typed_memory_relation_instances":              0,
		"typed_memory_relation_slots":                  0,
		"typed_memory_relation_fillers":                0,
		"typed_memory_relational_assertions_v3":        1,
		"typed_memory_relational_assertion_slots_v3":   1,
		"typed_memory_relational_assertion_fillers_v3": 1,
		"typed_memory_assertion_retractions":           0,
	})
}

func TestGenericCommitDeclaresExistingGlobalEntityInAnotherContext(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	candidate := fixture.declarationCandidate(
		t,
		fixture.anchor,
		"local:anchor-secondary",
		fixture.secondary,
		"Anchor in secondary context",
		"memory:test:anchor-secondary",
	)
	request := fixture.requestAt(
		t,
		2,
		"generic-cross-context",
		candidate,
		func(snapshot *genericMixedSnapshot) {
			snapshot.entityAbsent(t, fixture.anchor, fixture.secondary)
		},
	)

	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("cross-context CommitMemoryChangeSet: %v", err)
	}
	if receipt.GraphRevision().Value() != 3 {
		t.Fatalf("cross-context revision = %d; want 3", receipt.GraphRevision().Value())
	}
	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_entities":        1,
		"typed_memory_entity_contexts": 2,
	})
}

func TestGenericCommitMergeAndSplitRemainZeroWriteWithoutSealedAdmission(t *testing.T) {
	fixture := newGenericMixedStoreFixture(t)
	basis := mustGenericReconciliationBasis(t, "reconciliation:test")
	merged := mustGenericEntityID(t, "entity:merged")
	merge, err := typedmemory.NewMergeEntities(
		fixture.anchor,
		[]typedmemory.EntityID{merged},
		fixture.primary,
		basis,
	)
	if err != nil {
		t.Fatalf("NewMergeEntities: %v", err)
	}
	splitA := mustGenericEntityID(t, "entity:split-a")
	splitB := mustGenericEntityID(t, "entity:split-b")
	split, err := typedmemory.NewSplitEntity(
		fixture.anchor,
		[]typedmemory.EntityID{splitA, splitB},
		fixture.primary,
		basis,
	)
	if err != nil {
		t.Fatalf("NewSplitEntity: %v", err)
	}

	for name, identity := range map[string]typedmemory.IdentityChange{
		"merge": merge,
		"split": split,
	} {
		t.Run(name, func(t *testing.T) {
			effect, err := typedmemory.NewApplyIdentityChange(identity)
			if err != nil {
				t.Fatalf("NewApplyIdentityChange: %v", err)
			}
			candidate, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{effect})
			if err != nil {
				t.Fatalf("NewMemoryChangeSet: %v", err)
			}
			snapshot := fixture.snapshotAt(t, 2)
			verdict := typedmemory.ValidateMemoryChangeSet(
				fixture.environment,
				fixture.registry,
				snapshot,
				candidate,
			)
			if verdict.Kind() != typedmemory.ValidationUnderdetermined {
				t.Fatalf("validation kind = %s; want underdetermined", verdict.Kind())
			}
			request := fixture.base.request(
				t,
				2,
				fixture.environment.Ref(),
				"generic-"+name+"-rejected",
				candidate,
			)
			_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
			if !errors.Is(err, ErrAdmissionBatchRequired) {
				t.Fatalf("CommitMemoryChangeSet error = %v; want ErrAdmissionBatchRequired", err)
			}
		})
	}
	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_graph_events":                    2,
		"typed_memory_graph_commits":                   2,
		"typed_memory_event_admission_bases":           2,
		"typed_memory_commit_materialization_closures": 2,
	})
}

type genericMixedStoreFixture struct {
	base         sqliteStoreFixture
	environment  typedmemory.TypeEnv
	registry     typedmemory.CodecRegistry
	adapter      *SQLiteAdapter
	primary      typedmemory.BoundedContextRef
	secondary    typedmemory.BoundedContextRef
	textKind     typedmemory.ValueKindRef
	shape        typedmemory.ValueShapeRef
	codec        typedmemory.CodecRef
	signature    typedmemory.RelationSignatureRef
	payloadSlot  typedmemory.SlotKindID
	contextSlice typedmemory.ContextSlice
	anchor       typedmemory.EntityID
	oldAlias     typedmemory.EntityAlias
	oldAssertion typedmemory.AssertionID
}

func newGenericMixedStoreFixture(t *testing.T) genericMixedStoreFixture {
	t.Helper()
	fixture := newUnbootstrappedGenericMixedStoreFixture(t)
	fixture.bootstrap(t)
	return fixture
}

func newUnbootstrappedGenericMixedStoreFixture(t *testing.T) genericMixedStoreFixture {
	t.Helper()
	base := newSQLiteStoreFixture(t)
	provenance := mustFPFProvenance(t, base.snapshot.SourceRevision())
	primary, exists := base.environment.BoundedContext(base.context)
	if !exists {
		t.Fatal("base fixture primary context is missing")
	}
	secondaryRef := mustContextRef(t, "ctx:secondary")
	secondary, err := typedmemory.NewBoundedContext(secondaryRef, provenance)
	if err != nil {
		t.Fatalf("NewBoundedContext(secondary): %v", err)
	}
	textKindID := mustGenericKindID(t, "U.Text")
	textDefinition, err := typedmemory.NewKindDefinition(textKindID, provenance)
	if err != nil {
		t.Fatalf("NewKindDefinition: %v", err)
	}
	textKind, err := typedmemory.NewValueKindRef(base.environment.Ref(), textKindID)
	if err != nil {
		t.Fatalf("NewValueKindRef: %v", err)
	}
	admission := mustLocalContextKindAvailability(
		t,
		base.environment.Ref(),
		primary.Ref(),
		textKindID,
		provenance,
		"generic-mixed.availability",
	)
	shapeID := mustGenericShapeID(t, "TestTextShape")
	shape, err := typedmemory.NewScalarShape(typedmemory.ScalarText)
	if err != nil {
		t.Fatalf("NewScalarShape: %v", err)
	}
	shapeRef, err := typedmemory.DeriveValueShapeRef(shapeID, shape)
	if err != nil {
		t.Fatalf("DeriveValueShapeRef: %v", err)
	}
	shapeDeclaration, err := typedmemory.NewValueShapeDeclaration(shapeRef, shape, provenance)
	if err != nil {
		t.Fatalf("NewValueShapeDeclaration: %v", err)
	}
	codecRef, err := typedmemory.NewCodecRef(
		mustGenericCodecID(t, "TestTextCodec"),
		mustGenericCodecVersion(t, "1"),
		mustDigest(t, []byte("generic mixed text codec v1")),
	)
	if err != nil {
		t.Fatalf("NewCodecRef: %v", err)
	}
	binding, err := typedmemory.NewValueBinding(textKind, shapeRef, codecRef, provenance)
	if err != nil {
		t.Fatalf("NewValueBinding: %v", err)
	}
	payloadSlot := mustGenericSlotKindID(t, "payload")
	target, err := typedmemory.NewValueSlotTarget(textKind)
	if err != nil {
		t.Fatalf("NewValueSlotTarget: %v", err)
	}
	slot, err := typedmemory.NewSlotSpec(
		payloadSlot,
		target,
		typedmemory.ExactlyOneCardinality(),
		provenance,
	)
	if err != nil {
		t.Fatalf("NewSlotSpec: %v", err)
	}
	signatureRef, err := typedmemory.NewRelationSignatureRef(
		base.environment.Ref(),
		mustGenericSignatureID(t, "test.PayloadRelation"),
	)
	if err != nil {
		t.Fatalf("NewRelationSignatureRef: %v", err)
	}
	signature, err := typedmemory.NewRelationSignature(
		signatureRef,
		[]typedmemory.BoundedContextRef{primary.Ref()},
		[]typedmemory.SlotSpec{slot},
		provenance,
	)
	if err != nil {
		t.Fatalf("NewRelationSignature: %v", err)
	}
	environment, err := typedmemory.NewTypeEnvBuilder(base.environment.Ref()).
		SetSourceRevision(base.snapshot.SourceRevision()).
		SetCompilerSchemaVersion(base.snapshot.CompilerSchemaVersion()).
		SetCoverageManifest(base.environment.CoverageManifest()).
		AddBoundedContext(primary).
		AddBoundedContext(secondary).
		AddKindDefinition(textDefinition).
		AddContextKindAvailability(admission).
		AddRelationSignature(signature).
		AddValueShape(shapeDeclaration).
		AddValueBinding(binding).
		Build()
	if err != nil {
		t.Fatalf("build generic mixed TypeEnv: %v", err)
	}
	registry, err := typedmemory.NewCodecRegistry().Register(
		codecRef,
		genericTextCodec{shape: shapeRef},
	)
	if err != nil {
		t.Fatalf("CodecRegistry.Register: %v", err)
	}
	loader := staticTypeEnvLoader{
		reference:   environment.Ref(),
		environment: environment,
		registry:    registry,
	}
	adapter, err := NewGenericSQLiteAdapter(
		base.database,
		loader,
		base.clock,
		unexpectedMemberOfEngine{},
		unexpectedReferenceEngine{},
		unexpectedObservableProvider{},
	)
	if err != nil {
		t.Fatalf("NewGenericSQLiteAdapter: %v", err)
	}
	gamma, err := typedmemory.NewGammaPoint(time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewGammaPoint: %v", err)
	}
	contextSlice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   primary.Ref(),
		GammaTime: gamma,
	})
	if err != nil {
		t.Fatalf("NewContextSlice: %v", err)
	}
	fixture := genericMixedStoreFixture{
		base:         base,
		environment:  environment,
		registry:     registry,
		adapter:      adapter,
		primary:      primary.Ref(),
		secondary:    secondary.Ref(),
		textKind:     textKind,
		shape:        shapeRef,
		codec:        codecRef,
		signature:    signatureRef,
		payloadSlot:  payloadSlot,
		contextSlice: contextSlice,
		anchor:       mustGenericEntityID(t, "entity:anchor"),
		oldAlias:     mustGenericAlias(t, "old-anchor"),
		oldAssertion: mustGenericAssertionID(t, "assertion:old"),
	}
	return fixture
}

func mustLocalContextKindAvailability(
	t *testing.T,
	base typedmemory.TypeEnvRef,
	contextRef typedmemory.BoundedContextRef,
	kind typedmemory.KindID,
	upstream typedmemory.DeclarationProvenance,
	fixtureID string,
) typedmemory.ContextKindAvailability {
	t.Helper()
	symbol, err := typedmemory.KindSymbolRef(kind)
	if err != nil {
		t.Fatalf("KindSymbolRef: %v", err)
	}
	manifest, err := typedmemory.NewSignatureManifestRef(fixtureID, "1.0.0")
	if err != nil {
		t.Fatalf("NewSignatureManifestRef: %v", err)
	}
	basis, err := typedmemory.NewManifestSymbolBasis(
		manifest,
		typedmemory.ManifestProvide,
		symbol,
	)
	if err != nil {
		t.Fatalf("NewManifestSymbolBasis: %v", err)
	}
	digestSource := append(
		[]byte(fixtureID+"\x00"+contextRef.String()+"\x00"+kind.String()+"\x00"),
		upstream.CanonicalBytes()...,
	)
	digest := mustDigest(t, digestSource)
	reference, err := typedmemory.NewProvenanceRef("prov:" + fixtureID)
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	carrier, err := typedmemory.NewCarrierRef("carrier:" + fixtureID)
	if err != nil {
		t.Fatalf("NewCarrierRef: %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatalf("NewCarrierEdition: %v", err)
	}
	lineRange, err := typedmemory.NewSourceLineRange(1, 1)
	if err != nil {
		t.Fatalf("NewSourceLineRange: %v", err)
	}
	rule, err := typedmemory.NewCompilerRuleID("fixture.context-kind-availability.v1")
	if err != nil {
		t.Fatalf("NewCompilerRuleID: %v", err)
	}
	projectProvenance, err := typedmemory.NewProjectSourceProvenanceBuilder(
		reference,
		carrier,
		edition,
		digest,
	).
		SetDeclarationRange(lineRange).
		SetCompilerRule(rule).
		SetBoundedContext(contextRef).
		SetBaseTypeEnv(base).
		SetSignatureBlockRow(typedmemory.VocabularyRow).
		SetManifestBasis(basis).
		Build()
	if err != nil {
		t.Fatalf("ProjectSourceProvenanceBuilder.Build: %v", err)
	}
	contextSource, err := typedmemory.NewContextKindAvailabilitySource(
		contextRef.String(),
		projectProvenance,
	)
	if err != nil {
		t.Fatalf("NewContextKindAvailabilitySource(context): %v", err)
	}
	declarationSource, err := typedmemory.NewContextKindAvailabilitySource(
		kind.String(),
		projectProvenance,
	)
	if err != nil {
		t.Fatalf("NewContextKindAvailabilitySource(kind): %v", err)
	}
	extensionRef, err := typedmemory.ParseTypeEnvExtensionRef(
		"typeenv-extension:" + manifest.ID() + "@" + digest.String(),
	)
	if err != nil {
		t.Fatalf("ParseTypeEnvExtensionRef: %v", err)
	}
	provider, err := typedmemory.NewExtensionKindAvailabilityProvider(
		typedmemory.ExtensionKindAvailabilityProviderInput{
			ExtensionRef:      extensionRef,
			Context:           contextRef,
			ContextSource:     contextSource,
			Symbol:            symbol,
			DeclarationSource: declarationSource,
		},
	)
	if err != nil {
		t.Fatalf("NewExtensionKindAvailabilityProvider: %v", err)
	}
	ground, err := typedmemory.NewLocalContextKindAvailabilityGround(
		typedmemory.LocalContextKindAvailabilityGroundInput{
			Context:             contextRef,
			KindID:              kind,
			ContextSource:       contextSource,
			ApplicabilitySource: contextSource,
			Provider:            provider,
		},
	)
	if err != nil {
		t.Fatalf("NewLocalContextKindAvailabilityGround: %v", err)
	}
	grounds, err := typedmemory.NewContextKindAvailabilityGroundSet(
		[]typedmemory.ContextKindAvailabilityGround{ground},
	)
	if err != nil {
		t.Fatalf("NewContextKindAvailabilityGroundSet: %v", err)
	}
	availability, err := typedmemory.NewContextKindAvailability(contextRef, kind, grounds)
	if err != nil {
		t.Fatalf("NewContextKindAvailability: %v", err)
	}
	return availability
}

func (fixture genericMixedStoreFixture) bootstrap(t *testing.T) {
	t.Helper()
	declaration := fixture.declarationCandidate(
		t,
		fixture.anchor,
		"local:anchor",
		fixture.primary,
		"Anchor",
		"memory:test:anchor",
	)
	declareRequest := fixture.requestAt(
		t,
		0,
		"generic-bootstrap-anchor",
		declaration,
		func(snapshot *genericMixedSnapshot) {
			snapshot.entityAbsent(t, fixture.anchor, fixture.primary)
		},
	)
	if _, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), declareRequest); err != nil {
		t.Fatalf("bootstrap anchor: %v", err)
	}

	oldAlias := fixture.admitAliasChange(t, fixture.oldAlias, "memory:test:old-alias")
	oldRelation := fixture.relationChange(t, fixture.oldAssertion, "old payload", "memory:test:old-relation")
	seed, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{oldAlias, oldRelation})
	if err != nil {
		t.Fatalf("bootstrap NewMemoryChangeSet: %v", err)
	}
	seedRequest := fixture.requestAt(
		t,
		1,
		"generic-bootstrap-alias-relation",
		seed,
		func(snapshot *genericMixedSnapshot) {
			snapshot.entityExact(t, fixture.anchor, fixture.primary)
			snapshot.aliasUnbound(t, fixture.oldAlias, fixture.primary)
			snapshot.assertionAbsent(t, fixture.oldAssertion)
		},
	)
	if _, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), seedRequest); err != nil {
		t.Fatalf("bootstrap alias and relation: %v", err)
	}
}

func (fixture genericMixedStoreFixture) finalCandidate(
	t *testing.T,
	label string,
	payload string,
) typedmemory.MemoryChangeSet {
	t.Helper()
	newEntity := mustGenericEntityID(t, "entity:new")
	declaration := fixture.declarationChange(
		t,
		newEntity,
		"local:new",
		fixture.primary,
		label,
		"memory:test:new",
	)
	freshAlias := fixture.admitAliasChange(
		t,
		mustGenericAlias(t, "fresh-anchor"),
		"memory:test:fresh-alias",
	)
	supersession := fixture.supersedeAliasChange(
		t,
		fixture.oldAlias,
		mustGenericAlias(t, "replacement-anchor"),
		"memory:test:supersede-alias",
	)
	newAssertion := mustGenericAssertionID(t, "assertion:new")
	relation := fixture.relationChange(t, newAssertion, payload, "memory:test:new-relation")
	reason, err := typedmemory.NewRetractionReason("superseded by the new assertion")
	if err != nil {
		t.Fatalf("NewRetractionReason: %v", err)
	}
	retraction, err := typedmemory.NewRetractAssertion(
		fixture.oldAssertion,
		reason,
		mustGenericProvenanceRef(t, "memory:test:retraction"),
	)
	if err != nil {
		t.Fatalf("NewRetractAssertion: %v", err)
	}
	candidate, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{
		declaration,
		freshAlias,
		supersession,
		relation,
		retraction,
	})
	if err != nil {
		t.Fatalf("final NewMemoryChangeSet: %v", err)
	}
	return candidate
}

func (fixture genericMixedStoreFixture) finalRequest(
	t *testing.T,
	key string,
	candidate typedmemory.MemoryChangeSet,
) CommitRequest {
	t.Helper()
	return fixture.requestAt(
		t,
		2,
		key,
		candidate,
		func(snapshot *genericMixedSnapshot) {
			newEntity := mustGenericEntityID(t, "entity:new")
			newAssertion := mustGenericAssertionID(t, "assertion:new")
			snapshot.entityAbsent(t, newEntity, fixture.primary)
			snapshot.entityExact(t, fixture.anchor, fixture.primary)
			snapshot.aliasUnbound(t, mustGenericAlias(t, "fresh-anchor"), fixture.primary)
			snapshot.aliasBound(t, fixture.oldAlias, fixture.anchor, fixture.primary)
			snapshot.aliasUnbound(t, mustGenericAlias(t, "replacement-anchor"), fixture.primary)
			snapshot.assertionAbsent(t, newAssertion)
			snapshot.assertionActive(t, fixture.oldAssertion)
		},
	)
}

func (fixture genericMixedStoreFixture) requestAt(
	t *testing.T,
	revision uint64,
	key string,
	candidate typedmemory.MemoryChangeSet,
	configure func(*genericMixedSnapshot),
) CommitRequest {
	t.Helper()
	snapshot := fixture.snapshotAt(t, revision)
	configure(&snapshot)
	verdict := typedmemory.ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		snapshot,
		candidate,
	)
	valid, ok := verdict.(typedmemory.Valid)
	if !ok {
		t.Fatalf(
			"ValidateMemoryChangeSet = %T (%s), details=%#v; want Valid",
			verdict,
			verdict.Kind(),
			verdict,
		)
	}
	request, err := NewCommitRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.base.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(revision)).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(mustIdempotencyKey(t, key)).
		SetRequestProvenance(mustRequestProvenanceRef(t)).
		SetCandidate(candidate).
		SetAdmissionBatch(valid.AdmissionBatch()).
		Build()
	if err != nil {
		t.Fatalf("build generic mixed CommitRequest: %v", err)
	}
	return request
}

func (fixture genericMixedStoreFixture) snapshotAt(
	t *testing.T,
	revision uint64,
) genericMixedSnapshot {
	t.Helper()
	graphRevision := typedmemory.NewGraphRevision(revision)
	basis, err := snapshotResolutionBasis(fixture.base.project, graphRevision)
	if err != nil {
		t.Fatalf("snapshotResolutionBasis: %v", err)
	}
	rule, err := snapshotAssertionRule(fixture.base.project, graphRevision)
	if err != nil {
		t.Fatalf("snapshotAssertionRule: %v", err)
	}
	return genericMixedSnapshot{
		revision:   graphRevision,
		typeEnv:    fixture.environment.Ref(),
		basis:      basis,
		rule:       rule,
		entities:   make(map[string]typedmemory.EntityResolution),
		aliases:    make(map[string]typedmemory.AliasAvailability),
		assertions: make(map[string]typedmemory.AssertionState),
	}
}

func (fixture genericMixedStoreFixture) declarationCandidate(
	t *testing.T,
	entity typedmemory.EntityID,
	local string,
	contextRef typedmemory.BoundedContextRef,
	label string,
	provenance string,
) typedmemory.MemoryChangeSet {
	t.Helper()
	change := fixture.declarationChange(t, entity, local, contextRef, label, provenance)
	candidate, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{change})
	if err != nil {
		t.Fatalf("declaration NewMemoryChangeSet: %v", err)
	}
	return candidate
}

func (genericMixedStoreFixture) declarationChange(
	t *testing.T,
	entity typedmemory.EntityID,
	local string,
	contextRef typedmemory.BoundedContextRef,
	label string,
	provenance string,
) typedmemory.DeclareEntity {
	t.Helper()
	localRef, err := typedmemory.NewBatchLocalRef(local)
	if err != nil {
		t.Fatalf("NewBatchLocalRef: %v", err)
	}
	entityLabel, err := typedmemory.NewEntityLabel(label)
	if err != nil {
		t.Fatalf("NewEntityLabel: %v", err)
	}
	change, err := typedmemory.NewDeclareEntity(
		entity,
		localRef,
		contextRef,
		entityLabel,
		mustGenericProvenanceRef(t, provenance),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity: %v", err)
	}
	return change
}

func (fixture genericMixedStoreFixture) admitAliasChange(
	t *testing.T,
	alias typedmemory.EntityAlias,
	provenance string,
) typedmemory.ApplyIdentityChange {
	t.Helper()
	change, err := typedmemory.NewAdmitAlias(
		fixture.anchor,
		alias,
		fixture.primary,
		mustGenericProvenanceRef(t, provenance),
	)
	if err != nil {
		t.Fatalf("NewAdmitAlias: %v", err)
	}
	effect, err := typedmemory.NewApplyIdentityChange(change)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange(AdmitAlias): %v", err)
	}
	return effect
}

func (fixture genericMixedStoreFixture) supersedeAliasChange(
	t *testing.T,
	oldAlias typedmemory.EntityAlias,
	replacement typedmemory.EntityAlias,
	provenance string,
) typedmemory.ApplyIdentityChange {
	t.Helper()
	change, err := typedmemory.NewSupersedeAlias(
		fixture.anchor,
		oldAlias,
		replacement,
		fixture.primary,
		mustGenericProvenanceRef(t, provenance),
	)
	if err != nil {
		t.Fatalf("NewSupersedeAlias: %v", err)
	}
	effect, err := typedmemory.NewApplyIdentityChange(change)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange(SupersedeAlias): %v", err)
	}
	return effect
}

func (fixture genericMixedStoreFixture) relationChange(
	t *testing.T,
	assertion typedmemory.AssertionID,
	payload string,
	provenance string,
) typedmemory.AssertRelation {
	t.Helper()
	binding := fixture.relationBinding(t, payload)
	relation, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  assertion,
			Signature:  fixture.signature,
			Slice:      fixture.contextSlice,
			Modality:   typedmemory.NewAffirmsObtaining(),
			Bindings:   []typedmemory.CandidateSlotBinding{binding},
			Provenance: mustGenericProvenanceRef(t, provenance),
		},
	)
	if err != nil {
		t.Fatalf("NewRelationalAssertionCandidate: %v", err)
	}
	change, err := typedmemory.NewAssertRelation(relation)
	if err != nil {
		t.Fatalf("NewAssertRelation: %v", err)
	}
	return change
}

func (fixture genericMixedStoreFixture) legacyRelationChange(
	t *testing.T,
	assertion typedmemory.AssertionID,
	payload string,
	provenance string,
) typedmemory.InstantiateRelation {
	t.Helper()
	binding := fixture.relationBinding(t, payload)
	relation, err := typedmemory.NewRelationInstantiation(
		assertion,
		fixture.signature,
		fixture.contextSlice,
		[]typedmemory.CandidateSlotBinding{binding},
		mustGenericProvenanceRef(t, provenance),
	)
	if err != nil {
		t.Fatalf("NewRelationInstantiation(legacy fixture): %v", err)
	}
	change, err := typedmemory.NewInstantiateRelation(relation)
	if err != nil {
		t.Fatalf("NewInstantiateRelation(legacy fixture): %v", err)
	}
	return change
}

func (fixture genericMixedStoreFixture) relationBinding(
	t *testing.T,
	payload string,
) typedmemory.CandidateSlotBinding {
	t.Helper()
	value, err := typedmemory.NewTypedValueCandidate(
		fixture.textKind,
		fixture.shape,
		fixture.codec,
		[]byte(payload),
		typedmemory.NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate: %v", err)
	}
	filler, err := typedmemory.NewByValueCandidate(value)
	if err != nil {
		t.Fatalf("NewByValueCandidate: %v", err)
	}
	binding, err := typedmemory.NewCandidateSlotBinding(
		fixture.payloadSlot,
		[]typedmemory.CandidateSlotFiller{filler},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding: %v", err)
	}
	return binding
}

type genericMixedSnapshot struct {
	revision   typedmemory.GraphRevision
	typeEnv    typedmemory.TypeEnvRef
	basis      typedmemory.ResolutionBasisRef
	rule       typedmemory.RuleRef
	entities   map[string]typedmemory.EntityResolution
	aliases    map[string]typedmemory.AliasAvailability
	assertions map[string]typedmemory.AssertionState
}

func (snapshot genericMixedSnapshot) GraphRevision() typedmemory.GraphRevision {
	return snapshot.revision
}

func (snapshot genericMixedSnapshot) TypeEnvRef() typedmemory.TypeEnvRef {
	return snapshot.typeEnv
}

func (snapshot genericMixedSnapshot) ResolveEntity(
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.EntityResolution {
	return snapshot.entities[entityObservationKey(entity, contextRef)]
}

func (genericMixedSnapshot) ResolveReference(
	typedmemory.StrongRef,
	typedmemory.BoundedContextRef,
) typedmemory.StrongReferenceResolution {
	return nil
}

func (genericMixedSnapshot) EvaluateMemberOf(
	typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	return nil
}

func (snapshot genericMixedSnapshot) AssertionState(
	assertion typedmemory.AssertionID,
) typedmemory.AssertionState {
	return snapshot.assertions[assertion.String()]
}

func (snapshot genericMixedSnapshot) ResolveAlias(
	alias typedmemory.EntityAlias,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.AliasAvailability {
	return snapshot.aliases[aliasObservationKey(alias, contextRef)]
}

func (genericMixedSnapshot) ResolveReconciliationBasis(
	basis typedmemory.ReconciliationBasisRef,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.ReconciliationBasisResolution {
	resolution, _ := typedmemory.NewMissingReconciliationBasis(basis, contextRef)
	return resolution
}

func (snapshot *genericMixedSnapshot) entityAbsent(
	t *testing.T,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) {
	t.Helper()
	resolution, err := typedmemory.NewAbsentEntityResolution(entity, contextRef, snapshot.basis)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	snapshot.entities[entityObservationKey(entity, contextRef)] = resolution
}

func (snapshot *genericMixedSnapshot) entityExact(
	t *testing.T,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) {
	t.Helper()
	resolution, err := typedmemory.NewExactEntityResolution(entity, contextRef, snapshot.basis)
	if err != nil {
		t.Fatalf("NewExactEntityResolution: %v", err)
	}
	snapshot.entities[entityObservationKey(entity, contextRef)] = resolution
}

func (snapshot *genericMixedSnapshot) aliasUnbound(
	t *testing.T,
	alias typedmemory.EntityAlias,
	contextRef typedmemory.BoundedContextRef,
) {
	t.Helper()
	resolution, err := typedmemory.NewUnboundAliasResolution(alias, contextRef, snapshot.basis)
	if err != nil {
		t.Fatalf("NewUnboundAliasResolution: %v", err)
	}
	snapshot.aliases[aliasObservationKey(alias, contextRef)] = resolution
}

func (snapshot *genericMixedSnapshot) aliasBound(
	t *testing.T,
	alias typedmemory.EntityAlias,
	entity typedmemory.EntityID,
	contextRef typedmemory.BoundedContextRef,
) {
	t.Helper()
	resolution, err := typedmemory.NewBoundAliasResolution(alias, entity, contextRef, snapshot.basis)
	if err != nil {
		t.Fatalf("NewBoundAliasResolution: %v", err)
	}
	snapshot.aliases[aliasObservationKey(alias, contextRef)] = resolution
}

func (snapshot *genericMixedSnapshot) assertionAbsent(
	t *testing.T,
	assertion typedmemory.AssertionID,
) {
	t.Helper()
	state, err := typedmemory.NewAbsentAssertionState(assertion, snapshot.rule)
	if err != nil {
		t.Fatalf("NewAbsentAssertionState: %v", err)
	}
	snapshot.assertions[assertion.String()] = state
}

func (snapshot *genericMixedSnapshot) assertionActive(
	t *testing.T,
	assertion typedmemory.AssertionID,
) {
	t.Helper()
	state, err := typedmemory.NewActiveAssertion(assertion, snapshot.rule)
	if err != nil {
		t.Fatalf("NewActiveAssertion: %v", err)
	}
	snapshot.assertions[assertion.String()] = state
}

type genericTextCodec struct {
	shape typedmemory.ValueShapeRef
}

func (codec genericTextCodec) Canonicalize(
	_ typedmemory.ValueShapeRef,
	input []byte,
) typedmemory.CodecCanonicalization {
	value := typedmemory.NewTextValue(string(input))
	canonical, err := typedmemory.NewCanonicalizedCodecValue(value, input)
	if err != nil {
		panic(fmt.Sprintf("generic text codec fixture: %v", err))
	}
	return canonical
}

func assertMixedAliasLineage(
	t *testing.T,
	fixture genericMixedStoreFixture,
	eventRef string,
) {
	t.Helper()
	var oldAlias string
	var replacement string
	var supersedes string
	err := fixture.base.database.QueryRow(
		`SELECT alias, replacement_alias, supersedes_alias_change_ref
		 FROM typed_memory_alias_changes
		 WHERE project_id = ? AND event_ref = ? AND change_kind = 'supersede_alias'`,
		fixture.base.project.String(),
		eventRef,
	).Scan(&oldAlias, &replacement, &supersedes)
	if err != nil {
		t.Fatalf("load alias supersession: %v", err)
	}
	if oldAlias != fixture.oldAlias.String() || replacement != "replacement-anchor" || supersedes == "" {
		t.Fatalf(
			"alias supersession = (%q, %q, %q); want exact old, replacement, and lineage",
			oldAlias,
			replacement,
			supersedes,
		)
	}
}

func mustGenericKindID(t *testing.T, raw string) typedmemory.KindID {
	t.Helper()
	value, err := typedmemory.NewKindID(raw)
	if err != nil {
		t.Fatalf("NewKindID: %v", err)
	}
	return value
}

func mustGenericShapeID(t *testing.T, raw string) typedmemory.ShapeID {
	t.Helper()
	value, err := typedmemory.NewShapeID(raw)
	if err != nil {
		t.Fatalf("NewShapeID: %v", err)
	}
	return value
}

func mustGenericCodecID(t *testing.T, raw string) typedmemory.CodecID {
	t.Helper()
	value, err := typedmemory.NewCodecID(raw)
	if err != nil {
		t.Fatalf("NewCodecID: %v", err)
	}
	return value
}

func mustGenericCodecVersion(t *testing.T, raw string) typedmemory.CanonicalizationVersion {
	t.Helper()
	value, err := typedmemory.NewCanonicalizationVersion(raw)
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion: %v", err)
	}
	return value
}

func mustGenericSlotKindID(t *testing.T, raw string) typedmemory.SlotKindID {
	t.Helper()
	value, err := typedmemory.NewSlotKindID(raw)
	if err != nil {
		t.Fatalf("NewSlotKindID: %v", err)
	}
	return value
}

func mustGenericSignatureID(t *testing.T, raw string) typedmemory.SignatureID {
	t.Helper()
	value, err := typedmemory.NewSignatureID(raw)
	if err != nil {
		t.Fatalf("NewSignatureID: %v", err)
	}
	return value
}

func mustGenericEntityID(t *testing.T, raw string) typedmemory.EntityID {
	t.Helper()
	value, err := typedmemory.NewEntityID(raw)
	if err != nil {
		t.Fatalf("NewEntityID: %v", err)
	}
	return value
}

func mustGenericAssertionID(t *testing.T, raw string) typedmemory.AssertionID {
	t.Helper()
	value, err := typedmemory.NewAssertionID(raw)
	if err != nil {
		t.Fatalf("NewAssertionID: %v", err)
	}
	return value
}

func mustGenericAlias(t *testing.T, raw string) typedmemory.EntityAlias {
	t.Helper()
	value, err := typedmemory.NewEntityAlias(raw)
	if err != nil {
		t.Fatalf("NewEntityAlias: %v", err)
	}
	return value
}

func mustGenericProvenanceRef(t *testing.T, raw string) typedmemory.ProvenanceRef {
	t.Helper()
	value, err := typedmemory.NewProvenanceRef(raw)
	if err != nil {
		t.Fatalf("NewProvenanceRef: %v", err)
	}
	return value
}

func mustGenericReconciliationBasis(
	t *testing.T,
	raw string,
) typedmemory.ReconciliationBasisRef {
	t.Helper()
	value, err := typedmemory.NewReconciliationBasisRef(raw)
	if err != nil {
		t.Fatalf("NewReconciliationBasisRef: %v", err)
	}
	return value
}
