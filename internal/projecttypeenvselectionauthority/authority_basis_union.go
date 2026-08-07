package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	trustedDedicatedCLIInvocationSourceRefPrefix = "project-typeenv-head-selection-trusted-dedicated-cli-invocation:"
	trustedDedicatedCLIInvocationSourceSchema    = "haft.project-typeenv.head-selection-trusted-dedicated-cli-invocation/v1"
	trustedDedicatedCLIInvocationSourceDomain    = "haft.project-typeenv.head-selection-trusted-dedicated-cli-invocation/v1"
	maximumTrustedDedicatedCLIInvocationBytes    = 256 * 1024
)

// TrustedDedicatedCLIInvocationSourceRecordRef identifies the durable
// lower-assurance default-mode ingress record. It does not identify a human
// SpeechAct, host-skill occurrence, Permission, or performed Work.
type TrustedDedicatedCLIInvocationSourceRecordRef struct {
	digest authority.Digest
}

func ParseTrustedDedicatedCLIInvocationSourceRecordRef(
	raw string,
) (TrustedDedicatedCLIInvocationSourceRecordRef, error) {
	digest, err := parseDigestRef(
		"trusted dedicated CLI invocation source",
		trustedDedicatedCLIInvocationSourceRefPrefix,
		raw,
	)
	if err != nil {
		return TrustedDedicatedCLIInvocationSourceRecordRef{}, err
	}
	return TrustedDedicatedCLIInvocationSourceRecordRef{digest: digest}, nil
}

func (ref TrustedDedicatedCLIInvocationSourceRecordRef) Digest() authority.Digest {
	return ref.digest
}

func (ref TrustedDedicatedCLIInvocationSourceRecordRef) String() string {
	return trustedDedicatedCLIInvocationSourceRefPrefix + ref.digest.String()
}

// ProjectTypeEnvHeadSelectionAuthorityCoordinates is a durable projection of
// the exact policy, request, and content coordinates. It is data only:
// possession or decoding grants no authority to select a head.
type ProjectTypeEnvHeadSelectionAuthorityCoordinates struct {
	mode              ProjectTypeEnvHeadSelectionAuthorityMode
	project           projectidentity.ProjectID
	action            ProjectTypeEnvHeadSelectionAction
	requestRef        projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest     typedmemory.SHA256Digest
	contentRef        authority.DescriptionRef
	contentDigest     authority.Digest
	policyRef         ProjectTypeEnvHeadSelectionModePolicyRef
	policyDigest      authority.Digest
	configBasisRef    ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef
	configBasisDigest authority.Digest
	configCarrier     authority.ObservableCarrierBinding
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) Mode() ProjectTypeEnvHeadSelectionAuthorityMode {
	return value.mode
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) Project() projectidentity.ProjectID {
	return value.project
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) Action() ProjectTypeEnvHeadSelectionAction {
	return value.action
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) RequestRef() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef {
	return value.requestRef
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) RequestDigest() typedmemory.SHA256Digest {
	return value.requestDigest
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ContentRef() authority.DescriptionRef {
	return value.contentRef
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ContentDigest() authority.Digest {
	return value.contentDigest
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) PolicyRef() ProjectTypeEnvHeadSelectionModePolicyRef {
	return value.policyRef
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) PolicyDigest() authority.Digest {
	return value.policyDigest
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ConfigBasisRef() ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef {
	return value.configBasisRef
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ConfigBasisDigest() authority.Digest {
	return value.configBasisDigest
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ConfigCarrier() authority.ObservableCarrierBinding {
	return value.configCarrier
}

// TrustedDedicatedCLIInvocationSourceRecordInput describes one request that
// reached the dedicated CLI effect boundary under the configured default
// policy. RecordedAt is an ingress observation time, not a human-act time.
type TrustedDedicatedCLIInvocationSourceRecordInput struct {
	Policy     ExplicitHDecideAuthorityPolicy
	Request    projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Content    ProjectTypeEnvHeadSelectionAuthorizationContent
	RecordedAt time.Time
}

// TrustedDedicatedCLIInvocationSourceRecord is the durable lower-assurance
// source admitted by the default project policy. It records that the dedicated
// CLI accepted an exact request/content pair. It deliberately does not claim
// that the kernel observed h-decide, a human SpeechAct, a Permission, or Work.
type TrustedDedicatedCLIInvocationSourceRecord struct {
	ref           TrustedDedicatedCLIInvocationSourceRecordRef
	digest        authority.Digest
	policy        ExplicitHDecideAuthorityPolicy
	request       projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	content       ProjectTypeEnvHeadSelectionAuthorizationContent
	coordinates   ProjectTypeEnvHeadSelectionAuthorityCoordinates
	recordedAt    time.Time
	canonicalJSON []byte
}

type trustedDedicatedCLIInvocationSourceProjection struct {
	Schema                      string `json:"schema"`
	Mode                        string `json:"mode"`
	PolicyRef                   string `json:"policy_ref"`
	PolicyDigest                string `json:"policy_digest"`
	ConfigBasisRef              string `json:"config_basis_ref"`
	ConfigBasisDigest           string `json:"config_basis_digest"`
	ConfigCarrierRef            string `json:"config_carrier_ref"`
	ConfigCarrierDigest         string `json:"config_carrier_digest"`
	ProjectRoot                 string `json:"project_root"`
	ProjectContextBindingDigest string `json:"project_context_binding_digest"`
	JudgementContext            string `json:"judgement_context_ref"`
	ProjectBindingCarrierRef    string `json:"project_binding_carrier_ref"`
	ProjectBindingCarrierDigest string `json:"project_binding_carrier_digest"`
	ConfiguredHostSkill         string `json:"configured_host_skill"`
	IngressKind                 string `json:"ingress_kind"`
	Project                     string `json:"project_id"`
	Action                      string `json:"action"`
	RequestRef                  string `json:"request_ref"`
	RequestDigest               string `json:"request_digest"`
	ContentRefKind              string `json:"content_ref_kind"`
	ContentRef                  string `json:"content_ref"`
	ContentDigest               string `json:"content_digest"`
	RecordedAt                  string `json:"recorded_at"`
	AuthorityBoundary           string `json:"authority_boundary"`
}

func SealTrustedDedicatedCLIInvocationSourceRecord(
	input TrustedDedicatedCLIInvocationSourceRecordInput,
) (TrustedDedicatedCLIInvocationSourceRecord, error) {
	policy := input.Policy
	request := input.Request
	content := input.Content
	if err := policy.Verify(
		policy.ConfigBasis(),
		policy.ProjectBinding(),
	); err != nil {
		return TrustedDedicatedCLIInvocationSourceRecord{}, err
	}
	if policy.HostSkill() != ProjectTypeEnvHeadSelectionHostSkillHDecide {
		return TrustedDedicatedCLIInvocationSourceRecord{}, fmt.Errorf(
			"explicit policy does not name configured host skill h-decide",
		)
	}
	if err := request.Verify(); err != nil {
		return TrustedDedicatedCLIInvocationSourceRecord{}, fmt.Errorf(
			"dedicated CLI request: %w",
			err,
		)
	}
	if err := content.ExactAgainst(request); err != nil {
		return TrustedDedicatedCLIInvocationSourceRecord{}, fmt.Errorf(
			"dedicated CLI content/request mismatch: %w",
			err,
		)
	}
	if policy.Project() != request.Project() {
		return TrustedDedicatedCLIInvocationSourceRecord{}, fmt.Errorf(
			"explicit policy belongs to another project",
		)
	}
	projectBinding := policy.ProjectBinding()
	if projectBinding.Project() != request.Project() ||
		projectBinding.Context() != content.JudgementContext() {
		return TrustedDedicatedCLIInvocationSourceRecord{}, fmt.Errorf(
			"dedicated CLI content is outside the explicit policy project-context binding",
		)
	}
	recordedAt := input.RecordedAt.Round(0).UTC()
	if recordedAt.IsZero() || !content.ValidityWindow().Contains(recordedAt) {
		return TrustedDedicatedCLIInvocationSourceRecord{}, fmt.Errorf(
			"dedicated CLI ingress time is outside reviewed content validity",
		)
	}
	action := content.Action()
	if _, err := action.AuthorityActionKind(); err != nil {
		return TrustedDedicatedCLIInvocationSourceRecord{}, err
	}
	coordinates := authorityCoordinates(
		policy,
		request,
		content,
		action,
	)
	record := TrustedDedicatedCLIInvocationSourceRecord{
		policy:      policy,
		request:     request,
		content:     content,
		coordinates: coordinates,
		recordedAt:  recordedAt,
	}
	carrier := policy.ConfigCarrier()
	bindingCarrier := projectBinding.Carrier()
	projection := trustedDedicatedCLIInvocationSourceProjection{
		Schema:                      trustedDedicatedCLIInvocationSourceSchema,
		Mode:                        coordinates.Mode().String(),
		PolicyRef:                   coordinates.PolicyRef().String(),
		PolicyDigest:                coordinates.PolicyDigest().String(),
		ConfigBasisRef:              coordinates.ConfigBasisRef().String(),
		ConfigBasisDigest:           coordinates.ConfigBasisDigest().String(),
		ConfigCarrierRef:            carrier.Ref().String(),
		ConfigCarrierDigest:         carrier.Digest().String(),
		ProjectRoot:                 projectBinding.Root().String(),
		ProjectContextBindingDigest: projectBinding.Digest().String(),
		JudgementContext:            projectBinding.Context().String(),
		ProjectBindingCarrierRef:    bindingCarrier.Ref().String(),
		ProjectBindingCarrierDigest: bindingCarrier.Digest().String(),
		ConfiguredHostSkill:         policy.HostSkill().String(),
		IngressKind:                 "dedicated_cli",
		Project:                     coordinates.Project().String(),
		Action:                      coordinates.Action().String(),
		RequestRef:                  coordinates.RequestRef().String(),
		RequestDigest:               coordinates.RequestDigest().String(),
		ContentRefKind:              string(coordinates.ContentRef().Kind()),
		ContentRef:                  coordinates.ContentRef().String(),
		ContentDigest:               coordinates.ContentDigest().String(),
		RecordedAt:                  formatTime(recordedAt),
		AuthorityBoundary:           "configured_lower_assurance_policy_ingress_in_exact_profile_selected_project_context;_not_observed_host_skill_human_speech_act_permission_work_or_receipt",
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return TrustedDedicatedCLIInvocationSourceRecord{}, err
	}
	if len(canonical) == 0 || len(canonical) > maximumTrustedDedicatedCLIInvocationBytes {
		return TrustedDedicatedCLIInvocationSourceRecord{}, fmt.Errorf(
			"trusted dedicated CLI source has invalid canonical size",
		)
	}
	digest, err := digestCanonical(
		trustedDedicatedCLIInvocationSourceDomain,
		canonical,
	)
	if err != nil {
		return TrustedDedicatedCLIInvocationSourceRecord{}, err
	}
	record.ref = TrustedDedicatedCLIInvocationSourceRecordRef{digest: digest}
	record.digest = digest
	record.canonicalJSON = canonical
	return record, nil
}

func DecodeTrustedDedicatedCLIInvocationSourceRecord(
	input TrustedDedicatedCLIInvocationSourceRecordInput,
	canonical []byte,
	digest authority.Digest,
) (TrustedDedicatedCLIInvocationSourceRecord, error) {
	if len(canonical) == 0 || len(canonical) > maximumTrustedDedicatedCLIInvocationBytes {
		return TrustedDedicatedCLIInvocationSourceRecord{}, fmt.Errorf(
			"trusted dedicated CLI source has invalid canonical size",
		)
	}
	projection := trustedDedicatedCLIInvocationSourceProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return TrustedDedicatedCLIInvocationSourceRecord{}, err
	}
	rebuilt, err := SealTrustedDedicatedCLIInvocationSourceRecord(input)
	if err != nil {
		return TrustedDedicatedCLIInvocationSourceRecord{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return TrustedDedicatedCLIInvocationSourceRecord{}, fmt.Errorf(
			"trusted dedicated CLI source is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (record TrustedDedicatedCLIInvocationSourceRecord) Verify(
	input TrustedDedicatedCLIInvocationSourceRecordInput,
) error {
	rebuilt, err := SealTrustedDedicatedCLIInvocationSourceRecord(input)
	if err != nil {
		return err
	}
	if rebuilt.ref != record.ref ||
		rebuilt.digest != record.digest ||
		!bytes.Equal(rebuilt.canonicalJSON, record.canonicalJSON) {
		return fmt.Errorf("trusted dedicated CLI source differs from exact input")
	}
	return nil
}

func (record TrustedDedicatedCLIInvocationSourceRecord) Ref() TrustedDedicatedCLIInvocationSourceRecordRef {
	return record.ref
}

func (record TrustedDedicatedCLIInvocationSourceRecord) Digest() authority.Digest {
	return record.digest
}

func (record TrustedDedicatedCLIInvocationSourceRecord) Policy() ExplicitHDecideAuthorityPolicy {
	return record.policy
}

func (record TrustedDedicatedCLIInvocationSourceRecord) Request() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest {
	return record.request
}

func (record TrustedDedicatedCLIInvocationSourceRecord) Content() ProjectTypeEnvHeadSelectionAuthorizationContent {
	return record.content
}

func (record TrustedDedicatedCLIInvocationSourceRecord) Coordinates() ProjectTypeEnvHeadSelectionAuthorityCoordinates {
	return record.coordinates
}

func (record TrustedDedicatedCLIInvocationSourceRecord) RecordedAt() time.Time {
	return record.recordedAt
}

func (record TrustedDedicatedCLIInvocationSourceRecord) CanonicalJSON() []byte {
	return append([]byte(nil), record.canonicalJSON...)
}

// VerifiedSpeechActAuthoritySourceRecord is the strict source variant. It is
// an exact durable description of verified communicative Work and delegates
// identity to that source record rather than inventing a second occurrence.
type VerifiedSpeechActAuthoritySourceRecord struct {
	record ProjectTypeEnvHeadSelectionSpeechActRecord
}

func SealVerifiedSpeechActAuthoritySourceRecord(
	record ProjectTypeEnvHeadSelectionSpeechActRecord,
) (VerifiedSpeechActAuthoritySourceRecord, error) {
	content := record.Content()
	if err := content.Verify(); err != nil {
		return VerifiedSpeechActAuthoritySourceRecord{}, err
	}
	if err := record.Verify(content.Request()); err != nil {
		return VerifiedSpeechActAuthoritySourceRecord{}, err
	}
	if record.Ref().Digest() != record.Digest() {
		return VerifiedSpeechActAuthoritySourceRecord{}, fmt.Errorf(
			"verified SpeechAct authority source ref/digest mismatch",
		)
	}
	return VerifiedSpeechActAuthoritySourceRecord{record: record}, nil
}

func (record VerifiedSpeechActAuthoritySourceRecord) Verify() error {
	_, err := SealVerifiedSpeechActAuthoritySourceRecord(record.record)
	return err
}

func (record VerifiedSpeechActAuthoritySourceRecord) Ref() ProjectTypeEnvHeadSelectionSpeechActRecordRef {
	return record.record.Ref()
}

func (record VerifiedSpeechActAuthoritySourceRecord) Digest() authority.Digest {
	return record.record.Digest()
}

func (record VerifiedSpeechActAuthoritySourceRecord) Record() ProjectTypeEnvHeadSelectionSpeechActRecord {
	return record.record
}

func (record VerifiedSpeechActAuthoritySourceRecord) CanonicalJSON() []byte {
	return record.record.CanonicalJSON()
}

// AuthoritySourceKind is the closed durable source discriminator.
type AuthoritySourceKind uint8

const (
	AuthoritySourceTrustedDedicatedCLIInvocation AuthoritySourceKind = iota + 1
	AuthoritySourceVerifiedSpeechAct
)

func (kind AuthoritySourceKind) String() string {
	switch kind {
	case AuthoritySourceTrustedDedicatedCLIInvocation:
		return "trusted_dedicated_cli_invocation"
	case AuthoritySourceVerifiedSpeechAct:
		return "verified_speech_act"
	default:
		return ""
	}
}

type authoritySourceVariant interface {
	authoritySourceVariant()
}

func (TrustedDedicatedCLIInvocationSourceRecord) authoritySourceVariant() {}
func (VerifiedSpeechActAuthoritySourceRecord) authoritySourceVariant()    {}

// AuthoritySourceRecord is the closed durable sum:
//
//	TrustedDedicatedCLIInvocationSourceRecord | VerifiedSpeechActAuthoritySourceRecord
//
// It is replayable source data, not a live authority capability.
type AuthoritySourceRecord struct {
	variant authoritySourceVariant
}

func NewAuthoritySourceFromTrustedDedicatedCLIInvocation(
	record TrustedDedicatedCLIInvocationSourceRecord,
) (AuthoritySourceRecord, error) {
	input := TrustedDedicatedCLIInvocationSourceRecordInput{
		Policy:     record.Policy(),
		Request:    record.Request(),
		Content:    record.Content(),
		RecordedAt: record.RecordedAt(),
	}
	if err := record.Verify(input); err != nil {
		return AuthoritySourceRecord{}, err
	}
	return AuthoritySourceRecord{variant: record}, nil
}

func NewAuthoritySourceFromVerifiedSpeechAct(
	record VerifiedSpeechActAuthoritySourceRecord,
) (AuthoritySourceRecord, error) {
	if err := record.Verify(); err != nil {
		return AuthoritySourceRecord{}, err
	}
	return AuthoritySourceRecord{variant: record}, nil
}

func (record AuthoritySourceRecord) Kind() AuthoritySourceKind {
	switch record.variant.(type) {
	case TrustedDedicatedCLIInvocationSourceRecord:
		return AuthoritySourceTrustedDedicatedCLIInvocation
	case VerifiedSpeechActAuthoritySourceRecord:
		return AuthoritySourceVerifiedSpeechAct
	default:
		return 0
	}
}

func (record AuthoritySourceRecord) TrustedDedicatedCLIInvocation() (
	TrustedDedicatedCLIInvocationSourceRecord,
	bool,
) {
	value, ok := record.variant.(TrustedDedicatedCLIInvocationSourceRecord)
	return value, ok
}

func (record AuthoritySourceRecord) VerifiedSpeechAct() (
	VerifiedSpeechActAuthoritySourceRecord,
	bool,
) {
	value, ok := record.variant.(VerifiedSpeechActAuthoritySourceRecord)
	return value, ok
}

func authorityCoordinates(
	policy ExplicitHDecideAuthorityPolicy,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
	action ProjectTypeEnvHeadSelectionAction,
) ProjectTypeEnvHeadSelectionAuthorityCoordinates {
	return ProjectTypeEnvHeadSelectionAuthorityCoordinates{
		mode:              policy.Mode(),
		project:           request.Project(),
		action:            action,
		requestRef:        request.Ref(),
		requestDigest:     request.Ref().Digest(),
		contentRef:        content.DescriptionRef(),
		contentDigest:     content.Digest(),
		policyRef:         policy.Ref(),
		policyDigest:      policy.Digest(),
		configBasisRef:    policy.ConfigBasis().Ref(),
		configBasisDigest: policy.ConfigBasis().Digest(),
		configCarrier:     policy.ConfigCarrier(),
	}
}
