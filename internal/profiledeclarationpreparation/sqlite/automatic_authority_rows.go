package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const (
	automaticBasisSchema      = "haft.profile-authority.automatic-basis/v1"
	automaticResolutionSchema = "haft.profile-authority.automatic-resolution/v1"
	automaticAction           = "profile.apply_supported_singleton_default"
	automaticResolutionKind   = "deterministic_policy_satisfaction"
	automaticOrigin           = "detector_default"
)

type automaticAuthorityBasisJSON struct {
	Schema                            string `json:"schema"`
	BasisRef                          string `json:"basis_ref"`
	ProjectRoot                       string `json:"project_root"`
	ActionKind                        string `json:"action_kind"`
	AuthorityMode                     string `json:"authority_mode"`
	ProfileOrigin                     string `json:"profile_origin"`
	WorkInputRef                      string `json:"work_input_ref"`
	WorkInputDigest                   string `json:"work_input_digest"`
	ProfileAuthorRoleAssignmentRef    string `json:"profile_author_role_assignment_ref"`
	ProfileAuthorRoleAssignmentDigest string `json:"profile_author_role_assignment_digest"`
	MethodDescriptionRef              string `json:"method_description_ref"`
	MethodDescriptionDigest           string `json:"method_description_digest"`
	MethodContractRef                 string `json:"method_contract_ref"`
	MethodContractDigest              string `json:"method_contract_digest"`
	ClassifierVersion                 string `json:"classifier_version"`
	PolicyVersion                     string `json:"policy_version"`
	SuggestionRef                     string `json:"suggestion_ref"`
	ObservationDigest                 string `json:"observation_digest"`
	FutureWorkSessionRef              string `json:"future_work_session_ref"`
	AllowedWorkFrom                   string `json:"allowed_work_from"`
	AllowedWorkUntil                  string `json:"allowed_work_until"`
	BasisObservationFrom              string `json:"basis_observation_from"`
	BasisObservationUntil             string `json:"basis_observation_until"`
	SingleUseKey                      string `json:"single_use_key"`
}

type automaticAuthorityResolutionJSON struct {
	Schema                   string `json:"schema"`
	AuthorityResolutionRef   string `json:"authority_resolution_ref"`
	AuthorityBasisRef        string `json:"authority_basis_ref"`
	AuthorityBasisDigest     string `json:"authority_basis_digest"`
	ProjectRoot              string `json:"project_root"`
	ActionKind               string `json:"action_kind"`
	AuthorityMode            string `json:"authority_mode"`
	ResolutionKind           string `json:"resolution_kind"`
	ProfileOrigin            string `json:"profile_origin"`
	WorkInputRef             string `json:"work_input_ref"`
	WorkInputDigest          string `json:"work_input_digest"`
	ProjectBindingDigest     string `json:"project_binding_digest"`
	DetectorVersion          string `json:"detector_version"`
	DetectorPolicyVersion    string `json:"detector_policy_version"`
	SuggestionRef            string `json:"suggestion_ref"`
	ObservationDigest        string `json:"observation_digest"`
	VerifierIdentity         string `json:"verifier_identity"`
	VerifierVersion          string `json:"verifier_version"`
	VerificationPolicyRef    string `json:"verification_policy_ref"`
	VerificationPolicyDigest string `json:"verification_policy_digest"`
	CheckedAt                string `json:"checked_at"`
	CurrentnessResult        string `json:"currentness_result"`
	PredicateResult          string `json:"predicate_result"`
	AdmissionResult          string `json:"admission_result"`
}

type automaticAuthorityBasisRow struct {
	dto       automaticAuthorityBasisJSON
	digest    projectprofile.ContentDigest
	canonical []byte
	recorded  time.Time
}

type automaticAuthorityResolutionRow struct {
	dto       automaticAuthorityResolutionJSON
	digest    projectprofile.ContentDigest
	canonical []byte
	recorded  time.Time
}

func persistSelectedAuthorityRows(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	plan profiledeclarationpreparation.Plan,
	bindingDigest projectprofile.ContentDigest,
) error {
	if plan.Policy().Mode() ==
		profiledeclarationpreparation.ModeAutomaticSupportedSingleton {
		basis, resolution, err := buildAutomaticAuthorityRows(
			plan,
			bindingDigest,
		)
		if err != nil {
			return err
		}
		return persistAutomaticAuthorityRows(
			ctx,
			transaction,
			basis,
			resolution,
		)
	}
	basis, resolution, err := buildHostRoutedAuthorityRows(plan, bindingDigest)
	if err != nil {
		return err
	}
	return persistHostRoutedAuthorityRows(ctx, transaction, basis, resolution)
}

func buildAutomaticAuthorityRows(
	plan profiledeclarationpreparation.Plan,
	projectBindingDigest projectprofile.ContentDigest,
) (
	automaticAuthorityBasisRow,
	automaticAuthorityResolutionRow,
	error,
) {
	detectorVersion,
		policyVersion,
		suggestionRef,
		observationDigest,
		ok := plan.Policy().AutomaticProvenance()
	if !ok {
		return automaticAuthorityBasisRow{},
			automaticAuthorityResolutionRow{},
			fmt.Errorf("automatic profile authority provenance is unavailable")
	}
	support := plan.Support()
	assignment := support.RoleAssignment()
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(
		assignment,
	)
	if err != nil {
		return automaticAuthorityBasisRow{}, automaticAuthorityResolutionRow{}, err
	}
	description := support.MethodDescription()
	descriptionDigest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV2(
		description,
	)
	if err != nil {
		return automaticAuthorityBasisRow{}, automaticAuthorityResolutionRow{}, err
	}
	contract := support.MethodContract()
	contractDigest, err := projectprofile.DigestProfileOnboardingMethodContractV2(
		contract,
	)
	if err != nil {
		return automaticAuthorityBasisRow{}, automaticAuthorityResolutionRow{}, err
	}
	basisRef, err := plan.AuthorityBasisRef()
	if err != nil {
		return automaticAuthorityBasisRow{}, automaticAuthorityResolutionRow{}, err
	}
	input := plan.Input()
	allowedWork := plan.AllowedWork()
	allowedBasis := plan.AllowedBasis()
	basisDTO := automaticAuthorityBasisJSON{
		Schema:                            automaticBasisSchema,
		BasisRef:                          basisRef.String(),
		ProjectRoot:                       plan.Root().String(),
		ActionKind:                        automaticAction,
		AuthorityMode:                     plan.Policy().Mode(),
		ProfileOrigin:                     automaticOrigin,
		WorkInputRef:                      input.Ref().String(),
		WorkInputDigest:                   input.Digest().String(),
		ProfileAuthorRoleAssignmentRef:    assignment.RoleAssignmentRef().String(),
		ProfileAuthorRoleAssignmentDigest: assignmentDigest.String(),
		MethodDescriptionRef:              description.Ref().String(),
		MethodDescriptionDigest:           descriptionDigest.String(),
		MethodContractRef:                 contract.Ref().String(),
		MethodContractDigest:              contractDigest.String(),
		ClassifierVersion:                 detectorVersion,
		PolicyVersion:                     policyVersion,
		SuggestionRef:                     suggestionRef,
		ObservationDigest:                 observationDigest,
		FutureWorkSessionRef:              support.SessionRef().String(),
		AllowedWorkFrom:                   formatCanonicalTime(allowedWork.From()),
		AllowedWorkUntil:                  formatCanonicalTime(allowedWork.Until()),
		BasisObservationFrom:              formatCanonicalTime(allowedBasis.From()),
		BasisObservationUntil:             formatCanonicalTime(allowedBasis.Until()),
		SingleUseKey:                      plan.SingleUseKey(),
	}
	basisCanonical, basisDigest, err := digestAutomaticAuthorityJSON(
		automaticBasisSchema,
		basisDTO,
	)
	if err != nil {
		return automaticAuthorityBasisRow{}, automaticAuthorityResolutionRow{}, err
	}
	policyRef := "verification-policy:profile-initial-bootstrap/supported-singleton/v1"
	verificationDigest, err := digestStrings(
		"haft.profile-initial-bootstrap.verification-policy/v1",
		[]string{
			policyRef,
			projectBindingDigest.String(),
			detectorVersion,
			policyVersion,
			suggestionRef,
			observationDigest,
		},
	)
	if err != nil {
		return automaticAuthorityBasisRow{}, automaticAuthorityResolutionRow{}, err
	}
	resolutionDTO := automaticAuthorityResolutionJSON{
		Schema:                   automaticResolutionSchema,
		AuthorityResolutionRef:   plan.AuthorityResolutionRef(),
		AuthorityBasisRef:        basisRef.String(),
		AuthorityBasisDigest:     basisDigest.String(),
		ProjectRoot:              plan.Root().String(),
		ActionKind:               automaticAction,
		AuthorityMode:            plan.Policy().Mode(),
		ResolutionKind:           automaticResolutionKind,
		ProfileOrigin:            automaticOrigin,
		WorkInputRef:             input.Ref().String(),
		WorkInputDigest:          input.Digest().String(),
		ProjectBindingDigest:     projectBindingDigest.String(),
		DetectorVersion:          detectorVersion,
		DetectorPolicyVersion:    policyVersion,
		SuggestionRef:            suggestionRef,
		ObservationDigest:        observationDigest,
		VerifierIdentity:         "haft-core",
		VerifierVersion:          "v9",
		VerificationPolicyRef:    policyRef,
		VerificationPolicyDigest: verificationDigest.String(),
		CheckedAt:                formatCanonicalTime(plan.PreparedAt()),
		CurrentnessResult:        "current",
		PredicateResult:          "satisfied",
		AdmissionResult:          "admitted",
	}
	resolutionCanonical, resolutionDigest, err := digestAutomaticAuthorityJSON(
		automaticResolutionSchema,
		resolutionDTO,
	)
	if err != nil {
		return automaticAuthorityBasisRow{}, automaticAuthorityResolutionRow{}, err
	}
	return automaticAuthorityBasisRow{
			dto:       basisDTO,
			digest:    basisDigest,
			canonical: basisCanonical,
			recorded:  plan.PreparedAt(),
		}, automaticAuthorityResolutionRow{
			dto:       resolutionDTO,
			digest:    resolutionDigest,
			canonical: resolutionCanonical,
			recorded:  plan.PreparedAt(),
		}, nil
}

func persistAutomaticAuthorityRows(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis automaticAuthorityBasisRow,
	resolution automaticAuthorityResolutionRow,
) error {
	b := basis.dto
	_, err := transaction.Execute(ctx, `INSERT INTO profile_initial_bootstrap_authority_bases_v1 (
		basis_ref, basis_digest, project_root, action_kind, authority_mode, profile_origin,
		work_input_ref, work_input_digest,
		profile_author_role_assignment_ref, profile_author_role_assignment_digest,
		method_description_ref, method_description_digest,
		method_contract_ref, method_contract_digest,
		classifier_version, policy_version, suggestion_ref, observation_digest,
		future_work_session_ref, allowed_work_from, allowed_work_until,
		basis_observation_from, basis_observation_until, single_use_key,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, []any{
		b.BasisRef, basis.digest.String(), b.ProjectRoot, b.ActionKind,
		b.AuthorityMode, b.ProfileOrigin, b.WorkInputRef, b.WorkInputDigest,
		b.ProfileAuthorRoleAssignmentRef, b.ProfileAuthorRoleAssignmentDigest,
		b.MethodDescriptionRef, b.MethodDescriptionDigest,
		b.MethodContractRef, b.MethodContractDigest,
		b.ClassifierVersion, b.PolicyVersion, b.SuggestionRef,
		b.ObservationDigest, b.FutureWorkSessionRef, b.AllowedWorkFrom,
		b.AllowedWorkUntil, b.BasisObservationFrom, b.BasisObservationUntil,
		b.SingleUseKey, string(basis.canonical),
		formatCanonicalTime(basis.recorded),
	})
	if err != nil {
		return fmt.Errorf("persist automatic profile authority basis: %w", err)
	}
	r := resolution.dto
	_, err = transaction.Execute(ctx, `INSERT INTO profile_initial_bootstrap_authority_resolutions_v1 (
		authority_resolution_ref, authority_resolution_digest,
		authority_basis_ref, authority_basis_digest,
		project_root, action_kind, authority_mode, resolution_kind, profile_origin,
		work_input_ref, work_input_digest, project_binding_digest,
		detector_version, detector_policy_version, suggestion_ref, observation_digest,
		verifier_identity, verifier_version, verification_policy_ref, verification_policy_digest,
		checked_at, currentness_result, predicate_result, admission_result,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, []any{
		r.AuthorityResolutionRef, resolution.digest.String(),
		r.AuthorityBasisRef, r.AuthorityBasisDigest, r.ProjectRoot,
		r.ActionKind, r.AuthorityMode, r.ResolutionKind, r.ProfileOrigin,
		r.WorkInputRef, r.WorkInputDigest, r.ProjectBindingDigest,
		r.DetectorVersion, r.DetectorPolicyVersion, r.SuggestionRef,
		r.ObservationDigest, r.VerifierIdentity, r.VerifierVersion,
		r.VerificationPolicyRef, r.VerificationPolicyDigest, r.CheckedAt,
		r.CurrentnessResult, r.PredicateResult, r.AdmissionResult,
		string(resolution.canonical), formatCanonicalTime(resolution.recorded),
	})
	if err != nil {
		return fmt.Errorf("persist automatic profile authority resolution: %w", err)
	}
	return nil
}

func digestAutomaticAuthorityJSON(
	domain string,
	value any,
) ([]byte, projectprofile.ContentDigest, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, projectprofile.ContentDigest{}, err
	}
	digest, err := digestBytes(domain, canonical)
	return canonical, digest, err
}
