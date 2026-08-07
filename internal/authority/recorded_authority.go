package authority

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const reconciledAuthoritySchemaVersion = 36

type recordedAuthorityState struct {
	presentation canonicalPresentation
	resolution   canonicalAuthorityResolution
}

// recordedAuthority is an opaque, strictly revalidated presentation and
// resolution pair. It carries canonical material, not a claim that the
// resolution is currently effective or still unused; callers must pass its
// ResolveRequest through KernelGate at the reliance boundary.
type recordedAuthority struct {
	state *recordedAuthorityState
}

func newRecordedAuthority(
	presentation canonicalPresentation,
	resolution canonicalAuthorityResolution,
) (recordedAuthority, error) {
	if err := validateCanonicalPresentation(presentation); err != nil {
		return recordedAuthority{}, err
	}
	if err := validateCanonicalAuthorityResolution(resolution, presentation); err != nil {
		return recordedAuthority{}, err
	}
	state := recordedAuthorityState{
		presentation: presentation,
		resolution:   resolution,
	}
	return recordedAuthority{state: &state}, nil
}

func (record recordedAuthority) Valid() bool {
	if record.state == nil {
		return false
	}
	presentation := record.state.presentation
	resolution := record.state.resolution
	return validateCanonicalPresentation(presentation) == nil &&
		validateCanonicalAuthorityResolution(resolution, presentation) == nil
}

func (record recordedAuthority) Presentation() (Presentation, bool) {
	if !record.Valid() {
		return Presentation{}, false
	}
	return Presentation{value: record.state.presentation}, true
}

func (record recordedAuthority) AuthorityResolutionID() (AuthorityResolutionID, bool) {
	if !record.Valid() {
		return AuthorityResolutionID{}, false
	}
	return record.state.resolution.id, true
}

func (record recordedAuthority) AuthorityResolutionDigest() (Digest, bool) {
	if !record.Valid() {
		return Digest{}, false
	}
	return record.state.resolution.digest, true
}

func (record recordedAuthority) VerifierIdentity() (VerifierIdentity, bool) {
	if !record.Valid() {
		return VerifierIdentity{}, false
	}
	return record.state.resolution.verifierIdentity, true
}

func (record recordedAuthority) VerifierVersion() (VerifierVersion, bool) {
	if !record.Valid() {
		return VerifierVersion{}, false
	}
	return record.state.resolution.verifierVersion, true
}

func (record recordedAuthority) ResolutionWindow() (TimeWindow, bool) {
	if !record.Valid() {
		return TimeWindow{}, false
	}
	window := TimeWindow{
		from:  record.state.resolution.resolvedAt,
		until: record.state.resolution.validUntil,
	}
	return window, true
}

func (record recordedAuthority) resolveRequest() (ResolveRequest, bool) {
	if !record.Valid() {
		return ResolveRequest{}, false
	}
	request := ResolveRequest{
		presentationID:        record.state.presentation.id,
		authorityResolutionID: record.state.resolution.id,
		basis:                 record.state.presentation.basis,
		envelope:              record.state.presentation.envelope,
	}
	if err := validateResolveRequest(request); err != nil {
		return ResolveRequest{}, false
	}
	return request, true
}

// loadRecordedAuthorityByResolution resolves the durable pair by the exact
// resolution ID and digest, then reconstructs and validates every canonical
// presentation and resolution field from SQLite before returning it.
func loadRecordedAuthorityByResolution(
	ctx context.Context,
	database *sql.DB,
	authorityResolutionID AuthorityResolutionID,
	authorityResolutionDigest Digest,
) (recordedAuthority, error) {
	if ctx == nil {
		return recordedAuthority{}, fmt.Errorf("recorded authority load requires a context")
	}
	if database == nil {
		return recordedAuthority{}, fmt.Errorf("recorded authority load requires a database")
	}
	if !authorityResolutionID.valid() || !authorityResolutionDigest.valid() {
		return recordedAuthority{}, fmt.Errorf("recorded authority load requires canonical resolution identity")
	}
	if err := verifyReconciledAuthoritySchema(database); err != nil {
		return recordedAuthority{}, err
	}
	const presentationLookup = `
		SELECT presentation_id
		FROM authority_resolution_records
		WHERE authority_resolution_id = ?
		AND authority_resolution_digest = ?`
	var presentationIDText string
	err := database.QueryRowContext(
		ctx,
		presentationLookup,
		authorityResolutionID.String(),
		authorityResolutionDigest.String(),
	).Scan(&presentationIDText)
	if errors.Is(err, sql.ErrNoRows) {
		return recordedAuthority{}, fmt.Errorf("exact authority resolution is unavailable: %w", err)
	}
	if err != nil {
		return recordedAuthority{}, fmt.Errorf("locate exact authority resolution: %w", err)
	}
	presentationID, err := NewPresentationID(presentationIDText)
	if err != nil {
		return recordedAuthority{}, fmt.Errorf("parse authority resolution presentation ID: %w", err)
	}
	request := ResolveRequest{
		presentationID:        presentationID,
		authorityResolutionID: authorityResolutionID,
	}
	row, err := loadAuthorityRecord(ctx, database, request)
	if err != nil {
		return recordedAuthority{}, fmt.Errorf("load exact recorded authority pair: %w", err)
	}
	presentation, resolution, err := parseAuthorityRecord(row)
	if err != nil {
		return recordedAuthority{}, fmt.Errorf("validate exact recorded authority pair: %w", err)
	}
	if resolution.id != authorityResolutionID || resolution.digest != authorityResolutionDigest {
		return recordedAuthority{}, fmt.Errorf("recorded authority pair differs from requested resolution identity")
	}
	return newRecordedAuthority(presentation, resolution)
}

func verifyReconciledAuthoritySchema(database *sql.DB) error {
	if database == nil {
		return fmt.Errorf("authority schema verification requires a database")
	}
	if err := verifyAuthoritySchema(database); err != nil {
		return err
	}
	const migrationQuery = `SELECT COUNT(*) FROM schema_version WHERE version = ?`
	var migrationCount int
	err := database.QueryRow(
		migrationQuery,
		reconciledAuthoritySchemaVersion,
	).Scan(&migrationCount)
	if err != nil {
		return fmt.Errorf("inspect reconciled authority migration version: %w", err)
	}
	if migrationCount != 1 {
		return fmt.Errorf("reconciled authority schema migration 36 is unavailable")
	}
	return nil
}
