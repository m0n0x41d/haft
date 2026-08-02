package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	projectprofilesqlite "github.com/m0n0x41d/haft/internal/projectprofile/sqlite"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const (
	v4AuthorityMode  = "automatic_supported_singleton_init"
	v4ResolutionKind = "deterministic_policy_satisfaction"
	v4ActionKind     = "profile.apply_supported_singleton_default"
	v4ProfileOrigin  = "detector_default"

	v4BasisSchema      = "haft.profile-authority.automatic-basis/v1"
	v4ResolutionSchema = "haft.profile-authority.automatic-resolution/v1"
	v4UseSchema        = "haft.profile-authority.automatic-use/v1"
)

type directProfileAuthorityContract struct {
	generation       string
	mode             string
	resolutionKind   string
	actionKind       string
	origin           string
	basisSchema      string
	resolutionSchema string
	useSchema        string
	hostRouted       bool
}

var v4DirectProfileAuthorityContract = directProfileAuthorityContract{
	generation: "v4 automatic", mode: v4AuthorityMode,
	resolutionKind: v4ResolutionKind, actionKind: v4ActionKind,
	origin: v4ProfileOrigin, basisSchema: v4BasisSchema,
	resolutionSchema: v4ResolutionSchema, useSchema: v4UseSchema,
}

const selectV4BasisDiscoverySQL = `SELECT authority_mode
FROM profile_initial_bootstrap_authority_bases_v1
WHERE basis_ref = ?`

const selectV4BasisSQL = `SELECT
	basis_ref, basis_digest, project_root, action_kind, authority_mode,
	profile_origin, work_input_ref, work_input_digest,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest,
	method_description_ref, method_description_digest,
	method_contract_ref, method_contract_digest,
	classifier_version, policy_version, suggestion_ref, observation_digest,
	future_work_session_ref, allowed_work_from, allowed_work_until,
	basis_observation_from, basis_observation_until, single_use_key,
	canonical_json, recorded_at
FROM profile_initial_bootstrap_authority_bases_v1
WHERE basis_ref = ?`

const selectV4ResolutionByBasisSQL = `SELECT
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	project_root, action_kind, authority_mode, resolution_kind, profile_origin,
	work_input_ref, work_input_digest, project_binding_digest,
	detector_version, detector_policy_version, suggestion_ref, observation_digest,
	verifier_identity, verifier_version,
	verification_policy_ref, verification_policy_digest,
	checked_at, currentness_result, predicate_result, admission_result,
	canonical_json, recorded_at
FROM profile_initial_bootstrap_authority_resolutions_v1
WHERE authority_basis_ref = ?`

const selectV4UseByAdmissionSQL = `SELECT
	use_ref, use_digest, project_root, action_kind, authority_mode,
	resolution_kind, profile_origin, project_binding_digest,
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	work_input_ref, work_input_digest, single_use_key,
	admission_request_digest, committed_admission_ref,
	committed_admission_digest, canonical_json, consumed_at
FROM profile_declaration_authority_uses_v4
WHERE committed_admission_ref = ?`

type v4AuthorityBasisRow struct {
	ref                     string
	digest                  string
	projectRoot             string
	actionKind              string
	mode                    string
	origin                  string
	operatorRequestRef      string
	operatorRequestDigest   string
	operatorSubjectRef      string
	operatorPayloadDigest   string
	workInputRef            string
	workInputDigest         string
	profileAuthorRef        string
	profileAuthorDigest     string
	methodDescriptionRef    string
	methodDescriptionDigest string
	methodContractRef       string
	methodContractDigest    string
	classifierVersion       string
	policyVersion           string
	suggestionRef           string
	observationDigest       string
	futureWorkSessionRef    string
	allowedWorkFrom         string
	allowedWorkUntil        string
	basisObservationFrom    string
	basisObservationUntil   string
	singleUseKey            string
	canonicalJSON           string
	recordedAt              string
}

func (row *v4AuthorityBasisRow) scanTargets() []any {
	return []any{
		&row.ref, &row.digest, &row.projectRoot, &row.actionKind, &row.mode,
		&row.origin, &row.workInputRef, &row.workInputDigest,
		&row.profileAuthorRef, &row.profileAuthorDigest,
		&row.methodDescriptionRef, &row.methodDescriptionDigest,
		&row.methodContractRef, &row.methodContractDigest,
		&row.classifierVersion, &row.policyVersion, &row.suggestionRef,
		&row.observationDigest, &row.futureWorkSessionRef,
		&row.allowedWorkFrom, &row.allowedWorkUntil,
		&row.basisObservationFrom, &row.basisObservationUntil,
		&row.singleUseKey, &row.canonicalJSON, &row.recordedAt,
	}
}

func (row *v4AuthorityBasisRow) scanTargetsV5() []any {
	return []any{
		&row.ref, &row.digest, &row.projectRoot, &row.actionKind, &row.mode,
		&row.origin, &row.operatorRequestRef, &row.operatorRequestDigest,
		&row.operatorSubjectRef, &row.operatorPayloadDigest,
		&row.workInputRef, &row.workInputDigest,
		&row.profileAuthorRef, &row.profileAuthorDigest,
		&row.methodDescriptionRef, &row.methodDescriptionDigest,
		&row.methodContractRef, &row.methodContractDigest,
		&row.classifierVersion, &row.policyVersion, &row.suggestionRef,
		&row.observationDigest, &row.futureWorkSessionRef,
		&row.allowedWorkFrom, &row.allowedWorkUntil,
		&row.basisObservationFrom, &row.basisObservationUntil,
		&row.singleUseKey, &row.canonicalJSON, &row.recordedAt,
	}
}

type v4AuthorityResolutionRow struct {
	ref                      string
	digest                   string
	basisRef                 string
	basisDigest              string
	projectRoot              string
	actionKind               string
	mode                     string
	resolutionKind           string
	origin                   string
	operatorRequestRef       string
	operatorRequestDigest    string
	operatorSubjectRef       string
	operatorPayloadDigest    string
	workInputRef             string
	workInputDigest          string
	projectBindingDigest     string
	detectorVersion          string
	detectorPolicyVersion    string
	suggestionRef            string
	observationDigest        string
	verifierIdentity         string
	verifierVersion          string
	verificationPolicyRef    string
	verificationPolicyDigest string
	checkedAt                string
	currentnessResult        string
	predicateResult          string
	admissionResult          string
	canonicalJSON            string
	recordedAt               string
}

func (row *v4AuthorityResolutionRow) scanTargets() []any {
	return []any{
		&row.ref, &row.digest, &row.basisRef, &row.basisDigest,
		&row.projectRoot, &row.actionKind, &row.mode, &row.resolutionKind,
		&row.origin, &row.workInputRef, &row.workInputDigest,
		&row.projectBindingDigest, &row.detectorVersion,
		&row.detectorPolicyVersion, &row.suggestionRef,
		&row.observationDigest, &row.verifierIdentity, &row.verifierVersion,
		&row.verificationPolicyRef, &row.verificationPolicyDigest,
		&row.checkedAt, &row.currentnessResult, &row.predicateResult,
		&row.admissionResult, &row.canonicalJSON, &row.recordedAt,
	}
}

func (row *v4AuthorityResolutionRow) scanTargetsV5() []any {
	return []any{
		&row.ref, &row.digest, &row.basisRef, &row.basisDigest,
		&row.projectRoot, &row.actionKind, &row.mode, &row.resolutionKind,
		&row.origin, &row.operatorRequestRef, &row.operatorRequestDigest,
		&row.operatorSubjectRef, &row.operatorPayloadDigest,
		&row.workInputRef, &row.workInputDigest,
		&row.projectBindingDigest, &row.detectorVersion,
		&row.detectorPolicyVersion, &row.suggestionRef,
		&row.observationDigest, &row.verifierIdentity, &row.verifierVersion,
		&row.verificationPolicyRef, &row.verificationPolicyDigest,
		&row.checkedAt, &row.currentnessResult, &row.predicateResult,
		&row.admissionResult, &row.canonicalJSON, &row.recordedAt,
	}
}

type v4AuthorityClosure struct {
	basis      v4AuthorityBasisRow
	resolution v4AuthorityResolutionRow
	workInput  v3WorkInputRow
}

type v4AuthorityBasisJSON struct {
	Schema                            string `json:"schema"`
	BasisRef                          string `json:"basis_ref"`
	ProjectRoot                       string `json:"project_root"`
	ActionKind                        string `json:"action_kind"`
	AuthorityMode                     string `json:"authority_mode"`
	ProfileOrigin                     string `json:"profile_origin"`
	OperatorRequestRef                string `json:"operator_request_ref,omitempty"`
	OperatorRequestDigest             string `json:"operator_request_digest,omitempty"`
	OperatorRequestSubjectRef         string `json:"operator_request_subject_ref,omitempty"`
	OperatorRequestPayloadDigest      string `json:"operator_request_payload_digest,omitempty"`
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

type v4AuthorityResolutionJSON struct {
	Schema                       string `json:"schema"`
	AuthorityResolutionRef       string `json:"authority_resolution_ref"`
	AuthorityBasisRef            string `json:"authority_basis_ref"`
	AuthorityBasisDigest         string `json:"authority_basis_digest"`
	ProjectRoot                  string `json:"project_root"`
	ActionKind                   string `json:"action_kind"`
	AuthorityMode                string `json:"authority_mode"`
	ResolutionKind               string `json:"resolution_kind"`
	ProfileOrigin                string `json:"profile_origin"`
	OperatorRequestRef           string `json:"operator_request_ref,omitempty"`
	OperatorRequestDigest        string `json:"operator_request_digest,omitempty"`
	OperatorRequestSubjectRef    string `json:"operator_request_subject_ref,omitempty"`
	OperatorRequestPayloadDigest string `json:"operator_request_payload_digest,omitempty"`
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

type v4AuthorityUseJSON struct {
	Schema                       string `json:"schema"`
	UseRef                       string `json:"use_ref"`
	ProjectRoot                  string `json:"project_root"`
	ActionKind                   string `json:"action_kind"`
	AuthorityMode                string `json:"authority_mode"`
	ResolutionKind               string `json:"resolution_kind"`
	ProfileOrigin                string `json:"profile_origin"`
	OperatorRequestRef           string `json:"operator_request_ref,omitempty"`
	OperatorRequestDigest        string `json:"operator_request_digest,omitempty"`
	OperatorRequestSubjectRef    string `json:"operator_request_subject_ref,omitempty"`
	OperatorRequestPayloadDigest string `json:"operator_request_payload_digest,omitempty"`
	ProjectBindingDigest         string `json:"project_binding_digest"`
	AuthorityResolutionRef       string `json:"authority_resolution_ref"`
	AuthorityResolutionDigest    string `json:"authority_resolution_digest"`
	AuthorityBasisRef            string `json:"authority_basis_ref"`
	AuthorityBasisDigest         string `json:"authority_basis_digest"`
	WorkInputRef                 string `json:"work_input_ref"`
	WorkInputDigest              string `json:"work_input_digest"`
	SingleUseKey                 string `json:"single_use_key"`
	AdmissionRequestDigest       string `json:"admission_request_digest"`
	CommittedAdmissionRef        string `json:"committed_admission_ref"`
	CommittedAdmissionDigest     string `json:"committed_admission_digest"`
	ConsumedAt                   string `json:"consumed_at"`
}

type v4AuthorityUseRecord struct {
	ref                       string
	digest                    string
	projectRoot               string
	actionKind                string
	mode                      string
	resolutionKind            string
	origin                    string
	projectBindingDigest      string
	authorityResolutionRef    string
	authorityResolutionDigest string
	authorityBasisRef         string
	authorityBasisDigest      string
	workInputRef              string
	workInputDigest           string
	singleUseKey              string
	admissionRequestDigest    string
	committedAdmissionRef     string
	committedAdmissionDigest  string
	canonicalJSON             string
	consumedAt                string
}

func (record *v4AuthorityUseRecord) scanTargets() []any {
	return []any{
		&record.ref, &record.digest, &record.projectRoot, &record.actionKind,
		&record.mode, &record.resolutionKind, &record.origin,
		&record.projectBindingDigest, &record.authorityResolutionRef,
		&record.authorityResolutionDigest, &record.authorityBasisRef,
		&record.authorityBasisDigest, &record.workInputRef,
		&record.workInputDigest, &record.singleUseKey,
		&record.admissionRequestDigest, &record.committedAdmissionRef,
		&record.committedAdmissionDigest, &record.canonicalJSON,
		&record.consumedAt,
	}
}

func discoverV4Basis(
	ctx context.Context,
	database *sql.DB,
	basisRef string,
) (bool, error) {
	mode := ""
	err := database.QueryRowContext(ctx, selectV4BasisDiscoverySQL, basisRef).Scan(&mode)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("discover v4 profile authority basis: %w", err)
	}
	if mode != v4AuthorityMode {
		return false, fmt.Errorf("v4 profile authority basis has unknown mode %q", mode)
	}
	return true, nil
}

func loadV4AuthorityClosure(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basisRef string,
) (v4AuthorityClosure, error) {
	basis := v4AuthorityBasisRow{}
	err := transaction.ScanOne(ctx, selectV4BasisSQL, []any{basisRef}, basis.scanTargets())
	if err != nil {
		return v4AuthorityClosure{}, fmt.Errorf("load v4 profile authority basis: %w", err)
	}
	resolution := v4AuthorityResolutionRow{}
	err = transaction.ScanOne(
		ctx,
		selectV4ResolutionByBasisSQL,
		[]any{basisRef},
		resolution.scanTargets(),
	)
	if err != nil {
		return v4AuthorityClosure{}, fmt.Errorf("load v4 profile authority resolution: %w", err)
	}
	workInput := v3WorkInputRow{}
	err = transaction.ScanOne(
		ctx,
		selectV3WorkInputSQL,
		[]any{basis.workInputRef},
		workInput.scanTargets(),
	)
	if err != nil {
		return v4AuthorityClosure{}, fmt.Errorf("load v4 profile WorkInput: %w", err)
	}
	closure := v4AuthorityClosure{
		basis: basis, resolution: resolution, workInput: workInput,
	}
	if err := validateV4AuthorityClosure(closure); err != nil {
		return v4AuthorityClosure{}, err
	}
	return closure, nil
}

func validateV4AuthorityClosure(closure v4AuthorityClosure) error {
	return validateDirectProfileAuthorityClosure(
		closure,
		v4DirectProfileAuthorityContract,
	)
}

func validateDirectProfileAuthorityClosure(
	closure v4AuthorityClosure,
	contract directProfileAuthorityContract,
) error {
	if err := validateDirectProfileAuthorityBasis(closure.basis, contract); err != nil {
		return err
	}
	if err := validateDirectProfileAuthorityResolution(closure.resolution, contract); err != nil {
		return err
	}
	if err := validateV3WorkInput(closure.workInput); err != nil {
		return err
	}
	basis := closure.basis
	resolution := closure.resolution
	workInput := closure.workInput
	basisRecordedAt, err := parseV3Time(basis.recordedAt)
	if err != nil {
		return err
	}
	workInputRecordedAt, err := parseV3Time(workInput.recordedAt)
	if err != nil {
		return err
	}
	checkedAt, err := parseV3Time(resolution.checkedAt)
	if err != nil {
		return err
	}
	allowedWork, err := parseV3Window(basis.allowedWorkFrom, basis.allowedWorkUntil)
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: resolution.basisRef == basis.ref, name: "resolution basis ref"},
		{matches: resolution.basisDigest == basis.digest, name: "resolution basis digest"},
		{matches: resolution.projectRoot == basis.projectRoot, name: "resolution project root"},
		{matches: resolution.actionKind == basis.actionKind, name: "resolution action kind"},
		{matches: resolution.mode == basis.mode, name: "resolution authority mode"},
		{matches: resolution.origin == basis.origin, name: "resolution profile origin"},
		{matches: resolution.operatorRequestRef == basis.operatorRequestRef, name: "resolution operator request ref"},
		{matches: resolution.operatorRequestDigest == basis.operatorRequestDigest, name: "resolution operator request digest"},
		{matches: resolution.operatorSubjectRef == basis.operatorSubjectRef, name: "resolution operator subject"},
		{matches: resolution.operatorPayloadDigest == basis.operatorPayloadDigest, name: "resolution operator payload"},
		{matches: resolution.workInputRef == basis.workInputRef, name: "resolution WorkInput ref"},
		{matches: resolution.workInputDigest == basis.workInputDigest, name: "resolution WorkInput digest"},
		{matches: resolution.detectorVersion == basis.classifierVersion, name: "resolution detector version"},
		{matches: resolution.detectorPolicyVersion == basis.policyVersion, name: "resolution detector policy"},
		{matches: resolution.suggestionRef == basis.suggestionRef, name: "resolution suggestion ref"},
		{matches: resolution.observationDigest == basis.observationDigest, name: "resolution observation digest"},
		{matches: workInput.ref == basis.workInputRef, name: "basis WorkInput ref"},
		{matches: workInput.digest == basis.workInputDigest, name: "basis WorkInput digest"},
		{matches: workInput.projectRoot == basis.projectRoot, name: "WorkInput project root"},
		{matches: workInput.detectorVersion == basis.classifierVersion, name: "WorkInput detector version"},
		{matches: workInput.policyVersion == basis.policyVersion, name: "WorkInput policy version"},
		{matches: workInput.suggestionRef == basis.suggestionRef, name: "WorkInput suggestion ref"},
		{matches: workInput.observationDigest == basis.observationDigest, name: "WorkInput observation digest"},
		{matches: !basisRecordedAt.Before(workInputRecordedAt), name: "WorkInput recorded before basis"},
		{matches: allowedWork.Contains(checkedAt), name: "resolution inside allowed Work window"},
	}
	return firstMismatch(checks, contract.generation+" profile authority closure")
}

func validateV4AuthorityBasis(row v4AuthorityBasisRow) error {
	return validateDirectProfileAuthorityBasis(row, v4DirectProfileAuthorityContract)
}

func validateDirectProfileAuthorityBasis(
	row v4AuthorityBasisRow,
	contract directProfileAuthorityContract,
) error {
	value := v4AuthorityBasisJSON{
		Schema: contract.basisSchema, BasisRef: row.ref, ProjectRoot: row.projectRoot,
		ActionKind: row.actionKind, AuthorityMode: row.mode, ProfileOrigin: row.origin,
		OperatorRequestRef:           row.operatorRequestRef,
		OperatorRequestDigest:        row.operatorRequestDigest,
		OperatorRequestSubjectRef:    row.operatorSubjectRef,
		OperatorRequestPayloadDigest: row.operatorPayloadDigest,
		WorkInputRef:                 row.workInputRef, WorkInputDigest: row.workInputDigest,
		ProfileAuthorRoleAssignmentRef:    row.profileAuthorRef,
		ProfileAuthorRoleAssignmentDigest: row.profileAuthorDigest,
		MethodDescriptionRef:              row.methodDescriptionRef,
		MethodDescriptionDigest:           row.methodDescriptionDigest,
		MethodContractRef:                 row.methodContractRef,
		MethodContractDigest:              row.methodContractDigest,
		ClassifierVersion:                 row.classifierVersion, PolicyVersion: row.policyVersion,
		SuggestionRef: row.suggestionRef, ObservationDigest: row.observationDigest,
		FutureWorkSessionRef: row.futureWorkSessionRef,
		AllowedWorkFrom:      row.allowedWorkFrom, AllowedWorkUntil: row.allowedWorkUntil,
		BasisObservationFrom:  row.basisObservationFrom,
		BasisObservationUntil: row.basisObservationUntil,
		SingleUseKey:          row.singleUseKey,
	}
	if err := requireCanonicalV3JSON(contract.basisSchema, row.canonicalJSON, row.digest, value); err != nil {
		return fmt.Errorf("validate %s authority basis: %w", contract.generation, err)
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: row.actionKind == contract.actionKind, name: "action kind"},
		{matches: row.mode == contract.mode, name: "authority mode"},
		{matches: row.origin == contract.origin, name: "profile origin"},
	}
	if err := firstMismatch(checks, contract.generation+" authority basis"); err != nil {
		return err
	}
	validators := []error{}
	_, err := profileauthority.NewBasisRef(row.ref)
	validators = append(validators, err)
	_, err = authority.NewDigest(row.digest)
	validators = append(validators, err)
	_, err = authority.NewProjectRoot(row.projectRoot)
	validators = append(validators, err)
	_, err = projectprofile.NewWorkInputRef(row.workInputRef)
	validators = append(validators, err)
	_, err = authority.NewDigest(row.workInputDigest)
	validators = append(validators, err)
	_, err = authority.NewRoleAssignmentRef(row.profileAuthorRef)
	validators = append(validators, err)
	_, err = authority.NewDigest(row.profileAuthorDigest)
	validators = append(validators, err)
	_, err = authority.NewMethodDescriptionRef(row.methodDescriptionRef)
	validators = append(validators, err)
	_, err = authority.NewDigest(row.methodDescriptionDigest)
	validators = append(validators, err)
	_, err = authority.NewMethodContractRef(row.methodContractRef)
	validators = append(validators, err)
	_, err = authority.NewDigest(row.methodContractDigest)
	validators = append(validators, err)
	_, err = authority.NewClassifierVersion(row.classifierVersion)
	validators = append(validators, err)
	_, err = authority.NewPolicyVersion(row.policyVersion)
	validators = append(validators, err)
	_, err = authority.NewSessionRef(row.futureWorkSessionRef)
	validators = append(validators, err)
	_, err = authority.NewSingleUseKey(row.singleUseKey)
	validators = append(validators, err)
	_, err = projectprofile.NewContentDigest(row.observationDigest)
	validators = append(validators, err)
	_, err = parseV3Window(row.allowedWorkFrom, row.allowedWorkUntil)
	validators = append(validators, err)
	_, err = parseV3Window(row.basisObservationFrom, row.basisObservationUntil)
	validators = append(validators, err)
	_, err = parseV3Time(row.recordedAt)
	validators = append(validators, err)
	if contract.hostRouted {
		if row.operatorRequestRef == "" || row.operatorSubjectRef == "" {
			validators = append(validators, fmt.Errorf("host-routed operator request identity is incomplete"))
		}
		_, err = authority.NewDigest(row.operatorRequestDigest)
		validators = append(validators, err)
		_, err = authority.NewDigest(row.operatorPayloadDigest)
		validators = append(validators, err)
	}
	return errors.Join(validators...)
}

func validateV4AuthorityResolution(row v4AuthorityResolutionRow) error {
	return validateDirectProfileAuthorityResolution(row, v4DirectProfileAuthorityContract)
}

func validateDirectProfileAuthorityResolution(
	row v4AuthorityResolutionRow,
	contract directProfileAuthorityContract,
) error {
	value := v4AuthorityResolutionJSON{
		Schema: contract.resolutionSchema, AuthorityResolutionRef: row.ref,
		AuthorityBasisRef: row.basisRef, AuthorityBasisDigest: row.basisDigest,
		ProjectRoot: row.projectRoot, ActionKind: row.actionKind,
		AuthorityMode: row.mode, ResolutionKind: row.resolutionKind,
		ProfileOrigin: row.origin, WorkInputRef: row.workInputRef,
		OperatorRequestRef:           row.operatorRequestRef,
		OperatorRequestDigest:        row.operatorRequestDigest,
		OperatorRequestSubjectRef:    row.operatorSubjectRef,
		OperatorRequestPayloadDigest: row.operatorPayloadDigest,
		WorkInputDigest:              row.workInputDigest,
		ProjectBindingDigest:         row.projectBindingDigest,
		DetectorVersion:              row.detectorVersion,
		DetectorPolicyVersion:        row.detectorPolicyVersion,
		SuggestionRef:                row.suggestionRef, ObservationDigest: row.observationDigest,
		VerifierIdentity: row.verifierIdentity, VerifierVersion: row.verifierVersion,
		VerificationPolicyRef:    row.verificationPolicyRef,
		VerificationPolicyDigest: row.verificationPolicyDigest,
		CheckedAt:                row.checkedAt, CurrentnessResult: row.currentnessResult,
		PredicateResult: row.predicateResult, AdmissionResult: row.admissionResult,
	}
	if err := requireCanonicalV3JSON(contract.resolutionSchema, row.canonicalJSON, row.digest, value); err != nil {
		return fmt.Errorf("validate %s authority resolution: %w", contract.generation, err)
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: row.actionKind == contract.actionKind, name: "action kind"},
		{matches: row.mode == contract.mode, name: "authority mode"},
		{matches: row.resolutionKind == contract.resolutionKind, name: "resolution kind"},
		{matches: row.origin == contract.origin, name: "profile origin"},
		{matches: row.verifierIdentity == "haft-core", name: "verifier identity"},
		{matches: row.recordedAt == row.checkedAt, name: "recording time"},
		{matches: row.currentnessResult == "current", name: "currentness"},
		{matches: row.predicateResult == "satisfied", name: "predicate"},
		{matches: row.admissionResult == "admitted", name: "admission result"},
	}
	if err := firstMismatch(checks, contract.generation+" authority resolution"); err != nil {
		return err
	}
	validators := []error{}
	_, err := profileauthority.NewProfileDeclarationAuthorityResolutionRef(row.ref)
	validators = append(validators, err)
	_, err = authority.NewDigest(row.digest)
	validators = append(validators, err)
	_, err = authority.NewDigest(row.basisDigest)
	validators = append(validators, err)
	_, err = authority.NewDigest(row.workInputDigest)
	validators = append(validators, err)
	_, err = authority.NewDigest(row.projectBindingDigest)
	validators = append(validators, err)
	_, err = authority.NewVerifierIdentity(row.verifierIdentity)
	validators = append(validators, err)
	_, err = authority.NewVerifierVersion(row.verifierVersion)
	validators = append(validators, err)
	_, err = authority.NewVerificationPolicyRef(row.verificationPolicyRef)
	validators = append(validators, err)
	_, err = authority.NewDigest(row.verificationPolicyDigest)
	validators = append(validators, err)
	_, err = parseV3Time(row.checkedAt)
	validators = append(validators, err)
	if contract.hostRouted {
		if row.operatorRequestRef == "" || row.operatorSubjectRef == "" {
			validators = append(validators, fmt.Errorf("host-routed resolution request identity is incomplete"))
		}
		_, err = authority.NewDigest(row.operatorRequestDigest)
		validators = append(validators, err)
		_, err = authority.NewDigest(row.operatorPayloadDigest)
		validators = append(validators, err)
	}
	return errors.Join(validators...)
}

func newV4AuthorityUseRecord(
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
		v4DirectProfileAuthorityContract,
	)
}

func newDirectProfileAuthorityUseRecord(
	useRef string,
	authorityValue authorityMaterial,
	requestDigest string,
	committedRef string,
	committedDigest string,
	consumedAt time.Time,
	contract directProfileAuthorityContract,
) (v4AuthorityUseRecord, error) {
	if _, err := profileauthority.NewProfileDeclarationAuthorityUseRef(useRef); err != nil {
		return v4AuthorityUseRecord{}, err
	}
	canonicalTime := formatTime(consumedAt)
	value := v4AuthorityUseJSON{
		Schema: contract.useSchema, UseRef: useRef,
		ProjectRoot:               authorityValue.projectRoot.String(),
		ActionKind:                authorityValue.actionKind.String(),
		AuthorityMode:             authorityValue.authorityMode,
		ResolutionKind:            authorityValue.resolutionKind,
		ProfileOrigin:             contract.origin,
		ProjectBindingDigest:      authorityValue.projectBindingHash.String(),
		AuthorityResolutionRef:    authorityValue.resolutionRef.String(),
		AuthorityResolutionDigest: authorityValue.resolutionDigest.String(),
		AuthorityBasisRef:         authorityValue.authorityBasisRef.String(),
		AuthorityBasisDigest:      authorityValue.authorityBasisDigest.String(),
		WorkInputRef:              authorityValue.workInputRef,
		WorkInputDigest:           authorityValue.workInputDigest,
		SingleUseKey:              authorityValue.singleUseKey.String(),
		AdmissionRequestDigest:    requestDigest,
		CommittedAdmissionRef:     committedRef,
		CommittedAdmissionDigest:  committedDigest,
		ConsumedAt:                canonicalTime,
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return v4AuthorityUseRecord{}, err
	}
	return v4AuthorityUseRecord{
		ref: useRef, digest: canonicalV3Digest(contract.useSchema, canonical),
		projectRoot: value.ProjectRoot, actionKind: value.ActionKind,
		mode: value.AuthorityMode, resolutionKind: value.ResolutionKind,
		origin:                    value.ProfileOrigin,
		projectBindingDigest:      value.ProjectBindingDigest,
		authorityResolutionRef:    value.AuthorityResolutionRef,
		authorityResolutionDigest: value.AuthorityResolutionDigest,
		authorityBasisRef:         value.AuthorityBasisRef,
		authorityBasisDigest:      value.AuthorityBasisDigest,
		workInputRef:              value.WorkInputRef, workInputDigest: value.WorkInputDigest,
		singleUseKey:             value.SingleUseKey,
		admissionRequestDigest:   value.AdmissionRequestDigest,
		committedAdmissionRef:    value.CommittedAdmissionRef,
		committedAdmissionDigest: value.CommittedAdmissionDigest,
		canonicalJSON:            string(canonical), consumedAt: canonicalTime,
	}, nil
}

func validateV4AuthorityUseRecord(record v4AuthorityUseRecord) error {
	return validateDirectProfileAuthorityUseRecord(
		record,
		v4DirectProfileAuthorityContract,
	)
}

func validateDirectProfileAuthorityUseRecord(
	record v4AuthorityUseRecord,
	contract directProfileAuthorityContract,
) error {
	value := v4AuthorityUseJSON{
		Schema: contract.useSchema, UseRef: record.ref, ProjectRoot: record.projectRoot,
		ActionKind: record.actionKind, AuthorityMode: record.mode,
		ResolutionKind: record.resolutionKind, ProfileOrigin: record.origin,
		ProjectBindingDigest:      record.projectBindingDigest,
		AuthorityResolutionRef:    record.authorityResolutionRef,
		AuthorityResolutionDigest: record.authorityResolutionDigest,
		AuthorityBasisRef:         record.authorityBasisRef,
		AuthorityBasisDigest:      record.authorityBasisDigest,
		WorkInputRef:              record.workInputRef, WorkInputDigest: record.workInputDigest,
		SingleUseKey:             record.singleUseKey,
		AdmissionRequestDigest:   record.admissionRequestDigest,
		CommittedAdmissionRef:    record.committedAdmissionRef,
		CommittedAdmissionDigest: record.committedAdmissionDigest,
		ConsumedAt:               record.consumedAt,
	}
	if err := requireCanonicalV3JSON(contract.useSchema, record.canonicalJSON, record.digest, value); err != nil {
		return fmt.Errorf("validate %s authority use: %w", contract.generation, err)
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: record.actionKind == contract.actionKind, name: "action kind"},
		{matches: record.mode == contract.mode, name: "authority mode"},
		{matches: record.resolutionKind == contract.resolutionKind, name: "resolution kind"},
		{matches: record.origin == contract.origin, name: "profile origin"},
	}
	if err := firstMismatch(checks, contract.generation+" authority use"); err != nil {
		return err
	}
	if _, err := profileauthority.NewProfileDeclarationAuthorityUseRef(record.ref); err != nil {
		return err
	}
	_, err := parseV3Time(record.consumedAt)
	return err
}

func loadV4AuthorityUseByAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admissionRef string,
) (v4AuthorityUseRecord, error) {
	record := v4AuthorityUseRecord{}
	err := transaction.ScanOne(
		ctx,
		selectV4UseByAdmissionSQL,
		[]any{admissionRef},
		record.scanTargets(),
	)
	if err != nil {
		return v4AuthorityUseRecord{}, fmt.Errorf("load v4 automatic authority use: %w", err)
	}
	if err := validateV4AuthorityUseRecord(record); err != nil {
		return v4AuthorityUseRecord{}, err
	}
	return record, nil
}

func validateV4HistoricalMaterialInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	material canonicalAdmissionMaterial,
) error {
	use, err := loadV4AuthorityUseByAdmission(
		ctx,
		transaction,
		material.admissionRef.String(),
	)
	if err != nil {
		return err
	}
	closure, err := loadV4AuthorityClosure(
		ctx,
		transaction,
		material.authorityBasisRef.String(),
	)
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: use.projectRoot == material.projectRoot.String(), name: "use project root"},
		{matches: use.authorityResolutionRef == material.authorityResolutionRef.String(), name: "use resolution ref"},
		{matches: use.authorityResolutionDigest == material.authorityResolutionDigest.String(), name: "use resolution digest"},
		{matches: use.authorityBasisRef == material.authorityBasisRef.String(), name: "use basis ref"},
		{matches: use.authorityBasisDigest == material.authorityBasisDigest.String(), name: "use basis digest"},
		{matches: use.committedAdmissionRef == material.admissionRef.String(), name: "use admission ref"},
		{matches: use.committedAdmissionDigest == material.admissionDigest.String(), name: "use admission digest"},
		{matches: use.consumedAt == formatTime(material.recordedAt), name: "use consumption time"},
		{matches: use.authorityBasisRef == closure.basis.ref, name: "closure basis ref"},
		{matches: use.authorityBasisDigest == closure.basis.digest, name: "closure basis digest"},
		{matches: use.authorityResolutionRef == closure.resolution.ref, name: "closure resolution ref"},
		{matches: use.authorityResolutionDigest == closure.resolution.digest, name: "closure resolution digest"},
		{matches: use.workInputRef == closure.workInput.ref, name: "closure WorkInput ref"},
		{matches: use.workInputDigest == closure.workInput.digest, name: "closure WorkInput digest"},
		{matches: use.origin == v4ProfileOrigin, name: "profile origin"},
	}
	return firstMismatch(checks, "historical v4 automatic profile authority")
}

func materializeV4Authority(
	closure v4AuthorityClosure,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	admissionTime time.Time,
) (authorityMaterial, error) {
	return materializeDirectProfileAuthority(
		closure,
		values,
		admissionTime,
		v4DirectProfileAuthorityContract,
	)
}

func materializeDirectProfileAuthority(
	closure v4AuthorityClosure,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	admissionTime time.Time,
	contract directProfileAuthorityContract,
) (authorityMaterial, error) {
	basis := closure.basis
	resolution := closure.resolution
	work := values.WorkRecord()
	if err := validateDirectProfileClosureAgainstCandidateSupport(
		closure,
		values,
		admissionTime,
		contract,
	); err != nil {
		return authorityMaterial{}, err
	}
	projectRoot, err := authority.NewProjectRoot(basis.projectRoot)
	if err != nil {
		return authorityMaterial{}, err
	}
	actionKind, err := authority.NewActionKind(basis.actionKind)
	if err != nil {
		return authorityMaterial{}, err
	}
	profileAuthorRef, err := authority.NewRoleAssignmentRef(basis.profileAuthorRef)
	if err != nil {
		return authorityMaterial{}, err
	}
	profileAuthorDigest, err := authority.NewDigest(basis.profileAuthorDigest)
	if err != nil {
		return authorityMaterial{}, err
	}
	methodDescriptionRef, err := authority.NewMethodDescriptionRef(basis.methodDescriptionRef)
	if err != nil {
		return authorityMaterial{}, err
	}
	methodDescriptionDigest, err := authority.NewDigest(basis.methodDescriptionDigest)
	if err != nil {
		return authorityMaterial{}, err
	}
	methodContractRef, err := authority.NewMethodContractRef(basis.methodContractRef)
	if err != nil {
		return authorityMaterial{}, err
	}
	methodContractDigest, err := authority.NewDigest(basis.methodContractDigest)
	if err != nil {
		return authorityMaterial{}, err
	}
	classifierVersion, err := authority.NewClassifierVersion(basis.classifierVersion)
	if err != nil {
		return authorityMaterial{}, err
	}
	policyVersion, err := authority.NewPolicyVersion(basis.policyVersion)
	if err != nil {
		return authorityMaterial{}, err
	}
	sessionRef, err := authority.NewSessionRef(basis.futureWorkSessionRef)
	if err != nil {
		return authorityMaterial{}, err
	}
	allowedWork, err := parseV3Window(basis.allowedWorkFrom, basis.allowedWorkUntil)
	if err != nil {
		return authorityMaterial{}, err
	}
	allowedBasis, err := parseV3Window(basis.basisObservationFrom, basis.basisObservationUntil)
	if err != nil {
		return authorityMaterial{}, err
	}
	singleUseKey, err := authority.NewSingleUseKey(basis.singleUseKey)
	if err != nil {
		return authorityMaterial{}, err
	}
	resolutionRef, err := profileauthority.NewProfileDeclarationAuthorityResolutionRef(resolution.ref)
	if err != nil {
		return authorityMaterial{}, err
	}
	resolutionDigest, err := authority.NewDigest(resolution.digest)
	if err != nil {
		return authorityMaterial{}, err
	}
	basisRef, err := profileauthority.NewBasisRef(basis.ref)
	if err != nil {
		return authorityMaterial{}, err
	}
	basisDigest, err := authority.NewDigest(basis.digest)
	if err != nil {
		return authorityMaterial{}, err
	}
	projectBindingDigest, err := authority.NewDigest(resolution.projectBindingDigest)
	if err != nil {
		return authorityMaterial{}, err
	}
	checkedAt, err := parseV3Time(resolution.checkedAt)
	if err != nil {
		return authorityMaterial{}, err
	}
	verifierIdentity, err := authority.NewVerifierIdentity(resolution.verifierIdentity)
	if err != nil {
		return authorityMaterial{}, err
	}
	verifierVersion, err := authority.NewVerifierVersion(resolution.verifierVersion)
	if err != nil {
		return authorityMaterial{}, err
	}
	canonicalAdmissionTime := admissionTime.UTC().Round(0)
	if canonicalAdmissionTime.Before(work.WorkInterval().Until()) {
		return authorityMaterial{}, fmt.Errorf("%s authority cannot be consumed before Work ended", contract.generation)
	}
	if !allowedWork.Contains(canonicalAdmissionTime) {
		return authorityMaterial{}, fmt.Errorf("%s authority is not current at admission time", contract.generation)
	}
	return authorityMaterial{
		resolutionRef: resolutionRef, resolutionDigest: resolutionDigest,
		authorityBasisRef: basisRef, authorityBasisDigest: basisDigest,
		projectRoot: projectRoot, actionKind: actionKind,
		projectBindingHash: projectBindingDigest,
		profileAuthorRef:   profileAuthorRef, profileAuthorDigest: profileAuthorDigest,
		methodDescriptionRef:    methodDescriptionRef,
		methodDescriptionDigest: methodDescriptionDigest,
		methodContractRef:       methodContractRef,
		methodContractDigest:    methodContractDigest,
		classifierVersion:       classifierVersion, policyVersion: policyVersion,
		futureWorkSession: sessionRef, allowedWork: allowedWork,
		allowedBasisObservation: allowedBasis, permissionValidity: allowedWork,
		singleUseKey: singleUseKey, verifierIdentity: verifierIdentity,
		verifierVersion: verifierVersion, checkedAt: checkedAt,
		judgementTime: canonicalAdmissionTime, authorityMode: basis.mode,
		resolutionKind: resolution.resolutionKind,
		workInputRef:   basis.workInputRef, workInputDigest: basis.workInputDigest,
		permissionRequired: false,
	}, nil
}

func validateV4ClosureAgainstCandidateSupport(
	closure v4AuthorityClosure,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	admissionTime time.Time,
) error {
	return validateDirectProfileClosureAgainstCandidateSupport(
		closure,
		values,
		admissionTime,
		v4DirectProfileAuthorityContract,
	)
}

func validateDirectProfileClosureAgainstCandidateSupport(
	closure v4AuthorityClosure,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	admissionTime time.Time,
	contract directProfileAuthorityContract,
) error {
	work := values.WorkRecord()
	candidateOutcome, ok := work.Outcome().(projectprofile.CandidatePayloadProduced)
	if !ok {
		return fmt.Errorf("%s authority requires CandidatePayloadProduced Work outcome", contract.generation)
	}
	inputFound := false
	for _, ref := range work.InputRefs() {
		inputFound = inputFound || ref.String() == closure.basis.workInputRef
	}
	checkedAt, err := parseV3Time(closure.resolution.checkedAt)
	if err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: inputFound, name: "WorkInput occurrence"},
		{matches: closure.workInput.payloadDigest == candidateOutcome.PayloadDigest().String(), name: "WorkInput payload digest"},
		{matches: !work.WorkInterval().From().Before(checkedAt), name: "Work start after resolution"},
		{matches: !admissionTime.UTC().Before(work.WorkInterval().Until()), name: "admission after Work end"},
	}
	return firstMismatch(checks, contract.generation+" profile authority support")
}
