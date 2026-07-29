package profileauthority

import (
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

type exactSourceBindings struct {
	projectRoot       authority.ProjectRoot
	speechActRef      authority.SpeechActRef
	speechActDigest   authority.Digest
	captureRef        authority.CarrierRef
	captureDigest     authority.Digest
	performedByRef    authority.RoleAssignmentRef
	performedByDigest authority.Digest
	contextPolicyRef  authority.ContextPolicyRef
	contextPolicyDig  authority.Digest
	workWindow        authority.TimeWindow
	occurredAt        time.Time
	completedAt       time.Time
}

// ValidateRecordedSource proves that an immutable generic SpeechAct source is
// the exact occurrence prepared for this profile authorization. It does not
// turn the recorded source back into a completion capability.
func ValidateRecordedSource(
	prepared PreparedAuthorization,
	source authority.RecordedSpeechActSource,
) error {
	_, err := exactRecordedSourceBindings(prepared, source)
	return err
}

func exactRecordedSourceBindings(
	prepared PreparedAuthorization,
	source authority.RecordedSpeechActSource,
) (exactSourceBindings, error) {
	if !prepared.valid() {
		return exactSourceBindings{}, fmt.Errorf(
			"profile permission requires exact prepared authorization",
		)
	}
	if !source.Valid() {
		return exactSourceBindings{}, fmt.Errorf(
			"profile permission requires a durable canonical SpeechAct source",
		)
	}
	root, rootOK := source.ProjectRoot()
	intentDigest, intentDigestOK := source.PreparedIntentDigest()
	speechActRef, speechActRefOK := source.SpeechActRef()
	speechActDigest, speechActDigestOK := source.SpeechActDigest()
	captureRef, captureRefOK := source.TerminalCaptureRef()
	captureDigest, captureDigestOK := source.TerminalCaptureDigest()
	performedByRef, performedByRefOK := source.PerformedByRoleAssignmentRef()
	performedByDigest, performedByDigestOK := source.PerformedByRoleAssignmentDigest()
	reviewText, reviewTextOK := source.ReviewText()
	reviewDigest, reviewDigestOK := source.ReviewDigest()
	reviewSubjectRef, reviewSubjectRefOK := source.ReviewSubjectRef()
	reviewSubjectDigest, reviewSubjectDigestOK := source.ReviewSubjectDigest()
	institutedObjectRef, institutedObjectRefOK := source.InstitutedObjectRef()
	policyRef, policyRefOK := source.ContextPolicyRef()
	policyDigest, policyDigestOK := source.ContextPolicyDigest()
	workWindow, workWindowOK := source.WorkWindow()
	occurredAt, occurredAtOK := source.OccurredAt()
	completedAt, completedAtOK := source.CompletedAt()
	complete := rootOK && intentDigestOK && speechActRefOK && speechActDigestOK
	complete = complete && captureRefOK && captureDigestOK
	complete = complete && performedByRefOK && performedByDigestOK
	complete = complete && reviewTextOK && reviewDigestOK
	complete = complete && reviewSubjectRefOK && reviewSubjectDigestOK
	complete = complete && institutedObjectRefOK && policyRefOK && policyDigestOK
	complete = complete && workWindowOK && occurredAtOK && completedAtOK
	if !complete {
		return exactSourceBindings{}, fmt.Errorf(
			"durable profile SpeechAct source bindings are incomplete",
		)
	}
	expectedIntentDigest, _ := prepared.state.speechActIntent.Digest()
	expectedContentDigest, _ := prepared.state.content.Digest()
	expectedReviewText, _ := prepared.state.review.Text()
	expectedReviewDigest, err := authority.SpeechActIntentReviewDigest(
		prepared.state.speechActIntent,
		expectedReviewText,
	)
	if err != nil {
		return exactSourceBindings{}, err
	}
	expectedPolicyRef, _ := prepared.state.policy.Ref()
	expectedPolicyDigest, _ := prepared.state.policy.Digest()
	expectedRoot, _ := prepared.state.content.ProjectRoot()
	expectedContentRef, _ := prepared.state.content.Ref()
	expectedSubject := expectedContentRef.String()
	expectedInstituted := prepared.state.permissionRef.String()
	validity, _ := prepared.state.content.AuthorizationValidity()
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: root.String() == expectedRoot.String(), name: "project root"},
		{matches: speechActRef.String() == prepared.state.speechActRef.String(), name: "SpeechAct occurrence"},
		{matches: captureRef.String() == prepared.state.captureRef.String(), name: "terminal capture"},
		{matches: reviewSubjectRef.String() == expectedSubject, name: "review subject"},
		{matches: reviewSubjectDigest.String() == expectedContentDigest.String(), name: "review subject digest"},
		{matches: institutedObjectRef.String() == expectedInstituted, name: "instituted permission"},
		{matches: policyRef.String() == expectedPolicyRef.String(), name: "context policy"},
		{matches: policyDigest.String() == expectedPolicyDigest.String(), name: "context policy digest"},
		{matches: intentDigest.String() == expectedIntentDigest.String(), name: "prepared intent"},
		{matches: reviewText == expectedReviewText, name: "human review card"},
		{matches: reviewDigest.String() == expectedReviewDigest.String(), name: "review digest"},
		{matches: coveredBy(validity, workWindow), name: "SpeechAct Work window"},
		{matches: workWindow.Contains(occurredAt), name: "SpeechAct observation time"},
		{matches: completedAt.Equal(workWindow.Until()), name: "SpeechAct completion time"},
	}
	invalid := slices.IndexFunc(checks, func(check struct {
		matches bool
		name    string
	}) bool {
		return !check.matches
	})
	if invalid >= 0 {
		return exactSourceBindings{}, fmt.Errorf(
			"durable profile SpeechAct source has another %s",
			checks[invalid].name,
		)
	}
	return exactSourceBindings{
		projectRoot:       root,
		speechActRef:      speechActRef,
		speechActDigest:   speechActDigest,
		captureRef:        captureRef,
		captureDigest:     captureDigest,
		performedByRef:    performedByRef,
		performedByDigest: performedByDigest,
		contextPolicyRef:  policyRef,
		contextPolicyDig:  policyDigest,
		workWindow:        workWindow,
		occurredAt:        canonicalTime(occurredAt),
		completedAt:       canonicalTime(completedAt),
	}, nil
}
