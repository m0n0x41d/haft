package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

const (
	explicitPolicyAcceptanceResolutionSchema = "haft.project-typeenv.head-selection-explicit-policy-acceptance-resolution/v1"
	explicitPolicyAcceptanceResolutionDomain = "haft.project-typeenv.head-selection-explicit-policy-acceptance-resolution/v1"
	strictPermissionResolutionSchema         = "haft.project-typeenv.head-selection-strict-permission-resolution/v1"
	strictPermissionResolutionDomain         = "haft.project-typeenv.head-selection-strict-permission-resolution/v1"
	maximumAuthorityResolutionBytes          = 256 * 1024
)

// ExplicitPolicyAcceptanceResolutionInput evaluates the lower-assurance
// dedicated-CLI source under its exact configured policy. This is a policy
// acceptance record, not proof that a human or host performed h-decide.
type ExplicitPolicyAcceptanceResolutionInput struct {
	Source      TrustedDedicatedCLIInvocationSourceRecord
	EvaluatedAt time.Time
}

// ExplicitPolicyAcceptanceResolution is the default-mode durable resolution.
// It records only configured policy acceptance of an exact dedicated-CLI
// ingress. It contains no SpeechAct, Permission, Work, or human-act receipt.
type ExplicitPolicyAcceptanceResolution struct {
	ref           ProjectTypeEnvHeadSelectionAuthorityResolutionRef
	digest        authority.Digest
	source        TrustedDedicatedCLIInvocationSourceRecord
	evaluatedAt   time.Time
	canonicalJSON []byte
}

type explicitPolicyAcceptanceResolutionProjection struct {
	Schema              string `json:"schema"`
	Verdict             string `json:"verdict"`
	SourceRef           string `json:"trusted_dedicated_cli_source_ref"`
	SourceDigest        string `json:"trusted_dedicated_cli_source_digest"`
	PolicyRef           string `json:"policy_ref"`
	PolicyDigest        string `json:"policy_digest"`
	ConfigBasisRef      string `json:"config_basis_ref"`
	ConfigBasisDigest   string `json:"config_basis_digest"`
	ContentRef          string `json:"content_ref"`
	ContentDigest       string `json:"content_digest"`
	RequestRef          string `json:"request_ref"`
	RequestDigest       string `json:"request_digest"`
	EvaluatedAt         string `json:"evaluated_at"`
	InterpretationLimit string `json:"interpretation_limit"`
}

func SealExplicitPolicyAcceptanceResolution(
	input ExplicitPolicyAcceptanceResolutionInput,
) (ExplicitPolicyAcceptanceResolution, error) {
	source := input.Source
	sourceInput := TrustedDedicatedCLIInvocationSourceRecordInput{
		Policy:     source.Policy(),
		Request:    source.Request(),
		Content:    source.Content(),
		RecordedAt: source.RecordedAt(),
	}
	if err := source.Verify(sourceInput); err != nil {
		return ExplicitPolicyAcceptanceResolution{}, fmt.Errorf(
			"explicit policy resolution source: %w",
			err,
		)
	}
	evaluatedAt := input.EvaluatedAt.Round(0).UTC()
	if evaluatedAt.IsZero() || evaluatedAt.Before(source.RecordedAt()) {
		return ExplicitPolicyAcceptanceResolution{}, fmt.Errorf(
			"explicit policy evaluation precedes dedicated CLI ingress",
		)
	}
	if !source.Content().ValidityWindow().Contains(evaluatedAt) {
		return ExplicitPolicyAcceptanceResolution{}, fmt.Errorf(
			"explicit policy evaluation is outside reviewed content validity",
		)
	}
	resolution := ExplicitPolicyAcceptanceResolution{
		source:      source,
		evaluatedAt: evaluatedAt,
	}
	coordinates := source.Coordinates()
	projection := explicitPolicyAcceptanceResolutionProjection{
		Schema:              explicitPolicyAcceptanceResolutionSchema,
		Verdict:             "accepted_by_configured_explicit_h_decide_policy",
		SourceRef:           source.Ref().String(),
		SourceDigest:        source.Digest().String(),
		PolicyRef:           coordinates.PolicyRef().String(),
		PolicyDigest:        coordinates.PolicyDigest().String(),
		ConfigBasisRef:      coordinates.ConfigBasisRef().String(),
		ConfigBasisDigest:   coordinates.ConfigBasisDigest().String(),
		ContentRef:          coordinates.ContentRef().String(),
		ContentDigest:       coordinates.ContentDigest().String(),
		RequestRef:          coordinates.RequestRef().String(),
		RequestDigest:       coordinates.RequestDigest().String(),
		EvaluatedAt:         formatTime(evaluatedAt),
		InterpretationLimit: "lower_assurance_policy_acceptance_only;_not_observed_h_decide_speech_act_permission_work_or_human_receipt",
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ExplicitPolicyAcceptanceResolution{}, err
	}
	if len(canonical) == 0 || len(canonical) > maximumAuthorityResolutionBytes {
		return ExplicitPolicyAcceptanceResolution{}, fmt.Errorf(
			"explicit policy acceptance resolution has invalid canonical size",
		)
	}
	digest, err := digestCanonical(
		explicitPolicyAcceptanceResolutionDomain,
		canonical,
	)
	if err != nil {
		return ExplicitPolicyAcceptanceResolution{}, err
	}
	resolution.ref = ProjectTypeEnvHeadSelectionAuthorityResolutionRef{
		digest: digest,
	}
	resolution.digest = digest
	resolution.canonicalJSON = canonical
	return resolution, nil
}

func DecodeExplicitPolicyAcceptanceResolution(
	input ExplicitPolicyAcceptanceResolutionInput,
	canonical []byte,
	digest authority.Digest,
) (ExplicitPolicyAcceptanceResolution, error) {
	if len(canonical) == 0 || len(canonical) > maximumAuthorityResolutionBytes {
		return ExplicitPolicyAcceptanceResolution{}, fmt.Errorf(
			"explicit policy acceptance resolution has invalid canonical size",
		)
	}
	projection := explicitPolicyAcceptanceResolutionProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ExplicitPolicyAcceptanceResolution{}, err
	}
	rebuilt, err := SealExplicitPolicyAcceptanceResolution(input)
	if err != nil {
		return ExplicitPolicyAcceptanceResolution{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ExplicitPolicyAcceptanceResolution{}, fmt.Errorf(
			"explicit policy acceptance resolution is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (resolution ExplicitPolicyAcceptanceResolution) Verify(
	input ExplicitPolicyAcceptanceResolutionInput,
) error {
	rebuilt, err := SealExplicitPolicyAcceptanceResolution(input)
	if err != nil {
		return err
	}
	if rebuilt.ref != resolution.ref ||
		rebuilt.digest != resolution.digest ||
		!bytes.Equal(rebuilt.canonicalJSON, resolution.canonicalJSON) {
		return fmt.Errorf("explicit policy acceptance resolution differs from exact input")
	}
	return nil
}

func (resolution ExplicitPolicyAcceptanceResolution) Ref() ProjectTypeEnvHeadSelectionAuthorityResolutionRef {
	return resolution.ref
}

func (resolution ExplicitPolicyAcceptanceResolution) Digest() authority.Digest {
	return resolution.digest
}

func (resolution ExplicitPolicyAcceptanceResolution) Source() TrustedDedicatedCLIInvocationSourceRecord {
	return resolution.source
}

func (resolution ExplicitPolicyAcceptanceResolution) EvaluatedAt() time.Time {
	return resolution.evaluatedAt
}

func (resolution ExplicitPolicyAcceptanceResolution) CanonicalJSON() []byte {
	return append([]byte(nil), resolution.canonicalJSON...)
}

// StrictPermissionResolution is the admitted projection of one already-sealed
// verified SpeechAct occurrence basis and its distinct U.Commitment(MAY)
// Permission. It is durable data, not a live or consumed authority use.
type StrictPermissionResolution struct {
	ref           ProjectTypeEnvHeadSelectionAuthorityResolutionRef
	digest        authority.Digest
	basis         ProjectTypeEnvHeadSelectionAuthorityResolutionBasis
	source        VerifiedSpeechActAuthoritySourceRecord
	permission    ProjectTypeEnvHeadSelectionPermissionRecord
	canonicalJSON []byte
}

type strictPermissionResolutionProjection struct {
	Schema                 string `json:"schema"`
	Verdict                string `json:"verdict"`
	BasisRef               string `json:"authority_resolution_basis_ref"`
	BasisDigest            string `json:"authority_resolution_basis_digest"`
	ResolverPolicyRef      string `json:"resolver_policy_ref"`
	ResolverPolicyEdition  string `json:"resolver_policy_edition"`
	ResolverPolicyDigest   string `json:"resolver_policy_digest"`
	SpeechActRecordRef     string `json:"speech_act_record_ref"`
	SpeechActRecordDigest  string `json:"speech_act_record_digest"`
	ContentDescriptionRef  string `json:"content_description_ref"`
	ContentDigest          string `json:"content_digest"`
	PermissionRef          string `json:"permission_ref"`
	PermissionRecordDigest string `json:"permission_record_digest"`
	RequestRef             string `json:"request_ref"`
	RequestDigest          string `json:"request_digest"`
	Stage                  string `json:"stage_ref"`
	EvaluatedAt            string `json:"evaluated_at"`
	UseBoundary            string `json:"use_boundary"`
}

func SealStrictPermissionResolution(
	basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis,
) (StrictPermissionResolution, error) {
	input := ProjectTypeEnvHeadSelectionAuthorityResolutionBasisInput{
		Policy:      basis.Policy(),
		Record:      basis.Record(),
		Content:     basis.Content(),
		Request:     basis.Request(),
		Stage:       basis.Stage(),
		EvaluatedAt: basis.EvaluatedAt(),
	}
	if err := basis.Verify(input); err != nil {
		return StrictPermissionResolution{}, fmt.Errorf(
			"strict Permission resolution requires an exact sealed basis: %w",
			err,
		)
	}
	source, err := SealVerifiedSpeechActAuthoritySourceRecord(basis.Record())
	if err != nil {
		return StrictPermissionResolution{}, err
	}
	permission := basis.Record().PermissionRecord()
	if err := permission.Verify(
		basis.Content(),
		basis.Record().Source(),
	); err != nil {
		return StrictPermissionResolution{}, fmt.Errorf(
			"strict Permission resolution: %w",
			err,
		)
	}
	resolution := StrictPermissionResolution{
		basis:      basis,
		source:     source,
		permission: permission,
	}
	projection := projectStrictPermissionResolution(resolution)
	canonical, err := json.Marshal(projection)
	if err != nil {
		return StrictPermissionResolution{}, err
	}
	if len(canonical) == 0 || len(canonical) > maximumAuthorityResolutionBytes {
		return StrictPermissionResolution{}, fmt.Errorf(
			"strict Permission resolution has invalid canonical size",
		)
	}
	digest, err := digestCanonical(strictPermissionResolutionDomain, canonical)
	if err != nil {
		return StrictPermissionResolution{}, err
	}
	resolution.digest = digest
	resolution.ref = ProjectTypeEnvHeadSelectionAuthorityResolutionRef{
		digest: digest,
	}
	resolution.canonicalJSON = canonical
	return resolution, nil
}

func projectStrictPermissionResolution(
	resolution StrictPermissionResolution,
) strictPermissionResolutionProjection {
	basis := resolution.basis
	policy := basis.Policy()
	record := basis.Record()
	content := basis.Content()
	request := basis.Request()
	return strictPermissionResolutionProjection{
		Schema:                 strictPermissionResolutionSchema,
		Verdict:                "admitted_by_verified_speech_act_permission_policy",
		BasisRef:               basis.Ref().String(),
		BasisDigest:            basis.Digest().String(),
		ResolverPolicyRef:      policy.Ref().String(),
		ResolverPolicyEdition:  policy.Edition().String(),
		ResolverPolicyDigest:   policy.Digest().String(),
		SpeechActRecordRef:     record.Ref().String(),
		SpeechActRecordDigest:  record.Digest().String(),
		ContentDescriptionRef:  content.DescriptionRef().String(),
		ContentDigest:          content.Digest().String(),
		PermissionRef:          resolution.permission.Ref().String(),
		PermissionRecordDigest: resolution.permission.Digest().String(),
		RequestRef:             request.Ref().String(),
		RequestDigest:          requestDigest(request),
		Stage:                  basis.Stage().Ref().String(),
		EvaluatedAt:            formatTime(basis.EvaluatedAt()),
		UseBoundary:            "durable_resolution_only;_effect_service_mints_and_consumes_nonserializable_single_use_capability",
	}
}

func DecodeStrictPermissionResolution(
	basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis,
	canonical []byte,
	digest authority.Digest,
) (StrictPermissionResolution, error) {
	if len(canonical) == 0 || len(canonical) > maximumAuthorityResolutionBytes {
		return StrictPermissionResolution{}, fmt.Errorf(
			"strict Permission resolution has invalid canonical size",
		)
	}
	projection := strictPermissionResolutionProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return StrictPermissionResolution{}, err
	}
	rebuilt, err := SealStrictPermissionResolution(basis)
	if err != nil {
		return StrictPermissionResolution{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return StrictPermissionResolution{}, fmt.Errorf(
			"strict Permission resolution is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (resolution StrictPermissionResolution) Verify(
	basis ProjectTypeEnvHeadSelectionAuthorityResolutionBasis,
) error {
	rebuilt, err := SealStrictPermissionResolution(basis)
	if err != nil {
		return err
	}
	if rebuilt.ref != resolution.ref ||
		rebuilt.digest != resolution.digest ||
		!bytes.Equal(rebuilt.canonicalJSON, resolution.canonicalJSON) {
		return fmt.Errorf("strict Permission resolution differs from exact basis")
	}
	return nil
}

func (resolution StrictPermissionResolution) Ref() ProjectTypeEnvHeadSelectionAuthorityResolutionRef {
	return resolution.ref
}

func (resolution StrictPermissionResolution) Digest() authority.Digest {
	return resolution.digest
}

func (resolution StrictPermissionResolution) Basis() ProjectTypeEnvHeadSelectionAuthorityResolutionBasis {
	return resolution.basis
}

func (resolution StrictPermissionResolution) Source() VerifiedSpeechActAuthoritySourceRecord {
	return resolution.source
}

func (resolution StrictPermissionResolution) Permission() ProjectTypeEnvHeadSelectionPermissionRecord {
	return resolution.permission
}

func (resolution StrictPermissionResolution) EvaluatedAt() time.Time {
	return resolution.basis.EvaluatedAt()
}

func (resolution StrictPermissionResolution) CanonicalJSON() []byte {
	return append([]byte(nil), resolution.canonicalJSON...)
}

// AuthorityResolutionKind is the closed durable resolution discriminator.
type AuthorityResolutionKind uint8

const (
	AuthorityResolutionExplicitPolicyAcceptance AuthorityResolutionKind = iota + 1
	AuthorityResolutionStrictPermission
)

func (kind AuthorityResolutionKind) String() string {
	switch kind {
	case AuthorityResolutionExplicitPolicyAcceptance:
		return "explicit_policy_acceptance"
	case AuthorityResolutionStrictPermission:
		return "strict_permission"
	default:
		return ""
	}
}

type authorityResolutionVariant interface {
	authorityResolutionVariant()
}

func (ExplicitPolicyAcceptanceResolution) authorityResolutionVariant() {}
func (StrictPermissionResolution) authorityResolutionVariant()         {}

// AuthorityResolutionRecord is the closed durable sum:
//
//	ExplicitPolicyAcceptanceResolution | StrictPermissionResolution
//
// Decoding either branch never recreates a live authority use.
type AuthorityResolutionRecord struct {
	variant authorityResolutionVariant
}

func NewAuthorityResolutionFromExplicitPolicyAcceptance(
	resolution ExplicitPolicyAcceptanceResolution,
) (AuthorityResolutionRecord, error) {
	input := ExplicitPolicyAcceptanceResolutionInput{
		Source:      resolution.Source(),
		EvaluatedAt: resolution.EvaluatedAt(),
	}
	if err := resolution.Verify(input); err != nil {
		return AuthorityResolutionRecord{}, err
	}
	return AuthorityResolutionRecord{variant: resolution}, nil
}

func NewAuthorityResolutionFromStrictPermission(
	resolution StrictPermissionResolution,
) (AuthorityResolutionRecord, error) {
	if err := resolution.Verify(resolution.Basis()); err != nil {
		return AuthorityResolutionRecord{}, err
	}
	return AuthorityResolutionRecord{variant: resolution}, nil
}

func (record AuthorityResolutionRecord) Kind() AuthorityResolutionKind {
	switch record.variant.(type) {
	case ExplicitPolicyAcceptanceResolution:
		return AuthorityResolutionExplicitPolicyAcceptance
	case StrictPermissionResolution:
		return AuthorityResolutionStrictPermission
	default:
		return 0
	}
}

func (record AuthorityResolutionRecord) ExplicitPolicyAcceptance() (
	ExplicitPolicyAcceptanceResolution,
	bool,
) {
	value, ok := record.variant.(ExplicitPolicyAcceptanceResolution)
	return value, ok
}

func (record AuthorityResolutionRecord) StrictPermission() (
	StrictPermissionResolution,
	bool,
) {
	value, ok := record.variant.(StrictPermissionResolution)
	return value, ok
}
