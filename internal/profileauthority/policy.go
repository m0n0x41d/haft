package profileauthority

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
)

const (
	profileDeclarationActionValue = "profile.declare.from_onboarding_candidate"
	profileContextPolicyValue     = "context-policy:profile-declaration-authorization:v2"
	profileBoundedContextValue    = "bounded-context:profile-declaration-authority"
	profileActTypeValue           = "speech-act-type:authorize-profile-declaration"
	profileEffectRuleValue        = "institution-rule:authorize-institutes-profile-permission-may:v2"
	profileUtteranceValue         = "utterance:profile-declaration-authorization:v2"
	profileAuthorizationVerb      = "AUTHORIZE"
	profileAuthorizationLiteral   = "REVIEWED PROJECT PROFILE"
)

func ActionKind() (authority.ActionKind, error) {
	return authority.NewActionKind(profileDeclarationActionValue)
}

// AuthorizationPhrase is policy-owned semantic input. It contains no ref,
// digest, nonce, or other machine address that the operator must transcribe.
func AuthorizationPhrase() string {
	return profileAuthorizationVerb + " " + profileAuthorizationLiteral
}

func ContextPolicy() (authority.SpeechActContextPolicy, error) {
	policyRef, err := authority.NewContextPolicyRef(profileContextPolicyValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	boundedContext, err := authority.NewBoundedContextRef(profileBoundedContextValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	actType, err := authority.NewSpeechActTypeRef(profileActTypeValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	effectRule, err := profileInstitutionalEffectRule()
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	return authority.NewSpeechActContextPolicy(
		policyRef,
		boundedContext,
		actType,
		effectRule,
	)
}

func profileInstitutionalEffectRule() (authority.InstitutionalEffectRule, error) {
	ruleRef, err := authority.NewInstitutionalEffectRuleRef(profileEffectRuleValue)
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	objectKind, err := authority.NewInstitutedObjectKind("U.Commitment")
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	modality, err := authority.NewInstitutionalModality("MAY")
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	action, err := ActionKind()
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	utteranceRule, err := authority.NewLiteralSpeechActUtteranceRule(
		profileAuthorizationVerb,
		profileAuthorizationLiteral,
	)
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	utterance, err := authority.NewUtteranceRef(profileUtteranceValue)
	if err != nil {
		return authority.InstitutionalEffectRule{}, err
	}
	rule, err := authority.NewInstitutionalEffectRule(
		ruleRef,
		objectKind,
		modality,
		action,
		utteranceRule,
		utterance,
	)
	if err != nil {
		return authority.InstitutionalEffectRule{}, fmt.Errorf(
			"build profile authorization institutional rule: %w",
			err,
		)
	}
	return rule, nil
}
