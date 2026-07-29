package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type storedAuthorityRow struct {
	digest    string
	canonical string
}

type existingAuthority struct {
	plan       profiledeclarationpreparation.Plan
	basis      authorityBasisRowV3
	resolution authorityResolutionRowV3
}

type authorityPresence string

const (
	authorityAbsent   authorityPresence = "absent"
	authorityExact    authorityPresence = "exact"
	authorityConflict authorityPresence = "conflict"
)

func recoverExistingAuthority(
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
		 FROM profile_declaration_authority_bases_v3
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
		 FROM profile_declaration_authority_resolutions_v3
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
			"v3 profile authority basis and resolution are not an atomic pair", nil
	}
	resolutionDTO := authorityResolutionJSONV3{}
	if err := decodeExactJSON([]byte(resolution.canonical), &resolutionDTO); err != nil {
		return existingAuthority{}, authorityConflict, err.Error(), nil
	}
	checkedAt, err := time.Parse(time.RFC3339Nano, resolutionDTO.CheckedAt)
	if err != nil {
		return existingAuthority{}, authorityConflict,
			fmt.Sprintf("v3 profile authority checked_at is invalid: %v", err), nil
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
			"durable v3 profile authority binds another reviewed Work input", nil
	}
	bindingDigest, err := loadProjectBindingDigest(ctx, transaction, plan.Root())
	if err != nil {
		return existingAuthority{}, authorityAbsent, "", err
	}
	expectedBasis, expectedResolution, err := buildAuthorityRowsV3(plan, bindingDigest)
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
				"existing v3 profile declaration authority differs at " + check.name, nil
		}
	}
	return existingAuthority{
		plan:       plan,
		basis:      expectedBasis,
		resolution: expectedResolution,
	}, authorityExact, "", nil
}

func loadStoredAuthorityRow(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	query string,
	ref string,
) (storedAuthorityRow, bool, error) {
	row := storedAuthorityRow{}
	err := transaction.ScanOne(
		ctx,
		query,
		[]any{ref},
		[]any{&row.digest, &row.canonical},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return storedAuthorityRow{}, false, nil
	}
	if err != nil {
		return storedAuthorityRow{}, false, fmt.Errorf(
			"load v3 profile declaration authority: %w",
			err,
		)
	}
	return row, true, nil
}

func decodeExactJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode v3 profile authority JSON: %w", err)
	}
	extra := json.RawMessage{}
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return fmt.Errorf("v3 profile authority JSON contains multiple values")
	}
	if err != nil {
		return fmt.Errorf("decode trailing v3 profile authority JSON: %w", err)
	}
	return fmt.Errorf("v3 profile authority JSON contains multiple values")
}

func loadExistingOccurrenceTimes(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	plan profiledeclarationpreparation.Plan,
) (profiledeclarationpreparation.OccurrenceTimes, bool, error) {
	ref, err := plan.WorkRecordRef()
	if err != nil {
		return profiledeclarationpreparation.OccurrenceTimes{}, false, err
	}
	workFromRaw := ""
	basisFromRaw := ""
	basisUntilRaw := ""
	workUntilRaw := ""
	err = transaction.ScanOne(
		ctx,
		`SELECT work_from, basis_observation_from, basis_observation_until, work_until
		 FROM profile_onboarding_work_records
		 WHERE work_record_ref = ? AND project_root = ?`,
		[]any{ref.String(), plan.Root().String()},
		[]any{&workFromRaw, &basisFromRaw, &basisUntilRaw, &workUntilRaw},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return profiledeclarationpreparation.OccurrenceTimes{}, false, nil
	}
	if err != nil {
		return profiledeclarationpreparation.OccurrenceTimes{}, false, err
	}
	raw := []string{workFromRaw, basisFromRaw, basisUntilRaw, workUntilRaw}
	parsed := make([]time.Time, len(raw))
	for index, value := range raw {
		parsed[index], err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return profiledeclarationpreparation.OccurrenceTimes{}, false, fmt.Errorf(
				"parse durable profile-onboarding Work chronology: %w",
				err,
			)
		}
	}
	times, err := profiledeclarationpreparation.NewOccurrenceTimes(
		parsed[0],
		parsed[1],
		parsed[2],
		parsed[3],
	)
	if err != nil {
		return profiledeclarationpreparation.OccurrenceTimes{}, false, err
	}
	return times, true, nil
}
