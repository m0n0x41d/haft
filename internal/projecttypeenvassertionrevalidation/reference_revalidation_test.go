package projecttypeenvassertionrevalidation

import (
	"context"
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/memberofevaluation"
	"github.com/m0n0x41d/haft/internal/memberofruntime"
	"github.com/m0n0x41d/haft/internal/projectgraphobservation"
	"github.com/m0n0x41d/haft/internal/projecttypeenvruntime"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

func TestRevalidateReferenceUsesExactTargetMemberOfFactsForLegacyAndV3(
	t *testing.T,
) {
	t.Parallel()
	fixture := newReferenceRevalidationFixture(t)
	legacy := fixture.observation
	v3 := committedRelationalAssertionObservation(
		t,
		fixture.target.Ref(),
		"qnt_1234abcd",
		relationalAssertionFromRelation(
			t,
			fixture.relation,
			typedmemory.AssertionModalityAffirmsObtaining,
		),
	)
	for name, observation := range map[string]projectgraphobservation.CurrentProjectGraphObservation{
		"legacy": legacy,
		"v3":     v3,
	} {
		observation := observation
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			facts := fixture.factView(
				t,
				observation.GraphSnapshotBasis(),
				memberJudgement,
			)
			report, err := Revalidate(Input{
				CurrentGraph:  observation,
				TargetTypeEnv: fixture.target,
				TargetRuntime: fixture.runtime,
				ExactTargetReferenceKindFacts: mustTestValue(
					projectgraphobservation.NewExactTargetMemberOfReferenceKindFacts(facts),
				),
			})
			if err != nil {
				t.Fatalf("Revalidate(%s): %v", name, err)
			}
			if report.Posture() != typedmemory.RevalidationClean ||
				report.Outcomes()[0].Kind() != AssertionValid {
				t.Fatalf(
					"%s reference report = %s/%s; want clean/valid",
					name,
					report.Posture().String(),
					report.Outcomes()[0].Kind().String(),
				)
			}
		})
	}
}

func TestRevalidateReferenceKeepsNotMemberAndUndefinedDistinct(t *testing.T) {
	t.Parallel()
	fixture := newReferenceRevalidationFixture(t)
	for name, judgement := range map[string]referenceJudgementKind{
		"not_member": notMemberJudgement,
		"undefined":  undefinedJudgement,
	} {
		judgement := judgement
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			facts := fixture.factView(
				t,
				fixture.observation.GraphSnapshotBasis(),
				judgement,
			)
			report, err := Revalidate(Input{
				CurrentGraph:  fixture.observation,
				TargetTypeEnv: fixture.target,
				TargetRuntime: fixture.runtime,
				ExactTargetReferenceKindFacts: mustTestValue(
					projectgraphobservation.NewExactTargetMemberOfReferenceKindFacts(facts),
				),
			})
			if err != nil {
				t.Fatalf("Revalidate(%s): %v", name, err)
			}
			outcome := report.Outcomes()[0]
			if judgement == notMemberJudgement {
				if report.Posture() != typedmemory.RevalidationConflict ||
					outcome.Kind() != AssertionInvalid ||
					!hasGroundCode(outcome, CodeMemberOfNotMember) {
					t.Fatalf("NotMember report = %s/%s %#v", report.Posture(), outcome.Kind(), outcome.Grounds())
				}
				return
			}
			if report.Posture() != typedmemory.RevalidationUnderdetermined ||
				outcome.Kind() != AssertionUnderdetermined ||
				!hasGroundCode(outcome, CodeMemberOfObservableMissing) {
				t.Fatalf("Undefined report = %s/%s %#v", report.Posture(), outcome.Kind(), outcome.Grounds())
			}
		})
	}
}

func TestRevalidateReferenceRejectsMismatchedFactViewCoordinates(t *testing.T) {
	t.Parallel()
	fixture := newReferenceRevalidationFixture(t)
	facts := fixture.factView(
		t,
		fixture.observation.GraphSnapshotBasis(),
		memberJudgement,
	)
	facts.target = typeEnvRef(t, 0xee)
	_, err := Revalidate(Input{
		CurrentGraph:  fixture.observation,
		TargetTypeEnv: fixture.target,
		TargetRuntime: fixture.runtime,
		ExactTargetReferenceKindFacts: mustTestValue(
			projectgraphobservation.NewExactTargetMemberOfReferenceKindFacts(facts),
		),
	})
	if err == nil {
		t.Fatal("Revalidate accepted a fact view for another target C")
	}
}

func TestRevalidateReferenceRejectsMalformedOrMismatchedJudgement(t *testing.T) {
	t.Parallel()
	fixture := newReferenceRevalidationFixture(t)
	for name, evaluate := range map[string]func(
		typedmemory.MemberOfEvaluationRequest,
	) typedmemory.MemberOfJudgement{
		"malformed": func(typedmemory.MemberOfEvaluationRequest) typedmemory.MemberOfJudgement {
			return nil
		},
		"mismatched": func(request typedmemory.MemberOfEvaluationRequest) typedmemory.MemberOfJudgement {
			otherQuery := mustTestValue(typedmemory.NewMemberOfQuery(
				mustTestValue(typedmemory.NewEntityID("entity:other-reference")),
				request.Query().ValueKind(),
				request.Query().ContextSlice(),
			))
			otherRequest := mustTestValue(typedmemory.NewMemberOfEvaluationRequest(
				otherQuery,
				request.View(),
			))
			basis := mustTestValue(typedmemory.NewMemberOfBasis(
				typedmemory.MemberOfBasisInput{
					Query:                otherQuery,
					EvaluationView:       otherRequest.View(),
					KindSignature:        fixture.kindSignature,
					EntitySet:            fixture.entitySet,
					ObservableInputs:     []typedmemory.MemberOfObservableInput{fixture.observable},
					EvaluationProvenance: fixture.evaluationSource,
				},
			))
			return mustTestValue(typedmemory.NewMemberOfMember(otherQuery, basis))
		},
	} {
		evaluate := evaluate
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			facts := staticReferenceFactView{
				target:   fixture.target.Ref(),
				basis:    fixture.observation.GraphSnapshotBasis(),
				evaluate: evaluate,
			}
			_, err := Revalidate(Input{
				CurrentGraph:  fixture.observation,
				TargetTypeEnv: fixture.target,
				TargetRuntime: fixture.runtime,
				ExactTargetReferenceKindFacts: mustTestValue(
					projectgraphobservation.NewExactTargetMemberOfReferenceKindFacts(facts),
				),
			})
			if err == nil {
				t.Fatalf("Revalidate accepted %s target-C judgement", name)
			}
		})
	}
}

type referenceJudgementKind uint8

const (
	memberJudgement referenceJudgementKind = iota + 1
	notMemberJudgement
	undefinedJudgement
)

type referenceRevalidationFixture struct {
	target           typedmemory.TypeEnv
	runtime          projecttypeenvruntime.ExactTargetRuntimeRegistry
	relation         typedmemory.RelationInstance
	observation      projectgraphobservation.CurrentProjectGraphObservation
	kindSignature    typedmemory.KindSignatureDefinition
	entitySet        typedmemory.EntitySetDefinition
	evaluationSource typedmemory.MemberOfEvaluationProvenance
	observable       typedmemory.MemberOfObservableInput
}

func newReferenceRevalidationFixture(t *testing.T) referenceRevalidationFixture {
	t.Helper()
	ref := typeEnvRef(t, 0x81)
	provenance, coverage := typeEnvProvenance(t, 0x81)
	contextRef := mustTestValue(typedmemory.NewBoundedContextRef("ctx:test"))
	contextValue := mustTestValue(typedmemory.NewBoundedContext(contextRef, provenance))
	kindID := mustTestValue(typedmemory.NewKindID("U.Entity"))
	kind := mustTestValue(typedmemory.NewKindDefinition(kindID, provenance))
	valueKind := mustTestValue(typedmemory.NewValueKindRef(ref, kindID))
	refKindID := mustTestValue(typedmemory.NewRefKindID("U.EntityRef"))
	refKind := mustTestValue(typedmemory.NewRefKindRef(ref, refKindID))
	refDefinition := mustTestValue(typedmemory.NewRefKindDefinition(
		refKind,
		valueKind,
		provenance,
	))
	evaluator := mustTestValue(typedmemory.NewRuleRef("test:member-of/entity/v1"))
	entitySet := mustTestValue(typedmemory.NewEntitySetDefinition(
		typedmemory.EntitySetDefinitionInput{
			TypeEnv:         ref,
			Context:         contextRef,
			EnumerationRule: mustTestValue(typedmemory.NewRuleRef("test:entity-set/entity/v1")),
			CandidatePolicy: typedmemory.PersistedEntitiesOnly{},
			Provenance:      provenance,
		},
	))
	kindSignature := mustTestValue(typedmemory.NewKindSignatureDefinition(
		typedmemory.KindSignatureDefinitionInput{
			ValueKind:       valueKind,
			Formality:       typedmemory.SignatureF4,
			DefinednessRule: mustTestValue(typedmemory.NewRuleRef("test:definedness/entity/v1")),
			Evaluator:       evaluator,
			EntitySet:       entitySet.Ref(),
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
	signatureRef := mustTestValue(typedmemory.NewRelationSignatureRef(
		ref,
		mustTestValue(typedmemory.NewSignatureID("Haft.AtConcern")),
	))
	signature := mustTestValue(typedmemory.NewRelationSignature(
		signatureRef,
		[]typedmemory.BoundedContextRef{contextRef},
		[]typedmemory.SlotSpec{slot},
		provenance,
	))
	availability := kindAvailability(
		t,
		ref,
		contextRef,
		kindID,
		provenance,
		0x81,
	)
	target := mustTestValue(
		typedmemory.NewTypeEnvBuilder(ref).
			SetSourceRevision(mustTestValue(typedmemory.NewSourceRevision("reference-revalidation"))).
			SetCompilerSchemaVersion(mustTestValue(typedmemory.NewCompilerSchemaVersion("test.v1"))).
			SetCoverageManifest(coverage).
			AddBoundedContext(contextValue).
			AddKindDefinition(kind).
			AddContextKindAvailability(availability).
			AddRefKindDefinition(refDefinition).
			AddEntitySetDefinition(entitySet).
			AddKindSignatureDefinition(kindSignature).
			AddRelationSignature(signature).
			Build(),
	)
	entity := mustTestValue(typedmemory.NewEntityID("entity:reference-revalidation"))
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
	binding := typedMemoryEnvelope(
		"validated-slot-binding.v1",
		[]byte(slotID.String()),
		filler,
	)
	slice := contextSlice(t, contextRef)
	relation := mustTestValue(typedmemory.DecodeCanonicalRelationInstance(
		typedMemoryEnvelope(
			"validated-relation-instance.v2",
			[]byte("assertion:reference-revalidation"),
			[]byte(signatureRef.String()),
			[]byte(slice.Ref().String()),
			slice.CanonicalBytes(),
			binding,
			[]byte("memory:test:reference-revalidation"),
		),
	))
	evaluationSource := mustTestValue(typedmemory.NewMemberOfEvaluationProvenance(
		typedmemory.MemberOfEvaluationProvenanceInput{
			Reference:         mustTestValue(typedmemory.NewProvenanceRef("memory:test:reference-evaluation")),
			EvaluatorArtifact: mustTestValue(typedmemory.NewCarrierRef("artifact:test-reference-evaluator")),
			EvaluatorEdition:  mustTestValue(typedmemory.NewCarrierEdition("1.0.0")),
			EvaluatorDigest:   digest(t, 0x82),
		},
	))
	observable := mustTestValue(typedmemory.NewMemberOfObservableInput(
		mustTestValue(typedmemory.NewObservableInputRef("observable:test-reference")),
		digest(t, 0x83),
	))
	return referenceRevalidationFixture{
		target:           target,
		runtime:          exactReferenceRuntime(t, evaluator),
		relation:         relation,
		observation:      committedObservation(t, ref, "qnt_1234abcd", []typedmemory.RelationInstance{relation}),
		kindSignature:    kindSignature,
		entitySet:        entitySet,
		evaluationSource: evaluationSource,
		observable:       observable,
	}
}

func (fixture referenceRevalidationFixture) factView(
	t *testing.T,
	basis projecttypeenvselection.ProjectGraphSnapshotBasis,
	kind referenceJudgementKind,
) staticReferenceFactView {
	t.Helper()
	return staticReferenceFactView{
		target: fixture.target.Ref(),
		basis:  basis,
		evaluate: func(request typedmemory.MemberOfEvaluationRequest) typedmemory.MemberOfJudgement {
			switch kind {
			case memberJudgement, notMemberJudgement:
				memberBasis := mustTestValue(typedmemory.NewMemberOfBasis(
					typedmemory.MemberOfBasisInput{
						Query:                request.Query(),
						EvaluationView:       request.View(),
						KindSignature:        fixture.kindSignature,
						EntitySet:            fixture.entitySet,
						ObservableInputs:     []typedmemory.MemberOfObservableInput{fixture.observable},
						EvaluationProvenance: fixture.evaluationSource,
					},
				))
				if kind == memberJudgement {
					return mustTestValue(typedmemory.NewMemberOfMember(request.Query(), memberBasis))
				}
				return mustTestValue(typedmemory.NewMemberOfNotMember(request.Query(), memberBasis))
			case undefinedJudgement:
				missing := mustTestValue(typedmemory.MissingObservableSourceForMemberOf(request.Query()))
				return mustTestValue(typedmemory.NewMemberOfUndefined(
					request,
					[]typedmemory.MemberOfMissingBasis{missing},
					mustTestValue(typedmemory.NewRepairPointer("load-target-memberof-source")),
				))
			default:
				return nil
			}
		},
	}
}

type staticReferenceFactView struct {
	target   typedmemory.TypeEnvRef
	basis    projecttypeenvselection.ProjectGraphSnapshotBasis
	evaluate func(typedmemory.MemberOfEvaluationRequest) typedmemory.MemberOfJudgement
}

func (view staticReferenceFactView) TargetTypeEnv() typedmemory.TypeEnvRef {
	return view.target
}

func (view staticReferenceFactView) GraphSnapshotBasis() projecttypeenvselection.ProjectGraphSnapshotBasis {
	return view.basis
}

func (view staticReferenceFactView) EvaluateMemberOf(
	request typedmemory.MemberOfEvaluationRequest,
) typedmemory.MemberOfJudgement {
	return view.evaluate(request)
}

type inertReferenceMemberOfEngine struct{}

func (inertReferenceMemberOfEngine) EvaluateMemberOf(
	context.Context,
	memberofevaluation.MemberOfEvaluationInput,
) (typedmemory.MemberOfJudgement, error) {
	return nil, fmt.Errorf("inert reference revalidation evaluator")
}

func exactReferenceRuntime(
	t *testing.T,
	rule typedmemory.RuleRef,
) projecttypeenvruntime.ExactTargetRuntimeRegistry {
	t.Helper()
	memberEntry := mustTestValue(runtimemechanism.NewMemberOfEntry(rule))
	deliveryEntry := mustTestValue(
		runtimemechanism.NewCarrierMembershipDeliveryEntry(rule),
	)
	catalog := mustTestValue(runtimemechanism.SealRuntimeMechanismArtifactV1(
		mustTestValue(typedmemory.NewCarrierRef("artifact:test-reference-runtime")),
		mustTestValue(typedmemory.NewCarrierEdition("1.0.0")),
		[]runtimemechanism.RuntimeMechanismEntryV1{
			memberEntry,
			deliveryEntry,
		},
	))
	mechanism := mustTestValue(
		projecttypeenv.NewRuntimeMechanismArtifactPinFromArtifact(catalog),
	)
	evaluatorPin := mustTestValue(projecttypeenv.NewEvaluatorRuntimeMechanismPin(
		projecttypeenv.EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         projecttypeenv.RuntimeMechanismContractMemberOf,
			Mechanism:        mechanism,
			ResolvedArtifact: &catalog,
		},
	))
	deliveryPin := mustTestValue(
		projecttypeenv.NewCarrierMembershipRuntimeMechanismPin(
			projecttypeenv.CarrierMembershipRuntimeMechanismPinInput{
				Rule:             rule,
				Mechanism:        mechanism,
				ResolvedArtifact: &catalog,
			},
		),
	)
	policy := exactReferenceRegistrationPolicy(t, catalog, rule)
	policyPin := mustTestValue(projecttypeenv.NewRegistrationPolicyPin(policy))
	basis := mustTestValue(projecttypeenv.SealRuntimeEvaluationBasisWithPins(
		[]projecttypeenv.RuntimeEvaluationBasisPin{
			evaluatorPin,
			deliveryPin,
			policyPin,
		},
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
	registration := mustTestValue(memberofruntime.NewRegistration(
		rule,
		mechanismIdentity,
		inertReferenceMemberOfEngine{},
	))
	registry := mustTestValue(memberofruntime.NewRegistry(
		[]memberofruntime.Registration{registration},
	))
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
		issues := []projecttypeenvruntime.Issue(nil)
		if unavailable, present := resolution.(projecttypeenvruntime.Unavailable); present {
			issues = unavailable.Issues()
		}
		t.Fatalf("reference runtime = %s %#v, want matched", resolution.Kind(), issues)
	}
	exact, ok := matched.Registry()
	if !ok {
		t.Fatal("matched reference runtime omitted exact registry")
	}
	return exact
}

func exactReferenceRegistrationPolicy(
	t *testing.T,
	catalog runtimemechanism.RuntimeMechanismArtifactV1,
	rule typedmemory.RuleRef,
) recordmembershipregistration.RegistrationArtifactV1 {
	t.Helper()
	identity := catalog.Identity()
	evaluator := mustTestValue(recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.EvaluatorMechanism,
			Rule:     rule,
			Artifact: identity.Artifact(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	))
	delivery := mustTestValue(recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     recordmembershipregistration.SourceDeliveryBoundaryMechanism,
			Rule:     rule,
			Artifact: identity.Artifact(),
			Edition:  identity.Edition(),
			Digest:   identity.Digest(),
		},
	))
	manifest := mustTestValue(recordmapping.NewMappingManifestRef(
		"test.reference-revalidation",
		"1.0.0",
		digest(t, 0x84),
	))
	adapter := mustTestValue(recordmapping.NewAdapterVersion("test-reference/1.0.0"))
	mapping := mustTestValue(recordmembershipregistration.NewAcceptedMapping(
		recordmembershipregistration.AcceptedMappingInput{
			Manifest: manifest,
			Adapter:  adapter,
		},
	))
	return mustTestValue(recordmembershipregistration.SealRegistrationArtifactV1(
		recordmembershipregistration.RegistrationArtifactInputV1{
			Evaluator:      evaluator,
			SourceDelivery: delivery,
			Mappings:       []recordmembershipregistration.AcceptedMapping{mapping},
		},
	))
}
