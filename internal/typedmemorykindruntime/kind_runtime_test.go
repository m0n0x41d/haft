package typedmemorykindruntime

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestEntitySetEnumerationDistinguishesEmptyAndUndefined(t *testing.T) {
	fixture := newKindRuntimeFixture(t, true)
	request := fixture.entitySetRequest(
		t,
		[]typedmemory.EntityID{fixture.entity, fixture.otherEntity, fixture.entity},
	)
	evaluator := fixture.entitySetEvaluator(t)
	result, err := evaluator.Evaluate(request)
	if err != nil {
		t.Fatalf("EntitySet Evaluate(): %v", err)
	}
	enumerated, ok := result.(EntitySetEnumerated)
	if !ok {
		t.Fatalf("EntitySet result = %T; want EntitySetEnumerated", result)
	}
	if !enumerated.Contains(fixture.entity) || !enumerated.Contains(fixture.otherEntity) {
		t.Fatal("enumerated EntitySet lost an exact member")
	}
	if len(enumerated.Entities()) != 2 {
		t.Fatal("enumerated EntitySet did not normalize exact duplicates")
	}

	reversed := fixture.entitySetRequest(
		t,
		[]typedmemory.EntityID{fixture.otherEntity, fixture.entity},
	)
	reversedResult, err := evaluator.Evaluate(reversed)
	if err != nil {
		t.Fatalf("EntitySet Evaluate(reversed): %v", err)
	}
	if result.Digest() != reversedResult.Digest() ||
		!bytes.Equal(result.CanonicalBytes(), reversedResult.CanonicalBytes()) {
		t.Fatal("EntitySet order or exact duplication changed the result")
	}

	emptyResult, err := evaluator.Evaluate(fixture.entitySetRequest(t, nil))
	if err != nil {
		t.Fatalf("EntitySet Evaluate(empty): %v", err)
	}
	empty, ok := emptyResult.(EntitySetEnumerated)
	if !ok || len(empty.Entities()) != 0 {
		t.Fatal("exact empty EntitySet was not represented as Enumerated")
	}

	missingObservation, err := NewMissingEntitySetObservation(
		MissingEntitySetObservationInput{
			ObservableInputs: []typedmemory.ObservableInputRef{
				mustObservableRef(t, "observable:kind-runtime/project-entities"),
			},
			Repair: mustRepair(t, "repair:load-kind-runtime-project-entities"),
		},
	)
	if err != nil {
		t.Fatalf("NewMissingEntitySetObservation(): %v", err)
	}
	missingRequest := fixture.entitySetRequestWithObservation(t, missingObservation)
	undefinedResult, err := evaluator.Evaluate(missingRequest)
	if err != nil {
		t.Fatalf("EntitySet Evaluate(undefined): %v", err)
	}
	undefined, ok := undefinedResult.(EntitySetEnumerationUndefined)
	if !ok {
		t.Fatalf("missing EntitySet result = %T; want Undefined", undefinedResult)
	}
	if undefined.Kind() != EntitySetEnumerationUndefinedResult ||
		len(undefined.MissingObservableInputs()) != 1 {
		t.Fatal("undefined EntitySet lost the exact missing source")
	}
	if undefined.Digest() == empty.Digest() {
		t.Fatal("undefined EntitySet collapsed to a defined empty set")
	}
}

func TestCandidateVisibilityBindsExactProspectivePrefix(t *testing.T) {
	fixture := newKindRuntimeFixture(t, true)
	visibilityRule := mustRule(t, "test:kind-runtime/candidate-visibility/v1")
	policy, err := typedmemory.NewPriorBatchDeclarationsVisible(visibilityRule)
	if err != nil {
		t.Fatalf("NewPriorBatchDeclarationsVisible(): %v", err)
	}
	definition := fixture.entitySetDefinition(t, policy)
	view := fixture.prospectiveView(t)
	request, err := NewCandidateVisibilityRequest(CandidateVisibilityRequestInput{
		Definition: definition,
		View:       view,
	})
	if err != nil {
		t.Fatalf("NewCandidateVisibilityRequest(): %v", err)
	}
	evaluator, err := NewCandidateVisibilityEvaluator(
		visibilityRule,
		fixture.mechanism,
	)
	if err != nil {
		t.Fatalf("NewCandidateVisibilityEvaluator(): %v", err)
	}
	result, err := evaluator.Evaluate(request)
	if err != nil {
		t.Fatalf("CandidateVisibility Evaluate(): %v", err)
	}
	visible, ok := result.(CandidateVisible)
	if !ok {
		t.Fatalf("CandidateVisibility result = %T; want CandidateVisible", result)
	}
	if visible.Basis().PrefixDigest() != view.OrderedCandidatePrefix().Digest() ||
		visible.Basis().DeclarationDigest() != view.DeclarationDigest() {
		t.Fatal("candidate visibility lost the exact declaration prefix basis")
	}
	candidates, err := NewProspectiveEntitySetCandidateBasis(visible)
	if err != nil {
		t.Fatalf("NewProspectiveEntitySetCandidateBasis(): %v", err)
	}
	exactObservation := mustEntitySetObservation(t, []typedmemory.EntityID{fixture.entity})
	enumerationRequest, err := NewEntitySetEnumerationRequest(EntitySetEnumerationRequestInput{
		ContextSlice: fixture.contextSlice,
		View:         view,
		Definition:   definition,
		Candidates:   candidates,
		Observation:  exactObservation,
	})
	if err != nil {
		t.Fatalf("prospective EntitySet request: %v", err)
	}
	enumerationEvaluator, err := NewEntitySetEnumerationEvaluator(
		definition.EnumerationRule(),
		fixture.mechanism,
	)
	if err != nil {
		t.Fatalf("NewEntitySetEnumerationEvaluator(): %v", err)
	}
	enumerationResult, err := enumerationEvaluator.Evaluate(enumerationRequest)
	if err != nil {
		t.Fatalf("prospective EntitySet Evaluate(): %v", err)
	}
	if !bytes.Equal(
		enumerationResult.CandidateBasis().CanonicalBytes(),
		candidates.CanonicalBytes(),
	) {
		t.Fatal("EntitySet result dropped its exact prospective visibility basis")
	}
	enumerated, ok := enumerationResult.(EntitySetEnumerated)
	if !ok {
		t.Fatalf("prospective EntitySet result = %T", enumerationResult)
	}
	prospectiveSignature, err := typedmemory.NewKindSignatureDefinition(
		typedmemory.KindSignatureDefinitionInput{
			ValueKind:       fixture.signature.ValueKind(),
			Formality:       fixture.signature.Formality(),
			Assumptions:     fixture.signature.Assumptions(),
			DefinednessRule: fixture.signature.DefinednessRule(),
			Evaluator:       fixture.signature.Evaluator(),
			EntitySet:       definition.Ref(),
			Provenance:      fixture.provenance,
		},
	)
	if err != nil {
		t.Fatalf("NewKindSignatureDefinition(prospective): %v", err)
	}
	memberRequest, err := typedmemory.NewMemberOfEvaluationRequest(
		fixture.memberQuery,
		view,
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(prospective): %v", err)
	}
	definednessRequest, err := NewKindDefinednessRequest(
		KindDefinednessRequestInput{
			MemberOfRequest: memberRequest,
			Signature:       prospectiveSignature,
			Enumeration:     enumerated,
			Observation:     mustDefinednessObservation(t),
		},
	)
	if err != nil {
		t.Fatalf("NewKindDefinednessRequest(prospective): %v", err)
	}
	definednessEvaluator, err := NewKindDefinednessEvaluator(
		prospectiveSignature.DefinednessRule(),
		fixture.mechanism,
	)
	if err != nil {
		t.Fatalf("NewKindDefinednessEvaluator(prospective): %v", err)
	}
	definednessResult, err := definednessEvaluator.Evaluate(definednessRequest)
	if err != nil {
		t.Fatalf("KindDefinedness Evaluate(prospective): %v", err)
	}
	defined, ok := definednessResult.(KindDefined)
	if !ok {
		t.Fatalf("prospective definedness result = %T", definednessResult)
	}
	prospectiveEvidence, err := NewProspectiveC32CandidateEvidence(request, visible)
	if err != nil {
		t.Fatalf("NewProspectiveC32CandidateEvidence(): %v", err)
	}
	certificate, err := NewC32PrerequisiteCertificateFromResults(
		C32PrerequisiteCertificateFromResultsInput{
			EnumerationRequest: enumerationRequest,
			EnumerationResult:  enumerated,
			DefinednessRequest: definednessRequest,
			DefinednessResult:  defined,
			CandidateEvidence:  prospectiveEvidence,
		},
	)
	if err != nil {
		t.Fatalf("NewC32PrerequisiteCertificateFromResults(prospective): %v", err)
	}
	visibility, ok := certificate.CandidateVisibility().(typedmemory.C32ProspectiveVisibilityCoordinate)
	if !ok ||
		visibility.RequestDigest() != request.Digest() ||
		visibility.ResultDigest() != visible.Digest() {
		t.Fatal("prospective certificate lost candidate request/result coordinates")
	}

	otherView := fixture.prospectiveViewAtRevision(t, 8)
	if _, err := NewEntitySetEnumerationRequest(EntitySetEnumerationRequestInput{
		ContextSlice: fixture.contextSlice,
		View:         otherView,
		Definition:   definition,
		Candidates:   candidates,
		Observation:  exactObservation,
	}); err == nil {
		t.Fatal("prospective EntitySet accepted visibility from another exact view")
	}
}

func TestKindDefinednessReturnsDefinedOnlyWithEveryPrerequisite(t *testing.T) {
	fixture := newKindRuntimeFixture(t, true)
	enumeration := fixture.enumeration(t, []typedmemory.EntityID{fixture.entity})
	request := fixture.kindDefinednessRequest(
		t,
		enumeration,
		mustDefinednessObservation(t),
	)
	evaluator := fixture.kindDefinednessEvaluator(t)
	result, err := evaluator.Evaluate(request)
	if err != nil {
		t.Fatalf("KindDefinedness Evaluate(): %v", err)
	}
	defined, ok := result.(KindDefined)
	if !ok {
		t.Fatalf("KindDefinedness result = %T; want KindDefined", result)
	}
	if defined.Kind() != KindDefinedResult ||
		defined.Basis().EnumerationDigest() != enumeration.Digest() ||
		len(defined.Basis().MatchedAssumptions()) != 1 {
		t.Fatal("definedness basis lost an exact upstream prerequisite")
	}
	inputs := defined.Basis().ObservableInputs()
	inputs[0] = mustObservable(t, "observable:kind-runtime/mutated", 0xfe)
	if defined.Basis().ObservableInputs()[0].Reference() == inputs[0].Reference() {
		t.Fatal("mutating returned inputs changed the sealed definedness basis")
	}
	certificate, err := NewC32PrerequisiteCertificateFromResults(
		C32PrerequisiteCertificateFromResultsInput{
			EnumerationRequest: fixture.entitySetRequest(
				t,
				[]typedmemory.EntityID{fixture.entity},
			),
			EnumerationResult:  enumeration,
			DefinednessRequest: request,
			DefinednessResult:  defined,
			CandidateEvidence:  NewPersistedC32CandidateEvidence(),
		},
	)
	if err != nil {
		t.Fatalf("NewC32PrerequisiteCertificateFromResults(): %v", err)
	}
	if certificate.EnumerationResultDigest() != enumeration.Digest() ||
		certificate.DefinednessResultDigest() != defined.Digest() ||
		certificate.MemberOfRequestDigest() != fixture.memberRequest.Digest() ||
		certificate.CandidateVisibility().Kind() !=
			typedmemory.C32PersistedCandidateVisibility {
		t.Fatal("C.3.2 certificate lost an exact performed prerequisite coordinate")
	}
}

func TestKindDefinednessFailsClosedAcrossAllUndefinedPositions(t *testing.T) {
	fixture := newKindRuntimeFixture(t, true)
	evaluator := fixture.kindDefinednessEvaluator(t)
	exactObservation := mustDefinednessObservation(t)

	undefinedEnumeration := fixture.undefinedEnumeration(t)
	result, err := evaluator.Evaluate(fixture.kindDefinednessRequest(
		t,
		undefinedEnumeration,
		exactObservation,
	))
	if err != nil {
		t.Fatalf("KindDefinedness Evaluate(undefined EntitySet): %v", err)
	}
	assertDefinednessFailureKind(
		t,
		result,
		KindDefinednessEntitySetUnavailable,
	)

	outside := fixture.enumeration(t, []typedmemory.EntityID{fixture.otherEntity})
	result, err = evaluator.Evaluate(fixture.kindDefinednessRequest(
		t,
		outside,
		exactObservation,
	))
	if err != nil {
		t.Fatalf("KindDefinedness Evaluate(outside): %v", err)
	}
	assertDefinednessFailureKind(t, result, KindDefinednessEntityOutsideSet)

	missingObservation, err := NewMissingKindDefinednessObservation(
		MissingKindDefinednessObservationInput{
			ObservableInputs: []typedmemory.ObservableInputRef{
				mustObservableRef(t, "observable:kind-runtime/definedness"),
			},
			Repair: mustRepair(t, "repair:load-kind-runtime-definedness-input"),
		},
	)
	if err != nil {
		t.Fatalf("NewMissingKindDefinednessObservation(): %v", err)
	}
	inside := fixture.enumeration(t, []typedmemory.EntityID{fixture.entity})
	result, err = evaluator.Evaluate(fixture.kindDefinednessRequest(
		t,
		inside,
		missingObservation,
	))
	if err != nil {
		t.Fatalf("KindDefinedness Evaluate(missing observation): %v", err)
	}
	assertDefinednessFailureKind(t, result, KindDefinednessObservationUnavailable)

	missingAssumptionFixture := newKindRuntimeFixture(t, false)
	missingAssumptionEnumeration := missingAssumptionFixture.enumeration(
		t,
		[]typedmemory.EntityID{missingAssumptionFixture.entity},
	)
	result, err = missingAssumptionFixture.kindDefinednessEvaluator(t).Evaluate(
		missingAssumptionFixture.kindDefinednessRequest(
			t,
			missingAssumptionEnumeration,
			exactObservation,
		),
	)
	if err != nil {
		t.Fatalf("KindDefinedness Evaluate(missing assumption): %v", err)
	}
	assertDefinednessFailureKind(t, result, KindDefinednessAssumptionsUnavailable)
}

func TestKindDefinednessRejectsCrossViewEnumeration(t *testing.T) {
	fixture := newKindRuntimeFixture(t, true)
	enumeration := fixture.enumeration(t, []typedmemory.EntityID{fixture.entity})
	otherView, err := typedmemory.NewPersistedSnapshotView(
		fixture.typeEnv,
		typedmemory.NewGraphRevision(99),
	)
	if err != nil {
		t.Fatalf("NewPersistedSnapshotView(other): %v", err)
	}
	otherMemberRequest, err := typedmemory.NewMemberOfEvaluationRequest(
		fixture.memberQuery,
		otherView,
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(other): %v", err)
	}
	if _, err := NewKindDefinednessRequest(KindDefinednessRequestInput{
		MemberOfRequest: otherMemberRequest,
		Signature:       fixture.signature,
		Enumeration:     enumeration,
		Observation:     mustDefinednessObservation(t),
	}); err == nil {
		t.Fatal("Kind definedness accepted EntitySet evaluation from another view")
	}
}

func TestC32CertificateFactoryRejectsCrossChainSubstitution(t *testing.T) {
	fixture := newKindRuntimeFixture(t, true)
	firstRequest := fixture.entitySetRequest(
		t,
		[]typedmemory.EntityID{fixture.entity},
	)
	firstResult, err := fixture.entitySetEvaluator(t).Evaluate(firstRequest)
	if err != nil {
		t.Fatalf("EntitySet Evaluate(first): %v", err)
	}
	first, ok := firstResult.(EntitySetEnumerated)
	if !ok {
		t.Fatalf("first EntitySet result = %T", firstResult)
	}
	second := fixture.enumeration(
		t,
		[]typedmemory.EntityID{fixture.entity, fixture.otherEntity},
	)
	definednessRequest := fixture.kindDefinednessRequest(
		t,
		second,
		mustDefinednessObservation(t),
	)
	definednessResult, err := fixture.kindDefinednessEvaluator(t).Evaluate(
		definednessRequest,
	)
	if err != nil {
		t.Fatalf("KindDefinedness Evaluate(second): %v", err)
	}
	defined, ok := definednessResult.(KindDefined)
	if !ok {
		t.Fatalf("definedness result = %T", definednessResult)
	}
	if _, err := NewC32PrerequisiteCertificateFromResults(
		C32PrerequisiteCertificateFromResultsInput{
			EnumerationRequest: firstRequest,
			EnumerationResult:  first,
			DefinednessRequest: definednessRequest,
			DefinednessResult:  defined,
			CandidateEvidence:  NewPersistedC32CandidateEvidence(),
		},
	); err == nil {
		t.Fatal("C.3.2 certificate factory joined results from different chains")
	}
}

type kindRuntimeFixture struct {
	typeEnv       typedmemory.TypeEnvRef
	context       typedmemory.BoundedContextRef
	provenance    typedmemory.DeclarationProvenance
	entitySet     typedmemory.EntitySetDefinition
	signature     typedmemory.KindSignatureDefinition
	contextSlice  typedmemory.ContextSlice
	memberQuery   typedmemory.MemberOfQuery
	memberRequest typedmemory.MemberOfEvaluationRequest
	persistedView typedmemory.PersistedSnapshotView
	entity        typedmemory.EntityID
	otherEntity   typedmemory.EntityID
	mechanism     EvaluationMechanism
}

func newKindRuntimeFixture(t *testing.T, pinAssumption bool) kindRuntimeFixture {
	t.Helper()
	typeEnv := mustTypeEnvRef(t, 0x11)
	context := mustContext(t, "context:kind-runtime")
	provenance := mustProvenance(t)
	entity := mustEntity(t, "entity:kind-runtime/target")
	otherEntity := mustEntity(t, "entity:kind-runtime/other")
	assumption := mustAssumption(t, "standard:kind-runtime", "v1", 0x22)
	standardPins := []typedmemory.StandardPin(nil)
	if pinAssumption {
		pin, err := typedmemory.NewStandardPin(
			assumption.Reference(),
			assumption.Edition(),
			assumption.Digest(),
		)
		if err != nil {
			t.Fatalf("NewStandardPin(): %v", err)
		}
		standardPins = []typedmemory.StandardPin{pin}
	}
	gamma, err := typedmemory.NewGammaPoint(
		time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewGammaPoint(): %v", err)
	}
	contextSlice, err := typedmemory.NewContextSlice(typedmemory.ContextSliceInput{
		Context:      context,
		StandardPins: standardPins,
		GammaTime:    gamma,
	})
	if err != nil {
		t.Fatalf("NewContextSlice(): %v", err)
	}
	valueKind, err := typedmemory.NewValueKindRef(
		typeEnv,
		mustKindID(t, "Haft.TestRecord"),
	)
	if err != nil {
		t.Fatalf("NewValueKindRef(): %v", err)
	}
	fixture := kindRuntimeFixture{
		typeEnv:      typeEnv,
		context:      context,
		provenance:   provenance,
		contextSlice: contextSlice,
		entity:       entity,
		otherEntity:  otherEntity,
		mechanism:    mustMechanism(t),
	}
	fixture.entitySet = fixture.entitySetDefinition(t, typedmemory.PersistedEntitiesOnly{})
	formality, err := typedmemory.NewSignatureFormality(4)
	if err != nil {
		t.Fatalf("NewSignatureFormality(): %v", err)
	}
	fixture.signature, err = typedmemory.NewKindSignatureDefinition(
		typedmemory.KindSignatureDefinitionInput{
			ValueKind:       valueKind,
			Formality:       formality,
			Assumptions:     []typedmemory.KindAssumptionPin{assumption},
			DefinednessRule: mustRule(t, "test:kind-runtime/definedness/v1"),
			Evaluator:       mustRule(t, "test:kind-runtime/member-of/v1"),
			EntitySet:       fixture.entitySet.Ref(),
			Provenance:      provenance,
		},
	)
	if err != nil {
		t.Fatalf("NewKindSignatureDefinition(): %v", err)
	}
	fixture.memberQuery, err = typedmemory.NewMemberOfQuery(
		entity,
		valueKind,
		contextSlice,
	)
	if err != nil {
		t.Fatalf("NewMemberOfQuery(): %v", err)
	}
	fixture.persistedView, err = typedmemory.NewPersistedSnapshotView(
		typeEnv,
		typedmemory.NewGraphRevision(7),
	)
	if err != nil {
		t.Fatalf("NewPersistedSnapshotView(): %v", err)
	}
	fixture.memberRequest, err = typedmemory.NewMemberOfEvaluationRequest(
		fixture.memberQuery,
		fixture.persistedView,
	)
	if err != nil {
		t.Fatalf("NewMemberOfEvaluationRequest(): %v", err)
	}
	return fixture
}

func (fixture kindRuntimeFixture) entitySetDefinition(
	t *testing.T,
	policy typedmemory.EntitySetCandidatePolicy,
) typedmemory.EntitySetDefinition {
	t.Helper()
	definition, err := typedmemory.NewEntitySetDefinition(
		typedmemory.EntitySetDefinitionInput{
			TypeEnv:         fixture.typeEnv,
			Context:         fixture.context,
			EnumerationRule: mustRule(t, "test:kind-runtime/entity-set/v1"),
			CandidatePolicy: policy,
			Provenance:      fixture.provenance,
		},
	)
	if err != nil {
		t.Fatalf("NewEntitySetDefinition(): %v", err)
	}
	return definition
}

func (fixture kindRuntimeFixture) entitySetRequest(
	t *testing.T,
	entities []typedmemory.EntityID,
) EntitySetEnumerationRequest {
	t.Helper()
	return fixture.entitySetRequestWithObservation(
		t,
		mustEntitySetObservation(t, entities),
	)
}

func (fixture kindRuntimeFixture) entitySetRequestWithObservation(
	t *testing.T,
	observation EntitySetObservation,
) EntitySetEnumerationRequest {
	t.Helper()
	candidates, err := NewPersistedEntitySetCandidateBasis(fixture.persistedView)
	if err != nil {
		t.Fatalf("NewPersistedEntitySetCandidateBasis(): %v", err)
	}
	request, err := NewEntitySetEnumerationRequest(EntitySetEnumerationRequestInput{
		ContextSlice: fixture.contextSlice,
		View:         fixture.persistedView,
		Definition:   fixture.entitySet,
		Candidates:   candidates,
		Observation:  observation,
	})
	if err != nil {
		t.Fatalf("NewEntitySetEnumerationRequest(): %v", err)
	}
	return request
}

func (fixture kindRuntimeFixture) entitySetEvaluator(
	t *testing.T,
) EntitySetEnumerationEvaluator {
	t.Helper()
	evaluator, err := NewEntitySetEnumerationEvaluator(
		fixture.entitySet.EnumerationRule(),
		fixture.mechanism,
	)
	if err != nil {
		t.Fatalf("NewEntitySetEnumerationEvaluator(): %v", err)
	}
	return evaluator
}

func (fixture kindRuntimeFixture) enumeration(
	t *testing.T,
	entities []typedmemory.EntityID,
) EntitySetEnumerated {
	t.Helper()
	result, err := fixture.entitySetEvaluator(t).Evaluate(
		fixture.entitySetRequest(t, entities),
	)
	if err != nil {
		t.Fatalf("EntitySet Evaluate(): %v", err)
	}
	enumerated, ok := result.(EntitySetEnumerated)
	if !ok {
		t.Fatalf("EntitySet result = %T; want Enumerated", result)
	}
	return enumerated
}

func (fixture kindRuntimeFixture) undefinedEnumeration(
	t *testing.T,
) EntitySetEnumerationUndefined {
	t.Helper()
	observation, err := NewMissingEntitySetObservation(
		MissingEntitySetObservationInput{
			ObservableInputs: []typedmemory.ObservableInputRef{
				mustObservableRef(t, "observable:kind-runtime/entity-set"),
			},
			Repair: mustRepair(t, "repair:load-kind-runtime-entity-set"),
		},
	)
	if err != nil {
		t.Fatalf("NewMissingEntitySetObservation(): %v", err)
	}
	result, err := fixture.entitySetEvaluator(t).Evaluate(
		fixture.entitySetRequestWithObservation(t, observation),
	)
	if err != nil {
		t.Fatalf("EntitySet Evaluate(undefined): %v", err)
	}
	undefined, ok := result.(EntitySetEnumerationUndefined)
	if !ok {
		t.Fatalf("EntitySet result = %T; want Undefined", result)
	}
	return undefined
}

func (fixture kindRuntimeFixture) kindDefinednessRequest(
	t *testing.T,
	enumeration EntitySetEnumerationResult,
	observation KindDefinednessObservation,
) KindDefinednessRequest {
	t.Helper()
	request, err := NewKindDefinednessRequest(KindDefinednessRequestInput{
		MemberOfRequest: fixture.memberRequest,
		Signature:       fixture.signature,
		Enumeration:     enumeration,
		Observation:     observation,
	})
	if err != nil {
		t.Fatalf("NewKindDefinednessRequest(): %v", err)
	}
	return request
}

func (fixture kindRuntimeFixture) kindDefinednessEvaluator(
	t *testing.T,
) KindDefinednessEvaluator {
	t.Helper()
	evaluator, err := NewKindDefinednessEvaluator(
		fixture.signature.DefinednessRule(),
		fixture.mechanism,
	)
	if err != nil {
		t.Fatalf("NewKindDefinednessEvaluator(): %v", err)
	}
	return evaluator
}

func (fixture kindRuntimeFixture) prospectiveView(t *testing.T) typedmemory.ProspectiveBatchView {
	t.Helper()
	return fixture.prospectiveViewAtRevision(t, 7)
}

func (fixture kindRuntimeFixture) prospectiveViewAtRevision(
	t *testing.T,
	revision uint64,
) typedmemory.ProspectiveBatchView {
	t.Helper()
	localID, err := typedmemory.NewBatchLocalRef("local:kind-runtime/target")
	if err != nil {
		t.Fatalf("NewBatchLocalRef(): %v", err)
	}
	label, err := typedmemory.NewEntityLabel("Kind runtime target")
	if err != nil {
		t.Fatalf("NewEntityLabel(): %v", err)
	}
	declaration, err := typedmemory.NewDeclareEntity(
		fixture.entity,
		localID,
		fixture.context,
		label,
		fixture.provenance.Reference(),
	)
	if err != nil {
		t.Fatalf("NewDeclareEntity(): %v", err)
	}
	changeSet, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{declaration},
	)
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(): %v", err)
	}
	prefix, err := typedmemory.ComputeOrderedCandidatePrefix(changeSet, 1)
	if err != nil {
		t.Fatalf("ComputeOrderedCandidatePrefix(): %v", err)
	}
	refKind, err := typedmemory.NewRefKindRef(
		fixture.typeEnv,
		mustRefKindID(t, "Haft.TestRecordRef"),
	)
	if err != nil {
		t.Fatalf("NewRefKindRef(): %v", err)
	}
	localReference, err := typedmemory.NewLocalRef(refKind, localID)
	if err != nil {
		t.Fatalf("NewLocalRef(): %v", err)
	}
	referenceID, err := typedmemory.NewReferenceID(fixture.entity.String())
	if err != nil {
		t.Fatalf("NewReferenceID(): %v", err)
	}
	persistedReference, err := typedmemory.NewPersistedRef(refKind, referenceID)
	if err != nil {
		t.Fatalf("NewPersistedRef(): %v", err)
	}
	view, err := typedmemory.NewProspectiveBatchView(
		typedmemory.ProspectiveBatchViewInput{
			TypeEnv:                  fixture.typeEnv,
			PreStateGraphRevision:    typedmemory.NewGraphRevision(revision),
			EvaluationChangeOrdinal:  1,
			DeclarationChangeOrdinal: 0,
			Declaration:              declaration,
			LocalReference:           localReference,
			PersistedReference:       persistedReference,
			OrderedCandidatePrefix:   prefix,
		},
	)
	if err != nil {
		t.Fatalf("NewProspectiveBatchView(): %v", err)
	}
	return view
}

func mustEntitySetObservation(
	t *testing.T,
	entities []typedmemory.EntityID,
) ExactEntitySetObservation {
	t.Helper()
	observation, err := NewExactEntitySetObservation(ExactEntitySetObservationInput{
		Entities: entities,
		ObservableInputs: []typedmemory.MemberOfObservableInput{
			mustObservable(t, "observable:kind-runtime/project-entities", 0x41),
		},
	})
	if err != nil {
		t.Fatalf("NewExactEntitySetObservation(): %v", err)
	}
	return observation
}

func mustDefinednessObservation(t *testing.T) ExactKindDefinednessObservation {
	t.Helper()
	observation, err := NewExactKindDefinednessObservation(
		[]typedmemory.MemberOfObservableInput{
			mustObservable(t, "observable:kind-runtime/definedness", 0x42),
		},
	)
	if err != nil {
		t.Fatalf("NewExactKindDefinednessObservation(): %v", err)
	}
	return observation
}

func assertDefinednessFailureKind(
	t *testing.T,
	result KindDefinednessResult,
	want KindDefinednessFailureKind,
) {
	t.Helper()
	undefined, ok := result.(KindDefinednessUndefined)
	if !ok {
		t.Fatalf("KindDefinedness result = %T; want Undefined", result)
	}
	if undefined.Failure().Kind() != want {
		t.Fatalf(
			"definedness failure = %s; want %s",
			undefined.Failure().Kind().String(),
			want.String(),
		)
	}
}

func mustMechanism(t *testing.T) EvaluationMechanism {
	t.Helper()
	mechanism, err := NewEvaluationMechanism(EvaluationMechanismInput{
		Artifact: mustCarrier(t, "binary:kind-runtime-evaluator"),
		Edition:  mustEdition(t, "build-20260717.1"),
		Digest:   mustDigest(t, 0x51),
	})
	if err != nil {
		t.Fatalf("NewEvaluationMechanism(): %v", err)
	}
	return mechanism
}

func mustProvenance(t *testing.T) typedmemory.DeclarationProvenance {
	t.Helper()
	lineRange, err := typedmemory.NewSourceLineRange(1, 2)
	if err != nil {
		t.Fatalf("NewSourceLineRange(): %v", err)
	}
	location, err := typedmemory.NewUnpatternedSourceLocation(
		mustSourceUnit(t, "source:kind-runtime-fixture"),
		mustSourceRevision(t, "fixture-revision"),
		mustDigest(t, 0x61),
		lineRange,
	)
	if err != nil {
		t.Fatalf("NewUnpatternedSourceLocation(): %v", err)
	}
	provenance, err := typedmemory.NewFPFSourceProvenance(
		mustProvenanceRef(t, "provenance:kind-runtime-fixture"),
		location,
		mustCompilerRule(t, "haft.test.kind-runtime.v1"),
	)
	if err != nil {
		t.Fatalf("NewFPFSourceProvenance(): %v", err)
	}
	return provenance
}

func mustObservable(
	t *testing.T,
	raw string,
	seed byte,
) typedmemory.MemberOfObservableInput {
	t.Helper()
	input, err := typedmemory.NewMemberOfObservableInput(
		mustObservableRef(t, raw),
		mustDigest(t, seed),
	)
	if err != nil {
		t.Fatalf("NewMemberOfObservableInput(): %v", err)
	}
	return input
}

func mustDigest(t *testing.T, seed byte) typedmemory.SHA256Digest {
	t.Helper()
	raw := "sha256:" + strings.Repeat(fmt.Sprintf("%02x", seed), 32)
	value, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return value
}

func mustTypeEnvRef(t *testing.T, seed byte) typedmemory.TypeEnvRef {
	t.Helper()
	value, err := typedmemory.NewTypeEnvRef(mustDigest(t, seed))
	if err != nil {
		t.Fatalf("NewTypeEnvRef(): %v", err)
	}
	return value
}

func mustContext(t *testing.T, raw string) typedmemory.BoundedContextRef {
	t.Helper()
	value, err := typedmemory.NewBoundedContextRef(raw)
	if err != nil {
		t.Fatalf("NewBoundedContextRef(): %v", err)
	}
	return value
}

func mustEntity(t *testing.T, raw string) typedmemory.EntityID {
	t.Helper()
	value, err := typedmemory.NewEntityID(raw)
	if err != nil {
		t.Fatalf("NewEntityID(): %v", err)
	}
	return value
}

func mustKindID(t *testing.T, raw string) typedmemory.KindID {
	t.Helper()
	value, err := typedmemory.NewKindID(raw)
	if err != nil {
		t.Fatalf("NewKindID(): %v", err)
	}
	return value
}

func mustRefKindID(t *testing.T, raw string) typedmemory.RefKindID {
	t.Helper()
	value, err := typedmemory.NewRefKindID(raw)
	if err != nil {
		t.Fatalf("NewRefKindID(): %v", err)
	}
	return value
}

func mustRule(t *testing.T, raw string) typedmemory.RuleRef {
	t.Helper()
	value, err := typedmemory.NewRuleRef(raw)
	if err != nil {
		t.Fatalf("NewRuleRef(): %v", err)
	}
	return value
}

func mustAssumption(
	t *testing.T,
	reference string,
	edition string,
	seed byte,
) typedmemory.KindAssumptionPin {
	t.Helper()
	value, err := typedmemory.NewKindAssumptionPin(
		mustCarrier(t, reference),
		mustEdition(t, edition),
		mustDigest(t, seed),
	)
	if err != nil {
		t.Fatalf("NewKindAssumptionPin(): %v", err)
	}
	return value
}

func mustObservableRef(t *testing.T, raw string) typedmemory.ObservableInputRef {
	t.Helper()
	value, err := typedmemory.NewObservableInputRef(raw)
	if err != nil {
		t.Fatalf("NewObservableInputRef(): %v", err)
	}
	return value
}

func mustRepair(t *testing.T, raw string) typedmemory.RepairPointer {
	t.Helper()
	value, err := typedmemory.NewRepairPointer(raw)
	if err != nil {
		t.Fatalf("NewRepairPointer(): %v", err)
	}
	return value
}

func mustCarrier(t *testing.T, raw string) typedmemory.CarrierRef {
	t.Helper()
	value, err := typedmemory.NewCarrierRef(raw)
	if err != nil {
		t.Fatalf("NewCarrierRef(): %v", err)
	}
	return value
}

func mustEdition(t *testing.T, raw string) typedmemory.CarrierEdition {
	t.Helper()
	value, err := typedmemory.NewCarrierEdition(raw)
	if err != nil {
		t.Fatalf("NewCarrierEdition(): %v", err)
	}
	return value
}

func mustSourceUnit(t *testing.T, raw string) typedmemory.SourceUnitID {
	t.Helper()
	value, err := typedmemory.NewSourceUnitID(raw)
	if err != nil {
		t.Fatalf("NewSourceUnitID(): %v", err)
	}
	return value
}

func mustSourceRevision(t *testing.T, raw string) typedmemory.SourceRevision {
	t.Helper()
	value, err := typedmemory.NewSourceRevision(raw)
	if err != nil {
		t.Fatalf("NewSourceRevision(): %v", err)
	}
	return value
}

func mustProvenanceRef(t *testing.T, raw string) typedmemory.ProvenanceRef {
	t.Helper()
	value, err := typedmemory.NewProvenanceRef(raw)
	if err != nil {
		t.Fatalf("NewProvenanceRef(): %v", err)
	}
	return value
}

func mustCompilerRule(t *testing.T, raw string) typedmemory.CompilerRuleID {
	t.Helper()
	value, err := typedmemory.NewCompilerRuleID(raw)
	if err != nil {
		t.Fatalf("NewCompilerRuleID(): %v", err)
	}
	return value
}
