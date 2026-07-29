package projecttypeenvassertionrevalidation

import (
	"context"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/kindclassificationevaluation"
	"github.com/m0n0x41d/haft/internal/kindclassificationruntime"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

func TestRevalidateCurrentKindClassificationKeepsOutcomesDistinct(
	t *testing.T,
) {
	t.Parallel()
	fixture := newCurrentClassificationRevalidationFixture(t)
	tests := []struct {
		name        string
		judgement   typedmemory.KindClassificationJudgementKind
		posture     typedmemory.RevalidationPosture
		outcome     AssertionOutcomeKind
		groundCode  GroundCode
		groundKnown bool
	}{
		{
			name:      "true remains valid",
			judgement: typedmemory.KindClassificationTrue,
			posture:   typedmemory.RevalidationClean,
			outcome:   AssertionValid,
		},
		{
			name:        "false is a contradiction",
			judgement:   typedmemory.KindClassificationFalse,
			posture:     typedmemory.RevalidationConflict,
			outcome:     AssertionInvalid,
			groundCode:  CodeKindClassificationFalse,
			groundKnown: true,
		},
		{
			name:        "unknown is missing basis",
			judgement:   typedmemory.KindClassificationUnknown,
			posture:     typedmemory.RevalidationUnderdetermined,
			outcome:     AssertionUnderdetermined,
			groundCode:  CodeKindClassificationBasisMissing,
			groundKnown: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			facts := currentClassificationStaticFactView{
				target: fixture.target.Ref(),
				basis:  fixture.observation.GraphSnapshotBasis(),
				evaluate: func(
					request typedmemory.KindClassificationRequest,
				) typedmemory.KindClassificationJudgement {
					return fixture.judgement(t, request, test.judgement)
				},
			}
			report, err := Revalidate(Input{
				CurrentGraph:  fixture.observation,
				TargetTypeEnv: fixture.target,
				TargetRuntime: fixture.runtime,
				ExactTargetReferenceKindFacts: mustTestValue(
					projectgraphobservation.NewExactTargetKindClassificationReferenceKindFacts(
						facts,
					),
				),
			})
			if err != nil {
				t.Fatalf("Revalidate(%s): %v", test.name, err)
			}
			outcome := report.Outcomes()[0]
			if report.Posture() != test.posture || outcome.Kind() != test.outcome {
				t.Fatalf(
					"report = %s/%s; want %s/%s",
					report.Posture().String(),
					outcome.Kind().String(),
					test.posture.String(),
					test.outcome.String(),
				)
			}
			if test.groundKnown && !hasGroundCode(outcome, test.groundCode) {
				t.Fatalf("grounds = %#v; want %s", outcome.Grounds(), test.groundCode)
			}
		})
	}
}

func TestRevalidateCurrentKindClassificationReportsMissingEvaluatorSeparately(
	t *testing.T,
) {
	t.Parallel()
	fixture := newCurrentClassificationRevalidationFixture(t)
	runtime := exactRuntime(
		t,
		fixture.codec,
		passthroughCodec{},
		"artifact:test-current-classification-missing-evaluator",
	)
	facts := mustTestValue(projectgraphobservation.NewExactTargetNoReferenceKindFacts(
		fixture.target.Ref(),
		fixture.observation.GraphSnapshotBasis(),
	))
	report, err := Revalidate(Input{
		CurrentGraph:                  fixture.observation,
		TargetTypeEnv:                 fixture.target,
		TargetRuntime:                 runtime,
		ExactTargetReferenceKindFacts: facts,
	})
	if err != nil {
		t.Fatalf("Revalidate(missing evaluator): %v", err)
	}
	outcome := report.Outcomes()[0]
	if report.Posture() != typedmemory.RevalidationUnderdetermined ||
		outcome.Kind() != AssertionUnderdetermined ||
		!hasGroundCode(outcome, CodeKindClassificationEvaluatorMissing) ||
		hasGroundCode(outcome, CodeKindClassificationBasisMissing) ||
		hasGroundCode(outcome, CodeKindClassificationFalse) {
		t.Fatalf(
			"missing-evaluator report = %s/%s %#v",
			report.Posture().String(),
			outcome.Kind().String(),
			outcome.Grounds(),
		)
	}
}

type currentClassificationRevalidationFixture struct {
	target      typedmemory.TypeEnv
	runtime     projecttypeenvruntime.ExactTargetRuntimeRegistry
	observation projectgraphobservation.CurrentProjectGraphObservation
	signature   typedmemory.KindClassificationSignatureDefinition
	feature     typedmemory.VerifiedTypedValue
	codec       typedmemory.CodecRef
}

func newCurrentClassificationRevalidationFixture(
	t *testing.T,
) currentClassificationRevalidationFixture {
	t.Helper()
	ref := typeEnvRef(t, 0x91)
	provenance, coverage := typeEnvProvenance(t, 0x91)
	contextRef := mustTestValue(typedmemory.NewBoundedContextRef("ctx:test"))
	contextValue := mustTestValue(typedmemory.NewBoundedContext(contextRef, provenance))
	kindID := mustTestValue(typedmemory.NewKindID("U.Entity"))
	kind := mustTestValue(typedmemory.NewKindDefinition(kindID, provenance))
	valueKind := mustTestValue(typedmemory.NewValueKindRef(ref, kindID))
	availability := kindAvailability(t, ref, contextRef, kindID, provenance, 0x91)
	refKindID := mustTestValue(typedmemory.NewRefKindID("U.EntityRef"))
	refKind := mustTestValue(typedmemory.NewRefKindRef(ref, refKindID))
	refDefinition := mustTestValue(typedmemory.NewRefKindDefinition(
		refKind,
		valueKind,
		provenance,
	))
	shape := mustTestValue(typedmemory.NewScalarShape(typedmemory.ScalarBytes))
	shapeRef := mustTestValue(typedmemory.DeriveValueShapeRef(
		mustTestValue(typedmemory.NewShapeID("CurrentClassificationFeature")),
		shape,
	))
	shapeDeclaration := mustTestValue(typedmemory.NewValueShapeDeclaration(
		shapeRef,
		shape,
		provenance,
	))
	codec := codecRef(t, 0x91)
	binding := mustTestValue(typedmemory.NewValueBinding(
		valueKind,
		shapeRef,
		codec,
		provenance,
	))
	localKind := mustTestValue(typedmemory.NewLocalKindRef(valueKind, contextRef))
	criterion := mustTestValue(typedmemory.NewRuleRef(
		"test:kind-classification/entity/v1",
	))
	referenceScheme := mustTestValue(typedmemory.NewKindReferenceSchemePin(
		mustTestValue(typedmemory.NewCarrierRef("reference-scheme:test-current-revalidation")),
		mustTestValue(typedmemory.NewCarrierEdition("1.0.0")),
		digest(t, 0x92),
	))
	signature := mustTestValue(typedmemory.NewKindClassificationSignatureDefinition(
		typedmemory.KindClassificationSignatureDefinitionInput{
			LocalKind:          localKind,
			CandidateValueKind: valueKind,
			Criterion:          criterion,
			SliceConditions: mustTestValue(typedmemory.NewRuleRef(
				"test:kind-classification/slice/v1",
			)),
			ReferenceScheme: referenceScheme,
			Formality:       typedmemory.SignatureF4,
			ExtentRule:      typedmemory.NoKindExtentRule{},
			Provenance:      provenance,
		},
	))
	slotID := mustTestValue(typedmemory.NewSlotKindID("EntityOfConcernSlot"))
	slotTarget := mustTestValue(typedmemory.NewReferenceSlotTarget(valueKind, refKind))
	slot := mustTestValue(typedmemory.NewSlotSpec(
		slotID,
		slotTarget,
		typedmemory.ExactlyOneCardinality(),
		provenance,
	))
	relationRef := mustTestValue(typedmemory.NewRelationSignatureRef(
		ref,
		mustTestValue(typedmemory.NewSignatureID("Haft.CurrentAtConcern")),
	))
	relationSignature := mustTestValue(typedmemory.NewRelationSignature(
		relationRef,
		[]typedmemory.BoundedContextRef{contextRef},
		[]typedmemory.SlotSpec{slot},
		provenance,
	))
	target := mustTestValue(
		typedmemory.NewTypeEnvBuilder(ref).
			SetSourceRevision(mustTestValue(typedmemory.NewSourceRevision(
				"current-classification-revalidation",
			))).
			SetCompilerSchemaVersion(mustTestValue(
				typedmemory.NewCompilerSchemaVersion("test.v1"),
			)).
			SetCoverageManifest(coverage).
			AddBoundedContext(contextValue).
			AddKindDefinition(kind).
			AddContextKindAvailability(availability).
			AddRefKindDefinition(refDefinition).
			AddValueShape(shapeDeclaration).
			AddValueBinding(binding).
			AddKindClassificationSignatureDefinition(signature).
			AddRelationSignature(relationSignature).
			Build(),
	)
	entity := mustTestValue(typedmemory.NewEntityID(
		"entity:current-classification-revalidation",
	))
	persisted := mustTestValue(typedmemory.NewPersistedRef(
		refKind,
		mustTestValue(typedmemory.NewReferenceID(entity.String())),
	))
	filler := typedMemoryEnvelope(
		"validated-by-reference.v2",
		[]byte(refKind.String()),
		[]byte(persisted.ReferenceKey()),
		[]byte(entity.String()),
	)
	slotBinding := typedMemoryEnvelope(
		"validated-slot-binding.v1",
		[]byte(slotID.String()),
		filler,
	)
	slice := contextSlice(t, contextRef)
	relation := mustTestValue(typedmemory.DecodeCanonicalRelationInstance(
		typedMemoryEnvelope(
			"validated-relation-instance.v2",
			[]byte("assertion:current-classification-revalidation"),
			[]byte(relationRef.String()),
			[]byte(slice.Ref().String()),
			slice.CanonicalBytes(),
			slotBinding,
			[]byte("memory:test:current-classification-revalidation"),
		),
	))
	assertion := relationalAssertionFromRelation(
		t,
		relation,
		typedmemory.AssertionModalityAffirmsObtaining,
	)
	observation := committedRelationalAssertionObservation(
		t,
		ref,
		"qnt_1234abcd",
		assertion,
	)
	registry := mustTestValue(
		typedmemory.NewCodecRegistry().Register(codec, passthroughCodec{}),
	)
	featureCandidate := mustTestValue(typedmemory.NewTypedValueCandidate(
		valueKind,
		shapeRef,
		codec,
		[]byte("classified"),
		typedmemory.NoAssertedDigest{},
	))
	verification := typedmemory.VerifyTypedValue(registry, binding, featureCandidate)
	validFeature, valid := verification.(typedmemory.ValidTypedValue)
	if !valid {
		t.Fatalf("VerifyTypedValue(feature) = %T; want ValidTypedValue", verification)
	}
	return currentClassificationRevalidationFixture{
		target:      target,
		runtime:     exactCurrentClassificationRuntime(t, codec, criterion),
		observation: observation,
		signature:   signature,
		feature:     validFeature.Value(),
		codec:       codec,
	}
}

func (fixture currentClassificationRevalidationFixture) judgement(
	t *testing.T,
	request typedmemory.KindClassificationRequest,
	kind typedmemory.KindClassificationJudgementKind,
) typedmemory.KindClassificationJudgement {
	t.Helper()
	if kind == typedmemory.KindClassificationUnknown {
		reason := mustTestValue(typedmemory.NewKindClassificationUnknownReason(
			typedmemory.KindUnknownMissingGovernedFeature,
			mustTestValue(typedmemory.NewRepairPointer(
				"repair:kind-classification/load-current-feature",
			)),
		))
		return mustTestValue(typedmemory.NewUnknownKindClassification(
			request,
			[]typedmemory.KindClassificationUnknownReason{reason},
		))
	}
	feature := mustTestValue(typedmemory.NewGovernedCandidateFeature(
		typedmemory.GovernedCandidateFeatureInput{
			Key: mustTestValue(typedmemory.NewKindFeatureKey(
				"entity.current-classification",
			)),
			Value:        fixture.feature,
			Governor:     fixture.signature.Criterion(),
			Source:       mustTestValue(typedmemory.NewCarrierRef("record:test/current")),
			SourceDigest: digest(t, 0x93),
		},
	))
	features := mustTestValue(typedmemory.NewGovernedCandidateFeatureSet(
		request,
		[]typedmemory.GovernedCandidateFeature{feature},
	))
	basis := mustTestValue(typedmemory.NewKindClassificationEvaluationBasis(
		request,
		fixture.signature,
		features,
	))
	if kind == typedmemory.KindClassificationFalse {
		return mustTestValue(typedmemory.NewFalseKindClassification(request, basis))
	}
	return mustTestValue(typedmemory.NewTrueKindClassification(request, basis))
}

type currentClassificationStaticFactView struct {
	target   typedmemory.TypeEnvRef
	basis    projecttypeenvselection.ProjectGraphSnapshotBasis
	evaluate func(typedmemory.KindClassificationRequest) typedmemory.KindClassificationJudgement
}

func (view currentClassificationStaticFactView) TargetTypeEnv() typedmemory.TypeEnvRef {
	return view.target
}

func (view currentClassificationStaticFactView) GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis {
	return view.basis
}

func (view currentClassificationStaticFactView) EvaluateKindClassification(
	request typedmemory.KindClassificationRequest,
) typedmemory.KindClassificationJudgement {
	return view.evaluate(request)
}

type inertCurrentClassificationEngine struct{}

func (inertCurrentClassificationEngine) EvaluateKindClassification(
	context.Context,
	kindclassificationevaluation.EvaluationInput,
) (typedmemory.KindClassificationJudgement, error) {
	return nil, fmt.Errorf("inert current-classification evaluator")
}

func exactCurrentClassificationRuntime(
	t *testing.T,
	codec typedmemory.CodecRef,
	rule typedmemory.RuleRef,
) projecttypeenvruntime.ExactTargetRuntimeRegistry {
	t.Helper()
	codecEntry := mustTestValue(runtimemechanism.NewCodecCanonicalizationEntry(codec))
	classificationEntry := mustTestValue(runtimemechanism.NewKindClassificationEntry(rule))
	catalog := mustTestValue(runtimemechanism.SealRuntimeMechanismArtifactV1(
		mustTestValue(typedmemory.NewCarrierRef(
			"artifact:test-current-classification-runtime",
		)),
		mustTestValue(typedmemory.NewCarrierEdition("1.0.0")),
		[]runtimemechanism.RuntimeMechanismEntryV1{
			codecEntry,
			classificationEntry,
		},
	))
	mechanism := mustTestValue(
		projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog),
	)
	codecPin := mustTestValue(projecttypeenv.NewCodecRuntimeMechanismPin(
		projecttypeenv.CodecRuntimeMechanismPinInput{
			Codec:            codec,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	))
	classificationPin := mustTestValue(projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         projecttypeenv.RuntimeMechanismContractKindClassification,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	))
	basis := mustTestValue(projecttypeenv.SealRuntimeEvaluationBasisWithPins(
		[]projecttypeenv.RuntimeEvaluationBasisPin{codecPin, classificationPin},
		[]runtimemechanism.RuntimeMechanismArtifactV1{catalog},
		nil,
	))
	identity := catalog.Identity()
	mechanismIdentity := mustTestValue(typedmemoryevaluation.NewMechanismIdentity(
		identity.Artifact(),
		identity.Edition(),
		identity.Digest(),
		typedmemoryevaluation.EvaluatorRole,
	))
	registration := mustTestValue(kindclassificationruntime.NewRegistration(
		rule,
		mechanismIdentity,
		inertCurrentClassificationEngine{},
	))
	classification := mustTestValue(kindclassificationruntime.NewRegistry(
		[]kindclassificationruntime.Registration{registration},
	))
	codecs := mustTestValue(
		typedmemory.NewCodecRegistry().Register(codec, passthroughCodec{}),
	)
	memberOf := mustTestValue(memberofruntime.NewRegistry(nil))
	resolution := projecttypeenvruntime.ObserveCurrentTargetRuntime(
		projecttypeenvruntime.ObservationInput{
			RuntimeBasis: basis,
			Installed: projecttypeenvruntime.InstalledRuntimeRegistryInput{
				Codecs:                       codecs,
				MemberOfEvaluators:           memberOf,
				KindClassificationEvaluators: classification,
				MechanismCatalogs: []runtimemechanism.RuntimeMechanismArtifactV1{
					catalog,
				},
			},
		},
	)
	matched, ok := resolution.(projecttypeenvruntime.Matched)
	if !ok {
		t.Fatalf("current-classification runtime = %s; want matched", resolution.Kind())
	}
	runtime, ok := matched.Registry()
	if !ok {
		t.Fatal("matched current-classification runtime omitted exact registry")
	}
	return runtime
}
