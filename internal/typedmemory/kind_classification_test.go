package typedmemory

import (
	"bytes"
	"testing"
	"time"
)

func TestKindClassificationRequestHasFourExactInputsAndNoEntitySet(t *testing.T) {
	fixture := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, true)
	request := fixture.request

	if request.Candidate().Digest() != fixture.candidate.Digest() ||
		request.LocalKind() != fixture.localKind ||
		request.SignatureEdition() != fixture.signature.Ref() ||
		request.ContextSlice().Ref() != fixture.contextSlice.Ref() {
		t.Fatal("classification request did not retain its exact four inputs")
	}
	if fixture.signature.CandidateValueKind() != fixture.environment.entityValueKind {
		t.Fatal("KindSignature lost its distinct candidate ValueKind")
	}
	if fixture.signature.Criterion().String() == "" ||
		fixture.signature.SliceConditions().String() == "" ||
		fixture.signature.ReferenceScheme().Edition().String() == "" {
		t.Fatal("KindSignature lost a current C.3 declaration coordinate")
	}

	otherCandidate := mustKindClassificationValue(NewExactKindEntityCandidate(
		mustKindEntityID(t, "entity:vehicle-8"),
		fixture.environment.entityValueKind,
	))
	otherRequest := mustKindClassificationValue(NewKindClassificationRequest(
		KindClassificationRequestInput{
			Candidate:        otherCandidate,
			LocalKind:        fixture.localKind,
			SignatureEdition: fixture.signature.Ref(),
			ContextSlice:     fixture.contextSlice,
		},
	))
	if otherRequest.Digest() == request.Digest() {
		t.Fatal("changing the exact candidate did not change request identity")
	}

	laterSlice := memberOfTestContextSlice(
		t,
		fixture.localKind.Context(),
		time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC),
	)
	laterRequest := mustKindClassificationValue(NewKindClassificationRequest(
		KindClassificationRequestInput{
			Candidate:        fixture.candidate,
			LocalKind:        fixture.localKind,
			SignatureEdition: fixture.signature.Ref(),
			ContextSlice:     laterSlice,
		},
	))
	if laterRequest.Digest() == request.Digest() {
		t.Fatal("changing the exact ContextSlice did not change request identity")
	}

	otherSignature := fixture.signatureWithCriterion(t, "rule:kind/criterion/v2")
	otherSignatureRequest := mustKindClassificationValue(NewKindClassificationRequest(
		KindClassificationRequestInput{
			Candidate:        fixture.candidate,
			LocalKind:        fixture.localKind,
			SignatureEdition: otherSignature.Ref(),
			ContextSlice:     fixture.contextSlice,
		},
	))
	if otherSignatureRequest.Digest() == request.Digest() {
		t.Fatal("changing the KindSignature edition did not change request identity")
	}

	otherLocal := newKindClassificationFixture(t, "ctx:haft", "U.Entity", SignatureF4, true)
	if otherLocal.request.Digest() == request.Digest() {
		t.Fatal("changing the local kind did not change request identity")
	}
}

func TestGovernedFeatureSetCanonicalizesByKeyAndKeepsEveryLookupReachable(t *testing.T) {
	t.Parallel()

	fixture := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, true)
	vehicle := fixture.features.Features()[0]
	identity := mustKindClassificationValue(NewGovernedCandidateFeature(
		GovernedCandidateFeatureInput{
			Key:          mustKindFeatureKey(t, "entity.identity-present"),
			Value:        vehicle.Value(),
			Governor:     mustKindRuleRef(t, "rule:entity/identity-present/v1"),
			Source:       mustKindCarrierRef(t, "visibility:entity/vehicle-7"),
			SourceDigest: mustKindDigest(t, 0x63),
		},
	))
	forward := mustKindClassificationValue(NewGovernedCandidateFeatureSet(
		fixture.request,
		[]GovernedCandidateFeature{vehicle, identity},
	))
	reverse := mustKindClassificationValue(NewGovernedCandidateFeatureSet(
		fixture.request,
		[]GovernedCandidateFeature{identity, vehicle},
	))
	if forward.Digest() != reverse.Digest() ||
		!bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("governed feature permutation changed canonical set identity")
	}
	for _, expected := range []GovernedCandidateFeature{identity, vehicle} {
		actual, found := forward.Feature(expected.Key())
		if !found || !bytes.Equal(actual.CanonicalBytes(), expected.CanonicalBytes()) {
			t.Fatalf("Feature(%q) did not recover the exact governed value", expected.Key().String())
		}
	}
}

func TestTypeEnvKeepsCurrentKindSignaturesSeparateFromLegacyMembership(t *testing.T) {
	fixture := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, true)
	environment := mustKindClassificationValue(
		fixture.environment.builderWithoutBridge().
			AddKindClassificationSignatureDefinition(fixture.signature).
			Build(),
	)

	definitions := environment.KindClassificationSignatureDefinitions()
	if len(definitions) != 1 || definitions[0].Ref() != fixture.signature.Ref() {
		t.Fatal("TypeEnv lost the exact current KindSignature edition")
	}
	if len(environment.EntitySetDefinitions()) != 0 ||
		len(environment.KindSignatureDefinitions()) != 0 {
		t.Fatal("current KindSignature populated the sealed MemberOf/EntitySet collections")
	}
	retained, found := environment.KindClassificationSignatureDefinition(fixture.localKind)
	if !found || retained.Ref() != fixture.signature.Ref() {
		t.Fatal("current KindSignature lookup is not keyed by the exact local kind")
	}

	_, err := fixture.environment.builderWithoutBridge().
		AddKindClassificationSignatureDefinition(fixture.signature).
		AddKindClassificationSignatureDefinition(fixture.signature).
		Build()
	if err == nil {
		t.Fatal("TypeEnv admitted two current KindSignature editions for one local kind")
	}
}

func TestKindClassificationKeepsTrueFalseUnknownEvidenceAndGuardSeparate(t *testing.T) {
	fixture := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, true)
	trueJudgement := mustKindClassificationValue(NewTrueKindClassification(
		fixture.request,
		fixture.basis,
	))
	falseJudgement := mustKindClassificationValue(NewFalseKindClassification(
		fixture.request,
		fixture.basis,
	))
	unknownReason := mustKindClassificationValue(NewKindClassificationUnknownReason(
		KindUnknownDependencyUnavailable,
		mustKindRepairPointer(t, "repair:dependency/standard-registry"),
	))
	unknownJudgement := mustKindClassificationValue(NewUnknownKindClassification(
		fixture.request,
		[]KindClassificationUnknownReason{unknownReason},
	))

	if trueJudgement.Kind() != KindClassificationTrue ||
		falseJudgement.Kind() != KindClassificationFalse ||
		unknownJudgement.Kind() != KindClassificationUnknown {
		t.Fatal("classification judgement algebra lost true, false, or unknown")
	}
	if falseJudgement.Digest() == unknownJudgement.Digest() ||
		bytes.Equal(falseJudgement.CanonicalBytes(), unknownJudgement.CanonicalBytes()) {
		t.Fatal("unknown was encoded as false")
	}
	if _, err := NewTrueKindClassification(fixture.request, KindClassificationEvaluationBasis{}); err == nil {
		t.Fatal("true classification was constructible without direct governed features")
	}

	assertion := mustKindClassificationValue(NewKindClassificationAssertionPin(
		mustKindCarrierRef(t, "episteme:classification/vehicle-7"),
		mustKindCarrierEdition(t, "1"),
		mustKindDigest(t, 0x71),
	))
	evidence := mustKindClassificationValue(NewKindClassificationEvidencePin(
		mustKindCarrierRef(t, "evidence:inspection/vehicle-7"),
		mustKindCarrierEdition(t, "2026-07-22"),
		mustKindDigest(t, 0x72),
	))
	support := mustKindClassificationValue(NewKindClassificationSupport(
		unknownJudgement,
		assertion,
		[]KindClassificationEvidencePin{evidence},
	))
	if support.JudgementDigest() != unknownJudgement.Digest() ||
		unknownJudgement.Kind() != KindClassificationUnknown {
		t.Fatal("supporting Evidence changed the classification judgement")
	}

	disposition := mustKindClassificationValue(EvaluateFailClosedKindGuard(
		unknownJudgement,
		KindGuardScopeCovered,
	))
	if disposition.Kind() != KindGuardRefused ||
		disposition.ClassificationKind() != KindClassificationUnknown {
		t.Fatal("fail-closed guard rewrote unknown instead of preserving it")
	}
	allowed := mustKindClassificationValue(EvaluateFailClosedKindGuard(
		trueJudgement,
		KindGuardScopeCovered,
	))
	if allowed.Kind() != KindGuardAllowed || allowed.ClassificationKind() != KindClassificationTrue {
		t.Fatal("true plus covered scope did not produce a separate allow disposition")
	}
}

func TestKindExtensionIsNamedOptionalProjectionOfTrueJudgementsOnly(t *testing.T) {
	fixture := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, true)
	trueJudgement := mustKindClassificationValue(NewTrueKindClassification(
		fixture.request,
		fixture.basis,
	))
	use := mustKindClassificationValue(NewKindExtensionReceivingUseRef(
		"receiving-use:relation-slot-admission",
	))
	projection := mustKindClassificationValue(NewKindExtensionProjection(
		KindExtensionProjectionInput{
			Signature:      fixture.signature,
			ContextSlice:   fixture.contextSlice,
			ReceivingUse:   use,
			TrueJudgements: []TrueKindClassification{trueJudgement},
		},
	))
	if projection.ReceivingUse() != use ||
		len(projection.TrueJudgements()) != 1 ||
		projection.TrueJudgements()[0].Request().Candidate().Digest() != fixture.candidate.Digest() {
		t.Fatal("KindExtension lost its named use or exact true candidate")
	}

	withoutExtent := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, false)
	_, err := NewKindExtensionProjection(KindExtensionProjectionInput{
		Signature:      withoutExtent.signature,
		ContextSlice:   withoutExtent.contextSlice,
		ReceivingUse:   use,
		TrueJudgements: nil,
	})
	if err == nil {
		t.Fatal("KindExtension materialized without an explicit ExtentRule")
	}
}

func TestCurrentSubkindOrderUsesObtainingFactsAndSeparateAssertion(t *testing.T) {
	fixture := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, true)
	entityKind := mustKindClassificationValue(NewLocalKindRef(
		fixture.environment.entityValueKind,
		fixture.localKind.Context(),
	))
	kinds := []LocalKindRef{fixture.localKind, entityKind}
	facts := []SubkindOfObtainingFact{
		fixture.subkindFact(t, fixture.localKind, fixture.localKind),
		fixture.subkindFact(t, entityKind, entityKind),
		fixture.subkindFact(t, fixture.localKind, entityKind),
	}
	assessment := mustKindClassificationValue(AssessSubkindOrder(kinds, facts))
	if !assessment.Valid() || len(assessment.Violations()) != 0 {
		t.Fatalf("valid explicit subkind order was rejected: %#v", assessment.Violations())
	}
	assertion := mustKindClassificationValue(NewKindClassificationAssertionPin(
		mustKindCarrierRef(t, "episteme:subkind/system-entity"),
		mustKindCarrierEdition(t, "1"),
		mustKindDigest(t, 0x73),
	))
	link := mustKindClassificationValue(NewSubkindOfAssertionLink(facts[2], assertion))
	if link.FactDigest() != facts[2].Digest() || link.Digest() == facts[2].Digest() {
		t.Fatal("subkind assertion was collapsed into the obtaining fact")
	}

	thirdKindID := typeEnvTestKindID(t, "U.ThirdKind")
	thirdKind := mustKindClassificationValue(NewLocalKindRef(
		typeEnvTestValueKindRef(t, fixture.environment.ref, thirdKindID),
		fixture.localKind.Context(),
	))
	incomplete := []SubkindOfObtainingFact{
		fixture.subkindFact(t, fixture.localKind, fixture.localKind),
		fixture.subkindFact(t, entityKind, entityKind),
		fixture.subkindFact(t, thirdKind, thirdKind),
		fixture.subkindFact(t, fixture.localKind, entityKind),
		fixture.subkindFact(t, entityKind, thirdKind),
	}
	invalid := mustKindClassificationValue(AssessSubkindOrder(
		[]LocalKindRef{fixture.localKind, entityKind, thirdKind},
		incomplete,
	))
	if invalid.Valid() || !containsSubkindViolation(invalid, SubkindOrderMissingTransitiveFact) {
		t.Fatal("missing explicit transitive obtaining fact was not reported")
	}
}

func TestKindBridgeCreatesFreshTargetRequestWithoutSourceTruth(t *testing.T) {
	source := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, true)
	target := newKindClassificationFixture(t, "ctx:release", "U.System", SignatureF5, true)
	bridge := mustKindClassificationValue(NewKindBridge(KindBridgeInput{
		SourceKind:            source.localKind,
		TargetKind:            target.localKind,
		SourceReferenceScheme: source.referenceScheme,
		TargetReferenceScheme: target.referenceScheme,
		Direction:             KindBridgeForwardOnly,
		DefinednessRule:       mustKindRuleRef(t, "rule:kind-bridge/definedness/v1"),
		Provenance:            source.environment.provenance,
	}))
	targetRequest := mustKindClassificationValue(NewBridgedKindClassificationRequest(
		BridgedKindClassificationRequestInput{
			Bridge:          bridge,
			SourceRequest:   source.request,
			SourceSignature: source.signature,
			TargetSignature: target.signature,
			TargetSlice:     target.contextSlice,
		},
	))
	if targetRequest.LocalKind() != target.localKind ||
		targetRequest.SignatureEdition() != target.signature.Ref() ||
		targetRequest.Candidate().Digest() != source.candidate.Digest() {
		t.Fatal("KindBridge did not produce a fresh exact target-side request")
	}
	if targetRequest.Digest() == source.request.Digest() {
		t.Fatal("KindBridge reused source request identity as target truth")
	}
}

func TestRoleMaskKeepsFeatureJudgementAndScopeExpectationSeparate(t *testing.T) {
	fixture := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, true)
	mask := mustKindClassificationValue(NewRoleMaskDefinition(RoleMaskDefinitionInput{
		BaseSignature:    fixture.signature.Ref(),
		FeatureCriterion: mustKindRuleRef(t, "rule:role-mask/operator/v1"),
		ScopeExpectation: mustKindRuleRef(t, "rule:scope/operator/v1"),
		Provenance:       fixture.environment.provenance,
	}))
	request := mustKindClassificationValue(NewRoleMaskClassificationRequest(
		fixture.request,
		mask.Ref(),
	))
	basis := mustKindClassificationValue(NewRoleMaskEvaluationBasis(
		request,
		mask,
		fixture.features,
	))
	trueResult := mustKindClassificationValue(NewTrueRoleMaskJudgement(request, basis))
	falseResult := mustKindClassificationValue(NewFalseRoleMaskJudgement(request, basis))
	reason := mustKindClassificationValue(NewKindClassificationUnknownReason(
		KindUnknownMissingGovernedFeature,
		mustKindRepairPointer(t, "repair:feature/operator-role"),
	))
	unknownResult := mustKindClassificationValue(NewUnknownRoleMaskJudgement(
		request,
		[]KindClassificationUnknownReason{reason},
	))
	if trueResult.Kind() != KindClassificationTrue ||
		falseResult.Kind() != KindClassificationFalse ||
		unknownResult.Kind() != KindClassificationUnknown {
		t.Fatal("RoleMask did not retain its three-valued masked judgement")
	}
	if mask.FeatureCriterion() == mask.ScopeExpectation() {
		t.Fatal("RoleMask collapsed direct feature criterion into scope expectation")
	}
}

func TestKindClassificationBasisRejectsInternalTampering(t *testing.T) {
	fixture := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, true)

	tamperedBasis := fixture.basis
	tamperedBasis.canonicalBytes = append([]byte(nil), fixture.basis.canonicalBytes...)
	tamperedBasis.canonicalBytes[len(tamperedBasis.canonicalBytes)-1] ^= 0xff
	if tamperedBasis.validFor(fixture.request) {
		t.Fatal("classification basis accepted canonical-byte tampering")
	}
	if _, err := NewTrueKindClassification(fixture.request, tamperedBasis); err == nil {
		t.Fatal("settled classification accepted a tampered evaluation basis")
	}
}

func TestRoleMaskBasisRejectsInternalTampering(t *testing.T) {
	fixture := newKindClassificationFixture(t, "ctx:haft", "U.System", SignatureF4, true)
	mask := mustKindClassificationValue(NewRoleMaskDefinition(RoleMaskDefinitionInput{
		BaseSignature:    fixture.signature.Ref(),
		FeatureCriterion: mustKindRuleRef(t, "rule:role-mask/tamper-test/v1"),
		ScopeExpectation: mustKindRuleRef(t, "rule:scope/tamper-test/v1"),
		Provenance:       fixture.environment.provenance,
	}))
	request := mustKindClassificationValue(NewRoleMaskClassificationRequest(
		fixture.request,
		mask.Ref(),
	))
	basis := mustKindClassificationValue(NewRoleMaskEvaluationBasis(
		request,
		mask,
		fixture.features,
	))

	tamperedBasis := basis
	tamperedBasis.digest = mustKindDigest(t, 0x77)
	if tamperedBasis.validFor(request) {
		t.Fatal("RoleMask basis accepted digest tampering")
	}
}

type kindClassificationFixture struct {
	environment     typeEnvFixture
	localKind       LocalKindRef
	candidate       ExactKindEntityCandidate
	contextSlice    ContextSlice
	referenceScheme KindReferenceSchemePin
	signature       KindClassificationSignatureDefinition
	request         KindClassificationRequest
	features        GovernedCandidateFeatureSet
	basis           KindClassificationEvaluationBasis
}

func newKindClassificationFixture(
	t *testing.T,
	contextID string,
	localKindID string,
	formality SignatureFormality,
	withExtent bool,
) kindClassificationFixture {
	t.Helper()
	environment := newTypeEnvFixture(t)
	context := environment.primaryContext.Ref()
	if contextID == environment.secondaryContext.Ref().String() {
		context = environment.secondaryContext.Ref()
	}
	localKindValue := typeEnvTestValueKindRef(
		t,
		environment.ref,
		typeEnvTestKindID(t, localKindID),
	)
	localKind := mustKindClassificationValue(NewLocalKindRef(localKindValue, context))
	candidate := mustKindClassificationValue(NewExactKindEntityCandidate(
		mustKindEntityID(t, "entity:vehicle-7"),
		environment.entityValueKind,
	))
	contextSlice := memberOfTestContextSlice(
		t,
		context,
		time.Date(2026, time.July, 22, 10, 0, 0, 0, time.UTC),
	)
	referenceScheme := mustKindClassificationValue(NewKindReferenceSchemePin(
		mustKindCarrierRef(t, "reference-scheme:"+contextID),
		mustKindCarrierEdition(t, "2026.07"),
		mustKindDigest(t, byte(len(contextID)+40)),
	))
	extentRule := KindExtentRuleOption(NoKindExtentRule{})
	if withExtent {
		extentRule = mustKindClassificationValue(NewDeclaredKindExtentRule(
			mustKindRuleRef(t, "rule:kind/extent/v1"),
		))
	}
	signature := mustKindClassificationValue(NewKindClassificationSignatureDefinition(
		KindClassificationSignatureDefinitionInput{
			LocalKind:          localKind,
			CandidateValueKind: environment.entityValueKind,
			Criterion:          mustKindRuleRef(t, "rule:kind/criterion/v1"),
			SliceConditions:    mustKindRuleRef(t, "rule:kind/slice/v1"),
			ReferenceScheme:    referenceScheme,
			Dependencies: []KindSignatureDependencyPin{
				mustKindClassificationValue(NewKindSignatureDependencyPin(
					KindDependencyStandard,
					mustKindCarrierRef(t, "standard:vehicle-registry"),
					mustKindCarrierEdition(t, "2.3"),
					mustKindDigest(t, 0x61),
				)),
			},
			Formality:  formality,
			ExtentRule: extentRule,
			Provenance: environment.provenance,
		},
	))
	request := mustKindClassificationValue(NewKindClassificationRequest(
		KindClassificationRequestInput{
			Candidate:        candidate,
			LocalKind:        localKind,
			SignatureEdition: signature.Ref(),
			ContextSlice:     contextSlice,
		},
	))
	featureValue := kindClassificationVerifiedValue(t, environment.claimGraphValueKind)
	feature := mustKindClassificationValue(NewGovernedCandidateFeature(
		GovernedCandidateFeatureInput{
			Key:          mustKindFeatureKey(t, "vehicle.registry-status"),
			Value:        featureValue,
			Governor:     mustKindRuleRef(t, "rule:vehicle/registry-status/v1"),
			Source:       mustKindCarrierRef(t, "record:vehicle/vehicle-7"),
			SourceDigest: mustKindDigest(t, 0x62),
		},
	))
	features := mustKindClassificationValue(NewGovernedCandidateFeatureSet(
		request,
		[]GovernedCandidateFeature{feature},
	))
	basis := mustKindClassificationValue(NewKindClassificationEvaluationBasis(
		request,
		signature,
		features,
	))
	return kindClassificationFixture{
		environment:     environment,
		localKind:       localKind,
		candidate:       candidate,
		contextSlice:    contextSlice,
		referenceScheme: referenceScheme,
		signature:       signature,
		request:         request,
		features:        features,
		basis:           basis,
	}
}

func (fixture kindClassificationFixture) signatureWithCriterion(
	t *testing.T,
	rule string,
) KindClassificationSignatureDefinition {
	t.Helper()
	return mustKindClassificationValue(NewKindClassificationSignatureDefinition(
		KindClassificationSignatureDefinitionInput{
			LocalKind:          fixture.signature.LocalKind(),
			CandidateValueKind: fixture.signature.CandidateValueKind(),
			Criterion:          mustKindRuleRef(t, rule),
			SliceConditions:    fixture.signature.SliceConditions(),
			ReferenceScheme:    fixture.signature.ReferenceScheme(),
			Dependencies:       fixture.signature.Dependencies(),
			Formality:          fixture.signature.Formality(),
			ExtentRule:         fixture.signature.ExtentRule(),
			Provenance:         fixture.signature.Provenance(),
		},
	))
}

func (fixture kindClassificationFixture) subkindFact(
	t *testing.T,
	narrower LocalKindRef,
	broader LocalKindRef,
) SubkindOfObtainingFact {
	t.Helper()
	request := mustKindClassificationValue(NewSubkindOfRequest(
		narrower,
		broader,
		fixture.referenceScheme,
	))
	return mustKindClassificationValue(NewSubkindOfObtainingFact(
		request,
		mustKindRuleRef(t, "rule:subkind/obtains/v1"),
	))
}

func kindClassificationVerifiedValue(
	t *testing.T,
	valueKind ValueKindRef,
) VerifiedTypedValue {
	t.Helper()
	shape := typeEnvTestShapeRef(t, "KindFeatureShape", 0x63)
	codec := typeEnvTestCodecRef(t, "KindFeatureCodec", 0x64)
	canonical := []byte("registered")
	return verifiedTypedValue{
		valueKind:      valueKind,
		valueShape:     shape,
		codec:          codec,
		canonicalBytes: canonical,
		digest:         digestTypedValue(valueKind, shape, codec, canonical),
	}
}

func containsSubkindViolation(
	assessment SubkindOrderAssessment,
	kind SubkindOrderViolationKind,
) bool {
	for _, violation := range assessment.Violations() {
		if violation.Kind() == kind {
			return true
		}
	}
	return false
}

func mustKindEntityID(t *testing.T, raw string) EntityID {
	t.Helper()
	return mustKindClassificationValue(NewEntityID(raw))
}

func mustKindRuleRef(t *testing.T, raw string) RuleRef {
	t.Helper()
	return mustKindClassificationValue(NewRuleRef(raw))
}

func mustKindCarrierRef(t *testing.T, raw string) CarrierRef {
	t.Helper()
	return mustKindClassificationValue(NewCarrierRef(raw))
}

func mustKindCarrierEdition(t *testing.T, raw string) CarrierEdition {
	t.Helper()
	return mustKindClassificationValue(NewCarrierEdition(raw))
}

func mustKindDigest(t *testing.T, fill byte) SHA256Digest {
	t.Helper()
	return typeEnvTestDigest(t, fill)
}

func mustKindRepairPointer(t *testing.T, raw string) RepairPointer {
	t.Helper()
	return mustKindClassificationValue(NewRepairPointer(raw))
}

func mustKindFeatureKey(t *testing.T, raw string) KindFeatureKey {
	t.Helper()
	return mustKindClassificationValue(NewKindFeatureKey(raw))
}

func mustKindClassificationValue[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
