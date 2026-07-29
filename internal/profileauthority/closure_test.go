package profileauthority

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/authority"
)

func TestClosureKeepsFourRefsAndFPFTypedAdjudicationSeparate(t *testing.T) {
	prepared := testPreparedAuthorization(t, testPreparedInput{
		root:          "/tmp/haft-profile-authority-exact",
		profileAuthor: "role-assignment:profile-author:exact",
		identity:      "exact",
	})
	preparedBytes, ok := prepared.CanonicalBytes()
	if !ok {
		t.Fatal("prepared authorization omitted canonical bytes")
	}
	preparedDTO := preparedAuthorizationJSONV1{}
	if err := json.Unmarshal(preparedBytes, &preparedDTO); err != nil {
		t.Fatalf("decode prepared authorization: %v", err)
	}
	if preparedDTO.EnactabilityPredicateRef == "" {
		t.Fatalf("prepared enactability predicate = %#v", preparedDTO)
	}
	window := prepared.state.content.state.authorizationValidity
	recipe := exactSourceRecipeForPrepared(
		t,
		prepared,
		window.From().Add(time.Minute),
		window.From().Add(2*time.Minute),
		window.From().Add(3*time.Minute),
	)
	source := recordSourceFromRecipe(t, prepared, recipe)

	closure, err := NewClosure(prepared, source)
	if err != nil {
		t.Fatalf("NewClosure: %v", err)
	}
	permission, ok := closure.Permission()
	if !ok {
		t.Fatal("closure omitted permission")
	}
	evidence, ok := permission.EvidenceClaims()
	if !ok || len(evidence) != 1 || !strings.HasPrefix(evidence[0].String(), "E-") {
		t.Fatalf("evidence claims = %#v", evidence)
	}
	carrierClasses, ok := permission.CarrierClasses()
	if !ok || len(carrierClasses) != 1 ||
		!strings.HasPrefix(carrierClasses[0].String(), "carrier-class:") {
		t.Fatalf("carrier classes = %#v", carrierClasses)
	}
	captureRef, _, ok := permission.CaptureCarrier()
	if !ok {
		t.Fatal("permission omitted actual capture provenance")
	}
	if carrierClasses[0].String() == captureRef.String() {
		t.Fatal("actual capture occurrence was collapsed into CarrierClassRef")
	}
	permissionBytes, ok := permission.CanonicalBytes()
	if !ok {
		t.Fatal("permission omitted canonical bytes")
	}
	permissionDTO := profilePermissionJSONV2{}
	if err := json.Unmarshal(permissionBytes, &permissionDTO); err != nil {
		t.Fatalf("decode permission: %v", err)
	}
	if len(permissionDTO.CarrierClassRefs) != 1 || permissionDTO.CaptureCarrierRef == "" {
		t.Fatalf("permission source/adjudication split = %#v", permissionDTO)
	}
	if permissionDTO.EnactabilityPredicateRef == "" ||
		len(permissionDTO.Referents) != 2 ||
		permissionDTO.Referents[1] != permissionDTO.EnactabilityPredicateRef {
		t.Fatalf("permission enactability referent = %#v", permissionDTO)
	}
	if slices.Contains(permissionDTO.Referents, permissionDTO.AuthorizationContentRef) {
		t.Fatal("authorization description was mis-typed as a U.Commitment referent")
	}

	basis, ok := closure.Basis()
	if !ok {
		t.Fatal("closure omitted four-ref basis")
	}
	assertFourRefBasisMatchesClosure(t, basis, prepared, permission, source)
	basisBytes, _ := basis.CanonicalBytes()
	basisText := string(basisBytes)
	for _, forbidden := range []string{"legacy", "presentation", "resolution", "receipt"} {
		if strings.Contains(basisText, forbidden) {
			t.Fatalf("four-ref basis contains %q compatibility authority: %s", forbidden, basisText)
		}
	}

	review, ok := prepared.ReviewCard()
	if !ok {
		t.Fatal("prepared authorization omitted review card")
	}
	reviewText, _ := review.Text()
	if strings.Contains(reviewText, "sha256:") || !strings.Contains(reviewText, AuthorizationPhrase()) {
		t.Fatalf("normal review leaks machine copy-work: %s", reviewText)
	}
}

func TestValidateRecordedSourceRejectsEveryForeignBinding(t *testing.T) {
	prepared := testPreparedAuthorization(t, testPreparedInput{
		root:          "/tmp/haft-profile-authority-cross-binding",
		profileAuthor: "role-assignment:profile-author:cross-binding",
		identity:      "cross-binding",
	})
	window := prepared.state.content.state.authorizationValidity
	startedAt := window.From().Add(time.Minute)
	observedAt := startedAt.Add(time.Minute)
	endedAt := observedAt.Add(time.Minute)
	base := exactSourceRecipeForPrepared(t, prepared, startedAt, observedAt, endedAt)
	foreignRoot, err := authority.NewProjectRoot("/tmp/haft-profile-authority-foreign")
	if err != nil {
		t.Fatalf("foreign root: %v", err)
	}
	foreignSubject, err := authority.NewSpeechActReviewSubjectRef(
		"profile-authorization-content:foreign",
	)
	if err != nil {
		t.Fatalf("foreign subject: %v", err)
	}
	foreignPermission, err := authority.NewInstitutedObjectRef("permission:foreign-profile")
	if err != nil {
		t.Fatalf("foreign permission: %v", err)
	}
	foreignCapture, err := authority.NewCarrierRef("carrier:terminal-capture:profile-authority:foreign")
	if err != nil {
		t.Fatalf("foreign capture: %v", err)
	}
	foreignContentDigest := testDigest(t, "foreign-profile-author-content")
	foreignPolicy := testForeignProfilePolicy(t)
	cases := []struct {
		name     string
		expected string
		mutate   func(sourceRecipe) sourceRecipe
	}{
		{
			name:     "project root",
			expected: "project root",
			mutate: func(value sourceRecipe) sourceRecipe {
				value.projectRoot = foreignRoot
				return value
			},
		},
		{
			name:     "review subject",
			expected: "review subject",
			mutate: func(value sourceRecipe) sourceRecipe {
				value.reviewSubject = foreignSubject
				return value
			},
		},
		{
			name:     "profile author content",
			expected: "review subject digest",
			mutate: func(value sourceRecipe) sourceRecipe {
				value.reviewSubjectDig = foreignContentDigest
				return value
			},
		},
		{
			name:     "context policy",
			expected: "context policy",
			mutate: func(value sourceRecipe) sourceRecipe {
				value.policy = foreignPolicy
				return value
			},
		},
		{
			name:     "instituted object",
			expected: "instituted permission",
			mutate: func(value sourceRecipe) sourceRecipe {
				value.institutedObject = foreignPermission
				return value
			},
		},
		{
			name:     "terminal capture",
			expected: "terminal capture",
			mutate: func(value sourceRecipe) sourceRecipe {
				value.captureRef = foreignCapture
				return value
			},
		},
		{
			name:     "occurrence window",
			expected: "Work window",
			mutate: func(value sourceRecipe) sourceRecipe {
				value.startedAt = window.Until().Add(time.Minute)
				value.observedAt = value.startedAt.Add(time.Minute)
				value.endedAt = value.observedAt.Add(time.Minute)
				return value
			},
		},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			recipe := current.mutate(base)
			source := recordSourceFromRecipe(t, prepared, recipe)
			err := ValidateRecordedSource(prepared, source)
			if err == nil || !strings.Contains(err.Error(), current.expected) {
				t.Fatalf("ValidateRecordedSource error = %v, want %q", err, current.expected)
			}
		})
	}
}

type testPreparedInput struct {
	root          string
	profileAuthor string
	identity      string
}

func testPreparedAuthorization(
	t *testing.T,
	input testPreparedInput,
) PreparedAuthorization {
	t.Helper()
	root := mustParsed(t, input.root, authority.NewProjectRoot)
	contentRef := mustParsed(
		t,
		"profile-authorization-content:"+input.identity,
		authority.NewAuthorizationContentRef,
	)
	authorRef := mustParsed(t, input.profileAuthor, authority.NewRoleAssignmentRef)
	authorDigest := testDigest(t, "a"+input.identity)
	methodRef := mustParsed(
		t,
		"method-description:profile-onboarding:"+input.identity,
		authority.NewMethodDescriptionRef,
	)
	methodDigest := testDigest(t, "m"+input.identity)
	contractRef := mustParsed(
		t,
		"method-contract:profile-onboarding:"+input.identity,
		authority.NewMethodContractRef,
	)
	contractDigest := testDigest(t, "c"+input.identity)
	classifier := mustParsed(
		t,
		"classifier:v9:"+input.identity,
		authority.NewClassifierVersion,
	)
	policyVersion := mustParsed(
		t,
		"profile-policy:v9:"+input.identity,
		authority.NewPolicyVersion,
	)
	futureSession := mustParsed(
		t,
		"session:profile-onboarding:"+input.identity,
		authority.NewSessionRef,
	)
	validFrom := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	validUntil := validFrom.Add(4 * time.Hour)
	validity := mustTimeWindow(t, validFrom, validUntil)
	workWindow := mustTimeWindow(
		t,
		validFrom.Add(time.Hour),
		validFrom.Add(3*time.Hour),
	)
	basisWindow := mustTimeWindow(
		t,
		validFrom.Add(30*time.Minute),
		validFrom.Add(3*time.Hour),
	)
	singleUse := mustParsed(
		t,
		"profile.single-use."+input.identity,
		authority.NewSingleUseKey,
	)
	content, err := NewAuthorizationContentBuilder(contentRef, root).
		ForProfileAuthor(authorRef, authorDigest).
		ForMethod(methodRef, methodDigest, contractRef, contractDigest).
		WithVersions(classifier, policyVersion).
		InSession(futureSession).
		AllowWorkWithin(workWindow).
		AllowBasisObservationWithin(basisWindow).
		ValidWithin(validity).
		SingleUse(singleUse).
		Build()
	if err != nil {
		t.Fatalf("Build AuthorizationContent: %v", err)
	}
	permissionRef := mustParsed(
		t,
		"permission:profile-declaration:"+input.identity,
		authority.NewPermissionRef,
	)
	speechActRef := mustParsed(
		t,
		"speech-act:profile-declaration:"+input.identity,
		authority.NewSpeechActRef,
	)
	captureRef := mustParsed(
		t,
		"carrier:terminal-capture:profile-authority:"+input.identity,
		authority.NewCarrierRef,
	)
	speechSession := mustParsed(
		t,
		"session:profile-authority:"+input.identity,
		authority.NewSessionRef,
	)
	claimScope := mustParsed(
		t,
		"claim-scope:profile-declaration:"+input.identity,
		authority.NewClaimScopeRef,
	)
	predicate := mustParsed(
		t,
		"A-profile-enactability:"+input.identity,
		NewEnactabilityPredicateRef,
	)
	evidence := mustParsed(
		t,
		"E-profile-authorization:"+input.identity,
		NewEvidenceClaimRef,
	)
	carrierClass := mustParsed(
		t,
		"carrier-class:controlling-terminal-capture:"+input.identity,
		NewCarrierClassRef,
	)
	verifier := mustParsed(
		t,
		"verifier:profile-authority:"+input.identity,
		authority.NewVerifierIdentity,
	)
	verifierVersion := mustParsed(
		t,
		"verifier-version:v1:"+input.identity,
		authority.NewVerifierVersion,
	)
	verificationPolicy := mustParsed(
		t,
		"verification-policy:profile-authority:"+input.identity,
		authority.NewVerificationPolicyRef,
	)
	verificationPolicyDigest := testDigest(t, "v"+input.identity)
	basisRef := mustParsed(
		t,
		"profile-authority-basis:"+input.identity,
		NewBasisRef,
	)
	prepared, err := NewPreparedAuthorizationBuilder(
		content,
		permissionRef,
		speechActRef,
		captureRef,
	).
		InSpeechActSession(speechSession).
		WithinClaimScope(claimScope).
		UnderEnactabilityPredicate(predicate).
		WithAdjudication(evidence, carrierClass).
		VerifiedBy(verifier, verifierVersion, verificationPolicy, verificationPolicyDigest).
		AsBasis(basisRef).
		Build()
	if err != nil {
		t.Fatalf("Build PreparedAuthorization: %v", err)
	}
	return prepared
}

type sourceRecipe struct {
	projectRoot      authority.ProjectRoot
	speechActRef     authority.SpeechActRef
	captureRef       authority.CarrierRef
	sessionRef       authority.SessionRef
	reviewSubject    authority.SpeechActReviewSubjectRef
	reviewSubjectDig authority.Digest
	institutedObject authority.InstitutedObjectRef
	policy           authority.SpeechActContextPolicy
	startedAt        time.Time
	observedAt       time.Time
	endedAt          time.Time
}

func exactSourceRecipeForPrepared(
	t *testing.T,
	prepared PreparedAuthorization,
	startedAt time.Time,
	observedAt time.Time,
	endedAt time.Time,
) sourceRecipe {
	t.Helper()
	contentDigest, _ := prepared.state.content.Digest()
	contentRef, _ := prepared.state.content.Ref()
	reviewSubject := mustParsed(
		t,
		contentRef.String(),
		authority.NewSpeechActReviewSubjectRef,
	)
	instituted := mustParsed(
		t,
		prepared.state.permissionRef.String(),
		authority.NewInstitutedObjectRef,
	)
	root, _ := prepared.state.content.ProjectRoot()
	return sourceRecipe{
		projectRoot:      root,
		speechActRef:     prepared.state.speechActRef,
		captureRef:       prepared.state.captureRef,
		sessionRef:       prepared.state.speechActSession,
		reviewSubject:    reviewSubject,
		reviewSubjectDig: contentDigest,
		institutedObject: instituted,
		policy:           prepared.state.policy,
		startedAt:        startedAt,
		observedAt:       observedAt,
		endedAt:          endedAt,
	}
}

func recordSourceFromRecipe(
	t *testing.T,
	prepared PreparedAuthorization,
	recipe sourceRecipe,
) authority.RecordedSpeechActSource {
	t.Helper()
	frame, err := profileSpeechActExecutionFrame(
		prepared.state.content,
		prepared.state.permissionRef,
	)
	if err != nil {
		t.Fatalf("profileSpeechActExecutionFrame: %v", err)
	}
	intent, err := authority.NewPreparedSpeechActIntentBuilder(
		recipe.speechActRef,
		recipe.captureRef,
	).
		ForProject(recipe.projectRoot).
		InSession(recipe.sessionRef).
		Reviewing(recipe.reviewSubject, recipe.reviewSubjectDig).
		Institutes(recipe.institutedObject).
		UnderContextPolicy(recipe.policy).
		WithExecutionFrame(frame).
		Build()
	if err != nil {
		t.Fatalf("build source intent: %v", err)
	}
	reviewText, _ := prepared.state.review.Text()
	manual, err := authority.PrepareManualSpeechAct(intent, reviewText)
	if err != nil {
		t.Fatalf("PrepareManualSpeechAct: %v", err)
	}
	verified, err := authority.CaptureVerifiedSpeechActForTestFixture(
		t,
		manual,
		recipe.startedAt,
		recipe.observedAt,
		recipe.endedAt,
	)
	if err != nil {
		t.Fatalf("CaptureVerifiedSpeechActForTestFixture: %v", err)
	}
	store, err := kerneldb.NewStore(filepath.Join(t.TempDir(), "profile-authority.sqlite"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	database.SetMaxOpenConns(1)
	writer, err := authority.OpenSpeechActSourceWriter(database)
	if err != nil {
		t.Fatalf("OpenSpeechActSourceWriter: %v", err)
	}
	recorded, err := writer.Record(context.Background(), verified)
	if err != nil {
		t.Fatalf("Record SpeechAct source: %v", err)
	}
	return recorded
}

func testForeignProfilePolicy(t *testing.T) authority.SpeechActContextPolicy {
	t.Helper()
	policyRef := mustParsed(
		t,
		"context-policy:foreign-profile-authorization:v1",
		authority.NewContextPolicyRef,
	)
	bounded := mustParsed(
		t,
		profileBoundedContextValue,
		authority.NewBoundedContextRef,
	)
	actType := mustParsed(
		t,
		profileActTypeValue,
		authority.NewSpeechActTypeRef,
	)
	ruleRef := mustParsed(
		t,
		"institution-rule:foreign-profile-authorization:v1",
		authority.NewInstitutionalEffectRuleRef,
	)
	kind := mustParsed(t, "U.Commitment", authority.NewInstitutedObjectKind)
	modality := mustParsed(t, "MAY", authority.NewInstitutionalModality)
	action, err := ActionKind()
	if err != nil {
		t.Fatalf("profile action: %v", err)
	}
	utteranceRule, err := authority.NewLiteralSpeechActUtteranceRule(
		profileAuthorizationVerb,
		profileAuthorizationLiteral,
	)
	if err != nil {
		t.Fatalf("literal utterance: %v", err)
	}
	utterance := mustParsed(
		t,
		profileUtteranceValue,
		authority.NewUtteranceRef,
	)
	rule, err := authority.NewInstitutionalEffectRule(
		ruleRef,
		kind,
		modality,
		action,
		utteranceRule,
		utterance,
	)
	if err != nil {
		t.Fatalf("foreign effect rule: %v", err)
	}
	policy, err := authority.NewSpeechActContextPolicy(policyRef, bounded, actType, rule)
	if err != nil {
		t.Fatalf("foreign policy: %v", err)
	}
	return policy
}

func assertFourRefBasisMatchesClosure(
	t *testing.T,
	basis FourRefBasis,
	prepared PreparedAuthorization,
	permission Permission,
	source authority.RecordedSpeechActSource,
) {
	t.Helper()
	actualSpeechActRef, actualSpeechActDigest, speechOK := basis.SpeechAct()
	expectedSpeechActRef, _ := source.SpeechActRef()
	expectedSpeechActDigest, _ := source.SpeechActDigest()
	if !speechOK || actualSpeechActRef.String() != expectedSpeechActRef.String() ||
		actualSpeechActDigest.String() != expectedSpeechActDigest.String() {
		t.Fatal("basis SpeechAct pair differs from durable source")
	}
	actualContentRef, actualContentDigest, contentOK := basis.AuthorizationContent()
	expectedContentRef, _ := prepared.state.content.Ref()
	expectedContentDigest, _ := prepared.state.content.Digest()
	if !contentOK || actualContentRef.String() != expectedContentRef.String() ||
		actualContentDigest.String() != expectedContentDigest.String() {
		t.Fatal("basis content pair differs from authorization content")
	}
	actualPermissionRef, actualPermissionDigest, permissionOK := basis.Permission()
	expectedPermissionRef, _ := permission.Ref()
	expectedPermissionDigest, _ := permission.Digest()
	if !permissionOK || actualPermissionRef.String() != expectedPermissionRef.String() ||
		actualPermissionDigest.String() != expectedPermissionDigest.String() {
		t.Fatal("basis permission pair differs from instituted permission")
	}
	actualPolicyRef, actualPolicyDigest, policyOK := basis.ContextPolicy()
	expectedPolicyRef, _ := prepared.state.policy.Ref()
	expectedPolicyDigest, _ := prepared.state.policy.Digest()
	if !policyOK || actualPolicyRef.String() != expectedPolicyRef.String() ||
		actualPolicyDigest.String() != expectedPolicyDigest.String() {
		t.Fatal("basis context-policy pair differs from prepared policy")
	}
}

func testDigest(t *testing.T, seed string) authority.Digest {
	t.Helper()
	value := []byte(seed)
	digest, _, err := canonicalDigest("test.profile-authority.digest/v1\x00", value)
	if err != nil {
		t.Fatalf("test digest: %v", err)
	}
	return digest
}

func mustParsed[T any](
	t *testing.T,
	raw string,
	parse func(string) (T, error),
) T {
	t.Helper()
	value, err := parse(raw)
	if err != nil {
		t.Fatalf("fixture value: %v", err)
	}
	return value
}

func mustTimeWindow(
	t *testing.T,
	from time.Time,
	until time.Time,
) authority.TimeWindow {
	t.Helper()
	window, err := authority.NewTimeWindow(from, until)
	if err != nil {
		t.Fatalf("fixture time window: %v", err)
	}
	return window
}
