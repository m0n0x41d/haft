package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/project"
)

const (
	commissionProfileUnderdeterminedCode = "commission_profile_underdetermined"
	commissionScopeSelectionRequiredCode = "commission_scope_selection_required"
	commissionScopeNotFoundCode          = "commission_scope_not_found"

	commissionProfileSnapshotMissingCode      = "commission_profile_snapshot_missing"
	commissionProfileSnapshotInvalidCode      = "commission_profile_snapshot_invalid"
	commissionProfileApplicabilityChangedCode = "commission_profile_applicability_changed"
)

// workCommissionSpecAuthority is a request-scoped, trusted projection from the
// canonical profile-admission ledger. It is constructed only by the outer
// project shell; caller JSON cannot mint it or substitute its provenance.
type workCommissionSpecAuthority struct {
	applicability project.ProjectSpecificationSetApplicability
	specSet       project.ProjectSpecificationSet
	basis         canonicalProfileApplicabilityBasis
}

func (authority workCommissionSpecAuthority) valid() bool {
	return authority.applicability.Valid() &&
		authority.basis.valid() &&
		authority.applicability.ProfilePayloadDigest() ==
			authority.basis.payloadDigest
}

func resolveWorkCommissionSpecAuthority(
	ctx context.Context,
	args map[string]any,
) (workCommissionSpecAuthority, error) {
	projectRoot := strings.TrimSpace(stringArg(args, "project_root"))
	if projectRoot == "" {
		return workCommissionSpecAuthority{}, fmt.Errorf(
			"project_root is required at the trusted WorkCommission shell",
		)
	}
	request, err := projectSpecificationScopeRequestFromFlag(
		stringArg(args, "scope_id"),
	)
	if err != nil {
		return workCommissionSpecAuthority{}, err
	}
	specSet, resolution, err :=
		loadProjectSpecificationSetSQLFirstFromCanonicalProfile(
			ctx,
			projectRoot,
			request,
		)
	if err != nil {
		return workCommissionSpecAuthority{}, err
	}
	applicability, basis, resolved := resolution.Resolved()
	if !resolved {
		return workCommissionSpecAuthority{},
			workCommissionSpecAuthorityResolutionError(resolution)
	}
	authority := workCommissionSpecAuthority{
		applicability: applicability,
		specSet:       specSet,
		basis:         basis,
	}
	if !authority.valid() {
		return workCommissionSpecAuthority{}, fmt.Errorf(
			"resolved WorkCommission specification authority is invalid",
		)
	}
	return authority, nil
}

func workCommissionSpecAuthorityResolutionError(
	resolution projectSpecificationApplicabilityResolution,
) error {
	switch resolution.Kind() {
	case projectSpecificationProfileUnderdetermined:
		missingBasis, _ := resolution.MissingBasis()
		return fmt.Errorf(
			"%s: missing_basis=%q; admit one canonical project profile before creating a WorkCommission",
			commissionProfileUnderdeterminedCode,
			missingBasis,
		)
	case projectSpecificationScopeChoiceRequired:
		return fmt.Errorf(
			"%s: available_scope_ids=%q; provide one exact scope_id",
			commissionScopeSelectionRequiredCode,
			strings.Join(scopeIDStrings(resolution.AvailableScopeIDs()), ","),
		)
	case projectSpecificationRequestedScopeNotFound:
		return fmt.Errorf(
			"%s: requested_scope_id=%q available_scope_ids=%q; choose one current canonical scope_id",
			commissionScopeNotFoundCode,
			resolution.request.scopeID.String(),
			strings.Join(scopeIDStrings(resolution.AvailableScopeIDs()), ","),
		)
	default:
		return fmt.Errorf(
			"WorkCommission specification authority resolution is invalid",
		)
	}
}

func normalizeNewWorkCommissionForAuthority(
	commission map[string]any,
	now time.Time,
	authority workCommissionSpecAuthority,
) error {
	if !authority.valid() {
		return fmt.Errorf(
			"WorkCommission specification authority is invalid",
		)
	}
	check := func(
		candidate map[string]any,
		at time.Time,
	) error {
		err := ensureWorkCommissionSpecAuthorityForApplicability(
			candidate,
			at,
			authority.applicability,
			authority.specSet,
		)
		if err != nil {
			return err
		}
		return setWorkCommissionCanonicalProfileBasis(
			candidate,
			authority,
		)
	}
	return normalizeNewWorkCommissionWithSpecAuthority(
		commission,
		now,
		check,
	)
}

func setWorkCommissionCanonicalProfileBasis(
	commission map[string]any,
	authority workCommissionSpecAuthority,
) error {
	snapshot, found := mapArg(commission, "spec_snapshot")
	if !found {
		return fmt.Errorf(
			"scope-local WorkCommission specification snapshot is missing",
		)
	}
	snapshot["profile_admission_record_ref"] =
		authority.basis.admissionRecordRef.String()
	snapshot["profile_admission_record_digest"] =
		authority.basis.admissionRecordDigest.String()
	snapshot["profile_ledger_revision"] =
		authority.basis.ledgerRevision.Value()
	snapshot["project_root"] =
		authority.basis.projectRoot.String()
	return nil
}

func workCommissionProfileFreshnessIssues(
	ctx context.Context,
	commission map[string]any,
	args map[string]any,
) ([]commissionFreshnessIssue, error) {
	if !boolField(args, "require_current_profile_authority") {
		return nil, nil
	}
	projectRoot := strings.TrimSpace(stringArg(args, "project_root"))
	if projectRoot == "" {
		return nil, fmt.Errorf(
			"project_root is required for current WorkCommission profile authority",
		)
	}
	snapshot, found := mapArg(commission, "spec_snapshot")
	if !found {
		return singleCommissionProfileFreshnessIssue(
			commissionProfileSnapshotMissingCode,
			"spec_snapshot",
			stringField(commission, "id"),
			"canonical_profile_basis",
			"missing",
		), nil
	}
	scopeID := strings.TrimSpace(stringField(snapshot, "scope_id"))
	if scopeID == "" {
		return singleCommissionProfileFreshnessIssue(
			commissionProfileSnapshotInvalidCode,
			"spec_snapshot.scope_id",
			stringField(commission, "id"),
			"exact_scope_id",
			"missing",
		), nil
	}
	request, err := projectSpecificationScopeRequestFromFlag(
		scopeID,
	)
	if err != nil {
		return singleCommissionProfileFreshnessIssue(
			commissionProfileSnapshotInvalidCode,
			"spec_snapshot.scope_id",
			stringField(commission, "id"),
			"exact_scope_id",
			stringField(snapshot, "scope_id"),
		), nil
	}
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		ctx,
		projectRoot,
		request,
	)
	if err != nil {
		return nil, err
	}
	applicability, basis, resolved := resolution.Resolved()
	if !resolved {
		return singleCommissionProfileFreshnessIssue(
			commissionProfileApplicabilityChangedCode,
			"spec_snapshot.profile_applicability",
			stringField(snapshot, "scope_id"),
			string(projectSpecificationApplicabilityResolved),
			string(resolution.Kind()),
		), nil
	}
	return workCommissionProfileBasisFreshnessIssues(
		snapshot,
		applicability,
		basis,
	), nil
}

func workCommissionProfileBasisFreshnessIssues(
	snapshot map[string]any,
	applicability project.ProjectSpecificationSetApplicability,
	basis canonicalProfileApplicabilityBasis,
) []commissionFreshnessIssue {
	issues := make([]commissionFreshnessIssue, 0)
	issues = append(
		issues,
		hashFreshnessIssues(
			"commission_profile_scope_changed",
			"spec_snapshot.scope_id",
			stringField(snapshot, "scope_id"),
			stringField(snapshot, "scope_id"),
			applicability.ScopeID().String(),
		)...,
	)
	issues = append(
		issues,
		hashFreshnessIssues(
			"commission_profile_payload_changed",
			"spec_snapshot.profile_payload_digest",
			applicability.ScopeID().String(),
			stringField(snapshot, "profile_payload_digest"),
			basis.payloadDigest.String(),
		)...,
	)
	issues = append(
		issues,
		hashFreshnessIssues(
			"commission_profile_admission_changed",
			"spec_snapshot.profile_admission_record_ref",
			applicability.ScopeID().String(),
			stringField(snapshot, "profile_admission_record_ref"),
			basis.admissionRecordRef.String(),
		)...,
	)
	issues = append(
		issues,
		hashFreshnessIssues(
			"commission_profile_admission_digest_changed",
			"spec_snapshot.profile_admission_record_digest",
			basis.admissionRecordRef.String(),
			stringField(snapshot, "profile_admission_record_digest"),
			basis.admissionRecordDigest.String(),
		)...,
	)
	issues = append(
		issues,
		hashFreshnessIssues(
			"commission_profile_ledger_revision_changed",
			"spec_snapshot.profile_ledger_revision",
			basis.admissionRecordRef.String(),
			numericStringField(snapshot, "profile_ledger_revision"),
			strconv.FormatUint(basis.ledgerRevision.Value(), 10),
		)...,
	)
	issues = append(
		issues,
		hashFreshnessIssues(
			"commission_profile_project_root_changed",
			"spec_snapshot.project_root",
			applicability.ScopeID().String(),
			stringField(snapshot, "project_root"),
			basis.projectRoot.String(),
		)...,
	)
	return issues
}

func singleCommissionProfileFreshnessIssue(
	code string,
	field string,
	ref string,
	expected string,
	actual string,
) []commissionFreshnessIssue {
	return []commissionFreshnessIssue{{
		Code:     code,
		Field:    field,
		Ref:      ref,
		Expected: expected,
		Actual:   actual,
	}}
}

func numericStringField(payload map[string]any, key string) string {
	value, found := payload[key]
	if !found {
		return ""
	}
	switch number := value.(type) {
	case uint64:
		return strconv.FormatUint(number, 10)
	case int:
		return strconv.Itoa(number)
	case int64:
		return strconv.FormatInt(number, 10)
	case float64:
		return strconv.FormatFloat(number, 'f', -1, 64)
	default:
		return fmt.Sprint(number)
	}
}
