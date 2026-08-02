package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	projectprofilesqlite "github.com/m0n0x41d/haft/internal/projectprofile/sqlite"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const (
	v5ProfileOrigin    = "host_routed_operator_request"
	v5BasisSchema      = "haft.profile-authority.host-routed-basis/v1"
	v5ResolutionSchema = "haft.profile-authority.host-routed-resolution/v1"
	v5UseSchema        = "haft.profile-authority.host-routed-use/v1"
)

var v5DirectProfileAuthorityContract = directProfileAuthorityContract{
	generation:       "v5 host-routed",
	mode:             profiledeclarationpreparation.ModeHostRoutedOperatorRequest,
	resolutionKind:   profiledeclarationpreparation.ResolutionHostRoutedRequest,
	actionKind:       profiledeclarationpreparation.ActionHostRoutedProfileDeclaration,
	origin:           v5ProfileOrigin,
	basisSchema:      v5BasisSchema,
	resolutionSchema: v5ResolutionSchema,
	useSchema:        v5UseSchema,
	hostRouted:       true,
}

const selectV5BasisSQL = `SELECT
	basis_ref, basis_digest, project_root, action_kind, authority_mode,
	profile_origin, operator_request_ref, operator_request_digest,
	operator_request_subject_ref, operator_request_payload_digest,
	work_input_ref, work_input_digest,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest,
	method_description_ref, method_description_digest,
	method_contract_ref, method_contract_digest,
	classifier_version, policy_version, suggestion_ref, observation_digest,
	future_work_session_ref, allowed_work_from, allowed_work_until,
	basis_observation_from, basis_observation_until, single_use_key,
	canonical_json, recorded_at
FROM profile_declaration_authority_bases_v5
WHERE basis_ref = ?`

const selectV5ResolutionByBasisSQL = `SELECT
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	project_root, action_kind, authority_mode, resolution_kind, profile_origin,
	operator_request_ref, operator_request_digest,
	operator_request_subject_ref, operator_request_payload_digest,
	work_input_ref, work_input_digest, project_binding_digest,
	detector_version, detector_policy_version, suggestion_ref, observation_digest,
	verifier_identity, verifier_version,
	verification_policy_ref, verification_policy_digest,
	checked_at, currentness_result, predicate_result, admission_result,
	canonical_json, recorded_at
FROM profile_declaration_authority_resolutions_v5
WHERE authority_basis_ref = ?`

const selectV5UseByAdmissionSQL = `SELECT
	use_ref, use_digest, project_root, action_kind, authority_mode,
	resolution_kind, profile_origin, project_binding_digest,
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	work_input_ref, work_input_digest, single_use_key,
	admission_request_digest, committed_admission_ref,
	committed_admission_digest, canonical_json, consumed_at
FROM profile_declaration_authority_uses_v5
WHERE committed_admission_ref = ?`

func discoverV5Basis(
	ctx context.Context,
	database *sql.DB,
	basisRef string,
) (bool, error) {
	mode := ""
	err := database.QueryRowContext(
		ctx,
		"SELECT authority_mode FROM profile_declaration_authority_bases_v5 WHERE basis_ref = ?",
		basisRef,
	).Scan(&mode)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("discover v5 host-routed profile authority basis: %w", err)
	}
	if mode != v5DirectProfileAuthorityContract.mode {
		return false, fmt.Errorf("v5 profile authority basis has unknown mode %q", mode)
	}
	return true, nil
}

func loadV5AuthorityClosure(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basisRef string,
) (v4AuthorityClosure, error) {
	basis := v4AuthorityBasisRow{}
	err := transaction.ScanOne(ctx, selectV5BasisSQL, []any{basisRef}, basis.scanTargetsV5())
	if err != nil {
		return v4AuthorityClosure{}, fmt.Errorf("load v5 profile authority basis: %w", err)
	}
	resolution := v4AuthorityResolutionRow{}
	err = transaction.ScanOne(
		ctx,
		selectV5ResolutionByBasisSQL,
		[]any{basisRef},
		resolution.scanTargetsV5(),
	)
	if err != nil {
		return v4AuthorityClosure{}, fmt.Errorf("load v5 profile authority resolution: %w", err)
	}
	workInput := v3WorkInputRow{}
	err = transaction.ScanOne(
		ctx,
		selectV3WorkInputSQL,
		[]any{basis.workInputRef},
		workInput.scanTargets(),
	)
	if err != nil {
		return v4AuthorityClosure{}, fmt.Errorf("load v5 profile WorkInput: %w", err)
	}
	closure := v4AuthorityClosure{
		basis:      basis,
		resolution: resolution,
		workInput:  workInput,
	}
	if err := validateDirectProfileAuthorityClosure(
		closure,
		v5DirectProfileAuthorityContract,
	); err != nil {
		return v4AuthorityClosure{}, err
	}
	return closure, nil
}

func newV5AuthorityUseRecord(
	useRef string,
	authorityValue authorityMaterial,
	requestDigest string,
	committedRef string,
	committedDigest string,
	consumedAt time.Time,
) (v4AuthorityUseRecord, error) {
	return newDirectProfileAuthorityUseRecord(
		useRef,
		authorityValue,
		requestDigest,
		committedRef,
		committedDigest,
		consumedAt,
		v5DirectProfileAuthorityContract,
	)
}

func loadV5AuthorityUseByAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admissionRef string,
) (v4AuthorityUseRecord, error) {
	record := v4AuthorityUseRecord{}
	err := transaction.ScanOne(
		ctx,
		selectV5UseByAdmissionSQL,
		[]any{admissionRef},
		record.scanTargets(),
	)
	if err != nil {
		return v4AuthorityUseRecord{}, fmt.Errorf("load v5 host-routed authority use: %w", err)
	}
	if err := validateDirectProfileAuthorityUseRecord(
		record,
		v5DirectProfileAuthorityContract,
	); err != nil {
		return v4AuthorityUseRecord{}, err
	}
	return record, nil
}

func validateV5HistoricalMaterialInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	material canonicalAdmissionMaterial,
) error {
	use, err := loadV5AuthorityUseByAdmission(ctx, transaction, material.admissionRef.String())
	if err != nil {
		return err
	}
	closure, err := loadV5AuthorityClosure(ctx, transaction, material.authorityBasisRef.String())
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{use.projectRoot == material.projectRoot.String(), "use project root"},
		{use.authorityResolutionRef == material.authorityResolutionRef.String(), "use resolution ref"},
		{use.authorityResolutionDigest == material.authorityResolutionDigest.String(), "use resolution digest"},
		{use.authorityBasisRef == material.authorityBasisRef.String(), "use basis ref"},
		{use.authorityBasisDigest == material.authorityBasisDigest.String(), "use basis digest"},
		{use.committedAdmissionRef == material.admissionRef.String(), "use admission ref"},
		{use.committedAdmissionDigest == material.admissionDigest.String(), "use admission digest"},
		{use.authorityBasisRef == closure.basis.ref, "closure basis ref"},
		{use.authorityResolutionRef == closure.resolution.ref, "closure resolution ref"},
		{use.workInputRef == closure.workInput.ref, "closure WorkInput ref"},
		{use.origin == v5ProfileOrigin, "profile origin"},
	}
	return firstMismatch(checks, "historical v5 host-routed profile authority")
}

func materializeV5Authority(
	closure v4AuthorityClosure,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	admissionTime time.Time,
) (authorityMaterial, error) {
	return materializeDirectProfileAuthority(
		closure,
		values,
		admissionTime,
		v5DirectProfileAuthorityContract,
	)
}

func v5ProfileOriginValue() projectprofile.ProfileAdmissionOrigin {
	return projectprofile.ProfileAdmissionOriginHostRoutedOperatorRequest
}
