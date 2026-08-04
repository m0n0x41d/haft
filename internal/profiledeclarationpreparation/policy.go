package profiledeclarationpreparation

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/profiledetector"
)

const (
	ModeHostRoutedOperatorRequest   = "host_routed_operator_request"
	ModeAutomaticSupportedSingleton = "automatic_supported_singleton_init"

	ActionHostRoutedProfileDeclaration = "profile.declare.from_onboarding_candidate"
	ActionAutomaticSupportedSingleton  = "profile.apply_supported_singleton_default"
	ResolutionHostRoutedRequest        = "host_routed_request_acceptance"
	ResolutionAutomaticPolicy          = "deterministic_policy_satisfaction"
)

// Policy is the closed provenance basis for profile declaration. The
// host-routed branch records what the host recognized; it does not claim an
// independently proven SpeechAct or hidden operator intent.
type Policy struct {
	mode      string
	request   operatorrequest.Request
	automatic automaticPolicyProvenance
}

type automaticPolicyProvenance struct {
	detectorVersion   string
	policyVersion     string
	suggestionRef     string
	observationDigest string
}

func NewHostRoutedOperatorRequestPolicy(
	request operatorrequest.Request,
) (Policy, error) {
	if request.Provenance() != operatorrequest.HostRoutedOperatorRequest ||
		request.Effect() != operatorrequest.ProfileDeclaration ||
		request.Ref() == "" ||
		request.Digest() == "" {
		return Policy{}, fmt.Errorf(
			"profile declaration requires exact host-routed operator-request provenance",
		)
	}
	return Policy{mode: ModeHostRoutedOperatorRequest, request: request}, nil
}

// NewHostRoutedProfileChangePolicy seals a separate operator effect for
// changing one relation in an already-canonical profile. It cannot be reused
// as authority for initial declaration.
func NewHostRoutedProfileChangePolicy(
	request operatorrequest.Request,
) (Policy, error) {
	if request.Provenance() != operatorrequest.HostRoutedOperatorRequest ||
		request.Effect() != operatorrequest.ProfileChange ||
		request.Ref() == "" ||
		request.Digest() == "" {
		return Policy{}, fmt.Errorf(
			"profile change requires exact host-routed operator-request provenance",
		)
	}
	return Policy{mode: ModeHostRoutedOperatorRequest, request: request}, nil
}

// NewAutomaticSupportedSingletonPolicy seals the exact detector result that
// satisfied the automatic-init policy. It cannot represent mixed, truncated,
// weak, or multi-scope observations.
func NewAutomaticSupportedSingletonPolicy(
	suggestion profiledetector.Suggestion,
) (Policy, error) {
	if suggestion.Snapshot().Truncated() ||
		suggestion.ConfidencePosture() != profiledetector.SupportedConfidence ||
		len(suggestion.SuggestedScopes()) != 1 {
		return Policy{}, fmt.Errorf(
			"automatic profile policy requires a complete supported singleton detector result",
		)
	}
	return Policy{
		mode: ModeAutomaticSupportedSingleton,
		automatic: automaticPolicyProvenance{
			detectorVersion:   suggestion.DetectorVersion(),
			policyVersion:     profiledetector.PolicyVersion,
			suggestionRef:     suggestion.SuggestionRef(),
			observationDigest: suggestion.Snapshot().ObservationDigest(),
		},
	}, nil
}

func (policy Policy) Mode() string { return policy.mode }

func (policy Policy) OperatorRequest() (operatorrequest.Request, bool) {
	if policy.mode != ModeHostRoutedOperatorRequest || policy.request.Ref() == "" {
		return operatorrequest.Request{}, false
	}
	return policy.request, true
}

func (policy Policy) AutomaticProvenance() (
	detectorVersion string,
	policyVersion string,
	suggestionRef string,
	observationDigest string,
	ok bool,
) {
	if policy.mode != ModeAutomaticSupportedSingleton {
		return "", "", "", "", false
	}
	value := policy.automatic
	valid := value.detectorVersion != "" &&
		value.policyVersion != "" &&
		value.suggestionRef != "" &&
		value.observationDigest != ""
	return value.detectorVersion,
		value.policyVersion,
		value.suggestionRef,
		value.observationDigest,
		valid
}
