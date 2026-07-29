package decisionbinding

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/authority"
)

func TestDecisionSpeechActPreparationUsesReadableLiteralAndPureManualSource(t *testing.T) {
	if CanonicalDecisionSpeechActPhrase != "DECIDE THIS REVIEWED CHOICE" {
		t.Fatalf("canonical phrase = %q", CanonicalDecisionSpeechActPhrase)
	}
	content := mustDecisionBindingContent(
		t,
		t.TempDir(),
		testDecisionRef,
		decisionInputFixture(),
	)
	intent, err := PrepareDecisionSpeechActIntent(content)
	if err != nil {
		t.Fatalf("PrepareDecisionSpeechActIntent: %v", err)
	}
	if err := ValidatePreparedDecisionSpeechActIntent(content, intent); err != nil {
		t.Fatalf("ValidatePreparedDecisionSpeechActIntent: %v", err)
	}
	card, err := content.ReviewCard()
	if err != nil {
		t.Fatalf("ReviewCard: %v", err)
	}
	review, reviewOK := card.Text()
	if !reviewOK {
		t.Fatal("review text is absent")
	}
	manualSource, err := authority.PrepareManualSpeechAct(intent, review)
	if err != nil {
		t.Fatalf("PrepareManualSpeechAct compatibility: %v", err)
	}
	preparedIntent, intentOK := manualSource.Intent()
	preparedReview, reviewOK := manualSource.ReviewText()
	if !intentOK || !reviewOK {
		t.Fatal("generic manual source rejected decision-owned preparation")
	}
	wantDigest, _ := intent.Digest()
	gotDigest, _ := preparedIntent.Digest()
	if gotDigest.String() != wantDigest.String() || preparedReview != review {
		t.Fatal("generic manual source changed decision-owned intent or review")
	}
}

func TestDecisionSpeechActIntentIsDeterministicAndContentBound(t *testing.T) {
	root := t.TempDir()
	content := mustDecisionBindingContent(
		t,
		root,
		testDecisionRef,
		decisionInputFixture(),
	)
	first, err := PrepareDecisionSpeechActIntent(content)
	if err != nil {
		t.Fatalf("first intent: %v", err)
	}
	second, err := PrepareDecisionSpeechActIntent(content)
	if err != nil {
		t.Fatalf("second intent: %v", err)
	}
	firstDigest, firstOK := first.Digest()
	secondDigest, secondOK := second.Digest()
	if !firstOK || !secondOK || firstDigest.String() != secondDigest.String() {
		t.Fatal("same decision content produced unstable SpeechAct intent")
	}

	changedInput := decisionInputFixture()
	changedInput.SelectedTitle = "Use an explicit typed decision effect"
	changedInput.ChoiceResult.VariantRef = changedInput.SelectedTitle
	changedInput.ChoiceResult.OptionSet[0] = changedInput.SelectedTitle
	changedContent := mustDecisionBindingContent(
		t,
		root,
		"dec-20260715-typed-binding-b2c3d4e5",
		changedInput,
	)
	if err := ValidatePreparedDecisionSpeechActIntent(changedContent, first); err == nil {
		t.Fatal("intent for one decision content cross-bound to another")
	}
}

func TestDecisionContextPolicyRejectsProfileAndMigrationCrossBinding(t *testing.T) {
	decisionPolicy, err := DecisionSpeechActContextPolicy()
	if err != nil {
		t.Fatalf("DecisionSpeechActContextPolicy: %v", err)
	}
	if err := ValidateDecisionSpeechActContextPolicy(decisionPolicy); err != nil {
		t.Fatalf("decision policy rejected itself: %v", err)
	}
	profilePolicy := profileContextPolicyFixture(t)
	migrationPolicy := migrationContextPolicyFixture(t)

	foreign := map[string]authority.SpeechActContextPolicy{
		"profile declaration":        profilePolicy,
		"migration review admission": migrationPolicy,
	}
	for name, policy := range foreign {
		t.Run(name, func(t *testing.T) {
			if err := ValidateDecisionSpeechActContextPolicy(policy); err == nil {
				t.Fatal("foreign context policy cross-bound as a decision policy")
			}
		})
	}
}

func TestDecisionMethodAndPolicyShareBoundedContext(t *testing.T) {
	policy, err := DecisionSpeechActContextPolicy()
	if err != nil {
		t.Fatalf("DecisionSpeechActContextPolicy: %v", err)
	}
	policyContext, contextOK := policy.BoundedContext()
	if !contextOK || policyContext.String() != decisionBoundedContextValue {
		t.Fatalf("policy context = %q", policyContext.String())
	}
	method, err := decisionSpeechActMethodDescription()
	if err != nil {
		t.Fatalf("decisionSpeechActMethodDescription: %v", err)
	}
	methodRef, methodRefOK := method.MethodRef()
	methodDigest, methodDigestOK := method.Digest()
	if !methodRefOK || !methodDigestOK || methodRef.String() != decisionMethodValue {
		t.Fatal("decision method identity is incomplete")
	}

	expected := manualMethodDescriptionFixture(t, policyContext)
	expectedDigest, expectedDigestOK := expected.Digest()
	if !expectedDigestOK || expectedDigest.String() != methodDigest.String() {
		t.Fatal("decision method was not built in the policy's bounded context")
	}
	foreignContext := mustBoundedContextRef(t, "bounded-context:haft-local-authority")
	foreign := manualMethodDescriptionFixture(t, foreignContext)
	foreignDigest, foreignDigestOK := foreign.Digest()
	if !foreignDigestOK || foreignDigest.String() == methodDigest.String() {
		t.Fatal("bounded-context mismatch did not change MethodDescription identity")
	}
}

func profileContextPolicyFixture(t *testing.T) authority.SpeechActContextPolicy {
	t.Helper()
	policyRef := mustContextPolicyRef(t, "context-policy:profile-foreign:v1")
	boundedContext := mustBoundedContextRef(t, "bounded-context:haft-profile-foreign")
	actType := mustSpeechActTypeRef(t, "speech-act-type:authorize")
	policy, err := authority.NewProfileDeclarationContextPolicy(
		policyRef,
		boundedContext,
		actType,
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationContextPolicy: %v", err)
	}
	return policy
}

func migrationContextPolicyFixture(t *testing.T) authority.SpeechActContextPolicy {
	t.Helper()
	policyRef := mustContextPolicyRef(t, "context-policy:migration-review-foreign:v1")
	boundedContext := mustBoundedContextRef(t, "bounded-context:haft-spec-migration-v2")
	actType := mustSpeechActTypeRef(t, "speech-act-type:accept")
	ruleRef, err := authority.NewInstitutionalEffectRuleRef(
		"institution-rule:accept-institutes-migration-review-admission:v1",
	)
	if err != nil {
		t.Fatalf("NewInstitutionalEffectRuleRef: %v", err)
	}
	objectKind, err := authority.NewInstitutedObjectKind("haft.MigrationReviewAdmission")
	if err != nil {
		t.Fatalf("NewInstitutedObjectKind: %v", err)
	}
	modality, err := authority.NewInstitutionalModality("ADMITTED")
	if err != nil {
		t.Fatalf("NewInstitutionalModality: %v", err)
	}
	action, err := authority.NewActionKind("spec-migration-v2.review.admit")
	if err != nil {
		t.Fatalf("NewActionKind: %v", err)
	}
	utterance, err := authority.NewUtteranceRef(
		"utterance:exact-migration-review-acceptance",
	)
	if err != nil {
		t.Fatalf("NewUtteranceRef: %v", err)
	}
	rule, err := authority.NewInstitutionalEffectRule(
		ruleRef,
		objectKind,
		modality,
		action,
		authority.AcceptReviewedCarrierUtteranceRule(),
		utterance,
	)
	if err != nil {
		t.Fatalf("NewInstitutionalEffectRule: %v", err)
	}
	policy, err := authority.NewSpeechActContextPolicy(
		policyRef,
		boundedContext,
		actType,
		rule,
	)
	if err != nil {
		t.Fatalf("NewSpeechActContextPolicy: %v", err)
	}
	return policy
}

func manualMethodDescriptionFixture(
	t *testing.T,
	boundedContext authority.BoundedContextRef,
) authority.SpeechActMethodDescription {
	t.Helper()
	methodRef, err := authority.NewMethodRef(decisionMethodValue)
	if err != nil {
		t.Fatalf("NewMethodRef: %v", err)
	}
	descriptionRef, err := authority.NewMethodDescriptionRef(decisionMethodDescValue)
	if err != nil {
		t.Fatalf("NewMethodDescriptionRef: %v", err)
	}
	procedureRef, err := authority.NewMethodProcedureRef(decisionMethodProcedure)
	if err != nil {
		t.Fatalf("NewMethodProcedureRef: %v", err)
	}
	method, err := authority.NewManualControllingTTYMethodDescription(
		methodRef,
		descriptionRef,
		procedureRef,
		boundedContext,
	)
	if err != nil {
		t.Fatalf("NewManualControllingTTYMethodDescription: %v", err)
	}
	return method
}

func mustContextPolicyRef(t *testing.T, raw string) authority.ContextPolicyRef {
	t.Helper()
	value, err := authority.NewContextPolicyRef(raw)
	if err != nil {
		t.Fatalf("NewContextPolicyRef: %v", err)
	}
	return value
}

func mustBoundedContextRef(t *testing.T, raw string) authority.BoundedContextRef {
	t.Helper()
	value, err := authority.NewBoundedContextRef(raw)
	if err != nil {
		t.Fatalf("NewBoundedContextRef: %v", err)
	}
	return value
}

func mustSpeechActTypeRef(t *testing.T, raw string) authority.SpeechActTypeRef {
	t.Helper()
	value, err := authority.NewSpeechActTypeRef(raw)
	if err != nil {
		t.Fatalf("NewSpeechActTypeRef: %v", err)
	}
	return value
}
