package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const insertProfileOnboardingWorkInputSQL = `INSERT INTO profile_onboarding_work_inputs_v1 (
	work_input_ref, work_input_digest, project_root,
	suggestion_ref, detector_version, policy_version, observation_digest,
	profile_payload_json, profile_payload_digest, canonical_json, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(work_input_ref) DO NOTHING`

const selectProfileOnboardingWorkInputSQL = `SELECT
	work_input_digest, project_root, suggestion_ref, detector_version,
	policy_version, observation_digest, profile_payload_json,
	profile_payload_digest, canonical_json, recorded_at
FROM profile_onboarding_work_inputs_v1
WHERE work_input_ref = ?`

const selectProfileOnboardingWorkInputDigestSQL = `SELECT work_input_digest
FROM profile_onboarding_work_inputs_v1
WHERE work_input_ref = ?`

type durableProfileOnboardingWorkInputRow struct {
	digest            string
	projectRoot       string
	suggestionRef     string
	detectorVersion   string
	policyVersion     string
	observationDigest string
	payloadJSON       string
	payloadDigest     string
	canonicalJSON     string
	recordedAt        string
}

// storeAndReloadProfileOnboardingWorkInput persists the exact orchestration
// input before authority resolution and performed Work. The caller owns the
// transaction and therefore composes it atomically with the authority basis.
func StoreAndReloadProfileOnboardingWorkInput(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	input profiledeclarationpreparation.ProfileOnboardingWorkInput,
	recordedAt time.Time,
) (profiledeclarationpreparation.ProfileOnboardingWorkInput, error) {
	if ctx == nil || !input.Valid() {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, fmt.Errorf(
			"profile onboarding Work-input storage requires context and a sealed input",
		)
	}
	if err := transaction.RequireImmediate(); err != nil {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, err
	}
	canonicalTime := recordedAt.UTC().Round(0)
	if canonicalTime.IsZero() {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, fmt.Errorf(
			"profile onboarding Work-input recorded_at is required",
		)
	}
	existing, err := loadExistingProfileOnboardingWorkInput(
		ctx,
		transaction,
		input,
	)
	if err != nil {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, err
	}
	if existing.Valid() {
		return existing, nil
	}
	arguments := []any{
		input.Ref().String(),
		input.Digest().String(),
		input.ProjectRoot().String(),
		input.SuggestionRef(),
		input.DetectorVersion(),
		input.PolicyVersion(),
		input.ObservationDigest(),
		string(input.PayloadCanonicalJSON()),
		input.PayloadDigest().String(),
		string(input.CanonicalJSON()),
		canonicalTime.Format(time.RFC3339Nano),
	}
	_, err = transaction.Execute(
		ctx,
		insertProfileOnboardingWorkInputSQL,
		arguments,
	)
	if err != nil {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, fmt.Errorf(
			"persist profile onboarding Work input: %w",
			err,
		)
	}
	return LoadProfileOnboardingWorkInput(
		ctx,
		transaction,
		input.Ref().String(),
		input.Digest().String(),
	)
}

func loadExistingProfileOnboardingWorkInput(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	input profiledeclarationpreparation.ProfileOnboardingWorkInput,
) (profiledeclarationpreparation.ProfileOnboardingWorkInput, error) {
	storedDigest := ""
	err := transaction.ScanOne(
		ctx,
		selectProfileOnboardingWorkInputDigestSQL,
		[]any{input.Ref().String()},
		[]any{&storedDigest},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, nil
	}
	if err != nil {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, fmt.Errorf(
			"inspect existing profile onboarding Work input: %w",
			err,
		)
	}
	return LoadProfileOnboardingWorkInput(
		ctx,
		transaction,
		input.Ref().String(),
		input.Digest().String(),
	)
}

func LoadProfileOnboardingWorkInput(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref string,
	expectedDigest string,
) (profiledeclarationpreparation.ProfileOnboardingWorkInput, error) {
	row := durableProfileOnboardingWorkInputRow{}
	destinations := []any{
		&row.digest,
		&row.projectRoot,
		&row.suggestionRef,
		&row.detectorVersion,
		&row.policyVersion,
		&row.observationDigest,
		&row.payloadJSON,
		&row.payloadDigest,
		&row.canonicalJSON,
		&row.recordedAt,
	}
	err := transaction.ScanOne(
		ctx,
		selectProfileOnboardingWorkInputSQL,
		[]any{ref},
		destinations,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, fmt.Errorf(
			"profile onboarding Work input %q is absent",
			ref,
		)
	}
	if err != nil {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, fmt.Errorf(
			"reload profile onboarding Work input: %w",
			err,
		)
	}
	input, err := profiledeclarationpreparation.DecodeCanonicalProfileOnboardingWorkInput(
		[]byte(row.canonicalJSON),
	)
	if err != nil {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, fmt.Errorf(
			"decode durable profile onboarding Work input: %w",
			err,
		)
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: input.Ref().String() == ref, name: "ref"},
		{matches: input.Digest().String() == expectedDigest, name: "expected digest"},
		{matches: input.Digest().String() == row.digest, name: "stored digest"},
		{matches: input.ProjectRoot().String() == row.projectRoot, name: "project root"},
		{matches: input.SuggestionRef() == row.suggestionRef, name: "suggestion ref"},
		{matches: input.DetectorVersion() == row.detectorVersion, name: "detector version"},
		{matches: input.PolicyVersion() == row.policyVersion, name: "policy version"},
		{matches: input.ObservationDigest() == row.observationDigest, name: "observation digest"},
		{matches: string(input.PayloadCanonicalJSON()) == row.payloadJSON, name: "payload JSON"},
		{matches: input.PayloadDigest().String() == row.payloadDigest, name: "payload digest"},
	}
	for _, check := range checks {
		if !check.matches {
			return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, fmt.Errorf(
				"durable profile onboarding Work input has mismatched %s",
				check.name,
			)
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, row.recordedAt); err != nil {
		return profiledeclarationpreparation.ProfileOnboardingWorkInput{}, fmt.Errorf(
			"durable profile onboarding Work input recorded_at is invalid: %w",
			err,
		)
	}
	return input, nil
}
