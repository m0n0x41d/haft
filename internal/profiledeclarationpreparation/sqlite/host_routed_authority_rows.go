package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"time"

	"github.com/m0n0x41d/haft/internal/operatorrequest"
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const (
	hostRoutedBasisSchema      = "haft.profile-authority.host-routed-basis/v1"
	hostRoutedResolutionSchema = "haft.profile-authority.host-routed-resolution/v1"
	hostRoutedProfileOrigin    = "host_routed_operator_request"
)

type hostRoutedAuthorityBasisJSON struct {
	Schema                            string `json:"schema"`
	BasisRef                          string `json:"basis_ref"`
	ProjectRoot                       string `json:"project_root"`
	ActionKind                        string `json:"action_kind"`
	AuthorityMode                     string `json:"authority_mode"`
	ProfileOrigin                     string `json:"profile_origin"`
	OperatorRequestRef                string `json:"operator_request_ref"`
	OperatorRequestEffect             string `json:"operator_request_effect,omitempty"`
	OperatorRequestDigest             string `json:"operator_request_digest"`
	OperatorRequestSubjectRef         string `json:"operator_request_subject_ref"`
	OperatorRequestPayloadDigest      string `json:"operator_request_payload_digest"`
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

type hostRoutedAuthorityResolutionJSON struct {
	Schema                       string `json:"schema"`
	AuthorityResolutionRef       string `json:"authority_resolution_ref"`
	AuthorityBasisRef            string `json:"authority_basis_ref"`
	AuthorityBasisDigest         string `json:"authority_basis_digest"`
	ProjectRoot                  string `json:"project_root"`
	ActionKind                   string `json:"action_kind"`
	AuthorityMode                string `json:"authority_mode"`
	ResolutionKind               string `json:"resolution_kind"`
	ProfileOrigin                string `json:"profile_origin"`
	OperatorRequestRef           string `json:"operator_request_ref"`
	OperatorRequestEffect        string `json:"operator_request_effect,omitempty"`
	OperatorRequestDigest        string `json:"operator_request_digest"`
	OperatorRequestSubjectRef    string `json:"operator_request_subject_ref"`
	OperatorRequestPayloadDigest string `json:"operator_request_payload_digest"`
	WorkInputRef                 string `json:"work_input_ref"`
	WorkInputDigest              string `json:"work_input_digest"`
	ProjectBindingDigest         string `json:"project_binding_digest"`
	DetectorVersion              string `json:"detector_version"`
	DetectorPolicyVersion        string `json:"detector_policy_version"`
	SuggestionRef                string `json:"suggestion_ref"`
	ObservationDigest            string `json:"observation_digest"`
	VerifierIdentity             string `json:"verifier_identity"`
	VerifierVersion              string `json:"verifier_version"`
	VerificationPolicyRef        string `json:"verification_policy_ref"`
	VerificationPolicyDigest     string `json:"verification_policy_digest"`
	CheckedAt                    string `json:"checked_at"`
	CurrentnessResult            string `json:"currentness_result"`
	PredicateResult              string `json:"predicate_result"`
	AdmissionResult              string `json:"admission_result"`
}

type hostRoutedAuthorityBasisRow struct {
	dto       hostRoutedAuthorityBasisJSON
	digest    projectprofile.ContentDigest
	canonical []byte
	recorded  time.Time
}

type hostRoutedAuthorityResolutionRow struct {
	dto       hostRoutedAuthorityResolutionJSON
	digest    projectprofile.ContentDigest
	canonical []byte
	recorded  time.Time
}

func buildHostRoutedAuthorityRows(
	plan profiledeclarationpreparation.Plan,
	projectBindingDigest projectprofile.ContentDigest,
) (hostRoutedAuthorityBasisRow, hostRoutedAuthorityResolutionRow, error) {
	request, ok := plan.Policy().OperatorRequest()
	if !ok {
		return hostRoutedAuthorityBasisRow{},
			hostRoutedAuthorityResolutionRow{},
			fmt.Errorf("host-routed profile authority requires operator-request provenance")
	}
	support := plan.Support()
	assignment := support.RoleAssignment()
	assignmentDigest, err := projectprofile.DigestProfileAuthorRoleAssignmentV1(assignment)
	if err != nil {
		return hostRoutedAuthorityBasisRow{}, hostRoutedAuthorityResolutionRow{}, err
	}
	description := support.MethodDescription()
	descriptionDigest, err := projectprofile.DigestProfileOnboardingMethodDescriptionV2(description)
	if err != nil {
		return hostRoutedAuthorityBasisRow{}, hostRoutedAuthorityResolutionRow{}, err
	}
	contract := support.MethodContract()
	contractDigest, err := projectprofile.DigestProfileOnboardingMethodContractV2(contract)
	if err != nil {
		return hostRoutedAuthorityBasisRow{}, hostRoutedAuthorityResolutionRow{}, err
	}
	basisRef, err := plan.AuthorityBasisRef()
	if err != nil {
		return hostRoutedAuthorityBasisRow{}, hostRoutedAuthorityResolutionRow{}, err
	}
	input := plan.Input()
	actionKind, policyRef, policySchema, err := hostRoutedEffectPolicy(request.Effect())
	if err != nil {
		return hostRoutedAuthorityBasisRow{}, hostRoutedAuthorityResolutionRow{}, err
	}
	allowedWork := plan.AllowedWork()
	allowedBasis := plan.AllowedBasis()
	basisDTO := hostRoutedAuthorityBasisJSON{
		Schema: hostRoutedBasisSchema, BasisRef: basisRef.String(),
		ProjectRoot:   plan.Root().String(),
		ActionKind:    actionKind,
		AuthorityMode: plan.Policy().Mode(), ProfileOrigin: hostRoutedProfileOrigin,
		OperatorRequestRef:           request.Ref(),
		OperatorRequestEffect:        string(request.Effect()),
		OperatorRequestDigest:        request.Digest(),
		OperatorRequestSubjectRef:    request.SubjectRef(),
		OperatorRequestPayloadDigest: request.PayloadDigest(),
		WorkInputRef:                 input.Ref().String(), WorkInputDigest: input.Digest().String(),
		ProfileAuthorRoleAssignmentRef:    assignment.RoleAssignmentRef().String(),
		ProfileAuthorRoleAssignmentDigest: assignmentDigest.String(),
		MethodDescriptionRef:              description.Ref().String(),
		MethodDescriptionDigest:           descriptionDigest.String(),
		MethodContractRef:                 contract.Ref().String(),
		MethodContractDigest:              contractDigest.String(),
		ClassifierVersion:                 support.ClassifierVersion().String(),
		PolicyVersion:                     support.PolicyVersion().String(),
		SuggestionRef:                     input.SuggestionRef(),
		ObservationDigest:                 input.ObservationDigest(),
		FutureWorkSessionRef:              support.SessionRef().String(),
		AllowedWorkFrom:                   formatCanonicalTime(allowedWork.From()),
		AllowedWorkUntil:                  formatCanonicalTime(allowedWork.Until()),
		BasisObservationFrom:              formatCanonicalTime(allowedBasis.From()),
		BasisObservationUntil:             formatCanonicalTime(allowedBasis.Until()),
		SingleUseKey:                      plan.SingleUseKey(),
	}
	basisCanonical, basisDigest, err := digestAuthorityJSON(hostRoutedBasisSchema, basisDTO)
	if err != nil {
		return hostRoutedAuthorityBasisRow{}, hostRoutedAuthorityResolutionRow{}, err
	}
	verificationDigest, err := digestStrings(
		policySchema,
		[]string{policyRef, projectBindingDigest.String(), request.Digest()},
	)
	if err != nil {
		return hostRoutedAuthorityBasisRow{}, hostRoutedAuthorityResolutionRow{}, err
	}
	resolutionDTO := hostRoutedAuthorityResolutionJSON{
		Schema:                 hostRoutedResolutionSchema,
		AuthorityResolutionRef: plan.AuthorityResolutionRef(),
		AuthorityBasisRef:      basisRef.String(), AuthorityBasisDigest: basisDigest.String(),
		ProjectRoot:                  plan.Root().String(),
		ActionKind:                   actionKind,
		AuthorityMode:                plan.Policy().Mode(),
		ResolutionKind:               profiledeclarationpreparation.ResolutionHostRoutedRequest,
		ProfileOrigin:                hostRoutedProfileOrigin,
		OperatorRequestRef:           request.Ref(),
		OperatorRequestEffect:        string(request.Effect()),
		OperatorRequestDigest:        request.Digest(),
		OperatorRequestSubjectRef:    request.SubjectRef(),
		OperatorRequestPayloadDigest: request.PayloadDigest(),
		WorkInputRef:                 input.Ref().String(), WorkInputDigest: input.Digest().String(),
		ProjectBindingDigest:  projectBindingDigest.String(),
		DetectorVersion:       input.DetectorVersion(),
		DetectorPolicyVersion: input.PolicyVersion(),
		SuggestionRef:         input.SuggestionRef(), ObservationDigest: input.ObservationDigest(),
		VerifierIdentity: "haft-core", VerifierVersion: "v9",
		VerificationPolicyRef:    policyRef,
		VerificationPolicyDigest: verificationDigest.String(),
		CheckedAt:                formatCanonicalTime(plan.PreparedAt()),
		CurrentnessResult:        "current", PredicateResult: "satisfied",
		AdmissionResult: "admitted",
	}
	resolutionCanonical, resolutionDigest, err := digestAuthorityJSON(
		hostRoutedResolutionSchema,
		resolutionDTO,
	)
	if err != nil {
		return hostRoutedAuthorityBasisRow{}, hostRoutedAuthorityResolutionRow{}, err
	}
	return hostRoutedAuthorityBasisRow{
			dto: basisDTO, digest: basisDigest, canonical: basisCanonical,
			recorded: plan.PreparedAt(),
		}, hostRoutedAuthorityResolutionRow{
			dto: resolutionDTO, digest: resolutionDigest, canonical: resolutionCanonical,
			recorded: plan.PreparedAt(),
		}, nil
}

func hostRoutedEffectPolicy(
	effect operatorrequest.Effect,
) (actionKind string, policyRef string, policySchema string, err error) {
	// Both branches append a new canonical profile declaration revision at the
	// admission layer. The operator-request effect remains distinct in the
	// canonical authority coordinates and selects a separate verification
	// policy; it must not be inferred from this lower-layer admission action.
	policies := map[operatorrequest.Effect][3]string{
		operatorrequest.ProfileDeclaration: {
			profiledeclarationpreparation.ActionHostRoutedProfileDeclaration,
			"verification-policy:profile-declaration/host-routed-request/v1",
			"haft.profile-declaration.host-routed-verification-policy/v1",
		},
		operatorrequest.ProfileChange: {
			profiledeclarationpreparation.ActionHostRoutedProfileDeclaration,
			"verification-policy:profile-change/host-routed-request/v1",
			"haft.profile-change.host-routed-verification-policy/v1",
		},
	}
	selected, present := policies[effect]
	if !present {
		return "", "", "", fmt.Errorf(
			"host-routed profile authority effect %q is unsupported",
			effect,
		)
	}
	return selected[0], selected[1], selected[2], nil
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
		return projectprofile.ContentDigest{}, fmt.Errorf("load project binding digest: %w", err)
	}
	return projectprofile.NewContentDigest(raw)
}

func persistHostRoutedAuthorityRows(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basis hostRoutedAuthorityBasisRow,
	resolution hostRoutedAuthorityResolutionRow,
) error {
	b := basis.dto
	_, err := transaction.Execute(ctx, `INSERT INTO profile_declaration_authority_bases_v5 (
		basis_ref, basis_digest, project_root, action_kind, authority_mode, profile_origin,
		operator_request_ref, operator_request_digest, operator_request_subject_ref,
		operator_request_payload_digest, work_input_ref, work_input_digest,
		profile_author_role_assignment_ref, profile_author_role_assignment_digest,
		method_description_ref, method_description_digest,
		method_contract_ref, method_contract_digest,
		classifier_version, policy_version, suggestion_ref, observation_digest,
		future_work_session_ref, allowed_work_from, allowed_work_until,
		basis_observation_from, basis_observation_until, single_use_key,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, []any{
		b.BasisRef, basis.digest.String(), b.ProjectRoot, b.ActionKind, b.AuthorityMode,
		b.ProfileOrigin, b.OperatorRequestRef, b.OperatorRequestDigest,
		b.OperatorRequestSubjectRef, b.OperatorRequestPayloadDigest,
		b.WorkInputRef, b.WorkInputDigest,
		b.ProfileAuthorRoleAssignmentRef, b.ProfileAuthorRoleAssignmentDigest,
		b.MethodDescriptionRef, b.MethodDescriptionDigest,
		b.MethodContractRef, b.MethodContractDigest,
		b.ClassifierVersion, b.PolicyVersion, b.SuggestionRef, b.ObservationDigest,
		b.FutureWorkSessionRef, b.AllowedWorkFrom, b.AllowedWorkUntil,
		b.BasisObservationFrom, b.BasisObservationUntil, b.SingleUseKey,
		string(basis.canonical), formatCanonicalTime(basis.recorded),
	})
	if err != nil {
		return fmt.Errorf("persist host-routed profile authority basis: %w", err)
	}
	r := resolution.dto
	_, err = transaction.Execute(ctx, `INSERT INTO profile_declaration_authority_resolutions_v5 (
		authority_resolution_ref, authority_resolution_digest,
		authority_basis_ref, authority_basis_digest,
		project_root, action_kind, authority_mode, resolution_kind, profile_origin,
		operator_request_ref, operator_request_digest, operator_request_subject_ref,
		operator_request_payload_digest, work_input_ref, work_input_digest,
		project_binding_digest, detector_version, detector_policy_version,
		suggestion_ref, observation_digest, verifier_identity, verifier_version,
		verification_policy_ref, verification_policy_digest,
		checked_at, currentness_result, predicate_result, admission_result,
		canonical_json, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, []any{
		r.AuthorityResolutionRef, resolution.digest.String(),
		r.AuthorityBasisRef, r.AuthorityBasisDigest, r.ProjectRoot, r.ActionKind,
		r.AuthorityMode, r.ResolutionKind, r.ProfileOrigin,
		r.OperatorRequestRef, r.OperatorRequestDigest,
		r.OperatorRequestSubjectRef, r.OperatorRequestPayloadDigest,
		r.WorkInputRef, r.WorkInputDigest, r.ProjectBindingDigest,
		r.DetectorVersion, r.DetectorPolicyVersion, r.SuggestionRef,
		r.ObservationDigest, r.VerifierIdentity, r.VerifierVersion,
		r.VerificationPolicyRef, r.VerificationPolicyDigest,
		r.CheckedAt, r.CurrentnessResult, r.PredicateResult, r.AdmissionResult,
		string(resolution.canonical), formatCanonicalTime(resolution.recorded),
	})
	if err != nil {
		return fmt.Errorf("persist host-routed profile authority resolution: %w", err)
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

func digestBytes(domain string, value []byte) (projectprofile.ContentDigest, error) {
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

func digestStrings(domain string, values []string) (projectprofile.ContentDigest, error) {
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
