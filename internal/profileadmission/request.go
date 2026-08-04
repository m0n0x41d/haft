package profileadmission

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// ProfileDeclarationAdmissionRequest is the complete host input. The host
// supplies only a strong candidate. Authority resolution, expected revision,
// Prepared material, transaction facts, and raw JSON belong to the
// transaction adapter.
type ProfileDeclarationAdmissionRequest struct {
	candidate              projectprofile.ProfileDeclarationCandidateV1
	expectedLedgerRevision projectprofile.LedgerRevision
	expectedHeadPresent    bool
}

// NewProfileChangeAdmissionRequest creates the CAS-pinned request used only
// for an explicitly reviewed successor of an existing canonical profile.
func NewProfileChangeAdmissionRequest(
	candidate projectprofile.ProfileDeclarationCandidateV1,
	expectedLedgerRevision projectprofile.LedgerRevision,
) (ProfileDeclarationAdmissionRequest, error) {
	if expectedLedgerRevision.Value() == 0 {
		return ProfileDeclarationAdmissionRequest{}, fmt.Errorf(
			"profile change requires a non-zero predecessor ledger revision",
		)
	}
	if _, err := expectedLedgerRevision.Next(); err != nil {
		return ProfileDeclarationAdmissionRequest{}, err
	}
	request := ProfileDeclarationAdmissionRequest{
		candidate:              candidate,
		expectedLedgerRevision: expectedLedgerRevision,
		expectedHeadPresent:    true,
	}
	if err := validateProfileDeclarationAdmissionRequest(request); err != nil {
		return ProfileDeclarationAdmissionRequest{}, err
	}
	return request, nil
}

func NewProfileDeclarationAdmissionRequest(
	candidate projectprofile.ProfileDeclarationCandidateV1,
) (ProfileDeclarationAdmissionRequest, error) {
	request := ProfileDeclarationAdmissionRequest{
		candidate: candidate,
	}
	err := validateProfileDeclarationAdmissionRequest(request)
	if err != nil {
		return ProfileDeclarationAdmissionRequest{}, err
	}
	return request, nil
}

func (request ProfileDeclarationAdmissionRequest) Candidate() projectprofile.ProfileDeclarationCandidateV1 {
	return request.candidate
}

func (request ProfileDeclarationAdmissionRequest) ExpectedLedgerRevision() (
	projectprofile.LedgerRevision,
	bool,
) {
	return request.expectedLedgerRevision, request.expectedHeadPresent
}

func validateProfileDeclarationAdmissionRequest(
	request ProfileDeclarationAdmissionRequest,
) error {
	candidate := request.candidate
	_, err := projectprofile.NewProfileDeclarationCandidateV1(
		candidate.Payload(),
		candidate.Provenance(),
	)
	if err != nil {
		return fmt.Errorf("profile declaration candidate is invalid: %w", err)
	}
	return nil
}
