package sqlite

import (
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

// OutcomeKind is the closed preparation result algebra. Preparation either
// commits one new exact source closure, recovers the same closure, or reports
// an immutable collision. A conflict never exposes an admission candidate.
type OutcomeKind string

const (
	OutcomePreparedNew   OutcomeKind = "prepared_new"
	OutcomeExactExisting OutcomeKind = "exact_existing"
	OutcomeConflict      OutcomeKind = "conflict"
)

// Outcome is sealed to this package so callers cannot manufacture another
// preparation state that bypasses exact reread or conflict handling.
type Outcome interface {
	Kind() OutcomeKind
	Prepared() (Prepared, bool)
	ConflictDetail() (string, bool)
	isPreparationOutcome()
}

type preparedState struct {
	plan             profiledeclarationpreparation.Plan
	values           profiledeclarationpreparation.ValueSet
	candidate        projectprofile.ProfileDeclarationCandidateV1
	workInputRef     projectprofile.WorkInputRef
	workInputDigest  projectprofile.ContentDigest
	basisRef         projectprofile.ProfileDeclarationAuthorityBasisRef
	basisDigest      projectprofile.ContentDigest
	resolutionRef    string
	resolutionDigest projectprofile.ContentDigest
}

// Prepared is an exact pre-admission snapshot. It grants no authority to bind
// a project profile and cannot write an admission, authority use, or revision.
type Prepared struct {
	state *preparedState
}

func (prepared Prepared) Candidate() (
	projectprofile.ProfileDeclarationCandidateV1,
	bool,
) {
	if prepared.state == nil {
		return projectprofile.ProfileDeclarationCandidateV1{}, false
	}
	candidate := prepared.state.candidate
	_, err := projectprofile.NewProfileDeclarationCandidateV1(
		candidate.Payload(),
		candidate.Provenance(),
	)
	return candidate, err == nil
}

func (prepared Prepared) WorkInput() (
	projectprofile.WorkInputRef,
	projectprofile.ContentDigest,
	bool,
) {
	if prepared.state == nil {
		return projectprofile.WorkInputRef{}, projectprofile.ContentDigest{}, false
	}
	return prepared.state.workInputRef, prepared.state.workInputDigest, true
}

func (prepared Prepared) AuthorityBasis() (
	projectprofile.ProfileDeclarationAuthorityBasisRef,
	projectprofile.ContentDigest,
	bool,
) {
	if prepared.state == nil {
		return projectprofile.ProfileDeclarationAuthorityBasisRef{}, projectprofile.ContentDigest{}, false
	}
	return prepared.state.basisRef, prepared.state.basisDigest, true
}

func (prepared Prepared) AuthorityResolution() (
	string,
	projectprofile.ContentDigest,
	bool,
) {
	if prepared.state == nil {
		return "", projectprofile.ContentDigest{}, false
	}
	return prepared.state.resolutionRef, prepared.state.resolutionDigest, true
}

func (prepared Prepared) Plan() (
	profiledeclarationpreparation.Plan,
	bool,
) {
	if prepared.state == nil {
		return profiledeclarationpreparation.Plan{}, false
	}
	return prepared.state.plan, true
}

func (prepared Prepared) Values() (
	profiledeclarationpreparation.ValueSet,
	bool,
) {
	if prepared.state == nil {
		return profiledeclarationpreparation.ValueSet{}, false
	}
	return prepared.state.values, true
}

type PreparedNew struct{ prepared Prepared }

func (PreparedNew) Kind() OutcomeKind { return OutcomePreparedNew }
func (value PreparedNew) Prepared() (Prepared, bool) {
	return value.prepared, value.prepared.state != nil
}
func (PreparedNew) ConflictDetail() (string, bool) { return "", false }
func (PreparedNew) isPreparationOutcome()          {}

type ExactExisting struct{ prepared Prepared }

func (ExactExisting) Kind() OutcomeKind { return OutcomeExactExisting }
func (value ExactExisting) Prepared() (Prepared, bool) {
	return value.prepared, value.prepared.state != nil
}
func (ExactExisting) ConflictDetail() (string, bool) { return "", false }
func (ExactExisting) isPreparationOutcome()          {}

type Conflict struct{ detail string }

func (Conflict) Kind() OutcomeKind                    { return OutcomeConflict }
func (Conflict) Prepared() (Prepared, bool)           { return Prepared{}, false }
func (value Conflict) ConflictDetail() (string, bool) { return value.detail, value.detail != "" }
func (Conflict) isPreparationOutcome()                {}

func newPrepared(
	plan profiledeclarationpreparation.Plan,
	values profiledeclarationpreparation.ValueSet,
	candidate projectprofile.ProfileDeclarationCandidateV1,
	basisDigest projectprofile.ContentDigest,
	resolutionDigest projectprofile.ContentDigest,
) (Prepared, error) {
	basisRef, err := plan.AuthorityBasisRef()
	if err != nil {
		return Prepared{}, err
	}
	input := plan.Input()
	state := &preparedState{
		plan:             plan,
		values:           values,
		candidate:        candidate,
		workInputRef:     input.Ref(),
		workInputDigest:  input.Digest(),
		basisRef:         basisRef,
		basisDigest:      basisDigest,
		resolutionRef:    plan.AuthorityResolutionRef(),
		resolutionDigest: resolutionDigest,
	}
	return Prepared{state: state}, nil
}
