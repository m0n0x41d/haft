package decisionbinding

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

const (
	decisionRecordInstitutedEffectSchema = "haft.decision-record-instituted-effect/v1"
	decisionRecordEffectDigestDomain     = "haft.decision-record-instituted-effect-digest/v1\x00"
)

// DecisionRecordInstitutedEffect is the immutable decision-domain consequence
// of one exact durable manual SpeechAct. The generic SpeechAct source is not
// itself a DecisionRecord and this value is not a permission to perform Work.
type DecisionRecordInstitutedEffect struct {
	state *decisionRecordInstitutedEffectState
}

type decisionRecordInstitutedEffectState struct {
	content                 DecisionBindingContent
	source                  authority.RecordedSpeechActSource
	projectRoot             authority.ProjectRoot
	decisionRef             string
	contentRef              authority.SpeechActReviewSubjectRef
	contentDigest           authority.Digest
	speechActRef            authority.SpeechActRef
	speechActDigest         authority.Digest
	contextPolicyRef        authority.ContextPolicyRef
	contextPolicyDigest     authority.Digest
	institutionalEffectRule authority.InstitutionalEffectRuleRef
	occurredAt              time.Time
	digest                  authority.Digest
	canonicalBytes          []byte
}

type decisionRecordInstitutedEffectPayloadV1 struct {
	Schema                     string `json:"schema"`
	ProjectRoot                string `json:"project_root"`
	DecisionRef                string `json:"decision_ref"`
	DecisionContentRef         string `json:"decision_content_ref"`
	DecisionContentDigest      string `json:"decision_content_digest"`
	SpeechActRef               string `json:"speech_act_ref"`
	SpeechActDigest            string `json:"speech_act_digest"`
	ContextPolicyRef           string `json:"context_policy_ref"`
	ContextPolicyDigest        string `json:"context_policy_digest"`
	InstitutionalEffectRuleRef string `json:"institutional_effect_rule_ref"`
}

type decisionRecordInstitutedEffectJSONV1 struct {
	Schema                     string `json:"schema"`
	EffectDigest               string `json:"effect_digest"`
	ProjectRoot                string `json:"project_root"`
	DecisionRef                string `json:"decision_ref"`
	DecisionContentRef         string `json:"decision_content_ref"`
	DecisionContentDigest      string `json:"decision_content_digest"`
	SpeechActRef               string `json:"speech_act_ref"`
	SpeechActDigest            string `json:"speech_act_digest"`
	ContextPolicyRef           string `json:"context_policy_ref"`
	ContextPolicyDigest        string `json:"context_policy_digest"`
	InstitutionalEffectRuleRef string `json:"institutional_effect_rule_ref"`
}

// NewDecisionRecordInstitutedEffect accepts only exact decision content and a
// durable generic SpeechAct source. It rejects a valid source from another
// project, review subject, DecisionRecord, or institutional context.
func NewDecisionRecordInstitutedEffect(
	content DecisionBindingContent,
	source authority.RecordedSpeechActSource,
) (DecisionRecordInstitutedEffect, error) {
	state, err := canonicalDecisionRecordInstitutedEffect(content, source)
	if err != nil {
		return DecisionRecordInstitutedEffect{}, err
	}
	effect := DecisionRecordInstitutedEffect{state: &state}
	if !effect.valid() {
		return DecisionRecordInstitutedEffect{}, fmt.Errorf(
			"decision-record instituted effect is inconsistent",
		)
	}
	return effect, nil
}

func canonicalDecisionRecordInstitutedEffect(
	content DecisionBindingContent,
	source authority.RecordedSpeechActSource,
) (decisionRecordInstitutedEffectState, error) {
	bindings, err := exactDecisionSpeechActBindings(content, source)
	if err != nil {
		return decisionRecordInstitutedEffectState{}, err
	}
	effectRule, err := authority.NewInstitutionalEffectRuleRef(decisionEffectRuleValue)
	if err != nil {
		return decisionRecordInstitutedEffectState{}, err
	}
	state := decisionRecordInstitutedEffectState{
		content:                 content,
		source:                  source,
		projectRoot:             bindings.projectRoot,
		decisionRef:             bindings.decisionRef,
		contentRef:              bindings.contentRef,
		contentDigest:           bindings.contentDigest,
		speechActRef:            bindings.speechActRef,
		speechActDigest:         bindings.speechActDigest,
		contextPolicyRef:        bindings.contextPolicyRef,
		contextPolicyDigest:     bindings.contextPolicyDigest,
		institutionalEffectRule: effectRule,
		occurredAt:              bindings.occurredAt,
	}
	payload := decisionRecordEffectPayload(state)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return decisionRecordInstitutedEffectState{}, fmt.Errorf(
			"encode decision-record effect digest payload: %w",
			err,
		)
	}
	digestInput := append([]byte(decisionRecordEffectDigestDomain), payloadBytes...)
	digest, err := digestBytes(digestInput)
	if err != nil {
		return decisionRecordInstitutedEffectState{}, err
	}
	canonicalBytes, err := json.Marshal(decisionRecordEffectDTO(payload, digest))
	if err != nil {
		return decisionRecordInstitutedEffectState{}, fmt.Errorf(
			"encode decision-record instituted effect: %w",
			err,
		)
	}
	state.digest = digest
	state.canonicalBytes = canonicalBytes
	return state, nil
}

type exactDecisionSourceBindings struct {
	projectRoot         authority.ProjectRoot
	decisionRef         string
	contentRef          authority.SpeechActReviewSubjectRef
	contentDigest       authority.Digest
	speechActRef        authority.SpeechActRef
	speechActDigest     authority.Digest
	contextPolicyRef    authority.ContextPolicyRef
	contextPolicyDigest authority.Digest
	occurredAt          time.Time
}

func exactDecisionSpeechActBindings(
	content DecisionBindingContent,
	source authority.RecordedSpeechActSource,
) (exactDecisionSourceBindings, error) {
	if !content.valid() {
		return exactDecisionSourceBindings{}, fmt.Errorf(
			"decision-record effect requires exact decision-binding content",
		)
	}
	if !source.Valid() {
		return exactDecisionSourceBindings{}, fmt.Errorf(
			"decision-record effect requires a durable canonical SpeechAct source",
		)
	}
	contentRoot, contentRootOK := content.ProjectRoot()
	decisionRef, decisionRefOK := content.DecisionRef()
	contentRef, contentRefOK := content.ContentRef()
	contentDigest, contentDigestOK := content.Digest()
	sourceRoot, sourceRootOK := source.ProjectRoot()
	speechActRef, speechActRefOK := source.SpeechActRef()
	speechActDigest, speechActDigestOK := source.SpeechActDigest()
	preparedIntentDigest, preparedIntentDigestOK := source.PreparedIntentDigest()
	captureRef, captureRefOK := source.TerminalCaptureRef()
	reviewDigest, reviewDigestOK := source.ReviewDigest()
	reviewText, reviewTextOK := source.ReviewText()
	reviewSubjectRef, reviewSubjectRefOK := source.ReviewSubjectRef()
	reviewSubjectDigest, reviewSubjectDigestOK := source.ReviewSubjectDigest()
	institutedObjectRef, institutedObjectRefOK := source.InstitutedObjectRef()
	contextPolicyRef, contextPolicyRefOK := source.ContextPolicyRef()
	contextPolicyDigest, contextPolicyDigestOK := source.ContextPolicyDigest()
	occurredAt, occurredAtOK := source.OccurredAt()
	complete := contentRootOK && decisionRefOK && contentRefOK && contentDigestOK
	complete = complete && sourceRootOK && speechActRefOK && speechActDigestOK
	complete = complete && preparedIntentDigestOK && captureRefOK && reviewDigestOK
	complete = complete && reviewTextOK && reviewSubjectRefOK && reviewSubjectDigestOK
	complete = complete && institutedObjectRefOK && contextPolicyRefOK
	complete = complete && contextPolicyDigestOK && occurredAtOK
	if !complete {
		return exactDecisionSourceBindings{}, fmt.Errorf(
			"decision-record effect source bindings are incomplete",
		)
	}
	expectedIntent, err := PrepareDecisionSpeechActIntent(content)
	if err != nil {
		return exactDecisionSourceBindings{}, err
	}
	expectedIntentDigest, expectedIntentDigestOK := expectedIntent.Digest()
	if !expectedIntentDigestOK {
		return exactDecisionSourceBindings{}, fmt.Errorf(
			"prepared decision SpeechAct intent has no canonical digest",
		)
	}
	expectedPolicy, err := DecisionSpeechActContextPolicy()
	if err != nil {
		return exactDecisionSourceBindings{}, err
	}
	expectedPolicyRef, expectedPolicyRefOK := expectedPolicy.Ref()
	expectedPolicyDigest, expectedPolicyDigestOK := expectedPolicy.Digest()
	if !expectedPolicyRefOK || !expectedPolicyDigestOK {
		return exactDecisionSourceBindings{}, fmt.Errorf(
			"decision SpeechAct context policy is incomplete",
		)
	}
	reviewCard, err := content.ReviewCard()
	if err != nil {
		return exactDecisionSourceBindings{}, err
	}
	expectedReviewText, expectedReviewTextOK := reviewCard.Text()
	if !expectedReviewTextOK {
		return exactDecisionSourceBindings{}, fmt.Errorf(
			"decision review card has no canonical text",
		)
	}
	identity := strings.TrimPrefix(contentDigest.String(), "sha256:")
	expectedSpeechActRef := "speech-act:decision-binding:" + identity
	expectedCaptureRef := "carrier:terminal-capture:decision-binding:" + identity
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: sourceRoot.String() == contentRoot.String(), name: "project root"},
		{matches: institutedObjectRef.String() == decisionRef, name: "instituted DecisionRecord"},
		{matches: reviewSubjectRef.String() == contentRef.String(), name: "review subject ref"},
		{matches: reviewSubjectDigest.String() == contentDigest.String(), name: "review subject digest"},
		{matches: reviewText == expectedReviewText, name: "human review text"},
		{matches: preparedIntentDigest.String() == expectedIntentDigest.String(), name: "prepared intent"},
		{matches: speechActRef.String() == expectedSpeechActRef, name: "SpeechAct ref"},
		{matches: captureRef.String() == expectedCaptureRef, name: "terminal capture ref"},
		{matches: contextPolicyRef.String() == expectedPolicyRef.String(), name: "context policy ref"},
		{matches: contextPolicyDigest.String() == expectedPolicyDigest.String(), name: "context policy digest"},
		{matches: reviewDigest.String() != "", name: "review digest"},
		{matches: !occurredAt.IsZero(), name: "SpeechAct occurrence"},
	}
	for _, check := range checks {
		if !check.matches {
			return exactDecisionSourceBindings{}, fmt.Errorf(
				"durable decision SpeechAct source has another %s",
				check.name,
			)
		}
	}
	return exactDecisionSourceBindings{
		projectRoot:         contentRoot,
		decisionRef:         decisionRef,
		contentRef:          contentRef,
		contentDigest:       contentDigest,
		speechActRef:        speechActRef,
		speechActDigest:     speechActDigest,
		contextPolicyRef:    contextPolicyRef,
		contextPolicyDigest: contextPolicyDigest,
		occurredAt:          canonicalDecisionBindingTime(occurredAt),
	}, nil
}

func decisionRecordEffectPayload(
	state decisionRecordInstitutedEffectState,
) decisionRecordInstitutedEffectPayloadV1 {
	return decisionRecordInstitutedEffectPayloadV1{
		Schema:                     decisionRecordInstitutedEffectSchema,
		ProjectRoot:                state.projectRoot.String(),
		DecisionRef:                state.decisionRef,
		DecisionContentRef:         state.contentRef.String(),
		DecisionContentDigest:      state.contentDigest.String(),
		SpeechActRef:               state.speechActRef.String(),
		SpeechActDigest:            state.speechActDigest.String(),
		ContextPolicyRef:           state.contextPolicyRef.String(),
		ContextPolicyDigest:        state.contextPolicyDigest.String(),
		InstitutionalEffectRuleRef: state.institutionalEffectRule.String(),
	}
}

func decisionRecordEffectDTO(
	payload decisionRecordInstitutedEffectPayloadV1,
	digest authority.Digest,
) decisionRecordInstitutedEffectJSONV1 {
	return decisionRecordInstitutedEffectJSONV1{
		Schema:                     payload.Schema,
		EffectDigest:               digest.String(),
		ProjectRoot:                payload.ProjectRoot,
		DecisionRef:                payload.DecisionRef,
		DecisionContentRef:         payload.DecisionContentRef,
		DecisionContentDigest:      payload.DecisionContentDigest,
		SpeechActRef:               payload.SpeechActRef,
		SpeechActDigest:            payload.SpeechActDigest,
		ContextPolicyRef:           payload.ContextPolicyRef,
		ContextPolicyDigest:        payload.ContextPolicyDigest,
		InstitutionalEffectRuleRef: payload.InstitutionalEffectRuleRef,
	}
}

func (effect DecisionRecordInstitutedEffect) valid() bool {
	if effect.state == nil {
		return false
	}
	rebuilt, err := canonicalDecisionRecordInstitutedEffect(
		effect.state.content,
		effect.state.source,
	)
	if err != nil {
		return false
	}
	state := effect.state
	return state.projectRoot.String() == rebuilt.projectRoot.String() &&
		state.decisionRef == rebuilt.decisionRef &&
		state.contentRef.String() == rebuilt.contentRef.String() &&
		state.contentDigest.String() == rebuilt.contentDigest.String() &&
		state.speechActRef.String() == rebuilt.speechActRef.String() &&
		state.speechActDigest.String() == rebuilt.speechActDigest.String() &&
		state.contextPolicyRef.String() == rebuilt.contextPolicyRef.String() &&
		state.contextPolicyDigest.String() == rebuilt.contextPolicyDigest.String() &&
		state.institutionalEffectRule.String() == rebuilt.institutionalEffectRule.String() &&
		state.occurredAt.Equal(rebuilt.occurredAt) &&
		state.digest.String() == rebuilt.digest.String() &&
		slices.Equal(state.canonicalBytes, rebuilt.canonicalBytes)
}

func (effect DecisionRecordInstitutedEffect) Digest() (authority.Digest, bool) {
	if !effect.valid() {
		return authority.Digest{}, false
	}
	return effect.state.digest, true
}

func (effect DecisionRecordInstitutedEffect) CanonicalBytes() ([]byte, bool) {
	if !effect.valid() {
		return nil, false
	}
	return slices.Clone(effect.state.canonicalBytes), true
}

func (effect DecisionRecordInstitutedEffect) ProjectRoot() (authority.ProjectRoot, bool) {
	if !effect.valid() {
		return authority.ProjectRoot{}, false
	}
	return effect.state.projectRoot, true
}

func (effect DecisionRecordInstitutedEffect) DecisionRef() (string, bool) {
	if !effect.valid() {
		return "", false
	}
	return effect.state.decisionRef, true
}

func (effect DecisionRecordInstitutedEffect) ContentRef() (
	authority.SpeechActReviewSubjectRef,
	bool,
) {
	if !effect.valid() {
		return authority.SpeechActReviewSubjectRef{}, false
	}
	return effect.state.contentRef, true
}

func (effect DecisionRecordInstitutedEffect) ContentDigest() (authority.Digest, bool) {
	if !effect.valid() {
		return authority.Digest{}, false
	}
	return effect.state.contentDigest, true
}

func (effect DecisionRecordInstitutedEffect) SpeechActRef() (authority.SpeechActRef, bool) {
	if !effect.valid() {
		return authority.SpeechActRef{}, false
	}
	return effect.state.speechActRef, true
}

func (effect DecisionRecordInstitutedEffect) SpeechActDigest() (authority.Digest, bool) {
	if !effect.valid() {
		return authority.Digest{}, false
	}
	return effect.state.speechActDigest, true
}

func (effect DecisionRecordInstitutedEffect) ContextPolicyRef() (
	authority.ContextPolicyRef,
	bool,
) {
	if !effect.valid() {
		return authority.ContextPolicyRef{}, false
	}
	return effect.state.contextPolicyRef, true
}

func (effect DecisionRecordInstitutedEffect) ContextPolicyDigest() (
	authority.Digest,
	bool,
) {
	if !effect.valid() {
		return authority.Digest{}, false
	}
	return effect.state.contextPolicyDigest, true
}

func (effect DecisionRecordInstitutedEffect) InstitutionalEffectRuleRef() (
	authority.InstitutionalEffectRuleRef,
	bool,
) {
	if !effect.valid() {
		return authority.InstitutionalEffectRuleRef{}, false
	}
	return effect.state.institutionalEffectRule, true
}

func (effect DecisionRecordInstitutedEffect) OccurredAt() (time.Time, bool) {
	if !effect.valid() {
		return time.Time{}, false
	}
	return effect.state.occurredAt, true
}
