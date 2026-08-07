package typedmemorystore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// These tests deliberately inject storage faults after the writer has emitted
// a row. They prove that the sealed admission basis, rather than a merely
// self-consistent SQL footprint, remains the authority for materialization.
// A writer-generated mismatch must be detected inside the same BEGIN IMMEDIATE
// transaction so that the graph head and every event row can roll back.

func TestGenericCommitRejectsWrongSameBatchViewWitnessBeforeCommit(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	fixture.allowTestMutation(t, "typed_memory_memberof_evaluations")
	_, err := fixture.base.database.Exec(`CREATE TRIGGER exact_basis_test_wrong_view
		AFTER INSERT ON typed_memory_memberof_evaluations
		BEGIN
			UPDATE typed_memory_memberof_evaluations
			SET view_batch_local_ref = 'local:wrong-same-batch-witness'
			WHERE project_id = NEW.project_id
				AND event_ref = NEW.event_ref
				AND evaluation_ref = NEW.evaluation_ref;
		END`)
	if err != nil {
		t.Fatalf("install wrong-view trigger: %v", err)
	}

	request := fixture.request(t, "exact-basis-wrong-view")
	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err == nil {
		t.Fatal("writer-generated same-batch view/resolution witness mismatch committed")
	}
	if errors.Is(err, ErrCommitOutcomeUnknown) {
		t.Fatalf("wrong view was first detected after COMMIT: %v", err)
	}
	fixture.assertNoSemanticCommit(t)
}

func TestGenericCommitMemberOfEvaluationRejectsCrossProjectSubstitution(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	foreignProject := mustProjectID(t, "qnt_deadbeef")
	fixture.adapter.memberOfEngine = exactBasisMemberOfEngine{
		expectedProject: foreignProject,
		kindSignature:   fixture.kindSignature,
		entitySet:       fixture.entitySet,
		provenance:      fixture.evaluationSource,
	}

	request := fixture.request(t, "exact-basis-cross-project")
	_, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err == nil || !errors.Is(err, errExactBasisProjectMismatch) {
		t.Fatalf("cross-project MemberOf evaluation error = %v; want project mismatch", err)
	}
	fixture.assertNoSemanticCommit(t)
}

func TestMemberOfEvaluationInputRejectsMissingProject(t *testing.T) {
	_, err := newMemberOfEvaluationInput(
		projectledger.ProjectID{},
		typedmemory.TypeEnv{},
		typedmemory.MemberOfEvaluationRequest{},
		nil,
		memberofevaluation.NewPersistedEntityUniverseUnavailable(),
	)
	if err == nil {
		t.Fatal("MemberOf evaluation input admitted a missing project")
	}
}

func TestGenericCommitRejectsWrongObservableBasisProjectionBeforeCommit(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	fixture.allowTestMutation(t, "typed_memory_memberof_observable_inputs")
	_, err := fixture.base.database.Exec(`CREATE TRIGGER exact_basis_test_wrong_input_ordinal
		AFTER INSERT ON typed_memory_memberof_observable_inputs
		BEGIN
			UPDATE typed_memory_memberof_observable_inputs
			SET input_ordinal = input_ordinal + 100
			WHERE project_id = NEW.project_id
				AND event_ref = NEW.event_ref
				AND evaluation_ref = NEW.evaluation_ref
				AND input_ordinal = NEW.input_ordinal;
		END`)
	if err != nil {
		t.Fatalf("install wrong-input trigger: %v", err)
	}

	request := fixture.request(t, "exact-basis-wrong-input")
	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err == nil {
		t.Fatal("writer-generated observable-input projection drift committed")
	}
	if errors.Is(err, ErrCommitOutcomeUnknown) {
		t.Fatalf("wrong observable projection was first detected after COMMIT: %v", err)
	}
	fixture.assertNoSemanticCommit(t)
}

func TestGenericReplayRejectsClosureConsistentWrongBasisProjection(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "exact-basis-replay")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}

	fixture.allowTestMutation(t, "typed_memory_memberof_observable_inputs")
	result, err := fixture.base.database.Exec(`UPDATE typed_memory_memberof_observable_inputs
		SET input_ordinal = input_ordinal + 100
		WHERE project_id = ? AND event_ref = ?`, fixture.base.project.String(), receipt.EventRef())
	if err != nil {
		t.Fatalf("inject closure-consistent input drift: %v", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("read injected row count: %v", err)
	}
	if affected != int64(len(fixture.observableInputs)) {
		t.Fatalf("mutated input rows = %d; want %d", affected, len(fixture.observableInputs))
	}

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("wrong exact-basis replay error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestGenericReplayRejectsClosureConsistentWrongSameBatchWitness(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "exact-basis-replay-witness")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}

	fixture.allowTestMutation(t, "typed_memory_memberof_evaluations")
	result, err := fixture.base.database.Exec(`UPDATE typed_memory_memberof_evaluations
		SET view_batch_local_ref = 'local:wrong-same-batch-witness'
		WHERE project_id = ? AND event_ref = ?
			AND evaluation_view_kind = 'prospective_batch'`,
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("inject closure-consistent view witness drift: %v", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("read injected row count: %v", err)
	}
	if affected != 1 {
		t.Fatalf("mutated prospective views = %d; want 1", affected)
	}

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("wrong same-batch witness replay error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestGenericReplayRejectsInternallyCorrelatedButWrongSameBatchWitness(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "exact-basis-replay-correlated-witness")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}

	fixture.allowTestMutation(t, "typed_memory_relational_assertion_reference_resolution_uses_v3")
	fixture.allowTestMutation(t, "typed_memory_memberof_evaluations")
	transaction, err := fixture.base.database.Begin()
	if err != nil {
		t.Fatalf("begin correlated-witness mutation: %v", err)
	}
	wrongKind := "typeenv:wrong/ref-kind/U.WrongRef"
	resolutionResult, err := transaction.Exec(`UPDATE typed_memory_relational_assertion_reference_resolution_uses_v3
		SET local_reference_kind_ref = ?
		WHERE project_id = ? AND event_ref = ?
			AND resolution_kind = 'same_batch_declaration'`,
		wrongKind,
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("mutate correlated resolution witness: %v", err)
	}
	assertExactBasisRowsAffected(t, resolutionResult, 1, "same-batch resolutions")
	viewResult, err := transaction.Exec(`UPDATE typed_memory_memberof_evaluations
		SET view_local_reference_kind_ref = ?
		WHERE project_id = ? AND event_ref = ?
			AND evaluation_view_kind = 'prospective_batch'`,
		wrongKind,
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("mutate correlated evaluation witness: %v", err)
	}
	assertExactBasisRowsAffected(t, viewResult, 1, "prospective views")
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit internally correlated witness mutation: %v", err)
	}

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("correlated wrong same-batch witness replay error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestGenericReplayRejectsClosureConsistentWrongResolutionWitness(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "exact-basis-replay-resolution")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}

	fixture.allowTestMutation(t, "typed_memory_relational_assertion_reference_resolution_uses_v3")
	result, err := fixture.base.database.Exec(`UPDATE typed_memory_relational_assertion_reference_resolution_uses_v3
		SET local_reference_kind_ref = 'typeenv:wrong/ref-kind/U.WrongRef'
		WHERE project_id = ? AND event_ref = ?
			AND resolution_kind = 'same_batch_declaration'`,
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("inject closure-consistent resolution witness drift: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "same-batch resolutions")

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("wrong resolution witness replay error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestGenericReplayRejectsClosureConsistentWrongMemberUseCoordinate(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "exact-basis-replay-member-use")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}

	fixture.allowTestMutation(t, "typed_memory_relational_assertion_memberof_uses_v3")
	result, err := fixture.base.database.Exec(`UPDATE typed_memory_relational_assertion_memberof_uses_v3
		SET queried_value_kind_ref = 'typeenv:wrong/kind/U.WrongKind'
		WHERE project_id = ? AND event_ref = ?
			AND use_kind = 'required_member'`,
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		t.Fatalf("inject closure-consistent MemberOf-use drift: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "required MemberOf uses")

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("wrong MemberOf-use replay error = %v; want ErrStoredAdmissionIntegrity", err)
	}
}

func TestGenericReplayClassifiesSameBytesWrongStoredDigestAsIntegrityFailure(t *testing.T) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "exact-basis-replay-stored-digest")
	receipt, err := fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if err != nil {
		t.Fatalf("seed exact-basis commit: %v", err)
	}

	fixture.allowTestMutation(t, "typed_memory_event_admission_bases")
	fixture.allowTestMutation(t, "typed_memory_commit_materialization_closures")
	fixture.allowTestMutation(t, "typed_memory_ordered_candidate_prefixes")
	wrongDigest := mustDigest(t, []byte("wrong stored request digest"))
	transaction, err := fixture.base.database.Begin()
	if err != nil {
		t.Fatalf("begin stored-digest corruption transaction: %v", err)
	}
	if _, err := transaction.Exec(`PRAGMA defer_foreign_keys = ON`); err != nil {
		_ = transaction.Rollback()
		t.Fatalf("defer corruption-test foreign keys: %v", err)
	}
	result, err := transaction.Exec(`UPDATE typed_memory_event_admission_bases
		SET request_digest = ?
		WHERE project_id = ? AND event_ref = ?`,
		wrongDigest.String(),
		fixture.base.project.String(),
		receipt.EventRef(),
	)
	if err != nil {
		_ = transaction.Rollback()
		t.Fatalf("inject stored request-digest drift: %v", err)
	}
	assertExactBasisRowsAffected(t, result, 1, "stored admission request digests")
	for _, table := range []string{
		"typed_memory_commit_materialization_closures",
		"typed_memory_ordered_candidate_prefixes",
	} {
		result, updateErr := transaction.Exec(
			"UPDATE "+table+" SET request_digest = ? WHERE project_id = ? AND event_ref = ?",
			wrongDigest.String(),
			fixture.base.project.String(),
			receipt.EventRef(),
		)
		if updateErr != nil {
			_ = transaction.Rollback()
			t.Fatalf("propagate stored request-digest drift to %s: %v", table, updateErr)
		}
		assertExactBasisRowsAffected(t, result, 1, table+" request digests")
	}
	if err := transaction.Commit(); err != nil {
		t.Fatalf("commit stored-digest corruption fixture: %v", err)
	}

	_, err = fixture.adapter.CommitMemoryChangeSet(context.Background(), request)
	if !errors.Is(err, ErrStoredAdmissionIntegrity) {
		t.Fatalf("wrong stored request digest error = %v; want ErrStoredAdmissionIntegrity", err)
	}
	if errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("same-key same-bytes storage corruption was misclassified as caller conflict: %v", err)
	}
}

func assertExactBasisRowsAffected(
	t *testing.T,
	result sql.Result,
	want int64,
	label string,
) {
	t.Helper()
	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("read %s mutation count: %v", label, err)
	}
	if affected != want {
		t.Fatalf("mutated %s = %d; want %d", label, affected, want)
	}
}

type exactBasisStoreFixture struct {
	base             sqliteStoreFixture
	environment      typedmemory.TypeEnv
	adapter          *SQLiteAdapter
	contextSlice     typedmemory.ContextSlice
	entityKind       typedmemory.ValueKindRef
	entityRefKind    typedmemory.RefKindRef
	relation         typedmemory.RelationSignatureRef
	referenceSlot    typedmemory.SlotKindID
	entitySet        typedmemory.EntitySetDefinition
	kindSignature    typedmemory.KindSignatureDefinition
	evaluationSource typedmemory.MemberOfEvaluationProvenance
	observableInputs []typedmemory.MemberOfObservableInput
	observableBlobs  []ObservableInputBlob
}

func newExactBasisStoreFixture(t *testing.T) exactBasisStoreFixture {
	t.Helper()
	base := newSQLiteStoreFixture(t)
	provenance := mustFPFProvenance(t, base.snapshot.SourceRevision())
	boundedContext, exists := base.environment.BoundedContext(base.context)
	if !exists {
		t.Fatal("base fixture bounded context is missing")
	}
	entityKindID := mustGenericKindID(t, "U.Entity")
	entityKindDefinition, err := typedmemory.NewKindDefinition(entityKindID, provenance)
	if err != nil {
		t.Fatalf("NewKindDefinition: %v", err)
	}
	entityKind, err := typedmemory.NewValueKindRef(base.environment.Ref(), entityKindID)
	if err != nil {
		t.Fatalf("NewValueKindRef: %v", err)
	}
	refKindID, err := typedmemory.NewRefKindID("U.EntityRef")
	if err != nil {
		t.Fatalf("NewRefKindID: %v", err)
	}
	entityRefKind, err := typedmemory.NewRefKindRef(base.environment.Ref(), refKindID)
	if err != nil {
		t.Fatalf("NewRefKindRef: %v", err)
	}
	refKindDefinition, err := typedmemory.NewRefKindDefinition(
		entityRefKind,
		entityKind,
		provenance,
	)
	if err != nil {
		t.Fatalf("NewRefKindDefinition: %v", err)
	}
	admission := mustLocalContextKindAvailability(
		t,
		base.environment.Ref(),
		base.context,
		entityKindID,
		provenance,
		"exact-basis.availability",
	)
	prospectiveRule := exactBasisRuleRef(t, "test:entity-set/prospective/v1")
	prospectivePolicy, err := typedmemory.NewPriorBatchDeclarationsVisible(prospectiveRule)
	if err != nil {
		t.Fatalf("NewPriorBatchDeclarationsVisible: %v", err)
	}
	entitySet, err := typedmemory.NewEntitySetDefinition(typedmemory.EntitySetDefinitionInput{
		TypeEnv:         base.environment.Ref(),
		Context:         base.context,
		EnumerationRule: exactBasisRuleRef(t, "test:entity-set/persisted/v1"),
		CandidatePolicy: prospectivePolicy,
		Provenance:      provenance,
	})
	if err != nil {
		t.Fatalf("NewEntitySetDefinition: %v", err)
	}
	kindSignature, err := typedmemory.NewKindSignatureDefinition(
		typedmemory.KindSignatureDefinitionInput{
			ValueKind:       entityKind,
			Formality:       typedmemory.SignatureF4,
			DefinednessRule: exactBasisRuleRef(t, "test:member-of/entity/definedness/v1"),
			Evaluator:       exactBasisRuleRef(t, "test:member-of/entity/evaluator/v1"),
			EntitySet:       entitySet.Ref(),
			Provenance:      provenance,
		},
	)
	if err != nil {
		t.Fatalf("NewKindSignatureDefinition: %v", err)
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
		mustGenericSignatureID(t, "test.ExactBasisRelation"),
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
		AddBoundedContext(boundedContext).
		AddKindDefinition(entityKindDefinition).
		AddEntitySetDefinition(entitySet).
		AddKindSignatureDefinition(kindSignature).
		AddRefKindDefinition(refKindDefinition).
		AddContextKindAvailability(admission).
		AddRelationSignature(relation).
		Build()
	if err != nil {
		t.Fatalf("build exact-basis TypeEnv: %v", err)
	}
	gamma, err := typedmemory.NewGammaPoint(time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC))
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
	evaluationSource, err := typedmemory.NewMemberOfEvaluationProvenance(
		typedmemory.MemberOfEvaluationProvenanceInput{
			Reference:         mustGenericProvenanceRef(t, "memory:test:exact-basis-evaluation"),
			EvaluatorArtifact: exactBasisCarrierRef(t, "binary:exact-basis-evaluator"),
			EvaluatorEdition:  exactBasisCarrierEdition(t, "build-20260716.1"),
			EvaluatorDigest:   mustDigest(t, []byte("exact-basis-evaluator-v1")),
		},
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationProvenance: %v", err)
	}
	inputA, blobA := exactBasisObservable(t, "observable:exact-basis/a", []byte("input-a"))
	inputB, blobB := exactBasisObservable(t, "observable:exact-basis/b", []byte("input-b"))
	inputs := []typedmemory.MemberOfObservableInput{inputB, inputA}
	blobs := []ObservableInputBlob{blobB, blobA}
	memberEngine := exactBasisMemberOfEngine{
		expectedProject: base.project,
		kindSignature:   kindSignature,
		entitySet:       entitySet,
		provenance:      evaluationSource,
	}
	provider := exactBasisObservableProvider{blobs: map[string]ObservableInputBlob{
		blobA.Reference().String(): blobA,
		blobB.Reference().String(): blobB,
	}}
	loader := staticTypeEnvLoader{
		reference:   environment.Ref(),
		environment: environment,
		registry:    typedmemory.NewCodecRegistry(),
	}
	adapter, err := NewGenericSQLiteAdapter(
		base.database,
		loader,
		base.clock,
		memberEngine,
		unexpectedReferenceEngine{},
		provider,
	)
	if err != nil {
		t.Fatalf("NewGenericSQLiteAdapter: %v", err)
	}
	return exactBasisStoreFixture{
		base:             base,
		environment:      environment,
		adapter:          adapter,
		contextSlice:     contextSlice,
		entityKind:       entityKind,
		entityRefKind:    entityRefKind,
		relation:         relationRef,
		referenceSlot:    referenceSlot,
		entitySet:        entitySet,
		kindSignature:    kindSignature,
		evaluationSource: evaluationSource,
		observableInputs: inputs,
		observableBlobs:  blobs,
	}
}

func (fixture exactBasisStoreFixture) request(t *testing.T, key string) CommitRequest {
	t.Helper()
	return fixture.requestWithRelationFactory(t, key, fixture.assertRelationChange)
}

// legacyRequest constructs exact frozen v1 candidate bytes for replay/read
// compatibility tests. It must never be used as a fresh admission fixture.
func (fixture exactBasisStoreFixture) legacyRequest(t *testing.T, key string) CommitRequest {
	t.Helper()
	request := fixture.requestWithRelationFactory(t, key, fixture.instantiateRelationChange)
	return requestWithContractVersion(t, request, AdmissionContractV1())
}

type exactBasisRelationChangeFactory func(
	*testing.T,
	typedmemory.AssertionID,
	[]typedmemory.CandidateSlotBinding,
) typedmemory.MemoryChange

func (fixture exactBasisStoreFixture) requestWithRelationFactory(
	t *testing.T,
	key string,
	newRelation exactBasisRelationChangeFactory,
) CommitRequest {
	t.Helper()
	entity := mustGenericEntityID(t, "entity:exact-basis")
	localID, err := typedmemory.NewBatchLocalRef("local:exact-basis")
	if err != nil {
		t.Fatalf("NewBatchLocalRef: %v", err)
	}
	declaration, err := typedmemory.NewDeclareEntity(
		entity,
		localID,
		fixture.base.context,
		exactBasisEntityLabel(t, "Exact basis entity"),
		mustGenericProvenanceRef(t, "memory:test:exact-basis-declaration"),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity: %v", err)
	}
	localRef, err := typedmemory.NewLocalRef(fixture.entityRefKind, localID)
	if err != nil {
		t.Fatalf("NewLocalRef: %v", err)
	}
	filler, err := typedmemory.NewByReferenceCandidate(localRef)
	if err != nil {
		t.Fatalf("NewByReferenceCandidate: %v", err)
	}
	binding, err := typedmemory.NewCandidateSlotBinding(
		fixture.referenceSlot,
		[]typedmemory.CandidateSlotFiller{filler},
	)
	if err != nil {
		t.Fatalf("NewCandidateSlotBinding: %v", err)
	}
	assertion := mustGenericAssertionID(t, "assertion:exact-basis")
	change := newRelation(
		t,
		assertion,
		[]typedmemory.CandidateSlotBinding{binding},
	)
	candidate, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{
		declaration,
		change,
	})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet: %v", err)
	}
	absenceBasis, err := snapshotResolutionBasis(
		fixture.base.project,
		typedmemory.NewGraphRevision(0),
	)
	if err != nil {
		t.Fatalf("snapshotResolutionBasis: %v", err)
	}
	entityAbsent, err := typedmemory.NewAbsentEntityResolution(
		entity,
		fixture.base.context,
		absenceBasis,
	)
	if err != nil {
		t.Fatalf("NewAbsentEntityResolution: %v", err)
	}
	assertionRule, err := snapshotAssertionRule(
		fixture.base.project,
		typedmemory.NewGraphRevision(0),
	)
	if err != nil {
		t.Fatalf("snapshotAssertionRule: %v", err)
	}
	assertionAbsent, err := typedmemory.NewAbsentAssertionState(
		assertion,
		assertionRule,
	)
	if err != nil {
		t.Fatalf("NewAbsentAssertionState: %v", err)
	}
	snapshot := exactBasisSnapshot{
		revision:        typedmemory.NewGraphRevision(0),
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
		t.Fatalf("ValidateMemoryChangeSet = %T (%s); want Valid", verdict, verdict.Kind())
	}
	request := fixture.base.request(
		t,
		0,
		fixture.environment.Ref(),
		key,
		candidate,
	)
	request.admissionBatch = valid.AdmissionBatch()
	return request
}

func (fixture exactBasisStoreFixture) assertRelationChange(
	t *testing.T,
	assertion typedmemory.AssertionID,
	bindings []typedmemory.CandidateSlotBinding,
) typedmemory.MemoryChange {
	t.Helper()
	relation, err := typedmemory.NewRelationalAssertionCandidate(
		typedmemory.RelationalAssertionCandidateInput{
			Assertion:  assertion,
			Signature:  fixture.relation,
			Slice:      fixture.contextSlice,
			Modality:   typedmemory.NewAffirmsObtaining(),
			Bindings:   bindings,
			Provenance: mustGenericProvenanceRef(t, "memory:test:exact-basis-relation"),
		},
	)
	if err != nil {
		t.Fatalf("NewRelationalAssertionCandidate: %v", err)
	}
	instantiate, err := typedmemory.NewAssertRelation(relation)
	if err != nil {
		t.Fatalf("NewAssertRelation: %v", err)
	}
	return instantiate
}

func (fixture exactBasisStoreFixture) instantiateRelationChange(
	t *testing.T,
	assertion typedmemory.AssertionID,
	bindings []typedmemory.CandidateSlotBinding,
) typedmemory.MemoryChange {
	t.Helper()
	relation, err := typedmemory.NewRelationInstantiation(
		assertion,
		fixture.relation,
		fixture.contextSlice,
		bindings,
		mustGenericProvenanceRef(t, "memory:test:exact-basis-relation"),
	)
	if err != nil {
		t.Fatalf("NewRelationInstantiation(legacy): %v", err)
	}
	instantiate, err := typedmemory.NewInstantiateRelation(relation)
	if err != nil {
		t.Fatalf("NewInstantiateRelation(legacy): %v", err)
	}
	return instantiate
}

func (fixture exactBasisStoreFixture) allowTestMutation(t *testing.T, table string) {
	t.Helper()
	trigger := table + "_v46_no_update"
	if strings.HasSuffix(table, "_v3") {
		trigger = table + "_v53_no_update"
	}
	if _, err := fixture.base.database.Exec("DROP TRIGGER " + trigger); err != nil {
		t.Fatalf("drop %s: %v", trigger, err)
	}
}

func (fixture exactBasisStoreFixture) assertNoSemanticCommit(t *testing.T) {
	t.Helper()
	head, err := fixture.adapter.LoadHead(context.Background(), fixture.base.project)
	if err != nil {
		t.Fatalf("LoadHead: %v", err)
	}
	if head.Revision().Value() != 0 {
		t.Fatalf("graph revision = %d; want rolled-back revision 0", head.Revision().Value())
	}
	assertTypedMemoryRowCounts(t, fixture.base.database, map[string]int64{
		"typed_memory_graph_events":                    0,
		"typed_memory_graph_commits":                   0,
		"typed_memory_event_admission_bases":           0,
		"typed_memory_commit_materialization_closures": 0,
		"typed_memory_entities":                        0,
		"typed_memory_entity_contexts":                 0,
		"typed_memory_entity_declarations":             0,
		"typed_memory_reference_resolution_uses":       0,
		"typed_memory_memberof_evaluations":            0,
		"typed_memory_memberof_observable_inputs":      0,
		"typed_memory_observable_input_blobs":          0,
	})
}

type exactBasisSnapshot struct {
	revision        typedmemory.GraphRevision
	typeEnv         typedmemory.TypeEnvRef
	entityAbsent    typedmemory.AbsentEntityResolution
	assertionAbsent typedmemory.AssertionState
	memberEngine    exactBasisMemberOfEngine
	inputs          []typedmemory.MemberOfObservableInput
}

func (snapshot exactBasisSnapshot) GraphRevision() typedmemory.GraphRevision {
	return snapshot.revision
}

func (snapshot exactBasisSnapshot) TypeEnvRef() typedmemory.TypeEnvRef { return snapshot.typeEnv }

func (snapshot exactBasisSnapshot) ResolveEntity(
	typedmemory.EntityID,
	typedmemory.BoundedContextRef,
) typedmemory.EntityResolution {
	return snapshot.entityAbsent
}

func (exactBasisSnapshot) ResolveReference(
	typedmemory.StrongRef,
	typedmemory.BoundedContextRef,
) typedmemory.StrongReferenceResolution {
	return nil
}

func (snapshot exactBasisSnapshot) EvaluateMemberOf(
	request typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	judgement, _ := snapshot.memberEngine.judgement(request, snapshot.inputs)
	return judgement
}

func (snapshot exactBasisSnapshot) AssertionState(
	typedmemory.AssertionID,
) typedmemory.AssertionState {
	return snapshot.assertionAbsent
}

func (exactBasisSnapshot) ResolveAlias(
	typedmemory.EntityAlias,
	typedmemory.BoundedContextRef,
) typedmemory.AliasAvailability {
	return nil
}

func (exactBasisSnapshot) ResolveReconciliationBasis(
	basis typedmemory.ReconciliationBasisRef,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.ReconciliationBasisResolution {
	resolution, _ := typedmemory.NewMissingReconciliationBasis(basis, contextRef)
	return resolution
}

type exactBasisMemberOfEngine struct {
	expectedProject projectledger.ProjectID
	kindSignature   typedmemory.KindSignatureDefinition
	entitySet       typedmemory.EntitySetDefinition
	provenance      typedmemory.MemberOfEvaluationProvenance
}

var errExactBasisProjectMismatch = errors.New("exact-basis MemberOf project mismatch")

func (engine exactBasisMemberOfEngine) EvaluateMemberOf(
	_ context.Context,
	input MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	if input.ProjectID() != engine.expectedProject {
		return nil, errExactBasisProjectMismatch
	}
	observables := make([]typedmemory.MemberOfObservableInput, 0, len(input.ObservableInputs()))
	for _, blob := range input.ObservableInputs() {
		observable, err := typedmemory.NewMemberOfObservableInput(blob.Reference(), blob.Digest())
		if err != nil {
			return nil, err
		}
		observables = append(observables, observable)
	}
	return engine.judgement(input.Request(), observables)
}

func (engine exactBasisMemberOfEngine) judgement(
	request typedmemory.MemberOfEvaluationRequest,
	observables []typedmemory.MemberOfObservableInput,
) (typedmemory.MemberOfJudgement, error) {
	basis, err := typedmemory.NewMemberOfBasis(typedmemory.MemberOfBasisInput{
		Query:                request.Query(),
		EvaluationView:       request.View(),
		KindSignature:        engine.kindSignature,
		EntitySet:            engine.entitySet,
		ObservableInputs:     observables,
		EvaluationProvenance: engine.provenance,
	})
	if err != nil {
		return nil, err
	}
	return typedmemory.NewMemberOfMember(request.Query(), basis)
}

type exactBasisObservableProvider struct {
	blobs map[string]ObservableInputBlob
}

func (provider exactBasisObservableProvider) LoadObservableInput(
	_ context.Context,
	_ projectledger.ProjectID,
	reference typedmemory.ObservableInputRef,
	digest typedmemory.SHA256Digest,
) (ObservableInputBlob, error) {
	blob, exists := provider.blobs[reference.String()]
	if !exists || blob.Digest() != digest {
		return ObservableInputBlob{}, fmt.Errorf("unexpected observable %s %s", reference.String(), digest.String())
	}
	return blob, nil
}

func exactBasisObservable(
	t *testing.T,
	referenceText string,
	content []byte,
) (typedmemory.MemberOfObservableInput, ObservableInputBlob) {
	t.Helper()
	reference, err := typedmemory.NewObservableInputRef(referenceText)
	if err != nil {
		t.Fatalf("NewObservableInputRef: %v", err)
	}
	digest := mustDigest(t, content)
	input, err := typedmemory.NewMemberOfObservableInput(reference, digest)
	if err != nil {
		t.Fatalf("NewMemberOfObservableInput: %v", err)
	}
	blob, err := NewObservableInputBlob(reference, digest, content)
	if err != nil {
		t.Fatalf("NewObservableInputBlob: %v", err)
	}
	return input, blob
}

func exactBasisRuleRef(t *testing.T, raw string) typedmemory.RuleRef {
	t.Helper()
	value, err := typedmemory.NewRuleRef(raw)
	if err != nil {
		t.Fatalf("NewRuleRef(%q): %v", raw, err)
	}
	return value
}

func exactBasisCarrierRef(t *testing.T, raw string) typedmemory.CarrierRef {
	t.Helper()
	value, err := typedmemory.NewCarrierRef(raw)
	if err != nil {
		t.Fatalf("NewCarrierRef(%q): %v", raw, err)
	}
	return value
}

func exactBasisCarrierEdition(t *testing.T, raw string) typedmemory.CarrierEdition {
	t.Helper()
	value, err := typedmemory.NewCarrierEdition(raw)
	if err != nil {
		t.Fatalf("NewCarrierEdition(%q): %v", raw, err)
	}
	return value
}

func exactBasisEntityLabel(t *testing.T, raw string) typedmemory.EntityLabel {
	t.Helper()
	value, err := typedmemory.NewEntityLabel(raw)
	if err != nil {
		t.Fatalf("NewEntityLabel(%q): %v", raw, err)
	}
	return value
}

var _ typedmemory.MemorySnapshot = exactBasisSnapshot{}
var _ MemberOfEvaluationEngine = exactBasisMemberOfEngine{}
var _ ObservableInputContentProvider = exactBasisObservableProvider{}
