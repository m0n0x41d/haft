package profileonboarding

import (
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// ProfileOnboardingWorkInput remains the public onboarding name for the
// lower, opaque reviewed declaration input. The canonical bytes, identity,
// and payload are owned by the preparation core so admission-side tests do
// not need to import the orchestration package.
type ProfileOnboardingWorkInput = profiledeclarationpreparation.ProfileOnboardingWorkInput
type ManualProfileScopeInput = profiledeclarationpreparation.ManualProfileScopeInput
type ManualProfileProposalInput = profiledeclarationpreparation.ManualProfileProposalInput
type ProfileChangeBasis = profiledeclarationpreparation.ProfileChangeBasis

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

func NewProfileChangeBasis(
	admissionRecordRef projectprofile.ProfileDeclarationAdmissionRecordRef,
	admissionRecordDigest projectprofile.ContentDigest,
	payloadDigest projectprofile.ContentDigest,
	ledgerRevision projectprofile.LedgerRevision,
	scopeID projectprofile.ScopeID,
	previousEntityRef string,
	nextEntityRef projectprofile.EntityRef,
) (ProfileChangeBasis, error) {
	return profiledeclarationpreparation.NewProfileChangeBasis(
		admissionRecordRef,
		admissionRecordDigest,
		payloadDigest,
		ledgerRevision,
		scopeID,
		previousEntityRef,
		nextEntityRef,
	)
}

func ProposeProfileEntityRelationChangeWorkInput(
	suggestion profiledetector.Suggestion,
	current projectprofile.ProfileDeclarationPayload,
	basis ProfileChangeBasis,
) ([]byte, error) {
	return profiledeclarationpreparation.
		ProposeProfileEntityRelationChangeWorkInput(
			suggestion,
			current,
			basis,
		)
}
