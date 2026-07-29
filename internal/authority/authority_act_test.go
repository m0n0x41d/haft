package authority

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestAuthoritySpeechActObservesActualOperationOrder(t *testing.T) {
	intent, start := testPreparedAuthorityIntent(t)
	reviewText := "Authorize the exact profile declaration and no other action."
	reviewDigest, err := AuthorityIntentReviewDigest(intent, reviewText)
	if err != nil {
		t.Fatalf("AuthorityIntentReviewDigest: %v", err)
	}
	events := []string{}
	terminal := newAuthorityIssueTerminalFixture(
		"AUTHORIZE "+reviewDigest.String()+"\n",
		true,
	)
	terminal.events = &events
	times := []time.Time{start, start.Add(time.Millisecond), start.Add(2 * time.Millisecond)}
	index := 0
	clock := func() time.Time {
		labels := []string{"clock:start", "clock:exact-utterance", "clock:end"}
		events = append(events, labels[index])
		value := times[index]
		index++
		return value
	}
	act, err := captureVerifiedAuthorityAct(
		context.Background(),
		intent,
		reviewText,
		reviewDigest,
		terminal,
		clock,
		bytes.NewReader(bytes.Repeat([]byte{0x5a}, 16)),
	)
	if err != nil || !act.valid() {
		t.Fatalf("captureVerifiedAuthorityAct: %v", err)
	}
	want := []string{
		"clock:start",
		"tty:write",
		"tty:read",
		"clock:exact-utterance",
		"tty:observe-session",
		"clock:end",
	}
	if !slices.Equal(events, want) {
		t.Fatalf("authority event order = %v, want %v", events, want)
	}
	state := act.state
	if state.speechAct.state.window.from != times[0] ||
		state.capture.state.exactUtteranceObservedAt != times[1] ||
		state.speechAct.state.window.until != times[2] ||
		state.authorizer.state.assignmentWindow != state.speechAct.state.window {
		t.Fatal("SpeechAct Work and RoleAssignment do not bind the three observed boundaries")
	}
}

func TestVerifiedSpeechActRejectsUtteranceAtWorkBoundary(t *testing.T) {
	act := testVerifiedAuthorityAct(t)
	state := act.state.source.state
	for _, observedAt := range []time.Time{
		state.speechAct.state.window.from,
		state.speechAct.state.window.until,
	} {
		_, err := newVerifiedSpeechActSource(
			state.intent,
			state.capture.state.reviewText,
			state.reviewDigest,
			state.capture.state.canonicalUtterance,
			state.speechAct.state.window.from,
			observedAt,
			state.speechAct.state.window.until,
			state.capture.state.observation,
		)
		if err == nil || !strings.Contains(err.Error(), "actual ordered Work observations") {
			t.Fatalf("boundary utterance error = %v", err)
		}
	}
}

func TestProfileAuthorityActBindsScopeAdjudicationAndAssignmentProvenance(t *testing.T) {
	intent, now := testPreparedAuthorityIntent(t)
	reviewText := "Authorize the exact profile declaration and no other action."
	reviewDigest, err := AuthorityIntentReviewDigest(intent, reviewText)
	if err != nil {
		t.Fatalf("AuthorityIntentReviewDigest: %v", err)
	}
	terminal := newAuthorityIssueTerminalFixture(
		"AUTHORIZE "+reviewDigest.String()+"\n",
		true,
	)
	act, err := captureVerifiedAuthorityAct(
		context.Background(),
		intent,
		reviewText,
		reviewDigest,
		terminal,
		authorityWorkClock(now),
		bytes.NewReader(bytes.Repeat([]byte{0x5a}, 16)),
	)
	if err != nil {
		t.Fatalf("captureVerifiedAuthorityAct: %v", err)
	}
	if !act.valid() {
		t.Fatal("captured profile authority act is invalid")
	}
	state := act.state
	if state.authorizer.state.provenanceCarrierRef != state.capture.state.carrierRef ||
		state.authorizer.state.provenanceCarrierDigest != state.capture.state.carrierDigest {
		t.Fatal("authorizer assignment omitted exact terminal-capture provenance")
	}
	if state.authorizer.state.ref == state.permission.state.subjectRef {
		t.Fatal("terminal-session authorizer and ProfileAuthor subject collapsed")
	}
	if state.speechAct.state.methodDescriptionRef == state.permission.state.methodDescriptionRef {
		t.Fatal("manual authority-issue and profile-onboarding MethodDescriptions collapsed")
	}
	if len(state.speechAct.state.inputRefs) != 1 || len(state.speechAct.state.outputRefs) != 1 {
		t.Fatal("SpeechAct omitted explicit Work inputs or outputs")
	}
	permissionJSON := string(state.permission.state.canonicalJSON)
	wantBindings := []string{
		`"claim_scope_bounded_context_ref"`,
		`"adjudication_verification_policy_ref"`,
		`"adjudication_evidence_relation_ref"`,
		`"adjudication_carrier_expectation_ref"`,
		`"instituting_terminal_capture_carrier_digest"`,
	}
	for _, binding := range wantBindings {
		if !strings.Contains(permissionJSON, binding) {
			t.Fatalf("permission JSON omitted %s: %s", binding, permissionJSON)
		}
	}
	if !strings.Contains(
		string(intent.state.contextPolicy.state.canonicalJSON),
		profileDeclarationEffectRuleValue,
	) {
		t.Fatal("context policy omitted explicit Authorize-to-MAY institutional rule")
	}
}

func TestProfilePermissionAdjudicationMutationInvalidatesAct(t *testing.T) {
	act := testVerifiedAuthorityAct(t)
	act.state.permission.state.evidenceRelationRef = mustParse(
		t,
		NewVerificationEvidenceRelationRef,
		"evidence-relation:mutated",
	)
	if act.valid() {
		t.Fatal("mutated permission evidence relation remained valid")
	}

	act = testVerifiedAuthorityAct(t)
	act.state.permission.state.captureCarrierDigest = testDigest(t, '9')
	if act.valid() {
		t.Fatal("mutated permission carrier evidence remained valid")
	}

	act = testVerifiedAuthorityAct(t)
	act.state.permission.state.boundedContextRef = mustParse(
		t,
		NewBoundedContextRef,
		"bounded-context:mutated",
	)
	if act.valid() {
		t.Fatal("mutated permission ClaimScope remained valid")
	}
}

func TestGenericSpeechActIntentOwnsAcceptCarrierUtterance(t *testing.T) {
	profileIntent, now := testPreparedAuthorityIntent(t)
	profileSource := profileIntent.state.sourceIntent.state
	effectRule, err := NewInstitutionalEffectRule(
		mustParse(t, NewInstitutionalEffectRuleRef, "institution-rule:accept-migration-review:v1"),
		mustInstitutedObjectKind(t, "U.ReviewAdmission"),
		mustInstitutionalModality(t, "ADMITS"),
		mustParse(t, NewActionKind, "migration.review.accept"),
		AcceptReviewedCarrierUtteranceRule(),
		mustParse(t, NewUtteranceRef, "utterance:exact-terminal-accept"),
	)
	if err != nil {
		t.Fatalf("NewInstitutionalEffectRule: %v", err)
	}
	policy, err := NewSpeechActContextPolicy(
		mustParse(t, NewContextPolicyRef, "context-policy:migration-review:test"),
		profileSource.contextPolicy.state.boundedContext,
		profileSource.contextPolicy.state.recognizedActType,
		effectRule,
	)
	if err != nil {
		t.Fatalf("NewSpeechActContextPolicy: %v", err)
	}
	profileFrame := profileSource.executionFrame.state
	acceptFrame, err := NewSpeechActExecutionFrameBuilder(profileFrame.methodDescription).
		ExecutedWithin(profileFrame.executedWithin).
		OnStatePlane(profileFrame.statePlane, profileFrame.deltaPredicate).
		WithOutcome(profileFrame.outcome).
		WithUtteranceDescription(effectRule.utteranceDescription).
		BindParameter(profileFrame.parameters[0]).
		UseResource(profileFrame.resources[0]).
		Affect(profileFrame.affected[0]).
		Build()
	if err != nil {
		t.Fatalf("build ACCEPT execution frame: %v", err)
	}
	sourceIntent, err := NewPreparedSpeechActIntentBuilder(
		profileSource.speechActRef,
		profileSource.captureCarrierRef,
	).
		ForProject(profileSource.projectRoot).
		InSession(profileSource.sessionRef).
		Reviewing(profileSource.reviewSubjectRef, profileSource.reviewSubjectDig).
		Institutes(profileSource.institutedObject).
		UnderContextPolicy(policy).
		WithExecutionFrame(acceptFrame).
		Build()
	if err != nil {
		t.Fatalf("build migration SpeechAct intent: %v", err)
	}
	reviewText := "Accept the reviewed migration packet carrier."
	reviewDigest, err := SpeechActIntentReviewDigest(sourceIntent, reviewText)
	if err != nil {
		t.Fatalf("SpeechActIntentReviewDigest: %v", err)
	}
	prepared, err := PrepareManualSpeechAct(sourceIntent, reviewText)
	if err != nil {
		t.Fatalf("PrepareManualSpeechAct: %v", err)
	}
	want := "ACCEPT " + sourceIntent.state.reviewSubjectDig.String()
	terminal := newAuthorityIssueTerminalFixture(want+"\n", true)
	source, err := captureVerifiedSpeechAct(
		context.Background(),
		prepared,
		terminal,
		authorityWorkClock(now),
		bytes.NewReader(bytes.Repeat([]byte{0x2a}, 16)),
	)
	if err != nil {
		t.Fatalf("captureVerifiedSpeechAct: %v", err)
	}
	if !source.valid() || source.state.capture.state.canonicalUtterance != want {
		t.Fatal("generic source did not preserve intent-owned ACCEPT carrier utterance")
	}
	if strings.Contains(want, reviewDigest.String()) {
		t.Fatal("ACCEPT carrier rule accidentally bound the review digest")
	}
}

func testVerifiedAuthorityAct(t *testing.T) VerifiedAuthorityAct {
	t.Helper()
	intent, now := testPreparedAuthorityIntent(t)
	return captureTestVerifiedAuthorityAct(t, intent, now)
}

func testVerifiedAuthorityActAt(t *testing.T, now time.Time) VerifiedAuthorityAct {
	t.Helper()
	intent, canonicalNow := testPreparedAuthorityIntentAt(t, now)
	return captureTestVerifiedAuthorityAct(t, intent, canonicalNow)
}

func captureTestVerifiedAuthorityAct(
	t *testing.T,
	intent PreparedAuthorityIntent,
	now time.Time,
) VerifiedAuthorityAct {
	t.Helper()
	reviewText := "Authorize the exact profile declaration and no other action."
	reviewDigest, err := AuthorityIntentReviewDigest(intent, reviewText)
	if err != nil {
		t.Fatalf("AuthorityIntentReviewDigest: %v", err)
	}
	terminal := newAuthorityIssueTerminalFixture(
		"AUTHORIZE "+reviewDigest.String()+"\n",
		true,
	)
	act, err := captureVerifiedAuthorityAct(
		context.Background(),
		intent,
		reviewText,
		reviewDigest,
		terminal,
		authorityWorkClock(now),
		bytes.NewReader(bytes.Repeat([]byte{0x5a}, 16)),
	)
	if err != nil {
		t.Fatalf("captureVerifiedAuthorityAct: %v", err)
	}
	return act
}

func authorityWorkClock(start time.Time) func() time.Time {
	observations := []time.Time{
		start,
		start.Add(time.Millisecond),
		start.Add(2 * time.Millisecond),
	}
	index := 0
	return func() time.Time {
		if index >= len(observations) {
			panic("authority Work clock exhausted")
		}
		observed := observations[index]
		index++
		return observed
	}
}

func testPreparedAuthorityIntent(t *testing.T) (PreparedAuthorityIntent, time.Time) {
	t.Helper()
	now := canonicalAuthorityTime(time.Now())
	return testPreparedAuthorityIntentAt(t, now)
}

func testPreparedAuthorityIntentAt(
	t *testing.T,
	now time.Time,
) (PreparedAuthorityIntent, time.Time) {
	t.Helper()
	now = canonicalAuthorityTime(now)
	envelope := testAuthorizationEnvelope(t, now)
	content, err := NewProfileDeclarationAuthorizationContent(
		mustParse(t, NewAuthorizationContentRef, "authorization-content:profile:test"),
		envelope,
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationAuthorizationContent: %v", err)
	}
	policy, err := NewProfileDeclarationContextPolicy(
		mustParse(t, NewContextPolicyRef, "context-policy:profile-authority:test"),
		mustParse(t, NewBoundedContextRef, "bounded-context:haft-local-authority"),
		mustParse(t, NewSpeechActTypeRef, "speech-act-type:authorize"),
	)
	if err != nil {
		t.Fatalf("NewProfileDeclarationContextPolicy: %v", err)
	}
	frame, err := NewSpeechActExecutionFrameBuilder(
		ManualAuthorityIssueMethodDescription(),
	).
		ExecutedWithin(mustParse(t, NewSystemRef, "system:haft-kernel")).
		OnStatePlane(
			mustParse(t, NewStatePlaneRef, "state-plane:project-authority"),
			mustParse(t, NewDeltaPredicateRef, "delta-predicate:permission-instituted"),
		).
		WithOutcome(mustParse(t, NewWorkOutcomeRef, "work-outcome:permission-instituted")).
		WithUtteranceDescription(mustParse(t, NewUtteranceRef, "utterance:exact-terminal-authorize")).
		BindParameter(mustWorkParameter(t, "parameter:review-binding", "value:exact-digest")).
		UseResource(mustParse(t, NewWorkResourceRef, "resource:controlling-terminal")).
		Affect(mustParse(t, NewAffectedRef, "affected:project-profile-authority-state")).
		Build()
	if err != nil {
		t.Fatalf("build SpeechActExecutionFrame: %v", err)
	}
	resolutionWindow := mustWindow(t, now.Add(-time.Hour), now.Add(time.Hour))
	intent, err := NewPreparedAuthorityIntentBuilder(
		mustParse(t, NewPresentationID, "presentation.profile-authority-act"),
		mustParse(t, NewAuthorityResolutionID, "authority-resolution.profile-authority-act"),
		mustParse(t, NewSpeechActRef, "speech-act:profile-authority:test"),
		mustParse(t, NewPermissionRef, "permission:profile-declaration:test"),
		mustParse(t, NewCarrierRef, "carrier:terminal-capture:test"),
	).
		WithAuthorizationContent(content).
		InSpeechActSession(mustParse(t, NewSessionRef, "session:manual-authority:test")).
		UnderContextPolicy(policy).
		WithSpeechActExecutionFrame(frame).
		ScopedBy(mustParse(t, NewProfileAdmissionPredicateRef, "predicate:profile-admission:v1")).
		WithinClaimScope(mustParse(t, NewClaimScopeRef, "claim-scope:profile-declaration:test")).
		VerifiedBy(
			mustParse(t, NewVerifierIdentity, "verifier:authority-gate"),
			mustParse(t, NewVerifierVersion, "version:v1"),
		).
		UnderVerificationPolicy(
			mustParse(t, NewVerificationPolicyRef, "verification-policy:authority:v1"),
			testDigest(t, 'b'),
		).
		WithAdjudicationEvidence(
			mustParse(t, NewVerificationEvidenceRelationRef, "evidence-relation:verifies-permission-use"),
			mustParse(t, NewVerificationCarrierExpectationRef, "carrier-expectation:terminal-speech-act"),
		).
		ResolutionEffectiveWithin(resolutionWindow).
		Build()
	if err != nil {
		t.Fatalf("build PreparedAuthorityIntent: %v", err)
	}
	return intent, now
}

func mustWorkParameter(t *testing.T, name string, value string) WorkParameterBinding {
	t.Helper()
	result, err := NewWorkParameterBinding(name, value)
	if err != nil {
		t.Fatalf("NewWorkParameterBinding: %v", err)
	}
	return result
}

func mustInstitutedObjectKind(t *testing.T, raw string) InstitutedObjectKind {
	t.Helper()
	value, err := NewInstitutedObjectKind(raw)
	if err != nil {
		t.Fatalf("NewInstitutedObjectKind: %v", err)
	}
	return value
}

func mustInstitutionalModality(t *testing.T, raw string) InstitutionalModality {
	t.Helper()
	value, err := NewInstitutionalModality(raw)
	if err != nil {
		t.Fatalf("NewInstitutionalModality: %v", err)
	}
	return value
}
