package profileonboarding

import (
	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
)

const (
	ProfileDeclarationModeHostRoutedOperatorRequest   = profiledeclarationpreparation.ModeHostRoutedOperatorRequest
	ProfileDeclarationModeAutomaticSupportedSingleton = profiledeclarationpreparation.ModeAutomaticSupportedSingleton
)

// ProfileDeclarationPolicy records either host-routed operator-request
// provenance or the separate deterministic singleton bootstrap policy.
type ProfileDeclarationPolicy = profiledeclarationpreparation.Policy

func NewProfileDeclarationPolicy(
	request operatorrequest.Request,
) (ProfileDeclarationPolicy, error) {
	return profiledeclarationpreparation.NewHostRoutedOperatorRequestPolicy(
		request,
	)
}

func NewProfileChangePolicy(
	request operatorrequest.Request,
) (ProfileDeclarationPolicy, error) {
	return profiledeclarationpreparation.NewHostRoutedProfileChangePolicy(
		request,
	)
}
