package profileonboarding

import "github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"

const (
	ProfileDeclarationModeExplicitHOnboard = profiledeclarationpreparation.ModeExplicitHOnboard
	ProfileDeclarationModeStrictSpeechAct  = profiledeclarationpreparation.ModeStrictSpeechAct
)

// ProfileDeclarationPolicy is the exact project-local authority-sufficiency
// selection. In the default branch, the dedicated CLI invocation is the sole
// human gate; this value does not pretend that Haft independently observed a
// U.SpeechAct. The strict branch still requires the real terminal adapter.
type ProfileDeclarationPolicy = profiledeclarationpreparation.Policy

func NewProfileDeclarationPolicy(
	mode string,
	configCarrierRef string,
	configCarrier []byte,
) (ProfileDeclarationPolicy, error) {
	return profiledeclarationpreparation.NewPolicy(
		mode,
		configCarrierRef,
		configCarrier,
	)
}
