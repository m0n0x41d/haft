package typedmemorystore

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestV2RelationalAssertionSemanticFootprintMatchesSchemaV53View(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := v3ExactBasisRequest(t, fixture, typedmemory.NewAffirmsObtaining())
	prepared, err := prepareGenericAdmission(request)
	if err != nil {
		t.Fatalf("prepareGenericAdmission(v2): %v", err)
	}
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginImmediate(ctx, fixture.base.database)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if err := requireGenericAdmissionStorageCapability(
		ctx,
		transaction,
		request.ContractVersion(),
	); err != nil {
		t.Fatalf("requireGenericAdmissionStorageCapability(v2): %v", err)
	}
	runtime, err := fixture.adapter.resolveTypeEnvRuntimeTx(
		ctx,
		transaction,
		request.project,
		request.expectedRevision,
		request.expectedTypeEnv,
	)
	if err != nil {
		t.Fatalf("resolveTypeEnvRuntimeTx(v2): %v", err)
	}
	revalidated, err := fixture.adapter.revalidateGenericAdmission(
		ctx,
		transaction,
		request,
		prepared,
		runtime.environment,
		runtime.codecs,
		runtime.memberOf,
		runtime.classification,
	)
	if err != nil {
		t.Fatalf("revalidateGenericAdmission(v2): %v", err)
	}
	identity, err := newGenericEventIdentity(request, prepared)
	if err != nil {
		t.Fatalf("newGenericEventIdentity(v2): %v", err)
	}
	semantic, err := buildGenericSemanticMaterialization(
		ctx,
		transaction,
		request,
		revalidated,
		runtime.environment,
		identity,
		canonicalTime(fixture.base.clock.Now()),
	)
	if err != nil {
		t.Fatalf("buildGenericSemanticMaterialization(v2): %v", err)
	}
	manifest, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		t.Fatalf("buildExpectedMaterializationManifest(v2): %v", err)
	}
	statements := []statement{
		genericEventStatement(
			request,
			prepared,
			identity,
			canonicalTime(fixture.base.clock.Now()),
		),
		genericWriterGenerationStatement(request, identity),
		genericAdmissionBasisStatement(
			request,
			prepared,
			manifest,
			identity,
			canonicalTime(fixture.base.clock.Now()),
		),
	}
	statements = append(statements, semantic.statements...)
	if err := executeStatements(ctx, transaction, statements, 0); err != nil {
		t.Fatalf("execute v3 semantic statements: %v", err)
	}
	actual := genericMaterializationFootprint{}
	err = transaction.ScanOne(
		ctx,
		`SELECT entity_count, entity_context_count, entity_declaration_count,
			context_slice_catalog_count, context_slice_count,
			value_blob_count, observable_input_blob_count, relation_count,
			relation_slot_count, relation_filler_count,
			ordered_candidate_prefix_count, reference_resolution_use_count,
			memberof_evaluation_count, memberof_input_count, memberof_use_count,
			alias_change_count, retraction_count
		FROM typed_memory_event_materialization_footprints_v46
		WHERE project_id = ? AND event_ref = ?`,
		[]any{request.project.String(), identity.eventRef},
		[]any{
			&actual.entityCount,
			&actual.entityContextCount,
			&actual.entityDeclarationCount,
			&actual.contextSliceCatalogCount,
			&actual.contextSliceCount,
			&actual.valueBlobCount,
			&actual.observableInputBlobCount,
			&actual.relationCount,
			&actual.relationSlotCount,
			&actual.relationFillerCount,
			&actual.orderedCandidatePrefixCount,
			&actual.referenceResolutionCount,
			&actual.memberOfEvaluationCount,
			&actual.memberOfInputCount,
			&actual.memberOfUseCount,
			&actual.aliasChangeCount,
			&actual.retractionCount,
		},
	)
	if err != nil {
		t.Fatalf("load v53 aggregate footprint: %v", err)
	}
	if !reflect.DeepEqual(actual, semantic.footprint) {
		t.Fatalf("v53 footprint = %+v; want %+v", actual, semantic.footprint)
	}
}

func TestV2RelationalAssertionMaterializationUsesOnlyTheV3StorageFamily(
	t *testing.T,
) {
	fixture := newExactBasisStoreFixture(t)
	request := v3ExactBasisRequest(t, fixture, typedmemory.NewAffirmsObtaining())

	prepared, err := prepareGenericAdmission(request)
	if err != nil {
		t.Fatalf("prepareGenericAdmission(v2): %v", err)
	}
	kind, err := classifyAdmittedChange(prepared.changes[1].change)
	if err != nil {
		t.Fatalf("classify v3 assertion: %v", err)
	}
	if kind != "assert_relation" {
		t.Fatalf("v3 event kind = %q; want assert_relation", kind)
	}
	manifest, err := buildExpectedMaterializationManifest(prepared)
	if err != nil {
		t.Fatalf("buildExpectedMaterializationManifest(v2): %v", err)
	}
	assertV3ManifestFamily(t, manifest)

	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("CommitMemoryChangeSet(v2): %v", err)
	}
	if receipt.Disposition() != CommitApplied {
		t.Fatalf("v2 disposition = %q; want applied", receipt.Disposition())
	}
	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_relational_assertions_v3": 1,
		"typed_memory_relation_instances":       0,
	})
	var modality string
	err = fixture.base.database.QueryRow(
		`SELECT modality FROM typed_memory_relational_assertions_v3
		WHERE project_id = ? AND event_ref = ?`,
		fixture.base.project.String(),
		receipt.EventRef(),
	).Scan(&modality)
	if err != nil {
		t.Fatalf("load stored v3 modality: %v", err)
	}
	if modality != typedmemory.AssertionModalityAffirmsObtaining.String() {
		t.Fatalf(
			"stored modality = %q; want %q",
			modality,
			typedmemory.AssertionModalityAffirmsObtaining,
		)
	}
}

func TestPrepareGenericAdmissionRejectsCrossVersionRelationFamilies(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	v2 := v3ExactBasisRequest(t, fixture, typedmemory.NewObtainingUnknown())
	v1 := fixture.legacyRequest(t, "legacy-family-for-v2-rejection")

	v2.contractVersion = AdmissionContractV1()
	if _, err := prepareGenericAdmission(v2); !errors.Is(err, ErrUnsupportedBatch) {
		t.Fatalf(
			"v1 relational-assertion error = %v; want ErrUnsupportedBatch",
			err,
		)
	}
	v1.contractVersion = AdmissionContractV2()
	if _, err := prepareGenericAdmission(v1); !errors.Is(err, ErrUnsupportedBatch) {
		t.Fatalf(
			"v2 legacy-relation error = %v; want ErrUnsupportedBatch",
			err,
		)
	}
}

func assertV3ManifestFamily(
	t *testing.T,
	manifest expectedMaterializationManifest,
) {
	t.Helper()
	seen := make(map[string]bool)
	for _, row := range manifest.SemanticRows() {
		seen[row.rowKind] = true
	}
	for _, required := range []string{
		relationalAssertionStorageFamily.assertionRowKind,
		relationalAssertionStorageFamily.slotRowKind,
		relationalAssertionStorageFamily.fillerRowKind,
		relationalAssertionStorageFamily.resolutionRowKind,
		relationalAssertionStorageFamily.memberOfUseRowKind,
	} {
		if !seen[required] {
			t.Fatalf("v3 manifest is missing row kind %q", required)
		}
	}
	for _, forbidden := range []string{
		legacyRelationStorageFamily.assertionRowKind,
		legacyRelationStorageFamily.slotRowKind,
		legacyRelationStorageFamily.fillerRowKind,
		legacyRelationStorageFamily.resolutionRowKind,
		legacyRelationStorageFamily.memberOfUseRowKind,
	} {
		if seen[forbidden] {
			t.Fatalf("v3 manifest leaked legacy row kind %q", forbidden)
		}
	}
}

func v3ExactBasisRequest(
	t *testing.T,
	fixture exactBasisStoreFixture,
	modality typedmemory.AssertionModality,
) CommitRequest {
	t.Helper()
	entity := mustGenericEntityID(t, "entity:v3-exact-basis")
	localID, err := typedmemory.NewBatchLocalRef("local:v3-exact-basis")
	if err != nil {
		t.Fatalf("NewBatchLocalRef(v3): %v", err)
	}
	declaration, err := typedmemory.NewDeclareEntity(
		entity,
		localID,
		fixture.base.context,
		exactBasisEntityLabel(t, "V3 exact basis entity"),
		mustGenericProvenanceRef(t, "memory:test:v3-exact-basis-declaration"),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity(v3): %v", err)
	}
	localRef, err := typedmemory.NewLocalRef(fixture.entityRefKind, localID)
	if err != nil {
		t.Fatalf("NewLocalRef(v3): %v", err)
	}
	filler, err := typedmemory.NewByReferenceCandidate(localRef)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate(v3): %v", err)
	}
	binding, err := typedmemory.NewCandidateSlotBinding(
		fixture.referenceSlot,
		[]typedmemory.CandidateSlotFiller{filler},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding(v3): %v", err)
	}
	assertionID := mustGenericAssertionID(t, "assertion:v3-exact-basis")
	assertion, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  assertionID,
			Signature:  fixture.relation,
			Slice:      fixture.contextSlice,
			Modality:   modality,
			Bindings:   []typedmemory.CandidateSlotBinding{binding},
			Provenance: mustGenericProvenanceRef(t, "memory:test:v3-exact-basis-assertion"),
		},
	)
	if err != nil {
		t.Fatalf("NewRelationalAssertionCandidate: %v", err)
	}
	assertChange, err := typedmemory.NewAssertRelation(assertion)
	if err != nil {
		t.Fatalf("NewAssertRelation: %v", err)
	}
	candidate, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{
		declaration,
		assertChange,
	})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(v3): %v", err)
	}

	graphRevision := typedmemory.NewGraphRevision(0)
	absenceBasis, err := snapshotResolutionBasis(fixture.base.project, graphRevision)
	if err != nil {
		t.Fatalf("snapshotResolutionBasis(v3): %v", err)
	}
	entityAbsent, err := typedmemory.NewAbsentEntityResolution(
		entity,
		fixture.base.context,
		absenceBasis,
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution(v3): %v", err)
	}
	assertionRule, err := snapshotAssertionRule(fixture.base.project, graphRevision)
	if err != nil {
		t.Fatalf("snapshotAssertionRule(v3): %v", err)
	}
	assertionAbsent, err := typedmemory.NewAbsentAssertionState(assertionID, assertionRule)
	if err != nil {
		t.Fatalf("NewAbsentAssertionState(v3): %v", err)
	}
	snapshot := exactBasisSnapshot{
		revision:        graphRevision,
		typeEnv:         fixture.environment.Ref(),
		entityAbsent:    entityAbsent,
		assertionAbsent: assertionAbsent,
		memberEngine: exactBasisMemberOfEngine{
			expectedProject: fixture.base.project,
			kindSignature:   fixture.kindSignature,
			entitySet:       fixture.entitySet,
			provenance:      fixture.evaluationSource,
		},
		inputs: fixture.observableInputs,
	}
	verdict := typedmemory.ValidateMemoryChangeSet(
		fixture.environment,
		typedmemory.NewCodecRegistry(),
		snapshot,
		candidate,
	)
	valid, ok := verdict.(typedmemory.Valid)
	if !ok {
		t.Fatalf("ValidateMemoryChangeSet(v3) = %T (%s); want Valid", verdict, verdict.Kind())
	}
	request, err := NewCommitRequestBuilder().
		SetContractVersion(AdmissionContractV2()).
		SetProject(fixture.base.project).
		SetExpectedRevision(graphRevision).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(mustIdempotencyKey(t, "v3-exact-basis-assertion")).
		SetRequestProvenance(mustRequestProvenanceRef(t)).
		SetCandidate(candidate).
		SetAdmissionBatch(valid.AdmissionBatch()).
		Build()
	if err != nil {
		t.Fatalf("build v3 CommitRequest: %v", err)
	}
	return request
}
