package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const resolutionTable = "profile_declaration_authority_resolutions_v2"

const selectResolutionSQL = `SELECT
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	speech_act_ref, speech_act_digest,
	authorization_content_ref, authorization_content_digest,
	permission_ref, permission_digest,
	context_policy_ref, context_policy_digest,
	action_kind, project_root, action_envelope_digest, project_binding_digest,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest,
	claim_scope_ref, bounded_context_ref,
	method_description_ref, method_description_digest,
	method_contract_ref, method_contract_digest,
	classifier_version, policy_version, future_work_session_ref,
	allowed_work_from, allowed_work_until,
	basis_observation_from, basis_observation_until,
	authorization_valid_from, authorization_valid_until,
	permission_valid_from, permission_valid_until,
	single_use_key, enactability_predicate_ref,
	verifier_identity, verifier_version,
	verification_policy_ref, verification_policy_digest,
	checked_at, role_state_relation, enactable_state,
	currentness_result, predicate_result, admission_result,
	canonical_json, recorded_at
FROM profile_declaration_authority_resolutions_v2`

const insertResolutionSQL = `INSERT INTO profile_declaration_authority_resolutions_v2 (
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	speech_act_ref, speech_act_digest,
	authorization_content_ref, authorization_content_digest,
	permission_ref, permission_digest,
	context_policy_ref, context_policy_digest,
	action_kind, project_root, action_envelope_digest, project_binding_digest,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest,
	claim_scope_ref, bounded_context_ref,
	method_description_ref, method_description_digest,
	method_contract_ref, method_contract_digest,
	classifier_version, policy_version, future_work_session_ref,
	allowed_work_from, allowed_work_until,
	basis_observation_from, basis_observation_until,
	authorization_valid_from, authorization_valid_until,
	permission_valid_from, permission_valid_until,
	single_use_key, enactability_predicate_ref,
	verifier_identity, verifier_version,
	verification_policy_ref, verification_policy_digest,
	checked_at, role_state_relation, enactable_state,
	currentness_result, predicate_result, admission_result,
	canonical_json, recorded_at
) VALUES (` + resolutionPlaceholders + `)`

const resolutionPlaceholders = `?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
	?, ?, ?, ?, ?, ?, ?, ?, ?`

type resolutionRow struct {
	ref                      string
	digest                   string
	basisRef                 string
	basisDigest              string
	speechActRef             string
	speechActDigest          string
	contentRef               string
	contentDigest            string
	permissionRef            string
	permissionDigest         string
	contextPolicyRef         string
	contextPolicyDigest      string
	actionKind               string
	projectRoot              string
	actionEnvelopeDigest     string
	projectBindingDigest     string
	profileAuthorRef         string
	profileAuthorDigest      string
	claimScopeRef            string
	boundedContextRef        string
	methodDescriptionRef     string
	methodDescriptionDigest  string
	methodContractRef        string
	methodContractDigest     string
	classifierVersion        string
	policyVersion            string
	futureWorkSessionRef     string
	allowedWorkFrom          string
	allowedWorkUntil         string
	basisObservationFrom     string
	basisObservationUntil    string
	authorizationValidFrom   string
	authorizationValidUntil  string
	permissionValidFrom      string
	permissionValidUntil     string
	singleUseKey             string
	enactabilityPredicateRef string
	verifierIdentity         string
	verifierVersion          string
	verificationPolicyRef    string
	verificationPolicyDigest string
	checkedAt                string
	roleStateRelation        string
	enactableState           string
	currentnessResult        string
	predicateResult          string
	admissionResult          string
	canonical                string
	recordedAt               string
}

func (row *resolutionRow) scanTargets() []any {
	return []any{
		&row.ref, &row.digest, &row.basisRef, &row.basisDigest,
		&row.speechActRef, &row.speechActDigest,
		&row.contentRef, &row.contentDigest,
		&row.permissionRef, &row.permissionDigest,
		&row.contextPolicyRef, &row.contextPolicyDigest,
		&row.actionKind, &row.projectRoot,
		&row.actionEnvelopeDigest, &row.projectBindingDigest,
		&row.profileAuthorRef, &row.profileAuthorDigest,
		&row.claimScopeRef, &row.boundedContextRef,
		&row.methodDescriptionRef, &row.methodDescriptionDigest,
		&row.methodContractRef, &row.methodContractDigest,
		&row.classifierVersion, &row.policyVersion, &row.futureWorkSessionRef,
		&row.allowedWorkFrom, &row.allowedWorkUntil,
		&row.basisObservationFrom, &row.basisObservationUntil,
		&row.authorizationValidFrom, &row.authorizationValidUntil,
		&row.permissionValidFrom, &row.permissionValidUntil,
		&row.singleUseKey, &row.enactabilityPredicateRef,
		&row.verifierIdentity, &row.verifierVersion,
		&row.verificationPolicyRef, &row.verificationPolicyDigest,
		&row.checkedAt, &row.roleStateRelation, &row.enactableState,
		&row.currentnessResult, &row.predicateResult, &row.admissionResult,
		&row.canonical, &row.recordedAt,
	}
}

func (row resolutionRow) args() []any {
	return []any{
		row.ref, row.digest, row.basisRef, row.basisDigest,
		row.speechActRef, row.speechActDigest,
		row.contentRef, row.contentDigest,
		row.permissionRef, row.permissionDigest,
		row.contextPolicyRef, row.contextPolicyDigest,
		row.actionKind, row.projectRoot,
		row.actionEnvelopeDigest, row.projectBindingDigest,
		row.profileAuthorRef, row.profileAuthorDigest,
		row.claimScopeRef, row.boundedContextRef,
		row.methodDescriptionRef, row.methodDescriptionDigest,
		row.methodContractRef, row.methodContractDigest,
		row.classifierVersion, row.policyVersion, row.futureWorkSessionRef,
		row.allowedWorkFrom, row.allowedWorkUntil,
		row.basisObservationFrom, row.basisObservationUntil,
		row.authorizationValidFrom, row.authorizationValidUntil,
		row.permissionValidFrom, row.permissionValidUntil,
		row.singleUseKey, row.enactabilityPredicateRef,
		row.verifierIdentity, row.verifierVersion,
		row.verificationPolicyRef, row.verificationPolicyDigest,
		row.checkedAt, row.roleStateRelation, row.enactableState,
		row.currentnessResult, row.predicateResult, row.admissionResult,
		row.canonical, row.recordedAt,
	}
}

type resolutionCanonicalJSON struct {
	Schema                   string `json:"schema"`
	Ref                      string `json:"authority_resolution_ref"`
	BasisRef                 string `json:"authority_basis_ref"`
	BasisDigest              string `json:"authority_basis_digest"`
	SpeechActRef             string `json:"speech_act_ref"`
	SpeechActDigest          string `json:"speech_act_digest"`
	ContentRef               string `json:"authorization_content_ref"`
	ContentDigest            string `json:"authorization_content_digest"`
	PermissionRef            string `json:"permission_ref"`
	PermissionDigest         string `json:"permission_digest"`
	ContextPolicyRef         string `json:"context_policy_ref"`
	ContextPolicyDigest      string `json:"context_policy_digest"`
	ActionKind               string `json:"action_kind"`
	ProjectRoot              string `json:"project_root"`
	ActionEnvelopeDigest     string `json:"action_envelope_digest"`
	ProjectBindingDigest     string `json:"project_binding_digest"`
	ProfileAuthorRef         string `json:"profile_author_role_assignment_ref"`
	ProfileAuthorDigest      string `json:"profile_author_role_assignment_digest"`
	ClaimScopeRef            string `json:"claim_scope_ref"`
	BoundedContextRef        string `json:"bounded_context_ref"`
	MethodDescriptionRef     string `json:"method_description_ref"`
	MethodDescriptionDigest  string `json:"method_description_digest"`
	MethodContractRef        string `json:"method_contract_ref"`
	MethodContractDigest     string `json:"method_contract_digest"`
	ClassifierVersion        string `json:"classifier_version"`
	PolicyVersion            string `json:"policy_version"`
	FutureWorkSessionRef     string `json:"future_work_session_ref"`
	AllowedWorkFrom          string `json:"allowed_work_from"`
	AllowedWorkUntil         string `json:"allowed_work_until"`
	BasisObservationFrom     string `json:"basis_observation_from"`
	BasisObservationUntil    string `json:"basis_observation_until"`
	AuthorizationValidFrom   string `json:"authorization_valid_from"`
	AuthorizationValidUntil  string `json:"authorization_valid_until"`
	PermissionValidFrom      string `json:"permission_valid_from"`
	PermissionValidUntil     string `json:"permission_valid_until"`
	SingleUseKey             string `json:"single_use_key"`
	EnactabilityPredicateRef string `json:"enactability_predicate_ref"`
	VerifierIdentity         string `json:"verifier_identity"`
	VerifierVersion          string `json:"verifier_version"`
	VerificationPolicyRef    string `json:"verification_policy_ref"`
	VerificationPolicyDigest string `json:"verification_policy_digest"`
	CheckedAt                string `json:"checked_at"`
	RoleStateRelation        string `json:"role_state_relation"`
	EnactableState           string `json:"enactable_state"`
	CurrentnessResult        string `json:"currentness_result"`
	PredicateResult          string `json:"predicate_result"`
	AdmissionResult          string `json:"admission_result"`
}

func scanResolutionByRef(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref string,
) (resolutionRow, bool, error) {
	return scanResolution(ctx, transaction, selectResolutionSQL+" WHERE authority_resolution_ref = ?", ref)
}

func scanResolutionByBasis(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	ref string,
) (resolutionRow, bool, error) {
	return scanResolution(ctx, transaction, selectResolutionSQL+" WHERE authority_basis_ref = ?", ref)
}

func scanResolution(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	statement string,
	value string,
) (resolutionRow, bool, error) {
	row := resolutionRow{}
	err := transaction.ScanOne(ctx, statement, []any{value}, row.scanTargets())
	if err == sql.ErrNoRows {
		return resolutionRow{}, false, nil
	}
	if err != nil {
		return resolutionRow{}, false, fmt.Errorf("scan profile authority resolution: %w", err)
	}
	return row, true, nil
}

func reconstructResolution(
	row resolutionRow,
	closure profileauthority.Closure,
) (profileauthority.AuthorityResolutionRecord, error) {
	ref, err := profileauthority.NewProfileDeclarationAuthorityResolutionRef(row.ref)
	if err != nil {
		return profileauthority.AuthorityResolutionRecord{}, fmt.Errorf("parse authority resolution ref: %w", err)
	}
	checkedAt, err := parseCanonicalTime(row.checkedAt)
	if err != nil {
		return profileauthority.AuthorityResolutionRecord{}, fmt.Errorf("parse authority resolution checked_at: %w", err)
	}
	result := profileauthority.EvaluateNewResolution(ref, closure, checkedAt)
	if result.Kind() != profileauthority.ResolutionNew {
		return profileauthority.AuthorityResolutionRecord{}, fmt.Errorf("stored authority resolution no longer passes pure reconstruction")
	}
	created, ok := result.New()
	if !ok {
		return profileauthority.AuthorityResolutionRecord{}, fmt.Errorf("pure authority resolution reconstruction returned no record")
	}
	record, ok := created.Record()
	if !ok {
		return profileauthority.AuthorityResolutionRecord{}, fmt.Errorf("pure authority resolution record is unavailable")
	}
	if err := validateResolutionRow(row, record); err != nil {
		return profileauthority.AuthorityResolutionRecord{}, err
	}
	return record, nil
}

func validateResolutionRow(
	row resolutionRow,
	record profileauthority.AuthorityResolutionRecord,
) error {
	digest, digestOK := record.Digest()
	canonical, canonicalOK := record.CanonicalBytes()
	recordedAt, recordedAtErr := parseCanonicalTime(row.recordedAt)
	checkedAt, checkedAtOK := record.CheckedAt()
	valid := digestOK && canonicalOK && recordedAtErr == nil && checkedAtOK
	valid = valid && digest.String() == row.digest
	valid = valid && slices.Equal(canonical, []byte(row.canonical))
	valid = valid && recordedAt.Equal(checkedAt)
	if !valid {
		return fmt.Errorf("stored profile authority resolution failed canonical rehash")
	}
	expected := resolutionCanonicalJSON{}
	if err := json.Unmarshal(canonical, &expected); err != nil {
		return fmt.Errorf("decode canonical profile authority resolution: %w", err)
	}
	actual := resolutionJSONFromRow(row)
	if actual != expected {
		return fmt.Errorf("stored profile authority resolution columns differ from canonical content")
	}
	return nil
}

func resolutionJSONFromRow(row resolutionRow) resolutionCanonicalJSON {
	return resolutionCanonicalJSON{
		Schema: "haft.profile-authority.authority-resolution/v2",
		Ref:    row.ref, BasisRef: row.basisRef, BasisDigest: row.basisDigest,
		SpeechActRef: row.speechActRef, SpeechActDigest: row.speechActDigest,
		ContentRef: row.contentRef, ContentDigest: row.contentDigest,
		PermissionRef: row.permissionRef, PermissionDigest: row.permissionDigest,
		ContextPolicyRef: row.contextPolicyRef, ContextPolicyDigest: row.contextPolicyDigest,
		ActionKind: row.actionKind, ProjectRoot: row.projectRoot,
		ActionEnvelopeDigest: row.actionEnvelopeDigest, ProjectBindingDigest: row.projectBindingDigest,
		ProfileAuthorRef: row.profileAuthorRef, ProfileAuthorDigest: row.profileAuthorDigest,
		ClaimScopeRef: row.claimScopeRef, BoundedContextRef: row.boundedContextRef,
		MethodDescriptionRef: row.methodDescriptionRef, MethodDescriptionDigest: row.methodDescriptionDigest,
		MethodContractRef: row.methodContractRef, MethodContractDigest: row.methodContractDigest,
		ClassifierVersion: row.classifierVersion, PolicyVersion: row.policyVersion,
		FutureWorkSessionRef: row.futureWorkSessionRef,
		AllowedWorkFrom:      row.allowedWorkFrom, AllowedWorkUntil: row.allowedWorkUntil,
		BasisObservationFrom: row.basisObservationFrom, BasisObservationUntil: row.basisObservationUntil,
		AuthorizationValidFrom: row.authorizationValidFrom, AuthorizationValidUntil: row.authorizationValidUntil,
		PermissionValidFrom: row.permissionValidFrom, PermissionValidUntil: row.permissionValidUntil,
		SingleUseKey: row.singleUseKey, EnactabilityPredicateRef: row.enactabilityPredicateRef,
		VerifierIdentity: row.verifierIdentity, VerifierVersion: row.verifierVersion,
		VerificationPolicyRef:    row.verificationPolicyRef,
		VerificationPolicyDigest: row.verificationPolicyDigest,
		CheckedAt:                row.checkedAt, RoleStateRelation: row.roleStateRelation,
		EnactableState: row.enactableState, CurrentnessResult: row.currentnessResult,
		PredicateResult: row.predicateResult, AdmissionResult: row.admissionResult,
	}
}

func buildResolutionRow(
	record profileauthority.AuthorityResolutionRecord,
) (resolutionRow, error) {
	digest, digestOK := record.Digest()
	canonical, canonicalOK := record.CanonicalBytes()
	checkedAt, checkedAtOK := record.CheckedAt()
	if !digestOK || !canonicalOK || !checkedAtOK {
		return resolutionRow{}, fmt.Errorf("canonical authority resolution is unavailable")
	}
	dto := resolutionCanonicalJSON{}
	if err := json.Unmarshal(canonical, &dto); err != nil {
		return resolutionRow{}, fmt.Errorf("decode authority resolution for persistence: %w", err)
	}
	row := resolutionRow{
		ref: dto.Ref, digest: digest.String(), basisRef: dto.BasisRef, basisDigest: dto.BasisDigest,
		speechActRef: dto.SpeechActRef, speechActDigest: dto.SpeechActDigest,
		contentRef: dto.ContentRef, contentDigest: dto.ContentDigest,
		permissionRef: dto.PermissionRef, permissionDigest: dto.PermissionDigest,
		contextPolicyRef: dto.ContextPolicyRef, contextPolicyDigest: dto.ContextPolicyDigest,
		actionKind: dto.ActionKind, projectRoot: dto.ProjectRoot,
		actionEnvelopeDigest: dto.ActionEnvelopeDigest, projectBindingDigest: dto.ProjectBindingDigest,
		profileAuthorRef: dto.ProfileAuthorRef, profileAuthorDigest: dto.ProfileAuthorDigest,
		claimScopeRef: dto.ClaimScopeRef, boundedContextRef: dto.BoundedContextRef,
		methodDescriptionRef: dto.MethodDescriptionRef, methodDescriptionDigest: dto.MethodDescriptionDigest,
		methodContractRef: dto.MethodContractRef, methodContractDigest: dto.MethodContractDigest,
		classifierVersion: dto.ClassifierVersion, policyVersion: dto.PolicyVersion,
		futureWorkSessionRef: dto.FutureWorkSessionRef,
		allowedWorkFrom:      dto.AllowedWorkFrom, allowedWorkUntil: dto.AllowedWorkUntil,
		basisObservationFrom: dto.BasisObservationFrom, basisObservationUntil: dto.BasisObservationUntil,
		authorizationValidFrom: dto.AuthorizationValidFrom, authorizationValidUntil: dto.AuthorizationValidUntil,
		permissionValidFrom: dto.PermissionValidFrom, permissionValidUntil: dto.PermissionValidUntil,
		singleUseKey: dto.SingleUseKey, enactabilityPredicateRef: dto.EnactabilityPredicateRef,
		verifierIdentity: dto.VerifierIdentity, verifierVersion: dto.VerifierVersion,
		verificationPolicyRef:    dto.VerificationPolicyRef,
		verificationPolicyDigest: dto.VerificationPolicyDigest,
		checkedAt:                dto.CheckedAt, roleStateRelation: dto.RoleStateRelation,
		enactableState: dto.EnactableState, currentnessResult: dto.CurrentnessResult,
		predicateResult: dto.PredicateResult, admissionResult: dto.AdmissionResult,
		canonical: string(canonical), recordedAt: formatTime(checkedAt),
	}
	if row.ref == "" || row.checkedAt == "" {
		return resolutionRow{}, fmt.Errorf("canonical authority resolution omitted required identity")
	}
	return row, nil
}

func deriveResolutionRef(
	basisDigest authority.Digest,
) (profileauthority.ProfileDeclarationAuthorityResolutionRef, error) {
	suffix := strings.TrimPrefix(basisDigest.String(), "sha256:")
	return profileauthority.NewProfileDeclarationAuthorityResolutionRef(
		"profile-authority-resolution:" + suffix,
	)
}

func resolutionBasis(row resolutionRow) (
	profileauthority.BasisRef,
	authority.Digest,
	error,
) {
	ref, err := profileauthority.NewBasisRef(row.basisRef)
	if err != nil {
		return profileauthority.BasisRef{}, authority.Digest{}, err
	}
	digest, err := authority.NewDigest(row.basisDigest)
	if err != nil {
		return profileauthority.BasisRef{}, authority.Digest{}, err
	}
	return ref, digest, nil
}
