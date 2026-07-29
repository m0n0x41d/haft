package profileonboarding

import (
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/profiledetector"
)

// ProfileOnboardingWorkInput remains the public onboarding name for the
// lower, opaque reviewed declaration input. The canonical bytes, identity,
// and payload are owned by the preparation core so admission-side tests do
// not need to import the orchestration package.
type ProfileOnboardingWorkInput = profiledeclarationpreparation.ProfileOnboardingWorkInput
type ManualProfileScopeInput = profiledeclarationpreparation.ManualProfileScopeInput
type ManualProfileProposalInput = profiledeclarationpreparation.ManualProfileProposalInput

func DecodeProfileOnboardingWorkInput(
	data []byte,
	suggestion profiledetector.Suggestion,
) (ProfileOnboardingWorkInput, error) {
	return profiledeclarationpreparation.DecodeProfileOnboardingWorkInput(
		data,
		suggestion,
	)
}

func ProposeProfileOnboardingWorkInput(
	suggestion profiledetector.Suggestion,
) ([]byte, error) {
	return profiledeclarationpreparation.ProposeProfileOnboardingWorkInput(
		suggestion,
	)
}

func ProposeManualProfileOnboardingWorkInput(
	suggestion profiledetector.Suggestion,
	proposal ManualProfileProposalInput,
) ([]byte, error) {
	return profiledeclarationpreparation.
		ProposeManualProfileOnboardingWorkInput(
			suggestion,
			proposal,
		)
}
