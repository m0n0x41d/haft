package kindclassificationruntime_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/kindclassificationevaluation"
	"github.com/m0n0x41d/haft/internal/kindclassificationruntime"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

func TestDirectFeatureCriterionPreservesTrueFalseAndUnknown(t *testing.T) {
	fixture := newClassificationRuntimeFixture(t)
	criterion := mustClassificationRuntime(
		kindclassificationruntime.NewDirectFeatureCriterion(
			fixture.rule,
			[]kindclassificationruntime.DirectFeaturePredicate{fixture.predicate},
		),
	)
	available := mustClassificationRuntime(
		kindclassificationevaluation.NewGovernedFeaturesAvailable(
			fixture.request,
			fixture.features,
		),
	)
	input := mustClassificationRuntime(
		kindclassificationevaluation.NewEvaluationInput(
			fixture.request,
			fixture.signature,
			available,
			[]typedmemory.KindSignatureDependencyPin{fixture.dependency},
		),
	)

	trueJudgement := mustClassificationRuntime(
		criterion.EvaluateKindClassification(context.Background(), input),
	)
	if trueJudgement.Kind() != typedmemory.KindClassificationTrue {
		t.Fatalf("exact feature evaluation = %q, want true", trueJudgement.Kind())
	}

	falsePredicate := mustClassificationRuntime(
		kindclassificationruntime.NewDirectFeaturePredicate(
			kindclassificationruntime.DirectFeaturePredicateInput{
				Key:                 fixture.feature.Key(),
				Governor:            fixture.feature.Governor(),
				ExpectedValueKind:   fixture.feature.Value().ValueKind(),
				ExpectedValueDigest: classificationRuntimeDigest(t, 'f'),
			},
		),
	)
	falseCriterion := mustClassificationRuntime(
		kindclassificationruntime.NewDirectFeatureCriterion(
			fixture.rule,
			[]kindclassificationruntime.DirectFeaturePredicate{falsePredicate},
		),
	)
	falseJudgement := mustClassificationRuntime(
		falseCriterion.EvaluateKindClassification(context.Background(), input),
	)
	if falseJudgement.Kind() != typedmemory.KindClassificationFalse {
		t.Fatalf("contradicting exact feature evaluation = %q, want false", falseJudgement.Kind())
	}

	missingInput := mustClassificationRuntime(
		kindclassificationevaluation.NewEvaluationInput(
			fixture.request,
			fixture.signature,
			available,
			nil,
		),
	)
	unknown := mustClassificationRuntime(
		criterion.EvaluateKindClassification(context.Background(), missingInput),
	)
	assertClassificationUnknownReason(
		t,
		unknown,
		typedmemory.KindUnknownDependencyUnavailable,
	)
	if falseJudgement.Digest() == unknown.Digest() {
		t.Fatal("false and unknown collapsed to one classification value")
	}
}

func TestDirectFeatureCriterionTreatsSourceFailuresAsUnknown(t *testing.T) {
	fixture := newClassificationRuntimeFixture(t)
	criterion := mustClassificationRuntime(
		kindclassificationruntime.NewDirectFeatureCriterion(
			fixture.rule,
			[]kindclassificationruntime.DirectFeaturePredicate{fixture.predicate},
		),
	)
	reason := mustClassificationRuntime(
		typedmemory.NewKindClassificationUnknownReason(
			typedmemory.KindUnknownFeatureSourceUntrusted,
			mustClassificationRuntime(
				typedmemory.NewRepairPointer("repair:feature-source/trust"),
			),
		),
	)
	unavailable := mustClassificationRuntime(
		kindclassificationevaluation.NewGovernedFeaturesUnavailable(
			fixture.request,
			[]typedmemory.KindClassificationUnknownReason{reason},
		),
	)
	input := mustClassificationRuntime(
		kindclassificationevaluation.NewEvaluationInput(
			fixture.request,
			fixture.signature,
			unavailable,
			[]typedmemory.KindSignatureDependencyPin{fixture.dependency},
		),
	)
	judgement := mustClassificationRuntime(
		criterion.EvaluateKindClassification(context.Background(), input),
	)
	assertClassificationUnknownReason(
		t,
		judgement,
		typedmemory.KindUnknownFeatureSourceUntrusted,
	)
}

func TestClassificationRegistryIsDeterministicAndFailClosed(t *testing.T) {
	fixture := newClassificationRuntimeFixture(t)
	criterion := mustClassificationRuntime(
		kindclassificationruntime.NewDirectFeatureCriterion(
			fixture.rule,
			[]kindclassificationruntime.DirectFeaturePredicate{fixture.predicate},
		),
	)
	registration := mustClassificationRuntime(
		kindclassificationruntime.NewRegistration(
			fixture.rule,
			fixture.identity,
			criterion,
		),
	)
	registry := mustClassificationRuntime(
		kindclassificationruntime.NewRegistry(
			[]kindclassificationruntime.Registration{registration},
		),
	)
	if registry.Len() != 1 || registry.Registrations()[0].RuleRef() != fixture.rule {
		t.Fatal("registry lost its exact RuleRef registration")
	}
	_, err := kindclassificationruntime.NewRegistry(
		[]kindclassificationruntime.Registration{registration, registration},
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate RuleRef") {
		t.Fatalf("duplicate registry error = %v", err)
	}

	available := mustClassificationRuntime(
		kindclassificationevaluation.NewGovernedFeaturesAvailable(
			fixture.request,
			fixture.features,
		),
	)
	input := mustClassificationRuntime(
		kindclassificationevaluation.NewEvaluationInput(
			fixture.request,
			fixture.signature,
			available,
			[]typedmemory.KindSignatureDependencyPin{fixture.dependency},
		),
	)
	judgement := mustClassificationRuntime(
		registry.EvaluateKindClassification(context.Background(), input),
	)
	if judgement.Kind() != typedmemory.KindClassificationTrue {
		t.Fatalf("registered evaluation = %q, want true", judgement.Kind())
	}

	empty := mustClassificationRuntime(kindclassificationruntime.NewRegistry(nil))
	missing := mustClassificationRuntime(
		empty.EvaluateKindClassification(context.Background(), input),
	)
	assertClassificationUnknownReason(
		t,
		missing,
		typedmemory.KindUnknownCriterionUnavailable,
	)
}

func TestClassificationRegistryRejectsUncorrelatedAndCancelledEvaluation(t *testing.T) {
	fixture := newClassificationRuntimeFixture(t)
	other := fixture.requestWithEntity(t, "entity:other")
	uncorrelated := uncorrelatedClassificationEngine{
		judgement: mustClassificationRuntime(
			typedmemory.NewUnknownKindClassification(
				other,
				[]typedmemory.KindClassificationUnknownReason{
					mustClassificationRuntime(
						typedmemory.NewKindClassificationUnknownReason(
							typedmemory.KindUnknownCriterionUnavailable,
							mustClassificationRuntime(
								typedmemory.NewRepairPointer("repair:other"),
							),
						),
					),
				},
			),
		),
	}
	registration := mustClassificationRuntime(
		kindclassificationruntime.NewRegistration(
			fixture.rule,
			fixture.identity,
			uncorrelated,
		),
	)
	registry := mustClassificationRuntime(
		kindclassificationruntime.NewRegistry(
			[]kindclassificationruntime.Registration{registration},
		),
	)
	input := fixture.input(t)
	_, err := registry.EvaluateKindClassification(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "uncorrelated judgement") {
		t.Fatalf("uncorrelated evaluator error = %v", err)
	}

	criterion := mustClassificationRuntime(
		kindclassificationruntime.NewDirectFeatureCriterion(
			fixture.rule,
			[]kindclassificationruntime.DirectFeaturePredicate{fixture.predicate},
		),
	)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = criterion.EvaluateKindClassification(cancelled, input)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("cancelled evaluation error = %v", err)
	}
}

type classificationRuntimeFixture struct {
	typeEnv    typedmemory.TypeEnvRef
	context    typedmemory.BoundedContextRef
	localKind  typedmemory.LocalKindRef
	signature  typedmemory.KindClassificationSignatureDefinition
	request    typedmemory.KindClassificationRequest
	dependency typedmemory.KindSignatureDependencyPin
	feature    typedmemory.GovernedCandidateFeature
	features   typedmemory.GovernedCandidateFeatureSet
	rule       typedmemory.RuleRef
	predicate  kindclassificationruntime.DirectFeaturePredicate
	identity   typedmemoryevaluation.MechanismIdentity
}

func newClassificationRuntimeFixture(t *testing.T) classificationRuntimeFixture {
	t.Helper()
	typeEnv := mustClassificationRuntime(
		typedmemory.NewTypeEnvRef(classificationRuntimeDigest(t, '1')),
	)
	contextRef := mustClassificationRuntime(
		typedmemory.NewBoundedContextRef("ctx:classification-runtime"),
	)
	entityKind := mustClassificationRuntime(typedmemory.NewKindID("U.Entity"))
	localKindID := mustClassificationRuntime(typedmemory.NewKindID("Haft.ProjectRecord"))
	featureKindID := mustClassificationRuntime(typedmemory.NewKindID("Haft.FeatureStatus"))
	entityValueKind := mustClassificationRuntime(
		typedmemory.NewValueKindRef(typeEnv, entityKind),
	)
	localValueKind := mustClassificationRuntime(
		typedmemory.NewValueKindRef(typeEnv, localKindID),
	)
	featureValueKind := mustClassificationRuntime(
		typedmemory.NewValueKindRef(typeEnv, featureKindID),
	)
	localKind := mustClassificationRuntime(
		typedmemory.NewLocalKindRef(localValueKind, contextRef),
	)
	referenceScheme := mustClassificationRuntime(
		typedmemory.NewKindReferenceSchemePin(
			mustClassificationRuntime(typedmemory.NewCarrierRef("reference-scheme:project-memory")),
			mustClassificationRuntime(typedmemory.NewCarrierEdition("1.0.0")),
			classificationRuntimeDigest(t, '2'),
		),
	)
	dependency := mustClassificationRuntime(
		typedmemory.NewKindSignatureDependencyPin(
			typedmemory.KindDependencyStandard,
			mustClassificationRuntime(typedmemory.NewCarrierRef("standard:project-record")),
			mustClassificationRuntime(typedmemory.NewCarrierEdition("1.0.0")),
			classificationRuntimeDigest(t, '3'),
		),
	)
	rule := mustClassificationRuntime(
		typedmemory.NewRuleRef("rule:kind-classification/project-record/v1"),
	)
	signature := mustClassificationRuntime(
		typedmemory.NewKindClassificationSignatureDefinition(
			typedmemory.KindClassificationSignatureDefinitionInput{
				LocalKind:          localKind,
				CandidateValueKind: entityValueKind,
				Criterion:          rule,
				SliceConditions: mustClassificationRuntime(
					typedmemory.NewRuleRef("rule:context-slice/project/v1"),
				),
				ReferenceScheme: referenceScheme,
				Dependencies:    []typedmemory.KindSignatureDependencyPin{dependency},
				Formality:       typedmemory.SignatureF4,
				ExtentRule:      typedmemory.NoKindExtentRule{},
				Provenance:      classificationRuntimeProvenance(t),
			},
		),
	)
	request := mustClassificationRuntime(
		typedmemory.NewKindClassificationRequest(
			typedmemory.KindClassificationRequestInput{
				Candidate: mustClassificationRuntime(
					typedmemory.NewExactKindEntityCandidate(
						mustClassificationRuntime(typedmemory.NewEntityID("entity:record-1")),
						entityValueKind,
					),
				),
				LocalKind:        localKind,
				SignatureEdition: signature.Ref(),
				ContextSlice:     classificationRuntimeContextSlice(t, contextRef),
			},
		),
	)
	featureValue := classificationRuntimeVerifiedText(t, featureValueKind, "trusted")
	feature := mustClassificationRuntime(
		typedmemory.NewGovernedCandidateFeature(
			typedmemory.GovernedCandidateFeatureInput{
				Key: mustClassificationRuntime(
					typedmemory.NewKindFeatureKey("record.delivery-status"),
				),
				Value: featureValue,
				Governor: mustClassificationRuntime(
					typedmemory.NewRuleRef("rule:record-delivery/status/v1"),
				),
				Source: mustClassificationRuntime(
					typedmemory.NewCarrierRef("record:project/record-1"),
				),
				SourceDigest: classificationRuntimeDigest(t, '4'),
			},
		),
	)
	features := mustClassificationRuntime(
		typedmemory.NewGovernedCandidateFeatureSet(
			request,
			[]typedmemory.GovernedCandidateFeature{feature},
		),
	)
	predicate := mustClassificationRuntime(
		kindclassificationruntime.NewDirectFeaturePredicate(
			kindclassificationruntime.DirectFeaturePredicateInput{
				Key:                 feature.Key(),
				Governor:            feature.Governor(),
				ExpectedValueKind:   feature.Value().ValueKind(),
				ExpectedValueDigest: feature.Value().Digest(),
			},
		),
	)
	identity := mustClassificationRuntime(
		typedmemoryevaluation.NewMechanismIdentity(
			mustClassificationRuntime(typedmemory.NewCarrierRef("runtime:kind-classifier/project-record")),
			mustClassificationRuntime(typedmemory.NewCarrierEdition("1.0.0")),
			classificationRuntimeDigest(t, '5'),
			typedmemoryevaluation.EvaluatorRole,
		),
	)
	return classificationRuntimeFixture{
		typeEnv:    typeEnv,
		context:    contextRef,
		localKind:  localKind,
		signature:  signature,
		request:    request,
		dependency: dependency,
		feature:    feature,
		features:   features,
		rule:       rule,
		predicate:  predicate,
		identity:   identity,
	}
}

func (fixture classificationRuntimeFixture) input(
	t *testing.T,
) kindclassificationevaluation.EvaluationInput {
	t.Helper()
	available := mustClassificationRuntime(
		kindclassificationevaluation.NewGovernedFeaturesAvailable(
			fixture.request,
			fixture.features,
		),
	)
	return mustClassificationRuntime(
		kindclassificationevaluation.NewEvaluationInput(
			fixture.request,
			fixture.signature,
			available,
			[]typedmemory.KindSignatureDependencyPin{fixture.dependency},
		),
	)
}

func (fixture classificationRuntimeFixture) requestWithEntity(
	t *testing.T,
	raw string,
) typedmemory.KindClassificationRequest {
	t.Helper()
	candidate := mustClassificationRuntime(
		typedmemory.NewExactKindEntityCandidate(
			mustClassificationRuntime(typedmemory.NewEntityID(raw)),
			fixture.request.Candidate().ValueKind(),
		),
	)
	return mustClassificationRuntime(
		typedmemory.NewKindClassificationRequest(
			typedmemory.KindClassificationRequestInput{
				Candidate:        candidate,
				LocalKind:        fixture.localKind,
				SignatureEdition: fixture.signature.Ref(),
				ContextSlice:     fixture.request.ContextSlice(),
			},
		),
	)
}

type uncorrelatedClassificationEngine struct {
	judgement typedmemory.KindClassificationJudgement
}

func (engine uncorrelatedClassificationEngine) EvaluateKindClassification(
	context.Context,
	kindclassificationevaluation.EvaluationInput,
) (typedmemory.KindClassificationJudgement, error) {
	return engine.judgement, nil
}

type classificationRuntimeTextCodec struct{}

func (classificationRuntimeTextCodec) Canonicalize(
	_ typedmemory.ValueShapeRef,
	input []byte,
) typedmemory.CodecCanonicalization {
	return mustClassificationRuntime(
		typedmemory.NewCanonicalizedCodecValue(
			typedmemory.NewTextValue(string(input)),
			append([]byte(nil), input...),
		),
	)
}

func classificationRuntimeVerifiedText(
	t *testing.T,
	valueKind typedmemory.ValueKindRef,
	text string,
) typedmemory.VerifiedTypedValue {
	t.Helper()
	shape := mustClassificationRuntime(typedmemory.NewScalarShape(typedmemory.ScalarText))
	shapeRef := mustClassificationRuntime(
		typedmemory.DeriveValueShapeRef(
			mustClassificationRuntime(typedmemory.NewShapeID("Haft.Shape.FeatureStatusV1")),
			shape,
		),
	)
	codecRef := mustClassificationRuntime(
		typedmemory.NewCodecRef(
			mustClassificationRuntime(typedmemory.NewCodecID("Haft.Codec.FeatureStatusV1")),
			mustClassificationRuntime(typedmemory.NewCanonicalizationVersion("1")),
			classificationRuntimeDigest(t, '6'),
		),
	)
	binding := mustClassificationRuntime(
		typedmemory.NewValueBinding(
			valueKind,
			shapeRef,
			codecRef,
			classificationRuntimeProvenance(t),
		),
	)
	registry := typedmemory.NewCodecRegistry()
	registry = mustClassificationRuntime(
		registry.Register(codecRef, classificationRuntimeTextCodec{}),
	)
	candidate := mustClassificationRuntime(
		typedmemory.NewTypedValueCandidate(
			valueKind,
			shapeRef,
			codecRef,
			[]byte(text),
			typedmemory.NoAssertedDigest{},
		),
	)
	result := typedmemory.VerifyTypedValue(registry, binding, candidate)
	valid, ok := result.(typedmemory.ValidTypedValue)
	if !ok {
		t.Fatalf("feature typed value verification = %T", result)
	}
	return valid.Value()
}

func classificationRuntimeProvenance(t *testing.T) typedmemory.DeclarationProvenance {
	t.Helper()
	location := mustClassificationRuntime(
		typedmemory.NewUnpatternedSourceLocation(
			mustClassificationRuntime(typedmemory.NewSourceUnitID("fixture:kind-classification")),
			mustClassificationRuntime(typedmemory.NewSourceRevision(strings.Repeat("a", 40))),
			classificationRuntimeDigest(t, '7'),
			mustClassificationRuntime(typedmemory.NewSourceLineRange(1, 1)),
		),
	)
	return mustClassificationRuntime(
		typedmemory.NewFPFSourceProvenance(
			mustClassificationRuntime(typedmemory.NewProvenanceRef("prov:fixture:kind-classification")),
			location,
			mustClassificationRuntime(typedmemory.NewCompilerRuleID("fixture.kind-classification.v1")),
		),
	)
}

func classificationRuntimeContextSlice(
	t *testing.T,
	contextRef typedmemory.BoundedContextRef,
) typedmemory.ContextSlice {
	t.Helper()
	point := mustClassificationRuntime(
		typedmemory.NewGammaPoint(time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)),
	)
	return mustClassificationRuntime(
		typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
			Context:   contextRef,
			GammaTime: point,
		}),
	)
}

func classificationRuntimeDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	if t != nil {
		t.Helper()
	}
	return mustClassificationRuntime(
		typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(string(fill), 64)),
	)
}

func assertClassificationUnknownReason(
	t *testing.T,
	judgement typedmemory.KindClassificationJudgement,
	want typedmemory.KindClassificationUnknownReasonKind,
) {
	t.Helper()
	unknown, ok := judgement.(typedmemory.UnknownKindClassification)
	if !ok {
		t.Fatalf("classification = %T, want UnknownKindClassification", judgement)
	}
	reasons := unknown.Reasons()
	if len(reasons) != 1 || reasons[0].Kind() != want {
		t.Fatalf("unknown reasons = %#v, want %q", reasons, want.String())
	}
}

func mustClassificationRuntime[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
