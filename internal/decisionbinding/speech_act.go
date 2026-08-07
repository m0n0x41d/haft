package decisionbinding

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/authority"
)

const (
	decisionSpeechActVerb    = "DECIDE"
	decisionSpeechActLiteral = "THIS REVIEWED CHOICE"

	// CanonicalDecisionSpeechActPhrase is the only ordinary human utterance
	// recognized by the decision-binding context policy.
	CanonicalDecisionSpeechActPhrase = decisionSpeechActVerb + " " + decisionSpeechActLiteral

	decisionBoundedContextValue = "bounded-context:haft-decision-binding"
	decisionActTypeValue        = "speech-act-type:decide"
	decisionEffectRuleValue     = "institution-rule:decide-institutes-decision-record:v1"
	decisionObjectKindValue     = "haft.DecisionRecord"
	decisionModalityValue       = "BOUND"
	decisionActionValue         = "decision.bind"
	decisionUtteranceValue      = "utterance:decide-this-reviewed-choice:v1"
	decisionContextPolicyValue  = "context-policy:decision-binding:v1"
	decisionMethodValue         = "method:decision-binding-manual-speech-act"
	decisionMethodDescValue     = "method-description:decision-binding-manual-speech-act:v1"
	decisionMethodProcedure     = "procedure:review-exact-intent-capture-controlling-terminal:v1"
	decisionSystemValue         = "system:haft-decision-binding"
	decisionStatePlaneValue     = "state-plane:decision-record-binding"
	decisionDeltaValue          = "delta-predicate:decision-record-instituted"
	decisionOutcomeValue        = "work-outcome:decision-record-instituted"
)

// DecisionSpeechActContextPolicy is the decision-domain act-to-effect rule.
// It institutes a DecisionRecord binding; it grants no permission and carries
// no WorkCommission semantics.
func DecisionSpeechActContextPolicy() (authority.SpeechActContextPolicy, error) {
	policyRef, err := authority.NewContextPolicyRef(decisionContextPolicyValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	boundedContext, err := authority.NewBoundedContextRef(decisionBoundedContextValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	actType, err := authority.NewSpeechActTypeRef(decisionActTypeValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	rule, err := decisionInstitutionalEffectRule()
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	return authority.NewSpeechActContextPolicy(
		policyRef,
		boundedContext,
		actType,
		rule,
	)
}

func decisionInstitutionalEffectRule() (authority.InstitutionalEffectRule, error) {
	ruleRef, err := authority.NewInstitutionalEffectRuleRef(decisionEffectRuleValue)
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	objectKind, err := authority.NewInstitutedObjectKind(decisionObjectKindValue)
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	modality, err := authority.NewInstitutionalModality(decisionModalityValue)
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	action, err := authority.NewActionKind(decisionActionValue)
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	utteranceRule, err := authority.NewLiteralSpeechActUtteranceRule(
		decisionSpeechActVerb,
		decisionSpeechActLiteral,
	)
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	utterance, err := authority.NewUtteranceRef(decisionUtteranceValue)
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	return authority.NewInstitutionalEffectRule(
		ruleRef,
		objectKind,
		modality,
		action,
		utteranceRule,
		utterance,
	)
}

// PrepareDecisionSpeechActIntent builds design-time source material only. It
// performs no terminal capture and institutes no DecisionRecord.
func PrepareDecisionSpeechActIntent(
	content DecisionBindingContent,
) (authority.PreparedSpeechActIntent, error) {
	root, rootOK := content.ProjectRoot()
	decisionRef, decisionRefOK := content.DecisionRef()
	contentDigest, digestOK := content.Digest()
	if !rootOK || !decisionRefOK || !digestOK {
		return authority.PreparedSpeechActIntent{}, fmt.Errorf("decision-binding content is invalid")
	}
	identity := strings.TrimPrefix(contentDigest.String(), "sha256:")
	policy, err := DecisionSpeechActContextPolicy()
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	frame, err := decisionSpeechActExecutionFrame(contentDigest, decisionRef)
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	speechActRef, err := authority.NewSpeechActRef("speech-act:decision-binding:" + identity)
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	captureRef, err := authority.NewCarrierRef("carrier:terminal-capture:decision-binding:" + identity)
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	sessionRef, err := authority.NewSessionRef("session:decision-binding:" + identity)
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	reviewSubjectRef, reviewSubjectOK := content.ContentRef()
	if !reviewSubjectOK {
		return authority.PreparedSpeechActIntent{}, fmt.Errorf(
			"decision-binding content has no canonical content ref",
		)
	}
	institutedRef, err := authority.NewInstitutedObjectRef(decisionRef)
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	builder := authority.NewPreparedSpeechActIntentBuilder(speechActRef, captureRef)
	builder = builder.ForProject(root)
	builder = builder.InSession(sessionRef)
	builder = builder.Reviewing(reviewSubjectRef, contentDigest)
	builder = builder.Institutes(institutedRef)
	builder = builder.UnderContextPolicy(policy)
	builder = builder.WithExecutionFrame(frame)
	return builder.Build()
}

func decisionSpeechActExecutionFrame(
	contentDigest authority.Digest,
	decisionRef string,
) (authority.SpeechActExecutionFrame, error) {
	systemRef, err := authority.NewSystemRef(decisionSystemValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	statePlane, err := authority.NewStatePlaneRef(decisionStatePlaneValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	delta, err := authority.NewDeltaPredicateRef(decisionDeltaValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	outcome, err := authority.NewWorkOutcomeRef(decisionOutcomeValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	utterance, err := authority.NewUtteranceRef(decisionUtteranceValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	parameter, err := authority.NewWorkParameterBinding(
		"parameter:decision-binding-content-digest",
		contentDigest.String(),
	)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	resource, err := authority.NewWorkResourceRef("resource:controlling-terminal")
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	decisionAffected, err := authority.NewAffectedRef(
		"affected:decision-record:" + decisionRef,
	)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	contentAffected, err := authority.NewAffectedRef(
		"affected:decision-binding-content:" + contentDigest.String(),
	)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	method, err := decisionSpeechActMethodDescription()
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	builder := authority.NewSpeechActExecutionFrameBuilder(method)
	builder = builder.ExecutedWithin(systemRef)
	builder = builder.OnStatePlane(statePlane, delta)
	builder = builder.WithOutcome(outcome)
	builder = builder.WithUtteranceDescription(utterance)
	builder = builder.BindParameter(parameter)
	builder = builder.UseResource(resource)
	builder = builder.Affect(decisionAffected)
	builder = builder.Affect(contentAffected)
	return builder.Build()
}

func decisionSpeechActMethodDescription() (
	authority.SpeechActMethodDescription,
	error,
) {
	methodRef, err := authority.NewMethodRef(decisionMethodValue)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	descriptionRef, err := authority.NewMethodDescriptionRef(decisionMethodDescValue)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	procedureRef, err := authority.NewMethodProcedureRef(decisionMethodProcedure)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	boundedContext, err := authority.NewBoundedContextRef(decisionBoundedContextValue)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	return authority.NewManualControllingTTYMethodDescription(
		methodRef,
		descriptionRef,
		procedureRef,
		boundedContext,
	)
}

// ValidateDecisionSpeechActContextPolicy rejects a policy from any other
// institutional domain even when that policy is internally self-consistent.
func ValidateDecisionSpeechActContextPolicy(
	policy authority.SpeechActContextPolicy,
) error {
	expected, err := DecisionSpeechActContextPolicy()
	if err != nil {
		return err
	}
	expectedRef, expectedRefOK := expected.Ref()
	expectedDigest, expectedDigestOK := expected.Digest()
	actualRef, actualRefOK := policy.Ref()
	actualDigest, actualDigestOK := policy.Digest()
	complete := expectedRefOK && expectedDigestOK && actualRefOK && actualDigestOK
	if !complete {
		return fmt.Errorf("decision-binding context policy is incomplete")
	}
	matches := expectedRef.String() == actualRef.String() &&
		expectedDigest.String() == actualDigest.String()
	if !matches {
		return fmt.Errorf("context policy belongs to another institutional effect")
	}
	return nil
}

// ValidatePreparedDecisionSpeechActIntent prevents an exact act prepared for
// another content or context policy from being cross-bound as this decision.
func ValidatePreparedDecisionSpeechActIntent(
	content DecisionBindingContent,
	intent authority.PreparedSpeechActIntent,
) error {
	expected, err := PrepareDecisionSpeechActIntent(content)
	if err != nil {
		return err
	}
	expectedDigest, expectedOK := expected.Digest()
	actualDigest, actualOK := intent.Digest()
	if !expectedOK || !actualOK {
		return fmt.Errorf("prepared decision SpeechAct intent is incomplete")
	}
	if expectedDigest.String() != actualDigest.String() {
		return fmt.Errorf("prepared SpeechAct intent does not bind this exact decision content")
	}
	return nil
}
