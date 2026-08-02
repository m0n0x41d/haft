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
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

type storedAuthorityRow struct {
	digest    string
	canonical string
}

type existingAuthority struct {
	plan             profiledeclarationpreparation.Plan
	basisDigest      projectprofile.ContentDigest
	resolutionDigest projectprofile.ContentDigest
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
	if probe.Policy().Mode() ==
		profiledeclarationpreparation.ModeAutomaticSupportedSingleton {
		return recoverExistingAutomaticAuthority(ctx, transaction, probe)
	}
	return recoverExistingHostRoutedAuthority(ctx, transaction, probe)
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
