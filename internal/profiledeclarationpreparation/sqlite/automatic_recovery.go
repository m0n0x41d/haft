package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

func recoverExistingAutomaticAuthority(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	probe profiledeclarationpreparation.Plan,
) (existingAuthority, authorityPresence, string, error) {
	basisRef, err := probe.AuthorityBasisRef()
	if err != nil {
		return existingAuthority{}, authorityAbsent, "", err
	}
	basis, basisFound, err := loadStoredAuthorityRow(
		ctx,
		transaction,
		`SELECT basis_digest, canonical_json
		 FROM profile_initial_bootstrap_authority_bases_v1
		 WHERE basis_ref = ?`,
		basisRef.String(),
	)
	if err != nil {
		return existingAuthority{}, authorityAbsent, "", err
	}
	resolution, resolutionFound, err := loadStoredAuthorityRow(
		ctx,
		transaction,
		`SELECT authority_resolution_digest, canonical_json
		 FROM profile_initial_bootstrap_authority_resolutions_v1
		 WHERE authority_resolution_ref = ?`,
		probe.AuthorityResolutionRef(),
	)
	if err != nil {
		return existingAuthority{}, authorityAbsent, "", err
	}
	if !basisFound && !resolutionFound {
		return existingAuthority{}, authorityAbsent, "", nil
	}
	if basisFound != resolutionFound {
		return existingAuthority{}, authorityConflict,
			"automatic profile authority basis and resolution are not an atomic pair", nil
	}
	resolutionDTO := automaticAuthorityResolutionJSON{}
	if err := decodeExactJSON(
		[]byte(resolution.canonical),
		&resolutionDTO,
	); err != nil {
		return existingAuthority{}, authorityConflict, err.Error(), nil
	}
	checkedAt, err := time.Parse(time.RFC3339Nano, resolutionDTO.CheckedAt)
	if err != nil {
		return existingAuthority{}, authorityConflict,
			fmt.Sprintf("automatic profile authority checked_at is invalid: %v", err), nil
	}
	plan, err := profiledeclarationpreparation.NewPlan(
		probe.Root().String(),
		probe.Input(),
		probe.Policy(),
		checkedAt,
	)
	if err != nil {
		return existingAuthority{}, authorityConflict, err.Error(), nil
	}
	durableInput, err := LoadProfileOnboardingWorkInput(
		ctx,
		transaction,
		plan.Input().Ref().String(),
		plan.Input().Digest().String(),
	)
	if err != nil {
		return existingAuthority{}, authorityConflict, err.Error(), nil
	}
	if string(durableInput.CanonicalJSON()) != string(plan.Input().CanonicalJSON()) {
		return existingAuthority{}, authorityConflict,
			"durable automatic authority binds another detector WorkInput", nil
	}
	bindingDigest, err := loadProjectBindingDigest(ctx, transaction, plan.Root())
	if err != nil {
		return existingAuthority{}, authorityAbsent, "", err
	}
	expectedBasis, expectedResolution, err := buildAutomaticAuthorityRows(
		plan,
		bindingDigest,
	)
	if err != nil {
		return existingAuthority{}, authorityConflict, err.Error(), nil
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: basis.digest == expectedBasis.digest.String(), name: "basis digest"},
		{matches: basis.canonical == string(expectedBasis.canonical), name: "basis canonical JSON"},
		{matches: resolution.digest == expectedResolution.digest.String(), name: "resolution digest"},
		{matches: resolution.canonical == string(expectedResolution.canonical), name: "resolution canonical JSON"},
	}
	for _, check := range checks {
		if !check.matches {
			return existingAuthority{}, authorityConflict,
				"existing automatic profile authority differs at " + check.name, nil
		}
	}
	return existingAuthority{
		plan:             plan,
		basisDigest:      expectedBasis.digest,
		resolutionDigest: expectedResolution.digest,
	}, authorityExact, "", nil
}
