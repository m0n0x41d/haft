package typedmemorystore

import (
	"context"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

func TestExactTargetMemberOfFactViewReevaluatesCurrentFactsWithTargetRuntime(
	t *testing.T,
) {
	fixture := newExactBasisStoreFixture(t)
	request := fixture.request(t, "target-memberof-fact-view")
	if _, err := fixture.adapter.CommitMemoryChangeSet(
		context.Background(),
		request,
	); err != nil {
		t.Fatalf("CommitMemoryChangeSet: %v", err)
	}

	recorder := &recordingExactTargetMemberOfEngine{
		delegate: fixture.adapter.memberOfEngine.(exactBasisMemberOfEngine),
		blobs:    fixture.observableBlobs,
	}
	runtime := exactTargetMemberOfRuntime(
		t,
		fixture.kindSignature.Evaluator(),
		recorder,
	)
	ctx := context.Background()
	transaction, err := sqlitetransaction.BeginRead(ctx, fixture.base.database)
	if err != nil {
		t.Fatalf("BeginRead: %v", err)
	}
	graph, err := LoadCurrentGraphRevalidationBasisTx(
		ctx,
		transaction,
		fixture.base.project,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("LoadCurrentGraphRevalidationBasisTx: %v", err)
	}
	facts, err := LoadExactTargetMemberOfFactViewTx(
		ctx,
		transaction,
		fixture.base.project,
		graph.GraphSnapshotBasis(),
		fixture.environment,
		runtime,
	)
	if err != nil {
		_ = transaction.Rollback(ctx)
		t.Fatalf("LoadExactTargetMemberOfFactViewTx: %v", err)
	}
	evaluationRequest := exactBasisCurrentSnapshotRequest(t, fixture)
	judgement := facts.EvaluateMemberOf(evaluationRequest)
	if _, ok := judgement.(typedmemory.MemberOfMember); !ok {
		_ = transaction.Rollback(ctx)
		t.Fatalf("EvaluateMemberOf = %T; want MemberOfMember", judgement)
	}
	if recorder.selectionCalls != 1 || recorder.evaluationCalls != 1 {
		_ = transaction.Rollback(ctx)
		t.Fatalf(
			"target evaluator calls = selection %d/evaluation %d; want 1/1",
			recorder.selectionCalls,
			recorder.evaluationCalls,
		)
	}
	if facts.TargetTypeEnv() != fixture.environment.Ref() ||
		facts.GraphSnapshotBasis().Ref() != graph.GraphSnapshotBasis().Ref() {
		_ = transaction.Rollback(ctx)
		t.Fatal("target MemberOf fact view lost its exact C or graph coordinate")
	}
	if result := transaction.Rollback(ctx); !result.Succeeded() {
		t.Fatalf("Rollback: %v", result.Err())
	}
}

type recordingExactTargetMemberOfEngine struct {
	delegate        exactBasisMemberOfEngine
	blobs           []ObservableInputBlob
	selectionCalls  int
	evaluationCalls int
}

func (engine *recordingExactTargetMemberOfEngine) SelectSnapshotObservableInputs(
	MemberOfEvaluationInput,
) SnapshotObservableInputSelection {
	engine.selectionCalls++
	selected, err := NewSnapshotObservableInputsSelected(engine.blobs)
	if err != nil {
		return NewSnapshotObservableInputsUnavailable()
	}
	return selected
}

func (engine *recordingExactTargetMemberOfEngine) EvaluateMemberOf(
	ctx context.Context,
	input MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	engine.evaluationCalls++
	return engine.delegate.EvaluateMemberOf(ctx, input)
}

func exactTargetMemberOfRuntime(
	t *testing.T,
	rule typedmemory.RuleRef,
	engine MemberOfEvaluationEngine,
) projecttypeenvruntime.ExactTargetRuntimeRegistry {
	t.Helper()
	memberEntry := mustExactTargetViewValue(
		runtimemechanism.NewMemberOfEntry(rule),
	)
	deliveryEntry := mustExactTargetViewValue(
		runtimemechanism.NewCarrierMembershipDeliveryEntry(rule),
	)
	catalog := mustExactTargetViewValue(
		runtimemechanism.SealRuntimeMechanismArtifactV1(
			exactBasisCarrierRef(t, "artifact:exact-target-memberof-runtime"),
			exactBasisCarrierEdition(t, "1.0.0"),
			[]runtimemechanism.RuntimeMechanismEntryV1{
				memberEntry,
				deliveryEntry,
			},
		),
	)
	mechanism := mustExactTargetViewValue(
		projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog),
	)
	evaluatorPin := mustExactTargetViewValue(
		projecttypeenv.NewEvaluatorRuntimeMechanismPin(
			projecttypeenv.EvaluatorRuntimeMechanismPinInput{
				Rule:             rule,
				Contract:         projecttypeenv.RuntimeMechanismContractMemberOf,
				Mechanism:        mechanism,
				ResolvedArtifact: &catalog,
			},
		),
	)
	deliveryPin := mustExactTargetViewValue(
		projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
			projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
				Rule:             rule,
				Mechanism:        mechanism,
				ResolvedArtifact: &catalog,
			},
		),
	)
	policy := exactTargetMemberOfRegistrationPolicy(t, catalog, rule)
	policyPin := mustExactTargetViewValue(
		projecttypeenv.NewRegistrationPolicyPin(policy),
	)
	basis := mustExactTargetViewValue(
		projecttypeenv.SealRuntimeEvaluationBasisWithPins(
			[]projecttypeenv.RuntimeEvaluationBasisPin{
				evaluatorPin,
				deliveryPin,
				policyPin,
			},
			[]runtimemechanism.RuntimeMechanismArtifactV1{catalog},
			nil,
		),
	)
	identity := catalog.Identity()
	mechanismIdentity := mustExactTargetViewValue(
		typedmemoryevaluation.NewMechanismIdentity(
			identity.Artifact(),
			identity.Edition(),
			identity.Digest(),
			typedmemoryevaluation.EvaluatorRole,
		),
	)
	registration := mustExactTargetViewValue(
		memberofruntime.NewRegistration(rule, mechanismIdentity, engine),
	)
	registry := mustExactTargetViewValue(
		memberofruntime.NewRegistry([]memberofruntime.Registration{registration}),
	)
	resolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:             typedmemory.NewCodecRegistry(),
				MemberOfEvaluators: registry,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					catalog,
				},
				RegistrationPolicies: []recordmembershipregistration.RegistrationArtifactV1{
					policy,
				},
			},
		},
	)
	matched, ok := resolution.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("target MemberOf runtime = %s; want matched", resolution.Kind())
	}
	exact, ok := matched.Registry()
	if !ok {
		t.Fatal("matched target MemberOf runtime omitted exact registry")
	}
	return exact
}

func exactTargetMemberOfRegistrationPolicy(
	t *testing.T,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
	rule typedmemory.RuleRef,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	identity := catalog.Identity()
	evaluator := mustExactTargetViewValue(
		recordmembershipregistration.NewMechanismCoordinate(
			recordmembershipregistration.MechanismCoordinateInput{
				Role:     recordmembershipregistration.EvaluatorMechanism,
				Rule:     rule,
				Artifact: identity.Artifact(),
				Edition:  identity.Edition(),
				Digest:   identity.Digest(),
			},
		),
	)
	delivery := mustExactTargetViewValue(
		recordmembershipregistration.NewMechanismCoordinate(
			recordmembershipregistration.MechanismCoordinateInput{
				Role:     recordmembershipregistration.SourceDeliveryBoundaryMechanism,
				Rule:     rule,
				Artifact: identity.Artifact(),
				Edition:  identity.Edition(),
				Digest:   identity.Digest(),
			},
		),
	)
	manifest := mustExactTargetViewValue(
		recordmapping.NewMappingManifestRef(
			"test.exact-target-memberof",
			"1.0.0",
			mustDigest(t, []byte("exact-target-memberof-mapping")),
		),
	)
	adapter := mustExactTargetViewValue(
		recordmapping.NewAdapterVersion("exact-target-memberof/1.0.0"),
	)
	mapping := mustExactTargetViewValue(
		recordmembershipregistration.NewAcceptedMapping(
			recordmembershipregistration.AcceptedMappingInput{
				Manifest: manifest,
				Adapter:  adapter,
			},
		),
	)
	return mustExactTargetViewValue(
		recordmembershipregistration.SealRegistrationArtifactV1(
			recordmembershipregistration.RegistrationArtifactInputV1{
				Evaluator:      evaluator,
				SourceDelivery: delivery,
				Mappings:       []recordmembershipregistration.AcceptedMapping{mapping},
			},
		),
	)
}

func mustExactTargetViewValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
