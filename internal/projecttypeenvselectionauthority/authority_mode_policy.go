package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
)

const (
	configAuthorityBasisRefPrefix = "project-typeenv-head-selection-config-authority-basis:"
	authorityModePolicyRefPrefix  = "project-typeenv-head-selection-mode-policy:"

	configAuthorityBasisSchema = "haft.project-typeenv.head-selection-config-authority-basis/v1"
	configAuthorityBasisDomain = "haft.project-typeenv.head-selection-config-authority-basis/v1"

	explicitHDecidePolicySchema = "haft.project-typeenv.head-selection-mode-policy.explicit-h-decide/v1"
	explicitHDecidePolicyDomain = "haft.project-typeenv.head-selection-mode-policy.explicit-h-decide/v1"

	strictCLISpeechActPolicySchema = "haft.project-typeenv.head-selection-mode-policy.strict-cli-speech-act/v1"
	strictCLISpeechActPolicyDomain = "haft.project-typeenv.head-selection-mode-policy.strict-cli-speech-act/v1"

	maximumAuthorityModePolicyBytes = 128 * 1024
)

// ProjectTypeEnvHeadSelectionAuthorityMode mirrors the two admitted
// project-config values without importing the outer project-config adapter.
// The adapter must parse .haft/config.yaml and select exactly one constructor.
type ProjectTypeEnvHeadSelectionAuthorityMode uint8

const (
	ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide ProjectTypeEnvHeadSelectionAuthorityMode = iota + 1
	ProjectTypeEnvHeadSelectionAuthorityModeStrictCLISpeechAct
)

func (mode ProjectTypeEnvHeadSelectionAuthorityMode) String() string {
	switch mode {
	case ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide:
		return "explicit_h_decide"
	case ProjectTypeEnvHeadSelectionAuthorityModeStrictCLISpeechAct:
		return "strict_cli_speech_act"
	default:
		return ""
	}
}

type ProjectTypeEnvHeadSelectionHostSkill uint8

const (
	ProjectTypeEnvHeadSelectionHostSkillHDecide ProjectTypeEnvHeadSelectionHostSkill = iota + 1
)

func (skill ProjectTypeEnvHeadSelectionHostSkill) String() string {
	switch skill {
	case ProjectTypeEnvHeadSelectionHostSkillHDecide:
		return "h-decide"
	default:
		return ""
	}
}

type ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef struct {
	digest authority.Digest
}

func ParseProjectTypeEnvHeadSelectionConfigAuthorityBasisRef(
	raw string,
) (ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef, error) {
	digest, err := parseDigestRef(
		"head-selection config authority basis",
		configAuthorityBasisRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef{}, err
	}
	return ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef) Digest() authority.Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef) String() string {
	return configAuthorityBasisRefPrefix + ref.digest.String()
}

// ProjectTypeEnvHeadSelectionConfigAuthorityBasis is the pure boundary object
// produced after the outer adapter parses .haft/config.yaml. It binds the
// parsed mode to the exact config carrier, so a caller cannot accidentally
// feed a strict config into the explicit policy constructor or vice versa.
type ProjectTypeEnvHeadSelectionConfigAuthorityBasis struct {
	ref           ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef
	digest        authority.Digest
	project       projectidentity.ProjectID
	mode          ProjectTypeEnvHeadSelectionAuthorityMode
	configCarrier authority.ObservableCarrierBinding
	canonicalJSON []byte
}

type configAuthorityBasisProjection struct {
	Schema              string `json:"schema"`
	Project             string `json:"project_id"`
	Mode                string `json:"mode"`
	ConfigCarrierRef    string `json:"config_carrier_ref"`
	ConfigCarrierDigest string `json:"config_carrier_digest"`
	AdapterBoundary     string `json:"adapter_boundary"`
}

func SealProjectTypeEnvHeadSelectionConfigAuthorityBasis(
	project projectidentity.ProjectID,
	mode ProjectTypeEnvHeadSelectionAuthorityMode,
	configCarrier authority.ObservableCarrierBinding,
) (ProjectTypeEnvHeadSelectionConfigAuthorityBasis, error) {
	canonicalProject, carrier, err := normalizeModePolicyCoordinates(project, configCarrier)
	if err != nil {
		return ProjectTypeEnvHeadSelectionConfigAuthorityBasis{}, err
	}
	if mode.String() == "" {
		return ProjectTypeEnvHeadSelectionConfigAuthorityBasis{}, fmt.Errorf(
			"head-selection config authority mode is invalid",
		)
	}
	basis := ProjectTypeEnvHeadSelectionConfigAuthorityBasis{
		project:       canonicalProject,
		mode:          mode,
		configCarrier: carrier,
	}
	projection := configAuthorityBasisProjection{
		Schema:              configAuthorityBasisSchema,
		Project:             canonicalProject.String(),
		Mode:                mode.String(),
		ConfigCarrierRef:    carrier.Ref().String(),
		ConfigCarrierDigest: carrier.Digest().String(),
		AdapterBoundary:     "outer_adapter_parses_exact_project_config;_pure_core_binds_result",
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionConfigAuthorityBasis{}, err
	}
	digest, err := digestModePolicy(configAuthorityBasisDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionConfigAuthorityBasis{}, err
	}
	basis.ref = ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef{digest: digest}
	basis.digest = digest
	basis.canonicalJSON = canonical
	return basis, nil
}

func DecodeProjectTypeEnvHeadSelectionConfigAuthorityBasis(
	project projectidentity.ProjectID,
	mode ProjectTypeEnvHeadSelectionAuthorityMode,
	configCarrier authority.ObservableCarrierBinding,
	canonical []byte,
	digest authority.Digest,
) (ProjectTypeEnvHeadSelectionConfigAuthorityBasis, error) {
	if err := validateModePolicyCanonicalSize(canonical); err != nil {
		return ProjectTypeEnvHeadSelectionConfigAuthorityBasis{}, err
	}
	projection := configAuthorityBasisProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionConfigAuthorityBasis{}, err
	}
	rebuilt, err := SealProjectTypeEnvHeadSelectionConfigAuthorityBasis(
		project,
		mode,
		configCarrier,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionConfigAuthorityBasis{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ProjectTypeEnvHeadSelectionConfigAuthorityBasis{}, fmt.Errorf(
			"head-selection config authority basis is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (basis ProjectTypeEnvHeadSelectionConfigAuthorityBasis) Verify(
	project projectidentity.ProjectID,
	mode ProjectTypeEnvHeadSelectionAuthorityMode,
	configCarrier authority.ObservableCarrierBinding,
) error {
	rebuilt, err := SealProjectTypeEnvHeadSelectionConfigAuthorityBasis(
		project,
		mode,
		configCarrier,
	)
	if err != nil {
		return err
	}
	if basis.ref != rebuilt.ref ||
		basis.digest != rebuilt.digest ||
		!bytes.Equal(basis.canonicalJSON, rebuilt.canonicalJSON) {
		return fmt.Errorf("head-selection config authority basis differs from exact input")
	}
	return nil
}

func (basis ProjectTypeEnvHeadSelectionConfigAuthorityBasis) Ref() ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef {
	return basis.ref
}

func (basis ProjectTypeEnvHeadSelectionConfigAuthorityBasis) Digest() authority.Digest {
	return basis.digest
}

func (basis ProjectTypeEnvHeadSelectionConfigAuthorityBasis) Project() projectidentity.ProjectID {
	return basis.project
}

func (basis ProjectTypeEnvHeadSelectionConfigAuthorityBasis) Mode() ProjectTypeEnvHeadSelectionAuthorityMode {
	return basis.mode
}

func (basis ProjectTypeEnvHeadSelectionConfigAuthorityBasis) ConfigCarrier() authority.ObservableCarrierBinding {
	return basis.configCarrier
}

func (basis ProjectTypeEnvHeadSelectionConfigAuthorityBasis) CanonicalJSON() []byte {
	return append([]byte(nil), basis.canonicalJSON...)
}

type ProjectTypeEnvHeadSelectionModePolicyRef struct {
	digest authority.Digest
}

func ParseProjectTypeEnvHeadSelectionModePolicyRef(
	raw string,
) (ProjectTypeEnvHeadSelectionModePolicyRef, error) {
	digest, err := parseDigestRef("head-selection mode policy", authorityModePolicyRefPrefix, raw)
	if err != nil {
		return ProjectTypeEnvHeadSelectionModePolicyRef{}, err
	}
	return ProjectTypeEnvHeadSelectionModePolicyRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionModePolicyRef) Digest() authority.Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionModePolicyRef) String() string {
	return authorityModePolicyRefPrefix + ref.digest.String()
}

// AuthorityPolicyKind is the closed durable project-policy discriminator.
type AuthorityPolicyKind uint8

const (
	AuthorityPolicyExplicitHDecide AuthorityPolicyKind = iota + 1
	AuthorityPolicyStrictCLISpeechAct
)

func (kind AuthorityPolicyKind) String() string {
	switch kind {
	case AuthorityPolicyExplicitHDecide:
		return "explicit_h_decide"
	case AuthorityPolicyStrictCLISpeechAct:
		return "strict_cli_speech_act"
	default:
		return ""
	}
}

type authorityPolicyVariant interface {
	projectTypeEnvHeadSelectionAuthorityPolicy()
}

// ProjectTypeEnvHeadSelectionAuthorityPolicyRecord is the one canonical
// by-value sum of the two project-config policies. Its private variant cannot
// contain a caller-supplied pointer or typed nil.
type ProjectTypeEnvHeadSelectionAuthorityPolicyRecord struct {
	variant authorityPolicyVariant
}

func NewAuthorityPolicyFromExplicitHDecide(
	policy ExplicitHDecideAuthorityPolicy,
) (ProjectTypeEnvHeadSelectionAuthorityPolicyRecord, error) {
	if err := policy.Verify(
		policy.ConfigBasis(),
		policy.ProjectBinding(),
	); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityPolicyRecord{}, err
	}
	return ProjectTypeEnvHeadSelectionAuthorityPolicyRecord{
		variant: policy,
	}, nil
}

func NewAuthorityPolicyFromStrictCLISpeechAct(
	policy StrictCLISpeechActAuthorityPolicy,
) (ProjectTypeEnvHeadSelectionAuthorityPolicyRecord, error) {
	if err := policy.Verify(
		policy.ConfigBasis(),
		policy.ResolverPolicy(),
	); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityPolicyRecord{}, err
	}
	return ProjectTypeEnvHeadSelectionAuthorityPolicyRecord{
		variant: policy,
	}, nil
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) Kind() AuthorityPolicyKind {
	switch record.variant.(type) {
	case ExplicitHDecideAuthorityPolicy:
		return AuthorityPolicyExplicitHDecide
	case StrictCLISpeechActAuthorityPolicy:
		return AuthorityPolicyStrictCLISpeechAct
	default:
		return 0
	}
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) ExplicitHDecide() (
	ExplicitHDecideAuthorityPolicy,
	bool,
) {
	value, ok := record.variant.(ExplicitHDecideAuthorityPolicy)
	return value, ok
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) StrictCLISpeechAct() (
	StrictCLISpeechActAuthorityPolicy,
	bool,
) {
	value, ok := record.variant.(StrictCLISpeechActAuthorityPolicy)
	return value, ok
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) Mode() ProjectTypeEnvHeadSelectionAuthorityMode {
	switch value := record.variant.(type) {
	case ExplicitHDecideAuthorityPolicy:
		return value.Mode()
	case StrictCLISpeechActAuthorityPolicy:
		return value.Mode()
	default:
		return 0
	}
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) Project() projectidentity.ProjectID {
	switch value := record.variant.(type) {
	case ExplicitHDecideAuthorityPolicy:
		return value.Project()
	case StrictCLISpeechActAuthorityPolicy:
		return value.Project()
	default:
		return projectidentity.ProjectID{}
	}
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) Ref() ProjectTypeEnvHeadSelectionModePolicyRef {
	switch value := record.variant.(type) {
	case ExplicitHDecideAuthorityPolicy:
		return value.Ref()
	case StrictCLISpeechActAuthorityPolicy:
		return value.Ref()
	default:
		return ProjectTypeEnvHeadSelectionModePolicyRef{}
	}
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) Digest() authority.Digest {
	switch value := record.variant.(type) {
	case ExplicitHDecideAuthorityPolicy:
		return value.Digest()
	case StrictCLISpeechActAuthorityPolicy:
		return value.Digest()
	default:
		return authority.Digest{}
	}
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) ConfigBasis() ProjectTypeEnvHeadSelectionConfigAuthorityBasis {
	switch value := record.variant.(type) {
	case ExplicitHDecideAuthorityPolicy:
		return value.ConfigBasis()
	case StrictCLISpeechActAuthorityPolicy:
		return value.ConfigBasis()
	default:
		return ProjectTypeEnvHeadSelectionConfigAuthorityBasis{}
	}
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) ConfigCarrier() authority.ObservableCarrierBinding {
	switch value := record.variant.(type) {
	case ExplicitHDecideAuthorityPolicy:
		return value.ConfigCarrier()
	case StrictCLISpeechActAuthorityPolicy:
		return value.ConfigCarrier()
	default:
		return authority.ObservableCarrierBinding{}
	}
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) ProjectBinding() ProjectAuthorityContextBinding {
	switch value := record.variant.(type) {
	case ExplicitHDecideAuthorityPolicy:
		return value.ProjectBinding()
	case StrictCLISpeechActAuthorityPolicy:
		return value.ResolverPolicy().ProjectBinding()
	default:
		return ProjectAuthorityContextBinding{}
	}
}

func (record ProjectTypeEnvHeadSelectionAuthorityPolicyRecord) CanonicalJSON() []byte {
	switch value := record.variant.(type) {
	case ExplicitHDecideAuthorityPolicy:
		return value.CanonicalJSON()
	case StrictCLISpeechActAuthorityPolicy:
		return value.CanonicalJSON()
	default:
		return nil
	}
}

// ExplicitHDecideAuthorityPolicy binds the exact project config carrier to the
// compatibility/default rule. That project policy expects an explicit
// h-decide at the host boundary, but the pure kernel cannot observe the host
// occurrence. A dedicated CLI effect boundary may accept ingress under this
// lower-assurance policy. This record neither claims nor fabricates a
// kernel-observed SpeechAct, Permission, or controlling-terminal occurrence.
type ExplicitHDecideAuthorityPolicy struct {
	ref            ProjectTypeEnvHeadSelectionModePolicyRef
	digest         authority.Digest
	configBasis    ProjectTypeEnvHeadSelectionConfigAuthorityBasis
	projectBinding ProjectAuthorityContextBinding
	hostSkill      ProjectTypeEnvHeadSelectionHostSkill
	canonicalJSON  []byte
}

type explicitHDecidePolicyProjection struct {
	Schema                      string          `json:"schema"`
	Mode                        string          `json:"mode"`
	Project                     string          `json:"project_id"`
	ConfigBasisRef              string          `json:"config_basis_ref"`
	ConfigBasisDigest           string          `json:"config_basis_digest"`
	ConfigCarrierRef            string          `json:"config_carrier_ref"`
	ConfigCarrierDigest         string          `json:"config_carrier_digest"`
	ProjectRoot                 string          `json:"project_root"`
	ProjectContextBindingDigest string          `json:"project_context_binding_digest"`
	ProjectContextBinding       json.RawMessage `json:"project_context_binding"`
	JudgementContext            string          `json:"judgement_context_ref"`
	ProjectBindingCarrierRef    string          `json:"project_binding_carrier_ref"`
	ProjectBindingCarrierDigest string          `json:"project_binding_carrier_digest"`
	HostSkill                   string          `json:"host_skill"`
	AuthorityBoundary           string          `json:"authority_boundary"`
}

func SealExplicitHDecideAuthorityPolicy(
	configBasis ProjectTypeEnvHeadSelectionConfigAuthorityBasis,
	projectBinding ProjectAuthorityContextBinding,
) (ExplicitHDecideAuthorityPolicy, error) {
	if err := configBasis.Verify(
		configBasis.Project(),
		configBasis.Mode(),
		configBasis.ConfigCarrier(),
	); err != nil {
		return ExplicitHDecideAuthorityPolicy{}, err
	}
	if configBasis.Mode() != ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide {
		return ExplicitHDecideAuthorityPolicy{}, fmt.Errorf(
			"explicit h-decide policy requires explicit_h_decide config basis",
		)
	}
	if !projectBinding.ExactFor(
		projectBinding.Project(),
		projectBinding.Root(),
		projectBinding.Context(),
	) {
		return ExplicitHDecideAuthorityPolicy{}, fmt.Errorf(
			"explicit h-decide policy project-context binding is invalid",
		)
	}
	if projectBinding.Project() != configBasis.Project() {
		return ExplicitHDecideAuthorityPolicy{}, fmt.Errorf(
			"explicit h-decide policy project-context binding belongs to another project",
		)
	}
	policy := ExplicitHDecideAuthorityPolicy{
		configBasis:    configBasis,
		projectBinding: projectBinding,
		hostSkill:      ProjectTypeEnvHeadSelectionHostSkillHDecide,
	}
	carrier := configBasis.ConfigCarrier()
	bindingCarrier := projectBinding.Carrier()
	projection := explicitHDecidePolicyProjection{
		Schema:                      explicitHDecidePolicySchema,
		Mode:                        policy.Mode().String(),
		Project:                     configBasis.Project().String(),
		ConfigBasisRef:              configBasis.Ref().String(),
		ConfigBasisDigest:           configBasis.Digest().String(),
		ConfigCarrierRef:            carrier.Ref().String(),
		ConfigCarrierDigest:         carrier.Digest().String(),
		ProjectRoot:                 projectBinding.Root().String(),
		ProjectContextBindingDigest: projectBinding.Digest().String(),
		ProjectContextBinding:       json.RawMessage(projectBinding.CanonicalJSON()),
		JudgementContext:            projectBinding.Context().String(),
		ProjectBindingCarrierRef:    bindingCarrier.Ref().String(),
		ProjectBindingCarrierDigest: bindingCarrier.Digest().String(),
		HostSkill:                   policy.hostSkill.String(),
		AuthorityBoundary:           "configured_policy_accepts_dedicated_cli_ingress_only_in_exact_profile_selected_project_context;_host_h-decide_not_kernel_observed",
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ExplicitHDecideAuthorityPolicy{}, err
	}
	digest, err := digestModePolicy(explicitHDecidePolicyDomain, canonical)
	if err != nil {
		return ExplicitHDecideAuthorityPolicy{}, err
	}
	policy.ref = ProjectTypeEnvHeadSelectionModePolicyRef{digest: digest}
	policy.digest = digest
	policy.canonicalJSON = canonical
	return policy, nil
}

func DecodeExplicitHDecideAuthorityPolicy(
	configBasis ProjectTypeEnvHeadSelectionConfigAuthorityBasis,
	projectBinding ProjectAuthorityContextBinding,
	canonical []byte,
	digest authority.Digest,
) (ExplicitHDecideAuthorityPolicy, error) {
	if err := validateModePolicyCanonicalSize(canonical); err != nil {
		return ExplicitHDecideAuthorityPolicy{}, err
	}
	projection := explicitHDecidePolicyProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ExplicitHDecideAuthorityPolicy{}, err
	}
	rebuilt, err := SealExplicitHDecideAuthorityPolicy(
		configBasis,
		projectBinding,
	)
	if err != nil {
		return ExplicitHDecideAuthorityPolicy{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ExplicitHDecideAuthorityPolicy{}, fmt.Errorf(
			"explicit h-decide authority policy is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (policy ExplicitHDecideAuthorityPolicy) Verify(
	configBasis ProjectTypeEnvHeadSelectionConfigAuthorityBasis,
	projectBinding ProjectAuthorityContextBinding,
) error {
	rebuilt, err := SealExplicitHDecideAuthorityPolicy(
		configBasis,
		projectBinding,
	)
	if err != nil {
		return err
	}
	if !sameExplicitHDecidePolicy(policy, rebuilt) {
		return fmt.Errorf("explicit h-decide authority policy differs from exact config basis")
	}
	return nil
}

func (ExplicitHDecideAuthorityPolicy) projectTypeEnvHeadSelectionAuthorityPolicy() {}

func (ExplicitHDecideAuthorityPolicy) Mode() ProjectTypeEnvHeadSelectionAuthorityMode {
	return ProjectTypeEnvHeadSelectionAuthorityModeExplicitHDecide
}

func (policy ExplicitHDecideAuthorityPolicy) Project() projectidentity.ProjectID {
	return policy.configBasis.Project()
}

func (policy ExplicitHDecideAuthorityPolicy) Ref() ProjectTypeEnvHeadSelectionModePolicyRef {
	return policy.ref
}

func (policy ExplicitHDecideAuthorityPolicy) Digest() authority.Digest {
	return policy.digest
}

func (policy ExplicitHDecideAuthorityPolicy) ConfigCarrier() authority.ObservableCarrierBinding {
	return policy.configBasis.ConfigCarrier()
}

func (policy ExplicitHDecideAuthorityPolicy) ConfigBasis() ProjectTypeEnvHeadSelectionConfigAuthorityBasis {
	return policy.configBasis
}

func (policy ExplicitHDecideAuthorityPolicy) ProjectBinding() ProjectAuthorityContextBinding {
	return policy.projectBinding
}

func (policy ExplicitHDecideAuthorityPolicy) HostSkill() ProjectTypeEnvHeadSelectionHostSkill {
	return policy.hostSkill
}

func (policy ExplicitHDecideAuthorityPolicy) CanonicalJSON() []byte {
	return append([]byte(nil), policy.canonicalJSON...)
}

// StrictCLISpeechActAuthorityPolicy binds the opt-in config carrier to one
// exact strict resolver-policy snapshot. It cannot be used by the default
// explicit-h-decide constructor.
type StrictCLISpeechActAuthorityPolicy struct {
	ref            ProjectTypeEnvHeadSelectionModePolicyRef
	digest         authority.Digest
	configBasis    ProjectTypeEnvHeadSelectionConfigAuthorityBasis
	resolverPolicy ProjectTypeEnvHeadSelectionResolverPolicy
	canonicalJSON  []byte
}

type strictCLISpeechActPolicyProjection struct {
	Schema                string `json:"schema"`
	Mode                  string `json:"mode"`
	Project               string `json:"project_id"`
	ConfigBasisRef        string `json:"config_basis_ref"`
	ConfigBasisDigest     string `json:"config_basis_digest"`
	ConfigCarrierRef      string `json:"config_carrier_ref"`
	ConfigCarrierDigest   string `json:"config_carrier_digest"`
	ResolverPolicyRef     string `json:"resolver_policy_ref"`
	ResolverPolicyEdition string `json:"resolver_policy_edition"`
	ResolverPolicyDigest  string `json:"resolver_policy_digest"`
	AdmittedAction        string `json:"admitted_action"`
	AuthorityBoundary     string `json:"authority_boundary"`
}

func SealStrictCLISpeechActAuthorityPolicy(
	configBasis ProjectTypeEnvHeadSelectionConfigAuthorityBasis,
	resolverPolicy ProjectTypeEnvHeadSelectionResolverPolicy,
) (StrictCLISpeechActAuthorityPolicy, error) {
	if err := configBasis.Verify(
		configBasis.Project(),
		configBasis.Mode(),
		configBasis.ConfigCarrier(),
	); err != nil {
		return StrictCLISpeechActAuthorityPolicy{}, err
	}
	if configBasis.Mode() != ProjectTypeEnvHeadSelectionAuthorityModeStrictCLISpeechAct {
		return StrictCLISpeechActAuthorityPolicy{}, fmt.Errorf(
			"strict CLI SpeechAct policy requires strict_cli_speech_act config basis",
		)
	}
	if err := resolverPolicy.ExactAgainst(
		resolverPolicy.SourceContract(),
		resolverPolicy.SourceAdapter(),
		resolverPolicy.ProjectBinding(),
	); err != nil {
		return StrictCLISpeechActAuthorityPolicy{}, fmt.Errorf(
			"strict authority mode resolver policy: %w",
			err,
		)
	}
	if resolverPolicy.ProjectBinding().Project() != configBasis.Project() {
		return StrictCLISpeechActAuthorityPolicy{}, fmt.Errorf(
			"strict authority mode resolver policy belongs to another project",
		)
	}
	if _, err := resolverPolicy.Action().AuthorityActionKind(); err != nil {
		return StrictCLISpeechActAuthorityPolicy{}, err
	}
	policy := StrictCLISpeechActAuthorityPolicy{
		configBasis:    configBasis,
		resolverPolicy: resolverPolicy,
	}
	carrier := configBasis.ConfigCarrier()
	projection := strictCLISpeechActPolicyProjection{
		Schema:                strictCLISpeechActPolicySchema,
		Mode:                  policy.Mode().String(),
		Project:               configBasis.Project().String(),
		ConfigBasisRef:        configBasis.Ref().String(),
		ConfigBasisDigest:     configBasis.Digest().String(),
		ConfigCarrierRef:      carrier.Ref().String(),
		ConfigCarrierDigest:   carrier.Digest().String(),
		ResolverPolicyRef:     resolverPolicy.Ref().String(),
		ResolverPolicyEdition: resolverPolicy.Edition().String(),
		ResolverPolicyDigest:  resolverPolicy.Digest().String(),
		AdmittedAction:        resolverPolicy.Action().String(),
		AuthorityBoundary:     "exact_verified_speech_act_permission_and_resolution_required;_effect_service_owns_live_use",
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return StrictCLISpeechActAuthorityPolicy{}, err
	}
	digest, err := digestModePolicy(strictCLISpeechActPolicyDomain, canonical)
	if err != nil {
		return StrictCLISpeechActAuthorityPolicy{}, err
	}
	policy.ref = ProjectTypeEnvHeadSelectionModePolicyRef{digest: digest}
	policy.digest = digest
	policy.canonicalJSON = canonical
	return policy, nil
}

func DecodeStrictCLISpeechActAuthorityPolicy(
	configBasis ProjectTypeEnvHeadSelectionConfigAuthorityBasis,
	resolverPolicy ProjectTypeEnvHeadSelectionResolverPolicy,
	canonical []byte,
	digest authority.Digest,
) (StrictCLISpeechActAuthorityPolicy, error) {
	if err := validateModePolicyCanonicalSize(canonical); err != nil {
		return StrictCLISpeechActAuthorityPolicy{}, err
	}
	projection := strictCLISpeechActPolicyProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return StrictCLISpeechActAuthorityPolicy{}, err
	}
	rebuilt, err := SealStrictCLISpeechActAuthorityPolicy(
		configBasis,
		resolverPolicy,
	)
	if err != nil {
		return StrictCLISpeechActAuthorityPolicy{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return StrictCLISpeechActAuthorityPolicy{}, fmt.Errorf(
			"strict CLI SpeechAct authority policy is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (policy StrictCLISpeechActAuthorityPolicy) Verify(
	configBasis ProjectTypeEnvHeadSelectionConfigAuthorityBasis,
	resolverPolicy ProjectTypeEnvHeadSelectionResolverPolicy,
) error {
	rebuilt, err := SealStrictCLISpeechActAuthorityPolicy(
		configBasis,
		resolverPolicy,
	)
	if err != nil {
		return err
	}
	if !sameStrictCLISpeechActPolicy(policy, rebuilt) {
		return fmt.Errorf("strict CLI SpeechAct authority policy differs from exact config basis")
	}
	return nil
}

func (StrictCLISpeechActAuthorityPolicy) projectTypeEnvHeadSelectionAuthorityPolicy() {}

func (StrictCLISpeechActAuthorityPolicy) Mode() ProjectTypeEnvHeadSelectionAuthorityMode {
	return ProjectTypeEnvHeadSelectionAuthorityModeStrictCLISpeechAct
}

func (policy StrictCLISpeechActAuthorityPolicy) Project() projectidentity.ProjectID {
	return policy.configBasis.Project()
}

func (policy StrictCLISpeechActAuthorityPolicy) Ref() ProjectTypeEnvHeadSelectionModePolicyRef {
	return policy.ref
}

func (policy StrictCLISpeechActAuthorityPolicy) Digest() authority.Digest {
	return policy.digest
}

func (policy StrictCLISpeechActAuthorityPolicy) ConfigCarrier() authority.ObservableCarrierBinding {
	return policy.configBasis.ConfigCarrier()
}

func (policy StrictCLISpeechActAuthorityPolicy) ConfigBasis() ProjectTypeEnvHeadSelectionConfigAuthorityBasis {
	return policy.configBasis
}

func (policy StrictCLISpeechActAuthorityPolicy) ResolverPolicy() ProjectTypeEnvHeadSelectionResolverPolicy {
	return policy.resolverPolicy
}

func (policy StrictCLISpeechActAuthorityPolicy) CanonicalJSON() []byte {
	return append([]byte(nil), policy.canonicalJSON...)
}

func normalizeModePolicyCoordinates(
	project projectidentity.ProjectID,
	configCarrier authority.ObservableCarrierBinding,
) (
	projectidentity.ProjectID,
	authority.ObservableCarrierBinding,
	error,
) {
	canonicalProject, err := projectidentity.ParseProjectID(project.String())
	if err != nil || canonicalProject != project {
		return projectidentity.ProjectID{}, authority.ObservableCarrierBinding{}, fmt.Errorf(
			"head-selection authority policy ProjectID is invalid",
		)
	}
	carrier, err := authority.NewObservableCarrierBinding(
		configCarrier.Ref(),
		configCarrier.Digest(),
	)
	if err != nil {
		return projectidentity.ProjectID{}, authority.ObservableCarrierBinding{}, fmt.Errorf(
			"head-selection authority policy config carrier: %w",
			err,
		)
	}
	return canonicalProject, carrier, nil
}

func digestModePolicy(domain string, canonical []byte) (authority.Digest, error) {
	if err := validateModePolicyCanonicalSize(canonical); err != nil {
		return authority.Digest{}, err
	}
	return digestCanonical(domain, canonical)
}

func validateModePolicyCanonicalSize(canonical []byte) error {
	if len(canonical) == 0 || len(canonical) > maximumAuthorityModePolicyBytes {
		return fmt.Errorf("head-selection authority mode policy has invalid canonical size")
	}
	return nil
}

func sameExplicitHDecidePolicy(
	left ExplicitHDecideAuthorityPolicy,
	right ExplicitHDecideAuthorityPolicy,
) bool {
	return left.ref == right.ref &&
		left.digest == right.digest &&
		bytes.Equal(left.canonicalJSON, right.canonicalJSON)
}

func sameStrictCLISpeechActPolicy(
	left StrictCLISpeechActAuthorityPolicy,
	right StrictCLISpeechActAuthorityPolicy,
) bool {
	return left.ref == right.ref &&
		left.digest == right.digest &&
		bytes.Equal(left.canonicalJSON, right.canonicalJSON)
}
