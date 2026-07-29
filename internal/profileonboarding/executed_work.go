package profileonboarding

import "github.com/m0n0x41d/haft/internal/projectprofile"

type executedHaftSoftwareOnboardingState struct {
	candidate projectprofile.ProfileDeclarationCandidateV1
}

// ExecutedHaftSoftwareOnboarding is a typed result of actual post-resolution
// Work. It is not authority and is not a canonical profile admission.
type ExecutedHaftSoftwareOnboarding struct {
	state *executedHaftSoftwareOnboardingState
}

func (executed ExecutedHaftSoftwareOnboarding) Candidate() (
	projectprofile.ProfileDeclarationCandidateV1,
	bool,
) {
	if executed.state == nil {
		return projectprofile.ProfileDeclarationCandidateV1{}, false
	}
	candidate := executed.state.candidate
	_, err := projectprofile.NewProfileDeclarationCandidateV1(
		candidate.Payload(),
		candidate.Provenance(),
	)
	if err != nil {
		return projectprofile.ProfileDeclarationCandidateV1{}, false
	}
	return candidate, true
}
