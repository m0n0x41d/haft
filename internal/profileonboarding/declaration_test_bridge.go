package profileonboarding

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/profileadmission"
	profiledeclarationpreparationsqlite "github.com/m0n0x41d/haft/internal/profiledeclarationpreparation/sqlite"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

// ProfileDeclarationAdmissionTestFixture is the exact pre-admission result of
// production v3 authority and performed profile-declaration Work. It exposes
// only semantic addresses needed by cross-package fault/replay tests.
type ProfileDeclarationAdmissionTestFixture struct {
	request          profileadmission.ProfileDeclarationAdmissionRequest
	workInputRef     projectprofile.WorkInputRef
	workInputDigest  projectprofile.ContentDigest
	basisRef         projectprofile.ProfileDeclarationAuthorityBasisRef
	basisDigest      projectprofile.ContentDigest
	resolutionRef    string
	resolutionDigest projectprofile.ContentDigest
}

func (fixture ProfileDeclarationAdmissionTestFixture) AdmissionRequest() (
	profileadmission.ProfileDeclarationAdmissionRequest,
	bool,
) {
	candidate := fixture.request.Candidate()
	_, err := projectprofile.NewProfileDeclarationCandidateV1(
		candidate.Payload(),
		candidate.Provenance(),
	)
	return fixture.request, err == nil
}

func (fixture ProfileDeclarationAdmissionTestFixture) WorkInput() (
	projectprofile.WorkInputRef,
	projectprofile.ContentDigest,
) {
	return fixture.workInputRef, fixture.workInputDigest
}

func (fixture ProfileDeclarationAdmissionTestFixture) AuthorityBasis() (
	projectprofile.ProfileDeclarationAuthorityBasisRef,
	projectprofile.ContentDigest,
) {
	return fixture.basisRef, fixture.basisDigest
}

func (fixture ProfileDeclarationAdmissionTestFixture) AuthorityResolution() (
	string,
	projectprofile.ContentDigest,
) {
	return fixture.resolutionRef, fixture.resolutionDigest
}

// PrepareProfileDeclarationAdmissionForTestFixture traverses the production
// source path through exact Work/value-DAG persistence and stops immediately
// before admission. testing.TB is an unforgeable test-only capability for
// ordinary callers, and the runtime guard keeps this bridge unavailable from
// production binaries.
func PrepareProfileDeclarationAdmissionForTestFixture(
	t testing.TB,
	ctx context.Context,
	database *sql.DB,
	projectRoot string,
	input ProfileOnboardingWorkInput,
	policy ProfileDeclarationPolicy,
	clock func() time.Time,
) (ProfileDeclarationAdmissionTestFixture, error) {
	t.Helper()
	if !testing.Testing() {
		return ProfileDeclarationAdmissionTestFixture{}, fmt.Errorf(
			"profile declaration fixture is unavailable outside go test",
		)
	}
	if ctx == nil || database == nil || !input.Valid() || clock == nil {
		return ProfileDeclarationAdmissionTestFixture{}, fmt.Errorf(
			"profile declaration fixture requires context, database, exact input, and clock",
		)
	}
	if policy.Mode() != ProfileDeclarationModeExplicitHOnboard {
		return ProfileDeclarationAdmissionTestFixture{}, fmt.Errorf(
			"profile declaration fixture supports only explicit_h_onboard v3 authority",
		)
	}
	preparation, err := profiledeclarationpreparationsqlite.PrepareBeforeAdmission(
		ctx,
		database,
		projectRoot,
		input,
		policy,
		clock,
		func(ctx context.Context) error {
			transaction, err := sqlitetransaction.BeginImmediate(ctx, database)
			if err != nil {
				return err
			}
			finish := transaction.Rollback(ctx)
			return finish.Err()
		},
	)
	if err != nil {
		return ProfileDeclarationAdmissionTestFixture{}, err
	}
	if detail, conflict := preparation.ConflictDetail(); conflict {
		return ProfileDeclarationAdmissionTestFixture{}, fmt.Errorf(
			"profile declaration fixture preparation conflict: %s",
			detail,
		)
	}
	prepared, ok := preparation.Prepared()
	if !ok {
		return ProfileDeclarationAdmissionTestFixture{}, fmt.Errorf(
			"profile declaration fixture preparation omitted its sealed result",
		)
	}
	candidate, ok := prepared.Candidate()
	if !ok {
		return ProfileDeclarationAdmissionTestFixture{}, fmt.Errorf(
			"profile declaration fixture Work omitted its typed candidate",
		)
	}
	request, err := profileadmission.NewProfileDeclarationAdmissionRequest(candidate)
	if err != nil {
		return ProfileDeclarationAdmissionTestFixture{}, err
	}
	workInputRef, workInputDigest, ok := prepared.WorkInput()
	if !ok {
		return ProfileDeclarationAdmissionTestFixture{}, fmt.Errorf(
			"profile declaration fixture omitted its exact Work input",
		)
	}
	basisRef, basisDigest, ok := prepared.AuthorityBasis()
	if !ok {
		return ProfileDeclarationAdmissionTestFixture{}, fmt.Errorf(
			"profile declaration fixture omitted its authority basis",
		)
	}
	resolutionRef, resolutionDigest, ok := prepared.AuthorityResolution()
	if !ok {
		return ProfileDeclarationAdmissionTestFixture{}, fmt.Errorf(
			"profile declaration fixture omitted its authority resolution",
		)
	}
	return ProfileDeclarationAdmissionTestFixture{
		request:          request,
		workInputRef:     workInputRef,
		workInputDigest:  workInputDigest,
		basisRef:         basisRef,
		basisDigest:      basisDigest,
		resolutionRef:    resolutionRef,
		resolutionDigest: resolutionDigest,
	}, nil
}
