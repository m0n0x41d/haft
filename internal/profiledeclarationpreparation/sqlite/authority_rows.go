package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"time"

	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const (
	authorityBasisSchemaV3      = "haft.profile-authority.authority-basis/v3"
	authorityResolutionSchemaV3 = "haft.profile-authority.authority-resolution/v3"
	profileDeclarationAction    = "profile.declare.from_onboarding_candidate"
	explicitResolutionKind      = "explicit_policy_acceptance"
)

type authorityBasisJSONV3 struct {
	Schema                            string `json:"schema"`
	BasisRef                          string `json:"basis_ref"`
	ProjectRoot                       string `json:"project_root"`
	ActionKind                        string `json:"action_kind"`
	AuthorityMode                     string `json:"authority_mode"`
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
	FutureWorkSessionRef              string `json:"future_work_session_ref"`
	AllowedWorkFrom                   string `json:"allowed_work_from"`
	AllowedWorkUntil                  string `json:"allowed_work_until"`
	BasisObservationFrom              string `json:"basis_observation_from"`
	BasisObservationUntil             string `json:"basis_observation_until"`
	SingleUseKey                      string `json:"single_use_key"`
	ConfigCarrierRef                  string `json:"config_carrier_ref,omitempty"`
	ConfigCarrierDigest               string `json:"config_carrier_digest,omitempty"`
}

type authorityResolutionJSONV3 struct {
	Schema                   string `json:"schema"`
	AuthorityResolutionRef   string `json:"authority_resolution_ref"`
	AuthorityBasisRef        string `json:"authority_basis_ref"`
	AuthorityBasisDigest     string `json:"authority_basis_digest"`
	ProjectRoot              string `json:"project_root"`
	ActionKind               string `json:"action_kind"`
	AuthorityMode            string `json:"authority_mode"`
	ResolutionKind           string `json:"resolution_kind"`
	WorkInputRef             string `json:"work_input_ref"`
	WorkInputDigest          string `json:"work_input_digest"`
	ProjectBindingDigest     string `json:"project_binding_digest"`
	VerifierIdentity         string `json:"verifier_identity"`
	VerifierVersion          string `json:"verifier_version"`
	VerificationPolicyRef    string `json:"verification_policy_ref"`
	VerificationPolicyDigest string `json:"verification_policy_digest"`
	CheckedAt                string `json:"checked_at"`
	CurrentnessResult        string `json:"currentness_result"`
	PredicateResult          string `json:"predicate_result"`
	AdmissionResult          string `json:"admission_result"`
}

type authorityBasisRowV3 struct {
	dto       authorityBasisJSONV3
	digest    projectprofile.ContentDigest
	canonical []byte
	recorded  time.Time
}

type authorityResolutionRowV3 struct {
	dto       authorityResolutionJSONV3
	digest    projectprofile.ContentDigest
	canonical []byte
	recorded  time.Time
}

func buildAuthorityRowsV3(
	plan profiledeclarationpreparation.Plan,
	projectBindingDigest projectprofile.ContentDigest,
) (authorityBasisRowV3, authorityResolutionRowV3, error) {
	support := plan.Support()
	assignment := support.RoleAssignment()
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return authorityBasisRowV3{}, authorityResolutionRowV3{}, err
	}
	description := support.MethodDescription()
	descriptionDigest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV2(description)
	if err != nil {
		return authorityBasisRowV3{}, authorityResolutionRowV3{}, err
	}
	contract := support.MethodContract()
	contractDigest, err := projectprofile.DigestProfileOnboardingMethodContractV2(contract)
	if err != nil {
		return authorityBasisRowV3{}, authorityResolutionRowV3{}, err
	}
	basisRef, err := plan.AuthorityBasisRef()
	if err != nil {
		return authorityBasisRowV3{}, authorityResolutionRowV3{}, err
	}
	input := plan.Input()
	allowedWork := plan.AllowedWork()
	allowedBasis := plan.AllowedBasis()
	basisDTO := authorityBasisJSONV3{
		Schema:                            authorityBasisSchemaV3,
		BasisRef:                          basisRef.String(),
		ProjectRoot:                       plan.Root().String(),
		ActionKind:                        profileDeclarationAction,
		AuthorityMode:                     plan.Policy().Mode(),
		WorkInputRef:                      input.Ref().String(),
		WorkInputDigest:                   input.Digest().String(),
		ProfileAuthorRoleAssignmentRef:    assignment.RoleAssignmentRef().String(),
		ProfileAuthorRoleAssignmentDigest: assignmentDigest.String(),
		MethodDescriptionRef:              description.Ref().String(),
		MethodDescriptionDigest:           descriptionDigest.String(),
		MethodContractRef:                 contract.Ref().String(),
		MethodContractDigest:              contractDigest.String(),
		ClassifierVersion:                 support.ClassifierVersion().String(),
		PolicyVersion:                     support.PolicyVersion().String(),
		FutureWorkSessionRef:              support.SessionRef().String(),
		AllowedWorkFrom:                   formatCanonicalTime(allowedWork.From()),
		AllowedWorkUntil:                  formatCanonicalTime(allowedWork.Until()),
		BasisObservationFrom:              formatCanonicalTime(allowedBasis.From()),
		BasisObservationUntil:             formatCanonicalTime(allowedBasis.Until()),
		SingleUseKey:                      plan.SingleUseKey(),
	}
	configRef, configDigest, ok := plan.Policy().ConfigCarrier()
	if !ok {
		return authorityBasisRowV3{}, authorityResolutionRowV3{}, fmt.Errorf(
			"explicit profile preparation requires config-carrier provenance",
		)
	}
	basisDTO.ConfigCarrierRef = configRef
	basisDTO.ConfigCarrierDigest = configDigest.String()
	basisCanonical, basisDigest, err := digestAuthorityJSON(
		authorityBasisSchemaV3,
		basisDTO,
	)
	if err != nil {
		return authorityBasisRowV3{}, authorityResolutionRowV3{}, err
	}
	policyRef := "verification-policy:profile-declaration/" + plan.Policy().Mode() + "/v1"
	verificationDigest, err := digestStrings(
		"haft.profile-declaration.verification-policy/v1",
		[]string{policyRef, projectBindingDigest.String()},
	)
	if err != nil {
		return authorityBasisRowV3{}, authorityResolutionRowV3{}, err
	}
	resolutionDTO := authorityResolutionJSONV3{
		Schema:                   authorityResolutionSchemaV3,
		AuthorityResolutionRef:   plan.AuthorityResolutionRef(),
		AuthorityBasisRef:        basisRef.String(),
		AuthorityBasisDigest:     basisDigest.String(),
		ProjectRoot:              plan.Root().String(),
		ActionKind:               profileDeclarationAction,
		AuthorityMode:            plan.Policy().Mode(),
		ResolutionKind:           explicitResolutionKind,
		WorkInputRef:             input.Ref().String(),
		WorkInputDigest:          input.Digest().String(),
		ProjectBindingDigest:     projectBindingDigest.String(),
		VerifierIdentity:         "kernel-verifier:profile-declaration-policy",
		VerifierVersion:          "v1",
		VerificationPolicyRef:    policyRef,
		VerificationPolicyDigest: verificationDigest.String(),
		CheckedAt:                formatCanonicalTime(plan.PreparedAt()),
		CurrentnessResult:        "current",
		PredicateResult:          "satisfied",
		AdmissionResult:          "admitted",
	}
	resolutionCanonical, resolutionDigest, err := digestAuthorityJSON(
		authorityResolutionSchemaV3,
		resolutionDTO,
	)
	if err != nil {
		return authorityBasisRowV3{}, authorityResolutionRowV3{}, err
	}
	basisRow := authorityBasisRowV3{
		dto:       basisDTO,
		digest:    basisDigest,
		canonical: basisCanonical,
		recorded:  plan.PreparedAt(),
	}
	resolutionRow := authorityResolutionRowV3{
		dto:       resolutionDTO,
		digest:    resolutionDigest,
		canonical: resolutionCanonical,
		recorded:  plan.PreparedAt(),
	}
	return basisRow, resolutionRow, nil
}

func loadProjectBindingDigest(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	root projectprofile.ProjectRootV1,
) (projectprofile.ContentDigest, error) {
	raw := ""
	err := transaction.ScanOne(
		ctx,
		`SELECT binding_digest FROM project_ledger_binding WHERE project_root = ?`,
		[]any{root.String()},
		[]any{&raw},
	)
	if err != nil {
		return projectprofile.ContentDigest{}, fmt.Errorf(
			"load project binding digest: %w",
			err,
		)
	}
	return projectprofile.NewContentDigest(raw)
}

func persistAuthorityRowsV3(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis authorityBasisRowV3,
	resolution authorityResolutionRowV3,
) error {
	b := basis.dto
	basisArguments := []any{
		b.BasisRef, basis.digest.String(), b.ProjectRoot, b.ActionKind, b.AuthorityMode,
		b.WorkInputRef, b.WorkInputDigest,
		b.ProfileAuthorRoleAssignmentRef, b.ProfileAuthorRoleAssignmentDigest,
		b.MethodDescriptionRef, b.MethodDescriptionDigest,
		b.MethodContractRef, b.MethodContractDigest,
		b.ClassifierVersion, b.PolicyVersion, b.FutureWorkSessionRef,
		b.AllowedWorkFrom, b.AllowedWorkUntil,
		b.BasisObservationFrom, b.BasisObservationUntil, b.SingleUseKey,
		b.ConfigCarrierRef, b.ConfigCarrierDigest,
		nil, nil,
		string(basis.canonical), formatCanonicalTime(basis.recorded),
	}
	_, err := transaction.Execute(ctx, `INSERT INTO profile_declaration_authority_bases_v3 (
		basis_ref, basis_digest, project_root, action_kind, authority_mode,
		work_input_ref, work_input_digest,
		profile_author_role_assignment_ref, profile_author_role_assignment_digest,
		method_description_ref, method_description_digest,
		method_contract_ref, method_contract_digest,
		classifier_version, policy_version, future_work_session_ref,
		allowed_work_from, allowed_work_until,
		basis_observation_from, basis_observation_until, single_use_key,
		config_carrier_ref, config_carrier_digest,
		strict_authority_basis_ref, strict_authority_basis_digest,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, basisArguments)
	if err != nil {
		return fmt.Errorf("persist v3 profile authority basis: %w", err)
	}
	r := resolution.dto
	resolutionArguments := []any{
		r.AuthorityResolutionRef, resolution.digest.String(),
		r.AuthorityBasisRef, r.AuthorityBasisDigest,
		r.ProjectRoot, r.ActionKind, r.AuthorityMode, r.ResolutionKind,
		r.WorkInputRef, r.WorkInputDigest, r.ProjectBindingDigest,
		nil, nil,
		r.VerifierIdentity, r.VerifierVersion,
		r.VerificationPolicyRef, r.VerificationPolicyDigest,
		r.CheckedAt, r.CurrentnessResult, r.PredicateResult, r.AdmissionResult,
		string(resolution.canonical), formatCanonicalTime(resolution.recorded),
	}
	_, err = transaction.Execute(ctx, `INSERT INTO profile_declaration_authority_resolutions_v3 (
		authority_resolution_ref, authority_resolution_digest,
		authority_basis_ref, authority_basis_digest,
		project_root, action_kind, authority_mode, resolution_kind,
		work_input_ref, work_input_digest, project_binding_digest,
		strict_permission_ref, strict_permission_digest,
		verifier_identity, verifier_version,
		verification_policy_ref, verification_policy_digest,
		checked_at, currentness_result, predicate_result, admission_result,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, resolutionArguments)
	if err != nil {
		return fmt.Errorf("persist v3 profile authority resolution: %w", err)
	}
	return nil
}

func digestAuthorityJSON(
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

func digestBytes(
	domain string,
	value []byte,
) (projectprofile.ContentDigest, error) {
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(domain))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write(value)
	raw := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	return projectprofile.NewContentDigest(raw)
}

type stringDigestWriter struct{ hash hash.Hash }

func newStringDigestWriter(domain string) stringDigestWriter {
	writer := stringDigestWriter{hash: sha256.New()}
	writer.add(domain)
	return writer
}

func (writer stringDigestWriter) add(value string) {
	_, _ = writer.hash.Write([]byte(fmt.Sprintf("%d:%s", len(value), value)))
}

func digestStrings(
	domain string,
	values []string,
) (projectprofile.ContentDigest, error) {
	writer := newStringDigestWriter(domain)
	for _, value := range values {
		writer.add(value)
	}
	raw := "sha256:" + hex.EncodeToString(writer.hash.Sum(nil))
	return projectprofile.NewContentDigest(raw)
}

func formatCanonicalTime(value time.Time) string {
	return value.UTC().Round(0).Format(time.RFC3339Nano)
}
