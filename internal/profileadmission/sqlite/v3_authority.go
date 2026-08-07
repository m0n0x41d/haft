package sqlite

// This file is the read/verification codec for the sealed v3 profile-authority
// generation. Current operator-mediated profile writes use v5 host-routed
// authority; migration 56 prevents any new v3 authority row.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/profileauthority"
	profileauthoritysqlite "github.com/m0n0x41d/haft/internal/profileauthority/sqlite"
	"github.com/m0n0x41d/haft/internal/profiledeclarationpreparation"
	"github.com/m0n0x41d/haft/internal/projectprofile"
	projectprofilesqlite "github.com/m0n0x41d/haft/internal/projectprofile/sqlite"
	"github.com/m0n0x41d/haft/internal/sqlitetransaction"
)

const (
	v3ExplicitAuthorityMode = "explicit_h_onboard"
	v3StrictAuthorityMode   = "strict_cli_speech_act"

	v3ExplicitResolutionKind = "explicit_policy_acceptance"
	v3StrictResolutionKind   = "strict_permission"

	v3WorkInputSchema  = "haft.profile-onboarding.work-input/v1"
	v3BasisSchema      = "haft.profile-authority.authority-basis/v3"
	v3ResolutionSchema = "haft.profile-authority.authority-resolution/v3"
	v3UseSchema        = "haft.profile-authority.authority-use/v3"
	v3ActionKind       = "profile.declare.from_onboarding_candidate"
)

const selectV3BasisDiscoverySQL = `SELECT
	authority_mode,
	COALESCE(strict_authority_basis_ref, ''),
	COALESCE(strict_authority_basis_digest, '')
FROM profile_declaration_authority_bases_v3
WHERE basis_ref = ?`

const selectV3BasisSQL = `SELECT
	basis_ref, basis_digest, project_root, action_kind, authority_mode,
	work_input_ref, work_input_digest,
	profile_author_role_assignment_ref, profile_author_role_assignment_digest,
	method_description_ref, method_description_digest,
	method_contract_ref, method_contract_digest,
	classifier_version, policy_version, future_work_session_ref,
	allowed_work_from, allowed_work_until,
	basis_observation_from, basis_observation_until,
	single_use_key,
	COALESCE(config_carrier_ref, ''), COALESCE(config_carrier_digest, ''),
	COALESCE(strict_authority_basis_ref, ''), COALESCE(strict_authority_basis_digest, ''),
	canonical_json, recorded_at
FROM profile_declaration_authority_bases_v3
WHERE basis_ref = ?`

const selectV3ResolutionByBasisSQL = `SELECT
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	project_root, action_kind, authority_mode, resolution_kind,
	work_input_ref, work_input_digest, project_binding_digest,
	COALESCE(strict_permission_ref, ''), COALESCE(strict_permission_digest, ''),
	verifier_identity, verifier_version,
	verification_policy_ref, verification_policy_digest,
	checked_at, currentness_result, predicate_result, admission_result,
	canonical_json, recorded_at
FROM profile_declaration_authority_resolutions_v3
WHERE authority_basis_ref = ?`

const selectV3WorkInputSQL = `SELECT
	work_input_ref, work_input_digest, project_root,
	suggestion_ref, detector_version, policy_version, observation_digest,
	profile_payload_json, profile_payload_digest, canonical_json, recorded_at
FROM profile_onboarding_work_inputs_v1
WHERE work_input_ref = ?`

const selectV3UseByAdmissionSQL = `SELECT
	use_ref, use_digest, project_root, action_kind, authority_mode, resolution_kind,
	project_binding_digest,
	authority_resolution_ref, authority_resolution_digest,
	authority_basis_ref, authority_basis_digest,
	work_input_ref, work_input_digest, single_use_key,
	admission_request_digest, committed_admission_ref, committed_admission_digest,
	canonical_json, consumed_at
FROM profile_declaration_authority_uses_v3
WHERE committed_admission_ref = ?`

type v3BasisDiscovery struct {
	mode              string
	strictBasisRef    string
	strictBasisDigest string
}

func (value v3BasisDiscovery) strict() bool {
	return value.mode == v3StrictAuthorityMode
}

type v3AuthorityBasisRow struct {
	ref                        string
	digest                     string
	projectRoot                string
	actionKind                 string
	mode                       string
	workInputRef               string
	workInputDigest            string
	profileAuthorRef           string
	profileAuthorDigest        string
	methodDescriptionRef       string
	methodDescriptionDigest    string
	methodContractRef          string
	methodContractDigest       string
	classifierVersion          string
	policyVersion              string
	futureWorkSessionRef       string
	allowedWorkFrom            string
	allowedWorkUntil           string
	basisObservationFrom       string
	basisObservationUntil      string
	singleUseKey               string
	configCarrierRef           string
	configCarrierDigest        string
	strictAuthorityBasisRef    string
	strictAuthorityBasisDigest string
	canonicalJSON              string
	recordedAt                 string
}

func (row *v3AuthorityBasisRow) scanTargets() []any {
	return []any{
		&row.ref, &row.digest, &row.projectRoot, &row.actionKind, &row.mode,
		&row.workInputRef, &row.workInputDigest,
		&row.profileAuthorRef, &row.profileAuthorDigest,
		&row.methodDescriptionRef, &row.methodDescriptionDigest,
		&row.methodContractRef, &row.methodContractDigest,
		&row.classifierVersion, &row.policyVersion, &row.futureWorkSessionRef,
		&row.allowedWorkFrom, &row.allowedWorkUntil,
		&row.basisObservationFrom, &row.basisObservationUntil,
		&row.singleUseKey,
		&row.configCarrierRef, &row.configCarrierDigest,
		&row.strictAuthorityBasisRef, &row.strictAuthorityBasisDigest,
		&row.canonicalJSON, &row.recordedAt,
	}
}

type v3AuthorityResolutionRow struct {
	ref                      string
	digest                   string
	basisRef                 string
	basisDigest              string
	projectRoot              string
	actionKind               string
	mode                     string
	resolutionKind           string
	workInputRef             string
	workInputDigest          string
	projectBindingDigest     string
	strictPermissionRef      string
	strictPermissionDigest   string
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

func (row *v3AuthorityResolutionRow) scanTargets() []any {
	return []any{
		&row.ref, &row.digest, &row.basisRef, &row.basisDigest,
		&row.projectRoot, &row.actionKind, &row.mode, &row.resolutionKind,
		&row.workInputRef, &row.workInputDigest, &row.projectBindingDigest,
		&row.strictPermissionRef, &row.strictPermissionDigest,
		&row.verifierIdentity, &row.verifierVersion,
		&row.verificationPolicyRef, &row.verificationPolicyDigest,
		&row.checkedAt, &row.currentnessResult, &row.predicateResult,
		&row.admissionResult, &row.canonicalJSON, &row.recordedAt,
	}
}

type v3WorkInputRow struct {
	ref               string
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

func (row *v3WorkInputRow) scanTargets() []any {
	return []any{
		&row.ref, &row.digest, &row.projectRoot,
		&row.suggestionRef, &row.detectorVersion, &row.policyVersion,
		&row.observationDigest, &row.payloadJSON, &row.payloadDigest,
		&row.canonicalJSON, &row.recordedAt,
	}
}

type v3AuthorityClosure struct {
	basis      v3AuthorityBasisRow
	resolution v3AuthorityResolutionRow
	workInput  v3WorkInputRow
}

type v3AuthorityBasisJSON struct {
	Schema                      string `json:"schema"`
	BasisRef                    string `json:"basis_ref"`
	ProjectRoot                 string `json:"project_root"`
	ActionKind                  string `json:"action_kind"`
	AuthorityMode               string `json:"authority_mode"`
	WorkInputRef                string `json:"work_input_ref"`
	WorkInputDigest             string `json:"work_input_digest"`
	ProfileAuthorAssignmentRef  string `json:"profile_author_role_assignment_ref"`
	ProfileAuthorAssignmentHash string `json:"profile_author_role_assignment_digest"`
	MethodDescriptionRef        string `json:"method_description_ref"`
	MethodDescriptionDigest     string `json:"method_description_digest"`
	MethodContractRef           string `json:"method_contract_ref"`
	MethodContractDigest        string `json:"method_contract_digest"`
	ClassifierVersion           string `json:"classifier_version"`
	PolicyVersion               string `json:"policy_version"`
	FutureWorkSessionRef        string `json:"future_work_session_ref"`
	AllowedWorkFrom             string `json:"allowed_work_from"`
	AllowedWorkUntil            string `json:"allowed_work_until"`
	BasisObservationFrom        string `json:"basis_observation_from"`
	BasisObservationUntil       string `json:"basis_observation_until"`
	SingleUseKey                string `json:"single_use_key"`
	ConfigCarrierRef            string `json:"config_carrier_ref,omitempty"`
	ConfigCarrierDigest         string `json:"config_carrier_digest,omitempty"`
	StrictAuthorityBasisRef     string `json:"strict_authority_basis_ref,omitempty"`
	StrictAuthorityBasisDigest  string `json:"strict_authority_basis_digest,omitempty"`
}

type v3AuthorityResolutionJSON struct {
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
	StrictPermissionRef      string `json:"strict_permission_ref,omitempty"`
	StrictPermissionDigest   string `json:"strict_permission_digest,omitempty"`
	VerifierIdentity         string `json:"verifier_identity"`
	VerifierVersion          string `json:"verifier_version"`
	VerificationPolicyRef    string `json:"verification_policy_ref"`
	VerificationPolicyDigest string `json:"verification_policy_digest"`
	CheckedAt                string `json:"checked_at"`
	CurrentnessResult        string `json:"currentness_result"`
	PredicateResult          string `json:"predicate_result"`
	AdmissionResult          string `json:"admission_result"`
}

type v3WorkInputJSON struct {
	Schema                     string                    `json:"schema"`
	ProjectRoot                string                    `json:"project_root"`
	SuggestionRef              string                    `json:"suggestion_ref"`
	DetectorVersion            string                    `json:"detector_version"`
	PolicyVersion              string                    `json:"policy_version"`
	ObservationDetectorVersion string                    `json:"observation_detector_version,omitempty"`
	ObservationPolicyVersion   string                    `json:"observation_policy_version,omitempty"`
	ObservationDigest          string                    `json:"observation_digest"`
	ProposalSource             string                    `json:"proposal_source,omitempty"`
	ManualBasis                string                    `json:"manual_basis,omitempty"`
	ChangeBasis                *v3ProfileChangeBasisJSON `json:"change_basis,omitempty"`
	Scopes                     []v3WorkInputScopeJSON    `json:"scopes"`
}

type v3ProfileChangeBasisJSON struct {
	AdmissionRecordRef    string `json:"admission_record_ref"`
	AdmissionRecordDigest string `json:"admission_record_digest"`
	PayloadDigest         string `json:"payload_digest"`
	LedgerRevision        uint64 `json:"ledger_revision"`
	ScopeID               string `json:"scope_id"`
	PreviousEntityRef     string `json:"previous_entity_ref,omitempty"`
	NextEntityRef         string `json:"next_entity_ref"`
}

type v3WorkInputScopeJSON struct {
	ComponentCandidateRef string   `json:"component_candidate_ref"`
	ScopeID               string   `json:"scope_id"`
	Label                 string   `json:"label,omitempty"`
	RealizationKind       string   `json:"realization_kind"`
	EntityRef             string   `json:"entity_ref,omitempty"`
	AdmittedKindRef       string   `json:"admitted_kind_ref,omitempty"`
	GoverningPatternRefs  []string `json:"governing_pattern_refs,omitempty"`
	ContractRefs          []string `json:"contract_refs,omitempty"`
	EvidencePaths         []string `json:"evidence_paths,omitempty"`
}

type v3AuthorityUseJSON struct {
	Schema                    string `json:"schema"`
	UseRef                    string `json:"use_ref"`
	ProjectRoot               string `json:"project_root"`
	ActionKind                string `json:"action_kind"`
	AuthorityMode             string `json:"authority_mode"`
	ResolutionKind            string `json:"resolution_kind"`
	ProjectBindingDigest      string `json:"project_binding_digest"`
	AuthorityResolutionRef    string `json:"authority_resolution_ref"`
	AuthorityResolutionDigest string `json:"authority_resolution_digest"`
	AuthorityBasisRef         string `json:"authority_basis_ref"`
	AuthorityBasisDigest      string `json:"authority_basis_digest"`
	WorkInputRef              string `json:"work_input_ref"`
	WorkInputDigest           string `json:"work_input_digest"`
	SingleUseKey              string `json:"single_use_key"`
	AdmissionRequestDigest    string `json:"admission_request_digest"`
	CommittedAdmissionRef     string `json:"committed_admission_ref"`
	CommittedAdmissionDigest  string `json:"committed_admission_digest"`
	ConsumedAt                string `json:"consumed_at"`
}

type v3AuthorityUseRecord struct {
	ref                       string
	digest                    string
	projectRoot               string
	actionKind                string
	mode                      string
	resolutionKind            string
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

func (record *v3AuthorityUseRecord) scanTargets() []any {
	return []any{
		&record.ref, &record.digest, &record.projectRoot, &record.actionKind,
		&record.mode, &record.resolutionKind, &record.projectBindingDigest,
		&record.authorityResolutionRef, &record.authorityResolutionDigest,
		&record.authorityBasisRef, &record.authorityBasisDigest,
		&record.workInputRef, &record.workInputDigest, &record.singleUseKey,
		&record.admissionRequestDigest, &record.committedAdmissionRef,
		&record.committedAdmissionDigest, &record.canonicalJSON,
		&record.consumedAt,
	}
}

func loadV3AuthorityUseByAdmission(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	admissionRef string,
) (v3AuthorityUseRecord, error) {
	record := v3AuthorityUseRecord{}
	err := transaction.ScanOne(
		ctx,
		selectV3UseByAdmissionSQL,
		[]any{admissionRef},
		record.scanTargets(),
	)
	if err != nil {
		return v3AuthorityUseRecord{}, fmt.Errorf("load v3 profile authority use: %w", err)
	}
	if err := validateV3AuthorityUseRecord(record); err != nil {
		return v3AuthorityUseRecord{}, err
	}
	return record, nil
}

func validateV3AuthorityUseRecord(record v3AuthorityUseRecord) error {
	value := v3AuthorityUseJSON{
		Schema:                    v3UseSchema,
		UseRef:                    record.ref,
		ProjectRoot:               record.projectRoot,
		ActionKind:                record.actionKind,
		AuthorityMode:             record.mode,
		ResolutionKind:            record.resolutionKind,
		ProjectBindingDigest:      record.projectBindingDigest,
		AuthorityResolutionRef:    record.authorityResolutionRef,
		AuthorityResolutionDigest: record.authorityResolutionDigest,
		AuthorityBasisRef:         record.authorityBasisRef,
		AuthorityBasisDigest:      record.authorityBasisDigest,
		WorkInputRef:              record.workInputRef,
		WorkInputDigest:           record.workInputDigest,
		SingleUseKey:              record.singleUseKey,
		AdmissionRequestDigest:    record.admissionRequestDigest,
		CommittedAdmissionRef:     record.committedAdmissionRef,
		CommittedAdmissionDigest:  record.committedAdmissionDigest,
		ConsumedAt:                record.consumedAt,
	}
	if err := requireCanonicalV3JSON(
		v3UseSchema,
		record.canonicalJSON,
		record.digest,
		value,
	); err != nil {
		return fmt.Errorf("validate v3 profile authority use: %w", err)
	}
	if _, err := profileauthority.NewProfileDeclarationAuthorityUseRef(record.ref); err != nil {
		return err
	}
	if _, err := parseV3Time(record.consumedAt); err != nil {
		return err
	}
	explicit := record.mode == v3ExplicitAuthorityMode &&
		record.resolutionKind == v3ExplicitResolutionKind
	strict := record.mode == v3StrictAuthorityMode &&
		record.resolutionKind == v3StrictResolutionKind
	if !explicit && !strict {
		return fmt.Errorf("v3 profile authority use has an invalid closed authority branch")
	}
	return nil
}

func validateV3HistoricalMaterialInTransaction(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	material canonicalAdmissionMaterial,
) error {
	use, err := loadV3AuthorityUseByAdmission(
		ctx,
		transaction,
		material.admissionRef.String(),
	)
	if err != nil {
		return err
	}
	closure, err := loadV3AuthorityClosure(
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
		{matches: use.mode == closure.basis.mode, name: "closure authority mode"},
		{matches: use.resolutionKind == closure.resolution.resolutionKind, name: "closure resolution kind"},
		{matches: use.projectBindingDigest == closure.resolution.projectBindingDigest, name: "closure project binding"},
	}
	return firstMismatch(checks, "historical v3 profile authority")
}

func newV3AuthorityUseRecord(
	useRef string,
	authorityValue authorityMaterial,
	requestDigest string,
	committedRef string,
	committedDigest string,
	consumedAt time.Time,
) (v3AuthorityUseRecord, error) {
	if _, err := profileauthority.NewProfileDeclarationAuthorityUseRef(useRef); err != nil {
		return v3AuthorityUseRecord{}, err
	}
	canonicalTime := consumedAt.UTC().Round(0).Format(time.RFC3339Nano)
	value := v3AuthorityUseJSON{
		Schema:                    v3UseSchema,
		UseRef:                    useRef,
		ProjectRoot:               authorityValue.projectRoot.String(),
		ActionKind:                authorityValue.actionKind.String(),
		AuthorityMode:             authorityValue.authorityMode,
		ResolutionKind:            authorityValue.resolutionKind,
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
		return v3AuthorityUseRecord{}, err
	}
	return v3AuthorityUseRecord{
		ref:                       useRef,
		digest:                    canonicalV3Digest(v3UseSchema, canonical),
		projectRoot:               value.ProjectRoot,
		actionKind:                value.ActionKind,
		mode:                      value.AuthorityMode,
		resolutionKind:            value.ResolutionKind,
		projectBindingDigest:      value.ProjectBindingDigest,
		authorityResolutionRef:    value.AuthorityResolutionRef,
		authorityResolutionDigest: value.AuthorityResolutionDigest,
		authorityBasisRef:         value.AuthorityBasisRef,
		authorityBasisDigest:      value.AuthorityBasisDigest,
		workInputRef:              value.WorkInputRef,
		workInputDigest:           value.WorkInputDigest,
		singleUseKey:              value.SingleUseKey,
		admissionRequestDigest:    value.AdmissionRequestDigest,
		committedAdmissionRef:     value.CommittedAdmissionRef,
		committedAdmissionDigest:  value.CommittedAdmissionDigest,
		canonicalJSON:             string(canonical),
		consumedAt:                canonicalTime,
	}, nil
}

func discoverV3Basis(
	ctx context.Context,
	database *sql.DB,
	basisRef string,
) (v3BasisDiscovery, bool, error) {
	value := v3BasisDiscovery{}
	err := database.QueryRowContext(
		ctx,
		selectV3BasisDiscoverySQL,
		basisRef,
	).Scan(&value.mode, &value.strictBasisRef, &value.strictBasisDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return v3BasisDiscovery{}, false, nil
	}
	if err != nil {
		return v3BasisDiscovery{}, false, fmt.Errorf("discover v3 profile authority basis: %w", err)
	}
	if value.mode != v3ExplicitAuthorityMode && value.mode != v3StrictAuthorityMode {
		return v3BasisDiscovery{}, false, fmt.Errorf("v3 profile authority basis has unknown mode %q", value.mode)
	}
	return value, true, nil
}

func loadV3AuthorityClosure(
	ctx context.Context,
	transaction *sqlitetransaction.Transaction,
	basisRef string,
) (v3AuthorityClosure, error) {
	basis := v3AuthorityBasisRow{}
	err := transaction.ScanOne(
		ctx,
		selectV3BasisSQL,
		[]any{basisRef},
		basis.scanTargets(),
	)
	if err != nil {
		return v3AuthorityClosure{}, fmt.Errorf("load v3 profile authority basis: %w", err)
	}
	resolution := v3AuthorityResolutionRow{}
	err = transaction.ScanOne(
		ctx,
		selectV3ResolutionByBasisSQL,
		[]any{basisRef},
		resolution.scanTargets(),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return v3AuthorityClosure{}, profileauthoritysqlite.ErrAuthorityResolutionRequired
	}
	if err != nil {
		return v3AuthorityClosure{}, fmt.Errorf("load v3 profile authority resolution: %w", err)
	}
	workInput := v3WorkInputRow{}
	err = transaction.ScanOne(
		ctx,
		selectV3WorkInputSQL,
		[]any{basis.workInputRef},
		workInput.scanTargets(),
	)
	if err != nil {
		return v3AuthorityClosure{}, fmt.Errorf("load v3 profile WorkInput: %w", err)
	}
	closure := v3AuthorityClosure{
		basis:      basis,
		resolution: resolution,
		workInput:  workInput,
	}
	if err := validateV3AuthorityClosure(closure); err != nil {
		return v3AuthorityClosure{}, err
	}
	return closure, nil
}

func validateV3AuthorityClosure(closure v3AuthorityClosure) error {
	basis := closure.basis
	resolution := closure.resolution
	workInput := closure.workInput
	if err := validateV3AuthorityBasis(basis); err != nil {
		return err
	}
	if err := validateV3AuthorityResolution(resolution); err != nil {
		return err
	}
	if err := validateV3WorkInput(workInput); err != nil {
		return err
	}
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
		{matches: resolution.workInputRef == basis.workInputRef, name: "resolution WorkInput ref"},
		{matches: resolution.workInputDigest == basis.workInputDigest, name: "resolution WorkInput digest"},
		{matches: workInput.ref == basis.workInputRef, name: "basis WorkInput ref"},
		{matches: workInput.digest == basis.workInputDigest, name: "basis WorkInput digest"},
		{matches: workInput.projectRoot == basis.projectRoot, name: "WorkInput project root"},
		{matches: workInput.detectorVersion == basis.classifierVersion, name: "WorkInput classifier version"},
		{matches: workInput.policyVersion == basis.policyVersion, name: "WorkInput policy version"},
		{matches: !basisRecordedAt.Before(workInputRecordedAt), name: "WorkInput recorded before authority basis"},
		{matches: allowedWork.Contains(checkedAt), name: "resolution checked inside allowed Work window"},
	}
	return firstMismatch(checks, "v3 profile authority closure")
}

func validateV3AuthorityBasis(row v3AuthorityBasisRow) error {
	value := v3AuthorityBasisJSON{
		Schema:                      v3BasisSchema,
		BasisRef:                    row.ref,
		ProjectRoot:                 row.projectRoot,
		ActionKind:                  row.actionKind,
		AuthorityMode:               row.mode,
		WorkInputRef:                row.workInputRef,
		WorkInputDigest:             row.workInputDigest,
		ProfileAuthorAssignmentRef:  row.profileAuthorRef,
		ProfileAuthorAssignmentHash: row.profileAuthorDigest,
		MethodDescriptionRef:        row.methodDescriptionRef,
		MethodDescriptionDigest:     row.methodDescriptionDigest,
		MethodContractRef:           row.methodContractRef,
		MethodContractDigest:        row.methodContractDigest,
		ClassifierVersion:           row.classifierVersion,
		PolicyVersion:               row.policyVersion,
		FutureWorkSessionRef:        row.futureWorkSessionRef,
		AllowedWorkFrom:             row.allowedWorkFrom,
		AllowedWorkUntil:            row.allowedWorkUntil,
		BasisObservationFrom:        row.basisObservationFrom,
		BasisObservationUntil:       row.basisObservationUntil,
		SingleUseKey:                row.singleUseKey,
		ConfigCarrierRef:            row.configCarrierRef,
		ConfigCarrierDigest:         row.configCarrierDigest,
		StrictAuthorityBasisRef:     row.strictAuthorityBasisRef,
		StrictAuthorityBasisDigest:  row.strictAuthorityBasisDigest,
	}
	if err := requireCanonicalV3JSON(v3BasisSchema, row.canonicalJSON, row.digest, value); err != nil {
		return fmt.Errorf("validate v3 profile authority basis: %w", err)
	}
	if _, err := profileauthority.NewBasisRef(row.ref); err != nil {
		return err
	}
	if _, err := authority.NewDigest(row.digest); err != nil {
		return err
	}
	if row.actionKind != v3ActionKind {
		return fmt.Errorf("v3 profile authority basis has an invalid action kind")
	}
	if _, err := authority.NewProjectRoot(row.projectRoot); err != nil {
		return err
	}
	if _, err := projectprofile.NewWorkInputRef(row.workInputRef); err != nil {
		return err
	}
	if _, err := authority.NewDigest(row.workInputDigest); err != nil {
		return err
	}
	if _, err := authority.NewRoleAssignmentRef(row.profileAuthorRef); err != nil {
		return err
	}
	if _, err := authority.NewDigest(row.profileAuthorDigest); err != nil {
		return err
	}
	if _, err := authority.NewMethodDescriptionRef(row.methodDescriptionRef); err != nil {
		return err
	}
	if _, err := authority.NewDigest(row.methodDescriptionDigest); err != nil {
		return err
	}
	if _, err := authority.NewMethodContractRef(row.methodContractRef); err != nil {
		return err
	}
	if _, err := authority.NewDigest(row.methodContractDigest); err != nil {
		return err
	}
	if _, err := authority.NewClassifierVersion(row.classifierVersion); err != nil {
		return err
	}
	if _, err := authority.NewPolicyVersion(row.policyVersion); err != nil {
		return err
	}
	if _, err := authority.NewSessionRef(row.futureWorkSessionRef); err != nil {
		return err
	}
	if _, err := authority.NewSingleUseKey(row.singleUseKey); err != nil {
		return err
	}
	if _, err := parseV3Window(row.allowedWorkFrom, row.allowedWorkUntil); err != nil {
		return err
	}
	if _, err := parseV3Window(row.basisObservationFrom, row.basisObservationUntil); err != nil {
		return err
	}
	if _, err := parseV3Time(row.recordedAt); err != nil {
		return err
	}
	branchValid := row.mode == v3ExplicitAuthorityMode &&
		row.configCarrierRef != "" && row.configCarrierDigest != "" &&
		row.strictAuthorityBasisRef == "" && row.strictAuthorityBasisDigest == ""
	branchValid = branchValid || row.mode == v3StrictAuthorityMode &&
		row.configCarrierRef == "" && row.configCarrierDigest == "" &&
		row.strictAuthorityBasisRef != "" && row.strictAuthorityBasisDigest != ""
	if !branchValid {
		return fmt.Errorf("v3 profile authority basis has an invalid closed authority branch")
	}
	if row.mode == v3ExplicitAuthorityMode {
		if _, err := authority.NewDigest(row.configCarrierDigest); err != nil {
			return err
		}
	}
	if row.mode == v3StrictAuthorityMode {
		if _, err := profileauthority.NewBasisRef(row.strictAuthorityBasisRef); err != nil {
			return err
		}
		if _, err := authority.NewDigest(row.strictAuthorityBasisDigest); err != nil {
			return err
		}
	}
	return nil
}

func validateV3AuthorityResolution(row v3AuthorityResolutionRow) error {
	value := v3AuthorityResolutionJSON{
		Schema:                   v3ResolutionSchema,
		AuthorityResolutionRef:   row.ref,
		AuthorityBasisRef:        row.basisRef,
		AuthorityBasisDigest:     row.basisDigest,
		ProjectRoot:              row.projectRoot,
		ActionKind:               row.actionKind,
		AuthorityMode:            row.mode,
		ResolutionKind:           row.resolutionKind,
		WorkInputRef:             row.workInputRef,
		WorkInputDigest:          row.workInputDigest,
		ProjectBindingDigest:     row.projectBindingDigest,
		StrictPermissionRef:      row.strictPermissionRef,
		StrictPermissionDigest:   row.strictPermissionDigest,
		VerifierIdentity:         row.verifierIdentity,
		VerifierVersion:          row.verifierVersion,
		VerificationPolicyRef:    row.verificationPolicyRef,
		VerificationPolicyDigest: row.verificationPolicyDigest,
		CheckedAt:                row.checkedAt,
		CurrentnessResult:        row.currentnessResult,
		PredicateResult:          row.predicateResult,
		AdmissionResult:          row.admissionResult,
	}
	if err := requireCanonicalV3JSON(v3ResolutionSchema, row.canonicalJSON, row.digest, value); err != nil {
		return fmt.Errorf("validate v3 profile authority resolution: %w", err)
	}
	if _, err := profileauthority.NewProfileDeclarationAuthorityResolutionRef(row.ref); err != nil {
		return err
	}
	if row.actionKind != v3ActionKind {
		return fmt.Errorf("v3 profile authority resolution has an invalid action kind")
	}
	if _, err := authority.NewDigest(row.digest); err != nil {
		return err
	}
	if _, err := authority.NewDigest(row.basisDigest); err != nil {
		return err
	}
	if _, err := authority.NewDigest(row.workInputDigest); err != nil {
		return err
	}
	if _, err := authority.NewDigest(row.projectBindingDigest); err != nil {
		return err
	}
	if _, err := authority.NewVerifierIdentity(row.verifierIdentity); err != nil {
		return err
	}
	if _, err := authority.NewVerifierVersion(row.verifierVersion); err != nil {
		return err
	}
	if _, err := authority.NewVerificationPolicyRef(row.verificationPolicyRef); err != nil {
		return err
	}
	if _, err := authority.NewDigest(row.verificationPolicyDigest); err != nil {
		return err
	}
	if _, err := parseV3Time(row.checkedAt); err != nil {
		return err
	}
	if row.recordedAt != row.checkedAt || row.currentnessResult != "current" ||
		row.predicateResult != "satisfied" || row.admissionResult != "admitted" {
		return fmt.Errorf("v3 profile authority resolution has invalid judgement state")
	}
	explicit := row.mode == v3ExplicitAuthorityMode &&
		row.resolutionKind == v3ExplicitResolutionKind &&
		row.strictPermissionRef == "" && row.strictPermissionDigest == ""
	strict := row.mode == v3StrictAuthorityMode &&
		row.resolutionKind == v3StrictResolutionKind &&
		row.strictPermissionRef != "" && row.strictPermissionDigest != ""
	if !explicit && !strict {
		return fmt.Errorf("v3 profile authority resolution has an invalid closed authority branch")
	}
	return nil
}

func validateV3WorkInput(row v3WorkInputRow) error {
	if _, err := profiledeclarationpreparation.
		DecodeCanonicalProfileOnboardingWorkInput(
			[]byte(row.canonicalJSON),
		); err != nil {
		return fmt.Errorf(
			"decode canonical v3 profile WorkInput: %w",
			err,
		)
	}
	dto := v3WorkInputJSON{}
	decoder := json.NewDecoder(strings.NewReader(row.canonicalJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return fmt.Errorf("decode v3 profile WorkInput: %w", err)
	}
	if err := requireV3JSONEOF(decoder); err != nil {
		return err
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: dto.Schema == v3WorkInputSchema, name: "schema"},
		{matches: dto.ProjectRoot == row.projectRoot, name: "project root"},
		{matches: dto.SuggestionRef == row.suggestionRef, name: "suggestion ref"},
		{matches: dto.DetectorVersion == row.detectorVersion, name: "detector version"},
		{matches: dto.PolicyVersion == row.policyVersion, name: "policy version"},
		{matches: dto.ObservationDigest == row.observationDigest, name: "observation digest"},
		{matches: strings.TrimSpace(dto.SuggestionRef) == dto.SuggestionRef && dto.SuggestionRef != "", name: "suggestion ref"},
		{matches: strings.TrimSpace(dto.DetectorVersion) == dto.DetectorVersion && dto.DetectorVersion != "", name: "detector version"},
		{matches: strings.TrimSpace(dto.PolicyVersion) == dto.PolicyVersion && dto.PolicyVersion != "", name: "policy version"},
		{matches: len(dto.Scopes) > 0, name: "scopes"},
	}
	if err := firstMismatch(checks, "v3 profile WorkInput"); err != nil {
		return err
	}
	canonical, err := json.Marshal(dto)
	if err != nil || !bytes.Equal(canonical, []byte(row.canonicalJSON)) {
		return fmt.Errorf("v3 profile WorkInput JSON is not canonical")
	}
	if canonicalV3Digest(v3WorkInputSchema, canonical) != row.digest {
		return fmt.Errorf("v3 profile WorkInput digest does not match canonical JSON")
	}
	expectedRef := "profile-onboarding-work-input:" + strings.TrimPrefix(row.digest, "sha256:")
	if row.ref != expectedRef {
		return fmt.Errorf("v3 profile WorkInput ref does not match its digest")
	}
	if _, err := projectprofile.NewContentDigest(row.payloadDigest); err != nil {
		return err
	}
	if _, err := projectprofile.NewContentDigest(row.observationDigest); err != nil {
		return err
	}
	if _, err := projectprofile.NewProjectRootV1(row.projectRoot); err != nil {
		return err
	}
	if err := validateV3WorkInputPayload(row, dto); err != nil {
		return err
	}
	if _, err := parseV3Time(row.recordedAt); err != nil {
		return err
	}
	return nil
}

func validateV3WorkInputPayload(row v3WorkInputRow, dto v3WorkInputJSON) error {
	payload, err := projectprofile.DecodeProfileDeclarationPayloadCanonicalJSON(
		[]byte(row.payloadJSON),
	)
	if err != nil {
		return fmt.Errorf("decode v3 WorkInput profile payload: %w", err)
	}
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(payload)
	if err != nil {
		return err
	}
	if payloadDigest.String() != row.payloadDigest {
		return fmt.Errorf("v3 WorkInput profile payload digest does not match its canonical payload")
	}
	scopes := payload.Scopes().Values()
	if len(scopes) != len(dto.Scopes) {
		return fmt.Errorf("v3 WorkInput scopes do not match its canonical profile payload")
	}
	byID := make(map[string]projectprofile.RealizationScope, len(scopes))
	for _, scope := range scopes {
		byID[scope.ScopeID().String()] = scope
	}
	previousComponent := ""
	for _, declaration := range dto.Scopes {
		component := declaration.ComponentCandidateRef
		if component == "" || strings.TrimSpace(component) != component || component <= previousComponent {
			return fmt.Errorf("v3 WorkInput component candidates are not canonical and unique")
		}
		previousComponent = component
		scope, ok := byID[declaration.ScopeID]
		if !ok {
			return fmt.Errorf("v3 WorkInput scope %q is absent from its profile payload", declaration.ScopeID)
		}
		if err := validateV3WorkInputScope(declaration, scope); err != nil {
			return fmt.Errorf("v3 WorkInput scope %q: %w", declaration.ScopeID, err)
		}
		delete(byID, declaration.ScopeID)
	}
	if len(byID) != 0 {
		return fmt.Errorf("v3 WorkInput profile payload contains unbound scopes")
	}
	return nil
}

func validateV3WorkInputScope(
	declaration v3WorkInputScopeJSON,
	scope projectprofile.RealizationScope,
) error {
	switch value := scope.(type) {
	case projectprofile.SoftwareRealization:
		return validateV3SoftwareWorkInputScope(declaration, value)
	case projectprofile.NonSoftwareRealization:
		return validateV3NonSoftwareWorkInputScope(declaration, value)
	default:
		return fmt.Errorf("profile payload contains an unknown realization variant")
	}
}

func validateV3SoftwareWorkInputScope(
	declaration v3WorkInputScopeJSON,
	scope projectprofile.SoftwareRealization,
) error {
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: declaration.RealizationKind == "software", name: "realization kind"},
		{matches: declaration.EntityRef == v3EntityReference(scope.EntityReference()), name: "entity ref"},
		{matches: declaration.AdmittedKindRef == "", name: "admitted kind ref"},
		{matches: len(declaration.GoverningPatternRefs) == 0, name: "governing pattern refs"},
		{matches: len(declaration.ContractRefs) == 0, name: "contract refs"},
	}
	return firstMismatch(checks, "software WorkInput scope")
}

func validateV3NonSoftwareWorkInputScope(
	declaration v3WorkInputScopeJSON,
	scope projectprofile.NonSoftwareRealization,
) error {
	patterns := scope.GoverningPatternRefs()
	patternRefs := make([]string, len(patterns))
	for index, ref := range patterns {
		patternRefs[index] = ref.String()
	}
	contracts := scope.ContractRefs()
	contractRefs := make([]string, len(contracts))
	for index, ref := range contracts {
		contractRefs[index] = ref.String()
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: declaration.RealizationKind == "non_software", name: "realization kind"},
		{matches: declaration.EntityRef == v3EntityReference(scope.EntityReference()), name: "entity ref"},
		{matches: declaration.AdmittedKindRef == v3KindOrientation(scope.KindOrientation()), name: "admitted kind ref"},
		{matches: slices.Equal(declaration.GoverningPatternRefs, patternRefs), name: "governing pattern refs"},
		{matches: slices.Equal(declaration.ContractRefs, contractRefs), name: "contract refs"},
	}
	return firstMismatch(checks, "non-software WorkInput scope")
}

func v3EntityReference(reference projectprofile.EntityReference) string {
	switch value := reference.(type) {
	case projectprofile.NoEntityReference:
		return ""
	case projectprofile.ReferencedEntity:
		return value.Ref().String()
	default:
		return "<unknown>"
	}
}

func v3KindOrientation(orientation projectprofile.KindOrientation) string {
	switch value := orientation.(type) {
	case projectprofile.UnspecifiedKindOrientation:
		return ""
	case projectprofile.ReferencedKindOrientation:
		return value.Ref().String()
	default:
		return "<unknown>"
	}
}

func requireCanonicalV3JSON(
	schema string,
	raw string,
	digest string,
	value any,
) error {
	canonical, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if !bytes.Equal(canonical, []byte(raw)) {
		return fmt.Errorf("canonical JSON differs from exact typed material")
	}
	if canonicalV3Digest(schema, canonical) != digest {
		return fmt.Errorf("digest differs from canonical JSON")
	}
	return nil
}

func canonicalV3Digest(schema string, canonical []byte) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(schema))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(canonical)
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func requireV3JSONEOF(decoder *json.Decoder) error {
	extra := json.RawMessage{}
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("canonical JSON contains multiple values")
}

func parseV3Time(raw string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse canonical v3 time: %w", err)
	}
	canonical := value.UTC().Format(time.RFC3339Nano)
	if canonical != raw {
		return time.Time{}, fmt.Errorf("v3 time is not canonical UTC RFC3339Nano")
	}
	return value.UTC(), nil
}

func parseV3Window(fromRaw string, untilRaw string) (authority.TimeWindow, error) {
	from, err := parseV3Time(fromRaw)
	if err != nil {
		return authority.TimeWindow{}, err
	}
	until, err := parseV3Time(untilRaw)
	if err != nil {
		return authority.TimeWindow{}, err
	}
	return authority.NewTimeWindow(from, until)
}

func materializeV3Authority(
	closure v3AuthorityClosure,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	strictUse profileauthority.AdmittedUse,
	admissionTime time.Time,
) (authorityMaterial, error) {
	basis := closure.basis
	resolution := closure.resolution
	work := values.WorkRecord()
	if err := validateV3ClosureAgainstCandidateSupport(closure, values, admissionTime); err != nil {
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
	canonicalAdmissionTime := admissionTime.UTC().Round(0)
	if canonicalAdmissionTime.Before(work.WorkInterval().Until()) {
		return authorityMaterial{}, fmt.Errorf("v3 profile authority cannot be consumed before performed Work ended")
	}
	if canonicalAdmissionTime.Before(allowedWork.From()) || canonicalAdmissionTime.After(allowedWork.Until()) {
		return authorityMaterial{}, fmt.Errorf("v3 profile authority is not current at admission time")
	}
	material := authorityMaterial{
		resolutionRef:           resolutionRef,
		resolutionDigest:        resolutionDigest,
		authorityBasisRef:       basisRef,
		authorityBasisDigest:    basisDigest,
		projectRoot:             projectRoot,
		actionKind:              actionKind,
		projectBindingHash:      projectBindingDigest,
		profileAuthorRef:        profileAuthorRef,
		profileAuthorDigest:     profileAuthorDigest,
		methodDescriptionRef:    methodDescriptionRef,
		methodDescriptionDigest: methodDescriptionDigest,
		methodContractRef:       methodContractRef,
		methodContractDigest:    methodContractDigest,
		classifierVersion:       classifierVersion,
		policyVersion:           policyVersion,
		futureWorkSession:       sessionRef,
		allowedWork:             allowedWork,
		allowedBasisObservation: allowedBasis,
		permissionValidity:      allowedWork,
		singleUseKey:            singleUseKey,
		checkedAt:               checkedAt,
		judgementTime:           canonicalAdmissionTime,
		authorityMode:           basis.mode,
		resolutionKind:          resolution.resolutionKind,
		workInputRef:            basis.workInputRef,
		workInputDigest:         basis.workInputDigest,
		permissionRequired:      basis.mode == v3StrictAuthorityMode,
	}
	if basis.mode == v3StrictAuthorityMode {
		strict, err := materializeAuthority(strictUse)
		if err != nil {
			return authorityMaterial{}, err
		}
		if err := validateStrictV3Wrapper(closure, strict, canonicalAdmissionTime); err != nil {
			return authorityMaterial{}, err
		}
		material.permissionRef = strict.permissionRef
		material.permissionDigest = strict.permissionDigest
		material.permissionValidity = strict.permissionValidity
	}
	return material, nil
}

func validateV3ClosureAgainstCandidateSupport(
	closure v3AuthorityClosure,
	values projectprofilesqlite.ProfileOnboardingValueSetV1,
	admissionTime time.Time,
) error {
	basis := closure.basis
	resolution := closure.resolution
	workInput := closure.workInput
	work := values.WorkRecord()
	candidateOutcome, ok := work.Outcome().(projectprofile.CandidatePayloadProduced)
	if !ok {
		return fmt.Errorf("v3 profile authority support requires CandidatePayloadProduced Work outcome")
	}
	payloadDigest := candidateOutcome.PayloadDigest().String()
	inputFound := false
	for _, ref := range work.InputRefs() {
		inputFound = inputFound || ref.String() == basis.workInputRef
	}
	checkedAt, err := parseV3Time(resolution.checkedAt)
	if err != nil {
		return err
	}
	workFrom := work.WorkInterval().From()
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: inputFound, name: "WorkInput occurrence"},
		{matches: workInput.payloadDigest == payloadDigest, name: "WorkInput payload digest"},
		{matches: !workFrom.Before(checkedAt), name: "Work start after authority resolution"},
		{matches: !admissionTime.UTC().Before(work.WorkInterval().Until()), name: "admission after Work end"},
	}
	return firstMismatch(checks, "v3 profile authority support")
}

func validateStrictV3Wrapper(
	closure v3AuthorityClosure,
	strict authorityMaterial,
	admissionTime time.Time,
) error {
	basis := closure.basis
	resolution := closure.resolution
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: strict.authorityBasisRef.String() == basis.strictAuthorityBasisRef, name: "strict basis ref"},
		{matches: strict.authorityBasisDigest.String() == basis.strictAuthorityBasisDigest, name: "strict basis digest"},
		{matches: strict.permissionRef.String() == resolution.strictPermissionRef, name: "strict permission ref"},
		{matches: strict.permissionDigest.String() == resolution.strictPermissionDigest, name: "strict permission digest"},
		{matches: strict.projectRoot.String() == basis.projectRoot, name: "strict project root"},
		{matches: strict.profileAuthorRef.String() == basis.profileAuthorRef, name: "strict ProfileAuthor"},
		{matches: strict.methodDescriptionRef.String() == basis.methodDescriptionRef, name: "strict MethodDescription"},
		{matches: strict.methodContractRef.String() == basis.methodContractRef, name: "strict MethodContract"},
		{matches: strict.singleUseKey.String() == basis.singleUseKey, name: "strict single-use key"},
		{matches: strict.permissionValidity.Contains(admissionTime), name: "strict permission current at admission"},
	}
	return firstMismatch(checks, "strict v3 profile authority wrapper")
}
