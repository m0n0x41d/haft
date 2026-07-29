package typedmemorystore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCurrentKindClassificationCommitPersistsExactV54ClosureAndReplay(
	t *testing.T,
) {
	fixture := newCurrentClassificationStoreFixture(t)

	receipt, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		fixture.request,
	)
	if err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	if receipt.Disposition() != CommitApplied || receipt.GraphRevision().Value() != 1 {
		t.Fatalf("receipt = %#v; want applied revision 1", receipt)
	}

	replay, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		fixture.request,
	)
	if err != nil {
		t.Fatalf("replay CommitMemoryChangeSet: %v", err)
	}
	if replay.Disposition() != CommitReplay ||
		replay.EventRef() != receipt.EventRef() ||
		replay.CommitRef() != receipt.CommitRef() ||
		replay.GraphRevision() != receipt.GraphRevision() ||
		replay.ResultDigest() != receipt.ResultDigest() {
		t.Fatalf("replay = %#v; want exact original receipt %#v", replay, receipt)
	}

	assertCurrentClassificationWriter(t, fixture, receipt.EventRef())
	assertCurrentClassificationRows(t, fixture, receipt.EventRef())
	assertCurrentClassificationReadIntegrity(t, fixture, receipt)
}

func TestCurrentKindClassificationSourceCatalogRoundTripsAfterRestart(
	t *testing.T,
) {
	fixture := newCurrentClassificationStoreFixture(t)
	receipt, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		fixture.request,
	)
	if err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}
	if err := fixture.base.store.Close(); err != nil {
		t.Fatalf("close current-classification database: %v", err)
	}
	reopened := openStoreAt(t, fixture.base.databasePath)
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, reopened.GetRawDB())
	if err != nil {
		t.Fatalf("begin reopened current-classification read: %v", err)
	}
	head, err := loadHeadWithScanner(ctx, transaction, fixture.base.project)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("load reopened current-classification graph head: %v", err)
	}
	if _, err := verifyExactV46AdmissionIntegrity(ctx, transaction, head); err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("verify reopened writer-v54 integrity: %v", err)
	}
	catalog, err := loadCurrentKindClassificationSourceCatalog(
		ctx,
		transaction,
		head,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("load reopened current-classification sources: %v", err)
	}
	blobs := catalog.Blobs()
	if len(blobs) != 1 ||
		blobs[0].Reference() != fixture.source.Reference() ||
		blobs[0].Digest() != fixture.source.Digest() ||
		!bytes.Equal(blobs[0].Bytes(), fixture.source.Bytes()) {
		_ = transaction.Rollback(ctx)
		t.Fatalf("reopened source catalog = %#v; want exact committed source", blobs)
	}
	if head.LastEventRef() != receipt.EventRef() ||
		head.LastCommitRef() != receipt.CommitRef() {
		_ = transaction.Rollback(ctx)
		t.Fatal("reopened source catalog graph head differs from its commit receipt")
	}
	result := transaction.Rollback(ctx)
	if !result.Succeeded() {
		t.Fatalf("rollback reopened current-classification read: %v", result.Err())
	}
}

func assertCurrentClassificationReadIntegrity(
	t *testing.T,
	fixture currentClassificationStoreFixture,
	receipt CommitReceipt,
) {
	t.Helper()
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.base.database)
	if err != nil {
		t.Fatalf("begin current-classification integrity read: %v", err)
	}
	head, err := loadHeadWithScanner(ctx, transaction, fixture.base.project)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("load current-classification graph head: %v", err)
	}
	closure, err := verifyExactV46AdmissionIntegrity(ctx, transaction, head)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("verify writer-v54 read integrity: %v", err)
	}
	if closure == nil ||
		closure.eventRef != receipt.EventRef() ||
		closure.commit != receipt.CommitRef() {
		_ = transaction.Rollback(ctx)
		t.Fatalf("writer-v54 closure = %#v; want receipt %#v", closure, receipt)
	}
	result := transaction.Rollback(ctx)
	if !result.Succeeded() {
		t.Fatalf("rollback current-classification integrity read: %v", result.Err())
	}
}

func TestCurrentKindClassificationCommitRejectsSourceDriftBeforeWrite(
	t *testing.T,
) {
	fixture := newCurrentClassificationStoreFixture(t)
	drifted := currentClassificationSourceBlob(
		t,
		"record:test/current-classification/drifted",
		[]byte("classification-source-drifted"),
	)
	fixture.sourceProvider.blob = drifted

	_, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		fixture.request,
	)
	if !errors.Is(err, ErrAdmissionEnvelopeMismatch) {
		t.Fatalf("source-drift commit error = %v; want ErrAdmissionEnvelopeMismatch", err)
	}
	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_graph_events":                                 0,
		"typed_memory_graph_commits":                                0,
		"typed_memory_kind_classification_source_blobs_v54":         0,
		"typed_memory_kind_classification_evaluations_v54":          0,
		"typed_memory_kind_classification_features_v54":             0,
		"typed_memory_relational_assertion_classification_uses_v54": 0,
	})
}

type currentClassificationStoreFixture struct {
	base           sqliteStoreFixture
	environment    typedmemory.TypeEnv
	registry       typedmemory.CodecRegistry
	adapter        *SQLiteAdapter
	request        CommitRequest
	source         KindClassificationSourceBlob
	sourceProvider *mutableKindClassificationSourceProvider
}

func newCurrentClassificationStoreFixture(
	t *testing.T,
) currentClassificationStoreFixture {
	t.Helper()
	base := newSQLiteStoreFixture(t)
	provenance := mustFPFProvenance(t, base.snapshot.SourceRevision())
	contextDefinition, found := base.environment.BoundedContext(base.context)
	if !found {
		t.Fatal("base fixture bounded context is missing")
	}

	entityKindID := mustGenericKindID(t, "U.Entity")
	entityKind := currentClassificationValueKind(t, base.environment.Ref(), entityKindID)
	entityKindDefinition, err := typedmemory.NewKindDefinition(entityKindID, provenance)
	if err != nil {
		t.Fatalf("NewKindDefinition(U.Entity): %v", err)
	}
	featureKindID := mustGenericKindID(t, "U.Text")
	featureKind := currentClassificationValueKind(t, base.environment.Ref(), featureKindID)
	featureKindDefinition, err := typedmemory.NewKindDefinition(featureKindID, provenance)
	if err != nil {
		t.Fatalf("NewKindDefinition(U.Text): %v", err)
	}

	entityRefKind := currentClassificationRefKind(t, base.environment.Ref())
	refKindDefinition, err := typedmemory.NewRefKindDefinition(
		entityRefKind,
		entityKind,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewRefKindDefinition: %v", err)
	}
	entityAvailability := mustLocalContextKindAvailability(
		t,
		base.environment.Ref(),
		base.context,
		entityKindID,
		provenance,
		"current-classification.entity-availability",
	)
	featureAvailability := mustLocalContextKindAvailability(
		t,
		base.environment.Ref(),
		base.context,
		featureKindID,
		provenance,
		"current-classification.feature-availability",
	)

	shape, shapeDeclaration := currentClassificationShape(t, provenance)
	codec := currentClassificationCodecRef(t)
	binding, err := typedmemory.NewValueBinding(featureKind, shape, codec, provenance)
	if err != nil {
		t.Fatalf("NewValueBinding: %v", err)
	}
	registry, err := typedmemory.NewCodecRegistry().Register(
		codec,
		genericTextCodec{shape: shape},
	)
	if err != nil {
		t.Fatalf("CodecRegistry.Register: %v", err)
	}

	localKind, err := typedmemory.NewLocalKindRef(entityKind, base.context)
	if err != nil {
		t.Fatalf("NewLocalKindRef: %v", err)
	}
	referenceScheme, err := typedmemory.NewKindReferenceSchemePin(
		exactBasisCarrierRef(t, "reference-scheme:test/current-classification"),
		exactBasisCarrierEdition(t, "1.0.0"),
		mustDigest(t, []byte("current-classification-reference-scheme-v1")),
	)
	if err != nil {
		t.Fatalf("NewKindReferenceSchemePin: %v", err)
	}
	criterion := exactBasisRuleRef(t, "rule:test/current-classification/criterion/v1")
	classificationSignature, err := typedmemory.NewKindClassificationSignatureDefinition(
		typedmemory.KindClassificationSignatureDefinitionInput{
			LocalKind:          localKind,
			CandidateValueKind: entityKind,
			Criterion:          criterion,
			SliceConditions:    exactBasisRuleRef(t, "rule:test/current-classification/slice/v1"),
			ReferenceScheme:    referenceScheme,
			Formality:          typedmemory.SignatureF4,
			ExtentRule:         typedmemory.NoKindExtentRule{},
			Provenance:         provenance,
		},
	)
	if err != nil {
		t.Fatalf("NewKindClassificationSignatureDefinition: %v", err)
	}

	referenceSlot := mustGenericSlotKindID(t, "EntityOfConcernSlot")
	target, err := typedmemory.NewReferenceSlotTarget(entityKind, entityRefKind)
	if err != nil {
		t.Fatalf("NewReferenceSlotTarget: %v", err)
	}
	slot, err := typedmemory.NewSlotSpec(
		referenceSlot,
		target,
		typedmemory.ExactlyOneCardinality(),
		provenance,
	)
	if err != nil {
		t.Fatalf("NewSlotSpec: %v", err)
	}
	relationRef, err := typedmemory.NewRelationSignatureRef(
		base.environment.Ref(),
		mustGenericSignatureID(t, "test.CurrentClassificationRelation"),
	)
	if err != nil {
		t.Fatalf("NewRelationSignatureRef: %v", err)
	}
	relation, err := typedmemory.NewRelationSignature(
		relationRef,
		[]typedmemory.BoundedContextRef{base.context},
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
		AddBoundedContext(contextDefinition).
		AddKindDefinition(entityKindDefinition).
		AddKindDefinition(featureKindDefinition).
		AddKindClassificationSignatureDefinition(classificationSignature).
		AddRefKindDefinition(refKindDefinition).
		AddContextKindAvailability(entityAvailability).
		AddContextKindAvailability(featureAvailability).
		AddRelationSignature(relation).
		AddValueShape(shapeDeclaration).
		AddValueBinding(binding).
		Build()
	if err != nil {
		t.Fatalf("build current-classification TypeEnv: %v", err)
	}

	featureValue := currentClassificationFeatureValue(
		t,
		registry,
		binding,
		featureKind,
		shape,
		codec,
	)
	source := currentClassificationSourceBlob(
		t,
		"record:test/current-classification/entity",
		[]byte("classification-source-current"),
	)
	engine := currentClassificationTestEngine{
		expectedProject: base.project,
		signature:       classificationSignature,
		featureValue:    featureValue,
		featureKey:      currentClassificationFeatureKey(t),
		governor:        criterion,
		source:          source,
	}
	request := currentClassificationCommitRequest(
		t,
		base,
		environment,
		registry,
		entityRefKind,
		relationRef,
		referenceSlot,
		engine,
	)
	provider := &mutableKindClassificationSourceProvider{blob: source}
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
	adapter.kindClassificationEngine = engine
	adapter.kindClassificationSources = provider
	return currentClassificationStoreFixture{
		base:           base,
		environment:    environment,
		registry:       registry,
		adapter:        adapter,
		request:        request,
		source:         source,
		sourceProvider: provider,
	}
}

func currentClassificationCommitRequest(
	t *testing.T,
	base sqliteStoreFixture,
	environment typedmemory.TypeEnv,
	registry typedmemory.CodecRegistry,
	entityRefKind typedmemory.RefKindRef,
	relationRef typedmemory.RelationSignatureRef,
	referenceSlot typedmemory.SlotKindID,
	engine currentClassificationTestEngine,
) CommitRequest {
	t.Helper()
	entity := mustGenericEntityID(t, "entity:current-classification")
	localID, err := typedmemory.NewBatchLocalRef("local:current-classification")
	if err != nil {
		t.Fatalf("NewBatchLocalRef: %v", err)
	}
	declaration, err := typedmemory.NewDeclareEntity(
		entity,
		localID,
		base.context,
		exactBasisEntityLabel(t, "Current classification entity"),
		mustGenericProvenanceRef(t, "memory:test:current-classification-declaration"),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity: %v", err)
	}
	localRef, err := typedmemory.NewLocalRef(entityRefKind, localID)
	if err != nil {
		t.Fatalf("NewLocalRef: %v", err)
	}
	filler, err := typedmemory.NewByReferenceCandidate(localRef)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate: %v", err)
	}
	binding, err := typedmemory.NewCandidateSlotBinding(
		referenceSlot,
		[]typedmemory.CandidateSlotFiller{filler},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding: %v", err)
	}
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewGammaPoint: %v", err)
	}
	contextSlice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:   base.context,
		GammaTime: gamma,
	})
	if err != nil {
		t.Fatalf("NewContextSlice: %v", err)
	}
	assertion := mustGenericAssertionID(t, "assertion:current-classification")
	relation, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  assertion,
			Signature:  relationRef,
			Slice:      contextSlice,
			Modality:   typedmemory.NewAffirmsObtaining(),
			Bindings:   []typedmemory.CandidateSlotBinding{binding},
			Provenance: mustGenericProvenanceRef(t, "memory:test:current-classification-relation"),
		},
	)
	if err != nil {
		t.Fatalf("NewRelationalAssertionCandidate: %v", err)
	}
	assertRelation, err := typedmemory.NewAssertRelation(relation)
	if err != nil {
		t.Fatalf("NewAssertRelation: %v", err)
	}
	candidate, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{
		declaration,
		assertRelation,
	})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}
	graphRevision := typedmemory.NewGraphRevision(0)
	resolutionBasis, err := snapshotResolutionBasis(base.project, graphRevision)
	if err != nil {
		t.Fatalf("snapshotResolutionBasis: %v", err)
	}
	entityAbsent, err := typedmemory.NewAbsentEntityResolution(
		entity,
		base.context,
		resolutionBasis,
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	assertionRule, err := snapshotAssertionRule(base.project, graphRevision)
	if err != nil {
		t.Fatalf("snapshotAssertionRule: %v", err)
	}
	assertionAbsent, err := typedmemory.NewAbsentAssertionState(assertion, assertionRule)
	if err != nil {
		t.Fatalf("NewAbsentAssertionState: %v", err)
	}
	snapshot := currentClassificationValidationSnapshot{
		revision:        graphRevision,
		typeEnv:         environment.Ref(),
		entityAbsent:    entityAbsent,
		assertionAbsent: assertionAbsent,
		engine:          engine,
	}
	verdict := typedmemory.ValidateMemoryChangeSet(
		environment,
		registry,
		snapshot,
		candidate,
	)
	valid, ok := verdict.(typedmemory.Valid)
	if !ok {
		t.Fatalf("ValidateMemoryChangeSet = %T (%s); want Valid", verdict, verdict.Kind())
	}
	if _, ok := valid.AdmissionBatch().Basis().(typedmemory.ContextSliceClassificationBasis); !ok {
		t.Fatalf(
			"admission basis = %T; want ContextSliceClassificationBasis",
			valid.AdmissionBatch().Basis(),
		)
	}
	request := base.request(
		t,
		0,
		environment.Ref(),
		"current-kind-classification-v54",
		candidate,
	)
	request.admissionBatch = valid.AdmissionBatch()
	return request
}

type currentClassificationValidationSnapshot struct {
	revision        typedmemory.GraphRevision
	typeEnv         typedmemory.TypeEnvRef
	entityAbsent    typedmemory.AbsentEntityResolution
	assertionAbsent typedmemory.AssertionState
	engine          currentClassificationTestEngine
}

func (snapshot currentClassificationValidationSnapshot) GraphRevision() typedmemory.GraphRevision {
	return snapshot.revision
}

func (snapshot currentClassificationValidationSnapshot) TypeEnvRef() typedmemory.TypeEnvRef {
	return snapshot.typeEnv
}

func (snapshot currentClassificationValidationSnapshot) ResolveEntity(
	typedmemory.EntityID,
	typedmemory.BoundedContextRef,
) typedmemory.EntityResolution {
	return snapshot.entityAbsent
}

func (currentClassificationValidationSnapshot) ResolveReference(
	typedmemory.StrongRef,
	typedmemory.BoundedContextRef,
) typedmemory.StrongReferenceResolution {
	return nil
}

func (currentClassificationValidationSnapshot) EvaluateMemberOf(
	typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	return nil
}

func (snapshot currentClassificationValidationSnapshot) EvaluateKindClassification(
	request typedmemory.KindClassificationRequest,
) typedmemory.KindClassificationJudgement {
	judgement, err := snapshot.engine.judgement(request)
	if err != nil {
		return nil
	}
	return judgement
}

func (snapshot currentClassificationValidationSnapshot) AssertionState(
	typedmemory.AssertionID,
) typedmemory.AssertionState {
	return snapshot.assertionAbsent
}

func (currentClassificationValidationSnapshot) ResolveAlias(
	typedmemory.EntityAlias,
	typedmemory.BoundedContextRef,
) typedmemory.AliasAvailability {
	return nil
}

func (currentClassificationValidationSnapshot) ResolveReconciliationBasis(
	basis typedmemory.ReconciliationBasisRef,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.ReconciliationBasisResolution {
	resolution, _ := typedmemory.NewMissingReconciliationBasis(basis, contextRef)
	return resolution
}

type currentClassificationTestEngine struct {
	expectedProject projectledger.ProjectID
	signature       typedmemory.KindClassificationSignatureDefinition
	featureValue    typedmemory.VerifiedTypedValue
	featureKey      typedmemory.KindFeatureKey
	governor        typedmemory.RuleRef
	source          KindClassificationSourceBlob
}

func (engine currentClassificationTestEngine) EvaluateKindClassification(
	_ context.Context,
	input KindClassificationAdmissionInput,
) (typedmemory.KindClassificationJudgement, error) {
	if input.ProjectID() != engine.expectedProject {
		return nil, fmt.Errorf("current-classification project mismatch")
	}
	sources := input.Sources()
	if len(sources) != 1 ||
		sources[0].Reference() != engine.source.Reference() ||
		sources[0].Digest() != engine.source.Digest() ||
		!bytes.Equal(sources[0].Bytes(), engine.source.Bytes()) {
		return nil, fmt.Errorf("current-classification source mismatch")
	}
	return engine.judgement(input.Request())
}

func (engine currentClassificationTestEngine) judgement(
	request typedmemory.KindClassificationRequest,
) (typedmemory.KindClassificationJudgement, error) {
	feature, err := typedmemory.NewGovernedCandidateFeature(
		typedmemory.GovernedCandidateFeatureInput{
			Key:          engine.featureKey,
			Value:        engine.featureValue,
			Governor:     engine.governor,
			Source:       engine.source.Reference(),
			SourceDigest: engine.source.Digest(),
		},
	)
	if err != nil {
		return nil, err
	}
	features, err := typedmemory.NewGovernedCandidateFeatureSet(
		request,
		[]typedmemory.GovernedCandidateFeature{feature},
	)
	if err != nil {
		return nil, err
	}
	basis, err := typedmemory.NewKindClassificationEvaluationBasis(
		request,
		engine.signature,
		features,
	)
	if err != nil {
		return nil, err
	}
	return typedmemory.NewTrueKindClassification(request, basis)
}

type mutableKindClassificationSourceProvider struct {
	blob KindClassificationSourceBlob
}

func (provider *mutableKindClassificationSourceProvider) LoadKindClassificationSource(
	_ context.Context,
	_ projectledger.ProjectID,
	_ typedmemory.CarrierRef,
	_ typedmemory.SHA256Digest,
) (KindClassificationSourceBlob, error) {
	return provider.blob, nil
}

func currentClassificationValueKind(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
	kind typedmemory.KindID,
) typedmemory.ValueKindRef {
	t.Helper()
	value, err := typedmemory.NewValueKindRef(typeEnv, kind)
	if err != nil {
		t.Fatalf("NewValueKindRef(%s): %v", kind.String(), err)
	}
	return value
}

func currentClassificationRefKind(
	t *testing.T,
	typeEnv typedmemory.TypeEnvRef,
) typedmemory.RefKindRef {
	t.Helper()
	id, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatalf("NewRefKindID: %v", err)
	}
	value, err := typedmemory.NewRefKindRef(typeEnv, id)
	if err != nil {
		t.Fatalf("NewRefKindRef: %v", err)
	}
	return value
}

func currentClassificationShape(
	t *testing.T,
	provenance typedmemory.DeclarationProvenance,
) (typedmemory.ValueShapeRef, typedmemory.ValueShapeDeclaration) {
	t.Helper()
	shape, err := typedmemory.NewScalarShape(typedmemory.ScalarText)
	if err != nil {
		t.Fatalf("NewScalarShape: %v", err)
	}
	shapeRef, err := typedmemory.DeriveValueShapeRef(
		mustGenericShapeID(t, "CurrentClassificationFeatureShape"),
		shape,
	)
	if err != nil {
		t.Fatalf("DeriveValueShapeRef: %v", err)
	}
	declaration, err := typedmemory.NewValueShapeDeclaration(
		shapeRef,
		shape,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewValueShapeDeclaration: %v", err)
	}
	return shapeRef, declaration
}

func currentClassificationCodecRef(t *testing.T) typedmemory.CodecRef {
	t.Helper()
	value, err := typedmemory.NewCodecRef(
		mustGenericCodecID(t, "CurrentClassificationFeatureCodec"),
		mustGenericCodecVersion(t, "1"),
		mustDigest(t, []byte("current-classification-feature-codec-v1")),
	)
	if err != nil {
		t.Fatalf("NewCodecRef: %v", err)
	}
	return value
}

func currentClassificationFeatureValue(
	t *testing.T,
	registry typedmemory.CodecRegistry,
	binding typedmemory.ValueBinding,
	valueKind typedmemory.ValueKindRef,
	shape typedmemory.ValueShapeRef,
	codec typedmemory.CodecRef,
) typedmemory.VerifiedTypedValue {
	t.Helper()
	candidate, err := typedmemory.NewTypedValueCandidate(
		valueKind,
		shape,
		codec,
		[]byte("classified"),
		typedmemory.NoAssertedDigest{},
	)
	if err != nil {
		t.Fatalf("NewTypedValueCandidate: %v", err)
	}
	verification := typedmemory.VerifyTypedValue(registry, binding, candidate)
	valid, ok := verification.(typedmemory.ValidTypedValue)
	if !ok {
		t.Fatalf("VerifyTypedValue = %T; want ValidTypedValue", verification)
	}
	return valid.Value()
}

func currentClassificationFeatureKey(t *testing.T) typedmemory.KindFeatureKey {
	t.Helper()
	value, err := typedmemory.NewKindFeatureKey("entity.current-classification")
	if err != nil {
		t.Fatalf("NewKindFeatureKey: %v", err)
	}
	return value
}

func currentClassificationSourceBlob(
	t *testing.T,
	referenceText string,
	content []byte,
) KindClassificationSourceBlob {
	t.Helper()
	reference := exactBasisCarrierRef(t, referenceText)
	digest := mustDigest(t, content)
	blob, err := NewKindClassificationSourceBlob(reference, digest, content)
	if err != nil {
		t.Fatalf("NewKindClassificationSourceBlob: %v", err)
	}
	return blob
}

func assertCurrentClassificationWriter(
	t *testing.T,
	fixture currentClassificationStoreFixture,
	eventRef string,
) {
	t.Helper()
	var writer int64
	var provenance string
	var basisKind string
	err := fixture.base.database.QueryRow(
		`SELECT writer.writer_generation, writer.provenance_kind, basis.admission_basis_kind
		 FROM typed_memory_event_writer_generations writer
		 JOIN typed_memory_event_admission_bases basis
		   ON basis.project_id = writer.project_id AND basis.event_ref = writer.event_ref
		 WHERE writer.project_id = ? AND writer.event_ref = ?`,
		fixture.base.project.String(),
		eventRef,
	).Scan(&writer, &provenance, &basisKind)
	if err != nil {
		t.Fatalf("load current-classification writer: %v", err)
	}
	if writer != 54 || provenance != "writer_v54" || basisKind != "context_slice_classification" {
		t.Fatalf(
			"writer/basis = (%d, %q, %q); want (54, writer_v54, context_slice_classification)",
			writer,
			provenance,
			basisKind,
		)
	}
}

func assertCurrentClassificationRows(
	t *testing.T,
	fixture currentClassificationStoreFixture,
	eventRef string,
) {
	t.Helper()
	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_graph_events":                                      1,
		"typed_memory_graph_commits":                                     1,
		"typed_memory_event_admission_bases":                             1,
		"typed_memory_commit_materialization_closures":                   1,
		"typed_memory_reference_resolution_uses":                         0,
		"typed_memory_relational_assertion_reference_resolution_uses_v3": 1,
		"typed_memory_memberof_evaluations":                              0,
		"typed_memory_memberof_observable_inputs":                        0,
		"typed_memory_observable_input_blobs":                            0,
		"typed_memory_kind_classification_source_blobs_v54":              1,
		"typed_memory_kind_classification_evaluations_v54":               1,
		"typed_memory_kind_classification_features_v54":                  1,
		"typed_memory_relational_assertion_classification_uses_v54":      1,
	})
	var sourceRef string
	var sourceDigest string
	var sourceBytes []byte
	err := fixture.base.database.QueryRow(
		`SELECT source_ref, source_digest, canonical_source_bytes
		 FROM typed_memory_kind_classification_source_blobs_v54
		 WHERE project_id = ? AND event_ref = ?`,
		fixture.base.project.String(),
		eventRef,
	).Scan(&sourceRef, &sourceDigest, &sourceBytes)
	if err != nil {
		t.Fatalf("load current-classification source row: %v", err)
	}
	if sourceRef != fixture.source.Reference().String() ||
		sourceDigest != fixture.source.Digest().String() ||
		!bytes.Equal(sourceBytes, fixture.source.Bytes()) {
		t.Fatal("current-classification source row lost its exact external bytes")
	}
}
