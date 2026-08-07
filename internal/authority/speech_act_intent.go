package authority

import (
	"fmt"
	"regexp"
	"strings"
)

var speechActUtteranceVerbPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_-]{0,31}$`)

type speechActUtteranceBinding string

const (
	utteranceBindsReviewDigest  speechActUtteranceBinding = "review_digest"
	utteranceBindsReviewSubject speechActUtteranceBinding = "review_subject_digest"
	utteranceBindsLiteral       speechActUtteranceBinding = "literal"
)

// SpeechActUtteranceRule is a closed, typed canonical-utterance predicate.
// It keeps the generic source layer independent of profile and migration verbs.
type SpeechActUtteranceRule struct {
	verb    string
	binding speechActUtteranceBinding
	literal string
}

type InstitutedObjectKind struct{ value string }

func NewInstitutedObjectKind(raw string) (InstitutedObjectKind, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\n\r\t") {
		return InstitutedObjectKind{}, fmt.Errorf("instituted-object kind is not canonical")
	}
	return InstitutedObjectKind{value: raw}, nil
}

func (kind InstitutedObjectKind) String() string { return kind.value }
func (kind InstitutedObjectKind) valid() bool {
	return kind.value != "" && strings.TrimSpace(kind.value) == kind.value
}

type InstitutionalModality struct{ value string }

func NewInstitutionalModality(raw string) (InstitutionalModality, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\n\r\t") {
		return InstitutionalModality{}, fmt.Errorf("institutional modality is not canonical")
	}
	return InstitutionalModality{value: raw}, nil
}

func (modality InstitutionalModality) String() string { return modality.value }
func (modality InstitutionalModality) valid() bool {
	return modality.value != "" && strings.TrimSpace(modality.value) == modality.value
}

// InstitutionalEffectRule is an explicit act-to-effect mapping. The rule is
// source material; a later SpeechAct still has to occur before any effect is
// instituted.
type InstitutionalEffectRule struct {
	ref                  InstitutionalEffectRuleRef
	institutedObjectKind InstitutedObjectKind
	modality             InstitutionalModality
	scopedAction         ActionKind
	utteranceRule        SpeechActUtteranceRule
	utteranceDescription UtteranceRef
}

func NewInstitutionalEffectRule(
	ref InstitutionalEffectRuleRef,
	institutedObjectKind InstitutedObjectKind,
	modality InstitutionalModality,
	scopedAction ActionKind,
	utteranceRule SpeechActUtteranceRule,
	utteranceDescription UtteranceRef,
) (InstitutionalEffectRule, error) {
	rule := InstitutionalEffectRule{
		ref:                  ref,
		institutedObjectKind: institutedObjectKind,
		modality:             modality,
		scopedAction:         scopedAction,
		utteranceRule:        utteranceRule,
		utteranceDescription: utteranceDescription,
	}
	if !rule.valid() {
		return InstitutionalEffectRule{}, fmt.Errorf("institutional-effect rule is incomplete")
	}
	return rule, nil
}

func (rule InstitutionalEffectRule) valid() bool {
	return rule.ref.valid() &&
		rule.institutedObjectKind.valid() &&
		rule.modality.valid() &&
		rule.scopedAction.valid() &&
		rule.utteranceRule.valid() &&
		rule.utteranceDescription.valid()
}

func AuthorizeReviewedIntentUtteranceRule() SpeechActUtteranceRule {
	return SpeechActUtteranceRule{verb: "AUTHORIZE", binding: utteranceBindsReviewDigest}
}

func AcceptReviewedCarrierUtteranceRule() SpeechActUtteranceRule {
	return SpeechActUtteranceRule{verb: "ACCEPT", binding: utteranceBindsReviewSubject}
}

// NewLiteralSpeechActUtteranceRule creates a policy-owned, human-readable
// canonical utterance. The reviewed subject and its exact digest remain bound
// by the PreparedSpeechActIntent and terminal-capture record; a person does
// not have to transcribe that machine address to perform the act.
func NewLiteralSpeechActUtteranceRule(
	verb string,
	literal string,
) (SpeechActUtteranceRule, error) {
	rule := SpeechActUtteranceRule{
		verb:    verb,
		binding: utteranceBindsLiteral,
		literal: literal,
	}
	if !rule.valid() {
		return SpeechActUtteranceRule{}, fmt.Errorf("literal SpeechAct utterance rule is not canonical")
	}
	return rule, nil
}

func (rule SpeechActUtteranceRule) expected(
	reviewDigest Digest,
	reviewSubjectDigest Digest,
) (string, error) {
	if !rule.valid() || !reviewDigest.valid() || !reviewSubjectDigest.valid() {
		return "", fmt.Errorf("SpeechAct utterance rule requires canonical digest bindings")
	}
	if rule.binding == utteranceBindsLiteral {
		return rule.verb + " " + rule.literal, nil
	}
	binding := reviewDigest
	if rule.binding == utteranceBindsReviewSubject {
		binding = reviewSubjectDigest
	}
	return rule.verb + " " + binding.String(), nil
}

func (rule SpeechActUtteranceRule) valid() bool {
	validVerb := speechActUtteranceVerbPattern.MatchString(rule.verb)
	digestBinding := rule.binding == utteranceBindsReviewDigest ||
		rule.binding == utteranceBindsReviewSubject
	literalBinding := rule.binding == utteranceBindsLiteral &&
		rule.literal != "" &&
		len(rule.literal) <= 160 &&
		strings.TrimSpace(rule.literal) == rule.literal &&
		!strings.ContainsAny(rule.literal, "\n\r\t")
	bindingShapeValid := digestBinding && rule.literal == "" || literalBinding
	return validVerb && bindingShapeValid && strings.TrimSpace(rule.verb) == rule.verb
}

// PreparedSpeechActIntent is a reusable, design-time description of a manual
// SpeechAct. It binds the subject reviewed and the object that a successful act
// is expected to institute by stable refs. It contains no capture, performed
// Work, RoleAssignment, SpeechAct, or instituted-object digest.
type PreparedSpeechActIntent struct {
	state *preparedSpeechActIntentState
}

type preparedSpeechActIntentState struct {
	speechActRef      SpeechActRef
	captureCarrierRef CarrierRef
	projectRoot       ProjectRoot
	sessionRef        SessionRef
	reviewSubjectRef  SpeechActReviewSubjectRef
	reviewSubjectDig  Digest
	institutedObject  InstitutedObjectRef
	contextPolicy     SpeechActContextPolicy
	executionFrame    SpeechActExecutionFrame
	utteranceRule     SpeechActUtteranceRule
	digest            Digest
}

type PreparedSpeechActIntentBuilder struct {
	value preparedSpeechActIntentState
}

func NewPreparedSpeechActIntentBuilder(
	speechActRef SpeechActRef,
	captureCarrierRef CarrierRef,
) PreparedSpeechActIntentBuilder {
	return PreparedSpeechActIntentBuilder{value: preparedSpeechActIntentState{
		speechActRef:      speechActRef,
		captureCarrierRef: captureCarrierRef,
	}}
}

func (builder PreparedSpeechActIntentBuilder) InSession(
	sessionRef SessionRef,
) PreparedSpeechActIntentBuilder {
	builder.value.sessionRef = sessionRef
	return builder
}

func (builder PreparedSpeechActIntentBuilder) ForProject(
	projectRoot ProjectRoot,
) PreparedSpeechActIntentBuilder {
	builder.value.projectRoot = projectRoot
	return builder
}

func (builder PreparedSpeechActIntentBuilder) Reviewing(
	ref SpeechActReviewSubjectRef,
	digest Digest,
) PreparedSpeechActIntentBuilder {
	builder.value.reviewSubjectRef = ref
	builder.value.reviewSubjectDig = digest
	return builder
}

func (builder PreparedSpeechActIntentBuilder) Institutes(
	ref InstitutedObjectRef,
) PreparedSpeechActIntentBuilder {
	builder.value.institutedObject = ref
	return builder
}

func (builder PreparedSpeechActIntentBuilder) UnderContextPolicy(
	policy SpeechActContextPolicy,
) PreparedSpeechActIntentBuilder {
	builder.value.contextPolicy = policy
	return builder
}

func (builder PreparedSpeechActIntentBuilder) WithExecutionFrame(
	frame SpeechActExecutionFrame,
) PreparedSpeechActIntentBuilder {
	builder.value.executionFrame = frame
	return builder
}

func (builder PreparedSpeechActIntentBuilder) Build() (PreparedSpeechActIntent, error) {
	value, err := canonicalPreparedSpeechActIntent(builder.value)
	if err != nil {
		return PreparedSpeechActIntent{}, err
	}
	return PreparedSpeechActIntent{state: &value}, nil
}

func canonicalPreparedSpeechActIntent(
	value preparedSpeechActIntentState,
) (preparedSpeechActIntentState, error) {
	identitiesValid := value.speechActRef.valid() &&
		value.captureCarrierRef.valid() &&
		value.projectRoot.valid() &&
		value.sessionRef.valid() &&
		value.reviewSubjectRef.valid() &&
		value.reviewSubjectDig.valid() &&
		value.institutedObject.valid()
	if !identitiesValid || !value.contextPolicy.valid() || !value.executionFrame.valid() {
		return preparedSpeechActIntentState{}, fmt.Errorf("prepared SpeechAct intent is incomplete")
	}
	methodContext := value.executionFrame.state.methodDescription.state.boundedContext
	policyContext := value.contextPolicy.state.boundedContext
	if methodContext != policyContext {
		return preparedSpeechActIntentState{}, fmt.Errorf(
			"SpeechAct MethodDescription and context policy belong to different bounded contexts",
		)
	}
	value.utteranceRule = value.contextPolicy.state.effectRule.utteranceRule
	if value.executionFrame.state.utterance != value.contextPolicy.state.effectRule.utteranceDescription {
		return preparedSpeechActIntentState{}, fmt.Errorf("SpeechAct utterance description does not match context policy")
	}
	writer := newAuthorityDigestWriter(preparedSpeechActIntentDomain)
	writer.add(value.speechActRef.String())
	writer.add(value.captureCarrierRef.String())
	writer.add(value.projectRoot.String())
	writer.add(value.sessionRef.String())
	writer.add(value.reviewSubjectRef.String())
	writer.add(value.reviewSubjectDig.String())
	writer.add(value.institutedObject.String())
	writer.add(value.contextPolicy.state.ref.String())
	writer.add(value.contextPolicy.state.digest.String())
	writer.add(value.executionFrame.state.digest.String())
	writer.add(value.utteranceRule.verb)
	writer.add(string(value.utteranceRule.binding))
	if value.utteranceRule.literal != "" {
		writer.add(value.utteranceRule.literal)
	}
	value.digest = writer.digest()
	return value, nil
}

func (intent PreparedSpeechActIntent) Digest() (Digest, bool) {
	if !intent.valid() {
		return Digest{}, false
	}
	return intent.state.digest, true
}

func (intent PreparedSpeechActIntent) valid() bool {
	if intent.state == nil {
		return false
	}
	rebuilt, err := canonicalPreparedSpeechActIntent(*intent.state)
	return err == nil && rebuilt.digest == intent.state.digest
}

func SpeechActIntentReviewDigest(
	intent PreparedSpeechActIntent,
	reviewText string,
) (Digest, error) {
	if !intent.valid() {
		return Digest{}, fmt.Errorf("prepared SpeechAct intent is invalid")
	}
	if err := validateAuthorityIssueReviewText(reviewText); err != nil {
		return Digest{}, err
	}
	writer := newAuthorityDigestWriter(authorityIssueReviewDigestDomain)
	writer.add(intent.state.digest.String())
	writer.add(reviewText)
	return writer.digest(), nil
}

// PreparedManualSpeechAct is the exact pre-capture source presented for one
// manual SpeechAct. It seals the domain-owned review text to its typed intent;
// it is not a terminal capture, performed Work, or instituted effect.
type PreparedManualSpeechAct struct {
	state *preparedManualSpeechActState
}

type preparedManualSpeechActState struct {
	intent       PreparedSpeechActIntent
	reviewText   string
	reviewDigest Digest
}

func PrepareManualSpeechAct(
	intent PreparedSpeechActIntent,
	reviewText string,
) (PreparedManualSpeechAct, error) {
	reviewDigest, err := SpeechActIntentReviewDigest(intent, reviewText)
	if err != nil {
		return PreparedManualSpeechAct{}, err
	}
	state := preparedManualSpeechActState{
		intent:       intent,
		reviewText:   reviewText,
		reviewDigest: reviewDigest,
	}
	prepared := PreparedManualSpeechAct{state: &state}
	if !prepared.valid() {
		return PreparedManualSpeechAct{}, fmt.Errorf("prepared manual SpeechAct is invalid")
	}
	return prepared, nil
}

func (prepared PreparedManualSpeechAct) Intent() (PreparedSpeechActIntent, bool) {
	if !prepared.valid() {
		return PreparedSpeechActIntent{}, false
	}
	return prepared.state.intent, true
}

func (prepared PreparedManualSpeechAct) ReviewText() (string, bool) {
	if !prepared.valid() {
		return "", false
	}
	return prepared.state.reviewText, true
}

func (prepared PreparedManualSpeechAct) ReviewDigest() (Digest, bool) {
	if !prepared.valid() {
		return Digest{}, false
	}
	return prepared.state.reviewDigest, true
}

func (prepared PreparedManualSpeechAct) valid() bool {
	if prepared.state == nil || !prepared.state.intent.valid() {
		return false
	}
	digest, err := SpeechActIntentReviewDigest(
		prepared.state.intent,
		prepared.state.reviewText,
	)
	return err == nil && digest == prepared.state.reviewDigest
}
